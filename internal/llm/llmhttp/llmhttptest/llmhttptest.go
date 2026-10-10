//go:build test

// Package llmhttptest provides the test servers and loopback listeners that
// the llmhttp tests and the LLM adapters' tests share. It is built only with
// the test tag and deliberately does not import llmhttp, so a test in the
// llmhttp package can import it without an import cycle.
package llmhttptest

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

const (
	// partialResponseBody is a prefix of a JSON object. A server that writes
	// it and then stops makes a client block in the middle of reading a body.
	partialResponseBody = `{"data":"`
	// fragmentLength is the Content-Length a server advertises while sending
	// only part of a body, so a client keeps waiting for the rest.
	fragmentLength = 4096
)

// Recorded is one request received by a Recorder. Target is the request URI
// (path and query), so a test can assert that no secret appears in the URL.
type Recorded struct {
	Method string
	Target string
	Header http.Header
	Body   []byte
}

// Recorder is an http.Handler that records every request and answers each with
// the fixed body and a 200 status.
type Recorder struct {
	t        *testing.T
	body     []byte
	mu       sync.Mutex
	requests []Recorded
}

// NewRecordingServer starts a test server that records every request and
// answers each with body and a 200 status.
func NewRecordingServer(t *testing.T, body []byte) (*httptest.Server, *Recorder) {
	t.Helper()
	recorder := &Recorder{t: t, body: body}
	server := httptest.NewServer(recorder)
	t.Cleanup(server.Close)
	return server, recorder
}

// ServeHTTP implements http.Handler.
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		r.t.Errorf("read request body: %v", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.requests = append(r.requests, Recorded{
		Method: req.Method,
		Target: req.URL.RequestURI(),
		Header: req.Header.Clone(),
		Body:   data,
	})
	r.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(r.body); err != nil {
		r.t.Errorf("write response body: %v", err)
	}
}

// Count returns how many requests the recorder received.
func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

// All returns a copy of every recorded request.
func (r *Recorder) All() []Recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Recorded(nil), r.requests...)
}

// Only returns the single recorded request or fails the test.
func (r *Recorder) Only(t *testing.T) Recorded {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 1 {
		t.Fatalf("server received %d requests, want exactly 1", len(r.requests))
	}
	return r.requests[0]
}

// NewResponseServer starts a test server that answers every request with the
// given status and body. The write is best effort: a client that stops
// reading, as a size-limit test does, makes it fail with a broken pipe.
func NewResponseServer(t *testing.T, status int, body []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

// newStallingServer starts a test server whose handler runs writeHead and then
// waits until the test ends. The server never reads the request body, so it
// cannot notice that the client disconnected; the release channel closed at
// cleanup is what ends the handler before httptest.Server.Close waits for it.
func newStallingServer(t *testing.T, writeHead func(w http.ResponseWriter)) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		writeHead(w)
		select {
		case <-release:
		case <-req.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	return server
}

// NewStallingServer starts a test server that returns no response headers and
// then waits until the test ends, so a client with a deadline times out.
func NewStallingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newStallingServer(t, func(http.ResponseWriter) {})
}

// NewPartialStallingServer starts a test server that returns a 200 status, a
// Content-Length larger than the body, and part of a body, then waits until
// the test ends. A client with a deadline times out while reading the body.
func NewPartialStallingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newStallingServer(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Length", strconv.Itoa(fragmentLength))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(partialResponseBody)); err != nil {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	})
}

// NewStatusStallingServer starts a test server that returns the given non-200
// status, a Content-Length larger than any body, and no body, then waits until
// the test ends. A client that reads the body before reporting the status
// blocks until its deadline; one that reports the status first returns at once.
func NewStatusStallingServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	return newStallingServer(t, func(w http.ResponseWriter) {
		w.Header().Set("Content-Length", strconv.Itoa(fragmentLength))
		w.WriteHeader(status)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	})
}

// NewGateServer starts a test server that reports the first request on the
// returned channel, then waits until the test ends (see newStallingServer for
// why the request context alone is not enough).
func NewGateServer(t *testing.T) (*httptest.Server, <-chan struct{}) {
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

// NewTruncatedResponseServer starts a listener that answers one HTTP request
// with a 200, a Content-Length larger than the body, a fragment of a response
// JSON, and then closes the connection. It returns the listener's URL.
func NewTruncatedResponseServer(t *testing.T) string {
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
		head := "HTTP/1.1 200 OK\r\nContent-Length: " + strconv.Itoa(fragmentLength) +
			"\r\nContent-Type: application/json\r\n\r\n"
		_, _ = conn.Write([]byte(head + partialResponseBody))
	}()
	return "http://" + listener.Addr().String()
}

// NewClosedListener starts a loopback listener that accepts connections and
// closes them immediately, and returns its URL. A client that connects to it
// sees a transport failure. The listener is closed at test cleanup.
func NewClosedListener(t *testing.T) string {
	t.Helper()
	proxy, err := StartBlackholeProxy()
	if err != nil {
		t.Fatalf("start closed listener: %v", err)
	}
	t.Cleanup(proxy.Stop)
	return proxy.URL()
}

// BlackholeProxy accepts connections on a loopback listener and closes them
// immediately. A test installs it in the proxy environment variables so a
// request that accidentally targets a non-loopback endpoint cannot reach the
// network.
type BlackholeProxy struct {
	listener   net.Listener
	accepted   atomic.Int64
	acceptDone chan struct{}
}

// StartBlackholeProxy opens the loopback listener and begins accepting.
func StartBlackholeProxy() (*BlackholeProxy, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	proxy := &BlackholeProxy{listener: listener, acceptDone: make(chan struct{})}
	go proxy.acceptLoop()
	return proxy, nil
}

// acceptLoop closes every accepted connection.
func (p *BlackholeProxy) acceptLoop() {
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

// URL returns the proxy URL to install in the proxy environment variables.
func (p *BlackholeProxy) URL() string {
	return "http://" + p.listener.Addr().String()
}

// Accepted returns how many connections the listener has accepted.
func (p *BlackholeProxy) Accepted() int64 {
	return p.accepted.Load()
}

// Stop closes the listener and waits for the accept loop to end.
func (p *BlackholeProxy) Stop() {
	_ = p.listener.Close()
	<-p.acceptDone
}
