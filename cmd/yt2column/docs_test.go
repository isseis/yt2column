//go:build test

package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/llm"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// Paths from cmd/yt2column to the documents these tests check.
const (
	readmePath           = "../../README.md"
	projectOverviewPath  = "../../docs/dev/project_overview.md"
	securityPath         = "../../docs/dev/security.md"
	reviewPlanPath       = "../../docs/tasks/0010_post_generation_review/03_implementation_plan.md"
	reviewEvaluationPath = "../../docs/tasks/0010_post_generation_review/02_evaluation.md"
	columnArticlesPath   = "../../docs/tasks/0008_column_prompt/02_evaluation.md"
)

// evaluatedVideoCount is the number of videos the column-prompt task evaluated.
// The record guard requires a record for each of them, so a dropped row in that
// task's table does not silently shrink what is checked.
const evaluatedVideoCount = 5

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
	if !tableFlagContains(table, "--help") {
		t.Error("flags table does not document --help")
	}
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
		"test-integration-claude",
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

// TestReviewEvaluationRecords checks the post-generation review evaluation
// record. Until the real-API evaluation (step 6-3) is checked, the record is
// not required. Once it is checked, each of the videos the column-prompt task
// evaluated must have a row in the per-perspective judgment table, the run
// conditions table, and the review-log item table. The video IDs are read from
// the column-prompt task's document, never copied here, so this guard follows
// that task's video list.
func TestReviewEvaluationRecords(t *testing.T) {
	_, done, found := stepBlock(readDoc(t, reviewPlanPath), "**ステップ 6-3**")
	if !found {
		t.Fatal("the plan has no step 6-3")
	}
	if !done {
		return
	}

	records := sectionByHeading(readDoc(t, reviewEvaluationPath), "## 3. 記録")
	if records == "" {
		t.Fatal("02_evaluation.md has no 3. 記録 section")
	}
	judgments := findTable(records, "形式の選択")
	conditions := findTable(records, "所要時間")
	items := findTable(records, "新しいずれ")
	for _, table := range []struct {
		name string
		rows [][]string
	}{
		{"per-perspective judgments", judgments},
		{"run conditions", conditions},
		{"review-log items", items},
	} {
		if table.rows == nil {
			t.Errorf("3. 記録 has no %s table", table.name)
		}
	}
	if judgments == nil || conditions == nil || items == nil {
		return
	}

	for _, id := range columnEvaluationVideoIDs(t) {
		if row := tableRow(judgments, id); row == nil {
			t.Errorf("3. 記録 has no per-perspective row for video %s", id)
		} else {
			requireFilledRow(t, id, "per-perspective", row, 14)
		}

		row := tableRow(conditions, id)
		if row == nil {
			t.Errorf("3. 記録 has no run-conditions row for video %s", id)
		} else {
			requireFilledRow(t, id, "run-conditions", row, 8)
			if !strings.Contains(row[1], "v5") {
				t.Errorf("video %s does not record the v5 template: %q", id, row[1])
			}
		}

		itemRows := tableRowsWithFirstCell(items, id)
		if len(itemRows) == 0 {
			t.Errorf("3. 記録 has no review-log item row for video %s", id)
		}
		for _, row := range itemRows {
			requireFilledRow(t, id, "review-log item", row, 6)
		}
	}
}

