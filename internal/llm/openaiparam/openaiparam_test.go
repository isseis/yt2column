//go:build test

package openaiparam

import (
	"fmt"
	"slices"
	"testing"
)

// TestParseEffort checks the seven accepted values round-trip through String
// and that every other string is rejected.
func TestParseEffort(t *testing.T) {
	accepted := []struct {
		value string
		want  Effort
	}{
		{"none", EffortNone},
		{"minimal", EffortMinimal},
		{"low", EffortLow},
		{"medium", EffortMedium},
		{"high", EffortHigh},
		{"xhigh", EffortXHigh},
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
		"High",
		" high",
		"high\n",
		"min",
		"medium-high",
		"LOW",
		unknownEffort,
	}
	for _, value := range rejected {
		t.Run(fmt.Sprintf("rejects %q", value), func(t *testing.T) {
			got, err := ParseEffort(value)
			if err == nil {
				t.Fatalf("ParseEffort(%q) error = nil, want an error", value)
			}
			if got != EffortUnset {
				t.Errorf("ParseEffort(%q) = %v, want EffortUnset on error", value, got)
			}
		})
	}
}

// TestEffortStringAndValid checks that the accepted values are Valid and that
// EffortUnset and out-of-range values are not, that String renders those
// invalid values as a string ParseEffort rejects, and that EffortValues holds
// exactly the wire values of the valid values in the documented order: a
// missing, an extra, a duplicated, or a misplaced entry fails.
func TestEffortStringAndValid(t *testing.T) {
	wantOrder := []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
	if got := EffortValues(); !slices.Equal(got, wantOrder) {
		t.Errorf("EffortValues() = %q, want %q", got, wantOrder)
	}

	for _, e := range []Effort{
		EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax,
	} {
		t.Run("valid "+e.String(), func(t *testing.T) {
			if !e.Valid() {
				t.Errorf("Effort(%d).Valid() = false, want true", e)
			}
		})
	}

	invalid := []Effort{EffortUnset, -1, EffortMax + 1, 1000}
	for _, e := range invalid {
		t.Run(fmt.Sprintf("invalid %d", e), func(t *testing.T) {
			if e.Valid() {
				t.Errorf("Effort(%d).Valid() = true, want false", e)
			}
			if _, err := ParseEffort(e.String()); err == nil {
				t.Errorf("ParseEffort(Effort(%d).String()=%q) error = nil, want an error", e, e.String())
			}
		})
	}

	// Valid and ParseEffort must agree over the whole domain, not only at the
	// values listed above: a value Valid accepts must be one String renders as
	// a wire value ParseEffort accepts, and the reverse. The same walk builds
	// the list EffortValues must hold.
	t.Run("agrees with ParseEffort and EffortValues", func(t *testing.T) {
		var want []string
		for e := Effort(-2); e <= EffortMax+3; e++ {
			_, err := ParseEffort(e.String())
			if gotValid, wantValid := e.Valid(), err == nil; gotValid != wantValid {
				t.Errorf("Effort(%d): Valid() = %v, ParseEffort(String()=%q) error = %v", e, gotValid, e.String(), err)
			}
			if e.Valid() {
				want = append(want, e.String())
			}
		}
		slices.Sort(want)
		if got := slices.Sorted(slices.Values(EffortValues())); !slices.Equal(got, want) {
			t.Errorf("EffortValues() = %q, want %q", EffortValues(), want)
		}
	})
}

// TestEffortNoneIsNotUnset checks that "none" is a valid value distinct from
// the zero value, so a caller can tell "reasoning off" from "no value
// specified".
func TestEffortNoneIsNotUnset(t *testing.T) {
	if EffortNone == EffortUnset {
		t.Fatal("EffortNone equals EffortUnset")
	}
	if !EffortNone.Valid() {
		t.Error("EffortNone.Valid() = false, want true")
	}
	if EffortUnset.Valid() {
		t.Error("EffortUnset.Valid() = true, want false")
	}
}
