//go:build test

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/failure"
	"github.com/isseis/yt2column/internal/llm"
	llmtestutil "github.com/isseis/yt2column/internal/llm/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/udstest"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// configDocument is the document the argument diagnostic points at for the
// configuration: the English README, since the diagnostic is in English.
const configDocument = "README.en.md"

// errBuildRefused is what a constructor substituted by TestStartupFailures
// returns.
var errBuildRefused = errors.New("the test refuses to build this component")

// TestStartupFailures checks each step that can stop the start: the exit code,
// that run returns without serving, that standard error names the cause, that
// no secret reaches it, and that the socket path is left as it was: no socket
// is created where none was, and an existing one is not removed.
func TestStartupFailures(t *testing.T) {
	// The argument diagnostic points the user at the configuration document,
	// which must exist.
	if _, err := os.Stat(filepath.Join(repositoryRoot, configDocument)); err != nil {
		t.Errorf("the diagnostic points at %s, which does not exist: %v", configDocument, err)
	}
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
			hint:       configDocument,
		},
		{
			// Which values config.LoadServer rejects is its own tests'
			// concern; here every rejection takes the same path, so one row
			// with two of them checks the exit code and one line each. The
			// provider is chosen with an empty model, so its key is still
			// required: two independent rejections.
			name: "LLM model empty and API key missing",
			setup: func(_ *testing.T, e *serverEnv, _ *deps) {
				e.vars["YT2COLUMN_LLM"] = "deepseek/"
				delete(e.vars, "DEEPSEEK_API_KEY")
			},
			want:        exitUsage,
			wantStderr:  "configuration: YT2COLUMN_LLM",
			hint:        "configuration: DEEPSEEK_API_KEY",
			configLines: 2,
		},
		{
			// A retired variable stops the server before it listens, even
			// with an otherwise valid configuration.
			name: "a retired LLM variable",
			setup: func(_ *testing.T, e *serverEnv, _ *deps) {
				e.vars[config.RetiredLLMVarsForTest()[0]] = ""
			},
			want:        exitUsage,
			wantStderr:  "configuration: " + config.RetiredLLMVarErrorForTest(config.RetiredLLMVarsForTest()[0]),
			configLines: 1,
		},
		{
			// The last retired variable, YT2COLUMN_CLAUDE_EFFORT, also stops
			// the server before it listens (architecture section 7.3).
			name: "the retired Claude effort variable",
			setup: func(_ *testing.T, e *serverEnv, _ *deps) {
				retired := config.RetiredLLMVarsForTest()
				e.vars[retired[len(retired)-1]] = "low"
			},
			want:        exitUsage,
			wantStderr:  "configuration: " + config.RetiredLLMVarErrorForTest(config.RetiredLLMVarsForTest()[len(config.RetiredLLMVarsForTest())-1]),
			configLines: 1,
		},
		{
			name: "HOME unset",
			setup: func(_ *testing.T, e *serverEnv, _ *deps) {
				delete(e.vars, "HOME")
			},
			want:        exitUsage,
			wantStderr:  "configuration: HOME",
			configLines: 1,
		},
		{
			// HOME is padded so the derived path is exactly one byte over
			// this operating system's limit (sun_path minus its NUL), written
			// out so a change to the limit fails this row.
			name: "socket path one byte over the limit",
			setup: func(_ *testing.T, e *serverEnv, _ *deps) {
				limit := 103
				if runtime.GOOS == "linux" {
					limit = 107
				}
				homeLen := limit + 1 - len(config.SocketPathTailForTest(runtime.GOOS))
				e.vars["HOME"] += "/" + strings.Repeat("h", homeLen-len(e.vars["HOME"])-1)
			},
			want:        exitUsage,
			wantStderr:  "configuration: HOME",
			configLines: 1,
		},
		{
			name: "LLM client not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newLLMClient = func(config.LLMConfig) (llm.LLMClient, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the LLM client",
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
		},
		{
			name: "review LLM client not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the review LLM client",
		},
		{
			name: "writer not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newWriter = func(llm.LLMClient, writer.Options) (writer.ProgressWriter, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the article writer",
		},
		{
			name: "publisher not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newPublisher = func(secret.Secret) (server.WebhookPublisher, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the publisher",
		},
		{
			name: "notifier not built",
			setup: func(_ *testing.T, _ *serverEnv, d *deps) {
				d.newNotifier = func(secret.Secret) (failure.Notifier, error) { return nil, errBuildRefused }
			},
			want:       exitUsage,
			wantStderr: "build the failure notifier",
		},
		{
			name: "socket directory with a mode other than 0700",
			setup: func(t *testing.T, e *serverEnv, _ *deps) {
				if err := os.MkdirAll(filepath.Dir(e.socketPath(t)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(filepath.Dir(e.socketPath(t)), 0o755); err != nil { // MkdirAll's mode is masked by the umask
					t.Fatal(err)
				}
			},
			want:       exitFailure,
			wantStderr: "does not have mode 0700",
			hint:       "nothing was repaired or removed",
		},
		{
			name: "another server holds the start lock",
			setup: func(t *testing.T, e *serverEnv, _ *deps) {
				startServer(t, e, testDeps(validResponseLLM()))
			},
			want:       exitFailure,
			wantStderr: "another server holds the socket directory lock",
			hint:       "another server is running",
		},
		{
			// A process that listens on the socket path without the start
			// lock, which no server does.
			name: "another process listens on the socket path",
			setup: func(t *testing.T, e *serverEnv, _ *deps) {
				if err := os.MkdirAll(filepath.Dir(e.socketPath(t)), 0o700); err != nil {
					t.Fatal(err)
				}
				udstest.Listen(t, e.socketPath(t))
			},
			want:       exitFailure,
			wantStderr: "another process accepts connections on the socket path",
			hint:       "stop the process that listens on the socket",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newServerEnv(t)
			// The path is derived from the valid environment, before setup can
			// make it invalid.
			socket := e.socketPath(t)
			d := testDeps(validResponseLLM())
			if tc.setup != nil {
				tc.setup(t, e, &d)
			}
			_, err := os.Lstat(socket)
			existed := err == nil

			// A run that wrongly started serving returns only when ctx ends,
			// with exitOK, so the deadline turns a hang into a failure.
			ctx, cancel := context.WithTimeout(context.Background(), exitBound)
			defer cancel()
			var stderr strings.Builder
			code := run(ctx, tc.args, e.lookup(), &stderr, d)

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
			if _, err := os.Lstat(socket); (err == nil) != existed {
				t.Errorf("the socket path existed before the run: %v, after it: %v", existed, err == nil)
			}
			requireNoSecret(t, "stderr", stderr.String())
		})
	}
}

