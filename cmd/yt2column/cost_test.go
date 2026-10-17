//go:build test

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/pricing"
	"github.com/isseis/yt2column/internal/publisher"
	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/writer"
)

// Distinct models and token usages for the three stages, so a test can tell
// which stage produced a value and hand-check its cost against the rate table.
// The generation and shorten models are flat and the review model is
// prompt-tier at or below its threshold.
var (
	costGenTokens    = llm.KnownUsage(llm.TokenCounts{Input: 100, Output: 10})
	costShortTokens  = llm.KnownUsage(llm.TokenCounts{Input: 100, Output: 10})
	costReviewTokens = llm.KnownUsage(llm.TokenCounts{Input: 1000, Output: 200})
)

const (
	costGenModel    = "claude-opus-5-5"
	costShortModel  = "claude-sonnet-5-5"
	costReviewModel = "claude-haiku-5-5"
)

// The fixed call times, one per Generate call. costPeakTime is Monday 01:00
// UTC, inside deepseek-flash's peak window; costOffPeakTime is Monday 04:30
// UTC, outside both of its ranges.
var (
	costGenTime     = time.Date(2026, time.October, 16, 1, 0, 0, 0, time.UTC)
	costShortTime   = time.Date(2026, time.October, 16, 4, 2, 41, 0, time.UTC)
	costReviewTime  = time.Date(2026, time.October, 16, 4, 3, 15, 0, time.UTC)
	costPeakTime    = time.Date(2026, time.October, 19, 1, 0, 0, 0, time.UTC)
	costOffPeakTime = time.Date(2026, time.October, 19, 4, 30, 0, 0, time.UTC)
)

// Hand-checked cost lines, fixed by the rate table and the times above. The
// amount is tokens * rate where rate is microUSD per million tokens, which is
// picoUSD per token: generation 100*4000000 + 10*20000000 = 600,000,000
// picoUSD = 0.000600 USD; shorten 100*2000000 + 10*10000000 = 0.000300;
// review (at or below the 100,000-token threshold) 1000*100000 + 200*500000 =
// 0.000200. The total is 0.001100.
const (
	costGenLine    = "yt2column: cost: generation run known usd=0.000600 flat at=2026-10-16T01:00:00Z"
	costShortLine  = "yt2column: cost: shorten run known usd=0.000300 flat at=2026-10-16T04:02:41Z"
	costReviewLine = "yt2column: cost: review run known usd=0.000200 short-prompt at=2026-10-16T04:03:15Z"
	costRunTotal   = "yt2column: cost-total: run known usd=0.001100"
	costArtTotal   = "yt2column: cost-total: article known usd=0.001100"
)

// useFixedClock wraps the run's writer constructor so the ArticleWriter reads
// the given times in order; the last repeats once the list runs out. It makes
// each stage's call time and the cost line's at= deterministic.
func useFixedClock(e *runEnv, times ...time.Time) {
	index := 0
	inner := e.d.newWriter
	e.d.newWriter = func(client llm.LLMClient, opts writer.Options) (writer.ArticleWriter, error) {
		opts.Clock = func() time.Time {
			now := times[min(index, len(times)-1)]
			index++
			return now
		}
		return inner(client, opts)
	}
}

