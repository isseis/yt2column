//go:build test

package chief

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// readScreenSample returns a screen sample under testdata/screens.
func readScreenSample(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "screens", name))
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	return string(data)
}

var sgrSequence = regexp.MustCompile("\x1b\\[[0-9;]*m")

// plainSample returns a sample with its SGR sequences removed, for building
// broken variants of a shape by editing its text.
func plainSample(t *testing.T, name string) string {
	t.Helper()
	return sgrSequence.ReplaceAllString(readScreenSample(t, name), "")
}

// wantScreen is the reading a sample must give. Labels lists every option
// label in order and Relayable the labels that may be relayed.
type wantScreen struct {
	kind       ScreenKind
	question   QuestionKind
	relay      RelayBlock
	merge      bool
	text       string
	labels     []string
	relayable  []string
	tabs       []string
	inputEmpty bool
	inputLeft  string
	model      string
	newSession bool
	// cursor is the option the selection is on, read for single-select
	// questions only.
	cursor int
}

const sampleModel = "DeepSeek V4.1 Flash 1017 OpenCode Go"

var (
	fruitLabels       = []string{"Apple", "Banana", "Cherry"}
	claudeFruitSingle = wantScreen{
		kind: ScreenQuestion, question: QuestionSingle, relay: RelayAllowed,
		text:      "Which fruit do you prefer?",
		labels:    append(append([]string{}, fruitLabels...), "Type something.", "Chat about this"),
		relayable: fruitLabels,
		cursor:    1,
	}
	openCodeFruitSingle = wantScreen{
		kind: ScreenQuestion, question: QuestionSingle, relay: RelayAllowed,
		text:      "Which fruit do you prefer?",
		labels:    append(append([]string{}, fruitLabels...), "Type your own answer"),
		relayable: fruitLabels,
		cursor:    1,
	}
	mergeText   = "PR #123 をどうしますか（第 1 回。要約と詳細ファイルを確認のうえ）"
	mergeLabels = []string{"マージ", "マージしない"}
)

// claudeScreenWants and openCodeScreenWants give the reading each screen
// sample must give; every file under testdata/screens has an entry.
var claudeScreenWants = map[string]wantScreen{
	"claude-idle.ansi":                   {kind: ScreenPrompt, inputEmpty: true},
	"claude-working.ansi":                {kind: ScreenPrompt, inputEmpty: true},
	"claude-after-clear.ansi":            {kind: ScreenPrompt, inputEmpty: true},
	"claude-after-model.ansi":            {kind: ScreenPrompt, inputEmpty: true},
	"claude-command-report.ansi":         {kind: ScreenPrompt, inputEmpty: true},
	"claude-input-left.ansi":             {kind: ScreenPrompt, inputLeft: "unsent draft text"},
	"claude-question-single.ansi":        claudeFruitSingle,
	"claude-question-single-cursor.ansi": withCursor(claudeFruitSingle, 3),
	"claude-question-multi.ansi": {
		kind: ScreenQuestion, question: QuestionMulti, relay: RelayBlockMulti,
		text: "Which colors do you like?", tabs: []string{"Colors"},
		labels: []string{"Red", "Green", "Blue", "[ ] Type something", "Chat about this"},
	},
	"claude-question-tabs.ansi": {
		kind: ScreenQuestion, question: QuestionTabs, relay: RelayBlockTabs,
		text: "Which fruit?", tabs: []string{"Fruit", "Drink"},
		labels: []string{"Apple", "Banana", "Type something.", "Chat about this"},
	},
	"claude-question-tabs-second.ansi": {
		kind: ScreenQuestion, question: QuestionTabs, relay: RelayBlockTabs,
		text: "Which drink?", tabs: []string{"Fruit", "Drink"},
		labels: []string{"Tea", "Juice", "Water", "Type something.", "Chat about this"},
	},
	"claude-question-tabs-review.ansi": {
		kind: ScreenQuestion, question: QuestionTabs, relay: RelayBlockTabs,
		text:   "Review your answers\n ● Which fruit?\n   → Banana\n ● Which drink?\n   → Water\nReady to submit your answers?",
		tabs:   []string{"Fruit", "Drink"},
		labels: []string{"Submit answers", "Cancel"},
	},
	"claude-permission.ansi": {
		kind: ScreenQuestion, question: QuestionPermission, relay: RelayBlockPermission,
		text: "Bash command\nTip: auto mode handles these prompts for you — choose \"switch to auto mode\" below\n" +
			"Create an empty marker file in /tmp\n" + strings.Repeat("╌", 83) + "\ntouch /tmp/chief-sample-permission-check\n" +
			strings.Repeat("╌", 83) + "\nDo you want to proceed?",
		labels: []string{"Yes", "Yes, and always allow access to /tmp from this project", "Yes, and switch to auto mode · auto mode handles these prompts for you", "No"},
	},
	// A permission prompt whose command wraps onto lines marked "│" and
	// that ends with a warning and a blank line before the proceed line.
	"claude-permission-warning.ansi": {
		kind: ScreenQuestion, question: QuestionPermission, relay: RelayBlockPermission,
		text: "Bash command\nTip: auto mode handles these prompts for you — choose \"switch to auto mode\" below\n" +
			"Read sshd configuration and service status\n" + strings.Repeat("╌", 83) + "\n" +
			"│ echo \"=== sshd_config (non-comment) ===\"; grep -vE '^\\s*(#|$)'\n" +
			"│ /etc/ssh/sshd_config; echo; echo \"=== sshd_config.d ===\"; ls -la\n" +
			"│ /etc/ssh/sshd_config.d/ 2>&1; for f in /etc/ssh/sshd_config.d/*; do [ -f \"$f\" ]\n" +
			"│ && { echo \"--- $f\"; grep -vE '^\\s*(#|$)' \"$f\"; }; done; echo; echo \"=== Remote\n" +
			"│ Login status ===\"; launchctl print-disabled system 2>/dev/null | grep -i ssh;\n" +
			"│ launchctl list 2>/dev/null | grep -i ssh; echo; echo \"=== listening on 22 ===\";\n" +
			"│ netstat -an -p tcp 2>/dev/null | grep -E '\\.22 .*LISTEN'; echo; echo \"=== ssh\n" +
			"│ version ===\"; ssh -V 2>&1\n" + strings.Repeat("╌", 83) + "\n" +
			"Contains brace with quote character (expansion obfuscation)\n\nDo you want to proceed?",
		labels: []string{"Yes", "Yes, and switch to auto mode · auto mode handles these prompts for you", "No"},
	},
	"claude-merge-question.ansi": {
		kind: ScreenQuestion, question: QuestionSingle, relay: RelayAllowed, merge: true,
		text:      mergeText,
		labels:    append(append([]string{}, mergeLabels...), "Type something.", "Chat about this"),
		relayable: mergeLabels,
		cursor:    1,
	},
}

