//go:build test

package mergepr

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// testMainRef is origin/main's remote-tracking ref, spelled out here rather
// than taken from the production constant, so a change to how the check names
// the ref cannot change the test's setup with it.
const testMainRef = "refs/remotes/origin/main"

// testRepoDirMode is the mode of the directories a test commit creates.
const testRepoDirMode = 0o750

// gitRepo is a temporary git repository for the protocol-number check. HOME
// and XDG_CONFIG_HOME point at an empty directory, so the user's git
// configuration (commit signing, hooks, grep settings) cannot reach the test
// unless the test writes it. Commits are made from whole snapshots with
// commit-tree, so a test states every file of every commit and can give a
// commit any parents, a merge commit or a root commit included.
//
// The helper never writes Go source of its own: the snapshots, declaration
// lines included, are built in the _test.go files, because the check reads
// this file as part of internal/mergepr.
type gitRepo struct {
	t    *testing.T
	Dir  string
	Home string
}

// newGitRepo creates an empty repository on branch main. Tests that use it
// change HOME and cannot run in parallel.
func newGitRepo(t *testing.T) *gitRepo {
	t.Helper()
	root := t.TempDir()
	r := &gitRepo{t: t, Dir: filepath.Join(root, "repo"), Home: filepath.Join(root, "home")}
	if err := os.MkdirAll(r.Home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	t.Setenv("HOME", r.Home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(r.Home, ".config"))
	r.gitIn(root, "init", "-q", "--template=", "-b", mainBranch, r.Dir)
	r.Git("config", "user.name", "Test")
	r.Git("config", "user.email", "test@example.com")
	r.Git("config", "commit.gpgSign", "false")
	r.Git("config", "core.ignorecase", "false")
	return r
}

// Git runs git in the repository for the test's own setup, outside the check,
// and returns its output without the final newline.
func (r *gitRepo) Git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.Dir, args...)
}

func (r *gitRepo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(gitCommand, args...) //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = dir
	cmd.Env = cmdrunner.GitEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = string(exitErr.Stderr)
		}
		r.t.Fatalf("git %v: %v: %s", args, err, stderr)
	}
	return strings.TrimSuffix(string(out), "\n")
}

// Commit makes a commit whose tree holds exactly files (path to content),
// with the given parents (none makes a root commit), and returns its ID. It
// moves no branch.
func (r *gitRepo) Commit(subject string, files map[string]string, parents ...string) string {
	r.t.Helper()
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		r.t.Fatalf("read repository: %v", err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(r.Dir, e.Name())); err != nil {
			r.t.Fatalf("clear work tree: %v", err)
		}
	}
	for name, content := range files {
		path := filepath.Join(r.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), testRepoDirMode); err != nil {
			r.t.Fatalf("create dir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
			r.t.Fatalf("write %s: %v", name, err)
		}
	}
	r.Git("add", "-A")
	tree := r.Git("write-tree")
	args := []string{"commit-tree", tree, "-m", subject}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return r.Git(args...)
}

// SetMain points refs/remotes/origin/main at oid, as a fetch of main would.
func (r *gitRepo) SetMain(oid string) {
	r.t.Helper()
	r.Git("update-ref", testMainRef, oid)
}

// Check runs the production check in dir (the repository when empty).
func (r *gitRepo) Check(dir, head, base string) (protocolSummary, error) {
	r.t.Helper()
	if dir == "" {
		dir = r.Dir
	}
	return newProtocolCheck(newGitRunner(dir)).check(r.t.Context(), head, base)
}

// RemoveObject deletes the loose object file of oid, so reading it fails.
func (r *gitRepo) RemoveObject(oid string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, ".git", "objects", oid[:2], oid[2:])
	if err := os.Remove(path); err != nil {
		r.t.Fatalf("remove object %s: %v", oid, err)
	}
}

// ShallowClone clones the repository's branch main through a file:// URL
// with depth 1 (a local path would ignore --depth) and returns the clone's
// directory. The clone has refs/remotes/origin/main.
func (r *gitRepo) ShallowClone(mainOID string) string {
	r.t.Helper()
	r.Git("update-ref", "refs/heads/"+mainBranch, mainOID)
	dir := filepath.Join(filepath.Dir(r.Dir), "shallow")
	r.gitIn(filepath.Dir(r.Dir), "clone", "-q", "--depth", "1", "--template=", "file://"+filepath.ToSlash(r.Dir), dir)
	return dir
}
