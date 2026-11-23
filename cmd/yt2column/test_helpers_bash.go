//go:build test

package main

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// scriptBash is a bash a test runs a command document's bash script with.
type scriptBash struct {
	name, path string
}

// minBashMajorVersion is the oldest bash version scripts using associative
// arrays support.
const minBashMajorVersion = 4

// requireBash4 returns the bash on PATH after confirming it is version 4 or
// later, failing t otherwise. Scripts that use associative arrays require
// bash 4+. Reads BASH_VERSINFO[0] directly so localized --version output
// cannot break version detection.
func requireBash4(t *testing.T) scriptBash {
	t.Helper()
	onPath, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is not on PATH; bash 4 or later is required: %v", err)
	}
	out, err := exec.Command(onPath, "-c", `printf %s "${BASH_VERSINFO[0]}"`).Output() //nolint:gosec // runs the bash resolved from PATH
	if err != nil {
		t.Fatalf("read BASH_VERSINFO[0] (%s): %v", onPath, err)
	}
	major, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("cannot read the version of %s: %q", onPath, string(out))
	}
	if major < minBashMajorVersion {
		t.Fatalf("the bash on PATH (%s) is older than %d: %q", onPath, minBashMajorVersion, string(out))
	}
	return scriptBash{"path-bash", onPath}
}

// scriptBashes returns the bash on PATH and, when /bin/bash is a different
// bash 3.x (macOS), that one too, since a command runs its bash script with
// whichever bash the user's PATH gives and the scripts promise to work with
// bash 3.2. It fails t when bash is missing.
func scriptBashes(t *testing.T) []scriptBash {
	t.Helper()
	onPath, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is not on PATH: %v", err)
	}
	bashes := []scriptBash{{"path-bash", onPath}}
	const system = "/bin/bash"
	out, err := exec.Command(system, "-c", "printf %s \"$BASH_VERSION\"").Output()
	if err != nil || !strings.HasPrefix(string(out), "3.") {
		return bashes
	}
	resolvedPath, errPath := filepath.EvalSymlinks(onPath)
	resolvedSystem, errSystem := filepath.EvalSymlinks(system)
	if errPath == nil && errSystem == nil && resolvedPath == resolvedSystem {
		return bashes
	}
	return append(bashes, scriptBash{"bin-bash-3", system})
}
