package chief

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/privatefile"
)

// The windows of the automatic send. An observation counts only when the
// round that recorded it is less than observeWindow before the current one,
// and an observation carried after the merge counts only until carryWindow
// after the merge was first seen. /runpr can keep its pane busy for minutes
// after a merge, so both leave room for that.
const (
	observeWindow = 15 * time.Minute
	carryWindow   = 10 * time.Minute
)

// autoSendFacts is everything the conditions of an automatic send read for
// one PR a previous round saw open. Each read that failed is kept as its
// error, never as a value that would let the send go on.
type autoSendFacts struct {
	obs Observation
	// expired is true when the observation was carried past carryWindow.
	expired bool
	// partial is true when the reads stopped after the PR's state: the
	// workspace is gone, or the PR is not known to be merged, so the other
	// facts were not read and say nothing.
	partial bool
	// present is true when Herdr still has one task workspace at the
	// observation's worktree; shared counts the workspaces there when more
	// than one has it, so that none is chosen between them.
	present bool
	shared  int
	state   PRState
	// headRef is the merged PR's head branch.
	headRef  string
	stateErr error
	// fetchFailed is true when this round's `git fetch origin main` failed;
	// remoteErr is why origin/main could not be compared with the remote,
	// and remoteMatches whether it matched.
	fetchFailed   bool
	remoteErr     error
	remoteMatches bool
	// gitErr is why the worktree's git state could not be read; clean,
	// ahead, and atMain are that state.
	gitErr error
	clean  bool
	ahead  int
	atMain bool
	// head and branch are the worktree's HEAD.
	head   headState
	branch string
	// quietWhy is why the workspace has an agent running, on a question, or
	// unread; empty when every agent accepts input.
	quietWhy string
	// next is the next command of the task, done is true when the task is
	// complete, and nextWhy is why no next command was decided.
	next    *Next
	done    bool
	nextWhy string
	// autoSend is the auto-send field of the plan PR /runplan would create,
	// read only for /runplan.
	autoSend AutoSend
	// sent is true when the automatic send after the PR was recorded, or
	// when sentErr says its record cannot be read.
	sent    bool
	sentErr error
	// reportWhy is why a /runpr report on a pane disagrees with next or
	// could not be read; empty when none does.
	reportWhy string
	// detachedWhy is why a detached HEAD may not be sent to; empty when the
	// one /runpr pane's last report names the PR and next.
	detachedWhy string
	// stopped is true when the workspace is stopped, or when its stop
	// record cannot be read.
	stopped bool
	// laterSend is true when a send to the workspace started after the
	// round read it, read once its exclusion is held.
	laterSend bool
}

// autoSendConditions returns the conditions of an automatic send (as the
// design, 3.7, reads the requirements' list of them) that do not hold for f.
// Contract: an empty result is the only one that lets the send go on, and
// every fact read with an error, or left at its zero value, gives a reason;
// a condition is never assumed to hold.
func autoSendConditions(f autoSendFacts) []string {
	if f.partial {
		why := mergeConditions(f)
		if len(why) == 0 {
			why = []string{"the merge could not be confirmed"}
		}
		return append(why, "the other conditions were not checked, since the merge could not be confirmed")
	}
	var why []string
	for _, group := range []func(autoSendFacts) []string{mergeConditions, gitConditions, headConditions, nextConditions, recordConditions} {
		why = append(why, group(f)...)
	}
	return why
}

