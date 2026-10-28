//go:build test

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/articlecache"
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

// invalidBodyCases are bodies the body checks reject as invalid_body. The
// legacy endpoint and the generation start share them.
var invalidBodyCases = []struct {
	name string
	body []byte
}{
	{"missing_video_url", []byte(`{"transcript":"0:00\nHello\n"}`)},
	{"missing_transcript", []byte(`{"videoUrl":"` + testValidVideoURL + `"}`)},
	{"video_url_number", []byte(`{"videoUrl":1,"transcript":"0:00\nHello\n"}`)},
	{"video_url_null", []byte(`{"videoUrl":null,"transcript":"0:00\nHello\n"}`)},
	{"video_url_empty", []byte(`{"videoUrl":"","transcript":"0:00\nHello\n"}`)},
	{"video_url_whitespace", []byte(`{"videoUrl":"   ","transcript":"0:00\nHello\n"}`)},
	{"transcript_number", []byte(`{"videoUrl":"` + testValidVideoURL + `","transcript":2}`)},
	{"transcript_empty", []byte(`{"videoUrl":"` + testValidVideoURL + `","transcript":""}`)},
	{"transcript_whitespace", []byte(`{"videoUrl":"` + testValidVideoURL + `","transcript":" \t "}`)},
	{"duplicate_video_url", []byte(`{"videoUrl":"` + testValidVideoURL + `","videoUrl":"x","transcript":"0:00\nHello\n"}`)},
	{"trailing_data", []byte(`{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n"}{}`)},
	{"not_object", []byte(`[]`)},
	{"not_json", []byte(`not json`)},
	{"invalid_utf8", []byte("{\"videoUrl\":\"" + testValidVideoURL + "\",\"transcript\":\"\xff\"}")},
	{"unpaired_surrogate", []byte(`{"videoUrl":"` + testValidVideoURL + `","transcript":"\ud800"}`)},
	{"unpaired_surrogate_undeclared", []byte(`{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","extra":"\ud800"}`)},
	{"invalid_utf8_undeclared", []byte("{\"videoUrl\":\"" + testValidVideoURL + "\",\"transcript\":\"0:00\\nHello\\n\",\"extra\":\"\xff\"}")},
}

