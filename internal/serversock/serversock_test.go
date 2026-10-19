//go:build test && unix

package serversock

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cachelock"
	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/udstest"
)

// layout is a socket path and the directories above it inside a short
// temporary HOME. parent's own parent does not exist yet, so Acquire has to
// create an ancestor too.
type layout struct {
	home, parent, dir, socket string
}

func newLayout(t *testing.T) layout {
	t.Helper()
	home := udstest.Dir(t)
	parent := filepath.Join(home, "state", "app")
	dir := filepath.Join(parent, "run")
	return layout{home: home, parent: parent, dir: dir, socket: filepath.Join(dir, "server.sock")}
}

// mkdirMode creates path with exactly mode, whatever the umask.
func mkdirMode(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// acquire calls Acquire and releases the lock at cleanup.
func acquire(t *testing.T, socketPath string) *Lock {
	t.Helper()
	lock, err := Acquire(socketPath)
	if err != nil {
		t.Fatalf("Acquire(%s): %v", socketPath, err)
	}
	t.Cleanup(func() { _ = lock.Release() })
	return lock
}

// listen calls Listen and closes the listener at cleanup.
func listen(t *testing.T, lock *Lock) net.Listener {
	t.Helper()
	listener, err := lock.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

// staleSocket leaves a socket file at path with nothing listening on it, as a
// killed server does.
func staleSocket(t *testing.T, path string) {
	t.Helper()
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
}

func lstat(t *testing.T, path string) fs.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// requireEmpty checks that the directory at path has no entries.
func requireEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Errorf("%s contains %s, want it empty", path, entry.Name())
	}
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(%s): error = %v, want fs.ErrNotExist", path, err)
	}
}

// requireUnsafe checks that err is an *UnsafeError for path with reason.
func requireUnsafe(t *testing.T, err error, path string, reason UnsafeReason) {
	t.Helper()
	if !errors.Is(err, ErrUnsafe) {
		t.Fatalf("error = %v, want ErrUnsafe", err)
	}
	unsafe, ok := errors.AsType[*UnsafeError](err)
	if !ok {
		t.Fatalf("error = %v, want *UnsafeError", err)
	}
	if unsafe.Path != path || unsafe.Reason != reason {
		t.Errorf("UnsafeError = {%s, %v}, want {%s, %v}", unsafe.Path, unsafe.Reason, path, reason)
	}
}

// TestAcquireCreatesDirectories checks that missing directories are created,
// the socket directory with exactly mode 0700, and that an existing parent
// without group or other write bits is accepted even when it is not 0700.
func TestAcquireCreatesDirectories(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		l := newLayout(t)
		acquire(t, l.socket)
		if mode := lstat(t, l.dir).Mode(); mode != fs.ModeDir|0o700 {
			t.Errorf("socket directory mode = %v, want %v", mode, fs.ModeDir|0o700)
		}
		if mode := lstat(t, l.parent).Mode(); !mode.IsDir() || mode.Perm()&0o077 != 0 {
			t.Errorf("parent mode = %v, want a directory closed to group and others", mode)
		}
	})
	t.Run("existing parent 0755", func(t *testing.T) {
		l := newLayout(t)
		mkdirMode(t, l.parent, 0o755)
		acquire(t, l.socket)
		if mode := lstat(t, l.parent).Mode(); mode != fs.ModeDir|0o755 {
			t.Errorf("parent mode = %v, want it unchanged at %v", mode, fs.ModeDir|0o755)
		}
	})
}

