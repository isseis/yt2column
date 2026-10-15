// Package writer defines the article-writing stage: the article type and the
// interface that produces it from a transcript.
package writer

import (
	"context"
	"fmt"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/prompts"
)

// Article is the generated column article. It stays comparable with ==:
// the step results are pointers, nil when the step called no LLM.
type Article struct {
	Title        string
	Body         string // Markdown
	SourceURL    string
	Model        string        // the generation model
	ModelVersion string        // opaque, provider-defined; empty when the provider reports none
	Shorten      *StepModel    // nil when the shorten step called no LLM
	Review       *ReviewResult // nil when the review step did not run
}

// StepModel is the model one step's LLM call reported.
type StepModel struct {
	Model        string
	ModelVersion string // empty when the provider reports none
}

// ArticleWriter generates a column article from a transcript.
// Implementations must return an error on failure and must not
// return an empty Article as a successful result.
type ArticleWriter interface {
	Write(ctx context.Context, t transcript.Transcript) (Article, error)
}

// Options configures the ArticleWriter returned by New. An empty template path
// selects the embedded default template; a non-empty path names an override
// file that New reads once. The zero value generates the article only:
// neither post-generation step runs and no artifact is saved.
type Options struct {
	SystemTemplatePath string
	UserTemplatePath   string
	Shorten            ShortenOptions
	Review             ReviewOptions
	Artifacts          ArtifactOptions
}

// ShortenOptions configures the shorten step, which uses the generation
// LLMClient passed to New. Template paths follow the rule of
// Options.SystemTemplatePath: empty selects the embedded default.
type ShortenOptions struct {
	Enabled            bool
	SystemTemplatePath string
	UserTemplatePath   string
}

// ReviewOptions configures the review step. Client is required when Enabled is
// true and must be nil when it is false.
type ReviewOptions struct {
	Enabled            bool
	Client             llm.LLMClient
	SystemTemplatePath string
	UserTemplatePath   string
}

// stepTemplates is the checked system and user template pair of one step. Only
// New builds one, and it expands the pair with the step's own data type.
type stepTemplates[T any] struct {
	system checkedTemplate[T]
	user   checkedTemplate[T]
}

// expand expands both templates of the pair with data.
func (p stepTemplates[T]) expand(data T) (system, user string, err error) {
	system, err = p.system.expand(data)
	if err != nil {
		return "", "", err
	}
	user, err = p.user.expand(data)
	return system, user, err
}

// templateWriter is the ArticleWriter returned by New. Its fields are set once
// by New and never mutated afterwards. A nil shorten or review means the step
// is disabled; resume is the stage whose saved output Write starts from, and
// store saves and loads the artifacts (nil when none is configured).
type templateWriter struct {
	client       llm.LLMClient
	generation   stepTemplates[templateData]
	shorten      *stepTemplates[shortenTemplateData]
	review       *stepTemplates[reviewTemplateData]
	reviewClient llm.LLMClient
	store        ArtifactStore
	resume       Stage
}

// generationRuns reports whether Write runs the generation stage: it runs only
// when the run does not resume.
func (w *templateWriter) generationRuns() bool { return w.resume == StageUnset }

// shortenRuns reports whether Write runs the shorten step: it is enabled and
// the resume stage is at or before it. w.shorten is non-nil exactly when the
// step is enabled.
func (w *templateWriter) shortenRuns() bool {
	return w.shorten != nil && (w.resume == StageUnset || w.resume == StageGeneration)
}

// reviewRuns reports whether Write runs the review step: it is enabled and the
// resume stage is not review. w.review is non-nil exactly when the step is
// enabled.
func (w *templateWriter) reviewRuns() bool {
	return w.review != nil && w.resume != StageReview
}

// New validates client, the options, and the templates of the enabled steps,
// and returns an ArticleWriter. It never reads environment variables. The
// concrete type is unexported, so an ArticleWriter can only be obtained
// through New.
func New(client llm.LLMClient, opts Options) (ArticleWriter, error) {
	if err := validateOptions(client, opts); err != nil {
		return nil, err
	}
	system, err := loadTemplate[templateData](templateSource{name: systemTemplateName, path: opts.SystemTemplatePath}, prompts.System())
	if err != nil {
		return nil, err
	}
	user, err := loadTemplate[templateData](templateSource{name: userTemplateName, path: opts.UserTemplatePath}, prompts.User())
	if err != nil {
		return nil, err
	}
	w := &templateWriter{
		client:     client,
		generation: stepTemplates[templateData]{system: system, user: user},
		store:      opts.Artifacts.Store,
		resume:     opts.Artifacts.Resume,
	}
	if opts.Shorten.Enabled {
		shorten, err := loadStepTemplates[shortenTemplateData](
			templateSpec{shortenSystemTemplateName, opts.Shorten.SystemTemplatePath, prompts.ShortenSystem()},
			templateSpec{shortenUserTemplateName, opts.Shorten.UserTemplatePath, prompts.ShortenUser()},
		)
		if err != nil {
			return nil, err
		}
		w.shorten = &shorten
	}
	if opts.Review.Enabled {
		review, err := loadStepTemplates[reviewTemplateData](
			templateSpec{reviewSystemTemplateName, opts.Review.SystemTemplatePath, prompts.ReviewSystem()},
			templateSpec{reviewUserTemplateName, opts.Review.UserTemplatePath, prompts.ReviewUser()},
		)
		if err != nil {
			return nil, err
		}
		w.review = &review
		w.reviewClient = opts.Review.Client
	}
	return w, nil
}

