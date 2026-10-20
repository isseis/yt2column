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
	claudePath          = "../../CLAUDE.md"
	projectOverviewPath = "../../docs/dev/project_overview.md"
	securityPath        = "../../docs/dev/security.md"
	glossaryPath        = "../../docs/translation_glossary.md"
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
	glossary := readDoc(t, glossaryPath)

	t.Run("README.md", func(t *testing.T) {
		whole := sectionByHeading(readme, "## Localhost server")
		if whole == "" {
			t.Fatal("README.md has no '## Localhost server' section")
		}
		// The upgrade notes name the retired token file on purpose; every other
		// part of the section must not describe the token or the port.
		const upgradeHeading = "### Upgrading from the shared-token version"
		upgrade := sectionByHeading(whole, upgradeHeading)
		if upgrade == "" {
			t.Fatalf("README.md server section has no %q section", upgradeHeading)
		}
		if !strings.Contains(upgrade, "server-token") || !strings.Contains(upgrade, "can be deleted") {
			t.Error("README.md upgrade notes do not say the server-token file can be deleted")
		}
		section := strings.Replace(whole, upgradeHeading+"\n"+upgrade, "", 1)
		if section == whole {
			t.Fatal("README.md server section: the upgrade notes were not excluded")
		}
		for _, want := range []string{
			"yt2column-server",
			"build/yt2column-server",
			slackEnv,
			"`0700`",
			"`0600`",
		} {
			if !strings.Contains(section, want) {
				t.Errorf("README.md server section does not mention %q", want)
			}
		}
		// The socket path for each operating system, each on its own line; the
		// curl recipe repeats them, so a bare Contains would pass without it.
		if !hasLineContainingAll(section, "macOS:", "$HOME"+config.SocketPathTailForTest("darwin")) ||
			!hasLineContainingAll(section, "other Unix:", "$HOME"+config.SocketPathTailForTest("linux")) {
			t.Error("README.md server section does not state the socket path for macOS and for other Unix")
		}
		if !hasLineContainingAll(section, "needs no shared token") {
			t.Error("README.md server section does not say the server needs no shared token")
		}
		// The curl example sends to the socket, with the path quoted because
		// the macOS path holds a space.
		if !hasLineContainingAll(section, "curl", `--unix-socket "$socket"`, server.ArticlesPath) {
			t.Error("README.md server section has no curl --unix-socket example for " + server.ArticlesPath)
		}
		// The host is registered and removed by the user, one command each.
		for _, command := range []string{"install", "uninstall"} {
			for _, browser := range []string{"chrome", "brave"} {
				if line := "./build/yt2column-nmhost " + command + " " + browser; !hasLineContainingAll(section, line) {
					t.Errorf("README.md server section has no %q command", line)
				}
			}
		}
		// The host's install/uninstall exit-code table must name every code
		// the host defines; the host shares the values 0, 1, and 2 with the
		// server's constants, which this package can reach.
		hostTable := findTable(sectionByHeading(section, "### Installing the native messaging host"), "Code")
		if hostTable == nil {
			t.Fatal("README.md has no host exit-code table under '### Installing the native messaging host'")
		}
		for _, code := range []int{exitOK, exitFailure, exitUsage} {
			if tableRow(hostTable, strconv.Itoa(code)) == nil {
				t.Errorf("README.md host exit-code table has no code %d", code)
			}
		}
		troubleshooting := sectionByHeading(section, "### Troubleshooting")
		for _, want := range []string{"The server is not running", "The native messaging host is not found", "Reading the host's log"} {
			if !strings.Contains(troubleshooting, want) {
				t.Errorf("README.md server troubleshooting has no %q item", want)
			}
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
		// The token and port of the TCP server must not come back. "port"
		// alone is not checked: it is a substring of words such as "Support".
		for _, retired := range []string{"YT2COLUMN_SERVER_PORT", "8765", "Authorization: Bearer", "options page"} {
			if strings.Contains(section, retired) {
				t.Errorf("README.md server section still mentions %q", retired)
			}
		}
	})

	t.Run("README.md development server", func(t *testing.T) {
		section := sectionByHeading(sectionByHeading(readme, "## Development"), "### Running a separate server for development")
		if section == "" {
			t.Fatal("README.md has no '### Running a separate server for development' section under '## Development'")
		}
		if !hasLineContainingAll(section, "mktemp -d /tmp/") {
			t.Error("README.md development server section does not create a short HOME under /tmp")
		}
		if !hasLineContainingAll(section, `HOME="$dev_home"`, "yt2column-server") {
			t.Error("README.md development server section does not start the server with that HOME")
		}
		if !hasLineContainingAll(section, "curl", `--unix-socket "$socket"`) {
			t.Error("README.md development server section has no curl --unix-socket example")
		}
		// The host limits, which span lines, are matched on the joined text.
		joined := strings.Join(strings.Fields(section), " ")
		for _, want := range []string{
			"The extension cannot reach such a server.",
			"The host manifest is one file per browser",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("README.md development server section does not say %q", want)
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
		for _, want := range []string{"yt2column-server", "internal/server", "internal/serversock", "cmd/yt2column-nmhost", "internal/nmhost"} {
			if !strings.Contains(overview, want) {
				t.Errorf("project_overview.md does not mention %q", want)
			}
		}
		if !hasLineContainingAll(overview, "`yt2column-nmhost install <browser>`", "`yt2column-nmhost uninstall <browser>`") {
			t.Error("project_overview.md does not name the host's install and uninstall commands")
		}
		// The "does not read" claim must be one sentence; the variable names
		// also appear in the configuration table, so a bare Contains would pass
		// even without it.
		if !hasLineContainingAll(overview, cacheDirEnv, ytDlpEnv, "読まない") {
			t.Errorf("project_overview.md has no sentence saying the server does not read %s or %s", cacheDirEnv, ytDlpEnv)
		}
	})

	t.Run("security.md", func(t *testing.T) {
		section8 := sectionByHeading(security, "## 8. ブラウザ拡張と localhost サーバ（#43）")
		intro, _, _ := strings.Cut(section8, "\n### ")
		if intro == "" {
			t.Fatal("security.md has no section 8 introduction")
		}
		// The reason each kind of client is excluded, the limit of the socket's
		// permission, and the trusted same-user processes, each as a sentence.
		for _, want := range [][]string{
			{"他ユーザーのプロセス", "`0700`", "`0600`"},
			{"ウェブページ", "TCP を待ち受けない"},
			{"サンドボックス化された Agent", "サンドボックスのポリシー"},
			{"UDS の権限は同じ uid のプロセスを区別しない"},
			{"同一ユーザーのフル権限プロセスは信頼する"},
			{"共有トークンを削除した理由"},
		} {
			if !hasLineContainingAll(intro, want...) {
				t.Errorf("security.md section 8 introduction has no line with %q", want)
			}
		}

		section := sectionByHeading(security, "### 8.2. localhost サーバ（#109）")
		if section == "" {
			t.Fatal("security.md has no 8.2 server section")
		}
		bodyMiB := fmt.Sprintf("%d MiB", server.MaxBodyBytesForTest>>20)
		for _, want := range []string{
			"`0700`",
			"`0600`",
			"起動のロック",
			"枠（`" + strconv.Itoa(server.MaxInFlightForTest) + "`）",
			bodyMiB,
		} {
			if !strings.Contains(section, want) {
				t.Errorf("security.md 8.2 does not mention %q", want)
			}
		}
		if !hasLineContainingAll(section, "stale", "接続できるソケット", "削除せずに中止") {
			t.Error("security.md 8.2 does not say which socket is removed as stale and which is left in place")
		}
		if !hasLineContainingAll(section, "`unlink`", "bind", "同一ユーザー") {
			t.Error("security.md 8.2 does not name the residual risk of a same-user unlink and bind")
		}
		// The host's items: the one allowed origin, standard output as the
		// channel, and the residual risk of a replaced binary or manifest.
		for _, want := range [][]string{
			{"`allowed_origins`", nmhost.ExtensionOrigin},
			{"標準出力が通信路である"},
			{"ホストのバイナリと manifest を置き換えられ"},
		} {
			if !hasLineContainingAll(section, want...) {
				t.Errorf("security.md 8.2 has no line with %q", want)
			}
		}
		for _, retired := range []string{"トークン", "server-token", "O_NOFOLLOW", "link(2)", "chrome.storage.local", "起動のたび"} {
			if strings.Contains(section, retired) {
				t.Errorf("security.md 8.2 still mentions %q", retired)
			}
		}
	})

	t.Run("CLAUDE.md", func(t *testing.T) {
		claude := readDoc(t, claudePath)
		for _, want := range []string{
			"yt2column-server",
			"internal/server",
			"internal/serversock",
			"internal/failure",
			"internal/redact",
			"internal/signalrun",
			"internal/nmhost",
		} {
			if !strings.Contains(claude, want) {
				t.Errorf("CLAUDE.md does not mention %q", want)
			}
		}
		if !hasLineContainingAll(claude, "`make build`", "three binaries", "`yt2column-nmhost`") {
			t.Error("CLAUDE.md does not say make build builds the three binaries, the host included")
		}
		if !hasLineContainingAll(claude, "yt2column-nmhost install <browser>", "yt2column-nmhost uninstall <browser>") {
			t.Error("CLAUDE.md does not name the host's install and uninstall commands")
		}
		if strings.Contains(claude, "internal/authtoken") {
			t.Error("CLAUDE.md still mentions internal/authtoken")
		}
	})

	t.Run("translation_glossary.md", func(t *testing.T) {
		table := findTable(glossary, "日本語")
		if table == nil {
			t.Fatal("translation_glossary.md has no glossary table")
		}
		for term, want := range map[string]string{
			"起動のロック": "start lock",
			"枠":      "slot",
			"拒否の理由":  "reject reason",
			"ホスト":    "host",
			"ホスト名":   "host name",
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
		if tableRow(table, "共有トークン") != nil {
			t.Error("translation_glossary.md still has a row for 共有トークン")
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
