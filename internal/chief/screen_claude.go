package chief

import (
	"regexp"
	"strconv"
	"strings"
)

// Fixed lines of Claude Code 2.1.322 screens, as the samples show them.
const (
	claudeFooterSingle     = "Enter to select · ↑/↓ to navigate · Esc to cancel"
	claudeFooterTabs       = "Enter to select · Tab/Arrow keys to navigate · Esc to cancel"
	claudeFooterPermission = " Esc to cancel · Tab to amend"
	claudeProceed          = " Do you want to proceed?"
	claudeFreeText         = "Type something."
	claudeFreeTextMulti    = "Type something"
	claudeMultiSubmit      = "     Submit"
	claudeChat             = "Chat about this"
	claudeReviewTitle      = "Review your answers"
	claudeReviewAsk        = "Ready to submit your answers?"
	claudeReviewSubmit     = "Submit answers"
	claudeReviewCancel     = "Cancel"
	claudeCursor           = "❯ "
	claudeNoCursor         = "  "
	claudePromptMark       = "❯\u00a0"
	claudeEmptyPrompt      = "❯"
	claudeDescIndent       = "     "
	claudeMultiDescIndent  = "         "
	claudeMultiBox         = "[ ] "
	claudePermissionIndent = " "
	claudeTabSubmit        = "✔ Submit"
	claudeTabOpen          = "☐ "
	claudeTabAnswered      = "☒ "
	claudeTabSeparator     = "  "
	claudeStatusIndent     = "  "
)

// Line counts of the fixed parts at the bottom of Claude Code widgets.
const (
	// claudeWidgetTail is a rule, the chat option, a blank line, and the
	// footer.
	claudeWidgetTail = 4
	// claudeReviewTail is the submit question, a blank line, and the two
	// options.
	claudeReviewTail = 4
	// claudePermissionTail is a blank line and the footer.
	claudePermissionTail = 2
	// claudeMinOptions is the fewest options a question has besides its
	// free-text option.
	claudeMinOptions = 1
	// claudeMinTabs is the fewest tabs of a tabbed question.
	claudeMinTabs = 2
)

var (
	// claudeHeader matches the header chip of a one-question widget.
	claudeHeader = regexp.MustCompile(`^ ☐ (.+)$`)
	// claudePlaceholderText matches the placeholder of an empty input line:
	// one quoted suggestion after "Try".
	claudePlaceholderText = regexp.MustCompile(`^Try "[^"\n]+"$`)
	// claudeTabBar matches the tab bar of a multi-select or tabbed widget.
	claudeTabBar = regexp.MustCompile(`^←  (.+)  →$`)
)

// parseClaudeScreen reads a Claude Code pane screen. Contract: it returns a
// question or a prompt only for the shapes of the samples, with every line of
// the widget accounted for; any other screen is ScreenUnmatched.
func parseClaudeScreen(raw string) Screen {
	all, err := parseANSIScreen(raw)
	if err != nil {
		return unmatched
	}
	lines := trimmedLines(all)
	if len(lines) == 0 {
		return unmatched
	}
	var q *Question
	switch lines[len(lines)-1].text {
	case claudeFooterSingle:
		q = claudeOneQuestion(lines, false)
	case claudeFooterTabs:
		q = claudeOneQuestion(lines, true)
	case claudeFooterPermission:
		q = claudePermission(lines)
	default:
		if q = claudeTabsReview(lines); q == nil {
			return claudePrompt(lines)
		}
	}
	if q == nil || !finishQuestion(q) {
		return unmatched
	}
	return Screen{Kind: ScreenQuestion, Question: q}
}

// claudeOptionLine is an option line split into its parts.
type claudeOptionLine struct {
	cursor bool
	number int
	label  string
}

