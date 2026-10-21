//go:build test

package cmdrunner

import (
	"bytes"
	"context"
	"errors"
	"os"
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
			if !errors.Is(err, tt.wantErr) {
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

func TestMaxOutputIsCumulativeAcrossWrites(t *testing.T) {
	// Two separate writes, each under the cap and together over it.
	tests := []struct {
		name    string
		script  string
		wantErr error
	}{
		{"together over", "printf 0123456789; sleep 0.1; printf 0123456789", ErrOutputTooLarge},
		{"together exact", "printf 01234567; sleep 0.1; printf 01234567", nil},
		{"stderr exact", "printf 0123456789abcdef >&2", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(t, Options{MaxOutput: 16}, nil, tt.script)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCancelWithDescendantReturnsAndWrapsContextError(t *testing.T) {
	// The shell is killed by the deadline while its background child still
	// holds the output pipe; WaitDelay must end the wait, and the context
	// error must be visible next to the typed failure.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	args := sh("sleep 10 & wait")
	_, err := NewOS(Options{WaitDelay: 100 * time.Millisecond}).Run(ctx, args[0], args[1:]...)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Run took %v, want it bounded by the deadline and WaitDelay", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
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
	// The output is far larger than a pipe buffer. If the runner stopped
	// reading after the cap, dd would die of SIGPIPE and never write the
	// marker; draining lets it finish.
	marker := filepath.Join(t.TempDir(), "finished")
	_, err := run(t, Options{MaxOutput: 8}, nil, "dd if=/dev/zero bs=1048576 count=4 2>/dev/null && echo ok > "+marker)
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Errorf("error = %v, want ErrOutputTooLarge", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("the child did not run to completion: %v", statErr)
	}
}

func TestWaitDelayBoundsInheritedPipe(t *testing.T) {
	// The shell exits at once but its background child keeps the output pipe
	// open for a few seconds; WaitDelay must end the wait first.
	start := time.Now()
	_, err := run(t, Options{WaitDelay: 100 * time.Millisecond}, nil, "sleep 10 & echo done")
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Run took %v, want it bounded by WaitDelay", elapsed)
	}
	if !errors.Is(err, exec.ErrWaitDelay) {
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

func TestExitErrorIsTyped(t *testing.T) {
	_, err := run(t, Options{}, nil, "printf out; printf boom >&2; exit 3")
	exitErr, ok := errors.AsType[*ExitError](err)
	if !ok {
		t.Fatalf("error = %v, want *ExitError", err)
	}
	if exitErr.Code != 3 || string(exitErr.Stdout) != "out" || string(exitErr.Stderr) != "boom" {
		t.Errorf("ExitError = %+v, want code 3, stdout out, stderr boom", exitErr)
	}
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		t.Errorf("error = %v, want *exec.ExitError reachable through Unwrap", err)
	}
	// The wording is the contract here: internal/mergepr's callers and logs
	// always saw this shape, so the migration must not change it.
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
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("error = %v, want exec.ErrNotFound", err)
	}
	if _, ok := errors.AsType[*ExitError](err); ok {
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
