//go:build test || integration

// Package openaitestutil holds what the integration test that calls the real
// OpenAI Responses API needs: the decision whether it runs. It is built both
// by the unit tests (`-tags test`), which test the decision, and by the
// integration test (`-tags integration`), which uses it.
package openaitestutil

import (
	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
	"github.com/isseis/yt2column/internal/secret"
)

// Environment variables read by the integration test. The production
// OPENAI_API_KEY is never read: the test uses its own variable.
const (
	// OptInEnv opts in to the OpenAI adapter's integration test;
	// `make test-integration-openai` exports it.
	OptInEnv   = "YT2COLUMN_OPENAI_INTEGRATION"
	APIKeyEnv  = "YT2COLUMN_TEST_OPENAI_API_KEY" //nolint:gosec // the name of the variable holding the test key, not a key
	ModelEnv   = "YT2COLUMN_TEST_OPENAI_MODEL"
	EffortEnv  = "YT2COLUMN_TEST_OPENAI_EFFORT"
	GODEBUGEnv = "GODEBUG"

	// OptInValue is the only opt-in value that runs the test.
	OptInValue = "1"

	// MakeDefaultModel and MakeDefaultEffort are the model and effort
	// `make test-integration-openai` passes when ModelEnv and EffortEnv are
	// undefined. The make target test and the development guide's guard
	// compare the Makefile and the guide with them.
	MakeDefaultModel  = "gpt-6.1-sol"
	MakeDefaultEffort = "medium"
)

// IntegrationOptions names the opt-in variable and the make target shown in
// the skip and failure reasons.
type IntegrationOptions struct {
	OptInEnv   string
	MakeTarget string
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
// the variable behind a skip or a failure and never contains the API key or
// the effort; the other fields are set only when Action is ActionRun.
type IntegrationSettings struct {
	Action IntegrationAction
	Reason string
	APIKey secret.Secret
	Model  string
	Effort openaiparam.Effort
}

// SettingsFrom decides whether the integration test that calls the real
// OpenAI API runs, reading the environment through lookup (the signature of
// os.LookupEnv). The checks run in a fixed order: an opt-in that is not
// exactly OptInValue skips (so a plain `go test -tags integration` or an IDE
// run never incurs a charge), a missing test API key skips, a missing model
// name or a missing or unknown effort fails, and an HTTP/2 debug setting that
// would print the API key fails.
func SettingsFrom(lookup func(string) (string, bool), opts IntegrationOptions) IntegrationSettings {
	getenv := func(name string) string {
		value, _ := lookup(name)
		return value
	}
	fail := func(reason string) IntegrationSettings {
		return IntegrationSettings{Action: ActionFail, Reason: reason}
	}

	if getenv(opts.OptInEnv) != OptInValue {
		return IntegrationSettings{
			Action: ActionSkip,
			Reason: opts.OptInEnv + " is not " + OptInValue +
				": the integration test calls the real OpenAI API and incurs charges; run it with `make " + opts.MakeTarget + "`",
		}
	}
	key := getenv(APIKeyEnv)
	if key == "" {
		return IntegrationSettings{
			Action: ActionSkip,
			Reason: APIKeyEnv + " is not set: set it to an OpenAI API key for testing (OPENAI_API_KEY is not used)",
		}
	}
	model := getenv(ModelEnv)
	if model == "" {
		return fail(ModelEnv + " is not set: set it to the model name, or run `make " + opts.MakeTarget + "`")
	}
	// The empty check precedes the parser, which also rejects an empty value,
	// so an empty variable gets its own reason.
	effortValue := getenv(EffortEnv)
	if effortValue == "" {
		return fail(EffortEnv + " is not set: set it to the effort, or run `make " + opts.MakeTarget + "`")
	}
	effort, err := openaiparam.ParseEffort(effortValue)
	if err != nil {
		return fail(EffortEnv + " is not a supported effort value")
	}
	if config.HTTP2DebugEnabledIn(getenv(GODEBUGEnv)) {
		return fail(GODEBUGEnv + " contains http2debug=1 or http2debug=2, which would print the API key to standard error: unset it before running the integration test")
	}
	apiKey, err := secret.New(key)
	if err != nil {
		// Unreachable: secret.New rejects only the empty value, which is
		// handled above. Kept so an error is never dropped.
		return fail(APIKeyEnv + " cannot be held as a secret")
	}
	return IntegrationSettings{
		Action: ActionRun,
		APIKey: apiKey,
		Model:  model,
		Effort: effort,
	}
}
