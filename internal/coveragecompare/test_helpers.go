//go:build test

package coveragecompare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	// testModulePath is the module path the parser tests give parseGoProfile.
	testModulePath = "github.com/isseis/yt2column"
	// testExtensionDir is the checkout extension directory the TAP parser
	// tests give parseTAP.
	testExtensionDir = "/checkout/extension"
)

// requireStop fails the test unless err is an unreadable StopError at point and
// input, and returns its first reason.
func requireStop(t *testing.T, err error, point Point, input Input) StopReason {
	t.Helper()
	stop, ok := errors.AsType[*StopError](err)
	if !ok {
		t.Fatalf("error = %v, want *StopError", err)
	}
	if stop.Kind != StopUnreadable {
		t.Fatalf("stop kind = %v, want %v (reasons: %+v)", stop.Kind, StopUnreadable, stop.Reasons)
	}
	if len(stop.Reasons) == 0 {
		t.Fatal("stop has no reasons")
	}
	reason := stop.Reasons[0]
	if reason.Point != point {
		t.Errorf("reason point = %v, want %v", reason.Point, point)
	}
	if reason.Input != input {
		t.Errorf("reason input = %v, want %v", reason.Input, input)
	}
	return reason
}

// fakeSource is a source over in-memory files. ranges holds the range
// helper's answer for each file, for fakeRanges.
type fakeSource struct {
	at      Point
	files   map[string]string
	ranges  map[string]fileRanges
	readErr error
}

func (s *fakeSource) point() Point { return s.at }

func (s *fakeSource) readFile(path string) ([]byte, bool, error) {
	if s.readErr != nil {
		return nil, false, s.readErr
	}
	data, ok := s.files[path]
	if !ok {
		return nil, false, nil
	}
	return []byte(data), true, nil
}

// fakeRanges answers the range helper's call from the ranges of a fakeSource.
type fakeRanges struct {
	err error
}

func (r fakeRanges) functionRanges(_ context.Context, src source, files []string) ([]fileRanges, error) {
	if r.err != nil {
		return nil, r.err
	}
	fake, ok := src.(*fakeSource)
	if !ok {
		return nil, errFakeSourceType
	}
	answer := make([]fileRanges, 0, len(files))
	for _, file := range files {
		entry, ok := fake.ranges[file]
		if !ok {
			continue
		}
		entry.Path = file
		answer = append(answer, entry)
	}
	return answer, nil
}

// errFakeSourceType is returned when fakeRanges is given a source it did not
// expect.
var errFakeSourceType = errors.New("fakeRanges needs a *fakeSource")

// The fake go, node, and npm are the test binary itself, started through a
// symbolic link of that name. fakeMain decides from the name alone, before
// any flag is parsed, so a process started as go, node, or npm never runs the
// tests. Each test gives its fakes their behavior in a file under the HOME of
// the environment it passes to Run, which the tool passes on to every go,
// node, and npm it starts; PATH is never changed per test, so the tests can
// run in parallel.
const (
	// fakeBehaviorName is the behavior file under HOME.
	fakeBehaviorName = "fake-behavior.json"
	// fakeNoBehaviorExit is the exit status of a fake start with no behavior
	// to follow, or one it was not told how to answer.
	fakeNoBehaviorExit = 97
	// fakeGrandchildFlag, as the only argument but one, makes a start the
	// grandchild a fake suite starts in its process group.
	fakeGrandchildFlag = "--fake-grandchild"
	// fakeLifetime caps how long a fake waits when nothing stops it.
	fakeLifetime = 2 * time.Minute
	// fakeLargeLine is the length of each line of a large -json output: over
	// 1 MiB, past bufio.Scanner's default limit and any other small cap.
	fakeLargeLine = 1<<20 + 1024
	// fakeLargeLines lines of fakeLargeLine make over 20 MiB.
	fakeLargeLines = 21
)

// The commands the test binary plays.
const (
	fakeGo   = "go"
	fakeNode = "node"
	fakeNpm  = "npm"
)

// fakeNames are the commands the test binary plays.
var fakeNames = []string{fakeGo, fakeNode, fakeNpm}

// gitNames are the real git programs fakeBinDir links to: git, and the
// upload-pack a local clone runs through the shell. Inside a git hook, git
// puts its own exec directory first on PATH, and the git found there may not
// find its upload-pack from a PATH that holds only fakeBinDir.
var gitNames = []string{"git", "git-upload-pack"}

// fakeBinDir is the only directory on the test process's PATH: it holds the
// fakes and links to the real git programs.
var fakeBinDir string

