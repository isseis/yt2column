//go:build test

package main

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief"
	"github.com/isseis/yt2column/internal/doctestutil"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	return doctestutil.ReadDoc(t, filepath.Join(repoRoot, filepath.FromSlash(rel)))
}

// statusTable is the document status table a plan needs to be read.
const statusTable = "## Document Status\n\n| Item | Value |\n|---|---|\n| Status | `approved` |\n\n"

// codeSpan matches a code span.
var codeSpan = regexp.MustCompile("`([^`]+)`")

// numberedHeading matches a section heading with a number, such as the
// subsection heading mkplan.md step 8.5 names, and not a heading it names to
// count, such as a phase's.
var numberedHeading = regexp.MustCompile(`^### [0-9]+\.`)

// autoSendOf reads the auto-send field of a plan's first PR with the tool's
// plan reader.
func autoSendOf(t *testing.T, plan string) chief.AutoSend {
	t.Helper()
	p, err := chief.ParsePlan([]byte(statusTable+plan), nil)
	if err != nil {
		t.Fatalf("the tool cannot read the plan: %v\n%s", err, plan)
	}
	pr, ok := p.NextPR()
	if !ok {
		t.Fatalf("the plan has no PR to create:\n%s", plan)
	}
	return pr.AutoSend
}

// autoSendValues are the values each documented form is filled with and
// what the tool must read them as.
var autoSendValues = map[string]chief.AutoSend{"auto": chief.AutoSendAuto, "hold": chief.AutoSendHold}

// TestMarkerFormsCarryAutoSend checks that the forms the commands make a plan
// write hold the auto-send field in a form the tool reads: the PR marker of
// mkplan2.md step 6 and the PR structure subsection of mkplan.md step 8.5
// (which _lib/simple-process.md writes through), each filled with both
// values; and that _context.md lists the field among the PR marker's fields
// under the label of those forms.
func TestMarkerFormsCarryAutoSend(t *testing.T) {
	marker := markerForm(t)
	label := fieldLabel(t, marker)
	for value, want := range autoSendValues {
		if got := autoSendOf(t, fillAutoSend(t, marker, label, value)); got != want {
			t.Errorf("mkplan2.md step 6 with %s: the tool reads %d, want %d", value, got, want)
		}
		if got := autoSendOf(t, singleForm(t, label, value)); got != want {
			t.Errorf("mkplan.md step 8.5 with %s: the tool reads %d, want %d", value, got, want)
		}
	}
	fields := contextFields(t)
	if !slices.Contains(fields, strings.TrimSuffix(label, ": ")) {
		t.Errorf("_context.md lists the fields %v, without %q", fields, strings.TrimSuffix(label, ": "))
	}
}

// markerForm returns the PR marker form of mkplan2.md step 6, its indent
// removed and its marker number made 1.
func markerForm(t *testing.T) string {
	t.Helper()
	doc := readRepoFile(t, ".claude/commands/mkplan2.md")
	i := strings.Index(doc, "\n6. Insert `### PR-N ")
	if i < 0 {
		t.Fatal("mkplan2.md has no step 6 that inserts the PR markers")
	}
	blocks := doctestutil.FencedBlocks(doc[i:])
	if len(blocks) == 0 || !blocks[0].Closed {
		t.Fatal("mkplan2.md step 6 has no closed code block")
	}
	var lines []string
	for _, l := range blocks[0].Lines {
		lines = append(lines, strings.TrimPrefix(strings.TrimSuffix(l, "\n"), "   "))
	}
	form := strings.Join(lines, "\n") + "\n"
	return strings.Replace(form, "### PR-N ", "### PR-1 ", 1)
}

// fieldLabel returns the label of the form's field line whose values are
// the auto-send values, such as "**X**: " of "**X**: auto | hold".
func fieldLabel(t *testing.T, form string) string {
	t.Helper()
	var found []string
	for line := range strings.Lines(form) {
		label, values, ok := strings.Cut(strings.TrimSuffix(line, "\n"), ": ")
		if !ok || !strings.HasPrefix(label, "**") {
			continue
		}
		alternatives := strings.Split(values, " | ")
		slices.Sort(alternatives)
		if slices.Equal(alternatives, []string{"auto", "hold"}) {
			found = append(found, label+": ")
		}
	}
	if len(found) != 1 {
		t.Fatalf("the marker form has %d lines of the values auto | hold, want one:\n%s", len(found), form)
	}
	return found[0]
}

