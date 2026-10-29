//go:build test

// Package codexreviewtestutil turns the running test binary into a fake codex
// and keeps the real one out of reach.
//
// A test package that runs codexreview calls Main from its TestMain. Started
// under the name codex (through a symbolic link that NewFake puts on PATH),
// the test binary acts as the fake codex and never runs the tests. Started
// normally, it puts on PATH a directory that holds only git, confirms that
// codex cannot be found, and then runs the tests, so a test that forgets
// NewFake cannot reach the real codex, which would spend the Codex allowance
// and send the diff to OpenAI.
package codexreviewtestutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

const (
	// BehaviorEnv holds the fake codex's Behavior as JSON.
	BehaviorEnv = "CODEXREVIEW_FAKE_CODEX"
	// grandchildEnv makes a process started as codex the grandchild a fake
	// codex starts; its value is the file the grandchild writes its PID to.
	grandchildEnv = "CODEXREVIEW_FAKE_GRANDCHILD"
	// NoBehaviorExit is the exit status of a process started as codex with
	// no behavior to follow.
	NoBehaviorExit = 97
	// fakeName is the command name the fake answers to.
	fakeName = "codex"
	// lifetime caps how long a fake codex or its grandchild lives when
	// nothing stops it, so none outlives a broken test for long.
	lifetime = 2 * time.Minute
)

// role is what a start of the test binary does.
type role int

const (
	roleTests role = iota
	roleFakeCodex
	roleGrandchild
	roleNoBehavior
)

// roleOf decides the role from the arguments and the environment alone,
// before any flag is parsed: a process named codex never runs the tests.
func roleOf(args []string, lookup func(string) (string, bool)) role {
	if len(args) == 0 || filepath.Base(args[0]) != fakeName {
		return roleTests
	}
	if _, ok := lookup(grandchildEnv); ok {
		return roleGrandchild
	}
	if _, ok := lookup(BehaviorEnv); ok {
		return roleFakeCodex
	}
	return roleNoBehavior
}

// Main is what a TestMain calls before anything else; it returns the exit
// status for os.Exit.
func Main(m *testing.M) int {
	switch roleOf(os.Args, os.LookupEnv) {
	case roleFakeCodex:
		return runFake(os.Args[1:])
	case roleGrandchild:
		return runGrandchild(os.Getenv(grandchildEnv))
	case roleNoBehavior:
		return NoBehaviorExit
	}
	dir, err := isolatePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "codexreviewtestutil: %v\n", err)
		return 1
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	return code
}

// isolatePath points PATH at a new directory holding only git and confirms
// that codex is not found. It returns the directory.
func isolatePath() (string, error) {
	dir, err := os.MkdirTemp("", "codexreview-path-")
	if err != nil {
		return "", err
	}
	if err := linkGit(dir); err != nil {
		return dir, err
	}
	if err := os.Setenv("PATH", dir); err != nil {
		return dir, err
	}
	if path, err := exec.LookPath(fakeName); err == nil {
		return dir, fmt.Errorf("%w at %s", errCodexOnPath, path)
	}
	return dir, nil
}

// errCodexOnPath reports that codex is still found after PATH was replaced.
var errCodexOnPath = errors.New("codex is still found on PATH")

// realGit is the git executable found when the package started, with its
// symbolic links resolved.
var realGit string

// linkGit puts in dir a symbolic link named git to the real git executable.
func linkGit(dir string) error {
	if realGit == "" {
		path, err := exec.LookPath("git")
		if err != nil {
			return err
		}
		if realGit, err = filepath.EvalSymlinks(path); err != nil {
			return err
		}
	}
	return os.Symlink(realGit, filepath.Join(dir, "git"))
}

