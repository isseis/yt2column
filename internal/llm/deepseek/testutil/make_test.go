//go:build test

package deepseektestutil

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

// TestRecordedNames checks that extra names follow recordedEnv once each and
// pass the same name check as the fixed ones, since both are embedded in the
// stub script.
func TestRecordedNames(t *testing.T) {
	got, err := recordedNames([]string{"EXTRA_A", ModelEnv, "EXTRA_B", "EXTRA_A"})
	if err != nil {
		t.Fatalf("recordedNames() error = %v", err)
	}
	if want := slices.Concat(recordedEnv, []string{"EXTRA_A", "EXTRA_B"}); !slices.Equal(got, want) {
		t.Errorf("recordedNames() = %q, want %q", got, want)
	}
	if _, err := recordedNames([]string{"A;rm -rf x"}); !errors.Is(err, errEnvName) {
		t.Errorf("recordedNames with a shell metacharacter: error = %v, want errEnvName", err)
	}
}
