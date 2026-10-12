package writer

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/strictjson"
)

// Response keys of a review response. The parser and the default review
// template instruction share them.
const (
	reviewKeyRevisions = "revisions"
	reviewKeyBefore    = "before"
	reviewKeyAfter     = "after"
	reviewKeyDelete    = "delete"
	reviewKeyReason    = "reason"
	reviewKeyEvidence  = "evidence"
)

// headingPrefix starts a heading line whose set must not change under
// applyRevisions.
const headingPrefix = "## "

// revisionKeys are the keys a revision object may carry, in the shape of a
// CollectOnly call.
var revisionKeys = []string{reviewKeyBefore, reviewKeyAfter, reviewKeyDelete, reviewKeyReason, reviewKeyEvidence}

// parseReviewResponse turns the review LLM's response text into the list of
// revisions. It rejects any response that breaks the acceptance rules and
// returns no partial list. The errors wrap ErrMalformedOutput and name the
// broken rule and the item index only, never a response value, because the
// response is untrusted.
func parseReviewResponse(text string) ([]Revision, error) {
	if len(text) > maxTextBytes {
		return nil, fmt.Errorf("%w: review response exceeds the limit of %d bytes", ErrMalformedOutput, maxTextBytes)
	}
	object, err := strictjson.ParseObject([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("%w: review response is not one JSON object: %w", ErrMalformedOutput, err)
	}
	members, err := object.CollectOnly(reviewKeyRevisions)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedOutput, err)
	}
	raw, err := strictjson.Required(members, reviewKeyRevisions)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedOutput, err)
	}
	elements, err := raw.AsArray()
	if err != nil {
		return nil, fmt.Errorf("%w: revisions: %w", ErrMalformedOutput, err)
	}
	if len(elements) > maxRevisions {
		return nil, fmt.Errorf("%w: %w: limit %d", ErrMalformedOutput, errTooManyRevisions, maxRevisions)
	}
	revisions := make([]Revision, 0, len(elements))
	for i, element := range elements {
		revision, err := parseRevision(element)
		if err != nil {
			return nil, fmt.Errorf("%w: revisions[%d]: %w", ErrMalformedOutput, i, err)
		}
		revisions = append(revisions, revision)
	}
	return revisions, nil
}

// parseRevision turns one element of the revisions array into a Revision. It
// rejects a non-object, an unknown or duplicate key, a missing required key,
// and any value rule of a revision.
func parseRevision(value strictjson.Value) (Revision, error) {
	object, err := value.AsObject()
	if err != nil {
		return Revision{}, err
	}
	members, err := object.CollectOnly(revisionKeys...)
	if err != nil {
		return Revision{}, err
	}
	before, err := strictjson.RequiredString(members, reviewKeyBefore)
	if err != nil {
		return Revision{}, err
	}
	after, hasAfter, err := strictjson.OptionalString(members, reviewKeyAfter)
	if err != nil {
		return Revision{}, err
	}
	deleted, hasDelete, err := optionalBool(members, reviewKeyDelete)
	if err != nil {
		return Revision{}, err
	}
	if hasAfter == hasDelete {
		return Revision{}, errRevisionAfterDelete
	}
	if hasDelete && !deleted {
		return Revision{}, errRevisionDeleteFalse
	}
	reasonWire, err := strictjson.RequiredString(members, reviewKeyReason)
	if err != nil {
		return Revision{}, err
	}
	reason, ok := revisionReasonFromWire(reasonWire)
	if !ok {
		return Revision{}, errUnknownRevisionReason
	}
	evidence, err := strictjson.RequiredString(members, reviewKeyEvidence)
	if err != nil {
		return Revision{}, err
	}
	revision := Revision{Before: before, Reason: reason, Evidence: evidence}
	if hasAfter {
		revision.Action = ActionReplace
		revision.After = after
	} else {
		revision.Action = ActionDelete
	}
	if err := revision.check(); err != nil {
		return Revision{}, err
	}
	return revision, nil
}

// optionalBool returns the named member as a boolean when it is present.
// present is false for a missing member; null and other kinds are rejected.
func optionalBool(members map[string]strictjson.Value, key string) (value bool, present bool, err error) {
	raw, ok := members[key]
	if !ok {
		return false, false, nil
	}
	value, err = raw.AsBool()
	if err != nil {
		return false, false, fmt.Errorf("%s: %w", key, err)
	}
	return value, true, nil
}

// applyRevisions replaces each revision's Before with its After, or removes it
// for a deletion, in text and returns the revised text. A Before or After has
// no line feed (the parser rejected one), so the line structure does not
// change. It rejects a Before that does not appear exactly once, counting
// overlapping occurrences, ranges that share any byte, and any result whose
// "## " headings differ from text's in count, text, or order. The errors wrap
// ErrMalformedOutput and name the item index or the first changed heading index
// only, never a value.
func applyRevisions(text string, revisions []Revision) (string, error) {
	spans := make([]revisionSpan, 0, len(revisions))
	for i, revision := range revisions {
		start := strings.Index(text, revision.Before)
		if start < 0 {
			return "", fmt.Errorf("%w: revisions[%d]: %w", ErrMalformedOutput, i, errRevisionBeforeNotFound)
		}
		if strings.Contains(text[start+1:], revision.Before) {
			return "", fmt.Errorf("%w: revisions[%d]: %w", ErrMalformedOutput, i, errRevisionBeforeAmbiguous)
		}
		spans = append(spans, revisionSpan{
			index:  i,
			start:  start,
			end:    start + len(revision.Before),
			after:  revision.After,
			delete: revision.Action == ActionDelete,
		})
	}
	slices.SortFunc(spans, func(a, b revisionSpan) int { return cmp.Compare(a.start, b.start) })
	maxEnd := 0
	for i, span := range spans {
		if i > 0 && span.start < maxEnd {
			return "", fmt.Errorf("%w: revisions[%d]: %w", ErrMalformedOutput, span.index, errRevisionOverlap)
		}
		if span.end > maxEnd {
			maxEnd = span.end
		}
	}
	var builder strings.Builder
	last := 0
	for _, span := range spans {
		builder.WriteString(text[last:span.start])
		if !span.delete {
			builder.WriteString(span.after)
		}
		last = span.end
	}
	builder.WriteString(text[last:])
	revised := builder.String()
	if index, changed := firstHeadingDifference(headingLines(text), headingLines(revised)); changed {
		return "", fmt.Errorf("%w: headings differ at %d", ErrMalformedOutput, index)
	}
	return revised, nil
}

// revisionSpan is one revision's matched range in the text, with its
// replacement and the revision's index in the original list.
type revisionSpan struct {
	index  int
	start  int
	end    int
	after  string
	delete bool
}

// headingLines returns every line of text that starts with "## ", in order.
// A "## " line inside a code fence counts too: the requirement compares the
// headings as written, so this errs toward rejecting more.
func headingLines(text string) []string {
	var headings []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, headingPrefix) {
			headings = append(headings, line)
		}
	}
	return headings
}

// firstHeadingDifference returns the index of the first heading that differs
// between before and after, and whether any differs. A missing heading at the
// end of the shorter list is a difference at its length.
func firstHeadingDifference(before, after []string) (int, bool) {
	limit := min(len(before), len(after))
	for i := range limit {
		if before[i] != after[i] {
			return i, true
		}
	}
	if len(before) != len(after) {
		return limit, true
	}
	return 0, false
}
