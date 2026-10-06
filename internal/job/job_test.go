//go:build test

package job

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cachelock"
	"github.com/isseis/yt2column/internal/pipeline"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/transcript/testutil"
	"github.com/isseis/yt2column/internal/writer"
	"github.com/isseis/yt2column/internal/writer/testutil"
)

const (
	jobVideoID   = "2tcCWM-sRBw"
	jobVideoURL  = "https://www.youtube.com/watch?v=" + jobVideoID
	otherVideoID = "dQw4w9WgXcQ"

	// runStartBound bounds how long a test waits for a fake yt-dlp to signal
	// readiness, so a fake that fails to start is reported instead of hanging.
	runStartBound = 30 * time.Second
	// runBound bounds how long a test waits for Run to return.
	runBound = 15 * time.Second
)

var (
	errInjectedWrite   = errors.New("injected write failure")
	errInjectedPublish = errors.New("injected publish failure")
)

// validSubtitles parses as a usable json3 subtitle file for any video ID.
const validSubtitles = `{"events":[{"tStartMs":0,"segs":[{"utf8":"hello from the transcript"}]}]}`

// infoFor returns an info.json whose id matches videoID.
func infoFor(videoID string) string {
	return `{"id":"` + videoID + `","title":"A title","channel":"A channel","description":"A description"}`
}

func validArticle() writer.Article {
	return writer.Article{
		Title:        "A title",
		Body:         "The body.\n",
		SourceURL:    jobVideoURL,
		Model:        "model-x",
		ModelVersion: "v1",
	}
}

// seedCache places a valid cache for videoID under dir.
func seedCache(t *testing.T, dir, videoID, subtitles, info string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	if err := transcript.SeedCacheForTest(dir, videoID, []byte(subtitles), []byte(info)); err != nil {
		t.Fatalf("SeedCacheForTest: %v", err)
	}
}

// requireTripwireNotRun asserts the tripwire marker is absent.
func requireTripwireNotRun(t *testing.T, marker string) {
	t.Helper()
	if _, err := os.Lstat(marker); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("yt-dlp was started: %s exists (error = %v)", marker, err)
	}
}

// requireNoFile asserts path does not exist.
func requireNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s exists (error = %v), want it absent", path, err)
	}
}

type runOutcome struct {
	result Result
	err    error
}

// startRun runs Run in a goroutine and returns the outcome channel.
func startRun(ctx context.Context, req Request) <-chan runOutcome {
	done := make(chan runOutcome, 1)
	go func() {
		result, err := Run(ctx, req)
		done <- runOutcome{result: result, err: err}
	}()
	return done
}

// awaitRun waits for a run started by startRun.
func awaitRun(t *testing.T, done <-chan runOutcome) runOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(runBound):
		t.Fatal("Run did not return within the bound")
		return runOutcome{}
	}
}

// waitForFile waits for path to appear.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(runStartBound)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s did not appear within %v", path, runStartBound)
}

// failPublisher fails every Publish.
type failPublisher struct{ err error }

func (p failPublisher) Publish(context.Context, writer.Article) error { return p.err }

// cancelPublisher cancels the context and returns its error during Publish.
type cancelPublisher struct{ cancel context.CancelFunc }

func (p cancelPublisher) Publish(ctx context.Context, _ writer.Article) error {
	p.cancel()
	return ctx.Err()
}

// publishThenCancel publishes with inner and then cancels the context, so a
// later stage (the cache removal) fails.
type publishThenCancel struct {
	inner  publisher.Publisher
	cancel context.CancelFunc
}

func (p publishThenCancel) Publish(ctx context.Context, a writer.Article) error {
	if err := p.inner.Publish(ctx, a); err != nil {
		return err
	}
	p.cancel()
	return nil
}

// cancelWriter cancels the context during Write, so the pipeline observes the
// cancellation after the write stage.
type cancelWriter struct {
	inner  writer.ArticleWriter
	cancel context.CancelFunc
}

