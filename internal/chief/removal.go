package chief

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// RemovalCheck is one check before a removal. The zero value is no check
// and never passes.
type RemovalCheck int

const (
	// RemovalCheckUnknown is no check.
	RemovalCheckUnknown RemovalCheck = iota
	// RemovalCheckCandidate is the workspace still being a candidate: chief
	// made it, it is open at its path, and its issue's one task is done.
	RemovalCheckCandidate
	// RemovalCheckDetached is the worktree's HEAD being detached.
	RemovalCheckDetached
	// RemovalCheckClean is no tracked change and no untracked file that git
	// does not ignore.
	RemovalCheckClean
	// RemovalCheckHeadOnMain is the worktree's HEAD being reachable from
	// origin/main.
	RemovalCheckHeadOnMain
	// RemovalCheckUnlocked is the worktree's registration not being locked.
	RemovalCheckUnlocked
	// RemovalCheckAgentsIdle is no agent of the workspace working or asking.
	RemovalCheckAgentsIdle
	// RemovalCheckShellsIdle is every pane without an agent showing its
	// shell alone in the foreground.
	RemovalCheckShellsIdle
	// RemovalCheckPathUnused is no other workspace having a pane at the path.
	RemovalCheckPathUnused
)

// removalChecks are the checks in the order they are run and shown.
var removalChecks = []RemovalCheck{
	RemovalCheckCandidate, RemovalCheckDetached, RemovalCheckClean, RemovalCheckHeadOnMain,
	RemovalCheckUnlocked, RemovalCheckAgentsIdle, RemovalCheckShellsIdle, RemovalCheckPathUnused,
}

// removalCheckNames are the output spellings of the checks: each names what
// a check that does not pass found.
var removalCheckNames = map[RemovalCheck]string{
	RemovalCheckCandidate:  "not_candidate",
	RemovalCheckDetached:   "head_on_branch",
	RemovalCheckClean:      "not_clean",
	RemovalCheckHeadOnMain: "head_not_on_main",
	RemovalCheckUnlocked:   "worktree_locked",
	RemovalCheckAgentsIdle: "agent_active",
	RemovalCheckShellsIdle: "shell_busy",
	RemovalCheckPathUnused: "path_in_use",
}

// String returns the output spelling, "unknown" for no check.
func (c RemovalCheck) String() string {
	if name, ok := removalCheckNames[c]; ok {
		return name
	}
	return unknownName
}

// CheckResult is one check's outcome. Unreadable names what could not be
// read; a check with any is not passed.
type CheckResult struct {
	Check      RemovalCheck
	Passed     bool
	Unreadable []string
}

// checkResult builds the result of check: passed only when nothing was
// unreadable.
func checkResult(check RemovalCheck, passed bool, unreadable []string) CheckResult {
	return CheckResult{Check: check, Passed: passed && len(unreadable) == 0, Unreadable: unreadable}
}

// passedCheck reports whether the check of kind is among results and passed.
func passedCheck(results []CheckResult, kind RemovalCheck) bool {
	return slices.ContainsFunc(results, func(r CheckResult) bool { return r.Check == kind && r.Passed })
}

// errNoCandidate reports a removal check asked for with the zero
// RemovalCandidate.
var errNoCandidate = errors.New("no removal candidate")

// RemovalCandidate is a workspace chief made that met every condition of a
// removal proposal when read: its created record names its worktree, which
// is where its issue's task is made; the workspace is the only one open at
// that path; and origin/main has exactly one task of the issue, which is
// done. Only newRemovalCandidate makes one, and only readRemovalCandidate
// calls it, so no caller value names a workspace to remove. The zero value
// passes no check and makes no command run.
type RemovalCandidate struct {
	tree      Worktree
	workspace string
	issue     paneinput.IssueNumber
	task      paneinput.TaskID
}

// newRemovalCandidate is the one constructor of RemovalCandidate; its
// caller has read every condition.
func newRemovalCandidate(tree Worktree, workspace string, issue paneinput.IssueNumber, task paneinput.TaskID) RemovalCandidate {
	return RemovalCandidate{tree: tree, workspace: workspace, issue: issue, task: task}
}

// RemovalProposal is what a removal item shows: the workspace (its canonical
// worktree path, the identity the subcommands take), its label, the issue
// and its task, how many panes closing it closes, and the checks as this
// round read them.
type RemovalProposal struct {
	Workspace string
	Label     string
	Issue     paneinput.IssueNumber
	Task      paneinput.TaskID
	Panes     int
	Checks    []CheckResult
}

