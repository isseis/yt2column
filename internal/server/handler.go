package server

import (
	"errors"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/isseis/yt2column/internal/failure"
)

// ArticlesPath is the request target of the legacy endpoint. The handler
// accepts exactly this string, before any decoding, and no query.
const ArticlesPath = "/articles"

// JobsPath is the request target of a generation start. A status request
// appends a slash and the article key: JobsPath + "/" + key.
const JobsPath = "/jobs"

// PublishSuffix ends the request target of a publish operation, which follows
// the status request's target: JobsPath + "/" + key + PublishSuffix.
const PublishSuffix = "/publish"

// route is the operation a request target names. The zero value is
// routeNotFound: a target that matched nothing.
type route int

const (
	routeNotFound route = iota
	routeArticles
	routeStart
	routeStatus
	routePublish
)

// matchTarget returns the operation target names, and for a status request or
// a publish operation the key part of the target. It matches the request
// target as received, before any decoding, so an escaped character in the key
// part stays escaped and can never become a separator: a target with a query
// is not found, /articles and /jobs match exactly, and the rest of a target
// under /jobs/ is cut at its first slash: with no slash it is a status
// request's key part, and when exactly "publish" follows the slash, what comes
// before it is a publish operation's key part. The key part is not checked
// here; any string, the empty one included, is the key check's to reject.
func matchTarget(target string) (route, string) {
	if strings.Contains(target, "?") {
		return routeNotFound, ""
	}
	switch target {
	case ArticlesPath:
		return routeArticles, ""
	case JobsPath:
		return routeStart, ""
	}
	rest, ok := strings.CutPrefix(target, JobsPath+"/")
	if !ok {
		return routeNotFound, ""
	}
	keyPart, op, hasOp := strings.Cut(rest, "/")
	switch {
	case !hasOp:
		return routeStatus, keyPart
	case "/"+op == PublishSuffix:
		return routePublish, keyPart
	default:
		return routeNotFound, ""
	}
}

// handler gives every request a request ID, which is written to the response
// and the log (and, on the legacy endpoint, any failure notice), and hands the
// request to the handler of the operation its target names. Each of those
// recovers its own panics, so one request cannot take down the process.
func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	requestID, err := newRequestID()
	if err != nil {
		s.logger.Error("request ID generation failed", "error", err)
		_ = writeRejected(w, "", reasonInternal, true)
		return
	}
	route, keyPart := matchTarget(r.RequestURI)
	switch route {
	case routeArticles:
		s.articlesHandler(w, r, requestID)
	case routeStart:
		s.startHandler(w, r, requestID)
	case routeStatus:
		s.statusHandler(w, r, requestID, keyPart)
	case routePublish:
		s.publishHandler(w, r, requestID, keyPart)
	default:
		s.reject(w, requestID, rejection{reason: reasonNotFound, detail: detailNotFound})
	}
}

// articlesHandler serves the legacy endpoint: it validates a request in a
// fixed order and, on success, starts the job that generates and posts the
// article.
func (s *Server) articlesHandler(w http.ResponseWriter, r *http.Request, requestID string) {
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

	if rej := checkMethod(r, http.MethodPost); rej != nil {
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

// reject logs the rejection and writes its response. The connection is closed
// when the reason always closes it or when this rejection left the body
// unread.
func (s *Server) reject(w http.ResponseWriter, requestID string, rej rejection) {
	s.logger.Warn("request rejected",
		"requestId", requestID,
		"status", rej.reason.statusCode(),
		"reason", rej.reason.reasonValue(),
		"detail", string(rej.detail),
	)
	allow := rej.allow
	if allow == "" {
		allow = http.MethodPost
	}
	if err := writeRejectedAllow(w, requestID, rej.reason, rej.reason.forceClose() || rej.bodyUnread, allow); err != nil {
		s.logger.Error("response write failed", "requestId", requestID, "error", err)
	}
}

// checkMethod returns the method_not_allowed rejection, naming want in its
// Allow header, when the request's method is not want, and nil otherwise. The
// Host, Origin, Content-Type, and Authorization headers are not checked: any
// process that can connect to the socket can set them to anything, so
// checking them excludes no one.
func checkMethod(r *http.Request, want string) *rejection {
	if r.Method != want {
		return &rejection{reason: reasonMethodNotAllowed, detail: detailMethodNotAllowed, allow: want}
	}
	return nil
}
