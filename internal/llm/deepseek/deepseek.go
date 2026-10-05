// Package deepseek implements llm.LLMClient against the DeepSeek Chat
// Completions API. It sends one POST request per Generate call and validates
// the response body strictly before returning any of it.
package deepseek

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/secret"
)

const (
	// endpoint is the fixed production destination. It is not configurable:
	// only the test helper (built with the test tag) replaces it, and only
	// with a loopback URL.
	endpoint = "https://api.deepseek.com/chat/completions"
	// maxResponseBytes caps the response body. The adapter reads one byte
	// past the limit to detect an oversized body without buffering it all.
	maxResponseBytes = 8 << 20
	// errorPrefix is added once by New and Generate to every error message.
	errorPrefix = "deepseek: "
)

// Options configures the DeepSeek adapter. All fields are required.
type Options struct {
	APIKey  secret.Secret
	Model   string
	Timeout time.Duration
}

// client is the llm.LLMClient implementation returned by New. Its fields are
// set once by New and never mutated afterwards, so Generate is safe for
// concurrent use.
type client struct {
	apiKey     secret.Secret
	model      string
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
	if opts.Timeout <= 0 {
		return nil, wrapError(errNonPositiveTimeout)
	}
	return &client{
		apiKey:   opts.APIKey,
		model:    opts.Model,
		timeout:  opts.Timeout,
		endpoint: endpoint,
		httpClient: &http.Client{
			// A 3xx response is returned as the response instead of being
			// followed, so the API key is never sent to a redirect target.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
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
// matches context.DeadlineExceeded or context.Canceled.
func (c *client) Generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	response, err := c.generate(ctx, req)
	if err != nil {
		return llm.GenerateResponse{}, wrapError(err)
	}
	return response, nil
}

// generate is Generate before the one-time error prefix is added, so every
// failure path can return the sentinels unwrapped by the prefix.
func (c *client) generate(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	if err := req.Validate(); err != nil {
		return llm.GenerateResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return llm.GenerateResponse{}, err
	}

	// The single deadline set by the adapter covers the send and the whole
	// response body read. http.Client.Timeout is deliberately not used:
	// it would make a timeout indistinguishable from other transport
	// failures.
	callCtx, cancel := context.WithTimeoutCause(ctx, c.timeout, errAdapterTimeout)
	defer cancel()

	body, err := newChatRequestBody(c.model, req)
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	request, err := c.newRequest(callCtx, body)
	if err != nil {
		return llm.GenerateResponse{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return llm.GenerateResponse{}, c.classifyFailure(callCtx, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		// The body of a non-200 response is never read: it is untrusted
		// text that can even contain a fragment of the API key.
		return llm.GenerateResponse{}, c.statusError(response.StatusCode, req.MaxOutputTokens)
	}
	data, err := readResponseBody(response.Body)
	if err != nil {
		return llm.GenerateResponse{}, c.classifyFailure(callCtx, err)
	}
	return parseResponse(data)
}

// classifyFailure maps a send or body-read failure to its sentinel. The call
// context is checked first, so a timeout or cancellation is reported as such
// even when the transport returns another error. A transport cause is never
// %w-chained: the returned error must match exactly one sentinel, and the
// cause could itself contain a context error.
func (c *client) classifyFailure(callCtx context.Context, cause error) error {
	if err := callCtx.Err(); err != nil {
		return contextFailure(callCtx, c.timeout)
	}
	if errors.Is(cause, ErrInvalidResponse) {
		return cause
	}
	return fmt.Errorf("%w: %v", ErrTransport, cause)
}

// contextFailure describes where the deadline or cancellation came from. The
// adapter's own timeout includes its value; the caller's deadline or
// cancellation is named as such. The wrapped error is the call context's
// error, so errors.Is matches context.DeadlineExceeded or context.Canceled.
// The caller must have checked that callCtx.Err() is non-nil.
func contextFailure(callCtx context.Context, timeout time.Duration) error {
	err := callCtx.Err()
	if errors.Is(context.Cause(callCtx), errAdapterTimeout) {
		return fmt.Errorf("timed out after %s: %w", timeout, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("caller context deadline exceeded: %w", err)
	}
	return fmt.Errorf("caller context canceled: %w", err)
}

// statusError builds the HTTP status failure. The model name and the
// max_tokens value that was sent (or that none was sent) help the caller
// match the fixed guidance to the configuration; neither is a secret.
func (c *client) statusError(status, maxTokens int) error {
	limit := "not sent"
	if maxTokens > 0 {
		limit = strconv.Itoa(maxTokens)
	}
	return fmt.Errorf("model %q, max_tokens %s: %w", c.model, limit, &HTTPStatusError{StatusCode: status})
}
