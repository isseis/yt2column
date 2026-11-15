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

	"github.com/isseis/yt2column/internal/cachelock"
	"github.com/isseis/yt2column/internal/privatefile"
)

func ownerA() Owner { return Owner{Pane: "wc:p1", Session: "00000000-0000-4000-8000-0000000000a1"} }
func ownerB() Owner { return Owner{Pane: "wd:p2", Session: "00000000-0000-4000-8000-0000000000b2"} }

// herdrWith returns the Herdr reader of a fake whose agents are agents.
func herdrWith(t *testing.T, agents ...fakeAgent) Herdr {
	t.Helper()
	rec := newRecorder(t)
	rec.reply("herdr agent list", agentListOutput(agents))
	return NewHerdr(rec.commands(nil))
}

func liveAgent(o Owner) fakeAgent {
	return fakeAgent{Pane: o.Pane, Workspace: strings.Split(o.Pane, ":")[0], Kind: "claude", Status: "idle", Session: o.Session, Dir: "/work", Seq: 1}
}

func newStore(t *testing.T) Store {
	t.Helper()
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestStateBase(t *testing.T) {
	cases := []struct {
		env     Env
		want    string
		wantErr bool
	}{
		{Env{XDGStateHome: "/xdg", Home: "/home/u"}, "/xdg", false},
		{Env{Home: "/home/u"}, "/home/u/.local/state", false},
		{Env{XDGStateHome: "relative/path", Home: "/home/u"}, "/home/u/.local/state", false},
		{Env{XDGStateHome: "relative"}, "", true},
	}
	for _, c := range cases {
		got, err := StateBase(c.env)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("StateBase(%+v) = %q, %v; want %q, error %v", c.env, got, err, c.want, c.wantErr)
		}
	}
}

func TestOpenStoreRefusesMissingBase(t *testing.T) {
	_, err := OpenStore(filepath.Join(t.TempDir(), "absent"))
	if !errors.Is(err, ErrNoStateBase) {
		t.Fatalf("OpenStore = %v, want ErrNoStateBase", err)
	}
}

func TestStartTakesAndRefusesTheLock(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	t.Run("a free lock is taken and then the owner may start again", func(t *testing.T) {
		store := newStore(t)
		herdr := herdrWith(t, liveAgent(ownerA()))
		if res, err := store.Start(ctx, herdr, ownerA(), now); err != nil || res.TookOver {
			t.Fatalf("Start = %+v, %v", res, err)
		}
		if res, err := store.Start(ctx, herdr, ownerA(), now); err != nil || res.TookOver {
			t.Fatalf("second Start by the owner = %+v, %v", res, err)
		}
		if err := store.Require(ownerA()); err != nil {
			t.Errorf("Require(owner) = %v", err)
		}
	})
	t.Run("a live owner's lock refuses another chief", func(t *testing.T) {
		store := newStore(t)
		herdr := herdrWith(t, liveAgent(ownerA()), liveAgent(ownerB()))
		if _, err := store.Start(ctx, herdr, ownerA(), now); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Start(ctx, herdr, ownerB(), now); !errors.Is(err, ErrLockHeld) {
			t.Fatalf("Start by another chief = %v, want ErrLockHeld", err)
		}
		if err := store.Require(ownerB()); !errors.Is(err, ErrNotOwner) {
			t.Errorf("Require(not the owner) = %v, want ErrNotOwner", err)
		}
	})
	t.Run("a stale lock is taken over", func(t *testing.T) {
		for name, agents := range map[string][]fakeAgent{
			"the owner's pane is gone":             {liveAgent(ownerB())},
			"the pane has another session (clear)": {liveAgent(ownerB()), {Pane: ownerA().Pane, Workspace: "wc", Kind: "claude", Status: "idle", Session: "00000000-0000-4000-8000-0000000000ff", Dir: "/work", Seq: 1}},
		} {
			t.Run(name, func(t *testing.T) {
				store := newStore(t)
				if _, err := store.Start(ctx, herdrWith(t, liveAgent(ownerA())), ownerA(), now); err != nil {
					t.Fatal(err)
				}
				res, err := store.Start(ctx, herdrWith(t, agents...), ownerB(), now)
				if err != nil || !res.TookOver {
					t.Fatalf("Start = %+v, %v; want a takeover", res, err)
				}
				if err := store.Require(ownerB()); err != nil {
					t.Errorf("Require(new owner) = %v", err)
				}
				if err := store.Require(ownerA()); !errors.Is(err, ErrNotOwner) {
					t.Errorf("Require(old owner) = %v, want ErrNotOwner", err)
				}
			})
		}
	})
	t.Run("Herdr unreadable is no takeover", func(t *testing.T) {
		store := newStore(t)
		if _, err := store.Start(ctx, herdrWith(t, liveAgent(ownerA())), ownerA(), now); err != nil {
			t.Fatal(err)
		}
		rec := newRecorder(t)
		rec.fail("herdr agent list", errors.New("herdr is down"))
		if _, err := store.Start(ctx, NewHerdr(rec.commands(nil)), ownerB(), now); !errors.Is(err, ErrLockUnverifiable) {
			t.Fatalf("Start = %v, want ErrLockUnverifiable", err)
		}
	})
	t.Run("an unreadable lock is not taken over", func(t *testing.T) {
		store := newStore(t)
		// A well-formed record of mode 0600 that names no owner: only the
		// owner check can refuse it.
		if err := store.create(lockFile, lockRecord{Started: now}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Start(ctx, herdrWith(t), ownerB(), now); !errors.Is(err, ErrLockUnverifiable) {
			t.Fatalf("Start = %v, want ErrLockUnverifiable", err)
		}
	})
}

// TestRequireBeforeStart checks that no subcommand but start works without
// the lock.
func TestRequireBeforeStart(t *testing.T) {
	if err := newStore(t).Require(ownerA()); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Require = %v, want ErrNotStarted", err)
	}
}

