//go:build test || integration

package claude

import (
	"time"

	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
)

// This file is built both by the unit tests (`-tags test`), which check the
// make target against these values, and by the integration test
// (`-tags integration`), which uses them. It holds no test function, so the
// integration build runs TestIntegrationGenerate alone.

// integrationOptions is how TestIntegrationGenerate decides whether it runs.
var integrationOptions = claudetestutil.IntegrationOptions{
	OptInEnv:   claudetestutil.OptInEnv,
	MakeTarget: "test-integration-claude",
}

const (
	// integrationGenerateTimeout bounds one real Generate call. Output capped
	// at integrationGenerateMaxTokens takes seconds at the observed speed, so
	// this leaves wide room for a congested API.
	integrationGenerateTimeout = 5 * time.Minute

	// integrationGenerateCalls is how many Generate calls
	// TestIntegrationGenerate makes. The -timeout that
	// `make test-integration-claude` passes must exceed
	// integrationGenerateCalls x integrationGenerateTimeout, so the test
	// binary never times out before a Generate deadline does;
	// TestMakeTestIntegrationClaude checks it.
	integrationGenerateCalls = 2

	// integrationGenerateMaxTokens caps the ordinary generation, so a model
	// or effort the user overrides still produces at most this much output.
	integrationGenerateMaxTokens = 1024

	// integrationTruncationMaxTokens is small enough that the generation
	// cannot finish, so the response ends with stop_reason "max_tokens".
	integrationTruncationMaxTokens = 16
)
