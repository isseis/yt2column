// Package pipeline runs the transcript, article-writing, and publishing stages
// in order.
package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// Stage identifies the pipeline stage that failed.
// The zero value is StageUnknown: an unset Stage reports "unknown", never a
// wrong stage name.
type Stage int

const (
	// StageUnknown is the zero value. It reports "unknown".
	StageUnknown Stage = iota
	// StageTranscript is the transcript-fetch stage.
	StageTranscript
	// StageWrite is the article-writing stage.
	StageWrite
	// StagePublish is the publishing stage.
	StagePublish
)

// String returns the stage name. The zero value and unknown values return
// "unknown".
func (s Stage) String() string {
	switch s {
	case StageTranscript:
		return "transcript"
	case StageWrite:
		return "write"
	case StagePublish:
		return "publish"
	default:
		return "unknown"
	}
}

// ErrNilStage is wrapped by New, and by Run on a zero-value Pipeline, when a
// stage is not set.
var ErrNilStage = errors.New("nil pipeline stage")

// StageError wraps a stage failure. Unwrap returns the original error.
type StageError struct {
	Stage Stage
	Err   error
}

// Error returns "<stage name>: <original error>".
func (e *StageError) Error() string {
	return e.Stage.String() + ": " + e.Err.Error()
}

// Unwrap returns the original error.
func (e *StageError) Unwrap() error {
	return e.Err
}

// Pipeline runs the three stages in order.
type Pipeline struct {
	source    transcript.TranscriptSource
	writer    writer.ArticleWriter
	publisher publisher.Publisher
}

// New validates the stages and constructs a Pipeline. It returns an error
// wrapping ErrNilStage and naming the stage when a stage is not set, including
// a non-nil interface whose dynamic value is nil (a typed nil).
func New(source transcript.TranscriptSource, writer writer.ArticleWriter, publisher publisher.Publisher) (*Pipeline, error) {
	p := &Pipeline{source: source, writer: writer, publisher: publisher}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Run takes a video URL, calls each stage in order, and returns the published
// article. It returns an error wrapping ErrNilStage if a stage is not set, and
// the context error if the context is canceled between stages.
func (p *Pipeline) Run(ctx context.Context, videoURL string) (writer.Article, error) {
	if err := p.validate(); err != nil {
		return writer.Article{}, err
	}
	if err := ctx.Err(); err != nil {
		return writer.Article{}, err
	}

	t, err := p.source.Fetch(ctx, videoURL)
	if err != nil {
		return writer.Article{}, &StageError{Stage: StageTranscript, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return writer.Article{}, err
	}

	article, err := p.writer.Write(ctx, t)
	if err != nil {
		return writer.Article{}, &StageError{Stage: StageWrite, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return writer.Article{}, err
	}

	if err := p.publisher.Publish(ctx, article); err != nil {
		return writer.Article{}, &StageError{Stage: StagePublish, Err: err}
	}
	return article, nil
}

// validate reports an error wrapping ErrNilStage for each unset stage.
func (p *Pipeline) validate() error {
	if err := checkStage("transcript", p.source); err != nil {
		return err
	}
	if err := checkStage("write", p.writer); err != nil {
		return err
	}
	return checkStage("publish", p.publisher)
}

// checkStage rejects a nil interface and a typed-nil interface value.
func checkStage(name string, stage any) error {
	if nilcheck.IsNil(stage) {
		return fmt.Errorf("%w: %s", ErrNilStage, name)
	}
	return nil
}
