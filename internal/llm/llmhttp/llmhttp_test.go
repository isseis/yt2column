//go:build test

package llmhttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm/llmhttp/llmhttptest"
)

const (
	// testTimeout is the call timeout of a test that is not testing the
	// timeout itself.
	testTimeout = 2 * time.Second
	// testShortTimeout is the call timeout of a test that expects the
	// adapter's own deadline to fire.
	testShortTimeout = 100 * time.Millisecond
	// testGrace is the margin allowed for Post to return after a deadline.
	testGrace = 2 * time.Second
	// testStallTimeout is a call timeout long enough that returning well
	// before it proves the body of a non-200 response is not read.
	testStallTimeout = 3 * time.Second
)

// Test sentinels stand in for an adapter's own sentinels.
var (
	errTestTransport       = errors.New("test transport")
	errTestInvalidResponse = errors.New("test invalid response")
	errTestHTTPStatus      = errors.New("test http status")
)

// elapsedPattern matches the " (N ms elapsed)" suffix Post adds to its
// adapter-timeout and transport messages. It rejects a Duration.String()
// rendering such as "100ms" or "1.2s".
var elapsedPattern = regexp.MustCompile(` \(\d+ ms elapsed\)`)

// assertNamesElapsed checks that err names the elapsed time as an integer
// count of milliseconds.
func assertNamesElapsed(t *testing.T, err error) {
	t.Helper()
	if !elapsedPattern.MatchString(err.Error()) {
		t.Errorf("error %q does not name the elapsed time as an integer count of milliseconds", err)
	}
}

// httpStatusError is the stand-in for an adapter's HTTP status error. It
// carries the status code so a test can check that Post passed it through.
type httpStatusError struct {
	statusCode int
}

// Error implements the error interface.
func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s: %d", errTestHTTPStatus, e.statusCode)
}

// Unwrap returns the test sentinel.
func (e *httpStatusError) Unwrap() error {
	return errTestHTTPStatus
}

// testErrors returns the stand-in error set of an adapter.
func testErrors() Errors {
	return Errors{
		Transport:       errTestTransport,
		InvalidResponse: errTestInvalidResponse,
		Status:          func(statusCode int) error { return &httpStatusError{statusCode: statusCode} },
	}
}

// postRequest builds a NewRequest that sends a POST with a small body to
// serverURL using the ctx Post passes.
func postRequest(serverURL string) func(context.Context) (*http.Request, error) {
	return func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodPost, serverURL, bytes.NewReader([]byte("{}")))
	}
}

// assertSingleSentinel checks that err matches exactly one of the test
// sentinels and the two context errors.
func assertSingleSentinel(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want a sentinel error")
	}
	sentinels := []error{
		errTestTransport,
		errTestInvalidResponse,
		errTestHTTPStatus,
		context.DeadlineExceeded,
		context.Canceled,
	}
	matches := 0
	for _, sentinel := range sentinels {
		if errors.Is(err, sentinel) {
			matches++
		}
	}
	if matches != 1 {
		t.Errorf("error %v matches %d sentinels, want exactly 1", err, matches)
	}
}

// countingTransport counts RoundTrip calls and never sends.
type countingTransport struct {
	calls atomic.Int64
}

// RoundTrip implements http.RoundTripper.
func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return nil, errors.New("RoundTrip must not be called")
}

// sender is Post or Status behind one signature, so the checks of what the
// two share (the deadline, cancellation, and the classification of a send
// failure) run once over both. readsBody is true for the one that reads a
// response body, which only matters to a failure in the middle of a body.
type sender struct {
	name      string
	readsBody bool
	send      func(ctx context.Context, call Call) error
}

var senders = []sender{
	{"Post", true, func(ctx context.Context, call Call) error {
		_, err := Post(ctx, call)
		return err
	}},
	{"Status", false, func(ctx context.Context, call Call) error {
		_, err := Status(ctx, call)
		return err
	}},
}

