//go:build test

package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/writer"
)

// pricedUsage is a generation usage whose model is in the rate table, so the
// article's cost is known.
func pricedUsage() writer.RunUsage {
	return writer.RunUsage{Generation: writer.StageUsage{
		Origin: writer.OriginRun,
		Model:  "claude-haiku-5-5",
		Usage:  llm.KnownUsage(llm.TokenCounts{Input: 23000}),
	}}
}

// TestStatusDoneArticle checks a done article's status: the neutralized
// display Markdown and the file content are what the publisher renders for the article (the
// file being what --out writes), and the cost is the formatted known cost.
func TestStatusDoneArticle(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	article := testArticle()
	article.RunUsage = pricedUsage()
	ts.writer.Result = article
	key := acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))

	job := ts.getStatus(t, key).Job
	if job.State != "done" || job.Article == nil {
		t.Fatalf("job = %+v, want done with an article", job)
	}
	display, err := publisher.NeutralizedDisplayMarkdown(article)
	if err != nil {
		t.Fatalf("NeutralizedDisplayMarkdown: %v", err)
	}
	wantMarkdown := display.Markdown
	wantFile, err := publisher.FileContent(article)
	if err != nil {
		t.Fatalf("FileContent: %v", err)
	}
	if job.Article.Markdown != wantMarkdown {
		t.Errorf("markdown = %q, want %q", job.Article.Markdown, wantMarkdown)
	}
	if job.Article.File != wantFile {
		t.Errorf("file = %q, want %q", job.Article.File, wantFile)
	}
	usd, known := publisher.FormatArticleCost(article.RunUsage)
	if !known {
		t.Fatal("the priced usage has no known cost")
	}
	if job.Article.Cost != (statusCost{State: "known", USD: usd}) {
		t.Errorf("cost = %+v, want known %s", job.Article.Cost, usd)
	}
}

