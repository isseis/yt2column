package publisher

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/writer"
)

const (
	// slackMaxMessageRunes bounds one message in Unicode code points.
	slackMaxMessageRunes = 16383
	// slackMaxMessages bounds how many messages one article may become.
	slackMaxMessages = 10
	// slackDisplayMaxRunes is the length of the longest position marker,
	// "(10/10)\n\n", which every fragment is sized to leave room for.
	slackDisplayMaxRunes = len("(10/10)\n\n")
)

// prepareSlackMessages runs every pre-send check on article and returns the
// text of each message Publish sends, in order. It touches no network, never
// mutates article, and returns the same result for the same article.
func prepareSlackMessages(article writer.Article) ([]string, error) {
	if err := article.CheckPublishable(); err != nil {
		return nil, err
	}
	for _, value := range []string{article.Title, article.Body, article.Model, article.ModelVersion} {
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("%w: a field is not valid UTF-8", writer.ErrInvalidArticle)
		}
	}

	posted := renderArticle(article)
	messages, err := splitSlackText(posted)
	if err != nil {
		return nil, err
	}
	// Checking posted alone is enough: the position marker adds none of the
	// characters M1-M4 look for, so a match in a message is also a match in
	// posted. TestSlackMentionMessagesFollowPosted pins this.
	if err := rejectMention(posted); err != nil {
		return nil, err
	}
	return messages, nil
}

// SlackMessageCount returns how many messages Publish sends for article. It
// performs the same preparation as Publish, touches no network, and returns
// the same pre-send errors.
func SlackMessageCount(article writer.Article) (int, error) {
	messages, err := prepareSlackMessages(article)
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}

// splitSlackText breaks posted into the message texts Publish sends. A text
// that fits in one message is returned as it is, with no position marker. A
// longer text is cut where possible after a newline, and the rest of the text
// stays whole once it fits in one fragment; every fragment is at most
// slackMaxMessageRunes minus slackDisplayMaxRunes code points, so a marker
// never pushes a message past the limit. It returns ErrSlackUnsplittable when
// the fragments would number more than slackMaxMessages or one would hold only
// whitespace.
func splitSlackText(posted string) ([]string, error) {
	runes := []rune(posted)
	if len(runes) <= slackMaxMessageRunes {
		return []string{posted}, nil
	}

	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	var fragments []string
	for start := 0; start < len(runes); {
		end := min(start+capacity, len(runes))
		cut := end
		if end < len(runes) {
			if newline, ok := lastNewlineCut(runes, start, end); ok {
				cut = newline
			}
		}
		fragment := runes[start:cut]
		if isWhitespaceOnly(fragment) {
			return nil, ErrSlackUnsplittable
		}
		fragments = append(fragments, string(fragment))
		start = cut
		if len(fragments) > slackMaxMessages {
			return nil, ErrSlackUnsplittable
		}
	}

	messages := make([]string, len(fragments))
	for i, fragment := range fragments {
		messages[i] = fmt.Sprintf("(%d/%d)\n\n%s", i+1, len(fragments), fragment)
	}
	return messages, nil
}

// lastNewlineCut returns the latest cut in (start, end] that follows a newline
// and leaves a front fragment holding more than whitespace, or false when no
// such cut exists.
func lastNewlineCut(runes []rune, start, end int) (int, bool) {
	// A front fragment holds more than whitespace exactly when it reaches
	// past the leading whitespace of the range.
	first := start
	for first < end && unicode.IsSpace(runes[first]) {
		first++
	}
	for cut := end; cut > first; cut-- {
		if runes[cut-1] == '\n' {
			return cut, true
		}
	}
	return 0, false
}

