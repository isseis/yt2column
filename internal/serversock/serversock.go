//go:build unix

// Package serversock prepares the server's Unix domain socket: it checks the
// socket directory and its parent, takes the start lock, removes a stale
// socket, binds the socket path, and checks the result. Nothing it finds in a
// shape it does not accept is repaired or removed.
package serversock

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/isseis/yt2column/internal/cachelock"
)

const (
	// dirMode is the exact mode of the socket directory. It is the main
	// defense against other users: without search permission they cannot
	// reach the socket, even while it briefly has a wider mode after bind.
	dirMode = 0o700
	// socketMode is the exact mode of the socket after bind, a second defense
	// in case the directory's mode is later widened.
	socketMode = 0o600
	// writableByOthers are the group and other write bits. The parent of the
	// socket directory must have neither, or another user could rename the
	// socket directory after it was checked.
	writableByOthers = 0o022
	// staleDialTimeout bounds the connection attempt that tells a stale socket
	// from a live one. A Unix socket without a listener refuses at once.
	staleDialTimeout = time.Second
)

// ErrAlreadyRunning reports that another server holds the start lock of the
// socket directory: it is running, or it is shutting down and waiting for its
// jobs.
var ErrAlreadyRunning = errors.New("another server holds the socket directory lock")

// ErrInUse reports that a process accepts connections on the socket path
// although no server holds the start lock. The path is left as it is.
var ErrInUse = errors.New("another process accepts connections on the socket path")

// ErrUnsafe reports that the socket directory, its parent, or the name at the
// socket path does not meet the accepted form. Nothing is repaired or removed.
// *UnsafeError carries the path and the fixed reason.
var ErrUnsafe = errors.New("the socket directory or the socket path is not in the accepted form")

// errRelativePath reports a socket path that is not absolute.
var errRelativePath = errors.New("the socket path is not absolute")

// errUncleanPath reports a socket path that filepath.Clean would change. The
// directories are checked through the cleaned path while bind resolves the
// path as given, so the two must be the same name.
var errUncleanPath = errors.New("the socket path is not in clean form")

// errReleased reports Listen on a Lock whose start lock was released.
var errReleased = errors.New("the start lock was released")

// specialBits are the mode bits other than the permission that an exact-mode
// check rejects.
const specialBits = fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

// UnsafeReason is one fixed reason; its String is the text shown to the user.
// The zero value names no specific reason, so an unset reason still reads as
// a rejection.
type UnsafeReason int

const (
	_ UnsafeReason = iota // the zero value: no specific reason
	// ReasonSymlink means the name is a symbolic link.
	ReasonSymlink
	// ReasonNotDirectory means the name is not a directory.
	ReasonNotDirectory
	// ReasonNotSocket means the name at the socket path is not a socket.
	ReasonNotSocket
	// ReasonForeignOwner means the owner is not the effective user.
	ReasonForeignOwner
	// ReasonWritableByOthers means the parent of the socket directory has a group
	// or other write bit.
	ReasonWritableByOthers
	// ReasonDirMode means the socket directory's mode is not exactly 0700.
	ReasonDirMode
	// ReasonSocketMode means the created socket's mode is not exactly 0600.
	ReasonSocketMode
	// ReasonSpecialBits means the socket directory or the created socket has
	// the setuid, setgid, or sticky bit (Linux copies a parent's setgid bit
	// to a new directory).
	ReasonSpecialBits
	// ReasonUndetermined means the connection attempt to an existing socket failed
	// with something other than a refusal, so it cannot be called stale.
	ReasonUndetermined
)

func (r UnsafeReason) String() string {
	switch r {
	case ReasonSymlink:
		return "is a symbolic link"
	case ReasonNotDirectory:
		return "is not a directory"
	case ReasonNotSocket:
		return "is not a socket"
	case ReasonForeignOwner:
		return "is not owned by the current user"
	case ReasonWritableByOthers:
		return "is writable by its group or by other users"
	case ReasonDirMode:
		return "does not have mode 0700"
	case ReasonSocketMode:
		return "does not have mode 0600"
	case ReasonSpecialBits:
		return "has the setuid, setgid, or sticky bit"
	case ReasonUndetermined:
		return "is a socket that cannot be confirmed stale"
	default:
		return "is not in the accepted form"
	}
}

