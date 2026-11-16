package chief

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Deps is what a subcommand runs with. The entry point builds it from the
// environment and the real runners; a test builds it from a state directory
// under t.TempDir() and a recorder.
type Deps struct {
	Env   Env
	Cmds  Commands
	Store Store
	Now   func() time.Time
	// Sleep waits between the reads after an input; it returns early with
	// the context's error. Tests pass one that does not wait.
	Sleep func(context.Context, time.Duration) error
}

// SleepContext waits for d or until ctx is done, the Sleep of the entry
// point.
func SleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Snapshot is what `chief snapshot` returns: the rows of the status table,
// the items waiting on the human in the order to show them, and the problems
// that concern the whole round.
type Snapshot struct {
	StateDir string
	Rows     []Row
	Pending  []PendingItem
	// Problems are failures that are not one workspace's: the routing table
	// or the old-value table cannot be read, `git fetch` failed, a notice
	// record could not be made.
	Problems []string
	// Baseline is what `chief wait` compares against: the agents of the
	// task workspaces as this round read them.
	Baseline WaitBaseline
}

// roundView is one reading of every task workspace: chief's own pane and
// worktree, the agents, the task workspaces with the facts read of each and
// the row classified from them, and what concerns the whole round.
type roundView struct {
	chiefPane PaneID
	chief     Agent
	chiefTree Worktree
	agents    []Agent
	found     []TaskWorkspace
	// facts and rows are parallel to found.
	facts       []workspaceFacts
	rows        []Row
	routing     Routing
	routingErr  error
	old         OldValues
	oldErr      error
	fetchFailed bool
	problems    []string
}

// TakeSnapshot reads every task workspace and classifies it. Contract: it
// sends nothing and creates no record but the notice records of items it
// found; it never runs a command that writes, and a failed read makes its
// workspace StateNeedsCheck instead of failing the round, unless Herdr's own
// list or chief's worktree cannot be read, which fails the call.
func TakeSnapshot(ctx context.Context, deps Deps, owner Owner) (Snapshot, error) {
	view, err := readRound(ctx, deps)
	if err != nil {
		return Snapshot{}, err
	}
	return finishSnapshot(deps, owner, view, nil), nil
}

