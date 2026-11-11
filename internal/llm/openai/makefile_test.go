//go:build test

package openai

import (
	"fmt"
	"testing"

	openaitestutil "github.com/isseis/yt2column/internal/llm/openai/testutil"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
	"github.com/isseis/yt2column/internal/maketestutil"
)

// repositoryRoot is where the Makefile lives, relative to this package.
const repositoryRoot = "../../.."

// TestMakeIntegrationOpenAI checks the target that runs the adapter's
// integration test: the charge notice, the go test arguments, the opt-in, and
// the model name and effort defaults, and the number of charged calls. The -timeout value must exceed
// integrationGenerateCalls Generate calls of integrationGenerateTimeout each.
// The opt-in is named directly, not through integrationOptions, so a target
// that exports a different opt-in is caught.
func TestMakeIntegrationOpenAI(t *testing.T) {
	maketestutil.CheckChargedTarget(t, maketestutil.ChargedTarget{
		Root:       repositoryRoot,
		Target:     integrationOptions.MakeTarget,
		Package:    "./internal/llm/openai",
		MinTimeout: integrationGenerateCalls * integrationGenerateTimeout,
		OptInEnv:   openaitestutil.OptInEnv,
		OptInValue: openaitestutil.OptInValue,
		// The notice states the number of charged calls, which must match
		// what the integration test makes.
		ChargeNotice: fmt.Sprintf("calls the real OpenAI API, which incurs charges (%d calls;", integrationGenerateCalls),
		Vars: []maketestutil.TargetVar{
			{Env: openaitestutil.ModelEnv, Default: "gpt-6.1-sol", Custom: "gpt-custom"},
			{Env: openaitestutil.EffortEnv, Default: "medium", Custom: "high"},
		},
	})
}

// TestIntegrationTestBuildTag pins the build tags that keep the integration
// test, which calls the real OpenAI API, out of `make test` and
// `make test-ci`, while its settings build under both tags. The guard's own
// error paths are tested with the DeepSeek adapter's identical guard.
func TestIntegrationTestBuildTag(t *testing.T) {
	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "integration_test.go", want: "//go:build integration"},
		{path: "integration_env_test.go", want: "//go:build test || integration"},
	} {
		if err := maketestutil.FirstLineIs(tc.path, tc.want); err != nil {
			t.Errorf("%s must start with %q: %v", tc.path, tc.want, err)
		}
	}
}

// TestIntegrationClientOptions pins that integrationOptions runs the test in
// an environment that names the opt-in directly, so a wrong opt-in in
// integrationOptions is caught, and that the client the integration test
// builds carries every setting the decision read and the per-call
// integration timeout.
func TestIntegrationClientOptions(t *testing.T) {
	env := map[string]string{
		openaitestutil.OptInEnv:  openaitestutil.OptInValue,
		openaitestutil.APIKeyEnv: testAPIKey,
		openaitestutil.ModelEnv:  testModel,
		openaitestutil.EffortEnv: openaiparam.EffortHigh.String(),
	}
	settings := openaitestutil.SettingsFrom(func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}, integrationOptions)
	if settings.Action != openaitestutil.ActionRun {
		t.Fatalf("Action = %d, want ActionRun (reason %q)", settings.Action, settings.Reason)
	}
	options := integrationClientOptions(settings)
	if key, err := options.APIKey.Reveal(); err != nil || key != testAPIKey {
		t.Errorf("APIKey is not the test key (Reveal error = %v)", err)
	}
	if options.Model != testModel {
		t.Errorf("Model = %q, want %q", options.Model, testModel)
	}
	if options.Effort != openaiparam.EffortHigh {
		t.Errorf("Effort = %s, want %s", options.Effort, openaiparam.EffortHigh)
	}
	if options.Timeout != integrationGenerateTimeout {
		t.Errorf("Timeout = %s, want %s", options.Timeout, integrationGenerateTimeout)
	}
}
