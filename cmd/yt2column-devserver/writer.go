package main

import (
	"context"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// writerOptions returns the writer options of the production server: the
// shorten and review steps on, the review step on review. The production
// server's own copy is in package main of cmd/yt2column-server and cannot be
// imported; both are compared with writertestutil.DefaultOptions in tests.
func writerOptions(review llm.LLMClient) writer.Options {
	return writer.Options{
		Shorten: writer.ShortenOptions{Enabled: true},
		Review:  writer.ReviewOptions{Enabled: true, Client: review},
	}
}

// perJobWriter is the writer.ProgressWriter the development server gives the
// server. Contract: every Write and WriteWithProgress builds a new Job, so a
// generation shares no client, and so no call count, with another generation
// or with an earlier one, and builds a writer for it with the production
// options.
type perJobWriter struct {
	scenario  devfake.Scenario
	newWriter func(llm.LLMClient, writer.Options) (writer.ProgressWriter, error)
}

// jobWriter builds the writer of one generation job.
func (p perJobWriter) jobWriter() (writer.ProgressWriter, error) {
	job := p.scenario.NewJob()
	return p.newWriter(job.Generation(), writerOptions(job.Review()))
}

// Write implements writer.ArticleWriter. A failure to build the writer is
// reported as writer.ArticleWriter's contract requires, inside a
// *writer.TokenUsageError whose usage is empty because no call was made.
func (p perJobWriter) Write(ctx context.Context, t transcript.Transcript) (writer.Article, error) {
	w, err := p.jobWriter()
	if err != nil {
		return writer.Article{}, &writer.TokenUsageError{Err: err}
	}
	return w.Write(ctx, t)
}

// WriteWithProgress implements writer.ProgressWriter, with the same failure
// contract as Write.
func (p perJobWriter) WriteWithProgress(ctx context.Context, t transcript.Transcript, observer writer.ProgressObserver) (writer.Article, error) {
	w, err := p.jobWriter()
	if err != nil {
		return writer.Article{}, &writer.TokenUsageError{Err: err}
	}
	return w.WriteWithProgress(ctx, t, observer)
}
