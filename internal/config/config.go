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
// it only for a stage the run does not need (Need.Generation or Need.Review is
// false), whose client is never built.
const (
	ProviderUnset Provider = iota
	ProviderDeepSeek
	ProviderClaude
)

// Names of the environment variables Load reads.
const (
	llmEnv                  = "YT2COLUMN_LLM"
	reviewLLMEnv            = "YT2COLUMN_REVIEW_LLM"
	deepSeekAPIKeyEnv       = "DEEPSEEK_API_KEY"  //nolint:gosec // the variable's name, not a credential value
	anthropicAPIKeyEnv      = "ANTHROPIC_API_KEY" //nolint:gosec // the variable's name, not a credential value
	claudeEffortEnv         = "YT2COLUMN_CLAUDE_EFFORT"
	anthropicWorkspaceIDEnv = "ANTHROPIC_WORKSPACE_ID"
	slackEnv                = "SLACK_WEBHOOK_URL"
	cacheDirEnv             = "YT2COLUMN_CACHE_DIR"
	ytDlpEnv                = "YT2COLUMN_YTDLP_PATH"
	godebugEnv              = "GODEBUG"
)

// The accepted provider parts of llmEnv and reviewLLMEnv.
const (
	providerDeepSeek = "deepseek"
	providerClaude   = "claude"
)

// llmSeparator separates the provider from the model ID in an LLM variable.
// Only its first occurrence separates them; a later one belongs to the model
// ID.
const llmSeparator = "/"

// The default applied when YT2COLUMN_LLM is unset. Each value is written here
// alone, so changing the default touches one line of code and the documents
// that state it.
const (
	// defaultProvider is the provider part of the default selection. The
	// retired variables' rejection names it too, so it is written once.
	defaultProvider = providerClaude
	// defaultClaudeModel is the model ID of the default selection.
	defaultClaudeModel = "claude-opus-5-5"
	// defaultLLMValue is the YT2COLUMN_LLM value used when the variable is
	// unset. It goes through parseLLM like a present value, so the
	// string-to-Provider mapping stays in one place.
	defaultLLMValue = defaultProvider + llmSeparator + defaultClaudeModel
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
	reasonLLMForm              = `must be "<provider>/<model ID>" with a non-empty model ID`
	reasonLLMProvider          = `the provider before the first "/" must be exactly "` + providerDeepSeek + `" or "` + providerClaude + `"`
	reasonClaudeEffort         = "must be a supported effort value"
	reasonAnthropicWorkspaceID = "must be a non-empty string of printable ASCII characters other than space"
	reasonSlackWebhook         = "must be an https URL with a host"
	reasonDirRelative          = "must be an absolute path"
	reasonCacheDirAbsent       = "no absolute default could be derived from HOME or XDG_CACHE_HOME"
	reasonSocketPathAbsent     = "no absolute socket path could be derived from HOME"
	reasonSocketPathTooLong    = "the socket path derived from HOME exceeds the operating system's limit"
	reasonHomeAbsent           = "must be set to an absolute path"
	// reasonRetiredLLM and reasonRetiredReviewLLM reject a retired variable.
	// Each says the variable must be unset, which variable replaces it, and
	// what is used while the replacement is unset, so the line alone tells a
	// user that removing the retired variable without setting its replacement
	// sends the data to the default provider.
	reasonRetiredLLM = "is no longer supported and must be unset (an empty value counts as set); set " +
		llmEnv + "=<provider>/<model ID> instead; while " + llmEnv + " is unset, the provider is " + defaultProvider
	reasonRetiredReviewLLM = "is no longer supported and must be unset (an empty value counts as set); set " +
		reviewLLMEnv + "=<provider>/<model ID> instead; while " + reviewLLMEnv + " is unset, the review step uses " + llmEnv
)