func isWhitespaceOnly(runes []rune) bool {
	if len(runes) == 0 {
		return false
	}
	for _, r := range runes {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// rejectMention returns an error wrapping ErrSlackMention when text contains a
// sequence that Mattermost or Slack may treat as a mention (M1, M2) or rewrite
// (M3, M4). The error names the rule and the position only, never the text.
func rejectMention(text string) error {
	match, ok := findMention(text)
	if !ok {
		return nil
	}
	line, column := lineAndColumn([]rune(text), match.index)
	return fmt.Errorf("%w: rule %s at line %d, column %d", ErrSlackMention, match.rule, line, column)
}

type mentionMatch struct {
	rule  string
	index int
}

// findMention reports the earliest rune index of text that matches M1-M4.
// M1 and M2 are matched against the inspection string built by
// inspectMentionText; M3 and M4 against text itself.
func findMention(text string) (mentionMatch, bool) {
	raw := []rune(text)
	best := mentionMatch{}
	found := false
	consider := func(rule string, index int) {
		if !found || index < best.index {
			best = mentionMatch{rule: rule, index: index}
			found = true
		}
	}

	inspected, source := inspectMentionText(text)
	if index, rule, ok := findInspectionMention([]rune(inspected), source); ok {
		consider(rule, index)
	}
	if index, ok := findEntityMention(raw); ok {
		consider(ruleM3, index)
	}
	if index, ok := findLinkMention(raw); ok {
		consider(ruleM4, index)
	}

	return best, found
}

// findInspectionMention returns the first M1 or M2 match in the inspection
// string, reported at its index in the original text.
func findInspectionMention(ins []rune, source []int) (int, string, bool) {
	for i := 0; i+1 < len(ins); i++ {
		if ins[i] == '<' && (ins[i+1] == '!' || ins[i+1] == '@') {
			return source[i], ruleM1, true
		}
		if ins[i] == '@' && isMentionNameRune(ins[i+1]) {
			return source[i], ruleM2, true
		}
	}
	return 0, "", false
}

// findEntityMention returns the first M3 (character reference) match.
func findEntityMention(raw []rune) (int, bool) {
	for i := range raw {
		if raw[i] != '&' {
			continue
		}
		if i+1 < len(raw) && raw[i+1] == '#' {
			return i, true
		}
		j := i + 1
		for j < len(raw) && isASCIILetter(raw[j]) {
			j++
		}
		if j > i+1 && j < len(raw) && raw[j] == ';' {
			return i, true
		}
	}
	return 0, false
}

// findLinkMention returns the first M4 (link-like sequence) match.
func findLinkMention(raw []rune) (int, bool) {
	for i := range raw {
		if raw[i] != '<' {
			continue
		}
		j := i + 1
		for j < len(raw) && !isM4FirstTerminator(raw[j]) {
			j++
		}
		if j == i+1 || j >= len(raw) || raw[j] != '|' {
			continue
		}
		k := j + 1
		for k < len(raw) && raw[k] != '\n' && raw[k] != '>' {
			k++
		}
		if k > j+1 && k < len(raw) && raw[k] == '>' {
			return i, true
		}
	}
	return 0, false
}

const (
	ruleM1 = "M1"
	ruleM2 = "M2"
	ruleM3 = "M3"
	ruleM4 = "M4"
)

// inspectMentionText returns the text M1 and M2 are matched against, together
// with a map from each result rune to its index in text. It applies, in order,
// the transformations V2 (drop Unicode Cf format characters), V3 (replace
// look-alike characters with ASCII), and V1 (undo CommonMark backslash
// escapes). The order matters: a sequence becomes an escape only after V2 and
// V3, and V1 must be last to recover it.
func inspectMentionText(text string) (string, []int) {
	runes := []rune(text)
	stripped := make([]rune, 0, len(runes))
	source := make([]int, 0, len(runes))
	for i, r := range runes {
		if unicode.Is(unicode.Cf, r) {
			continue
		}
		stripped = append(stripped, replaceLookAlike(r))
		source = append(source, i)
	}

	unescaped := make([]rune, 0, len(stripped))
	unescapedSource := make([]int, 0, len(stripped))
	for i := 0; i < len(stripped); {
		if stripped[i] == '\\' && i+1 < len(stripped) && isASCIIPunctuation(stripped[i+1]) {
			unescaped = append(unescaped, stripped[i+1])
			unescapedSource = append(unescapedSource, source[i])
			i += 2
			continue
		}
		unescaped = append(unescaped, stripped[i])
		unescapedSource = append(unescapedSource, source[i])
		i++
	}
	return string(unescaped), unescapedSource
}

func replaceLookAlike(r rune) rune {
	switch r {
	case '\uFF20', '\uFE6B':
		return '@'
	case '\uFF1C', '\uFE64':
		return '<'
	case '\uFF01', '\uFE57':
		return '!'
	default:
		return r
	}
}

func isMentionNameRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

func isASCIILetter(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

func isASCIIPunctuation(r rune) bool {
	return strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", r)
}

func isM4FirstTerminator(r rune) bool {
	return r == '\n' || r == '<' || r == '|' || r == '>'
}

func lineAndColumn(runes []rune, index int) (line, column int) {
	line, column = 1, 1
	for i := 0; i < index && i < len(runes); i++ {
		if runes[i] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}
