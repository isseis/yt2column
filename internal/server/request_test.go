//go:build test

package server

import (
	"bytes"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// requestIDShape is the request ID form the server generates.
var requestIDShape = regexp.MustCompile(`^[0-9a-f]{16}$`)

// TestAcceptValidRequest checks that a valid request is accepted with 202 and
// the accepted response shape, and that the job runs.
func TestAcceptValidRequest(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.serveAccepted(t, validBody(testValidVideoURL, testValidRaw))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %q)", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body.Status != "accepted" || body.Reason != "" {
		t.Errorf("body = %+v, want accepted with no reason", body)
	}
	if !requestIDShape.MatchString(body.RequestID) {
		t.Errorf("requestId = %q, want 16 lowercase hexadecimal characters", body.RequestID)
	}
	if len(ts.publisher.Calls) != 1 {
		t.Errorf("publisher calls = %d, want 1", len(ts.publisher.Calls))
	}
}

// TestRejectBodyTable checks the body-shape rejections that map to
// invalid_body.
func TestRejectBodyTable(t *testing.T) {
	valid := `"` + testValidVideoURL + `"`
	tests := []struct {
		name string
		body []byte
	}{
		{"missing_video_url", []byte(`{"transcript":"0:00\nHello\n"}`)},
		{"missing_transcript", []byte(`{"videoUrl":` + valid + `}`)},
		{"video_url_number", []byte(`{"videoUrl":1,"transcript":"0:00\nHello\n"}`)},
		{"video_url_null", []byte(`{"videoUrl":null,"transcript":"0:00\nHello\n"}`)},
		{"video_url_empty", []byte(`{"videoUrl":"","transcript":"0:00\nHello\n"}`)},
		{"video_url_whitespace", []byte(`{"videoUrl":"   ","transcript":"0:00\nHello\n"}`)},
		{"transcript_number", []byte(`{"videoUrl":` + valid + `,"transcript":2}`)},
		{"transcript_empty", []byte(`{"videoUrl":` + valid + `,"transcript":""}`)},
		{"transcript_whitespace", []byte(`{"videoUrl":` + valid + `,"transcript":" \t "}`)},
		{"duplicate_video_url", []byte(`{"videoUrl":` + valid + `,"videoUrl":"x","transcript":"0:00\nHello\n"}`)},
		{"trailing_data", []byte(`{"videoUrl":` + valid + `,"transcript":"0:00\nHello\n"}{}`)},
		{"not_object", []byte(`[]`)},
		{"not_json", []byte(`not json`)},
		{"invalid_utf8", []byte("{\"videoUrl\":" + valid + ",\"transcript\":\"\xff\"}")},
		{"unpaired_surrogate", []byte(`{"videoUrl":` + valid + `,"transcript":"\ud800"}`)},
		{"unpaired_surrogate_undeclared", []byte(`{"videoUrl":` + valid + `,"transcript":"0:00\nHello\n","extra":"\ud800"}`)},
		{"invalid_utf8_undeclared", []byte("{\"videoUrl\":" + valid + ",\"transcript\":\"0:00\\nHello\\n\",\"extra\":\"\xff\"}")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
			req := newRequest(http.MethodPost, articlesPath, bytes.NewReader(tc.body))
			withValidHeaders(req, testTokenValue)
			rec := ts.serve(req)
			ts.waitJobs()
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
			}
			if body := decodeBody(t, rec); body.Reason != "invalid_body" {
				t.Errorf("reason = %q, want invalid_body", body.Reason)
			}
			if len(ts.writer.Calls) != 0 {
				t.Errorf("writer was called for a rejected body")
			}
		})
	}
}

// TestOptionalMembers checks that the optional metadata is accepted when
// missing or empty and rejected when it is not a string.
func TestOptionalMembers(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		ts := newTestServer(t)
		rec := ts.serveAccepted(t, validBody(testValidVideoURL, testValidRaw))
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", rec.Code)
		}
		got := ts.writer.Calls[0].Transcript
		if got.Title != "" || got.ChannelName != "" || got.Description != "" {
			t.Errorf("optional fields = %q/%q/%q, want empty", got.Title, got.ChannelName, got.Description)
		}
	})
	t.Run("empty", func(t *testing.T) {
		ts := newTestServer(t)
		body := `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","title":"","channelName":"","description":""}`
		rec := ts.serveAccepted(t, body)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", rec.Code)
		}
		got := ts.writer.Calls[0].Transcript
		if got.Title != "" || got.ChannelName != "" || got.Description != "" {
			t.Errorf("optional fields = %q/%q/%q, want empty", got.Title, got.ChannelName, got.Description)
		}
	})
	for _, member := range []string{"title", "channelName", "description"} {
		for _, value := range []string{"1", "null", "true"} {
			t.Run(member+"_"+value, func(t *testing.T) {
				ts := newTestServer(t)
				body := `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","` + member + `":` + value + `}`
				rec := ts.serve(ts.request(t, body))
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400", rec.Code)
				}
				if b := decodeBody(t, rec); b.Reason != "invalid_body" {
					t.Errorf("reason = %q, want invalid_body", b.Reason)
				}
			})
		}
	}
}

