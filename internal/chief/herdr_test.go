//go:build test

package chief

import (
	"context"
	"errors"
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
