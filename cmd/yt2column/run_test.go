//go:build test

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/transcript"
	transcripttestutil "github.com/isseis/yt2column/internal/transcript/testutil"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

const (
	runVideoID   = "2tcCWM-sRBw"
	runVideoURL  = "https://www.youtube.com/watch?v=" + runVideoID
	otherVideoID = "dQw4w9WgXcQ"

	// testAPIKey and testWebhook are distinctive secret values. Their last
	// eight characters differ from every other string a run writes.
	testAPIKey  = "sk-unit-APIKEYVALUE-0123456789-TAIL8KEY"
	testWebhook = "https://hooks.slack.com" + testWebhookPath
	// testWebhookPath is the path of testWebhook; a line holding it alone
	// must be redacted too. webhookPathMarker is the part of it no output
	// may hold.
	testWebhookPath   = "/services/T0/B0/" + webhookPathMarker + "-TAIL8HKS"
	webhookPathMarker = "WEBHOOKVALUE"

	lockFileName = ".yt2column.lock"

	// runBound bounds a wait for a run started in a goroutine to return.
	runBound = 30 * time.Second
)

// validSubtitles parses as a usable json3 subtitle file for any video ID.
const validSubtitles = `{"events":[{"tStartMs":0,"segs":[{"utf8":"hello from the transcript"}]}]}`

var (
	errInjectedPublish = errors.New("injected publish failure")
	errStubLLM         = errors.New("stub LLM failure")
)

// infoFor returns an info.json whose id matches videoID.
func infoFor(videoID string) string {
	return `{"id":"` + videoID + `","title":"A title","channel":"A channel","description":"A description"}`
}

// seedCache places a valid cache for videoID under dir, creating dir.
func seedCache(t *testing.T, dir, videoID, subtitles, info string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	if err := transcript.SeedCacheForTest(dir, videoID, []byte(subtitles), []byte(info)); err != nil {
		t.Fatalf("SeedCacheForTest: %v", err)
	}
}

// secretStrings returns every string that must never be written: each secret
// and its last eight characters, and webhookPathMarker, so a fragment of the
// Webhook URL path left around a redacted tail is caught too.
func secretStrings() []string {
	return []string{testAPIKey, testAPIKey[len(testAPIKey)-8:], testWebhook, testWebhook[len(testWebhook)-8:], webhookPathMarker}
}

// runEnv is the environment of one run invocation. Its fields are set to a
// successful run by newRunEnv and adjusted by a test before run is called.
type runEnv struct {
	base           string
	ctx            context.Context
	env            map[string]string
	args           []string // nil means --out outPath runVideoURL
	cacheDir       string
	outPath        string
	tripwireMarker string
	client         llm.LLMClient // the fake LLM client, counted by counter
	counter        *generateCounter
	webhook        *webhookServer // the loopback Webhook, when useWebhook set one up
	d              deps
	stdout         bytes.Buffer
	stderr         bytes.Buffer
}

// newRunEnv returns an environment where a run succeeds without yt-dlp: the
// video's cache is seeded, YT2COLUMN_YTDLP_PATH is a tripwire, the LLM client
// returns a valid response, and the real ArticleWriter and FilePublisher run.
// A --slack run fails to build its publisher unless the test calls useWebhook
// or substitutes newSlackPublisher, so it never posts to SLACK_WEBHOOK_URL.
func newRunEnv(t *testing.T) *runEnv {
	t.Helper()
	base := t.TempDir()
	e := &runEnv{
		base:     base,
		ctx:      context.Background(),
		cacheDir: filepath.Join(base, "cache"),
		outPath:  filepath.Join(base, "out", "article.md"),
		counter:  &generateCounter{},
	}
	seedCache(t, e.cacheDir, runVideoID, validSubtitles, infoFor(runVideoID))
	if err := os.Mkdir(filepath.Dir(e.outPath), 0o700); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(base, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tripwire, marker := transcripttestutil.NewTripwire(t, binDir)
	e.tripwireMarker = marker
	e.env = map[string]string{
		"YT2COLUMN_MODEL":      "fake-model",
		"DEEPSEEK_API_KEY":     testAPIKey,
		"SLACK_WEBHOOK_URL":    testWebhook,
		"YT2COLUMN_CACHE_DIR":  e.cacheDir,
		"YT2COLUMN_YTDLP_PATH": tripwire,
	}
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return validResponse(), nil
	})
	e.d = productionDeps()
	e.d.newLLMClient = func(config.Config) (llm.LLMClient, error) {
		return e.counter.wrap(e.client), nil
	}
	e.d.newSlackPublisher = refusingSlackPublisher
	return e
}

// webhookServer is a loopback Webhook that records the text of every message
// it receives. respond answers message n (from 1); nil answers 200 "ok".
type webhookServer struct {
	mu      sync.Mutex
	texts   []string
	respond func(n int, w http.ResponseWriter)
}

// ServeHTTP records the message and answers it.
func (s *webhookServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Text string `json:"text"`
	}
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &payload)
	s.mu.Lock()
	s.texts = append(s.texts, payload.Text)
	n := len(s.texts)
	respond := s.respond
	s.mu.Unlock()
	if respond == nil {
		_, _ = io.WriteString(w, "ok")
		return
	}
	respond(n, w)
}

// messages returns the text of every message received so far.
func (s *webhookServer) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.texts)
}

// useWebhook starts a loopback Webhook answering with respond, records it in
// e.webhook, and makes the run post to it with --slack.
func (e *runEnv) useWebhook(t *testing.T, respond func(n int, w http.ResponseWriter)) {
	t.Helper()
	e.webhook = &webhookServer{respond: respond}
	server := httptest.NewServer(e.webhook)
	t.Cleanup(server.Close)
	e.d.newSlackPublisher = loopbackSlackPublisher(t, server.URL)
	e.args = []string{"--slack", runVideoURL}
}

