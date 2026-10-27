package articlecache

import (
	"errors"
	"sync"
	"time"
)

// DefaultCapacity is the record limit the server uses. It counts running,
// done, and failed records alike; only done and failed records not being
// posted can be dropped to make room. It is exported because the server, in
// another package, passes it to New.
const DefaultCapacity = 100

// Options configures a Cache. Capacity must be positive; Clock nil selects
// time.Now.
type Options struct {
	Capacity int
	Clock    func() time.Time
}

// ErrInvalidOptions is wrapped by New for a non-positive Capacity.
var ErrInvalidOptions = errors.New("invalid article cache options")

// Cache holds one record per key. One mutex guards the records and the
// counters, and every method, the handles' included, releases it with defer
// and never blocks or panics while holding it, so one bad call cannot stop
// every other request.
type Cache struct {
	mu       sync.Mutex
	capacity int
	clock    func() time.Time
	records  map[Key]*record
	// nextGen numbers record generations and posts; nextSeq orders the
	// records that ended at the same time.
	nextGen uint64
	nextSeq uint64
}

// New returns an empty Cache.
func New(opts Options) (*Cache, error) {
	if opts.Capacity <= 0 {
		return nil, ErrInvalidOptions
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Cache{
		capacity: opts.Capacity,
		clock:    clock,
		records:  make(map[Key]*record, opts.Capacity),
	}, nil
}

// BeginOutcome is what Begin did. The zero value is BeginFull, which starts
// nothing.
type BeginOutcome int

// The BeginOutcome values.
const (
	BeginFull      BeginOutcome = iota // no evictable record: nothing changed
	BeginNoSlot                        // acquire reported false: nothing changed
	BeginExisting                      // the key is running or done: nothing changed
	BeginStarted                       // a running record was created
	BeginRestarted                     // a failed record was restarted
)

// Eviction describes a dropped record, for the log.
type Eviction struct {
	Key     Key
	State   JobState     // JobDone or JobFailed
	Publish PublishState // JobDone only
	Age     time.Duration
}

// Begin decides, under the lock, what a generation start does for key. It
// calls acquire only when a generation would start, and only after it has
// confirmed that a record can be added; Run is non-nil only for BeginStarted
// and BeginRestarted. Evicted reports the key of the record it dropped, if any.
// acquire must not block: it runs under the lock.
func (c *Cache) Begin(key Key, acquire func() bool) (outcome BeginOutcome, run *Run, evicted *Eviction) {
	c.mu.Lock()
	defer c.mu.Unlock()

	rec, exists := c.records[key]
	if exists && rec.state != JobFailed {
		return BeginExisting, nil, nil
	}
	var victim Key
	var victimRec *record
	if !exists && len(c.records) >= c.capacity {
		victim, victimRec = c.oldestEvictable()
		if victimRec == nil {
			return BeginFull, nil, nil
		}
	}
	if !acquire() {
		return BeginNoSlot, nil, nil
	}

	now := c.clock()
	if victimRec != nil {
		delete(c.records, victim)
		evicted = &Eviction{Key: victim, State: victimRec.state, Age: now.Sub(victimRec.ended)}
		if victimRec.state == JobDone {
			evicted.Publish = victimRec.publish
		}
	}
	c.nextGen++
	if exists {
		// Restarting a failed record forgets the failure and every stage.
		*rec = record{gen: c.nextGen, state: JobRunning, accepted: now}
		outcome = BeginRestarted
	} else {
		c.records[key] = &record{gen: c.nextGen, state: JobRunning, accepted: now}
		outcome = BeginStarted
	}
	return outcome, &Run{cache: c, key: key, gen: c.nextGen}, evicted
}

// oldestEvictable returns the evictable record that ended first, or a nil
// record when there is none. The caller holds the lock.
func (c *Cache) oldestEvictable() (Key, *record) {
	var oldestKey Key
	var oldest *record
	for key, rec := range c.records {
		if !rec.evictable() {
			continue
		}
		if oldest == nil || rec.ended.Before(oldest.ended) ||
			(rec.ended.Equal(oldest.ended) && rec.endSeq < oldest.endSeq) {
			oldestKey, oldest = key, rec
		}
	}
	return oldestKey, oldest
}

// end stamps rec as having become done or failed now. The caller holds the
// lock.
func (c *Cache) end(rec *record) {
	c.nextSeq++
	rec.ended = c.clock()
	rec.endSeq = c.nextSeq
}

// Lookup returns a copy of key's record, or a Snapshot with JobUnknown.
func (c *Cache) Lookup(key Key) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	rec, ok := c.records[key]
	if !ok {
		return Snapshot{}
	}
	return rec.snapshot()
}

// PublishBegin is what BeginPublish did. The zero value is
// PublishBeginNoArticle.
type PublishBegin int

// The PublishBegin values.
const (
	PublishBeginNoArticle  PublishBegin = iota // failed or unknown
	PublishBeginNotReady                       // running
	PublishBeginUnpostable                     // done and unpostable
	PublishBeginAlready                        // done and posting or posted: nothing changed
	PublishBeginStarted                        // the record is now posting
)

// BeginPublish moves a done, not-posted or publish-failed record to posting
// and returns a handle that carries the article and the index of the first
// message not delivered. The handle is non-nil only for PublishBeginStarted.
func (c *Cache) BeginPublish(key Key) (PublishBegin, *PublishRun) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rec, ok := c.records[key]
	if !ok {
		return PublishBeginNoArticle, nil
	}
	switch rec.state {
	case JobRunning:
		return PublishBeginNotReady, nil
	case JobDone:
	default:
		return PublishBeginNoArticle, nil
	}
	switch rec.publish {
	case PublishNotPosted, PublishFailed:
	case PublishPosting, PublishPosted:
		return PublishBeginAlready, nil
	default:
		return PublishBeginUnpostable, nil
	}
	c.nextGen++
	rec.publish = PublishPosting
	rec.publishGen = c.nextGen
	return PublishBeginStarted, &PublishRun{
		cache:      c,
		key:        key,
		gen:        rec.gen,
		publishGen: rec.publishGen,
		article:    rec.article,
		from:       rec.delivered,
	}
}