// retiredLLMVar is a variable that selected a provider or a model before
// YT2COLUMN_LLM and YT2COLUMN_REVIEW_LLM. Its presence is a rejection; its
// value is never used.
type retiredLLMVar struct {
	name   string
	reason string // reasonRetiredLLM or reasonRetiredReviewLLM
}

// retiredLLMVars lists the retired variables, each with the reason naming its
// replacement, in the order their rejections are reported.
var retiredLLMVars = []retiredLLMVar{
	{"YT2COLUMN_LLM_PROVIDER", reasonRetiredLLM},
	{"YT2COLUMN_MODEL", reasonRetiredLLM},
	{"YT2COLUMN_REVIEW_PROVIDER", reasonRetiredReviewLLM},
	{"YT2COLUMN_REVIEW_MODEL", reasonRetiredReviewLLM},
}

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

// llmSelection is one stage's provider and model ID. In an accepted selection
// both are set. parseLLM also returns the provider of a rejected value whose
// provider part was accepted (an empty model ID), so that provider's own
// variables are still checked; such a selection never leaves loadLLM, because
// the load then fails. ProviderUnset means the stage was not resolved or its
// provider part was rejected.
type llmSelection struct {
	provider Provider
	model    string
}

// LLMConfig is the validated configuration of the generation and review LLM
// calls. Its fields are unexported, so a non-zero LLMConfig can only come from
// Load, LoadFor, or LoadServer.
type LLMConfig struct {
	generation           llmSelection
	review               llmSelection
	deepSeekAPIKey       secret.Secret
	anthropicAPIKey      secret.Secret
	claudeEffort         claudeparam.Effort
	anthropicWorkspaceID claudeparam.WorkspaceID
}

// Provider returns the LLM provider of the generation and the shorten step:
// the provider of YT2COLUMN_LLM, or of defaultLLMValue when the variable is
// unset. It is set only when the run needs the generation stage
// (Need.Generation); otherwise it is ProviderUnset and no generation client is
// built.
func (c LLMConfig) Provider() Provider {
	return c.generation.provider
}

// Model returns the model ID of the generation and the shorten step, from the
// same value as Provider. It is non-empty exactly when Provider is set.
func (c LLMConfig) Model() string {
	return c.generation.model
}

// ReviewProvider returns the provider of the review step: the provider of
// YT2COLUMN_REVIEW_LLM, or, when that variable is unset, the generation
// selection (YT2COLUMN_LLM or its default). It is set only when the run needs
// the review stage (Need.Review); otherwise it is ProviderUnset.
func (c LLMConfig) ReviewProvider() Provider {
	return c.review.provider
}

// ReviewModel returns the model ID of the review step, from the same value as
// ReviewProvider. It is non-empty exactly when ReviewProvider is set.
func (c LLMConfig) ReviewModel() string {
	return c.review.model
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

// ClaudeEffort returns the effort: the value of YT2COLUMN_CLAUDE_EFFORT, or
// defaultClaudeEffort when it is unset. It is not claudeparam.EffortUnset when
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
	webhookURL        secret.Secret // zero when SLACK_WEBHOOK_URL is unset
	socketPath        string        // absolute, within the OS limit
	http2DebugEnabled bool
}

// LLM returns the validated LLM configuration.
func (c ServerConfig) LLM() LLMConfig {
	return c.llm
}