// claudeOption splits an option line into its cursor mark, number, and
// label. indent is the text before the cursor column.
func claudeOption(text, indent string) (claudeOptionLine, bool) {
	body, found := strings.CutPrefix(text, indent)
	if !found {
		return claudeOptionLine{}, false
	}
	var o claudeOptionLine
	switch {
	case strings.HasPrefix(body, claudeCursor):
		o.cursor, body = true, body[len(claudeCursor):]
	case strings.HasPrefix(body, claudeNoCursor):
		body = body[len(claudeNoCursor):]
	default:
		return claudeOptionLine{}, false
	}
	m := numberedOption.FindStringSubmatch(body)
	if m == nil {
		return claudeOptionLine{}, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return claudeOptionLine{}, false
	}
	o.number, o.label = n, m[2]
	return o, true
}

// lastRuleBefore returns the index of the last box-drawing rule (U+2500)
// before end, or -1.
func lastRuleBefore(lines []screenLine, end int) int {
	for i := end - 1; i >= 0; i-- {
		if isRule(lines[i].text, '─') {
			return i
		}
	}
	return -1
}

// nonBlank returns the lines of a widget without its blank spacer lines.
func nonBlank(lines []screenLine) []screenLine {
	out := make([]screenLine, 0, len(lines))
	for _, l := range lines {
		if l.text != "" {
			out = append(out, l)
		}
	}
	return out
}

// claudeTabNames reads the names of a tab bar whose last entry is Submit.
func claudeTabNames(bar string) ([]string, bool) {
	m := claudeTabBar.FindStringSubmatch(bar)
	if m == nil {
		return nil, false
	}
	entries := strings.Split(m[1], claudeTabSeparator)
	last := len(entries) - 1
	if last < 1 || entries[last] != claudeTabSubmit {
		return nil, false
	}
	names := make([]string, 0, last)
	for _, e := range entries[:last] {
		name, ok := strings.CutPrefix(e, claudeTabOpen)
		if !ok {
			name, ok = strings.CutPrefix(e, claudeTabAnswered)
		}
		if !ok || name == "" {
			return nil, false
		}
		names = append(names, name)
	}
	return names, true
}

// claudeWidget is the part of a one-question widget above its fixed bottom.
type claudeWidget struct {
	block []screenLine
	chat  claudeOptionLine
}

// claudeWidgetBody splits a one-question widget: from the last rule above
// the bottom, the non-blank lines, and the chat option of the bottom (a rule,
// the chat option, a blank line, and the footer).
func claudeWidgetBody(lines []screenLine) (claudeWidget, bool) {
	start := len(lines) - claudeWidgetTail
	if start < 1 {
		return claudeWidget{}, false
	}
	tail := lines[start:]
	if !isRule(tail[0].text, '─') || tail[2].text != "" {
		return claudeWidget{}, false
	}
	chat, ok := claudeOption(tail[1].text, "")
	if !ok || chat.label != claudeChat {
		return claudeWidget{}, false
	}
	top := lastRuleBefore(lines, start)
	if top < 0 {
		return claudeWidget{}, false
	}
	return claudeWidget{block: nonBlank(lines[top+1 : start]), chat: chat}, true
}

// claudeOneQuestion reads the widget of one question: a single-select or
// multi-select question (tabbed false) or the current tab of a tabbed question
// (tabbed true): header or tab bar, question text, numbered options with
// indented descriptions, and the free-text option, above the fixed bottom.
func claudeOneQuestion(lines []screenLine, tabbed bool) *Question {
	w, ok := claudeWidgetBody(lines)
	if !ok || len(w.block) == 0 {
		return nil
	}
	q := &Question{}
	multi, ok := claudeWidgetHead(q, w.block[0].text, tabbed)
	if !ok {
		return nil
	}
	body := w.block[1:]
	if multi {
		if len(body) == 0 || body[len(body)-1].text != claudeMultiSubmit {
			return nil
		}
		body = body[:len(body)-1]
	}
	first := 0
	for first < len(body) {
		if o, isOption := claudeOption(body[first].text, ""); isOption && o.number == 1 {
			break
		}
		first++
	}
	if first == 0 || first == len(body) {
		return nil
	}
	var text []string
	for _, l := range body[:first] {
		text = append(text, l.text)
	}
	q.Text = joinTexts(text)
	cursors, ok := claudeOptions(q, body[first:], multi)
	if !ok || cursors+boolCount(w.chat.cursor) != 1 || w.chat.number != len(q.Options)+1 {
		return nil
	}
	q.Options = append(q.Options, Option{Number: w.chat.number, Label: w.chat.label})
	switch {
	case tabbed:
		q.Kind, q.Relay = QuestionTabs, RelayBlockTabs
	case multi:
		q.Kind, q.Relay = QuestionMulti, RelayBlockMulti
	default:
		q.Kind, q.Relay = QuestionSingle, RelayAllowed
	}
	return q
}

