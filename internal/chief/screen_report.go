package chief

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// NextKind is what /runpr's report says about the next command. The zero
// value is NextUnmatched: a NEXT line was found but could not be read, which
// the caller treats as a report that differs from the tool's own next command.
type NextKind int

const (
	// NextUnmatched is a NEXT line that could not be read.
	NextUnmatched NextKind = iota
	// NextAbsent means the screen holds no NEXT line.
	NextAbsent
	// NextNone means the report says there is no next command.
	NextNone
	// NextCommand means the report names a command and a task.
	NextCommand
)

// NextCommandKind is a command a /runpr report can name as NEXT. The zero
// value is never a command.
type NextCommandKind int

const (
	// NextCommandUnknown is no command.
	NextCommandUnknown NextCommandKind = iota
	// NextCommandMkarch is /mkarch.
	NextCommandMkarch
	// NextCommandMkplan is /mkplan.
	NextCommandMkplan
	// NextCommandRunplan is /runplan.
	NextCommandRunplan
)

// RunprNext is the NEXT of a /runpr report. It is used only to compare with
// the next command the tool decides from GitHub and git; it never decides
// what is sent.
type RunprNext struct {
	Kind    NextKind
	Command NextCommandKind
	Task    paneinput.TaskID
}

var (
	// nextLine matches a report line that gives NEXT, with an optional list
	// bullet before it. The value may be missing, which is still a NEXT line,
	// but one that cannot be read.
	nextLine = regexp.MustCompile(`^(?:- )?NEXT:(?: (.*))?$`)
	// nextCommand matches a named command and a four-digit task at the start
	// of a NEXT value, optionally in backquotes.
	nextCommand = regexp.MustCompile("^(`?)/(mkarch|mkplan|runplan) ([0-9]{4})(`?)(?:（|$)")
	// nextNoneValue matches the value for no next command, alone or followed
	// by a reason in full-width parentheses.
	nextNoneValue = regexp.MustCompile(`^なし(?:（|$)`)
)

// readRunprNext reads NEXT from the text of a screen, its SGR sequences
// already removed (the text of the lines parseANSIScreen returns). It reads every line of
// the screen, earlier reports and echoed prompts included, so an older report
// that disagrees makes the result unmatched. Contract: every NEXT line on the
// screen must agree; a screen with lines that disagree, or with a
// value that is neither "none" nor a known command and task, is
// NextUnmatched, so a doubtful report never matches the tool's command.
func readRunprNext(text string) RunprNext {
	result := RunprNext{Kind: NextAbsent}
	for raw := range strings.SplitSeq(text, "\n") {
		m := nextLine.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			continue
		}
		next := readNextValue(m[1])
		if result.Kind != NextAbsent && next != result {
			return RunprNext{}
		}
		result = next
	}
	return result
}

// readNextValue reads the value after "NEXT: ". A line without a value cannot
// be read, so it is unmatched like any other value that is neither "none" nor
// a known command and task.
func readNextValue(value string) RunprNext {
	if value == "" {
		return RunprNext{}
	}
	if nextNoneValue.MatchString(value) {
		return RunprNext{Kind: NextNone}
	}
	m := nextCommand.FindStringSubmatch(value)
	if m == nil || m[1] != m[4] {
		return RunprNext{}
	}
	task, err := paneinput.ParseTaskID(m[3])
	if err != nil {
		return RunprNext{}
	}
	next := RunprNext{Kind: NextCommand, Task: task}
	switch m[2] {
	case nameMkarch:
		next.Command = NextCommandMkarch
	case nameMkplan:
		next.Command = NextCommandMkplan
	case nameRunplan:
		next.Command = NextCommandRunplan
	default:
		return RunprNext{}
	}
	return next
}

// squeeze joins texts with each one's blanks at both ends removed, so a URL
// or a PR number the terminal wrapped across lines is whole again.
func squeeze(texts []string) string {
	var joined strings.Builder
	for _, t := range texts {
		joined.WriteString(strings.TrimSpace(t))
	}
	return joined.String()
}

// Fixed parts of an OpenCode v2.2.6 screen around the last response, as the
// /runpr report sample shows them: the response's lines are indented, a
// thought line may come before them, and a line naming the mode, the model,
// the time, and the rate ends them, above the input panel.
const (
	openCodeMessageIndent = "     "
	openCodeThoughtMark   = "+ Thought · "
)