var openCodeScreenWants = map[string]wantScreen{
	"opencode-done.ansi":      {kind: ScreenPrompt, inputEmpty: true, model: sampleModel},
	"opencode-after-new.ansi": {kind: ScreenPrompt, inputEmpty: true, model: sampleModel, newSession: true},
	// A model with no reasoning effort: one separator on the status line.
	"opencode-model-no-effort.ansi":        {kind: ScreenPrompt, inputEmpty: true, model: "DeepSeek V4 Pro OpenCode Go", newSession: true},
	"opencode-input-left.ansi":             {kind: ScreenPrompt, inputLeft: "unsent draft text", model: sampleModel},
	"opencode-runpr-report.txt":            {kind: ScreenPrompt, inputEmpty: true, model: sampleModel},
	"opencode-question-single.ansi":        openCodeFruitSingle,
	"opencode-question-single-cursor.ansi": withCursor(openCodeFruitSingle, 3),
	"opencode-question-multi.ansi": {
		kind: ScreenQuestion, question: QuestionMulti, relay: RelayBlockMulti,
		text: "Which colors do you like?", tabs: []string{"Color preferences"},
		labels: []string{"Red", "Green", "Blue", "Type your own answer"},
	},
	"opencode-question-tabs.ansi": {
		kind: ScreenQuestion, question: QuestionTabs, relay: RelayBlockTabs,
		text: "Which fruit?", tabs: []string{"Fruit", "Drink"},
		labels: []string{"Apple", "Banana", "Type your own answer"},
	},
	"opencode-question-tabs-second.ansi": {
		kind: ScreenQuestion, question: QuestionTabs, relay: RelayBlockTabs,
		text: "Which drink?", tabs: []string{"Fruit", "Drink"},
		labels: []string{"Tea", "Juice", "Water", "Type your own answer"},
	},
	"opencode-question-tabs-review.ansi": {
		kind: ScreenQuestion, question: QuestionTabs, relay: RelayBlockTabs,
		text: "Fruit: Banana\nDrink: Water", tabs: []string{"Fruit", "Drink"},
	},
	"opencode-merge-question.ansi": {
		kind: ScreenQuestion, question: QuestionSingle, relay: RelayAllowed, merge: true,
		text:      mergeText,
		labels:    append(append([]string{}, mergeLabels...), "Type your own answer"),
		relayable: mergeLabels,
		cursor:    1,
	},
}

// withCursor returns want with the selection on option n.
func withCursor(want wantScreen, n int) wantScreen {
	want.cursor = n
	return want
}

func TestParseScreenSamples(t *testing.T) {
	readers := []struct {
		name         string
		parse, other func(string) Screen
		samples      map[string]wantScreen
	}{
		{"claude", parseClaudeScreen, parseOpenCodeScreen, claudeScreenWants},
		{"opencode", parseOpenCodeScreen, parseClaudeScreen, openCodeScreenWants},
	}
	for _, r := range readers {
		for name, want := range r.samples {
			t.Run(r.name+"/"+name, func(t *testing.T) {
				raw := readScreenSample(t, name)
				checkScreen(t, r.parse(raw), want)
				// The other agent's reader must not read this agent's screen.
				if got := r.other(raw); got.Kind != ScreenUnmatched {
					t.Errorf("the other reader read %s as kind %v", name, got.Kind)
				}
			})
		}
	}
}

func checkScreen(t *testing.T, got Screen, want wantScreen) {
	t.Helper()
	if got.Kind != want.kind {
		t.Fatalf("Kind = %v, want %v", got.Kind, want.kind)
	}
	if want.kind == ScreenPrompt {
		if got.Question != nil {
			t.Errorf("a prompt screen carries a question")
		}
		if got.Input != (InputLine{Empty: want.inputEmpty, Left: want.inputLeft}) {
			t.Errorf("Input = %+v, want empty %v left %q", got.Input, want.inputEmpty, want.inputLeft)
		}
		if got.OpenCodeModel != want.model || got.NewSession != want.newSession {
			t.Errorf("model %q new session %v, want %q %v", got.OpenCodeModel, got.NewSession, want.model, want.newSession)
		}
		return
	}
	q := got.Question
	if q == nil {
		t.Fatal("question screen without a question")
	}
	if q.Kind != want.question || q.Relay != want.relay || q.Merge != want.merge {
		t.Errorf("kind %v relay %v merge %v, want %v %v %v", q.Kind, q.Relay, q.Merge, want.question, want.relay, want.merge)
	}
	if q.Text != want.text {
		t.Errorf("Text = %q, want %q", q.Text, want.text)
	}
	if !reflect.DeepEqual(q.Tabs, want.tabs) {
		t.Errorf("Tabs = %q, want %q", q.Tabs, want.tabs)
	}
	if q.Cursor != want.cursor {
		t.Errorf("Cursor = %d, want %d", q.Cursor, want.cursor)
	}
	var labels, relayable []string
	for i, o := range q.Options {
		if o.Number != i+1 {
			t.Errorf("option %d has number %d", i+1, o.Number)
		}
		labels = append(labels, o.Label)
		if o.Relayable {
			relayable = append(relayable, o.Label)
		}
	}
	if !reflect.DeepEqual(labels, want.labels) {
		t.Errorf("labels = %q, want %q", labels, want.labels)
	}
	if !reflect.DeepEqual(relayable, want.relayable) {
		t.Errorf("relayable = %q, want %q", relayable, want.relayable)
	}
}

