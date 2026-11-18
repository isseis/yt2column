//go:build test

package extjob

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/devfake"
	llmtestutil "github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

var (
	started = func(s writer.Stage) writertestutil.FakeProgressStep {
		return writertestutil.FakeProgressStep{Kind: writertestutil.FakeStageStarted, Stage: s}
	}
	finished = func(s writer.Stage) writertestutil.FakeProgressStep {
		return writertestutil.FakeProgressStep{Kind: writertestutil.FakeStageFinished, Stage: s}
	}
	skipped = func(s writer.Stage) writertestutil.FakeProgressStep {
		return writertestutil.FakeProgressStep{Kind: writertestutil.FakeStageSkipped, Stage: s}
	}
)

// runWith drives run with a fake writer.
func runWith(t *testing.T, w *writertestutil.FakeArticleWriter, obs Observer) Result {
	t.Helper()
	return run(context.Background(), mustPrepare(t, StartInput{}), fakeWriterFactory(w), time.Now, obs)
}

// TestRunStages ports TestGenerationStages: the observer receives each step
// change, in the order the writer reports it, moving every step from pending
// through running to done, with the shorten step skipped for an article that
// needs no shortening; the run ends done with those steps.
func TestRunStages(t *testing.T) {
	w := &writertestutil.FakeArticleWriter{
		Result: testArticle(),
		Progress: []writertestutil.FakeProgressStep{
			started(writer.StageGeneration), finished(writer.StageGeneration),
			skipped(writer.StageShorten),
			started(writer.StageReview), finished(writer.StageReview),
		},
	}
	obs := &recordingObserver{}
	result := runWith(t, w, obs)
	want := []Steps{
		{Draft: StepRunning},
		{Draft: StepDone},
		{Draft: StepDone, Shorten: StepSkipped},
		{Draft: StepDone, Shorten: StepSkipped, Review: StepRunning},
		{Draft: StepDone, Shorten: StepSkipped, Review: StepDone},
	}
	if len(obs.changes) != len(want) {
		t.Fatalf("changes = %v, want %v", obs.changes, want)
	}
	for i := range want {
		if obs.changes[i] != want[i] {
			t.Errorf("change %d = %+v, want %+v", i, obs.changes[i], want[i])
		}
	}
	if _, ok := result.Article(); !ok || result.Steps() != want[len(want)-1] {
		t.Errorf("result = %+v, want done with the last steps", result)
	}
	if _, failed := result.Failure(); failed {
		t.Error("a done run reports a failure")
	}
}

// TestRunFailedStep ports TestGenerationFailedStep: a run that fails in a step
// is failed with its reason, the failed step marked failed, the steps before it
// as they were, and no article.
func TestRunFailedStep(t *testing.T) {
	w := &writertestutil.FakeArticleWriter{
		Err: llm.ErrTruncated,
		Progress: []writertestutil.FakeProgressStep{
			started(writer.StageGeneration), finished(writer.StageGeneration),
			skipped(writer.StageShorten), started(writer.StageReview),
		},
	}
	result := runWith(t, w, nil)
	failure, failed := result.Failure()
	if !failed || failure != FailureLLMTruncated {
		t.Errorf("failure = %v, %v; want llm_truncated", failure, failed)
	}
	if want := (Steps{Draft: StepDone, Shorten: StepSkipped, Review: StepFailed}); result.Steps() != want {
		t.Errorf("steps = %+v, want %+v", result.Steps(), want)
	}
	if _, ok := result.Article(); ok {
		t.Error("a failed run carries an article")
	}
}

// TestRunUnpublishable ports TestGenerationUnpublishable: an article the
// publishers would refuse to render fails the run as article_invalid with no
// article, and the body's bytes reach no result JSON.
func TestRunUnpublishable(t *testing.T) {
	const mark = "mark-body-V7dq"
	cases := map[string]func(a *writer.Article){
		"title empty":        func(a *writer.Article) { a.Title = "" },
		"body invalid UTF-8": func(a *writer.Article) { a.Body = mark + "\xff" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			article := testArticle()
			mutate(&article)
			result := runWith(t, &writertestutil.FakeArticleWriter{Result: article}, nil)
			if failure, failed := result.Failure(); !failed || failure != FailureArticleInvalid {
				t.Errorf("failure = %v, %v; want article_invalid", failure, failed)
			}
			data, err := marshalResult(result)
			if err != nil {
				t.Fatalf("marshalResult: %v", err)
			}
			if strings.Contains(string(data), mark) {
				t.Errorf("the result JSON holds the body's text: %s", data)
			}
		})
	}
}

