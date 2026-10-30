// Package deepseek implements llm.LLMClient against the DeepSeek Chat
// Completions API. It sends one POST request per Generate call and validates
// the response body strictly before returning any of it.
package deepseek

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/deepseekparam"
	"github.com/isseis/yt2column/internal/llm/llmhttp"
	"github.com/isseis/yt2column/internal/secret"
)

const (
	// endpoint is the fixed production destination. It is not configurable:
	// only the test helper (built with the test tag) replaces it, and only
	// with a loopback URL.
	endpoint = "https://api.deepseek.com/chat/completions"
	// maxResponseBytes is the response body cap that llmhttp.Post enforces;
	// the tests refer to it by this name.
	maxResponseBytes = llmhttp.MaxResponseBytes
	// errorPrefix is added once by New and Generate to every error message.
	errorPrefix = "deepseek: "
)

// Options configures the DeepSeek adapter. APIKey, Model, and Timeout are
// required. The zero Effort sends no reasoning_effort member; any other value
// must be one deepseekparam accepts.
type Options struct {
	APIKey  secret.Secret
	Model   string
	Effort  deepseekparam.Effort
	Timeout time.Duration
}

// client is the llm.LLMClient implementation returned by New. Its fields are
// set once by New and never mutated afterwards, so Generate is safe for
// concurrent use.
type client struct {
	apiKey     secret.Secret
	model      string
	effort     deepseekparam.Effort
	timeout    time.Duration
	endpoint   string
	httpClient *http.Client
}

// New validates opts and returns an llm.LLMClient that sends to the
// production endpoint. It never reads environment variables. The concrete
// type is unexported, so a client can only be obtained through New.
func New(opts Options) (llm.LLMClient, error) {
	if err := checkAPIKey(opts.APIKey); err != nil {
		return nil, wrapError(err)
	}
	if opts.Model == "" {
		return nil, wrapError(errEmptyModel)
	}
	if strings.TrimSpace(opts.Model) != opts.Model {
		return nil, wrapError(ErrPaddedModel)
	}
	if !utf8.ValidString(opts.Model) {
		return nil, wrapError(errInvalidModel)
	}
	if opts.Effort != deepseekparam.EffortUnspecified && !opts.Effort.Valid() {
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

// wrapError adds the single "deepseek: " prefix that every error message
// carries.
func wrapError(err error) error {
	return fmt.Errorf("%s%w", errorPrefix, err)
}

// Generate sends req to the DeepSeek Chat Completions API and returns the
// generated text. It reports a failure with the sentinels of internal/llm
// and of this package, and a timeout or cancellation with an error that
// matches context.DeadlineExceeded or context.Canceled. For a truncation, an
// unexpected finish reason, or an empty response, the returned response
// carries the rejected response's Model and Usage.
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
	body, err := newChatRequestBody(c.model, req, c.effort)
	if err != nil {
		return llm.GenerateResponse{}, err
	}
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
				return c.statusError(status, req.MaxOutputTokens)
			},
		},
	})
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	return parseResponse(data, requestSummary{effort: c.effort})
}

// statusError builds the HTTP status failure. The model name, the max_tokens
// value that was sent (or that none was sent), and the reasoning_effort that
// was sent (or that none was sent) help the caller match the fixed guidance to
// the configuration; none of them is a secret.
func (c *client) statusError(status, maxTokens int) error {
	limit := "not sent"
	if maxTokens > 0 {
		limit = strconv.Itoa(maxTokens)
	}
	return fmt.Errorf("model %q, max_tokens %s, reasoning_effort %s: %w",
		c.model, limit, effortSummary(c.effort), &HTTPStatusError{StatusCode: status})
}

// effortSummary renders the reasoning_effort a request carries: the wire value
// when one was specified, "not sent" otherwise. The alternative is unreachable
// because New and the request body accept the same values, and it reads as
// "not sent" rather than inventing a value the request does not carry.
func effortSummary(effort deepseekparam.Effort) string {
	if effort == deepseekparam.EffortUnspecified {
		return "not sent"
	}
	return effort.String()
}
