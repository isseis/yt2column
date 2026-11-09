//go:build test

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
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
		// error must hold: one per rejection.
		configLines int
		// fileSetup, when set, changes the configuration file at path after
		// it was written from the serverEnv.
		fileSetup func(t *testing.T, path string)
		// restore, when set, undoes fileSetup's change of a directory's
		// mode once the file's state is recorded, so the socket directory
		// below it can be checked.
		restore func(t *testing.T, path string)
		// configError marks a configuration error: the run must also leave
		// the socket directory uncreated and the configuration file and its
		// directory as they were.
		configError bool
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
				e.file["YT2COLUMN_LLM"] = "deepseek/"
				delete(e.file, "DEEPSEEK_API_KEY")
			},
			want:        exitUsage,
			wantStderr:  "configuration: configuration file key YT2COLUMN_LLM",
			hint:        "configuration: configuration file key DEEPSEEK_API_KEY",
			configLines: 2,
			configError: true,
		},
		{
			name:        "configuration file missing",
			fileSetup:   func(t *testing.T, path string) { removeFile(t, path) },
			want:        exitUsage,
			wantStderr:  "does not exist",
			hint:        "yt2column-server config init",
			configLines: 1,
			configError: true,
		},
		{
			name: "configuration file is a symbolic link",
			fileSetup: func(t *testing.T, path string) {
				target := path + ".target"
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
			want:        exitUsage,
			wantStderr:  "is a symbolic link",
			configLines: 1,
			configError: true,
		},
		{
			name: "configuration file is a directory",
			fileSetup: func(t *testing.T, path string) {
				removeFile(t, path)
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			want:        exitUsage,
			wantStderr:  "is not a regular file",
			configLines: 1,
			configError: true,
		},
		{
			name:        "configuration file with mode 0644",
			fileSetup:   func(t *testing.T, path string) { chmodFile(t, path, 0o644) },
			want:        exitUsage,
			wantStderr:  "has mode 0644",
			hint:        "chmod 600",
			configLines: 1,
			configError: true,
		},
		{
			name: "configuration file unreadable",
			fileSetup: func(t *testing.T, path string) {
				if os.Geteuid() == 0 {
					t.Skip("root reads a file below a directory it cannot search")
				}
				dir := filepath.Dir(path)
				chmodFile(t, dir, 0o000)
				t.Cleanup(func() { chmodFile(t, dir, 0o700) })
			},
			restore:     func(t *testing.T, path string) { chmodFile(t, filepath.Dir(path), 0o700) },
			want:        exitUsage,
			wantStderr:  "could not be read",
			configLines: 1,
			configError: true,
		},
		{
			name: "configuration file line rejected",
			fileSetup: func(t *testing.T, path string) {
				// Writing an existing file keeps its mode.
				if err := os.WriteFile(path, []byte("export ANTHROPIC_API_KEY=x\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want:        exitUsage,
			wantStderr:  ": line 1: ",
			configLines: 1,
			configError: true,
		},
		{
			name: "configuration file over the size limit",
			fileSetup: func(t *testing.T, path string) {
				appendFile(t, path, "#"+strings.Repeat("x", config.ServerFileMaxSizeForTest())+"\n")
			},
			want:        exitUsage,
			wantStderr:  "larger than",
			configLines: 1,
			configError: true,
		},
		{
			name: "HOME unset",
			setup: func(_ *testing.T, e *serverEnv, _ *deps) {
				delete(e.vars, "HOME")
			},
			want:        exitUsage,
			wantStderr:  "configuration: HOME",
			configLines: 1,
			configError: true,
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
			configError: true,
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
			// The socket directories a configuration error must not create:
			// under the valid HOME, and under HOME as setup leaves it.
			socketDirs := []string{filepath.Dir(socket)}
			d := testDeps(validResponseLLM())
			if tc.setup != nil {
				tc.setup(t, e, &d)
			}
			e.writeConfig(t)
			// The socket directory the run would create is derived from HOME
			// as setup left it, which may differ from the valid one above.
			var before, unchanged configState
			if home := e.vars["HOME"]; filepath.IsAbs(home) {
				socketDirs = append(socketDirs, filepath.Dir(e.socketPath(t)))
				unchanged = readConfigState(t, home)
				if tc.fileSetup != nil {
					tc.fileSetup(t, config.ServerFilePathForTest(home))
				}
				before = readConfigState(t, home)
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
			if tc.configError {
				if home := e.vars["HOME"]; filepath.IsAbs(home) {
					if after := readConfigState(t, home); after != before {
						t.Errorf("the configuration file changed: before %+v, after %+v", before, after)
					}
					if tc.restore != nil {
						// The file below the directory setup made unsearchable
						// is compared once the directory's mode is restored.
						tc.restore(t, config.ServerFilePathForTest(home))
						if after := readConfigState(t, home); after != unchanged {
							t.Errorf("the configuration file changed: before %+v, after %+v", unchanged, after)
						}
					}
				}
				// The socket directory holds the start lock and the socket,
				// so its absence covers both.
				for _, dir := range socketDirs {
					if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("the socket directory %s exists after a configuration error: %v", dir, err)
					}
				}
			}
			requireNoSecret(t, "stderr", stderr.String())
		})
	}
}

// configState is what a configuration error must leave unchanged: the
// configuration file's mode (and, through a symbolic link, its target's) and
// content, and its directory's mode. A part that cannot be read is recorded
// as such.
type configState struct {
	fileMode, targetMode, dirMode os.FileMode
	content                       string
	unreadable                    bool
}

// readConfigState records the configuration file under home and its
// directory.
func readConfigState(t *testing.T, home string) configState {
	t.Helper()
	path := config.ServerFilePathForTest(home)
	var st configState
	if info, err := os.Lstat(filepath.Dir(path)); err == nil {
		st.dirMode = info.Mode()
	}
	if info, err := os.Lstat(path); err == nil {
		st.fileMode = info.Mode()
	}
	if info, err := os.Stat(path); err == nil {
		st.targetMode = info.Mode()
	}
	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // the test's own configuration file
		st.content = string(data)
	} else {
		st.unreadable = true
	}
	return st
}

// removeFile removes path.
func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// chmodFile sets path's mode.
func chmodFile(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// appendFile appends text to the file at path.
func appendFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0) //nolint:gosec // the test's own configuration file
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestFormatRejectionLines checks that a file with more rejected lines than
// are reported one by one yields exactly that many configuration lines plus
// the one that counts the rest.
func TestFormatRejectionLines(t *testing.T) {
	e := newServerEnv(t)
	e.writeConfig(t)
	appendFile(t, config.ServerFilePathForTest(e.vars["HOME"]), strings.Repeat("not a setting\n", 25))
	var stderr strings.Builder
	if code := run(t.Context(), nil, e.lookup(), &stderr, testDeps(validResponseLLM())); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, stderr.String())
	}
	if got := strings.Count(stderr.String(), programName+": configuration: "); got != 21 {
		t.Errorf("configuration lines = %d, want 21:\n%s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "5 more lines were rejected") {
		t.Errorf("stderr does not count the 5 lines not reported:\n%s", stderr.String())
	}
}

