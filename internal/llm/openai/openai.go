// Package openai implements llm.LLMClient against the OpenAI Responses API.
// It sends one non-streaming POST request per Generate call and validates the
// response body strictly before returning any of it.
package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/llmhttp"
	"github.com/isseis/yt2column/internal/llm/openaiparam"
	"github.com/isseis/yt2column/internal/secret"
)

// maxResponseBytes is the response body cap that llmhttp.Post enforces; the
// tests refer to it by this name.
const maxResponseBytes = llmhttp.MaxResponseBytes

// errorPrefix is added once by New and Generate to every error message.
const errorPrefix = "openai: "

// Options configures the OpenAI adapter. Every field is required.
type Options struct {
	APIKey  secret.Secret
	Model   string
	Effort  openaiparam.Effort
	Timeout time.Duration
}

// client is the llm.LLMClient implementation returned by New. Its fields are
// set once by New and never mutated afterwards, so Generate is safe for
// concurrent use.
type client struct {
	apiKey     secret.Secret
	model      string
	effort     openaiparam.Effort
	timeout    time.Duration
	endpoint   string
	httpClient *http.Client
}

// New validates opts and returns an llm.LLMClient that sends to the
// production endpoint. It never reads environment variables. The concrete
// type is unexported, so a client can only be obtained through New. The
// model ID is kept as given: it is opaque, and a model or effort the API does
// not support is reported by the API.
func New(opts Options) (llm.LLMClient, error) {
	if err := llmhttp.CheckAPIKey(opts.APIKey); err != nil {
		return nil, wrapError(err)
	}
	if opts.Model == "" {
		return nil, wrapError(errEmptyModel)
	}
	if strings.TrimSpace(opts.Model) != opts.Model {
		return nil, wrapError(errPaddedModel)
	}
	if !utf8.ValidString(opts.Model) {
		return nil, wrapError(errInvalidModel)
	}
	if !opts.Effort.Valid() {
		return nil, wrapError(errInvalidEffort)
	}
	if opts.Timeout <= 0 {
		return nil, wrapError(errNonPositiveTimeout)
	}
	return &client{
		apiKey:     opts.APIKey,
		model:      opts.Model,
		effort:     opts.Effort,
		timeout:    opts.Timeout,
		endpoint:   endpoint,
		httpClient: llmhttp.NewClient(),
	}, nil
}

// wrapError adds the single "openai: " prefix that every error message
// carries.
func wrapError(err error) error {
	return fmt.Errorf("%s%w", errorPrefix, err)
}

// Generate sends req to the Responses API and returns the generated text. It
// reports a failure with the sentinels of internal/llm and of this package,
// and a timeout or cancellation with an error that matches
// context.DeadlineExceeded or context.Canceled. For a truncation, an
// unexpected finish, or an empty response, the returned response carries the
// rejected response's Model and Usage.
func (c *client) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	response, err := c.generate(ctx, req)
	if err != nil {
		return response, wrapError(err)
	}
	return response, nil
}

// generate is Generate before the one-time error prefix is added, so every
// failure path can return the sentinels unwrapped by the prefix.
func (c *client) generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	if err := req.Validate(); err != nil {
		return llm.GenerateResponse{}, err
	}
	body, err := newRequestBody(c.model, c.effort, req)
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	maxTokens := effectiveMaxTokens(req)
	data, err := llmhttp.Post(ctx, llmhttp.Call{
		Client:  c.httpClient,
		Timeout: c.timeout,
		NewRequest: func(callCtx context.Context) (*http.Request, error) {
			return c.newRequest(callCtx, body)
		},
		Errors: llmhttp.Errors{
			Transport:       ErrTransport,
			InvalidResponse: ErrInvalidResponse,
			Status: func(status int) error {
				return c.statusError(status, maxTokens)
			},
		},
	})
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	return parseResponse(data, requestSummary{maxTokens: maxTokens, effort: c.effort})
}

// statusError builds the HTTP status failure. The model name, the
// max_output_tokens that was sent, and the effort help the caller match the
// fixed guidance to the configuration; none of them is a secret.
func (c *client) statusError(status, maxTokens int) error {
	return fmt.Errorf("model %q, max_output_tokens %d, effort %s: %w",
		c.model, maxTokens, c.effort, &HTTPStatusError{StatusCode: status})
}
