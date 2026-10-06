//go:build test || integration

package main

import (
	"context"
	"sync/atomic"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/nilcheck"
)

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
