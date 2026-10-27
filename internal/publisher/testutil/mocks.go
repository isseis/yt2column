//go:build test

package publishertestutil

import (
	"context"
	"errors"
	"sync"

	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// FakePublisher is a configurable publisher.Publisher that also has the
// PostMessages method of publisher.SlackWebhookPublisher, and records its
// calls. It implements them with pointer receivers so the recorded calls are
// visible to the caller. Calls may be made from several goroutines: the
// records are kept under a mutex, and RecordedCalls and RecordedPostCalls read
// them while calls are running. A test that reads Calls or PostCalls directly
// does so after every call returned.
type FakePublisher struct {
	Err error
	// PostErr is what PostMessages returns. When it is nil, PostMessages
	// calls delivered for every index from from to the last message, in
	// order; otherwise it calls delivered for none. A partial delivery is
	// not simulated: a test that needs one posts to an httptest Webhook
	// with the real publisher.
	PostErr   error
	Calls     []FakePublisherCall
	PostCalls []FakePostCall

	mu sync.Mutex
}

// FakePublisherCall records the arguments of one Publish call.
type FakePublisherCall struct {
	Ctx     context.Context
	Article writer.Article
}

// FakePostCall records the arguments of one PostMessages call.
type FakePostCall struct {
	Ctx      context.Context
	Messages publisher.SlackMessages
	From     int
}

var _ publisher.Publisher = (*FakePublisher)(nil)

// ErrFakePostRefused is what FakePublisher.PostMessages returns for a call
// the real PostMessages refuses before sending. Like the real refusals, it is
// not a *publisher.SlackPostError.
var ErrFakePostRefused = errors.New("publishertestutil: PostMessages call refused")

// Publish records the call and returns the configured error.
func (f *FakePublisher) Publish(ctx context.Context, article writer.Article) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, FakePublisherCall{Ctx: ctx, Article: article})
	return f.Err
}

// PostMessages records the call and refuses, as the real method does, empty
// messages, a from outside 0..m.Len(), and a nil delivered, with
// ErrFakePostRefused. Otherwise it reports every message from from on as
// delivered when PostErr is nil, and returns PostErr.
func (f *FakePublisher) PostMessages(ctx context.Context, m publisher.SlackMessages, from int, delivered func(index int)) error {
	f.mu.Lock()
	f.PostCalls = append(f.PostCalls, FakePostCall{Ctx: ctx, Messages: m, From: from})
	err := f.PostErr
	f.mu.Unlock()
	if m.Len() == 0 || from < 0 || from > m.Len() || delivered == nil {
		return ErrFakePostRefused
	}
	if err != nil {
		return err
	}
	// delivered runs outside the mutex, so it may read the record.
	for i := from; i < m.Len(); i++ {
		delivered(i)
	}
	return nil
}

// RecordedCalls returns a copy of the Publish calls recorded so far. It is
// safe to call while other calls are running.
func (f *FakePublisher) RecordedCalls() []FakePublisherCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakePublisherCall(nil), f.Calls...)
}

// RecordedPostCalls returns a copy of the PostMessages calls recorded so far.
// It is safe to call while other calls are running.
func (f *FakePublisher) RecordedPostCalls() []FakePostCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakePostCall(nil), f.PostCalls...)
}
