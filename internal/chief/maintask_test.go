//go:build test && unix

package chief

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// The reasons of a row on main, as the design's 4.2 gives them: the wording
// is the row's output, read by the human in the status table.
const (
	reasonNoMerge    = "the worktree is on main and this chief session has seen no merged PR of it, so its task is not known"
	reasonOtherTask  = "the worktree is on main and this chief session has seen it with more than one task or with a PR of no task: "
	reasonOpenPR     = "the worktree is on main and a PR this chief session saw open on it is not known to be merged or closed: "
	reasonUnreadable = "the worktree is on main and a history record of this chief session cannot be read (start a new chief session with /clear): "
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
// first, and no takeover; none of them idle.
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
			if r.Idle || r.Takeover != nil {
				t.Errorf("row idle %v, takeover %+v; want neither", r.Idle, r.Takeover)
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
	if r.Removal == nil || passedCheck(r.Removal.Checks, RemovalCheckDetached) || !passedCheck(r.Removal.Checks, RemovalCheckAgentsIdle) {
		t.Errorf("removal = %+v; want it proposed and stopped by the detached check alone", r.Removal)
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
	if r.State != StateNeedsCheck || r.Task != "" || r.Next != nil || r.Takeover != nil || !slices.Equal(r.Unreadable, []string{why}) {
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
	if r.State != StateNeedsCheck || r.Next != nil || r.Task != "" || r.Idle || !slices.Equal(r.Unreadable, []string{"the worktree is on main and its history was not read, so its task is not known"}) {
		t.Errorf("row = %+v; want it to need a check with no next command", r)
	}
	if doc, _ := documentOf(r, nil); doc != PositionDocNeedsCheck {
		t.Errorf("document = %d; want needs check", doc)
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
