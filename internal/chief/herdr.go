package chief

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/strictjson"
)

// errHerdrOutput reports Herdr output of a shape the tool does not know: an
// unknown status value, a missing or malformed member, or another command's
// envelope. Such output is not read more widely; the caller treats what it
// describes as unreadable.
var errHerdrOutput = errors.New("unreadable herdr output")

// AgentStatus is an agent's status as Herdr reports it. The zero value is
// AgentStatusUnknown, which no reader returns: an unknown status makes the
// whole output unreadable.
type AgentStatus int

const (
	// AgentStatusUnknown is no status.
	AgentStatusUnknown AgentStatus = iota
	// AgentIdle waits for input before any response in this session.
	AgentIdle
	// AgentWorking is running.
	AgentWorking
	// AgentBlocked waits on a question or a permission prompt.
	AgentBlocked
	// AgentDone finished a response and waits for input.
	AgentDone
)

// agentStatuses maps Herdr's status values; a value not listed is unreadable.
var agentStatuses = map[string]AgentStatus{
	"idle":    AgentIdle,
	"working": AgentWorking,
	"blocked": AgentBlocked,
	"done":    AgentDone,
}

// agentKinds maps Herdr's agent names to the agents chief can route to. An
// agent of another name is read with paneinput.AgentUnknown, so it is listed
// but never chosen as a target.
var agentKinds = map[string]paneinput.AgentKind{
	"claude":   paneinput.AgentClaudeCode,
	"opencode": paneinput.AgentOpenCode,
}

// PaneID is a Herdr pane ID read from Herdr's own output, such as "w1N:p3".
// The zero value names no pane.
type PaneID struct {
	id string
}

// String returns the ID as Herdr wrote it.
func (p PaneID) String() string { return p.id }

// paneIDForm and workspaceIDForm are the ID shapes Herdr wrote in the
// samples. An ID of another shape (one starting with "-", which herdr would
// read as an option, among them) is unreadable.
var (
	paneIDForm      = regexp.MustCompile(`^[A-Za-z0-9]+:[A-Za-z0-9]+$`)
	workspaceIDForm = regexp.MustCompile(`^[A-Za-z0-9]+$`)
)

// Agent is one agent pane as Herdr reports it.
type Agent struct {
	Pane      PaneID
	Workspace string
	Kind      paneinput.AgentKind
	Status    AgentStatus
	// Session is the agent session ID Herdr holds for the pane, empty when
	// Herdr reports none (OpenCode's panes report none).
	Session string
	// Dir is the pane's working directory.
	Dir            string
	StateChangeSeq int64
}

// Workspace is one Herdr workspace with the label Herdr shows for it.
type Workspace struct {
	ID    string
	Label string
}

// ScreenSource is where `herdr agent read` takes a screen from. The zero
// value reads nothing.
type ScreenSource int

const (
	// ScreenSourceUnknown is no source.
	ScreenSourceUnknown ScreenSource = iota
	// ScreenVisible is the rows the pane shows now; Herdr allows only this
	// source while an agent is blocked.
	ScreenVisible
	// ScreenRecent is the recent scrollback.
	ScreenRecent
)

// herdrAgent is the herdr command group of the agent operations.
const herdrAgent = "agent"

// screenLines is how many lines a screen read asks for; Herdr returns at
// most the pane's height.
const screenLines = 200

// Herdr reads Herdr's state through the herdr command.
type Herdr struct {
	cmds Commands
}

// NewHerdr returns the Herdr reader that runs herdr through cmds.
func NewHerdr(cmds Commands) Herdr { return Herdr{cmds: cmds} }

// Agents returns every agent pane `herdr agent list` reports.
func (h Herdr) Agents(ctx context.Context) ([]Agent, error) {
	out, err := h.cmds.herdr(ctx, herdrAgent, "list")
	if err != nil {
		return nil, err
	}
	return parseAgentList(out)
}

// Agent returns the agent of one pane, read with `herdr agent get`.
func (h Herdr) Agent(ctx context.Context, pane PaneID) (Agent, error) {
	if pane.id == "" {
		return Agent{}, fmt.Errorf("%w: no pane", errHerdrOutput)
	}
	out, err := h.cmds.herdr(ctx, herdrAgent, "get", pane.id)
	if err != nil {
		return Agent{}, err
	}
	return parseAgentGet(out)
}

