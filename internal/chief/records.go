package chief

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// recordLimit caps a record read. The largest record is a PR detail, whose
// text is capped at 64 KiB; JSON escapes a character into at most six
// bytes, so the cap leaves room for a text of only escaped characters.
const recordLimit = 512 << 10

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
	// beforeCreate runs before each record is created and fails the
	// creation when it returns an error; the send tests use it to see the
	// order of a send's records and to make one fail. It is nil outside the
	// tests.
	beforeCreate func(name string) error
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
	if s.beforeCreate != nil {
		if err := s.beforeCreate(name); err != nil {
			return err
		}
	}
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
	if dec.Decode(&struct{}{}) != io.EOF {
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
	// CleanupError is why some finished session's notice record could not be
	// removed; the lock is taken regardless.
	CleanupError error
	// Removed counts the notice records of finished sessions that were
	// removed.
	Removed int
	// Stopped counts the sends found without an end, each of which stopped
	// its workspace.
	Stopped int
	// RemovedSends counts the send and detail records removed.
	RemovedSends int
}

// Start takes the chief lock for caller and cleans up records of finished
// sessions. Contract: the lock is created, never overwritten; a lock whose
// owner is another live chief is ErrLockHeld; a stale lock is replaced only
// while the exclusion of the state directory is held, and it is judged stale
// again under that exclusion, so two chiefs that start at once cannot both
// win; Herdr unreadable is ErrLockUnverifiable. After the lock, every send
// left without an end becomes a stop of its workspace before any record is
// removed.
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
	stopped, removedSends, settleErr := s.settleWorkspaces(now)
	res.Stopped, res.RemovedSends = stopped, removedSends
	removed, err := s.removeFinishedNotices(caller.Session)
	res.Removed = removed
	// A record that cannot be settled or removed does not undo the lock; it
	// is reported and left for the human.
	res.CleanupError = errors.Join(settleErr, err)
	return res, nil
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
	if errors.Is(err, cachelock.ErrLocked) {
		return false, fmt.Errorf("%w: %w", ErrLockHeld, err)
	}
	if err != nil {
		return false, err
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
			if errors.Is(err, privatefile.ErrExist) {
				// Another chief created its lock first.
				return false, ErrLockHeld
			}
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

// noticeFileName is the exact form noticeName makes, so a file of another
// kind in the directory is never taken for a notice record.
var noticeFileName = regexp.MustCompile(`^notice-[0-9a-f]{32}-[0-9a-f]{32}\.json$`)

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
	now = now.UTC()
	if !errors.Is(err, privatefile.ErrNotExist) {
		return now, false, err
	}
	if err := s.create(name, noticeRecord{Session: session, Item: item, FirstSeen: now.UTC()}); err != nil {
		if errors.Is(err, privatefile.ErrExist) {
			// Another chief process made it first; read what it wrote.
			if rerr := s.read(name, &rec); rerr != nil {
				return now, false, rerr
			}
			return rec.FirstSeen, false, nil
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
		if !noticeFileName.MatchString(name) || strings.HasPrefix(name, keep) {
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

// Worktree is a worktree path made canonical by CanonicalWorktree: absolute,
// with links resolved. Only CanonicalWorktree makes one, so a record keyed by
// a worktree cannot be looked up with a path that was never made canonical.
type Worktree struct {
	path string
}

// String returns the path.
func (w Worktree) String() string { return w.path }

// CanonicalWorktree returns the worktree that contains dir as an absolute
// path with its symbolic links resolved. Contract: the same worktree reached
// through a link (macOS's /private among them) gives one path, so a record
// kept under its name is found again.
func CanonicalWorktree(ctx context.Context, cmds Commands, dir string) (Worktree, error) {
	out, err := cmds.git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Worktree{}, err
	}
	top := strings.TrimSpace(string(out))
	if !filepath.IsAbs(top) {
		return Worktree{}, fmt.Errorf("%w: git rev-parse --show-toplevel gave %q", errGitOutput, top)
	}
	resolved, err := filepath.EvalSymlinks(top)
	if err != nil {
		return Worktree{}, err
	}
	return Worktree{path: resolved}, nil
}

// workspaceDirOf is the name of a worktree's directory: the hex SHA-256 of
// its canonical path.
func workspaceDirOf(w Worktree) string {
	sum := sha256.Sum256([]byte(w.path))
	return hex.EncodeToString(sum[:])
}

// WorkspaceDir returns the directory of the records of one worktree, creating
// it. The name is the hex SHA-256 of the canonical path, so a record and the
// exclusion kept in it follow the worktree and not the way it was reached.
func (s Store) WorkspaceDir(canonical Worktree) (string, error) {
	name := workspaceDirOf(canonical)
	if err := privatefile.MkdirBelow(s.dir, []string{name}); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, name), nil
}

// ErrWorkspaceBusy reports that another send holds the workspace's
// exclusion, so nothing is sent to it now.
var ErrWorkspaceBusy = errors.New("another send holds the workspace")

// Record file names in a workspace's directory.
const (
	stopFile     = "stop.json"
	sendPrefix   = "send-"
	startSuffix  = "-start.json"
	endSuffix    = "-end.json"
	detailPrefix = "detail-"
)

var (
	// sendFileName is a send record's name: the send's ID (the start time in
	// nanoseconds, then random hex, so names sort by time) and its half.
	sendFileName = regexp.MustCompile(`^send-([0-9]{20}-[0-9a-f]{16})-(start|end)\.json$`)
	// detailFileName is a PR detail record's name: the PR number and the
	// head commit.
	detailFileName = regexp.MustCompile(`^detail-([1-9][0-9]*)-([0-9a-f]{40})\.json$`)
	// workspaceDirName is the name WorkspaceDir gives a worktree's
	// directory.
	workspaceDirName = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// detailMaxAge is how long a PR detail record is kept; start removes older
// ones.
const detailMaxAge = 14 * 24 * time.Hour

// WorkspaceRecords is the directory of one worktree's records: its sends,
// its stop, and its PR details.
type WorkspaceRecords struct {
	dir string
	// beforeCreate is the store's test hook, carried to the records.
	beforeCreate func(name string) error
}

// Records returns the records of a worktree, creating their directory.
func (s Store) Records(w Worktree) (WorkspaceRecords, error) {
	dir, err := s.WorkspaceDir(w)
	if err != nil {
		return WorkspaceRecords{}, err
	}
	return WorkspaceRecords{dir: dir, beforeCreate: s.beforeCreate}, nil
}

// existingRecords returns the records of a worktree without creating
// anything: ok is false when the worktree has no directory yet.
func (s Store) existingRecords(w Worktree) (WorkspaceRecords, bool, error) {
	dir := filepath.Join(s.dir, workspaceDirOf(w))
	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return WorkspaceRecords{}, false, nil
	case err != nil:
		return WorkspaceRecords{}, false, err
	case !info.IsDir():
		return WorkspaceRecords{}, false, fmt.Errorf("%w: %s is not a directory", errRecord, dir)
	}
	return WorkspaceRecords{dir: dir, beforeCreate: s.beforeCreate}, true, nil
}

func (r WorkspaceRecords) store() Store { return Store{dir: r.dir, beforeCreate: r.beforeCreate} }

// HeldWorkspace is a worktree's records while its exclusion is held. Only
// Lock makes one, so a send record, a stop, or a release is written only
// under the exclusion.
type HeldWorkspace struct {
	WorkspaceRecords
	lock *cachelock.Lock
}

// Lock takes the worktree's exclusion without waiting. Contract: a held
// exclusion is ErrWorkspaceBusy and nothing else is done; the holder closes
// the result, which ends the exclusion.
func (r WorkspaceRecords) Lock() (*HeldWorkspace, error) {
	lock, err := cachelock.Acquire(r.dir)
	if errors.Is(err, cachelock.ErrLocked) {
		return nil, fmt.Errorf("%w: %w", ErrWorkspaceBusy, err)
	}
	if err != nil {
		return nil, err
	}
	return &HeldWorkspace{WorkspaceRecords: r, lock: lock}, nil
}

// Close ends the exclusion.
func (h *HeldWorkspace) Close() error { return h.lock.Close() }

// SendRecord is the start of one send: the pane, what is typed in order,
// and the agent and model it goes to.
type SendRecord struct {
	ID      string    `json:"id"`
	Pane    string    `json:"pane"`
	Input   string    `json:"input"`
	Command string    `json:"command"`
	Agent   string    `json:"agent"`
	Model   string    `json:"model"`
	Prompts []string  `json:"prompts"`
	Started time.Time `json:"started"`
}

// sendEnd is the end of one send.
type sendEnd struct {
	ID      string    `json:"id"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason"`
	Ended   time.Time `json:"ended"`
}

// StopRecord is why a workspace takes no input from chief until release:
// the reason, the tail of the pane's screen, and when.
type StopRecord struct {
	Reason string    `json:"reason"`
	Screen string    `json:"screen"`
	Send   string    `json:"send"`
	At     time.Time `json:"at"`
}

// unknownName is the output's name of any value that is not one of its
// kind's known values.
const unknownName = "unknown"

// outcomeUnknown is the end a send gets when its own end was never written:
// whether it was sent cannot be told.
const outcomeUnknown = unknownName

// newSendID returns an ID for a send started at now; IDs sort by time.
func newSendID(now time.Time) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%020d-%s", now.UnixNano(), hex.EncodeToString(b[:])), nil
}

// BeginSend writes the start of a send before its first input and returns
// its ID. Contract: nothing may be typed when it fails, since a send with no
// start record could not be found if the tool stopped halfway.
func (h *HeldWorkspace) BeginSend(rec SendRecord, now time.Time) (string, error) {
	id, err := newSendID(now)
	if err != nil {
		return "", err
	}
	rec.ID = id
	rec.Started = now.UTC()
	if err := h.store().create(sendPrefix+id+startSuffix, rec); err != nil {
		return "", err
	}
	return id, nil
}

// EndSend writes the end of the send id with its outcome.
func (h *HeldWorkspace) EndSend(id, outcome, reason string, now time.Time) error {
	return h.store().create(sendPrefix+id+endSuffix, sendEnd{ID: id, Outcome: outcome, Reason: reason, Ended: now.UTC()})
}

// Stop writes the workspace's stop. A stop that exists already is kept as it
// is: the first reason is the one the human needs.
func (h *HeldWorkspace) Stop(reason, screen, send string, now time.Time) error {
	err := h.store().create(stopFile, StopRecord{Reason: reason, Screen: screen, Send: send, At: now.UTC()})
	if errors.Is(err, privatefile.ErrExist) {
		return nil
	}
	return err
}

// Stopped returns the workspace's stop, or nil when it has none. A stop
// that cannot be read is an error, which the caller treats as stopped.
func (r WorkspaceRecords) Stopped() (*StopRecord, error) {
	var rec StopRecord
	err := r.store().read(stopFile, &rec)
	switch {
	case errors.Is(err, privatefile.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, err
	}
	return &rec, nil
}

// sendFiles lists the IDs of the sends with a start record, oldest first,
// and the set of IDs with an end record.
func (r WorkspaceRecords) sendFiles() (starts []string, ends map[string]struct{}, err error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, nil, err
	}
	ends = map[string]struct{}{}
	for _, e := range entries {
		m := sendFileName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if m[2] == "start" {
			starts = append(starts, m[1])
		} else {
			ends[m[1]] = struct{}{}
		}
	}
	return starts, ends, nil
}

// SettleUnfinished turns every send whose end was never written into a stop
// of the workspace and returns how many there were. The exclusion is held, so
// the process that wrote such a start has ended without its end: whether its
// input reached the pane cannot be told, and sending again could run the
// same command twice. The start record is kept, so the stop stays explained.
func (h *HeldWorkspace) SettleUnfinished(now time.Time) (int, error) {
	starts, ends, err := h.sendFiles()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range starts {
		if _, ok := ends[id]; ok {
			continue
		}
		n++
		if err := h.Stop("a send ended without its end record, so whether its input was sent is unknown", "", id, now); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Release lifts the workspace's stop at the human's choice. Contract: every
// send without an end first gets an end saying its outcome is unknown, so
// the same start does not stop the workspace again; when one of those ends
// cannot be written, the stop is kept.
func (h *HeldWorkspace) Release(now time.Time) (bool, error) {
	starts, ends, err := h.sendFiles()
	if err != nil {
		return false, err
	}
	for _, id := range starts {
		if _, ok := ends[id]; ok {
			continue
		}
		if err := h.EndSend(id, outcomeUnknown, "released by the human", now); err != nil && !errors.Is(err, privatefile.ErrExist) {
			return false, err
		}
	}
	err = privatefile.Remove(filepath.Join(h.dir, stopFile))
	switch {
	case errors.Is(err, privatefile.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// LastSend returns the newest send record to pane, or nil when there is
// none. It is used only to show what is running; it never decides a send.
func (r WorkspaceRecords) LastSend(pane string) (*SendRecord, error) {
	starts, _, err := r.sendFiles()
	if err != nil {
		return nil, err
	}
	for _, id := range slices.Backward(starts) {
		var rec SendRecord
		if err := r.store().read(sendPrefix+id+startSuffix, &rec); err != nil {
			return nil, err
		}
		if rec.Pane == pane {
			return &rec, nil
		}
	}
	return nil, nil
}

// cleanup removes the sends whose start and end are both written and the PR
// details older than detailMaxAge. A start without an end is never removed.
func (h *HeldWorkspace) cleanup(now time.Time) (int, error) {
	starts, ends, err := h.sendFiles()
	if err != nil {
		return 0, err
	}
	removed := 0
	var errs []error
	remove := func(name string) {
		if err := privatefile.Remove(filepath.Join(h.dir, name)); err != nil && !errors.Is(err, privatefile.ErrNotExist) {
			errs = append(errs, err)
			return
		}
		removed++
	}
	for _, id := range starts {
		if _, ok := ends[id]; !ok {
			continue
		}
		// The start goes first: an end left alone is harmless, while a
		// start left alone would stop the workspace at the next settle.
		before := removed
		remove(sendPrefix + id + startSuffix)
		if removed > before {
			remove(sendPrefix + id + endSuffix)
		}
	}
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		return removed, errors.Join(append(errs, err)...)
	}
	for _, e := range entries {
		if !detailFileName.MatchString(e.Name()) {
			continue
		}
		var rec detailRecord
		if err := h.store().read(e.Name(), &rec); err != nil {
			errs = append(errs, err)
			continue
		}
		if now.Sub(rec.Saved) > detailMaxAge {
			remove(e.Name())
		}
	}
	return removed, errors.Join(errs...)
}

// settleWorkspaces turns the unfinished sends of every worktree into stops,
// then removes the finished sends and old details. A worktree whose
// exclusion is held is skipped: a send is under way there.
func (s Store) settleWorkspaces(now time.Time) (stopped, removed int, err error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, 0, err
	}
	var errs []error
	for _, e := range entries {
		if !e.IsDir() || !workspaceDirName.MatchString(e.Name()) {
			continue
		}
		held, err := WorkspaceRecords{dir: filepath.Join(s.dir, e.Name()), beforeCreate: s.beforeCreate}.Lock()
		if errors.Is(err, ErrWorkspaceBusy) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		n, err := held.SettleUnfinished(now)
		stopped += n
		if err != nil {
			errs = append(errs, err)
		} else {
			// Only a settled workspace is cleaned: a send left unfinished
			// must become a stop before anything near it is removed.
			r, err := held.cleanup(now)
			removed += r
			if err != nil {
				errs = append(errs, err)
			}
		}
		if err := held.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return stopped, removed, errors.Join(errs...)
}
