//go:build js && wasm

package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claude"
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/pricing"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/strictjson"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// maxRequestBytes is the server's maxBodyBytes (internal/server/request.go),
// applied to the whole request string. The PoC members make the entry point
// slightly stricter than the server, never more lenient.
const maxRequestBytes = 4 << 20 // 4 MiB

// llmTimeout bounds one LLM call. It is provider.LLMTimeout
// (internal/llm/provider/provider.go), which cannot be imported here because
// internal/llm/provider does not build for WebAssembly.
const llmTimeout = 15 * time.Minute

// costDecimals is the number of digits after the decimal point of costUsd.
const costDecimals = 6

// allowedModels are the only models the PoC sends to. claude.New accepts any
// non-empty model name, and the adapter quotes the model in an HTTP status
// error, so a key pasted into a model field would be sent and then shown.
// Rejecting every other value before the send keeps that from happening.
var allowedModels = map[string]struct{}{
	"claude-haiku-5-5": {},
	"claude-opus-5-5":  {},
}

// apiCall is the validated common part of generate and longOutput.
type apiCall struct {
	apiKey              secret.Secret
	workspaceID         claudeparam.WorkspaceID
	browserAccessHeader bool
}

// modelSetting is one model and effort pair as received, before claude.New
// checks it.
type modelSetting struct {
	model  string
	effort string
}

// generateInput is a request that passed every check of generate.
type generateInput struct {
	source        *transcript.CopiedSource
	normalizedURL string
	generation    llm.LLMClient
	review        llm.LLMClient
	browserAccess bool
	// requested names the model each stage was asked to use, which a failed
	// stage reports when the response named none.
	requested requestedModels
}

// requestedModels is the validated model of the generation client (also used
// by the shorten step) and of the review client.
type requestedModels struct {
	generation string
	review     string
}

// forStage returns the model stage was configured with.
func (r requestedModels) forStage(stage writer.Stage) string {
	switch stage {
	case writer.StageGeneration, writer.StageShorten:
		return r.generation
	case writer.StageReview:
		return r.review
	default:
		return ""
	}
}

// generateResult is the GenerateResult of the page interface. It never
// carries an article together with a failure.
type generateResult struct {
	OK       bool           `json:"ok"`
	Markdown string         `json:"markdown,omitempty"`
	Failure  *failureReport `json:"failure,omitempty"`
	Stages   []stageReport  `json:"stages"`
}

// runGenerate validates the request, runs the three stages with the CLI's
// conditions, and returns the display Markdown or the failure. No LLM is
// called unless every check passed.
func runGenerate(transport *browserTransport, request string) generateResult {
	input, failure := validateGenerate(request)
	if failure != nil {
		return generateResult{OK: false, Failure: failure, Stages: notRunStages()}
	}
	transport.setCall(input.browserAccess)
	defer transport.setCall(false)

	// The same conditions as the CLI: the embedded templates, the shorten
	// step with the generation client, and the review step with its own
	// client.
	articleWriter, err := writer.NewProgressWriter(input.generation, writer.Options{
		Shorten: writer.ShortenOptions{Enabled: true},
		Review:  writer.ReviewOptions{Enabled: true, Client: input.review},
	})
	if err != nil {
		return generateResult{OK: false, Failure: classify(err), Stages: notRunStages()}
	}
	ctx := context.Background()
	t, err := input.source.Fetch(ctx, input.normalizedURL)
	if err != nil {
		return generateResult{OK: false, Failure: classify(err), Stages: notRunStages()}
	}
	observer := newStageTimer()
	article, err := articleWriter.WriteWithProgress(ctx, t, observer)
	returned := time.Now()
	if err != nil {
		var runUsage writer.RunUsage
		if usageErr, ok := errors.AsType[*writer.TokenUsageError](err); ok {
			runUsage = usageErr.RunUsage
		}
		return generateResult{OK: false, Failure: classify(err), Stages: observer.reports(runUsage, input.requested, returned)}
	}
	stages := observer.reports(article.RunUsage, input.requested, returned)
	markdown, err := publisher.DisplayMarkdown(article)
	if err != nil {
		return generateResult{OK: false, Failure: &failureReport{Kind: kindUnpublishable, Message: err.Error()}, Stages: stages}
	}
	return generateResult{OK: true, Markdown: markdown, Stages: stages}
}

