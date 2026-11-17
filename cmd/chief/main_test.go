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

// noSleep is the wait of the tests: it never waits.
func noSleep(context.Context, time.Duration) error { return nil }

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
	// paneDir, when set, is the working directory Herdr reports for every
	// agent, and git reports each directory as its own worktree, so the
	// pane's worktree differs from the process's.
	paneDir string
}

// agent returns the Herdr JSON of the agent in pane with session; the
// sessionless fake reports no session for the first pane.
func (f *fakeHerdr) agent(pane, session string) string {
	sessionJSON := fmt.Sprintf(`"agent_session":{"agent":"claude","kind":"id","source":"herdr:claude","value":%q},`, session)
	if f.sessionless && pane == testPane {
		sessionJSON = ""
	}
	dir := f.dir
	if f.paneDir != "" {
		dir = f.paneDir
	}
	return fmt.Sprintf(`{"agent":"claude",%s"agent_status":"working","cwd":%q,"pane_id":%q,"state_change_seq":1,"workspace_id":%q}`, sessionJSON, dir, pane, strings.Split(pane, ":")[0])
}

// sessions maps each pane the fake Herdr lists to its session.
var sessions = map[string]string{testPane: testSession, otherPane: otherSession}

func (f *fakeHerdr) factory(o cmdrunner.Options) cmdrunner.Runner {
	if f.paneDir != "" {
		return dirRunner{f: f, dir: o.Dir}
	}
	return f
}

// dirRunner answers git's worktree question with the directory it runs in
// and leaves every other command to f.
type dirRunner struct {
	f   *fakeHerdr
	dir string
}

func (d dirRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "git" && slices.Contains(args, "--show-toplevel") {
		return []byte(d.dir + "\n"), nil
	}
	return d.f.Run(ctx, name, args...)
}

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
		agents := f.agent(testPane, sessions[testPane])
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
	contextDoc, err := os.ReadFile(filepath.Join("..", "..", ".claude", "commands", "_context.md"))
	if err != nil {
		t.Fatal(err)
	}
	return &harness{
		t:     t,
		fake:  &fakeHerdr{t: t, dir: dir, context: contextDoc},
		env:   chief.Env{PaneID: testPane, WorkDir: dir, XDGStateHome: filepath.Join(root, "state"), Home: root},
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
	code := run(context.Background(), args, h.env, nil, h.fake.factory, now, noSleep, &stdout, &stderr)
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
	t.Run("reset before start", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.run("reset", "--workspace", "/w", "--check", testCheck, "--protocol", "1")
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
	t.Run("a pane of another worktree", func(t *testing.T) {
		h := newHarness(t)
		h.fake.paneDir = t.TempDir()
		code, out := h.run("start", "--protocol", "1")
		if code != exitRefused || out["reason"] != "pane_mismatch" {
			t.Errorf("run = %d, %v; want refused, pane_mismatch", code, out)
		}
	})
	t.Run("a working directory that cannot be read", func(t *testing.T) {
		h := newHarness(t)
		h.env.WorkDir = ""
		code, out := h.run("start", "--protocol", "1")
		if code != exitFailure || out["outcome"] != "failed" {
			t.Errorf("run = %d, %v; want failed, not a refusal", code, out)
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
		if code, out := h.run("reset", "--workspace", "/w", "--check", testCheck, "--protocol", "1"); code != exitRefused || out["reason"] != "not_owner" {
			t.Errorf("reset by another chief = %d, %v; want refused, not_owner", code, out)
		}
	})
}

// testCheck has the shape of a check value and is no question's.
const testCheck = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// testBaseline is a value of the form `chief wait --since` takes.
const testBaseline = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"start"},
		{"start", "--protocol"},
		{"start", "--protocol", "1", "extra"},
		{"snapshot", "1", "--protocol"},
		{"weakreview", "--protocol", "1"},
		{"tick", "--since", testBaseline, "--protocol", "1"},
		{"wait", "--protocol", "1"},
		{"wait", "--since", "not-a-baseline", "--protocol", "1"},
		{"wait", "--since", testBaseline, "--timeout", "0", "--protocol", "1"},
		{"wait", "--since", testBaseline, "--timeout", "3601", "--protocol", "1"},
		{"wait", "--since", testBaseline, "--timeout", "07", "--protocol", "1"},
		{"wait", "--since", testBaseline, "--timeout", "9223372036854775807", "--protocol", "1"},
		{"--protocol", "1", "start"},
		{"start", "--protocol", "1", "--protocol", "1"},
		{"snapshot", "--workspace", "/w", "--protocol", "1"},
		{"send", "--workspace", "/w", "--protocol", "1"},
		{"send", "--command", "mkarch", "--protocol", "1"},
		{"send", "--workspace", "/w", "--command", "weakreview", "--protocol", "1"},
		{"detail", "--protocol", "1"},
		{"detail", "--workspace", "/w", "--command", "runpr", "--protocol", "1"},
		{"release", "--protocol", "1"},
		{"relay", "--workspace", "/w", "--answer", "1=1", "--protocol", "1"},
		{"relay", "--workspace", "/w", "--check", testCheck, "--answer", "1=1", "--answer", "1=2", "--protocol", "1"},
		{"relay", "--workspace", "/w", "--check", "not-a-check", "--answer", "1=1", "--protocol", "1"},
		{"relay", "--workspace", "/w", "--check", testCheck, "--answer", "2=1", "--protocol", "1"},
		{"relay", "--workspace", "/w", "--check", testCheck, "--answer", "1=0", "--protocol", "1"},
		{"relay", "--workspace", "w", "--check", testCheck, "--answer", "1=1", "--protocol", "1"},
		{"reset", "--workspace", "/w", "--protocol", "1"},
		{"reset", "--workspace", "/w", "--check", "not-a-check", "--protocol", "1"},
		{"reset", "--workspace", "w", "--check", testCheck, "--protocol", "1"},
		{"reset", "--workspace", "/w", "--check", testCheck, "--answer", "1=1", "--protocol", "1"},
	} {
		h := newHarness(t)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run(context.Background(), args, h.env, nil, h.fake.factory, time.Now, noSleep, &stdout, &stderr)
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

// TestStartThenTickAndWait runs tick and wait after start: the tick prints
// the snapshot's members with its automatic sends and a baseline, and a
// wait on that baseline with nothing changing ends at its time limit with
// exit 1 and the outcome "timeout".
func TestStartThenTickAndWait(t *testing.T) {
	h := newHarness(t)
	if code, out := h.run("start", "--protocol", "1"); code != exitOK {
		t.Fatalf("start = %d, %v", code, out)
	}
	code, out := h.run("tick", "--protocol", "1")
	if code != exitOK {
		t.Fatalf("tick = %d, %v", code, out)
	}
	for _, key := range []string{"rows", "pending", "new_notices", "problems", "auto_sends"} {
		if list, ok := out[key].([]any); !ok || len(list) != 0 {
			t.Errorf("%s = %v, want an empty list and not null", key, out[key])
		}
	}
	baseline, ok := out["baseline"].(string)
	if !ok || baseline == "" {
		t.Fatalf("baseline = %v", out["baseline"])
	}
	clock := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	sleeps := 0
	sleep := func(_ context.Context, d time.Duration) error {
		sleeps++
		clock = clock.Add(d)
		return nil
	}
	var stdout, stderr bytes.Buffer
	code = run(context.Background(), []string{"wait", "--since", baseline, "--timeout", "6", "--protocol", "1"}, h.env, nil, h.fake.factory, func() time.Time { return clock }, sleep, &stdout, &stderr)
	var res map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("wait output %q: %v", stdout.String(), err)
	}
	if code != exitFailure || res["outcome"] != "timeout" || sleeps != 2 {
		t.Errorf("wait = %d, %v after %d sleeps; want exit 1, timeout, after 2", code, res, sleeps)
	}
}

