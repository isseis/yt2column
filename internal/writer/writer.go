// Package writer defines the article-writing stage: the article type and the
// interface that produces it from a transcript.
package writer

import (
	"context"
	"fmt"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/nilcheck"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/prompts"
)

// Article is the generated column article. It stays comparable with ==:
// the step results are pointers, nil when the step called no LLM.
type Article struct {
	Title        string
	ChannelName  string // the video's channel; empty when there is none
	Body         string // Markdown
	SourceURL    string
	Model        string        // the generation model
	ModelVersion string        // opaque, provider-defined; empty when the provider reports none
	Shorten      *StepModel    // nil when the shorten step called no LLM
	Review       *ReviewResult // nil when the review step did not run
	RunUsage     RunUsage      // each stage's token usage
}

// UsageOrigin is where a stage's token usage came from. The zero value
// OriginNone means the stage called no LLM in this run and reused no artifact,
// so the stage has no usage to report.
type UsageOrigin int

const (
	// OriginNone is the zero value: the stage made no LLM call and reused
	// nothing.
	OriginNone UsageOrigin = iota
	// OriginRun means this run called the stage's LLM client.
	OriginRun
	// OriginReused means the run reused the stage's artifact, and the usage is
	// the one that artifact recorded.
	OriginReused
)

// StageUsage is one stage's token usage, the model its call reported, the
// time its Generate call returned, and where each came from. Model is
// empty when Origin is OriginNone, or when a failed call reported no model
// that passed the model-name rules. CallTime is in UTC, truncated to the
// second, with no monotonic clock reading; it is the zero time when Origin is
// OriginNone.
type StageUsage struct {
	Origin   UsageOrigin
	Model    string
	Usage    llm.Usage
	CallTime time.Time
}

// RunUsage is the token usage of each stage of one Write.
type RunUsage struct {
	Generation StageUsage
	Shorten    StageUsage
	Review     StageUsage
}

// StepModel is the model one step's LLM call reported.
type StepModel struct {
	Model        string
	ModelVersion string // empty when the provider reports none
}

// ArticleWriter generates a column article from a transcript.
// Implementations must return an error on failure and must not return an empty
// Article as a successful result. Write wraps every failure in a
// *TokenUsageError that carries the usage of the stages that ran, or were
// reused, before the failure.
type ArticleWriter interface {
	Write(ctx context.Context, t transcript.Transcript) (Article, error)
}

// ProgressObserver receives the progress of one Write. The calls for one
// Write are made from the goroutine that runs it, in order. A stage that
// failed gets no StageFinished: the receiver tells the failed stage as the one
// that was started and never finished.
type ProgressObserver interface {
	StageStarted(s Stage)  // before the stage's first action
	StageFinished(s Stage) // after the stage's result passed every check of that stage
	StageSkipped(s Stage)  // the stage will not run in this Write
}

// ProgressWriter is an ArticleWriter that also reports its progress.
// WriteWithProgress behaves exactly as Write, and in addition calls observer,
// which must not be nil.
type ProgressWriter interface {
	ArticleWriter
	WriteWithProgress(ctx context.Context, t transcript.Transcript, observer ProgressObserver) (Article, error)
}

// nopObserver is the ProgressObserver Write passes: it ignores every call.
type nopObserver struct{}

func (nopObserver) StageStarted(Stage)  {}
func (nopObserver) StageFinished(Stage) {}
func (nopObserver) StageSkipped(Stage)  {}

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
	// Clock returns the current time. nil selects time.Now. It is read once
	// after each stage's Generate call returns, and the value is recorded on
	// the stage's usage and artifact.
	Clock func() time.Time
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
	clock        func() time.Time
}

// generationRuns reports whether Write runs the generation stage.
func (w *templateWriter) generationRuns() bool {
	generation, _, _ := stageRuns(w.resume, w.shorten != nil, w.review != nil)
	return generation
}

// shortenRuns reports whether Write runs the shorten step.
func (w *templateWriter) shortenRuns() bool {
	_, shorten, _ := stageRuns(w.resume, w.shorten != nil, w.review != nil)
	return shorten
}

// reviewRuns reports whether Write runs the review step.
func (w *templateWriter) reviewRuns() bool {
	_, _, review := stageRuns(w.resume, w.shorten != nil, w.review != nil)
	return review
}

// stageRuns derives which stages a run executes from the resume stage and the
// enabled steps (the table of architecture section 3.1). New and Write read
// this one definition, so the validation and the execution cannot drift.
func stageRuns(resume Stage, shortenEnabled, reviewEnabled bool) (generation, shorten, review bool) {
	generation = resume == StageUnset
	shorten = shortenEnabled && (resume == StageUnset || resume == StageGeneration)
	review = reviewEnabled && resume != StageReview
	return generation, shorten, review
}

