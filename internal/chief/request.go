package chief

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// ErrBadRequest reports a send request whose arguments do not fit together:
// an unknown command kind, an issue number for another command than /mkreq,
// a PR number for another than /runpr, or a missing one. It is a usage
// error; nothing is read.
var ErrBadRequest = errors.New("the send request is malformed")

// RequestKind is the command a send request names. The zero value is no
// command.
type RequestKind int

const (
	// RequestUnknown is no command.
	RequestUnknown RequestKind = iota
	// RequestMkreq is /mkreq with the human's issue number.
	RequestMkreq
	// RequestMkarch is /mkarch of the workspace's task.
	RequestMkarch
	// RequestMkplan is /mkplan of the workspace's task.
	RequestMkplan
	// RequestRunplan is /runplan of the workspace's task by its plan's route.
	RequestRunplan
	// RequestRunplanTakeover is /runplan handed to Claude Code.
	RequestRunplanTakeover
	// RequestRunpr is /runpr of the workspace's open PR.
	RequestRunpr
	// RequestMergemain is /mergemain of the workspace's open PR in a merge
	// conflict, for the conflict item the human chose.
	RequestMergemain
)

// The names of the task commands, without their slash: the --command
// values and the names a transcript records.
const (
	nameMkreq   = "mkreq"
	nameMkarch  = "mkarch"
	nameMkplan  = "mkplan"
	nameRunplan = "runplan"
	// The --command values that are no command name of their own.
	nameRunplanTakeover = "runplan-takeover"
	nameRunpr           = "runpr"
	nameMergemain       = "mergemain"
)

// requestKinds are the --command values.
var requestKinds = map[string]RequestKind{
	nameMkreq:           RequestMkreq,
	nameMkarch:          RequestMkarch,
	nameMkplan:          RequestMkplan,
	nameRunplan:         RequestRunplan,
	nameRunplanTakeover: RequestRunplanTakeover,
	nameRunpr:           RequestRunpr,
	nameMergemain:       RequestMergemain,
}

// requestNames are the --command values that send each kind of offer, so the
// /chief session passes back the value the output names instead of reading
// it from the command line. Contract: every kind of offer but none has one,
// and `chief send` with it sends that offer (offered).
var requestNames = map[TaskNextKind]string{
	TaskNextMkreq:           nameMkreq,
	TaskNextMkarch:          nameMkarch,
	TaskNextMkplan:          nameMkplan,
	TaskNextRunplan:         nameRunplan,
	TaskNextRunplanTakeover: nameRunplanTakeover,
	TaskNextRunpr:           nameRunpr,
}

// SendRequest is a validated `chief send`: the workspace, the command kind,
// the one number the kind takes, and the item the human chose for a task
// command. Only ParseSendRequest makes one.
type SendRequest struct {
	workspace string
	kind      RequestKind
	issue     paneinput.IssueNumber
	pr        fixprguard.PRNumber
	// item is the ID of the proposal the human chose, set for the task
	// commands (itemKinds) and for no other.
	item ItemID
}

// itemKinds are the requests of a task's command, which name the proposal
// the human chose: the row a send rebuilds may offer the same kind of
// command for another task.
var itemKinds = map[RequestKind]struct{}{
	RequestMkarch:          {},
	RequestMkplan:          {},
	RequestRunplan:         {},
	RequestRunplanTakeover: {},
}

