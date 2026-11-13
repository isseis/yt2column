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
	"math"
	"os"
	"path/filepath"
	"strings"
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

// opCreate is the operation an *fs.PathError from Create names.
const opCreate = "create"

// opMkdir is the operation an *fs.PathError from MkdirBelow names.
const opMkdir = "mkdir"

// DirMode is the mode MkdirBelow sets on each directory it creates.
const DirMode fs.FileMode = 0o700

// mkdirUmask is the umask in force while MkdirBelow calls mkdir: it keeps
// every owner bit, so the owner can open what was created to set its mode,
// whatever the caller's umask.
const mkdirUmask = 0o077

var (
	// ErrNotExist reports that nothing is at the path.
	ErrNotExist = errors.New("the file does not exist")
	// ErrUnsafe reports a file that must not be read; *UnsafeError carries
	// the path and the reason.
	ErrUnsafe = errors.New("the file is not in the accepted form")
	// ErrTooLarge reports a file larger than the caller's limit.
	ErrTooLarge = errors.New("the file exceeds the size limit")
	// ErrExist reports that something, of any kind, is already at the path.
	ErrExist = errors.New("a file already exists at the path")
	// ErrBaseMissing reports that MkdirBelow's base does not exist or is not a
	// directory.
	ErrBaseMissing = errors.New("the base directory does not exist or is not a directory")

	// errLimitRange reports a limit passed to Read that is negative or so
	// large that limit+1 bytes cannot be counted.
	errLimitRange = errors.New("the size limit is out of range")
	// errBadName reports a name passed to MkdirBelow that is not one path
	// element.
	errBadName = errors.New("a directory name is not a single path element")
	// errReplaced reports a directory MkdirBelow created that was no longer
	// the user's directory when it was opened to set its mode.
	errReplaced = errors.New("the created directory was replaced before its mode was set")
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
		return fmt.Sprintf("%v: %s has mode %04o, not exactly %04o", ErrUnsafe, e.Path, UnixMode(e.Mode), UnixMode(Mode))
	}
	return fmt.Sprintf("%v: %s %s", ErrUnsafe, e.Path, e.Reason)
}

func (e *UnsafeError) Unwrap() error { return ErrUnsafe }

// UnixMode returns m's permission and special bits in the numeric form chmod
// takes (setuid 04000, setgid 02000, sticky 01000), which fs.FileMode does
// not use, so a caller can show UnsafeError.Mode as chmod writes it.
func UnixMode(m fs.FileMode) uint32 {
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
	// hookReadAfterOpen runs in Read after the open and before the fstat.
	hookReadAfterOpen
	// hookReadAfterFstat runs in Read after the fstat and before the read.
	hookReadAfterFstat
	// hookCreateAfterLstat runs in Create after the path's Lstat, with the path.
	hookCreateAfterLstat
	// hookCreateBeforeWrite runs in Create before the content is written, with
	// the temporary name; so do the next three.
	hookCreateBeforeWrite
	// hookCreateBeforeSync runs before the temporary file is synced.
	hookCreateBeforeSync
	// hookCreateBeforeLink runs before the temporary file is linked at the path.
	hookCreateBeforeLink
	// hookCreateBeforeRemoveTemp runs before the temporary name is removed.
	hookCreateBeforeRemoveTemp
	// hookMkdirAfterMkdir runs in MkdirBelow after each directory it creates,
	// with the directory.
	hookMkdirAfterMkdir
)

// hook runs at each hookPoint. A non-nil error is returned as the failure of
// the step that follows. Production code never replaces it; it does nothing.
var hook = func(hookPoint, string) error { return nil }

