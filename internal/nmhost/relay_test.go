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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	// testKey is a key in the form the server accepts.
	testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
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
	contentLengthHeader                     []string // the raw Content-Length values, to tell 0 from absent
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
			method:              r.Method,
			requestURI:          r.RequestURI,
			expect:              r.Header.Get("Expect"),
			contentType:         r.Header.Get("Content-Type"),
			contentLengthHeader: r.Header.Values("Content-Length"),
			contentLength:       r.ContentLength,
			transferEncoding:    r.TransferEncoding,
			body:                data,
			requests:            got.requests + 1,
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
// The body is compared decoded and reported by its length only, so a failure
// with a body near the response limit stays readable.
func requireResponse(t *testing.T, members map[string]any, status int, body []byte) {
	t.Helper()
	wantMembers := map[string]any{"v": float64(protocolVersion), "kind": "response", "status": float64(status)}
	encoded, isString := members["body"].(string)
	others := maps.Clone(members)
	delete(others, "body")
	if !isString || !maps.Equal(others, wantMembers) {
		// Every member but the body is short; the body's type alone tells
		// whether it is present.
		t.Errorf("reply members = %v and a body of type %T, want %v and a string body", others, members["body"], wantMembers)
		return
	}
	got, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Errorf("the reply body of %d characters is not standard base64: %v", len(encoded), err)
		return
	}
	if !bytes.Equal(got, body) {
		t.Errorf("the reply body is %d bytes, want %d bytes; they differ from byte %d", len(got), len(body), firstDifference(got, body))
	}
}

// firstDifference returns the offset of the first byte where a and b differ,
// or the shorter length when one is a prefix of the other.
func firstDifference(a, b []byte) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

// statusMessage returns a new-form status request for testKey, a message
// the host sends to the server.
func statusMessage() []byte {
	return []byte(`{"v":1,"op":"status","key":"` + testKey + `"}`)
}

// startMessage returns a new-form generation start carrying input, which must
// be a JSON object.
func startMessage(input string) []byte {
	return []byte(`{"v":1,"op":"start","input":` + input + `}`)
}

// requireStage checks that the log names want as the stage the relay ended
// at. A test of a later stage would also pass with a message the form check
// stopped, so the form stage is ruled out explicitly when it is not want.
func requireStage(t *testing.T, log string, want stage) {
	t.Helper()
	if !strings.Contains(log, stageKey+"="+string(want)) {
		t.Errorf("the log does not name the %s stage:\n%s", want, log)
	}
	if want != stageForm && strings.Contains(log, stageKey+"="+string(stageForm)) {
		t.Errorf("the log names the form stage, want only %s:\n%s", want, log)
	}
}

// countedRaw is serveRaw that counts the requests whose head the server read.
// The count is taken in the server's goroutine, which a host that gave up
// waiting does not synchronize with, so the returned function waits up to
// testTimeout for the first request before reading the count.
func countedRaw(t *testing.T, serve func(net.Conn)) (Dialer, func() int32) {
	t.Helper()
	var n atomic.Int32
	path := serveRaw(t, func(conn net.Conn) {
		n.Add(1)
		serve(conn)
	})
	return SocketDialer(path), func() int32 {
		deadline := time.Now().Add(testTimeout)
		for n.Load() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		return n.Load()
	}
}

