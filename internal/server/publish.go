package server

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/publisher"
)

// WebhookPublisher posts to the Webhook: Publish for the legacy /articles job,
// PostMessages for the publish operation. The production value is
// *publisher.SlackWebhookPublisher, and a test uses the real publisher against
// a loopback Webhook or a fake of this interface.
type WebhookPublisher interface {
	publisher.Publisher
	PostMessages(ctx context.Context, m publisher.SlackMessages, from int, delivered func(index int)) error
}

// publishHandler serves a publish operation. It checks, in order, the method,
// that the server has a Webhook (refused whatever the key's state when it has
// none), that the request has no body (refused without reading one), and that
// keyPart is a key in its one accepted form, and then moves the key's record
// to posting. A done article not yet posted, or whose last post failed,
// starts a post job and is answered 202 before the Webhook is called; an
// article already posting or posted is answered 202 and posted no further, so
// the operation is idempotent. Every other record is refused.
//
// Ownership: the handler holds the PublishRun from BeginPublish until the
// s.jobs.Go call in startPublish hands it to the job. Its recover fails the
// post when it still holds the handle, so a panic never leaves a record
// posting. Nothing on this operation posts a failure notice.
func (s *Server) publishHandler(w http.ResponseWriter, r *http.Request, requestID, keyPart string) {
	responded := false
	var run *articlecache.PublishRun // non-nil while the handler owns it
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		s.logger.Error("request handler panicked", "requestId", requestID, "panic", p, "stack", string(debug.Stack()))
		if run != nil {
			run.Fail(articlecache.PublishFailureInternal)
		}
		if !responded {
			if err := writeRejected(w, requestID, reasonInternal, true); err != nil {
				s.logger.Error("response write failed", "requestId", requestID, "error", err)
			}
		}
	}()

	if rej := checkMethod(r, http.MethodPost); rej != nil {
		s.reject(w, requestID, *rej)
		responded = true
		return
	}
	if !s.webhookConfigured() {
		s.reject(w, requestID, rejection{reason: reasonWebhookNotConfigured, detail: detailWebhookNotConfigured})
		responded = true
		return
	}
	key, rej := checkKeyRequest(r, keyPart)
	if rej != nil {
		s.reject(w, requestID, *rej)
		responded = true
		return
	}

	outcome, begun := s.cache.BeginPublish(key)
	run = begun
	switch outcome {
	case articlecache.PublishBeginStarted:
		from := run.From()
		if s.beforePublishJob != nil {
			s.beforePublishJob()
		}
		// The job owns the PublishRun from here: once s.jobs.Go is called,
		// only the job ends it.
		s.startPublish(run, key)
		run = nil
		s.writePublishAccepted(w, requestID, key)
		responded = true
		s.logger.Info("publish accepted", "requestId", requestID, "key", key.String(), "outcome", "started", "from", from)
	case articlecache.PublishBeginAlready:
		s.writePublishAccepted(w, requestID, key)
		responded = true
		s.logger.Info("publish accepted", "requestId", requestID, "key", key.String(), "outcome", "already")
	case articlecache.PublishBeginUnpostable:
		s.reject(w, requestID, rejection{reason: reasonArticleNotPostable, detail: detailArticleNotPostable})
		responded = true
	case articlecache.PublishBeginNotReady:
		s.reject(w, requestID, rejection{reason: reasonArticleNotReady, detail: detailArticleNotReady})
		responded = true
	default:
		// PublishBeginNoArticle, the zero value: a failed or unknown key.
		s.reject(w, requestID, rejection{reason: reasonArticleNotFound, detail: detailArticleNotFound})
		responded = true
	}
}

// writePublishAccepted writes a publish operation's 202, logging a write
// failure.
func (s *Server) writePublishAccepted(w http.ResponseWriter, requestID string, key articlecache.Key) {
	if err := writeAccepted(w, requestID); err != nil {
		s.logger.Info("accepted response write failed", "requestId", requestID, "key", key.String(), "error", err)
	}
}

// startPublish runs the post job for key in its own goroutine. From the
// s.jobs.Go call on, the job owns run: it ends run in every case, a panic
// included. The job is counted in s.jobs and runs on jobCtx, so a shutdown
// waits for it instead of canceling it.
func (s *Server) startPublish(run *articlecache.PublishRun, key articlecache.Key) {
	s.postJobs.Add(1)
	s.jobs.Go(func() {
		defer s.postJobs.Add(-1)
		s.runPublish(run, key)
	})
}

// runPublish rebuilds the article's messages and posts them from the first one
// not delivered, recording each delivery on run as the Webhook acknowledges
// it, and ends run: posted on success, failed by the Webhook on a
// *publisher.SlackPostError, and failed as internal on any other error or a
// panic. The deliveries recorded before a failure or a panic stay recorded,
// so a retry starts at the first message not recorded as delivered. A message
// the Webhook accepted without the publisher seeing the acknowledgement (a
// timeout after Slack took it) is not recorded, so the retry sends it again:
// the requirements accept that one duplicate. The log line names the counts
// the transition returned, never a value read back from the record, which a
// later request may already have changed.
func (s *Server) runPublish(run *articlecache.PublishRun, key articlecache.Key) {
	total := 0
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		stack := string(debug.Stack())
		posted, ok := run.Fail(articlecache.PublishFailureInternal)
		if !ok {
			// The panic came after the job ended run: there is no failure
			// to record or count.
			s.logger.Error("post job panicked", "key", key.String(), "panic", p, "stack", stack)
			return
		}
		s.logger.Error("post job panicked",
			"key", key.String(),
			"posted", posted,
			"total", total,
			"reason", publishFailureCode(articlecache.PublishFailureInternal),
			"panic", p,
			"stack", stack,
		)
	}()

	messages, err := publisher.PrepareSlackMessages(run.Article())
	if err != nil {
		// The completion check accepted this article, and the messages
		// depend on the article alone, so this means a server bug.
		s.failPublish(run, key, total, articlecache.PublishFailureInternal, err)
		return
	}
	total = messages.Len()
	if err := s.publisher.PostMessages(s.jobCtx, messages, run.From(), run.Delivered); err != nil {
		reason := articlecache.PublishFailureInternal
		if _, ok := errors.AsType[*publisher.SlackPostError](err); ok {
			reason = articlecache.PublishFailureWebhook
		}
		s.failPublish(run, key, total, reason, err)
		return
	}
	posted, ok := run.Finish()
	if !ok {
		s.logEndedElsewhere(key)
		return
	}
	s.logger.Info("post job succeeded", "key", key.String(), "posted", posted, "total", total)
}

// logEndedElsewhere writes the line for a post job whose PublishRun was no
// longer valid when it reported its outcome, so nothing was recorded.
func (s *Server) logEndedElsewhere(key articlecache.Key) {
	s.logger.Warn("post job outcome not recorded: the record moved on", "key", key.String())
}

// failPublish fails run with reason and writes the one line for it, with the
// delivered count Fail returned. err comes from the publisher, whose errors
// never hold the Webhook URL or the article text.
func (s *Server) failPublish(run *articlecache.PublishRun, key articlecache.Key, total int, reason articlecache.PublishFailureReason, err error) {
	posted, ok := run.Fail(reason)
	if !ok {
		s.logEndedElsewhere(key)
		return
	}
	s.logger.Error("post job failed",
		"key", key.String(),
		"posted", posted,
		"total", total,
		"reason", publishFailureCode(reason),
		"error", err,
	)
}
