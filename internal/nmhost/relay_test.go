//go:build test

package nmhost

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/udstest"
)

// proxyURL is set as the environment's proxy for the whole package, before
// any test runs: net/http reads the proxy variables once per process, so a
// transport that consulted them would route every request in this package
// through it, and the request target would become an absolute URI.
const proxyURL = "http://proxy.invalid:9"

func TestMain(m *testing.M) {
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		_ = os.Setenv(name, proxyURL)
	}
	for _, name := range []string{"NO_PROXY", "no_proxy"} {
		_ = os.Unsetenv(name)
	}
	if os.Getenv("HTTP_PROXY") != proxyURL || os.Getenv("NO_PROXY") != "" {
		fmt.Fprintln(os.Stderr, "TestMain could not set the proxy environment")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

const (
	// testTimeout is the relay limit of a test that expects a reply long
	// before it.
	testTimeout = 5 * time.Second
	// shortTimeout is the relay limit of a test that waits for it to pass.
	shortTimeout = 300 * time.Millisecond
)

// stdinFake is standard input holding a message length and a body. It records
// how far it was read, so a test can tell whether the body was read. With
// hold set, a read at the end of the data waits until hold is closed instead
// of returning io.EOF, as a browser that stops writing mid-message does.
type stdinFake struct {
	mu   sync.Mutex
	data []byte
	pos  int
	hold chan struct{}
}

func newStdin(length uint32, body []byte) *stdinFake {
	return &stdinFake{data: append(nativeLength(length), body...)}
}

// message returns standard input holding body with its own length.
func message(body []byte) *stdinFake {
	return newStdin(uint32(len(body)), body)
}

// stalledStdin returns standard input that declares length but holds only
// body, and then stalls until the test ends.
func stalledStdin(t *testing.T, length uint32, body []byte) *stdinFake {
	t.Helper()
	s := newStdin(length, body)
	s.hold = make(chan struct{})
	t.Cleanup(func() { close(s.hold) })
	return s
}

func (s *stdinFake) Read(p []byte) (int, error) {
	s.mu.Lock()
	if s.pos >= len(s.data) {
		s.mu.Unlock()
		if s.hold != nil {
			<-s.hold
		}
		return 0, io.EOF
	}
	defer s.mu.Unlock()
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

// consumed returns how many bytes were read, the length included.
func (s *stdinFake) consumed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pos
}

// bodyRead returns how many bytes after the length were read.
func (s *stdinFake) bodyRead() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return max(0, s.pos-lengthBytes)
}

// countDials wraps dial and counts its calls.
func countDials(dial Dialer) (Dialer, *atomic.Int32) {
	var n atomic.Int32
	return func(ctx context.Context) (net.Conn, error) {
		n.Add(1)
		return dial(ctx)
	}, &n
}

// failingDial is a Dialer that never connects.
func failingDial(context.Context) (net.Conn, error) {
	return nil, errors.New("no server")
}

// serveHTTP serves handler on a new Unix socket and returns its path.
func serveHTTP(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	path := filepath.Join(udstest.Dir(t), "s.sock")
	listener := udstest.Listen(t, path)
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: testTimeout}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = srv.Close() })
	return path
}

// serveRaw runs serve on every connection to a new Unix socket and returns its
// path. serve sees the connection after the request head was read, so it can
// answer with bytes no HTTP server would send.
func serveRaw(t *testing.T, serve func(net.Conn)) string {
	t.Helper()
	path := filepath.Join(udstest.Dir(t), "s.sock")
	listener := udstest.Listen(t, path)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
					return
				}
				serve(conn)
			}()
		}
	}()
	return path
}

// received is what a recording server saw of the last request, and how many
// requests it saw.
type received struct {
	method, requestURI, expect, contentType string
	contentLength                           int64
	transferEncoding                        []string
	body                                    []byte
	requests                                int
}

