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
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/artifact"
	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	llmtestutil "github.com/isseis/yt2column/internal/llm/testutil"
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

	// testAPIKey is a distinctive secret value. Its last eight characters
	// differ from every other string a run writes.
	testAPIKey = "sk-unit-APIKEYVALUE-0123456789-TAIL8KEY"
	// testAnthropicKey is the Anthropic key of a claude configuration; a
	// failure whose message holds it must be redacted too.
	testAnthropicKey = "sk-ant-APIKEYVALUE-0123456789-TAIL8ANT"
	lockFileName     = ".yt2column.lock"

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

// secretStrings returns every string that must never be written: the API key
// and its last eight characters.
func secretStrings() []string {
	return []string{testAPIKey, testAPIKey[len(testAPIKey)-8:]}
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
	// withSteps keeps the post-generation steps enabled. By default the run
	// disables both, so the tests of this package keep their pre-step
	// behavior; only a test of the steps sets it.
	withSteps bool
	d         deps
	stdout    bytes.Buffer
	stderr    bytes.Buffer
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
	// The provider is explicit so the tests keep selecting DeepSeek, not the
	// default provider.
	e.env = map[string]string{
		"YT2COLUMN_LLM":        "deepseek/fake-model",
		"DEEPSEEK_API_KEY":     testAPIKey,
		"YT2COLUMN_CACHE_DIR":  e.cacheDir,
		"YT2COLUMN_YTDLP_PATH": tripwire,
	}
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return validResponse(), nil
	})
	e.d = productionDeps()
	e.d.newLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
		return e.counter.wrap(e.client), nil
	}
	e.d.newReviewLLMClient = refusingReviewLLMClient
	return e
}

// respondWith sets the LLM client to return text as the article.
func (e *runEnv) respondWith(text string) {
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		resp := validResponse()
		resp.Text = text
		return resp, nil
	})
}

// longGeneratedText is generated text whose body is over maxBodyChars, so the
// shorten step runs.
func longGeneratedText() string {
	return "# A long title\n\n" + strings.Repeat("word ", 900) + "\n"
}

// shortGeneratedText is generated text whose body is within maxBodyChars.
func shortGeneratedText() string {
	return "# A long title\n\nA short body.\n"
}

// emptyReviewJSON is a valid review response that revises nothing.
const emptyReviewJSON = `{"revisions":[]}`

// reviewJSON returns a valid review response with one replacement revision.
func reviewJSON(before, after string) string {
	return fmt.Sprintf(`{"revisions":[{"before":%q,"after":%q,"reason":"meaning_change","evidence":"the source says so"}]}`, before, after)
}

// reviewLLM returns a client that returns text as the review response.
func reviewLLM(text string) llm.LLMClient {
	return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return llm.GenerateResponse{Text: text, Model: "review-model", ModelVersion: "rv1"}, nil
	})
}

// useReviewClient makes the run build reviewClient for the review step.
func (e *runEnv) useReviewClient(client llm.LLMClient) {
	e.d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) { return client, nil }
}

// cliArgs returns the arguments run receives. Unless the test enabled the
// steps, the arguments that disable both come first, so the default (both on)
// does not change what a test of something else observes.
func (e *runEnv) cliArgs() []string {
	args := e.args
	if args == nil {
		args = []string{"--out", e.outPath, runVideoURL}
	}
	if e.withSteps {
		return args
	}
	return append(slices.Clone(noStepsArgs), args...)
}

// run calls run once and returns its exit code.
func (e *runEnv) run() int {
	return run(e.ctx, e.cliArgs(), lookupFrom(e.env), &e.stdout, &e.stderr, e.d)
}

// videoEntries returns the cache entries of runVideoID; none when the cache
// directory cannot exist. It is not videoCacheEntries: this helper also
// tolerates a cache path that is not a directory, because a test of this
// package makes it a regular file.
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

