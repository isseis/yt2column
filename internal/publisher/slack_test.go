//go:build test

package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/slackwebhook"
	"github.com/isseis/yt2column/internal/writer"
)

// slackWebhookMarker appears in the endpoint path of every test publisher, so
// a leaked part of the Webhook URL is recognizable. It is lowercase and
// alphanumeric so it also fits the identifier shapes read from a response.
const slackWebhookMarker = "marker9f31c2e7"

// slackWebhookPath is the path appended to a test server's URL.
const slackWebhookPath = "/hooks/" + slackWebhookMarker

// fatalRecorder is a testing.TB that records a Fatal message instead of failing
// the test. Fatalf deliberately does not end the calling goroutine, so the
// helper under test keeps running and the caller can observe that it returned
// no publisher.
type fatalRecorder struct {
	testing.TB
	message string
}

// Fatal records the message instead of failing.
func (r *fatalRecorder) Fatal(args ...any) {
	r.message = fmt.Sprint(args...)
}

// Fatalf records the formatted message instead of failing.
func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.message = fmt.Sprintf(format, args...)
}

// slackRequest is one recorded POST.
type slackRequest struct {
	method      string
	contentType string
	raw         []byte
	members     map[string]json.RawMessage
}

// slackRecorder records the requests a test server receives.
type slackRecorder struct {
	mu       sync.Mutex
	requests []slackRequest
}

// record appends one received request.
func (r *slackRecorder) record(req slackRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
}

// all returns a copy of the recorded requests.
func (r *slackRecorder) all() []slackRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]slackRequest(nil), r.requests...)
}

// count returns how many requests were recorded.
func (r *slackRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

// slackServer is a test server with its recorder.
type slackServer struct {
	server   *httptest.Server
	recorder *slackRecorder
}

// url is the endpoint a test publisher posts to: the server URL with the
// marker path.
func (s *slackServer) url() string {
	return s.server.URL + slackWebhookPath
}

// newSlackServer starts a server that records every request and answers with
// handler, which receives the request number starting at 1. Its body is read
// and decoded before handler runs.
func newSlackServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, n int)) *slackServer {
	t.Helper()
	recorder := &slackRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		members := map[string]json.RawMessage{}
		if err := json.Unmarshal(body, &members); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		recorder.record(slackRequest{
			method:      r.Method,
			contentType: r.Header.Get("Content-Type"),
			raw:         body,
			members:     members,
		})
		handler(w, r, recorder.count())
	}))
	t.Cleanup(server.Close)
	return &slackServer{server: server, recorder: recorder}
}

// newSlackOKServer answers every request with the success body.
func newSlackOKServer(t *testing.T) *slackServer {
	t.Helper()
	return newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		_, _ = io.WriteString(w, slackOKResponse)
	})
}

// newTestSlackPublisherAt builds a loopback publisher for the server URL plus
// path, defaulting the timeout.
func newTestSlackPublisherAt(t *testing.T, s *slackServer, opts SlackTestOptions, path string) *SlackWebhookPublisher {
	t.Helper()
	if opts.Timeout == 0 {
		opts.Timeout = time.Second
	}
	return NewSlackWebhookPublisherForLoopbackTest(t, opts, s.server.URL+path)
}

// newTestSlackPublisher builds a loopback publisher for the marker path.
func newTestSlackPublisher(t *testing.T, s *slackServer, opts SlackTestOptions) *SlackWebhookPublisher {
	t.Helper()
	return newTestSlackPublisherAt(t, s, opts, slackWebhookPath)
}

// threeMessageArticle returns an article whose posted text splits into exactly
// three messages.
func threeMessageArticle(t *testing.T) writer.Article {
	t.Helper()
	return articleWithPostedRunes(t, 2*(slackMaxMessageRunes-slackDisplayMaxRunes)+1, 'a')
}

// slackPayloadText decodes the text member of a recorded payload.
func slackPayloadText(t *testing.T, req slackRequest) string {
	t.Helper()
	raw, ok := req.members["text"]
	if !ok {
		t.Fatalf("payload has no text member: %s", req.raw)
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		t.Fatalf("text member is not a string: %v", err)
	}
	return text
}

// slackMessageTexts returns the text of every recorded request in order.
func slackMessageTexts(t *testing.T, server *slackServer) []string {
	t.Helper()
	requests := server.recorder.all()
	messages := make([]string, len(requests))
	for i, req := range requests {
		messages[i] = slackPayloadText(t, req)
	}
	return messages
}

// requireSlackPayloadShape checks the request method, content type, and the
// exact payload members.
func requireSlackPayloadShape(t *testing.T, req slackRequest) {
	t.Helper()
	if req.method != http.MethodPost {
		t.Fatalf("method = %q, want POST", req.method)
	}
	if req.contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", req.contentType)
	}
	if len(req.members) != 2 {
		t.Fatalf("payload members = %v, want exactly text and silent", req.members)
	}
	for _, key := range []string{"blocks", "attachments"} {
		if _, ok := req.members[key]; ok {
			t.Fatalf("payload contains %q", key)
		}
	}
	raw, ok := req.members["silent"]
	if !ok || string(raw) != "true" {
		t.Fatalf("payload silent = %q, want true", raw)
	}
	if _, ok := req.members["text"]; !ok {
		t.Fatalf("payload has no text member: %s", req.raw)
	}
}

// requireErrorOmitsEndpoint checks that err and every error reachable through
// errors.Unwrap and Unwrap() []error never show the endpoint, its path, or a
// sensitive part.
func requireErrorOmitsEndpoint(t *testing.T, err error, endpoint string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	secrets := append([]string{endpoint, slackWebhookPath, slackWebhookMarker}, slackwebhook.SensitiveParts(endpoint)...)
	var walk func(error)
	walk = func(e error) {
		for _, s := range secrets {
			if s != "" && strings.Contains(e.Error(), s) {
				t.Fatalf("error %q leaks %q", e.Error(), s)
			}
		}
		if multi, ok := e.(interface{ Unwrap() []error }); ok {
			for _, child := range multi.Unwrap() {
				if child != nil {
					walk(child)
				}
			}
			return
		}
		if child := errors.Unwrap(e); child != nil {
			walk(child)
		}
	}
	walk(err)
}

