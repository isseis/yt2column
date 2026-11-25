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
	// Table is the status table laid out from Rows, one line per element,
	// each already sanitized.
	Table []string
	// TableUnreadable names, in English and sanitized, the table sources
	// that could not be read and the question panes shown by their pane IDs.
	TableUnreadable []string
}

// roundView is one reading of every task workspace: chief's own pane and
// worktree, the agents, the task workspaces with the facts read of each and
// the row classified from them, and what concerns the whole round.
type roundView struct {
	chiefPane PaneID
	chief     Agent
	chiefTree Worktree
	agents    []Agent
	// paneTabs maps a pane ID to its tab, read from the same agent list as
	// agents, for the table only; nil when paneTabsErr is set.
	paneTabs    map[string]tabID
	paneTabsErr error
	found       []TaskWorkspace
	// facts and rows are parallel to found.
	facts       []workspaceFacts
	rows        []Row
	routing     Routing
	routingErr  error
	old         OldValues
	oldErr      error
	fetchFailed bool
	problems    []string
	// prStates are the states of PRs read this round outside any
	// exclusion, by number (prState); nil until the first is read.
	prStates map[int]prStateRead
}

// prState returns PR number's state as this round read it, reading it in
// dir and keeping it when the round has not. Contract: outside an
// exclusion a round reads a PR's state at most once, a failed read
// included, so the history and the automatic send's first reading agree;
// a read under an exclusion does not come here.
func (v *roundView) prState(ctx context.Context, git GitReader, dir string, number int) prStateRead {
	if read, ok := v.prStates[number]; ok {
		return read
	}
	state, head, err := git.PRStateOf(ctx, dir, number)
	read := prStateRead{State: state, HeadRef: head, Err: err}
	if v.prStates == nil {
		v.prStates = map[int]prStateRead{}
	}
	v.prStates[number] = read
	return read
}

// TakeSnapshot reads every task workspace and classifies it. Contract: it
// sends nothing and creates no record but the notice records of items it
// found; it never runs a command that writes, and a failed read makes its
// workspace StateNeedsCheck instead of failing the round, unless Herdr's own
// list or chief's worktree cannot be read, which fails the call.
func TakeSnapshot(ctx context.Context, deps Deps, owner Owner) (Snapshot, error) {
	view, err := readFacts(ctx, deps)
	if err != nil {
		return Snapshot{}, err
	}
	buildRows(ctx, deps, &view, deps.Store.readTraces(owner.Session), nil)
	return finishSnapshot(deps, owner, view, readTableSources(ctx, deps, view), nil), nil
}

// tableNoteMaxBytes caps the error text of a table source that could not be
// read: a command's error can carry Herdr's standard error, up to the
// output cap, and the note is printed every round.
const tableNoteMaxBytes = 512

// readTableSources reads the tab labels and the chief pane's width for the
// status table, and adds the pane-to-tab map readFacts read. Contract: the
// sources are used for the table only; a source that cannot be read is left
// at its zero value with a note, capped and sanitized, and never fails the
// round, adds a problem, or changes a row.
func readTableSources(ctx context.Context, deps Deps, view roundView) tableSources {
	herdr := NewHerdr(deps.Cmds)
	var src tableSources
	if view.paneTabsErr != nil {
		src.notes = append(src.notes, tableNote("the panes' tabs of the agent list", view.paneTabsErr))
	} else {
		src.paneTabs = view.paneTabs
	}
	labels, err := herdr.tabLabels(ctx)
	if err != nil {
		src.notes = append(src.notes, tableNote("the tab list", err))
	} else {
		src.tabLabels = labels
	}
	width, split, err := herdr.paneWidth(ctx, view.chiefPane)
	if err != nil {
		src.notes = append(src.notes, tableNote("the chief pane's layout", err))
	} else {
		src.paneWidth, src.paneSplit = width, split
	}
	return src
}

// tableNote says that a table source could not be read and why, with the
// error text cut at tableNoteMaxBytes (the cut marked) and then sanitized.
func tableNote(source string, err error) string {
	capped := CapHead(err.Error(), tableNoteMaxBytes)
	text := capped.Text
	if capped.Truncated {
		text += " [truncated]"
	}
	return source + " cannot be read: " + Sanitize(text)
}

