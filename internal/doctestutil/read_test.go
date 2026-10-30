//go:build test

package doctestutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadDoc checks that ReadDoc returns the document's whole content and
// fails t, naming the path, when the file cannot be read.
func TestReadDoc(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("text\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if got := ReadDoc(t, path); got != "text\n" {
		t.Errorf("ReadDoc(%s) = %q, want the file's content", path, got)
	}

	// A real Fatalf ends the test, so the failure is read from a fake that
	// records it instead. ReadDoc returns no error: the message that names the
	// path is the only observable of the failure, so there is no sentinel to
	// match with errors.Is.
	fake := &recordingTB{TB: t}
	absent := filepath.Join(t.TempDir(), "absent.md")
	ReadDoc(fake, absent)
	if !strings.Contains(fake.failure, absent) {
		t.Errorf("ReadDoc(%s) failed with %q, want the path named", absent, fake.failure)
	}
}

// recordingTB is a testing.TB whose Fatalf records the message instead of
// ending the test, so a helper that fails t can be checked for failing.
type recordingTB struct {
	testing.TB
	failure string
}

// Helper does nothing: the recorded failure is not attributed to a caller.
func (r *recordingTB) Helper() {}

// Fatalf records the formatted message.
func (r *recordingTB) Fatalf(format string, args ...any) {
	r.failure = fmt.Sprintf(format, args...)
}
