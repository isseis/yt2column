//go:build test

package writer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/transcript"
)

// requireStepError checks a step failure: it matches target, it is a *StepError
// for step, and its message names the step. The sentinel is judged with
// errors.Is or errors.AsType, never by the message text; the message is also
// checked, as a separate assertion, because a caller reads it.
func requireStepError(t *testing.T, err, target error, step Step, a Article) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("Write error = %v, want it to match %v", err, target)
	}
	stepErr, ok := errors.AsType[*StepError](err)
	if !ok {
		t.Fatalf("Write error = %v, want a *StepError", err)
	}
	if stepErr.Step != step {
		t.Errorf("StepError.Step = %v, want %v", stepErr.Step, step)
	}
	if want := step.String() + " step"; !strings.Contains(err.Error(), want) {
		t.Errorf("Write error = %v, want it to contain %q", err, want)
	}
	if a != (Article{}) {
		t.Errorf("Write article = %+v, want the zero value", a)
	}
}

func TestWriteStepOrder(t *testing.T) {
	t.Run("within the limit", func(t *testing.T) {
		gen := newStepClient(t, stepResult(llm.GenerateResponse{
			Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel, ModelVersion: markOutModelVersion,
		}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		// The generation client is called once and the shorten step is not
		// reached, so the extra step would have failed the sequence fake.
		if len(gen.Calls) != 1 {
			t.Errorf("generation client called %d times, want 1", len(gen.Calls))
		}
		if len(review.Calls) != 1 {
			t.Errorf("review client called %d times, want 1", len(review.Calls))
		}
		if a.Title != markGenTitle {
			t.Errorf("Title = %q, want the generation title %q", a.Title, markGenTitle)
		}
		if !strings.HasPrefix(a.Body, markGenBody) {
			t.Errorf("Body = %q, want it to start with the generation body %q", a.Body, markGenBody)
		}
		if a.Shorten != nil {
			t.Errorf("Shorten = %+v, want nil (the shorten step called no LLM)", a.Shorten)
		}
		if a.Review == nil {
			t.Fatal("Review = nil, want the review step's result")
		}
		if a.Review.Model.Model != markReviewModel {
			t.Errorf("Review.Model.Model = %q, want %q", a.Review.Model.Model, markReviewModel)
		}
		if len(a.Review.Revisions) != 0 {
			t.Errorf("Review.Revisions = %+v, want empty", a.Review.Revisions)
		}
	})

	t.Run("over the limit", func(t *testing.T) {
		gen := newStepClient(t,
			stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
			stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel, ModelVersion: markStepModelVersion}),
		)
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		// The generation and the shorten step share the generation client, in
		// that order.
		if len(gen.Calls) != 2 {
			t.Fatalf("generation client called %d times, want 2", len(gen.Calls))
		}
		if len(review.Calls) != 1 {
			t.Fatalf("review client called %d times, want 1", len(review.Calls))
		}
		if !strings.Contains(gen.Calls[1].Request.UserPrompt, markGenBody) {
			t.Errorf("shorten prompt = %q, want it to carry the generation body %q", gen.Calls[1].Request.UserPrompt, markGenBody)
		}
		reviewPrompt := review.Calls[0].Request.SystemPrompt + review.Calls[0].Request.UserPrompt
		if !strings.Contains(reviewPrompt, markShortBody) {
			t.Errorf("review prompt does not carry the shortened body %q", markShortBody)
		}
		if strings.Contains(reviewPrompt, markGenBody) {
			t.Errorf("review prompt carries the generation body %q, want the shortened body", markGenBody)
		}
		if a.Shorten == nil {
			t.Fatal("Shorten = nil, want the shorten step's model")
		}
		if a.Shorten.Model != markShortenModel || a.Shorten.ModelVersion != markStepModelVersion {
			t.Errorf("Shorten = %+v, want model %q version %q", a.Shorten, markShortenModel, markStepModelVersion)
		}
		if a.Title != markShortTitle {
			t.Errorf("Title = %q, want the shortened title %q", a.Title, markShortTitle)
		}
		if !strings.HasPrefix(a.Body, markShortBody) {
			t.Errorf("Body = %q, want it to start with the shortened body %q", a.Body, markShortBody)
		}
	})
}