// cancelableContext reports the error set by cancel without ever closing Done,
// so a publisher's pre-send check sees the cancellation while a message already
// in flight is not interrupted.
type cancelableContext struct {
	context.Context
	mu  sync.Mutex
	err error
}

// Err returns the error set by cancel, or nil.
func (c *cancelableContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// cancel records err; Done is never closed.
func (c *cancelableContext) cancel(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
}

// failingTransport returns a fixed error from every RoundTrip.
type failingTransport struct {
	err error
}

// RoundTrip returns the fixed error.
func (f failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, f.err
}

// cancelOnEOFTransport delegates to base and calls cancel once the first
// response body reaches EOF. The client has read the whole body by then, so a
// message already in flight still succeeds and the cancellation is seen only
// between messages.
type cancelOnEOFTransport struct {
	base   http.RoundTripper
	cancel func()
	once   sync.Once
}

// RoundTrip wraps the response body so cancel fires at EOF.
func (t *cancelOnEOFTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	response.Body = &cancelOnEOFBody{ReadCloser: response.Body, cancel: func() { t.once.Do(t.cancel) }}
	return response, nil
}

// cancelOnEOFBody calls cancel when the underlying body first reports EOF.
type cancelOnEOFBody struct {
	io.ReadCloser
	cancel func()
	fired  bool
}

// Read calls cancel when the underlying body reaches EOF.
func (b *cancelOnEOFBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF && !b.fired {
		b.fired = true
		b.cancel()
	}
	return n, err
}

// staticResponseTransport answers every request with a fixed status and a body
// built from the request, so a test can fail a body read or block it on the
// request context.
type staticResponseTransport struct {
	status int
	body   func(req *http.Request) io.ReadCloser
}

// RoundTrip answers with the fixed status and the built body.
func (t staticResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.status,
		Header:     http.Header{},
		Body:       t.body(req),
		Request:    req,
	}, nil
}

// failingReadBody fails every Read.
type failingReadBody struct{}

// Read always fails.
func (failingReadBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }

// Close does nothing.
func (failingReadBody) Close() error { return nil }

// blockingTransport signals when RoundTrip starts, blocks until the request
// context ends, and then returns a non-context error.
type blockingTransport struct {
	started chan struct{}
	once    sync.Once
}

// RoundTrip signals started, waits for the context, then fails.
func (t *blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.once.Do(func() { close(t.started) })
	<-req.Context().Done()
	return nil, errors.New("transport stopped")
}

// contextErrorBody blocks until the request context ends and then returns its
// error.
type contextErrorBody struct {
	ctx context.Context
}

// Read blocks until the context ends and returns its error.
func (b *contextErrorBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

// Close does nothing.
func (b *contextErrorBody) Close() error { return nil }

// TestSlackMaxPostDurationMatchesHelper checks that SlackMaxPostDuration and
// the test helper that bounds a post of any number of messages agree at the
// largest number, so the two formulas cannot drift apart.
func TestSlackMaxPostDurationMatchesHelper(t *testing.T) {
	if got := SlackPublishMaxDurationForTest(slackMaxMessages); got != SlackMaxPostDuration {
		t.Errorf("SlackPublishMaxDurationForTest(%d) = %v, SlackMaxPostDuration = %v; want them equal",
			slackMaxMessages, got, SlackMaxPostDuration)
	}
}

// TestNewSlackWebhookPublisher checks the accepted URL shapes and
// that construction sends nothing.
func TestNewSlackWebhookPublisher(t *testing.T) {
	for _, raw := range []string{
		"https://mattermost.example.com/hooks/xxxxxxxxxxxxxxxxxxxxxxxxxx",
		"https://hooks.slack.com/services/T000/B000/XXXX",
	} {
		t.Run(raw, func(t *testing.T) {
			value, err := secret.New(raw)
			if err != nil {
				t.Fatalf("secret.New: %v", err)
			}
			got, err := NewSlackWebhookPublisher(value)
			if err != nil {
				t.Fatalf("NewSlackWebhookPublisher(%q): %v", raw, err)
			}
			if got == nil {
				t.Fatalf("NewSlackWebhookPublisher(%q) returned a nil publisher", raw)
			}
		})
	}

	t.Run("construction sends nothing", func(t *testing.T) {
		server := newSlackOKServer(t)
		_ = NewSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, server.url())
		if got := server.recorder.count(); got != 0 {
			t.Fatalf("constructing sent %d requests, want 0", got)
		}
	})
}

// TestNewSlackWebhookPublisherRejects checks the zero value
// and the rejected URL shapes, each error free of the URL.
func TestNewSlackWebhookPublisherRejects(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		if _, err := NewSlackWebhookPublisher(secret.Secret{}); err == nil {
			t.Fatal("NewSlackWebhookPublisher(zero) has no error")
		}
	})

	rejected := []string{
		"http://mattermost.example.com/hooks/x",
		"mattermost.example.com/hooks/x",
		"https:///hooks/x",
		"https://mattermost.example.com/hooks/x\n",
	}
	for _, raw := range rejected {
		t.Run(fmt.Sprintf("reject %q", raw), func(t *testing.T) {
			value, err := secret.New(raw)
			if err != nil {
				t.Fatalf("secret.New: %v", err)
			}
			_, err = NewSlackWebhookPublisher(value)
			if err == nil {
				t.Fatalf("NewSlackWebhookPublisher(%q) has no error", raw)
			}
			requireErrorOmitsEndpoint(t, err, raw)
		})
	}
}