// WebhookURL returns the Webhook URL and whether one is configured. The server
// runs without one and rejects what needs it.
func (c ServerConfig) WebhookURL() (secret.Secret, bool) {
	if _, err := c.webhookURL.Reveal(); err != nil {
		return secret.Secret{}, false
	}
	return c.webhookURL, true
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
// LLM settings (shared with Load), SLACK_WEBHOOK_URL (optional), and HOME, from
// which the socket path is derived. It joins every rejection together and
// returns the zero ServerConfig on any error, so a value that did not pass
// validation never reaches a caller. It reads no variable the server does not
// use, with one exception: it checks whether each retired LLM variable is
// present (never its value), so a leftover setting stops the server instead of
// silently changing the provider.
func LoadServer(lookup LookupFunc) (ServerConfig, error) {
	llm, errs := loadLLM(lookup, Need{Generation: true, Review: true})
	var cfg ServerConfig
	webhook, webhookErr := loadSlackWebhook(lookup)
	if webhookErr != nil {
		errs = append(errs, webhookErr)
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
// zero Config on any error. A need that is true resolves its stage's provider
// and model, from its variable or the default, and reads and validates the
// selected providers' own variables; a need that is false does not check them.
// When Review is true but Generation is false and YT2COLUMN_REVIEW_LLM is
// unset, YT2COLUMN_LLM is still read and validated, because the review step
// inherits its selection. The one exception to reading only what the run
// needs: whatever need is, LoadFor checks whether each retired LLM variable is
// present (never its value) and rejects the run if one is, so a leftover
// setting is noticed even by a run that calls no LLM.
func LoadFor(lookup LookupFunc, need Need) (Config, error) {
	llm, errs := loadLLM(lookup, need)
	var cfg Config
	webhook, webhookErr := loadSlackWebhook(lookup)
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
// and review selections, and the variables of the selected providers. It
// returns the validated LLMConfig and each rejection separately, so LoadFor and
// LoadServer report them the same way.
//
// Contract: the retired variables are checked first, whatever need is, and
// their values are never read; every other variable is read only for a stage
// the run needs. A rejected value is never replaced by a default, so a run
// with any rejection sends nothing to a provider. The generation selection is
// resolved at most once: for the generation stage, or for a review stage that
// inherits it because YT2COLUMN_REVIEW_LLM is unset; it is stored in the
// generation accessors only when need.Generation is true.
func loadLLM(lookup LookupFunc, need Need) (LLMConfig, []error) {
	var cfg LLMConfig
	errs := retiredLLMErrors(lookup)

	reviewValue, reviewSet := "", false
	if need.Review {
		reviewValue, reviewSet = lookup(reviewLLMEnv)
	}
	var generation llmSelection
	if need.Generation || (need.Review && !reviewSet) {
		var err error
		generation, err = resolveGeneration(lookup)
		errs = append(errs, err)
	}
	if need.Generation {
		cfg.generation = generation
	}
	if need.Review {
		if reviewSet {
			var err error
			cfg.review, err = parseLLM(reviewLLMEnv, reviewValue)
			errs = append(errs, err)
		} else {
			// The inherited selection's rejection, if any, was reported once
			// by resolveGeneration.
			cfg.review = generation
		}
	}
	errs = append(errs, loadProviderVars(lookup, &cfg, need)...)
	return cfg, errs
}

// retiredLLMErrors returns one rejection for each retired variable that is
// present, an empty value included, in the order of retiredLLMVars. It reads
// whether each is present and never uses its value.
func retiredLLMErrors(lookup LookupFunc) []error {
	var errs []error
	for _, v := range retiredLLMVars {
		if _, ok := lookup(v.name); ok {
			errs = append(errs, invalidVar(v.name, v.reason))
		}
	}
	return errs
}

// resolveGeneration reads YT2COLUMN_LLM. When it is unset, it parses
// defaultLLMValue instead, with the same parseLLM as a present value, so the
// default goes through the one string-to-Provider mapping. A rejection names
// YT2COLUMN_LLM.
func resolveGeneration(lookup LookupFunc) (llmSelection, error) {
	value, ok := lookup(llmEnv)
	if !ok {
		value = defaultLLMValue
	}
	return parseLLM(llmEnv, value)
}

// parseLLM converts a present LLM variable's value, "<provider>/<model ID>",
// into a selection. name is the variable being read, so a rejection names it.
// The value is split at its first "/"; the model ID is the rest, kept as it is
// (a later "/" and surrounding white space included; the adapters reject a
// model ID they cannot send). Nothing is trimmed or case-folded.
//
// Contract: when the provider part is accepted, the returned selection holds
// that provider even with an error (an empty model ID), so the provider's own
// variables are still checked; otherwise its provider is ProviderUnset.
func parseLLM(name, value string) (llmSelection, error) {
	if value == "" {
		return llmSelection{}, missingVar(name)
	}
	providerPart, model, found := strings.Cut(value, llmSeparator)
	if !found {
		return llmSelection{}, invalidVar(name, reasonLLMForm)
	}
	var provider Provider
	switch providerPart {
	case providerDeepSeek:
		provider = ProviderDeepSeek
	case providerClaude:
		provider = ProviderClaude
	default:
		return llmSelection{}, invalidVar(name, reasonLLMProvider)
	}
	if model == "" {
		return llmSelection{provider: provider}, invalidVar(name, reasonLLMForm)
	}
	return llmSelection{provider: provider, model: model}, nil
}

// loadProviderVars returns the rejections of the selected providers' variables
// and reads no others. A provider's variables are read only when a stage the
// run needs (need) uses that provider, so a key shared by the generation and
// review stages is read once and a stage that does not run contributes
// nothing. It returns nothing when no needed stage uses either provider, so an
// LLM variable whose provider part was rejected is reported once by parseLLM
// and no provider variable rejects a value the user did not intend to use. Each
// rejection is a separate element so LoadFor joins them flat; a nested join
// would reach a caller that prints one error per line as a single line.
func loadProviderVars(lookup LookupFunc, cfg *LLMConfig, need Need) []error {
	var errs []error
	deepSeek := (need.Generation && cfg.generation.provider == ProviderDeepSeek) ||
		(need.Review && cfg.review.provider == ProviderDeepSeek)
	if deepSeek {
		errs = append(errs, loadDeepSeekAPIKey(lookup, cfg))
	}
	claude := (need.Generation && cfg.generation.provider == ProviderClaude) ||
		(need.Review && cfg.review.provider == ProviderClaude)
	if claude {
		errs = append(errs,
			loadAnthropicAPIKey(lookup, cfg),
			loadClaudeEffort(lookup, cfg),
			loadAnthropicWorkspaceID(lookup, cfg),
		)
	}
	return errs
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

// defaultClaudeEffort is the effort used when YT2COLUMN_CLAUDE_EFFORT is
// unset and a stage the run needs uses the claude provider. The adapter's
// default output limit was sized for this effort; changing it calls for the
// re-measurement in docs/dev/developer_guide/claude_output_limit.md.
const defaultClaudeEffort = claudeparam.EffortHigh

// loadClaudeEffort validates YT2COLUMN_CLAUDE_EFFORT when the generation or
// the review provider is claude. It is called only when one of them is. Unset
// selects defaultClaudeEffort; an empty value is rejected, not treated as
// unset. The accepted values live in claudeparam and are not repeated here.
func loadClaudeEffort(lookup LookupFunc, cfg *LLMConfig) error {
	value, ok := lookup(claudeEffortEnv)
	if !ok {
		cfg.claudeEffort = defaultClaudeEffort
		return nil
	}
	if value == "" {
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

// loadSlackWebhook validates SLACK_WEBHOOK_URL and returns the value and any
// rejection. Unset is accepted and returns the zero secret; a present value
// must satisfy slackwebhook.ValidURL. Load and LoadServer both treat the zero
// secret as no Webhook.
func loadSlackWebhook(lookup LookupFunc) (secret.Secret, error) {
	value, ok := lookup(slackEnv)
	switch {
	case !ok:
		return secret.Secret{}, nil
	case value == "":
		return secret.Secret{}, missingVar(slackEnv)
	case !slackwebhook.ValidURL(value):
		return secret.Secret{}, invalidVar(slackEnv, reasonSlackWebhook)
	default:
		url, err := secret.New(value)
		if err != nil {
			return secret.Secret{}, missingVar(slackEnv)
		}
		return url, nil
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
