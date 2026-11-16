package extjob

import (
	"context"
	"encoding/json"
	"errors"
	"runtime/debug"
	"sync"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/llm/llmhttp"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/strictjson"
)

// keyCheckTimeout bounds the one Models API call of a settings check. A call
// that passes it is reported as unverified, not as a rejection.
const keyCheckTimeout = 30 * time.Second

// Backend is what a module's main gives the job: how a run gets its LLM
// clients and how the settings check asks the API about a key. extjob checks
// the settings' shape itself before calling either, so a Backend only ever
// receives checked settings.
type Backend interface {
	// NewClients builds the generation and review clients of one run.
	NewClients(cfg config.LLMConfig) (Deps, error)
	// CheckKey returns nil when the API accepts the key, an error matching
	// llm.ErrAuthRejected when it rejects it, and another error otherwise.
	CheckKey(ctx context.Context, cfg config.LLMConfig) error
	// Info declares the implementation for buildInfo.
	Info() BackendInfo
}

// FakeScenario is the five values a development build embedded, as parsed,
// in the spellings devfake.ParseScenario accepts.
type FakeScenario struct {
	Article       string `json:"article"`
	Failure       string `json:"failure"`
	FailAt        string `json:"failAt"`
	DelaySeconds  int    `json:"delaySeconds"`
	SettingsCheck string `json:"settingsCheck"`
}

// BackendInfo is the llm member of buildInfo: claude, or fake with its
// scenario. Its fields are unexported, so the only values are the zero value
// and the two constructors' results; the zero value names no implementation.
type BackendInfo struct {
	kind     string
	scenario FakeScenario
}

// The implementation kinds buildInfo reports.
const (
	kindClaude = "claude"
	kindFake   = "fake"
)

// ClaudeInfo is the BackendInfo of the release backend.
func ClaudeInfo() BackendInfo { return BackendInfo{kind: kindClaude} }

// FakeInfo is the BackendInfo of a development backend with scenario s.
func FakeInfo(s FakeScenario) BackendInfo { return BackendInfo{kind: kindFake, scenario: s} }

// llmInfoJSON is the llm member of buildInfo.
type llmInfoJSON struct {
	Kind     string        `json:"kind"`
	Scenario *FakeScenario `json:"scenario,omitempty"`
}

// buildInfoJSON is the JSON buildInfo returns. llm is absent when the Backend
// declared nothing, so a missing declaration is never read as an
// implementation.
type buildInfoJSON struct {
	GoVersion   string       `json:"goVersion"`
	VCSRevision string       `json:"vcsRevision"`
	VCSModified bool         `json:"vcsModified"`
	MainPath    string       `json:"mainPath"`
	LLM         *llmInfoJSON `json:"llm,omitempty"`
}

// buildInfo reads the build's identity from the Go build information. A build
// without it reports empty values.
func buildInfo(info BackendInfo) buildInfoJSON {
	out := buildInfoJSON{}
	if bi, ok := debug.ReadBuildInfo(); ok {
		out.GoVersion = bi.GoVersion
		out.MainPath = bi.Path
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				out.VCSRevision = s.Value
			case "vcs.modified":
				out.VCSModified = s.Value == "true"
			}
		}
	}
	switch info.kind {
	case kindClaude:
		out.LLM = &llmInfoJSON{Kind: kindClaude}
	case kindFake:
		scenario := info.scenario
		out.LLM = &llmInfoJSON{Kind: kindFake, Scenario: &scenario}
	default:
		// The zero BackendInfo names no implementation.
	}
	return out
}

// Module holds one Worker's state: the Backend and the input Prepare kept. Its
// methods are the four JavaScript functions' JSON entry points. It is safe for
// concurrent use. Only PrepareJSON bounds its request (4 MiB); the other
// requests are small and come from the same page. The kept input stays until
// the next accepted prepare, so a retry with the same digest can run again.
type Module struct {
	backend Backend

	mu   sync.Mutex
	kept *Prepared // the input the last accepted prepare kept; nil before it
}

