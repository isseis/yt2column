//go:build test

package transcript

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
	err := osExecutor{}.Run(ctx, script, nil, allowlistEnv(os.Environ()), nil, &stderr)
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

	if err := (osExecutor{}).Run(t.Context(), script, args, allowlistEnv(nil), nil, nil); err != nil {
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
		if err := (osExecutor{}).Run(t.Context(), executable, args, env, nil, nil); err != nil {
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

func TestCommandExecutorInheritedFiles(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("inherited-content"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	input, err := os.Open(inputPath)
	if err != nil {
		t.Fatalf("open input: %v", err)
	}
	t.Cleanup(func() { _ = input.Close() })
	outPath := filepath.Join(dir, "out.txt")
	script := writeHelperScript(t, "#!/bin/sh\ncat <&3 > \"$1\"\n")

	if err := (osExecutor{}).Run(t.Context(), script, []string{outPath}, allowlistEnv(os.Environ()), []*os.File{input}, nil); err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read helper output: %v", err)
	}
	if string(data) != "inherited-content" {
		t.Fatalf("child read %q from descriptor 3, want %q", data, "inherited-content")
	}
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
			err := osExecutor{}.Run(t.Context(), tc.path, nil, nil, nil, nil)
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
		err := osExecutor{}.Run(t.Context(), script, []string{pidPath}, allowlistEnv(os.Environ()), nil, &stderr)
		elapsed := time.Since(start)
		if !errors.Is(err, exec.ErrWaitDelay) {
			t.Fatalf("Run error = %v, want exec.ErrWaitDelay", err)
		}
		if elapsed > execWaitDelay+waitBoundMargin {
			t.Fatalf("Run took %v, want at most %v", elapsed, execWaitDelay+waitBoundMargin)
		}
	})

	t.Run("timeout while a descendant outside the group holds stderr", func(t *testing.T) {
		// The group kill cannot reach a descendant that left the group, so only
		// WaitDelay bounds this run.
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("resolve test binary: %v", err)
		}
		dir := t.TempDir()
		pidPath := filepath.Join(dir, "descendant.pid")
		releasePath := filepath.Join(dir, "release")
		t.Cleanup(func() {
			releaseHelper(releasePath)
			killRecordedProcess(pidPath)
		})
		args := []string{"-test.run=TestDetachedDescendantHelperProcess", "--", detachedParentMode, pidPath, releasePath}
		var stderr cappedWriter
		ctx, elapsed, err := runCanceledOnceRecorded(t, executable, args, &stderr, pidPath)
		if err == nil {
			t.Fatal("Run error = nil, want a failure after the cancellation")
		}
		if elapsed > execWaitDelay+waitBoundMargin {
			t.Fatalf("Run took %v after the cancellation, want at most %v", elapsed, execWaitDelay+waitBoundMargin)
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("ctx error = %v, want context.Canceled", ctx.Err())
		}
		pid, ok := readRecordedPID(pidPath)
		if !ok {
			t.Fatal("helper did not record a descendant holding stderr")
		}
		if !processExists(pid) {
			t.Fatal("descendant outside the group is gone, want it left running: the test no longer exercises WaitDelay")
		}
	})
}

func TestCommandExecutorKillsProcessGroup(t *testing.T) {
	dir := t.TempDir()
	childPIDPath := filepath.Join(dir, "child.pid")
	grandchildPIDPath := filepath.Join(dir, "grandchild.pid")
	releasePath := filepath.Join(dir, "release")
	// The grandchild leaves on the release file or after about five minutes, so
	// a failing test cannot leave it running.
	script := writeHelperScript(t, `#!/bin/sh
echo $$ > "$1"
sh -c 'n=0; while [ ! -e "$0" ] && [ $n -lt 600 ]; do sleep 0.5; n=$((n+1)); done' "$3" &
echo $! > "$2"
wait
`)
	t.Cleanup(func() {
		releaseHelper(releasePath)
		killRecordedProcess(childPIDPath)
		killRecordedProcess(grandchildPIDPath)
	})

	var stderr cappedWriter
	args := []string{childPIDPath, grandchildPIDPath, releasePath}
	if _, _, err := runCanceledOnceRecorded(t, script, args, &stderr, childPIDPath, grandchildPIDPath); err == nil {
		t.Fatal("Run error = nil, want a failure after the cancellation")
	}

	for name, path := range map[string]string{"child": childPIDPath, "grandchild": grandchildPIDPath} {
		pid, ok := readRecordedPID(path)
		if !ok {
			t.Fatalf("helper did not record the %s PID", name)
		}
		if !waitProcessGone(pid, processGoneBound) {
			t.Errorf("%s (pid %d) is still running after the context ended, want the whole process group killed", name, pid)
		}
	}
}

