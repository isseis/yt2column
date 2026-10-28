//go:build test

package server

import (
	"context"
	"encoding/hex"
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

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/udstest"
)

// requireAccepted checks that rec is a publish operation's 202.
func requireAccepted(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %q)", rec.Code, rec.Body.String())
	}
	if body := decodeBody(t, rec); body.Status != "accepted" || body.Key != "" || !requestIDShape.MatchString(body.RequestID) {
		t.Fatalf("body = %+v, want accepted with a request ID and no key", body)
	}
}

// newWebhookTestServer builds a jobs test server whose publisher is the real
// one, posting to a fresh loopback Webhook, and returns both.
func newWebhookTestServer(t *testing.T) (*testServer, *loopbackWebhook) {
	t.Helper()
	hook := newLoopbackWebhook(t)
	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.Server.publisher = hook.publisherFor(t)
	return ts, hook
}

// publishOf returns key's publish state and reason from its status response.
func (ts *testServer) publishOf(t *testing.T, key string) statusPublish {
	t.Helper()
	job := ts.getStatus(t, key).Job
	if job.Publish == nil {
		t.Fatalf("job = %+v, want a done article with a publish state", job)
	}
	return *job.Publish
}

// waitEntered waits until a request reaches hook's hold, failing the test
// after five seconds instead of hanging when none ever does.
func waitEntered(t *testing.T, hook *loopbackWebhook) {
	t.Helper()
	select {
	case <-hook.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no request reached the Webhook")
	}
}

// TestPublishPostsLikeArticles checks that the publish operation posts the
// messages the legacy endpoint posts for the same article, here one that
// splits into three messages.
func TestPublishPostsLikeArticles(t *testing.T) {
	ts, hook := newWebhookTestServer(t)
	ts.writer.Result = messagesArticle(t, 3)
	body := validBody(testValidVideoURL, testValidRaw)

	if rec := ts.serveAccepted(t, body); rec.Code != http.StatusAccepted {
		t.Fatalf("legacy status = %d, want 202", rec.Code)
	}
	legacy := hook.received()
	key := acceptedKey(t, ts.startBody(body))
	requireAccepted(t, ts.publishRequest(key))
	ts.waitJobs()
	published := hook.received()

	if len(legacy) != 3 || !slices.Equal(published, legacy) {
		t.Errorf("published %d messages %q, want the legacy endpoint's %d %q", len(published), published, len(legacy), legacy)
	}
	if got := ts.publishOf(t, key); got.State != "posted" {
		t.Errorf("publish = %+v, want posted", got)
	}
	if line := logLine(ts.logs.String(), "post job succeeded"); !strings.Contains(line, "posted=3") || !strings.Contains(line, "total=3") {
		t.Errorf("the success log line = %q, want posted=3 total=3", line)
	}
}

// TestPublishRespondsBeforeWebhook checks that the publish operation answers
// 202 while the Webhook has not answered yet, and that the record is posting
// until it does.
func TestPublishRespondsBeforeWebhook(t *testing.T) {
	ts, hook := newWebhookTestServer(t)
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw)).String()
	hold := make(chan struct{})
	hook.set(0, hold)
	var once sync.Once
	release := func() { once.Do(func() { close(hold) }) }
	defer func() {
		release()
		ts.waitJobs()
	}()

	requireAccepted(t, ts.publishRequest(key))
	waitEntered(t, hook)
	if got := ts.publishOf(t, key); got.State != "posting" {
		t.Errorf("publish while the Webhook holds = %+v, want posting", got)
	}
	release()
	ts.waitJobs()
	if got := ts.publishOf(t, key); got.State != "posted" {
		t.Errorf("publish after the Webhook answered = %+v, want posted", got)
	}
}

