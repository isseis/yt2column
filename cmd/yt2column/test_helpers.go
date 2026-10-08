//go:build test

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/writer"
)

// Environment variables that switch the test binary into a CLI child process
// (see TestMain) and tell the child's fake LLM client where to put its
// markers.
const (
	childModeEnv    = "YT2COLUMN_TEST_CLI_MODE"
	childReadyEnv   = "YT2COLUMN_TEST_LLM_READY"
	childCtxDoneEnv = "YT2COLUMN_TEST_LLM_CTX_DONE"
)

// Child modes.
const (
	// childModeMain runs the production main, so a test observes exactly the
	// signal handling main installs.
	childModeMain = "main"
	// childModeStall runs runWithSignals with an LLM client that stops until
	// its context ends.
	childModeStall = "stall"
	// childModeIgnoreCtx runs runWithSignals with an LLM client that keeps
	// stopping after its context ends.
	childModeIgnoreCtx = "ignore-ctx"
	// childModeSlackStall runs runWithSignals with an LLM client that
	// succeeds at once and a webhook publisher that stops until its context
	// ends; the publisher, not the LLM client, creates the ready marker.
	childModeSlackStall = "slack-stall"
)

// childExitUnknownMode is the child's exit code for an unknown mode.
const childExitUnknownMode = 99

// stallLifetime caps how long a stalling fake LLM client waits, so a leaked
// child exits by itself. It is far longer than any bound a test waits for.
const stallLifetime = 3 * time.Minute

// childWaitDelay bounds how long Wait drains a child's output after it exits.
const childWaitDelay = 10 * time.Second

const (
	// readyBound bounds a wait for a fake to signal readiness, so a fake that
	// fails to start is reported instead of hanging.
	readyBound = 30 * time.Second
	// exitBound bounds a wait for a CLI child to exit.
	exitBound = 30 * time.Second
	// pollInterval paces the bounded polling loops.
	pollInterval = 20 * time.Millisecond
)

// childEnvAllowlist names the parent variables a CLI child inherits: what a
// shell script needs to run, and the proxy variables TestMain points at a
// closed listener. Everything else, including the developer's YT2COLUMN_*
// variables, secrets, and GODEBUG, is left out; a test adds what it needs.
var childEnvAllowlist = []string{
	"PATH", "HOME", "TMPDIR",
	"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy",
}

var (
	errStallExpired = errors.New("the stalling fake reached its lifetime")
	// errSlackPublisherNotSubstituted is what the default newSlackPublisher of
	// testDeps and newRunEnv returns: a test that posts must substitute one
	// that sends to a loopback server.
	errSlackPublisherNotSubstituted = errors.New("the test deps send nothing to a webhook; substitute newSlackPublisher")
	errLoopbackWebhookURL           = errors.New("the configured webhook URL does not parse")
)

// loopbackPostTimeout is the per-message timeout of a loopback test
// publisher; a loopback server answers far sooner.
const loopbackPostTimeout = 10 * time.Second

// validResponse is a generated text the real ArticleWriter accepts.
func validResponse() llm.GenerateResponse {
	return llm.GenerateResponse{Text: "# A title\n\nThe body.\n", Model: "fake-model", ModelVersion: "v1"}
}

// validResponseLLM returns a client that always succeeds.
func validResponseLLM() llm.LLMClient {
	return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return validResponse(), nil
	})
}

// funcLLM is an llm.LLMClient that runs the function.
type funcLLM func(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error)

// Generate implements llm.LLMClient.
func (f funcLLM) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	return f(ctx, req)
}

// stallingLLM creates ready and then waits for its context to end. With
// ignoreCtx it then creates ctxDone and keeps waiting, so only the default
// disposition of a signal can stop the process. Either way it gives up after
// stallLifetime.
type stallingLLM struct {
	ready     string
	ctxDone   string
	ignoreCtx bool
}

// Generate implements llm.LLMClient.
func (s stallingLLM) Generate(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	writeMarker(s.ready)
	timer := time.NewTimer(stallLifetime)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
		return llm.GenerateResponse{}, errStallExpired
	}
	if !s.ignoreCtx {
		return llm.GenerateResponse{}, ctx.Err()
	}
	writeMarker(s.ctxDone)
	<-timer.C
	return llm.GenerateResponse{}, errStallExpired
}

