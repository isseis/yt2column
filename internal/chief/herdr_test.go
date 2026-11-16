//go:build test

package chief

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// TestReadHerdrSamples reads the real Herdr outputs under testdata/herdr.
func TestReadHerdrSamples(t *testing.T) {
	t.Run("agent list", func(t *testing.T) {
		agents, err := parseAgentList(readTestdata(t, "herdr/agent-list.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(agents) != 10 {
			t.Fatalf("read %d agents, want 10", len(agents))
		}
		want := Agent{
			Pane:           PaneID{id: "w1N:p3"},
			Workspace:      "w1N",
			Kind:           paneinput.AgentClaudeCode,
			Status:         AgentIdle,
			Session:        "00000000-0000-4000-8000-000000000002",
			Dir:            "/Users/user/.herdr/worktrees/yt2column/chief-poc",
			StateChangeSeq: 3054,
		}
		if agents[9] != want {
			t.Errorf("agent 9 = %+v, want %+v", agents[9], want)
		}
		if agents[0].Kind != paneinput.AgentOpenCode || agents[0].Status != AgentWorking || agents[0].Session != "" {
			t.Errorf("agent 0 = %+v, want a working OpenCode agent without a session", agents[0])
		}
	})
	gets := map[string]struct {
		kind    paneinput.AgentKind
		status  AgentStatus
		session bool
	}{
		"agent-get-claude-blocked.json":   {paneinput.AgentClaudeCode, AgentBlocked, true},
		"agent-get-claude-idle.json":      {paneinput.AgentClaudeCode, AgentIdle, true},
		"agent-get-opencode-blocked.json": {paneinput.AgentOpenCode, AgentBlocked, false},
		"agent-get-opencode-done.json":    {paneinput.AgentOpenCode, AgentDone, false},
	}
	for name, want := range gets {
		t.Run(name, func(t *testing.T) {
			a, err := parseAgentGet(readTestdata(t, "herdr/"+name))
			if err != nil {
				t.Fatal(err)
			}
			if a.Kind != want.kind || a.Status != want.status || (a.Session != "") != want.session {
				t.Errorf("agent = %+v, want kind %d status %d session %v", a, want.kind, want.status, want.session)
			}
		})
	}
	t.Run("workspace list", func(t *testing.T) {
		workspaces, err := parseWorkspaceList(readTestdata(t, "herdr/workspace-list.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(workspaces) != 5 || workspaces[4] != (Workspace{ID: "w1N", Label: "chief-poc"}) {
			t.Errorf("workspaces = %+v", workspaces)
		}
	})
}

// TestHerdrRejectsUnknownShapes edits the real `herdr agent get` sample into
// shapes the samples did not show and checks each is unreadable rather than
// defaulted.
func TestHerdrRejectsUnknownShapes(t *testing.T) {
	sample := string(readTestdata(t, "herdr/agent-get-claude-blocked.json"))
	edits := map[string][2]string{
		"unknown status":         {`"agent_status":"blocked"`, `"agent_status":"sleeping"`},
		"missing status":         {`"agent_status":"blocked",`, ``},
		"negative seq":           {`"state_change_seq":3013`, `"state_change_seq":-1`},
		"missing seq":            {`"state_change_seq":3013,`, ``},
		"option-like pane":       {`"pane_id":"w1N:p3"`, `"pane_id":"--source"`},
		"relative cwd":           {`"cwd":"/Users/user/.herdr/worktrees/yt2column/chief-poc"`, `"cwd":"chief-poc"`},
		"session of other kind":  {`"kind":"id"`, `"kind":"name"`},
		"empty session value":    {`"value":"00000000-0000-4000-8000-000000000001"`, `"value":""`},
		"path session value":     {`"value":"00000000-0000-4000-8000-000000000001"`, `"value":"../../x"`},
		"session of other agent": {`"agent":"claude","kind"`, `"agent":"opencode","kind"`},
		"missing cwd":            {`"cwd":"/Users/user/.herdr/worktrees/yt2column/chief-poc",`, ``},
		"missing agent":          {`"agent":"claude","agent_session"`, `"agent_session"`},
		"fractional seq":         {`"state_change_seq":3013`, `"state_change_seq":30.5`},
		"malformed workspace":    {`"workspace_id":"w1N"`, `"workspace_id":"w1N:p3"`},
		"another envelope":       {`"cli:agent:get"`, `"cli:agent:list"`},
		"another result type":    {`"type":"agent_info"`, `"type":"agent_list"`},
		"trailing data":          {`"type":"agent_info"}}`, `"type":"agent_info"}}{}`},
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			edited := strings.Replace(sample, edit[0], edit[1], 1)
			if edited == sample {
				t.Fatalf("the sample has no %q", edit[0])
			}
			if _, err := parseAgentGet([]byte(edited)); !errors.Is(err, errHerdrOutput) {
				t.Errorf("error = %v, want errHerdrOutput", err)
			}
		})
	}
	t.Run("agents not an array", func(t *testing.T) {
		if _, err := parseAgentList([]byte(`{"id":"cli:agent:list","result":{"type":"agent_list","agents":{}}}`)); !errors.Is(err, errHerdrOutput) {
			t.Errorf("error = %v, want errHerdrOutput", err)
		}
	})
	t.Run("workspace without a label", func(t *testing.T) {
		if _, err := parseWorkspaceList([]byte(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[{"workspace_id":"w1"}]}}`)); !errors.Is(err, errHerdrOutput) {
			t.Errorf("error = %v, want errHerdrOutput", err)
		}
	})
}

// TestHerdrUnknownAgentIsNoTarget checks that an agent of a name chief does
// not route to is listed with no kind, not rejected.
func TestHerdrUnknownAgentIsNoTarget(t *testing.T) {
	sample := strings.Replace(string(readTestdata(t, "herdr/agent-get-opencode-done.json")), `"agent":"opencode"`, `"agent":"codex"`, 1)
	a, err := parseAgentGet([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != paneinput.AgentUnknown {
		t.Errorf("kind = %d, want AgentUnknown", a.Kind)
	}
}

// TestHerdrCommandLines checks the herdr command lines each read runs, and
// that a read without a pane or a source runs nothing.
func TestHerdrCommandLines(t *testing.T) {
	rec := newRecorder(t)
	rec.reply("herdr agent list", readTestdata(t, "herdr/agent-list.json"))
	rec.reply("herdr agent get w1N:p3", readTestdata(t, "herdr/agent-get-claude-blocked.json"))
	rec.reply("herdr workspace list", readTestdata(t, "herdr/workspace-list.json"))
	rec.reply("herdr agent read w1N:p3 --source visible --lines 200 --format ansi", []byte("screen"))
	rec.reply("herdr agent read w1N:p3 --source recent --lines 200 --format ansi", []byte("recent"))
	h := NewHerdr(rec.commands(nil))
	ctx := context.Background()
	agents, err := h.Agents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pane := agents[9].Pane
	if _, err := h.Agent(ctx, pane); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Workspaces(ctx); err != nil {
		t.Fatal(err)
	}
	screen, err := h.Screen(ctx, pane, ScreenVisible)
	if err != nil || screen != "screen" {
		t.Errorf("Screen = %q, %v", screen, err)
	}
	if recent, err := h.Screen(ctx, pane, ScreenRecent); err != nil || recent != "recent" {
		t.Errorf("Screen(recent) = %q, %v", recent, err)
	}
	before := len(rec.recorded())
	if _, err := h.Screen(ctx, pane, ScreenSourceUnknown); !errors.Is(err, errHerdrOutput) {
		t.Errorf("Screen without a source: %v", err)
	}
	if _, err := h.Screen(ctx, PaneID{}, ScreenRecent); !errors.Is(err, errHerdrOutput) {
		t.Errorf("Screen without a pane: %v", err)
	}
	if _, err := h.Agent(ctx, PaneID{}); !errors.Is(err, errHerdrOutput) {
		t.Errorf("Agent without a pane: %v", err)
	}
	if calls := rec.recorded(); len(calls) != before {
		t.Errorf("the rejected reads ran %v", calls[before:])
	}
}

// TestWriterRefuses checks that the pane writer types only a constructed
// prompt or relay key into a task pane: the zero prompt or key, chief's own
// pane, another pane of chief's workspace, a pane without an ID, and a
// writer no constructor made all send nothing, while a valid prompt is
// submitted without --wait and a valid key is pressed.
func TestWriterRefuses(t *testing.T) {
	ctx := context.Background()
	chiefPane := PaneID{id: "wc:p1"}
	task := Agent{Pane: PaneID{id: "w1:p2"}, Workspace: "w1"}
	clearPrompt := paneinput.ClearSession()
	relayKey, err := paneinput.SelectOption(paneinput.AgentClaudeCode, 2)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		writer func(Commands) PaneWriter
		target Agent
		prompt paneinput.Prompt
	}{
		{"zero prompt", nil, task, paneinput.Prompt{}},
		{"chief's pane", nil, Agent{Pane: chiefPane, Workspace: "wc"}, clearPrompt},
		{"chief's pane named in another workspace", nil, Agent{Pane: chiefPane, Workspace: "w1"}, clearPrompt},
		{"another pane of chief's workspace", nil, Agent{Pane: PaneID{id: "wc:p2"}, Workspace: "wc"}, clearPrompt},
		{"a pane without an ID", nil, Agent{Workspace: "w1"}, clearPrompt},
		{"a pane without a workspace", nil, Agent{Pane: PaneID{id: "w1:p2"}}, clearPrompt},
		{"a writer no constructor made", func(c Commands) PaneWriter { return PaneWriter{cmds: c} }, task, clearPrompt},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := newRecorder(t)
			rec.onPrompt = func(string, string) error { return nil }
			w, err := NewPaneWriter(rec.commands(nil), chiefPane, "wc")
			if err != nil {
				t.Fatal(err)
			}
			if c.writer != nil {
				w = c.writer(rec.commands(nil))
			}
			if err := w.Prompt(ctx, c.target, c.prompt); !errors.Is(err, errWriteRefused) {
				t.Errorf("Prompt = %v, want errWriteRefused", err)
			}
			key := relayKey
			if c.prompt == (paneinput.Prompt{}) {
				key = paneinput.RelayKey{}
			}
			if err := w.SendKeys(ctx, c.target, key); !errors.Is(err, errWriteRefused) {
				t.Errorf("SendKeys = %v, want errWriteRefused", err)
			}
			if calls := rec.recorded(); len(calls) != 0 {
				t.Errorf("the writer ran %v", calls)
			}
		})
	}
	t.Run("a constructed key is pressed", func(t *testing.T) {
		rec := newRecorder(t)
		rec.onKeys = func(string, string) error { return nil }
		w, err := NewPaneWriter(rec.commands(nil), chiefPane, "wc")
		if err != nil {
			t.Fatal(err)
		}
		if err := w.SendKeys(ctx, task, relayKey); err != nil {
			t.Fatal(err)
		}
		calls := rec.recorded()
		if len(calls) != 1 || calls[0].line() != "herdr agent send-keys w1:p2 2" {
			t.Errorf("calls = %v, want one key press", calls)
		}
	})
	t.Run("a constructed prompt is submitted", func(t *testing.T) {
		rec := newRecorder(t)
		rec.onPrompt = func(string, string) error { return nil }
		w, err := NewPaneWriter(rec.commands(nil), chiefPane, "wc")
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Prompt(ctx, task, clearPrompt); err != nil {
			t.Fatal(err)
		}
		calls := rec.recorded()
		if len(calls) != 1 || calls[0].line() != "herdr agent prompt w1:p2 /clear" {
			t.Errorf("calls = %v, want one prompt without --wait", calls)
		}
	})
	t.Run("a writer needs chief's pane and workspace", func(t *testing.T) {
		for _, args := range []struct {
			pane PaneID
			ws   string
		}{{PaneID{}, "wc"}, {chiefPane, ""}, {chiefPane, "-x"}} {
			if _, err := NewPaneWriter(Commands{}, args.pane, args.ws); !errors.Is(err, errWriteRefused) {
				t.Errorf("NewPaneWriter(%v, %q) = %v, want errWriteRefused", args.pane, args.ws, err)
			}
		}
	})
}

// TestReadHerdrCreationSamples reads the real outputs of the Herdr commands
// that make and close a task's workspace, taken in task 0092's field check.
func TestReadHerdrCreationSamples(t *testing.T) {
	const (
		dir        = "/Users/user/.herdr/worktrees/yt2column/probe-0092"
		sessionDir = "/Users/user/.herdr/worktrees/yt2column/chief-task-worktree"
	)
	t.Run("workspace create", func(t *testing.T) {
		got, err := parseWorkspaceCreate(readTestdata(t, "herdr/workspace-create.json"))
		if err != nil {
			t.Fatal(err)
		}
		want := createdWorkspace{Label: "probe-0092", First: createdTab{Workspace: "w1Y", Tab: tabID{id: "w1Y:t1"}, Pane: PaneID{id: "w1Y:p1"}, Dir: dir}}
		if got != want {
			t.Errorf("workspace = %+v, want %+v", got, want)
		}
	})
	t.Run("tab create", func(t *testing.T) {
		got, err := parseTabCreate(readTestdata(t, "herdr/tab-create.json"), "w1Y")
		if err != nil {
			t.Fatal(err)
		}
		want := createdTab{Workspace: "w1Y", Tab: tabID{id: "w1Y:t2"}, Pane: PaneID{id: "w1Y:p2"}, Dir: dir}
		if got != want {
			t.Errorf("tab = %+v, want %+v", got, want)
		}
	})
	t.Run("tab rename", func(t *testing.T) {
		got, err := parseTabRename(readTestdata(t, "herdr/tab-rename.json"))
		if err != nil {
			t.Fatal(err)
		}
		if want := (tabInfo{Workspace: "w1Y", Tab: tabID{id: "w1Y:t1"}, Label: "cmd"}); got != want {
			t.Errorf("tab = %+v, want %+v", got, want)
		}
	})
	starts := map[string]struct {
		name    string
		kind    paneinput.AgentKind
		pane    string
		session bool
	}{
		"agent-start-claude.json":   {"probe-0092-claude", paneinput.AgentClaudeCode, "w1Y:p3", true},
		"agent-start-opencode.json": {"probe-0092-opencode", paneinput.AgentOpenCode, "w1Y:p2", false},
	}
	for file, want := range starts {
		t.Run(file, func(t *testing.T) {
			got, err := parseAgentStart(readTestdata(t, "herdr/"+file))
			if err != nil {
				t.Fatal(err)
			}
			a := got.Agent
			if got.Name != want.name || a.Kind != want.kind || a.Pane.String() != want.pane || a.Status != AgentIdle || a.Dir != dir || (a.Session != "") != want.session {
				t.Errorf("started = %+v, want name %q kind %d pane %s idle in %s, session %v", got, want.name, want.kind, want.pane, dir, want.session)
			}
		})
	}
	t.Run("agent get right after start", func(t *testing.T) {
		a, err := parseAgentGet(readTestdata(t, "herdr/agent-get-claude-started.json"))
		if err != nil {
			t.Fatal(err)
		}
		if a.Kind != paneinput.AgentClaudeCode || a.Status != AgentIdle || a.Session == "" || a.Dir != dir {
			t.Errorf("agent = %+v, want an idle Claude Code agent with a session in %s", a, dir)
		}
	})
	lists := map[string][]listedPane{
		"pane-list-shells.json": {
			{ID: PaneID{id: "w1Y:p1"}, Workspace: "w1Y", Tab: tabID{id: "w1Y:t1"}, Dir: dir},
			{ID: PaneID{id: "w1Y:p2"}, Workspace: "w1Y", Tab: tabID{id: "w1Y:t2"}, Dir: dir},
			{ID: PaneID{id: "w1Y:p3"}, Workspace: "w1Y", Tab: tabID{id: "w1Y:t3"}, Dir: dir},
		},
		"pane-list-agents.json": {
			{ID: PaneID{id: "w1S:p1"}, Workspace: "w1S", Tab: tabID{id: "w1S:t1"}, Dir: sessionDir},
			{ID: PaneID{id: "w1S:p2"}, Workspace: "w1S", Tab: tabID{id: "w1S:t2"}, Dir: sessionDir, HasAgent: true, Kind: paneinput.AgentOpenCode},
			{ID: PaneID{id: "w1S:p3"}, Workspace: "w1S", Tab: tabID{id: "w1S:t3"}, Dir: sessionDir, HasAgent: true, Kind: paneinput.AgentClaudeCode},
		},
	}
	for file, want := range lists {
		t.Run(file, func(t *testing.T) {
			panes, err := parsePaneList(readTestdata(t, "herdr/"+file))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(panes, want) {
				t.Errorf("panes = %+v, want %+v", panes, want)
			}
		})
	}
	t.Run("pane of an agent chief does not route to", func(t *testing.T) {
		sample := strings.Replace(string(readTestdata(t, "herdr/pane-list-agents.json")), `"agent":"opencode",`, `"agent":"codex",`, 1)
		panes, err := parsePaneList([]byte(sample))
		if err != nil {
			t.Fatal(err)
		}
		if !panes[1].HasAgent || panes[1].Kind != paneinput.AgentUnknown {
			t.Errorf("pane 1 = %+v, want an agent of no kind", panes[1])
		}
	})
	infos := map[string]bool{
		"pane-process-info-shell.json": true,
		"pane-process-info-busy.json":  false,
	}
	for file, shell := range infos {
		t.Run(file, func(t *testing.T) {
			info, err := parsePaneProcessInfo(readTestdata(t, "herdr/"+file))
			if err != nil {
				t.Fatal(err)
			}
			if info.Pane.String() != "w1Y:p1" || info.ShellInForeground() != shell {
				t.Errorf("info = %+v, ShellInForeground = %v, want %v", info, info.ShellInForeground(), shell)
			}
		})
	}
	t.Run("workspace close", func(t *testing.T) {
		if err := parseWorkspaceClose(readTestdata(t, "herdr/workspace-close.json")); err != nil {
			t.Error(err)
		}
	})
	errs := map[string]struct{ id, code string }{
		"agent-start-name-taken.stderr":    {"cli:agent:start", "agent_name_taken"},
		"agent-start-pane-busy.stderr":     {"cli:agent:start", "agent_pane_busy"},
		"agent-start-timeout.stderr":       {"cli:agent:start", "timeout"},
		"workspace-close-not-found.stderr": {"cli:workspace:close", "workspace_not_found"},
	}
	for file, want := range errs {
		t.Run(file, func(t *testing.T) {
			got, err := parseHerdrError(readTestdata(t, "herdr/"+file), want.id)
			if err != nil {
				t.Fatal(err)
			}
			if got.Code != want.code || got.Message == "" {
				t.Errorf("error = %+v, want code %q with a message", got, want.code)
			}
		})
	}
}

// TestShellInForeground checks the shell-at-its-prompt rule on process
// information the samples did not show: each case breaks one of its
// conditions.
func TestShellInForeground(t *testing.T) {
	cases := map[string]struct {
		info processInfo
		want bool
	}{
		"shell alone":                {processInfo{ShellPID: 10, ForegroundGroup: 10, ForegroundPIDs: []int64{10}}, true},
		"another group":              {processInfo{ShellPID: 10, ForegroundGroup: 11, ForegroundPIDs: []int64{10}}, false},
		"shell group with a child":   {processInfo{ShellPID: 10, ForegroundGroup: 10, ForegroundPIDs: []int64{10, 12}}, false},
		"shell group, other process": {processInfo{ShellPID: 10, ForegroundGroup: 10, ForegroundPIDs: []int64{12}}, false},
		"no process":                 {processInfo{ShellPID: 10, ForegroundGroup: 10}, false},
		"zero value":                 {processInfo{}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := c.info.ShellInForeground(); got != c.want {
				t.Errorf("ShellInForeground() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestHerdrRejectsUnknownCreationShapes edits the real samples of the
// creation and removal commands into shapes the samples did not show and
// checks each is unreadable rather than defaulted.
func TestHerdrRejectsUnknownCreationShapes(t *testing.T) {
	parsers := map[string]func([]byte) error{
		"workspace-create.json": func(b []byte) error { _, err := parseWorkspaceCreate(b); return err },
		"tab-create.json":       func(b []byte) error { _, err := parseTabCreate(b, "w1Y"); return err },
		"tab-rename.json":       func(b []byte) error { _, err := parseTabRename(b); return err },
		"agent-start-claude.json": func(b []byte) error {
			_, err := parseAgentStart(b)
			return err
		},
		"pane-list-agents.json":        func(b []byte) error { _, err := parsePaneList(b); return err },
		"pane-process-info-shell.json": func(b []byte) error { _, err := parsePaneProcessInfo(b); return err },
		"workspace-close.json":         parseWorkspaceClose,
		"agent-start-timeout.stderr": func(b []byte) error {
			_, err := parseHerdrError(b, "cli:agent:start")
			return err
		},
	}
	edits := []struct {
		name, file, from, to string
	}{
		{"workspace of another tab", "workspace-create.json", `"tab_id":"w1Y:t1","workspace_id":"w1Y"},"type"`, `"tab_id":"w1Y:t1","workspace_id":"w1X"},"type"`},
		{"pane of another workspace", "workspace-create.json", `"terminal_id":"term_6606c93acb11142","workspace_id":"w1Y"`, `"terminal_id":"term_6606c93acb11142","workspace_id":"w1X"`},
		{"pane of another tab", "workspace-create.json", `"tab_id":"w1Y:t1","terminal_id"`, `"tab_id":"w1Y:t9","terminal_id"`},
		{"missing workspace label", "workspace-create.json", `"label":"probe-0092",`, ``},
		{"duplicate workspace", "workspace-create.json", `"type":"workspace_created",`, `"type":"workspace_created","workspace":{},`},
		{"tab of another workspace", "tab-create.json", `"tab_id":"w1Y:t2","workspace_id":"w1Y"},"type"`, `"tab_id":"w1Y:t2","workspace_id":"w1X"},"type"`},
		{"pane of another workspace", "tab-create.json", `"terminal_id":"term_6606c93f0ceaf43","workspace_id":"w1Y"`, `"terminal_id":"term_6606c93f0ceaf43","workspace_id":"w1X"`},
		{"duplicate root pane", "tab-create.json", `"type":"tab_created"`, `"type":"tab_created","root_pane":null`},
		{"relative cwd", "tab-create.json", `"cwd":"/Users/user/.herdr/worktrees/yt2column/probe-0092"`, `"cwd":"probe-0092"`},
		{"another envelope", "tab-create.json", `"cli:tab:create"`, `"cli:workspace:create"`},
		{"malformed tab", "tab-rename.json", `"tab_id":"w1Y:t1"`, `"tab_id":"w1Y"`},
		{"another result type", "tab-rename.json", `"type":"tab_info"`, `"type":"tab_created"`},
		{"malformed workspace", "tab-rename.json", `"workspace_id":"w1Y"`, `"workspace_id":"w1Y:t1"`},
		{"option-like pane", "tab-create.json", `"pane_id":"w1Y:p2"`, `"pane_id":"--pane"`},
		{"option-like pane", "pane-list-agents.json", `"pane_id":"w1S:p2"`, `"pane_id":"--pane"`},
		{"malformed tab", "pane-list-agents.json", `"tab_id":"w1S:t2"`, `"tab_id":"w1S"`},
		{"malformed workspace", "pane-list-agents.json", `"tab_id":"w1S:t1","terminal_id":"term_6606008f2268731","workspace_id":"w1S"`, `"tab_id":"w1S:t1","terminal_id":"term_6606008f2268731","workspace_id":"w1S:p1"`},
		{"relative cwd", "pane-list-agents.json", `"agent_status":"unknown","cwd":"/Users/user/.herdr/worktrees/yt2column/chief-task-worktree"`, `"agent_status":"unknown","cwd":"chief-task-worktree"`},
		{"option-like pane", "pane-process-info-shell.json", `"pane_id":"w1Y:p1"`, `"pane_id":"--x"`},
		{"missing name", "agent-start-claude.json", `"name":"probe-0092-claude",`, ``},
		{"unknown status", "agent-start-claude.json", `"agent_status":"idle"`, `"agent_status":"starting"`},
		{"another result type", "agent-start-claude.json", `"type":"agent_started"`, `"type":"agent_info"`},
		{"empty agent name", "pane-list-agents.json", `"agent":"opencode",`, `"agent":"",`},
		{"null agent", "pane-list-agents.json", `"agent":"opencode",`, `"agent":null,`},
		{"missing cwd", "pane-list-agents.json", `"agent_status":"unknown","cwd":"/Users/user/.herdr/worktrees/yt2column/chief-task-worktree",`, `"agent_status":"unknown",`},
		{"duplicate workspace", "pane-list-agents.json", `"workspace_id":"w1S"}`, `"workspace_id":"w1S","workspace_id":"w1T"}`},
		{"missing tab", "pane-list-agents.json", `"tab_id":"w1S:t1",`, ``},
		{"zero shell pid", "pane-process-info-shell.json", `"shell_pid":64286`, `"shell_pid":0`},
		{"fractional group", "pane-process-info-shell.json", `"foreground_process_group_id":64286`, `"foreground_process_group_id":64286.5`},
		{"process without pid", "pane-process-info-shell.json", `"name":"zsh","pid":64286`, `"name":"zsh"`},
		{"processes not an array", "pane-process-info-shell.json", `"foreground_processes":[`, `"foreground_processes":{"x":`},
		{"another result type", "workspace-close.json", `"type":"ok"`, `"type":"workspace_closed"`},
		{"another envelope", "workspace-close.json", `"cli:workspace:close"`, `"cli:workspace:list"`},
		{"another command's error", "agent-start-timeout.stderr", `"cli:agent:start"`, `"cli:agent:prompt"`},
		{"code of another shape", "agent-start-timeout.stderr", `"code":"timeout"`, `"code":"Timeout Error"`},
		{"missing message", "agent-start-timeout.stderr", `,"message":"timed out waiting for agent startup"`, ``},
		{"duplicate code", "agent-start-timeout.stderr", `"code":"timeout"`, `"code":"timeout","code":"timeout"`},
	}
	t.Run("workspace-create.json/option-like workspace of its tab and pane too", func(t *testing.T) {
		sample := string(readTestdata(t, "herdr/workspace-create.json"))
		if strings.Count(sample, `"workspace_id":"w1Y"`) != 3 {
			t.Fatal("the sample does not name the workspace in the workspace, its tab, and its pane")
		}
		edited := strings.ReplaceAll(sample, `"workspace_id":"w1Y"`, `"workspace_id":"--group"`)
		if _, err := parseWorkspaceCreate([]byte(edited)); !errors.Is(err, errHerdrOutput) {
			t.Errorf("error = %v, want errHerdrOutput", err)
		}
	})
	t.Run("tab-create.json/option-like tab of its pane too", func(t *testing.T) {
		sample := string(readTestdata(t, "herdr/tab-create.json"))
		edited := strings.ReplaceAll(sample, `"tab_id":"w1Y:t2"`, `"tab_id":"--tab"`)
		if strings.Count(sample, `"tab_id":"w1Y:t2"`) != 2 {
			t.Fatal("the sample does not name the tab in both the tab and its pane")
		}
		if _, err := parseTabCreate([]byte(edited), "w1Y"); !errors.Is(err, errHerdrOutput) {
			t.Errorf("error = %v, want errHerdrOutput", err)
		}
	})
	for _, e := range edits {
		t.Run(e.file+"/"+e.name, func(t *testing.T) {
			sample := string(readTestdata(t, "herdr/"+e.file))
			if err := parsers[e.file]([]byte(sample)); err != nil {
				t.Fatalf("the unedited sample is unreadable: %v", err)
			}
			edited := strings.Replace(sample, e.from, e.to, 1)
			if edited == sample {
				t.Fatalf("the sample has no %q", e.from)
			}
			if err := parsers[e.file]([]byte(edited)); !errors.Is(err, errHerdrOutput) {
				t.Errorf("error = %v, want errHerdrOutput", err)
			}
		})
	}
}
