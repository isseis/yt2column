//go:build test

package cmdrunner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

func TestStderrIsFailure(t *testing.T) {
	// The script writes secret without spelling it, since the error names the
	// command line.
	const secret = "stderr-secret"
	const writeSecret = "printf '%s-%s' stderr secret >&2"
	tests := []struct {
		name    string
		opts    Options
		script  string
		wantErr error
		wantOut string
	}{
		{"set_stderr_on_success", Options{StderrIsFailure: true}, "printf out; " + writeSecret, ErrUnexpectedStderr, ""},
		{"set_empty_stderr", Options{StderrIsFailure: true}, "printf out", nil, "out"},
		{"set_over_the_cap", Options{StderrIsFailure: true, MaxOutput: 4}, "printf 0123456789 >&2", ErrOutputTooLarge, ""},
		{"unset_stderr_on_success", Options{}, "printf out; " + writeSecret, nil, "out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := run(t, tt.opts, nil, tt.script)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if string(out) != tt.wantOut {
				t.Errorf("output = %q, want %q", out, tt.wantOut)
			}
			// The bytes a command wrote to standard error never reach the
			// error: a caller may print the error chain.
			if err != nil && strings.Contains(err.Error(), secret) {
				t.Errorf("error = %q carries the standard error", err)
			}
		})
	}
	t.Run("set_nonzero_exit", func(t *testing.T) {
		_, err := run(t, Options{StderrIsFailure: true}, nil, "printf boom >&2; exit 3")
		if exitErr, ok := errors.AsType[*ExitError](err); !ok || exitErr.Code != 3 {
			t.Errorf("error = %v, want *ExitError with code 3", err)
		}
		if errors.Is(err, ErrUnexpectedStderr) {
			t.Errorf("error = %v, want a failed exit, not ErrUnexpectedStderr", err)
		}
	})
}

const (
	// groupStopBound bounds how long a test waits for a group-mode run to
	// return, so a broken stop fails the test instead of hanging it.
	groupStopBound = 10 * time.Second
	// processGoneBound bounds how long a test waits for a killed process to
	// disappear from the process table.
	processGoneBound = 5 * time.Second
)

// groupScript starts a grandchild in the background that records its PID in
// $GRAND and then sleeps (a bounded lifetime, so a failed stop cannot leave it
// running for long). The direct child records its own PID, the group ID, in
// $LEADER first. Each PID is written to a temporary file and renamed, so a
// reader never sees a partial number. ignoreTerm makes both ignore SIGTERM;
// an ignored signal stays ignored across exec.
func groupScript(ignoreTerm bool, tail string) string {
	trap := ""
	if ignoreTerm {
		trap = `trap "" TERM; `
	}
	return trap + `echo $$ > "$LEADER.tmp" && mv "$LEADER.tmp" "$LEADER"; ` +
		`sh -c '` + trap + `echo $$ > "$GRAND.tmp" && mv "$GRAND.tmp" "$GRAND"; exec sleep 60' & ` + tail
}

type groupRun struct {
	out []byte
	err error
}

// startGroupRun runs script in a goroutine with opts, $LEADER and $GRAND set,
// waits until both PIDs are recorded, and returns the grandchild's PID and the
// function that cancels the run's context. As soon as each PID is known, a
// cleanup kills it (the leader's PID as a group), so nothing outlives the test
// whatever the runner does.
func startGroupRun(t *testing.T, opts Options, script string) (grand int, cancel context.CancelFunc, result <-chan groupRun) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	dir := t.TempDir()
	leaderPath, grandPath := filepath.Join(dir, "leader"), filepath.Join(dir, "grand")
	opts.Env = append(os.Environ(), "LEADER="+leaderPath, "GRAND="+grandPath)
	done := make(chan groupRun, 1)
	go func() {
		args := sh(script)
		out, err := NewOS(opts).RunInput(ctx, nil, args[0], args[1:]...)
		done <- groupRun{out, err}
	}()
	leader := waitForPID(t, leaderPath, done)
	t.Cleanup(func() { _ = syscall.Kill(-leader, syscall.SIGKILL) })
	grand = waitForPID(t, grandPath, done)
	t.Cleanup(func() { _ = syscall.Kill(grand, syscall.SIGKILL) })
	return grand, cancel, done
}