// TestAcquireRejectsUnsafeDirectories checks that a socket directory or parent
// in a rejected shape stops Acquire with the reason, that its mode and type
// are left as they were, and that nothing is created below it.
func TestAcquireRejectsUnsafeDirectories(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares the layout and returns the rejected path.
		setup  func(t *testing.T, l layout) string
		reason UnsafeReason
	}{
		{
			name: "socket directory 0711",
			setup: func(t *testing.T, l layout) string {
				mkdirMode(t, l.dir, 0o711)
				return l.dir
			},
			reason: ReasonDirMode,
		},
		{
			name: "socket directory is a symlink",
			setup: func(t *testing.T, l layout) string {
				target := filepath.Join(l.home, "elsewhere")
				mkdirMode(t, target, 0o700)
				mkdirMode(t, l.parent, 0o700)
				if err := os.Symlink(target, l.dir); err != nil {
					t.Fatal(err)
				}
				return l.dir
			},
			reason: ReasonSymlink,
		},
		{
			name: "socket directory is a regular file",
			setup: func(t *testing.T, l layout) string {
				mkdirMode(t, l.parent, 0o700)
				if err := os.WriteFile(l.dir, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				return l.dir
			},
			reason: ReasonNotDirectory,
		},
		{
			name: "parent writable by group",
			setup: func(t *testing.T, l layout) string {
				mkdirMode(t, l.parent, 0o770)
				return l.parent
			},
			reason: ReasonWritableByOthers,
		},
		{
			name: "parent writable by others",
			setup: func(t *testing.T, l layout) string {
				mkdirMode(t, l.parent, 0o707)
				return l.parent
			},
			reason: ReasonWritableByOthers,
		},
		{
			name: "parent is a symlink",
			setup: func(t *testing.T, l layout) string {
				target := filepath.Join(l.home, "elsewhere")
				mkdirMode(t, target, 0o700)
				mkdirMode(t, filepath.Dir(l.parent), 0o700)
				if err := os.Symlink(target, l.parent); err != nil {
					t.Fatal(err)
				}
				return l.parent
			},
			reason: ReasonSymlink,
		},
		{
			name: "parent is a regular file",
			setup: func(t *testing.T, l layout) string {
				mkdirMode(t, filepath.Dir(l.parent), 0o700)
				if err := os.WriteFile(l.parent, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				return l.parent
			},
			reason: ReasonNotDirectory,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayout(t)
			rejected := tc.setup(t, l)
			before := lstat(t, rejected)

			lock, err := Acquire(l.socket)
			if err == nil {
				_ = lock.Release()
			}
			requireUnsafe(t, err, rejected, tc.reason)

			after := lstat(t, rejected)
			if after.Mode() != before.Mode() || !os.SameFile(before, after) {
				t.Errorf("%s changed: mode %v -> %v", rejected, before.Mode(), after.Mode())
			}
			// Nothing is created below a rejected directory, and a symlink is
			// not followed to create anything in its target.
			if after.Mode().IsDir() {
				requireEmpty(t, rejected)
			}
			if after.Mode()&fs.ModeSymlink != 0 {
				requireEmpty(t, filepath.Join(l.home, "elsewhere"))
			}
		})
	}
}

// TestAcquireRejectsPathForm checks that a relative socket path and one that
// filepath.Clean would change are refused before anything is created, since
// the checks use the cleaned directories while bind resolves the path as
// given.
func TestAcquireRejectsPathForm(t *testing.T) {
	home := udstest.Dir(t)
	t.Chdir(home)
	tests := []struct {
		name, path string
		want       error
	}{
		{"relative", filepath.Join("app", "run", "server.sock"), errRelativePath},
		{"dot-dot", home + "/x/../app/run/server.sock", errUncleanPath},
		{"trailing slash", home + "/app/run/server.sock/", errUncleanPath},
		{"double slash", home + "/app//run/server.sock", errUncleanPath},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lock, err := Acquire(tc.path)
			if err == nil {
				_ = lock.Release()
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("Acquire(%q) error = %v, want %v", tc.path, err, tc.want)
			}
			requireEmpty(t, home)
		})
	}
}

// fakeInfo is an fs.FileInfo with a chosen mode and owner, so a check can be
// given a file owned by another user without root.
type fakeInfo struct {
	mode fs.FileMode
	uid  uint32
}

func (f fakeInfo) Name() string       { return "fake" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return &syscall.Stat_t{Uid: f.uid} }

// TestChecksRejectForeignOwner checks every owner check with an Lstat result
// in the accepted shape except for its owner, and the same result owned by
// the effective user as the accepted case.
func TestChecksRejectForeignOwner(t *testing.T) {
	euid := os.Geteuid()
	own := uint32(euid) //nolint:gosec // the effective user ID is a non-negative uid_t
	const path = "/home/u/app/run/server.sock"
	checks := []struct {
		name  string
		check func(path string, info fs.FileInfo, euid int) error
		mode  fs.FileMode
	}{
		{"parent", checkParent, fs.ModeDir | 0o755},
		{"socket directory", checkSocketDir, fs.ModeDir | 0o700},
		{"created socket", checkSocket, fs.ModeSocket | 0o600},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.check(path, fakeInfo{mode: tc.mode, uid: own}, euid); err != nil {
				t.Errorf("owned by the effective user: error = %v, want nil", err)
			}
			requireUnsafe(t, tc.check(path, fakeInfo{mode: tc.mode, uid: own + 1}, euid), path, ReasonForeignOwner)
		})
	}
	t.Run("existing socket", func(t *testing.T) {
		// The path does not exist, so a check that went on to dial or remove
		// would report a different error.
		requireUnsafe(t, removeStale(path, fakeInfo{mode: fs.ModeSocket | 0o600, uid: own + 1}, euid), path, ReasonForeignOwner)
	})
}

