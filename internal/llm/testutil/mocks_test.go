//go:build test

package llmtestutil

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
)

func TestFakeLLMClientRecordsAndReturns(t *testing.T) {
	result := llm.GenerateResponse{Text: "generated", Model: "deepseek-chat"}
	fake := &FakeLLMClient{Result: result}
	ctx := context.Background()
	requests := []llm.GenerateRequest{
		{SystemPrompt: "system-1", UserPrompt: "user-1", MaxOutputTokens: 256},
		{SystemPrompt: "system-2", UserPrompt: "user-2", MaxOutputTokens: 512},
	}

	for _, req := range requests {
		got, err := fake.Generate(ctx, req)
		if err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}
		if !reflect.DeepEqual(got, result) {
			t.Errorf("Generate result = %+v, want %+v", got, result)
		}
	}
	if len(fake.Calls) != len(requests) {
		t.Fatalf("recorded %d calls, want %d", len(fake.Calls), len(requests))
	}
	for i, req := range requests {
		if fake.Calls[i].Ctx != ctx {
			t.Errorf("call %d did not record the context", i)
		}
		if !reflect.DeepEqual(fake.Calls[i].Request, req) {
			t.Errorf("call %d request = %+v, want %+v", i, fake.Calls[i].Request, req)
		}
	}
}

func TestFakeLLMClientReturnsError(t *testing.T) {
	wantErr := errors.New("generate failed")
	fake := &FakeLLMClient{Err: wantErr}

	_, err := fake.Generate(context.Background(), llm.GenerateRequest{})
	if !errors.Is(err, wantErr) {
		t.Errorf("Generate error = %v, want %v", err, wantErr)
	}
}

// recordingReporter is an ErrorReporter that keeps the messages it is given,
// so a test can assert that a fake reported an unexpected call.
type recordingReporter struct{ messages []string }

func (r *recordingReporter) Errorf(format string, args ...any) {
	r.messages = append(r.messages, fmt.Sprintf(format, args...))
}

func TestFakeLLMSequenceReturnsStepsInOrder(t *testing.T) {
	results := []llm.GenerateResponse{
		{Text: "first", Model: "m1"},
		{Text: "second", Model: "m2"},
	}
	fake := &FakeLLMSequence{Steps: []FakeLLMSequenceStep{
		{Result: results[0]},
		{Result: results[1]},
	}}
	ctx := context.Background()

	for i, want := range results {
		got, err := fake.Generate(ctx, llm.GenerateRequest{UserPrompt: fmt.Sprintf("u%d", i)})
		if err != nil {
			t.Fatalf("call %d error = %v, want nil", i, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("call %d result = %+v, want %+v", i, got, want)
		}
	}
	if len(fake.Calls) != len(results) {
		t.Fatalf("recorded %d calls, want %d", len(fake.Calls), len(results))
	}
	if fake.Calls[0].Ctx != ctx || fake.Calls[0].Request.UserPrompt != "u0" {
		t.Errorf("call 0 = %+v, want the recorded context and request", fake.Calls[0])
	}
}

func TestFakeLLMSequenceRespondFunc(t *testing.T) {
	wantErr := errors.New("step error")
	fake := &FakeLLMSequence{Steps: []FakeLLMSequenceStep{
		{Respond: func(_ context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
			return llm.GenerateResponse{Text: req.UserPrompt}, nil
		}},
		{Err: wantErr},
	}}

	got, err := fake.Generate(context.Background(), llm.GenerateRequest{UserPrompt: "echo"})
	if err != nil || got.Text != "echo" {
		t.Errorf("call 0 = (%+v, %v), want the echoed text and no error", got, err)
	}
	if _, err := fake.Generate(context.Background(), llm.GenerateRequest{}); !errors.Is(err, wantErr) {
		t.Errorf("call 1 error = %v, want %v", err, wantErr)
	}
}

func TestFakeLLMSequenceReportsCallPastLastStep(t *testing.T) {
	reporter := &recordingReporter{}
	fake := &FakeLLMSequence{TB: reporter, Steps: []FakeLLMSequenceStep{{}}}

	if _, err := fake.Generate(context.Background(), llm.GenerateRequest{}); err != nil {
		t.Fatalf("call 0 error = %v, want nil", err)
	}
	if _, err := fake.Generate(context.Background(), llm.GenerateRequest{}); !errors.Is(err, errSequenceExhausted) {
		t.Errorf("call 1 error = %v, want errSequenceExhausted", err)
	}
	if len(reporter.messages) != 1 {
		t.Errorf("reporter got %d messages, want 1", len(reporter.messages))
	}
}