// failMessage returns a respond function that answers message n with status
// and body, and every other message with 200 "ok".
func failMessage(n, status int, body string) func(int, http.ResponseWriter) {
	return func(got int, w http.ResponseWriter) {
		if got != n {
			_, _ = io.WriteString(w, "ok")
			return
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// respondWith sets the LLM client to return text as the article.
func (e *runEnv) respondWith(text string) {
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		resp := validResponse()
		resp.Text = text
		return resp, nil
	})
}

// threeMessageArticle is generated text whose article Publish splits into
// three messages: each paragraph fills most of one message.
func threeMessageArticle() string {
	paragraph := strings.Repeat("Plain words fill this line of the article.\n", 340)
	return "# A long title\n\n" + paragraph + "\n" + paragraph + "\n" + paragraph
}

// cliArgs returns the arguments run receives.
func (e *runEnv) cliArgs() []string {
	if e.args != nil {
		return e.args
	}
	return []string{"--out", e.outPath, runVideoURL}
}

// run calls run once and returns its exit code.
func (e *runEnv) run() int {
	return run(e.ctx, e.cliArgs(), lookupFrom(e.env), &e.stdout, &e.stderr, e.d)
}

// videoEntries returns the cache entries of runVideoID; none when the cache
// directory cannot exist.
func videoEntries(t *testing.T, cacheDir string) []string {
	t.Helper()
	var entries []string
	for name := range snapshotTree(t, cacheDir) {
		if !strings.Contains(name, string(os.PathSeparator)) && strings.HasPrefix(name, runVideoID+".") {
			entries = append(entries, name)
		}
	}
	slices.Sort(entries)
	return entries
}

// removeVideoCache removes every cache entry of runVideoID.
func removeVideoCache(t *testing.T, cacheDir string) {
	t.Helper()
	for _, entry := range videoEntries(t, cacheDir) {
		if err := os.RemoveAll(filepath.Join(cacheDir, entry)); err != nil {
			t.Fatal(err)
		}
	}
}

// requireSafeOutput checks what every path must satisfy: no secret or its
// tail in standard output, standard error, or the --out file, and standard
// error made only of the CLI's own lines with no raw escape character. Rows
// that inject no control character only exercise the CLI's own lines here;
// TestRunEscapesUntrustedText injects them into each untrusted string.
func requireSafeOutput(t *testing.T, stdout, stderr, outPath string) {
	t.Helper()
	outContent := ""
	if data, err := os.ReadFile(outPath); err == nil { //nolint:gosec // a path inside the test's temporary directory
		outContent = string(data)
	}
	for _, s := range secretStrings() {
		for where, text := range map[string]string{"stdout": stdout, "stderr": stderr, "--out file": outContent} {
			if strings.Contains(text, s) {
				t.Errorf("%s holds a secret or its tail", where)
			}
		}
	}
	if strings.Contains(stderr, "\x1b") {
		t.Errorf("stderr holds a raw escape character:\n%q", stderr)
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(stderr, "\n"), "\n") {
		if stderr != "" && !strings.HasPrefix(line, programName+": ") {
			t.Errorf("stderr line %q does not start with %q", line, programName+": ")
		}
	}
}

// expiringContext is a context whose Err becomes DeadlineExceeded when the
// test calls expire, so a deadline can be made to pass at a chosen moment.
type expiringContext struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func newExpiringContext() *expiringContext {
	return &expiringContext{Context: context.Background(), done: make(chan struct{})}
}

func (c *expiringContext) Done() <-chan struct{} { return c.done }

func (c *expiringContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (c *expiringContext) expire() { c.once.Do(func() { close(c.done) }) }

// publisherFunc is a publisher.Publisher that runs the function.
type publisherFunc func(ctx context.Context, a writer.Article) error

func (f publisherFunc) Publish(ctx context.Context, a writer.Article) error { return f(ctx, a) }

// pathRow is one row of the execution-path table.
type pathRow struct {
	name     string
	setup    func(t *testing.T, e *runEnv)
	wantCode int
	// help means standard output holds the usage; otherwise it is empty.
	help bool
	// untouched means the cache directory (including the lock file) is
	// unchanged, yt-dlp is not started, the LLM is not called, and nothing
	// is created at --out. It is implied for help and for exitUsage.
	untouched bool
	// keepsTempFile allows the run to leave its kept temporary file next to
	// --out; otherwise a failed run leaves the --out directory as it was.
	keepsTempFile bool
	wantStderr    []string
	// notStderr lists hints standard error must not hold.
	notStderr []string
	check     func(t *testing.T, e *runEnv)
}

// envRow returns a row for a rejected environment: exitUsage, the variable
// names in standard error, and none of the variables' values.
func envRow(name string, change func(env map[string]string), wantNames ...string) pathRow {
	return pathRow{
		name:       name,
		setup:      func(_ *testing.T, e *runEnv) { change(e.env) },
		wantCode:   exitUsage,
		wantStderr: append(wantNames, "yt2column -h"),
		check: func(t *testing.T, e *runEnv) {
			for name, value := range e.env {
				if value != "" && strings.Contains(e.stderr.String(), value) {
					t.Errorf("stderr holds the value of %s", name)
				}
			}
		},
	}
}

// usageRow returns a row for an argument error.
func usageRow(name string, args func(e *runEnv) []string, want string) pathRow {
	return pathRow{
		name:       name,
		setup:      func(_ *testing.T, e *runEnv) { e.args = args(e) },
		wantCode:   exitUsage,
		wantStderr: []string{want, "yt2column -h"},
	}
}

// slackUsageRow returns a row for a rejected --out and --slack combination.
// A loopback Webhook is set up first, so a run that wrongly posted is seen.
func slackUsageRow(name string, args func(e *runEnv) []string, want string) pathRow {
	return pathRow{
		name: name,
		setup: func(t *testing.T, e *runEnv) {
			e.useWebhook(t, nil)
			e.args = args(e)
		},
		wantCode:   exitUsage,
		wantStderr: []string{want, "yt2column -h"},
	}
}

// insideCacheRow returns a row whose --out is inside the cache directory.
func insideCacheRow(name string, setup func(t *testing.T, e *runEnv) string) pathRow {
	return pathRow{
		name: name,
		setup: func(t *testing.T, e *runEnv) {
			e.args = []string{"--out", setup(t, e), runVideoURL}
		},
		wantCode:   exitUsage,
		wantStderr: []string{"inside the cache directory", "yt2column -h"},
	}
}

// startHolder starts a run in a goroutine that holds the cache directory lock
// with a stopping yt-dlp, waits until yt-dlp is ready, and registers a cleanup
// that cancels the run and waits for it.
func startHolder(t *testing.T, e *runEnv) {
	t.Helper()
	stopping := transcripttestutil.NewStopping(t, t.TempDir())
	env := maps.Clone(e.env)
	env["YT2COLUMN_YTDLP_PATH"] = stopping.Script
	holderOut := filepath.Join(filepath.Dir(e.outPath), "holder.md")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		var stdout, stderr bytes.Buffer
		run(ctx, []string{"--refresh", "--out", holderOut, runVideoURL}, lookupFrom(env), &stdout, &stderr, testDeps(validResponseLLM()))
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(runBound):
			t.Error("the holding run did not return after cancellation")
		}
	})
	waitForFile(t, stopping.Ready)
}

