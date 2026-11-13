package chief

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cachelock"
	"github.com/isseis/yt2column/internal/privatefile"
)

// Protocol is the number of the command-line and output contract. The skill
// that drives the tool passes the same number with --protocol, so an old
// install is refused instead of misread.
const Protocol = 1

// errRecord reports a record that is not in the accepted form.
var errRecord = errors.New("unreadable record")

var (
	// ErrNoStateBase reports that the directory the state directory lives
	// under does not exist.
	ErrNoStateBase = errors.New("the base of the state directory does not exist")
	// ErrNotStarted reports a subcommand run before start took the lock.
	ErrNotStarted = errors.New("chief has not been started in this session")
	// ErrNotOwner reports a caller that is not the lock's owner.
	ErrNotOwner = errors.New("the caller does not own the chief lock")
	// ErrLockHeld reports a lock whose owner is another live chief.
	ErrLockHeld = errors.New("another chief holds the lock")
	// ErrLockUnverifiable reports that the lock's owner could not be
	// checked, so the lock is not taken.
	ErrLockUnverifiable = errors.New("the owner of the chief lock cannot be checked")
	// ErrNoSession reports a caller whose Herdr agent session is unknown.
	ErrNoSession = errors.New("the calling pane has no agent session")
)

// Env is the part of the process environment the tool reads. The entry point
// fills it once; nothing below it reads the environment.
type Env struct {
	// PaneID is HERDR_PANE_ID: the pane chief runs in, empty outside Herdr.
	PaneID string
	// XDGStateHome, Home, and ClaudeConfigDir are the variables of the same
	// names.
	XDGStateHome    string
	Home            string
	ClaudeConfigDir string
}

// ReadEnv reads Env with lookup, which is os.LookupEnv in the entry point.
func ReadEnv(lookup func(string) (string, bool)) Env {
	get := func(name string) string {
		v, _ := lookup(name)
		return v
	}
	return Env{
		PaneID:          get("HERDR_PANE_ID"),
		XDGStateHome:    get("XDG_STATE_HOME"),
		Home:            get("HOME"),
		ClaudeConfigDir: get("CLAUDE_CONFIG_DIR"),
	}
}

// StateBase returns the directory the state directory lives under:
// XDG_STATE_HOME when it is an absolute path, else $HOME/.local/state, as the
// XDG specification says. It fails when neither gives an absolute path.
func StateBase(env Env) (string, error) {
	if filepath.IsAbs(env.XDGStateHome) {
		return env.XDGStateHome, nil
	}
	if filepath.IsAbs(env.Home) {
		return filepath.Join(env.Home, ".local", "state"), nil
	}
	return "", fmt.Errorf("%w: neither XDG_STATE_HOME nor HOME is an absolute path", ErrNoStateBase)
}

// recordLimit caps a record read. The records are small JSON objects.
const recordLimit = 64 << 10

// lockFile is the chief lock's file name; noticePrefix starts a notice
// record's.
const (
	lockFile     = "lock.json"
	noticePrefix = "notice-"
	recordSuffix = ".json"
)

// Store is the state directory: the records and the lock of chief. Every
// file in it is created and read with internal/privatefile.
type Store struct {
	dir string
	// betweenStaleReadAndLock runs after a lock was judged stale and before
	// the directory's exclusion is taken; the takeover tests use it to place
	// a live owner's lock in that gap. It is nil outside the tests.
	betweenStaleReadAndLock func()
}

// OpenStore creates the state directory $base/yt2column/chief when it is
// missing and returns it. A missing base is ErrNoStateBase: privatefile
// creates below an existing directory only.
func OpenStore(base string) (Store, error) {
	if err := privatefile.MkdirBelow(base, []string{"yt2column", "chief"}); err != nil {
		if errors.Is(err, privatefile.ErrBaseMissing) {
			return Store{}, fmt.Errorf("%w: %s", ErrNoStateBase, base)
		}
		return Store{}, err
	}
	return Store{dir: filepath.Join(base, "yt2column", "chief")}, nil
}

// Dir returns the resolved path of the state directory.
func (s Store) Dir() string { return s.dir }

// create writes a new record; a record of that name is never replaced.
func (s Store) create(name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = privatefile.Create(filepath.Join(s.dir, name), data)
	return err
}

