// Package config loads and validates the process configuration from
// environment variables. It is the only package that names the secret
// variables; every other package receives already-validated values.
package config

import (
	"errors"
	"path/filepath"
	"runtime"
	"strconv"
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
	reviewProviderEnv       = "YT2COLUMN_REVIEW_PROVIDER"
	reviewModelEnv          = "YT2COLUMN_REVIEW_MODEL"
	deepSeekAPIKeyEnv       = "DEEPSEEK_API_KEY"  //nolint:gosec // the variable's name, not a credential value
	anthropicAPIKeyEnv      = "ANTHROPIC_API_KEY" //nolint:gosec // the variable's name, not a credential value
	claudeEffortEnv         = "YT2COLUMN_CLAUDE_EFFORT"
	anthropicWorkspaceIDEnv = "ANTHROPIC_WORKSPACE_ID"
	slackEnv                = "SLACK_WEBHOOK_URL"
	cacheDirEnv             = "YT2COLUMN_CACHE_DIR"
	ytDlpEnv                = "YT2COLUMN_YTDLP_PATH"
	serverPortEnv           = "YT2COLUMN_SERVER_PORT"
	godebugEnv              = "GODEBUG"
)

// The accepted range and default of YT2COLUMN_SERVER_PORT. Port 0, which lets
// the operating system choose, is rejected because the extension has no way to
// learn the chosen port. Ports below 1024 are rejected because the server runs
// with ordinary user privileges and has no reason to use a privileged one.
const (
	defaultServerPort = 8765
	minServerPort     = 1024
	maxServerPort     = 65535
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
	reasonConfigDirAbsent      = "no absolute default could be derived from HOME or XDG_CONFIG_HOME"
	reasonServerPort           = "must be a decimal integer from 1024 through 65535"
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
// Load or LoadServer.
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

// Model returns the model name of the generation. It is non-empty.
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
// Config can only come from Load.
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
	port              uint16
	tokenPath         string // absolute
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

// Port returns the TCP port the server listens on. It is in the accepted range.
func (c ServerConfig) Port() uint16 {
	return c.port
}

// TokenPath returns the absolute path of the shared token file.
func (c ServerConfig) TokenPath() string {
	return c.tokenPath
}

// HTTP2DebugEnabled reports whether GODEBUG contains http2debug=1 or
// http2debug=2. LoadServer never rejects this setting; the caller warns instead.
func (c ServerConfig) HTTP2DebugEnabled() bool {
	return c.http2DebugEnabled
}

// LoadServer reads and validates the server's variables through lookup: the
// LLM settings (shared with Load), SLACK_WEBHOOK_URL (required), the listening
// port, and the shared token file's path. It joins every rejection together and
// returns the zero ServerConfig on any error, so a value that did not pass
// validation never reaches a caller. It reads no variable the server does not
// use.
func LoadServer(lookup LookupFunc) (ServerConfig, error) {
	llm, errs := loadLLM(lookup)
	var cfg ServerConfig
	webhook, set, webhookErr := loadSlackWebhook(lookup)
	switch {
	case webhookErr != nil:
		errs = append(errs, webhookErr)
	case !set:
		errs = append(errs, missingVar(slackEnv))
	}
	port, portErr := loadServerPort(lookup)
	if portErr != nil {
		errs = append(errs, portErr)
	}
	tokenPath, tokenErr := loadTokenPath(lookup)
	if tokenErr != nil {
		errs = append(errs, tokenErr)
	}
	if err := errors.Join(errs...); err != nil {
		return ServerConfig{}, err
	}
	cfg.llm = llm
	cfg.webhookURL = webhook
	cfg.port = port
	cfg.tokenPath = tokenPath
	cfg.http2DebugEnabled = hasHTTP2Debug(lookup)
	return cfg, nil
}

// loadServerPort validates YT2COLUMN_SERVER_PORT. Unset defaults to
// defaultServerPort; a present value must be in the accepted form.
func loadServerPort(lookup LookupFunc) (uint16, error) {
	value, ok := lookup(serverPortEnv)
	if !ok {
		return defaultServerPort, nil
	}
	return parseServerPort(value)
}

// parseServerPort converts a YT2COLUMN_SERVER_PORT value into a port. The
// accepted form is decimal digits only, with no leading zero, in the range
// minServerPort through maxServerPort. An empty value is missing; every other
// rejected form is invalid. The check uses strconv and a byte comparison rather
// than net, because internal/config must not import net.
func parseServerPort(value string) (uint16, error) {
	switch {
	case value == "":
		return 0, missingVar(serverPortEnv)
	case !isDecimalDigits(value) || value[0] == '0':
		return 0, invalidVar(serverPortEnv, reasonServerPort)
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < minServerPort || n > maxServerPort {
		return 0, invalidVar(serverPortEnv, reasonServerPort)
	}
	return uint16(n), nil
}

// isDecimalDigits reports whether s is non-empty and all of its bytes are
// decimal digits.
func isDecimalDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// loadTokenPath derives the shared token file's path from the configuration
// directory. A failure to derive the directory is reported with the variable
// that prevented it.
func loadTokenPath(lookup LookupFunc) (string, error) {
	dir, err := defaultConfigDir(runtime.GOOS, lookup)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tokenFileName), nil
}

// Load reads and validates every variable through lookup. It joins every
// rejection together so callers can report all of them; on any error it returns
// the zero Config, because a value that did not pass validation must not reach
// a caller.
func Load(lookup LookupFunc) (Config, error) {
	llm, errs := loadLLM(lookup)
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

// loadLLM reads and validates every LLM variable: the generation and review
// providers and models, and the variables of the selected providers. It returns
// the validated LLMConfig and each rejection separately, so Load and LoadServer
// report them the same way.
func loadLLM(lookup LookupFunc) (LLMConfig, []error) {
	var cfg LLMConfig
	errs := []error{
		loadProvider(lookup, &cfg),
		loadModel(lookup, &cfg),
	}
	errs = append(errs, loadReview(lookup, &cfg)...)
	errs = append(errs, loadProviderVars(lookup, &cfg)...)
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
// provider. It returns each rejection as a separate element so Load joins them
// flat.
func loadReview(lookup LookupFunc, cfg *LLMConfig) []error {
	var errs []error
	providerValue, providerPresent := lookup(reviewProviderEnv)
	modelValue, modelPresent := lookup(reviewModelEnv)

	providerOK := true
	if providerPresent {
		provider, err := parseProvider(reviewProviderEnv, providerValue, true)
		if err != nil {
			errs = append(errs, err)
			providerOK = false
		} else {
			cfg.reviewProvider = provider
		}
	} else {
		cfg.reviewProvider = cfg.provider
	}

	switch {
	case modelPresent && modelValue == "":
		errs = append(errs, missingVar(reviewModelEnv))
	case modelPresent:
		cfg.reviewModel = modelValue
	case !providerPresent:
		// Neither review variable is set: inherit the generation model. When
		// the generation provider was rejected, cfg.model is empty too, but
		// Load already reports that rejection.
		cfg.reviewModel = cfg.model
	case providerOK:
		// A review provider with no model of its own is incomplete.
		errs = append(errs, missingVar(reviewModelEnv))
	}
	return errs
}

// loadProviderVars returns the rejections of the selected providers' variables
// and reads no others. A provider's variables are read when either the
// generation or the review provider is that provider, so a key shared by both
// is read once. It returns nothing when neither provider is set, which happens
// only when YT2COLUMN_LLM_PROVIDER was rejected and the review provider fell
// back to it, so an invalid generation provider is reported alone and no
// provider variable rejects a value the user did not intend to use. Each
// rejection is a separate element so Load joins them flat; a nested join would
// reach a caller that prints one error per line as a single line.
func loadProviderVars(lookup LookupFunc, cfg *LLMConfig) []error {
	var errs []error
	if cfg.provider == ProviderDeepSeek || cfg.reviewProvider == ProviderDeepSeek {
		errs = append(errs, loadDeepSeekAPIKey(lookup, cfg))
	}
	if cfg.provider == ProviderClaude || cfg.reviewProvider == ProviderClaude {
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
