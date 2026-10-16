//go:build test

package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"reflect"
	"slices"
	"strconv"
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
		"negative zero":    `{"events":[{"tStartMs":-0,"segs":[{"utf8":"hi"}]}]}`,
		"fractional":       `{"events":[{"tStartMs":1.5,"segs":[{"utf8":"hi"}]}]}`,
		"whole float":      `{"events":[{"tStartMs":1.0,"segs":[{"utf8":"hi"}]}]}`,
		"exponent":         `{"events":[{"tStartMs":1e3,"segs":[{"utf8":"hi"}]}]}`,
		"string":           `{"events":[{"tStartMs":"0","segs":[{"utf8":"hi"}]}]}`,
		"boolean":          `{"events":[{"tStartMs":true,"segs":[{"utf8":"hi"}]}]}`,
		"array":            `{"events":[{"tStartMs":[],"segs":[{"utf8":"hi"}]}]}`,
		"object":           `{"events":[{"tStartMs":{},"segs":[{"utf8":"hi"}]}]}`,
		"overflow":         `{"events":[{"tStartMs":9223372036854775808,"segs":[{"utf8":"hi"}]}]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			assertSubtitlesRejected(t, input)
		})
	}

	t.Run("non-negative integers are valid", func(t *testing.T) {
		cases := map[string]struct {
			input string
			want  int64
		}{
			"zero":      {`{"events":[{"tStartMs":0,"segs":[{"utf8":"hi"}]}]}`, 0},
			"positive":  {`{"events":[{"tStartMs":5000,"segs":[{"utf8":"hi"}]}]}`, 5000},
			"max int64": {`{"events":[{"tStartMs":9223372036854775807,"segs":[{"utf8":"hi"}]}]}`, math.MaxInt64},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				segments, err := parseSubtitles("inline.json3", []byte(tc.input))
				if err != nil {
					t.Fatalf("parseSubtitles error = %v", err)
				}
				want := []Segment{{StartMs: tc.want, Text: "hi"}}
				if !reflect.DeepEqual(segments, want) {
					t.Errorf("segments = %+v, want %+v", segments, want)
				}
			})
		}
	})

	t.Run("discarded event ignores tStartMs", func(t *testing.T) {
		// An event without segs returns before tStartMs is read; an event whose
		// concatenated text is blank is discarded after that. Each branch is
		// checked with both a pre-existing invalid value and the -0 the new
		// reader rejects, so a discarded event never triggers the reader.
		cases := map[string]string{
			"null tStartMs with empty segs": `{"events":[{"tStartMs":null,"segs":[]}]}`,
			"negative zero without segs":    `{"events":[{"tStartMs":-0}]}`,
			"negative zero with blank text": `{"events":[{"tStartMs":-0,"segs":[{"utf8":" "}]}]}`,
		}
		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				segments, err := parseSubtitles("inline.json3", []byte(input))
				if err != nil {
					t.Fatalf("parseSubtitles error = %v", err)
				}
				if len(segments) != 0 {
					t.Errorf("got %d segments, want 0", len(segments))
				}
			})
		}
	})
}

func TestParseSubtitlesTimestampErrorOmitsValue(t *testing.T) {
	// The rejected value is untrusted input, so the error must not echo it. -0
	// is included because the old reader accepted it, which would show up as a
	// nil error rather than as a leak.
	values := []string{"-1", "-0", "9223372036854775808", "1.5", "1e3"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			input := `{"events":[{"tStartMs":` + value + `,"segs":[{"utf8":"hi"}]}]}`
			_, err := parseSubtitles("inline.json3", []byte(input))
			if err == nil {
				t.Fatalf("parseSubtitles(%s) error = nil, want an error", value)
			}
			if strings.Contains(err.Error(), value) {
				t.Errorf("parseSubtitles(%s) error = %q, must not contain the value", value, err.Error())
			}
		})
	}
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

func TestParseSubtitlesArrayElementLimit(t *testing.T) {
	// Only the segs case can discriminate internal/strictjson's AsArray cap
	// (maxArrayElements, 131072): an events array above the cap is already
	// rejected by maxSubtitleEvents (65,536). Both still surface as
	// ErrParseSubtitles.
	const overCap = 131073

	t.Run("segs over the strictjson cap", func(t *testing.T) {
		assertSubtitlesRejected(t, string(segsDocument(overCap)))
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

// segsDocument builds a json3 document with one event carrying segs empty
// segments.
func segsDocument(segs int) []byte {
	var builder strings.Builder
	builder.WriteString(`{"events":[{"tStartMs":0,"segs":[`)
	for i := range segs {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{}`)
	}
	builder.WriteString(`]}]}`)
	return []byte(builder.String())
}

// TestTimestampReadingIsUnified fixes the single non-negative integer rule for
// tStartMs: json3.go must read it with strictjson.Value.AsNonNegativeInt64 and
// must not reintroduce a second rule, whether by calling AsInt64/decodeTimestamp
// or by comparing a value derived from that read against zero or a negative
// constant. The guard parses the source, so an equivalent spelling of the same
// check is caught, while an unrelated comparison such as len(x) > 0 is not.
func TestTimestampReadingIsUnified(t *testing.T) {
	file, fset := parsePackageFile(t, "json3.go")
	readNames := readValueNames(file)

	var (
		readsUnified bool
		stale        []string
		signChecks   []string
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if v.Sel.Name == "AsNonNegativeInt64" {
				readsUnified = true
			}
		case *ast.Ident:
			switch v.Name {
			case "AsInt64", "decodeTimestamp", "errNegativeTimestamp":
				stale = append(stale, v.Name)
			}
		case *ast.BinaryExpr:
			if signCheckOn(v, readNames) {
				signChecks = append(signChecks, fset.Position(v.Pos()).String())
			}
		}
		return true
	})

	if !readsUnified {
		t.Error("json3.go does not read tStartMs with AsNonNegativeInt64")
	}
	if len(stale) > 0 {
		t.Errorf("json3.go still carries the old non-negative rule via %v", stale)
	}
	if len(signChecks) > 0 {
		t.Errorf("json3.go compares a tStartMs-derived value against zero or a negative constant at %v", signChecks)
	}
}