// fillAutoSend returns form with its auto-send line given value.
func fillAutoSend(t *testing.T, form, label, value string) string {
	t.Helper()
	line := label + "auto | hold\n"
	if !strings.Contains(form, line) {
		t.Fatalf("the form has no line %q", line)
	}
	return strings.Replace(form, line, label+value+"\n", 1)
}

// singleForm returns the PR structure subsection mkplan.md step 8.5 writes,
// built from the heading and the field lines it names, with the auto-send
// field set to value.
func singleForm(t *testing.T, label, value string) string {
	t.Helper()
	doc := readRepoFile(t, ".claude/commands/mkplan.md")
	start := strings.Index(doc, "\n8.5. ")
	end := strings.Index(doc, "\n9. ")
	if start < 0 || end < start {
		t.Fatal("mkplan.md has no step 8.5 before step 9")
	}
	step := doc[start:end]
	heading := ""
	var fields []string
	for _, m := range codeSpan.FindAllStringSubmatch(step, -1) {
		switch span := m[1]; {
		case numberedHeading.MatchString(span) && heading == "":
			heading = span
		case strings.HasPrefix(span, "**") && strings.Contains(span, "**: "):
			fields = append(fields, span)
		}
	}
	if heading == "" {
		t.Fatal("mkplan.md step 8.5 names no subsection heading")
	}
	var lines []string
	for _, f := range fields {
		if strings.HasPrefix(f, label) {
			f = label + value
		}
		lines = append(lines, f)
	}
	if n := len(slices.DeleteFunc(slices.Clone(lines), func(l string) bool { return l != label+value })); n != 1 {
		t.Fatalf("mkplan.md step 8.5 names the field lines %v, %d of them %q; want one", fields, n, label)
	}
	chapter, _, _ := strings.Cut(strings.TrimPrefix(heading, "### "), ".")
	return "## " + chapter + ". x\n\n" + heading + "\n\n" + strings.Join(lines, "\n\n") + "\n"
}

// contextFields returns the field labels _context.md's PR marker
// conventions list, without their asterisks' content changed.
func contextFields(t *testing.T) []string {
	t.Helper()
	section := doctestutil.SectionByHeading(readRepoFile(t, ".claude/commands/_context.md"), "### PR marker conventions")
	for line := range strings.Lines(section) {
		rest, ok := strings.CutPrefix(line, "- Fields: ")
		if !ok {
			continue
		}
		var fields []string
		for _, m := range codeSpan.FindAllStringSubmatch(rest, -1) {
			fields = append(fields, m[1])
		}
		return fields
	}
	t.Fatal("_context.md's PR marker conventions have no field list")
	return nil
}

// definitionPath is the /chief definition, the document that runs the tool.
const definitionPath = ".claude/commands/chief.md"

// toolWord is the tool's command name, which go install gives the binary
// built from this package's directory.
const toolWord = "chief"

// wantProtocolFlag is the one --protocol form the tool accepts.
func wantProtocolFlag() string { return "--protocol " + strconv.Itoa(chief.Protocol) }

// TestDocumentedProtocolMatches: every --protocol N of a chief call in the
// command documents, prose included, is the binary's Protocol in the form
// the tool reads, so the installed tool refuses a definition written for
// another contract and accepts one written for its own.
func TestDocumentedProtocolMatches(t *testing.T) {
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		for _, f := range doctestutil.ProtocolFlags(readRepoFile(t, rel)) {
			if f.Tool == doctestutil.ToolChief && f.Text != wantProtocolFlag() {
				t.Errorf("%s: %q, want %q", rel, f.Text, wantProtocolFlag())
			}
		}
	}
}

// placeholderValues are the values the definition's placeholders stand for,
// by name: each passes the tool's own check of the flag it is written in.
var placeholderValues = map[string]string{
	"<workspace>": "/w",
	"<request>":   "mkarch",
	"<id>":        strings.Repeat("c", 32),
	"<issue>":     "12",
	"<pr>":        "34",
	"<check>":     strings.Repeat("a", 64),
	"<number>":    "1",
	"<baseline>":  strings.Repeat("b", 64),
}

// placeholder is a value a document leaves for the session to fill in, such
// as <workspace>, alone or inside a value such as 1=<number>.
var placeholder = regexp.MustCompile(`<[A-Za-z_]+>`)