// executionPathRows lists every path of the execution-path list except the
// signals, which signal_test.go covers in a child process.
func executionPathRows() []pathRow {
	return []pathRow{
		{
			name:       "success",
			wantCode:   exitOK,
			wantStderr: []string{"wrote the article to", "fake-model", "v1"},
			check: func(t *testing.T, e *runEnv) {
				if got := e.counter.count(); got != 1 {
					t.Errorf("Generate calls = %d, want 1", got)
				}
				if entries := videoEntries(t, e.cacheDir); len(entries) != 0 {
					t.Errorf("the video's cache %v remains after a successful run", entries)
				}
				if !strings.Contains(e.stderr.String(), e.outPath) {
					t.Errorf("stderr does not name the --out path:\n%s", e.stderr.String())
				}
			},
		},
		{
			name:       "success with --keep-cache",
			setup:      func(_ *testing.T, e *runEnv) { e.args = []string{"--keep-cache", "--out", e.outPath, runVideoURL} },
			wantCode:   exitOK,
			wantStderr: []string{"wrote the article to"},
			check: func(t *testing.T, e *runEnv) {
				if len(videoEntries(t, e.cacheDir)) == 0 {
					t.Error("the video's cache was removed despite --keep-cache")
				}
			},
		},
		{name: "-h", setup: func(_ *testing.T, e *runEnv) { e.args = []string{"-h"} }, wantCode: exitOK, help: true},
		{name: "--help", setup: func(_ *testing.T, e *runEnv) { e.args = []string{"--help"} }, wantCode: exitOK, help: true},

		usageRow("no video URL", func(e *runEnv) []string { return []string{"--out", e.outPath} }, "expected exactly one video URL"),
		usageRow("two video URLs", func(e *runEnv) []string { return []string{"--out", e.outPath, runVideoURL, runVideoURL} }, "expected exactly one video URL"),
		usageRow("a flag after the video URL", func(e *runEnv) []string { return []string{"--out", e.outPath, runVideoURL, "--refresh"} }, "expected exactly one video URL"),
		usageRow("an undefined flag", func(e *runEnv) []string { return []string{"--bogus", "--out", e.outPath, runVideoURL} }, "flag provided but not defined"),
		slackUsageRow("neither --out nor --slack", func(*runEnv) []string { return []string{runVideoURL} }, "one of --out or --slack is required"),
		usageRow("a video URL on another host", func(e *runEnv) []string {
			return []string{"--out", e.outPath, "https://example.com/watch?v=dQw4w9WgXcQ"}
		}, "invalid video URL"),
		usageRow("a short video ID", func(e *runEnv) []string {
			return []string{"--out", e.outPath, "https://www.youtube.com/watch?v=short"}
		}, "invalid video URL"),
		usageRow("-rf after --", func(e *runEnv) []string { return []string{"--out", e.outPath, "--", "-rf"} }, "invalid video URL"),

		envRow("YT2COLUMN_MODEL unset", func(env map[string]string) { delete(env, "YT2COLUMN_MODEL") }, "YT2COLUMN_MODEL"),
		envRow("DEEPSEEK_API_KEY empty", func(env map[string]string) { env["DEEPSEEK_API_KEY"] = "" }, "DEEPSEEK_API_KEY"),
		envRow("YT2COLUMN_LLM_PROVIDER invalid", func(env map[string]string) { env["YT2COLUMN_LLM_PROVIDER"] = "MARKER-PROVIDER" }, "YT2COLUMN_LLM_PROVIDER"),
		envRow("SLACK_WEBHOOK_URL invalid", func(env map[string]string) {
			env["SLACK_WEBHOOK_URL"] = "http://hooks.slack.com/services/MARKER-WEBHOOK"
		}, "SLACK_WEBHOOK_URL"),
		envRow("YT2COLUMN_CACHE_DIR relative", func(env map[string]string) { env["YT2COLUMN_CACHE_DIR"] = "MARKER-CACHE" }, "YT2COLUMN_CACHE_DIR"),
		envRow("YT2COLUMN_YTDLP_PATH empty", func(env map[string]string) { env["YT2COLUMN_YTDLP_PATH"] = "" }, "YT2COLUMN_YTDLP_PATH"),
		envRow("several invalid variables", func(env map[string]string) {
			delete(env, "YT2COLUMN_MODEL")
			env["YT2COLUMN_LLM_PROVIDER"] = "MARKER-PROVIDER"
		}, "YT2COLUMN_MODEL", "YT2COLUMN_LLM_PROVIDER"),

		{
			name: "YT2COLUMN_MODEL with surrounding whitespace",
			setup: func(_ *testing.T, e *runEnv) {
				e.env["YT2COLUMN_MODEL"] = " fake-model "
				e.d.newLLMClient = productionDeps().newLLMClient
			},
			wantCode:   exitUsage,
			wantStderr: []string{"build the LLM client", "yt2column -h"},
		},
		{
			name: "missing --system-prompt file",
			setup: func(_ *testing.T, e *runEnv) {
				e.args = []string{"--system-prompt", filepath.Join(e.base, "missing.tmpl"), "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"build the article writer", "yt2column -h"},
		},
		{
			name: "missing --user-prompt file",
			setup: func(_ *testing.T, e *runEnv) {
				e.args = []string{"--user-prompt", filepath.Join(e.base, "missing.tmpl"), "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"build the article writer", "yt2column -h"},
		},
		{
			name: "a rejected template",
			setup: func(t *testing.T, e *runEnv) {
				path := filepath.Join(e.base, "define.tmpl")
				if err := os.WriteFile(path, []byte(`{{define "x"}}{{end}}{{.Title}}`), 0o600); err != nil {
					t.Fatal(err)
				}
				e.args = []string{"--user-prompt", path, "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"build the article writer", "yt2column -h"},
		},

		insideCacheRow("--out directly in the cache directory", func(_ *testing.T, e *runEnv) string {
			return filepath.Join(e.cacheDir, "article.md")
		}),
		insideCacheRow("--out on a cache entry name", func(t *testing.T, e *runEnv) string {
			seedCache(t, e.cacheDir, otherVideoID, validSubtitles, infoFor(otherVideoID))
			return filepath.Join(e.cacheDir, otherVideoID+".current")
		}),
		insideCacheRow("relative --out with the cache directory as working directory", func(t *testing.T, e *runEnv) string {
			t.Chdir(e.cacheDir)
			return "article.md"
		}),
		insideCacheRow("--out back into the cache directory through ..", func(_ *testing.T, e *runEnv) string {
			return joinRaw(e.cacheDir, "..", filepath.Base(e.cacheDir), "article.md")
		}),
		insideCacheRow("--out through a symbolic link to the cache directory", func(t *testing.T, e *runEnv) string {
			link := filepath.Join(e.base, "cache-link")
			if err := os.Symlink(e.cacheDir, link); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(link, "article.md")
		}),
		insideCacheRow("--out is the cache directory", func(_ *testing.T, e *runEnv) string {
			return e.cacheDir
		}),

		{
			name: "transcript failure",
			setup: func(t *testing.T, e *runEnv) {
				e.env["YT2COLUMN_YTDLP_PATH"] = transcripttestutil.NewStderrFailure(t, t.TempDir(), "ERROR: the fake yt-dlp failed")
				e.args = []string{"--refresh", "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the transcript stage failed", "ERROR: the fake yt-dlp"},
		},
		{
			name: "write failure whose message holds the API key",
			setup: func(_ *testing.T, e *runEnv) {
				e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
					return llm.GenerateResponse{}, fmt.Errorf("%w: the server rejected %s", errStubLLM, testAPIKey)
				})
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the write stage failed", "the server rejected " + redactedMarker},
		},
		{
			name: "LLM timeout",
			setup: func(_ *testing.T, e *runEnv) {
				e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
					return llm.GenerateResponse{}, fmt.Errorf("%w: %w", errStubLLM, context.DeadlineExceeded)
				})
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the write stage failed", "the LLM call timed out after the 15-minute limit"},
			notStderr:  []string{"the webhook post timed out"},
		},
		{
			name: "publish failure",
			setup: func(_ *testing.T, e *runEnv) {
				e.d.newFilePublisher = func(string) (publisher.Publisher, error) {
					return &publishertestutil.FakePublisher{Err: errInjectedPublish}, nil
				}
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the publish stage failed", "injected publish failure"},
		},
		{
			name: "--out is an existing file",
			setup: func(t *testing.T, e *runEnv) {
				if err := os.WriteFile(e.outPath, []byte("existing"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantCode:   exitFailure,
			untouched:  true,
			wantStderr: []string{"the publish stage failed", "output path already exists"},
		},
		{
			name: "--out is a dangling symbolic link",
			setup: func(t *testing.T, e *runEnv) {
				if err := os.Symlink(filepath.Join(e.base, "nowhere"), e.outPath); err != nil {
					t.Fatal(err)
				}
			},
			wantCode:   exitFailure,
			untouched:  true,
			wantStderr: []string{"the publish stage failed", "output path already exists"},
			check: func(t *testing.T, e *runEnv) {
				requireNoFile(t, filepath.Join(e.base, "nowhere"))
			},
		},
		{
			name:       "--out parent is missing",
			setup:      func(_ *testing.T, e *runEnv) { e.outPath = filepath.Join(e.base, "missing", "article.md") },
			wantCode:   exitFailure,
			untouched:  true,
			wantStderr: []string{"the publish stage failed"},
		},
		{
			name: "--out parent is a regular file",
			setup: func(t *testing.T, e *runEnv) {
				parent := filepath.Join(e.base, "afile")
				if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				e.outPath = filepath.Join(parent, "article.md")
			},
			wantCode:   exitFailure,
			untouched:  true,
			wantStderr: []string{"the publish stage failed"},
		},
		{
			name: "prune failure warns",
			setup: func(t *testing.T, e *runEnv) {
				requireNonRoot(t)
				dangling := filepath.Join(e.cacheDir, otherVideoID+".b")
				if err := os.Mkdir(dangling, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dangling, "file"), []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				chmodForTest(t, dangling, 0o500)
			},
			wantCode:   exitOK,
			wantStderr: []string{"warning: prune the cache", "wrote the article to"},
		},
		{
			name: "cache removal failure warns",
			setup: func(_ *testing.T, e *runEnv) {
				ctx, cancel := context.WithCancel(context.Background())
				e.ctx = ctx
				e.d.newFilePublisher = func(path string) (publisher.Publisher, error) {
					inner, err := newFilePublisher(path)
					if err != nil {
						return nil, err
					}
					return publisherFunc(func(ctx context.Context, a writer.Article) error {
						if err := inner.Publish(ctx, a); err != nil {
							return err
						}
						cancel()
						return nil
					}), nil
				}
			},
			wantCode:   exitOK,
			wantStderr: []string{"warning: remove the cache", "wrote the article to"},
			check: func(t *testing.T, e *runEnv) {
				if len(videoEntries(t, e.cacheDir)) == 0 {
					t.Error("the video's cache was removed although the removal was canceled")
				}
			},
		},
		{
			name: "cache directory cannot be created",
			setup: func(t *testing.T, e *runEnv) {
				parent := filepath.Join(e.base, "afile")
				if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				e.cacheDir = filepath.Join(parent, "cache")
				e.env["YT2COLUMN_CACHE_DIR"] = e.cacheDir
			},
			wantCode:   exitFailure,
			untouched:  true,
			wantStderr: []string{"the run failed: create the cache directory"},
			check:      requireNotLocked,
		},
		{
			name: "lock file cannot be created",
			setup: func(t *testing.T, e *runEnv) {
				requireNonRoot(t)
				chmodForTest(t, e.cacheDir, 0o500)
			},
			wantCode:   exitFailure,
			untouched:  true,
			wantStderr: []string{"the run failed: open the lock file"},
			check:      requireNotLocked,
		},
		{
			name:       "another run holds the lock",
			setup:      startHolder,
			wantCode:   exitFailure,
			untouched:  true,
			wantStderr: []string{"another run holds the cache directory lock", "yt-dlp"},
			check: func(t *testing.T, e *runEnv) {
				if lockPath := filepath.Join(e.cacheDir, lockFileName); !strings.Contains(e.stderr.String(), lockPath) {
					t.Errorf("stderr does not name the lock file %s:\n%s", lockPath, e.stderr.String())
				}
			},
		},
		{
			name: "invalid cached subtitles",
			setup: func(t *testing.T, e *runEnv) {
				if err := os.RemoveAll(e.cacheDir); err != nil {
					t.Fatal(err)
				}
				seedCache(t, e.cacheDir, runVideoID, `{"events":[`, infoFor(runVideoID))
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the transcript stage failed", "run again with --refresh"},
		},
		{
			name: "invalid cached info.json",
			setup: func(t *testing.T, e *runEnv) {
				if err := os.RemoveAll(e.cacheDir); err != nil {
					t.Fatal(err)
				}
				seedCache(t, e.cacheDir, runVideoID, validSubtitles, `{"id":`)
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the transcript stage failed", "run again with --refresh"},
		},
		{
			name: "interrupted while writing",
			setup: func(_ *testing.T, e *runEnv) {
				ctx, cancel := context.WithCancel(context.Background())
				e.ctx = ctx
				e.client = funcLLM(func(ctx context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
					cancel()
					return llm.GenerateResponse{}, ctx.Err()
				})
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the write stage failed", "interrupted"},
		},
		{
			name: "interrupted while publishing",
			setup: func(_ *testing.T, e *runEnv) {
				ctx, cancel := context.WithCancel(context.Background())
				e.ctx = ctx
				e.d.newFilePublisher = func(string) (publisher.Publisher, error) {
					return publisherFunc(func(ctx context.Context, _ writer.Article) error {
						cancel()
						return ctx.Err()
					}), nil
				}
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the publish stage failed", "interrupted"},
		},
		{
			name: "the article is kept in a temporary file",
			setup: func(_ *testing.T, e *runEnv) {
				e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
					// Something takes the --out name after the pre-check, so
					// link(2) fails with EEXIST.
					if err := os.WriteFile(e.outPath, []byte("raced"), 0o600); err != nil {
						return llm.GenerateResponse{}, err
					}
					return validResponse(), nil
				})
			},
			wantCode:      exitFailure,
			keepsTempFile: true,
			wantStderr:    []string{"the publish stage failed", "the finished article was kept in"},
			check: func(t *testing.T, e *runEnv) {
				matches, err := filepath.Glob(filepath.Join(filepath.Dir(e.outPath), tempPrefixForTest+"*"))
				if err != nil || len(matches) != 1 {
					t.Fatalf("kept temporary files = %v (error = %v), want one", matches, err)
				}
				if !strings.Contains(e.stderr.String(), matches[0]) {
					t.Errorf("stderr does not name the kept file %s:\n%s", matches[0], e.stderr.String())
				}
				if data, err := os.ReadFile(e.outPath); err != nil || string(data) != "raced" {
					t.Errorf("--out content = %q (error = %v), want the racing file untouched", data, err)
				}
			},
		},
		{
			name: "yt-dlp timeout",
			setup: func(t *testing.T, e *runEnv) {
				stopping := transcripttestutil.NewStopping(t, t.TempDir())
				e.env["YT2COLUMN_YTDLP_PATH"] = stopping.Script
				e.args = []string{"--refresh", "--out", e.outPath, runVideoURL}
				ctx := newExpiringContext()
				e.ctx = ctx
				stop := make(chan struct{})
				t.Cleanup(func() { close(stop) })
				go func() {
					// Expire once yt-dlp runs; expire anyway at the bound so
					// the run cannot hang, and the row then fails on its
					// expectations.
					deadline := time.Now().Add(readyBound)
				poll:
					for time.Now().Before(deadline) {
						if _, err := os.Stat(stopping.Ready); err == nil {
							break
						}
						select {
						case <-stop:
							break poll
						case <-time.After(pollInterval):
						}
					}
					ctx.expire()
				}()
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the transcript stage failed", "yt-dlp timed out after the 5-minute limit"},
			notStderr:  []string{"the webhook post timed out"},
		},
		{
			name:       "--slack success",
			setup:      func(t *testing.T, e *runEnv) { e.useWebhook(t, nil) },
			wantCode:   exitOK,
			wantStderr: []string{"posted the article to the webhook in 1 message", "fake-model", "v1"},
			check: func(t *testing.T, e *runEnv) {
				if got := e.counter.count(); got != 1 {
					t.Errorf("Generate calls = %d, want 1", got)
				}
				if entries := videoEntries(t, e.cacheDir); len(entries) != 0 {
					t.Errorf("the video's cache %v remains after a successful run", entries)
				}
				if got := e.webhook.messages(); len(got) != 1 || !strings.Contains(got[0], "The body.") {
					t.Errorf("the webhook received %q, want one message holding the article", got)
				}
			},
		},
		{
			name: "--slack success with --keep-cache",
			setup: func(t *testing.T, e *runEnv) {
				e.useWebhook(t, nil)
				e.args = []string{"--keep-cache", "--slack", runVideoURL}
			},
			wantCode:   exitOK,
			wantStderr: []string{"posted the article to the webhook in 1 message"},
			check: func(t *testing.T, e *runEnv) {
				if len(videoEntries(t, e.cacheDir)) == 0 {
					t.Error("the video's cache was removed despite --keep-cache")
				}
			},
		},
		{
			name: "--slack success in three messages",
			setup: func(t *testing.T, e *runEnv) {
				e.useWebhook(t, nil)
				e.respondWith(threeMessageArticle())
			},
			wantCode:   exitOK,
			wantStderr: []string{"posted the article to the webhook in 3 messages"},
			check: func(t *testing.T, e *runEnv) {
				if got := len(e.webhook.messages()); got != 3 {
					t.Errorf("the webhook received %d messages, want 3", got)
				}
			},
		},
		{
			// A --slack run has no --out, so the cache-directory check for it
			// must not run: with the working directory inside the cache
			// directory, an empty --out would resolve inside it.
			name: "--slack success with the cache directory as working directory",
			setup: func(t *testing.T, e *runEnv) {
				e.useWebhook(t, nil)
				t.Chdir(e.cacheDir)
			},
			wantCode:   exitOK,
			wantStderr: []string{"posted the article to the webhook in 1 message"},
		},
		slackUsageRow("--out and --slack", func(e *runEnv) []string {
			return []string{"--out", e.outPath, "--slack", runVideoURL}
		}, "--out and --slack cannot be used together"),
		slackUsageRow("--out with an empty path", func(*runEnv) []string {
			return []string{"--out", "", runVideoURL}
		}, "--out needs a path"),
		slackUsageRow("--slack=false", func(*runEnv) []string {
			return []string{"--slack=false", runVideoURL}
		}, "--slack=false is not accepted; omit --slack instead"),
		slackUsageRow("--out with an empty path and --slack", func(*runEnv) []string {
			return []string{"--out", "", "--slack", runVideoURL}
		}, "--out needs a path"),
		slackUsageRow("--out and --slack=false", func(e *runEnv) []string {
			return []string{"--out", e.outPath, "--slack=false", runVideoURL}
		}, "--slack=false is not accepted; omit --slack instead"),
		slackUsageRow("--out with an empty path and --slack=false", func(*runEnv) []string {
			return []string{"--out", "", "--slack=false", runVideoURL}
		}, "--slack=false is not accepted; omit --slack instead"),
		{
			name: "--slack with SLACK_WEBHOOK_URL unset",
			setup: func(t *testing.T, e *runEnv) {
				e.useWebhook(t, nil)
				delete(e.env, "SLACK_WEBHOOK_URL")
			},
			wantCode:   exitUsage,
			wantStderr: []string{"configuration: SLACK_WEBHOOK_URL", "yt2column -h"},
		},
		{
			name:       "--slack first message fails",
			setup:      func(t *testing.T, e *runEnv) { e.useWebhook(t, failMessage(1, http.StatusInternalServerError, "")) },
			wantCode:   exitFailure,
			wantStderr: []string{"the publish stage failed", "posted 0 of 1 messages", "unexpected HTTP status 500"},
			check: func(t *testing.T, e *runEnv) {
				if strings.Contains(e.stderr.String(), "stay in the channel") {
					t.Errorf("stderr reports messages left in the channel although none was posted:\n%s", e.stderr.String())
				}
			},
		},
		{
			name: "--slack failure whose message holds the webhook path",
			setup: func(_ *testing.T, e *runEnv) {
				e.args = []string{"--slack", runVideoURL}
				e.d.newSlackPublisher = func(secret.Secret) (publisher.Publisher, error) {
					return &publishertestutil.FakePublisher{Err: fmt.Errorf("%w: post to %s", errInjectedPublish, testWebhookPath)}, nil
				}
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the publish stage failed", "post to " + redactedMarker},
		},
		{
			name: "--slack fails partway through a split post",
			setup: func(t *testing.T, e *runEnv) {
				e.useWebhook(t, failMessage(2, http.StatusInternalServerError, ""))
				e.respondWith(threeMessageArticle())
			},
			wantCode: exitFailure,
			wantStderr: []string{
				"the publish stage failed", "posted 1 of 3 messages",
				"at least 1 of 3 messages stay in the channel (message 2 may also have been posted)",
				"running again generates a new article and posts all of it from the first message",
			},
			check: func(t *testing.T, e *runEnv) {
				if got := len(e.webhook.messages()); got != 2 {
					t.Errorf("the webhook received %d messages, want 2", got)
				}
			},
		},
		{
			name: "--slack preparation rejects a mention",
			setup: func(t *testing.T, e *runEnv) {
				e.useWebhook(t, nil)
				e.respondWith("# A title\n\nPing @channel now.\n")
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the publish stage failed", "the article was not posted and was discarded"},
			check: func(t *testing.T, e *runEnv) {
				if got := e.webhook.messages(); len(got) != 0 {
					t.Errorf("the webhook received %d messages, want none", len(got))
				}
			},
		},
	}
}

// tempPrefixForTest is the start of a FilePublisher temporary file's name.
const tempPrefixForTest = ".yt2column-"

// requireNotLocked asserts the failure was not reported as a concurrent run.
func requireNotLocked(t *testing.T, e *runEnv) {
	t.Helper()
	if strings.Contains(e.stderr.String(), "another run holds") {
		t.Errorf("a lock failure was reported as a concurrent run:\n%s", e.stderr.String())
	}
}

// TestRunExecutionPaths runs every path of the execution-path list except the
// signals and checks, per row, the exit code, standard output, the messages
// on standard error, and the side effects: no secret and no raw control
// character anywhere, nothing touched on a usage path, and neither --out
// created nor the video's cache removed on a failure.
func TestRunExecutionPaths(t *testing.T) {
	for _, row := range executionPathRows() {
		t.Run(row.name, func(t *testing.T) {
			e := newRunEnv(t)
			untouched := row.untouched || row.help || row.wantCode == exitUsage
			if untouched {
				// Without the video's cache, a run that wrongly reached the
				// transcript stage would start the tripwire, so the "yt-dlp
				// not started" check below can fail.
				removeVideoCache(t, e.cacheDir)
			}
			if row.setup != nil {
				row.setup(t, e)
			}
			cacheBefore := snapshotTree(t, e.cacheDir)
			outDirBefore := snapshotTree(t, filepath.Dir(e.outPath))
			videoBefore := videoEntries(t, e.cacheDir)

			code := e.run()

			stdout, stderr := e.stdout.String(), e.stderr.String()
			if code != row.wantCode {
				// Not fatal: the side-effect checks below show what a wrong
				// path did.
				t.Errorf("exit code = %d, want %d\nstderr:\n%s", code, row.wantCode, stderr)
			}
			if row.help {
				if !strings.Contains(stdout, "Usage: yt2column") {
					t.Errorf("stdout does not hold the usage:\n%s", stdout)
				}
			} else if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			for _, want := range row.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr does not contain %q:\n%s", want, stderr)
				}
			}
			for _, unwanted := range row.notStderr {
				if strings.Contains(stderr, unwanted) {
					t.Errorf("stderr contains %q:\n%s", unwanted, stderr)
				}
			}
			requireSafeOutput(t, stdout, stderr, e.outPath)

			if untouched {
				if diff := snapshotTree(t, e.cacheDir); !maps.Equal(cacheBefore, diff) {
					t.Errorf("the cache directory changed:\nbefore %v\nafter  %v", cacheBefore, diff)
				}
				if _, err := os.Lstat(e.tripwireMarker); err == nil {
					t.Error("yt-dlp was started")
				}
				if got := e.counter.count(); got != 0 {
					t.Errorf("Generate calls = %d, want 0", got)
				}
				if e.webhook != nil && len(e.webhook.messages()) != 0 {
					t.Errorf("the webhook received %d messages, want none", len(e.webhook.messages()))
				}
			}
			switch {
			case row.wantCode == exitFailure || untouched:
				if !row.keepsTempFile {
					if after := snapshotTree(t, filepath.Dir(e.outPath)); !maps.Equal(outDirBefore, after) {
						t.Errorf("the --out directory changed:\nbefore %v\nafter  %v", outDirBefore, after)
					}
				}
				after := videoEntries(t, e.cacheDir)
				for _, entry := range videoBefore {
					if !slices.Contains(after, entry) {
						t.Errorf("the video's cache entry %s was removed", entry)
					}
				}
			case e.webhook != nil:
				if len(e.webhook.messages()) == 0 {
					t.Error("the webhook received no message")
				}
				requireNoFile(t, e.outPath)
			default:
				if _, err := os.Stat(e.outPath); err != nil {
					t.Errorf("--out was not created: %v", err)
				}
			}
			if row.check != nil {
				row.check(t, e)
			}
		})
	}
}

// TestRunHelp checks the usage in detail: every flag of the CLI is listed,
// --slack names Mattermost as a target, the usage goes to standard output
// only, and it needs no configuration.
func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{arg}, lookupFrom(nil), &stdout, &stderr, productionDeps())
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d", code, exitOK)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
			for _, flag := range []string{"--out <path>", "--slack", "Mattermost", "--refresh", "--keep-cache", "--system-prompt <path>", "--user-prompt <path>", "-h, --help"} {
				if !strings.Contains(stdout.String(), flag) {
					t.Errorf("usage does not list %q:\n%s", flag, stdout.String())
				}
			}
		})
	}
}

// TestRunPromptOverrides checks that the --system-prompt and --user-prompt
// files are what the LLM client receives.
func TestRunPromptOverrides(t *testing.T) {
	e := newRunEnv(t)
	systemPath := filepath.Join(e.base, "system.tmpl")
	userPath := filepath.Join(e.base, "user.tmpl")
	if err := os.WriteFile(systemPath, []byte("SYSTEM-OVERRIDE for {{.Title}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, []byte("USER-OVERRIDE {{.Transcript}}"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got llm.GenerateRequest
	e.client = funcLLM(func(_ context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
		got = req
		return validResponse(), nil
	})
	e.args = []string{"--system-prompt", systemPath, "--user-prompt", userPath, "--out", e.outPath, runVideoURL}

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if got.SystemPrompt != "SYSTEM-OVERRIDE for A title" {
		t.Errorf("SystemPrompt = %q, want the override expanded", got.SystemPrompt)
	}
	if got.UserPrompt != "USER-OVERRIDE hello from the transcript" {
		t.Errorf("UserPrompt = %q, want the override expanded", got.UserPrompt)
	}
}

// TestRunEscapesUntrustedText injects an escape sequence and a newline that
// starts a fake line into each untrusted string that reaches standard error:
// the model fields on success (with --out and with --slack), yt-dlp's
// standard error, and an LLM error. A webhook's failure response body never
// reaches standard error at all: only identifier-shaped values are taken from
// it.
func TestRunEscapesUntrustedText(t *testing.T) {
	const injected = "\x1b[2J\nFAKE-LINE the run succeeded"
	modelArticle := writer.Article{Title: "A title", Body: "The body.\n", SourceURL: runVideoURL, Model: "m" + injected, ModelVersion: "v" + injected}
	cases := []struct {
		name     string
		setup    func(t *testing.T, e *runEnv)
		wantCode int
		// withheld means the injected text must not appear even escaped.
		withheld bool
	}{
		{
			name: "Model and ModelVersion on success",
			setup: func(_ *testing.T, e *runEnv) {
				e.d.newWriter = func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error) {
					return &writertestutil.FakeArticleWriter{Result: modelArticle}, nil
				}
				e.d.newFilePublisher = func(string) (publisher.Publisher, error) {
					return &publishertestutil.FakePublisher{}, nil
				}
			},
			wantCode: exitOK,
		},
		{
			name: "Model and ModelVersion on --slack success",
			setup: func(_ *testing.T, e *runEnv) {
				e.d.newWriter = func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error) {
					return &writertestutil.FakeArticleWriter{Result: modelArticle}, nil
				}
				e.d.newSlackPublisher = func(secret.Secret) (publisher.Publisher, error) {
					return &publishertestutil.FakePublisher{}, nil
				}
				e.args = []string{"--slack", runVideoURL}
			},
			wantCode: exitOK,
		},
		{
			name: "webhook failure response body",
			setup: func(t *testing.T, e *runEnv) {
				e.useWebhook(t, failMessage(1, http.StatusInternalServerError, "BODY-TEXT"+injected))
			},
			wantCode: exitFailure,
			withheld: true,
		},
		{
			name: "yt-dlp standard error",
			setup: func(t *testing.T, e *runEnv) {
				e.env["YT2COLUMN_YTDLP_PATH"] = transcripttestutil.NewStderrFailure(t, t.TempDir(), "yt-dlp says"+injected)
				e.args = []string{"--refresh", "--out", e.outPath, runVideoURL}
			},
			wantCode: exitFailure,
		},
		{
			name: "LLM error message",
			setup: func(_ *testing.T, e *runEnv) {
				e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
					return llm.GenerateResponse{}, fmt.Errorf("%w: upstream says%s", errStubLLM, injected)
				})
			},
			wantCode: exitFailure,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			tc.setup(t, e)
			if code := e.run(); code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, tc.wantCode, e.stderr.String())
			}
			stderr := e.stderr.String()
			switch {
			case tc.withheld:
				for _, s := range []string{"FAKE-LINE", "BODY-TEXT"} {
					if strings.Contains(stderr, s) {
						t.Errorf("stderr holds %q from the response body:\n%s", s, stderr)
					}
				}
			case !strings.Contains(stderr, `\x1b[2J\nFAKE-LINE`):
				t.Errorf("stderr does not hold the escaped injection:\n%s", stderr)
			}
			requireSafeOutput(t, e.stdout.String(), stderr, e.outPath)
		})
	}
}

// TestRunGODEBUGWarning checks that http2debug=1 or 2 in GODEBUG is warned
// about before the LLM call, without the API key or the Webhook URL, and
// without changing the exit code; any other GODEBUG gives no warning. The
// Webhook URL warning is added for --slack only.
func TestRunGODEBUGWarning(t *testing.T) {
	const (
		warning        = "GODEBUG enables http2debug, so the Go HTTP/2 log may write the API key"
		webhookWarning = "GODEBUG enables http2debug, so the Go HTTP/2 log may write the Webhook URL path"
	)
	cases := []struct {
		name               string
		godebug            *string
		slack              bool
		wantWarning        bool
		wantWebhookWarning bool
	}{
		{"unset", nil, false, false, false},
		{"http2debug=0", new("http2debug=0"), false, false, false},
		{"http2debug=1", new("http2debug=1"), false, true, false},
		{"http2debug=2", new("http2debug=2"), false, true, false},
		{"http2debug=1 after another setting", new("madvdontneed=1,http2debug=1"), false, true, false},
		{"--slack unset", nil, true, false, false},
		{"--slack http2debug=1", new("http2debug=1"), true, true, true},
		{"--slack http2debug=2", new("http2debug=2"), true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			if tc.godebug != nil {
				e.env["GODEBUG"] = *tc.godebug
			}
			if tc.slack {
				e.useWebhook(t, nil)
			}
			var stderrAtCall string
			e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
				stderrAtCall = e.stderr.String()
				return validResponse(), nil
			})

			if code := e.run(); code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
			}
			if got := strings.Contains(stderrAtCall, warning); got != tc.wantWarning {
				t.Errorf("warning before the LLM call = %v, want %v\nstderr at the call:\n%s", got, tc.wantWarning, stderrAtCall)
			}
			if got := strings.Contains(stderrAtCall, webhookWarning); got != tc.wantWebhookWarning {
				t.Errorf("Webhook URL warning before the LLM call = %v, want %v\nstderr at the call:\n%s", got, tc.wantWebhookWarning, stderrAtCall)
			}
			if !tc.wantWebhookWarning && strings.Contains(e.stderr.String(), webhookWarning) {
				t.Errorf("Webhook URL warning written after the LLM call:\n%s", e.stderr.String())
			}
			requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
		})
	}
}

