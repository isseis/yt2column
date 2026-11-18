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
	herdrClaude:   paneinput.AgentClaudeCode,
	herdrOpenCode: paneinput.AgentOpenCode,
}

// Herdr's names of the agents chief routes to, as `herdr agent start
// --kind` takes them.
const (
	herdrClaude   = "claude"
	herdrOpenCode = "opencode"
)

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

// The herdr command groups: herdrAgent of the agent operations,
// herdrWorkspace, herdrTab, and herdrPane of the workspace, tab, and pane
// operations.
const (
	herdrAgent     = "agent"
	herdrWorkspace = "workspace"
	herdrTab       = "tab"
	herdrPane      = "pane"
)

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
	out, err := h.cmds.herdr(ctx, herdrWorkspace, "list")
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
// and returns the result's members named by keys. Output of another command,
// and an envelope that also holds an error, is rejected.
func herdrResult(out []byte, id, kind string, keys ...string) (map[string]strictjson.Value, error) {
	envelope, err := strictjson.ParseObject(out)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	if envelope.Has("error") {
		return nil, fmt.Errorf("%w: result envelope holds an error", errHerdrOutput)
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
	for _, key := range []string{memberAgent, "agent_status", "cwd", "pane_id", "workspace_id"} {
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

// errWriteRefused reports an input the pane writer would not send: a pane
// with no ID, a pane of chief's own workspace, or a prompt no constructor of
// paneinput made. Nothing reaches Herdr when it is returned.
var errWriteRefused = errors.New("the pane writer refused the input")

// PaneWriter types into task agents' panes. It is the only part of the tool
// that sends input to a pane; WorkspaceMaker is the one that makes a
// workspace. Contract: it takes a paneinput.Prompt or a
// paneinput.RelayKey and never a string, so only an input of the routing
// table's kinds or an option number's key can be sent; it
// refuses chief's own pane and every pane of chief's workspace; and it never
// passes --wait, whose wait ends at the first matching state after the input
// and so says nothing about the input's own run.
type PaneWriter struct {
	cmds           Commands
	chiefPane      PaneID
	chiefWorkspace string
}

// NewPaneWriter returns the writer for the chief that runs in chiefPane of
// chiefWorkspace. Both must be known: without them no pane could be told
// apart from chief's own.
func NewPaneWriter(cmds Commands, chiefPane PaneID, chiefWorkspace string) (PaneWriter, error) {
	if chiefPane.id == "" || !workspaceIDForm.MatchString(chiefWorkspace) {
		return PaneWriter{}, fmt.Errorf("%w: chief's pane or workspace is unknown", errWriteRefused)
	}
	return PaneWriter{cmds: cmds, chiefPane: chiefPane, chiefWorkspace: chiefWorkspace}, nil
}

// Prompt types p into target's pane with `herdr agent prompt`, which submits
// it. target is the agent as Herdr reported it just before; its workspace is
// what tells it from chief's.
func (w PaneWriter) Prompt(ctx context.Context, target Agent, p paneinput.Prompt) error {
	if err := w.refuse(target); err != nil {
		return err
	}
	text, err := p.Text()
	if err != nil {
		return fmt.Errorf("%w: %w", errWriteRefused, err)
	}
	_, err = w.cmds.herdr(ctx, herdrAgent, "prompt", target.Pane.id, text)
	return err
}

// SendKeys presses k in target's pane with `herdr agent send-keys`, the
// only way to answer a question: `herdr agent prompt` refuses a blocked
// agent. It refuses the same panes as Prompt.
func (w PaneWriter) SendKeys(ctx context.Context, target Agent, k paneinput.RelayKey) error {
	if err := w.refuse(target); err != nil {
		return err
	}
	key, err := k.Key()
	if err != nil {
		return fmt.Errorf("%w: %w", errWriteRefused, err)
	}
	_, err = w.cmds.herdr(ctx, herdrAgent, "send-keys", target.Pane.id, key)
	return err
}

// refuse returns errWriteRefused for a writer no constructor made and for a
// target that is no task pane: one without an ID or a workspace, chief's own
// pane, or any pane of chief's workspace.
func (w PaneWriter) refuse(target Agent) error {
	if w.chiefPane.id == "" {
		return fmt.Errorf("%w: the writer was not made by NewPaneWriter", errWriteRefused)
	}
	if target.Pane.id == "" || target.Pane == w.chiefPane || target.Workspace == "" || target.Workspace == w.chiefWorkspace {
		return fmt.Errorf("%w: pane %q of workspace %q", errWriteRefused, target.Pane.id, target.Workspace)
	}
	return nil
}

// tabID is a Herdr tab ID read from Herdr's own output, such as "w1Y:t2".
// The zero value names no tab.
type tabID struct {
	id string
}

// String returns the ID as Herdr wrote it.
func (t tabID) String() string { return t.id }

// tabIDForm is the tab ID shape Herdr wrote in the samples, the same shape as
// a pane ID.
var tabIDForm = paneIDForm

// createdTab is a tab Herdr made, with the pane it made in it: the tab of a
// workspace create or of a tab create.
type createdTab struct {
	Workspace string
	Tab       tabID
	Pane      PaneID
	// Dir is the root pane's working directory.
	Dir string
}

// createdWorkspace is the workspace a `herdr workspace create` made, with its
// first tab and that tab's pane.
type createdWorkspace struct {
	Label string
	First createdTab
}

// parseWorkspaceCreate reads `herdr workspace create`. Contract: the first
// tab and its root pane must belong to the workspace the output names, and
// the pane to that tab; any other output is unreadable, so no ID of another
// workspace is ever read from it.
func parseWorkspaceCreate(out []byte) (createdWorkspace, error) {
	members, err := herdrResult(out, "cli:workspace:create", "workspace_created", "workspace", "tab", "root_pane")
	if err != nil {
		return createdWorkspace{}, err
	}
	ws, err := requiredObjectStrings(members, "workspace", "workspace_id", "label")
	if err != nil {
		return createdWorkspace{}, err
	}
	if !workspaceIDForm.MatchString(ws["workspace_id"]) {
		return createdWorkspace{}, fmt.Errorf("%w: workspace ID", errHerdrOutput)
	}
	first, err := parseCreatedTab(members, ws["workspace_id"])
	if err != nil {
		return createdWorkspace{}, err
	}
	return createdWorkspace{Label: ws["label"], First: first}, nil
}

// parseTabCreate reads `herdr tab create` for a tab made in workspace.
// Contract: a tab or pane of another workspace than the one asked for is
// unreadable.
func parseTabCreate(out []byte, workspace string) (createdTab, error) {
	members, err := herdrResult(out, "cli:tab:create", "tab_created", "tab", "root_pane")
	if err != nil {
		return createdTab{}, err
	}
	return parseCreatedTab(members, workspace)
}

// parseCreatedTab reads the tab and root_pane members of a creation's
// result and checks both belong to workspace and the pane to the tab.
func parseCreatedTab(members map[string]strictjson.Value, workspace string) (createdTab, error) {
	tab, err := requiredObjectStrings(members, "tab", "tab_id", "workspace_id")
	if err != nil {
		return createdTab{}, err
	}
	pane, err := requiredObjectStrings(members, "root_pane", "pane_id", "tab_id", "workspace_id", "cwd")
	if err != nil {
		return createdTab{}, err
	}
	if !tabIDForm.MatchString(tab["tab_id"]) || !paneIDForm.MatchString(pane["pane_id"]) {
		return createdTab{}, fmt.Errorf("%w: tab or pane ID", errHerdrOutput)
	}
	if tab["workspace_id"] != workspace || pane["workspace_id"] != workspace || pane["tab_id"] != tab["tab_id"] {
		return createdTab{}, fmt.Errorf("%w: the tab or its pane is not of workspace %q", errHerdrOutput, workspace)
	}
	if !filepath.IsAbs(pane["cwd"]) {
		return createdTab{}, fmt.Errorf("%w: cwd is not absolute", errHerdrOutput)
	}
	return createdTab{Workspace: workspace, Tab: tabID{id: tab["tab_id"]}, Pane: PaneID{id: pane["pane_id"]}, Dir: pane["cwd"]}, nil
}

// tabInfo is a tab as `herdr tab rename` reports it after the rename.
type tabInfo struct {
	Workspace string
	Tab       tabID
	Label     string
}

// parseTabRename reads `herdr tab rename`. The caller compares the tab and
// label it returns with the ones it asked for.
func parseTabRename(out []byte) (tabInfo, error) {
	members, err := herdrResult(out, "cli:tab:rename", "tab_info", "tab")
	if err != nil {
		return tabInfo{}, err
	}
	tab, err := requiredObjectStrings(members, "tab", "tab_id", "workspace_id", "label")
	if err != nil {
		return tabInfo{}, err
	}
	if !tabIDForm.MatchString(tab["tab_id"]) || !workspaceIDForm.MatchString(tab["workspace_id"]) {
		return tabInfo{}, fmt.Errorf("%w: tab or workspace ID", errHerdrOutput)
	}
	return tabInfo{Workspace: tab["workspace_id"], Tab: tabID{id: tab["tab_id"]}, Label: tab["label"]}, nil
}

// startedAgent is the agent a `herdr agent start` reports as started, with
// the name Herdr gave it.
type startedAgent struct {
	Agent Agent
	Name  string
}

// parseAgentStart reads a successful `herdr agent start`. The agent is read
// as parseAgent reads every agent, and it must carry a name. The caller
// compares the name, kind, and pane with the ones it asked for.
func parseAgentStart(out []byte) (startedAgent, error) {
	members, err := herdrResult(out, "cli:agent:start", "agent_started", "agent")
	if err != nil {
		return startedAgent{}, err
	}
	value, err := strictjson.Required(members, "agent")
	if err != nil {
		return startedAgent{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	a, err := parseAgent(value)
	if err != nil {
		return startedAgent{}, err
	}
	named, err := requiredObjectStrings(members, "agent", "name")
	if err != nil {
		return startedAgent{}, err
	}
	return startedAgent{Agent: a, Name: named["name"]}, nil
}

// listedPane is one pane as `herdr pane list` reports it, with or without an
// agent.
type listedPane struct {
	ID        PaneID
	Workspace string
	Tab       tabID
	// Dir is the pane's working directory.
	Dir string
	// HasAgent reports whether Herdr names an agent in the pane; Kind is
	// paneinput.AgentUnknown for an agent chief does not route to.
	HasAgent bool
	Kind     paneinput.AgentKind
}

// parsePaneList reads `herdr pane list`. Contract: a pane with a missing or
// malformed member, or an agent member that is not a non-empty string, makes
// the whole list unreadable; nothing is defaulted. Every pane is returned with
// its own workspace, so a caller that asked for one workspace filters by it.
func parsePaneList(out []byte) ([]listedPane, error) {
	members, err := herdrResult(out, "cli:pane:list", "pane_list", "panes")
	if err != nil {
		return nil, err
	}
	items, err := requiredArray(members, "panes")
	if err != nil {
		return nil, err
	}
	panes := make([]listedPane, 0, len(items))
	for i, item := range items {
		object, err := item.AsObject()
		if err != nil {
			return nil, fmt.Errorf("%w: pane %d: %w", errHerdrOutput, i, err)
		}
		fields, err := object.Collect("pane_id", "tab_id", "workspace_id", "cwd", "agent")
		if err != nil {
			return nil, fmt.Errorf("%w: pane %d: %w", errHerdrOutput, i, err)
		}
		strs := map[string]string{}
		for _, key := range []string{"pane_id", "tab_id", "workspace_id", "cwd"} {
			s, err := strictjson.RequiredString(fields, key)
			if err != nil {
				return nil, fmt.Errorf("%w: pane %d: %w", errHerdrOutput, i, err)
			}
			strs[key] = s
		}
		if !paneIDForm.MatchString(strs["pane_id"]) || !tabIDForm.MatchString(strs["tab_id"]) || !workspaceIDForm.MatchString(strs["workspace_id"]) {
			return nil, fmt.Errorf("%w: pane %d IDs", errHerdrOutput, i)
		}
		if !filepath.IsAbs(strs["cwd"]) {
			return nil, fmt.Errorf("%w: pane %d cwd is not absolute", errHerdrOutput, i)
		}
		agent, hasAgent, err := strictjson.OptionalString(fields, "agent")
		if err != nil {
			return nil, fmt.Errorf("%w: pane %d agent: %w", errHerdrOutput, i, err)
		}
		panes = append(panes, listedPane{
			ID:        PaneID{id: strs["pane_id"]},
			Workspace: strs["workspace_id"],
			Tab:       tabID{id: strs["tab_id"]},
			Dir:       strs["cwd"],
			HasAgent:  hasAgent,
			Kind:      agentKinds[agent],
		})
	}
	return panes, nil
}

// processInfo is what `herdr pane process-info` reports for one pane: the
// shell's pid and the foreground process group with its processes.
type processInfo struct {
	Pane            PaneID
	ShellPID        int64
	ForegroundGroup int64
	ForegroundPIDs  []int64
}

// ShellInForeground reports whether the pane's shell is at its prompt: the
// foreground process group is the shell's own and the shell is its only
// process. Contract: anything else, a build or an editor in the foreground
// among them, is not.
func (p processInfo) ShellInForeground() bool {
	return p.ShellPID > 0 && p.ForegroundGroup == p.ShellPID && len(p.ForegroundPIDs) == 1 && p.ForegroundPIDs[0] == p.ShellPID
}

// parsePaneProcessInfo reads `herdr pane process-info`. The caller compares
// the pane it returns with the one it asked about.
func parsePaneProcessInfo(out []byte) (processInfo, error) {
	members, err := herdrResult(out, "cli:pane:process_info", "pane_process_info", "process_info")
	if err != nil {
		return processInfo{}, err
	}
	value, err := strictjson.Required(members, "process_info")
	if err != nil {
		return processInfo{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	object, err := value.AsObject()
	if err != nil {
		return processInfo{}, fmt.Errorf("%w: process_info: %w", errHerdrOutput, err)
	}
	fields, err := object.Collect("pane_id", "shell_pid", "foreground_process_group_id", "foreground_processes")
	if err != nil {
		return processInfo{}, fmt.Errorf("%w: process_info: %w", errHerdrOutput, err)
	}
	pane, err := strictjson.RequiredString(fields, "pane_id")
	if err != nil || !paneIDForm.MatchString(pane) {
		return processInfo{}, fmt.Errorf("%w: process_info pane ID", errHerdrOutput)
	}
	shell, err := requiredPID(fields, "shell_pid")
	if err != nil {
		return processInfo{}, err
	}
	group, err := requiredPID(fields, "foreground_process_group_id")
	if err != nil {
		return processInfo{}, err
	}
	items, err := requiredArray(fields, "foreground_processes")
	if err != nil {
		return processInfo{}, err
	}
	pids := make([]int64, 0, len(items))
	for i, item := range items {
		process, err := item.AsObject()
		if err != nil {
			return processInfo{}, fmt.Errorf("%w: process %d: %w", errHerdrOutput, i, err)
		}
		pf, err := process.Collect("pid")
		if err != nil {
			return processInfo{}, fmt.Errorf("%w: process %d: %w", errHerdrOutput, i, err)
		}
		pid, err := requiredPID(pf, "pid")
		if err != nil {
			return processInfo{}, err
		}
		pids = append(pids, pid)
	}
	return processInfo{Pane: PaneID{id: pane}, ShellPID: shell, ForegroundGroup: group, ForegroundPIDs: pids}, nil
}

// processes reads the processes of pane with `herdr pane process-info`.
// Contract: an answer about another pane is an error, never read as pane's.
func (h Herdr) processes(ctx context.Context, pane PaneID) (processInfo, error) {
	out, err := h.cmds.herdr(ctx, herdrPane, "process-info", "--pane", pane.String())
	if err != nil {
		return processInfo{}, err
	}
	info, err := parsePaneProcessInfo(out)
	if err != nil {
		return processInfo{}, err
	}
	if info.Pane != pane {
		return processInfo{}, fmt.Errorf("%w: herdr reported the processes of pane %s for %s", errHerdrOutput, info.Pane, pane)
	}
	return info, nil
}

// requiredPID returns the named member as a positive process ID.
func requiredPID(members map[string]strictjson.Value, key string) (int64, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	pid, err := value.AsNonNegativeInt64()
	if err != nil || pid == 0 {
		return 0, fmt.Errorf("%w: %s is not a process ID", errHerdrOutput, key)
	}
	return pid, nil
}

// parseWorkspaceClose reads a successful `herdr workspace close`, whose
// result carries nothing but its type.
func parseWorkspaceClose(out []byte) error {
	_, err := herdrResult(out, "cli:workspace:close", "ok")
	return err
}

// herdrError is Herdr's error response, which herdr writes to standard error
// with exit status 1: {"error": {"code": ..., "message": ...}, "id": ...}.
// Code is a value Herdr declares and Message is Herdr's text; both are only
// shown, and no behavior is chosen by either.
type herdrError struct {
	Code    string
	Message string
}

// herdrErrorCodeForm is the shape of the codes the samples showed, such as
// "agent_name_taken".
var herdrErrorCodeForm = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// parseHerdrError reads the error response of the command whose envelope id
// is id. Contract: another command's response, a code of another shape, a
// missing member, or an envelope that also holds a result is unreadable.
func parseHerdrError(stderr []byte, id string) (herdrError, error) {
	envelope, err := strictjson.ParseObject(stderr)
	if err != nil {
		return herdrError{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	if envelope.Has("result") {
		return herdrError{}, fmt.Errorf("%w: error envelope holds a result", errHerdrOutput)
	}
	top, err := envelope.Collect("id", "error")
	if err != nil {
		return herdrError{}, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	gotID, err := strictjson.RequiredString(top, "id")
	if err != nil || gotID != id {
		return herdrError{}, fmt.Errorf("%w: envelope id", errHerdrOutput)
	}
	body, err := requiredObjectStrings(top, "error", "code", "message")
	if err != nil {
		return herdrError{}, err
	}
	if !herdrErrorCodeForm.MatchString(body["code"]) {
		return herdrError{}, fmt.Errorf("%w: error code", errHerdrOutput)
	}
	return herdrError{Code: body["code"], Message: body["message"]}, nil
}

// requiredObjectStrings returns the named members of the object member key,
// each a non-empty string.
func requiredObjectStrings(members map[string]strictjson.Value, key string, names ...string) (map[string]string, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errHerdrOutput, err)
	}
	object, err := value.AsObject()
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errHerdrOutput, key, err)
	}
	fields, err := object.Collect(names...)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errHerdrOutput, key, err)
	}
	strs := make(map[string]string, len(names))
	for _, name := range names {
		s, err := strictjson.RequiredString(fields, name)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errHerdrOutput, key, err)
		}
		strs[name] = s
	}
	return strs, nil
}
