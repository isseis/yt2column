//go:build test

package articlecache

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/writer"
)

// fakeClock is a settable clock; tests advance it between transitions.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestCache(t *testing.T, capacity int) (*Cache, *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	c, err := New(Options{Capacity: capacity, Clock: clock.Now})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	return c, clock
}

// testKey returns a distinct key for each n.
func testKey(n int) Key {
	in := baseKeyInput()
	in.Transcript = strings.Repeat("x", n)
	return NewKeyer(testSecret).Key(in)
}

func always() bool { return true }
func never() bool  { return false }

// countingAcquire returns an acquire function and a pointer to its call count.
func countingAcquire(result bool) (func() bool, *int) {
	calls := 0
	return func() bool {
		calls++
		return result
	}, &calls
}

func testArticle(title string) writer.Article {
	return writer.Article{Title: title, Body: "body", SourceURL: "https://www.youtube.com/watch?v=abcdefghijk", Model: "m"}
}

// begin starts key and fails the test unless a generation started.
func begin(t *testing.T, c *Cache, key Key) *Run {
	t.Helper()
	outcome, run, _ := c.Begin(key, always)
	if outcome != BeginStarted && outcome != BeginRestarted {
		t.Fatalf("Begin outcome = %v, want started or restarted", outcome)
	}
	return run
}

// done starts key and completes it as postable.
func done(t *testing.T, c *Cache, key Key) {
	t.Helper()
	begin(t, c, key).CompletePostable(testArticle("t"))
}

// failed starts key and fails it.
func failed(t *testing.T, c *Cache, key Key) {
	t.Helper()
	begin(t, c, key).Fail(FailureLLM)
}

// posting starts key, completes it, and begins its post.
func posting(t *testing.T, c *Cache, key Key) *PublishRun {
	t.Helper()
	done(t, c, key)
	outcome, pub := c.BeginPublish(key)
	if outcome != PublishBeginStarted {
		t.Fatalf("BeginPublish outcome = %v, want started", outcome)
	}
	return pub
}

func TestNewOptions(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		if _, err := New(Options{Capacity: capacity}); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("New(Capacity %d) error = %v, want ErrInvalidOptions", capacity, err)
		}
	}
	c, err := New(Options{Capacity: 1})
	if err != nil {
		t.Fatalf("New(Capacity 1) error = %v", err)
	}
	if got := c.Lookup(testKey(1)).State; got != JobUnknown {
		t.Errorf("new cache Lookup state = %v, want JobUnknown", got)
	}
}

func TestBeginNewKey(t *testing.T) {
	c, _ := newTestCache(t, 2)
	acquire, calls := countingAcquire(true)
	outcome, run, evicted := c.Begin(testKey(1), acquire)
	if outcome != BeginStarted || run == nil || evicted != nil {
		t.Fatalf("Begin = %v, %v, %v; want BeginStarted, a run, no eviction", outcome, run, evicted)
	}
	if *calls != 1 {
		t.Errorf("acquire calls = %d, want 1", *calls)
	}
	if got := c.Lookup(testKey(1)); got.State != JobRunning || got.Stages != (Stages{}) {
		t.Errorf("Lookup = %+v, want running with every stage pending", got)
	}
}

func TestBeginNoSlot(t *testing.T) {
	c, _ := newTestCache(t, 2)
	acquire, calls := countingAcquire(false)
	if outcome, run, _ := c.Begin(testKey(1), acquire); outcome != BeginNoSlot || run != nil {
		t.Fatalf("Begin = %v, %v; want BeginNoSlot and no run", outcome, run)
	}
	if *calls != 1 {
		t.Errorf("acquire calls = %d, want 1", *calls)
	}
	if got := c.Lookup(testKey(1)).State; got != JobUnknown {
		t.Errorf("Lookup state = %v, want JobUnknown: nothing was created", got)
	}

	// A failed record stays failed when no slot is free.
	failed(t, c, testKey(2))
	if outcome, _, _ := c.Begin(testKey(2), never); outcome != BeginNoSlot {
		t.Fatalf("Begin(failed) = %v, want BeginNoSlot", outcome)
	}
	if got := c.Lookup(testKey(2)); got.State != JobFailed || got.Failure != FailureLLM {
		t.Errorf("Lookup = %+v, want the failure kept", got)
	}
}

