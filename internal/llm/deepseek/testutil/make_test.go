//go:build test

package deepseektestutil

import (
	"errors"
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
