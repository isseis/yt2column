//go:build test && unix

package chief

import (
	"context"
	"errors"
	"testing"
	"time"
)

// waitWorld is a world with one task workspace whose agent list a test
// changes between the wait's reads, chief started, and a clock the wait's
// sleep moves forward.
type waitWorld struct {
	*world
	deps Deps
	// sleeps counts the wait's sleeps.
	sleeps int
	// reads counts the agent list reads.
	reads int
}

func newWaitWorld(t *testing.T, ws fakeWorkspace) *waitWorld {
	t.Helper()
	w := newWorld(t)
	w.add(ws)
	ww := &waitWorld{world: w}
	ww.deps = w.finish()
	ww.deps.Sleep = func(_ context.Context, d time.Duration) error {
		ww.sleeps++
		w.now = w.now.Add(d)
		return nil
	}
	if _, err := w.store.Start(context.Background(), NewHerdr(ww.deps.Cmds), w.owner(), w.now); err != nil {
		t.Fatal(err)
	}
	return ww
}

// baseline returns the baseline a snapshot of the world gives now.
func (ww *waitWorld) baseline() WaitBaseline {
	ww.t.Helper()
	snap, err := TakeSnapshot(context.Background(), ww.deps, ww.owner())
	if err != nil {
		ww.t.Fatal(err)
	}
	return snap.Baseline
}

// changeAgents makes the agent list come from list from now on.
func (ww *waitWorld) changeAgents(list func(read int) []fakeAgent) {
	ww.rec.replyFunc("herdr agent list", func() reply {
		ww.reads++
		return reply{out: agentListOutput(list(ww.reads))}
	})
}

func (ww *waitWorld) wait(since WaitBaseline, timeout string) (WaitOutcome, error) {
	ww.t.Helper()
	req, err := ParseWaitRequest(string(since), timeout)
	if err != nil {
		ww.t.Fatal(err)
	}
	return Wait(context.Background(), ww.deps, ww.owner(), req)
}

// withAgent returns the world's agents with the task agent (the second)
// changed by edit.
func (ww *waitWorld) withAgent(edit func(*fakeAgent)) []fakeAgent {
	agents := append([]fakeAgent{}, ww.agents...)
	edit(&agents[1])
	return agents
}

// TestWaitReturnsOnChange checks that a wait returns at once when the task
// agents already differ from the baseline, returns when they change during
// the wait (a status or a state_change_seq), and otherwise waits until its
// time limit, which is an error.
func TestWaitReturnsOnChange(t *testing.T) {
	cases := []struct {
		name string
		// edit changes the task agent from the given read on; nil keeps it.
		edit   func(*fakeAgent)
		from   int
		sleeps int
	}{
		{name: "nothing changes"},
		{name: "already changed", edit: func(a *fakeAgent) { a.Seq++ }, from: 1, sleeps: 0},
		{name: "the state_change_seq changes", edit: func(a *fakeAgent) { a.Seq++ }, from: 3, sleeps: 2},
		{name: "the status changes", edit: func(a *fakeAgent) { a.Status = "working" }, from: 2, sleeps: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ww := newWaitWorld(t, fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON})
			since := ww.baseline()
			ww.changeAgents(func(read int) []fakeAgent {
				if c.edit != nil && read >= c.from {
					return ww.withAgent(c.edit)
				}
				return ww.agents
			})
			out, err := ww.wait(since, "30")
			if c.edit == nil {
				if !errors.Is(err, ErrWaitTimeout) || ww.sleeps != 10 {
					t.Fatalf("Wait = %+v, %v after %d sleeps; want the time limit after 10", out, err, ww.sleeps)
				}
				return
			}
			if err != nil || out.Baseline == since || ww.sleeps != c.sleeps {
				t.Fatalf("Wait = %+v, %v after %d sleeps; want a new baseline after %d", out, err, ww.sleeps, c.sleeps)
			}
			if again := ww.baseline(); again != out.Baseline {
				t.Errorf("the wait's baseline %s is not the snapshot's %s of the same state", out.Baseline, again)
			}
		})
	}
}

// TestWaitKeepsWaitingOnBlocked checks that an agent blocked on a question
// that stays blocked does not end the wait, and that it ends once the agent
// moves on.
func TestWaitKeepsWaitingOnBlocked(t *testing.T) {
	ww := newWaitWorld(t, fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Status: "blocked", Screen: "claude-question-single.ansi"})
	since := ww.baseline()
	ww.changeAgents(func(int) []fakeAgent { return ww.agents })
	if _, err := ww.wait(since, "9"); !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("Wait = %v; a blocked agent that stays blocked ended the wait", err)
	}
	ww.changeAgents(func(read int) []fakeAgent {
		if read >= 2 {
			return ww.withAgent(func(a *fakeAgent) { a.Status, a.Seq = "working", a.Seq+1 })
		}
		return ww.agents
	})
	ww.reads = 0
	if out, err := ww.wait(since, "9"); err != nil || out.Baseline == since {
		t.Fatalf("Wait = %+v, %v; the agent that moved on did not end the wait", out, err)
	}
}

