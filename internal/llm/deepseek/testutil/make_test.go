//go:build test

package deepseektestutil

import (
	"maps"
	"slices"
	"testing"
)

// TestRecordedNames pins the variables the run helpers record: both opt-ins
// first, so a target that exports the wrong one is visible, then the model
// name, then any extra name the caller adds once each. TestMakeOptInsAre
// TargetSpecific relies on this record to see another target's opt-in.
func TestRecordedNames(t *testing.T) {
	got, err := recordedNames([]string{"EXTRA_A", ModelEnv, "EXTRA_B", "EXTRA_A"})
	if err != nil {
		t.Fatalf("recordedNames() error = %v", err)
	}
	if want := []string{DeepSeekOptInEnv, CLIOptInEnv, ModelEnv, "EXTRA_A", "EXTRA_B"}; !slices.Equal(got, want) {
		t.Errorf("recordedNames() = %q, want %q", got, want)
	}
}

// TestModelEnv pins how the model variable reaches the child: nil leaves it
// undefined, an empty string is passed through, and a value is kept.
func TestModelEnv(t *testing.T) {
	if got := modelEnv(nil); got != nil {
		t.Errorf("modelEnv(nil) = %v, want nil", got)
	}
	empty := ""
	if got := modelEnv(&empty); !maps.Equal(got, map[string]string{ModelEnv: ""}) {
		t.Errorf("modelEnv(&empty) = %v, want %s set to the empty string", got, ModelEnv)
	}
	custom := "custom-model"
	if got := modelEnv(&custom); !maps.Equal(got, map[string]string{ModelEnv: custom}) {
		t.Errorf("modelEnv(&custom) = %v, want %s=%s", got, ModelEnv, custom)
	}
}
