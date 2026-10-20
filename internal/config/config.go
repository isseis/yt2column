// Package config loads and validates the process configuration from
// environment variables. It is the only package that names the secret
// variables; every other package receives already-validated values.
package config

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/slackwebhook"
)

// Provider is the LLM provider. Its zero value is ProviderUnset.
type Provider int

// The supported Provider values. ProviderUnset is the zero value, which no
// factory accepts. Load never stores it in a Config it returns; LoadFor stores
// it only for a run that reads no generation provider (Need{}, or a review-only
// run with its own YT2COLUMN_REVIEW_PROVIDER), where no factory is built.
const (
	ProviderUnset Provider = iota
	ProviderDeepSeek
	ProviderClaude
)

// Names of the environment variables Load reads.
const (
	providerEnv             = "YT2COLUMN_LLM_PROVIDER"
	modelEnv                = "YT2COLUMN_MODEL"
	reviewProviderEnv       = "YT2COLUMN_REVIEW_PROVIDER"
	reviewModelEnv          = "YT2COLUMN_REVIEW_MODEL"
	deepSeekAPIKeyEnv       = "DEEPSEEK_API_KEY"  //nolint:gosec // the variable's name, not a credential value
	anthropicAPIKeyEnv      = "ANTHROPIC_API_KEY" //nolint:gosec // the variable's name, not a credential value
	claudeEffortEnv         = "YT2COLUMN_CLAUDE_EFFORT"
	anthropicWorkspaceIDEnv = "ANTHROPIC_WORKSPACE_ID"
	slackEnv                = "SLACK_WEBHOOK_URL"
	cacheDirEnv             = "YT2COLUMN_CACHE_DIR"
	ytDlpEnv                = "YT2COLUMN_YTDLP_PATH"
	godebugEnv              = "GODEBUG"
)

// The accepted values of providerEnv.
const (
	providerDeepSeek = "deepseek"
	providerClaude   = "claude"
)

// The GODEBUG values that turn on the Go HTTP/2 transport's verbose log, which
// writes every request header, the API key headers included, to standard error.
const (
	http2DebugInfo    = "1"
	http2DebugVerbose = "2"
)

// Fixed reasons for a rejected variable. A reason is chosen from these
// constants and built from nothing else, so a rejected value cannot reach an
// error message.
const (
	reasonUnset                = "is unset or empty"
	reasonProvider             = `must be exactly "deepseek" or "claude"`
	reasonClaudeEffort         = "must be a supported effort value"
	reasonAnthropicWorkspaceID = "must be a non-empty string of printable ASCII characters other than space"
	reasonSlackWebhook         = "must be an https URL with a host"
	reasonDirRelative          = "must be an absolute path"
	reasonCacheDirAbsent       = "no absolute default could be derived from HOME or XDG_CACHE_HOME"
	reasonSocketPathAbsent     = "no absolute socket path could be derived from HOME"
	reasonSocketPathTooLong    = "the socket path derived from HOME exceeds the operating system's limit"
	reasonHomeAbsent           = "must be set to an absolute path"
)

// LookupFunc has the signature of os.LookupEnv. Load reads every variable
// through one, so a test can supply an environment without changing the
// process environment.
type LookupFunc func(name string) (value string, ok bool)

// ErrMissing reports a variable that is unset, or present with an empty value,
// and has no usable default.
var ErrMissing = errors.New("environment variable is unset or empty")

// ErrInvalid reports a variable whose value is present but not acceptable.
var ErrInvalid = errors.New("environment variable has an invalid value")

// VarError names a rejected variable and gives a fixed reason. It never holds
// the variable's value, so an error can be logged safely even when the rejected
// variable was a secret set in the wrong place.
type VarError struct {
	Name   string
	Reason string
	Err    error // ErrMissing or ErrInvalid
}

// Error implements the error interface as "<Name>: <Reason>".
func (e *VarError) Error() string {
	return e.Name + ": " + e.Reason
}

// Unwrap returns the sentinel that classifies the rejection.
func (e *VarError) Unwrap() error {
	return e.Err
}

// LLMConfig is the validated configuration of the generation and review LLM
// calls. Its fields are unexported, so a non-zero LLMConfig can only come from
// Load, LoadFor, or LoadServer.
type LLMConfig struct {
	provider             Provider
	model                string
	reviewProvider       Provider
	reviewModel          string
	deepSeekAPIKey       secret.Secret
	anthropicAPIKey      secret.Secret
	claudeEffort         claudeparam.Effort
	anthropicWorkspaceID claudeparam.WorkspaceID
}

