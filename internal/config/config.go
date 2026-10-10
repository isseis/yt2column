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
// factory accepts, so Load never stores it in a Config it returns.
const (
	ProviderUnset Provider = iota
	ProviderDeepSeek
	ProviderClaude
)

// Names of the environment variables Load reads.
const (
	providerEnv             = "YT2COLUMN_LLM_PROVIDER"
	modelEnv                = "YT2COLUMN_MODEL"
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
	reasonCacheDir             = "must be an absolute path"
	reasonCacheDirAbsent       = "no absolute default could be derived from HOME or XDG_CACHE_HOME"
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

// Config is the validated configuration. Its fields are unexported, so a
// non-zero Config can only come from Load.
type Config struct {
	provider             Provider
	model                string
	deepSeekAPIKey       secret.Secret
	anthropicAPIKey      secret.Secret
	claudeEffort         claudeparam.Effort
	anthropicWorkspaceID claudeparam.WorkspaceID
	slackWebhookURL      secret.Secret
	cacheDir             string
	ytDlpPath            string
	http2DebugEnabled    bool
}

// Provider returns the LLM provider.
func (c Config) Provider() Provider {
	return c.provider
}

// Model returns the model name. It is non-empty.
func (c Config) Model() string {
	return c.model
}

// DeepSeekAPIKey returns the API key. It is non-zero when the provider is
// ProviderDeepSeek.
func (c Config) DeepSeekAPIKey() secret.Secret {
	return c.deepSeekAPIKey
}

// AnthropicAPIKey returns the Anthropic API key. It is non-zero when the
// provider is ProviderClaude.
func (c Config) AnthropicAPIKey() secret.Secret {
	return c.anthropicAPIKey
}

// ClaudeEffort returns the effort. It is not claudeparam.EffortUnset when
// the provider is ProviderClaude.
func (c Config) ClaudeEffort() claudeparam.Effort {
	return c.claudeEffort
}

// AnthropicWorkspaceID returns the workspace ID. It is the zero value when
// ANTHROPIC_WORKSPACE_ID is unset or the provider is not ProviderClaude.
func (c Config) AnthropicWorkspaceID() claudeparam.WorkspaceID {
	return c.anthropicWorkspaceID
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

// Load reads and validates every variable through lookup. It joins every
// rejection together so callers can report all of them; on any error it returns
// the zero Config, because a value that did not pass validation must not reach
// a caller.
func Load(lookup LookupFunc) (Config, error) {
	var cfg Config
	errs := []error{
		loadProvider(lookup, &cfg),
		loadModel(lookup, &cfg),
		loadProviderVars(lookup, &cfg),
		loadSlackWebhook(lookup, &cfg),
		loadCacheDir(lookup, &cfg),
		loadYtDlpPath(lookup, &cfg),
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	cfg.http2DebugEnabled = hasHTTP2Debug(lookup)
	return cfg, nil
}

// loadProvider validates YT2COLUMN_LLM_PROVIDER. Unset defaults to deepseek.
func loadProvider(lookup LookupFunc, cfg *Config) error {
	value, ok := lookup(providerEnv)
	switch {
	case !ok:
		cfg.provider = ProviderDeepSeek
		return nil
	case value == "":
		return missingVar(providerEnv)
	case value == providerDeepSeek:
		cfg.provider = ProviderDeepSeek
		return nil
	case value == providerClaude:
		cfg.provider = ProviderClaude
		return nil
	default:
		return invalidVar(providerEnv, reasonProvider)
	}
}

// loadProviderVars reads the variables of the selected provider and no others.
// It reads nothing when the provider is unset, which happens only when
// YT2COLUMN_LLM_PROVIDER was rejected, so an invalid provider is reported
// alone and no provider variable rejects a value the user did not intend to
// use.
func loadProviderVars(lookup LookupFunc, cfg *Config) error {
	switch cfg.provider {
	case ProviderDeepSeek:
		return loadDeepSeekAPIKey(lookup, cfg)
	case ProviderClaude:
		return errors.Join(
			loadAnthropicAPIKey(lookup, cfg),
			loadClaudeEffort(lookup, cfg),
			loadAnthropicWorkspaceID(lookup, cfg),
		)
	default:
		return nil
	}
}

// loadModel validates YT2COLUMN_MODEL, which has no default.
func loadModel(lookup LookupFunc, cfg *Config) error {
	value, ok := lookup(modelEnv)
	if !ok || value == "" {
		return missingVar(modelEnv)
	}
	cfg.model = value
	return nil
}

// loadDeepSeekAPIKey validates DEEPSEEK_API_KEY, which is required when the
// provider is deepseek. It is called only for that provider, so the variable is
// never read, and never rejected, for another provider.
func loadDeepSeekAPIKey(lookup LookupFunc, cfg *Config) error {
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
// provider is claude. It is called only for that provider, so the variable is
// never read, and never rejected, for another provider.
func loadAnthropicAPIKey(lookup LookupFunc, cfg *Config) error {
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
// the provider is claude. It is called only for that provider. The accepted
// values live in claudeparam and are not repeated here.
func loadClaudeEffort(lookup LookupFunc, cfg *Config) error {
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
// when the provider is claude. It is called only for that provider: unset is
// accepted, and a present value must satisfy claudeparam.ParseWorkspaceID.
func loadAnthropicWorkspaceID(lookup LookupFunc, cfg *Config) error {
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

// loadSlackWebhook validates SLACK_WEBHOOK_URL. Unset is accepted; a present
// value must satisfy slackwebhook.ValidURL.
func loadSlackWebhook(lookup LookupFunc, cfg *Config) error {
	value, ok := lookup(slackEnv)
	switch {
	case !ok:
		return nil
	case value == "":
		return missingVar(slackEnv)
	case !slackwebhook.ValidURL(value):
		return invalidVar(slackEnv, reasonSlackWebhook)
	default:
		url, err := secret.New(value)
		if err != nil {
			return missingVar(slackEnv)
		}
		cfg.slackWebhookURL = url
		return nil
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
			return invalidVar(cacheDirEnv, reasonCacheDir)
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
