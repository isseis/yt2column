//go:build test

package mergepr

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
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
	if name != step.name || !slices.Equal(args, step.args) {
		f.t.Fatalf("command %d = %s %q, want %s %q", f.index, name, args, step.name, step.args)
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

func newTool(t testing.TB, steps ...commandStep) (*Tool, *fakeRunner) {
	t.Helper()
	runner := &fakeRunner{t: t, steps: steps}
	return New(runner), runner
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
// its provenance check.
func writeStateFileRecording(t *testing.T, dir, workDir string) string {
	t.Helper()
	state := testState
	state.WorkDir = workDir
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("encode state: %v", err)
	}
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
