//go:build test

package coveragecompare

import (
	"os"
	"testing"
)

// TestMain lets the test binary play go, node, and npm, and keeps the real
// ones out of reach of every test (fakeMain).
func TestMain(m *testing.M) {
	os.Exit(fakeMain(m))
}
