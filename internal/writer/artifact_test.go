//go:build test

package writer

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/prompts"
)

// Models and identifiers of a reused (resumed) artifact, distinct from the
// models an executed step reports, so the article's provenance shows which
// stage each value came from.
const (
	reusedGenModel      = "reused-gen-model"
	reusedGenVersion    = "reused-gen-version"
	reusedShortModel    = "reused-short-model"
	reusedShortVersion  = "reused-short-version"
	reusedReviewModel   = "reused-review-model"
	reusedReviewVersion = "reused-review-version"
	genRecordID         = "record-generation"
	shortenRecordID     = "record-shorten"
	reviewRecordID      = "record-review"
)

// sameArtifact reports whether two stage records carry the same values,
// ignoring the store-assigned id.
func sameArtifact(a, b StageRecord) bool {
	a.ID, b.ID = "", ""
	return a.Stage == b.Stage && a.Title == b.Title && a.Body == b.Body &&
		a.Model == b.Model && a.ModelVersion == b.ModelVersion &&
		a.System == b.System && a.User == b.User && a.Input == b.Input &&
		slices.Equal(a.Revisions, b.Revisions)
}

// defaultIdentity is the identity of an embedded default template.
func defaultIdentity(embedded string) TemplateIdentity {
	return TemplateIdentity{Embedded: true, Digest: Digest(embedded)}
}

// generationRecord is the artifact the writer saves for a run whose generation
// returned markGenTitle and body.
func generationRecord(materialID, body string) StageRecord {
	return StageRecord{
		Stage:        StageGeneration,
		Title:        markGenTitle,
		Body:         body,
		Model:        markOutModel,
		ModelVersion: markOutModelVersion,
		System:       defaultIdentity(prompts.System()),
		User:         defaultIdentity(prompts.User()),
		Input:        materialID,
	}
}

// resumeChainGeneration is a loaded chain whose generation is reused.
func resumeChainGeneration() ArtifactChain {
	return ArtifactChain{Generation: StageRecord{
		Stage: StageGeneration, Title: markGenTitle, Body: markGenBody,
		Model: reusedGenModel, ModelVersion: reusedGenVersion, ID: genRecordID,
	}}
}

// resumeChainShorten adds a reused shorten stage.
func resumeChainShorten() ArtifactChain {
	chain := resumeChainGeneration()
	chain.Shorten = &StageRecord{
		Stage: StageShorten, Title: markShortTitle, Body: markShortBody,
		Model: reusedShortModel, ModelVersion: reusedShortVersion, ID: shortenRecordID,
	}
	return chain
}

// resumeChainReview adds a reused review stage.
func resumeChainReview(revisions []Revision) ArtifactChain {
	chain := resumeChainShorten()
	chain.Review = &StageRecord{
		Stage: StageReview, Title: markShortTitle, Body: markShortBody + "-reused",
		Model: reusedReviewModel, ModelVersion: reusedReviewVersion,
		Revisions: revisions, ID: reviewRecordID,
	}
	return chain
}