// TestNewSlackWebhookPublisherForLoopbackTestRejects checks that a non-loopback
// endpoint and a non-positive Timeout fail the test without echoing the endpoint.
func TestNewSlackWebhookPublisherForLoopbackTestRejects(t *testing.T) {
	cases := []struct {
		name     string
		opts     SlackTestOptions
		endpoint string
	}{
		{"not loopback", SlackTestOptions{Timeout: time.Second}, "https://mattermost.example.com" + slackWebhookPath},
		{"non-positive timeout", SlackTestOptions{}, "http://127.0.0.1:1" + slackWebhookPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &fatalRecorder{TB: t}
			value := NewSlackWebhookPublisherForLoopbackTest(recorder, tc.opts, tc.endpoint)
			if recorder.message == "" {
				t.Fatal("the constructor did not reject the endpoint")
			}
			if value != nil {
				t.Fatalf("the constructor returned %v, want nil", value)
			}
			if strings.Contains(recorder.message, slackWebhookMarker) {
				t.Fatalf("the failure message %q holds the endpoint path", recorder.message)
			}
		})
	}
}

// TestSlackMessagesForTest exercises the test-tag messages helper.
func TestSlackMessagesForTest(t *testing.T) {
	article := validArticle()
	messages, err := SlackMessagesForTest(article)
	if err != nil {
		t.Fatalf("SlackMessagesForTest: %v", err)
	}
	if len(messages) != 1 || messages[0] != renderSlackPost(article).text {
		t.Fatalf("SlackMessagesForTest = %q, want the posted article", messages)
	}
}

// TestSlackPostPayload checks the POST shape, the exact payload members,
// silent, and the posted text of the prepared messages.
func TestSlackPostPayload(t *testing.T) {
	t.Run("valid article", func(t *testing.T) {
		article := validArticle()
		m, err := PrepareSlackMessages(article)
		if err != nil {
			t.Fatalf("PrepareSlackMessages: %v", err)
		}
		server := newSlackOKServer(t)
		if _, _, err := postFrom(t, server, m, 0); err != nil {
			t.Fatalf("PostMessages: %v", err)
		}
		requests := server.recorder.all()
		if len(requests) != 1 {
			t.Fatalf("got %d requests, want 1", len(requests))
		}
		requireSlackPayloadShape(t, requests[0])
		if got, want := slackPayloadText(t, requests[0]), renderSlackPost(article).text; got != want {
			t.Fatalf("text = %q, want the posted article", got)
		}
	})

	t.Run("body with JSON-significant characters", func(t *testing.T) {
		article := validArticle()
		article.Body = "\"quoted\" \\ backslash } </script> \u2028 next"
		m, err := PrepareSlackMessages(article)
		if err != nil {
			t.Fatalf("PrepareSlackMessages: %v", err)
		}
		server := newSlackOKServer(t)
		if _, _, err := postFrom(t, server, m, 0); err != nil {
			t.Fatalf("PostMessages: %v", err)
		}
		requests := server.recorder.all()
		if len(requests) != 1 {
			t.Fatalf("got %d requests, want 1", len(requests))
		}
		requireSlackPayloadShape(t, requests[0])
		if got, want := slackPayloadText(t, requests[0]), renderSlackPost(article).text; got != want {
			t.Fatalf("text = %q, want the posted article", got)
		}
	})

	t.Run("empty model version", func(t *testing.T) {
		article := validArticle()
		article.ModelVersion = ""
		m, err := PrepareSlackMessages(article)
		if err != nil {
			t.Fatalf("PrepareSlackMessages: %v", err)
		}
		_, texts, err := postFrom(t, newSlackOKServer(t), m, 0)
		if err != nil {
			t.Fatalf("PostMessages: %v", err)
		}
		if len(texts) != 1 {
			t.Fatalf("got %d requests, want 1", len(texts))
		}
		// The footnote shows no model version, empty or not.
		if strings.Contains(texts[0], "(none)") {
			t.Fatalf("text = %q, want the post with no model version", texts[0])
		}
	})
}

// TestSlackPostCostLine checks that the cost is posted once, in the known and
// the unknown form, and that a split article carries it in the footnote of the
// last message only.
func TestSlackPostCostLine(t *testing.T) {
	for _, tc := range []struct {
		name     string
		article  writer.Article
		wantLine string
	}{
		{"known", costedArticle(), "Models: model-x (generation) \u00b7 Estimated cost: 0.0023 USD"},
		{"unknown", validArticle(), "Models: model-x (generation) \u00b7 Estimated cost: unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := PrepareSlackMessages(tc.article)
			if err != nil {
				t.Fatalf("PrepareSlackMessages: %v", err)
			}
			_, texts, err := postFrom(t, newSlackOKServer(t), m, 0)
			if err != nil {
				t.Fatalf("PostMessages: %v", err)
			}
			text := strings.Join(texts, "")
			if !strings.HasSuffix(text, tc.wantLine) {
				t.Fatalf("the posted text does not end with the footnote %q: %q", tc.wantLine, text)
			}
			if n := strings.Count(text, "Estimated cost"); n != 1 {
				t.Fatalf("the cost line appears %d times, want 1", n)
			}
		})
	}

	t.Run("split article keeps the footnote in the last message", func(t *testing.T) {
		m, err := PrepareSlackMessages(threeMessageArticle(t))
		if err != nil {
			t.Fatalf("PrepareSlackMessages: %v", err)
		}
		_, messages, err := postFrom(t, newSlackOKServer(t), m, 0)
		if err != nil {
			t.Fatalf("PostMessages: %v", err)
		}
		if len(messages) < 2 {
			t.Fatalf("got %d messages, want the article to split", len(messages))
		}
		if !strings.Contains(messages[len(messages)-1], "Estimated cost: ") {
			t.Fatalf("the last message has no footnote: %q", messages[len(messages)-1])
		}
		if n := strings.Count(strings.Join(messages, ""), "Estimated cost"); n != 1 {
			t.Fatalf("the cost line appears %d times across the messages, want 1", n)
		}
	})
}

