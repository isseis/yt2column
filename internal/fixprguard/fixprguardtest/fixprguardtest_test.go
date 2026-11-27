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

// TestEnvReplacesInheritedValues gives the function an inherited HOME,
// XDG_CONFIG_HOME, and PWD, which only a real replacement can get rid of.
func TestEnvReplacesInheritedValues(t *testing.T) {
	t.Parallel()
	base := []string{"HOME=/inherited", "KEEP=1", "XDG_CONFIG_HOME=/inherited/.config", "PWD=/inherited", "PATH=/bin"}
	got := withTestValues(base, "/h/home", "/w/dir")
	want := []string{"KEEP=1", "PATH=/bin", "HOME=/h/home", "XDG_CONFIG_HOME=/h/home/.config", "PWD=/w/dir"}
	if !slices.Equal(got, want) {
		t.Errorf("env = %q, want %q", got, want)
	}
}

// TestIsolationChangesWorkingDirectory: the working directory is no longer
// the package directory, which PackageDir still names.
func TestIsolationChangesWorkingDirectory(t *testing.T) {
	t.Parallel()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if cwd == isolation.PackageDir {
		t.Fatal("working directory was not changed")
	}
	if _, err := os.Stat(filepath.Join(isolation.PackageDir, "fixprguardtest_test.go")); err != nil {
		t.Errorf("PackageDir does not name the package directory: %v", err)
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
	if err := checkEmpty(dir); !errors.Is(err, errWorkdirNotEmpty) {
		t.Fatalf("checkEmpty(non-empty) = %v, want errWorkdirNotEmpty", err)
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
	cmd.Env = append(slices.DeleteFunc(Env(t.TempDir(), repo), func(e string) bool {
		return strings.HasPrefix(e, "TMPDIR=") || strings.HasPrefix(e, "GOTMPDIR=")
	}), "TMPDIR="+tmp)
	out, err := cmd.CombinedOutput()
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("exit = %v, want exit code 1; output: %s", err, out)
	}
	// The wording is the contract here: exit code 1 alone cannot tell this stop
	// from another failure to start.
	if !strings.Contains(string(out), "git repository") {
		t.Errorf("diagnostic does not name the repository: %s", out)
	}
}
