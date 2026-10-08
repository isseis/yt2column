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

func (r *slackRecorder) record(req slackRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
}

func (r *slackRecorder) all() []slackRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]slackRequest(nil), r.requests...)
}

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
// errors.Unwrap never show the endpoint, its path, or a sensitive part.
func requireErrorOmitsEndpoint(t *testing.T, err error, endpoint string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	secrets := append([]string{endpoint, slackWebhookPath, slackWebhookMarker}, slackwebhook.SensitiveParts(endpoint)...)
	for e := err; e != nil; e = errors.Unwrap(e) {
		for _, s := range secrets {
			if s != "" && strings.Contains(e.Error(), s) {
				t.Fatalf("error %q leaks %q", e.Error(), s)
			}
		}
	}
}

// cancelableContext reports the error set by cancel without ever closing Done,
// so a publisher's pre-send check sees the cancellation while a message already
// in flight is not interrupted.
type cancelableContext struct {
	context.Context
	mu  sync.Mutex
	err error
}

func (c *cancelableContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *cancelableContext) cancel(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
}

// failingTransport returns a fixed error from every RoundTrip.
type failingTransport struct {
	err error
}

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

func (b *cancelOnEOFBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF && !b.fired {
		b.fired = true
		b.cancel()
	}
	return n, err
}

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
			if _, err := NewSlackWebhookPublisher(value); err != nil {
				t.Fatalf("NewSlackWebhookPublisher(%q): %v", raw, err)
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

func TestSlackPublishPayload(t *testing.T) {
	t.Run("valid article", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		article := validArticle()
		if err := p.Publish(context.Background(), article); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		requests := server.recorder.all()
		if len(requests) != 1 {
			t.Fatalf("got %d requests, want 1", len(requests))
		}
		requireSlackPayloadShape(t, requests[0])
		if got, want := slackPayloadText(t, requests[0]), renderArticle(article); got != want {
			t.Fatalf("text = %q, want the posted article", got)
		}
	})

	t.Run("body with JSON-significant characters", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		article := validArticle()
		article.Body = "\"quoted\" \\ backslash } </script> \u2028 next"
		if err := p.Publish(context.Background(), article); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		requests := server.recorder.all()
		if len(requests) != 1 {
			t.Fatalf("got %d requests, want 1", len(requests))
		}
		requireSlackPayloadShape(t, requests[0])
		if got, want := slackPayloadText(t, requests[0]), renderArticle(article); got != want {
			t.Fatalf("text = %q, want the posted article", got)
		}
	})

	t.Run("empty model version", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		article := validArticle()
		article.ModelVersion = ""
		if err := p.Publish(context.Background(), article); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		requests := server.recorder.all()
		if len(requests) != 1 {
			t.Fatalf("got %d requests, want 1", len(requests))
		}
		if text := slackPayloadText(t, requests[0]); !strings.Contains(text, "(none)") {
			t.Fatalf("text does not name the empty model version: %q", text)
		}
	})
}

func TestSlackPublishSplit(t *testing.T) {
	t.Run("many lines", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		article := validArticle()
		article.Body = strings.Repeat("lorem ipsum dolor\n", 4000)
		posted := renderArticle(article)
		if err := p.Publish(context.Background(), article); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		requests := server.recorder.all()
		if len(requests) < 3 {
			t.Fatalf("got %d requests, want at least 3", len(requests))
		}
		for _, req := range requests {
			requireSlackPayloadShape(t, req)
		}
		checkSlackMessages(t, slackMessageTexts(t, server), posted)
		count, err := SlackMessageCount(article)
		if err != nil {
			t.Fatalf("SlackMessageCount: %v", err)
		}
		if len(requests) != count {
			t.Fatalf("received %d requests, SlackMessageCount = %d", len(requests), count)
		}
	})

	t.Run("one long line", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		article := validArticle()
		article.Body = strings.Repeat("\u3042", 20000) + strings.Repeat("\U0001F600", 4000)
		posted := renderArticle(article)
		if err := p.Publish(context.Background(), article); err != nil {
			t.Fatalf("Publish: %v", err)
		}
		messages := slackMessageTexts(t, server)
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
			server := newSlackOKServer(t)
			p := newTestSlackPublisher(t, server, SlackTestOptions{})
			article := articleWithPostedRunes(t, slackMaxMessageRunes+tc.extra, 'a')
			if err := p.Publish(context.Background(), article); err != nil {
				t.Fatalf("extra %d: Publish: %v", tc.extra, err)
			}
			requests := server.recorder.all()
			if len(requests) != tc.want {
				t.Fatalf("extra %d: got %d requests, want %d", tc.extra, len(requests), tc.want)
			}
			messages := slackMessageTexts(t, server)
			if tc.want == 1 && strings.HasPrefix(messages[0], "(") {
				t.Fatalf("a single message carries a position marker: %q", messages[0])
			}
			checkSlackMessages(t, messages, renderArticle(article))
		}
	})
}

