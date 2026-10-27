//go:build test

package main

import (
	"flag"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/pricing"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/writer"
)

// Paths from cmd/yt2column to the documents these tests check.
const (
	readmePath            = "../../README.md"
	projectOverviewPath   = "../../docs/dev/project_overview.md"
	securityPath          = "../../docs/dev/security.md"
	productionProfilePath = "../../docs/dev/production_profile.md"
	promptsReadmePath     = "../../prompts/README.md"
	reviewEvaluationPath  = "../../docs/tasks/0010_post_generation_review/02_evaluation.md"
	pricingRatesPath      = "../../internal/pricing/rates.go"
)

// The headings test (a) and test (b) read.
const (
	productionProfileSection = "## 本番の構成"
	quickStartHeading        = "## Quick start"
	recommendedConfigHeading = "### Recommended configuration"
)

// markdownLinkRE matches an inline Markdown link and captures its text and
// target. These documents use no reference links, and a link text or target
// holds no nested bracket or closing parenthesis.
var markdownLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

// markdownLinkTargets returns the target of every inline Markdown link outside
// a fenced code block in text, in order. A link-like string inside an example
// is not a link.
func markdownLinkTargets(text string) []string {
	var targets []string
	for _, line := range scanDocLines(slices.Collect(strings.Lines(text))) {
		if line.block {
			continue
		}
		for _, match := range markdownLinkRE.FindAllStringSubmatch(line.text, -1) {
			targets = append(targets, match[2])
		}
	}
	return targets
}

// anchorFromHeading returns the anchor GitHub derives from a heading line:
// lowercase, with markup punctuation removed, each ASCII space turned into a
// hyphen, and letters, numbers, marks, connector punctuation, and hyphens kept.
// Any other character, a non-ASCII space included, is dropped.
func anchorFromHeading(heading string) string {
	text := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(heading), "#"))
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || unicode.In(r, unicode.Pc):
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// headingsOutsideBlocks returns the heading lines of doc that lie outside a
// fenced code block, in order.
func headingsOutsideBlocks(doc string) []string {
	var headings []string
	for _, line := range scanDocLines(slices.Collect(strings.Lines(doc))) {
		if !line.block && headingLevel(line.text) > 0 {
			headings = append(headings, line.text)
		}
	}
	return headings
}

// headingAnchors returns the set of anchors the heading lines of doc outside
// code blocks produce. It fails the test when two headings share an anchor:
// the suffix GitHub adds to a repeated heading is not modelled.
func headingAnchors(t *testing.T, docName, doc string) map[string]struct{} {
	t.Helper()
	anchors := make(map[string]struct{})
	for _, heading := range headingsOutsideBlocks(doc) {
		anchor := anchorFromHeading(heading)
		if _, seen := anchors[anchor]; seen {
			t.Errorf("%s has two headings with the anchor %q, which this helper does not model", docName, anchor)
		}
		anchors[anchor] = struct{}{}
	}
	return anchors
}

// sectionOutsideBlocks returns the lines after the first heading line equal to
// heading and outside a fenced code block, up to the next heading line outside
// a block whose level is the same or higher. It returns the empty string when
// no such heading line exists.
func sectionOutsideBlocks(doc, heading string) string {
	scanned := scanDocLines(slices.Collect(strings.Lines(doc)))
	want := strings.TrimSpace(heading)
	for i, line := range scanned {
		if line.block || strings.TrimSpace(line.text) != want {
			continue
		}
		level := headingLevel(line.text)
		var b strings.Builder
		for _, rest := range scanned[i+1:] {
			if l := headingLevel(rest.text); !rest.block && l > 0 && l <= level {
				break
			}
			b.WriteString(rest.text)
			b.WriteString("\n")
		}
		return b.String()
	}
	return ""
}

// numberedItemRE matches the first line of a top-level numbered list item.
var numberedItemRE = regexp.MustCompile(`^\d+\.\s`)

// numberedItems returns the top-level numbered list items of section. An item
// starts at a line outside a code block that begins with a number and a period
// at column 0, and continues to the next item or the next blank line.
func numberedItems(section string) []string {
	var items, current []string
	flush := func() {
		if len(current) > 0 {
			items = append(items, strings.Join(current, "\n"))
			current = nil
		}
	}
	for _, line := range scanDocLines(slices.Collect(strings.Lines(section))) {
		switch {
		case line.block:
			if current != nil {
				current = append(current, line.text)
			}
		case numberedItemRE.MatchString(line.text):
			flush()
			current = []string{line.text}
		case strings.TrimSpace(line.text) == "":
			flush()
		case current != nil:
			current = append(current, line.text)
		}
	}
	flush()
	return items
}

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
		requireMentions(t, doc.name, doc.text, values)
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