// TestExactModeChecks checks that the socket directory must have exactly
// 0700 and the created socket exactly 0600, both without the setuid, setgid,
// or sticky bit, and that the created socket must be a socket. A setgid directory is what Linux creates under a setgid
// parent.
func TestExactModeChecks(t *testing.T) {
	euid := os.Geteuid()
	own := uint32(euid) //nolint:gosec // the effective user ID is a non-negative uid_t
	const path = "/home/u/app/run/server.sock"
	tests := []struct {
		name   string
		check  func(path string, info fs.FileInfo, euid int) error
		mode   fs.FileMode
		reason UnsafeReason
	}{
		{"directory 0750", checkSocketDir, fs.ModeDir | 0o750, ReasonDirMode},
		{"directory 0500", checkSocketDir, fs.ModeDir | 0o500, ReasonDirMode},
		{"directory setgid", checkSocketDir, fs.ModeDir | fs.ModeSetgid | 0o700, ReasonSpecialBits},
		{"directory sticky", checkSocketDir, fs.ModeDir | fs.ModeSticky | 0o700, ReasonSpecialBits},
		{"directory setuid", checkSocketDir, fs.ModeDir | fs.ModeSetuid | 0o700, ReasonSpecialBits},
		{"socket 0755", checkSocket, fs.ModeSocket | 0o755, ReasonSocketMode},
		{"socket 0660", checkSocket, fs.ModeSocket | 0o660, ReasonSocketMode},
		{"socket 0400", checkSocket, fs.ModeSocket | 0o400, ReasonSocketMode},
		{"socket setuid", checkSocket, fs.ModeSocket | fs.ModeSetuid | 0o600, ReasonSpecialBits},
		{"socket setgid", checkSocket, fs.ModeSocket | fs.ModeSetgid | 0o600, ReasonSpecialBits},
		{"regular file as socket", checkSocket, 0o600, ReasonNotSocket},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requireUnsafe(t, tc.check(path, fakeInfo{mode: tc.mode, uid: own}, euid), path, tc.reason)
		})
	}
}

