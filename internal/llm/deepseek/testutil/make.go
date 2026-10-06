//go:build test

package deepseektestutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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

// MakeRun describes one `make` invocation under a stub GOTEST.
type MakeRun struct {
	// Root is the directory holding the Makefile.
	Root string
	// Target is the make target to run.
	Target string
	// RecordEnv names the variables the stub records when they are set.
	RecordEnv []string
	// ModelEnv names the model variable that Model sets.
	ModelEnv string
	// Model is the ModelEnv entry of the child environment; nil leaves the
	// variable undefined.
	Model *string
}

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

// RunMakeTarget runs `make -s <Target>` in Root with GOTEST replaced by a stub,
// and returns make's output and what the stub recorded.
func RunMakeTarget(t *testing.T, r MakeRun) (string, MakeInvocation) {
	t.Helper()
	if err := validateEnvNames(append([]string{r.ModelEnv}, r.RecordEnv...)); err != nil {
		t.Fatal(err)
	}
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "gotest-stub")
	if err := os.WriteFile(stub, []byte(makeStubScript(r.RecordEnv)), 0o700); err != nil { //nolint:gosec // an executable stub inside the test's temporary directory
		t.Fatalf("write stub: %v", err)
	}

	env := []string{}
	for _, name := range childEnvAllowlist {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	if r.Model != nil {
		env = append(env, r.ModelEnv+"="+*r.Model)
	}
	cmd := exec.Command(makePath, "-s", r.Target, "GOTEST="+stub) //nolint:gosec // make from PATH with a test-chosen target
	cmd.Dir = r.Root
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make %s error = %v, output:\n%s", r.Target, err, output)
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
