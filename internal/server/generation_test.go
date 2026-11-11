//go:build test

package server

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
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

	// Begin and start the job by hand, so the record is read before the job
	// can report anything.
	input := ts.validInput(t, validBody(testValidVideoURL, testValidRaw))
	subject := ts.keyer.Subject(input.key)
	key := subject.Key()
	run := ts.cache.Begin(subject, articlecache.StartPlain, ts.acquireSlot).Run
	if got := ts.statusOf(t, key).Job; got.State != "running" || got.Steps == nil || *got.Steps != (statusSteps{"pending", "pending", "pending"}) {
		t.Errorf("before the first step: job = %+v, steps %+v, want running with every step pending", got, got.Steps)
	}
	ts.startGeneration(run, key, input)
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
	if got.State != "done" || got.Steps == nil || *got.Steps != (statusSteps{"done", "skipped", "done"}) {
		t.Errorf("after the job: state %q steps %+v, want done with draft and review done and shorten skipped", got.State, got.Steps)
	}
	if len(ts.slots) != 0 {
		t.Errorf("generation slots held = %d, want 0 after the job", len(ts.slots))
	}
}

// TestGenerationFailedStep checks that a generation that fails in a step is
// reported failed, with its reason, the failed step marked failed, the steps
// before it as they were, and no article.
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
	if got.Article != nil {
		t.Errorf("a failed job carries article %+v, want none", got.Article)
	}
	if line := logLine(ts.logs.String(), "generation job failed"); !strings.Contains(line, "failure=llm_truncated") || !strings.Contains(line, "step=review") {
		t.Errorf("the failure log line = %q, want failure=llm_truncated and step=review", line)
	}
	if len(ts.slots) != 0 {
		t.Errorf("generation slots held = %d, want 0 after the job", len(ts.slots))
	}
}

// TestGenerationUnpublishable checks that an article the publishers would
// refuse to render fails the generation as article_invalid, with no article: an
// empty title, and a body with invalid UTF-8 that also carries a marker, whose
// bytes reach neither the status nor the failure log line.
func TestGenerationUnpublishable(t *testing.T) {
	const mark = "mark-body-V7dq"
	cases := []struct {
		name   string
		mutate func(a *writer.Article)
	}{
		{"title empty", func(a *writer.Article) { a.Title = "" }},
		{"body invalid UTF-8", func(a *writer.Article) { a.Body = mark + "\xff" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			article := testArticle()
			tc.mutate(&article)
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
			line := logLine(ts.logs.String(), "generation job failed")
			if line == "" {
				t.Fatalf("no generation job failed line in the log")
			}
			if strings.Contains(line, mark) {
				t.Errorf("the failure log line holds the body's text: %q", line)
			}
		})
	}
}

// TestGenerationMentionsDone checks that an article is done, with its
// article, whatever mentions it holds or however long its body is: a body
// with a Slack mention and a body longer than one Slack message could carry
// both complete. The success log line carries exactly the key, the video ID,
// the model, its version, and the duration.
func TestGenerationMentionsDone(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"mention", "Hello <!channel>.\n"},
		{"longer than one post", strings.Repeat(" ", 20000) + "x\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			article := testArticle()
			article.Body = tc.body
			ts.writer.Result = article
			key := ts.beginGeneration(t, validBody(testValidVideoURL, testValidRaw))
			ts.waitJobs()

			got := ts.statusOf(t, key).Job
			if got.State != "done" || got.Article == nil {
				t.Fatalf("job = %+v, want done with the article", got)
			}
			line := logLine(ts.logs.String(), "generation job succeeded")
			if line == "" {
				t.Fatalf("no generation job succeeded line in the log:\n%s", ts.logs.String())
			}
			want := []string{"duration", "key", "model", "modelVersion", "videoId"}
			if attrs := logAttrNames(line); !slices.Equal(attrs, want) {
				t.Errorf("the success log line's attributes = %v, want %v: %q", attrs, want, line)
			}
		})
	}
}