// TestPublishIdempotent checks that a publish operation repeated while the
// first post runs and after it ended is answered 202 and posts nothing more.
func TestPublishIdempotent(t *testing.T) {
	ts, hook := newWebhookTestServer(t)
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw)).String()
	hold := make(chan struct{})
	hook.set(0, hold)
	var once sync.Once
	release := func() { once.Do(func() { close(hold) }) }
	defer func() {
		release()
		ts.waitJobs()
	}()

	requireAccepted(t, ts.publishRequest(key))
	waitEntered(t, hook)
	requireAccepted(t, ts.publishRequest(key))
	release()
	ts.waitJobs()
	requireAccepted(t, ts.publishRequest(key))
	ts.waitJobs()

	if got := hook.received(); len(got) != 1 {
		t.Errorf("Webhook messages = %d, want 1", len(got))
	}
	logs := ts.logs.String()
	if strings.Count(logs, "outcome=started") != 1 || strings.Count(logs, "outcome=already") != 2 {
		t.Errorf("want one outcome=started and two outcome=already lines:\n%s", logs)
	}
}

// TestPublishResumesAfterFailure checks a three-message article whose post
// fails at message k: the record is failed by the Webhook, and the retry
// posts from message k on, never the messages delivered before it.
func TestPublishResumesAfterFailure(t *testing.T) {
	article := messagesArticle(t, 3)
	all, err := publisher.SlackMessagesForTest(article)
	if err != nil {
		t.Fatalf("SlackMessagesForTest: %v", err)
	}
	for _, k := range []int{1, 2} {
		t.Run(strconv.Itoa(k), func(t *testing.T) {
			ts, hook := newWebhookTestServer(t)
			ts.writer.Result = article
			key := acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))

			hook.set(k, nil)
			requireAccepted(t, ts.publishRequest(key))
			ts.waitJobs()
			if got := hook.received(); !slices.Equal(got, all[:k-1]) {
				t.Errorf("first post delivered %d messages, want %d", len(got), k-1)
			}
			if got := ts.publishOf(t, key); got != (statusPublish{State: "failed", Reason: "webhook"}) {
				t.Errorf("publish after the failure = %+v, want failed by the webhook", got)
			}

			hook.set(0, nil)
			requireAccepted(t, ts.publishRequest(key))
			ts.waitJobs()
			// The retry's accepted line names where it resumes; for k = 1 the
			// first post's line has the same from, so it is the second one.
			if got := strings.Count(ts.logs.String(), "outcome=started from="+strconv.Itoa(k-1)); got != 1+(2-k) {
				t.Errorf("accepted lines naming from=%d = %d, want %d:\n%s", k-1, got, 1+(2-k), ts.logs.String())
			}
			if got := hook.received(); !slices.Equal(got, all[k-1:]) {
				t.Errorf("retry posted %d messages, want messages %d to 3", len(got), k)
			}
			if got := ts.publishOf(t, key); got.State != "posted" {
				t.Errorf("publish after the retry = %+v, want posted", got)
			}
		})
	}
}

// TestPublishRejections checks the records the publish operation refuses: a
// running generation, a failed one, an unknown key, and an unpostable
// article. None calls the Webhook or changes a record.
func TestPublishRejections(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	keyOf := func(text string) articlecache.Key { return ts.keyer.Key(ts.validInput(t, bodyWith(text)).key) }
	begin := func(text string) *articlecache.Run {
		_, run, _ := ts.cache.Begin(keyOf(text), func() bool { return true })
		return run
	}
	begin("running")
	begin("failed").Fail(articlecache.FailureLLM)
	begin("unpostable").CompleteUnpostable(testArticle(), articlecache.UnpostableMention)
	running, failed, unpostable, unknown := keyOf("running").String(), keyOf("failed").String(),
		keyOf("unpostable").String(), keyOf("unknown").String()

	tests := []struct {
		name   string
		key    string
		status int
		reason string
	}{
		{"running", running, http.StatusConflict, "article_not_ready"},
		{"failed", failed, http.StatusNotFound, "article_not_found"},
		{"unknown", unknown, http.StatusNotFound, "article_not_found"},
		{"unpostable", unpostable, http.StatusConflict, "article_not_postable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := articlecache.ParseKey(tc.key)
			if err != nil {
				t.Fatalf("ParseKey: %v", err)
			}
			before := ts.cache.Lookup(key)
			rec := ts.publishRequest(tc.key)
			requireRejected(t, rec, tc.status, tc.reason)
			if got := rec.Header().Get("Connection"); got == "close" {
				t.Errorf("Connection = %q, want the connection kept", got)
			}
			if after := ts.cache.Lookup(key); after != before {
				t.Errorf("the rejection changed the record: %+v, was %+v", after, before)
			}
		})
	}
	if calls := ts.publisher.RecordedPostCalls(); len(calls) != 0 {
		t.Errorf("PostMessages calls = %d, want 0", len(calls))
	}
}