// TestRelayForwardsOneRequest checks what the per-operation tests leave out:
// the server's response bytes reach the reply unchanged even when they are not
// valid UTF-8, and the request target stays in origin form although the
// environment sets a proxy.
func TestRelayForwardsOneRequest(t *testing.T) {
	// The server's body is not valid UTF-8: the reply must carry its bytes,
	// not a repaired string.
	answer := []byte("{\"status\":\"ok\",\"state\":\"\xff\xc0\"}")
	path, got := recordingServer(t, answer)
	dial, dials := countDials(SocketDialer(path))

	result := runRelay(message(statusMessage()), dial, testTimeout)

	requireResponse(t, result.reply(t), http.StatusAccepted, answer)
	requireStage(t, result.log, stageExchange)
	req := got()
	// TestMain sets a proxy in the environment, so this checks that the
	// transport ignores it: through a proxy the target would be an absolute
	// URI.
	if req.requests != 1 || req.method != http.MethodGet || req.requestURI != "/jobs/"+testKey {
		t.Errorf("the server received %d requests, the last %s %s; want one GET /jobs/%s", req.requests, req.method, req.requestURI, testKey)
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
		stage     stage
	}{
		{"one byte over the limit", maxMessageBytes + 1, HostErrorMessageTooLarge, 0, stageLength},
		{"largest length", math.MaxUint32, HostErrorMessageTooLarge, 0, stageLength},
		// At the limit the host connects; the failing dial makes that visible
		// without sending 32 MiB.
		{"at the limit", maxMessageBytes, HostErrorServerUnavailable, 1, stageConnect},
		// The browser never sends an empty message; it is not "too large".
		{"zero length", 0, HostErrorProtocol, 0, stageLength},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only the start of the body is present; bodyRead shows whether
			// the host read any of it before deciding.
			stdin := newStdin(tt.length, []byte(`{"v":1,"op":"start","input":`))
			dial, dials := countDials(failingDial)
			result := runRelay(stdin, dial, testTimeout)
			requireError(t, result.reply(t), tt.want)
			requireStage(t, result.log, tt.stage)
			if n := dials.Load(); n != tt.wantDials {
				t.Errorf("Dial called %d times, want %d", n, tt.wantDials)
			}
			if n := stdin.bodyRead(); n != 0 {
				t.Errorf("read %d body bytes, want none", n)
			}
		})
	}
}

// TestRelayServerEarlyRejection checks a server that answers from the
// request head alone, as the server does for a Content-Length over its limit:
// the host relays that answer, not a write failure, and sends no body bytes,
// because the body waits for 100 Continue. It has still read the whole
// message from stdin, since it reads before sending.
func TestRelayServerEarlyRejection(t *testing.T) {
	rejected := []byte(`{"status":"rejected","reason":"body_too_large","requestId":"0123456789abcdef"}`)
	path := filepath.Join(udstest.Dir(t), "s.sock")
	listener := udstest.Listen(t, path)
	// sent receives the number of body bytes the host sent after the head.
	sent := make(chan int64, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		// The head is read through the same buffer the rest is counted
		// from, so no body byte can hide in it.
		br := bufio.NewReader(conn)
		if _, err := http.ReadRequest(br); err != nil {
			sent <- -1
			return
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 413 Request Entity Too Large\r\nConnection: close\r\n"+
			"Content-Length: "+strconv.Itoa(len(rejected))+"\r\n\r\n"+string(rejected))
		// The host closes the connection once it has the answer.
		n, _ := io.Copy(io.Discard, br)
		sent <- n
	}()
	// A generation start with a large input, whose body waits for 100
	// Continue.
	msg := startMessage(`{"transcript":"` + strings.Repeat("a", 64<<10) + `"}`)
	stdin := message(msg)

	result := runRelay(stdin, SocketDialer(path), testTimeout)
	requireResponse(t, result.reply(t), http.StatusRequestEntityTooLarge, rejected)
	requireStage(t, result.log, stageExchange)
	select {
	case n := <-sent:
		if n != 0 {
			t.Errorf("the host sent %d bytes after the request head, want none", n)
		}
	case <-time.After(testTimeout):
		t.Fatal("the server did not see the host close the connection")
	}
	if n := stdin.bodyRead(); n != len(msg) {
		t.Errorf("read %d body bytes, want the whole message of %d", n, len(msg))
	}
}

