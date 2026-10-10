//go:build test

package writer

import (
	"errors"
	"strings"
	"testing"
)

// Markers placed in review values, so a test can confirm that an error message
// never carries a value.
const (
	markRevisionBefore   = "mark-before-Q1w2"
	markRevisionAfter    = "mark-after-E3r4"
	markRevisionEvidence = "mark-evidence-T5y6"
)

// validRevision returns a replacement revision that passes check.
func validRevision() Revision {
	return Revision{
		Before:   markRevisionBefore,
		Action:   ActionReplace,
		After:    markRevisionAfter,
		Reason:   ReasonMeaningChange,
		Evidence: markRevisionEvidence,
	}
}

func TestRevisionReasonWireValue(t *testing.T) {
	values := []string{"unsupported", "meaning_change", "speaker", "misrecognition", "unintelligible"}
	for _, wire := range values {
		t.Run(wire, func(t *testing.T) {
			reason, ok := revisionReasonFromWire(wire)
			if !ok {
				t.Fatalf("revisionReasonFromWire(%q) = false, want true", wire)
			}
			got, ok := reason.WireValue()
			if !ok || got != wire {
				t.Errorf("WireValue() = (%q, %v), want (%q, true)", got, ok, wire)
			}
		})
	}
	t.Run("unknown reason is false", func(t *testing.T) {
		if _, ok := ReasonUnknown.WireValue(); ok {
			t.Error("ReasonUnknown.WireValue() = true, want false")
		}
	})
	t.Run("out of range reason is false", func(t *testing.T) {
		if _, ok := RevisionReason(99).WireValue(); ok {
			t.Error("RevisionReason(99).WireValue() = true, want false")
		}
	})
	t.Run("wire value not in the table", func(t *testing.T) {
		for _, wire := range []string{"", "unknown", "Meaning_Change", "MEANING_CHANGE", " unsupported", "unsupported "} {
			if reason, ok := revisionReasonFromWire(wire); ok {
				t.Errorf("revisionReasonFromWire(%q) = (%v, true), want false", wire, reason)
			}
		}
	})
}

func TestReviewResultCheck(t *testing.T) {
	valid := validRevision()
	validDelete := validRevision()
	validDelete.Action = ActionDelete
	validDelete.After = ""
	validEvidenceNewline := validRevision()
	validEvidenceNewline.Evidence = "line one\nline two"
	validEvidenceTab := validRevision()
	validEvidenceTab.Evidence = "a\tb"

	accepted := []struct {
		name   string
		result ReviewResult
	}{
		{"empty revisions", ReviewResult{Model: StepModel{Model: markRevisionBefore}}},
		{"replacement", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{valid}}},
		{"deletion", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{validDelete}}},
		{"evidence with a line feed", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{validEvidenceNewline}}},
		{"tab in a value", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{validEvidenceTab}}},
		{"empty model version", ReviewResult{Model: StepModel{Model: markRevisionBefore, ModelVersion: ""}}},
		{"model version present", ReviewResult{Model: StepModel{Model: markRevisionBefore, ModelVersion: markRevisionAfter}}},
	}
	for _, tc := range accepted {
		t.Run("accepts/"+tc.name, func(t *testing.T) {
			if err := tc.result.Check(); err != nil {
				t.Errorf("Check() error = %v, want nil", err)
			}
		})
	}

	rejected := []struct {
		name   string
		result ReviewResult
	}{
		{"empty model", ReviewResult{Model: StepModel{Model: ""}}},
		{"whitespace model", ReviewResult{Model: StepModel{Model: " \t"}}},
		{"model with a control character", ReviewResult{Model: StepModel{Model: "a\x1b[31m"}}},
		{"unknown action", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionUnknown, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"out of range action", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: RevisionAction(99), After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"unknown reason", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionReplace, After: markRevisionAfter, Reason: ReasonUnknown, Evidence: markRevisionEvidence}}}},
		{"out of range reason", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionReplace, After: markRevisionAfter, Reason: RevisionReason(99), Evidence: markRevisionEvidence}}}},
		{"empty before", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: "", Action: ActionReplace, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"replace without after", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionReplace, After: "", Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"delete with after", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionDelete, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"whitespace only evidence", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionReplace, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: " \t "}}}},
		{"control character in before", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: "a\x01b", Action: ActionReplace, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"line feed in before", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: "a\nb", Action: ActionReplace, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"carriage return in after", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionReplace, After: "a\rb", Reason: ReasonMeaningChange, Evidence: markRevisionEvidence}}}},
		{"control character in evidence", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionReplace, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: "a\x01b"}}}},
		{"carriage return in evidence", ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: []Revision{{Before: markRevisionBefore, Action: ActionReplace, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: "a\rb"}}}},
	}
	for _, tc := range rejected {
		t.Run("rejects/"+tc.name, func(t *testing.T) {
			requireInvalidReview(t, tc.result.Check())
		})
	}

	t.Run("rejects/too many revisions", func(t *testing.T) {
		revisions := make([]Revision, maxRevisions+1)
		for i := range revisions {
			revisions[i] = validRevision()
		}
		requireInvalidReview(t, ReviewResult{Model: StepModel{Model: markRevisionBefore}, Revisions: revisions}.Check())
	})
}

// requireInvalidReview checks a ReviewResult.Check rejection: the error wraps
// ErrInvalidReview and carries no field value.
func requireInvalidReview(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidReview) {
		t.Fatalf("Check() error = %v, want ErrInvalidReview", err)
	}
	for _, mark := range []string{markRevisionBefore, markRevisionAfter, markRevisionEvidence} {
		if strings.Contains(err.Error(), mark) {
			t.Errorf("Check() error %q contains the value marked %q", err, mark)
		}
	}
}
