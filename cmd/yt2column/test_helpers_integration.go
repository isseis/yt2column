//go:build test || integration

package main

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/nilcheck"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
)

// cliIntegrationOptions is how TestIntegrationCLI decides whether it runs. It
// has its own opt-in, matching its own make target, and a missing test API
// key fails it instead of skipping, so an opted-in run cannot pass without
// calling the API.
var cliIntegrationOptions = deepseektestutil.IntegrationOptions{
	OptInEnv:   deepseektestutil.CLIOptInEnv,
	MakeTarget: "test-integration-cli",
	MissingKey: deepseektestutil.MissingKeyFail,
}

// gateCLIIntegration reads the environment through getenv and calls body only
// when the CLI integration test runs. Otherwise it skips or fails t with the
// reason, which never contains the API key, and body, which calls run and the
// real LLM client, is never reached.
func gateCLIIntegration(t testing.TB, getenv func(string) string, body func(deepseektestutil.IntegrationSettings)) {
	t.Helper()
	settings := deepseektestutil.SettingsFrom(getenv, cliIntegrationOptions)
	switch settings.Action {
	case deepseektestutil.ActionRun:
		body(settings)
	case deepseektestutil.ActionFail:
		t.Fatal(settings.Reason)
	default:
		t.Skip(settings.Reason)
	}
}

// cliSlackDeepSeekOptions is how TestIntegrationCLISlack decides whether its
// DeepSeek part runs. It shares the webhook test's opt-in and target, and a
// missing test API key fails it, as for TestIntegrationCLI.
var cliSlackDeepSeekOptions = deepseektestutil.IntegrationOptions{
	OptInEnv:   publishertestutil.CLISlackIntegrationOptions.OptInEnv,
	MakeTarget: publishertestutil.CLISlackIntegrationOptions.MakeTarget,
	MissingKey: deepseektestutil.MissingKeyFail,
}

// gateCLISlackIntegration reads the environment through getenv and calls body
// only when both the DeepSeek decision and the webhook decision say the CLI
// webhook integration test runs. Otherwise it skips or fails t with the first
// reason, which never contains the API key or the Webhook URL, and body,
// which calls run, the real LLM client, and the real Webhook, is never
// reached.
func gateCLISlackIntegration(t testing.TB, getenv func(string) string, body func(deepseektestutil.IntegrationSettings, publishertestutil.IntegrationSettings)) {
	t.Helper()
	llmSettings := deepseektestutil.SettingsFrom(getenv, cliSlackDeepSeekOptions)
	switch llmSettings.Action {
	case deepseektestutil.ActionRun:
	case deepseektestutil.ActionFail:
		t.Fatal(llmSettings.Reason)
		return
	default:
		t.Skip(llmSettings.Reason)
		return
	}
	webhookSettings := publishertestutil.SettingsFrom(getenv, publishertestutil.CLISlackIntegrationOptions)
	switch webhookSettings.Action {
	case publishertestutil.ActionRun:
		body(llmSettings, webhookSettings)
	case publishertestutil.ActionFail:
		t.Fatal(webhookSettings.Reason)
	default:
		t.Skip(webhookSettings.Reason)
	}
}

// lookupFrom returns a config.LookupFunc over env, so a test supplies the
// whole environment run sees without changing the process environment. A
// name absent from env is unset.
func lookupFrom(env map[string]string) config.LookupFunc {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// generateCounter counts the Generate calls of every client it wraps.
type generateCounter struct {
	calls atomic.Int64
}

// wrap returns client with its Generate calls counted. A nil client, typed
// or not, is returned as a nil interface, so a failed construction still
// looks failed to the caller.
func (c *generateCounter) wrap(client llm.LLMClient) llm.LLMClient {
	if nilcheck.IsNil(client) {
		return nil
	}
	return &countingClient{inner: client, counter: c}
}

// count returns the number of Generate calls so far.
func (c *generateCounter) count() int64 {
	return c.calls.Load()
}

// countingClient forwards Generate to inner after counting the call.
type countingClient struct {
	inner   llm.LLMClient
	counter *generateCounter
}

// Generate implements llm.LLMClient.
func (c *countingClient) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	c.counter.calls.Add(1)
	return c.inner.Generate(ctx, req)
}
