//go:build test

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief"
	"github.com/isseis/yt2column/internal/cmdrunner"
)

const (
	testPane     = "wc:p1"
	testSession  = "00000000-0000-4000-8000-0000000000c1"
	otherPane    = "wd:p2"
	otherSession = "00000000-0000-4000-8000-0000000000d2"
)

// fakeHerdr is a runner factory that answers the few commands a start and an
// empty snapshot run, and counts every command it is asked to run.
type fakeHerdr struct {
	t       *testing.T
	dir     string
	context []byte
	calls   []string
	// both lists the second chief pane too; otherwise only the first.
	both bool
	// sessionless makes the chief pane report no agent session.
	sessionless bool
}

// agent returns the Herdr JSON of the agent in pane with session; the
// sessionless fake reports no session for the first pane.
func (f *fakeHerdr) agent(pane, session string) string {
	sessionJSON := fmt.Sprintf(`"agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":%q},`, session)
	if f.sessionless && pane == testPane {
		sessionJSON = ""
	}
	return fmt.Sprintf(`{"agent":"claude",%s"agent_status":"working","cwd":%q,"pane_id":%q,"state_change_seq":1,"workspace_id":%q}`, sessionJSON, f.dir, pane, strings.Split(pane, ":")[0])
}

// sessions maps each pane the fake Herdr lists to its session.
var sessions = map[string]string{testPane: testSession, otherPane: otherSession}

func (f *fakeHerdr) factory(cmdrunner.Options) cmdrunner.Runner { return f }