// mergeConditions returns why the observed PR is not a merge to send after:
// the carry ran out, the workspace is gone, the PR is not merged, or its
// head branch is of another task.
func mergeConditions(f autoSendFacts) []string {
	var why []string
	if f.expired {
		why = append(why, fmt.Sprintf("the merge was first seen more than %s ago", carryWindow))
	}
	switch {
	case f.shared > 1:
		why = append(why, fmt.Sprintf("%d Herdr workspaces are at %s, so none is chosen between them", f.shared, f.obs.Worktree))
	case !f.present:
		why = append(why, "no task workspace is at "+f.obs.Worktree+" now")
	}
	switch {
	case f.stateErr != nil:
		why = append(why, fmt.Sprintf("the state of #%d cannot be read: %v", f.obs.PR, f.stateErr))
	case f.state != PRStateMerged:
		why = append(why, fmt.Sprintf("#%d is not merged", f.obs.PR))
	}
	if task, ok := taskOfBranch(f.headRef); f.stateErr == nil && (!ok || task.String() != f.obs.Task) {
		why = append(why, fmt.Sprintf("the head branch %q of #%d is not of task %s", f.headRef, f.obs.PR, f.obs.Task))
	}
	return why
}

// gitConditions returns why origin/main or the worktree is not at the merge:
// the fetch failed, origin/main is not the remote's, or the worktree has
// changes, commits of its own, or another HEAD.
func gitConditions(f autoSendFacts) []string {
	var why []string
	switch {
	case f.fetchFailed:
		why = append(why, "this round's git fetch origin main failed")
	case f.remoteErr != nil:
		why = append(why, fmt.Sprintf("origin/main cannot be compared with the remote: %v", f.remoteErr))
	case !f.remoteMatches:
		why = append(why, "origin/main is not the remote's main")
	}
	switch {
	case f.gitErr != nil:
		why = append(why, fmt.Sprintf("the git state of the worktree cannot be read: %v", f.gitErr))
	case !f.clean:
		why = append(why, "the worktree has uncommitted changes")
	case f.ahead != 0:
		why = append(why, fmt.Sprintf("the worktree has %d commits beyond origin/main", f.ahead))
	case !f.atMain:
		why = append(why, "HEAD is not origin/main")
	}
	return why
}

// nextConditions returns why the next command is not one to send now: an
// agent runs or waits, no command is decided, the command is never sent by
// itself, it has no route, or its plan PR holds it.
func nextConditions(f autoSendFacts) []string {
	var why []string
	if f.quietWhy != "" {
		why = append(why, f.quietWhy)
	}
	switch {
	case f.next == nil && f.done:
		why = append(why, "the task is complete")
	case f.next == nil:
		why = append(why, "no next command is decided: "+f.nextWhy)
	case f.next.Kind != TaskNextMkarch && f.next.Kind != TaskNextMkplan && f.next.Kind != TaskNextRunplan:
		why = append(why, f.next.Text+" is never sent automatically")
	case f.next.Command == nil:
		why = append(why, f.next.Text+" cannot be sent: "+f.next.Problem)
	case f.next.Kind == TaskNextRunplan && f.autoSend == AutoSendHold:
		why = append(why, "the plan PR's auto-send field is hold")
	case f.next.Kind == TaskNextRunplan && f.autoSend != AutoSendAuto:
		why = append(why, "the plan PR's auto-send field cannot be read")
	}
	return why
}

// recordConditions returns why the records or the panes stop the send: the
// PR was sent already, a /runpr report disagrees, the workspace is stopped,
// or another send started during the round.
func recordConditions(f autoSendFacts) []string {
	var why []string
	switch {
	case f.sentErr != nil:
		why = append(why, fmt.Sprintf("the automatic-send record of #%d cannot be read: %v", f.obs.PR, f.sentErr))
	case f.sent:
		why = append(why, fmt.Sprintf("#%d was already sent automatically", f.obs.PR))
	}
	if f.reportWhy != "" {
		why = append(why, f.reportWhy)
	}
	if f.stopped {
		why = append(why, "the workspace is stopped until release")
	}
	if f.laterSend {
		why = append(why, "another send to the workspace started during this round")
	}
	return why
}

