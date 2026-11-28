//go:build test && race

package publisher

// neutralizedLinearTimeChecksRatio is false: the race detector slows the code
// by an amount unrelated to the neutralizer's own time, so this build never
// compares the times. The linear-time test checks only that the neutralization
// succeeds and keeps its output contract here; the comparison is left to the
// non-race build, which test-parse-norace runs alone. This file is built by
// the -race runs (make test and make test-ci) but not by make lint, which
// passes only the test and integration tags; the non-race constant lives in
// display_render_norace_test.go, which lint compiles.
const neutralizedLinearTimeChecksRatio = false