// Behavior is what a fake codex does. The actions run in the order of the
// fields: record the start, act on the repository, start the grandchild,
// write the output, then hang or exit.
type Behavior struct {
	// StartsFile receives one line per start: "version" or "review", and
	// ArgsFile the arguments of a review start as a JSON array. Set fills
	// both in, so Fake.Starts and Fake.Args read what was recorded.
	StartsFile string `json:"starts_file"`
	ArgsFile   string `json:"args_file"`

	// Version is printed by codex --version, which exits with VersionExit or
	// hangs when VersionHang is set.
	Version     string `json:"version"`
	VersionExit int    `json:"version_exit"`
	VersionHang bool   `json:"version_hang"`
	// VersionWrites maps an absolute path to the content codex --version
	// writes there before it answers.
	VersionWrites map[string]string `json:"version_writes,omitempty"`

	// Writes maps an absolute path to the content written there.
	Writes map[string]string `json:"writes,omitempty"`
	// Removes lists absolute paths removed.
	Removes []string `json:"removes,omitempty"`
	// Git lists git commands run in GitDir.
	Git    [][]string `json:"git,omitempty"`
	GitDir string     `json:"git_dir,omitempty"`

	// GrandchildPIDFile, when set, makes the fake start a grandchild that
	// writes its PID there and lives until it is killed. The fake goes on
	// only once the PID is written, so a test can always find it.
	GrandchildPIDFile string `json:"grandchild_pid_file,omitempty"`

	// Events are written to standard output, one per line, followed by
	// StdoutPadding bytes; StderrPadding bytes go to standard error.
	Events        []string `json:"events,omitempty"`
	StdoutPadding int      `json:"stdout_padding,omitempty"`
	StderrPadding int      `json:"stderr_padding,omitempty"`
	// LastMessage, when set, is written to the file named by -o.
	LastMessage *string `json:"last_message,omitempty"`
	// LastMessageLink, when set, makes the path named by -o a symbolic link
	// to this file instead.
	LastMessageLink string `json:"last_message_link,omitempty"`

	// Hang makes the fake wait until it is killed; otherwise it exits with Exit.
	Hang bool `json:"hang,omitempty"`
	Exit int  `json:"exit,omitempty"`
}

// runFake acts as codex, following the Behavior in the environment.
func runFake(args []string) int {
	var b Behavior
	if err := json.Unmarshal([]byte(os.Getenv(BehaviorEnv)), &b); err != nil {
		fmt.Fprintf(os.Stderr, "fake codex: behavior: %v\n", err)
		return NoBehaviorExit
	}
	if slices.Equal(args, []string{"--version"}) {
		return b.version()
	}
	if err := b.review(args); err != nil {
		fmt.Fprintf(os.Stderr, "fake codex: %v\n", err)
		return NoBehaviorExit
	}
	if b.Hang {
		time.Sleep(lifetime)
	}
	return b.Exit
}

func (b Behavior) version() int {
	if err := appendLine(b.StartsFile, "version"); err != nil {
		return NoBehaviorExit
	}
	for path, content := range b.VersionWrites {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // test-only fake writes the file its test named
			return NoBehaviorExit
		}
	}
	if b.VersionHang {
		time.Sleep(lifetime)
	}
	fmt.Println(b.Version)
	return b.VersionExit
}