// fillPlaceholders returns args with each placeholder replaced by the value
// it stands for, keeping the text around it, so a call whose fixed part is
// wrong (such as invalid=<number>) still fails the tool's check. It fails t
// for a placeholder with no value.
func fillPlaceholders(t *testing.T, args []string) []string {
	t.Helper()
	filled := slices.Clone(args)
	for i := range filled {
		filled[i] = placeholder.ReplaceAllStringFunc(filled[i], func(p string) string {
			value, ok := placeholderValues[p]
			if !ok {
				t.Errorf("%q: no value for the placeholder %s", args, p)
			}
			return value
		})
	}
	return filled
}

// promptedSubcommand is the one subcommand the settings' ask rule makes
// Claude Code confirm at every call: the removal, which closes a workspace
// and removes a worktree.
const promptedSubcommand = cmdRemove

// askRemove is the ask rule that makes Claude Code confirm every removal.
const askRemove = "Bash(" + toolWord + " " + promptedSubcommand + " *)"

// TestDocumentedInvocationsParse: every chief call in the command documents
// is written in the form the permission rule Bash(chief *) allows (the bare
// tool name alone in its command), and passes the tool's own checks of its
// subcommand, flags, and request with its placeholders filled in, so the
// installed tool runs it instead of returning a usage error. In that bare
// form a call of promptedSubcommand begins with the prefix of askRemove,
// the settings' only ask rule (TestSettingsAsksBeforeRemove), so Claude
// Code's permission prompt appears at every documented removal and at no
// other call. The /chief definition must hold a call of every subcommand:
// one that lost a call would pass every check of the calls left.
func TestDocumentedInvocationsParse(t *testing.T) {
	called := map[string]struct{}{}
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		for _, call := range doctestutil.Invocations(readRepoFile(t, rel), toolWord) {
			if !doctestutil.PromptFree(call, toolWord) {
				t.Errorf("%s: %q %s %q is not the bare tool name alone in its command", rel, call.Lead, call.Word, call.Args)
			}
			args := fillPlaceholders(t, call.Args)
			subcommand, flags, err := parse(args)
			if err != nil {
				t.Errorf("%s: %s %q is a usage error: %v", rel, toolWord, call.Args, err)
				continue
			}
			if _, err := parseRequests(subcommand, flags); err != nil {
				t.Errorf("%s: %s %q is a malformed request: %v", rel, toolWord, call.Args, err)
			}
			if rel == definitionPath {
				called[subcommand] = struct{}{}
			}
		}
	}
	for subcommand := range subcommandFlags {
		if _, ok := called[subcommand]; !ok {
			t.Errorf("%s writes no call of %s %s", definitionPath, toolWord, subcommand)
		}
	}
}

// TestNoDirectHerdrInvocation: no command document writes herdr as a command
// to run, so the /chief session reaches the panes only through the tool,
// which builds every input from its types. The settings deny the calls that
// send input (TestSettingsDeniesPaneInput); this keeps the definition from
// asking for one.
func TestNoDirectHerdrInvocation(t *testing.T) {
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		for _, text := range doctestutil.CommandTexts(readRepoFile(t, rel)) {
			if doctestutil.RunsCommand(text, "herdr") {
				t.Errorf("%s runs herdr directly: %q", rel, text)
			}
		}
	}
}

// settings is the part of .claude/settings.json the guards read.
type settings struct {
	Permissions struct {
		Allow []string `json:"allow"`
		Ask   []string `json:"ask"`
		Deny  []string `json:"deny"`
	} `json:"permissions"`
}

// matchesRule reports whether the Bash permission rule rule matches the
// command text. It models only the forms the settings use: Bash(<prefix>*),
// where a prefix ending in a space also matches the command without it
// (Bash(x *) matches x alone, as Claude Code does), and Bash(<command>) for
// one exact command; it fails t for a rule of any other form, so a rule it
// cannot read never passes as matching nothing. Claude Code also tries
// normalized forms of a command against deny and ask rules, which this
// model leaves out.
func matchesRule(t *testing.T, rule, text string) bool {
	t.Helper()
	inner, ok := strings.CutPrefix(rule, "Bash(")
	inner, ok2 := strings.CutSuffix(inner, ")")
	if !ok || !ok2 || inner == "" || strings.ContainsAny(inner, "():") {
		t.Fatalf("permission rule %q is neither Bash(<prefix>*) nor Bash(<command>)", rule)
	}
	body, prefixRule := strings.CutSuffix(inner, "*")
	if strings.Contains(body, "*") {
		t.Fatalf("permission rule %q has a wildcard before its end, which matchesRule does not model", rule)
	}
	if prefixRule {
		return strings.HasPrefix(text, body) || (strings.HasSuffix(body, " ") && text == strings.TrimSuffix(body, " "))
	}
	return text == inner
}

