//go:build test

package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

// Paths from cmd/yt2column to the documents these tests check.
const (
	readmePath          = "../../README.md"
	projectOverviewPath = "../../docs/dev/project_overview.md"
	securityPath        = "../../docs/dev/security.md"
	planPath            = "../../docs/tasks/0005_cli_assembly/03_implementation_plan.md"
)

// configDocRow pairs a configuration variable with the "when unset" text each
// document must contain. The expected text comes from the requirements'
// environment-variable table, never from the code.
type configDocRow struct {
	name     string
	readme   string
	overview string
}

var configDocRows = []configDocRow{
	{"YT2COLUMN_LLM_PROVIDER", "deepseek", "deepseek"},
	{"YT2COLUMN_MODEL", "Required", "エラー"},
	{"DEEPSEEK_API_KEY", "Required", "エラー"},
	{"SLACK_WEBHOOK_URL", "Optional", "値なし"},
	{"YT2COLUMN_CACHE_DIR", "yt2column", "yt2column"},
	{"YT2COLUMN_YTDLP_PATH", "yt-dlp", "yt-dlp"},
}

// TestREADMEDocumentsCLI checks that README.md documents the CLI: the calling
// form, every flag run defines, the exit codes, the configuration table with
// each variable's unset value, and the concurrency, SIGKILL, and --refresh
// guidance.
func TestREADMEDocumentsCLI(t *testing.T) {
	doc := readDoc(t, readmePath)

	t.Run("calling form", func(t *testing.T) {
		if !strings.Contains(doc, "yt2column [flags] <video URL>") {
			t.Error("README does not show the calling form")
		}
	})

	t.Run("flags", func(t *testing.T) {
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
	})

	t.Run("exit codes", func(t *testing.T) {
		table := findTable(doc, "Code")
		if table == nil {
			t.Fatal("README has no exit-code table")
		}
		for _, code := range []string{"0", "1", "2"} {
			if tableRow(table, code) == nil {
				t.Errorf("exit-code table has no row for %s", code)
			}
		}
	})

	t.Run("configuration", func(t *testing.T) {
		table := findTable(doc, "Variable")
		if table == nil {
			t.Fatal("README has no configuration table")
		}
		for _, want := range configDocRows {
			row := tableRow(table, want.name)
			if row == nil {
				t.Errorf("configuration table has no row for %s", want.name)
				continue
			}
			if len(row) < 3 {
				t.Errorf("%s row has %d cells, want a when-unset cell", want.name, len(row))
				continue
			}
			if !strings.Contains(row[2], want.readme) {
				t.Errorf("%s when-unset cell = %q, want it to contain %q", want.name, row[2], want.readme)
			}
		}
	})

	t.Run("guidance", func(t *testing.T) {
		// Collapse line wrapping so a phrase that spans two source lines is
		// still found.
		flat := strings.Join(strings.Fields(doc), " ")
		for _, phrase := range []string{
			"serialized",
			"killed with SIGKILL",
			"`--refresh` to fetch the transcript again",
			"0o644",
			"hard link",
			"make test-integration-cli",
		} {
			if !strings.Contains(flat, phrase) {
				t.Errorf("README is missing %q", phrase)
			}
		}
	})
}

// TestProjectOverviewDocumentsConfig checks the project overview's configuration
// table against the same requirements-derived unset values.
func TestProjectOverviewDocumentsConfig(t *testing.T) {
	section := docSection(readDoc(t, projectOverviewPath), "設定（環境変数）")
	if section == "" {
		t.Fatal("project_overview.md has no 設定（環境変数） section")
	}
	table := findTable(section, "変数")
	if table == nil {
		t.Fatal("project_overview.md has no configuration table")
	}
	for _, want := range configDocRows {
		row := tableRow(table, want.name)
		if row == nil {
			t.Errorf("configuration table has no row for %s", want.name)
			continue
		}
		if len(row) < 3 {
			t.Errorf("%s row has %d cells, want a 未設定のとき cell", want.name, len(row))
			continue
		}
		if !strings.Contains(row[2], want.overview) {
			t.Errorf("%s 未設定のとき cell = %q, want it to contain %q", want.name, row[2], want.overview)
		}
	}
}

// TestSecurityDocumentsCLIIntegration checks that security.md section 2 names
// the CLI integration test and its opt-in, and documents that the CLI only
// warns about the GODEBUG http2debug setting.
func TestSecurityDocumentsCLIIntegration(t *testing.T) {
	section := docSection(readDoc(t, securityPath), "2. 秘密情報（API キー・Webhook URL）")
	if section == "" {
		t.Fatal("security.md has no section 2")
	}
	for _, phrase := range []string{"test-integration-cli", "YT2COLUMN_CLI_INTEGRATION", "http2debug", "警告"} {
		if !strings.Contains(section, phrase) {
			t.Errorf("security.md section 2 is missing %q", phrase)
		}
	}
}

// TestPlanRecordsManualRuns checks that the implementation plan carries the two
// manual runs as completion conditions and, once a step is checked, records the
// video URL and the result under it.
func TestPlanRecordsManualRuns(t *testing.T) {
	doc := readDoc(t, planPath)
	for _, step := range []string{"9-5", "9-6"} {
		block, checked, found := stepBlock(doc, "**ステップ "+step+"**")
		if !found {
			t.Errorf("the plan has no step %s", step)
			continue
		}
		if !checked {
			continue
		}
		if !strings.Contains(block, "https://") {
			t.Errorf("checked step %s has no video URL record", step)
		}
		if !strings.Contains(block, "結果") {
			t.Errorf("checked step %s has no result record", step)
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