// headConditions returns why the worktree's HEAD does not allow the send: a
// feature branch of the task other than the merged one, or a detached HEAD
// for /runplan whose /runpr report agrees.
func headConditions(f autoSendFacts) []string {
	switch f.head {
	case headBranch:
		task, ok := taskOfBranch(f.branch)
		switch {
		case f.branch == f.headRef:
			return []string{"the merged PR's branch is still checked out"}
		case !ok || task.String() != f.obs.Task:
			return []string{fmt.Sprintf("the branch %q is no feature branch of task %s", f.branch, f.obs.Task)}
		}
		return nil
	case headDetached:
		switch {
		case f.next == nil || f.next.Kind != TaskNextRunplan:
			return []string{"HEAD is detached and the next command is not /runplan"}
		case f.detachedWhy != "":
			return []string{"HEAD is detached and " + f.detachedWhy}
		}
		return nil
	default:
		return []string{"the HEAD of the worktree was not read"}
	}
}

// AutoSendResult is one merged PR an automatic send was considered for:
// where, the next command (nil when none was decided), why it was not sent,
// and what was typed.
type AutoSendResult struct {
	Workspace string
	Label     string
	PR        int
	Task      string
	Proposal  *Next
	Reasons   []string
	Actions   []ActionResult
}

// autoSender decides and makes the automatic sends of one tick.
type autoSender struct {
	deps  Deps
	view  *roundView
	owner Owner
	round Round
	// prevAt is when the previous round, whose observations are considered,
	// began.
	prevAt time.Time
	herdr  Herdr
	git    GitReader
	tasks  Tasks
	// remote is origin/main compared with the remote once per round: the
	// first candidate reads it and the others reuse it.
	remote *remoteCheck
}

// remoteCheck is the result of comparing origin/main with the remote.
type remoteCheck struct {
	matches bool
	err     error
}

// checkRemote compares origin/main with the remote's main once per round.
func (a *autoSender) checkRemote(ctx context.Context) remoteCheck {
	if a.remote != nil {
		return *a.remote
	}
	tree := a.view.chiefTree.String()
	local, err := a.git.OriginMain(ctx, tree)
	c := remoteCheck{err: err}
	if err == nil {
		var remote string
		remote, c.err = a.git.RemoteMain(ctx, tree)
		c.matches = c.err == nil && remote == local
	}
	a.remote = &c
	return c
}

// workspaceAt returns the index of the round's task workspace at worktree
// and how many of the round's task workspaces are there. Contract: the index
// is valid only when exactly one is, as for a send the human asks for
// (locate): a worktree two Herdr workspaces hold names no one workspace.
func (a *autoSender) workspaceAt(worktree string) (int, int) {
	i, n := -1, 0
	for j, w := range a.view.found {
		if w.Worktree == worktree {
			i, n = j, n+1
		}
	}
	return i, n
}

// gather reads the facts of the conditions for obs in the workspace ws
// whose facts are f (ws is nil when Herdr has no workspace there).
func (a *autoSender) gather(ctx context.Context, obs Observation, ws *TaskWorkspace, f workspaceFacts) autoSendFacts {
	facts := autoSendFacts{obs: obs, present: ws != nil, fetchFailed: a.view.fetchFailed}
	if obs.MergedSince != nil && a.round.At.Sub(*obs.MergedSince) >= carryWindow {
		facts.expired = true
	}
	dir := a.view.chiefTree.String()
	if ws != nil {
		dir = ws.Worktree
	}
	facts.state, facts.headRef, facts.stateErr = a.git.PRStateOf(ctx, dir, obs.PR)
	facts.sent, facts.sentErr = a.deps.Store.AutoSent(obs.PR)
	if ws == nil || facts.stateErr != nil || facts.state != PRStateMerged {
		facts.partial = true
		return facts
	}
	remote := a.checkRemote(ctx)
	facts.remoteMatches, facts.remoteErr = remote.matches, remote.err
	branch, err := a.git.Branch(ctx, ws.Worktree)
	switch {
	case err != nil:
		facts.head = headUnread
	case branch == "":
		facts.head = headDetached
	default:
		facts.head, facts.branch = headBranch, branch
	}
	facts.gitErr = a.readGitState(ctx, ws.Worktree, &facts)
	facts.quietWhy = quietness(f)
	a.decideNext(ctx, ws, &facts)
	if facts.next != nil && facts.next.Command != nil {
		facts.reportWhy = a.reportsAgree(ctx, *ws, facts)
		if facts.head == headDetached {
			facts.detachedWhy = a.detachedReport(ctx, *ws, facts)
		}
	}
	records, ok, err := a.deps.Store.existingRecords(ws.Tree)
	switch {
	case err != nil:
		facts.stopped = true
	case ok:
		stop, err := records.Stopped()
		facts.stopped = err != nil || stop != nil
	}
	return facts
}

