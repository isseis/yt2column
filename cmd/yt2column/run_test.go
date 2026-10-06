//go:build test

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
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
	testWebhook = "https://hooks.slack.com/services/T0/B0/WEBHOOKVALUE-TAIL8HKS"

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
// and its last eight characters.
func secretStrings() []string {
	return []string{testAPIKey, testAPIKey[len(testAPIKey)-8:], testWebhook, testWebhook[len(testWebhook)-8:]}
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
	d              deps
	stdout         bytes.Buffer
	stderr         bytes.Buffer
}

// newRunEnv returns an environment where a run succeeds without yt-dlp: the
// video's cache is seeded, YT2COLUMN_YTDLP_PATH is a tripwire, the LLM client
// returns a valid response, and the real ArticleWriter and FilePublisher run.
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
	return e
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
	check         func(t *testing.T, e *runEnv)
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

// validResponseLLM returns a client that always succeeds.
func validResponseLLM() llm.LLMClient {
	return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return validResponse(), nil
	})
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
		usageRow("no --out", func(*runEnv) []string { return []string{runVideoURL} }, "--out is required"),
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
		},
		{
			name: "publish failure",
			setup: func(_ *testing.T, e *runEnv) {
				e.d.newPublisher = func(string) (publisher.Publisher, error) {
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
				e.d.newPublisher = func(path string) (publisher.Publisher, error) {
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
				e.d.newPublisher = func(string) (publisher.Publisher, error) {
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
// it goes to standard output only, and it needs no configuration.
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
			for _, flag := range []string{"--out <path>", "--refresh", "--keep-cache", "--system-prompt <path>", "--user-prompt <path>", "-h, --help"} {
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
// the model fields on success, yt-dlp's standard error, and an LLM error.
func TestRunEscapesUntrustedText(t *testing.T) {
	const injected = "\x1b[2J\nFAKE-LINE the run succeeded"
	cases := []struct {
		name     string
		setup    func(t *testing.T, e *runEnv)
		wantCode int
	}{
		{
			name: "Model and ModelVersion on success",
			setup: func(_ *testing.T, e *runEnv) {
				article := writer.Article{Title: "A title", Body: "The body.\n", SourceURL: runVideoURL, Model: "m" + injected, ModelVersion: "v" + injected}
				e.d.newWriter = func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error) {
					return &writertestutil.FakeArticleWriter{Result: article}, nil
				}
				e.d.newPublisher = func(string) (publisher.Publisher, error) {
					return &publishertestutil.FakePublisher{}, nil
				}
			},
			wantCode: exitOK,
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
			if !strings.Contains(stderr, `\x1b[2J\nFAKE-LINE`) {
				t.Errorf("stderr does not hold the escaped injection:\n%s", stderr)
			}
			requireSafeOutput(t, e.stdout.String(), stderr, e.outPath)
		})
	}
}

// TestRunGODEBUGWarning checks that http2debug=1 or 2 in GODEBUG is warned
// about before the LLM call, without the API key, and without changing the
// exit code; any other GODEBUG gives no warning.
func TestRunGODEBUGWarning(t *testing.T) {
	const warning = "GODEBUG enables http2debug"
	cases := []struct {
		name        string
		godebug     *string
		wantWarning bool
	}{
		{"unset", nil, false},
		{"http2debug=0", new("http2debug=0"), false},
		{"http2debug=1", new("http2debug=1"), true},
		{"http2debug=2", new("http2debug=2"), true},
		{"http2debug=1 after another setting", new("madvdontneed=1,http2debug=1"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			if tc.godebug != nil {
				e.env["GODEBUG"] = *tc.godebug
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