// check reports why info, as found for a file, may not be read, or ok when it
// may: a regular file, not a symbolic link, owned by euid, with exactly Mode.
// Read applies it both to the path's Lstat and to the open descriptor's fstat.
// The owner is checked before the mode, so a foreign file is reported as
// foreign: chmod, which a mode refusal suggests, cannot fix it.
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
	if limit < 0 || limit == math.MaxInt {
		return nil, errLimitRange
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
	if err := hook(hookReadAfterOpen, path); err != nil {
		return nil, err
	}
	info, err = file.Stat()
	if err != nil {
		return nil, err
	}
	if err := refusal(path, info); err != nil {
		return nil, err
	}
	if err := hook(hookReadAfterFstat, path); err != nil {
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

// LeftoverError reports a failure of Create after which the temporary file
// could not be removed either, so the caller can name the leftover whatever
// the failure was. It wraps the failure, so errors.Is still classifies it.
type LeftoverError struct {
	Err       error
	Temp      string
	RemoveErr error
}

func (e *LeftoverError) Error() string {
	return fmt.Sprintf("%v; the temporary file %s could not be removed: %v", e.Err, e.Temp, e.RemoveErr)
}

func (e *LeftoverError) Unwrap() error { return e.Err }

// tempPrefix is the start of the temporary name Create writes next to path:
// a dot, path's base name, and ".tmp-", so a leftover is hidden and the user
// can still find it by the file's own name.
func tempPrefix(path string) string {
	return "." + filepath.Base(path) + ".tmp-"
}

// Create puts a new file with exactly Mode at path, holding content. After a
// success it returns the temporary file's name when it could not remove it,
// or "".
//
// Contract: what is at path, of any kind, is never replaced or changed
// (ErrExist). The content is written and synced under a temporary name in the
// same directory and put at path with link(2), which fails on an existing
// name, so on failure nothing is at path, even after the process is killed.
// The mode is set on the descriptor, whatever the umask. The temporary name is
// removed on every return; when that fails, a success returns the name and a
// failure's error names it.
func Create(path string, content []byte) (leftoverTemp string, err error) {
	// The Lstat only chooses the report: an existing file in a directory the
	// user cannot write to is "exists", not "could not create". link(2) below
	// is what keeps an existing name unchanged.
	if _, err := os.Lstat(path); err == nil {
		return "", &fs.PathError{Op: opCreate, Path: path, Err: ErrExist}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := hook(hookCreateAfterLstat, path); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(path), tempPrefix(path)+"*")
	if err != nil {
		return "", err
	}
	temp := file.Name()
	err = place(file, path, content)
	if removeErr := removeTemp(temp); removeErr != nil {
		if err != nil {
			return "", &LeftoverError{Err: err, Temp: temp, RemoveErr: removeErr}
		}
		return temp, nil
	}
	return "", err
}

// place writes content to the open temporary file, closes it, and links it at
// path. The file is closed on every return.
func place(file *os.File, path string, content []byte) error {
	err := writeTemp(file, content)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := hook(hookCreateBeforeLink, file.Name()); err != nil {
		return err
	}
	if err := os.Link(file.Name(), path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return &fs.PathError{Op: opCreate, Path: path, Err: ErrExist}
		}
		return err
	}
	return nil
}

// writeTemp sets the temporary file's mode to exactly Mode, writes content,
// and syncs it.
func writeTemp(file *os.File, content []byte) error {
	if err := file.Chmod(Mode); err != nil {
		return err
	}
	if err := hook(hookCreateBeforeWrite, file.Name()); err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := hook(hookCreateBeforeSync, file.Name()); err != nil {
		return err
	}
	return file.Sync()
}

// removeTemp removes the temporary name.
func removeTemp(temp string) error {
	if err := hook(hookCreateBeforeRemoveTemp, temp); err != nil {
		return err
	}
	return os.Remove(temp)
}

// MkdirBelow creates base/names[0], base/names[0]/names[1], ... that do not
// exist yet, each with exactly DirMode. base must exist and be a directory,
// followed through a symbolic link (ErrBaseMissing); an existing element is
// left as it is. An existing last element that is not a directory is not
// reported here; creating a file below it fails instead.
//
// Contract: only a directory this call created has its mode changed, and its
// mode becomes exactly DirMode whatever the umask (a setgid bit copied from
// the parent included). The mode is set through a descriptor opened without
// following a link, so a created name replaced by anything else is reported
// and the replacement is not changed. The process umask is mkdirUmask only
// during each mkdir and is restored on every return, so the call must not run
// concurrently with anything else that creates files.
func MkdirBelow(base string, names []string) error {
	for _, name := range names {
		if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') {
			return errBadName
		}
	}
	info, err := os.Stat(base)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) || (err == nil && !info.IsDir()) {
		return &fs.PathError{Op: opMkdir, Path: base, Err: ErrBaseMissing}
	}
	if err != nil {
		return err
	}
	dir := base
	for _, name := range names {
		dir = filepath.Join(dir, name)
		created, err := mkdir(dir)
		if err != nil {
			return err
		}
		if created {
			if err := setDirMode(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

// mkdir creates dir under mkdirUmask and reports whether it created it; an
// existing name of any kind is not an error. The umask is restored as soon as
// mkdir returns.
func mkdir(dir string) (created bool, err error) {
	previous := syscall.Umask(mkdirUmask)
	defer syscall.Umask(previous)
	err = os.Mkdir(dir, DirMode)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	return err == nil, err
}

// setDirMode sets the directory just created at dir to exactly DirMode
// through a descriptor. O_DIRECTORY and O_NOFOLLOW make the open fail when the
// name is no longer a directory; the fstat checks that the directory opened
// is the effective user's.
func setDirMode(dir string) error {
	if err := hook(hookMkdirAfterMkdir, dir); err != nil {
		return err
	}
	file, err := os.OpenFile(dir, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0) //nolint:gosec // opens the directory this call just created, without following a link, to set its mode through the descriptor
	if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOTDIR) {
		return &fs.PathError{Op: opMkdir, Path: dir, Err: errReplaced}
	}
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	// Only root could chmod another user's directory swapped in here; no
	// non-root test can create one.
	if !ownedBy(info, os.Geteuid()) {
		return &fs.PathError{Op: opMkdir, Path: dir, Err: errReplaced}
	}
	return file.Chmod(DirMode)
}

// opRemove is the operation an *fs.PathError from Remove names.
const opRemove = "remove"

// Remove deletes the file at path when it is a regular file, not a symbolic
// link, owned by the effective user, with exactly Mode: the same shape Read
// accepts. Any other shape is refused with the *UnsafeError Read would give,
// and nothing is deleted.
//
// Contract: only the file this package's other functions create is removed;
// a link, a directory, a file of another owner, or a file of another mode is
// never touched. The shape is judged by the path's Lstat just before the
// unlink, so a name replaced between the two can still be removed; the
// directory is the user's own, which is the same trust Create relies on.
func Remove(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &fs.PathError{Op: opRemove, Path: path, Err: ErrNotExist}
	}
	if err != nil {
		return err
	}
	if err := refusal(path, info); err != nil {
		return err
	}
	return os.Remove(path)
}
