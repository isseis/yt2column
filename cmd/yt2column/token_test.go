//go:build test

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/deepseek"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/writer"
)

// Distinct token usages and model names for the three stages, so a test can
// tell where each value came from.
var (
	testGenTokens    = llm.KnownUsage(llm.TokenCounts{Input: 11, CacheRead: 12, CacheWrite: 13, Output: 14})
	testShortTokens  = llm.KnownUsage(llm.TokenCounts{Input: 21, CacheRead: 22, CacheWrite: 23, Output: 24})
	testReviewTokens = llm.KnownUsage(llm.TokenCounts{Input: 31, CacheRead: 32, CacheWrite: 33, Output: 34})
)

// usageResponse builds a response with text, model, and usage.
func usageResponse(text, model string, usage llm.Usage) llm.GenerateResponse {
	return llm.GenerateResponse{Text: text, Model: model, Usage: usage}
}

// usageLLM returns a client that answers every call with resp.
func usageLLM(resp llm.GenerateResponse) llm.LLMClient {
	return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return resp, nil
	})
}

// usageSequence returns a client that answers each call with the next response.
func usageSequence(responses ...llm.GenerateResponse) llm.LLMClient {
	index := 0
	return funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		resp := responses[index]
		index++
		return resp, nil
	})
}

// tokenLines returns the token-usage lines of stderr, in order.
func tokenLines(stderr string) []string {
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSuffix(stderr, "\n"), "\n") {
		if strings.HasPrefix(line, tokensPrefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

// knownLine renders the expected token-usage line of a known usage.
func knownLine(stage, origin, model string, usage llm.Usage) string {
	counts, _ := usage.Counts()
	return fmt.Sprintf("%s%s %s known %s%d %s%d %s%d %s%d %s%s",
		tokensPrefix, stage, origin,
		tokensInput, counts.Input, tokensCacheRead, counts.CacheRead,
		tokensCacheWrite, counts.CacheWrite, tokensOutput, counts.Output,
		tokensModel, model)
}

// requireTokenLines asserts the token lines of e.stderr equal want, in order.
func requireTokenLines(t *testing.T, e *runEnv, want []string) {
	t.Helper()
	if got := tokenLines(e.stderr.String()); !slices.Equal(got, want) {
		t.Errorf("token lines = %q, want %q\nstderr:\n%s", got, want, e.stderr.String())
	}
}

func TestRunTokenLinesAllStages(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.client = usageSequence(
		usageResponse(longGeneratedText(), "gen-model", testGenTokens),
		usageResponse(shortGeneratedText(), "short-model", testShortTokens),
	)
	e.useReviewClient(usageLLM(usageResponse(reviewJSON("A short body.", "A shorter body."), "review-model", testReviewTokens)))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	want := []string{
		knownLine("generation", tokensRun, "gen-model", testGenTokens),
		knownLine("shorten", tokensRun, "short-model", testShortTokens),
		knownLine("review", tokensRun, "review-model", testReviewTokens),
	}
	requireTokenLines(t, e, want)
	// The token lines come after the existing summary lines.
	if last, first := strings.LastIndex(e.stderr.String(), "the review step returned"), strings.Index(e.stderr.String(), tokensPrefix); first < last {
		t.Errorf("token lines appear before the summary:\n%s", e.stderr.String())
	}
	requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
}

func TestRunTokenLinesSkippedStages(t *testing.T) {
	t.Run("shorten not called", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.client = usageLLM(usageResponse(shortGeneratedText(), "gen-model", testGenTokens))
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, "review-model", testReviewTokens)))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireTokenLines(t, e, []string{
			knownLine("generation", tokensRun, "gen-model", testGenTokens),
			knownLine("review", tokensRun, "review-model", testReviewTokens),
		})
	})

	t.Run("review disabled", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-review", "--out", e.outPath, runVideoURL}
		e.client = usageSequence(
			usageResponse(longGeneratedText(), "gen-model", testGenTokens),
			usageResponse(shortGeneratedText(), "short-model", testShortTokens),
		)

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireTokenLines(t, e, []string{
			knownLine("generation", tokensRun, "gen-model", testGenTokens),
			knownLine("shorten", tokensRun, "short-model", testShortTokens),
		})
	})
}

