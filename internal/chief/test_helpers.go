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
	"regexp"
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
	// and the utilities high-review-decision.sh uses), never herdr or gh.
	testToolsDir string
	// testBash is the bash found on PATH when the package started, or "".
	testBash string
)

// testTools are the programs linked into testToolsDir.
var testTools = []string{gitProgram, "env", "awk", "grep", "tr", "tail"}

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

// RealCommands returns Commands that run the real git (and no herdr or gh)
// with the repository's environment, and points PATH at the test tools for
// the rest of the test.
func (r *testRepo) RealCommands() Commands {
	r.t.Helper()
	r.t.Setenv("PATH", testToolsDir)
	return NewCommands(func(o cmdrunner.Options) cmdrunner.Runner { return cmdrunner.NewOS(o) }, r.Env())
}

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

// remoteOrigin is the one remote chief fetches from and reads.
const remoteOrigin = "origin"

// readOnlyCommands lists the commands the tool may run, by name and the words
// that select the operation. Every one reads: none pushes, merges, comments,
// or types into a pane. The recorder fails a test that runs anything else.
var readOnlyCommands = map[string][][]string{
	herdrProgram: {{herdrAgent, "list"}, {herdrAgent, "get"}, {herdrAgent, "read"}, {"workspace", "list"}, {"pane", "list"}, {"pane", "process-info"}, {"pane", "layout"}, {"tab", "list"}}, //nolint:goconst // each pair names one herdr operation
	gitProgram:   {{"show"}, {"ls-tree"}, {"rev-parse"}, {"rev-list", "--count"}, {"fetch", remoteOrigin, "main"}, {"ls-remote", remoteOrigin, "refs/heads/main"}, {"status", "--porcelain"}, {gitWorktree, "list"}, {"merge-base", "--is-ancestor"}, {"rev-list", "--parents"}, {"fetch", "--no-tags", "--no-prune", "--no-write-fetch-head", "--refmap=", remoteOrigin}},
	ghProgram:    {{"pr", "list"}, {"pr", "view"}, {"pr", "checks"}},
}

// issueNumberField is the one value `gh api graphql` may be given for the
// number: -F reads it as a number.
var issueNumberField = regexp.MustCompile(`^n=[1-9][0-9]{0,8}$`)

// isIssueRead reports whether args are the one `gh api graphql` call chief
// makes: the fixed question and the owner, repo, and number fields, and
// nothing else. `gh api graphql` can also send a mutation and take any
// method, so it is not a read-only prefix: only this exact shape passes.
func isIssueRead(args []string) bool {
	return len(args) == 10 &&
		slices.Equal(args[:7], []string{"api", "graphql", "-F", "owner={owner}", "-F", "repo={repo}", "-F"}) &&
		issueNumberField.MatchString(args[7]) &&
		args[8] == "-f" && args[9] == "query="+issueQuery
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
	// dirReplies are replies for a command line run in one directory; they
	// win over replies for the line alone.
	dirReplies map[string]reply
	// dynamic are replies computed when the line runs, for a fake whose state
	// changes; they win over the fixed replies.
	dynamic map[string]func() reply
	// onPrompt, when set, allows `herdr agent prompt` (the write of a send)
	// and answers it; a test that leaves it nil fails on it.
	onPrompt func(pane, text string) error
	// onKeys, when set, allows `herdr agent send-keys` with one key (the
	// relay's write) and answers it; a test that leaves it nil fails on it.
	onKeys func(pane, key string) error
	// writes are the writes the test allows, each for one exact command
	// line (see writeAllowance). A write no entry matches fails the test.
	writes []writeAllowance
}

// writeAllowance allows one write: the command, its working directory, and
// its arguments, all compared whole. A write whose ID or path differs from
// the one the test expects is therefore not allowed, which is what makes
// the tests fail when code closes or removes something it was not meant
// to. dir "" allows any working directory.
type writeAllowance struct {
	name string
	dir  string
	args []string
	rep  reply
	// act, when set, is run when the write runs and gives its reply, for a
	// fake whose state the write changes.
	act func() reply
}

