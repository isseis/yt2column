//go:build test && unix

package chief

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// The reasons of a row on main, as the design's 4.2 gives them: the wording
// is the row's output, read by the human in the status table.
const (
	reasonNoMerge    = "the worktree is on main and this chief session has seen no merged PR of it, so its task is not known"
	reasonOtherTask  = "the worktree is on main and this chief session has seen it with more than one task or with a PR of no task: "
	reasonOpenPR     = "the worktree is on main and a PR this chief session saw open on it is not known to be merged or closed: "
	reasonUnreadable = "the worktree is on main and a history record of this chief session cannot be read (start a new chief session with /clear): "
	reasonUnread     = "the worktree is on main and its history was not read, so its task is not known"
)

// otherTaskBranch is a feature branch of task 0002, whose directory the
// main tests serve as otherTaskDir.
const (
	otherTaskBranch = "user/0002-other-task-01"
	otherTaskDir    = "0002_other_task"
)

// putHistory writes events into tree's directory of records as the world
// owner's session wrote them: each event's session and worktree are set
// here.
func (w *world) putHistory(tree string, events ...traceEvent) {
	w.t.Helper()
	for _, e := range events {
		e.Session, e.Worktree = w.chief.Session, Worktree{path: tree}
		putTrace(w.t, w.store, e.Worktree, e.name(), traceJSON(w.t, e))
	}
}

// opened is the events of a round that saw PR pr open on branch, with the
// worktree on that branch.
func opened(t *testing.T, round, pr int, branch string) []traceEvent {
	t.Helper()
	task, ok := taskOfBranch(branch)
	if !ok {
		t.Fatalf("%q is no feature branch", branch)
	}
	return []traceEvent{
		{Round: traceRound(round), Kind: traceBranchTask, Task: task},
		{Round: traceRound(round), Kind: tracePROpen, PR: pr, Task: task, Branch: branch},
	}
}

// ended is the event of a round that read PR pr as merged or closed.
func ended(round int, kind traceKind, pr int) traceEvent {
	return traceEvent{Round: traceRound(round), Kind: kind, PR: pr}
}

// mergedOn is the history of PR pr of branch seen open in round 1 and
// merged in round 2.
func mergedOn(t *testing.T, pr int, branch string) []traceEvent {
	t.Helper()
	return append(opened(t, 1, pr, branch), ended(2, tracePRMerged, pr))
}

// onMain moves the workspace at tree to main, with no open PR on it.
func (w *world) onMain(tree string) {
	w.git(tree, "rev-parse --abbrev-ref HEAD", mainBranch+"\n")
	w.rec.replyIn(tree, "gh pr list --head "+mainBranch+" --state open --json headRefName,number,title,url", []byte("[]"))
}

// TestMainRowFollowsTaskRules checks that a worktree on main whose history
// identifies task 0001 gets the row a feature branch of 0001 gets: the next
// command of approved documents, the reason of a draft one, a question
// first; none of them idle; and no takeover on main.
func TestMainRowFollowsTaskRules(t *testing.T) {
	archDraft := statusDoc("| Status | `draft` |\n")
	cases := []struct {
		name     string
		ws       fakeWorkspace
		state    WorkspaceState
		next     string
		why      []string
		position Position
	}{
		{
			"approved architecture and no plan",
			fakeWorkspace{Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved}},
			StateProposeNext, "/mkplan 0001", nil,
			Position{Document: PositionDocPlan, Progress: ProgressAwaitingCreation},
		},
		{
			"draft architecture",
			fakeWorkspace{Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archDraft}},
			StateNeedsCheck, "",
			[]string{"the architecture document is not approved"},
			Position{Document: PositionDocNeedsCheck, Progress: ProgressNeedsCheck},
		},
		{
			"an agent asking",
			fakeWorkspace{Status: "blocked", Screen: "claude-question-single.ansi", Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved}},
			StateQuestion, "", nil,
			Position{Document: PositionDocPlan, Progress: ProgressQuestion},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			ws := c.ws
			ws.ID, ws.Label, ws.Branch = "w1", "main", mainBranch
			tree := w.add(ws)
			w.putHistory(tree, mergedOn(t, 7, taskBranch)...)
			r := rowOf(t, take(t, w), "main")
			next := ""
			if r.Next != nil {
				next = r.Next.Text
			}
			if r.State != c.state || r.Task != "0001" || next != c.next || !slices.Equal(r.Unreadable, c.why) || r.Position != c.position {
				t.Errorf("row = %+v (next %q); want %v of 0001 with next %q, why %q, position %+v", r, next, c.state, c.next, c.why, c.position)
			}
			if r.Idle {
				t.Errorf("row = %+v; want it not idle", r)
			}
		})
	}
	t.Run("no takeover on main", func(t *testing.T) {
		cmd, err := paneinput.NewRunplan(taskID(t, "0001"), paneinput.RouteRunplanOpus)
		if err != nil {
			t.Fatal(err)
		}
		next := &Next{Kind: TaskNextRunplan, Command: &cmd, Agent: paneinput.AgentOpenCode}
		f := workspaceFacts{Head: headBranch, Task: taskID(t, "0001"), Agents: []agentFacts{{Agent: Agent{Kind: paneinput.AgentClaudeCode, Status: AgentIdle}, ScreenRead: true, Screen: Screen{Kind: ScreenPrompt}}}}
		// The same facts on a feature branch need the takeover's reads, so
		// only the HEAD keeps main from them.
		if !needsTakeoverCheck(f, next) {
			t.Fatal("a feature branch with these facts needs no takeover check; the case tests nothing")
		}
		f.Head = headMain
		if needsTakeoverCheck(f, next) {
			t.Error("a worktree on main needs the takeover check")
		}
	})
}

