//go:build test

package privatefile

import (
	"errors"
	"testing"
)

// setHookForTest makes f run at every hookPoint for the rest of the test and
// restores the no-op hook at cleanup.
func setHookForTest(t *testing.T, f func(point hookPoint, path string) error) {
	t.Helper()
	previous := hook
	hook = f
	t.Cleanup(func() { hook = previous })
}

// errInjectedForTest is the failure the exported test hooks inject.
var errInjectedForTest = errors.New("privatefile: failure injected by a test")

// FailCreateBeforeLinkForTest makes every Create for the rest of the test
// fail just before it links the temporary file at the path, so a test in
// another package can reach a creation failure after the directories above
// the file were created.
func FailCreateBeforeLinkForTest(t *testing.T) {
	t.Helper()
	setHookForTest(t, func(point hookPoint, _ string) error {
		if point == hookCreateBeforeLink {
			return errInjectedForTest
		}
		return nil
	})
}

// FailRemoveTempForTest makes every Create for the rest of the test fail to
// remove its temporary file after the link, so a test in another package can
// reach a success that leaves the temporary file behind.
func FailRemoveTempForTest(t *testing.T) {
	t.Helper()
	setHookForTest(t, func(point hookPoint, _ string) error {
		if point == hookCreateBeforeRemoveTemp {
			return errInjectedForTest
		}
		return nil
	})
}