func (w cancelWriter) Write(ctx context.Context, tr transcript.Transcript) (writer.Article, error) {
	w.cancel()
	return w.inner.Write(ctx, tr)
}

func TestRunRemovesCache(t *testing.T) {
	cacheDir := t.TempDir()
	seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
	seedCache(t, cacheDir, otherVideoID, validSubtitles, infoFor(otherVideoID))
	outPath, pub := newOutput(t, t.TempDir())
	tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())

	result, err := Run(context.Background(), Request{
		VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
		Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Article != validArticle() {
		t.Fatalf("Article = %+v, want %+v", result.Article, validArticle())
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("output not created: %v", err)
	}
	requireTripwireNotRun(t, marker)

	for _, name := range transcripttestutil.ListDir(t, cacheDir) {
		if strings.HasPrefix(name, jobVideoID+".") {
			t.Errorf("cache entry %q remains after a successful run", name)
		}
	}
	kept := false
	for _, name := range transcripttestutil.ListDir(t, cacheDir) {
		if strings.HasPrefix(name, otherVideoID+".") {
			kept = true
		}
	}
	if !kept {
		t.Error("another video's cache was removed")
	}
}

func TestRunKeepCache(t *testing.T) {
	cacheDir := t.TempDir()
	seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
	tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())
	req := Request{
		VideoURL: jobVideoURL, CacheDir: cacheDir, YtDlpPath: tripwire, KeepCache: true,
		Writer: &writertestutil.FakeArticleWriter{Result: validArticle()},
	}

	outPath, pub := newOutput(t, t.TempDir())
	req.OutPath, req.Publisher = outPath, pub
	if _, err := Run(context.Background(), req); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	requireTripwireNotRun(t, marker)

	// The cache is reused, so a second run does not start yt-dlp.
	outPath2, pub2 := newOutput(t, t.TempDir())
	req.OutPath, req.Publisher = outPath2, pub2
	if _, err := Run(context.Background(), req); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	requireTripwireNotRun(t, marker)
}

func TestRunFailureKeepsCache(t *testing.T) {
	cacheDir := t.TempDir()
	seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
	tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())

	t.Run("write failure", func(t *testing.T) {
		outPath, pub := newOutput(t, t.TempDir())
		writer := &writertestutil.FakeArticleWriter{Err: errInjectedWrite}
		_, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer: writer, Publisher: pub,
		})
		if !errors.Is(err, errInjectedWrite) {
			t.Fatalf("Run error = %v, want the injected write error", err)
		}
		requireNoFile(t, outPath)
	})
	t.Run("publish failure", func(t *testing.T) {
		outPath, _ := newOutput(t, t.TempDir())
		_, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer:    &writertestutil.FakeArticleWriter{Result: validArticle()},
			Publisher: failPublisher{err: errInjectedPublish},
		})
		if !errors.Is(err, errInjectedPublish) {
			t.Fatalf("Run error = %v, want the injected publish error", err)
		}
		requireNoFile(t, outPath)
	})

	// The cache survived both failures, so a later run does not start yt-dlp.
	outPath, pub := newOutput(t, t.TempDir())
	if _, err := Run(context.Background(), Request{
		VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
		Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
	}); err != nil {
		t.Fatalf("Run after the failures: %v", err)
	}
	requireTripwireNotRun(t, marker)
}

func TestRunPrunesDangling(t *testing.T) {
	t.Run("later stage succeeds", func(t *testing.T) {
		cacheDir := t.TempDir()
		seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
		dangling := filepath.Join(cacheDir, otherVideoID+".b")
		if err := os.MkdirAll(dangling, 0o700); err != nil {
			t.Fatalf("create dangling slot: %v", err)
		}
		outPath, pub := newOutput(t, t.TempDir())
		tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
		if _, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
		}); err != nil {
			t.Fatalf("Run: %v", err)
		}
		requireNoFile(t, dangling)
	})
	t.Run("later stage fails", func(t *testing.T) {
		cacheDir := t.TempDir()
		seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
		dangling := filepath.Join(cacheDir, otherVideoID+".b")
		if err := os.MkdirAll(dangling, 0o700); err != nil {
			t.Fatalf("create dangling slot: %v", err)
		}
		outPath, pub := newOutput(t, t.TempDir())
		tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
		_, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer: &writertestutil.FakeArticleWriter{Err: errInjectedWrite}, Publisher: pub,
		})
		if !errors.Is(err, errInjectedWrite) {
			t.Fatalf("Run error = %v, want the injected write error", err)
		}
		requireNoFile(t, dangling)
	})
}

