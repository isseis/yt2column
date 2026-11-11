//go:build test

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
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
	// gateBuffer is the capacity of the channels a held generation reports
	// on.
	gateBuffer = 8
)

// testServer is a Server with recording fakes and a captured log, for the
// handler tests.
type testServer struct {
	*Server
	writer *writertestutil.FakeArticleWriter
	logs   *bytes.Buffer
}

// newTestServer builds a Server whose collaborators are recording fakes.
func newTestServer(t *testing.T) *testServer {
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
// newTestServer: the cache limit (0 selects articlecache.DefaultCapacity) and
// the cache clock (nil selects time.Now).
type jobsServerOptions struct {
	capacity int
	clock    func() time.Time
}

// newJobsTestServer builds a Server like newTestServer, through
// newWithInternals with the fixed key secret and the options' cache limit and
// clock.
func newJobsTestServer(t *testing.T, o jobsServerOptions) *testServer {
	t.Helper()
	fakeWriter := &writertestutil.FakeArticleWriter{Result: testArticle()}
	ts := &testServer{writer: fakeWriter, logs: &bytes.Buffer{}}
	opts := Options{Writer: fakeWriter, LogOutput: ts.logs, Secrets: []string{testMarker}}
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

// testArticle returns an article the completion check accepts.
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
	input, rej := ts.validateBody([]byte(body))
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
	subject := ts.keyer.Subject(input.key)
	key := subject.Key()
	begun := ts.cache.Begin(subject, articlecache.StartPlain, ts.acquireSlot)
	if begun.Outcome != articlecache.BeginStarted && begun.Outcome != articlecache.BeginRestarted {
		t.Fatalf("Begin outcome = %d, want started or restarted", begun.Outcome)
	}
	ts.startGeneration(begun.Run, key, input)
	return key
}

// statusOf renders key's status response with the server's state and decodes
// it.
func (ts *testServer) statusOf(t *testing.T, key articlecache.Key) statusResponse {
	t.Helper()
	data, err := renderStatus("0123456789abcdef", ts.cache.Lookup(key))
	if err != nil {
		t.Fatalf("renderStatus: %v", err)
	}
	var resp statusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode the status response %s: %v", data, err)
	}
	return resp
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

// start runs a generation start with body.
func (ts *testServer) start(body io.Reader) *httptest.ResponseRecorder {
	return ts.serve(newRequest(http.MethodPost, JobsPath, body))
}

// startBody runs a generation start with body, waits for every job, and
// returns the response.
func (ts *testServer) startBody(body string) *httptest.ResponseRecorder {
	rec := ts.start(strings.NewReader(body))
	ts.waitJobs()
	return rec
}

// statusRequest runs a status request for the key part keyPart.
func (ts *testServer) statusRequest(keyPart string) *httptest.ResponseRecorder {
	return ts.serve(newRequest(http.MethodGet, JobsPath+"/"+keyPart, nil))
}

// getStatus runs a status request for key and decodes the response, failing
// the test unless it is a 200.
func (ts *testServer) getStatus(t *testing.T, key string) statusResponse {
	t.Helper()
	rec := ts.statusRequest(key)
	if rec.Code != http.StatusOK {
		t.Fatalf("status request: %d %s, want 200", rec.Code, rec.Body.String())
	}
	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode the status response %q: %v", rec.Body.String(), err)
	}
	return resp
}

// validBody returns a request body with the required members, sent plain.
func validBody(videoURL, raw string) string {
	return modeBody(videoURL, raw, modePlain)
}

// modeBody returns a request body with the required members and the given
// mode member.
func modeBody(videoURL, raw, mode string) string {
	return `{"videoUrl":` + strconv.Quote(videoURL) + `,"transcript":` + strconv.Quote(raw) + `,"mode":` + strconv.Quote(mode) + `}`
}

// waitJobs waits for every in-flight job, so the fakes' recorded calls are
// complete.
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

// MaxInFlightForTest is the number of generation slots, for documentation
// guards in other packages.
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

// APICodeSetForTest is one set of codes the job API's status response uses,
// with the name the documentation's table of the set carries.
type APICodeSetForTest struct {
	Name  string
	Codes []string
}

// APICodesForTest returns every code of the four sets the status response
// uses, so a documentation guard in another package reads them from the code.
// Each set walks every value of its enumeration through the function the
// response uses, so a value appended later is included; the step set is the
// JSON member names of the steps object. Built only with the test tag.
func APICodesForTest() []APICodeSetForTest {
	var steps []string
	for field := range reflect.TypeFor[statusSteps]().Fields() {
		steps = append(steps, field.Tag.Get("json"))
	}
	return []APICodeSetForTest{
		{Name: "job state", Codes: codesOf(articlecache.JobStatesForTest(), jobStateCode)},
		{Name: "step", Codes: steps},
		{Name: "step state", Codes: codesOf(articlecache.StageStates(), stageStateCode)},
		{Name: "failure", Codes: codesOf(articlecache.FailureReasonsForTest(), failureCode)},
	}
}

// codesOf returns code(v) for every value in values.
func codesOf[E any](values []E, code func(E) string) []string {
	codes := make([]string, 0, len(values))
	for _, v := range values {
		codes = append(codes, code(v))
	}
	return codes
}
