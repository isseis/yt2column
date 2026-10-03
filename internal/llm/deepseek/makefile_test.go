//go:build test

package deepseek

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	// repositoryRoot is where the Makefile lives, relative to this package.
	repositoryRoot = "../../.."

	// makeStubScript records its arguments and the environment variables the
	// target is expected to export, one per line, next to itself. It never
	// runs go test, so the target is exercised without the API or the
	// network. An unset variable writes no line, so unset and empty differ.
	makeStubScript = `#!/bin/sh
out="$(dirname "$0")/invocation"
{
	for arg in "$@"; do
		printf 'arg %s\n' "$arg"
	done
	if [ "${YT2COLUMN_DEEPSEEK_INTEGRATION+set}" = set ]; then
		printf 'env YT2COLUMN_DEEPSEEK_INTEGRATION=%s\n' "$YT2COLUMN_DEEPSEEK_INTEGRATION"
	fi
	if [ "${YT2COLUMN_MODEL+set}" = set ]; then
		printf 'env YT2COLUMN_MODEL=%s\n' "$YT2COLUMN_MODEL"
	fi
} > "$out"
`
)

// makeChildEnvAllowlist is the environment the make child receives. It is an
// allowlist, so nothing from the outer `make test` leaks in: MAKEFLAGS,
// MFLAGS, and MAKELEVEL would carry the outer command-line variables, and the
// API key variables must never reach a child. The proxy variables TestMain
// installed are kept so an accidental real request still goes nowhere.
var makeChildEnvAllowlist = []string{
	"PATH", "HOME", "TMPDIR",
	"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy",
}

// makeInvocation is what the stub recorded.
type makeInvocation struct {
	args []string
	env  map[string]string
}

// runMakeTestIntegrationDeepSeek runs `make -s test-integration-deepseek` at
// the repository root with GOTEST replaced by the stub, and returns make's
// output and what the stub recorded. model is the YT2COLUMN_MODEL entry of
// the child environment; nil leaves the variable undefined.
func runMakeTestIntegrationDeepSeek(t *testing.T, model *string) (string, makeInvocation) {
	t.Helper()
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "gotest-stub")
	if err := os.WriteFile(stub, []byte(makeStubScript), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	env := []string{}
	for _, name := range makeChildEnvAllowlist {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	if model != nil {
		env = append(env, integrationModelEnv+"="+*model)
	}
	cmd := exec.Command(makePath, "-s", "test-integration-deepseek", "GOTEST="+stub)
	cmd.Dir = repositoryRoot
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make test-integration-deepseek error = %v, output:\n%s", err, output)
	}

	recorded, err := os.ReadFile(filepath.Join(dir, "invocation"))
	if err != nil {
		t.Fatalf("the stub given as GOTEST was not run: %v", err)
	}
	invocation := makeInvocation{env: map[string]string{}}
	for line := range strings.Lines(string(recorded)) {
		line = strings.TrimSuffix(line, "\n")
		if arg, ok := strings.CutPrefix(line, "arg "); ok {
			invocation.args = append(invocation.args, arg)
			continue
		}
		if entry, ok := strings.CutPrefix(line, "env "); ok {
			name, value, _ := strings.Cut(entry, "=")
			invocation.env[name] = value
			continue
		}
		t.Fatalf("unexpected stub output line %q", line)
	}
	return string(output), invocation
}

func TestMakeTestIntegrationDeepSeek(t *testing.T) {
	// The package path is the final, standalone argument, so go test treats
	// it as the package and not as the value of a flag such as -run.
	wantArgs := []string{"-tags", "integration", "-count=1", "-timeout", "40m", "-v", "./internal/llm/deepseek"}
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
			output, invocation := runMakeTestIntegrationDeepSeek(t, tc.model)
			if !strings.Contains(output, "calls the real DeepSeek API, which incurs charges") {
				t.Errorf("make output %q does not say that the target calls the real API and incurs charges", output)
			}
			if !slices.Equal(invocation.args, wantArgs) {
				t.Errorf("GOTEST arguments = %q, want %q", invocation.args, wantArgs)
			}
			if got, ok := invocation.env[integrationOptInEnv]; !ok || got != integrationOptInValue {
				t.Errorf("%s = %q (set %t), want %q", integrationOptInEnv, got, ok, integrationOptInValue)
			}
			if got, ok := invocation.env[integrationModelEnv]; !ok || got != tc.wantModel {
				t.Errorf("%s = %q (set %t), want %q", integrationModelEnv, got, ok, tc.wantModel)
			}
		})
	}
}
