//go:build test

package chief

import (
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
