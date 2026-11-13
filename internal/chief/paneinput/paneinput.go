// Package paneinput holds the only types that describe an input chief may send
// to an agent's pane: a task command, the inputs that reset a pane, and the
// switch of a Claude Code model. The part of chief that writes to Herdr takes
// these types and never a string, so what can be sent is limited by the type.
//
// Every struct here has unexported fields and is built only by its
// constructors, which validate their arguments. The enumerations can still be
// converted from any integer outside this package, so every constructor and
// every method that chooses by an enumeration rejects a value outside its
// range in the default branch of its switch. The zero value of every type here
// is never sent.
package paneinput

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/isseis/yt2column/internal/fixprguard"
)

// ErrInvalid reports a value that cannot become part of an input: a malformed
// number, a zero value, an enumeration outside its range, or a combination the
// routing table does not allow.
var ErrInvalid = errors.New("invalid pane input")

// TaskID is a task's four-digit number, the prefix of its directory under
// docs/tasks. The zero value is no task.
type TaskID struct {
	digits string
}

// taskIDDigits is the number of digits a task number has.
const taskIDDigits = 4

// ParseTaskID accepts exactly four ASCII digits.
func ParseTaskID(s string) (TaskID, error) {
	if len(s) != taskIDDigits {
		return TaskID{}, fmt.Errorf("%w: task number %q is not four digits", ErrInvalid, s)
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return TaskID{}, fmt.Errorf("%w: task number %q is not four digits", ErrInvalid, s)
		}
	}
	return TaskID{digits: s}, nil
}

// String returns the four digits, or "" for the zero value.
func (t TaskID) String() string { return t.digits }

// IsZero reports whether t is the zero value.
func (t TaskID) IsZero() bool { return t.digits == "" }

// IssueNumber is a GitHub issue number. The zero value is no issue.
type IssueNumber struct {
	n int
}

// maxIssueDigits bounds an issue number's length, as fixprguard bounds a PR
// number's.
const maxIssueDigits = 9