// TestOpenCodeCursorNeedsOneMark recolors the option numbers of an OpenCode
// question so that two options, or none, are drawn in the selection's color:
// the question still reads, with no cursor.
func TestOpenCodeCursorNeedsOneMark(t *testing.T) {
	const (
		gray   = "\x1b[38;2;138;138;138m\x1b[48;2;250;250;250m"
		accent = "\x1b[38;2;59;125;216m\x1b[48;2;250;250;250m"
	)
	base := readScreenSample(t, "opencode-question-single.ansi")
	for name, edit := range map[string][2]string{
		"two marked":  {gray + "2.", accent + "2."},
		"none marked": {accent + "1.", gray + "1."},
	} {
		t.Run(name, func(t *testing.T) {
			if strings.Count(base, edit[0]) != 1 {
				t.Fatalf("the sample does not hold %q once", edit[0])
			}
			got := parseOpenCodeScreen(strings.Replace(base, edit[0], edit[1], 1))
			if got.Question == nil || got.Question.Kind != QuestionSingle || got.Question.Cursor != 0 {
				t.Errorf("question = %+v, want a single-select question with no cursor", got.Question)
			}
		})
	}
}

// TestOpenCodeCursorSkipsQuestionText makes an OpenCode question's text
// read like its first option, drawn in a color other than the gray: the
// cursor is still read from the options alone.
func TestOpenCodeCursorSkipsQuestionText(t *testing.T) {
	const old = "\x1b[38;2;26;26;26m\x1b[48;2;250;250;250mWhich fruit do you prefer?"
	base := readScreenSample(t, "opencode-question-single-cursor.ansi")
	if strings.Count(base, old) != 1 {
		t.Fatalf("the sample does not hold %q once", old)
	}
	edited := strings.Replace(base, old, "\x1b[38;2;26;26;26m\x1b[48;2;250;250;250m1. Apple"+strings.Repeat(" ", 18), 1)
	got := parseOpenCodeScreen(edited)
	if got.Question == nil || got.Question.Text != "1. Apple" || got.Question.Cursor != 3 {
		t.Errorf("question = %+v, want the text 1. Apple and the cursor on 3", got.Question)
	}
}

// TestClaudeCursorOnChat moves Claude Code's cursor mark from option 1 to the
// chat option below the widget's rule: the cursor reads as that option.
func TestClaudeCursorOnChat(t *testing.T) {
	base := plainSample(t, "claude-question-single.ansi")
	if q := parseClaudeScreen(base).Question; q == nil || q.Cursor != 1 {
		t.Fatalf("the plain sample reads as %+v, want the cursor on 1", q)
	}
	edits := [][2]string{{"❯ 1. Apple", "  1. Apple"}, {"  5. Chat about this", "❯ 5. Chat about this"}}
	for _, e := range edits {
		if strings.Count(base, e[0]) != 1 {
			t.Fatalf("the sample does not hold %q once", e[0])
		}
		base = strings.Replace(base, e[0], e[1], 1)
	}
	got := parseClaudeScreen(base)
	if got.Question == nil || got.Question.Kind != QuestionSingle || got.Question.Cursor != 5 {
		t.Errorf("question = %+v, want a single-select question with the cursor on 5", got.Question)
	}
}

func TestParseScreenOptionDescriptions(t *testing.T) {
	for _, c := range []struct {
		name  string
		parse func(string) Screen
	}{
		{"claude-question-single.ansi", parseClaudeScreen},
		{"opencode-question-single.ansi", parseOpenCodeScreen},
	} {
		got := c.parse(readScreenSample(t, c.name))
		if got.Question == nil {
			t.Fatalf("%s: no question", c.name)
		}
		var desc []string
		for _, o := range got.Question.Options {
			desc = append(desc, o.Description)
		}
		if want := []string{"A red fruit", "A yellow fruit", "A small red fruit"}; !reflect.DeepEqual(desc[:3], want) {
			t.Errorf("%s: descriptions = %q, want %q", c.name, desc, want)
		}
	}
}

