// Package llmhttp performs one HTTP POST for an LLM adapter: it applies the
// adapter's timeout over the caller's context, sends a request, reads the
// response body under a size cap, and classifies a send or read failure into a
// context error or one of the adapter's own sentinels. It returns the body of a
// 200 response without reading the body of a non-200 response, and it never
// follows a redirect. It defines no sentinel for a send failure, so each
// adapter keeps the sentinels that name its provider. It also checks the shape
// of an API key before an adapter is built.
package llmhttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/isseis/yt2column/internal/secret"
)

// MaxResponseBytes caps the response body an adapter reads. Post reads one
// byte past the limit to detect an oversized body without buffering it all.
const MaxResponseBytes = 8 << 20

// Static errors of the send path that name no provider, so no adapter exposes
// them. errAdapterTimeout is the cause of the deadline Post adds to the
// caller's context; errIncompleteCall reports a Call that Post refuses to send.
var (
	errAdapterTimeout = errors.New("adapter timeout")
	errIncompleteCall = errors.New("incomplete call")
)

// Construction errors for an API key, shared by every adapter that sends the
// key in a header. They name no provider; an adapter wraps them with its own
// prefix.
var (
	ErrZeroAPIKey    = errors.New("the API key is the zero value")
	ErrInvalidAPIKey = errors.New("the API key contains a character outside printable ASCII (0x21-0x7E)")
)

// CheckAPIKey validates the shape of an API key: it must be present and made
// only of printable ASCII, so it can be sent as a header value unchanged. The
// revealed string is discarded after the check and is never stored or put in
// an error.
func CheckAPIKey(apiKey secret.Secret) error {
	value, err := apiKey.Reveal()
	if err != nil {
		return ErrZeroAPIKey
	}
	for i := range len(value) {
		if value[i] < '!' || value[i] > '~' {
			return ErrInvalidAPIKey
		}
	}
	return nil
}

// NewClient returns an http.Client that never follows a redirect and leaves
// Transport nil, so the proxy environment variables apply.
func NewClient() *http.Client {
	return &http.Client{
		// A 3xx response is returned as the response instead of being
		// followed. Post enforces the same policy whatever Client it is given.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Errors are the adapter's own error values. Post reports every failure with
// one of them, so each adapter keeps its sentinels.
type Errors struct {
	Transport       error
	InvalidResponse error
	Status          func(statusCode int) error
}

// Call describes one POST. NewRequest must build the request with the ctx it
// receives, which carries the call deadline.
type Call struct {
	Client     *http.Client
	Timeout    time.Duration
	NewRequest func(ctx context.Context) (*http.Request, error)
	Errors     Errors
}

// Post sends one request and returns the body of a 200 response, at most
// MaxResponseBytes long. It never follows a redirect, whatever Client is.
// A non-200 response returns Errors.Status(code) without reading the body; a
// timeout or cancellation returns an error matching context.DeadlineExceeded
// or context.Canceled; any other send or read failure wraps Errors.Transport;
// an oversized body wraps Errors.InvalidResponse.
func Post(ctx context.Context, call Call) ([]byte, error) {
	if err := checkCall(call); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// The single deadline Post adds covers the send and the whole response
	// body read. http.Client.Timeout is deliberately not used: it would make
	// a timeout indistinguishable from other transport failures.
	callCtx, cancel := context.WithTimeoutCause(ctx, call.Timeout, errAdapterTimeout)
	defer cancel()

	request, err := call.NewRequest(callCtx)
	if err != nil {
		return nil, err
	}
	// The deadline must be on the request itself, so NewRequest has to use
	// the context Post passes rather than one of its own. A nil request or a
	// different context is treated as an incomplete Call.
	if request == nil || request.Context() != callCtx {
		return nil, errIncompleteCall
	}
	// start marks when the send begins, so a timeout or transport failure
	// can report how long the attempt lasted.
	start := time.Now()
	response, err := newNoRedirectClient(call.Client).Do(request)
	if err != nil {
		return nil, classifyFailure(callCtx, call, start, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		// The body of a non-200 response is never read: it is untrusted
		// text that can even contain a fragment of the API key.
		return nil, call.Errors.Status(response.StatusCode)
	}
	data, err := readResponseBody(response.Body, call.Errors.InvalidResponse)
	if err != nil {
		return nil, classifyFailure(callCtx, call, start, err)
	}
	return data, nil
}

// checkCall rejects a Call that is incomplete. Timeout must be positive,
// NewRequest must be set, and every Errors field must be non-nil: a missing
// one would turn a failure into a nil error or a panic.
func checkCall(call Call) error {
	if call.Timeout <= 0 || call.NewRequest == nil ||
		call.Errors.Transport == nil || call.Errors.InvalidResponse == nil || call.Errors.Status == nil {
		return errIncompleteCall
	}
	return nil
}

// newNoRedirectClient returns a client that does not follow redirects, based
// on the given one. A nil client is replaced by a zero client, which uses the
// default transport and so honours the proxy environment variables. The
// client is copied rather than mutated, so a caller can share one across
// concurrent calls.
func newNoRedirectClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}

// classifyFailure maps a send or body-read failure to its sentinel. The call
// context is checked first, so a timeout or cancellation is reported as such
// even when the transport returns another error. A transport cause is never
// %w-chained: the returned error must match exactly one sentinel, and the
// cause could itself contain a context error. The elapsed time tells a
// connection that never opened apart from one that dropped long into the read.
func classifyFailure(callCtx context.Context, call Call, start time.Time, cause error) error {
	if err := callCtx.Err(); err != nil {
		return contextFailure(callCtx, call.Timeout, start)
	}
	if errors.Is(cause, call.Errors.InvalidResponse) {
		return cause
	}
	return fmt.Errorf("%w: %v (%d ms elapsed)", call.Errors.Transport, cause, time.Since(start).Milliseconds())
}

// contextFailure describes where the deadline or cancellation came from. The
// adapter's own timeout includes its value and the elapsed time; the caller's
// deadline or cancellation is named as such, without either. The wrapped error
// is the call context's error, so errors.Is matches context.DeadlineExceeded
// or context.Canceled. The caller must have checked that callCtx.Err() is
// non-nil.
func contextFailure(callCtx context.Context, timeout time.Duration, start time.Time) error {
	err := callCtx.Err()
	if errors.Is(context.Cause(callCtx), errAdapterTimeout) {
		return fmt.Errorf("timed out after %s (%d ms elapsed): %w", timeout, time.Since(start).Milliseconds(), err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("caller context deadline exceeded: %w", err)
	}
	return fmt.Errorf("caller context canceled: %w", err)
}

// readResponseBody reads at most MaxResponseBytes+1 bytes. One byte past the
// limit is enough to detect an oversized body without buffering it all.
func readResponseBody(body io.Reader, invalidResponse error) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxResponseBytes {
		return nil, fmt.Errorf("%w: response body exceeds the %d-byte limit", invalidResponse, MaxResponseBytes)
	}
	return data, nil
}
