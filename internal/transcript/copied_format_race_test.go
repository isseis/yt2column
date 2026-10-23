//go:build test && race

package transcript

// copiedParseAllocPerByte bounds the bytes TestParseCopiedLargeInputs may
// allocate per input byte. The race detector's runtime inflates TotalAlloc by
// about two orders of magnitude for the inputs with many timestamp matches
// (measured about 90 to 150 times for the shortest form A, textless form B, and
// form A heading inputs, at most about 1.9 KiB per input byte), so the race
// build uses a loose bound that still catches a super-linear (quadratic) number
// of allocations.
//
// copiedParseChecksDeadline is false: the race build does not compare the
// parse time with copiedParseDeadline. The server is built without the race
// detector, whose instrumentation slows the parse by an amount unrelated to the
// server's parse time, so the deadline is checked only by the non-race build.
//
// This file is built by the -race runs (`make test` and `make test-ci`) but not
// by `make lint`, which passes only the test,integration build tags. The
// non-race constants live in copied_format_norace_test.go, which lint compiles
// and `make test-parse-norace` (run by `make test` and by CI) builds and runs.
const (
	copiedParseAllocPerByte   = 8192
	copiedParseChecksDeadline = false
)
