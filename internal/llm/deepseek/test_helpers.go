//go:build test

package deepseek

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/secret"
)

const (
	// testAPIKey is a fixed, non-secret value. No test uses a real key.
	testAPIKey = "test-api-key-0123456789abcdef"
	// testModel is the model name used to build test clients.
	testModel = "deepseek-flash"
	// testClientTimeout is the adapter timeout of a test client.
	testClientTimeout = 2 * time.Second
	// testTimeoutGrace is the margin allowed for Generate to return after
	// the adapter timeout has elapsed.
	testTimeoutGrace = 2 * time.Second
	// Paths to the committed fixtures, relative to this package directory.
	testdataStopFixture   = "../../../testdata/deepseek_chat_completion_stop.json"
	testdataLengthFixture = "../../../testdata/deepseek_chat_completion_length.json"
)

// mustSecret wraps value in a secret.Secret or fails the test.
func mustSecret(t *testing.T, value string) secret.Secret {
	t.Helper()
	apiKey, err := secret.New(value)
	if err != nil {
		t.Fatalf("secret.New(%q) error = %v", value, err)
	}
	return apiKey
}

// newTestClient builds a client for the given loopback endpoint by delegating
// to NewForLoopbackTest with the test defaults. The address must be a loopback
// URL; every other address fails the test without building a client.
func newTestClient(t *testing.T, endpoint string, modify func(*Options)) *client {
	t.Helper()
	options := Options{APIKey: mustSecret(t, testAPIKey), Model: testModel, Timeout: testClientTimeout}
	if modify != nil {
		modify(&options)
	}
	value := NewForLoopbackTest(t, options, endpoint)
	client, ok := value.(*client)
	if !ok {
		t.Fatalf("NewForLoopbackTest returned %T, want *client", value)
	}
	return client
}

// validRequest returns a request that passes Validate.
func validRequest() llm.GenerateRequest {
	return llm.GenerateRequest{
		SystemPrompt: "Answer concisely.",
		UserPrompt:   "Why is the sky blue?",
	}
}

// readFixture returns the bytes of a committed testdata fixture.
func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the path names a committed testdata fixture
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// generateAgainst builds a test client for serverURL and runs Generate with a
// background context.
func generateAgainst(t *testing.T, serverURL string, req llm.GenerateRequest, modify func(*Options)) (llm.GenerateResponse, error) {
	t.Helper()
	return newTestClient(t, serverURL, modify).Generate(context.Background(), req)
}

// replaceOnce replaces the single occurrence of old in body, failing the test
// when the needle is absent or ambiguous.
func replaceOnce(t *testing.T, body []byte, old, replacement string) []byte {
	t.Helper()
	text := string(body)
	if count := strings.Count(text, old); count != 1 {
		t.Fatalf("needle %q appears %d times, want exactly 1", old, count)
	}
	return []byte(strings.Replace(text, old, replacement, 1))
}

// documentOf decodes a response body into a generic document for targeted
// mutation.
func documentOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return document
}

// encodeDocument marshals a mutated document back to JSON bytes.
func encodeDocument(t *testing.T, document map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode document: %v", err)
	}
	return data
}

// choiceOf returns choices[0] of a decoded document.
func choiceOf(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	choices, ok := document[keyChoices].([]any)
	if !ok || len(choices) == 0 {
		t.Fatalf("choices = %#v, want a non-empty array", document[keyChoices])
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		t.Fatalf("choices[0] = %#v, want an object", choices[0])
	}
	return choice
}

// messageOf returns choices[0].message of a decoded document.
func messageOf(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	message, ok := choiceOf(t, document)[keyMessage].(map[string]any)
	if !ok {
		t.Fatalf("message = %#v, want an object", choiceOf(t, document)[keyMessage])
	}
	return message
}

// assertSingleSentinel checks that err matches exactly one of the seven
// adapter sentinels and the two context errors.
func assertSingleSentinel(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want a sentinel error")
	}
	sentinels := []error{
		llm.ErrInvalidRequest,
		llm.ErrTruncated,
		llm.ErrUnexpectedFinishReason,
		llm.ErrEmptyResponse,
		ErrHTTPStatus,
		ErrInvalidResponse,
		ErrTransport,
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

// assertErrorPrefix checks the "deepseek: " prefix appears once.
func assertErrorPrefix(t *testing.T, err error) {
	t.Helper()
	message := err.Error()
	if !strings.HasPrefix(message, errorPrefix) {
		t.Errorf("error message %q does not start with %q", message, errorPrefix)
	}
	if strings.Count(message, errorPrefix) != 1 {
		t.Errorf("error message %q contains %q %d times, want 1", message, errorPrefix, strings.Count(message, errorPrefix))
	}
}

// assertNoAPIKey checks the test API key appears in none of the error's
// rendered forms.
func assertNoAPIKey(t *testing.T, err error) {
	t.Helper()
	rendered := []string{err.Error()}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		rendered = append(rendered, fmt.Sprintf(format, err))
	}
	for _, output := range rendered {
		if strings.Contains(output, testAPIKey) {
			t.Errorf("rendered error contains the API key: %q", output)
		}
	}
}