// TestPublishFailureNotNotified checks that a failed post, by the Webhook or
// for any other reason, records its reason and posts no failure notice.
func TestPublishFailureNotNotified(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		reason string
	}{
		{"webhook", &publisher.SlackPostError{Total: 1, Attempted: true, Err: errors.New("503")}, "webhook"},
		{"internal", errors.New("refused before sending"), "internal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			ts.publisher.PostErr = tc.err
			key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw)).String()
			requireAccepted(t, ts.publishRequest(key))
			ts.waitJobs()
			if got := ts.publishOf(t, key); got != (statusPublish{State: "failed", Reason: tc.reason}) {
				t.Errorf("publish = %+v, want failed with %s", got, tc.reason)
			}
			if len(ts.notifier.Calls) != 0 {
				t.Errorf("notifier calls = %d, want 0", len(ts.notifier.Calls))
			}
			if line := logLine(ts.logs.String(), "post job failed"); !strings.Contains(line, "reason="+tc.reason) {
				t.Errorf("the failure log line = %q, want reason=%s", line, tc.reason)
			}
		})
	}
}

// TestPublishResponsesOmitInputs checks that no response of the publish
// operation, an accepted one, a repeat, the rejections, and the status after a
// failed post included, carries the transcript, the metadata, a secret, or a
// publisher's error text.
func TestPublishResponsesOmitInputs(t *testing.T) {
	const errorMarker = "MARKER-ERROR-TEXT"
	ts := newJobsTestServer(t, jobsServerOptions{})
	body := `{"videoUrl":"` + testValidVideoURL + `","transcript":"0:00\n` + testTranscriptMarker + `\n",` +
		`"title":"` + testTranscriptMarker + `","channelName":"` + testMarker + `"}`
	key := acceptedKey(t, ts.startBody(body))
	ts.publisher.PostErr = errors.New(errorMarker + " " + testMarker)

	responses := []string{
		ts.publishRequest(key).Body.String(),
	}
	ts.waitJobs()
	// The raw status response with only the article removed, which may hold
	// anything the article holds: decoding into a map keeps every other
	// member, an undeclared one included.
	var status map[string]any
	if err := json.Unmarshal(ts.statusRequest(key).Body.Bytes(), &status); err != nil {
		t.Fatalf("decode the status response: %v", err)
	}
	job, ok := status["job"].(map[string]any)
	if !ok {
		t.Fatalf("status response %v has no job object", status)
	}
	delete(job, "article")
	rest, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	responses = append(responses, string(rest))
	ts.publisher.PostErr = nil
	responses = append(responses,
		ts.publishRequest(key).Body.String(),
		ts.publishRequest(key).Body.String(),
		ts.publishRequest(testTranscriptMarker).Body.String(),
		ts.publishRequest(ts.keyer.Key(ts.validInput(t, bodyWith(testTranscriptMarker)).key).String()).Body.String(),
	)
	ts.waitJobs()

	markers := []string{
		testTranscriptMarker, testMarker, errorMarker, testVideoID,
		hex.EncodeToString(testKeySecret[:]), string(testKeySecret[:]),
	}
	for _, response := range responses {
		for _, marker := range markers {
			if strings.Contains(response, marker) {
				t.Errorf("response %s carries %q", response, marker)
			}
		}
	}
	logs := ts.logs.String()
	for _, marker := range []string{testTranscriptMarker, testMarker, hex.EncodeToString(testKeySecret[:])} {
		if strings.Contains(logs, marker) {
			t.Errorf("the log carries %q:\n%s", marker, logs)
		}
	}
}

