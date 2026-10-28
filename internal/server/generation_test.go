//go:build test

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// TestGenerationStages checks that while a generation job runs, the status
// response reports running and moves each step from pending through running to
// done as the writer reports it, with the shorten step skipped for an article
// that needs no shortening, and that the job ends done.
func TestGenerationStages(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	type point struct {
		kind  writertestutil.FakeProgressKind
		stage writer.Stage
		want  statusSteps
	}
	points := []point{
		{writertestutil.FakeStageStarted, writer.StageGeneration, statusSteps{"running", "pending", "pending"}},
		{writertestutil.FakeStageFinished, writer.StageGeneration, statusSteps{"done", "pending", "pending"}},
		{writertestutil.FakeStageSkipped, writer.StageShorten, statusSteps{"done", "skipped", "pending"}},
		{writertestutil.FakeStageStarted, writer.StageReview, statusSteps{"done", "skipped", "running"}},
		{writertestutil.FakeStageFinished, writer.StageReview, statusSteps{"done", "skipped", "done"}},
	}
	reached := make([]chan struct{}, len(points))
	release := make([]chan struct{}, len(points))
	for i, p := range points {
		reached[i], release[i] = make(chan struct{}), make(chan struct{})
		ts.writer.Progress = append(ts.writer.Progress, writertestutil.FakeProgressStep{
			Kind: p.kind, Stage: p.stage, Reached: reached[i], Release: release[i],
		})
	}

	key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
	if got := ts.statusOf(t, key).Job; got.State != "running" || *got.Steps != (statusSteps{"pending", "pending", "pending"}) {
		t.Errorf("before the first step: job = %+v, steps %+v, want running with every step pending", got, got.Steps)
	}
	for i, p := range points {
		<-reached[i]
		got := ts.statusOf(t, key).Job
		if got.State != "running" || got.Steps == nil || *got.Steps != p.want {
			t.Errorf("after notification %d: state %q steps %+v, want running %+v", i, got.State, got.Steps, p.want)
		}
		close(release[i])
	}
	ts.waitJobs()
	got := ts.statusOf(t, key).Job
	if got.State != "done" || *got.Steps != (statusSteps{"done", "skipped", "done"}) {
		t.Errorf("after the job: state %q steps %+v, want done with draft and review done and shorten skipped", got.State, got.Steps)
	}
	if len(ts.slots) != 0 {
		t.Errorf("generation slots held = %d, want 0 after the job", len(ts.slots))
	}
}

// TestGenerationFailedStep checks that a generation that fails in a step is
// reported failed, with its reason, the failed step marked failed, the steps
// before it as they were, and no article or publish state.
func TestGenerationFailedStep(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.writer.Progress = []writertestutil.FakeProgressStep{
		{Kind: writertestutil.FakeStageStarted, Stage: writer.StageGeneration},
		{Kind: writertestutil.FakeStageFinished, Stage: writer.StageGeneration},
		{Kind: writertestutil.FakeStageSkipped, Stage: writer.StageShorten},
		{Kind: writertestutil.FakeStageStarted, Stage: writer.StageReview},
	}
	ts.writer.Err = llm.ErrTruncated
	key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
	ts.waitJobs()

	got := ts.statusOf(t, key).Job
	want := statusJob{State: "failed", Steps: &statusSteps{"done", "skipped", "failed"}, Failure: "llm_truncated"}
	if got.State != want.State || got.Failure != want.Failure || got.Steps == nil || *got.Steps != *want.Steps {
		t.Errorf("job = %+v steps %+v, want %+v steps %+v", got, got.Steps, want, want.Steps)
	}
	if got.Article != nil || got.Publish != nil {
		t.Errorf("a failed job carries article %+v and publish %+v, want neither", got.Article, got.Publish)
	}
	if line := logLine(ts.logs.String(), "generation job failed"); !strings.Contains(line, "failure=llm_truncated") || !strings.Contains(line, "step=review") {
		t.Errorf("the failure log line = %q, want failure=llm_truncated and step=review", line)
	}
	if len(ts.slots) != 0 {
		t.Errorf("generation slots held = %d, want 0 after the job", len(ts.slots))
	}
}

// TestGenerationUnpublishable checks that an article the publishers would
// refuse to render fails the generation as article_invalid, with no article.
func TestGenerationUnpublishable(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	article := testArticle()
	article.Title = ""
	ts.writer.Result = article
	key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
	ts.waitJobs()

	got := ts.statusOf(t, key).Job
	if got.State != "failed" || got.Failure != "article_invalid" {
		t.Errorf("job = %+v, want failed with article_invalid", got)
	}
	if got.Article != nil {
		t.Errorf("a failed job carries an article: %+v", got.Article)
	}
}

