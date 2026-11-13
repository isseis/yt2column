package chief

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
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
	default:
		return "unknown"
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
	// FirstSeen is when chief first found the item in this session; the
	// time of the observation when no record could be made.
	FirstSeen time.Time
	// New is true when the item was not announced before in this session.
	New bool
}

// itemID builds an opaque identity from parts. The parts are joined with a
// byte no part holds, so two different part lists never give one string.
func itemID(parts ...string) ItemID {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return ItemID(hex.EncodeToString(sum[:16]))
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
// PR, and a proposal for each next command or takeover that can be sent.
// Times are not set. Contract: a needs-check row yields no item.
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
				add(PendingItem{ID: itemID("review", r.Workspace, strconv.Itoa(r.PR.Number)), Kind: PendingReview, Workspace: r.Workspace, Label: r.Label, PR: r.PR})
			}
		case StateProposeNext:
			for _, n := range []*Next{r.Next, r.Takeover} {
				if n == nil {
					continue
				}
				add(PendingItem{ID: itemID("proposal", r.Workspace, strconv.Itoa(int(n.Kind)), n.Text), Kind: PendingProposal, Workspace: r.Workspace, Label: r.Label, Proposal: n})
			}
		}
	}
	return items
}

// kindRank is the order of kinds: the questions that stop an agent, the
// merge confirmations, the PRs waiting for review, then the other proposals.
func kindRank(k PendingKind) int {
	i := slices.Index(kindOrder, k)
	if i < 0 {
		return len(kindOrder)
	}
	return i
}

// kindOrder is the order of the kinds in the list of pending items.
var kindOrder = []PendingKind{PendingQuestion, PendingMergeConfirmation, PendingReview, PendingProposal}

// nameQuestion names the question state and kind in the output.
const nameQuestion = "question"

// sortPending orders items by kind and then by how long they have waited,
// longest first; ties keep a fixed order (workspace, then ID) so a listing
// does not shuffle between two reads of the same state.
func sortPending(items []PendingItem) {
	slices.SortStableFunc(items, func(a, b PendingItem) int {
		return cmp.Or(
			cmp.Compare(kindRank(a.Kind), kindRank(b.Kind)),
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