// ParseSendRequest validates the arguments of `chief send`; an empty issue,
// pr, or item means the flag was not given. Contract: --issue is taken by
// mkreq alone and required by it, --pr by runpr alone and required by it,
// --item by the task commands and mergemain alone and required by them, and
// each value passes its form; anything else is ErrBadRequest. The task is never an
// argument: the tool reads it from the workspace.
func ParseSendRequest(workspace, command, issue, pr, item string) (SendRequest, error) {
	kind, ok := requestKinds[command]
	if !ok {
		return SendRequest{}, fmt.Errorf("%w: unknown command %q", ErrBadRequest, command)
	}
	if !filepath.IsAbs(workspace) {
		return SendRequest{}, fmt.Errorf("%w: --workspace is not an absolute path", ErrBadRequest)
	}
	req := SendRequest{workspace: workspace, kind: kind}
	if (kind == RequestMkreq) != (issue != "") {
		return SendRequest{}, fmt.Errorf("%w: --issue goes with mkreq and only with it", ErrBadRequest)
	}
	if (kind == RequestRunpr) != (pr != "") {
		return SendRequest{}, fmt.Errorf("%w: --pr goes with runpr and only with it", ErrBadRequest)
	}
	if _, task := itemKinds[kind]; (task || kind == RequestMergemain) != (item != "") {
		return SendRequest{}, fmt.Errorf("%w: --item goes with mkarch, mkplan, runplan, runplan-takeover, and mergemain and only with them", ErrBadRequest)
	}
	var err error
	if item != "" {
		if req.item, err = parseItemID(item); err != nil {
			return SendRequest{}, fmt.Errorf("--item: %w", err)
		}
	}
	if issue != "" {
		if req.issue, err = paneinput.ParseIssueNumber(issue); err != nil {
			return SendRequest{}, fmt.Errorf("%w: %w", ErrBadRequest, err)
		}
	}
	if pr != "" {
		if req.pr, err = fixprguard.ParsePRNumber(pr); err != nil {
			return SendRequest{}, fmt.Errorf("%w: %w", ErrBadRequest, err)
		}
	}
	return req, nil
}

// located is chief's view of Herdr for one requested workspace.
type located struct {
	chiefPane PaneID
	chief     Agent
	chiefTree Worktree
	ws        TaskWorkspace
	found     bool
}

// locate finds the task workspace whose worktree is path. Contract: path is
// compared with the canonical worktrees of the current task workspaces after
// its own links are resolved, and git never runs in it; a path that is no
// current task workspace's worktree, or that more than one Herdr workspace
// holds, is not found.
func locate(ctx context.Context, deps Deps, path string) (located, error) {
	herdr := NewHerdr(deps.Cmds)
	chiefPane, err := ParsePaneID(deps.Env.PaneID)
	if err != nil {
		return located{}, err
	}
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return located{}, err
	}
	workspaces, err := herdr.Workspaces(ctx)
	if err != nil {
		return located{}, err
	}
	chief, err := chiefWorkspaceOf(agents, chiefPane)
	if err != nil {
		return located{}, err
	}
	chiefTree, err := CanonicalWorktree(ctx, deps.Cmds, chief.Dir)
	if err != nil {
		return located{}, fmt.Errorf("read the chief worktree: %w", err)
	}
	found, err := FindTaskWorkspaces(ctx, NewGitReader(deps.Cmds), deps.Cmds, agents, workspaces, chiefPane)
	if err != nil {
		return located{}, err
	}
	loc := located{chiefPane: chiefPane, chief: chief, chiefTree: chiefTree}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return loc, nil
	}
	matches := 0
	for _, w := range found {
		if w.Worktree == resolved {
			loc.ws = w
			matches++
		}
	}
	// A worktree two Herdr workspaces hold names no one workspace, so none
	// is chosen between them.
	loc.found = matches == 1
	return loc, nil
}

// SendOutcome is what `chief send` did: one result for the command and,
// after /runpr, one for the reset.
type SendOutcome struct {
	Actions []ActionResult
}

