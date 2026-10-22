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

// TestGuideDescribesReviewerList: both guides describe repeated --reviewer as
// the tool implements it. The subcommand headings that write
// "--reviewer LOGIN ..." are exactly the subcommands whose spec takes
// --reviewer; the flag row says the flag repeats once per name and that none
// or a name given twice is a usage error; other-bot is a Bot matching none of
// the names; the flag-position row lists --reviewer as repeatable; the exit
// code 2 row identifies a tool that predates the change and how to check its
// version; and section 9 states the Protocol proviso.
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
		flagRow     []string
		positionRow string
		otherBot    string
		proviso     string
	}{
		{
			rel:         guideFiles[0],
			positionRow: "| フラグの位置 |",
			flagRow:     []string{"名前ごとに 1 回ずつ繰り返して渡す", "1 つもないとき", "同じ名前を 2 回以上渡したとき", "使い方の誤り"},
			otherBot:    "`--reviewer` で渡したどの名前とも一致しない Bot",
			proviso:     "次の 3 つの条件がすべて成り立つ変更に限り、`Protocol` を上げなくてよい",
		},
		{
			rel:         guideFiles[1],
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
			if !strings.Contains(section(guide, "## 9."), tc.proviso) {
				t.Errorf("section 9 does not state the Protocol proviso %q", tc.proviso)
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
