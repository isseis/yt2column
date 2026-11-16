package claude

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/isseis/yt2column/internal/llm/claudeparam"
	"github.com/isseis/yt2column/internal/llm/llmhttp"
	"github.com/isseis/yt2column/internal/secret"
)

// modelsEndpoint is the fixed production destination of the key check. Like
// endpoint, it is not configurable: only the test helper replaces it, and only
// with a loopback URL. The Models API does not process tokens, so the call
// costs nothing.
const modelsEndpoint = "https://api.anthropic.com/v1/models?limit=1"

// KeyCheckOptions configures CheckKey. APIKey and Timeout are required. The
// zero WorkspaceID sends no anthropic-workspace-id header.
type KeyCheckOptions struct {
	APIKey      secret.Secret
	WorkspaceID claudeparam.WorkspaceID
	Timeout     time.Duration
}

// CheckKey calls the Models API once with the key and the optional workspace
// ID. It returns nil for 200, an error matching llm.ErrAuthRejected for 401,
// and another error for any other outcome (another status, a send failure, or
// a timeout), so a caller can tell a rejected key from a check that could not
// be made. It never reads the response body: the body is untrusted text that
// can even contain a fragment of the API key.
func CheckKey(ctx context.Context, opts KeyCheckOptions) error {
	return checkKey(ctx, opts, modelsEndpoint)
}

// checkKey is CheckKey for an explicit endpoint, which only the test helper
// passes.
func checkKey(ctx context.Context, opts KeyCheckOptions, endpoint string) error {
	if err := llmhttp.CheckAPIKey(opts.APIKey); err != nil {
		return wrapError(err)
	}
	if opts.Timeout <= 0 {
		return wrapError(errNonPositiveTimeout)
	}
	status, err := llmhttp.Status(ctx, llmhttp.Call{
		Client:  llmhttp.NewClient(),
		Timeout: opts.Timeout,
		NewRequest: func(callCtx context.Context) (*http.Request, error) {
			return newKeyCheckRequest(callCtx, endpoint, opts)
		},
		// Status reports a send failure only; it never builds the other two.
		Errors: llmhttp.Errors{Transport: ErrTransport},
	})
	if err != nil {
		return wrapError(err)
	}
	if status != http.StatusOK {
		return wrapError(fmt.Errorf("check key: %w", &HTTPStatusError{StatusCode: status}))
	}
	return nil
}

// newKeyCheckRequest builds the GET request of the key check. It sets the
// headers through setRequestHeaders and adds no Content-Type: the request has
// no body.
func newKeyCheckRequest(ctx context.Context, endpoint string, opts KeyCheckOptions) (*http.Request, error) {
	//nolint:gosec // the endpoint is the fixed production URL or a loopback URL checked by the test helper
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if err := setRequestHeaders(request, opts.APIKey, opts.WorkspaceID); err != nil {
		return nil, err
	}
	return request, nil
}