// TestModeRejectionChmodCommand checks that the command the mode refusal
// names runs as displayed, under a HOME with a space in it, and gives the
// file mode 0600.
func TestModeRejectionChmodCommand(t *testing.T) {
	e := newServerEnv(t)
	e.vars["HOME"] = filepath.Join(e.vars["HOME"], "a b")
	e.writeConfig(t)
	path := config.ServerFilePathForTest(e.vars["HOME"])
	chmodFile(t, path, 0o644)
	var stderr strings.Builder
	if code := run(t.Context(), nil, e.lookup(), &stderr, testDeps(validResponseLLM())); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, stderr.String())
	}
	line := stderrLine(stderr.String(), "has mode")
	_, command, found := strings.Cut(line, "; run ")
	if !found {
		t.Fatalf("the mode refusal names no command: %q", line)
	}
	if out, err := exec.Command("sh", "-c", command).CombinedOutput(); err != nil { //nolint:gosec // the command the server printed, the subject of the test
		t.Fatalf("sh -c %q: %v\n%s", command, err, out)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode after the command = %04o, want 0600", got)
	}
}

// TestServerIgnoresEnvSettings checks that the six keys and the retired LLM
// variables in the environment, empty or not, neither stop the server nor
// show up on its standard error, and that a key only the environment holds
// counts as not set in the configuration file.
func TestServerIgnoresEnvSettings(t *testing.T) {
	names := append(config.ServerFileKeysForTest(),
		"YT2COLUMN_LLM_PROVIDER", "YT2COLUMN_MODEL", "YT2COLUMN_REVIEW_PROVIDER",
		"YT2COLUMN_REVIEW_MODEL", "YT2COLUMN_CLAUDE_EFFORT",
	)
	for _, value := range []string{"", "x"} {
		t.Run(fmt.Sprintf("every variable %q", value), func(t *testing.T) {
			e := newServerEnv(t)
			for _, name := range names {
				e.vars[name] = value
			}
			srv := startServer(t, e, testDeps(validResponseLLM()))
			if code := srv.stop(t); code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, srv.stderr.String())
			}
			for _, name := range names {
				if strings.Contains(srv.stderr.String(), name) {
					t.Errorf("stderr names %s:\n%s", name, srv.stderr.String())
				}
			}
		})
	}
	t.Run("API key only in the environment", func(t *testing.T) {
		e := newServerEnv(t)
		e.file = map[string]string{}
		e.vars["ANTHROPIC_API_KEY"] = testAnthropicKey
		e.writeConfig(t)
		// A run that wrongly took the key from the environment would serve
		// until the deadline and return exitOK.
		ctx, cancel := context.WithTimeout(context.Background(), exitBound)
		defer cancel()
		var stderr strings.Builder
		if code := run(ctx, nil, e.lookup(), &stderr, testDeps(validResponseLLM())); code != exitUsage {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, stderr.String())
		}
		if want := "configuration: configuration file key ANTHROPIC_API_KEY: is not set"; !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr does not contain %q:\n%s", want, stderr.String())
		}
		if got := strings.Count(stderr.String(), programName+": configuration: "); got != 1 {
			t.Errorf("configuration lines = %d, want 1:\n%s", got, stderr.String())
		}
		requireNoSecret(t, "stderr", stderr.String())
	})
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