// columnEvaluationVideoIDs reads the evaluated video IDs from the column-prompt
// task's §3.2 video table. Only the numbered rows are evaluated videos; the
// reserve row is not, so its first cell does not parse as an integer.
func columnEvaluationVideoIDs(t *testing.T) []string {
	t.Helper()
	table := findTable(sectionByHeading(readDoc(t, columnArticlesPath), "## 3. 対象の動画"), "URL")
	if table == nil {
		t.Fatal("0008's evaluation has no video table with a URL column")
	}
	var ids []string
	for _, row := range table[1:] {
		if len(row) < 2 {
			continue
		}
		if _, err := strconv.Atoi(row[0]); err != nil {
			continue
		}
		id, _, err := transcript.ValidateVideoURL(row[1])
		if err != nil {
			t.Errorf("video row %q has an invalid video URL %q: %v", row[0], row[1], err)
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) != evaluatedVideoCount {
		t.Fatalf("0008's evaluation lists %d numbered videos, want %d", len(ids), evaluatedVideoCount)
	}
	return ids
}

// requireFilledRow fails when row is shorter than want or has an empty cell
// after its first, which is the video ID.
func requireFilledRow(t *testing.T, id, what string, row []string, want int) {
	t.Helper()
	if len(row) < want {
		t.Errorf("video %s %s row has %d cells, want at least %d", id, what, len(row), want)
		return
	}
	for i := 1; i < len(row); i++ {
		if strings.TrimSpace(row[i]) == "" {
			t.Errorf("video %s %s row cell %d is empty", id, what, i+1)
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

// stepBlock returns the text after the plan line containing marker, up to the
// next step marker or section heading, and whether that line is checked.
func stepBlock(doc, marker string) (block string, checked, found bool) {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, line := range lines {
		if strings.Contains(line, marker) {
			start = i
			checked = strings.Contains(line, "[x]")
			break
		}
	}
	if start < 0 {
		return "", false, false
	}
	var b strings.Builder
	for _, line := range lines[start+1:] {
		if strings.Contains(line, "**ステップ ") || strings.HasPrefix(line, "### ") {
			break
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String(), checked, true
}

// TestArtifactDocsContract checks the documents' save-and-resume contract: the
// README flags table and the -h usage state that --after requires --artifacts
// and that --after review does not accept --review-log; project_overview.md
// describes saving and resuming; security.md states the artifact file
// permissions and the read boundary; and 0010's evaluation procedure names
// --artifacts, --after, and --keep-cache.
//
// The behavior these documents describe is pinned by running tests, not by
// this test: security.md's permission (0o600) is internal/artifact's
// TestSaveFileMode, and its "outside the cache directory" rule and its
// inside-cache-directory rejection are cmd/yt2column's
// TestRunArtifactsInsideCacheDir.
func TestArtifactDocsContract(t *testing.T) {
	readme := readDoc(t, readmePath)
	table := findTable(readme, "Flag")
	if table == nil {
		t.Fatal("README has no flags table")
	}
	for _, flag := range []string{"--artifacts", "--after"} {
		if !tableFlagContains(table, flag) {
			t.Errorf("README flags table does not document %s", flag)
		}
	}
	var afterRow []string
	for _, row := range table[1:] {
		if len(row) > 0 && strings.Contains(row[0], "--after") {
			afterRow = row
			break
		}
	}
	if afterRow == nil {
		t.Fatal("README flags table has no --after row")
	}
	rowText := strings.Join(afterRow, " ")
	for _, want := range []string{"--artifacts", "--review-log"} {
		if !strings.Contains(rowText, want) {
			t.Errorf("README --after row does not state the %s rule: %q", want, rowText)
		}
	}

	var stdout bytes.Buffer
	if code := run(context.Background(), []string{"-h"}, lookupFrom(nil), &stdout, io.Discard, productionDeps()); code != exitOK {
		t.Fatalf("run(-h) exit code = %d, want %d", code, exitOK)
	}
	usage := stdout.String()
	for _, want := range []string{"requires --artifacts", "--after review does not accept --review-log"} {
		if !strings.Contains(usage, want) {
			t.Errorf("-h usage does not state %q:\n%s", want, usage)
		}
	}

	overview := readDoc(t, projectOverviewPath)
	for _, want := range []string{"--artifacts", "--after"} {
		if !strings.Contains(overview, want) {
			t.Errorf("project_overview.md does not mention %s", want)
		}
	}

	security := readDoc(t, securityPath)
	cacheSection := sectionByHeading(security, "## 5. キャッシュディレクトリ")
	if cacheSection == "" {
		t.Fatal("security.md has no 5. キャッシュディレクトリ section")
	}
	artifactLine := bulletContaining(cacheSection, "--artifacts")
	if artifactLine == "" {
		t.Fatal("security.md 5. キャッシュディレクトリ has no --artifacts bullet")
	}
	if !strings.Contains(artifactLine, "0o600") {
		t.Errorf("security.md's --artifacts bullet does not state the file mode 0o600: %q", artifactLine)
	}
	untrusted := sectionByHeading(security, "## 6. 信頼できないテキストの取り扱い")
	if untrusted == "" {
		t.Fatal("security.md has no 6. 信頼できないテキストの取り扱い section")
	}
	artifactContentLine := bulletContaining(untrusted, "--artifacts")
	if artifactContentLine == "" {
		t.Fatal("security.md 6. has no --artifacts bullet")
	}
	for _, want := range []string{"U+FEFF", "4 MiB"} {
		if !strings.Contains(artifactContentLine, want) {
			t.Errorf("security.md's --artifacts bullet does not state %q: %q", want, artifactContentLine)
		}
	}

	evaluation := sectionByHeading(readDoc(t, reviewEvaluationPath), "### 2.6. 中間生成物を固定した比較（タスク 0017 以降）")
	if evaluation == "" {
		t.Fatal("02_evaluation.md has no 2.6 section")
	}
	for _, want := range []string{"--artifacts", "--after", "--keep-cache"} {
		if !strings.Contains(evaluation, want) {
			t.Errorf("02_evaluation.md 2.6 does not mention %s", want)
		}
	}
}

// bulletContaining returns the first line of section that contains term, or "".
func bulletContaining(section, term string) string {
	for line := range strings.SplitSeq(section, "\n") {
		if strings.Contains(line, term) {
			return line
		}
	}
	return ""
}

// containsToken reports whether word appears in doc bounded by characters that
// are not letters, digits, underscores, or hyphens, so "known" is not satisfied
// by "unknown" or "recorded-unknown", and "run" is not satisfied by "runs".
func containsToken(doc, word string) bool {
	pattern := `(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(word) + `([^A-Za-z0-9_-]|$)`
	return regexp.MustCompile(pattern).MatchString(doc)
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
	// Every reason llm.Usage can produce, obtained from the code so the test
	// does not spell them: the member-less rules, a member-named rule, and a
	// rule outside the enumeration.
	reasons := []llm.Usage{
		llm.UnknownUsage(llm.RuleNotRead, ""),
		llm.UnknownUsage(llm.RuleRecordedUnknown, ""),
		llm.UnknownUsage(llm.RuleMissing, ""),
		llm.UnknownUsage(llm.RuleDuplicate, ""),
		llm.UnknownUsage(llm.RuleNotObject, ""),
		llm.UnknownUsage(llm.RuleNotNonNegativeInteger, ""),
		llm.UnknownUsage(llm.RuleNotNonNegativeInteger, "output_tokens"),
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
	examples := 0
	for line := range strings.SplitSeq(section, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), tokensPrefix)
		if !ok {
			continue
		}
		words := strings.Fields(rest)
		// A line whose first word is a placeholder shows the shape, not an
		// example; the lines below it are the examples.
		if len(words) > 0 && strings.HasPrefix(words[0], "<") {
			continue
		}
		examples++
		if len(words) < 3 {
			t.Errorf("README example line %q has fewer than three words after the prefix", line)
			continue
		}
		for i := range 3 {
			if !allowed[words[i]] {
				t.Errorf("README example line %q has the unknown word %q at position %d", line, words[i], i+1)
			}
		}
	}
	if examples == 0 {
		t.Error("README Token usage section has no example line")
	}

	overview := sectionByHeading(readDoc(t, projectOverviewPath), "## パイプライン")
	if overview == "" {
		t.Fatal("project_overview.md has no パイプライン section")
	}
	if !strings.Contains(overview, "Usage") {
		t.Error("project_overview.md パイプライン does not mention Usage")
	}

	untrusted := sectionByHeading(readDoc(t, securityPath), "## 6. 信頼できないテキストの取り扱い")
	if untrusted == "" {
		t.Fatal("security.md has no 6. 信頼できないテキストの取り扱い section")
	}
	if !strings.Contains(untrusted, "usage") {
		t.Error("security.md 6. does not mention the response usage")
	}
	artifactLine := bulletContaining(untrusted, "--artifacts")
	if artifactLine == "" {
		t.Fatal("security.md 6. has no --artifacts bullet")
	}
	for _, want := range []string{"usage", "トークン数"} {
		if !strings.Contains(artifactLine, want) {
			t.Errorf("security.md's --artifacts bullet does not mention %q: %q", want, artifactLine)
		}
	}

	reference := readDoc(t, packageReferencePath)
	table := findTable(reference, "Package")
	if table == nil {
		t.Fatal("package_reference.md has no package table")
	}
	for _, row := range []struct {
		pkg  string
		want []string
	}{
		{"internal/llm", []string{"TokenCounts", "UnknownRule", "Usage"}},
		{"internal/strictjson", []string{"IsNull", "AsNonNegativeInt64"}},
		{"internal/writer", []string{"UsageOrigin", "StageUsage", "RunUsage", "TokenUsageError"}},
		{"internal/artifact", []string{"version 2", "usage", "internal/llm"}},
		{"internal/llm/deepseek", []string{"usage"}},
		{"internal/llm/claude", []string{"usage"}},
	} {
		cells := tableRow(table, row.pkg)
		if cells == nil {
			t.Errorf("package_reference.md has no %s row", row.pkg)
			continue
		}
		joined := strings.Join(cells, " ")
		for _, want := range row.want {
			if !strings.Contains(joined, want) {
				t.Errorf("the package_reference.md %s row does not mention %s", row.pkg, want)
			}
		}
	}
}