func TestRunTokenLinesUnknown(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--no-review", "--out", e.outPath, runVideoURL}
	unknown := llm.UnknownUsage(llm.RuleNotNonNegativeInteger, "completion_tokens")
	e.client = usageLLM(usageResponse(shortGeneratedText(), "gen-model", unknown))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d (an unknown usage still publishes)\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	requirePublished(t, e.outPath)
	requireTokenLines(t, e, []string{
		tokensPrefix + "generation run unknown " + tokensReason + "usage.completion_tokens:not-a-non-negative-integer " + tokensModel + "gen-model",
	})
}

func TestRunTokenLinesReviewFailure(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.client = usageSequence(
		usageResponse(longGeneratedText(), "gen-model", testGenTokens),
		usageResponse(shortGeneratedText(), "short-model", testShortTokens),
	)
	e.useReviewClient(funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return llm.GenerateResponse{}, errStubLLM
	}))

	if code := e.run(); code == exitOK {
		t.Fatalf("exit code = %d, want a failure\nstderr:\n%s", code, e.stderr.String())
	}
	requireTokenLines(t, e, []string{
		knownLine("generation", tokensRun, "gen-model", testGenTokens),
		knownLine("shorten", tokensRun, "short-model", testShortTokens),
		tokensPrefix + "review run unknown " + tokensReason + "not-read " + tokensNoModel,
	})
	requireNoFile(t, e.outPath)
	if first, failure := strings.Index(e.stderr.String(), tokensPrefix), strings.Index(e.stderr.String(), "the write stage failed"); failure < first {
		t.Errorf("the failure line appears before the token lines:\n%s", e.stderr.String())
	}
}

func TestRunTokenLinesRejectedResponse(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return llm.GenerateResponse{Model: "gen-model", Usage: testGenTokens}, fmt.Errorf("stub: %w", llm.ErrTruncated)
	})

	if code := e.run(); code == exitOK {
		t.Fatalf("exit code = %d, want a failure (the generation was truncated)", code)
	}
	requireTokenLines(t, e, []string{knownLine("generation", tokensRun, "gen-model", testGenTokens)})
}

func TestRunTokenLinesNoUsage(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
	e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
		return llm.GenerateResponse{}, context.DeadlineExceeded
	})

	if code := e.run(); code == exitOK {
		t.Fatalf("exit code = %d, want a failure (the call had no usage)", code)
	}
	requireTokenLines(t, e, []string{
		tokensPrefix + "generation run unknown " + tokensReason + "not-read " + tokensNoModel,
	})
}

func TestRunTokenLinesBeforeLLM(t *testing.T) {
	t.Run("usage error", func(t *testing.T) {
		e := newRunEnv(t)
		// No destination: a usage error before any side effect.
		e.args = []string{runVideoURL}
		if code := e.run(); code != exitUsage {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
		}
		requireUntouched(t, e)
		requireNoTokenLines(t, e)
	})

	t.Run("configuration error", func(t *testing.T) {
		e := newRunEnv(t)
		delete(e.env, "DEEPSEEK_API_KEY")
		if code := e.run(); code != exitUsage {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
		}
		requireUntouched(t, e)
		requireNoTokenLines(t, e)
	})

	t.Run("--after rejection", func(t *testing.T) {
		e := newRunEnv(t)
		e.args = []string{"--after", "generation", "--out", e.outPath, runVideoURL}
		if code := e.run(); code != exitUsage {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, e.stderr.String())
		}
		requireUntouched(t, e)
		requireNoTokenLines(t, e)
	})

	t.Run("transcript failure", func(t *testing.T) {
		e := newRunEnv(t)
		removeVideoCache(t, e.cacheDir)
		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the transcript cannot be fetched)", code)
		}
		if got := e.counter.count(); got != 0 {
			t.Errorf("the LLM was called %d times, want 0", got)
		}
		requireNoTokenLines(t, e)
	})
}

// requireNoTokenLines asserts no token-usage line was written.
func requireNoTokenLines(t *testing.T, e *runEnv) {
	t.Helper()
	if got := tokenLines(e.stderr.String()); len(got) != 0 {
		t.Errorf("token lines = %q, want none", got)
	}
}