// gatedPublisher is a Webhook publisher whose posts signal entered and then
// wait until release is closed.
type gatedPublisher struct {
	entered chan struct{}
	release chan struct{}
}

// PostMessages implements server.WebhookPublisher.
func (g gatedPublisher) PostMessages(_ context.Context, m publisher.SlackMessages, from int, delivered func(int)) error {
	g.entered <- struct{}{}
	<-g.release
	for i := from; i < m.Len(); i++ {
		delivered(i)
	}
	return nil
}

// TestStartDuringShutdownRefused checks that a server started while another is
// shutting down (its listener closed and its socket gone, but its post job
// still running) stops with exitFailure instead of serving beside it: the
// first server holds the start lock until its jobs have finished.
func TestStartDuringShutdownRefused(t *testing.T) {
	e := newServerEnv(t)
	gate := gatedPublisher{entered: make(chan struct{}, 1), release: make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	t.Cleanup(release)
	d := testDeps(validResponseLLM())
	d.newPublisher = func(secret.Secret) (server.WebhookPublisher, error) { return gate, nil }
	first := startServer(t, e, d)
	socket := e.socketPath(t)
	generateAndPublish(t, socket)
	select {
	case <-gate.entered:
	case <-time.After(readyBound):
		t.Fatal("the post job did not start")
	}

	first.cancel()
	// Closing the listener removes the socket: from then on the first server
	// only waits for its post job.
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

// TestOutputsOmitSecrets checks that the server's log redacts every secret run
// hands it: the LLM client fails with an error that holds both API keys (the
// review step uses Claude, so both are configured) and the webhook URL, and
// the "generation job failed" line that carries that error shows none of
// them.
func TestOutputsOmitSecrets(t *testing.T) {
	e := newServerEnv(t)
	e.file["YT2COLUMN_REVIEW_LLM"] = "claude/fake-review-model"
	e.file["ANTHROPIC_API_KEY"] = testAnthropicKey
	leaking := funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return llm.GenerateResponse{}, errors.New("leak " + testAPIKey + " " + testAnthropicKey + " " + testWebhook)
	})

	srv := startServer(t, e, testDeps(leaking))
	doJobs(t, e.socketPath(t), http.MethodPost, server.JobsPath, strings.NewReader(requestBody(testValidRaw)), http.StatusAccepted)
	// The record fails before the job logs the failure, and a stop that began
	// in between would log the cancellation instead, so the test waits for
	// the line itself.
	line := srv.waitStderr(t, "generation job failed")
	if code := srv.stop(t); code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}

	if !strings.Contains(line, "leak [REDACTED]") {
		t.Fatalf("the failure line does not hold the redacted error: %s", line)
	}
	requireNoSecret(t, "stderr", srv.stderr.String())
}

