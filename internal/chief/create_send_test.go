//go:build test && unix

package chief

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// createdWorld returns a world with the workspace create just made: w1
// holds a worktree on a detached HEAD with one Claude Code pane showing a
// freshly started Claude Code, as the real one does before its first input.
func createdWorld(t *testing.T) (*sendWorld, string, Deps) {
	t.Helper()
	sw := newSendWorld(t)
	tree := sw.addTask(fakeWorkspace{ID: "w1", Label: "issue-12", Branch: detachedHead}, "claude")
	sw.panes[0].screen = readScreenSample(t, "claude-started.ansi")
	return sw, tree, sw.finish()
}

// holdFor takes the exclusion of the worktree at tree, as create does
// before it makes anything.
func holdFor(t *testing.T, deps Deps, tree string) *HeldWorkspace {
	t.Helper()
	records, err := deps.Store.Records(Worktree{path: tree})
	if err != nil {
		t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() })
	return held
}

func TestSendAfterCreate(t *testing.T) {
	ctx := context.Background()
	t.Run("sends /clear, /model, and /mkreq to a freshly started Claude Code", func(t *testing.T) {
		sw, tree, deps := createdWorld(t)
		// The offer is the one check left out: the workspace is on a
		// detached HEAD, so Send itself would not type /mkreq into it.
		out, err := Send(ctx, deps, sendReq(t, tree, "mkreq", "12", ""))
		if err != nil || out.Actions[0].Reason != NotSentNotOffered {
			t.Fatalf("Send to the detached workspace = %+v, %v; want not offered", out, err)
		}
		held := holdFor(t, deps, tree)
		res := sendAfterCreate(ctx, deps, held, Worktree{path: tree}, "w1", issueNumber(t, "12"))
		if res.Outcome != OutcomeSent {
			t.Fatalf("sendAfterCreate = %+v, want sent", res)
		}
		want := []string{"w1:p1 /clear", "w1:p1 /model opus", "w1:p1 /mkreq 12"}
		if got := sw.rec.prompts(); !slices.Equal(got, want) {
			t.Errorf("typed %v, want %v", got, want)
		}
		// The exclusion create holds is still held: the send did not end it.
		if _, err := held.Lock(); !errors.Is(err, ErrWorkspaceBusy) {
			t.Errorf("Lock after the send = %v, want ErrWorkspaceBusy", err)
		}
	})
	t.Run("a stopped workspace gets nothing", func(t *testing.T) {
		sw, tree, deps := createdWorld(t)
		held := holdFor(t, deps, tree)
		if err := held.Stop("an earlier send failed", "", "", time.Now()); err != nil {
			t.Fatal(err)
		}
		res := sendAfterCreate(ctx, deps, held, Worktree{path: tree}, "w1", issueNumber(t, "12"))
		if res.Outcome != OutcomeNotSent || res.Reason != NotSentStopped {
			t.Fatalf("sendAfterCreate = %+v, want not sent, stopped", res)
		}
		if got := sw.rec.prompts(); len(got) != 0 {
			t.Errorf("typed %v", got)
		}
	})
	t.Run("a send left unfinished becomes a stop and nothing is sent", func(t *testing.T) {
		sw, tree, deps := createdWorld(t)
		held := holdFor(t, deps, tree)
		if _, err := held.BeginSend(SendRecord{Pane: "w1:p1", Input: "command", Command: "/mkreq 12"}, time.Now()); err != nil {
			t.Fatal(err)
		}
		res := sendAfterCreate(ctx, deps, held, Worktree{path: tree}, "w1", issueNumber(t, "12"))
		if res.Outcome != OutcomeNotSent || res.Reason != NotSentStopped {
			t.Fatalf("sendAfterCreate = %+v, want not sent, stopped", res)
		}
		if got := sw.rec.prompts(); len(got) != 0 {
			t.Errorf("typed %v", got)
		}
	})
	t.Run("another workspace than the one made gets nothing", func(t *testing.T) {
		sw, tree, deps := createdWorld(t)
		held := holdFor(t, deps, tree)
		res := sendAfterCreate(ctx, deps, held, Worktree{path: tree}, "w2", issueNumber(t, "12"))
		if res.Outcome != OutcomeNotSent || res.Reason != NotSentNoWorkspace {
			t.Fatalf("sendAfterCreate = %+v, want not sent, no workspace", res)
		}
		if got := sw.rec.prompts(); len(got) != 0 {
			t.Errorf("typed %v", got)
		}
	})
	t.Run("the exclusion of another worktree is not accepted", func(t *testing.T) {
		sw, tree, deps := createdWorld(t)
		held := holdFor(t, deps, tree+"-other")
		res := sendAfterCreate(ctx, deps, held, Worktree{path: tree}, "w1", issueNumber(t, "12"))
		if res.Outcome != OutcomeNotSent || res.Reason != NotSentUnreadable {
			t.Fatalf("sendAfterCreate = %+v, want not sent, unreadable", res)
		}
		if got := sw.rec.prompts(); len(got) != 0 {
			t.Errorf("typed %v", got)
		}
	})
	t.Run("a check that fails after the first input stops the workspace", func(t *testing.T) {
		sw, tree, deps := createdWorld(t)
		sw.react = func(p *simPane, text string) error {
			err := sw.realReact(p, text)
			p.Status = "working"
			return err
		}
		held := holdFor(t, deps, tree)
		res := sendAfterCreate(ctx, deps, held, Worktree{path: tree}, "w1", issueNumber(t, "12"))
		if res.Outcome != OutcomeSentThenFailed || res.Step != StepClear {
			t.Fatalf("sendAfterCreate = %+v, want sent then failed at /clear", res)
		}
		if got := sw.rec.prompts(); !slices.Equal(got, []string{"w1:p1 /clear"}) {
			t.Errorf("typed %v, want only /clear: a failed send is not retried", got)
		}
		stop, err := held.Stopped()
		if err != nil || stop == nil {
			t.Fatalf("Stopped = %+v, %v; want a stop record", stop, err)
		}
	})
}
