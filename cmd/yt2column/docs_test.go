//go:build test

package main

import (
	"flag"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	"github.com/isseis/yt2column/internal/pricing"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/writer"
)

// Paths from cmd/yt2column to the documents these tests check.
const (
	readmePath           = "../../README.md"
	projectOverviewPath  = "../../docs/dev/project_overview.md"
	securityPath         = "../../docs/dev/security.md"
	reviewEvaluationPath = "../../docs/tasks/0010_post_generation_review/02_evaluation.md"
	pricingRatesPath     = "../../internal/pricing/rates.go"
)

// TestREADMEDocumentsFlags checks that README.md documents every flag run
// defines. The flag names come from newFlagSet, not from a list kept here, so a
// new flag fails this test until README names it.
func TestREADMEDocumentsFlags(t *testing.T) {
	doc := readDoc(t, readmePath)
	table := findTable(doc, "Flag")
	if table == nil {
		t.Fatal("README has no flags table")
	}
	fs, _ := newFlagSet()
	fs.VisitAll(func(f *flag.Flag) {
		if !tableFlagContains(table, "--"+f.Name) {
			t.Errorf("flags table does not document --%s", f.Name)
		}
	})
}

// TestSlackDocsContract pins the machine-checkable contract values of webhook
// publishing: the test environment variables, the make targets, and the
// per-message limit. The values come from the publisher's exported test
// constants, not from the documents.
func TestSlackDocsContract(t *testing.T) {
	// The documents state the limit with a thousands separator, so the
	// literal below is pinned to the publisher's own exported test constant:
	// a changed limit fails this test, not only the integration size check.
	if publisher.SlackMaxMessageRunesForTest != 16383 {
		t.Fatalf("the publisher per-message limit is %d, not 16,383; update the documents and this test", publisher.SlackMaxMessageRunesForTest)
	}
	readme := readDoc(t, readmePath)
	values := []string{
		publishertestutil.SlackOptInEnv,
		publishertestutil.CLISlackOptInEnv,
		publishertestutil.WebhookURLEnv,
		publishertestutil.SlackIntegrationOptions.MakeTarget,
		publishertestutil.CLISlackIntegrationOptions.MakeTarget,
	}
	for _, doc := range []struct {
		name, text string
	}{
		{"README.md", readme},
		{"security.md", readDoc(t, securityPath)},
	} {
		for _, value := range values {
			if !strings.Contains(doc.text, value) {
				t.Errorf("%s does not mention %q", doc.name, value)
			}
		}
	}
	for _, doc := range []struct {
		name, text string
	}{
		{"README.md", readme},
		{"project_overview.md", readDoc(t, projectOverviewPath)},
	} {
		if !strings.Contains(doc.text, "16,383") {
			t.Errorf("%s does not state the per-message limit 16,383", doc.name)
		}
	}
}

// TestClaudeDocsContract pins the machine-checkable contract values of the
// Claude provider: the integration test's environment variables and make target.
// The environment variables come from the claude test package's exported
// constants, not from the documents.
func TestClaudeDocsContract(t *testing.T) {
	values := []string{
		claudetestutil.APIKeyEnv,
		claudetestutil.WorkspaceIDEnv,
		claudetestutil.OptInEnv,
	}
	for _, doc := range []struct {
		name, text string
	}{
		{"README.md", readDoc(t, readmePath)},
		{"security.md", readDoc(t, securityPath)},
	} {
		for _, value := range values {
			if !strings.Contains(doc.text, value) {
				t.Errorf("%s does not mention %q", doc.name, value)
			}
		}
	}
}