func TestPostSuccess(t *testing.T) {
	server, recorder := llmhttptest.NewRecordingServer(t, []byte(`{"ok":true}`))
	data, err := Post(context.Background(), Call{
		Client:     NewClient(),
		Timeout:    testTimeout,
		NewRequest: postRequest(server.URL),
		Errors:     testErrors(),
	})
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("Post() body = %q, want the server body", data)
	}
	if got := recorder.Count(); got != 1 {
		t.Errorf("server received %d requests, want 1", got)
	}
	recorded := recorder.Only(t)
	if recorded.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", recorded.Method, http.MethodPost)
	}
	if recorded.Target != "/" {
		t.Errorf("request URI = %q, want %q", recorded.Target, "/")
	}
	if string(recorded.Body) != "{}" {
		t.Errorf("body = %q, want %q", recorded.Body, "{}")
	}
}

func TestPostNon200(t *testing.T) {
	const marker = "RESPONSE-BODY-MARKER"
	statuses := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusRequestEntityTooLarge,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		http.StatusTemporaryRedirect,
	}
	for _, status := range statuses {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := llmhttptest.NewResponseServer(t, status, []byte("{ not json "+marker))
			_, err := Post(context.Background(), Call{
				Client:     NewClient(),
				Timeout:    testTimeout,
				NewRequest: postRequest(server.URL),
				Errors:     testErrors(),
			})
			if !errors.Is(err, errTestHTTPStatus) {
				t.Errorf("Post() error = %v, want the status sentinel", err)
			}
			assertSingleSentinel(t, err)
			statusErr, ok := errors.AsType[*httpStatusError](err)
			if !ok {
				t.Fatalf("Post() error = %v, want *httpStatusError", err)
			}
			if statusErr.statusCode != status {
				t.Errorf("StatusCode = %d, want %d", statusErr.statusCode, status)
			}
			if errors.Is(err, errTestInvalidResponse) {
				t.Errorf("Post() error = %v, must not match the invalid-response sentinel", err)
			}
			if bytes.Contains([]byte(err.Error()), []byte(marker)) {
				t.Errorf("error %q contains the response body", err)
			}
		})
	}

	t.Run("does not read the body", func(t *testing.T) {
		server := llmhttptest.NewStatusStallingServer(t, http.StatusInternalServerError)
		start := time.Now()
		_, err := Post(context.Background(), Call{
			Client:     NewClient(),
			Timeout:    testStallTimeout,
			NewRequest: postRequest(server.URL),
			Errors:     testErrors(),
		})
		elapsed := time.Since(start)
		if !errors.Is(err, errTestHTTPStatus) {
			t.Errorf("Post() error = %v, want the status sentinel", err)
		}
		if elapsed >= testStallTimeout {
			t.Errorf("Post() took %v; it must return without reading the body", elapsed)
		}
	})
}

func TestSendTimeout(t *testing.T) {
	cases := []struct {
		name      string
		bodyOnly  bool
		newServer func(t *testing.T) *httptest.Server
	}{
		{"no response headers", false, llmhttptest.NewStallingServer},
		{"partial response body", true, llmhttptest.NewPartialStallingServer},
	}
	for _, snd := range senders {
		for _, tc := range cases {
			if tc.bodyOnly && !snd.readsBody {
				continue
			}
			t.Run(snd.name+"/"+tc.name, func(t *testing.T) {
				server := tc.newServer(t)
				start := time.Now()
				err := snd.send(context.Background(), Call{
					Client:     NewClient(),
					Timeout:    testShortTimeout,
					NewRequest: postRequest(server.URL),
					Errors:     testErrors(),
				})
				elapsed := time.Since(start)
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("%s() error = %v, want context.DeadlineExceeded", snd.name, err)
				}
				assertSingleSentinel(t, err)
				assertNamesElapsed(t, err)
				if elapsed > testShortTimeout+testGrace {
					t.Errorf("%s() took %v, want at most %v", snd.name, elapsed, testShortTimeout+testGrace)
				}
			})
		}
	}
}

