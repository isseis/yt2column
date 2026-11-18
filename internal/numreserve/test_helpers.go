//go:build test

package numreserve

import (
	"context"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// newTestTool returns a Tool that runs every git and gh command on runner
// (usually one cmdrunner/testutil Fake, so the order across git and gh is
// checked too), with bounds b.
func newTestTool(runner cmdrunner.InputRunner, b bounds) *Tool {
	return &Tool{git: runner, gh: runner, bounds: b}
}

// deadlineRunner wraps a runner, records whether and when each call's
// context ends, and can hold one call until its context ends, to stand for a
// command that does not finish in time.
type deadlineRunner struct {
	inner cmdrunner.InputRunner
	// holdCall is the 1-based index of the call to hold; 0 holds none.
	holdCall int

	mu        sync.Mutex
	calls     int
	deadlines []time.Time
	missing   int
}

var _ cmdrunner.InputRunner = (*deadlineRunner)(nil)

// Run implements cmdrunner.Runner.
func (d *deadlineRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return d.RunInput(ctx, nil, name, args...)
}

// RunInput implements cmdrunner.InputRunner.
func (d *deadlineRunner) RunInput(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	d.mu.Lock()
	d.calls++
	call := d.calls
	if deadline, ok := ctx.Deadline(); ok {
		d.deadlines = append(d.deadlines, deadline)
	} else {
		d.missing++
	}
	d.mu.Unlock()
	if call == d.holdCall {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if stdin == nil {
		return d.inner.Run(ctx, name, args...)
	}
	return d.inner.RunInput(ctx, stdin, name, args...)
}
