//go:build test || integration

package deepseek

import (
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/secret"
)

// This file is built both by the unit tests (`-tags test`), which test
// integrationSettingsFrom, and by the integration test (`-tags integration`),
// which uses it. It holds no test function, so the integration build runs
// TestIntegrationGenerate alone.

// Environment variables read by the integration test. The production
// DEEPSEEK_API_KEY is never read: the test uses its own key variable.
const (
	integrationOptInEnv  = "YT2COLUMN_DEEPSEEK_INTEGRATION"
	integrationAPIKeyEnv = "YT2COLUMN_TEST_DEEPSEEK_API_KEY"
	integrationModelEnv  = "YT2COLUMN_MODEL"
	godebugEnv           = "GODEBUG"

	// integrationOptInValue is the only opt-in value that runs the test.
	// `make test-integration-deepseek` exports it.
	integrationOptInValue = "1"
)

const (
	// integrationGenerateTimeout bounds one real Generate call. The API can
	// hold a request for up to 10 minutes before inference starts; 5 more
	// minutes cover the generation itself.
	integrationGenerateTimeout = 15 * time.Minute

	// integrationGenerateCalls is how many Generate calls
	// TestIntegrationGenerate makes. The -timeout that
	// `make test-integration-deepseek` passes must exceed
	// integrationGenerateCalls x integrationGenerateTimeout, so the test
	// binary never times out before a Generate deadline does;
	// TestMakeTestIntegrationDeepSeek checks it.
	integrationGenerateCalls = 2
)

// http2VerboseSettings are the GODEBUG settings that make the HTTP/2 transport
// log every request header, Authorization included, to standard error. The
// transport reads GODEBUG once at program start and matches these substrings,
// so removing them from the environment inside the test would come too late;
// the test fails instead.
var http2VerboseSettings = []string{"http2debug=1", "http2debug=2"}

// integrationAction is what the integration test does with its environment.
// The zero value skips, so a decision that was never made sends no request
// and incurs no charge.
type integrationAction int

const (
	integrationSkip integrationAction = iota
	integrationFail
	integrationRun
)

// integrationSettings is the outcome of reading the integration test's
// environment. reason names the variable behind a skip or a failure; apiKey
// and model are set only when action is integrationRun.
type integrationSettings struct {
	action integrationAction
	reason string
	apiKey secret.Secret
	model  string
}

// integrationSettingsFrom decides whether the integration test runs. The
// checks run in a fixed order: a missing opt-in skips (so a plain
// `go test -tags integration` or an IDE run never incurs a charge), a missing
// test API key skips, a missing model name fails, and an HTTP/2 debug setting
// that would print the API key fails. Reasons never contain the API key.
func integrationSettingsFrom(getenv func(string) string) integrationSettings {
	if getenv(integrationOptInEnv) != integrationOptInValue {
		return integrationSettings{
			action: integrationSkip,
			reason: integrationOptInEnv + " is not " + integrationOptInValue +
				": the integration test calls the real DeepSeek API and incurs charges; run it with `make test-integration-deepseek`",
		}
	}
	key := getenv(integrationAPIKeyEnv)
	if key == "" {
		return integrationSettings{
			action: integrationSkip,
			reason: integrationAPIKeyEnv + " is not set: set it to a DeepSeek API key for testing (DEEPSEEK_API_KEY is not used)",
		}
	}
	model := getenv(integrationModelEnv)
	if model == "" {
		return integrationSettings{
			action: integrationFail,
			reason: integrationModelEnv + " is not set: set it to the model name, or run `make test-integration-deepseek`",
		}
	}
	godebug := getenv(godebugEnv)
	for _, setting := range http2VerboseSettings {
		if strings.Contains(godebug, setting) {
			return integrationSettings{
				action: integrationFail,
				reason: godebugEnv + " contains " + setting + ", which would print the API key to standard error: unset it before running the integration test",
			}
		}
	}
	apiKey, err := secret.New(key)
	if err != nil {
		// Unreachable: secret.New rejects only the empty value, which is
		// skipped above. Kept so an error is never dropped.
		return integrationSettings{action: integrationFail, reason: integrationAPIKeyEnv + ": " + err.Error()}
	}
	return integrationSettings{action: integrationRun, apiKey: apiKey, model: model}
}
