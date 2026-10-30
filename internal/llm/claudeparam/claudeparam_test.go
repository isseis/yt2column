//go:build test

package claudeparam

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// TestParseEffort checks the five accepted values round-trip through String
// and that every other string is rejected.
func TestParseEffort(t *testing.T) {
	accepted := []struct {
		value string
		want  Effort
	}{
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
		"none",
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
// exactly the wire values of the valid values: a missing, an extra, or a
// duplicated entry fails.
func TestEffortStringAndValid(t *testing.T) {
	for _, e := range []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax} {
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

// TestParseWorkspaceID checks the printable ASCII boundaries are accepted and
// that empty, spaced, control, DEL, and non-ASCII values are rejected.
func TestParseWorkspaceID(t *testing.T) {
	accepted := []string{
		"!",
		"~",
		"!~",
		"wrkspc_x",
		"wrkspc-123",
	}
	for _, value := range accepted {
		t.Run(fmt.Sprintf("accepts %q", value), func(t *testing.T) {
			w, err := ParseWorkspaceID(value)
			if err != nil {
				t.Fatalf("ParseWorkspaceID(%q) error = %v, want nil", value, err)
			}
			got, ok := w.Value()
			if !ok {
				t.Fatalf("ParseWorkspaceID(%q).Value() ok = false, want true", value)
			}
			if got != value {
				t.Errorf("ParseWorkspaceID(%q).Value() = %q, want %q", value, got, value)
			}
		})
	}

	rejected := []string{
		"",
		" wrkspc_x",
		"wrkspc_x\n",
		"wrkspc \u00e9",
		"wrkspc_x\x7f",
		"wrkspc_x\t",
		"\x00wrkspc",
		"\u00e9",
		"wrk spc",
	}
	for _, value := range rejected {
		t.Run(fmt.Sprintf("rejects %q", value), func(t *testing.T) {
			w, err := ParseWorkspaceID(value)
			if err == nil {
				t.Fatalf("ParseWorkspaceID(%q) error = nil, want an error", value)
			}
			if _, ok := w.Value(); ok {
				t.Errorf("ParseWorkspaceID(%q).Value() ok = true, want false on error", value)
			}
		})
	}
}

// TestWorkspaceIDZeroValue checks the zero value means no workspace was
// specified.
func TestWorkspaceIDZeroValue(t *testing.T) {
	var w WorkspaceID
	got, ok := w.Value()
	if ok {
		t.Errorf("zero WorkspaceID.Value() ok = true, want false")
	}
	if got != "" {
		t.Errorf("zero WorkspaceID.Value() = %q, want %q", got, "")
	}
}

// TestWorkspaceIDHasNoExportedFields checks that a workspace ID cannot be
// built outside this package except through ParseWorkspaceID, so an empty or
// non-printable value is not representable.
func TestWorkspaceIDHasNoExportedFields(t *testing.T) {
	for field := range reflect.TypeFor[WorkspaceID]().Fields() {
		if field.PkgPath == "" {
			t.Errorf("WorkspaceID has an exported field %q; an invalid value would be settable outside ParseWorkspaceID", field.Name)
		}
	}
}
