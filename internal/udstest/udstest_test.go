//go:build test

package udstest

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"
)

const (
	// serverSocketSuffix is the server's socket path below HOME on macOS, the
	// longest of the supported layouts.
	serverSocketSuffix = "/Library/Application Support/yt2column/run/server.sock"
	// darwinSocketPathLimit is the longest socket path macOS accepts: sun_path
	// is 104 bytes, one of which is the terminating NUL.
	darwinSocketPathLimit = 103
)

// TestDirLeavesRoomForSocketPath checks that a server socket path under Dir,
// used as HOME, fits the macOS limit.
func TestDirLeavesRoomForSocketPath(t *testing.T) {
	dir := Dir(t)
	if got := len(dir) + len(serverSocketSuffix); got > darwinSocketPathLimit {
		t.Errorf("len(%q) + len(%q) = %d, want at most %d", dir, serverSocketSuffix, got, darwinSocketPathLimit)
	}
}

// TestDirUnique checks that two calls return different directories, so tests
// that run in parallel never share one.
func TestDirUnique(t *testing.T) {
	first, second := Dir(t), Dir(t)
	if first == second {
		t.Errorf("Dir returned %q twice", first)
	}
}

// TestDirRemovedAtCleanup checks that the directory and its contents are gone
// once the test that created it ends.
func TestDirRemovedAtCleanup(t *testing.T) {
	var dir string
	t.Run("create", func(t *testing.T) {
		dir = Dir(t)
		if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(%s) after cleanup: error = %v, want fs.ErrNotExist", dir, err)
	}
}

// TestListen checks that the listener accepts a connection and is closed at
// cleanup, which removes the socket file.
func TestListen(t *testing.T) {
	path := filepath.Join(Dir(t), "s.sock")
	t.Run("listen", func(t *testing.T) {
		Listen(t, path)
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("dial %s: %v", path, err)
		}
		_ = conn.Close()
	})
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(%s) after cleanup: error = %v, want fs.ErrNotExist", path, err)
	}
}