// TestRelayMessageReadFailures checks a message that ends early or stalls:
// the host has connected, reads the whole message before sending, and sends
// nothing.
func TestRelayMessageReadFailures(t *testing.T) {
	msg := startMessage(`{"videoUrl":"x","transcript":"y"}`)
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
			requireStage(t, result.log, stageRead)
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
			path, got := recordingServer(t, body)
			// A status request, whose response carries the largest article.
			// At the limit, the reply must also fit the browser's message
			// limit: encodeReply would turn a larger one into protocol_error.
			msg := []byte(`{"v":1,"op":"status","key":"` + testKey + `"}`)
			members := runRelay(message(msg), SocketDialer(path), testTimeout).reply(t)
			if req := got(); req.method != http.MethodGet || req.requestURI != "/jobs/"+testKey {
				t.Errorf("request line = %s %s, want the status request", req.method, req.requestURI)
			}
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
	msg := statusMessage()
	tests := []struct {
		name string
		// dial returns the dialer and, when a server answers on it, the
		// number of request heads that server read.
		dial    func(t *testing.T) (Dialer, func() int32)
		timeout time.Duration
		want    HostError
		stage   stage
		// unread is whether the body must be left unread: nothing was sent.
		unread bool
		// errno is the OS error the log must name, when one causes the
		// failure; it is how a user tells the connect failures apart.
		errno syscall.Errno
	}{
		{
			name: "no socket",
			dial: func(t *testing.T) (Dialer, func() int32) {
				return SocketDialer(filepath.Join(udstest.Dir(t), "absent.sock")), nil
			},
			want: HostErrorServerUnavailable, stage: stageConnect, unread: true, errno: syscall.ENOENT,
		},
		{
			name: "stale socket",
			dial: func(t *testing.T) (Dialer, func() int32) {
				path := filepath.Join(udstest.Dir(t), "stale.sock")
				listener, err := net.Listen("unix", path)
				if err != nil {
					t.Fatal(err)
				}
				listener.(*net.UnixListener).SetUnlinkOnClose(false)
				_ = listener.Close()
				return SocketDialer(path), nil
			},
			want: HostErrorServerUnavailable, stage: stageConnect, unread: true, errno: syscall.ECONNREFUSED,
		},
		{
			name: "directory without permission",
			dial: func(t *testing.T) (Dialer, func() int32) {
				skipIfRoot(t)
				dir := filepath.Join(udstest.Dir(t), "locked")
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "s.sock")
				udstest.Listen(t, path)
				chmodForTest(t, dir, 0)
				return SocketDialer(path), nil
			},
			want: HostErrorServerUnavailable, stage: stageConnect, unread: true, errno: syscall.EACCES,
		},
		{
			name: "connect attempt outlasts the limit",
			dial: func(*testing.T) (Dialer, func() int32) {
				return func(ctx context.Context) (net.Conn, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				}, nil
			},
			timeout: shortTimeout,
			want:    HostErrorServerUnavailable, stage: stageConnect, unread: true,
		},
		{
			name: "server never answers",
			dial: func(t *testing.T) (Dialer, func() int32) {
				return countedRaw(t, func(conn net.Conn) { _, _ = io.Copy(io.Discard, conn) })
			},
			timeout: shortTimeout,
			want:    HostErrorServerTimeout, stage: stageExchange,
		},
		{
			// The transport reports the broken write itself and the relay's
			// context is still live, so only the os.ErrDeadlineExceeded
			// check makes this a timeout.
			name: "write reports a connection deadline",
			dial: func(*testing.T) (Dialer, func() int32) {
				return func(context.Context) (net.Conn, error) { return newDeadlineConn(), nil }, nil
			},
			want: HostErrorServerTimeout, stage: stageExchange,
		},
		{
			name: "response body stalls",
			dial: func(t *testing.T) (Dialer, func() int32) {
				return countedRaw(t, func(conn net.Conn) {
					_, _ = io.WriteString(conn, "HTTP/1.1 202 Accepted\r\nContent-Length: 100\r\n\r\n{\"status\"")
					_, _ = io.Copy(io.Discard, conn)
				})
			},
			timeout: shortTimeout,
			want:    HostErrorServerTimeout, stage: stageExchange,
		},
		{
			name: "response cut short",
			dial: func(t *testing.T) (Dialer, func() int32) {
				return countedRaw(t, func(conn net.Conn) {
					_, _ = io.WriteString(conn, "HTTP/1.1 202 Accepted\r\nContent-Length: 100\r\n\r\n{\"status\"")
				})
			},
			want: HostErrorProtocol, stage: stageExchange,
		},
		{
			name: "response not HTTP",
			dial: func(t *testing.T) (Dialer, func() int32) {
				return countedRaw(t, func(conn net.Conn) { _, _ = io.WriteString(conn, "not http\r\n\r\n") })
			},
			want: HostErrorProtocol, stage: stageExchange,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDial, requests := tt.dial(t)
			dial, dials := countDials(baseDial)
			stdin := message(msg)
			timeout := cmp.Or(tt.timeout, testTimeout)

			result := runRelay(stdin, dial, timeout)
			requireError(t, result.reply(t), tt.want)
			requireStage(t, result.log, tt.stage)
			if requests != nil {
				if n := requests(); n != 1 {
					t.Errorf("the server read %d requests, want 1", n)
				}
			}
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
	msg := startMessage(`{"videoUrl":"x","transcript":"` + messageMarker + `"}`)
	responseBody := []byte(`{"key":"` + responseMarker + `"}`)
	tests := []struct {
		name string
		// dial returns the dialer and the number of requests its server read.
		dial  func(t *testing.T) (Dialer, func() int32)
		check func(t *testing.T, members map[string]any)
	}{
		{
			name: "response",
			dial: func(t *testing.T) (Dialer, func() int32) {
				path, got := recordingServer(t, responseBody)
				return SocketDialer(path), func() int32 { return int32(got().requests) } //nolint:gosec // a request count
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
			dial: func(t *testing.T) (Dialer, func() int32) {
				return countedRaw(t, func(conn net.Conn) { _, _ = io.WriteString(conn, responseMarker+"\r\n\r\n") })
			},
			check: func(t *testing.T, members map[string]any) { requireError(t, members, HostErrorProtocol) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dial, requests := tt.dial(t)
			result := runRelay(message(msg), dial, testTimeout)
			tt.check(t, result.reply(t))
			requireStage(t, result.log, stageExchange)
			if n := requests(); n != 1 {
				t.Errorf("the server read %d requests, want 1", n)
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
		var log bytes.Buffer
		opts := valid
		opts.Log = &log
		if err := Relay(context.Background(), message(statusMessage()), failingWriter{}, opts); !errors.Is(err, errStdoutClosed) {
			t.Errorf("Relay error = %v, want the write failure", err)
		}
		if !strings.Contains(log.String(), stageKey+"="+string(stageReply)) {
			t.Errorf("the log does not name the reply stage:\n%s", log.String())
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
				stdin := message(statusMessage())
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

// newFormMessage marshals members as one message. A map marshals its keys in
// sorted order, which the host must not depend on.
func newFormMessage(t *testing.T, members map[string]any) []byte {
	t.Helper()
	msg, err := json.Marshal(members)
	if err != nil {
		t.Fatalf("marshal the message: %v", err)
	}
	return msg
}

func TestRelayNewFormOperations(t *testing.T) {
	const key = testKey
	// The input keeps whitespace, an escape, an undeclared member, and a
	// member order no marshaller produces, so a host that rebuilt it would
	// change its bytes.
	input := `{ "transcript" : "\u3042" ,"videoUrl":"x","extra":[1]}`
	tests := []struct {
		name, msg, op       string
		method, requestURI  string
		body                string
		contentType, expect string
		// contentLength is the raw Content-Length header, "" for none.
		contentLength string
	}{
		{
			name: "start", msg: `{"v":1,"op":"start","input":` + input + `}`, op: "start",
			method: http.MethodPost, requestURI: "/jobs", body: input,
			contentType: "application/json", expect: "100-continue", contentLength: strconv.Itoa(len(input)),
		},
		{
			name: "status", msg: `{"key":"` + key + `","op":"status","v":1}`, op: "status",
			method: http.MethodGet, requestURI: "/jobs/" + key,
		},
		{
			name: "publish", msg: `{"v":1,"op":"publish","key":"` + key + `"}`, op: "publish",
			method: http.MethodPost, requestURI: "/jobs/" + key + "/publish",
			contentLength: "0",
		},
		{
			// The version member's name is written with an escape.
			name: "escaped version name", msg: `{"\u0076":1,"op":"status","key":"` + key + `"}`, op: "status",
			method: http.MethodGet, requestURI: "/jobs/" + key,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer := []byte(`{"status":"ok"}`)
			path, got := recordingServer(t, answer)
			dial, dials := countDials(SocketDialer(path))

			result := runRelay(message([]byte(tt.msg)), dial, testTimeout)
			requireResponse(t, result.reply(t), http.StatusAccepted, answer)
			req := got()
			if req.requests != 1 || dials.Load() != 1 {
				t.Errorf("the server received %d requests over %d dials, want 1 over 1", req.requests, dials.Load())
			}
			if req.method != tt.method || req.requestURI != tt.requestURI {
				t.Errorf("request line = %s %s, want %s %s", req.method, req.requestURI, tt.method, tt.requestURI)
			}
			if string(req.body) != tt.body || req.contentLength != int64(len(tt.body)) || len(req.transferEncoding) != 0 {
				t.Errorf("body = %q (Content-Length %d, Transfer-Encoding %v), want %q", req.body, req.contentLength, req.transferEncoding, tt.body)
			}
			if req.contentType != tt.contentType || req.expect != tt.expect {
				t.Errorf("Content-Type = %q, Expect = %q; want %q, %q", req.contentType, req.expect, tt.contentType, tt.expect)
			}
			if got := strings.Join(req.contentLengthHeader, ","); got != tt.contentLength {
				t.Errorf("Content-Length header = %q, want %q", got, tt.contentLength)
			}
			if !strings.Contains(result.log, "op="+tt.op) {
				t.Errorf("the log does not contain op=%s:\n%s", tt.op, result.log)
			}
			requireNoFormAttr(t, result.log)
		})
	}
}

// requireNoFormAttr checks that the log has no form attribute: the host has
// one message form, so such an attribute would tell nothing apart.
func requireNoFormAttr(t *testing.T, log string) {
	t.Helper()
	if strings.Contains(log, "form=") {
		t.Errorf("the log has a form attribute:\n%s", log)
	}
}

// TestRelayRejectsOtherForms checks the messages that are not the new form:
// those whose top level has no "v", however complete the rest is, and those
// that are not one strict JSON object even when they hold what looks like a
// new-form message. Each is answered with protocol_error and never sent,
// though the server is reachable, and the log records only the form stage and
// the rule, never the message's members or values.
func TestRelayRejectsOtherForms(t *testing.T) {
	const marker = "marker-3b7c"
	tests := []struct {
		name, msg string
		rule      rule
	}{
		// The message an extension built before the job API sends.
		{"extension message without v", `{"videoUrl":"` + marker + `","transcript":"y","title":"t","channelName":"c","description":"d"}`, ruleVersion},
		{"every member but v", `{"op":"status","key":"` + marker + `"}`, ruleVersion},
		{"v only below the top level", `{"videoUrl":"x","transcript":{"v":1,"op":"status","key":"` + marker + `"}}`, ruleVersion},
		{"invalid UTF-8", "{\"v\":1,\"op\":\"status\",\"key\":\"" + marker + "\xff\"}", ruleObject},
		{"not JSON", `v=1&op=status&key=` + marker, ruleObject},
		{"top level not an object", `[{"v":1,"op":"status","key":"` + marker + `"}]`, ruleObject},
		{"trailing data", `{"v":1,"op":"status","key":"` + marker + `"} {}`, ruleObject},
		{"unpaired surrogate", `{"v":1,"op":"status","key":"` + marker + `\ud800"}`, ruleObject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, got := recordingServer(t, []byte("{}"))
			dial, dials := countDials(SocketDialer(path))
			result := runRelay(message([]byte(tt.msg)), dial, testTimeout)
			requireError(t, result.reply(t), HostErrorProtocol)
			if n, d := got().requests, dials.Load(); n != 0 || d != 1 {
				t.Errorf("the server received %d requests over %d dials, want none over 1", n, d)
			}
			for _, attr := range []string{"stage=form", "rule=" + string(tt.rule)} {
				if !strings.Contains(result.log, attr) {
					t.Errorf("the log does not contain %s:\n%s", attr, result.log)
				}
			}
			// The rule is decided before "op" is read, so no operation is
			// logged either.
			for _, leak := range []string{marker, "op=", "videoUrl", "status"} {
				if strings.Contains(result.log, leak) {
					t.Errorf("the log contains %q:\n%s", leak, result.log)
				}
			}
			requireNoFormAttr(t, result.log)
		})
	}
}

func TestRelayNewFormRejections(t *testing.T) {
	tests := []struct {
		name, msg string
		rule      rule
	}{
		{"unknown version", `{"v":2,"op":"status","key":"k"}`, ruleVersion},
		{"version zero", `{"v":0,"op":"status","key":"k"}`, ruleVersion},
		{"version as a string", `{"v":"1","op":"status","key":"k"}`, ruleVersion},
		{"version with a fraction", `{"v":1.0,"op":"status","key":"k"}`, ruleVersion},
		{"version null", `{"v":null,"op":"status","key":"k"}`, ruleVersion},
		{"escaped version name, unknown version", `{"\u0076":2,"op":"status","key":"k"}`, ruleVersion},
		{"duplicate version", `{"v":1,"v":1,"op":"status","key":"k"}`, ruleMembers},
		{"duplicate version, one escaped", `{"v":1,"\u0076":1,"op":"status","key":"k"}`, ruleMembers},
		{"duplicate operation", `{"v":1,"op":"status","op":"status","key":"k"}`, ruleMembers},
		{"undeclared member", `{"v":1,"op":"status","key":"k","path":"/articles"}`, ruleMembers},
		{"operation missing", `{"v":1,"key":"k"}`, ruleOperation},
		{"operation not in the set", `{"v":1,"op":"delete","key":"k"}`, ruleOperation},
		{"operation in another case", `{"v":1,"op":"Status","key":"k"}`, ruleOperation},
		{"operation not a string", `{"v":1,"op":1,"key":"k"}`, ruleOperation},
		{"start without input", `{"v":1,"op":"start"}`, ruleMembers},
		{"start with a key", `{"v":1,"op":"start","input":{},"key":"k"}`, ruleMembers},
		{"start input not an object", `{"v":1,"op":"start","input":"{}"}`, ruleInput},
		{"start input null", `{"v":1,"op":"start","input":null}`, ruleInput},
		{"status without key", `{"v":1,"op":"status"}`, ruleMembers},
		{"status with input", `{"v":1,"op":"status","key":"k","input":{}}`, ruleMembers},
		{"publish without key", `{"v":1,"op":"publish"}`, ruleMembers},
		{"publish key not a string", `{"v":1,"op":"publish","key":1}`, ruleKey},
		{"status key null", `{"v":1,"op":"status","key":null}`, ruleKey},
		// The version is checked before the members, so a message with both
		// faults breaks the version rule.
		{"no version, undeclared member", `{"op":"status","key":"k","extra":1}`, ruleVersion},
		{"unknown version, undeclared member", `{"v":2,"op":"status","key":"k","extra":1}`, ruleVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, got := recordingServer(t, []byte("{}"))
			dial, dials := countDials(SocketDialer(path))
			result := runRelay(message([]byte(tt.msg)), dial, testTimeout)
			requireError(t, result.reply(t), HostErrorProtocol)
			// The host connects before it reads the message, so it has
			// connected once; it sends nothing over the connection.
			if n, d := got().requests, dials.Load(); n != 0 || d != 1 {
				t.Errorf("the server received %d requests over %d dials, want none over 1", n, d)
			}
			for _, attr := range []string{"stage=form", "rule=" + string(tt.rule)} {
				if !strings.Contains(result.log, attr) {
					t.Errorf("the log does not contain %s:\n%s", attr, result.log)
				}
			}
			requireNoFormAttr(t, result.log)
		})
	}
}

// TestRelayKeyPassedUnchecked checks that any string key reaches the server
// unchanged, escaped into exactly one path segment: a key can neither turn a
// status request into another operation nor start a query.
func TestRelayKeyPassedUnchecked(t *testing.T) {
	keys := map[string]string{
		"empty":           "",
		"uppercase":       "ABC",
		"long":            strings.Repeat("f", 1000),
		"publish suffix":  "abc/publish",
		"query":           "?x=1",
		"query inside":    "a?x=1",
		"dot segment":     "..",
		"escaped slash":   "%2F",
		"control":         "\x01",
		"non-ASCII":       "\u3042",
		"space and hash":  "a b#c",
		"sub-delimiters":  "a;b,c:d@e&f=g+h$",
		"parent and root": "../../articles",
	}
	for name, key := range keys {
		for _, op := range []struct{ name, method, suffix string }{
			{"status", http.MethodGet, ""},
			{"publish", http.MethodPost, "/publish"},
		} {
			t.Run(name+"/"+op.name, func(t *testing.T) {
				path, got := recordingServer(t, []byte("{}"))
				msg := newFormMessage(t, map[string]any{"v": 1, "op": op.name, "key": key})
				requireResponse(t, runRelay(message(msg), SocketDialer(path), testTimeout).reply(t), http.StatusAccepted, []byte("{}"))

				req := got()
				if req.requests != 1 || req.method != op.method {
					t.Fatalf("the server received %d requests, the last %s; want one %s", req.requests, req.method, op.method)
				}
				segment, ok := strings.CutPrefix(req.requestURI, "/jobs/")
				if ok {
					segment, ok = strings.CutSuffix(segment, op.suffix)
				}
				if !ok || strings.ContainsAny(segment, "/?") {
					t.Fatalf("request target = %q, want /jobs/, one path segment, then %q", req.requestURI, op.suffix)
				}
				if decoded, err := url.PathUnescape(segment); err != nil || decoded != key {
					t.Errorf("the segment %q decodes to %q (%v), want the key %q", segment, decoded, err, key)
				}
				// The client sends the escaped form as is: it neither
				// normalizes the path nor escapes it again.
				if want := "/jobs/" + url.PathEscape(key) + op.suffix; req.requestURI != want {
					t.Errorf("request target = %q, want %q", req.requestURI, want)
				}
			})
		}
	}
}

// TestRelayNewFormTimeout checks that a new-form request the server never
// answers ends with server_timeout after exactly one request.
func TestRelayNewFormTimeout(t *testing.T) {
	const key = testKey
	for _, msg := range []string{
		`{"v":1,"op":"start","input":{"videoUrl":"x","transcript":"y"}}`,
		`{"v":1,"op":"status","key":"` + key + `"}`,
		`{"v":1,"op":"publish","key":"` + key + `"}`,
	} {
		t.Run(msg, func(t *testing.T) {
			var requests atomic.Int32
			path := serveRaw(t, func(conn net.Conn) {
				requests.Add(1)
				_, _ = io.Copy(io.Discard, conn)
			})
			dial, dials := countDials(SocketDialer(path))
			requireError(t, runRelay(message([]byte(msg)), dial, shortTimeout).reply(t), HostErrorServerTimeout)
			if n, d := requests.Load(), dials.Load(); n != 1 || d != 1 {
				t.Errorf("the server read %d requests over %d dials, want 1 over 1", n, d)
			}
		})
	}
}

// TestRelayNewFormLogOmitsMessage checks that the new form's log attributes
// are fixed values: no key, operation, member name, or input reaches the log,
// whether the message is sent or rejected.
func TestRelayNewFormLogOmitsMessage(t *testing.T) {
	const marker = "marker-5d1e"
	// want is a fixed value the log must hold, showing the relay got as far
	// as the row means it to.
	tests := []struct{ name, msg, want string }{
		{"start input", `{"v":1,"op":"start","input":{"transcript":"` + marker + `","` + marker + `":1}}`, "op=start"},
		{"status key", `{"v":1,"op":"status","key":"` + marker + `"}`, "op=status"},
		{"publish key", `{"v":1,"op":"publish","key":"` + marker + `"}`, "op=publish"},
		{"unknown operation", `{"v":1,"op":"` + marker + `","key":"k"}`, "rule=operation"},
		{"undeclared member", `{"v":1,"op":"status","key":"k","` + marker + `":1}`, "rule=members"},
		{"key not a string", `{"v":1,"op":"status","key":{"` + marker + `":1}}`, "rule=key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, _ := recordingServer(t, []byte("{}"))
			result := runRelay(message([]byte(tt.msg)), SocketDialer(path), testTimeout)
			result.reply(t)
			if !strings.Contains(result.log, tt.want) {
				t.Fatalf("the log does not contain %s:\n%s", tt.want, result.log)
			}
			requireNoFormAttr(t, result.log)
			if strings.Contains(result.log, marker) {
				t.Errorf("the log contains the message's content:\n%s", result.log)
			}
		})
	}
}

// TestRequestRefusesUnroutable checks the fail-secure default of request: a
// destination whose operation route never returns, the zero value included,
// builds no request, whatever its body and key.
func TestRequestRefusesUnroutable(t *testing.T) {
	for _, dest := range []destination{
		{},
		{body: []byte("{}")},
		{key: "k"},
		{op: OperationPublish + 1, key: "k"},
		{op: -1, body: []byte("{}")},
	} {
		if req, err := dest.request(context.Background()); req != nil || !errors.Is(err, errUnroutable) {
			t.Errorf("request(%+v) = %v, %v; want no request and errUnroutable", dest, req, err)
		}
	}
}
