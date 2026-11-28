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

// TestTimingEnabled checks that only TestTimingEnv set to TestTimingValue asks
// for the timing comparison: an unset or empty variable does not, and every
// other value is rejected rather than read as another value.
func TestTimingEnabled(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		set     bool
		want    bool
		wantErr bool
	}{
		{name: "unset"},
		{name: "empty", set: true},
		{name: "one", value: TestTimingValue, set: true, want: true},
		{name: "zero", value: "0", set: true, wantErr: true},
		{name: "true", value: "true", set: true, wantErr: true},
		{name: "leading space", value: " 1", set: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(name string) (string, bool) {
				if name != TestTimingEnv || !tc.set {
					return "", false
				}
				return tc.value, true
			}
			got, err := TimingEnabled(getenv)
			if tc.wantErr {
				if !errors.Is(err, ErrTestTimingValue) {
					t.Fatalf("TimingEnabled(%q) error = %v, want ErrTestTimingValue", tc.value, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Errorf("TimingEnabled(%q) = %v, %v; want %v, nil", tc.value, got, err, tc.want)
			}
		})
	}
}

// TestRunNames checks that a parenthesized list of names is split on '|' and
// that a value of any other shape, an empty name included, is rejected.
func TestRunNames(t *testing.T) {
	got, err := RunNames("^(TestA|TestB)$")
	if err != nil {
		t.Fatalf("RunNames() error = %v", err)
	}
	if want := []string{"TestA", "TestB"}; !slices.Equal(got, want) {
		t.Errorf("RunNames() = %q, want %q", got, want)
	}
	for _, value := range []string{
		"TestA",
		"^TestA$",
		"^(TestA)$x",
		"^()$",
		"^(TestA|)$",
		"^(|TestA)$",
	} {
		if _, err := RunNames(value); !errors.Is(err, ErrRunNames) {
			t.Errorf("RunNames(%q) error = %v, want ErrRunNames", value, err)
		}
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
