//go:build test

package codexreview

import (
	"testing"
	"time"
)

// shortenTimeouts replaces the time limits of a run for one test and restores
// them when it ends. Only test builds compile it, so no argument or
// environment variable can change the limits. A test that calls it must not
// run in parallel.
func shortenTimeouts(t *testing.T, codex, grace, version, git time.Duration) {
	t.Helper()
	saved := [4]time.Duration{codexTimeout, stopGrace, versionTimeout, gitTimeout}
	codexTimeout, stopGrace, versionTimeout, gitTimeout = codex, grace, version, git
	t.Cleanup(func() {
		codexTimeout, stopGrace, versionTimeout, gitTimeout = saved[0], saved[1], saved[2], saved[3]
	})
}