// requireMentions fails the test for each value that text does not contain.
func requireMentions(t *testing.T, docName, text string, values []string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(text, value) {
			t.Errorf("%s does not mention %q", docName, value)
		}
	}
}

// TestClaudeDocsContract pins the machine-checkable contract values of the
// Claude provider: the integration test's environment variables. They come
// from the claude test package's exported constants, not from the documents.
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
		requireMentions(t, doc.name, doc.text, values)
	}
}

// TestDeepSeekDocsContract pins the machine-checkable contract values of the
// DeepSeek integration tests: the key they read, the opt-in that arms each
// one, and the model variable. Every integration-test section of README.md
// names its own values, so a section left behind by a rename fails. The values
// come from the deepseek and publisher test packages' exported constants, not
// from the document.
func TestDeepSeekDocsContract(t *testing.T) {
	readme := readDoc(t, readmePath)
	for _, section := range []struct {
		name    string
		heading string
		values  []string
	}{
		{
			name:    "deepseek",
			heading: "### DeepSeek integration test",
			values:  []string{deepseektestutil.APIKeyEnv, deepseektestutil.DeepSeekOptInEnv, deepseektestutil.ModelEnv},
		},
		{
			name:    "cli",
			heading: "### CLI integration test",
			values:  []string{deepseektestutil.APIKeyEnv, deepseektestutil.CLIOptInEnv, deepseektestutil.ModelEnv},
		},
		{
			name:    "cli_slack",
			heading: "### CLI webhook integration test",
			values:  []string{deepseektestutil.APIKeyEnv, publishertestutil.CLISlackOptInEnv, deepseektestutil.ModelEnv},
		},
	} {
		t.Run(section.name, func(t *testing.T) {
			text := sectionOutsideBlocks(readme, section.heading)
			if text == "" {
				t.Fatalf("README.md has no %q section", section.heading)
			}
			requireMentions(t, "README.md "+section.heading, text, section.values)
		})
	}
}

