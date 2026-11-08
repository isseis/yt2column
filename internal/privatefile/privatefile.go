//go:build unix

// Package privatefile reads and creates files that only their owner may read:
// a regular file, owned by the effective user, with exactly mode 0600. It
// knows nothing about what a file holds or where it lives; the caller passes
// the path and the bytes. Every shape it does not accept is refused, never
// repaired.
//
// MkdirBelow changes the process umask while it creates a directory, so it
// must not be called concurrently with anything else that creates files.
package privatefile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// Mode is the only mode Read accepts and Create sets: 0600, without the
// setuid, setgid, or sticky bit.
const Mode fs.FileMode = 0o600

// specialBits are the mode bits besides the permission that the exact-mode
// check rejects.
const specialBits = fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

// opRead is the operation an *fs.PathError from Read names.
const opRead = "read"

var (
	// ErrNotExist reports that nothing is at the path.
	ErrNotExist = errors.New("the file does not exist")
	// ErrUnsafe reports a file that must not be read; *UnsafeError carries
	// the path and the reason.
	ErrUnsafe = errors.New("the file is not in the accepted form")
	// ErrTooLarge reports a file larger than the caller's limit.
	ErrTooLarge = errors.New("the file exceeds the size limit")

	// errNegativeLimit reports a negative limit passed to Read.
	errNegativeLimit = errors.New("the size limit is negative")
)

// UnsafeReason is why Read refused a file; its String is the text shown to
// the user. The zero value, ReasonUnknown, names no specific reason, so an
// unset reason still reads as a refusal.
type UnsafeReason int

const (
	// ReasonUnknown names no specific reason.
	ReasonUnknown UnsafeReason = iota
	// ReasonSymlink means the name is a symbolic link.
	ReasonSymlink
	// ReasonNotRegular means the name is not a regular file.
	ReasonNotRegular
	// ReasonForeignOwner means the owner is not the effective user.
	ReasonForeignOwner
	// ReasonMode means the mode is not exactly Mode: another permission bit,
	// a missing one, or the setuid, setgid, or sticky bit.
	ReasonMode
)

func (r UnsafeReason) String() string {
	switch r {
	case ReasonSymlink:
		return "is a symbolic link"
	case ReasonNotRegular:
		return "is not a regular file"
	case ReasonForeignOwner:
		return "is owned by another user"
	case ReasonMode:
		return "does not have exactly mode 0600"
	default:
		return "is not in the accepted form"
	}
}

// UnsafeError names the path and the reason. Mode is the file's mode as found
// (its permission and special bits); it is set only for ReasonMode and is not
// secret. It wraps ErrUnsafe.
type UnsafeError struct {
	Path   string
	Reason UnsafeReason
	Mode   fs.FileMode
}

func (e *UnsafeError) Error() string {
	if e.Reason == ReasonMode {
		return fmt.Sprintf("%v: %s has mode %04o, not exactly %04o", ErrUnsafe, e.Path, unixMode(e.Mode), unixMode(Mode))
	}
	return fmt.Sprintf("%v: %s %s", ErrUnsafe, e.Path, e.Reason)
}

func (e *UnsafeError) Unwrap() error { return ErrUnsafe }

// unixMode returns m's permission and special bits in the numeric form chmod
// takes (setuid 04000, setgid 02000, sticky 01000), which fs.FileMode does
// not use.
func unixMode(m fs.FileMode) uint32 {
	n := uint32(m.Perm())
	if m&fs.ModeSetuid != 0 {
		n |= syscall.S_ISUID
	}
	if m&fs.ModeSetgid != 0 {
		n |= syscall.S_ISGID
	}
	if m&fs.ModeSticky != 0 {
		n |= syscall.S_ISVTX
	}
	return n
}

// hookPoint names a place where a test acts on the file system or injects a
// failure between two steps.
type hookPoint int

const (
	_ hookPoint = iota
	// hookReadAfterLstat runs in Read after the path's Lstat, with the path.
	hookReadAfterLstat
)

// hook runs at each hookPoint. A non-nil error is returned as the failure of
// the step that follows. Production code never replaces it; it does nothing.
var hook = func(hookPoint, string) error { return nil }

// check reports why info, as found for a file, may not be read, or ok when it
// may: a regular file, not a symbolic link, owned by euid, with exactly Mode.
// Read applies it both to the path's Lstat and to the open descriptor's fstat.
func check(info fs.FileInfo, euid int) (reason UnsafeReason, ok bool) {
	mode := info.Mode()
	switch {
	case mode&fs.ModeSymlink != 0:
		return ReasonSymlink, false
	case !mode.IsRegular():
		return ReasonNotRegular, false
	case !ownedBy(info, euid):
		return ReasonForeignOwner, false
	case mode&(fs.ModePerm|specialBits) != Mode:
		return ReasonMode, false
	}
	return ReasonUnknown, true
}

// ownedBy reports whether info's owner is euid. An owner that cannot be read
// is treated as foreign.
func ownedBy(info fs.FileInfo, euid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && euid >= 0 && uint64(stat.Uid) == uint64(euid)
}

// refusal returns the *UnsafeError for info at path, or nil when info may be
// read.
func refusal(path string, info fs.FileInfo) error {
	reason, ok := check(info, os.Geteuid())
	if ok {
		return nil
	}
	unsafe := &UnsafeError{Path: path, Reason: reason}
	if reason == ReasonMode {
		unsafe.Mode = info.Mode() & (fs.ModePerm | specialBits)
	}
	return unsafe
}

// Read returns the content of the file at path when it is a regular file, not
// a symbolic link, owned by the effective user, with exactly Mode. A file
// larger than limit bytes is ErrTooLarge.
//
// Contract: only the open descriptor's fstat can allow the read; the path's
// Lstat can only refuse, so a name replaced between the two is judged by what
// was opened. The checks and the read use one descriptor, which is closed on
// every return. At most limit+1 bytes are read, and opening never blocks on a
// named pipe or makes a terminal the controlling terminal.
func Read(path string, limit int) ([]byte, error) {
	if limit < 0 {
		return nil, errNegativeLimit
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &fs.PathError{Op: opRead, Path: path, Err: ErrNotExist}
	}
	if err != nil {
		return nil, err
	}
	// The Lstat result refuses early, so a named pipe or a socket is never
	// opened and a mode 0000 file reads as a mode error, not as unreadable.
	if err := refusal(path, info); err != nil {
		return nil, err
	}
	if err := hook(hookReadAfterLstat, path); err != nil {
		return nil, err
	}
	// O_NOFOLLOW fails on a symbolic link put at the name after the Lstat,
	// O_NONBLOCK returns at once for a named pipe, and O_NOCTTY keeps a
	// terminal from becoming the controlling terminal before fstat refuses it.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0) //nolint:gosec // the caller's path is opened without following a link and checked through the descriptor before any read
	switch {
	case errors.Is(err, syscall.ELOOP):
		return nil, &UnsafeError{Path: path, Reason: ReasonSymlink}
	case errors.Is(err, fs.ErrNotExist):
		return nil, &fs.PathError{Op: opRead, Path: path, Err: ErrNotExist}
	case err != nil:
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil {
		return nil, err
	}
	if err := refusal(path, info); err != nil {
		return nil, err
	}
	content, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(content) > limit {
		return nil, &fs.PathError{Op: opRead, Path: path, Err: ErrTooLarge}
	}
	return content, nil
}
