//go:build test

package chief

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/privatefile"
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
		{"plan PR left with no step left", fakeWorkspace{Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-steps-all-done.md")}}, StateNeedsCheck, func(t *testing.T, r Row) {
			if r.Next != nil || len(r.Unreadable) != 1 || !strings.Contains(r.Unreadable[0], "no step left") {
				t.Errorf("row = %+v, want no command and the reason", r)
			}
		}},
		{"task done", fakeWorkspace{Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-all-merged.md")}}, StateTaskDone, nil},
		{"no task", fakeWorkspace{Branch: "main"}, StateProposeNext, func(t *testing.T, r Row) {
			if r.Next == nil || r.Next.Kind != TaskNextMkreq || r.Next.Command != nil {
				t.Errorf("next = %+v", r.Next)
			}
		}},
		{"task without documents", fakeWorkspace{Branch: taskBranch}, StateNeedsCheck, nil},
		{"detached HEAD", fakeWorkspace{Branch: detachedHead}, StateNeedsCheck, func(t *testing.T, r Row) {
			if r.Next != nil || len(r.Unreadable) != 1 || !strings.Contains(r.Unreadable[0], "detached") {
				t.Errorf("row = %+v, want no command and the detached HEAD named", r)
			}
		}},
		{"detached HEAD with a question", fakeWorkspace{Branch: detachedHead, Status: "blocked", Screen: "claude-question-single.ansi"}, StateQuestion, nil},
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
// /runplan. A row with no next command names its reason: the reason says
// which document stage the rule stopped at, and its wording is the contract
// shown to the human in Unreadable.
func TestNextCommandRules(t *testing.T) {
	draft := statusDoc("| Status | `draft` |\n| Process | `full` |\n")
	simple := statusDoc("| Status | `approved` |\n| Process | `simple` |\n")
	unrecorded := statusDoc("| Status | `approved` |\n")
	cases := []struct {
		name string
		docs fakeDocs
		want TaskNextKind
		text string
		why  string
	}{
		{"requirements approved, full", fakeDocs{Dir: taskDir, Req: reqApproved}, TaskNextMkarch, "/mkarch 0001", ""},
		{"requirements approved, no record", fakeDocs{Dir: taskDir, Req: unrecorded}, TaskNextMkarch, "/mkarch 0001", ""},
		{"architecture approved", fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved}, TaskNextMkplan, "/mkplan 0001", ""},
		{"plan approved", fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}, TaskNextRunplan, "/runplan 0001", ""},
		{"simple process after the three documents", fakeDocs{Dir: taskDir, Req: simple, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}, TaskNextRunplan, "/runplan 0001", ""},
		{"requirements draft", fakeDocs{Dir: taskDir, Req: draft}, TaskNextNone, "", "no rule gives a next command"},
		{"architecture draft", fakeDocs{Dir: taskDir, Req: reqApproved, Arch: draft}, TaskNextNone, "", "the architecture document is not approved"},
		{"plan draft after an approved architecture", fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: []byte(strings.Replace(string(planSample(t, "marker-opus.md")), "`approved`", "`draft`", 1))}, TaskNextNone, "", "the plan is not approved"},
		{"documents unreadable (a broken process record)", fakeDocs{Dir: taskDir, Req: statusDoc("| Status | `approved` |\n| Process | full |\n")}, TaskNextNone, "", "task documents on origin/main"},
		{"simple process without architecture", fakeDocs{Dir: taskDir, Req: simple}, TaskNextNone, "", "no rule gives a next command"},
		{"no requirements document", fakeDocs{Dir: taskDir}, TaskNextNone, "", "no requirements document of the task on origin/main"},
		{"no task directory", fakeDocs{}, TaskNextNone, "", "no requirements document of the task on origin/main"},
		{"plan with every PR merged", fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-all-merged.md")}, TaskNextNone, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &c.docs})
			r := rowOf(t, take(t, w), "ws")
			if c.want == TaskNextNone {
				if c.why == "" {
					if r.State != StateTaskDone || r.Next != nil {
						t.Errorf("row = %+v, want task done", r)
					}
					return
				}
				if r.State != StateNeedsCheck || r.Next != nil || !slices.Equal(r.Unreadable, []string{c.why}) {
					t.Errorf("row = %+v, want needs check with the reason %q", r, c.why)
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
// kinds and waiting time, and that items first seen in one round get their
// numbers in that order.
func TestTakeSnapshotOrder(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "running", Branch: taskBranch, Status: "working"})
	w.add(fakeWorkspace{ID: "w2", Label: "proposal", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved}})
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
	for i, p := range snap.Pending {
		if !p.New {
			t.Errorf("first sighting of %v is not new", p.Kind)
		}
		// Items first seen together are numbered in the order they list.
		if p.Number != i+1 {
			t.Errorf("item %d (%v) has number %d, want %d", i, p.Kind, p.Number, i+1)
		}
	}
	again := take(t, w)
	for _, p := range again.Pending {
		if p.New {
			t.Errorf("%v announced twice in one session", p.Kind)
		}
	}
}

// TestTakeoverRefusals removes each takeover condition that the state order
// would otherwise hide, by asking takeoverProposal directly: classify returns
// earlier for a running agent or an open PR, so the proposal's own checks
// are reached only here.
func TestTakeoverRefusals(t *testing.T) {
	task := taskID(t, "0001")
	cmd, err := paneinput.NewRunplan(task, paneinput.RouteRunplanOpus)
	if err != nil {
		t.Fatal(err)
	}
	routing, err := ParseRouting([]byte(strings.Replace(string(repoContext(t)), "| `runplan-opus` | `/runplan` for the requirement `opus` | Claude Code | `opus` |", "| `runplan-opus` | `/runplan` for the requirement `opus` | OpenCode | `DeepSeek V4.1 Flash 1017 OpenCode Go` |", 1)))
	if err != nil {
		t.Fatal(err)
	}
	next := &Next{Kind: TaskNextRunplan, Text: "/runplan 0001", Command: &cmd, Agent: paneinput.AgentOpenCode, Key: "2"}
	agent := func(kind paneinput.AgentKind, status AgentStatus) agentFacts {
		return agentFacts{Agent: Agent{Kind: kind, Status: status}}
	}
	base := func() workspaceFacts {
		return workspaceFacts{
			Task:     task,
			Agents:   []agentFacts{agent(paneinput.AgentOpenCode, AgentIdle), agent(paneinput.AgentClaudeCode, AgentIdle)},
			Routing:  routing,
			Takeover: takeoverFacts{Read: true, Ahead: 1, StepsDone: true},
		}
	}
	if takeoverProposal(base(), next) == nil {
		t.Fatal("the base facts do not give a proposal; the removals below prove nothing")
	}
	removals := map[string]func(f *workspaceFacts){
		"an open PR":           func(f *workspaceFacts) { f.PR = &OpenPR{Number: 1} },
		"a working agent":      func(f *workspaceFacts) { f.Agents[0] = agent(paneinput.AgentOpenCode, AgentWorking) },
		"no Claude Code agent": func(f *workspaceFacts) { f.Agents = f.Agents[:1] },
		"facts not read":       func(f *workspaceFacts) { f.Takeover.Read = false },
		"no commits ahead":     func(f *workspaceFacts) { f.Takeover.Ahead = 0 },
		"steps not done":       func(f *workspaceFacts) { f.Takeover.StepsDone = false },
		"something unreadable": func(f *workspaceFacts) { f.Unreadable = []string{"x"} },
	}
	for name, remove := range removals {
		f := base()
		remove(&f)
		if got := takeoverProposal(f, next); got != nil {
			t.Errorf("proposal made without %s: %+v", name, got)
		}
	}
	if got := takeoverProposal(base(), next); got.Key != "2" {
		t.Errorf("the takeover lost the next PR's key: %+v", got)
	}
}

// TestTakeoverReadFailures checks that a failed read behind the takeover
// proposal makes the workspace need a check instead of meaning "no takeover".
func TestTakeoverReadFailures(t *testing.T) {
	boom := errors.New("boom")
	for name, breakIt := range map[string]func(w *world, tree string){
		"commits ahead": func(w *world, tree string) {
			w.rec.failIn(tree, "git rev-list --count refs/remotes/origin/main..HEAD", boom)
		},
		"the branch plan": func(w *world, tree string) {
			w.rec.failIn(tree, "git show HEAD:docs/tasks/"+taskDir+"/"+planFile, boom)
		},
	} {
		t.Run(name, func(t *testing.T) {
			w, tree := takeoverSetup(t)
			breakIt(w, tree)
			r := rowOf(t, take(t, w), "ws")
			if r.State != StateNeedsCheck || len(r.Unreadable) != 1 || r.Takeover != nil {
				t.Errorf("row = %+v, want needs check with one reason", r)
			}
		})
	}
}

// TestFetchUsesItsOwnTimeLimit checks that the network fetch gets the longer
// limit and the local reads the shorter one.
func TestFetchUsesItsOwnTimeLimit(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: "main"})
	take(t, w)
	for _, c := range w.rec.recorded() {
		long := c.line() == "git fetch origin main"
		if long != (c.limit > gitTimeout) {
			t.Errorf("%s ran with a limit of %v", c.line(), c.limit)
		}
	}
}