// validateOptions rejects the step combinations New must not accept, deriving
// which stages run from Artifacts.Resume and the enabled steps (the table of
// architecture §3.1). It rejects a resume with no store to load from, a client
// missing for a stage that runs, a client set for a stage that does not run,
// and a template path for a stage that does not run. Silently ignoring such a
// setting would hide the caller's mistake.
func validateOptions(client llm.LLMClient, opts Options) error {
	resume := opts.Artifacts.Resume
	if resume != StageUnset && nilcheck.IsNil(opts.Artifacts.Store) {
		return errResumeWithoutStore
	}
	generationRuns := resume == StageUnset
	shortenRuns := opts.Shorten.Enabled && (resume == StageUnset || resume == StageGeneration)
	reviewRuns := opts.Review.Enabled && resume != StageReview
	if err := validateClients(client, opts.Review.Client, generationRuns, shortenRuns, reviewRuns); err != nil {
		return err
	}
	return validateTemplatePaths(opts, generationRuns, shortenRuns, reviewRuns)
}

// validateClients rejects a client missing for a stage that runs, and a client
// set for a stage that does not.
func validateClients(generation, review llm.LLMClient, generationRuns, shortenRuns, reviewRuns bool) error {
	if generationRuns || shortenRuns {
		if nilcheck.IsNil(generation) {
			return errNilLLMClient
		}
	} else if !nilcheck.IsNil(generation) {
		return errGenerationClientSet
	}
	if reviewRuns {
		if nilcheck.IsNil(review) {
			return errReviewClientNil
		}
	} else if !nilcheck.IsNil(review) {
		return errReviewClientSet
	}
	return nil
}

// validateTemplatePaths rejects a template path for a stage that does not run.
func validateTemplatePaths(opts Options, generationRuns, shortenRuns, reviewRuns bool) error {
	if !generationRuns && (opts.SystemTemplatePath != "" || opts.UserTemplatePath != "") {
		return errDisabledStepTemplate
	}
	if !shortenRuns && (opts.Shorten.SystemTemplatePath != "" || opts.Shorten.UserTemplatePath != "") {
		return errDisabledStepTemplate
	}
	if !reviewRuns && (opts.Review.SystemTemplatePath != "" || opts.Review.UserTemplatePath != "") {
		return errDisabledStepTemplate
	}
	return nil
}

// templateSpec names one template source: its name, its override path (empty
// for the embedded default), and the embedded default itself.
type templateSpec struct {
	name     string
	path     string
	embedded string
}

// loadStepTemplates loads and checks the system and user template of one step.
func loadStepTemplates[T any](system, user templateSpec) (stepTemplates[T], error) {
	systemTmpl, err := loadTemplate[T](templateSource{name: system.name, path: system.path}, system.embedded)
	if err != nil {
		return stepTemplates[T]{}, err
	}
	userTmpl, err := loadTemplate[T](templateSource{name: user.name, path: user.path}, user.embedded)
	if err != nil {
		return stepTemplates[T]{}, err
	}
	return stepTemplates[T]{system: systemTmpl, user: userTmpl}, nil
}

