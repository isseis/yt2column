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
	"github.com/isseis/yt2column/internal/numreserve"
	"github.com/isseis/yt2column/internal/tasknaming"
)

// TestParseRequiresProtocol verifies, for every subcommand, that a right
// command line is accepted, and that parse refuses it with errUsage when only
// --protocol is changed (another notation of the same number, another number,
// no number) or when only its shape is changed (an extra argument, a flag the
// subcommand does not take, a missing flag, an unknown subcommand). Every
// refused line differs from an accepted one, so each is refused by the check
// it targets. parse reads and runs nothing, so no row can reach gh or git.
func TestParseRequiresProtocol(t *testing.T) {
	p := strconv.Itoa(mergepr.Protocol)
	bases := map[string][]string{
		"prepare":         {"prepare", "--protocol", p, "42"},
		"prepare current": {"prepare", "--protocol", p},
		"merge":           {"merge", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b"},
		"cleanup":         {"cleanup", "--protocol", p, "--state", "s"},
		"discard":         {"discard", "--protocol", p, "--state", "s"},
	}
	wrongProtocol := []string{"0" + p, "+" + p, fmt.Sprintf("0x%x", mergepr.Protocol), p + "junk", " " + p, strconv.Itoa(mergepr.Protocol + 1), "0", ""}
	for name, base := range bases {
		if _, err := parse(base); err != nil {
			t.Errorf("%s: parse(%q) error = %v, want nil", name, base, err)
		}
		for _, value := range wrongProtocol {
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
	extend := func(name string, extra ...string) []string { return append(slices.Clone(bases[name]), extra...) }
	shapes := [][]string{
		extend("prepare", "43"),
		extend("merge", "43"),
		extend("cleanup", "43"),
		extend("discard", "43"),
		extend("prepare current", "--state", "s"),
		extend("prepare current", "--subject-file", "s"),
		extend("prepare current", "--body-file", "b"),
		extend("cleanup", "--subject-file", "s"),
		extend("discard", "--body-file", "b"),
		slices.Clone(bases["merge"][:len(bases["merge"])-2]),
		slices.Clone(bases["cleanup"][:3]),
		append([]string{"status"}, bases["cleanup"][1:]...),
	}
	for _, args := range shapes {
		if _, err := parse(args); !errors.Is(err, errUsage) {
			t.Errorf("parse(%q) error = %v, want errUsage", args, err)
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

// TestPrintErrorListsNumberIssues verifies that a stop of the number check
// prints each finding on a line of its own, before the error, and that any
// other error prints one line.
func TestPrintErrorListsNumberIssues(t *testing.T) {
	task, err := tasknaming.ParseNumber("0104")
	if err != nil {
		t.Fatal(err)
	}
	issues := []numreserve.AddedNumberIssue{
		{Space: numreserve.SpaceTask, Number: task, Kind: numreserve.AddedUnreserved, Path: "docs/tasks/0104_a"},
		{Space: numreserve.SpaceTask, Number: task, Kind: numreserve.AddedOnOtherBranch, Path: "docs/tasks/0104_a", UsedBy: "docs/tasks/0104_b", Branch: "owner/0104-b-01"},
	}
	stopped := fmt.Errorf("%w\nnothing was merged", &mergepr.NumberIssuesError{Issues: issues})
	var b strings.Builder
	printError(&b, stopped)
	want := "mergepr: number check: " + issues[0].Line() + "\n" +
		"mergepr: number check: " + issues[1].Line() + "\n" +
		"mergepr: " + stopped.Error() + "\n"
	if b.String() != want {
		t.Errorf("printError wrote\n%s\nwant\n%s", b.String(), want)
	}

	b.Reset()
	printError(&b, errUsage)
	if got, want := b.String(), "mergepr: "+errUsage.Error()+"\n"; got != want {
		t.Errorf("printError wrote %q, want %q", got, want)
	}
}