// TestParseScreenRejectsNonMatching breaks one part of a sample at a time.
// Each base case is first checked to match, so a broken variant that does
// not match fails for the part the case changed.
func TestParseScreenRejectsNonMatching(t *testing.T) {
	type mutation struct {
		name     string
		parse    func(string) Screen
		base     string
		old, new string
	}
	const cq, oq = "claude-question-single.ansi", "opencode-question-single.ansi"
	cases := []mutation{
		{"claude option number gap", parseClaudeScreen, cq, "  2. Banana", "  3. Banana"},
		{"claude two cursors", parseClaudeScreen, cq, "  2. Banana", "❯ 2. Banana"},
		{"claude no free-text option", parseClaudeScreen, cq, "  4. Type something.", "  4. Durian"},
		{"claude unknown line in options", parseClaudeScreen, cq, "     A yellow fruit", "    A yellow fruit"},
		{"claude no question text", parseClaudeScreen, cq, "Which fruit do you prefer?\r\n", ""},
		{"claude no header", parseClaudeScreen, cq, " ☐ Fruit", "Fruit"},
		{"claude unknown footer", parseClaudeScreen, cq, "Esc to cancel", "Esc to quit"},
		{"claude chat number", parseClaudeScreen, cq, "  5. Chat about this", "  6. Chat about this"},
		{"claude chat label", parseClaudeScreen, cq, "  5. Chat about this", "  5. Chat about that"},
		{"claude multi-select with two tabs", parseClaudeScreen, "claude-question-multi.ansi", "☐ Colors  ✔ Submit", "☐ Colors  ☐ More  ✔ Submit"},
		{"claude permission without proceed line", parseClaudeScreen, "claude-permission.ansi", " Do you want to proceed?", " Proceed?"},
		{"claude permission with an unindented warning", parseClaudeScreen, "claude-permission-warning.ansi", " Contains brace", "Contains brace"},
		{"claude tab bar without submit", parseClaudeScreen, "claude-question-tabs.ansi", "✔ Submit", "✔ Send"},
		{"claude prompt with a line after the status line", parseClaudeScreen, "claude-idle.ansi", "auto mode on", "auto mode on\nunexpected"},
		{"opencode option number gap", parseOpenCodeScreen, oq, "2. Banana", "3. Banana"},
		{"opencode no free-text option", parseOpenCodeScreen, oq, "4. Type your own answer", "4. Durian"},
		{"opencode no questions title", parseOpenCodeScreen, oq, "┃  Questions", "┃  Question"},
		{"opencode unknown footer", parseOpenCodeScreen, oq, "esc dismiss", "esc close"},
		{"opencode description indent", parseOpenCodeScreen, oq, "     A yellow fruit", "      A yellow fruit"},
		{"opencode tab bar on single footer", parseOpenCodeScreen, oq, "┃  Which fruit do you prefer?", "┃  Fruit  Submit\n  ┃\n  ┃  Which fruit do you prefer?"},
		{"opencode prompt without bottom edge", parseOpenCodeScreen, "opencode-done.ansi", "╹▀▀▀▀", "x▀▀▀▀"},
		{"opencode status line without model", parseOpenCodeScreen, "opencode-done.ansi", "Build · DeepSeek V4.1 Flash 1017 OpenCode Go · high", "Build"},
		{"opencode status line with an empty model", parseOpenCodeScreen, "opencode-model-no-effort.ansi", "Build · DeepSeek V4 Pro OpenCode Go", "Build · · high"},
		{"opencode status line without agent", parseOpenCodeScreen, "opencode-model-no-effort.ansi", "Build · DeepSeek V4 Pro OpenCode Go", " · DeepSeek V4 Pro OpenCode Go"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := plainSample(t, c.base)
			if !strings.Contains(base, c.old) {
				t.Fatalf("the sample does not hold %q", c.old)
			}
			if got := c.parse(base); got.Kind == ScreenUnmatched {
				t.Fatalf("the unbroken sample does not match")
			}
			broken := strings.Replace(base, c.old, c.new, 1)
			if got := c.parse(broken); got.Kind != ScreenUnmatched {
				t.Errorf("broken screen read as kind %v", got.Kind)
			}
		})
	}

	raw := map[string]string{
		"foreign escape sequence": strings.Replace(readScreenSample(t, cq), "Which fruit", "\x1b[2JWhich fruit", 1),
		"over the line cap":       strings.Repeat("x\r\n", screenMaxLines) + readScreenSample(t, cq),
		"over the byte cap":       strings.Repeat(strings.Repeat("x", 1024)+"\r\n", screenMaxBytes/1024) + readScreenSample(t, cq),
	}
	for name, screen := range raw {
		t.Run(name, func(t *testing.T) {
			if got := parseClaudeScreen(screen); got.Kind != ScreenUnmatched {
				t.Errorf("read as kind %v", got.Kind)
			}
		})
	}
}

// TestInputPlaceholderNeedsItsStyle checks that placeholder text counts as
// empty only in the style the agent draws it: the same text in another style
// is unsent input.
func TestInputPlaceholderNeedsItsStyle(t *testing.T) {
	claude := readScreenSample(t, "claude-idle.ansi")
	if got := parseClaudeScreen(claude); !got.Input.Empty {
		t.Fatalf("the sample's placeholder is not read as empty: %+v", got.Input)
	}
	if got := parseClaudeScreen(strings.Replace(claude, "\x1b[2m", "", 1)); got.Input.Empty || got.Input.Left == "" {
		t.Errorf("an undimmed placeholder was read as empty: %+v", got.Input)
	}
	openCode := readScreenSample(t, "opencode-after-new.ansi")
	if got := parseOpenCodeScreen(openCode); !got.Input.Empty {
		t.Fatalf("the sample's placeholder is not read as empty: %+v", got.Input)
	}
	recolored := strings.Replace(openCode, "\x1b[38;2;138;138;138m\x1b[48;2;245;245;245mAsk", "\x1b[38;2;26;26;26m\x1b[48;2;245;245;245mAsk", 1)
	if recolored == openCode {
		t.Fatal("the sample does not hold the placeholder sequence")
	}
	if got := parseOpenCodeScreen(recolored); got.Input.Empty || got.Input.Left == "" {
		t.Errorf("a placeholder in the typed-text color was read as empty: %+v", got.Input)
	}
}

func TestOpenCodeNewSessionNeedsOnlyTheLogo(t *testing.T) {
	screen := plainSample(t, "opencode-after-new.ansi")
	if !parseOpenCodeScreen(screen).NewSession {
		t.Fatal("the sample is not read as a new session")
	}
	withLine := strings.Replace(screen, "/update to install", "Earlier reply\n/update to install", 1)
	if got := parseOpenCodeScreen(withLine); got.Kind != ScreenPrompt || got.NewSession {
		t.Errorf("a screen with a conversation line read as kind %v new session %v", got.Kind, got.NewSession)
	}
	// Drop the last logo line and the update notice, so only blank lines
	// and part of the logo are left above the box.
	partLogo := strings.Replace(screen, "▀▀▀▀ █▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀", "", 1)
	partLogo = regexp.MustCompile(`/update to install v[0-9.]+`).ReplaceAllString(partLogo, "")
	if !strings.Contains(screen, "/update to install v") || strings.Count(partLogo, "▀▀▀▀ █▀▀▀") != 0 {
		t.Fatal("the sample does not hold the last logo line and the update notice")
	}
	if got := parseOpenCodeScreen(partLogo); got.Kind != ScreenPrompt || got.NewSession {
		t.Errorf("a screen with part of the logo read as kind %v new session %v", got.Kind, got.NewSession)
	}
}

