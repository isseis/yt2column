//go:build test

package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseSubtitlesRealData(t *testing.T) {
	data := readTestdataFile(t, testdataRealSubtitles)

	segments, err := parseSubtitles(testdataRealSubtitles, data)
	if err != nil {
		t.Fatalf("parseSubtitles(%s) error = %v", testdataRealSubtitles, err)
	}
	want := subtitlesOracle(t, data)
	if len(segments) == 0 {
		t.Fatal("expected at least one segment from the real fixture")
	}
	if len(segments) != len(want) {
		t.Fatalf("got %d segments, want %d", len(segments), len(want))
	}
	for i := range want {
		if segments[i] != want[i] {
			t.Errorf("segment %d = %+v, want %+v", i, segments[i], want[i])
		}
	}
	for i, segment := range segments {
		if strings.TrimSpace(segment.Text) == "" {
			t.Errorf("segment %d has whitespace-only text %q", i, segment.Text)
		}
	}
}

func TestParseSubtitlesEmptySegs(t *testing.T) {
	cases := map[string]string{
		"empty utf8":       `{"events":[{"tStartMs":0,"segs":[{"utf8":""}]}]}`,
		"absent utf8":      `{"events":[{"tStartMs":0,"segs":[{}]}]}`,
		"no segs member":   `{"events":[{"tStartMs":0}]}`,
		"whitespace text":  `{"events":[{"tStartMs":0,"segs":[{"utf8":" "}]}]}`,
		"no events":        `{"events":[]}`,
		"empty segs array": `{"events":[{"tStartMs":0,"segs":[]}]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			segments, err := parseSubtitles("inline.json3", []byte(input))
			if err != nil {
				t.Fatalf("parseSubtitles(%q) error = %v", input, err)
			}
			if len(segments) != 0 {
				t.Errorf("got %d segments, want 0: %+v", len(segments), segments)
			}
		})
	}
}

func TestParseSubtitlesRejectsSimple(t *testing.T) {
	cases := map[string]string{
		"truncated":        `{`,
		"not json":         `not json`,
		"top-level array":  `[]`,
		"top-level null":   `null`,
		"top-level number": `42`,
		"top-level string": `"x"`,
		"missing events":   `{}`,
		"events null":      `{"events":null}`,
		"events object":    `{"events":{}}`,
		"events string":    `{"events":"x"}`,
		"events number":    `{"events":7}`,
		"trailing object":  `{"events":[]} {}`,
		"trailing text":    `{"events":[]} x`,
		"trailing char":    `{"events":[]}x`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			assertSubtitlesRejected(t, input)
		})
	}
}

func TestParseSubtitlesElementKinds(t *testing.T) {
	cases := map[string]string{
		"event null":   `{"events":[null]}`,
		"event number": `{"events":[1]}`,
		"event string": `{"events":["x"]}`,
		"event array":  `{"events":[[]]}`,
		"mixed null":   `{"events":[{"segs":[]},null]}`,
		"segs object":  `{"events":[{"tStartMs":0,"segs":{}}]}`,
		"segs null":    `{"events":[{"tStartMs":0,"segs":null}]}`,
		"segs string":  `{"events":[{"tStartMs":0,"segs":"x"}]}`,
		"seg null":     `{"events":[{"tStartMs":0,"segs":[{"utf8":"kept"},null]}]}`,
		"seg number":   `{"events":[{"tStartMs":0,"segs":[1]}]}`,
		"utf8 null":    `{"events":[{"tStartMs":0,"segs":[{"utf8":null}]}]}`,
		"utf8 number":  `{"events":[{"tStartMs":0,"segs":[{"utf8":5}]}]}`,
		"utf8 object":  `{"events":[{"tStartMs":0,"segs":[{"utf8":{}}]}]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			assertSubtitlesRejected(t, input)
		})
	}
}

