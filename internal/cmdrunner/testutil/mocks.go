//go:build test

// Package cmdrunnertestutil provides a test double for cmdrunner.InputRunner.
package cmdrunnertestutil

import (
	"context"
	"slices"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// TB is the part of testing.TB the fake needs; *testing.T satisfies it.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Step scripts one expected command and its result. Effect, when set, runs
// before the result is returned and stands for a side effect of the real
// command (for example `make fmt` rewriting a file); a non-nil error from it
// becomes the command's error.
type Step struct {
	Name   string
	Args   []string
	Out    string
	Err    error
	Effect func() error
}

// Fake replays a fixed command sequence and fails the test on the first
// mismatch or extra call, so a test cannot pass by skipping a step.
type Fake struct {
	t     TB
	steps []Step
	index int

	// Stdin records the standard input of each call, in order; a call made
	// through Run records nil.
	Stdin [][]byte
}

var _ cmdrunner.InputRunner = (*Fake)(nil)

// New returns a Fake that expects exactly steps, in order.
func New(t TB, steps ...Step) *Fake {
	return &Fake{t: t, steps: steps}
}

// Run implements cmdrunner.Runner.
func (f *Fake) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.t.Helper()
	return f.RunInput(ctx, nil, name, args...)
}

// RunInput implements cmdrunner.InputRunner.
func (f *Fake) RunInput(_ context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	f.t.Helper()
	if f.index >= len(f.steps) {
		f.t.Fatalf("command %d: unexpected call %s %q", f.index+1, name, args)
	}
	step := f.steps[f.index]
	f.index++
	if step.Name != name || !slices.Equal(step.Args, args) {
		f.t.Fatalf("command %d: got %s %q, want %s %q", f.index, name, args, step.Name, step.Args)
	}
	f.Stdin = append(f.Stdin, stdin)
	if step.Effect != nil {
		if err := step.Effect(); err != nil {
			return nil, err
		}
	}
	if step.Err != nil {
		return nil, step.Err
	}
	return []byte(step.Out), nil
}

// Done fails the test when fewer commands ran than were scripted.
func (f *Fake) Done() {
	f.t.Helper()
	if f.index != len(f.steps) {
		f.t.Fatalf("ran %d of %d scripted commands", f.index, len(f.steps))
	}
}