func TestWriteSavesAllArtifacts(t *testing.T) {
	tr := validTranscript()
	materialID := MaterialID(tr)
	appliedBody := markShortBody + "-revised"
	gen := newStepClient(t,
		stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel, ModelVersion: markOutModelVersion}),
		stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel, ModelVersion: markStepModelVersion}),
	)
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{
		Text:  reviewResponse(replaceItem(markShortBody, appliedBody, "meaning_change", markRevisionEvidence)),
		Model: markReviewModel,
	}}
	store := &fakeArtifactStore{}
	w := mustNew(t, gen, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store},
	})

	a, err := w.Write(context.Background(), tr)
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if got := store.saveStages(); !slices.Equal(got, []Stage{StageGeneration, StageShorten, StageReview}) {
		t.Fatalf("Save stages = %v, want generation, shorten, review", got)
	}

	generation := store.saveFor(StageGeneration)
	if !sameArtifact(generation.record, generationRecord(materialID, overLimitBody())) {
		t.Errorf("generation record = %+v, want %+v", generation.record, generationRecord(materialID, overLimitBody()))
	}
	if generation.videoID != tr.VideoID {
		t.Errorf("Save videoID = %q, want %q", generation.videoID, tr.VideoID)
	}
	shorten := store.saveFor(StageShorten)
	wantShorten := StageRecord{
		Stage: StageShorten, Title: markShortTitle, Body: markShortBody,
		Model: markShortenModel, ModelVersion: markStepModelVersion,
		System: defaultIdentity(prompts.ShortenSystem()), User: defaultIdentity(prompts.ShortenUser()),
		Input: "id-generation",
	}
	if !sameArtifact(shorten.record, wantShorten) {
		t.Errorf("shorten record = %+v, want %+v", shorten.record, wantShorten)
	}
	reviewCall := store.saveFor(StageReview)
	wantReview := StageRecord{
		Stage: StageReview, Title: markShortTitle, Body: appliedBody,
		Model: markReviewModel, Revisions: []Revision{{
			Before: markShortBody, Action: ActionReplace, After: appliedBody,
			Reason: ReasonMeaningChange, Evidence: markRevisionEvidence,
		}},
		System: defaultIdentity(prompts.ReviewSystem()), User: defaultIdentity(prompts.ReviewUser()),
		Input: "id-shorten",
	}
	if !sameArtifact(reviewCall.record, wantReview) {
		t.Errorf("review record = %+v, want %+v", reviewCall.record, wantReview)
	}

	if a.Model != markOutModel || a.ModelVersion != markOutModelVersion {
		t.Errorf("Article model = %q/%q, want %q/%q", a.Model, a.ModelVersion, markOutModel, markOutModelVersion)
	}
	if a.Shorten == nil || a.Shorten.Model != markShortenModel {
		t.Errorf("Article.Shorten = %+v, want model %q", a.Shorten, markShortenModel)
	}
	if a.Review == nil || a.Review.Model.Model != markReviewModel {
		t.Errorf("Article.Review = %+v, want model %q", a.Review, markReviewModel)
	}
	if !strings.HasPrefix(a.Body, appliedBody) {
		t.Errorf("Article body = %q, want it to start with the applied body %q", a.Body, appliedBody)
	}
}

func TestWriteSavesOnlyRunStages(t *testing.T) {
	t.Run("no shorten when the body is within the limit", func(t *testing.T) {
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
		store := &fakeArtifactStore{}
		w := mustNew(t, gen, Options{
			Shorten:   ShortenOptions{Enabled: true},
			Review:    ReviewOptions{Enabled: true, Client: review},
			Artifacts: ArtifactOptions{Store: store},
		})
		if _, err := w.Write(context.Background(), validTranscript()); err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		if got := store.saveStages(); !slices.Equal(got, []Stage{StageGeneration, StageReview}) {
			t.Errorf("Save stages = %v, want generation and review only", got)
		}
	})

	t.Run("generation only", func(t *testing.T) {
		gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}))
		store := &fakeArtifactStore{}
		w := mustNew(t, gen, Options{Artifacts: ArtifactOptions{Store: store}})
		if _, err := w.Write(context.Background(), validTranscript()); err != nil {
			t.Fatalf("Write error = %v, want nil", err)
		}
		if got := store.saveStages(); !slices.Equal(got, []Stage{StageGeneration}) {
			t.Errorf("Save stages = %v, want generation only", got)
		}
	})
}

func TestWriteNoStoreSavesNothing(t *testing.T) {
	// A nil store is not saved to. A typed-nil store must behave the same:
	// without normalizing it in New, Write would call Save on a nil receiver
	// and panic.
	cases := []struct {
		name string
		opts Options
	}{
		{"nil store", Options{}},
		{"typed-nil store", Options{Artifacts: ArtifactOptions{Store: (*fakeArtifactStore)(nil)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newFakeClient()
			w := mustNew(t, client, tc.opts)
			if _, err := w.Write(context.Background(), validTranscript()); err != nil {
				t.Fatalf("Write error = %v, want nil", err)
			}
			if len(client.Calls) != 1 {
				t.Errorf("Generate called %d times, want 1", len(client.Calls))
			}
		})
	}
}

func TestWriteSavesBeforeLaterFailure(t *testing.T) {
	// The review step fails, but the generation and shorten saves already
	// happened.
	gen := newStepClient(t,
		stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
		stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel}),
	)
	review := &llmtestutil.FakeLLMClient{Err: errors.New("review failed")}
	store := &fakeArtifactStore{}
	w := mustNew(t, gen, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store},
	})

	a, err := w.Write(context.Background(), validTranscript())
	if err == nil {
		t.Fatal("Write error = nil, want the review failure")
	}
	if a != (Article{}) {
		t.Errorf("Write article = %+v, want the zero value", a)
	}
	if got := store.saveStages(); !slices.Equal(got, []Stage{StageGeneration, StageShorten}) {
		t.Errorf("Save stages = %v, want generation and shorten", got)
	}
}