// read reads a record into v. A record with a member the reader does not know
// is unreadable, never read in part.
func (s Store) read(name string, v any) error {
	data, err := privatefile.Read(filepath.Join(s.dir, name), recordLimit)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}
	if dec.More() {
		return fmt.Errorf("%w: record %s: trailing data", errRecord, name)
	}
	return nil
}

// Owner is who holds the chief lock: the pane chief runs in and the Herdr
// agent session of that pane.
type Owner struct {
	Pane    string `json:"pane"`
	Session string `json:"session"`
}

// lockRecord is the content of the lock file.
type lockRecord struct {
	Owner
	Started time.Time `json:"started"`
}

// ResolveOwner returns the caller's owner identity: its pane and the session
// Herdr holds for the pane. A pane without a session cannot own the lock,
// since the session is what tells a cleared chief session from the same one.
func ResolveOwner(ctx context.Context, herdr Herdr, paneID string) (Owner, error) {
	pane, err := ParsePaneID(paneID)
	if err != nil {
		return Owner{}, err
	}
	agent, err := herdr.Agent(ctx, pane)
	if err != nil {
		return Owner{}, err
	}
	if agent.Session == "" {
		return Owner{}, ErrNoSession
	}
	return Owner{Pane: pane.String(), Session: agent.Session}, nil
}

// StartResult says what Start did.
type StartResult struct {
	// TookOver is true when a stale lock was replaced.
	TookOver bool
	// Removed counts the notice records of finished sessions that were
	// removed.
	Removed int
}

// Start takes the chief lock for caller and cleans up records of finished
// sessions. Contract: the lock is created, never overwritten; a lock whose
// owner is another live chief is ErrLockHeld; a stale lock is replaced only
// while the exclusion of the state directory is held, and it is judged stale
// again under that exclusion, so two chiefs that start at once cannot both
// win; Herdr unreadable is ErrLockUnverifiable. The stale-send handling of
// the design (3.5) belongs to the phase that creates send records.
func (s Store) Start(ctx context.Context, herdr Herdr, caller Owner, now time.Time) (StartResult, error) {
	var res StartResult
	rec := lockRecord{Owner: caller, Started: now.UTC()}
	err := s.create(lockFile, rec)
	switch {
	case err == nil:
	case errors.Is(err, privatefile.ErrExist):
		took, err := s.settleExistingLock(ctx, herdr, caller, rec)
		if err != nil {
			return res, err
		}
		res.TookOver = took
	default:
		return res, err
	}
	removed, err := s.removeFinishedNotices(caller.Session)
	res.Removed = removed
	return res, err
}

// settleExistingLock handles a lock that already exists: the caller's own
// lock is kept, a live owner's lock refuses, a stale lock is taken over.
func (s Store) settleExistingLock(ctx context.Context, herdr Herdr, caller Owner, rec lockRecord) (bool, error) {
	held, err := s.readLock()
	if err != nil {
		return false, err
	}
	if held.Owner == caller {
		return false, nil
	}
	live, err := ownerLive(ctx, herdr, held.Owner)
	if err != nil {
		return false, err
	}
	if live {
		return false, ErrLockHeld
	}
	if s.betweenStaleReadAndLock != nil {
		s.betweenStaleReadAndLock()
	}
	// The exclusion is held for the whole takeover, so the lock read and
	// judged below is the one that is replaced.
	excl, err := cachelock.Acquire(s.dir)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrLockHeld, err)
	}
	defer func() { _ = excl.Close() }()
	held, err = s.readLock()
	if err != nil {
		return false, err
	}
	if held.Owner != caller {
		live, err := ownerLive(ctx, herdr, held.Owner)
		if err != nil {
			return false, err
		}
		if live {
			return false, ErrLockHeld
		}
		if err := privatefile.Remove(filepath.Join(s.dir, lockFile)); err != nil {
			return false, err
		}
		if err := s.create(lockFile, rec); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// readLock reads the lock; a lock that cannot be read is not taken over,
// since its owner is unknown.
func (s Store) readLock() (lockRecord, error) {
	var rec lockRecord
	if err := s.read(lockFile, &rec); err != nil {
		return lockRecord{}, fmt.Errorf("%w: %w", ErrLockUnverifiable, err)
	}
	if rec.Pane == "" || rec.Session == "" {
		return lockRecord{}, fmt.Errorf("%w: the lock names no owner", ErrLockUnverifiable)
	}
	return rec, nil
}

// ownerLive reports whether the pane of owner exists in Herdr with the
// recorded session. A pane that is gone, or one with another session (after
// /clear), is not live. Herdr unreadable is an error, never "not live".
func ownerLive(ctx context.Context, herdr Herdr, owner Owner) (bool, error) {
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrLockUnverifiable, err)
	}
	for _, a := range agents {
		if a.Pane.String() == owner.Pane {
			return a.Session == owner.Session, nil
		}
	}
	return false, nil
}