func TestReadRunprNext(t *testing.T) {
	report := plainSample(t, "opencode-runpr-report.txt")
	if got := readRunprNext(report); got != (RunprNext{Kind: NextNone}) {
		t.Errorf("report sample: %+v, want none", got)
	}
	merge := plainSample(t, "claude-merge-question.ansi")
	if got := readRunprNext(merge); got != (RunprNext{Kind: NextCommand, Command: NextCommandMkarch, Task: taskID(t, "0123")}) {
		t.Errorf("merge question sample: %+v", got)
	}
	cases := map[string]struct {
		text string
		want RunprNext
	}{
		"no NEXT line":              {"done\n", RunprNext{Kind: NextAbsent}},
		"runplan in quotes":         {"- NEXT: `/runplan 0088`（要件 opus → Opus）", RunprNext{Kind: NextCommand, Command: NextCommandRunplan, Task: taskID(t, "0088")}},
		"mkplan":                    {"NEXT: /mkplan 0007", RunprNext{Kind: NextCommand, Command: NextCommandMkplan, Task: taskID(t, "0007")}},
		"agreeing lines":            {"NEXT: /mkplan 0007\n  - NEXT: `/mkplan 0007`（次）", RunprNext{Kind: NextCommand, Command: NextCommandMkplan, Task: taskID(t, "0007")}},
		"none with more text":       {"NEXT: なしではなく /runplan 0088", RunprNext{}},
		"task with a suffix":        {"NEXT: /runplan 0088abc", RunprNext{}},
		"unpaired backquote":        {"NEXT: `/runplan 0088", RunprNext{}},
		"disagreeing lines":         {"NEXT: /mkplan 0007\nNEXT: なし", RunprNext{}},
		"unknown command":           {"NEXT: /runpr 0012", RunprNext{}},
		"none alone":                {"NEXT: なし", RunprNext{Kind: NextNone}},
		"three-digit task":          {"NEXT: /runplan 088", RunprNext{}},
		"free text value":           {"NEXT: see above", RunprNext{}},
		"NEXT inside a line":        {"the NEXT: /mkplan 0007 line", RunprNext{Kind: NextAbsent}},
		"empty NEXT alone":          {"NEXT:", RunprNext{}},
		"empty NEXT after a value":  {"NEXT: /runplan 0088\nNEXT:", RunprNext{}},
		"empty NEXT before a value": {"NEXT:\nNEXT: /runplan 0088", RunprNext{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := readRunprNext(c.text); got != c.want {
				t.Errorf("readRunprNext(%q) = %+v, want %+v", c.text, got, c.want)
			}
		})
	}
}

// TestInputEmptyNeedsThePlaceholderForm checks that styled text counts as an
// empty input only in the placeholder's whole form, on one line.
func TestInputEmptyNeedsThePlaceholderForm(t *testing.T) {
	claude := readScreenSample(t, "claude-idle.ansi")
	const placeholder = `Try "refactor launch.ts"`
	if !strings.Contains(claude, placeholder) {
		t.Fatal("the sample does not hold the placeholder")
	}
	openCode := readScreenSample(t, "opencode-after-new.ansi")
	const ocPlaceholder = `Ask anything… "What is the tech stack of this project?"`
	if !strings.Contains(openCode, ocPlaceholder) {
		t.Fatal("the sample does not hold the placeholder")
	}
	cases := map[string]struct {
		parse  func(string) Screen
		screen string
	}{
		"claude dim text that is not the placeholder": {parseClaudeScreen, strings.Replace(claude, placeholder, "unsent", 1)},
		"claude dim placeholder with more text":       {parseClaudeScreen, strings.Replace(claude, placeholder, placeholder+" and more", 1)},
		"opencode gray text after the prefix":         {parseOpenCodeScreen, strings.Replace(openCode, ocPlaceholder, "Ask anything… rm -rf and push", 1)},
		// The dim state carries to the next line, so the typed line is dim too.
		"claude placeholder with a second line":        {parseClaudeScreen, strings.Replace(claude, placeholder, placeholder+"\r\n  typed text", 1)},
		"claude two-line text in the placeholder form": {parseClaudeScreen, strings.Replace(claude, placeholder, `Try "refactor`+"\r\n  "+`more"`, 1)},
		"opencode placeholder with a second line":      {parseOpenCodeScreen, strings.Replace(openCode, ocPlaceholder, ocPlaceholder+"\r\n    ┃  typed text", 1)},
		"claude full-width space":                      {parseClaudeScreen, strings.Replace(claude, "\x1b[2m"+placeholder, "\u3000", 1)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := c.parse(c.screen)
			if got.Kind != ScreenPrompt || got.Input.Empty || got.Input.Left == "" {
				t.Errorf("read as kind %v input %+v, want unsent text", got.Kind, got.Input)
			}
		})
	}
}

