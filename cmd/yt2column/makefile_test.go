//go:build test

package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/llm/provider"
)

const (
	// repositoryRoot is where the Makefile lives, relative to this package.
	repositoryRoot = "../.."

	// cliIntegrationTarget and deepSeekIntegrationTarget are the make targets
	// that run the charged integration tests.
	cliIntegrationTarget      = "test-integration-cli"
	deepSeekIntegrationTarget = "test-integration-deepseek"
)

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

func TestMakeTestIntegrationCLI(t *testing.T) {
	// The package path is the final, standalone argument, so go test treats
	// it as the package and not as the value of a flag such as -run. The
	// -timeout value is checked separately against the LLM timeout: the test
	// makes one Generate call, so the test binary must not time out first.
	const timeoutPlaceholder = "<timeout>"
	wantArgs := []string{"-tags", "integration", "-count=1", "-timeout", timeoutPlaceholder, "-v", "./cmd/yt2column"}
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
			output, invocation := runMake(t, cliIntegrationTarget, tc.model)
			if !strings.Contains(output, "calls the real DeepSeek API, which incurs charges") {
				t.Errorf("make output %q does not say that the target calls the real API and incurs charges", output)
			}
			args := slices.Clone(invocation.Args)
			if i := slices.Index(args, "-timeout"); i >= 0 && i+1 < len(args) {
				timeout, err := time.ParseDuration(args[i+1])
				if err != nil || timeout <= provider.LLMTimeout {
					t.Errorf("-timeout %q (parse error %v), want a duration above provider.LLMTimeout %s", args[i+1], err, provider.LLMTimeout)
				}
				args[i+1] = timeoutPlaceholder
			}
			if !slices.Equal(args, wantArgs) {
				t.Errorf("GOTEST arguments = %q, want %q", invocation.Args, wantArgs)
			}
			optIn := cliIntegrationOptions.OptInEnv
			if got, ok := invocation.Env[optIn]; !ok || got != deepseektestutil.OptInValue {
				t.Errorf("%s = %q (set %t), want %q", optIn, got, ok, deepseektestutil.OptInValue)
			}
			if got, ok := invocation.Env[deepseektestutil.ModelEnv]; !ok || got != tc.wantModel {
				t.Errorf("%s = %q (set %t), want %q", deepseektestutil.ModelEnv, got, ok, tc.wantModel)
			}
		})
	}
}

// TestMakeOptInsAreTargetSpecific pins that each charged target exports only
// its own opt-in, so running one never arms the other's test.
func TestMakeOptInsAreTargetSpecific(t *testing.T) {
	for _, tc := range []struct {
		target    string
		wantUnset string
	}{
		{target: cliIntegrationTarget, wantUnset: deepseektestutil.DeepSeekOptInEnv},
		{target: deepSeekIntegrationTarget, wantUnset: deepseektestutil.CLIOptInEnv},
	} {
		t.Run(tc.target, func(t *testing.T) {
			_, invocation := runMake(t, tc.target, nil)
			if value, ok := invocation.Env[tc.wantUnset]; ok {
				t.Errorf("make %s exported %s=%q", tc.target, tc.wantUnset, value)
			}
		})
	}
}

// gateRecorder is a testing.TB that records Skip and Fatal instead of ending
// the test, so a test can observe what gateCLIIntegration decided and that it
// did not go on to call the body.
type gateRecorder struct {
	testing.TB
	skipped []string
	failed  []string
}

// Skip records the reason instead of skipping.
func (r *gateRecorder) Skip(args ...any) {
	r.skipped = append(r.skipped, fmt.Sprint(args...))
}

// Fatal records the reason instead of failing.
func (r *gateRecorder) Fatal(args ...any) {
	r.failed = append(r.failed, fmt.Sprint(args...))
}