// openCodeResponseEnd matches the line that ends an OpenCode response: the
// mode, the model, the time, and the rate in tokens per second, separated by
// the middle dots OpenCode draws, as the /runpr report sample shows it.
var openCodeResponseEnd = regexp.MustCompile(`^\S+ · .+ · .+ · [0-9]+(?:\.[0-9]+)? tok/s$`)

// openCodeLastMessage returns the lines of the last response of an OpenCode
// screen in the sampled shape: the input panel at the bottom, the response's
// end line right above it, and the indented response lines above that up to
// the panel before them, a leading thought line left out. Each line has its
// indent removed.
func openCodeLastMessage(raw string) ([]string, bool) {
	all, err := parseANSIScreen(raw)
	if err != nil {
		return nil, false
	}
	lines := trimmedLines(all)
	if openCodePrompt(lines).Kind != ScreenPrompt {
		return nil, false
	}
	// The input panel and its footer: the panel lines at the bottom, then the
	// lines below the last panel line.
	end := len(lines)
	for end > 0 {
		if _, ok := asPanel(lines[end-1]); ok {
			break
		}
		end--
	}
	for end > 0 {
		if _, ok := asPanel(lines[end-1]); !ok {
			break
		}
		end--
	}
	for end > 0 && lines[end-1].text == "" {
		end--
	}
	if end == 0 || !openCodeResponseEnd.MatchString(strings.TrimSpace(lines[end-1].text)) {
		return nil, false
	}
	start := end - 1
	for start > 0 {
		if _, ok := asPanel(lines[start-1]); ok {
			break
		}
		start--
	}
	var texts []string
	for _, l := range lines[start : end-1] {
		switch {
		case l.text == "":
			if len(texts) > 0 {
				texts = append(texts, "")
			}
		case !strings.HasPrefix(l.text, openCodeMessageIndent):
			return nil, false
		case len(texts) == 0 && strings.HasPrefix(strings.TrimPrefix(l.text, openCodeMessageIndent), openCodeThoughtMark):
			// The thought line before the response.
		default:
			texts = append(texts, strings.TrimPrefix(l.text, openCodeMessageIndent))
		}
	}
	if len(texts) == 0 {
		return nil, false
	}
	return texts, true
}

// RunprReport is the last report on a pane: its NEXT and the PR numbers it
// names. It is used only to compare with what the tool decided from GitHub
// and git; it never decides what is sent.
type RunprReport struct {
	Next RunprNext
	// PRs are the numbers the report names as "#N" or as a PR URL's
	// "/pull/N". From a screen they are read from each line and from the
	// lines joined again, since a line break may be the terminal's wrap or
	// the report's own; from a transcript, whose lines are never wrapped,
	// from each line alone.
	PRs []int
}

// prReference matches a PR number written as "#N" or "/pull/N"; the digits
// are taken whole, so "#42" is never read from "#421".
var prReference = regexp.MustCompile(`(?:#|/pull/)([1-9][0-9]*)`)

// readRunprReport reads the last report on an OpenCode pane's screen, by the
// sampled shape of OpenCode's last response; a Claude Code pane's report is
// read from its transcript instead (readClaudeReport). Contract: a screen of
// any other shape, or of any agent but OpenCode, gives false; the report's
// NEXT is read as readRunprNext reads it, from the report alone.
func readRunprReport(agent paneinput.AgentKind, raw string) (RunprReport, bool) {
	if agent != paneinput.AgentOpenCode {
		return RunprReport{}, false
	}
	texts, ok := openCodeLastMessage(raw)
	if !ok {
		return RunprReport{}, false
	}
	report := RunprReport{Next: readRunprNext(joinTexts(texts))}
	for _, text := range append(slices.Clone(texts), squeeze(texts)) {
		report.PRs = appendPRReferences(report.PRs, text)
	}
	return report, true
}

// appendPRReferences appends to prs each PR number text names by
// prReference that prs does not hold yet.
func appendPRReferences(prs []int, text string) []int {
	for _, m := range prReference.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil && !slices.Contains(prs, n) {
			prs = append(prs, n)
		}
	}
	return prs
}

// readScreenNext reads every NEXT line of a whole pane screen, its SGR
// sequences removed, as readRunprNext does: earlier reports and echoed
// prompts included. It gives false when the screen cannot be parsed.
func readScreenNext(raw string) (RunprNext, bool) {
	all, err := parseANSIScreen(raw)
	if err != nil {
		return RunprNext{}, false
	}
	texts := make([]string, len(all))
	for i, l := range all {
		texts[i] = l.text
	}
	return readRunprNext(joinTexts(texts)), true
}