// TestWaitIgnoresChiefWorkspace checks that a change of chief's own agent,
// or of an agent of a workspace of another repository, does not end the wait.
func TestWaitIgnoresChiefWorkspace(t *testing.T) {
	ww := newWaitWorld(t, fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON})
	other := fakeAgent{Pane: "wx:p1", Workspace: "wx", Kind: "claude", Status: "idle", Session: "00000000-0000-4000-8000-0000000000e1", Dir: "/elsewhere", Seq: 3}
	ww.rec.replyIn("/elsewhere", "git rev-parse --path-format=absolute --git-common-dir", []byte("/elsewhere/.git"))
	ww.agents = append(ww.agents, other)
	ww.rec.reply("herdr agent list", agentListOutput(ww.agents))
	since := ww.baseline()
	ww.changeAgents(func(read int) []fakeAgent {
		agents := append([]fakeAgent{}, ww.agents...)
		agents[0].Seq += read
		agents[0].Status = "idle"
		agents[2].Seq += read
		agents[2].Status = "working"
		return agents
	})
	if _, err := ww.wait(since, "9"); !errors.Is(err, ErrWaitTimeout) {
		t.Fatalf("Wait = %v; a change outside the task workspaces ended the wait", err)
	}
}

// TestWaitFailure checks that a Herdr failure during the wait ends it with
// that error, not with the time limit or a change.
func TestWaitFailure(t *testing.T) {
	ww := newWaitWorld(t, fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON})
	since := ww.baseline()
	errHerdr := errors.New("herdr is gone")
	ww.rec.replyFunc("herdr agent list", func() reply {
		ww.reads++
		if ww.reads >= 3 {
			return reply{err: errHerdr}
		}
		return reply{out: agentListOutput(ww.agents)}
	})
	if _, err := ww.wait(since, "60"); !errors.Is(err, errHerdr) {
		t.Fatalf("Wait = %v; want the Herdr failure", err)
	}
}

// TestWaitStopsForANewOwner checks that a wait left running by a chief
// session that was cleared stops: when chief's pane has another session, and
// when another session took the lock over with start, the next read refuses
// with ErrNotOwner instead of waiting on as the old owner.
func TestWaitStopsForANewOwner(t *testing.T) {
	for _, takeOver := range []bool{false, true} {
		ww := newWaitWorld(t, fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON})
		since := ww.baseline()
		cleared := ww.chief
		cleared.Session = "00000000-0000-4000-8000-0000000000c2"
		ww.changeAgents(func(read int) []fakeAgent {
			agents := append([]fakeAgent{}, ww.agents...)
			if read >= 2 {
				agents[0] = cleared
			}
			return agents
		})
		if takeOver {
			ww.deps.Sleep = func(_ context.Context, d time.Duration) error {
				ww.sleeps++
				ww.now = ww.now.Add(d)
				if ww.sleeps == 2 {
					ww.rec.reply("herdr agent get "+worldChiefPane, agentGetOutput(cleared))
					newOwner := Owner{Pane: cleared.Pane, Session: cleared.Session}
					if _, err := ww.store.Start(context.Background(), herdrWith(t, cleared), newOwner, ww.now); err != nil {
						t.Fatal(err)
					}
				}
				return nil
			}
		}
		if _, err := ww.wait(since, "60"); !errors.Is(err, ErrNotOwner) {
			t.Errorf("take over %v: Wait = %v; want ErrNotOwner", takeOver, err)
		}
		if ww.sleeps > 2 {
			t.Errorf("take over %v: the wait slept %d times after its owner changed", takeOver, ww.sleeps)
		}
	}
}

// TestParseWaitRequest checks the accepted baseline and time limits; the
// command-line test of cmd/chief checks the rejected ones.
func TestParseWaitRequest(t *testing.T) {
	since := string(baselineOf(nil, nil))
	for timeout, want := range map[string]time.Duration{"": waitDefaultTimeout, "1": time.Second, "3600": time.Hour} {
		req, err := ParseWaitRequest(since, timeout)
		if err != nil || req.timeout != want || string(req.since) != since {
			t.Errorf("ParseWaitRequest(%q) = %+v, %v; want a timeout of %s", timeout, req, err, want)
		}
	}
}
