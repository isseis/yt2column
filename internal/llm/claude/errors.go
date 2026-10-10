package claude

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for failures that depend on the HTTP and JSON shape of the
// Messages API. Generate wraps them with context so callers can identify a
// failure with errors.Is.
var (
	ErrHTTPStatus      = errors.New("unexpected HTTP status")
	ErrInvalidResponse = errors.New("invalid response body")
	ErrTransport       = errors.New("transport failure")
)

// Static errors for constructing a client and for failure paths that have no
// public sentinel. The construction errors are unexported: the requirements
// ask only that a rejected construction fail, and no production code inspects
// which reason it was.
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

// statusOverloaded is the Messages API's 529 status, which has no net/http
// constant. It means the API is overloaded.
const statusOverloaded = 529

// statusGuidance returns fixed, status-specific guidance that helps a caller
// tell a configuration error apart from an account or server-side problem. It
// never names an environment variable or the response body.
func statusGuidance(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "check the model name, the effort, max_tokens, the prompt length, and the workspace id (a key not bound to a workspace requires anthropic-workspace-id)"
	case http.StatusUnauthorized:
		return "check the API key"
	case http.StatusPaymentRequired:
		return "check the credit balance and billing settings"
	case http.StatusForbidden:
		return "check the key permissions and the workspace"
	case http.StatusNotFound:
		return "check the model name and whether the organization may use the model"
	case http.StatusRequestEntityTooLarge:
		return "the request is too large"
	case http.StatusTooManyRequests:
		return "the account is rate limited; retry later"
	case http.StatusInternalServerError, http.StatusServiceUnavailable, statusOverloaded:
		return "the API server reported a problem; retry later"
	default:
		return "see the Anthropic API documentation"
	}
}
