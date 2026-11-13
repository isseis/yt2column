//go:build test

package chief

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	taskBranch = "user/0001-test-task-02"
	taskDir    = "0001_test_task"
	openPRJSON = `[{"headRefName":"user/0001-test-task-02","number":42,"title":"feat(0001): x","url":"https://github.com/owner/repo/pull/42"}]`
)

var (
	reqApproved  = statusDoc("| Status | `approved` |\n| Process | `full` |\n")
	archApproved = statusDoc("| Status | `approved` |\n")
)

func rowOf(t *testing.T, snap Snapshot, label string) Row {
	t.Helper()
	for _, r := range snap.Rows {
		if r.Label == label {
			return r
		}
	}
	t.Fatalf("no row labeled %q in %+v", label, snap.Rows)
	return Row{}
}

func take(t *testing.T, w *world) Snapshot {
	t.Helper()
	snap, err := TakeSnapshot(context.Background(), w.finish(), w.owner())
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// TestTakeSnapshotStates reads one workspace of each state through the fake
// Herdr and checks its row.
func TestTakeSnapshotStates(t *testing.T) {
	cases := []struct {
		name  string
		ws    fakeWorkspace
		state WorkspaceState
		check func(t *testing.T, r Row)
	}{
		{"running", fakeWorkspace{Branch: taskBranch, Status: "working"}, StateRunning, nil},
		{"question", fakeWorkspace{Branch: taskBranch, Status: "blocked", Screen: "claude-question-single.ansi"}, StateQuestion, func(t *testing.T, r Row) {
			if len(r.Questions) != 1 || r.Questions[0].Question.Text == "" || len(r.Questions[0].Question.Options) == 0 || r.Questions[0].Pane == "" || r.MergeQuestion {
				t.Errorf("question row = %+v", r)
			}
		}},
		{"merge question", fakeWorkspace{Branch: taskBranch, Status: "blocked", Screen: "claude-merge-question.ansi"}, StateQuestion, func(t *testing.T, r Row) {
			if !r.MergeQuestion || r.Next != nil || r.Takeover != nil {
				t.Errorf("merge question row = %+v, want it marked and with no command offered", r)
			}
		}},
		{"opencode question while idle", fakeWorkspace{Kind: "opencode", Branch: "main", Status: "idle", Screen: "opencode-question-single.ansi"}, StateQuestion, nil},
		{"opencode prompt", fakeWorkspace{Kind: "opencode", Branch: "main", Status: "done", Screen: "opencode-done.ansi"}, StateProposeNext, nil},
		{"idle on a screen of no known shape", fakeWorkspace{Branch: "main", Status: "idle", Screen: "opencode-done.ansi"}, StateNeedsCheck, func(t *testing.T, r Row) {
			if len(r.Unreadable) != 1 || !strings.Contains(r.Unreadable[0], "w1:p1") {
				t.Errorf("unreadable = %v", r.Unreadable)
			}
		}},
		{"blocked on an unknown shape", fakeWorkspace{Branch: taskBranch, Status: "blocked", Screen: "claude-idle.ansi"}, StateNeedsCheck, func(t *testing.T, r Row) {
			if len(r.Unreadable) != 1 || !strings.Contains(r.Unreadable[0], "w1:p1") {
				t.Errorf("unreadable = %v", r.Unreadable)
			}
		}},
		{"awaiting review", fakeWorkspace{Branch: taskBranch, PRs: openPRJSON}, StateAwaitingReview, func(t *testing.T, r Row) {
			if r.PR == nil || r.PR.Number != 42 {
				t.Errorf("PR = %+v", r.PR)
			}
		}},
		{"propose runplan", fakeWorkspace{Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, StateProposeNext, func(t *testing.T, r Row) {
			if r.Next == nil || r.Next.Kind != TaskNextRunplan || r.Next.Text != "/runplan 0001" || r.Next.Command == nil || r.Next.Agent == 0 || r.Next.Model == "" {
				t.Errorf("next = %+v", r.Next)
			}
		}},
		{"task done", fakeWorkspace{Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-all-merged.md")}}, StateTaskDone, nil},
		{"no task", fakeWorkspace{Branch: "main"}, StateProposeNext, func(t *testing.T, r Row) {
			if r.Next == nil || r.Next.Kind != TaskNextMkreq || r.Next.Command != nil {
				t.Errorf("next = %+v", r.Next)
			}
		}},
		{"task without documents", fakeWorkspace{Branch: taskBranch}, StateNeedsCheck, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			c.ws.ID, c.ws.Label = "w1", "ws"
			w.add(c.ws)
			r := rowOf(t, take(t, w), "ws")
			if r.State != c.state {
				t.Fatalf("state = %v (%v), want %v", r.State, r.Unreadable, c.state)
			}
			if c.check != nil {
				c.check(t, r)
			}
		})
	}
}

// TestTakeSnapshotUnreadable makes each read of one workspace fail and checks
// that the workspace, and only it, needs a check with the item's name.
func TestTakeSnapshotUnreadable(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]func(w *world, tree string){
		"gh fails": func(w *world, tree string) {
			w.rec.failIn(tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", boom)
		},
		"gh output unreadable": func(w *world, tree string) {
			w.rec.replyIn(tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", []byte(`{"not":"an array"}`))
		},
		"two open PRs": func(w *world, tree string) {
			w.rec.replyIn(tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", []byte(strings.Replace(openPRJSON, "]", ","+openPRJSON[1:], 1)))
		},
		"branch fails": func(w *world, tree string) { w.rec.failIn(tree, "git rev-parse --abbrev-ref HEAD", boom) },
		"documents fail": func(w *world, tree string) {
			w.rec.failIn(tree, "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/", boom)
		},
		"screen fails": func(w *world, _ string) {
			w.rec.fail("herdr agent read w1:p1 --source visible --lines 200 --format ansi", boom)
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			tree := w.add(fakeWorkspace{ID: "w1", Label: "broken", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}})
			w.add(fakeWorkspace{ID: "w2", Label: "fine", Branch: "main"})
			breakIt(w, tree)
			snap := take(t, w)
			if r := rowOf(t, snap, "broken"); r.State != StateNeedsCheck || len(r.Unreadable) == 0 || r.Next != nil {
				t.Errorf("broken row = %+v, want a needs-check row naming what was unreadable", r)
			}
			if r := rowOf(t, snap, "fine"); r.State != StateProposeNext {
				t.Errorf("the other workspace was affected: %+v", r)
			}
		})
	}
}

// TestTakeSnapshotFetchFailure checks that a failed fetch is a problem of the
// round and that a workspace that would be proposed a command needs a check
// instead, since origin/main may be stale.
func TestTakeSnapshotFetchFailure(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: "main"})
	w.rec.failIn(w.chiefTree, "git fetch origin main", errors.New("offline"))
	snap := take(t, w)
	if len(snap.Problems) != 1 || !strings.Contains(snap.Problems[0], "git fetch origin main") {
		t.Errorf("problems = %v", snap.Problems)
	}
	if r := rowOf(t, snap, "ws"); r.State != StateNeedsCheck {
		t.Errorf("state = %v, want needs check", r.State)
	}
}

