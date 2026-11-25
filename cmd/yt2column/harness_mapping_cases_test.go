//go:build test

package main

import (
	"strings"
	"testing"
)

// review-records.sh and high-review-decision.sh each carry their own copy of
// the "### Harness model mapping" parser (parse_harness_mapping) and of the
// non-regex pattern match (matches_pattern), because the scripts are run from
// origin/main one file at a time. The cases below are the one list both
// scripts' tests run, so a grammar or matching change made in one copy only
// fails the other script's test.

// mappingValidTable is a mapping section both scripts accept: it has a
// Claude Code high row with Launch agent (high-review-decision.sh needs one)
// and both cross-check and high patterns (review-records.sh needs both).
const mappingValidTable = "### Harness model mapping\n\n" +
	"| Harness | Role | Launch | Model | Verification pattern |\n" +
	"|---|---|---|---|---|\n" +
	"| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |\n" +
	"| `Claude Code` | `standard` | `agent` | `sonnet` | `claude-sonnet-*` |\n" +
	"| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |\n" +
	"| `Antigravity` | `cross-check` | `swarm` | `claude-opus-5-5-medium` | `claude-opus-5-5-medium` |\n" +
	"| `OpenCode` | `low` | `none` | `DeepSeek` | `-` |\n\n"

// mappingGrammarCase is a _context.md both scripts must reject, with the
// diagnostic both must write to standard error.
type mappingGrammarCase struct {
	name       string
	ctx        string
	wantStderr string
}

// mappingGrammarCases returns the malformed _context.md cases, each built from
// realContext with one change.
func mappingGrammarCases(t *testing.T, realContext string) []mappingGrammarCase {
	t.Helper()
	table := func(oldText, newText string) string {
		return replaceMappingSection(t, realContext, replaceContextRow(t, mappingValidTable, oldText, newText))
	}
	section := func(s string) string { return replaceMappingSection(t, realContext, s) }
	const header = "| Harness | Role | Launch | Model | Verification pattern |\n"
	const separator = "|---|---|---|---|---|\n"
	const crossCheckPattern = "`claude-fable-*`"
	return []mappingGrammarCase{
		{"missing _context.md", "", "cannot read origin/main:.claude/commands/_context.md"},
		{"missing mapping section", section(""), "missing Harness model mapping section"},
		{"duplicate mapping section", section(mappingValidTable + mappingValidTable), "duplicate Harness model mapping section"},
		{"unclosed fence in _context.md", realContext + "\n```text\nunclosed\n", "unclosed code fence in _context.md"},
		{"empty table with zero data rows", section("### Harness model mapping\n\n" + header + separator + "\n"), "missing or empty Harness model mapping table"},
		{"wrong header", table(header, "| Harness | Role | Launch | Model | Pattern |\n"), "invalid Harness model mapping header"},
		{"wrong separator", table(separator, "|---|---|---|---|\n"), "invalid Harness model mapping separator"},
		{"wrong column count", table("| `OpenCode` | `low` | `none` | `DeepSeek` | `-` |", "| `OpenCode` | `low` | `none` | `DeepSeek` |"), "must have 5 columns"},
		{"indented table row", table(header, " "+header), "indented table row"},
		{"unquoted cell", table("`fable`", "fable"), "single code span"},
		{"multiple tables", section(mappingValidTable + "prose\n\n" + header + "\n"), "multiple tables"},
		{"unknown Harness", table("`OpenCode`", "`OtherHarness`"), "unknown Harness"},
		{"unknown Role", table("`low`", "`ultra`"), "unknown Role"},
		{"unknown Launch", table("`none`", "`cli`"), "unknown Launch"},
		{"whitespace-only Model", table("`DeepSeek`", "`   `"), "invalid Model"},
		{"duplicate Harness Role", section(strings.TrimSuffix(mappingValidTable, "\n") + "| `Claude Code` | `cross-check` | `agent` | `fable2` | `claude-fable-2-*` |\n\n"), "duplicate (Harness, Role)"},
		{"none launch with non-dash pattern", table("| `-` |", "| `deepseek` |"), "Launch none row must have Verification pattern -"},
		{"agent launch with dash pattern", table(crossCheckPattern, "`-`"), "cannot have Verification pattern -"},
		{"swarm launch with dash pattern", table("| `claude-opus-5-5-medium` | `claude-opus-5-5-medium` |", "| `claude-opus-5-5-medium` | `-` |"), "cannot have Verification pattern -"},
		{"65-char literal pattern", table(crossCheckPattern, "`"+strings.Repeat("a", 65)+"`"), "invalid literal Verification pattern"},
		{"64-char prefix plus star", table(crossCheckPattern, "`"+strings.Repeat("a", 64)+"*`"), "invalid prefix Verification pattern"},
		{"regex dot-star metacharacter", table(crossCheckPattern, "`.*`"), "invalid prefix Verification pattern"},
		{"regex char class metacharacter", table(crossCheckPattern, "`claude-[a-z]+`"), "invalid literal Verification pattern"},
		{"regex alternation pipe forbidden", table(crossCheckPattern, "`claude-(a|b)`"), "forbidden character"},
		{"regex anchors metacharacter", table(crossCheckPattern, "`^claude-fable$`"), "invalid literal Verification pattern"},
		{"regex grouping metacharacter", table(crossCheckPattern, "`claude-(ab)`"), "invalid literal Verification pattern"},
		{"whitespace in pattern", table(crossCheckPattern, "`claude fable`"), "invalid literal Verification pattern"},
		{"mid-wildcard", table(crossCheckPattern, "`claude*fable`"), "invalid literal Verification pattern"},
		{"broken uncollected role row fails whole table", table("`claude-sonnet-*`", "`claude-[0-9]+`"), "invalid literal Verification pattern"},
	}
}

// mappingMatchCase is a Verification pattern and a model ID; each script's
// test puts the pattern in the Claude Code row of the role it reads and
// checks that the script accepts the model exactly when match is true.
type mappingMatchCase struct {
	name    string
	pattern string
	model   string
	match   bool
}

func mappingMatchCases() []mappingMatchCase {
	maxLiteral := strings.Repeat("a", 64)
	maxPrefix := strings.Repeat("b", 63)
	return []mappingMatchCase{
		{"48-char suffix accepted", "model-*", "model-" + strings.Repeat("a", 48), true},
		{"49-char suffix rejected", "model-*", "model-" + strings.Repeat("a", 49), false},
		{"empty suffix rejected", "model-*", "model-", false},
		{"uppercase suffix rejected", "model-*", "model-V4", false},
		{"underscore suffix rejected", "model-*", "model-4_7", false},
		{"space in model id rejected", "model-*", "model-4 7", false},
		{"unregistered model rejected", "model-*", "other-model-4", false},
		{"prefix dot literal matches", "v1.0-*", "v1.0-a", true},
		{"prefix dot is not regex wildcard", "v1.0-*", "v1X0-a", false},
		{"exact dot literal matches", "v1.0", "v1.0", true},
		{"exact dot is not regex wildcard", "v1.0", "v1X0", false},
		{"64-char literal pattern accepted", maxLiteral, maxLiteral, true},
		{"63-char prefix plus star accepted", maxPrefix + "*", maxPrefix + "x", true},
	}
}
