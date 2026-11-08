//go:build test && unix

package privatefile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