// TestTakeSnapshotRoutingUnreadable checks that an unreadable routing table
// leaves the state alone and offers no command.
func TestTakeSnapshotRoutingUnreadable(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}})
	w.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", []byte("### Command routing\n\nnothing\n"))
	snap := take(t, w)
	r := rowOf(t, snap, "ws")
	if r.State != StateProposeNext || r.Next == nil || r.Next.Command != nil || r.Next.Problem == "" {
		t.Errorf("row = %+v, want a proposal without a sendable command", r)
	}
	if len(snap.Problems) == 0 {
		t.Error("the unreadable routing table is not a problem of the round")
	}
}

// TestNextCommandRules covers each row of the rule table, the simple
// process's three-document merge, and the requirements that offer no
// /runplan.
func TestNextCommandRules(t *testing.T) {
	draft := statusDoc("| Status | `draft` |\n| Process | `full` |\n")
	simple := statusDoc("| Status | `approved` |\n| Process | `simple` |\n")
	unrecorded := statusDoc("| Status | `approved` |\n")
	cases := []struct {
		name string
		docs fakeDocs
		want TaskNextKind
		text string
	}{
		{"requirements approved, full", fakeDocs{Dir: taskDir, Req: reqApproved}, TaskNextMkarch, "/mkarch 0001"},
		{"requirements approved, no record", fakeDocs{Dir: taskDir, Req: unrecorded}, TaskNextMkarch, "/mkarch 0001"},
		{"architecture approved", fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved}, TaskNextMkplan, "/mkplan 0001"},
		{"plan approved", fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}, TaskNextRunplan, "/runplan 0001"},
		{"simple process after the three documents", fakeDocs{Dir: taskDir, Req: simple, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}, TaskNextRunplan, "/runplan 0001"},
		{"requirements draft", fakeDocs{Dir: taskDir, Req: draft}, TaskNextNone, ""},
		{"simple process without architecture", fakeDocs{Dir: taskDir, Req: simple}, TaskNextNone, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &c.docs})
			r := rowOf(t, take(t, w), "ws")
			if c.want == TaskNextNone {
				if r.State != StateNeedsCheck || r.Next != nil || len(r.Unreadable) != 1 {
					t.Errorf("row = %+v, want needs check with one reason", r)
				}
				return
			}
			if r.State != StateProposeNext || r.Next == nil || r.Next.Kind != c.want || r.Next.Text != c.text || r.Next.Command == nil {
				t.Errorf("row = %+v / %+v, want %v %q", r, r.Next, c.want, c.text)
			}
		})
	}
}