// costStageLines returns the per-stage cost lines of stderr, in order.
func costStageLines(stderr string) []string {
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSuffix(stderr, "\n"), "\n") {
		if strings.HasPrefix(line, costPrefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

// costTotalLines returns the total lines of stderr, in order.
func costTotalLines(stderr string) []string {
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSuffix(stderr, "\n"), "\n") {
		if strings.HasPrefix(line, costTotalPrefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

// costLines returns every cost line of stderr, stage and total, in the order
// they were written, so a test can tell that the totals follow the stages.
func costLines(stderr string) []string {
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSuffix(stderr, "\n"), "\n") {
		if strings.HasPrefix(line, costPrefix) || strings.HasPrefix(line, costTotalPrefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

// requireCostLines asserts the cost lines of e.stderr equal want, in order.
func requireCostLines(t *testing.T, e *runEnv, want []string) {
	t.Helper()
	if got := costLines(e.stderr.String()); !slices.Equal(got, want) {
		t.Errorf("cost lines = %q, want %q\nstderr:\n%s", got, want, e.stderr.String())
	}
}

// requireNoCostLines asserts no cost line was written.
func requireNoCostLines(t *testing.T, e *runEnv) {
	t.Helper()
	if got := costLines(e.stderr.String()); len(got) != 0 {
		t.Errorf("cost lines = %q, want none", got)
	}
}

// useAllStagesCost sets the clients for a run whose three stages each report a
// distinct priced model and known usage.
func useAllStagesCost(e *runEnv) {
	e.client = usageSequence(
		usageResponse(longGeneratedText(), costGenModel, costGenTokens),
		usageResponse(shortGeneratedText(), costShortModel, costShortTokens),
	)
	e.useReviewClient(usageLLM(usageResponse(reviewJSON("A short body.", "A shorter body."), costReviewModel, costReviewTokens)))
}

func TestRunCostLinesAllStages(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	useFixedClock(e, costGenTime, costShortTime, costReviewTime)
	useAllStagesCost(e)

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	requireCostLines(t, e, []string{
		costGenLine,
		costShortLine,
		costReviewLine,
		costRunTotal,
		costArtTotal,
	})
	// The cost lines come after the token lines, which come after the summary.
	if first, lastToken := strings.Index(e.stderr.String(), costPrefix), strings.LastIndex(e.stderr.String(), tokensPrefix); first < lastToken {
		t.Errorf("cost lines appear before the token lines:\n%s", e.stderr.String())
	}
	// The --out file's cost line is the article cost rounded to four decimals.
	data, err := os.ReadFile(e.outPath)
	if err != nil {
		t.Fatalf("read --out: %v", err)
	}
	if want := "- Estimated cost: 0.0011 USD\n"; !strings.Contains(string(data), want) {
		t.Errorf("--out has no line %q:\n%s", want, data)
	}
	requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
}

func TestRunCostLinesRateBasis(t *testing.T) {
	cases := []struct {
		name     string
		model    string
		usage    llm.Usage
		at       time.Time
		wantLine string
	}{
		{
			"deepseek peak",
			"deepseek-flash",
			llm.KnownUsage(llm.TokenCounts{Input: 100, Output: 10}),
			costPeakTime,
			"yt2column: cost: generation run known usd=0.000042 peak at=2026-10-19T01:00:00Z",
		},
		{
			"deepseek off-peak",
			"deepseek-flash",
			llm.KnownUsage(llm.TokenCounts{Input: 100, Output: 10}),
			costOffPeakTime,
			"yt2column: cost: generation run known usd=0.000021 off-peak at=2026-10-19T04:30:00Z",
		},
		{
			"claude flat",
			costGenModel,
			costGenTokens,
			costGenTime,
			costGenLine,
		},
		{
			"claude short-prompt",
			costReviewModel,
			costReviewTokens,
			costGenTime,
			"yt2column: cost: generation run known usd=0.000200 short-prompt at=2026-10-16T01:00:00Z",
		},
		{
			"claude long-prompt",
			costReviewModel,
			llm.KnownUsage(llm.TokenCounts{Input: 100001}),
			costGenTime,
			"yt2column: cost: generation run known usd=0.050001 long-prompt at=2026-10-16T01:00:00Z",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			e.withSteps = true
			e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
			useFixedClock(e, tc.at)
			e.client = usageLLM(usageResponse(shortGeneratedText(), tc.model, tc.usage))

			if code := e.run(); code != exitOK {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
			}
			if got := costStageLines(e.stderr.String()); !slices.Equal(got, []string{tc.wantLine}) {
				t.Errorf("stage cost lines = %q, want %q", got, tc.wantLine)
			}
		})
	}
}

func TestRunCostLinesUnknown(t *testing.T) {
	cases := []struct {
		name       string
		client     llm.LLMClient
		wantReason string
		wantExit   int
	}{
		{
			"unknown usage",
			usageLLM(usageResponse(shortGeneratedText(), costGenModel, llm.UnknownUsage(llm.RuleNotNonNegativeInteger, "completion_tokens"))),
			costReasonUsageUnknown,
			exitOK,
		},
		{
			"no model",
			funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
				return llm.GenerateResponse{Usage: costGenTokens}, fmt.Errorf("stub: %w", llm.ErrTruncated)
			}),
			costReasonNoModel,
			exitFailure,
		},
		{
			"unpriced model",
			usageLLM(usageResponse(shortGeneratedText(), "gpt-5", costGenTokens)),
			costReasonUnpricedModel,
			exitOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			e.withSteps = true
			e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
			useFixedClock(e, costGenTime)
			e.client = tc.client

			if code := e.run(); code != tc.wantExit {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, tc.wantExit, e.stderr.String())
			}
			requireCostLines(t, e, []string{
				"yt2column: cost: generation run unknown reason=" + tc.wantReason + " at=2026-10-16T01:00:00Z",
				"yt2column: cost-total: run unknown",
				"yt2column: cost-total: article unknown",
			})
			for _, line := range costStageLines(e.stderr.String()) {
				if strings.Contains(line, costUSD) {
					t.Errorf("an unknown cost line carries a number: %q", line)
				}
			}
			if tc.wantExit == exitOK {
				requirePublished(t, e.outPath)
			} else {
				requireNoFile(t, e.outPath)
			}
		})
	}
}

func TestRunCostLinesSkippedStages(t *testing.T) {
	t.Run("shorten not called", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		useFixedClock(e, costGenTime, costReviewTime)
		e.client = usageLLM(usageResponse(shortGeneratedText(), costGenModel, costGenTokens))
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireCostLines(t, e, []string{
			costGenLine,
			"yt2column: cost: review run known usd=0.000200 short-prompt at=2026-10-16T04:03:15Z",
			"yt2column: cost-total: run known usd=0.000800",
			"yt2column: cost-total: article known usd=0.000800",
		})
	})

	t.Run("review disabled", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-review", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costGenTime, costShortTime)
		e.client = usageSequence(
			usageResponse(longGeneratedText(), costGenModel, costGenTokens),
			usageResponse(shortGeneratedText(), costShortModel, costShortTokens),
		)

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireCostLines(t, e, []string{
			costGenLine,
			costShortLine,
			"yt2column: cost-total: run known usd=0.000900",
			"yt2column: cost-total: article known usd=0.000900",
		})
	})
}