// claudeWidgetHead reads the first line of a one-question widget: the
// header chip of a single-select question, or a tab bar, which has one tab
// for a multi-select question and two or more for a tabbed one. It returns
// whether the widget is a multi-select question.
func claudeWidgetHead(q *Question, head string, tabbed bool) (bool, bool) {
	names, isBar := claudeTabNames(head)
	if !isBar {
		return false, !tabbed && claudeHeader.MatchString(head)
	}
	q.Tabs = names
	if tabbed {
		return false, len(names) >= claudeMinTabs
	}
	return true, len(names) == 1
}

// boolCount is 1 for true and 0 for false.
func boolCount(b bool) int {
	if b {
		return 1
	}
	return 0
}

// claudeOptions reads numbered options, each followed by its description
// lines, ending with the free-text option, into q. It returns how many
// options carry the cursor.
func claudeOptions(q *Question, lines []screenLine, multi bool) (int, bool) {
	descIndent, freeTextLabel := claudeDescIndent, claudeFreeText
	if multi {
		descIndent, freeTextLabel = claudeMultiDescIndent, claudeFreeTextMulti
	}
	cursors := 0
	freeText := false
	for i := 0; i < len(lines); {
		o, isOption := claudeOption(lines[i].text, "")
		if freeText || !isOption || o.number != len(q.Options)+1 {
			return 0, false
		}
		cursors += boolCount(o.cursor)
		label := o.label
		if multi {
			var boxed bool
			if label, boxed = strings.CutPrefix(label, claudeMultiBox); !boxed {
				return 0, false
			}
		}
		freeText = label == freeTextLabel
		var desc []string
		for i++; i < len(lines); i++ {
			d, isDesc := strings.CutPrefix(lines[i].text, descIndent)
			if !isDesc || d == "" || d[0] == ' ' {
				break
			}
			desc = append(desc, d)
		}
		q.Options = append(q.Options, Option{Number: o.number, Label: label, Description: joinTexts(desc), Relayable: !freeText})
	}
	if multi && freeText {
		// The free-text option of a multi-select question keeps its box in
		// the label shown to the human, as the screen draws it.
		q.Options[len(q.Options)-1].Label = claudeMultiBox + freeTextLabel
	}
	return cursors, freeText && len(q.Options) > claudeMinOptions
}

// claudeTabsReview reads the last page of a tabbed question, which asks to
// submit the answers given so far. It has no footer, so it is recognized by
// its tab bar, title, and two fixed options at the bottom of the screen.
func claudeTabsReview(lines []screenLine) *Question {
	start := len(lines) - claudeReviewTail
	if start < 1 {
		return nil
	}
	tail := lines[start:]
	submit, ok1 := claudeOption(tail[2].text, "")
	cancel, ok2 := claudeOption(tail[3].text, "")
	if !ok1 || !ok2 || submit.label != claudeReviewSubmit || cancel.label != claudeReviewCancel ||
		submit.number != 1 || cancel.number != submit.number+1 || submit.cursor == cancel.cursor ||
		tail[0].text != claudeReviewAsk || tail[1].text != "" {
		return nil
	}
	top := lastRuleBefore(lines, start)
	if top < 0 {
		return nil
	}
	block := nonBlank(lines[top+1 : start+1])
	if len(block) < claudeMinTabs {
		return nil
	}
	names, isBar := claudeTabNames(block[0].text)
	if !isBar || len(names) < claudeMinTabs || block[1].text != claudeReviewTitle {
		return nil
	}
	var text []string
	for _, l := range block[1:] {
		text = append(text, l.text)
	}
	return &Question{
		Kind: QuestionTabs, Relay: RelayBlockTabs, Text: joinTexts(text), Tabs: names,
		Options: []Option{{Number: submit.number, Label: submit.label}, {Number: cancel.number, Label: cancel.label}},
	}
}

