//go:build test

package transcript

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// waitBoundMargin is added to execWaitDelay when a test asserts that a run
// returned within the grace period, so the check is a bound rather than a
// strict timing comparison.
const waitBoundMargin = 5 * time.Second

func TestCappedWriter(t *testing.T) {
	head := strings.Repeat("a", maxStderrBytes)
	cases := []struct {
		name   string
		writes []string
		want   string
	}{
		{"below the limit", []string{"abc"}, "abc"},
		{"empty write", []string{""}, ""},
		{"exactly the limit", []string{head}, head},
		{"one byte over the limit", []string{head + "b"}, head},
		{"crossing the limit in two writes", []string{head[:maxStderrBytes-1], "ab"}, head},
		{"write after the limit is dropped", []string{head, "tail"}, head},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var writer cappedWriter
			for _, chunk := range tc.writes {
				n, err := writer.Write([]byte(chunk))
				if err != nil {
					t.Fatalf("Write(%d bytes) error = %v, want nil", len(chunk), err)
				}
				if n != len(chunk) {
					t.Fatalf("Write(%d bytes) = %d, want %d: every byte must be reported as written so the child is drained", len(chunk), n, len(chunk))
				}
			}
			if got := writer.String(); got != tc.want {
				t.Fatalf("kept %d bytes, want %d", len(got), len(tc.want))
			}
		})
	}
}

func TestCommandExecutorDrainsStderr(t *testing.T) {
	const (
		burst   = 256 << 10
		timeout = 2 * time.Second
	)
	script := writeHelperScript(t, "#!/bin/sh\nprintf '%s' '"+strings.Repeat("x", burst)+"' >&2\nexit 3\n")

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	var stderr cappedWriter
	err := osExecutor{}.Run(ctx, script, nil, allowlistEnv(os.Environ()), &stderr)
	if err == nil {
		t.Fatal("Run error = nil, want a non-zero exit error")
	}
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 3 {
		t.Fatalf("Run error = %v, want exit code 3: a write that is not drained blocks the child or kills it with SIGPIPE", err)
	}
	if got := stderr.String(); got != strings.Repeat("x", maxStderrBytes) {
		t.Fatalf("kept stderr = %d bytes, want the first %d bytes", len(got), maxStderrBytes)
	}
}

func TestCommandExecutorNoShell(t *testing.T) {
	script := writeHelperScript(t, `#!/bin/sh
out="$1"
shift
printf '%s\n' "$@" > "$out"
`)
	dir := t.TempDir()
	outPath := filepath.Join(dir, "args.txt")
	markerPath := filepath.Join(dir, "shell-ran")
	args := []string{
		outPath,
		"a;b",
		"$(touch " + markerPath + ")",
		"`echo injected`",
		"a b",
		"*",
		"&&",
		"|",
		">redirect",
	}

	if err := (osExecutor{}).Run(t.Context(), script, args, allowlistEnv(nil), nil); err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read helper output: %v", err)
	}
	want := strings.Join(args[1:], "\n") + "\n"
	if string(data) != want {
		t.Fatalf("helper received %q, want %q", data, want)
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("command substitution ran: os.Stat(%q) error = %v, want fs.ErrNotExist", markerPath, err)
	}
}

func TestCommandExecutorEnvAllowlist(t *testing.T) {
	const helperRan = "helper-ran"

	run := func(t *testing.T, env []string) map[string]string {
		t.Helper()
		outPath := filepath.Join(t.TempDir(), "env.txt")
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("resolve test binary: %v", err)
		}
		args := []string{"-test.run=TestExecutorHelperProcess", "--", outPath}
		if err := (osExecutor{}).Run(t.Context(), executable, args, env, nil); err != nil {
			t.Fatalf("Run error = %v, want nil", err)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read helper output: %v", err)
		}
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		if len(lines) == 0 || lines[0] != helperRan {
			t.Fatalf("helper output = %q, want it to start with %q", data, helperRan)
		}
		got := make(map[string]string, len(lines)-1)
		for _, line := range lines[1:] {
			name, value, ok := strings.Cut(line, "=")
			if !ok {
				t.Fatalf("malformed environment line %q", line)
			}
			got[name] = value
		}
		return got
	}

	t.Run("passes every allowlisted variable that is set", func(t *testing.T) {
		// Every name of the fixed allowlist is set to a distinct value, so a
		// dropped or renamed entry fails the exact comparison below. TMPDIR
		// must exist: under coverage the re-executed test binary creates its
		// coverage temp files there and exits non-zero if it cannot.
		want := sampleAllowlistEnv()
		want["TMPDIR"] = t.TempDir()
		parent := []string{
			"YT2COLUMN_TEST_UNLISTED_MARKER=marker",
			"DEEPSEEK_API_KEY=secret",
			"SLACK_WEBHOOK_URL=https://hooks.example.invalid/secret",
			"MALFORMED",
		}
		for name, value := range want {
			parent = append(parent, name+"="+value)
		}
		if got := run(t, allowlistEnv(parent)); !maps.Equal(got, want) {
			t.Fatalf("child environment = %v, want %v", got, want)
		}
	})

	t.Run("passes an empty environment when no allowlisted variable is set", func(t *testing.T) {
		// The marker is set in the test process, so inheriting the parent
		// environment (a nil Cmd.Env) would show up in the helper's report.
		t.Setenv("YT2COLUMN_TEST_UNLISTED_MARKER", "inherited")
		parent := []string{
			"YT2COLUMN_TEST_UNLISTED_MARKER=parent",
			"DEEPSEEK_API_KEY=secret",
		}
		if got := run(t, allowlistEnv(parent)); len(got) != 0 {
			t.Fatalf("child environment = %v, want it empty", got)
		}
	})

	t.Run("treats a nil environment as empty", func(t *testing.T) {
		t.Setenv("YT2COLUMN_TEST_UNLISTED_MARKER", "inherited")
		if got := run(t, nil); len(got) != 0 {
			t.Fatalf("child environment = %v, want it empty", got)
		}
	})
}