// removalID is a removal item's identity: the workspace and its issue.
func removalID(p *RemovalProposal) ItemID {
	return itemID("removal", p.Workspace, p.Issue.String())
}

// removalReader reads what the candidates of one round share: Herdr's panes
// and the issue lines of origin/main. Each is read at most once, and only
// when a workspace has a readable created record, so a round with none runs
// neither command.
type removalReader struct {
	cmds      Commands
	store     Store
	home      string
	chiefTree Worktree

	panesRead bool
	panes     []listedPane
	panesErr  error

	indexRead bool
	index     IssueIndex
	indexErr  error
}

func (r *removalReader) readPanes(ctx context.Context) ([]listedPane, error) {
	if !r.panesRead {
		r.panesRead = true
		r.panes, r.panesErr = NewHerdr(r.cmds).panes(ctx)
	}
	return r.panes, r.panesErr
}

func (r *removalReader) readIndex(ctx context.Context) (IssueIndex, error) {
	if !r.indexRead {
		r.indexRead = true
		r.index, r.indexErr = NewTasks(r.cmds).IssueIndex(ctx, r.chiefTree.String())
	}
	return r.index, r.indexErr
}

// readRemovalCandidate returns the removal candidate of w, or nil when w is
// not one, with the problem to show when a read failed. Contract: a
// workspace with no readable created record runs no command; the record
// directory is read without being created; a record that cannot be read,
// that names another worktree, or that names a worktree its issue's task is
// not made at, makes no candidate; the workspace must be the only one with a
// pane at the path; origin/main must have exactly one task of the record's
// issue, and its approved plan must have every PR merged.
func (r *removalReader) readRemovalCandidate(ctx context.Context, w TaskWorkspace) (*RemovalCandidate, string) {
	records, ok, err := r.store.existingRecords(w.Tree)
	if err != nil || !ok {
		// A round's addRecords already names a directory that cannot be
		// read; a removal refuses with the candidate check not passed.
		return nil, ""
	}
	rec, err := records.Created(r.home)
	if err != nil {
		return nil, "the created record of " + w.Worktree + " cannot be read, so it is not offered for removal: " + err.Error()
	}
	if rec == nil || rec.Worktree != w.Tree {
		return nil, ""
	}
	panes, err := r.readPanes(ctx)
	if err != nil {
		return nil, "herdr pane list cannot be read, so " + w.Worktree + " is not offered for removal: " + err.Error()
	}
	if at := workspacesAtPath(panes, w.Tree); len(at) != 1 || at[0] != w.ID {
		return nil, ""
	}
	index, err := r.readIndex(ctx)
	if err != nil {
		return nil, "the issue lines of origin/main cannot be read, so " + w.Worktree + " is not offered for removal: " + err.Error()
	}
	dirs, err := index.Tasks(rec.Issue)
	if err != nil {
		return nil, "the tasks of issue " + rec.Issue.String() + " on origin/main cannot be read, so " + w.Worktree + " is not offered for removal: " + err.Error()
	}
	if len(dirs) != 1 {
		return nil, ""
	}
	docs, err := NewTasks(r.cmds).ReadDir(ctx, r.chiefTree.String(), dirs[0])
	if err != nil {
		return nil, "the documents of " + dirs[0] + " on origin/main cannot be read, so " + w.Worktree + " is not offered for removal: " + err.Error()
	}
	if stageOf(&docs) != stagePlanApproved || !docs.Plan.Done() {
		return nil, ""
	}
	c := newRemovalCandidate(w.Tree, w.ID, rec.Issue, docs.Task)
	return &c, ""
}