// TestRunMentionsDone ports TestGenerationMentionsDone: an article is done
// whatever mentions it holds or however long its body is.
func TestRunMentionsDone(t *testing.T) {
	for name, body := range map[string]string{
		"mention":              "Hello <!channel>.\n",
		"longer than one post": strings.Repeat(" ", 20000) + "x\n",
	} {
		t.Run(name, func(t *testing.T) {
			article := testArticle()
			article.Body = body
			if _, ok := runWith(t, &writertestutil.FakeArticleWriter{Result: article}, nil).Article(); !ok {
				t.Error("the article is not done")
			}
		})
	}
}

// TestRunFailureReasons ports TestGenerationFailureReasons: each failure the
// writer or the completion check can produce is reported with its own
// reason, and each row's code is its own. The rejected key row is new.
func TestRunFailureReasons(t *testing.T) {
	invalid := testArticle()
	invalid.Model = ""
	tooLarge := testArticle()
	tooLarge.Body = strings.Repeat("a", MaxResultBytesForTest) + "\n"
	tests := []struct {
		name    string
		err     error
		article writer.Article
		want    Failure
		code    string
	}{
		{"prompt_too_large", writer.ErrPromptTooLarge, writer.Article{}, FailureTranscriptTooLong, "transcript_too_long"},
		{"llm_timeout", context.DeadlineExceeded, writer.Article{}, FailureLLMTimeout, "llm_timeout"},
		{"llm_empty", llm.ErrEmptyResponse, writer.Article{}, FailureLLMEmpty, "llm_empty"},
		{"llm_truncated", llm.ErrTruncated, writer.Article{}, FailureLLMTruncated, "llm_truncated"},
		{"api_key_rejected", llm.ErrAuthRejected, writer.Article{}, FailureAPIKeyRejected, "api_key_rejected"},
		{"llm_other", errors.New("the provider refused"), writer.Article{}, FailureLLM, "llm_failed"},
		{"internal", writer.ErrInvalidTemplate, writer.Article{}, FailureInternal, "internal"},
		{"article_invalid", nil, invalid, FailureArticleInvalid, "article_invalid"},
		{"article_too_large", nil, tooLarge, FailureArticleTooLarge, "article_too_large"},
	}
	seen := map[string]bool{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := runWith(t, &writertestutil.FakeArticleWriter{Err: tc.err, Result: tc.article}, nil)
			failure, failed := result.Failure()
			if !failed || failure != tc.want || failure.String() != tc.code {
				t.Errorf("failure = %v (%s), %v; want %s", failure, failure, failed, tc.code)
			}
		})
		if seen[tc.code] {
			t.Errorf("code %s is used by two rows", tc.code)
		}
		seen[tc.code] = true
	}
}

// panickingWriter panics in WriteWithProgress after the generation step has
// started.
type panickingWriter struct{}

func (panickingWriter) Write(context.Context, transcript.Transcript) (writer.Article, error) {
	panic("boom")
}

func (panickingWriter) WriteWithProgress(_ context.Context, _ transcript.Transcript, obs writer.ProgressObserver) (writer.Article, error) {
	obs.StageStarted(writer.StageGeneration)
	panic("secret-in-panic")
}

// TestRunPanic ports TestGenerationPanic: a panic in the run fails it as
// internal with the panic detail, the running step marked failed, and the
// panic's value nowhere in the result.
func TestRunPanic(t *testing.T) {
	result := run(context.Background(), mustPrepare(t, StartInput{}), fakeWriterFactory(panickingWriter{}), time.Now, nil)
	failure, failed := result.Failure()
	if !failed || failure != FailureInternal || result.InternalDetail() != DetailPanic {
		t.Errorf("result = %v %v detail %q, want internal with the panic detail", failure, failed, result.InternalDetail())
	}
	if result.Steps().Draft != StepFailed {
		t.Errorf("steps = %+v, want the draft step failed", result.Steps())
	}
	data, err := marshalResult(result)
	if err != nil {
		t.Fatalf("marshalResult: %v", err)
	}
	if strings.Contains(string(data), "secret-in-panic") {
		t.Errorf("the result holds the panic value: %s", data)
	}
}