func TestWriteBodyLength(t *testing.T) {
	// The title is long and the body carries a line feed, so only the body
	// part's code points minus line feeds can decide the branch.
	title := strings.Repeat("t", 500)
	cases := []struct {
		name        string
		body        string
		wantShorten bool
	}{
		{"at the limit", "x\n" + strings.Repeat("a", maxBodyChars-1), false},
		{"one over the limit", "x\n" + strings.Repeat("a", maxBodyChars), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps := []llmtestutil.FakeLLMSequenceStep{
				stepResult(llm.GenerateResponse{Text: stepResponseText(title, tc.body), Model: markOutModel}),
			}
			if tc.wantShorten {
				steps = append(steps, stepResult(llm.GenerateResponse{Text: stepResponseText(title, markShortBody), Model: markShortenModel}))
			}
			gen := newStepClient(t, steps...)
			review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
			w := mustNew(t, gen, withStepsEnabled(review))

			if _, err := w.Write(context.Background(), validTranscript()); err != nil {
				t.Fatalf("Write error = %v, want nil", err)
			}
			wantCalls := 1
			if tc.wantShorten {
				wantCalls = 2
			}
			if len(gen.Calls) != wantCalls {
				t.Errorf("generation client called %d times, want %d", len(gen.Calls), wantCalls)
			}
		})
	}
}

func TestShortenPrompt(t *testing.T) {
	gen := newStepClient(t,
		stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
		stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel}),
	)
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
	w := mustNew(t, gen, withStepsEnabled(review))

	if _, err := w.Write(context.Background(), validTranscript()); err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	prompt := gen.Calls[1].Request.SystemPrompt + gen.Calls[1].Request.UserPrompt
	for _, want := range []string{markGenTitle, markGenBody, strconv.Itoa(targetBodyChars), strconv.Itoa(maxBodyChars)} {
		if !strings.Contains(prompt, want) {
			t.Errorf("shorten prompt does not contain %q", want)
		}
	}
	// The shorten step must not see the material: no transcript, no video
	// title, no channel name, no description.
	for _, mark := range []string{markSegment, markTitle, markChannel, markDescription} {
		if strings.Contains(prompt, mark) {
			t.Errorf("shorten prompt contains the material value marked %q", mark)
		}
	}
}

func TestReviewPrompt(t *testing.T) {
	const (
		genOpen, genClose = "<gen>", "</gen>"
		revOpen, revClose = "<rev>", "</rev>"
	)
	genUser := genOpen + "{{.Transcript}}" + genClose + " {{.Title}}"
	reviewUser := revOpen + "{{.Transcript}}" + revClose + " {{.Title}} {{.ChannelName}} {{.Description}} {{.ArticleTitle}} {{.ArticleBody}}"

	gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
	w := mustNew(t, gen, Options{
		UserTemplatePath: writeOverrideFile(t, genUser),
		Review:           ReviewOptions{Enabled: true, Client: review, UserTemplatePath: writeOverrideFile(t, reviewUser)},
	})

	tr := validTranscript()
	if _, err := w.Write(context.Background(), tr); err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	genPrompt := gen.Calls[0].Request.UserPrompt
	reviewPrompt := review.Calls[0].Request.UserPrompt

	extract := func(t *testing.T, text, openTag, closeTag string) string {
		t.Helper()
		start := strings.Index(text, openTag)
		end := strings.Index(text, closeTag)
		if start < 0 || end < start {
			t.Fatalf("prompt %q does not contain %q ... %q", text, openTag, closeTag)
		}
		return text[start+len(openTag) : end]
	}
	genTranscript := extract(t, genPrompt, genOpen, genClose)
	reviewTranscript := extract(t, reviewPrompt, revOpen, revClose)
	if genTranscript != reviewTranscript {
		t.Errorf("review transcript = %q, want the generation transcript %q", reviewTranscript, genTranscript)
	}
	for _, want := range []string{markTitle, markChannel, markDescription, markGenTitle, markGenBody} {
		if !strings.Contains(reviewPrompt, want) {
			t.Errorf("review prompt does not contain the value marked %q", want)
		}
	}
}

