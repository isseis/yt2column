//go:build test

package main

import (
	"context"
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
	const other = "4444444444444444444444444444444444444444"
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
		"merge":           {"merge", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b"},
		"merge dropped":   {"merge", "--protocol", p, "--state", "s", "--subject-file", "j", "--body-file", "b", "--drop", "1:reverted"},
		"cleanup":         {"cleanup", "--protocol", p, "--state", "s"},
		"cleanup body":    {"cleanup", "--protocol", p, "--state", "s", "--body-file", "b"},
		"discard":         {"discard", "--protocol", p, "--state", "s"},
		"mergemain-begin": {"mergemain-begin", "--protocol", p, "42"},
		"mergemain-merge": {
			"mergemain-merge", "--protocol", p, "--head-branch", "feature/foo", "--base-head", commit, "--main", other, "42",
		},
		"mergemain-restore": {
			"mergemain-restore", "--protocol", p, "--head-branch", "feature/foo", "--base-head", commit, "--main", other, "42",
		},
		"mergemain-push": {"mergemain-push", "--protocol", p, "--base-head", commit, "--main", other, "42"},
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
		// The value forms are mergepr's own tests (TestParseDrop,
		// TestParseRejectedDrop); one row per flag shows each is wired.
		extend("paragraph", "--drop", "1:removed"),
		extend("paragraph", "--drop-rejected", commit),
		extend("paragraph", "--drop", "1:disputed", "--drop", "1:reverted"),
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
		// The take-in subcommands: each needs its PR number and exactly the
		// values the subcommand takes, and none takes a state file or a drop.
		slices.Clone(bases["mergemain-begin"][:3]),
		extend("mergemain-begin", "43"),
		extend("mergemain-begin", "--state", "s"),
		extend("mergemain-begin", "--head-branch", "feature/foo"),
		extend("mergemain-begin", "--drop", "1:disputed"),
		slices.Clone(bases["mergemain-merge"][:len(bases["mergemain-merge"])-1]),
		{"mergemain-merge", "--protocol", p, "--head-branch", "feature/foo", "--base-head", commit, "42"},
		{"mergemain-merge", "--protocol", p, "--head-branch", "feature/foo", "--main", other, "42"},
		{"mergemain-merge", "--protocol", p, "--base-head", commit, "--main", other, "42"},
		append(slices.Clone(bases["mergemain-merge"]), "43"),
		extend("mergemain-merge", "--subject-file", "j"),
		extend("mergemain-restore", "--body-file", "b"),
		{"mergemain-restore", "--protocol", p, "--head-branch", "feature/foo", "--base-head", commit, "--main", other},
		{"mergemain-push", "--protocol", p, "--main", other, "42"},
		{"mergemain-push", "--protocol", p, "--base-head", commit, "42"},
		extend("mergemain-push", "--head-branch", "feature/foo"),
		slices.Clone(bases["mergemain-push"][:len(bases["mergemain-push"])-1]),
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

// TestPrintReport verifies that once the PR is known to be merged the report
// says whether the post-merge comparison verified the correction lines, also
// when the merge commit is unknown, and that a report of no merge says
// nothing about them.
func TestPrintReport(t *testing.T) {
	merged := mergepr.Report{ProtocolCheck: "passed", MergeCommitOID: "m", BaseUpdated: true, LocalDeleted: true, Merged: true}
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
		{"merge commit unknown", mergepr.Report{ProtocolCheck: "passed", Merged: true}, "protocol:             passed\ncorrections carried:  not verified\n"},
		{"not merged", mergepr.Report{ProtocolCheck: "passed"}, "protocol:             passed\n"},
		{"not merged with a note", mergepr.Report{ProtocolCheck: "passed", Note: "n"}, "protocol:             passed\nnote: n\n"},
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

// fakeTake records each take-in call and returns the results and the error
// the test scripts, so a test of the command never reaches git or gh.
type fakeTake struct {
	calls   []string
	begin   mergepr.MergemainBeginResult
	merge   mergepr.MergemainMergeResult
	restore mergepr.MergemainRestoreResult
	push    mergepr.MergemainPushResult
	err     error
}

func (f *fakeTake) MergemainBegin(_ context.Context, pr mergepr.MergemainPR) (mergepr.MergemainBeginResult, error) {
	f.calls = append(f.calls, "begin "+pr.String())
	return f.begin, f.err
}

func (f *fakeTake) MergemainMerge(_ context.Context, pr mergepr.MergemainPR, headBranch, baseHead, main string) (mergepr.MergemainMergeResult, error) {
	f.calls = append(f.calls, strings.Join([]string{"merge", pr.String(), headBranch, baseHead, main}, " "))
	return f.merge, f.err
}

func (f *fakeTake) MergemainRestore(_ context.Context, pr mergepr.MergemainPR, headBranch, baseHead, main string) (mergepr.MergemainRestoreResult, error) {
	f.calls = append(f.calls, strings.Join([]string{"restore", pr.String(), headBranch, baseHead, main}, " "))
	return f.restore, f.err
}

func (f *fakeTake) MergemainPush(_ context.Context, pr mergepr.MergemainPR, baseHead, main string) (mergepr.MergemainPushResult, error) {
	f.calls = append(f.calls, strings.Join([]string{"push", pr.String(), baseHead, main}, " "))
	return f.push, f.err
}

// runTakeIn runs one take-in command line against take and returns the output
// and the error. The take-in subcommands never touch the /mergepr tool, so
// none is passed.
func runTakeIn(t *testing.T, take *fakeTake, args ...string) (string, error) {
	t.Helper()
	inv, err := parse(args)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	err = execute(context.Background(), &b, nil, take, inv)
	return b.String(), err
}

// TestMergemainArgValidation verifies what the command accepts as a take-in
// PR number: decimal digits without a sign or a leading zero, one to nine of
// them. A number of another form stops the tool with a status: stop block that
// asks for no restore, before it calls the tool or reads anything from git; a
// missing number, a second positional argument, or a signed one is a usage
// error instead, because the command line's shape is wrong.
func TestMergemainArgValidation(t *testing.T) {
	p := strconv.Itoa(mergepr.Protocol)
	for _, pr := range []string{"1", "42", "123456789"} {
		take := &fakeTake{}
		if _, err := runTakeIn(t, take, "mergemain-begin", "--protocol", p, pr); err != nil {
			t.Errorf("mergemain-begin %q: error = %v, want nil", pr, err)
		}
		if want := "begin " + pr; !slices.Equal(take.calls, []string{want}) {
			t.Errorf("mergemain-begin %q: calls = %v, want %q", pr, take.calls, want)
		}
	}
	for _, pr := range []string{"0", "042", "4a", "1.0", " 42", "1234567890", ""} {
		take := &fakeTake{}
		got, err := runTakeIn(t, take, "mergemain-begin", "--protocol", p, pr)
		if !errors.Is(err, errStopped) {
			t.Errorf("mergemain-begin %q: error = %v, want the stop error", pr, err)
		}
		want := "status: stop\nreason: premise\nrestore: not_needed\ndetail: the PR argument " + strconv.Quote(pr) + " is not a decimal PR number\n"
		if got != want {
			t.Errorf("mergemain-begin %q wrote\n%s\nwant\n%s", pr, got, want)
		}
		if len(take.calls) != 0 {
			t.Errorf("mergemain-begin %q called the tool %v, want no call", pr, take.calls)
		}
	}
	for _, args := range [][]string{
		{"mergemain-begin", "--protocol", p},
		{"mergemain-begin", "--protocol", p, "42", "43"},
		// A signed number is read as a flag, so the flag parsing refuses it.
		{"mergemain-begin", "--protocol", p, "-1"},
	} {
		if _, err := runTakeIn(t, &fakeTake{}, args...); !errors.Is(err, errUsage) {
			t.Errorf("parse(%q) error = %v, want errUsage", args, err)
		}
	}
}

// TestMergemainOutput verifies the block each take-in subcommand writes when
// it finishes: status: ok and the values the caller and the report need, each
// on a field line of its own, and that the arguments the command line gives
// reach the tool unchanged.
func TestMergemainOutput(t *testing.T) {
	p := strconv.Itoa(mergepr.Protocol)
	const baseHead = "2222222222222222222222222222222222222222"
	const mainHead = "4444444444444444444444444444444444444444"
	const newHead = "3333333333333333333333333333333333333333"
	const branch = "feature/foo"
	tests := []struct {
		name     string
		args     []string
		take     *fakeTake
		wantCall string
		want     string
	}{
		{
			name:     "begin",
			args:     []string{"mergemain-begin", "--protocol", p, "42"},
			take:     &fakeTake{begin: mergepr.MergemainBeginResult{BaseHead: baseHead, HeadBranch: branch, Main: mainHead}},
			wantCall: "begin 42",
			want:     "status: ok\nbase_head: " + baseHead + "\nhead_branch: " + branch + "\nmain_head: " + mainHead + "\n",
		},
		{
			name: "merge",
			args: []string{
				"mergemain-merge", "--protocol", p, "--head-branch", branch, "--base-head", baseHead, "--main", mainHead, "42",
			},
			take: &fakeTake{merge: mergepr.MergemainMergeResult{
				Conflicts: []mergepr.PathConflict{
					{Path: "CLAUDE.md", Class: mergepr.PathConflictMarkdown},
					{Path: "internal/chief/records.go", Class: mergepr.PathConflictProtocolOnly},
				},
				Protocol:           []mergepr.ProtocolBump{{Tool: "chief", Value: 7}, {Tool: "mergepr", Value: 4}},
				ReadmeChanged:      true,
				ADRMainChanged:     false,
				ADRConflictNumbers: []string{"0046"},
				ExtCheckHint:       true,
			}},
			wantCall: "merge 42 " + branch + " " + baseHead + " " + mainHead,
			want: "status: ok\n" +
				"conflict: markdown CLAUDE.md\n" +
				"conflict: protocol_only internal/chief/records.go\n" +
				"protocol_tool: chief 7\nprotocol_tool: mergepr 4\n" +
				"readme_changed: true\nadr_main_changed: false\nadr_conflict_number: 0046\next_check_hint: true\n",
		},
		{
			name: "merge with no conflict left",
			args: []string{
				"mergemain-merge", "--protocol", p, "--head-branch", branch, "--base-head", baseHead, "--main", mainHead, "42",
			},
			take:     &fakeTake{},
			wantCall: "merge 42 " + branch + " " + baseHead + " " + mainHead,
			want:     "status: ok\nreadme_changed: false\nadr_main_changed: false\next_check_hint: false\n",
		},
		{
			name: "restore",
			args: []string{
				"mergemain-restore", "--protocol", p, "--head-branch", branch, "--base-head", baseHead, "--main", mainHead, "42",
			},
			take:     &fakeTake{restore: mergepr.MergemainRestoreResult{Head: baseHead}},
			wantCall: "restore 42 " + branch + " " + baseHead + " " + mainHead,
			want:     "status: ok\nbase_head: " + baseHead + "\n",
		},
		{
			name:     "push",
			args:     []string{"mergemain-push", "--protocol", p, "--base-head", baseHead, "--main", mainHead, "42"},
			take:     &fakeTake{push: mergepr.MergemainPushResult{NewHead: newHead}},
			wantCall: "push 42 " + baseHead + " " + mainHead,
			want:     "status: ok\nnew_head: " + newHead + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runTakeIn(t, tt.take, tt.args...)
			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("wrote\n%s\nwant\n%s", got, tt.want)
			}
			if want := []string{tt.wantCall}; !slices.Equal(tt.take.calls, want) {
				t.Errorf("calls = %v, want %q", tt.take.calls, want)
			}
		})
	}
}

// takeInFieldKeys are the keys a take-in block may write, one field per line.
var takeInFieldKeys = map[string]bool{
	"status": true, "reason": true, "restore": true, "detail": true,
	"base_head": true, "head_branch": true, "main_head": true, "new_head": true,
	"conflict": true, "protocol_tool": true, "readme_changed": true,
	"adr_main_changed": true, "adr_conflict_number": true, "ext_check_hint": true,
}

// TestMergemainStopReasonsAndOutput verifies the block a stopped take-in
// subcommand writes: status: stop, each declared reason with its restore need,
// the conflicted paths with their classes, and the detail; that a stop of no
// declared kind reads as reason: unknown with restore: unknown; and that a
// value holding a newline, a tab, a carriage return, a backslash, a control
// byte, or a Unicode line separator cannot write a field line of its own.
// TestEscapedField covers the rest of the escaped character set.
func TestMergemainStopReasonsAndOutput(t *testing.T) {
	p := strconv.Itoa(mergepr.Protocol)
	const baseHead = "2222222222222222222222222222222222222222"
	const mainHead = "4444444444444444444444444444444444444444"
	const branch = "feature/foo"
	mergeArgs := []string{
		"mergemain-merge", "--protocol", p, "--head-branch", branch, "--base-head", baseHead, "--main", mainHead, "42",
	}
	tests := []struct {
		name string
		args []string
		err  error
		want string
	}{
		{
			name: "conflicts out of scope",
			args: mergeArgs,
			err: &mergepr.MergemainStopError{
				Stop:      mergepr.MergemainStopOutOfScope,
				Restore:   mergepr.RestoreNeeded,
				Detail:    "a conflict is out of scope",
				Conflicts: []mergepr.PathConflict{{Path: "internal/pipeline/pipeline.go", Class: mergepr.PathConflictOutOfScope}},
			},
			want: "status: stop\nreason: out_of_scope\nrestore: needed\n" +
				"conflict: out_of_scope internal/pipeline/pipeline.go\ndetail: a conflict is out of scope\n",
		},
		{
			name: "premise stopped before the merge",
			args: mergeArgs,
			err:  &mergepr.MergemainStopError{Stop: mergepr.MergemainStopPremise, Restore: mergepr.RestoreNotNeeded, Detail: "HEAD is not the base head"},
			want: "status: stop\nreason: premise\nrestore: not_needed\ndetail: HEAD is not the base head\n",
		},
		{
			name: "nothing to take in",
			args: []string{"mergemain-begin", "--protocol", p, "42"},
			err:  &mergepr.MergemainStopError{Stop: mergepr.MergemainStopNothingToTakeIn, Restore: mergepr.RestoreNotNeeded, Detail: "origin/main is already an ancestor of the head"},
			want: "status: stop\nreason: nothing_to_take_in\nrestore: not_needed\ndetail: origin/main is already an ancestor of the head\n",
		},
		{
			name: "push unknown, not restored",
			args: []string{"mergemain-push", "--protocol", p, "--base-head", baseHead, "--main", mainHead, "42"},
			err:  &mergepr.MergemainStopError{Stop: mergepr.MergemainStopPushUnknown, Restore: mergepr.RestoreUnknown, Detail: "origin's branch could not be read again"},
			want: "status: stop\nreason: push_unknown\nrestore: unknown\ndetail: origin's branch could not be read again\n",
		},
		{
			name: "push rejected, restored",
			args: []string{"mergemain-push", "--protocol", p, "--base-head", baseHead, "--main", mainHead, "42"},
			err:  &mergepr.MergemainStopError{Stop: mergepr.MergemainStopPushRejected, Restore: mergepr.RestoreNeeded, Detail: "origin's branch is still the base head"},
			want: "status: stop\nreason: push_rejected\nrestore: needed\ndetail: origin's branch is still the base head\n",
		},
		{
			name: "other changes, not restored",
			args: []string{
				"mergemain-restore", "--protocol", p, "--head-branch", branch, "--base-head", baseHead, "--main", mainHead, "42",
			},
			err:  &mergepr.MergemainStopError{Stop: mergepr.MergemainStopOtherChanges, Restore: mergepr.RestoreNotNeeded, Detail: "an untracked file is in the work tree"},
			want: "status: stop\nreason: other_changes\nrestore: not_needed\ndetail: an untracked file is in the work tree\n",
		},
		{
			name: "stop of no declared kind",
			args: mergeArgs,
			err:  errors.New("git failed\nwith two lines"),
			want: "status: stop\nreason: unknown\nrestore: unknown\ndetail: git failed\\nwith two lines\n",
		},
		{
			name: "values cannot write a field line",
			args: mergeArgs,
			err: &mergepr.MergemainStopError{
				Stop:    mergepr.MergemainStopOutOfScope,
				Restore: mergepr.RestoreNeeded,
				// A carriage return, DEL, and a Unicode line separator are
				// escaped like a newline and a control byte.
				Detail:    "x\nreason: nothing_to_take_in",
				Conflicts: []mergepr.PathConflict{{Path: "a\rstatus: ok\tb\\n\x01\x7f\u2028", Class: mergepr.PathConflictOutOfScope}},
			},
			want: "status: stop\nreason: out_of_scope\nrestore: needed\n" +
				"conflict: out_of_scope a\\rstatus: ok\\tb\\\\n\\x01\\x7f\\u2028\ndetail: x\\nreason: nothing_to_take_in\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			take := &fakeTake{err: tt.err}
			got, err := runTakeIn(t, take, tt.args...)
			if !errors.Is(err, errStopped) {
				t.Fatalf("error = %v, want the stop error", err)
			}
			if got != tt.want {
				t.Errorf("wrote\n%s\nwant\n%s", got, tt.want)
			}
			for line := range strings.Lines(got) {
				key, _, ok := strings.Cut(strings.TrimSuffix(line, "\n"), ": ")
				if !ok || !takeInFieldKeys[key] {
					t.Errorf("line %q is not a field line of the take-in output", strings.TrimSuffix(line, "\n"))
				}
			}
		})
	}
}

