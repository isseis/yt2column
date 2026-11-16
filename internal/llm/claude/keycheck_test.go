//go:build test

package claude

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/llmhttp"
	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
	"github.com/isseis/yt2column/internal/secret"
)

// checkKeyAgainst runs the key check against a loopback endpoint with the test
// key and the given options applied.
func checkKeyAgainst(t *testing.T, endpoint string, modify func(*KeyCheckOptions)) error {
	t.Helper()
	opts := KeyCheckOptions{APIKey: mustSecret(t, testAPIKey), Timeout: testClientTimeout}
	if modify != nil {
		modify(&opts)
	}
	return CheckKeyForLoopbackTest(context.Background(), t, opts, endpoint)
}

func TestCheckKeyStatus(t *testing.T) {
	const marker = "RESPONSE-BODY-MARKER"
	cases := []struct {
		status       int
		wantRejected bool
	}{
		{http.StatusOK, false},
		{http.StatusUnauthorized, true},
		{http.StatusBadRequest, false},
		{http.StatusForbidden, false},
		{http.StatusTooManyRequests, false},
		{http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			server := llmhttptest.NewResponseServer(t, tc.status, []byte("{ not json "+marker))
			err := checkKeyAgainst(t, server.URL, nil)
			if tc.status == http.StatusOK {
				if err != nil {
					t.Fatalf("CheckKey() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("CheckKey() error = nil, want an error")
			}
			if got := errors.Is(err, llm.ErrAuthRejected); got != tc.wantRejected {
				t.Errorf("errors.Is(err, llm.ErrAuthRejected) = %t, want %t (error: %v)", got, tc.wantRejected, err)
			}
			statusErr, ok := errors.AsType[*HTTPStatusError](err)
			if !ok || statusErr.StatusCode != tc.status {
				t.Errorf("errors.AsType[*HTTPStatusError] = %+v, %t, want status %d", statusErr, ok, tc.status)
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("error %q contains the response body", err)
			}
			assertErrorPrefix(t, err)
			assertNoAPIKey(t, err)
		})
	}
}

func TestCheckKeyDoesNotReadTheBody(t *testing.T) {
	// The server answers 200 and then never finishes the body. A check that
	// read the body, as llmhttp.Post does for a 200, would run into its own
	// deadline instead of returning nil as soon as the status arrives.
	server := llmhttptest.NewPartialStallingServer(t)
	start := time.Now()
	err := checkKeyAgainst(t, server.URL, func(opts *KeyCheckOptions) { opts.Timeout = testClientTimeout * 2 })
	if err != nil {
		t.Errorf("CheckKey() error = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed >= testClientTimeout {
		t.Errorf("CheckKey() took %v; it must return without reading the body", elapsed)
	}
}

func TestCheckKeyNotRejected(t *testing.T) {
	// shortTimeout makes the stalling server hit the check's own deadline.
	const shortTimeout = 100 * time.Millisecond
	// A key check that could not be made is not a rejected key: neither a
	// send failure nor a timeout matches llm.ErrAuthRejected.
	t.Run("send failure", func(t *testing.T) {
		err := checkKeyAgainst(t, llmhttptest.NewClosedListener(t), nil)
		if !errors.Is(err, ErrTransport) {
			t.Errorf("CheckKey() error = %v, want ErrTransport", err)
		}
		if errors.Is(err, llm.ErrAuthRejected) {
			t.Errorf("CheckKey() error = %v, must not match llm.ErrAuthRejected", err)
		}
		assertErrorPrefix(t, err)
	})
	t.Run("timeout", func(t *testing.T) {
		server := llmhttptest.NewStallingServer(t)
		err := checkKeyAgainst(t, server.URL, func(opts *KeyCheckOptions) { opts.Timeout = shortTimeout })
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("CheckKey() error = %v, want context.DeadlineExceeded", err)
		}
		if errors.Is(err, llm.ErrAuthRejected) {
			t.Errorf("CheckKey() error = %v, must not match llm.ErrAuthRejected", err)
		}
		assertErrorPrefix(t, err)
	})
}

func TestCheckKeyRejectsInvalidOptions(t *testing.T) {
	server, recorder := llmhttptest.NewRecordingServer(t, nil)
	// want names the layer that rejects: CheckKey's own check, not the
	// llmhttp.Status check behind it, which would also stop a bad timeout.
	cases := map[string]struct {
		modify func(*KeyCheckOptions)
		want   error
	}{
		"zero API key": {
			func(opts *KeyCheckOptions) { opts.APIKey = secret.Secret{} }, llmhttp.ErrZeroAPIKey,
		},
		"API key outside printable ASCII": {
			func(opts *KeyCheckOptions) { opts.APIKey = mustSecret(t, "bad key") }, llmhttp.ErrInvalidAPIKey,
		},
		"zero timeout": {
			func(opts *KeyCheckOptions) { opts.Timeout = 0 }, errNonPositiveTimeout,
		},
		"negative timeout": {
			func(opts *KeyCheckOptions) { opts.Timeout = -time.Second }, errNonPositiveTimeout,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := checkKeyAgainst(t, server.URL, tc.modify)
			if !errors.Is(err, tc.want) {
				t.Fatalf("CheckKey() error = %v, want %v", err, tc.want)
			}
			if errors.Is(err, llm.ErrAuthRejected) {
				t.Errorf("CheckKey() error = %v, must not match llm.ErrAuthRejected", err)
			}
			assertErrorPrefix(t, err)
			if got := recorder.Count(); got != 0 {
				t.Errorf("%d request(s) were sent, want none", got)
			}
		})
	}
}

