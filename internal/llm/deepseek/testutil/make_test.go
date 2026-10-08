//go:build test

package deepseektestutil

import (
	"errors"
	"os"
	"path/filepath"
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

func TestRecordedNames(t *testing.T) {
	t.Run("extra_names_appended_once", func(t *testing.T) {
		got, err := recordedNames([]string{"EXTRA_A", ModelEnv, "EXTRA_B", "EXTRA_A"})
		if err != nil {
			t.Fatalf("recordedNames() error = %v", err)
		}
		want := slices.Concat(recordedEnv, []string{"EXTRA_A", "EXTRA_B"})
		if !slices.Equal(got, want) {
			t.Errorf("recordedNames() = %q, want %q", got, want)
		}
	})
	t.Run("no_extra_names", func(t *testing.T) {
		got, err := recordedNames(nil)
		if err != nil || !slices.Equal(got, recordedEnv) {
			t.Errorf("recordedNames(nil) = %q, %v; want %q, nil", got, err, recordedEnv)
		}
	})
	// An extra name is embedded in the stub script like the fixed ones, so it
	// must be rejected the same way.
	for _, name := range []string{"A;rm -rf x", "$X", "A-B"} {
		if _, err := recordedNames([]string{name}); !errors.Is(err, errEnvName) {
			t.Errorf("recordedNames(%q) error = %v, want errEnvName", name, err)
		}
	}
}

// TestRunMakeTargetRecordsExtraNames runs a Makefile whose target exports a
// variable outside recordedEnv, and checks that the stub records it only when
// the caller names it.
func TestRunMakeTargetRecordsExtraNames(t *testing.T) {
	const extra = "YT2COLUMN_EXAMPLE_OPT_IN"
	root := t.TempDir()
	makefile := "example: export " + extra + " := 1\n\nexample:\n\t$(GOTEST) -v ./example\n"
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(makefile), 0o600); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}

	_, invocation := RunMakeTarget(t, root, "example", nil, extra)
	if got, ok := invocation.Env[extra]; !ok || got != OptInValue {
		t.Errorf("%s = %q (set %t), want %q", extra, got, ok, OptInValue)
	}
	if want := []string{"-v", "./example"}; !slices.Equal(invocation.Args, want) {
		t.Errorf("GOTEST arguments = %q, want %q", invocation.Args, want)
	}

	_, invocation = RunMakeTarget(t, root, "example", nil)
	if got, ok := invocation.Env[extra]; ok {
		t.Errorf("%s = %q was recorded without being named", extra, got)
	}
}