// TestEscapedField verifies the character set a field value is escaped with:
// the short escapes for a backslash, a newline, a carriage return, and a tab;
// a visible escape for an invalid UTF-8 byte and for every control, line
// separator, and format character; and no change to any other character. A
// reader that decodes the output as UTF-8 therefore sees no line break and no
// reordering character inside a value.
func TestEscapedField(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "a b", "a b"},
		{"backslash", `a\b`, `a\\b`},
		{"newline", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"null and other C0", "a\x00\x01b", `a\x00\x01b`},
		{"DEL", "a\x7fb", `a\x7fb`},
		{"C1 control", "a\u0085b", `a\u0085b`},
		{"line separator", "a\u2028b", `a\u2028b`},
		{"paragraph separator", "a\u2029b", `a\u2029b`},
		{"bidirectional override", "a\u202eb", `a\u202eb`},
		{"zero width space", "a\u200bb", `a\u200bb`},
		{"tag character", "a\U000e0041b", `a\U000e0041b`},
		{"invalid UTF-8", "a\xffb", `a\xffb`},
		{"non-ASCII kept", "a\u3042\u0301b", "a\u3042\u0301b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapedField(tt.in); got != tt.want {
				t.Errorf("escapedField(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestExitStatus verifies the process's exit status and what it writes on
// standard error: a finished run exits 0 and writes nothing, a declared
// take-in stop exits 1 and writes nothing more because its status block is
// already on standard output, and any other error exits 1 with its line.
func TestExitStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStderr string
	}{
		{"finished", nil, 0, ""},
		{"declared stop", fmt.Errorf("%w: out_of_scope", errStopped), 1, ""},
		{"usage error", errUsage, 1, "mergepr: " + errUsage.Error() + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			if got := exitStatus(&b, tt.err); got != tt.wantCode {
				t.Errorf("exitStatus = %d, want %d", got, tt.wantCode)
			}
			if got := b.String(); got != tt.wantStderr {
				t.Errorf("standard error = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}