func TestWriteSaveFailure(t *testing.T) {
	cases := []struct {
		name   string
		stage  Stage
		review bool
		opts   func(*fakeArtifactStore, llm.LLMClient) Options
		calls  []llmtestutil.FakeLLMSequenceStep
	}{
		{
			name:  "generation",
			stage: StageGeneration,
			opts: func(store *fakeArtifactStore, _ llm.LLMClient) Options {
				return Options{Artifacts: ArtifactOptions{Store: store}}
			},
			calls: []llmtestutil.FakeLLMSequenceStep{stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel})},
		},
		{
			name:  "shorten",
			stage: StageShorten,
			opts: func(store *fakeArtifactStore, _ llm.LLMClient) Options {
				return Options{Shorten: ShortenOptions{Enabled: true}, Artifacts: ArtifactOptions{Store: store}}
			},
			calls: []llmtestutil.FakeLLMSequenceStep{
				stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
				stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel}),
			},
		},
		{
			name:   "review",
			stage:  StageReview,
			review: true,
			opts: func(store *fakeArtifactStore, review llm.LLMClient) Options {
				return Options{Review: ReviewOptions{Enabled: true, Client: review}, Artifacts: ArtifactOptions{Store: store}}
			},
			calls: []llmtestutil.FakeLLMSequenceStep{stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel})},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var review llm.LLMClient
			if tc.review {
				review = &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
			}
			store := &fakeArtifactStore{saveErrs: map[Stage]error{tc.stage: errors.New("save failed")}}
			gen := newStepClient(t, tc.calls...)
			w := mustNew(t, gen, tc.opts(store, review))

			a, err := w.Write(context.Background(), validTranscript())
			if a != (Article{}) {
				t.Errorf("Write article = %+v, want the zero value", a)
			}
			artifactErr, ok := errors.AsType[*ArtifactError](err)
			if !ok {
				t.Fatalf("Write error = %v, want a *ArtifactError", err)
			}
			if artifactErr.Stage != tc.stage {
				t.Errorf("ArtifactError.Stage = %v, want %v", artifactErr.Stage, tc.stage)
			}
			if !strings.Contains(err.Error(), tc.stage.String()+" artifact") {
				t.Errorf("Write error = %v, want it to name the %s stage", err, tc.stage)
			}
		})
	}
}

func TestWriteSaveContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	gen := newStepClient(t, llmtestutil.FakeLLMSequenceStep{Respond: func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		cancel()
		return llm.GenerateResponse{Text: stepResponseText(markGenTitle, markGenBody), Model: markOutModel}, nil
	}})
	store := &fakeArtifactStore{}
	w := mustNew(t, gen, Options{Artifacts: ArtifactOptions{Store: store}})

	a, err := w.Write(ctx, validTranscript())
	if a != (Article{}) {
		t.Errorf("Write article = %+v, want the zero value", a)
	}
	artifactErr, ok := errors.AsType[*ArtifactError](err)
	if !ok || artifactErr.Stage != StageGeneration {
		t.Fatalf("Write error = %v, want a *ArtifactError for generation", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Write error = %v, want context.Canceled", err)
	}
	if len(store.saveCalls) != 0 {
		t.Errorf("Save called %d times, want 0 (the context was done before the save)", len(store.saveCalls))
	}
}

func TestResumeAfterGeneration(t *testing.T) {
	tr := validTranscript()
	gen := newStepClient(t) // fails the test if the generation client is called
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
	store := &fakeArtifactStore{loadResult: resumeChainGeneration()}
	w := mustNew(t, gen, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration},
	})

	a, err := w.Write(context.Background(), tr)
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if len(store.loadCalls) != 1 {
		t.Fatalf("Load called %d times, want 1", len(store.loadCalls))
	}
	load := store.loadCalls[0]
	if load.stage != StageGeneration || load.videoID != tr.VideoID || load.materialID != MaterialID(tr) {
		t.Errorf("Load args = %+v, want generation, %q, the material id", load, tr.VideoID)
	}
	if len(gen.Calls) != 0 {
		t.Errorf("generation client called %d times, want 0 (the generation was reused and the body is within the limit)", len(gen.Calls))
	}
	if len(review.Calls) != 1 {
		t.Errorf("review client called %d times, want 1", len(review.Calls))
	}
	if a.Model != reusedGenModel || a.ModelVersion != reusedGenVersion {
		t.Errorf("Article model = %q/%q, want the reused %q/%q", a.Model, a.ModelVersion, reusedGenModel, reusedGenVersion)
	}
	if !strings.HasPrefix(a.Body, markGenBody) {
		t.Errorf("Article body = %q, want it to start with the reused generation body %q", a.Body, markGenBody)
	}
	if a.Shorten != nil {
		t.Errorf("Article.Shorten = %+v, want nil (the shorten step did not run)", a.Shorten)
	}
}

