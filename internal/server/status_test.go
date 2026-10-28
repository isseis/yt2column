//go:build test

package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
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

// TestStatusDoneArticle checks a done article's status: the display Markdown
// and the file content are what the publisher renders for the article (the
// file being what --out writes), the cost is the formatted known cost, and
// the post has not started.
func TestStatusDoneArticle(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	article := testArticle()
	article.RunUsage = pricedUsage()
	ts.writer.Result = article
	key := acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))

	job := ts.getStatus(t, key).Job
	if job.State != "done" || job.Article == nil || job.Publish == nil {
		t.Fatalf("job = %+v, want done with an article and a publish state", job)
	}
	wantMarkdown, err := publisher.DisplayMarkdown(article)
	if err != nil {
		t.Fatalf("DisplayMarkdown: %v", err)
	}
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
	if *job.Publish != (statusPublish{State: "not_posted"}) {
		t.Errorf("publish = %+v, want not_posted", *job.Publish)
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
// call no writer, publisher, or notifier.
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
	if len(ts.writer.Calls) != 1 || len(ts.publisher.Calls) != 0 || len(ts.publisher.PostCalls) != 0 || len(ts.notifier.Calls) != 0 {
		t.Errorf("side effects: writer=%d publish=%d post=%d notifier=%d, want only the one generation",
			len(ts.writer.Calls), len(ts.publisher.Calls), len(ts.publisher.PostCalls), len(ts.notifier.Calls))
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

// TestResponsesOmitInputs checks that no job API response, a rejection
// included, carries a request value, an error text, or a secret, apart from
// the done article's Markdown and file, and that the log carries neither the
// transcript nor a secret.
func TestResponsesOmitInputs(t *testing.T) {
	const errorMarker = "MARKER-ERROR-TEXT"
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
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	failing, err := json.Marshal(map[string]string{
		"videoUrl":   testValidVideoURL,
		"transcript": "0:00\n" + testTranscriptMarker + " failing\n",
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
	// The article's own strings may hold anything the article holds.
	resp.Job.Article.Markdown, resp.Job.Article.File = "", ""
	rest, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	responses = append(responses, string(rest))

	ts.writer.Err = errors.New(errorMarker + " " + testMarker)
	failed := ts.startBody(string(failing))
	responses = append(responses, failed.Body.String(), ts.statusRequest(acceptedKey(t, failed)).Body.String())

	responses = append(responses,
		ts.startBody(`{"videoUrl":"`+testTranscriptMarker+`","transcript":"x"}`).Body.String(),
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
	for _, marker := range []string{testTranscriptMarker, testMarker, hex.EncodeToString(testKeySecret[:])} {
		if strings.Contains(logs, marker) {
			t.Errorf("the log carries %q:\n%s", marker, logs)
		}
	}
}

// TestStatusWebhookConfigured checks that a server with a Webhook reports it
// and a done article as not posted, and that a server without one runs the
// same start and reports the same job, with webhookConfigured false.
func TestStatusWebhookConfigured(t *testing.T) {
	clock := newTestClock()
	jobs := map[bool]statusJob{}
	for _, configured := range []bool{true, false} {
		ts := newJobsTestServer(t, jobsServerOptions{clock: clock.Now, noWebhook: !configured})
		key := acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))
		resp := ts.getStatus(t, key)
		if resp.WebhookConfigured != configured {
			t.Errorf("webhookConfigured = %v, want %v", resp.WebhookConfigured, configured)
		}
		if resp.Job.Publish == nil || resp.Job.Publish.State != "not_posted" {
			t.Errorf("configured=%v: publish = %+v, want not_posted", configured, resp.Job.Publish)
		}
		jobs[configured] = resp.Job
	}
	if !reflect.DeepEqual(jobs[true], jobs[false]) {
		t.Errorf("the job differs without a Webhook:\nwith    %+v\nwithout %+v", jobs[true], jobs[false])
	}
}
