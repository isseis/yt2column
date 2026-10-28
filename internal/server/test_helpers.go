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
