//go:build test

package devfake

import (
	"context"
	"time"
)

// WithWaitForTest returns a copy of s whose calls wait with wait instead of a
// timer, so a test can observe each call's delay, or hold calls inside their
// wait to overlap jobs, without waiting in real time. Every call still checks
// its context after wait returns. s must come from NewScenario: this helper
// changes how a call waits and nothing the scenario selects.
func WithWaitForTest(s Scenario, wait func(ctx context.Context, delay time.Duration)) Scenario {
	if wait == nil {
		panic("devfake.WithWaitForTest: nil wait")
	}
	s.wait = wait
	return s
}
