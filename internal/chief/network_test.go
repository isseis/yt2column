//go:build test && unix

package chief

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestSnapshotRunsOnlyReadsInTheRightPlaces runs a whole snapshot over a
// workspace with an open PR and a workspace proposed a command, with the
// target-redirecting variables set in the process environment, and checks
// every command the tool ran: each is in the recorder's read-only list (the
// recorder fails the test otherwise), gh never sees GH_REPO or GH_HOST, no
// command sees a GIT_ variable, git and gh run only in a task worktree or
// chief's own, and the PR's detail (checks, view) is not gathered.
func TestSnapshotRunsOnlyReadsInTheRightPlaces(t *testing.T) {
	t.Setenv("GH_REPO", "other/repo")
	t.Setenv("GH_HOST", "ghe.example.com")
	t.Setenv("GIT_DIR", "/elsewhere/.git")
	w := newWorld(t)
	review := w.add(fakeWorkspace{ID: "w1", Label: "review", Branch: taskBranch, PRs: openPRJSON})
	propose := w.add(fakeWorkspace{ID: "w2", Label: "propose", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}})
	take(t, w)
	places := []string{w.chiefTree, review, propose}
	sawGH, sawGit := false, false
	for _, c := range w.rec.recorded() {
		switch c.name {
		case ghProgram:
			sawGH = true
			if len(c.args) < 2 || c.args[0] != "pr" || c.args[1] != "list" {
				t.Errorf("gh ran %v; a snapshot reads only the open PR list", c.args)
			}
		case gitProgram:
			sawGit = true
		}
		if c.name != gitProgram && c.name != ghProgram {
			continue
		}
		if !slices.Contains(places, c.dir) {
			t.Errorf("%s ran in %q, not in a task worktree or chief's own", c.line(), c.dir)
		}
		for _, e := range c.env {
			name, _, _ := strings.Cut(e, "=")
			if strings.HasPrefix(name, "GIT_DIR") || strings.HasPrefix(name, "GIT_WORK_TREE") || (c.name == ghProgram && (name == "GH_REPO" || name == "GH_HOST")) {
				t.Errorf("%s ran with %s in its environment", c.line(), name)
			}
		}
	}
	if !sawGH || !sawGit {
		t.Fatalf("the snapshot ran gh=%v git=%v; the test reached nothing", sawGH, sawGit)
	}
}