// TestQuestionShapesNotSampled checks two shapes the samples did not show: an
// option numbered 10 or more, and a merge question wrapped across lines.
func TestQuestionShapesNotSampled(t *testing.T) {
	// Drop the padding blanks so whole lines can be replaced.
	single := regexp.MustCompile(` +\r\n`).ReplaceAllString(plainSample(t, "opencode-question-single.ansi"), "\r\n")
	const freeTextLine = "  ┃  4. Type your own answer\r\n"
	head, rest, ok1 := strings.Cut(single, "  ┃  1. Apple")
	_, tail, ok2 := strings.Cut(rest, freeTextLine)
	if !ok1 || !ok2 {
		t.Fatal("the sample does not hold its options")
	}
	// withOptions replaces the sample's options by n options and the
	// free-text option numbered n+1.
	withOptions := func(n int) string {
		var options strings.Builder
		for k := 1; k <= n; k++ {
			fmt.Fprintf(&options, "  ┃  %d. Option %d\r\n", k, k)
		}
		fmt.Fprintf(&options, "  ┃  %d. Type your own answer\r\n", n+1)
		return head + options.String() + tail
	}
	if got := parseOpenCodeScreen(withOptions(8)); got.Kind != ScreenQuestion {
		t.Fatalf("options numbered up to 9 read as kind %v", got.Kind)
	}
	if got := parseOpenCodeScreen(withOptions(9)); got.Kind != ScreenUnmatched {
		t.Errorf("an option numbered 10 read as kind %v", got.Kind)
	}

	merge := plainSample(t, "claude-merge-question.ansi")
	// Wrap the question line itself, the last place the text appears (the
	// echoed prompt above it holds the same words).
	cut := strings.LastIndex(merge, "要約と詳細ファイルを確認のうえ）")
	if cut < 0 {
		t.Fatal("the sample does not hold the merge question")
	}
	wrapped := merge[:cut] + "要約と詳細\r\n" + merge[cut+len("要約と詳細"):]
	if got := parseClaudeScreen(merge); got.Question == nil || !got.Question.Merge {
		t.Fatal("the sample does not hold the merge question")
	}
	if got := parseClaudeScreen(wrapped); got.Kind != ScreenUnmatched {
		t.Errorf("a merge question wrapped inside a word read as kind %v", got.Kind)
	}
	// A terminal also wraps at a blank, which the reader then drops.
	atSpace := strings.LastIndex(merge, "（第 1 回")
	wrappedAtSpace := merge[:atSpace] + "（第\r\n   1 回" + merge[atSpace+len("（第 1 回"):]
	if got := parseClaudeScreen(wrappedAtSpace); got.Kind != ScreenUnmatched {
		t.Errorf("a merge question wrapped at a blank read as kind %v", got.Kind)
	}
}

func TestParseANSIScreenBounds(t *testing.T) {
	atCap := strings.Repeat("x\r\n", screenMaxLines)
	if _, err := parseANSIScreen(atCap); err != nil {
		t.Errorf("a screen of %d lines: %v", screenMaxLines, err)
	}
	if _, err := parseANSIScreen(atCap + "x"); !errors.Is(err, errScreenTooLarge) {
		t.Errorf("a screen of %d lines: %v, want errScreenTooLarge", screenMaxLines+1, err)
	}
	cases := map[string]struct {
		screen string
		target error
	}{
		"DEL":                 {"a\x7fb", errScreenEscape},
		"invalid UTF-8":       {"a\xffb", errScreenEncoding},
		"foreign escape":      {"\x1b[2Jx", errScreenEscape},
		"unknown SGR":         {"\x1b[58;5;1mx", errScreenSGR},
		"text over byte cap":  {strings.Repeat("x", screenMaxBytes), errScreenTooLarge},
		"C1 control":          {"a\u009bb", errScreenEscape},
		"color out of range":  {"\x1b[38;2;256;0;0mx", errScreenSGR},
		"short indexed color": {"\x1b[38;5mx", errScreenSGR},
		"unknown color mode":  {"\x1b[48;3;1mx", errScreenSGR},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseANSIScreen(c.screen); !errors.Is(err, c.target) {
				t.Errorf("error = %v, want %v", err, c.target)
			}
		})
	}
	// The byte cap counts text: one line of screenMaxBytes-1 bytes and its
	// line end is at the cap, and SGR sequences beyond the cap do not count.
	if _, err := parseANSIScreen(strings.Repeat("x", screenMaxBytes-1)); err != nil {
		t.Errorf("text at the byte cap: %v", err)
	}
	colored := strings.Repeat("\x1b[38;2;1;2;3mx", screenMaxBytes/len("\x1b[38;2;1;2;3m"))
	if len(colored) <= screenMaxBytes {
		t.Fatal("the colored screen is not over the cap in raw bytes")
	}
	if _, err := parseANSIScreen(colored); err != nil {
		t.Errorf("a screen over the cap only by its SGR sequences: %v", err)
	}
	// 48;2;1;2;3 consumes four fields, so its "2" does not set dim.
	lines, err := parseANSIScreen("\x1b[2ma\x1b[22mb\x1b[38;5;1;48;2;1;2;3mc")
	if err != nil {
		t.Fatal(err)
	}
	if c := lines[0].cells; !c[0].dim || c[1].dim || c[2].dim || c[2].fg.set {
		t.Errorf("cells = %+v, want dim only on the first", c)
	}
}

// TestReadCommandReport reads the final report of a command from the
// sampled Claude Code screen: found only when it names the PR's URL (also
// wrapped across lines, and not as a prefix of another PR's), and never on
// an OpenCode screen or a screen of another shape.
func TestReadCommandReport(t *testing.T) {
	const url = "https://github.com/owner/repo/pull/42"
	sample := readScreenSample(t, "claude-command-report.ansi")
	reportScreen(t, "")
	with := func(text string) string { return strings.Replace(sample, reportMention, text, 1) }
	cases := []struct {
		name   string
		agent  paneinput.AgentKind
		screen string
		url    string
		want   bool
	}{
		{"the sample, which names no URL", paneinput.AgentClaudeCode, sample, url, false},
		{"the URL in the report", paneinput.AgentClaudeCode, with(url + " に push しました"), url, true},
		{"the URL wrapped", paneinput.AgentClaudeCode, with("https://github.com/owner/\n  repo/pull/42 に push しました"), url, true},
		{"a longer PR number", paneinput.AgentClaudeCode, with(url + "0 に push しました"), url, false},
		{"the URL of another PR named too", paneinput.AgentClaudeCode, with(url + "0 と " + url + " に push しました"), url, true},
		{"an OpenCode screen", paneinput.AgentOpenCode, with(url), url, false},
		{"no URL asked", paneinput.AgentClaudeCode, with(url), "", false},
		{"a question screen", paneinput.AgentClaudeCode, readScreenSample(t, "claude-question-single.ansi"), url, false},
		{"a line outside the report's indent", paneinput.AgentClaudeCode, with(url + "\nunindented"), url, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			report, ok := readCommandReport(c.agent, c.screen, c.url)
			if ok != c.want {
				t.Fatalf("readCommandReport = %q, %v; want %v", report, ok, c.want)
			}
			if ok && !isSampleReport(report) {
				t.Errorf("report = %q, want the last message alone", report)
			}
		})
	}
}

