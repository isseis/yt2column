//go:build test

package main

import (
	"encoding/json"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/numreserve"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

// toolWord is the tool's name, its install target's suffix, and its package
// directory under cmd/.
const toolWord = "numreserve"

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	return doctestutil.ReadDoc(t, filepath.Join(repoRoot, filepath.FromSlash(rel)))
}

// TestInstallTargetInMakefile checks make install-numreserve installs this
// package, the way the user installs the tool from main.
func TestInstallTargetInMakefile(t *testing.T) {
	target := "install-" + toolWord
	lines := strings.Split(readRepoFile(t, "Makefile"), "\n")
	i := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, target+":") })
	if i < 0 || i+1 >= len(lines) || lines[i+1] != "\t$(GOCMD) install ./cmd/"+toolWord {
		t.Errorf("Makefile has no %s target whose recipe is $(GOCMD) install ./cmd/%s", target, toolWord)
	}
}

// bashRule matches a Bash permission rule and captures its command pattern.
var bashRule = regexp.MustCompile(`^Bash\((.*)\)$`)

// allowedRules are the only permission rules for the tool: one per space,
// each the whole command line, so no other argument runs unprompted.
var allowedRules = []string{"Bash(" + toolWord + " task)", "Bash(" + toolWord + " adr)"}

// namesTool reports whether a permission rule is a Bash rule whose command
// word is the tool, by name, by path, or quoted.
func namesTool(rule string) bool {
	m := bashRule.FindStringSubmatch(rule)
	if m == nil {
		return false
	}
	word, _, _ := strings.Cut(strings.TrimSpace(m[1]), " ")
	word, _, _ = strings.Cut(word, ":")
	return path.Base(strings.Trim(word, `"'`)) == toolWord
}

// TestNamesTool: the rule reader finds the tool's rules in every spelling
// and no rule of another word.
func TestNamesTool(t *testing.T) {
	for _, tc := range []struct {
		rule string
		want bool
	}{
		{"Bash(numreserve task)", true},
		{"Bash(numreserve adr)", true},
		{"Bash(numreserve *)", true},
		{"Bash(numreserve:*)", true},
		{"Bash(numreserve task *)", true},
		{"Bash(/usr/local/bin/numreserve task)", true},
		{`Bash("numreserve" adr)`, true},
		{"Bash(numreservex task)", false},
		{"Bash(chief *)", false},
		{"Read(numreserve)", false},
	} {
		if got := namesTool(tc.rule); got != tc.want {
			t.Errorf("namesTool(%q) = %v, want %v", tc.rule, got, tc.want)
		}
	}
}

// toolRuleMismatches returns the tool's rules of allow that are not one of
// allowedRules, and the allowed rules allow lacks.
func toolRuleMismatches(allow []string) (extra, missing []string) {
	for _, rule := range allow {
		if namesTool(rule) && !slices.Contains(allowedRules, rule) {
			extra = append(extra, rule)
		}
	}
	for _, rule := range allowedRules {
		if !slices.Contains(allow, rule) {
			missing = append(missing, rule)
		}
	}
	return extra, missing
}

// TestToolRuleMismatches: only the two fixed rules pass; a wider rule for
// the tool is extra, and a list without one of the two is missing it.
func TestToolRuleMismatches(t *testing.T) {
	for _, tc := range []struct {
		name           string
		allow          []string
		extra, missing []string
	}{
		{"exact", []string{"Bash(chief *)", "Bash(numreserve task)", "Bash(numreserve adr)"}, nil, nil},
		{"wildcard", []string{"Bash(numreserve task)", "Bash(numreserve adr)", "Bash(numreserve *)"}, []string{"Bash(numreserve *)"}, nil},
		{"colon", []string{"Bash(numreserve task)", "Bash(numreserve adr)", "Bash(numreserve:*)"}, []string{"Bash(numreserve:*)"}, nil},
		{"trailing_args", []string{"Bash(numreserve task *)", "Bash(numreserve adr)"}, []string{"Bash(numreserve task *)"}, []string{"Bash(numreserve task)"}},
		{"none", []string{"Bash(chief *)"}, nil, allowedRules},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extra, missing := toolRuleMismatches(tc.allow)
			if !slices.Equal(extra, tc.extra) || !slices.Equal(missing, tc.missing) {
				t.Errorf("toolRuleMismatches(%q) = %q, %q, want %q, %q", tc.allow, extra, missing, tc.extra, tc.missing)
			}
		})
	}
}

// TestSettingsAllowsTool: .claude/settings.json allows the tool in exactly
// the two documented forms, so a call with any other argument, which would
// create a record tag the procedures did not ask for, still prompts.
func TestSettingsAllowsTool(t *testing.T) {
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, ".claude/settings.json")), &settings); err != nil {
		t.Fatalf("parse .claude/settings.json: %v", err)
	}
	extra, missing := toolRuleMismatches(settings.Permissions.Allow)
	for _, rule := range extra {
		t.Errorf("permissions.allow has %q; only %q may name %s", rule, allowedRules, toolWord)
	}
	for _, rule := range missing {
		t.Errorf("permissions.allow lacks %q", rule)
	}
}

