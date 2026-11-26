//go:build test

package chief

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// conflictItemOf returns the conflict item of workspace in the snapshot.
func conflictItemOf(t *testing.T, snap Snapshot, workspace string) PendingItem {
	t.Helper()
	for _, p := range snap.Pending {
		if p.Kind == PendingConflict && p.Workspace == workspace {
			return p
		}
	}
	t.Fatalf("no conflict item for %s in %+v", workspace, snap.Pending)
	return PendingItem{}
}

func conflictSnapshot(t *testing.T, cw *conflictWorld) Snapshot {
	t.Helper()
	snap, err := TakeSnapshot(context.Background(), cw.deps, cw.owner())
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func mergemainReq(t *testing.T, cw *conflictWorld, item ItemID) SendRequest {
	t.Helper()
	return sendReq(t, cw.tree, "mergemain", "", "", string(item))
}

// TestConflictItemOffer checks the item a conflict row gets, once or in a
// tick: it offers the send only when every condition of the human's choice
// holds, says why it does not otherwise, marks a conversation the send would
// clear, never offers /runpr, and is the same item while the phase moves
// within "needed" and another item for another head or an unresolved one.
func TestConflictItemOffer(t *testing.T) {
	t.Run("offers the send", func(t *testing.T) {
		cw := newConflictWorld(t)
		snap := conflictSnapshot(t, cw)
		it := conflictItemOf(t, snap, cw.tree)
		if it.Conflict == nil || !it.Conflict.Offer.CanSend || len(it.Conflict.Offer.Reasons) != 0 || it.Conflict.Offer.ConversationLost || it.Conflict.Phase != ConflictUnsent {
			t.Errorf("item = %+v, want an offer that can send", it.Conflict)
		}
		for _, p := range snap.Pending {
			if p.Kind == PendingReview || (p.Proposal != nil && p.Proposal.Kind == TaskNextRunpr) {
				t.Errorf("a conflict row offers %+v", p)
			}
		}
		out := snap.Output().Pending
		if i := slices.IndexFunc(out, func(o PendingOutput) bool { return o.Kind == "conflict" }); i < 0 || out[i].Conflict == nil || !out[i].Conflict.CanSend || out[i].Conflict.Phase != "unsent" {
			t.Errorf("output = %+v", out)
		}
	})

	t.Run("says why not", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			setup  func(cw *conflictWorld)
			reason string
		}{
			{"dirty worktree", func(cw *conflictWorld) {
				cw.git(cw.tree, "status --porcelain --untracked-files=all", " M file\n")
			}, conflictReasonDirty},
			{"head differs from the PR's", func(cw *conflictWorld) {
				cw.git(cw.tree, "rev-parse HEAD", conflictTestHeadB+"\n")
			}, conflictReasonHeadMismatch},
			{"definition differs", func(cw *conflictWorld) {
				cw.git(cw.tree, "show HEAD:"+mergemainDefPath, "# another\n")
			}, conflictReasonDefinitionDiffers},
			{"agent working", func(cw *conflictWorld) {
				cw.firstPane().Status = "working"
			}, ""},
			{"OpenCode route", func(cw *conflictWorld) {
				orig := string(repoContext(t))
				row := repositoryRow(t, "mergemain")
				modified := strings.Replace(orig, row, rowWithCell(t, rowWithCell(t, row, 2, "OpenCode"), 3, "`DeepSeek V4.1 Flash 1017 OpenCode Go`"), 1)
				cw.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", []byte(modified))
			}, conflictReasonRouteNotClaude},
		} {
			t.Run(tc.name, func(t *testing.T) {
				cw := newConflictWorld(t)
				tc.setup(cw)
				snap := conflictSnapshot(t, cw)
				if tc.reason == "" {
					// A working pane makes the row a running one: no conflict item.
					for _, p := range snap.Pending {
						if p.Kind == PendingConflict {
							t.Fatalf("a running row has the conflict item %+v", p)
						}
					}
					return
				}
				it := conflictItemOf(t, snap, cw.tree)
				if it.Conflict.Offer.CanSend || !slices.Contains(it.Conflict.Offer.Reasons, tc.reason) {
					t.Errorf("offer = %+v, want no send with reason %q", it.Conflict.Offer, tc.reason)
				}
			})
		}
	})

	t.Run("marks a conversation the send would clear", func(t *testing.T) {
		cw := newConflictWorld(t)
		cw.setTranscript(cw.firstPane(), commandLine("/runplan 0001"), typedLine("a question"))
		it := conflictItemOf(t, conflictSnapshot(t, cw), cw.tree)
		if !it.Conflict.Offer.CanSend || !it.Conflict.Offer.ConversationLost {
			t.Errorf("offer = %+v, want a send that clears the conversation", it.Conflict.Offer)
		}
	})

	t.Run("identity", func(t *testing.T) {
		pr := OpenPR{Number: 50, Head: mustCommitID(t, conflictTestHeadA)}
		other := OpenPR{Number: 50, Head: mustCommitID(t, conflictTestHeadB)}
		needed := conflictItemID("/w", pr, ConflictUnsent)
		for _, p := range []ConflictPhase{ConflictRecordsUnreadable, ConflictEndedBeforeCommand, ConflictOutcomeUnknown} {
			if got := conflictItemID("/w", pr, p); got != needed {
				t.Errorf("phase %d changes the item", p)
			}
		}
		for name, id := range map[string]ItemID{
			"unresolved":   conflictItemID("/w", pr, ConflictUnresolved),
			"another head": conflictItemID("/w", other, ConflictUnsent),
			"another tree": conflictItemID("/v", pr, ConflictUnsent),
		} {
			if id == needed {
				t.Errorf("%s keeps the item", name)
			}
		}
		if conflictOfferPhase(ConflictSending) {
			t.Error("a sending row gets an item")
		}
	})
}

