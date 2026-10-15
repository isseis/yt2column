package server

import (
	"errors"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/failure"
)

// ArticlesPath is the request target of the one endpoint. The handler accepts
// exactly this string, before any decoding, and no query.
const ArticlesPath = "/articles"

// The other fixed request values the handler checks: the only accepted content
// type and method, the extension's origin, and the bearer scheme. The shared
// token is 64 characters, which is a public part of the protocol.
const (
	extensionOrigin   = "chrome-extension://clfmbbcdpnjcefbdihdoahomaifbabkk"
	jsonContentType   = "application/json"
	bearerPrefix      = "Bearer "
	sharedTokenLength = 64
)

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
		s.reject(w, r, requestID, *rej)
		responded = true
		return
	}

	if !s.acquireSlot() {
		s.reject(w, r, requestID, rejection{reason: reasonTooManyRequests, detail: detailTooManyRequests})
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
		s.reject(w, r, requestID, *rej)
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
	s.logger.Info("request accepted", "requestId", requestID, "remoteAddr", r.RemoteAddr, "videoId", input.videoID)
}

// reject logs the rejection and writes its response.
func (s *Server) reject(w http.ResponseWriter, r *http.Request, requestID string, rej rejection) {
	s.logger.Warn("request rejected",
		"requestId", requestID,
		"remoteAddr", r.RemoteAddr,
		"status", rej.reason.statusCode(),
		"reason", rej.reason.reasonValue(),
		"detail", string(rej.detail),
	)
	if err := writeRejected(w, requestID, rej.reason, rej.reason.forceClose()); err != nil {
		s.logger.Error("response write failed", "requestId", requestID, "error", err)
	}
}

// checkHeaders runs the checks that need no body, in the order that puts the
// cheapest and least trusted first: the Host (before the path, so a
// DNS-rebound page cannot probe paths), the request target, the method, the
// content type, the origin, and the shared token. It returns nil when every
// check passes.
func (s *Server) checkHeaders(r *http.Request) *rejection {
	if !s.hostAllowed(r.Host) {
		return &rejection{reason: reasonForbiddenHost, detail: detailHostMismatch}
	}
	if r.RequestURI != ArticlesPath {
		return &rejection{reason: reasonNotFound, detail: detailNotFound}
	}
	if r.Method != http.MethodPost {
		return &rejection{reason: reasonMethodNotAllowed, detail: detailMethodNotAllowed}
	}
	switch values := r.Header.Values("Content-Type"); {
	case len(values) == 0:
		return &rejection{reason: reasonUnsupportedMediaType, detail: detailContentTypeMissing}
	case len(values) > 1:
		return &rejection{reason: reasonUnsupportedMediaType, detail: detailContentTypeMultiple}
	case values[0] != jsonContentType:
		return &rejection{reason: reasonUnsupportedMediaType, detail: detailContentTypeMismatch}
	}
	switch values := r.Header.Values("Origin"); {
	case len(values) == 0:
		return &rejection{reason: reasonForbiddenOrigin, detail: detailOriginMissing}
	case len(values) > 1:
		return &rejection{reason: reasonForbiddenOrigin, detail: detailOriginMultiple}
	case !strings.HasPrefix(values[0], "chrome-extension://"):
		return &rejection{reason: reasonForbiddenOrigin, detail: detailOriginNotExtension}
	case values[0] != extensionOrigin:
		return &rejection{reason: reasonForbiddenOrigin, detail: detailOriginMismatch}
	}
	return s.checkAuthorization(r)
}

// checkAuthorization rejects a missing, repeated, or malformed Authorization
// header, then compares the presented token in constant time. The length check
// only chooses the log detail; the token's length is public.
func (s *Server) checkAuthorization(r *http.Request) *rejection {
	values := r.Header.Values("Authorization")
	switch {
	case len(values) == 0:
		return &rejection{reason: reasonUnauthorized, detail: detailAuthMissing}
	case len(values) > 1:
		return &rejection{reason: reasonUnauthorized, detail: detailAuthMultiple}
	case !strings.HasPrefix(values[0], bearerPrefix):
		return &rejection{reason: reasonUnauthorized, detail: detailAuthNotBearer}
	}
	presented := strings.TrimPrefix(values[0], bearerPrefix)
	if len(presented) != sharedTokenLength {
		return &rejection{reason: reasonUnauthorized, detail: detailAuthWrongLength}
	}
	if !s.token.Matches(presented) {
		return &rejection{reason: reasonUnauthorized, detail: detailAuthMismatch}
	}
	return nil
}

// hostAllowed reports whether host is exactly one of the loopback hosts with
// the server's port. Case and the port are significant, and a missing port is
// not accepted.
func (s *Server) hostAllowed(host string) bool {
	port := ":" + strconv.Itoa(int(s.port))
	for _, name := range []string{"127.0.0.1", "[::1]", "localhost"} {
		if host == name+port {
			return true
		}
	}
	return false
}