// TestDetailAndSendRunOnlyReadsInTheRightPlaces runs a detail and a send of
// /runpr with its reset, with the target-redirecting variables set, and
// checks every command: git and gh only read (the recorder fails anything
// outside its read-only list), gh reads only PRs and never sees GH_REPO or
// GH_HOST, both run only in the task worktree or chief's own, and the only
// writes are prompts to the task workspace's panes. It also checks that the
// detail reads no pane's screen, and that the reset reads the screen of the
// pane it empties once, for the check just before its input.
func TestDetailAndSendRunOnlyReadsInTheRightPlaces(t *testing.T) {
	t.Setenv("GH_REPO", "other/repo")
	t.Setenv("GH_HOST", "ghe.example.com")
	sw, tree := detailWorld(t)
	deps := sw.finish()
	sw.endWithReport(sw.firstPane(), makerReport)
	if d, err := Detail(context.Background(), deps, tree); err != nil || d.Detail == nil || d.Detail.Report != ReportFromPane {
		t.Fatalf("Detail = %+v, %v; want the pane's report", d, err)
	}
	isRead := func(c recordedCall) bool {
		return c.name == herdrProgram && len(c.args) > 2 && c.args[0] == herdrAgent && c.args[1] == "read"
	}
	detailCalls := sw.rec.recorded()
	for _, c := range detailCalls {
		if isRead(c) {
			t.Errorf("the detail read a screen: %s", c.line())
		}
	}
	out := sendRunpr(t, deps, tree)
	if len(out.Actions) != 2 || out.Actions[1].Outcome != OutcomeSent {
		t.Fatalf("actions = %+v; the test reached no reset", out.Actions)
	}
	// The reset runs from /runpr's input to /clear's; in that span it reads
	// the screen of w1:p1 only for the check just before /clear.
	reads, inReset := 0, false
	for _, c := range sw.rec.recorded()[len(detailCalls):] {
		switch {
		case isPrompt(c) && c.args[3] == "/runpr 42":
			inReset = true
		case isPrompt(c) && c.args[3] == "/clear":
			inReset = false
		case inReset && isRead(c) && c.args[2] == "w1:p1":
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("the reset read the screen of w1:p1 %d times before /clear, want once", reads)
	}
	places := []string{sw.chiefTree, tree}
	sawChecks := false
	for _, c := range sw.rec.recorded() {
		switch c.name {
		case ghProgram:
			if len(c.args) < 2 || c.args[0] != "pr" || !slices.Contains([]string{"list", "view", "checks"}, c.args[1]) {
				t.Errorf("gh ran %v", c.args)
				continue
			}
			sawChecks = sawChecks || c.args[1] == "checks"
			for _, e := range c.env {
				if name, _, _ := strings.Cut(e, "="); name == "GH_REPO" || name == "GH_HOST" {
					t.Errorf("%s ran with %s", c.line(), name)
				}
			}
		case herdrProgram:
			if isPrompt(c) && !strings.HasPrefix(c.args[2], "w1:") {
				t.Errorf("a prompt went to %s", c.args[2])
			}
			continue
		}
		if !slices.Contains(places, c.dir) {
			t.Errorf("%s ran in %q", c.line(), c.dir)
		}
	}
	if !sawChecks {
		t.Error("no gh pr checks ran; the test reached no detail")
	}
}

// TestTickRunsOnlyReadsInTheRightPlaces runs the ticks around a merge up to
// its automatic send and the reset, with the target-redirecting variables
// set, and checks every command: git and gh only read (the recorder fails
// anything outside its read-only list), gh reads only PRs and never sees
// GH_REPO or GH_HOST, the remote is read only from chief's worktree, both
// run only in the task worktree or chief's own, and the only writes are
// prompts to the task workspace's panes.
func TestTickRunsOnlyReadsInTheRightPlaces(t *testing.T) {
	t.Setenv("GH_REPO", "other/repo")
	t.Setenv("GH_HOST", "ghe.example.com")
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	if res := only(t, mw.tick(time.Minute)); !sentCommand(res) {
		t.Fatalf("auto send = %+v; the test reached no send", res)
	}
	places := []string{mw.chiefTree, mw.tree}
	sawView, sawRemote := false, false
	for _, c := range mw.rec.recorded() {
		switch c.name {
		case ghProgram:
			if len(c.args) < 2 || c.args[0] != "pr" || !slices.Contains([]string{"list", "view"}, c.args[1]) {
				t.Errorf("gh ran %v", c.args)
				continue
			}
			sawView = sawView || c.args[1] == "view"
			for _, e := range c.env {
				if name, _, _ := strings.Cut(e, "="); name == "GH_REPO" || name == "GH_HOST" {
					t.Errorf("%s ran with %s", c.line(), name)
				}
			}
		case gitProgram:
			if c.args[0] == "ls-remote" {
				sawRemote = true
				if c.dir != mw.chiefTree {
					t.Errorf("%s ran in %q, not in chief's worktree", c.line(), c.dir)
				}
			}
		case herdrProgram:
			if isPrompt(c) && !strings.HasPrefix(c.args[2], "w1:") {
				t.Errorf("a prompt went to %s", c.args[2])
			}
			continue
		}
		if !slices.Contains(places, c.dir) {
			t.Errorf("%s ran in %q", c.line(), c.dir)
		}
	}
	if !sawView || !sawRemote {
		t.Errorf("gh pr view ran %v, git ls-remote ran %v; the test reached no merge check", sawView, sawRemote)
	}
}

// TestCreateRunsOnlyInTheRightPlaces runs a whole creation with the
// target-redirecting variables set and checks every command: gh runs only
// the one issue read, in chief's worktree and without GH_REPO or GH_HOST;
// git runs only in chief's worktree and in the worktree made; and every
// write is one the test allowed (the recorder fails any other). The send's
// lookup also reads, with git rev-parse, the directory of every other
// agent; this world has no other agent, so that read is not checked here.
func TestCreateRunsOnlyInTheRightPlaces(t *testing.T) {
	t.Setenv("GH_REPO", "other/repo")
	t.Setenv("GH_HOST", "ghe.example.com")
	cw := newCreateWorld(t, nil)
	if out := cw.create(createIssue); out.Result != CreateMade || out.Mkreq == nil || out.Mkreq.Outcome != OutcomeSent {
		t.Fatalf("Create = %+v; the test reached no send", out)
	}
	ghCalls := 0
	for _, c := range cw.rec.recorded() {
		switch c.name {
		case ghProgram:
			ghCalls++
			if !isIssueRead(c.args) || c.dir != cw.chiefTree {
				t.Errorf("gh ran %s in %q; a creation reads only the issue, in chief's worktree", c.line(), c.dir)
			}
			for _, e := range c.env {
				if name, _, _ := strings.Cut(e, "="); name == "GH_REPO" || name == "GH_HOST" {
					t.Errorf("%s ran with %s", c.line(), name)
				}
			}
		case gitProgram:
			if c.dir != cw.chiefTree && c.dir != cw.tree {
				t.Errorf("%s ran in %q, not in chief's worktree or the one made", c.line(), c.dir)
			}
		}
	}
	if ghCalls != 1 {
		t.Errorf("gh ran %d times, want once", ghCalls)
	}
}

// TestRemoveRunsOnlyInTheRightPlaces runs a whole removal next to another
// task workspace and checks every command: gh never runs; git runs in chief's worktree and in the
// worktree removed, and in the other agent's directory only the two
// rev-parse reads by which the lookup tells task workspaces apart; and the
// only writes are the workspace's close and the worktree's removal (the
// recorder fails any other).
func TestRemoveRunsOnlyInTheRightPlaces(t *testing.T) {
	sw := newSendWorld(t)
	other := sw.addTask(fakeWorkspace{ID: "w1", Label: "other", Branch: taskBranch}, "claude")
	rs := addRemovalWorkspace(t, sw)
	rs.allowRemoval()
	lookup := [][]string{{"rev-parse", "--path-format=absolute", "--git-common-dir"}, {"rev-parse", "--show-toplevel"}}
	if out := rs.remove(sw.finish(), rs.tree); out.Result != RemoveDone {
		t.Fatalf("Remove = %+v; the test reached no removal", out)
	}
	sawTree, sawOther := false, false
	for _, c := range sw.rec.recorded() {
		switch c.name {
		case ghProgram:
			t.Errorf("gh ran %s; a removal reads nothing from GitHub", c.line())
		case gitProgram:
			sawTree = sawTree || c.dir == rs.tree
			if c.dir == other {
				sawOther = true
				if !slices.ContainsFunc(lookup, func(l []string) bool { return slices.Equal(c.args, l) }) {
					t.Errorf("%s ran in the other workspace's directory; only the lookup's reads may", c.line())
				}
				continue
			}
			if c.dir != sw.chiefTree && c.dir != rs.tree {
				t.Errorf("%s ran in %q, not in chief's worktree or the one removed", c.line(), c.dir)
			}
		}
	}
	if !sawTree || !sawOther {
		t.Errorf("git ran in the worktree %v and in the other directory %v; the test reached neither the checks nor the lookup", sawTree, sawOther)
	}
	if got, want := writesOf(sw.rec), rs.removalWrites(); !slices.Equal(got, want) {
		t.Errorf("writes %q, want %q", got, want)
	}
}