// checkRemoval runs every check of a removal on c and returns one result
// per check, in the order of removalChecks. agents are the workspace's
// agents with their screens and panes every pane Herdr lists, both read by
// the caller. Contract: the zero candidate passes no check and runs nothing;
// a read that fails makes its check not passed with what it could not read
// named, never passed. head is the HEAD commit the check of HEAD on
// origin/main found reachable, and "" when that check did not pass.
func checkRemoval(ctx context.Context, cmds Commands, chiefTree Worktree, c RemovalCandidate, agents []agentFacts, panes []listedPane) (results []CheckResult, head string) {
	if c.tree.path == "" {
		out := make([]CheckResult, 0, len(removalChecks))
		for _, check := range removalChecks {
			out = append(out, checkResult(check, false, []string{errNoCandidate.Error()}))
		}
		return out, ""
	}
	git := NewGitReader(cmds)
	tree := c.tree.String()
	onMain, head := checkHeadOnMain(ctx, git, tree)
	return []CheckResult{
		checkResult(RemovalCheckCandidate, true, nil),
		checkDetached(ctx, git, tree),
		checkClean(ctx, git, tree),
		onMain,
		checkUnlocked(ctx, git, chiefTree, c.tree),
		checkAgentsIdle(c.workspace, agents, panes),
		checkShellsIdle(ctx, NewHerdr(cmds), c.workspace, panes),
		checkPathUnused(c, panes),
	}, head
}

func checkDetached(ctx context.Context, git GitReader, tree string) CheckResult {
	branch, err := git.Branch(ctx, tree)
	if err != nil {
		return checkResult(RemovalCheckDetached, false, []string{"the HEAD of the worktree"})
	}
	return checkResult(RemovalCheckDetached, branch == "", nil)
}

func checkClean(ctx context.Context, git GitReader, tree string) CheckResult {
	clean, err := git.Clean(ctx, tree)
	if err != nil {
		return checkResult(RemovalCheckClean, false, []string{"the status of the worktree"})
	}
	return checkResult(RemovalCheckClean, clean, nil)
}

// checkHeadOnMain checks that origin/main reaches the worktree's HEAD and
// returns that HEAD when it does, so a removal compares a later read with
// the commit this check passed.
func checkHeadOnMain(ctx context.Context, git GitReader, tree string) (CheckResult, string) {
	head, err := git.Head(ctx, tree)
	if err != nil {
		return checkResult(RemovalCheckHeadOnMain, false, []string{"the HEAD commit of the worktree"}), ""
	}
	main, err := git.OriginMain(ctx, tree)
	if err != nil {
		return checkResult(RemovalCheckHeadOnMain, false, []string{"origin/main"}), ""
	}
	reachable, err := git.IsAncestor(ctx, tree, head, main)
	if err != nil {
		return checkResult(RemovalCheckHeadOnMain, false, []string{"whether origin/main reaches the HEAD"}), ""
	}
	if !reachable {
		return checkResult(RemovalCheckHeadOnMain, false, nil), ""
	}
	return checkResult(RemovalCheckHeadOnMain, true, nil), head
}

// checkUnlocked finds the worktree's registration among git's and checks
// that it is not locked; a worktree git does not register is unreadable.
func checkUnlocked(ctx context.Context, git GitReader, chiefTree, tree Worktree) CheckResult {
	entries, err := git.Worktrees(ctx, chiefTree.String())
	if err != nil {
		return checkResult(RemovalCheckUnlocked, false, []string{"the worktree list"})
	}
	i := slices.IndexFunc(entries, func(e WorktreeEntry) bool { return e.Path == tree.path || resolvedPath(e.Path) == tree.path })
	if i < 0 {
		return checkResult(RemovalCheckUnlocked, false, []string{"the registration of the worktree"})
	}
	return checkResult(RemovalCheckUnlocked, !entries[i].Locked, nil)
}

// checkAgentsIdle checks the workspace's agents as classify tells them
// apart: an agent that works or is blocked, or whose screen shows a
// question, is active; a screen that was not read or matches no known shape,
// and a pane Herdr lists with an agent that the agent list lacks, are
// unreadable.
func checkAgentsIdle(workspace string, agents []agentFacts, panes []listedPane) CheckResult {
	active := false
	var unreadable []string
	for _, a := range agents {
		pane := a.Agent.Pane.String()
		switch {
		case a.Agent.Status == AgentWorking || a.Agent.Status == AgentBlocked:
			active = true
		case a.Agent.Status != AgentIdle && a.Agent.Status != AgentDone:
			unreadable = append(unreadable, "the status of the agent of pane "+pane)
		case !a.ScreenRead:
			unreadable = append(unreadable, "the screen of pane "+pane)
		case a.Screen.Kind == ScreenQuestion:
			active = true
		case a.Screen.Kind != ScreenPrompt:
			unreadable = append(unreadable, "the screen of pane "+pane+", which matches no known shape")
		}
	}
	for _, p := range panes {
		if p.Workspace != workspace || !p.HasAgent {
			continue
		}
		if !slices.ContainsFunc(agents, func(a agentFacts) bool { return a.Agent.Pane == p.ID }) {
			unreadable = append(unreadable, "the agent of pane "+p.ID.String())
		}
	}
	return checkResult(RemovalCheckAgentsIdle, !active, unreadable)
}

