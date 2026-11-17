//go:build test && unix

package chief

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// runprRan is /runpr 42, the command that ran in the /runpr pane.
var runprRan = ranCommand{kind: paneinput.CommandRunpr, argument: "42", route: paneinput.RouteRunpr}

// skipPane writes the skip record of p's current session and state in the
// merge world's workspace, as the reset of scene with cmd left it, and
// returns it.
func (mw *mergeWorld) skipPane(p *simPane, scene skipScene, cmd ranCommand) skipRecord {
	mw.t.Helper()
	pane, err := ParsePaneID(p.Pane)
	if err != nil {
		mw.t.Fatal(err)
	}
	a := Agent{Pane: pane, Workspace: p.Workspace, Kind: agentKinds[p.Kind], Session: p.Session, StateChangeSeq: int64(p.Seq)}
	rec, err := newSkipRecord(p.Workspace, a, scene, cmd, mw.now)
	if err != nil {
		mw.t.Fatal(err)
	}
	held := heldRecords(mw.t, mw.store, mw.tree)
	if err := held.recordSkip(rec); err != nil {
		mw.t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		mw.t.Fatal(err)
	}
	return rec
}

// skipOpenCode keeps the skip of the OpenCode /runpr pane w1:p2 after an
// automatic send.
func (mw *mergeWorld) skipOpenCode() skipRecord {
	mw.t.Helper()
	return mw.skipPane(mw.byPane["w1:p2"], sceneAfterAutoSend, runprRan)
}

// renamePane gives the OpenCode pane w1:p2 the pane ID id, as if it were
// closed and another opened, keeping p2 when keep is true (a second pane of
// the agent).
func (mw *mergeWorld) renamePane(id string, keep bool) {
	mw.t.Helper()
	p := mw.byPane["w1:p2"]
	if keep {
		copied := *p
		p = &copied
		mw.panes = append(mw.panes, p)
	}
	p.Pane = id
	mw.byPane[id] = p
	for _, src := range []string{"visible", "recent"} {
		mw.rec.replyFunc("herdr agent read "+id+" --source "+src+" --lines 200 --format ansi", func() reply {
			mw.mu.Lock()
			defer mw.mu.Unlock()
			return reply{out: []byte(p.screen)}
		})
	}
}

// snapshot takes a snapshot of the merge world.
func (mw *mergeWorld) snapshot() Snapshot {
	mw.t.Helper()
	snap, err := TakeSnapshot(context.Background(), mw.deps, mw.owner())
	if err != nil {
		mw.t.Fatal(err)
	}
	return snap
}

// resetItems returns the reset items of pending.
func resetItems(pending []PendingItem) []PendingItem {
	return slices.DeleteFunc(slices.Clone(pending), func(p PendingItem) bool { return p.Kind != PendingReset })
}

