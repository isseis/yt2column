//go:build test

package fixprguard

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
	"github.com/isseis/yt2column/internal/fixprguard/fixprguardtest"
)

const (
	testBranch = "feature"
	testRemote = "origin"
	// testFile is the file the initial commit holds and the threads point at.
	testFile = "docs/notes/a.md"

	// testDirMode is the mode of directories a test creates.
	testDirMode = 0o750
)

// configListResult is what git config --list returns when a test replaces
// it: its output and error, which the test builds.
type configListResult struct {
	Out []byte
	Err error
}

// gitRouter runs git for real and every other command (make, gh) through the
// fake. Calls records each command's name and first argument in order, so a
// test can see where a run stopped. ConfigList, when set, answers git config
// --list without running git; the call is still recorded. List runs the
// listing of ignored files, which reaches it through ListRunner; a test may
// replace it.
type gitRouter struct {
	real       cmdrunner.Runner
	List       cmdrunner.Runner
	fake       *cmdrunnertestutil.Fake
	Calls      []string
	ConfigList *configListResult
	// Before runs a function right before the git call it is keyed by (as
	// Calls records it), outside the router, so a test can change the
	// repository between two calls of the code under test. It runs before
	// every matching call and covers the git runner only, not ListRunner.
	Before map[string]func()
}

// ListRunner returns the runner a Worktree lists ignored files with: it
// records the call in Calls, as the git runner does, and runs it through
// List.
func (r *gitRouter) ListRunner() cmdrunner.Runner { return listRoute{r} }

type listRoute struct{ r *gitRouter }

func (l listRoute) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	l.r.Calls = append(l.r.Calls, name+" "+gitSubcommand(args))
	return l.r.List.Run(ctx, name, args...)
}

func (r *gitRouter) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name != gitCommand {
		r.Calls = append(r.Calls, name+" "+strings.Join(args, " "))
		return r.fake.Run(ctx, name, args...)
	}
	call := gitCommand + " " + gitSubcommand(args)
	r.Calls = append(r.Calls, call)
	if f := r.Before[call]; f != nil {
		f()
	}
	if r.ConfigList != nil && call == gitConfigList {
		return r.ConfigList.Out, r.ConfigList.Err
	}
	return r.real.Run(ctx, name, args...)
}

// gitConfigList is how Calls records git config --list, apart from the
// git config --get calls that read one value.
const gitConfigList = gitCommand + " config --list"

// gitSubcommand skips the global options cmdrunner.GitHardening and Worktree
// add. It names config --list in full, so a test can tell it from config
// --get.
func gitSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-c":
			i++
		case strings.HasPrefix(args[i], "--"):
		case args[i] == "config" && i+1 < len(args) && args[i+1] == "--list":
			return "config --list"
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
// repository as origin. Every git the test starts, and every runner built for
// it, gets fixprguardtest.Env(Home, dir), so the user's git configuration
// (commit signing, hooks) cannot reach the test. The repository configuration
// turns signing off, and only hooks a test writes exist. The process's working
// directory is not changed, so tests that use it can run in parallel.
type testRepo struct {
	t      *testing.T
	Dir    string
	Remote string
	Home   string
}

// fixtureTemplate is built once by TestMain and copied by newTestRepo.
var fixtureTemplate *fixprguardtest.Template

// newTestRepo creates the repository with one commit holding testFile by
// copying the package's fixture template and refreshing the index stat cache.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	root := t.TempDir()
	if err := fixtureTemplate.Copy(root); err != nil {
		t.Fatalf("copy fixture template: %v", err)
	}
	r := &testRepo{t: t, Dir: filepath.Join(root, "work"), Remote: filepath.Join(root, "remote.git"), Home: filepath.Join(root, "home")}
	r.Git("update-index", "-q", "--refresh")
	return r
}

// buildTestRepoTemplate runs buildTestRepo with real git commands using
// fixprguardtest.Env, for TestMain's BuildTemplate and fixture equivalence tests.
func buildTestRepoTemplate(root string) error {
	work := filepath.Join(root, "work")
	home := filepath.Join(root, "home")
	gitIn := func(dir string, args ...string) error {
		cmd := exec.Command(gitCommand, args...) //nolint:gosec // test setup runs git with fixed arguments
		cmd.Dir = dir
		cmd.Env = fixprguardtest.Env(home, dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
		return nil
	}
	gitWork := func(args ...string) error {
		return gitIn(work, args...)
	}
	return buildTestRepo(root, gitIn, gitWork)
}

// buildTestRepo creates the initial repository fixture under root using the
// supplied git callbacks.
func buildTestRepo(root string, gitIn func(dir string, args ...string) error, gitWork func(args ...string) error) error {
	work := filepath.Join(root, "work")
	remote := filepath.Join(root, "remote.git")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("create home: %w", err)
	}
	if err := gitIn(root, "init", "-q", "--bare", "--template=", remote); err != nil {
		return err
	}
	if err := gitIn(root, "init", "-q", "--template=", "-b", testBranch, work); err != nil {
		return err
	}
	if err := pinIgnoreConfig(home, gitWork); err != nil {
		return err
	}
	if err := gitWork("config", "user.name", "Test"); err != nil {
		return err
	}
	if err := gitWork("config", "user.email", "test@example.com"); err != nil {
		return err
	}
	if err := gitWork("config", "commit.gpgSign", "false"); err != nil {
		return err
	}
	if err := gitWork("config", "tag.gpgSign", "false"); err != nil {
		return err
	}
	if err := gitWork("remote", "add", testRemote, remote); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(work, "docs/notes"), testDirMode); err != nil {
		return fmt.Errorf("create dir for docs/notes: %w", err)
	}
	if err := os.WriteFile(filepath.Join(work, testFile), []byte("one\ntwo\n"), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
		return fmt.Errorf("write %s: %w", testFile, err)
	}
	if err := os.WriteFile(filepath.Join(work, "docs/notes/b.md"), []byte("b\n"), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
		return fmt.Errorf("write docs/notes/b.md: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(work, "internal/x"), testDirMode); err != nil {
		return fmt.Errorf("create dir for internal/x: %w", err)
	}
	if err := os.WriteFile(filepath.Join(work, "internal/x/x.go"), []byte("package x\n"), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
		return fmt.Errorf("write internal/x/x.go: %w", err)
	}
	if err := os.WriteFile(filepath.Join(work, "internal/x/x_test.go"), []byte("package x\n"), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
		return fmt.Errorf("write internal/x/x_test.go: %w", err)
	}
	if err := os.WriteFile(filepath.Join(work, "Makefile"), []byte("all:\n"), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
		return fmt.Errorf("write Makefile: %w", err)
	}
	if err := commitAll(gitWork, "initial"); err != nil {
		return err
	}
	return gitWork("push", "-q", "-u", testRemote, testBranch)
}