// Workspaces returns every workspace `herdr workspace list` reports.
func (h Herdr) Workspaces(ctx context.Context) ([]Workspace, error) {
	out, err := h.cmds.herdr(ctx, "workspace", "list")
	if err != nil {
		return nil, err
	}
	return parseWorkspaceList(out)
}

// Screen returns a pane's screen with its colors and attributes, as the
// screen readers take it.
func (h Herdr) Screen(ctx context.Context, pane PaneID, source ScreenSource) (string, error) {
	if pane.id == "" {
		return "", fmt.Errorf("%w: no pane", errHerdrOutput)
	}
	var src string
	switch source {
	case ScreenVisible:
		src = "visible"
	case ScreenRecent:
		src = "recent"
	default:
		return "", fmt.Errorf("%w: screen source %d", errHerdrOutput, int(source))
	}
	out, err := h.cmds.herdr(ctx, herdrAgent, "read", pane.id, "--source", src, "--lines", strconv.Itoa(screenLines), "--format", "ansi")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// herdrResult reads Herdr's envelope {"id": id, "result": {"type": kind, ...}}
// and returns the result's members named by keys. Output of another command
// is rejected.
func herdrResult(out []byte, id, kind string, keys ...string) (map[string]strictjson.Value, error) {
	envelope, err := strictjson.ParseObject(out)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	top, err := envelope.Collect("id", "result")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	gotID, err := strictjson.RequiredString(top, "id")
	if err != nil || gotID != id {
		return nil, fmt.Errorf("%w: envelope id", errHerdrOutput)
	}
	resultValue, err := strictjson.Required(top, "result")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	result, err := resultValue.AsObject()
	if err != nil {
		return nil, fmt.Errorf("%w: result: %w", errHerdrOutput, err)
	}
	members, err := result.Collect(append([]string{"type"}, keys...)...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	gotKind, err := strictjson.RequiredString(members, "type")
	if err != nil || gotKind != kind {
		return nil, fmt.Errorf("%w: result type", errHerdrOutput)
	}
	return members, nil
}

// requiredArray returns the named member as an array.
func requiredArray(members map[string]strictjson.Value, key string) ([]strictjson.Value, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	items, err := value.AsArray()
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errHerdrOutput, key, err)
	}
	return items, nil
}