// TestReadModelSwitch reads the /model records of the sampled transcript and
// of variants: the last record decides, its model is a whole word of the
// display name, and a line that is not JSON makes the transcript unreadable.
func TestReadModelSwitch(t *testing.T) {
	sample := readTestdata(t, "transcript/claude-clear-model.jsonl")
	if got := readModelSwitch(sample); got != (ModelSwitch{Kind: SwitchFound, Model: paneinput.ClaudeModelOpus}) {
		t.Errorf("the sample = %+v, want the last switch, to Opus", got)
	}
	lines := strings.Split(strings.TrimSpace(string(sample)), "\n")
	cases := []struct {
		name string
		text string
		want ModelSwitch
	}{
		{"up to the switch to Sonnet", strings.Join(lines[:len(lines)-3], "\n"), ModelSwitch{Kind: SwitchFound, Model: paneinput.ClaudeModelSonnet}},
		{"no switch", strings.Join(lines[:3], "\n"), ModelSwitch{Kind: SwitchNone}},
		{"a broken line", string(sample) + "{\"type\":", ModelSwitch{}},
		{"an empty transcript", "", ModelSwitch{Kind: SwitchNone}},
	}
	for _, c := range cases {
		if got := readModelSwitch([]byte(c.text)); got != c.want {
			t.Errorf("%s: readModelSwitch = %+v, want %+v", c.name, got, c.want)
		}
	}
	for name, want := range map[string]paneinput.ClaudeModel{
		"Opus 5.5":    paneinput.ClaudeModelOpus,
		"Sonnet 5.5":  paneinput.ClaudeModelSonnet,
		"opus":        paneinput.ClaudeModelOpus,
		"Opus Sonnet": paneinput.ClaudeModelUnknown,
		"Opus4.6":     paneinput.ClaudeModelUnknown,
		"Haiku 5.5":   paneinput.ClaudeModelUnknown,
	} {
		if got := routingModelOf(name); got != want {
			t.Errorf("routingModelOf(%q) = %d, want %d", name, got, want)
		}
	}
}

// reportMention is the PR mention of the sampled Claude Code report, which
// the tests replace with the URL of the PR under test.
const reportMention = "PR #867 に push しました"

// reportScreen returns the sampled Claude Code report with its PR mention
// replaced by text, such as the URL of the PR under test.
func reportScreen(t *testing.T, text string) string {
	t.Helper()
	screen := readScreenSample(t, "claude-command-report.ansi")
	if !strings.Contains(screen, reportMention) {
		t.Fatal("the report sample no longer holds its PR mention")
	}
	return strings.Replace(screen, reportMention, text+" に push しました", 1)
}

// isSampleReport reports whether text is the last message of the sampled
// report, from its first line to its last and nothing around it.
func isSampleReport(text string) bool {
	return strings.HasPrefix(text, "ADR-0004 を改訂し") && strings.HasSuffix(text, "を実行してください。")
}

