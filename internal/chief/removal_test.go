//go:build test && unix

package chief

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The values of the removal worlds: the workspace w2 was made for issue
// removalIssue, whose task removalTaskDir is done on origin/main.
const (
	removalIssue   = "839"
	removalTaskDir = "0005_done_task"
	removalTask    = "0005"
	removalWS      = "w2"
	removalLabel   = "issue-839"
)

// removalSetup is a workspace chief made for removalIssue, added to a send
// world: a Claude Code pane (p1), an OpenCode pane (p2), and a shell pane
// (p3) at its worktree on a detached HEAD at origin/main, and origin/main
// with exactly one task of the issue, whose plan has every PR merged. Every
// condition of a removal proposal holds, and so does every check.
type removalSetup struct {
	sw   *sendWorld
	tree string
	// others are panes of other workspaces the pane list adds.
	others []fakePane
	// closed is the workspace closed: its panes are gone.
	closed bool
	// during, when set, runs inside each write allowRemoval allows, before
	// the write changes the fake.
	during func()
}

// issueLineOf is one line of the issue line search for a task directory.
func issueLineOf(dir, issue string) string {
	return originMain + ":" + taskRoot + "/" + dir + "/" + requirementsFile + ":20:" + issueLinePrefix + "[#" + issue + "](https://github.com/owner/repo/issues/" + issue + ")\n"
}

// addRemovalWorkspace adds the workspace to sw, before sw.finish, with its
// created record kept.
func addRemovalWorkspace(t *testing.T, sw *sendWorld) *removalSetup {
	t.Helper()
	rs := &removalSetup{sw: sw}
	rs.tree = sw.addTask(fakeWorkspace{ID: removalWS, Label: removalLabel, Branch: detachedHead, Tree: filepath.Join(taskSpaceRoot[0], taskSpaceRoot[1], taskSpaceRoot[2], removalLabel)}, "claude", "opencode")
	rs.keepCreated(removalIssue)
	sw.rec.replyFunc("herdr pane list", func() reply { return reply{out: paneListOutput(rs.paneList())} })
	sw.rec.reply("herdr pane process-info --pane "+removalWS+":p3", processInfoOutput(removalWS+":p3", 100, 100, 100))
	rs.issueLines(issueLineOf(removalTaskDir, removalIssue))
	sw.docs(sw.chiefTree, fakeDocs{Dir: removalTaskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-all-merged.md")})
	sw.git(rs.tree, "status --porcelain --untracked-files=all", "")
	sw.git(rs.tree, "rev-parse HEAD", mainCommit+"\n")
	sw.git(rs.tree, "rev-parse "+originMain, mainCommit+"\n")
	sw.git(rs.tree, "merge-base --is-ancestor "+mainCommit+" "+mainCommit, "")
	rs.worktrees(false)
	return rs
}

// paneList is Herdr's pane list: chief's pane, every agent pane, the
// workspace's shell pane until it is closed, and the others.
func (rs *removalSetup) paneList() []fakePane {
	sw := rs.sw
	sw.mu.Lock()
	defer sw.mu.Unlock()
	panes := []fakePane{{Pane: sw.chief.Pane, Tab: "wc:t1", Workspace: sw.chief.Workspace, Dir: sw.chief.Dir, Agent: sw.chief.Kind}}
	for _, p := range sw.panes {
		panes = append(panes, fakePane{Pane: p.Pane, Tab: strings.Replace(p.Pane, ":p", ":t", 1), Workspace: p.Workspace, Dir: p.Dir, Agent: p.Kind})
	}
	if !rs.closed {
		panes = append(panes, fakePane{Pane: removalWS + ":p3", Tab: removalWS + ":t3", Workspace: removalWS, Dir: rs.tree})
	}
	return append(panes, rs.others...)
}

// issueLines sets what the issue line search finds on origin/main.
func (rs *removalSetup) issueLines(out string) {
	rs.sw.rec.replyIn(rs.sw.chiefTree, gitProgram+" "+strings.Join(issueLineSearch, " "), []byte(out))
}

// worktrees sets git's worktree list: chief's and the workspace's, the
// latter locked when locked is true.
func (rs *removalSetup) worktrees(locked bool) {
	lock := ""
	if locked {
		lock = "locked\n"
	}
	rs.sw.git(rs.sw.chiefTree, "worktree list --porcelain", "worktree "+rs.sw.chiefTree+"\nHEAD "+mainCommit+"\nbranch refs/heads/main\n\nworktree "+rs.tree+"\nHEAD "+mainCommit+"\ndetached\n"+lock+"\n")
}

// held takes the exclusion of the workspace's records.
func (rs *removalSetup) held() *HeldWorkspace {
	rs.sw.t.Helper()
	records, err := rs.sw.store.Records(Worktree{path: rs.tree})
	if err != nil {
		rs.sw.t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		rs.sw.t.Fatal(err)
	}
	rs.sw.t.Cleanup(func() { _ = held.Close() })
	return held
}

// keepCreated keeps the created record of the workspace for issue.
func (rs *removalSetup) keepCreated(issue string) {
	rs.sw.t.Helper()
	space, err := NewTaskSpace(rs.sw.env.Home, issueNumber(rs.sw.t, issue))
	if err != nil {
		rs.sw.t.Fatal(err)
	}
	held := rs.held()
	if err := held.ClearRecords(); err != nil {
		rs.sw.t.Fatal(err)
	}
	if err := held.KeepCreated(space, rs.sw.now); err != nil {
		rs.sw.t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		rs.sw.t.Fatal(err)
	}
}

// forgeCreated replaces the created record with one holding raw, written
// as KeepCreated writes a record but with none of its checks.
func (rs *removalSetup) forgeCreated(raw createdJSON) {
	rs.sw.t.Helper()
	held := rs.held()
	if err := held.ClearRecords(); err != nil {
		rs.sw.t.Fatal(err)
	}
	if err := held.store().create(createdFile, raw); err != nil {
		rs.sw.t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		rs.sw.t.Fatal(err)
	}
}

// createdPath is the path of the workspace's created record.
func (rs *removalSetup) createdPath() string {
	return filepath.Join(rs.sw.store.Dir(), workspaceDirOf(Worktree{path: rs.tree}), createdFile)
}

// newRemovalWorld is a send world holding only the removal workspace.
func newRemovalWorld(t *testing.T) (*sendWorld, *removalSetup) {
	t.Helper()
	sw := newSendWorld(t)
	return sw, addRemovalWorkspace(t, sw)
}

// snapshotOf takes a snapshot of the world.
func snapshotOf(t *testing.T, sw *sendWorld) Snapshot {
	t.Helper()
	snap, err := TakeSnapshot(context.Background(), sw.finish(), sw.owner())
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// removalItems returns the removal items of a snapshot.
func removalItems(snap Snapshot) []PendingItem {
	return slices.DeleteFunc(slices.Clone(snap.Pending), func(p PendingItem) bool { return p.Kind != PendingRemoval })
}

// allPassed reports whether every check is present once, in order, and
// passed.
func allPassed(checks []CheckResult) bool {
	if len(checks) != len(removalChecks) {
		return false
	}
	for i, c := range checks {
		if c.Check != removalChecks[i] || !c.Passed || len(c.Unreadable) != 0 {
			return false
		}
	}
	return true
}

// TestRemovalRow checks the row of a finished task's workspace chief made:
// the row shows the task done, the issue's task, and the removal proposed,
// with one removal item whose checks all pass.
func TestRemovalRow(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	snap := snapshotOf(t, sw)
	r := rowOf(t, snap, removalLabel)
	if r.State != StateTaskDone || r.Task != removalTask || !r.RemovalRow || r.Idle || r.Next != nil || len(r.Unreadable) != 0 {
		t.Errorf("row = %+v, want the task done with the removal proposed", r)
	}
	if r.Removal == nil || r.Removal.Workspace != rs.tree || r.Removal.Issue.String() != removalIssue || r.Removal.Task.String() != removalTask || r.Removal.Panes != 3 || !allPassed(r.Removal.Checks) {
		t.Errorf("removal = %+v", r.Removal)
	}
	items := removalItems(snap)
	if len(items) != 1 || items[0].Removal == nil || items[0].Removal.Issue != r.Removal.Issue || items[0].Removal.Task != r.Removal.Task || !allPassed(items[0].Removal.Checks) || items[0].Workspace != rs.tree || snap.Pending[len(snap.Pending)-1].ID != items[0].ID {
		t.Errorf("pending = %+v, want one removal item, listed last", snap.Pending)
	}
	if len(snap.Problems) != 0 {
		t.Errorf("problems = %v", snap.Problems)
	}
}

// TestRemovalRowFallsBack takes away one condition of the removal row that
// a proposal does not need, at a time: the row is then 0088's, and the
// removal is still proposed with the check that does not pass.
func TestRemovalRowFallsBack(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(sw *sendWorld, rs *removalSetup)
		state  WorkspaceState
		failed RemovalCheck
	}{
		{"HEAD on a branch", func(sw *sendWorld, rs *removalSetup) {
			sw.git(rs.tree, "rev-parse --abbrev-ref HEAD", "main\n")
			sw.rec.replyIn(rs.tree, "gh pr list --head main --state open --json headRefName,number,title,url", []byte("[]"))
		}, StateProposeNext, RemovalCheckDetached},
		{"an agent working", func(sw *sendWorld, _ *removalSetup) {
			p := sw.byPane[removalWS+":p1"]
			p.Status, p.screen = "working", readScreenSample(t, "claude-working.ansi")
		}, StateRunning, RemovalCheckAgentsIdle},
		{"HEAD on a branch by the time of the checks", func(sw *sendWorld, _ *removalSetup) {
			// The round reads HEAD detached, the checks read it on a branch.
			reads := 0
			sw.rec.replyFunc("git rev-parse --abbrev-ref HEAD", func() reply {
				reads++
				if reads == 1 {
					return reply{out: []byte(detachedHead + "\n")}
				}
				return reply{out: []byte("main\n")}
			})
		}, StateNeedsCheck, RemovalCheckDetached},
		{"an agent asking", func(sw *sendWorld, _ *removalSetup) {
			sw.ask(sw.byPane[removalWS+":p1"], "claude-question-single.ansi")
		}, StateQuestion, RemovalCheckAgentsIdle},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			c.edit(sw, rs)
			snap := snapshotOf(t, sw)
			r := rowOf(t, snap, removalLabel)
			if r.State != c.state || r.RemovalRow || r.Task != "" {
				t.Errorf("row = %+v, want 0088's %v row", r, c.state)
			}
			if r.Removal == nil || len(removalItems(snap)) != 1 {
				t.Fatalf("removal = %+v, pending = %+v; want the removal still proposed", r.Removal, snap.Pending)
			}
			for _, check := range r.Removal.Checks {
				if check.Passed == (check.Check == c.failed) {
					t.Errorf("check %v passed = %v", check.Check, check.Passed)
				}
			}
		})
	}
}