// afterFilePublish makes the run's --out publisher write the article with the
// real FilePublisher and then call then, whose error becomes the publish's
// own. A test uses it to act at the end of the publish stage, after every
// other stage ran.
func (e *runEnv) afterFilePublish(then func() error) {
	e.d.newFilePublisher = func(path string) (publisher.Publisher, error) {
		inner, err := newFilePublisher(path)
		if err != nil {
			return nil, err
		}
		return publisherFunc(func(ctx context.Context, a writer.Article) error {
			if err := inner.Publish(ctx, a); err != nil {
				return err
			}
			return then()
		}), nil
	}
}

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
	// articlePublished marks a run that published the article and then failed,
	// as a review-log write failure does: --out is created and the video's
	// cache is removed, so the failure-only checks do not apply.
	articlePublished bool
	wantStderr       []string
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
		run(ctx, append(slices.Clone(noStepsArgs), "--refresh", "--out", holderOut, runVideoURL), lookupFrom(env), &stdout, &stderr, testDeps(validResponseLLM()))
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
		usageRow("no --out", func(*runEnv) []string { return []string{runVideoURL} }, "--out is required"),
		usageRow("a video URL on another host", func(e *runEnv) []string {
			return []string{"--out", e.outPath, "https://example.com/watch?v=dQw4w9WgXcQ"}
		}, "invalid video URL"),
		usageRow("a short video ID", func(e *runEnv) []string {
			return []string{"--out", e.outPath, "https://www.youtube.com/watch?v=short"}
		}, "invalid video URL"),
		usageRow("-rf after --", func(e *runEnv) []string { return []string{"--out", e.outPath, "--", "-rf"} }, "invalid video URL"),

		envRow("YT2COLUMN_LLM empty", func(env map[string]string) { env["YT2COLUMN_LLM"] = "" }, "YT2COLUMN_LLM"),
		envRow("DEEPSEEK_API_KEY empty", func(env map[string]string) { env["DEEPSEEK_API_KEY"] = "" }, "DEEPSEEK_API_KEY"),
		envRow("YT2COLUMN_LLM invalid provider", func(env map[string]string) { env["YT2COLUMN_LLM"] = "MARKER-PROVIDER/fake-model" }, "YT2COLUMN_LLM"),
		envRow("YT2COLUMN_CACHE_DIR relative", func(env map[string]string) { env["YT2COLUMN_CACHE_DIR"] = "MARKER-CACHE" }, "YT2COLUMN_CACHE_DIR"),
		envRow("YT2COLUMN_YTDLP_PATH empty", func(env map[string]string) { env["YT2COLUMN_YTDLP_PATH"] = "" }, "YT2COLUMN_YTDLP_PATH"),
		envRow("several invalid variables", func(env map[string]string) {
			env["YT2COLUMN_LLM"] = "MARKER-PROVIDER/fake-model"
			env["YT2COLUMN_REVIEW_LLM"] = ""
		}, "YT2COLUMN_LLM", "YT2COLUMN_REVIEW_LLM"),

		{
			name: "YT2COLUMN_LLM model ID with surrounding whitespace",
			setup: func(_ *testing.T, e *runEnv) {
				e.env["YT2COLUMN_LLM"] = "deepseek/ fake-model "
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
			wantStderr: []string{"the write stage failed", "the server rejected " + "[REDACTED]"},
		},
		{
			name: "write failure whose message holds the Anthropic API key",
			setup: func(_ *testing.T, e *runEnv) {
				delete(e.env, "DEEPSEEK_API_KEY")
				e.env["YT2COLUMN_LLM"] = "claude/fake-model"
				e.env["ANTHROPIC_API_KEY"] = testAnthropicKey
				e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
					return llm.GenerateResponse{}, fmt.Errorf("%w: the server rejected %s", errStubLLM, testAnthropicKey)
				})
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the write stage failed", "the server rejected " + "[REDACTED]"},
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
			// The publisher is built before any side effect, so a failure to
			// build it is a usage error that touches nothing.
			name: "the --out publisher cannot be built",
			setup: func(_ *testing.T, e *runEnv) {
				e.d.newFilePublisher = func(string) (publisher.Publisher, error) {
					return nil, errInjectedPublish
				}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"build the publisher: injected publish failure", "yt2column -h"},
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
				e.afterFilePublish(func() error {
					cancel()
					return nil
				})
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
			wantStderr: []string{"the run failed: create the directory"},
			check: func(t *testing.T, e *runEnv) {
				requireNotLocked(t, e)
				// The phrase and the path are matched together, ending at the
				// colon, so a longer path under the cache directory (such as the
				// lock file) or the path on another line does not satisfy it.
				if want := "create the directory " + e.cacheDir + ":"; !strings.Contains(e.stderr.String(), want) {
					t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
				}
			},
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
			wantStderr: []string{"another run holds the directory lock", "yt-dlp"},
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
		},
		usageRow("--out with an empty path", func(*runEnv) []string {
			return []string{"--out", "", runVideoURL}
		}, "--out needs a path"),
		{
			name: "--review-log with --no-review",
			setup: func(_ *testing.T, e *runEnv) {
				e.withSteps = true
				e.args = []string{"--review-log", filepath.Join(e.base, "review.md"), "--no-review", "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"--review-log cannot be used with --no-review", "yt2column -h"},
		},
		{
			name: "--review-log with an empty path",
			setup: func(_ *testing.T, e *runEnv) {
				e.withSteps = true
				e.args = []string{"--review-log", "", "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"--review-log needs a path", "yt2column -h"},
		},
		{
			name: "--review-log names the same file as --out",
			setup: func(_ *testing.T, e *runEnv) {
				e.withSteps = true
				e.args = []string{"--review-log", e.outPath, "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"--out and --review-log cannot name the same file", "yt2column -h"},
		},
		{
			name: "review-log inside the cache directory",
			setup: func(_ *testing.T, e *runEnv) {
				e.withSteps = true
				e.args = []string{"--review-log", filepath.Join(e.cacheDir, "review.md"), "--out", e.outPath, runVideoURL}
			},
			wantCode:   exitUsage,
			wantStderr: []string{"inside the cache directory", "yt2column -h"},
		},
		{
			name: "review step fails",
			setup: func(_ *testing.T, e *runEnv) {
				e.withSteps = true
				e.useReviewClient(reviewLLM("not json"))
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the write stage failed", "review step", "--no-review"},
		},
		{
			name: "review-log write failure after the publish",
			setup: func(_ *testing.T, e *runEnv) {
				e.withSteps = true
				logPath := filepath.Join(e.base, "review.md")
				e.args = []string{"--review-log", logPath, "--out", e.outPath, runVideoURL}
				e.useReviewClient(reviewLLM(emptyReviewJSON))
				// The publisher creates the review log file after the article is
				// written, so the review log's link(2) fails with EEXIST.
				e.afterFilePublish(func() error { return os.WriteFile(logPath, []byte("raced"), 0o600) })
			},
			wantCode:         exitFailure,
			articlePublished: true,
			wantStderr:       []string{"do not run again"},
			notStderr:        []string{"the run failed:", "the finished article was kept in"},
			check: func(t *testing.T, e *runEnv) {
				// The lines name both paths, which the static lines above
				// cannot hold.
				for _, want := range []string{
					"the article was written to " + e.outPath,
					"writing the review log to " + filepath.Join(e.base, "review.md"),
				} {
					if !strings.Contains(e.stderr.String(), want) {
						t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
					}
				}
			},
		},
		{
			name: "review failure whose message holds the API key",
			setup: func(_ *testing.T, e *runEnv) {
				e.withSteps = true
				e.d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
					return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
						return llm.GenerateResponse{}, fmt.Errorf("review call failed with %s", testAPIKey)
					}), nil
				}
			},
			wantCode:   exitFailure,
			wantStderr: []string{"the write stage failed", "review step", "review call failed with " + "[REDACTED]"},
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
			}
			switch {
			case row.articlePublished:
				if _, err := os.Stat(e.outPath); err != nil {
					t.Errorf("--out was not created: %v", err)
				}
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

// TestCheckOut checks the --out rejections by their sentinels: an absent
// --out is errOutRequired, not errOutEmpty, an empty one is errOutEmpty, not
// errOutRequired, and a path passes.
func TestCheckOut(t *testing.T) {
	sentinels := []error{errOutRequired, errOutEmpty}
	cases := []struct {
		name string
		args []string
		want error
	}{
		{"absent", nil, errOutRequired},
		{"empty", []string{"--out", ""}, errOutEmpty},
		{"a path", []string{"--out", "article.md"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs, opts := newFlagSet()
			if err := fs.Parse(tc.args); err != nil {
				t.Fatalf("Parse(%q): %v", tc.args, err)
			}
			err := checkOut(fs, opts)
			if !errors.Is(err, tc.want) {
				t.Fatalf("checkOut() = %v, want %v", err, tc.want)
			}
			for _, other := range sentinels {
				if other != tc.want && errors.Is(err, other) {
					t.Errorf("checkOut() = %v, which is also %v", err, other)
				}
			}
		})
	}
}

// TestRunHelp checks the usage in detail: every flag of the CLI is listed,
// the text before the flags says --out is required, the --after description
// states that it requires --artifacts and that --after review does not accept
// --review-log, the usage goes to standard output only, and it needs no
// configuration.
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
			for _, flag := range []string{
				"--out <path>", "--refresh", "--keep-cache",
				"--system-prompt <path>", "--user-prompt <path>",
				"--no-shorten", "--no-review", "--review-log <path>",
				"--shorten-system-prompt <path>", "--shorten-user-prompt <path>",
				"--review-system-prompt <path>", "--review-user-prompt <path>",
				"--artifacts <path>", "--after <stage>",
				"requires --artifacts",
				"--after review does not accept --review-log",
				"-h, --help",
			} {
				if !strings.Contains(stdout.String(), flag) {
					t.Errorf("usage does not list %q:\n%s", flag, stdout.String())
				}
			}
			// The usage text before the flag list says the article goes to
			// the --out file, which is required, and points at the CLI guide
			// for the configuration.
			intro, _, ok := strings.Cut(stdout.String(), "\nFlags:\n")
			if !ok {
				t.Fatalf("usage has no flag list:\n%s", stdout.String())
			}
			for _, want := range []string{"writes it to the --out file", "--out is required", cliGuide} {
				if !strings.Contains(intro, want) {
					t.Errorf("the usage text before the flags does not contain %q:\n%s", want, intro)
				}
			}
		})
	}
	if _, err := os.Stat(cliDocPath); err != nil {
		t.Errorf("the help points at %s, which does not exist: %v", cliGuide, err)
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
	modelArticle := writer.Article{Title: "A title", Body: "The body.\n", SourceURL: runVideoURL, Model: "m" + injected, ModelVersion: "v" + injected}
	cases := []struct {
		name     string
		setup    func(t *testing.T, e *runEnv)
		wantCode int
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
// about before the LLM call, without the API key or the Webhook URL, and
// without changing the exit code; any other GODEBUG gives no warning. The
// warning is one line, about the API key: the CLI sends nothing but the LLM
// call.
func TestRunGODEBUGWarning(t *testing.T) {
	const (
		// anyWarning starts every GODEBUG warning line; warning is the
		// API-key one.
		anyWarning = "GODEBUG enables http2debug"
		warning    = anyWarning + ", so the Go HTTP/2 log may write the API key"
	)
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
			want := 0
			if tc.wantWarning {
				want = 1
			}
			if got := strings.Count(e.stderr.String(), anyWarning); got != want {
				t.Errorf("GODEBUG warning lines = %d, want %d:\n%s", got, want, e.stderr.String())
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

// TestRunStepFlags checks --no-review, --no-shorten, and their combination:
// each disables exactly its step, and disabling both restores the pre-step
// output.
func TestRunStepFlags(t *testing.T) {
	t.Run("--no-review does not build or call the review client", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-review", "--out", e.outPath, runVideoURL}
		reviewCalls := 0
		e.d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
			reviewCalls++
			return reviewLLM(emptyReviewJSON), nil
		}
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		if reviewCalls != 0 {
			t.Errorf("newReviewLLMClient calls = %d, want 0", reviewCalls)
		}
		if strings.Contains(e.stderr.String(), "the review step returned") {
			t.Errorf("stderr reports a review step although --no-review was given:\n%s", e.stderr.String())
		}
	})

	t.Run("--no-shorten keeps an over-long body", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--out", e.outPath, runVideoURL}
		e.respondWith(longGeneratedText())
		e.useReviewClient(reviewLLM(emptyReviewJSON))
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		// The generation client is also the shorten client, so one call proves
		// the shorten step did not run.
		if got := e.counter.count(); got != 1 {
			t.Errorf("Generate calls = %d, want 1", got)
		}
		if strings.Contains(e.stderr.String(), "the shorten step ran") {
			t.Errorf("stderr reports a shorten step although --no-shorten was given:\n%s", e.stderr.String())
		}
		if !strings.Contains(e.stderr.String(), "the review step returned") {
			t.Errorf("--no-shorten disabled the review step too:\n%s", e.stderr.String())
		}
	})

	t.Run("both disabled restore the pre-step output", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		data, err := os.ReadFile(e.outPath) //nolint:gosec // a path inside the test's temporary directory
		if err != nil {
			t.Fatalf("read --out: %v", err)
		}
		if !strings.Contains(string(data), "model: \"fake-model\"\nmodel_version: \"v1\"\n") {
			t.Errorf("--out does not have the pre-step model keys:\n%s", data)
		}
		if !strings.Contains(string(data), "\n---\n\n# A title\n\n") {
			t.Errorf("--out does not have the title heading after the front matter:\n%s", data)
		}
		if !strings.Contains(string(data), "The body.") {
			t.Errorf("--out does not hold the generated body:\n%s", data)
		}
		for _, key := range []string{"shorten_model:", "review_model:"} {
			if strings.Contains(string(data), key) {
				t.Errorf("--out holds the step key %q:\n%s", key, data)
			}
		}
		for _, line := range []string{"the shorten step ran", "the review step returned", "wrote the review log to"} {
			if strings.Contains(e.stderr.String(), line) {
				t.Errorf("stderr reports %q although both steps were disabled:\n%s", line, e.stderr.String())
			}
		}
	})
}

// TestRunDefaultSteps checks that the default run calls both steps, builds the
// review client once, and writes a --out model line per step that ran.
func TestRunDefaultSteps(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	generation := 0
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		generation++
		if generation == 1 {
			return llm.GenerateResponse{Text: longGeneratedText(), Model: "fake-model", ModelVersion: "v1"}, nil
		}
		return llm.GenerateResponse{Text: shortGeneratedText(), Model: "fake-model", ModelVersion: "v1"}, nil
	})
	reviewCalls := 0
	e.d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
		reviewCalls++
		return reviewLLM(emptyReviewJSON), nil
	}
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if generation != 2 {
		t.Errorf("generation client calls = %d, want 2 (generation, shorten)", generation)
	}
	if reviewCalls != 1 {
		t.Errorf("newReviewLLMClient calls = %d, want 1", reviewCalls)
	}
	for _, want := range []string{"the shorten step ran", "the review step returned no revisions"} {
		if !strings.Contains(e.stderr.String(), want) {
			t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
		}
	}
	data, err := os.ReadFile(e.outPath) //nolint:gosec // a path inside the test's temporary directory
	if err != nil {
		t.Fatalf("read --out: %v", err)
	}
	for _, want := range []string{"\nshorten_model: \"fake-model\"\n", "\nreview_model: \"review-model\"\n", "channel: \"A channel\"\n", "\n# A long title\n\nA channel\n\n"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("--out does not contain %q:\n%s", want, data)
		}
	}
}