func TestSendCallerDeadline(t *testing.T) {
	for _, snd := range senders {
		t.Run(snd.name, func(t *testing.T) {
			server := llmhttptest.NewStallingServer(t)
			ctx, cancel := context.WithTimeout(context.Background(), testShortTimeout)
			defer cancel()
			start := time.Now()
			err := snd.send(ctx, Call{
				Client:     NewClient(),
				Timeout:    testTimeout,
				NewRequest: postRequest(server.URL),
				Errors:     testErrors(),
			})
			elapsed := time.Since(start)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%s() error = %v, want context.DeadlineExceeded", snd.name, err)
			}
			assertSingleSentinel(t, err)
			if strings.Contains(err.Error(), "ms elapsed") {
				t.Errorf("error %q must not include the adapter's elapsed time", err)
			}
			if elapsed > testShortTimeout+testGrace {
				t.Errorf("%s() took %v, want at most %v", snd.name, elapsed, testShortTimeout+testGrace)
			}
		})
	}
}

func TestSendCanceled(t *testing.T) {
	for _, snd := range senders {
		t.Run(snd.name+"/cancel while running", func(t *testing.T) {
			server, entered := llmhttptest.NewGateServer(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var (
				err  error
				done = make(chan struct{})
			)
			go func() {
				err = snd.send(ctx, Call{
					Client:     NewClient(),
					Timeout:    testTimeout,
					NewRequest: postRequest(server.URL),
					Errors:     testErrors(),
				})
				close(done)
			}()
			<-entered
			cancel()
			<-done

			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s() error = %v, want context.Canceled", snd.name, err)
			}
			assertSingleSentinel(t, err)
			if strings.Contains(err.Error(), "ms elapsed") {
				t.Errorf("error %q must not include the adapter's elapsed time", err)
			}
		})

		t.Run(snd.name+"/pre-canceled context sends nothing", func(t *testing.T) {
			var newRequestCalls atomic.Int64
			transport := &countingTransport{}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := snd.send(ctx, Call{
				Client:  &http.Client{Transport: transport},
				Timeout: testTimeout,
				NewRequest: func(context.Context) (*http.Request, error) {
					newRequestCalls.Add(1)
					return nil, errors.New("NewRequest must not be called")
				},
				Errors: testErrors(),
			})
			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s() error = %v, want context.Canceled", snd.name, err)
			}
			if got := newRequestCalls.Load(); got != 0 {
				t.Errorf("NewRequest calls = %d, want 0", got)
			}
			if got := transport.calls.Load(); got != 0 {
				t.Errorf("RoundTrip calls = %d, want 0", got)
			}
		})
	}
}

func TestSendTransportFailure(t *testing.T) {
	for _, snd := range senders {
		t.Run(snd.name+"/listener closes the connection", func(t *testing.T) {
			err := snd.send(context.Background(), Call{
				Client:     NewClient(),
				Timeout:    testTimeout,
				NewRequest: postRequest(llmhttptest.NewClosedListener(t)),
				Errors:     testErrors(),
			})
			assertTransportFailure(t, err)
		})
	}

	// Only a send that reads the body can see a body cut short.
	t.Run("Post/body truncated before Content-Length", func(t *testing.T) {
		_, err := Post(context.Background(), Call{
			Client:     NewClient(),
			Timeout:    testTimeout,
			NewRequest: postRequest(llmhttptest.NewTruncatedResponseServer(t)),
			Errors:     testErrors(),
		})
		assertTransportFailure(t, err)
	})
}

// assertTransportFailure checks that err matches only the transport sentinel.
func assertTransportFailure(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errTestTransport) {
		t.Errorf("error = %v, want the transport sentinel", err)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, must not match a context error", err)
	}
	if errors.Is(err, errTestInvalidResponse) {
		t.Errorf("error = %v, must not match the invalid-response sentinel", err)
	}
	assertSingleSentinel(t, err)
	assertNamesElapsed(t, err)
}