// TestExportedSentinelErrorsUnchanged fixes the exported sentinel errors of the
// package. A new sentinel changes the error contract that callers branch on
// with errors.Is, so adding one must revisit the callers and this guard.
func TestExportedSentinelErrorsUnchanged(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	want := []string{
		"ErrCopiedTranscriptFormat",
		"ErrInvalidVideoURL",
		"ErrNoSubtitles",
		"ErrParseInfo",
		"ErrParseSubtitles",
		"ErrYtDlpExec",
	}

	var got []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, _ := parsePackageFile(t, name)
		got = append(got, exportedErrNames(file)...)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("exported sentinel errors = %v, want %v; a new sentinel changes the error contract callers branch on, so revisit the callers and this guard", got, want)
	}
}

// exportedErrNames returns the package-level var and const identifiers of the
// file that are exported and start with "Err".
func exportedErrNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.VAR && gen.Tok != token.CONST) {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if ast.IsExported(name.Name) && strings.HasPrefix(name.Name, "Err") {
					names = append(names, name.Name)
				}
			}
		}
	}
	return names
}

// parsePackageFile parses a source file of the package under test. The test
// working directory is the package directory.
func parsePackageFile(t *testing.T, name string) (*ast.File, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return file, fset
}

// readValueNames returns the identifiers that hold a value produced by
// AsNonNegativeInt64: the identifier that receives the call's integer result,
// and any identifier later assigned from one of them. The call's trailing error
// result is not a read value and is not tracked.
func readValueNames(file *ast.File) map[string]bool {
	var assigns []*ast.AssignStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			assigns = append(assigns, assign)
		}
		return true
	})

	names := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, assign := range assigns {
			for i, rhs := range assign.Rhs {
				if callsAsNonNegativeInt64(rhs) && i < len(assign.Lhs) {
					if markName(assign.Lhs[i], names) {
						changed = true
					}
					continue
				}
				if referencesAny(rhs, names) {
					for _, lhs := range assign.Lhs {
						if markName(lhs, names) {
							changed = true
						}
					}
				}
			}
		}
	}
	return names
}

// markName records id in names and reports whether it was newly added.
func markName(expr ast.Expr, names map[string]bool) bool {
	id, ok := expr.(*ast.Ident)
	if !ok || id.Name == "_" || names[id.Name] {
		return false
	}
	names[id.Name] = true
	return true
}

