package chief

import (
	"bytes"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/strictjson"
)

// SessionReportKind is what a Claude Code transcript says about the pane's
// last report. The zero value is SessionReportUnreadable: the transcript does
// not have the accepted shape, so whether there is a report cannot be told.
type SessionReportKind int

const (
	// SessionReportUnreadable is a transcript outside the accepted shape.
	SessionReportUnreadable SessionReportKind = iota
	// SessionReportAbsent is a transcript with no last report.
	SessionReportAbsent
	// SessionReportFound is a transcript whose last finished response has a
	// body.
	SessionReportFound
)

// SessionReport is the last report of a Claude Code session, the PRs its
// pr-link lines name, and the last line the human typed. Text is set only
// for SessionReportFound; nothing but Kind is set for
// SessionReportUnreadable.
type SessionReport struct {
	Kind      SessionReportKind
	Text      string
	LastHuman HumanInput
	links     []prLink
	// absence says why a readable transcript has no last report.
	absence string
}

// prLink is the PR a pr-link line names.
type prLink struct {
	number int64
	url    string
}

// Names reports whether a pr-link line of the session names the PR with
// this number and URL, both compared whole.
func (r SessionReport) Names(number int, url string) bool {
	return slices.Contains(r.links, prLink{number: int64(number), url: url})
}

// The values of a transcript line this reader decides by, as the samples of
// Claude Code 2.1.325 and 2.1.326 show them.
const (
	// assistantLine is the type of a line of a response.
	assistantLine = "assistant"
	// prLinkLine is the type of the line Claude Code writes for a PR the
	// session made or worked on.
	prLinkLine = "pr-link"
	// syntheticModel is the model of a response Claude Code writes itself,
	// such as an API error notice.
	syntheticModel = "<synthetic>"
	// finishedTurn is the stop reason of a response that ended its turn.
	finishedTurn = "end_turn"
	// textPart is the content type of a response's text.
	textPart = "text"
)

// sessionResponse is what the lines of one response say: whether every line
// ended the turn, and the text of their text parts in written order.
type sessionResponse struct {
	finished bool
	texts    []string
}

// readSessionReport reads the last report, the pr-link lines, and the last
// human input of a transcript, one JSON object per line. Contract: a line
// that is not one JSON object with a non-empty type, a line the human input
// reader cannot tell, and a response or pr-link line missing a member this
// reader uses or holding it with another type make the whole transcript
// SessionReportUnreadable, so a response is never skipped to make an older
// one the last; an unknown line type, an unknown member, an unknown content
// part, and an unknown stop reason are read past. The last response is the
// one with the last line among those neither a subagent's nor synthetic; it
// is the report only when every line of it ended its turn, no human line
// follows it, and its text parts, joined with a blank line and trimmed, are
// not empty.
func readSessionReport(transcript []byte) SessionReport {
	lastHuman := HumanInput{Kind: HumanInputNone}
	var links []prLink
	responses := map[string]*sessionResponse{}
	lastID := ""
	humanAfter := false
	for line := range bytes.SplitSeq(transcript, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		object, err := strictjson.ParseObject(line)
		if err != nil {
			return SessionReport{}
		}
		members, err := object.Collect("type")
		if err != nil {
			return SessionReport{}
		}
		kind, err := strictjson.RequiredString(members, "type")
		if err != nil {
			return SessionReport{}
		}
		in, human, ok := humanInputOf(object)
		if !ok {
			return SessionReport{}
		}
		if human {
			lastHuman, humanAfter = in, true
		}
		switch kind {
		case assistantLine:
			r, ok := readResponseLine(object)
			if !ok {
				return SessionReport{}
			}
			if r.skipped {
				continue
			}
			kept, seen := responses[r.id]
			if !seen {
				kept = &sessionResponse{finished: true}
				responses[r.id] = kept
			}
			kept.finished = kept.finished && r.finished
			kept.texts = append(kept.texts, r.texts...)
			lastID, humanAfter = r.id, false
		case prLinkLine:
			link, ok := readPRLink(object)
			if !ok {
				return SessionReport{}
			}
			links = append(links, link)
		}
	}
	report := SessionReport{Kind: SessionReportAbsent, LastHuman: lastHuman, links: links}
	last, ok := responses[lastID]
	switch {
	case !ok:
		report.absence = "no response"
	case !last.finished:
		report.absence = "the last response did not end its turn"
	case humanAfter:
		report.absence = "the human typed after the last response"
	default:
		text := strings.TrimSpace(strings.Join(last.texts, "\n\n"))
		if text == "" {
			report.absence = "the last response has no text"
			break
		}
		report.Kind, report.Text = SessionReportFound, text
	}
	return report
}

// responseLine is what one response line says.
type responseLine struct {
	id       string
	skipped  bool
	finished bool
	texts    []string
}