// readRound reads Herdr, fetches origin/main once, reads the routing and
// old-value tables, and reads and classifies every task workspace. It
// creates no record.
func readRound(ctx context.Context, deps Deps) (roundView, error) {
	herdr := NewHerdr(deps.Cmds)
	git := NewGitReader(deps.Cmds)
	tasks := NewTasks(deps.Cmds)
	chiefPane, err := ParsePaneID(deps.Env.PaneID)
	if err != nil {
		return roundView{}, err
	}
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return roundView{}, err
	}
	workspaces, err := herdr.Workspaces(ctx)
	if err != nil {
		return roundView{}, err
	}
	chief, err := chiefWorkspaceOf(agents, chiefPane)
	if err != nil {
		return roundView{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	chiefTree, err := CanonicalWorktree(ctx, deps.Cmds, chief.Dir)
	if err != nil {
		return roundView{}, fmt.Errorf("read the chief worktree: %w", err)
	}
	view := roundView{chiefPane: chiefPane, chief: chief, chiefTree: chiefTree, agents: agents}
	if err := git.Fetch(ctx, chiefTree.String()); err != nil {
		view.fetchFailed = true
		view.problems = append(view.problems, "git fetch origin main failed: "+err.Error())
	}
	view.routing, view.routingErr = ReadRouting(ctx, deps.Cmds, chiefTree.String())
	if view.routingErr != nil {
		view.problems = append(view.problems, "the routing table cannot be read: "+view.routingErr.Error())
	}
	view.old, view.oldErr = readOldValues(ctx, tasks, chiefTree.String())
	if view.oldErr != nil {
		view.problems = append(view.problems, "the old-value table cannot be read: "+view.oldErr.Error())
	}
	view.found, err = FindTaskWorkspaces(ctx, git, deps.Cmds, agents, workspaces, chiefPane)
	if err != nil {
		return roundView{}, err
	}
	for _, w := range view.found {
		f := readWorkspace(ctx, herdr, git, tasks, w)
		f.Routing, f.RoutingErr, f.FetchFailed = view.routing, view.routingErr, view.fetchFailed
		row := classifyWorkspace(ctx, git, tasks, f, view.old, view.oldErr)
		if err := addRecords(ctx, git, deps.Store, w, &row); err != nil {
			view.problems = append(view.problems, "the records of "+w.Worktree+" cannot be read: "+err.Error())
		}
		view.facts = append(view.facts, f)
		view.rows = append(view.rows, row)
	}
	return view, nil
}

// finishSnapshot lists the pending items of the round's rows with extra
// (proposals a tick made, merged into the row's item of the same identity),
// stamps them with the notice records of owner's session, and orders the
// items and the rows.
func finishSnapshot(deps Deps, owner Owner, view roundView, extra []PendingItem) Snapshot {
	snap := Snapshot{StateDir: deps.Store.Dir(), Rows: view.rows, Problems: view.problems, Baseline: baselineOf(view.agents, workspaceIDs(view.found))}
	snap.Pending = mergePending(buildPending(snap.Rows), extra)
	idle := idleWorkspaces(snap.Rows)
	// Sorting before stamping gives the items first seen in this round
	// their numbers in the order they are listed.
	sortPending(snap.Pending, idle)
	for _, err := range stampPending(deps.Store, owner.Session, snap.Pending, deps.Now()) {
		snap.Problems = append(snap.Problems, "a notice record failed: "+err.Error())
	}
	sortPending(snap.Pending, idle)
	sortRows(snap.Rows, snap.Pending)
	return snap
}

// addRecords shows a workspace's stop, on a running row what chief last
// sent to its working pane, and on a row awaiting review the review-skippable
// judgment of the PR detail kept for the worktree's HEAD. It reads records
// only; a workspace never sent to has no records and nothing is created for
// it. Contract: the judgment is shown only from a detail of the same PR at
// the same HEAD, never from an older one.
func addRecords(ctx context.Context, git GitReader, store Store, w TaskWorkspace, row *Row) error {
	records, ok, err := store.existingRecords(w.Tree)
	if err != nil || !ok {
		return err
	}
	stop, err := records.Stopped()
	if err != nil {
		return err
	}
	row.Stopped = stop
	if stop != nil {
		// A stopped workspace waits on the human's release.
		row.Idle = false
	}
	if row.State == StateAwaitingReview && row.PR != nil {
		// A HEAD that cannot be read only leaves the mark off.
		if head, err := git.Head(ctx, w.Worktree); err == nil {
			rec, ok, err := records.readDetail(row.PR.Number, head)
			if err != nil {
				return err
			}
			row.ReviewSkippable = ok && rec.Skippable
		}
	}
	if row.State != StateRunning {
		return nil
	}
	for _, a := range w.Agents {
		if a.Status != AgentWorking {
			continue
		}
		rec, err := records.LastSend(a.Pane.String())
		if err != nil {
			return err
		}
		if rec != nil {
			row.Running = rec
			return nil
		}
	}
	return nil
}

// readOldValues reads the old-value table of origin/main's _context.md.
func readOldValues(ctx context.Context, tasks Tasks, tree string) (OldValues, error) {
	doc, err := tasks.show(ctx, tree, originMain, contextPath)
	if err != nil {
		return nil, err
	}
	return ParseOldValues(doc), nil
}

// readWorkspace reads one workspace's facts: its agents and screens, its
// branch, its open PR, and the task documents. A failed read is named in
// Unreadable, never defaulted.
func readWorkspace(ctx context.Context, herdr Herdr, git GitReader, tasks Tasks, w TaskWorkspace) workspaceFacts {
	f := workspaceFacts{Workspace: w.Worktree, Label: w.Label}
	for _, a := range w.Agents {
		af := agentFacts{Agent: a}
		raw, err := herdr.Screen(ctx, a.Pane, ScreenVisible)
		if err == nil {
			af.ScreenRead = true
			af.Screen = readScreen(a.Kind, raw)
		}
		f.Agents = append(f.Agents, af)
	}
	branch, err := git.Branch(ctx, w.Worktree)
	if err != nil {
		f.Unreadable = append(f.Unreadable, "branch of the worktree")
		return f
	}
	if branch == "" {
		f.Head = headDetached
		return f
	}
	f.Head, f.Branch = headBranch, branch
	prs, err := git.OpenPRs(ctx, w.Worktree, branch)
	switch {
	case err != nil:
		f.Unreadable = append(f.Unreadable, "open pull requests of the branch")
	case len(prs) > 1:
		f.Unreadable = append(f.Unreadable, "more than one open pull request on the branch")
	case len(prs) == 1:
		f.PR = &prs[0]
	}
	if task, ok := taskOfBranch(branch); ok {
		f.Task = task
		docs, err := tasks.Read(ctx, w.Worktree, branch)
		if err != nil {
			f.Unreadable = append(f.Unreadable, "task documents on origin/main")
		} else {
			f.Docs = &docs
		}
	}
	return f
}

// classifyWorkspace reads what the takeover proposal needs when its other
// conditions hold, then classifies. The reads wait for those conditions so
// the common workspace costs no extra command.
func classifyWorkspace(ctx context.Context, git GitReader, tasks Tasks, f workspaceFacts, old OldValues, oldErr error) Row {
	if next, _, _ := decideNext(f); needsTakeoverCheck(f, next) && f.Docs != nil && f.Docs.Plan != nil {
		ahead, err := git.Ahead(ctx, f.Workspace)
		pr, ok := f.Docs.Plan.NextPR()
		switch {
		case err != nil:
			f.Unreadable = append(f.Unreadable, "commits ahead of origin/main")
		case !ok:
			f.Unreadable = append(f.Unreadable, "the next PR of the plan")
		case ahead > 0 && oldErr != nil:
			f.Unreadable = append(f.Unreadable, "the old-value table of origin/main")
		case ahead > 0:
			plan, err := tasks.BranchPlan(ctx, f.Workspace, *f.Docs, old)
			if err != nil {
				f.Unreadable = append(f.Unreadable, "the plan on the branch")
				break
			}
			done, err := plan.StepsDone(pr.Target)
			if err != nil {
				f.Unreadable = append(f.Unreadable, "the steps of the next PR on the branch")
				break
			}
			f.Takeover = takeoverFacts{Read: true, Ahead: ahead, StepsDone: done}
		default:
			f.Takeover = takeoverFacts{Read: true}
		}
	}
	return classify(f)
}

// mergeOnlyRank is 1 for a row whose questions are all merge confirmations,
// which list after the rows with another question (the order of pending
// items), and 0 otherwise.
func mergeOnlyRank(r Row) int {
	if r.State == StateQuestion && r.MergeQuestion && !slices.ContainsFunc(r.Questions, func(q QuestionItem) bool { return !q.Question.Merge }) {
		return 1
	}
	return 0
}

// idleRank is 1 for an idle row (Row.Idle), which lists after every other
// row, and 0 otherwise.
func idleRank(r Row) int {
	if r.Idle {
		return 1
	}
	return 0
}

// idleWorkspaces returns the workspaces of the idle rows (Row.Idle).
func idleWorkspaces(rows []Row) map[string]struct{} {
	idle := map[string]struct{}{}
	for _, r := range rows {
		if r.Idle {
			idle[r.Workspace] = struct{}{}
		}
	}
	return idle
}

// sortRows orders the rows as the table is shown: idle rows last, then by
// state, then, for the states that wait on the human, by how long their
// first item has waited, then by workspace path. pending must already carry
// its times.
func sortRows(rows []Row, pending []PendingItem) {
	first := map[string]time.Time{}
	for _, p := range pending {
		if t, ok := first[p.Workspace]; !ok || p.FirstSeen.Before(t) {
			first[p.Workspace] = p.FirstSeen
		}
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		return cmp.Or(
			cmp.Compare(idleRank(a), idleRank(b)),
			cmp.Compare(a.State.rank(), b.State.rank()),
			cmp.Compare(mergeOnlyRank(a), mergeOnlyRank(b)),
			first[a.Workspace].Compare(first[b.Workspace]),
			strings.Compare(a.Workspace, b.Workspace),
		)
	})
}
