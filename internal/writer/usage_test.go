//go:build test

package writer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
)

// Distinct token usages for the three stages, so a test can tell which stage
// produced a value.
var (
	genUsage    = llm.KnownUsage(llm.TokenCounts{Input: 11, CacheRead: 12, CacheWrite: 13, Output: 14})
	shortUsage  = llm.KnownUsage(llm.TokenCounts{Input: 21, CacheRead: 22, CacheWrite: 23, Output: 24})
	reviewUsage = llm.KnownUsage(llm.TokenCounts{Input: 31, CacheRead: 32, CacheWrite: 33, Output: 34})
)

// generationResponse is a valid generation response carrying body and usage,
// with the generation model.
func generationResponse(body string, usage llm.Usage) llm.GenerateResponse {
	return llm.GenerateResponse{Text: stepResponseText(markGenTitle, body), Model: markOutModel, Usage: usage}
}

// allStagesClient runs a generation (over the limit), a shorten, and a review,
// each with its own usage.
func allStagesClient(t *testing.T) (llm.LLMClient, *llmtestutil.FakeLLMClient) {
	t.Helper()
	gen := newStepClient(t,
		stepResult(generationResponse(overLimitBody(), genUsage)),
		stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel, Usage: shortUsage}),
	)
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel, Usage: reviewUsage}}
	return gen, review
}

// requireRunUsageInvariants checks the successful-run invariants of an Article:
// generation ran, the nil step results agree with the origins, and every stage
// model matches the article's model.
func requireRunUsageInvariants(t *testing.T, a Article) {
	t.Helper()
	if a.RunUsage.Generation.Origin == OriginNone {
		t.Error("generation origin is OriginNone on a successful run")
	}
	if (a.Shorten != nil) != (a.RunUsage.Shorten.Origin != OriginNone) {
		t.Errorf("Shorten %v disagrees with RunUsage.Shorten.Origin %v", a.Shorten, a.RunUsage.Shorten.Origin)
	}
	if (a.Review != nil) != (a.RunUsage.Review.Origin != OriginNone) {
		t.Errorf("Review %v disagrees with RunUsage.Review.Origin %v", a.Review, a.RunUsage.Review.Origin)
	}
	if a.RunUsage.Generation.Model != a.Model {
		t.Errorf("generation model = %q, want the article model %q", a.RunUsage.Generation.Model, a.Model)
	}
	if a.Shorten != nil && a.RunUsage.Shorten.Model != a.Shorten.Model {
		t.Errorf("shorten model = %q, want %q", a.RunUsage.Shorten.Model, a.Shorten.Model)
	}
	if a.Review != nil && a.RunUsage.Review.Model != a.Review.Model.Model {
		t.Errorf("review model = %q, want %q", a.RunUsage.Review.Model, a.Review.Model.Model)
	}
	requireCallTime(t, "generation", a.RunUsage.Generation)
	requireCallTime(t, "shorten", a.RunUsage.Shorten)
	requireCallTime(t, "review", a.RunUsage.Review)
}

// requireFailureUsage checks that Write failed with a zero Article and a
// *TokenUsageError whose RunUsage is want.
func requireFailureUsage(t *testing.T, a Article, err error, want RunUsage) *TokenUsageError {
	t.Helper()
	if a != (Article{}) {
		t.Errorf("Write article = %+v, want the zero value", a)
	}
	tue, ok := errors.AsType[*TokenUsageError](err)
	if !ok {
		t.Fatalf("Write error = %v, want a *TokenUsageError", err)
	}
	if tue.RunUsage != want {
		t.Errorf("RunUsage = %+v, want %+v", tue.RunUsage, want)
	}
	return tue
}

func TestWriteRunUsagePerStage(t *testing.T) {
	gen, review := allStagesClient(t)
	w := newFixedWriter(t, gen, withStepsEnabled(review))

	a, err := w.Write(context.Background(), validTranscript())
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	want := RunUsage{
		Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
		Shorten:    StageUsage{Origin: OriginRun, Model: markShortenModel, Usage: shortUsage, CallTime: callTime2},
		Review:     StageUsage{Origin: OriginRun, Model: markReviewModel, Usage: reviewUsage, CallTime: callTime3},
	}
	if a.RunUsage != want {
		t.Errorf("RunUsage = %+v, want %+v", a.RunUsage, want)
	}
	requireRunUsageInvariants(t, a)

	t.Run("an all-zero known usage is known", func(t *testing.T) {
		zero := llm.KnownUsage(llm.TokenCounts{})
		w := newFixedWriter(t, &llmtestutil.FakeLLMClient{Result: generationResponse(markGenBody, zero)}, Options{})
		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		counts, known := a.RunUsage.Generation.Usage.Counts()
		if !known {
			t.Fatal("usage = unknown, want the known zero usage")
		}
		if counts != (llm.TokenCounts{}) {
			t.Errorf("counts = %+v, want all zero", counts)
		}
	})
}