// callIssue returns why call is not a call the permission rules allow and
// the tool accepts, or "" when it is one: the bare tool name alone in its
// command, with exactly one argument that names a space.
func callIssue(call doctestutil.Invocation) string {
	if !doctestutil.PromptFree(call, toolWord) {
		return "not the bare tool name alone in its command"
	}
	if len(call.Args) != 1 {
		return "not exactly one argument"
	}
	if _, err := numreserve.ParseSpace(call.Args[0]); err != nil {
		return "argument is not a number space"
	}
	return ""
}

// TestCallIssue: the call reader accepts the two forms and rejects a
// prefixed, joined, or path call, a missing or extra argument, a flag, and
// an argument that is no space.
func TestCallIssue(t *testing.T) {
	for _, tc := range []struct {
		doc string
		ok  bool
	}{
		{"Run `numreserve task` now.\n", true},
		{"```bash\nnumreserve adr\n```\n", true},
		{"```bash\nenv X=1 numreserve task\n```\n", false},
		{"```bash\ncd x && numreserve task\n```\n", false},
		{"```bash\n~/go/bin/numreserve task\n```\n", false},
		{"```bash\nnumreserve\n```\n", false},
		{"```bash\nnumreserve task adr\n```\n", false},
		{"```bash\nnumreserve --help\n```\n", false},
		{"```bash\nnumreserve tasks\n```\n", false},
	} {
		calls := doctestutil.Invocations(tc.doc, toolWord)
		if len(calls) != 1 {
			t.Fatalf("%q: %d calls, want 1", tc.doc, len(calls))
		}
		if got := callIssue(calls[0]) == ""; got != tc.ok {
			t.Errorf("%q: accepted = %v, want %v (%s)", tc.doc, got, tc.ok, callIssue(calls[0]))
		}
	}
}

// Where the calls live outside the command documents. The headings and the
// paragraph starts are English because the documents are.
const (
	conventionPath        = "docs/dev/conventions/adr.md"
	claudePath            = "CLAUDE.md"
	safetyHeading         = "## Tool Execution Safety"
	namingHeading         = "## Location and naming"
	takingInHeading       = "## Taking in `main`"
	renumberStart         = "2. **Renumber.**"
	failsOnMainStart      = "**When `TestADRRecords` fails on `main`.**"
	mkreqPath             = ".claude/commands/mkreq.md"
	guidePath             = "docs/dev/guides/numreserve_guide.md"
	guideReasonColumn     = "理由の語"
	guideCallColumn       = "呼び出し"
	guideTagColumn        = "記録の tag"
	guideRulesetColumn    = "ruleset の対象"
	rulesetTargetSuffix   = "**/*"
	recordTagNumberMarker = "NNNN"
)

// paragraphAt returns the block of doc that starts with the line starting
// with start, up to the next blank line or numbered list item, or "" when no
// line starts with it.
func paragraphAt(doc, start string) string {
	lines := strings.Split(doc, "\n")
	i := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, start) })
	if i < 0 {
		return ""
	}
	end := i + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" && !numberedItem.MatchString(lines[end]) {
		end++
	}
	return strings.Join(lines[i:end], "\n")
}

// numberedItem is the first line of a numbered list item.
var numberedItem = regexp.MustCompile(`^[0-9]+\. `)

// spacesCalled returns the spaces of the calls in text, in order.
func spacesCalled(text string) []string {
	var args []string
	for _, call := range doctestutil.Invocations(text, toolWord) {
		args = append(args, strings.Join(call.Args, " "))
	}
	return args
}

