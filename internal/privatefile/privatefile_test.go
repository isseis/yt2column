//go:build test && unix

package privatefile

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/udstest"
)

// fifoWait bounds how long a test waits for Read on a named pipe. Read opens
// with O_NONBLOCK, so it returns at once; without the flag it would block
// until a writer came.
const fifoWait = 2 * time.Second

// skipIfRoot skips a test that depends on ownership or permissions, which
// root bypasses.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("ownership and permission checks do not hold for root")
	}
}

// writeFile writes content to path and sets exactly mode, whatever the umask.
func writeFile(t *testing.T, path string, content []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// writeSpecialFile writes a mode 0600 file with the special bit set and fails
// the test when the system silently drops the bit, so a case never passes
// without testing what it names.
func writeSpecialFile(t *testing.T, path string, special fs.FileMode) {
	t.Helper()
	writeFile(t, path, []byte("x"), Mode|special)
	if got := lstat(t, path).Mode() & specialBits; got != special {
		t.Fatalf("%s has special bits %v after chmod, want %v", path, got, special)
	}
}

// chmodForTest changes path's mode and restores the original mode at cleanup,
// registered at once, so the temporary directory can be removed afterwards.
func chmodForTest(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	original := lstat(t, path).Mode().Perm()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, original) })
}

func lstat(t *testing.T, path string) fs.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// mkfifo creates a named pipe at path and registers, at cleanup, opening it
// for writing without blocking, which releases a reader blocked on it.
func mkfifo(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if w, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = w.Close()
		}
	})
}

// readResult is what Read returned.
type readResult struct {
	content []byte
	err     error
}

// readWithin calls Read in a goroutine and fails the test when it does not
// return within fifoWait, so a Read that blocks on a named pipe fails instead
// of hanging the test binary.
func readWithin(t *testing.T, path string, limit int) ([]byte, error) {
	t.Helper()
	done := make(chan readResult, 1)
	go func() {
		content, err := Read(path, limit)
		done <- readResult{content, err}
	}()
	select {
	case r := <-done:
		return r.content, r.err
	case <-time.After(fifoWait):
		t.Fatalf("Read(%s) did not return within %v", path, fifoWait)
		return nil, nil
	}
}

// requireUnsafe checks that err is an *UnsafeError for path with reason and
// the mode found.
func requireUnsafe(t *testing.T, err error, path string, reason UnsafeReason, mode fs.FileMode) {
	t.Helper()
	if !errors.Is(err, ErrUnsafe) {
		t.Fatalf("error = %v, want ErrUnsafe", err)
	}
	unsafe, ok := errors.AsType[*UnsafeError](err)
	if !ok {
		t.Fatalf("error = %v, want *UnsafeError", err)
	}
	if unsafe.Path != path || unsafe.Reason != reason || unsafe.Mode != mode {
		t.Errorf("UnsafeError = {%s, %v, %v}, want {%s, %v, %v}", unsafe.Path, unsafe.Reason, unsafe.Mode, path, reason, mode)
	}
}

// requireSentinel checks that err wraps want and is an *fs.PathError for path.
func requireSentinel(t *testing.T, err error, path string, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	pathErr, ok := errors.AsType[*fs.PathError](err)
	if !ok || pathErr.Path != path {
		t.Errorf("error = %#v, want an *fs.PathError for %s", err, path)
	}
}