// Send sends the command the human chose for a workspace (the design, 6.3).
// Contract: the command is sent only when the tool itself offers it for the
// workspace now (the next command of its row, the takeover the row proposes,
// /mkreq to a workspace without a task, /runpr of the branch's open PR of
// that number, or /mergemain of its PR in a merge conflict when every
// condition of the human's choice holds, mergemainSend); the route comes from origin/main's routing table; the send
// holds the workspace's exclusion, settles unfinished sends first, and sends
// nothing to a stopped workspace; the workspace's row is read and the
// command checked only once the exclusion is held, so two sends cannot both
// act on a row read before either sent; a task command is sent only when the
// proposal of the row read again is the item the human chose (its item ID
// is req's), so a row whose task or proposal changed since it was shown
// sends nothing; a worktree on main is read with owner's history; after
// /runpr the detail is kept before the send and the maker's pane is reset
// only as resetAfterRunpr allows.
func Send(ctx context.Context, deps Deps, owner Owner, req SendRequest) (SendOutcome, error) {
	if req.kind == RequestUnknown {
		return SendOutcome{}, fmt.Errorf("%w: a request no parser made", ErrBadRequest)
	}
	loc, err := locate(ctx, deps, req.workspace)
	if err != nil {
		return SendOutcome{}, err
	}
	one := func(r ActionResult) SendOutcome { return SendOutcome{Actions: []ActionResult{r}} }
	base := ActionResult{Workspace: req.workspace, Input: InputCommand}
	if !loc.found {
		return one(notSent(base, NotSentNoWorkspace, "the path is no current task workspace's worktree, or more than one Herdr workspace holds it")), nil
	}
	base.Workspace = loc.ws.Worktree
	routing, routingErr := ReadRouting(ctx, deps.Cmds, loc.chiefTree.String())
	if routingErr != nil {
		return one(notSent(base, NotSentUnreadable, "the routing table cannot be read: "+routingErr.Error())), nil
	}
	held, refused := holdWorkspace(ctx, deps, &loc, base)
	if refused != nil {
		return one(*refused), nil
	}
	defer func() { _ = held.Close() }()
	tasks := NewTasks(deps.Cmds)
	git := NewGitReader(deps.Cmds)
	herdr := NewHerdr(deps.Cmds)
	old, oldErr := readOldValues(ctx, tasks, loc.chiefTree.String())
	f := readWorkspace(ctx, herdr, git, tasks, loc.ws)
	f.Routing = routing
	// A worktree on main is classified from the caller's history, as the
	// round that showed the item classified it. It is read under the
	// workspace's exclusion, which every write of this worktree's events
	// takes too, so they do not change before the send; a record of another
	// worktree can only make the row need a check.
	f = withHistory(ctx, deps.Store, tasks, f, owner.Session, loc.ws.Tree)
	row := classifyWorkspace(ctx, git, tasks, f, old, oldErr)
	if req.kind == RequestMergemain {
		mm := mergemainSend{deps: deps, loc: loc, herdr: herdr, git: git, tasks: tasks, held: held, facts: f, row: row, item: req.item, base: base}
		return one(mm.send(ctx)), nil
	}
	cmd, proposal, why := offered(row, req)
	if why != "" {
		return one(notSent(base, NotSentNotOffered, why)), nil
	}
	if _, task := itemKinds[req.kind]; task && proposalID(row.Workspace, proposal) != req.item {
		// The same kind of command, for another task or another PR of the
		// plan, or another proposal than the one the human chose.
		return one(notSent(base, NotSentNotOffered, "the workspace no longer offers the chosen item (it offers "+proposal.Text+" now)")), nil
	}
	base.Command, _ = cmd.Text()
	route, err := routing.Route(cmd.Route())
	if err != nil {
		return one(notSent(base, NotSentUnreadable, "the routing table has no route for the command")), nil
	}
	base.Agent = route.Agent()
	base.Model, _ = route.ModelText()
	writer, err := NewPaneWriter(deps.Cmds, loc.chiefPane, loc.chief.Workspace)
	if err != nil {
		return SendOutcome{}, err
	}
	sc := sendContext{deps: deps, herdr: herdr, writer: writer, held: held, ws: loc.ws}
	var rec *detailRecord
	docsDir := ""
	if f.Docs != nil {
		docsDir = f.Docs.Dir
	}
	if req.kind == RequestRunpr {
		rec = ensureDetail(ctx, deps, git, loc.ws, *row.PR, docsDir, routing)
	}
	res := sc.sendCommand(ctx, cmd, route)
	out := SendOutcome{Actions: []ActionResult{res}}
	if req.kind == RequestRunpr && res.Outcome == OutcomeSent {
		out.Actions = append(out.Actions, sc.resetMaker(ctx, f, res.Pane, *row.PR, rec, routing))
	}
	return out, nil
}

// mergemainSend is the send of /mergemain the human chose for a conflict
// item: the workspace as Send read it under the exclusion.
type mergemainSend struct {
	deps  Deps
	loc   located
	herdr Herdr
	git   GitReader
	tasks Tasks
	held  *HeldWorkspace
	facts workspaceFacts
	row   Row
	item  ItemID
	base  ActionResult
}

