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

// planPRDocs returns task documents whose plan records PR 42 as its PR-1,
// with model requirement requirement, on origin/main and on the branch.
func planPRDocs(t *testing.T, sw *sendWorld, tree, requirement string) *fakeDocs {
	t.Helper()
	plan := string(planSample(t, "marker-sonnet.md"))
	plan = strings.Replace(plan, "https://github.com/owner/repo/pull/1", prURL, 1)
	// PR-1's model requirement is the plan's first value line ending in
	// ": opus".
	plan = strings.Replace(plan, ": opus\n", ": "+requirement+"\n", 1)
	path := "docs/tasks/" + taskDir + "/" + planFile
	sw.git(tree, "ls-tree -z --name-only HEAD -- "+path, path+"\x00")
	sw.git(tree, "show HEAD:"+path, plan)
	return &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: []byte(plan)}
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

// TestResetAfterRunpr checks the reset after /runpr: the pane of the
// command that created the PR is emptied only when it is another pane than
// /runpr's, its report was kept by the detail, its state has not changed,
// and it still shows the report; otherwise it is left as it is, with the
// reason.
func TestResetAfterRunpr(t *testing.T) {
	cases := []struct {
		name   string
		plan   string
		setup  func(sw *sendWorld)
		react  func(sw *sendWorld, p *simPane, text string) error
		reason NotSentReason
	}{
		{"a task-document PR's pane", "", nil, nil, 0},
		{"a plan PR made on Claude Code", "opus", nil, nil, 0},
		{"a plan PR made on OpenCode, the pane /runpr went to", "sonnet", nil, nil, NotSentSamePane},
		{"a plan PR whose requirement has no route", "unknown-value", nil, nil, NotSentUnreadable},
		{"no report on the pane", "", func(sw *sendWorld) {
			sw.firstPane().screen = readScreenSample(sw.t, "claude-idle.ansi")
		}, nil, NotSentNoReport},
		{"the pane changed after the detail", "", nil, func(sw *sendWorld, p *simPane, text string) error {
			sw.byPane["w1:p1"].Seq++
			return sw.realReact(p, text)
		}, NotSentPaneChanged},
		{"the pane changes after its report was read", "", nil, func(sw *sendWorld, p *simPane, text string) error {
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
		}, NotSentPaneChanged},
		{"the report left the screen", "", nil, func(sw *sendWorld, p *simPane, text string) error {
			sw.byPane["w1:p1"].screen = readScreenSample(sw.t, "claude-idle.ansi")
			return sw.realReact(p, text)
		}, NotSentNoReport},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			if c.plan != "" {
				sw.docs(tree, *planPRDocs(t, sw, tree, c.plan))
			}
			deps := sw.finish()
			sw.firstPane().screen = reportScreen(t, prURL)
			if c.setup != nil {
				c.setup(sw)
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
