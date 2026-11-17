package chief

import (
	"context"
	"errors"
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

// RunningUnknown is why no running command is decided for a running row.
// The zero value is RunningUnknownNone: a command is decided.
type RunningUnknown int

const (
	// RunningUnknownNone is a decided command.
	RunningUnknownNone RunningUnknown = iota
	// RunningUnknownNoSession is a Claude Code pane Herdr reports no session
	// for, or a session ID not of the form Herdr gives.
	RunningUnknownNoSession
	// RunningUnknownUnsupportedPath is a transcript whose place cannot be
	// decided from the pane's working directory.
	RunningUnknownUnsupportedPath
	// RunningUnknownUnreadable is a transcript or a send record that cannot
	// be read. A pane of no known agent reads so too: neither of its records
	// can be read.
	RunningUnknownUnreadable
	// RunningUnknownOverCap is a transcript over the read cap.
	RunningUnknownOverCap
	// RunningUnknownPartialLine is a transcript that still ends inside a line
	// after its re-reads.
	RunningUnknownPartialLine
	// RunningUnknownNotTaskCommand is a session whose last human input is
	// none of the five task commands, or no input at all.
	RunningUnknownNotTaskCommand
	// RunningUnknownNoSentCommand is an OpenCode pane whose last send from
	// chief is not a command recorded as sent.
	RunningUnknownNoSentCommand
	// RunningUnknownPanesDisagree is working panes that do not name one
	// command: different commands, or an argument dropped on one of two or
	// more, which cannot be compared.
	RunningUnknownPanesDisagree
)

// RunningShown is what a running row shows: the command, and the model of
// chief's send record when the conditions of the design (3.5) hold, ""
// otherwise.
type RunningShown struct {
	Command RunningCommand
	Model   string
}

// lastCommand is chief's last send to a pane, a relay aside, when it was a
// command whose end says it was sent: the command its record declares, and
// the record's model. ok is false when there is no such send.
type lastCommand struct {
	cmd     RunningCommand
	dropped bool
	// agent is the agent label the record declares it was sent to.
	agent string
	model string
	ok    bool
}

// lastSentCommand reads chief's last send to pane for the running command.
// Contract: the command is read only from the record's declared kind and
// argument, never from its command text; a send that is not a command (a
// reset), a record without a kind of the five (one written before the kind
// was declared), and a send whose end is missing or not sent is no command;
// a record that cannot be read is an error.
func (r WorkspaceRecords) lastSentCommand(pane string) (lastCommand, error) {
	rec, err := r.LastSend(pane)
	if err != nil || rec == nil || rec.Input != InputCommand.String() {
		return lastCommand{}, err
	}
	kind, err := paneinput.ParseCommandKind(rec.Kind)
	if err != nil {
		return lastCommand{}, nil
	}
	end, err := r.sendEnded(rec.ID)
	if err != nil || end == nil || end.Outcome != OutcomeSent.String() {
		return lastCommand{}, err
	}
	cmd, dropped, err := newRunningCommand(kind, rec.Arg)
	if err != nil {
		return lastCommand{}, nil
	}
	return lastCommand{cmd: cmd, dropped: dropped, agent: rec.Agent, model: rec.Model, ok: true}, nil
}

// paneRunning is the running command of one working pane, or why none is
// decided, and chief's last command sent to the pane.
type paneRunning struct {
	cmd     RunningCommand
	dropped bool
	why     RunningUnknown
	sent    lastCommand
}

// runningOfPane decides the running command of one working pane, by its
// agent: a Claude Code pane by its session's last human input, an OpenCode
// pane by chief's last command sent to it, a send to another agent the
// pane held before counting as none. Contract: the screen is never
// read, and nothing read here changes a row's state or a send; sends reads
// chief's last send to a pane, and its error makes only an OpenCode pane
// undecided, since a Claude Code pane reads it for the model alone.
func runningOfPane(ctx context.Context, deps Deps, a Agent, sends func(pane string) (lastCommand, error)) paneRunning {
	sent, sentErr := sends(a.Pane.String())
	if sentErr == nil && sent.agent != agentLabel(a.Kind) {
		// The send went to another agent the pane held then: it says
		// nothing of the agent working there now.
		sent = lastCommand{}
	}
	switch a.Kind {
	case paneinput.AgentClaudeCode:
		if sentErr != nil {
			sent = lastCommand{}
		}
		if !sessionIDForm.MatchString(a.Session) {
			return paneRunning{why: RunningUnknownNoSession}
		}
		path, err := transcriptPath(deps.Env, a.Dir, a.Session)
		switch {
		case errors.Is(err, errUnsupportedPath):
			return paneRunning{why: RunningUnknownUnsupportedPath}
		case err != nil:
			return paneRunning{why: RunningUnknownUnreadable}
		}
		data, err := readCompleteTranscript(ctx, deps.Sleep, path, sessionTranscriptLimit)
		switch {
		case errors.Is(err, errOverCap):
			return paneRunning{why: RunningUnknownOverCap}
		case errors.Is(err, errPartialLine):
			return paneRunning{why: RunningUnknownPartialLine}
		case err != nil:
			return paneRunning{why: RunningUnknownUnreadable}
		}
		cmd, dropped, why := readRunningCommand(data)
		return paneRunning{cmd: cmd, dropped: dropped, why: why, sent: sent}
	case paneinput.AgentOpenCode:
		switch {
		case sentErr != nil:
			return paneRunning{why: RunningUnknownUnreadable}
		case !sent.ok:
			return paneRunning{why: RunningUnknownNoSentCommand}
		}
		return paneRunning{cmd: sent.cmd, dropped: sent.dropped, sent: sent}
	default:
		return paneRunning{why: RunningUnknownUnreadable}
	}
}

// runningOf decides the running command of a running row from its working
// panes, in Herdr's order. Contract: a command is shown only when every
// working pane decides one and all of them name the same command, with no
// argument dropped on any pane when there are two or more; otherwise
// nothing is shown, with the reason of the first undecided pane, or
// RunningUnknownPanesDisagree. The model of chief's send record is shown
// only for exactly one working pane whose last command sent is the command
// shown, the argument whole on both.
func runningOf(ctx context.Context, deps Deps, agents []Agent, sends func(pane string) (lastCommand, error)) (*RunningShown, RunningUnknown) {
	var panes []paneRunning
	for _, a := range agents {
		if a.Status != AgentWorking {
			continue
		}
		panes = append(panes, runningOfPane(ctx, deps, a, sends))
	}
	if len(panes) == 0 {
		// classify makes a row running only for a working pane, so this is a
		// caller's mistake; no pane names a command, and nothing is shown.
		return nil, RunningUnknownPanesDisagree
	}
	for _, p := range panes {
		if p.why != RunningUnknownNone {
			return nil, p.why
		}
	}
	first := panes[0]
	if len(panes) > 1 {
		for _, p := range panes {
			if p.dropped || p.cmd != first.cmd {
				return nil, RunningUnknownPanesDisagree
			}
		}
	}
	shown := &RunningShown{Command: first.cmd}
	if len(panes) == 1 && first.sent.ok && !first.dropped && !first.sent.dropped && first.sent.cmd == first.cmd {
		shown.Model = first.sent.model
	}
	return shown, RunningUnknownNone
}