func TestResumeSavesExecutedStages(t *testing.T) {
	// The reused generation body is over the limit, so the shorten step runs on
	// the generation client and only that stage's save is added.
	chain := resumeChainGeneration()
	chain.Generation.Body = overLimitBody()
	gen := newStepClient(t, stepResult(llm.GenerateResponse{Text: stepResponseText(markShortTitle, markShortBody), Model: markShortenModel}))
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
	store := &fakeArtifactStore{loadResult: chain}
	w := mustNew(t, gen, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration},
	})

	if _, err := w.Write(context.Background(), validTranscript()); err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if len(gen.Calls) != 1 {
		t.Fatalf("generation client called %d times, want 1 (the shorten step)", len(gen.Calls))
	}
	if !strings.Contains(gen.Calls[0].Request.UserPrompt, markGenBody) {
		t.Errorf("the single generation-client call did not carry the reused generation body")
	}
	if got := store.saveStages(); !slices.Equal(got, []Stage{StageShorten, StageReview}) {
		t.Errorf("Save stages = %v, want shorten and review (the generation was reused)", got)
	}
	shorten := store.saveFor(StageShorten)
	if shorten.record.Input != genRecordID {
		t.Errorf("shorten input = %q, want the reused generation id %q", shorten.record.Input, genRecordID)
	}
}

func TestResumeAfterShorten(t *testing.T) {
	tr := validTranscript()
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel}}
	store := &fakeArtifactStore{loadResult: resumeChainShorten()}
	// The generation client is nil: neither generation nor shorten runs, so a
	// call to it would panic.
	w := mustNew(t, nil, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store, Resume: StageShorten},
	})

	a, err := w.Write(context.Background(), tr)
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if store.loadCalls[0].stage != StageShorten {
		t.Errorf("Load stage = %v, want shorten", store.loadCalls[0].stage)
	}
	if len(review.Calls) != 1 {
		t.Errorf("review client called %d times, want 1", len(review.Calls))
	}
	if a.Shorten == nil || a.Shorten.Model != reusedShortModel || a.Shorten.ModelVersion != reusedShortVersion {
		t.Errorf("Article.Shorten = %+v, want the reused %q/%q", a.Shorten, reusedShortModel, reusedShortVersion)
	}
	if a.Title != markShortTitle || !strings.HasPrefix(a.Body, markShortBody) {
		t.Errorf("Article text = %q/%q, want the reused shorten text", a.Title, a.Body)
	}
	reviewCall := store.saveFor(StageReview)
	if reviewCall.record.Input != shortenRecordID {
		t.Errorf("review input = %q, want the reused shorten id %q", reviewCall.record.Input, shortenRecordID)
	}
}

func TestResumeAfterReview(t *testing.T) {
	tr := validTranscript()
	revisions := []Revision{{Before: markShortBody, Action: ActionReplace, After: markShortBody + "-reused", Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}
	store := &fakeArtifactStore{loadResult: resumeChainReview(revisions)}
	// No clients: no stage runs, so New accepts nil for both.
	w := mustNew(t, nil, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true},
		Artifacts: ArtifactOptions{Store: store, Resume: StageReview},
	})

	a, err := w.Write(context.Background(), tr)
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if store.loadCalls[0].stage != StageReview {
		t.Errorf("Load stage = %v, want review", store.loadCalls[0].stage)
	}
	if len(store.saveCalls) != 0 {
		t.Errorf("Save called %d times, want 0 (no stage ran)", len(store.saveCalls))
	}
	if a.Title != markShortTitle || !strings.HasPrefix(a.Body, markShortBody+"-reused") {
		t.Errorf("Article text = %q/%q, want the reused review text", a.Title, a.Body)
	}
	if a.Review == nil || a.Review.Model.Model != reusedReviewModel || !slices.Equal(a.Review.Revisions, revisions) {
		t.Errorf("Article.Review = %+v, want the reused review %q with its list", a.Review, reusedReviewModel)
	}
	if a.Shorten == nil || a.Shorten.Model != reusedShortModel {
		t.Errorf("Article.Shorten = %+v, want the reused shorten model %q", a.Shorten, reusedShortModel)
	}
}