func TestWriteRunUsageUnknownStage(t *testing.T) {
	unknown := llm.UnknownUsage(llm.RuleNotNonNegativeInteger, "completion_tokens")
	w := newFixedWriter(t, &llmtestutil.FakeLLMClient{Result: generationResponse(markGenBody, unknown)}, Options{})

	a, err := w.Write(context.Background(), validTranscript())
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if a.RunUsage.Generation.Usage != unknown {
		t.Errorf("generation usage = %+v, want %+v", a.RunUsage.Generation.Usage, unknown)
	}
	requireRunUsageInvariants(t, a)
}

func TestWriteRunUsageInvariants(t *testing.T) {
	reviewResp := llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel, Usage: reviewUsage}
	shortResp := llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel, Usage: shortUsage}

	cases := []struct {
		name        string
		shorten     bool
		review      bool
		body        string
		wantShorten UsageOrigin
		wantReview  UsageOrigin
	}{
		{"generation only", false, false, markGenBody, OriginNone, OriginNone},
		{"shorten not called", true, false, markGenBody, OriginNone, OriginNone},
		{"shorten called", true, false, overLimitBody(), OriginRun, OriginNone},
		{"review run", false, true, markGenBody, OriginNone, OriginRun},
		{"both run", true, true, overLimitBody(), OriginRun, OriginRun},
		{"shorten not called, review run", true, true, markGenBody, OriginNone, OriginRun},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps := []llmtestutil.FakeLLMSequenceStep{stepResult(generationResponse(tc.body, genUsage))}
			if tc.wantShorten == OriginRun {
				steps = append(steps, stepResult(shortResp))
			}
			gen := newStepClient(t, steps...)
			review := &llmtestutil.FakeLLMClient{Result: reviewResp}
			var opts Options
			if tc.shorten {
				opts.Shorten = ShortenOptions{Enabled: true}
			}
			if tc.review {
				opts.Review = ReviewOptions{Enabled: true, Client: review}
			}
			w := newFixedWriter(t, gen, opts)

			a, err := w.Write(context.Background(), validTranscript())
			if err != nil {
				t.Fatalf("Write error = %v, want nil", err)
			}
			requireRunUsageInvariants(t, a)
			if a.RunUsage.Generation.Origin != OriginRun {
				t.Errorf("generation origin = %v, want OriginRun", a.RunUsage.Generation.Origin)
			}
			if a.RunUsage.Shorten.Origin != tc.wantShorten {
				t.Errorf("shorten origin = %v, want %v", a.RunUsage.Shorten.Origin, tc.wantShorten)
			}
			if a.RunUsage.Review.Origin != tc.wantReview {
				t.Errorf("review origin = %v, want %v", a.RunUsage.Review.Origin, tc.wantReview)
			}
		})
	}
}

