//go:build test

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/fixprguard"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

// guideFiles are the Japanese original and the English translation of the
// fixprguard guide.
var guideFiles = []string{
	"docs/dev/developer_guide/fixprguard_guide.ja.md",
	"docs/dev/developer_guide/fixprguard_guide.md",
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestGuideCoversStopKindsAndScope: both guides name, in code spans, every
// stop kind the tool can print, and CLAUDE.md links to the guide. In chapter
// 6, every deny-list item is in the execution-related file table and every
// allow-list item in the allow-list table, and *.go is not in the
// execution-related file table. The values come from internal/fixprguard, so
// a kind or an item added there without the guide fails here. Reading the
// tables alone keeps a code span in the prose around them from standing in
// for a row.
func TestGuideCoversStopKindsAndScope(t *testing.T) {
	for _, tc := range []struct {
		rel             string
		execMark, allow string
	}{
		{rel: guideFiles[0], execMark: "**実行関連ファイル。**", allow: "**許可リスト。**"},
		{rel: guideFiles[1], execMark: "**Execution-related files.**", allow: "**Allow list.**"},
	} {
		guide := readRepoFile(t, tc.rel)
		for _, kind := range fixprguard.StopKinds() {
			if !strings.Contains(guide, "`"+kind+"`") {
				t.Errorf("%s does not name stop kind `%s`", tc.rel, kind)
			}
		}
		scope := section(guide, "## 6.")
		execTable := guideTable(scope, tc.execMark)
		allowTable := guideTable(scope, tc.allow)
		for _, table := range []struct {
			name, text string
			patterns   []string
		}{
			{"execution-related file", execTable, fixprguard.ExecutionFilePatterns()},
			{"allow-list", allowTable, fixprguard.AllowListPatterns()},
		} {
			for _, pattern := range table.patterns {
				if !strings.Contains(table.text, "`"+pattern+"`") {
					t.Errorf("%s: the %s table in chapter 6 does not list `%s`", tc.rel, table.name, pattern)
				}
			}
		}
		if strings.Contains(execTable, "`*.go`") {
			t.Errorf("%s: the execution-related file table lists `*.go`, which is on the allow list", tc.rel)
		}
	}
	if claude := readRepoFile(t, "CLAUDE.md"); !strings.Contains(claude, "](docs/dev/developer_guide/fixprguard_guide.md)") {
		t.Error("CLAUDE.md does not link to the fixprguard guide")
	}
}

// guideTable returns the lines of the first table after the line of doc that
// starts with marker. It returns "" when there is no such line or another
// bold heading comes before a table, so a missing table never borrows the
// next one.
func guideTable(doc, marker string) string {
	_, rest, ok := strings.Cut(doc, "\n"+marker)
	if !ok {
		return ""
	}
	_, rest, _ = strings.Cut(rest, "\n")
	var table strings.Builder
	for line := range strings.Lines(rest) {
		switch {
		case strings.HasPrefix(line, "|"):
			table.WriteString(line)
		case table.Len() > 0, strings.HasPrefix(line, "**"):
			return table.String()
		}
	}
	return table.String()
}

// TestGuideTable: guideTable takes the table's lines only, stops where the
// table ends, and finds nothing for a missing marker or table.
func TestGuideTable(t *testing.T) {
	doc := "## 6\n**A.** a `p`\n\n| h |\n|---|\n| `x` |\nafter `y`\n\n| `w` |\n**B.** b\n**C.** c\n\n| `z` |\n"
	for _, tc := range []struct {
		name   string
		doc    string
		marker string
		want   string
	}{
		{"stops_at_table_end", doc, "**A.**", "| h |\n|---|\n| `x` |\n"},
		{"table_after_prose", doc, "**C.**", "| `z` |\n"},
		// Bullets and a paragraph between the marker and the table, as in
		// the guide.
		{"table_after_bullets", "x\n**E.** e\n- bullet `q`\npara\n\n| `v` |\n", "**E.**", "| `v` |\n"},
		// The next bold heading comes before any table.
		{"no_table_before_next_heading", doc, "**B.**", ""},
		{"missing_marker", doc, "**D.**", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := guideTable(tc.doc, tc.marker); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// reviewerPlaceholder is the --reviewer value the command documents write.
// The documents write it once per call and define it as one --reviewer per
// name of the automated reviewer list in _context.md, so the names live in
// _context.md alone and the list can change without editing a call.
const reviewerPlaceholder = "<reviewer>"

var (
	errReviewerCount = errors.New("--reviewer not given exactly once")
	errReviewerValue = errors.New("--reviewer is not the placeholder")
)

// checkDocumentedReviewer checks a parsed documented call of a subcommand that
// takes --reviewer: its collected --reviewer values must be exactly one, the
// placeholder. A call of another subcommand has nothing to check (parse
// refuses --reviewer there).
func checkDocumentedReviewer(inv invocation) error {
	if !slices.Contains(specs[inv.name].flags, flagReviewer) {
		return nil
	}
	values := inv.multi[flagReviewer]
	if len(values) != 1 {
		return fmt.Errorf("%w: %d values", errReviewerCount, len(values))
	}
	if values[0] != reviewerPlaceholder {
		return fmt.Errorf("%w: %q", errReviewerValue, values[0])
	}
	return nil
}

// documentedInvocations returns the argument lists of the fixprguard calls in
// doc's fenced code blocks, one per line that starts with "fixprguard ". A
// blockquote marker is dropped first, so a block inside an agent prompt is
// read too.
func documentedInvocations(doc string) [][]string {
	var calls [][]string
	inBlock := false
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(trimmed, ">"); ok {
			trimmed = strings.TrimSpace(rest)
		}
		if strings.HasPrefix(trimmed, "```") {
			inBlock = !inBlock
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "fixprguard "); inBlock && ok {
			calls = append(calls, strings.Fields(rest))
		}
	}
	return calls
}

// TestDocumentedInvocationsParse runs every fixprguard call written in
// fixpr.md and runpr.md through the tool's own command-line parser: the
// subcommand, the flag names, the positional argument, and --protocol must be
// what this binary accepts, and --reviewer must be given once, as the
// placeholder for the names in _context.md. Each document must also call the
// subcommands it uses.
func TestDocumentedInvocationsParse(t *testing.T) {
	for _, tc := range []struct {
		rel  string
		want []string
	}{
		{".claude/commands/fixpr.md", []string{cmdBegin, cmdFence, cmdInspect, cmdFinish, cmdPush, cmdReply, cmdEditPR}},
		{".claude/commands/runpr.md", []string{cmdHead, cmdList, cmdReply}},
	} {
		calls := documentedInvocations(readRepoFile(t, tc.rel))
		seen := map[string]struct{}{}
		for _, args := range calls {
			inv, err := parse(args)
			if err != nil {
				t.Errorf("%s: fixprguard %s: %v", tc.rel, strings.Join(args, " "), err)
				continue
			}
			seen[inv.name] = struct{}{}
			if err := checkDocumentedReviewer(inv); err != nil {
				t.Errorf("%s: fixprguard %s: %v", tc.rel, inv.name, err)
			}
		}
		for _, name := range tc.want {
			if _, ok := seen[name]; !ok {
				t.Errorf("%s has no fixprguard %s call", tc.rel, name)
			}
		}
	}
}

// TestCheckDocumentedReviewer runs the documented-call check on synthesized
// calls, so a check loosened later fails here and not only when a document
// happens to break it.
func TestCheckDocumentedReviewer(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want error
	}{
		{"one_placeholder", args(cmdList, "--reviewer", reviewerPlaceholder, testPR), nil},
		{"no_reviewer_subcommand", args(cmdHead, testPR), nil},
		{"two_placeholders", args(cmdBegin, "--reviewer", reviewerPlaceholder, "--reviewer", reviewerPlaceholder, testPR), errReviewerCount},
		{"placeholder_and_name", args(cmdBegin, "--reviewer", reviewerPlaceholder, "--reviewer", testReviewer, testPR), errReviewerCount},
		{"name_itself", args(cmdInspect, "--reviewer", testReviewer, "--baseline", "<b>", testPR), errReviewerValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := parse(tc.args)
			if err != nil {
				t.Fatalf("parse(%q) = %v", tc.args, err)
			}
			if err := checkDocumentedReviewer(inv); !errors.Is(err, tc.want) {
				t.Errorf("checkDocumentedReviewer = %v, want %v", err, tc.want)
			}
		})
	}
}

// singleLine returns the one line of doc that starts with prefix, failing when
// there is none or more than one.
func singleLine(t *testing.T, rel, doc, prefix string) string {
	t.Helper()
	var found []string
	for line := range strings.Lines(doc) {
		if strings.HasPrefix(line, prefix) {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Errorf("%s has %d lines starting with %q, want 1", rel, len(found), prefix)
		return ""
	}
	return found[0]
}

// section returns the text of doc from the heading line that starts with
// heading up to the next level-2 heading.
func section(doc, heading string) string { return cutSection(doc, heading, "\n## ") }

// subsection returns the text of doc from the line that starts with heading
// up to the next heading of level 2 or 3.
func subsection(doc, heading string) string { return cutSection(doc, heading, "\n## ", "\n### ") }

// cutSection returns the text of doc from the line that starts with heading
// up to the first of ends, or "" when there is no such line.
func cutSection(doc, heading string, ends ...string) string {
	_, rest, ok := strings.Cut(doc, "\n"+heading)
	if !ok {
		return ""
	}
	end := len(rest)
	for _, next := range ends {
		if i := strings.Index(rest, next); i >= 0 && i < end {
			end = i
		}
	}
	return rest[:end]
}

// reviewerListName is the _context.md row the guides point to for the names
// /fixpr and /runpr pass as --reviewer, and contentGuard the test file that
// pins that row's names, which section 9 tells a maintainer to update with it.
const (
	reviewerListName = "`Automated reviewer list`"
	contentGuard     = "cmd/yt2column/fixpr_docs_test.go"
)

// TestGuideDescribesReviewerList: both guides describe repeated --reviewer as
// the tool implements it. The subcommand headings that write
// "--reviewer LOGIN ..." are exactly the subcommands whose spec takes
// --reviewer; the flag row says the flag repeats once per name and that none
// or a name given twice is a usage error; other-bot is a Bot matching none of
// the names; the flag-position row lists --reviewer as repeatable; the exit
// code 2 row identifies a tool that predates the change and how to check its
// version; section 9 states the Protocol proviso; and the trusted-author
// item and the flag row say the names passed are every name on _context.md's
// "Automated reviewer list", and section 9 names the content guard that pins
// that list, which must exist.
// The phrases are the guides' own wording, Japanese for the Japanese guide.
func TestGuideDescribesReviewerList(t *testing.T) {
	takesReviewer := map[string]struct{}{}
	for name, sp := range specs {
		if slices.Contains(sp.flags, flagReviewer) {
			takesReviewer[name] = struct{}{}
		}
	}
	for _, tc := range []struct {
		rel         string
		trustedItem string
		flagRow     []string
		positionRow string
		otherBot    string
		proviso     string
	}{
		{
			rel:         guideFiles[0],
			trustedItem: "1. **著者の判定。**",
			positionRow: "| フラグの位置 |",
			flagRow:     []string{"名前ごとに 1 回ずつ繰り返して渡す", "1 つもないとき", "同じ名前を 2 回以上渡したとき", "使い方の誤り"},
			otherBot:    "`--reviewer` で渡したどの名前とも一致しない Bot",
			proviso:     "次の 3 つの条件がすべて成り立つ変更に限り、`Protocol` を上げなくてよい",
		},
		{
			rel:         guideFiles[1],
			trustedItem: "1. **Author judgment.**",
			positionRow: "| Flag position |",
			flagRow:     []string{"repeating the flag once per name", "When none is given", "when the same name is given twice or more", "usage error"},
			otherBot:    "A Bot matching none of the names passed with `--reviewer`",
			proviso:     "only a change that meets all three conditions below may leave `Protocol` unchanged",
		},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			guide := readRepoFile(t, tc.rel)
			headed := map[string]struct{}{}
			for line := range strings.Lines(guide) {
				rest, ok := strings.CutPrefix(line, "### `")
				if !ok {
					continue
				}
				if name, _, _ := strings.Cut(rest, " "); strings.Contains(rest, "--reviewer LOGIN ...") {
					headed[name] = struct{}{}
				}
			}
			if !maps.Equal(headed, takesReviewer) {
				t.Errorf("headings with --reviewer LOGIN ... are %v, want the subcommands taking --reviewer %v", slices.Sorted(maps.Keys(headed)), slices.Sorted(maps.Keys(takesReviewer)))
			}
			flagRow := singleLine(t, tc.rel, guide, "| `--reviewer LOGIN` |")
			for _, want := range tc.flagRow {
				if !strings.Contains(flagRow, want) {
					t.Errorf("the --reviewer row does not say %q", want)
				}
			}
			// The flag-position row lists --reviewer among the repeatable flags.
			if row := singleLine(t, tc.rel, guide, tc.positionRow); !strings.Contains(row, "`--reviewer`") {
				t.Errorf("the flag-position row does not list --reviewer as repeatable")
			}
			if row := singleLine(t, tc.rel, guide, "| `other-bot` |"); !strings.Contains(row, tc.otherBot) {
				t.Errorf("the other-bot row does not say %q", tc.otherBot)
			}
			exit2 := singleLine(t, tc.rel, guide, "| `2` |")
			for _, want := range []string{"`flag given twice: --reviewer`", "go version -m", "make install-fixprguard"} {
				if !strings.Contains(exit2, want) {
					t.Errorf("the exit code 2 row does not mention %s", want)
				}
			}
			sec9 := section(guide, "## 9.")
			if !strings.Contains(sec9, tc.proviso) {
				t.Errorf("section 9 does not state the Protocol proviso %q", tc.proviso)
			}
			for what, line := range map[string]string{
				"the trusted-author item": singleLine(t, tc.rel, guide, tc.trustedItem),
				"the --reviewer row":      flagRow,
			} {
				if !strings.Contains(line, reviewerListName) {
					t.Errorf("%s does not name %s", what, reviewerListName)
				}
			}
			if !strings.Contains(sec9, "`"+contentGuard+"`") {
				t.Errorf("section 9 does not name the content guard %s", contentGuard)
			}
			// Section 9 tells a maintainer to edit these by name, so they must
			// still exist under that name.
			guard := readRepoFile(t, contentGuard)
			for ident, decl := range map[string]string{
				"wantReviewers":                    "var wantReviewers ",
				"TestContextReviewerRowsMalformed": "func TestContextReviewerRowsMalformed(",
			} {
				if !strings.Contains(sec9, "`"+ident+"`") {
					t.Errorf("section 9 does not name %s", ident)
				}
				if !strings.Contains(guard, decl) {
					t.Errorf("%s does not declare %s, which section 9 names", contentGuard, ident)
				}
			}
		})
	}
}