// TestSlackPostSplit checks the split messages, the boundary lengths, and that
// the request count matches the Len of the messages PrepareSlackMessages
// returns.
func TestSlackPostSplit(t *testing.T) {
	t.Run("many lines", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat("lorem ipsum dolor\n", 4000)
		posted := renderSlackPost(article).text
		m, err := PrepareSlackMessages(article)
		if err != nil {
			t.Fatalf("PrepareSlackMessages: %v", err)
		}
		server := newSlackOKServer(t)
		_, messages, err := postFrom(t, server, m, 0)
		if err != nil {
			t.Fatalf("PostMessages: %v", err)
		}
		requests := server.recorder.all()
		if len(requests) < 3 {
			t.Fatalf("got %d requests, want at least 3", len(requests))
		}
		for _, req := range requests {
			requireSlackPayloadShape(t, req)
		}
		checkSlackMessages(t, messages, posted)
		if len(requests) != m.Len() {
			t.Fatalf("received %d requests, PrepareSlackMessages Len() = %d", len(requests), m.Len())
		}
	})

	t.Run("one long line", func(t *testing.T) {
		article := validArticle()
		article.Body = strings.Repeat("\u3042", 20000) + strings.Repeat("\U0001F600", 4000) + shortLastLine
		posted := renderSlackPost(article).text
		m, err := PrepareSlackMessages(article)
		if err != nil {
			t.Fatalf("PrepareSlackMessages: %v", err)
		}
		_, messages, err := postFrom(t, newSlackOKServer(t), m, 0)
		if err != nil {
			t.Fatalf("PostMessages: %v", err)
		}
		if len(messages) < 2 {
			t.Fatalf("got %d messages, want at least 2", len(messages))
		}
		checkSlackMessages(t, messages, posted)
	})

	t.Run("exactly the limit and one over", func(t *testing.T) {
		for _, tc := range []struct {
			extra int
			want  int
		}{{0, 1}, {1, 2}} {
			article := articleWithPostedRunes(t, slackMaxMessageRunes+tc.extra, 'a')
			m, err := PrepareSlackMessages(article)
			if err != nil {
				t.Fatalf("extra %d: PrepareSlackMessages: %v", tc.extra, err)
			}
			_, messages, err := postFrom(t, newSlackOKServer(t), m, 0)
			if err != nil {
				t.Fatalf("extra %d: PostMessages: %v", tc.extra, err)
			}
			if len(messages) != tc.want {
				t.Fatalf("extra %d: got %d requests, want %d", tc.extra, len(messages), tc.want)
			}
			if tc.want == 1 && strings.HasPrefix(messages[0], "(") {
				t.Fatalf("a single message carries a position marker: %q", messages[0])
			}
			checkSlackMessages(t, messages, renderSlackPost(article).text)
		}
	})
}

// TestSlackPostWaitsBetweenMessages exercises the inter-message wait and
// checks that every message still arrives.
func TestSlackPostWaitsBetweenMessages(t *testing.T) {
	article := threeMessageArticle(t)
	m, err := PrepareSlackMessages(article)
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	server := newSlackOKServer(t)
	p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: time.Second, Interval: 20 * time.Millisecond})
	if err := p.PostMessages(context.Background(), m, 0, func(int) {}); err != nil {
		t.Fatalf("PostMessages: %v", err)
	}
	if got := server.recorder.count(); got != 3 {
		t.Fatalf("server received %d requests, want 3", got)
	}
	checkSlackMessages(t, slackMessageTexts(t, server), renderSlackPost(article).text)
}

// TestPrepareSlackMessagesRejectionNotPostError checks that every preparation
// rejection is its own error, not a *SlackPostError, and returns no messages,
// which PostMessages refuses to send.
func TestPrepareSlackMessagesRejectionNotPostError(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*writer.Article)
		wantErr error
	}{
		{"invalid title", func(a *writer.Article) { a.Title = "" }, writer.ErrInvalidArticle},
		{"invalid body", func(a *writer.Article) { a.Body = "x\x1b[2J" }, writer.ErrInvalidArticle},
		{"invalid channel name", func(a *writer.Article) { a.ChannelName = "a\u202eb" }, writer.ErrInvalidChannelName},
		{"mention in body", func(a *writer.Article) { a.Body = "`@channel`" }, ErrSlackMention},
		{"mention in title", func(a *writer.Article) { a.Title = "<!here>" }, ErrSlackMention},
		{"whitespace-only fragment", func(a *writer.Article) { a.Body = strings.Repeat(" ", 20000) + "x" }, ErrSlackUnsplittable},
		{"too many messages", func(a *writer.Article) {
			a.Body = strings.Repeat(strings.Repeat("a", slackMaxMessageRunes-slackDisplayMaxRunes-1)+"\n", slackMaxMessages+1)
		}, ErrSlackUnsplittable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			article := validArticle()
			tc.mutate(&article)
			m, err := PrepareSlackMessages(article)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if m.Len() != 0 {
				t.Fatalf("a rejection returned %d messages, want none", m.Len())
			}
			if _, ok := errors.AsType[*SlackPostError](err); ok {
				t.Fatalf("err = %v, must not wrap *SlackPostError", err)
			}
		})
	}
}