// send sends /mergemain <PR> when the row read under the exclusion is the
// conflict item the human chose and every condition of the human's choice holds.
// Contract: the PR is the row's open PR in a merge conflict, in a phase that
// offers an item, and the item ID of the row read now is the chosen one, so a
// new head or a resolution that has since failed sends nothing; main is
// fetched here and compared with the remote, and the other conditions are
// read under the exclusion (manualConflictConditions), never from the round
// that showed the item; the human's input after the task command, the
// earlier sends, and the other sends of a round are not conditions; the
// pane's report is kept before the first input, and a failure to keep it
// does not stop the send.
func (m mergemainSend) send(ctx context.Context) ActionResult {
	row := m.row
	if row.State != StateConflict || row.PR == nil {
		return notSent(m.base, NotSentNotOffered, "the workspace's PR is not in a merge conflict")
	}
	pr := *row.PR
	phase := conflictPhaseOf(m.held.WorkspaceRecords, pr)
	if !conflictOfferPhase(phase) {
		return notSent(m.base, NotSentNotOffered, "the conflict resolution is not in a phase that offers a send")
	}
	if conflictItemID(row.Workspace, pr, phase) != m.item {
		return notSent(m.base, NotSentNotOffered, "the workspace no longer offers the chosen item")
	}
	prNumber, err := fixprguard.ParsePRNumber(strconv.Itoa(pr.Number))
	if err != nil {
		return notSent(m.base, NotSentNotOffered, err.Error())
	}
	cmd, err := paneinput.NewMergemain(prNumber)
	if err != nil {
		return notSent(m.base, NotSentNotOffered, err.Error())
	}
	base := m.base
	base.Command, _ = cmd.Text()
	view := &roundView{chiefPane: m.loc.chiefPane, chief: m.loc.chief, chiefTree: m.loc.chiefTree}
	if err := m.git.Fetch(ctx, m.loc.chiefTree.String()); err != nil {
		view.fetchFailed = true
	}
	// The route is read after the fetch, so a routing table the fetch
	// brought in is the one sent by, never the one read before it.
	routing, err := ReadRouting(ctx, m.deps.Cmds, m.loc.chiefTree.String())
	if err != nil {
		return notSent(base, NotSentUnreadable, "the routing table cannot be read: "+err.Error())
	}
	view.routing = routing
	route, err := routing.Route(cmd.Route())
	if err != nil {
		return notSent(base, NotSentUnreadable, "the routing table has no route for the command")
	}
	base.Agent = route.Agent()
	base.Model, _ = route.ModelText()
	// The agents and the git state are read after the fetch, which can take
	// long, so a pane that started working meanwhile is seen.
	ws := m.loc.ws
	agents, err := m.herdr.Agents(ctx)
	if err != nil {
		return notSent(base, NotSentUnreadable, "herdr agent list failed: "+err.Error())
	}
	ws.Agents = slices.DeleteFunc(agents, func(a Agent) bool { return a.Workspace != ws.ID })
	fresh := readWorkspace(ctx, m.herdr, m.git, m.tasks, ws)
	cs := &conflictSender{a: &autoSender{deps: m.deps, view: view, herdr: m.herdr, git: m.git, tasks: m.tasks}}
	// The branch is the one the row (and so the chosen item) was read on: a
	// worktree switched to another branch during the fetch does not match.
	facts := cs.gather(ctx, ws, fresh, pr, m.facts.Branch, false, nil)
	if reasons := manualConflictConditions(facts); len(reasons) > 0 {
		return notSent(base, NotSentConditions, "a condition of the chosen /mergemain does not hold: "+strings.Join(reasons, ", "))
	}
	// The report is kept before the first input; a report that cannot be
	// kept is a lost nicety, not a reason to leave the conflict unresolved.
	_ = keepPaneReport(m.deps, m.loc.ws, pr)
	writer, err := NewPaneWriter(m.deps.Cmds, m.loc.chiefPane, m.loc.chief.Workspace)
	if err != nil {
		return notSent(base, NotSentUnreadable, err.Error())
	}
	sc := sendContext{deps: m.deps, herdr: m.herdr, writer: writer, held: m.held, ws: m.loc.ws, mergemainPR: pr.Number, mergemainHead: pr.Head}
	return sc.sendCommand(ctx, cmd, route)
}

