//go:build test

package llmtestutil

import (
	"context"
	"errors"
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