// TestRead checks the accepted file, the size limit, and each shape Read
// refuses without reading it.
func TestRead(t *testing.T) {
	const limit = 4
	type want struct {
		content []byte
		reason  UnsafeReason // set for an *UnsafeError
		mode    fs.FileMode  // the mode an *UnsafeError for ReasonMode reports
		err     error        // a sentinel wrapped in an *fs.PathError
	}
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
		limit int
		want  want
	}{
		{
			name:  "accepted file at the limit",
			setup: func(t *testing.T, path string) { writeFile(t, path, []byte("abcd"), Mode) },
			limit: limit,
			want:  want{content: []byte("abcd")},
		},
		{
			name:  "empty file with limit zero",
			setup: func(t *testing.T, path string) { writeFile(t, path, nil, Mode) },
			limit: 0,
			want:  want{content: []byte{}},
		},
		{
			name:  "one byte over the limit",
			setup: func(t *testing.T, path string) { writeFile(t, path, []byte("abcde"), Mode) },
			limit: limit,
			want:  want{err: ErrTooLarge},
		},
		{
			name:  "missing",
			setup: func(*testing.T, string) {},
			limit: limit,
			want:  want{err: ErrNotExist},
		},
		{
			name: "symbolic link to an accepted file",
			setup: func(t *testing.T, path string) {
				target := filepath.Join(filepath.Dir(path), "target")
				writeFile(t, target, []byte("abcd"), Mode)
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
			limit: limit,
			want:  want{reason: ReasonSymlink},
		},
		{
			name: "dangling symbolic link",
			setup: func(t *testing.T, path string) {
				if err := os.Symlink(filepath.Join(filepath.Dir(path), "absent"), path); err != nil {
					t.Fatal(err)
				}
			},
			limit: limit,
			want:  want{reason: ReasonSymlink},
		},
		{
			name: "directory",
			setup: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			limit: limit,
			want:  want{reason: ReasonNotRegular},
		},
		{
			name:  "named pipe without a writer",
			setup: mkfifo,
			limit: limit,
			want:  want{reason: ReasonNotRegular},
		},
		{name: "mode 0644", setup: modeFile(0o644), limit: limit, want: want{reason: ReasonMode, mode: 0o644}},
		{name: "mode 0640", setup: modeFile(0o640), limit: limit, want: want{reason: ReasonMode, mode: 0o640}},
		{name: "mode 0400", setup: modeFile(0o400), limit: limit, want: want{reason: ReasonMode, mode: 0o400}},
		{name: "mode 0700", setup: modeFile(0o700), limit: limit, want: want{reason: ReasonMode, mode: 0o700}},
		{name: "mode 0000", setup: modeFile(0), limit: limit, want: want{reason: ReasonMode, mode: 0}},
		{
			name:  "mode 0600 with setuid",
			setup: func(t *testing.T, path string) { writeSpecialFile(t, path, fs.ModeSetuid) },
			limit: limit,
			want:  want{reason: ReasonMode, mode: Mode | fs.ModeSetuid},
		},
		{
			name:  "mode 0600 with setgid",
			setup: func(t *testing.T, path string) { writeSpecialFile(t, path, fs.ModeSetgid) },
			limit: limit,
			want:  want{reason: ReasonMode, mode: Mode | fs.ModeSetgid},
		},
		{
			name:  "mode 0600 with sticky",
			setup: func(t *testing.T, path string) { writeSpecialFile(t, path, fs.ModeSticky) },
			limit: limit,
			want:  want{reason: ReasonMode, mode: Mode | fs.ModeSticky},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.env")
			tt.setup(t, path)
			content, err := readWithin(t, path, tt.limit)
			switch {
			case tt.want.reason != ReasonUnknown:
				requireUnsafe(t, err, path, tt.want.reason, tt.want.mode)
			case tt.want.err != nil:
				requireSentinel(t, err, path, tt.want.err)
			case err != nil:
				t.Fatalf("Read: %v", err)
			case !bytes.Equal(content, tt.want.content):
				t.Errorf("Read = %q, want %q", content, tt.want.content)
			}
			if err != nil && content != nil {
				t.Errorf("Read returned content %q with error %v", content, err)
			}
		})
	}

	t.Run("socket", func(t *testing.T) {
		path := filepath.Join(udstest.Dir(t), "s")
		udstest.Listen(t, path)
		_, err := readWithin(t, path, limit)
		requireUnsafe(t, err, path, ReasonNotRegular, 0)
	})

	t.Run("negative limit", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.env")
		writeFile(t, path, nil, Mode)
		if _, err := Read(path, -1); !errors.Is(err, errNegativeLimit) {
			t.Errorf("Read(-1): error = %v, want errNegativeLimit", err)
		}
	})
}

// modeFile returns a setup that writes a small file with exactly mode.
func modeFile(mode fs.FileMode) func(t *testing.T, path string) {
	return func(t *testing.T, path string) { writeFile(t, path, []byte("x"), mode) }
}

// TestReadUnreadable checks that a path Read cannot reach is reported with the
// operating system's error, not as a missing or refused file.
func TestReadUnreadable(t *testing.T) {
	t.Run("parent without search permission", func(t *testing.T) {
		skipIfRoot(t)
		dir := filepath.Join(t.TempDir(), "closed")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "config.env")
		writeFile(t, path, []byte("x"), Mode)
		chmodForTest(t, dir, 0o600)
		_, err := Read(path, 1)
		requireUnreadable(t, err)
		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("error = %v, want fs.ErrPermission", err)
		}
	})
	t.Run("parent is a regular file", func(t *testing.T) {
		parent := filepath.Join(t.TempDir(), "file")
		writeFile(t, parent, []byte("x"), Mode)
		_, err := Read(filepath.Join(parent, "config.env"), 1)
		requireUnreadable(t, err)
		if !errors.Is(err, syscall.ENOTDIR) {
			t.Errorf("error = %v, want ENOTDIR", err)
		}
	})
}