// readGitState reads whether the worktree is clean, how far HEAD is beyond
// origin/main, and whether HEAD is origin/main.
func (a *autoSender) readGitState(ctx context.Context, worktree string, facts *autoSendFacts) error {
	clean, err := a.git.Clean(ctx, worktree)
	if err != nil {
		return err
	}
	ahead, err := a.git.Ahead(ctx, worktree)
	if err != nil {
		return err
	}
	head, err := a.git.Head(ctx, worktree)
	if err != nil {
		return err
	}
	main, err := a.git.OriginMain(ctx, worktree)
	if err != nil {
		return err
	}
	facts.clean, facts.ahead, facts.atMain = clean, ahead, head == main
	return nil
}

// quietness returns why a workspace's agents do not all accept input: one is
// working or on a question, or a screen was not read or matches no prompt.
// It is empty when every agent waits for input on its prompt.
func quietness(f workspaceFacts) string {
	if len(f.Agents) == 0 {
		return "the workspace has no agent"
	}
	for _, a := range f.Agents {
		switch {
		case a.Agent.Status == AgentWorking:
			return "an agent of the workspace is running"
		case a.Agent.Status == AgentBlocked || (a.ScreenRead && a.Screen.Kind == ScreenQuestion):
			return "an agent of the workspace is stopped on a question"
		case !a.ScreenRead || a.Screen.Kind != ScreenPrompt:
			return "the screen of pane " + a.Agent.Pane.String() + " matches no prompt"
		case a.Agent.Status != AgentIdle && a.Agent.Status != AgentDone:
			return "an agent of the workspace does not wait for input"
		}
	}
	return ""
}

// decideNext decides the next command of the observed task from
// origin/main's documents, the task taken from the merged PR's head branch
// (the design, 3.1.4) and never from a screen.
func (a *autoSender) decideNext(ctx context.Context, ws *TaskWorkspace, facts *autoSendFacts) {
	task, ok := taskOfBranch(facts.headRef)
	if !ok || task.String() != facts.obs.Task {
		facts.nextWhy = "the merged PR's head branch names another task"
		return
	}
	docs, err := a.tasks.Read(ctx, ws.Worktree, facts.headRef)
	if err != nil {
		facts.nextWhy = "the task documents on origin/main cannot be read"
		return
	}
	f := workspaceFacts{Head: headBranch, Task: task, Docs: &docs, Routing: a.view.routing, RoutingErr: a.view.routingErr}
	facts.next, facts.done, facts.nextWhy = decideNext(f)
	if facts.next != nil && facts.next.Kind == TaskNextRunplan && docs.Plan != nil {
		if pr, ok := docs.Plan.NextPR(); ok {
			facts.autoSend = pr.AutoSend
		}
	}
}

// agrees reports whether a /runpr report's NEXT names the next command and
// its task.
func agrees(r RunprNext, next *Next, task string) bool {
	if r.Kind != NextCommand || r.Task.String() != task {
		return false
	}
	switch next.Kind {
	case TaskNextMkarch:
		return r.Command == NextCommandMkarch
	case TaskNextMkplan:
		return r.Command == NextCommandMkplan
	case TaskNextRunplan:
		return r.Command == NextCommandRunplan
	default:
		return false
	}
}