// TestNewFilePublisherNilOnFailure checks that the production publisher
// constructor returns a nil interface, not a typed nil, when construction
// fails, so a caller comparing the result with nil is not misled.
func TestNewFilePublisherNilOnFailure(t *testing.T) {
	p, err := newFilePublisher("")
	if err == nil {
		t.Fatal("newFilePublisher(\"\") succeeded, want an error")
	}
	if p != nil {
		t.Fatalf("newFilePublisher(\"\") = %#v, want a nil interface", p)
	}
}

// TestRunSlackFailureHints checks the hints added for a failed webhook post:
// each appears exactly for the failure it describes.
func TestRunSlackFailureHints(t *testing.T) {
	const (
		stay     = "stay in the channel"
		rejected = "the server rejected the post; Mattermost does not report which of these it was"
		reqID    = "request ID"
		timedOut = "the webhook post timed out after the 30-second limit"
		mention  = "the article was not posted and was discarded"
		rerun    = "running again generates a new article and posts all of it from the first message"
	)
	requestID := strings.Repeat("r", 26)
	statusFailure := func(code int, id string) error {
		return &publisher.SlackPostError{Total: 1, Posted: 0, Attempted: true, Err: &publisher.SlackHTTPStatusError{StatusCode: code, RequestID: id}}
	}
	cases := []struct {
		name string
		err  error
		want []string
	}{
		{"nothing posted", &publisher.SlackPostError{Total: 3, Posted: 0, Attempted: true, Err: publisher.ErrSlackTransport}, nil},
		{
			"posted, next not attempted",
			&publisher.SlackPostError{Total: 3, Posted: 2, Attempted: false, Err: publisher.ErrSlackTransport},
			[]string{"yt2column: 2 of 3 messages " + stay + "; " + rerun},
		},
		{
			"posted, next attempted",
			&publisher.SlackPostError{Total: 3, Posted: 2, Attempted: true, Err: publisher.ErrSlackTransport},
			[]string{"at least 2 of 3 messages " + stay + " (message 3 may also have been posted); " + rerun},
		},
		{"400 with a request ID", statusFailure(http.StatusBadRequest, requestID), []string{rejected, "Check the webhook settings and the server log (" + reqID + ": " + requestID + ")"}},
		{"403 with a request ID", statusFailure(http.StatusForbidden, requestID), []string{rejected, reqID + ": " + requestID}},
		{"404 with a request ID", statusFailure(http.StatusNotFound, requestID), []string{rejected, reqID + ": " + requestID}},
		{"501 with a request ID", statusFailure(http.StatusNotImplemented, requestID), []string{rejected, reqID + ": " + requestID}},
		{"404 without a request ID", statusFailure(http.StatusNotFound, ""), []string{rejected}},
		{"500 with a request ID", statusFailure(http.StatusInternalServerError, requestID), nil},
		{
			"timeout",
			&publisher.SlackPostError{Total: 1, Posted: 0, Attempted: true, Err: fmt.Errorf("webhook: the post timed out after 30s: %w", context.DeadlineExceeded)},
			[]string{timedOut},
		},
		{"mention", fmt.Errorf("%w: rule M1 at line 3, column 6", publisher.ErrSlackMention), []string{mention}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			e.args = []string{"--slack", runVideoURL}
			e.d.newSlackPublisher = func(secret.Secret) (publisher.Publisher, error) {
				return &publishertestutil.FakePublisher{Err: tc.err}, nil
			}
			if code := e.run(); code != exitFailure {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, e.stderr.String())
			}
			stderr := e.stderr.String()
			for _, want := range tc.want {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr does not contain %q:\n%s", want, stderr)
				}
			}
			for _, hint := range []string{stay, rejected, reqID, timedOut, mention} {
				wanted := slices.ContainsFunc(tc.want, func(w string) bool { return strings.Contains(w, hint) })
				if got := strings.Contains(stderr, hint); got != wanted {
					t.Errorf("stderr contains %q = %v, want %v:\n%s", hint, got, wanted, stderr)
				}
			}
			requireSafeOutput(t, e.stdout.String(), stderr, e.outPath)
		})
	}
}

