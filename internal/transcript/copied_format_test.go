//go:build test

package transcript

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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
		{
			name:  "form B two trailing separators",
			input: "0:00\nx\n\n",
			want:  []Segment{{StartMs: 0, Text: "x"}},
		},
		{
			name:  "form B three trailing separators",
			input: "0:00\nx\n\n\n",
			want:  []Segment{{StartMs: 0, Text: "x"}},
		},
		{
			name:  "empty line at the start",
			input: "\n0:00\nx\n",
			want:  []Segment{{StartMs: 0, Text: "x"}},
		},
		{
			name:  "form B trailing whitespace-only line",
			input: "0:00\nx\n \n",
			want:  []Segment{{StartMs: 0, Text: "x"}},
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
	type rejectCase struct {
		name   string
		input  string
		reason error
		line   int
	}
	cases := []rejectCase{
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
		{name: "form A empty line in the body", input: "0:000 秒はい\n\n0:1010 秒です\n", reason: errCopiedEmptyLine, line: 2},
		{name: "control tab in text", input: "0:00\nx\ty\n", reason: errCopiedControlChar, line: 2},
		{name: "control NUL in timestamp", input: "0:00\x00\nx\n", reason: errCopiedControlChar, line: 1},
		{name: "leading display items", input: "タイムライン\n", reason: errCopiedNoText},
		{name: "prefix only", input: "文字起こし\n\n動画の関連情報", reason: errCopiedNoText},
		{name: "prefix and operation label only", input: "文字起こし\n動画の時間と同期\n", reason: errCopiedNoText},
		{name: "prefix minute leading zero", input: "文字起こし\n05:00\n0:00\nx\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "prefix single-digit second", input: "文字起こし\n0:5\n0:00\nx\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "prefix leading BOM", input: "文字起こし\n\uFEFF0:00\nx\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "prefix leading space", input: "文字起こし\n 0:00\nx\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "prefix control character", input: "文字起こし\nタイ\tムライン\n0:00\nx\n", reason: errCopiedControlChar, line: 2},
		{name: "body error line counts the prefix", input: "文字起こし\n\n0:000 秒はい\n0:05BOGUS\n", reason: errCopiedLabelMismatch, line: 4},
		{name: "form B body error line counts the prefix", input: "文字起こし\n\n0:05\nx\n0:00\ny\n", reason: errCopiedTimestampOrder, line: 5},
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
		{name: "form B whitespace-only text", input: "0:00\n \n", reason: errCopiedNoText},
		{name: "form B whitespace-only text in the body", input: "0:00\n \n0:05\nx\n", reason: errCopiedBlankText, line: 2},
		{name: "operation label between segments", input: "0:2424 秒はい\n動画の時間と同期\n0:1010 秒です\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "trailing line resembling the operation label", input: "0:2424 秒はい\n動画の時間と同期について話します\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "trailing operation label with a trailing space", input: "0:2424 秒はい\n動画の時間と同期 \n", reason: errCopiedNotTimestamp, line: 2},
		{name: "trailing English operation label prefix", input: "0:2424 secondsはい\nSync to video\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "trailing English operation label with different case", input: "0:2424 secondsはい\nsync to video time\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "trailing control character", input: "0:000 秒はい\n\n\t\n", reason: errCopiedControlChar, line: 3},
		{name: "operation label only", input: "動画の時間と同期\n", reason: errCopiedNoText},
		{name: "English chapter heading in lowercase", input: "0:000 secondsHello\nchapter 2: title\n0:055 secondsWorld\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "chapter heading with a control character", input: "0:000 秒はい\nチャプター 2: タイ\tトル\n0:1010 秒です\n", reason: errCopiedControlChar, line: 2},
		{name: "chapter heading only", input: "チャプター 1: はじめに\n", reason: errCopiedNoText},
		{name: "form A chapter heading after a textless segment only", input: "0:000 秒\nチャプター 1: はじめに\n", reason: errCopiedNoText},
		{name: "form A display item in the body", input: "0:000 秒はい\nタイムライン\n0:1010 秒です\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "form A plain text line in the body", input: "0:000 秒はい\nこんにちは\n0:1010 秒です\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "form A unknown Japanese label in the body", input: "0:000 秒はい\nチャプターの一覧\n0:1010 秒です\n", reason: errCopiedNotTimestamp, line: 2},
		{name: "form A unknown English label in the body", input: "0:000 secondsHello\nDownload transcript\n0:055 secondsWorld\n", reason: errCopiedNotTimestamp, line: 2},
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
	// Each near-miss differs from a heading in one part of its shape. The rows
	// are generated for every heading word, because each word has its own
	// pattern and pinning one says nothing about the other.
	nearMissHeadings := []struct {
		name   string
		format string
	}{
		{name: "without a number", format: "%s: Title"},
		{name: "number zero", format: "%s 0: Title"},
		{name: "number with a leading zero", format: "%s 01: Title"},
		{name: "full-width number", format: "%s ２: Title"},
		{name: "without the space before the number", format: "%s2: Title"},
		{name: "two spaces before the number", format: "%s  2: Title"},
		{name: "U+3000 before the number", format: "%s　2: Title"},
		{name: "without the colon", format: "%s 2 Title"},
		{name: "with a full-width colon", format: "%s 2： Title"},
		{name: "without the space after the colon", format: "%s 2:Title"},
		{name: "with an empty title", format: "%s 2: "},
		{name: "title ends in U+3000", format: "%s 2: Title　"},
		{name: "title ends in U+0020", format: "%s 2: Title "},
		{name: "with a leading space", format: " %s 2: Title"},
	}
	for _, word := range []string{"チャプター", "Chapter"} {
		for _, nearMiss := range nearMissHeadings {
			cases = append(cases, rejectCase{
				name:   word + " heading " + nearMiss.name,
				input:  "0:000 秒はい\n" + fmt.Sprintf(nearMiss.format, word) + "\n0:1010 秒です\n",
				reason: errCopiedNotTimestamp,
				line:   2,
			})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := assertCopiedRejected(t, tc.input, tc.reason)
			// The line number has no typed form, so the message text is the
			// contract under test; assertCopiedRejected already checked the
			// reason with errors.Is.
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
}

// TestParseCopiedTrailingRegion checks that the trailing region (blank lines
// and the panel's operation label) is skipped. Each skip case compares the
// parsed segments against the input with the trailing region removed, which
// never enters the skip path, and against the literal expectation.
func TestParseCopiedTrailingRegion(t *testing.T) {
	formASegments := []Segment{{StartMs: 24000, Text: "Hello"}, {StartMs: 60000, Text: "World"}}
	formBSegments := []Segment{{StartMs: 0, Text: "Hello"}, {StartMs: 5000, Text: "World"}}
	formABody := "0:2424 秒Hello\n1:001 分World\n"
	formAEnglishBody := "0:2424 secondsHello\n1:001 minuteWorld\n"
	formBBody := "0:00\nHello\n0:05\nWorld\n"

	cases := []struct {
		name  string
		input string
		want  []Segment
		body  string
	}{
		{
			name:  "form A blank line then the Japanese operation label",
			input: formABody + "\n動画の時間と同期",
			want:  formASegments,
			body:  formABody,
		},
		{
			name:  "form A blank line then the English operation label",
			input: formAEnglishBody + "\nSync to video time",
			want:  formASegments,
			body:  formAEnglishBody,
		},
		{
			name:  "form A one trailing empty line",
			input: formABody + "\n",
			want:  formASegments,
			body:  formABody,
		},
		{
			name:  "form A trailing U+0020 only line",
			input: formABody + " \n",
			want:  formASegments,
			body:  formABody,
		},
		{
			name:  "form A trailing U+3000 only line",
			input: formABody + "\u3000\n",
			want:  formASegments,
			body:  formABody,
		},
		{
			name:  "form A two trailing empty lines",
			input: formABody + "\n\n",
			want:  formASegments,
			body:  formABody,
		},
		{
			name:  "form A mixed empty lines and operation label",
			input: formABody + "\n動画の時間と同期\n\n",
			want:  formASegments,
			body:  formABody,
		},
		{
			name:  "form B one trailing empty line",
			input: formBBody + "\n",
			want:  formBSegments,
			body:  formBBody,
		},
		{
			name:  "form B trailing whitespace-only line after a timestamp",
			input: "0:00\nx\n0:05\n \n",
			want:  []Segment{{StartMs: 0, Text: "x"}},
			body:  "0:00\nx\n0:05\n",
		},
		{
			name:  "form B blank line then the Japanese operation label",
			input: formBBody + "\n動画の時間と同期",
			want:  formBSegments,
			body:  formBBody,
		},
		{
			name:  "form B operation label after a timestamp is the segment text",
			input: "0:00\n前の字幕\n0:05\n動画の時間と同期",
			want:  []Segment{{StartMs: 0, Text: "前の字幕"}, {StartMs: 5000, Text: "動画の時間と同期"}},
		},
		{
			name:  "form B trailing operation label after a blank line",
			input: "0:00\n前の字幕\n0:05\n本文\n\n動画の時間と同期",
			want:  []Segment{{StartMs: 0, Text: "前の字幕"}, {StartMs: 5000, Text: "本文"}},
			body:  "0:00\n前の字幕\n0:05\n本文\n",
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
			if tc.body == "" {
				return
			}
			assertCopiedBodyHasNoSkippedLine(t, tc.body)
			bodySegments, err := parseCopiedTranscript(tc.body)
			if err != nil {
				t.Fatalf("parseCopiedTranscript(%q) error = %v", tc.body, err)
			}
			if !reflect.DeepEqual(bodySegments, tc.want) {
				t.Errorf("parseCopiedTranscript(%q) = %+v, want the same segments %+v", tc.body, bodySegments, tc.want)
			}
		})
	}
}

// assertCopiedBodyHasNoSkippedLine fails unless body contains only form A or
// form B lines: the first line starts with a timestamp, none of its lines is
// blank or whitespace-only, none is a form A chapter heading, and it has no
// trailing region. Such an input never enters a skip path, so a skip test can
// use it as the comparison target. The heading and trailing-region checks call
// the production functions, so a new skip kind is caught here rather than
// silently skipping a comparison line.
func assertCopiedBodyHasNoSkippedLine(t *testing.T, body string) {
	t.Helper()
	separator, err := copiedLineSeparator(body)
	if err != nil {
		t.Fatalf("comparison body %q has inconsistent line breaks: %v", body, err)
	}
	lines := splitCopiedLines(body, separator)
	if _, _, ok := parseTimestampPrefix(lines[0]); !ok {
		t.Fatalf("comparison body %q does not start with a timestamp", body)
	}
	for i, line := range lines {
		if copiedBlankLine(line) {
			t.Fatalf("comparison body line %d %q is blank or whitespace-only", i+1, line)
		}
		if copiedIsChapterHeading(line) {
			t.Fatalf("comparison body line %d %q is a chapter heading", i+1, line)
		}
	}
	if end := copiedTrailingStart(lines); end != len(lines) {
		t.Fatalf("comparison body %q has a trailing region from line %d", body, end+1)
	}
}

// TestParseCopiedPrefix compares an input that has a prefix before the first
// segment line with the same input without it. The prefix lines must not reach
// any segment's text (the literal want pins every text), and the prefix must
// not decide the form. The near-miss
// case pins the digit-colon-digit rule from the accepting side: a colon or a
// digit alone, a colon between non-digits or with a space, and full-width
// digits are not a run, so those prefix lines are skipped.
func TestParseCopiedPrefix(t *testing.T) {
	formABody := "0:000 秒はい\n0:1010 秒です\n"
	formBBody := "0:00\nHello\n0:05\nWorld\n"
	englishBody := "0:000 secondsHello\n0:055 secondsWorld\n"
	helloWorld := []Segment{{StartMs: 0, Text: "Hello"}, {StartMs: 5000, Text: "World"}}
	cases := []struct {
		name   string
		prefix []string
		body   string
		want   []Segment
	}{
		{
			name:   "observed panel prefix then form A",
			prefix: []string{"動画の関連情報", "", "", "タイムライン", "", "文字起こし", "文字音声変換を検索", "文字音声変換を検索", ""},
			body:   formABody,
			want:   []Segment{{StartMs: 0, Text: "はい"}, {StartMs: 10000, Text: "です"}},
		},
		{
			name:   "prefix with a plain chapter title then form B",
			prefix: []string{"文字起こし", "動画内を検索", "はじめの話"},
			body:   formBBody,
			want:   helloWorld,
		},
		{
			name:   "Japanese prefix then English labels",
			prefix: []string{"文字起こし", "\u3000"},
			body:   englishBody,
			want:   helloWorld,
		},
		{
			name:   "prefix lines without a digit-colon-digit run",
			prefix: []string{"チャプター 1: はじめに", "16 : 9", "Note: x", "第3戦", "１：００", "x:1", "1:x"},
			body:   formBBody,
			want:   helloWorld,
		},
		{
			name:   "form A empty line at the start",
			prefix: []string{""},
			body:   "0:000 秒はい\n",
			want:   []Segment{{StartMs: 0, Text: "はい"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCopiedBodyHasNoSkippedLine(t, tc.body)
			input := strings.Join(tc.prefix, "\n") + "\n" + tc.body
			got, err := parseCopiedTranscript(input)
			if err != nil {
				t.Fatalf("parseCopiedTranscript(%q) error = %v", input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseCopiedTranscript(%q) = %+v, want %+v", input, got, tc.want)
			}
			bodySegments, err := parseCopiedTranscript(tc.body)
			if err != nil {
				t.Fatalf("parseCopiedTranscript(%q) error = %v", tc.body, err)
			}
			if !reflect.DeepEqual(bodySegments, tc.want) {
				t.Errorf("parseCopiedTranscript(%q) = %+v, want the same segments %+v", tc.body, bodySegments, tc.want)
			}
		})
	}
}

// TestParseCopiedFormAChapterHeadings compares a form A input that has chapter
// heading lines with the same input without them. The literal want pins every
// segment's text and the segment count, so a heading title that reached a
// segment, or a heading that produced or dropped one, fails the comparison. The
// panel case also carries the prefix and the trailing region of a whole-panel
// selection.
func TestParseCopiedFormAChapterHeadings(t *testing.T) {
	cases := []struct {
		name  string
		input string
		body  string
		want  []Segment
	}{
		{
			name:  "Japanese headings between segments",
			input: "0:000 秒はい\nチャプター 1: はじめに\n0:1010 秒です\nチャプター 2: 3つの準備と1つの注意\n0:2020 秒以上\n",
			body:  "0:000 秒はい\n0:1010 秒です\n0:2020 秒以上\n",
			want:  []Segment{{StartMs: 0, Text: "はい"}, {StartMs: 10000, Text: "です"}, {StartMs: 20000, Text: "以上"}},
		},
		{
			name:  "heading after the last segment",
			input: "0:000 秒はい\n0:1010 秒です\nチャプター 3: おわりに\n",
			body:  "0:000 秒はい\n0:1010 秒です\n",
			want:  []Segment{{StartMs: 0, Text: "はい"}, {StartMs: 10000, Text: "です"}},
		},
		{
			name:  "heading after the last segment before the trailing region",
			input: "0:000 秒はい\n0:1010 秒です\nチャプター 3: おわりに\n\n動画の時間と同期",
			body:  "0:000 秒はい\n0:1010 秒です\n",
			want:  []Segment{{StartMs: 0, Text: "はい"}, {StartMs: 10000, Text: "です"}},
		},
		{
			name:  "Japanese heading with English labels",
			input: "0:000 secondsHello\nチャプター 1: はじめに\n0:055 secondsWorld\n",
			body:  "0:000 secondsHello\n0:055 secondsWorld\n",
			want:  []Segment{{StartMs: 0, Text: "Hello"}, {StartMs: 5000, Text: "World"}},
		},
		{
			name:  "English heading with Japanese labels",
			input: "0:000 秒はい\nChapter 2: Intro\n0:1010 秒です\n",
			body:  "0:000 秒はい\n0:1010 秒です\n",
			want:  []Segment{{StartMs: 0, Text: "はい"}, {StartMs: 10000, Text: "です"}},
		},
		{
			name:  "titles with a colon, spaces, and the heading word",
			input: "0:000 秒はい\nチャプター 5: 手順 2: 次のチャプターの準備\n0:1010 秒です\nチャプター 12: 朝 と 夜\nチャプター 13:  前に空白\n0:2020 秒以上\n",
			body:  "0:000 秒はい\n0:1010 秒です\n0:2020 秒以上\n",
			want:  []Segment{{StartMs: 0, Text: "はい"}, {StartMs: 10000, Text: "です"}, {StartMs: 20000, Text: "以上"}},
		},
		{
			name:  "English titles with a colon, spaces, and the heading word",
			input: "0:000 secondsHello\nChapter 12: Step 2: Next Chapter\n0:055 secondsWorld\nChapter 13:  Leading space\n0:1010 secondsBye\n",
			body:  "0:000 secondsHello\n0:055 secondsWorld\n0:1010 secondsBye\n",
			want:  []Segment{{StartMs: 0, Text: "Hello"}, {StartMs: 5000, Text: "World"}, {StartMs: 10000, Text: "Bye"}},
		},
		{
			// The first heading precedes the first segment line, so the prefix
			// skips it; only the heading after it reaches the form A heading skip.
			name: "whole panel selection",
			input: "動画の関連情報\n\n\nタイムライン\n\n文字起こし\n文字音声変換を検索\n文字音声変換を検索\n\n" +
				"チャプター 1: はじめに\n0:011 秒はい\n0:1010 秒です\nチャプター 2: 本題\n1:001 分以上\n\n動画の時間と同期",
			body: "0:011 秒はい\n0:1010 秒です\n1:001 分以上\n",
			want: []Segment{{StartMs: 1000, Text: "はい"}, {StartMs: 10000, Text: "です"}, {StartMs: 60000, Text: "以上"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCopiedBodyHasNoSkippedLine(t, tc.body)
			got, err := parseCopiedTranscript(tc.input)
			if err != nil {
				t.Fatalf("parseCopiedTranscript(%q) error = %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseCopiedTranscript(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
			bodySegments, err := parseCopiedTranscript(tc.body)
			if err != nil {
				t.Fatalf("parseCopiedTranscript(%q) error = %v", tc.body, err)
			}
			if !reflect.DeepEqual(bodySegments, tc.want) {
				t.Errorf("parseCopiedTranscript(%q) = %+v, want the same segments %+v", tc.body, bodySegments, tc.want)
			}
		})
	}
}

func TestParseCopiedCheckOrder(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    error
		line    int
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
			input:   "0:00\nx\n \n0:05\ny\n",
			want:    errCopiedNotTimestamp,
			notWant: []error{errCopiedBlankText},
		},
		{
			name:    "trailing control character before the no-text check",
			input:   "0:000 秒\n\t\n",
			want:    errCopiedControlChar,
			line:    2,
			notWant: []error{errCopiedNoText},
		},
		{
			name:    "empty body skips the trailing control-character check",
			input:   "動画の時間と同期\n\t\n",
			want:    errCopiedNoText,
			notWant: []error{errCopiedControlChar},
		},
		{
			name:    "prefix control character before digit-colon-digit",
			input:   "文字起こし\n1:2\t3\n0:00\nx\n",
			want:    errCopiedControlChar,
			line:    2,
			notWant: []error{errCopiedNotTimestamp},
		},
		{
			name:    "prefix scan stops before the trailing region",
			input:   "文字起こし\n\t\n",
			want:    errCopiedNoText,
			notWant: []error{errCopiedControlChar},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := assertCopiedRejected(t, tc.input, tc.want)
			// The line number has no typed form, so the message text is the
			// contract under test; assertCopiedRejected already checked the
			// reason with errors.Is.
			if tc.line > 0 && !strings.Contains(message, fmt.Sprintf("line %d:", tc.line)) {
				t.Errorf("Error() = %q, want line %d", message, tc.line)
			}
			_, err := parseCopiedTranscript(tc.input)
			for _, other := range tc.notWant {
				if errors.Is(err, other) {
					t.Errorf("parseCopiedTranscript(%q) error = %v, must not wrap %v", tc.input, err, other)
				}
			}
		})
	}
}

func TestParseCopiedErrorOmitsInput(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		reason  error
		markers []string
	}{
		{
			name:    "label mismatch omits the line",
			reason:  errCopiedLabelMismatch,
			input:   "0:24XX24 秒YY",
			markers: []string{"XX", "YY", "0:24"},
		},
		{
			name:    "not a timestamp omits the line",
			reason:  errCopiedNotTimestamp,
			input:   "0:000 秒x\nMARKER_LINE\n",
			markers: []string{"MARKER_LINE"},
		},
		{
			name:    "form A near-miss chapter heading omits the line",
			reason:  errCopiedNotTimestamp,
			input:   "0:000 秒x\nチャプター 0: HEADING_MARKER\n0:1010 秒y\n",
			markers: []string{"HEADING_MARKER", "チャプター"},
		},
		{
			name:    "prefix digit-colon-digit omits the line",
			reason:  errCopiedNotTimestamp,
			input:   "\nPREFIX_A 1:23 PREFIX_B\n0:00\nx\n",
			markers: []string{"PREFIX_A", "PREFIX_B", "1:23"},
		},
		{
			name:    "prefix control character omits the line",
			reason:  errCopiedControlChar,
			input:   "文字起こし\nCONTROL_MARKER\x01\n0:00\nx\n",
			markers: []string{"CONTROL_MARKER"},
		},
		{
			name:    "timestamp order omits the bodies",
			reason:  errCopiedTimestampOrder,
			input:   "0:2424 秒AAA\n0:1010 秒BBB\n",
			markers: []string{"AAA", "BBB", "0:24", "0:10"},
		},
		{
			name:    "mixed forms omits the line",
			reason:  errCopiedMixedForms,
			input:   "0:00\n0:2424 秒ZZZ\n",
			markers: []string{"ZZZ", "0:24"},
		},
		{
			name:    "chapter heading omits the line",
			reason:  errCopiedNotTimestamp,
			input:   "0:00\nTEXT_MARKER\nCHAPTER_MARKER\n",
			markers: []string{"CHAPTER_MARKER", "TEXT_MARKER"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := assertCopiedRejected(t, tc.input, tc.reason)
			for _, marker := range tc.markers {
				if strings.Contains(message, marker) {
					t.Errorf("Error() = %q, must not contain %q", message, marker)
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

// The server caps the request body at 4 MiB; the parser must finish such an
// input within a small part of the server's request window and without
// allocating super-linearly. The deadline is generous because make test builds
// with the race detector (measured about 3.2s for the 4 MiB inputs under -race)
// and CI may run on a slower, contended runner; the parse budget on the server
// is under 30s. copiedParseAllocPerByte is defined per build: the race
// detector's runtime inflates TotalAlloc, so the race build uses a loose bound.
const (
	copiedLargeInputLimit = 4 << 20
	copiedParseDeadline   = 15 * time.Second
)

// TestParseCopiedLargeInputs parses three synthetic 4 MiB inputs: the shortest
// form A line repeated (the most segments), one form A line nearly 4 MiB long,
// and many textless form B timestamps followed by one text. Each must parse
// within the deadline and allocate at most copiedParseAllocPerByte bytes per
// input byte. The test does not run in parallel, so TotalAlloc around the parse
// measures only this parse.
func TestParseCopiedLargeInputs(t *testing.T) {
	inputs := []struct {
		name         string
		input        string
		wantSegments int
	}{
		{"shortest form A lines", buildCopiedShortestFormALines(copiedLargeInputLimit), copiedLargeInputLimit / len("0:000 秒x\n")},
		{"one long form A line", buildCopiedLongFormALine(copiedLargeInputLimit), 1},
		{"textless form B timestamps", buildCopiedTextlessFormB(copiedLargeInputLimit), 1},
	}
	for _, tc := range inputs {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.input) > copiedLargeInputLimit {
				t.Fatalf("input is %d bytes, over the %d limit", len(tc.input), copiedLargeInputLimit)
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			segments, err := parseCopiedTranscript(tc.input)
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatalf("parse error = %v", err)
			}
			if len(segments) != tc.wantSegments {
				t.Errorf("parsed into %d segments, want %d", len(segments), tc.wantSegments)
			}
			if elapsed > copiedParseDeadline {
				t.Errorf("parsed %d bytes in %s, want under %s", len(tc.input), elapsed, copiedParseDeadline)
			}
			allocated := after.TotalAlloc - before.TotalAlloc
			if allocated > copiedParseAllocPerByte*uint64(len(tc.input)) {
				t.Errorf("parse allocated %d bytes for %d input bytes, want at most %d per byte",
					allocated, len(tc.input), copiedParseAllocPerByte)
			}
			t.Logf("%d bytes, %d segments, %s, %d allocated (%.2f per byte)",
				len(tc.input), len(segments), elapsed, allocated, float64(allocated)/float64(len(tc.input)))
		})
	}
}

func buildCopiedShortestFormALines(limit int) string {
	line := "0:000 秒x\n"
	return strings.Repeat(line, limit/len(line))
}

func buildCopiedLongFormALine(limit int) string {
	prefix := "0:000 秒"
	return prefix + strings.Repeat("x", limit-len(prefix))
}

func buildCopiedTextlessFormB(limit int) string {
	timestamp := "0:00\n"
	tail := "0:05\nx"
	return strings.Repeat(timestamp, (limit-len(tail))/len(timestamp)) + tail
}

// FuzzNewCopiedSource checks six properties: it never panics; an accepted
// transcript has a non-whitespace, valid-UTF-8 text and a non-decreasing
// StartMs for every segment; appending a trailing region to an accepted input
// leaves the segments unchanged; prepending a prefix to an accepted input
// leaves the segments unchanged; replacing LF with CRLF in a CR-free input
// gives the same result; and a rejection wraps ErrCopiedTranscriptFormat with a
// fixed reason and an optional line number only. The seeds are the committed
// real fixtures and a few table inputs; make test runs only the seeds.
func FuzzNewCopiedSource(f *testing.F) {
	paths, err := filepath.Glob(testdataCopiedTranscripts)
	if err != nil {
		f.Fatalf("glob %s: %v", testdataCopiedTranscripts, err)
	}
	if len(paths) != len(copiedSampleFiles) {
		f.Fatalf("found %d copied transcript fixtures, want %d", len(paths), len(copiedSampleFiles))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // the path names a committed testdata fixture
		if err != nil {
			f.Fatalf("read %s: %v", path, err)
		}
		f.Add(string(data))
	}
	f.Add("0:2424 secondsHello\n")
	f.Add("0:00\n0:05\nこんにちは\n")
	f.Add("0:000 秒\n")
	f.Add("not a transcript\n")
	f.Add("1:251 minute 25 secondsHello\n")
	f.Add("0:00\n1:30 から始めます\n")
	f.Add("0:00\nx\n\n動画の時間と同期")
	f.Add("0:2424 秒Hello\n\nSync to video time")
	f.Add("文字起こし\n\n0:2424 秒Hello\n")
	f.Add("文字起こし\n動画内を検索\n0:00\nx\n")
	f.Add("0:000 秒はい\nチャプター 2: 次へ\n0:1010 秒です\n")

	f.Fuzz(func(t *testing.T, raw string) {
		source, err := NewCopiedSource(CopiedInput{VideoID: copiedTestVideoID, Raw: raw})
		if err != nil {
			assertCopiedFuzzRejection(t, err)
			return
		}
		transcript, err := source.Fetch(context.Background(), copiedTestVideoURL)
		if err != nil {
			t.Fatalf("Fetch after a successful NewCopiedSource = %v", err)
		}
		assertCopiedFuzzAcceptance(t, transcript, raw)
	})
}

func assertCopiedFuzzRejection(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrCopiedTranscriptFormat) {
		t.Fatalf("error = %v, want ErrCopiedTranscriptFormat", err)
	}
	if errors.Is(err, ErrInvalidVideoURL) {
		t.Errorf("error = %v, also matches ErrInvalidVideoURL", err)
	}
	if !copiedErrorMessageOK(err.Error()) {
		t.Errorf("message = %q, want a fixed reason and an optional line number", err.Error())
	}
}

func assertCopiedFuzzAcceptance(t *testing.T, transcript Transcript, raw string) {
	t.Helper()
	if len(transcript.Segments) == 0 {
		t.Fatal("an accepted input produced no segments")
	}
	previous := int64(-1)
	for i, segment := range transcript.Segments {
		if !utf8.ValidString(segment.Text) {
			t.Errorf("segment %d text is not valid UTF-8", i)
		}
		if strings.TrimSpace(segment.Text) == "" {
			t.Errorf("segment %d text is whitespace only", i)
		}
		if segment.StartMs < previous {
			t.Errorf("segment %d StartMs %d is earlier than %d", i, segment.StartMs, previous)
		}
		previous = segment.StartMs
	}
	if !strings.Contains(raw, "\r") {
		crlfSegments, err := parseCopiedTranscript(strings.ReplaceAll(raw, "\n", "\r\n"))
		if err != nil {
			t.Fatalf("the CRLF version of an accepted input was rejected: %v", err)
		}
		if !reflect.DeepEqual(transcript.Segments, crlfSegments) {
			t.Error("LF and CRLF versions of the same input differ")
		}
	}
	assertCopiedFuzzTrailingRegion(t, transcript.Segments, raw)
	assertCopiedFuzzPrefix(t, transcript.Segments, raw)
}

// copiedFuzzSeparator returns the separator to append or prepend lines to an
// accepted input with: the input's own, or LF for a single-line input.
func copiedFuzzSeparator(t *testing.T, raw string) string {
	t.Helper()
	separator, err := copiedLineSeparator(raw)
	if err != nil {
		t.Fatalf("an accepted input has inconsistent line breaks: %v", err)
	}
	if separator == "" {
		return "\n"
	}
	return separator
}

// assertCopiedFuzzPrefix checks property (b): prepending prefix lines that start
// with no timestamp and that the prefix rules do not reject (a panel label, an
// empty line, a whitespace-only line, a chapter heading with a digit and a
// colon but no digit-colon-digit run) leaves the segments unchanged. The
// prepended lines use the input's own separator, or LF for a single-line input.
func assertCopiedFuzzPrefix(t *testing.T, want []Segment, raw string) {
	t.Helper()
	separator := copiedFuzzSeparator(t, raw)
	prefix := strings.Join([]string{"文字起こし", "", " ", "チャプター 1: はじめに", "動画内を検索"}, separator) + separator
	got, err := parseCopiedTranscript(prefix + raw)
	if err != nil {
		t.Fatalf("prepending a prefix rejected an accepted input: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("prepending a prefix changed the segments: got %+v, want %+v", got, want)
	}
}

// assertCopiedFuzzTrailingRegion checks property (a): appending a run of empty
// or whitespace-only lines, with or without a trailing operation-label line
// after them, leaves the segments unchanged. The blank run keeps the operation
// label's previous line from being a timestamp-only line, so the label stays in
// the trailing region instead of becoming a segment's text. The appended lines
// use the input's own separator, or LF for a single-line input, and a single
// U+0020 for the whitespace-only run so the run is not also a control character.
func assertCopiedFuzzTrailingRegion(t *testing.T, want []Segment, raw string) {
	t.Helper()
	separator := copiedFuzzSeparator(t, raw)
	labels := append([]string{""}, copiedOperationLabels...)
	for _, run := range []string{separator + separator, separator + " "} {
		for _, label := range labels {
			appended := raw + run
			if label != "" {
				appended += separator + label
			}
			got, err := parseCopiedTranscript(appended)
			if err != nil {
				t.Fatalf("appending a trailing region %q with label %q rejected an accepted input: %v", run, label, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("appending a trailing region %q with label %q changed the segments: got %+v, want %+v", run, label, got, want)
			}
		}
	}
}

// copiedErrorMessageOK reports whether message is the fixed prefix plus either
// a bare reason string or "line N: <reason>".
func copiedErrorMessageOK(message string) bool {
	rest, ok := strings.CutPrefix(message, ErrCopiedTranscriptFormat.Error()+": ")
	if !ok {
		return false
	}
	if isCopiedReason(rest) {
		return true
	}
	number, reason, ok := strings.Cut(rest, ": ")
	if !ok || !strings.HasPrefix(number, "line ") {
		return false
	}
	if _, err := strconv.Atoi(strings.TrimPrefix(number, "line ")); err != nil {
		return false
	}
	return isCopiedReason(reason)
}

func isCopiedReason(text string) bool {
	for _, reason := range []error{
		errCopiedInvalidUTF8,
		errCopiedBlank,
		errCopiedLineBreak,
		errCopiedEmptyLine,
		errCopiedControlChar,
		errCopiedNotTimestamp,
		errCopiedLabelMismatch,
		errCopiedBlankText,
		errCopiedMixedForms,
		errCopiedTimestampOrder,
		errCopiedNoText,
	} {
		if reason.Error() == text {
			return true
		}
	}
	return false
}