// fakeMain is what TestMain runs. A start as a fake exits here through
// syscall.Exit, so a race-enabled binary skips the second it otherwise sleeps
// at exit. A normal start replaces PATH with fakeBinDir and confirms that go,
// node, and npm resolve to the fakes before any test runs.
func fakeMain(m *testing.M) int {
	if name := filepath.Base(os.Args[0]); slices.Contains(fakeNames, name) {
		syscall.Exit(runFake(name, os.Args[1:]))
	}
	dir, err := isolatePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "coveragecompare tests: %v\n", err)
		return 1
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	return code
}

// isolatePath creates fakeBinDir, points PATH at it, and checks that each
// fake name resolves into it.
func isolatePath() (string, error) {
	dir, err := os.MkdirTemp("", "coveragecompare-path-")
	if err != nil {
		return "", err
	}
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return dir, err
	}
	for _, name := range fakeNames {
		if err := os.Symlink(executable, filepath.Join(dir, name)); err != nil {
			return dir, err
		}
	}
	for _, name := range gitNames {
		path, err := exec.LookPath(name)
		if err != nil {
			return dir, err
		}
		if path, err = filepath.EvalSymlinks(path); err != nil {
			return dir, err
		}
		if err := os.Symlink(path, filepath.Join(dir, name)); err != nil {
			return dir, err
		}
	}
	if err := os.Setenv("PATH", dir); err != nil {
		return dir, err
	}
	fakeBinDir = dir
	for _, name := range fakeNames {
		if path, err := exec.LookPath(name); err != nil || path != filepath.Join(dir, name) {
			return dir, fmt.Errorf("%w: %s resolves to %q (%v)", errRealToolchain, name, path, err)
		}
	}
	return dir, nil
}

// errRealToolchain reports a fake name that does not resolve to the fake.
var errRealToolchain = errors.New("a toolchain command does not resolve to its fake")

// fakeBehavior is what the fakes of one test do.
type fakeBehavior struct {
	// Record receives one JSON line per start; fakeEnv.set fills it in.
	Record      string `json:"record"`
	GoVersion   string `json:"go_version"`
	NodeVersion string `json:"node_version"`
	NpmVersion  string `json:"npm_version"`
	// Suites are the test suite runs by point ("base" or "head").
	Suites map[string]fakeSuite `json:"suites"`
	// Ranges are the range helper's answers by point.
	Ranges map[string]fakeAnswer `json:"ranges"`
}

// fakeSuite is one point's test suite run. Its actions run in the order of
// the fields: run Git in GitDir, write Writes, write the reports, write the
// standard output, then wait or exit.
type fakeSuite struct {
	Git    [][]string        `json:"git,omitempty"`
	GitDir string            `json:"git_dir,omitempty"`
	Writes map[string]string `json:"writes,omitempty"`
	// Report is the Go profile or the lcov report, written where the
	// command line says; TAP is the extension's TAP report.
	Report string `json:"report"`
	TAP    string `json:"tap"`
	// Stdout is written to standard output; with LargeStdout it follows
	// fakeLargeLines lines of fakeLargeLine bytes each.
	Stdout      string `json:"stdout"`
	LargeStdout bool   `json:"large_stdout,omitempty"`
	// Ready, when set, makes the suite start a grandchild, write its own PID
	// and the grandchild's there, and wait for SIGTERM, after which it exits
	// with 1. The grandchild stays in the suite's process group and lives
	// until it is killed.
	Ready string `json:"ready,omitempty"`
	Exit  int    `json:"exit"`
}

// fakeAnswer is a command's answer: its standard output and error and its
// exit status.
type fakeAnswer struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Exit   int    `json:"exit"`
}

// fakeStart is the record of one start of a fake.
type fakeStart struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Dir  string   `json:"dir"`
	// NodeModules is the shape of node_modules in the working directory at
	// the start, when it has one: the checkouts are gone after Run returns.
	NodeModules *fakeNodeModules `json:"node_modules,omitempty"`
}

// fakeNodeModules is a node_modules directory: whether it is a real
// directory, and the target of each entry that is a symbolic link.
type fakeNodeModules struct {
	RealDir bool              `json:"real_dir"`
	Links   map[string]string `json:"links"`
}

// suite reports whether the start ran a test suite.
func (s fakeStart) suite() bool {
	return len(s.Args) > 0 && ((s.Name == fakeGo && s.Args[0] == "test") || (s.Name == fakeNode && s.Args[0] == "--test"))
}

