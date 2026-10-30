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
		"medium",
		"xhigh",
		"ultra",
		"minimal",
		unknownEffort,
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

// TestEffortValidAndValues checks that the zero Effort is EffortUnspecified,
// which Valid rejects and whose String ParseEffort rejects, so "no value
// specified" cannot be mistaken for a wire value, and that EffortValues holds
// exactly the wire values of the valid values: a missing, an extra, or a
// duplicated entry fails.
func TestEffortValidAndValues(t *testing.T) {
	var zero Effort
	if zero != EffortUnspecified {
		t.Fatalf("the zero Effort = %d, want EffortUnspecified", zero)
	}
	if zero.Valid() {
		t.Error("the zero Effort.Valid() = true, want false")
	}
	if _, err := ParseEffort(zero.String()); err == nil {
		t.Errorf("ParseEffort(zero.String()=%q) error = nil, want an error", zero.String())
	}

	var want []string
	for e := Effort(-1); e <= EffortMax+2; e++ {
		if e.Valid() {
			want = append(want, e.String())
		}
	}
	slices.Sort(want)
	if got := slices.Sorted(slices.Values(EffortValues())); !slices.Equal(got, want) {
		t.Errorf("EffortValues() = %q, want %q", EffortValues(), want)
	}
}
