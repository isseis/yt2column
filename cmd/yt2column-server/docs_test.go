//go:build test

package main

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/server"
)

// Paths from cmd/yt2column-server to the documents this test checks.
const (
	readmePath          = "../../README.md"
	claudePath          = "../../CLAUDE.md"
	projectOverviewPath = "../../docs/dev/project_overview.md"
	securityPath        = "../../docs/dev/security.md"
	glossaryPath        = "../../docs/translation_glossary.md"
)

// TestServerDocsContract pins the machine-checkable contract of the localhost
// server's documents. The values that can drift from the code are derived from
// it: the default port and the variable names come from internal/config, and
// the rejection reasons and status codes come from internal/server's test
// accessor. A value that changes in the code fails this test until the
// documents follow, and a document that states a wrong value fails it too.
func TestServerDocsContract(t *testing.T) {
	port := serverDefaultPort(t)

	readme := readDoc(t, readmePath)
	overview := readDoc(t, projectOverviewPath)
	security := readDoc(t, securityPath)
	glossary := readDoc(t, glossaryPath)

	t.Run("README.md", func(t *testing.T) {
		section := sectionByHeading(readme, "## Localhost server")
		if section == "" {
			t.Fatal("README.md has no '## Localhost server' section")
		}
		for _, want := range []string{
			"yt2column-server",
			"build/yt2column-server",
			"YT2COLUMN_SERVER_PORT",
			"SLACK_WEBHOOK_URL",
			"server-token",
			"0600",
			strconv.Itoa(port),
		} {
			if !strings.Contains(section, want) {
				t.Errorf("README.md server section does not mention %q", want)
			}
		}
		// A curl example must carry the endpoint, the four headers, and the
		// token fed through standard input (not as an argument).
		for _, want := range []string{"/articles", "Origin: chrome-extension://", "Content-Type: application/json", "-H @-"} {
			if !strings.Contains(section, want) {
				t.Errorf("README.md server section has no %q in its curl example", want)
			}
		}
	})

	t.Run("rejection_reasons", func(t *testing.T) {
		table := findTable(readme, "reason")
		if table == nil {
			t.Fatal("README.md has no rejection reason table")
		}
		byReason := map[string]int{}
		for _, row := range table[1:] {
			if len(row) < 2 {
				continue
			}
			byReason[stripBackticks(row[0])] = parseStatus(t, row[1])
		}
		for _, reason := range server.RejectReasonsForTest() {
			got, ok := byReason[reason.Value]
			if !ok {
				t.Errorf("README.md rejection table has no reason %q", reason.Value)
				continue
			}
			if got != reason.Status {
				t.Errorf("README.md lists %s with status %d, want %d", reason.Value, got, reason.Status)
			}
		}
		if len(byReason) != len(server.RejectReasonsForTest()) {
			t.Errorf("README.md rejection table has %d rows, want %d", len(byReason), len(server.RejectReasonsForTest()))
		}
	})

	t.Run("project_overview.md", func(t *testing.T) {
		for _, want := range []string{"yt2column-server", "internal/server", "YT2COLUMN_SERVER_PORT"} {
			if !strings.Contains(overview, want) {
				t.Errorf("project_overview.md does not mention %q", want)
			}
		}
		for _, name := range []string{"YT2COLUMN_CACHE_DIR", "YT2COLUMN_YTDLP_PATH"} {
			if !strings.Contains(overview, name) {
				t.Errorf("project_overview.md does not mention %q among the server's unused variables", name)
			}
		}
	})

	t.Run("security.md", func(t *testing.T) {
		section := sectionByHeading(security, "### 8.2. localhost サーバ（#109）")
		if section == "" {
			t.Fatal("security.md has no 8.2 server section")
		}
		for _, want := range []string{"server-token", "0600", "O_NOFOLLOW", "link(2)", "chrome.storage.local"} {
			if !strings.Contains(section, want) {
				t.Errorf("security.md 8.2 does not mention %q", want)
			}
		}
	})

	t.Run("CLAUDE.md", func(t *testing.T) {
		for _, want := range []string{
			"yt2column-server",
			"internal/server",
			"internal/authtoken",
			"internal/failure",
			"internal/redact",
			"internal/signalrun",
		} {
			if !strings.Contains(readDoc(t, claudePath), want) {
				t.Errorf("CLAUDE.md does not mention %q", want)
			}
		}
	})

	t.Run("translation_glossary.md", func(t *testing.T) {
		table := findTable(glossary, "日本語")
		if table == nil {
			t.Fatal("translation_glossary.md has no glossary table")
		}
		for term, want := range map[string]string{
			"共有トークン": "shared token",
			"枠":      "slot",
			"拒否の理由":  "reject reason",
		} {
			row := tableRow(table, term)
			if row == nil {
				t.Errorf("translation_glossary.md has no row for %q", term)
				continue
			}
			if len(row) < 2 || row[1] != want {
				t.Errorf("translation_glossary.md translates %q as %q, want %q", term, row[1], want)
			}
		}
	})
}

// serverDefaultPort returns the default YT2COLUMN_SERVER_PORT from
// config.LoadServer, with the server's required variables set and
// YT2COLUMN_SERVER_PORT unset.
func serverDefaultPort(t *testing.T) int {
	t.Helper()
	lookup := func(name string) (string, bool) {
		switch name {
		case "YT2COLUMN_MODEL":
			return "test-model", true
		case "DEEPSEEK_API_KEY":
			return "test-key", true
		case "SLACK_WEBHOOK_URL":
			return "https://hooks.example.com/services/test", true
		case "HOME":
			return "/home/test", true
		default:
			return "", false
		}
	}
	cfg, err := config.LoadServer(lookup)
	if err != nil {
		t.Fatalf("config.LoadServer: %v", err)
	}
	return int(cfg.Port())
}

// parseStatus reads the first integer in cell, failing the test otherwise.
func parseStatus(t *testing.T, cell string) int {
	t.Helper()
	for _, field := range strings.FieldsFunc(cell, func(r rune) bool { return r < '0' || r > '9' }) {
		if n, err := strconv.Atoi(field); err == nil {
			return n
		}
	}
	t.Fatalf("no status code in table cell %q", cell)
	return 0
}

// The Markdown helpers below are duplicated from cmd/yt2column/docs_test.go.
// The two commands are separate main packages, so neither can import the
// other's helpers; extracting them into a shared test-only package would add a
// package and rewrite an existing 600-line guard for a documentation-only
// change. Keep the two copies in step.

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

// stripBackticks removes the code-span backticks around a table cell.
func stripBackticks(s string) string {
	return strings.Trim(s, "`")
}

// headingLevel reports the level of a Markdown heading line: the number of
// leading '#' characters when the line starts with one or more '#' followed by
// a space or tab. It returns 0 for any other line.
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
// when no line matches.
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
