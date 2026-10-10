//go:build test

package maketestutil

import (
	"errors"
	"slices"
	"testing"
)

func TestValidateEnvNames(t *testing.T) {
	if err := validateEnvNames([]string{"A", "_x", "YT2COLUMN_MODEL", "a1_B2"}); err != nil {
		t.Errorf("plain names: error = %v, want nil", err)
	}
	for _, name := range []string{"", "1A", "A;rm", "$X", "A B", "A-B", "A\nB"} {
		if err := validateEnvNames([]string{"OK", name}); !errors.Is(err, errEnvName) {
			t.Errorf("validateEnvNames(%q) error = %v, want errEnvName", name, err)
		}
	}
}

// TestValidateChargedTarget checks that a target naming at least one variable
// passes and one naming none fails, so CheckChargedTarget cannot pass by
// running no subtest.
func TestValidateChargedTarget(t *testing.T) {
	if err := validateChargedTarget(ChargedTarget{Vars: []TargetVar{{Env: "A"}}}); err != nil {
		t.Errorf("with one variable: error = %v, want nil", err)
	}
	if err := validateChargedTarget(ChargedTarget{}); !errors.Is(err, errNoVars) {
		t.Errorf("with no variable: error = %v, want errNoVars", err)
	}
}

// TestRecordedNames checks that extra names follow base once each, with no
// duplicate, and pass the same name check as the base ones, since both are
// embedded in the stub script.
func TestRecordedNames(t *testing.T) {
	got, err := RecordedNames([]string{"BASE"}, []string{"EXTRA_A", "BASE", "EXTRA_B", "EXTRA_A"})
	if err != nil {
		t.Fatalf("RecordedNames() error = %v", err)
	}
	if want := []string{"BASE", "EXTRA_A", "EXTRA_B"}; !slices.Equal(got, want) {
		t.Errorf("RecordedNames() = %q, want %q", got, want)
	}
	if _, err := RecordedNames([]string{"BASE"}, []string{"A;rm -rf x"}); !errors.Is(err, errEnvName) {
		t.Errorf("RecordedNames with a shell metacharacter: error = %v, want errEnvName", err)
	}
}
