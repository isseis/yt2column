package articlecache

import (
	"time"

	"github.com/isseis/yt2column/internal/writer"
)

// JobState is a record's generation state. The zero value is JobUnknown: the
// state of a key the cache does not hold.
type JobState int

// The JobState values.
const (
	JobUnknown JobState = iota // the cache holds no record for the key
	JobRunning                 // a generation job is running
	JobDone                    // the article is stored
	JobFailed                  // the generation failed
	// jobStateEnd is not a state: it follows the last one, so an enumeration
	// of the values includes one appended later.
	jobStateEnd
)

// StageState is the state of one step (a writer.Stage) of a generation. The
// zero value is StagePending.
type StageState int

// The StageState values.
const (
	StagePending  StageState = iota // not started
	StageRunning                    // started and not finished
	StageDone                       // finished
	StageSkipped                    // not needed for this article
	StageFailed                     // running when the generation failed
	stageStateEnd                   // not a state: follows the last one
)

// Stages holds the state of each step.
type Stages struct {
	Draft   StageState // writer.StageGeneration
	Shorten StageState // writer.StageShorten
	Review  StageState // writer.StageReview
}

// FailureReason is why a generation failed. The zero value is
// FailureInternal.
type FailureReason int

// The FailureReason values.
const (
	FailureInternal          FailureReason = iota // unclassified, a panic included
	FailureTranscriptTooLong                      // the prompt built from the transcript is too large
	FailureLLMTimeout                             // an LLM call timed out
	FailureLLMEmpty                               // an LLM returned an empty response
	FailureLLMTruncated                           // an LLM response was cut off
	FailureLLM                                    // any other LLM failure
	FailureArticleInvalid                         // the article fails writer.Article.CheckPublishable
	FailureArticleTooLarge                        // the status response would exceed its limit
	failureReasonEnd                              // not a reason: follows the last one
)

// PublishState is a completed article's publish state. The zero value is
// PublishUnpostable, which neither sends nor claims a delivery.
type PublishState int

// The PublishState values.
const (
	PublishUnpostable PublishState = iota // the pre-send checks reject the article
	PublishNotPosted                      // postable and never posted
	PublishPosting                        // a post job is sending
	PublishPosted                         // every message was delivered
	PublishFailed                         // the last post stopped before the end
	publishStateEnd                       // not a state: follows the last one
)

// UnpostableReason is why the pre-send checks reject an article. The zero
// value is UnpostableOther.
type UnpostableReason int

// The UnpostableReason values.
const (
	UnpostableOther     UnpostableReason = iota // any rejection but a mention
	UnpostableMention                           // the article contains a mention
	unpostableReasonEnd                         // not a reason: follows the last one
)

// PublishFailureReason is why a post failed after it started. The zero value
// is PublishFailureInternal.
type PublishFailureReason int

// The PublishFailureReason values.
const (
	PublishFailureInternal  PublishFailureReason = iota // unclassified, a panic included
	PublishFailureWebhook                               // the Webhook did not acknowledge a message
	publishFailureReasonEnd                             // not a reason: follows the last one
)

// StageStates returns every StageState value, in declaration order.
func StageStates() []StageState { return enumerate(stageStateEnd) }

// PublishStates returns every PublishState value, in declaration order.
func PublishStates() []PublishState { return enumerate(publishStateEnd) }

// UnpostableReasons returns every UnpostableReason value, in declaration
// order.
func UnpostableReasons() []UnpostableReason { return enumerate(unpostableReasonEnd) }

// PublishFailureReasons returns every PublishFailureReason value, in
// declaration order.
func PublishFailureReasons() []PublishFailureReason { return enumerate(publishFailureReasonEnd) }

// enumerate returns the values from the zero value up to, not including, end.
func enumerate[E ~int](end E) []E {
	values := make([]E, 0, int(end))
	for v := range end {
		values = append(values, v)
	}
	return values
}

