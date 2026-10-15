package server

import (
	"context"
	"errors"
	"runtime/debug"
	"time"

	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/pipeline"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// maxInFlight bounds the authenticated requests in progress: from the body read
// through the end of the job or of the failure notice. A request that cannot
// take a slot is refused at once; there is no queue.
const maxInFlight = 2

// notifyTimeout bounds one failure notice. The notifier has its own shorter
// bound; this is a second, outer limit so a notice cannot hold a slot forever.
const notifyTimeout = 60 * time.Second

// acquireSlot takes one slot without waiting. It reports false when both are
// taken.
func (s *Server) acquireSlot() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

// releaseSlot gives back one slot.
func (s *Server) releaseSlot() {
	<-s.slots
}

// startJob records the job before it starts and runs it in its own goroutine.
// The caller must hold a slot; ownership of it passes to the goroutine, which
// releases it when the job and any notice are done.
func (s *Server) startJob(input jobInput) {
	s.jobs.Go(func() {
		defer s.releaseSlot()
		s.runJob(input)
	})
}

// runJob builds the pipeline for one job and runs it. Its own panic and a
// pipeline failure are both reported as a failure notice, so one bad job cannot
// take down the process. The job holds its slot throughout.
func (s *Server) runJob(input jobInput) {
	start := time.Now()
	defer func() {
		if p := recover(); p != nil {
			s.logger.Error("job panicked", "requestId", input.requestID, "videoId", input.videoID, "panic", p, "stack", string(debug.Stack()))
			s.notify(input.requestID, input.videoID, failure.KindInternal)
		}
	}()

	p, err := pipeline.New(input.source, s.writer, s.publisher)
	if err != nil {
		s.logger.Error("pipeline construction failed", "requestId", input.requestID, "videoId", input.videoID, "error", err)
		s.notify(input.requestID, input.videoID, failure.KindInternal)
		return
	}
	article, err := p.Run(s.jobCtx, input.videoURL)
	if err != nil {
		kind := classify(err)
		s.logJobFailure(input, kind, err)
		s.notify(input.requestID, input.videoID, kind)
		return
	}
	s.logJobSuccess(input, article, time.Since(start))
}

// notify posts one failure notice while the caller still holds a slot. A notice
// failure, and a panic from the notifier, are logged and otherwise ignored; the
// server keeps running. The recover keeps a notifier panic from escaping into
// the caller's own recovery, where it would crash the process.
func (s *Server) notify(requestID, videoID string, kind failure.Kind) {
	defer func() {
		if p := recover(); p != nil {
			s.logger.Error("failure notice panicked", "requestId", requestID, "kind", int(kind), "panic", p, "stack", string(debug.Stack()))
		}
	}()
	ctx, cancel := context.WithTimeout(s.jobCtx, notifyTimeout)
	defer cancel()
	notice := failure.Notice{RequestID: requestID, VideoID: videoID, Kind: kind}
	if err := s.notifier.Notify(ctx, notice); err != nil {
		s.logger.Error("failure notice failed", "requestId", requestID, "kind", int(kind), "error", err)
	}
}

// notifyAsync posts a failure notice in its own goroutine, so it does not delay
// the rejection response. The caller must hold a slot; ownership of it passes
// to the goroutine.
func (s *Server) notifyAsync(requestID, videoID string, kind failure.Kind) {
	s.jobs.Go(func() {
		defer s.releaseSlot()
		s.notify(requestID, videoID, kind)
	})
}

// classify maps a pipeline failure to a failure.Kind. It reads the failure only
// from the pipeline stage and the wrapped sentinels, never from a message.
func classify(err error) failure.Kind {
	stageErr, ok := errors.AsType[*pipeline.StageError](err)
	if !ok {
		// pipeline.Run returns the context error between stages; the job
		// context is not canceled, so this is normally unreachable.
		return failure.KindUnknown
	}
	switch stageErr.Stage {
	case pipeline.StageTranscript:
		// The parsed source's Fetch failed (a URL mismatch), not a request
		// failure.
		return failure.KindInternal
	case pipeline.StagePublish:
		return failure.KindPublish
	case pipeline.StageWrite:
		return classifyWrite(stageErr.Err)
	default:
		return failure.KindUnknown
	}
}

// classifyWrite maps a write-stage failure to its kind. The order matters:
// writer wraps a prompt-too-large failure with ErrInvalidTemplate as well, so
// ErrPromptTooLarge is checked first.
func classifyWrite(err error) failure.Kind {
	switch {
	case errors.Is(err, writer.ErrPromptTooLarge):
		return failure.KindTranscriptTooLong
	case errors.Is(err, context.DeadlineExceeded):
		return failure.KindLLMTimeout
	case errors.Is(err, llm.ErrEmptyResponse):
		return failure.KindLLMEmpty
	case errors.Is(err, llm.ErrTruncated):
		return failure.KindLLMTruncated
	case errors.Is(err, writer.ErrInvalidTranscript), errors.Is(err, writer.ErrInvalidTemplate):
		// With the embedded templates and a checked transcript these cannot
		// happen, so a match means the server built something wrong.
		return failure.KindInternal
	default:
		return failure.KindLLM
	}
}

// reasonToKind maps a rejection reason to the failure.Kind a notice reports.
// The reasons that only happen before a slot is taken map to KindUnknown, the
// value the notice text treats as unclassified; they are never notified.
func reasonToKind(reason rejectReason) failure.Kind {
	switch reason {
	case reasonBodyTooLarge:
		return failure.KindBodyTooLarge
	case reasonInvalidBody:
		return failure.KindInvalidBody
	case reasonInvalidVideoURL:
		return failure.KindInvalidVideoURL
	case reasonInvalidTranscript:
		return failure.KindInvalidTranscript
	case reasonInternal:
		return failure.KindInternal
	default:
		return failure.KindUnknown
	}
}

// logJobFailure writes the one line for a failed job. It names the failure only
// with fixed values (the kind and the stage) and the wrapped error, which never
// holds a secret or transcript text.
func (s *Server) logJobFailure(input jobInput, kind failure.Kind, err error) {
	attrs := []any{
		"requestId", input.requestID,
		"videoId", input.videoID,
		"kind", int(kind),
		"stage", stageOf(err).String(),
		"error", err,
	}
	if postErr, ok := errors.AsType[*publisher.SlackPostError](err); ok {
		attrs = append(attrs, "posted", postErr.Posted, "total", postErr.Total)
	}
	s.logger.Error("job failed", attrs...)
}

// logJobSuccess writes the one line for a successful job.
func (s *Server) logJobSuccess(input jobInput, article writer.Article, elapsed time.Duration) {
	attrs := []any{
		"requestId", input.requestID,
		"videoId", input.videoID,
		"model", article.Model,
		"modelVersion", article.ModelVersion,
		"duration", elapsed,
	}
	if count, err := publisher.SlackMessageCount(article); err == nil {
		attrs = append(attrs, "messages", count)
	}
	s.logger.Info("job succeeded", attrs...)
}

// stageOf returns the pipeline stage of a failure, or StageUnknown.
func stageOf(err error) pipeline.Stage {
	if stageErr, ok := errors.AsType[*pipeline.StageError](err); ok {
		return stageErr.Stage
	}
	return pipeline.StageUnknown
}
