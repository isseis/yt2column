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
}

// TakeSnapshot reads every task workspace and classifies it. Contract: it
// sends nothing and creates no record but the notice records of items it
// found; it never runs a command that writes, and a failed read makes its
// workspace StateNeedsCheck instead of failing the round, unless Herdr's own
// list or chief's worktree cannot be read, which fails the call.
func TakeSnapshot(ctx context.Context, deps Deps, owner Owner) (Snapshot, error) {
	herdr := NewHerdr(deps.Cmds)
	git := NewGitReader(deps.Cmds)
	tasks := NewTasks(deps.Cmds)
	chiefPane, err := ParsePaneID(deps.Env.PaneID)
	if err != nil {
		return Snapshot{}, err
	}
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	workspaces, err := herdr.Workspaces(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	chiefIdx := slices.IndexFunc(agents, func(a Agent) bool { return a.Pane == chiefPane })
	if chiefIdx < 0 {
		return Snapshot{}, fmt.Errorf("%w: the chief pane %s is not among Herdr's agents", errHerdrOutput, chiefPane)
	}
	chiefTree, err := CanonicalWorktree(ctx, deps.Cmds, agents[chiefIdx].Dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read the chief worktree: %w", err)
	}
	snap := Snapshot{StateDir: deps.Store.Dir()}
	fetchFailed := false
	if err := git.Fetch(ctx, chiefTree.String()); err != nil {
		fetchFailed = true
		snap.Problems = append(snap.Problems, "git fetch origin main failed: "+err.Error())
	}
	routing, routingErr := ReadRouting(ctx, deps.Cmds, chiefTree.String())
	if routingErr != nil {
		snap.Problems = append(snap.Problems, "the routing table cannot be read: "+routingErr.Error())
	}
	old, oldErr := readOldValues(ctx, tasks, chiefTree.String())
	if oldErr != nil {
		snap.Problems = append(snap.Problems, "the old-value table cannot be read: "+oldErr.Error())
	}
	found, err := FindTaskWorkspaces(ctx, git, deps.Cmds, agents, workspaces, chiefPane)
	if err != nil {
		return Snapshot{}, err
	}
	for _, w := range found {
		f := readWorkspace(ctx, herdr, git, tasks, w)
		f.Routing, f.RoutingErr, f.FetchFailed = routing, routingErr, fetchFailed
		snap.Rows = append(snap.Rows, classifyWorkspace(ctx, git, tasks, f, old, oldErr))
	}
	snap.Pending = buildPending(snap.Rows)
	for _, err := range stampPending(deps.Store, owner.Session, snap.Pending, deps.Now()) {
		snap.Problems = append(snap.Problems, "a notice record failed: "+err.Error())
	}
	sortPending(snap.Pending)
	sortRows(snap.Rows, snap.Pending)
	return snap, nil
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
	if branch != "" {
		prs, err := git.OpenPRs(ctx, w.Worktree, branch)
		switch {
		case err != nil:
			f.Unreadable = append(f.Unreadable, "open pull requests of the branch")
		case len(prs) > 1:
			f.Unreadable = append(f.Unreadable, "more than one open pull request on the branch")
		case len(prs) == 1:
			f.PR = &prs[0]
		}
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

// sortRows orders the rows as the table is shown: by state, then, for the
// states that wait on the human, by how long their first item has waited,
// then by workspace path. pending must already carry its times.
func sortRows(rows []Row, pending []PendingItem) {
	first := map[string]time.Time{}
	for _, p := range pending {
		if t, ok := first[p.Workspace]; !ok || p.FirstSeen.Before(t) {
			first[p.Workspace] = p.FirstSeen
		}
	}
	slices.SortStableFunc(rows, func(a, b Row) int {
		return cmp.Or(
			cmp.Compare(a.State.rank(), b.State.rank()),
			cmp.Compare(mergeOnlyRank(a), mergeOnlyRank(b)),
			first[a.Workspace].Compare(first[b.Workspace]),
			strings.Compare(a.Workspace, b.Workspace),
		)
	})
}