func TestRunPruneFailureWarns(t *testing.T) {
	requireNonRoot(t)

	// setup returns a cache directory holding a dangling slot whose contents
	// cannot be removed (the directory is not writable), so pruning fails.
	setup := func(t *testing.T) string {
		t.Helper()
		cacheDir := t.TempDir()
		seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
		dangling := filepath.Join(cacheDir, otherVideoID+".b")
		if err := os.MkdirAll(dangling, 0o700); err != nil {
			t.Fatalf("create dangling slot: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dangling, "file"), []byte("x"), 0o600); err != nil {
			t.Fatalf("write dangling file: %v", err)
		}
		chmodForTest(t, dangling, 0o500)
		return cacheDir
	}

	t.Run("pipeline succeeds", func(t *testing.T) {
		cacheDir := setup(t)
		tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())
		outPath, pub := newOutput(t, t.TempDir())
		result, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(result.Warnings) == 0 {
			t.Fatal("no warning for the prune failure")
		}
		requireTripwireNotRun(t, marker)
		if _, err := os.Stat(outPath); err != nil {
			t.Fatalf("output not created: %v", err)
		}
	})
	t.Run("pipeline fails", func(t *testing.T) {
		cacheDir := setup(t)
		tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
		outPath, pub := newOutput(t, t.TempDir())
		result, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer: &writertestutil.FakeArticleWriter{Err: errInjectedWrite}, Publisher: pub,
		})
		if !errors.Is(err, errInjectedWrite) {
			t.Fatalf("Run error = %v, want the injected write error", err)
		}
		if len(result.Warnings) == 0 {
			t.Fatal("the prune warning was discarded when the pipeline failed")
		}
	})
}

func TestRunRemoveCacheFailureWarns(t *testing.T) {
	cacheDir := t.TempDir()
	seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
	outPath, pub := newOutput(t, t.TempDir())
	tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result, err := Run(ctx, Request{
		VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
		Writer:    &writertestutil.FakeArticleWriter{Result: validArticle()},
		Publisher: publishThenCancel{inner: pub, cancel: cancel},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("no warning for the cache-removal failure")
	}
	requireTripwireNotRun(t, marker)
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("the output was removed: %v", err)
	}
	// The cache removal failed, so the video's cache remains.
	pointer := filepath.Join(cacheDir, jobVideoID+".current")
	if _, err := os.Lstat(pointer); err != nil {
		t.Fatalf("the cache was removed despite the warning: %v", err)
	}
}