func TestWriteRunUsageResume(t *testing.T) {
	// A chain whose records carry usages and models distinct from a run's, so a
	// reused value is distinguishable from a fresh call.
	chain := func() ArtifactChain {
		return ArtifactChain{
			Generation: StageRecord{
				Stage: StageGeneration, Title: markGenTitle, Body: overLimitBody(),
				Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime, Input: "material", ID: "gen-id",
			},
			Shorten: &StageRecord{
				Stage: StageShorten, Title: markShortTitle, Body: markShortBody,
				Model: "reused-short-model", Usage: llm.UnknownUsage(llm.RuleRecordedUnknown, ""), CallTime: reusedCallTime, Input: "gen-id", ID: "short-id",
			},
			Review: &StageRecord{
				Stage: StageReview, Title: markShortBody,
				Model: "reused-review-model", Usage: reviewUsage, CallTime: reusedCallTime, Input: "short-id", ID: "review-id",
			},
		}
	}

	t.Run("after review reuses every stage", func(t *testing.T) {
		store := &fakeArtifactStore{loadResult: chain()}
		w := newFixedWriter(t, nil, Options{Artifacts: ArtifactOptions{Store: store, Resume: StageReview}})
		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		want := RunUsage{
			Generation: StageUsage{Origin: OriginReused, Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime},
			Shorten:    StageUsage{Origin: OriginReused, Model: "reused-short-model", Usage: llm.UnknownUsage(llm.RuleRecordedUnknown, ""), CallTime: reusedCallTime},
			Review:     StageUsage{Origin: OriginReused, Model: "reused-review-model", Usage: reviewUsage, CallTime: reusedCallTime},
		}
		if a.RunUsage != want {
			t.Errorf("RunUsage = %+v, want %+v", a.RunUsage, want)
		}
		requireRunUsageInvariants(t, a)
		if got := a.RunUsage.Shorten.Usage.UnknownReason(); got != "recorded-unknown" {
			t.Errorf("reused shorten reason = %q, want %q", got, "recorded-unknown")
		}
	})

	t.Run("after shorten reuses generation and shorten, runs review", func(t *testing.T) {
		store := &fakeArtifactStore{loadResult: chain()}
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel, Usage: reviewUsage}}
		w := newFixedWriter(t, nil, Options{
			Artifacts: ArtifactOptions{Store: store, Resume: StageShorten},
			Review:    ReviewOptions{Enabled: true, Client: review},
		})
		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		want := RunUsage{
			Generation: StageUsage{Origin: OriginReused, Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime},
			Shorten:    StageUsage{Origin: OriginReused, Model: "reused-short-model", Usage: llm.UnknownUsage(llm.RuleRecordedUnknown, ""), CallTime: reusedCallTime},
			Review:     StageUsage{Origin: OriginRun, Model: markReviewModel, Usage: reviewUsage, CallTime: callTime1},
		}
		if a.RunUsage != want {
			t.Errorf("RunUsage = %+v, want %+v", a.RunUsage, want)
		}
		if len(review.Calls) != 1 {
			t.Errorf("review client called %d times, want 1", len(review.Calls))
		}
	})

	t.Run("after generation reuses generation, runs shorten and review", func(t *testing.T) {
		store := &fakeArtifactStore{loadResult: ArtifactChain{Generation: StageRecord{
			Stage: StageGeneration, Title: markGenTitle, Body: overLimitBody(),
			Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime, Input: "material", ID: "gen-id",
		}}}
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel, Usage: shortUsage}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel, Usage: reviewUsage}}
		w := newFixedWriter(t, gen, Options{
			Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration},
			Shorten:   ShortenOptions{Enabled: true},
			Review:    ReviewOptions{Enabled: true, Client: review},
		})
		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		want := RunUsage{
			Generation: StageUsage{Origin: OriginReused, Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime},
			Shorten:    StageUsage{Origin: OriginRun, Model: markShortenModel, Usage: shortUsage, CallTime: callTime1},
			Review:     StageUsage{Origin: OriginRun, Model: markReviewModel, Usage: reviewUsage, CallTime: callTime2},
		}
		if a.RunUsage != want {
			t.Errorf("RunUsage = %+v, want %+v", a.RunUsage, want)
		}
	})

	t.Run("after shorten with review disabled reuses generation and shorten", func(t *testing.T) {
		store := &fakeArtifactStore{loadResult: chain()}
		w := newFixedWriter(t, nil, Options{Artifacts: ArtifactOptions{Store: store, Resume: StageShorten}})
		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		want := RunUsage{
			Generation: StageUsage{Origin: OriginReused, Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime},
			Shorten:    StageUsage{Origin: OriginReused, Model: "reused-short-model", Usage: llm.UnknownUsage(llm.RuleRecordedUnknown, ""), CallTime: reusedCallTime},
		}
		if a.RunUsage != want {
			t.Errorf("RunUsage = %+v, want %+v", a.RunUsage, want)
		}
		requireRunUsageInvariants(t, a)
	})

	t.Run("after generation with a short body leaves shorten unrun", func(t *testing.T) {
		store := &fakeArtifactStore{loadResult: ArtifactChain{Generation: StageRecord{
			Stage: StageGeneration, Title: markGenTitle, Body: markGenBody,
			Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime, Input: "material", ID: "gen-id",
		}}}
		gen := newStepClient(t)
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel, Usage: reviewUsage}}
		w := newFixedWriter(t, gen, Options{
			Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration},
			Shorten:   ShortenOptions{Enabled: true},
			Review:    ReviewOptions{Enabled: true, Client: review},
		})
		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		want := RunUsage{
			Generation: StageUsage{Origin: OriginReused, Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime},
			Review:     StageUsage{Origin: OriginRun, Model: markReviewModel, Usage: reviewUsage, CallTime: callTime1},
		}
		if a.RunUsage != want {
			t.Errorf("RunUsage = %+v, want %+v", a.RunUsage, want)
		}
		requireRunUsageInvariants(t, a)
	})
}

