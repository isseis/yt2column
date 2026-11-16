//go:build test

package extjob

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
	llmtestutil "github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// validMembers are the members of a request the checks accept, so a row that
// adds one member to them is rejected for that member alone.
const validMembers = `"videoUrl":"` + TestVideoURL + `","transcript":"0:00\nHello\n"`

// requireRejected checks a prepare result is the rejection and that it holds
// no value of the request.
func requireRejected(t *testing.T, got string) {
	t.Helper()
	if got != `{"status":"rejected","reason":"invalid_body"}` {
		t.Errorf("result = %s, want the invalid_body rejection", got)
	}
}

// TestPrepareJSONRejectsRequest ports the body rows of the server's
// TestStartRejectsBody: every request the shape checks reject is an
// invalid_body rejection that keeps nothing. The members are a closed set,
// so an unknown member is rejected too (the server ignored them).
func TestPrepareJSONRejectsRequest(t *testing.T) {
	cases := []struct{ name, request string }{
		{"missing_video_url", `{"transcript":"0:00\nHello\n"}`},
		{"missing_transcript", `{"videoUrl":"` + TestVideoURL + `"}`},
		{"video_url_number", `{"videoUrl":1,"transcript":"0:00\nHello\n"}`},
		{"video_url_null", `{"videoUrl":null,"transcript":"0:00\nHello\n"}`},
		{"video_url_empty", `{"videoUrl":"","transcript":"0:00\nHello\n"}`},
		{"video_url_whitespace", `{"videoUrl":"   ","transcript":"0:00\nHello\n"}`},
		{"transcript_number", `{"videoUrl":"` + TestVideoURL + `","transcript":2}`},
		{"transcript_empty", `{"videoUrl":"` + TestVideoURL + `","transcript":""}`},
		{"transcript_whitespace", `{"videoUrl":"` + TestVideoURL + `","transcript":" \t "}`},
		{"duplicate_video_url", `{"videoUrl":"` + TestVideoURL + `","videoUrl":"x","transcript":"0:00\nHello\n"}`},
		{"trailing_data", `{` + validMembers + `}{}`},
		{"not_object", `[]`},
		{"not_json", `not json`},
		{"invalid_utf8", "{\"videoUrl\":\"" + TestVideoURL + "\",\"transcript\":\"\xff\"}"},
		{"unpaired_surrogate", `{"videoUrl":"` + TestVideoURL + `","transcript":"\ud800"}`},
		{"unknown_member", `{` + validMembers + `,"extra":"x"}`},
		{"unknown_member_surrogate", `{` + validMembers + `,"extra":"\ud800"}`},
		{"unknown_member_invalid_utf8", "{" + validMembers + ",\"extra\":\"\xff\"}"},
		// The send mode the server read is gone: it is an unknown member.
		{"mode_member", `{` + validMembers + `,"mode":"plain"}`},
	}
	for _, member := range []string{"title", "channelName", "description"} {
		for _, value := range []string{"1", "null", "true"} {
			cases = append(cases, struct{ name, request string }{
				"optional/" + member + "_" + value, `{` + validMembers + `,"` + member + `":` + value + `}`,
			})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModule(&fakeBackend{})
			requireRejected(t, m.PrepareJSON(tc.request))
			if m.kept != nil {
				t.Error("a rejected prepare kept an input")
			}
		})
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

// invalidTranscripts are transcripts that do not match the copied format.
var invalidTranscripts = []string{
	"not a transcript",
	"0:00\n", // a timestamp with no text
	"0:24MARKER_BODY",
}

// TestPrepareRejectsInput ports the video URL and transcript rows: Prepare
// wraps ErrInvalidInput for each, and PrepareJSON answers invalid_body.
func TestPrepareRejectsInput(t *testing.T) {
	inputs := map[string]StartInput{}
	for _, raw := range invalidVideoURLs {
		inputs["video_url/"+raw] = StartInput{VideoURL: raw, Transcript: TestRawText}
	}
	for _, raw := range invalidTranscripts {
		inputs["transcript/"+raw] = StartInput{VideoURL: TestVideoURL, Transcript: raw}
	}
	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			if _, err := Prepare(in); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Prepare error = %v, want ErrInvalidInput", err)
			}
			request, err := json.Marshal(map[string]string{"videoUrl": in.VideoURL, "transcript": in.Transcript})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			requireRejected(t, NewModule(&fakeBackend{}).PrepareJSON(string(request)))
		})
	}
}