// TestRunCostLinesGoldenFormat pins the literal spellings of the known and the
// unknown line, so a change to a prefix, a word, usd=, reason=, or at= is
// caught even though the other tests build the expected lines from constants.
func TestRunCostLinesGoldenFormat(t *testing.T) {
	t.Run("known", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costGenTime)
		e.client = usageLLM(usageResponse(shortGeneratedText(), costGenModel, costGenTokens))
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireCostLines(t, e, []string{
			"yt2column: cost: generation run known usd=0.000600 flat at=2026-10-16T01:00:00Z",
			"yt2column: cost-total: run known usd=0.000600",
			"yt2column: cost-total: article known usd=0.000600",
		})
	})

	t.Run("unknown", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costGenTime)
		e.client = usageLLM(usageResponse(shortGeneratedText(), "gpt-5", costGenTokens))
		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireCostLines(t, e, []string{
			"yt2column: cost: generation run unknown reason=unpriced-model at=2026-10-16T01:00:00Z",
			"yt2column: cost-total: run unknown",
			"yt2column: cost-total: article unknown",
		})
	})
}

func TestCostTimeWord(t *testing.T) {
	if got := costTimeWord(time.Time{}); got != costNoTime {
		t.Errorf("costTimeWord(zero) = %q, want %q", got, costNoTime)
	}
	if got := costTimeWord(costGenTime); got != "2026-10-16T01:00:00Z" {
		t.Errorf("costTimeWord = %q, want the CallTimeLayout form", got)
	}
}

