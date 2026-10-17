//go:build test

package writer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
)

// The distinct times the fixed clock hands out, one per Generate call, so a
// test can tell which stage's CallTime it reads. They are in UTC.
var (
	callTime1 = time.Date(2026, time.October, 16, 1, 0, 0, 0, time.UTC)
	callTime2 = time.Date(2026, time.October, 16, 4, 2, 41, 0, time.UTC)
	callTime3 = time.Date(2026, time.October, 16, 4, 3, 15, 0, time.UTC)
)

// reusedCallTime is the call time on the records of a loaded chain. It differs
// from every fixed-clock time, so a reused stage's CallTime is distinguishable
// from a run stage's.
var reusedCallTime = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

// sliceClock returns a clock that hands out times in order. It fails the test
// if it is read past the end, so an extra stage does not go unnoticed.
func sliceClock(t *testing.T, times []time.Time) func() time.Time {
	t.Helper()
	index := 0
	return func() time.Time {
		if index >= len(times) {
			t.Fatalf("clock read %d times, want at most %d", index+1, len(times))
		}
		now := times[index]
		index++
		return now
	}
}

// fixedClock returns the clock the writer tests inject: one distinct time per
// Generate call.
func fixedClock(t *testing.T) func() time.Time {
	t.Helper()
	return sliceClock(t, []time.Time{callTime1, callTime2, callTime3})
}

// newFixedWriter builds an ArticleWriter whose stages read the fixed clock, so
// every CallTime it records is one of callTime1..3. A test that compares a
// RunUsage uses it; a test that needs the default clock builds the writer with
// New directly.
func newFixedWriter(t *testing.T, client llm.LLMClient, opts Options) ArticleWriter {
	t.Helper()
	opts.Clock = fixedClock(t)
	return mustNew(t, client, opts)
}

// requireCallTime checks the call-time invariant of one stage: a stage that ran
// or reused an artifact has a non-zero call time, and an OriginNone stage has a
// zero one.
func requireCallTime(t *testing.T, name string, usage StageUsage) {
	t.Helper()
	if usage.Origin == OriginNone {
		if !usage.CallTime.IsZero() {
			t.Errorf("%s is OriginNone but has call time %v", name, usage.CallTime)
		}
		return
	}
	if usage.CallTime.IsZero() {
		t.Errorf("%s has origin %v but a zero call time", name, usage.Origin)
	}
}

func TestWriteCallTimePerStage(t *testing.T) {
	// The times are in a non-UTC zone and carry nanoseconds, so the test proves
	// the writer normalizes to UTC and whole seconds.
	zone := time.FixedZone("JST", 9*60*60)
	calls := []time.Time{
		time.Date(2026, time.October, 16, 1, 0, 0, 123456789, time.UTC).In(zone),
		time.Date(2026, time.October, 16, 4, 2, 41, 500000000, time.UTC).In(zone),
		time.Date(2026, time.October, 16, 4, 3, 15, 1, time.UTC).In(zone),
	}
	want := []time.Time{
		calls[0].UTC().Truncate(time.Second),
		calls[1].UTC().Truncate(time.Second),
		calls[2].UTC().Truncate(time.Second),
	}

	gen, review := allStagesClient(t)
	store := &fakeArtifactStore{}
	w := mustNew(t, gen, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store},
		Clock:     sliceClock(t, calls),
	})

	a, err := w.Write(context.Background(), validTranscript())
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	gotUsage := []StageUsage{a.RunUsage.Generation, a.RunUsage.Shorten, a.RunUsage.Review}
	stages := []Stage{StageGeneration, StageShorten, StageReview}
	for i, stage := range stages {
		if !gotUsage[i].CallTime.Equal(want[i]) {
			t.Errorf("%s StageUsage.CallTime = %v, want %v", stage, gotUsage[i].CallTime, want[i])
		}
		saved := store.saveFor(stage).record.CallTime
		if !saved.Equal(want[i]) {
			t.Errorf("%s StageRecord.CallTime = %v, want %v", stage, saved, want[i])
		}
		if saved.Location() != time.UTC {
			t.Errorf("%s StageRecord.CallTime location = %v, want UTC", stage, saved.Location())
		}
	}
}

