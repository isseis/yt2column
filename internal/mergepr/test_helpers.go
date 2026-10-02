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

func writeScript(t testing.TB, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil { //nolint:gosec // the helper script must be executable
		t.Fatalf("write helper script: %v", err)
	}
	return path
}
