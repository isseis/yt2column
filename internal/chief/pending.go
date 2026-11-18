package chief

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// ItemID identifies a pending item within one chief session. It is opaque:
// the session passes it back and never builds or parses it.
type ItemID string

// PendingKind is the kind of an item waiting on the human. The zero value is
// PendingUnknown and is never listed.
type PendingKind int

const (
	// PendingUnknown is no kind.
	PendingUnknown PendingKind = iota
	// PendingQuestion is a question that stops an agent.
	PendingQuestion
	// PendingMergeConfirmation is /runpr's merge question.
	PendingMergeConfirmation
	// PendingReview is an open PR waiting for review.
	PendingReview
	// PendingProposal is a proposed command.
	PendingProposal
	// PendingReset is a skipped reset offered to the human.
	PendingReset
	// PendingRemoval is the removal of a workspace chief made whose task is
	// done, proposed to the human.
	PendingRemoval
)

// String returns the name the output uses for the kind.
func (k PendingKind) String() string {
	switch k {
	case PendingQuestion:
		return nameQuestion
	case PendingMergeConfirmation:
		return "merge_confirmation"
	case PendingReview:
		return "review"
	case PendingProposal:
		return "proposal"
	case PendingReset:
		return "reset"
	case PendingRemoval:
		return "removal"
	default:
		return unknownName
	}
}

// PendingItem is one thing that waits on the human.
type PendingItem struct {
	ID        ItemID
	Kind      PendingKind
	Workspace string
	Label     string
	Question  *QuestionItem
	PR        *OpenPR
	Proposal  *Next
	// Reset is the skipped reset of a reset item.
	Reset *ResetItem
	// Removal is the proposal of a removal item.
	Removal *RemovalProposal
	// Reasons are why a proposal was not sent by itself after a merge: the
	// conditions of the automatic send that did not hold.
	Reasons []string
	// ReviewSkippable marks a review item whose PR detail, kept for the
	// worktree's HEAD, judged the review skippable.
	ReviewSkippable bool
	// FirstSeen is when chief first found the item in this session; the
	// time of the observation when no record could be made.
	FirstSeen time.Time
	// New is true when the item was not announced before in this session.
	New bool
	// Number is the item's number in this session's lists: given when the
	// item is first seen and kept, never given to another item; 0 when none
	// could be given.
	Number int
}

// itemID builds an opaque identity from parts. The parts are joined with a
// byte no part holds, so two different part lists never give one string.
func itemID(parts ...string) ItemID {
	return ItemID(hashName(strings.Join(parts, "\x00")))
}

// questionID is a question's identity: the workspace and the question and
// option texts as read, so the same question shown again is the same item.
func questionID(workspace string, q Question) ItemID {
	parts := []string{nameQuestion, workspace, q.Text}
	for _, o := range q.Options {
		parts = append(parts, strconv.Itoa(o.Number), o.Label, o.Description)
	}
	return itemID(parts...)
}

// buildPending lists the items waiting on the human in rows: a question or a
// merge confirmation for each distinct question, a review item for each open
// PR, a proposal for each next command or takeover, including one that
// cannot be sent as it is (a /mkreq waits for the human's issue number, and
// any other says why in its Problem), a reset item for each reset the row
// offers, whatever its state, and a removal item for each row with a removal
// proposal. Times are not set. Contract: a needs-check row yields no item
// but its removal item, whose checks say what stops the removal.
func buildPending(rows []Row) []PendingItem {
	var items []PendingItem
	seen := map[ItemID]struct{}{}
	add := func(it PendingItem) {
		if _, dup := seen[it.ID]; dup {
			return
		}
		seen[it.ID] = struct{}{}
		items = append(items, it)
	}
	for _, r := range rows {
		switch r.State {
		case StateQuestion:
			for i := range r.Questions {
				q := r.Questions[i]
				kind := PendingQuestion
				if q.Question.Merge {
					kind = PendingMergeConfirmation
				}
				add(PendingItem{ID: questionID(r.Workspace, q.Question), Kind: kind, Workspace: r.Workspace, Label: r.Label, Question: &q})
			}
		case StateAwaitingReview:
			if r.PR != nil {
				add(PendingItem{ID: itemID("review", r.Workspace, strconv.Itoa(r.PR.Number)), Kind: PendingReview, Workspace: r.Workspace, Label: r.Label, PR: r.PR, ReviewSkippable: r.ReviewSkippable})
			}
		case StateProposeNext:
			for _, n := range []*Next{r.Next, r.Takeover} {
				if n == nil {
					continue
				}
				add(PendingItem{ID: proposalID(r.Workspace, n), Kind: PendingProposal, Workspace: r.Workspace, Label: r.Label, Proposal: n})
			}
		}
		if r.Removal != nil {
			add(PendingItem{ID: removalID(r.Removal), Kind: PendingRemoval, Workspace: r.Workspace, Label: r.Label, Removal: r.Removal})
		}
		if r.State == StateNeedsCheck {
			continue
		}
		for i := range r.Resets {
			it := r.Resets[i]
			add(PendingItem{ID: it.id(r.Workspace), Kind: PendingReset, Workspace: r.Workspace, Label: r.Label, Reset: &it})
		}
	}
	return items
}