func TestParseSubtitlesTimestamps(t *testing.T) {
	cases := map[string]string{
		"missing tStartMs": `{"events":[{"segs":[{"utf8":"hi"}]}]}`,
		"null tStartMs":    `{"events":[{"tStartMs":null,"segs":[{"utf8":"hi"}]}]}`,
		"negative":         `{"events":[{"tStartMs":-1,"segs":[{"utf8":"hi"}]}]}`,
		"fractional":       `{"events":[{"tStartMs":1.5,"segs":[{"utf8":"hi"}]}]}`,
		"string":           `{"events":[{"tStartMs":"0","segs":[{"utf8":"hi"}]}]}`,
		"overflow":         `{"events":[{"tStartMs":9223372036854775808,"segs":[{"utf8":"hi"}]}]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			assertSubtitlesRejected(t, input)
		})
	}

	t.Run("zero is valid", func(t *testing.T) {
		segments, err := parseSubtitles("inline.json3", []byte(`{"events":[{"tStartMs":0,"segs":[{"utf8":"hi"}]}]}`))
		if err != nil {
			t.Fatalf("parseSubtitles error = %v", err)
		}
		want := []Segment{{StartMs: 0, Text: "hi"}}
		if !reflect.DeepEqual(segments, want) {
			t.Errorf("segments = %+v, want %+v", segments, want)
		}
	})

	t.Run("discarded event ignores tStartMs", func(t *testing.T) {
		segments, err := parseSubtitles("inline.json3", []byte(`{"events":[{"tStartMs":null,"segs":[]}]}`))
		if err != nil {
			t.Fatalf("parseSubtitles error = %v", err)
		}
		if len(segments) != 0 {
			t.Errorf("got %d segments, want 0", len(segments))
		}
	})
}

func TestParseSubtitlesKeepsRepeatedText(t *testing.T) {
	input := `{"events":[{"tStartMs":0,"segs":[{"utf8":"hello"}]},{"tStartMs":5000,"segs":[{"utf8":"hello"}]}]}`
	segments, err := parseSubtitles("inline.json3", []byte(input))
	if err != nil {
		t.Fatalf("parseSubtitles error = %v", err)
	}
	want := []Segment{{StartMs: 0, Text: "hello"}, {StartMs: 5000, Text: "hello"}}
	if !reflect.DeepEqual(segments, want) {
		t.Errorf("segments = %+v, want %+v", segments, want)
	}
}

func TestParseSubtitlesDuplicateMembers(t *testing.T) {
	rejected := map[string]string{
		"duplicate events":   `{"events":[],"events":[]}`,
		"duplicate tStartMs": `{"events":[{"tStartMs":0,"tStartMs":1,"segs":[{"utf8":"hi"}]}]}`,
		"duplicate segs":     `{"events":[{"tStartMs":0,"segs":[],"segs":[]}]}`,
		"duplicate utf8":     `{"events":[{"tStartMs":0,"segs":[{"utf8":"a","utf8":"b"}]}]}`,
	}
	for name, input := range rejected {
		t.Run("reject/"+name, func(t *testing.T) {
			assertSubtitlesRejected(t, input)
		})
	}

	accepted := map[string]string{
		"unknown top-level": `{"events":[],"pens":1,"pens":2}`,
		"unknown event":     `{"events":[{"tStartMs":0,"segs":[{"utf8":"hi"}],"aAppend":1,"aAppend":2}]}`,
		"unknown seg":       `{"events":[{"tStartMs":0,"segs":[{"utf8":"hi","wc":1,"wc":2}]}]}`,
	}
	for name, input := range accepted {
		t.Run("accept/"+name, func(t *testing.T) {
			if _, err := parseSubtitles("inline.json3", []byte(input)); err != nil {
				t.Errorf("parseSubtitles(%q) error = %v", input, err)
			}
		})
	}

	t.Run("accept/discarded duplicate tStartMs", func(t *testing.T) {
		input := `{"events":[{"tStartMs":0,"tStartMs":1,"segs":[]}]}`
		segments, err := parseSubtitles("inline.json3", []byte(input))
		if err != nil {
			t.Fatalf("parseSubtitles(%q) error = %v", input, err)
		}
		if len(segments) != 0 {
			t.Errorf("got %d segments, want 0: %+v", len(segments), segments)
		}
	})
}

func TestParseSubtitlesLimits(t *testing.T) {
	t.Run("size at limit", func(t *testing.T) {
		data := padToSize([]byte(`{"events":[]}`), maxSubtitlesBytes)
		segments, err := parseSubtitles("inline.json3", data)
		if err != nil {
			t.Fatalf("parseSubtitles at size limit error = %v", err)
		}
		if len(segments) != 0 {
			t.Errorf("got %d segments, want 0", len(segments))
		}
	})

	t.Run("size over limit", func(t *testing.T) {
		data := padToSize([]byte(`{"events":[]}`), maxSubtitlesBytes+1)
		assertSubtitlesRejected(t, string(data))
	})

	t.Run("events at limit", func(t *testing.T) {
		segments, err := parseSubtitles("inline.json3", eventsDocument(maxSubtitleEvents))
		if err != nil {
			t.Fatalf("parseSubtitles at event limit error = %v", err)
		}
		if len(segments) != 0 {
			t.Errorf("got %d segments, want 0", len(segments))
		}
	})

	t.Run("events over limit", func(t *testing.T) {
		assertSubtitlesRejected(t, string(eventsDocument(maxSubtitleEvents+1)))
	})
}

func TestParseSubtitlesUTF8(t *testing.T) {
	t.Run("invalid UTF-8 sample", func(t *testing.T) {
		data := readTestdataFile(t, testdataInvalidUTF8Subtitles)
		if utf8.Valid(data) {
			t.Fatal("sample is valid UTF-8, so it does not exercise the raw-byte check")
		}
		assertSubtitlesRejected(t, string(data))
	})

	t.Run("unpaired surrogate sample", func(t *testing.T) {
		data := readTestdataFile(t, testdataSurrogateSubtitles)
		if !utf8.Valid(data) {
			t.Fatal("sample is not valid UTF-8, so it would be rejected for the wrong reason")
		}
		if !bytes.Contains(data, []byte(`\ud800`)) {
			t.Fatal("sample does not contain the expected surrogate escape")
		}
		assertSubtitlesRejected(t, string(data))
	})

	t.Run("low surrogate", func(t *testing.T) {
		assertSubtitlesRejected(t, `{"events":[{"tStartMs":0,"segs":[{"utf8":"\udc00"}]}]}`)
	})

	t.Run("valid surrogate pair accepted", func(t *testing.T) {
		segments, err := parseSubtitles("inline.json3", []byte(`{"events":[{"tStartMs":0,"segs":[{"utf8":"\ud83d\ude00"}]}]}`))
		if err != nil {
			t.Fatalf("parseSubtitles error = %v", err)
		}
		want := []Segment{{StartMs: 0, Text: "\U0001F600"}}
		if !reflect.DeepEqual(segments, want) {
			t.Errorf("segments = %+v, want %+v", segments, want)
		}
	})

	t.Run("escaped backslash accepted", func(t *testing.T) {
		segments, err := parseSubtitles("inline.json3", []byte(`{"events":[{"tStartMs":0,"segs":[{"utf8":"\\ud800"}]}]}`))
		if err != nil {
			t.Fatalf("parseSubtitles error = %v", err)
		}
		want := []Segment{{StartMs: 0, Text: `\ud800`}}
		if !reflect.DeepEqual(segments, want) {
			t.Errorf("segments = %+v, want %+v", segments, want)
		}
	})
}

func assertSubtitlesRejected(t *testing.T, input string) {
	t.Helper()
	segments, err := parseSubtitles("inline.json3", []byte(input))
	if !errors.Is(err, ErrParseSubtitles) {
		t.Fatalf("parseSubtitles(%q) error = %v, want ErrParseSubtitles", input, err)
	}
	if errors.Is(err, ErrNoSubtitles) {
		t.Errorf("parseSubtitles(%q) error also matches ErrNoSubtitles", input)
	}
	if segments != nil {
		t.Errorf("parseSubtitles(%q) returned segments %+v", input, segments)
	}
	parseErr, ok := errors.AsType[*ParseError](err)
	if !ok {
		t.Fatalf("parseSubtitles(%q) error is not *ParseError", input)
	}
	if parseErr.Path != "inline.json3" {
		t.Errorf("ParseError.Path = %q, want %q", parseErr.Path, "inline.json3")
	}
}

// subtitlesOracle builds the expected segments from the real fixture using
// encoding/json, independent of the parser under test.
func subtitlesOracle(t *testing.T, data []byte) []Segment {
	t.Helper()
	var document struct {
		Events []struct {
			TStartMs int64 `json:"tStartMs"`
			Segs     []struct {
				UTF8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("oracle decode: %v", err)
	}
	var segments []Segment
	for _, event := range document.Events {
		var text strings.Builder
		for _, seg := range event.Segs {
			text.WriteString(seg.UTF8)
		}
		if strings.TrimSpace(text.String()) == "" {
			continue
		}
		segments = append(segments, Segment{StartMs: event.TStartMs, Text: text.String()})
	}
	return segments
}

// padToSize pads base with trailing whitespace to exactly size bytes.
func padToSize(base []byte, size int) []byte {
	data := make([]byte, size)
	copy(data, base)
	for i := len(base); i < size; i++ {
		data[i] = ' '
	}
	return data
}

// eventsDocument builds a json3 document with events empty events.
func eventsDocument(events int) []byte {
	var builder strings.Builder
	builder.WriteString(`{"events":[`)
	for i := range events {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{}`)
	}
	builder.WriteString(`]}`)
	return []byte(builder.String())
}
