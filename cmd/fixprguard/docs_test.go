//go:build test

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
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
// stop kind the tool can print and every execution-related file definition
// the scope rules use, and CLAUDE.md links to the guide. The values come from
// internal/fixprguard, so a kind or a definition added there without the
// guide fails here.
func TestGuideCoversStopKindsAndScope(t *testing.T) {
	for _, rel := range guideFiles {
		guide := readRepoFile(t, rel)
		for _, kind := range fixprguard.StopKinds() {
			if !strings.Contains(guide, "`"+kind+"`") {
				t.Errorf("%s does not name stop kind `%s`", rel, kind)
			}
		}
		for _, pattern := range fixprguard.ExecutionFilePatterns() {
			if !strings.Contains(guide, "`"+pattern+"`") {
				t.Errorf("%s does not list execution-related file `%s`", rel, pattern)
			}
		}
	}
	if claude := readRepoFile(t, "CLAUDE.md"); !strings.Contains(claude, "](docs/dev/developer_guide/fixprguard_guide.md)") {
		t.Error("CLAUDE.md does not link to the fixprguard guide")
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
	return uniqueLine(t, rel, doc, "starting with", prefix, strings.HasPrefix)
}

// lineContaining returns the one line of doc that contains substr, failing
// when there is none or more than one.
func lineContaining(t *testing.T, rel, doc, substr string) string {
	t.Helper()
	return uniqueLine(t, rel, doc, "containing", substr, strings.Contains)
}

// uniqueLine returns the one line of doc for which match(line, s) holds,
// failing when there is none or more than one; how says how s matches.
func uniqueLine(t *testing.T, rel, doc, how, s string, match func(line, s string) bool) string {
	t.Helper()
	var found []string
	for line := range strings.Lines(doc) {
		if match(line, s) {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Errorf("%s has %d lines %s %q, want 1", rel, len(found), how, s)
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

// jaConfigScope is the Japanese wording of the config scope, shared by the
// Japanese guide and task 0030's documents so that the two say it alike.
var jaConfigScope = configScope{
	otherBranches:   "今のブランチと `main` を除く、ほかのブランチの `branch.<name>.*`",
	unchangedValues: "設定の値の一覧を変えない編集",
	residualRisk:    "ほかのブランチの設定の書き換えは、残余のリスク",
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
			rel:        guideFiles[0],
			scope:      jaConfigScope,
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

// task0030Dir holds the requirements and architecture of the task that
// introduced the git state comparison; task 0035 narrowed its config scope.
const task0030Dir = "docs/tasks/0030_fixpr_untrusted_threads/"

// task0035Decision is the dated Comments record of task 0035's decision.
var task0035Decision = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2} \(decision\): タスク 0035（#431）の要件で決めた変更`)

// TestTask0030DocsDescribeConfigScope: in task 0030's requirements and
// architecture, every passage on the config comparison scope makes the three
// points of configScope in the Japanese guide's wording and no longer holds
// the wording it replaced: in the requirements, the chapter 1 decision on
// fix-phase anomalies, the acceptance criterion on git state differences,
// layer (3) of 4.2, and the residual risks of 4.2; in the architecture, the
// push decision of 1.2, the finish checks and push invariants of 3.6, the
// config row and the tool's git of 3.7, the 5.2 row, two items of 5.3, and
// the git state tests of 7.1. Both documents stay approved and record the
// change, with its date, as a decision of task 0035.
func TestTask0030DocsDescribeConfigScope(t *testing.T) {
	for _, tc := range []struct {
		rel      string
		passages func(t *testing.T, rel, doc string) []passage
	}{
		{
			rel: task0030Dir + "01_requirements.md",
			passages: func(t *testing.T, rel, doc string) []passage {
				return []passage{
					{name: "the fix-phase anomaly decision", text: singleLine(t, rel, doc, "-   **修正のフェーズの異常では止める:**")},
					{name: "the git state criterion", text: lineContaining(t, rel, doc, "修正のフェーズの前と後で、次のいずれかが違う場合"), stale: []string{"git の設定ファイル（リポジトリの config と、worktree の config）"}},
					// The bullet ends where the next top-level bullet starts; its
					// nested lines are indented.
					{name: "layer (3)", text: cutSection(doc, "-   守りは 3 つの層からなる。", "\n-   "), stale: []string{"git の hooks と設定ファイルの違い"}},
					{name: "the residual risks", text: cutSection(doc, "-   **残余のリスク:**", "\n### "), stale: []string{"次の 4 つ"}},
				}
			},
		},
		{
			rel: task0030Dir + "02_architecture.md",
			passages: func(t *testing.T, rel, doc string) []passage {
				return []passage{
					{name: "the push decision", text: singleLine(t, rel, doc, "-   **push は、ツールの `push` が直前に再確認して行う。**")},
					{name: "the judged-set check", text: singleLine(t, rel, doc, "-   **判定した集合の厳密な確認。**")},
					{name: "the check after build checks", text: singleLine(t, rel, doc, "-   **build checks の後の確認。**")},
					{name: "the push invariants", text: singleLine(t, rel, doc, "**`push` の不変条件。**")},
					{name: "the config baseline row", text: singleLine(t, rel, doc, "| config |"), stale: []string{"の config の内容のダイジェスト"}},
					{name: "the tool's git", text: singleLine(t, rel, doc, "-   **ツールが起動する git。**"), stale: []string{"ファイルの読み取りだけで求める config と info"}},
					{name: "the 5.2 row", text: singleLine(t, rel, doc, "| 修正のフェーズが HEAD・hooks・config・info を変える |")},
					{name: "the check-to-use gap", text: singleLine(t, rel, doc, "-   **確認と実行の間のずれ。**")},
					{name: "subcommands without a baseline", text: singleLine(t, rel, doc, "-   **基準値を持たないサブコマンドの起動前の比較。**")},
					{name: "the git state tests", text: singleLine(t, rel, doc, "-   **git の状態。**"), stale: []string{"`inspect` が git を起動せずに止まること"}},
				}
			},
		},
	} {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			doc := readRepoFile(t, tc.rel)
			checkPassages(t, tc.rel, jaConfigScope, tc.passages(t, tc.rel, doc))
			if status := singleLine(t, tc.rel, doc, "| Status |"); !strings.Contains(status, "`approved`") {
				t.Errorf("%s: Status is not approved: %q", tc.rel, status)
			}
			if comments := singleLine(t, tc.rel, doc, "| Comments |"); !task0035Decision.MatchString(comments) {
				t.Errorf("%s: Comments does not record the decision of task 0035", tc.rel)
			}
		})
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
