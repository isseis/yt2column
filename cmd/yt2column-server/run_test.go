//go:build test

package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/llm"
	llmtestutil "github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// errBuildRefused and errWriteRefused are what a constructor or a writer
// substituted by TestStartupFailures and TestDisplayFailureKeepsServing
// returns.
var (
	errBuildRefused = errors.New("the test refuses to build this component")
	errWriteRefused = errors.New("the test refuses to write")
)

// validTokenContent is a token file content the server accepts.
const validTokenContent = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// TestStartupFailures checks each step that can stop the start: the exit code,
// that run returns without serving, that standard error names the cause, and
// that no secret reaches either output. A rejected token file is left
// unchanged.
func TestStartupFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		// setup changes the environment or the deps before the run.
		setup func(t *testing.T, e *serverEnv, d *deps)
		want  int
		// wantStderr is a part of standard error that names the cause. run
		// returns only an exit code, so its standard error, which tells the
		// operator what to fix, is the observable contract matched here.
		wantStderr string
		// tokenShown is whether the run wrote the shared token to standard
		// output before it stopped.
		tokenShown bool
		// hint, when set, is a second part standard error must hold.
		hint string
		// configLines, when set, is how many configuration lines standard
		// error must hold: one per rejected variable.
		configLines int
	}{
		{
			name:       "an argument",
			args:       []string{"--port", "8765"},
			want:       exitUsage,
			wantStderr: "takes no arguments",
		},
		{
			// Which values config.LoadServer rejects is its own tests'
			// concern; here every rejection takes the same path, so one row
			// with two of them checks the exit code and one line each.
			name: "model and webhook missing",
			setup: func(_ *testing.T, e *serverEnv, _ *deps) {
				delete(e.vars, "YT2COLUMN_MODEL")
				delete(e.vars, "SLACK_WEBHOOK_URL")
			},
			want:        exitUsage,
			wantStderr:  "configuration: SLACK_WEBHOOK_URL",
			hint:        "configuration: YT2COLUMN_MODEL",
			configLines: 2,
		},
		{
			name: "token file readable by others",
			setup: func(t *testing.T, e *serverEnv, _ *deps) {
				writeTokenFile(t, e.tokenPath(t), validTokenContent, 0o644)
			},
			want:       exitUsage,
			wantStderr: "shared token: ",
			hint:       "delete it and restart",
		},
		{
			name: "LLM client not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newLLMClient = func(config.LLMConfig) (llm.LLMClient, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the LLM client",
			tokenShown: true,
		},
		{
			// The warning comes before any component is built.
			name: "GODEBUG enables http2debug",
			setup: func(_ *testing.T, e *serverEnv, d *deps) {
				e.vars["GODEBUG"] = "http2debug=1"
				d.newLLMClient = func(config.LLMConfig) (llm.LLMClient, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "warning: GODEBUG enables http2debug",
			tokenShown: true,
		},
		{
			name: "review LLM client not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the review LLM client",
			tokenShown: true,
		},
		{
			name: "writer not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newWriter = func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the article writer",
			tokenShown: true,
		},
		{
			name: "publisher not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newPublisher = func(secret.Secret) (publisher.Publisher, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the publisher",
			tokenShown: true,
		},
		{
			name: "notifier not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newNotifier = func(secret.Secret) (failure.Notifier, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the failure notifier",
			tokenShown: true,
		},
		{
			// The token is displayed before listening, so a start whose
			// listen fails still shows it.
			name: "port in use",
			setup: func(t *testing.T, e *serverEnv, _ *deps) {
				occupied, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("occupy a port: %v", err)
				}
				t.Cleanup(func() { _ = occupied.Close() })
				e.setPort(uint16(occupied.Addr().(*net.TCPAddr).Port)) //nolint:gosec // a TCP port fits in uint16
			},
			want:       exitFailure,
			wantStderr: "listen on 127.0.0.1:",
			tokenShown: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// No case listens, except "port in use", which sets its own
			// port, so the default port is left as it is.
			e := newServerEnv(t)
			// HOME is not changed by setup, so the path is derived from the
			// valid environment, before setup can make it invalid.
			tokenPath := e.tokenPath(t)
			d := testDeps(validResponseLLM())
			if tc.setup != nil {
				tc.setup(t, e, &d)
			}
			before, _ := os.ReadFile(tokenPath)

			// A run that wrongly started serving returns only when ctx ends,
			// with exitOK, so the deadline turns a hang into a failure.
			ctx, cancel := context.WithTimeout(context.Background(), exitBound)
			defer cancel()
			var stdout, stderr strings.Builder
			code := run(ctx, tc.args, e.lookup(), &stdout, &stderr, d)

			if code != tc.want {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, tc.want, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr does not contain %q:\n%s", tc.wantStderr, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.hint) {
				t.Errorf("stderr does not contain %q:\n%s", tc.hint, stderr.String())
			}
			if tc.configLines != 0 {
				if got := strings.Count(stderr.String(), programName+": configuration: "); got != tc.configLines {
					t.Errorf("configuration lines = %d, want %d:\n%s", got, tc.configLines, stderr.String())
				}
			}
			after, _ := os.ReadFile(tokenPath)
			token := string(after)
			switch {
			case tc.tokenShown:
				if len(before) != 0 || stdout.String() != token+"\n" {
					t.Errorf("stdout = %q, want the token %q on one line", stdout.String(), token)
				}
			default:
				if stdout.Len() != 0 {
					t.Errorf("stdout = %q, want empty", stdout.String())
				}
				if string(before) != token {
					t.Errorf("the token file changed from %q to %q", before, token)
				}
			}
			requireNoSecret(t, "stderr", stderr.String(), token)
		})
	}
}