func TestBeginExistingDoesNotAcquire(t *testing.T) {
	c, _ := newTestCache(t, 10)
	begin(t, c, testKey(1))
	done(t, c, testKey(2))
	posting(t, c, testKey(3))
	pub := posting(t, c, testKey(4))
	pub.Finish()
	pub = posting(t, c, testKey(5))
	pub.Fail(PublishFailureWebhook)

	cases := map[string]struct {
		key     Key
		state   JobState
		publish PublishState
	}{
		"running":        {testKey(1), JobRunning, PublishUnpostable},
		"not posted":     {testKey(2), JobDone, PublishNotPosted},
		"posting":        {testKey(3), JobDone, PublishPosting},
		"posted":         {testKey(4), JobDone, PublishPosted},
		"publish failed": {testKey(5), JobDone, PublishFailed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			before := c.Lookup(tc.key)
			acquire, calls := countingAcquire(true)
			outcome, run, evicted := c.Begin(tc.key, acquire)
			if outcome != BeginExisting || run != nil || evicted != nil {
				t.Fatalf("Begin = %v, %v, %v; want BeginExisting only", outcome, run, evicted)
			}
			if *calls != 0 {
				t.Errorf("acquire calls = %d, want 0", *calls)
			}
			after := c.Lookup(tc.key)
			if after.State != tc.state || after.Publish != tc.publish || after.PublishFailure != before.PublishFailure {
				t.Errorf("Lookup after Begin = %+v, want unchanged %+v", after, before)
			}
		})
	}
}

func TestBeginRestartsFailed(t *testing.T) {
	c, clock := newTestCache(t, 1)
	run := begin(t, c, testKey(1))
	run.StageStarted(writer.StageGeneration)
	run.StageFinished(writer.StageGeneration)
	run.StageSkipped(writer.StageShorten)
	run.StageStarted(writer.StageReview)
	run.Fail(FailureLLMTimeout)

	// The cache is full, but restarting needs no room.
	clock.Advance(time.Minute)
	acquire, calls := countingAcquire(true)
	outcome, restarted, evicted := c.Begin(testKey(1), acquire)
	if outcome != BeginRestarted || restarted == nil || evicted != nil {
		t.Fatalf("Begin = %v, %v, %v; want BeginRestarted, a run, no eviction", outcome, restarted, evicted)
	}
	if *calls != 1 {
		t.Errorf("acquire calls = %d, want 1", *calls)
	}
	got := c.Lookup(testKey(1))
	if got.State != JobRunning || got.Stages != (Stages{}) || got.Failure != FailureInternal {
		t.Errorf("Lookup = %+v, want running with stages and failure reset", got)
	}

	// The duration counts from the restart, not from the first start.
	clock.Advance(3 * time.Second)
	restarted.CompletePostable(testArticle("t"))
	if got := c.Lookup(testKey(1)).Duration; got != 3*time.Second {
		t.Errorf("Duration = %v, want 3s from the restart", got)
	}
}

func TestBeginFull(t *testing.T) {
	c, _ := newTestCache(t, 2)
	begin(t, c, testKey(1))
	posting(t, c, testKey(2))
	acquire, calls := countingAcquire(true)
	outcome, run, evicted := c.Begin(testKey(3), acquire)
	if outcome != BeginFull || run != nil || evicted != nil {
		t.Fatalf("Begin = %v, %v, %v; want BeginFull only", outcome, run, evicted)
	}
	if *calls != 0 {
		t.Errorf("acquire calls = %d, want 0", *calls)
	}
	if c.Lookup(testKey(1)).State != JobRunning || c.Lookup(testKey(2)).Publish != PublishPosting {
		t.Error("BeginFull changed a record")
	}
}

func TestEvictionOrderAndTargets(t *testing.T) {
	c, clock := newTestCache(t, 4)
	// Key 1 ends first but is posting; key 2 is the oldest evictable.
	posting(t, c, testKey(1))
	clock.Advance(time.Second)
	failed(t, c, testKey(2))
	clock.Advance(time.Second)
	done(t, c, testKey(3))
	begin(t, c, testKey(4))
	clock.Advance(5 * time.Second)

	outcome, _, evicted := c.Begin(testKey(5), always)
	if outcome != BeginStarted {
		t.Fatalf("Begin outcome = %v, want BeginStarted", outcome)
	}
	want := Eviction{Key: testKey(2), State: JobFailed, Age: 6 * time.Second}
	if evicted == nil || *evicted != want {
		t.Fatalf("evicted = %+v, want %+v", evicted, want)
	}
	if c.Lookup(testKey(2)).State != JobUnknown {
		t.Error("the evicted record is still held")
	}

	outcome, _, evicted = c.Begin(testKey(6), always)
	want = Eviction{Key: testKey(3), State: JobDone, Publish: PublishNotPosted, Age: 5 * time.Second}
	if outcome != BeginStarted || evicted == nil || *evicted != want {
		t.Fatalf("second Begin = %v, %+v; want BeginStarted evicting %+v", outcome, evicted, want)
	}

	// Left: posting key 1 and running keys 4, 5, 6.
	if outcome, _, _ = c.Begin(testKey(7), always); outcome != BeginFull {
		t.Errorf("third Begin = %v, want BeginFull: posting and running records stay", outcome)
	}
}