// TestRunplanRouting checks that the model requirement picks the route and
// that a requirement that cannot be read offers no /runplan.
func TestRunplanRouting(t *testing.T) {
	cases := []struct {
		plan     string
		sendable bool
		problem  string
	}{
		{"marker-opus.md", true, ""},
		{"marker-sonnet-review.md", true, ""},
		{"marker-sonnet.md", true, ""},
		{"marker-unreadable.md", false, "unreadable"},
		{"marker-two-values.md", false, "unreadable"},
		{"marker-absent.md", false, "absent"},
	}
	for _, c := range cases {
		t.Run(c.plan, func(t *testing.T) {
			w := newWorld(t)
			w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, c.plan)}})
			r := rowOf(t, take(t, w), "ws")
			if r.State != StateProposeNext || r.Next == nil || (r.Next.Command != nil) != c.sendable || !strings.Contains(r.Next.Problem, c.problem) {
				t.Errorf("next = %+v", r.Next)
			}
		})
	}
}

// takeoverSetup returns a world whose one workspace meets every condition of
// the takeover proposal when the routing table sends the opus /runplan to
// OpenCode.
func takeoverSetup(t *testing.T) (*world, string) {
	t.Helper()
	w := newWorld(t)
	context := strings.Replace(string(repoContext(t)), "| `runplan-opus` | `/runplan` for the requirement `opus` | Claude Code | `opus` |", "| `runplan-opus` | `/runplan` for the requirement `opus` | OpenCode | `DeepSeek V4.1 Flash 1017 OpenCode Go` |", 1)
	if context == string(repoContext(t)) {
		t.Fatal("the routing row to change was not found in _context.md")
	}
	tree := w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}})
	w.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", []byte(context))
	w.rec.replyIn(tree, "git rev-list --count refs/remotes/origin/main..HEAD", []byte("3\n"))
	w.rec.replyIn(tree, "git show HEAD:docs/tasks/"+taskDir+"/"+planFile, planSample(t, "marker-opus.md"))
	return w, tree
}

