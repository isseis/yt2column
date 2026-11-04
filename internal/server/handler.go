package server

import (
	"net/http"
	"strings"
)

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
	routeStart
	routeStatus
	routePublish
)

// matchTarget returns the operation target names, and for a status request or
// a publish operation the key part of the target. It matches the request
// target as received, before any decoding, so an escaped character in the key
// part stays escaped and can never become a separator: a target with a query
// is not found, /jobs matches exactly, and the rest of a target
// under /jobs/ is cut at its first slash: with no slash it is a status
// request's key part, and when exactly "publish" follows the slash, what comes
// before it is a publish operation's key part. The key part is not checked
// here; any string, the empty one included, is the key check's to reject.
func matchTarget(target string) (route, string) {
	if strings.Contains(target, "?") {
		return routeNotFound, ""
	}
	if target == JobsPath {
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
// and the log, and hands the request to the handler of the operation its
// target names; every other target is not_found, refused before its body is
// read or a slot is taken. Each operation handler recovers its own panics, so
// one request cannot take down the process.
func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	requestID, err := newRequestID()
	if err != nil {
		s.logger.Error("request ID generation failed", "error", err)
		_ = writeRejected(w, "", reasonInternal, true)
		return
	}
	route, keyPart := matchTarget(r.RequestURI)
	switch route {
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