func TestSlackPublishPrepareSendsNothing(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*writer.Article)
		wantErr error
	}{
		{"invalid title", func(a *writer.Article) { a.Title = "" }, writer.ErrInvalidArticle},
		{"invalid body", func(a *writer.Article) { a.Body = "x\x1b[2J" }, writer.ErrInvalidArticle},
		{"mention in body", func(a *writer.Article) { a.Body = "@channel" }, ErrSlackMention},
		{"mention in title", func(a *writer.Article) { a.Title = "<!here>" }, ErrSlackMention},
		{"whitespace-only fragment", func(a *writer.Article) { a.Body = strings.Repeat(" ", 20000) + "x" }, ErrSlackUnsplittable},
		{"too many messages", func(a *writer.Article) {
			a.Body = strings.Repeat("a", (slackMaxMessages+1)*(slackMaxMessageRunes-slackDisplayMaxRunes))
		}, ErrSlackUnsplittable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newSlackOKServer(t)
			p := newTestSlackPublisher(t, server, SlackTestOptions{})
			article := validArticle()
			tc.mutate(&article)
			err := p.Publish(context.Background(), article)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if _, ok := errors.AsType[*SlackPostError](err); ok {
				t.Fatalf("err = %v, must not wrap *SlackPostError", err)
			}
			if got := server.recorder.count(); got != 0 {
				t.Fatalf("server received %d requests, want 0", got)
			}
		})
	}
}

func TestSlackPublishResponses(t *testing.T) {
	t.Run("non-200 statuses", func(t *testing.T) {
		for _, status := range []int{400, 403, 404, 410, 429, 500} {
			t.Run(strconv.Itoa(status), func(t *testing.T) {
				status := status
				server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
					w.WriteHeader(status)
				})
				p := newTestSlackPublisher(t, server, SlackTestOptions{})
				err := p.Publish(context.Background(), threeMessageArticle(t))
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
				p := newTestSlackPublisher(t, server, SlackTestOptions{})
				err := p.Publish(context.Background(), validArticle())
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
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		err := p.Publish(context.Background(), validArticle())
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
				p := newTestSlackPublisher(t, server, SlackTestOptions{})
				err := p.Publish(context.Background(), validArticle())
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
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		err := p.Publish(context.Background(), validArticle())
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
		err := p.Publish(context.Background(), validArticle())
		statusErr, ok := errors.AsType[*SlackHTTPStatusError](err)
		if !ok {
			t.Fatalf("err = %v, want *SlackHTTPStatusError", err)
		}
		if statusErr.Reason != "" || statusErr.RequestID != "" {
			t.Fatalf("identifiers = %q/%q, want both dropped", statusErr.Reason, statusErr.RequestID)
		}
	})
}

func TestSlackPublishRedirect(t *testing.T) {
	target := newSlackOKServer(t)
	for _, status := range []int{301, 302, 307, 308} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			status := status
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.Header().Set("Location", target.server.URL+slackWebhookPath)
				w.WriteHeader(status)
			})
			p := newTestSlackPublisher(t, server, SlackTestOptions{})
			err := p.Publish(context.Background(), validArticle())
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

func TestSlackPublishTimeout(t *testing.T) {
	release := make(chan struct{})
	server := newSlackServer(t, func(http.ResponseWriter, *http.Request, int) {
		<-release
	})
	t.Cleanup(func() { close(release) })

	p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: 100 * time.Millisecond})
	err := p.Publish(context.Background(), validArticle())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	postErr, ok := errors.AsType[*SlackPostError](err)
	if !ok || !postErr.Attempted {
		t.Fatalf("SlackPostError = %+v, want Attempted", postErr)
	}
}

func TestSlackPublishCanceled(t *testing.T) {
	t.Run("before the first message", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := p.Publish(ctx, validArticle())
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
		err := p.Publish(ctx, threeMessageArticle(t))
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
		err := p.Publish(ctx, threeMessageArticle(t))
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

	t.Run("after the last message", func(t *testing.T) {
		ctx := &cancelableContext{Context: context.Background()}
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			ctx.cancel(context.Canceled)
			_, _ = io.WriteString(w, slackOKResponse)
		})
		p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: time.Second})
		if err := p.Publish(ctx, validArticle()); err != nil {
			t.Fatalf("Publish: %v, want nil", err)
		}
	})
}

