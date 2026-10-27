//go:build test

package writer

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
)

// progressCase builds a fresh writer for one run, so a test can run the same
// configuration twice (Write and WriteWithProgress) on unused fakes.
type progressCase struct {
	name  string
	build func(t *testing.T) ProgressWriter
	want  []progressEvent
}

// stepsWriter builds a writer with both steps enabled and the fixed clock,
// whose generation answers with body and whose shorten step (when reached)
// answers with a short body.
func stepsWriter(t *testing.T, body string, shortenRuns bool) ProgressWriter {
	t.Helper()
	steps := []llmtestutil.FakeLLMSequenceStep{
		stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, body), Model: markOutModel}),
	}
	if shortenRuns {
		steps = append(steps, stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel}))
	}
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
	opts := withStepsEnabled(review)
	opts.Clock = fixedClock(t)
	return mustNewProgress(t, newStepClient(t, steps...), opts)
}

// requireProgress runs each case through WriteWithProgress and checks the
// events, then runs a second writer of the same case through Write and checks
// that both return the same article.
func requireProgress(t *testing.T, cases []progressCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observer := &recordingObserver{}
			got, err := tc.build(t).WriteWithProgress(context.Background(), validTranscript(), observer)
			if err != nil {
				t.Fatalf("WriteWithProgress error = %v, want nil", err)
			}
			if !slices.Equal(observer.events, tc.want) {
				t.Errorf("events = %v, want %v", observer.events, tc.want)
			}
			want, err := tc.build(t).Write(context.Background(), validTranscript())
			if err != nil {
				t.Fatalf("Write error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("WriteWithProgress article = %+v, want the Write article %+v", got, want)
			}
		})
	}
}

func TestWriteWithProgressOrder(t *testing.T) {
	requireProgress(t, []progressCase{
		{
			name: "only review enabled",
			build: func(t *testing.T) ProgressWriter {
				review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
				return mustNewProgress(t, newFakeClient(), Options{Review: ReviewOptions{Enabled: true, Client: review}, Clock: fixedClock(t)})
			},
			want: []progressEvent{
				started(StageGeneration), finished(StageGeneration),
				skipped(StageShorten),
				started(StageReview), finished(StageReview),
			},
		},
		{
			name: "both steps disabled",
			build: func(t *testing.T) ProgressWriter {
				return mustNewProgress(t, newFakeClient(), Options{Clock: fixedClock(t)})
			},
			want: []progressEvent{
				started(StageGeneration), finished(StageGeneration),
				skipped(StageShorten), skipped(StageReview),
			},
		},
		{
			name: "resume after shorten",
			build: func(t *testing.T) ProgressWriter {
				review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
				return mustNewProgress(t, nil, Options{
					Shorten:   ShortenOptions{Enabled: true},
					Review:    ReviewOptions{Enabled: true, Client: review},
					Artifacts: ArtifactOptions{Store: &fakeArtifactStore{loadResult: resumeChainShorten()}, Resume: StageShorten},
					Clock:     fixedClock(t),
				})
			},
			want: []progressEvent{
				finished(StageGeneration), finished(StageShorten),
				started(StageReview), finished(StageReview),
			},
		},
		{
			name: "resume after review",
			build: func(t *testing.T) ProgressWriter {
				return mustNewProgress(t, nil, Options{
					Shorten:   ShortenOptions{Enabled: true},
					Review:    ReviewOptions{Enabled: true},
					Artifacts: ArtifactOptions{Store: &fakeArtifactStore{loadResult: resumeChainReview(nil)}, Resume: StageReview},
				})
			},
			want: []progressEvent{
				finished(StageGeneration), finished(StageShorten), finished(StageReview),
			},
		},
	})
}

func TestWriteWithProgressShortenBoundary(t *testing.T) {
	// The body part carries a line feed, which bodyChars does not count, so
	// only the counted length decides the branch, as in TestWriteBodyLength.
	atLimit := "x\n" + strings.Repeat("a", maxBodyChars-1)
	overLimit := "x\n" + strings.Repeat("a", maxBodyChars)
	requireProgress(t, []progressCase{
		{
			name:  "at the limit",
			build: func(t *testing.T) ProgressWriter { return stepsWriter(t, atLimit, false) },
			want: []progressEvent{
				started(StageGeneration), finished(StageGeneration),
				skipped(StageShorten),
				started(StageReview), finished(StageReview),
			},
		},
		{
			name:  "one over the limit",
			build: func(t *testing.T) ProgressWriter { return stepsWriter(t, overLimit, true) },
			want: []progressEvent{
				started(StageGeneration), finished(StageGeneration),
				started(StageShorten), finished(StageShorten),
				started(StageReview), finished(StageReview),
			},
		},
	})
}