// agentStartTimeoutMS is the --timeout the tool gives `herdr agent start`.
const agentStartTimeoutMS = "60000"

// allowWrite allows the command name with exactly args (in dir, when dir is
// not empty) and answers it with rep.
func (r *recorder) allowWrite(name, dir string, args []string, rep reply) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writes = append(r.writes, writeAllowance{name: name, dir: dir, args: slices.Clone(args), rep: rep})
}

// allowWriteDo allows the command name with exactly args (in dir, when dir
// is not empty) and answers it with what act returns when it runs.
func (r *recorder) allowWriteDo(name, dir string, args []string, act func() reply) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writes = append(r.writes, writeAllowance{name: name, dir: dir, args: slices.Clone(args), act: act})
}

// allowWorkspaceCreate allows `herdr workspace create` for exactly this
// path and label.
func (r *recorder) allowWorkspaceCreate(cwd, label string, out []byte) {
	r.allowWrite(herdrProgram, "", []string{herdrWorkspace, "create", "--cwd", cwd, "--label", label, "--no-focus"}, reply{out: out})
}

// allowTabRename allows renaming exactly this tab to label.
func (r *recorder) allowTabRename(tab, label string, out []byte) {
	r.allowWrite(herdrProgram, "", []string{herdrTab, "rename", tab, label}, reply{out: out})
}

// allowTabCreate allows creating a tab in exactly this workspace, at this
// path, with this label.
func (r *recorder) allowTabCreate(workspace, cwd, label string, out []byte) {
	r.allowWrite(herdrProgram, "", []string{herdrTab, "create", "--workspace", workspace, "--cwd", cwd, "--label", label, "--no-focus"}, reply{out: out})
}

// allowAgentStart allows starting exactly this agent in exactly this pane.
func (r *recorder) allowAgentStart(name, kind, pane string, out []byte) {
	r.allowWrite(herdrProgram, "", []string{herdrAgent, "start", name, "--kind", kind, "--pane", pane, "--timeout", agentStartTimeoutMS}, reply{out: out})
}

// allowWorkspaceClose allows closing exactly this workspace, with no
// --group.
func (r *recorder) allowWorkspaceClose(id string, out []byte) {
	r.allowWrite(herdrProgram, "", []string{herdrWorkspace, "close", id}, reply{out: out})
}

// allowWorktreeAdd allows `git worktree add --detach` of exactly this path
// at exactly this commit, run in exactly dir.
func (r *recorder) allowWorktreeAdd(dir, path, commit string) {
	r.allowWrite(gitProgram, dir, []string{gitWorktree, "add", "--detach", path, commit}, reply{})
}

// worktreeRemoveArgs are the arguments of `git worktree remove` of path, as
// the tool runs it: without --force, counting untracked files whatever the
// user's configuration says.
func worktreeRemoveArgs(path string) []string {
	return []string{"-c", "status.showUntrackedFiles=all", gitWorktree, "remove", path}
}

// allowWorktreeRemove allows `git worktree remove` of exactly this path,
// without --force, run in exactly dir.
func (r *recorder) allowWorktreeRemove(dir, path string) {
	r.allowWrite(gitProgram, dir, worktreeRemoveArgs(path), reply{})
}

// allowedWrite returns the allowance's reply when c is a write the test
// allowed.
func (r *recorder) allowedWrite(c recordedCall) (reply, bool) {
	r.mu.Lock()
	var found *writeAllowance
	for i, w := range r.writes {
		if w.name == c.name && (w.dir == "" || w.dir == c.dir) && slices.Equal(w.args, c.args) {
			found = &r.writes[i]
			break
		}
	}
	r.mu.Unlock()
	switch {
	case found == nil:
		return reply{}, false
	case found.act != nil:
		// The act runs without the lock, so it may read the recorder.
		return found.act(), true
	default:
		return found.rep, true
	}
}

