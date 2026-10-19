package nmhost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
)

// Dialer opens the one connection to the server. Production passes
// SocketDialer with the socket path from config.LoadHost; a test passes a
// fake or a temporary socket.
type Dialer func(ctx context.Context) (net.Conn, error)

// RelayTimeout bounds one relay, from the length read to the end of the
// server's reply. It cannot cut short a read of stdin that stalls mid-body:
// the transport waits for its body read to return. The browser writes the
// whole message at once and closes stdin when it gives up, so that read ends.
const RelayTimeout = 20 * time.Second

const (
	// maxMessageBytes is the host's request limit. It is above the server's
	// 4 MiB body limit, so a body the extension measured as within 4 MiB is
	// always the server's to reject, and below the 64 MiB the browser can
	// send to a host.
	maxMessageBytes = 32 << 20
	// maxResponseBytes is the largest server response body the host relays.
	maxResponseBytes = 64 << 10
	// dialTimeout bounds the connection attempt. A Unix socket with a
	// listener accepts at once and one without refuses at once; the limit is
	// only for an abnormal backlog.
	dialTimeout = 2 * time.Second
	// protocolVersion is the reply's "v" member.
	protocolVersion = 1
	// requestURL is the request target. The host is a fixed name the server
	// does not check; the connection is the transport's only dial.
	requestURL = "http://yt2column/articles"
)

// errInvalidOptions reports a RelayOptions with a missing field.
var errInvalidOptions = errors.New("nmhost: every RelayOptions field is required")

// errRedial reports a second dial on the one-use transport. The transport
// never redials on its own for a request whose body cannot be replayed; this
// makes that structural.
var errRedial = errors.New("nmhost: the relay connection was already used")

// HostError is one of the host's fixed errors. The zero value is
// HostErrorProtocol, which the extension shows as "result unknown", so an unset
// value never tells the user the request was not sent.
type HostError int

// The host's fixed errors; the comment on each is its wire value.
const (
	HostErrorProtocol          HostError = iota // "protocol_error"
	HostErrorServerUnavailable                  // "server_unavailable"
	HostErrorServerTimeout                      // "server_timeout"
	HostErrorMessageTooLarge                    // "message_too_large"
)

// Value returns the fixed wire value. Every value outside the four constants
// returns "protocol_error".
func (e HostError) Value() string {
	switch e {
	case HostErrorServerUnavailable:
		return "server_unavailable"
	case HostErrorServerTimeout:
		return "server_timeout"
	case HostErrorMessageTooLarge:
		return "message_too_large"
	default: // HostErrorProtocol and every value outside the constants
		return "protocol_error"
	}
}

// The two shapes of the host's reply. Each is marshalled from its own struct,
// so a reply never carries a member of the other shape.
type responseReply struct {
	V      int    `json:"v"`      // always protocolVersion
	Kind   string `json:"kind"`   // always "response"
	Status int    `json:"status"` // the server's HTTP status code
	Body   string `json:"body"`   // the server's response body, standard base64
}

type errorReply struct {
	V     int    `json:"v"`     // always protocolVersion
	Kind  string `json:"kind"`  // always "error"
	Error string `json:"error"` // HostError.Value()
}

func newResponseReply(status int, body []byte) responseReply {
	return responseReply{
		V:      protocolVersion,
		Kind:   "response",
		Status: status,
		Body:   base64.StdEncoding.EncodeToString(body),
	}
}

func newErrorReply(e HostError) errorReply {
	return errorReply{V: protocolVersion, Kind: "error", Error: e.Value()}
}

// encodeReply marshals reply. A reply over the browser's message limit, which
// the response limit keeps out of reach, becomes the protocol_error reply.
func encodeReply(reply any) []byte {
	payload, err := json.Marshal(reply)
	if err != nil || len(payload) > maxFrameBytes {
		// Neither happens: both shapes are fixed structs of strings and
		// ints, and the largest response reply is about 88 KiB.
		payload, _ = json.Marshal(newErrorReply(HostErrorProtocol)) //nolint:errchkjson // an int and two strings always marshal
	}
	return payload
}

// SocketDialer returns the Dialer that connects to the Unix socket at path,
// giving up after the connect limit.
func SocketDialer(path string) Dialer {
	return func(ctx context.Context) (net.Conn, error) {
		d := net.Dialer{Timeout: dialTimeout}
		return d.DialContext(ctx, "unix", path)
	}
}

// RelayOptions holds the relay's seams. Every field is required.
type RelayOptions struct {
	Dial    Dialer
	Log     io.Writer     // standard error in production
	Timeout time.Duration // RelayTimeout in production; a test passes a short value
}

func (o RelayOptions) validate() error {
	if o.Dial == nil || o.Log == nil || o.Timeout <= 0 {
		return errInvalidOptions
	}
	return nil
}

// stageKey is the log attribute that names the stage.
const stageKey = "stage"

