//go:build test && unix

package chief

import (
	"context"
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
		typed    string
		noReport bool
		react    func(sw *sendWorld, p *simPane, text string) error
		reason   NotSentReason
	}{
		{name: "a task-document PR's pane"},
		{name: "a /mkreq PR's pane", plan: "none", ran: "/mkreq 12"},
		{name: "a plan PR made on Claude Code", plan: "marker-opus.md", ran: "/runplan 0001"},
		{name: "another document command of the task", ran: "/mkplan 0001", reason: NotSentOtherInput},
		{name: "a plan PR whose pane last ran another command", plan: "marker-opus.md", reason: NotSentOtherInput},
		{name: "text typed after the command", typed: "what is the state of PR 42?", reason: NotSentOtherInput},
		{name: "the command of another task", ran: "/mkarch 0002", reason: NotSentOtherInput},
		{name: "/mkreq without an issue", plan: "none", ran: "/mkreq", reason: NotSentOtherInput},
		{name: "a plan PR made on OpenCode, the pane /runpr went to", plan: "marker-sonnet.md", reason: NotSentSamePane},
		{name: "a plan PR whose requirement has no route", plan: "marker-unreadable.md", reason: NotSentUnreadable},
		{name: "no report on the pane", noReport: true, reason: NotSentNoReport},
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
		{name: "the report left the screen", react: func(sw *sendWorld, p *simPane, text string) error {
			sw.byPane["w1:p1"].screen = readScreenSample(sw.t, "claude-idle.ansi")
			return sw.realReact(p, text)
		}, reason: NotSentNoReport},
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
