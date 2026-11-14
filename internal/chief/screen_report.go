package chief

import (
	"regexp"
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
	case "mkarch":
		next.Command = NextCommandMkarch
	case "mkplan":
		next.Command = NextCommandMkplan
	case "runplan":
		next.Command = NextCommandRunplan
	default:
		return RunprNext{}
	}
	return next
}

// Fixed marks of a Claude Code 2.1.322 screen around a command's final
// report, as the sample shows them.
const (
	claudeMessageMark = "⏺ "
	claudeDoneMark    = "✻ "
	claudeIndent      = "  "
)

// readCommandReport returns the final report of the command that ran in a
// pane when it names the PR at url, read from the pane's screen. Contract: it
// reads only the sampled Claude Code shape (the last message, its lines
// indented under its mark, then the run's done line and the input box) and
// returns false for any other screen; it never reads an OpenCode screen,
// since no OpenCode report of a command that created a PR was sampled. The
// report is used to show, to summarize, and to narrow a reset, never to
// decide a send. A URL the terminal wrapped across lines is still found.
func readCommandReport(agent paneinput.AgentKind, raw, url string) (string, bool) {
	if agent != paneinput.AgentClaudeCode || url == "" {
		return "", false
	}
	all, err := parseANSIScreen(raw)
	if err != nil {
		return "", false
	}
	lines := trimmedLines(all)
	if claudePrompt(lines).Kind != ScreenPrompt {
		return "", false
	}
	top := lastRuleBefore(lines, lastRuleBefore(lines, len(lines)))
	done := -1
	for i := top - 1; i >= 0 && done < 0; i-- {
		switch text := lines[i].text; {
		case strings.HasPrefix(text, claudeDoneMark):
			done = i
		case text != "" && !strings.HasPrefix(text, claudeIndent):
			return "", false
		}
	}
	start := -1
	for i := done - 1; i >= 0 && start < 0; i-- {
		switch text := lines[i].text; {
		case strings.HasPrefix(text, claudeMessageMark):
			start = i
		case text != "" && !strings.HasPrefix(text, claudeIndent):
			return "", false
		}
	}
	if done < 0 || start < 0 {
		return "", false
	}
	texts := []string{strings.TrimPrefix(lines[start].text, claudeMessageMark)}
	for _, l := range lines[start+1 : done] {
		texts = append(texts, strings.TrimPrefix(l.text, claudeIndent))
	}
	report := strings.TrimSpace(joinTexts(texts))
	var joined strings.Builder
	for _, t := range texts {
		joined.WriteString(strings.TrimSpace(t))
	}
	if !namesURL(joined.String(), url) {
		return "", false
	}
	return report, true
}

// namesURL reports whether text holds url not followed by a digit, so the
// URL of PR 4 is not found in the URL of PR 42.
func namesURL(text, url string) bool {
	for rest := text; ; {
		i := strings.Index(rest, url)
		if i < 0 {
			return false
		}
		rest = rest[i+len(url):]
		if rest == "" || rest[0] < '0' || rest[0] > '9' {
			return true
		}
	}
}
