//go:build test

package main

import (
	"errors"
	"testing"
)

func TestRunRejectsExtraArguments(t *testing.T) {
	tests := [][]string{
		{"merge", "--state", "s", "--subject-file", "s", "--body-file", "b", "43"},
		{"cleanup", "--state", "s", "43"},
		{"discard", "--state", "s", "43"},
	}
	for _, args := range tests {
		if err := run(args); !errors.Is(err, errUsage) {
			t.Errorf("run(%q) error = %v, want errUsage", args, err)
		}
	}
}