// runFake plays the command name.
func runFake(name string, args []string) int {
	if len(args) == 1 && args[0] == fakeGrandchildFlag {
		time.Sleep(fakeLifetime)
		return 0
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), fakeBehaviorName)) //nolint:gosec // G304: test-only fake reads the behavior its test wrote
	if err != nil {
		return fakeNoBehaviorExit
	}
	var b fakeBehavior
	if err := json.Unmarshal(data, &b); err != nil {
		return fakeNoBehaviorExit
	}
	if err := b.record(name, args); err != nil {
		return fakeNoBehaviorExit
	}
	if version, ok := b.version(name, args); ok {
		return answer(fakeAnswer{Stdout: version + "\n"})
	}
	switch {
	case name == fakeGo && len(args) > 0 && args[0] == "test":
		return b.suite(args, "-coverprofile=", ".cover")
	case name == fakeNode && len(args) > 0 && args[0] == "--test":
		return b.suite(args, "--test-reporter-destination=", ".lcov")
	case name == fakeNode && len(args) > 0 && filepath.Base(args[0]) == "function-ranges.ts":
		root := argAfter(args, "--root")
		return answer(b.Ranges[filepath.Base(filepath.Dir(root))])
	default:
		return fakeNoBehaviorExit
	}
}

// version returns the version a start asks for: go env GOVERSION,
// node --version, or npm --version.
func (b fakeBehavior) version(name string, args []string) (string, bool) {
	switch {
	case name == fakeGo && slices.Equal(args, []string{"env", "GOVERSION"}):
		return b.GoVersion, true
	case name == fakeNode && slices.Equal(args, []string{"--version"}):
		return b.NodeVersion, true
	case name == fakeNpm && slices.Equal(args, []string{"--version"}):
		return b.NpmVersion, true
	default:
		return "", false
	}
}

// record appends the start to the record file.
func (b fakeBehavior) record(name string, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	start := fakeStart{Name: name, Args: args, Env: os.Environ(), Dir: dir}
	modules := filepath.Join(dir, "node_modules")
	if info, err := os.Lstat(modules); err == nil {
		start.NodeModules = &fakeNodeModules{RealDir: info.IsDir(), Links: map[string]string{}}
		entries, err := os.ReadDir(modules)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if target, err := os.Readlink(filepath.Join(modules, entry.Name())); err == nil {
				start.NodeModules.Links[entry.Name()] = target
			}
		}
	}
	line, err := json.Marshal(start)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(b.Record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // G304: test-only fake appends to the record its test named
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// suite plays a test suite run. The point is the base name of the report
// destination: the argument with prefix whose value ends with ext.
func (b fakeBehavior) suite(args []string, prefix, ext string) int {
	var report, tap string
	for _, arg := range args {
		if value, ok := strings.CutPrefix(arg, prefix); ok {
			switch filepath.Ext(value) {
			case ext:
				report = value
			case ".tap":
				tap = value
			}
		}
	}
	s, ok := b.Suites[strings.TrimSuffix(filepath.Base(report), ext)]
	if report == "" || !ok {
		return fakeNoBehaviorExit
	}
	if err := s.act(report, tap); err != nil {
		fmt.Fprintf(os.Stderr, "fake suite: %v\n", err)
		return fakeNoBehaviorExit
	}
	if s.Ready != "" {
		return waitForTerm(s.Ready)
	}
	return s.Exit
}

// act runs the suite's actions up to its output.
func (s fakeSuite) act(report, tap string) error {
	for _, gitArgs := range s.Git {
		cmd := exec.Command("git", gitArgs...) //nolint:gosec // G204: test-only fake runs the git commands its test scripted
		cmd.Dir = s.GitDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", gitArgs, err, out)
		}
	}
	for path, content := range s.Writes {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // G703: test-only fake writes the file its test named
			return err
		}
	}
	if err := os.WriteFile(report, []byte(s.Report), 0o600); err != nil { //nolint:gosec // G703: test-only fake writes the report where the tool told it to
		return err
	}
	if tap != "" {
		if err := os.WriteFile(tap, []byte(s.TAP), 0o600); err != nil { //nolint:gosec // G703: test-only fake writes the report where the tool told it to
			return err
		}
	}
	if s.LargeStdout {
		line := `{"Action":"output","Package":"example.com/large","Output":"` + strings.Repeat("x", fakeLargeLine) + `\n"}` + "\n"
		for range fakeLargeLines {
			if _, err := os.Stdout.WriteString(line); err != nil {
				return err
			}
		}
	}
	_, err := os.Stdout.WriteString(s.Stdout)
	return err
}

// waitForTerm starts a grandchild in the process group, writes both PIDs to
// ready, and waits for SIGTERM.
func waitForTerm(ready string) int {
	terms := make(chan os.Signal, 1)
	signal.Notify(terms, syscall.SIGTERM)
	// Found on the fake's PATH, which is fakeBinDir: the grandchild is the
	// fake go, in the suite's process group.
	grandchild := exec.Command(fakeGo, fakeGrandchildFlag)
	if err := grandchild.Start(); err != nil {
		return fakeNoBehaviorExit
	}
	pids := strconv.Itoa(os.Getpid()) + " " + strconv.Itoa(grandchild.Process.Pid)
	if err := os.WriteFile(ready, []byte(pids), 0o600); err != nil { //nolint:gosec // G703: test-only fake writes the file its test named
		return fakeNoBehaviorExit
	}
	select {
	case <-terms:
		return 1
	case <-time.After(fakeLifetime):
		return 0
	}
}

