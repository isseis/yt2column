//go:build test

package cmdrunner

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sh(script string) []string { return []string{"sh", "-c", script} }

func run(t *testing.T, opts Options, stdin []byte, script string) ([]byte, error) {
	t.Helper()
	args := sh(script)
	return NewOS(opts).RunInput(t.Context(), stdin, args[0], args[1:]...)
}

func TestMaxOutputBoundary(t *testing.T) {
	const limit = 16
	tests := []struct {
		name    string
		script  string
		wantErr error
	}{
		{"exact stdout", "head -c 16 /dev/zero", nil},
		{"one byte over stdout", "head -c 17 /dev/zero", ErrOutputTooLarge},
		{"one byte over stderr", "head -c 17 /dev/zero >&2", ErrOutputTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := run(t, Options{MaxOutput: limit}, nil, tt.script)
			if !errorsIs(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil && out != nil {
				t.Errorf("output = %d bytes, want none on overflow", len(out))
			}
			if tt.wantErr == nil && len(out) != limit {
				t.Errorf("output = %d bytes, want %d", len(out), limit)
			}
		})
	}
}

func TestZeroMaxOutputIsUnbounded(t *testing.T) {
	const size = 1 << 20
	out, err := run(t, Options{}, nil, "head -c 1048576 /dev/zero")
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if len(out) != size {
		t.Errorf("output = %d bytes, want %d", len(out), size)
	}
}

func TestOverflowIsDrained(t *testing.T) {
	// The output is far larger than a pipe buffer: a runner that stopped
	// reading after the cap would leave the child blocked, and the test would
	// run into its timeout instead of failing with ErrOutputTooLarge.
	done := make(chan error, 1)
	go func() {
		_, err := run(t, Options{MaxOutput: 8}, nil, "head -c 4194304 /dev/zero")
		done <- err
	}()
	select {
	case err := <-done:
		if !errorsIs(err, ErrOutputTooLarge) {
			t.Errorf("error = %v, want ErrOutputTooLarge", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("runner blocked after the cap")
	}
}

func TestWaitDelayBoundsInheritedPipe(t *testing.T) {
	// The shell exits at once but its background child keeps the output pipe
	// open for a few seconds; WaitDelay must end the wait first.
	start := time.Now()
	_, err := run(t, Options{WaitDelay: 100 * time.Millisecond}, nil, "sleep 3 & echo done")
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Run took %v, want it bounded by WaitDelay", elapsed)
	}
	if !errorsIs(err, exec.ErrWaitDelay) {
		t.Errorf("error = %v, want exec.ErrWaitDelay", err)
	}
}

func TestStdinIsPassed(t *testing.T) {
	out, err := run(t, Options{}, []byte("payload"), "cat")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if string(out) != "payload" {
		t.Errorf("output = %q, want %q", out, "payload")
	}
}

func TestRunReturnsOutput(t *testing.T) {
	out, err := NewOS(Options{}).Run(t.Context(), "sh", "-c", "printf hello")
	if err != nil || string(out) != "hello" {
		t.Errorf("Run = %q, %v; want hello, nil", out, err)
	}
}

func TestExitErrorIsTyped(t *testing.T) {
	_, err := run(t, Options{}, nil, "printf out; printf boom >&2; exit 3")
	exitErr, ok := errorsAsExit(err)
	if !ok {
		t.Fatalf("error = %v, want *ExitError", err)
	}
	if exitErr.Code != 3 || string(exitErr.Stdout) != "out" || string(exitErr.Stderr) != "boom" {
		t.Errorf("ExitError = %+v, want code 3, stdout out, stderr boom", exitErr)
	}
	// The message keeps the shape internal/mergepr always produced.
	msg := err.Error()
	if !strings.Contains(msg, "exit status 3") || !strings.Contains(msg, "boom") || !strings.HasPrefix(msg, "sh -c ") {
		t.Errorf("message = %q, want command, exit status and stderr", msg)
	}
}

func TestStartFailureIsNotExitError(t *testing.T) {
	_, err := NewOS(Options{}).Run(t.Context(), "no-such-command-for-cmdrunner")
	if err == nil {
		t.Fatal("error = nil, want a failure")
	}
	if _, ok := errorsAsExit(err); ok {
		t.Errorf("error = %v, want a non-ExitError start failure", err)
	}
}

func TestDirAndEnv(t *testing.T) {
	dir := t.TempDir()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := run(t, Options{Dir: dir, Env: []string{"CMDRUNNER_PROBE=yes"}}, nil, `printf '%s|%s|%s' "$PWD" "$CMDRUNNER_PROBE" "${HOME-unset}"`)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	got := string(bytes.TrimSpace(out))
	parts := strings.Split(got, "|")
	if len(parts) != 3 {
		t.Fatalf("output = %q", got)
	}
	if resolved, _ := filepath.EvalSymlinks(parts[0]); resolved != want {
		t.Errorf("cwd = %q, want %q", parts[0], want)
	}
	if parts[1] != "yes" {
		t.Errorf("CMDRUNNER_PROBE = %q, want yes", parts[1])
	}
	if parts[2] != "unset" {
		t.Errorf("HOME = %q, want the environment replaced, not extended", parts[2])
	}
}

func errorsIs(err, target error) bool {
	if target == nil {
		return err == nil
	}
	return errors.Is(err, target)
}

func errorsAsExit(err error) (*ExitError, bool) { return errors.AsType[*ExitError](err) }