func TestCommandExecutorStartFailure(t *testing.T) {
	dir := t.TempDir()
	nonExecutable := filepath.Join(dir, "not-executable")
	if err := os.WriteFile(nonExecutable, []byte("not an executable\n"), 0o644); err != nil {
		t.Fatalf("write non-executable file: %v", err)
	}
	cases := []struct {
		name string
		path string
	}{
		{"missing file", filepath.Join(dir, "missing")},
		{"non-executable file", nonExecutable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Run must surface a start failure instead of swallowing it.
			err := osExecutor{}.Run(t.Context(), tc.path, nil, nil, nil)
			if err == nil {
				t.Fatalf("Run(%q) error = nil, want a start error", tc.path)
			}
		})
	}

	t.Run("Fetch maps the start failure to ErrYtDlpExec", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				source, err := NewYtDlpSource(Options{
					CacheDir:  filepath.Join(t.TempDir(), "cache"),
					YtDlpPath: tc.path,
					Timeout:   time.Minute,
				})
				if err != nil {
					t.Fatalf("NewYtDlpSource error = %v", err)
				}
				_, err = source.Fetch(t.Context(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
				if !errors.Is(err, ErrYtDlpExec) {
					t.Fatalf("Fetch error = %v, want ErrYtDlpExec", err)
				}
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					t.Errorf("start failure also matches a context error: %v", err)
				}
			})
		}
	})
}

// TestExecutorHelperProcess is not a regular test: TestCommandExecutorEnvAllowlist
// re-executes the test binary with -test.run selecting it, to report the exact
// environment the process received. An invocation without the positional
// output path is a normal test pass, and is skipped.
func TestExecutorHelperProcess(t *testing.T) {
	args := flag.Args()
	if len(args) != 1 {
		t.Skip("helper process for TestCommandExecutorEnvAllowlist")
	}
	var report strings.Builder
	report.WriteString("helper-ran\n")
	for _, entry := range os.Environ() {
		report.WriteString(entry)
		report.WriteByte('\n')
	}
	if err := os.WriteFile(args[0], []byte(report.String()), 0o600); err != nil {
		t.Fatalf("write environment: %v", err)
	}
}

func TestCommandExecutorWaitDelay(t *testing.T) {
	const (
		descendantSleep = 20 * time.Second
		timeout         = 500 * time.Millisecond
	)

	newHelper := func(t *testing.T, exitImmediately bool) (script, pidPath string) {
		t.Helper()
		sleepSeconds := int(descendantSleep / time.Second)
		body := fmt.Sprintf("#!/bin/sh\nsleep %d &\necho $! > \"$1\"\n", sleepSeconds)
		if exitImmediately {
			body += "exit 0\n"
		} else {
			// wait is a shell builtin, so the timeout leaves only the recorded
			// descendant behind instead of a second orphaned sleep.
			body += "wait\n"
		}
		pidPath = filepath.Join(t.TempDir(), "descendant.pid")
		t.Cleanup(func() { killRecordedProcess(pidPath) })
		return writeHelperScript(t, body), pidPath
	}

	t.Run("child exits while a descendant holds stderr", func(t *testing.T) {
		script, pidPath := newHelper(t, true)
		var stderr cappedWriter
		start := time.Now()
		err := osExecutor{}.Run(t.Context(), script, []string{pidPath}, allowlistEnv(os.Environ()), &stderr)
		elapsed := time.Since(start)
		if !errors.Is(err, exec.ErrWaitDelay) {
			t.Fatalf("Run error = %v, want exec.ErrWaitDelay", err)
		}
		if elapsed > execWaitDelay+waitBoundMargin {
			t.Fatalf("Run took %v, want at most %v", elapsed, execWaitDelay+waitBoundMargin)
		}
	})

	t.Run("timeout while a descendant holds stderr", func(t *testing.T) {
		script, pidPath := newHelper(t, false)
		ctx, cancel := context.WithTimeout(t.Context(), timeout)
		defer cancel()
		var stderr cappedWriter
		start := time.Now()
		err := osExecutor{}.Run(ctx, script, []string{pidPath}, allowlistEnv(os.Environ()), &stderr)
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("Run error = nil, want a failure after the timeout")
		}
		if elapsed > timeout+execWaitDelay+waitBoundMargin {
			t.Fatalf("Run took %v, want at most %v", elapsed, timeout+execWaitDelay+waitBoundMargin)
		}
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("ctx error = %v, want context.DeadlineExceeded", ctx.Err())
		}
		if _, ok := readRecordedPID(pidPath); !ok {
			t.Fatal("helper did not record a descendant holding stderr")
		}
	})
}

// writeHelperScript writes an executable POSIX shell script and returns its path.
func writeHelperScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write helper script: %v", err)
	}
	return path
}

// readRecordedPID reads a descendant PID recorded by a helper script.
func readRecordedPID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

// killRecordedProcess kills the descendant whose PID is recorded in path. It
// is safe when the file is missing or the process already exited.
func killRecordedProcess(path string) {
	pid, ok := readRecordedPID(path)
	if !ok {
		return
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
}