func TestWriteFailureRunUsage(t *testing.T) {
	stepFailure := fmt.Errorf("fake: %w", llm.ErrTruncated)

	t.Run("invalid transcript", func(t *testing.T) {
		w := newFixedWriter(t, newFakeClient(), Options{})
		a, err := w.Write(context.Background(), transcript.Transcript{})
		requireFailureUsage(t, a, err, RunUsage{})
	})

	t.Run("canceled before the run starts", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		w := newFixedWriter(t, newFakeClient(), Options{})
		a, err := w.Write(ctx, validTranscript())
		requireFailureUsage(t, a, err, RunUsage{})
	})

	t.Run("generation truncated with usage", func(t *testing.T) {
		gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Result: llm.GenerateResponse{Model: markOutModel, Usage: genUsage}, Err: stepFailure})
		w := newFixedWriter(t, gen, Options{})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
		})
	})

	t.Run("generation malformed output", func(t *testing.T) {
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: "no heading", Model: markOutModel, Usage: genUsage}))
		w := newFixedWriter(t, gen, Options{})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
		})
	})

	t.Run("generation with no usage keeps the stage run", func(t *testing.T) {
		gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Err: context.DeadlineExceeded})
		w := newFixedWriter(t, gen, Options{})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, CallTime: callTime1},
		})
	})

	t.Run("a failed call with an invalid model reports no model", func(t *testing.T) {
		gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{
			Result: llm.GenerateResponse{Model: "bad\x1bmodel", Usage: genUsage},
			Err:    stepFailure,
		})
		w := newFixedWriter(t, gen, Options{})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Usage: genUsage, CallTime: callTime1},
		})
	})

	t.Run("generation expansion failure leaves the stage unrun", func(t *testing.T) {
		gen := newStepClient(t)
		w := newFixedWriter(t, gen, Options{UserTemplatePath: writeOverrideFile(t, "{{index .Title 100}}")})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{})
	})

	t.Run("review response shape invalid keeps the stage run", func(t *testing.T) {
		gen := newStepClient(t, stepResult(generationResponse(markGenBody, genUsage)))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: "not json", Model: markReviewModel, Usage: reviewUsage}}
		w := newFixedWriter(t, gen, Options{Review: ReviewOptions{Enabled: true, Client: review}})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
			Review:     StageUsage{Origin: OriginRun, Model: markReviewModel, Usage: reviewUsage, CallTime: callTime2},
		})
	})

	t.Run("shorten expansion failure leaves the stage unrun", func(t *testing.T) {
		gen := newStepClient(t, stepResult(generationResponse(overLimitBody(), genUsage)))
		w := newFixedWriter(t, gen, Options{Shorten: ShortenOptions{
			Enabled: true, UserTemplatePath: writeOverrideFile(t, "{{index .ArticleTitle 100}}"),
		}})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
		})
	})

	t.Run("canceled before the shorten call leaves the stage unrun", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Respond: func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			cancel()
			return generationResponse(overLimitBody(), genUsage), nil
		}})
		w := newFixedWriter(t, gen, Options{Shorten: ShortenOptions{Enabled: true}})
		a, err := w.Write(ctx, validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
		})
	})

	t.Run("canceled before the review call leaves the stage unrun", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Respond: func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			cancel()
			return generationResponse(markGenBody, genUsage), nil
		}})
		review := &llmtestutil.FakeLLMClient{}
		w := newFixedWriter(t, gen, Options{Review: ReviewOptions{Enabled: true, Client: review}})
		a, err := w.Write(ctx, validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
		})
	})

	t.Run("shorten truncated with usage", func(t *testing.T) {
		gen := newStepClient(t,
			stepResult(generationResponse(overLimitBody(), genUsage)),
			llmtestutil.FakeLLMSequenceStep{Result: llm.GenerateResponse{Model: markShortenModel, Usage: shortUsage}, Err: stepFailure},
		)
		w := newFixedWriter(t, gen, Options{Shorten: ShortenOptions{Enabled: true}})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
			Shorten:    StageUsage{Origin: OriginRun, Model: markShortenModel, Usage: shortUsage, CallTime: callTime2},
		})
	})

	t.Run("review truncated with usage", func(t *testing.T) {
		gen := newStepClient(t, stepResult(generationResponse(markGenBody, genUsage)))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Model: markReviewModel, Usage: reviewUsage}, Err: stepFailure}
		w := newFixedWriter(t, gen, Options{Review: ReviewOptions{Enabled: true, Client: review}})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
			Review:     StageUsage{Origin: OriginRun, Model: markReviewModel, Usage: reviewUsage, CallTime: callTime2},
		})
	})

	t.Run("save failure after generation", func(t *testing.T) {
		store := &fakeArtifactStore{saveErrs: map[Stage]error{StageGeneration: errors.New("boom")}}
		gen := newStepClient(t, stepResult(generationResponse(markGenBody, genUsage)))
		w := newFixedWriter(t, gen, Options{Artifacts: ArtifactOptions{Store: store}})
		a, err := w.Write(context.Background(), validTranscript())
		tue := requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
		})
		if _, ok := errors.AsType[*ArtifactError](tue.Err); !ok {
			t.Errorf("inner error = %v, want a *ArtifactError", tue.Err)
		}
	})

	t.Run("final length check after review", func(t *testing.T) {
		longer := markShortBody + strings.Repeat("a", maxBodyChars)
		gen := newStepClient(t, stepResult(generationResponse(markGenBody, genUsage)))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
			Text:  reviewResponse(replaceItem(markGenBody, longer, "meaning_change", markRevisionEvidence)),
			Model: markReviewModel, Usage: reviewUsage,
		}}
		w := newFixedWriter(t, gen, withStepsEnabled(review))
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginRun, Model: markOutModel, Usage: genUsage, CallTime: callTime1},
			Review:     StageUsage{Origin: OriginRun, Model: markReviewModel, Usage: reviewUsage, CallTime: callTime2},
		})
	})

	t.Run("resume load failure has no usage", func(t *testing.T) {
		store := &fakeArtifactStore{loadErr: errors.New("boom")}
		w := newFixedWriter(t, nil, Options{Artifacts: ArtifactOptions{Store: store, Resume: StageReview}})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{})
	})

	t.Run("failure after a successful resume keeps the reused stage", func(t *testing.T) {
		store := &fakeArtifactStore{loadResult: ArtifactChain{
			Generation: StageRecord{Stage: StageGeneration, Title: markGenTitle, Body: markGenBody, Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime, Input: "material", ID: "gen-id"},
		}}
		review := &llmtestutil.FakeLLMClient{Err: stepFailure}
		w := newFixedWriter(t, nil, Options{
			Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration},
			Review:    ReviewOptions{Enabled: true, Client: review},
		})
		a, err := w.Write(context.Background(), validTranscript())
		requireFailureUsage(t, a, err, RunUsage{
			Generation: StageUsage{Origin: OriginReused, Model: "reused-gen-model", Usage: genUsage, CallTime: reusedCallTime},
			Review:     StageUsage{Origin: OriginRun, CallTime: callTime1},
		})
	})
}

