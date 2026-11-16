//go:build test

package claude

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
)

// TestHTTPStatusErrorAuthRejected checks that only a 401 matches
// llm.ErrAuthRejected, through wrapping as the adapter wraps it, and that
// every status keeps matching ErrHTTPStatus.
func TestHTTPStatusErrorAuthRejected(t *testing.T) {
	cases := []struct {
		status       int
		wantRejected bool
	}{
		{http.StatusUnauthorized, true},
		{http.StatusBadRequest, false},
		{http.StatusForbidden, false},
		{http.StatusTooManyRequests, false},
		{http.StatusInternalServerError, false},
		{statusOverloaded, false},
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			err := fmt.Errorf("claude: context: %w", &HTTPStatusError{StatusCode: tc.status})
			if got := errors.Is(err, llm.ErrAuthRejected); got != tc.wantRejected {
				t.Errorf("errors.Is(err, llm.ErrAuthRejected) = %t, want %t", got, tc.wantRejected)
			}
			if !errors.Is(err, ErrHTTPStatus) {
				t.Errorf("error = %v, want it to match ErrHTTPStatus", err)
			}
			// A 401 must not match an unrelated target.
			if errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, must not match context.Canceled", err)
			}
		})
	}
}

// TestGenerateAuthRejected checks the same through Generate, so the error
// Generate builds around HTTPStatusError keeps the match.
func TestGenerateAuthRejected(t *testing.T) {
	server := llmhttptest.NewResponseServer(t, http.StatusUnauthorized, []byte("{}"))
	_, err := generateAgainst(t, server.URL, validRequest(), nil)
	if !errors.Is(err, llm.ErrAuthRejected) {
		t.Errorf("Generate() error = %v, want it to match llm.ErrAuthRejected", err)
	}
	statusErr, ok := errors.AsType[*HTTPStatusError](err)
	if !ok || statusErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("errors.AsType[*HTTPStatusError] = %+v, %t, want status 401", statusErr, ok)
	}
}
