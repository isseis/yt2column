//go:build test

package job

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/isseis/yt2column/internal/publisher"
)

// requireNonRoot fails the test when it runs as root, where permission
// failures cannot be produced. Such a run is an unsupported environment, so it
// fails instead of being skipped silently.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("unsupported test environment: permission-failure tests require a non-root user")
	}
}

// chmodForTest changes path's mode and restores the original mode at cleanup,
// so the test temp directory can be removed afterwards.
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
	t.Cleanup(func() {
		_ = os.Chmod(path, original)
	})
}

// newOutput returns an --out path inside dir and a FilePublisher for that same
// path, so a caller can build a FileOutput whose path and Publisher agree.
func newOutput(t *testing.T, dir string) (outPath string, pub publisher.Publisher) {
	t.Helper()
	outPath = filepath.Join(dir, "article.md")
	pub, err := publisher.NewFilePublisher(outPath)
	if err != nil {
		t.Fatalf("NewFilePublisher(%q): %v", outPath, err)
	}
	return outPath, pub
}
