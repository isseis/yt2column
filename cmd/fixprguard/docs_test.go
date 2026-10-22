//go:build test

package main

import (
	"errors"
	"fmt"
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