// assertRejected checks the common contract of a failure case: exactly one
// sentinel, the single error prefix, no API key, and the zero response.
func assertRejected(t *testing.T, response llm.GenerateResponse, err error) {
	t.Helper()
	assertSingleSentinel(t, err)
	assertErrorPrefix(t, err)
	assertNoAPIKey(t, err)
	if response != (llm.GenerateResponse{}) {
		t.Errorf("Generate() response = %+v, want the zero value", response)
	}
}

// recordedRequest is one request received by a requestRecorder. target is
// the request URI (path and query), so tests can assert that no secret
// appears in the URL either.
type recordedRequest struct {
	method string
	target string
	header http.Header
	body   []byte
}

// requestRecorder is an http.Handler that records every request and answers
// each with the fixed body and a 200 status.
type requestRecorder struct {
	t        *testing.T
	body     []byte
	mu       sync.Mutex
	requests []recordedRequest
}

// newRecordingServer starts a test server that records every request.
func newRecordingServer(t *testing.T, body []byte) (*httptest.Server, *requestRecorder) {
	t.Helper()
	recorder := &requestRecorder{t: t, body: body}
	server := httptest.NewServer(recorder)
	t.Cleanup(server.Close)
	return server, recorder
}

// ServeHTTP implements http.Handler.
func (r *requestRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		r.t.Errorf("read request body: %v", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.requests = append(r.requests, recordedRequest{
		method: req.Method,
		target: req.URL.RequestURI(),
		header: req.Header.Clone(),
		body:   data,
	})
	r.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(r.body); err != nil {
		r.t.Errorf("write response body: %v", err)
	}
}

// count returns how many requests the recorder received.
func (r *requestRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

// only returns the single recorded request or fails the test.
func (r *requestRecorder) only(t *testing.T) recordedRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 1 {
		t.Fatalf("server received %d requests, want exactly 1", len(r.requests))
	}
	return r.requests[0]
}

// newResponseServer starts a test server that answers every request with the
// given status and body. The write is best effort: a client that stops
// reading, as the size-limit test does, makes it fail with a broken pipe.
func newResponseServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

// newStallingServer starts a test server whose handler sends nothing (or,
// with partial, headers and part of a body) and then waits until the test
// ends. The server never reads the request body, so it cannot notice that the
// client disconnected; the release channel closed at cleanup is what ends the
// handler before httptest.Server.Close waits for it.
func newStallingServer(t *testing.T, partial bool) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if partial {
			w.Header().Set("Content-Length", "4096")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"model":"deepseek-flash"`)); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		select {
		case <-release:
		case <-req.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	return server
}

// newGateServer starts a test server that reports the first request on the
// returned channel, then waits until the test ends (see newStallingServer for
// why the request context alone is not enough).
func newGateServer(t *testing.T) (*httptest.Server, <-chan struct{}) {
	t.Helper()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		case <-req.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	return server, entered
}

// newTruncatedResponseServer starts a listener that answers one HTTP request
// with a 200, a Content-Length larger than the body, a fragment of a valid
// response JSON, and then closes the connection.
func newTruncatedResponseServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		const body = `{"model":"deepseek-flash","choices":[`
		head := "HTTP/1.1 200 OK\r\nContent-Length: 4096\r\nContent-Type: application/json\r\n\r\n"
		_, _ = conn.Write([]byte(head + body))
	}()
	return "http://" + listener.Addr().String()
}

// blackholeProxy accepts connections on a loopback listener and closes them
// immediately. TestMain installs it in the proxy environment variables so a
// test that accidentally targets a non-loopback endpoint cannot reach the
// network.
type blackholeProxy struct {
	listener   net.Listener
	accepted   atomic.Int64
	acceptDone chan struct{}
}

// testProxy is the proxy TestMain installed. Tests read it to confirm that the
// production endpoint is routed to the blackhole listener.
var testProxy *blackholeProxy

// startBlackholeProxy opens the loopback listener and begins accepting.
func startBlackholeProxy() (*blackholeProxy, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	proxy := &blackholeProxy{listener: listener, acceptDone: make(chan struct{})}
	go proxy.acceptLoop()
	return proxy, nil
}

// acceptLoop closes every accepted connection.
func (p *blackholeProxy) acceptLoop() {
	defer close(p.acceptDone)
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}
		p.accepted.Add(1)
		_ = conn.Close()
	}
}

// url returns the proxy URL to install in the proxy environment variables.
func (p *blackholeProxy) url() string {
	return "http://" + p.listener.Addr().String()
}

// stop closes the listener and waits for the accept loop to end.
func (p *blackholeProxy) stop() {
	_ = p.listener.Close()
	<-p.acceptDone
}
