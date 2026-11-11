//go:build test || integration

package openai

import (
	"time"

	openaitestutil "github.com/isseis/yt2column/internal/llm/openai/testutil"
)

// This file is built both by the unit tests (`-tags test`), which check the
// make target against these values, and by the integration test
// (`-tags integration`), which uses them. It holds no test function, so the
// integration build runs TestIntegrationResponses alone.

// integrationOptions is how TestIntegrationResponses decides whether it runs.
var integrationOptions = openaitestutil.IntegrationOptions{
	OptInEnv:   openaitestutil.OptInEnv,
	MakeTarget: "test-integration-openai",
}

const (
	// integrationGenerateTimeout bounds one real Generate call. Output capped
	// at integrationGenerateMaxTokens takes well under a minute at the speed
	// observed when the default output limit was measured, so this leaves
	// wide room for a congested API.
	integrationGenerateTimeout = 5 * time.Minute

	// integrationGenerateCalls is how many Generate calls
	// TestIntegrationResponses makes. The -timeout that
	// `make test-integration-openai` passes must exceed
	// integrationGenerateCalls x integrationGenerateTimeout, so the test
	// binary never times out before a Generate deadline does;
	// TestMakeIntegrationOpenAI checks it.
	integrationGenerateCalls = 2

	// integrationGenerateMaxTokens caps the ordinary generation, reasoning
	// included, so a model or effort the user overrides still produces at
	// most this much output.
	integrationGenerateMaxTokens = 2048

	// integrationTruncationMaxTokens is small enough that the generation
	// cannot finish, so the response ends incomplete with the reason
	// "max_output_tokens".
	integrationTruncationMaxTokens = 16
)

// integrationClientOptions builds the adapter options from the settings of a
// run. It is built under both tags so a unit test can pin every field the
// integration build depends on.
func integrationClientOptions(settings openaitestutil.IntegrationSettings) Options {
	return Options{
		APIKey:  settings.APIKey,
		Model:   settings.Model,
		Effort:  settings.Effort,
		Timeout: integrationGenerateTimeout,
	}
}