// New validates client, the options, and the templates of the enabled steps,
// and returns an ArticleWriter. It never reads environment variables. The
// concrete type is unexported, so an ArticleWriter can only be obtained
// through New or NewProgressWriter.
func New(client llm.LLMClient, opts Options) (ArticleWriter, error) {
	w, err := newTemplateWriter(client, opts)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// NewProgressWriter validates the same Options as New and returns the same
// writer as a ProgressWriter.
func NewProgressWriter(client llm.LLMClient, opts Options) (ProgressWriter, error) {
	w, err := newTemplateWriter(client, opts)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// newTemplateWriter is the validation and construction New and
// NewProgressWriter share.
func newTemplateWriter(client llm.LLMClient, opts Options) (*templateWriter, error) {
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
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	w := &templateWriter{
		client:     client,
		generation: stepTemplates[templateData]{system: system, user: user},
		resume:     opts.Artifacts.Resume,
		clock:      clock,
	}
	// Normalize a typed-nil store to a nil interface, so Write's "no store"
	// test is the same as the caller's "no store".
	if !nilcheck.IsNil(opts.Artifacts.Store) {
		w.store = opts.Artifacts.Store
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
// architecture section 3.1). It rejects an unknown resume, a resume with no
// store to load from, a client missing for a stage that runs, a client set for
// a stage that does not run, and a template path for a stage that does not run.
// Silently ignoring such a setting would hide the caller's mistake.
func validateOptions(client llm.LLMClient, opts Options) error {
	resume := opts.Artifacts.Resume
	if !isKnownResume(resume) {
		return errUnknownResume
	}
	if resume != StageUnset && nilcheck.IsNil(opts.Artifacts.Store) {
		return errResumeWithoutStore
	}
	generationRuns, shortenRuns, reviewRuns := stageRuns(resume, opts.Shorten.Enabled, opts.Review.Enabled)
	if err := validateClients(client, opts.Review.Client, generationRuns, shortenRuns, reviewRuns); err != nil {
		return err
	}
	return validateTemplatePaths(opts, generationRuns, shortenRuns, reviewRuns)
}

// isKnownResume reports whether stage is a value Artifacts.Resume may name.
func isKnownResume(stage Stage) bool {
	switch stage {
	case StageUnset, StageGeneration, StageShorten, StageReview:
		return true
	default:
		return false
	}
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
// every store failure is a *ArtifactError, and every failure carries the
// stages' usage in a *TokenUsageError.
func (w *templateWriter) Write(ctx context.Context, t transcript.Transcript) (Article, error) {
	return w.WriteWithProgress(ctx, t, nopObserver{})
}

// WriteWithProgress is Write, reporting each stage to observer:
//   - a stage that runs gets StageStarted before its first action and
//     StageFinished after its result passed every check of that stage and
//     its artifact was saved (when a store is configured); the review step's
//     checks include the final length check, so a body the review made too
//     long is a failure of the review step;
//   - a stage reused from Artifacts.Resume gets StageFinished when it is
//     reused;
//   - a stage that neither runs nor is reused gets StageSkipped, right after
//     the previous stage's last call;
//   - a failure adds no call, and a failure before the generation stage
//     (the context, the transcript, the resume load) makes none. When the
//     review step did not run (disabled or reused), a failure of the final
//     length check leaves no stage unfinished; the server always runs the
//     review step and never resumes, so it does not meet this case.
//
// A nil observer is rejected before anything runs.
func (w *templateWriter) WriteWithProgress(ctx context.Context, t transcript.Transcript, observer ProgressObserver) (Article, error) {
	if nilcheck.IsNil(observer) {
		return Article{}, tokenUsageError(RunUsage{}, errNilProgressObserver)
	}
	if err := ctx.Err(); err != nil {
		return Article{}, tokenUsageError(RunUsage{}, fmt.Errorf("article generation not started: %w", err))
	}
	sourceURL, err := validateTranscript(t)
	if err != nil {
		return Article{}, tokenUsageError(RunUsage{}, err)
	}
	materialID := MaterialID(t)
	data := newTemplateData(t)

	var runUsage RunUsage
	chain, err := w.loadResume(ctx, t.VideoID, materialID)
	if err != nil {
		return Article{}, tokenUsageError(runUsage, err)
	}

	generation := chain.Generation
	if w.generationRuns() {
		observer.StageStarted(StageGeneration)
		var usage StageUsage
		generation, usage, err = w.generateStage(ctx, t.VideoID, data, materialID)
		runUsage.Generation = usage
		if err != nil {
			return Article{}, tokenUsageError(runUsage, err)
		}
	} else {
		runUsage.Generation = reusedUsage(chain.Generation)
	}
	observer.StageFinished(StageGeneration)
	title, body, previousID := generation.Title, generation.Body, generation.ID

	var shortenModel *StepModel
	switch {
	case w.shortenRuns() && bodyChars(body) > maxBodyChars:
		observer.StageStarted(StageShorten)
		var usage StageUsage
		title, body, shortenModel, previousID, usage, err = w.shortenStage(ctx, t.VideoID, title, body, previousID)
		runUsage.Shorten = usage
		if err != nil {
			return Article{}, tokenUsageError(runUsage, err)
		}
		observer.StageFinished(StageShorten)
	case chain.Shorten != nil:
		observer.StageFinished(StageShorten)
		shortenModel = &StepModel{Model: chain.Shorten.Model, ModelVersion: chain.Shorten.ModelVersion}
		previousID = chain.Shorten.ID
		runUsage.Shorten = reusedUsage(*chain.Shorten)
		if w.resume == StageShorten {
			title, body = chain.Shorten.Title, chain.Shorten.Body
		}
	default:
		observer.StageSkipped(StageShorten)
	}

	var reviewResult *ReviewResult
	reviewRan := false
	switch {
	case w.reviewRuns():
		observer.StageStarted(StageReview)
		reviewRan = true
		var usage StageUsage
		title, body, reviewResult, usage, err = w.reviewStage(ctx, t.VideoID, data, title, body, previousID)
		runUsage.Review = usage
		if err != nil {
			return Article{}, tokenUsageError(runUsage, err)
		}
	case w.resume == StageReview:
		observer.StageFinished(StageReview)
		title, body = chain.Review.Title, chain.Review.Body
		reviewResult = &ReviewResult{
			Model:     StepModel{Model: chain.Review.Model, ModelVersion: chain.Review.ModelVersion},
			Revisions: chain.Review.Revisions,
		}
		runUsage.Review = reusedUsage(*chain.Review)
	default:
		observer.StageSkipped(StageReview)
	}

	if w.shorten != nil {
		if chars := bodyChars(body); chars > maxBodyChars {
			return Article{}, tokenUsageError(runUsage, stepError(StepReview, tooLongError(chars)))
		}
	}
	if reviewRan {
		observer.StageFinished(StageReview)
	}
	return newArticle(title, t.ChannelName, body, sourceURL, generation.Model, generation.ModelVersion, shortenModel, reviewResult, runUsage), nil
}

// loadResume loads the artifact chain Write resumes from and checks its call
// times. It returns the zero chain when Write does not resume.
func (w *templateWriter) loadResume(ctx context.Context, videoID, materialID string) (ArtifactChain, error) {
	if w.resume == StageUnset {
		return ArtifactChain{}, nil
	}
	if err := ctx.Err(); err != nil {
		return ArtifactChain{}, artifactError(w.resume, err)
	}
	chain, err := w.store.Load(ctx, videoID, w.resume, materialID)
	if err != nil {
		return ArtifactChain{}, artifactError(w.resume, err)
	}
	if err := checkChainCallTimes(chain); err != nil {
		return ArtifactChain{}, err
	}
	return chain, nil
}

// stageUsage is the usage of a stage that called Generate: OriginRun, the
// response's Usage, and resp.Model only when it passes the model-name rule (so
// a failed call whose model was invalid reports an empty model). callTime is
// the time the call returned, normalized to UTC, truncated to the second, and
// stripped of any monotonic clock reading.
func stageUsage(resp llm.GenerateResponse, callTime time.Time) StageUsage {
	model := ""
	if checkModelString("Model", resp.Model) == nil {
		model = resp.Model
	}
	return StageUsage{Origin: OriginRun, Model: model, Usage: resp.Usage, CallTime: normalizeCallTime(callTime)}
}

// reusedUsage is the usage of a stage that reused a loaded artifact: the
// record's already-verified model, usage, and call time.
func reusedUsage(record StageRecord) StageUsage {
	return StageUsage{Origin: OriginReused, Model: record.Model, Usage: record.Usage, CallTime: record.CallTime}
}

// normalizeCallTime returns t in the one form a StageUsage or a StageRecord
// holds: UTC, whole seconds, no monotonic clock reading.
func normalizeCallTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Second)
}

// checkChainCallTimes rejects a loaded chain whose records are missing their
// call time. Write records a call time on every stage it runs and Save refuses
// a record without one, so a store that returns such a record breaks the
// ArtifactStore contract; failing closed keeps a time-of-day stage from being
// priced from a zero time.
func checkChainCallTimes(chain ArtifactChain) error {
	records := []StageRecord{chain.Generation}
	if chain.Shorten != nil {
		records = append(records, *chain.Shorten)
	}
	if chain.Review != nil {
		records = append(records, *chain.Review)
	}
	for _, record := range records {
		if record.CallTime.IsZero() {
			return artifactError(record.Stage, fmt.Errorf("%w: the %s artifact has no call time", ErrMissingCallTime, record.Stage))
		}
	}
	return nil
}

// generateStage runs the generation stage and saves its artifact when a store
// is configured, returning the record with its id set and the stage's usage. A
// failure after Generate returns the stage's usage with the error.
func (w *templateWriter) generateStage(ctx context.Context, videoID string, data templateData, materialID string) (StageRecord, StageUsage, error) {
	system, user, err := w.generation.expand(data)
	if err != nil {
		return StageRecord{}, StageUsage{}, err
	}
	resp, err := w.client.Generate(ctx, llm.GenerateRequest{SystemPrompt: system, UserPrompt: user, MaxOutputTokens: 0})
	usage := stageUsage(resp, w.clock())
	if err != nil {
		return StageRecord{}, usage, fmt.Errorf("LLM generation failed: %w", err)
	}
	title, body, err := checkResponse(resp)
	if err != nil {
		return StageRecord{}, usage, err
	}
	record := StageRecord{
		Stage:        StageGeneration,
		Title:        title,
		Body:         body,
		Model:        resp.Model,
		ModelVersion: resp.ModelVersion,
		Usage:        resp.Usage,
		CallTime:     usage.CallTime,
		System:       w.generation.system.identity,
		User:         w.generation.user.identity,
		Input:        materialID,
	}
	if w.store == nil {
		return record, usage, nil
	}
	id, err := w.saveStage(ctx, videoID, StageGeneration, record)
	if err != nil {
		return StageRecord{}, usage, err
	}
	record.ID = id
	return record, usage, nil
}

// shortenStage runs the shorten step and saves its artifact, returning the new
// text, the step model, the id of the saved artifact (or its input id when no
// store is configured), and the stage's usage. A failure after Generate
// returns the stage's usage with the error.
func (w *templateWriter) shortenStage(ctx context.Context, videoID, title, body, input string) (string, string, *StepModel, string, StageUsage, error) {
	if err := ctx.Err(); err != nil {
		return "", "", nil, "", StageUsage{}, stepError(StepShorten, err)
	}
	title, body, model, usage, err := w.runShorten(ctx, title, body)
	if err != nil {
		return "", "", nil, "", usage, err
	}
	if w.store == nil {
		return title, body, model, input, usage, nil
	}
	id, err := w.saveStage(ctx, videoID, StageShorten, StageRecord{
		Stage:        StageShorten,
		Title:        title,
		Body:         body,
		Model:        model.Model,
		ModelVersion: model.ModelVersion,
		Usage:        usage.Usage,
		CallTime:     usage.CallTime,
		System:       w.shorten.system.identity,
		User:         w.shorten.user.identity,
		Input:        input,
	})
	if err != nil {
		return "", "", nil, "", usage, err
	}
	return title, body, model, id, usage, nil
}

// reviewStage runs the review step and saves its artifact, returning the stage's
// usage. A failure after Generate returns the stage's usage with the error.
func (w *templateWriter) reviewStage(ctx context.Context, videoID string, data templateData, title, body, input string) (string, string, *ReviewResult, StageUsage, error) {
	if err := ctx.Err(); err != nil {
		return "", "", nil, StageUsage{}, stepError(StepReview, err)
	}
	title, body, result, usage, err := w.runReview(ctx, data, title, body)
	if err != nil {
		return "", "", nil, usage, err
	}
	if w.store == nil {
		return title, body, result, usage, nil
	}
	if _, err := w.saveStage(ctx, videoID, StageReview, StageRecord{
		Stage:        StageReview,
		Title:        title,
		Body:         body,
		Revisions:    result.Revisions,
		Model:        result.Model.Model,
		ModelVersion: result.Model.ModelVersion,
		Usage:        usage.Usage,
		CallTime:     usage.CallTime,
		System:       w.review.system.identity,
		User:         w.review.user.identity,
		Input:        input,
	}); err != nil {
		return "", "", nil, usage, err
	}
	return title, body, result, usage, nil
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