// Write validates t, then runs the stages that Artifacts.Resume and the
// enabled steps select, in order (generation, shorten, review, the final
// length check), saving each completed stage's artifact when a store is
// configured, and builds the article from the executed and the reused stages.
// It returns the zero Article on every failure. An LLM client's error is
// wrapped as is, never with a sentinel of this package, so callers can tell it
// from a template or output failure; every failure of a step is a *StepError,
// and every store failure is a *ArtifactError.
func (w *templateWriter) Write(ctx context.Context, t transcript.Transcript) (Article, error) {
	if err := ctx.Err(); err != nil {
		return Article{}, fmt.Errorf("article generation not started: %w", err)
	}
	sourceURL, err := validateTranscript(t)
	if err != nil {
		return Article{}, err
	}
	materialID := MaterialID(t)
	data := newTemplateData(t)

	var chain ArtifactChain
	if w.resume != StageUnset {
		if err := ctx.Err(); err != nil {
			return Article{}, artifactError(w.resume, err)
		}
		loaded, err := w.store.Load(ctx, t.VideoID, w.resume, materialID)
		if err != nil {
			return Article{}, artifactError(w.resume, err)
		}
		chain = loaded
	}

	generation := chain.Generation
	if w.generationRuns() {
		generation, err = w.generateStage(ctx, t.VideoID, data, materialID)
		if err != nil {
			return Article{}, err
		}
	}
	title, body, previousID := generation.Title, generation.Body, generation.ID

	var shortenModel *StepModel
	switch {
	case w.shortenRuns() && bodyChars(body) > maxBodyChars:
		title, body, shortenModel, previousID, err = w.shortenStage(ctx, t.VideoID, title, body, previousID)
		if err != nil {
			return Article{}, err
		}
	case chain.Shorten != nil:
		shortenModel = &StepModel{Model: chain.Shorten.Model, ModelVersion: chain.Shorten.ModelVersion}
		previousID = chain.Shorten.ID
		if w.resume == StageShorten {
			title, body = chain.Shorten.Title, chain.Shorten.Body
		}
	}

	var reviewResult *ReviewResult
	switch {
	case w.reviewRuns():
		title, body, reviewResult, err = w.reviewStage(ctx, t.VideoID, data, title, body, previousID)
		if err != nil {
			return Article{}, err
		}
	case w.resume == StageReview:
		title, body = chain.Review.Title, chain.Review.Body
		reviewResult = &ReviewResult{
			Model:     StepModel{Model: chain.Review.Model, ModelVersion: chain.Review.ModelVersion},
			Revisions: chain.Review.Revisions,
		}
	}

	if w.shorten != nil {
		if chars := bodyChars(body); chars > maxBodyChars {
			return Article{}, stepError(StepReview, tooLongError(chars))
		}
	}
	return newArticle(title, body, sourceURL, generation.Model, generation.ModelVersion, shortenModel, reviewResult), nil
}

// generateStage runs the generation stage and saves its artifact when a store
// is configured, returning the record with its id set.
func (w *templateWriter) generateStage(ctx context.Context, videoID string, data templateData, materialID string) (StageRecord, error) {
	system, user, err := w.generation.expand(data)
	if err != nil {
		return StageRecord{}, err
	}
	resp, err := w.client.Generate(ctx, llm.GenerateRequest{SystemPrompt: system, UserPrompt: user, MaxOutputTokens: 0})
	if err != nil {
		return StageRecord{}, fmt.Errorf("LLM generation failed: %w", err)
	}
	title, body, err := checkResponse(resp)
	if err != nil {
		return StageRecord{}, err
	}
	record := StageRecord{
		Stage:        StageGeneration,
		Title:        title,
		Body:         body,
		Model:        resp.Model,
		ModelVersion: resp.ModelVersion,
		System:       w.generation.system.identity,
		User:         w.generation.user.identity,
		Input:        materialID,
	}
	if w.store == nil {
		return record, nil
	}
	id, err := w.saveStage(ctx, videoID, StageGeneration, record)
	if err != nil {
		return StageRecord{}, err
	}
	record.ID = id
	return record, nil
}

// shortenStage runs the shorten step and saves its artifact, returning the new
// text, the step model, and the id of the saved artifact (or its input id when
// no store is configured).
func (w *templateWriter) shortenStage(ctx context.Context, videoID, title, body, input string) (string, string, *StepModel, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", nil, "", stepError(StepShorten, err)
	}
	title, body, model, err := w.runShorten(ctx, title, body)
	if err != nil {
		return "", "", nil, "", err
	}
	if w.store == nil {
		return title, body, model, input, nil
	}
	id, err := w.saveStage(ctx, videoID, StageShorten, StageRecord{
		Stage:        StageShorten,
		Title:        title,
		Body:         body,
		Model:        model.Model,
		ModelVersion: model.ModelVersion,
		System:       w.shorten.system.identity,
		User:         w.shorten.user.identity,
		Input:        input,
	})
	if err != nil {
		return "", "", nil, "", err
	}
	return title, body, model, id, nil
}

// reviewStage runs the review step and saves its artifact.
func (w *templateWriter) reviewStage(ctx context.Context, videoID string, data templateData, title, body, input string) (string, string, *ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return "", "", nil, stepError(StepReview, err)
	}
	title, body, result, err := w.runReview(ctx, data, title, body)
	if err != nil {
		return "", "", nil, err
	}
	if w.store == nil {
		return title, body, result, nil
	}
	if _, err := w.saveStage(ctx, videoID, StageReview, StageRecord{
		Stage:        StageReview,
		Title:        title,
		Body:         body,
		Revisions:    result.Revisions,
		Model:        result.Model.Model,
		ModelVersion: result.Model.ModelVersion,
		System:       w.review.system.identity,
		User:         w.review.user.identity,
		Input:        input,
	}); err != nil {
		return "", "", nil, err
	}
	return title, body, result, nil
}

// saveStage checks the context, saves one completed stage's record, and
// returns its identifier. A store or context failure is a *ArtifactError
// naming the stage, so the caller can tell which stage failed.
func (w *templateWriter) saveStage(ctx context.Context, videoID string, stage Stage, record StageRecord) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", artifactError(stage, err)
	}
	id, err := w.store.Save(ctx, videoID, record)
	if err != nil {
		return "", artifactError(stage, err)
	}
	return id, nil
}