// NewModule returns a Module that gets its LLM implementation from b.
func NewModule(b Backend) *Module { return &Module{backend: b} }

// The member names of the JSON requests, in one place.
const (
	memberAPIKey      = "anthropicApiKey" //nolint:gosec // a JSON member name, not a credential
	memberWorkspaceID = "anthropicWorkspaceId"
)

// loadSettings checks the stored settings as the page read them and returns
// the LLM configuration they make. ok is false for any value that is not
// accepted: the API key must be a string of printable ASCII (the rule of
// llmhttp.CheckAPIKey), and the workspace ID a string that is empty (no
// workspace) or accepted by claudeparam.ParseWorkspaceID. Nothing is trimmed
// or repaired. Contract: the key's text reaches config.LoadExtension, which
// makes it a secret.Secret, and nothing else holds it; the fields reports
// which of the two members was rejected.
func loadSettings(apiKey, workspaceID string) (cfg config.LLMConfig, rejected []string) {
	if key, err := secret.New(apiKey); err != nil || llmhttp.CheckAPIKey(key) != nil {
		rejected = append(rejected, memberAPIKey)
	}
	if workspaceID != "" {
		if _, err := claudeparam.ParseWorkspaceID(workspaceID); err != nil {
			rejected = append(rejected, memberWorkspaceID)
		}
	}
	if len(rejected) != 0 {
		return config.LLMConfig{}, rejected
	}
	cfg, err := config.LoadExtension(config.ExtensionLookup(apiKey, workspaceID))
	if err != nil {
		// The two checks above accept what the loader accepts, so a loader
		// rejection means they disagree: neither value is trusted.
		return config.LLMConfig{}, []string{memberAPIKey, memberWorkspaceID}
	}
	return cfg, nil
}

// settingsCheckResult is the JSON of a settings check.
type settingsCheckResult struct {
	Status string   `json:"status"`
	Fields []string `json:"fields,omitempty"`
}

func marshalFixed(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		// The values marshaled here are strings and slices of strings.
		return `{"status":"rejected"}`
	}
	return string(data)
}

// CheckSettingsJSON checks the settings the options page is about to save:
// their shape, then, only when the shape is accepted, the API key with one
// Models API call. The request is {anthropicApiKey, anthropicWorkspaceId},
// both strings; a request of another shape is rejected with both fields.
func (m *Module) CheckSettingsJSON(ctx context.Context, request string) (out string) {
	defer func() {
		if recover() != nil {
			// A panic of the Backend must not end the Go program, which
			// would fail every later call; the check could not be made.
			out = marshalFixed(settingsCheckResult{Status: "unverified"})
		}
	}()
	both := []string{memberAPIKey, memberWorkspaceID}
	object, err := strictjson.ParseObject([]byte(request))
	if err != nil {
		return marshalFixed(settingsCheckResult{Status: statusRejected, Fields: both})
	}
	members, err := object.CollectOnly(memberAPIKey, memberWorkspaceID)
	if err != nil {
		return marshalFixed(settingsCheckResult{Status: statusRejected, Fields: both})
	}
	apiKey, keyErr := strictjson.RequiredStringAllowEmpty(members, memberAPIKey)
	workspaceID, workspaceErr := strictjson.RequiredStringAllowEmpty(members, memberWorkspaceID)
	if keyErr != nil || workspaceErr != nil {
		var fields []string
		if keyErr != nil {
			fields = append(fields, memberAPIKey)
		}
		if workspaceErr != nil {
			fields = append(fields, memberWorkspaceID)
		}
		return marshalFixed(settingsCheckResult{Status: statusRejected, Fields: fields})
	}
	cfg, rejected := loadSettings(apiKey, workspaceID)
	if len(rejected) != 0 {
		return marshalFixed(settingsCheckResult{Status: statusRejected, Fields: rejected})
	}
	ctx, cancel := context.WithTimeout(ctx, keyCheckTimeout)
	defer cancel()
	switch err := m.backend.CheckKey(ctx, cfg); {
	case err == nil:
		return marshalFixed(settingsCheckResult{Status: "ok"})
	case errors.Is(err, llm.ErrAuthRejected):
		return marshalFixed(settingsCheckResult{Status: "key_rejected"})
	default:
		return marshalFixed(settingsCheckResult{Status: "unverified"})
	}
}