// gatedLLM is an LLM client whose calls signal entered and then wait until
// release is closed.
type gatedLLM struct {
	entered chan struct{}
	release chan struct{}
}

// Generate implements llm.LLMClient.
func (g gatedLLM) Generate(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
	g.entered <- struct{}{}
	<-g.release
	return validResponse(), nil
}

// TestStartDuringShutdownRefused checks that a server started while another is
// shutting down (its listener closed and its socket gone, but its job still
// running) stops with exitFailure instead of serving beside it: the first
// server holds the start lock until its jobs have finished.
func TestStartDuringShutdownRefused(t *testing.T) {
	e := newServerEnv(t)
	gate := gatedLLM{entered: make(chan struct{}, 1), release: make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	t.Cleanup(release)
	first := startServer(t, e, testDeps(gate))
	socket := e.socketPath(t)
	mustPostArticle(t, socket, requestBody(testValidRaw), 202)
	select {
	case <-gate.entered:
	case <-time.After(readyBound):
		t.Fatal("the job did not start")
	}

	first.cancel()
	// Closing the listener removes the socket: from then on the first server
	// only waits for its job.
	deadline := time.Now().Add(readyBound)
	for _, err := os.Lstat(socket); err == nil; _, err = os.Lstat(socket) {
		if time.Now().After(deadline) {
			t.Fatalf("the socket %s was not removed within %v of the cancellation", socket, readyBound)
		}
		time.Sleep(pollInterval)
	}

	// A second server that wrongly started would serve until this deadline
	// and return exitOK, so the deadline turns that into a failure.
	ctx, cancel := context.WithTimeout(context.Background(), exitBound)
	defer cancel()
	var stderr strings.Builder
	if code := run(ctx, nil, e.lookup(), &stderr, testDeps(validResponseLLM())); code != exitFailure {
		t.Fatalf("second run exit code = %d, want %d\nstderr:\n%s", code, exitFailure, stderr.String())
	}
	if !strings.Contains(stderr.String(), "another server holds the socket directory lock") {
		t.Errorf("stderr does not name the held start lock:\n%s", stderr.String())
	}

	release()
	if code := first.stop(t); code != exitOK {
		t.Fatalf("first run exit code = %d, want %d\nstderr:\n%s", code, exitOK, first.stderr.String())
	}
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

// noticeSignal is a notifier that reports each notice on a channel.
type noticeSignal chan failure.Notice

// Notify implements failure.Notifier.
func (n noticeSignal) Notify(_ context.Context, notice failure.Notice) error {
	n <- notice
	return nil
}

// TestOutputsOmitSecrets checks that the server's log redacts every secret run
// hands it: the LLM client fails with an error that holds both API keys (the
// review step uses Claude, so both are configured) and the webhook URL, and
// the "job failed" line that carries that error shows none of them.
func TestOutputsOmitSecrets(t *testing.T) {
	e := newServerEnv(t)
	e.vars["YT2COLUMN_REVIEW_LLM"] = "claude/fake-review-model"
	e.vars["ANTHROPIC_API_KEY"] = testAnthropicKey
	leaking := funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return llm.GenerateResponse{}, errors.New("leak " + testAPIKey + " " + testAnthropicKey + " " + testWebhook)
	})
	notices := make(noticeSignal, 1)
	d := testDeps(leaking)
	d.newNotifier = func(secret.Secret) (failure.Notifier, error) { return notices, nil }

	srv := startServer(t, e, d)
	mustPostArticle(t, e.socketPath(t), requestBody(testValidRaw), 202)
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
	requireNoSecret(t, "stderr", stderr)
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
	d.newWriter = func(client llm.LLMClient, opts writer.Options) (writer.ProgressWriter, error) {
		got = append(got, opts)
		return writer.NewProgressWriter(client, opts)
	}
	d.newPublisher = func(secret.Secret) (server.WebhookPublisher, error) { return nil, errBuildRefused }

	var stderr strings.Builder
	if code := run(t.Context(), nil, e.lookup(), &stderr, d); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, stderr.String())
	}
	if len(got) != 1 {
		t.Fatalf("newWriter calls = %d, want 1", len(got))
	}
	if want := writertestutil.DefaultOptions(review); !reflect.DeepEqual(got[0], want) {
		t.Errorf("writer options = %+v, want %+v", got[0], want)
	}
}

