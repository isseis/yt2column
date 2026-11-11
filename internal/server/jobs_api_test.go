//go:build test

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/articlecache"
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
	subject := ts.keyer.Subject(ts.validInput(t, body).key)
	begun := ts.cache.Begin(subject, articlecache.StartPlain, func() bool { return true })
	if begun.Outcome != articlecache.BeginStarted {
		t.Fatalf("Begin outcome = %d, want started", begun.Outcome)
	}
	begun.Run.Complete(testArticle())
	return subject.Key()
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
// normalizes to the same URL, an absent versus an empty optional member, and
// the send mode.
func TestStartKeyEquivalence(t *testing.T) {
	const raw = `"0:00\nHello\n"`
	groups := map[string][]string{
		"no_title": {
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"mode":"plain"}`,
			`{"transcript":` + raw + `,"videoUrl":"` + testValidVideoURL + `","mode":"plain"}`,
			"{ \"videoUrl\" : \"" + testValidVideoURL + "\" ,\n\t\"transcript\" : " + raw + ",\"mode\":\"plain\" }",
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"extra":1,"mode":"plain"}`,
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"extra":{"nested":"x"},"mode":"plain"}`,
			`{"videoUrl":"https://youtu.be/dQw4w9WgXcQ","transcript":` + raw + `,"mode":"plain"}`,
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"title":"","channelName":"","description":"","mode":"plain"}`,
			// The mode is not part of the key. Not first: an inquiry start
			// with no record of its own would be answered with the other
			// group's article of the same video.
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"mode":"inquiry"}`,
		},
		"title": {
			// The same title, once as UTF-8 and once as a JSON escape.
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"title":"` + "\u3042" + `","mode":"plain"}`,
			`{"videoUrl":"` + testValidVideoURL + `","transcript":` + raw + `,"title":"\u3042","mode":"plain"}`,
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

