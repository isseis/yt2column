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
	r := newIntegrationRun(t, settings)
	outPath := filepath.Join(r.base, "article.md")

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--out", outPath, integrationVideoURL}, lookupFrom(r.env), &stdout, &stderr, r.deps)

	article := ""
	if data, err := os.ReadFile(outPath); err == nil { //nolint:gosec // a path inside the test's temporary directory
		article = string(data)
	}
	requireNoSecrets(t, []string{r.apiKey}, []outputPlace{
		{"standard output", stdout.String()},
		{"standard error", stderr.String()},
		{"the --out file", article},
	})

	r.checkRun(t, code)
	checkIntegrationArticle(t, article)
}

// integrationRun is one CLI run assembled for an integration test: a seeded
// transcript cache, a yt-dlp tripwire, the environment run sees, and
// productionDeps() with the Generate calls counted.
type integrationRun struct {
	base           string
	cacheDir       string
	tripwireMarker string
	apiKey         string
	env            map[string]string
	deps           deps
	counter        *generateCounter
}

// newIntegrationRun seeds the cache with the video's testdata, installs the
// tripwire, and builds the environment and the deps. The environment is built
// here and the process environment is not changed; the test API key is given
// to run as DEEPSEEK_API_KEY. A caller adds the variables its run needs.
func newIntegrationRun(t *testing.T, settings deepseektestutil.IntegrationSettings) integrationRun {
	t.Helper()
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
	return integrationRun{
		base:           base,
		cacheDir:       cacheDir,
		tripwireMarker: tripwireMarker,
		apiKey:         key,
		env:            env,
		deps:           d,
		counter:        counter,
	}
}

// checkRun checks what every successful integration run leaves: exit code 0,
// the tripwire never started, one Generate call, and no cache entry for the
// video, since --keep-cache is not given.
func (r integrationRun) checkRun(t *testing.T, code int) {
	t.Helper()
	if code != 0 {
		t.Errorf("run exit code = %d, want 0", code)
	}
	if _, err := os.Lstat(r.tripwireMarker); err == nil {
		t.Error("the yt-dlp tripwire was started")
	}
	if got := r.counter.count(); got != 1 {
		t.Errorf("Generate was called %d times, want 1", got)
	}
	for _, name := range videoCacheEntries(t, r.cacheDir) {
		t.Errorf("the cache directory still holds the entry %s; without --keep-cache it is removed", name)
	}
}

// outputPlace is a named piece of output an integration run produced.
type outputPlace struct {
	name, text string
}

// requireNoSecrets fails t unless no place holds any of secrets or the last
// eight characters of one, taken by character and by byte. It runs before
// anything the run wrote is printed, and its failure names the place only.
func requireNoSecrets(t *testing.T, secrets []string, places []outputPlace) {
	t.Helper()
	var forbidden []string
	for _, s := range secrets {
		runes := []rune(s)
		forbidden = append(forbidden, s, string(runes[max(0, len(runes)-8):]), s[max(0, len(s)-8):])
	}
	for _, place := range places {
		for _, s := range forbidden {
			if strings.Contains(place.text, s) {
				t.Fatalf("%s holds a test secret or its last eight characters", place.name)
			}
		}
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
