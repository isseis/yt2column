package publisher

import (
	"fmt"
	"slices"
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
	// CheckPublishable checks the UTF-8 of every other value the post holds;
	// the body is the one value it does not check.
	if !utf8.ValidString(article.Body) {
		return nil, fmt.Errorf("%w: the body is not valid UTF-8", writer.ErrInvalidArticle)
	}

	posted := renderArticle(article)
	messages, err := splitSlackText(slackPost{text: posted, keepTogetherFrom: utf8.RuneCountInString(posted)})
	if err != nil {
		return nil, err
	}
	// Checking posted alone is enough for M1, M3 and M4: the position marker
	// adds none of the characters they look for, so a match in a message is
	// also a match in posted. M2 also looks at the rune before an '@', which a
	// cut changes at the start of a fragment; splitSlackText never starts a
	// fragment with an '@' that M2 accepts in posted and reports in the
	// message, so M2 holds for the messages too.
	// TestSlackMentionMessagesFollowPosted pins this for M1, M3 and M4;
	// TestSlackSplitAvoidsMidWordAt, TestSlackSplitRefusesUnmovableCut and
	// TestCheckFragmentStarts pin it for M2.
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

// splitSlackText breaks post into the message texts Publish sends. A text
// that fits in one message is returned as it is, with no position marker. A
// longer text is cut into the fragments cutSlackFragments chooses, each
// prefixed with its position marker. It returns ErrSlackUnsplittable when
// cutSlackFragments does, or when checkFragmentStarts finds a fragment whose
// message would start with a mention the post does not hold.
func splitSlackText(post slackPost) ([]string, error) {
	fragments, err := cutSlackFragments(post)
	if err != nil {
		return nil, err
	}
	if len(fragments) == 1 {
		return fragments, nil
	}
	if err := checkFragmentStarts(fragments); err != nil {
		return nil, err
	}

	messages := make([]string, len(fragments))
	for i, fragment := range fragments {
		messages[i] = fmt.Sprintf("(%d/%d)\n\n%s", i+1, len(fragments), fragment)
	}
	return messages, nil
}

// cutSlackFragments returns post.text whole when it fits in one message, and
// otherwise the fragments it is cut into, which joined give post.text back.
// Every fragment of a cut text is non-empty and at most slackMaxMessageRunes
// minus slackDisplayMaxRunes code points, so a marker never pushes a message
// past the limit, and every fragment but the last ends at or before
// post.keepTogetherFrom. A fragment is cut where possible after a newline,
// the rest of the text stays whole once it fits in one fragment, and a cut
// that would start the next message with a mid-word '@' moves one rune back
// (cutStartsMention). It returns ErrSlackUnsplittable when the region from
// post.keepTogetherFrom does not fit in one fragment, when the fragments would
// number more than slackMaxMessages, or when one would hold only whitespace.
func cutSlackFragments(post slackPost) ([]string, error) {
	runes := []rune(post.text)
	if len(runes) <= slackMaxMessageRunes {
		return []string{post.text}, nil
	}

	capacity := slackMaxMessageRunes - slackDisplayMaxRunes
	var fragments []string
	for start := 0; start < len(runes); {
		cut := len(runes)
		if len(runes)-start > capacity {
			if start >= post.keepTogetherFrom {
				return nil, ErrSlackUnsplittable
			}
			end := min(start+capacity, post.keepTogetherFrom)
			cut = end
			if newline, ok := lastNewlineCut(runes, start, end); ok {
				cut = newline
			}
			// Moving back one rune is enough: the moved cut is followed by
			// a letter or a digit, not an '@'. A move that would empty the
			// fragment is left undone for checkFragmentStarts to refuse.
			if cutStartsMention(runes, cut) && cut-1 > start {
				cut--
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
	return fragments, nil
}

// checkFragmentStarts returns ErrSlackUnsplittable when a fragment after the
// first starts with an '@' that a cut turned into a mention: mentionAts
// reports it in the fragment's message, where a position marker and a blank
// line precede it, but not in the joined text, where a letter or a digit
// does. cutSlackFragments avoids such cuts; this refuses the ones it could
// not avoid. It relies on the position marker ending in a newline, so that
// the rune before a fragment in its message is never a letter or a digit.
func checkFragmentStarts(fragments []string) error {
	for i := 1; i < len(fragments); i++ {
		last, _ := utf8.DecodeLastRuneInString(fragments[i-1])
		if cutStartsMention(append([]rune{last}, []rune(fragments[i])...), 1) {
			return ErrSlackUnsplittable
		}
	}
	return nil
}

// cutStartsMention reports whether cutting runes before index cut makes
// mentionAts report the '@' at cut in the next message, where it is preceded
// by a newline, when it does not report it in runes.
func cutStartsMention(runes []rune, cut int) bool {
	if cut <= 0 || cut+1 >= len(runes) {
		return false
	}
	inMessage := slices.Contains(mentionAtsOf(runes[cut:cut+2]), 0)
	inText := slices.Contains(mentionAtsOf(runes[cut-1:cut+2]), 1)
	return inMessage && !inText
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
// M1 is matched against the inspection string built by inspectMentionText,
// M2 against the V3-replaced text (mentionAtsOf), and M3 and M4 against text
// itself.
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
	if index, ok := findInspectionMention([]rune(inspected), source); ok {
		consider(ruleM1, index)
	}
	if ats := mentionAtsOf(raw); len(ats) > 0 {
		consider(ruleM2, ats[0])
	}
	if index, ok := findEntityMention(raw); ok {
		consider(ruleM3, index)
	}
	if index, ok := findLinkMention(raw); ok {
		consider(ruleM4, index)
	}

	return best, found
}

// findInspectionMention returns the first M1 match in the inspection string,
// reported at its index in the original text.
func findInspectionMention(ins []rune, source []int) (int, bool) {
	for i := 0; i+1 < len(ins); i++ {
		if ins[i] == '<' && (ins[i+1] == '!' || ins[i+1] == '@') {
			return source[i], true
		}
	}
	return 0, false
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
	// nextStop[i] is the first index at or after i whose rune ends the second
	// part of M4 (a newline or '>'), or len(raw) if there is none. Building it
	// once keeps the scan linear when many '<' are each followed by '|'.
	nextStop := make([]int, len(raw)+1)
	nextStop[len(raw)] = len(raw)
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] == '\n' || raw[i] == '>' {
			nextStop[i] = i
			continue
		}
		nextStop[i] = nextStop[i+1]
	}

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
		stop := nextStop[j+1]
		if stop > j+1 && stop < len(raw) && raw[stop] == '>' {
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

// inspectMentionText returns the text M1 is matched against, together
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

// mentionAts returns the rune indices of the '@' characters that may start a
// mention: a name character follows the '@', and the rune before it is not a
// letter or a digit. text is the V3-replaced form of the checked string (look-
// alikes replaced one for one, nothing dropped or unescaped), so an index is
// also an index of the original string. Format characters and backslashes
// are word separators here, not stripped, so "a", U+200B, "@here" reports the
// '@' and "@", U+200B, "name" does not. The rule is wider than Mattermost's
// word splitting on purpose: it looks only at the rune before the '@', so it
// also reports "foo_@bar" and "x@@y", and a '@' after emphasis markup.
func mentionAts(text []rune) []int {
	var ats []int
	for i := 0; i+1 < len(text); i++ {
		if text[i] != '@' || !isMentionNameChar(text[i+1]) {
			continue
		}
		if i > 0 && (unicode.IsLetter(text[i-1]) || unicode.IsNumber(text[i-1])) {
			continue
		}
		ats = append(ats, i)
	}
	return ats
}

// mentionAtsOf applies the V3 replacement to runes and returns mentionAts of
// the result. Because V3 replaces one rune with one rune, an index is also an
// index of runes.
func mentionAtsOf(runes []rune) []int {
	replaced := make([]rune, len(runes))
	for i, r := range runes {
		replaced[i] = replaceLookAlike(r)
	}
	return mentionAts(replaced)
}

// isMentionNameChar reports whether r can follow '@' in a mention. '.' and '-'
// are included although Mattermost user names cannot start with them, to err
// on the side of reporting.
func isMentionNameChar(r rune) bool {
	return r == '_' || r == '.' || r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r)
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