// failingWriter rejects every write, so run's token display fails.
type failingWriter struct{}

// Write implements io.Writer.
func (failingWriter) Write([]byte) (int, error) { return 0, errWriteRefused }

// TestDisplayFailureKeepsServing checks that a start whose token display fails
// keeps serving: with a standard output that rejects every write, run reports
// the display failure on standard error (naming the token file and without the
// token) and still accepts a request, then stops with exitOK. It covers a start
// that reads an existing token (whose file must not change) and a first start
// (whose token file is created).
func TestDisplayFailureKeepsServing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		existing bool
	}{
		{name: "existing token file", existing: true},
		{name: "first start", existing: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newServerEnv(t)
			if tc.existing {
				writeTokenFile(t, e.tokenPath(t), validTokenContent, 0o600)
			}
			srv := startServerWithStdout(t, e, testDeps(validResponseLLM()), failingWriter{})
			// A first start reads the file it created (readToken fails when the
			// file is absent); an existing file must be unchanged below.
			token := e.readToken(t)
			mustPostArticle(t, e.port(t), token, requestBody(testValidRaw), 202)
			if code := srv.stop(t); code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, srv.stderr.String())
			}

			stderr := srv.stderr.String()
			// run returns only an exit code across the process boundary, so the
			// failure line is the observable contract; assert on it alone,
			// since the "listening" log line also names the token file. The
			// write error never crosses as a value, so its text is the only
			// form the reason takes here.
			failure := stderrLine(stderr, "writing the token to standard output failed")
			if failure == "" {
				t.Fatalf("stderr does not report the display failure:\n%s", stderr)
			}
			if !strings.Contains(failure, errWriteRefused.Error()) {
				t.Errorf("the display failure does not name the reason %q: %s", errWriteRefused, failure)
			}
			if !strings.Contains(failure, e.tokenPath(t)) {
				t.Errorf("the display failure does not name the token file %s: %s", e.tokenPath(t), failure)
			}
			if notice := stderrLine(stderr, "wrote the shared token to standard output"); notice != "" {
				t.Errorf("a failed display still wrote the success notice: %s", notice)
			}
			requireNoSecret(t, "stderr", stderr, token)

			if tc.existing {
				if got := e.readToken(t); got != validTokenContent {
					t.Errorf("the token file changed from %q to %q", validTokenContent, got)
				}
			}
		})
	}
}

// writeTokenFile creates the token file at path with content and mode.
func writeTokenFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil { // WriteFile's mode is masked by the umask
		t.Fatal(err)
	}
}

