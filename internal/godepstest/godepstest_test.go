//go:build test

package godepstest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/isseis/yt2column"

// realRoot returns the module root with the real go command, before a test
// changes PATH.
func realRoot(t *testing.T) string {
	t.Helper()
	root, err := Root()
	if err != nil {
		t.Fatalf("Root() error = %v", err)
	}
	return root
}

// fakeGoOnPath puts a directory holding only a go script with body first on
// PATH, alone, so List runs the script instead of the go command.
func fakeGoOnPath(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte("#!/bin/sh\n"+body), 0o700); err != nil { //nolint:gosec // an executable stub inside the test's temporary directory
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestList(t *testing.T) {
	root := realRoot(t)

	t.Run("lists a package's dependencies", func(t *testing.T) {
		deps, err := List(root, "./internal/writer")
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if err := deps.Require(module+"/internal/llm", "fmt"); err != nil {
			t.Errorf("Require() error = %v, want nil", err)
		}
		if err := deps.Forbid(module+"/internal/server", module+"/cmd/..."); err != nil {
			t.Errorf("Forbid() error = %v, want nil", err)
		}
	})

	failures := []struct {
		name  string
		setup func(t *testing.T)
		pkg   string
	}{
		{"a package that does not exist", func(*testing.T) {}, "./internal/doesnotexist"},
		{"no go on PATH", func(t *testing.T) { t.Setenv("PATH", t.TempDir()) }, "./internal/writer"},
		{"go prints nothing", func(t *testing.T) { fakeGoOnPath(t, "exit 0\n") }, "./internal/writer"},
		{"go prints no package list", func(t *testing.T) { fakeGoOnPath(t, "echo 'not a package list'\n") }, "./internal/writer"},
	}
	for _, tc := range failures {
		t.Run("fails when "+tc.name, func(t *testing.T) {
			tc.setup(t)
			if _, err := List(root, tc.pkg); !errors.Is(err, ErrList) {
				t.Errorf("List() error = %v, want ErrList", err)
			}
		})
	}
}

func TestRequireAndForbid(t *testing.T) {
	deps, err := List(realRoot(t), "./internal/writer")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if err := deps.Require(module + "/internal/server"); !errors.Is(err, ErrMissing) {
		t.Errorf("Require(a package that is not a dependency) error = %v, want ErrMissing", err)
	}
	for _, pattern := range []string{"fmt", module + "/internal/llm", module + "/internal/...", module + "/internal/llm/..."} {
		if err := deps.Forbid(pattern); !errors.Is(err, ErrForbidden) {
			t.Errorf("Forbid(%q) error = %v, want ErrForbidden", pattern, err)
		}
	}
	// A pattern matches at a path boundary only: internal/ll is not a parent
	// of internal/llm.
	if err := deps.Forbid(module + "/internal/ll/..."); err != nil {
		t.Errorf("Forbid(a prefix that is not a path element) error = %v, want nil", err)
	}
}

func TestRoot(t *testing.T) {
	failures := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"no go on PATH", func(t *testing.T) { t.Setenv("PATH", t.TempDir()) }},
		{"go prints nothing", func(t *testing.T) { fakeGoOnPath(t, "exit 0\n") }},
		{"go prints two lines", func(t *testing.T) { fakeGoOnPath(t, "printf '/a\\n/b\\n'\n") }},
		{"go fails", func(t *testing.T) { fakeGoOnPath(t, "exit 1\n") }},
	}
	for _, tc := range failures {
		t.Run("fails when "+tc.name, func(t *testing.T) {
			tc.setup(t)
			if _, err := Root(); !errors.Is(err, ErrList) {
				t.Errorf("Root() error = %v, want ErrList", err)
			}
		})
	}
}

// TestChildEnv checks that the child go command gets the target OS and the
// fixed settings, whatever the process environment holds, and keeps the rest.
func TestChildEnv(t *testing.T) {
	t.Setenv("GOOS", "windows")
	t.Setenv("GOFLAGS", "-tags=test")
	t.Setenv("GOWORK", "/tmp/go.work")
	t.Setenv("GODEPSTEST_KEPT", "kept")
	env := childEnv("linux")
	values := map[string][]string{}
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		values[name] = append(values[name], value)
	}
	want := map[string]string{
		"GOOS": "linux", "GOFLAGS": "", "GOWORK": "off", "GOTOOLCHAIN": "local",
		"GOENV": "off", "GOPROXY": "off", "GODEPSTEST_KEPT": "kept",
	}
	for name, value := range want {
		if got := values[name]; len(got) != 1 || got[0] != value {
			t.Errorf("%s = %q, want exactly [%q]", name, got, value)
		}
	}
}
