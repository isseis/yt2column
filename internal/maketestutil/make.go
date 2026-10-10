//go:build test

// Package maketestutil checks make targets that run charged integration tests.
// A test hands a target to these helpers, which run it with GOTEST replaced by
// a stub, so the target is exercised without running go test, calling an API,
// or touching the network. The provider-specific variable names, defaults, and
// notices are the caller's; the stub, the child environment, and the checks
// against the go test command line live here.
package maketestutil

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
	// errNoVars reports a ChargedTarget that names no variable.
	errNoVars = errors.New("ChargedTarget.Vars must name at least one variable")
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

// RecordedNames returns base followed by the names in extra that it does not
// already hold, each once. Every name must be a plain shell variable name,
// since makeStubScript embeds the names in shell text.
func RecordedNames(base, extra []string) ([]string, error) {
	names := slices.Clone(base)
	for _, name := range extra {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	if err := validateEnvNames(names); err != nil {
		return nil, err
	}
	return names, nil
}

// MakeInvocation is what the stub GOTEST recorded: its arguments, and the
// recorded variables that were set, by name. An unset variable has no entry,
// so unset and empty differ.
type MakeInvocation struct {
	Args []string
	Env  map[string]string
}

// RunTarget runs `make -s <target>` in root with GOTEST replaced by a stub, and
// returns make's output and what the stub recorded. record names the variables
// the stub records, as base followed by the names in extra that base does not
// already hold. set gives the values the child environment receives, in
// addition to the fixed allowlist; an entry whose value is empty passes an
// empty but defined variable, which a target's `?=` keeps. Every recorded name
// must be a plain shell variable name.
func RunTarget(t *testing.T, root, target string, base, extra []string, set map[string]string) (string, MakeInvocation) {
	t.Helper()
	record, err := RecordedNames(base, extra)
	if err != nil {
		t.Fatal(err)
	}
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "gotest-stub")
	if err := os.WriteFile(stub, []byte(makeStubScript(record)), 0o700); err != nil { //nolint:gosec // an executable stub inside the test's temporary directory
		t.Fatalf("write stub: %v", err)
	}

	env := []string{}
	for _, name := range childEnvAllowlist {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	for name, value := range set {
		env = append(env, name+"="+value)
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

// TargetVar is one variable of a charged make target that the target gives a
// default value to.
type TargetVar struct {
	// Env is the variable's name.
	Env string
	// Default is the value the target exports when Env is undefined.
	Default string
	// Custom is the value a check gives Env when it is set.
	Custom string
}

// ChargedTarget describes a make target that runs a charged integration test.
type ChargedTarget struct {
	// Root is the directory holding the Makefile.
	Root string
	// Target is the make target.
	Target string
	// Package is the package path the target passes to go test.
	Package string
	// MinTimeout is what the go test -timeout value must exceed, so the test
	// binary does not time out before the calls it makes can.
	MinTimeout time.Duration
	// OptInEnv is the opt-in variable the target must export.
	OptInEnv string
	// OptInValue is the only opt-in value that arms the target.
	OptInValue string
	// ChargeNotice is a substring of make's output that says the target calls
	// the real API and incurs charges.
	ChargeNotice string
	// Vars are the variables the target gives default values to.
	Vars []TargetVar
	// Record is the base list of variable names the stub records, before
	// OptInEnv and Vars are added, so a target that exports the wrong
	// variable is visible.
	Record []string
}

// validateChargedTarget rejects a ChargedTarget that names no variable. Every
// check runs inside the Vars loop, so without this a target with no Vars would
// be checked by running nothing and passing vacuously.
func validateChargedTarget(c ChargedTarget) error {
	if len(c.Vars) == 0 {
		return errNoVars
	}
	return nil
}

// CheckChargedTarget runs c.Target under the stub GOTEST with each of c.Vars
// undefined, empty, and set, and checks the charge notice, the go test
// arguments, the -timeout bound, the opt-in, and every variable's value. The
// package path must be the final, standalone argument, so go test treats it
// as the package and not as the value of a flag such as -run.
func CheckChargedTarget(t *testing.T, c ChargedTarget) {
	t.Helper()
	if err := validateChargedTarget(c); err != nil {
		t.Fatal(err)
	}
	const timeoutPlaceholder = "<timeout>"
	wantArgs := []string{"-tags", "integration", "-count=1", "-timeout", timeoutPlaceholder, "-v", c.Package}
	extra := []string{c.OptInEnv}
	for _, v := range c.Vars {
		extra = append(extra, v.Env)
	}
	for _, v := range c.Vars {
		empty := ""
		for _, tc := range []struct {
			name  string
			value *string
			want  string
		}{
			{name: "undefined_uses_default", value: nil, want: v.Default},
			{name: "empty_is_kept", value: &empty, want: ""},
			{name: "value_is_kept", value: &v.Custom, want: v.Custom},
		} {
			t.Run(v.Env+"/"+tc.name, func(t *testing.T) {
				set := map[string]string{}
				if tc.value != nil {
					set[v.Env] = *tc.value
				}
				output, invocation := RunTarget(t, c.Root, c.Target, c.Record, extra, set)
				if !strings.Contains(output, c.ChargeNotice) {
					t.Errorf("make output %q does not contain %q", output, c.ChargeNotice)
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
				if got, ok := invocation.Env[c.OptInEnv]; !ok || got != c.OptInValue {
					t.Errorf("%s = %q (set %t), want %q", c.OptInEnv, got, ok, c.OptInValue)
				}
				for _, w := range c.Vars {
					want := w.Default
					if w.Env == v.Env {
						want = tc.want
					}
					if got, ok := invocation.Env[w.Env]; !ok || got != want {
						t.Errorf("%s = %q (set %t), want %q", w.Env, got, ok, want)
					}
				}
			})
		}
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