// checkShellsIdle reads the processes of every pane of the workspace that
// has no agent and checks that its shell is alone in the foreground.
func checkShellsIdle(ctx context.Context, herdr Herdr, workspace string, panes []listedPane) CheckResult {
	busy := false
	var unreadable []string
	for _, p := range panes {
		if p.Workspace != workspace || p.HasAgent {
			continue
		}
		info, err := herdr.processes(ctx, p.ID)
		if err != nil {
			unreadable = append(unreadable, "the processes of pane "+p.ID.String())
			continue
		}
		if !info.ShellInForeground() {
			busy = true
		}
	}
	return checkResult(RemovalCheckShellsIdle, !busy, unreadable)
}

// checkPathUnused checks that the candidate's workspace is the only one with
// a pane at its path.
func checkPathUnused(c RemovalCandidate, panes []listedPane) CheckResult {
	at := workspacesAtPath(panes, c.tree)
	return checkResult(RemovalCheckPathUnused, len(at) == 1 && at[0] == c.workspace, nil)
}

// workspacePanes counts the panes Herdr lists in workspace.
func workspacePanes(workspace string, panes []listedPane) int {
	n := 0
	for _, p := range panes {
		if p.Workspace == workspace {
			n++
		}
	}
	return n
}

// addRemovals reads the removal candidates of the round's task workspaces,
// puts each one's proposal with its checks on its row, and makes the row of
// a candidate whose HEAD is detached and whose agents neither work nor ask
// show the task done with the removal proposed (Row.RemovalRow). It returns
// the problems of the reads that failed. Contract: a workspace that is not a
// candidate keeps its row as classify made it, and a round with no readable
// created record runs no command here.
func addRemovals(ctx context.Context, deps Deps, view *roundView) []string {
	r := &removalReader{cmds: deps.Cmds, store: deps.Store, home: deps.Env.Home, chiefTree: view.chiefTree}
	var problems []string
	for i, w := range view.found {
		c, problem := r.readRemovalCandidate(ctx, w)
		if problem != "" && !slices.Contains(problems, problem) {
			problems = append(problems, problem)
		}
		if c == nil {
			continue
		}
		f := view.facts[i]
		checks, _ := checkRemoval(ctx, deps.Cmds, view.chiefTree, *c, f.Agents, r.panes)
		row := &view.rows[i]
		row.Removal = &RemovalProposal{Workspace: w.Worktree, Label: w.Label, Issue: c.issue, Task: c.task, Panes: workspacePanes(w.ID, r.panes), Checks: checks}
		// The row follows the checks' own reading of HEAD and the agents, so
		// the row and its item never disagree.
		if f.Head == headDetached && passedCheck(checks, RemovalCheckDetached) && passedCheck(checks, RemovalCheckAgentsIdle) {
			showRemovalRow(row, *c)
		}
	}
	return problems
}

// showRemovalRow makes row the row of a finished task whose workspace is
// proposed for removal: the task is the issue's, nothing is offered to send,
// and it is not idle, since it waits on the human's answer. It runs after
// addResets saw the detached row, so a removal row offers no reset.
func showRemovalRow(row *Row, c RemovalCandidate) {
	row.State = StateTaskDone
	row.Task = c.task.String()
	row.Next, row.Takeover, row.Unreadable = nil, nil, nil
	row.Idle = false
	row.RemovalRow = true
}

// The time values of a removal (the plan, decision 6).
const (
	// removeLimit bounds a whole removal, shorter than the ten minutes the
	// /chief session gives the call, so the tool writes its output before it
	// is stopped.
	removeLimit = 3 * time.Minute
	// closeInterval and closeAttempts bound the wait for a closed
	// workspace's panes to leave Herdr's pane list (10 seconds).
	closeInterval = 500 * time.Millisecond
	closeAttempts = 20
)

// RemoveRefusal says why nothing was removed. The zero value is never
// treated as permission to remove.
type RemoveRefusal int