func TestRunTokenLinesAfterAllCalls(t *testing.T) {
	t.Run("publish failure", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		// The webhook rejects the post, so the publish fails after every stage
		// ran. (An existing --out file would fail the job's precheck before any
		// call.)
		e.useWebhook(t, failMessage(1, http.StatusServiceUnavailable, "boom"))
		e.client = usageSequence(
			usageResponse(longGeneratedText(), "gen-model", testGenTokens),
			usageResponse(shortGeneratedText(), "short-model", testShortTokens),
		)
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, "review-model", testReviewTokens)))

		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the post must fail)", code)
		}
		requireTokenLines(t, e, []string{
			knownLine("generation", tokensRun, "gen-model", testGenTokens),
			knownLine("shorten", tokensRun, "short-model", testShortTokens),
			knownLine("review", tokensRun, "review-model", testReviewTokens),
		})
	})

	t.Run("length limit after review", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		tooLong := "A short body." + strings.Repeat("a", 10000)
		e.client = usageLLM(usageResponse(shortGeneratedText(), "gen-model", testGenTokens))
		e.useReviewClient(usageLLM(usageResponse(reviewJSON("A short body.", tooLong), "review-model", testReviewTokens)))

		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the final length check must fail)", code)
		}
		requireTokenLines(t, e, []string{
			knownLine("generation", tokensRun, "gen-model", testGenTokens),
			knownLine("review", tokensRun, "review-model", testReviewTokens),
		})
	})
}

func TestRunTokenLineModelName(t *testing.T) {
	const model = "m \"quoted\" \\ \\ \u202e end"
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
	e.client = usageLLM(usageResponse(shortGeneratedText(), model, testGenTokens))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	lines := tokenLines(e.stderr.String())
	if len(lines) != 1 {
		t.Fatalf("token lines = %q, want one", lines)
	}
	index := strings.Index(lines[0], tokensModel)
	if index < 0 {
		t.Fatalf("token line %q has no %s field", lines[0], tokensModel)
	}
	escaped := lines[0][index+len(tokensModel):]
	got, err := strconv.Unquote(`"` + escaped + `"`)
	if err != nil {
		t.Fatalf("the model field %q is not a recoverable quoted string: %v", escaped, err)
	}
	if got != model {
		t.Errorf("recovered model = %q, want %q", got, model)
	}
}

// requirePublished asserts the --out file exists and holds text.
func requirePublished(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("--out file %s: %v", path, err)
	}
}

func TestRunTokenLinesResume(t *testing.T) {
	// Save a run whose generation and shorten usages are known and whose
	// review usage is not-read (reviewLLM returns none).
	dir := saveArtifactsOnce(t, func(e *runEnv) {
		e.client = usageSequence(
			usageResponse(longGeneratedText(), "gen-model", testGenTokens),
			usageResponse(shortGeneratedText(), "short-model", testShortTokens),
		)
	})

	t.Run("after review reuses every stage", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		if got := e.counter.count(); got != 0 {
			t.Errorf("the LLM was called %d times, want 0 (every stage was reused)", got)
		}
		requireTokenLines(t, e, []string{
			knownLine("generation", tokensReused, "gen-model", testGenTokens),
			knownLine("shorten", tokensReused, "short-model", testShortTokens),
			tokensPrefix + "review reused unknown " + tokensReason + "recorded-unknown " + tokensModel + "review-model",
		})
	})

	t.Run("after shorten reuses generation and shorten and runs review", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "shorten", "--out", e.outPath, runVideoURL}
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, "review-model", testReviewTokens)))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireTokenLines(t, e, []string{
			knownLine("generation", tokensReused, "gen-model", testGenTokens),
			knownLine("shorten", tokensReused, "short-model", testShortTokens),
			knownLine("review", tokensRun, "review-model", testReviewTokens),
		})
	})

	t.Run("after generation reuses generation and runs shorten and review", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "generation", "--out", e.outPath, runVideoURL}
		e.client = usageSequence(usageResponse(shortGeneratedText(), "short-run-model", testShortTokens))
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, "review-model", testReviewTokens)))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireTokenLines(t, e, []string{
			knownLine("generation", tokensReused, "gen-model", testGenTokens),
			knownLine("shorten", tokensRun, "short-run-model", testShortTokens),
			knownLine("review", tokensRun, "review-model", testReviewTokens),
		})
	})
}

