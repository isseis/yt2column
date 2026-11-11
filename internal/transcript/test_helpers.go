//go:build test

package transcript

import (
	"os"
	"testing"
)

// testdataCopiedTranscripts is the glob of the committed copied-transcript
// fixtures, relative to this package directory.
const testdataCopiedTranscripts = "../../testdata/*.transcript_copy.txt"

// readTestdataFile returns the bytes of a committed testdata fixture.
func readTestdataFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the path names a committed testdata fixture
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
