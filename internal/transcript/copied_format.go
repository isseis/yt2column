package transcript

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reasons a copied transcript is rejected. Each is wrapped together with
// ErrCopiedTranscriptFormat, so a caller can tell them apart with errors.Is and
// a user only ever sees ErrCopiedTranscriptFormat.
var (
	errCopiedInvalidUTF8    = errors.New("input is not valid UTF-8")
	errCopiedBlank          = errors.New("input has no non-whitespace character")
	errCopiedLineBreak      = errors.New("line breaks mix LF and CRLF or include a lone CR")
	errCopiedEmptyLine      = errors.New("empty line")
	errCopiedControlChar    = errors.New("line contains a control character")
	errCopiedNotTimestamp   = errors.New("line does not start with a timestamp where one is required")
	errCopiedLabelMismatch  = errors.New("screen-reader label does not match the timestamp")
	errCopiedBlankText      = errors.New("segment text is whitespace only")
	errCopiedMixedForms     = errors.New("line belongs to the other form")
	errCopiedTimestampOrder = errors.New("timestamp is earlier than the previous one")
	errCopiedNoText         = errors.New("no segment has text")
)

const (
	secondsPerMinute      = 60
	millisecondsPerSecond = 1000
	// labelUnits bounds the hour, minute, and second parts of a label.
	labelUnits = 3
	// Submatch groups in the index slice FindStringSubmatchIndex returns. Group
	// n's start and end are at positions 2n and 2n+1, so the first three groups
	// begin at these offsets.
	regexpFirstGroup  = 2
	regexpSecondGroup = 4
	regexpThirdGroup  = 6
)

// The Japanese screen-reader units are the literal words YouTube's transcript
// panel emits for the Japanese display language. The parser compares them
// against the input byte for byte, so they stay in Japanese; translating them
// would change the strings under comparison.
const (
	jpUnitSecond = "秒"
	jpUnitMinute = "分"
	jpUnitHour   = "時間"
)

// The timestamp grammars of the copied transcript format. minuteSecondPattern
// accepts minutes 0-59 with no leading zero; hourMinuteSecondPattern accepts
// hours 1-99 with no leading zero. Both require two-digit minutes and seconds.
var (
	hourMinuteSecondPattern = regexp.MustCompile(`^([1-9][0-9]?):([0-5][0-9]):([0-5][0-9])`)
	minuteSecondPattern     = regexp.MustCompile(`^([1-5]?[0-9]):([0-5][0-9])`)
)

// labelLanguage identifies which display language a form A line's screen-reader
// label belongs to. labelNone means the language is not fixed yet.
type labelLanguage int

const (
	labelNone labelLanguage = iota
	labelJapanese
	labelEnglish
)

// copiedTimestamp is a timestamp parsed from the input, kept in both its parts
// (for label derivation) and milliseconds (for Segment.StartMs).
type copiedTimestamp struct {
	startMs int64
	hours   int
	minutes int
	seconds int
}

// parseCopiedTranscript parses a copied YouTube transcript into segments. It
// accepts form A (a timestamp, a screen-reader label, and the text concatenated
// on one line) or form B (a timestamp line followed by a text line), decides
// which by the first line alone, and rejects every input that matches neither.
// It never repairs an input, never returns a partial result, and includes no
// part of the input in its errors.
func parseCopiedTranscript(raw string) ([]Segment, error) {
	if !utf8.ValidString(raw) {
		return nil, copiedGlobalError(errCopiedInvalidUTF8)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, copiedGlobalError(errCopiedBlank)
	}
	separator, err := copiedLineSeparator(raw)
	if err != nil {
		return nil, err
	}
	lines := splitCopiedLines(raw, separator)
	if _, ok := parseTimestampLine(lines[0]); ok {
		return parseCopiedFormB(lines)
	}
	return parseCopiedFormA(lines)
}

// copiedGlobalError reports a rejection that concerns the input as a whole, so
// it carries no line number.
func copiedGlobalError(reason error) error {
	return fmt.Errorf("%w: %w", ErrCopiedTranscriptFormat, reason)
}

// copiedLineError reports a rejection at one line, so the caller can point the
// user at it.
func copiedLineError(line int, reason error) error {
	return fmt.Errorf("%w: line %d: %w", ErrCopiedTranscriptFormat, line, reason)
}

// copiedLineSeparator returns the single line separator the input uses. It
// requires every separator to be LF or every separator to be CRLF, so a lone CR
// or a mix of the two is rejected at the line where the first mismatch occurs.
func copiedLineSeparator(raw string) (string, error) {
	separator := ""
	line := 1
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\r':
			if i+1 >= len(raw) || raw[i+1] != '\n' {
				return "", copiedLineError(line, errCopiedLineBreak)
			}
			if separator != "" && separator != "\r\n" {
				return "", copiedLineError(line, errCopiedLineBreak)
			}
			separator = "\r\n"
			i++
			line++
		case '\n':
			if separator != "" && separator != "\n" {
				return "", copiedLineError(line, errCopiedLineBreak)
			}
			separator = "\n"
			line++
		}
	}
	return separator, nil
}