func (b Behavior) review(args []string) error {
	if err := appendLine(b.StartsFile, "review"); err != nil {
		return err
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := os.WriteFile(b.ArgsFile, encoded, 0o600); err != nil { //nolint:gosec // test-only fake writes the file its test named
		return err
	}
	for path, content := range b.Writes {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}
	for _, path := range b.Removes {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	for _, gitArgs := range b.Git {
		cmd := exec.Command("git", gitArgs...) //nolint:gosec // test-only fake runs the git commands its test scripted
		cmd.Dir = b.GitDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", gitArgs, err, out)
		}
	}
	if b.GrandchildPIDFile != "" {
		// Started under this process's own name, so it answers as the
		// grandchild and never runs the tests.
		grandchild := exec.Command(os.Args[0]) //nolint:gosec // test-only fake re-executes itself
		grandchild.Env = append(os.Environ(), grandchildEnv+"="+b.GrandchildPIDFile)
		if err := grandchild.Start(); err != nil {
			return err
		}
		if !waitForFile(b.GrandchildPIDFile) {
			return errNoGrandchildPID
		}
	}
	for _, event := range b.Events {
		fmt.Println(event)
	}
	if err := pad(os.Stdout, b.StdoutPadding); err != nil {
		return err
	}
	if err := pad(os.Stderr, b.StderrPadding); err != nil {
		return err
	}
	if b.LastMessageLink != "" {
		if err := os.Symlink(b.LastMessageLink, argAfter(args, "-o")); err != nil {
			return err
		}
	}
	if b.LastMessage != nil {
		if err := os.WriteFile(argAfter(args, "-o"), []byte(*b.LastMessage), 0o600); err != nil { //nolint:gosec // test-only fake writes where codexreview told it to
			return err
		}
	}
	return nil
}

// errNoGrandchildPID reports a grandchild that did not write its PID in time.
var errNoGrandchildPID = errors.New("the grandchild wrote no PID")

// waitForFile reports whether path exists and is not empty within
// processGoneBound.
func waitForFile(path string) bool {
	deadline := time.Now().Add(processGoneBound)
	for {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

// runGrandchild writes the process's PID to pidFile and waits to be killed.
func runGrandchild(pidFile string) int {
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil { //nolint:gosec // test-only grandchild writes the file its test named
		return NoBehaviorExit
	}
	time.Sleep(lifetime)
	return 0
}

// pad writes n bytes to f.
func pad(f *os.File, n int) error {
	_, err := f.Write(bytes.Repeat([]byte("x"), n))
	return err
}

// argAfter returns the argument after flag, or "" when there is none.
func argAfter(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

// appendLine appends line to path.
func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // test-only fake writes the file its test named
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Fake is the fake codex of one test. Its files live outside any worktree.
type Fake struct {
	t *testing.T
	// Dir holds the fake's files; tests may put theirs there too.
	Dir string
	// BinDir is the only directory on PATH: it holds codex and git.
	BinDir string
}

// NewFake puts a fake codex and git on PATH and points HOME,
// XDG_CONFIG_HOME, and TMPDIR at new empty directories, so the user's git
// configuration cannot reach the test. Until Set is called, a start of codex
// exits with NoBehaviorExit. Tests that use it cannot run in parallel.
func NewFake(t *testing.T) *Fake {
	t.Helper()
	dir := resolvedTempDir(t)
	f := &Fake{t: t, Dir: dir, BinDir: filepath.Join(dir, "bin")}
	home := filepath.Join(dir, "home")
	tmp := filepath.Join(dir, "tmp")
	for _, d := range []string{f.BinDir, home, tmp} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatalf("create %s: %v", d, err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve the test binary: %v", err)
	}
	if err := os.Symlink(executable, filepath.Join(f.BinDir, fakeName)); err != nil {
		t.Fatalf("link codex: %v", err)
	}
	if err := linkGit(f.BinDir); err != nil {
		t.Fatalf("link git: %v", err)
	}
	t.Setenv("PATH", f.BinDir)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("TMPDIR", tmp)
	// A race-enabled binary sleeps a second at exit by default; every start
	// of the fake would pay it.
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	return f
}

// Set makes b the behavior of every later start of codex, with the starts
// and arguments files that Starts and Args read.
func (f *Fake) Set(b Behavior) {
	f.t.Helper()
	b.StartsFile = filepath.Join(f.Dir, "starts")
	b.ArgsFile = filepath.Join(f.Dir, "args.json")
	encoded, err := json.Marshal(b)
	if err != nil {
		f.t.Fatalf("encode behavior: %v", err)
	}
	f.t.Setenv(BehaviorEnv, string(encoded))
}

// Starts returns the recorded starts of codex in order: "version" or "review".
func (f *Fake) Starts() []string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.Dir, "starts"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		f.t.Fatalf("read starts: %v", err)
	}
	return strings.Fields(string(data))
}

// Args returns the arguments of the last review start.
func (f *Fake) Args() []string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.Dir, "args.json"))
	if err != nil {
		f.t.Fatalf("read args: %v", err)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		f.t.Fatalf("decode args: %v", err)
	}
	return args
}