// TestCostWords checks that the enum-to-word functions map every known value
// and fall back to "invalid" for the zero value and a value outside the
// enumeration.
func TestCostWords(t *testing.T) {
	basis := []struct {
		value pricing.RateBasis
		want  string
	}{
		{pricing.BasisFlat, costBasisFlat},
		{pricing.BasisOffPeak, costBasisOffPeak},
		{pricing.BasisPeak, costBasisPeak},
		{pricing.BasisShortPrompt, costBasisShortPrompt},
		{pricing.BasisLongPrompt, costBasisLongPrompt},
		{pricing.BasisUnset, costInvalid},
		{pricing.RateBasis(99), costInvalid},
	}
	for _, tc := range basis {
		if got := costBasisWord(tc.value); got != tc.want {
			t.Errorf("costBasisWord(%d) = %q, want %q", tc.value, got, tc.want)
		}
	}
	reasons := []struct {
		value pricing.UnknownReason
		want  string
	}{
		{pricing.ReasonUsageUnknown, costReasonUsageUnknown},
		{pricing.ReasonNoModel, costReasonNoModel},
		{pricing.ReasonUnpricedModel, costReasonUnpricedModel},
		{pricing.ReasonUnset, costInvalid},
		{pricing.ReasonInvalid, costInvalid},
		{pricing.UnknownReason(99), costInvalid},
	}
	for _, tc := range reasons {
		if got := costReasonWord(tc.value); got != tc.want {
			t.Errorf("costReasonWord(%d) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestRunCostLinesOmitModel(t *testing.T) {
	t.Run("unpriced model name stays out of the cost lines", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costGenTime)
		e.client = usageLLM(usageResponse(shortGeneratedText(), "mystery-model", costGenTokens))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		if !strings.Contains(e.stderr.String(), tokensModel+"mystery-model") {
			t.Errorf("the token line does not name the model:\n%s", e.stderr.String())
		}
		for _, line := range costStageLines(e.stderr.String()) {
			if strings.Contains(line, "mystery-model") {
				t.Errorf("a cost line names the model: %q", line)
			}
		}
		requireSafeOutput(t, e.stdout.String(), e.stderr.String(), e.outPath)
	})

	t.Run("a secret in the model name is masked on stderr", func(t *testing.T) {
		// The mystery-model subtest above pins that the cost lines omit the
		// model. Here redact.Line masks the secret from the CLI's own lines, so
		// the check pins that masking still holds. The published article header
		// (--out and Slack) writes the raw model name, a pre-existing behavior
		// this task does not change, so this subtest deliberately does not
		// claim the published output is secret-free.
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costGenTime)
		e.client = usageLLM(usageResponse(shortGeneratedText(), testAPIKey, costGenTokens))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		for _, s := range secretStrings() {
			if strings.Contains(e.stderr.String(), s) {
				t.Errorf("stderr holds a secret or its tail")
			}
		}
	})
}

func TestRunCostLinesBeforeLLM(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, e *runEnv)
	}{
		{"usage error", func(_ *testing.T, e *runEnv) { e.args = []string{runVideoURL} }},
		{"configuration error", func(_ *testing.T, e *runEnv) { delete(e.env, "DEEPSEEK_API_KEY") }},
		{"--after rejection", func(_ *testing.T, e *runEnv) {
			e.args = []string{"--after", "generation", "--out", e.outPath, runVideoURL}
		}},
		{"transcript failure", func(t *testing.T, e *runEnv) { removeVideoCache(t, e.cacheDir) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newRunEnv(t)
			useFixedClock(e, costGenTime)
			tc.prepare(t, e)
			if code := e.run(); code == exitOK {
				t.Fatalf("exit code = %d, want a failure\nstderr:\n%s", code, e.stderr.String())
			}
			requireNoCostLines(t, e)
		})
	}
}