// claudePermission reads a permission prompt: from a rule, a tool header and
// its details, the proceed line, numbered options indented by one space, a
// blank line, and the footer.
func claudePermission(lines []screenLine) *Question {
	end := len(lines) - claudePermissionTail
	if end < 1 || lines[end].text != "" {
		return nil
	}
	first := end
	for first > 0 {
		if _, ok := claudeOption(lines[first-1].text, claudePermissionIndent); !ok {
			break
		}
		first--
	}
	if first == end || first == 0 || lines[first-1].text != claudeProceed {
		return nil
	}
	top := lastRuleBefore(lines, first-1)
	if top < 0 {
		return nil
	}
	q := &Question{Kind: QuestionPermission, Relay: RelayBlockPermission}
	var text []string
	for _, l := range lines[top+1 : first] {
		t, ok := strings.CutPrefix(l.text, claudePermissionIndent)
		if !ok && !isRule(l.text, '╌') {
			return nil
		}
		text = append(text, t)
	}
	q.Text = joinTexts(text)
	cursors := 0
	for _, l := range lines[first:end] {
		o, _ := claudeOption(l.text, claudePermissionIndent)
		if o.number != len(q.Options)+1 {
			return nil
		}
		cursors += boolCount(o.cursor)
		q.Options = append(q.Options, Option{Number: o.number, Label: o.label})
	}
	if cursors != 1 {
		return nil
	}
	return q
}

// claudePrompt reads the input box at the bottom of a Claude Code screen: a
// rule, the input lines (the first starts with the prompt mark), a rule, and
// one or more status lines. Placeholder text is drawn dim, so the input is
// empty when every visible character on it is dim.
func claudePrompt(lines []screenLine) Screen {
	bottom := lastRuleBefore(lines, len(lines))
	if bottom < 0 || bottom == len(lines)-1 {
		return unmatched
	}
	for _, l := range lines[bottom+1:] {
		if !strings.HasPrefix(l.text, claudeStatusIndent) {
			return unmatched
		}
	}
	top := lastRuleBefore(lines, bottom)
	if top < 0 || top+1 == bottom {
		return unmatched
	}
	input := lines[top+1 : bottom]
	if input[0].text != claudeEmptyPrompt && !strings.HasPrefix(input[0].text, claudePromptMark) {
		return unmatched
	}
	// Every input line starts with two columns: the prompt mark and its
	// space, or the continuation indent.
	lead := len([]rune(claudePromptMark))
	var left []string
	allDim := true
	for k, l := range input {
		if k > 0 && !strings.HasPrefix(l.text, claudeStatusIndent) {
			return unmatched
		}
		var b strings.Builder
		for _, c := range l.cells[min(len(l.cells), lead):] {
			b.WriteRune(c.r)
			if c.r != ' ' && !c.dim {
				allDim = false
			}
		}
		left = append(left, b.String())
	}
	text := strings.TrimSpace(joinTexts(left))
	// The input is empty when it holds nothing, or only the placeholder: the
	// sampled placeholder form, which has no line break, every visible
	// character dim.
	empty := text == "" || (allDim && claudePlaceholderText.MatchString(text))
	screen := Screen{Kind: ScreenPrompt, Input: InputLine{Empty: empty}}
	if !empty {
		screen.Input.Left = text
	}
	return screen
}