// ParseIssueNumber accepts decimal digits only: no sign, no leading zero, and
// at most maxIssueDigits digits.
func ParseIssueNumber(s string) (IssueNumber, error) {
	if s == "" || len(s) > maxIssueDigits || s[0] == '0' {
		return IssueNumber{}, fmt.Errorf("%w: issue number %q", ErrInvalid, s)
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return IssueNumber{}, fmt.Errorf("%w: issue number %q", ErrInvalid, s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return IssueNumber{}, fmt.Errorf("%w: issue number %q", ErrInvalid, s)
	}
	return IssueNumber{n: n}, nil
}

// String returns the number in decimal, or "" for the zero value.
func (i IssueNumber) String() string {
	if i.n <= 0 {
		return ""
	}
	return strconv.Itoa(i.n)
}

// CommandKind is a task command chief may submit. The zero value is never
// sent.
type CommandKind int

const (
	// CommandUnknown is no command.
	CommandUnknown CommandKind = iota
	// CommandMkreq is /mkreq <issue>.
	CommandMkreq
	// CommandMkarch is /mkarch <task>.
	CommandMkarch
	// CommandMkplan is /mkplan <task>.
	CommandMkplan
	// CommandRunplan is /runplan <task>.
	CommandRunplan
	// CommandRunpr is /runpr <PR>.
	CommandRunpr
)

// RouteKey names one row of the routing table in _context.md. The zero value
// has no route, so nothing is sent.
type RouteKey int

const (
	// RouteUnknown is no route.
	RouteUnknown RouteKey = iota
	// RouteTaskDocuments is the row of /mkreq, /mkarch, and /mkplan.
	RouteTaskDocuments
	// RouteRunplanOpus is /runplan for the requirement opus.
	RouteRunplanOpus
	// RouteRunplanSonnetReview is /runplan for the requirement
	// sonnet+opus-review.
	RouteRunplanSonnetReview
	// RouteRunplanSonnet is /runplan for the requirement sonnet.
	RouteRunplanSonnet
	// RouteRunplanTakeover is /runplan handed back to open the PR after an
	// OpenCode run stopped before it.
	RouteRunplanTakeover
	// RouteRunpr is /runpr.
	RouteRunpr
)

// RouteKeys lists every route key with a row, in the order of the routing
// table.
func RouteKeys() []RouteKey {
	return []RouteKey{
		RouteTaskDocuments,
		RouteRunplanOpus,
		RouteRunplanSonnetReview,
		RouteRunplanSonnet,
		RouteRunplanTakeover,
		RouteRunpr,
	}
}

// AgentKind is the agent a route sends to. The zero value is no agent.
type AgentKind int

const (
	// AgentUnknown is no agent.
	AgentUnknown AgentKind = iota
	// AgentClaudeCode is Claude Code.
	AgentClaudeCode
	// AgentOpenCode is OpenCode.
	AgentOpenCode
)

// ClaudeModel is the argument of Claude Code's /model. Only the values the
// routing table may name exist; the zero value is no model.
type ClaudeModel int

const (
	// ClaudeModelUnknown is no model.
	ClaudeModelUnknown ClaudeModel = iota
	// ClaudeModelOpus is /model opus.
	ClaudeModelOpus
	// ClaudeModelSonnet is /model sonnet.
	ClaudeModelSonnet
)

// argument returns the /model argument of m, rejecting the zero value and a
// value outside the range.
func (m ClaudeModel) argument() (string, error) {
	switch m {
	case ClaudeModelOpus:
		return "opus", nil
	case ClaudeModelSonnet:
		return "sonnet", nil
	default:
		return "", fmt.Errorf("%w: Claude Code model %d", ErrInvalid, int(m))
	}
}

// ParseClaudeModel accepts the /model arguments the routing table may name:
// "opus" and "sonnet".
func ParseClaudeModel(s string) (ClaudeModel, error) {
	switch s {
	case "opus":
		return ClaudeModelOpus, nil
	case "sonnet":
		return ClaudeModelSonnet, nil
	default:
		return ClaudeModelUnknown, fmt.Errorf("%w: Claude Code model %q", ErrInvalid, s)
	}
}

// OpenCodeModel is the whole text between the first and the last middle-dot
// separator (" \u00b7 ") of an
// OpenCode status line, such as "DeepSeek V4.1 Flash 1017 OpenCode Go". It is
// compared with the screen as a whole, never word by word. The zero value is
// no model.
type OpenCodeModel struct {
	name string
}

// maxOpenCodeModelLength bounds a model name; a routing-table cell longer than
// this is not a model name.
const maxOpenCodeModelLength = 128

// ParseOpenCodeModel accepts words of printable ASCII other than the space,
// separated by single spaces, with no space at either end.
func ParseOpenCodeModel(s string) (OpenCodeModel, error) {
	if s == "" || len(s) > maxOpenCodeModelLength || s[0] == ' ' || s[len(s)-1] == ' ' {
		return OpenCodeModel{}, fmt.Errorf("%w: OpenCode model %q", ErrInvalid, s)
	}
	for i := range len(s) {
		c := s[i]
		if c < ' ' || c > '~' || (c == ' ' && s[i-1] == ' ') {
			return OpenCodeModel{}, fmt.Errorf("%w: OpenCode model %q", ErrInvalid, s)
		}
	}
	return OpenCodeModel{name: s}, nil
}

// String returns the model name, or "" for the zero value.
func (m OpenCodeModel) String() string { return m.name }

// Matches reports whether the text an OpenCode status line shows between its
// first and last middle-dot separators is exactly this model. The zero value
// matches nothing.
func (m OpenCodeModel) Matches(shown string) bool {
	return m.name != "" && shown == m.name
}

// Route is one parsed row of the routing table: the agent and its model. The
// zero value has no agent and is never sent to.
type Route struct {
	agent    AgentKind
	claude   ClaudeModel
	openCode OpenCodeModel
}

// NewClaudeRoute returns a Claude Code route with model m.
func NewClaudeRoute(m ClaudeModel) (Route, error) {
	if _, err := m.argument(); err != nil {
		return Route{}, err
	}
	return Route{agent: AgentClaudeCode, claude: m}, nil
}

// NewOpenCodeRoute returns an OpenCode route with model m.
func NewOpenCodeRoute(m OpenCodeModel) (Route, error) {
	if m.name == "" {
		return Route{}, fmt.Errorf("%w: OpenCode route without a model", ErrInvalid)
	}
	return Route{agent: AgentOpenCode, openCode: m}, nil
}

// Agent returns the route's agent, AgentUnknown for the zero value.
func (r Route) Agent() AgentKind { return r.agent }

// ClaudeModel returns the model of a Claude Code route, ClaudeModelUnknown
// otherwise.
func (r Route) ClaudeModel() ClaudeModel { return r.claude }

// OpenCodeModel returns the model of an OpenCode route, the zero value
// otherwise.
func (r Route) OpenCodeModel() OpenCodeModel { return r.openCode }

// ModelText returns the model as the routing table names it, for display.
func (r Route) ModelText() (string, error) {
	switch r.agent {
	case AgentClaudeCode:
		return r.claude.argument()
	case AgentOpenCode:
		if r.openCode.name == "" {
			return "", fmt.Errorf("%w: OpenCode route without a model", ErrInvalid)
		}
		return r.openCode.name, nil
	default:
		return "", fmt.Errorf("%w: route agent %d", ErrInvalid, int(r.agent))
	}
}

// Command is a validated task command; only its constructors make one. The
// route is decided by the constructor from the command kind (and, for
// /runplan, from the route key the caller read from the plan), never from
// text.
type Command struct {
	kind  CommandKind
	task  TaskID
	issue IssueNumber
	pr    fixprguard.PRNumber
	route RouteKey
}

// NewMkreq returns /mkreq <issue>.
func NewMkreq(issue IssueNumber) (Command, error) {
	if issue.n <= 0 {
		return Command{}, fmt.Errorf("%w: /mkreq without an issue", ErrInvalid)
	}
	return Command{kind: CommandMkreq, issue: issue, route: RouteTaskDocuments}, nil
}

// NewMkarch returns /mkarch <task>.
func NewMkarch(task TaskID) (Command, error) {
	return newTaskCommand(CommandMkarch, task, RouteTaskDocuments)
}

// NewMkplan returns /mkplan <task>.
func NewMkplan(task TaskID) (Command, error) {
	return newTaskCommand(CommandMkplan, task, RouteTaskDocuments)
}

// NewRunplan returns /runplan <task> sent by route, which must be one of the
// /runplan routes.
func NewRunplan(task TaskID, route RouteKey) (Command, error) {
	switch route {
	case RouteRunplanOpus, RouteRunplanSonnetReview, RouteRunplanSonnet, RouteRunplanTakeover:
		return newTaskCommand(CommandRunplan, task, route)
	default:
		return Command{}, fmt.Errorf("%w: route %d is not a /runplan route", ErrInvalid, int(route))
	}
}

// NewRunpr returns /runpr <pr>.
func NewRunpr(pr fixprguard.PRNumber) (Command, error) {
	if pr == (fixprguard.PRNumber{}) {
		return Command{}, fmt.Errorf("%w: /runpr without a PR number", ErrInvalid)
	}
	return Command{kind: CommandRunpr, pr: pr, route: RouteRunpr}, nil
}

func newTaskCommand(kind CommandKind, task TaskID, route RouteKey) (Command, error) {
	if task.IsZero() {
		return Command{}, fmt.Errorf("%w: command %d without a task", ErrInvalid, int(kind))
	}
	return Command{kind: kind, task: task, route: route}, nil
}

// Route returns the routing-table row the command is sent by, RouteUnknown
// for the zero value.
func (c Command) Route() RouteKey { return c.route }

// Text returns the command line typed into the pane, such as
// "/runplan 0088". It rejects the zero value and a kind outside the range, so
// a Command that no constructor made is never typed.
func (c Command) Text() (string, error) {
	switch c.kind {
	case CommandMkreq:
		if c.issue.n <= 0 {
			break
		}
		return "/mkreq " + c.issue.String(), nil
	case CommandMkarch:
		return c.taskText("/mkarch ")
	case CommandMkplan:
		return c.taskText("/mkplan ")
	case CommandRunplan:
		return c.taskText("/runplan ")
	case CommandRunpr:
		if c.pr == (fixprguard.PRNumber{}) {
			break
		}
		return "/runpr " + c.pr.String(), nil
	}
	return "", fmt.Errorf("%w: command %d", ErrInvalid, int(c.kind))
}

func (c Command) taskText(prefix string) (string, error) {
	if c.task.IsZero() {
		return "", fmt.Errorf("%w: command %d without a task", ErrInvalid, int(c.kind))
	}
	return prefix + c.task.String(), nil
}

// PromptKind is the kind of one line typed into a pane. The zero value is
// never typed.
type PromptKind int

const (
	// PromptUnknown is no input.
	PromptUnknown PromptKind = iota
	// PromptClear is Claude Code's /clear, which starts a new session.
	PromptClear
	// PromptNew is OpenCode's /new, which starts a new session.
	PromptNew
	// PromptModel is Claude Code's /model <model>.
	PromptModel
	// PromptCommand is a task command.
	PromptCommand
)

// Prompt is one line chief types into a pane: a reset, a model switch, or a
// task command. Only its constructors make one.
type Prompt struct {
	kind    PromptKind
	model   ClaudeModel
	command Command
}

// ClearSession returns Claude Code's /clear.
func ClearSession() Prompt { return Prompt{kind: PromptClear} }

// NewSession returns OpenCode's /new.
func NewSession() Prompt { return Prompt{kind: PromptNew} }

// SwitchModel returns Claude Code's /model <m>.
func SwitchModel(m ClaudeModel) (Prompt, error) {
	if _, err := m.argument(); err != nil {
		return Prompt{}, err
	}
	return Prompt{kind: PromptModel, model: m}, nil
}

// TypeCommand returns the line that submits c.
func TypeCommand(c Command) (Prompt, error) {
	if _, err := c.Text(); err != nil {
		return Prompt{}, err
	}
	return Prompt{kind: PromptCommand, command: c}, nil
}

// Text returns the line to type. It rejects the zero value and a kind outside
// the range.
func (p Prompt) Text() (string, error) {
	switch p.kind {
	case PromptClear:
		return "/clear", nil
	case PromptNew:
		return "/new", nil
	case PromptModel:
		arg, err := p.model.argument()
		if err != nil {
			return "", err
		}
		return "/model " + arg, nil
	case PromptCommand:
		return p.command.Text()
	default:
		return "", fmt.Errorf("%w: prompt %d", ErrInvalid, int(p.kind))
	}
}