// TestTokenShownOnEveryStart checks that each start writes the shared token to
// standard output as one line: the first start creates the file, and a second
// start reads the same value and writes it too, leaving the file unchanged.
// Neither start writes the token to standard error, and the notice does not
// claim the token will not be shown again.
func TestTokenShownOnEveryStart(t *testing.T) {
	e := newServerEnv(t)
	d := testDeps(validResponseLLM())

	first := startServer(t, e, d)
	if code := first.stop(t); code != exitOK {
		t.Fatalf("first run exit code = %d, want %d\nstderr:\n%s", code, exitOK, first.stderr.String())
	}
	token := e.readToken(t)
	if first.stdout.String() != token+"\n" {
		t.Errorf("first stdout = %q, want the token file's content %q on one line", first.stdout.String(), token)
	}

	second := startServer(t, e, d)
	if code := second.stop(t); code != exitOK {
		t.Fatalf("second run exit code = %d, want %d\nstderr:\n%s", code, exitOK, second.stderr.String())
	}
	if second.stdout.String() != token+"\n" {
		t.Errorf("second stdout = %q, want the same token %q on one line", second.stdout.String(), token)
	}
	if got := e.readToken(t); got != token {
		t.Errorf("the token file changed from %q to %q", token, got)
	}
	// The display notice is the one standard-error line that mentions standard
	// output. Assert on that line alone: the "listening" log line also carries
	// the token path, so checking the whole output would pass even if the
	// notice omitted it.
	notice := stderrLine(second.stderr.String(), "standard output")
	if notice == "" {
		t.Fatalf("second stderr has no display notice:\n%s", second.stderr.String())
	}
	if strings.Contains(notice, "not shown again") {
		t.Errorf("the display notice claims the token is not shown again: %s", notice)
	}
	if !strings.Contains(notice, e.tokenPath(t)) {
		t.Errorf("the display notice does not name the token file %s: %s", e.tokenPath(t), notice)
	}
	requireNoSecret(t, "first stderr", first.stderr.String(), token)
	requireNoSecret(t, "second stderr", second.stderr.String(), token)
}

// stderrLine returns the first line of output that contains sub, or "" when
// there is none.
func stderrLine(output, sub string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	return ""
}

// TestClosedStdoutPipeKeepsServing checks that a start whose standard output is
// a pipe with no reader does not stop the process: SIGPIPE is process-wide, so
// the production main runs in a child process. The child's first display write
// reaches the closed pipe, and because main ignores SIGPIPE the write fails
// with EPIPE, run reports the display failure without the token, and the child
// keeps listening until a signal stops it with exitOK.
func TestClosedStdoutPipeKeepsServing(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create a pipe: %v", err)
	}
	// Close the reader before the child starts, so the child's first write
	// always reaches a closed pipe.
	if err := reader.Close(); err != nil {
		t.Fatalf("close the pipe's reader: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	e := newServerEnv(t)
	// startServerChildWithStdout returns only once the child listens, so
	// reaching the next line proves the display failure did not stop it.
	child := startServerChildWithStdout(t, childModeMain, e, nil, writer)
	child.signal(t, syscall.SIGTERM)
	if state := child.wait(t); state.ExitCode() != exitOK {
		t.Fatalf("exit status = %v, want exit code %d\nstderr:\n%s", state, exitOK, child.stderr.String())
	}
	stderr := child.stderr.String()
	// run returns only an exit code across the process boundary, so the
	// failure line on standard error is the observable contract; assert on it
	// alone, since the "listening" log line also names the token file.
	failure := stderrLine(stderr, "writing the token to standard output failed")
	if failure == "" {
		t.Fatalf("stderr does not report the display failure:\n%s", stderr)
	}
	if !strings.Contains(failure, e.tokenPath(t)) {
		t.Errorf("the display failure does not name the token file %s: %s", e.tokenPath(t), failure)
	}
	requireNoSecret(t, "child stderr", stderr, e.readToken(t))
}

// noticeSignal is a notifier that reports each notice on a channel.
type noticeSignal chan failure.Notice

// Notify implements failure.Notifier.
func (n noticeSignal) Notify(_ context.Context, notice failure.Notice) error {
	n <- notice
	return nil
}

// TestOutputsOmitSecrets checks that the server's log redacts every secret run
// hands it: the LLM client fails with an error that holds both API keys (the
// review step uses Claude, so both are configured), the webhook URL, and the
// shared token, and the "job failed" line that carries that error shows none
// of them.
func TestOutputsOmitSecrets(t *testing.T) {
	e := newServerEnv(t)
	e.vars["YT2COLUMN_REVIEW_PROVIDER"] = "claude"
	e.vars["YT2COLUMN_REVIEW_MODEL"] = "fake-review-model"
	e.vars["ANTHROPIC_API_KEY"] = testAnthropicKey
	e.vars["YT2COLUMN_CLAUDE_EFFORT"] = "low"
	tokenPath := e.tokenPath(t)
	leaking := funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		token, err := os.ReadFile(tokenPath) //nolint:gosec // a path inside the test's temporary directory
		if err != nil {
			return llm.GenerateResponse{}, err
		}
		return llm.GenerateResponse{}, errors.New("leak " + testAPIKey + " " + testAnthropicKey + " " + testWebhook + " " + string(token))
	})
	notices := make(noticeSignal, 1)
	d := testDeps(leaking)
	d.newNotifier = func(secret.Secret) (failure.Notifier, error) { return notices, nil }

	srv := startServer(t, e, d)
	token := e.readToken(t)
	mustPostArticle(t, e.port(t), token, requestBody(testValidRaw), 202)
	// The job logs its failure before it sends the notice.
	select {
	case <-notices:
	case <-time.After(readyBound):
		t.Fatal("the job did not fail")
	}
	if code := srv.stop(t); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	stderr := srv.stderr.String()
	if !strings.Contains(stderr, "job failed") || !strings.Contains(stderr, "leak [REDACTED]") {
		t.Fatalf("stderr does not hold the redacted job failure:\n%s", stderr)
	}
	requireNoSecret(t, "stderr", stderr, token)
}