func TestEvictionTieBreaksByEndOrder(t *testing.T) {
	// Map iteration order varies, so repeat to catch an order that depends
	// on it.
	for range 20 {
		c, _ := newTestCache(t, 2)
		first := begin(t, c, testKey(1))
		second := begin(t, c, testKey(2))
		second.Fail(FailureLLM)
		first.CompletePostable(testArticle("t"))
		_, _, evicted := c.Begin(testKey(3), always)
		if evicted == nil || evicted.Key != testKey(2) {
			t.Fatalf("evicted = %+v, want key 2, which ended first at the same time", evicted)
		}
	}
}

func TestNoSlotDoesNotEvict(t *testing.T) {
	c, _ := newTestCache(t, 1)
	done(t, c, testKey(1))
	if outcome, _, evicted := c.Begin(testKey(2), never); outcome != BeginNoSlot || evicted != nil {
		t.Fatalf("Begin = %v, %+v; want BeginNoSlot and no eviction", outcome, evicted)
	}
	if c.Lookup(testKey(1)).State != JobDone {
		t.Error("BeginNoSlot dropped a record")
	}
}

func TestRunStages(t *testing.T) {
	c, _ := newTestCache(t, 1)
	run := begin(t, c, testKey(1))
	steps := []struct {
		apply func()
		want  Stages
	}{
		{func() { run.StageStarted(writer.StageGeneration) }, Stages{Draft: StageRunning}},
		{func() { run.StageFinished(writer.StageGeneration) }, Stages{Draft: StageDone}},
		{func() { run.StageSkipped(writer.StageShorten) }, Stages{Draft: StageDone, Shorten: StageSkipped}},
		{func() { run.StageStarted(writer.StageReview) }, Stages{Draft: StageDone, Shorten: StageSkipped, Review: StageRunning}},
		// Values outside the three stages are ignored.
		{func() { run.StageStarted(writer.StageUnset) }, Stages{Draft: StageDone, Shorten: StageSkipped, Review: StageRunning}},
		{func() { run.StageFinished(writer.Stage(-1)) }, Stages{Draft: StageDone, Shorten: StageSkipped, Review: StageRunning}},
		{func() { run.StageSkipped(writer.Stage(99)) }, Stages{Draft: StageDone, Shorten: StageSkipped, Review: StageRunning}},
		{func() { run.Fail(FailureLLMEmpty) }, Stages{Draft: StageDone, Shorten: StageSkipped, Review: StageFailed}},
	}
	for i, step := range steps {
		step.apply()
		if got := c.Lookup(testKey(1)).Stages; got != step.want {
			t.Fatalf("after step %d Stages = %+v, want %+v", i, got, step.want)
		}
	}
	if got := c.Lookup(testKey(1)); got.State != JobFailed || got.Failure != FailureLLMEmpty {
		t.Errorf("Lookup = %+v, want failed with FailureLLMEmpty", got)
	}
}

func TestRunEndsAfterFirstOutcome(t *testing.T) {
	cases := map[string]func(*Run){
		"complete postable":   func(r *Run) { r.CompletePostable(testArticle("first")) },
		"complete unpostable": func(r *Run) { r.CompleteUnpostable(testArticle("first"), UnpostableMention) },
		"fail":                func(r *Run) { r.Fail(FailureArticleTooLarge) },
	}
	for name, end := range cases {
		t.Run(name, func(t *testing.T) {
			c, clock := newTestCache(t, 1)
			run := begin(t, c, testKey(1))
			end(run)
			want := c.Lookup(testKey(1))

			clock.Advance(time.Hour)
			run.StageStarted(writer.StageGeneration)
			run.CompletePostable(testArticle("second"))
			run.CompleteUnpostable(testArticle("second"), UnpostableOther)
			run.Fail(FailureLLM)
			if got := c.Lookup(testKey(1)); !snapshotEqual(got, want) {
				t.Errorf("Lookup after later calls = %+v, want unchanged %+v", got, want)
			}
		})
	}
}