// callsAsNonNegativeInt64 reports whether expr is a call to AsNonNegativeInt64.
func callsAsNonNegativeInt64(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "AsNonNegativeInt64"
}

// referencesAny reports whether expr mentions one of names.
func referencesAny(expr ast.Expr, names map[string]bool) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return true
	})
	return found
}

// signCheckOn reports whether expr tests the zero boundary of one of the read
// values, the shape of a hand-written non-negative check (v < 0, v <= -1, 0 > v,
// v >= 0). A comparison on an unrelated value (len(x) > 0) or a different rule
// (v > 0, a strictly positive check) is not flagged.
func signCheckOn(expr *ast.BinaryExpr, readNames map[string]bool) bool {
	if !isSignComparison(expr) {
		return false
	}
	return referencesAny(expr.X, readNames) || referencesAny(expr.Y, readNames)
}

// isSignComparison reports whether expr tests the boundary zero of the value it
// compares: it rejects a negative value (v < 0, v <= -1) or accepts a
// non-negative one (v >= 0, v > -1). A strictly positive check (v > 0) is a
// different rule and is not matched.
func isSignComparison(expr *ast.BinaryExpr) bool {
	if value, ok := constantInt(expr.X); ok {
		return nonNegativeBoundary(expr.Op, value, false)
	}
	if value, ok := constantInt(expr.Y); ok {
		return nonNegativeBoundary(expr.Op, value, true)
	}
	return false
}

// nonNegativeBoundary reports whether a comparison with integer constant c tests
// the boundary zero with the non-negative side inclusive. constantOnRight tells
// whether the comparison reads value op c (true) or c op value (false).
func nonNegativeBoundary(op token.Token, c int64, constantOnRight bool) bool {
	if !constantOnRight {
		op = reverseComparison(op)
	}
	switch op {
	case token.LSS: // value < c
		return c == 0
	case token.LEQ: // value <= c
		return c == -1
	case token.GTR: // value > c
		return c == -1
	case token.GEQ: // value >= c
		return c == 0
	default:
		return false
	}
}

// reverseComparison returns the operator that means the same with its operands
// swapped.
func reverseComparison(op token.Token) token.Token {
	switch op {
	case token.LSS:
		return token.GTR
	case token.LEQ:
		return token.GEQ
	case token.GTR:
		return token.LSS
	case token.GEQ:
		return token.LEQ
	default:
		return op
	}
}

// constantInt returns the value of an integer literal, including a negated,
// parenthesized, or explicitly positive one.
func constantInt(expr ast.Expr) (int64, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.INT {
			return 0, false
		}
		value, err := strconv.ParseInt(v.Value, 0, 64)
		if err != nil {
			return 0, false
		}
		return value, true
	case *ast.ParenExpr:
		return constantInt(v.X)
	case *ast.UnaryExpr:
		switch v.Op {
		case token.SUB:
			value, ok := constantInt(v.X)
			if !ok {
				return 0, false
			}
			return -value, true
		case token.ADD:
			return constantInt(v.X)
		}
	}
	return 0, false
}

// TestSignCheckOn pins the guard's detection logic: a sign check on the read
// value is flagged, an unrelated comparison is not.
func TestSignCheckOn(t *testing.T) {
	readNames := map[string]bool{"startMs": true}
	cases := map[string]struct {
		expr string
		want bool
	}{
		"less than zero":          {"startMs < 0", true},
		"less or equal minus one": {"startMs <= -1", true},
		"zero greater than value": {"0 > startMs", true},
		"non-negative accepted":   {"startMs >= 0", true},
		"greater than minus one":  {"startMs > -1", true},
		"parenthesized zero":      {"startMs < (0)", true},
		"unary plus zero":         {"startMs < +0", true},
		"strictly positive":       {"startMs > 0", false},
		"unrelated length":        {"len(segs) > 0", false},
		"unrelated variable":      {"count > 0", false},
		"equality is not a sign":  {"startMs == 0", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tc.expr)
			if err != nil {
				t.Fatalf("ParseExpr(%q): %v", tc.expr, err)
			}
			comparison, ok := expr.(*ast.BinaryExpr)
			if !ok {
				t.Fatalf("%q is not a comparison", tc.expr)
			}
			if got := signCheckOn(comparison, readNames); got != tc.want {
				t.Errorf("signCheckOn(%q) = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}
