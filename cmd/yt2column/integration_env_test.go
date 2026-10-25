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
	// integrationEnvTestModel is a model name no other test environment uses,
	// so a value inherited from the process environment cannot stand in for it.
	integrationEnvTestModel = "deepseek-integration-env-test"
)

// TestIntegrationRunProviderSelection checks, without calling the API, that the
// environment newIntegrationRun builds sends both the generation and the review
// stage to DeepSeek. The CLI integration tests must keep calling only DeepSeek
// after the default provider changes to Claude. The environment is read through
// a recording LookupFunc, so the test also proves that no Anthropic variable is
// read. No LLM client is built and nothing is posted to a webhook.
func TestIntegrationRunProviderSelection(t *testing.T) {
	key, err := secret.New(integrationEnvTestKey)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	r := newIntegrationRun(t, deepseektestutil.IntegrationSettings{APIKey: key, Model: integrationEnvTestModel})

	// The default provider is DeepSeek today, so the loader result alone cannot
	// tell an explicit DeepSeek from the default. Check that the common
	// environment names the provider, which is what keeps the integration tests
	// on DeepSeek once the default becomes Claude.
	if _, ok := r.env["YT2COLUMN_LLM_PROVIDER"]; !ok {
		t.Fatal("the integration environment does not set YT2COLUMN_LLM_PROVIDER explicitly")
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
	if got := llmCfg.Provider(); got != config.ProviderDeepSeek {
		t.Errorf("generation provider = %d, want %d", got, config.ProviderDeepSeek)
	}
	if got := llmCfg.Model(); got != integrationEnvTestModel {
		t.Errorf("generation model = %q, want %q", got, integrationEnvTestModel)
	}
	if got := llmCfg.ReviewProvider(); got != config.ProviderDeepSeek {
		t.Errorf("review provider = %d, want %d", got, config.ProviderDeepSeek)
	}
	if got := llmCfg.ReviewModel(); got != integrationEnvTestModel {
		t.Errorf("review model = %q, want %q", got, integrationEnvTestModel)
	}
	for _, name := range []string{"ANTHROPIC_API_KEY", "YT2COLUMN_CLAUDE_EFFORT", "ANTHROPIC_WORKSPACE_ID"} {
		if slices.Contains(read, name) {
			t.Errorf("the environment read %s, but both stages use DeepSeek", name)
		}
	}
}
