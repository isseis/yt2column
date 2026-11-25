package chief

import (
	"bytes"
	"context"
	"slices"
	"strconv"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// ConflictPhase is how far the resolution of a conflict row has gone for the
// PR's current head. The zero value is ConflictRecordsUnreadable, which never
// sends automatically.
type ConflictPhase int

const (
	// ConflictRecordsUnreadable is a row whose send records cannot be read.
	ConflictRecordsUnreadable ConflictPhase = iota
	// ConflictUnsent is a row with no resolution send for the PR's head.
	ConflictUnsent
	// ConflictSending is a row whose last resolution send has started and
	// has no end yet.
	ConflictSending
	// ConflictEndedBeforeCommand is a row whose last resolution send ended
	// before /mergemain was typed.
	ConflictEndedBeforeCommand
	// ConflictOutcomeUnknown is a row whose last resolution send cannot be
	// told to have started /mergemain or not.
	ConflictOutcomeUnknown
	// ConflictUnresolved is a row whose last resolution send started
	// /mergemain, with every agent waiting for input and the PR still
	// conflicting at the same head.
	ConflictUnresolved
)

// conflictPhaseName names a phase for the output, unknownName for a value
// outside the range.
func conflictPhaseName(p ConflictPhase) string {
	switch p {
	case ConflictRecordsUnreadable:
		return "records_unreadable"
	case ConflictUnsent:
		return "unsent"
	case ConflictSending:
		return "sending"
	case ConflictEndedBeforeCommand:
		return "ended_before_command"
	case ConflictOutcomeUnknown:
		return "outcome_unknown"
	case ConflictUnresolved:
		return "unresolved"
	default:
		return unknownName
	}
}

// conflictPhaseOf decides how far the resolution of a conflict row has gone
// for pr from the worktree's send records (the design, 3.3.2 and 3.3.3).
// Contract: if the records directory cannot be read, or any send record
// (start or end) in the directory cannot be read, it returns
// ConflictRecordsUnreadable, which never sends automatically; otherwise the
// newest /mergemain send matching pr's number and head decides the phase, or
// ConflictUnsent when none matches.
func conflictPhaseOf(records WorkspaceRecords, pr OpenPR) ConflictPhase {
	if pr.Number <= 0 || pr.Head.IsZero() {
		return ConflictRecordsUnreadable
	}
	starts, ends, err := records.sendFiles()
	if err != nil {
		return ConflictRecordsUnreadable
	}
	slices.Sort(starts)
	var lastID string
	for _, id := range starts {
		rec, err := records.readSendStart(id)
		if err != nil {
			return ConflictRecordsUnreadable
		}
		if rec.Kind == paneinput.CommandMergemain.Name() && rec.PR == pr.Number && rec.Head == pr.Head {
			lastID = id
		}
	}
	var lastEnd *sendEnd
	for id := range ends {
		end, err := records.sendEnded(id)
		if err != nil {
			return ConflictRecordsUnreadable
		}
		if id == lastID {
			lastEnd = end
		}
	}
	if lastID == "" {
		return ConflictUnsent
	}
	return conflictPhaseOfEnd(lastEnd)
}

// conflictPhaseOfEnd maps the end record of the newest /mergemain send for
// the PR and head to its phase (the design, 3.3.2). A nil end means the send
// has no end record yet; a missing step member (Release or an older record)
// is never treated as StepUnknown.
func conflictPhaseOfEnd(end *sendEnd) ConflictPhase {
	if end == nil {
		return ConflictSending
	}
	if end.Step == nil {
		return ConflictOutcomeUnknown
	}
	step, ok := parseRecordStep(*end.Step)
	if !ok {
		return ConflictOutcomeUnknown
	}
	switch end.Outcome {
	case OutcomeNotSent.String():
		if step == StepUnknown {
			return ConflictEndedBeforeCommand
		}
	case OutcomeSent.String():
		if step == StepCommand {
			return ConflictUnresolved
		}
	case OutcomeSentThenFailed.String():
		switch step {
		case StepClear, StepModel:
			return ConflictEndedBeforeCommand
		case StepCommand:
			return ConflictOutcomeUnknown
		}
	}
	return ConflictOutcomeUnknown
}

// mergemainDefPath is the repository path of the /mergemain command
// definition compared between the PR branch's HEAD and origin/main.
const mergemainDefPath = ".claude/commands/mergemain.md"

// The English reason names a conflict auto-send reports when a condition
// does not hold (the design, 3.4.3).
const (
	conflictReasonPRChanged            = "pr_changed"
	conflictReasonPRNotMainBase        = "pr_not_main_base"
	conflictReasonPRCrossRepository    = "pr_cross_repository"
	conflictReasonNoSingleClaudePane   = "no_single_claude_pane"
	conflictReasonBranchMismatch       = "branch_mismatch"
	conflictReasonHeadMismatch         = "head_mismatch"
	conflictReasonDirty                = "dirty"
	conflictReasonFetchFailed          = "fetch_failed"
	conflictReasonRemoteMismatch       = "remote_mismatch"
	conflictReasonAgentsNotWaiting     = "agents_not_waiting"
	conflictReasonStopped              = nameStopped
	conflictReasonLaterInput           = "later_input"
	conflictReasonNoTranscript         = "no_transcript"
	conflictReasonTranscriptUnreadable = "transcript_unreadable"
	conflictReasonAlreadySent          = "already_sent"
	conflictReasonRecordUnreadable     = "record_unreadable"
	conflictReasonDefinitionMissing    = "definition_missing"
	conflictReasonDefinitionDiffers    = "definition_differs"
	conflictReasonRouteNotClaude       = "route_not_claude"
	conflictReasonRouteUnreadable      = "route_unreadable"
	conflictReasonAnotherSent          = "another_sent"
	conflictReasonMergemainRunning     = "mergemain_running"
	conflictReasonOwnerChanged         = "owner_changed"
	conflictReasonWorkspaceChanged     = "workspace_changed"
	conflictReasonSentSince            = "sent_since"
	conflictReasonCreationLocked       = "creation_locked"
	conflictReasonAutosentSameRound    = "autosent_same_round"
)

// mergemainDefState is the comparison of HEAD's and origin/main's
// mergemain.md. Its zero value is mergemainDefMissing, which never sends.
type mergemainDefState int

const (
	mergemainDefMissing mergemainDefState = iota
	mergemainDefDiffers
	mergemainDefSame
)

// mergemainRouteState is the routing table's mergemain row. Its zero value
// is mergemainRouteUnreadable, which never sends.
type mergemainRouteState int

const (
	mergemainRouteUnreadable mergemainRouteState = iota
	mergemainRouteNotClaude
	mergemainRouteClaude
)

// conflictFacts is every condition checked for an automatic /mergemain send.
// Its zero value fails every check so an unread fact never permits sending.
type conflictFacts struct {
	recheck           bool
	prChanged         bool
	baseIsMain        bool
	crossRepository   bool
	singleClaudePane  bool
	branchMatches     bool
	headMatches       bool
	clean             bool
	fetchFailed       bool
	remoteMatches     bool
	agentsWaiting     bool
	stopped           bool
	laterInput        LaterInput
	phase             ConflictPhase
	defState          mergemainDefState
	routeState        mergemainRouteState
	anotherSent       bool
	mergemainRunning  bool
	creationLocked    bool
	autosentSameRound bool
	workspaceChanged  bool
	sentSince         bool
}

// conflictConditions returns the English reason names of every condition that
// does not hold for f. Contract: an empty slice is the only result that
// allows sending; the zero value of conflictFacts never returns empty.
func conflictConditions(f conflictFacts) []string {
	var why []string
	why = append(why, conflictPRConditions(f)...)
	why = append(why, conflictGitConditions(f)...)
	why = append(why, conflictAgentAndRecordConditions(f)...)
	why = append(why, conflictDefAndRouteConditions(f)...)
	return why
}

func conflictPRConditions(f conflictFacts) []string {
	var why []string
	if f.recheck {
		if f.prChanged {
			why = append(why, conflictReasonPRChanged)
		}
		return why
	}
	if !f.baseIsMain {
		why = append(why, conflictReasonPRNotMainBase)
	}
	if f.crossRepository {
		why = append(why, conflictReasonPRCrossRepository)
	}
	return why
}

func conflictGitConditions(f conflictFacts) []string {
	var why []string
	if !f.branchMatches {
		why = append(why, conflictReasonBranchMismatch)
	}
	if !f.headMatches {
		why = append(why, conflictReasonHeadMismatch)
	}
	if !f.clean {
		why = append(why, conflictReasonDirty)
	}
	if f.fetchFailed {
		why = append(why, conflictReasonFetchFailed)
	} else if !f.remoteMatches {
		why = append(why, conflictReasonRemoteMismatch)
	}
	return why
}

func conflictAgentAndRecordConditions(f conflictFacts) []string {
	var why []string
	if !f.singleClaudePane {
		why = append(why, conflictReasonNoSingleClaudePane)
	}
	if !f.agentsWaiting {
		why = append(why, conflictReasonAgentsNotWaiting)
	}
	if f.stopped {
		why = append(why, conflictReasonStopped)
	}
	if f.singleClaudePane {
		switch f.laterInput {
		case LaterInputNone, LaterInputNoTaskEmpty:
		case LaterInputAfter, LaterInputNoTaskInput:
			why = append(why, conflictReasonLaterInput)
		case LaterInputNoTranscript:
			why = append(why, conflictReasonNoTranscript)
		default:
			why = append(why, conflictReasonTranscriptUnreadable)
		}
	}
	switch f.phase {
	case ConflictUnsent:
	case ConflictRecordsUnreadable:
		why = append(why, conflictReasonRecordUnreadable)
	default:
		why = append(why, conflictReasonAlreadySent)
	}
	if f.sentSince {
		why = append(why, conflictReasonSentSince)
	}
	return why
}

func conflictDefAndRouteConditions(f conflictFacts) []string {
	var why []string
	switch f.defState {
	case mergemainDefSame:
	case mergemainDefDiffers:
		why = append(why, conflictReasonDefinitionDiffers)
	default:
		why = append(why, conflictReasonDefinitionMissing)
	}
	switch f.routeState {
	case mergemainRouteClaude:
	case mergemainRouteNotClaude:
		why = append(why, conflictReasonRouteNotClaude)
	default:
		why = append(why, conflictReasonRouteUnreadable)
	}
	if f.anotherSent {
		why = append(why, conflictReasonAnotherSent)
	}
	if f.mergemainRunning {
		why = append(why, conflictReasonMergemainRunning)
	}
	if f.creationLocked {
		why = append(why, conflictReasonCreationLocked)
	}
	if f.autosentSameRound {
		why = append(why, conflictReasonAutosentSameRound)
	}
	if f.workspaceChanged {
		why = append(why, conflictReasonWorkspaceChanged)
	}
	return why
}

// ConflictSendResult is one conflict row an automatic /mergemain send was
// considered for in a tick.
type ConflictSendResult struct {
	Workspace string
	Label     string
	PR        int
	Head      CommitID
	Reasons   []string
	Action    *ActionResult
	// rowIndex is the index in roundView.rows this result belongs to, so
	// two workspaces sharing a worktree update their own rows.
	rowIndex int
}

// conflictSender decides and sends automatic /mergemain runs during a tick
// while sharing the round's autoSender state (creation lock status, remote
// check cache, owner verification, and readers).
type conflictSender struct {
	a        *autoSender
	autoSent map[string]struct{}
}

// run considers every unsent conflict row of the round in workspace-path
// order and sends at most one /mergemain per round. Contract: it never looks
// at the previous round; once any attempt may have typed input (typedInput),
// every subsequent candidate stops with another_sent; an attempt that typed
// nothing reports its reasons and lets the next candidate try.
func (c *conflictSender) run(ctx context.Context) []ConflictSendResult {
	running := c.anyMergemainRunning(ctx)
	sentThisRound := false
	var results []ConflictSendResult
	for i, row := range c.a.view.rows {
		if row.State != StateConflict || row.Conflict != ConflictUnsent || row.PR == nil {
			continue
		}
		if i >= len(c.a.view.found) || i >= len(c.a.view.facts) {
			continue
		}
		ws := c.a.view.found[i]
		wsFacts := c.a.view.facts[i]
		res, attemptedInput := c.consider(ctx, i, ws, wsFacts, *row.PR, running, sentThisRound)
		results = append(results, res)
		if attemptedInput {
			sentThisRound = true
		}
	}
	return results
}

// anyMergemainRunning reports whether any task workspace read this round has
// a working or blocked Claude Code pane whose running command is /mergemain
// or undecided (RunningUnknown). OpenCode panes never run /mergemain and are
// ignored.
func (c *conflictSender) anyMergemainRunning(ctx context.Context) bool {
	noSends := func(string) (lastCommand, error) { return lastCommand{}, nil }
	for _, w := range c.a.view.found {
		for _, a := range w.Agents {
			if a.Kind != paneinput.AgentClaudeCode {
				continue
			}
			if a.Status != AgentWorking && a.Status != AgentBlocked {
				continue
			}
			r := runningOfPane(ctx, c.a.deps, a, noSends)
			if r.why != RunningUnknownNone || r.cmd.Kind() == paneinput.CommandMergemain {
				return true
			}
		}
	}
	return false
}

// consider evaluates one unsent conflict row and sends /mergemain when every
// condition holds. It returns the result and whether the attempt may have
// typed an input into the pane.
func (c *conflictSender) consider(ctx context.Context, rowIndex int, ws TaskWorkspace, wsFacts workspaceFacts, pr OpenPR, running, anotherSent bool) (ConflictSendResult, bool) {
	res := ConflictSendResult{Workspace: ws.Worktree, Label: ws.Label, PR: pr.Number, Head: pr.Head, rowIndex: rowIndex}
	facts := c.gather(ctx, ws, wsFacts, pr, wsFacts.Branch, false, nil)
	facts.mergemainRunning = running
	facts.anotherSent = anotherSent
	facts.creationLocked = c.a.noSend != nil
	_, facts.autosentSameRound = c.autoSent[ws.Worktree]
	_, n := c.a.workspaceAt(ws.Worktree)
	facts.workspaceChanged = n != 1
	if reasons := conflictConditions(facts); len(reasons) > 0 {
		res.Reasons = reasons
		return res, false
	}
	return res, c.send(ctx, ws, pr, wsFacts.Branch, &res)
}

// gather reads the conditions for sending /mergemain to ws for pr. When
// recheck is true, held is the workspace's locked records and prChanged
// reports any change in PR state, base, repository, or head.
func (c *conflictSender) gather(ctx context.Context, ws TaskWorkspace, wsFacts workspaceFacts, pr OpenPR, origBranch string, recheck bool, held *HeldWorkspace) conflictFacts {
	f := conflictFacts{
		recheck:         recheck,
		baseIsMain:      pr.BaseIsMain,
		crossRepository: pr.CrossRepository,
		fetchFailed:     c.a.view.fetchFailed,
		agentsWaiting:   quietness(wsFacts) == "",
	}
	if !f.fetchFailed {
		remote := c.a.checkRemote(ctx)
		f.remoteMatches = remote.err == nil && remote.matches
	}
	branch, bErr := c.a.git.Branch(ctx, ws.Worktree)
	f.branchMatches = bErr == nil && origBranch != "" && origBranch != mainBranch && wsFacts.Head == headBranch && wsFacts.Branch == origBranch && branch == origBranch
	if recheck && f.branchMatches {
		f.prChanged = wsFacts.PR == nil ||
			wsFacts.PR.Number != pr.Number ||
			wsFacts.PR.Mergeable != MergeableConflicting ||
			wsFacts.PR.Head != pr.Head ||
			!wsFacts.PR.BaseIsMain ||
			wsFacts.PR.CrossRepository
	}
	if head, err := c.a.git.Head(ctx, ws.Worktree); err == nil {
		f.headMatches = head == pr.Head.String()
	}
	if clean, err := c.a.git.Clean(ctx, ws.Worktree); err == nil {
		f.clean = clean
	}
	if target, _, ok := targetPane(ws, paneinput.AgentClaudeCode); ok {
		f.singleClaudePane = true
		noSends := func(string) (lastCommand, error) { return lastCommand{}, nil }
		f.laterInput = runningOfPane(ctx, c.a.deps, target, noSends).input
	}
	f.stopped, f.phase, f.sentSince = c.readRecordFacts(ws, pr, recheck, held)
	f.defState = c.checkMergemainDef(ctx, ws.Worktree)
	f.routeState = c.checkMergemainRoute()
	return f
}

func (c *conflictSender) readRecordFacts(ws TaskWorkspace, pr OpenPR, recheck bool, held *HeldWorkspace) (stopped bool, phase ConflictPhase, sentSince bool) {
	if recheck && held != nil {
		stop, err := held.Stopped()
		stopped = err != nil || stop != nil
		phase = conflictPhaseOf(held.WorkspaceRecords, pr)
		later, err := held.SentSince(c.a.round.At)
		sentSince = err != nil || later
		return stopped, phase, sentSince
	}
	records, ok, err := c.a.deps.Store.existingRecords(ws.Tree)
	switch {
	case err != nil:
		return true, ConflictRecordsUnreadable, false
	case !ok:
		return false, ConflictUnsent, false
	default:
		stop, serr := records.Stopped()
		return serr != nil || stop != nil, conflictPhaseOf(records, pr), false
	}
}

func (c *conflictSender) checkMergemainDef(ctx context.Context, worktree string) mergemainDefState {
	branchDef, bErr := c.a.tasks.show(ctx, worktree, "HEAD", mergemainDefPath)
	mainDef, mErr := c.a.tasks.show(ctx, worktree, originMain, mergemainDefPath)
	if bErr != nil || mErr != nil {
		return mergemainDefMissing
	}
	if !bytes.Equal(branchDef, mainDef) {
		return mergemainDefDiffers
	}
	return mergemainDefSame
}

func (c *conflictSender) checkMergemainRoute() mergemainRouteState {
	if c.a.view.routingErr != nil {
		return mergemainRouteUnreadable
	}
	route, err := c.a.view.routing.Route(paneinput.RouteMergemain)
	if err != nil {
		return mergemainRouteUnreadable
	}
	if route.Agent() != paneinput.AgentClaudeCode {
		return mergemainRouteNotClaude
	}
	return mergemainRouteClaude
}

// send locks ws, rechecks every condition under the exclusion, keeps the
// pane's report if readable, and sends /mergemain <PR>. It returns whether
// an input may have been typed into the pane.
func (c *conflictSender) send(ctx context.Context, ws TaskWorkspace, pr OpenPR, origBranch string, res *ConflictSendResult) bool {
	prNum, err := fixprguard.ParsePRNumber(strconv.Itoa(pr.Number))
	if err != nil {
		res.Reasons = []string{conflictReasonPRChanged}
		return false
	}
	cmd, err := paneinput.NewMergemain(prNum)
	if err != nil {
		res.Reasons = []string{conflictReasonPRChanged}
		return false
	}
	route, err := c.a.view.routing.Route(paneinput.RouteMergemain)
	if err != nil {
		res.Reasons = []string{conflictReasonRouteUnreadable}
		return false
	}
	if route.Agent() != paneinput.AgentClaudeCode {
		res.Reasons = []string{conflictReasonRouteNotClaude}
		return false
	}
	cmdText, _ := cmd.Text()
	model, _ := route.ModelText()
	base := ActionResult{Workspace: ws.Worktree, Input: InputCommand, Command: cmdText, Agent: route.Agent(), Model: model}
	loc := located{chiefPane: c.a.view.chiefPane, chief: c.a.view.chief, chiefTree: c.a.view.chiefTree, ws: ws, found: true}
	held, refused := holdWorkspace(ctx, c.a.deps, &loc, base)
	if refused != nil {
		res.Action = refused
		res.Reasons = []string{refused.Reason.String()}
		return false
	}
	defer func() { _ = held.Close() }()
	if !c.recheck(ctx, held, &loc, pr, origBranch, res, base) {
		return false
	}
	if err := keepPaneReport(c.a.deps, loc.ws, pr); err != nil {
		c.a.view.problems = append(c.a.view.problems, "the pane report of #"+strconv.Itoa(pr.Number)+" cannot be kept before /mergemain: "+err.Error())
	}
	writer, err := NewPaneWriter(c.a.deps.Cmds, c.a.view.chiefPane, c.a.view.chief.Workspace)
	if err != nil {
		act := notSent(base, NotSentUnreadable, err.Error())
		res.Action = &act
		res.Reasons = []string{act.Reason.String()}
		return false
	}
	sc := sendContext{
		deps:          c.a.deps,
		herdr:         c.a.herdr,
		writer:        writer,
		held:          held,
		ws:            loc.ws,
		mergemainPR:   pr.Number,
		mergemainHead: pr.Head,
		beforeBegin:   func() (NotSentReason, error) { return c.a.ownerStill(ctx) },
	}
	sent := sc.sendCommand(ctx, cmd, route)
	res.Action = &sent
	if sent.Outcome == OutcomeNotSent {
		if sent.Reason == NotSentConditions {
			res.Reasons = []string{conflictReasonOwnerChanged}
		} else {
			res.Reasons = []string{sent.Reason.String()}
		}
	}
	return typedInput(sent)
}

// recheck verifies chief's owner, unique workspace identity, fresh git/PR/
// agent/transcript/record/definition conditions, and SentSince under held.
func (c *conflictSender) recheck(ctx context.Context, held *HeldWorkspace, loc *located, pr OpenPR, origBranch string, res *ConflictSendResult, base ActionResult) bool {
	if err := checkOwner(ctx, c.a.deps, c.a.owner); err != nil {
		res.Reasons = []string{conflictReasonOwnerChanged}
		act := notSent(base, NotSentConditions, "chief's lock or session changed: "+err.Error())
		res.Action = &act
		return false
	}
	now, err := locate(ctx, c.a.deps, loc.ws.Worktree)
	if err != nil || !now.found || now.ws.ID != loc.ws.ID {
		res.Reasons = []string{conflictReasonWorkspaceChanged}
		act := notSent(base, NotSentConditions, "the worktree is no longer held by this one Herdr workspace alone")
		res.Action = &act
		return false
	}
	loc.ws = now.ws
	f := readWorkspace(ctx, c.a.herdr, c.a.git, c.a.tasks, loc.ws)
	// Force a fresh remote comparison under the exclusion.
	c.a.remote = nil
	facts := c.gather(ctx, loc.ws, f, pr, origBranch, true, held)
	if reasons := conflictConditions(facts); len(reasons) > 0 {
		res.Reasons = reasons
		act := notSent(base, NotSentConditions, "a condition no longer held once the workspace's exclusion was taken")
		res.Action = &act
		return false
	}
	return true
}
