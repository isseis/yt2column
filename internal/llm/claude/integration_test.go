//go:build integration

package claude

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
)

const (
	integrationSystemPrompt     = "You are a helpful assistant."
	integrationGeneratePrompt   = "In one short sentence, why is the sky blue?"
	integrationTruncationPrompt = "Explain in several detailed paragraphs how a compiler turns source code into machine code."
)

// TestIntegrationGenerate calls the real Anthropic Messages API: one ordinary
// generation, then one generation whose output limit forces truncation. It is
// excluded from `make test` by its build tag and is run by
// `make test-integration-claude`, which sets the opt-in variable, the model
// name, and the effort. It makes integrationGenerateCalls Generate calls.
// Nothing derived from the API key or the workspace ID is ever written to the
// output.
func TestIntegrationGenerate(t *testing.T) {
	settings := claudetestutil.SettingsFrom(os.LookupEnv, integrationOptions)
	switch settings.Action {
	case claudetestutil.ActionRun:
	case claudetestutil.ActionFail:
		t.Fatal(settings.Reason)
	default:
		t.Skip(settings.Reason)
	}
	_, workspaceSpecified := settings.WorkspaceID.Value()
	t.Logf("model %+q, effort %s, workspace id specified %t", settings.Model, settings.Effort, workspaceSpecified)
	value, err := New(integrationClientOptions(settings))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("generate", func(t *testing.T) {
		response, err := value.Generate(context.Background(), llm.GenerateRequest{
			SystemPrompt:    integrationSystemPrompt,
			UserPrompt:      integrationGeneratePrompt,
			MaxOutputTokens: integrationGenerateMaxTokens,
		})
		if errors.Is(err, llm.ErrTruncated) {
			t.Fatalf("Generate() with MaxOutputTokens %d error = %v: the reasoning used the whole output budget; "+
				"an effort above the default may not finish within it", integrationGenerateMaxTokens, err)
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
		// Model is provider-controlled: a faulty response or a
		// TLS-terminating proxy could echo the API key into it, and logging
		// it would then write the live key to the test output. The failure
		// message names the field only, never its value or the key.
		key, err := settings.APIKey.Reveal()
		if err != nil {
			t.Fatal("the API key cannot be revealed")
		}
		if strings.Contains(response.Model, key) {
			t.Fatal("Generate() Model contains the API key; it is not logged")
		}
		// Model is an untrusted string; %+q escapes control and non-ASCII
		// characters so it cannot alter the terminal or log.
		t.Logf("Model %+q", response.Model)
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
