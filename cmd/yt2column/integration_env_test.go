//go:build test

package main

import (
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/secret"
)

const (
	// integrationEnvTestKey is a fake DeepSeek key: this test never calls the
	// API, it only reads the environment through the config loader.
	integrationEnvTestKey = "sk-integration-env-test-0123456789"
	// integrationEnvTestModel is the model name the test passes to
	// newIntegrationRun, so the loaded model proves the argument reached the
	// environment.
	integrationEnvTestModel = "deepseek-integration-env-test"
)

// TestIntegrationRunProviderSelection checks, without calling the API, that the
// environment newIntegrationRun builds sends both the generation and the review
// stage to DeepSeek. The CLI integration tests must call only DeepSeek, not the
// default provider, Claude. The environment is read through
// a recording LookupFunc, so the test also proves that no Anthropic variable is
// read. No LLM client is built and nothing is posted to a webhook.
func TestIntegrationRunProviderSelection(t *testing.T) {
	key, err := secret.New(integrationEnvTestKey)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	r := newIntegrationRun(t, deepseektestutil.IntegrationSettings{
		Action: deepseektestutil.ActionRun,
		APIKey: key,
		Model:  integrationEnvTestModel,
	})

	// Check the map directly as well: it is the value newIntegrationRun
	// writes, deepseek followed by the model it was given, which keeps the
	// integration tests off the default provider, Claude.
	if got, ok := r.env["YT2COLUMN_LLM"]; !ok || got != "deepseek/"+integrationEnvTestModel {
		t.Fatalf("YT2COLUMN_LLM = %q (set: %t), want %q", got, ok, "deepseek/"+integrationEnvTestModel)
	}

	env := lookupFrom(r.env)
	var read []string
	lookup := func(name string) (string, bool) {
		read = append(read, name)
		return env(name)
	}
	cfg, err := config.LoadFor(lookup, config.Need{Generation: true, Review: true})
	if err != nil {
		t.Fatalf("LoadFor: %v", err)
	}
	llmCfg := cfg.LLM()
	if got := llmCfg.Generation().Provider(); got != config.ProviderDeepSeek {
		t.Errorf("generation provider = %d, want %d", got, config.ProviderDeepSeek)
	}
	if got := llmCfg.Generation().Model(); got != integrationEnvTestModel {
		t.Errorf("generation model = %q, want %q", got, integrationEnvTestModel)
	}
	if got := llmCfg.Review().Provider(); got != config.ProviderDeepSeek {
		t.Errorf("review provider = %d, want %d", got, config.ProviderDeepSeek)
	}
	if got := llmCfg.Review().Model(); got != integrationEnvTestModel {
		t.Errorf("review model = %q, want %q", got, integrationEnvTestModel)
	}
	// The positive check keeps the negative one below meaningful: a run that
	// read no provider variable at all would pass it.
	if !slices.Contains(read, "DEEPSEEK_API_KEY") {
		t.Error("the environment read no DEEPSEEK_API_KEY, so it read no provider variable at all")
	}
	for _, name := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_WORKSPACE_ID"} {
		if slices.Contains(read, name) {
			t.Errorf("the environment read %s, but both stages use DeepSeek", name)
		}
	}
}
