//go:build integration

package deepseek

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
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
	settings := integrationSettingsFrom(os.Getenv)
	switch settings.action {
	case integrationRun:
	case integrationFail:
		t.Fatal(settings.reason)
	default:
		t.Skip(settings.reason)
	}
	value, err := New(Options{APIKey: settings.apiKey, Model: settings.model, Timeout: integrationGenerateTimeout})
	if err != nil {
		t.Fatalf("New() error = %v", err)
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
		// Model and ModelVersion are untrusted strings; %+q escapes control
		// and non-ASCII characters so they cannot alter the terminal or log.
		t.Logf("Model %+q, ModelVersion %+q", response.Model, response.ModelVersion)
	})

	t.Run("truncated", func(t *testing.T) {
		_, err := value.Generate(context.Background(), llm.GenerateRequest{
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
	})
}
