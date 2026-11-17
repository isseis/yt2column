//go:build test && unix

package chief

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
)

// withInput returns a Claude Code screen with text typed into its empty
// input line.
func withInput(t *testing.T, screen, text string) string {
	t.Helper()
	lines := strings.Split(screen, "\n")
	for i, l := range lines {
		if strings.TrimSuffix(l, "\r") == claudePromptMark {
			lines[i] = strings.Replace(l, claudePromptMark, claudePromptMark+text, 1)
			return strings.Join(lines, "\n")
		}
	}
	t.Fatal("the screen has no empty input line")
	return ""
}

// sendRunpr sends /runpr 42 to the workspace at tree.
func sendRunpr(t *testing.T, deps Deps, tree string) SendOutcome {
	t.Helper()
	req, err := ParseSendRequest(tree, "runpr", "", "42")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Send(context.Background(), deps, req)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestResetAfterRunpr checks the reset after /runpr: the command that created
// the PR is the next command origin/main's documents give while the PR is
// open, and its pane is emptied only when it is another pane than /runpr's,
// its report was kept by the detail, its state has not changed, it still
// shows the report, and the last line the human typed into its session is
// that command, its name and argument whole; otherwise it is left as it is,
// with the reason.
func TestResetAfterRunpr(t *testing.T) {
	cases := []struct {
		name string
		// plan is the plan sample origin/main holds, or "none" for no task
		// documents at all; empty keeps detailWorld's requirements.
		plan string
		// ran is the command the pane last ran, when not detailWorld's.
		ran string
		// typed is text the human typed after it.
		typed string
		// queued adds the sampled message the human typed while the agent
		// worked, after the command.
		queued   bool
		noReport bool
		react    func(sw *sendWorld, p *simPane, text string) error
		reason   NotSentReason
		// offered is true when the skip is kept as a reset item: no report
		// was found on the Claude Code pane that ran the command.
		offered bool
	}{
		{name: "a task-document PR's pane"},
		{name: "a /mkreq PR's pane", plan: "none", ran: "/mkreq 12"},
		{name: "a plan PR made on Claude Code", plan: "marker-opus.md", ran: "/runplan 0001"},
		{name: "another document command of the task", ran: "/mkplan 0001", reason: NotSentOtherInput},
		{name: "a plan PR whose pane last ran another command", plan: "marker-opus.md", reason: NotSentOtherInput},
		{name: "text typed after the command", typed: "what is the state of PR 42?", reason: NotSentOtherInput},
		{name: "text typed while the command worked", queued: true, reason: NotSentOtherInput},
		{name: "the command of another task", ran: "/mkarch 0002", reason: NotSentOtherInput},
		{name: "/mkreq without an issue", plan: "none", ran: "/mkreq", reason: NotSentOtherInput},
		{name: "a plan PR made on OpenCode, the pane /runpr went to", plan: "marker-sonnet.md", reason: NotSentSamePane},
		{name: "a plan PR whose requirement has no route", plan: "marker-unreadable.md", reason: NotSentUnreadable},
		{name: "no report on the pane", noReport: true, reason: NotSentNoReport, offered: true},
		{name: "the pane changed after the detail", react: func(sw *sendWorld, p *simPane, text string) error {
			sw.byPane["w1:p1"].Seq++
			return sw.realReact(p, text)
		}, reason: NotSentPaneChanged},
		{name: "the pane changes after its report was read", react: func(sw *sendWorld, p *simPane, text string) error {
			// The reset reads the pane's agent twice (for the report, then
			// before /clear); its state changes between the two.
			if text == "/runpr 42" {
				pane, reads := sw.byPane["w1:p1"], 0
				sw.rec.replyFunc("herdr agent get w1:p1", func() reply {
					sw.mu.Lock()
					defer sw.mu.Unlock()
					reads++
					if reads == 2 {
						pane.Seq++
					}
					return reply{out: agentGetOutput(pane.fakeAgent)}
				})
			}
			return sw.realReact(p, text)
		}, reason: NotSentPaneChanged},
		{name: "the pane's directory changed after it was listed", react: func(sw *sendWorld, p *simPane, text string) error {
			if text == "/runpr 42" {
				sw.byPane["w1:p1"].Dir += "-other"
			}
			return sw.realReact(p, text)
		}, reason: NotSentUnreadable},
		{name: "the report left the screen", react: func(sw *sendWorld, p *simPane, text string) error {
			sw.byPane["w1:p1"].screen = readScreenSample(sw.t, "claude-idle.ansi")
			return sw.realReact(p, text)
		}, reason: NotSentNoReport, offered: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			switch c.plan {
			case "":
			case "none":
				sw.rec.replyIn(tree, "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/", nil)
			default:
				sw.docs(tree, fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, c.plan)})
			}
			if c.ran != "" {
				sw.firstPane().ran = c.ran
			}
			deps := sw.finish()
			p := sw.firstPane()
			if c.typed != "" {
				if err := sw.writeTranscript(p, p.Session, typedLine(c.typed)); err != nil {
					t.Fatal(err)
				}
			}
			if c.queued {
				queued := strings.Split(strings.TrimSpace(string(readTestdata(t, "transcript/claude-queued-input.jsonl"))), "\n")
				if err := sw.writeTranscript(p, p.Session, queued[2]); err != nil {
					t.Fatal(err)
				}
			}
			if !c.noReport {
				p.screen = reportScreen(t, prURL)
			}
			if c.react != nil {
				sw.react = func(p *simPane, text string) error { return c.react(sw, p, text) }
			}
			out := sendRunpr(t, deps, tree)
			if len(out.Actions) != 2 || out.Actions[0].Outcome != OutcomeSent {
				t.Fatalf("actions = %+v; want /runpr sent and a reset result", out.Actions)
			}
			reset := out.Actions[1]
			typed := sw.rec.prompts()
			note := notOfferedNote(reset)
			if c.offered {
				note = skipOfferedNote
			}
			checkOffered(t, deps.Store, tree, reset, note)
			if c.reason == 0 {
				want := []string{"w1:p2 /new", "w1:p2 /runpr 42", "w1:p1 /clear"}
				if reset.Outcome != OutcomeSent || reset.Input != InputReset || reset.Pane != "w1:p1" || !slices.Equal(typed, want) {
					t.Errorf("reset = %+v, typed %v; want /clear sent to w1:p1 after %v", reset, typed, want[:2])
				}
				return
			}
			if reset.Outcome != OutcomeNotSent || reset.Reason != c.reason || len(typed) != 2 {
				t.Errorf("reset = %+v, typed %v; want not sent, %s", reset, typed, c.reason)
			}
		})
	}
}