// TestMainRowDocumentsOfTheTask checks a worktree on main identified as
// 0001 whose documents cannot be read, or whose task has no directory on
// origin/main: the row needs a check as a feature branch's would.
func TestMainRowDocumentsOfTheTask(t *testing.T) {
	listing := "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/"
	cases := []struct {
		name     string
		setup    func(w *world, tree string)
		why      string
		document PositionDocument
	}{
		{"the documents cannot be read", func(w *world, tree string) { w.rec.failIn(tree, listing, os.ErrPermission) }, "task documents on origin/main", PositionDocNeedsCheck},
		{"no directory of the task", func(w *world, tree string) { w.rec.replyIn(tree, listing, nil) }, "no requirements document of the task on origin/main", PositionDocRequirements},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			tree := w.add(fakeWorkspace{ID: "w1", Label: "main", Branch: mainBranch})
			c.setup(w, tree)
			w.putHistory(tree, mergedOn(t, 7, taskBranch)...)
			r := rowOf(t, take(t, w), "main")
			if r.State != StateNeedsCheck || r.Task != "0001" || r.Next != nil || r.Idle || !slices.Equal(r.Unreadable, []string{c.why}) || r.Position.Document != c.document {
				t.Errorf("row = %+v; want 0001 to need a check for %q with document %d", r, c.why, c.document)
			}
		})
	}
}

