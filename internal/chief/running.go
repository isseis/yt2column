package chief

import (
	"fmt"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// RunningCommand is a command a running row's agents run: one of the five
// task commands, with its argument only when the argument is a number of the
// form chief accepts for the kind. Contract: a value shown is one the
// constructor made, or one Text and Kind re-validate; a kind outside the five
// shows nothing and an argument that does not validate is dropped, so a
// literal built inside the package with unvalidated members never reaches
// the output as typed.
type RunningCommand struct {
	kind paneinput.CommandKind
	arg  string
}

// errRunningCommand reports a kind outside the five task commands.
var errRunningCommand = fmt.Errorf("%w: not a task command kind", paneinput.ErrInvalid)

// newRunningCommand returns the command of kind with arg, the argument as the
// pane showed it. An empty arg is a command with no argument; an arg that is
// not of the form the kind accepts (a control character, a text that is not a
// number, a second argument) is dropped and reported by dropped, so a caller
// that compares commands can tell a command with no argument from one whose
// argument could not be shown. A kind outside the five is an error.
func newRunningCommand(kind paneinput.CommandKind, arg string) (cmd RunningCommand, dropped bool, err error) {
	if kind.Name() == "" {
		return RunningCommand{}, false, fmt.Errorf("%w: kind %d", errRunningCommand, int(kind))
	}
	if arg == "" {
		return RunningCommand{kind: kind}, false, nil
	}
	canonical, ok := validRunningArg(kind, arg)
	if !ok {
		return RunningCommand{kind: kind}, true, nil
	}
	return RunningCommand{kind: kind, arg: canonical}, false, nil
}

// validRunningArg returns arg in its canonical form when it is a number of
// the form the kind accepts: an issue number for /mkreq, a task number for
// /mkarch, /mkplan, and /runplan, and a PR number for /runpr.
func validRunningArg(kind paneinput.CommandKind, arg string) (string, bool) {
	switch kind {
	case paneinput.CommandMkreq:
		n, err := paneinput.ParseIssueNumber(arg)
		return n.String(), err == nil
	case paneinput.CommandMkarch, paneinput.CommandMkplan, paneinput.CommandRunplan:
		id, err := paneinput.ParseTaskID(arg)
		return id.String(), err == nil
	case paneinput.CommandRunpr:
		pr, err := fixprguard.ParsePRNumber(arg)
		return pr.String(), err == nil
	default:
		return "", false
	}
}

// Kind returns the command's kind, CommandUnknown for a value of a kind
// outside the five.
func (c RunningCommand) Kind() paneinput.CommandKind {
	if c.kind.Name() == "" {
		return paneinput.CommandUnknown
	}
	return c.kind
}

// Text returns the command line, such as "/runplan 0093", or the name alone,
// such as "/runpr", for a command with no argument or one that does not
// validate again. It returns "" for a kind outside the five, which shows no
// command.
func (c RunningCommand) Text() string {
	name := c.kind.Name()
	if name == "" {
		return ""
	}
	if arg, ok := validRunningArg(c.kind, c.arg); ok {
		return "/" + name + " " + arg
	}
	return "/" + name
}