// TestRunDefaultWriterOptions checks that a run with no step flags builds the
// writer with writertestutil.DefaultOptions, the value the server's writer is
// also checked against, so the two defaults cannot drift apart.
func TestRunDefaultWriterOptions(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	// A pointer client, so reflect.DeepEqual can match it (a func-typed client
	// never compares equal).
	review := &llmtestutil.FakeLLMClient{Result: llm.GenerateResponse{Text: emptyReviewJSON, Model: "review-model"}}
	e.useReviewClient(review)
	var got []writer.Options
	e.d.newWriter = func(client llm.LLMClient, opts writer.Options) (writer.ArticleWriter, error) {
		got = append(got, opts)
		return writer.New(client, opts)
	}
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if len(got) != 1 {
		t.Fatalf("newWriter calls = %d, want 1", len(got))
	}
	if want := writertestutil.DefaultOptions(review); !reflect.DeepEqual(got[0], want) {
		t.Errorf("writer options = %+v, want %+v", got[0], want)
	}
}

// TestRunReviewLog checks --review-log: the list is written after a successful
// publish, and the summary names it.
func TestRunReviewLog(t *testing.T) {
	t.Run("--out", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		logPath := filepath.Join(e.base, "review.md")
		e.args = []string{"--review-log", logPath, "--out", e.outPath, runVideoURL}
		e.useReviewClient(reviewLLM(reviewJSON("The body.", "A revised body.")))
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		data, err := os.ReadFile(logPath) //nolint:gosec // a path inside the test's temporary directory
		if err != nil {
			t.Fatalf("read the review log: %v", err)
		}
		if !strings.Contains(string(data), "# Review log") {
			t.Errorf("the review log has no header:\n%s", data)
		}
		article, err := os.ReadFile(e.outPath) //nolint:gosec // a path inside the test's temporary directory
		if err != nil {
			t.Fatalf("read --out: %v", err)
		}
		// The body is short, so the shorten step did not run: --out has no
		// shorten key, only the review key. The positive lines are checked in
		// TestRunDefaultSteps.
		if strings.Contains(string(article), "shorten_model:") {
			t.Errorf("--out holds a shorten key although the shorten step did not run:\n%s", article)
		}
		if !strings.Contains(string(article), "review_model: \"review-model\"") {
			t.Errorf("--out does not hold the review key:\n%s", article)
		}
		for _, want := range []string{"wrote the article to", "wrote the review log to " + logPath, "the review step returned 1 revision (meaning_change 1)"} {
			if !strings.Contains(e.stderr.String(), want) {
				t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
			}
		}
		requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
	})
}