// protocolFlag and protocolOutput match the spellings the documents use:
// --protocol N or --protocol=N, and "protocol":N with optional space.
var (
	protocolFlag   = regexp.MustCompile("--protocol[ =]([0-9]+)")
	protocolOutput = regexp.MustCompile(`"protocol":\s*([0-9]+)`)
)

// TestDocumentedProtocolMatches: every protocol number the documents write
// is the binary's Protocol: each --protocol N in both guides and in every
// command document under .claude/commands (prose included, not only code
// blocks), each "protocol":N of the guides' output examples, and the current
// number chapter 4 of each guide states. The guides must write each at least
// once, so a guide that stops writing them is not read as agreeing; that
// fixpr.md and runpr.md write them is TestDocumentedInvocationsParse's check.
func TestDocumentedProtocolMatches(t *testing.T) {
	want := strconv.Itoa(fixprguard.Protocol)
	check := func(rel, doc string, re *regexp.Regexp, required bool) {
		t.Helper()
		found := re.FindAllStringSubmatch(doc, -1)
		if required && len(found) == 0 {
			t.Errorf("%s has no %s", rel, re)
		}
		for _, m := range found {
			if m[1] != want {
				t.Errorf("%s: %q, want %s", rel, m[0], want)
			}
		}
	}
	var commandDocs []string
	err := filepath.WalkDir(filepath.Join(repoRoot, ".claude", "commands"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
			rel, relErr := filepath.Rel(repoRoot, path)
			commandDocs = append(commandDocs, filepath.ToSlash(rel))
			return relErr
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk command documents: %v", err)
	}
	for _, rel := range commandDocs {
		check(rel, readRepoFile(t, rel), protocolFlag, false)
	}
	for _, tc := range []struct {
		rel     string
		current *regexp.Regexp
	}{
		{guideFiles[0], regexp.MustCompile("現在の番号は `([0-9]+)`")},
		{guideFiles[1], regexp.MustCompile("The current number is `([0-9]+)`")},
	} {
		guide := readRepoFile(t, tc.rel)
		check(tc.rel, guide, protocolFlag, true)
		check(tc.rel, guide, protocolOutput, true)
		check(tc.rel, singleLine(t, tc.rel, guide, "| `--protocol N` |"), tc.current, true)
	}
}

// configScope is how a document states the config comparison scope: the
// phrases of the three points every such passage makes, (a) other branches'
// branch settings are not compared (the current branch's and main's are),
// (b) edits that leave the listed values unchanged are not detected, and
// (c) a rewrite of other branches' settings is a residual risk.
type configScope struct {
	otherBranches, unchangedValues, residualRisk string
}

func (c configScope) phrases() []string {
	return []string{c.otherBranches, c.unchangedValues, c.residualRisk}
}

// passage is one place of a document that states the config scope: its text,
// phrases it must hold besides the scope's three, and phrases of the wording
// it replaced, which must be gone. risk, when set, replaces the scope's
// residual-risk phrase for a passage that words it differently.
type passage struct {
	name  string
	text  string
	risk  string
	more  []string
	stale []string
}

// checkPassages fails for a passage that is missing, lacks a phrase, or
// still holds stale wording.
func checkPassages(t *testing.T, rel string, scope configScope, passages []passage) {
	t.Helper()
	for _, p := range passages {
		if p.text == "" {
			t.Errorf("%s: %s not found", rel, p.name)
			continue
		}
		phrases := scope.phrases()
		if p.risk != "" {
			phrases[2] = p.risk
		}
		for _, want := range append(phrases, p.more...) {
			if !strings.Contains(p.text, want) {
				t.Errorf("%s: %s does not say %q", rel, p.name, want)
			}
		}
		for _, old := range p.stale {
			if strings.Contains(p.text, old) {
				t.Errorf("%s: %s still says %q", rel, p.name, old)
			}
		}
	}
}

// TestGuideDescribesConfigScope: in both guides, every passage on the config
// comparison scope makes the three points of configScope: layer (3) of
// chapter 1, inspect and push in chapter 5, the 7.1 row on rewritten hooks
// and config, 7.2, and the git-state-changed row of chapter 8. The 7.1 row
// also says the listing is compared without other branches' settings and
// that git config --list is the only git run before the comparison, and the
// rewritten passages no longer hold the wording they replaced. The phrases
// are the guides' own, Japanese for the Japanese guide.
func TestGuideDescribesConfigScope(t *testing.T) {
	for _, tc := range []struct {
		rel                         string
		scope                       configScope
		row71, row71Old, inspectOld string
		row71More                   []string
		risk72                      string
	}{
		{
			rel: guideFiles[0],
			scope: configScope{
				otherBranches:   "今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*`",
				unchangedValues: "設定の値の一覧を変えない編集",
				residualRisk:    "ほかのブランチの設定の書き換えは、残余のリスク",
			},
			row71:      "| 書き換えられた hooks・config で git を起動しない |",
			row71More:  []string{"を除いた設定の値の一覧を基準値と比べる", "config を比べる前に起動する git は `git config --list` だけ"},
			row71Old:   "git や `gh` を起動する前に基準値と比べる",
			inspectOld: "まず hooks・config・info を基準値と比べ",
			risk72:     "の設定の書き換え（残余のリスク）",
		},
		{
			rel: guideFiles[1],
			scope: configScope{
				otherBranches:   "`branch.<name>.*` settings of other branches (every branch except the current branch and `main`)",
				unchangedValues: "leave the list of configuration values unchanged",
				residualRisk:    "A rewrite of another branch's settings is a residual risk",
			},
			row71:      "| git is not started with rewritten hooks or config |",
			row71More:  []string{"compare the list of configuration values without", "The only git started before the config comparison is `git config --list`"},
			row71Old:   "compare with the baseline before starting git or `gh`",
			inspectOld: "It first compares the hooks, config, and info with the baseline",
			risk72:     "and `main`). This is a residual risk",
		},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			guide := readRepoFile(t, tc.rel)
			checkPassages(t, tc.rel, tc.scope, []passage{
				{name: "layer (3)", text: singleLine(t, tc.rel, guide, "3. **")},
				{name: "inspect", text: subsection(guide, "### `inspect"), stale: []string{tc.inspectOld}},
				{name: "push", text: subsection(guide, "### `push")},
				{name: "the 7.1 row", text: singleLine(t, tc.rel, guide, tc.row71), more: tc.row71More, stale: []string{tc.row71Old}},
				{name: "7.2", text: subsection(guide, "### 7.2"), risk: tc.risk72},
				{name: "the git-state-changed row", text: singleLine(t, tc.rel, guide, "| `git-state-changed` |")},
			})
		})
	}
}

