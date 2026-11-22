//go:build test

package ciskip

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

const (
	testSHA1        = "1111111111111111111111111111111111111111"
	testSHA2        = "2222222222222222222222222222222222222222"
	testSHA3        = "3333333333333333333333333333333333333333"
	testSHA4        = "4444444444444444444444444444444444444444"
	testRepoDirMode = 0o750
)

// gitArgs builds the full argument list runGit passes to cmdrunner.Runner.
func gitArgs(args ...string) []string {
	return append(cmdrunner.GitHardening(), args...)
}

// writeTempFile writes content to a file inside t.TempDir() and returns its path.
func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// testGitRepo is an isolated temporary git repository for integration tests.
// HOME and XDG_CONFIG_HOME point at an empty directory under t.TempDir() so the
// user's global git configuration cannot affect commits or diffs.
type testGitRepo struct {
	t    *testing.T
	dir  string
	home string
}

func newTestGitRepo(t *testing.T) *testGitRepo {
	t.Helper()
	root := t.TempDir()
	r := &testGitRepo{
		t:    t,
		dir:  filepath.Join(root, "repo"),
		home: filepath.Join(root, "home"),
	}
	if err := os.MkdirAll(r.home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	t.Setenv("HOME", r.home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(r.home, ".config"))
	r.gitIn(root, "init", "-q", "--template=", "-b", "main", r.dir)
	r.git("config", "user.name", "Test")
	r.git("config", "user.email", "test@example.com")
	r.git("config", "commit.gpgSign", "false")
	r.git("config", "core.ignorecase", "false")
	return r
}

func (r *testGitRepo) git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.dir, args...)
}

func (r *testGitRepo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // test setup invokes git with fixed arguments inside t.TempDir
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

// commit builds a commit whose tree holds exactly files (slash-separated path -> content)
// with the given parents (none creates a root commit) and returns its 40-char SHA.
func (r *testGitRepo) commit(subject string, files map[string]string, parents ...string) string {
	r.t.Helper()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		r.t.Fatalf("read repo: %v", err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(r.dir, e.Name())); err != nil {
			r.t.Fatalf("clear work tree: %v", err)
		}
	}
	for name, content := range files {
		path := filepath.Join(r.dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), testRepoDirMode); err != nil {
			r.t.Fatalf("mkdir for %q: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // test setup writes inside t.TempDir
			r.t.Fatalf("write %q: %v", name, err)
		}
	}
	r.git("add", "-A")
	tree := r.git("write-tree")
	args := []string{"commit-tree", tree, "-m", subject}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return r.git(args...)
}

// setHead detaches HEAD at oid.
func (r *testGitRepo) setHead(oid string) {
	r.t.Helper()
	r.git("update-ref", "--no-deref", "HEAD", oid)
}

// setOriginMain points refs/remotes/origin/main at oid.
func (r *testGitRepo) setOriginMain(oid string) {
	r.t.Helper()
	r.git("update-ref", "refs/remotes/origin/main", oid)
}

// runners returns real cmdrunner instances: git pointed at r.dir and script pointed
// at the repository root (two levels above internal/ciskip).
func (r *testGitRepo) runners() (cmdrunner.Runner, cmdrunner.InputRunner) {
	r.t.Helper()
	gitOpts, scriptOpts, _ := runnerOptions(os.Environ(), defaultLimits())
	gitOpts.Dir = r.dir
	scriptOpts.Dir = filepath.Join("..", "..")
	return cmdrunner.NewOS(gitOpts), cmdrunner.NewOS(scriptOpts)
}