func TestPostSizeLimit(t *testing.T) {
	t.Run("exactly the limit", func(t *testing.T) {
		body := bytes.Repeat([]byte("a"), MaxResponseBytes)
		server := llmhttptest.NewResponseServer(t, http.StatusOK, body)
		data, err := Post(context.Background(), Call{
			Client:     NewClient(),
			Timeout:    testTimeout,
			NewRequest: postRequest(server.URL),
			Errors:     testErrors(),
		})
		if err != nil {
			t.Fatalf("Post() error = %v, want nil", err)
		}
		if len(data) != MaxResponseBytes {
			t.Errorf("Post() body is %d bytes, want %d", len(data), MaxResponseBytes)
		}
	})

	t.Run("one byte over the limit", func(t *testing.T) {
		body := bytes.Repeat([]byte("a"), MaxResponseBytes+1)
		server := llmhttptest.NewResponseServer(t, http.StatusOK, body)
		_, err := Post(context.Background(), Call{
			Client:     NewClient(),
			Timeout:    testTimeout,
			NewRequest: postRequest(server.URL),
			Errors:     testErrors(),
		})
		if !errors.Is(err, errTestInvalidResponse) {
			t.Errorf("Post() error = %v, want the invalid-response sentinel", err)
		}
		assertSingleSentinel(t, err)
	})
}

func TestPostIncompleteCall(t *testing.T) {
	valid := func() Call {
		return Call{
			Client:     NewClient(),
			Timeout:    testTimeout,
			NewRequest: postRequest("http://127.0.0.1:1/"),
			Errors:     testErrors(),
		}
	}
	cases := map[string]func(call *Call){
		"zero timeout":     func(call *Call) { call.Timeout = 0 },
		"negative timeout": func(call *Call) { call.Timeout = -time.Second },
		"nil NewRequest":   func(call *Call) { call.NewRequest = nil },
		"nil Transport":    func(call *Call) { call.Errors.Transport = nil },
		"nil InvalidResponse": func(call *Call) {
			call.Errors.InvalidResponse = nil
		},
		"nil Status": func(call *Call) { call.Errors.Status = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			call := valid()
			mutate(&call)
			transport := &countingTransport{}
			call.Client = &http.Client{Transport: transport}
			_, err := Post(context.Background(), call)
			if !errors.Is(err, errIncompleteCall) {
				t.Errorf("Post() error = %v, want errIncompleteCall", err)
			}
			if errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Post() error = %v, must not match context.DeadlineExceeded", err)
			}
			if got := transport.calls.Load(); got != 0 {
				t.Errorf("RoundTrip calls = %d, want 0", got)
			}
		})
	}
	t.Run("NewRequest uses a different context", func(t *testing.T) {
		transport := &countingTransport{}
		var newRequestCalls atomic.Int64
		_, err := Post(context.Background(), Call{
			Client:  &http.Client{Transport: transport},
			Timeout: testTimeout,
			NewRequest: func(context.Context) (*http.Request, error) {
				newRequestCalls.Add(1)
				return http.NewRequestWithContext(context.Background(), http.MethodPost, "http://127.0.0.1:1/", nil)
			},
			Errors: testErrors(),
		})
		if !errors.Is(err, errIncompleteCall) {
			t.Errorf("Post() error = %v, want errIncompleteCall", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Post() error = %v, must not match context.DeadlineExceeded", err)
		}
		if got := newRequestCalls.Load(); got != 1 {
			t.Errorf("NewRequest calls = %d, want 1", got)
		}
		if got := transport.calls.Load(); got != 0 {
			t.Errorf("RoundTrip calls = %d, want 0", got)
		}
	})

	t.Run("NewRequest returns a nil request", func(t *testing.T) {
		transport := &countingTransport{}
		_, err := Post(context.Background(), Call{
			Client:  &http.Client{Transport: transport},
			Timeout: testTimeout,
			NewRequest: func(context.Context) (*http.Request, error) {
				return nil, nil
			},
			Errors: testErrors(),
		})
		if !errors.Is(err, errIncompleteCall) {
			t.Errorf("Post() error = %v, want errIncompleteCall", err)
		}
		if got := transport.calls.Load(); got != 0 {
			t.Errorf("RoundTrip calls = %d, want 0", got)
		}
	})
}