// reportsAgree reads every pane of the workspace for a /runpr report and
// returns why one disagrees with the next command, empty when none does: a
// report the human sees must not say another command than the one sent. Contract: a screen that cannot be read, or a NEXT
// line that cannot be read, counts as disagreeing; a pane with no NEXT line
// does not.
func (a *autoSender) reportsAgree(ctx context.Context, ws TaskWorkspace, facts autoSendFacts) string {
	for _, agent := range ws.Agents {
		raw, err := a.herdr.Screen(ctx, agent.Pane, reportSource(agent))
		if err != nil {
			return "the screen of pane " + agent.Pane.String() + " cannot be read for a /runpr report"
		}
		next, ok := readScreenNext(raw)
		switch {
		case !ok:
			return "the screen of pane " + agent.Pane.String() + " cannot be read for a /runpr report"
		case next.Kind == NextAbsent:
		case next.Kind == NextNone:
			return "the /runpr report on pane " + agent.Pane.String() + " says there is no next command, not " + facts.next.Text
		case !agrees(next, facts.next, facts.obs.Task):
			return "the /runpr report on pane " + agent.Pane.String() + " names another next command than " + facts.next.Text
		}
	}
	return ""
}

// detachedReport returns why a detached HEAD may not be sent to, empty when
// the workspace has one pane of the agent /runpr is routed to and that
// pane's last report names the merged PR and the next command.
func (a *autoSender) detachedReport(ctx context.Context, ws TaskWorkspace, facts autoSendFacts) string {
	route, err := a.view.routing.Route(paneinput.RouteRunpr)
	if err != nil {
		return "the routing table has no route for /runpr"
	}
	pane, _, ok := targetPane(ws, route.Agent())
	if !ok {
		return "the workspace has no single pane of the agent /runpr is routed to"
	}
	raw, err := a.herdr.Screen(ctx, pane.Pane, reportSource(pane))
	if err != nil {
		return "the screen of the /runpr pane cannot be read"
	}
	report, ok := readRunprReport(pane.Kind, raw)
	switch {
	case !ok:
		return "the /runpr pane shows no report"
	case !slices.Contains(report.PRs, facts.obs.PR):
		return fmt.Sprintf("the last report on the /runpr pane does not name #%d", facts.obs.PR)
	case !agrees(report.Next, facts.next, facts.obs.Task):
		return "the last report on the /runpr pane does not name " + facts.next.Text
	}
	return ""
}

// run considers every PR the previous round saw open for an automatic send,
// in the order of worktree and PR, and carries forward each one that may
// still be sent. Contract: a PR is sent at most once (the automatic-send
// record is written before the send and checked under the workspace's
// exclusion); nothing is sent unless every condition holds when read again
// under the exclusion; a PR still open, or closed without a merge, gives no
// result.
func (a *autoSender) run(ctx context.Context, prev []Observation, open map[observedPR]struct{}) []AutoSendResult {
	slices.SortFunc(prev, func(x, y Observation) int {
		if x.Worktree != y.Worktree {
			if x.Worktree < y.Worktree {
				return -1
			}
			return 1
		}
		return x.PR - y.PR
	})
	var results []AutoSendResult
	for _, obs := range prev {
		if _, still := open[observedPR{obs.Worktree, obs.PR}]; still {
			continue
		}
		res, carry := a.consider(ctx, obs)
		if carry {
			a.carry(obs, &res)
		}
		if res != nil {
			results = append(results, *res)
		}
	}
	return results
}

// observedPR is a PR in a worktree, as an observation names it.
type observedPR struct {
	worktree string
	pr       int
}

// carry writes obs into the current round with the time the merge was first
// seen and the time of the round that last saw the PR open, so the next
// round considers it again and still counts a send made since that round.
func (a *autoSender) carry(obs Observation, res **AutoSendResult) {
	since := a.round.At
	if obs.MergedSince != nil {
		since = *obs.MergedSince
	}
	open := a.openAt(obs)
	obs.Round, obs.MergedSince, obs.OpenAt = a.round.ID, &since, &open
	if err := a.deps.Store.RecordObservation(obs); err != nil && *res != nil {
		(*res).Reasons = append((*res).Reasons, "the observation could not be carried to the next round: "+err.Error())
	}
}

