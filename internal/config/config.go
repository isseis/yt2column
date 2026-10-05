// Package config loads and validates the process configuration from
// environment variables. It is the only package that names the secret
// variables; every other package receives already-validated values.
package config

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/isseis/yt2column/internal/secret"
)

// Provider is the LLM provider. Its zero value is ProviderUnset.
type Provider int

// The supported Provider values. ProviderUnset is the zero value, which no
// factory accepts, so Load never stores it in a Config it returns.
const (
	ProviderUnset Provider = iota
	ProviderDeepSeek
)

// Names of the environment variables Load reads.
const (
	providerEnv = "YT2COLUMN_LLM_PROVIDER"
	modelEnv    = "YT2COLUMN_MODEL"
	apiKeyEnv   = "DEEPSEEK_API_KEY" //nolint:gosec // the variable's name, not a credential value
	slackEnv    = "SLACK_WEBHOOK_URL"
	cacheDirEnv = "YT2COLUMN_CACHE_DIR"
	ytDlpEnv    = "YT2COLUMN_YTDLP_PATH"
	godebugEnv  = "GODEBUG"
)

// providerDeepSeek is the only accepted value of providerEnv.
const providerDeepSeek = "deepseek"

// The GODEBUG values that turn on the Go HTTP/2 transport's verbose log, which
// writes the Authorization header to standard error.
const (
	http2DebugInfo    = "1"
	http2DebugVerbose = "2"
)

// Fixed reasons for a rejected variable. A reason is chosen from these
// constants and built from nothing else, so a rejected value cannot reach an
// error message.
const (
	reasonUnset          = "is unset or empty"
	reasonProvider       = `must be exactly "deepseek"`
	reasonSlackWebhook   = `must start with "https://hooks.slack.com/" and contain no whitespace`
	reasonCacheDir       = "must be an absolute path"
	reasonCacheDirAbsent = "no absolute default could be derived from HOME or XDG_CACHE_HOME"
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
	provider          Provider
	model             string
	deepSeekAPIKey    secret.Secret
	slackWebhookURL   secret.Secret
	cacheDir          string
	ytDlpPath         string
	http2DebugEnabled bool
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

// SlackWebhookURL returns the Webhook URL and whether one is configured.
func (c Config) SlackWebhookURL() (secret.Secret, bool) {
	if _, err := c.slackWebhookURL.Reveal(); err != nil {
		return secret.Secret{}, false
	}
	return c.slackWebhookURL, true
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
		loadAPIKey(lookup, &cfg),
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
	default:
		return invalidVar(providerEnv, reasonProvider)
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

// loadAPIKey validates DEEPSEEK_API_KEY, which is required when the provider is
// deepseek. An empty value is rejected however the provider is set; a non-empty
// value is kept only for deepseek.
func loadAPIKey(lookup LookupFunc, cfg *Config) error {
	value, ok := lookup(apiKeyEnv)
	switch {
	case !ok:
		if cfg.provider == ProviderDeepSeek {
			return missingVar(apiKeyEnv)
		}
		return nil
	case value == "":
		return missingVar(apiKeyEnv)
	case cfg.provider != ProviderDeepSeek:
		return nil
	default:
		key, err := secret.New(value)
		if err != nil {
			return missingVar(apiKeyEnv)
		}
		cfg.deepSeekAPIKey = key
		return nil
	}
}

// loadSlackWebhook validates SLACK_WEBHOOK_URL. Unset is accepted; a present
// value must be a hooks.slack.com URL.
func loadSlackWebhook(lookup LookupFunc, cfg *Config) error {
	value, ok := lookup(slackEnv)
	switch {
	case !ok:
		return nil
	case value == "":
		return missingVar(slackEnv)
	case !validSlackWebhook(value):
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

// validSlackWebhook reports whether value is a Slack Incoming Webhook URL: the
// fixed prefix, at least one more character, and no whitespace anywhere.
func validSlackWebhook(value string) bool {
	rest, found := strings.CutPrefix(value, "https://hooks.slack.com/")
	return found && rest != "" && !strings.ContainsFunc(rest, unicode.IsSpace)
}

// hasHTTP2Debug reports whether GODEBUG has an http2debug=1 or http2debug=2
// entry. The setting is read, never rejected.
func hasHTTP2Debug(lookup LookupFunc) bool {
	godebug, _ := lookup(godebugEnv)
	for entry := range strings.SplitSeq(godebug, ",") {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == "http2debug" && (value == http2DebugInfo || value == http2DebugVerbose) {
			return true
		}
	}
	return false
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