// recordingServer answers every request with 202 and body and records the last
// request it read.
func recordingServer(t *testing.T, body []byte) (string, func() received) {
	t.Helper()
	var mu sync.Mutex
	var got received
	path := serveHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = received{
			method:           r.Method,
			requestURI:       r.RequestURI,
			expect:           r.Header.Get("Expect"),
			contentType:      r.Header.Get("Content-Type"),
			contentLength:    r.ContentLength,
			transferEncoding: r.TransferEncoding,
			body:             data,
			requests:         got.requests + 1,
		}
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(body)
	})
	return path, func() received {
		mu.Lock()
		defer mu.Unlock()
		return got
	}
}

// relayResult is what one Relay call wrote.
type relayResult struct {
	stdout []byte
	log    string
	err    error
}

func runRelay(stdin io.Reader, dial Dialer, timeout time.Duration) relayResult {
	var stdout, log bytes.Buffer
	err := Relay(context.Background(), stdin, &stdout, RelayOptions{Dial: dial, Log: &log, Timeout: timeout})
	return relayResult{stdout: stdout.Bytes(), log: log.String(), err: err}
}

// reply decodes stdout as exactly one frame and returns its members.
func (r relayResult) reply(t *testing.T) map[string]any {
	t.Helper()
	if r.err != nil {
		t.Fatalf("Relay: %v", r.err)
	}
	length, err := readLength(bytes.NewReader(r.stdout))
	if err != nil {
		t.Fatalf("read the reply length: %v", err)
	}
	payload := r.stdout[lengthBytes:]
	if int(length) != len(payload) {
		t.Fatalf("stdout holds %d bytes after a length of %d; want exactly one frame", len(payload), length)
	}
	var members map[string]any
	if err := json.Unmarshal(payload, &members); err != nil {
		t.Fatalf("decode the reply: %v", err)
	}
	return members
}

// requireError checks that the reply is the error shape carrying want.
func requireError(t *testing.T, members map[string]any, want HostError) {
	t.Helper()
	wantMembers := map[string]any{"v": float64(protocolVersion), "kind": "error", "error": want.Value()}
	if !maps.Equal(members, wantMembers) {
		t.Errorf("reply = %v, want %v", members, wantMembers)
	}
}

// requireResponse checks that the reply is the response shape carrying status
// and exactly body.
func requireResponse(t *testing.T, members map[string]any, status int, body []byte) {
	t.Helper()
	wantMembers := map[string]any{
		"v": float64(protocolVersion), "kind": "response", "status": float64(status),
		"body": base64.StdEncoding.EncodeToString(body),
	}
	if !maps.Equal(members, wantMembers) {
		t.Errorf("reply = %v, want %v", members, wantMembers)
	}
}

func TestRelayForwardsOneRequest(t *testing.T) {
	// The message is not valid UTF-8, so a host that decoded or re-encoded it
	// would change its bytes.
	msg := []byte("{\"videoUrl\":\"x\",\"transcript\":\"\xff\xfe\"}")
	// The server's body is not valid UTF-8 either: the reply must carry its
	// bytes, not a repaired string.
	accepted := []byte("{\"status\":\"accepted\",\"requestId\":\"\xff\xc0\"}")
	path, got := recordingServer(t, accepted)
	dial, dials := countDials(SocketDialer(path))

	members := runRelay(message(msg), dial, testTimeout).reply(t)

	requireResponse(t, members, http.StatusAccepted, accepted)
	req := got()
	// TestMain sets a proxy in the environment, so this also checks that the
	// transport ignores it: through a proxy the target would be an absolute
	// URI.
	if req.method != http.MethodPost || req.requestURI != "/articles" {
		t.Errorf("request line = %s %s, want POST /articles", req.method, req.requestURI)
	}
	if req.contentLength != int64(len(msg)) || len(req.transferEncoding) != 0 {
		t.Errorf("Content-Length = %d, Transfer-Encoding = %v; want Content-Length %d", req.contentLength, req.transferEncoding, len(msg))
	}
	if req.expect != "100-continue" {
		t.Errorf("Expect = %q, want 100-continue", req.expect)
	}
	if !bytes.Equal(req.body, msg) {
		t.Errorf("server received %q, want the message bytes %q", req.body, msg)
	}
	if n := dials.Load(); n != 1 {
		t.Errorf("Dial called %d times, want 1", n)
	}
}