// readFacts reads Herdr, fetches origin/main once, reads the routing and
// old-value tables, and reads the facts of every task workspace; buildRows
// makes the rows from them. It creates no record.
func readFacts(ctx context.Context, deps Deps) (roundView, error) {
	herdr := NewHerdr(deps.Cmds)
	git := NewGitReader(deps.Cmds)
	tasks := NewTasks(deps.Cmds)
	chiefPane, err := ParsePaneID(deps.Env.PaneID)
	if err != nil {
		return roundView{}, err
	}
	listing, err := herdr.agentListing(ctx)
	if err != nil {
		return roundView{}, err
	}
	agents := listing.agents
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
	view := roundView{chiefPane: chiefPane, chief: chief, chiefTree: chiefTree, agents: agents, paneTabs: listing.paneTabs, paneTabsErr: listing.paneTabsErr}
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
		view.facts = append(view.facts, f)
	}
	return view, nil
}

// buildRows makes the rows of the workspaces readFacts read: it identifies
// the task of each worktree on main from history and applies it
// (applyMainTask), classifies every workspace, and puts on the rows their
// records, resets, and removal proposals. view.facts becomes the facts the
// rows were classified from. Contract: it writes no record and runs no gh
// command; besides the reads of classifyWorkspace, addRecords, and
// addRemovals, it reads only the documents of an identified task; history
// is the history of the caller's session, or an empty one for a caller that
// reads none, which identifies no task; unread are the PRs whose state this
// round could not read (nil when it read none), and a worktree on main whose
// history has one of them open is marked MainStateUnread.
func buildRows(ctx context.Context, deps Deps, view *roundView, history traceHistory, unread map[int]struct{}) {
	git := NewGitReader(deps.Cmds)
	tasks := NewTasks(deps.Cmds)
	for i, w := range view.found {
		f := view.facts[i]
		if f.Head == headMain {
			f.MainStateUnread = slices.ContainsFunc(applyTrace(history.events[w.Tree]).openPRs(), func(pr int) bool {
				_, bad := unread[pr]
				return bad
			})
		}
		f = applyMainTask(ctx, tasks, f, identifyMainTask(history, w.Tree))
		row := classifyWorkspace(ctx, git, tasks, f, view.old, view.oldErr)
		if err := addRecords(ctx, deps, git, w, &row); err != nil {
			view.problems = append(view.problems, "the records of "+w.Worktree+" cannot be read: "+err.Error())
		} else {
			view.problems = append(view.problems, addResets(deps.Store, w, &row, view.routing)...)
		}
		view.facts[i] = f
		view.rows = append(view.rows, row)
	}
	view.problems = append(view.problems, addRemovals(ctx, deps, view)...)
}

// finishSnapshot sets each row's urgency, turn, and position (finishRow),
// lists the pending items of the round's rows with extra (proposals a tick
// made, merged into the row's item of the same identity), stamps them with
// the notice records of owner's session, orders the items and the rows, and
// lays out the status table from the ordered rows and src.
func finishSnapshot(deps Deps, owner Owner, view roundView, src tableSources, extra []PendingItem) Snapshot {
	snap := Snapshot{StateDir: deps.Store.Dir(), Rows: view.rows, Problems: view.problems, Baseline: baselineOf(view.agents, workspaceIDs(view.found))}
	for i := range snap.Rows {
		finishRow(&snap.Rows[i])
	}
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
	// The table is laid out from the rows as they are output, so it never
	// disagrees with them.
	snap.Table, snap.TableUnreadable = layoutTable(snap.Rows, src)
	return snap
}

// addRecords shows a workspace's stop, on a running row the command its
// working agents run, on a row awaiting review the review-skippable
// judgment of the PR detail kept for the worktree's HEAD, and on a conflict
// row the phase of its resolution. It reads records
// only; a workspace never sent to has no records and nothing is created for
// it. Contract: the judgment is shown only from a detail of the same PR at
// the same HEAD, never from an older one; a stop that cannot be read marks
// the row StopUnreadable and is returned, so the round names it as a
// problem; the running command is read for display only, and a send record
// that cannot be read makes it undecided (RunningUnknownUnreadable) without
// being returned, so it never adds a problem; a conflict row whose records
// directory or send records cannot be read keeps ConflictRecordsUnreadable,
// the phase that never sends, and any other conflict row takes its phase from
// conflictPhaseOf (a stop record that cannot be read is shown by
// StopUnreadable, not by the phase).
func addRecords(ctx context.Context, deps Deps, git GitReader, w TaskWorkspace, row *Row) error {
	var stopErr error
	sends := func(string) (lastCommand, error) { return lastCommand{}, nil }
	records, ok, dirErr := deps.Store.existingRecords(w.Tree)
	switch {
	case dirErr != nil:
		// Whether the workspace is stopped cannot be told, so it is not idle
		// and calls the human like a stopped one.
		row.Idle, row.StopUnreadable, stopErr = false, true, dirErr
		sends = func(string) (lastCommand, error) { return lastCommand{}, dirErr }
	case ok:
		sends = records.lastSentCommand
		stop, err := records.Stopped()
		switch {
		case err != nil:
			row.Idle, row.StopUnreadable, stopErr = false, true, err
		case stop != nil:
			// A stopped workspace waits on the human's release.
			row.Stopped, row.Idle = stop, false
		}
		if stopErr == nil && row.State == StateAwaitingReview && row.PR != nil {
			// A HEAD that cannot be read only leaves the mark off.
			if head, err := git.Head(ctx, w.Worktree); err == nil {
				rec, found, err := records.readDetail(row.PR.Number, head)
				if err != nil {
					return err
				}
				row.ReviewSkippable = found && rec.Skippable
			}
		}
	}
	if row.State == StateRunning {
		row.Running, row.RunningUnknown = runningOf(ctx, deps, w.Agents, sends)
	}
	if row.State == StateConflict && dirErr == nil {
		if !ok {
			row.Conflict = ConflictUnsent
		} else if row.PR != nil {
			row.Conflict = conflictPhaseOf(records, *row.PR)
		}
	}
	return stopErr
}

