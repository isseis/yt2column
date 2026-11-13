//go:build test

package chief

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The tests run inside a Herdr pane, where a real herdr, a real gh, and a
// running chief may exist. TestMain therefore points PATH at an empty
// directory before any test runs, so code under test that started a command
// by mistake finds nothing. The programs the tests run on purpose are
// resolved first and reached only through testToolsDir.
var (
	// testToolsDir holds links to the programs a test runs on purpose (git
	// and the utilities opus-review-decision.sh uses), never herdr or gh.
	testToolsDir string
	// testBash is the bash found on PATH when the package started, or "".
	testBash string
)

// testTools are the programs linked into testToolsDir.
var testTools = []string{gitProgram, "awk", "grep", "tr", "tail"}

// isolateTestPath resolves the test programs, links them into a new
// directory, and points PATH at another, empty directory. It returns the
// directories to remove after the tests.
func isolateTestPath() ([]string, error) {
	tools, err := os.MkdirTemp("", "chief-tools-")
	if err != nil {
		return nil, err
	}
	empty, err := os.MkdirTemp("", "chief-path-")
	if err != nil {
		return []string{tools}, err
	}
	dirs := []string{tools, empty}
	for _, name := range testTools {
		path, err := exec.LookPath(name)
		if err != nil {
			return dirs, err
		}
		if err := os.Symlink(path, filepath.Join(tools, name)); err != nil {
			return dirs, err
		}
	}
	if path, err := exec.LookPath("bash"); err == nil {
		testBash = path
	}
	testToolsDir = tools
	if err := os.Setenv("PATH", empty); err != nil {
		return dirs, err
	}
	for _, name := range []string{herdrProgram, ghProgram} {
		if path, err := exec.LookPath(name); err == nil {
			return dirs, fmt.Errorf("%w: %s at %s", errToolOnPath, name, path)
		}
	}
	return dirs, nil
}

// errToolOnPath reports that herdr or gh is still found after PATH was
// replaced.
var errToolOnPath = errors.New("a real tool is still found on PATH")

// testEnv is the whole environment a test gives the programs it runs: only
// the variables listed here, a HOME of its own, no system git configuration,
// and a fixed author, so the developer's git configuration and signing never
// apply.
func testEnv(home string) []string {
	return []string{
		"PATH=" + testToolsDir,
		"HOME=" + home,
		"LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	}
}

// testRepo is a temporary git repository for a test.
type testRepo struct {
	t    *testing.T
	Dir  string
	home string
}

// newTestRepo creates a repository on branch main with one commit, its
// signing turned off, in a resolved temporary directory.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	if testToolsDir == "" {
		t.Fatal("TestMain did not prepare the test tools")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve the temporary directory: %v", err)
	}
	r := &testRepo{t: t, Dir: filepath.Join(base, "repo"), home: filepath.Join(base, "home")}
	if err := os.Mkdir(r.home, 0o700); err != nil {
		t.Fatalf("create HOME: %v", err)
	}
	r.run(base, filepath.Join(testToolsDir, gitProgram), "init", "-q", "--template=", "-b", "main", r.Dir)
	r.Git("config", "commit.gpgSign", "false")
	r.Git("config", "tag.gpgSign", "false")
	r.Write("README.md", "initial\n")
	r.CommitAll("initial")
	return r
}

// Env returns the environment the repository's programs run with.
func (r *testRepo) Env() []string { return testEnv(r.home) }

// Git runs git in the repository and returns its standard output.
func (r *testRepo) Git(args ...string) string {
	r.t.Helper()
	return r.run(r.Dir, filepath.Join(testToolsDir, gitProgram), args...)
}

// Run runs program in the repository with the test environment and returns
// its standard output.
func (r *testRepo) Run(program string, args ...string) string {
	r.t.Helper()
	return r.run(r.Dir, program, args...)
}

func (r *testRepo) run(dir, program string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(program, args...) //nolint:gosec // test setup runs a resolved program with the test's arguments
	cmd.Dir = dir
	cmd.Env = r.Env()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("%s %v: %v: %s", filepath.Base(program), args, err, stderr.String())
	}
	return string(out)
}

