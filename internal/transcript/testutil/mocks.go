//go:build test

// Package transcripttestutil provides test doubles for the transcript stage.
package transcripttestutil

import (
	"context"

	"github.com/isseis/yt2column/internal/transcript"
)

// FakeTranscriptSource is a configurable transcript.TranscriptSource that
// records its calls. It implements the interface with pointer receivers so the
// recorded calls are visible to the caller.
type FakeTranscriptSource struct {
	Result transcript.Transcript
	Err    error
	Calls  []FakeTranscriptSourceCall
}

// FakeTranscriptSourceCall records the arguments of one Fetch call.
type FakeTranscriptSourceCall struct {
	Ctx      context.Context
	VideoURL string
}

var _ transcript.TranscriptSource = (*FakeTranscriptSource)(nil)

// Fetch records the call and returns the configured result and error.
func (f *FakeTranscriptSource) Fetch(ctx context.Context, videoURL string) (transcript.Transcript, error) {
	f.Calls = append(f.Calls, FakeTranscriptSourceCall{Ctx: ctx, VideoURL: videoURL})
	return f.Result, f.Err
}