// UnsafeError names the path and one fixed reason. It wraps ErrUnsafe.
type UnsafeError struct {
	Path   string
	Reason UnsafeReason
}

func (e *UnsafeError) Error() string {
	return fmt.Sprintf("%v: %s %s; nothing was repaired or removed, so fix it by hand", ErrUnsafe, e.Path, e.Reason)
}

func (e *UnsafeError) Unwrap() error { return ErrUnsafe }

// Lock is the held start lock of one socket directory. Listen is a method of
// Lock, so the socket cannot be bound without holding the lock.
//
// Contract: the lock is held until Release, and Listen fails after it. The
// caller keeps the *Lock reachable until then; it holds the lock file, whose
// finalizer would close the descriptor and drop the lock if the *Lock were
// lost.
type Lock struct {
	lock       *cachelock.Lock // nil after Release
	socketPath string
}

// Acquire prepares the socket directory of socketPath (creating it with mode
// 0o700 when missing), checks the directory and its parent, and takes the
// start lock without waiting.
//
// Contract: an existing directory's permission is never changed and nothing
// is removed. A parent that is a symbolic link, is not a directory, is owned
// by another user, or is writable by its group or others is an *UnsafeError;
// so is a socket directory that is a symbolic link, is not a directory, is
// owned by another user, or does not have exactly mode 0o700 without the
// setuid, setgid, or sticky bit. A held lock is ErrAlreadyRunning. The socket
// path must be absolute and in clean form.
func Acquire(socketPath string) (*Lock, error) {
	if !filepath.IsAbs(socketPath) {
		return nil, fmt.Errorf("%w: %s", errRelativePath, socketPath)
	}
	if filepath.Clean(socketPath) != socketPath {
		return nil, fmt.Errorf("%w: %s", errUncleanPath, socketPath)
	}
	dir := filepath.Dir(socketPath)
	parent := filepath.Dir(dir)
	euid := os.Geteuid()

	if _, err := os.Lstat(parent); errors.Is(err, fs.ErrNotExist) {
		// Missing ancestors (such as $HOME/.local/state for a new HOME) are
		// created by the same call; an existing ancestor keeps its mode.
		if err := os.MkdirAll(parent, dirMode); err != nil {
			return nil, fmt.Errorf("create %s: %w", parent, err)
		}
	}
	// The parent is checked even when it was just created, so a name put
	// there between the two calls is checked too.
	info, err := os.Lstat(parent)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", parent, err)
	}
	if err := checkParent(parent, info, euid); err != nil {
		return nil, err
	}

	// A umask that narrows the mode below 0o700 is caught by the check below,
	// not corrected.
	if err := os.Mkdir(dir, dirMode); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	info, err = os.Lstat(dir)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", dir, err)
	}
	if err := checkSocketDir(dir, info, euid); err != nil {
		return nil, err
	}

	lock, err := cachelock.Acquire(dir)
	if err != nil {
		// LockedError's text is about yt-dlp, so it is replaced, not wrapped.
		if locked, ok := errors.AsType[*cachelock.LockedError](err); ok {
			return nil, fmt.Errorf("%w: %s is held; check its holder with lsof %s", ErrAlreadyRunning, locked.Path, locked.Path)
		}
		return nil, fmt.Errorf("take the start lock in %s: %w", dir, err)
	}
	return &Lock{lock: lock, socketPath: socketPath}, nil
}

// Listen removes a stale socket, binds the socket path, sets it to 0o600, and
// checks the result. Closing the returned listener removes the socket.
//
// Contract: only a socket owned by the effective user whose connection
// attempt is refused is removed. A socket that accepts a connection is
// ErrInUse and is left as it is. Any other name at the path (a regular file,
// a directory, a symbolic link, a named pipe), a socket owned by another
// user, and a socket whose connection attempt fails otherwise are an
// *UnsafeError and are left as they are. The created socket must be a socket
// owned by the effective user with exactly mode 0o600, or the listener is
// closed and an *UnsafeError returned.
func (l *Lock) Listen() (net.Listener, error) {
	if l.lock == nil {
		return nil, errReleased
	}
	path := l.socketPath
	euid := os.Geteuid()
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	default:
		if err := removeStale(path, info, euid); err != nil {
			return nil, err
		}
	}

	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", path, err)
	}
	// From bind until here the socket may have the umask's wider mode; the
	// 0o700 directory keeps other users out meanwhile.
	if err := os.Chmod(path, socketMode); err != nil {
		abandon(listener, path, euid)
		return nil, fmt.Errorf("set the mode of %s: %w", path, err)
	}
	info, err = os.Lstat(path)
	if err != nil {
		abandon(listener, path, euid)
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	}
	if err := checkSocket(path, info, euid); err != nil {
		abandon(listener, path, euid)
		return nil, err
	}
	return listener, nil
}

