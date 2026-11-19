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
	const commit = "0123456789abcdef0123456789abcdef01234567"
	bases := map[string][]string{
		"prepare":           {"prepare", "--protocol", p, "42"},
		"prepare current":   {"prepare", "--protocol", p},
		"paragraph":         {"paragraph", "--protocol", p, "--state", "s"},
		"paragraph dropped": {"paragraph", "--protocol", p, "--state", "s", "--drop", "1:disputed", "--drop", "2:reverted", "--drop-rejected", commit + ":3"},
		"check-body":        {"check-body", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b"},
		"check-body dropped": {
			"check-body", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b",
			"--drop", "1:disputed", "--drop-rejected", commit + ":3", "--drop-rejected", commit + ":4",
		},
		"merge":         {"merge", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b"},
		"merge dropped": {"merge", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b", "--drop", "1:reverted"},
		"cleanup":       {"cleanup", "--protocol", p, "--state", "s"},
		"cleanup body":  {"cleanup", "--protocol", p, "--state", "s", "--body-file", "b"},
		"discard":       {"discard", "--protocol", p, "--state", "s"},
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
		// A drop of the wrong form, a line named twice, or a drop on a
		// subcommand that takes none.
		extend("paragraph", "--drop", "1"),
		extend("paragraph", "--drop", "x:disputed"),
		extend("paragraph", "--drop", "0:disputed"),
		extend("paragraph", "--drop", "01:disputed"),
		extend("paragraph", "--drop", "+1:disputed"),
		extend("paragraph", "--drop", "1:removed"),
		extend("paragraph", "--drop-rejected", commit),
		extend("paragraph", "--drop-rejected", "abc:3"),
		extend("paragraph", "--drop-rejected", strings.ToUpper(commit)+":3"),
		extend("paragraph", "--drop-rejected", commit+":0"),
		extend("paragraph", "--drop", "1:disputed", "--drop", "1:disputed"),
		extend("paragraph", "--drop", "1:disputed", "--drop", "1:reverted"),
		extend("check-body", "--drop-rejected", commit+":3", "--drop-rejected", commit+":3"),
		extend("prepare current", "--drop", "1:disputed"),
		extend("cleanup", "--drop", "1:disputed"),
		extend("cleanup", "--drop-rejected", commit+":3"),
		extend("discard", "--drop", "1:disputed"),
		// The other shapes.
		extend("paragraph", "43"),
		extend("paragraph", "--subject-file", "j"),
		extend("paragraph", "--body-file", "b"),
		extend("check-body", "43"),
		slices.Clone(bases["check-body"][:len(bases["check-body"])-2]),
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
		slices.Clone(bases["paragraph"][:3]),
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
	for _, command := range mergepr.ShownCommands(42, "/repo/.git/mergepr-1/state.json", "/repo/.git/mergepr-1/body.txt") {
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

// testRejected is a rejected correction line as prepare reports it.
var testRejected = mergepr.RejectedLine{
	Commit: "0123456789abcdef0123456789abcdef01234567",
	Line:   4,
	Kinds:  []mergepr.RejectKind{mergepr.RejectInvisible, mergepr.RejectAt},
	Shown:  `"Editorial correction: aU+200Bb me@example.com"`,
}

// testRejectedLine is the line printed for testRejected.
const testRejectedLine = "correction rejected: 0123456789abcdef0123456789abcdef01234567 line 4: invisible,at: \"Editorial correction: aU+200Bb me@example.com\""

// TestPrintPrepared verifies that prepare prints the count of accepted and
// rejected correction lines, one line for each rejected line with its
// commit, line number, kinds, and shown form, and corrections.txt among the
// files.
func TestPrintPrepared(t *testing.T) {
	prepared := mergepr.Prepared{
		State:         mergepr.State{Number: 42, Title: "T", URL: "u", HeadRefName: "h", HeadRefOID: "o", BaseRefName: "main"},
		Dir:           "/repo/.git/mergepr-1",
		ProtocolCheck: "passed",
		Accepted:      2,
		Rejected:      []mergepr.RejectedLine{testRejected},
	}
	var b strings.Builder
	printPrepared(&b, prepared)
	want := "PR #42: T\nurl:   u\nhead:  h at o\nbase:  main\nprotocol: passed\nCI:    all checks passed\n" +
		"corrections: 2 accepted, 1 rejected\n" +
		testRejectedLine + "\n" +
		"dir:   /repo/.git/mergepr-1 (state.json, log.txt, stat.txt, body.txt, corrections.txt)\n" +
		"state: /repo/.git/mergepr-1/state.json\n"
	if b.String() != want {
		t.Errorf("printPrepared wrote\n%s\nwant\n%s", b.String(), want)
	}
}

// TestPrintReport verifies that once the merge commit is known the report
// says whether the post-merge comparison verified the correction lines, and
// that a report with no merge commit says nothing about them.
func TestPrintReport(t *testing.T) {
	merged := mergepr.Report{ProtocolCheck: "passed", MergeCommitOID: "m", BaseUpdated: true, LocalDeleted: true}
	head := "protocol:             passed\nmerge commit:         m\nbase branch updated:  true\nlocal branch deleted: true\n"
	verified := merged
	verified.CorrectionsChecked, verified.CorrectionsVerified = true, 3
	tests := []struct {
		name   string
		report mergepr.Report
		want   string
	}{
		{"verified", verified, head + "corrections carried:  3 line(s) verified\n"},
		{"not verified", merged, head + "corrections carried:  not verified\n"},
		{"not merged", mergepr.Report{ProtocolCheck: "passed"}, "protocol:             passed\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			printReport(&b, tt.report)
			if b.String() != tt.want {
				t.Errorf("printReport wrote\n%s\nwant\n%s", b.String(), tt.want)
			}
		})
	}
}

// TestPrintErrorListsCorrectionMismatch verifies that a stop of the
// correction check prints one line per reason before the error, a rejected
// line in prepare's form, and that a post-merge difference prints one line
// per line, so each can be read on its own.
func TestPrintErrorListsCorrectionMismatch(t *testing.T) {
	mismatch := &mergepr.CorrectionMismatchError{
		Missing:         []string{`"Editorial correction: a"`},
		Extra:           []string{`"Editorial correction: b"`},
		NotParagraph:    true,
		TrailersChanged: true,
		Undeclared:      []mergepr.RejectedLine{testRejected},
		UnknownDrops:    []string{"--drop 9:disputed"},
	}
	stopped := fmt.Errorf("%w\nnothing was merged", mismatch)
	notCarried := &mergepr.CorrectionsNotCarriedError{MergeCommit: "m", Missing: []string{`"Editorial correction: c"`}, Extra: []string{`"Editorial correction: d"`}}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"mismatch", stopped, "mergepr: correction check: missing: \"Editorial correction: a\"\n" +
			"mergepr: correction check: extra: \"Editorial correction: b\"\n" +
			"mergepr: correction check: the correction lines are not one paragraph of their own\n" +
			"mergepr: correction check: the correction lines change which trailers git reads from the body\n" +
			"mergepr: " + testRejectedLine + "\n" +
			"mergepr: correction check: names no line: --drop 9:disputed\n" +
			"mergepr: " + stopped.Error() + "\n"},
		{"not carried", notCarried, "mergepr: post-merge check: missing: \"Editorial correction: c\"\n" +
			"mergepr: post-merge check: extra: \"Editorial correction: d\"\n" +
			"mergepr: " + notCarried.Error() + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			printError(&b, tt.err)
			if b.String() != tt.want {
				t.Errorf("printError wrote\n%s\nwant\n%s", b.String(), tt.want)
			}
		})
	}
}