func TestWriteSavesUsage(t *testing.T) {
	store := &fakeArtifactStore{}
	gen, review := allStagesClient(t)
	w := newFixedWriter(t, gen, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store},
	})

	if _, err := w.Write(context.Background(), validTranscript()); err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	for _, tc := range []struct {
		stage Stage
		want  llm.Usage
	}{
		{StageGeneration, genUsage},
		{StageShorten, shortUsage},
		{StageReview, reviewUsage},
	} {
		call := store.saveFor(tc.stage)
		if call.record.Usage != tc.want {
			t.Errorf("%s saved usage = %+v, want %+v", tc.stage, call.record.Usage, tc.want)
		}
	}
}

func TestTokenUsageErrorTransparent(t *testing.T) {
	stepFailure := stepError(StepReview, ErrTooLong)
	wrapped := tokenUsageError(RunUsage{}, stepFailure)
	if wrapped.Error() != stepFailure.Error() {
		t.Errorf("Error() = %q, want the inner message %q", wrapped.Error(), stepFailure.Error())
	}
	if !errors.Is(wrapped, ErrTooLong) {
		t.Error("TokenUsageError does not unwrap to the inner sentinel")
	}
	if _, ok := errors.AsType[*StepError](wrapped); !ok {
		t.Error("TokenUsageError does not unwrap to the inner *StepError")
	}

	artifactFailure := artifactError(StageShorten, ErrMalformedOutput)
	artifactWrapped := tokenUsageError(RunUsage{}, artifactFailure)
	if !errors.Is(artifactWrapped, ErrMalformedOutput) {
		t.Error("TokenUsageError does not unwrap to the artifact sentinel")
	}
	if _, ok := errors.AsType[*ArtifactError](artifactWrapped); !ok {
		t.Error("TokenUsageError does not unwrap to the inner *ArtifactError")
	}

	t.Run("a nil inner error does not panic", func(t *testing.T) {
		e := &TokenUsageError{}
		if got := e.Error(); got != "token usage error" {
			t.Errorf("Error() = %q, want the fixed fallback", got)
		}
	})
}
