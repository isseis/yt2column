//go:build test

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/nmhost"
	"github.com/isseis/yt2column/internal/server"
)

// Paths from cmd/yt2column-server to the documents this test checks.
const (
	readmePath          = "../../README.md"
	projectOverviewPath = "../../docs/dev/project_overview.md"
	securityPath        = "../../docs/dev/security.md"
)

// TestServerDocsContract pins the machine-checkable contract of the localhost
// server's documents. The values that can drift from the code are derived from
// it: the socket path and the variable names come from internal/config, and
// the rejection reasons and status codes come from internal/server's test
// accessor. A value that changes in the code fails this test until the
// documents follow, and a document that states a wrong value fails it too.
func TestServerDocsContract(t *testing.T) {
	slackEnv := config.SlackWebhookEnvForTest()
	cacheDirEnv := config.CacheDirEnvForTest()
	ytDlpEnv := config.YtDlpEnvForTest()

	readme := readDoc(t, readmePath)
	overview := readDoc(t, projectOverviewPath)
	security := readDoc(t, securityPath)

	t.Run("README.md", func(t *testing.T) {
		whole := sectionByHeading(readme, "## Localhost server")
		if whole == "" {
			t.Fatal("README.md has no '## Localhost server' section")
		}
		// The upgrade notes name the retired token file on purpose, so the
		// checks below read the section without them. The heading is used only
		// to find the range to exclude; the notes themselves are not checked.
		const upgradeHeading = "### Upgrading from the shared-token version"
		section := whole
		if upgrade := sectionByHeading(whole, upgradeHeading); upgrade != "" {
			section = strings.Replace(whole, upgradeHeading+"\n"+upgrade, "", 1)
		}
		if !strings.Contains(section, slackEnv) {
			t.Errorf("README.md server section does not mention %q", slackEnv)
		}
		// The socket path for each operating system, each on its own line; the
		// curl recipe repeats them, so a bare Contains would pass without it.
		if !hasLineContainingAll(section, "macOS:", "$HOME"+config.SocketPathTailForTest("darwin")) ||
			!hasLineContainingAll(section, "other Unix:", "$HOME"+config.SocketPathTailForTest("linux")) {
			t.Error("README.md server section does not state the socket path for macOS and for other Unix")
		}
		// The curl example sends to the socket.
		if !hasLineContainingAll(section, "curl", server.ArticlesPath) {
			t.Error("README.md server section has no curl example for " + server.ArticlesPath)
		}
		// The exit-code table must name every code the entry point defines.
		exitCodes := findTable(section, "Code")
		if exitCodes == nil {
			t.Fatal("README.md server section has no exit-code table")
		}
		for _, code := range []int{exitOK, exitFailure, exitUsage} {
			if tableRow(exitCodes, strconv.Itoa(code)) == nil {
				t.Errorf("README.md server exit-code table has no code %d", code)
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
		reasons := server.RejectReasonsForTest()
		for _, reason := range reasons {
			got, ok := byReason[reason.Value]
			if !ok {
				t.Errorf("README.md rejection table has no reason %q", reason.Value)
				continue
			}
			if got != reason.Status {
				t.Errorf("README.md lists %s with status %d, want %d", reason.Value, got, reason.Status)
			}
		}
		if len(byReason) != len(reasons) {
			t.Errorf("README.md rejection table has %d rows, want %d", len(byReason), len(reasons))
		}
	})

	t.Run("project_overview.md", func(t *testing.T) {
		// The variable names also appear in the configuration table, so the
		// check looks for a line that holds both of them.
		if !hasLineContainingAll(overview, cacheDirEnv, ytDlpEnv) {
			t.Errorf("project_overview.md does not name %s and %s on one line", cacheDirEnv, ytDlpEnv)
		}
	})

	t.Run("security.md", func(t *testing.T) {
		section := sectionByHeading(security, "### 8.2. localhost サーバ（#109）")
		if section == "" {
			t.Fatal("security.md has no 8.2 server section")
		}
		bodyMiB := fmt.Sprintf("%d MiB", server.MaxBodyBytesForTest>>20)
		// The Japanese word for "slot" is kept as the hint word: the slots'
		// number alone also matches other small values in 8.2.
		for _, want := range []string{
			"枠（`" + strconv.Itoa(server.MaxInFlightForTest) + "`）",
			bodyMiB,
		} {
			if !strings.Contains(section, want) {
				t.Errorf("security.md 8.2 does not mention %q", want)
			}
		}
		if !hasLineContainingAll(section, "`allowed_origins`", nmhost.ExtensionOrigin) {
			t.Error("security.md 8.2 does not name the host's allowed origin")
		}
	})
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

// TestSectionByHeading pins the duplicated helper's boundaries, so the copy
// cannot drift from cmd/yt2column's TestSectionByHeading.
func TestSectionByHeading(t *testing.T) {
	const doc = "# Top\ntop body\n## Middle\nmiddle body\n### Deep\ndeep body\n## Two\ntwo body\n"
	for _, tc := range []struct {
		heading, want string
	}{
		{"## Middle", "middle body\n### Deep\ndeep body\n"},
		{"### Deep", "deep body\n"},
		{"## Absent", ""},
	} {
		if got := sectionByHeading(doc, tc.heading); got != tc.want {
			t.Errorf("sectionByHeading(doc, %q) = %q, want %q", tc.heading, got, tc.want)
		}
	}
}

// hasLineContainingAll reports whether one line of doc contains every one of
// subs, so a claim is checked as a whole sentence rather than as several
// Contains that could each match elsewhere in the document.
func hasLineContainingAll(doc string, subs ...string) bool {
	for line := range strings.Lines(doc) {
		all := true
		for _, sub := range subs {
			if !strings.Contains(line, sub) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
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