func TestRelayLengthChecks(t *testing.T) {
	tests := []struct {
		name      string
		length    uint32
		want      HostError
		wantDials int32
	}{
		{"one byte over the limit", maxMessageBytes + 1, HostErrorMessageTooLarge, 0},
		{"largest length", math.MaxUint32, HostErrorMessageTooLarge, 0},
		// At the limit the host connects; the failing dial makes that visible
		// without sending 32 MiB.
		{"at the limit", maxMessageBytes, HostErrorServerUnavailable, 1},
		// The browser never sends an empty message; it is not "too large".
		{"zero length", 0, HostErrorProtocol, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only the start of the body is present; bodyRead shows whether
			// the host read any of it before deciding.
			stdin := newStdin(tt.length, []byte(`{"videoUrl":`))
			dial, dials := countDials(failingDial)
			requireError(t, runRelay(stdin, dial, testTimeout).reply(t), tt.want)
			if n := dials.Load(); n != tt.wantDials {
				t.Errorf("Dial called %d times, want %d", n, tt.wantDials)
			}
			if n := stdin.bodyRead(); n != 0 {
				t.Errorf("read %d body bytes, want none", n)
			}
		})
	}
}

func TestRelayServerRejectsWithoutBody(t *testing.T) {
	rejected := []byte(`{"status":"rejected","reason":"body_too_large","requestId":"0123456789abcdef"}`)
	path := serveHTTP(t, func(w http.ResponseWriter, _ *http.Request) {
		// Answer from the head alone, as the server does for a
		// Content-Length over its limit.
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write(rejected)
	})
	body := bytes.Repeat([]byte("a"), 64<<10)
	stdin := message(body)

	requireResponse(t, runRelay(stdin, SocketDialer(path), testTimeout).reply(t), http.StatusRequestEntityTooLarge, rejected)
	// The host reads the whole message before it sends, so the server's
	// early answer no longer leaves the body unread.
	if n := stdin.bodyRead(); n != len(body) {
		t.Errorf("read %d body bytes, want the whole message of %d", n, len(body))
	}
}

// TestRelayMessageReadFailures checks a message that ends early or stalls:
// the host has connected, reads the whole message before sending, and sends
// nothing.
func TestRelayMessageReadFailures(t *testing.T) {
	msg := []byte(`{"videoUrl":"x","transcript":"y"}`)
	tests := []struct {
		name    string
		stdin   func(t *testing.T) io.Reader
		timeout time.Duration
		want    HostError
	}{
		{
			name:  "body shorter than the declared length",
			stdin: func(*testing.T) io.Reader { return newStdin(uint32(len(msg))+10, msg) },
			want:  HostErrorProtocol,
		},
		{
			name:    "stdin stalls mid-message",
			stdin:   func(t *testing.T) io.Reader { return stalledStdin(t, uint32(len(msg))+10, msg) },
			timeout: shortTimeout,
			want:    HostErrorServerTimeout,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, got := recordingServer(t, []byte("{}"))
			dial, dials := countDials(SocketDialer(path))

			result := runRelay(tt.stdin(t), dial, cmp.Or(tt.timeout, testTimeout))
			requireError(t, result.reply(t), tt.want)
			if n := dials.Load(); n != 1 {
				t.Errorf("Dial called %d times, want 1", n)
			}
			if n := got().requests; n != 0 {
				t.Errorf("the server received %d requests, want none", n)
			}
			if !strings.Contains(result.log, "stage=read") {
				t.Errorf("the log does not name the read stage:\n%s", result.log)
			}
		})
	}
}

func TestRelayResponseOverLimit(t *testing.T) {
	tests := []struct {
		name string
		size int
		over bool
	}{
		{"at the limit", maxResponseBytes, false},
		{"one byte over", maxResponseBytes + 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.Repeat([]byte("r"), tt.size)
			path, _ := recordingServer(t, body)
			members := runRelay(message([]byte("{}")), SocketDialer(path), testTimeout).reply(t)
			if tt.over {
				requireError(t, members, HostErrorProtocol)
			} else {
				requireResponse(t, members, http.StatusAccepted, body)
			}
		})
	}
}