// validateGenerate runs the checks of the server's validateBody in the same
// steps and order (internal/server/request.go), then the PoC members, then
// the configuration. Every rejection's kind is chosen by the step that
// rejected.
//
//nolint:gocyclo // one flat sequence, kept so it reads side by side with validateBody
func validateGenerate(request string) (*generateInput, *failureReport) {
	// Step 1: the size limit, on the whole request.
	if len(request) > maxRequestBytes {
		return nil, newFailure(kindInvalidInput, "the request exceeds the size limit")
	}
	// Step 2: strictjson's whole-body checks and the declared members.
	object, err := strictjson.ParseObject([]byte(request))
	if err != nil {
		return nil, newFailure(kindInvalidInput, "the request is not one valid JSON object")
	}
	members, err := object.Collect("videoUrl", "transcript", "title", "channelName", "description",
		"apiKey", "workspaceId", "browserAccessHeader", "generation", "review")
	if err != nil {
		return nil, newFailure(kindInvalidInput, "a member appears more than once")
	}
	// Step 3: the required strings, whitespace-only rejected.
	rawVideoURL, err := strictjson.RequiredString(members, "videoUrl")
	if err != nil || strings.TrimSpace(rawVideoURL) == "" {
		return nil, newFailure(kindInvalidInput, "videoUrl is missing, empty, or not a string")
	}
	rawTranscript, err := strictjson.RequiredString(members, "transcript")
	if err != nil || strings.TrimSpace(rawTranscript) == "" {
		return nil, newFailure(kindInvalidInput, "transcript is missing, empty, or not a string")
	}
	// Step 4: the optional strings and the channel name rules.
	title, ok := optionalMember(members, "title")
	if !ok {
		return nil, newFailure(kindInvalidInput, "title is not a string")
	}
	channelName, ok := optionalMember(members, "channelName")
	if !ok {
		return nil, newFailure(kindInvalidInput, "channelName is not a string")
	}
	if writer.CheckChannelName(channelName) != nil {
		return nil, newFailure(kindInvalidInput, "channelName breaks the channel name rules")
	}
	description, ok := optionalMember(members, "description")
	if !ok {
		return nil, newFailure(kindInvalidInput, "description is not a string")
	}
	// The PoC members' shapes, still before the video URL and transcript
	// checks, as the server reads its mode member last among the members.
	rawCall, failure := readAPICall(members)
	if failure != nil {
		return nil, failure
	}
	generation, failure := readModelSetting(members, "generation")
	if failure != nil {
		return nil, failure
	}
	review, failure := readModelSetting(members, "review")
	if failure != nil {
		return nil, failure
	}
	// Step 5: the video URL.
	videoID, normalizedURL, err := transcript.ValidateVideoURL(rawVideoURL)
	if err != nil {
		return nil, newFailure(kindInvalidVideoURL, "the video URL was not accepted")
	}
	// Step 6: the copied transcript's format.
	source, err := transcript.NewCopiedSource(transcript.CopiedInput{
		VideoID:     videoID,
		Raw:         rawTranscript,
		Title:       title,
		ChannelName: channelName,
		Description: description,
	})
	if err != nil {
		if errors.Is(err, transcript.ErrCopiedTranscriptFormat) {
			return nil, newFailure(kindTranscriptFormat, "the copied transcript's format was not accepted")
		}
		return nil, classify(err)
	}
	// The configuration, checked by secret.New and the adapter's own types.
	call, ok := parseAPICall(rawCall)
	if !ok {
		return nil, newFailure(kindInvalidConfig, invalidConfigMessage)
	}
	generationClient, ok := newClient(call, generation)
	if !ok {
		return nil, newFailure(kindInvalidConfig, invalidConfigMessage)
	}
	reviewClient, ok := newClient(call, review)
	if !ok {
		return nil, newFailure(kindInvalidConfig, invalidConfigMessage)
	}
	return &generateInput{
		source:        source,
		normalizedURL: normalizedURL,
		generation:    generationClient,
		review:        reviewClient,
		browserAccess: call.browserAccessHeader,
		requested:     requestedModels{generation: generation.model, review: review.model},
	}, nil
}