// TestSlackPostResponses checks the non-200 statuses, the invalid 200 bodies,
// and the identifier extraction.
func TestSlackPostResponses(t *testing.T) {
	m, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}

	t.Run("non-200 statuses", func(t *testing.T) {
		three, err := PrepareSlackMessages(threeMessageArticle(t))
		if err != nil {
			t.Fatalf("PrepareSlackMessages: %v", err)
		}
		for _, status := range []int{400, 403, 404, 410, 429, 500} {
			t.Run(strconv.Itoa(status), func(t *testing.T) {
				status := status
				server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
					w.WriteHeader(status)
				})
				_, _, err := postFrom(t, server, three, 0)
				if !errors.Is(err, ErrSlackHTTPStatus) {
					t.Fatalf("err = %v, want ErrSlackHTTPStatus", err)
				}
				if !strings.Contains(err.Error(), strconv.Itoa(status)) {
					t.Fatalf("error = %q, want the status code", err.Error())
				}
				if got := server.recorder.count(); got != 1 {
					t.Fatalf("server received %d requests, want 1", got)
				}
			})
		}
	})

	t.Run("invalid 200 bodies", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{"empty", ""},
			{"uppercase", "OK"},
			{"trailing newline", "ok\n"},
			{"json", `{"ok":true}`},
			{"at the limit", strings.Repeat("a", slackMaxResponseBytes)},
			{"one over the limit", strings.Repeat("a", slackMaxResponseBytes+1)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				body := tc.body
				server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
					_, _ = io.WriteString(w, body)
				})
				_, _, err := postFrom(t, server, m, 0)
				if !errors.Is(err, ErrSlackInvalidResponse) {
					t.Fatalf("err = %v, want ErrSlackInvalidResponse", err)
				}
				if got := server.recorder.count(); got != 1 {
					t.Fatalf("server received %d requests, want 1", got)
				}
			})
		}
	})

	t.Run("mattermost app error", func(t *testing.T) {
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"id":"web.incoming_webhook.parse.app_error","message":"x","request_id":"0123456789abcdefghijklmnop"}`)
		})
		_, _, err := postFrom(t, server, m, 0)
		if !errors.Is(err, ErrSlackHTTPStatus) {
			t.Fatalf("err = %v, want ErrSlackHTTPStatus", err)
		}
		statusErr, ok := errors.AsType[*SlackHTTPStatusError](err)
		if !ok {
			t.Fatalf("errors.AsType[*SlackHTTPStatusError] = false")
		}
		if statusErr.StatusCode != http.StatusBadRequest {
			t.Fatalf("StatusCode = %d, want 400", statusErr.StatusCode)
		}
		if statusErr.Reason != "web.incoming_webhook.parse.app_error" {
			t.Fatalf("Reason = %q", statusErr.Reason)
		}
		if statusErr.RequestID != "0123456789abcdefghijklmnop" {
			t.Fatalf("RequestID = %q", statusErr.RequestID)
		}
	})

	t.Run("identifier shapes", func(t *testing.T) {
		cases := []struct {
			name       string
			body       string
			wantReason string
			wantReqID  string
		}{
			{"reason and request id", `{"id":"web.incoming_webhook.general.app_error","request_id":"0123456789abcdefghijklmnop"}`, "web.incoming_webhook.general.app_error", "0123456789abcdefghijklmnop"},
			{"plain reason", "web_incoming_error", "web_incoming_error", ""},
			{"id with a bad shape", `{"id":"Not-An-Id"}`, "", ""},
			{"request id with a bad length", `{"request_id":"abc"}`, "", ""},
			{"non-string id", `{"id":123}`, "", ""},
			{"malformed json", `{"id":`, "", ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				body := tc.body
				server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(w, body)
				})
				_, _, err := postFrom(t, server, m, 0)
				statusErr, ok := errors.AsType[*SlackHTTPStatusError](err)
				if !ok {
					t.Fatalf("err = %v, want *SlackHTTPStatusError", err)
				}
				if statusErr.Reason != tc.wantReason || statusErr.RequestID != tc.wantReqID {
					t.Fatalf("identifiers = %q/%q, want %q/%q", statusErr.Reason, statusErr.RequestID, tc.wantReason, tc.wantReqID)
				}
			})
		}
	})

	t.Run("oversized error body drops identifiers", func(t *testing.T) {
		// The body is a complete AppError followed by enough spaces to exceed
		// the read limit. Reading exactly the limit would parse the JSON and
		// keep the id, so the identifiers must be dropped.
		body := `{"id":"web.incoming_webhook.parse.app_error"}` + strings.Repeat(" ", slackMaxResponseBytes)
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, body)
		})
		_, _, err := postFrom(t, server, m, 0)
		statusErr, ok := errors.AsType[*SlackHTTPStatusError](err)
		if !ok {
			t.Fatalf("err = %v, want *SlackHTTPStatusError", err)
		}
		if statusErr.Reason != "" || statusErr.RequestID != "" {
			t.Fatalf("identifiers = %q/%q, want both dropped for an oversized body", statusErr.Reason, statusErr.RequestID)
		}
	})

	t.Run("webhook key dropped", func(t *testing.T) {
		const key = "abcdefghij0123456789klmnop"
		if len(key) != 26 {
			t.Fatalf("key length = %d, want 26", len(key))
		}
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"id":"`+key+`","request_id":"`+key+`"}`)
		})
		p := NewSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, server.server.URL+"/hooks/"+key)
		err := p.PostMessages(context.Background(), m, 0, func(int) {})
		statusErr, ok := errors.AsType[*SlackHTTPStatusError](err)
		if !ok {
			t.Fatalf("err = %v, want *SlackHTTPStatusError", err)
		}
		if statusErr.Reason != "" || statusErr.RequestID != "" {
			t.Fatalf("identifiers = %q/%q, want both dropped", statusErr.Reason, statusErr.RequestID)
		}
	})
}

// TestSlackPostRedirect checks that a 3xx is a status failure and the Location
// target receives nothing.
func TestSlackPostRedirect(t *testing.T) {
	m, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	target := newSlackOKServer(t)
	for _, status := range []int{301, 302, 307, 308} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			status := status
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.Header().Set("Location", target.server.URL+slackWebhookPath)
				w.WriteHeader(status)
			})
			_, _, err := postFrom(t, server, m, 0)
			if !errors.Is(err, ErrSlackHTTPStatus) {
				t.Fatalf("err = %v, want ErrSlackHTTPStatus", err)
			}
			if got := target.recorder.count(); got != 0 {
				t.Fatalf("the redirect target received %d requests, want 0", got)
			}
			if got := server.recorder.count(); got != 1 {
				t.Fatalf("the server received %d requests, want 1", got)
			}
		})
	}
}

// TestSlackPostTimeout checks that an unanswered request ends with a
// context.DeadlineExceeded wrapped in a *SlackPostError.
func TestSlackPostTimeout(t *testing.T) {
	m, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	release := make(chan struct{})
	server := newSlackServer(t, func(http.ResponseWriter, *http.Request, int) {
		<-release
	})
	t.Cleanup(func() { close(release) })

	p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: 100 * time.Millisecond})
	err = p.PostMessages(context.Background(), m, 0, func(int) {})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	postErr, ok := errors.AsType[*SlackPostError](err)
	if !ok || !postErr.Attempted {
		t.Fatalf("SlackPostError = %+v, want Attempted", postErr)
	}
}