func TestCheckKeySendsRequest(t *testing.T) {
	cases := []struct {
		name        string
		workspaceID string
	}{
		{"without a workspace ID", ""},
		{"with a workspace ID", testWorkspaceID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, recorder := llmhttptest.NewRecordingServer(t, nil)
			err := checkKeyAgainst(t, server.URL, func(opts *KeyCheckOptions) {
				if tc.workspaceID != "" {
					opts.WorkspaceID = mustWorkspaceID(t, tc.workspaceID)
				}
			})
			if err != nil {
				t.Fatalf("CheckKey() error = %v", err)
			}
			recorded := recorder.Only(t)
			if recorded.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", recorded.Method, http.MethodGet)
			}
			if got := recorded.Header.Get(headerAPIKey); got != testAPIKey {
				t.Errorf("x-api-key = %q, want the test key", got)
			}
			if got := recorded.Header.Get(headerVersion); got != anthropicVersion {
				t.Errorf("anthropic-version = %q, want %q", got, anthropicVersion)
			}
			got, sent := recorded.Header[http.CanonicalHeaderKey(headerWorkspace)]
			if tc.workspaceID == "" && sent {
				t.Errorf("anthropic-workspace-id = %q, want it unset", got)
			}
			if tc.workspaceID != "" && (len(got) != 1 || got[0] != tc.workspaceID) {
				t.Errorf("anthropic-workspace-id = %q, want [%q]", got, tc.workspaceID)
			}
			if got := recorded.Header.Get("Authorization"); got != "" {
				t.Errorf("Authorization = %q, want it unset", got)
			}
			if got := recorded.Header.Get("Content-Type"); got != "" {
				t.Errorf("Content-Type = %q, want it unset: the request has no body", got)
			}
			if len(recorded.Body) != 0 {
				t.Errorf("body = %q, want it empty", recorded.Body)
			}
			// The key travels only in x-api-key, never in another header or the URL.
			for name, values := range recorded.Header {
				if name == http.CanonicalHeaderKey(headerAPIKey) {
					continue
				}
				if slices.ContainsFunc(values, func(v string) bool { return strings.Contains(v, testAPIKey) }) {
					t.Errorf("header %s carries the API key", name)
				}
			}
			if strings.Contains(recorded.Target, testAPIKey) {
				t.Error("the request URI carries the API key")
			}
		})
	}
}

func TestCheckKeyUsesProxyForProductionEndpoint(t *testing.T) {
	// The test proxy that TestMain installed closes every connection, so a
	// CheckKey that targets a non-loopback address fails with a send failure
	// and the proxy sees the attempt. Nothing reaches the network.
	before := testProxy.Accepted()
	err := CheckKey(context.Background(), KeyCheckOptions{
		APIKey:  mustSecret(t, testAPIKey),
		Timeout: testClientTimeout,
	})
	if !errors.Is(err, ErrTransport) {
		t.Errorf("CheckKey() error = %v, want ErrTransport", err)
	}
	if testProxy.Accepted() <= before {
		t.Error("the blackhole listener accepted no connection; the check did not use the test proxy")
	}
}
