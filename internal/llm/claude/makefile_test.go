//go:build test

package claude

import (
	"testing"

	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	"github.com/isseis/yt2column/internal/maketestutil"
)

// repositoryRoot is where the Makefile lives, relative to this package.
const repositoryRoot = "../../.."

// TestMakeTestIntegrationClaude checks the target that runs the adapter's
// integration test: the charge notice, the go test arguments, the opt-in, and
// the model name and effort defaults. The -timeout value must exceed
// integrationGenerateCalls Generate calls of integrationGenerateTimeout each.
// The opt-in is named directly, not through integrationOptions, so a target
// that exports a different opt-in is caught.
func TestMakeTestIntegrationClaude(t *testing.T) {
	maketestutil.CheckChargedTarget(t, maketestutil.ChargedTarget{
		Root:         repositoryRoot,
		Target:       integrationOptions.MakeTarget,
		Package:      "./internal/llm/claude",
		MinTimeout:   integrationGenerateCalls * integrationGenerateTimeout,
		OptInEnv:     claudetestutil.OptInEnv,
		OptInValue:   claudetestutil.OptInValue,
		ChargeNotice: "calls the real Anthropic API, which incurs charges",
		Vars: []maketestutil.TargetVar{
			{Env: claudetestutil.ModelEnv, Default: "claude-haiku-5-5", Custom: "claude-custom"},
			{Env: claudetestutil.EffortEnv, Default: "low", Custom: "high"},
		},
	})
}