func readSettings(t *testing.T) settings {
	t.Helper()
	var s settings
	if err := json.Unmarshal([]byte(readRepoFile(t, ".claude/settings.json")), &s); err != nil {
		t.Fatalf("parse .claude/settings.json: %v", err)
	}
	return s
}

// deniedHerdr are the herdr calls the settings must deny (ADR-0046): the
// calls that type into a pane, run a command in it, or route its input; the
// calls that report an agent's state or session to Herdr, which chief's
// checks read; the calls that close or remove a pane, tab, workspace,
// session, or worktree, change Herdr's agent integration, or stop or
// replace Herdr, which would end the work of a task without typing into it;
// the global flags before a subcommand that point at another session or
// machine, which would get past the prefixes above; and the calls that
// create a workspace, a tab, a pane, or a worktree, or move a pane, which
// only the tool does, from a validated issue number. A rule of the closing
// group ends in "*" with no space, so it also matches the call with no
// argument, which acts on the current pane, tab, or workspace (a rule
// ending in " *" matches the call with no argument too). A rule ending in
// "*" right after the flag also matches the flag's =value form and its
// longer spellings such as --remote-keybindings.
var deniedHerdr = []string{
	"Bash(herdr agent prompt *)",
	"Bash(herdr agent send-keys *)",
	"Bash(herdr agent start *)",
	"Bash(herdr agent attach *)",
	"Bash(herdr pane send-text *)",
	"Bash(herdr pane send-keys *)",
	"Bash(herdr pane run *)",
	"Bash(herdr pane input *)",
	"Bash(herdr pane report-agent *)",
	"Bash(herdr pane report-agent-session *)",
	"Bash(herdr pane release-agent *)",
	"Bash(herdr pane close*)",
	"Bash(herdr tab close*)",
	"Bash(herdr workspace close*)",
	"Bash(herdr session stop*)",
	"Bash(herdr session delete*)",
	"Bash(herdr worktree remove*)",
	"Bash(herdr integration install*)",
	"Bash(herdr integration uninstall*)",
	"Bash(herdr server stop*)",
	"Bash(herdr update*)",
	"Bash(herdr --session*)",
	"Bash(herdr --machine*)",
	"Bash(herdr --remote*)",
	"Bash(herdr workspace create*)",
	"Bash(herdr tab create*)",
	"Bash(herdr pane split*)",
	"Bash(herdr pane move*)",
	"Bash(herdr worktree create*)",
	"Bash(herdr worktree open*)",
}

// deniedGit are the git calls the settings must deny (ADR-0046): adding,
// removing, or moving a worktree, which only the tool does, removing only a
// worktree that passed every removal check.
var deniedGit = []string{
	"Bash(git worktree add*)",
	"Bash(git worktree remove*)",
	"Bash(git worktree move*)",
}

// TestSettingsDeniesPaneInput: .claude/settings.json denies every herdr
// call of deniedHerdr and allows no herdr rule, so a session that follows an
// instruction written on a screen or in a PR cannot type into another pane,
// or forge the state chief checks, without the tool.
func TestSettingsDeniesPaneInput(t *testing.T) {
	s := readSettings(t)
	for _, rule := range deniedHerdr {
		if !slices.Contains(s.Permissions.Deny, rule) {
			t.Errorf("permissions.deny %q does not contain %q", s.Permissions.Deny, rule)
		}
	}
	for _, rule := range s.Permissions.Allow {
		if strings.HasPrefix(rule, "Bash(herdr") {
			t.Errorf("permissions.allow has %q, which may reach a pane without the tool", rule)
		}
	}
}

// worktreeChanges are commands that add, remove, or move a worktree,
// including the forms with git's options before the subcommand, which the
// deny rules' prefixes do not reach.
var worktreeChanges = []string{
	"git worktree add /p HEAD",
	"git worktree remove /p",
	"git -C /r worktree add /p HEAD",
	"git -c core.x=y worktree remove /p",
	"git --git-dir=/r/.git worktree move /p /q",
}

