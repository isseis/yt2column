//go:build test

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
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

// errBuildRefused is what a constructor substituted by TestStartupFailures
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
		// tokenShown is whether the run created the token and wrote it to
		// standard output before it stopped.
		tokenShown bool
		// hint, when set, is a second part standard error must hold.
		hint string
		// configLines, when set, is how many configuration lines standard
		// error must hold: one per rejected variable.
		configLines int
		// stdoutFails makes standard output reject every write.
		stdoutFails bool
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
			// The token is kept in the file, and the run stops rather than
			// claim it was shown.
			name:        "token not written to stdout",
			stdoutFails: true,
			want:        exitUsage,
			wantStderr:  "writing the created token to standard output failed",
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
			// The token is created before listening, so a start whose listen
			// fails still shows it once.
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
			var out io.Writer = &stdout
			if tc.stdoutFails {
				out = failingWriter{}
			}
			code := run(ctx, tc.args, e.lookup(), out, &stderr, d)

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
			case tc.stdoutFails:
				if len(after) == 0 {
					t.Error("the token file was not created")
				}
			case tc.tokenShown:
				if len(before) != 0 || stdout.String() != token+"\n" {
					t.Errorf("stdout = %q, want the created token %q on one line", stdout.String(), token)
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

// failingWriter rejects every write.
type failingWriter struct{}

// Write implements io.Writer.
func (failingWriter) Write([]byte) (int, error) { return 0, errWriteRefused }

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

// TestTokenShownOnlyWhenCreated checks that the first start writes the token it
// creates to standard output as one line, and that a second start with the
// same token file writes nothing there and keeps the file. Neither start writes
// the token to standard error.
func TestTokenShownOnlyWhenCreated(t *testing.T) {
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
	if second.stdout.Len() != 0 {
		t.Errorf("second stdout = %q, want empty", second.stdout.String())
	}
	if got := e.readToken(t); got != token {
		t.Errorf("the token file changed from %q to %q", token, got)
	}
	requireNoSecret(t, "first stderr", first.stderr.String(), token)
	requireNoSecret(t, "second stderr", second.stderr.String(), token)
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
	// An existing token, so no "created" line also names the file's path.
	writeTokenFile(t, e.tokenPath(t), validTokenContent, 0o600)
	srv := startServer(t, e, testDeps(validResponseLLM()))
	port := strconv.Itoa(int(e.port(t)))
	if code := srv.stop(t); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	stderr := srv.stderr.String()
	for _, want := range []string{
		"http://127.0.0.1:" + port + server.ArticlesPath,
		"http://[::1]:" + port + server.ArticlesPath,
		e.tokenPath(t),
		// Three LLM calls of at most 15 minutes each.
		"up to about 45 minutes",
		"send the signal again to exit at once",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, stderr)
		}
	}
}