// TestSlackPostBodyFailures checks the body-read outcomes the design names: a
// 200 read failure is a transport failure, a read outliving the deadline wraps
// the context error, and a non-200 read failure keeps the status without
// identifiers.
func TestSlackPostBodyFailures(t *testing.T) {
	endpoint := "http://127.0.0.1:1" + slackWebhookPath
	m, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}

	t.Run("200 body read failure is a transport failure", func(t *testing.T) {
		transport := staticResponseTransport{status: http.StatusOK, body: func(*http.Request) io.ReadCloser {
			return failingReadBody{}
		}}
		p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint, transport)
		err := p.PostMessages(context.Background(), m, 0, func(int) {})
		if !errors.Is(err, ErrSlackTransport) {
			t.Fatalf("err = %v, want ErrSlackTransport", err)
		}
		requireErrorOmitsEndpoint(t, err, endpoint)
	})

	t.Run("200 body read timeout wraps the context error", func(t *testing.T) {
		transport := staticResponseTransport{status: http.StatusOK, body: func(req *http.Request) io.ReadCloser {
			return &contextErrorBody{ctx: req.Context()}
		}}
		p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: 100 * time.Millisecond}, endpoint, transport)
		err := p.PostMessages(context.Background(), m, 0, func(int) {})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("non-200 body read failure drops identifiers", func(t *testing.T) {
		transport := staticResponseTransport{status: http.StatusBadRequest, body: func(*http.Request) io.ReadCloser {
			return failingReadBody{}
		}}
		p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint, transport)
		err := p.PostMessages(context.Background(), m, 0, func(int) {})
		statusErr, ok := errors.AsType[*SlackHTTPStatusError](err)
		if !ok {
			t.Fatalf("err = %v, want *SlackHTTPStatusError", err)
		}
		if statusErr.StatusCode != http.StatusBadRequest || statusErr.Reason != "" || statusErr.RequestID != "" {
			t.Fatalf("status error = %+v, want 400 with no identifiers", statusErr)
		}
	})
}

// TestSlackPostCanceled checks a canceled context before sending, in flight,
// and between messages, and a successful last message after a cancel.
func TestSlackPostCanceled(t *testing.T) {
	one, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	three, err := PrepareSlackMessages(threeMessageArticle(t))
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}

	t.Run("before the first message", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := p.PostMessages(ctx, one, 0, func(int) {})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		postErr, ok := errors.AsType[*SlackPostError](err)
		if !ok || postErr.Posted != 0 || postErr.Attempted {
			t.Fatalf("SlackPostError = %+v, want Posted 0, Attempted false", postErr)
		}
		if got := server.recorder.count(); got != 0 {
			t.Fatalf("server received %d requests, want 0", got)
		}
	})

	t.Run("before the second message", func(t *testing.T) {
		ctx := &cancelableContext{Context: context.Background()}
		var mu sync.Mutex
		seen := 0
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			mu.Lock()
			seen++
			first := seen == 1
			mu.Unlock()
			if first {
				ctx.cancel(context.Canceled)
			}
			_, _ = io.WriteString(w, slackOKResponse)
		})
		p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: time.Second})
		err := p.PostMessages(ctx, three, 0, func(int) {})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		postErr, ok := errors.AsType[*SlackPostError](err)
		if !ok {
			t.Fatalf("errors.AsType[*SlackPostError] = false")
		}
		if postErr.Total != 3 || postErr.Posted != 1 || postErr.Attempted {
			t.Fatalf("SlackPostError = %+v, want Total 3, Posted 1, Attempted false", postErr)
		}
		if got := server.recorder.count(); got != 1 {
			t.Fatalf("server received %d requests, want 1", got)
		}
	})

	t.Run("during the wait between messages", func(t *testing.T) {
		server := newSlackOKServer(t)
		ctx, cancel := context.WithCancel(context.Background())
		transport := &cancelOnEOFTransport{base: http.DefaultTransport, cancel: cancel}
		p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second, Interval: time.Hour}, server.url(), transport)
		err := p.PostMessages(ctx, three, 0, func(int) {})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		postErr, ok := errors.AsType[*SlackPostError](err)
		if !ok || postErr.Total != 3 || postErr.Posted != 1 || postErr.Attempted {
			t.Fatalf("SlackPostError = %+v, want Total 3, Posted 1, Attempted false", postErr)
		}
		if got := server.recorder.count(); got != 1 {
			t.Fatalf("server received %d requests, want 1", got)
		}
	})

	t.Run("while a request is in flight", func(t *testing.T) {
		endpoint := "http://127.0.0.1:1" + slackWebhookPath
		transport := &blockingTransport{started: make(chan struct{})}
		p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Hour}, endpoint, transport)
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- p.PostMessages(ctx, one, 0, func(int) {}) }()
		<-transport.started
		cancel()
		err := <-errCh
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		postErr, ok := errors.AsType[*SlackPostError](err)
		if !ok || !postErr.Attempted || postErr.Posted != 0 {
			t.Fatalf("SlackPostError = %+v, want Attempted true, Posted 0", postErr)
		}
		requireErrorOmitsEndpoint(t, err, endpoint)
	})

	t.Run("after the last message", func(t *testing.T) {
		ctx := &cancelableContext{Context: context.Background()}
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			ctx.cancel(context.Canceled)
			_, _ = io.WriteString(w, slackOKResponse)
		})
		p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: time.Second})
		if err := p.PostMessages(ctx, one, 0, func(int) {}); err != nil {
			t.Fatalf("PostMessages: %v, want nil", err)
		}
	})
}