// runCanceledOnceRecorded starts osExecutor.Run and cancels its context only
// after every file in pidPaths holds a PID, so the cancellation never races
// with helper startup. It returns the context, how long Run
// took after the cancellation, and Run's error.
func runCanceledOnceRecorded(t *testing.T, name string, args []string, stderr io.Writer, pidPaths ...string) (context.Context, time.Duration, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- osExecutor{}.Run(ctx, name, args, allowlistEnv(os.Environ()), nil, stderr)
	}()
	deadline := time.After(helperStartBound)
	for _, path := range pidPaths {
		for {
			if _, ok := readRecordedPID(path); ok {
				break
			}
			select {
			case err := <-done:
				t.Fatalf("Run returned before the helper recorded %s: %v", path, err)
			case <-deadline:
				cancel()
				<-done
				t.Fatalf("helper did not record %s within %v", path, helperStartBound)
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	start := time.Now()
	cancel()
	err := <-done
	return ctx, time.Since(start), err
}

const (
	// helperStartBound bounds how long a test waits for a helper to record its
	// PIDs.
	helperStartBound = 30 * time.Second
	// processGoneBound bounds how long a test waits for a killed process to
	// disappear from the process table.
	processGoneBound = 5 * time.Second
	// detachedParentMode and detachedDescendantMode select the role of the
	// re-executed test binary in TestDetachedDescendantHelperProcess.
	detachedParentMode     = "parent"
	detachedDescendantMode = "descendant"
	// detachedHelperLifetime caps how long a helper process lives when nothing
	// releases or kills it.
	detachedHelperLifetime = 3 * time.Minute
)

// TestDetachedDescendantHelperProcess is not a regular test: it is re-executed
// by TestCommandExecutorWaitDelay. In parent mode it starts itself in
// descendant mode in a new process group, with the standard error pipe
// inherited, records the descendant's PID, and then blocks. In descendant mode
// it waits for the release file or the lifetime cap. A normal test pass has no
// positional arguments and skips it.
func TestDetachedDescendantHelperProcess(t *testing.T) {
	args := flag.Args()
	if len(args) != 3 {
		t.Skip("helper process for TestCommandExecutorWaitDelay")
	}
	mode, pidPath, releasePath := args[0], args[1], args[2]
	switch mode {
	case detachedDescendantMode:
		waitForFile(releasePath, detachedHelperLifetime)
	case detachedParentMode:
		executable, err := os.Executable()
		if err != nil {
			t.Fatalf("resolve test binary: %v", err)
		}
		descendant := exec.Command(executable, "-test.run=TestDetachedDescendantHelperProcess", "--", detachedDescendantMode, pidPath, releasePath) //nolint:gosec // re-executes the running test binary
		descendant.Stderr = os.Stderr
		descendant.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := descendant.Start(); err != nil {
			t.Fatalf("start descendant: %v", err)
		}
		if err := os.WriteFile(pidPath, []byte(strconv.Itoa(descendant.Process.Pid)), 0o600); err != nil {
			t.Fatalf("record descendant PID: %v", err)
		}
		waitForFile(releasePath, detachedHelperLifetime)
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
}

// waitForFile blocks until path exists or limit has passed.
func waitForFile(path string, limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// releaseHelper creates the release file that lets helper processes exit.
func releaseHelper(path string) {
	_ = os.WriteFile(path, nil, 0o600)
}

// processExists reports whether a process with pid is in the process table.
func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
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

func TestRedactStderr(t *testing.T) {
	const secret = "s3cret-proxy-value"
	// shortProxy is a prefix of longProxy and comes first in env, so redacting
	// in env order would split longProxy and leave its token visible.
	const shortProxy = "https://proxy.example"
	const longProxy = shortProxy + "?token=T0KEN"
	env := []string{
		"HTTP_PROXY=" + shortProxy,
		"https_proxy=" + longProxy,
		"HTTPS_PROXY=" + secret,
		"NO_PROXY=",
		"LANG=en_US.UTF-8",
	}
	cases := map[string]struct {
		input string
		want  string
	}{
		"full proxy value":                {"error: " + secret + " failed", "error: " + redactedMarker + " failed"},
		"value cut at the cap":            {"error: " + secret[:7], "error: " + redactedMarker},
		"URL userinfo":                    {"https://user:pass@proxy.example/x", "https://" + redactedMarker + "@proxy.example/x"},
		"empty proxy value":               {"nothing to redact", "nothing to redact"},
		"non-proxy value kept":            {"LANG=en_US.UTF-8", "LANG=en_US.UTF-8"},
		"value containing another value":  {"via " + longProxy + " failed", "via " + redactedMarker + " failed"},
		"containing value cut at the cap": {"via " + longProxy[:len(shortProxy)+4], "via " + redactedMarker},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := redactStderr(tc.input, env); got != tc.want {
				t.Errorf("redactStderr(%q, env) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
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
	if !ok || !processExists(pid) {
		return
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
}
