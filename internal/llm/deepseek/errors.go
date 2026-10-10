package deepseek

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for failures that depend on the HTTP and JSON shape of the
// DeepSeek API. Generate wraps them with context so callers can identify a
// failure with errors.Is.
var (
	ErrHTTPStatus      = errors.New("unexpected HTTP status")
	ErrInvalidResponse = errors.New("invalid response body")
	ErrTransport       = errors.New("transport failure")
)

// ErrPaddedModel reports that the model name has leading or trailing
// whitespace. New returns it so a caller can tell this rejection apart from
// the other invalid model names with errors.Is.
var ErrPaddedModel = errors.New("the model has leading or trailing whitespace")

// Static errors for constructing a client and for failure paths that have no
// public sentinel.
var (
	errZeroAPIKey         = errors.New("the API key is the zero value")
	errInvalidAPIKey      = errors.New("the API key contains a character outside printable ASCII (0x21-0x7E)")
	errEmptyModel         = errors.New("the model is empty")
	errInvalidModel       = errors.New("the model is not valid UTF-8")
	errNonPositiveTimeout = errors.New("the timeout must be positive")
	errRevealAPIKey       = errors.New("the API key cannot be revealed")
	errChoiceCount        = errors.New("choices must have exactly one element")
)

// HTTPStatusError reports a response with a status other than 200. It holds
// only the status code: never the response body, the request, or its headers.
type HTTPStatusError struct {
	StatusCode int
}

// Error implements the error interface. The message names the status code and
// gives fixed guidance for it, never the response body.
func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("%s: %d: %s", ErrHTTPStatus, e.StatusCode, statusGuidance(e.StatusCode))
}

// Unwrap returns ErrHTTPStatus.
func (e *HTTPStatusError) Unwrap() error {
	return ErrHTTPStatus
}

// statusGuidance returns fixed, status-specific guidance that helps a caller
// tell a configuration error apart from an account or server-side problem.
func statusGuidance(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "check the model name, max_tokens, and prompt length"
	case http.StatusUnauthorized:
		return "check the API key"
	case http.StatusPaymentRequired:
		return "check the account balance"
	case http.StatusTooManyRequests:
		return "the account is rate limited; retry later"
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		return "the API server reported a problem; retry later"
	default:
		return "see the DeepSeek API documentation"
	}
}