// TestSectionByHeading pins sectionByHeading's boundaries: the heading line is
// excluded, the section ends before the next heading of the same or a higher
// level, a missing heading returns the empty string, and a '#' not followed by
// whitespace is not a heading.
func TestSectionByHeading(t *testing.T) {
	const doc = `# Top
top body
## Middle
middle body
### Deep
deep body
#### Deeper
deeper body
## Middle two
middle two body
#109 note
note body
###
hash body
# Top two
top two body`
	for _, tc := range []struct {
		name    string
		heading string
		want    string
	}{
		{
			name:    "level two ends before the next level two",
			heading: "## Middle",
			want:    "middle body\n### Deep\ndeep body\n#### Deeper\ndeeper body\n",
		},
		{
			name:    "level three includes a lower level four",
			heading: "### Deep",
			want:    "deep body\n#### Deeper\ndeeper body\n",
		},
		{
			name:    "level four keeps only its own body",
			heading: "#### Deeper",
			want:    "deeper body\n",
		},
		{
			name:    "level one ends before the next level one",
			heading: "# Top",
			want:    "top body\n## Middle\nmiddle body\n### Deep\ndeep body\n#### Deeper\ndeeper body\n## Middle two\nmiddle two body\n#109 note\nnote body\n###\nhash body\n",
		},
		{
			name:    "missing heading yields the empty string",
			heading: "## Absent",
			want:    "",
		},
		{
			name:    "hash without following whitespace is not a heading",
			heading: "#109 note",
			want:    "",
		},
		{
			name:    "bare hashes are not a heading",
			heading: "###",
			want:    "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sectionByHeading(doc, tc.heading); got != tc.want {
				t.Errorf("sectionByHeading(doc, %q) = %q, want %q", tc.heading, got, tc.want)
			}
		})
	}
}

// readDoc reads a repository document, failing the test on error.
func readDoc(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// markdownTables returns every Markdown table in doc. Each table is a slice of
// rows, and each row a slice of trimmed cells. A separator row is dropped, and
// a blank or non-table line ends a table.
func markdownTables(doc string) [][][]string {
	var tables [][][]string
	var rows [][]string
	flush := func() {
		if len(rows) > 0 {
			tables = append(tables, rows)
			rows = nil
		}
	}
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			cells := splitTableRow(trimmed)
			if isTableSeparator(cells) {
				continue
			}
			rows = append(rows, cells)
			continue
		}
		flush()
	}
	flush()
	return tables
}

// splitTableRow splits one Markdown table line into trimmed cells. Cells must
// not contain an escaped pipe, which these documents do not use.
func splitTableRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// isTableSeparator reports whether cells are the "---" row under a header.
func isTableSeparator(cells []string) bool {
	dash := false
	for _, cell := range cells {
		if strings.Contains(cell, "-") {
			dash = true
		}
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return dash
}

// findTable returns the first table whose header row contains header.
func findTable(doc, header string) [][]string {
	for _, table := range markdownTables(doc) {
		if len(table) == 0 {
			continue
		}
		for _, cell := range table[0] {
			if strings.EqualFold(stripBackticks(cell), header) {
				return table
			}
		}
	}
	return nil
}

// tableRow returns the first body row whose first cell equals first, ignoring
// backticks.
func tableRow(table [][]string, first string) []string {
	for _, row := range table[1:] {
		if len(row) > 0 && stripBackticks(row[0]) == first {
			return row
		}
	}
	return nil
}

// tableRowsWithFirstCell returns every body row whose first cell equals first,
// ignoring backticks. It is tableRow for a table that can hold more than one row
// per key, such as the review-log item table.
func tableRowsWithFirstCell(table [][]string, first string) [][]string {
	var rows [][]string
	for _, row := range table[1:] {
		if len(row) > 0 && stripBackticks(row[0]) == first {
			rows = append(rows, row)
		}
	}
	return rows
}

// tableFlagContains reports whether the flags table documents want, a flag name
// such as --out. It splits the first cell (the flag column) into tokens and
// requires an exact match, so a documented --output does not satisfy --out and a
// flag named in another row's description does not count as documented.
func tableFlagContains(table [][]string, want string) bool {
	for _, row := range table[1:] {
		if len(row) == 0 {
			continue
		}
		tokens := strings.FieldsFunc(row[0], func(r rune) bool {
			return r == '`' || r == ',' || r == ' ' || r == '\t'
		})
		if slices.Contains(tokens, want) {
			return true
		}
	}
	return false
}

// stripBackticks removes the code-span backticks around a table cell.
func stripBackticks(s string) string {
	return strings.Trim(s, "`")
}

// headingLevel reports the level of a Markdown heading line: the number of
// leading '#' characters when the line starts with one or more '#' followed by
// a space or tab. It returns 0 for any other line, so a line such as
// "#109 note" (no whitespace after '#') is not a heading.
func headingLevel(line string) int {
	line = strings.TrimRight(line, "\n")
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n == len(line) {
		return 0
	}
	switch line[n] {
	case ' ', '\t':
		return n
	}
	return 0
}

// sectionByHeading returns the text under the first heading line whose trimmed
// text equals heading, up to the next heading line of the same or a higher
// level. The matching heading line is not included; it returns the empty string
// when no line matches. The caller passes the whole heading line (for example
// "## Heading"), so the helper works for any level.
func sectionByHeading(doc, heading string) string {
	want := strings.TrimSpace(heading)
	level := 0
	inSection := false
	var b strings.Builder
	for line := range strings.Lines(doc) {
		if l := headingLevel(line); l > 0 {
			if !inSection {
				if strings.TrimSpace(line) == want {
					inSection = true
					level = l
				}
				continue
			}
			if l <= level {
				break
			}
		}
		if inSection {
			b.WriteString(line)
		}
	}
	return b.String()
}

// containsToken reports whether word appears in doc bounded by characters that
// are not letters, digits, underscores, or hyphens, so "known" is not satisfied
// by "unknown" or "recorded-unknown", and "run" is not satisfied by "runs".
func containsToken(doc, word string) bool {
	pattern := `(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_-]|$)`
	return regexp.MustCompile(pattern).MatchString(doc)
}

// checkExampleLines checks the example lines in section that use prefix: the
// first three words after the prefix must be in allowed. A line whose first word
// is a placeholder (one starting with "<") shows the line's shape, not an
// example, and is skipped. what names the section in the failure messages, and
// the call fails t when the section has no example line.
func checkExampleLines(t *testing.T, section, prefix string, allowed map[string]bool, what string) {
	t.Helper()
	examples := 0
	for line := range strings.SplitSeq(section, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), prefix)
		if !ok {
			continue
		}
		words := strings.Fields(rest)
		if len(words) > 0 && strings.HasPrefix(words[0], "<") {
			continue
		}
		examples++
		if len(words) < 3 {
			t.Errorf("%s example line %q has fewer than three words after the prefix", what, line)
			continue
		}
		for i := range 3 {
			if !allowed[words[i]] {
				t.Errorf("%s example line %q has the unknown word %q at position %d", what, line, words[i], i+1)
			}
		}
	}
	if examples == 0 {
		t.Errorf("%s has no example line", what)
	}
}