// TestRemovalCandidateConditions takes away one condition of a removal
// proposal at a time: the workspace is then not proposed, and its row is
// 0088's detached one.
func TestRemovalCandidateConditions(t *testing.T) {
	notDone := strings.Replace(string(planSample(t, "marker-all-merged.md")), "| Status | `approved` |", "| Status | `draft` |", 1)
	cases := map[string]func(t *testing.T, sw *sendWorld, rs *removalSetup){
		"no created record": func(t *testing.T, _ *sendWorld, rs *removalSetup) {
			if err := os.Remove(rs.createdPath()); err != nil {
				t.Fatal(err)
			}
		},
		"another workspace at the path": func(_ *testing.T, _ *sendWorld, rs *removalSetup) {
			rs.others = []fakePane{{Pane: "w9:p1", Tab: "w9:t1", Workspace: "w9", Dir: filepath.Join(rs.tree, "sub")}}
		},
		"only another workspace at the path": func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			// The agents stay in w2, but Herdr lists every pane at the path
			// under another workspace.
			sw.rec.replyFunc("herdr pane list", func() reply {
				return reply{out: paneListOutput([]fakePane{{Pane: "w9:p1", Tab: "w9:t1", Workspace: "w9", Dir: rs.tree}})}
			})
		},
		"no task of the issue": func(_ *testing.T, _ *sendWorld, rs *removalSetup) {
			rs.issueLines(issueLineOf(removalTaskDir, "840"))
		},
		"two tasks of the issue": func(_ *testing.T, _ *sendWorld, rs *removalSetup) {
			rs.issueLines(issueLineOf(removalTaskDir, removalIssue) + issueLineOf("0006_other_task", removalIssue))
		},
		"the task not done": func(t *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.docs(sw.chiefTree, fakeDocs{Dir: removalTaskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")})
		},
		"the plan not approved": func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.docs(sw.chiefTree, fakeDocs{Dir: removalTaskDir, Req: reqApproved, Arch: archApproved, Plan: []byte(notDone)})
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			edit(t, sw, rs)
			snap := snapshotOf(t, sw)
			assertNotProposed(t, snap)
		})
	}
}

// assertNotProposed checks that the removal workspace has 0088's detached
// row and no removal item.
func assertNotProposed(t *testing.T, snap Snapshot) {
	t.Helper()
	r := rowOf(t, snap, removalLabel)
	if r.State != StateNeedsCheck || r.RemovalRow || r.Removal != nil || r.Task != "" {
		t.Errorf("row = %+v, want 0088's detached row", r)
	}
	if items := removalItems(snap); len(items) != 0 {
		t.Errorf("removal items = %+v", items)
	}
}

// TestRemovalCandidateRefusals checks the workspaces that are not proposed
// although a record is there: a record that cannot be read, a forged record
// naming another worktree or a worktree its issue's task is not made at,
// and a read of origin/main or Herdr that fails. Each is named among the
// problems, and none is proposed.
func TestRemovalCandidateRefusals(t *testing.T) {
	type refusal struct {
		edit func(t *testing.T, sw *sendWorld, rs *removalSetup)
		// named is the part of the problem that names the read that failed;
		// the problem's wording is what the output shows the human.
		named string
	}
	cases := map[string]refusal{
		"a record that is not JSON": {func(t *testing.T, _ *sendWorld, rs *removalSetup) {
			if err := os.WriteFile(rs.createdPath(), []byte("not json"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "created record of"},
		"a record naming another worktree": {func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			rs.forgeCreated(createdJSON{Worktree: sw.chiefTree, Issue: removalIssue, Created: sw.now})
		}, "created record of"},
		"a record of another issue": {func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			rs.forgeCreated(createdJSON{Worktree: rs.tree, Issue: "840", Created: sw.now})
		}, "created record of"},
		"the issue lines cannot be read": {func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.rec.failIn(sw.chiefTree, gitProgram+" "+strings.Join(issueLineSearch, " "), &cmdrunner.ExitError{Code: 128})
		}, "issue lines of origin/main"},
		"the task's documents cannot be read": {func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.rec.failIn(sw.chiefTree, "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/", errInjected)
		}, "the documents of"},
		"the pane list cannot be read": {func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.rec.replyFunc("herdr pane list", func() reply { return reply{err: errInjected} })
		}, "herdr pane list"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			c.edit(t, sw, rs)
			snap := snapshotOf(t, sw)
			assertNotProposed(t, snap)
			if !slices.ContainsFunc(snap.Problems, func(p string) bool {
				return strings.Contains(p, c.named) && strings.Contains(p, "not offered for removal")
			}) {
				t.Errorf("problems = %v, want the failed read named", snap.Problems)
			}
		})
	}
}

// TestRemovalRunsNothingWithoutARecord checks that a round with no
// readable created record runs neither the pane list nor the issue line
// search, and that a record that cannot be read runs neither either.
func TestRemovalRunsNothingWithoutARecord(t *testing.T) {
	search := gitProgram + " " + strings.Join(issueLineSearch, " ")
	for _, broken := range []bool{false, true} {
		sw, rs := newRemovalWorld(t)
		var err error
		if broken {
			err = os.WriteFile(rs.createdPath(), []byte("{}"), 0o600)
		} else {
			err = os.Remove(rs.createdPath())
		}
		if err != nil {
			t.Fatal(err)
		}
		snapshotOf(t, sw)
		for _, c := range sw.rec.recorded() {
			if c.line() == "herdr pane list" || c.line() == search || (c.name == herdrProgram && len(c.args) > 1 && c.args[1] == "process-info") {
				t.Errorf("broken record %v: ran %s", broken, c.line())
			}
		}
	}
}

