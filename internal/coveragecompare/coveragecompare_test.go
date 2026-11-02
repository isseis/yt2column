//go:build test

package coveragecompare

import (
	"errors"
	"math"
	"strconv"
	"testing"
)

func TestParseNonNegativeInt(t *testing.T) {
	maxIntText := strconv.FormatInt(int64(math.MaxInt), 10)
	tooLarge := strconv.FormatUint(uint64(math.MaxInt)+1, 10)

	accepted := map[string]int{
		"0":        0,
		"42":       42,
		maxIntText: math.MaxInt,
	}
	for text, want := range accepted {
		t.Run("accepts "+text, func(t *testing.T) {
			got, ok := parseNonNegativeInt(text)
			if !ok || got != want {
				t.Errorf("parseNonNegativeInt(%q) = %d, %v; want %d, true", text, got, ok, want)
			}
		})
	}

	rejected := []string{"", "-1", "+1", "1.5", "1_000", "abc", tooLarge}
	for _, text := range rejected {
		t.Run("rejects "+text, func(t *testing.T) {
			if _, ok := parseNonNegativeInt(text); ok {
				t.Errorf("parseNonNegativeInt(%q) accepted the value", text)
			}
		})
	}
}

func TestStopError(t *testing.T) {
	inner := errors.New("inner failure")
	withCause := &StopError{Kind: StopUnreadable, Reasons: []StopReason{{Detail: "detail"}}, Err: inner}
	if got := withCause.Error(); got != "inner failure" {
		t.Errorf("Error = %q, want the cause", got)
	}
	if !errors.Is(withCause, inner) {
		t.Error("Unwrap does not expose the cause")
	}
	withoutCause := &StopError{Kind: StopUsage, Reasons: []StopReason{{Detail: "detail"}}}
	if got := withoutCause.Error(); got == "" {
		t.Error("Error is empty for a reason without a cause")
	}
	if got := (&StopError{}).Error(); got == "" {
		t.Error("Error is empty for the zero value")
	}
	var nilStop *StopError
	if got := nilStop.Error(); got == "" {
		t.Error("Error is empty for a nil receiver")
	}
	if nilStop.Unwrap() != nil {
		t.Error("nil Unwrap is not nil")
	}
}

func TestParsersReportLineAndExcerpt(t *testing.T) {
	tests := []struct {
		name    string
		parse   func() error
		input   Input
		line    int
		excerpt string
	}{
		{
			name: "profile",
			parse: func() error {
				_, err := parseGoProfile([]byte("mode: set\nbad\n"), testModulePath, PointBase)
				return err
			},
			input:   InputGoProfile,
			line:    2,
			excerpt: "bad",
		},
		{
			name:    "go test -json",
			parse:   func() error { _, err := parseGoTestJSON([]byte("bad\n"), PointBase); return err },
			input:   InputGoTestJSON,
			line:    1,
			excerpt: "bad",
		},
		{
			name:    "lcov",
			parse:   func() error { _, err := parseLCOV([]byte("SF:x.ts\nXX:1\n"), PointBase); return err },
			input:   InputLCOV,
			line:    2,
			excerpt: "XX:1",
		},
		{
			name: "tap",
			parse: func() error {
				_, err := parseTAP([]byte("# tests 1\n# fail 0\n# cancelled 0\n# tests 1\n"), testExtensionDir, PointBase)
				return err
			},
			input:   InputTAP,
			line:    4,
			excerpt: "# tests 1",
		},
		{
			name:    "diff",
			parse:   func() error { _, err := parseDiff([]byte("diff --git a/x.go b/x.go\nbad\n"), PointBase); return err },
			input:   InputDiff,
			line:    2,
			excerpt: "bad",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reason := requireStop(t, test.parse(), PointBase, test.input)
			if reason.Line != test.line || reason.Excerpt != test.excerpt {
				t.Errorf("reason = line %d excerpt %q, want line %d excerpt %q", reason.Line, reason.Excerpt, test.line, test.excerpt)
			}
		})
	}
}
