//go:build test

// Package failuretestutil provides test doubles for the failure notifier.
package failuretestutil

import (
	"context"

	"github.com/isseis/yt2column/internal/failure"
)

// FakeNotifier is a configurable failure.Notifier that records its calls. It
// implements the interface with pointer receivers so the recorded calls are
// visible to the caller.
type FakeNotifier struct {
	Err   error
	Calls []FakeNotifierCall
}

// FakeNotifierCall records the arguments of one Notify call.
type FakeNotifierCall struct {
	Ctx    context.Context
	Notice failure.Notice
}

var _ failure.Notifier = (*FakeNotifier)(nil)

// Notify records the call and returns the configured error.
func (f *FakeNotifier) Notify(ctx context.Context, notice failure.Notice) error {
	f.Calls = append(f.Calls, FakeNotifierCall{Ctx: ctx, Notice: notice})
	return f.Err
}
