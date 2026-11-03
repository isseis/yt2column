//go:build test

package coveragecompare

import (
	"errors"
	"testing"
)

const (
	// testModulePath is the module path the parser tests give parseGoProfile.
	testModulePath = "github.com/isseis/yt2column"
	// testExtensionDir is the checkout extension directory the TAP parser
	// tests give parseTAP.
	testExtensionDir = "/checkout/extension"
)

// requireStop fails the test unless err is an unreadable StopError at point and
// input, and returns its first reason.
func requireStop(t *testing.T, err error, point Point, input Input) StopReason {
	t.Helper()
	stop, ok := errors.AsType[*StopError](err)
	if !ok {
		t.Fatalf("error = %v, want *StopError", err)
	}
	if stop.Kind != StopUnreadable {
		t.Fatalf("stop kind = %v, want %v (reasons: %+v)", stop.Kind, StopUnreadable, stop.Reasons)
	}
	if len(stop.Reasons) == 0 {
		t.Fatal("stop has no reasons")
	}
	reason := stop.Reasons[0]
	if reason.Point != point {
		t.Errorf("reason point = %v, want %v", reason.Point, point)
	}
	if reason.Input != input {
		t.Errorf("reason input = %v, want %v", reason.Input, input)
	}
	return reason
}