// TestReadLastHumanInput reads the last human input of the sampled
// transcript and of its prefixes: only a line Claude Code declares the
// human's counts, so a peer's message or a task notification after a command
// leaves the command the last input; a command is read in the sampled form
// alone; a line that is not JSON makes the transcript unreadable.
func TestReadLastHumanInput(t *testing.T) {
	sample := strings.TrimSpace(string(readTestdata(t, "transcript/claude-human-input.jsonl")))
	lines := strings.Split(sample, "\n")
	queued := strings.Split(strings.TrimSpace(string(readTestdata(t, "transcript/claude-queued-input.jsonl"))), "\n")
	command := HumanInput{Kind: HumanInputCommand, Name: "runplan", Args: "0088"}
	cases := []struct {
		name string
		text string
		want HumanInput
	}{
		{"the whole sample, ending in typed text", sample, HumanInput{Kind: HumanInputOther}},
		{"up to the lines after the command", strings.Join(lines[:len(lines)-1], "\n"), command},
		{"up to the command", strings.Join(lines[:3], "\n"), command},
		{"before any human line", strings.Join(lines[:2], "\n"), HumanInput{Kind: HumanInputNone}},
		{"a broken line", sample + "\n{\"type\":", HumanInput{}},
		{"two names that differ", strings.Replace(strings.Join(lines[:3], "\n"), "<command-message>runplan<", "<command-message>mkplan<", 1), HumanInput{Kind: HumanInputOther}},
		{"the command line without its human origin", strings.Replace(strings.Join(lines[:3], "\n"), `"origin":{"kind":"human"},`, "", 1), HumanInput{Kind: HumanInputNone}},
		{"a later line of an unsampled origin", strings.Join(lines[:3], "\n") + "\n" + strings.Replace(lines[len(lines)-1], `"kind":"human"`, `"kind":"scheduled"`, 1), HumanInput{}},
		{"a message the human typed while the agent worked", strings.Join(lines[:3], "\n") + "\n" + queued[2], HumanInput{Kind: HumanInputOther}},
		{"a peer's message and a task notification delivered while the agent worked", strings.Join(lines[:3], "\n") + "\n" + queued[0] + "\n" + queued[1], command},
		{"a queued command of an unsampled origin", strings.Join(lines[:3], "\n") + "\n" + strings.Replace(queued[2], `"kind":"human"`, `"kind":"coordinator"`, 1), HumanInput{}},
		{"a queued command without an origin", strings.Join(lines[:3], "\n") + "\n" + strings.Replace(queued[2], `"origin":{"kind":"human"},`, "", 1), HumanInput{}},
		{"a later human line repeating a member", strings.Join(lines[:3], "\n") + "\n" + strings.Replace(lines[len(lines)-1], `"type":"user",`, `"type":"user","type":"user",`, 1), HumanInput{}},
	}
	for _, c := range cases {
		if got := readLastHumanInput([]byte(c.text)); got != c.want {
			t.Errorf("%s: readLastHumanInput = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// runprNoneLine is the NEXT line of the sampled OpenCode /runpr report.
const runprNoneLine = "     - NEXT: なし（タスク 0087 の最終 PR のため）。worktree は origin/"

// runprReportScreen returns the sampled OpenCode /runpr report with its PR
// number replaced by pr and, unless next is empty, its NEXT value replaced
// by next.
func runprReportScreen(t *testing.T, pr int, next string) string {
	t.Helper()
	screen := readScreenSample(t, "opencode-runpr-report.txt")
	if !strings.Contains(screen, runprNoneLine) {
		t.Fatal("the /runpr report sample no longer holds its NEXT line")
	}
	if next != "" {
		screen = strings.Replace(screen, runprNoneLine, "     - NEXT: "+next, 1)
	}
	return strings.ReplaceAll(screen, "867", strconv.Itoa(pr))
}

// TestReadRunprReport reads the last report of the sampled OpenCode /runpr
// report and Claude Code command report, and checks that a screen whose last
// response is not in the sampled shape gives none.
func TestReadRunprReport(t *testing.T) {
	report, ok := readRunprReport(paneinput.AgentOpenCode, readScreenSample(t, "opencode-runpr-report.txt"))
	if !ok || report.Next != (RunprNext{Kind: NextNone}) || !slices.Contains(report.PRs, 867) {
		t.Fatalf("OpenCode report = %+v, %v; want NEXT none naming #867", report, ok)
	}
	if slices.Contains(report.PRs, 86) || slices.Contains(report.PRs, 1013033322) {
		t.Errorf("OpenCode report PRs = %v; a number outside the last report or a part of one was read", report.PRs)
	}
	report, ok = readRunprReport(paneinput.AgentOpenCode, runprReportScreen(t, 42, "`/runplan 0088`（要件 opus → Opus）"))
	if !ok || report.Next != (RunprNext{Kind: NextCommand, Command: NextCommandRunplan, Task: taskID(t, "0088")}) || !slices.Contains(report.PRs, 42) {
		t.Errorf("edited OpenCode report = %+v, %v", report, ok)
	}
	spaced := strings.Replace(runprReportScreen(t, 42, ""), "PR #42 のマージまで", "PR #42 1 件のマージまで", 1)
	if report, ok := readRunprReport(paneinput.AgentOpenCode, spaced); !ok || !slices.Contains(report.PRs, 42) || slices.Contains(report.PRs, 421) {
		t.Errorf("a number after a space = %+v, %v; want #42 and not #421", report, ok)
	}
	// Only this line names #42; the sample's other mentions stay #867.
	split := strings.Replace(readScreenSample(t, "opencode-runpr-report.txt"), "     PR #867 のマージまで完了しました。\n", "     PR #42\n     1 件のマージまで完了しました。\n", 1)
	if report, ok := readRunprReport(paneinput.AgentOpenCode, split); !ok || !slices.Contains(report.PRs, 42) {
		t.Errorf("a number at a line end followed by a line of digits = %+v, %v; want #42", report, ok)
	}
	report, ok = readRunprReport(paneinput.AgentClaudeCode, readScreenSample(t, "claude-command-report.ansi"))
	if !ok || report.Next.Kind != NextAbsent || !slices.Contains(report.PRs, 867) {
		t.Errorf("Claude Code report = %+v, %v; want no NEXT naming #867", report, ok)
	}
	plain := readScreenSample(t, "opencode-runpr-report.txt")
	end := "     Build · DeepSeek V4.1 Flash 1017 · 31m 33s · 59.8 tok/s\n"
	if !strings.Contains(plain, end) {
		t.Fatal("the sample no longer holds its response end line")
	}
	thought := "     + Thought · 9.6s\n"
	i, j := strings.LastIndex(plain, thought), strings.Index(plain, end)
	if i < 0 || j < i {
		t.Fatal("the sample no longer holds its last thought line before the end line")
	}
	for name, screen := range map[string]string{
		"a response of a thought only": plain[:i+len(thought)] + plain[j:],
		"no end line":                  strings.Replace(plain, end, "", 1),
		"an unindented report line":    strings.Replace(plain, "     PR レビューと CI\n", "PR レビューと CI\n", 1),
		"the report under the input":   strings.Replace(plain, end, end+"  ┃\n", 1),
	} {
		if screen == plain {
			t.Fatalf("%s: the edit changed nothing", name)
		}
		if got, ok := readRunprReport(paneinput.AgentOpenCode, screen); ok {
			t.Errorf("%s: read as %+v", name, got)
		}
	}
	for name, c := range map[string]struct {
		agent  paneinput.AgentKind
		sample string
	}{
		"an OpenCode screen read as Claude Code": {paneinput.AgentClaudeCode, "opencode-runpr-report.txt"},
		"a Claude Code screen with no message":   {paneinput.AgentClaudeCode, "claude-idle.ansi"},
		"an agent chief does not know":           {paneinput.AgentUnknown, "claude-command-report.ansi"},
	} {
		if got, ok := readRunprReport(c.agent, readScreenSample(t, c.sample)); ok {
			t.Errorf("%s: read as %+v", name, got)
		}
	}
}

// TestReadScreenNext checks that the NEXT lines of a whole ANSI screen are
// read, and that a screen the ANSI reader refuses gives none.
func TestReadScreenNext(t *testing.T) {
	next, ok := readScreenNext(readScreenSample(t, "claude-merge-question.ansi"))
	if !ok || next != (RunprNext{Kind: NextCommand, Command: NextCommandMkarch, Task: taskID(t, "0123")}) {
		t.Errorf("merge question = %+v, %v", next, ok)
	}
	if next, ok := readScreenNext(readScreenSample(t, "claude-idle.ansi")); !ok || next.Kind != NextAbsent {
		t.Errorf("idle screen = %+v, %v; want no NEXT", next, ok)
	}
	if _, ok := readScreenNext("NEXT: /mkplan 0007\x1b[2J"); ok {
		t.Error("a screen with a non-SGR escape was read")
	}
}
