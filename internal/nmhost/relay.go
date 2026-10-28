package nmhost

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"

	"github.com/isseis/yt2column/internal/strictjson"
)

// Dialer opens the one connection to the server. Production passes
// SocketDialer with the socket path from config.LoadHost; a test passes a
// fake or a temporary socket.
type Dialer func(ctx context.Context) (net.Conn, error)

// RelayTimeout bounds one relay, from the length read to the end of the
// server's reply. Reading the message counts against it, so a stdin that
// stalls mid-message ends the relay with server_timeout; the browser writes
// the whole message at once, so the read normally ends at once.
const RelayTimeout = 20 * time.Second

const (
	// maxMessageBytes is the host's request limit. It is above the server's
	// 4 MiB body limit, so a body the extension measured as within 4 MiB is
	// always the server's to reject, and below the 64 MiB the browser can
	// send to a host.
	maxMessageBytes = 32 << 20
	// maxResponseBytes is the largest server response body the host relays.
	// It is twice the server's 256 KiB status response limit, so a status
	// response with the largest article is relayed whole, and its base64
	// with the fixed members stays within maxFrameBytes.
	maxResponseBytes = 512 << 10
	// dialTimeout bounds the connection attempt. A Unix socket with a
	// listener accepts at once and one without refuses at once; the limit is
	// only for an abnormal backlog.
	dialTimeout = 2 * time.Second
	// protocolVersion is the reply's "v" member.
	protocolVersion = 1
	// serverURL is the scheme and host of every request target. The host is
	// a fixed name the server does not check; the connection is the
	// transport's only dial.
	serverURL = "http://yt2column"
	// articlesURL is the target of a message in the current form.
	articlesURL = serverURL + "/articles"
	// jobsURL is the target of a generation start; a status request appends
	// a slash and the escaped key, and a publish operation then publishSuffix.
	jobsURL       = serverURL + "/jobs"
	publishSuffix = "/publish"
	// newFormVersion is the only "v" value a new-form message may carry. It
	// is independent of protocolVersion, the reply's "v".
	newFormVersion = 1
)

// errUnroutable reports a destination route never returns.
var errUnroutable = errors.New("nmhost: the destination has no route")

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
		// ints, and the largest response reply is about 683 KiB.
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
	stageRead     stage = "read"     // after connecting: reading the message failed
	stageForm     stage = "form"     // a new-form message broke a rule; nothing was sent
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
// server, and writes exactly one reply frame to stdout. It reads nothing from
// stdin beyond that one message. It returns a non-nil error only when no reply frame could
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
// whole message is read after connecting and before sending, within the relay
// limit: a failed connection leaves stdin unread, and a message that ends
// early or stalls is never sent. The message is never retried.
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

	msg, err := readMessage(ctx, stdin, length)
	if err != nil {
		return readFailure(ctx, err)
	}
	dest, broken := route(msg)
	if broken != "" {
		result := hostFailure(stageForm, HostErrorProtocol, nil)
		result.attrs = append(result.attrs, dest.logAttrs()...)
		result.attrs = append(result.attrs, "rule", broken)
		return result
	}
	req, err := dest.request(ctx)
	if err != nil {
		return hostFailure(stageExchange, HostErrorProtocol, nil)
	}
	result := exchange(ctx, conn, opts.Timeout, req)
	result.attrs = append(result.attrs, dest.logAttrs()...)
	return result
}

