//go:build test

package privatefile

import "testing"

// setHookForTest makes f run at every hookPoint for the rest of the test and
// restores the no-op hook at cleanup.
func setHookForTest(t *testing.T, f func(point hookPoint, path string) error) {
	t.Helper()
	previous := hook
	hook = f
	t.Cleanup(func() { hook = previous })
}
