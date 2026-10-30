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

// TestEffortValidAndValues checks that Valid, String, and EffortValues agree
// over the whole domain in both directions: a value Valid accepts renders a
// string the list holds, and every string the list holds parses to a Valid
// value. It also checks that the zero value is invalid and renders a string
// ParseEffort rejects, so "no value specified" cannot be mistaken for a wire
// value.
func TestEffortValidAndValues(t *testing.T) {
	values := EffortValues()
	if EffortUnspecified.Valid() {
		t.Error("EffortUnspecified.Valid() = true, want false")
	}
	if _, err := ParseEffort(EffortUnspecified.String()); err == nil {
		t.Errorf("ParseEffort(EffortUnspecified.String()=%q) error = nil, want an error", EffortUnspecified.String())
	}
	if slices.Contains(values, EffortUnspecified.String()) {
		t.Errorf("EffortValues() contains %q, the placeholder of EffortUnspecified", EffortUnspecified.String())
	}

	for e := Effort(-1); e <= EffortMax+2; e++ {
		if got, want := slices.Contains(values, e.String()), e.Valid(); got != want {
			t.Errorf("Effort(%d): EffortValues() contains %q = %v, Valid() = %v", e, e.String(), got, want)
		}
	}

	for _, value := range values {
		parsed, err := ParseEffort(value)
		if err != nil {
			t.Errorf("EffortValues() holds %q, which ParseEffort rejects: %v", value, err)
			continue
		}
		if !parsed.Valid() {
			t.Errorf("ParseEffort(%q).Valid() = false, want true", value)
		}
		if round := parsed.String(); round != value {
			t.Errorf("ParseEffort(%q).String() = %q, want %q", value, round, value)
		}
	}
}