// Write writes content to the path relative to the repository root.
func (r *testRepo) Write(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		r.t.Fatalf("create dir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		r.t.Fatalf("write %s: %v", rel, err)
	}
}

// CommitAll commits every change and returns the new HEAD.
func (r *testRepo) CommitAll(message string) string {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", message)
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

// recordedCall is one external command the code under test ran.
type recordedCall struct {
	name string
	// args are the arguments, for git without the hardening options before
	// the subcommand.
	args []string
	dir  string
	env  []string
	// limit is the time left before the command's context deadline when it
	// ran, or zero when the context had none.
	limit time.Duration
}

// line returns the command line used as a reply key: the name and the args.
func (c recordedCall) line() string {
	return strings.Join(append([]string{c.name}, c.args...), " ")
}

// reply is what the recorder returns for one command line.
type reply struct {
	out []byte
	err error
}

// errNoReply reports a command the test gave no reply for.
var errNoReply = errors.New("no reply recorded for the command")

// readOnlyCommands lists the commands the tool may run, by name and the words
// that select the operation. Every one reads: none pushes, merges, comments,
// or types into a pane. The recorder fails a test that runs anything else.
var readOnlyCommands = map[string][][]string{
	herdrProgram: {{herdrAgent, "list"}, {herdrAgent, "get"}, {herdrAgent, "read"}, {"workspace", "list"}}, //nolint:goconst // each pair names one herdr operation
	gitProgram:   {{"show"}, {"ls-tree"}, {"rev-parse"}},
	ghProgram:    {{"pr", "list"}, {"pr", "view"}, {"pr", "checks"}},
}

// recorder is a RunnerFactory that starts no process: it records each
// command with its working directory and environment, checks that the
// command is a read-only one, and returns the reply the test set for its
// command line.
type recorder struct {
	t       *testing.T
	mu      sync.Mutex
	calls   []recordedCall
	replies map[string]reply
}

func newRecorder(t *testing.T) *recorder {
	return &recorder{t: t, replies: map[string]reply{}}
}

// reply sets what the command line writes to its standard output when it
// succeeds.
func (r *recorder) reply(line string, out []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies[line] = reply{out: out}
}

// commands returns the Commands under test, running through the recorder
// with environ as the process environment.
func (r *recorder) commands(environ []string) Commands {
	return NewCommands(func(o cmdrunner.Options) cmdrunner.Runner {
		return recordingRunner{r: r, opts: o}
	}, environ)
}

// recorded returns a copy of the calls so far.
func (r *recorder) recorded() []recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

type recordingRunner struct {
	r    *recorder
	opts cmdrunner.Options
}

func (rr recordingRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r := rr.r
	r.t.Helper()
	call := recordedCall{name: name, args: slices.Clone(args), dir: rr.opts.Dir, env: slices.Clone(rr.opts.Env)}
	if deadline, ok := ctx.Deadline(); ok {
		call.limit = time.Until(deadline)
	}
	if name == gitProgram {
		hardening := cmdrunner.GitHardening()
		if !slices.Equal(args[:min(len(args), len(hardening))], hardening) {
			r.t.Errorf("git ran without the hardening options: %v", args)
		} else {
			call.args = slices.Clone(args[len(hardening):])
		}
	}
	if !isReadOnly(call) {
		r.t.Errorf("the tool ran a command outside the read-only list: %s", call.line())
	}
	r.mu.Lock()
	r.calls = append(r.calls, call)
	rep, ok := r.replies[call.line()]
	r.mu.Unlock()
	if !ok {
		r.t.Errorf("no reply for %q", call.line())
		return nil, errNoReply
	}
	return rep.out, rep.err
}

func isReadOnly(c recordedCall) bool {
	for _, prefix := range readOnlyCommands[c.name] {
		if len(c.args) >= len(prefix) && slices.Equal(c.args[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

// readTestdata returns a file under testdata.
func readTestdata(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	return data
}

// taskID returns the task number s, failing the test when it is malformed.
func taskID(t *testing.T, s string) paneinput.TaskID {
	t.Helper()
	task, err := paneinput.ParseTaskID(s)
	if err != nil {
		t.Fatalf("ParseTaskID(%q): %v", s, err)
	}
	return task
}