// holdWorkspace takes the exclusion of the located workspace for a send or a
// relay and re-reads its agents under it. Contract: nothing is sent unless
// it returns a held workspace: another send holding it, an unfinished send
// it cannot settle, and a stop (or a stop record it cannot read) each
// return the result of nothing sent; the agents of loc are replaced by a
// list read once the exclusion is held, so a send never acts on a list read
// before another send ended.
func holdWorkspace(ctx context.Context, deps Deps, loc *located, base ActionResult) (*HeldWorkspace, *ActionResult) {
	records, err := deps.Store.Records(loc.ws.Tree)
	if err != nil {
		r := notSent(base, NotSentUnreadable, "the workspace's records cannot be opened: "+err.Error())
		return nil, &r
	}
	held, err := records.Lock()
	if errors.Is(err, ErrWorkspaceBusy) {
		r := notSent(base, NotSentBusy, "another send to the workspace is under way")
		return nil, &r
	}
	if err != nil {
		r := notSent(base, NotSentUnreadable, "the workspace's exclusion cannot be taken: "+err.Error())
		return nil, &r
	}
	if refused := settleHeld(ctx, deps, loc, held, base); refused != nil {
		_ = held.Close()
		return nil, refused
	}
	return held, nil
}

// settleHeld is what a send does once it holds the workspace's exclusion:
// it settles the sends left unfinished, refuses a stopped workspace, and
// replaces the agents of loc by a list read under the exclusion. Contract:
// a non-nil result means nothing is to be sent; the caller still holds the
// exclusion and closes it.
func settleHeld(ctx context.Context, deps Deps, loc *located, held *HeldWorkspace, base ActionResult) *ActionResult {
	refused := func(reason NotSentReason, detail string) *ActionResult {
		r := notSent(base, reason, detail)
		return &r
	}
	if _, err := held.SettleUnfinished(deps.Now()); err != nil {
		return refused(NotSentUnreadable, "an unfinished send cannot be settled: "+err.Error())
	}
	stop, err := held.Stopped()
	switch {
	case err != nil:
		return refused(NotSentStopped, "the stop record cannot be read, so the workspace counts as stopped: "+err.Error())
	case stop != nil:
		return refused(NotSentStopped, "the workspace is stopped until release: "+stop.Reason)
	}
	agents, err := NewHerdr(deps.Cmds).Agents(ctx)
	if err != nil {
		return refused(NotSentUnreadable, "herdr agent list failed: "+err.Error())
	}
	loc.ws.Agents = slices.DeleteFunc(agents, func(a Agent) bool { return a.Workspace != loc.ws.ID })
	return nil
}

// sendAfterCreate sends /mkreq of issue to the workspace `create` has just
// made, with the exclusion `create` has held since before it made anything.
// Contract: it is Send without the row's offer: the offer is the one check
// left out, because a workspace made a moment ago is on a detached HEAD
// and reads as "needs a look" until /mkreq makes the task's branch. Every
// other check is Send's: the workspace is found again from Herdr and git by
// its path and must be the workspace create made (workspaceID); held must
// be that worktree's own exclusion; the route is the routing table's
// /mkreq route; the sends left unfinished are settled, a stop refuses, and
// the agents are read again under the exclusion; the pane is chosen and
// every input checked by sendCommand, and a check that fails after the first
// input stops the workspace. The command is built from issue, which the
// caller validated, never from a record. It returns the result of nothing
// sent, never an error, so create can report it.
func sendAfterCreate(ctx context.Context, deps Deps, held *HeldWorkspace, tree Worktree, workspaceID string, issue paneinput.IssueNumber) ActionResult {
	base := ActionResult{Workspace: tree.String(), Input: InputCommand}
	cmd, err := paneinput.NewMkreq(issue)
	if err != nil {
		return notSent(base, NotSentUnreadable, "the command cannot be built: "+err.Error())
	}
	base.Command, _ = cmd.Text()
	loc, err := locate(ctx, deps, tree.String())
	if err != nil {
		return notSent(base, NotSentUnreadable, "the workspace cannot be found: "+err.Error())
	}
	if !loc.found || loc.ws.ID != workspaceID || loc.ws.Tree != tree {
		return notSent(base, NotSentNoWorkspace, "the path is not the worktree of the workspace that was made")
	}
	if filepath.Base(held.dir) != workspaceDirOf(loc.ws.Tree) {
		return notSent(base, NotSentUnreadable, "the exclusion held is not the one of the workspace's worktree")
	}
	routing, err := ReadRouting(ctx, deps.Cmds, loc.chiefTree.String())
	if err != nil {
		return notSent(base, NotSentUnreadable, "the routing table cannot be read: "+err.Error())
	}
	route, err := routing.Route(cmd.Route())
	if err != nil {
		return notSent(base, NotSentUnreadable, "the routing table has no route for the command")
	}
	base.Agent = route.Agent()
	base.Model, _ = route.ModelText()
	if refused := settleHeld(ctx, deps, &loc, held, base); refused != nil {
		return *refused
	}
	writer, err := NewPaneWriter(deps.Cmds, loc.chiefPane, loc.chief.Workspace)
	if err != nil {
		return notSent(base, NotSentUnreadable, "the writer cannot be made: "+err.Error())
	}
	sc := sendContext{deps: deps, herdr: NewHerdr(deps.Cmds), writer: writer, held: held, ws: loc.ws}
	return sc.sendCommand(ctx, cmd, route)
}