// TestPublishWithoutWebhook checks that a server without a Webhook refuses
// every publish operation with webhook_not_configured, closing the
// connection, before it checks the body or the key, and changes no record.
func TestPublishWithoutWebhook(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{noWebhook: true})
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw)).String()
	withBody := newRequest(http.MethodPost, publishTarget(key), strings.NewReader("x"))
	before := ts.cache.DumpRecordsForTest()
	for name, req := range map[string]*http.Request{
		"done_key":    newRequest(http.MethodPost, publishTarget(key), nil),
		"invalid_key": newRequest(http.MethodPost, publishTarget("nope"), nil),
		"with_body":   withBody,
	} {
		t.Run(name, func(t *testing.T) {
			rec := ts.serve(req)
			requireRejected(t, rec, http.StatusServiceUnavailable, "webhook_not_configured")
			if got := rec.Header().Get("Connection"); got != "close" {
				t.Errorf("Connection = %q, want close", got)
			}
		})
	}
	// The method is checked before the Webhook.
	t.Run("get", func(t *testing.T) {
		rec := ts.serve(newRequest(http.MethodGet, publishTarget(key), nil))
		requireRejected(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
		if got := rec.Header().Get("Allow"); got != http.MethodPost {
			t.Errorf("Allow = %q, want POST", got)
		}
	})
	if after := ts.cache.DumpRecordsForTest(); after != before {
		t.Errorf("a rejection changed the records:\nbefore %s\nafter  %s", before, after)
	}
}

// TestPublishTargetRejections checks the targets, methods, and bodies the
// publish operation refuses before it looks the key up, on a server holding a
// done record: no rejection calls the Webhook or changes the record.
func TestPublishTargetRejections(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw)).String()
	tests := []struct {
		name    string
		method  string
		target  string
		status  int
		reason  string
		allow   string
		closeIt bool
	}{
		{"suffix_longer", http.MethodPost, JobsPath + "/" + key + "/publishx", http.StatusNotFound, "not_found", "", true},
		{"suffix_uppercase", http.MethodPost, JobsPath + "/" + key + "/Publish", http.StatusNotFound, "not_found", "", true},
		{"suffix_escaped", http.MethodPost, JobsPath + "/" + key + "/%70ublish", http.StatusNotFound, "not_found", "", true},
		{"query", http.MethodPost, publishTarget(key) + "?x=1", http.StatusNotFound, "not_found", "", true},
		{"uppercase_key", http.MethodPost, publishTarget(strings.ToUpper(key)), http.StatusBadRequest, "invalid_key", "", false},
		{"escaped_key", http.MethodPost, publishTarget(fmt.Sprintf("%%%02X", key[0]) + key[1:]), http.StatusBadRequest, "invalid_key", "", false},
		{"escaped_slash", http.MethodPost, publishTarget(key[:32] + "%2F" + key[35:]), http.StatusBadRequest, "invalid_key", "", false},
		{"empty_key", http.MethodPost, publishTarget(""), http.StatusBadRequest, "invalid_key", "", false},
		{"short_key", http.MethodPost, publishTarget(key[:63]), http.StatusBadRequest, "invalid_key", "", false},
		{"long_key", http.MethodPost, publishTarget(key + "0"), http.StatusBadRequest, "invalid_key", "", false},
		{"padded_key", http.MethodPost, publishTarget("%20" + key + "%20"), http.StatusBadRequest, "invalid_key", "", false},
		{"get", http.MethodGet, publishTarget(key), http.StatusMethodNotAllowed, "method_not_allowed", http.MethodPost, true},
	}
	before := ts.cache.DumpRecordsForTest()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := ts.serve(newRequest(tc.method, tc.target, nil))
			requireRejected(t, rec, tc.status, tc.reason)
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Errorf("Allow = %q, want %q", got, tc.allow)
			}
			if got := rec.Header().Get("Connection"); (got == "close") != tc.closeIt {
				t.Errorf("Connection = %q, want close=%v", got, tc.closeIt)
			}
		})
	}
	bodies := []struct {
		name    string
		keyPart string
		mutate  func(*http.Request)
	}{
		{"content_length", key, func(req *http.Request) { req.ContentLength = 1 }},
		{"transfer_encoding", key, func(req *http.Request) {
			req.ContentLength = -1
			req.TransferEncoding = []string{"chunked"}
		}},
		// The body is checked before the key, so the rejection that leaves
		// the body unread is the one that closes the connection.
		{"content_length_invalid_key", "nope", func(req *http.Request) { req.ContentLength = 1 }},
	}
	for _, tc := range bodies {
		mutate := tc.mutate
		t.Run("body_"+tc.name, func(t *testing.T) {
			counter := &recordingReader{r: strings.NewReader("x")}
			req := newRequest(http.MethodPost, publishTarget(tc.keyPart), io.NopCloser(counter))
			mutate(req)
			rec := ts.serve(req)
			requireRejected(t, rec, http.StatusBadRequest, "invalid_body")
			if got := rec.Header().Get("Connection"); got != "close" {
				t.Errorf("Connection = %q, want close", got)
			}
			if counter.n != 0 {
				t.Errorf("read %d bytes of the body, want 0", counter.n)
			}
		})
	}
	if after := ts.cache.DumpRecordsForTest(); after != before {
		t.Errorf("a rejection changed the records:\nbefore %s\nafter  %s", before, after)
	}
	if calls := ts.publisher.RecordedPostCalls(); len(calls) != 0 {
		t.Errorf("PostMessages calls = %d, want 0", len(calls))
	}
}

