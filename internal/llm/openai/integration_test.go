//go:build integration

package openai

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	openaitestutil "github.com/isseis/yt2column/internal/llm/openai/testutil"
	"github.com/isseis/yt2column/internal/pricing"
	pricingtestutil "github.com/isseis/yt2column/internal/pricing/testutil"
)

const (
	integrationSystemPrompt     = "You are a concise assistant."
	integrationGeneratePrompt   = "In one short sentence, why is the sky blue?"
	integrationTruncationPrompt = "Explain in several detailed paragraphs how a compiler turns source code into machine code."
)

// TestIntegrationResponses calls the real OpenAI Responses API: one ordinary
// generation, then one generation whose output limit forces truncation. It is
// excluded from `make test` by its build tag and is run by
// `make test-integration-openai`, which sets the opt-in variable, the model
// name, and the effort. It makes integrationGenerateCalls Generate calls and
// never retries one. Nothing derived from the API key and no response text is
// ever written to the output.
func TestIntegrationResponses(t *testing.T) {
	settings := openaitestutil.SettingsFrom(os.LookupEnv, integrationOptions)
	switch settings.Action {
	case openaitestutil.ActionRun:
	case openaitestutil.ActionFail:
		t.Fatal(settings.Reason)
	default:
		t.Skip(settings.Reason)
	}
	// The configured model is operator-supplied and is logged as a
	// diagnostic, so guard it the same way as the response's Model below: a
	// value that embeds the API key (for example a shell mishap when setting
	// the model variable) must fail rather than appear in the output. The
	// failure message names the field only, never its value or the key.
	key, err := settings.APIKey.Reveal()
	if err != nil {
		t.Fatal("the API key cannot be revealed")
	}
	if strings.Contains(settings.Model, key) {
		t.Fatal("the configured model contains the API key; it is not logged")
	}
	// The model name is an untrusted string; %+q escapes control and non-ASCII
	// characters so it cannot alter the terminal or log.
	t.Logf("model %+q, effort %s", settings.Model, settings.Effort)
	value, err := New(integrationClientOptions(settings))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("completed", func(t *testing.T) {
		start := time.Now()
		response, err := value.Generate(context.Background(), llm.GenerateRequest{
			SystemPrompt:    integrationSystemPrompt,
			UserPrompt:      integrationGeneratePrompt,
			MaxOutputTokens: integrationGenerateMaxTokens,
		})
		now := time.Now()
		if errors.Is(err, llm.ErrTruncated) {
			t.Fatalf("Generate() with MaxOutputTokens %d error = %v: the reasoning used the whole output budget, "+
				"so the test's premise does not hold; it is not retried with a larger limit", integrationGenerateMaxTokens, err)
		}
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if strings.TrimSpace(response.Text) == "" {
			t.Error("Generate() Text has no non-whitespace character")
		}
		if response.Model == "" {
			t.Error("Generate() Model is empty")
		}
		// The request always names the Standard tier; a response that does
		// not confirm it would leave every OpenAI cost unknown.
		if got := response.Usage.PricingContext(); got != llm.PricingContextStandard30m {
			t.Errorf("Generate() Usage.PricingContext() = %v, want %v", got, llm.PricingContextStandard30m)
		}
		logResponse(t, response, key, now.Sub(start), now)
	})

	t.Run("truncated", func(t *testing.T) {
		start := time.Now()
		response, err := value.Generate(context.Background(), llm.GenerateRequest{
			SystemPrompt:    integrationSystemPrompt,
			UserPrompt:      integrationTruncationPrompt,
			MaxOutputTokens: integrationTruncationMaxTokens,
		})
		now := time.Now()
		if err == nil {
			t.Fatalf("Generate() with MaxOutputTokens %d returned no error: the response finished without being truncated, "+
				"so the test's premise does not hold (likely a model change, not an adapter defect)", integrationTruncationMaxTokens)
		}
		if !errors.Is(err, llm.ErrTruncated) {
			t.Fatalf("Generate() with MaxOutputTokens %d error = %v, want llm.ErrTruncated", integrationTruncationMaxTokens, err)
		}
		logResponse(t, response, key, now.Sub(start), now)
	})
}

// logResponse logs a response's token counts, estimated cost, and duration.
// It refuses to log a Model that contains the API key: the model is
// provider-controlled, so a faulty response or a TLS-terminating proxy could
// echo the key into it, and logging it would then write the live value. The
// failure message names the field only, never its value or the key. The line
// carries no response text. Model is escaped with %+q, like the other
// diagnostics, so it cannot alter the terminal or log.
func logResponse(t *testing.T, response llm.GenerateResponse, key string, elapsed time.Duration, now time.Time) {
	t.Helper()
	if strings.Contains(response.Model, key) {
		t.Fatal("Generate() Model contains the API key; it is not logged")
	}
	// The cost is estimated from the time Generate returned and the response's
	// own usage, like the CLI's cost line.
	cost := pricing.Estimate(response.Model, response.Usage, now)
	if counts, known := response.Usage.Counts(); known {
		t.Logf("tokens input=%d cache_read=%d cache_write=%d output=%d; cost %s; model %+q; elapsed %s",
			counts.Input, counts.CacheRead, counts.CacheWrite, counts.Output,
			pricingtestutil.CostSummary(cost), response.Model, elapsed)
		return
	}
	t.Logf("tokens unknown reason=%s; cost %s; model %+q; elapsed %s",
		response.Usage.UnknownReason(), pricingtestutil.CostSummary(cost), response.Model, elapsed)
}