// Provider returns the LLM provider of the generation.
func (c LLMConfig) Provider() Provider {
	return c.provider
}

// Model returns the model name of the generation. It is non-empty when the
// generation or the shorten step runs (Need.Generation), or when a review-only
// run inherits YT2COLUMN_MODEL; otherwise it is empty and no generation client
// is built.
func (c LLMConfig) Model() string {
	return c.model
}

// ReviewProvider returns the provider of the review step. It equals
// Provider() when YT2COLUMN_REVIEW_PROVIDER is unset.
func (c LLMConfig) ReviewProvider() Provider {
	return c.reviewProvider
}

// ReviewModel returns the model name of the review step. It is non-empty.
// It equals Model() when both YT2COLUMN_REVIEW_PROVIDER and
// YT2COLUMN_REVIEW_MODEL are unset.
func (c LLMConfig) ReviewModel() string {
	return c.reviewModel
}

// DeepSeekAPIKey returns the API key. It is non-zero when the generation or the
// review provider is ProviderDeepSeek.
func (c LLMConfig) DeepSeekAPIKey() secret.Secret {
	return c.deepSeekAPIKey
}

// AnthropicAPIKey returns the Anthropic API key. It is non-zero when the
// generation or the review provider is ProviderClaude.
func (c LLMConfig) AnthropicAPIKey() secret.Secret {
	return c.anthropicAPIKey
}

// ClaudeEffort returns the effort. It is not claudeparam.EffortUnset when
// the generation or the review provider is ProviderClaude.
func (c LLMConfig) ClaudeEffort() claudeparam.Effort {
	return c.claudeEffort
}

// AnthropicWorkspaceID returns the workspace ID. It is the zero value when
// ANTHROPIC_WORKSPACE_ID is unset or neither the generation nor the review
// provider is ProviderClaude.
func (c LLMConfig) AnthropicWorkspaceID() claudeparam.WorkspaceID {
	return c.anthropicWorkspaceID
}

// Config is the CLI configuration. Its fields are unexported, so a non-zero
// Config can only come from Load or LoadFor.
type Config struct {
	llm               LLMConfig
	slackWebhookURL   secret.Secret
	cacheDir          string
	ytDlpPath         string
	http2DebugEnabled bool
}

// LLM returns the validated LLM configuration.
func (c Config) LLM() LLMConfig {
	return c.llm
}

// SlackWebhookURL returns the Webhook URL and whether one is configured.
func (c Config) SlackWebhookURL() (secret.Secret, bool) {
	if _, err := c.slackWebhookURL.Reveal(); err != nil {
		return secret.Secret{}, false
	}
	return c.slackWebhookURL, true
}

// RequireSlackWebhookURL returns the Webhook URL, or a *VarError naming
// SLACK_WEBHOOK_URL and wrapping ErrMissing when none is configured. The error
// never holds a value.
func (c Config) RequireSlackWebhookURL() (secret.Secret, error) {
	if _, err := c.slackWebhookURL.Reveal(); err != nil {
		return secret.Secret{}, missingVar(slackEnv)
	}
	return c.slackWebhookURL, nil
}

// CacheDir returns the absolute cache directory.
func (c Config) CacheDir() string {
	return c.cacheDir
}

// YtDlpPath returns the yt-dlp path. Empty means "yt-dlp" on PATH.
func (c Config) YtDlpPath() string {
	return c.ytDlpPath
}

// HTTP2DebugEnabled reports whether GODEBUG contains http2debug=1 or
// http2debug=2. Load never rejects this setting; the caller warns instead.
func (c Config) HTTP2DebugEnabled() bool {
	return c.http2DebugEnabled
}

// ServerConfig is the server configuration. Its fields are unexported, so a
// non-zero ServerConfig can only come from LoadServer. LoadServer never reads
// or checks YT2COLUMN_CACHE_DIR, YT2COLUMN_YTDLP_PATH, or the cache directory
// defaults.
type ServerConfig struct {
	llm               LLMConfig
	webhookURL        secret.Secret // always set
	socketPath        string        // absolute, within the OS limit
	http2DebugEnabled bool
}

// LLM returns the validated LLM configuration.
func (c ServerConfig) LLM() LLMConfig {
	return c.llm
}