func newRecorder(t *testing.T) *recorder {
	return &recorder{t: t, replies: map[string]reply{}, dirReplies: map[string]reply{}, dynamic: map[string]func() reply{}}
}

// reply sets what the command line writes to its standard output when it
// succeeds.
func (r *recorder) reply(line string, out []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies[line] = reply{out: out}
}

// replyIn sets what the command line writes when it runs in dir.
func (r *recorder) replyIn(dir, line string, out []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirReplies[dir+"\x00"+line] = reply{out: out}
}

// replyFunc sets a reply computed each time the command line runs.
func (r *recorder) replyFunc(line string, f func() reply) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dynamic[line] = f
}

// prompts returns the inputs typed into panes so far, as "pane text".
func (r *recorder) prompts() []string {
	var out []string
	for _, c := range r.recorded() {
		if isPrompt(c) {
			out = append(out, c.args[2]+" "+c.args[3])
		}
	}
	return out
}

// isPrompt reports whether c is `herdr agent prompt <pane> <text>`.
func isPrompt(c recordedCall) bool {
	return c.name == herdrProgram && len(c.args) == 4 && c.args[0] == herdrAgent && c.args[1] == "prompt"
}

// isKeys reports whether c is `herdr agent send-keys <pane> <key>`.
func isKeys(c recordedCall) bool {
	return c.name == herdrProgram && len(c.args) == 4 && c.args[0] == herdrAgent && c.args[1] == "send-keys"
}

// keys returns the keys sent to panes so far, as "pane key".
func (r *recorder) keys() []string {
	var out []string
	for _, c := range r.recorded() {
		if isKeys(c) {
			out = append(out, c.args[2]+" "+c.args[3])
		}
	}
	return out
}

// fail makes the command line fail with err, wherever it runs.
func (r *recorder) fail(line string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies[line] = reply{err: err}
}

// failIn makes the command line fail with err when it runs in dir.
func (r *recorder) failIn(dir, line string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dirReplies[dir+"\x00"+line] = reply{err: err}
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
	r.mu.Lock()
	onPrompt, onKeys := r.onPrompt, r.onKeys
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	if isPrompt(call) && onPrompt != nil {
		return nil, onPrompt(call.args[2], call.args[3])
	}
	if isKeys(call) && onKeys != nil {
		return nil, onKeys(call.args[2], call.args[3])
	}
	if rep, ok := r.allowedWrite(call); ok && !isReadOnly(call) {
		return rep.out, rep.err
	}
	if !isReadOnly(call) {
		r.t.Errorf("the tool ran a command that is neither read-only nor an allowed write: %s", call.line())
	}
	r.mu.Lock()
	dyn, isDynamic := r.dynamic[call.line()]
	rep, ok := r.dirReplies[call.dir+"\x00"+call.line()]
	if !ok {
		rep, ok = r.replies[call.line()]
	}
	r.mu.Unlock()
	if isDynamic {
		rep, ok = dyn(), true
	}
	if !ok {
		r.t.Errorf("no reply for %q", call.line())
		return nil, errNoReply
	}
	return rep.out, rep.err
}