func TestPostRequestBuildError(t *testing.T) {
	transport := &countingTransport{}
	buildErr := errors.New("build request failed")
	_, err := Post(context.Background(), Call{
		Client:  &http.Client{Transport: transport},
		Timeout: testTimeout,
		NewRequest: func(context.Context) (*http.Request, error) {
			return nil, buildErr
		},
		Errors: testErrors(),
	})
	if !errors.Is(err, buildErr) {
		t.Errorf("Post() error = %v, want the NewRequest error", err)
	}
	if got := transport.calls.Load(); got != 0 {
		t.Errorf("RoundTrip calls = %d, want 0", got)
	}
}

func TestPostNoRedirect(t *testing.T) {
	var hits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/primary", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/secondary", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/secondary", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cases := map[string]*http.Client{
		"client that follows redirects": {},
		"nil client":                    nil,
	}
	for name, client := range cases {
		t.Run(name, func(t *testing.T) {
			before := hits.Load()
			_, err := Post(context.Background(), Call{
				Client:     client,
				Timeout:    testTimeout,
				NewRequest: postRequest(server.URL + "/primary"),
				Errors:     testErrors(),
			})
			if !errors.Is(err, errTestHTTPStatus) {
				t.Errorf("Post() error = %v, want the status sentinel", err)
			}
			statusErr, ok := errors.AsType[*httpStatusError](err)
			if !ok {
				t.Fatalf("Post() error = %v, want *httpStatusError", err)
			}
			if statusErr.statusCode != http.StatusTemporaryRedirect {
				t.Errorf("StatusCode = %d, want %d", statusErr.statusCode, http.StatusTemporaryRedirect)
			}
			if hits.Load() != before {
				t.Errorf("the redirect target was requested; Post must not follow redirects")
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client.Transport != nil {
		t.Error("NewClient().Transport is not nil")
	}
	if client.CheckRedirect == nil {
		t.Fatal("NewClient().CheckRedirect is nil")
	}
	if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Errorf("CheckRedirect() = %v, want http.ErrUseLastResponse", err)
	}
}

// trackingTransport answers every request with a fixed status and a body that
// records whether it was read and whether it was closed.
type trackingTransport struct {
	status int
	body   *trackingBody
}

// RoundTrip implements http.RoundTripper.
func (tr *trackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: tr.status,
		Header:     http.Header{},
		Body:       tr.body,
		Request:    req,
	}, nil
}

// trackingBody is a response body that notes the first Read and the first
// Close.
type trackingBody struct {
	read   atomic.Bool
	closed atomic.Bool
}

// Read implements io.Reader.
func (b *trackingBody) Read([]byte) (int, error) {
	b.read.Store(true)
	return 0, io.EOF
}

// Close implements io.Closer.
func (b *trackingBody) Close() error {
	b.closed.Store(true)
	return nil
}

func TestStatusReturnsStatusCode(t *testing.T) {
	// Every status is a result, not an error, because the caller decides what a
	// status means; that includes 200 and a redirect.
	statuses := []int{
		http.StatusOK,
		http.StatusNoContent,
		http.StatusTemporaryRedirect,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	}
	for _, status := range statuses {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := llmhttptest.NewResponseServer(t, status, []byte("{ not json }"))
			got, err := Status(context.Background(), Call{
				Client:     NewClient(),
				Timeout:    testTimeout,
				NewRequest: postRequest(server.URL),
				Errors:     Errors{Transport: errTestTransport},
			})
			if err != nil {
				t.Fatalf("Status() error = %v, want nil", err)
			}
			if got != status {
				t.Errorf("Status() = %d, want %d", got, status)
			}
		})
	}
}

