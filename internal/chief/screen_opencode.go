package chief

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Fixed lines of OpenCode v2.2.6 screens, as the samples show them. The
// texts are the contents of a panel line after its bar and two spaces.
const (
	openCodeBar               = "┃"
	openCodePad               = "  "
	openCodeQuestions         = "Questions"
	openCodeFooterSingle      = "↑↓ select  enter submit  esc dismiss"
	openCodeFooterMulti       = "⇆ tab  ↑↓ select  enter toggle  esc dismiss"
	openCodeFooterTabs        = "⇆ tab  ↑↓ select  enter confirm  esc dismiss"
	openCodeFooterReview      = "⇆ tab  enter submit  esc dismiss"
	openCodeFreeText          = "Type your own answer"
	openCodeTabSubmit         = "Submit"
	openCodeTabSeparator      = "  "
	openCodeMultiBox          = "[ ] "
	openCodeDescIndent        = "   "
	openCodeMultiDescIndent   = "       "
	openCodeModelSeparator    = " · "
	openCodePlaceholderPrefix = "Ask anything… "
	openCodeUpdatePrefix      = "/update to install v"
)

// maxOpenCodeStatusLines is how many non-blank lines OpenCode draws below
// its input box: the path line and, in a new session, the version line.
const maxOpenCodeStatusLines = 2

// openCodePlaceholderColor is the foreground of the input placeholder. A
// line drawn in any other color is read as typed text, so a changed theme
// stops sends rather than allowing them.
var openCodePlaceholderColor = rgb{set: true, r: openCodePlaceholderGray, g: openCodePlaceholderGray, b: openCodePlaceholderGray}

// Sizes of the fixed parts of OpenCode panels.
const (
	// openCodePlaceholderGray is each component of the placeholder color.
	openCodePlaceholderGray = 138
	// openCodeMinBox is the input box: a blank line, an input line, a blank
	// line, and the status line.
	openCodeMinBox = 4
	// openCodeBoxPadding is the blank line before the status line.
	openCodeBoxPadding = 2
	// openCodeMinTabs is the fewest tabs of a tabbed question.
	openCodeMinTabs = 2
	// openCodeMinOptions is the fewest options a question has, its free-text
	// option included.
	openCodeMinOptions = 2
)

// openCodeLogo is the logo a new OpenCode session draws above its input box.
var openCodeLogo = []string{
	"▄",
	"█▀▀█ █▀▀█ █▀▀█ █▀▀▄ █▀▀▀ █▀▀█ █▀▀█ █▀▀█",
	"█  █ █  █ █▀▀▀ █  █ █    █  █ █  █ █▀▀▀",
	"▀▀▀▀ █▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀ ▀▀▀▀",
}

// panelLine is a line of an OpenCode panel: the cells after the bar.
type panelLine struct {
	cells []cell
	text  string
}

// asPanel returns the part of a line after its leading blanks and the panel
// bar, or false when the line is not a panel line.
func asPanel(l screenLine) (panelLine, bool) {
	i := 0
	for i < len(l.cells) && l.cells[i].r == ' ' {
		i++
	}
	if i == len(l.cells) || string(l.cells[i].r) != openCodeBar {
		return panelLine{}, false
	}
	cells := l.cells[i+1:]
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.r)
	}
	return panelLine{cells: cells, text: b.String()}, true
}

// panelContent returns a panel line's text after the two-space pad; an empty
// panel line has empty content.
func panelContent(p panelLine) (string, bool) {
	if p.text == "" {
		return "", true
	}
	return strings.CutPrefix(p.text, openCodePad)
}

// parseOpenCodeScreen reads an OpenCode pane screen. Contract: it returns a
// question or a prompt only for the shapes of the samples; any other screen
// is ScreenUnmatched.
func parseOpenCodeScreen(raw string) Screen {
	all, err := parseANSIScreen(raw)
	if err != nil {
		return unmatched
	}
	lines := trimmedLines(all)
	if len(lines) == 0 {
		return unmatched
	}
	if q, isQuestion := openCodeQuestion(lines); isQuestion {
		if q == nil || !finishQuestion(q) {
			return unmatched
		}
		return Screen{Kind: ScreenQuestion, Question: q}
	}
	return openCodePrompt(lines)
}