// TestRunTooLongHint checks that ErrTooLong suggests --no-shorten.
func TestRunTooLongHint(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.respondWith(longGeneratedText())
	e.useReviewClient(reviewLLM(emptyReviewJSON))
	if code := e.run(); code != exitFailure {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, e.stderr.String())
	}
	stderr := e.stderr.String()
	for _, want := range []string{"the write stage failed", "shorten step", "exceeds the length limit", "--no-shorten"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, stderr)
		}
	}
}

// TestRunReviewMalformedHint checks that the --no-review hint does not appear
// for a generation or shorten rejection. The review rejection that does show
// it is the "review step fails" row of executionPathRows.
func TestRunReviewMalformedHint(t *testing.T) {
	t.Run("generation", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.respondWith("not a heading")
		e.useReviewClient(reviewLLM(emptyReviewJSON))
		if code := e.run(); code != exitFailure {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, e.stderr.String())
		}
		if strings.Contains(e.stderr.String(), "--no-review") {
			t.Errorf("stderr suggests --no-review for a generation rejection:\n%s", e.stderr.String())
		}
	})

	t.Run("shorten step", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		generation := 0
		e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			generation++
			if generation == 1 {
				return llm.GenerateResponse{Text: longGeneratedText(), Model: "fake-model", ModelVersion: "v1"}, nil
			}
			return llm.GenerateResponse{Text: "not a heading", Model: "fake-model", ModelVersion: "v1"}, nil
		})
		e.useReviewClient(reviewLLM(emptyReviewJSON))
		if code := e.run(); code != exitFailure {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, e.stderr.String())
		}
		stderr := e.stderr.String()
		if !strings.Contains(stderr, "shorten step") {
			t.Errorf("stderr does not name the shorten step:\n%s", stderr)
		}
		if strings.Contains(stderr, "--no-review") {
			t.Errorf("stderr suggests --no-review for a shorten rejection:\n%s", stderr)
		}
	})
}