// writeMarker creates path by renaming a temporary file, so a reader never
// sees it half-written. Failures are ignored: a missing marker makes the
// waiting test fail with the marker's name.
func writeMarker(path string) {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, nil, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// stallingPublisher creates ready and then waits for its context to end, as a
// webhook post in flight does. It then returns the error SlackWebhookPublisher
// returns for a canceled first message, so the CLI reports it as it would a
// real one. It gives up after stallLifetime.
type stallingPublisher struct {
	ready string
}

// Publish implements publisher.Publisher.
func (s stallingPublisher) Publish(ctx context.Context, _ writer.Article) error {
	writeMarker(s.ready)
	timer := time.NewTimer(stallLifetime)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return &publisher.SlackPostError{Total: 1, Posted: 0, Attempted: true, Err: ctx.Err()}
	case <-timer.C:
		return errStallExpired
	}
}

// testDeps returns the production deps with the LLM client replaced by
// client, so the real ArticleWriter and FilePublisher run. newSlackPublisher
// sends nothing and fails (refusingSlackPublisher), so a --slack run can never
// reach the Webhook named by the environment.
func testDeps(client llm.LLMClient) deps {
	d := productionDeps()
	d.newLLMClient = func(config.Config) (llm.LLMClient, error) { return client, nil }
	d.newSlackPublisher = refusingSlackPublisher
	return d
}

// refusingSlackPublisher is the default newSlackPublisher of the test deps: it
// builds nothing and returns errSlackPublisherNotSubstituted, which run
// reports on its "build the publisher" line.
func refusingSlackPublisher(secret.Secret) (publisher.Publisher, error) {
	return nil, errSlackPublisherNotSubstituted
}

// loopbackSlackPublisher returns a newSlackPublisher that builds a real
// SlackWebhookPublisher posting to serverURL, a loopback test server, with no
// wait between messages. The path of the configured Webhook URL is kept, so
// the endpoint carries the same secret path run redacts: a failure that
// leaked part of the endpoint would show up in the redaction checks.
func loopbackSlackPublisher(t testing.TB, serverURL string) func(secret.Secret) (publisher.Publisher, error) {
	return func(webhookURL secret.Secret) (publisher.Publisher, error) {
		value, err := webhookURL.Reveal()
		if err != nil {
			return nil, err
		}
		parsed, err := url.Parse(value)
		if err != nil {
			return nil, errLoopbackWebhookURL
		}
		opts := publisher.SlackTestOptions{Timeout: loopbackPostTimeout}
		return publisher.NewSlackWebhookPublisherForLoopbackTest(t, opts, serverURL+parsed.EscapedPath()), nil
	}
}

// runChildMode runs the CLI in a child process started by startCLIChild and
// returns its exit code. The arguments after the program name are the CLI
// arguments; the configuration comes from the child's environment.
func runChildMode(mode string) int {
	var client llm.LLMClient
	var newSlack func(secret.Secret) (publisher.Publisher, error)
	switch mode {
	case childModeMain:
		main()
		return exitFailure // main exits; this line is never reached
	case childModeStall:
		client = stallingLLM{ready: os.Getenv(childReadyEnv)}
	case childModeIgnoreCtx:
		client = stallingLLM{ready: os.Getenv(childReadyEnv), ctxDone: os.Getenv(childCtxDoneEnv), ignoreCtx: true}
	case childModeSlackStall:
		client = validResponseLLM()
		newSlack = func(secret.Secret) (publisher.Publisher, error) {
			return stallingPublisher{ready: os.Getenv(childReadyEnv)}, nil
		}
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown child mode %q\n", mode)
		return childExitUnknownMode
	}
	d := testDeps(client)
	if newSlack != nil {
		d.newSlackPublisher = newSlack
	}
	return runWithSignals(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, d)
}

// cliChild is a CLI child process started by startCLIChild.
type cliChild struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   chan struct{}
}

// startCLIChild starts the test binary as a CLI child in mode with args. Its
// environment is the allowlisted parent variables plus env and the mode. A
// cleanup that kills and reaps the child is registered as soon as it starts.
func startCLIChild(t *testing.T, mode string, env map[string]string, args ...string) *cliChild {
	t.Helper()
	return startChildCommand(t, mode, env, exec.Command(os.Args[0], args...)) //nolint:gosec // the test binary itself, re-run as the CLI
}

