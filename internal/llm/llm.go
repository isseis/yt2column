// Package llm defines the provider-independent LLM client interface and the
// request and response types exchanged with it.
package llm

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Provider-independent sentinel errors. LLMClient implementations wrap these
// so that callers can identify a failure with errors.Is.
var (
	ErrInvalidRequest         = errors.New("invalid generate request")
	ErrTruncated              = errors.New("generation truncated by the output token limit")
	ErrUnexpectedFinishReason = errors.New("generation finished for an unexpected reason")
	ErrEmptyResponse          = errors.New("empty generation")
)

// GenerateRequest is the provider-common request.
// It must not carry provider-specific fields (e.g. DeepSeek thinking).
// MaxOutputTokens == 0 leaves the output limit to the provider default;
// a negative value is invalid.
type GenerateRequest struct {
	SystemPrompt    string
	UserPrompt      string
	MaxOutputTokens int
}

// Validate reports whether the request can be sent: both prompts are non-empty
// valid UTF-8 and MaxOutputTokens is not negative. The returned error wraps
// ErrInvalidRequest.
func (r GenerateRequest) Validate() error {
	if r.SystemPrompt == "" {
		return fmt.Errorf("%w: SystemPrompt is empty", ErrInvalidRequest)
	}
	if !utf8.ValidString(r.SystemPrompt) {
		return fmt.Errorf("%w: SystemPrompt is not valid UTF-8", ErrInvalidRequest)
	}
	if r.UserPrompt == "" {
		return fmt.Errorf("%w: UserPrompt is empty", ErrInvalidRequest)
	}
	if !utf8.ValidString(r.UserPrompt) {
		return fmt.Errorf("%w: UserPrompt is not valid UTF-8", ErrInvalidRequest)
	}
	if r.MaxOutputTokens < 0 {
		return fmt.Errorf("%w: MaxOutputTokens is negative", ErrInvalidRequest)
	}
	return nil
}

// GenerateResponse is the generated text and the model that produced it.
// ModelVersion is an opaque, provider-defined identifier of the model or
// backend version that produced the text, comparable only within one provider.
// It is empty when the provider reports none.
type GenerateResponse struct {
	Text         string
	Model        string
	ModelVersion string
}

// LLMClient sends a provider-common request and returns the generated result.
// Implementations must not return an empty response without an error;
// a truncated generation is an error. They report a failure with
// ErrInvalidRequest, ErrTruncated, ErrUnexpectedFinishReason, or
// ErrEmptyResponse, and report a timeout or cancellation with an error that
// matches context.DeadlineExceeded or context.Canceled.
type LLMClient interface { //nolint:revive // LLMClient is the name fixed by the pipeline design
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
