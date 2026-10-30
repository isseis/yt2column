//go:build test

package deepseekparam

import (
	"fmt"
	"slices"
	"testing"
)

// TestParseEffort checks the four accepted values round-trip through String and
// that every other string is rejected.
func TestParseEffort(t *testing.T) {
	accepted := []struct {
		value string
		want  Effort
	}{
		{"none", EffortNone},
		{"low", EffortLow},
		{"high", EffortHigh},
		{"max", EffortMax},
	}
	for _, tc := range accepted {
		t.Run("accepts "+tc.value, func(t *testing.T) {
			got, err := ParseEffort(tc.value)
			if err != nil {
				t.Fatalf("ParseEffort(%q) error = %v, want nil", tc.value, err)
			}
			if got != tc.want {
				t.Errorf("ParseEffort(%q) = %v, want %v", tc.value, got, tc.want)
			}
			if round := got.String(); round != tc.value {
				t.Errorf("ParseEffort(%q).String() = %q, want %q", tc.value, round, tc.value)
			}
		})
	}

	rejected := []string{
		"",
		"None",
		" none",
		"none ",
		"none\n",
		"HIGH",
		"medium",
		"xhigh",
		"ultra",
		"minimal",
		"unknown",
	}
	for _, value := range rejected {
		t.Run(fmt.Sprintf("rejects %q", value), func(t *testing.T) {
			got, err := ParseEffort(value)
			if err == nil {
				t.Fatalf("ParseEffort(%q) error = nil, want an error", value)
			}
			if got != EffortUnspecified {
				t.Errorf("ParseEffort(%q) = %v, want EffortUnspecified on error", value, got)
			}
		})
	}
}

// TestEffortValidAndValues checks that the zero value is invalid and renders a
// string ParseEffort rejects, so "no value specified" cannot be mistaken for a
// wire value, and that EffortValues is exactly the wire values of the valid
// values, in the order String renders them: a missing, an extra, a duplicated,
// or a reordered entry fails.
func TestEffortValidAndValues(t *testing.T) {
	if EffortUnspecified.Valid() {
		t.Error("EffortUnspecified.Valid() = true, want false")
	}
	if _, err := ParseEffort(EffortUnspecified.String()); err == nil {
		t.Errorf("ParseEffort(EffortUnspecified.String()=%q) error = nil, want an error", EffortUnspecified.String())
	}

	var want []string
	for e := Effort(-1); e <= EffortMax+2; e++ {
		if e.Valid() {
			want = append(want, e.String())
		}
	}
	if got := EffortValues(); !slices.Equal(got, want) {
		t.Errorf("EffortValues() = %q, want %q", got, want)
	}
}
