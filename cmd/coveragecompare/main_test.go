//go:build test

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/coveragecompare"
)

// childDirEnv, when set, makes the test binary run runMain with the slow
// comparison of TestSignalWaitsForRun instead of the tests. Its value is the
// directory the comparison writes its marker files to.
const childDirEnv = "COVERAGECOMPARE_TEST_CHILD_DIR"

// The marker files the child's comparison writes in childDirEnv's directory.
const (
	startedFile  = "started"
	returnedFile = "returned"
)

// returnDelay is how long the child's comparison takes to return after it
// is canceled: long enough for a second signal to arrive while it runs.
const returnDelay = time.Second

// TestMain runs runMain with the slow comparison when the test binary was
// started as TestSignalWaitsForRun's child, and the tests otherwise.
func TestMain(m *testing.M) {
	if dir, ok := os.LookupEnv(childDirEnv); ok {
		os.Exit(runMain(slowCompare(dir)))
	}
	os.Exit(m.Run())
}

// slowCompare is a comparison that marks that it started, waits for the
// cancellation, takes returnDelay to return, marks that it returned, and
// stops as internal, as Run does after a signal.
func slowCompare(dir string) compareFunc {
	return func(ctx context.Context, opts coveragecompare.Options) coveragecompare.Result {
		mark(dir, startedFile)
		<-ctx.Done()
		time.Sleep(returnDelay)
		mark(dir, returnedFile)
		return coveragecompare.Result{
			Outcome: coveragecompare.OutcomeStop,
			Stop:    coveragecompare.StopInternal,
			Target:  opts.Target,
			BaseArg: opts.Base,
		}
	}
}

// mark creates the named empty file in dir; the test sees a missing file.
func mark(dir, name string) {
	_ = os.WriteFile(filepath.Join(dir, name), nil, 0o600) //nolint:gosec // a file in the test's temporary directory
}

// lastLine returns the last line of out.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	return lines[len(lines)-1]
}

// failingWriter fails every write.
type failingWriter struct{}

var errWrite = errors.New("write failed")

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

// TestRun checks the command line: the two accepted forms reach the
// comparison with the target, BASE, root, and environment, and the outcome
// decides the exit code; every other shape stops as usage with exit code 3
// and an empty standard error, without running the comparison.
func TestRun(t *testing.T) {
	const root = "/repo"
	environ := []string{"HOME=/home/u"}
	t.Run("accepted", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			args    []string
			target  coveragecompare.Target
			outcome coveragecompare.Outcome
			code    int
			final   string
		}{
			{"go", []string{"go", "--base", "HEAD~1"}, coveragecompare.TargetGo, coveragecompare.OutcomePass, 0, "coverage-compare: PASS"},
			{"ext", []string{"ext", "--base", "HEAD~1"}, coveragecompare.TargetExt, coveragecompare.OutcomeFail, 1, "coverage-compare: FAIL"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var got []coveragecompare.Options
				compare := func(_ context.Context, opts coveragecompare.Options) coveragecompare.Result {
					got = append(got, opts)
					return coveragecompare.Result{Outcome: tc.outcome, Target: opts.Target, BaseArg: opts.Base}
				}
				var stdout, stderr bytes.Buffer
				code := run(context.Background(), tc.args, root, environ, &stdout, &stderr, compare)
				if len(got) != 1 {
					t.Fatalf("comparison ran %d times, want 1", len(got))
				}
				opts := got[0]
				if opts.Target != tc.target || opts.Base != "HEAD~1" || opts.RepoRoot != root || !slices.Equal(opts.Env, environ) {
					t.Errorf("options = %+v, want target %v, base HEAD~1, root %s, env %v", opts, tc.target, root, environ)
				}
				if code != tc.code || lastLine(stdout.String()) != tc.final || stderr.Len() != 0 {
					t.Errorf("exit %d, final %q, stderr %q; want exit %d, final %q, empty stderr", code, lastLine(stdout.String()), stderr.String(), tc.code, tc.final)
				}
			})
		}
	})
	t.Run("refused", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{"no_arguments", nil},
			{"target_only", []string{"go"}},
			{"base_missing", []string{"go", "HEAD~1"}},
			{"base_value_missing", []string{"go", "--base"}},
			{"base_empty", []string{"ext", "--base", ""}},
			{"base_twice", []string{"go", "--base", "HEAD~1", "--base", "HEAD~2"}},
			{"equals_form", []string{"go", "--base=HEAD~1"}},
			{"unknown_target", []string{"js", "--base", "HEAD~1"}},
			{"target_last", []string{"--base", "HEAD~1", "go"}},
			{"extra_argument", []string{"go", "--base", "HEAD~1", "extra"}},
			{"single_dash", []string{"go", "-base", "HEAD~1"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				ran := false
				compare := func(context.Context, coveragecompare.Options) coveragecompare.Result {
					ran = true
					return coveragecompare.Result{Outcome: coveragecompare.OutcomePass}
				}
				var stdout, stderr bytes.Buffer
				code := run(context.Background(), tc.args, root, environ, &stdout, &stderr, compare)
				if ran {
					t.Error("the comparison ran for a refused command line")
				}
				if code != 3 || lastLine(stdout.String()) != "coverage-compare: STOP usage" || stderr.Len() != 0 {
					t.Errorf("exit %d, output %q, stderr %q; want exit 3, final usage stop, empty stderr", code, stdout.String(), stderr.String())
				}
			})
		}
	})
	t.Run("write_fails", func(t *testing.T) {
		compare := func(context.Context, coveragecompare.Options) coveragecompare.Result {
			return coveragecompare.Result{Outcome: coveragecompare.OutcomePass}
		}
		var stderr bytes.Buffer
		code := run(context.Background(), []string{"go", "--base", "HEAD~1"}, root, environ, failingWriter{}, &stderr, compare)
		if code != exitWriteFailed || stderr.Len() == 0 {
			t.Errorf("exit %d, stderr %q; want exit %d and a diagnostic", code, stderr.String(), exitWriteFailed)
		}
	})
}

// TestSignalWaitsForRun starts the test binary as the tool with a comparison
// that takes returnDelay to return after the cancellation, and sends it
// SIGTERM once or twice: either way the tool waits for the comparison to
// return and only then writes the internal final line and exits 3, so a
// second signal cannot end it before Run has cleaned up.
func TestSignalWaitsForRun(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		signals int
	}{{"one_signal", 1}, {"two_signals", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(executable, "go", "--base", "HEAD~1") //nolint:gosec // the test binary, run as the tool
			cmd.Env = append(os.Environ(), childDirEnv+"="+dir)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			// Bounded, so a tool that ignores the signal cannot hang the test.
			cmd.WaitDelay = 30 * time.Second
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			waitForFile(t, filepath.Join(dir, startedFile))
			for range tc.signals {
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
				// Well inside returnDelay, so a second signal comes while the
				// comparison still runs.
				time.Sleep(100 * time.Millisecond)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err = <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("the tool did not exit after the signal")
			}
			if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != 3 {
				t.Errorf("tool ended with %v, want exit status 3 (stderr %q)", err, stderr.String())
			}
			if _, err := os.Stat(filepath.Join(dir, returnedFile)); err != nil {
				t.Errorf("the tool exited before the comparison returned: %v", err)
			}
			if lastLine(stdout.String()) != "coverage-compare: STOP internal" || stderr.Len() != 0 {
				t.Errorf("output %q, stderr %q; want the internal final line and empty stderr", stdout.String(), stderr.String())
			}
		})
	}
}

// waitForFile waits until path exists, failing the test after 30 seconds.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not appear", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