// TestSlackPostErrorsOmitWebhookURL checks that no failure and no unwrapped
// error shows the URL, its path, or a part, and neither does %+v or %#v.
func TestSlackPostErrorsOmitWebhookURL(t *testing.T) {
	m, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}

	t.Run("closed server", func(t *testing.T) {
		server := newSlackOKServer(t)
		endpoint := server.url()
		server.server.Close()
		p := NewSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint)
		requireErrorOmitsEndpoint(t, p.PostMessages(context.Background(), m, 0, func(int) {}), endpoint)
	})

	t.Run("timeout", func(t *testing.T) {
		release := make(chan struct{})
		server := newSlackServer(t, func(http.ResponseWriter, *http.Request, int) { <-release })
		t.Cleanup(func() { close(release) })
		p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: 100 * time.Millisecond})
		requireErrorOmitsEndpoint(t, p.PostMessages(context.Background(), m, 0, func(int) {}), server.url())
	})

	t.Run("cancel", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		requireErrorOmitsEndpoint(t, p.PostMessages(ctx, m, 0, func(int) {}), server.url())
	})

	t.Run("non-200 status", func(t *testing.T) {
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, slackWebhookMarker)
		})
		_, _, err := postFrom(t, server, m, 0)
		requireErrorOmitsEndpoint(t, err, server.url())
	})

	t.Run("oversized body", func(t *testing.T) {
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			_, _ = io.WriteString(w, strings.Repeat("a", slackMaxResponseBytes+1))
		})
		_, _, err := postFrom(t, server, m, 0)
		requireErrorOmitsEndpoint(t, err, server.url())
	})

	t.Run("redirect", func(t *testing.T) {
		target := newSlackOKServer(t)
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.Header().Set("Location", target.server.URL+slackWebhookPath)
			w.WriteHeader(http.StatusFound)
		})
		_, _, err := postFrom(t, server, m, 0)
		requireErrorOmitsEndpoint(t, err, server.url())
	})

	t.Run("struct formatting", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
			if got := fmt.Sprintf(format, p); strings.Contains(got, slackWebhookMarker) {
				t.Fatalf("%s output leaks the Webhook URL: %s", format, got)
			}
		}
	})
}

// TestSlackPostTransportErrorWithheld checks that a transport error naming the
// URL path is replaced by the fixed withheld message.
func TestSlackPostTransportErrorWithheld(t *testing.T) {
	m, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	server := newSlackOKServer(t)
	endpoint := server.url()
	transport := failingTransport{err: errors.New("dial " + endpoint + ": connection refused")}
	p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint, transport)
	err = p.PostMessages(context.Background(), m, 0, func(int) {})
	if !errors.Is(err, ErrSlackTransport) {
		t.Fatalf("err = %v, want ErrSlackTransport", err)
	}
	if strings.Contains(err.Error(), slackWebhookMarker) {
		t.Fatalf("error leaks the endpoint path: %v", err)
	}
	if !strings.Contains(err.Error(), "withheld") {
		t.Fatalf("error = %v, want the withheld details message", err)
	}
}

// TestSlackPostErrorClasses checks that each failure, of the preparation or of
// the send, matches exactly one of the six classifications.
func TestSlackPostErrorClasses(t *testing.T) {
	m, err := PrepareSlackMessages(validArticle())
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	cases := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{"http status", func(t *testing.T) error {
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.WriteHeader(http.StatusInternalServerError)
			})
			_, _, err := postFrom(t, server, m, 0)
			return err
		}},
		{"invalid response", func(t *testing.T) error {
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				_, _ = io.WriteString(w, "OK")
			})
			_, _, err := postFrom(t, server, m, 0)
			return err
		}},
		{"transport", func(t *testing.T) error {
			server := newSlackOKServer(t)
			endpoint := server.url()
			server.server.Close()
			p := NewSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint)
			return p.PostMessages(context.Background(), m, 0, func(int) {})
		}},
		{"unsplittable", func(*testing.T) error {
			article := validArticle()
			article.Body = strings.Repeat(" ", 20000) + "x"
			_, err := PrepareSlackMessages(article)
			return err
		}},
		{"mention", func(*testing.T) error {
			article := validArticle()
			article.Body = "`@channel`"
			_, err := PrepareSlackMessages(article)
			return err
		}},
		{"invalid article", func(*testing.T) error {
			article := validArticle()
			article.Title = ""
			_, err := PrepareSlackMessages(article)
			return err
		}},
	}

	predicates := []struct {
		name  string
		match func(error) bool
	}{
		{"http status", func(err error) bool { return errors.Is(err, ErrSlackHTTPStatus) }},
		{"invalid response", func(err error) bool { return errors.Is(err, ErrSlackInvalidResponse) }},
		{"transport", func(err error) bool { return errors.Is(err, ErrSlackTransport) }},
		{"unsplittable", func(err error) bool { return errors.Is(err, ErrSlackUnsplittable) }},
		{"mention", func(err error) bool { return errors.Is(err, ErrSlackMention) }},
		{"invalid article", func(err error) bool { return errors.Is(err, writer.ErrInvalidArticle) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(t)
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, predicate := range predicates {
				got := predicate.match(err)
				if predicate.name == tc.name && !got {
					t.Fatalf("err = %v does not match its own class %s", err, tc.name)
				}
				if predicate.name != tc.name && got {
					t.Fatalf("err = %v also matches the %s class", err, predicate.name)
				}
			}
		})
	}

	t.Run("transport failure with a context cause", func(t *testing.T) {
		server := newSlackOKServer(t)
		endpoint := server.url()
		transport := failingTransport{err: context.Canceled}
		p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint, transport)
		err := p.PostMessages(context.Background(), m, 0, func(int) {})
		if !errors.Is(err, ErrSlackTransport) {
			t.Fatalf("err = %v, want ErrSlackTransport", err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v must match exactly one classification, not a context error", err)
		}
	})
}

// postFrom runs PostMessages for m from index from against server and returns
// the indices passed to delivered, the texts the server received, and the
// error.
func postFrom(t *testing.T, server *slackServer, m SlackMessages, from int) ([]int, []string, error) {
	t.Helper()
	p := newTestSlackPublisher(t, server, SlackTestOptions{})
	var delivered []int
	err := p.PostMessages(context.Background(), m, from, func(i int) { delivered = append(delivered, i) })
	return delivered, slackMessageTexts(t, server), err
}