// splitCopiedLines splits an input into lines. A single trailing separator just
// ends the last line, so it produces no empty line; a second or later trailing
// separator does, and is caught by the empty-line check.
func splitCopiedLines(raw, separator string) []string {
	if separator == "" {
		return []string{raw}
	}
	lines := strings.Split(raw, separator)
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// parseTimestampLine reports whether the whole line is a timestamp and returns
// it. Whitespace around a timestamp makes it not a timestamp line.
func parseTimestampLine(line string) (copiedTimestamp, bool) {
	timestamp, consumed, ok := parseTimestampPrefix(line)
	if !ok || consumed != len(line) {
		return copiedTimestamp{}, false
	}
	return timestamp, true
}

// parseTimestampPrefix parses the timestamp at the start of line and returns it
// with the number of bytes it consumes. The reading is unique: m:ss and
// h:mm:ss can both match at the start only when m:ss is followed by ":", and a
// label (which always starts with a digit) cannot follow there, so such a line
// starts with no timestamp at all.
func parseTimestampPrefix(line string) (copiedTimestamp, int, bool) {
	if match := hourMinuteSecondPattern.FindStringSubmatchIndex(line); match != nil {
		timestamp := copiedTimestamp{hours: atoiMatch(line, match, regexpFirstGroup), minutes: atoiMatch(line, match, regexpSecondGroup), seconds: atoiMatch(line, match, regexpThirdGroup)}
		timestamp.startMs = timestampToMilliseconds(timestamp)
		return timestamp, match[1], true
	}
	match := minuteSecondPattern.FindStringSubmatchIndex(line)
	if match == nil {
		return copiedTimestamp{}, 0, false
	}
	if match[1] < len(line) && line[match[1]] == ':' {
		return copiedTimestamp{}, 0, false
	}
	timestamp := copiedTimestamp{minutes: atoiMatch(line, match, regexpFirstGroup), seconds: atoiMatch(line, match, regexpSecondGroup)}
	timestamp.startMs = timestampToMilliseconds(timestamp)
	return timestamp, match[1], true
}

// atoiMatch returns group as an int. The pattern guarantees digits, so it
// cannot fail.
func atoiMatch(line string, match []int, group int) int {
	value, _ := strconv.Atoi(line[match[group]:match[group+1]])
	return value
}

// timestampToMilliseconds converts a parsed timestamp to its start in
// milliseconds. The grammar caps the value far below int64's range.
func timestampToMilliseconds(timestamp copiedTimestamp) int64 {
	totalSeconds := int64(timestamp.hours)*secondsPerMinute + int64(timestamp.minutes)
	totalSeconds = totalSeconds*secondsPerMinute + int64(timestamp.seconds)
	return totalSeconds * millisecondsPerSecond
}

// japaneseLabel derives the Japanese display language's screen-reader label for
// a timestamp: nonzero units in hour, minute, second order, separated by one
// space. A timestamp with no nonzero unit keeps its seconds.
func japaneseLabel(timestamp copiedTimestamp) string {
	parts := make([]string, 0, labelUnits)
	parts = appendJapaneseUnit(parts, jpUnitHour, timestamp.hours)
	parts = appendJapaneseUnit(parts, jpUnitMinute, timestamp.minutes)
	if timestamp.seconds != 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d %s", timestamp.seconds, jpUnitSecond))
	}
	return strings.Join(parts, " ")
}

func appendJapaneseUnit(parts []string, unit string, value int) []string {
	if value == 0 {
		return parts
	}
	return append(parts, fmt.Sprintf("%d %s", value, unit))
}

// englishLabel derives the English display language's screen-reader label for a
// timestamp: nonzero units in hour, minute, second order, separated by ", ". A
// unit of 1 is singular and every other count, including 0, is plural.
func englishLabel(timestamp copiedTimestamp) string {
	parts := make([]string, 0, labelUnits)
	parts = appendEnglishUnit(parts, englishHourUnit, timestamp.hours)
	parts = appendEnglishUnit(parts, englishMinuteUnit, timestamp.minutes)
	if timestamp.seconds != 0 || len(parts) == 0 {
		parts = append(parts, formatEnglishUnit(englishSecondUnit, timestamp.seconds))
	}
	return strings.Join(parts, ", ")
}

const (
	englishHourUnit   = "hour"
	englishMinuteUnit = "minute"
	englishSecondUnit = "second"
)

func appendEnglishUnit(parts []string, unit string, value int) []string {
	if value == 0 {
		return parts
	}
	return append(parts, formatEnglishUnit(unit, value))
}

func formatEnglishUnit(unit string, value int) string {
	if value == 1 {
		return fmt.Sprintf("%d %s", value, unit)
	}
	return fmt.Sprintf("%d %ss", value, unit)
}