// TestRejectBodyTable checks the body-shape rejections that map to
// invalid_body.
func TestRejectBodyTable(t *testing.T) {
	tests := invalidBodyCases
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

// optionalMemberNames are the optional members, and nonStringValues the JSON
// values that are not a string, which a body may not give one of them.
var (
	optionalMemberNames = []string{"title", "channelName", "description"}
	nonStringValues     = []string{"1", "null", "true"}
)

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
	for _, member := range optionalMemberNames {
		for _, value := range nonStringValues {
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

// invalidVideoURLs are the video URLs the validator rejects.
var invalidVideoURLs = []string{
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

// TestRejectVideoURLTable checks the video URLs the validator rejects.
func TestRejectVideoURLTable(t *testing.T) {
	for _, raw := range invalidVideoURLs {
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

// invalidTranscriptRaws are transcripts that do not match the copied format.
var invalidTranscriptRaws = []string{
	"not a transcript",
	"0:00\n", // a timestamp with no text
	"0:24MARKER_BODY",
}

// TestRejectInvalidTranscript checks that a transcript that does not match the
// copied format is rejected as invalid_transcript.
func TestRejectInvalidTranscript(t *testing.T) {
	for _, raw := range invalidTranscriptRaws {
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

// TestValidateBodyKeyInput checks the key input validateBody builds: an absent
// optional member and an empty one give the same input, the video URL is the
// normalized one rather than the request's string, and the other values are
// the request's.
func TestValidateBodyKeyInput(t *testing.T) {
	ts := newTestServer(t)
	keyInput := func(t *testing.T, body string) articlecache.KeyInput {
		t.Helper()
		input, rej := ts.validateBody([]byte(body), "0123456789abcdef")
		if rej != nil {
			t.Fatalf("validateBody rejected %s: %+v", body, *rej)
		}
		return input.key
	}

	absent := keyInput(t, validBody("https://youtu.be/dQw4w9WgXcQ", testValidRaw))
	empty := keyInput(t, `{"videoUrl":"https://youtu.be/dQw4w9WgXcQ","transcript":"0:00\nHello\n","title":"","channelName":"","description":""}`)
	want := articlecache.KeyInput{VideoURL: testValidVideoURL, Transcript: testValidRaw}
	if absent != want {
		t.Errorf("absent members: key input = %+v, want %+v", absent, want)
	}
	if empty != want {
		t.Errorf("empty members: key input = %+v, want %+v", empty, want)
	}

	set := keyInput(t, `{"videoUrl":"`+testValidVideoURL+`","transcript":"0:00\nHello\n","title":"T","channelName":"C","description":"D"}`)
	want = articlecache.KeyInput{VideoURL: testValidVideoURL, Transcript: testValidRaw, Title: "T", ChannelName: "C", Description: "D"}
	if set != want {
		t.Errorf("set members: key input = %+v, want %+v", set, want)
	}
}

// TestStartRejectsLikeArticles sends every body the legacy endpoint's
// rejection tables use to a generation start too, and checks that each is
// rejected with the same status and reason, and that the start adds no record,
// calls no writer, posts no notice, and leaves no slot taken.
func TestStartRejectsLikeArticles(t *testing.T) {
	type input struct {
		name          string
		body          func() io.Reader
		contentLength int64 // 0 keeps the length the body gives
	}
	text := func(body string) func() io.Reader {
		return func() io.Reader { return strings.NewReader(body) }
	}
	var inputs []input
	for _, tc := range invalidBodyCases {
		inputs = append(inputs, input{name: "body/" + tc.name, body: func() io.Reader { return bytes.NewReader(tc.body) }})
	}
	for _, member := range optionalMemberNames {
		for _, value := range nonStringValues {
			body := `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","` + member + `":` + value + `}`
			inputs = append(inputs, input{name: "optional/" + member + "_" + value, body: text(body)})
		}
	}
	for _, raw := range invalidVideoURLs {
		inputs = append(inputs, input{name: "video_url/" + raw, body: text(validBody(raw, testValidRaw))})
	}
	for _, raw := range invalidTranscriptRaws {
		inputs = append(inputs, input{name: "transcript/" + raw, body: text(validBody(testValidVideoURL, raw))})
	}
	inputs = append(inputs,
		input{name: "too_large/content_length", body: text("{}"), contentLength: maxBodyBytes + 1},
		input{name: "too_large/no_content_length", body: text(strings.Repeat("x", maxBodyBytes+10)), contentLength: -1},
	)

	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			send := func(ts *testServer, target string) *httptest.ResponseRecorder {
				req := newRequest(http.MethodPost, target, in.body())
				if in.contentLength != 0 {
					req.ContentLength = in.contentLength
				}
				rec := ts.serve(req)
				ts.waitJobs()
				return rec
			}
			legacy := send(newTestServer(t), ArticlesPath)
			ts := newTestServer(t)
			got := send(ts, JobsPath)
			if got.Code != legacy.Code || decodeBody(t, got).Reason != decodeBody(t, legacy).Reason {
				t.Errorf("generation start: %d %s, want the legacy endpoint's %d %s",
					got.Code, got.Body.String(), legacy.Code, legacy.Body.String())
			}
			if got.Code < http.StatusBadRequest {
				t.Fatalf("the input was accepted: %d %s", got.Code, got.Body.String())
			}
			if dump := ts.cache.DumpRecordsForTest(); dump != "" {
				t.Errorf("a rejected start added a record:\n%s", dump)
			}
			if len(ts.writer.Calls) != 0 || len(ts.notifier.Calls) != 0 {
				t.Errorf("a rejected start caused a side effect: writer=%d notifier=%d", len(ts.writer.Calls), len(ts.notifier.Calls))
			}
			if len(ts.slots) != 0 || len(ts.reading) != 0 {
				t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
			}
		})
	}
}