// abandon closes listener after a failed check that follows bind. Closing
// unlinks the path by name, so the name is unlinked only while it still holds
// a socket of the effective user; anything put there after bind is left as it
// is.
func abandon(listener *net.UnixListener, path string, euid int) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Type() != fs.ModeSocket || !ownedBy(info, euid) {
		listener.SetUnlinkOnClose(false)
	}
	_ = listener.Close()
}

// Release releases the start lock. The caller calls it after every job and
// failure notice has finished. Listen fails after Release.
func (l *Lock) Release() error {
	if l.lock == nil {
		return errReleased
	}
	err := l.lock.Close()
	l.lock = nil
	return err
}

// removeStale removes the existing name at path when it is a stale socket of
// the effective user, and otherwise returns why it stays.
func removeStale(path string, info fs.FileInfo, euid int) error {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return &UnsafeError{Path: path, Reason: ReasonSymlink}
	case info.Mode().Type() != fs.ModeSocket:
		return &UnsafeError{Path: path, Reason: ReasonNotSocket}
	case !ownedBy(info, euid):
		return &UnsafeError{Path: path, Reason: ReasonForeignOwner}
	}
	conn, err := net.DialTimeout("unix", path, staleDialTimeout)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("%w: %s", ErrInUse, path)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return &UnsafeError{Path: path, Reason: ReasonUndetermined}
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove the stale socket %s: %w", path, err)
	}
	return nil
}

// checkParent accepts a directory, not a symbolic link, owned by euid,
// without group or other write bits.
func checkParent(path string, info fs.FileInfo, euid int) error {
	if err := checkOwnedDir(path, info, euid); err != nil {
		return err
	}
	if info.Mode().Perm()&writableByOthers != 0 {
		return &UnsafeError{Path: path, Reason: ReasonWritableByOthers}
	}
	return nil
}

// checkSocketDir accepts a directory, not a symbolic link, owned by euid,
// with exactly mode 0o700 and no special bits.
func checkSocketDir(path string, info fs.FileInfo, euid int) error {
	if err := checkOwnedDir(path, info, euid); err != nil {
		return err
	}
	if info.Mode().Perm() != dirMode {
		return &UnsafeError{Path: path, Reason: ReasonDirMode}
	}
	if info.Mode()&specialBits != 0 {
		return &UnsafeError{Path: path, Reason: ReasonSpecialBits}
	}
	return nil
}

// checkOwnedDir accepts a directory, not a symbolic link, owned by euid.
func checkOwnedDir(path string, info fs.FileInfo, euid int) error {
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return &UnsafeError{Path: path, Reason: ReasonSymlink}
	case !info.IsDir():
		return &UnsafeError{Path: path, Reason: ReasonNotDirectory}
	case !ownedBy(info, euid):
		return &UnsafeError{Path: path, Reason: ReasonForeignOwner}
	}
	return nil
}

// checkSocket accepts a socket owned by euid with exactly mode 0o600 and no
// special bits.
func checkSocket(path string, info fs.FileInfo, euid int) error {
	switch {
	case info.Mode().Type() != fs.ModeSocket:
		return &UnsafeError{Path: path, Reason: ReasonNotSocket}
	case !ownedBy(info, euid):
		return &UnsafeError{Path: path, Reason: ReasonForeignOwner}
	case info.Mode().Perm() != socketMode:
		return &UnsafeError{Path: path, Reason: ReasonSocketMode}
	case info.Mode()&specialBits != 0:
		return &UnsafeError{Path: path, Reason: ReasonSpecialBits}
	}
	return nil
}

// ownedBy reports whether info's owner is euid. An owner that cannot be read
// is treated as foreign.
func ownedBy(info fs.FileInfo, euid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && euid >= 0 && uint64(stat.Uid) == uint64(euid)
}