// TestMainRowTaskDone checks that a worktree on main identified as a done
// task chief made the workspace for shows the task done with the removal
// proposed, stopped by the check that HEAD is detached.
func TestMainRowTaskDone(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	sw.onMain(rs.tree)
	sw.docs(rs.tree, fakeDocs{Dir: removalTaskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-all-merged.md")})
	sw.putHistory(rs.tree, mergedOn(t, 9, "user/0005-done-task-01")...)
	r := rowOf(t, snapshotOf(t, sw), removalLabel)
	if r.State != StateTaskDone || r.Task != removalTask || r.RemovalRow || r.Next != nil || r.Idle {
		t.Errorf("row = %+v; want task %s done, not the removal row", r, removalTask)
	}
	if r.Removal == nil {
		t.Fatal("no removal proposed")
	}
	// The detached check is there and fails; every other check passes.
	others := slices.ContainsFunc(r.Removal.Checks, func(c CheckResult) bool {
		return c.Check != RemovalCheckDetached && (!c.Passed || len(c.Unreadable) > 0)
	})
	detached := slices.IndexFunc(r.Removal.Checks, func(c CheckResult) bool { return c.Check == RemovalCheckDetached })
	if len(r.Removal.Checks) != len(removalChecks) || detached < 0 || r.Removal.Checks[detached].Passed || others {
		t.Errorf("checks = %+v; want the removal stopped by the detached check alone", r.Removal.Checks)
	}
}

// TestMainRowNotIdentified checks the row of a worktree on main whose
// history identifies no task: it needs a check with the reason of the
// identification, offers nothing (no /mkreq either), shows no document, and
// is idle only for no merge or a PR still open, after reads that all
// succeeded.
func TestMainRowNotIdentified(t *testing.T) {
	cases := []struct {
		name    string
		history func(t *testing.T) []traceEvent
		why     string
		idle    bool
	}{
		{"no history", func(*testing.T) []traceEvent { return nil }, reasonNoMerge, true},
		// No merged PR comes before a PR still open in the rules' order.
		{"a PR seen open only", func(t *testing.T) []traceEvent { return opened(t, 1, 7, taskBranch) }, reasonNoMerge, true},
		{"a PR closed without a merge", func(t *testing.T) []traceEvent {
			return append(opened(t, 1, 7, taskBranch), ended(2, tracePRClosed, 7))
		}, reasonNoMerge, true},
		{"a merged PR and another PR of the task still open", func(t *testing.T) []traceEvent {
			return append(mergedOn(t, 7, taskBranch), opened(t, 3, 8, taskBranch)...)
		}, reasonOpenPR + "#8", true},
		{"merged PRs of two tasks", func(t *testing.T) []traceEvent {
			return append(mergedOn(t, 7, taskBranch), append(opened(t, 3, 8, otherTaskBranch), ended(4, tracePRMerged, 8))...)
		}, reasonOtherTask + "#7 of 0001, #8 of 0002", false},
		{"a branch of another task after the merge", func(t *testing.T) []traceEvent {
			return append(mergedOn(t, 7, taskBranch), traceEvent{Round: traceRound(3), Kind: traceBranchTask, Task: taskID(t, "0002")})
		}, reasonOtherTask + "a branch of 0002", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			tree := w.add(fakeWorkspace{ID: "w1", Label: "main", Branch: mainBranch})
			w.putHistory(tree, c.history(t)...)
			assertMainNeedsCheck(t, rowOf(t, take(t, w), "main"), c.why, c.idle)
		})
	}
	t.Run("a failed fetch", func(t *testing.T) {
		w := newWorld(t)
		w.add(fakeWorkspace{ID: "w1", Label: "main", Branch: mainBranch})
		w.rec.failIn(w.chiefTree, "git fetch origin main", os.ErrDeadlineExceeded)
		assertMainNeedsCheck(t, rowOf(t, take(t, w), "main"), reasonNoMerge, false)
	})
	t.Run("a PR whose state this round could not read", func(t *testing.T) {
		tree := Worktree{path: "/work/main"}
		events := append(mergedOn(t, 6, taskBranch), opened(t, 3, 7, taskBranch)...)
		for i := range events {
			events[i].Session, events[i].Worktree = "s", tree
		}
		f := workspaceFacts{Head: headMain, Branch: mainBranch, Agents: []agentFacts{{Agent: Agent{Kind: paneinput.AgentClaudeCode, Status: AgentIdle}, ScreenRead: true, Screen: Screen{Kind: ScreenPrompt}}}}
		f.MainTask = identifyMainTask(traceHistory{events: map[Worktree][]traceEvent{tree: events}}, tree)
		if f.MainTask.Kind != mainTaskOpenPR {
			t.Fatalf("identification = %+v; want a PR still open", f.MainTask)
		}
		if r := classify(f); !r.Idle {
			t.Fatalf("row = %+v; with every state read the row is idle, so the case tests nothing", r)
		}
		f.MainStateUnread = true
		if r := classify(f); r.Idle || r.State != StateNeedsCheck {
			t.Errorf("row = %+v; want it to need a check, not idle", r)
		}
		// A PR seen open and never merged gives no merge, and its state not
		// read keeps that row from being idle too.
		noMerge := opened(t, 1, 7, taskBranch)
		for i := range noMerge {
			noMerge[i].Session, noMerge[i].Worktree = "s", tree
		}
		f.MainTask = identifyMainTask(traceHistory{events: map[Worktree][]traceEvent{tree: noMerge}}, tree)
		if f.MainTask.Kind != mainTaskNoMerge {
			t.Fatalf("identification = %+v; want no merge", f.MainTask)
		}
		if r := classify(f); r.Idle {
			t.Errorf("row = %+v; want it not idle", r)
		}
	})
}

// assertMainNeedsCheck checks that r, a row on main with no task
// identified, needs a check for why alone, offers nothing, shows the
// document as needing a check (never the "-" of a branch without a task),
// and is idle as idle says.
func assertMainNeedsCheck(t *testing.T, r Row, why string, idle bool) {
	t.Helper()
	if r.State != StateNeedsCheck || r.Task != "" || r.Next != nil || !slices.Equal(r.Unreadable, []string{why}) {
		t.Errorf("row = %+v; want it to need a check for %q with nothing offered", r, why)
	}
	if r.Position.Document != PositionDocNeedsCheck || r.Idle != idle {
		t.Errorf("document %d, idle %v; want needs check, idle %v", r.Position.Document, r.Idle, idle)
	}
}

// TestMainRowUnreadableHistory puts a record that cannot be read in the
// directory of another worktree than a main one, on main or on a feature
// branch, or makes a feature branch worktree's directory unlistable: every
// row on main then needs a check for it, whatever its own history, and the
// feature branch's row is as it was. A main row whose agent asks needs the
// check too.
func TestMainRowUnreadableHistory(t *testing.T) {
	cases := []struct {
		name string
		// spoil breaks the history, given the second main worktree and the
		// feature branch's, and returns the path the reason names.
		spoil func(t *testing.T, store Store, main2, branch string) string
	}{
		{"a record of no form in another main worktree's directory", func(t *testing.T, store Store, main2, _ string) string {
			return putTrace(t, store, Worktree{path: main2}, "trace-broken.json", []byte("{}"))
		}},
		{"a record of no form in a feature branch worktree's directory", func(t *testing.T, store Store, _, branch string) string {
			return putTrace(t, store, Worktree{path: branch}, "trace-broken.json", []byte("{}"))
		}},
		{"an unlistable directory of a feature branch worktree", func(t *testing.T, store Store, _, branch string) string {
			if os.Geteuid() == 0 {
				t.Skip("root lists a directory of mode 0")
			}
			records, err := store.Records(Worktree{path: branch})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(records.dir, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(records.dir, 0o700) })
			return records.dir
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			// main1 has an agent asking: a history that cannot be read is a
			// failed read, which comes before the question.
			m1 := w.add(fakeWorkspace{ID: "m1", Label: "main1", Branch: mainBranch, Status: "blocked", Screen: "claude-question-single.ansi", Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved}})
			m2 := w.add(fakeWorkspace{ID: "m2", Label: "main2", Branch: mainBranch})
			branch := w.add(fakeWorkspace{ID: "b1", Label: "branch", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved}})
			// main1's own history identifies 0001 until the record is spoiled.
			w.putHistory(m1, mergedOn(t, 7, taskBranch)...)
			before := take(t, w)
			if r := rowOf(t, before, "main1"); r.Task != "0001" || r.State != StateQuestion {
				t.Fatalf("main1 = %+v before the history is spoiled; want the question of task 0001", r)
			}
			path := c.spoil(t, w.store, m2, branch)
			snap := take(t, w)
			for _, label := range []string{"main1", "main2"} {
				r := rowOf(t, snap, label)
				if r.State != StateNeedsCheck || r.Task != "" || r.Next != nil || r.Idle || len(r.Unreadable) != 1 || !strings.HasPrefix(r.Unreadable[0], reasonUnreadable) || !strings.Contains(r.Unreadable[0], path) {
					t.Errorf("%s = %+v; want it to need a check for %s", label, r, path)
				}
			}
			// An unlistable directory also hides the branch's stop, which
			// addRecords reports; its state and proposal stay.
			got, want := rowOf(t, snap, "branch"), rowOf(t, before, "branch")
			if got.State != want.State || got.Task != want.Task || !reflect.DeepEqual(got.Next, want.Next) || !slices.Equal(got.Unreadable, want.Unreadable) {
				t.Errorf("branch row = %+v, was %+v", got, want)
			}
		})
	}
}

