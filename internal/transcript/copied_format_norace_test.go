//go:build test && !race

package transcript

// copiedParseAllocPerByte bounds the bytes TestParseCopiedLargeInputs may
// allocate per input byte. Measured about 19 per byte for the worst input, so
// this tight bound catches a constant-factor allocation regression that the
// race build's loose bound would miss.
//
// copiedParseChecksDeadline is true: the non-race build compares the parse time
// with copiedParseDeadline, because the server is built without the race
// detector and this build parses as the server does.
const (
	copiedParseAllocPerByte   = 128
	copiedParseChecksDeadline = true
)