// proposalID is the identity of a proposal of n for a workspace: the same
// command proposed again is the same item.
func proposalID(workspace string, n *Next) ItemID {
	return itemID("proposal", workspace, strconv.Itoa(int(n.Kind)), n.Text, n.Key)
}

// mergePending adds extra to items: an extra item of the identity of an item
// already listed gives that item its reasons, and any other is appended.
// Contract: no item is listed twice and none is dropped.
func mergePending(items, extra []PendingItem) []PendingItem {
	for _, e := range extra {
		i := slices.IndexFunc(items, func(it PendingItem) bool { return it.ID == e.ID })
		if i < 0 {
			items = append(items, e)
			continue
		}
		items[i].Reasons = append(items[i].Reasons, e.Reasons...)
	}
	return items
}

// kindRank is the order of kinds: the questions that stop an agent, the
// merge confirmations, the PRs waiting for review, the other proposals, a
// reset item among them, then the removals.
func kindRank(k PendingKind) int {
	if k == PendingReset {
		k = PendingProposal
	}
	i := slices.Index(kindOrder, k)
	if i < 0 {
		return len(kindOrder)
	}
	return i
}

// kindOrder is the order of the kinds in the list of pending items.
var kindOrder = []PendingKind{PendingQuestion, PendingMergeConfirmation, PendingReview, PendingProposal, PendingRemoval}

// nameQuestion names the question state and kind in the output.
const nameQuestion = "question"

// sortPending orders items by kind, then, except for removals, the items of
// other workspaces before those of idle ones (Row.Idle), then by how long they have
// waited, longest first; ties keep a fixed order (workspace, then ID) so a listing
// does not shuffle between two reads of the same state.
func sortPending(items []PendingItem, idle map[string]struct{}) {
	rank := func(p PendingItem) int {
		// Removals wait in the order they were first seen, whatever the
		// workspace's row: idleness orders the commands to send, not them.
		if p.Kind == PendingRemoval {
			return 0
		}
		if _, ok := idle[p.Workspace]; ok {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(items, func(a, b PendingItem) int {
		return cmp.Or(
			cmp.Compare(kindRank(a.Kind), kindRank(b.Kind)),
			cmp.Compare(rank(a), rank(b)),
			a.FirstSeen.Compare(b.FirstSeen),
			strings.Compare(a.Workspace, b.Workspace),
			strings.Compare(string(a.ID), string(b.ID)),
		)
	})
}

// stampPending sets FirstSeen and New of each item from the notice records
// of session. A record that cannot be made or read costs only the item's
// time (it is now) and its newness (see Store.Observe), and its error is
// returned for the output's problems. Contract: no item is dropped and no
// item is listed twice.
func stampPending(store Store, session string, items []PendingItem, now time.Time) []error {
	var errs []error
	for i := range items {
		first, isNew, err := store.Observe(session, items[i].ID, now)
		items[i].FirstSeen = first
		items[i].New = isNew
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if items[i].Number, err = store.NoticeNumber(session, items[i].ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// agentLabel returns the agent's name for display.
func agentLabel(a paneinput.AgentKind) string {
	switch a {
	case paneinput.AgentClaudeCode:
		return "claude_code"
	case paneinput.AgentOpenCode:
		return "opencode"
	default:
		return ""
	}
}