// task0036Docs are the links a note in task 0030's documents gives to the
// task that changed the scope rules, relative to 0030's directory.
var task0036Docs = []string{
	"../0036_fixprguard_exec_file_scope/01_requirements.md",
	"../0036_fixprguard_exec_file_scope/02_architecture.md",
}

// commentRecord matches the start of one record in a Comments cell: its date
// and its kind.
var commentRecord = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \(([a-z]+)\): `)

// scopeRequirement ends the level-4 heading of task 0030's requirements that
// states the change scope. The guard finds the heading by its title.
const scopeRequirement = ": 変更範囲と作業ツリーの確認\n"

// TestTask0030NotesScopeChange: task 0030's architecture 3.6 and the change
// scope requirement of its requirements state the scope rules as 0030 decided
// them. Each carries a quoted note linking task 0036's requirements and
// architecture, both documents stay approved, and each Comments cell has an
// editorial record that names #425, the issue of task 0036.
func TestTask0030NotesScopeChange(t *testing.T) {
	const dir = "docs/tasks/0030_fixpr_untrusted_threads/"
	for _, tc := range []struct {
		rel     string
		name    string
		section func(doc string) string
	}{
		{rel: dir + "02_architecture.md", name: "3.6", section: func(doc string) string { return subsection(doc, "### 3.6.") }},
		{rel: dir + "01_requirements.md", name: "the change scope requirement", section: func(doc string) string {
			for line := range strings.Lines(doc) {
				if strings.HasPrefix(line, "#### ") && strings.HasSuffix(line, scopeRequirement) {
					return cutSection(doc, line, "\n## ", "\n### ", "\n#### ")
				}
			}
			return ""
		}},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			doc := readRepoFile(t, tc.rel)
			text := tc.section(doc)
			if text == "" {
				t.Fatalf("%s: %s not found", tc.rel, tc.name)
			}
			noted := false
			for line := range strings.Lines(text) {
				if strings.HasPrefix(line, "> ") && containsAll(line, task0036Docs) {
					noted = true
				}
			}
			if !noted {
				t.Errorf("%s: %s has no quoted note linking %v", tc.rel, tc.name, task0036Docs)
			}
			if status := singleLine(t, tc.rel, doc, "| Status |"); !strings.Contains(status, "`approved`") {
				t.Errorf("%s: status is not approved: %q", tc.rel, status)
			}
			if !hasRecord(singleLine(t, tc.rel, doc, "| Comments |"), "editorial", "#425") {
				t.Errorf("%s: Comments has no editorial record naming #425", tc.rel)
			}
		})
	}
}

// hasRecord reports whether the Comments cell has a record of the kind whose
// text names ref.
func hasRecord(comments, kind, ref string) bool {
	starts := commentRecord.FindAllStringSubmatchIndex(comments, -1)
	for i, m := range starts {
		end := len(comments)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		if comments[m[2]:m[3]] == kind && strings.Contains(comments[m[1]:end], ref) {
			return true
		}
	}
	return false
}

// task0034Requirements is the link task 0030's documents give to the
// requirements of task 0034, relative to 0030's directory.
const task0034Requirements = "../0034_fixprguard_ignored_files/01_requirements.md"

// remainingIgnoredRisks are the phrases of the residual risks task 0034's
// requirements 4.2 leaves for ignored files, in the wording both of task
// 0030's documents use for them.
var remainingIgnoredRisks = []string{
	"監視の一覧に当たらない、git が無視するファイルを変えること",
	"この作業ツリーの外のファイルを変えること",
	"通常のファイルでないもの（FIFO・ソケット・デバイスなど）を新しく作ること",
	"完全な大文字小文字の変換だけで監視の一覧の項目と同じになる名前",
	"確認の前に元に戻すこと",
	"`push` の後に変えること",
	"検出の前に効く書き込み",
	"まとめた値からの推測",
	"中身を確かめずに `begin` を実行すること",
}

// TestTask0030DescribesIgnoredFiles: the "ignored files" item of task 0030's
// architecture 5.3 and the ignored-file residual risk of its requirements 4.2
// link task 0034's requirements, name its issue #424, and list each residual
// risk 0034 leaves; the requirements item still names the network risk, and
// the architecture item no longer says an ignored _test.go or a file hidden
// by the global exclude file goes undetected. The other passage each
// document states the ignored-file scope in, layer (3) of the requirements
// 4.2 and the change count of the architecture 3.6, says 0034 detects the
// watch-listed files.
// Both documents stay approved, and each Comments cell has a decision record
// naming #424. The guard checks that the words are there; whether the scope
// matches 0034's requirements 4.2 is checked in review.
func TestTask0030DescribesIgnoredFiles(t *testing.T) {
	const dir = "docs/tasks/0030_fixpr_untrusted_threads/"
	for _, tc := range []struct {
		rel   string
		name  string
		item  func(doc string) string
		more  []string
		stale []string
		// other is the other passage on the ignored-file scope, and
		// otherSays the phrase it must hold.
		other     func(doc string) string
		otherSays string
	}{
		{
			rel:  dir + "02_architecture.md",
			name: "the 5.3 ignored files item",
			// The item runs to the next top-level bullet; its sub-bullets
			// are indented.
			item: func(doc string) string {
				return cutSection(subsection(doc, "### 5.3."), "-   **無視されるファイル。**", "\n-   ")
			},
			stale: []string{
				"`make test` が実行することは、検出できない",
				"グローバルの除外のファイルで新しいファイルを隠すことは、検出しない",
			},
			other:     func(doc string) string { return subsection(doc, "### 3.6.") },
			otherSays: "監視の一覧に当たるものはタスク 0034 が検出する",
		},
		{
			rel:  dir + "01_requirements.md",
			name: "the 4.2 ignored-file residual risk",
			// The item is the residual-risk bullet on ignored files, with its
			// own line and its more-indented sub-bullets.
			item: func(doc string) string {
				sec := subsection(doc, "### 4.2.")
				for line := range strings.Lines(sec) {
					if strings.HasPrefix(line, "    -   ") && strings.Contains(line, "git が無視するファイル") {
						return line + cutSection(sec, line, "\n    -   ")
					}
				}
				return ""
			},
			more:      []string{"ネットワークに接続すること"},
			stale:     []string{"git が無視するファイル（`.envrc` など）を変えること"},
			other:     func(doc string) string { return subsection(doc, "### 4.2.") },
			otherSays: "タスク 0034 の後は、git が無視するファイルのうち監視の一覧に当たるものの変化も検出する",
		},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			doc := readRepoFile(t, tc.rel)
			text := tc.item(doc)
			requireAll(t, tc.rel, tc.name, text, append([]string{task0034Requirements, "#424"}, append(remainingIgnoredRisks, tc.more...)...)...)
			for _, old := range tc.stale {
				if strings.Contains(text, old) {
					t.Errorf("%s: %s still says %q", tc.rel, tc.name, old)
				}
			}
			requireAll(t, tc.rel, "the other ignored-file passage", tc.other(doc), tc.otherSays)
			if status := singleLine(t, tc.rel, doc, "| Status |"); !strings.Contains(status, "`approved`") {
				t.Errorf("%s: status is not approved: %q", tc.rel, status)
			}
			if !hasRecord(singleLine(t, tc.rel, doc, "| Comments |"), "decision", "#424") {
				t.Errorf("%s: Comments has no decision record naming #424", tc.rel)
			}
		})
	}
}

// containsAll reports whether s contains every one of subs.
func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// TestGuideDescribesMentionCheck: both guides name, in 7.1 and 7.2, the
// mention reply and edit-pr reject, taking the value from internal/fixprguard,
// so changing the constant without the guides fails here. Whether the prose
// around it is right is checked by review, not pinned by wording.
func TestGuideDescribesMentionCheck(t *testing.T) {
	mention := "`" + fixprguard.CodexMention + "`"
	for _, rel := range guideFiles {
		guide := readRepoFile(t, rel)
		for _, heading := range []string{"### 7.1", "### 7.2"} {
			if !strings.Contains(subsection(guide, heading), mention) {
				t.Errorf("%s %s does not name %s", rel, heading, mention)
			}
		}
	}
}

// TestCutSection: the extraction the guards read through stops where it
// should. A section that ran on would let a later passage supply a phrase
// the checked one lacks.
func TestCutSection(t *testing.T) {
	doc := "intro\n## A\na\n### B\nb\n### C\nc\n## D\nd\n### E\ne\n"
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"section_runs_past_level_3", section(doc, "## A"), "\na\n### B\nb\n### C\nc"},
		{"subsection_stops_at_level_3", subsection(doc, "### B"), "\nb"},
		// A level-3 heading follows, but the level-2 one comes first.
		{"subsection_stops_at_level_2", subsection(doc, "### C"), "\nc"},
		{"section_runs_to_the_end", section(doc, "## D"), "\nd\n### E\ne\n"},
		{"missing_heading", subsection(doc, "### F"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

// codeSpan matches one inline code span.
var codeSpan = regexp.MustCompile("`([^`]+)`")

// codeSpans returns the set of code spans in text.
func codeSpans(text string) map[string]struct{} {
	spans := map[string]struct{}{}
	for _, m := range codeSpan.FindAllStringSubmatch(text, -1) {
		spans[m[1]] = struct{}{}
	}
	return spans
}

// TestGuideListsWatchList: in both guides, the table after the watch list
// marker in 7.1 lists exactly the patterns WatchListPatterns returns, no
// more and no fewer, and 7.1 has the row stating the guarantee. Only the
// table is read: most items also appear in chapter 6 and in prose, which
// would hide an item missing from the table.
func TestGuideListsWatchList(t *testing.T) {
	want := map[string]struct{}{}
	for _, p := range fixprguard.WatchListPatterns() {
		want[p] = struct{}{}
	}
	for _, tc := range []struct{ rel, marker, row string }{
		{guideFiles[0], "**監視の一覧。**", "| git が無視するファイルのうち監視の一覧に当たるものの作成・書き換え・削除を、`inspect`・`finish`・`push` が基準値と比べて検出する |"},
		{guideFiles[1], "**Watch list.**", "| The creation, rewrite, or removal of a file git ignores that is on the watch list is detected by `inspect`, `finish`, and `push` comparing with the baseline |"},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			guarantees := subsection(readRepoFile(t, tc.rel), "### 7.1")
			table := guideTable(guarantees, tc.marker)
			if got := codeSpans(table); !maps.Equal(got, want) {
				t.Errorf("the watch list table lists %v, want %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
			}
			singleLine(t, tc.rel, guarantees, tc.row)
		})
	}
}

// TestGuideNamesGitStateParts: the list of parts in the git-state-changed
// row of chapter 8 in both guides names every value parts can hold, and
// fixpr.md's list of the parts a git-state-changed stop names includes
// ignored. Only the list is read, since the row's other cells name parts
// too. fixpr.md words the other parts (HEAD, the remote branch, ...), so only
// ignored is pinned there.
func TestGuideNamesGitStateParts(t *testing.T) {
	for _, tc := range []struct{ rel, open, closing string }{
		{guideFiles[0], "`parts`（", "）"},
		{guideFiles[1], "`parts` (", ")"},
	} {
		row := singleLine(t, tc.rel, readRepoFile(t, tc.rel), "| `git-state-changed` |")
		_, rest, ok := strings.Cut(row, tc.open)
		list, _, closed := strings.Cut(rest, tc.closing)
		if !ok || !closed {
			t.Errorf("%s: the git-state-changed row has no list after %q", tc.rel, tc.open)
			continue
		}
		spans := codeSpans(list)
		for _, part := range fixprguard.GitStateParts() {
			if _, ok := spans[part]; !ok {
				t.Errorf("%s: the git-state-changed row's parts do not name `%s`", tc.rel, part)
			}
		}
	}
	const fixprDoc = ".claude/commands/fixpr.md"
	const partsList = "name the `parts` that differ from the baseline ("
	_, rest, ok := strings.Cut(readRepoFile(t, fixprDoc), partsList)
	list, _, closed := strings.Cut(rest, ")")
	if !ok || !closed || !strings.Contains(list, "`"+fixprguard.PartIgnored+"`") {
		t.Errorf("%s: the list after %q does not name `%s`", fixprDoc, partsList, fixprguard.PartIgnored)
	}
}

// requireAll fails for each phrase text lacks; an empty text fails once.
func requireAll(t *testing.T, rel, name, text string, phrases ...string) {
	t.Helper()
	if text == "" {
		t.Errorf("%s: %s not found", rel, name)
		return
	}
	for _, p := range phrases {
		if !strings.Contains(text, p) {
			t.Errorf("%s: %s does not say %q", rel, name, p)
		}
	}
}

// TestGuideDescribesIgnoredFiles: both guides state, in 7.2, the hidden-file
// items split as the requirements ask and the residual risks of the target
// files; in 8.1, steps (a) to (e) with both SHA-256 commands; and in 8.2
// every rule IgnoredFailureRules names, which the git-failed row points to.
// fixpr.md recognizes an ignored-file rule by "contains ignored, or is .git
// name below .claude", so every rule must fit that. The guards check that the
// words are there; whether the prose is right is checked in review. The
// phrases are the guides' own, Japanese for the Japanese guide.
func TestGuideDescribesIgnoredFiles(t *testing.T) {
	const claudeGitRule = ".git name below .claude"
	for _, rule := range fixprguard.IgnoredFailureRules() {
		if !strings.Contains(rule, fixprguard.PartIgnored) && rule != claudeGitRule {
			t.Errorf("rule %q contains neither %q nor is %q, so fixpr.md would not recognize it", rule, fixprguard.PartIgnored, claudeGitRule)
		}
	}
	for _, tc := range []struct {
		rel                       string
		hidden, risks, steps      []string
		gitFailedRef, failedTable string
	}{
		{
			rel: guideFiles[0],
			hidden: []string{
				"監視の一覧（7.1）に当たらない、git が無視するファイルの書き換え",
				"グローバルの除外のファイルで隠したファイルのうち、対象のファイルでないもの",
				"index の stat の偽造で隠した変更",
			},
			risks: []string{
				"監視の一覧に当たらない、git が無視するファイルを変えること",
				"この作業ツリーの外のファイルを変えること",
				"通常のファイルでないもの（FIFO・ソケット・デバイスなど）を新しく作ること",
				"完全な大文字小文字の変換だけで監視の一覧の項目と同じになる名前",
				"確認の前に元に戻すこと",
				"検出の前に効く書き込み",
				"まとめた値からの推測",
				"記録のファイルの削除と書き換え",
				"利用者が中身を確かめずに `begin` を実行すること",
				"ディレクトリを指すシンボリックリンク",
				"`.claude/worktrees/`",
			},
			steps: []string{
				"**(a) 記録の照合と (b) 変わったファイルの特定。**",
				"**(c) 内容を確かめるファイル。**",
				"**(d) 作業ツリーに残った変更と、作られた commit。**",
				"**(e) 正当な原因の場合。**",
				"Claude Code のセッションの外の端末",
				"更新時刻で絞り込まない",
				"侵害を強く疑う",
				"信頼できる写しから戻す",
				"このラウンドの記録はない",
				"その commit を調べて取り消す",
			},
			gitFailedRef: "標準エラーの規則の名前で分かる（8.2）",
			failedTable:  "| 規則の名前 | 原因 | 対処 |",
		},
		{
			rel: guideFiles[1],
			hidden: []string{
				"Rewrites of files git ignores that are not on the watch list (7.1)",
				"or by the global exclude file that are not target files",
				"Changes hidden by forging the stat information of the index",
			},
			risks: []string{
				"Changing a file git ignores that is not on the watch list",
				"Changing files outside this work tree",
				"something that is not a regular file (a FIFO, a socket, a device, and so on)",
				"equals an item of the watch list only under full case folding",
				"changing it back before the check",
				"A write that takes effect before the detection",
				"Inference from the summarized value",
				"Removing or rewriting a record file",
				"The user running `begin` on a redo without examining the contents",
				"a symbolic link to a directory",
				"`.claude/worktrees/`",
			},
			steps: []string{
				"**(a) Checking the record and (b) finding the changed files.**",
				"**(c) Files whose content to examine.**",
				"**(d) Changes left in the work tree, and a commit made.**",
				"**(e) When the cause is legitimate.**",
				"a terminal outside the Claude Code session",
				"do not narrow the files to check by modification time",
				"strongly suspect a compromise",
				"Restore `.claude/settings.local.json`, `.envrc`, and `.env` from trusted copies",
				"this round has no records",
				"examine that commit and undo it",
			},
			gitFailedRef: "which the rule name on standard error shows (8.2)",
			failedTable:  "| Rule name | Cause | What to do |",
		},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			guide := readRepoFile(t, tc.rel)
			requireAll(t, tc.rel, "7.2.3", cutSection(guide, "#### 7.2.3", "\n## ", "\n### ", "\n#### "), tc.hidden...)
			requireAll(t, tc.rel, "7.2.5", cutSection(guide, "#### 7.2.5", "\n## ", "\n### ", "\n#### "), tc.risks...)
			requireAll(t, tc.rel, "8.1", subsection(guide, "### 8.1"), tc.steps...)
			requireAll(t, tc.rel, "the git-failed row", singleLine(t, tc.rel, guide, "| `git-failed` |"), tc.gitFailedRef)
			table := guideTable(subsection(guide, "### 8.2"), tc.failedTable)
			// A "|" inside a cell, even in a code span, splits the cell and
			// drops the rest of a command from the rendered table.
			for line := range strings.Lines(table) {
				if n := strings.Count(line, "|"); n != 4 {
					t.Errorf("%s: 8.2 table row has %d pipes, want 4 (three cells): %q", tc.rel, n, line)
				}
			}
			spans := codeSpans(table)
			for _, rule := range fixprguard.IgnoredFailureRules() {
				if _, ok := spans[rule]; !ok {
					t.Errorf("%s: the 8.2 table does not list `%s`", tc.rel, rule)
				}
			}
		})
	}
}

