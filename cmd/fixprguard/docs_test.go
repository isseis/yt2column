//go:build test

package main

import (
	"os"
	"path/filepath"
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

// reviewerPlaceholder is the --reviewer value the command documents write: the
// automated reviewer's name is defined in _context.md alone.
const reviewerPlaceholder = "<reviewer>"

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
// what this binary accepts, and --reviewer must be the placeholder for the
// name in _context.md. Each document must also call the subcommands it uses.
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
			if reviewer, ok := inv.values[flagReviewer]; ok && reviewer != reviewerPlaceholder {
				t.Errorf("%s: fixprguard %s passes --reviewer %q, want %q", tc.rel, inv.name, reviewer, reviewerPlaceholder)
			}
		}
		for _, name := range tc.want {
			if _, ok := seen[name]; !ok {
				t.Errorf("%s has no fixprguard %s call", tc.rel, name)
			}
		}
	}
}
