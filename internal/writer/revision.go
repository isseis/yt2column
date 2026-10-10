package writer

import (
	"fmt"
	"strings"
	"unicode"
)

// maxRevisions caps the number of entries a review response may carry. It is
// checked after AsArray splits the array and again by ReviewResult.Check.
const maxRevisions = 256

// ReviewResult is what the review step recorded.
type ReviewResult struct {
	Model     StepModel
	Revisions []Revision // in response order; empty when nothing was revised
}

// Check reports whether r could have come from an accepted review response:
// Model passes the display-string rules, there are at most maxRevisions
// revisions, and every revision has a known Action and Reason, a non-empty
// Before, an After that is non-empty exactly when Action is ActionReplace, an
// Evidence that is not whitespace only, and no character that the response
// rules of each value reject. It wraps ErrInvalidReview and never puts a field
// value in the error.
func (r ReviewResult) Check() error {
	if reason := displayStringProblem("model", r.Model.Model); reason != "" {
		return fmt.Errorf("%w: %s", ErrInvalidReview, reason)
	}
	if len(r.Revisions) > maxRevisions {
		return fmt.Errorf("%w: %w: limit %d", ErrInvalidReview, errTooManyRevisions, maxRevisions)
	}
	for i, revision := range r.Revisions {
		if err := revision.check(); err != nil {
			return fmt.Errorf("%w: revisions[%d]: %w", ErrInvalidReview, i, err)
		}
	}
	return nil
}

// Revision is one entry of the review list.
type Revision struct {
	Before   string // verbatim quote of the reviewed text
	Action   RevisionAction
	After    string // the replacement; empty exactly when Action is ActionDelete
	Reason   RevisionReason
	Evidence string // the quoted source material
}

// check reports whether r satisfies the value rules a revision of an accepted
// response must satisfy. The parser wraps the rejection with
// ErrMalformedOutput; ReviewResult.Check wraps it with ErrInvalidReview.
func (r Revision) check() error {
	switch r.Action {
	case ActionReplace:
		if r.After == "" {
			return errRevisionAfterMissing
		}
	case ActionDelete:
		if r.After != "" {
			return errRevisionAfterUnexpected
		}
	default:
		return errUnknownRevisionAction
	}
	if _, ok := r.Reason.WireValue(); !ok {
		return errUnknownRevisionReason
	}
	if r.Before == "" {
		return errEmptyRevisionBefore
	}
	if revisionTextProblem(r.Before, false) {
		return errRevisionControlChar
	}
	if r.Action == ActionReplace && revisionTextProblem(r.After, false) {
		return errRevisionControlChar
	}
	if strings.TrimSpace(r.Evidence) == "" {
		return errEmptyRevisionEvidence
	}
	if revisionTextProblem(r.Evidence, true) {
		return errRevisionControlChar
	}
	return nil
}

// revisionTextProblem reports whether value contains a character the response
// rules reject: any control character except a tab, and, unless allowNewline,
// a line feed as well.
func revisionTextProblem(value string, allowNewline bool) bool {
	return strings.ContainsFunc(value, func(r rune) bool {
		if r == '\t' || (allowNewline && r == '\n') {
			return false
		}
		return unicode.IsControl(r)
	})
}

// RevisionAction is what a revision does. The zero value is ActionUnknown,
// which Check rejects.
type RevisionAction int

// ActionUnknown is the zero value of RevisionAction, which Check rejects.
const (
	ActionUnknown RevisionAction = iota
	ActionReplace
	ActionDelete
)

// RevisionReason is the kind of problem a revision fixes. The zero value is
// ReasonUnknown, which Check rejects.
type RevisionReason int

// ReasonUnknown is the zero value of RevisionReason, which Check rejects.
const (
	ReasonUnknown        RevisionReason = iota
	ReasonUnsupported                   // "unsupported": no basis in the material
	ReasonMeaningChange                 // "meaning_change": a paraphrase or omission that changes the meaning
	ReasonSpeaker                       // "speaker": a wrong speaker attribution
	ReasonMisrecognition                // "misrecognition": a copied speech-recognition error
	ReasonUnintelligible                // "unintelligible": a sentence whose meaning cannot be made out
)

// revisionReasons pairs each RevisionReason with the exact string it uses in a
// review response. It is the single source of the spellings: WireValue and the
// response parser both read it.
var revisionReasons = []struct {
	reason RevisionReason
	wire   string
}{
	{ReasonUnsupported, "unsupported"},
	{ReasonMeaningChange, "meaning_change"},
	{ReasonSpeaker, "speaker"},
	{ReasonMisrecognition, "misrecognition"},
	{ReasonUnintelligible, "unintelligible"},
}

// WireValue returns the response value of r ("unsupported", ...) and true, or
// "" and false for ReasonUnknown and values outside the enumeration.
func (r RevisionReason) WireValue() (string, bool) {
	for _, entry := range revisionReasons {
		if entry.reason == r {
			return entry.wire, true
		}
	}
	return "", false
}

// revisionReasonFromWire returns the RevisionReason whose wire value is wire,
// or ReasonUnknown and false for a value outside the enumeration. It matches
// exactly: no case folding and no trimming.
func revisionReasonFromWire(wire string) (RevisionReason, bool) {
	for _, entry := range revisionReasons {
		if entry.wire == wire {
			return entry.reason, true
		}
	}
	return ReasonUnknown, false
}