func TestResumeFinalLengthAndReviewLog(t *testing.T) {
	// The reused review text is over the limit and Shorten is enabled, so the
	// final length check still rejects it.
	chain := resumeChainReview(nil)
	chain.Review.Body = overLimitBody()
	store := &fakeArtifactStore{loadResult: chain}
	w := mustNew(t, nil, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Artifacts: ArtifactOptions{Store: store, Resume: StageReview},
	})

	a, err := w.Write(context.Background(), validTranscript())
	requireStepError(t, err, ErrTooLong, StepReview, a)
}

func TestResumeAfterReviewNoShortenSkipsFinalLength(t *testing.T) {
	chain := resumeChainReview(nil)
	chain.Review.Body = overLimitBody()
	store := &fakeArtifactStore{loadResult: chain}
	w := mustNew(t, nil, Options{Artifacts: ArtifactOptions{Store: store, Resume: StageReview}})

	a, err := w.Write(context.Background(), validTranscript())
	if err != nil {
		t.Fatalf("Write error = %v, want nil (Shorten is disabled, so the final check is off)", err)
	}
	if a == (Article{}) {
		t.Error("Write article is the zero value, want the reused review article")
	}
}

func TestResumeLoadFailure(t *testing.T) {
	tr := validTranscript()
	loadErr := errors.New("load failed")
	store := &fakeArtifactStore{loadErr: loadErr}
	// The generation client is nil: neither generation nor shorten runs.
	w := mustNew(t, nil, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Artifacts: ArtifactOptions{Store: store, Resume: StageShorten},
	})

	a, err := w.Write(context.Background(), tr)
	if !errors.Is(err, loadErr) {
		t.Fatalf("Write error = %v, want it to wrap %v", err, loadErr)
	}
	artifactErr, ok := errors.AsType[*ArtifactError](err)
	if !ok || artifactErr.Stage != StageShorten {
		t.Errorf("Write error = %v, want a *ArtifactError for shorten", err)
	}
	if a != (Article{}) {
		t.Errorf("Write article = %+v, want the zero value", a)
	}
}

func TestNewArtifactOptions(t *testing.T) {
	client := newFakeClient()
	store := &fakeArtifactStore{}
	rejected := []struct {
		name   string
		want   error
		client llm.LLMClient
		opts   Options
	}{
		{"resume without a store", errResumeWithoutStore, client, Options{Artifacts: ArtifactOptions{Resume: StageGeneration}}},
		{
			"resume with a typed-nil store",
			errResumeWithoutStore,
			client,
			Options{Artifacts: ArtifactOptions{Store: (*fakeArtifactStore)(nil), Resume: StageGeneration}},
		},
		{
			"unknown resume stage",
			errUnknownResume,
			client,
			Options{Artifacts: ArtifactOptions{Store: store, Resume: Stage(99)}},
		},
		{
			"generation client set but unused",
			errGenerationClientSet,
			client,
			Options{Artifacts: ArtifactOptions{Store: store, Resume: StageShorten}},
		},
		{
			"shorten runs on a generation resume without a client",
			errNilLLMClient,
			nil,
			Options{Shorten: ShortenOptions{Enabled: true}, Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration}},
		},
		{
			"review client set for a review resume",
			errReviewClientSet,
			nil,
			Options{Review: ReviewOptions{Enabled: true, Client: client}, Artifacts: ArtifactOptions{Store: store, Resume: StageReview}},
		},
		{
			"generation template path for a resume",
			errDisabledStepTemplate,
			nil,
			Options{SystemTemplatePath: writeOverrideFile(t, "x"), Artifacts: ArtifactOptions{Store: store, Resume: StageShorten}},
		},
		{
			"shorten template path for a review resume",
			errDisabledStepTemplate,
			nil,
			Options{Shorten: ShortenOptions{Enabled: true, UserTemplatePath: writeOverrideFile(t, "x")}, Artifacts: ArtifactOptions{Store: store, Resume: StageReview}},
		},
		{
			"review runs without a client",
			errReviewClientNil,
			nil,
			Options{Review: ReviewOptions{Enabled: true}, Artifacts: ArtifactOptions{Store: store, Resume: StageShorten}},
		},
	}
	for _, tc := range rejected {
		t.Run("rejects/"+tc.name, func(t *testing.T) {
			w, err := New(tc.client, tc.opts)
			if !errors.Is(err, tc.want) {
				t.Errorf("New error = %v, want %v", err, tc.want)
			}
			if w != nil {
				t.Errorf("New returned a writer %v, want nil", w)
			}
		})
	}

	accepted := []struct {
		name string
		c    llm.LLMClient
		opts Options
	}{
		{"generation and both steps", client, withStepsEnabled(client)},
		{
			"after generation with both steps",
			client,
			Options{
				Shorten:   ShortenOptions{Enabled: true},
				Review:    ReviewOptions{Enabled: true, Client: client},
				Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration},
			},
		},
		{
			"after generation without shorten and no client",
			nil,
			Options{Artifacts: ArtifactOptions{Store: store, Resume: StageGeneration}},
		},
		{
			"after review with no clients",
			nil,
			Options{
				Shorten:   ShortenOptions{Enabled: true},
				Review:    ReviewOptions{Enabled: true},
				Artifacts: ArtifactOptions{Store: store, Resume: StageReview},
			},
		},
	}
	for _, tc := range accepted {
		t.Run("accepts/"+tc.name, func(t *testing.T) {
			w, err := New(tc.c, tc.opts)
			requireAccepted(t, w, err)
		})
	}
}