// WebhookURL returns the Webhook URL. It is always set.
func (c ServerConfig) WebhookURL() secret.Secret {
	return c.webhookURL
}

// SocketPath returns the absolute path of the server's Unix domain socket,
// derived from HOME alone. It is within the OS limit.
func (c ServerConfig) SocketPath() string {
	return c.socketPath
}

// HTTP2DebugEnabled reports whether GODEBUG contains http2debug=1 or
// http2debug=2. LoadServer never rejects this setting; the caller warns instead.
func (c ServerConfig) HTTP2DebugEnabled() bool {
	return c.http2DebugEnabled
}

// LoadServer reads and validates the server's variables through lookup: the
// LLM settings (shared with Load), SLACK_WEBHOOK_URL (required), and HOME, from
// which the socket path is derived. It joins every rejection together and
// returns the zero ServerConfig on any error, so a value that did not pass
// validation never reaches a caller. It reads no variable the server does not
// use.
func LoadServer(lookup LookupFunc) (ServerConfig, error) {
	llm, errs := loadLLM(lookup, Need{Generation: true, Review: true})
	var cfg ServerConfig
	webhook, set, webhookErr := loadSlackWebhook(lookup)
	switch {
	case webhookErr != nil:
		errs = append(errs, webhookErr)
	case !set:
		errs = append(errs, missingVar(slackEnv))
	}
	sockPath, sockErr := socketPath(runtime.GOOS, lookup)
	if sockErr != nil {
		errs = append(errs, sockErr)
	}
	if err := errors.Join(errs...); err != nil {
		return ServerConfig{}, err
	}
	cfg.llm = llm
	cfg.webhookURL = webhook
	cfg.socketPath = sockPath
	cfg.http2DebugEnabled = hasHTTP2Debug(lookup)
	return cfg, nil
}

// HostConfig is the Native Messaging host's relay configuration. Its field is
// unexported, so a non-zero HostConfig can only come from LoadHost.
type HostConfig struct {
	socketPath string // absolute, within the OS limit
}

// SocketPath returns the absolute path of the server's Unix domain socket,
// derived from HOME alone with the same rule as ServerConfig.SocketPath.
func (c HostConfig) SocketPath() string {
	return c.socketPath
}

// LoadHost reads HOME, and nothing else, through lookup. Contract: it derives
// the socket path with the same function as LoadServer, so a host the browser
// starts and a server started from a shell reach one socket whenever they see
// the same HOME. It returns a *VarError naming HOME, and the zero HostConfig,
// when HOME is unusable or the derived path exceeds the OS limit.
func LoadHost(lookup LookupFunc) (HostConfig, error) {
	sockPath, err := socketPath(runtime.GOOS, lookup)
	if err != nil {
		return HostConfig{}, err
	}
	return HostConfig{socketPath: sockPath}, nil
}

// LoadHostHome reads HOME, and nothing else, through lookup, and returns it
// when it is set and absolute. install and uninstall derive the browsers'
// manifest directories from it. Contract: it never checks the socket path, so
// a user whose socket path is too long can still remove the manifest. It
// returns a *VarError naming HOME when HOME is unset, empty, or relative.
func LoadHostHome(lookup LookupFunc) (string, error) {
	home, ok := absoluteHome(lookup)
	if !ok {
		return "", missingVarWithReason("HOME", reasonHomeAbsent)
	}
	return home, nil
}

// Load reads and validates every variable through lookup. It joins every
// rejection together so callers can report all of them; on any error it returns
// the zero Config, because a value that did not pass validation must not reach
// a caller. It is LoadFor with both needs set, so a run that does not resume
// keeps the behavior of task 0010.
func Load(lookup LookupFunc) (Config, error) {
	return LoadFor(lookup, Need{Generation: true, Review: true})
}

// Need says which LLM configurations a run uses, so LoadFor reads and
// validates only those. A stage that does not run does not contribute its
// provider's variables. Generation also covers the shorten step, which uses
// the generation client.
type Need struct {
	Generation bool // the generation client is built (generation and/or shorten runs)
	Review     bool // the review client is built (the review step runs)
}