// TestTakeoverRechecksUnderTheExclusion places a live owner's lock after a
// takeover judged the old lock stale and before it took the exclusion: the
// takeover must read the lock again under the exclusion and refuse. The
// exclusion is held by the test meanwhile, so the takeover cannot proceed
// until the lock was replaced.
func TestTakeoverRechecksUnderTheExclusion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	// ownerA starts first and then goes away.
	if _, err := store.Start(ctx, herdrWith(t, liveAgent(ownerA())), ownerA(), now); err != nil {
		t.Fatal(err)
	}
	third := Owner{Pane: "we:p3", Session: "00000000-0000-4000-8000-0000000000c3"}
	// ownerB takes over; third replaces the stale lock with its own live one
	// inside the gap while the test holds the exclusion.
	store.betweenStaleReadAndLock = func() {
		excl, err := cachelock.Acquire(store.Dir())
		if err != nil {
			t.Error(err)
			return
		}
		if err := privatefile.Remove(filepath.Join(store.Dir(), lockFile)); err != nil {
			t.Error(err)
		}
		if err := store.create(lockFile, lockRecord{Owner: third, Started: now}); err != nil {
			t.Error(err)
		}
		_ = excl.Close()
	}
	herdr := herdrWith(t, liveAgent(ownerB()), liveAgent(third))
	if _, err := store.Start(ctx, herdr, ownerB(), now); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("Start = %v, want ErrLockHeld: the stale judgment was not repeated under the exclusion", err)
	}
	if err := store.Require(third); err != nil {
		t.Errorf("the live owner lost the lock: %v", err)
	}
}

