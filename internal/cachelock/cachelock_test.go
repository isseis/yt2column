//go:build test

package cachelock

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/transcript"
)

const (
	// acquireBound bounds how long a test waits for Acquire before reporting a
	// failure, so an implementation that waits in flock does not run until the
	// go test timeout.
	acquireBound = 5 * time.Second
	// lockHelperMarker selects the child mode of TestLockInheritedHelperProcess.
	lockHelperMarker = "YT2COLUMN_TEST_LOCK_HELPER"
	// lockHelperLifetime caps how long the helper process lives when nothing
	// kills it. It stays below the go test timeout, so a test that fails before
	// its cleanup can kill the child does not leave it running past the test
	// run.
	lockHelperLifetime = time.Minute
)

var errAcquireBound = errors.New("Acquire did not return within the bound")

// lockHelperEnv returns the minimal environment for the re-executed test
// binary: only the variables it needs, so the parent's secrets are not handed
// to a child process. extra entries are appended unchanged.
func lockHelperEnv(extra ...string) []string {
	env := make([]string, 0, 4)
	for _, name := range []string{"PATH", "HOME", "TMPDIR"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return append(env, extra...)
}

// lockPath returns the lock file path inside dir.
func lockPath(dir string) string { return filepath.Join(dir, lockFileName) }

// acquireWithin runs Acquire in a goroutine and gives up after limit. The
// goroutine leaks when Acquire blocks past the bound, which only happens for a
// broken implementation.
func acquireWithin(dir string, limit time.Duration) (*Lock, error) {
	type result struct {
		lock *Lock
		err  error
	}
	done := make(chan result, 1)
	go func() {
		lock, err := Acquire(dir)
		done <- result{lock: lock, err: err}
	}()
	select {
	case r := <-done:
		return r.lock, r.err
	case <-time.After(limit):
		return nil, errAcquireBound
	}
}

// assertMode asserts that path has the given permission bits.
func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestAcquireCreatesDirectory(t *testing.T) {
	t.Run("creates a missing directory with 0o700 and a 0o600 lock file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "cache")
		lock, err := Acquire(dir)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		defer func() { _ = lock.Close() }()

		assertMode(t, dir, 0o700)
		assertMode(t, lockPath(dir), 0o600)
	})
	t.Run("keeps an existing directory's mode", func(t *testing.T) {
		dir := t.TempDir()
		chmodForTest(t, dir, 0o755)
		lock, err := Acquire(dir)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		defer func() { _ = lock.Close() }()

		assertMode(t, dir, 0o755)
	})
}

