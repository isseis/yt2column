//go:build test

// Package writertestutil provides test doubles for the article-writing stage.
package writertestutil

import (
	"context"
	"errors"
	"sync"

	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// errUnknownProgressKind is returned by WriteWithProgress for a progress step
// whose Kind is not one of the FakeProgressKind values.
var errUnknownProgressKind = errors.New("writertestutil: unknown progress kind")

// errNilObserver is returned by WriteWithProgress for a nil or typed-nil
// observer, as the real writer rejects one.
var errNilObserver = errors.New("writertestutil: progress observer is nil")

// FakeArticleWriter is a configurable writer.ProgressWriter that records its
// calls. It implements the interface with pointer receivers so the recorded
// calls are visible to the caller. Calls may be made from several goroutines:
// the record is kept under a mutex, and RecordedCalls reads it while calls
// are running. A test that reads Calls directly does so after every call
// returned.
type FakeArticleWriter struct {
	Result writer.Article
	Err    error
	// Progress lists the notifications WriteWithProgress sends, in order,
	// before it returns Result and Err. Write sends none.
	Progress []FakeProgressStep
	Calls    []FakeArticleWriterCall

	mu sync.Mutex
}

// FakeProgressKind selects the ProgressObserver method a FakeProgressStep
// calls. The zero value is not a kind: WriteWithProgress rejects it.
type FakeProgressKind int

const (
	// FakeStageStarted calls StageStarted.
	FakeStageStarted FakeProgressKind = iota + 1
	// FakeStageFinished calls StageFinished.
	FakeStageFinished
	// FakeStageSkipped calls StageSkipped.
	FakeStageSkipped
)

// FakeProgressStep is one notification of FakeArticleWriter.WriteWithProgress.
type FakeProgressStep struct {
	Kind  FakeProgressKind
	Stage writer.Stage
	// Reached, when non-nil, receives one value from every call that sends
	// the notification, right after sending it, so a test can wait for each
	// call to get there. The send waits for a receiver (or buffer space) or
	// for ctx to be done, so concurrent calls never panic on it; a test that
	// does not receive from it gives it room for every call.
	Reached chan<- struct{}
	// Release, when non-nil, holds the call after the notification (and
	// after the Reached send) until Release is closed or ctx is done.
	// Closing it releases every call held on it. When ctx is done first, the
	// call returns the zero Article and ctx's error.
	Release <-chan struct{}
}

// FakeArticleWriterCall records the arguments of one Write or
// WriteWithProgress call.
type FakeArticleWriterCall struct {
	Ctx        context.Context
	Transcript transcript.Transcript
}

var _ writer.ProgressWriter = (*FakeArticleWriter)(nil)

// Write records the call and returns the configured result and error.
func (f *FakeArticleWriter) Write(ctx context.Context, t transcript.Transcript) (writer.Article, error) {
	f.record(ctx, t)
	return f.Result, f.Err
}

// WriteWithProgress records the call, sends each Progress step to observer in
// order, and returns the configured result and error.
func (f *FakeArticleWriter) WriteWithProgress(ctx context.Context, t transcript.Transcript, observer writer.ProgressObserver) (writer.Article, error) {
	f.record(ctx, t)
	if nilcheck.IsNil(observer) {
		return writer.Article{}, errNilObserver
	}
	for _, step := range f.Progress {
		switch step.Kind {
		case FakeStageStarted:
			observer.StageStarted(step.Stage)
		case FakeStageFinished:
			observer.StageFinished(step.Stage)
		case FakeStageSkipped:
			observer.StageSkipped(step.Stage)
		default:
			return writer.Article{}, errUnknownProgressKind
		}
		if step.Reached != nil {
			select {
			case step.Reached <- struct{}{}:
			case <-ctx.Done():
				return writer.Article{}, ctx.Err()
			}
		}
		if step.Release != nil {
			select {
			case <-step.Release:
			case <-ctx.Done():
				return writer.Article{}, ctx.Err()
			}
		}
	}
	return f.Result, f.Err
}

// RecordedCalls returns a copy of the calls recorded so far. It is safe to
// call while other calls are running.
func (f *FakeArticleWriter) RecordedCalls() []FakeArticleWriterCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FakeArticleWriterCall(nil), f.Calls...)
}

func (f *FakeArticleWriter) record(ctx context.Context, t transcript.Transcript) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, FakeArticleWriterCall{Ctx: ctx, Transcript: t})
}
