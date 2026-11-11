package articlecache

import (
	"errors"
	"sync"
	"time"
)

// DefaultCapacity is the record limit the server uses. It counts running,
// done, and failed records alike; only done and failed records can be dropped
// to make room. It is exported because the server, in
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
	// nextGen numbers record generations; nextSeq orders the
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
	BeginSameVideo                     // another article of the same video was found: nothing changed
)

// StartMode is whether a generation start asks for another article of the
// same video. The zero value is StartInquiry: a start that does not say
// otherwise starts nothing when such an article exists, so it never spends an
// LLM call the caller did not ask for.
type StartMode int

// The StartMode values.
const (
	StartInquiry StartMode = iota // report another article of the same video instead of starting
	StartPlain                    // start as before; never report another article of the same video
)

// BeginResult is what Begin did. Run is non-nil only for BeginStarted and
// BeginRestarted, Evicted only when one of them dropped a record, and
// SameVideoKey is set only for BeginSameVideo.
type BeginResult struct {
	Outcome      BeginOutcome
	Run          *Run
	Evicted      *Eviction
	SameVideoKey Key
}

// Eviction describes a dropped record, for the log.
type Eviction struct {
	Key   Key
	State JobState // JobDone or JobFailed
	Age   time.Duration
}

// Begin decides, under the lock, what a generation start does for subject,
// in this order:
//
//  1. subject's key is running or done: BeginExisting.
//  2. mode is StartInquiry and another article of the same video exists:
//     BeginSameVideo with the latest such article's key. Nothing changes:
//     acquire is not called, no record is dropped, and subject's failed
//     record, if any, is not restarted.
//  3. otherwise BeginFull, BeginNoSlot, BeginStarted, or BeginRestarted, as a
//     start that does not look for the same video.
//
// Looking for the same video and starting happen under one lock, so of two
// starts of one video that arrive together only one can start. Begin calls
// acquire only when a generation would start, and only after it has
// confirmed that a record can be added. acquire must not block: it runs under
// the lock.
func (c *Cache) Begin(subject Subject, mode StartMode, acquire func() bool) BeginResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := subject.key
	rec, exists := c.records[key]
	if exists && rec.state != JobFailed {
		return BeginResult{Outcome: BeginExisting}
	}
	switch mode {
	case StartPlain:
	default:
		// StartInquiry, and any value not declared, which is treated as the
		// mode that starts nothing.
		if sameKey, ok := c.latestSameVideo(subject); ok {
			return BeginResult{Outcome: BeginSameVideo, SameVideoKey: sameKey}
		}
	}
	var victim Key
	var victimRec *record
	if !exists && len(c.records) >= c.capacity {
		victim, victimRec = c.oldestEvictable()
		if victimRec == nil {
			return BeginResult{Outcome: BeginFull}
		}
	}
	if !acquire() {
		return BeginResult{Outcome: BeginNoSlot}
	}

	result := BeginResult{Outcome: BeginStarted}
	now := c.clock()
	if victimRec != nil {
		delete(c.records, victim)
		result.Evicted = &Eviction{Key: victim, State: victimRec.state, Age: now.Sub(victimRec.ended)}
	}
	c.nextGen++
	fresh := record{gen: c.nextGen, videoID: subject.videoID, videoTitle: subject.videoTitle, state: JobRunning, accepted: now}
	if exists {
		// Restarting a failed record forgets the failure and every stage.
		*rec = fresh
		result.Outcome = BeginRestarted
	} else {
		c.records[key] = &fresh
	}
	result.Run = &Run{cache: c, key: key, gen: c.nextGen}
	return result
}

// latestSameVideo returns the key of the latest article of subject's video
// other than subject's own: a running or done record with the same video ID,
// the one with the highest gen. Begin calls it only when subject's own record
// is absent or failed, so the state check alone leaves that record out. A
// failed record is not an article, and an empty video ID matches nothing, so
// a Subject built by mistake never reports an unrelated article. gen grows
// only under the lock and is set only when a record is created or restarted,
// so it orders the records by acceptance and no two records share one. The
// caller holds the lock.
func (c *Cache) latestSameVideo(subject Subject) (Key, bool) {
	if subject.videoID == "" {
		return Key{}, false
	}
	var latestKey Key
	var latest *record
	for key, rec := range c.records {
		if rec.videoID != subject.videoID {
			continue
		}
		switch rec.state {
		case JobRunning, JobDone:
		default:
			continue
		}
		if latest == nil || rec.gen > latest.gen {
			latestKey, latest = key, rec
		}
	}
	return latestKey, latest != nil
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
