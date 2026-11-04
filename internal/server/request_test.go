//go:build test

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// requestIDShape is the request ID form the server generates.
var requestIDShape = regexp.MustCompile(`^[0-9a-f]{16}$`)

// requireStartUntouched checks, after a generation start was refused, that it
// added no record, called no writer, and left no slot taken.
func requireStartUntouched(t *testing.T, ts *testServer) {
	t.Helper()
	if dump := ts.cache.DumpRecordsForTest(); dump != "" {
		t.Errorf("a rejected start added a record:\n%s", dump)
	}
	if len(ts.writer.Calls) != 0 {
		t.Errorf("writer calls = %d for a rejected start, want 0", len(ts.writer.Calls))
	}
	if len(ts.slots) != 0 || len(ts.reading) != 0 {
		t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
	}
}

// invalidBodyCases are bodies the body checks reject as invalid_body for
// their shape.
var invalidBodyCases = []struct {
	name string
	body string
}{
	{"missing_video_url", `{"transcript":"0:00\nHello\n"}`},
	{"missing_transcript", `{"videoUrl":"` + testValidVideoURL + `"}`},
	{"video_url_number", `{"videoUrl":1,"transcript":"0:00\nHello\n"}`},
	{"video_url_null", `{"videoUrl":null,"transcript":"0:00\nHello\n"}`},
	{"video_url_empty", `{"videoUrl":"","transcript":"0:00\nHello\n"}`},
	{"video_url_whitespace", `{"videoUrl":"   ","transcript":"0:00\nHello\n"}`},
	{"transcript_number", `{"videoUrl":"` + testValidVideoURL + `","transcript":2}`},
	{"transcript_empty", `{"videoUrl":"` + testValidVideoURL + `","transcript":""}`},
	{"transcript_whitespace", `{"videoUrl":"` + testValidVideoURL + `","transcript":" \t "}`},
	{"duplicate_video_url", `{"videoUrl":"` + testValidVideoURL + `","videoUrl":"x","transcript":"0:00\nHello\n"}`},
	{"trailing_data", `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n"}{}`},
	{"not_object", `[]`},
	{"not_json", `not json`},
	{"invalid_utf8", "{\"videoUrl\":\"" + testValidVideoURL + "\",\"transcript\":\"\xff\"}"},
	{"unpaired_surrogate", `{"videoUrl":"` + testValidVideoURL + `","transcript":"\ud800"}`},
	{"unpaired_surrogate_undeclared", `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","extra":"\ud800"}`},
	{"invalid_utf8_undeclared", "{\"videoUrl\":\"" + testValidVideoURL + "\",\"transcript\":\"0:00\\nHello\\n\",\"extra\":\"\xff\"}"},
}

// optionalMemberNames are the optional members, and nonStringValues the JSON
// values that are not a string, which a body may not give one of them.
var (
	optionalMemberNames = []string{"title", "channelName", "description"}
	nonStringValues     = []string{"1", "null", "true"}
)

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

// invalidTranscriptRaws are transcripts that do not match the copied format.
var invalidTranscriptRaws = []string{
	"not a transcript",
	"0:00\n", // a timestamp with no text
	"0:24MARKER_BODY",
}

// TestStartRejectsBody checks every body the body checks reject, by the
// reason each maps to: a malformed body or an optional member that is not a
// string is invalid_body, a video URL the validator refuses is
// invalid_video_url, and a transcript not in the copied format is
// invalid_transcript. A refused start adds no record, calls no writer, and
// leaves no slot taken.
func TestStartRejectsBody(t *testing.T) {
	type rejected struct{ name, body, reason string }
	var tests []rejected
	for _, tc := range invalidBodyCases {
		tests = append(tests, rejected{"body/" + tc.name, tc.body, "invalid_body"})
	}
	for _, member := range optionalMemberNames {
		for _, value := range nonStringValues {
			body := `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","` + member + `":` + value + `}`
			tests = append(tests, rejected{"optional/" + member + "_" + value, body, "invalid_body"})
		}
	}
	for _, raw := range invalidVideoURLs {
		tests = append(tests, rejected{"video_url/" + raw, validBody(raw, testValidRaw), "invalid_video_url"})
	}
	for _, raw := range invalidTranscriptRaws {
		tests = append(tests, rejected{"transcript/" + raw, validBody(testValidVideoURL, raw), "invalid_transcript"})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.startBody(tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
			}
			if body := decodeBody(t, rec); body.Reason != tc.reason {
				t.Errorf("reason = %q, want %s", body.Reason, tc.reason)
			}
			requireStartUntouched(t, ts)
		})
	}
}