// TestBuildsSlackPublisher checks that the production constructors build the
// publisher the CLI's --slack uses and the Slack failure notifier, and that a
// rejected webhook URL yields a nil interface rather than a typed nil.
func TestBuildsSlackPublisher(t *testing.T) {
	webhook, err := secret.New(testWebhook)
	if err != nil {
		t.Fatal(err)
	}
	d := productionDeps()
	pub, err := d.newPublisher(webhook)
	if err != nil {
		t.Fatalf("newPublisher: %v", err)
	}
	if _, ok := pub.(*publisher.SlackWebhookPublisher); !ok {
		t.Errorf("publisher is %T, want *publisher.SlackWebhookPublisher", pub)
	}
	notifier, err := d.newNotifier(webhook)
	if err != nil {
		t.Fatalf("newNotifier: %v", err)
	}
	if _, ok := notifier.(*publisher.SlackFailureNotifier); !ok {
		t.Errorf("notifier is %T, want *publisher.SlackFailureNotifier", notifier)
	}

	var zero secret.Secret
	if pub, err := d.newPublisher(zero); err == nil || pub != nil {
		t.Errorf("newPublisher(zero) = (%v, %v), want a nil interface and an error", pub, err)
	}
	if notifier, err := d.newNotifier(zero); err == nil || notifier != nil {
		t.Errorf("newNotifier(zero) = (%v, %v), want a nil interface and an error", notifier, err)
	}
}

// TestWriterOptionsMatchCLIDefaults checks that run builds the writer with
// writertestutil.DefaultOptions and the review client it built, the value
// the CLI's options with no step flags are checked against in cmd/yt2column.
// The publisher is refused, so run stops right after building the writer.
func TestWriterOptionsMatchCLIDefaults(t *testing.T) {
	e := newServerEnv(t)
	review := &llmtestutil.FakeLLMClient{}
	d := testDeps(validResponseLLM())
	d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) { return review, nil }
	var got []writer.Options
	d.newWriter = func(client llm.LLMClient, opts writer.Options) (writer.ArticleWriter, error) {
		got = append(got, opts)
		return writer.New(client, opts)
	}
	d.newPublisher = func(secret.Secret) (publisher.Publisher, error) { return nil, errBuildRefused }

	var stdout, stderr strings.Builder
	if code := run(t.Context(), nil, e.lookup(), &stdout, &stderr, d); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, stderr.String())
	}
	if len(got) != 1 {
		t.Fatalf("newWriter calls = %d, want 1", len(got))
	}
	if want := writertestutil.DefaultOptions(review); !reflect.DeepEqual(got[0], want) {
		t.Errorf("writer options = %+v, want %+v", got[0], want)
	}
}

