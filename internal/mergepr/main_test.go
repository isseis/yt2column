//go:build test

package mergepr

import (
	"fmt"
	"os"
	"testing"

	"github.com/isseis/yt2column/internal/snapshottest"
)

// snapshotRootPrefix is the name prefix of the store directory; the snapshot
// comparison test looks for it in a copy to find a builder's path.
const snapshotRootPrefix = "mergepr-snapshots-"

// TestMain creates the store of shared take-in states before the tests run
// and removes it before exiting with m.Run's status.
func TestMain(m *testing.M) {
	os.Exit(runWithSnapshotStore(m))
}

// runWithSnapshotStore creates the store of shared take-in states, runs the
// tests, and removes the store and its root directory before returning m.Run's
// exit code.
func runWithSnapshotStore(m *testing.M) int {
	root, err := os.MkdirTemp("", snapshotRootPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create the snapshot store directory: %v\n", err)
		return 1
	}
	snapshotStore = snapshottest.NewStore(root)
	code := m.Run()
	if err := snapshotStore.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "remove the snapshot store: %v\n", err)
		code = 1
	}
	if err := os.RemoveAll(root); err != nil {
		fmt.Fprintf(os.Stderr, "remove %s: %v\n", root, err)
		code = 1
	}
	return code
}