func TestRunCostLinesFailure(t *testing.T) {
	// Each rejected-response sentinel that carries a usage and a model still
	// records the generation stage's known cost before the failure line.
	for _, sentinel := range []error{llm.ErrTruncated, llm.ErrUnexpectedFinishReason, llm.ErrEmptyResponse} {
		t.Run("rejected response: "+sentinel.Error(), func(t *testing.T) {
			e := newRunEnv(t)
			e.withSteps = true
			e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
			useFixedClock(e, costGenTime)
			e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
				return usageResponse("", costGenModel, costGenTokens), fmt.Errorf("stub: %w", sentinel)
			})

			if code := e.run(); code == exitOK {
				t.Fatalf("exit code = %d, want a failure (the response was rejected)", code)
			}
			requireCostLines(t, e, []string{
				costGenLine,
				"yt2column: cost-total: run known usd=0.000600",
				"yt2column: cost-total: article known usd=0.000600",
			})
			if last, failure := strings.LastIndex(e.stderr.String(), costTotalPrefix), strings.Index(e.stderr.String(), "the write stage failed"); failure < last {
				t.Errorf("the failure line appears before the cost lines:\n%s", e.stderr.String())
			}
		})
	}

	t.Run("generation failure with no usage", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--no-shorten", "--no-review", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costGenTime)
		e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			return llm.GenerateResponse{}, context.DeadlineExceeded
		})

		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the call had no usage)", code)
		}
		requireCostLines(t, e, []string{
			"yt2column: cost: generation run unknown reason=usage-unknown at=2026-10-16T01:00:00Z",
			"yt2column: cost-total: run unknown",
			"yt2column: cost-total: article unknown",
		})
	})

	t.Run("review call fails", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		useFixedClock(e, costGenTime, costShortTime, costReviewTime)
		e.client = usageSequence(
			usageResponse(longGeneratedText(), costGenModel, costGenTokens),
			usageResponse(shortGeneratedText(), costShortModel, costShortTokens),
		)
		e.useReviewClient(funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			return llm.GenerateResponse{}, errStubLLM
		}))

		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the review call must fail)", code)
		}
		requireCostLines(t, e, []string{
			costGenLine,
			costShortLine,
			"yt2column: cost: review run unknown reason=usage-unknown at=2026-10-16T04:03:15Z",
			"yt2column: cost-total: run unknown",
			"yt2column: cost-total: article unknown",
		})
	})

	t.Run("length limit after review", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		tooLong := "A short body." + strings.Repeat("a", 10000)
		useFixedClock(e, costGenTime, costReviewTime)
		e.client = usageLLM(usageResponse(shortGeneratedText(), costGenModel, costGenTokens))
		e.useReviewClient(usageLLM(usageResponse(reviewJSON("A short body.", tooLong), costReviewModel, costReviewTokens)))

		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the final length check must fail)", code)
		}
		requireCostLines(t, e, []string{
			costGenLine,
			costReviewLine,
			"yt2column: cost-total: run known usd=0.000800",
			"yt2column: cost-total: article known usd=0.000800",
		})
	})

	t.Run("publish failure after every stage", func(t *testing.T) {
		e := newRunEnv(t)
		e.withSteps = true
		useFixedClock(e, costGenTime, costShortTime, costReviewTime)
		useAllStagesCost(e)
		e.useWebhook(t, failMessage(1, http.StatusServiceUnavailable, "boom"))

		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the post must fail)", code)
		}
		requireCostLines(t, e, []string{costGenLine, costShortLine, costReviewLine, costRunTotal, costArtTotal})
	})

	t.Run("--after accepts the input then fails", func(t *testing.T) {
		dir := saveCostArtifacts(t, func(e *runEnv) {
			useFixedClock(e, costGenTime, costShortTime, costReviewTime)
			useAllStagesCost(e)
		})
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "generation", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costShortTime, costReviewTime)
		e.client = funcLLM(func(context.Context, llm.GenerateRequest) (llm.GenerateResponse, error) {
			return llm.GenerateResponse{}, errStubLLM
		})
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))

		if code := e.run(); code == exitOK {
			t.Fatalf("exit code = %d, want a failure (the shorten call must fail)", code)
		}
		requireCostLines(t, e, []string{
			"yt2column: cost: generation reused known usd=0.000600 flat at=2026-10-16T01:00:00Z",
			"yt2column: cost: shorten run unknown reason=usage-unknown at=2026-10-16T04:02:41Z",
			"yt2column: cost-total: run unknown",
			"yt2column: cost-total: article unknown",
		})
	})
}