// stage is where a relay ended, for the log only; the host error is chosen by
// control flow, not from this value.
type stage string

const (
	stageLength   stage = "length"   // before connecting: the length decided it
	stageConnect  stage = "connect"  // the connection attempt failed
	stageExchange stage = "exchange" // after connecting
	stageReply    stage = "reply"    // writing the reply frame
)

// outcome is the result of one relay: the reply to write and the fixed values
// the log records about it.
type outcome struct {
	reply any
	attrs []any
}

func hostFailure(s stage, e HostError, cause error) outcome {
	attrs := []any{stageKey, s, "error", e.Value()}
	if text := errnoText(cause); text != "" {
		attrs = append(attrs, "errno", text)
	}
	return outcome{reply: newErrorReply(e), attrs: attrs}
}

// Relay reads one Native Messaging message from stdin, forwards it to the
// server, and writes exactly one reply frame to stdout without reading the
// rest of stdin. It returns a non-nil error only when no reply frame could
// be written (the options are incomplete, the length could not be read, or
// stdout failed); a host error is a reply, not a returned error.
//
// Contract: stdout receives nothing but that one frame, and the log never
// receives the message or the server's response, only fixed values.
func Relay(ctx context.Context, stdin io.Reader, stdout io.Writer, opts RelayOptions) error {
	if err := opts.validate(); err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(opts.Log, nil))
	length, err := readLength(stdin)
	if err != nil {
		log.Error("relay", stageKey, stageLength, "failure", "length unreadable")
		return fmt.Errorf("read the message length: %w", err)
	}
	start := time.Now()
	result := forward(ctx, stdin, length, opts)
	log.Info("relay", append(result.attrs, "length", length, "elapsed", time.Since(start).Round(time.Millisecond))...)
	if err := writeFrame(stdout, encodeReply(result.reply)); err != nil {
		attrs := []any{stageKey, stageReply, "failure", "stdout write failed"}
		if text := errnoText(err); text != "" {
			attrs = append(attrs, "errno", text)
		}
		log.Error("relay", attrs...)
		return err
	}
	return nil
}

// forward relays a message of the given length.
//
// Contract: the host error is chosen by the stage reached, by control flow:
// nothing is sent before the connection returns, so every failure up to it
// is "not sent", and every failure after it may have reached the server. The
// message is never retried, and stdin is read only while the transport sends
// the body.
func forward(ctx context.Context, stdin io.Reader, length uint32, opts RelayOptions) outcome {
	if length > maxMessageBytes {
		return hostFailure(stageLength, HostErrorMessageTooLarge, nil)
	}
	if length == 0 {
		return hostFailure(stageLength, HostErrorProtocol, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	conn, err := opts.Dial(ctx)
	if err != nil {
		return hostFailure(stageConnect, HostErrorServerUnavailable, err)
	}
	defer func() { _ = conn.Close() }()

	transport := oneUseTransport(conn, opts.Timeout)
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, io.LimitReader(stdin, int64(length)))
	if err != nil {
		return hostFailure(stageExchange, HostErrorProtocol, nil)
	}
	req.ContentLength = int64(length)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Expect", "100-continue")
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return exchangeFailure(ctx, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return exchangeFailure(ctx, err)
	}
	if len(body) > maxResponseBytes {
		return hostFailure(stageExchange, HostErrorProtocol, nil)
	}
	return outcome{
		reply: newResponseReply(resp.StatusCode, body),
		attrs: []any{stageKey, stageExchange, "status", resp.StatusCode},
	}
}

// exchangeFailure classifies a failure after the connection. A deadline takes
// precedence: once the relay's deadline has passed, every failure is a
// timeout, whatever error the transport reports. The transport may report the
// broken write instead of the deadline when both happen at once, so the error
// alone does not decide it. A connection's own deadline error is a timeout
// too.
func exchangeFailure(ctx context.Context, err error) outcome {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return hostFailure(stageExchange, HostErrorServerTimeout, err)
	}
	return hostFailure(stageExchange, HostErrorProtocol, err)
}

// oneUseTransport returns a transport for exactly one request over conn.
//
// Contract: DialContext hands out conn once and fails every later call, so
// the transport can neither reconnect nor resend; no proxy from the
// environment is used (a proxy would turn the request target into an
// absolute URI the server does not route); the body waits for 100 Continue
// for as long as the relay may last.
func oneUseTransport(conn net.Conn, timeout time.Duration) *http.Transport {
	handed := make(chan net.Conn, 1)
	handed <- conn
	return &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			select {
			case c := <-handed:
				return c, nil
			default:
				return nil, errRedial
			}
		},
		Proxy:                 nil,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
		ExpectContinueTimeout: timeout,
	}
}

// errnoText returns the fixed text of the OS error number in err's chain, or
// "" when there is none. Only this text is logged: other error text can quote
// bytes the server sent.
func errnoText(err error) string {
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		return errno.Error()
	}
	return ""
}
