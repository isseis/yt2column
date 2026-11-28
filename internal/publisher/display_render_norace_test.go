//go:build test && !race

package publisher

// neutralizedLinearTimeChecksRatio is true: this build has no race detector,
// so the linear-time test compares the times here when YT2COLUMN_TEST_TIMING
// asks for it. test-parse-norace sets that variable and runs this check alone,
// with no other test running alongside; the -race build does not compare the
// times.
const neutralizedLinearTimeChecksRatio = true