// TestRunLogsListeningAndStop checks the lines run itself logs: the endpoint
// URL on both loopback addresses and the token file when it listens, and,
// when the stop begins, the longest wait for in-flight jobs and that a second
// signal exits at once.
func TestRunLogsListeningAndStop(t *testing.T) {
	e := newServerEnv(t)
	// An existing token, so the start reads it rather than creating one.
	writeTokenFile(t, e.tokenPath(t), validTokenContent, 0o600)
	srv := startServer(t, e, testDeps(validResponseLLM()))
	port := strconv.Itoa(int(e.port(t)))
	if code := srv.stop(t); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	stderr := srv.stderr.String()
	// The token path is logged on the listening line; assert on that line, not
	// on the whole output, where the display notice also carries the path.
	listening := stderrLine(stderr, "msg=listening")
	if listening == "" {
		t.Fatalf("stderr has no listening log line:\n%s", stderr)
	}
	if !strings.Contains(listening, e.tokenPath(t)) {
		t.Errorf("the listening log does not name the token file %s: %s", e.tokenPath(t), listening)
	}
	for _, want := range []string{
		"http://127.0.0.1:" + port + server.ArticlesPath,
		"http://[::1]:" + port + server.ArticlesPath,
		// Three LLM calls of at most 15 minutes each.
		"up to about 45 minutes",
		"send the signal again to exit at once",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, stderr)
		}
	}
}

// gapContext is a canceled context whose AfterFunc callbacks have not been
// scheduled yet: Err already reports the cancellation, but the callbacks are
// scheduled only when start is called. A real cancellation passes through this
// window between closing Done and scheduling its AfterFunc callbacks.
// context.AfterFunc hands its scheduling to a parent's AfterFunc method when
// the parent's Done is non-nil, so the window stays open until start.
type gapContext struct {
	context.Context
	done     chan struct{}
	schedule func()
}

func newGapContext() *gapContext {
	return &gapContext{Context: context.Background(), done: make(chan struct{})}
}

func (c *gapContext) Done() <-chan struct{} { return c.done }

func (c *gapContext) Err() error { return context.Canceled }

func (c *gapContext) AfterFunc(f func()) func() bool {
	c.schedule = f
	return func() bool { return true }
}

func (c *gapContext) start() { c.schedule() }

// TestAfterCancel checks that finish waits for the callback whenever ctx was
// canceled, including when the cancellation has not scheduled the callback
// yet, and that the callback never runs when ctx was not canceled.
func TestAfterCancel(t *testing.T) {
	const wait = 5 * time.Second

	t.Run("canceled before its callback is scheduled", func(t *testing.T) {
		ctx := newGapContext()
		var ran atomic.Bool
		finish := afterCancel(ctx, func() { ran.Store(true) }).finish
		finished := make(chan struct{})
		go func() {
			finish()
			close(finished)
		}()
		// Give finish time to act inside the window before the callback is
		// scheduled; a finish that unregisters the callback here returns
		// without it.
		select {
		case <-finished:
		case <-time.After(50 * time.Millisecond):
		}
		ctx.start()
		select {
		case <-finished:
		case <-time.After(wait):
			t.Fatal("finish did not return after the callback was scheduled")
		}
		if !ran.Load() {
			t.Error("finish returned without running the callback of a canceled context")
		}
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var ran atomic.Bool
		finish := afterCancel(ctx, func() { ran.Store(true) }).finish
		cancel()
		finish()
		if !ran.Load() {
			t.Error("finish returned before the callback of a canceled context ran")
		}
	})

	t.Run("not canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		var ran atomic.Bool
		finish := afterCancel(ctx, func() { ran.Store(true) }).finish
		finish()
		cancel()
		// A callback that was not unregistered starts in its own goroutine
		// after the cancellation; give it time to.
		time.Sleep(50 * time.Millisecond)
		if ran.Load() {
			t.Error("the callback ran although ctx was not canceled when finish was called")
		}
	})
}