// request builds a valid-header request with body.
func (ts *testServer) request(t *testing.T, body string) *http.Request {
	t.Helper()
	req := newRequest(http.MethodPost, articlesPath, strings.NewReader(body))
	withValidHeaders(req, ts.token)
	return req
}

// TestUnknownMemberIgnored checks that an undeclared member is accepted and its
// value ignored.
func TestUnknownMemberIgnored(t *testing.T) {
	ts := newTestServer(t)
	body := `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","future":{"nested":[1,2,3]}}`
	rec := ts.serveAccepted(t, body)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %q)", rec.Code, rec.Body.String())
	}
}

// TestRejectVideoURLTable checks the video URLs the validator rejects.
func TestRejectVideoURLTable(t *testing.T) {
	urls := []string{
		"http://example.com/watch?v=dQw4w9WgXcQ",
		"ftp://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/playlist?list=x",
		"https://www.youtube.com/watch",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&v=abcdefghijk",
		"https://user:pass@www.youtube.com/watch?v=dQw4w9WgXcQ",
		" https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=short",
		"https://youtu.be/dQw4w9WgXcQ/extra",
	}
	for _, raw := range urls {
		t.Run(raw, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.serve(ts.request(t, validBody(raw, testValidRaw)))
			ts.waitJobs()
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
			}
			if body := decodeBody(t, rec); body.Reason != "invalid_video_url" {
				t.Errorf("reason = %q, want invalid_video_url", body.Reason)
			}
		})
	}
}

// TestRejectInvalidTranscript checks that a transcript that does not match the
// copied format is rejected as invalid_transcript.
func TestRejectInvalidTranscript(t *testing.T) {
	raws := []string{
		"not a transcript",
		"0:00\n", // a timestamp with no text
		"0:24MARKER_BODY",
	}
	for _, raw := range raws {
		t.Run(raw, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.serve(ts.request(t, validBody(testValidVideoURL, raw)))
			ts.waitJobs()
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
			}
			if body := decodeBody(t, rec); body.Reason != "invalid_transcript" {
				t.Errorf("reason = %q, want invalid_transcript", body.Reason)
			}
		})
	}
}

// TestHeaderRejectionReadsNoBody checks that a request rejected on its headers
// (including an oversized body with a wrong token) has zero bytes of its body
// read.
func TestHeaderRejectionReadsNoBody(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, body io.Reader) *http.Request
		code  int
	}{
		{"host", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPost, articlesPath, body)
			withValidHeaders(req, testTokenValue)
			req.Host = "example.com:8765"
			return req
		}, http.StatusForbidden},
		{"path", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPost, "/nope", body)
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusNotFound},
		{"method", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodGet, articlesPath, body)
			withValidHeaders(req, testTokenValue)
			return req
		}, http.StatusMethodNotAllowed},
		{"content_type", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPost, articlesPath, body)
			withValidHeaders(req, testTokenValue)
			req.Header.Set("Content-Type", "text/plain")
			return req
		}, http.StatusUnsupportedMediaType},
		{"origin", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPost, articlesPath, body)
			withValidHeaders(req, testTokenValue)
			req.Header.Set("Origin", "chrome-extension://other")
			return req
		}, http.StatusForbidden},
		{"token", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPost, articlesPath, body)
			withValidHeaders(req, testOtherTokenValue)
			return req
		}, http.StatusUnauthorized},
		{"oversized_body_wrong_token", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPost, articlesPath, body)
			withValidHeaders(req, testOtherTokenValue)
			req.ContentLength = maxBodyBytes * 4
			return req
		}, http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
			counter := &recordingReader{r: strings.NewReader(strings.Repeat("x", 1024))}
			rec := ts.serve(tc.build(t, io.NopCloser(counter)))
			ts.waitJobs()
			if rec.Code != tc.code {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.code, rec.Body.String())
			}
			if counter.n != 0 {
				t.Errorf("read %d bytes of the body, want 0", counter.n)
			}
		})
	}
}

// TestBodyTooLarge checks that an oversized body is rejected with 413, and that
// no more than the limit plus one byte is read.
func TestBodyTooLarge(t *testing.T) {
	t.Run("content_length", func(t *testing.T) {
		ts := newTestServer(t)
		counter := &recordingReader{r: bytes.NewReader([]byte("{}"))}
		req := newRequest(http.MethodPost, articlesPath, io.NopCloser(counter))
		withValidHeaders(req, testTokenValue)
		req.ContentLength = maxBodyBytes + 1
		rec := ts.serve(req)
		ts.waitJobs()
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413 (body %q)", rec.Code, rec.Body.String())
		}
		if counter.n != 0 {
			t.Errorf("read %d bytes of a body refused on its length, want 0", counter.n)
		}
	})
	t.Run("no_content_length", func(t *testing.T) {
		ts := newTestServer(t)
		counter := &recordingReader{r: strings.NewReader(strings.Repeat("x", maxBodyBytes+10))}
		req := newRequest(http.MethodPost, articlesPath, io.NopCloser(counter))
		withValidHeaders(req, testTokenValue)
		req.ContentLength = -1
		rec := ts.serve(req)
		ts.waitJobs()
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413 (body %q)", rec.Code, rec.Body.String())
		}
		if counter.n > maxBodyBytes+1 {
			t.Errorf("read %d bytes, want at most %d", counter.n, maxBodyBytes+1)
		}
	})
}