// diffLine is one line of the 8.1 block's diff of two records: a change
// command, the separator, or a whole record line on either side.
var diffLine = regexp.MustCompile(`^([0-9,]+[acd][0-9,]+|---|[<>] [0-9a-f]{64} "[ -~]*")$`)

// guideBlock returns the first sh code block of the guide's subsection.
func guideBlock(t *testing.T, rel, heading string) string {
	t.Helper()
	_, rest, ok := strings.Cut(subsection(readRepoFile(t, rel), heading), "\n```sh\n")
	block, _, closed := strings.Cut(rest, "\n```\n")
	if !ok || !closed {
		t.Fatalf("%s %s has no sh code block", rel, heading)
	}
	return block + "\n"
}

// TestGuideIgnoredCommands runs the record check of 8.1 as the user would,
// on records begin and inspect wrote: the block is the same in both guides;
// with the token begin printed it reports a match and diff names the path
// that changed; with the begin record rewritten it reports no match and runs
// no diff. The block prefers sha256sum and falls back to shasum, so it is run
// once with each alone on PATH; each installed tool runs, and at least one
// must be installed.
func TestGuideIgnoredCommands(t *testing.T) {
	block := guideBlock(t, guideFiles[0], "### 8.1")
	if en := guideBlock(t, guideFiles[1], "### 8.1"); en != block {
		t.Fatalf("the 8.1 code blocks differ:\n%s\n%s", block, en)
	}
	// pathWith is a PATH directory holding the block's tools and one of
	// the two SHA-256 commands, so the block takes that command's branch.
	pathWith := func(sha string) (string, bool) {
		dir := t.TempDir()
		for _, tool := range []string{sha, "sed", "cut", "diff", "cat"} {
			target, err := exec.LookPath(tool)
			if err != nil {
				return "", false
			}
			if err := os.Symlink(target, filepath.Join(dir, tool)); err != nil {
				t.Fatal(err)
			}
		}
		return dir, true
	}
	var paths []string
	for _, sha := range []string{"sha256sum", "shasum"} {
		if dir, ok := pathWith(sha); ok {
			paths = append(paths, dir)
		}
	}
	if len(paths) == 0 {
		t.Fatal("unsupported test environment: neither sha256sum nor shasum is installed")
	}
	repo := newTestRepo(t)
	repo.Ignore("build/")
	repo.Write("build/kept.go", "package x\n")
	repo.Write("build/changed.go", "package x\n")
	r := newTestRun(t, beginSteps(threadsPage(t, false)))
	if code := r.Run(t, args(cmdBegin, "--reviewer", testReviewer, testPR)...); code != exitContinue {
		t.Fatalf("begin: exit %d (%s)", code, r.Stderr)
	}
	var out beginJSON
	r.Decode(t, &out)
	repo.Write("build/changed.go", "package y\n")
	inspect := newTestRun(t, nil)
	if code := inspect.Run(t, args(cmdInspect, "--reviewer", testReviewer, "--baseline", out.Baseline, testPR)...); code != exitStop {
		t.Fatalf("inspect: exit %d, want %d (%s)", code, exitStop, inspect.Stderr)
	}
	runBlock := func(path string) string {
		t.Helper()
		cmd := exec.Command("sh", "-c", block) //nolint:gosec // the guide's own code block, run in the test's repository
		cmd.Dir = repo.Dir
		cmd.Env = append(os.Environ(), "token="+out.Baseline, "PATH="+path)
		got, err := cmd.Output()
		// diff exits 1 when the records differ; any other failure is one.
		if exitErr, ok := errors.AsType[*exec.ExitError](err); err != nil && (!ok || exitErr.ExitCode() != 1) {
			t.Fatalf("run the block: %v\n%s", err, got)
		}
		return string(got)
	}
	for _, path := range paths {
		got := runBlock(path)
		if !strings.HasPrefix(got, "begin record: matches the baseline\n") || !strings.Contains(got, `"build/changed.go"`) || strings.Contains(got, `"build/kept.go"`) {
			t.Errorf("PATH %s: block output = %q, want the match and a diff naming build/changed.go only", path, got)
		}
		// Each diff line is whole: a tool that wraps long lines (GNU sed's
		// l does) would split a record line inside its path.
		for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n")[1:] {
			if !diffLine.MatchString(line) {
				t.Errorf("PATH %s: diff line %q is not a whole diff or record line", path, line)
			}
		}
	}
	f, err := os.OpenFile(beginRecordName, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("0 \"forged\"\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if got := runBlock(path); got != "begin record: does not match the baseline, do not trust it\n" {
			t.Errorf("PATH %s: block output with the begin record rewritten = %q", path, got)
		}
	}
}