// Snapshot is a copy of one record, taken under the cache's lock. Article is
// never modified after CompletePostable or CompleteUnpostable stores it, so
// the pointers it shares with the record are safe to read without the lock.
type Snapshot struct {
	State          JobState
	Stages         Stages
	Failure        FailureReason        // JobFailed only
	Article        writer.Article       // JobDone only; Review.Revisions is always nil
	Duration       time.Duration        // JobDone only
	Publish        PublishState         // JobDone only
	Unpostable     UnpostableReason     // PublishUnpostable only
	PublishFailure PublishFailureReason // PublishFailed only
}

// record is the cache's value for one key. Every field is guarded by the
// cache's lock.
type record struct {
	// gen changes each time the record is created or restarted; a Run acts
	// only while it matches.
	gen uint64
	// publishGen changes each time BeginPublish starts a post; a PublishRun
	// acts only while it matches and the record is posting.
	publishGen uint64

	state   JobState
	stages  Stages
	failure FailureReason
	article writer.Article

	accepted time.Time
	// ended is when the record became done or failed. The eviction order is
	// ended, then endSeq for records that ended at the same time.
	ended  time.Time
	endSeq uint64

	publish        PublishState
	unpostable     UnpostableReason
	publishFailure PublishFailureReason
	// delivered counts the messages that reached the Webhook in a row from
	// the first one. A retry starts from this index.
	delivered int
}

// snapshot copies the fields the record's state gives meaning to.
func (r *record) snapshot() Snapshot {
	s := Snapshot{State: r.state, Stages: r.stages}
	switch r.state {
	case JobFailed:
		s.Failure = r.failure
	case JobDone:
		s.Article = r.article
		s.Duration = r.duration()
		s.Publish = r.publish
		switch r.publish {
		case PublishUnpostable:
			s.Unpostable = r.unpostable
		case PublishFailed:
			s.PublishFailure = r.publishFailure
		}
	}
	return s
}

// duration is how long a done record took, from acceptance to its end; the
// status response and the success log both report it.
func (r *record) duration() time.Duration {
	return r.ended.Sub(r.accepted)
}

// evictable reports whether the limit may drop the record: a running record
// and a done record being posted are never dropped.
func (r *record) evictable() bool {
	switch r.state {
	case JobFailed:
		return true
	case JobDone:
		return r.publish != PublishPosting
	default:
		return false
	}
}

// stage returns the field for s, or nil for a value outside the three writer
// stages.
func (st *Stages) stage(s writer.Stage) *StageState {
	switch s {
	case writer.StageGeneration:
		return &st.Draft
	case writer.StageShorten:
		return &st.Shorten
	case writer.StageReview:
		return &st.Review
	default:
		return nil
	}
}

// withoutRevisions returns a copy of a whose Review, if any, is a new value
// with no revisions, so the record never holds the quoted transcript the
// revisions carry and a's own ReviewResult is left as it was. Shorten is
// copied too, so the stored article shares no pointer with the caller.
func withoutRevisions(a writer.Article) writer.Article {
	if a.Shorten != nil {
		shorten := *a.Shorten
		a.Shorten = &shorten
	}
	if a.Review != nil {
		review := *a.Review
		review.Revisions = nil
		a.Review = &review
	}
	return a
}

// Run is the handle a generation job reports through; its stage methods are
// the ones writer's progress observer is to call. A Run acts only on the
// record generation it was issued for: after CompletePostable,
// CompleteUnpostable, or Fail, or once the record was replaced, every method
// does nothing. A stage outside the three writer stages is ignored.
type Run struct {
	cache *Cache
	key   Key
	gen   uint64
}

// running returns the record while the Run is valid, or nil. The caller holds
// the lock.
func (r *Run) running() *record {
	rec, ok := r.cache.records[r.key]
	if !ok || rec.gen != r.gen || rec.state != JobRunning {
		return nil
	}
	return rec
}

// setStage moves stage s of a valid Run's record to to.
func (r *Run) setStage(s writer.Stage, to StageState) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	rec := r.running()
	if rec == nil {
		return
	}
	if field := rec.stages.stage(s); field != nil {
		*field = to
	}
}

// StageStarted marks stage s running.
func (r *Run) StageStarted(s writer.Stage) { r.setStage(s, StageRunning) }