func TestWriteStepMalformedOutput(t *testing.T) {
	t.Run("shorten output", func(t *testing.T) {
		cases := []struct{ name, text string }{
			{"first line is not a heading", "no heading\nbody"},
			{"body is empty", "# T\n \t "},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				gen := newStepClient(t,
					stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
					stepResult(llm.GenerateResponse{Text: tc.text, Model: markOutModel}),
				)
				review := &llmtestutil.FakeLLMClient{}
				w := mustNew(t, gen, withStepsEnabled(review))

				a, err := w.Write(context.Background(), validTranscript())
				requireStepError(t, err, ErrMalformedOutput, StepShorten, a)
				if len(review.Calls) != 0 {
					t.Errorf("review client called %d times, want 0", len(review.Calls))
				}
			})
		}
	})

	t.Run("applied text", func(t *testing.T) {
		// Deleting the whole title leaves an empty title, which the
		// generated-text check rejects.
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
			Text:  reviewResponse(deleteItem(markGenTitle, "unsupported", markRevisionEvidence)),
			Model: markReviewModel,
		}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		requireStepError(t, err, ErrMalformedOutput, StepReview, a)
	})
}

func TestWriteTooLong(t *testing.T) {
	t.Run("shorten step", func(t *testing.T) {
		gen := newStepClient(t,
			stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
			stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, overLimitShortenBody()), Model: markShortenModel}),
		)
		review := &llmtestutil.FakeLLMClient{}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		requireStepError(t, err, ErrTooLong, StepShorten, a)
		if want := strconv.Itoa(maxBodyChars + 1); !strings.Contains(err.Error(), want) {
			t.Errorf("Write error = %v, want it to contain the counted length %s", err, want)
		}
		if want := "limit " + strconv.Itoa(maxBodyChars); !strings.Contains(err.Error(), want) {
			t.Errorf("Write error = %v, want it to contain the limit %s", err, want)
		}
		if len(review.Calls) != 0 {
			t.Errorf("review client called %d times, want 0", len(review.Calls))
		}
	})

	t.Run("review step", func(t *testing.T) {
		// The review step lengthens the body past the limit.
		longer := markShortBody + strings.Repeat("a", maxBodyChars)
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
			Text:  reviewResponse(replaceItem(markGenBody, longer, "meaning_change", markRevisionEvidence)),
			Model: markReviewModel,
		}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		requireStepError(t, err, ErrTooLong, StepReview, a)
	})

	t.Run("no shorten skips the final check", func(t *testing.T) {
		longer := markShortBody + strings.Repeat("a", maxBodyChars)
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
			Text:  reviewResponse(replaceItem(markGenBody, longer, "meaning_change", markRevisionEvidence)),
			Model: markReviewModel,
		}}
		w := mustNew(t, gen, Options{Review: ReviewOptions{Enabled: true, Client: review}})

		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil (the final length check is off)", err)
		}
		if a == (Article{}) {
			t.Error("Write article is the zero value, want the reviewed article")
		}
	})
}