// TestTokenUsageDocsContract checks the token-usage documents: the README
// section describes the line's words and every reason llm.Usage can produce,
// project_overview.md and security.md describe the usage handling, and
// package_reference.md lists the new types.
//
// The behavior these documents describe is pinned by running tests, not by this
// test: the line format by cmd/yt2column's TestRunTokenLinesGoldenFormat, and
// the usage validation by the adapters' TestGenerateUsageUnknown and
// internal/artifact's TestParseRecordRejectsUsage.
func TestTokenUsageDocsContract(t *testing.T) {
	readme := readDoc(t, readmePath)
	section := sectionByHeading(readme, "## Token usage")
	if section == "" {
		t.Fatal("README has no Token usage section")
	}
	// Every format word run.go defines, plus the three stage names. The
	// value-carrying fields are matched as substrings; the fixed words are
	// matched as whole tokens, so "known" is not satisfied by "unknown" and
	// "run" is not satisfied by "runs".
	for _, word := range []string{tokensInput, tokensCacheRead, tokensCacheWrite, tokensOutput, tokensReason, tokensModel} {
		if !strings.Contains(section, word) {
			t.Errorf("README Token usage section does not contain %q", word)
		}
	}
	for _, word := range []string{
		tokensPrefix, tokensRun, tokensReused, tokensKnown, tokensUnknown, tokensNoModel,
		writer.StageGeneration.String(), writer.StageShorten.String(), writer.StageReview.String(),
	} {
		if !containsToken(section, word) {
			t.Errorf("README Token usage section does not contain the token %q", word)
		}
	}
	// Every member-less reason llm.Usage can produce, obtained from the code so
	// the test does not spell them. The README also documents the member-named
	// forms (`usage.<member>:<rule>`) in prose.
	reasons := []llm.Usage{
		llm.UnknownUsage(llm.RuleNotRead, ""),
		llm.UnknownUsage(llm.RuleRecordedUnknown, ""),
		llm.UnknownUsage(llm.RuleMissing, ""),
		llm.UnknownUsage(llm.RuleDuplicate, ""),
		llm.UnknownUsage(llm.RuleNotObject, ""),
		llm.UnknownUsage(llm.RuleNotNonNegativeInteger, ""),
		llm.UnknownUsage(llm.UnknownRule(99), ""),
	}
	for _, usage := range reasons {
		if reason := usage.UnknownReason(); !strings.Contains(section, reason) {
			t.Errorf("README Token usage section does not contain the reason %q", reason)
		}
	}
	// Each example line's first three words after the prefix are known words.
	allowed := map[string]bool{
		tokensRun: true, tokensReused: true, tokensKnown: true, tokensUnknown: true,
	}
	for _, stage := range []writer.Stage{writer.StageGeneration, writer.StageShorten, writer.StageReview} {
		allowed[stage.String()] = true
	}
	checkExampleLines(t, section, tokensPrefix, allowed, "README Token usage section")
}