// resetMaker resets the pane of the command that created pr after /runpr
// went to runprPane. A send that ended without its end record stopped the
// workspace; the reset is a send too and does not go to a stopped workspace.
// A reset skipped because no report was found is kept for the human to
// choose when the last line the human typed into the pane is that command.
func (s sendContext) resetMaker(ctx context.Context, f workspaceFacts, runprPane string, pr OpenPR, rec *detailRecord, routing Routing) ActionResult {
	if stop, err := s.held.Stopped(); err != nil || stop != nil {
		return notSent(ActionResult{Workspace: s.ws.Worktree, Input: InputReset}, NotSentStopped, "the workspace was stopped by the /runpr send")
	}
	made, why := prMaker(f)
	res, skip := s.resetAfterRunpr(ctx, runprPane, pr, rec, made, why, routing)
	return s.offerSkip(ctx, res, skip, sceneAfterRunpr, func(a Agent) string {
		if reason, detail := s.paneLastInputMadePR(a, made); reason != NotSentUnknown {
			return reason.String() + ": " + detail
		}
		return ""
	})
}

// offered returns the command of req when the workspace's row offers it,
// with the proposal that offers it for a task command (row.Next, or
// row.Takeover for the takeover), or why it does not.
func offered(row Row, req SendRequest) (paneinput.Command, *Next, string) {
	next := func(kind TaskNextKind) (paneinput.Command, *Next, string) {
		if row.State != StateProposeNext || row.Next == nil || row.Next.Kind != kind || row.Next.Command == nil {
			return paneinput.Command{}, nil, fmt.Sprintf("the workspace's next command is not this one (state %s)", row.State)
		}
		return *row.Next.Command, row.Next, ""
	}
	switch req.kind {
	case RequestMkreq:
		cmd, why := offeredMkreq(row, req.issue)
		return cmd, nil, why
	case RequestMkarch:
		return next(TaskNextMkarch)
	case RequestMkplan:
		return next(TaskNextMkplan)
	case RequestRunplan:
		return next(TaskNextRunplan)
	case RequestRunplanTakeover:
		if row.State != StateProposeNext || row.Takeover == nil || row.Takeover.Command == nil {
			return paneinput.Command{}, nil, "the workspace has no takeover proposal"
		}
		return *row.Takeover.Command, row.Takeover, ""
	case RequestRunpr:
		cmd, why := offeredRunpr(row, req.pr)
		return cmd, nil, why
	default:
		return paneinput.Command{}, nil, "no command"
	}
}

// offeredMkreq returns /mkreq of issue when the row is a workspace without a
// task that is offered one.
func offeredMkreq(row Row, issue paneinput.IssueNumber) (paneinput.Command, string) {
	if row.State != StateProposeNext || row.Next == nil || row.Next.Kind != TaskNextMkreq {
		return paneinput.Command{}, "the workspace is not one without a task that waits for input"
	}
	cmd, err := paneinput.NewMkreq(issue)
	if err != nil {
		return paneinput.Command{}, err.Error()
	}
	return cmd, ""
}

