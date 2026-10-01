//go:build test

package mergepr

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// commandStep scripts one expected command and its result.
type commandStep struct {
	name string
	args []string
	out  string
	err  error
}

func gitStep(args []string, out string) commandStep {
	return commandStep{name: gitCommand, args: args, out: out}
}

func ghStep(args []string, out string) commandStep {
	return commandStep{name: ghCommand, args: args, out: out}
}

// fakeRunner replays a fixed command sequence and fails the test on the first
// mismatch, extra call, or command that was not run with a deadline, so a test
// cannot pass by skipping a step or by dropping the timeout.
type fakeRunner struct {
	t     testing.TB
	steps []commandStep
	index int
}

// Run implements Runner.
func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		f.t.Errorf("command %s %q ran without a deadline", name, args)
	}
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

const (
	testPRNumber = 42
	testHeadOID  = "2222222222222222222222222222222222222222"
	testBaseOID  = "1111111111111111111111111111111111111111"
	testOtherOID = "4444444444444444444444444444444444444444"
	testMergeOID = "3333333333333333333333333333333333333333"

	testFetchURLOut  = "git@github.com:isseis/yt2column.git\n"
	testFetchURL     = "git@github.com:isseis/yt2column.git"
	testRepoViewOut  = `{"nameWithOwner":"isseis/yt2column","url":"https://github.com/isseis/yt2column"}`
	testRepoArg      = "isseis/yt2column"
	testRefsWildcard = "+refs/heads/*:refs/remotes/origin/*"
)

func writeStateFile(t testing.TB, dir string) string {
	t.Helper()
	state := State{
		Number:      testPRNumber,
		Owner:       "isseis",
		Repo:        "yt2column",
		HeadRefName: "feature/foo",
		HeadRefOID:  testHeadOID,
		BaseRefName: "main",
		Title:       "Test PR",
		URL:         "https://github.com/isseis/yt2column/pull/42",
	}
	path := filepath.Join(dir, stateFileName)
	if err := writeState(path, state); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return path
}

func writeTempFile(t testing.TB, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func writeScript(t testing.TB, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil { //nolint:gosec // the helper script must be executable
		t.Fatalf("write helper script: %v", err)
	}
	return path
}
