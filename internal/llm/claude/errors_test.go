//go:build test

package claude

import (
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
)

// TestGenerateAuthRejected checks that only a 401 response matches
// llm.ErrAuthRejected, and that every status keeps matching ErrHTTPStatus and
// keeps its status code reachable with errors.AsType.
func TestGenerateAuthRejected(t *testing.T) {
	cases := []struct {
		status       int
		wantRejected bool
	}{
		{http.StatusUnauthorized, true},
		{http.StatusBadRequest, false},
		{http.StatusForbidden, false},
		{http.StatusTooManyRequests, false},
		{http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			server := llmhttptest.NewResponseServer(t, tc.status, []byte("{}"))
			_, err := generateAgainst(t, server.URL, validRequest(), nil)
			if got := errors.Is(err, llm.ErrAuthRejected); got != tc.wantRejected {
				t.Errorf("errors.Is(err, llm.ErrAuthRejected) = %t, want %t (error: %v)", got, tc.wantRejected, err)
			}
			if !errors.Is(err, ErrHTTPStatus) {
				t.Errorf("error = %v, want it to match ErrHTTPStatus", err)
			}
			statusErr, ok := errors.AsType[*HTTPStatusError](err)
			if !ok || statusErr.StatusCode != tc.status {
				t.Errorf("errors.AsType[*HTTPStatusError] = %+v, %t, want status %d", statusErr, ok, tc.status)
			}
		})
	}
}