// TestGenerationUnpostable checks that an article the Slack pre-send checks
// refuse is done, with its article, and unpostable with the reason the check
// gives: mention for a mention, other for a body that cannot be split. The
// check runs the same with and without a Webhook.
func TestGenerationUnpostable(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		reason string
	}{
		{"mention", "Hello <!channel>.\n", "mention"},
		{"unsplittable", strings.Repeat(" ", 20000) + "x\n", "other"},
	}
	for _, tc := range tests {
		for _, noWebhook := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/noWebhook=%v", tc.name, noWebhook), func(t *testing.T) {
				ts := newJobsTestServer(t, jobsServerOptions{noWebhook: noWebhook})
				article := testArticle()
				article.Body = tc.body
				ts.writer.Result = article
				key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
				ts.waitJobs()

				got := ts.statusOf(t, key).Job
				if got.State != "done" || got.Article == nil {
					t.Fatalf("job = %+v, want done with the article", got)
				}
				if got.Publish == nil || *got.Publish != (statusPublish{State: "unpostable", Reason: tc.reason}) {
					t.Errorf("publish = %+v, want unpostable with reason %s", got.Publish, tc.reason)
				}
				if line := logLine(ts.logs.String(), "generation job succeeded"); !strings.Contains(line, "publish=unpostable") {
					t.Errorf("the success log line = %q, want publish=unpostable", line)
				}
			})
		}
	}
}

// TestGenerationKeepsNoTranscript checks that once a generation ends, no record
// holds the copied transcript: not after a done article whose review revisions
// quote it, and not after a failure whose error carries it.
func TestGenerationKeepsNoTranscript(t *testing.T) {
	const marker = "TRANSCRIPTMARKER"
	body := validBody(testValidVideoURL, "0:00\n"+marker+"\n")

	t.Run("done", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		article := testArticle()
		article.Review = &writer.ReviewResult{
			Model: writer.StepModel{Model: "review-model"},
			Revisions: []writer.Revision{{
				Before: "Body", Action: writer.ActionReplace, After: "Text",
				Reason: writer.ReasonUnsupported, Evidence: marker,
			}},
		}
		ts.writer.Result = article
		key := ts.beginGeneration(t, body)
		ts.waitJobs()
		if got := ts.statusOf(t, key).Job.State; got != "done" {
			t.Fatalf("job state = %q, want done", got)
		}
		if dump := ts.cache.DumpRecordsForTest(); strings.Contains(dump, marker) {
			t.Errorf("a record holds the transcript text:\n%s", dump)
		}
		// The writer did receive the transcript, so the check above can fail.
		if calls := ts.writer.RecordedCalls(); len(calls) != 1 || !strings.Contains(calls[0].Transcript.Segments[0].Text, marker) {
			t.Fatalf("the writer did not receive the transcript text: %+v", calls)
		}
	})
	t.Run("failed", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		ts.writer.Err = fmt.Errorf("generation failed on %q", marker)
		key := ts.beginGeneration(t, body)
		ts.waitJobs()
		if got := ts.statusOf(t, key).Job.State; got != "failed" {
			t.Fatalf("job state = %q, want failed", got)
		}
		if dump := ts.cache.DumpRecordsForTest(); strings.Contains(dump, marker) {
			t.Errorf("a record holds the transcript text:\n%s", dump)
		}
	})
}

// TestGenerationFailureReasons checks that each generation failure the writer
// or the completion check can produce is reported with its own reason code, and
// that the eight codes differ.
func TestGenerationFailureReasons(t *testing.T) {
	invalid := testArticle()
	invalid.Model = ""
	tooLarge := testArticle()
	tooLarge.Body = strings.Repeat("a", maxStatusResponseBytes) + "\n"
	tests := []struct {
		name    string
		err     error
		article writer.Article
		want    string
	}{
		{"prompt_too_large", writer.ErrPromptTooLarge, writer.Article{}, "transcript_too_long"},
		{"llm_timeout", context.DeadlineExceeded, writer.Article{}, "llm_timeout"},
		{"llm_empty", llm.ErrEmptyResponse, writer.Article{}, "llm_empty"},
		{"llm_truncated", llm.ErrTruncated, writer.Article{}, "llm_truncated"},
		{"llm_other", errors.New("the provider refused"), writer.Article{}, "llm_failed"},
		{"internal", writer.ErrInvalidTemplate, writer.Article{}, "internal"},
		{"article_invalid", nil, invalid, "article_invalid"},
		{"article_too_large", nil, tooLarge, "article_too_large"},
	}
	seen := map[string]string{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			ts.writer.Err = tc.err
			ts.writer.Result = tc.article
			key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
			ts.waitJobs()
			got := ts.statusOf(t, key).Job
			if got.State != "failed" || got.Failure != tc.want {
				t.Errorf("job = %+v, want failed with %s", got, tc.want)
			}
			if other, dup := seen[got.Failure]; dup {
				t.Errorf("reason %q is shared with %s", got.Failure, other)
			}
			seen[got.Failure] = tc.name
		})
	}
}

