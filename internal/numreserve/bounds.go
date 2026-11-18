package numreserve

import "time"

// bounds holds the limits the tool imposes on itself, whoever calls it. They
// live in a struct, not in constants read directly, so a test helper can
// shrink the time limits instead of waiting for them.
type bounds struct {
	// jumpWidth: a candidate above the base maximum plus this stops (jump).
	jumpWidth int
	// maxPosts: the creation is tried at most this many times per run.
	maxPosts int
	// commandTimeout bounds each external command.
	commandTimeout time.Duration
	// runTimeout bounds one whole run.
	runTimeout time.Duration
	// maxOutput caps each of a command's standard output and standard error.
	maxOutput int
	// maxBranches caps the number of origin branches a run reads.
	maxBranches int
	// waitDelay bounds the wait for a command's pipes after it is stopped.
	waitDelay time.Duration
}

// maxNumber is the largest four-digit number.
const maxNumber = 9999

// The production bounds; the architecture's table (3.4) gives the reason for
// each.
const (
	defaultJumpWidth      = 20
	defaultMaxPosts       = 10
	defaultCommandTimeout = 60 * time.Second
	defaultRunTimeout     = 3 * time.Minute
	defaultMaxOutput      = 4 << 20
	defaultMaxBranches    = 500
	defaultWaitDelay      = 5 * time.Second
)

// defaultBounds returns the production bounds.
func defaultBounds() bounds {
	return bounds{
		jumpWidth:      defaultJumpWidth,
		maxPosts:       defaultMaxPosts,
		commandTimeout: defaultCommandTimeout,
		runTimeout:     defaultRunTimeout,
		maxOutput:      defaultMaxOutput,
		maxBranches:    defaultMaxBranches,
		waitDelay:      defaultWaitDelay,
	}
}
