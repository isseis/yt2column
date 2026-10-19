//go:build test

package serversock

import (
	"os"
	"testing"
)

// skipIfRoot skips a test that depends on ownership or permissions, which
// root bypasses.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("ownership and permission checks do not hold for root")
	}
}

// chmodForTest changes path's mode and restores the original mode at cleanup,
// so the temporary directory can be removed afterwards.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	original := info.Mode().Perm()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, original) })
}
