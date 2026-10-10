//go:build test

package deepseektestutil

import (
	"maps"
	"slices"
	"testing"
)

// TestRecordedEnv pins the variables the run helpers record: both opt-ins
// first, so a target that exports the wrong one is visible, then the model
// name. TestMakeOptInsAreTargetSpecific relies on this record to see another
// target's opt-in.
func TestRecordedEnv(t *testing.T) {
	if want := []string{DeepSeekOptInEnv, CLIOptInEnv, ModelEnv}; !slices.Equal(recordedEnv, want) {
		t.Errorf("recordedEnv = %q, want %q", recordedEnv, want)
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
