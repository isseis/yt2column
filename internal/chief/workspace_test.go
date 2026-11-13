//go:build test

package chief

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePRList(t *testing.T) {
	prs, err := parsePRList(readTestdata(t, "gh/pr-list-open-867.json"), "owner/0087-openai-combination-eval-05")
	if err != nil || len(prs) != 1 || prs[0].Number != 867 || prs[0].URL != "https://github.com/owner/yt2column/pull/867" {
		t.Fatalf("parsePRList = %+v, %v", prs, err)
	}
	if prs, err := parsePRList(readTestdata(t, "gh/pr-list-none.json"), "any"); err != nil || len(prs) != 0 {
		t.Errorf("an empty list = %+v, %v", prs, err)
	}
	rejected := map[string]string{
		"another branch":   `[{"headRefName":"other","number":1,"title":"t","url":"u"}]`,
		"zero number":      `[{"headRefName":"b","number":0,"title":"t","url":"u"}]`,
		"missing url":      `[{"headRefName":"b","number":1,"title":"t"}]`,
		"object not array": `{"headRefName":"b"}`,
	}
	for name, doc := range rejected {
		if _, err := parsePRList([]byte(doc), "b"); !errors.Is(err, errPRList) {
			t.Errorf("%s: error = %v, want errPRList", name, err)
		}
	}
}

func TestBranch(t *testing.T) {
	for out, want := range map[string]string{"user/0001-x-01\n": "user/0001-x-01", "HEAD\n": ""} {
		rec := newRecorder(t)
		rec.reply("git rev-parse --abbrev-ref HEAD", []byte(out))
		got, err := NewGitReader(rec.commands(nil)).Branch(context.Background(), t.TempDir())
		if err != nil || got != want {
			t.Errorf("Branch(%q) = %q, %v; want %q", out, got, err, want)
		}
	}
}

// TestFindTaskWorkspaces checks the chief and task workspaces are told apart
// by the git common directory: the chief's own workspace and a workspace of
// another repository or of no repository are not task workspaces.
func TestFindTaskWorkspaces(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "task", Branch: "main"})
	// A second pane of the chief's own workspace must not count.
	w.agents = append(w.agents, fakeAgent{Pane: "wc:p2", Workspace: "wc", Kind: "claude", Status: "idle", Dir: w.chiefTree, Seq: 2})
	other := filepath.Join(filepath.Dir(w.chiefTree), "elsewhere")
	notRepo := filepath.Join(filepath.Dir(w.chiefTree), "norepo")
	for _, d := range []string{other, notRepo} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	w.agents = append(w.agents,
		fakeAgent{Pane: "w9:p1", Workspace: "w9", Kind: "claude", Status: "idle", Dir: other, Seq: 3},
		fakeAgent{Pane: "w8:p1", Workspace: "w8", Kind: "claude", Status: "idle", Dir: notRepo, Seq: 4})
	w.labels = append(w.labels, [2]string{"w9", "another repo"}, [2]string{"w8", "no repo"})
	w.git(other, "rev-parse --path-format=absolute --git-common-dir", filepath.Join(filepath.Dir(w.chiefTree), "other.git"))
	if err := os.MkdirAll(filepath.Join(filepath.Dir(w.chiefTree), "other.git"), 0o700); err != nil {
		t.Fatal(err)
	}
	w.rec.failIn(notRepo, "git rev-parse --path-format=absolute --git-common-dir", errors.New("not a git repository"))
	deps := w.finish()
	cmds := deps.Cmds
	herdr := NewHerdr(cmds)
	agents, err := herdr.Agents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	spaces, err := herdr.Workspaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	chiefPane, _ := ParsePaneID(worldChiefPane)
	found, err := FindTaskWorkspaces(context.Background(), NewGitReader(cmds), cmds, agents, spaces, chiefPane)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].ID != "w1" || found[0].Label != "task" || found[0].Worktree != w.trees["w1"] {
		t.Errorf("found = %+v, want only the task workspace w1", found)
	}
	t.Run("the chief pane is not a Herdr agent", func(t *testing.T) {
		gone, _ := ParsePaneID("zz:p9")
		if _, err := FindTaskWorkspaces(context.Background(), NewGitReader(cmds), cmds, agents, spaces, gone); !errors.Is(err, errHerdrOutput) {
			t.Errorf("error = %v, want errHerdrOutput", err)
		}
	})
}
