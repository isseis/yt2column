//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	transcripttestutil "github.com/isseis/yt2column/internal/transcript/testutil"
)

const (
	integrationVideoID  = "2tcCWM-sRBw"
	integrationVideoURL = "https://www.youtube.com/watch?v=" + integrationVideoID

	// integrationTestdata is the repository's testdata directory, which holds
	// the video's subtitles and info.json.
	integrationTestdata = "../../testdata"
)

// TestIntegrationCLI runs the CLI, assembled exactly as main assembles it,
// from a seeded transcript cache to the --out file with the real DeepSeek API.
// yt-dlp is a tripwire and is never started. It is excluded from `make test`
// by its build tag and is run by `make test-integration-cli`, which sets the
// opt-in variable and the model name. It calls run once and makes one
// Generate call. Nothing the run wrote is printed until it is shown to hold
// neither the API key nor its last eight characters, and the failure messages
// name the place only.
func TestIntegrationCLI(t *testing.T) {
	gateCLIIntegration(t, os.Getenv, func(settings deepseektestutil.IntegrationSettings) {
		runIntegrationCLI(t, settings)
	})
}

// runIntegrationCLI is the body of TestIntegrationCLI once the environment
// says it runs.
func runIntegrationCLI(t *testing.T, settings deepseektestutil.IntegrationSettings) {
	key, err := settings.APIKey.Reveal()
	if err != nil {
		t.Fatal("the test API key cannot be revealed")
	}

	base := t.TempDir()
	cacheDir := filepath.Join(base, "cache")
	if err := os.Mkdir(cacheDir, 0o700); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	subtitles, err := os.ReadFile(filepath.Join(integrationTestdata, integrationVideoID+".ja.json3"))
	if err != nil {
		t.Fatalf("read subtitles: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(integrationTestdata, integrationVideoID+".info.json"))
	if err != nil {
		t.Fatalf("read info.json: %v", err)
	}
	if err := transcript.SeedCacheForTest(cacheDir, integrationVideoID, subtitles, info); err != nil {
		t.Fatalf("SeedCacheForTest: %v", err)
	}
	if len(videoCacheEntries(t, cacheDir)) == 0 {
		t.Fatal("the seeded cache has no entry for the video, so its removal cannot be checked")
	}
	binDir := filepath.Join(base, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("create bin dir: %v", err)
	}
	tripwire, tripwireMarker := transcripttestutil.NewTripwire(t, binDir)
	outPath := filepath.Join(base, "article.md")

	// The environment run sees is built here; the process environment is not
	// changed. The test API key is given to run as DEEPSEEK_API_KEY.
	env := map[string]string{
		"YT2COLUMN_MODEL":      settings.Model,
		"DEEPSEEK_API_KEY":     key,
		"YT2COLUMN_CACHE_DIR":  cacheDir,
		"YT2COLUMN_YTDLP_PATH": tripwire,
	}
	// The real client is wrapped from the outside only, so the assembly stays
	// productionDeps() and the Generate calls can be counted.
	counter := &generateCounter{}
	d := productionDeps()
	newLLMClient := d.newLLMClient
	d.newLLMClient = func(cfg config.Config) (llm.LLMClient, error) {
		client, err := newLLMClient(cfg)
		return counter.wrap(client), err
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--out", outPath, integrationVideoURL}, lookupFrom(env), &stdout, &stderr, d)

	article := ""
	if data, err := os.ReadFile(outPath); err == nil { //nolint:gosec // a path inside the test's temporary directory
		article = string(data)
	}
	keyRunes := []rune(key)
	forbidden := []string{key, string(keyRunes[max(0, len(keyRunes)-8):]), key[max(0, len(key)-8):]}
	for _, place := range []struct{ name, text string }{
		{"standard output", stdout.String()},
		{"standard error", stderr.String()},
		{"the --out file", article},
	} {
		for _, s := range forbidden {
			if strings.Contains(place.text, s) {
				t.Fatalf("%s holds the test API key or its last eight characters", place.name)
			}
		}
	}

	if code != 0 {
		t.Errorf("run exit code = %d, want 0", code)
	}
	if _, err := os.Lstat(tripwireMarker); err == nil {
		t.Error("the yt-dlp tripwire was started")
	}
	if got := counter.count(); got != 1 {
		t.Errorf("Generate was called %d times, want 1", got)
	}
	checkIntegrationArticle(t, article)
	for _, name := range videoCacheEntries(t, cacheDir) {
		t.Errorf("the cache directory still holds the entry %s; without --keep-cache it is removed", name)
	}
}

// videoCacheEntries returns the cache directory's entries for the video.
func videoCacheEntries(t *testing.T, cacheDir string) []string {
	t.Helper()
	var entries []string
	for _, name := range transcripttestutil.ListDir(t, cacheDir) {
		if strings.HasPrefix(name, integrationVideoID+".") {
			entries = append(entries, name)
		}
	}
	return entries
}

// checkIntegrationArticle checks the --out file: a non-empty title heading, a
// non-empty model name, a generated body, and the source link. The content itself is
// never printed.
func checkIntegrationArticle(t *testing.T, article string) {
	t.Helper()
	if article == "" {
		t.Error("the --out file is missing or empty")
		return
	}
	header, rest, _ := strings.Cut(article, "\n\n")
	if title, ok := strings.CutPrefix(header, "# "); !ok || strings.TrimSpace(title) == "" {
		t.Error("the --out file does not start with a non-empty title heading")
	}
	metadata, body, _ := strings.Cut(rest, "\n\n")
	modelLine, _, _ := strings.Cut(metadata, "\n")
	if model, ok := strings.CutPrefix(modelLine, "- Model: "); !ok || strings.TrimSpace(model) == "" {
		t.Error("the --out file has no non-empty model name")
	}
	// The article ends with the source line, an autolink to the video, which
	// the writer appends itself; the generated text is what precedes it.
	// Anchoring at the end keeps a link the generated text repeats from
	// standing in for the writer's own.
	beforeLink, found := strings.CutSuffix(body, "<"+integrationVideoURL+">\n")
	if !found {
		t.Error("the --out file does not end with the source link")
		return
	}
	sourceLine := strings.LastIndex(beforeLink, "\n") + 1
	if strings.TrimSpace(beforeLink[:sourceLine]) == "" {
		t.Error("the --out file has no generated body before the source link")
	}
}