const (
	// RemoveRefusalUnknown is no refusal.
	RemoveRefusalUnknown RemoveRefusal = iota
	// RemoveRefusalNoWorkspace is a path that is no current task
	// workspace's worktree.
	RemoveRefusalNoWorkspace
	// RemoveRefusalFetchFailed is a failed fetch of origin/main.
	RemoveRefusalFetchFailed
	// RemoveRefusalBusy is a send, a creation, or a removal of the path
	// under way.
	RemoveRefusalBusy
	// RemoveRefusalChecksFailed is a check that did not pass; Checks says
	// which.
	RemoveRefusalChecksFailed
	// RemoveRefusalChanged is the workspace read again just before it was to
	// be closed differing from what the checks read.
	RemoveRefusalChanged
)

// removeRefusalNames are the output spellings of the refusals.
var removeRefusalNames = map[RemoveRefusal]string{
	RemoveRefusalNoWorkspace:  "no_workspace",
	RemoveRefusalFetchFailed:  "fetch_failed",
	RemoveRefusalBusy:         nameBusy,
	RemoveRefusalChecksFailed: "checks_failed",
	RemoveRefusalChanged:      "changed",
}

// String returns the output spelling, "" for no refusal.
func (r RemoveRefusal) String() string { return removeRefusalNames[r] }

// RemoveStage names the stage a removal stopped at. The zero value is no
// stage.
type RemoveStage int

const (
	// RemoveStageUnknown is no stage.
	RemoveStageUnknown RemoveStage = iota
	// RemoveStageCloseWorkspace closes the workspace.
	RemoveStageCloseWorkspace
	// RemoveStageWaitPanes waits for the workspace's panes to be gone.
	RemoveStageWaitPanes
	// RemoveStageRecheckHead checks the worktree's HEAD again.
	RemoveStageRecheckHead
	// RemoveStageRemoveWorktree removes the worktree.
	RemoveStageRemoveWorktree
	// RemoveStageRemoveRecords removes the worktree's records.
	RemoveStageRemoveRecords
	// RemoveStageDeadline is the time limit of the whole removal passing.
	RemoveStageDeadline
)

// removeStageNames are the output spellings of the stages.
var removeStageNames = map[RemoveStage]string{
	RemoveStageCloseWorkspace: "close_workspace",
	RemoveStageWaitPanes:      "wait_panes",
	RemoveStageRecheckHead:    "recheck_head",
	RemoveStageRemoveWorktree: "remove_worktree",
	RemoveStageRemoveRecords:  "remove_records",
	RemoveStageDeadline:       "deadline",
}

// String returns the output spelling, "" for no stage.
func (s RemoveStage) String() string { return removeStageNames[s] }

// RemoveResult is the kind of a removal's outcome; only the fields of that
// kind are set. The zero value is no result and is never shown as done.
type RemoveResult int

const (
	// RemoveResultUnknown is no result.
	RemoveResultUnknown RemoveResult = iota
	// RemoveRefused removed nothing: Refusal says why, and Checks the
	// checks when they ran.
	RemoveRefused
	// RemoveStopped stopped at Stage: Left lists what is left.
	RemoveStopped
	// RemoveDone closed the workspace and removed the worktree and its
	// records.
	RemoveDone
)

// removeResultNames are the output spellings of the results.
var removeResultNames = map[RemoveResult]string{
	RemoveRefused: "refused",
	RemoveStopped: nameStopped,
	RemoveDone:    "removed",
}

// String returns the output spelling, "" for no result.
func (r RemoveResult) String() string { return removeResultNames[r] }

// RemoveOutcome is what `chief remove` did.
type RemoveOutcome struct {
	Result RemoveResult
	// Workspace is the workspace's canonical worktree once it was found,
	// and the path given until then.
	Workspace string
	Label     string
	Refusal   RemoveRefusal
	Checks    []CheckResult
	Stage     RemoveStage
	// Left lists what a removal that stopped left: the workspace, the
	// worktree's directory, git's registration of it, and its records.
	Left    []Made
	Failure string
}

// errNotAsChecked reports a workspace or worktree that, read during its
// removal, is no longer as the checks read it or as closing it should leave
// it.
var errNotAsChecked = errors.New("the workspace is not as checked")

// errCloserRefused reports a WorkspaceCloser no constructor made.
var errCloserRefused = errors.New("the workspace closer refused")

// WorkspaceCloser closes one workspace and removes its worktree. Only
// newWorkspaceCloser makes one, from a RemovalCandidate that passed every
// removal check it ran itself, so no other workspace (chief's own, a
// hand-made one) can be closed and no other worktree removed. It never
// passes --group to herdr or --force to git. The zero value runs nothing:
// every method refuses before a call.
type WorkspaceCloser struct {
	cmds      Commands
	chiefTree Worktree
	tree      Worktree
	workspace string
	// head is the HEAD commit the checks found reachable from origin/main.
	head string
}

