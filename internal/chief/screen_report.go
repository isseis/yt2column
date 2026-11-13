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