// openAt returns when the round that last saw obs's PR open began: the
// previous round for a first consideration, the time a carry kept
// otherwise.
func (a *autoSender) openAt(obs Observation) time.Time {
	if obs.OpenAt != nil {
		return *obs.OpenAt
	}
	return a.prevAt
}

// consider decides one PR: the result to show (nil for none) and whether it
// is carried to the next round.
func (a *autoSender) consider(ctx context.Context, obs Observation) (*AutoSendResult, bool) {
	var ws *TaskWorkspace
	var f workspaceFacts
	i, n := a.workspaceAt(obs.Worktree)
	if n == 1 {
		ws, f = &a.view.found[i], a.view.facts[i]
	}
	facts := a.gather(ctx, obs, ws, f)
	facts.shared = n
	if facts.stateErr == nil && facts.state != PRStateMerged {
		// Still open (in another branch now) or closed: nothing to send.
		return nil, false
	}
	res := &AutoSendResult{Workspace: obs.Worktree, PR: obs.PR, Task: obs.Task, Proposal: facts.next}
	if ws != nil {
		res.Label = ws.Label
	}
	res.Reasons = autoSendConditions(facts)
	if len(res.Reasons) > 0 {
		// Only a PR seen merged is carried (the design, 3.5): one whose state
		// could not be read is proposed, not tried again.
		return res, facts.stateErr == nil && !facts.sent && !facts.expired && !facts.done && facts.sentErr == nil
	}
	recorded := a.send(ctx, *ws, obs, res)
	return res, !recorded
}

// send makes the automatic send of res.Proposal to ws (the design, 3.7):
// the workspace's exclusion, the conditions read again under it, the check
// before the first input, the automatic-send record, the send record, and
// the inputs, then the reset of the /runpr pane. It returns whether the
// automatic-send record was written.
func (a *autoSender) send(ctx context.Context, ws TaskWorkspace, obs Observation, res *AutoSendResult) bool {
	cmd := *res.Proposal.Command
	base := ActionResult{Workspace: ws.Worktree, Input: InputCommand, Command: res.Proposal.Text, Agent: res.Proposal.Agent, Model: res.Proposal.Model}
	loc := located{chiefPane: a.view.chiefPane, chief: a.view.chief, chiefTree: a.view.chiefTree, ws: ws, found: true}
	held, refused := holdWorkspace(ctx, a.deps, &loc, base)
	if refused != nil {
		res.Actions = append(res.Actions, *refused)
		return false
	}
	defer func() { _ = held.Close() }()
	if !a.recheck(ctx, held, &loc, obs, res, base) {
		return false
	}
	route, err := a.view.routing.Route(cmd.Route())
	if err != nil {
		res.Actions = append(res.Actions, notSent(base, NotSentUnreadable, "the routing table has no route for the command"))
		return false
	}
	writer, err := NewPaneWriter(a.deps.Cmds, a.view.chiefPane, a.view.chief.Workspace)
	if err != nil {
		res.Actions = append(res.Actions, notSent(base, NotSentUnreadable, err.Error()))
		return false
	}
	recorded := false
	sc := sendContext{deps: a.deps, herdr: a.herdr, writer: writer, held: held, ws: loc.ws, beforeBegin: func() (NotSentReason, error) {
		// The owner may have changed while the conditions were read again.
		if reason, err := a.ownerStill(ctx); err != nil {
			return reason, err
		}
		err := a.deps.Store.recordAutoSend(autoSendRecord{PR: obs.PR, Worktree: obs.Worktree, Task: obs.Task, Command: res.Proposal.Text, Round: a.round.ID, At: a.deps.Now()})
		if errors.Is(err, privatefile.ErrExist) {
			return NotSentAutoSendRecord, fmt.Errorf("#%d was already sent automatically: %w", obs.PR, err)
		}
		recorded = err == nil
		if err != nil {
			return NotSentAutoSendRecord, fmt.Errorf("the automatic-send record cannot be written: %w", err)
		}
		return NotSentAutoSendRecord, nil
	}}
	sent := sc.sendCommand(ctx, cmd, route)
	if recorded && sent.Outcome == OutcomeNotSent {
		sent.Outcome = OutcomeRecordedNotSent
	}
	res.Actions = append(res.Actions, sent)
	switch sent.Outcome {
	case OutcomeSent:
		res.Actions = append(res.Actions, a.resetRunpr(ctx, sc, obs, res.Proposal, sent.Pane))
	case OutcomeRecordedNotSent:
		res.Reasons = []string{"the automatic send was recorded, but nothing was typed: " + sent.Detail}
	case OutcomeNotSent:
		res.Reasons = []string{"nothing was typed: " + sent.Detail}
	default:
		res.Reasons = []string{"the send failed after an input, so the workspace is stopped: " + sent.Detail}
	}
	return recorded
}