// requireUnreadable checks that err is a failure that wraps none of the
// package's sentinels, so the caller reports it as unreadable.
func requireUnreadable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Read: error = nil, want a failure")
	}
	for _, sentinel := range []error{ErrNotExist, ErrUnsafe, ErrTooLarge} {
		if errors.Is(err, sentinel) {
			t.Errorf("error = %v wraps %v, want an unclassified failure", err, sentinel)
		}
	}
}

// TestReadChecksDescriptor replaces the accepted file between the Lstat and
// the open, and checks that what was opened decides: the Lstat result never
// allows the read.
func TestReadChecksDescriptor(t *testing.T) {
	tests := []struct {
		name    string
		replace func(t *testing.T, path string)
		check   func(t *testing.T, path string, err error)
	}{
		{
			name:    "mode 0644 file",
			replace: func(t *testing.T, path string) { writeFile(t, path, []byte("wide"), 0o644) },
			check: func(t *testing.T, path string, err error) {
				requireUnsafe(t, err, path, ReasonMode, 0o644)
			},
		},
		{
			name: "directory",
			replace: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, path string, err error) {
				requireUnsafe(t, err, path, ReasonNotRegular, 0)
			},
		},
		{
			name:    "named pipe",
			replace: mkfifo,
			check: func(t *testing.T, path string, err error) {
				requireUnsafe(t, err, path, ReasonNotRegular, 0)
			},
		},
		{
			name: "symbolic link to an accepted file",
			replace: func(t *testing.T, path string) {
				target := filepath.Join(filepath.Dir(path), "target")
				writeFile(t, target, []byte("link"), Mode)
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, path string, err error) {
				requireUnsafe(t, err, path, ReasonSymlink, 0)
			},
		},
		{
			name:    "removed",
			replace: func(*testing.T, string) {},
			check: func(t *testing.T, path string, err error) {
				requireSentinel(t, err, path, ErrNotExist)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.env")
			writeFile(t, path, []byte("good"), Mode)
			replaced := false
			setHookForTest(t, func(point hookPoint, p string) error {
				if point != hookReadAfterLstat || p != path {
					return nil
				}
				replaced = true
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				tt.replace(t, path)
				return nil
			})
			content, err := readWithin(t, path, 64)
			if !replaced {
				t.Fatal("the hook after Lstat did not run")
			}
			if content != nil {
				t.Errorf("Read returned content %q, want none", content)
			}
			tt.check(t, path, err)
		})
	}
}

// fakeInfo is an fs.FileInfo with a chosen mode and Sys value, for owners the
// test cannot create without root.
type fakeInfo struct {
	fs.FileInfo
	mode fs.FileMode
	sys  any
}

func (f fakeInfo) Mode() fs.FileMode { return f.mode }
func (f fakeInfo) Sys() any          { return f.sys }

// TestOwnerAndModeCheck calls the check Read applies to both the Lstat and the
// fstat result directly, for the owners a non-root test cannot create.
func TestOwnerAndModeCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.env")
	writeFile(t, path, []byte("x"), Mode)
	info := lstat(t, path)
	euid := os.Geteuid()
	tests := []struct {
		name       string
		info       fs.FileInfo
		euid       int
		wantReason UnsafeReason
		wantOK     bool
	}{
		{name: "owned by the effective user", info: info, euid: euid, wantOK: true},
		{name: "owned by another user", info: info, euid: euid + 1, wantReason: ReasonForeignOwner},
		{name: "negative effective user", info: info, euid: -1, wantReason: ReasonForeignOwner},
		{name: "owner unreadable", info: fakeInfo{FileInfo: info, mode: Mode, sys: nil}, euid: euid, wantReason: ReasonForeignOwner},
		{name: "foreign owner and wrong mode", info: fakeInfo{FileInfo: info, mode: 0o644, sys: info.Sys()}, euid: euid + 1, wantReason: ReasonForeignOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, ok := check(tt.info, tt.euid)
			if reason != tt.wantReason || ok != tt.wantOK {
				t.Errorf("check = (%v, %v), want (%v, %v)", reason, ok, tt.wantReason, tt.wantOK)
			}
		})
	}
}