func TestRunCostLinesReviewLogFailure(t *testing.T) {
	e := newRunEnv(t)
	e.withSteps = true
	e.useWebhook(t, nil)
	logPath := filepath.Join(e.base, "review.md")
	e.args = []string{"--review-log", logPath, "--slack", runVideoURL}
	useFixedClock(e, costGenTime, costReviewTime)
	e.client = usageLLM(usageResponse(shortGeneratedText(), costGenModel, costGenTokens))
	e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))
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
	// The cost lines appear once, not again in the review-log failure report.
	if got := len(costStageLines(e.stderr.String())); got != 2 {
		t.Errorf("stage cost lines = %d, want 2:\n%s", got, e.stderr.String())
	}
	if got := len(costTotalLines(e.stderr.String())); got != 2 {
		t.Errorf("total cost lines = %d, want 2:\n%s", got, e.stderr.String())
	}
}

// saveCostArtifacts saves a run with --artifacts and --keep-cache, set up by
// setup, and returns the artifacts directory. It mirrors saveArtifactsOnce but
// leaves the review client to setup, so a cost test can give the review stage a
// priced model and a known usage.
func saveCostArtifacts(t *testing.T, setup func(*runEnv)) string {
	t.Helper()
	e := newRunEnv(t)
	e.withSteps = true
	dir := artifactsPath(t, e.base)
	e.args = []string{"--artifacts", dir, "--keep-cache", "--out", e.outPath, runVideoURL}
	setup(e)
	if code := e.run(); code != exitOK {
		t.Fatalf("the artifacts run failed: exit code = %d\nstderr:\n%s", code, e.stderr.String())
	}
	return dir
}

func TestRunCostLinesResume(t *testing.T) {
	// Each subtest saves its own artifacts, so a resume never reads a record a
	// sibling subtest wrote.
	save := func(t *testing.T) string {
		t.Helper()
		return saveCostArtifacts(t, func(e *runEnv) {
			useFixedClock(e, costGenTime, costShortTime, costReviewTime)
			useAllStagesCost(e)
		})
	}

	t.Run("after review reuses every stage", func(t *testing.T) {
		dir := save(t)
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireCostLines(t, e, []string{
			"yt2column: cost: generation reused known usd=0.000600 flat at=2026-10-16T01:00:00Z",
			"yt2column: cost: shorten reused known usd=0.000300 flat at=2026-10-16T04:02:41Z",
			"yt2column: cost: review reused known usd=0.000200 short-prompt at=2026-10-16T04:03:15Z",
			"yt2column: cost-total: run known usd=0.000000",
			costArtTotal,
		})
	})

	t.Run("after shorten runs review", func(t *testing.T) {
		dir := save(t)
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "shorten", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costReviewTime)
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireCostLines(t, e, []string{
			"yt2column: cost: generation reused known usd=0.000600 flat at=2026-10-16T01:00:00Z",
			"yt2column: cost: shorten reused known usd=0.000300 flat at=2026-10-16T04:02:41Z",
			costReviewLine,
			"yt2column: cost-total: run known usd=0.000200",
			costArtTotal,
		})
	})

	t.Run("after generation runs shorten and review", func(t *testing.T) {
		dir := save(t)
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "generation", "--out", e.outPath, runVideoURL}
		useFixedClock(e, costShortTime, costReviewTime)
		e.client = usageSequence(usageResponse(shortGeneratedText(), costShortModel, costShortTokens))
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		requireCostLines(t, e, []string{
			"yt2column: cost: generation reused known usd=0.000600 flat at=2026-10-16T01:00:00Z",
			costShortLine,
			costReviewLine,
			"yt2column: cost-total: run known usd=0.000500",
			costArtTotal,
		})
	})
}

