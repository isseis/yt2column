//go:build test && race

package transcript

// copiedParseAllocPerByte bounds the bytes TestParseCopiedLargeInputs may
// allocate per input byte. The race detector's runtime inflates TotalAlloc by
// about two orders of magnitude (measured about 1.9 KiB per input byte against
// about 19 without -race), so the race build uses a loose bound that still
// catches a super-linear (quadratic) number of allocations.
//
// This file is built by `make test` (-race) but not by `make lint`, which
// passes only the test,integration build tags; the non-race bound lives in
// copied_format_norace_test.go and is checked by lint.
const copiedParseAllocPerByte = 8192