// TestMainRowZeroResult checks that a row on main to which no history was
// applied needs a check and offers nothing, /mkreq included.
func TestMainRowZeroResult(t *testing.T) {
	f := workspaceFacts{Head: headMain, Branch: mainBranch, Agents: []agentFacts{{Agent: Agent{Kind: paneinput.AgentClaudeCode, Status: AgentIdle}, ScreenRead: true, Screen: Screen{Kind: ScreenPrompt}}}}
	r := classify(f)
	if r.State != StateNeedsCheck || r.Next != nil || r.Task != "" || r.Idle || !slices.Equal(r.Unreadable, []string{reasonUnread}) {
		t.Errorf("row = %+v; want it to need a check with no next command", r)
	}
}

// TestOtherHeadsIgnoreHistory writes, for a worktree on a feature branch, a
// detached HEAD, or another branch, a history that identifies task 0002,
// and checks that the row and its removal proposal are what they were
// without it. Each case first checks that the same history identifies 0002
// once the worktree is on main, so the history is one a main row reads.
func TestOtherHeadsIgnoreHistory(t *testing.T) {
	otherDocs := fakeDocs{Dir: otherTaskDir, Req: reqApproved, Arch: archApproved}
	cases := []struct {
		name  string
		setup func(t *testing.T) (*world, string, string, func() Snapshot)
	}{
		{"a feature branch", func(t *testing.T) (*world, string, string, func() Snapshot) {
			w := newWorld(t)
			tree := w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved}})
			return w, tree, "ws", func() Snapshot { return take(t, w) }
		}},
		{"a detached HEAD", func(t *testing.T) (*world, string, string, func() Snapshot) {
			w := newWorld(t)
			tree := w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: detachedHead})
			return w, tree, "ws", func() Snapshot { return take(t, w) }
		}},
		{"another branch", func(t *testing.T) (*world, string, string, func() Snapshot) {
			w := newWorld(t)
			tree := w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: "scratch"})
			return w, tree, "ws", func() Snapshot { return take(t, w) }
		}},
		{"a detached HEAD of a done task chief made", func(t *testing.T) (*world, string, string, func() Snapshot) {
			sw, rs := newRemovalWorld(t)
			return sw.world, rs.tree, removalLabel, func() Snapshot { return snapshotOf(t, sw) }
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, tree, label, snapshot := c.setup(t)
			want := rowOf(t, snapshot(), label)
			w.putHistory(tree, mergedOn(t, 7, otherTaskBranch)...)
			if got := rowOf(t, snapshot(), label); !reflect.DeepEqual(got, want) {
				t.Errorf("row with the history = %+v; want %+v", got, want)
			}
			w.onMain(tree)
			w.docs(tree, otherDocs)
			if r := rowOf(t, snapshot(), label); r.Task != "0002" {
				t.Errorf("row on main = %+v; want the history to identify 0002", r)
			}
		})
	}
}

