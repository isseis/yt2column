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