func isReadOnly(c recordedCall) bool {
	if c.name == ghProgram && isIssueRead(c.args) {
		return true
	}
	// git grep can open files in a pager program, so only the one search the
	// tool makes passes.
	if c.name == gitProgram && slices.Equal(c.args, issueLineSearch) {
		return true
	}
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

// planSample returns a plan under testdata/plans. A name ending in
// " crlf" returns that plan with CRLF line ends, which a sample file would
// lose to the repository's whitespace hooks.
func planSample(t *testing.T, name string) []byte {
	t.Helper()
	if file, ok := strings.CutSuffix(name, " crlf"); ok {
		return bytes.ReplaceAll(readTestdata(t, "plans/"+file), []byte("\n"), []byte("\r\n"))
	}
	return readTestdata(t, "plans/"+name)
}

// planSampleLine returns line n (1-based) of the plan sample name.
func planSampleLine(t *testing.T, name string, n int) string {
	t.Helper()
	lines := strings.Split(string(planSample(t, name)), "\n")
	if n < 1 || n > len(lines) {
		t.Fatalf("%s has no line %d", name, n)
	}
	return lines[n-1]
}

// testOldValues is an old-value table for the tests that read plans, so they
// do not depend on the old values _context.md still lists.
var testOldValues = OldValues{
	"opus":                 RequirementHigh,
	"sonnet+opus-review":   RequirementStandardReview,
	"sonnet":               RequirementLow,
	"frontier-required":    RequirementHigh,
	"frontier-recommended": RequirementStandardReview,
	"standard":             RequirementLow,
}

// mustPlan returns the plan the testdata sample name holds.
func mustPlan(t *testing.T, name string) *Plan {
	t.Helper()
	return mustPlanDoc(t, planSample(t, name))
}

// mustPlanDoc parses doc, failing the test when it does not parse.
func mustPlanDoc(t *testing.T, doc []byte) *Plan {
	t.Helper()
	plan, err := ParsePlan(doc, testOldValues)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	return &plan
}

// planBeforeTheFirstPR is marker-opus.md with its first PR not started.
func planBeforeTheFirstPR(t *testing.T) *Plan {
	t.Helper()
	doc := string(planSample(t, "marker-opus.md"))
	started := "- [x] PR を作成した: https://github.com/owner/repo/pull/1\n- [x] PR がマージされた\n"
	if !strings.Contains(doc, started) {
		t.Fatal("marker-opus.md no longer holds a merged first PR")
	}
	return mustPlanDoc(t, []byte(strings.Replace(doc, started, "- [ ] PR を作成した\n- [ ] PR がマージされた\n", 1)))
}

// docsInfo returns a document of status with the process record of a
// requirements document.
func docsInfo(status DocStatus, process Process) *DocInfo {
	return &DocInfo{Status: status, Process: process}
}

// planFacts returns the facts of an approved plan.
func planFacts(t *testing.T, plan *Plan) positionFacts {
	t.Helper()
	return positionFactsOf(workspaceFacts{
		Head: headBranch, Task: taskID(t, "0001"),
		Docs: &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull), Architecture: docsInfo(DocApproved, ProcessFull), Plan: plan},
	})
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

// mustMkreq returns a running /mkreq for issue 890.
func mustMkreq(t *testing.T) *RunningCommand {
	t.Helper()
	cmd, _, err := newRunningCommand(paneinput.CommandMkreq, "890")
	if err != nil {
		t.Fatal(err)
	}
	return &cmd
}

// mustRunning returns a running command of kind with arg.
func mustRunning(t *testing.T, kind paneinput.CommandKind, arg string) *RunningCommand {
	t.Helper()
	cmd, _, err := newRunningCommand(kind, arg)
	if err != nil {
		t.Fatal(err)
	}
	return &cmd
}

// namesUpToUnknown returns name(0), name(1), ... up to the first value
// whose name is unknownName, which it leaves out, so a test can walk every
// value of a type the output names without listing them. A name function
// that names no value unknown stops at maxValueNames values instead of
// looping, and its list then repeats names, which TestOutputValueNames
// reports.
func namesUpToUnknown[T ~int](name func(T) string) []string {
	var names []string
	for v := T(0); v < maxValueNames; v++ {
		n := name(v)
		if n == unknownName {
			return names
		}
		names = append(names, n)
	}
	return names
}

// maxValueNames bounds the values namesUpToUnknown walks, far above the
// number any of the output's types declares.
const maxValueNames = 64
