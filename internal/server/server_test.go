//go:build test

package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/failure"
	failuretestutil "github.com/isseis/yt2column/internal/failure/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/udstest"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// newServer builds a Server with the given collaborators and log output.
func newServer(t *testing.T, w writer.ArticleWriter, p publisher.Publisher, n failure.Notifier, logs io.Writer) *Server {
	t.Helper()
	s, err := New(Options{
		Writer:    w,
		Publisher: p,
		Notifier:  n,
		LogOutput: logs,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// listenUnix opens a Unix domain socket listener in a fresh short directory and
// returns it with its path.
func listenUnix(t *testing.T) (net.Listener, string) {
	t.Helper()
	path := filepath.Join(udstest.Dir(t), "s.sock")
	return udstest.Listen(t, path), path
}

// serveUnix runs s.Serve on a fresh Unix socket until the test ends, and
// returns the socket's path. At cleanup it cancels Serve and waits for it.
func serveUnix(t *testing.T, s *Server) string {
	t.Helper()
	listener, path := listenUnix(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return path
}

// postOnUnix sends a valid article request to the socket at path and returns
// the status and the decoded body.
func postOnUnix(t *testing.T, path string) (int, responseBody) {
	t.Helper()
	body := strings.NewReader(validBody(testValidVideoURL, testValidRaw))
	req, err := http.NewRequest(http.MethodPost, "http://yt2column"+ArticlesPath, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	client := &http.Client{Transport: udstest.Transport(path)}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send the request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var decoded responseBody
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode the response body: %v", err)
	}
	return resp.StatusCode, decoded
}

// TestServeOnUnixListener checks that Serve answers a request that reaches it
// over a Unix domain socket listener with the accepted response.
func TestServeOnUnixListener(t *testing.T) {
	s := newServer(t, &writertestutil.FakeArticleWriter{}, &publishertestutil.FakePublisher{}, &failuretestutil.FakeNotifier{}, &bytes.Buffer{})
	path := serveUnix(t, s)
	status, body := postOnUnix(t, path)
	if status != http.StatusAccepted || body.Status != "accepted" {
		t.Errorf("response = %d %+v, want 202 accepted", status, body)
	}
}

// TestServeListenerFailureStops checks that when the listener fails, Serve stops
// and returns the error.
func TestServeListenerFailureStops(t *testing.T) {
	s := newServer(t, &writertestutil.FakeArticleWriter{}, &publishertestutil.FakePublisher{}, &failuretestutil.FakeNotifier{}, &bytes.Buffer{})
	listener, _ := listenUnix(t)
	done := make(chan error, 1)
	go func() { done <- s.Serve(context.Background(), listener) }()
	time.Sleep(50 * time.Millisecond)
	if err := listener.Close(); err != nil {
		t.Fatalf("close the listener: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Errorf("Serve error = nil, want the listener error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the listener failed")
	}
}

// TestServeShutdownWaitsForJobs checks that a canceled context stops accepting
// connections but Serve waits for the in-flight job before returning, and that
// the log names how many jobs the shutdown waits for.
func TestServeShutdownWaitsForJobs(t *testing.T) {
	gate := newGateWriter()
	logs := &bytes.Buffer{}
	s := newServer(t, gate, &publishertestutil.FakePublisher{}, &failuretestutil.FakeNotifier{}, logs)
	listener, path := listenUnix(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listener) }()
	var once sync.Once
	releaseGate := func() { once.Do(func() { close(gate.release) }) }
	defer func() {
		cancel()
		releaseGate()
	}()

	if status, _ := postOnUnix(t, path); status != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", status)
	}

	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the job did not start")
	}

	cancel()
	select {
	case <-done:
		t.Fatal("Serve returned before the in-flight job finished")
	case <-time.After(100 * time.Millisecond):
	}

	releaseGate()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve error = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the job finished")
	}
	// Serve has returned, so nothing writes to logs any more.
	if !strings.Contains(logs.String(), "pending=1") {
		t.Errorf("the shutdown log does not name the one pending job:\n%s", logs.String())
	}
}

// TestBodyReadFailureWritesNoResponse checks on a real socket that a body the
// server cannot read (a malformed chunked encoding) gets no response at all:
// the connection is closed without net/http's implicit empty 200.
func TestBodyReadFailureWritesNoResponse(t *testing.T) {
	ts := newTestServer(t)
	conn := dialUnix(t, serveUnix(t, ts.Server))
	request := "POST " + ArticlesPath + " HTTP/1.1\r\n" +
		"Host: yt2column\r\n" +
		"Transfer-Encoding: chunked\r\n\r\n" +
		"zz\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("write the request: %v", err)
	}
	data, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read the response: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("response = %q, want the connection closed with no response", data)
	}
	ts.waitJobs()
	if len(ts.slots) != 0 {
		t.Errorf("slots held = %d, want 0", len(ts.slots))
	}
	if len(ts.notifier.Calls) != 0 {
		t.Errorf("notifier calls = %d, want 0 for a body-read failure", len(ts.notifier.Calls))
	}
}

// dialUnix connects to the socket at path with a read deadline, and closes the
// connection at cleanup.
func dialUnix(t *testing.T, path string) net.Conn {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set the read deadline: %v", err)
	}
	return conn
}

// TestExpectContinueOversizedBodyNotSent checks over a Unix socket that a
// request declaring a body over the limit with Expect: 100-continue is answered
// with 413 before the client sends a byte of the body, so the client receives
// the rejection instead of a write error.
func TestExpectContinueOversizedBodyNotSent(t *testing.T) {
	ts := newTestServer(t)
	path := serveUnix(t, ts.Server)
	counter := &recordingReader{r: strings.NewReader(strings.Repeat("x", maxBodyBytes+1))}
	req, err := http.NewRequest(http.MethodPost, "http://yt2column"+ArticlesPath, io.NopCloser(counter))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.ContentLength = maxBodyBytes + 1
	req.Header.Set("Expect", "100-continue")
	transport := udstest.Transport(path)
	// A zero ExpectContinueTimeout sends the body at once; a non-zero one waits
	// for 100 Continue, which the server writes only when the handler reads.
	transport.ExpectContinueTimeout = 5 * time.Second
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatalf("send the request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
	if counter.n != 0 {
		t.Errorf("the client sent %d bytes of the body, want 0", counter.n)
	}
}

// TestUnparsableRequestBypassesHandler checks that bytes that are not an HTTP
// request are answered by net/http itself with 400, and that the handler never
// runs for them: no slot is taken, no failure notice is posted, and no article
// is generated.
func TestUnparsableRequestBypassesHandler(t *testing.T) {
	ts := newTestServer(t)
	conn := dialUnix(t, serveUnix(t, ts.Server))
	if _, err := conn.Write([]byte("NOT AN HTTP REQUEST\r\n\r\n")); err != nil {
		t.Fatalf("write the request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read the response: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read the response body: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	// The handler answers with a JSON body carrying a requestId and logs every
	// path it takes; net/http's own answer does neither, and nothing else logs
	// before the shutdown.
	if strings.Contains(string(body), "requestId") || ts.logs.Len() != 0 {
		t.Errorf("the handler answered the unparsable request: body %q, log:\n%s", body, ts.logs.String())
	}
	ts.waitJobs()
	if len(ts.slots) != 0 {
		t.Errorf("slots held = %d, want 0", len(ts.slots))
	}
	if len(ts.notifier.Calls) != 0 || len(ts.writer.Calls) != 0 {
		t.Errorf("an unparsable request caused a side effect: notifier=%d writer=%d",
			len(ts.notifier.Calls), len(ts.writer.Calls))
	}
}
