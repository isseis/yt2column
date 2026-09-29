//go:build test

// Package publishertestutil provides test doubles for the publishing stage.
package publishertestutil

import (
	"context"

	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// FakePublisher is a configurable publisher.Publisher that records its calls.
// It implements the interface with pointer receivers so the recorded calls are
// visible to the caller.
type FakePublisher struct {
	Err   error
	Calls []FakePublisherCall
}

// FakePublisherCall records the arguments of one Publish call.
type FakePublisherCall struct {
	Ctx     context.Context
	Article writer.Article
}

var _ publisher.Publisher = (*FakePublisher)(nil)

// Publish records the call and returns the configured error.
func (f *FakePublisher) Publish(ctx context.Context, article writer.Article) error {
	f.Calls = append(f.Calls, FakePublisherCall{Ctx: ctx, Article: article})
	return f.Err
}