func parseAgentList(out []byte) ([]Agent, error) {
	members, err := herdrResult(out, "cli:agent:list", "agent_list", "agents")
	if err != nil {
		return nil, err
	}
	items, err := requiredArray(members, "agents")
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, 0, len(items))
	for i, item := range items {
		a, err := parseAgent(item)
		if err != nil {
			return nil, fmt.Errorf("agent %d: %w", i, err)
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func parseAgentGet(out []byte) (Agent, error) {
	members, err := herdrResult(out, "cli:agent:get", "agent_info", "agent")
	if err != nil {
		return Agent{}, err
	}
	value, err := strictjson.Required(members, "agent")
	if err != nil {
		return Agent{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	return parseAgent(value)
}

// parseAgent reads one agent object. Contract: an unknown status, a missing or
// malformed member, or an agent session of another shape than the samples'
// makes the agent unreadable; nothing is defaulted.
func parseAgent(v strictjson.Value) (Agent, error) {
	object, err := v.AsObject()
	if err != nil {
		return Agent{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	members, err := object.Collect("agent", "agent_status", "agent_session", "cwd", "pane_id", "workspace_id", "state_change_seq")
	if err != nil {
		return Agent{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	strs := map[string]string{}
	for _, key := range []string{"agent", "agent_status", "cwd", "pane_id", "workspace_id"} {
		s, err := strictjson.RequiredString(members, key)
		if err != nil {
			return Agent{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
		}
		strs[key] = s
	}
	status, ok := agentStatuses[strs["agent_status"]]
	if !ok {
		return Agent{}, fmt.Errorf("%w: agent status %q", errHerdrOutput, strs["agent_status"])
	}
	if !paneIDForm.MatchString(strs["pane_id"]) || !workspaceIDForm.MatchString(strs["workspace_id"]) {
		return Agent{}, fmt.Errorf("%w: pane or workspace ID", errHerdrOutput)
	}
	if !filepath.IsAbs(strs["cwd"]) {
		return Agent{}, fmt.Errorf("%w: cwd is not absolute", errHerdrOutput)
	}
	seqValue, err := strictjson.Required(members, "state_change_seq")
	if err != nil {
		return Agent{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	seq, err := seqValue.AsNonNegativeInt64()
	if err != nil {
		return Agent{}, fmt.Errorf("%w: state_change_seq: %w", errHerdrOutput, err)
	}
	session, err := parseAgentSession(members, strs["agent"])
	if err != nil {
		return Agent{}, err
	}
	return Agent{
		Pane:           PaneID{id: strs["pane_id"]},
		Workspace:      strs["workspace_id"],
		Kind:           agentKinds[strs["agent"]],
		Status:         status,
		Session:        session,
		Dir:            strs["cwd"],
		StateChangeSeq: seq,
	}, nil
}

// sessionIDForm is the shape of the session IDs the samples showed (a UUID).
// Claude Code's transcript path is built from the ID, so no other shape is
// read.
var sessionIDForm = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// parseAgentSession reads the optional agent_session member: absent or null is
// no session; otherwise it must be an object of kind "id", for the pane's own
// agent, with a UUID value, the only shape the samples showed.
func parseAgentSession(members map[string]strictjson.Value, agent string) (string, error) {
	value, ok := members["agent_session"]
	if !ok || value.IsNull() {
		return "", nil
	}
	object, err := value.AsObject()
	if err != nil {
		return "", fmt.Errorf("%w: agent_session: %w", errHerdrOutput, err)
	}
	session, err := object.Collect("agent", "kind", "value")
	if err != nil {
		return "", fmt.Errorf("%w: agent_session: %w", errHerdrOutput, err)
	}
	kind, err := strictjson.RequiredString(session, "kind")
	if err != nil || kind != "id" {
		return "", fmt.Errorf("%w: agent_session kind", errHerdrOutput)
	}
	owner, err := strictjson.RequiredString(session, "agent")
	if err != nil || owner != agent {
		return "", fmt.Errorf("%w: agent_session agent", errHerdrOutput)
	}
	id, err := strictjson.RequiredString(session, "value")
	if err != nil || !sessionIDForm.MatchString(id) {
		return "", fmt.Errorf("%w: agent_session value", errHerdrOutput)
	}
	return id, nil
}

func parseWorkspaceList(out []byte) ([]Workspace, error) {
	members, err := herdrResult(out, "cli:workspace:list", "workspace_list", "workspaces")
	if err != nil {
		return nil, err
	}
	items, err := requiredArray(members, "workspaces")
	if err != nil {
		return nil, err
	}
	workspaces := make([]Workspace, 0, len(items))
	for i, item := range items {
		object, err := item.AsObject()
		if err != nil {
			return nil, fmt.Errorf("%w: workspace %d: %w", errHerdrOutput, i, err)
		}
		fields, err := object.Collect("workspace_id", "label")
		if err != nil {
			return nil, fmt.Errorf("%w: workspace %d: %w", errHerdrOutput, i, err)
		}
		id, err := strictjson.RequiredString(fields, "workspace_id")
		if err != nil || !workspaceIDForm.MatchString(id) {
			return nil, fmt.Errorf("%w: workspace %d ID", errHerdrOutput, i)
		}
		label, err := strictjson.RequiredStringAllowEmpty(fields, "label")
		if err != nil {
			return nil, fmt.Errorf("%w: workspace %d label: %w", errHerdrOutput, i, err)
		}
		workspaces = append(workspaces, Workspace{ID: id, Label: label})
	}
	return workspaces, nil
}

// ParsePaneID returns the pane ID s, such as the HERDR_PANE_ID the entry
// point read. An ID of another shape than Herdr writes is rejected.
func ParsePaneID(s string) (PaneID, error) {
	if !paneIDForm.MatchString(s) {
		return PaneID{}, fmt.Errorf("%w: pane ID %q", errHerdrOutput, s)
	}
	return PaneID{id: s}, nil
}