func TestSlackPublishPartialFailure(t *testing.T) {
	server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		if n == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, slackOKResponse)
	})
	p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: time.Second})
	err := p.Publish(context.Background(), threeMessageArticle(t))
	if !errors.Is(err, ErrSlackHTTPStatus) {
		t.Fatalf("err = %v, want ErrSlackHTTPStatus", err)
	}
	postErr, ok := errors.AsType[*SlackPostError](err)
	if !ok || postErr.Total != 3 || postErr.Posted != 1 || !postErr.Attempted {
		t.Fatalf("SlackPostError = %+v, want Total 3, Posted 1, Attempted true", postErr)
	}
	if got := server.recorder.count(); got != 2 {
		t.Fatalf("server received %d requests, want 2", got)
	}
	if !strings.Contains(err.Error(), "posted 1 of 3") {
		t.Fatalf("error = %q, want it to name 1 of 3", err.Error())
	}
}

func TestSlackPublishErrorsOmitWebhookURL(t *testing.T) {
	article := validArticle()

	t.Run("closed server", func(t *testing.T) {
		server := newSlackOKServer(t)
		endpoint := server.url()
		server.server.Close()
		p := NewSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint)
		requireErrorOmitsEndpoint(t, p.Publish(context.Background(), article), endpoint)
	})

	t.Run("timeout", func(t *testing.T) {
		release := make(chan struct{})
		server := newSlackServer(t, func(http.ResponseWriter, *http.Request, int) { <-release })
		t.Cleanup(func() { close(release) })
		p := newTestSlackPublisher(t, server, SlackTestOptions{Timeout: 100 * time.Millisecond})
		requireErrorOmitsEndpoint(t, p.Publish(context.Background(), article), server.url())
	})

	t.Run("cancel", func(t *testing.T) {
		server := newSlackOKServer(t)
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		requireErrorOmitsEndpoint(t, p.Publish(ctx, article), server.url())
	})

	t.Run("non-200 status", func(t *testing.T) {
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, slackWebhookMarker)
		})
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		requireErrorOmitsEndpoint(t, p.Publish(context.Background(), article), server.url())
	})

	t.Run("oversized body", func(t *testing.T) {
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			_, _ = io.WriteString(w, strings.Repeat("a", slackMaxResponseBytes+1))
		})
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		requireErrorOmitsEndpoint(t, p.Publish(context.Background(), article), server.url())
	})

	t.Run("redirect", func(t *testing.T) {
		target := newSlackOKServer(t)
		server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.Header().Set("Location", target.server.URL+slackWebhookPath)
			w.WriteHeader(http.StatusFound)
		})
		p := newTestSlackPublisher(t, server, SlackTestOptions{})
		requireErrorOmitsEndpoint(t, p.Publish(context.Background(), article), server.url())
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

func TestSlackPublishTransportErrorWithheld(t *testing.T) {
	server := newSlackOKServer(t)
	endpoint := server.url()
	transport := failingTransport{err: errors.New("dial " + endpoint + ": connection refused")}
	p := newSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint, transport)
	err := p.Publish(context.Background(), validArticle())
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

func TestSlackPublishZeroValue(t *testing.T) {
	var p SlackWebhookPublisher
	err := p.Publish(context.Background(), validArticle())
	if err == nil {
		t.Fatal("Publish on the zero value has no error")
	}
	if _, ok := errors.AsType[*SlackPostError](err); ok {
		t.Fatalf("err = %v, must not wrap *SlackPostError", err)
	}
}

func TestSlackPublishErrorClasses(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{"http status", func(t *testing.T) error {
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.WriteHeader(http.StatusInternalServerError)
			})
			return newTestSlackPublisher(t, server, SlackTestOptions{}).Publish(context.Background(), validArticle())
		}},
		{"invalid response", func(t *testing.T) error {
			server := newSlackServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				_, _ = io.WriteString(w, "OK")
			})
			return newTestSlackPublisher(t, server, SlackTestOptions{}).Publish(context.Background(), validArticle())
		}},
		{"transport", func(t *testing.T) error {
			server := newSlackOKServer(t)
			endpoint := server.url()
			server.server.Close()
			p := NewSlackWebhookPublisherForLoopbackTest(t, SlackTestOptions{Timeout: time.Second}, endpoint)
			return p.Publish(context.Background(), validArticle())
		}},
		{"unsplittable", func(t *testing.T) error {
			article := validArticle()
			article.Body = strings.Repeat(" ", 20000) + "x"
			return newTestSlackPublisher(t, newSlackOKServer(t), SlackTestOptions{}).Publish(context.Background(), article)
		}},
		{"mention", func(t *testing.T) error {
			article := validArticle()
			article.Body = "@channel"
			return newTestSlackPublisher(t, newSlackOKServer(t), SlackTestOptions{}).Publish(context.Background(), article)
		}},
		{"invalid article", func(t *testing.T) error {
			article := validArticle()
			article.Title = ""
			return newTestSlackPublisher(t, newSlackOKServer(t), SlackTestOptions{}).Publish(context.Background(), article)
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
		err := p.Publish(context.Background(), validArticle())
		if !errors.Is(err, ErrSlackTransport) {
			t.Fatalf("err = %v, want ErrSlackTransport", err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v must match exactly one classification, not a context error", err)
		}
	})
}