// TestResetItemOffered checks the reset item of a skip record: a snapshot
// and a tick each list it once, with its pane, agent, input, scene,
// command, and check value, numbered and announced like any item, after
// the row's other items of higher kinds; a running row offers it too; the
// same skip kept twice is one item; and chief restarted by another session
// lists it again with a number and a notice. No round removes the record
// or sends the reset.
func TestResetItemOffered(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	rec := mw.skipOpenCode()
	mw.skipOpenCode()
	snap := mw.snapshot()
	items := resetItems(snap.Pending)
	if len(items) != 1 {
		t.Fatalf("reset items = %+v; want one", items)
	}
	out := snap.Output()
	i := slices.IndexFunc(out.Pending, func(p PendingOutput) bool { return p.Kind == "reset" })
	want := ResetOutput{Pane: "w1:p2", Agent: "opencode", Input: "/new", Scene: "after_autosend", Command: "/runpr 42", Check: resetCheck(mw.tree, rec)}
	if got := out.Pending[i]; got.Reset == nil || *got.Reset != want || got.Number == 0 || !got.New || !slices.Contains(out.NewNotices, got.ID) {
		t.Errorf("reset item = %+v (reset %+v); want %+v, numbered and announced", got, got.Reset, want)
	}
	// The workspace's PR waits for review, a kind listed before it.
	if kinds := []PendingKind{snap.Pending[0].Kind, snap.Pending[len(snap.Pending)-1].Kind}; kinds[0] != PendingReview || kinds[1] != PendingReset {
		t.Errorf("pending kinds = %v; want the review first and the reset last", kinds)
	}
	if tick := resetItems(mw.tick(time.Minute).Snapshot.Pending); len(tick) != 1 || tick[0].ID != items[0].ID || tick[0].Number != items[0].Number {
		t.Errorf("tick's reset items = %+v; want the same item", tick)
	}
	mw.byPane["w1:p1"].Status = "working"
	if running := mw.snapshot(); rowOf(t, running, "ws").State != StateRunning || len(resetItems(running.Pending)) != 1 {
		t.Errorf("running row: state %s, items %+v; want the reset offered", rowOf(t, running, "ws").State, running.Pending)
	}
	// chief restarted from another session lists and announces it again.
	other := mw.chief
	other.Session = "00000000-0000-4000-8000-0000000000c9"
	mw.chief = other
	mw.rec.reply("herdr agent get "+worldChiefPane, agentGetOutput(other))
	mw.now = mw.now.Add(time.Hour)
	if _, err := mw.store.Start(context.Background(), NewHerdr(mw.deps.Cmds), mw.owner(), mw.now); err != nil {
		t.Fatal(err)
	}
	again := resetItems(mw.snapshot().Pending)
	if len(again) != 1 || again[0].Number == 0 || !again[0].New {
		t.Errorf("after the restart: %+v; want the item numbered and announced", again)
	}
	if len(skipsOf(t, mw.store, mw.tree)) != 1 {
		t.Error("a round removed the skip record")
	}
	if typed := mw.rec.prompts(); len(typed) != 0 {
		t.Errorf("a round typed %v", typed)
	}
}