func TestWriteWithProgressFailures(t *testing.T) {
	failure := errors.New("fake step failure")
	reviewOK := func() *llmtestutil.FakeLLMClient {
		return &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
	}
	cases := []struct {
		name       string
		gen        func(t *testing.T) llm.LLMClient
		review     *llmtestutil.FakeLLMClient
		ctx        func() context.Context
		transcript func() transcript.Transcript
		artifacts  ArtifactOptions
		// wantErr tells this row's failure from the other rows', so a row
		// whose fixture drifted to another failure does not pass.
		wantErr func(err error) bool
		want    []progressEvent
	}{
		{
			name:    "context done before generation",
			wantErr: func(err error) bool { return errors.Is(err, context.Canceled) },
			gen:     func(t *testing.T) llm.LLMClient { return newStepClient(t) },
			review:  reviewOK(),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			want: nil,
		},
		{
			name:    "invalid transcript",
			wantErr: func(err error) bool { return errors.Is(err, ErrInvalidTranscript) },
			gen:     func(t *testing.T) llm.LLMClient { return newStepClient(t) },
			review:  reviewOK(),
			transcript: func() transcript.Transcript {
				tr := validTranscript()
				tr.Segments = nil
				return tr
			},
			want: nil,
		},
		{
			name:      "resume load",
			wantErr:   isArtifactError(StageGeneration),
			gen:       func(t *testing.T) llm.LLMClient { return newStepClient(t) },
			review:    reviewOK(),
			artifacts: ArtifactOptions{Store: &fakeArtifactStore{loadErr: failure}, Resume: StageGeneration},
			want:      nil,
		},
		{
			name:    "generation artifact save",
			wantErr: isArtifactError(StageGeneration),
			gen: func(t *testing.T) llm.LLMClient {
				return newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
			},
			review:    reviewOK(),
			artifacts: ArtifactOptions{Store: &fakeArtifactStore{saveErrs: map[Stage]error{StageGeneration: failure}}},
			want:      []progressEvent{started(StageGeneration)},
		},
		{
			name:    "generation",
			wantErr: func(err error) bool { return errors.Is(err, failure) && !isStepError(err) },
			gen: func(t *testing.T) llm.LLMClient {
				return newStepClient(t, llmtestutil.FakeLLMSequenceStep{Err: failure})
			},
			review: reviewOK(),
			want:   []progressEvent{started(StageGeneration)},
		},
		{
			name:    "shorten",
			wantErr: func(err error) bool { return errors.Is(err, failure) && isStepErrorOf(err, StepShorten) },
			gen: func(t *testing.T) llm.LLMClient {
				return newStepClient(t,
					stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
					llmtestutil.FakeLLMSequenceStep{Err: failure},
				)
			},
			review: reviewOK(),
			want: []progressEvent{
				started(StageGeneration), finished(StageGeneration),
				started(StageShorten),
			},
		},
		{
			name:    "review",
			wantErr: func(err error) bool { return errors.Is(err, failure) && isStepErrorOf(err, StepReview) },
			gen: func(t *testing.T) llm.LLMClient {
				return newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
			},
			review: &llmtestutil.FakeLLMClient{Err: failure},
			want: []progressEvent{
				started(StageGeneration), finished(StageGeneration),
				skipped(StageShorten),
				started(StageReview),
			},
		},
		{
			// The review passes its own checks but lengthens the body past
			// the limit, so the final length check fails it.
			name:    "final length check",
			wantErr: func(err error) bool { return errors.Is(err, ErrTooLong) && isStepErrorOf(err, StepReview) },
			gen: func(t *testing.T) llm.LLMClient {
				return newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
			},
			review: &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
				Text:  reviewResponse(replaceItem(markGenBody, markShortBody+strings.Repeat("a", maxBodyChars), "meaning_change", markRevisionEvidence)),
				Model: markReviewModel,
			}},
			want: []progressEvent{
				started(StageGeneration), finished(StageGeneration),
				skipped(StageShorten),
				started(StageReview),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx()
			}
			tr := validTranscript()
			if tc.transcript != nil {
				tr = tc.transcript()
			}
			opts := withStepsEnabled(tc.review)
			opts.Artifacts = tc.artifacts
			w := mustNewProgress(t, tc.gen(t), opts)
			observer := &recordingObserver{}

			a, err := w.WriteWithProgress(ctx, tr, observer)
			if !tc.wantErr(err) {
				t.Fatalf("WriteWithProgress error = %v, want this row's failure", err)
			}
			if a != (Article{}) {
				t.Errorf("WriteWithProgress article = %+v, want the zero value", a)
			}
			if !slices.Equal(observer.events, tc.want) {
				t.Errorf("events = %v, want %v", observer.events, tc.want)
			}
		})
	}
}

// isArtifactError returns a check that err is a *ArtifactError for stage.
func isArtifactError(stage Stage) func(error) bool {
	return func(err error) bool {
		artifactErr, ok := errors.AsType[*ArtifactError](err)
		return ok && artifactErr.Stage == stage
	}
}

// isStepError reports whether err is a *StepError.
func isStepError(err error) bool {
	_, ok := errors.AsType[*StepError](err)
	return ok
}

// isStepErrorOf reports whether err is a *StepError for step.
func isStepErrorOf(err error, step Step) bool {
	stepErr, ok := errors.AsType[*StepError](err)
	return ok && stepErr.Step == step
}

func TestWriteWithProgressNilObserver(t *testing.T) {
	gen := newStepClient(t) // fails the test if the generation client is called
	w := mustNewProgress(t, gen, Options{})
	var typedNil *recordingObserver
	for name, observer := range map[string]ProgressObserver{"nil": nil, "typed nil": typedNil} {
		t.Run(name, func(t *testing.T) {
			a, err := w.WriteWithProgress(context.Background(), validTranscript(), observer)
			if !errors.Is(err, errNilProgressObserver) {
				t.Errorf("WriteWithProgress error = %v, want errNilProgressObserver", err)
			}
			if a != (Article{}) {
				t.Errorf("WriteWithProgress article = %+v, want the zero value", a)
			}
		})
	}
}

func TestNewProgressWriterValidatesOptions(t *testing.T) {
	// One rejection is enough: New and NewProgressWriter share their
	// validation, which the New tests cover row by row.
	w, err := NewProgressWriter(nil, Options{})
	if !errors.Is(err, errNilLLMClient) {
		t.Errorf("NewProgressWriter error = %v, want errNilLLMClient", err)
	}
	if w != nil {
		t.Errorf("NewProgressWriter writer = %v, want nil", w)
	}
}