func TestWriteReadsClockAfterGenerate(t *testing.T) {
	// The fake LLM sets the shared clock's value when it answers, so the
	// recorded call time is non-zero only if the writer reads the clock after
	// Generate returns. Reading it before the call (or not at all) would leave
	// the value at its zero initial state.
	var now time.Time
	gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Respond: func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		now = callTime1
		return generationResponse(markGenBody, genUsage), nil
	}})
	w := mustNew(t, gen, Options{Clock: func() time.Time { return now }})

	a, err := w.Write(context.Background(), validTranscript())
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if got := a.RunUsage.Generation.CallTime; !got.Equal(callTime1) {
		t.Errorf("generation CallTime = %v, want %v (read after Generate returned)", got, callTime1)
	}
}

func TestWriteRejectsMissingCallTime(t *testing.T) {
	cases := []struct {
		name    string
		stage   Stage
		zeroOut func(*ArtifactChain)
	}{
		{"generation", StageGeneration, func(c *ArtifactChain) { c.Generation.CallTime = time.Time{} }},
		{"shorten", StageShorten, func(c *ArtifactChain) { c.Shorten.CallTime = time.Time{} }},
		{"review", StageReview, func(c *ArtifactChain) { c.Review.CallTime = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chain := resumeChainReview(nil)
			tc.zeroOut(&chain)
			store := &fakeArtifactStore{loadResult: chain}
			// The generation client is required because the shorten step is
			// enabled; it fails the test if it is ever called.
			review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
			w := mustNew(t, newStepClient(t), Options{
				Shorten:   ShortenOptions{Enabled: true},
				Review:    ReviewOptions{Enabled: true, Client: review},
				Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration},
			})

			a, err := w.Write(context.Background(), validTranscript())
			if a != (Article{}) {
				t.Errorf("Write article = %+v, want the zero value", a)
			}
			if !errors.Is(err, ErrMissingCallTime) {
				t.Fatalf("Write error = %v, want ErrMissingCallTime", err)
			}
			tue, ok := errors.AsType[*TokenUsageError](err)
			if !ok {
				t.Fatalf("Write error = %v, want a *TokenUsageError", err)
			}
			if tue.RunUsage != (RunUsage{}) {
				t.Errorf("RunUsage = %+v, want the zero value", tue.RunUsage)
			}
			artifactErr, ok := errors.AsType[*ArtifactError](tue.Err)
			if !ok {
				t.Fatalf("inner error = %v, want a *ArtifactError", tue.Err)
			}
			if artifactErr.Stage != tc.stage {
				t.Errorf("ArtifactError.Stage = %v, want %v (the record missing its call time)", artifactErr.Stage, tc.stage)
			}
			if len(review.Calls) != 0 {
				t.Errorf("the review client was called %d times, want 0", len(review.Calls))
			}
		})
	}
}

func TestWriteDefaultClock(t *testing.T) {
	// The writer is built with no Clock, so New selects time.Now: the recorded
	// call time falls between the times read just before and just after the
	// run, truncated to the second.
	before := time.Now().UTC().Truncate(time.Second)
	w, err := New(newFakeClient(), Options{})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	a, err := w.Write(context.Background(), validTranscript())
	after := time.Now().UTC().Truncate(time.Second)
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	got := a.RunUsage.Generation.CallTime
	if got.Before(before) || got.After(after) {
		t.Errorf("CallTime = %v, want it in [%v, %v]", got, before, after)
	}
	if got.Location() != time.UTC {
		t.Errorf("CallTime location = %v, want UTC", got.Location())
	}
}
