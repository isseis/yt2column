//go:build test

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
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
			req := newRequest(http.MethodPost, ArticlesPath, bytes.NewReader(tc.body))
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
	req := newRequest(http.MethodPost, ArticlesPath, strings.NewReader(body))
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
			if len(ts.notifier.Calls) != 1 || ts.notifier.Calls[0].Notice.Kind != failure.KindInvalidVideoURL {
				t.Errorf("notifier calls = %+v, want one KindInvalidVideoURL", ts.notifier.Calls)
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
			if len(ts.notifier.Calls) != 1 || ts.notifier.Calls[0].Notice.Kind != failure.KindInvalidTranscript {
				t.Errorf("notifier calls = %+v, want one KindInvalidTranscript", ts.notifier.Calls)
			}
		})
	}
}

// TestHeaderRejectionReadsNoBody checks that a request rejected on its target
// or method (including one that declares an oversized body) has zero bytes of
// its body read.
func TestHeaderRejectionReadsNoBody(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, body io.Reader) *http.Request
		code  int
	}{
		{"path", func(_ *testing.T, body io.Reader) *http.Request {
			return newRequest(http.MethodPost, "/nope", body)
		}, http.StatusNotFound},
		{"method", func(_ *testing.T, body io.Reader) *http.Request {
			return newRequest(http.MethodGet, ArticlesPath, body)
		}, http.StatusMethodNotAllowed},
		{"oversized_body_wrong_method", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPut, ArticlesPath, body)
			req.ContentLength = maxBodyBytes * 4
			return req
		}, http.StatusMethodNotAllowed},
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
		req := newRequest(http.MethodPost, ArticlesPath, io.NopCloser(counter))
		req.ContentLength = maxBodyBytes + 1
		rec := ts.serve(req)
		ts.waitJobs()
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413 (body %q)", rec.Code, rec.Body.String())
		}
		if counter.n != 0 {
			t.Errorf("read %d bytes of a body refused on its length, want 0", counter.n)
		}
		if len(ts.notifier.Calls) != 1 || ts.notifier.Calls[0].Notice.Kind != failure.KindBodyTooLarge {
			t.Errorf("notifier calls = %+v, want one KindBodyTooLarge", ts.notifier.Calls)
		}
	})
	t.Run("no_content_length", func(t *testing.T) {
		ts := newTestServer(t)
		counter := &recordingReader{r: strings.NewReader(strings.Repeat("x", maxBodyBytes+10))}
		req := newRequest(http.MethodPost, ArticlesPath, io.NopCloser(counter))
		req.ContentLength = -1
		rec := ts.serve(req)
		ts.waitJobs()
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413 (body %q)", rec.Code, rec.Body.String())
		}
		if counter.n > maxBodyBytes+1 {
			t.Errorf("read %d bytes, want at most %d", counter.n, maxBodyBytes+1)
		}
		if len(ts.notifier.Calls) != 1 || ts.notifier.Calls[0].Notice.Kind != failure.KindBodyTooLarge {
			t.Errorf("notifier calls = %+v, want one KindBodyTooLarge", ts.notifier.Calls)
		}
	})
}

// TestChannelNameAgreesWithWriter feeds one table of channel names to the three
// places that check one (the request, the writer before the LLM call, and
// Article.CheckPublishable) and requires the same verdict from each. A request
// the server accepts must reach a writer and a publisher that accept it too.
func TestChannelNameAgreesWithWriter(t *testing.T) {
	for _, tc := range writertestutil.ChannelNameCases() {
		t.Run(tc.Name, func(t *testing.T) {
			ts := newTestServer(t)
			// The strict JSON layer rejects invalid UTF-8 for the whole body
			// before the channel name check runs, so the request leg proves
			// nothing about that check for such a value; the other legs
			// still apply.
			if utf8.ValidString(tc.Value) {
				quoted, err := json.Marshal(tc.Value)
				if err != nil {
					t.Fatalf("Marshal: %v", err)
				}
				body := `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","channelName":` + string(quoted) + `}`
				requireChannelNameRequestVerdict(t, ts, body, tc)
			}

			article := writer.Article{
				Title: "t", ChannelName: tc.Value, Body: "b", SourceURL: testValidVideoURL, Model: "m",
			}
			err := article.CheckPublishable()
			if tc.Valid != (err == nil) {
				t.Errorf("CheckPublishable() = %v, want valid = %v", err, tc.Valid)
			}
			if err != nil && (!errors.Is(err, writer.ErrInvalidArticle) || !errors.Is(err, writer.ErrInvalidChannelName)) {
				t.Errorf("CheckPublishable() = %v, want ErrInvalidArticle and ErrInvalidChannelName", err)
			}

			client := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: "# t\nb\n", Model: "m"}}
			w, err := writer.New(client, writer.Options{})
			if err != nil {
				t.Fatalf("writer.New: %v", err)
			}
			got, err := w.Write(context.Background(), transcript.Transcript{
				VideoID:     testVideoID,
				VideoURL:    testValidVideoURL,
				ChannelName: tc.Value,
				Segments:    []transcript.Segment{{Text: "Hello"}},
			})
			if tc.Valid != (err == nil) {
				t.Errorf("Write() = %v, want valid = %v", err, tc.Valid)
			}
			if err == nil && got.ChannelName != tc.Value {
				t.Errorf("Write() ChannelName = %q, want %q", got.ChannelName, tc.Value)
			}
			if err != nil && (!errors.Is(err, writer.ErrInvalidTranscript) || !errors.Is(err, writer.ErrInvalidChannelName)) {
				t.Errorf("Write() = %v, want ErrInvalidTranscript and ErrInvalidChannelName", err)
			}
			if !tc.Valid && len(client.Calls) != 0 {
				t.Errorf("LLM called %d times for a rejected channel name, want 0", len(client.Calls))
			}
		})
	}
}

// requireChannelNameRequestVerdict posts body and checks the request is
// accepted and its channel name reaches the writer, or is rejected as
// invalid_body without a writer call.
func requireChannelNameRequestVerdict(t *testing.T, ts *testServer, body string, tc writertestutil.ChannelNameCase) {
	t.Helper()
	rec := ts.serveAccepted(t, body)
	if tc.Valid {
		if rec.Code != http.StatusAccepted {
			t.Fatalf("request status = %d, want 202 (body %q)", rec.Code, rec.Body.String())
		}
		if got := ts.writer.Calls[0].Transcript.ChannelName; got != tc.Value {
			t.Errorf("channel name reaching the writer = %q, want %q", got, tc.Value)
		}
		return
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("request status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec); got.Reason != "invalid_body" {
		t.Errorf("request reason = %q, want invalid_body", got.Reason)
	}
	if len(ts.writer.Calls) != 0 {
		t.Errorf("writer called %d times for a rejected request, want 0", len(ts.writer.Calls))
	}
}
