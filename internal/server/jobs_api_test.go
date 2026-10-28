//go:build test

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// keyShape is the article key form the server issues.
var keyShape = regexp.MustCompile(`^[0-9a-f]{64}$`)

// bodyWith returns a valid body whose transcript text is text, so two calls
// with different texts give different keys.
func bodyWith(text string) string {
	return validBody(testValidVideoURL, "0:00\n"+text+"\n")
}

// acceptedKey checks that rec is a generation start's 202 and returns its key.
func acceptedKey(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body %q)", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body.Status != "accepted" || !keyShape.MatchString(body.Key) || !requestIDShape.MatchString(body.RequestID) {
		t.Fatalf("body = %+v, want accepted with a key and a request ID", body)
	}
	return body.Key
}

// requireRejected checks that rec is a rejection with status and reason.
func requireRejected(t *testing.T, rec *httptest.ResponseRecorder, status int, reason string) {
	t.Helper()
	if rec.Code != status {
		t.Errorf("status = %d, want %d (body %q)", rec.Code, status, rec.Body.String())
	}
	if body := decodeBody(t, rec); body.Status != "rejected" || body.Reason != reason {
		t.Errorf("body = %+v, want rejected %s", body, reason)
	}
}

// holdAtDraft makes the fake writer stop every generation right after it
// reports the draft step started, until the returned release function runs,
// and returns a channel that receives once per generation that got there.
func holdAtDraft(fake *writertestutil.FakeArticleWriter) (reached <-chan struct{}, release func()) {
	reachedCh := make(chan struct{}, gateBuffer)
	releaseCh := make(chan struct{})
	fake.Progress = []writertestutil.FakeProgressStep{{
		Kind: writertestutil.FakeStageStarted, Stage: writer.StageGeneration,
		Reached: reachedCh, Release: releaseCh,
	}}
	var once sync.Once
	return reachedCh, func() { once.Do(func() { close(releaseCh) }) }
}

// splitWriter holds the legacy job's Write at a gate and runs the job API's
// WriteWithProgress through a fake, so one test can fill the generation slots
// with a job of each API.
type splitWriter struct {
	gate *gateWriter
	fake *writertestutil.FakeArticleWriter
}

func (w splitWriter) Write(ctx context.Context, t transcript.Transcript) (writer.Article, error) {
	return w.gate.Write(ctx, t)
}

func (w splitWriter) WriteWithProgress(ctx context.Context, t transcript.Transcript, o writer.ProgressObserver) (writer.Article, error) {
	return w.fake.WriteWithProgress(ctx, t, o)
}

// blockingBody is a request body whose first Read reports it on entered and
// then waits for release, so a test can keep a generation start inside its
// body read.
type blockingBody struct {
	entered chan<- struct{}
	release <-chan struct{}
	r       io.Reader
	once    sync.Once
}

func (b *blockingBody) Read(p []byte) (int, error) {
	b.once.Do(func() {
		b.entered <- struct{}{}
		<-b.release
	})
	return b.r.Read(p)
}

