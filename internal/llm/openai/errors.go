package openai

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for failures that depend on the HTTP and JSON shape of the
// Responses API. Generate wraps them with context so callers can identify a
// failure with errors.Is.
var (
	ErrHTTPStatus      = errors.New("unexpected HTTP status")
	ErrInvalidResponse = errors.New("invalid response body")
	ErrTransport       = errors.New("transport failure")
)

// Static errors for constructing a client and for failure paths that have no
// public sentinel. The construction errors are unexported: a rejected
// construction only has to fail, and no production code inspects which
// reason it was.
var (
	errZeroAPIKey         = errors.New("the API key is the zero value")
	errInvalidAPIKey      = errors.New("the API key contains a character outside printable ASCII (0x21-0x7E)")
	errEmptyModel         = errors.New("the model is empty")
	errPaddedModel        = errors.New("the model has leading or trailing whitespace")
	errInvalidModel       = errors.New("the model is not valid UTF-8")
	errInvalidEffort      = errors.New("the effort is not one of the accepted values")
	errNonPositiveTimeout = errors.New("the timeout must be positive")
	errRevealAPIKey       = errors.New("the API key cannot be revealed")
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
// tell a configuration error apart from an account or server-side problem. It
// never names an environment variable or the response body.
func statusGuidance(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return "check the API key"
	case status == http.StatusForbidden:
		return "check the permissions of the API key and its project"
	case status == http.StatusBadRequest || status == http.StatusNotFound:
		return "check the model name, the effort, max_output_tokens, and the input"
	case status == http.StatusTooManyRequests:
		return "the usage limit or the billing settings were reached; check them and retry later"
	case status >= http.StatusInternalServerError && status <= lastServerErrorStatus:
		return "the API server reported a problem; retry later"
	default:
		return "see the OpenAI API documentation"
	}
}

// lastServerErrorStatus is the last status code of the 5xx class.
const lastServerErrorStatus = 599