// TestRemovalProposal checks a removal item's output: the workspace, label,
// issue, task, pane count, and every check in order, with a check that did
// not pass and what could not be read, and the row's mark.
func TestRemovalProposal(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	sw.git(rs.tree, "status --porcelain --untracked-files=all", "?? notes.txt\n")
	sw.rec.reply("herdr pane process-info --pane "+removalWS+":p3", []byte("{}"))
	data, err := json.Marshal(snapshotOf(t, sw).Output())
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Rows []struct {
			Label           string `json:"label"`
			State           string `json:"state"`
			Task            string `json:"task"`
			ProposesRemoval bool   `json:"proposes_removal"`
		} `json:"rows"`
		Pending []struct {
			Kind    string         `json:"kind"`
			Removal *RemovalOutput `json:"removal"`
		} `json:"pending"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Rows) != 1 || out.Rows[0].State != "task_done" || out.Rows[0].Task != removalTask || !out.Rows[0].ProposesRemoval {
		t.Errorf("rows = %+v", out.Rows)
	}
	i := slices.IndexFunc(out.Pending, func(p struct {
		Kind    string         `json:"kind"`
		Removal *RemovalOutput `json:"removal"`
	},
	) bool {
		return p.Kind == "removal"
	})
	if i < 0 || out.Pending[i].Removal == nil {
		t.Fatalf("pending = %s", data)
	}
	got := out.Pending[i].Removal
	if got.Workspace != rs.tree || got.Label != removalLabel || got.Issue != removalIssue || got.Task != removalTask || got.Panes != 3 {
		t.Errorf("removal = %+v", got)
	}
	want := []RemovalCheckOutput{
		{Check: "not_candidate", Passed: true, Unreadable: []string{}},
		{Check: "head_on_branch", Passed: true, Unreadable: []string{}},
		{Check: "not_clean", Passed: false, Unreadable: []string{}},
		{Check: "head_not_on_main", Passed: true, Unreadable: []string{}},
		{Check: "worktree_locked", Passed: true, Unreadable: []string{}},
		{Check: "agent_active", Passed: true, Unreadable: []string{}},
		{Check: "shell_busy", Passed: false, Unreadable: []string{"the processes of pane w2:p3"}},
		{Check: "path_in_use", Passed: true, Unreadable: []string{}},
	}
	if len(got.Checks) != len(want) {
		t.Fatalf("checks = %+v", got.Checks)
	}
	for i := range want {
		if got.Checks[i].Check != want[i].Check || got.Checks[i].Passed != want[i].Passed || !slices.Equal(got.Checks[i].Unreadable, want[i].Unreadable) {
			t.Errorf("check %d = %+v, want %+v", i, got.Checks[i], want[i])
		}
	}
}

// checkWorld is a recorder with every read of the removal checks of a
// candidate at /work/tree answered so that every check passes.
type checkWorld struct {
	rec    *recorder
	tree   Worktree
	chief  Worktree
	agents []agentFacts
	panes  []listedPane
}

const checkCommit = "2222222222222222222222222222222222222222"

func newCheckWorld(t *testing.T) *checkWorld {
	t.Helper()
	cw := &checkWorld{rec: newRecorder(t), tree: Worktree{path: "/work/tree"}, chief: Worktree{path: "/work/chief"}}
	tree := cw.tree.path
	cw.rec.replyIn(tree, "git rev-parse --abbrev-ref HEAD", []byte("HEAD\n"))
	cw.rec.replyIn(tree, "git status --porcelain --untracked-files=all", nil)
	cw.rec.replyIn(tree, "git rev-parse HEAD", []byte(checkCommit+"\n"))
	cw.rec.replyIn(tree, "git rev-parse "+originMain, []byte(checkCommit+"\n"))
	cw.rec.replyIn(tree, "git merge-base --is-ancestor "+checkCommit+" "+checkCommit, nil)
	cw.worktrees("")
	cw.rec.reply("herdr pane process-info --pane w2:p3", processInfoOutput("w2:p3", 100, 100, 100))
	cw.agents = []agentFacts{
		{Agent: Agent{Pane: PaneID{id: "w2:p1"}, Workspace: removalWS, Status: AgentIdle, Dir: tree}, ScreenRead: true, Screen: Screen{Kind: ScreenPrompt}},
		{Agent: Agent{Pane: PaneID{id: "w2:p2"}, Workspace: removalWS, Status: AgentDone, Dir: tree}, ScreenRead: true, Screen: Screen{Kind: ScreenPrompt}},
	}
	cw.panes = []listedPane{
		{ID: PaneID{id: "w2:p1"}, Workspace: removalWS, Dir: tree, HasAgent: true},
		{ID: PaneID{id: "w2:p2"}, Workspace: removalWS, Dir: tree, HasAgent: true},
		{ID: PaneID{id: "w2:p3"}, Workspace: removalWS, Dir: tree},
		{ID: PaneID{id: "wc:p1"}, Workspace: "wc", Dir: cw.chief.path, HasAgent: true},
	}
	return cw
}

// worktrees sets git's worktree list with extra lines in the candidate's
// block.
func (cw *checkWorld) worktrees(extra string) {
	cw.rec.replyIn(cw.chief.path, "git worktree list --porcelain", []byte("worktree "+cw.chief.path+"\nHEAD "+checkCommit+"\n\nworktree "+cw.tree.path+"\nHEAD "+checkCommit+"\ndetached\n"+extra+"\n"))
}

func (cw *checkWorld) check(t *testing.T) []CheckResult {
	t.Helper()
	c := newRemovalCandidate(cw.tree, removalWS, issueNumber(t, removalIssue), taskID(t, removalTask))
	checks, _ := checkRemoval(context.Background(), cw.rec.commands(nil), cw.chief, c, cw.agents, cw.panes)
	return checks
}

// TestRemovalChecks breaks one check at a time, by a value that fails it and
// by a read that cannot be made, and checks that only that check does not
// pass, with what could not be read named only for an unreadable one.
func TestRemovalChecks(t *testing.T) {
	if got := newCheckWorld(t).check(t); !allPassed(got) {
		t.Fatalf("the passing world gives %+v", got)
	}
	exit := func(code int) error { return &cmdrunner.ExitError{Code: code} }
	cases := []struct {
		name       string
		check      RemovalCheck
		unreadable bool
		edit       func(cw *checkWorld)
	}{
		{"HEAD on a branch", RemovalCheckDetached, false, func(cw *checkWorld) {
			cw.rec.replyIn(cw.tree.path, "git rev-parse --abbrev-ref HEAD", []byte("main\n"))
		}},
		{"the branch cannot be read", RemovalCheckDetached, true, func(cw *checkWorld) {
			cw.rec.failIn(cw.tree.path, "git rev-parse --abbrev-ref HEAD", errInjected)
		}},
		{"a change", RemovalCheckClean, false, func(cw *checkWorld) {
			cw.rec.replyIn(cw.tree.path, "git status --porcelain --untracked-files=all", []byte(" M f\n"))
		}},
		{"the status cannot be read", RemovalCheckClean, true, func(cw *checkWorld) {
			cw.rec.failIn(cw.tree.path, "git status --porcelain --untracked-files=all", errInjected)
		}},
		{"HEAD not on origin/main", RemovalCheckHeadOnMain, false, func(cw *checkWorld) {
			cw.rec.failIn(cw.tree.path, "git merge-base --is-ancestor "+checkCommit+" "+checkCommit, exit(1))
		}},
		{"the ancestry cannot be read", RemovalCheckHeadOnMain, true, func(cw *checkWorld) {
			cw.rec.failIn(cw.tree.path, "git merge-base --is-ancestor "+checkCommit+" "+checkCommit, exit(128))
		}},
		{"HEAD cannot be read", RemovalCheckHeadOnMain, true, func(cw *checkWorld) {
			cw.rec.failIn(cw.tree.path, "git rev-parse HEAD", errInjected)
		}},
		{"origin/main cannot be read", RemovalCheckHeadOnMain, true, func(cw *checkWorld) {
			cw.rec.failIn(cw.tree.path, "git rev-parse "+originMain, errInjected)
		}},
		{"a locked worktree", RemovalCheckUnlocked, false, func(cw *checkWorld) { cw.worktrees("locked kept\n") }},
		{"no registration", RemovalCheckUnlocked, true, func(cw *checkWorld) {
			cw.rec.replyIn(cw.chief.path, "git worktree list --porcelain", []byte("worktree "+cw.chief.path+"\nHEAD "+checkCommit+"\n\n"))
		}},
		{"the worktree list cannot be read", RemovalCheckUnlocked, true, func(cw *checkWorld) {
			cw.rec.failIn(cw.chief.path, "git worktree list --porcelain", errInjected)
		}},
		{"an agent working", RemovalCheckAgentsIdle, false, func(cw *checkWorld) { cw.agents[0].Agent.Status = AgentWorking }},
		{"an agent blocked", RemovalCheckAgentsIdle, false, func(cw *checkWorld) { cw.agents[0].Agent.Status = AgentBlocked }},
		{"a question on an idle agent's screen", RemovalCheckAgentsIdle, false, func(cw *checkWorld) {
			cw.agents[1].Screen = Screen{Kind: ScreenQuestion, Question: &Question{Text: "Which?"}}
		}},
		{"a screen not read", RemovalCheckAgentsIdle, true, func(cw *checkWorld) { cw.agents[0].ScreenRead = false }},
		{"a screen of no known shape", RemovalCheckAgentsIdle, true, func(cw *checkWorld) { cw.agents[0].Screen = Screen{} }},
		{"an agent the agent list lacks", RemovalCheckAgentsIdle, true, func(cw *checkWorld) { cw.agents = cw.agents[:1] }},
		{"a busy shell", RemovalCheckShellsIdle, false, func(cw *checkWorld) {
			cw.rec.reply("herdr pane process-info --pane w2:p3", processInfoOutput("w2:p3", 100, 200, 200))
		}},
		{"the processes cannot be read", RemovalCheckShellsIdle, true, func(cw *checkWorld) {
			cw.rec.fail("herdr pane process-info --pane w2:p3", errInjected)
		}},
		{"the processes of another pane", RemovalCheckShellsIdle, true, func(cw *checkWorld) {
			cw.rec.reply("herdr pane process-info --pane w2:p3", processInfoOutput("w2:p1", 100, 100, 100))
		}},
		{"another workspace at the path", RemovalCheckPathUnused, false, func(cw *checkWorld) {
			cw.panes = append(cw.panes, listedPane{ID: PaneID{id: "w9:p1"}, Workspace: "w9", Dir: cw.tree.path + "/sub"})
		}},
		{"only another workspace at the path", RemovalCheckPathUnused, false, func(cw *checkWorld) {
			for i := range cw.panes {
				if cw.panes[i].Workspace == removalWS {
					cw.panes[i].Dir = "/work/elsewhere"
				}
			}
			cw.panes = append(cw.panes, listedPane{ID: PaneID{id: "w9:p1"}, Workspace: "w9", Dir: cw.tree.path})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cw := newCheckWorld(t)
			c.edit(cw)
			got := cw.check(t)
			if len(got) != len(removalChecks) {
				t.Fatalf("checks = %+v", got)
			}
			for i, r := range got {
				broken := r.Check == c.check
				if r.Check != removalChecks[i] || r.Passed == broken || (len(r.Unreadable) > 0) != (broken && c.unreadable) {
					t.Errorf("check %v = %+v", removalChecks[i], r)
				}
			}
		})
	}
	t.Run("the zero candidate", func(t *testing.T) {
		cw := newCheckWorld(t)
		got, head := checkRemoval(context.Background(), cw.rec.commands(nil), cw.chief, RemovalCandidate{}, cw.agents, cw.panes)
		if head != "" {
			t.Errorf("head = %q for the zero candidate", head)
		}
		if len(got) != len(removalChecks) || slices.ContainsFunc(got, func(r CheckResult) bool { return r.Passed }) {
			t.Errorf("checks = %+v, want every check not passed", got)
		}
		if calls := cw.rec.recorded(); len(calls) != 0 {
			t.Errorf("ran %v", calls)
		}
	})
}

// TestRemovalRowUrgencyAndDocument checks the removal row in 0093's urgency
// table and document rule: each rule is checked first without the mark, so
// the result comes from the mark's row alone.
func TestRemovalRowUrgencyAndDocument(t *testing.T) {
	detached := positionFacts{head: headDetached}
	t.Run("urgency", func(t *testing.T) {
		row := Row{State: StateTaskDone, Idle: true}
		if u, turn := urgencyOf(row); u != UrgencyWait || turn != TurnNone {
			t.Fatalf("without the mark = %v, %v; want the idle row's wait", u, turn)
		}
		row.RemovalRow = true
		if u, turn := urgencyOf(row); u != UrgencyLater || turn != TurnHuman {
			t.Errorf("with the mark = %v, %v; want later, the human", u, turn)
		}
		row.Stopped = &StopRecord{Reason: "x"}
		if u, turn := urgencyOf(row); u != UrgencyNow || turn != TurnHuman {
			t.Errorf("stopped with the mark = %v, %v; want now, the human", u, turn)
		}
		running := Row{State: StateRunning, Removal: &RemovalProposal{}}
		if u, turn := urgencyOf(running); u != UrgencyWait || turn != TurnAI {
			t.Errorf("a proposal's running row without the mark = %v, %v; want 0093's running row", u, turn)
		}
	})
	t.Run("document", func(t *testing.T) {
		row := Row{State: StateTaskDone, position: detached}
		if doc, _ := documentOf(row, nil); doc != PositionDocNeedsCheck {
			t.Fatalf("without the mark = %v; want rule 10's needs check", doc)
		}
		row.RemovalRow = true
		if doc, _ := documentOf(row, nil); doc != PositionDocNone {
			t.Errorf("with the mark = %v; want none", doc)
		}
		row.position.fetchFailed = true
		if doc, _ := documentOf(row, nil); doc != PositionDocNeedsCheck {
			t.Errorf("with the mark after a failed fetch = %v; want rule 1's needs check", doc)
		}
	})
}

// TestRemovalRowSortsLater checks where 0093's row order puts a removal
// row: by its urgency, later, before a running row, which waits. Without
// the mark the row would be a finished task's, which waits too and whose
// state lists it after the running row.
func TestRemovalRowSortsLater(t *testing.T) {
	sw := newSendWorld(t)
	sw.addTask(fakeWorkspace{ID: "w1", Label: "running", Branch: taskBranch}, "claude")
	p := sw.byPane["w1:p1"]
	p.Status, p.screen = "working", readScreenSample(t, "claude-working.ansi")
	addRemovalWorkspace(t, sw)
	snap := snapshotOf(t, sw)
	var labels []string
	for _, r := range snap.Rows {
		labels = append(labels, r.Label)
	}
	if want := []string{removalLabel, "running"}; !slices.Equal(labels, want) {
		t.Errorf("row order = %v, want %v", labels, want)
	}
	if r := rowOf(t, snap, removalLabel); !r.RemovalRow || r.Urgency != UrgencyLater || r.Turn != TurnHuman || r.Position.Document != PositionDocNone {
		t.Errorf("removal row = %+v, want later, the human, no document", r)
	}
}

// TestCreatedWorkspaceRows checks a workspace chief made whose issue's task
// is not done: once /mkreq made the task's branch, the row is that task's,
// by 0088's rules, and nothing is proposed for removal.
func TestCreatedWorkspaceRows(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	sw.git(rs.tree, "rev-parse --abbrev-ref HEAD", taskBranch+"\n")
	sw.rec.replyIn(rs.tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", []byte("[]"))
	sw.docs(rs.tree, fakeDocs{Dir: taskDir, Req: reqApproved})
	rs.issueLines(issueLineOf(taskDir, removalIssue))
	sw.docs(sw.chiefTree, fakeDocs{Dir: taskDir, Req: reqApproved})
	snap := snapshotOf(t, sw)
	r := rowOf(t, snap, removalLabel)
	if r.State != StateProposeNext || r.Task != "0001" || r.Next == nil || r.Next.Kind != TaskNextMkarch || r.Removal != nil || r.RemovalRow {
		t.Errorf("row = %+v, want the task's /mkarch", r)
	}
	if items := removalItems(snap); len(items) != 0 {
		t.Errorf("removal items = %+v", items)
	}
}

// TestCreatedDetachedRowOffersNoMkreq checks a workspace chief made before
// /mkreq made its task: the detached row needs a check by 0088's rule and
// offers no command, /mkreq included, and no removal.
func TestCreatedDetachedRowOffersNoMkreq(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	sw.rec.failIn(sw.chiefTree, gitProgram+" "+strings.Join(issueLineSearch, " "), &cmdrunner.ExitError{Code: 1})
	snap := snapshotOf(t, sw)
	r := rowOf(t, snap, removalLabel)
	if r.State != StateNeedsCheck || r.Next != nil || r.Removal != nil {
		t.Errorf("row = %+v", r)
	}
	if len(snap.Pending) != 0 || len(snap.Problems) != 0 {
		t.Errorf("pending = %+v, problems = %v; want nothing", snap.Pending, snap.Problems)
	}
	if _, err := os.Stat(rs.createdPath()); err != nil {
		t.Errorf("the created record: %v", err)
	}
}

// TestRemovalLeavesRecordsOnSnapshotAndTargets runs a snapshot and the
// targets of an issue over a world with a removal proposal and checks that
// the state directory, the created record among it, is as it was.
func TestRemovalLeavesRecordsOnSnapshotAndTargets(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	deps := sw.finish()
	before := stateEntries(t, deps.Store.Dir())
	if !slices.ContainsFunc(before, func(p string) bool { return strings.HasSuffix(p, createdFile) }) {
		t.Fatalf("state directory %v has no created record", before)
	}
	snap, err := TakeSnapshot(context.Background(), deps, sw.owner())
	if err != nil || len(removalItems(snap)) != 1 {
		t.Fatalf("snapshot = %+v, %v; want the removal proposed", snap.Pending, err)
	}
	// The snapshot leaves the notice records of its items; the comparison
	// starts after it.
	before = stateEntries(t, deps.Store.Dir())
	if _, err := Targets(context.Background(), deps, createIssue); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeSnapshot(context.Background(), deps, sw.owner()); err != nil {
		t.Fatal(err)
	}
	if after := stateEntries(t, deps.Store.Dir()); !slices.Equal(after, before) {
		t.Errorf("state directory %v, was %v", after, before)
	}
	if _, err := os.Stat(rs.createdPath()); err != nil {
		t.Errorf("the created record: %v", err)
	}
}

// TestRemovalReaderReadsOnce checks that a round's reader runs the pane list
// and the issue line search once each, however many workspaces ask.
func TestRemovalReaderReadsOnce(t *testing.T) {
	rec := newRecorder(t)
	search := gitProgram + " " + strings.Join(issueLineSearch, " ")
	rec.reply("herdr pane list", paneListOutput(nil))
	rec.reply(search, []byte(issueLineOf(removalTaskDir, removalIssue)))
	r := &removalReader{cmds: rec.commands(nil), chiefTree: Worktree{path: "/work/chief"}}
	for range 3 {
		if _, err := r.readPanes(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err := r.readIndex(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	counts := map[string]int{}
	for _, c := range rec.recorded() {
		counts[c.line()]++
	}
	if counts["herdr pane list"] != 1 || counts[search] != 1 || len(counts) != 2 {
		t.Errorf("calls = %v, want one pane list and one search", counts)
	}
}

// TestWorkspaceCloser checks that a closer is made only from a candidate
// that passes every check, that it closes exactly the candidate's workspace
// without --group and removes exactly its worktree, in chief's worktree and
// without --force (the recorder allows only those exact lines), and that a
// closer no constructor made runs nothing.
func TestWorkspaceCloser(t *testing.T) {
	t.Run("every check passes", func(t *testing.T) {
		cw := newCheckWorld(t)
		cw.rec.allowWorkspaceClose(removalWS, readTestdata(t, "herdr/workspace-close.json"))
		cw.rec.allowWorktreeRemove(cw.chief.path, cw.tree.path)
		c := newRemovalCandidate(cw.tree, removalWS, issueNumber(t, removalIssue), taskID(t, removalTask))
		closer, checks, ok := newWorkspaceCloser(t.Context(), cw.rec.commands(nil), cw.chief, c, cw.agents, cw.panes)
		if !ok || !allPassed(checks) {
			t.Fatalf("newWorkspaceCloser = %v with %+v, want a closer", ok, checks)
		}
		if err := closer.close(t.Context()); err != nil {
			t.Errorf("close: %v", err)
		}
		if err := closer.removeWorktree(t.Context()); err != nil {
			t.Errorf("removeWorktree: %v", err)
		}
		writes := writesOf(cw.rec)
		want := []string{" herdr workspace close " + removalWS, cw.chief.path + " git " + strings.Join(worktreeRemoveArgs(cw.tree.path), " ")}
		if !slices.Equal(writes, want) {
			t.Errorf("writes %q, want %q", writes, want)
		}
	})
	t.Run("a check does not pass", func(t *testing.T) {
		cw := newCheckWorld(t)
		cw.agents[0].Agent.Status = AgentWorking
		c := newRemovalCandidate(cw.tree, removalWS, issueNumber(t, removalIssue), taskID(t, removalTask))
		closer, checks, ok := newWorkspaceCloser(t.Context(), cw.rec.commands(nil), cw.chief, c, cw.agents, cw.panes)
		if ok || passedCheck(checks, RemovalCheckAgentsIdle) {
			t.Fatalf("newWorkspaceCloser = %v with %+v, want no closer", ok, checks)
		}
		before := len(cw.rec.recorded())
		if err := closer.close(t.Context()); !errors.Is(err, errCloserRefused) {
			t.Errorf("close of the closer returned = %v, want errCloserRefused", err)
		}
		if calls := cw.rec.recorded()[before:]; len(calls) != 0 {
			t.Errorf("the refused closer ran %v", calls)
		}
	})
	t.Run("the zero closer", func(t *testing.T) {
		rec := newRecorder(t)
		closer := WorkspaceCloser{}
		closer.cmds = rec.commands(nil)
		if err := closer.close(t.Context()); !errors.Is(err, errCloserRefused) {
			t.Errorf("close = %v, want errCloserRefused", err)
		}
		if err := closer.removeWorktree(t.Context()); !errors.Is(err, errCloserRefused) {
			t.Errorf("removeWorktree = %v, want errCloserRefused", err)
		}
		if calls := rec.recorded(); len(calls) != 0 {
			t.Errorf("ran %v", calls)
		}
	})
}

// TestWorktreeRemoveKeepsBranch removes a task's worktree with the real git:
// git refuses a worktree with a tracked change and one with an untracked
// file (also when the user's configuration hides untracked files), and a worktree on which a branch was made and left again for a
// detached HEAD is removed with that branch kept.
func TestWorktreeRemoveKeepsBranch(t *testing.T) {
	repo := newTestRepo(t)
	cmds := repo.RealCommands()
	path := filepath.Join(filepath.Dir(repo.Dir), "issue-"+removalIssue)
	repo.Git("worktree", "add", "--detach", path, "HEAD")
	gitIn := func(args ...string) {
		repo.Run(filepath.Join(testToolsDir, gitProgram), append([]string{"-C", path}, args...)...)
	}
	closer := WorkspaceCloser{cmds: cmds, chiefTree: Worktree{path: repo.Dir}, tree: Worktree{path: path}, workspace: removalWS, head: strings.TrimSpace(repo.Git("rev-parse", "HEAD"))}
	writeFile(t, filepath.Join(path, "README.md"), "changed\n")
	if err := closer.removeWorktree(t.Context()); err == nil {
		t.Fatal("git removed a worktree with a tracked change")
	}
	gitIn("checkout", "--", "README.md")
	// The user's configuration that hides untracked files from git's
	// status does not let git remove one.
	repo.Git("config", "status.showUntrackedFiles", "no")
	writeFile(t, filepath.Join(path, "new.txt"), "new\n")
	if err := closer.removeWorktree(t.Context()); err == nil {
		t.Fatal("git removed a worktree with an untracked file")
	}
	if _, err := os.Stat(filepath.Join(path, "new.txt")); err != nil {
		t.Fatalf("the untracked file after the refusal: %v", err)
	}
	if err := os.Remove(filepath.Join(path, "new.txt")); err != nil {
		t.Fatal(err)
	}
	gitIn("switch", "-q", "-c", "kept")
	gitIn("switch", "-q", "--detach")
	if err := closer.removeWorktree(t.Context()); err != nil {
		t.Fatalf("removeWorktree: %v", err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the worktree is still at %s: %v", path, err)
	}
	if got := strings.TrimSpace(repo.Git("branch", "--list", "kept")); got != "kept" {
		t.Errorf("branch kept = %q, want it kept", got)
	}
}

// allowRemoval allows the two writes of a removal of the workspace, each
// changing the fake as the real ones do: closing the workspace takes its
// panes and agents out of Herdr's lists, and removing the worktree removes
// its directory and git's registration.
func (rs *removalSetup) allowRemoval() {
	sw := rs.sw
	sw.rec.allowWriteDo(herdrProgram, "", []string{herdrWorkspace, "close", removalWS}, func() reply {
		if rs.during != nil {
			rs.during()
		}
		rs.close()
		return reply{out: readTestdata(sw.t, "herdr/workspace-close.json")}
	})
	sw.rec.allowWriteDo(gitProgram, sw.chiefTree, worktreeRemoveArgs(rs.tree), func() reply {
		if rs.during != nil {
			rs.during()
		}
		if err := os.RemoveAll(rs.tree); err != nil {
			return reply{err: err}
		}
		sw.git(sw.chiefTree, "worktree list --porcelain", "worktree "+sw.chiefTree+"\nHEAD "+mainCommit+"\nbranch refs/heads/main\n\n")
		return reply{}
	})
}

// close takes the workspace's panes and agents out of Herdr.
func (rs *removalSetup) close() {
	sw := rs.sw
	sw.mu.Lock()
	defer sw.mu.Unlock()
	rs.closed = true
	sw.panes = slices.DeleteFunc(sw.panes, func(p *simPane) bool { return p.Workspace == removalWS })
}

// remove runs Remove of path.
func (rs *removalSetup) remove(deps Deps, path string) RemoveOutcome {
	rs.sw.t.Helper()
	out, err := Remove(rs.sw.t.Context(), deps, path)
	if err != nil {
		rs.sw.t.Fatalf("Remove(%q): %v", path, err)
	}
	return out
}

// writesOf returns the writes rec ran so far, as "dir line" (dir empty
// for herdr).
func writesOf(rec *recorder) []string {
	var out []string
	for _, c := range rec.recorded() {
		if !isReadOnly(c) {
			out = append(out, c.dir+" "+c.line())
		}
	}
	return out
}

// removalWrites are the writes of a whole removal, in order.
func (rs *removalSetup) removalWrites() []string {
	return []string{" herdr workspace close " + removalWS, rs.sw.chiefTree + " git " + strings.Join(worktreeRemoveArgs(rs.tree), " ")}
}

// recordFiles returns the record files left in the workspace's directory.
func (rs *removalSetup) recordFiles() []string {
	rs.sw.t.Helper()
	entries, err := os.ReadDir(filepath.Dir(rs.createdPath()))
	if err != nil {
		rs.sw.t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if isRecordFile(e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names
}

// TestRemoveClosesThenRemoves removes a proposed workspace and checks that
// the tool fetched before it checked, read Herdr again before it closed,
// closed the workspace and then removed the worktree, and removed the
// workspace's records; the output says so.
func TestRemoveClosesThenRemoves(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	rs.allowRemoval()
	deps := sw.finish()
	out := rs.remove(deps, rs.tree)
	if out.Result != RemoveDone || out.Workspace != rs.tree || out.Label != removalLabel || !allPassed(out.Checks) || len(out.Left) != 0 {
		t.Fatalf("Remove = %+v, want removed", out)
	}
	if got, want := writesOf(sw.rec), rs.removalWrites(); !slices.Equal(got, want) {
		t.Errorf("writes %q, want %q", got, want)
	}
	calls := sw.rec.recorded()
	at := func(match func(recordedCall) bool) []int {
		var idx []int
		for i, c := range calls {
			if match(c) {
				idx = append(idx, i)
			}
		}
		return idx
	}
	fetch := at(func(c recordedCall) bool { return c.line() == "git fetch origin main" })
	status := at(func(c recordedCall) bool {
		return c.dir == rs.tree && c.line() == "git status --porcelain --untracked-files=all"
	})
	paneLists := at(func(c recordedCall) bool { return c.line() == "herdr pane list" })
	closeAt := at(func(c recordedCall) bool { return c.line() == "herdr workspace close "+removalWS })
	removeAt := at(func(c recordedCall) bool { return c.line() == "git "+strings.Join(worktreeRemoveArgs(rs.tree), " ") })
	if len(fetch) != 1 || len(status) != 1 || len(closeAt) != 1 || len(removeAt) != 1 || fetch[0] > status[0] || status[0] > closeAt[0] || closeAt[0] > removeAt[0] {
		t.Errorf("fetch %v, status %v, close %v, remove %v; want one each, in that order", fetch, status, closeAt, removeAt)
	}
	// The checks read the pane list once, the re-read before the close once
	// more, and the wait after it at least once.
	if len(paneLists) < 3 || paneLists[1] > closeAt[0] || paneLists[len(paneLists)-1] < closeAt[0] {
		t.Errorf("pane lists at %v, close at %v", paneLists, closeAt)
	}
	if got := rs.recordFiles(); len(got) != 0 {
		t.Errorf("records left %v", got)
	}
	o := out.Output()
	if o.Result != "removed" || o.Refusal != "" || o.Stage != "" || o.Left == nil || len(o.Checks) != len(removalChecks) {
		t.Errorf("output = %+v", o)
	}
}

// TestRemoveLeavesTheRest removes the workspace next to another task
// workspace with records of its own, and checks that the other workspace's
// records stay and no write names it.
func TestRemoveLeavesTheRest(t *testing.T) {
	sw := newSendWorld(t)
	other := sw.addTask(fakeWorkspace{ID: "w1", Label: "other", Branch: taskBranch}, "claude")
	rs := addRemovalWorkspace(t, sw)
	rs.allowRemoval()
	records, err := sw.store.Records(Worktree{path: other})
	if err != nil {
		t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if err := held.Stop("kept", "", "", sw.now); err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	deps := sw.finish()
	if out := rs.remove(deps, rs.tree); out.Result != RemoveDone {
		t.Fatalf("Remove = %+v, want removed", out)
	}
	if got, want := writesOf(sw.rec), rs.removalWrites(); !slices.Equal(got, want) {
		t.Errorf("writes %q, want %q", got, want)
	}
	if stop, err := records.Stopped(); err != nil || stop == nil {
		t.Errorf("the other workspace's stop = %+v, %v; want it kept", stop, err)
	}
}

// TestRemoveHoldsTheExclusion checks that the workspace's exclusion is held
// while the workspace is closed and its worktree removed, so a send cannot
// take it then, and that it is free again once the removal ended.
func TestRemoveHoldsTheExclusion(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	rs.allowRemoval()
	records, err := sw.store.Records(Worktree{path: rs.tree})
	if err != nil {
		t.Fatal(err)
	}
	var during []error
	rs.during = func() {
		held, err := records.Lock()
		if err == nil {
			_ = held.Close()
		}
		during = append(during, err)
	}
	if out := rs.remove(sw.finish(), rs.tree); out.Result != RemoveDone {
		t.Fatalf("Remove = %+v, want removed", out)
	}
	if len(during) != 2 || !errors.Is(during[0], ErrWorkspaceBusy) || !errors.Is(during[1], ErrWorkspaceBusy) {
		t.Errorf("locks taken during the writes = %v, want busy twice", during)
	}
	held, err := records.Lock()
	if err != nil {
		t.Fatalf("the exclusion after the removal: %v", err)
	}
	_ = held.Close()
}

// TestRemoveRefusesOnEachCheck breaks one check at a time and checks that
// the removal is refused with that check not passed, that nothing is
// written (the recorder allows no write), and that the created record stays.
func TestRemoveRefusesOnEachCheck(t *testing.T) {
	cases := []struct {
		name  string
		check RemovalCheck
		edit  func(t *testing.T, sw *sendWorld, rs *removalSetup)
	}{
		{"the task not done", RemovalCheckCandidate, func(t *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.docs(sw.chiefTree, fakeDocs{Dir: removalTaskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")})
		}},
		{"HEAD on a branch", RemovalCheckDetached, func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			sw.git(rs.tree, "rev-parse --abbrev-ref HEAD", "main\n")
		}},
		{"a change", RemovalCheckClean, func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			sw.git(rs.tree, "status --porcelain --untracked-files=all", "?? new.txt\n")
		}},
		{"HEAD not on origin/main", RemovalCheckHeadOnMain, func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			sw.rec.failIn(rs.tree, "git merge-base --is-ancestor "+mainCommit+" "+mainCommit, &cmdrunner.ExitError{Code: 1})
		}},
		{"a locked worktree", RemovalCheckUnlocked, func(_ *testing.T, _ *sendWorld, rs *removalSetup) { rs.worktrees(true) }},
		{"an agent working", RemovalCheckAgentsIdle, func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.byPane[removalWS+":p1"].Status = "working"
		}},
		{"a busy shell", RemovalCheckShellsIdle, func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.rec.reply("herdr pane process-info --pane "+removalWS+":p3", processInfoOutput(removalWS+":p3", 100, 200, 200))
		}},
		// Another workspace at the path already makes the workspace no
		// candidate, the condition read before the checks; the check of
		// the path refuses it only when the pane list differs between the
		// two, which TestRemovalChecks covers.
		{"another workspace at the path", RemovalCheckCandidate, func(_ *testing.T, _ *sendWorld, rs *removalSetup) {
			rs.others = []fakePane{{Pane: "w9:p1", Tab: "w9:t1", Workspace: "w9", Dir: rs.tree}}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			c.edit(t, sw, rs)
			out := rs.remove(sw.finish(), rs.tree)
			if out.Result != RemoveRefused || out.Refusal != RemoveRefusalChecksFailed || len(out.Checks) == 0 || passedCheck(out.Checks, c.check) {
				t.Fatalf("Remove = %+v, want refused with %v not passed", out, c.check)
			}
			if !slices.ContainsFunc(out.Checks, func(r CheckResult) bool { return r.Check == c.check }) {
				t.Errorf("checks %+v lack %v", out.Checks, c.check)
			}
			// A workspace no longer a candidate carries the candidate check
			// alone, since the others need a candidate, and says why.
			if c.check == RemovalCheckCandidate && (len(out.Checks) != 1 || out.Failure == "") {
				t.Errorf("checks %+v, failure %q; want the candidate check alone, with a failure", out.Checks, out.Failure)
			}
			if w := writesOf(sw.rec); len(w) != 0 {
				t.Errorf("writes %v", w)
			}
			if _, err := os.Stat(rs.createdPath()); err != nil {
				t.Errorf("the created record: %v", err)
			}
			if o := out.Output(); o.Result != "refused" || o.Refusal != "checks_failed" {
				t.Errorf("output = %+v", o)
			}
		})
	}
}

// TestRemoveRefusals checks the refusals before the fetch: a path that is
// no task workspace's worktree (git never runs in it) and a workspace whose
// exclusion is held; neither writes (the recorder allows no write).
func TestRemoveRefusals(t *testing.T) {
	t.Run("no workspace", func(t *testing.T) {
		sw, rs := newRemovalWorld(t)
		elsewhere := filepath.Join(filepath.Dir(rs.tree), "elsewhere")
		mustMkdir(t, elsewhere)
		out := rs.remove(sw.finish(), elsewhere)
		if out.Result != RemoveRefused || out.Refusal != RemoveRefusalNoWorkspace || out.Workspace != elsewhere {
			t.Fatalf("Remove = %+v, want no_workspace", out)
		}
		for _, c := range sw.rec.recorded() {
			if c.dir == elsewhere {
				t.Errorf("%s ran in the path given", c.line())
			}
		}
	})
	t.Run("busy", func(t *testing.T) {
		sw, rs := newRemovalWorld(t)
		rs.held()
		if out := rs.remove(sw.finish(), rs.tree); out.Result != RemoveRefused || out.Refusal != RemoveRefusalBusy {
			t.Fatalf("Remove = %+v, want busy", out)
		}
	})
}

// TestRemoveFetchFailure checks that a failed fetch of origin/main refuses
// the removal before any check: no git runs in the worktree after it and
// nothing is written.
func TestRemoveFetchFailure(t *testing.T) {
	sw, rs := newRemovalWorld(t)
	sw.rec.failIn(sw.chiefTree, "git fetch origin main", errInjected)
	out := rs.remove(sw.finish(), rs.tree)
	if out.Result != RemoveRefused || out.Refusal != RemoveRefusalFetchFailed || len(out.Checks) != 0 {
		t.Fatalf("Remove = %+v, want fetch_failed", out)
	}
	calls := sw.rec.recorded()
	fetch := slices.IndexFunc(calls, func(c recordedCall) bool { return c.line() == "git fetch origin main" })
	if fetch < 0 {
		t.Fatal("no fetch ran")
	}
	for _, c := range calls[fetch+1:] {
		if c.dir == rs.tree {
			t.Errorf("%s ran after the failed fetch", c.line())
		}
	}
}

// TestRemoveRefusesWhenChanged changes, between the checks and the close,
// one thing the re-read compares, and checks that nothing is closed or
// removed and the refusal is changed.
func TestRemoveRefusesWhenChanged(t *testing.T) {
	cases := []struct {
		name string
		edit func(sw *sendWorld, rs *removalSetup)
	}{
		{"a new pane", func(sw *sendWorld, rs *removalSetup) {
			after(sw, "herdr pane list", 1, func() reply {
				return reply{out: paneListOutput(append(rs.paneList(), fakePane{Pane: removalWS + ":p4", Tab: removalWS + ":t4", Workspace: removalWS, Dir: rs.tree}))}
			})
		}},
		{"another workspace at the path", func(sw *sendWorld, rs *removalSetup) {
			after(sw, "herdr pane list", 1, func() reply {
				return reply{out: paneListOutput(append(rs.paneList(), fakePane{Pane: "w9:p1", Tab: "w9:t1", Workspace: "w9", Dir: rs.tree}))}
			})
		}},
		{"an agent's state", func(sw *sendWorld, _ *removalSetup) {
			// The agent list is read by the lookup, under the exclusion, and
			// again before the close.
			after(sw, "herdr agent list", 2, func() reply {
				sw.mu.Lock()
				defer sw.mu.Unlock()
				sw.byPane[removalWS+":p2"].Seq++
				agents := []fakeAgent{sw.chief}
				for _, p := range sw.panes {
					agents = append(agents, p.fakeAgent)
				}
				return reply{out: agentListOutput(agents)}
			})
		}},
		{"a pane opened while the processes were read", func(sw *sendWorld, rs *removalSetup) {
			after(sw, "herdr pane list", 2, func() reply {
				return reply{out: paneListOutput(append(rs.paneList(), fakePane{Pane: removalWS + ":p4", Tab: removalWS + ":t4", Workspace: removalWS, Dir: rs.tree}))}
			})
		}},
		{"the pane list cannot be read again", func(sw *sendWorld, _ *removalSetup) {
			after(sw, "herdr pane list", 1, func() reply { return reply{err: errInjected} })
		}},
		{"the agent list cannot be read again", func(sw *sendWorld, _ *removalSetup) {
			after(sw, "herdr agent list", 2, func() reply { return reply{err: errInjected} })
		}},
		{"a busy shell", func(sw *sendWorld, _ *removalSetup) {
			after(sw, "herdr pane process-info --pane "+removalWS+":p3", 1, func() reply {
				return reply{out: processInfoOutput(removalWS+":p3", 100, 200, 200)}
			})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			deps := sw.finish()
			c.edit(sw, rs)
			out := rs.remove(deps, rs.tree)
			if out.Result != RemoveRefused || out.Refusal != RemoveRefusalChanged || !allPassed(out.Checks) {
				t.Fatalf("Remove = %+v, want changed after every check passed", out)
			}
			if w := writesOf(sw.rec); len(w) != 0 {
				t.Errorf("writes %v", w)
			}
			if _, err := os.Stat(rs.createdPath()); err != nil {
				t.Errorf("the created record: %v", err)
			}
		})
	}
}

// after makes line answer as before for its first n runs and with changed
// from then on. It replaces the reply sw.finish set, so it runs after it;
// changed takes the world's lock itself when it needs it.
func after(sw *sendWorld, line string, n int, changed func() reply) {
	sw.rec.mu.Lock()
	before, isDynamic := sw.rec.dynamic[line]
	fixed := sw.rec.replies[line]
	sw.rec.mu.Unlock()
	runs := 0
	sw.rec.replyFunc(line, func() reply {
		runs++
		if runs > n {
			return changed()
		}
		if isDynamic {
			return before()
		}
		return fixed
	})
}

// TestRemoveStopsAtEachStage makes each stage after the checks fail in turn
// and checks that the removal stops there, lists what is left by what is
// really left, and runs no write again or after it.
func TestRemoveStopsAtEachStage(t *testing.T) {
	closeLine := []string{herdrWorkspace, "close", removalWS}
	closeOK := func(sw *sendWorld, _ *removalSetup) {
		sw.rec.allowWrite(herdrProgram, "", closeLine, reply{out: readTestdata(sw.t, "herdr/workspace-close.json")})
	}
	cases := []struct {
		name  string
		stage string
		setup func(sw *sendWorld, rs *removalSetup)
		// late runs after sw.finish, for a reply that replaces one it sets.
		late   func(sw *sendWorld, rs *removalSetup)
		writes int
		// sleeps is how many waits for the panes the removal made.
		sleeps int
		left   []string
	}{
		{"close fails", "close_workspace", func(sw *sendWorld, _ *removalSetup) {
			sw.rec.allowWrite(herdrProgram, "", closeLine, reply{err: errInjected})
		}, nil, 1, 0, []string{"workspace", "worktree", "registration", "record"}},
		// The wait reads up to its bound, sleeping between the reads.
		{"the panes stay", "wait_panes", closeOK, nil, 1, closeAttempts - 1, []string{"workspace", "worktree", "registration", "record"}},
		// A read that fails stops the wait at once, with no sleep. The
		// checks and the re-read before the close (twice) read the list
		// first.
		{"the pane list fails during the wait", "wait_panes", closeOK, func(sw *sendWorld, _ *removalSetup) {
			after(sw, "herdr pane list", 3, func() reply { return reply{err: errInjected} })
		}, 1, 0, []string{"workspace", "worktree", "registration", "record"}},
		// Another workspace at the worktree once the panes are gone stops
		// the wait at once.
		{"another workspace takes the path", "wait_panes", func(sw *sendWorld, rs *removalSetup) {
			sw.rec.allowWriteDo(herdrProgram, "", closeLine, func() reply {
				rs.close()
				rs.others = []fakePane{{Pane: "w9:p1", Tab: "w9:t1", Workspace: "w9", Dir: rs.tree}}
				return reply{out: readTestdata(sw.t, "herdr/workspace-close.json")}
			})
		}, nil, 1, 0, []string{"worktree", "registration", "record"}},
		{"git refuses the worktree", "remove_worktree", func(sw *sendWorld, rs *removalSetup) {
			sw.rec.allowWriteDo(herdrProgram, "", closeLine, func() reply {
				rs.close()
				return reply{out: readTestdata(sw.t, "herdr/workspace-close.json")}
			})
			sw.rec.allowWrite(gitProgram, sw.chiefTree, worktreeRemoveArgs(rs.tree), reply{err: &cmdrunner.ExitError{Code: 128}})
		}, nil, 2, 0, []string{"worktree", "registration", "record"}},
		{"a record cannot be removed", "remove_records", func(sw *sendWorld, rs *removalSetup) {
			rs.allowRemoval()
			// A directory where the stop record goes is refused by the
			// removal of records, so the created record, removed last,
			// stays too.
			mustMkdir(sw.t, filepath.Join(filepath.Dir(rs.createdPath()), stopFile))
		}, nil, 2, 0, []string{"record", "record"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			c.setup(sw, rs)
			deps := sw.finish()
			if c.late != nil {
				c.late(sw, rs)
			}
			slept := 0
			deps.Sleep = func(_ context.Context, d time.Duration) error {
				if d != closeInterval {
					t.Errorf("slept %v, want %v", d, closeInterval)
				}
				slept++
				return nil
			}
			out := rs.remove(deps, rs.tree)
			o := out.Output()
			if out.Result != RemoveStopped || o.Stage != c.stage || out.Failure == "" {
				t.Fatalf("Remove = %+v, want stopped at %s", out, c.stage)
			}
			if got, want := writesOf(sw.rec), rs.removalWrites()[:c.writes]; !slices.Equal(got, want) {
				t.Errorf("writes %q, want %q", got, want)
			}
			if slept != c.sleeps {
				t.Errorf("slept %d times, want %d", slept, c.sleeps)
			}
			var kinds []string
			for _, m := range o.Left {
				kinds = append(kinds, m.Kind)
			}
			if !slices.Equal(kinds, c.left) || o.Result != "stopped" {
				t.Errorf("output = %+v, want left %v", o, c.left)
			}
		})
	}
}

// TestRemoveStopsWhenHeadMoved moves the worktree's HEAD, or makes
// origin/main stop reaching it, after the checks, and checks that the
// workspace is closed but the worktree is not removed.
func TestRemoveStopsWhenHeadMoved(t *testing.T) {
	cases := []struct {
		name string
		line string
		// n is how many runs of line answer as before: the checks'.
		n       int
		changed reply
		setup   func(sw *sendWorld, rs *removalSetup)
	}{
		{"HEAD moved", "git rev-parse HEAD", 1, reply{out: []byte(strings.Repeat("3", 40) + "\n")}, nil},
		// The recheck reads HEAD before and after the ancestry.
		{"HEAD moved while the ancestry was read", "git rev-parse HEAD", 2, reply{out: []byte(strings.Repeat("3", 40) + "\n")}, nil},
		{"HEAD no longer on origin/main", "git merge-base --is-ancestor " + mainCommit + " " + mainCommit, 1, reply{err: &cmdrunner.ExitError{Code: 1}}, nil},
		// origin/main read again is another commit, which does not reach
		// HEAD, so a recheck that kept the first read would remove.
		{"origin/main read again does not reach HEAD", "git rev-parse " + originMain, 1, reply{out: []byte(strings.Repeat("4", 40) + "\n")}, func(sw *sendWorld, rs *removalSetup) {
			sw.rec.failIn(rs.tree, "git merge-base --is-ancestor "+mainCommit+" "+strings.Repeat("4", 40), &cmdrunner.ExitError{Code: 1})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			rs.allowRemoval()
			if c.setup != nil {
				c.setup(sw, rs)
			}
			sw.rec.mu.Lock()
			first := sw.rec.dirReplies[rs.tree+"\x00"+c.line]
			sw.rec.mu.Unlock()
			runs := 0
			sw.rec.replyFunc(c.line, func() reply {
				runs++
				if runs > c.n {
					return c.changed
				}
				return first
			})
			out := rs.remove(sw.finish(), rs.tree)
			if out.Result != RemoveStopped || out.Stage != RemoveStageRecheckHead {
				t.Fatalf("Remove = %+v, want stopped at recheck_head", out)
			}
			if got, want := writesOf(sw.rec), rs.removalWrites()[:1]; !slices.Equal(got, want) {
				t.Errorf("writes %q, want %q", got, want)
			}
			if got, want := madeKinds(out.Left), []string{"worktree", "registration", "record"}; !slices.Equal(got, want) {
				t.Errorf("left %v, want %v", got, want)
			}
		})
	}
}

// TestRemoveDeadline makes the removal's context end while the workspace is
// closed, as its time limit does (a cancelled context stands in for the
// limit), and checks that the removal stops at the next stage as the
// deadline, listing what is left and writing nothing more.
func TestRemoveDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sw, rs := newRemovalWorld(t)
	sw.rec.allowWriteDo(herdrProgram, "", []string{herdrWorkspace, "close", removalWS}, func() reply {
		cancel()
		rs.close()
		return reply{out: readTestdata(t, "herdr/workspace-close.json")}
	})
	out, err := Remove(ctx, sw.finish(), rs.tree)
	if err != nil || out.Result != RemoveStopped || out.Stage != RemoveStageDeadline {
		t.Fatalf("Remove = %+v, %v; want stopped at the deadline", out, err)
	}
	if got, want := writesOf(sw.rec), rs.removalWrites()[:1]; !slices.Equal(got, want) {
		t.Errorf("writes %q, want %q", got, want)
	}
	if got, want := madeKinds(out.Left), []string{"worktree", "registration", "record"}; !slices.Equal(got, want) {
		t.Errorf("left %v, want %v", got, want)
	}
}

// finishTask makes the task of the workspace cw made finished, as /runpr
// leaves it after the task's last PR: the worktree on a detached HEAD at
// origin/main, clean, its agents idle at their prompts, and origin/main
// with the issue's one task, whose plan has every PR merged.
func (cw *createWorld) finishTask() {
	t := cw.t
	cw.mu.Lock()
	for _, p := range cw.panes {
		if p.Workspace == "w1" {
			p.Status, p.Seq = "idle", p.Seq+1
			if p.Kind == "claude" {
				p.screen = readScreenSample(t, "claude-idle.ansi")
			}
		}
	}
	cw.mu.Unlock()
	cw.git(cw.tree, "rev-parse --abbrev-ref HEAD", detachedHead+"\n")
	cw.git(cw.tree, "status --porcelain --untracked-files=all", "")
	cw.git(cw.tree, "rev-parse HEAD", cw.commit+"\n")
	cw.git(cw.tree, "rev-parse "+originMain, cw.commit+"\n")
	cw.git(cw.tree, "merge-base --is-ancestor "+cw.commit+" "+cw.commit, "")
	cw.git(cw.chiefTree, "worktree list --porcelain", "worktree "+cw.chiefTree+"\nHEAD "+cw.commit+"\nbranch refs/heads/main\n\nworktree "+cw.tree+"\nHEAD "+cw.commit+"\ndetached\n\n")
	cw.rec.replyIn(cw.chiefTree, gitProgram+" "+strings.Join(issueLineSearch, " "), []byte(issueLineOf(removalTaskDir, createIssue)))
	cw.docs(cw.chiefTree, fakeDocs{Dir: removalTaskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-all-merged.md")})
	cw.rec.reply("herdr pane process-info --pane w1:p1", processInfoOutput("w1:p1", 100, 100, 100))
}

// allowRemoval allows the removal of the workspace cw made, each write
// changing the fake as the real one does.
func (cw *createWorld) allowRemoval() {
	cw.rec.allowWriteDo(herdrProgram, "", []string{herdrWorkspace, "close", "w1"}, func() reply {
		cw.mu.Lock()
		defer cw.mu.Unlock()
		cw.listed = slices.DeleteFunc(cw.listed, func(p fakePane) bool { return p.Workspace == "w1" })
		cw.panes = slices.DeleteFunc(cw.panes, func(p *simPane) bool { return p.Workspace == "w1" })
		return reply{out: readTestdata(cw.t, "herdr/workspace-close.json")}
	})
	cw.rec.allowWriteDo(gitProgram, cw.chiefTree, worktreeRemoveArgs(cw.tree), func() reply {
		if err := os.RemoveAll(cw.tree); err != nil {
			return reply{err: err}
		}
		cw.git(cw.chiefTree, "worktree list --porcelain", "worktree "+cw.chiefTree+"\nHEAD "+cw.commit+"\nbranch refs/heads/main\n\n")
		return reply{}
	})
}

// TestCreateToRemove runs a task's workspace from its creation to its
// removal in one world: chief makes it and sends /mkreq; once the task is
// finished, the round proposes its removal; the removal closes it and
// removes its worktree and records; and the next round shows neither its
// row nor its removal again.
func TestCreateToRemove(t *testing.T) {
	cw := newCreateWorld(t, nil)
	cw.allowRemoval()
	made := cw.create(createIssue)
	if made.Result != CreateMade || made.Mkreq == nil || made.Mkreq.Outcome != OutcomeSent {
		t.Fatalf("Create = %+v; want made with /mkreq sent", made)
	}
	cw.finishTask()
	snap, err := TakeSnapshot(context.Background(), cw.deps, cw.owner())
	if err != nil {
		t.Fatal(err)
	}
	row := rowOf(t, snap, "issue-"+createIssue)
	if row.State != StateTaskDone || !row.RemovalRow || row.Task != removalTask {
		t.Errorf("row = %+v, want the task done with the removal proposed", row)
	}
	if items := removalItems(snap); len(items) != 1 || items[0].Workspace != cw.tree || !allPassed(items[0].Removal.Checks) {
		t.Fatalf("removal items = %+v, want one for %s", items, cw.tree)
	}
	out, err := Remove(context.Background(), cw.deps, cw.tree)
	if err != nil || out.Result != RemoveDone {
		t.Fatalf("Remove = %+v, %v; want removed", out, err)
	}
	if _, err := os.Lstat(cw.tree); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the worktree is still there: %v", err)
	}
	if records, ok, err := cw.deps.Store.existingRecords(Worktree{path: cw.tree}); err != nil || !ok {
		t.Errorf("records of %s: ok %v, %v", cw.tree, ok, err)
	} else if rec, err := records.Created(cw.env.Home); rec != nil || err != nil {
		t.Errorf("the created record = %+v, %v; want it gone", rec, err)
	}
	next, err := TakeSnapshot(context.Background(), cw.deps, cw.owner())
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(next.Rows, func(r Row) bool { return r.Workspace == cw.tree }) || len(removalItems(next)) != 0 {
		t.Errorf("the next round shows rows %+v and items %+v", next.Rows, next.Pending)
	}
}