// LoadFor reads and validates the variables the run needs, and rejects no
// other provider variable. It joins every rejection together and returns the
// zero Config on any error. A need that is true reads and requires its stage's
// provider and model variables and the selected providers' own variables; a
// need that is false does not check them. When Review is true but Generation is
// false, the generation provider and model are still read, without being
// required, so the review step inherits them when its own variables are unset.
func LoadFor(lookup LookupFunc, need Need) (Config, error) {
	llm, errs := loadLLM(lookup, need)
	var cfg Config
	webhook, _, webhookErr := loadSlackWebhook(lookup)
	errs = append(errs,
		webhookErr,
		loadCacheDir(lookup, &cfg),
		loadYtDlpPath(lookup, &cfg),
	)
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	cfg.llm = llm
	cfg.slackWebhookURL = webhook
	cfg.http2DebugEnabled = hasHTTP2Debug(lookup)
	return cfg, nil
}

// loadLLM reads and validates the LLM variables the run needs: the generation
// and review providers and models, and the variables of the selected
// providers. It returns the validated LLMConfig and each rejection separately,
// so LoadFor and LoadServer report them the same way. A need that is false
// leaves its stage's settings unchecked.
func loadLLM(lookup LookupFunc, need Need) (LLMConfig, []error) {
	var cfg LLMConfig
	var errs []error
	if need.Generation {
		errs = append(errs, loadProvider(lookup, &cfg), loadModel(lookup, &cfg))
	}
	if need.Review {
		errs = append(errs, loadReview(lookup, &cfg, need.Generation)...)
	}
	errs = append(errs, loadProviderVars(lookup, &cfg, need)...)
	return cfg, errs
}

// loadProvider validates YT2COLUMN_LLM_PROVIDER with parseProvider. Unset
// defaults to deepseek.
func loadProvider(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(providerEnv)
	provider, err := parseProvider(providerEnv, value, ok)
	if err != nil {
		return err
	}
	cfg.provider = provider
	return nil
}

// loadProviderOptional reads YT2COLUMN_LLM_PROVIDER without requiring it, for a
// run whose generation does not execute: unset falls back to the default
// deepseek provider, while a present value must be accepted (an empty one is
// rejected, matching loadProvider, so a broken export is not silently
// defaulted).
func loadProviderOptional(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(providerEnv)
	if !ok {
		cfg.provider = ProviderDeepSeek
		return nil
	}
	provider, err := parseProvider(providerEnv, value, true)
	if err != nil {
		return err
	}
	cfg.provider = provider
	return nil
}

// loadModelOptional reads YT2COLUMN_MODEL without requiring it, for a run whose
// generation does not execute: unset leaves the model empty for the review
// step's own resolution, while a present value (an empty one included) follows
// loadModel's rule.
func loadModelOptional(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(modelEnv)
	switch {
	case !ok:
		return nil
	case value == "":
		return missingVar(modelEnv)
	default:
		cfg.model = value
		return nil
	}
}

// parseProvider converts a provider variable's value into a Provider. present
// reports whether the variable is set: an unset value defaults to deepseek,
// which is the rule for YT2COLUMN_LLM_PROVIDER alone, so a caller with its own
// default (YT2COLUMN_REVIEW_PROVIDER inherits the generation provider) handles
// the unset case itself. A present empty value is rejected. name is the
// variable being read, so a rejection names it. The accepted values are shared
// by every provider variable.
func parseProvider(name, value string, present bool) (Provider, error) {
	switch {
	case !present:
		return ProviderDeepSeek, nil
	case value == "":
		return ProviderUnset, missingVar(name)
	case value == providerDeepSeek:
		return ProviderDeepSeek, nil
	case value == providerClaude:
		return ProviderClaude, nil
	default:
		return ProviderUnset, invalidVar(name, reasonProvider)
	}
}