// matchFormALabel matches the screen-reader label at the start of rest against
// the label the timestamp dictates. When the language is not fixed yet it tries
// both; afterwards it only accepts the fixed language. It returns the matched
// label so the caller can find where the text begins.
func matchFormALabel(rest string, timestamp copiedTimestamp, language labelLanguage) (string, labelLanguage, bool) {
	if language == labelNone || language == labelJapanese {
		if label := japaneseLabel(timestamp); strings.HasPrefix(rest, label) {
			return label, labelJapanese, true
		}
	}
	if language == labelNone || language == labelEnglish {
		if label := englishLabel(timestamp); strings.HasPrefix(rest, label) {
			return label, labelEnglish, true
		}
	}
	return "", labelNone, false
}

// checkCopiedLine applies the checks every line gets before its form-specific
// rules: it must not be empty and must not contain a control character.
func checkCopiedLine(line string, lineNumber int) error {
	if line == "" {
		return copiedLineError(lineNumber, errCopiedEmptyLine)
	}
	if hasCopiedControlChar(line) {
		return copiedLineError(lineNumber, errCopiedControlChar)
	}
	return nil
}

func hasCopiedControlChar(line string) bool {
	for _, r := range line {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// parseCopiedFormA parses form A: every line is a timestamp, a screen-reader
// label, and the text concatenated without separators. The label's language is
// fixed by the first line. A segment without text is skipped; a line that is
// only a timestamp belongs to the other form. It rejects on the first error in
// the documented check order.
func parseCopiedFormA(lines []string) ([]Segment, error) {
	language := labelNone
	var segments []Segment
	previousMs := int64(-1)
	for index, line := range lines {
		lineNumber := index + 1
		if err := checkCopiedLine(line, lineNumber); err != nil {
			return nil, err
		}
		timestamp, consumed, ok := parseTimestampPrefix(line)
		if !ok {
			return nil, copiedLineError(lineNumber, errCopiedNotTimestamp)
		}
		if consumed == len(line) {
			return nil, copiedLineError(lineNumber, errCopiedMixedForms)
		}
		label, matchedLanguage, ok := matchFormALabel(line[consumed:], timestamp, language)
		if !ok {
			return nil, copiedLineError(lineNumber, errCopiedLabelMismatch)
		}
		if language == labelNone {
			language = matchedLanguage
		}
		text := line[consumed+len(label):]
		if text != "" && strings.TrimSpace(text) == "" {
			return nil, copiedLineError(lineNumber, errCopiedBlankText)
		}
		if timestamp.startMs < previousMs {
			return nil, copiedLineError(lineNumber, errCopiedTimestampOrder)
		}
		previousMs = timestamp.startMs
		if text != "" {
			segments = append(segments, Segment{StartMs: timestamp.startMs, Text: text})
		}
	}
	if len(segments) == 0 {
		return nil, copiedGlobalError(errCopiedNoText)
	}
	return segments, nil
}

// parseCopiedFormB parses form B: a timestamp line followed by zero or one text
// line, alternating. A timestamp followed by another timestamp or the end has
// no text. The line after a text line must be a timestamp, so a chapter heading
// there is rejected. A text line that reads as a form A line is rejected as a
// mix of the forms. It rejects on the first error in the documented check
// order.
func parseCopiedFormB(lines []string) ([]Segment, error) {
	var segments []Segment
	previousMs := int64(-1)
	pendingMs := int64(0)
	awaitingText := false
	for index, line := range lines {
		lineNumber := index + 1
		if err := checkCopiedLine(line, lineNumber); err != nil {
			return nil, err
		}
		if timestamp, ok := parseTimestampLine(line); ok {
			if timestamp.startMs < previousMs {
				return nil, copiedLineError(lineNumber, errCopiedTimestampOrder)
			}
			previousMs = timestamp.startMs
			pendingMs = timestamp.startMs
			awaitingText = true
			continue
		}
		if !awaitingText {
			return nil, copiedLineError(lineNumber, errCopiedNotTimestamp)
		}
		if readableAsFormA(line) {
			return nil, copiedLineError(lineNumber, errCopiedMixedForms)
		}
		if strings.TrimSpace(line) == "" {
			return nil, copiedLineError(lineNumber, errCopiedBlankText)
		}
		segments = append(segments, Segment{StartMs: pendingMs, Text: line})
		awaitingText = false
	}
	if len(segments) == 0 {
		return nil, copiedGlobalError(errCopiedNoText)
	}
	return segments, nil
}

// readableAsFormA reports whether line could be a form A line: a timestamp
// immediately followed by the label that timestamp dictates in either display
// language.
func readableAsFormA(line string) bool {
	timestamp, consumed, ok := parseTimestampPrefix(line)
	if !ok || consumed == len(line) {
		return false
	}
	_, _, matched := matchFormALabel(line[consumed:], timestamp, labelNone)
	return matched
}
