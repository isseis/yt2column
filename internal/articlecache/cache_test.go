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
	// A nil Clock selects time.Now, so the cache can time a generation.
	c, err := New(Options{Capacity: 1})
	if err != nil {
		t.Fatalf("New(Capacity 1) error = %v", err)
	}
	begin(t, c, testKey(1)).CompletePostable(testArticle("t"))
	if got := c.Lookup(testKey(1)).Duration; got < 0 || got > time.Minute {
		t.Errorf("Duration with the default clock = %v, want a short non-negative value", got)
	}
}

// TestBeginOutcomes covers each Begin outcome but BeginFull, which
// TestBeginFullChangesNothing covers, and whether it calls acquire.
func TestBeginOutcomes(t *testing.T) {
	t.Run("started", testBeginNewKey)
	t.Run("no slot", testBeginNoSlot)
	t.Run("existing", testBeginExistingDoesNotAcquire)
	t.Run("restarted", testBeginRestartsFailed)
}

func testBeginNewKey(t *testing.T) {
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

func testBeginNoSlot(t *testing.T) {
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

	// A full cache drops nothing when no slot is free.
	full, _ := newTestCache(t, 1)
	done(t, full, testKey(1))
	if outcome, _, evicted := full.Begin(testKey(2), never); outcome != BeginNoSlot || evicted != nil {
		t.Fatalf("Begin(full) = %v, %+v; want BeginNoSlot and no eviction", outcome, evicted)
	}
	if full.Lookup(testKey(1)).State != JobDone {
		t.Error("BeginNoSlot dropped a record")
	}
}

func testBeginExistingDoesNotAcquire(t *testing.T) {
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
			if before.State != tc.state || before.Publish != tc.publish {
				t.Fatalf("setup Lookup = %+v, want state %v and publish %v", before, tc.state, tc.publish)
			}
			if after := c.Lookup(tc.key); after != before {
				t.Errorf("Lookup after Begin = %+v, want unchanged %+v", after, before)
			}
		})
	}
}

func testBeginRestartsFailed(t *testing.T) {
	c, _ := newTestCache(t, 1)
	run := begin(t, c, testKey(1))
	run.StageStarted(writer.StageGeneration)
	run.StageFinished(writer.StageGeneration)
	run.StageSkipped(writer.StageShorten)
	run.StageStarted(writer.StageReview)
	run.Fail(FailureLLMTimeout)

	// The cache is full, but restarting needs no room.
	acquire, calls := countingAcquire(true)
	outcome, restarted, evicted := c.Begin(testKey(1), acquire)
	if outcome != BeginRestarted || restarted == nil || evicted != nil {
		t.Fatalf("Begin = %v, %v, %v; want BeginRestarted, a run, no eviction", outcome, restarted, evicted)
	}
	if *calls != 1 {
		t.Errorf("acquire calls = %d, want 1", *calls)
	}
	if got := c.Lookup(testKey(1)); got.State != JobRunning || got.Stages != (Stages{}) {
		t.Errorf("Lookup = %+v, want running with every stage reset", got)
	}
}

func TestDurationFromAcceptance(t *testing.T) {
	c, clock := newTestCache(t, 1)
	first := begin(t, c, testKey(1))
	clock.Advance(7 * time.Second)
	first.Fail(FailureLLM)
	clock.Advance(time.Minute)

	// The duration counts from the restart's acceptance, not the first one's.
	restarted := begin(t, c, testKey(1))
	clock.Advance(3 * time.Second)
	restarted.CompletePostable(testArticle("t"))
	if got := c.Lookup(testKey(1)).Duration; got != 3*time.Second {
		t.Errorf("Duration = %v, want 3s from the restart", got)
	}
}

func TestBeginFullChangesNothing(t *testing.T) {
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
	if c.Lookup(testKey(1)).State != JobRunning || c.Lookup(testKey(2)).Publish != PublishPosting ||
		c.Lookup(testKey(3)).State != JobUnknown {
		t.Error("BeginFull changed a record or added one")
	}
}