// skipsOf returns the readable skip records of the workspace at tree,
// failing t on one that cannot be read.
func skipsOf(t *testing.T, store Store, tree string) []skipRecord {
	t.Helper()
	records, err := store.Records(Worktree{path: tree})
	if err != nil {
		t.Fatal(err)
	}
	found, err := records.skipRecords()
	if err != nil {
		t.Fatal(err)
	}
	var recs []skipRecord
	for _, f := range found {
		if f.err != nil {
			t.Fatalf("skip record %s: %v", f.path, f.err)
		}
		recs = append(recs, f.rec)
	}
	return recs
}

// skipNotes are the notes the tool adds to a reset skipped with no_report.
var skipNotes = []string{skipOfferedNote, skipNotOfferedNote, skipUnwrittenNote}

// notOfferedNote is the note a reset that is not offered carries:
// skipNotOfferedNote for one skipped with no_report, none for any other.
func notOfferedNote(reset ActionResult) string {
	if reset.Outcome == OutcomeNotSent && reset.Reason == NotSentNoReport {
		return skipNotOfferedNote
	}
	return ""
}

// checkOffered checks that the workspace at tree keeps one skip record
// exactly when note is skipOfferedNote, and that the reset's detail carries
// note and no other of skipNotes (none when note is empty). The detail is
// compared with the notes the tool adds because their wording is what the
// human reads.
func checkOffered(t *testing.T, store Store, tree string, reset ActionResult, note string) {
	t.Helper()
	recs := skipsOf(t, store, tree)
	if want := map[bool]int{true: 1, false: 0}[note == skipOfferedNote]; len(recs) != want {
		t.Errorf("skip records %+v; want %d", recs, want)
	}
	for _, n := range skipNotes {
		if strings.Contains(reset.Detail, n) != (n == note) {
			t.Errorf("detail %q; want the note %q and no other", reset.Detail, note)
			return
		}
	}
}

// TestResetChecks checks that the pane to empty gets the checks of any
// input: a pane that does not accept input, or that holds unsent text, is
// left as it is, with the text.
func TestResetChecks(t *testing.T) {
	cases := []struct {
		name   string
		set    func(sw *sendWorld, p *simPane)
		reason NotSentReason
		left   string
	}{
		{"unsent text", func(sw *sendWorld, p *simPane) {
			p.screen = withInput(sw.t, reportScreen(sw.t, prURL), "unsent draft text")
		}, NotSentInputLeft, "unsent draft text"},
		{"not accepting input", func(_ *sendWorld, p *simPane) { p.Status = "working" }, NotSentNotAccepting, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			deps := sw.finish()
			sw.firstPane().screen = reportScreen(t, prURL)
			sw.react = func(p *simPane, text string) error {
				if text == "/runpr 42" {
					c.set(sw, sw.byPane["w1:p1"])
				}
				return sw.realReact(p, text)
			}
			out := sendRunpr(t, deps, tree)
			if len(out.Actions) != 2 {
				t.Fatalf("actions = %+v", out.Actions)
			}
			reset := out.Actions[1]
			if reset.Outcome != OutcomeNotSent || reset.Reason != c.reason || reset.Left != c.left {
				t.Errorf("reset = %+v; want not sent, %s, left %q", reset, c.reason, c.left)
			}
			checkOffered(t, deps.Store, tree, reset, "")
			if typed := sw.rec.prompts(); len(typed) != 2 {
				t.Errorf("typed %v; want only /new and /runpr", typed)
			}
		})
	}
}

