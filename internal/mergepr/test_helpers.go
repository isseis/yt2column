//go:build test

package mergepr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/numreserve"
)

const (
	testPRNumber = 42
	testHeadOID  = "2222222222222222222222222222222222222222"
	testOtherOID = "4444444444444444444444444444444444444444"
	testMergeOID = "3333333333333333333333333333333333333333"
)

var testState = State{
	Number:      testPRNumber,
	HeadRefName: "feature/foo",
	HeadRefOID:  testHeadOID,
	BaseRefName: "main",
	Title:       "Test PR",
	URL:         "https://github.com/isseis/yt2column/pull/42",
}

// commandStep scripts one expected command and its result.
type commandStep struct {
	name string
	args []string
	out  string
	err  error
	// hook, when set, runs inside the call with the arguments as given, so a
	// test can look at a file that exists only while the command runs.
	hook func(args []string)
}

// anyBodyFile stands in a step's arguments for the value after --body-file,
// a path the tool makes at random.
const anyBodyFile = "<any body file>"

// maskBodyFile returns args with the value after --body-file replaced by
// anyBodyFile.
func maskBodyFile(args []string) []string {
	masked := slices.Clone(args)
	for i := range len(masked) - 1 {
		if masked[i] == "--body-file" {
			masked[i+1] = anyBodyFile
		}
	}
	return masked
}

func gitStep(out string, args ...string) commandStep {
	return commandStep{name: gitCommand, args: args, out: out}
}

func ghStep(out string, args ...string) commandStep {
	return commandStep{name: ghCommand, args: args, out: out}
}

// fakeRunner replays a fixed command sequence and fails the test on the first
// mismatch or extra call, so a test cannot pass by skipping a step.
type fakeRunner struct {
	t     testing.TB
	steps []commandStep
	index int
}

// Run implements Runner.
func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if f.index >= len(f.steps) {
		f.t.Fatalf("command %d: unexpected call %s %q", f.index+1, name, args)
	}
	step := f.steps[f.index]
	f.index++
	compared := args
	if slices.Contains(step.args, anyBodyFile) {
		compared = maskBodyFile(args)
	}
	if name != step.name || !slices.Equal(compared, step.args) {
		f.t.Fatalf("command %d = %s %q, want %s %q", f.index, name, args, step.name, step.args)
	}
	if step.hook != nil {
		step.hook(args)
	}
	return []byte(step.out), step.err
}

// done asserts that every scripted command ran.
func (f *fakeRunner) done() {
	f.t.Helper()
	if f.index != len(f.steps) {
		f.t.Errorf("ran %d commands, want %d", f.index, len(f.steps))
	}
}

// checkCall records one call of the protocol-number check: the head and base
// it was given and how many scripted commands had run before it.
type checkCall struct {
	head, base string
	index      int
}

// fakeChecker stands in for the protocol-number check. It returns the result
// it was given and records each call with the fake runner's position, so a
// test can tell which commands ran before the check.
type fakeChecker struct {
	runner  *fakeRunner
	summary protocolSummary
	err     error
	calls   []checkCall
}

func (f *fakeChecker) check(_ context.Context, headOID, baseRefName string) (protocolSummary, error) {
	f.calls = append(f.calls, checkCall{head: headOID, base: baseRefName, index: f.runner.index})
	return f.summary, f.err
}

// testSummary is the summary the passing fake check returns.
var testSummary = protocolSummary{
	MainOID:   testOtherOID,
	HeadOID:   testHeadOID,
	MainTools: map[string]protocolValue{"alpha": {State: protocolPresent, Value: 1}},
}

// numberCall records one call of the number check: the head and branch it
// was given and how many scripted commands had run before it.
type numberCall struct {
	head, branch string
	index        int
}

// fakeNumbers stands in for the number check. It returns the findings and
// error it was given and records each call with the fake runner's position,
// so a test can tell which commands ran before the check.
type fakeNumbers struct {
	runner *fakeRunner
	issues []numreserve.AddedNumberIssue
	err    error
	calls  []numberCall
}

func (f *fakeNumbers) CheckAdded(_ context.Context, head, headBranch string) ([]numreserve.AddedNumberIssue, error) {
	f.calls = append(f.calls, numberCall{head: head, branch: headBranch, index: f.runner.index})
	return f.issues, f.err
}

// newTool returns a Tool on a scripted runner whose protocol-number check
// passes with testSummary and whose number check finds nothing.
func newTool(t testing.TB, steps ...commandStep) (*Tool, *fakeRunner) {
	t.Helper()
	tool, runner, _ := newCheckedTool(t, nil, steps...)
	return tool, runner
}

