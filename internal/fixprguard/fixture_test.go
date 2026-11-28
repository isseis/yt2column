//go:build test

package fixprguard

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/fixprguard/fixprguardtest"
)

func TestFixtureTemplateEquivalence(t *testing.T) {
	t.Parallel()

	// Verify newTestRepo itself leaves the index stat cache refreshed before any
	// git command that might rewrite the index.
	repo := newTestRepo(t)
	runFixtureGit(t, repo.Home, repo.Dir, []string{"GIT_OPTIONAL_LOCKS=0"}, "diff-files", "--quiet")

	freshRoot := filepath.Join(t.TempDir(), "fresh")
	if err := os.MkdirAll(freshRoot, 0o700); err != nil {
		t.Fatalf("mkdir fresh: %v", err)
	}
	if err := buildTestRepoTemplate(freshRoot); err != nil {
		t.Fatalf("buildTestRepoTemplate: %v", err)
	}

	copyRoot := filepath.Join(t.TempDir(), "copy with #01 space")
	if err := os.MkdirAll(copyRoot, 0o700); err != nil {
		t.Fatalf("mkdir copy: %v", err)
	}
	if err := fixtureTemplate.Copy(copyRoot); err != nil {
		t.Fatalf("fixtureTemplate.Copy: %v", err)
	}
	copyWork := filepath.Join(copyRoot, "work")
	copyHome := filepath.Join(copyRoot, "home")
	copyRemote := filepath.Join(copyRoot, "remote.git")
	runFixtureGit(t, copyHome, copyWork, nil, "update-index", "-q", "--refresh")

	// Check index stat cache before any git command that might refresh it.
	runFixtureGit(t, copyHome, copyWork, []string{"GIT_OPTIONAL_LOCKS=0"}, "diff-files", "--quiet")

	freshWork := filepath.Join(freshRoot, "work")
	freshHome := filepath.Join(freshRoot, "home")
	freshRemote := filepath.Join(freshRoot, "remote.git")

	// 1. Work tree files (excluding .git): list, type, perm, content.
	compareDirTree(t, "worktree", freshWork, copyWork, "", "", true, func(rel string) bool { return rel == ".git" }, func(string) bool { return true })

	// 2. work/.git and remote.git: list, type, perm; content of HEAD and COMMIT_EDITMSG only.
	// Normalize the loose commit object path because the commit OID depends on timestamp.
	freshCommitOID := strings.TrimSpace(runFixtureGit(t, freshHome, freshWork, nil, "rev-parse", "HEAD"))
	copyCommitOID := strings.TrimSpace(runFixtureGit(t, copyHome, copyWork, nil, "rev-parse", "HEAD"))
	metaContent := func(rel string) bool { return rel == "HEAD" || rel == "COMMIT_EDITMSG" }
	compareDirTree(t, "work/.git", filepath.Join(freshWork, ".git"), filepath.Join(copyWork, ".git"), freshCommitOID, copyCommitOID, false, nil, metaContent)
	compareDirTree(t, "remote.git", freshRemote, copyRemote, freshCommitOID, copyCommitOID, false, nil, metaContent)

	// 3. git config --list --local with root path normalized.
	for _, pair := range []struct {
		name     string
		freshDir string
		copyDir  string
	}{
		{"work", freshWork, copyWork},
		{"remote.git", freshRemote, copyRemote},
	} {
		freshCfg := strings.ReplaceAll(runFixtureGit(t, freshHome, pair.freshDir, nil, "config", "--list", "--local"), freshRoot, "<ROOT>")
		copyCfg := strings.ReplaceAll(runFixtureGit(t, copyHome, pair.copyDir, nil, "config", "--list", "--local"), copyRoot, "<ROOT>")
		if freshCfg != copyCfg {
			t.Errorf("%s git config --list --local differs:\nfresh:\n%s\ncopy:\n%s", pair.name, freshCfg, copyCfg)
		}
	}

	// 4. git config --get for the two rewritten paths.
	if got := strings.TrimSpace(runFixtureGit(t, copyHome, copyWork, nil, "config", "--get", "remote.origin.url")); got != copyRemote {
		t.Errorf("remote.origin.url = %q, want %q", got, copyRemote)
	}
	wantExcludes := filepath.Join(copyHome, "excludes")
	if got := strings.TrimSpace(runFixtureGit(t, copyHome, copyWork, nil, "config", "--get", "core.excludesFile")); got != wantExcludes {
		t.Errorf("core.excludesFile = %q, want %q", got, wantExcludes)
	}

	// 5. git ls-files -s, HEAD symbolic ref, all ref names, and per-ref tree/parents/message.
	if got, want := runFixtureGit(t, copyHome, copyWork, nil, "ls-files", "-s"), runFixtureGit(t, freshHome, freshWork, nil, "ls-files", "-s"); got != want {
		t.Errorf("ls-files -s differs:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if got, want := runFixtureGit(t, copyHome, copyWork, nil, "symbolic-ref", "HEAD"), runFixtureGit(t, freshHome, freshWork, nil, "symbolic-ref", "HEAD"); got != want {
		t.Errorf("symbolic-ref HEAD = %q, want %q", got, want)
	}
	freshRefs := strings.Fields(runFixtureGit(t, freshHome, freshWork, nil, "for-each-ref", "--format=%(refname)"))
	copyRefs := strings.Fields(runFixtureGit(t, copyHome, copyWork, nil, "for-each-ref", "--format=%(refname)"))
	if !slices.Equal(freshRefs, copyRefs) {
		t.Fatalf("refs = %v, want %v", copyRefs, freshRefs)
	}
	for _, ref := range freshRefs {
		got := runFixtureGit(t, copyHome, copyWork, nil, "log", "-1", "--format=%T%x00%P%x00%B", ref)
		want := runFixtureGit(t, freshHome, freshWork, nil, "log", "-1", "--format=%T%x00%P%x00%B", ref)
		if got != want {
			t.Errorf("ref %s commit metadata differs:\ngot:  %q\nwant: %q", ref, got, want)
		}
	}

	// 6. git status --porcelain is empty.
	if status := runFixtureGit(t, copyHome, copyWork, nil, "status", "--porcelain"); status != "" {
		t.Errorf("git status --porcelain = %q, want empty", status)
	}

	// 7. home files: list, perm, content.
	compareDirTree(t, "home", freshHome, copyHome, "", "", false, nil, func(string) bool { return true })

	// 8. No copied file (reflogs and remote.git/config included) still names the
	// template root: only the two rewritten config values may hold a root path.
	assertNoTemplateRoot(t, copyRoot, isolation.TemplateRoot)
}

// assertNoTemplateRoot fails when any regular file under root contains the
// bytes of templateRoot.
func assertNoTemplateRoot(t *testing.T, root, templateRoot string) {
	t.Helper()
	needle := []byte(templateRoot)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // reads a file inside the test's temporary fixture copy
		if err != nil {
			return err
		}
		if bytes.Contains(data, needle) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("copied file %s contains the template root %q", rel, templateRoot)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func TestFixtureTemplateIsolation(t *testing.T) {
	t.Parallel()
	r1 := newTestRepo(t)
	r2 := newTestRepo(t)

	origHead := r2.Head()
	origRemote := r2.RemoteHead()

	// Mutate r1's work tree, .git/config, and remote.git.
	r1.Write(testFile, "mutated\n")
	r1.Write("extra.txt", "extra\n")
	r1.Git("config", "user.name", "Mutated")
	r1.CommitAll("mutated")
	r1.Git("push", "-q", testRemote, testBranch)

	// Check r2 and a fresh copy r3 (proving the template itself is untouched).
	r3 := newTestRepo(t)
	for name, r := range map[string]*testRepo{"r2": r2, "r3": r3} {
		data, err := os.ReadFile(filepath.Join(r.Dir, testFile))
		if err != nil {
			t.Fatalf("%s read %s: %v", name, testFile, err)
		}
		if string(data) != "one\ntwo\n" {
			t.Errorf("%s %s = %q, want %q", name, testFile, string(data), "one\ntwo\n")
		}
		if _, err := os.Lstat(filepath.Join(r.Dir, "extra.txt")); !os.IsNotExist(err) {
			t.Errorf("%s unexpectedly contains extra.txt (err=%v)", name, err)
		}
		if got := strings.TrimSpace(r.Git("config", "--get", "user.name")); got != "Test" {
			t.Errorf("%s user.name = %q, want Test", name, got)
		}
		if got := r.Head(); got != origHead {
			t.Errorf("%s HEAD = %q, want %q", name, got, origHead)
		}
		if got := r.RemoteHead(); got != origRemote {
			t.Errorf("%s RemoteHead = %q, want %q", name, got, origRemote)
		}
	}
}

func runFixtureGit(t *testing.T, home, dir string, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(gitCommand, args...) //nolint:gosec // test helper runs git with fixed arguments
	cmd.Dir = dir
	cmd.Env = append(fixprguardtest.Env(home, dir), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return string(out)
}

type treeEntry struct {
	rel     string
	mode    fs.FileMode
	content []byte
}

func compareDirTree(t *testing.T, label, freshDir, copyDir, freshCommitOID, copyCommitOID string, skipDir bool, skip func(string) bool, readContent func(string) bool) {
	t.Helper()
	freshEntries := walkFixtureTree(t, freshDir, freshCommitOID, skipDir, skip, readContent)
	copyEntries := walkFixtureTree(t, copyDir, copyCommitOID, skipDir, skip, readContent)
	if len(freshEntries) != len(copyEntries) {
		t.Fatalf("%s entry count = %d, want %d\nfresh: %v\ncopy:  %v", label, len(copyEntries), len(freshEntries), entryNames(freshEntries), entryNames(copyEntries))
	}
	for i := range freshEntries {
		a, b := freshEntries[i], copyEntries[i]
		if a.rel != b.rel {
			t.Errorf("%s[%d] path = %q, want %q", label, i, b.rel, a.rel)
			continue
		}
		if a.mode != b.mode {
			t.Errorf("%s %s mode = %v, want %v", label, a.rel, b.mode, a.mode)
		}
		if !bytes.Equal(a.content, b.content) {
			t.Errorf("%s %s content differs:\ngot:  %q\nwant: %q", label, a.rel, b.content, a.content)
		}
	}
}

func walkFixtureTree(t *testing.T, root, commitOID string, skipDir bool, skip func(string) bool, readContent func(string) bool) []treeEntry {
	t.Helper()
	commitFanout := ""
	commitObj := ""
	if len(commitOID) == 40 {
		commitFanout = "objects/" + commitOID[:2]
		commitObj = commitFanout + "/" + commitOID[2:]
	}
	var out []treeEntry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel) {
			if skipDir && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip loose-object 2-hex fanout directories when comparing commit-OID-bearing
		// object stores: a commit OID's prefix can collide with a blob/tree prefix on
		// one side only, changing the fanout directory count by 1.
		if commitFanout != "" && d.IsDir() && strings.HasPrefix(rel, "objects/") && len(rel) == len("objects/xx") {
			return nil
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if commitObj != "" && rel == commitObj {
			rel = "objects/<COMMIT_OID>"
		}
		var content []byte
		if info.Mode().IsRegular() && readContent(rel) {
			content, err = os.ReadFile(p)
			if err != nil {
				return err
			}
		}
		out = append(out, treeEntry{rel: rel, mode: info.Mode(), content: content})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.SortFunc(out, func(a, b treeEntry) int { return strings.Compare(a.rel, b.rel) })
	return out
}

func entryNames(entries []treeEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.rel
	}
	return names
}
