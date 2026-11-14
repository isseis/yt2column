package chief

import (
	"regexp"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// ScreenKind is what a pane screen shows. The zero value is ScreenUnmatched,
// the reading that assumes least: the screen matched no shape read from the
// samples, so nothing is sent to that pane and its workspace needs a check.
type ScreenKind int

const (
	// ScreenUnmatched is a screen of no known shape.
	ScreenUnmatched ScreenKind = iota
	// ScreenPrompt is an agent's input line with no question over it.
	ScreenPrompt
	// ScreenQuestion is a question widget waiting for an answer.
	ScreenQuestion
)

// QuestionKind is the shape of a question widget. The zero value is
// QuestionUnknown and is never relayed.
type QuestionKind int

const (
	// QuestionUnknown is a question of no known shape.
	QuestionUnknown QuestionKind = iota
	// QuestionSingle asks for one option of one question.
	QuestionSingle
	// QuestionMulti lets the human check several options.
	QuestionMulti
	// QuestionTabs holds several questions; only the current tab is drawn.
	QuestionTabs
	// QuestionPermission asks whether a tool call may run.
	QuestionPermission
)

// RelayBlock says why a question cannot be relayed. The zero value is
// RelayBlockUnknown, so a question is relayable only when a reader set
// RelayAllowed after matching a single-select shape.
type RelayBlock int

const (
	// RelayBlockUnknown blocks a relay for a reason no reader set.
	RelayBlockUnknown RelayBlock = iota
	// RelayAllowed marks a single-select question.
	RelayAllowed
	// RelayBlockMulti blocks a multi-select question.
	RelayBlockMulti
	// RelayBlockTabs blocks a question with several tabs.
	RelayBlockTabs
	// RelayBlockPermission blocks a permission prompt.
	RelayBlockPermission
)

// Option is one option of a question as the screen draws it. Relayable is
// false for the options that move to free text or to a chat, and for every
// option of a question that cannot be relayed.
type Option struct {
	Number      int
	Label       string
	Description string
	Relayable   bool
}

// Question is a question widget read from a screen. Text and the option texts
// are as read; they are sanitized only when they are shown.
type Question struct {
	Kind  QuestionKind
	Relay RelayBlock
	// Merge is true for /runpr's merge question (step 4 item 3). The deferred
	// question before it is an ordinary single-select question.
	Merge   bool
	Text    string
	Options []Option
	// Tabs names the tabs of a tabbed question, the current one included, in
	// screen order; the other tabs' questions are not on the screen.
	Tabs []string
}

// InputLine is the agent's input line. Empty is true only when the line was
// read and holds nothing but the agent's own placeholder; Left is the unsent
// text otherwise.
type InputLine struct {
	Empty bool
	Left  string
}

// Screen is what a pane screen shows, read by the shapes of the samples.
type Screen struct {
	Kind     ScreenKind
	Question *Question
	Input    InputLine
	// OpenCodeModel is the text between the first and the last " · " of an
	// OpenCode status line, compared whole with the routing table.
	OpenCodeModel string
	// NewSession is true when an OpenCode prompt screen holds no
	// conversation, the screen after /new.
	NewSession bool
}

// mergeQuestionText matches the question line of /runpr's merge question
// (runpr.md step 4 item 3). The deferred question of item 1 does not match.
var mergeQuestionText = regexp.MustCompile(`^PR #[1-9][0-9]* をどうしますか（第 [1-9][0-9]* 回。要約と詳細ファイルを確認のうえ）$`)

// mergeQuestionSqueezed matches the merge question with every blank and line
// break removed, so a wrapped one is found wherever the terminal broke it.
var mergeQuestionSqueezed = regexp.MustCompile(`^PR#[1-9][0-9]*をどうしますか（第[1-9][0-9]*回。要約と詳細ファイルを確認のうえ）$`)

// numberedOption matches an option line's number and the rest of the line.
var numberedOption = regexp.MustCompile(`^([1-9][0-9]*)\. (.+)$`)

// unmatched is the Screen every reader returns for a shape it does not know.
var unmatched = Screen{Kind: ScreenUnmatched}

// trimmedLines drops the trailing blank lines of a screen.
func trimmedLines(lines []screenLine) []screenLine {
	end := len(lines)
	for end > 0 && lines[end-1].text == "" {
		end--
	}
	return lines[:end]
}

// isRule reports whether text is a horizontal rule drawn with r, at least
// minRuleWidth characters long.
func isRule(text string, r rune) bool {
	n := 0
	for _, c := range text {
		if c != r {
			return false
		}
		n++
	}
	return n >= minRuleWidth
}

// minRuleWidth is the shortest line read as a rule; a few rule characters in
// a sentence are not one.
const minRuleWidth = 10

// joinTexts joins the texts of lines with newlines.
func joinTexts(texts []string) string {
	return strings.Join(texts, "\n")
}

// maxOptionNumber is the highest option number a question may have. The
// samples showed that one digit key selects its option at once, so an option
// numbered 10 or more could not be chosen by its number.
const maxOptionNumber = 9

// finishQuestion settles what the readers leave to one place: only a
// single-select question the reader allowed keeps its relayable options (the
// reader already marked the free-text and chat ones), and only such a question
// can be the merge question. It returns false for a question the samples did
// not show: one with an option numbered past maxOptionNumber, or a merge
// question whose text wraps across lines.
func finishQuestion(q *Question) bool {
	for _, o := range q.Options {
		if o.Number > maxOptionNumber {
			return false
		}
	}
	if mergeQuestionSqueezed.MatchString(strings.Join(strings.Fields(q.Text), "")) && !mergeQuestionText.MatchString(q.Text) {
		return false
	}
	if q.Kind == QuestionSingle && q.Relay == RelayAllowed {
		q.Merge = mergeQuestionText.MatchString(q.Text)
		return true
	}
	for i := range q.Options {
		q.Options[i].Relayable = false
	}
	return true
}

// readScreen reads the screen of an agent's pane with the reader of its
// agent. An agent chief does not know is never read: ScreenUnmatched.
func readScreen(agent paneinput.AgentKind, raw string) Screen {
	switch agent {
	case paneinput.AgentClaudeCode:
		return parseClaudeScreen(raw)
	case paneinput.AgentOpenCode:
		return parseOpenCodeScreen(raw)
	default:
		return unmatched
	}
}