func TestResumeArticleProvenance(t *testing.T) {
	// A resumed run writes the reused generation and shorten provenance and the
	// executed review provenance into the Article, which --out reads.
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: reviewResponse(), Model: markReviewModel, ModelVersion: markStepModelVersion}}
	store := &fakeArtifactStore{loadResult: resumeChainShorten()}
	w := mustNew(t, nil, Options{
		Shorten:   ShortenOptions{Enabled: true},
		Review:    ReviewOptions{Enabled: true, Client: review},
		Artifacts: ArtifactOptions{Store: store, Resume: StageShorten},
	})

	a, err := w.Write(context.Background(), validTranscript())
	if err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if a.Model != reusedGenModel || a.ModelVersion != reusedGenVersion {
		t.Errorf("Article model = %q/%q, want the reused generation %q/%q", a.Model, a.ModelVersion, reusedGenModel, reusedGenVersion)
	}
	if a.Shorten == nil || a.Shorten.Model != reusedShortModel || a.Shorten.ModelVersion != reusedShortVersion {
		t.Errorf("Article.Shorten = %+v, want the reused shorten %q/%q", a.Shorten, reusedShortModel, reusedShortVersion)
	}
	if a.Review == nil || a.Review.Model.Model != markReviewModel || a.Review.Model.ModelVersion != markStepModelVersion {
		t.Errorf("Article.Review = %+v, want the executed review %q/%q", a.Review, markReviewModel, markStepModelVersion)
	}
}

func TestWriteMalformedOutputIsNotSaved(t *testing.T) {
	// A stage's output is saved only after it passes validation, so a malformed
	// output leaves no artifact for that stage.
	cases := []struct {
		name      string
		opts      func(*fakeArtifactStore) Options
		calls     []llmtestutil.FakeLLMSequenceStep
		wantSaved []Stage
	}{
		{
			name:      "generation",
			opts:      func(store *fakeArtifactStore) Options { return Options{Artifacts: ArtifactOptions{Store: store}} },
			calls:     []llmtestutil.FakeLLMSequenceStep{stepResult(llm.GenerateResponse{Text: "no heading", Model: markOutModel})},
			wantSaved: nil,
		},
		{
			name: "shorten",
			opts: func(store *fakeArtifactStore) Options {
				return Options{Shorten: ShortenOptions{Enabled: true}, Artifacts: ArtifactOptions{Store: store}}
			},
			calls: []llmtestutil.FakeLLMSequenceStep{
				stepResult(llm.GenerateResponse{Text: stepResponseText(markGenTitle, overLimitBody()), Model: markOutModel}),
				stepResult(llm.GenerateResponse{Text: "no heading", Model: markOutModel}),
			},
			wantSaved: []Stage{StageGeneration},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeArtifactStore{}
			gen := newStepClient(t, tc.calls...)
			w := mustNew(t, gen, tc.opts(store))

			a, err := w.Write(context.Background(), validTranscript())
			if !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("Write error = %v, want ErrMalformedOutput", err)
			}
			if a != (Article{}) {
				t.Errorf("Write article = %+v, want the zero value", a)
			}
			if got := store.saveStages(); !slices.Equal(got, tc.wantSaved) {
				t.Errorf("Save stages = %v, want %v (the malformed stage must not be saved)", got, tc.wantSaved)
			}
		})
	}
}