// newWorkspaceCloser runs every removal check on c and returns the checks,
// with the closer of c's workspace only when every check passed. agents and
// panes are as checkRemoval takes them.
func newWorkspaceCloser(ctx context.Context, cmds Commands, chiefTree Worktree, c RemovalCandidate, agents []agentFacts, panes []listedPane) (WorkspaceCloser, []CheckResult, bool) {
	checks, head := checkRemoval(ctx, cmds, chiefTree, c, agents, panes)
	if !allChecksPassed(checks) || chiefTree.path == "" || head == "" {
		return WorkspaceCloser{}, checks, false
	}
	return WorkspaceCloser{cmds: cmds, chiefTree: chiefTree, tree: c.tree, workspace: c.workspace, head: head}, checks, true
}

// allChecksPassed reports whether results hold every removal check, in
// order, each passed.
func allChecksPassed(results []CheckResult) bool {
	if len(results) != len(removalChecks) {
		return false
	}
	for i, r := range results {
		if r.Check != removalChecks[i] || !r.Passed {
			return false
		}
	}
	return true
}

// made reports a closer no constructor made, which runs nothing.
func (w WorkspaceCloser) made() error {
	if w.tree.path == "" || w.chiefTree.path == "" || w.workspace == "" || w.head == "" {
		return fmt.Errorf("%w: it was not made by newWorkspaceCloser", errCloserRefused)
	}
	return nil
}

// close closes the workspace with `herdr workspace close`, without --group.
func (w WorkspaceCloser) close(ctx context.Context) error {
	if err := w.made(); err != nil {
		return err
	}
	out, err := w.cmds.herdr(ctx, herdrWorkspace, "close", w.workspace)
	if err != nil {
		return err
	}
	return parseWorkspaceClose(out)
}

// removeWorktree removes the worktree with `git worktree remove`, run in
// chief's worktree and without --force, so git refuses a worktree with a
// change or an untracked file and never removes a branch.
func (w WorkspaceCloser) removeWorktree(ctx context.Context) error {
	if err := w.made(); err != nil {
		return err
	}
	_, err := w.cmds.git(ctx, w.chiefTree.String(), gitWorktree, "remove", w.tree.path)
	return err
}

// closeView is what the removal checks read of a workspace that closing it
// ends: its panes, and its agents with their states.
type closeView struct {
	panes  []string
	agents []string
}

// closeViewOf returns the view of workspace from Herdr's panes and agents.
func closeViewOf(workspace string, agents []Agent, panes []listedPane) closeView {
	var v closeView
	for _, p := range panes {
		if p.Workspace == workspace {
			v.panes = append(v.panes, p.ID.String())
		}
	}
	for _, a := range agents {
		if a.Workspace == workspace {
			v.agents = append(v.agents, fmt.Sprintf("%s %d %d", a.Pane, a.Status, a.StateChangeSeq))
		}
	}
	slices.Sort(v.panes)
	slices.Sort(v.agents)
	return v
}

// unchanged reads Herdr's panes and agents again and returns why closing
// the workspace now would act on something the checks did not see, or ""
// when nothing changed. Contract: a read that fails is a change; the
// workspace's panes and its agents' states must be as before, every pane of
// it without an agent must show its shell alone in the foreground, and it
// must still be the only workspace with a pane at the worktree.
func unchanged(ctx context.Context, herdr Herdr, c RemovalCandidate, before closeView) string {
	panes, err := herdr.panes(ctx)
	if err != nil {
		return "herdr pane list cannot be read again: " + err.Error()
	}
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return "herdr agent list cannot be read again: " + err.Error()
	}
	now := closeViewOf(c.workspace, agents, panes)
	if !slices.Equal(now.panes, before.panes) {
		return "the workspace's panes changed"
	}
	if !slices.Equal(now.agents, before.agents) {
		return "the state of an agent of the workspace changed"
	}
	if r := checkShellsIdle(ctx, herdr, c.workspace, panes); !r.Passed {
		return "a pane without an agent no longer shows its shell alone"
	}
	if r := checkPathUnused(c, panes); !r.Passed {
		return "another workspace has a pane at the worktree"
	}
	return ""
}