// TestRunInvalidStepTemplate checks that a rejected step template override is
// a usage error before any LLM call, for each of the four flags.
func TestRunInvalidStepTemplate(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "bad.tmpl")
	if err := os.WriteFile(badPath, []byte(`{{define "x"}}{{end}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	validPath := filepath.Join(t.TempDir(), "valid.tmpl")
	if err := os.WriteFile(validPath, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"--shorten-system-prompt", []string{"--shorten-system-prompt", badPath}},
		{"--shorten-user-prompt", []string{"--shorten-user-prompt", badPath}},
		{"--review-system-prompt", []string{"--review-system-prompt", badPath}},
		{"--review-user-prompt", []string{"--review-user-prompt", badPath}},
		{"disabled review step with a template override", []string{"--no-review", "--review-system-prompt", validPath}},
		{"disabled shorten step with a template override", []string{"--no-shorten", "--shorten-user-prompt", validPath}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			e.withSteps = true
			e.args = append(slices.Clone(tc.args), "--out", e.outPath, runVideoURL)
			e.useReviewClient(reviewLLM(emptyReviewJSON))
			if code := e.run(); code != exitUsage {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
			}
			if !strings.Contains(e.stderr.String(), "build the article writer") {
				t.Errorf("stderr does not report building the article writer:\n%s", e.stderr.String())
			}
			if got := e.counter.count(); got != 0 {
				t.Errorf("Generate calls = %d, want 0", got)
			}
		})
	}
}

// TestRunClaudeConfigurationErrorsOneLineEach checks that when the CLI loads a
// claude environment with more than one rejected provider variable, it writes
// each rejection on its own line. A nested errors.Join would otherwise render
// them as one line with an escaped newline.
func TestRunClaudeConfigurationErrorsOneLineEach(t *testing.T) {
	e := newRunEnv(t)
	// The effort part is rejected, but the provider part still selects
	// claude, whose missing key is the second rejection.
	e.env["YT2COLUMN_LLM"] = "claude/fake-model+High"
	delete(e.env, "DEEPSEEK_API_KEY")
	delete(e.env, "ANTHROPIC_API_KEY")
	if code := e.run(); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
	}
	stderr := e.stderr.String()
	lines := strings.Split(stderr, "\n")
	for _, name := range []string{"ANTHROPIC_API_KEY", "YT2COLUMN_LLM"} {
		prefix := programName + ": configuration: " + name
		if !slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, prefix) }) {
			t.Errorf("stderr has no line starting with %q:\n%s", prefix, stderr)
		}
	}
}

// TestRunDefaultLLMConfig checks, at the CLI entry point, that a run with
// YT2COLUMN_LLM unset builds its generation client from the claude provider
// and the default model.
func TestRunDefaultLLMConfig(t *testing.T) {
	e := newRunEnv(t)
	delete(e.env, "YT2COLUMN_LLM")
	delete(e.env, "DEEPSEEK_API_KEY")
	e.env["ANTHROPIC_API_KEY"] = testAnthropicKey
	var built []config.LLMConfig
	e.d.newLLMClient = func(cfg config.LLMConfig) (llm.LLMClient, error) {
		built = append(built, cfg)
		return e.counter.wrap(e.client), nil
	}
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if len(built) != 1 {
		t.Fatalf("generation clients built = %d, want 1", len(built))
	}
	if got := built[0].Generation().Provider(); got != config.ProviderClaude {
		t.Errorf("Provider() = %v, want ProviderClaude", got)
	}
	if got, want := built[0].Generation().Model(), config.DefaultClaudeModelForTest(); got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
}

// TestRunRejectsInvalidEffortPart checks that an LLM value whose effort part
// the provider does not accept stops the run with the usage exit code before
// any LLM client is built, so the value is never sent as a model ID.
func TestRunRejectsInvalidEffortPart(t *testing.T) {
	e := newRunEnv(t)
	e.env["YT2COLUMN_LLM"] = "claude/claude-opus-5-5+ultra"
	delete(e.env, "DEEPSEEK_API_KEY")
	e.env["ANTHROPIC_API_KEY"] = testAnthropicKey
	built := 0
	e.d.newLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
		built++
		return e.counter.wrap(e.client), nil
	}
	e.d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
		built++
		return e.counter.wrap(e.client), nil
	}
	if code := e.run(); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
	}
	// The printed line is the contract here: it is all the user sees.
	prefix := programName + ": configuration: YT2COLUMN_LLM"
	if !slices.ContainsFunc(strings.Split(e.stderr.String(), "\n"), func(line string) bool { return strings.HasPrefix(line, prefix) }) {
		t.Errorf("stderr has no line starting with %q:\n%s", prefix, e.stderr.String())
	}
	if built != 0 {
		t.Errorf("LLM clients built = %d, want 0", built)
	}
}

// TestTestDepsReviewClientBuildsNothing checks that the default
// newReviewLLMClient of testDeps and of newRunEnv builds nothing: with the
// steps enabled, a run stops at building the review client with the stub's
// error.
func TestTestDepsReviewClientBuildsNothing(t *testing.T) {
	for name, depsFor := range map[string]func(e *runEnv) deps{
		"testDeps":  func(*runEnv) deps { return testDeps(validResponseLLM()) },
		"newRunEnv": func(e *runEnv) deps { return e.d },
	} {
		t.Run(name, func(t *testing.T) {
			e := newRunEnv(t)
			e.d = depsFor(e)
			e.withSteps = true
			if code := e.run(); code != exitUsage {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
			}
			if want := "build the review LLM client: " + errReviewClientNotSubstituted.Error(); !strings.Contains(e.stderr.String(), want) {
				t.Errorf("stderr does not contain %q:\n%s", want, e.stderr.String())
			}
		})
	}
}

// artifactsPath returns a fresh artifacts directory under base.
func artifactsPath(t *testing.T, base string) string {
	t.Helper()
	dir := filepath.Join(base, "artifacts")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// artifactFile returns the path of videoID's stage artifact under dir.
func artifactFile(dir, videoID, stage string) string {
	return filepath.Join(dir, videoID+"."+stage+".json")
}

// requireArtifactStages asserts dir holds a parseable artifact of each stage.
func requireArtifactStages(t *testing.T, dir, videoID string, stages ...string) {
	t.Helper()
	for _, stage := range stages {
		data, err := os.ReadFile(artifactFile(dir, videoID, stage))
		if err != nil {
			t.Errorf("read the %s artifact: %v", stage, err)
			continue
		}
		record, err := artifact.Parse(data)
		if err != nil {
			t.Errorf("parse the %s artifact: %v", stage, err)
			continue
		}
		if record.Stage.String() != stage {
			t.Errorf("%s file holds stage %v", stage, record.Stage)
		}
	}
}

// requireUntouched asserts a run touched nothing external: yt-dlp was not
// started, the LLM was not called, and --out does not exist.
func requireUntouched(t *testing.T, e *runEnv) {
	t.Helper()
	if _, err := os.Stat(e.tripwireMarker); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("yt-dlp was started (tripwire marker stat error = %v)", err)
	}
	if got := e.counter.count(); got != 0 {
		t.Errorf("the LLM was called %d times, want 0", got)
	}
	requireNoFile(t, e.outPath)
}

// firstLongThenShort sets e.client to answer the generation call with an
// over-long body and the shorten call with a short one, so the shorten step
// runs.
func firstLongThenShort(e *runEnv) {
	calls := 0
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		calls++
		if calls == 1 {
			return llm.GenerateResponse{Text: longGeneratedText(), Model: "fake-model", ModelVersion: "v1"}, nil
		}
		return llm.GenerateResponse{Text: shortGeneratedText(), Model: "fake-model", ModelVersion: "v1"}, nil
	})
}

// saveArtifactsOnce runs a run with --artifacts and --keep-cache, set up by
// setup, and returns the artifacts directory. The cache is kept so a later
// resume in the same test reuses the seeded transcript.
func saveArtifactsOnce(t *testing.T, setup func(*runEnv)) string {
	t.Helper()
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base)
	e.args = []string{"--artifacts", dir, "--keep-cache", "--out", e.outPath, runVideoURL}
	setup(e)
	e.useReviewClient(reviewLLM(emptyReviewJSON))
	if code := e.run(); code != exitOK {
		t.Fatalf("the artifacts run failed: exit code = %d\nstderr:\n%s", code, e.stderr.String())
	}
	return dir
}

// shortGeneration keeps the generation body within the limit, so the shorten
// step does not run.
func shortGeneration(e *runEnv) { e.respondWith(shortGeneratedText()) }

func TestRunArtifactsSavesAllStages(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base)
	e.args = []string{"--artifacts", dir, "--out", e.outPath, runVideoURL}
	firstLongThenShort(e)
	e.useReviewClient(reviewLLM(reviewJSON("A short body.", "A shorter body.")))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	requireArtifactStages(t, dir, runVideoID, "generation", "shorten", "review")
	if !strings.Contains(e.stderr.String(), "saved the artifacts to "+dir) {
		t.Errorf("stderr does not report the artifacts directory:\n%s", e.stderr.String())
	}
	requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
}

func TestRunAfterGeneration(t *testing.T) {
	dir := saveArtifactsOnce(t, shortGeneration)
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", dir, "--after", "generation", "--out", e.outPath, runVideoURL}
	e.useReviewClient(reviewLLM(emptyReviewJSON))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if got := e.counter.count(); got != 0 {
		t.Errorf("the generation client was called %d times, want 0 (reused, within the limit)", got)
	}
	if !strings.Contains(e.stderr.String(), "the review step returned") {
		t.Errorf("the review step did not run on the resume:\n%s", e.stderr.String())
	}
}

func TestRunAfterShorten(t *testing.T) {
	dir := saveArtifactsOnce(t, firstLongThenShort)
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", dir, "--after", "shorten", "--out", e.outPath, runVideoURL}
	e.useReviewClient(reviewLLM(emptyReviewJSON))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if got := e.counter.count(); got != 0 {
		t.Errorf("the generation client was called %d times, want 0 (generation and shorten were skipped)", got)
	}
	if !strings.Contains(e.stderr.String(), "the review step returned") {
		t.Errorf("the review step did not run on the resume:\n%s", e.stderr.String())
	}
}

func TestRunAfterShortenOutModelLines(t *testing.T) {
	dir := saveArtifactsOnce(t, firstLongThenShort)
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", dir, "--after", "shorten", "--out", e.outPath, runVideoURL}
	e.useReviewClient(reviewLLM(emptyReviewJSON))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	data, err := os.ReadFile(e.outPath) //nolint:gosec // a path inside the test's temporary directory
	if err != nil {
		t.Fatalf("read --out: %v", err)
	}
	for _, want := range []string{"\nmodel: \"fake-model\"\n", "\nshorten_model: \"fake-model\"\n", "\nreview_model: \"review-model\"\n"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("--out does not contain %q:\n%s", want, data)
		}
	}
}

func TestRunAfterReview(t *testing.T) {
	dir := saveArtifactsOnce(t, shortGeneration)
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}
	e.d.newLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
		t.Error("built the generation client for --after review")
		return validResponseLLM(), nil
	}
	e.d.newReviewLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
		t.Error("built the review client for --after review")
		return nil, nil
	}

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "model: fake-model") {
		t.Errorf("the summary does not carry the reused model:\n%s", e.stderr.String())
	}
}

func TestRunResumeConfigScoping(t *testing.T) {
	// --after review runs no LLM, so its configuration is not required.
	dir := saveArtifactsOnce(t, shortGeneration)
	e := newRunEnv(t)
	e.withSteps = true
	delete(e.env, "YT2COLUMN_LLM")
	delete(e.env, "DEEPSEEK_API_KEY")
	e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d with YT2COLUMN_LLM and the key unset\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
}

func TestRunResumeSavesExecutedStages(t *testing.T) {
	dir := saveArtifactsOnce(t, firstLongThenShort)
	// Remove the stages the resume will re-run, so their re-creation proves the
	// resume saved them.
	for _, stage := range []string{"shorten", "review"} {
		if err := os.Remove(artifactFile(dir, runVideoID, stage)); err != nil {
			t.Fatal(err)
		}
	}
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", dir, "--after", "generation", "--out", e.outPath, runVideoURL}
	e.useReviewClient(reviewLLM(emptyReviewJSON))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	requireArtifactStages(t, dir, runVideoID, "shorten", "review")
}

func TestRunResumeFetchesTranscript(t *testing.T) {
	t.Run("reuses the cached transcript", func(t *testing.T) {
		dir := saveArtifactsOnce(t, shortGeneration)
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		if _, err := os.Stat(e.tripwireMarker); !errors.Is(err, os.ErrNotExist) {
			t.Error("yt-dlp was started although the transcript was cached")
		}
	})

	t.Run("fetches when the cache is gone", func(t *testing.T) {
		dir := saveArtifactsOnce(t, shortGeneration)
		e := newRunEnv(t)
		e.withSteps = true
		removeVideoCache(t, e.cacheDir)
		e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}
		if code := e.run(); code != exitFailure {
			t.Fatalf("exit code = %d, want %d (yt-dlp is a tripwire)\nstderr:\n%s", code, exitFailure, e.stderr.String())
		}
		if _, err := os.Stat(e.tripwireMarker); err != nil {
			t.Errorf("yt-dlp was not started although the cache was gone: %v", err)
		}
	})
}

func TestRunReviewLogNotInput(t *testing.T) {
	// A review log left on disk is never read as the resume's input.
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base)
	logPath := filepath.Join(e.base, "review-log.md")
	e.args = []string{"--artifacts", dir, "--keep-cache", "--review-log", logPath, "--out", e.outPath, runVideoURL}
	firstLongThenShort(e)
	e.useReviewClient(reviewLLM(emptyReviewJSON))
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	// Replace the review log with something no parser would accept.
	if err := os.WriteFile(logPath, []byte("not a review list"), 0o600); err != nil {
		t.Fatal(err)
	}

	e2 := newRunEnv(t)
	e2.withSteps = true
	e2.args = []string{"--artifacts", dir, "--after", "shorten", "--out", e2.outPath, runVideoURL}
	e2.useReviewClient(reviewLLM(emptyReviewJSON))
	if code := e2.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d (the review log must not be read)\nstderr:\n%s", code, exitOK, e2.stderr.String())
	}
}

func TestRunArtifactsInsideCacheDir(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", e.cacheDir, "--out", e.outPath, runVideoURL}
	if code := e.run(); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "inside the cache directory") {
		t.Errorf("stderr does not name the cache directory rule:\n%s", e.stderr.String())
	}
	requireUntouched(t, e)
}

func TestRunArtifactsBadDir(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", filepath.Join(e.base, "no-such-artifacts"), "--out", e.outPath, runVideoURL}
	if code := e.run(); code != exitFailure {
		t.Fatalf("exit code = %d, want %d (an unusable directory is a run failure)\nstderr:\n%s", code, exitFailure, e.stderr.String())
	}
	requireUntouched(t, e)
}

func TestRunAfterRequiresArtifacts(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--after", "generation", "--out", e.outPath, runVideoURL}
	if code := e.run(); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "--after requires --artifacts") {
		t.Errorf("stderr does not name the rule:\n%s", e.stderr.String())
	}
	requireUntouched(t, e)
}

func TestRunAfterInvalidValue(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base)
	e.args = []string{"--after", "generationz", "--artifacts", dir, "--out", e.outPath, runVideoURL}
	if code := e.run(); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "generationz") {
		t.Errorf("stderr does not name the rejected value:\n%s", e.stderr.String())
	}
	requireUntouched(t, e)
}

func TestRunAfterRejectsGenerationFlags(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base)
	prompt := filepath.Join(e.base, "system.tmpl")
	if err := os.WriteFile(prompt, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.args = []string{"--after", "generation", "--artifacts", dir, "--system-prompt", prompt, "--out", e.outPath, runVideoURL}
	if code := e.run(); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "--after cannot be used with --system-prompt") {
		t.Errorf("stderr does not name the CLI rule (the writer would reject it too):\n%s", e.stderr.String())
	}
	requireUntouched(t, e)
}

func TestRunAfterReviewRejectsReviewLog(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base)
	e.args = []string{"--after", "review", "--artifacts", dir, "--review-log", filepath.Join(e.base, "log.md"), "--out", e.outPath, runVideoURL}
	if code := e.run(); code != exitUsage {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
	}
	requireUntouched(t, e)
}

func TestRunAfterMissingArtifact(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base) // empty: no generation artifact
	e.args = []string{"--artifacts", dir, "--after", "generation", "--out", e.outPath, runVideoURL}
	// The writer is built before the artifact is loaded, so a review client is
	// needed for the build to reach the load.
	e.useReviewClient(reviewLLM(emptyReviewJSON))
	if code := e.run(); code != exitFailure {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, e.stderr.String())
	}
	requireUntouched(t, e)
}

func TestRunArtifactExitCodes(t *testing.T) {
	validGeneration := func(input string) []byte {
		record := writer.StageRecord{
			Stage: writer.StageGeneration, Title: "T", Body: "B.", Model: "m",
			System:   writer.TemplateIdentity{Embedded: true, Digest: writer.Digest("s")},
			User:     writer.TemplateIdentity{Embedded: true, Digest: writer.Digest("u")},
			Input:    input,
			CallTime: time.Date(2026, time.October, 16, 1, 0, 0, 0, time.UTC),
		}
		data, _, err := artifact.Marshal(record)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		return data
	}
	cases := []struct {
		name string
		want int
		// wantStderr, when set, must appear in the run's standard error. It
		// isolates a rejection to the rule under test when another layer (the
		// id check) would also reject the same input.
		wantStderr string
		prepared   func(t *testing.T, e *runEnv, dir string)
		args       []string
	}{
		{
			name: "argument shape is a usage error",
			want: exitUsage,
			args: []string{"--after", "bogus", "--artifacts", "@dir", "--out", "@out", runVideoURL},
		},
		{
			name: "missing directory is a run failure",
			want: exitFailure,
			args: []string{"--artifacts", filepath.Join("@base", "absent"), "--out", "@out", runVideoURL},
		},
		{
			name: "a broken artifact is a run failure",
			want: exitFailure,
			args: []string{"--artifacts", "@dir", "--after", "generation", "--out", "@out", runVideoURL},
			prepared: func(t *testing.T, e *runEnv, dir string) {
				e.useReviewClient(reviewLLM(emptyReviewJSON))
				if err := os.WriteFile(artifactFile(dir, runVideoID, "generation"), []byte("not json"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a mismatched artifact is a run failure",
			want: exitFailure,
			args: []string{"--artifacts", "@dir", "--after", "generation", "--out", "@out", runVideoURL},
			prepared: func(t *testing.T, e *runEnv, dir string) {
				e.useReviewClient(reviewLLM(emptyReviewJSON))
				if err := os.WriteFile(artifactFile(dir, runVideoID, "generation"), validGeneration(writer.Digest("other")), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a malformed usage is a run failure",
			want: exitFailure,
			args: []string{"--artifacts", "@dir", "--after", "generation", "--out", "@out", runVideoURL},
			prepared: func(t *testing.T, e *runEnv, dir string) {
				e.useReviewClient(reviewLLM(emptyReviewJSON))
				// Save a real generation artifact (so its input matches the
				// material), then break only its usage: a valid usage would let
				// the run proceed, so the rejection is the usage rule's.
				saved := saveArtifactsOnce(t, shortGeneration)
				data, err := os.ReadFile(artifactFile(saved, runVideoID, "generation"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), `"usage":null`) {
					t.Fatalf("the saved artifact has no usage field to tamper with:\n%s", data)
				}
				tampered := strings.Replace(string(data), `"usage":null`, `"usage":"x"`, 1)
				if err := os.WriteFile(artifactFile(dir, runVideoID, "generation"), []byte(tampered), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a malformed call time is a run failure",
			want: exitFailure,
			// The call-time rule rejects before the id check, so the message
			// names call_time; without this the id check would mask a broken
			// call-time rule and the case would pass either way.
			wantStderr: "call_time",
			args:       []string{"--artifacts", "@dir", "--after", "generation", "--out", "@out", runVideoURL},
			prepared: func(t *testing.T, e *runEnv, dir string) {
				e.useReviewClient(reviewLLM(emptyReviewJSON))
				// Save a real generation artifact, then break only its
				// call_time with a spelling time.Parse rejects (a zone
				// offset). The time rule rejects it before the id check, so
				// the id need not be recomputed.
				saved := saveArtifactsOnce(t, shortGeneration)
				data, err := os.ReadFile(artifactFile(saved, runVideoID, "generation"))
				if err != nil {
					t.Fatal(err)
				}
				const key = `"call_time":"`
				start := strings.Index(string(data), key)
				if start < 0 {
					t.Fatalf("the saved artifact has no call_time to tamper with:\n%s", data)
				}
				valueStart := start + len(key)
				valueEnd := strings.Index(string(data)[valueStart:], `"`)
				if valueEnd < 0 {
					t.Fatalf("the saved artifact has an unterminated call_time:\n%s", data)
				}
				tampered := string(data)[:valueStart] + "2026-10-16T01:00:00+00:00" + string(data)[valueStart+valueEnd:]
				if err := os.WriteFile(artifactFile(dir, runVideoID, "generation"), []byte(tampered), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			e.withSteps = true
			dir := artifactsPath(t, e.base)
			if tc.prepared != nil {
				tc.prepared(t, e, dir)
			}
			args := make([]string, len(tc.args))
			for i, arg := range tc.args {
				switch arg {
				case "@dir":
					arg = dir
				case "@out":
					arg = e.outPath
				case "@base":
					arg = e.base
				}
				args[i] = arg
			}
			e.args = args
			if code := e.run(); code != tc.want {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, tc.want, e.stderr.String())
			}
			if tc.wantStderr != "" && !strings.Contains(e.stderr.String(), tc.wantStderr) {
				t.Errorf("stderr does not contain %q, so the rejection may come from another rule:\n%s", tc.wantStderr, e.stderr.String())
			}
			requireUntouched(t, e)
		})
	}
}