// TestStartGenerationSlotsFull fills the generation slots with generation
// jobs, and checks that a new key is refused with too_many_requests and stays
// unknown, while a running key and a done key are still answered with their
// keys.
func TestStartGenerationSlotsFull(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	reached, release := holdAtDraft(ts.writer)
	defer func() {
		release()
		ts.waitJobs()
	}()
	doneKey := ts.seedDone(t, bodyWith("done"))

	var runningKey string
	for i := range maxInFlight {
		runningKey = acceptedKey(t, ts.start(strings.NewReader(bodyWith(fmt.Sprint("running", i)))))
		<-reached
	}
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
	if got := acceptedKey(t, ts.start(strings.NewReader(bodyWith(fmt.Sprint("running", maxInFlight-1))))); got != runningKey {
		t.Errorf("running key resent: %s, want %s", got, runningKey)
	}
	if got := acceptedKey(t, ts.start(strings.NewReader(bodyWith("done")))); got != doneKey.String() {
		t.Errorf("done key resent: %s, want %s", got, doneKey)
	}
	if calls := len(ts.writer.RecordedCalls()); calls != maxInFlight {
		t.Errorf("generation writer calls = %d, want %d", calls, maxInFlight)
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

// TestUnknownPathTouchesNothing checks that a target the server does not
// serve is refused as not_found, closing the connection, whatever the method,
// without reading a byte of the body or taking a slot: with every generation
// and reading slot already taken the answer is still not_found, not
// too_many_requests. No writer call and no record follow.
func TestUnknownPathTouchesNothing(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			for range maxInFlight {
				ts.slots <- struct{}{}
			}
			for range maxReading {
				ts.reading <- struct{}{}
			}
			body := &recordingReader{r: strings.NewReader(validBody(testValidVideoURL, testValidRaw))}
			req := newRequest(method, "/unknown", io.NopCloser(body))
			rec := ts.serve(req)
			ts.waitJobs()
			requireRejected(t, rec, http.StatusNotFound, "not_found")
			if got := rec.Header().Get("Connection"); got != "close" {
				t.Errorf("Connection = %q, want close", got)
			}
			if body.n != 0 {
				t.Errorf("read %d bytes of the body, want 0", body.n)
			}
			if len(ts.slots) != maxInFlight || len(ts.reading) != maxReading {
				t.Errorf("slots held: generation %d, reading %d, want the %d and %d the test took",
					len(ts.slots), len(ts.reading), maxInFlight, maxReading)
			}
			if len(ts.writer.Calls) != 0 {
				t.Errorf("side effects: writer=%d, want none", len(ts.writer.Calls))
			}
			if dump := ts.cache.DumpRecordsForTest(); dump != "" {
				t.Errorf("a rejected request added a record:\n%s", dump)
			}
		})
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
// frees both slots, for a plain and an inquiry start; a panic while an inquiry
// start's same-video answer is written leaves the cache as it was; a panic
// inside the body checks, before Begin, frees the reading slot and adds no
// record. After each, the server answers the next start.
func TestStartHandlerPanicCleanup(t *testing.T) {
	for _, mode := range []string{modePlain, modeInquiry} {
		t.Run("after_begin_"+mode, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			ts.beforeGenerationJob = func() { panic("test hook panic") }
			body := modeBody(testValidVideoURL, testValidRaw, mode)
			requireRejected(t, ts.startBody(body), http.StatusInternalServerError, "internal_error")
			key := ts.keyer.Key(ts.validInput(t, body).key)
			if job := ts.getStatus(t, key.String()).Job; job.State != "failed" || job.Failure != "internal" {
				t.Errorf("job = %+v, want failed with internal", job)
			}
			if len(ts.slots) != 0 || len(ts.reading) != 0 {
				t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
			}
			if len(ts.writer.Calls) != 0 {
				t.Errorf("writer calls = %d, want 0", len(ts.writer.Calls))
			}
			ts.beforeGenerationJob = nil
			acceptedKey(t, ts.startBody(bodyWith("next")))
		})
	}
	t.Run("same_video_answer", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		done := ts.seedDone(t, bodyWith("done"))
		before := ts.cache.DumpRecordsForTest()
		ts.beforeSameVideoAnswer = func() { panic("test hook panic") }
		requireRejected(t, ts.startBody(inquiryWith("inquiry")), http.StatusInternalServerError, "internal_error")
		if after := ts.cache.DumpRecordsForTest(); after != before {
			t.Errorf("the panic changed the records:\nbefore %s\nafter  %s", before, after)
		}
		if len(ts.slots) != 0 || len(ts.reading) != 0 {
			t.Errorf("slots held: generation %d, reading %d, want 0", len(ts.slots), len(ts.reading))
		}
		ts.beforeSameVideoAnswer = nil
		if got, _ := sameVideoAnswer(t, ts.startBody(inquiryWith("inquiry"))); got != done.String() {
			t.Errorf("the next start's key = %s, want %s", got, done)
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

// otherVideoURL names a video other than testVideoID's.
const otherVideoURL = "https://www.youtube.com/watch?v=abcdefghijk"

// startJSON returns a generation start's body with members as its string
// members.
func startJSON(t *testing.T, members map[string]string) string {
	t.Helper()
	data, err := json.Marshal(members)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

// inquiryWith returns an inquiry start's body for testValidVideoURL whose
// transcript text is text.
func inquiryWith(text string) string {
	return modeBody(testValidVideoURL, "0:00\n"+text+"\n", modeInquiry)
}

// sameVideoAnswer checks that rec is an inquiry start's same-video answer: a
// 200 whose members are exactly status, key, and requestId, with status
// same_video. It returns the key and the request ID.
func sameVideoAnswer(t *testing.T, rec *httptest.ResponseRecorder) (key, requestID string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var members map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &members); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if got := slices.Sorted(maps.Keys(members)); !slices.Equal(got, []string{"key", "requestId", "status"}) {
		t.Errorf("members = %v, want key, requestId, status", got)
	}
	body := decodeBody(t, rec)
	if body.Status != "same_video" || !keyShape.MatchString(body.Key) || !requestIDShape.MatchString(body.RequestID) {
		t.Fatalf("body = %+v, want same_video with a key and a request ID", body)
	}
	return body.Key, body.RequestID
}

// TestStartInquirySameVideo checks that an inquiry start for a video that
// already has a done or a running article under another key is answered
// with that article's key, whichever URL form names the video and whichever
// of the transcript and the metadata differs: no generation, no change to the
// cache, and one log line that carries the fixed attributes
// and neither the request's URL string nor its text.
func TestStartInquirySameVideo(t *testing.T) {
	const sentURL = "https://youtu.be/" + testVideoID
	tests := []struct {
		name    string
		running bool // the existing article is held running, else done
		body    func(t *testing.T) string
	}{
		{"done_short_url_other_transcript", false, func(*testing.T) string {
			return modeBody(sentURL, "0:00\n"+testTranscriptMarker+"\n", modeInquiry)
		}},
		{"done_same_url_other_transcript", false, func(*testing.T) string {
			return inquiryWith(testTranscriptMarker)
		}},
		{"running_other_metadata", true, func(t *testing.T) string {
			return startJSON(t, map[string]string{
				"videoUrl": sentURL, "transcript": testValidRaw, "mode": modeInquiry,
				"title": testTranscriptMarker, "channelName": testTranscriptMarker, "description": testTranscriptMarker,
			})
		}},
		{"running_other_transcript", true, func(*testing.T) string {
			return inquiryWith(testTranscriptMarker)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			var existing string
			if tc.running {
				reached, release := holdAtDraft(ts.writer)
				t.Cleanup(func() {
					release()
					ts.waitJobs()
				})
				existing = acceptedKey(t, ts.start(strings.NewReader(validBody(testValidVideoURL, testValidRaw))))
				<-reached
			} else {
				existing = acceptedKey(t, ts.startBody(validBody(testValidVideoURL, testValidRaw)))
			}
			before := ts.cache.DumpRecordsForTest()
			logsBefore := ts.logs.Len()
			body := tc.body(t)
			own := ts.keyer.Key(ts.validInput(t, body).key).String()
			if own == existing {
				t.Fatalf("the inquiry has the existing article's key: the row tests nothing")
			}

			key, requestID := sameVideoAnswer(t, ts.start(strings.NewReader(body)))
			if key != existing {
				t.Errorf("same-video key = %s, want the existing article's %s", key, existing)
			}
			if after := ts.cache.DumpRecordsForTest(); after != before {
				t.Errorf("the answer changed the records:\nbefore %s\nafter  %s", before, after)
			}
			if calls := len(ts.writer.RecordedCalls()); calls != 1 {
				t.Errorf("writer calls = %d, want only the existing article's 1", calls)
			}
			wantSlots := 0
			if tc.running {
				wantSlots = 1 // the existing generation's
			}
			if len(ts.slots) != wantSlots || len(ts.reading) != 0 {
				t.Errorf("slots held: generation %d, reading %d, want %d and 0", len(ts.slots), len(ts.reading), wantSlots)
			}

			var lines []string
			for line := range strings.Lines(ts.logs.String()[logsBefore:]) {
				if strings.Contains(line, "requestId="+requestID) {
					lines = append(lines, line)
				}
			}
			if len(lines) != 1 {
				t.Fatalf("log lines with the request ID = %d, want 1:\n%s", len(lines), strings.Join(lines, ""))
			}
			line := lines[0]
			for _, want := range []string{
				"generation start answered with a same-video article",
				"key=" + own, "videoId=" + testVideoID, "outcome=same_video", "mode=inquiry", "sameVideoKey=" + existing,
			} {
				if !strings.Contains(line, want) {
					t.Errorf("the log line lacks %q: %s", want, line)
				}
			}
			logs := ts.logs.String()
			for _, leak := range []string{sentURL, testValidVideoURL, testTranscriptMarker} {
				if strings.Contains(logs, leak) {
					t.Errorf("the log carries %q:\n%s", leak, logs)
				}
			}
		})
	}
}

// TestStartInquiryNoSameVideo checks that an inquiry start starts a
// generation and is answered as accepted, under its own key, when the only
// other records are a failed one of the same video or an article of another
// video.
func TestStartInquiryNoSameVideo(t *testing.T) {
	tests := []struct {
		name string
		seed func(t *testing.T, ts *testServer)
	}{
		{"failed_same_video", func(t *testing.T, ts *testServer) {
			ts.writer.Err = errors.New("the provider refused")
			key := acceptedKey(t, ts.startBody(bodyWith("failed")))
			ts.writer.Err = nil
			if state := ts.getStatus(t, key).Job.State; state != "failed" {
				t.Fatalf("seeded state = %q, want failed", state)
			}
		}},
		{"other_video", func(t *testing.T, ts *testServer) {
			acceptedKey(t, ts.startBody(validBody(otherVideoURL, testValidRaw)))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := newJobsTestServer(t, jobsServerOptions{})
			tc.seed(t, ts)
			body := inquiryWith("inquiry")
			key := acceptedKey(t, ts.startBody(body))
			if want := ts.keyer.Key(ts.validInput(t, body).key).String(); key != want {
				t.Errorf("key = %s, want the inquiry's own %s", key, want)
			}
			if calls := len(ts.writer.RecordedCalls()); calls != 2 {
				t.Errorf("writer calls = %d, want 2: the seed and the inquiry", calls)
			}
			if !strings.Contains(ts.logs.String(), "outcome=started mode=inquiry") {
				t.Errorf("no start line with outcome=started mode=inquiry:\n%s", ts.logs.String())
			}
		})
	}
}

// TestStartInquiryOwnKey checks the inquiry start's own key against another
// done article of the same video: a running or done own key is answered as
// accepted with that key and generates nothing; a failed own key is answered
// with the other article and is left failed, not retried.
func TestStartInquiryOwnKey(t *testing.T) {
	own := inquiryWith("own")
	t.Run("running", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		ts.seedDone(t, bodyWith("other"))
		reached, release := holdAtDraft(ts.writer)
		defer func() {
			release()
			ts.waitJobs()
		}()
		key := acceptedKey(t, ts.start(strings.NewReader(bodyWith("own"))))
		<-reached
		if got := acceptedKey(t, ts.start(strings.NewReader(own))); got != key {
			t.Errorf("key = %s, want the running own key %s", got, key)
		}
		if calls := len(ts.writer.RecordedCalls()); calls != 1 {
			t.Errorf("writer calls = %d, want 1", calls)
		}
	})
	t.Run("done", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		ts.seedDone(t, bodyWith("other"))
		key := acceptedKey(t, ts.startBody(bodyWith("own")))
		if got := acceptedKey(t, ts.startBody(own)); got != key {
			t.Errorf("key = %s, want the done own key %s", got, key)
		}
		if calls := len(ts.writer.RecordedCalls()); calls != 1 {
			t.Errorf("writer calls = %d, want 1", calls)
		}
	})
	t.Run("failed", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		ts.writer.Err = errors.New("the provider refused")
		failed := acceptedKey(t, ts.startBody(bodyWith("own")))
		ts.writer.Err = nil
		other := ts.seedDone(t, bodyWith("other"))
		if got, _ := sameVideoAnswer(t, ts.startBody(own)); got != other.String() {
			t.Errorf("key = %s, want the other article's %s", got, other)
		}
		if job := ts.getStatus(t, failed).Job; job.State != "failed" || job.Failure != "llm_failed" {
			t.Errorf("own job = %+v, want still failed with llm_failed", job)
		}
		if calls := len(ts.writer.RecordedCalls()); calls != 1 {
			t.Errorf("writer calls = %d, want only the failed 1", calls)
		}
	})
}

