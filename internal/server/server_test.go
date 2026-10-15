//go:build test

package server

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/failure"
	failuretestutil "github.com/isseis/yt2column/internal/failure/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// requireIPv6Loopback skips the test when the IPv6 loopback is unavailable.
func requireIPv6Loopback(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	_ = listener.Close()
}

// freePort returns a port that was free on the IPv4 loopback a moment ago.
func freePort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("close the probe listener: %v", err)
	}
	return uint16(port) //nolint:gosec // a TCP port fits in uint16
}

// newServerOnPort builds a Server with the given port, collaborators, and log
// output.
func newServerOnPort(t *testing.T, port uint16, w writer.ArticleWriter, p publisher.Publisher, n failure.Notifier, logs io.Writer) *Server {
	t.Helper()
	s, err := New(Options{
		Port:      port,
		Token:     newTestToken(t),
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

// listenOnFreePort finds a free port and binds both loopback addresses on it.
func listenOnFreePort(t *testing.T, s *Server) ([]net.Listener, uint16) {
	t.Helper()
	requireIPv6Loopback(t)
	var lastErr error
	for range 10 {
		port := freePort(t)
		s.port = port
		listeners, err := s.Listen()
		if err == nil {
			return listeners, port
		}
		lastErr = err
	}
	t.Fatalf("Listen could not bind a free port after several attempts: %v", lastErr)
	return nil, 0
}

// closeListeners closes every listener, ignoring an already-closed one.
func closeListeners(listeners []net.Listener) {
	for _, l := range listeners {
		_ = l.Close()
	}
}

// TestListenBindsLoopbackOnly checks that Listen binds only 127.0.0.1 and ::1 on
// the configured port.
func TestListenBindsLoopbackOnly(t *testing.T) {
	s := newServerOnPort(t, testPort, &writertestutil.FakeArticleWriter{}, &publishertestutil.FakePublisher{}, &failuretestutil.FakeNotifier{}, &bytes.Buffer{})
	listeners, port := listenOnFreePort(t, s)
	defer closeListeners(listeners)

	got := map[string]bool{}
	for _, l := range listeners {
		got[l.Addr().String()] = true
	}
	want := map[string]bool{
		"127.0.0.1:" + strconv.Itoa(int(port)): true,
		"[::1]:" + strconv.Itoa(int(port)):     true,
	}
	for addr := range want {
		if !got[addr] {
			t.Errorf("missing listener for %s (got %v)", addr, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("listeners = %v, want only %v", got, want)
	}
}

// TestServeListenerFailureStops checks that when one listener fails, Serve stops
// and returns the error instead of serving on the other alone.
func TestServeListenerFailureStops(t *testing.T) {
	s := newServerOnPort(t, testPort, &writertestutil.FakeArticleWriter{}, &publishertestutil.FakePublisher{}, &failuretestutil.FakeNotifier{}, &bytes.Buffer{})
	listeners, _ := listenOnFreePort(t, s)
	done := make(chan error, 1)
	go func() { done <- s.Serve(context.Background(), listeners) }()
	time.Sleep(50 * time.Millisecond)
	if err := listeners[0].Close(); err != nil {
		t.Fatalf("close a listener: %v", err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Errorf("Serve error = nil, want the listener error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after a listener failed")
	}
	if _, err := listeners[1].Accept(); err == nil {
		t.Error("the other listener is still open after one failed")
	}
}

// TestServeShutdownWaitsForJobs checks that a canceled context stops accepting
// connections but Serve waits for the in-flight job before returning, and that
// the log names how many jobs the shutdown waits for.
func TestServeShutdownWaitsForJobs(t *testing.T) {
	gate := newGateWriter()
	logs := &bytes.Buffer{}
	s := newServerOnPort(t, testPort, gate, &publishertestutil.FakePublisher{}, &failuretestutil.FakeNotifier{}, logs)
	listeners, port := listenOnFreePort(t, s)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, listeners) }()
	var once sync.Once
	releaseGate := func() { once.Do(func() { close(gate.release) }) }
	defer func() {
		cancel()
		releaseGate()
	}()

	body := strings.NewReader(validBody(testValidVideoURL, testValidRaw))
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(int(port))+ArticlesPath, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", jsonContentType)
	req.Header.Set("Origin", extensionOrigin)
	req.Header.Set("Authorization", bearerPrefix+testTokenValue)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send the request: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
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
	srv := httptest.NewServer(http.HandlerFunc(ts.handler))
	t.Cleanup(srv.Close)
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	request := "POST " + ArticlesPath + " HTTP/1.1\r\n" +
		"Host: 127.0.0.1:" + strconv.Itoa(testPort) + "\r\n" +
		"Content-Type: " + jsonContentType + "\r\n" +
		"Origin: " + extensionOrigin + "\r\n" +
		"Authorization: " + bearerPrefix + testTokenValue + "\r\n" +
		"Transfer-Encoding: chunked\r\n\r\n" +
		"zz\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("write the request: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set the read deadline: %v", err)
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