// TestStatusArticleNeutralized checks the article member of a done status
// response: exactly the members the Viewer reads, the Markdown and its flag
// from NeutralizedDisplayMarkdown, the start's video title and the article's
// channel name neutralized for the heading ("" for none), and the file
// unchanged. The article holds every sequence the removed Webhook post
// refused, and it is still returned.
func TestStatusArticleNeutralized(t *testing.T) {
	refused := testArticle()
	refused.Title = "Talk @channel"
	refused.ChannelName = "@handle"
	refused.Body = "Hi @here and `@code`, <!channel> <@name> <a|b> AC/@x &#64;y https://www.youtube.com/@kept\n\nSource: " + testValidVideoURL + "\n"
	plain := testArticle()
	plain.ChannelName = ""

	cases := []struct {
		name        string
		article     writer.Article
		videoTitle  string // "" sends no title member
		wantTitle   string
		wantChannel string
	}{
		{"mentions", refused, "Live with @here https://www.youtube.com/@x", "Live with @\u200bhere https://www.youtube.com/@\u200bx", "@\u200bhandle"},
		{"no title and no channel", plain, "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			ts.writer.Result = tc.article
			body := validBody(testValidVideoURL, testValidRaw)
			if tc.videoTitle != "" {
				body = `{"videoUrl":` + strconv.Quote(testValidVideoURL) + `,"transcript":` + strconv.Quote(testValidRaw) +
					`,"title":` + strconv.Quote(tc.videoTitle) + `,"mode":"plain"}`
			}
			key := acceptedKey(t, ts.startBody(body))
			rec := ts.statusRequest(key)
			var raw struct {
				Job struct {
					State   string                     `json:"state"`
					Article map[string]json.RawMessage `json:"article"`
				} `json:"job"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatalf("decode %q: %v", rec.Body.String(), err)
			}
			if raw.Job.State != "done" {
				t.Fatalf("state = %q, want done (body %s)", raw.Job.State, rec.Body.String())
			}
			members := slices.Sorted(maps.Keys(raw.Job.Article))
			want := []string{"channelName", "cost", "durationMs", "file", "markdown", "mentionsNeutralized", "videoTitle"}
			if !slices.Equal(members, want) {
				t.Errorf("article members = %v, want %v", members, want)
			}

			got := ts.getStatus(t, key).Job.Article
			display, err := publisher.NeutralizedDisplayMarkdown(tc.article)
			if err != nil {
				t.Fatalf("NeutralizedDisplayMarkdown: %v", err)
			}
			file, err := publisher.FileContent(tc.article)
			if err != nil {
				t.Fatalf("FileContent: %v", err)
			}
			if got.Markdown != display.Markdown || got.MentionsNeutralized != display.Neutralized {
				t.Errorf("markdown, mentionsNeutralized = %q, %v; want %q, %v", got.Markdown, got.MentionsNeutralized, display.Markdown, display.Neutralized)
			}
			if got.VideoTitle != tc.wantTitle {
				t.Errorf("videoTitle = %q, want %q", got.VideoTitle, tc.wantTitle)
			}
			if got.ChannelName != tc.wantChannel {
				t.Errorf("channelName = %q, want %q", got.ChannelName, tc.wantChannel)
			}
			if got.File != file {
				t.Errorf("file = %q, want %q", got.File, file)
			}
		})
	}
}

// TestStatusCostUnknown checks that an article whose cost is not known reports
// the unknown cost state and no amount.
func TestStatusCostUnknown(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	key := acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))
	job := ts.getStatus(t, key).Job
	if job.Article == nil || job.Article.Cost != (statusCost{State: "unknown"}) {
		t.Errorf("article = %+v, want the unknown cost", job.Article)
	}
}

// TestStatusDuration checks that a done article's duration runs from the
// accepted start to the end of the generation, and, after a failure and a
// retry, from the retry.
func TestStatusDuration(t *testing.T) {
	clock := newTestClock()
	ts := newJobsTestServer(t, jobsServerOptions{clock: clock.Now})
	body := validBody(testValidVideoURL, testValidRaw)

	reached, release := holdAtDraft(ts.writer)
	key := acceptedKey(t, ts.start(strings.NewReader(body)))
	<-reached
	clock.Advance(5 * time.Second)
	release()
	ts.waitJobs()
	if got := ts.getStatus(t, key).Job.Article.DurationMs; got != 5000 {
		t.Errorf("duration = %d ms, want 5000", got)
	}

	retried := bodyWith("retried")
	ts.writer.Progress = nil
	ts.writer.Err = errors.New("the provider refused")
	acceptedKey(t, ts.startBody(retried))
	clock.Advance(100 * time.Second)
	ts.writer.Err = nil
	reached, release = holdAtDraft(ts.writer)
	retriedKey := acceptedKey(t, ts.start(strings.NewReader(retried)))
	<-reached
	clock.Advance(7 * time.Second)
	release()
	ts.waitJobs()
	if got := ts.getStatus(t, retriedKey).Job.Article.DurationMs; got != 7000 {
		t.Errorf("duration after a retry = %d ms, want 7000", got)
	}
}

// TestStatusUnknown checks that a key no start has used is answered with 200
// and the unknown state alone, not with a rejection.
func TestStatusUnknown(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	key := ts.keyer.Key(ts.validInput(t, validBody(testValidVideoURL, testValidRaw)).key)
	rec := ts.statusRequest(key.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var got statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != "ok" || !requestIDShape.MatchString(got.RequestID) || !reflect.DeepEqual(got.Job, statusJob{State: "unknown"}) {
		t.Errorf("response = %+v, want ok with a request ID and the unknown state alone", got)
	}
}

// TestStatusHasNoSideEffects checks that status requests change no record and
// call no writer.
func TestStatusHasNoSideEffects(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	key := acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))
	before := ts.cache.DumpRecordsForTest()
	for range 3 {
		ts.getStatus(t, key)
	}
	ts.waitJobs()
	if after := ts.cache.DumpRecordsForTest(); after != before {
		t.Errorf("status requests changed the records:\nbefore %s\nafter  %s", before, after)
	}
	if len(ts.writer.Calls) != 1 {
		t.Errorf("side effects: writer=%d, want only the one generation", len(ts.writer.Calls))
	}
}

// TestStatusUnknownAfterRestart checks that a key done on one server is
// unknown on the next one, as after a restart.
func TestStatusUnknownAfterRestart(t *testing.T) {
	first := newTestServer(t)
	first.writer.Result = testArticle()
	key := acceptedKey(t, first.startBody(validBody(testValidVideoURL, testValidRaw)))
	if state := first.getStatus(t, key).Job.State; state != "done" {
		t.Fatalf("state on the first server = %q, want done", state)
	}
	second := newTestServer(t)
	if job := second.getStatus(t, key).Job; !reflect.DeepEqual(job, statusJob{State: "unknown"}) {
		t.Errorf("job on the next server = %+v, want unknown", job)
	}
}

// TestResponsesOmitInputs checks that no job API response, a rejection and a
// same-video answer included, carries a request value, an error text, or a
// secret, apart from the done article's Markdown and file and its heading
// values (the video title the start sent and the article's channel name),
// and that the log
// carries neither the transcript, a secret, nor the video URL string a start
// sent.
func TestResponsesOmitInputs(t *testing.T) {
	const errorMarker = "MARKER-ERROR-TEXT"
	// sentURL is the video URL string an inquiry start sends; the log may
	// carry the video ID, never the string.
	const sentURL = "https://youtu.be/" + testVideoID
	ts := newJobsTestServer(t, jobsServerOptions{})
	article := testArticle()
	article.Title = testTranscriptMarker
	ts.writer.Result = article
	body, err := json.Marshal(map[string]string{
		"videoUrl":    testValidVideoURL,
		"transcript":  "0:00\n" + testTranscriptMarker + "\n",
		"title":       testTranscriptMarker,
		"channelName": testMarker,
		"description": testTranscriptMarker,
		"mode":        modePlain,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	failing, err := json.Marshal(map[string]string{
		"videoUrl":   testValidVideoURL,
		"transcript": "0:00\n" + testTranscriptMarker + " failing\n",
		"mode":       modePlain,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var responses []string
	accepted := ts.startBody(string(body))
	responses = append(responses, accepted.Body.String())
	key := acceptedKey(t, accepted)

	done := ts.statusRequest(key)
	var resp statusResponse
	if err := json.Unmarshal(done.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Job.Article == nil || !strings.Contains(resp.Job.Article.Markdown, testTranscriptMarker) {
		t.Fatalf("the done article does not carry its title: %+v", resp.Job.Article)
	}
	// The heading shows the video title the start sent, and the article's
	// channel name, never the start's.
	if resp.Job.Article.VideoTitle != testTranscriptMarker || resp.Job.Article.ChannelName != article.ChannelName {
		t.Fatalf("heading = %q, %q; want the start's title and the article's channel name", resp.Job.Article.VideoTitle, resp.Job.Article.ChannelName)
	}
	// The article's own strings may hold anything the article holds.
	resp.Job.Article.Markdown, resp.Job.Article.File = "", ""
	resp.Job.Article.VideoTitle = ""
	rest, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	responses = append(responses, string(rest))

	// An inquiry start for the same video, sent in the short URL form with
	// other text, is answered with the done article's key.
	inquiry, err := json.Marshal(map[string]string{
		"videoUrl":    sentURL,
		"transcript":  "0:00\n" + testTranscriptMarker + " inquiry\n",
		"title":       testTranscriptMarker,
		"channelName": testMarker,
		"description": testTranscriptMarker,
		"mode":        modeInquiry,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	sameVideo := ts.startBody(string(inquiry))
	if got, _ := sameVideoAnswer(t, sameVideo); got != key {
		t.Fatalf("same-video key = %s, want %s", got, key)
	}
	responses = append(responses, sameVideo.Body.String())

	ts.writer.Err = errors.New(errorMarker + " " + testMarker)
	failed := ts.startBody(string(failing))
	responses = append(responses, failed.Body.String(), ts.statusRequest(acceptedKey(t, failed)).Body.String())

	responses = append(responses,
		ts.startBody(`{"videoUrl":"`+testTranscriptMarker+`","transcript":"x","mode":"plain"}`).Body.String(),
		ts.startBody(validBody(testValidVideoURL, testTranscriptMarker)).Body.String(),
		ts.statusRequest(testTranscriptMarker).Body.String(),
	)

	markers := []string{
		testTranscriptMarker, testMarker, errorMarker, testVideoID,
		hex.EncodeToString(testKeySecret[:]), string(testKeySecret[:]),
	}
	for _, response := range responses {
		for _, marker := range markers {
			if strings.Contains(response, marker) {
				t.Errorf("response %s carries %q", response, marker)
			}
		}
	}
	logs := ts.logs.String()
	if !strings.Contains(logs, "outcome=same_video") {
		t.Fatalf("the log has no same-video line, so the check below proves nothing about it:\n%s", logs)
	}
	for _, marker := range []string{testTranscriptMarker, testMarker, hex.EncodeToString(testKeySecret[:]), sentURL, testValidVideoURL} {
		if strings.Contains(logs, marker) {
			t.Errorf("the log carries %q:\n%s", marker, logs)
		}
	}
}
