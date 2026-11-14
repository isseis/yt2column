//go:build test

package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestMain points PATH at an empty directory before any test runs. The tests
// run inside a Herdr pane, where a real herdr, gh, and git exist; every test
// passes run a fake runner factory, so a command started outside it by
// mistake finds nothing instead of reaching the real tools. No test here runs
// a program on purpose.
func TestMain(m *testing.M) {
	code := 1
	empty, err := os.MkdirTemp("", "chief-cmd-path-")
	if err == nil {
		err = isolatePath(empty)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "chief command tests: %v\n", err)
	} else {
		code = m.Run()
	}
	if empty != "" {
		_ = os.RemoveAll(empty)
	}
	os.Exit(code)
}

// isolatePath sets PATH to dir and fails when herdr, gh, or git is still
// found.
func isolatePath(dir string) error {
	if err := os.Setenv("PATH", dir); err != nil {
		return err
	}
	for _, name := range []string{"herdr", "gh", "git"} {
		if path, err := exec.LookPath(name); err == nil {
			return fmt.Errorf("a real tool is still found on PATH: %s at %s", name, path)
		}
	}
	return nil
}