// TestBuildsSlackPublisher checks that the production constructor builds the
// SlackWebhookPublisher the server posts with, and that a rejected webhook URL
// yields a nil interface rather than a typed nil.
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

	var zero secret.Secret
	if pub, err := d.newPublisher(zero); err == nil || pub != nil {
		t.Errorf("newPublisher(zero) = (%v, %v), want a nil interface and an error", pub, err)
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

	e.writeConfig(t)
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
// is configured and names what that refuses, rejects the publish operation on
// its socket, and never builds the publisher.
func TestStartsWithoutWebhook(t *testing.T) {
	e := newServerEnv(t)
	delete(e.file, "SLACK_WEBHOOK_URL")
	publisherCalls := 0
	d := testDeps(validResponseLLM())
	d.newPublisher = func(secret.Secret) (server.WebhookPublisher, error) {
		publisherCalls++
		return nopPublisher{}, nil
	}

	srv := startServer(t, e, d)
	// The server run built, not a hand-built one, refuses what needs a Webhook.
	target := server.JobsPath + "/" + strings.Repeat("0", 64) + server.PublishSuffix
	if resp := doJobs(t, e.socketPath(t), http.MethodPost, target, nil, http.StatusServiceUnavailable); resp.Reason != "webhook_not_configured" {
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
	if publisherCalls != 0 {
		t.Errorf("the publisher was built %d times without a Webhook", publisherCalls)
	}
	requireNoSecret(t, "stderr", stderr)
}

// TestRunLogsListeningAndStop checks the lines run itself logs: the socket
// path and the configured Webhook when it listens (with no warning about the
// Webhook), and, when the stop begins, that the generations are canceled, the
// longest wait for the posts, and that a second signal exits at once.
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
		// Ten messages of at most 30 seconds each, a second apart: 309 s.
		"up to about 5 minutes",
		"the in-flight generations are canceled",
		"the posts sent from the viewer are allowed to finish",
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

// TestApproxMinutes checks the minutes the stop notice names: the nearest
// minute, half a minute rounded up, and never 0.
func TestApproxMinutes(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want int
	}{
		{0, 1},
		{29 * time.Second, 1},
		{30 * time.Second, 1},
		{89 * time.Second, 1},
		{90 * time.Second, 2},
		{150 * time.Second, 3},
		{309 * time.Second, 5},
		{330 * time.Second, 6},
	} {
		if got := approxMinutes(tc.d); got != tc.want {
			t.Errorf("approxMinutes(%v) = %d, want %d", tc.d, got, tc.want)
		}
	}
}

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
