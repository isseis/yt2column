//go:build test

package codexreview

import (
	"os"
	"testing"

	"github.com/isseis/yt2column/internal/codexreview/testutil"
)

// TestMain lets the test binary act as the fake codex when it is started under
// that name, and otherwise runs the tests with a PATH on which the real codex
// cannot be found.
func TestMain(m *testing.M) {
	os.Exit(codexreviewtestutil.Main(m))
}