// TestCostDocsContract checks the cost documents: the README's Estimated cost
// section uses every word run.go's cost format defines, its example lines use
// only those words and the stage names in the fixed positions, and it names the
// heading word of the Slack footnote's cost; the rate-update procedure
// names the rate table and the hand-calculated tests, is linked from CLAUDE.md,
// and is pointed at from the rate table's leading comment; project_overview.md
// and security.md describe the cost line and the artifact call_time.
//
// The behavior these documents describe is pinned by running tests, not by this
// test: the line format by cmd/yt2column's TestRunCostLinesGoldenFormat, and the
// call_time validation by internal/artifact's TestParseRecordRejectsCallTime.
func TestCostDocsContract(t *testing.T) {
	readme := readDoc(t, readmePath)
	section := sectionByHeading(readme, "## Estimated cost")
	if section == "" {
		t.Fatal("README has no Estimated cost section")
	}
	// The value-carrying fields of the cost lines, taken from run.go so the
	// test does not spell them.
	for _, word := range []string{costPrefix, costTotalPrefix, costUSD, costReason, costAt} {
		if !strings.Contains(section, word) {
			t.Errorf("README Estimated cost section does not contain %q", word)
		}
	}
	// The fixed words of the lines, matched as whole tokens so "known" is not
	// satisfied by "unknown". The two total-kind words are checked in their
	// format position below, because both also appear in the section's prose.
	for _, word := range []string{
		costKnown, costUnknown, costNoTime, costInvalid,
		writer.StageGeneration.String(), writer.StageShorten.String(), writer.StageReview.String(),
	} {
		if !containsToken(section, word) {
			t.Errorf("README Estimated cost section does not contain the token %q", word)
		}
	}
	// Each total kind must appear on a cost-total line, not only in prose.
	for _, kind := range []string{costTotalRun, costTotalArticle} {
		if !strings.Contains(section, costTotalPrefix+kind) {
			t.Errorf("README Estimated cost section has no %q total line", costTotalPrefix+kind)
		}
	}
	// Every rate basis and unknown reason the word mapping returns, obtained
	// from the code so the test does not spell them. The zero value and a value
	// outside the enumeration map to costInvalid, which the token loop above
	// already checks, so this only checks the mapping-specific words.
	for _, basis := range []pricing.RateBasis{
		pricing.BasisUnset, pricing.BasisFlat, pricing.BasisOffPeak, pricing.BasisPeak,
		pricing.BasisShortPrompt, pricing.BasisLongPrompt, pricing.RateBasis(99),
	} {
		if word := costBasisWord(basis); word != costInvalid && !containsToken(section, word) {
			t.Errorf("README Estimated cost section does not contain the rate-basis word %q", word)
		}
	}
	for _, reason := range []pricing.UnknownReason{
		pricing.ReasonUnset, pricing.ReasonUsageUnknown, pricing.ReasonNoModel,
		pricing.ReasonUnpricedModel, pricing.ReasonInvalid, pricing.UnknownReason(99),
	} {
		if word := costReasonWord(reason); word != costInvalid && !containsToken(section, word) {
			t.Errorf("README Estimated cost section does not contain the reason word %q", word)
		}
	}
	// Each example line's first three words after the prefix are the stage, the
	// origin, and the cost state.
	allowed := map[string]bool{
		tokensRun: true, tokensReused: true, costKnown: true, costUnknown: true,
	}
	for _, stage := range []writer.Stage{writer.StageGeneration, writer.StageShorten, writer.StageReview} {
		allowed[stage.String()] = true
	}
	checkExampleLines(t, section, costPrefix, allowed, "README Estimated cost section")

	overview := readDoc(t, projectOverviewPath)
	if !strings.Contains(overview, costPrefix) {
		t.Errorf("project_overview.md does not contain %q", costPrefix)
	}

	rates := readDoc(t, pricingRatesPath)
	head, _, _ := strings.Cut(rates, "\npackage ")
	if !strings.Contains(head, "pricing_update.md") {
		t.Error("internal/pricing/rates.go's leading comment does not point at pricing_update.md")
	}
}
