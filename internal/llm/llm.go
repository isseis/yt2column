// Package llm defines the provider-independent LLM client interface and the
// request and response types exchanged with it.
package llm

import "context"

// GenerateRequest is the provider-common request.
// It must not carry provider-specific fields (e.g. DeepSeek thinking).
type GenerateRequest struct {
	SystemPrompt    string
	UserPrompt      string
	MaxOutputTokens int
}

// GenerateResponse is the generated text and the model that produced it.
type GenerateResponse struct {
	Text  string
	Model string
}

// LLMClient sends a provider-common request and returns the generated result.
// Implementations must not return an empty response without an error;
// a truncated generation is an error.
type LLMClient interface { //nolint:revive // LLMClient is the name fixed by the pipeline design
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