// panickingPostPublisher reports the first deliver messages from its from as
// delivered and then panics, to test the post job's recovery. Its Publish is
// never called.
type panickingPostPublisher struct {
	publishertestutil.FakePublisher
	deliver int
}

func (p *panickingPostPublisher) PostMessages(_ context.Context, _ publisher.SlackMessages, from int, delivered func(index int)) error {
	for i := range p.deliver {
		delivered(from + i)
	}
	panic("test post panic")
}

// TestPublishPanicResumes checks a post job that panics on a three-message
// article: the record fails as internal, keeping the deliveries reported before
// the panic, and the retry posts from the first message not delivered. A
// panic after the last delivery leaves nothing to send, and the retry is
// posted without a message.
func TestPublishPanicResumes(t *testing.T) {
	for _, delivered := range []int{1, 3} {
		t.Run(strconv.Itoa(delivered), func(t *testing.T) {
			ts, hook := newWebhookTestServer(t)
			article := messagesArticle(t, 3)
			all, err := publisher.SlackMessagesForTest(article)
			if err != nil {
				t.Fatalf("SlackMessagesForTest: %v", err)
			}
			ts.writer.Result = article
			key := acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))
			webhookPublisher := ts.Server.publisher
			ts.Server.publisher = &panickingPostPublisher{deliver: delivered}
			requireAccepted(t, ts.publishRequest(key))
			ts.waitJobs()
			if got := ts.publishOf(t, key); got != (statusPublish{State: "failed", Reason: "internal"}) {
				t.Fatalf("publish after the panic = %+v, want failed as internal", got)
			}
			if line := logLine(ts.logs.String(), "post job panicked"); !strings.Contains(line, "posted="+strconv.Itoa(delivered)) {
				t.Errorf("the panic log line = %q, want posted=%d", line, delivered)
			}

			ts.Server.publisher = webhookPublisher
			requireAccepted(t, ts.publishRequest(key))
			ts.waitJobs()
			if got := hook.received(); !slices.Equal(got, all[delivered:]) {
				t.Errorf("retry posted %d messages, want the %d after the delivered ones", len(got), len(all)-delivered)
			}
			if got := ts.publishOf(t, key); got.State != "posted" {
				t.Errorf("publish after the retry = %+v, want posted", got)
			}
		})
	}
}

// TestPublishHandlerPanicCleanup checks the publish handler's recovery: a
// panic after BeginPublish and before the job takes over fails the post as
// internal, so the record is not left posting, and posts no notice.
func TestPublishHandlerPanicCleanup(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw)).String()
	ts.beforePublishJob = func() { panic("test hook panic") }
	rec := ts.publishRequest(key)
	ts.waitJobs()
	requireRejected(t, rec, http.StatusInternalServerError, "internal_error")
	if got := rec.Header().Get("Connection"); got != "close" {
		t.Errorf("Connection = %q, want close", got)
	}
	if got := ts.publishOf(t, key); got != (statusPublish{State: "failed", Reason: "internal"}) {
		t.Errorf("publish = %+v, want failed as internal", got)
	}
	if calls := ts.publisher.RecordedPostCalls(); len(calls) != 0 || len(ts.notifier.Calls) != 0 {
		t.Errorf("side effects: PostMessages=%d notifier=%d, want none", len(calls), len(ts.notifier.Calls))
	}
}

