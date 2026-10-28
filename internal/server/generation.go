package server

import (
	"errors"
	"runtime/debug"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// startGeneration runs the generation job for key in its own goroutine. The
// caller holds a generation slot and run, the handle Begin returned; from the
// s.jobs.Go call on, the job owns both: it ends run in every case, a panic
// included, and releases the slot.
//
// Contract: the job keeps the copied transcript (input.source and the
// Transcript it returns) to itself. The record receives only the article,
// which the cache stores without the review's revisions, so no transcript
// text outlives the job.
func (s *Server) startGeneration(run *articlecache.Run, key articlecache.Key, input jobInput) {
	s.jobs.Go(func() {
		defer s.releaseSlot()
		s.runGeneration(run, key, input)
	})
}

// runGeneration generates the article and ends run with the outcome: done
// (postable or not) after the completion check, or failed with its reason.
// A panic is recovered and fails run as internal, so the record never stays
// running. A failure that came from genCtx being canceled, which happens only
// when the server shuts down, is logged as a cancellation, not a failure.
func (s *Server) runGeneration(run *articlecache.Run, key articlecache.Key, input jobInput) {
	defer func() {
		if p := recover(); p != nil {
			run.Fail(articlecache.FailureInternal)
			s.logger.Error("generation job panicked", "key", key.String(), "videoId", input.videoID, "panic", p, "stack", string(debug.Stack()))
		}
	}()

	t, err := input.source.Fetch(s.genCtx, input.videoURL)
	if err != nil {
		// The parsed source's Fetch failed: a URL mismatch, which the
		// validated input cannot cause, or the cancellation.
		s.failGeneration(run, key, input, articlecache.FailureInternal, err)
		return
	}
	article, err := s.writer.WriteWithProgress(s.genCtx, t, run)
	if err != nil {
		s.failGeneration(run, key, input, classifyWrite(err), err)
		return
	}
	s.completeGeneration(run, key, input, article)
}

// completeGeneration runs the completion check on a generated article and ends
// run with its outcome. The checks run in this order:
//
//  1. CheckPublishable: an article no output can render fails as invalid.
//  2. The status response size: an article whose status response could exceed
//     maxStatusResponseBytes fails as too large, so no response is ever
//     refused for its size.
//  3. The Slack pre-send checks: an article they refuse is done but
//     unpostable, with the mention reason when they refused a mention.
//
// The third check runs with or without a Webhook: its result depends on the
// article alone.
func (s *Server) completeGeneration(run *articlecache.Run, key articlecache.Key, input jobInput, article writer.Article) {
	if err := article.CheckPublishable(); err != nil {
		s.failGeneration(run, key, input, articlecache.FailureArticleInvalid, err)
		return
	}
	size, err := maxStatusSize(article)
	if err != nil {
		// CheckPublishable passed, so rendering cannot fail.
		s.failGeneration(run, key, input, articlecache.FailureInternal, err)
		return
	}
	if size > maxStatusResponseBytes {
		s.failGeneration(run, key, input, articlecache.FailureArticleTooLarge, errStatusTooLarge)
		return
	}
	publish := articlecache.PublishNotPosted
	var duration time.Duration
	if _, err := publisher.PrepareSlackMessages(article); err != nil {
		reason := articlecache.UnpostableOther
		if errors.Is(err, publisher.ErrSlackMention) {
			reason = articlecache.UnpostableMention
		}
		duration, _ = run.CompleteUnpostable(article, reason)
		publish = articlecache.PublishUnpostable
	} else {
		duration, _ = run.CompletePostable(article)
	}
	s.logger.Info("generation job succeeded",
		"key", key.String(),
		"videoId", input.videoID,
		"model", article.Model,
		"modelVersion", article.ModelVersion,
		"duration", duration,
		"publish", publishStateCode(publish),
	)
}

// errStatusTooLarge is the error the generation failure log names for an
// article whose status response would exceed maxStatusResponseBytes.
var errStatusTooLarge = errors.New("the status response would exceed its size limit")

// failGeneration fails run with reason and writes the one line for it: the
// failure line, or, when genCtx was canceled, the cancellation line alone. The
// failed step is the one Fail marked, as Fail returns it: the record is not
// read back, since a retry may already have reset its stages.
func (s *Server) failGeneration(run *articlecache.Run, key articlecache.Key, input jobInput, reason articlecache.FailureReason, err error) {
	stages, _ := run.Fail(reason)
	if s.genCtx.Err() != nil {
		s.logger.Info("generation job canceled by the shutdown", "key", key.String(), "videoId", input.videoID)
		return
	}
	s.logger.Error("generation job failed",
		"key", key.String(),
		"videoId", input.videoID,
		"failure", failureCode(reason),
		"step", failedStep(stages),
		"error", err,
	)
}

// failedStep returns the code of the step a failed record marks failed, or ""
// when the failure came outside the steps.
func failedStep(st articlecache.Stages) string {
	switch {
	case st.Draft == articlecache.StageFailed:
		return "draft"
	case st.Shorten == articlecache.StageFailed:
		return "shorten"
	case st.Review == articlecache.StageFailed:
		return "review"
	default:
		return ""
	}
}
