//go:build test

package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/udstest"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// newServer builds a Server with the given collaborators and log output.
func newServer(t *testing.T, w writer.ProgressWriter, p WebhookPublisher, logs io.Writer) *Server {
	t.Helper()
	s, err := New(Options{
		Writer:    w,
		Webhook:   &WebhookOptions{Publisher: p},
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

// TestNewWebhookOptions checks that New treats a nil Webhook as a server
// without a Webhook, refuses a WebhookOptions whose publisher is missing, a
// typed nil included, and still refuses a missing writer or log output.
func TestNewWebhookOptions(t *testing.T) {
	pub := &publishertestutil.FakePublisher{}
	var nilPublisher *publishertestutil.FakePublisher
	fakeWriter := &writertestutil.FakeArticleWriter{}
	logs := &bytes.Buffer{}
	complete := &WebhookOptions{Publisher: pub}
	// all is every reason New can join, so each row pins the exact set: a
	// reason the row does not name must be absent.
	all := []error{errNilArticleWriter, errNilWebhookPublisher, errNilLogOutput}
	tests := []struct {
		name string
		opts Options
		want []error // empty means New accepts the options
	}{
		{"complete", Options{Writer: fakeWriter, Webhook: complete, LogOutput: logs}, nil},
		{"no_webhook", Options{Writer: fakeWriter, LogOutput: logs}, nil},
		{"no_publisher", Options{Writer: fakeWriter, Webhook: &WebhookOptions{}, LogOutput: logs}, []error{errNilWebhookPublisher}},
		{"typed_nil_publisher", Options{Writer: fakeWriter, Webhook: &WebhookOptions{Publisher: nilPublisher}, LogOutput: logs}, []error{errNilWebhookPublisher}},
		{"no_writer", Options{Webhook: complete, LogOutput: logs}, []error{errNilArticleWriter}},
		{"no_log_output", Options{Writer: fakeWriter, Webhook: complete}, []error{errNilLogOutput}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.opts)
			if len(tc.want) == 0 {
				if err != nil {
					t.Fatalf("New error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("New error = %v, want it to wrap ErrInvalidOptions", err)
			}
			for _, sentinel := range all {
				want := slices.Contains(tc.want, sentinel)
				if got := errors.Is(err, sentinel); got != want {
					t.Errorf("New error = %v, wraps %v: %v, want %v", err, sentinel, got, want)
				}
			}
		})
	}
}

// TestNewCopiesWebhookOptions checks that the server keeps the publisher it
// was built with, so a caller that clears its WebhookOptions afterwards
// changes nothing: a publish operation still posts through that publisher.
func TestNewCopiesWebhookOptions(t *testing.T) {
	pub := &publishertestutil.FakePublisher{}
	webhook := &WebhookOptions{Publisher: pub}
	s, err := New(Options{
		Writer:    &writertestutil.FakeArticleWriter{},
		Webhook:   webhook,
		LogOutput: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	webhook.Publisher = nil
	if !s.webhookConfigured() {
		t.Error("webhookConfigured() = false after the caller cleared its options")
	}
	ts := &testServer{Server: s, publisher: pub}
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw))
	rec := ts.publishRequest(key.String())
	ts.waitJobs()
	requireAccepted(t, rec)
	if calls := pub.RecordedPostCalls(); len(calls) != 1 {
		t.Errorf("PostMessages calls = %d, want 1", len(calls))
	}
}

// TestServeOnUnixListener checks that Serve answers a generation start that
// reaches it over a Unix domain socket listener with the accepted response.
func TestServeOnUnixListener(t *testing.T) {
	s := newServer(t, &writertestutil.FakeArticleWriter{}, &publishertestutil.FakePublisher{}, &bytes.Buffer{})
	path := serveUnix(t, s)
	if key := startOnUnix(t, path, validBody(testValidVideoURL, testValidRaw)); !keyShape.MatchString(key) {
		t.Errorf("key = %q, want an article key", key)
	}
}

// TestServeListenerFailureStops checks that when the listener fails, Serve stops
// and returns the error.
func TestServeListenerFailureStops(t *testing.T) {
	s := newServer(t, &writertestutil.FakeArticleWriter{}, &publishertestutil.FakePublisher{}, &bytes.Buffer{})
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

// TestStartBodyReadFailureWritesNoResponse checks on a real socket that a
// generation start whose body the server cannot read (a malformed chunked
// encoding) gets no response at all, and leaves no reading slot taken and no
// record.
func TestStartBodyReadFailureWritesNoResponse(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	conn := dialUnix(t, serveUnix(t, ts.Server))
	request := "POST " + JobsPath + " HTTP/1.1\r\n" +
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
	if len(ts.reading) != 0 || len(ts.slots) != 0 {
		t.Errorf("slots held: reading %d, generation %d, want 0", len(ts.reading), len(ts.slots))
	}
	if dump := ts.cache.DumpRecordsForTest(); dump != "" {
		t.Errorf("a failed body read added a record:\n%s", dump)
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
	req, err := http.NewRequest(http.MethodPost, "http://yt2column"+JobsPath, io.NopCloser(counter))
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
// runs for them: no slot is taken and no article is generated.
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
	if len(ts.writer.Calls) != 0 {
		t.Errorf("an unparsable request called the writer %d times", len(ts.writer.Calls))
	}
}

// startOnUnix sends a generation start to the socket at path and returns the
// key of its 202 response.
func startOnUnix(t *testing.T, path, body string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://yt2column"+JobsPath, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := (&http.Client{Transport: udstest.Transport(path)}).Do(req)
	if err != nil {
		t.Fatalf("send the request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var decoded responseBody
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode the response body: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted || decoded.Key == "" {
		t.Fatalf("response = %d %+v, want 202 with a key", resp.StatusCode, decoded)
	}
	return decoded.Key
}

// TestServeShutdownCancelsGeneration checks a shutdown with a generation job
// in flight: the writer sees its context canceled and the record fails,
// logging only the cancellation line, and the shutdown line counts the one
// generation that was running when the stop began.
func TestServeShutdownCancelsGeneration(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	reached, release := holdAtDraft(ts.writer)
	defer release()
	listener, path := listenUnix(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ts.Serve(ctx, listener) }()

	key, err := articlecache.ParseKey(startOnUnix(t, path, bodyWith("generation")))
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	<-reached

	cancel()
	waitFor(t, "the generation to be canceled", func() bool { return ts.cache.Lookup(key).State == articlecache.JobFailed })
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve error = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the generation was canceled")
	}
	logs := ts.logs.String()
	if !strings.Contains(logs, "generation job canceled by the shutdown") || strings.Contains(logs, "generation job failed") {
		t.Errorf("want the cancellation line and no failure line:\n%s", logs)
	}
	if line := logLine(logs, "stopped accepting connections"); !strings.Contains(line, "canceledGenerations=1") {
		t.Errorf("the shutdown log line = %q, want canceledGenerations=1", line)
	}
}

// TestServeShutdownCancelsLateStart checks a generation start that reaches
// the cache only after the shutdown began: its request is held inside the body
// read while Serve cancels the generations, and once the body arrives the start
// is still accepted, but its job begins already canceled. The record fails, the
// writer is never called, only the cancellation line is logged, the shutdown
// line does not count it among the canceled generations, and Serve returns
// with every slot free.
func TestServeShutdownCancelsLateStart(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	listener, path := listenUnix(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ts.Serve(ctx, listener) }()

	body := bodyWith("late")
	conn := dialUnix(t, path)
	header := "POST " + JobsPath + " HTTP/1.1\r\n" +
		"Host: yt2column\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n"
	if _, err := conn.Write([]byte(header)); err != nil {
		t.Fatalf("write the header: %v", err)
	}
	// The handler holds a reading slot while it waits for the body.
	waitFor(t, "the handler to start reading the body", func() bool { return len(ts.reading) == 1 })

	cancel()
	waitFor(t, "the generations to be canceled", func() bool { return ts.genCtx.Err() != nil })
	if _, err := conn.Write([]byte(body)); err != nil {
		t.Fatalf("write the body: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read the response: %v", err)
	}
	var accepted responseBody
	err = json.NewDecoder(resp.Body).Decode(&accepted)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("decode the response: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted || accepted.Key == "" {
		t.Fatalf("response = %d %+v, want 202 with a key", resp.StatusCode, accepted)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve error = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
	}
	key, err := articlecache.ParseKey(accepted.Key)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if state := ts.cache.Lookup(key).State; state != articlecache.JobFailed {
		t.Errorf("the late start's state = %d, want failed", state)
	}
	if calls := len(ts.writer.Calls); calls != 0 {
		t.Errorf("writer calls = %d, want 0 for a canceled generation", calls)
	}
	logs := ts.logs.String()
	if !strings.Contains(logs, "generation job canceled by the shutdown") || strings.Contains(logs, "generation job failed") {
		t.Errorf("want the cancellation line and no failure line:\n%s", logs)
	}
	if line := logLine(logs, "stopped accepting connections"); !strings.Contains(line, "canceledGenerations=0") {
		t.Errorf("the shutdown log line = %q, want canceledGenerations=0", line)
	}
	if len(ts.slots) != 0 || len(ts.reading) != 0 {
		t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
	}
}

// waitFor polls cond until it holds, failing the test after five seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