// TestRowOrderRules checks the order of rows by state, by waiting time inside
// one state, and plain questions before merge confirmations; a workspace with
// no task that waits on nothing (a /mkreq proposal, a detached HEAD) lists
// after every other row, while one with no task that asks a question, or
// whose pane is blocked on a screen of no known shape, does not; the idle
// workspace's /mkreq proposal lists after the other proposal.
func TestRowOrderRules(t *testing.T) {
	w := newWorld(t)
	// The earlier sighting has the later path, so only the waiting time puts it first.
	w.add(fakeWorkspace{ID: "w9", Label: "review-old", Branch: taskBranch, PRs: openPRJSON})
	w.add(fakeWorkspace{ID: "w5", Label: "needs-check", Branch: taskBranch})
	w.add(fakeWorkspace{ID: "wa", Label: "no-task-blocked", Branch: "main", Status: "blocked", Screen: "claude-idle.ansi"})
	w.add(fakeWorkspace{ID: "w6", Label: "done", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-all-merged.md")}})
	w.add(fakeWorkspace{ID: "w4", Label: "running", Branch: taskBranch, Status: "working"})
	w.add(fakeWorkspace{ID: "w3", Label: "proposal", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved}})
	w.add(fakeWorkspace{ID: "w0", Label: "idle-mkreq", Branch: "main"})
	w.add(fakeWorkspace{ID: "w2", Label: "idle-detached", Branch: detachedHead})
	w.add(fakeWorkspace{ID: "w7", Label: "merge", Branch: taskBranch, Status: "blocked", Screen: "claude-merge-question.ansi"})
	take(t, w)
	w.now = w.now.Add(time.Hour)
	w.add(fakeWorkspace{ID: "w1", Label: "review-new", Branch: taskBranch, PRs: openPRJSON})
	w.add(fakeWorkspace{ID: "w8", Label: "question", Branch: "main", Status: "blocked", Screen: "claude-question-single.ansi"})
	snap := take(t, w)
	var labels []string
	for _, r := range snap.Rows {
		labels = append(labels, r.Label)
	}
	want := []string{"question", "merge", "review-old", "review-new", "proposal", "needs-check", "no-task-blocked", "running", "done", "idle-mkreq", "idle-detached"}
	if !slices.Equal(labels, want) {
		t.Errorf("row order = %v, want %v", labels, want)
	}
	// Both proposals were first seen together and the idle one has the
	// lower path, so only the idle rank lists it after the other.
	var proposals []string
	for _, p := range snap.Pending {
		if p.Kind == PendingProposal {
			proposals = append(proposals, p.Label)
		}
	}
	if want := []string{"proposal", "idle-mkreq"}; !slices.Equal(proposals, want) {
		t.Errorf("proposal order = %v, want %v", proposals, want)
	}
}

// TestDetachedRowIdleOnlyWhenEverythingWasRead checks that a row whose only
// problem is a detached HEAD is idle, and is not when the fetch of
// origin/main failed too.
func TestDetachedRowIdleOnlyWhenEverythingWasRead(t *testing.T) {
	for _, tc := range []struct {
		name        string
		fetchFailed bool
		want        bool
	}{
		{"fetched", false, true},
		{"fetch failed", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := classify(workspaceFacts{Head: headDetached, FetchFailed: tc.fetchFailed})
			if r.State != StateNeedsCheck || r.Idle != tc.want {
				t.Errorf("row = %+v, want needs check with Idle %v", r, tc.want)
			}
		})
	}
}