// TestRunSlackSummaryUnknownCount checks that a post accepted by a publisher
// whose article SlackMessageCount rejects still succeeds, with the count
// reported as unknown.
func TestRunSlackSummaryUnknownCount(t *testing.T) {
	e := newRunEnv(t)
	e.args = []string{"--slack", runVideoURL}
	e.respondWith("# A title\n\nPing @channel now.\n")
	e.d.newSlackPublisher = func(secret.Secret) (publisher.Publisher, error) {
		return &publishertestutil.FakePublisher{}, nil
	}
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if want := "posted the article to the webhook in unknown messages"; !strings.Contains(e.stderr.String(), want) {
		t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
	}
}

// TestRunSlackSignalAfterPosting cancels the run's context after every message
// was posted, as a signal arriving then does: the run still succeeds, and only
// the cache removal is reported as a warning.
func TestRunSlackSignalAfterPosting(t *testing.T) {
	e := newRunEnv(t)
	e.args = []string{"--slack", runVideoURL}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.ctx = ctx
	e.d.newSlackPublisher = func(secret.Secret) (publisher.Publisher, error) {
		return publisherFunc(func(context.Context, writer.Article) error {
			cancel()
			return nil
		}), nil
	}
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	for _, want := range []string{"warning: remove the cache", "posted the article to the webhook in 1 message"} {
		if !strings.Contains(e.stderr.String(), want) {
			t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
		}
	}
	if len(videoEntries(t, e.cacheDir)) == 0 {
		t.Error("the video's cache was removed although the removal was canceled")
	}
	requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
}