// startCLIChildIgnoring is startCLIChild with signalName (a shell trap name
// such as INT) ignored when the child starts, as a shell does for a
// background job or nohup does for HUP. An ignored disposition survives exec.
func startCLIChildIgnoring(t *testing.T, signalName, mode string, env map[string]string, args ...string) *cliChild {
	t.Helper()
	script := "trap '' " + signalName + `; exec "$0" "$@"`
	return startChildCommand(t, mode, env, exec.Command("/bin/sh", append([]string{"-c", script, os.Args[0]}, args...)...)) //nolint:gosec // the test binary itself, re-run as the CLI
}

// startChildCommand starts cmd as a CLI child; see startCLIChild.
func startChildCommand(t *testing.T, mode string, env map[string]string, cmd *exec.Cmd) *cliChild {
	t.Helper()
	cmd.Env = childEnv(mode, env)
	cmd.WaitDelay = childWaitDelay
	c := &cliChild{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &c.stdout
	cmd.Stderr = &c.stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the CLI child: %v", err)
	}
	go func() {
		_ = cmd.Wait()
		close(c.done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-c.done
	})
	return c
}

// childEnv builds a child's environment from the allowlist, env, and mode.
func childEnv(mode string, env map[string]string) []string {
	var out []string
	for _, name := range childEnvAllowlist {
		if value, ok := os.LookupEnv(name); ok {
			out = append(out, name+"="+value)
		}
	}
	for name, value := range env {
		out = append(out, name+"="+value)
	}
	return append(out, childModeEnv+"="+mode)
}

// signal sends sig to the child.
func (c *cliChild) signal(t *testing.T, sig os.Signal) {
	t.Helper()
	if err := c.cmd.Process.Signal(sig); err != nil {
		t.Fatalf("send %v to the CLI child: %v", sig, err)
	}
}

// exited reports whether the child has been reaped.
func (c *cliChild) exited() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// wait waits up to exitBound for the child to exit and returns its state.
// Its output may be read afterwards.
func (c *cliChild) wait(t *testing.T) *os.ProcessState {
	t.Helper()
	select {
	case <-c.done:
		return c.cmd.ProcessState
	case <-time.After(exitBound):
		t.Fatalf("the CLI child did not exit within %v; its stderr is not shown because the child still writes it", exitBound)
		return nil
	}
}

// waitForFile waits up to readyBound for path to appear, naming it on
// failure.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(readyBound)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("%s did not appear within %v", path, readyBound)
}

// readPID reads a PID recorded by a fake yt-dlp.
func readPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // a PID file inside the test's temporary directory
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse the PID in %s: %v", path, err)
	}
	return pid
}

// waitForProcessGone waits up to bound for kill(pid, 0) to report ESRCH. An
// orphan stays a zombie until init reaps it, so absence is awaited, not
// checked once.
func waitForProcessGone(t *testing.T, pid int, bound time.Duration) {
	t.Helper()
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("process %d still exists after %v", pid, bound)
}

// snapshotTree records every entry under dir by relative path: a regular
// file's content, or the kind of anything else. A dir that cannot exist
// (missing, or below a non-directory) is an empty snapshot, so creating it is
// a change.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == dir && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)) {
				return filepath.SkipDir
			}
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		switch {
		case entry.Type().IsRegular():
			data, err := os.ReadFile(path) //nolint:gosec // a path inside the test's temporary directory
			if err != nil {
				return err
			}
			snapshot[rel] = "file:" + string(data)
		case entry.IsDir():
			snapshot[rel] = "dir"
		default:
			snapshot[rel] = "other:" + entry.Type().String()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return snapshot
}

// requireNoFile asserts path does not exist.
func requireNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s exists (error = %v), want it absent", path, err)
	}
}

// requireNonRoot fails the test when it runs as root, where permission
// failures cannot be produced. Such a run is an unsupported environment, so it
// fails instead of being skipped silently.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("unsupported test environment: permission-failure tests require a non-root user")
	}
}

// chmodForTest changes path's mode and restores the original mode at cleanup,
// so the test temp directory can be removed afterwards.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	original := info.Mode().Perm()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, original)
	})
}