// TestGenerationPanic checks that a panic in the generation job fails the
// record as internal and releases the generation slot.
func TestGenerationPanic(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.Server.writer = panickingWriter{}
	key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
	ts.waitJobs()
	if got := ts.statusOf(t, key).Job; got.State != "failed" || got.Failure != "internal" {
		t.Errorf("job = %+v, want failed with internal", got)
	}
	if len(ts.slots) != 0 {
		t.Errorf("generation slots held = %d, want 0", len(ts.slots))
	}
}

// TestStatusResponseWithinLimit checks the size limit at its boundary: an
// article whose largest status response is exactly maxStatusResponseBytes is
// done, one a byte larger fails as article_too_large, and the done article's
// status responses fit the limit in every publish state and reason a done
// record can hold, with the values the server varies at their longest.
func TestStatusResponseWithinLimit(t *testing.T) {
	at, over := articlesAtStatusLimit(t)

	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.writer.Result = at
	key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
	ts.waitJobs()
	snap := ts.cache.Lookup(key)
	if snap.State != articlecache.JobDone || snap.Publish != articlecache.PublishNotPosted {
		t.Fatalf("the article at the limit: state %d publish %d, want done and not posted", snap.State, snap.Publish)
	}

	other := newJobsTestServer(t, jobsServerOptions{})
	other.writer.Result = over
	overKey := other.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
	other.waitJobs()
	if got := other.statusOf(t, overKey).Job; got.State != "failed" || got.Failure != "article_too_large" {
		t.Errorf("the article a byte over: job = %+v, want failed with article_too_large", got)
	}

	snap.Duration = 1<<63 - 1
	snap.Stages = articlecache.Stages{Draft: articlecache.StageSkipped, Shorten: articlecache.StageSkipped, Review: articlecache.StageSkipped}
	states := []struct {
		publish    articlecache.PublishState
		failure    articlecache.PublishFailureReason
		unpostable articlecache.UnpostableReason
	}{
		{publish: articlecache.PublishNotPosted},
		{publish: articlecache.PublishPosting},
		{publish: articlecache.PublishFailed, failure: articlecache.PublishFailureInternal},
		{publish: articlecache.PublishFailed, failure: articlecache.PublishFailureWebhook},
		{publish: articlecache.PublishPosted},
		{publish: articlecache.PublishUnpostable, unpostable: articlecache.UnpostableOther},
		{publish: articlecache.PublishUnpostable, unpostable: articlecache.UnpostableMention},
	}
	for _, st := range states {
		snap.Publish, snap.PublishFailure, snap.Unpostable = st.publish, st.failure, st.unpostable
		data, err := renderStatus(strings.Repeat("f", 2*requestIDBytes), false, snap)
		if err != nil {
			t.Fatalf("renderStatus: %v", err)
		}
		if len(data) > maxStatusResponseBytes {
			t.Errorf("publish %+v: response is %d bytes, over the %d limit", st, len(data), maxStatusResponseBytes)
		}
	}
}

// articlesAtStatusLimit returns a postable article whose largest status
// response is exactly maxStatusResponseBytes, and the same article a byte
// larger. The body, which the response holds twice, takes most of the room in
// lines short enough to split; the model, which it holds once, takes the last
// byte.
func articlesAtStatusLimit(t *testing.T) (at, over writer.Article) {
	t.Helper()
	size := func(a writer.Article) int {
		t.Helper()
		n, err := maxStatusSize(a)
		if err != nil {
			t.Fatalf("maxStatusSize: %v", err)
		}
		return n
	}
	at = testArticle()
	at.Body = strings.Repeat(strings.Repeat("a", 999)+"\n", 120)
	base := size(at)
	at.Body += strings.Repeat("a", (maxStatusResponseBytes-base)/2)
	at.Model += strings.Repeat("m", maxStatusResponseBytes-size(at))
	if got := size(at); got != maxStatusResponseBytes {
		t.Fatalf("the article's largest status response is %d bytes, want exactly %d", got, maxStatusResponseBytes)
	}
	over = at
	over.Model += "m"
	if got := size(over); got != maxStatusResponseBytes+1 {
		t.Fatalf("the larger article's largest status response is %d bytes, want %d", got, maxStatusResponseBytes+1)
	}
	return at, over
}

// logLine returns the first line of logs that contains msg, or "".
func logLine(logs, msg string) string {
	for line := range strings.Lines(logs) {
		if strings.Contains(line, msg) {
			return line
		}
	}
	return ""
}
