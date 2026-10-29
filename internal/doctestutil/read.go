//go:build test

package doctestutil

import (
	"os"
	"testing"
)

// ReadDoc returns the document at path, or fails t. A guard test names the path
// of a repository document, so the path is not untrusted input: a failure means
// the document is missing or cannot be read.
func ReadDoc(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // a guard test names a repository document, not external input
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
