//go:build test

// Package udstest gives tests a short directory for Unix domain sockets and a
// listener on one. macOS limits a socket path to 103 bytes, and t.TempDir()
// there (/var/folders/...) leaves too little room for the server's socket path
// under it, so the directories are created under /tmp instead. Built only with
// the test tag.
package udstest

import (
	"net"
	"os"
	"testing"
)

const (
	// parentDir is where Dir creates its directories. It is short on every
	// Unix this project supports.
	parentDir = "/tmp"
	// dirPattern is the os.MkdirTemp pattern. The random suffix differs on
	// every call, so packages that go test runs in parallel, and a rerun after
	// an interrupted run that skipped its cleanup, never share a directory.
	dirPattern = "y2c"
)

// Dir creates a new, empty directory under /tmp and removes it, with
// everything in it, at cleanup.
//
// Contract: two calls never return the same directory, even across
// processes, because the name comes from os.MkdirTemp.
func Dir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp(parentDir, dirPattern)
	if err != nil {
		t.Fatalf("create a short socket directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("remove the short socket directory %s: %v", dir, err)
		}
	})
	return dir
}

// Listen listens on the Unix domain socket at path and registers closing the
// listener at cleanup as soon as it exists, so a test that fails later does
// not leave it open. Closing removes the socket file.
func Listen(t testing.TB, path string) net.Listener {
	t.Helper()
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen on %s: %v", path, err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}