func snapshotEqual(a, b Snapshot) bool {
	return a.State == b.State && a.Stages == b.Stages && a.Failure == b.Failure &&
		a.Article.Title == b.Article.Title && a.Duration == b.Duration &&
		a.Publish == b.Publish && a.Unpostable == b.Unpostable && a.PublishFailure == b.PublishFailure
}

func TestStaleRunAfterRestart(t *testing.T) {
	c, _ := newTestCache(t, 1)
	old := begin(t, c, testKey(1))
	old.Fail(FailureLLM)
	current := begin(t, c, testKey(1))

	// A late call from the first job must not touch the restarted record.
	old.StageStarted(writer.StageGeneration)
	old.CompletePostable(testArticle("old"))
	if got := c.Lookup(testKey(1)); got.State != JobRunning || got.Stages != (Stages{}) {
		t.Fatalf("Lookup after stale calls = %+v, want running and untouched", got)
	}
	current.CompletePostable(testArticle("current"))
	if got := c.Lookup(testKey(1)).Article.Title; got != "current" {
		t.Errorf("Article.Title = %q, want current", got)
	}
}

func TestStaleRunAfterEviction(t *testing.T) {
	c, _ := newTestCache(t, 1)
	stale := begin(t, c, testKey(1))
	stale.Fail(FailureLLM)
	failed(t, c, testKey(2)) // evicts key 1
	if outcome, _, evicted := c.Begin(testKey(1), always); outcome != BeginStarted || evicted == nil {
		t.Fatalf("Begin = %v, %+v; want key 1 recreated by evicting key 2", outcome, evicted)
	}
	stale.CompletePostable(testArticle("stale"))
	if got := c.Lookup(testKey(1)).State; got != JobRunning {
		t.Errorf("Lookup state = %v, want running: the stale Run acted on a new record", got)
	}
}

func TestStalePublishRunAfterEviction(t *testing.T) {
	c, _ := newTestCache(t, 1)
	stale := posting(t, c, testKey(1))
	stale.Fail(PublishFailureWebhook)
	done(t, c, testKey(2))    // evicts key 1
	posting(t, c, testKey(1)) // evicts key 2 and recreates key 1, posting

	stale.Delivered(0)
	stale.Finish()
	if got := c.Lookup(testKey(1)).Publish; got != PublishPosting {
		t.Errorf("Publish = %v, want posting: the stale PublishRun acted on a new record", got)
	}
}

func TestCompleteStoresArticleWithoutRevisions(t *testing.T) {
	const marker = "MARKER-QUOTED-TRANSCRIPT"
	c, clock := newTestCache(t, 2)
	article := testArticle("t")
	article.Review = &writer.ReviewResult{
		Model:     writer.StepModel{Model: "review-model"},
		Revisions: []writer.Revision{{Before: "a", Action: writer.ActionDelete, Evidence: marker}},
	}
	run := begin(t, c, testKey(1))
	clock.Advance(2 * time.Second)
	run.CompletePostable(article)

	got := c.Lookup(testKey(1))
	if got.State != JobDone || got.Publish != PublishNotPosted || got.Duration != 2*time.Second {
		t.Fatalf("Lookup = %+v, want done, not posted, 2s", got)
	}
	if got.Article.Review == nil || got.Article.Review.Revisions != nil || got.Article.Review.Model.Model != "review-model" {
		t.Errorf("stored Review = %+v, want the model kept and no revisions", got.Article.Review)
	}
	if len(article.Review.Revisions) != 1 {
		t.Error("CompletePostable modified the caller's article")
	}
	dump := c.DumpRecordsForTest()
	if strings.Contains(dump, marker) {
		t.Error("a record holds the revisions' quoted transcript")
	}
	if !strings.Contains(dump, "review-model") {
		t.Errorf("DumpRecordsForTest does not render the stored article: %s", dump)
	}
}

func TestCompleteUnpostable(t *testing.T) {
	c, _ := newTestCache(t, 1)
	begin(t, c, testKey(1)).CompleteUnpostable(testArticle("t"), UnpostableMention)
	got := c.Lookup(testKey(1))
	if got.State != JobDone || got.Publish != PublishUnpostable || got.Unpostable != UnpostableMention {
		t.Errorf("Lookup = %+v, want done and unpostable for a mention", got)
	}
}

