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

// StageStates returns every StageState value, in declaration order.
func StageStates() []StageState { return enumerate(stageStateEnd) }

// enumerate returns the values from the zero value up to, not including, end.
func enumerate[E ~int](end E) []E {
	values := make([]E, 0, int(end))
	for v := range end {
		values = append(values, v)
	}
	return values
}

// Snapshot is a copy of one record, taken under the cache's lock. Article is
// never modified after Complete stores it, so the pointers it shares with the
// record are safe to read without the lock.
type Snapshot struct {
	State      JobState
	Stages     Stages
	Failure    FailureReason  // JobFailed only
	Article    writer.Article // JobDone only; Review.Revisions is always nil
	VideoTitle string         // JobDone only; "" when the start sent none
	Duration   time.Duration  // JobDone only
}

// record is the cache's value for one key. Every field is guarded by the
// cache's lock.
type record struct {
	// gen changes each time the record is created or restarted; a Run acts
	// only while it matches.
	gen uint64
	// videoID is the video the record's input names, from the Subject that
	// created or restarted it. Begin compares it only for equality, and no
	// Snapshot carries it.
	videoID string
	// videoTitle is the video title the generation start sent ("" for none),
	// from the same Subject. The Viewer's heading shows it once the article
	// is done.
	videoTitle string

	state   JobState
	stages  Stages
	failure FailureReason
	article writer.Article

	accepted time.Time
	// ended is when the record became done or failed. The eviction order is
	// ended, then endSeq for records that ended at the same time.
	ended  time.Time
	endSeq uint64
}

// snapshot copies the fields the record's state gives meaning to.
func (r *record) snapshot() Snapshot {
	s := Snapshot{State: r.state, Stages: r.stages}
	switch r.state {
	case JobFailed:
		s.Failure = r.failure
	case JobDone:
		s.Article = r.article
		s.VideoTitle = r.videoTitle
		s.Duration = r.duration()
	}
	return s
}

// duration is how long a done record took, from acceptance to its end; the
// status response and the success log both report it.
func (r *record) duration() time.Duration {
	return r.ended.Sub(r.accepted)
}

// evictable reports whether the limit may drop the record: a done or failed
// record may be dropped, a running one never.
func (r *record) evictable() bool {
	switch r.state {
	case JobDone, JobFailed:
		return true
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
// record generation it was issued for: after Complete or Fail, or once the
// record was replaced, every method does nothing. A stage outside the three writer stages is ignored.
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

// Complete stores a, with Review.Revisions dropped, as done. It returns the
// generation's duration as it stored it, read under the same lock as the
// transition (once the lock is released a retry or an eviction may replace
// the record, so the caller never reads it back), and false when the Run is
// no longer valid and nothing changed.
func (r *Run) Complete(a writer.Article) (time.Duration, bool) {
	stored := withoutRevisions(a)
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	rec := r.running()
	if rec == nil {
		return 0, false
	}
	rec.state = JobDone
	rec.article = stored
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