// readMessage reads the message's length bytes from stdin.
//
// Contract: it returns by the end of ctx even when stdin stalls. The read
// left waiting then ends with the process, which exits after its reply.
func readMessage(ctx context.Context, stdin io.Reader, length uint32) ([]byte, error) {
	type result struct {
		msg []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		msg := make([]byte, length)
		_, err := io.ReadFull(stdin, msg)
		done <- result{msg: msg, err: err}
	}()
	select {
	case r := <-done:
		return r.msg, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// readFailure classifies a failed message read. Nothing was sent, but the
// connection was open, so the rule after connecting applies: a passed
// deadline is a timeout, and every other failure is a protocol error.
func readFailure(ctx context.Context, err error) outcome {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return hostFailure(stageRead, HostErrorServerTimeout, err)
	}
	return hostFailure(stageRead, HostErrorProtocol, err)
}

// exchange sends req over conn, exactly once, and reads the server's reply.
func exchange(ctx context.Context, conn net.Conn, timeout time.Duration, req *http.Request) outcome {
	transport := oneUseTransport(conn, timeout)
	defer transport.CloseIdleConnections()
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

// Operation is the operation a new-form message names. The zero value is
// OperationUnknown, which is rejected without sending.
type Operation int

// The operations; the comment on each is its request.
const (
	OperationUnknown Operation = iota
	OperationStart             // POST /jobs
	OperationStatus            // GET /jobs/{key}
	OperationPublish           // POST /jobs/{key}/publish
)

// The "op" values of a new-form message, which are also the log's "op"
// values.
const (
	opStart   = "start"
	opStatus  = "status"
	opPublish = "publish"
)

// parseOperation returns the operation an "op" value names, and
// OperationUnknown for every other value.
func parseOperation(op string) Operation {
	switch op {
	case opStart:
		return OperationStart
	case opStatus:
		return OperationStatus
	case opPublish:
		return OperationPublish
	default:
		return OperationUnknown
	}
}

// logValue returns the fixed log value of o, and "" for OperationUnknown
// and every value outside the constants.
func (o Operation) logValue() string {
	switch o {
	case OperationStart:
		return opStart
	case OperationStatus:
		return opStatus
	case OperationPublish:
		return opPublish
	default: // OperationUnknown and every value outside the constants
		return ""
	}
}

// form is a message's shape, which route decides; request dispatches on it,
// and the log records it.
type form string

const (
	formLegacy form = "legacy" // the current extension's message, sent unchanged
	formNew    form = "new"    // a message carrying "v"
)

// rule names the new-form rule a message broke, for the log only. The empty
// value means no rule was broken.
type rule string

const (
	ruleVersion   rule = "version"   // "v" is not newFormVersion
	ruleOperation rule = "operation" // "op" is missing, not a string, or unknown
	ruleMembers   rule = "members"   // an unlisted, duplicate, missing, or extra member
	ruleInput     rule = "input"     // "input" is not a JSON object
	ruleKey       rule = "key"       // "key" is not a JSON string
)

// destination is where one message goes, as route decides it. The zero value
// has no form, and request refuses it.
type destination struct {
	form form
	op   Operation
	body []byte // the request body: the message, or a start's "input"
	key  string // the key of a status request or a publish operation
}

// route decides the destination of msg.
//
// Contract: this is the one place the host chooses behavior from the content
// of the message (an exception to "Declare, don't infer" recorded in
// CLAUDE.md, because the current extension declares nothing). A message that
// is not one strict JSON object, or whose top level has no "v" member (escapes
// decoded), is the current form, sent unchanged; nothing else in it is parsed.
// A message with "v" is the new form, checked strictly: when it breaks a rule,
// route returns that rule and the message is never sent, and never retried as
// the current form. Inside the new form, the operation is the declared "op".
func route(msg []byte) (destination, rule) {
	object, err := strictjson.ParseObject(msg)
	if err != nil || !object.Has("v") {
		return destination{form: formLegacy, body: msg}, ""
	}
	dest := destination{form: formNew}
	members, err := object.CollectOnly("v", "op", "input", "key")
	if err != nil {
		return dest, ruleMembers
	}
	if v, err := members["v"].AsNonNegativeInt64(); err != nil || v != newFormVersion {
		return dest, ruleVersion
	}
	op, err := members["op"].AsString()
	if err != nil {
		return dest, ruleOperation
	}
	dest.op = parseOperation(op)
	input, hasInput := members["input"]
	key, hasKey := members["key"]
	switch dest.op {
	case OperationStart:
		if !hasInput || hasKey {
			return dest, ruleMembers
		}
		if _, err := input.AsObject(); err != nil {
			return dest, ruleInput
		}
		dest.body = input.Raw()
	case OperationStatus, OperationPublish:
		if !hasKey || hasInput {
			return dest, ruleMembers
		}
		// The key's form is the server's to check: any string goes, escaped.
		text, err := key.AsString()
		if err != nil {
			return dest, ruleKey
		}
		dest.key = text
	default: // OperationUnknown and every value outside the constants
		return dest, ruleOperation
	}
	return dest, ""
}

// logAttrs returns the fixed log values that describe d.
func (d destination) logAttrs() []any {
	attrs := []any{"form", d.form}
	if op := d.op.logValue(); op != "" {
		attrs = append(attrs, "op", op)
	}
	return attrs
}

// request builds the one request for d.
//
// Contract: the method and target come from the form and the operation alone,
// never from the message. A key is escaped with url.PathEscape into exactly
// one path segment, so it can neither add a segment nor start a query; the
// escaped form is the request target as sent.
func (d destination) request(ctx context.Context) (*http.Request, error) {
	switch d.form {
	case formLegacy:
		return bodyRequest(ctx, articlesURL, d.body)
	case formNew:
		keyURL := jobsURL + "/" + url.PathEscape(d.key)
		switch d.op {
		case OperationStart:
			return bodyRequest(ctx, jobsURL, d.body)
		case OperationStatus:
			return http.NewRequestWithContext(ctx, http.MethodGet, keyURL, nil)
		case OperationPublish:
			return http.NewRequestWithContext(ctx, http.MethodPost, keyURL+publishSuffix, nil)
		default: // route never returns these; send nothing
			return nil, errUnroutable
		}
	default: // the zero form and every value outside the constants
		return nil, errUnroutable
	}
}

// bodyRequest builds a POST of a JSON body to target, with the headers the
// current form has always been sent with.
func bodyRequest(ctx context.Context, target string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Expect", "100-continue")
	return req, nil
}