// TestSendMergemain checks the send of the conflict item the human chose:
// the right item sends /mergemain with the usual three inputs and a record
// of the PR and head, and a wrong or stale item, a condition that no longer
// holds, an unreadable fetch, or another kind of row sends nothing.
func TestSendMergemain(t *testing.T) {
	t.Run("sends", func(t *testing.T) {
		cw := newConflictWorld(t)
		item := conflictItemOf(t, conflictSnapshot(t, cw), cw.tree)
		// The human's input after the task command is no condition.
		cw.setTranscript(cw.firstPane(), commandLine("/runplan 0001"), typedLine("wait"))
		out, err := Send(context.Background(), cw.deps, cw.owner(), mergemainReq(t, cw, item.ID))
		if err != nil || len(out.Actions) != 1 || out.Actions[0].Outcome != OutcomeSent {
			t.Fatalf("Send = %+v, %v; want sent", out, err)
		}
		if got := cw.rec.prompts(); len(got) == 0 || got[len(got)-1] != "w1:p1 /mergemain 50" {
			t.Errorf("prompts = %q, want /mergemain 50 last", got)
		}
		records, ok, err := cw.store.existingRecords(Worktree{path: cw.tree})
		if err != nil || !ok {
			t.Fatalf("records = %v, %v", ok, err)
		}
		if phase := conflictPhaseOf(records, OpenPR{Number: 50, Head: mustCommitID(t, conflictTestHeadA)}); phase == ConflictUnsent {
			t.Error("the send left no record of the PR and head")
		}
	})

	t.Run("keeps the pane's report before sending", func(t *testing.T) {
		cw := newConflictWorld(t)
		item := conflictItemOf(t, conflictSnapshot(t, cw), cw.tree)
		cw.appendTranscript(cw.firstPane(), prURLRecordLine(50, "https://github.com/o/r/pull/50"), answerLine("msg_report", makerReport))
		if out, err := Send(context.Background(), cw.deps, cw.owner(), mergemainReq(t, cw, item.ID)); err != nil || out.Actions[0].Outcome != OutcomeSent {
			t.Fatalf("Send = %+v, %v", out, err)
		}
		records, err := cw.deps.Store.Records(Worktree{path: cw.tree})
		if err != nil {
			t.Fatal(err)
		}
		if rec, ok, err := records.keptPaneReport(50, conflictTestHeadA); err != nil || !ok || rec.Text != makerReport {
			t.Errorf("kept report = %+v, %v, %v; want the pane's report", rec, ok, err)
		}
	})

	notSent := func(t *testing.T, name string, reason NotSentReason, setup func(cw *conflictWorld, item ItemID) ItemID) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			cw := newConflictWorld(t)
			item := conflictItemOf(t, conflictSnapshot(t, cw), cw.tree)
			use := setup(cw, item.ID)
			out, err := Send(context.Background(), cw.deps, cw.owner(), mergemainReq(t, cw, use))
			if err != nil || len(out.Actions) != 1 || out.Actions[0].Outcome != OutcomeNotSent || out.Actions[0].Reason != reason {
				t.Fatalf("Send = %+v, %v; want not sent (%s)", out, err, reason)
			}
			assertNoPrompts(t, cw.sendWorld)
		})
	}
	notSent(t, "another item", NotSentNotOffered, func(_ *conflictWorld, _ ItemID) ItemID { return anyItem })
	notSent(t, "a new head", NotSentNotOffered, func(cw *conflictWorld, _ ItemID) ItemID {
		cw.rec.replyIn(cw.tree, "gh pr list --head "+taskBranch+" --state open --json "+prListFields,
			[]byte(prListEntryJSON(taskBranch, 50, "Conflict PR", conflictTestHeadB, "CONFLICTING", mainBranch, false)))
		return conflictItemID(cw.tree, OpenPR{Number: 50, Head: mustCommitID(t, conflictTestHeadA)}, ConflictUnsent)
	})
	notSent(t, "no longer conflicting", NotSentNotOffered, func(cw *conflictWorld, item ItemID) ItemID {
		cw.rec.replyIn(cw.tree, "gh pr list --head "+taskBranch+" --state open --json "+prListFields,
			[]byte(prListEntryJSON(taskBranch, 50, "Conflict PR", conflictTestHeadA, "MERGEABLE", mainBranch, false)))
		return item
	})
	notSent(t, "dirty worktree", NotSentConditions, func(cw *conflictWorld, item ItemID) ItemID {
		cw.git(cw.tree, "status --porcelain --untracked-files=all", " M file\n")
		return item
	})
	notSent(t, "main cannot be fetched", NotSentConditions, func(cw *conflictWorld, item ItemID) ItemID {
		cw.rec.failIn(cw.chiefTree, "git fetch origin main", errors.New("offline"))
		return item
	})
	notSent(t, "remote main differs", NotSentConditions, func(cw *conflictWorld, item ItemID) ItemID {
		cw.git(cw.chiefTree, "ls-remote origin "+remoteMainRef, conflictTestHeadB+"\t"+remoteMainRef+"\n")
		return item
	})
	notSent(t, "definition differs", NotSentConditions, func(cw *conflictWorld, item ItemID) ItemID {
		cw.git(cw.tree, "show HEAD:"+mergemainDefPath, "# another\n")
		return item
	})
	notSent(t, "OpenCode route", NotSentConditions, func(cw *conflictWorld, item ItemID) ItemID {
		orig := string(repoContext(t))
		row := repositoryRow(t, "mergemain")
		modified := strings.Replace(orig, row, rowWithCell(t, rowWithCell(t, row, 2, "OpenCode"), 3, "`DeepSeek V4.1 Flash 1017 OpenCode Go`"), 1)
		cw.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", []byte(modified))
		return item
	})
}

