//go:build test

package fixprguardtest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEnv(t *testing.T) {
	t.Parallel()
	home, dir := "/h/home", "/w/dir"
	env := Env(home, dir)
	values := map[string][]string{}
	for _, e := range env {
		name, value, _ := strings.Cut(e, "=")
		values[name] = append(values[name], value)
	}
	for name, want := range map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"PWD":             dir,
	} {
		if got := values[name]; !slices.Equal(got, []string{want}) {
			t.Errorf("%s = %q, want exactly %q", name, got, want)
		}
	}
	for name := range values {
		if strings.HasPrefix(name, "GIT_") && name != "GIT_OPTIONAL_LOCKS" && name != "GIT_TERMINAL_PROMPT" {
			t.Errorf("inherited %s reached the environment", name)
		}
	}
}

func TestIsolationIsEmptyDirOutsideRepository(t *testing.T) {
	t.Parallel()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if cwd == isolation.PackageDir {
		t.Fatal("working directory was not changed")
	}
	if err := checkEmpty(cwd); err != nil {
		t.Errorf("the isolation directory is not empty: %v", err)
	}
	if _, err := os.Stat(filepath.Join(isolation.PackageDir, "fixprguardtest_test.go")); err != nil {
		t.Errorf("PackageDir does not name the package directory: %v", err)
	}
	if fi, err := os.Stat(isolation.TemplateRoot); err != nil || !fi.IsDir() {
		t.Errorf("TemplateRoot is not a directory: %v", err)
	}
}

func TestCheckEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := checkEmpty(dir); err != nil {
		t.Fatalf("empty directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leak"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkEmpty(dir); err == nil {
		t.Fatal("a directory holding a file passed")
	}
}

// TestIsolateWorkingDirRefusesRepository starts this test binary with its
// temporary directory inside a git repository: TestMain must stop with exit
// code 1 before any test runs.
func TestIsolateWorkingDirRefusesRepository(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	init := exec.Command("git", "init", "-q", repo) //nolint:gosec // test setup runs git with fixed arguments
	init.Env = Env(t.TempDir(), repo)
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	tmp := filepath.Join(repo, "tmp")
	if err := os.Mkdir(tmp, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$") //nolint:gosec // re-runs this test binary
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, "TMPDIR=") || strings.HasPrefix(e, "GOTMPDIR=")
	}), "TMPDIR="+tmp)
	out, err := cmd.CombinedOutput()
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("exit = %v, want exit code 1; output: %s", err, out)
	}
	if !strings.Contains(string(out), "git repository") {
		t.Errorf("diagnostic does not name the repository: %s", out)
	}
}