// TestConfiguredSecretsIncludesWebhookParts checks that a line holding only
// part of the Webhook URL, its path, is redacted. Which parts are produced
// (and that short ones are omitted) is slackwebhook.SensitiveParts' contract,
// tested there.
func TestConfiguredSecretsIncludesWebhookParts(t *testing.T) {
	cfg, err := config.Load(lookupFrom(newRunEnv(t).env))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	got := sanitize("POST "+testWebhookPath+" failed", configuredSecrets(cfg)...)
	if strings.Contains(got, webhookPathMarker) {
		t.Errorf("sanitize left part of the webhook path: %q", got)
	}
}

// TestConfiguredSecretsIncludesAnthropicAPIKey checks that a claude
// configuration's Anthropic API key, and a line holding only its last eight
// characters, are both redacted.
func TestConfiguredSecretsIncludesAnthropicAPIKey(t *testing.T) {
	const key = "sk-ant-APIKEYVALUE-0123456789-TAIL8ANT"
	env := map[string]string{
		"YT2COLUMN_LLM_PROVIDER":  "claude",
		"YT2COLUMN_MODEL":         "claude-opus-5-5",
		"ANTHROPIC_API_KEY":       key,
		"YT2COLUMN_CLAUDE_EFFORT": "high",
		"YT2COLUMN_CACHE_DIR":     t.TempDir(),
	}
	cfg, err := config.Load(lookupFrom(env))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if _, err := cfg.AnthropicAPIKey().Reveal(); err != nil {
		t.Fatalf("AnthropicAPIKey().Reveal() error = %v", err)
	}
	tail := key[len(key)-8:]
	if got := sanitize("POST with "+key+" failed", configuredSecrets(cfg)...); strings.Contains(got, key) {
		t.Errorf("sanitize left the Anthropic API key: %q", got)
	}
	if got := sanitize("short tail "+tail+" seen", configuredSecrets(cfg)...); strings.Contains(got, tail) {
		t.Errorf("sanitize left the Anthropic API key tail: %q", got)
	}
}