// TestRunDuration checks the duration runs from the start of the run to its
// end by the injected clock.
func TestRunDuration(t *testing.T) {
	now := time.Unix(1000, 0)
	calls := 0
	clock := func() time.Time {
		calls++
		if calls == 2 {
			now = now.Add(5 * time.Second)
		}
		return now
	}
	result := run(context.Background(), mustPrepare(t, StartInput{}),
		fakeWriterFactory(&writertestutil.FakeArticleWriter{Result: testArticle()}), clock, nil)
	article, ok := result.Article()
	if !ok || article.DurationMs != 5000 {
		t.Errorf("duration = %d ms (done %v), want 5000", article.DurationMs, ok)
	}
}

// TestRunAuthRejectedAtEveryStep runs the real Run with the fake clients and
// checks a rejected key at each of the three calls (the draft, the shorten
// step, and the review) is api_key_rejected with that step failed.
func TestRunAuthRejectedAtEveryStep(t *testing.T) {
	tests := []struct {
		name    string
		article devfake.Article
		failAt  devfake.FailAt
		want    Steps
	}{
		{"draft", devfake.ArticleStandard, devfake.FailAtDraft, Steps{Draft: StepFailed}},
		{"shorten", devfake.ArticleLong, devfake.FailAtShorten, Steps{Draft: StepDone, Shorten: StepFailed}},
		{"review", devfake.ArticleStandard, devfake.FailAtReview, Steps{Draft: StepDone, Shorten: StepSkipped, Review: StepFailed}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scenario, err := devfake.NewScenario(tc.article, devfake.FailureAPIKeyRejected, tc.failAt, 0)
			if err != nil {
				t.Fatalf("NewScenario: %v", err)
			}
			job := scenario.NewJob()
			result := Run(context.Background(), mustPrepare(t, StartInput{}), Deps{Generation: job.Generation(), Review: job.Review()}, nil)
			if failure, failed := result.Failure(); !failed || failure != FailureAPIKeyRejected {
				t.Errorf("failure = %v, %v; want api_key_rejected", failure, failed)
			}
			if result.Steps() != tc.want {
				t.Errorf("steps = %+v, want %+v", result.Steps(), tc.want)
			}
		})
	}
}

// TestRunWithFakeClientsIsDone checks Run builds the writer with the shorten
// and the review steps on: the standard fake article ends done with the
// shorten step skipped, and the long one with it done.
func TestRunWithFakeClientsIsDone(t *testing.T) {
	for _, tc := range []struct {
		article devfake.Article
		shorten StepState
	}{{devfake.ArticleStandard, StepSkipped}, {devfake.ArticleLong, StepDone}} {
		t.Run(tc.article.String(), func(t *testing.T) {
			scenario, err := devfake.NewScenario(tc.article, devfake.FailureNone, devfake.FailAtNone, 0)
			if err != nil {
				t.Fatalf("NewScenario: %v", err)
			}
			job := scenario.NewJob()
			result := Run(context.Background(), mustPrepare(t, StartInput{}), Deps{Generation: job.Generation(), Review: job.Review()}, nil)
			if _, ok := result.Article(); !ok {
				t.Fatalf("not done: %+v", result)
			}
			if want := (Steps{Draft: StepDone, Shorten: tc.shorten, Review: StepDone}); result.Steps() != want {
				t.Errorf("steps = %+v, want %+v", result.Steps(), want)
			}
		})
	}
}

// TestRunWriterOptions checks that a run builds the writer with
// writertestutil.DefaultOptions, the value the CLI's writer with no step flags
// is also checked against, so the two entry points cannot drift apart.
func TestRunWriterOptions(t *testing.T) {
	// A pointer client, so reflect.DeepEqual can match it (a func-typed client
	// never compares equal).
	review := &llmtestutil.FakeLLMClient{}
	if got, want := writerOptions(review), writertestutil.DefaultOptions(review); !reflect.DeepEqual(got, want) {
		t.Errorf("writerOptions() = %+v, want %+v", got, want)
	}
}