// TestLLMDefaultsDocsContract checks that the configuration tables of
// README.md and project_overview.md state the code's LLM default: each table
// has exactly one row for each LLM variable, and the first code span of the
// "when unset" cell of the YT2COLUMN_LLM row is the value the config package
// applies. The YT2COLUMN_REVIEW_LLM row inherits the generation selection,
// which no code constant spells, so only its presence is checked. A stale
// value written as a code span in front of the new one fails; prose in front
// of it, and the rest of the cell, are not checked here but read in review.
func TestLLMDefaultsDocsContract(t *testing.T) {
	want := []struct{ name, value string }{
		{config.LLMEnvForTest(), config.DefaultLLMValueForTest()},
		{config.ReviewLLMEnvForTest(), ""}, // presence only
	}
	for _, doc := range []struct {
		name, path, nameHeader, unsetHeader string
	}{
		{"README.md", readmePath, "Variable", "When unset"},
		{"project_overview.md", projectOverviewPath, "変数", "未設定のとき"},
	} {
		t.Run(doc.name, func(t *testing.T) {
			table := findTable(readDoc(t, doc.path), doc.nameHeader)
			if table == nil {
				t.Fatalf("%s has no table with a %q column", doc.name, doc.nameHeader)
			}
			column := slices.Index(table[0], doc.unsetHeader)
			if column < 0 {
				t.Fatalf("%s: the %q table has no %q column", doc.name, doc.nameHeader, doc.unsetHeader)
			}
			for _, v := range want {
				// A second row for the same variable would go unchecked, so
				// exactly one is required.
				rows := tableRowsWithFirstCell(table, v.name)
				if len(rows) != 1 || len(rows[0]) <= column {
					t.Errorf("%s: %d rows for %s, want exactly one with a %q cell", doc.name, len(rows), v.name, doc.unsetHeader)
					continue
				}
				if v.value == "" {
					continue
				}
				cell := rows[0][column]
				value, ok := firstCodeSpan(cell)
				if !ok || value != v.value {
					t.Errorf("%s: %s %q cell = %q, want its first code span to be %q", doc.name, v.name, doc.unsetHeader, cell, v.value)
				}
			}
		})
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

// TestScanDocLines checks the flags of the fence scan: a fence line and the
// lines it encloses are in a block, and a '#' line inside a block stays in it.
// TestFencedBlocks and TestFirstCodeBlockAfter observe the same scan through
// its consumers, so they cannot tell a wrong flag from a wrong consumer.
func TestScanDocLines(t *testing.T) {
	lines := []string{"intro\n", "```sh\n", "# not a heading\n", "export A=1\n", "```not-closed\n", "```\n", "## Heading\n", "```text\n", "tail"}
	want := []docLine{
		{text: "intro", fence: false, block: false},
		{text: "```sh", fence: true, block: true},
		{text: "# not a heading", fence: false, block: true},
		{text: "export A=1", fence: false, block: true},
		{text: "```not-closed", fence: false, block: true},
		{text: "```", fence: true, block: true},
		{text: "## Heading", fence: false, block: false},
		{text: "```text", fence: true, block: true},
		{text: "tail", fence: false, block: true},
	}
	if got := scanDocLines(lines); !slices.Equal(got, want) {
		t.Errorf("scanDocLines = %#v, want %#v", got, want)
	}
}

// TestFencedBlocks checks the block extraction: a closing fence ends a block,
// an indented fence is a fence, blocks come back in order with their info
// string, and a block left unclosed at the end is reported as not closed.
func TestFencedBlocks(t *testing.T) {
	doc := "intro\n```sh\nexport A=1\n```\ntext\n  ```text\nx\n  ```\n```json\n{}\n"
	want := []fencedBlock{
		{info: "sh", lines: []string{"export A=1"}, closed: true},
		{info: "text", lines: []string{"x"}, closed: true},
		{info: "json", lines: []string{"{}"}, closed: false},
	}
	if got := fencedBlocks(slices.Collect(strings.Lines(doc))); !slices.EqualFunc(got, want, func(a, b fencedBlock) bool {
		return a.info == b.info && a.closed == b.closed && slices.Equal(a.lines, b.lines)
	}) {
		t.Errorf("fencedBlocks = %#v, want %#v", got, want)
	}
}

// codeSpanRE matches one Markdown code span and captures its content. These
// documents do not nest code spans.
var codeSpanRE = regexp.MustCompile("`([^`]*)`")

// firstCodeSpan returns the content of the first code span in cell.
func firstCodeSpan(cell string) (string, bool) {
	match := codeSpanRE.FindStringSubmatch(cell)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// docLine is one line of a Markdown document as the code-fence scan classifies
// it: the text without its newline, whether the line is a fence, and whether
// the line is part of a fenced code block. An opening fence, the lines it
// encloses, and its closing fence are all part of the block.
type docLine struct {
	text  string
	fence bool
	block bool
}

// scanDocLines classifies lines, starting outside a code block. Passing only
// the lines after a heading therefore scans them from their first line: a
// fence opened earlier in the document does not change the result. An opening
// fence may carry an info string; a closing fence is backticks alone, so a
// line such as "```not-closed" inside a block stays part of the block.
func scanDocLines(lines []string) []docLine {
	scanned := make([]docLine, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		text := strings.TrimSuffix(line, "\n")
		trimmed := strings.TrimSpace(text)
		fence := false
		switch {
		case !inBlock && strings.HasPrefix(trimmed, "```"):
			fence, inBlock = true, true
		case inBlock && isClosingFence(trimmed):
			fence, inBlock = true, false
		}
		scanned = append(scanned, docLine{text: text, fence: fence, block: fence || inBlock})
	}
	return scanned
}

// isClosingFence reports whether line is a closing code fence: three or more
// backticks and nothing else.
func isClosingFence(line string) bool {
	return len(line) >= 3 && strings.Trim(line, "`") == ""
}

// fencedBlock is one fenced code block: the info string after its opening
// fence's backticks, its content lines without the fence lines, and whether a
// closing fence was found.
type fencedBlock struct {
	info   string
	lines  []string
	closed bool
}

// fencedBlocks returns every fenced code block in lines, in order. A block with
// no closing fence runs to the end of lines and is returned with closed false.
func fencedBlocks(lines []string) []fencedBlock {
	var blocks []fencedBlock
	var current *fencedBlock
	for _, line := range scanDocLines(lines) {
		switch {
		case line.fence && current == nil:
			current = &fencedBlock{info: strings.TrimPrefix(strings.TrimSpace(line.text), "```")}
		case line.fence:
			current.closed = true
			blocks = append(blocks, *current)
			current = nil
		case current != nil:
			current.lines = append(current.lines, line.text)
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}
	return blocks
}

// exportLineRE matches one line of a configuration block: "export", one space,
// an uppercase variable name, "=", and a value of letters, digits, ".", "_",
// "-", and "/" (the separator of an LLM variable's provider and model ID).
var exportLineRE = regexp.MustCompile(`^export ([A-Z][A-Z0-9_]*)=([A-Za-z0-9._/-]+)$`)

// checkExportBlock checks that section holds exactly one sh code block and that
// every line of it has the export-line shape, names one of want, and does not
// repeat a variable. It returns the block's values. The failure messages name
// the document, section, line, and variable, never a line's value: a secret
// written into the block by mistake must not reach the log.
func checkExportBlock(t *testing.T, docName, sectionName, section string, want []string) map[string]string {
	t.Helper()
	var shBlocks []fencedBlock
	for _, block := range fencedBlocks(slices.Collect(strings.Lines(section))) {
		if block.info == "sh" {
			shBlocks = append(shBlocks, block)
		}
	}
	if len(shBlocks) != 1 {
		t.Fatalf("%s %s: %d sh code blocks, want exactly one", docName, sectionName, len(shBlocks))
	}
	if !shBlocks[0].closed {
		t.Fatalf("%s %s: the sh code block has no closing fence; an unclosed fence runs to the end of the document", docName, sectionName)
	}
	values := make(map[string]string, len(want))
	for i, line := range shBlocks[0].lines {
		match := exportLineRE.FindStringSubmatch(line)
		if match == nil {
			t.Errorf("%s %s: block line %d is not an export line of the form export <NAME>=<value>", docName, sectionName, i+1)
			continue
		}
		name, value := match[1], match[2]
		if _, repeated := values[name]; repeated {
			t.Errorf("%s %s: block line %d repeats the variable %s", docName, sectionName, i+1, name)
			continue
		}
		values[name] = value
	}
	for _, name := range want {
		if _, ok := values[name]; !ok {
			t.Errorf("%s %s: the block does not set the variable %s", docName, sectionName, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if !slices.Contains(want, name) {
			t.Errorf("%s %s: the block sets the unexpected variable %s", docName, sectionName, name)
		}
	}
	accepted := make(map[string]string, len(want))
	for _, name := range want {
		if value, ok := values[name]; ok {
			accepted[name] = value
		}
	}
	return accepted
}

// docsProfileEnv is the closed environment test (a) loads the server
// configuration from: a dummy API key, a dummy Webhook URL under the reserved
// .invalid domain, and a short fixed HOME, because the socket path derived from
// it must fit the operating system's limit. No key of the process environment
// reaches LoadServer through it.
func docsProfileEnv() map[string]string {
	return map[string]string{
		config.AnthropicAPIKeyEnvForTest(): "docs-test-api-key",
		config.SlackWebhookEnvForTest():    "https://example.invalid/webhook",
		"HOME":                             "/tmp/yt2column-docs-test",
	}
}

// checkExportBlockMatchesDefaults loads the server configuration without the
// block's variables and with them, and fails unless both load and their
// generation and review provider, model, and effort are equal. It returns the
// configuration loaded without the block, which is the code's default profile.
func checkExportBlockMatchesDefaults(t *testing.T, docName, sectionName string, block map[string]string) config.LLMConfig {
	t.Helper()
	without, err := config.LoadServer(lookupFrom(docsProfileEnv()))
	if err != nil {
		t.Fatalf("%s %s: LoadServer on the default variables: %v", docName, sectionName, err)
	}
	env := docsProfileEnv()
	maps.Copy(env, block)
	loaded, err := config.LoadServer(lookupFrom(env))
	if err != nil {
		t.Fatalf("%s %s: LoadServer rejected the block's variables: %v", docName, sectionName, err)
	}
	// The model names are document values, so a mismatch says only whether
	// they are equal: a secret pasted into the block by mistake must not reach
	// the log.
	got, want := loaded.LLM(), without.LLM()
	modelEqual := got.Model() == want.Model()
	reviewModelEqual := got.ReviewModel() == want.ReviewModel()
	if got.Provider() != want.Provider() || !modelEqual ||
		got.ReviewProvider() != want.ReviewProvider() || !reviewModelEqual ||
		got.ClaudeEffort() != want.ClaudeEffort() {
		t.Errorf("%s %s: the block's profile differs from the code's default profile "+
			"(provider %v/%v, model equal %t, review provider %v/%v, review model equal %t, effort %v/%v)",
			docName, sectionName, got.Provider(), want.Provider(), modelEqual,
			got.ReviewProvider(), want.ReviewProvider(), reviewModelEqual,
			got.ClaudeEffort(), want.ClaudeEffort())
	}
	return without.LLM()
}

// profileStages are the rows the shared document's profile table must hold,
// in the order the article is written.
var profileStages = []string{"生成", "字数の調整", "検証・推敲"}

// productionProfileVars returns the variables that describe the production
// profile: the generation provider and model, which the review step inherits,
// and the Claude effort, which all three stages share.
func productionProfileVars() []string {
	return []string{
		config.LLMEnvForTest(),
		config.ClaudeEffortEnvForTest(),
	}
}

// tableColumn returns the index of the column whose header equals name,
// ignoring case and backticks, or -1. It matches the way findTable and
// tableRowsWithFirstCell read a header cell.
func tableColumn(header []string, name string) int {
	for i, cell := range header {
		if strings.EqualFold(stripBackticks(cell), name) {
			return i
		}
	}
	return -1
}

// checkProfileTable checks the shared document's profile table: each stage has
// exactly one row, and the first code spans of its provider and model cells,
// joined by "/" into one LLM variable value, and of its effort cell, loaded
// through LoadServer, give the default profile's values for that stage. The
// review row's value goes into YT2COLUMN_REVIEW_LLM and the other rows' into
// YT2COLUMN_LLM. The shortening stage calls the generation client, so its row
// is compared with the generation values.
func checkProfileTable(t *testing.T, section string, defaults config.LLMConfig) {
	t.Helper()
	table := findTable(section, "段階")
	if table == nil {
		t.Fatal("production_profile.md: the profile section has no table with a 段階 column")
	}
	names := []string{"プロバイダ", "モデル", "effort"}
	columns := make([]int, len(names))
	for i, name := range names {
		if columns[i] = tableColumn(table[0], name); columns[i] < 0 {
			t.Fatalf("production_profile.md: the profile table has no %s column", name)
		}
	}
	for _, row := range table[1:] {
		if len(row) > 0 && !slices.Contains(profileStages, stripBackticks(row[0])) {
			t.Errorf("production_profile.md: the profile table has a row for %q, which is not a stage of the profile", row[0])
		}
	}
	for _, stage := range profileStages {
		rows := tableRowsWithFirstCell(table, stage)
		if len(rows) != 1 {
			t.Errorf("production_profile.md: the profile table has %d rows for %q, want exactly one", len(rows), stage)
			continue
		}
		values := make([]string, len(names))
		complete := true
		for i, name := range names {
			if len(rows[0]) <= columns[i] {
				t.Errorf("production_profile.md: the %q row has %d cells, want more than %d", stage, len(rows[0]), columns[i])
				complete = false
				continue
			}
			value, ok := firstCodeSpan(rows[0][columns[i]])
			if !ok {
				t.Errorf("production_profile.md: the %q row's %s cell has no code span", stage, name)
				complete = false
				continue
			}
			values[i] = value
		}
		if !complete {
			continue
		}
		review := stage == "検証・推敲"
		llmEnv := config.LLMEnvForTest()
		if review {
			llmEnv = config.ReviewLLMEnvForTest()
		}
		env := docsProfileEnv()
		env[llmEnv] = values[0] + "/" + values[1]
		env[config.ClaudeEffortEnvForTest()] = values[2]
		cfg, err := config.LoadServer(lookupFrom(env))
		if err != nil {
			t.Errorf("production_profile.md: LoadServer rejects the %q row's values: %v", stage, err)
			continue
		}
		got := cfg.LLM()
		gotProvider, gotModel := got.Provider(), got.Model()
		wantProvider, wantModel := defaults.Provider(), defaults.Model()
		if review {
			gotProvider, gotModel = got.ReviewProvider(), got.ReviewModel()
			wantProvider, wantModel = defaults.ReviewProvider(), defaults.ReviewModel()
		}
		// The model name comes from the document, so the diagnostic says only
		// whether it matches: a secret pasted into the cell by mistake must not
		// reach the log.
		modelEqual := gotModel == wantModel
		if gotProvider != wantProvider || !modelEqual || got.ClaudeEffort() != defaults.ClaudeEffort() {
			t.Errorf("production_profile.md: the %q row gives provider %v, model equal %t, effort %v; the default profile gives provider %v, effort %v",
				stage, gotProvider, modelEqual, got.ClaudeEffort(), wantProvider, defaults.ClaudeEffort())
		}
	}
}

// TestProductionProfileDocsContract checks the shared production-profile
// document's profile section and the README's recommended configuration
// section: each configuration block holds exactly the variables of the
// production profile and loads to the code's default profile, the README's
// subsection lies inside Configuration, and the shared document's table names
// the same profile for each stage.
func TestProductionProfileDocsContract(t *testing.T) {
	want := productionProfileVars()

	doc := readDoc(t, productionProfilePath)
	section := sectionOutsideBlocks(doc, productionProfileSection)
	if section == "" {
		t.Fatalf("production_profile.md has no %q section", productionProfileSection)
	}
	values := checkExportBlock(t, "production_profile.md", productionProfileSection, section, want)
	defaults := checkExportBlockMatchesDefaults(t, "production_profile.md", productionProfileSection, values)
	checkProfileTable(t, section, defaults)

	readme := readDoc(t, readmePath)
	recommended := sectionOutsideBlocks(readme, recommendedConfigHeading)
	if recommended == "" {
		t.Fatalf("README has no %q section", recommendedConfigHeading)
	}
	configSection := sectionOutsideBlocks(readme, "## Configuration")
	inConfiguration := false
	for _, line := range scanDocLines(slices.Collect(strings.Lines(configSection))) {
		if !line.block && strings.TrimSpace(line.text) == recommendedConfigHeading {
			inConfiguration = true
		}
	}
	if !inConfiguration {
		t.Errorf("README: %q is not inside the Configuration section", recommendedConfigHeading)
	}
	readmeValues := checkExportBlock(t, "README.md", recommendedConfigHeading, recommended, want)
	checkExportBlockMatchesDefaults(t, "README.md", recommendedConfigHeading, readmeValues)
}

// migrationHeading is the README section that shows how to move from the
// retired LLM variables to the new ones.
const migrationHeading = "### Migrating from the retired LLM variables"

// TestMigrationDocsContract checks the README's migration example: the
// section holds one text block of the retired generation variables and one sh
// block setting YT2COLUMN_LLM alone, and the sh block loads, through
// LoadServer, to the provider and model the retired block named, for both the
// generation and the review step. The retired names come from the config
// package, whose table orders the generation provider variable first and the
// generation model variable second.
func TestMigrationDocsContract(t *testing.T) {
	section := sectionOutsideBlocks(readDoc(t, readmePath), migrationHeading)
	if section == "" {
		t.Fatalf("README has no %q section", migrationHeading)
	}
	after := checkExportBlock(t, "README.md", migrationHeading, section, []string{config.LLMEnvForTest()})

	retired := config.RetiredLLMVarsForTest()
	providerVar, modelVar := retired[0], retired[1]
	var before map[string]string
	for _, block := range fencedBlocks(slices.Collect(strings.Lines(section))) {
		if block.info != "text" {
			continue
		}
		if before != nil {
			t.Fatalf("README %s: more than one text block, want exactly one", migrationHeading)
		}
		before = map[string]string{}
		for i, line := range block.lines {
			name, value, ok := strings.Cut(strings.TrimRight(line, "\n"), "=")
			if !ok || !slices.Contains(retired, name) {
				t.Errorf("README %s: text block line %d is not <retired variable>=<value>", migrationHeading, i+1)
				continue
			}
			before[name] = value
		}
	}
	if before == nil {
		t.Fatalf("README %s: no text block of the retired variables", migrationHeading)
	}
	if len(before) != 2 || before[providerVar] == "" || before[modelVar] == "" {
		t.Fatalf("README %s: the text block sets %d variables, want exactly %s and %s",
			migrationHeading, len(before), providerVar, modelVar)
	}

	load := func(value string) config.LLMConfig {
		t.Helper()
		env := docsProfileEnv()
		env[config.DeepSeekAPIKeyEnvForTest()] = "docs-test-api-key"
		env[config.LLMEnvForTest()] = value
		cfg, err := config.LoadServer(lookupFrom(env))
		if err != nil {
			t.Fatalf("README %s: LoadServer rejects a value of the example: %v", migrationHeading, err)
		}
		return cfg.LLM()
	}
	got := load(after[config.LLMEnvForTest()])
	want := load(before[providerVar] + "/" + before[modelVar])
	// The model names come from the document, so the diagnostic says only
	// whether they are equal: a secret pasted into the block by mistake must
	// not reach the log.
	if got.Provider() != want.Provider() || got.Model() != want.Model() ||
		got.ReviewProvider() != want.ReviewProvider() || got.ReviewModel() != want.ReviewModel() {
		t.Errorf("README %s: the new setting gives provider %v/%v (review %v/%v), model equal %t (review %t); want the retired setting's",
			migrationHeading, got.Provider(), want.Provider(), got.ReviewProvider(), want.ReviewProvider(),
			got.Model() == want.Model(), got.ReviewModel() == want.ReviewModel())
	}
}

// TestQuickStartLinks checks the README's Quick start section: it comes before
// Requirements, every numbered item links to a README heading, and every link
// in the section is either such an anchor or an existing relative path.
func TestQuickStartLinks(t *testing.T) {
	readme := readDoc(t, readmePath)
	section := sectionOutsideBlocks(readme, quickStartHeading)
	if section == "" {
		t.Fatalf("README has no %q section", quickStartHeading)
	}
	headings := headingsOutsideBlocks(readme)
	at, requirements := slices.Index(headings, quickStartHeading), slices.Index(headings, "## Requirements")
	if at < 0 || requirements < 0 || at > requirements {
		t.Errorf("README: %q is at heading %d, Requirements at %d; Quick start must come first", quickStartHeading, at, requirements)
	}
	anchors := headingAnchors(t, "README.md", readme)
	items := numberedItems(section)
	if len(items) == 0 {
		t.Fatalf("README %s has no numbered list", quickStartHeading)
	}
	for _, item := range items {
		linked := false
		for _, target := range markdownLinkTargets(item) {
			if anchor, ok := strings.CutPrefix(target, "#"); ok {
				if _, known := anchors[anchor]; known {
					linked = true
				}
			}
		}
		if !linked {
			first, _, _ := strings.Cut(item, "\n")
			t.Errorf("README Quick start item %q has no link to a README heading", first)
		}
	}
	// TestReadmeLinks checks that every link in the README, this section's
	// included, resolves; only the step guidance's own rule is checked here.
	for _, target := range markdownLinkTargets(section) {
		if strings.Contains(target, "://") {
			t.Errorf("README Quick start links to the external URL %q; link to a README section instead", target)
		}
	}
}

// TestProductionProfileLinks checks that every document that describes the
// production profile links to the shared document by a relative path that
// resolves to the file the test reads. The expected target comes from the path
// constant, not from a literal copied out of a document.
func TestProductionProfileLinks(t *testing.T) {
	want, err := filepath.Abs(productionProfilePath)
	if err != nil {
		t.Fatalf("resolve %s: %v", productionProfilePath, err)
	}
	for _, tc := range []struct{ name, path, heading string }{
		{"README.md", readmePath, recommendedConfigHeading},
		{"project_overview.md", projectOverviewPath, "## 決定済みの方針"},
		{"CLAUDE.md", claudePath, "## Quick Links"},
		{"prompts/README.md", promptsReadmePath, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := readDoc(t, tc.path)
			if tc.heading != "" {
				if text = sectionOutsideBlocks(text, tc.heading); text == "" {
					t.Fatalf("%s has no %q section", tc.name, tc.heading)
				}
			}
			for _, target := range markdownLinkTargets(text) {
				if strings.Contains(target, "://") {
					continue
				}
				path, _, _ := strings.Cut(target, "#")
				got, err := filepath.Abs(filepath.Join(filepath.Dir(tc.path), path))
				if err == nil && got == want {
					return
				}
			}
			where := "the whole document"
			if tc.heading != "" {
				where = strconv.Quote(tc.heading)
			}
			t.Errorf("%s has no link that resolves to %s in %s", tc.name, productionProfilePath, where)
		})
	}
}

// TestReadmeLinks checks the README's links: every relative link points at an
// existing path, and every in-page link resolves to an anchor the helper
// produces. GitHub resolves those, so they are the oracle for the anchor rule.
func TestReadmeLinks(t *testing.T) {
	readme := readDoc(t, readmePath)
	anchors := headingAnchors(t, "README.md", readme)
	targets := markdownLinkTargets(readme)
	openers := 0
	for _, line := range scanDocLines(slices.Collect(strings.Lines(readme))) {
		if !line.block {
			openers += strings.Count(line.text, "](")
		}
	}
	if len(targets) != openers {
		t.Errorf("README has %d link openers outside code blocks but the extraction found %d targets; a link form it drops would go unchecked", openers, len(targets))
	}
	for _, target := range targets {
		if anchor, ok := strings.CutPrefix(target, "#"); ok {
			if _, known := anchors[anchor]; !known {
				t.Errorf("README links to %q, which the anchor helper does not produce", target)
			}
			continue
		}
		if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		path, _, _ := strings.Cut(target, "#")
		if _, err := os.Stat(filepath.Join(filepath.Dir(readmePath), path)); err != nil {
			t.Errorf("README links to %q, which does not exist", target)
		}
	}
}

// TestAnchorFromHeading pins the anchor rule: lowercase, markup punctuation
// dropped, spaces turned into hyphens, digits and connector punctuation kept.
// The rows for headings the README already has were checked against GitHub's
// rendering of the README on 2026-10-26; the others pin the same rule for a
// heading the README does not have yet.
func TestAnchorFromHeading(t *testing.T) {
	for _, tc := range []struct{ heading, want string }{
		{"## Setting up yt-dlp", "setting-up-yt-dlp"},
		{"### Installing the native messaging host", "installing-the-native-messaging-host"},
		{"## Cache and concurrency", "cache-and-concurrency"},
		{"### The `--out` file", "the---out-file"},
		{"### The Slack post", "the-slack-post"},
		{"### Using DeepSeek", "using-deepseek"},
		{"## Step 2", "step-2"},
		{"## prompt_v2 notes", "prompt_v2-notes"},
		{"## Step\u00a0²", "step²"},
	} {
		if got := anchorFromHeading(tc.heading); got != tc.want {
			t.Errorf("anchorFromHeading(%q) = %q, want %q", tc.heading, got, tc.want)
		}
	}
}

// TestNumberedItems checks the numbered-list extraction: an item ends at a
// blank line or the next item, a fenced block that belongs to an item stays in
// it, and an indented number is not a top-level item.
func TestNumberedItems(t *testing.T) {
	const section = "intro\n1. first\n   continued\n2. second\n\n   3. indented\n\n```text\n3. not an item\n```\n3. third\n"
	want := []string{"1. first\n   continued", "2. second", "3. third"}
	if got := numberedItems(section); !slices.Equal(got, want) {
		t.Errorf("numberedItems = %q, want %q", got, want)
	}
	const withBlock = "1. run this\n   ```sh\n   3. not an item\n\n   make test\n   ```\n2. next\n"
	wantWithBlock := []string{"1. run this\n   ```sh\n   3. not an item\n\n   make test\n   ```", "2. next"}
	if got := numberedItems(withBlock); !slices.Equal(got, wantWithBlock) {
		t.Errorf("numberedItems with a block inside an item = %q, want %q", got, wantWithBlock)
	}
}

// TestSectionOutsideBlocks checks the fence-aware section extraction: a '#' or
// '##' line inside a code block neither ends the section nor starts one.
func TestSectionOutsideBlocks(t *testing.T) {
	const doc = "# Top\ntop\n## Middle\nmiddle\n```text\n# not a heading\n## also not\n```\n### Deep\ndeep\n## Next\nnext\n"
	for _, tc := range []struct {
		name, heading, want string
	}{
		{"in-block headings do not end the section", "## Middle", "middle\n```text\n# not a heading\n## also not\n```\n### Deep\ndeep\n"},
		{"an in-block heading is not a section start", "## also not", ""},
		{"a missing heading yields the empty string", "## Absent", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sectionOutsideBlocks(doc, tc.heading); got != tc.want {
				t.Errorf("sectionOutsideBlocks(doc, %q) = %q, want %q", tc.heading, got, tc.want)
			}
		})
	}
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

// TestTokenUsageDocsContract checks that the README's Token usage section
// contains the line's words and every reason llm.Usage can produce.
//
// The behavior these words describe is pinned by running tests, not by this
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

// TestCostDocsContract checks that the README's Estimated cost section uses
// every word run.go's cost format defines, that its example lines use only
// those words and the stage names in the fixed positions, that
// project_overview.md contains the cost prefix, and that the rate table's
// leading comment points at the update procedure.
//
// The behavior these words describe is pinned by running tests, not by this
// test: the line format by cmd/yt2column's TestRunCostLinesGoldenFormat.
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
