//go:build test && !race

package transcript

// copiedParseAllocPerByte bounds the bytes TestParseCopiedLargeInputs may
// allocate per input byte. Measured about 19 per byte for the worst input, so
// this tight bound catches a constant-factor allocation regression that the
// race build's loose bound would miss.
//
// copiedLargeInputSize equals copiedLargeInputLimit: this build parses inputs
// of the module's full 4 MiB input cap.
//
// copiedParseChecksDeadline is true: the non-race build compares the parse time
// with copiedParseDeadline, because the production builds have no race
// detector and this build parses as they do.
const (
	copiedLargeInputSize      = copiedLargeInputLimit
	copiedParseAllocPerByte   = 128
	copiedParseChecksDeadline = true
)
