package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
)

// requestIDBytes is the number of random bytes in a request ID.
const requestIDBytes = 8

// rejectReason is why a request was rejected. The zero value is reasonInternal:
// a reason that was never set is answered with 500, never with 202 or a 4xx
// that names a wrong cause.
type rejectReason int

const (
	reasonInternal rejectReason = iota
	reasonNotFound
	reasonMethodNotAllowed
	reasonTooManyRequests
	reasonBodyTooLarge
	reasonInvalidBody
	reasonInvalidVideoURL
	reasonInvalidTranscript
)

// reasonValue returns the fixed `reason` value of the response body. It is the
// English identifier the extension reads, so it never holds a request value.
func (r rejectReason) reasonValue() string {
	switch r {
	case reasonNotFound:
		return "not_found"
	case reasonMethodNotAllowed:
		return "method_not_allowed"
	case reasonTooManyRequests:
		return "too_many_requests"
	case reasonBodyTooLarge:
		return "body_too_large"
	case reasonInvalidBody:
		return "invalid_body"
	case reasonInvalidVideoURL:
		return "invalid_video_url"
	case reasonInvalidTranscript:
		return "invalid_transcript"
	default:
		return "internal_error"
	}
}

// statusCode returns the HTTP status code of the rejection.
func (r rejectReason) statusCode() int {
	switch r {
	case reasonNotFound:
		return http.StatusNotFound
	case reasonMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case reasonTooManyRequests:
		return http.StatusTooManyRequests
	case reasonBodyTooLarge:
		return http.StatusRequestEntityTooLarge
	case reasonInvalidBody, reasonInvalidVideoURL, reasonInvalidTranscript:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// notify reports whether a rejection with this reason posts a failure notice.
// Only a rejection that happens after the request has taken a slot is notified;
// the handler decides that from the slot it holds, so this only names the
// reasons that can be notified at all.
func (r rejectReason) notify() bool {
	switch r {
	case reasonBodyTooLarge, reasonInvalidBody, reasonInvalidVideoURL, reasonInvalidTranscript, reasonInternal:
		return true
	default:
		return false
	}
}

// forceClose reports whether the rejection adds `Connection: close`. Rejections
// that did not read the whole body (the header checks, the slot and size
// checks) close the connection, so net/http does not read and discard the body
// to reuse it. A pre-slot internal error (a request ID failure or a panic
// before the slot) also closes the connection, but the handler passes
// closeConnection directly for those cases rather than using this method; the
// post-slot internal error reads the body and does not force close.
func (r rejectReason) forceClose() bool {
	switch r {
	case reasonNotFound, reasonMethodNotAllowed, reasonTooManyRequests, reasonBodyTooLarge:
		return true
	default:
		return false
	}
}

// rejectDetail is the fixed value the rejection log carries to tell apart the
// sub-cases of a reason. It is never part of the response.
type rejectDetail string

const (
	detailNotFound          rejectDetail = "not_found"
	detailMethodNotAllowed  rejectDetail = "method_not_allowed"
	detailTooManyRequests   rejectDetail = "too_many_requests"
	detailBodyTooLarge      rejectDetail = "body_too_large"
	detailInvalidBody       rejectDetail = "invalid_body"
	detailInvalidVideoURL   rejectDetail = "invalid_video_url"
	detailInvalidTranscript rejectDetail = "invalid_transcript"
	detailInternalError     rejectDetail = "internal_error"
)

// rejection is a response the handler writes plus the detail it logs. videoID
// is set when a failure notice should name the video, which is only after the
// video URL was accepted. readFailed marks a body-read failure, which is
// answered with no response at all.
type rejection struct {
	reason     rejectReason
	detail     rejectDetail
	videoID    string
	readFailed bool
	err        error
}

// responseBody is the JSON object every response carries. It holds only fixed
// values and the request ID the server generated, so no request value and no
// secret can appear in it.
type responseBody struct {
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	RequestID string `json:"requestId"`
}

// newRequestID returns a fresh request ID: 8 crypto-random bytes as 16
// lowercase hexadecimal characters.
func newRequestID() (string, error) {
	var buf [requestIDBytes]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate a request ID: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

// writeAccepted writes the 202 response for an accepted request.
func writeAccepted(w http.ResponseWriter, requestID string) error {
	return writeJSON(w, http.StatusAccepted, responseBody{Status: "accepted", RequestID: requestID}, nil)
}

// writeRejected writes the rejection response. closeConnection adds
// `Connection: close` for a reason whose body was not fully read, and a
// pre-slot internal error; the caller decides it from the slot it holds.
func writeRejected(w http.ResponseWriter, requestID string, reason rejectReason, closeConnection bool) error {
	var headers [][2]string
	if reason == reasonMethodNotAllowed {
		headers = append(headers, [2]string{"Allow", http.MethodPost})
	}
	if closeConnection {
		headers = append(headers, [2]string{"Connection", "close"})
	}
	body := responseBody{Status: "rejected", Reason: reason.reasonValue(), RequestID: requestID}
	return writeJSON(w, reason.statusCode(), body, headers)
}

// writeJSON writes the status, the fixed response headers, the extra headers,
// and the marshalled body.
func writeJSON(w http.ResponseWriter, status int, body responseBody, headers [][2]string) error {
	header := w.Header()
	header.Set("Content-Type", "application/json; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	for _, kv := range headers {
		header.Add(kv[0], kv[1])
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal the response body: %w", err)
	}
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write the response body: %w", err)
	}
	return nil
}