// recheck reads every condition of the automatic send of res.Proposal
// again once the workspace's exclusion is held: chief's owner, the one
// Herdr workspace at the worktree, the facts of the conditions, the sends
// since the PR was seen, and the next command and its route. It reports
// whether the send may go on, and otherwise says why in res.
func (a *autoSender) recheck(ctx context.Context, held *HeldWorkspace, loc *located, obs Observation, res *AutoSendResult, base ActionResult) bool {
	if err := checkOwner(ctx, a.deps, a.owner); err != nil {
		res.Actions = append(res.Actions, notSent(base, NotSentConditions, "chief's lock or session changed: "+err.Error()))
		return false
	}
	// The worktree must still be held by this one Herdr workspace: another
	// opened on it since the round was read would not be read below.
	if now, err := locate(ctx, a.deps, loc.ws.Worktree); err != nil || !now.found || now.ws.ID != loc.ws.ID {
		res.Actions = append(res.Actions, notSent(base, NotSentConditions, "the worktree is no longer held by this one Herdr workspace alone"))
		return false
	}
	f := readWorkspace(ctx, a.herdr, a.git, a.tasks, loc.ws)
	f.Routing, f.RoutingErr, f.FetchFailed = a.view.routing, a.view.routingErr, a.view.fetchFailed
	// The remote is compared again: it may have moved since the first
	// reading of this round.
	a.remote = nil
	facts := a.gather(ctx, obs, &loc.ws, f)
	// A send made since the merge was first seen (by the human, between
	// rounds of a carry) or during this round may have sent the command.
	since := a.round.At
	if obs.MergedSince != nil && obs.MergedSince.Before(since) {
		since = *obs.MergedSince
	}
	later, err := held.SentSince(since)
	facts.laterSend = err != nil || later
	// The same command sent by hand since the round that last saw the PR
	// open, before a round saw it merged, is not sent again; a carry keeps
	// that round's time.
	if !facts.laterSend {
		again, err := held.SentCommandSince(a.openAt(obs), res.Proposal.Text)
		facts.laterSend = err != nil || again
	}
	if why := autoSendConditions(facts); len(why) > 0 {
		res.Reasons = why
		res.Actions = append(res.Actions, notSent(base, NotSentConditions, "a condition no longer held once the workspace's exclusion was taken"))
		return false
	}
	if facts.next.Text != res.Proposal.Text || facts.next.Command.Route() != res.Proposal.Command.Route() {
		res.Proposal = facts.next
		res.Reasons = []string{"the next command or its route changed to " + facts.next.Text + " while the workspace's exclusion was taken"}
		return false
	}
	return true
}

// ownerStill checks before an input of the automatic send that this tick's
// owner still holds chief: the reason to report with the error, if any.
func (a *autoSender) ownerStill(ctx context.Context) (NotSentReason, error) {
	if err := checkOwner(ctx, a.deps, a.owner); err != nil {
		return NotSentConditions, fmt.Errorf("chief's lock or session changed: %w", err)
	}
	return NotSentConditions, nil
}

