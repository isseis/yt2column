//go:build test

package chief

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
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
}

const sampleModel = "DeepSeek V4.1 Flash 1017 OpenCode Go"

var (
	fruitLabels       = []string{"Apple", "Banana", "Cherry"}
	claudeFruitSingle = wantScreen{
		kind: ScreenQuestion, question: QuestionSingle, relay: RelayAllowed,
		text:      "Which fruit do you prefer?",
		labels:    append(append([]string{}, fruitLabels...), "Type something.", "Chat about this"),
		relayable: fruitLabels,
	}
	openCodeFruitSingle = wantScreen{
		kind: ScreenQuestion, question: QuestionSingle, relay: RelayAllowed,
		text:      "Which fruit do you prefer?",
		labels:    append(append([]string{}, fruitLabels...), "Type your own answer"),
		relayable: fruitLabels,
	}
	mergeText   = "PR #123 をどうしますか（第 1 回。要約と詳細ファイルを確認のうえ）"
	mergeLabels = []string{"マージ", "マージしない"}
)

func TestParseScreenSamples(t *testing.T) {
	claude := map[string]wantScreen{
		"claude-idle.ansi":                   {kind: ScreenPrompt, inputEmpty: true},
		"claude-working.ansi":                {kind: ScreenPrompt, inputEmpty: true},
		"claude-after-clear.ansi":            {kind: ScreenPrompt, inputEmpty: true},
		"claude-after-model.ansi":            {kind: ScreenPrompt, inputEmpty: true},
		"claude-command-report.ansi":         {kind: ScreenPrompt, inputEmpty: true},
		"claude-input-left.ansi":             {kind: ScreenPrompt, inputLeft: "unsent draft text"},
		"claude-question-single.ansi":        claudeFruitSingle,
		"claude-question-single-cursor.ansi": claudeFruitSingle,
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
		"claude-merge-question.ansi": {
			kind: ScreenQuestion, question: QuestionSingle, relay: RelayAllowed, merge: true,
			text:      mergeText,
			labels:    append(append([]string{}, mergeLabels...), "Type something.", "Chat about this"),
			relayable: mergeLabels,
		},
	}
	openCode := map[string]wantScreen{
		"opencode-done.ansi":                   {kind: ScreenPrompt, inputEmpty: true, model: sampleModel},
		"opencode-after-new.ansi":              {kind: ScreenPrompt, inputEmpty: true, model: sampleModel, newSession: true},
		"opencode-input-left.ansi":             {kind: ScreenPrompt, inputLeft: "unsent draft text", model: sampleModel},
		"opencode-runpr-report.txt":            {kind: ScreenPrompt, inputEmpty: true, model: sampleModel},
		"opencode-question-single.ansi":        openCodeFruitSingle,
		"opencode-question-single-cursor.ansi": openCodeFruitSingle,
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
		},
	}
	readers := []struct {
		name         string
		parse, other func(string) Screen
		samples      map[string]wantScreen
	}{
		{"claude", parseClaudeScreen, parseOpenCodeScreen, claude},
		{"opencode", parseOpenCodeScreen, parseClaudeScreen, openCode},
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
		"unknown SGR parameter":   strings.Replace(readScreenSample(t, cq), "Which fruit", "\x1b[58;5;1mWhich fruit", 1),
		"invalid UTF-8":           strings.Replace(readScreenSample(t, cq), "Which fruit", "\xffWhich fruit", 1),
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
	if got := readRunprNext(merge); got != (RunprNext{Kind: NextCommand, Command: NextCommandMkarch, Task: "0123"}) {
		t.Errorf("merge question sample: %+v", got)
	}
	cases := map[string]struct {
		text string
		want RunprNext
	}{
		"no NEXT line":        {"done\n", RunprNext{Kind: NextAbsent}},
		"runplan in quotes":   {"- NEXT: `/runplan 0088`（要件 opus → Opus）", RunprNext{Kind: NextCommand, Command: NextCommandRunplan, Task: "0088"}},
		"mkplan":              {"NEXT: /mkplan 0007", RunprNext{Kind: NextCommand, Command: NextCommandMkplan, Task: "0007"}},
		"agreeing lines":      {"NEXT: /mkplan 0007\n  - NEXT: `/mkplan 0007`（次）", RunprNext{Kind: NextCommand, Command: NextCommandMkplan, Task: "0007"}},
		"none with more text": {"NEXT: なしではなく /runplan 0088", RunprNext{}},
		"task with a suffix":  {"NEXT: /runplan 0088abc", RunprNext{}},
		"unpaired backquote":  {"NEXT: `/runplan 0088", RunprNext{}},
		"disagreeing lines":   {"NEXT: /mkplan 0007\nNEXT: なし", RunprNext{}},
		"unknown command":     {"NEXT: /runpr 12", RunprNext{}},
		"five-digit task":     {"NEXT: /runplan 00881", RunprNext{}},
		"three-digit task":    {"NEXT: /runplan 088", RunprNext{}},
		"free text value":     {"NEXT: see above", RunprNext{}},
		"NEXT inside a line":  {"the NEXT: /mkplan 0007 line", RunprNext{Kind: NextAbsent}},
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
	if got := parseClaudeScreen(merge); !got.Question.Merge {
		t.Fatal("the sample does not hold the merge question")
	}
	if got := parseClaudeScreen(wrapped); got.Kind != ScreenUnmatched {
		t.Errorf("a wrapped merge question read as kind %v", got.Kind)
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
	lines, err := parseANSIScreen("\x1b[2ma\x1b[22mb\x1b[38;5;1;48;2;1;2;3mc")
	if err != nil {
		t.Fatal(err)
	}
	if c := lines[0].cells; !c[0].dim || c[1].dim || c[2].dim || c[2].fg.set {
		t.Errorf("cells = %+v, want dim only on the first", c)
	}
}