// Require checks that start ran and caller is its owner, for every
// subcommand but start.
func (s Store) Require(caller Owner) error {
	var rec lockRecord
	err := s.read(lockFile, &rec)
	switch {
	case errors.Is(err, privatefile.ErrNotExist):
		return ErrNotStarted
	case err != nil:
		return fmt.Errorf("%w: %w", ErrLockUnverifiable, err)
	case rec.Owner != caller:
		return ErrNotOwner
	}
	return nil
}

// hashName returns the hex of the first 16 bytes of the SHA-256 of s, for a
// record's file name.
func hashName(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

// noticeName is the file name of the notice record of item in session.
func noticeName(session string, item ItemID) string {
	return noticePrefix + hashName(session) + "-" + hashName(string(item)) + recordSuffix
}

// noticeRecord is the content of a notice record.
type noticeRecord struct {
	Session   string    `json:"session"`
	Item      ItemID    `json:"item"`
	FirstSeen time.Time `json:"first_seen"`
}

// Observe records that item was seen in session. It returns when the item
// was first seen and whether this call is the first. Contract: an existing
// record is never rewritten, so the first time stays; a record that cannot
// be created gives now and isNew true (the item is shown, possibly again,
// rather than lost), and a record that exists but cannot be read gives now
// and isNew false.
func (s Store) Observe(session string, item ItemID, now time.Time) (first time.Time, isNew bool, err error) {
	name := noticeName(session, item)
	var rec noticeRecord
	err = s.read(name, &rec)
	if err == nil {
		return rec.FirstSeen, false, nil
	}
	if !errors.Is(err, privatefile.ErrNotExist) {
		return now, false, err
	}
	if err := s.create(name, noticeRecord{Session: session, Item: item, FirstSeen: now.UTC()}); err != nil {
		if errors.Is(err, privatefile.ErrExist) {
			// Another chief process made it first; read what it wrote.
			if rerr := s.read(name, &rec); rerr == nil {
				return rec.FirstSeen, false, nil
			}
		}
		return now, true, err
	}
	return now, true, nil
}

// removeFinishedNotices removes the notice records of sessions other than
// session. A name that is not a notice record, and a record of the current
// session, are left alone.
func (s Store) removeFinishedNotices(session string) (int, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}
	keep := noticePrefix + hashName(session) + "-"
	removed := 0
	var errs []error
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, noticePrefix) || !strings.HasSuffix(name, recordSuffix) || strings.HasPrefix(name, keep) {
			continue
		}
		if err := privatefile.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, privatefile.ErrNotExist) {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}

// CanonicalWorktree returns the worktree that contains dir as an absolute
// path with its symbolic links resolved. Contract: the same worktree reached
// through a link (macOS's /private among them) gives one path, so a record
// kept under its name is found again.
func CanonicalWorktree(ctx context.Context, cmds Commands, dir string) (string, error) {
	out, err := cmds.git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top := strings.TrimSpace(string(out))
	if !filepath.IsAbs(top) {
		return "", fmt.Errorf("%w: git rev-parse --show-toplevel gave %q", errGitOutput, top)
	}
	return filepath.EvalSymlinks(top)
}

// WorkspaceDir returns the directory of the records of one worktree, creating
// it. The name is the hex SHA-256 of the canonical path, so a record and the
// exclusion kept in it follow the worktree and not the way it was reached.
func (s Store) WorkspaceDir(canonical string) (string, error) {
	sum := sha256.Sum256([]byte(canonical))
	name := hex.EncodeToString(sum[:])
	if err := privatefile.MkdirBelow(s.dir, []string{name}); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, name), nil
}