// TestResetItemConditions checks that a skip record gives no item while one
// condition does not hold, and that no round removes it: the row needs a
// check or is stopped, the record names another Herdr workspace, the
// workspace has no single pane of the agent or its pane is another, the
// pane's state, session, or status moved, or the routing table routes the
// command to another agent.
func TestResetItemConditions(t *testing.T) {
	for _, c := range []struct {
		name string
		// claude keeps the skip of the Claude Code pane after /runpr
		// instead of the OpenCode pane's.
		claude bool
		change func(t *testing.T, mw *mergeWorld)
	}{
		{name: "the row needs a check", change: func(_ *testing.T, mw *mergeWorld) {
			mw.rec.failIn(mw.tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", errNoReply)
		}},
		{name: "the workspace is stopped", change: func(t *testing.T, mw *mergeWorld) {
			held := heldRecords(t, mw.store, mw.tree)
			if err := held.Stop("why", "", "", mw.now); err != nil {
				t.Fatal(err)
			}
			if err := held.Close(); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "another Herdr workspace", change: func(t *testing.T, mw *mergeWorld) {
			held := heldRecords(t, mw.store, mw.tree)
			found, err := held.skipRecords()
			if err != nil || len(found) != 1 {
				t.Fatalf("skip records = %+v, %v", found, err)
			}
			if err := os.Remove(found[0].path); err != nil {
				t.Fatal(err)
			}
			rec := found[0].rec
			rec.workspace = "w9"
			if err := held.recordSkip(rec); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a second pane of the agent", change: func(_ *testing.T, mw *mergeWorld) { mw.renamePane("w1:p3", true) }},
		{name: "the pane changed state", change: func(_ *testing.T, mw *mergeWorld) { mw.byPane["w1:p2"].Seq++ }},
		{name: "the pane works", change: func(_ *testing.T, mw *mergeWorld) { mw.byPane["w1:p2"].Status = "working" }},
		{name: "the routing table moved the command", change: func(t *testing.T, mw *mergeWorld) {
			mw.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", claudeRunprContext(t))
		}},
		{name: "the Claude Code pane has another session", claude: true, change: func(_ *testing.T, mw *mergeWorld) {
			mw.byPane["w1:p1"].Session = mw.newSession()
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			if c.claude {
				mw.skipPane(mw.byPane["w1:p1"], sceneAfterRunpr, mkarchRan)
			} else {
				mw.skipOpenCode()
			}
			if len(resetItems(mw.snapshot().Pending)) != 1 {
				t.Fatal("no reset item before the change")
			}
			c.change(t, mw)
			if items := resetItems(mw.snapshot().Pending); len(items) != 0 {
				t.Errorf("reset items = %+v; want none", items)
			}
			if recs := skipsOf(t, mw.store, mw.tree); len(recs) != 1 {
				t.Errorf("skip records = %+v; want the record kept", recs)
			}
		})
	}
}

// TestUnreadableSkipRecordIsAProblem checks that a skip record that cannot
// be read is named with its path in the round's problems, gives no item,
// and is left in place.
func TestUnreadableSkipRecordIsAProblem(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	records, err := mw.store.Records(Worktree{path: mw.tree})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(records.dir, skipPrefix+strings.Repeat("c", 64)+recordSuffix)
	if err := os.WriteFile(path, []byte(`{"pane":"w1:p2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snap := mw.snapshot()
	if !slices.ContainsFunc(snap.Problems, func(p string) bool { return strings.Contains(p, path) }) {
		t.Errorf("problems = %v; want the record's path", snap.Problems)
	}
	if len(resetItems(snap.Pending)) != 0 {
		t.Error("the unreadable record gave an item")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the record: %v; want it left", err)
	}
}

// offeredReset sends the reset item the snapshot offers with check.
func (mw *mergeWorld) offeredReset(check CheckValue) ActionResult {
	mw.t.Helper()
	req, err := ParseOfferedResetRequest(mw.tree, string(check))
	if err != nil {
		mw.t.Fatal(err)
	}
	out, err := SendOfferedReset(context.Background(), mw.deps, mw.owner(), req)
	if err != nil || len(out.Actions) != 1 {
		mw.t.Fatalf("SendOfferedReset = %+v, %v", out, err)
	}
	return out.Actions[0]
}

// shownCheck returns the check value of the one reset item a snapshot
// offers now.
func (mw *mergeWorld) shownCheck() CheckValue {
	mw.t.Helper()
	items := resetItems(mw.snapshot().Pending)
	if len(items) != 1 {
		mw.t.Fatalf("reset items = %+v; want one", items)
	}
	return items[0].Reset.Check
}

// TestSendOfferedReset checks that the check value a snapshot shows sends
// the item's reset once: /new to the OpenCode pane, /clear to a Claude Code
// pane whose last human input is the record's command (its name and
// argument after /runpr, though the documents now give another command,
// any argument for a /mkreq, and the name alone for /runpr); the send is
// recorded, the record is gone, and the next round offers no item.
func TestSendOfferedReset(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(mw *mergeWorld)
		typed string
	}{
		{name: "the OpenCode pane after an automatic send", setup: func(mw *mergeWorld) { mw.skipOpenCode() }, typed: "w1:p2 /new"},
		{name: "the Claude Code pane after /runpr", setup: func(mw *mergeWorld) {
			// The documents now give /mkplan, as after the PR's merge.
			mw.docs(mw.tree, fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved})
			mw.claudeTranscript(commandLine("/mkarch 0001"))
			mw.skipPane(mw.byPane["w1:p1"], sceneAfterRunpr, mkarchRan)
		}, typed: "w1:p1 /clear"},
		{name: "the Claude Code pane after /mkreq", setup: func(mw *mergeWorld) {
			mw.claudeTranscript(commandLine("/mkreq 12"))
			mw.skipPane(mw.byPane["w1:p1"], sceneAfterRunpr, ranCommand{kind: paneinput.CommandMkreq, route: paneinput.RouteTaskDocuments})
		}, typed: "w1:p1 /clear"},
		{name: "the Claude Code /runpr pane called with a branch", setup: func(mw *mergeWorld) {
			mw.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", claudeRunprContext(mw.t))
			mw.claudeTranscript(commandLine("/runpr user/0001-test-task-02"))
			mw.skipPane(mw.byPane["w1:p1"], sceneAfterAutoSend, runprRan)
		}, typed: "w1:p1 /clear"},
	} {
		t.Run(c.name, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			c.setup(mw)
			got := mw.offeredReset(mw.shownCheck())
			if got.Outcome != OutcomeSent || got.Input != InputReset || got.Pane+" "+got.Command != c.typed {
				t.Fatalf("reset = %+v; want %s sent", got, c.typed)
			}
			if typed := mw.rec.prompts(); !slices.Equal(typed, []string{c.typed}) {
				t.Errorf("typed %v, want only %s", typed, c.typed)
			}
			records, err := mw.store.Records(Worktree{path: mw.tree})
			if err != nil {
				t.Fatal(err)
			}
			last, err := records.LastSend(got.Pane)
			if err != nil || last == nil || last.Input != InputReset.String() {
				t.Errorf("last send = %+v, %v; want the reset recorded", last, err)
			}
			if recs := skipsOf(t, mw.store, mw.tree); len(recs) != 0 {
				t.Errorf("skip records = %+v; want the used one gone", recs)
			}
			if items := resetItems(mw.snapshot().Pending); len(items) != 0 {
				t.Errorf("reset items after the send = %+v; want none", items)
			}
		})
	}
}

// TestSendOfferedResetRefusals checks that a chosen reset types nothing
// when a check fails, with the reason: the item changed or is gone
// (check_mismatch), the routing table cannot be read, the transcript does
// not show the record's command, or the pane's check before the input
// fails. Each leaves the record, so the item is offered again while its
// conditions hold.
func TestSendOfferedResetRefusals(t *testing.T) {
	routeRunprToClaude := func(mw *mergeWorld) {
		mw.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", claudeRunprContext(mw.t))
	}
	for _, c := range []struct {
		name string
		// claude keeps the skip of the Claude Code pane after /runpr,
		// whose transcript ends with /mkarch 0001.
		claude bool
		// change runs after the check value is shown.
		change func(mw *mergeWorld)
		reason NotSentReason
	}{
		{name: "the pane changed state", change: func(mw *mergeWorld) { mw.byPane["w1:p2"].Seq++ }, reason: NotSentCheckMismatch},
		{name: "the row needs a check", change: func(mw *mergeWorld) {
			mw.rec.failIn(mw.tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", errNoReply)
		}, reason: NotSentCheckMismatch},
		{name: "the pane is another", change: func(mw *mergeWorld) { mw.renamePane("w1:p3", false) }, reason: NotSentCheckMismatch},
		{name: "the session changed", claude: true, change: func(mw *mergeWorld) { mw.byPane["w1:p1"].Session = mw.newSession() }, reason: NotSentCheckMismatch},
		{name: "the routing table moved the command", change: routeRunprToClaude, reason: NotSentCheckMismatch},
		{name: "the routing table cannot be read", change: func(mw *mergeWorld) {
			mw.rec.fail("git show refs/remotes/origin/main:.claude/commands/_context.md", errNoReply)
		}, reason: NotSentUnreadable},
		{name: "text typed after the command", claude: true, change: func(mw *mergeWorld) {
			p := mw.byPane["w1:p1"]
			if err := mw.writeTranscript(p, p.Session, typedLine("one more thing")); err != nil {
				mw.t.Fatal(err)
			}
		}, reason: NotSentOtherInput},
		{name: "the transcript is gone", claude: true, change: (*mergeWorld).removeClaudeTranscript, reason: NotSentNoTranscript},
		{name: "unsent text in the input line", claude: true, change: func(mw *mergeWorld) {
			mw.byPane["w1:p1"].screen = withInput(mw.t, readScreenSample(mw.t, "claude-after-clear.ansi"), "draft")
		}, reason: NotSentInputLeft},
		{name: "the pane moves before the input", change: func(mw *mergeWorld) {
			// The workspace's list is read under the exclusion; the pane
			// moves before the check that precedes the input.
			p := mw.byPane["w1:p2"]
			mw.rec.replyFunc("herdr agent get w1:p2", func() reply {
				mw.mu.Lock()
				defer mw.mu.Unlock()
				p.Seq++
				return reply{out: agentGetOutput(p.fakeAgent)}
			})
		}, reason: NotSentPaneChanged},
	} {
		t.Run(c.name, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			if c.claude {
				mw.claudeTranscript(commandLine("/mkarch 0001"))
				mw.skipPane(mw.byPane["w1:p1"], sceneAfterRunpr, mkarchRan)
			} else {
				mw.skipOpenCode()
			}
			check := mw.shownCheck()
			c.change(mw)
			got := mw.offeredReset(check)
			if got.Outcome != OutcomeNotSent || got.Reason != c.reason {
				t.Errorf("reset = %+v; want not sent, %s", got, c.reason)
			}
			if typed := mw.rec.prompts(); len(typed) != 0 {
				t.Errorf("typed %v; want nothing", typed)
			}
			if recs := skipsOf(t, mw.store, mw.tree); len(recs) != 1 {
				t.Errorf("skip records = %+v; want the record kept", recs)
			}
		})
	}
}

// TestSendOfferedResetUseAndRecords checks the use of the item before the
// input: a record that cannot be used up, or chief's owner changed by
// then, types nothing; an item used whose send record then cannot be
// written types nothing and is not offered again; and a failure after the
// input stops the workspace.
func TestSendOfferedResetUseAndRecords(t *testing.T) {
	// beforeInput runs f on the check of the pane before the input, the
	// second read of the pane's agent.
	beforeInput := func(mw *mergeWorld, f func()) {
		p, reads := mw.byPane["w1:p2"], 0
		mw.rec.replyFunc("herdr agent get w1:p2", func() reply {
			reads++
			if reads == 1 {
				f()
			}
			mw.mu.Lock()
			defer mw.mu.Unlock()
			return reply{out: agentGetOutput(p.fakeAgent)}
		})
	}
	t.Run("the record is gone before the input", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		rec := mw.skipOpenCode()
		check := mw.shownCheck()
		records, err := mw.store.Records(Worktree{path: mw.tree})
		if err != nil {
			t.Fatal(err)
		}
		beforeInput(mw, func() {
			if err := os.Remove(filepath.Join(records.dir, rec.name())); err != nil {
				t.Error(err)
			}
		})
		if got := mw.offeredReset(check); got.Outcome != OutcomeNotSent || got.Reason != NotSentUnreadable || len(mw.rec.prompts()) != 0 {
			t.Errorf("reset = %+v, typed %v; want nothing typed, unreadable", got, mw.rec.prompts())
		}
	})
	t.Run("chief's owner changed before the input", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.skipOpenCode()
		check := mw.shownCheck()
		owner := mw.owner()
		beforeInput(mw, func() {
			other := mw.chief
			other.Session = "00000000-0000-4000-8000-0000000000c9"
			mw.rec.reply("herdr agent get "+worldChiefPane, agentGetOutput(other))
		})
		req, err := ParseOfferedResetRequest(mw.tree, string(check))
		if err != nil {
			t.Fatal(err)
		}
		out, err := SendOfferedReset(context.Background(), mw.deps, owner, req)
		if err != nil || len(out.Actions) != 1 || out.Actions[0].Reason != NotSentConditions || len(mw.rec.prompts()) != 0 {
			t.Errorf("reset = %+v, %v, typed %v; want nothing typed, conditions", out, err, mw.rec.prompts())
		}
		if recs := skipsOf(t, mw.store, mw.tree); len(recs) != 1 {
			t.Errorf("skip records = %+v; want the record kept", recs)
		}
	})
	t.Run("the send record cannot be written", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.skipOpenCode()
		check := mw.shownCheck()
		mw.deps.Store.beforeCreate = failCreates(sendPrefix)
		got := mw.offeredReset(check)
		mw.deps.Store.beforeCreate = nil
		if got.Outcome != OutcomeNotSent || got.Reason != NotSentUnreadable || len(mw.rec.prompts()) != 0 {
			t.Errorf("reset = %+v, typed %v; want nothing typed, unreadable", got, mw.rec.prompts())
		}
		if items := resetItems(mw.snapshot().Pending); len(items) != 0 {
			t.Errorf("reset items = %+v; want the used item gone", items)
		}
	})
	t.Run("the reset fails after the input", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.skipOpenCode()
		check := mw.shownCheck()
		mw.react = func(*simPane, string) error { return nil }
		got := mw.offeredReset(check)
		if got.Outcome != OutcomeSentThenFailed || got.Step != StepNew {
			t.Errorf("reset = %+v; want sent then failed after new", got)
		}
		records, err := mw.store.Records(Worktree{path: mw.tree})
		if err != nil {
			t.Fatal(err)
		}
		if stop, _ := records.Stopped(); stop == nil {
			t.Error("the workspace was not stopped")
		}
		if recs := skipsOf(t, mw.store, mw.tree); len(recs) != 0 {
			t.Errorf("skip records = %+v; want the used one gone", recs)
		}
	})
}

// TestSendRemovesThePaneSkips checks that a command typed into a pane
// removes that pane's skip records and leaves another pane's.
func TestSendRemovesThePaneSkips(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.skipOpenCode()
	kept := mw.skipPane(mw.byPane["w1:p1"], sceneAfterRunpr, mkarchRan)
	mw.rec.replyIn(mw.tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", nil))
	mw.rec.replyIn(mw.tree, "gh pr checks 42 --json "+prChecksFields, readTestdata(t, "gh/pr-checks-200-fail.json"))
	req, err := ParseSendRequest(mw.tree, "runpr", "", "42")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Send(context.Background(), mw.deps, req)
	if err != nil || len(out.Actions) == 0 || out.Actions[0].Outcome != OutcomeSent || out.Actions[0].Pane != "w1:p2" {
		t.Fatalf("Send = %+v, %v; want /runpr sent to w1:p2", out, err)
	}
	if recs := skipsOf(t, mw.store, mw.tree); len(recs) != 1 || recs[0] != kept {
		t.Errorf("skip records = %+v; want only w1:p1's", recs)
	}
}

// TestChosenResetMakesTheNextSendAProposal checks that a reset the human
// chooses after a round saw the PR merged counts as a send since then, so
// the next command is proposed instead of sent automatically.
func TestChosenResetMakesTheNextSendAProposal(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	mw.byPane["w1:p1"].Status = "working"
	if res := only(t, mw.tick(time.Minute)); sentCommand(res) {
		t.Fatalf("the tick of a working pane sent %+v", res)
	}
	mw.skipOpenCode()
	if got := mw.offeredReset(mw.shownCheck()); got.Outcome != OutcomeSent {
		t.Fatalf("reset = %+v; want sent", got)
	}
	mw.byPane["w1:p1"].Status = "idle"
	res := only(t, mw.tick(time.Minute))
	if sentCommand(res) || len(res.Reasons) == 0 {
		t.Errorf("auto send = %+v; want the command proposed with its reasons", res)
	}
}

// TestParseOfferedResetRequest checks that a relative workspace and a check
// value of another shape are malformed, and a valid pair is accepted.
func TestParseOfferedResetRequest(t *testing.T) {
	check := strings.Repeat("a", 64)
	if _, err := ParseOfferedResetRequest("/w", check); err != nil {
		t.Errorf("a valid request: %v", err)
	}
	for _, c := range [][2]string{{"w", check}, {"/w", "abc"}, {"/w", strings.Repeat("A", 64)}} {
		if _, err := ParseOfferedResetRequest(c[0], c[1]); !errors.Is(err, ErrBadRequest) {
			t.Errorf("%q: %v, want ErrBadRequest", c, err)
		}
	}
}

// TestAutoSendDropsTheSentPaneReset checks that the tick that sends a
// command to a pane no longer offers that pane's reset, which the round
// read before the send.
func TestAutoSendDropsTheSentPaneReset(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	mw.skipPane(mw.byPane["w1:p1"], sceneAfterRunpr, mkarchRan)
	out := mw.tick(time.Minute)
	if res := only(t, out); !sentCommand(res) || res.Actions[0].Pane != "w1:p1" {
		t.Fatalf("auto send = %+v; want the command sent to w1:p1", res)
	}
	if items := resetItems(out.Snapshot.Pending); len(items) != 0 {
		t.Errorf("reset items = %+v; want none for the pane sent to", items)
	}
}