// errInjected is the failure a test hook returns.
var errInjected = errors.New("injected failure")

// withUmask runs f with the process umask set to mask and restores the
// previous umask as soon as f returns. Callers create their temporary
// directories before calling it, so t.TempDir is not affected.
func withUmask(mask int, f func()) {
	previous := syscall.Umask(mask)
	defer syscall.Umask(previous)
	f()
}

// requireNoTemp checks that dir holds no temporary name Create would make for
// the file name "config.env".
func requireNoTemp(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".config.env.tmp-") {
			t.Errorf("%s holds the temporary name %s", dir, entry.Name())
		}
	}
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(%s): error = %v, want fs.ErrNotExist", path, err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// TestCreate checks that the created file has exactly mode 0600 and the
// content under umasks that drop owner bits, add none, or drop the rest.
func TestCreate(t *testing.T) {
	content := []byte("# template\n")
	for _, mask := range []int{0o400, 0o200, 0o077, 0} {
		t.Run(fmt.Sprintf("umask %04o", mask), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.env")
			var leftover string
			var err error
			withUmask(mask, func() { leftover, err = Create(path, content) })
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if leftover != "" {
				t.Errorf("Create left %s behind", leftover)
			}
			if mode := lstat(t, path).Mode(); mode != Mode {
				t.Errorf("mode = %v, want %v", mode, Mode)
			}
			if got := readFile(t, path); !bytes.Equal(got, content) {
				t.Errorf("content = %q, want %q", got, content)
			}
			requireNoTemp(t, dir)
		})
	}
}

// TestCreateExisting checks that every kind of name already at the path is
// reported as existing and left unchanged.
func TestCreateExisting(t *testing.T) {
	const old = "old"
	tests := []struct {
		name     string
		setup    func(t *testing.T, path string)
		readOnly bool // the directory is not writable, so only the Lstat can tell
		check    func(t *testing.T, path string)
	}{
		{
			name:  "regular file",
			setup: func(t *testing.T, path string) { writeFile(t, path, []byte(old), 0o644) },
			check: requireUnchangedFile(old),
		},
		{
			name: "regular file in a read-only directory",
			setup: func(t *testing.T, path string) {
				skipIfRoot(t)
				writeFile(t, path, []byte(old), 0o644)
			},
			readOnly: true,
			check:    requireUnchangedFile(old),
		},
		{
			name: "dangling symbolic link",
			setup: func(t *testing.T, path string) {
				if err := os.Symlink("absent", path); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, path string) {
				if target, err := os.Readlink(path); err != nil || target != "absent" {
					t.Errorf("Readlink = (%q, %v), want the link unchanged", target, err)
				}
				requireAbsent(t, filepath.Join(filepath.Dir(path), "absent"))
			},
		},
		{
			name: "directory",
			setup: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, path string) {
				if mode := lstat(t, path).Mode(); mode != fs.ModeDir|0o755 {
					t.Errorf("mode = %v, want the directory unchanged", mode)
				}
				if entries, err := os.ReadDir(path); err != nil || len(entries) != 0 {
					t.Errorf("ReadDir = (%v, %v), want the directory still empty", entries, err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.env")
			tt.setup(t, path)
			if tt.readOnly {
				chmodForTest(t, dir, 0o500)
			}
			leftover, err := Create(path, []byte("new"))
			requireSentinel(t, err, path, ErrExist)
			if leftover != "" {
				t.Errorf("Create left %s behind", leftover)
			}
			tt.check(t, path)
			requireNoTemp(t, dir)
		})
	}
}

// requireUnchangedFile returns a check that the file at path still holds
// content with mode 0644, the mode the tests give a file the call under test must not touch.
func requireUnchangedFile(content string) func(t *testing.T, path string) {
	const mode fs.FileMode = 0o644
	return func(t *testing.T, path string) {
		t.Helper()
		if got := readFile(t, path); string(got) != content {
			t.Errorf("content = %q, want %q", got, content)
		}
		if got := lstat(t, path).Mode(); got != mode {
			t.Errorf("mode = %v, want %v", got, mode)
		}
	}
}

// TestCreateRaces puts a file at the path after Create's Lstat found nothing,
// and checks that the link refuses it: the file is unchanged and no temporary
// name is left.
func TestCreateRaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")
	setHookForTest(t, func(point hookPoint, _ string) error {
		if point == hookCreateAfterLstat {
			writeFile(t, path, []byte("raced"), 0o644)
		}
		return nil
	})
	leftover, err := Create(path, []byte("new"))
	requireSentinel(t, err, path, ErrExist)
	if leftover != "" {
		t.Errorf("Create left %s behind", leftover)
	}
	requireUnchangedFile("raced")(t, path)
	requireNoTemp(t, dir)
}

