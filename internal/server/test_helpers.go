//go:build test

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/failure"
	failuretestutil "github.com/isseis/yt2column/internal/failure/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// Fixed values the tests share. The transcript is form B (a timestamp line and
// a text line), which is ASCII so the test files stay ASCII.
const (
	testVideoID       = "dQw4w9WgXcQ"
	testValidRaw      = "0:00\nHello\n"
	testValidVideoURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	testMarker        = "MARKER-SECRET-VALUE"
	// testTranscriptMarker stands in for transcript or metadata text. It is
	// deliberately not in Options.Secrets, so a log line that carried it would
	// still contain it after redaction.
	testTranscriptMarker = "MARKER-TRANSCRIPT-TEXT"
	// gateBuffer is the capacity of the gate writer's entered channel.
	gateBuffer = 8
)

// testServer is a Server with recording fakes and a captured log, for the
// handler tests.
type testServer struct {
	*Server
	writer    *writertestutil.FakeArticleWriter
	publisher *publishertestutil.FakePublisher
	notifier  *failuretestutil.FakeNotifier
	logs      *bytes.Buffer
}

// newTestServer builds a Server whose collaborators are recording fakes.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	fakeWriter := &writertestutil.FakeArticleWriter{}
	pub := &publishertestutil.FakePublisher{}
	notifier := &failuretestutil.FakeNotifier{}
	logs := &bytes.Buffer{}
	s, err := New(Options{
		Writer:    fakeWriter,
		Webhook:   &WebhookOptions{Publisher: pub, Notifier: notifier},
		LogOutput: logs,
		Secrets:   []string{testMarker},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &testServer{Server: s, writer: fakeWriter, publisher: pub, notifier: notifier, logs: logs}
}

// newTestServerWithoutWebhook builds a Server the way run builds one without
// SLACK_WEBHOOK_URL: a recording writer, no Webhook options, and so no
// publisher or notifier at all.
func newTestServerWithoutWebhook(t *testing.T) *testServer {
	t.Helper()
	fakeWriter := &writertestutil.FakeArticleWriter{}
	logs := &bytes.Buffer{}
	s, err := New(Options{
		Writer:    fakeWriter,
		LogOutput: logs,
		Secrets:   []string{testMarker},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &testServer{Server: s, writer: fakeWriter, logs: logs}
}

// testKeySecret is the fixed key secret of the servers newJobsTestServer
// builds, so a test can look for it in a response or a log.
var testKeySecret = [32]byte{
	0x4d, 0x41, 0x52, 0x4b, 0x45, 0x52, 0x2d, 0x4b, 0x45, 0x59, 0x2d, 0x53, 0x45, 0x43, 0x52, 0x45,
	0x54, 0x2d, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x61, 0x62, 0x63, 0x64,
}

// jobsServerOptions selects what newJobsTestServer changes from
// newTestServer: the cache limit (0 selects articlecache.DefaultCapacity),
// the cache clock (nil selects time.Now), and no Webhook at all.
type jobsServerOptions struct {
	capacity  int
	clock     func() time.Time
	noWebhook bool
}

// newJobsTestServer builds a Server like newTestServer, through
// newWithInternals with the fixed key secret and the options' cache limit and
// clock.
func newJobsTestServer(t *testing.T, o jobsServerOptions) *testServer {
	t.Helper()
	fakeWriter := &writertestutil.FakeArticleWriter{Result: testArticle()}
	ts := &testServer{writer: fakeWriter, logs: &bytes.Buffer{}}
	opts := Options{Writer: fakeWriter, LogOutput: ts.logs, Secrets: []string{testMarker}}
	if !o.noWebhook {
		ts.publisher = &publishertestutil.FakePublisher{}
		ts.notifier = &failuretestutil.FakeNotifier{}
		opts.Webhook = &WebhookOptions{Publisher: ts.publisher, Notifier: ts.notifier}
	}
	capacity := o.capacity
	if capacity == 0 {
		capacity = articlecache.DefaultCapacity
	}
	s, err := newWithInternals(opts, internals{capacity: capacity, clock: o.clock, secret: testKeySecret})
	if err != nil {
		t.Fatalf("newWithInternals: %v", err)
	}
	ts.Server = s
	return ts
}

// testArticle returns an article the completion check accepts as postable.
func testArticle() writer.Article {
	return writer.Article{
		Title:       "Title",
		ChannelName: "Channel",
		Body:        "Body text.\n\n" + "Source: " + testValidVideoURL + "\n",
		SourceURL:   testValidVideoURL,
		Model:       "model",
	}
}

// validInput validates body as the start handler does, failing the test when
// it is rejected.
func (ts *testServer) validInput(t *testing.T, body string) jobInput {
	t.Helper()
	input, rej := ts.validateBody([]byte(body), "0123456789abcdef")
	if rej != nil {
		t.Fatalf("validateBody rejected the body: %+v", *rej)
	}
	return *input
}

// beginGeneration begins body's key in the cache and starts its generation
// job, as the start handler does after the body checks, and returns the key.
// It fails the test unless the cache starts or restarts a generation.
func (ts *testServer) beginGeneration(t *testing.T, body string) articlecache.Key {
	t.Helper()
	input := ts.validInput(t, body)
	key := ts.keyer.Key(input.key)
	outcome, run, _ := ts.cache.Begin(key, ts.acquireSlot)
	if outcome != articlecache.BeginStarted && outcome != articlecache.BeginRestarted {
		t.Fatalf("Begin outcome = %d, want started or restarted", outcome)
	}
	ts.startGeneration(run, key, input)
	return key
}

// statusOf renders key's status response with the server's state and decodes
// it.
func (ts *testServer) statusOf(t *testing.T, key articlecache.Key) statusResponse {
	t.Helper()
	data, err := renderStatus("0123456789abcdef", ts.webhookConfigured(), ts.cache.Lookup(key))
	if err != nil {
		t.Fatalf("renderStatus: %v", err)
	}
	var resp statusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode the status response %s: %v", data, err)
	}
	return resp
}

// newCustomServer builds a Server with the given collaborators, for tests that
// need a writer or notifier with special behavior.
func newCustomServer(t *testing.T, w writer.ProgressWriter, p WebhookPublisher, n failure.Notifier) *Server {
	t.Helper()
	s, err := New(Options{
		Writer:    w,
		Webhook:   &WebhookOptions{Publisher: p, Notifier: n},
		LogOutput: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

// countingPublisher counts Publish calls under a mutex, so two jobs posting at
// once do not race (the shared fakes record calls without a lock).
type countingPublisher struct {
	mu    sync.Mutex
	calls int
}

func (p *countingPublisher) Publish(context.Context, writer.Article) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return nil
}

// PostMessages implements WebhookPublisher. The legacy job never calls it, so
// it adds no call to the count.
func (p *countingPublisher) PostMessages(context.Context, publisher.SlackMessages, int, func(int)) error {
	return nil
}

func (p *countingPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// gateWriter blocks every Write until release is closed, so a test can hold a
// slot open on purpose.
type gateWriter struct {
	entered chan struct{}
	release chan struct{}
}

// newGateWriter builds a gate writer whose entered channel holds gateBuffer
// sends.
func newGateWriter() *gateWriter {
	return &gateWriter{entered: make(chan struct{}, gateBuffer), release: make(chan struct{})}
}

func (g *gateWriter) Write(context.Context, transcript.Transcript) (writer.Article, error) {
	g.entered <- struct{}{}
	<-g.release
	return writer.Article{}, nil
}

// WriteWithProgress implements writer.ProgressWriter by ignoring the observer:
// these tests hold the job at the write stage, not at a stage notification.
func (g *gateWriter) WriteWithProgress(ctx context.Context, t transcript.Transcript, _ writer.ProgressObserver) (writer.Article, error) {
	return g.Write(ctx, t)
}

// panickingWriter panics on Write, to test the job goroutine's recovery.
type panickingWriter struct{}

func (panickingWriter) Write(context.Context, transcript.Transcript) (writer.Article, error) {
	panic("test writer panic")
}

// WriteWithProgress implements writer.ProgressWriter.
func (w panickingWriter) WriteWithProgress(ctx context.Context, t transcript.Transcript, _ writer.ProgressObserver) (writer.Article, error) {
	return w.Write(ctx, t)
}

// panickingReader panics on Read, to test the handler's recovery.
type panickingReader struct{}

func (panickingReader) Read([]byte) (int, error) { panic("test body panic") }

// panickingNotifier panics on Notify, to test the notice recovery.
type panickingNotifier struct{}

func (panickingNotifier) Notify(context.Context, failure.Notice) error { panic("test notifier panic") }

// newRequest builds a server-side request with only a method, a target, and a
// body: the handler checks no header, so an accepted request needs none.
func newRequest(method, target string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, target, body)
}

// serve runs the handler and returns the recorder.
func (ts *testServer) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	ts.handler(rec, req)
	return rec
}

// serveAccepted runs a valid request and waits for its job to finish, so the
// fakes' recorded calls are complete.
func (ts *testServer) serveAccepted(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := ts.serve(newRequest(http.MethodPost, ArticlesPath, bytes.NewBufferString(body)))
	ts.jobs.Wait()
	return rec
}

// validBody returns a request body with the required members.
func validBody(videoURL, raw string) string {
	return `{"videoUrl":` + strconv.Quote(videoURL) + `,"transcript":` + strconv.Quote(raw) + `}`
}

// waitJobs waits for every in-flight job and notice, so the fakes' recorded
// calls are complete.
func (ts *testServer) waitJobs() {
	ts.jobs.Wait()
}

// decodeBody decodes the recorder's JSON body, failing the test when it is not
// the fixed response shape.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) responseBody {
	t.Helper()
	var body responseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
	return body
}

// recordingReader counts the bytes read from it, so a test can prove the
// handler did not read the body.
type recordingReader struct {
	r io.Reader
	n int
}

func (rr *recordingReader) Read(p []byte) (int, error) {
	n, err := rr.r.Read(p)
	rr.n += n
	return n, err
}

// MaxBodyBytesForTest is the body size limit, for tests in other packages that
// build a body just over it. Built only with the test tag.
const MaxBodyBytesForTest = maxBodyBytes

// MaxStatusResponseBytesForTest is the status response size limit, for the
// test in another package that checks the host relays a response this large.
const MaxStatusResponseBytesForTest = maxStatusResponseBytes

// MaxInFlightForTest is the number of requests allowed in flight
// at once, for documentation guards in other packages.
const MaxInFlightForTest = maxInFlight

// RejectReasonForTest is one rejection's fixed reason value and HTTP status.
type RejectReasonForTest struct {
	Value  string
	Status int
}

// RejectReasonsForTest returns every rejection's fixed reason value and status,
// in the declaration order, so a documentation guard in another package reads
// the table from the code instead of copying it. Built only with the test tag.
func RejectReasonsForTest() []RejectReasonForTest {
	table := make([]RejectReasonForTest, 0, int(reasonCount))
	for reason := range reasonCount {
		table = append(table, RejectReasonForTest{Value: reason.reasonValue(), Status: reason.statusCode()})
	}
	return table
}
