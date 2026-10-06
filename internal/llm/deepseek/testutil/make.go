//go:build test

package deepseektestutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

var (
	// ErrFirstLineMismatch reports that a file's first line is not the
	// expected one.
	ErrFirstLineMismatch = errors.New("first line mismatch")
	// errEnvName reports a variable name the stub script cannot embed.
	errEnvName = errors.New("variable name is not a plain shell name")
)

// childEnvAllowlist is the environment the make child receives. It is an
// allowlist, so nothing from the outer `make test` leaks in: MAKEFLAGS,
// MFLAGS, and MAKELEVEL would carry the outer command-line variables, and the
// API key variables must never reach a child. The proxy variables a TestMain
// installed are kept so an accidental real request still goes nowhere.
var childEnvAllowlist = []string{
	"PATH", "HOME", "TMPDIR",
	"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy",
}

// envNamePattern is what a recorded variable name must match. The stub
// script embeds the names in shell text, so anything else is rejected rather
// than quoted.
var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// recordedEnv names the variables the stub GOTEST records: both opt-ins, so
// a target that exports the wrong one is visible, and the model name.
var recordedEnv = []string{DeepSeekOptInEnv, CLIOptInEnv, ModelEnv}

// MakeInvocation is what the stub GOTEST recorded: its arguments, and the
// recorded variables that were set, by name. An unset variable has no entry,
// so unset and empty differ.
type MakeInvocation struct {
	Args []string
	Env  map[string]string
}

// validateEnvNames rejects any name that is not a plain shell variable name,
// since makeStubScript embeds the names in shell text.
func validateEnvNames(names []string) error {
	for _, name := range names {
		if !envNamePattern.MatchString(name) {
			return fmt.Errorf("%w: %q", errEnvName, name)
		}
	}
	return nil
}

// makeStubScript returns a script that records its arguments and the named
// variables, one per line, to a file named invocation next to itself. It
// never runs go test, so a target is exercised without the API or the network.
func makeStubScript(names []string) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\nout=\"$(dirname \"$0\")/invocation\"\n{\n")
	b.WriteString("\tfor arg in \"$@\"; do\n\t\tprintf 'arg %s\\n' \"$arg\"\n\tdone\n")
	for _, name := range names {
		fmt.Fprintf(&b, "\tif [ \"${%[1]s+set}\" = set ]; then\n\t\tprintf 'env %[1]s=%%s\\n' \"$%[1]s\"\n\tfi\n", name)
	}
	b.WriteString("} > \"$out\"\n")
	return b.String()
}

// RunMakeTarget runs `make -s <target>` in root with GOTEST replaced by a stub,
// and returns make's output and what the stub recorded. model is the ModelEnv
// entry of the child environment; nil leaves the variable undefined.
func RunMakeTarget(t *testing.T, root, target string, model *string) (string, MakeInvocation) {
	t.Helper()
	if err := validateEnvNames(recordedEnv); err != nil {
		t.Fatal(err)
	}
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "gotest-stub")
	if err := os.WriteFile(stub, []byte(makeStubScript(recordedEnv)), 0o700); err != nil { //nolint:gosec // an executable stub inside the test's temporary directory
		t.Fatalf("write stub: %v", err)
	}

	env := []string{}
	for _, name := range childEnvAllowlist {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	if model != nil {
		env = append(env, ModelEnv+"="+*model)
	}
	cmd := exec.Command(makePath, "-s", target, "GOTEST="+stub) //nolint:gosec // make from PATH with a test-chosen target
	cmd.Dir = root
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make %s error = %v, output:\n%s", target, err, output)
	}

	recorded, err := os.ReadFile(filepath.Join(dir, "invocation")) //nolint:gosec // a path inside the test's temporary directory
	if err != nil {
		t.Fatalf("the stub given as GOTEST was not run: %v", err)
	}
	invocation := MakeInvocation{Env: map[string]string{}}
	for line := range strings.Lines(string(recorded)) {
		line = strings.TrimSuffix(line, "\n")
		if arg, ok := strings.CutPrefix(line, "arg "); ok {
			invocation.Args = append(invocation.Args, arg)
			continue
		}
		if entry, ok := strings.CutPrefix(line, "env "); ok {
			name, value, _ := strings.Cut(entry, "=")
			invocation.Env[name] = value
			continue
		}
		t.Fatalf("unexpected stub output line %q", line)
	}
	return string(output), invocation
}

// ChargedTarget describes a make target that runs a charged integration test.
type ChargedTarget struct {
	// Root is the directory holding the Makefile.
	Root string
	// Target is the make target.
	Target string
	// OptInEnv is the opt-in variable the target must export.
	OptInEnv string
	// Package is the package path the target passes to go test.
	Package string
	// MinTimeout is what the go test -timeout value must exceed, so the test
	// binary does not time out before the calls it makes can.
	MinTimeout time.Duration
}

// CheckChargedTarget runs c.Target under the stub GOTEST with the model
// variable undefined, empty, and set, and checks the charge notice, the go
// test arguments, the -timeout bound, the opt-in, and the model name. The
// package path must be the final, standalone argument, so go test treats it
// as the package and not as the value of a flag such as -run.
func CheckChargedTarget(t *testing.T, c ChargedTarget) {
	t.Helper()
	const timeoutPlaceholder = "<timeout>"
	wantArgs := []string{"-tags", "integration", "-count=1", "-timeout", timeoutPlaceholder, "-v", c.Package}
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
			output, invocation := RunMakeTarget(t, c.Root, c.Target, tc.model)
			if !strings.Contains(output, "calls the real DeepSeek API, which incurs charges") {
				t.Errorf("make output %q does not say that the target calls the real API and incurs charges", output)
			}
			args := slices.Clone(invocation.Args)
			if i := slices.Index(args, "-timeout"); i >= 0 && i+1 < len(args) {
				timeout, err := time.ParseDuration(args[i+1])
				if err != nil || timeout <= c.MinTimeout {
					t.Errorf("-timeout %q (parse error %v), want a duration above %s", args[i+1], err, c.MinTimeout)
				}
				args[i+1] = timeoutPlaceholder
			}
			if !slices.Equal(args, wantArgs) {
				t.Errorf("GOTEST arguments = %q, want %q", invocation.Args, wantArgs)
			}
			if got, ok := invocation.Env[c.OptInEnv]; !ok || got != OptInValue {
				t.Errorf("%s = %q (set %t), want %q", c.OptInEnv, got, ok, OptInValue)
			}
			if got, ok := invocation.Env[ModelEnv]; !ok || got != tc.wantModel {
				t.Errorf("%s = %q (set %t), want %q", ModelEnv, got, ok, tc.wantModel)
			}
		})
	}
}

// FirstLineIs reports an error unless the file at path exists and its first
// line is exactly want. A different first line wraps ErrFirstLineMismatch.
func FirstLineIs(path, want string) error {
	content, err := os.ReadFile(path) //nolint:gosec // a source file a test names
	if err != nil {
		return err
	}
	line, _, _ := strings.Cut(string(content), "\n")
	if line != want {
		return fmt.Errorf("%w: %s: first line is %q, want %q", ErrFirstLineMismatch, path, line, want)
	}
	return nil
}