func commitAll(gitWork func(...string) error, message string) error {
	if err := gitWork("add", "-A"); err != nil {
		return err
	}
	return gitWork("commit", "-q", "-m", message)
}

// pinIgnoreConfig makes which files git ignores, and how it compares their
// names, the same on every machine: the global excludes file is an empty
// file under home, and case and Unicode composition are significant, as on a
// Linux CI file system, even on a case-insensitive macOS volume.
func pinIgnoreConfig(home string, gitWork func(...string) error) error {
	excludes := filepath.Join(home, "excludes")
	if err := os.WriteFile(excludes, nil, 0o600); err != nil { //nolint:gosec // a path inside the test's temporary home
		return fmt.Errorf("write excludes: %w", err)
	}
	if err := gitWork("config", "core.excludesFile", excludes); err != nil {
		return err
	}
	if err := gitWork("config", "core.ignorecase", "false"); err != nil {
		return err
	}
	return gitWork("config", "core.precomposeunicode", "false")
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
	cmd.Env = fixprguardtest.Env(r.Home, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// ConfigList returns git config --list --show-origin -z as the tool reads it,
// so a test can tell whether a change reached the listed values.
func (r *testRepo) ConfigList() string {
	r.t.Helper()
	cmd := exec.Command(gitCommand, append(cmdrunner.GitHardening(), configListArgs...)...) //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = r.Dir
	cmd.Env = fixprguardtest.Env(r.Home, r.Dir)
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git config --list: %v", err)
	}
	return string(out)
}

// AddWorktree adds a linked worktree at dir on a new branch made from the
// remote's feature branch, as another session's git worktree add -b does;
// starting from the remote-tracking branch writes branch.<branch>.*.
func (r *testRepo) AddWorktree(dir, branch string) {
	r.t.Helper()
	r.Git("worktree", "add", "-q", "-b", branch, dir, testRemote+"/"+testBranch)
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

// mustWorktreeDir builds a WorktreeDir for dir or fails the test.
func mustWorktreeDir(t *testing.T, dir string) WorktreeDir {
	t.Helper()
	wd, err := NewWorktreeDir(dir)
	if err != nil {
		t.Fatalf("NewWorktreeDir(%q): %v", dir, err)
	}
	return wd
}

// newTestWorktree returns a Worktree targeting dir whose git runs for real
// with fixprguardtest.Env(home, dir) and Options.Dir set to dir, and whose
// other commands run through a fake expecting exactly steps. One dir makes
// both the WorktreeDir and the runners' Dir, so they cannot disagree. A test
// with no repository passes an empty directory of its own as home.
func newTestWorktree(t *testing.T, dir, home string, steps ...cmdrunnertestutil.Step) (*Worktree, *gitRouter) {
	t.Helper()
	wd := mustWorktreeDir(t, dir)
	env := fixprguardtest.Env(home, dir)
	router := &gitRouter{
		real: cmdrunner.NewOS(cmdrunner.Options{Dir: dir, Env: env}),
		List: cmdrunner.NewOS(cmdrunner.Options{Dir: dir, Env: env, StderrIsFailure: true}),
		fake: cmdrunnertestutil.New(t, steps...),
	}
	return NewWorktree(wd, router, router.ListRunner(), router), router
}

// requireNonRoot fails the test when it runs as root, where permission
// failures cannot be produced. Such a run is an unsupported environment, so it
// fails instead of being skipped silently.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("unsupported test environment: permission-failure tests require a non-root user")
	}
}

// chmodForTest changes path's mode and restores the original mode at cleanup,
// registered after any t.TempDir of the caller, so the directory can be
// removed afterwards.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	original := info.Mode().Perm()
	if err := os.Chmod(path, mode); err != nil { //nolint:gosec // a path inside the test's temporary directory, made unreadable on purpose
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, original) //nolint:gosec // restores the mode the test changed
	})
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

// mustBaseline takes a baseline of dir with a Worktree that runs no other command.
func mustBaseline(t *testing.T, dir, home string) Baseline {
	t.Helper()
	w, _ := newTestWorktree(t, dir, home)
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

// mustInspect inspects dir with threads on testFile and returns the judged set.
func mustInspect(t *testing.T, dir, home string, base Baseline) Inspection {
	t.Helper()
	w, _ := newTestWorktree(t, dir, home)
	inspection, err := w.Inspect(t.Context(), base, threadAt(testFile))
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	return inspection
}
