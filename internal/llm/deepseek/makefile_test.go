//go:build test

package deepseek

import (
	"testing"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
)

// repositoryRoot is where the Makefile lives, relative to this package.
const repositoryRoot = "../../.."

// TestMakeTestIntegrationDeepSeek checks the target that runs the adapter's
// integration test. The -timeout value must exceed integrationGenerateCalls
// Generate calls of integrationGenerateTimeout each.
func TestMakeTestIntegrationDeepSeek(t *testing.T) {
	deepseektestutil.CheckChargedTarget(t, deepseektestutil.ChargedTarget{
		Root:       repositoryRoot,
		Target:     integrationOptions.MakeTarget,
		OptInEnv:   integrationOptions.OptInEnv,
		Package:    "./internal/llm/deepseek",
		MinTimeout: integrationGenerateCalls * integrationGenerateTimeout,
	})
}

// TestMakeOptInExportedToDeepSeekTargetOnly pins that the opt-ins are
// target-specific exports: another target that runs GOTEST must see neither,
// or a global export would arm a charged test for every recipe.
func TestMakeOptInExportedToDeepSeekTargetOnly(t *testing.T) {
	_, invocation := deepseektestutil.RunMakeTarget(t, repositoryRoot, "test-integration", nil)
	for _, optIn := range []string{deepseektestutil.DeepSeekOptInEnv, deepseektestutil.CLIOptInEnv} {
		if value, ok := invocation.Env[optIn]; ok {
			t.Errorf("make test-integration exported %s=%q; each opt-in must be exported to its own target only", optIn, value)
		}
	}
}