// TestOptionalMembers checks that the optional metadata is accepted when
// missing or empty, and reaches the writer empty. A value that is not a
// string is TestStartRejectsBody's.
func TestOptionalMembers(t *testing.T) {
	for name, body := range map[string]string{
		"missing": validBody(testValidVideoURL, testValidRaw),
		"empty":   `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\nHello\n","title":"","channelName":"","description":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			rec := ts.startBody(body)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202", rec.Code)
			}
			got := ts.writer.Calls[0].Transcript
			if got.Title != "" || got.ChannelName != "" || got.Description != "" {
				t.Errorf("optional fields = %q/%q/%q, want empty", got.Title, got.ChannelName, got.Description)
			}
		})
	}
}

// TestStartTranscriptValues checks that a generation start hands the writer
// the video ID and the normalized URL derived from the request's video URL,
// never the request's string, for each URL form the validator accepts.
func TestStartTranscriptValues(t *testing.T) {
	for _, raw := range []string{
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
		"https://m.youtube.com/shorts/dQw4w9WgXcQ",
	} {
		t.Run(raw, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			acceptedKey(t, ts.startBody(validBody(raw, testValidRaw)))
			calls := ts.writer.RecordedCalls()
			if len(calls) != 1 {
				t.Fatalf("writer calls = %d, want 1", len(calls))
			}
			got := calls[0].Transcript
			if got.VideoID != testVideoID || got.VideoURL != testValidVideoURL {
				t.Errorf("transcript video = %q %q, want %q %q", got.VideoID, got.VideoURL, testVideoID, testValidVideoURL)
			}
		})
	}
}

// TestHeaderRejectionReadsNoBody checks that a generation start rejected on
// its method (including one that declares an oversized body) has zero bytes of
// its body read.
func TestHeaderRejectionReadsNoBody(t *testing.T) {
	type headerCase struct {
		name  string
		build func(t *testing.T, body io.Reader) *http.Request
		code  int
	}
	// An unknown path is TestUnknownPathTouchesNothing's.
	tests := []headerCase{
		{"method", func(_ *testing.T, body io.Reader) *http.Request {
			return newRequest(http.MethodGet, JobsPath, body)
		}, http.StatusMethodNotAllowed},
		{"oversized_body_wrong_method", func(_ *testing.T, body io.Reader) *http.Request {
			req := newRequest(http.MethodPut, JobsPath, body)
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

// TestBodyTooLarge checks that an oversized body is rejected with 413, that no
// more than the limit plus one byte is read, and that the refused start leaves
// nothing behind.
func TestBodyTooLarge(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		contentLength int64
		maxRead       int // the most bytes the server may read
	}{
		{"content_length", "{}", maxBodyBytes + 1, 0},
		{"no_content_length", strings.Repeat("x", maxBodyBytes+10), -1, maxBodyBytes + 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestServer(t)
			counter := &recordingReader{r: strings.NewReader(tc.body)}
			req := newRequest(http.MethodPost, JobsPath, io.NopCloser(counter))
			req.ContentLength = tc.contentLength
			rec := ts.serve(req)
			ts.waitJobs()
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413 (body %q)", rec.Code, rec.Body.String())
			}
			if counter.n > tc.maxRead {
				t.Errorf("read %d bytes, want at most %d", counter.n, tc.maxRead)
			}
			requireStartUntouched(t, ts)
		})
	}
}

// TestChannelNameAgreesWithWriter feeds one table of channel names to the three
// places that check one (the request, the writer before the LLM call, and
// Article.CheckPublishable) and requires the same verdict from each. A request
// the server accepts must reach a writer and a publisher that accept it too.
func TestChannelNameAgreesWithWriter(t *testing.T) {
	for _, tc := range writertestutil.ChannelNameCases() {
		t.Run(tc.Name, func(t *testing.T) {
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
				requireChannelNameRequestVerdict(t, newTestServer(t), body, tc)
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

// requireChannelNameRequestVerdict sends body as a generation start and checks
// it is accepted and its channel name reaches the writer, or is rejected as
// invalid_body, leaving nothing behind.
func requireChannelNameRequestVerdict(t *testing.T, ts *testServer, body string, tc writertestutil.ChannelNameCase) {
	t.Helper()
	rec := ts.startBody(body)
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
	requireStartUntouched(t, ts)
}

// TestValidateBodyKeyInput checks the key input validateBody builds: an absent
// optional member and an empty one give the same input, the video URL is the
// normalized one rather than the request's string, the video ID is the one
// the URL names whichever form it is written in, and the other values are the
// request's.
func TestValidateBodyKeyInput(t *testing.T) {
	ts := newTestServer(t)
	keyInput := func(t *testing.T, body string) articlecache.KeyInput {
		t.Helper()
		input, rej := ts.validateBody([]byte(body))
		if rej != nil {
			t.Fatalf("validateBody rejected %s: %+v", body, *rej)
		}
		return input.key
	}

	absent := keyInput(t, validBody("https://youtu.be/dQw4w9WgXcQ", testValidRaw))
	empty := keyInput(t, `{"videoUrl":"https://youtu.be/dQw4w9WgXcQ","transcript":"0:00\nHello\n","title":"","channelName":"","description":""}`)
	want := articlecache.KeyInput{VideoURL: testValidVideoURL, VideoID: "dQw4w9WgXcQ", Transcript: testValidRaw}
	if absent != want {
		t.Errorf("absent members: key input = %+v, want %+v", absent, want)
	}
	if empty != want {
		t.Errorf("empty members: key input = %+v, want %+v", empty, want)
	}

	set := keyInput(t, `{"videoUrl":"`+testValidVideoURL+`","transcript":"0:00\nHello\n","title":"T","channelName":"C","description":"D"}`)
	want = articlecache.KeyInput{VideoURL: testValidVideoURL, VideoID: "dQw4w9WgXcQ", Transcript: testValidRaw, Title: "T", ChannelName: "C", Description: "D"}
	if set != want {
		t.Errorf("set members: key input = %+v, want %+v", set, want)
	}
}