func TestAcquireLocked(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	second, err := acquireWithin(dir, acquireBound)
	if second != nil {
		_ = second.Close()
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("second Acquire = %v, want ErrLocked", err)
	}
	locked, ok := errors.AsType[*LockedError](err)
	if !ok {
		t.Fatalf("second Acquire = %T, want *LockedError", err)
	}
	if want := lockPath(dir); locked.Path != want {
		t.Fatalf("LockedError.Path = %q, want %q", locked.Path, want)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	third, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire after the holder closed: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAcquireOtherDirectory(t *testing.T) {
	first, err := Acquire(t.TempDir())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = first.Close() }()

	second, err := Acquire(t.TempDir())
	if err != nil {
		t.Fatalf("Acquire on another directory: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAcquireAfterHolderGone(t *testing.T) {
	dir := t.TempDir()
	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(lockPath(dir)); err != nil {
		t.Fatalf("the lock file was removed: %v", err)
	}

	next, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire after the holder closed: %v", err)
	}
	if err := next.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAcquireOtherFailures(t *testing.T) {
	t.Run("parent is a regular file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "file")
		writeFile(t, parent, "x")

		got, err := Acquire(filepath.Join(parent, "cache"))
		if err == nil {
			_ = got.Close()
			t.Fatal("Acquire succeeded with a regular file as the parent")
		}
		if errors.Is(err, ErrLocked) {
			t.Fatalf("err = %v, must not wrap ErrLocked", err)
		}
	})
	t.Run("unwritable directory", func(t *testing.T) {
		requireNonRoot(t)
		dir := t.TempDir()
		chmodForTest(t, dir, 0o500)

		got, err := Acquire(dir)
		if err == nil {
			_ = got.Close()
			t.Fatal("Acquire succeeded in an unwritable directory")
		}
		if errors.Is(err, ErrLocked) {
			t.Fatalf("err = %v, must not wrap ErrLocked", err)
		}
	})
}

// TestLockError covers the flock(2) failure classification, whose non-held
// branch a real file cannot reach on demand.
func TestLockError(t *testing.T) {
	path := lockPath("/cache")

	held := lockError(path, syscall.EWOULDBLOCK)
	if !errors.Is(held, ErrLocked) {
		t.Fatalf("EWOULDBLOCK error = %v, want ErrLocked", held)
	}
	locked, ok := errors.AsType[*LockedError](held)
	if !ok {
		t.Fatalf("EWOULDBLOCK error = %T, want *LockedError", held)
	}
	if locked.Path != path {
		t.Fatalf("LockedError.Path = %q, want %q", locked.Path, path)
	}

	other := lockError(path, syscall.ENOLCK)
	if errors.Is(other, ErrLocked) {
		t.Fatalf("ENOLCK error = %v, must not wrap ErrLocked", other)
	}
	if _, ok := errors.AsType[*LockedError](other); ok {
		t.Fatalf("ENOLCK error = %T, must not be *LockedError", other)
	}
	if !errors.Is(other, syscall.ENOLCK) {
		t.Fatalf("ENOLCK error = %v, the cause is lost", other)
	}
}

func TestAcquireRejectsNonRegularLockFile(t *testing.T) {
	t.Run("symbolic link", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		writeFile(t, target, "x")
		if err := os.Symlink(target, lockPath(dir)); err != nil {
			t.Fatalf("symlink: %v", err)
		}

		got, err := Acquire(dir)
		if err == nil {
			_ = got.Close()
			t.Fatal("Acquire accepted a symbolic link at the lock name")
		}
		if errors.Is(err, ErrLocked) {
			t.Fatalf("err = %v, must not wrap ErrLocked", err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(lockPath(dir), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		got, err := Acquire(dir)
		if err == nil {
			_ = got.Close()
			t.Fatal("Acquire accepted a directory at the lock name")
		}
		if errors.Is(err, ErrLocked) {
			t.Fatalf("err = %v, must not wrap ErrLocked", err)
		}
	})
	t.Run("named pipe", func(t *testing.T) {
		dir := t.TempDir()
		fifo := lockPath(dir)
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Skipf("mkfifo: %v", err)
		}
		type result struct {
			lock *Lock
			err  error
		}
		done := make(chan result, 1)
		go func() {
			lock, err := Acquire(dir)
			done <- result{lock: lock, err: err}
		}()
		select {
		case r := <-done:
			if r.lock != nil {
				_ = r.lock.Close()
			}
			if r.err == nil {
				t.Fatal("Acquire accepted a named pipe at the lock name")
			}
			if errors.Is(r.err, ErrLocked) {
				t.Fatalf("err = %v, must not wrap ErrLocked", r.err)
			}
		case <-time.After(acquireBound):
			// Opening the write side releases a reader blocked in open, so
			// the goroutine ends before the test does.
			writer, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err != nil {
				t.Fatalf("Acquire did not return within %v on a named pipe with no writer, and opening the write side failed: %v", acquireBound, err)
			}
			_ = writer.Close()
			select {
			case <-done:
			case <-time.After(acquireBound):
			}
			t.Fatalf("Acquire did not return within %v on a named pipe with no writer", acquireBound)
		}
	})
}

func TestLockFileSurvivesPrune(t *testing.T) {
	dir := t.TempDir()
	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	source, err := transcript.NewYtDlpSource(transcript.Options{CacheDir: dir, Timeout: time.Minute})
	if err != nil {
		t.Fatalf("NewYtDlpSource: %v", err)
	}
	if err := source.PruneCache(context.Background()); err != nil {
		t.Fatalf("PruneCache: %v", err)
	}
	if _, err := os.Stat(lockPath(dir)); err != nil {
		t.Fatalf("the lock file was pruned: %v", err)
	}
}

func TestLockInheritedByChild(t *testing.T) {
	dir := t.TempDir()
	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	cmd := exec.Command(executable, "-test.run=TestLockInheritedHelperProcess") //nolint:gosec // re-executes the running test binary
	cmd.Env = lockHelperEnv(lockHelperMarker + "=1")
	cmd.ExtraFiles = []*os.File{lock.File()}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	waited := false
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		if !waited {
			_ = cmd.Wait()
		}
	})

	// The child inherited the descriptor for the same open file description,
	// so closing this process's reference must not release the lock.
	if err := lock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	second, err := Acquire(dir)
	if !errors.Is(err, ErrLocked) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatalf("Acquire while the child holds the lock = %v, want ErrLocked", err)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	// Wait returns the kill signal for a process this test killed on purpose.
	_ = cmd.Wait()
	waited = true

	acquired, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire after the child exited: %v", err)
	}
	if err := acquired.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestLockInheritedHelperProcess is not a regular test: TestLockInheritedByChild
// re-executes the test binary with the marker set, so the child keeps the
// inherited lock file descriptor open until it is killed. A normal test pass
// has no marker and skips it.
func TestLockInheritedHelperProcess(t *testing.T) {
	if os.Getenv(lockHelperMarker) != "1" {
		t.Skip("helper process for TestLockInheritedByChild")
	}
	// A timer keeps the runtime from reporting a deadlock in this
	// single-goroutine child; the cap keeps a leaked child from outliving the
	// test run.
	deadline := time.Now().Add(lockHelperLifetime)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
}
