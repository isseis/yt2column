//go:build test

package fixprguard

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

const (
	testBranch = "feature"
	testRemote = "origin"
	// testFile is the file the initial commit holds and the threads point at.
	testFile = "docs/notes/a.md"

	// testDirMode is the mode of directories a test creates.
	testDirMode = 0o750
)

// gitRouter runs git for real and every other command (make, gh) through the
// fake. Calls records each command's name and first argument in order, so a
// test can see where a run stopped.
type gitRouter struct {
	real  cmdrunner.Runner
	fake  *cmdrunnertestutil.Fake
	Calls []string
}

func (r *gitRouter) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name != gitCommand {
		r.Calls = append(r.Calls, name+" "+strings.Join(args, " "))
		return r.fake.Run(ctx, name, args...)
	}
	r.Calls = append(r.Calls, gitCommand+" "+gitSubcommand(args))
	return r.real.Run(ctx, name, args...)
}

// gitSubcommand skips the global options gitHardening and Worktree add.
func gitSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-c":
			i++
		case strings.HasPrefix(args[i], "--"):
		default:
			return args[i]
		}
	}
	return ""
}

// gitCalls counts the git commands among calls.
func gitCalls(calls []string) int {
	n := 0
	for _, c := range calls {
		if strings.HasPrefix(c, gitCommand+" ") {
			n++
		}
	}
	return n
}

// testRepo is a work tree on branch feature, pushed to a local bare
// repository as origin. The test's current directory is the work tree, and
// HOME and XDG_CONFIG_HOME point at an empty directory, so the user's git
// configuration (commit signing, hooks) cannot reach the test. The repository
// configuration turns signing off, and only hooks a test writes exist.
type testRepo struct {
	t      *testing.T
	Dir    string
	Remote string
	Home   string
}

// newTestRepo creates the repository with one commit holding testFile and
// changes the current directory into it. Tests that use it cannot run in
// parallel.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	root := t.TempDir()
	r := &testRepo{t: t, Dir: filepath.Join(root, "work"), Remote: filepath.Join(root, "remote.git"), Home: filepath.Join(root, "home")}
	if err := os.MkdirAll(r.Home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	t.Setenv("HOME", r.Home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(r.Home, ".config"))
	r.gitIn(root, "init", "-q", "--bare", "--template=", r.Remote)
	r.gitIn(root, "init", "-q", "--template=", "-b", testBranch, r.Dir)
	t.Chdir(r.Dir)
	r.Git("config", "user.name", "Test")
	r.Git("config", "user.email", "test@example.com")
	r.Git("config", "commit.gpgSign", "false")
	r.Git("config", "tag.gpgSign", "false")
	r.Git("remote", "add", testRemote, r.Remote)
	r.Write(testFile, "one\ntwo\n")
	r.Write("docs/notes/b.md", "b\n")
	r.Write("internal/x/x.go", "package x\n")
	r.Write("internal/x/x_test.go", "package x\n")
	r.Write("Makefile", "all:\n")
	r.CommitAll("initial")
	r.Git("push", "-q", "-u", testRemote, testBranch)
	return r
}

// Git runs git in the work tree for the test's own setup, outside the tool.
func (r *testRepo) Git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.Dir, args...)
}

func (r *testRepo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(gitCommand, args...) //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = dir
	cmd.Env = GitEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// Write writes a file in the work tree, creating its directory.
func (r *testRepo) Write(name, content string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, name)
	if err := os.MkdirAll(filepath.Dir(path), testDirMode); err != nil {
		r.t.Fatalf("create dir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
		r.t.Fatalf("write %s: %v", name, err)
	}
}

// WriteHook writes an executable hook into the repository's hooks directory.
func (r *testRepo) WriteHook(name, script string) {
	r.t.Helper()
	dir := filepath.Join(r.Dir, ".git", "hooks")
	if err := os.MkdirAll(dir, testDirMode); err != nil {
		r.t.Fatalf("create hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o700); err != nil { //nolint:gosec // a hook of the test's temporary repository
		r.t.Fatalf("write hook %s: %v", name, err)
	}
}

// CommitAll stages everything and commits.
func (r *testRepo) CommitAll(message string) {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", message)
}

// Head returns the HEAD commit.
func (r *testRepo) Head() string {
	r.t.Helper()
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

// RemoteHead returns the remote's feature branch commit.
func (r *testRepo) RemoteHead() string {
	r.t.Helper()
	return strings.TrimSpace(r.gitIn(r.Remote, "rev-parse", headsPrefix+testBranch))
}

// newTestWorktree returns a Worktree whose git runs for real with GitEnv and
// whose other commands run through a fake expecting exactly steps.
func newTestWorktree(t *testing.T, steps ...cmdrunnertestutil.Step) (*Worktree, *gitRouter) {
	t.Helper()
	router := &gitRouter{
		real: cmdrunner.NewOS(cmdrunner.Options{Env: GitEnv(os.Environ())}),
		fake: cmdrunnertestutil.New(t, steps...),
	}
	return NewWorktree(router), router
}

// makeStep scripts one make target; effect, when set, stands for what the
// target writes.
func makeStep(stage string, effect func() error) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: makeCommand, Args: []string{stage}, Effect: effect}
}

// buildCheckSteps scripts make fmt, test and lint with no effect.
func buildCheckSteps() []cmdrunnertestutil.Step {
	return []cmdrunnertestutil.Step{makeStep(StageFmt, nil), makeStep(StageTest, nil), makeStep(StageLint, nil)}
}

// mustBaseline takes a baseline with a Worktree that runs no other command.
func mustBaseline(t *testing.T) Baseline {
	t.Helper()
	w, _ := newTestWorktree(t)
	base, err := w.Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	return base
}

// threadAt is a trusted thread on path.
func threadAt(path string) []ThreadPath {
	return []ThreadPath{{ThreadID: testThreadID(1), Path: path}}
}

// mustInspect inspects with threads on testFile and returns the judged set.
func mustInspect(t *testing.T, base Baseline) Inspection {
	t.Helper()
	w, _ := newTestWorktree(t)
	inspection, err := w.Inspect(t.Context(), base, threadAt(testFile))
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	return inspection
}