// testClock is a clock a test advances by hand. It is safe for concurrent use:
// the cache reads it from the handler and from the generation job.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock() *testClock {
	return &testClock{now: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// seedDone stores a done article for body's key without running a generation
// or taking a generation slot, and returns the key.
func (ts *testServer) seedDone(t *testing.T, body string) articlecache.Key {
	t.Helper()
	key := ts.keyer.Key(ts.validInput(t, body).key)
	outcome, run, _ := ts.cache.Begin(key, func() bool { return true })
	if outcome != articlecache.BeginStarted {
		t.Fatalf("Begin outcome = %d, want started", outcome)
	}
	run.CompletePostable(testArticle())
	return key
}

// TestStartAccepted checks that a valid generation start answers 202 with a
// key and a request ID, and that the key's status is running or done right
// away and done once the job ends.
func TestStartAccepted(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	key := acceptedKey(t, ts.start(strings.NewReader(validBody(testValidVideoURL, testValidRaw))))
	if state := ts.getStatus(t, key).Job.State; state != "running" && state != "done" {
		t.Errorf("state right after the start = %q, want running or done", state)
	}
	ts.waitJobs()
	if state := ts.getStatus(t, key).Job.State; state != "done" {
		t.Errorf("state after the job = %q, want done", state)
	}
}

// TestStartSameInputGeneratesOnce checks that resending the same input while
// its generation runs and after it is done returns the same key and calls the
// writer no further.
func TestStartSameInputGeneratesOnce(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	reached, release := holdAtDraft(ts.writer)
	defer release()
	body := validBody(testValidVideoURL, testValidRaw)

	key := acceptedKey(t, ts.start(strings.NewReader(body)))
	<-reached
	if again := acceptedKey(t, ts.start(strings.NewReader(body))); again != key {
		t.Errorf("key while running = %s, want %s", again, key)
	}
	release()
	ts.waitJobs()
	if again := acceptedKey(t, ts.startBody(body)); again != key {
		t.Errorf("key after done = %s, want %s", again, key)
	}
	if calls := len(ts.writer.RecordedCalls()); calls != 1 {
		t.Errorf("writer calls = %d, want 1", calls)
	}
	logs := ts.logs.String()
	if !strings.Contains(logs, "outcome=started") || strings.Count(logs, "outcome=existing") != 2 {
		t.Errorf("want one outcome=started and two outcome=existing lines:\n%s", logs)
	}
}

// TestStartKeyEquivalence checks that bodies that differ only in what never
// reaches the validated values give the same key: member order, whitespace,
// how a character is escaped, undeclared members, a video URL form that
// normalizes to the same URL, and an absent versus an empty optional member.
func TestStartKeyEquivalence(t *testing.T) {
	const raw = `"0:00\nHello\n"`
	groups := map[string][]string{
		"no_title": {
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `}`,
			`{"transcript":` + raw + `,"videoUrl":"` + testValidVideoURL + `"}`,
			"{ \"videoUrl\" : \"" + testValidVideoURL + "\" ,\n\t\"transcript\" : " + raw + " }",
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"extra":1}`,
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"extra":{"nested":"x"}}`,
			`{"videoUrl":"https://youtu.be/dQw4w9WgXcQ","transcript":` + raw + `}`,
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"title":"","channelName":"","description":""}`,
		},
		"title": {
			// The same title, once as UTF-8 and once as a JSON escape.
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"title":"` + "\u3042" + `"}`,
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"title":"\u3042"}`,
		},
	}
	ts := newJobsTestServer(t, jobsServerOptions{})
	keys := map[string]string{}
	for name, bodies := range groups {
		for i, body := range bodies {
			key := acceptedKey(t, ts.startBody(body))
			if i == 0 {
				keys[name] = key
				continue
			}
			if key != keys[name] {
				t.Errorf("%s body %d: key %s, want %s for %s", name, i, key, keys[name], body)
			}
		}
	}
	// The two groups differ in a validated value, so the check above could
	// tell a key that ignored the body.
	if keys["no_title"] == keys["title"] {
		t.Errorf("a body with a title has the key of one without")
	}
}

// TestStartRetriesFailed checks a failed key: resent while every generation
// slot is taken it is refused with too_many_requests and stays failed with its
// reason; resent with a slot free it is regenerated under the same key.
func TestStartRetriesFailed(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	body := validBody(testValidVideoURL, testValidRaw)
	ts.writer.Err = errors.New("the provider refused")
	key := acceptedKey(t, ts.startBody(body))
	if job := ts.getStatus(t, key).Job; job.State != "failed" || job.Failure != "llm_failed" {
		t.Fatalf("job = %+v, want failed with llm_failed", job)
	}
	ts.writer.Err = nil

	for range maxInFlight {
		ts.slots <- struct{}{}
	}
	requireRejected(t, ts.startBody(body), http.StatusTooManyRequests, "too_many_requests")
	if job := ts.getStatus(t, key).Job; job.State != "failed" || job.Failure != "llm_failed" {
		t.Errorf("job after the refused retry = %+v, want failed with llm_failed", job)
	}
	for range maxInFlight {
		<-ts.slots
	}

	if again := acceptedKey(t, ts.startBody(body)); again != key {
		t.Errorf("retry key = %s, want %s", again, key)
	}
	if job := ts.getStatus(t, key).Job; job.State != "done" {
		t.Errorf("job after the retry = %+v, want done", job)
	}
	if calls := len(ts.writer.RecordedCalls()); calls != 2 {
		t.Errorf("writer calls = %d, want 2", calls)
	}
}

// TestStartGenerationSlotsShared fills the two generation slots with a legacy
// job and a generation job, and checks that a new key is refused with
// too_many_requests and stays unknown, while a running key and a done key are
// still answered with their keys.
func TestStartGenerationSlotsShared(t *testing.T) {
	gate := newGateWriter()
	fake := &writertestutil.FakeArticleWriter{Result: testArticle()}
	reached, release := holdAtDraft(fake)
	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.Server.writer = splitWriter{gate: gate, fake: fake}
	var once sync.Once
	releaseGate := func() { once.Do(func() { close(gate.release) }) }
	defer func() {
		release()
		releaseGate()
		ts.waitJobs()
	}()
	doneKey := ts.seedDone(t, bodyWith("done"))

	if rec := ts.serve(newRequest(http.MethodPost, ArticlesPath, strings.NewReader(bodyWith("legacy")))); rec.Code != http.StatusAccepted {
		t.Fatalf("legacy request status = %d, want 202", rec.Code)
	}
	<-gate.entered
	runningKey := acceptedKey(t, ts.start(strings.NewReader(bodyWith("running"))))
	<-reached
	if len(ts.slots) != maxInFlight {
		t.Fatalf("generation slots held = %d, want %d", len(ts.slots), maxInFlight)
	}

	requireRejected(t, ts.start(strings.NewReader(bodyWith("new"))), http.StatusTooManyRequests, "too_many_requests")
	newKey := ts.keyer.Key(ts.validInput(t, bodyWith("new")).key)
	if state := ts.getStatus(t, newKey.String()).Job.State; state != "unknown" {
		t.Errorf("refused key state = %q, want unknown", state)
	}
	if !strings.Contains(ts.logs.String(), "detail=generation_slot") {
		t.Errorf("the refusal does not log the generation slot detail:\n%s", ts.logs.String())
	}
	if got := acceptedKey(t, ts.start(strings.NewReader(bodyWith("running")))); got != runningKey {
		t.Errorf("running key resent: %s, want %s", got, runningKey)
	}
	if got := acceptedKey(t, ts.start(strings.NewReader(bodyWith("done")))); got != doneKey.String() {
		t.Errorf("done key resent: %s, want %s", got, doneKey)
	}
	if calls := len(fake.RecordedCalls()); calls != 1 {
		t.Errorf("generation writer calls = %d, want 1", calls)
	}
}

// TestNewAPINeverNotifies checks that no generation failure and no key
// rejection of the job API posts a failure notice. The body rejections are
// checked for it by TestStartRejectsLikeArticles.
func TestNewAPINeverNotifies(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.writer.Err = errors.New("the provider refused")
	acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))
	ts.writer.Err = nil
	ts.writer.Result = writer.Article{}
	acceptedKey(t, ts.startBody(bodyWith("invalid article")))
	requireRejected(t, ts.statusRequest("nope"), http.StatusBadRequest, "invalid_key")
	if len(ts.notifier.Calls) != 0 {
		t.Errorf("notifier calls = %+v, want none", ts.notifier.Calls)
	}
}

// TestStartReadSlotsFull fills the reading slots with two starts held inside
// their body reads, and checks that a third start is refused with
// too_many_requests, closing the connection, without reading a byte of its
// body.
func TestStartReadSlotsFull(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	entered := make(chan struct{}, maxReading)
	release := make(chan struct{})
	var held sync.WaitGroup
	for i := range maxReading {
		body := &blockingBody{entered: entered, release: release, r: strings.NewReader(bodyWith(fmt.Sprint(i)))}
		held.Go(func() { ts.start(body) })
	}
	for range maxReading {
		<-entered
	}

	counter := &recordingReader{r: strings.NewReader(validBody(testValidVideoURL, testValidRaw))}
	rec := ts.start(counter)
	requireRejected(t, rec, http.StatusTooManyRequests, "too_many_requests")
	if got := rec.Header().Get("Connection"); got != "close" {
		t.Errorf("Connection = %q, want close", got)
	}
	if counter.n != 0 {
		t.Errorf("the refused start read %d bytes of its body, want 0", counter.n)
	}
	if !strings.Contains(ts.logs.String(), "detail=read_slot") {
		t.Errorf("the refusal does not log the reading slot detail:\n%s", ts.logs.String())
	}
	close(release)
	held.Wait()
	ts.waitJobs()
	if len(ts.reading) != 0 {
		t.Errorf("reading slots held = %d, want 0", len(ts.reading))
	}
}

// TestStartConcurrentSameKey sends the same input from as many goroutines at
// once as there are reading slots (more would be refused by them, by design)
// while the writer is held, for a new key and for a failed one, and checks
// that every start gets the same key and the writer runs once. It checks the
// outcome only: the senders are not lined up at Begin, so the atomicity of
// the check and the slot take is proven by articlecache's concurrent Begin
// test.
func TestStartConcurrentSameKey(t *testing.T) {
	const senders = maxReading
	body := validBody(testValidVideoURL, testValidRaw)
	sendAll := func(t *testing.T, ts *testServer) {
		t.Helper()
		recs := make([]*httptest.ResponseRecorder, senders)
		var wg sync.WaitGroup
		for i := range senders {
			wg.Go(func() { recs[i] = ts.start(strings.NewReader(body)) })
		}
		wg.Wait()
		keys := make([]string, senders)
		for i, rec := range recs {
			keys[i] = acceptedKey(t, rec)
			if keys[i] != keys[0] {
				t.Errorf("start %d: key %s, want %s", i, keys[i], keys[0])
			}
		}
	}

	t.Run("new", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		_, release := holdAtDraft(ts.writer)
		defer release()
		sendAll(t, ts)
		release()
		ts.waitJobs()
		if calls := len(ts.writer.RecordedCalls()); calls != 1 {
			t.Errorf("writer calls = %d, want 1", calls)
		}
	})
	t.Run("failed", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		ts.writer.Err = errors.New("the provider refused")
		acceptedKey(t, ts.startBody(body))
		ts.writer.Err = nil
		_, release := holdAtDraft(ts.writer)
		defer release()
		sendAll(t, ts)
		release()
		ts.waitJobs()
		if calls := len(ts.writer.RecordedCalls()); calls != 2 {
			t.Errorf("writer calls = %d, want 2: the failed one and one retry", calls)
		}
	})
}

// TestStartEvictsOldest fills a cache of two with done articles and checks
// that a new key drops the one that was done first, which becomes unknown,
// and keeps the other.
func TestStartEvictsOldest(t *testing.T) {
	clock := newTestClock()
	ts := newJobsTestServer(t, jobsServerOptions{capacity: 2, clock: clock.Now})
	first := acceptedKey(t, ts.startBody(bodyWith("first")))
	clock.Advance(time.Minute)
	second := acceptedKey(t, ts.startBody(bodyWith("second")))
	clock.Advance(time.Minute)
	third := acceptedKey(t, ts.startBody(bodyWith("third")))

	for key, want := range map[string]string{first: "unknown", second: "done", third: "done"} {
		if got := ts.getStatus(t, key).Job.State; got != want {
			t.Errorf("key %s state = %q, want %q", key, got, want)
		}
	}
	if line := logLine(ts.logs.String(), "article record evicted"); !strings.Contains(line, "key="+first) || !strings.Contains(line, "state=done") {
		t.Errorf("the eviction log line = %q, want the dropped key and state=done", line)
	}
}

// TestStartCacheFull checks that when the cache is full of records it may not
// drop (here, a running one), a new key is refused with too_many_requests and
// the existing record is left as it was.
func TestStartCacheFull(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{capacity: 1})
	reached, release := holdAtDraft(ts.writer)
	defer func() {
		release()
		ts.waitJobs()
	}()
	running := acceptedKey(t, ts.start(strings.NewReader(bodyWith("running"))))
	<-reached
	before := ts.cache.DumpRecordsForTest()

	requireRejected(t, ts.start(strings.NewReader(bodyWith("new"))), http.StatusTooManyRequests, "too_many_requests")
	if after := ts.cache.DumpRecordsForTest(); after != before {
		t.Errorf("the refusal changed the records:\nbefore %s\nafter  %s", before, after)
	}
	if got := ts.getStatus(t, running).Job.State; got != "running" {
		t.Errorf("running key state = %q, want running", got)
	}
	if !strings.Contains(ts.logs.String(), "detail=cache_full") {
		t.Errorf("the refusal does not log the cache-full detail:\n%s", ts.logs.String())
	}
	if len(ts.slots) != 1 {
		t.Errorf("generation slots held = %d, want only the running job's", len(ts.slots))
	}
}

// TestArticlesIgnoresCache checks that the legacy endpoint neither reads nor
// writes the article cache: the same input sent twice is generated and posted
// twice, and its key stays unknown.
func TestArticlesIgnoresCache(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	body := validBody(testValidVideoURL, testValidRaw)
	for range 2 {
		if rec := ts.serveAccepted(t, body); rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", rec.Code)
		}
	}
	if len(ts.writer.Calls) != 2 || len(ts.publisher.Calls) != 2 {
		t.Errorf("writer calls = %d, publisher calls = %d, want 2 each", len(ts.writer.Calls), len(ts.publisher.Calls))
	}
	key := ts.keyer.Key(ts.validInput(t, body).key)
	if state := ts.getStatus(t, key.String()).Job.State; state != "unknown" {
		t.Errorf("key state = %q, want unknown", state)
	}
	if dump := ts.cache.DumpRecordsForTest(); dump != "" {
		t.Errorf("the legacy endpoint added a record:\n%s", dump)
	}
}

// TestJobsTargetRejections checks the request targets, methods, and bodies the
// job API refuses before it looks a key up, on a server holding a done
// record: the target is matched before decoding, so an escaped character in
// the key part is never read as the character it encodes, and no rejection
// changes the record.
func TestJobsTargetRejections(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	key := ts.seedDone(t, validBody(testValidVideoURL, testValidRaw)).String()
	if got := ts.getStatus(t, key).Job.State; got != "done" {
		t.Fatalf("the seeded key is %q, want done", got)
	}
	// escaped is the key with its first character percent-encoded: decoding
	// it gives the seeded key back.
	escaped := fmt.Sprintf("%%%02X", key[0]) + key[1:]
	tests := []struct {
		name    string
		method  string
		target  string
		status  int
		reason  string
		allow   string
		closeIt bool
	}{
		{"publish_extra_segment", http.MethodGet, JobsPath + "/" + key + "/publish/x", http.StatusNotFound, "not_found", "", true},
		{"other_segment", http.MethodGet, JobsPath + "/" + key + "/x", http.StatusNotFound, "not_found", "", true},
		{"status_query", http.MethodGet, JobsPath + "/" + key + "?x", http.StatusNotFound, "not_found", "", true},
		{"start_query", http.MethodPost, JobsPath + "?x=1", http.StatusNotFound, "not_found", "", true},
		{"jobs_prefix", http.MethodPost, JobsPath + "x", http.StatusNotFound, "not_found", "", true},
		{"escaped_jobs", http.MethodGet, "/%6Aobs/" + key, http.StatusNotFound, "not_found", "", true},
		{"uppercase_key", http.MethodGet, JobsPath + "/" + strings.ToUpper(key), http.StatusBadRequest, "invalid_key", "", false},
		{"escaped_key", http.MethodGet, JobsPath + "/" + escaped, http.StatusBadRequest, "invalid_key", "", false},
		{"escaped_slash", http.MethodGet, JobsPath + "/" + key[:32] + "%2F" + key[35:], http.StatusBadRequest, "invalid_key", "", false},
		{"empty_key", http.MethodGet, JobsPath + "/", http.StatusBadRequest, "invalid_key", "", false},
		{"short_key", http.MethodGet, JobsPath + "/" + key[:63], http.StatusBadRequest, "invalid_key", "", false},
		{"long_key", http.MethodGet, JobsPath + "/" + key + "0", http.StatusBadRequest, "invalid_key", "", false},
		{"padded_key", http.MethodGet, JobsPath + "/%20" + key + "%20", http.StatusBadRequest, "invalid_key", "", false},
		{"non_hex_key", http.MethodGet, JobsPath + "/" + strings.Repeat("g", 64), http.StatusBadRequest, "invalid_key", "", false},
		{"status_post", http.MethodPost, JobsPath + "/" + key, http.StatusMethodNotAllowed, "method_not_allowed", http.MethodGet, true},
		{"start_get", http.MethodGet, JobsPath, http.StatusMethodNotAllowed, "method_not_allowed", http.MethodPost, true},
		{"start_put", http.MethodPut, JobsPath, http.StatusMethodNotAllowed, "method_not_allowed", http.MethodPost, true},
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
		name  string
		build func(body io.Reader) *http.Request
	}{
		{"content_length", func(body io.Reader) *http.Request {
			req := newRequest(http.MethodGet, JobsPath+"/"+key, body)
			req.ContentLength = 1
			return req
		}},
		{"transfer_encoding", func(body io.Reader) *http.Request {
			req := newRequest(http.MethodGet, JobsPath+"/"+key, body)
			req.ContentLength = -1
			req.TransferEncoding = []string{"chunked"}
			return req
		}},
	}
	for _, tc := range bodies {
		t.Run("body_"+tc.name, func(t *testing.T) {
			counter := &recordingReader{r: strings.NewReader("x")}
			rec := ts.serve(tc.build(io.NopCloser(counter)))
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
}

// TestStartHandlerPanicCleanup checks the start handler's recovery: a panic
// after Begin and before the job takes over fails the record as internal and
// frees both slots; a panic inside the body checks, before Begin, frees the
// reading slot and adds no record. Neither posts a notice.
func TestStartHandlerPanicCleanup(t *testing.T) {
	t.Run("after_begin", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		ts.beforeGenerationJob = func() { panic("test hook panic") }
		body := validBody(testValidVideoURL, testValidRaw)
		requireRejected(t, ts.startBody(body), http.StatusInternalServerError, "internal_error")
		key := ts.keyer.Key(ts.validInput(t, body).key)
		if job := ts.getStatus(t, key.String()).Job; job.State != "failed" || job.Failure != "internal" {
			t.Errorf("job = %+v, want failed with internal", job)
		}
		if len(ts.slots) != 0 || len(ts.reading) != 0 {
			t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
		}
		if len(ts.writer.Calls) != 0 || len(ts.notifier.Calls) != 0 {
			t.Errorf("side effects: writer=%d notifier=%d, want none", len(ts.writer.Calls), len(ts.notifier.Calls))
		}
	})
	t.Run("body_check", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		rec := ts.start(panickingReader{})
		ts.waitJobs()
		requireRejected(t, rec, http.StatusInternalServerError, "internal_error")
		if dump := ts.cache.DumpRecordsForTest(); dump != "" {
			t.Errorf("the panic left a record:\n%s", dump)
		}
		if len(ts.slots) != 0 || len(ts.reading) != 0 {
			t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
		}
		if len(ts.notifier.Calls) != 0 {
			t.Errorf("notifier calls = %d, want 0", len(ts.notifier.Calls))
		}
	})
}

// TestGenerationPanicServerResponds checks that after a generation job
// panics, the server goes on answering: the key reports the failure, and a
// later start is accepted.
func TestGenerationPanicServerResponds(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.Server.writer = panickingWriter{}
	key := acceptedKey(t, ts.startBody(bodyWith("panics")))
	if job := ts.getStatus(t, key).Job; job.State != "failed" || job.Failure != "internal" {
		t.Errorf("job = %+v, want failed with internal", job)
	}
	ts.Server.writer = ts.writer
	next := acceptedKey(t, ts.startBody(bodyWith("next")))
	if state := ts.getStatus(t, next).Job.State; state != "done" {
		t.Errorf("the next start's state = %q, want done", state)
	}
}
