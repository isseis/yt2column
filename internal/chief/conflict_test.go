//go:build test && unix

package chief

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/privatefile"
)

func mustCommitID(t *testing.T, hex string) CommitID {
	t.Helper()
	id, err := ParseCommitID(hex)
	if err != nil {
		t.Fatalf("ParseCommitID(%q): %v", hex, err)
	}
	return id
}

func prListEntryJSON(branch string, number int, title, head, mergeable, base string, cross bool) string {
	return fmt.Sprintf(`[{"headRefName":%q,"number":%d,"title":%q,"url":"https://github.com/o/r/pull/%d","mergeable":%q,"headRefOid":%q,"baseRefName":%q,"isCrossRepository":%t}]`,
		branch, number, title, number, mergeable, head, base, cross)
}

// TestConflictPhaseTransitions checks conflictPhaseOf and its snapshot
// integration across every row of the design's end-record table, unreadable
// records, newest-send selection, different PR or head, stop priority and
// release, and worktree recreation (ClearRecords at the same path).
func TestConflictPhaseTransitions(t *testing.T) {
	headA := mustCommitID(t, strings.Repeat("a", 40))
	headB := mustCommitID(t, strings.Repeat("b", 40))
	pr := OpenPR{Number: 50, Mergeable: MergeableConflicting, Head: headA, BaseIsMain: true}
	now := time.Date(2026, 11, 25, 11, 0, 0, 0, time.UTC)

	t.Run("end record combinations", func(t *testing.T) {
		strPtr := func(s string) *string { return &s }
		cases := []struct {
			name string
			end  *sendEnd
			want ConflictPhase
		}{
			{"no end record", nil, ConflictSending},
			{"not_sent before input", &sendEnd{Outcome: OutcomeNotSent.String(), Step: strPtr(nameNone)}, ConflictEndedBeforeCommand},
			{"sent command", &sendEnd{Outcome: OutcomeSent.String(), Step: strPtr(StepCommand.String())}, ConflictUnresolved},
			{"sent_then_failed after clear", &sendEnd{Outcome: OutcomeSentThenFailed.String(), Step: strPtr(StepClear.String())}, ConflictEndedBeforeCommand},
			{"sent_then_failed after model", &sendEnd{Outcome: OutcomeSentThenFailed.String(), Step: strPtr(StepModel.String())}, ConflictEndedBeforeCommand},
			{"sent_then_failed after command", &sendEnd{Outcome: OutcomeSentThenFailed.String(), Step: strPtr(StepCommand.String())}, ConflictOutcomeUnknown},
			{"released with absent step", &sendEnd{Outcome: outcomeUnknown, Step: nil}, ConflictOutcomeUnknown},
			{"not_sent with absent step", &sendEnd{Outcome: OutcomeNotSent.String(), Step: nil}, ConflictOutcomeUnknown},
			{"not_sent with command step", &sendEnd{Outcome: OutcomeNotSent.String(), Step: strPtr(StepCommand.String())}, ConflictOutcomeUnknown},
			{"sent with clear step", &sendEnd{Outcome: OutcomeSent.String(), Step: strPtr(StepClear.String())}, ConflictOutcomeUnknown},
			{"sent_then_failed with none step", &sendEnd{Outcome: OutcomeSentThenFailed.String(), Step: strPtr(nameNone)}, ConflictOutcomeUnknown},
			{"sent_then_failed with new step", &sendEnd{Outcome: OutcomeSentThenFailed.String(), Step: strPtr(StepNew.String())}, ConflictOutcomeUnknown},
			{"unrecognized step string", &sendEnd{Outcome: OutcomeNotSent.String(), Step: strPtr("bogus")}, ConflictOutcomeUnknown},
			{"unrecognized outcome", &sendEnd{Outcome: "other", Step: strPtr(StepCommand.String())}, ConflictOutcomeUnknown},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				store := newStore(t)
				held := heldRecords(t, store, "/work/phase")
				id, err := held.BeginSend(SendRecord{
					Pane: "w1:p1", Input: InputCommand.String(), Command: "/mergemain 50",
					Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headA,
				}, now)
				if err != nil {
					t.Fatal(err)
				}
				if c.end != nil {
					end := *c.end
					end.ID = id
					end.Ended = now.Add(time.Second)
					if err := held.store().create(sendPrefix+id+endSuffix, end); err != nil {
						t.Fatal(err)
					}
				}
				if got := conflictPhaseOf(held.WorkspaceRecords, pr); got != c.want {
					t.Errorf("conflictPhaseOf = %s, want %s", conflictPhaseName(got), conflictPhaseName(c.want))
				}
			})
		}
	})

	t.Run("newest matching send wins and other PR or head ignored", func(t *testing.T) {
		store := newStore(t)
		held := heldRecords(t, store, "/work/order")
		// Different PR and different head do not match pr (50, headA).
		for i, rec := range []SendRecord{
			{Pane: "w1:p1", Input: InputCommand.String(), Kind: paneinput.CommandMergemain.Name(), Arg: "51", PR: 51, Head: headA},
			{Pane: "w1:p1", Input: InputCommand.String(), Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headB},
			{Pane: "w1:p1", Input: InputCommand.String(), Kind: paneinput.CommandRunpr.Name(), Arg: "50"},
		} {
			id, err := held.BeginSend(rec, now.Add(time.Duration(i)*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if err := held.EndSend(id, OutcomeSent.String(), "", StepCommand, now.Add(time.Duration(i)*time.Second)); err != nil {
				t.Fatal(err)
			}
		}
		if got := conflictPhaseOf(held.WorkspaceRecords, pr); got != ConflictUnsent {
			t.Fatalf("unrelated sends: phase = %s, want unsent", conflictPhaseName(got))
		}
		// First matching send ended before command; second matching send succeeded.
		id1, err := held.BeginSend(SendRecord{
			Pane: "w1:p1", Input: InputCommand.String(), Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headA,
		}, now.Add(10*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if err := held.EndSend(id1, OutcomeNotSent.String(), "refused", StepUnknown, now.Add(11*time.Second)); err != nil {
			t.Fatal(err)
		}
		if got := conflictPhaseOf(held.WorkspaceRecords, pr); got != ConflictEndedBeforeCommand {
			t.Fatalf("after first send: phase = %s, want ended_before_command", conflictPhaseName(got))
		}
		id2, err := held.BeginSend(SendRecord{
			Pane: "w1:p1", Input: InputCommand.String(), Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headA,
		}, now.Add(20*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		if err := held.EndSend(id2, OutcomeSent.String(), "", StepCommand, now.Add(21*time.Second)); err != nil {
			t.Fatal(err)
		}
		if got := conflictPhaseOf(held.WorkspaceRecords, pr); got != ConflictUnresolved {
			t.Errorf("after second send: phase = %s, want unresolved", conflictPhaseName(got))
		}
	})

	t.Run("unreadable send records return records_unreadable", func(t *testing.T) {
		cases := []struct {
			name  string
			setup func(t *testing.T, held *HeldWorkspace)
		}{
			{"corrupt start record", func(t *testing.T, held *HeldWorkspace) {
				id := fmt.Sprintf("%020d-1111111111111111", now.UnixNano())
				if _, err := privatefile.Create(filepath.Join(held.dir, sendPrefix+id+startSuffix), []byte("{bad")); err != nil {
					t.Fatal(err)
				}
			}},
			{"start record mismatched ID", func(t *testing.T, held *HeldWorkspace) {
				id := fmt.Sprintf("%020d-2222222222222222", now.UnixNano())
				if err := held.store().create(sendPrefix+id+startSuffix, SendRecord{ID: "other"}); err != nil {
					t.Fatal(err)
				}
			}},
			{"mergemain start missing head", func(t *testing.T, held *HeldWorkspace) {
				id := fmt.Sprintf("%020d-3333333333333333", now.UnixNano())
				if err := held.store().create(sendPrefix+id+startSuffix, SendRecord{ID: id, Kind: paneinput.CommandMergemain.Name(), PR: 50}); err != nil {
					t.Fatal(err)
				}
			}},
			{"corrupt end record", func(t *testing.T, held *HeldWorkspace) {
				id, err := held.BeginSend(SendRecord{
					Pane: "w1:p1", Input: InputCommand.String(), Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headA,
				}, now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := privatefile.Create(filepath.Join(held.dir, sendPrefix+id+endSuffix), []byte("{bad")); err != nil {
					t.Fatal(err)
				}
			}},
			{"end record mismatched ID", func(t *testing.T, held *HeldWorkspace) {
				id, err := held.BeginSend(SendRecord{
					Pane: "w1:p1", Input: InputCommand.String(), Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headA,
				}, now)
				if err != nil {
					t.Fatal(err)
				}
				if err := held.store().create(sendPrefix+id+endSuffix, sendEnd{ID: "wrong", Outcome: OutcomeSent.String()}); err != nil {
					t.Fatal(err)
				}
			}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				store := newStore(t)
				held := heldRecords(t, store, "/work/bad")
				c.setup(t, held)
				if got := conflictPhaseOf(held.WorkspaceRecords, pr); got != ConflictRecordsUnreadable {
					t.Errorf("conflictPhaseOf = %s, want records_unreadable", conflictPhaseName(got))
				}
			})
		}
	})

	t.Run("snapshot stop priority, release, and worktree recreation", func(t *testing.T) {
		w := newWorld(t)
		tree := w.add(fakeWorkspace{
			ID: "w1", Label: "ws", Branch: taskBranch, Status: "idle", Screen: "claude-idle.ansi",
			PRs: prListEntryJSON(taskBranch, 50, "Conflict PR", headA.String(), "CONFLICTING", mainBranch, false),
		})

		// Initially no records directory -> ConflictUnsent.
		snap := take(t, w)
		row := rowOf(t, snap, "ws")
		if row.State != StateConflict || row.Conflict != ConflictUnsent || row.Urgency != UrgencyLater || row.Turn != TurnHuman || row.Position.Progress != ProgressConflict {
			t.Fatalf("initial row = %+v, want conflict/unsent/later/human", row)
		}

		// Write an unfinished /mergemain send: snapshot shows ConflictSending (wait/AI).
		records, err := w.store.Records(Worktree{path: tree})
		if err != nil {
			t.Fatal(err)
		}
		held, err := records.Lock()
		if err != nil {
			t.Fatal(err)
		}
		id, err := held.BeginSend(SendRecord{
			Pane: "old:p9", Input: InputCommand.String(), Command: "/mergemain 50",
			Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headA,
		}, w.now)
		if err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}

		snap = take(t, w)
		row = rowOf(t, snap, "ws")
		if row.Conflict != ConflictSending || row.Urgency != UrgencyWait || row.Turn != TurnAI {
			t.Fatalf("sending row = %+v, want sending/wait/AI", row)
		}

		// Simulate worktree recreation at the same path (ClearRecords) followed
		// by chief start (SettleUnfinished): stop wins (now/human), then release
		// transitions to outcome_unknown (later/human).
		held, err = records.Lock()
		if err != nil {
			t.Fatal(err)
		}
		if err := held.ClearRecords(); err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := w.store.Start(context.Background(), herdrWith(t, liveAgent(w.owner())), w.owner(), w.now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		snap = take(t, w)
		row = rowOf(t, snap, "ws")
		if row.Stopped == nil || row.Stopped.Send != id || row.Urgency != UrgencyNow || row.Turn != TurnHuman {
			t.Fatalf("stopped after recreate+start = %+v, want stopped with now/human", row)
		}

		held, err = records.Lock()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := held.Release(w.now.Add(2 * time.Minute)); err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
		snap = take(t, w)
		row = rowOf(t, snap, "ws")
		if row.Stopped != nil || row.Conflict != ConflictOutcomeUnknown || row.Urgency != UrgencyLater || row.Turn != TurnHuman {
			t.Fatalf("after release = %+v, want no stop and outcome_unknown/later/human", row)
		}

		// A completed /mergemain send transitions to ConflictUnresolved.
		held, err = records.Lock()
		if err != nil {
			t.Fatal(err)
		}
		id2, err := held.BeginSend(SendRecord{
			Pane: "w1:p1", Input: InputCommand.String(), Command: "/mergemain 50",
			Kind: paneinput.CommandMergemain.Name(), Arg: "50", PR: 50, Head: headA,
		}, w.now.Add(3*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if err := held.EndSend(id2, OutcomeSent.String(), "", StepCommand, w.now.Add(4*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
		snap = take(t, w)
		row = rowOf(t, snap, "ws")
		if row.Conflict != ConflictUnresolved || row.Urgency != UrgencyLater || row.Turn != TurnHuman || row.Position.Progress != ProgressConflictUnresolved {
			t.Fatalf("unresolved row = %+v, want unresolved/later/human/ProgressConflictUnresolved", row)
		}

		// Unreadable records directory keeps ConflictRecordsUnreadable.
		w2 := newWorld(t)
		tree2 := w2.add(fakeWorkspace{
			ID: "w2", Label: "bad", Branch: taskBranch, Status: "idle", Screen: "claude-idle.ansi",
			PRs: prListEntryJSON(taskBranch, 50, "Conflict PR", headA.String(), "CONFLICTING", mainBranch, false),
		})
		if err := os.WriteFile(filepath.Join(w2.store.Dir(), workspaceDirOf(Worktree{path: tree2})), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		snap2 := take(t, w2)
		row2 := rowOf(t, snap2, "bad")
		if row2.Conflict != ConflictRecordsUnreadable || !row2.StopUnreadable || row2.Urgency != UrgencyNow {
			t.Errorf("unreadable dir row = %+v, want records_unreadable + StopUnreadable + UrgencyNow", row2)
		}
	})
}