// TestStartInquiryLatest checks that with two articles of the same video the
// same-video answer names the one accepted last, and that a failed record
// retried later counts from its retry.
func TestStartInquiryLatest(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	ts.writer.Err = errors.New("the provider refused")
	retried := acceptedKey(t, ts.startBody(bodyWith("retried")))
	ts.writer.Err = nil
	first := acceptedKey(t, ts.startBody(bodyWith("first")))
	second := acceptedKey(t, ts.startBody(bodyWith("second")))
	if got, _ := sameVideoAnswer(t, ts.startBody(inquiryWith("inquiry"))); got != second {
		t.Errorf("before the retry: key = %s, want the later done %s (not %s)", got, second, first)
	}
	if got := acceptedKey(t, ts.startBody(bodyWith("retried"))); got != retried {
		t.Fatalf("retry key = %s, want %s", got, retried)
	}
	if got, _ := sameVideoAnswer(t, ts.startBody(inquiryWith("inquiry"))); got != retried {
		t.Errorf("after the retry: key = %s, want the retried %s", got, retried)
	}
}

// TestStartInquiryWhenFull checks that an article of the same video is
// returned even when no generation slot is free and when the cache is full of
// records it may not drop, while in the same state an inquiry for another
// video is refused with too_many_requests, which proves the state is full.
func TestStartInquiryWhenFull(t *testing.T) {
	t.Run("generation_slots", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{})
		done := ts.seedDone(t, bodyWith("done"))
		for range maxInFlight {
			ts.slots <- struct{}{}
		}
		requireRejected(t, ts.startBody(modeBody(otherVideoURL, testValidRaw, modeInquiry)), http.StatusTooManyRequests, "too_many_requests")
		if !strings.Contains(ts.logs.String(), "detail=generation_slot") {
			t.Errorf("the other video's refusal is not the generation-slot one:\n%s", ts.logs.String())
		}
		if got, _ := sameVideoAnswer(t, ts.startBody(inquiryWith("inquiry"))); got != done.String() {
			t.Errorf("key = %s, want %s", got, done)
		}
		if len(ts.slots) != maxInFlight {
			t.Errorf("generation slots held = %d, want the %d the test took", len(ts.slots), maxInFlight)
		}
	})
	t.Run("cache_full", func(t *testing.T) {
		ts := newJobsTestServer(t, jobsServerOptions{capacity: 1})
		reached, release := holdAtDraft(ts.writer)
		defer func() {
			release()
			ts.waitJobs()
		}()
		running := acceptedKey(t, ts.start(strings.NewReader(bodyWith("running"))))
		<-reached
		requireRejected(t, ts.start(strings.NewReader(modeBody(otherVideoURL, testValidRaw, modeInquiry))), http.StatusTooManyRequests, "too_many_requests")
		if !strings.Contains(ts.logs.String(), "detail=cache_full") {
			t.Errorf("the other video's refusal is not the cache-full one:\n%s", ts.logs.String())
		}
		if got, _ := sameVideoAnswer(t, ts.start(strings.NewReader(inquiryWith("inquiry")))); got != running {
			t.Errorf("key = %s, want %s", got, running)
		}
	})
}

