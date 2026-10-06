//go:build test

package deepseek

import (
	"slices"
	"strings"
	"testing"
	"time"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
)

// repositoryRoot is where the Makefile lives, relative to this package.
const repositoryRoot = "../../.."

// makeRecordedEnv names the variables the stub GOTEST records: both opt-ins,
// so a target that exports the wrong one is visible, and the model name.
var makeRecordedEnv = []string{
	deepseektestutil.DeepSeekOptInEnv,
	deepseektestutil.CLIOptInEnv,
	deepseektestutil.ModelEnv,
}

// runMake runs target under the stub GOTEST with the model variable set to
// model, or undefined when model is nil.
func runMake(t *testing.T, target string, model *string) (string, deepseektestutil.MakeInvocation) {
	t.Helper()
	return deepseektestutil.RunMakeTarget(t, deepseektestutil.MakeRun{
		Root:      repositoryRoot,
		Target:    target,
		RecordEnv: makeRecordedEnv,
		ModelEnv:  deepseektestutil.ModelEnv,
		Model:     model,
	})
}

func TestMakeTestIntegrationDeepSeek(t *testing.T) {
	// The package path is the final, standalone argument, so go test treats
	// it as the package and not as the value of a flag such as -run. The
	// -timeout value is checked separately against the Generate timeouts.
	const timeoutPlaceholder = "<timeout>"
	wantArgs := []string{"-tags", "integration", "-count=1", "-timeout", timeoutPlaceholder, "-v", "./internal/llm/deepseek"}
	minTimeout := integrationGenerateCalls * integrationGenerateTimeout
	empty := ""
	custom := "deepseek-custom"
	for _, tc := range []struct {
		name      string
		model     *string
		wantModel string
	}{
		{name: "model_undefined_uses_default", model: nil, wantModel: "deepseek-flash"},
		{name: "model_empty_is_kept", model: &empty, wantModel: ""},
		{name: "model_value_is_kept", model: &custom, wantModel: custom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, invocation := runMake(t, integrationOptions.MakeTarget, tc.model)
			if !strings.Contains(output, "calls the real DeepSeek API, which incurs charges") {
				t.Errorf("make output %q does not say that the target calls the real API and incurs charges", output)
			}
			args := slices.Clone(invocation.Args)
			if i := slices.Index(args, "-timeout"); i >= 0 && i+1 < len(args) {
				timeout, err := time.ParseDuration(args[i+1])
				if err != nil || timeout <= minTimeout {
					t.Errorf("-timeout %q (parse error %v), want a duration above %d x %s = %s",
						args[i+1], err, integrationGenerateCalls, integrationGenerateTimeout, minTimeout)
				}
				args[i+1] = timeoutPlaceholder
			}
			if !slices.Equal(args, wantArgs) {
				t.Errorf("GOTEST arguments = %q, want %q", invocation.Args, wantArgs)
			}
			optIn := integrationOptions.OptInEnv
			if got, ok := invocation.Env[optIn]; !ok || got != deepseektestutil.OptInValue {
				t.Errorf("%s = %q (set %t), want %q", optIn, got, ok, deepseektestutil.OptInValue)
			}
			if got, ok := invocation.Env[deepseektestutil.ModelEnv]; !ok || got != tc.wantModel {
				t.Errorf("%s = %q (set %t), want %q", deepseektestutil.ModelEnv, got, ok, tc.wantModel)
			}
		})
	}
}

// TestMakeOptInExportedToDeepSeekTargetOnly pins that the opt-ins are
// target-specific exports: another target that runs GOTEST must see neither,
// or a global export would arm a charged test for every recipe.
func TestMakeOptInExportedToDeepSeekTargetOnly(t *testing.T) {
	_, invocation := runMake(t, "test-integration", nil)
	for _, optIn := range []string{deepseektestutil.DeepSeekOptInEnv, deepseektestutil.CLIOptInEnv} {
		if value, ok := invocation.Env[optIn]; ok {
			t.Errorf("make test-integration exported %s=%q; each opt-in must be exported to its own target only", optIn, value)
		}
	}
}