// TestResumeNeed checks, for every combination of --after and the step flags,
// the configuration the run needs and the clients buildWriter builds, and that
// a client is built only for a stage whose configuration was loaded: the
// generation accessors are empty unless Need.Generation is true, so a client
// built outside it would be built from no selection. No combination of these
// flags is rejected by the argument checks.
func TestResumeNeed(t *testing.T) {
	cases := []struct {
		after                      string // "" means no --after
		noShorten, noReview        bool
		wantGeneration, wantReview bool
		buildsGeneration           bool
		buildsReview               bool
	}{
		{"", false, false, true, true, true, true},
		{"", true, false, true, true, true, true},
		{"", false, true, true, true, true, false},
		{"", true, true, true, true, true, false},
		{"generation", false, false, true, true, true, true},
		{"generation", true, false, false, true, false, true},
		{"generation", false, true, true, false, true, false},
		{"generation", true, true, false, false, false, false},
		{"shorten", false, false, false, true, false, true},
		{"shorten", true, false, false, true, false, true},
		{"shorten", false, true, false, false, false, false},
		{"shorten", true, true, false, false, false, false},
		{"review", false, false, false, false, false, false},
		{"review", true, false, false, false, false, false},
		{"review", false, true, false, false, false, false},
		{"review", true, true, false, false, false, false},
	}
	for _, tc := range cases {
		name := fmt.Sprintf("after=%q noShorten=%v noReview=%v", tc.after, tc.noShorten, tc.noReview)
		t.Run(name, func(t *testing.T) {
			afterGiven := tc.after != ""
			need := resumeNeed(tc.after, tc.noShorten, tc.noReview, afterGiven)
			if need.Generation != tc.wantGeneration || need.Review != tc.wantReview {
				t.Errorf("resumeNeed() = %+v, want generation=%v review=%v", need, tc.wantGeneration, tc.wantReview)
			}

			resume := writer.StageUnset
			if afterGiven {
				var ok bool
				if resume, ok = afterStage(tc.after); !ok {
					t.Fatalf("afterStage(%q) rejected a valid stage", tc.after)
				}
			}
			var generationBuilt, reviewBuilt int
			d := deps{
				newLLMClient: func(config.LLMConfig) (llm.LLMClient, error) {
					generationBuilt++
					return validResponseLLM(), nil
				},
				newReviewLLMClient: func(config.LLMConfig) (llm.LLMClient, error) {
					reviewBuilt++
					return validResponseLLM(), nil
				},
				newWriter: func(llm.LLMClient, writer.Options) (writer.ArticleWriter, error) { return nil, nil },
			}
			opts := &cliOptions{after: tc.after, noShorten: tc.noShorten, noReview: tc.noReview}
			if _, _, err := buildWriter(d, config.Config{}, opts, false, nil, resume); err != nil {
				t.Fatalf("buildWriter() error = %v", err)
			}
			if got := generationBuilt == 1; got != tc.buildsGeneration || generationBuilt > 1 {
				t.Errorf("generation clients built = %d, want built %v", generationBuilt, tc.buildsGeneration)
			}
			if got := reviewBuilt == 1; got != tc.buildsReview || reviewBuilt > 1 {
				t.Errorf("review clients built = %d, want built %v", reviewBuilt, tc.buildsReview)
			}
			if generationBuilt > 0 && !need.Generation {
				t.Error("built the generation client for a run whose generation configuration was not loaded")
			}
			if reviewBuilt > 0 && !need.Review {
				t.Error("built the review client for a run whose review configuration was not loaded")
			}
		})
	}
}

