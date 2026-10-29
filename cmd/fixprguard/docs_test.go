//go:build test

package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestGuidePathExists: a usage error's diagnostic names the guide, and that
// file exists, so a user who sees the diagnostic can open it.
func TestGuidePathExists(t *testing.T) {
	const guide = "docs/dev/guides/fixprguard_guide.md"
	r := newTestRun(t, nil)
	if code := r.Run(t); code != exitUsage {
		t.Fatalf("no arguments: exit %d, want %d (%s)", code, exitUsage, r.Stderr)
	}
	if want := "(see " + guide + ")"; !strings.Contains(r.Stderr.String(), want) {
		t.Errorf("stderr = %q, want it to contain %q", r.Stderr, want)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, guide)); err != nil {
		t.Errorf("the guide %q does not exist: %v", guide, err)
	}
}

// TestGuideCoversStopKindsAndScope: the guide names, in code spans, every stop
// kind the tool can print. In chapter 6, every deny-list item is in the
// execution-related file table and every allow-list item in the allow-list
// table. The values come from internal/fixprguard, so a kind or an item added
// there without the guide fails here. Reading the tables alone keeps a code
// span in the prose around them from standing in for a row.
func TestGuideCoversStopKindsAndScope(t *testing.T) {
	guide := readRepoFile(t, guidePath)
	for _, kind := range fixprguard.StopKinds() {
		if !strings.Contains(guide, "`"+kind+"`") {
			t.Errorf("%s does not name stop kind `%s`", guidePath, kind)
		}
	}
	scope := section(guide, "## 6.")
	execTable := guideTable(scope, "**実行関連ファイル。**")
	allowTable := guideTable(scope, "**許可リスト。**")
	for _, table := range []struct {
		name, text string
		patterns   []string
	}{
		{"execution-related file", execTable, fixprguard.ExecutionFilePatterns()},
		{"allow-list", allowTable, fixprguard.AllowListPatterns()},
	} {
		for _, pattern := range table.patterns {
			if !strings.Contains(table.text, "`"+pattern+"`") {
				t.Errorf("%s: the %s table in chapter 6 does not list `%s`", guidePath, table.name, pattern)
			}
		}
	}
}

// guideTable returns the lines of the first table after the line of doc that
// starts with marker. It returns "" when there is no such line or another
// bold heading comes before a table, so a missing table never borrows the
// next one.
func guideTable(doc, marker string) string {
	_, rest, ok := strings.Cut(doc, "\n"+marker)
	if !ok {
		return ""
	}
	_, rest, _ = strings.Cut(rest, "\n")
	var table strings.Builder
	for line := range strings.Lines(rest) {
		switch {
		case strings.HasPrefix(line, "|"):
			table.WriteString(line)
		case table.Len() > 0, strings.HasPrefix(line, "**"):
			return table.String()
		}
	}
	return table.String()
}