// checkOwner checks that owner still holds the lock and that chief's pane
// still has owner's session.
func checkOwner(ctx context.Context, deps Deps, owner Owner) error {
	if err := deps.Store.Require(owner); err != nil {
		return err
	}
	current, err := ResolveOwner(ctx, NewHerdr(deps.Cmds), deps.Env.PaneID)
	if err != nil {
		return err
	}
	if current != owner {
		return fmt.Errorf("%w: chief's pane no longer has the session that started this run", ErrNotOwner)
	}
	return nil
}

// resetRunpr empties the pane /runpr ran in after the automatic send went
// to sentPane, so /runpr's context is not carried into the pane's next
// command. Contract: the candidate is the one pane of the agent /runpr is
// routed to, never chosen by a screen; it is emptied only when it is not
// sentPane and its last message is still the /runpr report naming the
// merged PR and the command sent, its state_change_seq unchanged since the
// workspace's agents were read under the exclusion, before the command was
// sent, and chief's owner unchanged; it is checked and recorded like any
// send.
func (a *autoSender) resetRunpr(ctx context.Context, sc sendContext, obs Observation, next *Next, sentPane string) ActionResult {
	res := ActionResult{Workspace: sc.ws.Worktree, Input: InputReset}
	// The automatic-send record belongs to the command; the reset writes
	// only its own send record, once the owner is checked again: the
	// command's inputs may have taken long enough for chief to change hands.
	sc.beforeBegin = func() (NotSentReason, error) { return a.ownerStill(ctx) }
	if stop, err := sc.held.Stopped(); err != nil || stop != nil {
		return notSent(res, NotSentStopped, "the workspace was stopped by the automatic send")
	}
	route, err := a.view.routing.Route(paneinput.RouteRunpr)
	if err != nil {
		return notSent(res, NotSentUnreadable, "the routing table has no route for /runpr")
	}
	res.Agent = route.Agent()
	st, ok := resetPrompt(route.Agent())
	if !ok {
		return notSent(res, NotSentUnreadable, "the route has no agent")
	}
	res.Command, _ = st.prompt.Text()
	candidate, panes, ok := targetPane(sc.ws, route.Agent())
	if !ok {
		res.Panes = panes
		return notSent(res, NotSentPaneAmbiguous, "the workspace has no single pane of the agent /runpr is routed to")
	}
	res.Pane = candidate.Pane.String()
	if res.Pane == sentPane {
		return notSent(res, NotSentSamePane, "the automatic send went to the pane /runpr ran in")
	}
	agent, err := a.herdr.Agent(ctx, candidate.Pane)
	if err != nil {
		return notSent(res, NotSentUnreadable, "herdr agent get failed: "+err.Error())
	}
	if !sameAgent(agent, candidate) {
		return notSent(res, NotSentUnreadable, "the pane is no longer the same agent of the workspace")
	}
	raw, err := a.herdr.Screen(ctx, agent.Pane, reportSource(agent))
	if err != nil {
		return notSent(res, NotSentUnreadable, "herdr agent read failed: "+err.Error())
	}
	// The last message must still be /runpr's report of this merge, its PR
	// and the command sent: a later answer in the pane is not emptied.
	if report, ok := readRunprReport(agent.Kind, raw); !ok || !slices.Contains(report.PRs, obs.PR) || !agrees(report.Next, next, obs.Task) {
		return notSent(res, NotSentNoReport, fmt.Sprintf("the pane's last message is not the /runpr report of #%d naming %s", obs.PR, next.Text))
	}
	// The pane must not have moved since its agent was read under the
	// exclusion, before the command was sent: the report read now may be a
	// new answer of the same shape.
	seq := candidate.StateChangeSeq
	return sc.run(ctx, res, agent, []step{st}, paneCheck{session: agent.Session, seq: &seq}, route)
}
