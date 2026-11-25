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
	"strconv"
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
	// lenient fails a command it does not answer without failing the test.
	lenient bool
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
	ws, _, _ := strings.Cut(pane, ":")
	return fmt.Sprintf(`{"agent":"claude",%s"agent_status":"working","cwd":%q,"pane_id":%q,"state_change_seq":1,"tab_id":%q,"workspace_id":%q}`, sessionJSON, dir, pane, ws+":t1", ws)
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
	case name == "herdr" && has("tab", "list"):
		return []byte(`{"id":"cli:tab:list","result":{"tabs":[{"label":"claude","tab_id":"wc:t1"},{"label":"claude","tab_id":"wd:t1"}],"type":"tab_list"}}`), nil
	case name == "herdr" && has("pane", "layout"):
		pane := args[len(args)-1]
		return []byte(`{"id":"cli:pane:layout","result":{"layout":{"panes":[{"pane_id":"` + pane + `","rect":{"width":100}}],"splits":[]},"type":"pane_layout"}}`), nil
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
	case name == "gh" && has("api", "graphql") && has("n="+openIssue):
		// The answer GitHub gave about an open issue, sampled for
		// internal/chief.
		return os.ReadFile(filepath.Join("..", "..", "internal", "chief", "testdata", "gh", "issue-open-839.json"))
	}
	if !f.lenient {
		f.t.Errorf("unexpected command %s %v", name, args)
	}
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
	code, out := h.runWithoutBase("snapshot", "--protocol", protocol)
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

// protocol is the --protocol value of the installed tool's contract.
var protocol = strconv.Itoa(chief.Protocol)

func TestRunRefusals(t *testing.T) {
	t.Run("protocol mismatch", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.run("start", "--protocol", strconv.Itoa(chief.Protocol+1))
		if code != exitRefused || out["reason"] != "protocol_mismatch" || len(h.fake.calls) != 0 {
			t.Errorf("run = %d, %v, calls %v", code, out, h.fake.calls)
		}
	})
	t.Run("a send of the previous protocol's form", func(t *testing.T) {
		// The previous definition sends a task command without --item: it
		// is told to reinstall, not that its request is malformed.
		h := newHarness(t)
		code, out := h.run("send", "--workspace", "/w", "--command", "mkarch", "--protocol", strconv.Itoa(chief.Protocol-1))
		if code != exitRefused || out["reason"] != "protocol_mismatch" || len(h.fake.calls) != 0 {
			t.Errorf("run = %d, %v, calls %v; want refused, protocol_mismatch", code, out, h.fake.calls)
		}
	})
	t.Run("state base is missing", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.runWithoutBase("start", "--protocol", protocol)
		if code != exitRefused || out["reason"] != "no_state_base" {
			t.Errorf("run = %d, %v", code, out)
		}
	})
	t.Run("snapshot before start", func(t *testing.T) {
		h := newHarness(t)
		code, out := h.run("snapshot", "--protocol", protocol)
		if code != exitRefused || out["reason"] != "not_started" {
			t.Errorf("run = %d, %v", code, out)
		}
	})
	for _, args := range ownerCalls {
		t.Run(args[0]+" before start", func(t *testing.T) {
			h := newHarness(t)
			code, out := h.run(args...)
			if code != exitRefused || out["reason"] != "not_started" {
				t.Errorf("run = %d, %v", code, out)
			}
		})
	}
	t.Run("a pane without a session", func(t *testing.T) {
		h := newHarness(t)
		h.fake.sessionless = true
		code, out := h.run("start", "--protocol", protocol)
		if code != exitRefused || out["reason"] != "no_session" {
			t.Errorf("run = %d, %v", code, out)
		}
	})
	t.Run("a pane of another worktree", func(t *testing.T) {
		h := newHarness(t)
		h.fake.paneDir = t.TempDir()
		code, out := h.run("start", "--protocol", protocol)
		if code != exitRefused || out["reason"] != "pane_mismatch" {
			t.Errorf("run = %d, %v; want refused, pane_mismatch", code, out)
		}
	})
	t.Run("a working directory that cannot be read", func(t *testing.T) {
		h := newHarness(t)
		h.env.WorkDir = ""
		code, out := h.run("start", "--protocol", protocol)
		if code != exitFailure || out["outcome"] != "failed" {
			t.Errorf("run = %d, %v; want failed, not a refusal", code, out)
		}
	})
	t.Run("the lock belongs to a live chief", func(t *testing.T) {
		h := newHarness(t)
		if code, out := h.run("start", "--protocol", protocol); code != exitOK {
			t.Fatalf("first start = %d, %v", code, out)
		}
		h.env.PaneID = otherPane
		h.fake.both = true
		if code, out := h.run("start", "--protocol", protocol); code != exitRefused || out["reason"] != "lock_held" {
			t.Errorf("start by another chief = %d, %v; want refused, lock_held", code, out)
		}
		if code, out := h.run("snapshot", "--protocol", protocol); code != exitRefused || out["reason"] != "not_owner" {
			t.Errorf("snapshot by another chief = %d, %v; want refused, not_owner", code, out)
		}
		for _, args := range ownerCalls {
			if code, out := h.run(args...); code != exitRefused || out["reason"] != "not_owner" {
				t.Errorf("%s by another chief = %d, %v; want refused, not_owner", args[0], code, out)
			}
		}
	})
}

