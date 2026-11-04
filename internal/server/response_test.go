//go:build test

package server

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRejectReasonZeroValueIs500 checks that a reason that was never set is
// answered with 500 and internal_error, never with 202 or a 4xx that names a
// wrong cause.
func TestRejectReasonZeroValueIs500(t *testing.T) {
	for _, reason := range []rejectReason{reasonInternal, rejectReason(999)} {
		rec := httptest.NewRecorder()
		if err := writeRejected(rec, "0123456789abcdef", reason, false); err != nil {
			t.Fatalf("writeRejected: %v", err)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("reason %d status = %d, want 500", reason, rec.Code)
		}
		if body := decodeBody(t, rec); body.Reason != "internal_error" {
			t.Errorf("reason %d body.reason = %q, want internal_error", reason, body.Reason)
		}
	}
}

// rejectRequest builds a generation start with the given body.
func rejectRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	return newRequest(http.MethodPost, JobsPath, strings.NewReader(body))
}

// TestRejectReasonSet checks that the server answers with exactly these ten
// rejection reasons and statuses: the reasons of the header checks it no
// longer runs (the Host, the content type, the origin, and the token) are gone.
func TestRejectReasonSet(t *testing.T) {
	want := map[RejectReasonForTest]struct{}{
		{Value: "internal_error", Status: http.StatusInternalServerError}:        {},
		{Value: "not_found", Status: http.StatusNotFound}:                        {},
		{Value: "method_not_allowed", Status: http.StatusMethodNotAllowed}:       {},
		{Value: "too_many_requests", Status: http.StatusTooManyRequests}:         {},
		{Value: "body_too_large", Status: http.StatusRequestEntityTooLarge}:      {},
		{Value: "invalid_body", Status: http.StatusBadRequest}:                   {},
		{Value: "invalid_video_url", Status: http.StatusBadRequest}:              {},
		{Value: "invalid_transcript", Status: http.StatusBadRequest}:             {},
		{Value: "webhook_not_configured", Status: http.StatusServiceUnavailable}: {},
		{Value: "invalid_key", Status: http.StatusBadRequest}:                    {},
		{Value: "article_not_ready", Status: http.StatusConflict}:                {},
		{Value: "article_not_found", Status: http.StatusNotFound}:                {},
		{Value: "article_not_postable", Status: http.StatusConflict}:             {},
	}
	got := map[RejectReasonForTest]struct{}{}
	for _, reason := range RejectReasonsForTest() {
		if _, dup := got[reason]; dup {
			t.Errorf("reason %+v appears twice", reason)
		}
		got[reason] = struct{}{}
	}
	if !maps.Equal(got, want) {
		t.Errorf("rejection reasons = %v, want %v", got, want)
	}
}