// openCodeQuestion reads the question panel at the bottom of the screen. The
// second result is false when the bottom is not a question footer; the
// question is nil when the footer is there but the panel does not match.
func openCodeQuestion(lines []screenLine) (*Question, bool) {
	var contents []string
	end := len(lines)
	for end > 0 {
		p, ok := asPanel(lines[end-1])
		if !ok || p.text != "" {
			break
		}
		end--
	}
	if end == 0 {
		return nil, false
	}
	p, ok := asPanel(lines[end-1])
	if !ok {
		return nil, false
	}
	footer, _ := panelContent(p)
	var kind QuestionKind
	review := false
	switch footer {
	case openCodeFooterSingle:
		kind = QuestionSingle
	case openCodeFooterMulti:
		kind = QuestionMulti
	case openCodeFooterTabs:
		kind = QuestionTabs
	case openCodeFooterReview:
		kind, review = QuestionTabs, true
	default:
		return nil, false
	}
	footerAt := end - 1
	start := -1
	for i := footerAt - 1; i >= 0; i-- {
		p, ok := asPanel(lines[i])
		if !ok {
			return nil, true
		}
		c, ok := panelContent(p)
		if !ok {
			return nil, true
		}
		if c == openCodeQuestions {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, true
	}
	for _, l := range lines[start+1 : footerAt] {
		p, _ := asPanel(l)
		c, _ := panelContent(p)
		contents = append(contents, c)
	}
	return openCodeQuestionBody(contents, kind, review), true
}

// openCodeQuestionBody reads the panel between "Questions" and the footer:
// a blank line, an optional tab bar and blank line, the question text, a
// blank line, the options, and a blank line.
func openCodeQuestionBody(c []string, kind QuestionKind, review bool) *Question {
	if len(c) < 2 || c[0] != "" || c[len(c)-1] != "" {
		return nil
	}
	q := &Question{Kind: kind}
	c, ok := openCodeTabBar(q, c[1:len(c)-1])
	if !ok {
		return nil
	}
	if review {
		if len(c) == 0 || slices.Contains(c, "") {
			return nil
		}
		q.Text, q.Relay = joinTexts(c), RelayBlockTabs
		return q
	}
	blank := slices.Index(c, "")
	if blank < 1 {
		return nil
	}
	q.Text = joinTexts(c[:blank])
	if !openCodeOptions(q, c[blank+1:], kind == QuestionMulti) {
		return nil
	}
	switch kind {
	case QuestionSingle:
		q.Relay = RelayAllowed
	case QuestionMulti:
		q.Relay = RelayBlockMulti
	default:
		q.Relay = RelayBlockTabs
	}
	return q
}

// openCodeTabBar reads the optional tab bar and the blank line after it, and
// checks that the bar is there exactly for the question kinds that draw it: a
// multi-select question has a bar with its one header, a tabbed question a
// bar with two or more, a single-select question none.
func openCodeTabBar(q *Question, c []string) ([]string, bool) {
	suffix := openCodeTabSeparator + openCodeTabSubmit
	hasBar := len(c) > 1 && strings.HasSuffix(c[0], suffix) && c[1] == ""
	if hasBar {
		names := strings.Split(strings.TrimSuffix(c[0], suffix), openCodeTabSeparator)
		if slices.Contains(names, "") {
			return nil, false
		}
		q.Tabs = names
		c = c[2:]
	}
	switch q.Kind {
	case QuestionSingle:
		return c, !hasBar
	case QuestionMulti:
		return c, hasBar && len(q.Tabs) == 1
	case QuestionTabs:
		return c, hasBar && len(q.Tabs) >= openCodeMinTabs
	default:
		return nil, false
	}
}

// openCodeOptions reads numbered options with indented descriptions, ending
// with the free-text option.
func openCodeOptions(q *Question, c []string, multi bool) bool {
	descIndent := openCodeDescIndent
	if multi {
		descIndent = openCodeMultiDescIndent
	}
	freeText := false
	for i := 0; i < len(c); {
		if freeText {
			return false
		}
		m := numberedOption.FindStringSubmatch(c[i])
		if m == nil || m[1] != strconv.Itoa(len(q.Options)+1) {
			return false
		}
		label := m[2]
		if multi {
			var ok bool
			if label, ok = strings.CutPrefix(label, openCodeMultiBox); !ok {
				return false
			}
		}
		freeText = label == openCodeFreeText
		opt := Option{Number: len(q.Options) + 1, Label: label, Relayable: !freeText}
		i++
		var desc []string
		for i < len(c) {
			d, isDesc := strings.CutPrefix(c[i], descIndent)
			if !isDesc || d == "" || d[0] == ' ' {
				break
			}
			desc = append(desc, d)
			i++
		}
		opt.Description = joinTexts(desc)
		q.Options = append(q.Options, opt)
	}
	return freeText && len(q.Options) >= openCodeMinOptions
}

// openCodePrompt reads the input box at the bottom of an OpenCode screen: a
// run of panel lines made of a blank line, the input lines, a blank line, and
// the status line, closed by the box's bottom edge and followed by at most two
// status lines.
func openCodePrompt(lines []screenLine) Screen {
	edge := -1
	statusLines := 0
	for i := len(lines) - 1; i >= 0 && statusLines <= maxOpenCodeStatusLines; i-- {
		t := strings.TrimLeft(lines[i].text, " ")
		if r, rest, ok := strings.Cut(t, "╹"); ok && r == "" && isRule(rest, '▀') {
			edge = i
			break
		}
		if t != "" {
			statusLines++
		}
	}
	if edge < openCodeMinBox {
		return unmatched
	}
	first := edge
	for first > 0 {
		if _, ok := asPanel(lines[first-1]); !ok {
			break
		}
		first--
	}
	box := make([]panelLine, 0, edge-first)
	for _, l := range lines[first:edge] {
		p, _ := asPanel(l)
		box = append(box, p)
	}
	if len(box) < openCodeMinBox || box[0].text != "" || box[len(box)-openCodeBoxPadding].text != "" {
		return unmatched
	}
	status, ok := panelContent(box[len(box)-1])
	if !ok {
		return unmatched
	}
	firstSep := strings.Index(status, openCodeModelSeparator)
	lastSep := strings.LastIndex(status, openCodeModelSeparator)
	if firstSep < 1 || lastSep <= firstSep {
		return unmatched
	}
	model := status[firstSep+len(openCodeModelSeparator) : lastSep]
	input, ok := openCodeInput(box[1 : len(box)-openCodeBoxPadding])
	if !ok {
		return unmatched
	}
	return Screen{
		Kind:          ScreenPrompt,
		Input:         input,
		OpenCodeModel: model,
		NewSession:    openCodeNewSession(lines[:first]),
	}
}

// openCodeInput reads the input lines of the box. The input is empty when
// every line is blank, or when the only text is the placeholder drawn in the
// placeholder color.
func openCodeInput(box []panelLine) (InputLine, bool) {
	contents := make([]string, len(box))
	first, last := -1, -1
	for i, p := range box {
		c, ok := panelContent(p)
		if !ok {
			return InputLine{}, false
		}
		contents[i] = c
		if c != "" {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 || (first == last && openCodeIsPlaceholder(contents[first], box[first])) {
		return InputLine{Empty: true}, true
	}
	// Blank lines between typed lines are part of the unsent text.
	return InputLine{Left: joinTexts(contents[first : last+1])}, true
}

// openCodePlaceholderText matches the whole placeholder line the samples
// show: a fixed prefix and one quoted suggestion.
var openCodePlaceholderText = regexp.MustCompile(`^Ask anything… "[^"]+"$`)

// openCodeIsPlaceholder reports whether an input line is the placeholder: its
// whole text has the placeholder form and every visible character is drawn in
// the placeholder color.
func openCodeIsPlaceholder(text string, p panelLine) bool {
	if !strings.HasPrefix(text, openCodePlaceholderPrefix) || !openCodePlaceholderText.MatchString(text) {
		return false
	}
	for _, c := range p.cells[len(openCodePad):] {
		if c.r != ' ' && c.fg != openCodePlaceholderColor {
			return false
		}
	}
	return true
}

// openCodeNewSession reports whether the lines above the input box hold no
// conversation: only blank lines, the logo, and the update notice.
func openCodeNewSession(above []screenLine) bool {
	logo := 0
	for _, l := range above {
		t := strings.TrimSpace(l.text)
		switch {
		case t == "":
		case logo < len(openCodeLogo) && t == openCodeLogo[logo]:
			logo++
		case logo == len(openCodeLogo) && strings.HasPrefix(t, openCodeUpdatePrefix):
		default:
			return false
		}
	}
	return logo == len(openCodeLogo)
}
