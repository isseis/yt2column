//go:build test

// Package llmtestutil provides test doubles for the LLM client stage.
package llmtestutil

import (
	"context"
	"errors"

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

// errSequenceExhausted is returned by FakeLLMSequence when it is called more
// times than it has steps. It is a package-level sentinel so the failure is
// comparable and no per-call error is allocated.
var errSequenceExhausted = errors.New("FakeLLMSequence called past its last step")

// ErrorReporter is the part of testing.TB that FakeLLMSequence uses to report
// an unexpected call. testing.TB satisfies it, and a test can substitute a
// recorder to assert the failure was reported.
type ErrorReporter interface {
	Errorf(format string, args ...any)
}

// FakeLLMSequence is an llm.LLMClient that returns a different prepared
// response on each call, in order. It suits a stage that uses one client for
// several calls, such as the generation and shorten steps. It implements the
// interface with pointer receivers so the recorded calls are visible to the
// caller.
type FakeLLMSequence struct {
	// TB, when set, reports a call past the last step, so an unexpected
	// extra call is surfaced instead of silently returning the sentinel.
	TB    ErrorReporter
	Steps []FakeLLMSequenceStep
	Calls []FakeLLMClientCall
}

// FakeLLMSequenceStep is one prepared response of a FakeLLMSequence. When
// Respond is non-nil it computes the response from the call, which lets a step
// cancel the context or assert on the request; otherwise Result and Err are
// returned as they are.
type FakeLLMSequenceStep struct {
	Result  llm.GenerateResponse
	Err     error
	Respond func(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error)
}

var _ llm.LLMClient = (*FakeLLMSequence)(nil)

// Generate records the call and returns the next prepared response.
func (f *FakeLLMSequence) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	f.Calls = append(f.Calls, FakeLLMClientCall{Ctx: ctx, Request: req})
	index := len(f.Calls) - 1
	if index >= len(f.Steps) {
		if f.TB != nil {
			f.TB.Errorf("FakeLLMSequence.Generate called %d times, only %d steps prepared", len(f.Calls), len(f.Steps))
		}
		return llm.GenerateResponse{}, errSequenceExhausted
	}
	step := f.Steps[index]
	if step.Respond != nil {
		return step.Respond(ctx, req)
	}
	return step.Result, step.Err
}