// TestDetailSavedBeforeReset checks that the PR's detail is kept before
// /runpr's first input, so the report survives the reset of its pane.
func TestDetailSavedBeforeReset(t *testing.T) {
	sw, tree := detailWorld(t)
	deps := sw.finish()
	sw.firstPane().screen = reportScreen(t, prURL)
	records, err := deps.Store.Records(Worktree{path: tree})
	if err != nil {
		t.Fatal(err)
	}
	kept, checked := false, false
	sw.react = func(p *simPane, text string) error {
		if !checked {
			checked = true
			rec, ok, err := records.readDetail(42, prHead)
			kept = err == nil && ok && rec.Source == ReportFromScreen.String() && rec.Pane == "w1:p1"
		}
		return sw.realReact(p, text)
	}
	out := sendRunpr(t, deps, tree)
	if !kept {
		t.Fatalf("no detail with the report was kept before the first input; actions %+v", out.Actions)
	}
	if len(out.Actions) != 2 || out.Actions[1].Outcome != OutcomeSent {
		t.Errorf("actions = %+v; want the reset sent", out.Actions)
	}
}

// TestResetFailureAfterClear checks a reset whose /clear keeps the session:
// the output says the reset input was sent and then failed after clear, and
// the workspace is stopped.
func TestResetFailureAfterClear(t *testing.T) {
	sw, tree := detailWorld(t)
	deps := sw.finish()
	sw.firstPane().screen = reportScreen(t, prURL)
	sw.react = func(p *simPane, text string) error {
		if text == "/clear" {
			return nil
		}
		return sw.realReact(p, text)
	}
	out := sendRunpr(t, deps, tree).Output()
	if len(out.Actions) != 2 {
		t.Fatalf("actions = %+v", out.Actions)
	}
	if got := out.Actions[1]; got.Input != "reset" || got.Outcome != "sent_then_failed" || got.Step != "clear" || got.Pane != "w1:p1" {
		t.Errorf("reset = %+v, want sent then failed after clear", got)
	}
	records, err := deps.Store.Records(Worktree{path: tree})
	if err != nil {
		t.Fatal(err)
	}
	if stop, _ := records.Stopped(); stop == nil {
		t.Error("the workspace was not stopped")
	}
}

// TestNoResetAfterAnUnrecordedRunpr checks that when /runpr is sent but its
// end record cannot be written, which stops the workspace, no pane of it is
// reset.
func TestNoResetAfterAnUnrecordedRunpr(t *testing.T) {
	sw, tree := detailWorld(t)
	deps := sw.finish()
	sw.firstPane().screen = reportScreen(t, prURL)
	deps.Store.beforeCreate = func(name string) error {
		if strings.HasSuffix(name, endSuffix) {
			return errNoReply
		}
		return nil
	}
	out := sendRunpr(t, deps, tree)
	if len(out.Actions) != 2 || out.Actions[0].Outcome != OutcomeSent {
		t.Fatalf("actions = %+v", out.Actions)
	}
	if reset := out.Actions[1]; reset.Outcome != OutcomeNotSent || reset.Reason != NotSentStopped {
		t.Errorf("reset = %+v, want not sent, stopped", reset)
	}
	if typed := sw.rec.prompts(); len(typed) != 2 {
		t.Errorf("typed %v; want only /new and /runpr", typed)
	}
}

