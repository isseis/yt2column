package chief

import (
	"context"
	"errors"
	"slices"

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
	label     string
	issue     paneinput.IssueNumber
	task      paneinput.TaskID
}

// newRemovalCandidate is the one constructor of RemovalCandidate; its
// caller has read every condition.
func newRemovalCandidate(tree Worktree, workspace, label string, issue paneinput.IssueNumber, task paneinput.TaskID) RemovalCandidate {
	return RemovalCandidate{tree: tree, workspace: workspace, label: label, issue: issue, task: task}
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
		// A directory that cannot be read is the problem addRecords reports.
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
	if err != nil || len(dirs) != 1 {
		return nil, ""
	}
	docs, err := NewTasks(r.cmds).ReadDir(ctx, r.chiefTree.String(), dirs[0])
	if err != nil {
		return nil, "the documents of " + dirs[0] + " on origin/main cannot be read, so " + w.Worktree + " is not offered for removal: " + err.Error()
	}
	if stageOf(&docs) != stagePlanApproved || !docs.Plan.Done() {
		return nil, ""
	}
	c := newRemovalCandidate(w.Tree, w.ID, w.Label, rec.Issue, docs.Task)
	return &c, ""
}

// checkRemoval runs every check of a removal on c and returns one result
// per check, in the order of removalChecks. agents are the workspace's
// agents with their screens and panes every pane Herdr lists, both read by
// the caller. Contract: the zero candidate passes no check and runs nothing;
// a read that fails makes its check not passed with what it could not read
// named, never passed.
func checkRemoval(ctx context.Context, cmds Commands, chiefTree Worktree, c RemovalCandidate, agents []agentFacts, panes []listedPane) []CheckResult {
	if c.tree.path == "" {
		out := make([]CheckResult, 0, len(removalChecks))
		for _, check := range removalChecks {
			out = append(out, checkResult(check, false, []string{errNoCandidate.Error()}))
		}
		return out
	}
	git := NewGitReader(cmds)
	tree := c.tree.String()
	return []CheckResult{
		checkResult(RemovalCheckCandidate, true, nil),
		checkDetached(ctx, git, tree),
		checkClean(ctx, git, tree),
		checkHeadOnMain(ctx, git, tree),
		checkUnlocked(ctx, git, chiefTree, c.tree),
		checkAgentsIdle(c.workspace, agents, panes),
		checkShellsIdle(ctx, NewHerdr(cmds), c.workspace, panes),
		checkPathUnused(c, panes),
	}
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

func checkHeadOnMain(ctx context.Context, git GitReader, tree string) CheckResult {
	head, err := git.Head(ctx, tree)
	if err != nil {
		return checkResult(RemovalCheckHeadOnMain, false, []string{"the HEAD commit of the worktree"})
	}
	main, err := git.OriginMain(ctx, tree)
	if err != nil {
		return checkResult(RemovalCheckHeadOnMain, false, []string{"origin/main"})
	}
	reachable, err := git.IsAncestor(ctx, tree, head, main)
	if err != nil {
		return checkResult(RemovalCheckHeadOnMain, false, []string{"whether origin/main reaches the HEAD"})
	}
	return checkResult(RemovalCheckHeadOnMain, reachable, nil)
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
		checks := checkRemoval(ctx, deps.Cmds, view.chiefTree, *c, f.Agents, r.panes)
		row := &view.rows[i]
		row.Removal = &RemovalProposal{Workspace: w.Worktree, Label: w.Label, Issue: c.issue, Task: c.task, Panes: workspacePanes(w.ID, r.panes), Checks: checks}
		if f.Head == headDetached && passedCheck(checks, RemovalCheckAgentsIdle) {
			showRemovalRow(row, *c)
		}
	}
	return problems
}

// showRemovalRow makes row the row of a finished task whose workspace is
// proposed for removal: the task is the issue's, nothing is offered to send,
// and it is not idle, since it waits on the human's answer.
func showRemovalRow(row *Row, c RemovalCandidate) {
	row.State = StateTaskDone
	row.Task = c.task.String()
	row.Next, row.Takeover, row.Unreadable = nil, nil, nil
	row.Idle = false
	row.RemovalRow = true
}
