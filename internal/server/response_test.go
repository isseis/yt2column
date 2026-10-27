//go:build test

package server

import (
	"encoding/json"
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

// TestResponseOmitsInputs checks that no response body carries a request value
// or a secret, and that the logs do not either.
func TestResponseOmitsInputs(t *testing.T) {
	ts := newTestServer(t)
	// The transcript and metadata use the non-secret transcript marker, so a
	// leaked value would still be visible in the log. channelName uses the
	// secret marker, to prove the redacting writer is a backstop.
	body, err := json.Marshal(map[string]string{
		"videoUrl":    testValidVideoURL,
		"transcript":  "0:00\n" + testTranscriptMarker + "\n",
		"title":       testTranscriptMarker,
		"channelName": testMarker,
		"description": testTranscriptMarker,
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	accepted := ts.serveAccepted(t, string(body))
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %q)", accepted.Code, accepted.Body.String())
	}
	for _, marker := range []string{testTranscriptMarker, testMarker} {
		if strings.Contains(accepted.Body.String(), marker) {
			t.Errorf("accepted response body %q carries a request value", accepted.Body.String())
		}
	}
	if got := decodeBody(t, accepted); got.Status != "accepted" || got.Reason != "" {
		t.Errorf("accepted body = %+v", got)
	}

	// A rejection that read the body must also omit the values.
	rejected := ts.serve(rejectRequest(t, `{"videoUrl":"`+testMarker+`","transcript":"x","extra":1}`))
	ts.waitJobs()
	if strings.Contains(rejected.Body.String(), testMarker) {
		t.Errorf("rejected response body %q carries a request value", rejected.Body.String())
	}
	logs := ts.logs.String()
	if strings.Contains(logs, testTranscriptMarker) {
		t.Errorf("logs carry transcript or metadata text:\n%s", logs)
	}
	if strings.Contains(logs, testMarker) {
		t.Errorf("logs carry a secret:\n%s", logs)
	}
}

// rejectRequest builds a request to the endpoint with the given body.
func rejectRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	return newRequest(http.MethodPost, ArticlesPath, strings.NewReader(body))
}

// TestRejectReasonSet checks that the server answers with exactly these nine
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
