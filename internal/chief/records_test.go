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

// TestStartCleansFinishedSessionNotices checks that start removes the notice,
// round, and observation records of other sessions and keeps the current
// session's, the automatic-send records, and every file of no record form.
func TestStartCleansFinishedSessionNotices(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	var kept, gone []string
	for _, session := range []string{"old-session", ownerA().Session} {
		if _, _, err := store.Observe(session, "item", now); err != nil {
			t.Fatal(err)
		}
		start, err := store.BeginRound(session, now)
		if err != nil {
			t.Fatal(err)
		}
		obs := Observation{Session: session, Round: start.Cur.ID, PR: 42, Worktree: "/w", Task: "0001", Branch: taskBranch}
		if err := store.RecordObservation(obs); err != nil {
			t.Fatal(err)
		}
		names := []string{noticeName(session, "item"), roundPrefix + hashName(session) + "-" + start.Cur.ID + recordSuffix, observeName(session, start.Cur.ID, "/w", 42)}
		if session == ownerA().Session {
			kept = append(kept, names...)
		} else {
			gone = append(gone, names...)
		}
	}
	if err := store.recordAutoSend(autoSendRecord{PR: 42, Worktree: "/w", Task: "0001", Command: "/mkarch 0001", At: now}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes.txt", "notice-backup.json", "round-backup.json"} {
		if err := os.WriteFile(filepath.Join(store.Dir(), name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	res, err := store.Start(ctx, herdrWith(t, liveAgent(ownerA())), ownerA(), now)
	if err != nil || res.Removed != len(gone) {
		t.Fatalf("Start = %+v, %v; want %d records removed", res, err, len(gone))
	}
	for _, name := range append(kept, autoSendName(42), "notes.txt", "notice-backup.json", "round-backup.json", lockFile) {
		if _, err := os.Lstat(filepath.Join(store.Dir(), name)); err != nil {
			t.Errorf("%s was removed: %v", name, err)
		}
	}
	for _, name := range gone {
		if _, err := os.Lstat(filepath.Join(store.Dir(), name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the finished session's %s is still there: %v", name, err)
		}
	}
}

// TestBeginRound checks that a round returns the session's newest round
// before it, never another session's, removes the records of the rounds
// before that one, and refuses a previous round it cannot read and a clock
// that did not move on.
func TestBeginRound(t *testing.T) {
	store := newStore(t)
	t0 := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	session := ownerA().Session
	var rounds []Round
	for i := range 3 {
		start, err := store.BeginRound(session, t0.Add(time.Duration(i)*time.Minute))
		if err != nil || start.PruneErr != nil {
			t.Fatalf("round %d: %+v, %v", i, start, err)
		}
		if i == 0 && start.Prev != nil {
			t.Fatalf("the first round has a previous round %+v", start.Prev)
		}
		if i > 0 && (start.Prev == nil || *start.Prev != rounds[i-1]) {
			t.Fatalf("round %d: previous %+v, want %+v", i, start.Prev, rounds[i-1])
		}
		if err := store.RecordObservation(Observation{Session: session, Round: start.Cur.ID, PR: 42, Worktree: "/w", Task: "0001", Branch: taskBranch}); err != nil {
			t.Fatal(err)
		}
		rounds = append(rounds, start.Cur)
		if _, err := store.BeginRound("other", t0.Add(time.Duration(i)*time.Minute+time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if obs, _ := store.Observations(session, rounds[0].ID); len(obs) != 0 {
		t.Errorf("the observations of the round before the previous one were kept: %+v", obs)
	}
	if obs, errs := store.Observations(session, rounds[1].ID); len(obs) != 1 || len(errs) != 0 {
		t.Errorf("the previous round's observations = %+v, %v; want one", obs, errs)
	}
	if _, err := store.BeginRound(session, rounds[2].At); !errors.Is(err, errRecord) {
		t.Errorf("a round at the previous round's time = %v; want errRecord", err)
	}
	name := filepath.Join(store.Dir(), roundPrefix+hashName(session)+"-"+rounds[2].ID+recordSuffix)
	if err := os.Chmod(name, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginRound(session, rounds[2].At.Add(time.Minute)); err == nil {
		t.Error("a previous round that cannot be read was skipped")
	}
}

// TestObservationsSkipUnreadable checks that an observation record that
// cannot be read, or whose content names another PR than its file name, is
// reported and never returned.
func TestObservationsSkipUnreadable(t *testing.T) {
	store := newStore(t)
	good := Observation{Session: "s", Round: "00000000000000000001-0000000000000000", PR: 42, Worktree: "/w", Task: "0001", Branch: taskBranch}
	if err := store.RecordObservation(good); err != nil {
		t.Fatal(err)
	}
	moved := good
	moved.PR = 43
	if err := store.create(observeName("s", good.Round, "/w", 44), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), observeName("s", good.Round, "/w", 45)), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	obs, errs := store.Observations("s", good.Round)
	if len(obs) != 1 || obs[0].PR != 42 || len(errs) != 2 {
		t.Fatalf("Observations = %+v, %v; want only #42 and two errors", obs, errs)
	}
}

// TestAutoSentRecord checks the automatic-send record: absent is not sent,
// a written record is sent and cannot be written again, and a record that
// cannot be read counts as sent with its error.
func TestAutoSentRecord(t *testing.T) {
	store := newStore(t)
	if sent, err := store.AutoSent(42); sent || err != nil {
		t.Fatalf("AutoSent before the record = %v, %v", sent, err)
	}
	rec := autoSendRecord{PR: 42, Worktree: "/w", Task: "0001", Command: "/mkarch 0001", At: time.Now()}
	if err := store.recordAutoSend(rec); err != nil {
		t.Fatal(err)
	}
	if err := store.recordAutoSend(rec); !errors.Is(err, privatefile.ErrExist) {
		t.Errorf("a second record = %v; want ErrExist", err)
	}
	if sent, err := store.AutoSent(42); !sent || err != nil {
		t.Errorf("AutoSent after the record = %v, %v", sent, err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), autoSendName(43)), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if sent, err := store.AutoSent(43); !sent || err == nil {
		t.Errorf("AutoSent of an unreadable record = %v, %v; want sent with an error", sent, err)
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

// TestNoticeNumbers checks that each item of a session gets the next number
// when first seen and keeps it, that a number is not given again after its
// item stopped being seen, and that each session counts from 1.
func TestNoticeNumbers(t *testing.T) {
	store := newStore(t)
	now := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	number := func(session string, item ItemID) int {
		t.Helper()
		if _, _, err := store.Observe(session, item, now); err != nil {
			t.Fatal(err)
		}
		n, err := store.NoticeNumber(session, item)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, step := range []struct {
		session string
		item    ItemID
		want    int
	}{
		{"s", "a", 1},
		{"s", "b", 2},
		{"s", "a", 1},
		// b is not seen again; a new item does not take its number.
		{"s", "c", 3},
		{"other", "a", 1},
		{"s", "d", 4},
	} {
		if got := number(step.session, step.item); got != step.want {
			t.Errorf("session %s item %s: number %d, want %d", step.session, step.item, got, step.want)
		}
	}
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

// TestObserveReportsANumberThatCannotBeGiven checks that a new item whose
// number cannot be found (another notice record of the session is
// unreadable) is still recorded as new, unnumbered, and the failure is
// returned so the round reports it.
func TestObserveReportsANumberThatCannotBeGiven(t *testing.T) {
	store := newStore(t)
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	if _, err := privatefile.Create(filepath.Join(store.Dir(), noticeName("s", "broken")), []byte("not json")); err != nil {
		t.Fatal(err)
	}
	if _, isNew, err := store.Observe("s", "item", now); err == nil || !isNew {
		t.Errorf("Observe = new %v, error %v; want new and an error", isNew, err)
	}
	if n, err := store.NoticeNumber("s", "item"); err != nil || n != 0 {
		t.Errorf("NoticeNumber = %d, %v; want 0, the item unnumbered", n, err)
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

// TestStartCleansSendsAndDetails checks start's cleanup: the records of a
// send finished more than sendKeepAge ago and a detail older than the kept
// days are removed, while a send finished within sendKeepAge (which the
// automatic send still reads), an unfinished send's start, a recent detail,
// and a stop stay.
func TestStartCleansSendsAndDetails(t *testing.T) {
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	held := heldRecords(t, store, "/work/a")
	var recent string
	for _, at := range []time.Time{now.Add(-sendKeepAge - time.Second), now.Add(-sendKeepAge + time.Second)} {
		done, err := held.BeginSend(SendRecord{Pane: "w1:p1"}, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := held.EndSend(done, OutcomeSent.String(), "", at); err != nil {
			t.Fatal(err)
		}
		recent = done
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
		if err := held.store().create(reportName(pr, head), detailRecord{PR: pr, Head: head, Saved: saved, Source: ReportFromScreen.String()}); err != nil {
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
	if res.RemovedSends != 4 || res.Stopped != 1 {
		t.Errorf("Start = %+v; want 4 records removed and 1 send stopped", res)
	}
	records, err := store.Records(Worktree{path: "/work/a"})
	if err != nil {
		t.Fatal(err)
	}
	starts, ends, err := records.sendFiles()
	if _, kept := ends[recent]; err != nil || !slices.Equal(starts, []string{recent, open}) || len(ends) != 1 || !kept {
		t.Errorf("send records = %v %v %v; want the recent finished send and the unfinished start", starts, ends, err)
	}
	if _, ok, _ := records.readDetail(1, head); ok {
		t.Error("the old detail was kept")
	}
	if _, ok, err := records.readDetail(2, head); !ok || err != nil {
		t.Errorf("the recent detail: %v, %v; want it kept", ok, err)
	}
	if _, err := os.Stat(filepath.Join(records.dir, reportName(1, head))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the old report record: %v; want it removed", err)
	}
	if rec, ok, err := records.keptPaneReport(2, head); !ok || err != nil || rec.Source != ReportFromScreen.String() {
		t.Errorf("the recent report record: %+v, %v, %v; want it kept", rec, ok, err)
	}
	if stop, _ := records.Stopped(); stop == nil {
		t.Error("the stop of the unfinished send is missing")
	}
}

// TestResolveOwnerChecksTheWorktree checks that HERDR_PANE_ID is trusted
// only when its pane's agent works in the caller's worktree: a pane of
// another workspace (an environment inherited from another pane) is
// ErrPaneMismatch, and a working directory that is not absolute (one that
// could not be read) is a failure, not a mismatch; a working
// directory below the worktree is accepted; a pane whose worktree cannot be
// read is a failure, not a mismatch.
func TestResolveOwnerChecksTheWorktree(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "other", Branch: taskBranch})
	deps := w.finish()
	w.rec.reply("herdr agent get w1:p1", agentGetOutput(w.agents[1]))
	ctx := context.Background()
	if got, err := ResolveOwner(ctx, deps.Cmds, deps.Env); err != nil || got != w.owner() {
		t.Fatalf("own pane = %+v, %v; want the owner", got, err)
	}
	inherited := deps.Env
	inherited.PaneID = "w1:p1"
	if _, err := ResolveOwner(ctx, deps.Cmds, inherited); !errors.Is(err, ErrPaneMismatch) {
		t.Errorf("a pane of another worktree = %v, want ErrPaneMismatch", err)
	}
	relative := deps.Env
	relative.WorkDir = "chief"
	if _, err := ResolveOwner(ctx, deps.Cmds, relative); !errors.Is(err, errWorkDir) || errors.Is(err, ErrPaneMismatch) {
		t.Errorf("a relative working directory = %v, want a failure that is not ErrPaneMismatch", err)
	}
	below := deps.Env
	below.WorkDir = filepath.Join(w.chiefTree, "internal")
	w.git(below.WorkDir, "rev-parse --show-toplevel", w.chiefTree)
	if got, err := ResolveOwner(ctx, deps.Cmds, below); err != nil || got != w.owner() {
		t.Errorf("a directory below the worktree = %+v, %v; want the owner", got, err)
	}
	unread := deps.Env
	unread.PaneID = "w2:p1"
	gone := w.agents[0]
	gone.Pane, gone.Dir = "w2:p1", filepath.Join(w.chiefTree, "..", "gone")
	w.rec.reply("herdr agent get w2:p1", agentGetOutput(gone))
	errNotRepo := errors.New("not a git repository")
	w.rec.failIn(gone.Dir, "git rev-parse --show-toplevel", errNotRepo)
	if _, err := ResolveOwner(ctx, deps.Cmds, unread); !errors.Is(err, errNotRepo) || errors.Is(err, ErrPaneMismatch) {
		t.Errorf("a pane whose worktree cannot be read = %v, want the read's failure, not ErrPaneMismatch", err)
	}
}

// TestKeptPaneReport checks the order the two records of a PR's head are
// read in: the report record before the detail record, neither giving false,
// and a report record naming another PR or head, or not from a pane, being a
// record error rather than read past.
func TestKeptPaneReport(t *testing.T) {
	head := strings.Repeat("b", 40)
	other := strings.Repeat("c", 40)
	saved := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	detail := detailRecord{PR: 7, Head: head, Saved: saved, Source: ReportFromDescription.String(), Text: "description"}
	report := detailRecord{PR: 7, Head: head, Saved: saved, Source: ReportFromScreen.String(), Text: "report", Pane: "w1:p1", Seq: 3}
	cases := []struct {
		name    string
		detail  *detailRecord
		report  *detailRecord
		want    string
		ok      bool
		wantErr bool
	}{
		{"neither", nil, nil, "", false, false},
		{"the detail record alone", &detail, nil, "description", true, false},
		{"the report record alone", nil, &report, "report", true, false},
		{"both", &detail, &report, "report", true, false},
		{"a report record of another PR", &detail, &detailRecord{PR: 8, Head: head, Source: ReportFromScreen.String()}, "", false, true},
		{"a report record of another head", &detail, &detailRecord{PR: 7, Head: other, Source: ReportFromScreen.String()}, "", false, true},
		{"a report record from the description", &detail, &detailRecord{PR: 7, Head: head, Source: ReportFromDescription.String()}, "", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			records, err := newStore(t).Records(Worktree{path: "/work/a"})
			if err != nil {
				t.Fatal(err)
			}
			if c.detail != nil {
				if err := records.store().create(detailName(7, head), *c.detail); err != nil {
					t.Fatal(err)
				}
			}
			if c.report != nil {
				if err := records.store().create(reportName(7, head), *c.report); err != nil {
					t.Fatal(err)
				}
			}
			rec, ok, err := records.keptPaneReport(7, head)
			if ok != c.ok || rec.Text != c.want || errors.Is(err, errRecord) != c.wantErr || (err != nil) != c.wantErr {
				t.Errorf("keptPaneReport = %+v, %v, %v; want text %q, ok %v, record error %v", rec, ok, err, c.want, c.ok, c.wantErr)
			}
		})
	}
}