// waitForPID polls path until it holds a PID. It fails the test when the run
// ends first or the bound passes.
func waitForPID(t *testing.T, path string, done <-chan groupRun) int {
	t.Helper()
	deadline := time.Now().Add(groupStopBound)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatalf("PID file %s = %q: %v", path, data, err)
			}
			return pid
		}
		select {
		case r := <-done:
			t.Fatalf("run ended before %s was written: %v", path, r.err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatalf("%s was not written within %v", path, groupStopBound)
	return 0
}

// awaitRun returns the run's result, failing the test when it does not end
// within groupStopBound.
func awaitRun(t *testing.T, result <-chan groupRun) groupRun {
	t.Helper()
	select {
	case r := <-result:
		return r
	case <-time.After(groupStopBound):
		t.Fatalf("run did not return within %v", groupStopBound)
		return groupRun{}
	}
}

// assertGone fails the test unless pid disappears within processGoneBound.
func assertGone(t *testing.T, pid int, what string) {
	t.Helper()
	if !waitProcessGone(pid, processGoneBound) {
		t.Errorf("%s (PID %d) is still running", what, pid)
	}
}

func TestProcessGroupStoppedOnCancel(t *testing.T) {
	tests := []struct {
		name       string
		ignoreTerm bool
		grace      time.Duration
	}{
		// The group exits on SIGTERM.
		{"terminate", false, 5 * time.Second},
		// Both processes ignore SIGTERM; only the SIGKILL after the grace
		// stops them.
		{"ignores_terminate", true, 200 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{ProcessGroup: true, GroupStopGrace: tt.grace, WaitDelay: time.Second}
			grand, cancel, result := startGroupRun(t, opts, groupScript(tt.ignoreTerm, "wait"))
			cancelled := time.Now()
			cancel()
			r := awaitRun(t, result)
			if !errors.Is(r.err, context.Canceled) {
				t.Errorf("error = %v, want context.Canceled", r.err)
			}
			if tt.ignoreTerm {
				if elapsed := time.Since(cancelled); elapsed < tt.grace {
					t.Errorf("run returned %v after cancellation, want SIGKILL only after the %v grace", elapsed, tt.grace)
				}
			}
			assertGone(t, grand, "grandchild")
		})
	}
}

func TestProcessGroupKilledAfterChildExits(t *testing.T) {
	// The direct child exits 0 once the grandchild is running, and the
	// grandchild keeps the output pipe open. Without the group kill after the
	// exit, WaitDelay would end the wait with exec.ErrWaitDelay.
	opts := Options{ProcessGroup: true, WaitDelay: 2 * time.Second}
	tail := `while [ ! -f "$GRAND" ]; do sleep 0.05; done; printf out`
	grand, _, result := startGroupRun(t, opts, groupScript(false, tail))
	r := awaitRun(t, result)
	if r.err != nil {
		t.Fatalf("error = %v, want success", r.err)
	}
	if string(r.out) != "out" {
		t.Errorf("output = %q, want %q", r.out, "out")
	}
	assertGone(t, grand, "grandchild")
}

// processExists reports whether a live process with pid exists. A zombie counts
// as gone: kill(pid, 0) succeeds for it, and a container whose PID 1 does not
// reap orphans would otherwise leave every killed descendant looking alive.
func processExists(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	return processState(pid) != 'Z'
}

// processState returns the one-letter state of pid, or 0 when it cannot be read.
func processState(pid int) byte {
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		// The command name may contain spaces and parentheses, so the state is
		// the first field after the last closing parenthesis.
		if i := bytes.LastIndex(data, []byte(") ")); i >= 0 && i+2 < len(data) {
			return data[i+2]
		}
		return 0
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // fixed arguments, pid is an integer
	if err != nil {
		return 0
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return s[0]
	}
	return 0
}

// waitProcessGone polls until pid is gone or limit has passed.
func waitProcessGone(pid int, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for {
		if !processExists(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}
