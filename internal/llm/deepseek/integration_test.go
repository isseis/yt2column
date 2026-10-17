//go:build integration

package deepseek

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/pricing"
	pricingtestutil "github.com/isseis/yt2column/internal/pricing/testutil"
)

const (
	// integrationTruncationMaxTokens is small enough that the reasoning
	// alone exhausts it, so the response ends with finish_reason "length".
	integrationTruncationMaxTokens = 16

	integrationSystemPrompt     = "You are a helpful assistant."
	integrationGeneratePrompt   = "In one short sentence, why is the sky blue?"
	integrationTruncationPrompt = "Explain in several detailed paragraphs how a compiler turns source code into machine code."
)

// TestIntegrationGenerate calls the real DeepSeek API: one ordinary
// generation, then one generation whose output limit forces truncation. It is
// excluded from `make test` by its build tag and is run by
// `make test-integration-deepseek`, which sets the opt-in variable and the
// model name. It makes integrationGenerateCalls Generate calls. Nothing derived from the API key is ever written to the output.
func TestIntegrationGenerate(t *testing.T) {
	settings := deepseektestutil.SettingsFrom(os.Getenv, integrationOptions)
	switch settings.Action {
	case deepseektestutil.ActionRun:
	case deepseektestutil.ActionFail:
		t.Fatal(settings.Reason)
	default:
		t.Skip(settings.Reason)
	}
	value, err := New(Options{APIKey: settings.APIKey, Model: settings.Model, Timeout: integrationGenerateTimeout})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	// The key is revealed here, once, so both subtests share the guard that
	// keeps it out of the log.
	key, err := settings.APIKey.Reveal()
	if err != nil {
		t.Fatal("the API key cannot be revealed")
	}

	t.Run("generate", func(t *testing.T) {
		response, err := value.Generate(context.Background(), llm.GenerateRequest{
			SystemPrompt: integrationSystemPrompt,
			UserPrompt:   integrationGeneratePrompt,
		})
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if strings.TrimSpace(response.Text) == "" {
			t.Error("Generate() Text has no non-whitespace character")
		}
		if response.Model == "" {
			t.Error("Generate() Model is empty")
		}
		logResponse(t, response, key)
	})

	t.Run("truncated", func(t *testing.T) {
		response, err := value.Generate(context.Background(), llm.GenerateRequest{
			SystemPrompt:    integrationSystemPrompt,
			UserPrompt:      integrationTruncationPrompt,
			MaxOutputTokens: integrationTruncationMaxTokens,
		})
		if err == nil {
			t.Fatalf("Generate() with MaxOutputTokens %d returned no error: the response finished without being truncated, "+
				"so the test's premise no longer holds (likely a model change, not an adapter defect)", integrationTruncationMaxTokens)
		}
		if !errors.Is(err, llm.ErrTruncated) {
			t.Fatalf("Generate() with MaxOutputTokens %d error = %v, want llm.ErrTruncated", integrationTruncationMaxTokens, err)
		}
		logResponse(t, response, key)
	})
}

// containsAPIKey reports whether value holds the API key or its last eight
// characters, which must never be logged (security.md: neither the key nor any
// part of it).
func containsAPIKey(value, key string) bool {
	if strings.Contains(value, key) {
		return true
	}
	runes := []rune(key)
	return len(runes) > 8 && strings.Contains(value, string(runes[len(runes)-8:]))
}

// logResponse logs a response's token counts and estimated cost. It refuses to
// log a Model or ModelVersion that contains the API key: both are
// provider-controlled, so a faulty response or a TLS-terminating proxy could
// echo the bearer key into them, and logging them would then write the live
// key. The failure messages name the field only, never its value or the key.
// The count line carries no response text. Model is escaped with %+q, like the
// other diagnostics, so it cannot alter the terminal or log.
func logResponse(t *testing.T, response llm.GenerateResponse, key string) {
	t.Helper()
	for _, field := range []struct{ name, value string }{
		{"Model", response.Model},
		{"ModelVersion", response.ModelVersion},
	} {
		if containsAPIKey(field.value, key) {
			t.Fatalf("Generate() %s contains the API key or its tail; it is not logged", field.name)
		}
	}
	// The cost is estimated from the time Generate returned and the response's
	// own usage, like the CLI's cost line.
	cost := pricing.Estimate(response.Model, response.Usage, time.Now())
	if counts, known := response.Usage.Counts(); known {
		t.Logf("tokens input=%d cache_read=%d cache_write=%d output=%d; cost %s; model %+q, model version %+q",
			counts.Input, counts.CacheRead, counts.CacheWrite, counts.Output,
			pricingtestutil.CostSummary(cost), response.Model, response.ModelVersion)
		return
	}
	t.Logf("tokens unknown reason=%s; cost %s; model %+q, model version %+q",
		response.Usage.UnknownReason(), pricingtestutil.CostSummary(cost), response.Model, response.ModelVersion)
}