func (f *fakeHerdr) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	has := func(words ...string) bool {
		for _, w := range words {
			if !slices.Contains(args, w) {
				return false
			}
		}
		return true
	}
	switch {
	case name == "herdr" && has("agent", "get"):
		pane := args[len(args)-1]
		return []byte(`{"id":"cli:agent:get","result":{"agent":` + f.agent(pane, sessions[pane]) + `,"type":"agent_info"}}`), nil
	case name == "herdr" && has("agent", "list"):
		agents := f.agent(testPane, testSession)
		if f.both {
			agents += "," + f.agent(otherPane, otherSession)
		}
		return []byte(`{"id":"cli:agent:list","result":{"agents":[` + agents + `],"type":"agent_list"}}`), nil
	case name == "herdr" && has("workspace", "list"):
		return []byte(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[{"label":"chief","workspace_id":"wc"}]}}`), nil
	case name == "git" && has("--show-toplevel"):
		return []byte(f.dir + "\n"), nil
	case name == "git" && has("--git-common-dir"):
		return []byte(filepath.Join(f.dir, ".git") + "\n"), nil
	case name == "git" && has("fetch"):
		return nil, nil
	case name == "git" && has("show"):
		return f.context, nil
	}
	f.t.Errorf("unexpected command %s %v", name, args)
	return nil, errors.New("unexpected command")
}

type harness struct {
	t     *testing.T
	fake  *fakeHerdr
	env   chief.Env
	state string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "chief")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	context, err := os.ReadFile(filepath.Join("..", "..", ".claude", "commands", "_context.md"))
	if err != nil {
		t.Fatal(err)
	}
	return &harness{
		t:     t,
		fake:  &fakeHerdr{t: t, dir: dir, context: context},
		env:   chief.Env{PaneID: testPane, XDGStateHome: filepath.Join(root, "state"), Home: root},
		state: filepath.Join(root, "state"),
	}
}

// run runs the tool with args and returns its exit code and parsed output.
func (h *harness) run(args ...string) (int, map[string]any) {
	h.t.Helper()
	if err := os.MkdirAll(h.state, 0o700); err != nil {
		h.t.Fatal(err)
	}
	return h.runWithoutBase(args...)
}

func (h *harness) runWithoutBase(args ...string) (int, map[string]any) {
	h.t.Helper()
	var stdout, stderr bytes.Buffer
	now := func() time.Time { return time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC) }
	code := run(context.Background(), args, h.env, nil, h.fake.factory, now, &stdout, &stderr)
	var out map[string]any
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
			h.t.Fatalf("output is not one JSON object: %q (%v)", stdout.String(), err)
		}
	}
	return code, out
}

func TestRunOutsideHerdrReadsNothing(t *testing.T) {
	h := newHarness(t)
	h.env.PaneID = ""
	code, out := h.runWithoutBase("snapshot", "--protocol", "1")
	if code != exitRefused || out["reason"] != "not_in_herdr" {
		t.Fatalf("run = %d, %v; want refused, not_in_herdr", code, out)
	}
	if len(h.fake.calls) != 0 {
		t.Errorf("commands ran outside Herdr: %v", h.fake.calls)
	}
	if _, err := os.Stat(h.state); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the state directory was touched outside Herdr: %v", err)
	}
}

func TestRunRefusals(t *testing.T) {
	t.Run("protocol mismatch", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.run("start", "--protocol", "2")
		if code != exitRefused || out["reason"] != "protocol_mismatch" || len(h.fake.calls) != 0 {
			t.Errorf("run = %d, %v, calls %v", code, out, h.fake.calls)
		}
	})
	t.Run("state base is missing", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.runWithoutBase("start", "--protocol", "1")
		if code != exitRefused || out["reason"] != "no_state_base" {
			t.Errorf("run = %d, %v", code, out)
		}
	})
	t.Run("snapshot before start", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.run("snapshot", "--protocol", "1")
		if code != exitRefused || out["reason"] != "not_started" {
			t.Errorf("run = %d, %v", code, out)
		}
	})
	t.Run("a pane without a session", func(t *testing.T) {
		h := newHarness(t)
		h.fake.sessionless = true
		code, out := h.run("start", "--protocol", "1")
		if code != exitRefused || out["reason"] != "no_session" {
			t.Errorf("run = %d, %v", code, out)
		}
	})
	t.Run("the lock belongs to a live chief", func(t *testing.T) {
		h := newHarness(t)
		if code, out := h.run("start", "--protocol", "1"); code != exitOK {
			t.Fatalf("first start = %d, %v", code, out)
		}
		h.env.PaneID = otherPane
		h.fake.both = true
		if code, out := h.run("start", "--protocol", "1"); code != exitRefused || out["reason"] != "lock_held" {
			t.Errorf("start by another chief = %d, %v; want refused, lock_held", code, out)
		}
		if code, out := h.run("snapshot", "--protocol", "1"); code != exitRefused || out["reason"] != "not_owner" {
			t.Errorf("snapshot by another chief = %d, %v; want refused, not_owner", code, out)
		}
	})
}

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"start"},
		{"start", "--protocol"},
		{"start", "--protocol", "1", "extra"},
		{"snapshot", "1", "--protocol"},
		{"tick", "--protocol", "1"},
		{"--protocol", "1", "start"},
	} {
		h := newHarness(t)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run(context.Background(), args, h.env, nil, h.fake.factory, time.Now, &stdout, &stderr)
		if code != exitUsage || stdout.Len() != 0 || stderr.Len() == 0 || len(h.fake.calls) != 0 {
			t.Errorf("run(%v) = %d, stdout %q, stderr %q, calls %v; want a usage error that reads nothing", args, code, stdout.String(), stderr.String(), h.fake.calls)
		}
	}
}

func TestStartThenSnapshot(t *testing.T) {
	h := newHarness(t)
	code, out := h.run("start", "--protocol", "1")
	if code != exitOK || out["outcome"] != "started" {
		t.Fatalf("start = %d, %v", code, out)
	}
	if code, out := h.run("start", "--protocol", "1"); code != exitOK || out["outcome"] != "started" {
		t.Fatalf("a second start by the owner = %d, %v", code, out)
	}
	code, out = h.run("snapshot", "--protocol", "1")
	if code != exitOK {
		t.Fatalf("snapshot = %d, %v", code, out)
	}
	for _, key := range []string{"protocol", "state_dir", "rows", "pending", "new_notices", "problems"} {
		if _, ok := out[key]; !ok {
			t.Errorf("the snapshot output has no %q: %v", key, out)
		}
	}
	for _, key := range []string{"rows", "pending", "new_notices", "problems"} {
		if list, ok := out[key].([]any); !ok || len(list) != 0 {
			t.Errorf("%s = %v, want an empty list and not null", key, out[key])
		}
	}
	if out["state_dir"] != filepath.Join(h.state, "yt2column", "chief") {
		t.Errorf("state_dir = %v", out["state_dir"])
	}
}