// waitPanesGone reads Herdr's pane list until no pane of workspace is in
// it. Contract: a read that fails stops the wait at once; panes still
// listed after closeAttempts reads fail it.
func waitPanesGone(ctx context.Context, herdr Herdr, sleep func(context.Context, time.Duration) error, workspace string) error {
	for attempt := range closeAttempts {
		if attempt > 0 {
			if err := sleep(ctx, closeInterval); err != nil {
				return err
			}
		}
		panes, err := herdr.panes(ctx)
		if err != nil {
			return err
		}
		if workspacePanes(workspace, panes) == 0 {
			return nil
		}
	}
	return fmt.Errorf("%w: the panes of workspace %s are still listed after it was closed", errNotAsChecked, workspace)
}

// recheckHead checks, just before the worktree is removed, that its HEAD is
// still head and still reachable from origin/main: a commit made after the
// checks would not stop git from removing a clean worktree.
func recheckHead(ctx context.Context, git GitReader, tree, head string) error {
	now, err := git.Head(ctx, tree)
	if err != nil {
		return err
	}
	if now != head {
		return fmt.Errorf("%w: the HEAD of the worktree moved from %s to %s", errNotAsChecked, head, now)
	}
	main, err := git.OriginMain(ctx, tree)
	if err != nil {
		return err
	}
	reachable, err := git.IsAncestor(ctx, tree, now, main)
	if err != nil {
		return err
	}
	if !reachable {
		return fmt.Errorf("%w: origin/main no longer reaches the HEAD %s", errNotAsChecked, now)
	}
	return nil
}

// Remove closes the workspace whose worktree is path and removes that
// worktree and its records (the design, 3.4.3). Contract: path is compared
// with the current task workspaces (locate), and git never runs in it
// otherwise; nothing is closed or removed unless the workspace's exclusion
// is held (until its records are gone), origin/main was fetched, the
// workspace is a removal candidate read again now, and every removal check
// passed; Herdr is closed and git removes only through the WorkspaceCloser
// of that candidate; the workspace is closed only when Herdr read again
// just before shows what the checks saw, and the worktree removed only once
// the workspace's panes are gone and its HEAD is unchanged and still
// reachable from origin/main; the created record goes last; a stage that
// fails stops there with what is left listed, and nothing is retried. An
// error is returned only before anything was closed or removed.
func Remove(ctx context.Context, deps Deps, path string) (RemoveOutcome, error) {
	ctx, cancel := context.WithTimeout(ctx, removeLimit)
	defer cancel()
	out := RemoveOutcome{Workspace: path}
	loc, err := locate(ctx, deps, path)
	if err != nil {
		return RemoveOutcome{}, err
	}
	if !loc.found {
		return refusedRemove(out, RemoveRefusalNoWorkspace, "the path is no current task workspace's worktree, or more than one Herdr workspace holds it"), nil
	}
	out.Workspace, out.Label = loc.ws.Worktree, loc.ws.Label
	records, err := deps.Store.Records(loc.ws.Tree)
	if err != nil {
		return RemoveOutcome{}, err
	}
	held, err := records.Lock()
	if errors.Is(err, ErrWorkspaceBusy) {
		return refusedRemove(out, RemoveRefusalBusy, "a send to the workspace or its removal is under way"), nil
	}
	if err != nil {
		return RemoveOutcome{}, err
	}
	defer func() { _ = held.Close() }()
	git := NewGitReader(deps.Cmds)
	if err := git.Fetch(ctx, loc.chiefTree.String()); err != nil {
		return refusedRemove(out, RemoveRefusalFetchFailed, err.Error()), nil
	}
	herdr := NewHerdr(deps.Cmds)
	// The agents are read under the exclusion, as a send reads them; a list
	// that cannot be read leaves every agent pane unaccounted for, which the
	// agents' check reads as unreadable.
	agents, _ := herdr.Agents(ctx)
	wsAgents := slices.DeleteFunc(slices.Clone(agents), func(a Agent) bool { return a.Workspace != loc.ws.ID })
	reader := &removalReader{cmds: deps.Cmds, store: deps.Store, home: deps.Env.Home, chiefTree: loc.chiefTree}
	c, problem := reader.readRemovalCandidate(ctx, loc.ws)
	if c == nil {
		var unreadable []string
		if problem != "" {
			unreadable = []string{problem}
		}
		// The other checks need a candidate, so a workspace that is no
		// longer one carries the candidate check alone.
		out.Checks = []CheckResult{checkResult(RemovalCheckCandidate, false, unreadable)}
		return refusedRemove(out, RemoveRefusalChecksFailed, "the workspace is no longer a removal candidate: its created record, its path in Herdr, or its issue's one finished task on origin/main does not hold"), nil
	}
	closer, checks, ok := newWorkspaceCloser(ctx, deps.Cmds, loc.chiefTree, *c, readAgentFacts(ctx, herdr, wsAgents), reader.panes)
	out.Checks = checks
	if !ok {
		return refusedRemove(out, RemoveRefusalChecksFailed, ""), nil
	}
	tree := c.tree.String()
	if why := unchanged(ctx, herdr, *c, closeViewOf(c.workspace, agents, reader.panes)); why != "" {
		return refusedRemove(out, RemoveRefusalChanged, why), nil
	}
	stop := func(stage RemoveStage, err error) RemoveOutcome {
		out.Result, out.Stage, out.Failure = RemoveStopped, stage, err.Error()
		if ctx.Err() != nil {
			out.Stage, out.Failure = RemoveStageDeadline, fmt.Sprintf("the time limit passed at stage %s: %v", stage, err)
		}
		out.Left = leftAfter(deps, held, loc.chiefTree, *c, out.Label, stage)
		return out
	}
	for _, step := range []struct {
		stage RemoveStage
		run   func() error
	}{
		{RemoveStageCloseWorkspace, func() error { return closer.close(ctx) }},
		{RemoveStageWaitPanes, func() error { return waitPanesGone(ctx, herdr, deps.Sleep, c.workspace) }},
		{RemoveStageRecheckHead, func() error { return recheckHead(ctx, git, tree, closer.head) }},
		{RemoveStageRemoveWorktree, func() error { return closer.removeWorktree(ctx) }},
		{RemoveStageRemoveRecords, held.ClearRecords},
	} {
		if err := ctx.Err(); err != nil {
			return stop(step.stage, err), nil
		}
		if err := step.run(); err != nil {
			return stop(step.stage, err), nil
		}
	}
	out.Result = RemoveDone
	return out, nil
}

