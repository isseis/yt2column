//go:build test

package chief

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestParseWorktreeListSample(t *testing.T) {
	got, err := parseWorktreeList(readTestdata(t, "git/worktree-list-porcelain.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []WorktreeEntry{
		{Path: "/Users/user/src/repo/main"},
		{Path: "/Users/user/src/repo/detached"},
		{Path: "/Users/user/src/repo/gone", Prunable: true},
		{Path: "/Users/user/src/repo/locked", Locked: true},
		{Path: "/Users/user/src/repo/lockednoreason", Locked: true},
		{Path: "/Users/user/src/repo/onbranch"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("parseWorktreeList = %+v, want %+v", got, want)
	}
}

func TestParseWorktreeListRejects(t *testing.T) {
	cases := map[string]string{
		"empty":                  "",
		"a block with no path":   "HEAD 6f1d\n\n",
		"a relative path":        "worktree repo\nHEAD 6f1d\n\n",
		"a quoted path":          "worktree \"/tmp/a\\tb\"\nHEAD 6f1d\n\n",
		"a repeated path":        "worktree /a\nHEAD 6f1d\n\nworktree /a\nHEAD 6f1d\n\n",
		"a second block broken":  "worktree /a\nHEAD 6f1d\n\nbranch refs/heads/x\n\n",
		"no line end at the end": "worktree /a\nHEAD 6f1d",
	}
	for name, out := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := parseWorktreeList([]byte(out)); !errors.Is(err, errGitOutput) {
				t.Errorf("parseWorktreeList = %+v, %v, want errGitOutput", got, err)
			}
		})
	}
}

// TestGitWorktreesOfARealRepository reads the output of the real git for a
// detached, a locked, and a prunable worktree.
func TestGitWorktreesOfARealRepository(t *testing.T) {
	repo := newTestRepo(t)
	base := filepath.Dir(repo.Dir)
	detached, locked, gone := filepath.Join(base, "detached"), filepath.Join(base, "locked"), filepath.Join(base, "gone")
	repo.Git("worktree", "add", "-q", "--detach", detached)
	repo.Git("worktree", "add", "-q", "--detach", locked)
	repo.Git("worktree", "lock", "--reason", "pinned", locked)
	repo.Git("worktree", "add", "-q", "--detach", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	git := NewGitReader(repo.RealCommands())
	got, err := git.Worktrees(t.Context(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]WorktreeEntry{}
	for _, e := range got {
		byPath[e.Path] = e
	}
	want := map[string]WorktreeEntry{
		repo.Dir: {Path: repo.Dir},
		detached: {Path: detached},
		locked:   {Path: locked, Locked: true},
		gone:     {Path: gone, Prunable: true},
	}
	if !maps.Equal(byPath, want) || len(got) != len(want) {
		t.Errorf("Worktrees = %+v, want %+v", got, want)
	}
}

func TestGitIsAncestor(t *testing.T) {
	repo := newTestRepo(t)
	first := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Write("second.txt", "x\n")
	second := repo.CommitAll("second")
	repo.Git("switch", "-q", "-c", "side", first)
	repo.Write("side.txt", "x\n")
	side := repo.CommitAll("side")
	git := NewGitReader(repo.RealCommands())
	ctx := t.Context()
	cases := []struct {
		name                 string
		ancestor, descendant string
		want                 bool
		wantErr              bool
	}{
		{"an ancestor", first, second, true, false},
		{"the same commit", second, second, true, false},
		{"not an ancestor", second, first, false, false},
		{"commits on two branches", second, side, false, false},
		{"a commit that does not exist", strings.Repeat("0", 40), second, false, true},
		{"not a full commit ID", "HEAD", second, false, true},
		{"an option", "--all", second, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := git.IsAncestor(ctx, repo.Dir, c.ancestor, c.descendant)
			if (err != nil) != c.wantErr || got != c.want {
				t.Errorf("IsAncestor = %v, %v, want %v (error %v)", got, err, c.want, c.wantErr)
			}
		})
	}
}

// TestGitCleanCountsStagedAndNestedFiles runs Clean against the real git: a
// staged change and an untracked file in a nested new directory are not
// clean, and an ignored file is.
func TestGitCleanCountsStagedAndNestedFiles(t *testing.T) {
	repo := newTestRepo(t)
	repo.Write(".gitignore", "ignored/\n")
	repo.CommitAll("ignore")
	git := NewGitReader(repo.RealCommands())
	clean := func() bool {
		t.Helper()
		ok, err := git.Clean(t.Context(), repo.Dir)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !clean() {
		t.Fatal("a clean worktree reads as changed")
	}
	repo.Write("ignored/deep/file.txt", "x\n")
	if !clean() {
		t.Error("a file in an ignored directory reads as a change")
	}
	repo.Write("a/b/c/new.txt", "x\n")
	if clean() {
		t.Error("an untracked file in a nested new directory reads as clean")
	}
	repo.Git("add", "a/b/c/new.txt")
	if clean() {
		t.Error("a staged new file reads as clean")
	}
	repo.Git("reset", "-q")
	if err := os.RemoveAll(filepath.Join(repo.Dir, "a")); err != nil {
		t.Fatal(err)
	}
	repo.Write("README.md", "staged change\n")
	repo.Git("add", "README.md")
	if clean() {
		t.Error("a staged change to a tracked file reads as clean")
	}
}