// TestSendMergemainArguments checks the arguments of a mergemain request:
// --item is required, and --pr and --issue are refused.
func TestSendMergemainArguments(t *testing.T) {
	for _, c := range []struct{ issue, pr, item string }{
		{"", "", ""},
		{"", "42", anyItem},
		{"12", "", anyItem},
		{"", "", anyItem[:31]},
	} {
		if _, err := ParseSendRequest("/w", "mergemain", c.issue, c.pr, c.item); !errors.Is(err, ErrBadRequest) {
			t.Errorf("ParseSendRequest(mergemain, %+v) = %v, want ErrBadRequest", c, err)
		}
	}
	req, err := ParseSendRequest("/w", "mergemain", "", "", anyItem)
	if err != nil || req.kind != RequestMergemain {
		t.Errorf("ParseSendRequest(mergemain, item) = %+v, %v", req, err)
	}
}

// TestDetailInheritsThePaneReport checks the parent walk of `chief detail`: a
// head that is a merge of an earlier head with main shows the earlier head's
// pane report, and every other shape of head, a failed read or fetch, and
// the hop limit show the description as before.
func TestDetailInheritsThePaneReport(t *testing.T) {
	const (
		parent = "3333333333333333333333333333333333333333"
		merged = "4444444444444444444444444444444444444444"
		side   = "5555555555555555555555555555555555555555"
	)
	notAncestor := &cmdrunner.ExitError{Code: 1}
	keepParentReport := func(t *testing.T, deps Deps, tree string) {
		t.Helper()
		records, err := deps.Store.Records(Worktree{path: tree})
		if err != nil {
			t.Fatal(err)
		}
		rec := detailRecord{PR: 42, Head: parent, Source: ReportFromPane.String(), Text: makerReport, Pane: "w1:p1"}
		if err := records.store().create(detailName(42, parent), rec); err != nil {
			t.Fatal(err)
		}
	}
	setup := func(t *testing.T) (*sendWorld, string) {
		sw, tree := detailWorld(t)
		sw.git(tree, "rev-list --parents -n 1 "+prHead, prHead+" "+parent+" "+side+"\n")
		sw.git(tree, "merge-base --is-ancestor "+side+" "+mainHead, "")
		return sw, tree
	}
	t.Run("a merge of a head with main", func(t *testing.T) {
		sw, tree := setup(t)
		deps := sw.finish()
		keepParentReport(t, deps, tree)
		d := detailOf(t, deps, tree)
		if d.Report != ReportFromPane || d.ReportText != makerReport || d.DescriptionReason != DescriptionReasonNone {
			t.Errorf("detail = %v %q %v, want the parent's pane report", d.Report, d.ReportText, d.DescriptionReason)
		}
		if d.record == nil || d.record.Source == ReportFromPane.String() || d.record.Head != prHead {
			t.Errorf("record = %+v, want the description record of the head, not the inherited report", d.record)
		}
		records, _ := deps.Store.Records(Worktree{path: tree})
		if _, ok, err := records.keptPaneReport(42, prHead); err != nil || !ok {
			t.Fatal(ok, err)
		}
		if kept, _, _ := records.keptPaneReport(42, prHead); kept.Source == ReportFromPane.String() {
			t.Error("the inherited report was written as the head's record")
		}
	})
	t.Run("not inherited", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			setup func(sw *sendWorld, tree string)
		}{
			{"one parent", func(sw *sendWorld, tree string) {
				sw.git(tree, "rev-list --parents -n 1 "+prHead, prHead+" "+parent+"\n")
			}},
			{"the other parent is not in main", func(sw *sendWorld, tree string) {
				sw.rec.failIn(tree, "git merge-base --is-ancestor "+side+" "+mainHead, notAncestor)
			}},
			{"parents cannot be read", func(sw *sendWorld, tree string) {
				sw.rec.failIn(tree, "git rev-list --parents -n 1 "+prHead, errors.New("broken"))
			}},
			{"the first parent has no report and is no merge", func(sw *sendWorld, tree string) {
				const bare = "6666666666666666666666666666666666666666"
				sw.git(tree, "rev-list --parents -n 1 "+prHead, prHead+" "+bare+" "+side+"\n")
				sw.git(tree, "rev-list --parents -n 1 "+bare, bare+"\n")
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				sw, tree := setup(t)
				deps := sw.finish()
				keepParentReport(t, deps, tree)
				tc.setup(sw, tree)
				if d := detailOf(t, deps, tree); d.Report == ReportFromPane {
					t.Errorf("detail shows the pane report %q", d.ReportText)
				}
			})
		}
	})
	t.Run("a head the worktree lacks is fetched, only its own branch", func(t *testing.T) {
		sw, tree := setup(t)
		has := false
		sw.rec.replyFunc("git rev-parse --verify --quiet "+prHead+"^{commit}", func() reply {
			if !has {
				return reply{err: &cmdrunner.ExitError{Code: 1}}
			}
			return reply{out: []byte(prHead + "\n")}
		})
		fetch := "git fetch --no-tags --no-prune origin +refs/heads/" + taskBranch + ":refs/remotes/origin/" + taskBranch
		sw.rec.replyFunc(fetch, func() reply { has = true; return reply{} })
		deps := sw.finish()
		keepParentReport(t, deps, tree)
		if d := detailOf(t, deps, tree); d.Report != ReportFromPane {
			t.Errorf("report = %v after the fetch, want the pane's", d.Report)
		}
		if !has {
			t.Error("the missing head was not fetched")
		}
	})
	t.Run("a failed fetch inherits nothing", func(t *testing.T) {
		sw, tree := setup(t)
		sw.rec.replyIn(tree, "git rev-parse --verify --quiet "+prHead+"^{commit}", nil)
		sw.rec.failIn(tree, "git rev-parse --verify --quiet "+prHead+"^{commit}", &cmdrunner.ExitError{Code: 1})
		sw.rec.failIn(tree, "git fetch --no-tags --no-prune origin +refs/heads/"+taskBranch+":refs/remotes/origin/"+taskBranch, errors.New("offline"))
		deps := sw.finish()
		keepParentReport(t, deps, tree)
		if d := detailOf(t, deps, tree); d.Report == ReportFromPane {
			t.Error("a report was inherited without the head")
		}
	})
	t.Run("the hop limit", func(t *testing.T) {
		sw, tree := setup(t)
		deps := sw.finish()
		keepParentReport(t, deps, tree)
		// Every commit is a merge of the next one and the report is kept for
		// the last of them, one hop beyond the limit.
		chain := make([]string, maxDetailMergeHops+3)
		chain[0] = prHead
		for i := 1; i < len(chain); i++ {
			chain[i] = fmt.Sprintf("%040x", 0x7000+i)
		}
		for i := range len(chain) - 1 {
			sw.git(tree, "rev-list --parents -n 1 "+chain[i], chain[i]+" "+chain[i+1]+" "+side+"\n")
		}
		records, err := deps.Store.Records(Worktree{path: tree})
		if err != nil {
			t.Fatal(err)
		}
		last := chain[maxDetailMergeHops+1]
		if err := records.store().create(detailName(42, last), detailRecord{PR: 42, Head: last, Source: ReportFromPane.String(), Text: makerReport}); err != nil {
			t.Fatal(err)
		}
		if d := detailOf(t, deps, tree); d.Report == ReportFromPane {
			t.Error("the walk went beyond the hop limit")
		}
	})
	t.Run("not before /runpr", func(t *testing.T) {
		sw, tree := setup(t)
		deps := sw.finish()
		keepParentReport(t, deps, tree)
		docs := ""
		loc, err := locate(context.Background(), deps, tree)
		if err != nil || !loc.found {
			t.Fatal(err)
		}
		pr := OpenPR{Number: 42, URL: prURL, Head: mustCommitID(t, prHead)}
		d := CollectDetail(context.Background(), deps, loc.ws, pr, docs, Routing{}, errors.New("unused"), ReportRecordNone)
		if d.Report == ReportFromPane {
			t.Error("a detail kept for /runpr inherited a report")
		}
	})
}