func TestWriteReviewApplies(t *testing.T) {
	t.Run("applies a revision", func(t *testing.T) {
		edited := markGenTitle + "-edited"
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
			Text:  reviewResponse(replaceItem(markGenTitle, edited, "meaning_change", markRevisionEvidence)),
			Model: markReviewModel,
		}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		if a.Title != edited {
			t.Errorf("Title = %q, want the revised title %q", a.Title, edited)
		}
		if !strings.HasPrefix(a.Body, markGenBody) {
			t.Errorf("Body = %q, want it to start with the unchanged body %q", a.Body, markGenBody)
		}
		if a.Review == nil {
			t.Fatal("Review = nil, want the review result")
		}
		want := []Revision{{Before: markGenTitle, Action: ActionReplace, After: edited, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}
		if !slices.Equal(a.Review.Revisions, want) {
			t.Errorf("Review.Revisions = %+v, want %+v", a.Review.Revisions, want)
		}
		if a.Review.Model.Model != markReviewModel {
			t.Errorf("Review.Model.Model = %q, want %q", a.Review.Model.Model, markReviewModel)
		}
	})

	t.Run("applies a no-change revision", func(t *testing.T) {
		// The response holds one replacement whose before and after are the
		// same string, so the reviewed title is unchanged and Write still
		// succeeds with the revision kept in response order.
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
			Text:  reviewResponse(replaceItem(markGenTitle, markGenTitle, "meaning_change", markRevisionEvidence)),
			Model: markReviewModel,
		}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		if a == (Article{}) {
			t.Fatal("Write article is the zero value, want the reviewed article")
		}
		if a.Title != markGenTitle {
			t.Errorf("Title = %q, want the unchanged title %q", a.Title, markGenTitle)
		}
		if !strings.HasPrefix(a.Body, markGenBody) {
			t.Errorf("Body = %q, want it to start with the unchanged body %q", a.Body, markGenBody)
		}
		if a.Review == nil {
			t.Fatal("Review = nil, want the review result")
		}
		want := []Revision{{Before: markGenTitle, Action: ActionReplace, After: markGenTitle, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}
		if !slices.Equal(a.Review.Revisions, want) {
			t.Errorf("Review.Revisions = %+v, want %+v", a.Review.Revisions, want)
		}
	})

	t.Run("applies a no-change and a changed revision", func(t *testing.T) {
		edited := markGenBody + "-edited"
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
			Text: reviewResponse(
				replaceItem(markGenTitle, markGenTitle, "meaning_change", markRevisionEvidence),
				replaceItem(markGenBody, edited, "meaning_change", markRevisionEvidence),
			),
			Model: markReviewModel,
		}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		if a == (Article{}) {
			t.Fatal("Write article is the zero value, want the reviewed article")
		}
		if a.Title != markGenTitle {
			t.Errorf("Title = %q, want the unchanged title %q", a.Title, markGenTitle)
		}
		if !strings.HasPrefix(a.Body, edited) {
			t.Errorf("Body = %q, want it to start with the revised body %q", a.Body, edited)
		}
		if a.Review == nil {
			t.Fatal("Review = nil, want the review result")
		}
		want := []Revision{
			{Before: markGenTitle, Action: ActionReplace, After: markGenTitle, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence},
			{Before: markGenBody, Action: ActionReplace, After: edited, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence},
		}
		if !slices.Equal(a.Review.Revisions, want) {
			t.Errorf("Review.Revisions = %+v, want %+v", a.Review.Revisions, want)
		}
	})

	t.Run("empty list leaves the text", func(t *testing.T) {
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(context.Background(), validTranscript())
		if err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		if a.Title != markGenTitle {
			t.Errorf("Title = %q, want %q", a.Title, markGenTitle)
		}
		wantBody := markGenBody + bodySeparator + sourceBlock(testSourceURL)
		if a.Body != wantBody {
			t.Errorf("Body = %q, want %q", a.Body, wantBody)
		}
		if a.Review == nil {
			t.Fatal("Review = nil, want the (empty) review result, not an inference that the step did not run")
		}
		if len(a.Review.Revisions) != 0 {
			t.Errorf("Review.Revisions = %+v, want empty", a.Review.Revisions)
		}
	})
}

func TestWriteReviewRejects(t *testing.T) {
	cases := []struct {
		name     string
		response string
	}{
		{"not JSON", "not json"},
		{"missing revisions", `{}`},
		{"before not found", reviewResponse(replaceItem("zzz-not-in-the-text", "x", "unsupported", markRevisionEvidence))},
		{"before creates a heading", reviewResponse(replaceItem(markGenBody, "## heading", "unsupported", markRevisionEvidence))},
		{"before has a line feed", reviewResponse(replaceItem(markGenBody+"\n", markGenBody, "unsupported", markRevisionEvidence))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
			review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: tc.response, Model: markReviewModel}}
			w := mustNew(t, gen, withStepsEnabled(review))

			a, err := w.Write(context.Background(), validTranscript())
			requireStepError(t, err, ErrMalformedOutput, StepReview, a)
		})
	}
}

// TestWriteReviewModelString checks that the review step validates the model
// name and the model version of its response, not only its parsed list.
func TestWriteReviewModelString(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		version string
	}{
		{"empty model", "", ""},
		{"whitespace model", " \t", ""},
		{"control character in model", "review\x1b[31m", ""},
		{"control character in model version", markReviewModel, "v\x00"},
		{"newline in model version", markReviewModel, "v\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
			review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: tc.model, ModelVersion: tc.version}}
			w := mustNew(t, gen, withStepsEnabled(review))

			a, err := w.Write(context.Background(), validTranscript())
			requireStepError(t, err, ErrMalformedOutput, StepReview, a)
		})
	}
}