func TestStatusDoesNotReadTheBody(t *testing.T) {
	t.Run("returns before a body that never arrives", func(t *testing.T) {
		server := llmhttptest.NewStatusStallingServer(t, http.StatusUnauthorized)
		start := time.Now()
		got, err := Status(context.Background(), Call{
			Client:     NewClient(),
			Timeout:    testStallTimeout,
			NewRequest: postRequest(server.URL),
			Errors:     testErrors(),
		})
		elapsed := time.Since(start)
		if err != nil || got != http.StatusUnauthorized {
			t.Errorf("Status() = %d, %v, want %d, nil", got, err, http.StatusUnauthorized)
		}
		if elapsed >= testStallTimeout {
			t.Errorf("Status() took %v; it must return without reading the body", elapsed)
		}
	})

	t.Run("closes the body unread", func(t *testing.T) {
		body := &trackingBody{}
		_, err := Status(context.Background(), Call{
			Client:     &http.Client{Transport: &trackingTransport{status: http.StatusOK, body: body}},
			Timeout:    testTimeout,
			NewRequest: postRequest("http://127.0.0.1:1/"),
			Errors:     testErrors(),
		})
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if body.read.Load() {
			t.Error("Status read the response body")
		}
		if !body.closed.Load() {
			t.Error("Status did not close the response body")
		}
	})
}

func TestStatusNoRedirect(t *testing.T) {
	var hits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/primary", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/secondary", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/secondary", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cases := map[string]*http.Client{
		"client that follows redirects": {},
		"nil client":                    nil,
	}
	for name, client := range cases {
		t.Run(name, func(t *testing.T) {
			before := hits.Load()
			got, err := Status(context.Background(), Call{
				Client:     client,
				Timeout:    testTimeout,
				NewRequest: postRequest(server.URL + "/primary"),
				Errors:     testErrors(),
			})
			if err != nil || got != http.StatusTemporaryRedirect {
				t.Errorf("Status() = %d, %v, want %d, nil", got, err, http.StatusTemporaryRedirect)
			}
			if hits.Load() != before {
				t.Error("the redirect target was requested; Status must not follow redirects")
			}
		})
	}
}

func TestStatusIncompleteCall(t *testing.T) {
	valid := func() Call {
		return Call{
			Client:     NewClient(),
			Timeout:    testTimeout,
			NewRequest: postRequest("http://127.0.0.1:1/"),
			Errors:     Errors{Transport: errTestTransport},
		}
	}
	cases := map[string]func(call *Call){
		"zero timeout":   func(call *Call) { call.Timeout = 0 },
		"nil NewRequest": func(call *Call) { call.NewRequest = nil },
		"nil Transport":  func(call *Call) { call.Errors.Transport = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			call := valid()
			mutate(&call)
			transport := &countingTransport{}
			call.Client = &http.Client{Transport: transport}
			_, err := Status(context.Background(), call)
			if !errors.Is(err, errIncompleteCall) {
				t.Errorf("Status() error = %v, want errIncompleteCall", err)
			}
			if got := transport.calls.Load(); got != 0 {
				t.Errorf("RoundTrip calls = %d, want 0", got)
			}
		})
	}

	// Status never builds the status or invalid-response errors, so a Call
	// that leaves them out is complete.
	t.Run("only Transport is required", func(t *testing.T) {
		server := llmhttptest.NewResponseServer(t, http.StatusOK, nil)
		got, err := Status(context.Background(), Call{
			Client:     NewClient(),
			Timeout:    testTimeout,
			NewRequest: postRequest(server.URL),
			Errors:     Errors{Transport: errTestTransport},
		})
		if err != nil || got != http.StatusOK {
			t.Errorf("Status() = %d, %v, want %d, nil", got, err, http.StatusOK)
		}
	})
}