// TestClassifyZeroFacts checks classify's contract that facts nobody read
// assume nothing: no HEAD read gives no next command.
func TestClassifyZeroFacts(t *testing.T) {
	if r := classify(workspaceFacts{}); r.State != StateNeedsCheck || r.Next != nil {
		t.Errorf("zero facts = %+v, want StateNeedsCheck with no command", r)
	}
}

// TestRunningRowShowsTheLastSend checks that a running row shows the command
// and model of chief's last send to the working pane, that a row chief sent
// nothing shows none, and that a stopped workspace's row carries its stop.
func TestRunningRowShowsTheLastSend(t *testing.T) {
	w := newWorld(t)
	sent := w.add(fakeWorkspace{ID: "w1", Label: "sent", Branch: taskBranch, Status: "working"})
	w.add(fakeWorkspace{ID: "w2", Label: "unsent", Branch: taskBranch, Status: "working"})
	records, err := w.store.Records(Worktree{path: sent})
	if err != nil {
		t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		t.Fatal(err)
	}
	for i, cmd := range []string{"/mkplan 0001", "/runplan 0001"} {
		id, err := held.BeginSend(SendRecord{Pane: "w1:p1", Command: cmd, Agent: "claude_code", Model: "opus"}, w.now.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if err := held.EndSend(id, OutcomeSent.String(), "", w.now); err != nil {
			t.Fatal(err)
		}
	}
	if err := held.Stop("why", "", "", w.now); err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	snap := take(t, w)
	row := rowOf(t, snap, "sent")
	if row.State != StateRunning || row.Running == nil || row.Running.Command != "/runplan 0001" || row.Running.Model != "opus" {
		t.Errorf("sent row = %+v, running %+v; want the last send shown", row, row.Running)
	}
	if row.Stopped == nil || row.Stopped.Reason != "why" {
		t.Errorf("sent row stop = %+v, want the stop", row.Stopped)
	}
	if other := rowOf(t, snap, "unsent"); other.Running != nil || other.Stopped != nil {
		t.Errorf("unsent row = %+v; want nothing shown", other)
	}
	if len(snap.Problems) != 0 {
		t.Errorf("problems = %v", snap.Problems)
	}
}

// TestIdleClearedByWhatTheHumanMustSee checks that an idle row stops being
// idle when its workspace gets something to look at after classify: each
// outcome of an automatic send, its proposal, and a stop of the workspace.
// A stopped row then lists before a running row instead of after it.
func TestIdleClearedByWhatTheHumanMustSee(t *testing.T) {
	proposal := &Next{Kind: TaskNextRunplan, Text: "/runplan 0001"}
	for _, tc := range []struct {
		name   string
		result AutoSendResult
	}{
		{"sent", AutoSendResult{Workspace: "/idle", Actions: []ActionResult{{Input: InputCommand, Outcome: OutcomeSent}}}},
		{"sent then failed", AutoSendResult{Workspace: "/idle", Actions: []ActionResult{{Input: InputCommand, Outcome: OutcomeSentThenFailed}}}},
		{"a proposal", AutoSendResult{Workspace: "/idle", Proposal: proposal}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := roundView{rows: []Row{
				{Workspace: "/idle", State: StateNeedsCheck, Idle: true},
				{Workspace: "/busy", Task: "0002", State: StateRunning},
			}}
			applyAutoSends(&view, []AutoSendResult{tc.result})
			if view.rows[0].Idle {
				t.Fatalf("row after %s is still idle", tc.name)
			}
		})
	}
	t.Run("a stop that cannot be read", func(t *testing.T) {
		w := newWorld(t)
		tree := w.add(fakeWorkspace{ID: "w1", Label: "unread", Branch: "main"})
		dir, err := w.store.WorkspaceDir(Worktree{path: tree})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := privatefile.Create(filepath.Join(dir, stopFile), []byte("not json")); err != nil {
			t.Fatal(err)
		}
		snap, err := TakeSnapshot(context.Background(), w.finish(), w.owner())
		if err != nil {
			t.Fatal(err)
		}
		if row := rowOf(t, snap, "unread"); row.Idle {
			t.Errorf("row with an unreadable stop = %+v, want not idle", row)
		}
	})
	t.Run("a stop", func(t *testing.T) {
		w := newWorld(t)
		tree := w.add(fakeWorkspace{ID: "w1", Label: "stopped", Branch: "main"})
		records, err := w.store.Records(Worktree{path: tree})
		if err != nil {
			t.Fatal(err)
		}
		held, err := records.Lock()
		if err != nil {
			t.Fatal(err)
		}
		if err := held.Stop("why", "", "", w.now); err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
		w.add(fakeWorkspace{ID: "w2", Label: "running", Branch: taskBranch, Status: "working"})
		snap := take(t, w)
		if row := rowOf(t, snap, "stopped"); row.Idle {
			t.Errorf("stopped row = %+v, want not idle", row)
		}
		if last := snap.Rows[len(snap.Rows)-1]; last.Label != "running" {
			t.Errorf("last row = %s, want the running row after the stopped one", last.Label)
		}
	})
}
