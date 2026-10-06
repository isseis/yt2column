//go:build unix

// Package cachelock serializes the runs that share one cache directory. The
// lock is an flock(2) on a fixed file in that directory, so a run can hand the
// file descriptor to yt-dlp and keep the lock while the child lives.
package cachelock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockFileName is the lock file. It must not match the cache's own naming rule
// (<video ID>.<a|b|current|current.tmp>), so pruning and removal never touch
// it.
const lockFileName = ".yt2column.lock"

// dirMode is the permission of a cache directory this package creates. An
// existing directory keeps its own mode.
const dirMode = 0o700

// fileMode is the permission of the lock file. It carries no content, only the
// lock, but it is created 0o600 like the rest of the cache.
const fileMode = 0o600

// ErrLocked reports that another run holds the cache directory lock. A failure
// for any other reason does not wrap it, so a caller can tell a concurrent run
// from an I/O or permission problem.
var ErrLocked = errors.New("another run holds the cache directory lock")

// errNotRegularFile reports a lock name that is not a regular file.
var errNotRegularFile = errors.New("the lock file is not a regular file")

// Lock is an exclusive flock(2) lock on a file in the cache directory.
//
// Contract: flock is attached to the open file description, so the lock is
// held until every descriptor that references it is closed, including a
// descriptor inherited by a child process. A run that dies without cleanup
// (even by SIGKILL) therefore keeps the lock while the yt-dlp it started still
// runs.
type Lock struct {
	file *os.File
}

// Acquire creates dir with mode 0o700 when it is missing and takes the lock
// without waiting.
//
// Contract: an existing directory's permission is never changed. The lock name
// is neither followed nor replaced: a symbolic link, a directory, a named pipe,
// or any other non-regular file there is a failure, and opening a named pipe
// does not block. A failure other than a held lock never wraps ErrLocked. The
// lock file is never deleted, so the next Acquire after the holder exits
// succeeds without manual cleanup.
func Acquire(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, fmt.Errorf("create the cache directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, lockFileName)
	// O_NOFOLLOW refuses a symbolic link at the name instead of locking its
	// target; O_NONBLOCK makes opening a named pipe return at once instead of
	// waiting for a writer. The type is still checked below, so the flags are
	// relied on only for the open itself.
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, fileMode) //nolint:gosec // opens the lock file inside the user's cache directory
	if err != nil {
		return nil, fmt.Errorf("open the lock file %s: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat the lock file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("the lock file %s: %w", path, errNotRegularFile)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil { //nolint:gosec // the file descriptor is a non-negative integer within int range
		_ = file.Close()
		return nil, lockError(path, err)
	}
	return &Lock{file: file}, nil
}

// lockError classifies a flock(2) failure for path. EWOULDBLOCK means another
// run holds the lock. Any other cause (no locks available, an invalid
// descriptor) is reported as it is, so a caller does not mistake an I/O
// failure for a concurrent run.
func lockError(path string, err error) error {
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return fmt.Errorf("%w: %s is held; if a previous run was killed while yt-dlp was running, check for it with lsof %s and wait for it or stop it before retrying", ErrLocked, path, path)
	}
	return fmt.Errorf("lock %s: %w", path, err)
}

// File returns the locked file so a child process can inherit the lock.
func (l *Lock) File() *os.File { return l.file }

// Close releases this process's reference to the lock. The lock lasts while
// any other descriptor, including one inherited by a child, still references
// it.
func (l *Lock) Close() error { return l.file.Close() }