func TestEvictionOrder(t *testing.T) {
	t.Run("oldest end first", func(t *testing.T) {
		c, clock := newTestCache(t, 3)
		// Key 1 is accepted first but ends last; key 2 ends first.
		first := begin(t, c, testKey(1))
		clock.Advance(time.Second)
		second := begin(t, c, testKey(2))
		clock.Advance(time.Second)
		second.Fail(FailureLLM)
		clock.Advance(time.Second)
		first.CompletePostable(testArticle("t"))
		clock.Advance(5 * time.Second)
		begin(t, c, testKey(3))

		_, _, evicted := c.Begin(testKey(4), always)
		want := Eviction{Key: testKey(2), State: JobFailed, Age: 6 * time.Second}
		if evicted == nil || *evicted != want {
			t.Fatalf("evicted = %+v, want %+v", evicted, want)
		}
		if c.Lookup(testKey(2)).State != JobUnknown {
			t.Error("the evicted record is still held")
		}
		_, _, evicted = c.Begin(testKey(5), always)
		want = Eviction{Key: testKey(1), State: JobDone, Publish: PublishNotPosted, Age: 5 * time.Second}
		if evicted == nil || *evicted != want {
			t.Fatalf("second evicted = %+v, want %+v", evicted, want)
		}
	})

	t.Run("end order breaks a tie", func(t *testing.T) {
		// Map iteration order varies, so repeat to catch an order that
		// depends on it.
		// Both end orders are checked, so a tie-break other than end order
		// (such as one by key) fails one of them.
		for range 20 {
			for _, firstEnded := range []int{1, 2} {
				c, _ := newTestCache(t, 2)
				runs := map[int]*Run{1: begin(t, c, testKey(1)), 2: begin(t, c, testKey(2))}
				runs[firstEnded].Fail(FailureLLM)
				runs[3-firstEnded].CompletePostable(testArticle("t"))
				_, _, evicted := c.Begin(testKey(3), always)
				if evicted == nil || evicted.Key != testKey(firstEnded) {
					t.Fatalf("evicted = %+v, want key %d, which ended first at the same time", evicted, firstEnded)
				}
			}
		}
	})
}

func TestEvictionKeepsRunningAndPosting(t *testing.T) {
	cases := map[string]struct {
		prepare func(*testing.T, *Cache)
		evicted bool
	}{
		"running":    {func(t *testing.T, c *Cache) { begin(t, c, testKey(1)) }, false},
		"posting":    {func(t *testing.T, c *Cache) { posting(t, c, testKey(1)) }, false},
		"failed":     {func(t *testing.T, c *Cache) { failed(t, c, testKey(1)) }, true},
		"not posted": {func(t *testing.T, c *Cache) { done(t, c, testKey(1)) }, true},
		"posted":     {func(t *testing.T, c *Cache) { posting(t, c, testKey(1)).Finish() }, true},
		"publish failed": {func(t *testing.T, c *Cache) {
			posting(t, c, testKey(1)).Fail(PublishFailureWebhook)
		}, true},
		"unpostable": {func(t *testing.T, c *Cache) {
			begin(t, c, testKey(1)).CompleteUnpostable(testArticle("t"), UnpostableOther)
		}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := newTestCache(t, 1)
			tc.prepare(t, c)
			outcome, _, evicted := c.Begin(testKey(2), always)
			if tc.evicted {
				if outcome != BeginStarted || evicted == nil || evicted.Key != testKey(1) {
					t.Errorf("Begin = %v, %+v; want key 1 evicted", outcome, evicted)
				}
			} else if outcome != BeginFull || evicted != nil {
				t.Errorf("Begin = %v, %+v; want BeginFull and the record kept", outcome, evicted)
			}
		})
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
			if got := c.Lookup(testKey(1)); got != want {
				t.Errorf("Lookup after later calls = %+v, want unchanged %+v", got, want)
			}
		})
	}
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

	// While key 1 is absent, every method does nothing and does not panic.
	stale.StageStarted(writer.StageGeneration)
	stale.CompletePostable(testArticle("stale"))
	stale.CompleteUnpostable(testArticle("stale"), UnpostableOther)
	stale.Fail(FailureLLM)
	if got := c.Lookup(testKey(1)).State; got != JobUnknown {
		t.Fatalf("Lookup state = %v, want JobUnknown: the stale Run recreated a record", got)
	}

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
	done(t, c, testKey(2)) // evicts key 1

	// While key 1 is absent, every method does nothing and does not panic.
	stale.Delivered(0)
	stale.Finish()
	stale.Fail(PublishFailureInternal)
	if got := c.Lookup(testKey(1)).State; got != JobUnknown {
		t.Fatalf("Lookup state = %v, want JobUnknown", got)
	}

	current := posting(t, c, testKey(1)) // evicts key 2 and recreates key 1, posting
	stale.Delivered(0)
	current.Fail(PublishFailureWebhook)
	if _, retry := c.BeginPublish(testKey(1)); retry.From() != 0 {
		t.Errorf("From = %d, want 0: the stale PublishRun's Delivered reached a new record", retry.From())
	}
	stale.Finish()
	if got := c.Lookup(testKey(1)).Publish; got != PublishPosting {
		t.Errorf("Publish = %v, want posting: the stale PublishRun acted on a new record", got)
	}
}