// addResets offers on the row the resets its skip records name, as
// resetItemOf allows, and returns the problems of the records it could not
// read. Contract: it reads only, so a record is never created or removed by
// a round; a row that needs a check or is stopped gets no item; a record
// that cannot be read is named with its path and never offered.
func addResets(store Store, w TaskWorkspace, row *Row, routing Routing) []string {
	records, ok, err := store.existingRecords(w.Tree)
	if err != nil || !ok {
		// addRecords read the same directory and reported its failure.
		return nil
	}
	found, err := records.skipRecords()
	if err != nil {
		return []string{"the skip records of " + w.Worktree + " cannot be listed: " + err.Error()}
	}
	var problems []string
	for _, f := range found {
		if f.err != nil {
			problems = append(problems, "the skip record "+f.path+" cannot be read, so it is not offered (remove it by hand): "+f.err.Error())
			continue
		}
		if row.State == StateNeedsCheck || row.Stopped != nil {
			continue
		}
		if item, _, ok := resetItemOf(w, f.rec, routing); ok {
			row.Resets = append(row.Resets, item)
			// A reset item waits on the human.
			row.Idle = false
		}
	}
	return problems
}

// readOldValues reads the old-value table of origin/main's _context.md.
func readOldValues(ctx context.Context, tasks Tasks, tree string) (OldValues, error) {
	doc, err := tasks.show(ctx, tree, originMain, contextPath)
	if err != nil {
		return nil, err
	}
	return ParseOldValues(doc), nil
}

// readAgentFacts reads the visible screen of each agent; a screen that
// cannot be fetched is marked not read, never defaulted.
func readAgentFacts(ctx context.Context, herdr Herdr, agents []Agent) []agentFacts {
	var out []agentFacts
	for _, a := range agents {
		af := agentFacts{Agent: a}
		raw, err := herdr.Screen(ctx, a.Pane, ScreenVisible)
		if err == nil {
			af.ScreenRead = true
			af.Screen = readScreen(a.Kind, raw)
		}
		out = append(out, af)
	}
	return out
}

// readWorkspace reads one workspace's facts: its agents and screens, its
// branch, its open PR, and the task documents. A failed read is named in
// Unreadable, never defaulted. The branch main is headMain, whose task only
// applyMainTask gives.
func readWorkspace(ctx context.Context, herdr Herdr, git GitReader, tasks Tasks, w TaskWorkspace) workspaceFacts {
	f := workspaceFacts{Workspace: w.Worktree, Label: w.Label, Agents: readAgentFacts(ctx, herdr, w.Agents)}
	branch, err := git.Branch(ctx, w.Worktree)
	if err != nil {
		f.Unreadable = append(f.Unreadable, "branch of the worktree")
		return f
	}
	switch branch {
	case "":
		f.Head = headDetached
		return f
	case mainBranch:
		// The one comparison of a branch's name with main: everything after
		// it goes by headMain.
		f.Head, f.Branch = headMain, branch
	default:
		f.Head, f.Branch = headBranch, branch
	}
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
			f.DocsUnreadable = true
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
	if onlyMergeQuestions(r) {
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
// urgency (now, later, wait), then by state, then, for the states that wait
// on the human, by how long their first item has waited, then by workspace
// path. pending must already carry its times.
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
			cmp.Compare(a.Urgency, b.Urgency),
			cmp.Compare(a.State.rank(), b.State.rank()),
			cmp.Compare(mergeOnlyRank(a), mergeOnlyRank(b)),
			first[a.Workspace].Compare(first[b.Workspace]),
			strings.Compare(a.Workspace, b.Workspace),
		)
	})
}