// TestDocumentedInvocations: every call of the tool an agent follows (the
// command documents, the ADR convention, and CLAUDE.md's Tool Execution
// Safety) is in a form the permission rules allow and the tool accepts, and
// each place a procedure decides a number holds its call. Only the Tool
// Execution Safety section of CLAUDE.md is read: its other sections name
// ./cmd/numreserve in go install and go test lists, which are not calls.
func TestDocumentedInvocations(t *testing.T) {
	convention := readRepoFile(t, conventionPath)
	safety := doctestutil.SectionOutsideBlocks(readRepoFile(t, claudePath), safetyHeading)
	if safety == "" {
		t.Fatalf("%s has no %q section", claudePath, safetyHeading)
	}
	texts := map[string]string{conventionPath: convention, claudePath + " " + safetyHeading: safety}
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		texts[rel] = readRepoFile(t, rel)
	}
	for name, text := range texts {
		for _, call := range doctestutil.Invocations(text, toolWord) {
			if issue := callIssue(call); issue != "" {
				t.Errorf("%s: %q %s %q: %s", name, call.Lead, call.Word, call.Args, issue)
			}
		}
	}

	taking := doctestutil.SectionOutsideBlocks(convention, takingInHeading)
	for _, tc := range []struct {
		place string
		text  string
		want  []string
	}{
		{mkreqPath, texts[mkreqPath], []string{"task"}},
		{conventionPath + " " + namingHeading, doctestutil.SectionOutsideBlocks(convention, namingHeading), []string{"adr"}},
		{conventionPath + " " + renumberStart, paragraphAt(taking, renumberStart), []string{"adr"}},
		{conventionPath + " " + failsOnMainStart, paragraphAt(taking, failsOnMainStart), []string{"adr"}},
		{claudePath + " " + safetyHeading, safety, []string{"task", "adr"}},
	} {
		got := spacesCalled(tc.text)
		for _, space := range tc.want {
			if !slices.Contains(got, space) {
				t.Errorf("%s holds no %s %s call (calls: %q)", tc.place, toolWord, space, got)
			}
		}
	}
}

// TestParagraphAt: a paragraph ends at a blank line or at the next numbered
// item, and a missing start gives "".
func TestParagraphAt(t *testing.T) {
	doc := "1. one\n2. **two** a\n   b\n3. three\n\n**P.** x\ny\n\nz\n"
	for _, tc := range []struct{ start, want string }{
		{"2. **two**", "2. **two** a\n   b"},
		{"**P.**", "**P.** x\ny"},
		{"**Q.**", ""},
	} {
		if got := paragraphAt(doc, tc.start); got != tc.want {
			t.Errorf("paragraphAt(%q) = %q, want %q", tc.start, got, tc.want)
		}
	}
}

// columnValues returns the cells of column name in the guide's first table
// that has it, without backticks.
func columnValues(t *testing.T, table [][]string, name string) []string {
	t.Helper()
	i := doctestutil.ColumnIndex(table[0], name)
	if i < 0 {
		t.Fatalf("%s: table has no column %q", guidePath, name)
	}
	var values []string
	for _, row := range table[1:] {
		if i >= len(row) {
			t.Fatalf("%s: row %q has no column %q", guidePath, row, name)
		}
		values = append(values, doctestutil.StripBackticks(row[i]))
	}
	return values
}

// TestGuideValuesMatchTool: the guide's table of stop reasons lists every
// reason word the tool prints, once each, and nothing else; and its table of
// spaces gives, for each space, the call, the record tag, and the ruleset
// target the tool's record namespace defines. A user follows the guide to
// read a stop and to set the ruleset, so a word or a target that differs
// from the tool would mislead the stop handling or leave the records
// unprotected.
func TestGuideValuesMatchTool(t *testing.T) {
	guide := readRepoFile(t, guidePath)

	reasons := doctestutil.FindTable(guide, guideReasonColumn)
	if reasons == nil {
		t.Fatalf("%s has no table with a %q column", guidePath, guideReasonColumn)
	}
	var want []string
	for _, kind := range numreserve.StopKinds() {
		reason, ok := kind.Reason()
		if !ok {
			t.Fatalf("stop kind %d has no reason word", kind)
		}
		want = append(want, reason)
	}
	got := columnValues(t, reasons, guideReasonColumn)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("%s: reason words %q, want %q", guidePath, got, want)
	}

	spaces := doctestutil.FindTable(guide, guideRulesetColumn)
	if spaces == nil {
		t.Fatalf("%s has no table with a %q column", guidePath, guideRulesetColumn)
	}
	calls := columnValues(t, spaces, guideCallColumn)
	tags := columnValues(t, spaces, guideTagColumn)
	targets := columnValues(t, spaces, guideRulesetColumn)
	seen := make(map[numreserve.Space]struct{})
	for i, call := range calls {
		arg, ok := strings.CutPrefix(call, toolWord+" ")
		space, err := numreserve.ParseSpace(arg)
		if !ok || err != nil {
			t.Errorf("%s: call %q is not %s with a space", guidePath, call, toolWord)
			continue
		}
		seen[space] = struct{}{}
		prefix, err := space.TagPrefix()
		if err != nil {
			t.Fatal(err)
		}
		if tags[i] != prefix+recordTagNumberMarker {
			t.Errorf("%s: %s record tag %q, want %q", guidePath, call, tags[i], prefix+recordTagNumberMarker)
		}
		if targets[i] != prefix+rulesetTargetSuffix {
			t.Errorf("%s: %s ruleset target %q, want %q", guidePath, call, targets[i], prefix+rulesetTargetSuffix)
		}
	}
	if len(seen) != len(allowedRules) || len(calls) != len(allowedRules) {
		t.Errorf("%s: spaces table has calls %q, want one row for each of %q", guidePath, calls, allowedRules)
	}
}
