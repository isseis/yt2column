//go:build test

// Package publishertestutil provides the test double for the publishing
// stage.
package publishertestutil

import (
	"context"
	"sync"

	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// FakePublisher is a configurable publisher.Publisher that records its
// calls. It implements Publish with a pointer receiver so the recorded calls
// are visible to the caller. Calls may be made from several goroutines: the
// record is kept under a mutex, and RecordedCalls reads it while calls are
// running. A test that reads Calls directly does so after every call
// returned.
type FakePublisher struct {
	Err   error
	Calls []FakePublisherCall

	mu sync.Mutex
}

// FakePublisherCall records the arguments of one Publish call.
type FakePublisherCall struct {
	Ctx     context.Context
	Article writer.Article
}

var _ publisher.Publisher = (*FakePublisher)(nil)

// Publish records the call and returns the configured error.
func (f *FakePublisher) Publish(ctx context.Context, article writer.Article) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, FakePublisherCall{Ctx: ctx, Article: article})
	return f.Err
}

// RecordedCalls returns a copy of the Publish calls recorded so far. It is
// safe to call while other calls are running.
func (f *FakePublisher) RecordedCalls() []FakePublisherCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakePublisherCall(nil), f.Calls...)
}
