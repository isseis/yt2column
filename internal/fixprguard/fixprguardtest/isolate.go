//go:build test

package fixprguardtest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// gitNotARepository is the exit code git returns when no repository
	// contains the directory. It is also returned for a repository git does
	// not trust and for a broken configuration; a leaked git run with the
	// same environment is refused the same way, so none of them is unsafe.
	gitNotARepository = 128
	dirMode           = 0o700
)

var (
	errInsideRepository = errors.New("a git repository contains the test temporary directory: " +
		"point TMPDIR (and GOTMPDIR) at a directory outside every git repository")
	errUnexpectedGitExit = errors.New("git rev-parse in the test temporary directory exited unexpectedly")
	errWorkdirNotEmpty   = errors.New("a test wrote to the process working directory: " +
		"join the name with the directory the test was given")
)

// Isolation is what IsolateWorkingDir set up for one test binary.
type Isolation struct {
	// PackageDir is the directory the test binary started in: the package
	// directory, from which tests read testdata, sources and documents.
	PackageDir string
	// TemplateRoot is where a package's fixture template is built.
	TemplateRoot string
}

// IsolateWorkingDir records the current directory, makes a new empty
// directory, checks with git that no repository contains it, and makes it the
// process's working directory. TestMain calls it once before m.Run.
//
// Why the process's working directory changes at all: an operation under test
// that ignores the directory it was given and relies on the working directory
// would otherwise read and write this repository and push to its origin.
// In an empty directory outside any repository it fails instead. This is the
// only place the tests change the process's working directory.
//
// The returned function checks that the directory is still empty, which finds
// a write made through a relative name, and removes everything it made.
func IsolateWorkingDir() (Isolation, func() error, error) {
	packageDir, err := os.Getwd()
	if err != nil {
		return Isolation{}, nil, fmt.Errorf("read the working directory: %w", err)
	}
	root, err := os.MkdirTemp("", "fixprguardtest-")
	if err != nil {
		return Isolation{}, nil, fmt.Errorf("create the isolation directory: %w", err)
	}
	fail := func(err error) (Isolation, func() error, error) {
		_ = os.RemoveAll(root) //nolint:gosec // a directory this function just made
		return Isolation{}, nil, err
	}
	iso := Isolation{PackageDir: packageDir, TemplateRoot: filepath.Join(root, "template")}
	cwd, home := filepath.Join(root, "cwd"), filepath.Join(root, "home")
	for _, dir := range []string{cwd, home, iso.TemplateRoot} {
		if err := os.Mkdir(dir, dirMode); err != nil {
			return fail(fmt.Errorf("create %s: %w", filepath.Base(dir), err))
		}
	}
	if err := checkOutsideRepository(cwd, home); err != nil {
		return fail(err)
	}
	// t.TempDir lives under GOTMPDIR when it is set, so the directories tests
	// hand out as "no repository here" must be outside a repository too.
	if goTmp := os.Getenv("GOTMPDIR"); goTmp != "" {
		check, err := os.MkdirTemp(goTmp, "fixprguardtest-check-")
		if err != nil {
			return fail(fmt.Errorf("create a directory under GOTMPDIR: %w", err))
		}
		err = checkOutsideRepository(check, home)
		_ = os.RemoveAll(check) //nolint:gosec // a directory this function just made
		if err != nil {
			return fail(err)
		}
	}
	if err := os.Chdir(cwd); err != nil {
		return fail(fmt.Errorf("enter the isolation directory: %w", err))
	}
	finish := func() error {
		// The process stays in the isolation directory: it is the only
		// os.Chdir of the test binary, and removing the directory a process
		// is in works on the systems the tests run on.
		return errors.Join(checkEmpty(cwd), os.RemoveAll(root)) //nolint:gosec // a directory this function made
	}
	return iso, finish, nil
}

// checkOutsideRepository runs git rev-parse in dir with Env(home, dir) and
// succeeds only when git exits with gitNotARepository.
func checkOutsideRepository(dir, home string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("git", "rev-parse", "--git-dir") //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = dir
	cmd.Env = Env(home, dir)
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return errInsideRepository
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if exitErr.ExitCode() == gitNotARepository {
			return nil
		}
		return fmt.Errorf("%w: code %d: %s", errUnexpectedGitExit,
			exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
	}
	return fmt.Errorf("start git to check the test temporary directory: %w", err)
}

// checkEmpty fails when dir holds anything: a test wrote through a relative
// name instead of the directory it was given.
func checkEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read the isolation directory: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return fmt.Errorf("%w (%d entries: %s)", errWorkdirNotEmpty, len(entries), strings.Join(names, ", "))
}

// Run runs the package's tests inside an isolated working directory and
// returns the exit code TestMain passes to os.Exit. set receives the
// Isolation before any test runs. A failure to isolate, and a working
// directory that is not empty afterwards, print one line to standard error and
// give exit code 1; in the first case no test runs.
func Run(m *testing.M, set func(Isolation)) int {
	iso, finish, err := IsolateWorkingDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixprguardtest: "+err.Error())
		return 1
	}
	set(iso)
	code := m.Run()
	if err := finish(); err != nil {
		fmt.Fprintln(os.Stderr, "fixprguardtest: "+strings.ReplaceAll(err.Error(), "\n", "; "))
		return 1
	}
	return code
}
