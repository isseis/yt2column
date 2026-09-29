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
	req := llm.GenerateRequest{SystemPrompt: "system", UserPrompt: "user", MaxOutputTokens: 256}

	got, err := fake.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !reflect.DeepEqual(got, result) {
		t.Errorf("Generate result = %+v, want %+v", got, result)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("recorded %d calls, want 1", len(fake.Calls))
	}
	if fake.Calls[0].Ctx != ctx {
		t.Error("Generate did not record the context")
	}
	if !reflect.DeepEqual(fake.Calls[0].Req, req) {
		t.Errorf("recorded request = %+v, want %+v", fake.Calls[0].Req, req)
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