// TestGenerationOutcomeAfterRecordMovedOn checks that a generation job whose
// Run is no longer valid when it reports, because its record failed and was
// restarted meanwhile, records nothing and logs that the record moved on,
// whether it ends done or failed: the restarted record keeps running.
func TestGenerationOutcomeAfterRecordMovedOn(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"done", nil},
		{"failed", llm.ErrTruncated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			ts.writer.Err = tc.err
			input := ts.validInput(t, validBody(testValidVideoURL, testValidRaw))
			subject := ts.keyer.Subject(input.key)
			stale := ts.cache.Begin(subject, articlecache.StartPlain, ts.acquireSlot).Run
			stale.Fail(articlecache.FailureInternal)
			if again := ts.cache.Begin(subject, articlecache.StartPlain, ts.acquireSlot); again.Outcome != articlecache.BeginRestarted {
				t.Fatalf("Begin outcome = %d, want restarted", again.Outcome)
			}
			ts.startGeneration(stale, subject.Key(), input)
			ts.waitJobs()

			if got := ts.cache.Lookup(subject.Key()).State; got != articlecache.JobRunning {
				t.Errorf("the restarted record's state = %d, want running", got)
			}
			if line := logLine(ts.logs.String(), "the record moved on"); line == "" {
				t.Errorf("no record-moved-on line in the log:\n%s", ts.logs.String())
			}
			for _, msg := range []string{"generation job succeeded", "generation job failed"} {
				if line := logLine(ts.logs.String(), msg); line != "" {
					t.Errorf("the stale job logged %q: %s", msg, line)
				}
			}
		})
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
	// The record keeps a failure's reason, not its error, so this subtest
	// guards the record's shape against a later change that stores the error.
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
// that each row's code is its own.
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
// status response fits the limit with the values the server varies at their
// longest. The
// second case fills the body and the start's video title with '@' units, so
// the bytes the neutralization adds and the title must both be counted.
func TestStatusResponseWithinLimit(t *testing.T) {
	cases := []struct {
		name       string
		videoTitle string // "" sends no title member
		line       string // the body starts with lines copies of line
		lines      int
	}{
		{"plain", "", strings.Repeat("a", 999) + "\n", 120},
		{"mentions and a long title", strings.Repeat("@t ", 1000), strings.Repeat("@a ", 300) + "\n", 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at, over := articlesAtStatusLimit(t, tc.videoTitle, strings.Repeat(tc.line, tc.lines))
			body := validBody(testValidVideoURL, testValidRaw)
			if tc.videoTitle != "" {
				body = `{"videoUrl":` + strconv.Quote(testValidVideoURL) + `,"transcript":` + strconv.Quote(testValidRaw) +
					`,"title":` + strconv.Quote(tc.videoTitle) + `,"mode":"plain"}`
			}

			ts := newJobsTestServer(t, jobsServerOptions{})
			ts.writer.Result = at
			key := ts.beginGeneration(t, body)
			ts.waitJobs()
			snap := ts.cache.Lookup(key)
			if snap.State != articlecache.JobDone {
				t.Fatalf("the article at the limit: state %d, want done", snap.State)
			}

			other := newJobsTestServer(t, jobsServerOptions{})
			other.writer.Result = over
			overKey := other.beginGeneration(t, body)
			other.waitJobs()
			if got := other.statusOf(t, overKey).Job; got.State != "failed" || got.Failure != "article_too_large" {
				t.Errorf("the article a byte over: job = %+v, want failed with article_too_large", got)
			}

			snap.Duration = 1<<63 - 1
			snap.Stages = articlecache.Stages{Draft: articlecache.StageSkipped, Shorten: articlecache.StageSkipped, Review: articlecache.StageSkipped}
			// The values the server varies are this test's own choice of their
			// longest.
			data, err := renderStatus(strings.Repeat("f", 2*requestIDBytes), snap)
			if err != nil {
				t.Fatalf("renderStatus: %v", err)
			}
			if len(data) > maxStatusResponseBytes {
				t.Errorf("response is %d bytes, over the %d limit", len(data), maxStatusResponseBytes)
			}
		})
	}
}

// articlesAtStatusLimit returns an article whose largest status response,
// started with videoTitle, is exactly maxStatusResponseBytes, and the same
// article a byte larger. The body, which the response holds twice, takes most
// of the room: start, then 'a's; the model, which it holds once, takes the
// last byte.
func articlesAtStatusLimit(t *testing.T, videoTitle, start string) (at, over writer.Article) {
	t.Helper()
	size := func(a writer.Article) int {
		t.Helper()
		n, err := maxStatusSize(a, videoTitle)
		if err != nil {
			t.Fatalf("maxStatusSize: %v", err)
		}
		return n
	}
	at = testArticle()
	at.Body = start
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

// logAttrName matches the name of one attribute of a slog text line.
var logAttrName = regexp.MustCompile(`(?:^| )([A-Za-z][A-Za-z0-9]*)=`)

// logAttrNames returns the sorted names of a slog text line's attributes,
// without the time, level, and msg the handler adds. It reads the lines these
// tests write, whose quoted values (the message) hold no word directly
// followed by "=".
func logAttrNames(line string) []string {
	var names []string
	for _, m := range logAttrName.FindAllStringSubmatch(line, -1) {
		switch m[1] {
		case "time", "level", "msg":
		default:
			names = append(names, m[1])
		}
	}
	slices.Sort(names)
	return names
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