// traceNames returns the names of the history records in tree's directory
// of records.
func traceNames(t *testing.T, store Store, tree string) []string {
	t.Helper()
	records, ok, err := store.existingRecords(Worktree{path: tree})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		return nil
	}
	entries, err := os.ReadDir(records.dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tracePrefix) {
			names = append(names, e.Name())
		}
	}
	return names
}

// prViews counts the readings of PR 42's state so far.
func prViews(rec *recorder) int {
	n := 0
	for _, c := range rec.recorded() {
		if c.name == ghProgram && strings.Join(c.args, " ") == "pr view 42 --json "+prStateFields {
			n++
		}
	}
	return n
}

// hasProblem reports whether a problem of snap starts with prefix.
func hasProblem(snap Snapshot, prefix string) bool {
	return slices.ContainsFunc(snap.Problems, func(p string) bool { return strings.HasPrefix(p, prefix) })
}

// TestTickWritesHistory runs the merge world's PR 42 from open on the task
// branch to merged with the worktree on main, and checks the history the
// ticks write: the open's round writes the branch and the open, and the
// merge's round writes the merge and shows task 0001 on main in the same
// round, reading the PR's state once. When the merge cannot be written
// safely (the exclusion is held, the branch or the open record changed
// under it, the state cannot be read, the record cannot be made, or the
// round cannot be recorded) nothing is written and the round says why; a
// held creation exclusion does not stop the write.
func TestTickWritesHistory(t *testing.T) {
	// branchReads answers the branch reads of the merge world, whose one
	// worktree is the only one read: the round's reading is the first and
	// the reading under the history's exclusion the second.
	branchReads := func(mw *mergeWorld, under func(n int) string) {
		n := 0
		mw.rec.replyFunc("git rev-parse --abbrev-ref HEAD", func() reply {
			n++
			return reply{out: []byte(under(n) + "\n")}
		})
	}
	cases := []struct {
		name string
		// before runs after the merge, before the merge's tick; it returns
		// what the round's problems must hold, "" for none.
		before func(t *testing.T, mw *mergeWorld) string
		// written is whether the merge's tick writes the merge.
		written bool
		// views is how many times the merge's tick reads #42's state.
		views int
	}{
		{"nothing in the way", func(*testing.T, *mergeWorld) string { return "" }, true, 1},
		{"the creation exclusion is held", func(t *testing.T, mw *mergeWorld) string {
			lock, err := mw.deps.Store.LockCreation()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = lock.Close() })
			return ""
		}, true, 1},
		{"the worktree's exclusion is held", func(t *testing.T, mw *mergeWorld) string {
			records, err := mw.store.Records(Worktree{path: mw.tree})
			if err != nil {
				t.Fatal(err)
			}
			held, err := records.Lock()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = held.Close() })
			return "the history record of " + mw.tree + " cannot be written: its exclusion cannot be taken"
		}, false, 1},
		{"the branch changes under the exclusion", func(_ *testing.T, mw *mergeWorld) string {
			// The round reads main; the reading under the exclusion is the
			// second.
			branchReads(mw, func(n int) string {
				if n == 1 {
					return mainBranch
				}
				return "scratch"
			})
			return "the history record of " + mw.tree + " cannot be written: " + errHistoryChanged.Error() + ": its branch is no longer the one the round read"
		}, false, 1},
		{"the open record is gone under the exclusion", func(t *testing.T, mw *mergeWorld) string {
			branchReads(mw, func(n int) string {
				if n == 2 {
					removeOpenRecord(t, mw.store, mw.tree)
				}
				return mainBranch
			})
			return "the history record of " + mw.tree + " cannot be written: " + errHistoryChanged.Error() + ": the open record of #42 is gone"
		}, false, 1},
		{"the state cannot be read", func(_ *testing.T, mw *mergeWorld) string {
			mw.rec.failIn(mw.tree, prStateLine, errNoReply)
			return "the state of #42 cannot be read for the history of " + mw.tree
		}, false, 1},
		{"the record cannot be made", func(_ *testing.T, mw *mergeWorld) string {
			mw.store.beforeCreate = failCreates(tracePrefix)
			mw.deps.Store = mw.store
			return "the history record of " + mw.tree + " cannot be written: " + errNoReply.Error()
		}, false, 1},
		{"the round cannot be recorded", func(_ *testing.T, mw *mergeWorld) string {
			mw.store.beforeCreate = failCreates(roundPrefix)
			mw.deps.Store = mw.store
			return "the round cannot be recorded"
		}, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			mw.tick(time.Minute)
			if got := len(traceNames(t, mw.store, mw.tree)); got != 2 {
				t.Fatalf("history after the open's round = %d records; want the branch and the open", got)
			}
			mw.merge(mainBranch, "")
			want := c.before(t, mw)
			before := traceNames(t, mw.store, mw.tree)
			views := prViews(mw.rec)
			snap := mw.tick(time.Minute).Snapshot
			if want != "" && !hasProblem(snap, want) {
				t.Errorf("problems = %q; want one starting %q", snap.Problems, want)
			}
			if got := prViews(mw.rec) - views; got != c.views {
				t.Errorf("the merge's round read #42's state %d times; want %d", got, c.views)
			}
			r := rowOf(t, snap, "ws")
			names := traceNames(t, mw.store, mw.tree)
			if !c.written {
				// The open-record case removes a record during the tick, so
				// only an added name is a write.
				if slices.ContainsFunc(names, func(n string) bool { return !slices.Contains(before, n) }) {
					t.Errorf("history = %v, was %v; want nothing written", names, before)
				}
				// The row goes by the history without the merge.
				if r.Task != "" || r.State != StateNeedsCheck {
					t.Errorf("row = %+v; want main with no task identified", r)
				}
				return
			}
			if !mergeWritten(t, mw) || r.Task != "0001" || r.Next == nil || r.Next.Text != "/mkarch 0001" {
				t.Errorf("history %v, row %+v; want the merge written and /mkarch 0001 shown", names, r)
			}
		})
	}
	t.Run("a state not read keeps the row from idle", func(t *testing.T) {
		// The tick comes after the observation window, so no automatic
		// send considers the PR and clears the mark: only the history's
		// reading decides it.
		for _, unread := range []bool{false, true} {
			mw := newMergeWorld(t, mkarchDocs)
			mw.tick(time.Minute)
			mw.onMain(mw.tree)
			if unread {
				mw.rec.failIn(mw.tree, prStateLine, errNoReply)
			}
			out := mw.tick(observeWindow + time.Minute)
			if r := rowOf(t, out.Snapshot, "ws"); len(out.AutoSends) != 0 || r.State != StateNeedsCheck || r.Idle == unread {
				t.Errorf("state unread %v: row %+v, auto sends %+v; want it idle only when the state was read", unread, r, out.AutoSends)
			}
		}
	})
	t.Run("a merge read on a detached HEAD", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(detachedHead, "")
		mw.tick(time.Minute)
		if !mergeWritten(t, mw) {
			t.Errorf("history = %v; want the merge written", traceNames(t, mw.store, mw.tree))
		}
	})
	t.Run("a round that cannot be recorded goes by the history written", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(mainBranch, "")
		mw.tick(time.Minute)
		mw.store.beforeCreate = failCreates(roundPrefix)
		mw.deps.Store = mw.store
		if r := rowOf(t, mw.tick(time.Minute).Snapshot, "ws"); r.Task != "0001" {
			t.Errorf("row = %+v; want 0001 from the merge written before", r)
		}
	})
}