func TestCompleteDropsRevisions(t *testing.T) {
	const marker = "MARKER-QUOTED-TRANSCRIPT"
	c, _ := newTestCache(t, 2)
	article := testArticle("t")
	article.Review = &writer.ReviewResult{
		Model:     writer.StepModel{Model: "review-model"},
		Revisions: []writer.Revision{{Before: "a", Action: writer.ActionDelete, Evidence: marker}},
	}
	article.Shorten = &writer.StepModel{Model: "shorten-model"}
	begin(t, c, testKey(1)).CompletePostable(article)
	article.Shorten.Model = "changed"

	got := c.Lookup(testKey(1))
	if got.Article.Review == nil || got.Article.Review.Revisions != nil || got.Article.Review.Model.Model != "review-model" {
		t.Errorf("stored Review = %+v, want the model kept and no revisions", got.Article.Review)
	}
	if got.Article.Shorten == nil || got.Article.Shorten.Model != "shorten-model" {
		t.Errorf("stored Shorten = %+v, want a copy the caller cannot change", got.Article.Shorten)
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

func TestPublishResumeFrom(t *testing.T) {
	c, _ := newTestCache(t, 1)
	pub := posting(t, c, testKey(1))
	if pub.From() != 0 || pub.Article().Title != "t" {
		t.Fatalf("first post From = %d, Article.Title = %q; want 0 and t", pub.From(), pub.Article().Title)
	}
	pub.Delivered(0)
	pub.Delivered(1)
	// A gap, a repeat, and a negative or far index are ignored: only the
	// next undelivered index counts.
	for _, index := range []int{3, 1, -1, 1 << 30} {
		pub.Delivered(index)
	}
	pub.Fail(PublishFailureWebhook)
	if got := c.Lookup(testKey(1)); got.Publish != PublishFailed || got.PublishFailure != PublishFailureWebhook {
		t.Fatalf("Lookup = %+v, want publish failed by the Webhook", got)
	}
	// A handle that has failed no longer records deliveries.
	pub.Delivered(2)

	// The retry starts at the first message not delivered.
	outcome, retry := c.BeginPublish(testKey(1))
	if outcome != PublishBeginStarted || retry.From() != 2 {
		t.Fatalf("retry = %v, From %d; want started from 2", outcome, retry.From())
	}
	// The ended handle no longer acts.
	pub.Delivered(2)
	retry.Fail(PublishFailureWebhook)
	outcome, retry = c.BeginPublish(testKey(1))
	if outcome != PublishBeginStarted || retry.From() != 2 {
		t.Fatalf("retry after a stale Delivered = %v, From %d; want started from 2", outcome, retry.From())
	}
	pub.Finish()
	if got := c.Lookup(testKey(1)).Publish; got != PublishPosting {
		t.Fatalf("Publish after a stale Finish = %v, want posting", got)
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

func TestBeginConcurrentAcquiresOnce(t *testing.T) {
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

func TestTransitionsReturnTheirValues(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		c, clock := newTestCache(t, 1)
		run := begin(t, c, testKey(1))
		clock.Advance(5 * time.Minute)
		if d, ok := run.CompletePostable(testArticle("t")); d != 5*time.Minute || !ok {
			t.Errorf("CompletePostable = %v, %v; want 5m, true", d, ok)
		}
		if d, ok := run.CompleteUnpostable(testArticle("t"), UnpostableOther); d != 0 || ok {
			t.Errorf("CompleteUnpostable on an ended Run = %v, %v; want 0, false", d, ok)
		}
		if st, ok := run.Fail(FailureLLM); st != (Stages{}) || ok {
			t.Errorf("Fail on an ended Run = %+v, %v; want zero, false", st, ok)
		}

		retry := begin(t, c, testKey(2)) // evicts key 1
		clock.Advance(time.Minute)
		if d, ok := retry.CompleteUnpostable(testArticle("t"), UnpostableMention); d != time.Minute || !ok {
			t.Errorf("CompleteUnpostable = %v, %v; want 1m, true", d, ok)
		}

		failing := begin(t, c, testKey(3)) // evicts key 2
		failing.StageFinished(writer.StageGeneration)
		failing.StageStarted(writer.StageReview)
		want := Stages{Draft: StageDone, Review: StageFailed}
		if st, ok := failing.Fail(FailureLLM); st != want || !ok {
			t.Errorf("Fail = %+v, %v; want %+v, true", st, ok, want)
		}
		// The restart resets the stages the Fail above returned.
		begin(t, c, testKey(3))
		if got := c.Lookup(testKey(3)).Stages; got != (Stages{}) {
			t.Errorf("Stages after the restart = %+v, want zero", got)
		}
		if st, ok := failing.Fail(FailureLLM); st != (Stages{}) || ok {
			t.Errorf("Fail on a replaced Run = %+v, %v; want zero, false", st, ok)
		}
	})
	t.Run("publish_run", func(t *testing.T) {
		c, _ := newTestCache(t, 1)
		pub := posting(t, c, testKey(1))
		pub.Delivered(0)
		pub.Delivered(1)
		if n, ok := pub.Fail(PublishFailureWebhook); n != 2 || !ok {
			t.Errorf("Fail = %d, %v; want 2, true", n, ok)
		}
		if n, ok := pub.Finish(); n != 0 || ok {
			t.Errorf("Finish on an ended PublishRun = %d, %v; want 0, false", n, ok)
		}
		_, retry := c.BeginPublish(testKey(1))
		retry.Delivered(2)
		if n, ok := retry.Finish(); n != 3 || !ok {
			t.Errorf("Finish = %d, %v; want 3, true", n, ok)
		}
		if n, ok := retry.Fail(PublishFailureInternal); n != 0 || ok {
			t.Errorf("Fail on an ended PublishRun = %d, %v; want 0, false", n, ok)
		}
	})
}
