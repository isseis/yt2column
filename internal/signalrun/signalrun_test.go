//go:build test

package signalrun

import (
	"os"
	"slices"
	"syscall"
	"testing"
)

// TestSubscribedSignals checks that SIGINT and SIGHUP are each subscribed
// only when not ignored at startup, and SIGTERM always.
func TestSubscribedSignals(t *testing.T) {
	for _, intIgnored := range []bool{false, true} {
		for _, hupIgnored := range []bool{false, true} {
			ignored := func(sig os.Signal) bool {
				return (sig == os.Interrupt && intIgnored) || (sig == syscall.SIGHUP && hupIgnored)
			}
			got := subscribedSignals(ignored)
			if !slices.Contains(got, os.Signal(syscall.SIGTERM)) {
				t.Errorf("SIGINT ignored = %v, SIGHUP ignored = %v: subscribed %v, want SIGTERM", intIgnored, hupIgnored, got)
			}
			if slices.Contains(got, os.Interrupt) == intIgnored {
				t.Errorf("SIGINT ignored = %v: subscribed %v", intIgnored, got)
			}
			if slices.Contains(got, os.Signal(syscall.SIGHUP)) == hupIgnored {
				t.Errorf("SIGHUP ignored = %v: subscribed %v", hupIgnored, got)
			}
		}
	}
}
