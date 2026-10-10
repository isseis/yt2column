//go:build test || integration

// Package deepseektestutil holds what the integration tests that call the real
// DeepSeek API share: the decision whether such a test runs, and the helpers
// that check the make targets running them. It is built both by the unit tests
// (`-tags test`), which test the decision, and by the integration tests
// (`-tags integration`), which use it.
package deepseektestutil

import (
	"strings"

	"github.com/isseis/yt2column/internal/secret"
)

// Environment variables read by the integration tests. The production
// DEEPSEEK_API_KEY is never read: the tests use their own key variable.
const (
	// DeepSeekOptInEnv opts in to the DeepSeek adapter's integration test;
	// `make test-integration-deepseek` exports it.
	DeepSeekOptInEnv = "YT2COLUMN_DEEPSEEK_INTEGRATION"
	// CLIOptInEnv opts in to the CLI's integration test;
	// `make test-integration-cli` exports it.
	CLIOptInEnv = "YT2COLUMN_CLI_INTEGRATION"
	APIKeyEnv   = "YT2COLUMN_TEST_DEEPSEEK_API_KEY" //nolint:gosec // the name of the variable holding the test key, not a key
	ModelEnv    = "YT2COLUMN_MODEL"
	GODEBUGEnv  = "GODEBUG"

	// OptInValue is the only opt-in value that runs a test.
	OptInValue = "1"
)

// http2VerboseSettings are the GODEBUG settings that make the HTTP/2 transport
// log every request header, the API key header included, to standard error. The
// transport reads GODEBUG once at program start and matches these substrings,
// so removing them from the environment inside the test would come too late;
// the test fails instead.
var http2VerboseSettings = []string{"http2debug=1", "http2debug=2"}

// MissingKeyAction is what the test does when the opt-in is set but the test
// API key is not. The zero value fails: a skip is hard to tell from a pass, so
// failing is the side that cannot hide a misconfigured run.
type MissingKeyAction int

// The missing-key actions.
const (
	MissingKeyFail MissingKeyAction = iota
	MissingKeySkip
)

// IntegrationOptions names the opt-in variable, the make target shown in the
// skip reason, and the missing-key action of one integration test.
type IntegrationOptions struct {
	OptInEnv   string
	MakeTarget string
	MissingKey MissingKeyAction
}

// IntegrationAction is what the integration test does. The zero value skips,
// so a decision that was never made sends no request and incurs no charge.
type IntegrationAction int

// The integration actions.
const (
	ActionSkip IntegrationAction = iota
	ActionFail
	ActionRun
)

// IntegrationSettings is the outcome of reading the environment. Reason names
// the variable behind a skip or a failure and never contains the API key;
// APIKey and Model are set only when Action is ActionRun.
type IntegrationSettings struct {
	Action IntegrationAction
	Reason string
	APIKey secret.Secret
	Model  string
}

// SettingsFrom decides whether an integration test that calls the real
// DeepSeek API runs. The checks run in a fixed order: a missing opt-in skips
// (so a plain `go test -tags integration` or an IDE run never incurs a
// charge), a missing test API key skips or fails as opts.MissingKey says, a
// missing model name fails, and an HTTP/2 debug setting that would print the
// API key fails.
func SettingsFrom(getenv func(string) string, opts IntegrationOptions) IntegrationSettings {
	if getenv(opts.OptInEnv) != OptInValue {
		return IntegrationSettings{
			Action: ActionSkip,
			Reason: opts.OptInEnv + " is not " + OptInValue +
				": the integration test calls the real DeepSeek API and incurs charges; run it with `make " + opts.MakeTarget + "`",
		}
	}
	key := getenv(APIKeyEnv)
	if key == "" {
		reason := APIKeyEnv + " is not set: set it to a DeepSeek API key for testing (DEEPSEEK_API_KEY is not used)"
		switch opts.MissingKey {
		case MissingKeySkip:
			return IntegrationSettings{Action: ActionSkip, Reason: reason}
		default:
			return IntegrationSettings{Action: ActionFail, Reason: reason}
		}
	}
	model := getenv(ModelEnv)
	if model == "" {
		return IntegrationSettings{
			Action: ActionFail,
			Reason: ModelEnv + " is not set: set it to the model name, or run `make " + opts.MakeTarget + "`",
		}
	}
	godebug := getenv(GODEBUGEnv)
	for _, setting := range http2VerboseSettings {
		if strings.Contains(godebug, setting) {
			return IntegrationSettings{
				Action: ActionFail,
				Reason: GODEBUGEnv + " contains " + setting + ", which would print the API key to standard error: unset it before running the integration test",
			}
		}
	}
	apiKey, err := secret.New(key)
	if err != nil {
		// Unreachable: secret.New rejects only the empty value, which is
		// handled above. Kept so an error is never dropped.
		return IntegrationSettings{Action: ActionFail, Reason: APIKeyEnv + ": " + err.Error()}
	}
	return IntegrationSettings{Action: ActionRun, APIKey: apiKey, Model: model}
}