func TestBeginPublishOutcomes(t *testing.T) {
	c, _ := newTestCache(t, 10)
	begin(t, c, testKey(1))
	failed(t, c, testKey(2))
	begin(t, c, testKey(3)).CompleteUnpostable(testArticle("t"), UnpostableOther)
	done(t, c, testKey(4))
	posting(t, c, testKey(5))
	posting(t, c, testKey(6)).Finish()

	cases := map[string]struct {
		key     Key
		want    PublishBegin
		publish PublishState // after the call, for done records
	}{
		"unknown":    {testKey(9), PublishBeginNoArticle, PublishUnpostable},
		"running":    {testKey(1), PublishBeginNotReady, PublishUnpostable},
		"failed":     {testKey(2), PublishBeginNoArticle, PublishUnpostable},
		"unpostable": {testKey(3), PublishBeginUnpostable, PublishUnpostable},
		"not posted": {testKey(4), PublishBeginStarted, PublishPosting},
		"posting":    {testKey(5), PublishBeginAlready, PublishPosting},
		"posted":     {testKey(6), PublishBeginAlready, PublishPosted},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, pub := c.BeginPublish(tc.key)
			if got != tc.want {
				t.Fatalf("BeginPublish = %v, want %v", got, tc.want)
			}
			if (pub != nil) != (tc.want == PublishBeginStarted) {
				t.Errorf("BeginPublish handle = %v, want one only when started", pub)
			}
			if after := c.Lookup(tc.key).Publish; after != tc.publish {
				t.Errorf("Publish after = %v, want %v", after, tc.publish)
			}
		})
	}
}

func TestPublishDeliveredAndFrom(t *testing.T) {
	c, _ := newTestCache(t, 1)
	pub := posting(t, c, testKey(1))
	if pub.From() != 0 || pub.Article().Title != "t" {
		t.Fatalf("first post From = %d, Article.Title = %q; want 0 and t", pub.From(), pub.Article().Title)
	}
	pub.Delivered(0)
	pub.Delivered(1)
	// Out of order, repeated, and out-of-range indexes are ignored.
	for _, index := range []int{3, 1, -1, 1 << 30} {
		pub.Delivered(index)
	}
	pub.Fail(PublishFailureWebhook)
	if got := c.Lookup(testKey(1)); got.Publish != PublishFailed || got.PublishFailure != PublishFailureWebhook {
		t.Fatalf("Lookup = %+v, want publish failed by the Webhook", got)
	}

	// The retry starts at the first message not delivered.
	outcome, retry := c.BeginPublish(testKey(1))
	if outcome != PublishBeginStarted || retry.From() != 2 {
		t.Fatalf("retry = %v, From %d; want started from 2", outcome, retry.From())
	}
	// The ended handle no longer acts.
	pub.Delivered(2)
	pub.Finish()
	if got := c.Lookup(testKey(1)).Publish; got != PublishPosting {
		t.Fatalf("Publish after stale calls = %v, want posting", got)
	}

	retry.Delivered(2)
	retry.Fail(PublishFailureInternal)
	_, again := c.BeginPublish(testKey(1))
	if again.From() != 3 {
		t.Errorf("From after an internal failure = %d, want 3: delivered messages are kept", again.From())
	}
	again.Finish()
	if got := c.Lookup(testKey(1)).Publish; got != PublishPosted {
		t.Errorf("Publish = %v, want posted", got)
	}
	again.Fail(PublishFailureWebhook)
	if got := c.Lookup(testKey(1)).Publish; got != PublishPosted {
		t.Errorf("Publish after Fail on a finished handle = %v, want posted", got)
	}
}

func TestConcurrentBeginStartsOnce(t *testing.T) {
	cases := map[string]struct {
		prepare func(*testing.T, *Cache)
		want    BeginOutcome
	}{
		"new key":    {func(*testing.T, *Cache) {}, BeginStarted},
		"failed key": {func(t *testing.T, c *Cache) { failed(t, c, testKey(1)) }, BeginRestarted},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := newTestCache(t, 4)
			tc.prepare(t, c)
			var acquired, started, existing atomic.Int32
			acquire := func() bool {
				acquired.Add(1)
				return true
			}
			var wg sync.WaitGroup
			gate := make(chan struct{})
			for range 16 {
				wg.Go(func() {
					<-gate
					switch outcome, _, _ := c.Begin(testKey(1), acquire); outcome {
					case tc.want:
						started.Add(1)
					case BeginExisting:
						existing.Add(1)
					}
				})
			}
			close(gate)
			wg.Wait()
			if acquired.Load() != 1 || started.Load() != 1 || existing.Load() != 15 {
				t.Errorf("acquire calls %d, %v %d, existing %d; want 1, 1, 15",
					acquired.Load(), tc.want, started.Load(), existing.Load())
			}
		})
	}
}