// TestPrepareOptionalMembers checks that the optional metadata is accepted
// when missing or empty, and reaches the transcript empty.
func TestPrepareOptionalMembers(t *testing.T) {
	for name, request := range map[string]string{
		"missing": `{` + validMembers + `}`,
		"empty":   `{` + validMembers + `,"title":"","channelName":"","description":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := NewModule(&fakeBackend{})
			var got prepareResult
			if err := json.Unmarshal([]byte(m.PrepareJSON(request)), &got); err != nil || got.Status != "ok" {
				t.Fatalf("result = %+v, %v; want ok", got, err)
			}
			tr, err := m.kept.source.Fetch(context.Background(), m.kept.videoURL)
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if tr.Title != "" || tr.ChannelName != "" || tr.Description != "" {
				t.Errorf("optional fields = %q/%q/%q, want empty", tr.Title, tr.ChannelName, tr.Description)
			}
		})
	}
}

// TestPrepareTranscriptValues checks that the transcript carries the video ID
// and the normalized URL derived from the input's video URL, never the
// input's string, for each URL form the validator accepts, and that the three
// forms of one video give one digest.
func TestPrepareTranscriptValues(t *testing.T) {
	var digests []string
	for _, raw := range []string{
		"https://youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s",
		"https://m.youtube.com/shorts/dQw4w9WgXcQ",
	} {
		t.Run(raw, func(t *testing.T) {
			p, err := Prepare(StartInput{VideoURL: raw, Transcript: TestRawText})
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			tr, err := p.source.Fetch(context.Background(), p.videoURL)
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if tr.VideoID != TestVideoID || tr.VideoURL != TestVideoURL || p.VideoID() != TestVideoID {
				t.Errorf("video = %q %q (%q), want %q %q", tr.VideoID, tr.VideoURL, p.VideoID(), TestVideoID, TestVideoURL)
			}
			digests = append(digests, p.InputDigest())
		})
	}
	for _, d := range digests[1:] {
		if d != digests[0] {
			t.Errorf("digests of one video differ: %v", digests)
		}
	}
}

// TestInputDigest checks the digest: 64 lowercase hex, the same for the same
// input, an absent optional value the same as an empty one, and changed by
// every one of the five values, including a boundary moved between two
// neighbors.
func TestInputDigest(t *testing.T) {
	base := StartInput{VideoURL: TestVideoURL, Transcript: TestRawText, Title: "T", ChannelName: "C", Description: "D"}
	digestOf := func(in StartInput) string {
		t.Helper()
		p, err := Prepare(in)
		if err != nil {
			t.Fatalf("Prepare(%+v): %v", in, err)
		}
		return p.InputDigest()
	}
	want := digestOf(base)
	if len(want) != 64 || strings.Trim(want, "0123456789abcdef") != "" {
		t.Fatalf("digest = %q, want 64 lowercase hex", want)
	}
	if digestOf(base) != want {
		t.Error("the same input gave two digests")
	}
	if digestOf(StartInput{VideoURL: TestVideoURL, Transcript: TestRawText}) != digestOf(StartInput{VideoURL: TestVideoURL, Transcript: TestRawText, Title: ""}) {
		t.Error("an absent and an empty optional value gave different digests")
	}
	changed := map[string]StartInput{
		"video url":   {VideoURL: "https://www.youtube.com/watch?v=abcdefghijk", Transcript: base.Transcript, Title: "T", ChannelName: "C", Description: "D"},
		"transcript":  {VideoURL: base.VideoURL, Transcript: "0:00\nHello!\n", Title: "T", ChannelName: "C", Description: "D"},
		"title":       {VideoURL: base.VideoURL, Transcript: base.Transcript, Title: "T2", ChannelName: "C", Description: "D"},
		"channel":     {VideoURL: base.VideoURL, Transcript: base.Transcript, Title: "T", ChannelName: "C2", Description: "D"},
		"description": {VideoURL: base.VideoURL, Transcript: base.Transcript, Title: "T", ChannelName: "C", Description: "D2"},
		// "T"+"C" and "TC"+"" are one string run together; the length prefix
		// keeps them apart.
		"boundary": {VideoURL: base.VideoURL, Transcript: base.Transcript, Title: "TC", ChannelName: "", Description: "D"},
	}
	for name, in := range changed {
		if got := digestOf(in); got == want {
			t.Errorf("changing the %s did not change the digest", name)
		}
	}
}

// TestPrepareJSONRequestSizeBoundary checks the 4 MiB bound on a request: one
// of exactly maxRequestBytes is accepted, one byte more is rejected.
func TestPrepareJSONRequestSizeBoundary(t *testing.T) {
	prefix := `{` + validMembers + `,"description":"`
	const suffix = `"}`
	pad := func(total int) string {
		return prefix + strings.Repeat("a", total-len(prefix)-len(suffix)) + suffix
	}
	at := pad(maxRequestBytes)
	if len(at) != maxRequestBytes {
		t.Fatalf("request is %d bytes, want %d", len(at), maxRequestBytes)
	}
	var got prepareResult
	if err := json.Unmarshal([]byte(NewModule(&fakeBackend{}).PrepareJSON(at)), &got); err != nil || got.Status != "ok" {
		t.Errorf("a request of exactly the limit: %+v, %v; want ok", got, err)
	}
	requireRejected(t, NewModule(&fakeBackend{}).PrepareJSON(pad(maxRequestBytes+1)))
}

// TestPrepareJSONKeepsEarlierInput checks that a rejected prepare leaves the
// input an earlier prepare kept, and an accepted one replaces it.
func TestPrepareJSONKeepsEarlierInput(t *testing.T) {
	m := NewModule(&fakeBackend{})
	var first prepareResult
	if err := json.Unmarshal([]byte(m.PrepareJSON(`{`+validMembers+`}`)), &first); err != nil {
		t.Fatalf("decode: %v", err)
	}
	requireRejected(t, m.PrepareJSON(`{"videoUrl":"x","transcript":"y"}`))
	if m.kept == nil || m.kept.InputDigest() != first.InputDigest {
		t.Error("a rejection changed the kept input")
	}
	var second prepareResult
	if err := json.Unmarshal([]byte(m.PrepareJSON(`{`+validMembers+`,"title":"other"}`)), &second); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if second.InputDigest == first.InputDigest || m.kept.InputDigest() != second.InputDigest {
		t.Error("an accepted prepare did not replace the kept input")
	}
}

// TestChannelNameAgreesWithWriter feeds one table of channel names to the
// three places that check one (Prepare, the writer before the LLM call, and
// Article.CheckPublishable) and requires the same verdict from each, so an
// input Prepare accepts reaches a writer and a publisher that accept it too.
func TestChannelNameAgreesWithWriter(t *testing.T) {
	for _, tc := range writertestutil.ChannelNameCases() {
		t.Run(tc.Name, func(t *testing.T) {
			// The strict JSON layer rejects invalid UTF-8 for the whole
			// request before the channel name check runs, so the JSON leg
			// proves nothing about that check for such a value; Prepare and
			// the other legs still apply.
			_, err := Prepare(StartInput{VideoURL: TestVideoURL, Transcript: TestRawText, ChannelName: tc.Value})
			if tc.Valid != (err == nil) {
				t.Errorf("Prepare error = %v, want valid = %v", err, tc.Valid)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Prepare error = %v, want ErrInvalidInput", err)
			}
			if utf8.ValidString(tc.Value) {
				quoted, err := json.Marshal(tc.Value)
				if err != nil {
					t.Fatalf("Marshal: %v", err)
				}
				got := NewModule(&fakeBackend{}).PrepareJSON(`{` + validMembers + `,"channelName":` + string(quoted) + `}`)
				if accepted := strings.Contains(got, `"status":"ok"`); accepted != tc.Valid {
					t.Errorf("PrepareJSON = %s, want valid = %v", got, tc.Valid)
				}
			}

			article := writer.Article{Title: "t", ChannelName: tc.Value, Body: "b", SourceURL: TestVideoURL, Model: "m"}
			err = article.CheckPublishable()
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
			got2, err := w.Write(context.Background(), transcript.Transcript{
				VideoID: TestVideoID, VideoURL: TestVideoURL, ChannelName: tc.Value,
				Segments: []transcript.Segment{{Text: "Hello"}},
			})
			if tc.Valid != (err == nil) {
				t.Errorf("Write() = %v, want valid = %v", err, tc.Valid)
			}
			if err == nil && got2.ChannelName != tc.Value {
				t.Errorf("Write() ChannelName = %q, want %q", got2.ChannelName, tc.Value)
			}
			if !tc.Valid && len(client.Calls) != 0 {
				t.Errorf("LLM called %d times for a rejected channel name, want 0", len(client.Calls))
			}
		})
	}
}