// ownerCalls are calls of some of the subcommands that act only for the
// lock's owner (the three of task 0092 and reset), with arguments that would
// otherwise run them.
var ownerCalls = [][]string{
	{"reset", "--workspace", "/w", "--check", testCheck, "--protocol", protocol},
	{"targets", "--issue", "12", "--protocol", protocol},
	{"create", "--issue", "12", "--protocol", protocol},
	{"remove", "--workspace", "/w", "--protocol", protocol},
}

// TestIssueFormIsNoUsageError checks that targets and create answer an
// --issue of another form than an issue number with exit 0 and issue_form,
// a finished run that tells the human what to type, and not with the usage
// error a mistake of the definition gets. The fake fails the test on any
// command it does not answer, and it answers only reads and git fetch, so
// neither writes to Herdr. The forms a number is refused for are
// internal/chief's tests (TestCreateIssueForm, TestTargets); one value
// shows that cmd/chief passes the text through.
func TestIssueFormIsNoUsageError(t *testing.T) {
	for _, c := range []struct{ subcommand, member string }{{"targets", "refusal"}, {"create", "create_refusal"}} {
		for _, issue := range []string{"#12"} {
			t.Run(c.subcommand+" "+issue, func(t *testing.T) {
				h := newHarness(t)
				if code, out := h.run("start", "--protocol", protocol); code != exitOK {
					t.Fatalf("start = %d, %v", code, out)
				}
				code, out := h.run(c.subcommand, "--issue", issue, "--protocol", protocol)
				if code != exitOK || out[c.member] != "issue_form" {
					t.Errorf("%s --issue %q = %d, %v; want exit 0 with %s issue_form", c.subcommand, issue, code, out, c.member)
				}
			})
		}
	}
}

// openIssue is the number of the open issue the fake GitHub answers about.
const openIssue = "839"