// readResponseLine reads a response line: its message ID, whether it is a
// subagent's or synthetic (skipped), whether it ended its turn, and its text
// parts. ok is false when a member it uses is missing or of another type.
func readResponseLine(object strictjson.Object) (responseLine, bool) {
	members, err := object.Collect("isSidechain", "message")
	if err != nil {
		return responseLine{}, false
	}
	sidechainValue, err := strictjson.Required(members, "isSidechain")
	if err != nil {
		return responseLine{}, false
	}
	sidechain, err := sidechainValue.AsBool()
	if err != nil {
		return responseLine{}, false
	}
	messageValue, err := strictjson.Required(members, "message")
	if err != nil {
		return responseLine{}, false
	}
	message, err := messageValue.AsObject()
	if err != nil {
		return responseLine{}, false
	}
	fields, err := message.Collect("id", "model", "stop_reason", "content")
	if err != nil {
		return responseLine{}, false
	}
	id, err := strictjson.RequiredString(fields, "id")
	if err != nil {
		return responseLine{}, false
	}
	model, err := strictjson.RequiredStringAllowEmpty(fields, "model")
	if err != nil {
		return responseLine{}, false
	}
	stopValue, err := strictjson.Required(fields, "stop_reason")
	if err != nil {
		return responseLine{}, false
	}
	stop := ""
	if !stopValue.IsNull() {
		if stop, err = stopValue.AsString(); err != nil {
			return responseLine{}, false
		}
	}
	contentValue, err := strictjson.Required(fields, "content")
	if err != nil {
		return responseLine{}, false
	}
	parts, err := contentValue.AsArray()
	if err != nil {
		return responseLine{}, false
	}
	line := responseLine{id: id, skipped: sidechain || model == syntheticModel, finished: stop == finishedTurn}
	for _, p := range parts {
		text, isText, ok := readContentPart(p)
		if !ok {
			return responseLine{}, false
		}
		if isText {
			line.texts = append(line.texts, text)
		}
	}
	return line, true
}

// readContentPart reads one part of a response's content: isText is true
// for a text part, whose text is returned.
func readContentPart(v strictjson.Value) (text string, isText, ok bool) {
	part, err := v.AsObject()
	if err != nil {
		return "", false, false
	}
	fields, err := part.Collect("type", "text")
	if err != nil {
		return "", false, false
	}
	kind, err := strictjson.RequiredStringAllowEmpty(fields, "type")
	if err != nil {
		return "", false, false
	}
	if kind != textPart {
		return "", false, true
	}
	text, err = strictjson.RequiredStringAllowEmpty(fields, "text")
	if err != nil {
		return "", false, false
	}
	return text, true, true
}

// readPRLink reads a pr-link line: a positive PR number, a non-empty URL,
// and a repository that is a string.
func readPRLink(object strictjson.Object) (prLink, bool) {
	members, err := object.Collect("prNumber", "prUrl", "prRepository")
	if err != nil {
		return prLink{}, false
	}
	numberValue, err := strictjson.Required(members, "prNumber")
	if err != nil {
		return prLink{}, false
	}
	number, err := numberValue.AsNonNegativeInt64()
	if err != nil || number == 0 {
		return prLink{}, false
	}
	url, err := strictjson.RequiredString(members, "prUrl")
	if err != nil {
		return prLink{}, false
	}
	if _, err := strictjson.RequiredStringAllowEmpty(members, "prRepository"); err != nil {
		return prLink{}, false
	}
	return prLink{number: number, url: url}, true
}

// nextNaming matches a report line, its surrounding spaces removed, that
// names NEXT: after leading Markdown marks (list bullets and numbers, quote
// marks, emphasis and code marks), the word NEXT and a colon, ASCII or
// full-width, with emphasis or code marks allowed between them. A line that
// names NEXT outside the grammar of nextLine is a NEXT that cannot be read;
// a line that only mentions NEXT in a sentence does not name it.
var nextNaming = regexp.MustCompile("^(?:[-*+>_`\\s]|[0-9]+\\.)*NEXT[*_`]*[:\\x{FF1A}]")

// readReportNext reads the NEXT and the PR numbers of a Claude Code pane's
// last report, read from its transcript. Contract: a line in the grammar of
// nextLine is read as readNextValue reads it, any other line naming NEXT is
// NextUnmatched, and lines that disagree make the result NextUnmatched; with
// no line naming NEXT it is NextAbsent. PR numbers are read from each line
// alone: a transcript's report has no terminal wrap to join again. The
// result is only compared with what the tool decided; it never decides what
// is sent.
func readReportNext(text string) RunprReport {
	report := RunprReport{Next: RunprNext{Kind: NextAbsent}}
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimSpace(raw)
		report.PRs = appendPRReferences(report.PRs, line)
		var next RunprNext
		switch m := nextLine.FindStringSubmatch(line); {
		case m != nil:
			next = readNextValue(m[1])
		case nextNaming.MatchString(line):
			next = RunprNext{}
		default:
			continue
		}
		if report.Next.Kind != NextAbsent && next != report.Next {
			next = RunprNext{}
		}
		report.Next = next
	}
	return report
}