// TestStartCleansFinishedSessionNotices checks that start removes the notice
// records of other sessions and keeps the current session's and every file
// that is not a notice record.
func TestStartCleansFinishedSessionNotices(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	if _, _, err := store.Observe("old-session", "item-1", now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Observe(ownerA().Session, "item-2", now); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes.txt", "notice-backup.json"} {
		if err := os.WriteFile(filepath.Join(store.Dir(), name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	res, err := store.Start(ctx, herdrWith(t, liveAgent(ownerA())), ownerA(), now)
	if err != nil || res.Removed != 1 {
		t.Fatalf("Start = %+v, %v; want one notice removed", res, err)
	}
	for _, name := range []string{noticeName(ownerA().Session, "item-2"), "notes.txt", "notice-backup.json", lockFile} {
		if _, err := os.Lstat(filepath.Join(store.Dir(), name)); err != nil {
			t.Errorf("%s was removed: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(store.Dir(), noticeName("old-session", "item-1"))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the finished session's notice is still there: %v", err)
	}
}

// TestObserve checks that a record is created once and its time is kept, and
// that a record that cannot be created gives the time of the observation.
func TestObserve(t *testing.T) {
	store := newStore(t)
	t1 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	first, isNew, err := store.Observe("s", "item", t1)
	if err != nil || !isNew || !first.Equal(t1) {
		t.Fatalf("first Observe = %v, %v, %v", first, isNew, err)
	}
	first, isNew, err = store.Observe("s", "item", t2)
	if err != nil || isNew || !first.Equal(t1) {
		t.Fatalf("second Observe = %v, %v, %v; want the first time and not new", first, isNew, err)
	}
	if _, isNew, _ := store.Observe("other", "item", t2); !isNew {
		t.Error("the same item in another session is not new")
	}
	t.Run("a record that cannot be created", func(t *testing.T) {
		broken := Store{dir: filepath.Join(t.TempDir(), "absent")}
		first, isNew, err := broken.Observe("s", "item", t2)
		if err == nil || !isNew || !first.Equal(t2) {
			t.Errorf("Observe = %v, %v, %v; want now, new, and an error", first, isNew, err)
		}
	})
}

// TestCanonicalWorktreeResolvesLinks checks that the same worktree reached
// through a symbolic link has one records directory.
func TestCanonicalWorktreeResolvesLinks(t *testing.T) {
	target, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	rec := newRecorder(t)
	rec.reply("git rev-parse --show-toplevel", []byte(link+"\n"))
	cmds := rec.commands(nil)
	canonical, err := CanonicalWorktree(context.Background(), cmds, target)
	if err != nil || canonical.String() != target {
		t.Fatalf("CanonicalWorktree = %q, %v; want %q", canonical, err, target)
	}
	store := newStore(t)
	direct, err := store.WorkspaceDir(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(direct); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("records directory = %v, %v; want mode 0700", info, err)
	}
}

// TestTakeoverNeedsTheExclusion holds the state directory's exclusion for the
// whole takeover while the lock is stale: the takeover must refuse and leave
// the lock as it was.
func TestTakeoverNeedsTheExclusion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	if _, err := store.Start(ctx, herdrWith(t, liveAgent(ownerA())), ownerA(), now); err != nil {
		t.Fatal(err)
	}
	excl, err := cachelock.Acquire(store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = excl.Close() }()
	if _, err := store.Start(ctx, herdrWith(t, liveAgent(ownerB())), ownerB(), now); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("Start = %v, want ErrLockHeld while the exclusion is held", err)
	}
	if err := store.Require(ownerA()); err != nil {
		t.Errorf("the lock changed without the exclusion: %v", err)
	}
}

// TestObserveUnreadableRecord checks that a notice record that exists but
// cannot be read is not announced again on every snapshot.
func TestObserveUnreadableRecord(t *testing.T) {
	store := newStore(t)
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	if _, err := privatefile.Create(filepath.Join(store.Dir(), noticeName("s", "item")), []byte("not json")); err != nil {
		t.Fatal(err)
	}
	if _, isNew, err := store.Observe("s", "item", now); err == nil || isNew {
		t.Errorf("Observe = new %v, error %v; want an error and not new", isNew, err)
	}
}

// heldRecords returns the records of a worktree with its exclusion held.
func heldRecords(t *testing.T, store Store, path string) *HeldWorkspace {
	t.Helper()
	records, err := store.Records(Worktree{path: path})
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

// TestUnfinishedSendBecomesStop checks that a send whose end was never
// written stops its workspace: once its writer's exclusion is closed, the
// next holder's settle and start both turn it into a stop, and the start
// record is kept.
func TestUnfinishedSendBecomesStop(t *testing.T) {
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	write := func(t *testing.T, store Store) {
		t.Helper()
		held := heldRecords(t, store, "/work/a")
		if _, err := held.BeginSend(SendRecord{Pane: "w1:p1"}, now); err != nil {
			t.Fatal(err)
		}
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("settle", func(t *testing.T) {
		store := newStore(t)
		write(t, store)
		held := heldRecords(t, store, "/work/a")
		if n, err := held.SettleUnfinished(now); err != nil || n != 1 {
			t.Fatalf("SettleUnfinished = %d, %v; want 1", n, err)
		}
		if stop, err := held.Stopped(); err != nil || stop == nil {
			t.Fatalf("Stopped = %v, %v; want a stop", stop, err)
		}
		if starts, _, err := held.sendFiles(); err != nil || len(starts) != 1 {
			t.Errorf("starts = %v, %v; want the start kept", starts, err)
		}
	})
	t.Run("start", func(t *testing.T) {
		store := newStore(t)
		write(t, store)
		res, err := store.Start(context.Background(), herdrWith(t, liveAgent(ownerA())), ownerA(), now)
		if err != nil || res.Stopped != 1 {
			t.Fatalf("Start = %+v, %v; want one send stopped", res, err)
		}
		records, err := store.Records(Worktree{path: "/work/a"})
		if err != nil {
			t.Fatal(err)
		}
		if stop, err := records.Stopped(); err != nil || stop == nil {
			t.Errorf("Stopped = %v, %v; want a stop", stop, err)
		}
	})
}

// TestStopSurvivesRestart checks that a stop is read again by a store opened
// anew on the same directory.
func TestStopSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	held := heldRecords(t, store, "/work/a")
	if err := held.Stop("why", "screen", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	records, err := again.Records(Worktree{path: "/work/a"})
	if err != nil {
		t.Fatal(err)
	}
	stop, err := records.Stopped()
	if err != nil || stop == nil || stop.Reason != "why" || stop.Screen != "screen" {
		t.Errorf("Stopped = %+v, %v; want the stop kept", stop, err)
	}
}

// TestReleaseClearsStop checks that release removes the stop and gives
// every unfinished send an end, so the same start does not stop the
// workspace again.
func TestReleaseClearsStop(t *testing.T) {
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	held := heldRecords(t, store, "/work/a")
	if _, err := held.BeginSend(SendRecord{Pane: "w1:p1"}, now); err != nil {
		t.Fatal(err)
	}
	if n, err := held.SettleUnfinished(now); err != nil || n != 1 {
		t.Fatalf("SettleUnfinished = %d, %v", n, err)
	}
	released, err := held.Release(now)
	if err != nil || !released {
		t.Fatalf("Release = %v, %v; want released", released, err)
	}
	if stop, err := held.Stopped(); err != nil || stop != nil {
		t.Errorf("Stopped after release = %v, %v; want none", stop, err)
	}
	if n, err := held.SettleUnfinished(now); err != nil || n != 0 {
		t.Errorf("SettleUnfinished after release = %d, %v; want 0", n, err)
	}
	if stop, _ := held.Stopped(); stop != nil {
		t.Errorf("the released start stopped the workspace again: %+v", stop)
	}
	if released, err := held.Release(now); err != nil || released {
		t.Errorf("a second Release = %v, %v; want nothing released", released, err)
	}
}

// TestWorkspaceBusy checks that a send to a workspace whose exclusion the
// test holds types nothing and says another send is under way, and that it
// read nothing of the workspace's row first: the row is read only under the
// exclusion, so no send acts on a row read before another sent.
func TestWorkspaceBusy(t *testing.T) {
	sw := newSendWorld(t)
	tree := sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, "claude")
	deps := sw.finish()
	heldRecords(t, deps.Store, tree)
	req, err := ParseSendRequest(tree, "runplan", "", "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Send(context.Background(), deps, req)
	if err != nil || len(out.Actions) != 1 || out.Actions[0].Outcome != OutcomeNotSent || out.Actions[0].Reason != NotSentBusy {
		t.Fatalf("Send = %+v, %v; want not sent, busy", out, err)
	}
	if got := sw.rec.prompts(); len(got) != 0 {
		t.Errorf("typed %v", got)
	}
	for _, c := range sw.rec.recorded() {
		if c.name == ghProgram || (c.name == gitProgram && c.dir == tree && slices.Contains(c.args, "--abbrev-ref")) {
			t.Errorf("the row was read before the exclusion: %s", c.line())
		}
	}
}

// TestStartCleansSendsAndDetails checks start's cleanup: a finished send's
// records and a detail older than the kept days are removed, while an
// unfinished send's start, a recent detail, and a stop stay.
func TestStartCleansSendsAndDetails(t *testing.T) {
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	held := heldRecords(t, store, "/work/a")
	done, err := held.BeginSend(SendRecord{Pane: "w1:p1"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := held.EndSend(done, OutcomeSent.String(), "", now); err != nil {
		t.Fatal(err)
	}
	open, err := held.BeginSend(SendRecord{Pane: "w1:p1"}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	head := strings.Repeat("a", 40)
	for pr, saved := range map[int]time.Time{1: now.Add(-detailMaxAge - time.Hour), 2: now.Add(-detailMaxAge + time.Hour)} {
		if err := held.store().create(detailName(pr, head), detailRecord{PR: pr, Head: head, Saved: saved, Source: ReportNone.String()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	res, err := store.Start(context.Background(), herdrWith(t, liveAgent(ownerA())), ownerA(), now)
	if err != nil || res.CleanupError != nil {
		t.Fatalf("Start = %+v, %v", res, err)
	}
	if res.RemovedSends != 3 || res.Stopped != 1 {
		t.Errorf("Start = %+v; want 3 records removed and 1 send stopped", res)
	}
	records, err := store.Records(Worktree{path: "/work/a"})
	if err != nil {
		t.Fatal(err)
	}
	starts, ends, err := records.sendFiles()
	if err != nil || !slices.Equal(starts, []string{open}) || len(ends) != 0 {
		t.Errorf("send records = %v %v %v; want only the unfinished start", starts, ends, err)
	}
	if _, ok, _ := records.readDetail(1, head); ok {
		t.Error("the old detail was kept")
	}
	if _, ok, err := records.readDetail(2, head); !ok || err != nil {
		t.Errorf("the recent detail: %v, %v; want it kept", ok, err)
	}
	if stop, _ := records.Stopped(); stop == nil {
		t.Error("the stop of the unfinished send is missing")
	}
}
