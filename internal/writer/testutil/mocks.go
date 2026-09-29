//go:build test

// Package writertestutil provides test doubles for the article-writing stage.
package writertestutil

import (
	"context"

	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// FakeArticleWriter is a configurable writer.ArticleWriter that records its
// calls. It implements the interface with pointer receivers so the recorded
// calls are visible to the caller.
type FakeArticleWriter struct {
	Result writer.Article
	Err    error
	Calls  []FakeArticleWriterCall
}

// FakeArticleWriterCall records the arguments of one Write call.
type FakeArticleWriterCall struct {
	Ctx        context.Context
	Transcript transcript.Transcript
}

var _ writer.ArticleWriter = (*FakeArticleWriter)(nil)

// Write records the call and returns the configured result and error.
func (f *FakeArticleWriter) Write(ctx context.Context, t transcript.Transcript) (writer.Article, error) {
	f.Calls = append(f.Calls, FakeArticleWriterCall{Ctx: ctx, Transcript: t})
	return f.Result, f.Err
}
