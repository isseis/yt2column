//go:build test

// Package llmtestutil provides test doubles for the LLM client stage.
package llmtestutil

import (
	"context"

	"github.com/isseis/yt2column/internal/llm"
)

// FakeLLMClient is a configurable llm.LLMClient that records its calls. It
// implements the interface with pointer receivers so the recorded calls are
// visible to the caller.
type FakeLLMClient struct {
	Result llm.GenerateResponse
	Err    error
	Calls  []FakeLLMClientCall
}

// FakeLLMClientCall records the arguments of one Generate call.
type FakeLLMClientCall struct {
	Ctx     context.Context
	Request llm.GenerateRequest
}

var _ llm.LLMClient = (*FakeLLMClient)(nil)

// Generate records the call and returns the configured result and error.
func (f *FakeLLMClient) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	f.Calls = append(f.Calls, FakeLLMClientCall{Ctx: ctx, Request: req})
	return f.Result, f.Err
}