// TestSettingsDeniesWorktreeChanges: .claude/settings.json denies every git
// call of deniedGit, and no allow rule lets a command of worktreeChanges run
// unchecked (an allowed command skips both the prompt and auto mode's
// classifier), so a session cannot make or remove a task's worktree without
// the tool and its checks, past what the deny rules' prefixes reach.
func TestSettingsDeniesWorktreeChanges(t *testing.T) {
	s := readSettings(t)
	for _, rule := range deniedGit {
		if !slices.Contains(s.Permissions.Deny, rule) {
			t.Errorf("permissions.deny %q does not contain %q", s.Permissions.Deny, rule)
		}
	}
	for _, rule := range s.Permissions.Allow {
		for _, text := range worktreeChanges {
			if matchesRule(t, rule, text) {
				t.Errorf("permissions.allow has %q, which lets %q run without the tool", rule, text)
			}
		}
	}
}

// TestSettingsAsksBeforeRemove: the only ask rule of .claude/settings.json
// is askRemove, so Claude Code asks the human to confirm every removal
// outside the session too (an ask rule wins over the allow rule
// Bash(chief *)), and no other call of the tool, however its arguments are
// written, stops watching at a prompt.
func TestSettingsAsksBeforeRemove(t *testing.T) {
	if ask := readSettings(t).Permissions.Ask; !slices.Equal(ask, []string{askRemove}) {
		t.Errorf("permissions.ask %q, want only %q", ask, askRemove)
	}
}

// TestSettingsAllowsTool: .claude/settings.json allows the tool under the
// name go install gives it, so the watching session calls it without a
// prompt.
func TestSettingsAllowsTool(t *testing.T) {
	want := "Bash(" + toolWord + " *)"
	if allow := readSettings(t).Permissions.Allow; !slices.Contains(allow, want) {
		t.Errorf("permissions.allow %q does not contain %q", allow, want)
	}
}

// TestInstallTargetInMakefile: the Makefile's install-chief target installs
// this package, so make install-chief installs the tool the definition runs.
func TestInstallTargetInMakefile(t *testing.T) {
	target := "install-" + toolWord
	lines := strings.Split(readRepoFile(t, "Makefile"), "\n")
	i := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, target+":") })
	if i < 0 || i+1 >= len(lines) || lines[i+1] != "\t$(GOCMD) install ./cmd/"+toolWord {
		t.Errorf("Makefile has no %s target whose recipe is $(GOCMD) install ./cmd/%s", target, toolWord)
	}
}

// guidePath is the human guide, whose values the tool defines.
const guidePath = "docs/dev/guides/chief_guide.md"

// TestGuideValuesMatchTool: the guide's table of subcommands names exactly
// the tool's subcommands, its table of exit codes exactly the four codes the
// tool defines (a code added to main.go must be added here),
// its protocol number is the tool's Protocol, and the state directory it
// names is the one the tool opens below XDG_STATE_HOME, so the guide is not
// left describing another version of the tool.
func TestGuideValuesMatchTool(t *testing.T) {
	guide := readRepoFile(t, guidePath)
	var subcommands []string
	for subcommand := range subcommandFlags {
		subcommands = append(subcommands, subcommand)
	}
	codes := []string{strconv.Itoa(exitOK), strconv.Itoa(exitFailure), strconv.Itoa(exitUsage), strconv.Itoa(exitRefused)}
	for header, want := range map[string][]string{"サブコマンド": subcommands, "終了コード": codes} {
		table := doctestutil.FindTable(guide, header)
		if table == nil {
			t.Errorf("%s has no table with a %s column", guidePath, header)
			continue
		}
		var got []string
		for _, row := range table[1:] {
			got = append(got, doctestutil.StripBackticks(row[0]))
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: the %s table lists %q, the tool has %q", guidePath, header, got, want)
		}
	}
	if got := guideValue(guide, "プロトコル番号"); got != strconv.Itoa(chief.Protocol) {
		t.Errorf("%s: protocol number %q, want %d", guidePath, got, chief.Protocol)
	}
	base := t.TempDir()
	store, err := chief.OpenStore(base)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(base, store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if want := "`$XDG_STATE_HOME/" + filepath.ToSlash(rel) + "/`"; !strings.Contains(guide, want) {
		t.Errorf("%s does not name the state directory %s", guidePath, want)
	}
}

// guideValue returns the value cell of the guide's value table (the table
// with a 値 column) in the row whose first cell is item, or "".
func guideValue(guide, item string) string {
	table := doctestutil.FindTable(guide, "値")
	if table == nil {
		return ""
	}
	col := doctestutil.ColumnIndex(table[0], "値")
	row := doctestutil.TableRow(table, item)
	if row == nil || col >= len(row) {
		return ""
	}
	return doctestutil.StripBackticks(row[col])
}