// StageFinished marks stage s done.
func (r *Run) StageFinished(s writer.Stage) { r.setStage(s, StageDone) }

// StageSkipped marks stage s skipped.
func (r *Run) StageSkipped(s writer.Stage) { r.setStage(s, StageSkipped) }

// CompletePostable stores a, with Review.Revisions dropped, as done and not
// posted. It returns the generation's duration as it stored it, and false
// when the Run is no longer valid and nothing changed.
func (r *Run) CompletePostable(a writer.Article) (time.Duration, bool) {
	return r.complete(a, PublishNotPosted, UnpostableOther)
}

// CompleteUnpostable stores a, with Review.Revisions dropped, as done and
// unpostable. It returns what CompletePostable returns.
func (r *Run) CompleteUnpostable(a writer.Article, reason UnpostableReason) (time.Duration, bool) {
	return r.complete(a, PublishUnpostable, reason)
}

// complete stores a as done and returns the duration it stored, read under
// the same lock as the transition: once the lock is released a retry or an
// eviction may replace the record, so the caller never reads it back.
func (r *Run) complete(a writer.Article, publish PublishState, reason UnpostableReason) (time.Duration, bool) {
	stored := withoutRevisions(a)
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	rec := r.running()
	if rec == nil {
		return 0, false
	}
	rec.state = JobDone
	rec.article = stored
	rec.publish = publish
	rec.unpostable = reason
	rec.delivered = 0
	r.cache.end(rec)
	return rec.duration(), true
}

// Fail marks the record failed; the running stage, if any, becomes
// StageFailed. It returns the stages as it left them, the failed one marked
// StageFailed, read under the same lock as the transition, and false when the
// Run is no longer valid and nothing changed.
func (r *Run) Fail(reason FailureReason) (Stages, bool) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	rec := r.running()
	if rec == nil {
		return Stages{}, false
	}
	for _, field := range []*StageState{&rec.stages.Draft, &rec.stages.Shorten, &rec.stages.Review} {
		if *field == StageRunning {
			*field = StageFailed
		}
	}
	rec.state = JobFailed
	rec.failure = reason
	r.cache.end(rec)
	return rec.stages, true
}

// PublishRun is the handle a post job reports through, with the same
// generation rule as Run: after Finish or Fail, or once the record was
// replaced, every method but Article and From does nothing. Delivered ignores
// every index but the first message not yet delivered; the cache does not know
// the message count, so the caller reports only indexes of the article's
// messages.
type PublishRun struct {
	cache      *Cache
	key        Key
	gen        uint64
	publishGen uint64
	article    writer.Article
	from       int
}

// posting returns the record while the PublishRun is valid, or nil. The
// caller holds the lock.
func (p *PublishRun) posting() *record {
	rec, ok := p.cache.records[p.key]
	if !ok || rec.gen != p.gen || rec.publishGen != p.publishGen ||
		rec.state != JobDone || rec.publish != PublishPosting {
		return nil
	}
	return rec
}

// Article returns the article to post.
func (p *PublishRun) Article() writer.Article { return p.article }

// From returns the index of the first message not delivered when the post
// began.
func (p *PublishRun) From() int { return p.from }

// Delivered records that message index reached the Webhook.
func (p *PublishRun) Delivered(index int) {
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()
	rec := p.posting()
	if rec == nil || index != rec.delivered {
		return
	}
	rec.delivered++
}

// Finish marks the article posted; the caller calls it once every message
// was delivered. It returns the number of messages recorded as delivered, read
// under the same lock as the transition, and false when the PublishRun is no
// longer valid and nothing changed.
func (p *PublishRun) Finish() (int, bool) {
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()
	rec := p.posting()
	if rec == nil {
		return 0, false
	}
	rec.publish = PublishPosted
	return rec.delivered, true
}

// Fail marks the post failed. The delivered messages stay recorded, so a retry
// starts from the first one not delivered. It returns what Finish returns.
func (p *PublishRun) Fail(reason PublishFailureReason) (int, bool) {
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()
	rec := p.posting()
	if rec == nil {
		return 0, false
	}
	rec.publish = PublishFailed
	rec.publishFailure = reason
	return rec.delivered, true
}