// TestTargetsListsNoWorkspaceAndTypesNothing runs targets with an open
// issue's number when no task workspace offers /mkreq: it finishes with the
// issue, its title, no refusal, and an empty list (not null), and the fake,
// which fails the test on any command it does not answer and answers only
// reads and git fetch, gets no call that writes to Herdr or adds a
// worktree.
func TestTargetsListsNoWorkspaceAndTypesNothing(t *testing.T) {
	h := newHarness(t)
	if code, out := h.run("start", "--protocol", protocol); code != exitOK {
		t.Fatalf("start = %d, %v", code, out)
	}
	code, out := h.run("targets", "--issue", openIssue, "--protocol", protocol)
	if list, ok := out["workspaces"].([]any); code != exitOK || out["issue"] != openIssue || out["title"] != "Sample title of an open issue" || out["refusal"] != "" || !ok || len(list) != 0 {
		t.Errorf("targets = %d, %v; want exit 0, issue %s with its title, no refusal, no workspace", code, out, openIssue)
	}
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
		{"start", "--protocol", protocol, "extra"},
		{"snapshot", "1", "--protocol"},
		{"weakreview", "--protocol", protocol},
		{"tick", "--since", testBaseline, "--protocol", protocol},
		{"wait", "--protocol", protocol},
		{"wait", "--since", "not-a-baseline", "--protocol", protocol},
		{"wait", "--since", testBaseline, "--timeout", "0", "--protocol", protocol},
		{"wait", "--since", testBaseline, "--timeout", "3601", "--protocol", protocol},
		{"wait", "--since", testBaseline, "--timeout", "07", "--protocol", protocol},
		{"wait", "--since", testBaseline, "--timeout", "9223372036854775807", "--protocol", protocol},
		{"--protocol", protocol, "start"},
		{"start", "--protocol", protocol, "--protocol", protocol},
		{"snapshot", "--workspace", "/w", "--protocol", protocol},
		{"send", "--workspace", "/w", "--protocol", protocol},
		{"send", "--command", "mkarch", "--protocol", protocol},
		{"send", "--workspace", "/w", "--command", "runpr", "--pr", "4", "--item", strings.Repeat("c", 32), "--protocol", protocol},
		{"send", "--workspace", "/w", "--command", "weakreview", "--protocol", protocol},
		{"detail", "--protocol", protocol},
		{"detail", "--workspace", "/w", "--command", "runpr", "--protocol", protocol},
		{"release", "--protocol", protocol},
		{"relay", "--workspace", "/w", "--answer", "1=1", "--protocol", protocol},
		{"relay", "--workspace", "/w", "--check", testCheck, "--answer", "1=1", "--answer", "1=2", "--protocol", protocol},
		{"relay", "--workspace", "/w", "--check", "not-a-check", "--answer", "1=1", "--protocol", protocol},
		{"relay", "--workspace", "/w", "--check", testCheck, "--answer", "2=1", "--protocol", protocol},
		{"relay", "--workspace", "/w", "--check", testCheck, "--answer", "1=0", "--protocol", protocol},
		{"relay", "--workspace", "w", "--check", testCheck, "--answer", "1=1", "--protocol", protocol},
		{"reset", "--workspace", "/w", "--protocol", protocol},
		{"reset", "--workspace", "/w", "--check", "not-a-check", "--protocol", protocol},
		{"reset", "--workspace", "w", "--check", testCheck, "--protocol", protocol},
		{"reset", "--workspace", "/w", "--check", testCheck, "--answer", "1=1", "--protocol", protocol},
		{"targets", "--protocol", protocol},
		{"targets", "--issue", "12", "--workspace", "/w", "--protocol", protocol},
		{"create", "--protocol", protocol},
		{"create", "--issue", "12", "--pr", "34", "--protocol", protocol},
		{"remove", "--protocol", protocol},
		{"remove", "--workspace", "/w", "--issue", "12", "--protocol", protocol},
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
	code, out := h.run("start", "--protocol", protocol)
	if code != exitOK || out["outcome"] != "started" {
		t.Fatalf("start = %d, %v", code, out)
	}
	if code, out := h.run("start", "--protocol", protocol); code != exitOK || out["outcome"] != "started" {
		t.Fatalf("a second start by the owner = %d, %v", code, out)
	}
	code, out = h.run("snapshot", "--protocol", protocol)
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
	checkTableMembers(t, out)
}

// checkTableMembers checks the status table members of a snapshot or tick
// output: table is a list of lines that is not empty (a table with no row
// still has its legend and heading), and table_unreadable is a list, not
// null. The layout itself is internal/chief's to test.
func checkTableMembers(t *testing.T, out map[string]any) {
	t.Helper()
	if table, ok := out["table"].([]any); !ok || len(table) == 0 {
		t.Errorf("table = %v, want a list of lines", out["table"])
	}
	if _, ok := out["table_unreadable"].([]any); !ok {
		t.Errorf("table_unreadable = %v, want a list and not null", out["table_unreadable"])
	}
}

// TestStartThenTickAndWait runs tick and wait after start: the tick prints
// the snapshot's members with its automatic sends and a baseline, and a
// wait on that baseline with nothing changing ends at its time limit with
// exit 1 and the outcome "timeout".
func TestStartThenTickAndWait(t *testing.T) {
	h := newHarness(t)
	if code, out := h.run("start", "--protocol", protocol); code != exitOK {
		t.Fatalf("start = %d, %v", code, out)
	}
	code, out := h.run("tick", "--protocol", protocol)
	if code != exitOK {
		t.Fatalf("tick = %d, %v", code, out)
	}
	for _, key := range []string{"rows", "pending", "new_notices", "problems", "auto_sends"} {
		if list, ok := out[key].([]any); !ok || len(list) != 0 {
			t.Errorf("%s = %v, want an empty list and not null", key, out[key])
		}
	}
	checkTableMembers(t, out)
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
	code = run(context.Background(), []string{"wait", "--since", baseline, "--timeout", "6", "--protocol", protocol}, h.env, nil, h.fake.factory, func() time.Time { return clock }, sleep, &stdout, &stderr)
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
	if code, out := h.run("start", "--protocol", protocol); code != exitOK {
		t.Fatalf("start = %d, %v", code, out)
	}
	_, out := h.run("snapshot", "--protocol", protocol)
	baseline, _ := out["baseline"].(string)
	clock := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	t.Cleanup(func() { sessions[testPane] = testSession })
	sleep := func(_ context.Context, d time.Duration) error {
		clock = clock.Add(d)
		sessions[testPane] = otherSession
		return nil
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"wait", "--since", baseline, "--timeout", "60", "--protocol", protocol}, h.env, nil, h.fake.factory, func() time.Time { return clock }, sleep, &stdout, &stderr)
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

