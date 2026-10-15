//go:build test

package server

import (
	"net/http"
	"strings"
	"testing"
)

// validHTTPRequest builds a request that passes every header check.
func validHTTPRequest(t *testing.T) *http.Request {
	t.Helper()
	req := newRequest(http.MethodPost, ArticlesPath, strings.NewReader(validBody(testValidVideoURL, testValidRaw)))
	withValidHeaders(req, testTokenValue)
	return req
}

// TestRejectHeaders checks every header sub-case: the status and reason, and
// that no side effect (no writer, publisher, or notifier call) happens.
func TestRejectHeaders(t *testing.T) {
	tests := []struct {
		name   string
		build  func(t *testing.T) *http.Request
		status int
		reason string
	}{
		{"host_uppercase", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Host = "LOCALHOST:8765"
			return req
		}, http.StatusForbidden, "forbidden_host"},
		{"host_no_port", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Host = "127.0.0.1"
			return req
		}, http.StatusForbidden, "forbidden_host"},
		{"host_not_loopback", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Host = "example.com:8765"
			return req
		}, http.StatusForbidden, "forbidden_host"},
		{"path_other", func(_ *testing.T) *http.Request {
			req := newRequest(http.MethodPost, "/other", strings.NewReader("{}"))
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusNotFound, "not_found"},
		{"path_encoded", func(_ *testing.T) *http.Request {
			req := newRequest(http.MethodPost, "/%61rticles", strings.NewReader("{}"))
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusNotFound, "not_found"},
		{"method_get", func(_ *testing.T) *http.Request {
			req := newRequest(http.MethodGet, ArticlesPath, nil)
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusMethodNotAllowed, "method_not_allowed"},
		{"options_star", func(_ *testing.T) *http.Request {
			req := newRequest(http.MethodOptions, "*", nil)
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusNotFound, "not_found"},
		{"content_type_missing", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Del("Content-Type")
			return req
		}, http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"content_type_multiple", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Add("Content-Type", jsonContentType)
			return req
		}, http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"content_type_mismatch", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Content-Type", "text/plain")
			return req
		}, http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"origin_missing", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Del("Origin")
			return req
		}, http.StatusForbidden, "forbidden_origin"},
		{"origin_multiple", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Add("Origin", extensionOrigin)
			return req
		}, http.StatusForbidden, "forbidden_origin"},
		{"origin_not_extension", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Origin", "http://localhost")
			return req
		}, http.StatusForbidden, "forbidden_origin"},
		{"origin_mismatch", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Origin", "chrome-extension://other")
			return req
		}, http.StatusForbidden, "forbidden_origin"},
		{"auth_missing", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Del("Authorization")
			return req
		}, http.StatusUnauthorized, "unauthorized"},
		{"auth_multiple", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Add("Authorization", bearerPrefix+testTokenValue)
			return req
		}, http.StatusUnauthorized, "unauthorized"},
		{"auth_not_bearer", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Authorization", "Token "+testTokenValue)
			return req
		}, http.StatusUnauthorized, "unauthorized"},
		{"auth_wrong_length", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Authorization", bearerPrefix+"abc")
			return req
		}, http.StatusUnauthorized, "unauthorized"},
		{"auth_mismatch", func(t *testing.T) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Authorization", bearerPrefix+testOtherTokenValue)
			return req
		}, http.StatusUnauthorized, "unauthorized"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.serve(tc.build(t))
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d (body %q)", rec.Code, tc.status, rec.Body.String())
			}
			if body := decodeBody(t, rec); body.Status != "rejected" || body.Reason != tc.reason {
				t.Errorf("body = %+v, want rejected %s", body, tc.reason)
			}
			if len(ts.writer.Calls) != 0 || len(ts.publisher.Calls) != 0 || len(ts.notifier.Calls) != 0 {
				t.Errorf("a rejected request caused a side effect: writer=%d publisher=%d notifier=%d",
					len(ts.writer.Calls), len(ts.publisher.Calls), len(ts.notifier.Calls))
			}
			if len(ts.slots) != 0 {
				t.Errorf("slots held = %d, want 0", len(ts.slots))
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
		www    bool
		close  bool
	}{
		{"forbidden_host", func(t *testing.T, _ *testServer) *http.Request {
			req := validHTTPRequest(t)
			req.Host = "example.com:8765"
			return req
		}, http.StatusForbidden, "forbidden_host", false, false, true},
		{"not_found", func(_ *testing.T, _ *testServer) *http.Request {
			req := newRequest(http.MethodPost, "/nope", strings.NewReader("{}"))
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusNotFound, "not_found", false, false, true},
		{"method_not_allowed", func(_ *testing.T, _ *testServer) *http.Request {
			req := newRequest(http.MethodGet, ArticlesPath, nil)
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusMethodNotAllowed, "method_not_allowed", true, false, true},
		{"unsupported_media_type", func(t *testing.T, _ *testServer) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Content-Type", "text/plain")
			return req
		}, http.StatusUnsupportedMediaType, "unsupported_media_type", false, false, true},
		{"forbidden_origin", func(t *testing.T, _ *testServer) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Origin", "chrome-extension://other")
			return req
		}, http.StatusForbidden, "forbidden_origin", false, false, true},
		{"unauthorized", func(t *testing.T, _ *testServer) *http.Request {
			req := validHTTPRequest(t)
			req.Header.Set("Authorization", bearerPrefix+testOtherTokenValue)
			return req
		}, http.StatusUnauthorized, "unauthorized", false, true, true},
		{"too_many_requests", func(t *testing.T, ts *testServer) *http.Request {
			ts.slots <- struct{}{}
			ts.slots <- struct{}{}
			return validHTTPRequest(t)
		}, http.StatusTooManyRequests, "too_many_requests", false, false, true},
		{"body_too_large", func(t *testing.T, _ *testServer) *http.Request {
			req := validHTTPRequest(t)
			req.ContentLength = maxBodyBytes + 1
			return req
		}, http.StatusRequestEntityTooLarge, "body_too_large", false, false, true},
		{"invalid_body", func(_ *testing.T, _ *testServer) *http.Request {
			req := newRequest(http.MethodPost, ArticlesPath, strings.NewReader(`{"videoUrl":1}`))
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusBadRequest, "invalid_body", false, false, false},
		{"invalid_video_url", func(t *testing.T, _ *testServer) *http.Request {
			return rejectRequest(t, validBody("http://example.com/video", testValidRaw))
		}, http.StatusBadRequest, "invalid_video_url", false, false, false},
		{"invalid_transcript", func(t *testing.T, _ *testServer) *http.Request {
			return rejectRequest(t, validBody(testValidVideoURL, "not a transcript"))
		}, http.StatusBadRequest, "invalid_transcript", false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
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
			if got := rec.Header().Get("WWW-Authenticate"); (got == "Bearer") != tc.www {
				t.Errorf("WWW-Authenticate = %q, want %v", got, tc.www)
			}
			if got := rec.Header().Get("Connection"); (got == "close") != tc.close {
				t.Errorf("Connection = %q, want close=%v", got, tc.close)
			}
		})
	}
}