// errStuck is a connection failure that is not a deadline error.
var errStuck = errors.New("the connection failed")

// deadlineConn is a connection whose every write fails at once with a
// connection deadline error, as a connection with an expired write deadline
// does. Reads wait for Close, so the write failure is the only one.
type deadlineConn struct {
	closed    chan struct{}
	closeOnce sync.Once
}

func (*deadlineConn) LocalAddr() net.Addr              { return &net.UnixAddr{Net: "unix"} }
func (*deadlineConn) RemoteAddr() net.Addr             { return &net.UnixAddr{Net: "unix"} }
func (*deadlineConn) SetDeadline(time.Time) error      { return nil }
func (*deadlineConn) SetReadDeadline(time.Time) error  { return nil }
func (*deadlineConn) SetWriteDeadline(time.Time) error { return nil }

func newDeadlineConn() *deadlineConn { return &deadlineConn{closed: make(chan struct{})} }

func (*deadlineConn) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write: %w", os.ErrDeadlineExceeded)
}

func (c *deadlineConn) Read([]byte) (int, error) {
	<-c.closed
	return 0, net.ErrClosed
}

func (c *deadlineConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func TestRelayHostErrors(t *testing.T) {
	msg := []byte(`{"videoUrl":"x","transcript":"y"}`)
	tests := []struct {
		name    string
		dial    func(t *testing.T) Dialer
		timeout time.Duration
		want    HostError
		// unread is whether the body must be left unread: nothing was sent.
		unread bool
		// errno is the OS error the log must name, when one causes the
		// failure; it is how a user tells the connect failures apart.
		errno syscall.Errno
	}{
		{
			name: "no socket",
			dial: func(t *testing.T) Dialer {
				return SocketDialer(filepath.Join(udstest.Dir(t), "absent.sock"))
			},
			want: HostErrorServerUnavailable, unread: true, errno: syscall.ENOENT,
		},
		{
			name: "stale socket",
			dial: func(t *testing.T) Dialer {
				path := filepath.Join(udstest.Dir(t), "stale.sock")
				listener, err := net.Listen("unix", path)
				if err != nil {
					t.Fatal(err)
				}
				listener.(*net.UnixListener).SetUnlinkOnClose(false)
				_ = listener.Close()
				return SocketDialer(path)
			},
			want: HostErrorServerUnavailable, unread: true, errno: syscall.ECONNREFUSED,
		},
		{
			name: "directory without permission",
			dial: func(t *testing.T) Dialer {
				skipIfRoot(t)
				dir := filepath.Join(udstest.Dir(t), "locked")
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "s.sock")
				udstest.Listen(t, path)
				chmodForTest(t, dir, 0)
				return SocketDialer(path)
			},
			want: HostErrorServerUnavailable, unread: true, errno: syscall.EACCES,
		},
		{
			name: "connect attempt outlasts the limit",
			dial: func(*testing.T) Dialer {
				return func(ctx context.Context) (net.Conn, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				}
			},
			timeout: shortTimeout,
			want:    HostErrorServerUnavailable, unread: true,
		},
		{
			name: "server never answers",
			dial: func(t *testing.T) Dialer {
				return SocketDialer(serveRaw(t, func(conn net.Conn) { _, _ = io.Copy(io.Discard, conn) }))
			},
			timeout: shortTimeout,
			want:    HostErrorServerTimeout,
		},
		{
			// The transport reports the broken write itself and the relay's
			// context is still live, so only the os.ErrDeadlineExceeded
			// check makes this a timeout.
			name: "write reports a connection deadline",
			dial: func(*testing.T) Dialer {
				return func(context.Context) (net.Conn, error) { return newDeadlineConn(), nil }
			},
			want: HostErrorServerTimeout,
		},
		{
			name: "response body stalls",
			dial: func(t *testing.T) Dialer {
				return SocketDialer(serveRaw(t, func(conn net.Conn) {
					_, _ = io.WriteString(conn, "HTTP/1.1 202 Accepted\r\nContent-Length: 100\r\n\r\n{\"status\"")
					_, _ = io.Copy(io.Discard, conn)
				}))
			},
			timeout: shortTimeout,
			want:    HostErrorServerTimeout,
		},
		{
			name: "response cut short",
			dial: func(t *testing.T) Dialer {
				return SocketDialer(serveRaw(t, func(conn net.Conn) {
					_, _ = io.WriteString(conn, "HTTP/1.1 202 Accepted\r\nContent-Length: 100\r\n\r\n{\"status\"")
				}))
			},
			want: HostErrorProtocol,
		},
		{
			name: "response not HTTP",
			dial: func(t *testing.T) Dialer {
				return SocketDialer(serveRaw(t, func(conn net.Conn) { _, _ = io.WriteString(conn, "not http\r\n\r\n") }))
			},
			want: HostErrorProtocol,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dial, dials := countDials(tt.dial(t))
			stdin := message(msg)
			timeout := cmp.Or(tt.timeout, testTimeout)

			result := runRelay(stdin, dial, timeout)
			requireError(t, result.reply(t), tt.want)
			if tt.errno != 0 && !strings.Contains(result.log, "errno=\""+tt.errno.Error()+"\"") {
				t.Errorf("the log does not name %q:\n%s", tt.errno.Error(), result.log)
			}
			if n := dials.Load(); n != 1 {
				t.Errorf("Dial called %d times, want 1", n)
			}
			if n := stdin.bodyRead(); tt.unread && n != 0 {
				t.Errorf("read %d body bytes, want none", n)
			}
		})
	}
}