// TestAcquireAlreadyRunning checks that a held start lock stops a second
// Acquire with ErrAlreadyRunning, without cachelock's LockedError, and that
// Acquire succeeds again after Release.
func TestAcquireAlreadyRunning(t *testing.T) {
	l := newLayout(t)
	first, err := Acquire(l.socket)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })
	second, err := Acquire(l.socket)
	if err == nil {
		_ = second.Release()
		t.Fatal("second Acquire succeeded while the lock was held")
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Acquire error = %v, want ErrAlreadyRunning", err)
	}
	if _, ok := errors.AsType[*cachelock.LockedError](err); ok {
		t.Errorf("second Acquire error = %v wraps cachelock.LockedError, whose text is about yt-dlp", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	acquire(t, l.socket)
}

// TestListenAfterRelease checks that a released Lock cannot bind the socket
// and that a second Release reports the misuse.
func TestListenAfterRelease(t *testing.T) {
	l := newLayout(t)
	lock, err := Acquire(l.socket)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	listener, err := lock.Listen()
	if err == nil {
		_ = listener.Close()
	}
	if !errors.Is(err, errReleased) {
		t.Errorf("Listen after Release: error = %v, want errReleased", err)
	}
	requireAbsent(t, l.socket)
	if err := lock.Release(); !errors.Is(err, errReleased) {
		t.Errorf("second Release: error = %v, want errReleased", err)
	}
}

// TestListenCreatesSocket checks that the socket accepts connections with
// exactly mode 0600, and that closing the listener removes it.
func TestListenCreatesSocket(t *testing.T) {
	l := newLayout(t)
	listener := listen(t, acquire(t, l.socket))
	if mode := lstat(t, l.socket).Mode(); mode != fs.ModeSocket|0o600 {
		t.Errorf("socket mode = %v, want %v", mode, fs.ModeSocket|0o600)
	}
	conn, err := net.Dial("unix", l.socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	requireAbsent(t, l.socket)
}

// TestListenRemovesStaleSocket checks that a socket left by a killed server
// is replaced by a working one. Bind fails while the stale file is there, so
// a successful Listen and dial show it was removed; the inode number cannot
// show it, since ext4 reuses a freed one at once.
func TestListenRemovesStaleSocket(t *testing.T) {
	l := newLayout(t)
	lock := acquire(t, l.socket)
	staleSocket(t, l.socket)
	listen(t, lock)
}

// TestListenInUse checks that a socket another process accepts connections on
// stops Listen with ErrInUse and is left in place.
func TestListenInUse(t *testing.T) {
	l := newLayout(t)
	lock := acquire(t, l.socket)
	udstest.Listen(t, l.socket)
	before := lstat(t, l.socket)
	listener, err := lock.Listen()
	if err == nil {
		_ = listener.Close()
	}
	if !errors.Is(err, ErrInUse) {
		t.Fatalf("Listen error = %v, want ErrInUse", err)
	}
	if !os.SameFile(before, lstat(t, l.socket)) {
		t.Error("the live socket was replaced")
	}
}

// TestListenRejectsExistingNames checks that every existing name at the
// socket path other than a socket that is stale or in use stops Listen with
// the reason and is left in place.
func TestListenRejectsExistingNames(t *testing.T) {
	tests := []struct {
		name   string
		create func(t *testing.T, l layout)
		reason UnsafeReason
	}{
		{
			name: "regular file",
			create: func(t *testing.T, l layout) {
				if err := os.WriteFile(l.socket, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			reason: ReasonNotSocket,
		},
		{
			name: "directory",
			create: func(t *testing.T, l layout) {
				mkdirMode(t, l.socket, 0o700)
			},
			reason: ReasonNotSocket,
		},
		{
			name: "named pipe",
			create: func(t *testing.T, l layout) {
				if err := syscall.Mkfifo(l.socket, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			reason: ReasonNotSocket,
		},
		{
			// The link points at a stale socket, so following it would make
			// the target look removable.
			name: "symlink to a stale socket",
			create: func(t *testing.T, l layout) {
				target := filepath.Join(l.dir, "target.sock")
				staleSocket(t, target)
				if err := os.Symlink(target, l.socket); err != nil {
					t.Fatal(err)
				}
			},
			reason: ReasonSymlink,
		},
		{
			// A socket whose connection attempt is denied, not refused, is
			// not known to be stale.
			name: "socket without write permission",
			create: func(t *testing.T, l layout) {
				skipIfRoot(t)
				staleSocket(t, l.socket)
				if err := os.Chmod(l.socket, 0o000); err != nil {
					t.Fatal(err)
				}
			},
			reason: ReasonUndetermined,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayout(t)
			lock := acquire(t, l.socket)
			tc.create(t, l)
			before := lstat(t, l.socket)
			listener, err := lock.Listen()
			if err == nil {
				_ = listener.Close()
			}
			requireUnsafe(t, err, l.socket, tc.reason)
			after := lstat(t, l.socket)
			if !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Errorf("%s changed: mode %v -> %v", l.socket, before.Mode(), after.Mode())
			}
			if target := filepath.Join(l.dir, "target.sock"); tc.reason == ReasonSymlink {
				lstat(t, target)
			}
		})
	}
}

// TestUnsafeErrorText checks that every reason has its own text, distinct
// from the zero value's, and that the message names the path, the reason, and
// that the user has to fix it.
//
// The message is what the server prints to the user, so its wording is the
// observable contract and is matched as text.
func TestUnsafeErrorText(t *testing.T) {
	seen := map[string]struct{}{UnsafeReason(0).String(): {}}
	for reason := ReasonSymlink; reason <= ReasonUndetermined; reason++ {
		text := reason.String()
		if _, dup := seen[text]; dup {
			t.Errorf("reason %d text %q repeats another reason's or the zero value's", reason, text)
		}
		seen[text] = struct{}{}
		const path = "/home/u/app/run"
		msg := (&UnsafeError{Path: path, Reason: reason}).Error()
		for _, want := range []string{path, text, "fix it by hand"} {
			if !strings.Contains(msg, want) {
				t.Errorf("UnsafeError message %q does not contain %q", msg, want)
			}
		}
	}
}

// requireIOFailure checks that err is the expected I/O failure (when want is
// not nil) and is not reported as one of the classified outcomes, so an I/O
// problem is not mistaken for an unsafe shape, a running server, or a live
// socket.
func requireIOFailure(t *testing.T, err, want error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want a failure")
	}
	if want != nil && !errors.Is(err, want) {
		t.Errorf("error = %v, want %v", err, want)
	}
	for _, sentinel := range []error{ErrUnsafe, ErrAlreadyRunning, ErrInUse} {
		if errors.Is(err, sentinel) {
			t.Errorf("error = %v, want one that does not wrap %v", err, sentinel)
		}
	}
}

// TestAcquireIOFailures checks that a directory that cannot be created or
// inspected, and a lock that cannot be taken for a reason other than a holder,
// fail Acquire without a classified error.
func TestAcquireIOFailures(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, l layout)
		// want is the expected cause; nil where it is not exported.
		want error
	}{
		{
			name: "ancestor not writable",
			setup: func(t *testing.T, l layout) {
				skipIfRoot(t)
				mkdirMode(t, filepath.Dir(l.parent), 0o700)
				chmodForTest(t, filepath.Dir(l.parent), 0o500)
			},
			want: fs.ErrPermission,
		},
		{
			name: "ancestor is a regular file",
			setup: func(t *testing.T, l layout) {
				if err := os.WriteFile(filepath.Dir(l.parent), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: syscall.ENOTDIR,
		},
		{
			name: "parent not writable",
			setup: func(t *testing.T, l layout) {
				skipIfRoot(t)
				mkdirMode(t, l.parent, 0o700)
				chmodForTest(t, l.parent, 0o500)
			},
			want: fs.ErrPermission,
		},
		{
			name: "lock name is a directory",
			setup: func(t *testing.T, l layout) {
				mkdirMode(t, l.dir, 0o700)
				// cachelock's fixed lock file name. Were it renamed, Acquire
				// would succeed and this row would fail.
				mkdirMode(t, filepath.Join(l.dir, ".yt2column.lock"), 0o700)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayout(t)
			tc.setup(t, l)
			lock, err := Acquire(l.socket)
			if err == nil {
				_ = lock.Release()
			}
			requireIOFailure(t, err, tc.want)
		})
	}
}

// TestListenIOFailures checks that a socket path that cannot be inspected,
// bound, or cleared of a stale socket fails Listen without a classified
// error.
func TestListenIOFailures(t *testing.T) {
	tests := []struct {
		name string
		// setup runs after Acquire and returns the socket path to listen on.
		setup func(t *testing.T, l layout) string
		want  error
	}{
		{
			name: "socket directory not searchable",
			setup: func(t *testing.T, l layout) string {
				skipIfRoot(t)
				chmodForTest(t, l.dir, 0o600)
				return l.socket
			},
			want: fs.ErrPermission,
		},
		{
			name: "stale socket cannot be removed",
			setup: func(t *testing.T, l layout) string {
				skipIfRoot(t)
				staleSocket(t, l.socket)
				chmodForTest(t, l.dir, 0o500)
				return l.socket
			},
			want: fs.ErrPermission,
		},
		{
			name: "socket path too long to bind",
			setup: func(_ *testing.T, l layout) string {
				return filepath.Join(l.dir, strings.Repeat("s", 120)+".sock")
			},
			want: syscall.EINVAL,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayout(t)
			lock := acquire(t, l.socket)
			lock.socketPath = tc.setup(t, l)
			listener, err := lock.Listen()
			if err == nil {
				_ = listener.Close()
			}
			requireIOFailure(t, err, tc.want)
		})
	}
}

// TestAbandon checks that closing a listener after a failed post-bind check
// unlinks the path only while it still holds the effective user's socket, so
// a name put there after bind, or a socket of another user, is left.
func TestAbandon(t *testing.T) {
	euid := os.Geteuid()
	tests := []struct {
		name string
		// swap replaces the bound socket, if at all, before abandon runs.
		swap     func(t *testing.T, path string)
		euid     int
		wantKept bool
	}{
		{name: "own socket", swap: func(*testing.T, string) {}, euid: euid, wantKept: false},
		{name: "socket of another user", swap: func(*testing.T, string) {}, euid: euid + 1, wantKept: true},
		{
			name: "regular file swapped in",
			swap: func(t *testing.T, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			euid:     euid,
			wantKept: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(udstest.Dir(t), "s.sock")
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			tc.swap(t, path)
			abandon(listener, path, tc.euid)
			_, err = os.Lstat(path)
			if kept := err == nil; kept != tc.wantKept {
				t.Errorf("name kept = %v (Lstat error %v), want %v", kept, err, tc.wantKept)
			}
		})
	}
}

// TestSocketPathLimitMatchesSockaddr checks that the limit internal/config
// derives the socket path within is the longest path this operating system's
// sockaddr_un holds with its terminating NUL. internal/config cannot import the
// OS-specific type, so it keeps the limit as a constant and this test fixes it.
func TestSocketPathLimitMatchesSockaddr(t *testing.T) {
	want := len(syscall.RawSockaddrUnix{}.Path) - 1
	if got := config.SocketPathMaxForTest(runtime.GOOS); got != want {
		t.Errorf("config socket path limit for %s = %d, want %d", runtime.GOOS, got, want)
	}
}