// TestGuideTable: guideTable takes the table's lines only, stops where the
// table ends, and finds nothing for a missing marker or table.
func TestGuideTable(t *testing.T) {
	doc := "## 6\n**A.** a `p`\n\n| h |\n|---|\n| `x` |\nafter `y`\n\n| `w` |\n**B.** b\n**C.** c\n\n| `z` |\n"
	for _, tc := range []struct {
		name   string
		doc    string
		marker string
		want   string
	}{
		{"stops_at_table_end", doc, "**A.**", "| h |\n|---|\n| `x` |\n"},
		{"table_after_prose", doc, "**C.**", "| `z` |\n"},
		// Bullets and a paragraph between the marker and the table, as in
		// the guide.
		{"table_after_bullets", "x\n**E.** e\n- bullet `q`\npara\n\n| `v` |\n", "**E.**", "| `v` |\n"},
		// The next bold heading comes before any table.
		{"no_table_before_next_heading", doc, "**B.**", ""},
		{"missing_marker", doc, "**D.**", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := guideTable(tc.doc, tc.marker); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// reviewerPlaceholder is the --reviewer value the command documents write.
// The documents write it once per call and define it as one --reviewer per
// name of the automated reviewer list in _context.md, so the names live in
// _context.md alone and the list can change without editing a call.
const reviewerPlaceholder = "<reviewer>"

var (
	errReviewerCount = errors.New("--reviewer not given exactly once")
	errReviewerValue = errors.New("--reviewer is not the placeholder")
)

// checkDocumentedReviewer checks a parsed documented call of a subcommand that
// takes --reviewer: its collected --reviewer values must be exactly one, the
// placeholder. A call of another subcommand has nothing to check (parse
// refuses --reviewer there).
func checkDocumentedReviewer(inv invocation) error {
	if !slices.Contains(specs[inv.name].flags, flagReviewer) {
		return nil
	}
	values := inv.multi[flagReviewer]
	if len(values) != 1 {
		return fmt.Errorf("%w: %d values", errReviewerCount, len(values))
	}
	if values[0] != reviewerPlaceholder {
		return fmt.Errorf("%w: %q", errReviewerValue, values[0])
	}
	return nil
}

// documentedInvocations returns the argument lists of the fixprguard calls in
// doc's fenced code blocks, one per line that starts with "fixprguard ". A
// blockquote marker is dropped first, so a block inside an agent prompt is
// read too.
func documentedInvocations(doc string) [][]string {
	var calls [][]string
	inBlock := false
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(trimmed, ">"); ok {
			trimmed = strings.TrimSpace(rest)
		}
		if strings.HasPrefix(trimmed, "```") {
			inBlock = !inBlock
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "fixprguard "); inBlock && ok {
			calls = append(calls, strings.Fields(rest))
		}
	}
	return calls
}

// TestDocumentedInvocationsParse runs every fixprguard call written in
// fixpr.md and runpr.md through the tool's own command-line parser: the
// subcommand, the flag names, the positional argument, and --protocol must be
// what this binary accepts, and --reviewer must be given once, as the
// placeholder for the names in _context.md. Each document must also call the
// subcommands it uses.
func TestDocumentedInvocationsParse(t *testing.T) {
	for _, tc := range []struct {
		rel  string
		want []string
	}{
		{".claude/commands/fixpr.md", []string{cmdBegin, cmdFence, cmdInspect, cmdFinish, cmdPush, cmdReply, cmdEditPR}},
		{".claude/commands/runpr.md", []string{cmdHead, cmdList, cmdReply}},
	} {
		calls := documentedInvocations(readRepoFile(t, tc.rel))
		seen := map[string]struct{}{}
		for _, args := range calls {
			inv, err := parse(args)
			if err != nil {
				t.Errorf("%s: fixprguard %s: %v", tc.rel, strings.Join(args, " "), err)
				continue
			}
			seen[inv.name] = struct{}{}
			if err := checkDocumentedReviewer(inv); err != nil {
				t.Errorf("%s: fixprguard %s: %v", tc.rel, inv.name, err)
			}
		}
		for _, name := range tc.want {
			if _, ok := seen[name]; !ok {
				t.Errorf("%s has no fixprguard %s call", tc.rel, name)
			}
		}
	}
}

// TestCheckDocumentedReviewer runs the documented-call check on synthesized
// calls, so a check loosened later fails here and not only when a document
// happens to break it.
func TestCheckDocumentedReviewer(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want error
	}{
		{"one_placeholder", args(cmdList, "--reviewer", reviewerPlaceholder, testPR), nil},
		{"no_reviewer_subcommand", args(cmdHead, testPR), nil},
		{"two_placeholders", args(cmdBegin, "--reviewer", reviewerPlaceholder, "--reviewer", reviewerPlaceholder, testPR), errReviewerCount},
		{"placeholder_and_name", args(cmdBegin, "--reviewer", reviewerPlaceholder, "--reviewer", testReviewer, testPR), errReviewerCount},
		{"name_itself", args(cmdInspect, "--reviewer", testReviewer, "--baseline", "<b>", testPR), errReviewerValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := parse(tc.args)
			if err != nil {
				t.Fatalf("parse(%q) = %v", tc.args, err)
			}
			if err := checkDocumentedReviewer(inv); !errors.Is(err, tc.want) {
				t.Errorf("checkDocumentedReviewer = %v, want %v", err, tc.want)
			}
		})
	}
}