func TestExchangeFailurePrecedence(t *testing.T) {
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	t.Cleanup(cancel)
	live := context.Background()
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want HostError
	}{
		// The transport can report the broken write rather than the deadline
		// when both happen at once, and which one it reports is a race; this
		// input leaves only the passed-deadline check to decide.
		{"deadline passed, write error", expired, errStuck, HostErrorServerTimeout},
		{"other failure", live, errStuck, HostErrorProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := exchangeFailure(tt.ctx, tt.err).reply, any(newErrorReply(tt.want)); got != want {
				t.Errorf("reply = %v, want %v", got, want)
			}
		})
	}
}

// TestOneUseTransportRefusesRedial checks the transport directly: a second
// dial cannot be provoked through Relay, since net/http does not redial for a
// body it cannot replay. The Dial count is asserted in TestRelayHostErrors.
func TestOneUseTransportRefusesRedial(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	transport := oneUseTransport(client, testTimeout)

	first, err := transport.DialContext(context.Background(), "tcp", "yt2column:80")
	if err != nil || first != client {
		t.Fatalf("first DialContext = %v, %v; want the relay connection", first, err)
	}
	if second, err := transport.DialContext(context.Background(), "tcp", "yt2column:80"); !errors.Is(err, errRedial) {
		t.Errorf("second DialContext = %v, %v; want errRedial", second, err)
	}
}

func TestReplyShapes(t *testing.T) {
	t.Run("over the frame limit", func(t *testing.T) {
		huge := newResponseReply(http.StatusAccepted, bytes.Repeat([]byte("a"), maxFrameBytes))
		var members map[string]any
		if err := json.Unmarshal(encodeReply(huge), &members); err != nil {
			t.Fatalf("decode the reply: %v", err)
		}
		requireError(t, members, HostErrorProtocol)
	})
	t.Run("host error values", func(t *testing.T) {
		tests := []struct {
			e    HostError
			want string
		}{
			{HostError(0), "protocol_error"},
			{HostErrorServerUnavailable, "server_unavailable"},
			{HostErrorServerTimeout, "server_timeout"},
			{HostErrorMessageTooLarge, "message_too_large"},
			{HostError(-1), "protocol_error"},
			{HostErrorMessageTooLarge + 1, "protocol_error"},
		}
		for _, tt := range tests {
			if got := tt.e.Value(); got != tt.want {
				t.Errorf("HostError(%d).Value() = %q, want %q", int(tt.e), got, tt.want)
			}
		}
	})
}

