//go:build test

package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"testing"

	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	"github.com/isseis/yt2column/internal/publisher"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
	"github.com/isseis/yt2column/internal/transcript"
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
		if !tableContains(table, "--"+f.Name) {
			t.Errorf("flags table does not document --%s", f.Name)
		}
	})
	if !tableContains(table, "--help") {
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

	records := docSection(readDoc(t, reviewEvaluationPath), "3. 記録")
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
	table := findTable(docSection(readDoc(t, columnArticlesPath), "3. 対象の動画"), "URL")
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
	if len(ids) == 0 {
		t.Fatal("0008's video table has no numbered rows")
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

// tableContains reports whether any cell of table contains want.
func tableContains(table [][]string, want string) bool {
	for _, row := range table {
		for _, cell := range row {
			if strings.Contains(cell, want) {
				return true
			}
		}
	}
	return false
}

// stripBackticks removes the code-span backticks around a table cell.
func stripBackticks(s string) string {
	return strings.Trim(s, "`")
}

// docSection returns the text under the "## <heading>" section, up to the next
// "## " heading.
func docSection(doc, heading string) string {
	var b strings.Builder
	inSection := false
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimRight(line, "\n")
		if strings.HasPrefix(trimmed, "## ") {
			if inSection {
				break
			}
			if strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")) == heading {
				inSection = true
				continue
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