// newCheckedTool returns a Tool on a scripted runner whose protocol-number
// check returns checkErr, or passes with testSummary when checkErr is nil.
func newCheckedTool(t testing.TB, checkErr error, steps ...commandStep) (*Tool, *fakeRunner, *fakeChecker) {
	t.Helper()
	runner := &fakeRunner{t: t, steps: steps}
	checker := &fakeChecker{runner: runner, summary: testSummary, err: checkErr}
	if checkErr != nil {
		checker.summary = protocolSummary{}
	}
	tool := New(runner, &fakeNumbers{runner: runner})
	tool.mainFetch = runner
	tool.protocol = checker
	source := newFakeSource(t)
	source.runner = runner
	tool.corrections = source
	return tool, runner, checker
}

// newNumberedTool returns a Tool on a scripted runner whose protocol-number
// check passes and whose number check returns issues and err.
func newNumberedTool(t testing.TB, issues []numreserve.AddedNumberIssue, err error, steps ...commandStep) (*Tool, *fakeRunner, *fakeNumbers) {
	t.Helper()
	tool, runner, _ := newCheckedTool(t, nil, steps...)
	numbers := &fakeNumbers{runner: runner, issues: issues, err: err}
	tool.numbers = numbers
	return tool, runner, numbers
}

func writeTempFile(t testing.TB, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// writeStateFileRecording writes a state file that records workDir as the
// directory Prepare created, so a test can exercise work-directory removal and
// its provenance check. It also writes the marker file Prepare puts in that
// directory, so removal accepts the directory.
func writeStateFileRecording(t *testing.T, dir, workDir string) string {
	t.Helper()
	state := testState
	state.WorkDir = workDir
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("encode state: %v", err)
	}
	writeTempFile(t, dir, markerFileName, markerContent)
	return writeTempFile(t, dir, stateFileName, string(data))
}

func writeScript(t testing.TB, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil { //nolint:gosec // the helper script must be executable
		t.Fatalf("write helper script: %v", err)
	}
	return path
}

// invalidBaseNames are base branch names that refs/remotes/origin/<name>
// makes an invalid ref name.
var invalidBaseNames = []string{"main^", "a:b", "a..b", "x~1", ""}

// hardened returns the arguments the reader passes git for args.
func hardened(args ...string) []string {
	return append(cmdrunner.GitHardening(), args...)
}

// fakeCorrectionSource stands in for the correction-line reading. It returns
// fixed range lines and merge-commit lines, records the arguments it was
// given, and reads trailers with the production git, so a check that depends
// on how git reads trailers is checked against git itself.
type fakeCorrectionSource struct {
	lines     []correctionLine
	err       error
	merged    [][]byte
	mergedErr error
	// trailFailAt makes the n-th trailers call fail (1-based); 0 never.
	trailFailAt int
	trailCalls  int
	real        *gitCorrectionSource
	gotBase     string // the base rangeLines or mergedLines was last given
	gotHead     string // the head rangeLines was last given
	gotMerge    string // the merge commit mergedLines was last given
	// runner, when set, gives the position each call is recorded at, as
	// fakeChecker records its calls, so a test can tell which commands ran
	// before a reading.
	runner *fakeRunner
	calls  []sourceCall
	// onRange, when set, runs inside each rangeLines call, so a test can
	// change a file while the correction check runs.
	onRange func()
}

// sourceCall records one reading: which method ran and how many scripted
// commands had run before it (-1 without a runner).
type sourceCall struct {
	method string
	index  int
}

func (f *fakeCorrectionSource) record(method string) {
	index := -1
	if f.runner != nil {
		index = f.runner.index
	}
	f.calls = append(f.calls, sourceCall{method: method, index: index})
}

func (f *fakeCorrectionSource) rangeLines(_ context.Context, base, head string) ([]correctionLine, error) {
	f.record("rangeLines")
	if f.onRange != nil {
		f.onRange()
	}
	f.gotBase, f.gotHead = base, head
	return f.lines, f.err
}

func (f *fakeCorrectionSource) mergedLines(_ context.Context, base, merge string) ([][]byte, error) {
	f.record("mergedLines")
	f.gotBase, f.gotMerge = base, merge
	return f.merged, f.mergedErr
}

func (f *fakeCorrectionSource) trailers(ctx context.Context, message []byte) ([]byte, error) {
	f.trailCalls++
	if f.trailCalls == f.trailFailAt {
		return nil, fmt.Errorf("%w: git interpret-trailers failed", errCorrectionUnreadable)
	}
	return f.real.trailers(ctx, message)
}

func newFakeSource(t testing.TB, lines ...correctionLine) *fakeCorrectionSource {
	t.Helper()
	return &fakeCorrectionSource{lines: lines, real: newGitCorrectionSource(t.TempDir())}
}