// TestStartKeepsPosted checks that a generation start resent for a posted
// article returns its key and leaves the article posted, generating nothing.
func TestStartKeepsPosted(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	body := validBody(testValidVideoURL, testValidRaw)
	key := acceptedKey(t, ts.startBody(body))
	requireAccepted(t, ts.publishRequest(key))
	ts.waitJobs()
	if got := acceptedKey(t, ts.startBody(body)); got != key {
		t.Errorf("key = %s, want %s", got, key)
	}
	if got := ts.publishOf(t, key); got.State != "posted" {
		t.Errorf("publish = %+v, want posted", got)
	}
	if calls := len(ts.writer.RecordedCalls()); calls != 1 {
		t.Errorf("writer calls = %d, want 1", calls)
	}
}

// publishOnUnix sends a publish operation for key to the socket at path and
// returns the status code.
func publishOnUnix(t *testing.T, path, key string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://yt2column"+publishTarget(key), nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := (&http.Client{Transport: udstest.Transport(path)}).Do(req)
	if err != nil {
		t.Fatalf("send the request: %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// TestServeShutdownWaitsForPublish checks a shutdown with a post job in
// flight: the job is not canceled, Serve returns only after the Webhook
// answered, and the article is posted.
func TestServeShutdownWaitsForPublish(t *testing.T) {
	ts, hook := newWebhookTestServer(t)
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw))
	hold := make(chan struct{})
	hook.set(0, hold)
	var once sync.Once
	release := func() { once.Do(func() { close(hold) }) }
	defer release()
	listener, path := listenUnix(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ts.Serve(ctx, listener) }()

	if status := publishOnUnix(t, path, key.String()); status != http.StatusAccepted {
		t.Fatalf("publish status = %d, want 202", status)
	}
	waitEntered(t, hook)
	cancel()
	waitFor(t, "the shutdown to begin", func() bool { return ts.genCtx.Err() != nil })
	select {
	case <-done:
		t.Fatal("Serve returned before the post job finished")
	case <-time.After(100 * time.Millisecond):
	}

	release()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve error = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the post job finished")
	}
	if got := ts.cache.Lookup(key).Publish; got != articlecache.PublishPosted {
		t.Errorf("publish state = %d, want posted", got)
	}
	if line := logLine(ts.logs.String(), "stopped accepting connections"); !strings.Contains(line, "postJobs=1") {
		t.Errorf("the shutdown log line = %q, want postJobs=1", line)
	}
}

// TestJobsLogUnrecordedOutcome checks that a job whose handle no longer acts
// when it reports its outcome writes the not-recorded line instead of a
// success or failure line with values it could not record.
func TestJobsLogUnrecordedOutcome(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	body := validBody(testValidVideoURL, testValidRaw)
	input := ts.validInput(t, body)
	key := ts.keyer.Key(input.key)
	_, run, _ := ts.cache.Begin(key, func() bool { return true })
	run.CompletePostable(testArticle())
	ts.completeGeneration(run, key, input, testArticle())
	ts.failGeneration(run, key, input, articlecache.FailureLLM, errors.New("late"))

	_, pub := ts.cache.BeginPublish(key)
	pub.Finish()
	ts.runPublish(pub, key)
	ts.publisher.PostErr = errors.New("late")
	ts.runPublish(pub, key)

	logs := ts.logs.String()
	for _, line := range []string{"generation job succeeded", "generation job failed", "post job succeeded", "post job failed"} {
		if strings.Contains(logs, line) {
			t.Errorf("the log has %q for an outcome not recorded:\n%s", line, logs)
		}
	}
	if got := strings.Count(logs, "outcome not recorded"); got != 4 {
		t.Errorf("not-recorded lines = %d, want 4:\n%s", got, logs)
	}
}
