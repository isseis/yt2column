//go:build integration

package claude

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	"github.com/isseis/yt2column/internal/pricing"
	pricingtestutil "github.com/isseis/yt2column/internal/pricing/testutil"
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
	// The configured model is operator-supplied and is logged as a
	// diagnostic, so guard it the same way as the response's Model below: a
	// value that embeds the API key or the workspace ID (for example a shell
	// mishap when setting the model variable) must fail rather than appear in
	// the output. The failure message names the field only, never its value,
	// the key, or the workspace ID.
	key, err := settings.APIKey.Reveal()
	if err != nil {
		t.Fatal("the API key cannot be revealed")
	}
	workspaceID, workspaceSpecified := settings.WorkspaceID.Value()
	if containsSecret(settings.Model, key, workspaceID) {
		t.Fatal("the configured model contains the API key or the workspace ID; it is not logged")
	}
	// The model name is an untrusted string; %+q escapes control and non-ASCII
	// characters so it cannot alter the terminal or log.
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
		now := time.Now()
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
		logResponse(t, response, key, workspaceID, now)
	})

	t.Run("truncated", func(t *testing.T) {
		response, err := value.Generate(context.Background(), llm.GenerateRequest{
			SystemPrompt:    integrationSystemPrompt,
			UserPrompt:      integrationTruncationPrompt,
			MaxOutputTokens: integrationTruncationMaxTokens,
		})
		now := time.Now()
		if err == nil {
			t.Fatalf("Generate() with MaxOutputTokens %d returned no error: the response finished without being truncated, "+
				"so the test's premise no longer holds (likely a model change, not an adapter defect)", integrationTruncationMaxTokens)
		}
		if !errors.Is(err, llm.ErrTruncated) {
			t.Fatalf("Generate() with MaxOutputTokens %d error = %v, want llm.ErrTruncated", integrationTruncationMaxTokens, err)
		}
		logResponse(t, response, key, workspaceID, now)
	})
}

// containsSecret reports whether value holds the API key or, when it is
// non-empty, the workspace ID. Neither may be logged. The empty workspace ID
// (none specified) matches nothing.
func containsSecret(value, key, workspaceID string) bool {
	return strings.Contains(value, key) || (workspaceID != "" && strings.Contains(value, workspaceID))
}

// logResponse logs a response's token counts and estimated cost. It refuses to
// log a Model that contains the API key or the workspace ID: the model is
// provider-controlled, so a faulty response or a TLS-terminating proxy could
// echo either into it, and logging it would then write the live value. The
// failure message names the field only, never its value or the secret. The
// count line carries no response text. Model is escaped with %+q, like the
// other diagnostics, so it cannot alter the terminal or log.
func logResponse(t *testing.T, response llm.GenerateResponse, key, workspaceID string, now time.Time) {
	t.Helper()
	if containsSecret(response.Model, key, workspaceID) {
		t.Fatal("Generate() Model contains the API key or the workspace ID; it is not logged")
	}
	// The cost is estimated from the time Generate returned and the response's
	// own usage, like the CLI's cost line.
	cost := pricing.Estimate(response.Model, response.Usage, now)
	if counts, known := response.Usage.Counts(); known {
		t.Logf("tokens input=%d cache_read=%d cache_write=%d output=%d; cost %s; model %+q",
			counts.Input, counts.CacheRead, counts.CacheWrite, counts.Output,
			pricingtestutil.CostSummary(cost), response.Model)
		return
	}
	t.Logf("tokens unknown reason=%s; cost %s; model %+q",
		response.Usage.UnknownReason(), pricingtestutil.CostSummary(cost), response.Model)
}
