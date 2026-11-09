//go:build test

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scriptBash is a bash a test runs a command document's bash script with.
type scriptBash struct {
	name, path string
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