// TestCreateFailureLeavesNothing fails each step after the temporary file is
// made and checks that nothing is at the path and no temporary name is left,
// and that a temporary name Create cannot remove is reported.
func TestCreateFailureLeavesNothing(t *testing.T) {
	for _, step := range []struct {
		name  string
		point hookPoint
	}{
		{"write fails", hookCreateBeforeWrite},
		{"sync fails", hookCreateBeforeSync},
		{"link fails", hookCreateBeforeLink},
	} {
		point := step.point
		t.Run(step.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.env")
			setHookForTest(t, failAt(point))
			leftover, err := Create(path, []byte("new"))
			if !errors.Is(err, errInjected) {
				t.Fatalf("Create: error = %v, want the injected failure", err)
			}
			if leftover != "" {
				t.Errorf("Create left %s behind", leftover)
			}
			requireAbsent(t, path)
			requireNoTemp(t, dir)
		})
	}

	t.Run("temporary name not removed after a success", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.env")
		setHookForTest(t, failAt(hookCreateBeforeRemoveTemp))
		leftover, err := Create(path, []byte("new"))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if filepath.Dir(leftover) != dir || !strings.HasPrefix(filepath.Base(leftover), ".config.env.tmp-") {
			t.Fatalf("Create returned %q, want the temporary name in %s", leftover, dir)
		}
		if got := readFile(t, path); string(got) != "new" {
			t.Errorf("content = %q, want %q", got, "new")
		}
		if _, err := os.Lstat(leftover); err != nil {
			t.Errorf("Lstat(%s): %v, want the leftover still there", leftover, err)
		}
	})

	t.Run("temporary name not removed after a failure", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.env")
		var temp string
		setHookForTest(t, func(point hookPoint, name string) error {
			switch point {
			case hookCreateBeforeLink:
				temp = name
				return errInjected
			case hookCreateBeforeRemoveTemp:
				return errInjected
			}
			return nil
		})
		leftover, err := Create(path, []byte("new"))
		if !errors.Is(err, errInjected) || leftover != "" {
			t.Fatalf("Create = (%q, %v), want the injected failure and no name", leftover, err)
		}
		// The name in the error text is how the user learns what to remove,
		// so the wording is the contract here.
		if temp == "" || !strings.Contains(err.Error(), temp) {
			t.Errorf("error = %q, want it to name the temporary file %q", err, temp)
		}
		requireAbsent(t, path)
	})
}

// failAt returns a hook that fails at point and does nothing elsewhere.
func failAt(point hookPoint) func(hookPoint, string) error {
	return func(p hookPoint, _ string) error {
		if p == point {
			return errInjected
		}
		return nil
	}
}

// requireDirMode checks that path is a directory with exactly mode.
func requireDirMode(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	if got := lstat(t, path).Mode(); got != fs.ModeDir|mode {
		t.Errorf("%s mode = %v, want %v", path, got, fs.ModeDir|mode)
	}
}