// TestTakeoverProposal checks the proposal and then removes one condition at
// a time.
func TestTakeoverProposal(t *testing.T) {
	t.Run("all conditions", func(t *testing.T) {
		w, _ := takeoverSetup(t)
		r := rowOf(t, take(t, w), "ws")
		if r.Next == nil || r.Takeover == nil || r.Takeover.Kind != TaskNextRunplanTakeover || r.Takeover.Text != "/runplan 0001" || r.Takeover.Command == nil {
			t.Fatalf("takeover = %+v (next %+v, unreadable %v)", r.Takeover, r.Next, r.Unreadable)
		}
	})
	removals := map[string]func(w *world, tree string){
		"no commits ahead": func(w *world, tree string) {
			w.rec.replyIn(tree, "git rev-list --count refs/remotes/origin/main..HEAD", []byte("0\n"))
		},
		"an open PR": func(w *world, tree string) {
			w.rec.replyIn(tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", []byte(openPRJSON))
		},
		"an agent that does not accept input": func(w *world, _ string) {
			for i := range w.agents {
				if w.agents[i].Pane == "w1:p1" {
					w.agents[i].Status = "working"
				}
			}
		},
		"steps not done": func(w *world, tree string) {
			w.rec.replyIn(tree, "git show HEAD:docs/tasks/"+taskDir+"/"+planFile, planSample(t, "marker-opus-steps-open.md"))
		},
		"the route is not OpenCode": func(w *world, _ string) {
			w.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", repoContext(t))
		},
	}
	for name, remove := range removals {
		t.Run(name, func(t *testing.T) {
			w, tree := takeoverSetup(t)
			remove(w, tree)
			r := rowOf(t, take(t, w), "ws")
			if r.Takeover != nil {
				t.Errorf("takeover proposed without %s: %+v", name, r.Takeover)
			}
		})
	}
	t.Run("a requirement that is sonnet", func(t *testing.T) {
		w, tree := takeoverSetup(t)
		w.docs(tree, fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-sonnet.md")})
		if r := rowOf(t, take(t, w), "ws"); r.Takeover != nil {
			t.Errorf("takeover proposed for a sonnet requirement: %+v", r.Takeover)
		}
	})
}

// TestTakeSnapshotOrder checks the row order and the pending order across
// kinds and waiting time.
func TestTakeSnapshotOrder(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "running", Branch: taskBranch, Status: "working"})
	w.add(fakeWorkspace{ID: "w2", Label: "proposal", Branch: "main"})
	w.add(fakeWorkspace{ID: "w3", Label: "review", Branch: taskBranch, PRs: openPRJSON})
	w.add(fakeWorkspace{ID: "w4", Label: "question", Branch: taskBranch, Status: "blocked", Screen: "claude-question-single.ansi"})
	snap := take(t, w)
	var labels []string
	for _, r := range snap.Rows {
		labels = append(labels, r.Label)
	}
	if want := []string{"question", "review", "proposal", "running"}; !slices.Equal(labels, want) {
		t.Errorf("row order = %v, want %v", labels, want)
	}
	var kinds []PendingKind
	for _, p := range snap.Pending {
		kinds = append(kinds, p.Kind)
	}
	if want := []PendingKind{PendingQuestion, PendingReview, PendingProposal}; !slices.Equal(kinds, want) {
		t.Errorf("pending kinds = %v, want %v", kinds, want)
	}
	for _, p := range snap.Pending {
		if !p.New {
			t.Errorf("first sighting of %v is not new", p.Kind)
		}
	}
	again := take(t, w)
	for _, p := range again.Pending {
		if p.New {
			t.Errorf("%v announced twice in one session", p.Kind)
		}
	}
}