// TestRunIgnoresRetiredLLMVars checks that the CLI neither stops on the
// retired generation variables, set empty or to values that would select
// another provider, model, and effort, nor uses their values: the run
// completes and builds its client from YT2COLUMN_LLM, or from the default
// when that is unset. The run has no review step; that no load reads the
// review ones either is TestLoadForIgnoresRetiredVars in internal/config.
func TestRunIgnoresRetiredLLMVars(t *testing.T) {
	towardClaude := map[string]string{
		"YT2COLUMN_LLM_PROVIDER": "claude", "YT2COLUMN_MODEL": "claude-opus-5-5", "YT2COLUMN_CLAUDE_EFFORT": "low",
	}
	towardDeepSeek := map[string]string{
		"YT2COLUMN_LLM_PROVIDER": "deepseek", "YT2COLUMN_MODEL": "deepseek-flash", "YT2COLUMN_CLAUDE_EFFORT": "low",
	}
	empty := map[string]string{"YT2COLUMN_LLM_PROVIDER": "", "YT2COLUMN_MODEL": "", "YT2COLUMN_CLAUDE_EFFORT": ""}
	withLLM := loadedGeneration(t, map[string]string{"YT2COLUMN_LLM": "deepseek/fake-model", "DEEPSEEK_API_KEY": testAPIKey})
	withoutLLM := loadedGeneration(t, map[string]string{"ANTHROPIC_API_KEY": testAnthropicKey})
	cases := []struct {
		name    string
		retired map[string]string
		llmSet  bool // YT2COLUMN_LLM keeps deepseek/fake-model; otherwise it is unset
		want    config.LLMSelection
	}{
		{"toward claude with YT2COLUMN_LLM", towardClaude, true, withLLM},
		{"empty with YT2COLUMN_LLM", empty, true, withLLM},
		{"toward deepseek without YT2COLUMN_LLM", towardDeepSeek, false, withoutLLM},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			maps.Copy(e.env, tc.retired)
			if !tc.llmSet {
				delete(e.env, "YT2COLUMN_LLM")
				e.env["ANTHROPIC_API_KEY"] = testAnthropicKey
			}
			var built []config.LLMSelection
			e.d.newLLMClient = func(cfg config.LLMConfig) (llm.LLMClient, error) {
				built = append(built, cfg.Generation())
				return e.counter.wrap(e.client), nil
			}
			if code := e.run(); code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
			}
			if len(built) != 1 || built[0] != tc.want {
				t.Errorf("generation selections built = %+v, want one %+v", built, tc.want)
			}
		})
	}
}