// optionalMember is the server's optionalMember: an absent member is "", and
// a present one must be a string. ok is false on a rejection.
func optionalMember(members map[string]strictjson.Value, key string) (string, bool) {
	value, present := members[key]
	if !present {
		return "", true
	}
	text, err := value.AsString()
	if err != nil {
		return "", false
	}
	return text, true
}

// rawAPICall is apiKey, workspaceId, and browserAccessHeader with their
// shapes checked. The strings are parsed by parseAPICall, after the video URL
// and transcript checks, as the configuration step.
type rawAPICall struct {
	apiKey              string
	workspaceID         string
	browserAccessHeader bool
}

// readAPICall reads apiKey, workspaceId, and browserAccessHeader. The two
// strings must be present and non-empty strings, and the flag a boolean.
func readAPICall(members map[string]strictjson.Value) (rawAPICall, *failureReport) {
	rawKey, err := strictjson.RequiredString(members, "apiKey")
	if err != nil {
		return rawAPICall{}, newFailure(kindInvalidInput, "apiKey is missing, empty, or not a string")
	}
	rawWorkspace, err := strictjson.RequiredString(members, "workspaceId")
	if err != nil {
		return rawAPICall{}, newFailure(kindInvalidInput, "workspaceId is missing, empty, or not a string")
	}
	headerValue, err := strictjson.Required(members, "browserAccessHeader")
	if err != nil {
		return rawAPICall{}, newFailure(kindInvalidInput, "browserAccessHeader is missing")
	}
	browserAccess, err := headerValue.AsBool()
	if err != nil {
		return rawAPICall{}, newFailure(kindInvalidInput, "browserAccessHeader is not a boolean")
	}
	return rawAPICall{apiKey: rawKey, workspaceID: rawWorkspace, browserAccessHeader: browserAccess}, nil
}

// parseAPICall checks the key and the workspace ID with secret.New and
// ParseWorkspaceID. Both values are secrets, so a rejection reports only
// that it failed, and the caller returns the fixed invalid_config message.
func parseAPICall(raw rawAPICall) (apiCall, bool) {
	key, err := secret.New(raw.apiKey)
	if err != nil {
		return apiCall{}, false
	}
	workspaceID, err := claudeparam.ParseWorkspaceID(raw.workspaceID)
	if err != nil {
		return apiCall{}, false
	}
	return apiCall{apiKey: key, workspaceID: workspaceID, browserAccessHeader: raw.browserAccessHeader}, true
}

// readModelSetting reads one nested {model, effort} object. Unknown and
// duplicate members inside it are rejected.
func readModelSetting(members map[string]strictjson.Value, key string) (modelSetting, *failureReport) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return modelSetting{}, newFailure(kindInvalidInput, key+" is missing")
	}
	object, err := value.AsObject()
	if err != nil {
		return modelSetting{}, newFailure(kindInvalidInput, key+" is not an object")
	}
	inner, err := object.CollectOnly("model", "effort")
	if err != nil {
		return modelSetting{}, newFailure(kindInvalidInput, key+" has an unknown or duplicate member")
	}
	model, err := strictjson.RequiredString(inner, "model")
	if err != nil {
		return modelSetting{}, newFailure(kindInvalidInput, key+".model is missing, empty, or not a string")
	}
	effort, err := strictjson.RequiredString(inner, "effort")
	if err != nil {
		return modelSetting{}, newFailure(kindInvalidInput, key+".effort is missing, empty, or not a string")
	}
	return modelSetting{model: model, effort: effort}, nil
}

// newClient builds a Claude adapter with the CLI's timeout and the adapter's
// default max_tokens, for a model in allowedModels only. Its errors can quote
// the received values, so only whether it succeeded is returned.
func newClient(call apiCall, setting modelSetting) (llm.LLMClient, bool) {
	if _, ok := allowedModels[setting.model]; !ok {
		return nil, false
	}
	effort, err := claudeparam.ParseEffort(setting.effort)
	if err != nil {
		return nil, false
	}
	client, err := claude.New(claude.Options{
		APIKey:      call.apiKey,
		Model:       setting.model,
		Effort:      effort,
		WorkspaceID: call.workspaceID,
		Timeout:     llmTimeout,
	})
	if err != nil {
		return nil, false
	}
	return client, true
}

