//go:build test

package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/articlecache"
)

// validHTTPRequest builds a generation start that passes the target and
// method checks.
func validHTTPRequest(t *testing.T) *http.Request {
	t.Helper()
	return newRequest(http.MethodPost, JobsPath, strings.NewReader(validBody(testValidVideoURL, testValidRaw)))
}

// TestRejectHeaders checks every target and method rejection: the status and
// reason, and that no side effect (no writer or publisher call, no record) and
// no slot follows.
func TestRejectHeaders(t *testing.T) {
	tests := []struct {
		name   string
		req    *http.Request
		status int
		reason string
	}{
		{"path_encoded", newRequest(http.MethodPost, "/%6Aobs", strings.NewReader("{}")), http.StatusNotFound, "not_found"},
		{"method_get", newRequest(http.MethodGet, JobsPath, nil), http.StatusMethodNotAllowed, "method_not_allowed"},
		{"options_star", newRequest(http.MethodOptions, "*", nil), http.StatusNotFound, "not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.serve(tc.req)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d (body %q)", rec.Code, tc.status, rec.Body.String())
			}
			if body := decodeBody(t, rec); body.Status != "rejected" || body.Reason != tc.reason {
				t.Errorf("body = %+v, want rejected %s", body, tc.reason)
			}
			if len(ts.writer.Calls) != 0 || len(ts.publisher.PostCalls) != 0 {
				t.Errorf("a rejected request caused a side effect: writer=%d post=%d",
					len(ts.writer.Calls), len(ts.publisher.PostCalls))
			}
			if dump := ts.cache.DumpRecordsForTest(); dump != "" {
				t.Errorf("a rejected request added a record:\n%s", dump)
			}
			if len(ts.slots) != 0 || len(ts.reading) != 0 {
				t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
			}
		})
	}
}

// TestAcceptsWithoutBrowserHeaders checks that a generation start with any
// Host and no Origin, Content-Type, or Authorization header is accepted and
// runs its generation:
// the server no longer checks the headers a browser sets, because any process
// that reaches the socket can set them to anything.
func TestAcceptsWithoutBrowserHeaders(t *testing.T) {
	for _, host := range []string{"yt2column", "example.com:8765", "LOCALHOST", ""} {
		t.Run("host "+host, func(t *testing.T) {
			ts := newTestServer(t)
			req := validHTTPRequest(t)
			req.Host = host
			for _, name := range []string{"Origin", "Content-Type", "Authorization"} {
				if req.Header.Get(name) != "" {
					t.Fatalf("the request carries %s, want no such header", name)
				}
			}
			rec := ts.serve(req)
			ts.waitJobs()
			if rec.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202 (body %q)", rec.Code, rec.Body.String())
			}
			if len(ts.writer.Calls) != 1 {
				t.Errorf("writer calls = %d, want 1", len(ts.writer.Calls))
			}
		})
	}
}

// TestRejectStatusCodes checks the status code, reason value, and extra headers
// of every rejection reason, driven through the handler.
func TestRejectStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		build  func(t *testing.T, ts *testServer) *http.Request
		status int
		reason string
		allow  bool
		close  bool
		// noWebhook builds the server without SLACK_WEBHOOK_URL, as run does.
		noWebhook bool
	}{
		{"not_found", func(_ *testing.T, _ *testServer) *http.Request {
			return newRequest(http.MethodPost, "/nope", strings.NewReader("{}"))
		}, http.StatusNotFound, "not_found", false, true, false},
		{"method_not_allowed", func(_ *testing.T, _ *testServer) *http.Request {
			return newRequest(http.MethodGet, JobsPath, nil)
		}, http.StatusMethodNotAllowed, "method_not_allowed", true, true, false},
		{"too_many_requests", func(t *testing.T, ts *testServer) *http.Request {
			ts.slots <- struct{}{}
			ts.slots <- struct{}{}
			return validHTTPRequest(t)
		}, http.StatusTooManyRequests, "too_many_requests", false, true, false},
		{"body_too_large", func(t *testing.T, _ *testServer) *http.Request {
			req := validHTTPRequest(t)
			req.ContentLength = maxBodyBytes + 1
			return req
		}, http.StatusRequestEntityTooLarge, "body_too_large", false, true, false},
		{"invalid_body", func(_ *testing.T, _ *testServer) *http.Request {
			return newRequest(http.MethodPost, JobsPath, strings.NewReader(`{"videoUrl":1,"transcript":"0:00\nHello\n","mode":"plain"}`))
		}, http.StatusBadRequest, "invalid_body", false, false, false},
		{"invalid_video_url", func(t *testing.T, _ *testServer) *http.Request {
			return rejectRequest(t, validBody("http://example.com/video", testValidRaw))
		}, http.StatusBadRequest, "invalid_video_url", false, false, false},
		{"invalid_transcript", func(t *testing.T, _ *testServer) *http.Request {
			return rejectRequest(t, validBody(testValidVideoURL, "not a transcript"))
		}, http.StatusBadRequest, "invalid_transcript", false, false, false},
		{"webhook_not_configured", func(t *testing.T, ts *testServer) *http.Request {
			return newRequest(http.MethodPost, publishTarget(publishTestKey(t, ts).String()), nil)
		}, http.StatusServiceUnavailable, "webhook_not_configured", false, true, true},
		{"invalid_key", func(_ *testing.T, _ *testServer) *http.Request {
			return newRequest(http.MethodGet, JobsPath+"/not-a-key", nil)
		}, http.StatusBadRequest, "invalid_key", false, false, false},
		{"article_not_ready", func(t *testing.T, ts *testServer) *http.Request {
			ts.cache.Begin(publishTestSubject(t, ts), articlecache.StartPlain, func() bool { return true })
			return newRequest(http.MethodPost, publishTarget(publishTestKey(t, ts).String()), nil)
		}, http.StatusConflict, "article_not_ready", false, false, false},
		{"article_not_found", func(t *testing.T, ts *testServer) *http.Request {
			return newRequest(http.MethodPost, publishTarget(publishTestKey(t, ts).String()), nil)
		}, http.StatusNotFound, "article_not_found", false, false, false},
		{"article_not_postable", func(t *testing.T, ts *testServer) *http.Request {
			run := ts.cache.Begin(publishTestSubject(t, ts), articlecache.StartPlain, func() bool { return true }).Run
			run.CompleteUnpostable(testArticle(), articlecache.UnpostableMention)
			return newRequest(http.MethodPost, publishTarget(publishTestKey(t, ts).String()), nil)
		}, http.StatusConflict, "article_not_postable", false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
			if tc.noWebhook {
				ts = newJobsTestServer(t, jobsServerOptions{noWebhook: true})
			}
			rec := ts.serve(tc.build(t, ts))
			ts.waitJobs()
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d (body %q)", rec.Code, tc.status, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body.Status != "rejected" || body.Reason != tc.reason {
				t.Errorf("body = %+v, want rejected %s", body, tc.reason)
			}
			if got := rec.Header().Get("Allow"); (got == http.MethodPost) != tc.allow {
				t.Errorf("Allow = %q, want %v", got, tc.allow)
			}
			if got := rec.Header().Get("Connection"); (got == "close") != tc.close {
				t.Errorf("Connection = %q, want close=%v", got, tc.close)
			}
		})
	}
}