// Repo is a temporary git repository on branch main with one commit.
type Repo struct {
	t *testing.T
	// Dir is the worktree root, with its symbolic links resolved.
	Dir string
}

// NewRepo creates the repository. Call it after NewFake, so its git runs
// with the empty HOME. The repository configuration turns commit and tag
// signing off and sets an identity.
func NewRepo(t *testing.T) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: filepath.Join(resolvedTempDir(t), "repo")}
	r.gitIn(filepath.Dir(r.Dir), "init", "-q", "--template=", "-b", "main", r.Dir)
	r.Git("config", "user.name", "Test")
	r.Git("config", "user.email", "test@example.com")
	r.Git("config", "commit.gpgSign", "false")
	r.Git("config", "tag.gpgSign", "false")
	r.Write("README.md", "initial\n")
	r.CommitAll("initial")
	return r
}

// Git runs git in the worktree for the test's own setup and returns its
// standard output.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.Dir, args...)
}

func (r *Repo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // test setup runs git with the test's arguments
	cmd.Dir = dir
	cmd.Env = append(cmdrunner.GitEnv(os.Environ()), "GIT_CONFIG_NOSYSTEM=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git %v: %v: %s", args, err, stderr.String())
	}
	return string(out)
}

// Write writes content to the path relative to the worktree root, creating
// its directories.
func (r *Repo) Write(rel, content string) {
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
func (r *Repo) CommitAll(message string) string {
	r.t.Helper()
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", message)
	return r.Head()
}

// Head returns the full commit ID of HEAD.
func (r *Repo) Head() string {
	r.t.Helper()
	return strings.TrimSpace(r.Git("rev-parse", "HEAD"))
}

// resolvedTempDir returns t.TempDir with its symbolic links resolved.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	return dir
}

const (
	// processGoneBound bounds how long ReadPID and WaitGone wait.
	processGoneBound = 10 * time.Second
	// pollInterval is how often they look again.
	pollInterval = 20 * time.Millisecond
)

// ReadPID waits for path to hold a PID, as a grandchild writes it, and
// returns it. It fails the test after processGoneBound.
func ReadPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(processGoneBound)
	for {
		if data, err := os.ReadFile(path); err == nil { //nolint:gosec // the test names the file
			if pid, err := strconv.Atoi(string(data)); err == nil && pid > 0 {
				// Kill it when the test ends, whatever the test checks.
				t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no PID in %s", path)
		}
		time.Sleep(pollInterval)
	}
}

// WaitGone reports whether pid is gone within processGoneBound. A zombie
// counts as gone.
func WaitGone(pid int) bool {
	deadline := time.Now().Add(processGoneBound)
	for {
		if !processExists(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

// processExists reports whether a live process with pid exists. A zombie
// counts as gone: kill(pid, 0) succeeds for it, and a container whose PID 1
// does not reap orphans would otherwise leave every killed process alive.
func processExists(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	return processState(pid) != 'Z'
}

// processState returns the one-letter state of pid, or 0 when it cannot be read.
func processState(pid int) byte {
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		// The command name may contain spaces and parentheses, so the state is
		// the first field after the last closing parenthesis.
		if i := bytes.LastIndex(data, []byte(") ")); i >= 0 && i+2 < len(data) {
			return data[i+2]
		}
		return 0
	}
	// By absolute path: PATH holds only codex and git during the tests.
	out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // fixed arguments, pid is an integer
	if err != nil {
		return 0
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return s[0]
	}
	return 0
}
