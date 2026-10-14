//go:build test

package transcript

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCopiedLabels(t *testing.T) {
	cases := []struct {
		name    string
		hours   int
		minutes int
		seconds int
		wantJP  string
		wantEN  string
	}{
		{name: "zero keeps seconds", hours: 0, minutes: 0, seconds: 0, wantJP: "0 秒", wantEN: "0 seconds"},
		{name: "seconds plural", hours: 0, minutes: 0, seconds: 3, wantJP: "3 秒", wantEN: "3 seconds"},
		{name: "seconds singular", hours: 0, minutes: 0, seconds: 1, wantJP: "1 秒", wantEN: "1 second"},
		{name: "seconds two digits", hours: 0, minutes: 0, seconds: 24, wantJP: "24 秒", wantEN: "24 seconds"},
		{name: "minutes omit seconds", hours: 0, minutes: 1, seconds: 0, wantJP: "1 分", wantEN: "1 minute"},
		{name: "minutes plural", hours: 0, minutes: 24, seconds: 0, wantJP: "24 分", wantEN: "24 minutes"},
		{name: "minutes and seconds", hours: 0, minutes: 1, seconds: 6, wantJP: "1 分 6 秒", wantEN: "1 minute, 6 seconds"},
		{name: "minutes and seconds plural", hours: 0, minutes: 5, seconds: 12, wantJP: "5 分 12 秒", wantEN: "5 minutes, 12 seconds"},
		{name: "hours omit minutes", hours: 1, minutes: 0, seconds: 30, wantJP: "1 時間 30 秒", wantEN: "1 hour, 30 seconds"},
		{name: "hours and minutes", hours: 1, minutes: 9, seconds: 0, wantJP: "1 時間 9 分", wantEN: "1 hour, 9 minutes"},
		{name: "hours singular unit", hours: 1, minutes: 0, seconds: 7, wantJP: "1 時間 7 秒", wantEN: "1 hour, 7 seconds"},
		{name: "hours plural no minutes", hours: 2, minutes: 0, seconds: 0, wantJP: "2 時間", wantEN: "2 hours"},
		{name: "all units", hours: 2, minutes: 10, seconds: 26, wantJP: "2 時間 10 分 26 秒", wantEN: "2 hours, 10 minutes, 26 seconds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			timestamp := copiedTimestamp{hours: tc.hours, minutes: tc.minutes, seconds: tc.seconds}
			if got := japaneseLabel(timestamp); got != tc.wantJP {
				t.Errorf("japaneseLabel(%d:%02d:%02d) = %q, want %q", tc.hours, tc.minutes, tc.seconds, got, tc.wantJP)
			}
			if got := englishLabel(timestamp); got != tc.wantEN {
				t.Errorf("englishLabel(%d:%02d:%02d) = %q, want %q", tc.hours, tc.minutes, tc.seconds, got, tc.wantEN)
			}
		})
	}
}

