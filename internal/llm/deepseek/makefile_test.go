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