// TestRunWorkspaceSubcommands runs send, relay, reset, detail, release, and remove after start
// with a --workspace that is no task workspace: each finishes (exit 0) and
// says so, and nothing is typed into a pane (the fake fails on any command it
// does not answer, prompt among them).
func TestRunWorkspaceSubcommands(t *testing.T) {
	cases := []struct {
		args  []string
		check func(map[string]any) bool
	}{
		{[]string{"send", "--workspace", "/nowhere", "--command", "mkarch", "--item", strings.Repeat("c", 32), "--protocol", protocol}, noWorkspace},
		{[]string{"relay", "--workspace", "/nowhere", "--check", testCheck, "--answer", "1=1", "--protocol", protocol}, noWorkspace},
		{[]string{"reset", "--workspace", "/nowhere", "--check", testCheck, "--protocol", protocol}, noWorkspace},
		{[]string{"detail", "--workspace", "/nowhere", "--protocol", protocol}, func(out map[string]any) bool {
			p, ok := out["problem"].(string)
			return ok && p != "" && out["pr"] == nil
		}},
		{[]string{"release", "--workspace", "/nowhere", "--protocol", protocol}, func(out map[string]any) bool {
			p, ok := out["problem"].(string)
			return out["released"] == false && ok && p != ""
		}},
		{[]string{"remove", "--workspace", "/nowhere", "--protocol", protocol}, func(out map[string]any) bool {
			return out["result"] == "refused" && out["remove_refusal"] == "no_workspace"
		}},
	}
	for _, c := range cases {
		t.Run(c.args[0], func(t *testing.T) {
			h := newHarness(t)
			if code, out := h.run("start", "--protocol", protocol); code != exitOK {
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
		code, out := h.run("release", "--workspace", "/nowhere", "--protocol", protocol)
		if code != exitRefused || out["reason"] != "not_started" {
			t.Errorf("release before start = %d, %v; want refused, not_started", code, out)
		}
	})
}

// mainFake is the fake Herdr, git, and gh of TestRunSendPassesTheOwner: the
// chief workspace of fakeHerdr and a task workspace w1, whose one Claude Code
// pane is idle in tree, on branch with PR 42 in prState. It answers what a
// round and a send read, and fails, without failing the test, every command
// it does not know (a send's checks of the pane among them), so a send that
// passes the check of the chosen item stops there with nothing typed.
type mainFake struct {
	*fakeHerdr
	tree, branch, prState string
	screen                []byte
	docs                  map[string][]byte
}

// errNotFaked is the failure of a command mainFake does not answer.
var errNotFaked = errors.New("not faked")

func (m *mainFake) factory(o cmdrunner.Options) cmdrunner.Runner {
	return mainRunner{m: m, dir: o.Dir}
}

type mainRunner struct {
	m   *mainFake
	dir string
}

func (r mainRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m := r.m
	line := name + " " + strings.Join(args, " ")
	has := func(words ...string) bool {
		for _, w := range words {
			if !slices.Contains(args, w) {
				return false
			}
		}
		return true
	}
	task := m.agent("w1:p1", "00000000-0000-4000-8000-000000000011")
	task = strings.Replace(strings.Replace(task, `"agent_status":"working"`, `"agent_status":"idle"`, 1), `"cwd":"`+m.dir+`"`, `"cwd":"`+m.tree+`"`, 1)
	switch {
	case name == "herdr" && has("agent", "list"):
		m.calls = append(m.calls, line)
		return []byte(`{"id":"cli:agent:list","result":{"agents":[` + m.agent(testPane, sessions[testPane]) + `,` + task + `],"type":"agent_list"}}`), nil
	case name == "herdr" && has("workspace", "list"):
		m.calls = append(m.calls, line)
		return []byte(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[{"label":"chief","workspace_id":"wc"},{"label":"ws","workspace_id":"w1"}]}}`), nil
	case name == "herdr" && has("agent", "read", "w1:p1"):
		m.calls = append(m.calls, line)
		return m.screen, nil
	case r.dir != m.tree:
		return m.Run(ctx, name, args...)
	}
	m.calls = append(m.calls, line)
	switch {
	case name == "git" && has("--show-toplevel"):
		return []byte(m.tree + "\n"), nil
	case name == "git" && has("--git-common-dir"):
		return []byte(filepath.Join(m.dir, ".git") + "\n"), nil
	case name == "git" && has("--abbrev-ref", "HEAD"):
		return []byte(m.branch + "\n"), nil
	case name == "gh" && has("pr", "list"):
		if m.prState == "OPEN" && has(m.branch) {
			return []byte(`[{"headRefName":"` + m.branch + `","number":42,"title":"x","url":"https://github.com/owner/repo/pull/42","mergeable":"MERGEABLE","headRefOid":"1111111111111111111111111111111111111111","baseRefName":"main","isCrossRepository":false}]`), nil
		}
		return []byte("[]"), nil
	case name == "gh" && has("pr", "view", "42"):
		return []byte(`{"headRefName":"user/0001-test-task-02","number":42,"state":"` + m.prState + `"}`), nil
	case name == "git" && has("ls-tree"):
		if has("docs/tasks/") {
			return []byte("docs/tasks/0001_test_task\x00"), nil
		}
		var names []string
		for path := range m.docs {
			names = append(names, path)
		}
		slices.Sort(names)
		return []byte(strings.Join(names, "\x00") + "\x00"), nil
	case name == "git" && has("show"):
		for path, body := range m.docs {
			if has("refs/remotes/origin/main:" + path) {
				return body, nil
			}
		}
	}
	return nil, errNotFaked
}

// TestRunSendPassesTheOwner runs, through the tool, the merge of PR 42 of
// task 0001 read with the worktree on main, then the send of the /mkarch
// 0001 its tick offers: the send reads the started session's history, so
// the row it reads again under the exclusion still offers the chosen item,
// and it gets past that check (it stops at the pane's checks, which the
// fake does not answer); with no owner's session it would find main with
// no task and answer not_offered.
func TestRunSendPassesTheOwner(t *testing.T) {
	h := newHarness(t)
	tree := filepath.Join(filepath.Dir(h.fake.dir), "tree-w1")
	if err := os.MkdirAll(tree, 0o700); err != nil {
		t.Fatal(err)
	}
	screen, err := os.ReadFile(filepath.Join("..", "..", "internal", "chief", "testdata", "screens", "claude-idle.ansi"))
	if err != nil {
		t.Fatal(err)
	}
	status := "| Item | Value |\n|---|---|\n| Status | `approved` |\n| Process | `full` |\n"
	m := &mainFake{
		fakeHerdr: h.fake, tree: tree, branch: "user/0001-test-task-02", prState: "OPEN", screen: screen,
		docs: map[string][]byte{"docs/tasks/0001_test_task/01_requirements.md": []byte("## Document Status\n\n" + status)},
	}
	clock := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	runOne := func(args ...string) map[string]any {
		t.Helper()
		clock = clock.Add(time.Minute)
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), append(args, "--protocol", protocol), h.env, nil, m.factory, func() time.Time { return clock }, noSleep, &stdout, &stderr)
		var out map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &out); err != nil || code != exitOK {
			t.Fatalf("%v = %d, %q (%v)", args, code, stdout.String(), err)
		}
		return out
	}
	if err := os.MkdirAll(h.state, 0o700); err != nil {
		t.Fatal(err)
	}
	runOne("start")
	runOne("tick")
	m.branch, m.prState = "main", "MERGED"
	// From here, the automatic send's reads of the merge and the send's
	// checks of the pane are left unanswered; the start and the first tick
	// are answered in full.
	h.fake.lenient = true
	out := runOne("tick")
	item := ""
	pending, _ := out["pending"].([]any)
	for _, p := range pending {
		p, _ := p.(map[string]any)
		if proposal, ok := p["proposal"].(map[string]any); ok && proposal["command"] == "/mkarch 0001" && p["workspace"] == tree {
			item, _ = p["id"].(string)
		}
	}
	if item == "" {
		t.Fatalf("the tick offers no /mkarch 0001 on main: %v", out)
	}
	sent := runOne("send", "--workspace", tree, "--command", "mkarch", "--item", item)
	actions, _ := sent["actions"].([]any)
	if len(actions) != 1 {
		t.Fatalf("send = %v; want one action", sent)
	}
	action, _ := actions[0].(map[string]any)
	if action["outcome"] != "not_sent" || action["reason"] == "not_offered" || action["command"] != "/mkarch 0001" {
		t.Errorf("send action = %v; want /mkarch 0001 past the check of the item", action)
	}
	if slices.ContainsFunc(m.calls, func(c string) bool { return strings.HasPrefix(c, "herdr agent prompt") }) {
		t.Errorf("typed into a pane: %v", m.calls)
	}
}