func TestWriteStepFailures(t *testing.T) {
	stepSentinel := errors.New("step-specific failure")
	targets := []struct {
		name   string
		target error
	}{
		{"truncated", llm.ErrTruncated},
		{"empty response", llm.ErrEmptyResponse},
		{"deadline", context.DeadlineExceeded},
		{"step sentinel", stepSentinel},
	}
	steps := []struct {
		name string
		step Step
	}{
		{"shorten", StepShorten},
		{"review", StepReview},
	}
	for _, st := range steps {
		for _, tc := range targets {
			t.Run(st.name+"/"+tc.name, func(t *testing.T) {
				failure := fmt.Errorf("fake: %w", tc.target)
				var gen *llmtestutil.FakeLLMSequence
				var review *llmtestutil.FakeLLMClient
				if st.step == StepShorten {
					gen = newStepClient(t,
						stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
						llmtestutil.FakeLLMSequenceStep{Err: failure},
					)
					review = &llmtestutil.FakeLLMClient{}
				} else {
					gen = newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
					review = &llmtestutil.FakeLLMClient{Err: failure}
				}
				w := mustNew(t, gen, withStepsEnabled(review))

				a, err := w.Write(context.Background(), validTranscript())
				requireStepError(t, err, tc.target, st.step, a)
				if len(review.Calls) != 0 && st.step == StepShorten {
					t.Errorf("review client called %d times, want 0", len(review.Calls))
				}
			})
		}
	}
}

func TestWriteStepContextDone(t *testing.T) {
	t.Run("shorten not run", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Respond: func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			cancel()
			return llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}, nil
		}})
		review := &llmtestutil.FakeLLMClient{}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(ctx, validTranscript())
		requireStepError(t, err, context.Canceled, StepShorten, a)
		if len(gen.Calls) != 1 {
			t.Errorf("generation client called %d times, want 1 (the shorten step must not run)", len(gen.Calls))
		}
		if len(review.Calls) != 0 {
			t.Errorf("review client called %d times, want 0", len(review.Calls))
		}
	})

	t.Run("review not run", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Respond: func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			cancel()
			return llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}, nil
		}})
		review := &llmtestutil.FakeLLMClient{}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(ctx, validTranscript())
		requireStepError(t, err, context.Canceled, StepReview, a)
		if len(review.Calls) != 0 {
			t.Errorf("review client called %d times, want 0", len(review.Calls))
		}
	})

	t.Run("review not run after the shorten step", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		gen := newStepClient(t,
			stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
			llmtestutil.FakeLLMSequenceStep{Respond: func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
				cancel()
				return llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel}, nil
			}},
		)
		review := &llmtestutil.FakeLLMClient{}
		w := mustNew(t, gen, withStepsEnabled(review))

		a, err := w.Write(ctx, validTranscript())
		requireStepError(t, err, context.Canceled, StepReview, a)
		if len(gen.Calls) != 2 {
			t.Errorf("generation client called %d times, want 2", len(gen.Calls))
		}
		if len(review.Calls) != 0 {
			t.Errorf("review client called %d times, want 0", len(review.Calls))
		}
	})
}

func TestWriteStepExpansionFailure(t *testing.T) {
	t.Run("shorten template", func(t *testing.T) {
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText("short", overLimitBody()), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{}
		w := mustNew(t, gen, Options{
			Shorten: ShortenOptions{Enabled: true, UserTemplatePath: writeOverrideFile(t, "{{index .ArticleTitle 100}}")},
			Review:  ReviewOptions{Enabled: true, Client: review},
		})

		a, err := w.Write(context.Background(), validTranscript())
		requireStepError(t, err, ErrInvalidTemplate, StepShorten, a)
		if !strings.Contains(err.Error(), shortenUserTemplateName) {
			t.Errorf("Write error = %v, want it to name the %s template", err, shortenUserTemplateName)
		}
	})

	t.Run("review prompt too large", func(t *testing.T) {
		tr := validTranscript()
		tr.Segments = []transcript.Segment{{Text: strings.Repeat("a", maxPromptBytes+1)}}
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{}
		w := mustNew(t, gen, Options{
			// The generation prompt must not embed the transcript, so it
			// stays small and the review prompt is the first to overflow.
			UserTemplatePath: writeOverrideFile(t, "x"),
			Review:           ReviewOptions{Enabled: true, Client: review, UserTemplatePath: writeOverrideFile(t, "{{.Transcript}}")},
		})

		a, err := w.Write(context.Background(), tr)
		requireStepError(t, err, ErrInvalidTemplate, StepReview, a)
		if !strings.Contains(err.Error(), reviewUserTemplateName) {
			t.Errorf("Write error = %v, want it to name the %s template", err, reviewUserTemplateName)
		}
	})
}
