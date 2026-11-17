//go:build test

package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/mergepr"
)

func TestRunRejectsExtraArguments(t *testing.T) {
	tests := [][]string{
		{"prepare", "--protocol", "1", "42", "43"},
		{"merge", "--protocol", "1", "--state", "s", "--subject-file", "s", "--body-file", "b", "43"},
		{"cleanup", "--protocol", "1", "--state", "s", "43"},
		{"discard", "--protocol", "1", "--state", "s", "43"},
		// A flag the subcommand does not take.
		{"prepare", "--protocol", "1", "--state", "s"},
		{"prepare", "--protocol", "1", "--subject-file", "s"},
		{"prepare", "--protocol", "1", "--body-file", "b"},
		{"cleanup", "--protocol", "1", "--state", "s", "--subject-file", "s"},
		{"discard", "--protocol", "1", "--state", "s", "--body-file", "b"},
		// A required flag missing.
		{"merge", "--protocol", "1", "--state", "s", "--subject-file", "s"},
		{"cleanup", "--protocol", "1"},
		// An unknown subcommand, even with the right --protocol.
		{"status", "--protocol", "1"},
	}
	for _, args := range tests {
		if err := run(args); !errors.Is(err, errUsage) {
			t.Errorf("run(%q) error = %v, want errUsage", args, err)
		}
	}
}

// TestParseRequiresProtocol verifies that every subcommand accepts a command
// line that is right but for --protocol only when --protocol is this
// binary's number written as plain decimal text, and refuses it otherwise.
func TestParseRequiresProtocol(t *testing.T) {
	p := strconv.Itoa(mergepr.Protocol)
	bases := [][]string{
		{"prepare", "--protocol", p, "42"},
		{"prepare", "--protocol", p},
		{"merge", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b"},
		{"cleanup", "--protocol", p, "--state", "s"},
		{"discard", "--protocol", p, "--state", "s"},
	}
	// The same number in other notations, other numbers, and no number.
	wrong := []string{"0" + p, "+" + p, fmt.Sprintf("0x%x", mergepr.Protocol), p + "junk", " " + p, strconv.Itoa(mergepr.Protocol + 1), "0", ""}
	for _, base := range bases {
		if _, err := parse(base); err != nil {
			t.Errorf("parse(%q) error = %v, want nil", base, err)
		}
		for _, value := range wrong {
			args := slices.Clone(base)
			args[2] = value
			if _, err := parse(args); !errors.Is(err, errUsage) {
				t.Errorf("parse(%q) error = %v, want errUsage", args, err)
			}
		}
		without := slices.Delete(slices.Clone(base), 1, 3)
		if _, err := parse(without); !errors.Is(err, errUsage) {
			t.Errorf("parse(%q) error = %v, want errUsage", without, err)
		}
	}
}

// TestShownCommandsParse verifies that every mergepr command an error message
// can show passes this binary's argument check, so running it as shown works.
func TestShownCommandsParse(t *testing.T) {
	for _, command := range mergepr.ShownCommands(42, "/repo/.git/mergepr-1/state.json") {
		fields := strings.Fields(command)
		if len(fields) < 2 || fields[0] != "mergepr" {
			t.Errorf("shown command %q does not start with mergepr", command)
			continue
		}
		if _, err := parse(fields[1:]); err != nil {
			t.Errorf("parse(%q) error = %v, want nil", fields[1:], err)
		}
	}
}