// prepareResult is the JSON of prepare.
type prepareResult struct {
	Status      string `json:"status"`
	InputDigest string `json:"inputDigest,omitempty"`
	VideoID     string `json:"videoId,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// prepareInvalidBody is the one reason prepare rejects an input for: a request
// of another shape, an oversized request, and an input Prepare does not
// accept are all invalid_body.
const prepareInvalidBody = "invalid_body"

func rejectedPrepare() string {
	return marshalFixed(prepareResult{Status: statusRejected, Reason: prepareInvalidBody})
}

// optionalString returns the string value of an optional member, or "" when it
// is absent. A value that is present but null or not a string is rejected; an
// empty string is accepted.
func optionalString(members map[string]strictjson.Value, key string) (string, bool) {
	value, ok := members[key]
	if !ok {
		return "", true
	}
	text, err := value.AsString()
	return text, err == nil
}

// PrepareJSON validates a start input, keeps it in the module's memory, and
// returns the values the reservation needs. The request is {videoUrl,
// transcript, title?, channelName?, description?}, a closed set of members of
// at most 4 MiB. A rejection keeps nothing and leaves the input an earlier
// prepare kept as it was.
func (m *Module) PrepareJSON(request string) string {
	if len(request) > maxRequestBytes {
		return rejectedPrepare()
	}
	object, err := strictjson.ParseObject([]byte(request))
	if err != nil {
		return rejectedPrepare()
	}
	members, err := object.CollectOnly("videoUrl", "transcript", "title", "channelName", "description")
	if err != nil {
		return rejectedPrepare()
	}
	videoURL, err := strictjson.RequiredString(members, "videoUrl")
	if err != nil {
		return rejectedPrepare()
	}
	transcriptText, err := strictjson.RequiredString(members, "transcript")
	if err != nil {
		return rejectedPrepare()
	}
	in := StartInput{VideoURL: videoURL, Transcript: transcriptText}
	var ok bool
	if in.Title, ok = optionalString(members, "title"); !ok {
		return rejectedPrepare()
	}
	if in.ChannelName, ok = optionalString(members, "channelName"); !ok {
		return rejectedPrepare()
	}
	if in.Description, ok = optionalString(members, "description"); !ok {
		return rejectedPrepare()
	}
	prepared, err := Prepare(in)
	if err != nil {
		return rejectedPrepare()
	}
	m.mu.Lock()
	m.kept = &prepared
	m.mu.Unlock()
	return marshalFixed(prepareResult{Status: "ok", InputDigest: prepared.InputDigest(), VideoID: prepared.VideoID()})
}

// progressEvent is the JSON the progress callback receives.
type progressEvent struct {
	Steps stepsJSON `json:"steps"`
}

// callbackObserver passes each step change to the page as a ProgressEvent.
type callbackObserver struct{ fn func(event string) }

func (o callbackObserver) StepChanged(steps Steps) {
	if o.fn == nil {
		return
	}
	o.fn(marshalFixed(progressEvent{Steps: steps.json()}))
}

// generateRequest is a parsed generate request. The settings are the two
// members as the page read them, unchecked.
type generateRequest struct {
	inputDigest string
	apiKey      strictjson.Value
	workspaceID strictjson.Value
	hasWorkspID bool
}

// parseGenerateRequest reads {inputDigest, settings: {anthropicApiKey,
// anthropicWorkspaceId?}}. Both objects are closed sets: an unknown or
// repeated member, a missing inputDigest, settings, or anthropicApiKey, and a
// non-string inputDigest are rejected. The two settings values keep their JSON
// kind, which loadStored checks.
func parseGenerateRequest(request string) (generateRequest, bool) {
	object, err := strictjson.ParseObject([]byte(request))
	if err != nil {
		return generateRequest{}, false
	}
	members, err := object.CollectOnly("inputDigest", "settings")
	if err != nil {
		return generateRequest{}, false
	}
	digest, err := strictjson.RequiredStringAllowEmpty(members, "inputDigest")
	if err != nil {
		return generateRequest{}, false
	}
	settingsValue, err := strictjson.Required(members, "settings")
	if err != nil {
		return generateRequest{}, false
	}
	settingsObject, err := settingsValue.AsObject()
	if err != nil {
		return generateRequest{}, false
	}
	settings, err := settingsObject.CollectOnly(memberAPIKey, memberWorkspaceID)
	if err != nil {
		return generateRequest{}, false
	}
	apiKey, err := strictjson.Required(settings, memberAPIKey)
	if err != nil {
		return generateRequest{}, false
	}
	workspaceID, hasWorkspaceID := settings[memberWorkspaceID]
	return generateRequest{inputDigest: digest, apiKey: apiKey, workspaceID: workspaceID, hasWorkspID: hasWorkspaceID}, true
}

// loadStored checks the stored settings and returns the configuration they
// make. ok is false when either value is not a string, when the workspace ID
// is absent (the options page always writes it, so an absent item is not a
// choice of no workspace), or when loadSettings rejects a value.
func (r generateRequest) loadStored() (config.LLMConfig, bool) {
	if !r.hasWorkspID {
		return config.LLMConfig{}, false
	}
	apiKey, err := r.apiKey.AsString()
	if err != nil {
		return config.LLMConfig{}, false
	}
	workspaceID, err := r.workspaceID.AsString()
	if err != nil {
		return config.LLMConfig{}, false
	}
	cfg, rejected := loadSettings(apiKey, workspaceID)
	return cfg, len(rejected) == 0
}

// failedGenerate is the JSON of a failure no step had started for.
func failedGenerate(failure Failure, detail InternalDetail) string {
	return marshalFixed(resultJSON{Status: statusFailed, Steps: Steps{}.json(), Failure: failure.String(), InternalDetail: detail})
}

// GenerateJSON runs one generation of the input the last accepted prepare
// kept, for the settings the request carries, and returns the result. The
// request is {inputDigest, settings: {anthropicApiKey, anthropicWorkspaceId?}}.
// onProgress receives a progress event at each step change and may be nil.
//
// The checks run in this order, each before anything that could call the LLM
// API: the request's shape (internal, request_shape), the digest against the
// kept input (internal, input_mismatch), and the settings (settings_invalid,
// every step pending). The result and the progress events never carry the API
// key.
func (m *Module) GenerateJSON(ctx context.Context, request string, onProgress func(event string)) (out string) {
	defer func() {
		if recover() != nil {
			// Run recovers its own panics; this covers the Backend's
			// NewClients and the marshaling.
			out = failedGenerate(FailureInternal, DetailPanic)
		}
	}()
	req, ok := parseGenerateRequest(request)
	if !ok {
		return failedGenerate(FailureInternal, DetailRequestShape)
	}
	m.mu.Lock()
	kept := m.kept
	m.mu.Unlock()
	if kept == nil || kept.InputDigest() != req.inputDigest {
		return failedGenerate(FailureInternal, DetailInputMismatch)
	}
	cfg, ok := req.loadStored()
	if !ok {
		return failedGenerate(FailureSettingsInvalid, DetailNone)
	}
	deps, err := m.backend.NewClients(cfg)
	if err != nil {
		return failedGenerate(FailureInternal, DetailNone)
	}
	result := Run(ctx, *kept, deps, callbackObserver{fn: onProgress})
	data, err := marshalResult(result)
	if err != nil {
		return failedGenerate(FailureInternal, DetailNone)
	}
	return string(data)
}

// BuildInfoJSON returns the build's identity and the Backend's declaration for
// the options page's debug line.
func (m *Module) BuildInfoJSON() string {
	return marshalFixed(buildInfo(m.backend.Info()))
}