// TestResetAfterRunprSkipOffer checks which skips of the reset after /runpr
// are kept as reset items: one where the Claude Code pane that ran the
// command shows no report is kept with the pane's session, state, and the
// command; a skip whose report was another one, a pane of OpenCode, and a
// skip whose other conditions do not hold when the skip is kept, or whose
// record cannot be written, are not. Every one keeps the reset's outcome,
// reason, and inputs.
func TestResetAfterRunprSkipOffer(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, sw *sendWorld, tree string, deps *Deps)
		react   func(sw *sendWorld, p *simPane, text string)
		offered bool
		// unwritten is true when the record could not be written.
		unwritten bool
		// why, when set, is the reason the detail must give for not
		// offering the skip: its wording is what the human reads.
		why string
	}{
		{name: "no report on the Claude Code pane", offered: true},
		{name: "an OpenCode pane made the PR", why: whyOpenCodeMaker, setup: func(t *testing.T, sw *sendWorld, tree string, _ *Deps) {
			sw.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", claudeRunprContext(t))
			sw.docs(tree, fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-sonnet.md")})
		}},
		{name: "no PR detail was kept", setup: func(_ *testing.T, sw *sendWorld, tree string, _ *Deps) {
			sw.rec.failIn(tree, "gh pr view 42 --json "+prViewFields, errNoReply)
		}},
		{name: "the detail kept another pane's report", setup: func(t *testing.T, sw *sendWorld, tree string, deps *Deps) {
			records, err := deps.Store.Records(Worktree{path: tree})
			if err != nil {
				t.Fatal(err)
			}
			if err := records.store().create(detailName(42, prHead), detailRecord{PR: 42, Head: prHead, Saved: sw.now, Source: ReportFromScreen.String(), Text: "report", Pane: "w1:p9", Seq: 10}); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "text typed after the command", setup: func(t *testing.T, sw *sendWorld, _ string, _ *Deps) {
			p := sw.firstPane()
			if err := sw.writeTranscript(p, p.Session, typedLine("is it done?")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "the transcript is gone", setup: func(t *testing.T, sw *sendWorld, _ string, _ *Deps) {
			p := sw.firstPane()
			if err := os.Remove(sw.transcript(p, p.Session)); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "the pane works by then", react: func(sw *sendWorld, _ *simPane, _ string) {
			sw.byPane["w1:p1"].Status = "working"
		}},
		{name: "the pane is in another Herdr workspace", react: func(sw *sendWorld, _ *simPane, _ string) {
			sw.byPane["w1:p1"].Workspace = "w9"
		}},
		{name: "the pane moves after the screen lost the report", setup: func(_ *testing.T, sw *sendWorld, _ string, _ *Deps) {
			sw.firstPane().screen = reportScreen(sw.t, prURL)
		}, react: func(sw *sendWorld, _ *simPane, _ string) {
			// The reset reads the pane's agent, then its screen, which no
			// longer shows the report; the pane moves before the offer reads
			// its agent again.
			pane, reads := sw.byPane["w1:p1"], 0
			pane.screen = readScreenSample(sw.t, "claude-idle.ansi")
			sw.rec.replyFunc("herdr agent get w1:p1", func() reply {
				sw.mu.Lock()
				defer sw.mu.Unlock()
				reads++
				if reads == 2 {
					pane.Seq++
				}
				return reply{out: agentGetOutput(pane.fakeAgent)}
			})
		}},
		{name: "the pane cannot be read again", react: func(sw *sendWorld, _ *simPane, _ string) {
			sw.rec.replyFunc("herdr agent get w1:p1", func() reply { return reply{err: errNoReply} })
		}},
		{name: "the record cannot be written", setup: func(_ *testing.T, _ *sendWorld, _ string, deps *Deps) {
			deps.Store.beforeCreate = failCreates(skipPrefix)
		}, unwritten: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			deps := sw.finish()
			if c.setup != nil {
				c.setup(t, sw, tree, &deps)
			}
			if c.react != nil {
				sw.react = func(p *simPane, text string) error {
					err := sw.realReact(p, text)
					if strings.HasPrefix(text, "/runpr") {
						c.react(sw, p, text)
					}
					return err
				}
			}
			out := sendRunpr(t, deps, tree)
			if len(out.Actions) != 2 || out.Actions[0].Outcome != OutcomeSent {
				t.Fatalf("actions = %+v; want /runpr sent and a reset result", out.Actions)
			}
			reset := out.Actions[1]
			typed := sw.rec.prompts()
			if reset.Outcome != OutcomeNotSent || reset.Reason != NotSentNoReport || len(typed) == 0 || !strings.HasSuffix(typed[len(typed)-1], " /runpr 42") {
				t.Fatalf("reset = %+v, typed %v; want not sent, no_report, and nothing typed after /runpr", reset, typed)
			}
			deps.Store.beforeCreate = nil
			note := skipNotOfferedNote
			switch {
			case c.offered:
				note = skipOfferedNote
			case c.unwritten:
				note = skipUnwrittenNote
			}
			checkOffered(t, deps.Store, tree, reset, note)
			if c.why != "" && !strings.HasSuffix(reset.Detail, skipNotOfferedNote+c.why) {
				t.Errorf("detail %q; want it to end with %q", reset.Detail, c.why)
			}
			if !c.offered {
				return
			}
			p := sw.firstPane()
			rec := skipsOf(t, deps.Store, tree)[0]
			if rec.pane.String() != "w1:p1" || rec.session != p.Session || rec.seq != int64(p.Seq) || rec.scene != sceneAfterRunpr || rec.command != mkarchRan || rec.workspace != "w1" {
				t.Errorf("record = %+v; want w1:p1's session and state after /mkarch 0001", rec)
			}
		})
	}
}
