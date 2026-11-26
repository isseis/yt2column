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
// parse time with copiedParseDeadline. The production builds have no race
// detector, whose instrumentation slows the parse by an amount unrelated to
// their parse time, so the deadline is checked only by the non-race build.
//
// copiedLargeInputSize is 1 MiB, a quarter of the 4 MiB input cap the
// non-race build uses, because the race detector slows the parse of the large
// inputs several-fold (7.5 s for the seven inputs at 4 MiB, 1.9 s at 1 MiB). 1 MiB
// is still large enough to tell a quadratic allocation from a linear one: with
// the allocation of each path made quadratic in turn, the input that stays
// closest to the bound (the form A headings) allocated about 22,000 bytes per
// input byte at 1 MiB, 2.7 times the bound, while at 512 KiB it was 1.4 times
// the bound and at 256 KiB it fell below it. A degradation about four times
// shallower than a quadratic one is not caught at this size; the non-race
// build's tighter bound on 4 MiB inputs is the main check for those.
//
// This file is built by the -race runs (`make test` and `make test-ci`) but not
// by `make lint`, which passes only the test,integration build tags. The
// non-race constants live in copied_format_norace_test.go, which lint compiles
// and `make test-parse-norace` (run by `make test` and by CI) builds and runs.
const (
	copiedLargeInputSize      = 1 << 20
	copiedParseAllocPerByte   = 8192
	copiedParseChecksDeadline = false
)
