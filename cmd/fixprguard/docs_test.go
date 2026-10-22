//go:build test

package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/fixprguard"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

// guideFiles are the Japanese original and the English translation of the
// fixprguard guide.
var guideFiles = []string{
	"docs/dev/developer_guide/fixprguard_guide.ja.md",
	"docs/dev/developer_guide/fixprguard_guide.md",
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestGuideCoversStopKindsAndScope: both guides name, in code spans, every
// stop kind the tool can print and every execution-related file definition
// the scope rules use, and CLAUDE.md links to the guide. The values come from
// internal/fixprguard, so a kind or a definition added there without the
// guide fails here.
func TestGuideCoversStopKindsAndScope(t *testing.T) {
	for _, rel := range guideFiles {
		guide := readRepoFile(t, rel)
		for _, kind := range fixprguard.StopKinds() {
			if !strings.Contains(guide, "`"+kind+"`") {
				t.Errorf("%s does not name stop kind `%s`", rel, kind)
			}
		}
		for _, pattern := range fixprguard.ExecutionFilePatterns() {
			if !strings.Contains(guide, "`"+pattern+"`") {
				t.Errorf("%s does not list execution-related file `%s`", rel, pattern)
			}
		}
	}
	if claude := readRepoFile(t, "CLAUDE.md"); !strings.Contains(claude, "](docs/dev/developer_guide/fixprguard_guide.md)") {
		t.Error("CLAUDE.md does not link to the fixprguard guide")
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
		{"one_placeholder_reply", args(cmdReply, "--reviewer", reviewerPlaceholder, "--thread-id", "<id>", "--comment-id", "<n>",
			"--fingerprint", "<f>", "--body-file", "<file>", testPR), nil},
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
// heading up to the next heading of the same level.
func section(doc, heading string) string {
	_, rest, ok := strings.Cut(doc, "\n"+heading)
	if !ok {
		return ""
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return body
}

// TestGuideDescribesReviewerList: both guides describe repeated --reviewer as
// the tool implements it. The subcommand headings that write
// "--reviewer LOGIN ..." are exactly the subcommands whose spec takes
// --reviewer; the flag row says the flag repeats once per name and that none
// or a name given twice is a usage error; other-bot is a Bot matching none of
// the names; the exit code 2 row identifies a tool that predates the change
// and how to check its version; and section 9 states the Protocol proviso.
// The phrases are the guides' own wording, Japanese for the Japanese guide.
func TestGuideDescribesReviewerList(t *testing.T) {
	takesReviewer := map[string]struct{}{}
	for name, sp := range specs {
		if slices.Contains(sp.flags, flagReviewer) {
			takesReviewer[name] = struct{}{}
		}
	}
	for _, tc := range []struct {
		rel      string
		flagRow  []string
		otherBot string
		proviso  string
	}{
		{
			rel:      guideFiles[0],
			flagRow:  []string{"名前ごとに 1 回ずつ繰り返して渡す", "1 つもないとき", "同じ名前を 2 回以上渡したとき", "使い方の誤り"},
			otherBot: "`--reviewer` で渡したどの名前とも一致しない Bot",
			proviso:  "次の 3 つの条件がすべて成り立つ変更に限り、`Protocol` を上げなくてよい",
		},
		{
			rel:      guideFiles[1],
			flagRow:  []string{"repeating the flag once per name", "When none is given", "when the same name is given twice or more", "usage error"},
			otherBot: "A Bot matching none of the names passed with `--reviewer`",
			proviso:  "only a change that meets all three conditions below may leave `Protocol` unchanged",
		},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			guide := readRepoFile(t, tc.rel)
			headed := map[string]struct{}{}
			for line := range strings.Lines(guide) {
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
			flagRow := singleLine(t, tc.rel, guide, "| `--reviewer LOGIN` |")
			for _, want := range tc.flagRow {
				if !strings.Contains(flagRow, want) {
					t.Errorf("the --reviewer row does not say %q", want)
				}
			}
			if row := singleLine(t, tc.rel, guide, "| `other-bot` |"); !strings.Contains(row, tc.otherBot) {
				t.Errorf("the other-bot row does not say %q", tc.otherBot)
			}
			exit2 := singleLine(t, tc.rel, guide, "| `2` |")
			for _, want := range []string{"`flag given twice: --reviewer`", "go version -m", "make install-fixprguard"} {
				if !strings.Contains(exit2, want) {
					t.Errorf("the exit code 2 row does not mention %s", want)
				}
			}
			if !strings.Contains(section(guide, "## 9."), tc.proviso) {
				t.Errorf("section 9 does not state the Protocol proviso %q", tc.proviso)
			}
		})
	}
}