func TestRunRefresh(t *testing.T) {
	cacheDir := t.TempDir()
	seedCache(t, cacheDir, jobVideoID, `{"events":[{"tStartMs":0,"segs":[{"utf8":"cached text"}]}]}`, infoFor(jobVideoID))
	fresh := `{"events":[{"tStartMs":0,"segs":[{"utf8":"fresh text"}]}]}`
	fake := transcripttestutil.NewSuccess(t, t.TempDir(), jobVideoID, fresh, infoFor(jobVideoID))

	writer := &writertestutil.FakeArticleWriter{Result: validArticle()}
	outPath, pub := newOutput(t, t.TempDir())
	if _, err := Run(context.Background(), Request{
		VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: fake, Refresh: true,
		Writer: writer, Publisher: pub,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(writer.Calls) != 1 {
		t.Fatalf("Write called %d times, want 1", len(writer.Calls))
	}
	got := ""
	if len(writer.Calls[0].Transcript.Segments) > 0 {
		got = writer.Calls[0].Transcript.Segments[0].Text
	}
	if got != "fresh text" {
		t.Fatalf("the writer received %q, want the refreshed transcript", got)
	}
}

func TestRunCanceled(t *testing.T) {
	t.Run("during the fetch", func(t *testing.T) {
		cacheDir := t.TempDir()
		fake := transcripttestutil.NewStopping(t, t.TempDir())
		outPath, pub := newOutput(t, t.TempDir())
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		done := startRun(ctx, Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: fake.Script,
			Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
		})
		waitForFile(t, fake.Ready)
		cancel()

		outcome := awaitRun(t, done)
		if !errors.Is(outcome.err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", outcome.err)
		}
		requireNoFile(t, outPath)
		requireNoFile(t, filepath.Join(cacheDir, jobVideoID+".current"))
	})
	t.Run("before the lock", func(t *testing.T) {
		cacheDir := filepath.Join(t.TempDir(), "cache")
		outPath, pub := newOutput(t, t.TempDir())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := Run(ctx, Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir,
			Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
		requireNoFile(t, outPath)
		if got := transcripttestutil.ListDir(t, cacheDir); got != nil {
			t.Errorf("cache directory was created: %v", got)
		}
	})
	t.Run("during the write", func(t *testing.T) {
		cacheDir := t.TempDir()
		seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
		outPath, pub := newOutput(t, t.TempDir())
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())

		_, err := Run(ctx, Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer:    cancelWriter{inner: &writertestutil.FakeArticleWriter{Result: validArticle()}, cancel: cancel},
			Publisher: pub,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
		requireNoFile(t, outPath)
		requireCacheIntact(t, cacheDir)
	})
	t.Run("during the publish", func(t *testing.T) {
		cacheDir := t.TempDir()
		seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
		outPath, _ := newOutput(t, t.TempDir())
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())

		_, err := Run(ctx, Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer:    &writertestutil.FakeArticleWriter{Result: validArticle()},
			Publisher: cancelPublisher{cancel: cancel},
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
		requireNoFile(t, outPath)
		requireCacheIntact(t, cacheDir)
	})
}

// requireCacheIntact asserts the pointer of the job's video still exists.
func requireCacheIntact(t *testing.T, cacheDir string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(cacheDir, jobVideoID+".current")); err != nil {
		t.Fatalf("the cache was removed on failure: %v", err)
	}
}

func TestRunInvalidCache(t *testing.T) {
	cases := []struct {
		name      string
		subtitles string
		info      string
		want      error
	}{
		{"truncated subtitles", `{"events":[{"tStartMs":0,"segs":[{"utf8":"cut`, infoFor(jobVideoID), transcript.ErrParseSubtitles},
		{"truncated info", validSubtitles, `{"id":"` + jobVideoID + `","title":"cut`, transcript.ErrParseInfo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			seedCache(t, cacheDir, jobVideoID, tc.subtitles, tc.info)
			outPath, pub := newOutput(t, t.TempDir())

			_, err := Run(context.Background(), Request{
				VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir,
				Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Run error = %v, want %v", err, tc.want)
			}
			requireNoFile(t, outPath)

			// A refresh recovers: yt-dlp writes a valid cache.
			fresh := transcripttestutil.NewSuccess(t, t.TempDir(), jobVideoID, validSubtitles, infoFor(jobVideoID))
			outPath2, pub2 := newOutput(t, t.TempDir())
			if _, err := Run(context.Background(), Request{
				VideoURL: jobVideoURL, OutPath: outPath2, CacheDir: cacheDir, YtDlpPath: fresh,
				Refresh: true, Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub2,
			}); err != nil {
				t.Fatalf("Run with --refresh: %v", err)
			}
			if _, err := os.Stat(outPath2); err != nil {
				t.Fatalf("output not created after the refresh: %v", err)
			}
		})
	}
}

func TestRunOutputExists(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, dir, out string)
		check func(t *testing.T, dir, out string)
	}{
		{"regular file", func(t *testing.T, _, out string) {
			if err := os.WriteFile(out, []byte("precious"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, func(t *testing.T, _, out string) {
			if got, _ := os.ReadFile(out); string(got) != "precious" {
				t.Fatalf("existing file changed: %q", got)
			}
		}},
		{"directory", func(t *testing.T, _, out string) {
			if err := os.Mkdir(out, 0o700); err != nil {
				t.Fatal(err)
			}
		}, func(t *testing.T, _, out string) {
			if info, err := os.Lstat(out); err != nil || !info.IsDir() {
				t.Fatalf("directory replaced: %v %v", info, err)
			}
		}},
		{"symlink to a file", func(t *testing.T, dir, out string) {
			if err := os.WriteFile(filepath.Join(dir, "target"), []byte("target content"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("target", out); err != nil {
				t.Fatal(err)
			}
		}, func(t *testing.T, dir, _ string) {
			if got, _ := os.ReadFile(filepath.Join(dir, "target")); string(got) != "target content" {
				t.Fatalf("link target changed: %q", got)
			}
		}},
		{"symlink to a missing path", func(t *testing.T, _, out string) {
			if err := os.Symlink("missing", out); err != nil {
				t.Fatal(err)
			}
		}, func(t *testing.T, dir, _ string) {
			if _, err := os.Lstat(filepath.Join(dir, "missing")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("the dangling link's target was created: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		for _, cachePresent := range []bool{false, true} {
			name := tc.name
			if cachePresent {
				name += " (cache present)"
			} else {
				name += " (cache absent)"
			}
			t.Run(name, func(t *testing.T) {
				outDir := t.TempDir()
				outPath := filepath.Join(outDir, "article.md")
				tc.setup(t, outDir, outPath)
				cacheDir := filepath.Join(t.TempDir(), "cache")
				var before []string
				if cachePresent {
					// The refusal must leave the cache directory and its lock
					// file untouched.
					if err := os.MkdirAll(cacheDir, 0o700); err != nil {
						t.Fatal(err)
					}
					for path, content := range map[string]string{
						filepath.Join(cacheDir, ".yt2column.lock"):       "",
						filepath.Join(cacheDir, otherVideoID+".current"): "a",
					} {
						if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					before = transcripttestutil.ListDir(t, cacheDir)
				}
				writer := &writertestutil.FakeArticleWriter{Result: validArticle()}
				tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())

				_, err := Run(context.Background(), Request{
					VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
					Writer: writer, Publisher: &publishertestutil.FakePublisher{},
				})
				stageErr, ok := errors.AsType[*pipeline.StageError](err)
				if !ok || stageErr.Stage != pipeline.StagePublish {
					t.Fatalf("Run error = %v, want a publish StageError", err)
				}
				if !errors.Is(err, publisher.ErrOutputExists) {
					t.Fatalf("Run error = %v, want ErrOutputExists", err)
				}
				if len(writer.Calls) != 0 {
					t.Error("the writer was called")
				}
				requireTripwireNotRun(t, marker)
				if after := transcripttestutil.ListDir(t, cacheDir); !slices.Equal(after, before) {
					t.Errorf("cache directory changed: before %v, after %v", before, after)
				}
				tc.check(t, outDir, outPath)
			})
		}
	}
}

func TestRunOutputParentInvalid(t *testing.T) {
	cases := []struct {
		name    string
		outPath func(t *testing.T) string
	}{
		{"missing parent", func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "missing", "article.md")
		}},
		{"parent is a regular file", func(t *testing.T) string {
			file := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(file, "article.md")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outPath := tc.outPath(t)
			cacheDir := filepath.Join(t.TempDir(), "cache")
			writer := &writertestutil.FakeArticleWriter{Result: validArticle()}
			tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())

			_, err := Run(context.Background(), Request{
				VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
				Writer: writer, Publisher: &publishertestutil.FakePublisher{},
			})
			stageErr, ok := errors.AsType[*pipeline.StageError](err)
			if !ok || stageErr.Stage != pipeline.StagePublish {
				t.Fatalf("Run error = %v, want a publish StageError", err)
			}
			if errors.Is(err, publisher.ErrOutputExists) {
				t.Fatalf("Run error = %v, must not wrap ErrOutputExists", err)
			}
			if len(writer.Calls) != 0 {
				t.Error("the writer was called")
			}
			requireTripwireNotRun(t, marker)
			if got := transcripttestutil.ListDir(t, cacheDir); got != nil {
				t.Errorf("cache directory was created: %v", got)
			}
		})
	}
}

func TestRunLocked(t *testing.T) {
	cacheDir := t.TempDir()
	fake := transcripttestutil.NewStopping(t, t.TempDir())
	outPath1, pub1 := newOutput(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := startRun(ctx, Request{
		VideoURL: jobVideoURL, OutPath: outPath1, CacheDir: cacheDir, YtDlpPath: fake.Script,
		Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub1,
	})
	waitForFile(t, fake.Ready)
	before := transcripttestutil.ListDir(t, cacheDir)

	outPath2, pub2 := newOutput(t, t.TempDir())
	writer2 := &writertestutil.FakeArticleWriter{Result: validArticle()}
	tripwire2, marker2 := transcripttestutil.NewTripwire(t, t.TempDir())
	_, err := Run(context.Background(), Request{
		VideoURL: jobVideoURL, OutPath: outPath2, CacheDir: cacheDir, YtDlpPath: tripwire2,
		Writer: writer2, Publisher: pub2,
	})
	if !errors.Is(err, cachelock.ErrLocked) {
		t.Fatalf("second Run error = %v, want ErrLocked", err)
	}
	if len(writer2.Calls) != 0 {
		t.Error("the second run called the writer")
	}
	requireNoFile(t, outPath2)
	requireTripwireNotRun(t, marker2)
	if after := transcripttestutil.ListDir(t, cacheDir); !slices.Equal(after, before) {
		t.Errorf("the cache directory changed: before %v, after %v", before, after)
	}

	cancel()
	awaitRun(t, done)
}

func TestRunOtherCacheDir(t *testing.T) {
	cacheDirA := t.TempDir()
	fake := transcripttestutil.NewStopping(t, t.TempDir())
	outPathA, pubA := newOutput(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := startRun(ctx, Request{
		VideoURL: jobVideoURL, OutPath: outPathA, CacheDir: cacheDirA, YtDlpPath: fake.Script,
		Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pubA,
	})
	waitForFile(t, fake.Ready)

	cacheDirB := t.TempDir()
	seedCache(t, cacheDirB, jobVideoID, validSubtitles, infoFor(jobVideoID))
	outPathB, pubB := newOutput(t, t.TempDir())
	tripwireB, markerB := transcripttestutil.NewTripwire(t, t.TempDir())
	if _, err := Run(context.Background(), Request{
		VideoURL: jobVideoURL, OutPath: outPathB, CacheDir: cacheDirB, YtDlpPath: tripwireB,
		Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pubB,
	}); err != nil {
		t.Fatalf("Run on another cache directory: %v", err)
	}
	requireTripwireNotRun(t, markerB)

	cancel()
	awaitRun(t, done)
}

func TestRunLockFailure(t *testing.T) {
	t.Run("cache parent is a regular file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		requireLockFailure(t, filepath.Join(parent, "cache"))
	})
	t.Run("unwritable cache directory", func(t *testing.T) {
		requireNonRoot(t)
		cacheDir := t.TempDir()
		chmodForTest(t, cacheDir, 0o500)
		requireLockFailure(t, cacheDir)
	})
}

// requireLockFailure asserts a run fails to take the lock for a reason other
// than a concurrent run.
func requireLockFailure(t *testing.T, cacheDir string) {
	t.Helper()
	outPath, pub := newOutput(t, t.TempDir())
	writer := &writertestutil.FakeArticleWriter{Result: validArticle()}
	tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())

	_, err := Run(context.Background(), Request{
		VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
		Writer: writer, Publisher: pub,
	})
	if err == nil {
		t.Fatal("Run succeeded")
	}
	if errors.Is(err, cachelock.ErrLocked) {
		t.Fatalf("Run error = %v, must not wrap ErrLocked", err)
	}
	if len(writer.Calls) != 0 {
		t.Error("the writer was called")
	}
	requireTripwireNotRun(t, marker)
	requireNoFile(t, outPath)
}

func TestRunPassesLockToYtDlp(t *testing.T) {
	cacheDir := t.TempDir()
	fake := transcripttestutil.NewStopping(t, t.TempDir())
	outPath, pub := newOutput(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := startRun(ctx, Request{
		VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: fake.Script,
		Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
	})
	waitForFile(t, fake.Ready)

	data, err := os.ReadFile(fake.FD3)
	if err != nil {
		t.Fatalf("read the descriptor-3 record: %v", err)
	}
	if string(data) != "open" {
		t.Fatalf("yt-dlp saw descriptor 3 %q, want it open", data)
	}

	cancel()
	awaitRun(t, done)
}

func TestRunValidatesRequest(t *testing.T) {
	valid := func(t *testing.T) Request {
		t.Helper()
		outPath, pub := newOutput(t, t.TempDir())
		return Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: filepath.Join(t.TempDir(), "cache"),
			Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
		}
	}
	cases := []struct {
		name   string
		mutate func(t *testing.T, req *Request)
	}{
		{"empty cache dir", func(_ *testing.T, req *Request) { req.CacheDir = "" }},
		{"empty video URL", func(_ *testing.T, req *Request) { req.VideoURL = "" }},
		{"empty out path", func(_ *testing.T, req *Request) { req.OutPath = "" }},
		{"nil writer", func(_ *testing.T, req *Request) { req.Writer = nil }},
		{"typed-nil writer", func(_ *testing.T, req *Request) {
			var w *writertestutil.FakeArticleWriter
			req.Writer = w
		}},
		{"nil publisher", func(_ *testing.T, req *Request) { req.Publisher = nil }},
		{"typed-nil publisher", func(_ *testing.T, req *Request) {
			var p *publishertestutil.FakePublisher
			req.Publisher = p
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid(t)
			tc.mutate(t, &req)
			cacheDir := req.CacheDir
			if _, err := Run(context.Background(), req); !errors.Is(err, errInvalidRequest) {
				t.Fatalf("Run error = %v, want errInvalidRequest", err)
			}
			if cacheDir != "" {
				if got := transcripttestutil.ListDir(t, cacheDir); got != nil {
					t.Errorf("cache directory was created: %v", got)
				}
			}
		})
	}
}

func TestRunReleasesLock(t *testing.T) {
	t.Run("after success", func(t *testing.T) {
		cacheDir := t.TempDir()
		seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
		outPath, pub := newOutput(t, t.TempDir())
		tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
		if _, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer: &writertestutil.FakeArticleWriter{Result: validArticle()}, Publisher: pub,
		}); err != nil {
			t.Fatalf("Run: %v", err)
		}
		requireLockAcquirable(t, cacheDir)
	})
	t.Run("after failure", func(t *testing.T) {
		cacheDir := t.TempDir()
		seedCache(t, cacheDir, jobVideoID, validSubtitles, infoFor(jobVideoID))
		outPath, pub := newOutput(t, t.TempDir())
		tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
		if _, err := Run(context.Background(), Request{
			VideoURL: jobVideoURL, OutPath: outPath, CacheDir: cacheDir, YtDlpPath: tripwire,
			Writer: &writertestutil.FakeArticleWriter{Err: errInjectedWrite}, Publisher: pub,
		}); err == nil {
			t.Fatal("Run succeeded")
		}
		requireLockAcquirable(t, cacheDir)
	})
}

// requireLockAcquirable asserts the cache directory lock is free.
func requireLockAcquirable(t *testing.T, cacheDir string) {
	t.Helper()
	lock, err := cachelock.Acquire(cacheDir)
	if err != nil {
		t.Fatalf("Acquire after the run: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
