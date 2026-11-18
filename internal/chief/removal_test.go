//go:build test && unix

package chief

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
	sw.rec.replyFunc("herdr pane list", func() reply {
		sw.mu.Lock()
		defer sw.mu.Unlock()
		panes := []fakePane{{Pane: sw.chief.Pane, Tab: "wc:t1", Workspace: sw.chief.Workspace, Dir: sw.chief.Dir, Agent: sw.chief.Kind}}
		for _, p := range sw.panes {
			panes = append(panes, fakePane{Pane: p.Pane, Tab: strings.Replace(p.Pane, ":p", ":t", 1), Workspace: p.Workspace, Dir: p.Dir, Agent: p.Kind})
		}
		panes = append(panes, fakePane{Pane: removalWS + ":p3", Tab: removalWS + ":t3", Workspace: removalWS, Dir: rs.tree})
		return reply{out: paneListOutput(append(panes, rs.others...))}
	})
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
// without its created record the row is 0088's detached one, and with it
// the row shows the task done, the issue's task, and the removal proposed,
// with one removal item whose checks all pass.
func TestRemovalRow(t *testing.T) {
	t.Run("without the record", func(t *testing.T) {
		sw, rs := newRemovalWorld(t)
		if err := os.Remove(rs.createdPath()); err != nil {
			t.Fatal(err)
		}
		r := rowOf(t, snapshotOf(t, sw), removalLabel)
		if r.State != StateNeedsCheck || !r.Idle || r.RemovalRow || r.Removal != nil || r.Task != "" {
			t.Fatalf("row = %+v, want 0088's detached row", r)
		}
	})
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
	if len(items) != 1 || items[0].Removal != r.Removal || items[0].Workspace != rs.tree || snap.Pending[len(snap.Pending)-1].ID != items[0].ID {
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
		"no task of the issue": func(_ *testing.T, _ *sendWorld, rs *removalSetup) {
			rs.issueLines(issueLineOf(removalTaskDir, "840"))
		},
		"two tasks of the issue": func(_ *testing.T, _ *sendWorld, rs *removalSetup) {
			rs.issueLines(issueLineOf(removalTaskDir, removalIssue) + issueLineOf("0006_other_task", removalIssue))
		},
		"the issue only in the remarks": func(_ *testing.T, _ *sendWorld, rs *removalSetup) {
			rs.issueLines(strings.TrimSuffix(issueLineOf(removalTaskDir, "840"), "\n") + issueMentionEnd + "#" + removalIssue + "\n")
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
	cases := map[string]func(t *testing.T, sw *sendWorld, rs *removalSetup){
		"a record that is not JSON": func(t *testing.T, _ *sendWorld, rs *removalSetup) {
			if err := os.WriteFile(rs.createdPath(), []byte("not json"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"a record naming another worktree": func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			rs.forgeCreated(createdJSON{Worktree: sw.chiefTree, Issue: removalIssue, Created: sw.now})
		},
		"a record of another issue": func(_ *testing.T, sw *sendWorld, rs *removalSetup) {
			rs.forgeCreated(createdJSON{Worktree: rs.tree, Issue: "840", Created: sw.now})
		},
		"the issue lines cannot be read": func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.rec.failIn(sw.chiefTree, gitProgram+" "+strings.Join(issueLineSearch, " "), &cmdrunner.ExitError{Code: 128})
		},
		"the task's documents cannot be read": func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.rec.failIn(sw.chiefTree, "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/", errInjected)
		},
		"the pane list cannot be read": func(_ *testing.T, sw *sendWorld, _ *removalSetup) {
			sw.rec.replyFunc("herdr pane list", func() reply { return reply{err: errInjected} })
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			sw, rs := newRemovalWorld(t)
			edit(t, sw, rs)
			snap := snapshotOf(t, sw)
			assertNotProposed(t, snap)
			if !slices.ContainsFunc(snap.Problems, func(p string) bool { return strings.Contains(p, "not offered for removal") }) {
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
	c := newRemovalCandidate(cw.tree, removalWS, removalLabel, issueNumber(t, removalIssue), taskID(t, removalTask))
	return checkRemoval(context.Background(), cw.rec.commands(nil), cw.chief, c, cw.agents, cw.panes)
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
		got := checkRemoval(context.Background(), cw.rec.commands(nil), cw.chief, RemovalCandidate{}, cw.agents, cw.panes)
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