func TestParseCopiedAccepts(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []Segment
	}{
		{
			name:  "form B skips an empty timestamp",
			input: "0:00\n0:05\nこんにちは\n",
			want:  []Segment{{StartMs: 5000, Text: "こんにちは"}},
		},
		{
			name:  "form A Japanese label",
			input: "0:2424 秒Hello\n",
			want:  []Segment{{StartMs: 24000, Text: "Hello"}},
		},
		{
			name:  "form A English label",
			input: "0:2424 secondsHello\n",
			want:  []Segment{{StartMs: 24000, Text: "Hello"}},
		},
		{
			name:  "form A Japanese single-unit label",
			input: "1:001 分Hello\n",
			want:  []Segment{{StartMs: 60000, Text: "Hello"}},
		},
		{
			name:  "form A English single-unit label",
			input: "1:001 minuteHello\n",
			want:  []Segment{{StartMs: 60000, Text: "Hello"}},
		},
		{
			name:  "form A equal timestamps are accepted",
			input: "0:2424 秒x\n0:2424 秒y\n",
			want:  []Segment{{StartMs: 24000, Text: "x"}, {StartMs: 24000, Text: "y"}},
		},
		{
			name:  "form B equal timestamps are accepted",
			input: "0:00\nx\n0:00\ny\n",
			want:  []Segment{{StartMs: 0, Text: "x"}, {StartMs: 0, Text: "y"}},
		},
		{
			name:  "form A English minutes and singular second",
			input: "1:251 minute, 25 secondsHello\n",
			want:  []Segment{{StartMs: 85000, Text: "Hello"}},
		},
		{
			name:  "form A Japanese zero seconds",
			input: "0:000 秒Hello\n",
			want:  []Segment{{StartMs: 0, Text: "Hello"}},
		},
		{
			name:  "form A skips a segment without text",
			input: "0:044 秒\n0:1010 秒Next\n",
			want:  []Segment{{StartMs: 10000, Text: "Next"}},
		},
		{
			name:  "form B three timestamp lines then text",
			input: "0:00\n0:05\n0:10\nHello\n",
			want:  []Segment{{StartMs: 10000, Text: "Hello"}},
		},
		{
			name:  "form A text begins with a digit and a unit",
			input: "1:00:571 時間 57 秒1 種類\n",
			want:  []Segment{{StartMs: 3657000, Text: "1 種類"}},
		},
		{
			name:  "form A text has leading and trailing whitespace",
			input: "0:2424 秒 Hello ",
			want:  []Segment{{StartMs: 24000, Text: " Hello "}},
		},
		{
			name:  "form B text line merely starts with a timestamp",
			input: "0:00\n1:30 から始めます\n",
			want:  []Segment{{StartMs: 0, Text: "1:30 から始めます"}},
		},
		{
			name:  "form B text line 05:00 is not a timestamp",
			input: "0:00\n05:00\n",
			want:  []Segment{{StartMs: 0, Text: "05:00"}},
		},
		{
			name:  "form B text line 100:00:00 is not a timestamp",
			input: "0:00\n100:00:00\n",
			want:  []Segment{{StartMs: 0, Text: "100:00:00"}},
		},
		{
			name:  "form A boundary 59:59",
			input: "59:5959 分 59 秒x\n",
			want:  []Segment{{StartMs: 3599000, Text: "x"}},
		},
		{
			name:  "form A boundary 1:00:00",
			input: "1:00:001 時間x\n",
			want:  []Segment{{StartMs: 3600000, Text: "x"}},
		},
		{
			name:  "form A boundary 99:59:59",
			input: "99:59:5999 時間 59 分 59 秒x\n",
			want:  []Segment{{StartMs: 359999000, Text: "x"}},
		},
		{
			name:  "form B boundary 59:59",
			input: "59:59\nx\n",
			want:  []Segment{{StartMs: 3599000, Text: "x"}},
		},
		{
			name:  "form B boundary 1:00:00",
			input: "1:00:00\nx\n",
			want:  []Segment{{StartMs: 3600000, Text: "x"}},
		},
		{
			name:  "form B boundary 99:59:59",
			input: "99:59:59\nx\n",
			want:  []Segment{{StartMs: 359999000, Text: "x"}},
		},
		{
			name:  "form B boundary 0:00",
			input: "0:00\nx\n",
			want:  []Segment{{StartMs: 0, Text: "x"}},
		},
		{
			name:  "form A English 2 minutes, 1 second",
			input: "2:012 minutes, 1 secondHello\n",
			want:  []Segment{{StartMs: 121000, Text: "Hello"}},
		},
		{
			name:  "form A English 1 hour, 30 seconds",
			input: "1:00:301 hour, 30 secondsHello\n",
			want:  []Segment{{StartMs: 3630000, Text: "Hello"}},
		},
		{
			name:  "form A English 1 hour, 9 minutes",
			input: "1:09:001 hour, 9 minutesHello\n",
			want:  []Segment{{StartMs: 4140000, Text: "Hello"}},
		},
		{
			name:  "form A English all units",
			input: "2:10:262 hours, 10 minutes, 26 secondsHello\n",
			want:  []Segment{{StartMs: 7826000, Text: "Hello"}},
		},
		{
			name:  "form A English zero seconds",
			input: "0:000 secondsHello\n",
			want:  []Segment{{StartMs: 0, Text: "Hello"}},
		},
		{
			name:  "form B text keeps a trailing NBSP",
			input: "0:00\nx\u00A0\n",
			want:  []Segment{{StartMs: 0, Text: "x\u00A0"}},
		},
		{
			name:  "form B accepts ten-hour timestamp",
			input: "10:00:00\nx\n",
			want:  []Segment{{StartMs: 36000000, Text: "x"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCopiedTranscript(tc.input)
			if err != nil {
				t.Fatalf("parseCopiedTranscript(%q) error = %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseCopiedTranscript(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseCopiedRejects(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		reason error
		line   int
	}{
		{name: "invalid UTF-8", input: string([]byte{0xff}), reason: errCopiedInvalidUTF8},
		{name: "blank empty", input: "", reason: errCopiedBlank},
		{name: "blank spaces", input: "   ", reason: errCopiedBlank},
		{name: "blank tab", input: "\t", reason: errCopiedBlank},
		{name: "blank ideographic space", input: "\u3000", reason: errCopiedBlank},
		{name: "blank NBSP", input: "\u00A0", reason: errCopiedBlank},
		{name: "blank newline", input: "\n", reason: errCopiedBlank},
		{name: "line break mixed", input: "0:00\nx\n0:05\r\n", reason: errCopiedLineBreak, line: 3},
		{name: "line break lone CR", input: "0:00\rx\n", reason: errCopiedLineBreak, line: 1},
		{name: "line break trailing CR", input: "0:00\nx\r", reason: errCopiedLineBreak, line: 2},
		{name: "empty line in the middle", input: "0:00\n\nx\n", reason: errCopiedEmptyLine, line: 2},
		{name: "empty line at the start", input: "\n0:00\nx\n", reason: errCopiedEmptyLine, line: 1},
		{name: "two trailing separators", input: "0:00\nx\n\n", reason: errCopiedEmptyLine, line: 3},
		{name: "three trailing separators", input: "0:00\nx\n\n\n", reason: errCopiedEmptyLine, line: 3},
		{name: "control tab in text", input: "0:00\nx\ty\n", reason: errCopiedControlChar, line: 2},
		{name: "control NUL in timestamp", input: "0:00\x00\nx\n", reason: errCopiedControlChar, line: 1},
		{name: "leading display items", input: "タイムライン\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "chapter heading after text", input: "0:00\nx\n存在感のある声\n", reason: errCopiedNotTimestamp, line: 3},
		{name: "leading BOM", input: "\uFEFF0:00\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "minute leading zero", input: "05:00\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "minute over 59", input: "60:00\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "single-digit second", input: "0:5\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "zero hour in h:mm:ss", input: "0:01:00\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "three-digit hour", input: "100:00:00\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "m:ss then colon and three digits", input: "0:12:34\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "m:ss then colon and one-digit second", input: "1:23:4\nx\n", reason: errCopiedNotTimestamp, line: 1},
		{name: "bad timestamp at a form B position", input: "0:00\nx\n05:00\n", reason: errCopiedNotTimestamp, line: 3},
		{name: "English label without comma", input: "1:251 minute 25 secondsHello\n", reason: errCopiedLabelMismatch, line: 1},
		{name: "label does not match", input: "0:24abc\n", reason: errCopiedLabelMismatch, line: 1},
		{name: "second line uses the other language", input: "0:2424 秒x\n0:2424 secondsy\n", reason: errCopiedLabelMismatch, line: 2},
		{name: "form A whitespace-only text", input: "0:2424 秒 ", reason: errCopiedBlankText, line: 1},
		{name: "form B whitespace-only text", input: "0:00\n \n", reason: errCopiedBlankText, line: 2},
		{name: "form A has a whole timestamp line", input: "0:2424 秒x\n0:30\n", reason: errCopiedMixedForms, line: 2},
		{name: "form A has a whole hour timestamp line", input: "0:2424 秒x\n1:00:00\n", reason: errCopiedMixedForms, line: 2},
		{name: "form B text reads as form A", input: "0:00\n0:2424 秒x\n", reason: errCopiedMixedForms, line: 2},
		{name: "form A line where form B wants a timestamp", input: "0:00\nx\n1:0024 秒y\n", reason: errCopiedNotTimestamp, line: 3},
		{name: "English first then a Japanese label", input: "0:2424 secondsx\n0:2424 秒y\n", reason: errCopiedLabelMismatch, line: 2},
		{name: "form A timestamp goes backward", input: "0:2424 秒x\n0:1010 秒y\n", reason: errCopiedTimestampOrder, line: 2},
		{name: "form B timestamp goes backward", input: "0:05\nx\n0:00\ny\n", reason: errCopiedTimestampOrder, line: 3},
		{name: "form B has no text", input: "0:00\n0:05\n", reason: errCopiedNoText},
		{name: "form A has no text", input: "0:000 秒\n", reason: errCopiedNoText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := assertCopiedRejected(t, tc.input, tc.reason)
			if tc.line > 0 {
				if !strings.Contains(message, fmt.Sprintf("line %d:", tc.line)) {
					t.Errorf("Error() = %q, want line %d", message, tc.line)
				}
			} else if strings.Contains(message, ": line ") {
				t.Errorf("Error() = %q, want no line number", message)
			}
		})
	}
}

func TestParseCopiedLineBreaks(t *testing.T) {
	t.Run("LF and CRLF give the same segments", func(t *testing.T) {
		lf := "0:00\nx\n0:05\ny\n"
		crlf := "0:00\r\nx\r\n0:05\r\ny\r\n"
		want := []Segment{{StartMs: 0, Text: "x"}, {StartMs: 5000, Text: "y"}}
		for name, input := range map[string]string{"LF": lf, "CRLF": crlf} {
			got, err := parseCopiedTranscript(input)
			if err != nil {
				t.Fatalf("%s: parseCopiedTranscript(%q) error = %v", name, input, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: segments = %+v, want %+v", name, got, want)
			}
		}
	})

	t.Run("a trailing separator changes nothing", func(t *testing.T) {
		withTrailing := "0:00\nx\n"
		withoutTrailing := "0:00\nx"
		want := []Segment{{StartMs: 0, Text: "x"}}
		for name, input := range map[string]string{"one trailing": withTrailing, "none": withoutTrailing} {
			got, err := parseCopiedTranscript(input)
			if err != nil {
				t.Fatalf("%s: parseCopiedTranscript(%q) error = %v", name, input, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: segments = %+v, want %+v", name, got, want)
			}
		}
	})

	separatorRejects := []struct {
		name  string
		input string
		line  int
	}{
		{name: "LF then CRLF", input: "0:00\nx\r\n", line: 2},
		{name: "CRLF then LF", input: "0:00\r\nx\n", line: 2},
		{name: "lone CR", input: "0:00\rx", line: 1},
		{name: "trailing CR", input: "0:00\nx\r", line: 2},
	}
	for _, tc := range separatorRejects {
		t.Run(tc.name, func(t *testing.T) {
			message := assertCopiedRejected(t, tc.input, errCopiedLineBreak)
			if !strings.Contains(message, fmt.Sprintf("line %d:", tc.line)) {
				t.Errorf("Error() = %q, want line %d", message, tc.line)
			}
		})
	}

	trailingRejects := []struct {
		name  string
		input string
	}{
		{name: "two trailing separators", input: "0:00\nx\n\n"},
		{name: "three trailing separators", input: "0:00\nx\n\n\n"},
	}
	for _, tc := range trailingRejects {
		t.Run(tc.name, func(t *testing.T) {
			message := assertCopiedRejected(t, tc.input, errCopiedEmptyLine)
			if !strings.Contains(message, "line 3:") {
				t.Errorf("Error() = %q, want line 3", message)
			}
		})
	}
}

func TestParseCopiedCheckOrder(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    error
		notWant []error
	}{
		{
			name:    "invalid UTF-8 before line break",
			input:   "\xff\r",
			want:    errCopiedInvalidUTF8,
			notWant: []error{errCopiedLineBreak, errCopiedBlank},
		},
		{
			name:    "blank before empty line",
			input:   "\n\n",
			want:    errCopiedBlank,
			notWant: []error{errCopiedEmptyLine},
		},
		{
			name:    "line break before empty line",
			input:   "0:00\n\n\r\n",
			want:    errCopiedLineBreak,
			notWant: []error{errCopiedEmptyLine},
		},
		{
			name:    "empty line before control character",
			input:   "0:00\n\nx\ty\n",
			want:    errCopiedEmptyLine,
			notWant: []error{errCopiedControlChar},
		},
		{
			name:    "label mismatch before timestamp order",
			input:   "0:2424 秒x\n0:10BOGUS\n",
			want:    errCopiedLabelMismatch,
			notWant: []error{errCopiedTimestampOrder},
		},
		{
			name:    "line kind before blank text",
			input:   "0:00\nx\n \n",
			want:    errCopiedNotTimestamp,
			notWant: []error{errCopiedBlankText},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := assertCopiedRejected(t, tc.input, tc.want)
			for _, other := range tc.notWant {
				if strings.Contains(message, other.Error()) {
					t.Errorf("Error() = %q, must not report %v", message, other)
				}
			}
		})
	}
}

func TestParseCopiedErrorOmitsInput(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		markers []string
	}{
		{
			name:    "label mismatch omits the line",
			input:   "0:24XX24 秒YY",
			markers: []string{"XX", "YY", "0:24"},
		},
		{
			name:    "not a timestamp omits the line",
			input:   "MARKER_LINE\n",
			markers: []string{"MARKER_LINE"},
		},
		{
			name:    "timestamp order omits the bodies",
			input:   "0:2424 秒AAA\n0:1010 秒BBB\n",
			markers: []string{"AAA", "BBB", "0:24", "0:10"},
		},
		{
			name:    "mixed forms omits the line",
			input:   "0:00\n0:2424 秒ZZZ\n",
			markers: []string{"ZZZ", "0:24"},
		},
		{
			name:    "chapter heading omits the line",
			input:   "0:00\nTEXT_MARKER\nCHAPTER_MARKER\n",
			markers: []string{"CHAPTER_MARKER", "TEXT_MARKER"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCopiedTranscript(tc.input)
			if err == nil {
				t.Fatalf("parseCopiedTranscript(%q) accepted the input", tc.input)
			}
			for _, marker := range tc.markers {
				if strings.Contains(err.Error(), marker) {
					t.Errorf("Error() = %q, must not contain %q", err.Error(), marker)
				}
			}
		})
	}
}

// assertCopiedRejected fails the test unless parseCopiedTranscript rejects input
// with ErrCopiedTranscriptFormat wrapping reason, returns no partial result, and
// it returns the error message for further checks.
func assertCopiedRejected(t *testing.T, input string, reason error) string {
	t.Helper()
	segments, err := parseCopiedTranscript(input)
	if !errors.Is(err, ErrCopiedTranscriptFormat) {
		t.Fatalf("parseCopiedTranscript(%q) error = %v, want ErrCopiedTranscriptFormat", input, err)
	}
	if !errors.Is(err, reason) {
		t.Fatalf("parseCopiedTranscript(%q) error = %v, want %v", input, err, reason)
	}
	if segments != nil {
		t.Errorf("parseCopiedTranscript(%q) returned segments %+v", input, segments)
	}
	return err.Error()
}
