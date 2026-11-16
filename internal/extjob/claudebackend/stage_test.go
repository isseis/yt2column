//go:build test

package claudebackend

import (
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm/claude"
)

// TestClientsUseTheirOwnStage builds clients from a configuration whose review
// selection differs from the generation selection in model and effort (the
// CLI's loader can select this; the production profile does not) and checks
// each client gets its own stage's values.
func TestClientsUseTheirOwnStage(t *testing.T) {
	values := map[string]string{
		"ANTHROPIC_API_KEY":    testKey,
		"YT2COLUMN_CACHE_DIR":  "/tmp/yt2column-stage-test",
		"YT2COLUMN_LLM":        "claude/claude-sonnet-5-5/low",
		"YT2COLUMN_REVIEW_LLM": "claude/claude-haiku-5-5/high",
	}
	cfg, err := config.LoadFor(func(name string) (string, bool) { v, ok := values[name]; return v, ok }, config.Need{Generation: true, Review: true})
	if err != nil {
		t.Fatalf("LoadFor: %v", err)
	}
	llmCfg := cfg.LLM()
	if llmCfg.Generation().Model() == llmCfg.Review().Model() {
		t.Fatalf("test setup: both stages select %q", llmCfg.Generation().Model())
	}
	var got []claude.Options
	if _, err := optionsRecorder(&got).NewClients(llmCfg); err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if len(got) != 2 || got[0].Model != llmCfg.Generation().Model() || got[1].Model != llmCfg.Review().Model() ||
		got[0].Effort != llmCfg.Generation().ClaudeEffort() || got[1].Effort != llmCfg.Review().ClaudeEffort() {
		t.Errorf("options = %+v, want generation then review from their own stages", got)
	}
}
