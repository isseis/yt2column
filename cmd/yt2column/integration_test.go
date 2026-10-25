//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
)

const integrationVideoURL = "https://www.youtube.com/watch?v=" + integrationVideoID

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
	code := run(context.Background(), append(slices.Clone(noStepsArgs), "--out", outPath, integrationVideoURL), lookupFrom(r.env), &stdout, &stderr, r.deps)

	article := ""
	if data, err := os.ReadFile(outPath); err == nil { //nolint:gosec // a path inside the test's temporary directory
		article = string(data)
	}
	requireNoSecrets(t, secretTails(r.apiKey), []outputPlace{
		{"standard output", stdout.String()},
		{"standard error", stderr.String()},
		{"the --out file", article},
	})

	// The lines are logged only after stderr is shown to hold no secret, and
	// before checkRun, so a failed run still leaves them in the log.
	tokens, costs, totals := logUsageLines(t, stderr.String())
	r.checkRun(t, code)
	if code == 0 && (tokens < 1 || costs < 1 || totals != 2) {
		t.Errorf("the run wrote %d token, %d cost, and %d total lines; want at least one of each and two totals", tokens, costs, totals)
	}
	checkIntegrationArticle(t, article)
}

// logUsageLines logs the token-usage, cost, and total lines of stderr, and
// returns how many of each it saw. The caller must have shown stderr holds no
// secret first.
func logUsageLines(t *testing.T, stderr string) (tokens, costs, totals int) {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimSuffix(stderr, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, tokensPrefix):
			tokens++
		case strings.HasPrefix(line, costPrefix):
			costs++
		case strings.HasPrefix(line, costTotalPrefix):
			totals++
		default:
			continue
		}
		t.Log(line)
	}
	return tokens, costs, totals
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

// secretTails returns s and its last eight characters, taken by character
// and by byte.
func secretTails(s string) []string {
	runes := []rune(s)
	return []string{s, string(runes[max(0, len(runes)-8):]), s[max(0, len(s)-8):]}
}

// requireNoSecrets fails t unless no place holds any of the forbidden
// strings. It runs before anything the run wrote is printed, and its failure
// names the place only.
func requireNoSecrets(t *testing.T, forbidden []string, places []outputPlace) {
	t.Helper()
	for _, place := range places {
		for _, s := range forbidden {
			if strings.Contains(place.text, s) {
				t.Fatalf("%s holds a test secret or its last eight characters", place.name)
			}
		}
	}
}

// checkIntegrationArticle checks the --out file: YAML front matter with a
// non-empty model name, a non-empty title heading, a generated body, and the
// source link. The content itself is never printed.
func checkIntegrationArticle(t *testing.T, article string) {
	t.Helper()
	if article == "" {
		t.Error("the --out file is missing or empty")
		return
	}
	afterOpen, opened := strings.CutPrefix(article, "---\n")
	frontMatter, rest, found := strings.Cut(afterOpen, "\n---\n\n")
	if !opened || !found {
		t.Error("the --out file does not start with YAML front matter")
		return
	}
	var model string
	hasChannel := false
	for line := range strings.SplitSeq(frontMatter, "\n") {
		if strings.HasPrefix(line, "channel: ") {
			hasChannel = true
		}
		if value, ok := strings.CutPrefix(line, "model: "); ok {
			model, _ = strconv.Unquote(value)
		}
	}
	if strings.TrimSpace(model) == "" {
		t.Error("the --out file has no non-empty model name")
	}
	header, body, _ := strings.Cut(rest, "\n\n")
	if title, ok := strings.CutPrefix(header, "# "); !ok || strings.TrimSpace(title) == "" {
		t.Error("the --out file has no non-empty title heading after the front matter")
	}
	if hasChannel {
		// The channel name line is not generated text.
		_, body, _ = strings.Cut(body, "\n\n")
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
