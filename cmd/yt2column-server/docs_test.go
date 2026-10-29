//go:build test

package main

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/doctestutil"
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

	readme := doctestutil.ReadDoc(t, readmePath)
	overview := doctestutil.ReadDoc(t, projectOverviewPath)
	security := doctestutil.ReadDoc(t, securityPath)

	t.Run("README.md", func(t *testing.T) {
		whole := doctestutil.SectionByHeading(readme, "## Localhost server")
		if whole == "" {
			t.Fatal("README.md has no '## Localhost server' section")
		}
		// The upgrade notes describe retired leftovers, not the current server,
		// so the checks below read the section without them, as before. The
		// heading is used only to find the range to exclude; the notes
		// themselves are not checked.
		const upgradeHeading = "### Upgrading from the shared-token version"
		section := whole
		if upgrade := doctestutil.SectionByHeading(whole, upgradeHeading); upgrade != "" {
			section = strings.Replace(whole, upgradeHeading+"\n"+upgrade, "", 1)
			if section == whole {
				t.Fatal("README.md server section: the upgrade notes were not excluded")
			}
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
		// The curl examples name each operation's method and a URL whose path
		// has the form the server's path constants define, so a route or
		// method change fails here. <key> stands for the job API's key
		// segment, whatever spelling the example gives it. The shared
		// SectionByHeading is fence-unaware, so a line starting with "# "
		// inside a fenced example would end the section: keep shell comments
		// out of these examples.
		for _, example := range []struct{ operation, method, path string }{
			{"the articles endpoint", "POST", server.ArticlesPath},
			{"the job API's generation start", "POST", server.JobsPath},
			{"the job API's status", "GET", server.JobsPath + "/<key>"},
			{"the job API's publish", "POST", server.JobsPath + "/<key>" + server.PublishSuffix},
		} {
			if !hasCurlCommand(section, example.method, example.path) {
				t.Errorf("README.md server section has no curl example for %s (%s %s)", example.operation, example.method, example.path)
			}
		}
		// The exit-code table must name every code the entry point defines.
		exitCodes := doctestutil.FindTable(section, "Code")
		if exitCodes == nil {
			t.Fatal("README.md server section has no exit-code table")
		}
		for _, code := range []int{exitOK, exitFailure, exitUsage} {
			if doctestutil.TableRow(exitCodes, strconv.Itoa(code)) == nil {
				t.Errorf("README.md server exit-code table has no code %d", code)
			}
		}
	})

	t.Run("rejection_reasons", func(t *testing.T) {
		table := doctestutil.FindTable(readme, "reason")
		if table == nil {
			t.Fatal("README.md has no rejection reason table")
		}
		byReason := map[string]int{}
		for _, row := range table[1:] {
			if len(row) < 2 {
				continue
			}
			byReason[doctestutil.StripBackticks(row[0])] = parseStatus(t, row[1])
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

	t.Run("api_codes", func(t *testing.T) {
		section := doctestutil.SectionByHeading(readme, "## Localhost server")
		for _, set := range server.APICodesForTest() {
			tables := tablesByFirstHeader(section, set.Name)
			if len(tables) != 1 {
				t.Errorf("README.md server section has %d tables headed %q, want 1", len(tables), set.Name)
				continue
			}
			documented := map[string]struct{}{}
			for _, row := range tables[0][1:] {
				if len(row) == 0 {
					continue
				}
				code := doctestutil.StripBackticks(row[0])
				if _, dup := documented[code]; dup {
					t.Errorf("README.md %q table lists %q twice", set.Name, code)
				}
				documented[code] = struct{}{}
				// A set that names each code's state documents it in the
				// second column.
				if want, ok := set.States[code]; ok {
					got := ""
					if len(row) > 1 {
						got = doctestutil.StripBackticks(row[1])
					}
					if got != want {
						t.Errorf("README.md %q table gives %q the state %q, want %q", set.Name, code, got, want)
					}
				}
			}
			defined := map[string]struct{}{}
			for _, code := range set.Codes {
				defined[code] = struct{}{}
			}
			// One code per value: a value a code switch does not name falls
			// to its default, which repeats another value's code.
			if len(defined) != len(set.Codes) {
				t.Errorf("the %q codes %v repeat a code", set.Name, set.Codes)
			}
			if !maps.Equal(documented, defined) {
				t.Errorf("README.md %q table lists %v, want %v", set.Name, slices.Sorted(maps.Keys(documented)), slices.Sorted(maps.Keys(defined)))
			}
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
		section := doctestutil.SectionByHeading(security, "### 8.2. localhost サーバ（#109）")
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

// TestHasCurlCommand pins the forms hasCurlCommand accepts and rejects, so the
// README examples may be spelled in the ordinary shell ways while a wrong
// method, a query, or a command quoted to another program still fails.
func TestHasCurlCommand(t *testing.T) {
	const status = "/jobs/<key>"
	for _, tc := range []struct {
		name string
		doc  string
		want bool
	}{
		{"double quoted target", "curl -X GET \"http://localhost/jobs/$key\"\n", true},
		{"single quoted target", "curl -X GET 'http://localhost/jobs/${key}'\n", true},
		{"joined method", "curl -XGET http://localhost/jobs/abc\n", true},
		{"request flag", "curl --request GET http://localhost/jobs/abc\n", true},
		{"url flag", "curl --url http://localhost/jobs/abc --request GET\n", true},
		{"continuation", "curl -X GET \\\n  http://localhost/jobs/abc\n", true},
		{"wrong method", "curl -X POST http://localhost/jobs/abc\n", false},
		{"query", "curl -X GET \"http://localhost/jobs/abc?x=1\"\n", false},
		{"quoted command", "echo 'curl -X GET http://localhost/jobs/abc'\n", false},
		{"curl as another command's argument", "echo curl -X GET http://localhost/jobs/abc\n", false},
		{"other path", "curl -X GET http://localhost/articles\n", false},
		{"no curl", "wget -X GET http://localhost/jobs/abc\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasCurlCommand(tc.doc, "GET", status); got != tc.want {
				t.Errorf("hasCurlCommand(%q) = %v, want %v", tc.doc, got, tc.want)
			}
		})
	}
}

// hasCurlCommand reports whether doc has a line that runs curl with the given
// HTTP method on a URL whose path has the given form. A trailing backslash
// joins the next line, and shell quotes are trimmed from an argument.
func hasCurlCommand(doc, method, path string) bool {
	var command strings.Builder
	for line := range strings.Lines(doc) {
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		trimmed := strings.TrimRight(line, " \t")
		if before, ok := strings.CutSuffix(trimmed, `\`); ok {
			command.WriteString(before)
			command.WriteString(" ")
			continue
		}
		command.WriteString(line)
		if commandRunsCurl(strings.Fields(command.String()), method, path) {
			return true
		}
		command.Reset()
	}
	return false
}

// commandRunsCurl reports whether fields run curl with the given method on a
// URL whose path has the given form. curl must be the command the line runs, so
// a command only quoted to another program does not count.
func commandRunsCurl(fields []string, method, path string) bool {
	if len(fields) == 0 || fields[0] != "curl" {
		return false
	}
	args := fields[1:]
	var hasMethod, hasURL bool
	for i := 0; i < len(args); i++ {
		arg := strings.Trim(args[i], "\"'")
		switch {
		case arg == "-X" || arg == "--request":
			if i+1 < len(args) && strings.Trim(args[i+1], "\"'") == method {
				hasMethod = true
				i++
			}
		case strings.TrimPrefix(arg, "-X") == method && strings.HasPrefix(arg, "-X"):
			hasMethod = true
		case strings.TrimPrefix(arg, "--request=") == method && strings.HasPrefix(arg, "--request="):
			hasMethod = true
		case arg == "--url":
			if i+1 < len(args) && pathForm(strings.Trim(args[i+1], "\"'")) == path {
				hasURL = true
				i++
			}
		case strings.Contains(arg, "://"):
			if pathForm(arg) == path {
				hasURL = true
			}
		}
	}
	return hasMethod && hasURL
}

// pathForm returns the path a documented URL names, with <key> standing for a
// job API key segment, or "" for a path the server does not serve. A query or a
// fragment is not part of an accepted target, so it returns "".
func pathForm(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	switch u.Path {
	case server.ArticlesPath, server.JobsPath:
		return u.Path
	}
	rest, ok := strings.CutPrefix(u.Path, server.JobsPath+"/")
	if !ok || rest == "" {
		return ""
	}
	if key, found := strings.CutSuffix(rest, server.PublishSuffix); found {
		if key == "" || strings.Contains(key, "/") {
			return ""
		}
		return server.JobsPath + "/<key>" + server.PublishSuffix
	}
	if strings.Contains(rest, "/") {
		return ""
	}
	return server.JobsPath + "/<key>"
}

// tablesByFirstHeader returns every table whose first header cell equals
// header, ignoring backticks and case.
func tablesByFirstHeader(doc, header string) [][][]string {
	var found [][][]string
	for _, table := range doctestutil.MarkdownTables(doc) {
		if len(table) > 0 && len(table[0]) > 0 && strings.EqualFold(doctestutil.StripBackticks(table[0][0]), header) {
			found = append(found, table)
		}
	}
	return found
}
