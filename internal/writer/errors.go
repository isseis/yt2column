package writer

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by New and Write. They are wrapped with context;
// callers identify a failure with errors.Is.
var (
	ErrInvalidTemplate   = errors.New("invalid prompt template")
	ErrInvalidTranscript = errors.New("invalid transcript")
	ErrMalformedOutput   = errors.New("malformed LLM output")
	// ErrInvalidArticle is returned by Article.CheckPublishable.
	ErrInvalidArticle = errors.New("invalid article")
	// ErrInvalidReview is returned by ReviewResult.Check.
	ErrInvalidReview = errors.New("invalid review result")
	// ErrTooLong reports that the body is over the length limit after the
	// shorten step or after the review step.
	ErrTooLong = errors.New("article body exceeds the length limit")
)

var (
	// errNilLLMClient rejects a nil or typed-nil LLM client in New. It is not
	// exported: a missing dependency is a programming error, not a condition
	// callers branch on.
	errNilLLMClient = errors.New("LLM client is nil")
	// errReviewClientNil, errReviewClientSet, and errDisabledStepTemplate
	// reject the step combinations New must not accept (an enabled review
	// step without a client, a client set while the step is disabled, and a
	// template path for a disabled step). They are not exported: each is a
	// caller mistake, not a condition to branch on.
	errReviewClientNil      = errors.New("review step is enabled but its LLM client is nil")
	errReviewClientSet      = errors.New("review step is disabled but its LLM client is set")
	errDisabledStepTemplate = errors.New("a disabled step has a template path")
	// errNotRegularFile is joined with ErrInvalidTemplate when an override
	// path names a directory, FIFO, device, or socket.
	errNotRegularFile = errors.New("not a regular file")
	// errTemplateDefinition and errDisallowedSyntax are joined with
	// ErrInvalidTemplate when a template uses {{define}} or {{block}}, or
	// syntax outside the allowlist, so tests can tell which check rejected
	// a template.
	errTemplateDefinition = errors.New("define and block are not allowed")
	errDisallowedSyntax   = errors.New("disallowed syntax")
	// errPromptTooLarge is returned by boundedWriter when a write would take
	// a prompt past its limit, and joined with ErrInvalidTemplate.
	errPromptTooLarge = errors.New("prompt exceeds the limit")
	// errRawHTML, errUnclosedFence, and errFenceLikeLine are wrapped together
	// with ErrMalformedOutput when the body part breaks a Markdown rule, so
	// tests can tell which rule rejected it.
	errRawHTML       = errors.New("contains raw HTML")
	errUnclosedFence = errors.New("ends inside an unclosed code fence")
	errFenceLikeLine = errors.New("has a line that looks like a code fence but does not open one at the top level")
	// errLeadingBOM is wrapped together with ErrMalformedOutput when the body
	// part still starts with U+FEFF after normalizeBody removed one.
	errLeadingBOM = errors.New("starts with a byte order mark after normalization")
)

// Review response rule errors. parseReviewResponse wraps one with
// ErrMalformedOutput and ReviewResult.Check wraps one with ErrInvalidReview, so
// a caller can tell which rule rejected a response without seeing a value.
var (
	errTooManyRevisions        = errors.New("too many revisions")
	errRevisionAfterDelete     = errors.New("revision has neither after nor delete, or both")
	errRevisionDeleteFalse     = errors.New("revision delete is false")
	errUnknownRevisionAction   = errors.New("revision action is not known")
	errUnknownRevisionReason   = errors.New("revision reason is not known")
	errEmptyRevisionBefore     = errors.New("revision before is empty")
	errRevisionAfterMissing    = errors.New("replacement revision has no after")
	errRevisionAfterUnexpected = errors.New("deletion revision has an after")
	errEmptyRevisionEvidence   = errors.New("revision evidence is empty or whitespace only")
	errRevisionControlChar     = errors.New("revision value contains a control character")
	errRevisionBeforeNotFound  = errors.New("revision before does not appear in the reviewed text")
	errRevisionBeforeAmbiguous = errors.New("revision before appears in the reviewed text more than once")
	errRevisionOverlap         = errors.New("revision ranges overlap")
)

// Step identifies a post-generation step in a StepError. The zero value is
// StepUnknown.
type Step int

// StepUnknown is the zero value of Step, which no step uses.
const (
	StepUnknown Step = iota
	StepShorten
	StepReview
)

// String returns "shorten" or "review"; StepUnknown and values outside the
// enumeration return "unknown".
func (s Step) String() string {
	switch s {
	case StepShorten:
		return "shorten"
	case StepReview:
		return "review"
	default:
		return "unknown"
	}
}

// StepError wraps every failure of a post-generation step. Error returns
// "<step> step: <original error>"; Unwrap returns the original error.
type StepError struct {
	Step Step
	Err  error
}

func (e *StepError) Error() string {
	return fmt.Sprintf("%s step: %s", e.Step, e.Err)
}

func (e *StepError) Unwrap() error { return e.Err }

// stepError wraps err as the failure of step. Every failure a step can report
// goes through it, so a caller reads the failing step from the type instead of
// from the message text.
func stepError(step Step, err error) error {
	return &StepError{Step: step, Err: err}
}
