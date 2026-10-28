package server

import (
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/isseis/yt2column/internal/failure"
)

// ArticlesPath is the request target of the one endpoint. The handler accepts
// exactly this string, before any decoding, and no query.
const ArticlesPath = "/articles"

// handler validates a request in a fixed order and, on success, starts the job
// that generates and posts the article. Every request gets a request ID, which
// is written to the response, the log, and any failure notice. The handler
// recovers its own panics, so one request cannot take down the process.
func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	requestID, err := newRequestID()
	if err != nil {
		s.logger.Error("request ID generation failed", "error", err)
		_ = writeRejected(w, "", reasonInternal, true)
		return
	}

	responded := false
	slotHeld := false
	defer func() {
		p := recover()
		if p == nil {
			return
		}
		if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) && !slotHeld {
			// The handler's own abort after a body-read failure: let net/http
			// close the connection without writing a response.
			panic(p)
		}
		s.logger.Error("request handler panicked", "requestId", requestID, "panic", p, "stack", string(debug.Stack()))
		if !responded {
			if err := writeRejected(w, requestID, reasonInternal, !slotHeld); err != nil {
				s.logger.Error("response write failed", "requestId", requestID, "error", err)
			}
			responded = true
		}
		if slotHeld {
			s.notifyAsync(requestID, "", failure.KindInternal)
		}
	}()

	if rej := s.checkHeaders(r); rej != nil {
		s.reject(w, requestID, *rej)
		responded = true
		return
	}

	// Without a Webhook there is nowhere to post the article or its failure
	// notice, so the request is refused before any slot or body read.
	if !s.webhookConfigured() {
		s.reject(w, requestID, rejection{reason: reasonWebhookNotConfigured, detail: detailWebhookNotConfigured})
		responded = true
		return
	}

	if !s.acquireSlot() {
		s.reject(w, requestID, rejection{reason: reasonTooManyRequests, detail: detailTooManyRequests})
		responded = true
		return
	}
	slotHeld = true

	input, rej := s.readRequest(w, r, requestID)
	if rej != nil {
		if rej.readFailed {
			// A cut connection, a timeout, or a malformed chunked body: there
			// is no response and no notice, only a log line. No goroutine took
			// the slot, so the handler releases it. Returning normally would
			// make net/http write an empty 200, so the handler aborts instead,
			// which closes the connection with no response at all.
			s.logger.Warn("request body read failed", "requestId", requestID, "error", rej.err)
			s.releaseSlot()
			slotHeld = false
			responded = true
			panic(http.ErrAbortHandler)
		}
		s.reject(w, requestID, *rej)
		responded = true
		if rej.reason.notify() {
			// The body was read, so the connection can be reused; the notice
			// runs in its own goroutine and keeps the slot.
			s.notifyAsync(requestID, rej.videoID, reasonToKind(rej.reason))
		} else {
			s.releaseSlot()
		}
		slotHeld = false
		return
	}

	// The job owns the slot from here. Start it before writing 202, so no
	// response can be observed without its job running.
	s.startJob(*input)
	slotHeld = false
	if err := writeAccepted(w, requestID); err != nil {
		s.logger.Info("accepted response write failed", "requestId", requestID, "videoId", input.videoID, "error", err)
	}
	responded = true
	s.logger.Info("request accepted", "requestId", requestID, "videoId", input.videoID)
}

// reject logs the rejection and writes its response.
func (s *Server) reject(w http.ResponseWriter, requestID string, rej rejection) {
	s.logger.Warn("request rejected",
		"requestId", requestID,
		"status", rej.reason.statusCode(),
		"reason", rej.reason.reasonValue(),
		"detail", string(rej.detail),
	)
	if err := writeRejected(w, requestID, rej.reason, rej.reason.forceClose()); err != nil {
		s.logger.Error("response write failed", "requestId", requestID, "error", err)
	}
}

// checkHeaders runs the checks that need no body: the request target, then the
// method. It returns nil when both pass. The Host, Origin, Content-Type, and
// Authorization headers are not checked: any process that can connect to the
// socket can set them to anything, so checking them excludes no one.
func (s *Server) checkHeaders(r *http.Request) *rejection {
	if r.RequestURI != ArticlesPath {
		return &rejection{reason: reasonNotFound, detail: detailNotFound}
	}
	if r.Method != http.MethodPost {
		return &rejection{reason: reasonMethodNotAllowed, detail: detailMethodNotAllowed}
	}
	return nil
}