// stageReport is the StageReport of the page interface.
type stageReport struct {
	Stage     string  `json:"stage"`
	Status    string  `json:"status"`
	ElapsedMs *int64  `json:"elapsedMs"`
	Model     *string `json:"model"` // the model the response reported
	// RequestedModel is the model the stage was configured with; null only
	// when validation rejected the request before any stage.
	RequestedModel *string `json:"requestedModel"`
	InputTokens    *uint64 `json:"inputTokens"`
	OutputTokens   *uint64 `json:"outputTokens"`
	CostUSD        *string `json:"costUsd"`
}

// reportedStages are the three stages in the order Write runs them.
var reportedStages = []writer.Stage{writer.StageGeneration, writer.StageShorten, writer.StageReview}

// notRunStages is the stage list of a call that ran no stage.
func notRunStages() []stageReport {
	reports := make([]stageReport, 0, len(reportedStages))
	for _, stage := range reportedStages {
		reports = append(reports, stageReport{Stage: stage.String(), Status: "not_run"})
	}
	return reports
}

// stageTimer is the ProgressObserver of one Write. It records when each
// stage started and finished, and which were skipped.
type stageTimer struct {
	mu       sync.Mutex
	started  map[writer.Stage]time.Time
	finished map[writer.Stage]time.Time
	skipped  map[writer.Stage]struct{}
}

func newStageTimer() *stageTimer {
	return &stageTimer{
		started:  map[writer.Stage]time.Time{},
		finished: map[writer.Stage]time.Time{},
		skipped:  map[writer.Stage]struct{}{},
	}
}

func (s *stageTimer) StageStarted(stage writer.Stage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started[stage] = time.Now()
}

func (s *stageTimer) StageFinished(stage writer.Stage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished[stage] = time.Now()
}

func (s *stageTimer) StageSkipped(stage writer.Stage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipped[stage] = struct{}{}
}

// reports builds each stage's report. A stage that started and never
// finished failed, and its end is the time Write returned.
func (s *stageTimer) reports(runUsage writer.RunUsage, requested requestedModels, returned time.Time) []stageReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	reports := make([]stageReport, 0, len(reportedStages))
	for _, stage := range reportedStages {
		requestedModel := requested.forStage(stage)
		report := stageReport{Stage: stage.String(), Status: "not_run", RequestedModel: &requestedModel}
		start, started := s.started[stage]
		end, finished := s.finished[stage]
		_, skipped := s.skipped[stage]
		switch {
		case finished:
			report.Status = "done"
		case skipped:
			report.Status = "skipped"
		case started:
			report.Status = "failed"
			end = returned
		}
		if started {
			elapsed := end.Sub(start).Milliseconds()
			report.ElapsedMs = &elapsed
		}
		fillUsage(&report, stageUsage(runUsage, stage))
		reports = append(reports, report)
	}
	return reports
}

// stageUsage returns the usage RunUsage holds for stage.
func stageUsage(runUsage writer.RunUsage, stage writer.Stage) writer.StageUsage {
	switch stage {
	case writer.StageGeneration:
		return runUsage.Generation
	case writer.StageShorten:
		return runUsage.Shorten
	case writer.StageReview:
		return runUsage.Review
	default:
		return writer.StageUsage{}
	}
}

// fillUsage sets the model, the token counts, and the cost estimate the
// stage's usage makes known. A stage that called no LLM leaves them null.
func fillUsage(report *stageReport, usage writer.StageUsage) {
	if usage.Origin == writer.OriginNone {
		return
	}
	if usage.Model != "" {
		model := usage.Model
		report.Model = &model
	}
	report.InputTokens, report.OutputTokens = tokenCounts(usage.Usage)
	if amount, _, known := pricing.Estimate(usage.Model, usage.Usage, usage.CallTime).Known(); known {
		cost := amount.Format(costDecimals)
		report.CostUSD = &cost
	}
}

// tokenCounts returns the input tokens (every input category summed) and the
// output tokens, or nil for both when the usage is unknown.
func tokenCounts(usage llm.Usage) (input, output *uint64) {
	counts, known := usage.Counts()
	if !known {
		return nil, nil
	}
	in := counts.Input + counts.CacheRead + counts.CacheWrite
	out := counts.Output
	return &in, &out
}