// loadedGeneration loads env, with a cache directory, and returns its
// generation selection: the selection a run with env builds.
func loadedGeneration(t *testing.T, env map[string]string) config.LLMSelection {
	t.Helper()
	env["YT2COLUMN_CACHE_DIR"] = t.TempDir()
	cfg, err := config.Load(lookupFrom(env))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg.LLM().Generation()
}

// TestRunNoReviewPaddedReviewModel checks that a run without --after and with
// --no-review succeeds when YT2COLUMN_REVIEW_LLM's model ID has surrounding
// white space: the configuration load checks the review value but accepts the
// model ID, and the review client, whose adapter would reject it, is not
// built. The real review client constructor is in place, so building it would
// fail the run. This is the end-to-end form of three checks made layer by
// layer elsewhere (the load accepts the model ID, the adapter rejects it, and
// buildWriter skips the review client under --no-review), so a change that
// keeps each layer's test but breaks how they combine still fails here.
func TestRunNoReviewPaddedReviewModel(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.env["YT2COLUMN_REVIEW_LLM"] = "deepseek/ fake-model "
	e.d.newReviewLLMClient = productionDeps().newReviewLLMClient
	e.args = []string{"--no-review", "--out", e.outPath, runVideoURL}
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
}

// TestRunAfterShortenWritesReviewLog checks that --review-log is accepted on a
// resume whose review step runs.
func TestRunAfterShortenWritesReviewLog(t *testing.T) {
	dir := saveArtifactsOnce(t, firstLongThenShort)
	e := newRunEnv(t)
	e.withSteps = true
	logPath := filepath.Join(e.base, "review-log.md")
	e.args = []string{"--artifacts", dir, "--after", "shorten", "--review-log", logPath, "--out", e.outPath, runVideoURL}
	e.useReviewClient(reviewLLM(emptyReviewJSON))
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("the review log was not written: %v", err)
	}
}

// TestRunAfterShortenEmptyModelVersion checks that a reused stage with an empty
// ModelVersion is written as (none), not omitted.
func TestRunAfterShortenEmptyModelVersion(t *testing.T) {
	dir := saveArtifactsOnce(t, func(e *runEnv) {
		calls := 0
		e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			calls++
			text := longGeneratedText()
			if calls > 1 {
				text = shortGeneratedText()
			}
			return llm.GenerateResponse{Text: text, Model: "fake-model"}, nil
		})
	})
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", dir, "--after", "shorten", "--out", e.outPath, runVideoURL}
	e.useReviewClient(reviewLLM(emptyReviewJSON))
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	data, err := os.ReadFile(e.outPath) //nolint:gosec // a path inside the test's temporary directory
	if err != nil {
		t.Fatalf("read --out: %v", err)
	}
	if !strings.Contains(string(data), "shorten_model_version: \"(none)\"") {
		t.Errorf("--out does not write the empty reused model version as (none):\n%s", data)
	}
}