// singleLine returns the one line of doc that starts with prefix, failing when
// there is none or more than one.
func singleLine(t *testing.T, rel, doc, prefix string) string {
	t.Helper()
	var found []string
	for line := range strings.Lines(doc) {
		if strings.HasPrefix(line, prefix) {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Errorf("%s has %d lines starting with %q, want 1", rel, len(found), prefix)
		return ""
	}
	return found[0]
}

// section returns the text of doc from the heading line that starts with
// heading up to the next level-2 heading.
func section(doc, heading string) string { return cutSection(doc, heading, "\n## ") }

// subsection returns the text of doc from the line that starts with heading
// up to the next heading of level 2 or 3.
func subsection(doc, heading string) string { return cutSection(doc, heading, "\n## ", "\n### ") }

// cutSection returns the text of doc from the line that starts with heading
// up to the first of ends, or "" when there is no such line.
func cutSection(doc, heading string, ends ...string) string {
	_, rest, ok := strings.Cut(doc, "\n"+heading)
	if !ok {
		return ""
	}
	end := len(rest)
	for _, next := range ends {
		if i := strings.Index(rest, next); i >= 0 && i < end {
			end = i
		}
	}
	return rest[:end]
}

// TestGuideDescribesReviewerList: the subcommand headings that write
// "--reviewer LOGIN ..." in the guide are exactly the subcommands whose
// spec takes --reviewer, so a subcommand that starts or stops taking the flag
// fails here until the guide changes too.
func TestGuideDescribesReviewerList(t *testing.T) {
	takesReviewer := map[string]struct{}{}
	for name, sp := range specs {
		if slices.Contains(sp.flags, flagReviewer) {
			takesReviewer[name] = struct{}{}
		}
	}
	headed := map[string]struct{}{}
	for line := range strings.Lines(readRepoFile(t, guidePath)) {
		rest, ok := strings.CutPrefix(line, "### `")
		if !ok {
			continue
		}
		if name, _, _ := strings.Cut(rest, " "); strings.Contains(rest, "--reviewer LOGIN ...") {
			headed[name] = struct{}{}
		}
	}
	if !maps.Equal(headed, takesReviewer) {
		t.Errorf("headings with --reviewer LOGIN ... are %v, want the subcommands taking --reviewer %v", slices.Sorted(maps.Keys(headed)), slices.Sorted(maps.Keys(takesReviewer)))
	}
}

// protocolFlag and protocolOutput match the spellings the documents use:
// --protocol N or --protocol=N, and "protocol":N with optional space.
var (
	protocolFlag   = regexp.MustCompile("--protocol[ =]([0-9]+)")
	protocolOutput = regexp.MustCompile(`"protocol":\s*([0-9]+)`)
)

// TestDocumentedProtocolMatches: every protocol number the documents write
// is the binary's Protocol: each --protocol N in the guide and in every
// command document under .claude/commands (prose included, not only code
// blocks) that is not in a codexreview call, which codexreview's own guard
// checks, each "protocol":N of the guide's output examples, and the current
// number chapter 4 of the guide states. The guide must write each at least
// once, so a guide that stops writing them is not read as agreeing; that
// fixpr.md and runpr.md write them is TestDocumentedInvocationsParse's check.
func TestDocumentedProtocolMatches(t *testing.T) {
	want := strconv.Itoa(fixprguard.Protocol)
	check := func(rel, doc string, re *regexp.Regexp, required bool) {
		t.Helper()
		found := re.FindAllStringSubmatch(doc, -1)
		if required && len(found) == 0 {
			t.Errorf("%s has no %s", rel, re)
		}
		for _, m := range found {
			if m[1] != want {
				t.Errorf("%s: %q, want %s", rel, m[0], want)
			}
		}
	}
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		for _, text := range fixprguardProtocolMismatches(readRepoFile(t, rel)) {
			t.Errorf("%s: %q, want %s", rel, text, want)
		}
	}
	guide := readRepoFile(t, guidePath)
	check(guidePath, guide, protocolFlag, true)
	check(guidePath, guide, protocolOutput, true)
	check(guidePath, singleLine(t, guidePath, guide, "| `--protocol N` |"), regexp.MustCompile("現在の番号は `([0-9]+)`"), true)
}