// offeredRunpr returns /runpr of pr when pr is the row's open PR awaiting
// review.
func offeredRunpr(row Row, pr fixprguard.PRNumber) (paneinput.Command, string) {
	if row.State != StateAwaitingReview || row.PR == nil || strconv.Itoa(row.PR.Number) != pr.String() {
		return paneinput.Command{}, "the PR is not the open PR of the workspace's branch awaiting review"
	}
	cmd, err := paneinput.NewRunpr(pr)
	if err != nil {
		return paneinput.Command{}, err.Error()
	}
	return cmd, ""
}

// ensureDetail returns the record kept for the PR at the worktree's HEAD
// (the report record before the detail record), gathering and keeping a
// detail first when none is kept. Contract: it never keeps a report record,
// even when only a record of the description is kept: one kept just before
// /runpr is sent would let the reset's check of an unchanged pane miss what
// the human did in the pane before. A detail that cannot be kept gives nil,
// which keeps every pane from being reset.
func ensureDetail(ctx context.Context, deps Deps, git GitReader, ws TaskWorkspace, pr OpenPR, docsDir string, routing Routing) *detailRecord {
	if head, err := git.Head(ctx, ws.Worktree); err == nil {
		if records, err := deps.Store.Records(ws.Tree); err == nil {
			if rec, ok, err := records.keptPaneReport(pr.Number, head); err == nil && ok {
				return &rec
			}
		}
	}
	return CollectDetail(ctx, deps, ws, pr, docsDir, routing, nil, ReportRecordNone).record
}

// DetailOutcome is what `chief detail` found: the PR's detail, or why there
// is none.
type DetailOutcome struct {
	Detail  *PRDetail
	Problem string
}

// Detail gathers the detail of the workspace's open PR.
func Detail(ctx context.Context, deps Deps, path string) (DetailOutcome, error) {
	loc, err := locate(ctx, deps, path)
	if err != nil {
		return DetailOutcome{}, err
	}
	if !loc.found {
		return DetailOutcome{Problem: "the path is no current task workspace's worktree"}, nil
	}
	git := NewGitReader(deps.Cmds)
	branch, err := git.Branch(ctx, loc.ws.Worktree)
	if err != nil || branch == "" {
		return DetailOutcome{Problem: "the worktree has no branch to read a PR of"}, nil
	}
	prs, err := git.OpenPRs(ctx, loc.ws.Worktree, branch)
	switch {
	case err != nil:
		return DetailOutcome{Problem: "the open pull requests cannot be read: " + err.Error()}, nil
	case len(prs) != 1:
		return DetailOutcome{Problem: fmt.Sprintf("the branch has %d open pull requests, not one", len(prs))}, nil
	}
	docsDir := ""
	if docs, err := NewTasks(deps.Cmds).Read(ctx, loc.ws.Worktree, branch); err == nil {
		docsDir = docs.Dir
	}
	routing, routingErr := ReadRouting(ctx, deps.Cmds, loc.chiefTree.String())
	d := CollectDetail(ctx, deps, loc.ws, prs[0], docsDir, routing, routingErr, ReportRecordAllowed)
	return DetailOutcome{Detail: &d}, nil
}

// ReleaseOutcome is what `chief release` did.
type ReleaseOutcome struct {
	// Released is true when a stop was removed.
	Released bool
	Problem  string
}

// Release lifts the stop of the workspace at path, at the human's choice.
func Release(ctx context.Context, deps Deps, path string) (ReleaseOutcome, error) {
	loc, err := locate(ctx, deps, path)
	if err != nil {
		return ReleaseOutcome{}, err
	}
	if !loc.found {
		return ReleaseOutcome{Problem: "the path is no current task workspace's worktree"}, nil
	}
	records, err := deps.Store.Records(loc.ws.Tree)
	if err != nil {
		return ReleaseOutcome{}, err
	}
	held, err := records.Lock()
	if errors.Is(err, ErrWorkspaceBusy) {
		return ReleaseOutcome{Problem: "another send to the workspace is under way"}, nil
	}
	if err != nil {
		return ReleaseOutcome{}, err
	}
	defer func() { _ = held.Close() }()
	released, err := held.Release(deps.Now())
	if err != nil {
		return ReleaseOutcome{}, err
	}
	return ReleaseOutcome{Released: released}, nil
}
