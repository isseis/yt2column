//go:build test || integration

package deepseek

import (
	"time"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
)

// This file is built both by the unit tests (`-tags test`), which check the
// make target against these values, and by the integration test
// (`-tags integration`), which uses them. It holds no test function, so the
// integration build runs TestIntegrationGenerate alone.

// integrationOptions is how TestIntegrationGenerate decides whether it runs.
// A missing test API key skips, as the adapter's integration test always has.
var integrationOptions = deepseektestutil.IntegrationOptions{
	OptInEnv:   deepseektestutil.DeepSeekOptInEnv,
	MakeTarget: "test-integration-deepseek",
	MissingKey: deepseektestutil.MissingKeySkip,
}

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
