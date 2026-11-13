//go:build test

package chief

import (
	"slices"
	"testing"
	"time"
)

// TestSortPending checks the order: kind first, then the longest wait.
func TestSortPending(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	items := []PendingItem{
		{ID: "p-new", Kind: PendingProposal, FirstSeen: t0},
		{ID: "r-new", Kind: PendingReview, FirstSeen: t0.Add(time.Hour)},
		{ID: "m", Kind: PendingMergeConfirmation, FirstSeen: t0.Add(3 * time.Hour)},
		{ID: "q-new", Kind: PendingQuestion, FirstSeen: t0.Add(2 * time.Hour)},
		{ID: "q-old", Kind: PendingQuestion, FirstSeen: t0.Add(time.Hour)},
		{ID: "r-old", Kind: PendingReview, FirstSeen: t0},
	}
	sortPending(items)
	var ids []ItemID
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if want := []ItemID{"q-old", "q-new", "m", "r-old", "r-new", "p-new"}; !slices.Equal(ids, want) {
		t.Errorf("order = %v, want %v", ids, want)
	}
}

// TestPendingWaitsFromTheFirstSighting checks that an item keeps its first
// time across snapshots, so the wait grows, and that two sessions each
// announce it once.
func TestPendingWaitsFromTheFirstSighting(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON})
	first := take(t, w)
	w.now = w.now.Add(time.Hour)
	second := take(t, w)
	if len(first.Pending) != 1 || len(second.Pending) != 1 {
		t.Fatalf("pending = %d and %d items, want 1 and 1", len(first.Pending), len(second.Pending))
	}
	if !second.Pending[0].FirstSeen.Equal(first.Pending[0].FirstSeen) || second.Pending[0].New {
		t.Errorf("second sighting = %+v, want the first time and not new", second.Pending[0])
	}
	if first.Pending[0].ID != second.Pending[0].ID {
		t.Error("the item's identity changed between snapshots")
	}
	// After /clear the session changes: the same item is announced again.
	cleared := w.owner()
	cleared.Session = "00000000-0000-4000-8000-0000000000d4"
	snap, err := TakeSnapshot(t.Context(), w.finish(), cleared)
	if err != nil || len(snap.Pending) != 1 || !snap.Pending[0].New {
		t.Errorf("new session: %+v, %v; want the item announced again", snap.Pending, err)
	}
}

// TestPendingQuestionIdentity checks that the same question text is the same
// item and a different option text is another.
func TestPendingQuestionIdentity(t *testing.T) {
	q := Question{Text: "Which?", Options: []Option{{Number: 1, Label: "A"}, {Number: 2, Label: "B"}}}
	same := q
	other := Question{Text: "Which?", Options: []Option{{Number: 1, Label: "A"}, {Number: 2, Label: "C"}}}
	if questionID("/w", q) != questionID("/w", same) {
		t.Error("the same question has two identities")
	}
	if questionID("/w", q) == questionID("/w", other) || questionID("/w", q) == questionID("/x", q) {
		t.Error("different questions or workspaces share an identity")
	}
}

// TestNeedsCheckRowIsNotPending checks that a row that needs a check is
// listed in the table but is no item waiting on a human decision.
func TestNeedsCheckRowIsNotPending(t *testing.T) {
	if items := buildPending([]Row{{State: StateNeedsCheck, Unreadable: []string{"x"}}, {State: StateRunning}, {State: StateTaskDone}}); len(items) != 0 {
		t.Errorf("items = %+v", items)
	}
}