// refusedRemove is out refused for refusal.
func refusedRemove(out RemoveOutcome, refusal RemoveRefusal, failure string) RemoveOutcome {
	out.Result, out.Refusal, out.Failure = RemoveRefused, refusal, failure
	return out
}

// leftAfter lists what a removal that stopped at stage left: the workspace
// while Herdr still lists a pane of it (it can be left only up to the wait
// for its panes), the worktree's directory if
// it is there, git's registration of the worktree unless git's list shows
// it gone, and each record still in the worktree's directory. Contract: what
// cannot be read is listed as left, so the human is never told something is
// gone that may not be; it reads with a context of its own, since the
// removal's may have run out.
func leftAfter(deps Deps, held *HeldWorkspace, chiefTree Worktree, c RemovalCandidate, label string, stage RemoveStage) []Made {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	var left []Made
	if stage <= RemoveStageWaitPanes && workspaceStillListed(ctx, NewHerdr(deps.Cmds), c.workspace) {
		left = append(left, Made{Kind: MadeWorkspace, ID: c.workspace, Label: label})
	}
	if _, err := os.Lstat(c.tree.path); !errors.Is(err, os.ErrNotExist) {
		left = append(left, Made{Kind: MadeWorktree, ID: c.tree.path})
	}
	entries, err := NewGitReader(deps.Cmds).Worktrees(ctx, chiefTree.String())
	if err != nil || slices.ContainsFunc(entries, func(e WorktreeEntry) bool { return e.Path == c.tree.path || resolvedPath(e.Path) == c.tree.path }) {
		left = append(left, Made{Kind: MadeRegistration, ID: c.tree.path})
	}
	names, err := os.ReadDir(held.dir)
	if err != nil {
		return append(left, Made{Kind: MadeRecord, ID: held.dir})
	}
	for _, e := range names {
		if isRecordFile(e.Name()) {
			left = append(left, Made{Kind: MadeRecord, ID: filepath.Join(held.dir, e.Name())})
		}
	}
	return left
}

// workspaceStillListed reports whether Herdr's pane list has a pane of
// workspace; a list that cannot be read counts as listed.
func workspaceStillListed(ctx context.Context, herdr Herdr, workspace string) bool {
	panes, err := herdr.panes(ctx)
	return err != nil || workspacePanes(workspace, panes) > 0
}