// TestStartsWithoutWebhook checks that a server whose SLACK_WEBHOOK_URL is
// unset starts, reports in one warning after the listening line that no Webhook
// is configured and names what that refuses, rejects /articles on its socket,
// and never builds the publisher or the failure notifier.
func TestStartsWithoutWebhook(t *testing.T) {
	e := newServerEnv(t)
	delete(e.vars, "SLACK_WEBHOOK_URL")
	publisherCalls, notifierCalls := 0, 0
	d := testDeps(validResponseLLM())
	d.newPublisher = func(secret.Secret) (server.WebhookPublisher, error) {
		publisherCalls++
		return nopPublisher{}, nil
	}
	d.newNotifier = func(secret.Secret) (failure.Notifier, error) {
		notifierCalls++
		return nopNotifier{}, nil
	}

	srv := startServer(t, e, d)
	// The server run built, not a hand-built one, refuses what needs a Webhook.
	if resp := mustPostArticle(t, e.socketPath(t), requestBody(testValidRaw), http.StatusServiceUnavailable); resp.Reason != "webhook_not_configured" {
		t.Errorf("reason = %q, want webhook_not_configured", resp.Reason)
	}
	if code := srv.stop(t); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, srv.stderr.String())
	}
	stderr := srv.stderr.String()
	listening := stderrLine(stderr, "msg=listening")
	if listening == "" {
		t.Fatalf("stderr has no listening log line:\n%s", stderr)
	}
	if !strings.Contains(listening, "webhookConfigured=false") {
		t.Errorf("the listening log does not report no Webhook: %s", listening)
	}
	// The rejection the test causes logs a warning of its own, so the startup
	// warning is counted by its own text.
	const startupWarning = "no Webhook is configured"
	if got := strings.Count(stderr, startupWarning); got != 1 {
		t.Errorf("startup warning lines = %d, want 1:\n%s", got, stderr)
	}
	warning := stderrLine(stderr, startupWarning)
	if !strings.Contains(warning, "level=WARN") {
		t.Errorf("the startup line is not a warning: %s", warning)
	}
	if !strings.Contains(warning, "webhook_not_configured") {
		t.Errorf("the warning does not name webhook_not_configured: %s", warning)
	}
	if strings.Index(stderr, "msg=listening") > strings.Index(stderr, startupWarning) {
		t.Errorf("the warning comes before the listening line:\n%s", stderr)
	}
	if publisherCalls != 0 || notifierCalls != 0 {
		t.Errorf("constructors called without a Webhook: publisher=%d notifier=%d", publisherCalls, notifierCalls)
	}
	requireNoSecret(t, "stderr", stderr)
}

// TestRunLogsListeningAndStop checks the lines run itself logs: the socket
// path and the configured Webhook when it listens (with no warning about the
// Webhook), and, when the stop begins, the longest wait for in-flight jobs and
// that a second signal exits at once.
func TestRunLogsListeningAndStop(t *testing.T) {
	e := newServerEnv(t)
	srv := startServer(t, e, testDeps(validResponseLLM()))
	if code := srv.stop(t); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	stderr := srv.stderr.String()
	listening := stderrLine(stderr, "msg=listening")
	if listening == "" {
		t.Fatalf("stderr has no listening log line:\n%s", stderr)
	}
	if !strings.Contains(listening, e.socketPath(t)) {
		t.Errorf("the listening log does not name the socket %s: %s", e.socketPath(t), listening)
	}
	if !strings.Contains(listening, "webhookConfigured=true") {
		t.Errorf("the listening log does not report the configured Webhook: %s", listening)
	}
	if warning := stderrLine(stderr, "no Webhook is configured"); warning != "" {
		t.Errorf("stderr warns that no Webhook is configured although one is: %s", warning)
	}
	for _, want := range []string{
		// Three LLM calls of at most 15 minutes each.
		"up to about 45 minutes",
		"the job API's in-flight generations are canceled",
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