// TestMkdirBelow checks that every directory MkdirBelow creates has exactly
// mode 0700, under umasks that drop the owner's read or search bit too, that
// an existing element is not changed, and how the base is checked.
func TestMkdirBelow(t *testing.T) {
	for _, mask := range []int{0o400, 0o100, 0, 0o077} {
		t.Run(fmt.Sprintf("umask %04o", mask), func(t *testing.T) {
			base := t.TempDir()
			var err error
			var after int
			withUmask(mask, func() {
				err = MkdirBelow(base, []string{"a", "b"})
				after = syscall.Umask(mask)
			})
			if err != nil {
				t.Fatalf("MkdirBelow: %v", err)
			}
			if after != mask {
				t.Errorf("umask after MkdirBelow = %04o, want %04o restored", after, mask)
			}
			requireDirMode(t, filepath.Join(base, "a"), DirMode)
			requireDirMode(t, filepath.Join(base, "a", "b"), DirMode)
		})
	}

	t.Run("existing element unchanged", func(t *testing.T) {
		base := t.TempDir()
		existing := filepath.Join(base, "a")
		if err := os.Mkdir(existing, 0o700); err != nil {
			t.Fatal(err)
		}
		chmodForTest(t, existing, 0o755)
		if err := MkdirBelow(base, []string{"a", "b"}); err != nil {
			t.Fatalf("MkdirBelow: %v", err)
		}
		requireDirMode(t, existing, 0o755)
		requireDirMode(t, filepath.Join(existing, "b"), DirMode)
	})

	t.Run("setgid parent", func(t *testing.T) {
		base := t.TempDir()
		if err := os.Chmod(base, 0o700|fs.ModeSetgid); err != nil {
			t.Fatal(err)
		}
		if lstat(t, base).Mode()&fs.ModeSetgid == 0 {
			t.Skip("this system does not keep the setgid bit on the temporary directory")
		}
		if err := MkdirBelow(base, []string{"a"}); err != nil {
			t.Fatalf("MkdirBelow: %v", err)
		}
		requireDirMode(t, filepath.Join(base, "a"), DirMode)
	})

	t.Run("base through a symbolic link", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if err := MkdirBelow(link, []string{"a"}); err != nil {
			t.Fatalf("MkdirBelow: %v", err)
		}
		requireDirMode(t, filepath.Join(target, "a"), DirMode)
	})

	t.Run("existing regular file in the way", func(t *testing.T) {
		base := t.TempDir()
		writeFile(t, filepath.Join(base, "a"), []byte("x"), 0o644)
		err := MkdirBelow(base, []string{"a", "b"})
		if err == nil || errors.Is(err, ErrBaseMissing) {
			t.Fatalf("MkdirBelow: error = %v, want a failure other than ErrBaseMissing", err)
		}
		requireUnchangedFile("x")(t, filepath.Join(base, "a"))
	})

	for _, tt := range []struct {
		name string
		// base returns the base inside dir after preparing what is there.
		base func(t *testing.T, dir string) string
	}{
		{name: "base missing", base: func(_ *testing.T, dir string) string { return filepath.Join(dir, "base") }},
		{name: "base is a regular file", base: func(t *testing.T, dir string) string {
			base := filepath.Join(dir, "base")
			writeFile(t, base, []byte("x"), Mode)
			return base
		}},
		{name: "base below a regular file", base: func(t *testing.T, dir string) string {
			file := filepath.Join(dir, "file")
			writeFile(t, file, []byte("x"), Mode)
			return filepath.Join(file, "base")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			base := tt.base(t, dir)
			before, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			err = MkdirBelow(base, []string{"a"})
			requireSentinel(t, err, base, ErrBaseMissing)
			if after, err := os.ReadDir(dir); err != nil || len(after) != len(before) {
				t.Errorf("ReadDir = (%v, %v), want only %v", after, err, before)
			}
		})
	}

	t.Run("names that are not one element", func(t *testing.T) {
		base := t.TempDir()
		for _, name := range []string{"", ".", "..", "a/b"} {
			if err := MkdirBelow(base, []string{"ok", name}); !errors.Is(err, errBadName) {
				t.Errorf("MkdirBelow(%q): error = %v, want errBadName", name, err)
			}
		}
		requireAbsent(t, filepath.Join(base, "ok"))
	})
}

// TestMkdirBelowSwap replaces a directory MkdirBelow just created before its
// mode is set, and checks that the call fails and the replacement's target
// keeps its mode.
func TestMkdirBelowSwap(t *testing.T) {
	tests := []struct {
		name    string
		replace func(t *testing.T, created, other string)
		check   func(t *testing.T, created, other string)
	}{
		{
			name: "symbolic link to another directory",
			replace: func(t *testing.T, created, other string) {
				if err := os.Mkdir(other, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(other, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, created); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, _, other string) { requireDirMode(t, other, 0o755) },
		},
		{
			name: "regular file",
			replace: func(t *testing.T, created, _ string) {
				writeFile(t, created, []byte("x"), 0o644)
			},
			check: func(t *testing.T, created, _ string) { requireUnchangedFile("x")(t, created) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			created := filepath.Join(base, "a")
			other := filepath.Join(base, "other")
			swapped := false
			setHookForTest(t, func(point hookPoint, dir string) error {
				if point != hookMkdirAfterMkdir || dir != created {
					return nil
				}
				swapped = true
				if err := os.Remove(created); err != nil {
					t.Fatal(err)
				}
				tt.replace(t, created, other)
				return nil
			})
			err := MkdirBelow(base, []string{"a"})
			if !swapped {
				t.Fatal("the hook after mkdir did not run")
			}
			requireSentinel(t, err, created, errReplaced)
			tt.check(t, created, other)
		})
	}
}