// TestTestDepsSlackPublisherDoesNotSend checks that the default
// newSlackPublisher of testDeps and of newRunEnv sends nothing: a --slack run
// stops at building the publisher with the stub's error.
func TestTestDepsSlackPublisherDoesNotSend(t *testing.T) {
	for name, depsFor := range map[string]func(e *runEnv) deps{
		"testDeps":  func(*runEnv) deps { return testDeps(validResponseLLM()) },
		"newRunEnv": func(e *runEnv) deps { return e.d },
	} {
		t.Run(name, func(t *testing.T) {
			e := newRunEnv(t)
			e.d = depsFor(e)
			e.args = []string{"--slack", runVideoURL}
			if code := e.run(); code != exitUsage {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
			}
			if want := "build the publisher: " + errSlackPublisherNotSubstituted.Error(); !strings.Contains(e.stderr.String(), want) {
				t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
			}
		})
	}
}

// TestNewSlackPublisherNilOnFailure checks that the production webhook
// publisher constructor returns a nil interface, not a typed nil, when
// construction fails.
func TestNewSlackPublisherNilOnFailure(t *testing.T) {
	p, err := productionDeps().newSlackPublisher(secret.Secret{})
	if err == nil {
		t.Fatal("newSlackPublisher(zero Secret) succeeded, want an error")
	}
	if p != nil {
		t.Fatalf("newSlackPublisher(zero Secret) = %#v, want a nil interface", p)
	}
}