// mergeWritten reports whether the merge world's history holds the merge of
// PR 42 after its branch and its open.
func mergeWritten(t *testing.T, mw *mergeWorld) bool {
	t.Helper()
	events := mw.store.readTraces(mw.chief.Session).events[Worktree{path: mw.tree}]
	return len(events) == 3 && events[2].Kind == tracePRMerged && events[2].PR == 42
}

// removeOpenRecord removes the pr_open record of tree's history.
func removeOpenRecord(t *testing.T, store Store, tree string) {
	t.Helper()
	records, err := store.Records(Worktree{path: tree})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range traceNames(t, store, tree) {
		data, err := os.ReadFile(records.dir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"kind":"`+tracePROpen.String()+`"`) {
			if err := os.Remove(records.dir + "/" + name); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestMainRowNoAutoSend checks that the merge of PR 42 read with the
// worktree on main, which identifies task 0001, is not sent automatically
// and is proposed for the reason that main is no feature branch of 0001.
func TestMainRowNoAutoSend(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(mainBranch, "`/mkarch 0001`")
	out := mw.tick(time.Minute)
	if r := rowOf(t, out.Snapshot, "ws"); r.Task != "0001" {
		t.Fatalf("row = %+v; want main identified as 0001", r)
	}
	res := only(t, out)
	if sentCommand(res) || len(mw.rec.prompts()) != 0 || !slices.Contains(res.Reasons, `the branch "main" is no feature branch of task 0001`) {
		t.Errorf("auto send = %+v; want nothing sent because main is no feature branch of 0001", res)
	}
}

// TestTraceReadingOf checks what the history's writes take from a round's
// facts: the place each HEAD is, the open PR, and a branch's task, which a
// pr_open names only when the round read that task's documents.
func TestTraceReadingOf(t *testing.T) {
	task := taskID(t, "0001")
	docs := &TaskDocs{Dir: taskDir}
	cases := []struct {
		name  string
		facts workspaceFacts
		want  traceReading
	}{
		{"a HEAD not read", workspaceFacts{PR: &OpenPR{Number: 7}}, traceReading{Place: tracePlaceUnread, PR: 7}},
		{"a detached HEAD", workspaceFacts{Head: headDetached}, traceReading{Place: tracePlaceDetached}},
		{"main", workspaceFacts{Head: headMain, Branch: mainBranch}, traceReading{Place: tracePlaceMain, Branch: mainBranch}},
		{"a task branch with its documents", workspaceFacts{Head: headBranch, Branch: taskBranch, Task: task, Docs: docs, PR: &OpenPR{Number: 7}}, traceReading{Place: tracePlaceBranch, Branch: taskBranch, BranchTask: task, DocsRead: true, PR: 7}},
		{"a task branch whose documents were not read", workspaceFacts{Head: headBranch, Branch: taskBranch, Task: task, DocsUnreadable: true}, traceReading{Place: tracePlaceBranch, Branch: taskBranch, BranchTask: task}},
		{"a branch of no task", workspaceFacts{Head: headBranch, Branch: "scratch"}, traceReading{Place: tracePlaceBranch, Branch: "scratch"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := traceReadingOf(c.facts); got != c.want {
				t.Errorf("reading = %+v, want %+v", got, c.want)
			}
		})
	}
}