// loopbackDeepseekBody returns the recorded DeepSeek stop response with its
// content replaced by an article, so the real adapter accepts it and reports
// the recorded usage.
func loopbackDeepseekBody(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/deepseek_chat_completion_stop.json") //nolint:gosec // a committed testdata fixture
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode the fixture: %v", err)
	}
	choices, ok := document["choices"].([]any)
	if !ok || len(choices) == 0 {
		t.Fatalf("choices = %#v, want a non-empty array", document["choices"])
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		t.Fatalf("choices[0] = %#v, want an object", choices[0])
	}
	message, ok := choice["message"].(map[string]any)
	if !ok {
		t.Fatalf("message = %#v, want an object", choice["message"])
	}
	message["content"] = "# Loopback title\n\nLoopback body.\n"
	out, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode the fixture: %v", err)
	}
	return out
}

func TestRunTokenLinesLoopbackAdapter(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
	body := loopbackDeepseekBody(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	apiKey, err := secret.New(testAPIKey)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	e.d.newLLMClient = func(config.LLMConfig) (llm.LLMClient, error) {
		return deepseek.NewForLoopbackTest(t, deepseek.Options{APIKey: apiKey, Model: "deepseek-flash", Timeout: 5 * time.Second}, server.URL), nil
	}

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	requirePublished(t, e.outPath)
	// The recorded usage: prompt_cache_miss_tokens 46, prompt_cache_hit_tokens
	// 0, completion_tokens 85.
	want := llm.KnownUsage(llm.TokenCounts{Input: 46, Output: 85})
	requireTokenLines(t, e, []string{knownLine("generation", tokensRun, "deepseek-flash", want)})
}

func TestRunTokenLinesReviewLogFailure(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.useWebhook(t, nil)
	logPath := filepath.Join(e.base, "review.md")
	e.args = []string{"--review-log", logPath, "--slack", runVideoURL}
	e.client = usageLLM(usageResponse(shortGeneratedText(), "gen-model", testGenTokens))
	e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, "review-model", testReviewTokens)))
	inner := e.d.newSlackPublisher
	e.d.newSlackPublisher = func(url secret.Secret) (publisher.Publisher, error) {
		pub, err := inner(url)
		if err != nil {
			return nil, err
		}
		return publisherFunc(func(ctx context.Context, a writer.Article) error {
			if err := pub.Publish(ctx, a); err != nil {
				return err
			}
			// Write the log path first, so the review log's own create finds it
			// and fails after the article is published.
			return os.WriteFile(logPath, []byte("raced"), 0o600)
		}), nil
	}

	if code := e.run(); code != exitFailure {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, e.stderr.String())
	}
	if got := strings.Count(e.stderr.String(), tokensPrefix); got != 2 {
		t.Errorf("token lines appear %d times, want 2 (once each for generation and review)\nstderr:\n%s", got, e.stderr.String())
	}
	requireTokenLines(t, e, []string{
		knownLine("generation", tokensRun, "gen-model", testGenTokens),
		knownLine("review", tokensRun, "review-model", testReviewTokens),
	})
}

// TestRunTokenLinesGoldenFormat pins the literal line spellings, so a change to
// the prefix, a count label, reason=, model=, or nomodel is caught even though
// the other tests build the expected line from the same constants.
func TestRunTokenLinesGoldenFormat(t *testing.T) {
	t.Run("known", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		e.client = usageLLM(usageResponse(shortGeneratedText(), "gen-model", testGenTokens))
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireTokenLines(t, e, []string{
			"yt2column: tokens: generation run known input=11 cache_read=12 cache_write=13 output=14 model=gen-model",
		})
	})

	t.Run("unknown", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		e.client = usageLLM(usageResponse(shortGeneratedText(), "gen-model", llm.UnknownUsage(llm.RuleNotNonNegativeInteger, "completion_tokens")))
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireTokenLines(t, e, []string{
			"yt2column: tokens: generation run unknown reason=usage.completion_tokens:not-a-non-negative-integer model=gen-model",
		})
	})

	t.Run("no model", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			return llm.GenerateResponse{}, context.DeadlineExceeded
		})
		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the call reported no usage)", code)
		}
		requireTokenLines(t, e, []string{
			"yt2column: tokens: generation run unknown reason=not-read nomodel",
		})
	})
}