// TestCLIIntegrationSettings checks the decision the CLI integration test
// makes in each environment: it runs only with its own opt-in set to exactly
// 1, fails rather than skips without the test API key even when the
// production key is set, and fails on an HTTP/2 debug setting. When it does
// not run, the body that calls run and the LLM client is never reached, and
// the reason holds neither key nor the last eight characters of either.
func TestCLIIntegrationSettings(t *testing.T) {
	const (
		testKey       = "sk-integration-TESTKEYVALUE-0123456789-TAIL8TST"
		productionKey = testAPIKey
		model         = "deepseek-custom"
	)
	// The environment names the CLI opt-in directly, not through
	// cliIntegrationOptions, so a wrong opt-in in the options is caught.
	complete := map[string]string{
		deepseektestutil.CLIOptInEnv: deepseektestutil.OptInValue,
		deepseektestutil.APIKeyEnv:   testKey,
		deepseektestutil.ModelEnv:    model,
		"DEEPSEEK_API_KEY":           productionKey,
	}
	keys := []string{testKey, testKey[len(testKey)-8:], productionKey, productionKey[len(productionKey)-8:]}

	// outcomes counts what the gate did: called the body, skipped, failed.
	type outcomes struct{ ran, skipped, failed int }
	var (
		ran     = outcomes{ran: 1}
		skipped = outcomes{skipped: 1}
		failed  = outcomes{failed: 1}
	)
	for _, tc := range []struct {
		name   string
		change map[string]string
		want   outcomes
	}{
		{name: "opt_in_unset", change: map[string]string{deepseektestutil.CLIOptInEnv: ""}, want: skipped},
		{name: "opt_in_zero", change: map[string]string{deepseektestutil.CLIOptInEnv: "0"}, want: skipped},
		{name: "opt_in_true", change: map[string]string{deepseektestutil.CLIOptInEnv: "true"}, want: skipped},
		{name: "opt_in_padded", change: map[string]string{deepseektestutil.CLIOptInEnv: " 1"}, want: skipped},
		{name: "deepseek_opt_in_only", change: map[string]string{deepseektestutil.CLIOptInEnv: "", deepseektestutil.DeepSeekOptInEnv: deepseektestutil.OptInValue}, want: skipped},
		{name: "test_key_missing_with_production_key_set", change: map[string]string{deepseektestutil.APIKeyEnv: ""}, want: failed},
		{name: "godebug_http2debug_1", change: map[string]string{deepseektestutil.GODEBUGEnv: "http2debug=1"}, want: failed},
		{name: "godebug_http2debug_2_among_others", change: map[string]string{deepseektestutil.GODEBUGEnv: "gctrace=1,http2debug=2"}, want: failed},
		{name: "complete", want: ran},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := maps.Clone(complete)
			maps.Copy(env, tc.change)
			recorder := &gateRecorder{TB: t}
			var bodySettings []deepseektestutil.IntegrationSettings
			gateCLIIntegration(recorder, func(name string) string { return env[name] }, func(s deepseektestutil.IntegrationSettings) {
				bodySettings = append(bodySettings, s)
			})

			got := outcomes{ran: len(bodySettings), skipped: len(recorder.skipped), failed: len(recorder.failed)}
			if got != tc.want {
				t.Fatalf("gate outcomes = %+v, want %+v", got, tc.want)
			}
			for _, reason := range slices.Concat(recorder.skipped, recorder.failed) {
				for _, key := range keys {
					if strings.Contains(reason, key) {
						t.Error("the reason holds an API key or its last eight characters")
					}
				}
			}
			if tc.want != ran {
				return
			}
			if bodySettings[0].Model != model {
				t.Errorf("Model = %q, want %q", bodySettings[0].Model, model)
			}
			if key, err := bodySettings[0].APIKey.Reveal(); err != nil || key != testKey {
				t.Errorf("APIKey is not the value of %s (Reveal error = %v)", deepseektestutil.APIKeyEnv, err)
			}
		})
	}
}

// TestCLIIntegrationTestBuildTag pins the build tag that keeps the CLI
// integration test, which calls the real DeepSeek API, out of `make test`
// while `make lint` still compiles it through `go vet -tags integration`.
func TestCLIIntegrationTestBuildTag(t *testing.T) {
	if err := deepseektestutil.FirstLineIs("integration_test.go", "//go:build integration"); err != nil {
		t.Errorf("integration_test.go must start with %q: %v", "//go:build integration", err)
	}
}