func TestRelayOutputs(t *testing.T) {
	const (
		messageMarker  = "transcript-marker-7f3a"
		responseMarker = "response-marker-91c2"
	)
	msg := []byte(`{"videoUrl":"x","transcript":"` + messageMarker + `"}`)
	responseBody := []byte(`{"requestId":"` + responseMarker + `"}`)
	tests := []struct {
		name  string
		dial  func(t *testing.T) Dialer
		check func(t *testing.T, members map[string]any)
	}{
		{
			name: "response",
			dial: func(t *testing.T) Dialer {
				path, _ := recordingServer(t, responseBody)
				return SocketDialer(path)
			},
			check: func(t *testing.T, members map[string]any) {
				if members["kind"] != "response" {
					t.Errorf("reply = %v, want the response shape", members)
				}
			},
		},
		{
			// net/http quotes a malformed response in its error, so a host
			// that logged the error text would log the server's bytes.
			name: "unreadable response",
			dial: func(t *testing.T) Dialer {
				return SocketDialer(serveRaw(t, func(conn net.Conn) { _, _ = io.WriteString(conn, responseMarker+"\r\n\r\n") }))
			},
			check: func(t *testing.T, members map[string]any) { requireError(t, members, HostErrorProtocol) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runRelay(message(msg), tt.dial(t), testTimeout)
			tt.check(t, result.reply(t))
			if result.log == "" {
				t.Error("nothing was logged")
			}
			// The reply carries the server's bytes in base64, so a log of the
			// reply would hide them from a plain-text search.
			encoded := base64.StdEncoding.EncodeToString(responseBody)
			for _, marker := range []string{messageMarker, responseMarker, encoded} {
				if strings.Contains(result.log, marker) {
					t.Errorf("the log contains %q:\n%s", marker, result.log)
				}
			}
		})
	}
}

var errStdoutClosed = errors.New("stdout closed")

// failingWriter is standard output that rejects every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errStdoutClosed }

func TestRelayNoReplyFrame(t *testing.T) {
	valid := RelayOptions{Dial: failingDial, Log: io.Discard, Timeout: testTimeout}
	t.Run("length unreadable", func(t *testing.T) {
		var stdout bytes.Buffer
		err := Relay(context.Background(), bytes.NewReader([]byte{1, 2}), &stdout, valid)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("Relay error = %v, want io.ErrUnexpectedEOF", err)
		}
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want nothing", stdout.Bytes())
		}
	})
	t.Run("stdout fails", func(t *testing.T) {
		if err := Relay(context.Background(), message([]byte("{}")), failingWriter{}, valid); !errors.Is(err, errStdoutClosed) {
			t.Errorf("Relay error = %v, want the write failure", err)
		}
	})
	t.Run("incomplete options", func(t *testing.T) {
		tests := []struct {
			name string
			opts RelayOptions
		}{
			{"no Dial", RelayOptions{Log: io.Discard, Timeout: testTimeout}},
			{"no Log", RelayOptions{Dial: failingDial, Timeout: testTimeout}},
			{"no Timeout", RelayOptions{Dial: failingDial, Log: io.Discard}},
			{"negative Timeout", RelayOptions{Dial: failingDial, Log: io.Discard, Timeout: -time.Second}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				stdin := message([]byte("{}"))
				var stdout bytes.Buffer
				if err := Relay(context.Background(), stdin, &stdout, tt.opts); !errors.Is(err, errInvalidOptions) {
					t.Errorf("Relay error = %v, want errInvalidOptions", err)
				}
				if read := stdin.consumed(); read != 0 || stdout.Len() != 0 {
					t.Errorf("read %d bytes and wrote %d, want neither", read, stdout.Len())
				}
			})
		}
	})
}
