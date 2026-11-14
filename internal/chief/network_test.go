//go:build test && unix

package chief

import (
	"context"
	"slices"
	"strings"
	"testing"
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
// writes are prompts to the task workspace's panes.
func TestDetailAndSendRunOnlyReadsInTheRightPlaces(t *testing.T) {
	t.Setenv("GH_REPO", "other/repo")
	t.Setenv("GH_HOST", "ghe.example.com")
	sw, tree := detailWorld(t)
	deps := sw.finish()
	sw.firstPane().screen = reportScreen(t, prURL)
	if _, err := Detail(context.Background(), deps, tree); err != nil {
		t.Fatal(err)
	}
	out := sendRunpr(t, deps, tree)
	if len(out.Actions) != 2 || out.Actions[1].Outcome != OutcomeSent {
		t.Fatalf("actions = %+v; the test reached no reset", out.Actions)
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