// loadReview reads YT2COLUMN_REVIEW_PROVIDER and YT2COLUMN_REVIEW_MODEL, which
// select the review step's provider and model. Both unset means the review step
// reuses the generation provider and model. A set provider requires a set
// model; only the model may be set on its own, to reuse the generation
// provider. generationRuns reports whether the generation model is required
// (and so already reported missing by loadModel); when it is false and the
// review model still resolves to empty, the review step has no model, so it is
// reported here. It returns each rejection as a separate element so LoadFor
// joins them flat.
func loadReview(lookup LookupFunc, cfg *LLMConfig, generationRuns bool) []error {
	var errs []error
	providerValue, providerPresent := lookup(reviewProviderEnv)
	modelValue, modelPresent := lookup(reviewModelEnv)

	providerOK := true
	switch {
	case providerPresent:
		provider, err := parseProvider(reviewProviderEnv, providerValue, true)
		if err != nil {
			errs = append(errs, err)
			providerOK = false
		} else {
			cfg.reviewProvider = provider
		}
	case generationRuns:
		cfg.reviewProvider = cfg.provider
	default:
		// The review step inherits the generation provider, which the run did
		// not read because generation does not run. Read it here, without
		// requiring it: an unset value falls back to the default deepseek.
		if err := loadProviderOptional(lookup, cfg); err != nil {
			errs = append(errs, err)
		} else {
			cfg.reviewProvider = cfg.provider
		}
	}

	switch {
	case modelPresent && modelValue == "":
		errs = append(errs, missingVar(reviewModelEnv))
	case modelPresent:
		cfg.reviewModel = modelValue
	case providerPresent:
		if providerOK {
			// A review provider with no model of its own is incomplete.
			errs = append(errs, missingVar(reviewModelEnv))
		}
	default:
		// The review step inherits the generation model. When generation does
		// not run, read that model here, without requiring it, and report a
		// still-empty inherited model because the review step needs one.
		if !generationRuns {
			if err := loadModelOptional(lookup, cfg); err != nil {
				errs = append(errs, err)
			}
		}
		cfg.reviewModel = cfg.model
		if !generationRuns && cfg.reviewModel == "" {
			errs = append(errs, missingVar(reviewModelEnv))
		}
	}
	return errs
}

// loadProviderVars returns the rejections of the selected providers' variables
// and reads no others. A provider's variables are read only when a stage the
// run needs (need) uses that provider, so a key shared by the generation and
// review stages is read once and a stage that does not run contributes
// nothing. It returns nothing when no needed stage uses either provider, so an
// invalid generation provider is reported once by loadProvider and no provider
// variable rejects a value the user did not intend to use. Each rejection is a
// separate element so LoadFor joins them flat; a nested join would reach a
// caller that prints one error per line as a single line.
func loadProviderVars(lookup LookupFunc, cfg *LLMConfig, need Need) []error {
	var errs []error
	deepSeek := (need.Generation && cfg.provider == ProviderDeepSeek) ||
		(need.Review && cfg.reviewProvider == ProviderDeepSeek)
	if deepSeek {
		errs = append(errs, loadDeepSeekAPIKey(lookup, cfg))
	}
	claude := (need.Generation && cfg.provider == ProviderClaude) ||
		(need.Review && cfg.reviewProvider == ProviderClaude)
	if claude {
		errs = append(errs,
			loadAnthropicAPIKey(lookup, cfg),
			loadClaudeEffort(lookup, cfg),
			loadAnthropicWorkspaceID(lookup, cfg),
		)
	}
	return errs
}

// loadModel validates YT2COLUMN_MODEL, which has no default.
func loadModel(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(modelEnv)
	if !ok || value == "" {
		return missingVar(modelEnv)
	}
	cfg.model = value
	return nil
}

// loadDeepSeekAPIKey validates DEEPSEEK_API_KEY, which is required when the
// generation or the review provider is deepseek. It is called only when one of
// them is, so the variable is never read, and never rejected, otherwise.
func loadDeepSeekAPIKey(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(deepSeekAPIKeyEnv)
	if !ok || value == "" {
		return missingVar(deepSeekAPIKeyEnv)
	}
	key, err := secret.New(value)
	if err != nil {
		return missingVar(deepSeekAPIKeyEnv)
	}
	cfg.deepSeekAPIKey = key
	return nil
}

// loadAnthropicAPIKey validates ANTHROPIC_API_KEY, which is required when the
// generation or the review provider is claude. It is called only when one of
// them is, so the variable is never read, and never rejected, otherwise.
func loadAnthropicAPIKey(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(anthropicAPIKeyEnv)
	if !ok || value == "" {
		return missingVar(anthropicAPIKeyEnv)
	}
	key, err := secret.New(value)
	if err != nil {
		return missingVar(anthropicAPIKeyEnv)
	}
	cfg.anthropicAPIKey = key
	return nil
}

// loadClaudeEffort validates YT2COLUMN_CLAUDE_EFFORT, which is required when
// the generation or the review provider is claude. It is called only when one
// of them is. The accepted values live in claudeparam and are not repeated
// here.
func loadClaudeEffort(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(claudeEffortEnv)
	if !ok || value == "" {
		return missingVar(claudeEffortEnv)
	}
	effort, err := claudeparam.ParseEffort(value)
	if err != nil {
		return invalidVar(claudeEffortEnv, reasonClaudeEffort)
	}
	cfg.claudeEffort = effort
	return nil
}