func TestPostMessagesResume(t *testing.T) {
	article := threeMessageArticle(t)
	m, err := PrepareSlackMessages(article)
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	texts, err := SlackMessagesForTest(article)
	if err != nil {
		t.Fatalf("SlackMessagesForTest: %v", err)
	}
	if m.Len() != 3 || len(texts) != 3 {
		t.Fatalf("messages = %d and %d, want 3", m.Len(), len(texts))
	}

	// from is the number of messages delivered before: 0 resumes at the
	// first message, and 1 < k resumes at message k with from = k - 1.
	for _, from := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("from %d", from), func(t *testing.T) {
			delivered, got, err := postFrom(t, newSlackOKServer(t), m, from)
			if err != nil {
				t.Fatalf("PostMessages error = %v, want nil", err)
			}
			if !slices.Equal(got, texts[from:]) {
				t.Errorf("server received messages %d.. = %d texts, want the %d from index %d", from, len(got), len(texts)-from, from)
			}
			wantDelivered := []int{0, 1, 2}[from:]
			if !slices.Equal(delivered, wantDelivered) {
				t.Errorf("delivered = %v, want %v", delivered, wantDelivered)
			}
		})
	}

	t.Run("from the total sends nothing", func(t *testing.T) {
		server := newSlackOKServer(t)
		delivered, got, err := postFrom(t, server, m, m.Len())
		if err != nil {
			t.Fatalf("PostMessages error = %v, want nil", err)
		}
		if len(got) != 0 || len(delivered) != 0 {
			t.Errorf("sent %d messages and delivered %v, want none", len(got), delivered)
		}
	})
}

func TestPostMessagesFailureCountsFromFirst(t *testing.T) {
	m, err := PrepareSlackMessages(threeMessageArticle(t))
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	cases := []struct {
		name          string
		from          int
		failRequest   int // the server's request number that fails, from 1
		wantPosted    int
		wantDelivered []int
	}{
		{"first message of a fresh post", 0, 1, 0, nil},
		{"third message of a fresh post", 0, 3, 2, []int{0, 1}},
		{"first message of a resumed post", 1, 1, 1, nil},
		{"second message of a resumed post", 1, 2, 2, []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
				if n == tc.failRequest {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = io.WriteString(w, slackOKResponse)
			})
			delivered, got, err := postFrom(t, server, m, tc.from)
			postErr, ok := errors.AsType[*SlackPostError](err)
			if !ok {
				t.Fatalf("PostMessages error = %v, want a *SlackPostError", err)
			}
			if postErr.Total != 3 || postErr.Posted != tc.wantPosted || !postErr.Attempted {
				t.Errorf("SlackPostError = %+v, want Total 3, Posted %d, Attempted true", postErr, tc.wantPosted)
			}
			if !slices.Equal(delivered, tc.wantDelivered) {
				t.Errorf("delivered = %v, want %v", delivered, tc.wantDelivered)
			}
			if len(got) != tc.failRequest {
				t.Errorf("server received %d requests, want %d (nothing after the failure)", len(got), tc.failRequest)
			}
		})
	}
}

// TestPostMessagesResumedWaits checks the timing of a resumed post: it sends
// its first message without waiting the interval, and a context done before
// a later message stops it with the count of the messages delivered so far.
func TestPostMessagesResumedWaits(t *testing.T) {
	m, err := PrepareSlackMessages(threeMessageArticle(t))
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}

	t.Run("no wait before the first message sent", func(t *testing.T) {
		server := newSlackOKServer(t)
		// A wait for the hour-long interval would outlast the context's
		// deadline and fail the post.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		p := newTestSlackPublisher(t, server, SlackTestOptions{Interval: time.Hour})
		if err := p.PostMessages(ctx, m, m.Len()-1, func(int) {}); err != nil {
			t.Fatalf("PostMessages error = %v, want nil", err)
		}
		if n := server.recorder.count(); n != 1 {
			t.Errorf("server received %d requests, want 1", n)
		}
	})

	t.Run("canceled before the next message", func(t *testing.T) {
		// The context is canceled once the first resumed message is
		// delivered, so the wait before the next one is interrupted.
		ctx, cancel := context.WithCancel(context.Background())
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{Interval: time.Hour})
		var delivered []int
		err := p.PostMessages(ctx, m, 1, func(i int) {
			delivered = append(delivered, i)
			cancel()
		})
		postErr, ok := errors.AsType[*SlackPostError](err)
		if !ok || postErr.Total != 3 || postErr.Posted != 2 || postErr.Attempted {
			t.Fatalf("PostMessages error = %v (%+v), want a SlackPostError with Total 3, Posted 2, Attempted false", err, postErr)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("PostMessages error = %v, want context.Canceled", err)
		}
		if !slices.Equal(delivered, []int{1}) {
			t.Errorf("delivered = %v, want [1]", delivered)
		}
	})
}

func TestPostMessagesRejectsWithoutSending(t *testing.T) {
	m, err := PrepareSlackMessages(threeMessageArticle(t))
	if err != nil {
		t.Fatalf("PrepareSlackMessages: %v", err)
	}
	nop := func(int) {}
	cases := []struct {
		name      string
		m         SlackMessages
		from      int
		delivered func(int)
		want      error
	}{
		{"from below zero", m, -1, nop, errSlackFromRange},
		{"from past the total", m, m.Len() + 1, nop, errSlackFromRange},
		{"messages not prepared", SlackMessages{}, 0, nop, errSlackNoMessages},
		{"nil delivered", m, 0, nil, errSlackNilDelivered},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newSlackOKServer(t)
			p := newTestSlackPublisher(t, server, SlackTestOptions{})
			err := p.PostMessages(context.Background(), tc.m, tc.from, tc.delivered)
			if !errors.Is(err, tc.want) {
				t.Errorf("PostMessages error = %v, want %v", err, tc.want)
			}
			if _, ok := errors.AsType[*SlackPostError](err); ok {
				t.Errorf("PostMessages error = %v, must not be a *SlackPostError", err)
			}
			if n := server.recorder.count(); n != 0 {
				t.Errorf("server received %d requests, want 0", n)
			}
		})
	}

	t.Run("zero value publisher", func(t *testing.T) {
		var p SlackWebhookPublisher
		err := p.PostMessages(context.Background(), m, 0, nop)
		if !errors.Is(err, errZeroWebhookURL) {
			t.Errorf("PostMessages error = %v, want errZeroWebhookURL", err)
		}
		if _, ok := errors.AsType[*SlackPostError](err); ok {
			t.Errorf("PostMessages error = %v, must not be a *SlackPostError", err)
		}
	})
}