// fixprguardProtocolMismatches returns the --protocol flags of doc that this
// guard checks, every one doctestutil.ProtocolFlags does not give to
// codexreview, whose value is not Protocol.
func fixprguardProtocolMismatches(doc string) []string {
	var mismatches []string
	for _, f := range doctestutil.ProtocolFlags(doc) {
		if f.Tool != doctestutil.ToolCodexreview && f.Value != strconv.Itoa(fixprguard.Protocol) {
			mismatches = append(mismatches, f.Text)
		}
	}
	return mismatches
}

// TestFixprguardProtocolMismatches: the check skips codexreview's flags
// only, so a wrong number in a fixprguard call, or in prose naming no tool,
// is still found next to a codexreview call.
func TestFixprguardProtocolMismatches(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	right := strconv.Itoa(fixprguard.Protocol)
	wrong := strconv.Itoa(fixprguard.Protocol + 1)
	for _, tc := range []struct {
		name string
		doc  string
		want []string
	}{
		{"codexreview_call_skipped", "codexreview --protocol " + wrong + " --base " + sha + "\n", nil},
		{"wrong_fixprguard_call_after_codexreview", "codexreview --protocol " + wrong + " --base " + sha + " && fixprguard head --protocol " + wrong + "\n", []string{"--protocol " + wrong}},
		{"wrong_number_in_prose", "pass `--protocol=" + wrong + "`\n", []string{"--protocol=" + wrong}},
		{"right_number", "fixprguard head --protocol " + right + "\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fixprguardProtocolMismatches(tc.doc); !slices.Equal(got, tc.want) {
				t.Errorf("mismatches = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGuideDescribesMentionCheck: the guide names, in 7.1 and 7.2, the
// mention reply and edit-pr reject, taking the value from internal/fixprguard,
// so changing the constant without the guide fails here. Whether the prose
// around it is right is checked by review, not pinned by wording.
func TestGuideDescribesMentionCheck(t *testing.T) {
	mention := "`" + fixprguard.CodexMention + "`"
	guide := readRepoFile(t, guidePath)
	for _, heading := range []string{"### 7.1", "### 7.2"} {
		if !strings.Contains(subsection(guide, heading), mention) {
			t.Errorf("%s %s does not name %s", guidePath, heading, mention)
		}
	}
}

// TestCutSection: the extraction the guards read through stops where it
// should. A section that ran on would let a later passage supply a phrase
// the checked one lacks.
func TestCutSection(t *testing.T) {
	doc := "intro\n## A\na\n### B\nb\n### C\nc\n## D\nd\n### E\ne\n"
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"section_runs_past_level_3", section(doc, "## A"), "\na\n### B\nb\n### C\nc"},
		{"subsection_stops_at_level_3", subsection(doc, "### B"), "\nb"},
		// A level-3 heading follows, but the level-2 one comes first.
		{"subsection_stops_at_level_2", subsection(doc, "### C"), "\nc"},
		{"section_runs_to_the_end", section(doc, "## D"), "\nd\n### E\ne\n"},
		{"missing_heading", subsection(doc, "### F"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

// codeSpan matches one inline code span.
var codeSpan = regexp.MustCompile("`([^`]+)`")

// codeSpans returns the set of code spans in text.
func codeSpans(text string) map[string]struct{} {
	spans := map[string]struct{}{}
	for _, m := range codeSpan.FindAllStringSubmatch(text, -1) {
		spans[m[1]] = struct{}{}
	}
	return spans
}

// TestGuideListsWatchList: the table after the watch list marker in 7.1 lists
// exactly the patterns WatchListPatterns returns, no more and no fewer. Only
// the table is read: most items also appear in chapter 6 and in prose, which
// would hide an item missing from the table.
func TestGuideListsWatchList(t *testing.T) {
	want := map[string]struct{}{}
	for _, p := range fixprguard.WatchListPatterns() {
		want[p] = struct{}{}
	}
	table := guideTable(subsection(readRepoFile(t, guidePath), "### 7.1"), "**監視の一覧。**")
	if got := codeSpans(table); !maps.Equal(got, want) {
		t.Errorf("the watch list table lists %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
	}
}

// TestGuideNamesGitStateParts: the list of parts in the git-state-changed
// row of chapter 8 in the guide names every value parts can hold, and
// fixpr.md's list of the parts a git-state-changed stop names includes
// ignored. Only the list is read, since the row's other cells name parts
// too. fixpr.md words the other parts (HEAD, the remote branch, ...), so only
// ignored is pinned there.
func TestGuideNamesGitStateParts(t *testing.T) {
	row := singleLine(t, guidePath, readRepoFile(t, guidePath), "| `git-state-changed` |")
	_, rest, ok := strings.Cut(row, "`parts`（")
	list, _, closed := strings.Cut(rest, "）")
	if !ok || !closed {
		t.Errorf("%s: the git-state-changed row has no list after %q", guidePath, "`parts`（")
	} else {
		spans := codeSpans(list)
		for _, part := range fixprguard.GitStateParts() {
			if _, ok := spans[part]; !ok {
				t.Errorf("%s: the git-state-changed row's parts do not name `%s`", guidePath, part)
			}
		}
	}
	const fixprDoc = ".claude/commands/fixpr.md"
	const partsList = "name the `parts` that differ from the baseline ("
	_, docRest, docOK := strings.Cut(readRepoFile(t, fixprDoc), partsList)
	docList, _, docClosed := strings.Cut(docRest, ")")
	if !docOK || !docClosed || !strings.Contains(docList, "`"+fixprguard.PartIgnored+"`") {
		t.Errorf("%s: the list after %q does not name `%s`", fixprDoc, partsList, fixprguard.PartIgnored)
	}
}

// TestGuideDescribesIgnoredFiles: in 8.2 the guide lists every rule
// IgnoredFailureRules names. fixpr.md recognizes an ignored-file rule by
// "contains ignored, or is .git name below .claude", so every rule must fit
// that.
func TestGuideDescribesIgnoredFiles(t *testing.T) {
	const claudeGitRule = ".git name below .claude"
	for _, rule := range fixprguard.IgnoredFailureRules() {
		if !strings.Contains(rule, fixprguard.PartIgnored) && rule != claudeGitRule {
			t.Errorf("rule %q contains neither %q nor is %q, so fixpr.md would not recognize it", rule, fixprguard.PartIgnored, claudeGitRule)
		}
	}
	guide := readRepoFile(t, guidePath)
	spans := codeSpans(guideTable(subsection(guide, "### 8.2"), "| 規則の名前 | 原因 | 対処 |"))
	for _, rule := range fixprguard.IgnoredFailureRules() {
		if _, ok := spans[rule]; !ok {
			t.Errorf("%s: the 8.2 table does not list `%s`", guidePath, rule)
		}
	}
}

// diffLine is one line of the 8.1 block's diff of two records: a change
// command, the separator, or a whole record line on either side.
var diffLine = regexp.MustCompile(`^([0-9,]+[acd][0-9,]+|---|[<>] [0-9a-f]{64} "[ -~]*")$`)

// guideBlock returns the first sh code block of the guide's subsection.
func guideBlock(t *testing.T, rel, heading string) string {
	t.Helper()
	_, rest, ok := strings.Cut(subsection(readRepoFile(t, rel), heading), "\n```sh\n")
	block, _, closed := strings.Cut(rest, "\n```\n")
	if !ok || !closed {
		t.Fatalf("%s %s has no sh code block", rel, heading)
	}
	return block + "\n"
}

// TestGuideIgnoredCommands runs the guide's record check of 8.1 as the user
// would, on records begin and inspect wrote: with the token begin printed it
// reports a match and diff names the path that changed; with the begin record
// rewritten it reports no match and runs no diff. The block prefers sha256sum
// and falls
// back to shasum, so it is run once with each alone on PATH; each installed
// tool runs, and at least one must be installed.
func TestGuideIgnoredCommands(t *testing.T) {
	block := guideBlock(t, guidePath, "### 8.1")
	// pathWith is a PATH directory holding the block's tools and one of
	// the two SHA-256 commands, so the block takes that command's branch.
	pathWith := func(sha string) (string, bool) {
		dir := t.TempDir()
		for _, tool := range []string{sha, "sed", "cut", "diff", "cat"} {
			target, err := exec.LookPath(tool)
			if err != nil {
				return "", false
			}
			if err := os.Symlink(target, filepath.Join(dir, tool)); err != nil {
				t.Fatal(err)
			}
		}
		return dir, true
	}
	var paths []string
	for _, sha := range []string{"sha256sum", "shasum"} {
		if dir, ok := pathWith(sha); ok {
			paths = append(paths, dir)
		}
	}
	if len(paths) == 0 {
		t.Fatal("unsupported test environment: neither sha256sum nor shasum is installed")
	}
	repo := newTestRepo(t)
	repo.Ignore("build/")
	repo.Write("build/kept.go", "package x\n")
	repo.Write("build/changed.go", "package x\n")
	r := newTestRun(t, beginSteps(threadsPage(t, false)))
	if code := r.Run(t, args(cmdBegin, "--reviewer", testReviewer, testPR)...); code != exitContinue {
		t.Fatalf("begin: exit %d (%s)", code, r.Stderr)
	}
	var out beginJSON
	r.Decode(t, &out)
	repo.Write("build/changed.go", "package y\n")
	inspect := newTestRun(t, nil)
	if code := inspect.Run(t, args(cmdInspect, "--reviewer", testReviewer, "--baseline", out.Baseline, testPR)...); code != exitStop {
		t.Fatalf("inspect: exit %d, want %d (%s)", code, exitStop, inspect.Stderr)
	}
	runBlock := func(path string) string {
		t.Helper()
		cmd := exec.Command("sh", "-c", block) //nolint:gosec // the guide's own code block, run in the test's repository
		cmd.Dir = repo.Dir
		cmd.Env = append(os.Environ(), "token="+out.Baseline, "PATH="+path)
		got, err := cmd.Output()
		// diff exits 1 when the records differ; any other failure is one.
		if exitErr, ok := errors.AsType[*exec.ExitError](err); err != nil && (!ok || exitErr.ExitCode() != 1) {
			t.Fatalf("run the block: %v\n%s", err, got)
		}
		return string(got)
	}
	for _, path := range paths {
		got := runBlock(path)
		if !strings.HasPrefix(got, "begin record: matches the baseline\n") || !strings.Contains(got, `"build/changed.go"`) || strings.Contains(got, `"build/kept.go"`) {
			t.Errorf("PATH %s: block output = %q, want the match and a diff naming build/changed.go only", path, got)
		}
		// Each diff line is whole: a tool that wraps long lines (GNU sed's
		// l does) would split a record line inside its path.
		for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n")[1:] {
			if !diffLine.MatchString(line) {
				t.Errorf("PATH %s: diff line %q is not a whole diff or record line", path, line)
			}
		}
	}
	f, err := os.OpenFile(beginRecordName, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("0 \"forged\"\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if got := runBlock(path); got != "begin record: does not match the baseline, do not trust it\n" {
			t.Errorf("PATH %s: block output with the begin record rewritten = %q", path, got)
		}
	}
}
