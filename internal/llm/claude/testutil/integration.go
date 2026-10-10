//go:build test || integration

// Package claudetestutil holds what the integration test that calls the real
// Anthropic Messages API needs: the decision whether it runs. It is built both
// by the unit tests (`-tags test`), which test the decision, and by the
// integration test (`-tags integration`), which uses it.
package claudetestutil

import (
	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/secret"
)

// Environment variables read by the integration test. The production
// ANTHROPIC_API_KEY and ANTHROPIC_WORKSPACE_ID are never read: the test uses
// its own variables.
const (
	// OptInEnv opts in to the Claude adapter's integration test;
	// `make test-integration-claude` exports it.
	OptInEnv       = "YT2COLUMN_CLAUDE_INTEGRATION"
	APIKeyEnv      = "YT2COLUMN_TEST_ANTHROPIC_API_KEY" //nolint:gosec // the name of the variable holding the test key, not a key
	ModelEnv       = "YT2COLUMN_TEST_CLAUDE_MODEL"
	EffortEnv      = "YT2COLUMN_TEST_CLAUDE_EFFORT"
	WorkspaceIDEnv = "YT2COLUMN_TEST_ANTHROPIC_WORKSPACE_ID"
	GODEBUGEnv     = "GODEBUG"

	// OptInValue is the only opt-in value that runs the test.
	OptInValue = "1"
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
// the variable behind a skip or a failure and never contains the API key, the
// effort, or the workspace ID; the other fields are set only when Action is
// ActionRun. The zero WorkspaceID means none was specified.
type IntegrationSettings struct {
	Action      IntegrationAction
	Reason      string
	APIKey      secret.Secret
	Model       string
	Effort      claudeparam.Effort
	WorkspaceID claudeparam.WorkspaceID
}

// SettingsFrom decides whether the integration test that calls the real
// Anthropic API runs, reading the environment through lookup (the signature
// of os.LookupEnv). The checks run in a fixed order: an opt-in that is not
// exactly OptInValue skips (so a plain `go test -tags integration` or an IDE
// run never incurs a charge), a missing test API key skips, a missing model
// name or a missing or unknown effort fails, an empty or malformed test
// workspace ID fails while an unset one specifies none, and an HTTP/2 debug
// setting that would print the API key fails.
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
				": the integration test calls the real Anthropic API and incurs charges; run it with `make " + opts.MakeTarget + "`",
		}
	}
	key := getenv(APIKeyEnv)
	if key == "" {
		return IntegrationSettings{
			Action: ActionSkip,
			Reason: APIKeyEnv + " is not set: set it to an Anthropic API key for testing (ANTHROPIC_API_KEY is not used)",
		}
	}
	model := getenv(ModelEnv)
	if model == "" {
		return fail(ModelEnv + " is not set: set it to the model name, or run `make " + opts.MakeTarget + "`")
	}
	// The empty checks precede the parsers, which also reject an empty value,
	// so an empty variable gets its own reason.
	effortValue := getenv(EffortEnv)
	if effortValue == "" {
		return fail(EffortEnv + " is not set: set it to the effort, or run `make " + opts.MakeTarget + "`")
	}
	effort, err := claudeparam.ParseEffort(effortValue)
	if err != nil {
		return fail(EffortEnv + " is not a supported effort value")
	}
	var workspaceID claudeparam.WorkspaceID
	if value, ok := lookup(WorkspaceIDEnv); ok {
		if value == "" {
			return fail(WorkspaceIDEnv + " is set but empty: unset it to specify no workspace ID (ANTHROPIC_WORKSPACE_ID is not used)")
		}
		workspaceID, err = claudeparam.ParseWorkspaceID(value)
		if err != nil {
			return fail(WorkspaceIDEnv + " is not a valid workspace ID")
		}
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
		Action:      ActionRun,
		APIKey:      apiKey,
		Model:       model,
		Effort:      effort,
		WorkspaceID: workspaceID,
	}
}