// answer writes a fake answer and returns its exit status.
func answer(a fakeAnswer) int {
	_, _ = os.Stdout.WriteString(a.Stdout)
	_, _ = os.Stderr.WriteString(a.Stderr)
	return a.Exit
}

// argAfter returns the argument after flag, or "".
func argAfter(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

// fakeEnv is the environment one test passes to Run: PATH is fakeBinDir, and
// HOME and TMPDIR are the test's own empty directories, so the test can check
// that the run left TMPDIR empty.
type fakeEnv struct {
	t       *testing.T
	home    string
	tmp     string
	record  string
	environ []string
}

// newFakeEnv returns a test's environment, with no behavior set yet.
func newFakeEnv(t *testing.T) *fakeEnv {
	t.Helper()
	f := &fakeEnv{t: t, home: resolvedTempDir(t), tmp: resolvedTempDir(t)}
	f.record = filepath.Join(resolvedTempDir(t), "starts.jsonl")
	f.environ = []string{"PATH=" + fakeBinDir, "HOME=" + f.home, "TMPDIR=" + f.tmp}
	return f
}

// set makes b the behavior of every later fake start.
func (f *fakeEnv) set(b fakeBehavior) {
	f.t.Helper()
	b.Record = f.record
	data, err := json.Marshal(b)
	if err != nil {
		f.t.Fatalf("encode the behavior: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.home, fakeBehaviorName), data, 0o600); err != nil {
		f.t.Fatalf("write the behavior: %v", err)
	}
}

// starts returns the recorded fake starts in order.
func (f *fakeEnv) starts() []fakeStart {
	f.t.Helper()
	data, err := os.ReadFile(f.record)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		f.t.Fatalf("read the starts: %v", err)
	}
	var starts []fakeStart
	for _, line := range splitLines(string(data)) {
		var start fakeStart
		if err := json.Unmarshal([]byte(line), &start); err != nil {
			f.t.Fatalf("decode a start: %v", err)
		}
		starts = append(starts, start)
	}
	return starts
}

// requireTmpEmpty fails the test unless the run left TMPDIR empty.
func (f *fakeEnv) requireTmpEmpty() {
	f.t.Helper()
	entries, err := os.ReadDir(f.tmp)
	if err != nil {
		f.t.Fatalf("read TMPDIR: %v", err)
	}
	if len(entries) > 0 {
		f.t.Errorf("TMPDIR holds %d entries after the run, want none (first %q)", len(entries), entries[0].Name())
	}
}

// testRepo is a temporary git repository on branch main.
type testRepo struct {
	t   *testing.T
	dir string // the worktree root, with its symbolic links resolved
}

// newTestRepo creates an empty repository. Its git never reads the user's
// global or system configuration, and commits are not signed.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	r := &testRepo{t: t, dir: filepath.Join(resolvedTempDir(t), "repo")}
	r.gitIn(filepath.Dir(r.dir), "init", "-q", "--template=", "-b", "main", r.dir)
	r.git("config", "user.name", "Test")
	r.git("config", "user.email", "test@example.com")
	r.git("config", "commit.gpgSign", "false")
	r.git("config", "tag.gpgSign", "false")
	return r
}

// git runs git in the worktree for the test's setup and returns its output.
func (r *testRepo) git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.dir, args...)
}

// gitInput runs git in the worktree with stdin as its standard input.
func (r *testRepo) gitInput(stdin string, args ...string) string {
	r.t.Helper()
	return r.gitRun(r.dir, stdin, args...)
}

func (r *testRepo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	return r.gitRun(dir, "", args...)
}

func (r *testRepo) gitRun(dir, stdin string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // G204: test setup runs git with the test's arguments
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = gitEnv(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		r.t.Fatalf("git %v: %v: %s", args, err, stderr.String())
	}
	return string(out)
}

// write writes content to the path relative to the worktree root.
func (r *testRepo) write(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		r.t.Fatalf("create the directory of %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		r.t.Fatalf("write %s: %v", rel, err)
	}
}

// writeGitDir writes content to the path relative to the repository's .git
// directory, creating its directories.
func (r *testRepo) writeGitDir(rel, content string) {
	r.t.Helper()
	r.write(filepath.Join(".git", rel), content)
}

// commitAll commits every change and returns the new HEAD.
func (r *testRepo) commitAll(message string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "--allow-empty", "-m", message)
	return r.rev("HEAD")
}

// rev returns the full ID rev names.
func (r *testRepo) rev(rev string) string {
	r.t.Helper()
	return strings.TrimSpace(r.git("rev-parse", "--verify", rev))
}

// resolvedTempDir returns t.TempDir with its symbolic links resolved.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve a temporary directory: %v", err)
	}
	return dir
}