// TestWaitRefusedForANewSession checks that a wait whose pane gets another
// session while it runs (a cleared chief session left it running) exits 3
// with not_owner, like any subcommand its caller does not own.
func TestWaitRefusedForANewSession(t *testing.T) {
	h := newHarness(t)
	if code, out := h.run("start", "--protocol", "1"); code != exitOK {
		t.Fatalf("start = %d, %v", code, out)
	}
	_, out := h.run("snapshot", "--protocol", "1")
	baseline, _ := out["baseline"].(string)
	clock := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { sessions[testPane] = testSession })
	sleep := func(_ context.Context, d time.Duration) error {
		clock = clock.Add(d)
		sessions[testPane] = otherSession
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"wait", "--since", baseline, "--timeout", "60", "--protocol", "1"}, h.env, nil, h.fake.factory, func() time.Time { return clock }, sleep, &stdout, &stderr)
	var res map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("wait output %q: %v", stdout.String(), err)
	}
	if code != exitRefused || res["reason"] != "not_owner" {
		t.Errorf("wait = %d, %v; want exit 3, not_owner", code, res)
	}
}

// noWorkspace reports whether out is one action not sent for want of a task
// workspace.
func noWorkspace(out map[string]any) bool {
	actions, ok := out["actions"].([]any)
	if !ok || len(actions) != 1 {
		return false
	}
	a, ok := actions[0].(map[string]any)
	return ok && a["outcome"] == "not_sent" && a["reason"] == "no_workspace"
}

// TestRunWorkspaceSubcommands runs send, relay, reset, detail, and release after start
// with a --workspace that is no task workspace: each finishes (exit 0) and
// says so, and nothing is typed into a pane (the fake fails on any command it
// does not answer, prompt among them).
func TestRunWorkspaceSubcommands(t *testing.T) {
	cases := []struct {
		args  []string
		check func(map[string]any) bool
	}{
		{[]string{"send", "--workspace", "/nowhere", "--command", "mkarch", "--protocol", "1"}, noWorkspace},
		{[]string{"relay", "--workspace", "/nowhere", "--check", testCheck, "--answer", "1=1", "--protocol", "1"}, noWorkspace},
		{[]string{"reset", "--workspace", "/nowhere", "--check", testCheck, "--protocol", "1"}, noWorkspace},
		{[]string{"detail", "--workspace", "/nowhere", "--protocol", "1"}, func(out map[string]any) bool {
			p, ok := out["problem"].(string)
			return ok && p != "" && out["pr"] == nil
		}},
		{[]string{"release", "--workspace", "/nowhere", "--protocol", "1"}, func(out map[string]any) bool {
			p, ok := out["problem"].(string)
			return out["released"] == false && ok && p != ""
		}},
	}
	for _, c := range cases {
		t.Run(c.args[0], func(t *testing.T) {
			h := newHarness(t)
			if code, out := h.run("start", "--protocol", "1"); code != exitOK {
				t.Fatalf("start = %d, %v", code, out)
			}
			code, out := h.run(c.args...)
			if code != exitOK || !c.check(out) {
				t.Errorf("run(%v) = %d, %v", c.args, code, out)
			}
		})
	}
	t.Run("before start", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.run("release", "--workspace", "/nowhere", "--protocol", "1")
		if code != exitRefused || out["reason"] != "not_started" {
			t.Errorf("release before start = %d, %v; want refused, not_started", code, out)
		}
	})
}