// loadAnthropicWorkspaceID validates ANTHROPIC_WORKSPACE_ID, which is optional
// when the generation or the review provider is claude. It is called only when
// one of them is: unset is accepted, and a present value must satisfy
// claudeparam.ParseWorkspaceID.
func loadAnthropicWorkspaceID(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(anthropicWorkspaceIDEnv)
	switch {
	case !ok:
		return nil
	case value == "":
		return missingVar(anthropicWorkspaceIDEnv)
	default:
		workspaceID, err := claudeparam.ParseWorkspaceID(value)
		if err != nil {
			return invalidVar(anthropicWorkspaceIDEnv, reasonAnthropicWorkspaceID)
		}
		cfg.anthropicWorkspaceID = workspaceID
		return nil
	}
}

// loadSlackWebhook validates SLACK_WEBHOOK_URL and returns the value, whether
// one is set, and any rejection. Unset is accepted here; a present value must
// satisfy slackwebhook.ValidURL. Load treats unset as no Webhook; LoadServer,
// where the Webhook is required, adds its own ErrMissing.
func loadSlackWebhook(lookup LookupFunc) (secret.Secret, bool, error) {
	value, ok := lookup(slackEnv)
	switch {
	case !ok:
		return secret.Secret{}, false, nil
	case value == "":
		return secret.Secret{}, false, missingVar(slackEnv)
	case !slackwebhook.ValidURL(value):
		return secret.Secret{}, false, invalidVar(slackEnv, reasonSlackWebhook)
	default:
		url, err := secret.New(value)
		if err != nil {
			return secret.Secret{}, false, missingVar(slackEnv)
		}
		return url, true, nil
	}
}

// loadCacheDir validates YT2COLUMN_CACHE_DIR. Unset derives the OS default;
// a present value must be an absolute path.
func loadCacheDir(lookup LookupFunc, cfg *Config) error {
	value, ok := lookup(cacheDirEnv)
	switch {
	case ok && value == "":
		return missingVar(cacheDirEnv)
	case ok:
		if !filepath.IsAbs(value) {
			return invalidVar(cacheDirEnv, reasonDirRelative)
		}
		cfg.cacheDir = value
		return nil
	default:
		dir, found := defaultCacheDir(runtime.GOOS, lookup)
		if !found {
			return missingVarWithReason(cacheDirEnv, reasonCacheDirAbsent)
		}
		cfg.cacheDir = dir
		return nil
	}
}

// loadYtDlpPath validates YT2COLUMN_YTDLP_PATH. Unset means "yt-dlp" on PATH; a
// present value must be non-empty.
func loadYtDlpPath(lookup LookupFunc, cfg *Config) error {
	value, ok := lookup(ytDlpEnv)
	switch {
	case !ok:
		return nil
	case value == "":
		return missingVar(ytDlpEnv)
	default:
		cfg.ytDlpPath = value
		return nil
	}
}

// HTTP2DebugEnabledIn reports whether a GODEBUG value turns on the Go HTTP/2
// transport's log ("http2debug=1" or "http2debug=2" as a substring). net/http
// enables it when GODEBUG merely contains either substring, so this uses the
// same test rather than parsing entries: a stricter parse would miss forms
// such as "http2debug=10" or a space after a comma, and the key would be
// logged without a warning.
func HTTP2DebugEnabledIn(godebug string) bool {
	return strings.Contains(godebug, "http2debug="+http2DebugInfo) ||
		strings.Contains(godebug, "http2debug="+http2DebugVerbose)
}

// hasHTTP2Debug reports whether GODEBUG may turn on the HTTP/2 transport's
// log, reading the variable through lookup. The setting is read, never
// rejected.
func hasHTTP2Debug(lookup LookupFunc) bool {
	godebug, _ := lookup(godebugEnv)
	return HTTP2DebugEnabledIn(godebug)
}

// missingVar reports name as unset or empty.
func missingVar(name string) *VarError {
	return &VarError{Name: name, Reason: reasonUnset, Err: ErrMissing}
}

// missingVarWithReason reports name as unset or empty with a fixed reason that
// explains why no default could be derived.
func missingVarWithReason(name, reason string) *VarError {
	return &VarError{Name: name, Reason: reason, Err: ErrMissing}
}

// invalidVar reports name as present but unacceptable.
func invalidVar(name, reason string) *VarError {
	return &VarError{Name: name, Reason: reason, Err: ErrInvalid}
}