// TestStartConcurrentSameVideo sends inquiry starts for different inputs of
// one video from as many goroutines at once as there are reading slots while
// the writer is held, and checks that exactly one starts a generation and
// every other is answered with its key. Like TestStartConcurrentSameKey it
// checks the outcome only; articlecache's concurrent same-video test lines the
// calls up at Begin.
func TestStartConcurrentSameVideo(t *testing.T) {
	const senders = maxReading
	ts := newJobsTestServer(t, jobsServerOptions{})
	_, release := holdAtDraft(ts.writer)
	defer func() {
		release()
		ts.waitJobs()
	}()
	recs := make([]*httptest.ResponseRecorder, senders)
	var wg sync.WaitGroup
	for i := range senders {
		wg.Go(func() { recs[i] = ts.start(strings.NewReader(inquiryWith(fmt.Sprint("sender", i)))) })
	}
	wg.Wait()
	var started string
	var answered []string
	for _, rec := range recs {
		if rec.Code == http.StatusAccepted {
			if started != "" {
				t.Fatalf("more than one start was accepted")
			}
			started = acceptedKey(t, rec)
			continue
		}
		key, _ := sameVideoAnswer(t, rec)
		answered = append(answered, key)
	}
	if started == "" {
		t.Fatalf("no start was accepted")
	}
	for _, key := range answered {
		if key != started {
			t.Errorf("same-video key = %s, want the started %s", key, started)
		}
	}
	release()
	ts.waitJobs()
	if calls := len(ts.writer.RecordedCalls()); calls != 1 {
		t.Errorf("writer calls = %d, want 1", calls)
	}
}

// TestStartPlainIgnoresSameVideo checks that a plain start generates its own
// input's article even though a done article of the same video exists.
func TestStartPlainIgnoresSameVideo(t *testing.T) {
	ts := newJobsTestServer(t, jobsServerOptions{})
	done := ts.seedDone(t, bodyWith("done"))
	key := acceptedKey(t, ts.startBody(bodyWith("plain")))
	if key == done.String() {
		t.Errorf("the plain start got the done article's key")
	}
	if calls := len(ts.writer.RecordedCalls()); calls != 1 {
		t.Errorf("writer calls = %d, want 1", calls)
	}
	if state := ts.getStatus(t, key).Job.State; state != "done" {
		t.Errorf("state = %q, want done", state)
	}
	if !strings.Contains(ts.logs.String(), "outcome=started mode=plain") {
		t.Errorf("no start line with outcome=started mode=plain:\n%s", ts.logs.String())
	}
}