func TestRunCostLinesResumeUnknownStage(t *testing.T) {
	// Save a run whose generation usage is unknown, so the reused generation
	// makes the article total unknown while the run's own stages stay known.
	dir := saveCostArtifacts(t, func(e *runEnv) {
		useFixedClock(e, costGenTime, costShortTime, costReviewTime)
		e.client = usageSequence(
			usageResponse(longGeneratedText(), costGenModel, llm.UnknownUsage(llm.RuleNotNonNegativeInteger, "completion_tokens")),
			usageResponse(shortGeneratedText(), costShortModel, costShortTokens),
		)
		e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))
	})

	e := newRunEnv(t)
	e.withSteps = true
	e.args = []string{"--artifacts", dir, "--after", "generation", "--out", e.outPath, runVideoURL}
	useFixedClock(e, costShortTime, costReviewTime)
	e.client = usageSequence(usageResponse(shortGeneratedText(), costShortModel, costShortTokens))
	e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))

	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
	if got := costTotalLines(e.stderr.String()); !slices.Equal(got, []string{
		"yt2column: cost-total: run known usd=0.000500",
		"yt2column: cost-total: article unknown",
	}) {
		t.Errorf("total cost lines = %q", got)
	}
}

func TestRunCostLinesResumeUsesSavedTime(t *testing.T) {
	dsTokens := llm.KnownUsage(llm.TokenCounts{Input: 100, Output: 10})

	t.Run("a peak save is peak on an off-peak resume", func(t *testing.T) {
		dir := saveCostArtifacts(t, func(e *runEnv) {
			useFixedClock(e, costPeakTime, costReviewTime)
			e.client = usageSequence(usageResponse(shortGeneratedText(), "deepseek-flash", dsTokens))
			e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))
		})
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}
		// The resume clock is the other window; the reused stage must ignore it.
		useFixedClock(e, costOffPeakTime)

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		first := costStageLines(e.stderr.String())
		if len(first) != 2 || first[0] != "yt2column: cost: generation reused known usd=0.000042 peak at=2026-10-19T01:00:00Z" {
			t.Errorf("reused generation line = %q, want the saved peak basis and time", first)
		}
	})

	t.Run("an off-peak save is off-peak on a peak resume", func(t *testing.T) {
		dir := saveCostArtifacts(t, func(e *runEnv) {
			useFixedClock(e, costOffPeakTime, costReviewTime)
			e.client = usageSequence(usageResponse(shortGeneratedText(), "deepseek-flash", dsTokens))
			e.useReviewClient(usageLLM(usageResponse(emptyReviewJSON, costReviewModel, costReviewTokens)))
		})
		e := newRunEnv(t)
		e.withSteps = true
		e.args = []string{"--artifacts", dir, "--after", "review", "--out", e.outPath, runVideoURL}
		// The resume clock is the other window; the reused stage must ignore it.
		useFixedClock(e, costPeakTime)

		if code := e.run(); code != exitOK {
			t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
		}
		first := costStageLines(e.stderr.String())
		if len(first) != 2 || first[0] != "yt2column: cost: generation reused known usd=0.000021 off-peak at=2026-10-19T04:30:00Z" {
			t.Errorf("reused generation line = %q, want the saved off-peak basis and time", first)
		}
	})
}
