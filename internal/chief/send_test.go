//go:build test && unix

package chief

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// openCodeModel is the model the OpenCode samples show.
const openCodeModel = "DeepSeek V4.1 Flash 1017 OpenCode Go"

func claudeRoute(t *testing.T, m paneinput.ClaudeModel) paneinput.Route {
	t.Helper()
	r, err := paneinput.NewClaudeRoute(m)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func openCodeRoute(t *testing.T, name string) paneinput.Route {
	t.Helper()
	m, err := paneinput.ParseOpenCodeModel(name)
	if err != nil {
		t.Fatal(err)
	}
	r, err := paneinput.NewOpenCodeRoute(m)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func runplanCmd(t *testing.T) paneinput.Command {
	t.Helper()
	c, err := paneinput.NewRunplan(taskID(t, "0001"), paneinput.RouteRunplanOpus)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func runprCmd(t *testing.T) paneinput.Command {
	t.Helper()
	pr, err := fixprguard.ParsePRNumber("42")
	if err != nil {
		t.Fatal(err)
	}
	c, err := paneinput.NewRunpr(pr)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// sendContextOf returns the send context of workspace w1 with its exclusion
// held.
func (sw *sendWorld) sendContextOf(deps Deps) sendContext {
	sw.t.Helper()
	const id = "w1"
	tree := sw.trees[id]
	var agents []Agent
	for _, p := range sw.panes {
		if p.Workspace != id {
			continue
		}
		a, err := parseAgentGet(agentGetOutput(p.fakeAgent))
		if err != nil {
			sw.t.Fatal(err)
		}
		agents = append(agents, a)
	}
	ws := TaskWorkspace{ID: id, Worktree: tree, Tree: Worktree{path: tree}, Agents: agents}
	records, err := deps.Store.Records(ws.Tree)
	if err != nil {
		sw.t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		sw.t.Fatal(err)
	}
	sw.t.Cleanup(func() { _ = held.Close() })
	writer, err := NewPaneWriter(deps.Cmds, PaneID{id: worldChiefPane}, "wc")
	if err != nil {
		sw.t.Fatal(err)
	}
	return sendContext{deps: deps, herdr: NewHerdr(deps.Cmds), writer: writer, held: held, ws: ws}
}

// sendRecordNames returns the names of the send and stop records of a held
// workspace.
func sendRecordNames(t *testing.T, h *HeldWorkspace) []string {
	t.Helper()
	entries, err := os.ReadDir(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if sendFileName.MatchString(e.Name()) || e.Name() == stopFile {
			names = append(names, e.Name())
		}
	}
	return names
}

// TestSendNeedsOnePaneOfTheAgent checks that a workspace with no pane, or
// with two panes, of the route's agent gets nothing, and that the result
// lists its panes.
func TestSendNeedsOnePaneOfTheAgent(t *testing.T) {
	for name, kinds := range map[string][]string{
		"no Claude Code pane":   {"opencode"},
		"two Claude Code panes": {"claude", "claude", "opencode"},
	} {
		t.Run(name, func(t *testing.T) {
			sw := newSendWorld(t)
			sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, kinds...)
			deps := sw.finish()
			sc := sw.sendContextOf(deps)
			res := sc.sendCommand(context.Background(), runplanCmd(t), claudeRoute(t, paneinput.ClaudeModelOpus))
			if res.Outcome != OutcomeNotSent || res.Reason != NotSentPaneAmbiguous || len(res.Panes) != len(kinds) {
				t.Errorf("result = %+v, want not sent, pane ambiguous, %d panes listed", res, len(kinds))
			}
			if got := sw.rec.prompts(); len(got) != 0 {
				t.Errorf("typed %v", got)
			}
		})
	}
}

// TestSendChecksBeforeFirstInput checks each check before the first input:
// when one fails nothing is typed, no send record is written, and the result
// says why (with the unsent text for an input line that holds some).
func TestSendChecksBeforeFirstInput(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		route  func(*testing.T) paneinput.Route
		set    func(sw *sendWorld, p *simPane)
		reason NotSentReason
		left   string
	}{
		{"working", "claude", opus, func(_ *sendWorld, p *simPane) { p.Status = "working" }, NotSentNotAccepting, ""},
		{"idle with a question on screen", "claude", opus, func(sw *sendWorld, p *simPane) {
			p.screen = readScreenSample(sw.t, "claude-question-single.ansi")
		}, NotSentNotAccepting, ""},
		{"a screen of no known shape", "claude", opus, func(_ *sendWorld, p *simPane) { p.screen = "no known shape\n" }, NotSentUnreadable, ""},
		{"unsent text", "claude", opus, func(sw *sendWorld, p *simPane) {
			p.screen = readScreenSample(sw.t, "claude-input-left.ansi")
		}, NotSentInputLeft, "unsent draft text"},
		{"no agent session", "claude", opus, func(_ *sendWorld, p *simPane) { p.Session = "" }, NotSentNoTranscript, ""},
		{"no transcript", "claude", opus, func(sw *sendWorld, p *simPane) {
			if err := os.Remove(sw.transcript(p, p.Session)); err != nil {
				sw.t.Fatal(err)
			}
		}, NotSentNoTranscript, ""},
		{"OpenCode unsent text", "opencode", flash, func(sw *sendWorld, p *simPane) {
			p.screen = readScreenSample(sw.t, "opencode-input-left.ansi")
		}, NotSentInputLeft, "unsent draft text"},
		{"OpenCode of another model", "opencode", func(t *testing.T) paneinput.Route { return openCodeRoute(t, "DeepSeek V4.1 Pro 1017 OpenCode Go") }, func(*sendWorld, *simPane) {}, NotSentModelMismatch, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw := newSendWorld(t)
			sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, c.kind)
			deps := sw.finish()
			c.set(sw, sw.firstPane())
			sc := sw.sendContextOf(deps)
			cmd := runplanCmd(t)
			if c.kind == "opencode" {
				cmd = runprCmd(t)
			}
			res := sc.sendCommand(context.Background(), cmd, c.route(t))
			if res.Outcome != OutcomeNotSent || res.Reason != c.reason || res.Left != c.left {
				t.Errorf("result = %+v, want not sent, %s, left %q", res, c.reason, c.left)
			}
			if got := sw.rec.prompts(); len(got) != 0 {
				t.Errorf("typed %v", got)
			}
			if names := sendRecordNames(t, sc.held); len(names) != 0 {
				t.Errorf("records written: %v", names)
			}
		})
	}
}

func opus(t *testing.T) paneinput.Route  { return claudeRoute(t, paneinput.ClaudeModelOpus) }
func flash(t *testing.T) paneinput.Route { return openCodeRoute(t, openCodeModel) }

// TestSendInputsInOrder checks the inputs of a send that succeeds: Claude
// Code gets /clear, /model, and the command; OpenCode gets /new and the
// command; the send's start and end records are written.
func TestSendInputsInOrder(t *testing.T) {
	cases := []struct {
		name  string
		kind  string
		cmd   func(*testing.T) paneinput.Command
		route func(*testing.T) paneinput.Route
		want  []string
	}{
		{"Claude Code", "claude", runplanCmd, opus, []string{"w1:p1 /clear", "w1:p1 /model opus", "w1:p1 /runplan 0001"}},
		{"OpenCode", "opencode", runprCmd, flash, []string{"w1:p1 /new", "w1:p1 /runpr 42"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw := newSendWorld(t)
			sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, c.kind)
			deps := sw.finish()
			sc := sw.sendContextOf(deps)
			res := sc.sendCommand(context.Background(), c.cmd(t), c.route(t))
			if res.Outcome != OutcomeSent || res.Pane != "w1:p1" {
				t.Fatalf("result = %+v, want sent to w1:p1", res)
			}
			if got := sw.rec.prompts(); !slices.Equal(got, c.want) {
				t.Errorf("typed %v, want %v", got, c.want)
			}
			starts, ends, err := sc.held.sendFiles()
			if _, ok := ends[starts[0]]; err != nil || len(starts) != 1 || len(ends) != 1 || !ok {
				t.Errorf("records = %v %v %v, want one start and its end", starts, ends, err)
			}
		})
	}
}

// TestSendStopsBetweenInputs checks a failure after an input: nothing more
// is typed, the workspace is stopped, the send ends as sent-then-failed with
// the last input typed, and the pane's state and screen are returned.
func TestSendStopsBetweenInputs(t *testing.T) {
	cases := []struct {
		name   string
		react  func(sw *sendWorld, p *simPane, text string) error
		typed  int
		step   SentStep
		reason NotSentReason
	}{
		{"text appears after /clear", func(sw *sendWorld, p *simPane, text string) error {
			err := sw.realReact(p, text)
			p.screen = readScreenSample(sw.t, "claude-input-left.ansi")
			return err
		}, 1, StepClear, NotSentInputLeft},
		{"the agent starts working after /clear", func(sw *sendWorld, p *simPane, text string) error {
			err := sw.realReact(p, text)
			p.Status = "working"
			return err
		}, 1, StepClear, NotSentNotAccepting},
		{"the pane moves to another workspace during the wait", func(sw *sendWorld, p *simPane, text string) error {
			err := sw.realReact(p, text)
			p.Workspace = "w9"
			return err
		}, 1, StepClear, NotSentUnreadable},
		{"the session changes after /model", func(sw *sendWorld, p *simPane, text string) error {
			err := sw.realReact(p, text)
			if strings.HasPrefix(text, "/model ") {
				p.Session = sw.newSession()
			}
			return err
		}, 2, StepModel, NotSentPaneChanged},
		{"/clear keeps the session", func(sw *sendWorld, p *simPane, text string) error {
			if text == "/clear" {
				return nil
			}
			return sw.realReact(p, text)
		}, 1, StepClear, NotSentNoStep},
		{"the command does not start", func(sw *sendWorld, p *simPane, text string) error {
			if strings.HasPrefix(text, "/runplan") {
				return nil
			}
			return sw.realReact(p, text)
		}, 3, StepCommand, NotSentNoStep},
		{"herdr fails to type the command", func(sw *sendWorld, p *simPane, text string) error {
			if strings.HasPrefix(text, "/runplan") {
				return errNoReply
			}
			return sw.realReact(p, text)
		}, 3, StepCommand, NotSentUnreadable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw := newSendWorld(t)
			sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, "claude")
			deps := sw.finish()
			sw.react = func(p *simPane, text string) error { return c.react(sw, p, text) }
			var created []string
			deps.Store.beforeCreate = func(name string) error {
				created = append(created, name)
				return nil
			}
			sc := sw.sendContextOf(deps)
			res := sc.sendCommand(context.Background(), runplanCmd(t), opus(t))
			stopAt := slices.Index(created, stopFile)
			endAt := slices.IndexFunc(created, func(n string) bool { return strings.HasSuffix(n, endSuffix) })
			if stopAt < 0 || endAt < 0 || stopAt > endAt {
				t.Errorf("records created %v; want the stop before the end", created)
			}
			if res.Outcome != OutcomeSentThenFailed || res.Step != c.step || res.Reason != c.reason {
				t.Errorf("result = %+v, want sent then failed after %s, %s", res, c.step, c.reason)
			}
			if res.Status == AgentStatusUnknown {
				t.Errorf("the result has no pane status: %+v", res)
			}
			if got := sw.rec.prompts(); len(got) != c.typed {
				t.Errorf("typed %v, want %d inputs", got, c.typed)
			}
			stop, err := sc.held.Stopped()
			if err != nil || stop == nil {
				t.Fatalf("Stopped = %v, %v; want the workspace stopped", stop, err)
			}
			starts, ends, err := sc.held.sendFiles()
			if err != nil || len(starts) != 1 || len(ends) != 1 {
				t.Errorf("send records: %v %v %v; want the start and its end", starts, ends, err)
			}
		})
	}
}

// TestSendFailureAfterInputStops checks that a workspace a failed send
// stopped takes nothing more: a later send, made by a tool built anew on the
// same state directory, types nothing and says the workspace is stopped.
func TestSendFailureAfterInputStops(t *testing.T) {
	sw := newSendWorld(t)
	sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, "claude")
	deps := sw.finish()
	sw.react = func(p *simPane, text string) error {
		if text == "/clear" {
			return nil
		}
		return sw.realReact(p, text)
	}
	req, err := ParseSendRequest(sw.trees["w1"], "runplan", "", "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Send(context.Background(), deps, req)
	if err != nil || len(out.Actions) != 1 || out.Actions[0].Outcome != OutcomeSentThenFailed {
		t.Fatalf("first send = %+v, %v; want sent then failed", out, err)
	}
	sw.react = sw.realReact
	store, err := OpenStore(sw.env.XDGStateHome)
	if err != nil {
		t.Fatal(err)
	}
	deps.Store = store
	typed := len(sw.rec.prompts())
	out, err = Send(context.Background(), deps, req)
	if err != nil || len(out.Actions) != 1 || out.Actions[0].Outcome != OutcomeNotSent || out.Actions[0].Reason != NotSentStopped {
		t.Fatalf("second send = %+v, %v; want not sent, stopped", out, err)
	}
	if got := sw.rec.prompts(); len(got) != typed {
		t.Errorf("the second send typed %v", got[typed:])
	}
}

// TestClaudeModelSwitchCheck checks the confirmation of /model in the new
// session's transcript: the route's model passes, another model, no record,
// and a transcript that cannot be read each stop the send before the
// command.
func TestClaudeModelSwitchCheck(t *testing.T) {
	cases := []struct {
		name   string
		record func(sw *sendWorld, p *simPane) error
		ok     bool
		reason NotSentReason
	}{
		{"the route's model", func(sw *sendWorld, p *simPane) error {
			return sw.writeTranscript(p, p.Session, switchLine("Opus 5.5"))
		}, true, 0},
		{"another model", func(sw *sendWorld, p *simPane) error {
			return sw.writeTranscript(p, p.Session, switchLine("Sonnet 5.5"))
		}, false, NotSentModelMismatch},
		{"a name holding both models", func(sw *sendWorld, p *simPane) error {
			return sw.writeTranscript(p, p.Session, switchLine("Opus Sonnet"))
		}, false, NotSentModelMismatch},
		{"a name holding the model inside a word", func(sw *sendWorld, p *simPane) error {
			return sw.writeTranscript(p, p.Session, switchLine("Opusx 5.5"))
		}, false, NotSentModelMismatch},
		{"no record", func(*sendWorld, *simPane) error { return nil }, false, NotSentNoTranscript},
		{"a line that is not JSON", func(sw *sendWorld, p *simPane) error {
			return sw.writeTranscript(p, p.Session, switchLine("Opus 5.5"), "{")
		}, false, NotSentNoTranscript},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw := newSendWorld(t)
			sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, "claude")
			deps := sw.finish()
			sw.react = func(p *simPane, text string) error {
				if strings.HasPrefix(text, "/model ") {
					p.screen = readScreenSample(sw.t, "claude-after-model.ansi")
					return c.record(sw, p)
				}
				return sw.realReact(p, text)
			}
			sc := sw.sendContextOf(deps)
			res := sc.sendCommand(context.Background(), runplanCmd(t), opus(t))
			typed := sw.rec.prompts()
			if c.ok {
				if res.Outcome != OutcomeSent || len(typed) != 3 {
					t.Errorf("result = %+v, typed %v; want sent", res, typed)
				}
				return
			}
			if res.Outcome != OutcomeSentThenFailed || res.Step != StepModel || res.Reason != c.reason || len(typed) != 2 {
				t.Errorf("result = %+v, typed %v; want sent then failed after /model, %s, no command", res, typed, c.reason)
			}
		})
	}
}

// TestOpenCodeModelMatch checks that OpenCode's model is compared whole: a
// route model that is a prefix of the shown one sends nothing, the exact one
// sends, and a pane that shows another model after /new gets no command.
func TestOpenCodeModelMatch(t *testing.T) {
	t.Run("a prefix of the shown model", func(t *testing.T) {
		sw := newSendWorld(t)
		sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, "opencode")
		sc := sw.sendContextOf(sw.finish())
		res := sc.sendCommand(context.Background(), runprCmd(t), openCodeRoute(t, "DeepSeek V4.1 Flash"))
		if res.Outcome != OutcomeNotSent || res.Reason != NotSentModelMismatch || len(sw.rec.prompts()) != 0 {
			t.Errorf("result = %+v, typed %v; want not sent, model mismatch", res, sw.rec.prompts())
		}
	})
	t.Run("another model after /new", func(t *testing.T) {
		sw := newSendWorld(t)
		sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, "opencode")
		deps := sw.finish()
		sw.react = func(p *simPane, text string) error {
			err := sw.realReact(p, text)
			if text == "/new" {
				p.screen = strings.ReplaceAll(p.screen, "DeepSeek V4.1 Flash 1017", "DeepSeek V4.1 Pro 1017")
			}
			return err
		}
		sc := sw.sendContextOf(deps)
		res := sc.sendCommand(context.Background(), runprCmd(t), flash(t))
		if res.Outcome != OutcomeSentThenFailed || res.Step != StepNew || res.Reason != NotSentModelMismatch || len(sw.rec.prompts()) != 1 {
			t.Errorf("result = %+v, typed %v; want sent then failed after /new, model mismatch", res, sw.rec.prompts())
		}
	})
}

// TestTranscriptPath checks where a transcript is looked for, and that a
// place the sampled rule does not decide is an error.
func TestTranscriptPath(t *testing.T) {
	session := sessionPrefix + "01"
	cases := []struct {
		env     Env
		dir     string
		session string
		want    string
	}{
		{Env{Home: "/home/u"}, "/work/a.b/c", session, "/home/u/.claude/projects/-work-a-b-c/" + session + ".jsonl"},
		{Env{Home: "/home/u", ClaudeConfigDir: "/cfg"}, "/work/x-y", session, "/cfg/projects/-work-x-y/" + session + ".jsonl"},
		{Env{Home: "/home/u"}, "/work/x_y", session, ""},
		{Env{Home: "/home/u"}, "/work/a b", session, ""},
		{Env{Home: "/home/u"}, "relative", session, ""},
		{Env{Home: "/home/u"}, "/work", "", ""},
		{Env{Home: "/home/u"}, "/work", "../x", ""},
		{Env{}, "/work", session, ""},
		{Env{ClaudeConfigDir: "rel"}, "/work", session, ""},
	}
	for _, c := range cases {
		got, err := transcriptPath(c.env, c.dir, c.session)
		if c.want == "" {
			if !errors.Is(err, errTranscript) {
				t.Errorf("transcriptPath(%+v, %q, %q) = %q, %v; want errTranscript", c.env, c.dir, c.session, got, err)
			}
			continue
		}
		if err != nil || got != filepath.FromSlash(c.want) {
			t.Errorf("transcriptPath(%+v, %q, %q) = %q, %v; want %q", c.env, c.dir, c.session, got, err, c.want)
		}
	}
}

// sendReq parses a send request, failing the test when it is malformed.
func sendReq(t *testing.T, workspace, command, issue, pr string) SendRequest {
	t.Helper()
	req, err := ParseSendRequest(workspace, command, issue, pr)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// TestMkreqTarget checks that /mkreq goes, with the human's issue number,
// only to a workspace without a task, and never to one with a task.
func TestMkreqTarget(t *testing.T) {
	sw := newSendWorld(t)
	free := sw.addTask(fakeWorkspace{ID: "w1", Label: "free", Branch: "user/feature"}, "claude")
	busy := sw.addTask(fakeWorkspace{ID: "w2", Label: "task", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, "claude")
	deps := sw.finish()
	out, err := Send(context.Background(), deps, sendReq(t, busy, "mkreq", "12", ""))
	if err != nil || out.Actions[0].Outcome != OutcomeNotSent || out.Actions[0].Reason != NotSentNotOffered {
		t.Fatalf("mkreq to a task workspace = %+v, %v; want not offered", out, err)
	}
	if got := sw.rec.prompts(); len(got) != 0 {
		t.Fatalf("typed %v", got)
	}
	out, err = Send(context.Background(), deps, sendReq(t, free, "mkreq", "12", ""))
	if err != nil || out.Actions[0].Outcome != OutcomeSent {
		t.Fatalf("mkreq to a free workspace = %+v, %v; want sent", out, err)
	}
	want := []string{"w1:p1 /clear", "w1:p1 /model opus", "w1:p1 /mkreq 12"}
	if got := sw.rec.prompts(); !slices.Equal(got, want) {
		t.Errorf("typed %v, want %v", got, want)
	}
}

// TestSendArgumentValidation checks the arguments of a send: the parser's
// combinations, then requests the tool does not offer for the workspace now
// (another next command, a PR number that is not the open PR, a takeover
// not proposed, a path that is no task workspace), none of which types
// anything.
func TestSendArgumentValidation(t *testing.T) {
	for _, c := range []struct{ workspace, command, issue, pr string }{
		{"/w", "weakreview", "", ""},
		{"/w", "", "", ""},
		{"w", "mkarch", "", ""},
		{"/w", "mkreq", "", ""},
		{"/w", "mkreq", "0", ""},
		{"/w", "mkarch", "3", ""},
		{"/w", "runpr", "", ""},
		{"/w", "runpr", "", "042"},
		{"/w", "runplan", "", "42"},
		{"/w", "runpr", "3", "42"},
	} {
		if _, err := ParseSendRequest(c.workspace, c.command, c.issue, c.pr); !errors.Is(err, ErrBadRequest) {
			t.Errorf("ParseSendRequest(%+v) = %v, want ErrBadRequest", c, err)
		}
	}
	for _, c := range []struct{ command, issue, pr string }{{"mkreq", "12", ""}, {"mkarch", "", ""}, {"runplan-takeover", "", ""}, {"runpr", "", "42"}} {
		if _, err := ParseSendRequest("/w", c.command, c.issue, c.pr); err != nil {
			t.Errorf("ParseSendRequest(%+v) = %v, want a request", c, err)
		}
	}
	if _, err := Send(context.Background(), Deps{}, SendRequest{}); !errors.Is(err, ErrBadRequest) {
		t.Errorf("Send of the zero request = %v, want ErrBadRequest", err)
	}
	sw := newSendWorld(t)
	propose := sw.addTask(fakeWorkspace{ID: "w1", Label: "propose", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, "claude")
	review := sw.addTask(fakeWorkspace{ID: "w2", Label: "review", Branch: taskBranch, PRs: openPRJSON}, "claude", "opencode")
	deps := sw.finish()
	cases := []struct {
		name   string
		req    SendRequest
		reason NotSentReason
	}{
		{"mkplan when the next is runplan", sendReq(t, propose, "mkplan", "", ""), NotSentNotOffered},
		{"a takeover not proposed", sendReq(t, propose, "runplan-takeover", "", ""), NotSentNotOffered},
		{"runpr where no PR is open", sendReq(t, propose, "runpr", "", "42"), NotSentNotOffered},
		{"runpr of another PR", sendReq(t, review, "runpr", "", "43"), NotSentNotOffered},
		{"runplan to a workspace awaiting review", sendReq(t, review, "runplan", "", ""), NotSentNotOffered},
		{"no task workspace", sendReq(t, propose+"-elsewhere", "runplan", "", ""), NotSentNoWorkspace},
	}
	for _, c := range cases {
		out, err := Send(context.Background(), deps, c.req)
		if err != nil || len(out.Actions) != 1 || out.Actions[0].Outcome != OutcomeNotSent || out.Actions[0].Reason != c.reason {
			t.Errorf("%s: Send = %+v, %v; want not sent, %s", c.name, out, err, c.reason)
		}
	}
	if got := sw.rec.prompts(); len(got) != 0 {
		t.Errorf("typed %v", got)
	}
}

// TestActionResults checks that the output of a send tells what was sent
// from what was not: a sent command names its workspace, pane, command,
// agent, and model; a command not sent has its reason and no step; a send
// that failed after an input has its reason, its last input, the pane's
// status, and its screen.
func TestActionResults(t *testing.T) {
	run := func(t *testing.T, react func(sw *sendWorld, p *simPane, text string) error, set func(sw *sendWorld)) ActionOutput {
		t.Helper()
		sw := newSendWorld(t)
		tree := sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, "claude")
		deps := sw.finish()
		if react != nil {
			sw.react = func(p *simPane, text string) error { return react(sw, p, text) }
		}
		if set != nil {
			set(sw)
		}
		out, err := Send(context.Background(), deps, sendReq(t, tree, "runplan", "", ""))
		if err != nil || len(out.Actions) != 1 {
			t.Fatalf("Send = %+v, %v", out, err)
		}
		got := out.Output().Actions[0]
		if got.Workspace != tree || got.Pane != "w1:p1" || got.Command != "/runplan 0001" || got.Agent != "claude_code" || got.Model != "opus" || got.Input != "command" {
			t.Errorf("result = %+v; want workspace, pane, command, agent, and model", got)
		}
		return got
	}
	t.Run("sent", func(t *testing.T) {
		got := run(t, nil, nil)
		if got.Outcome != "sent" || got.Reason != "" || got.Step != "" {
			t.Errorf("result = %+v, want sent with no reason", got)
		}
	})
	t.Run("not sent", func(t *testing.T) {
		got := run(t, nil, func(sw *sendWorld) {
			p := sw.firstPane()
			if err := os.Remove(sw.transcript(p, p.Session)); err != nil {
				t.Fatal(err)
			}
		})
		if got.Outcome != "not_sent" || got.Reason != "no_transcript" || got.Step != "" {
			t.Errorf("result = %+v, want not sent, no_transcript, no step", got)
		}
	})
	t.Run("sent then failed", func(t *testing.T) {
		got := run(t, func(sw *sendWorld, p *simPane, text string) error {
			err := sw.realReact(p, text)
			if text == "/clear" {
				p.screen = readScreenSample(sw.t, "claude-input-left.ansi")
			}
			return err
		}, nil)
		if got.Outcome != "sent_then_failed" || got.Reason != "input_left" || got.Step != "clear" || got.Status != "idle" || got.Left != "unsent draft text" || !strings.Contains(got.Screen, "unsent draft text") {
			t.Errorf("result = %+v, want sent then failed after clear with the status and the screen", got)
		}
	})
}

// TestSendTakeover checks that a /runplan handed back to Claude Code goes,
// when the tool proposes it, to the Claude Code pane with the takeover row's
// model, though the plan's route is OpenCode.
func TestSendTakeover(t *testing.T) {
	sw := newSendWorld(t)
	routed := strings.Replace(string(repoContext(t)), "| `runplan-opus` | `/runplan` for the requirement `opus` | Claude Code | `opus` |", "| `runplan-opus` | `/runplan` for the requirement `opus` | OpenCode | `"+openCodeModel+"` |", 1)
	if routed == string(repoContext(t)) {
		t.Fatal("the routing row to change was not found in _context.md")
	}
	tree := sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, "claude", "opencode")
	sw.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", []byte(routed))
	sw.git(tree, "rev-list --count refs/remotes/origin/main..HEAD", "3\n")
	sw.git(tree, "show HEAD:docs/tasks/"+taskDir+"/"+planFile, string(planSample(t, "marker-opus.md")))
	deps := sw.finish()
	out, err := Send(context.Background(), deps, sendReq(t, tree, "runplan-takeover", "", ""))
	if err != nil || len(out.Actions) != 1 || out.Actions[0].Outcome != OutcomeSent {
		t.Fatalf("Send = %+v, %v; want sent", out, err)
	}
	want := []string{"w1:p1 /clear", "w1:p1 /model sonnet", "w1:p1 /runplan 0001"}
	if got := sw.rec.prompts(); !slices.Equal(got, want) {
		t.Errorf("typed %v, want %v", got, want)
	}
}

// TestSendWriterRefusalSendsNothing checks that a prompt the writer refuses
// before anything was typed ends the send as not sent, without a stop.
func TestSendWriterRefusalSendsNothing(t *testing.T) {
	sw := newSendWorld(t)
	sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, "claude")
	deps := sw.finish()
	sc := sw.sendContextOf(deps)
	var err error
	// A writer that takes w1 for chief's workspace refuses every pane of it.
	if sc.writer, err = NewPaneWriter(deps.Cmds, PaneID{id: worldChiefPane}, "w1"); err != nil {
		t.Fatal(err)
	}
	res := sc.sendCommand(context.Background(), runplanCmd(t), opus(t))
	if res.Outcome != OutcomeNotSent || len(sw.rec.prompts()) != 0 {
		t.Fatalf("result = %+v, typed %v; want not sent", res, sw.rec.prompts())
	}
	if stop, err := sc.held.Stopped(); err != nil || stop != nil {
		t.Errorf("Stopped = %+v, %v; want no stop", stop, err)
	}
	if starts, ends, err := sc.held.sendFiles(); err != nil || len(starts) != 1 || len(ends) != 1 {
		t.Errorf("send records = %v %v %v; want the start and its end", starts, ends, err)
	}
}

// TestSendStopsWithoutANewSession checks that an OpenCode pane that shows no
// new session after /new gets no command and stops the workspace.
func TestSendStopsWithoutANewSession(t *testing.T) {
	sw := newSendWorld(t)
	sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, "opencode")
	deps := sw.finish()
	sw.react = func(p *simPane, text string) error {
		if text == "/new" {
			return nil
		}
		return sw.realReact(p, text)
	}
	sc := sw.sendContextOf(deps)
	res := sc.sendCommand(context.Background(), runprCmd(t), flash(t))
	if res.Outcome != OutcomeSentThenFailed || res.Step != StepNew || res.Reason != NotSentNoStep || len(sw.rec.prompts()) != 1 {
		t.Errorf("result = %+v, typed %v; want sent then failed after /new, the step not finished", res, sw.rec.prompts())
	}
	if stop, _ := sc.held.Stopped(); stop == nil {
		t.Error("the workspace was not stopped")
	}
}

// TestSendStopUnwritten checks a failure after an input whose stop cannot be
// written: the end is not written either, so the start alone stops the
// workspace at the next settle.
func TestSendStopUnwritten(t *testing.T) {
	sw := newSendWorld(t)
	sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch}, "claude")
	deps := sw.finish()
	sw.react = func(p *simPane, text string) error {
		if text == "/clear" {
			return nil
		}
		return sw.realReact(p, text)
	}
	deps.Store.beforeCreate = func(name string) error {
		if name == stopFile {
			return errNoReply
		}
		return nil
	}
	sc := sw.sendContextOf(deps)
	res := sc.sendCommand(context.Background(), runplanCmd(t), opus(t))
	if res.Outcome != OutcomeSentThenFailed {
		t.Fatalf("result = %+v, want sent then failed", res)
	}
	starts, ends, err := sc.held.sendFiles()
	if err != nil || len(starts) != 1 || len(ends) != 0 {
		t.Fatalf("send records = %v %v %v; want the start without an end", starts, ends, err)
	}
	sc.held.beforeCreate = nil
	if n, err := sc.held.SettleUnfinished(deps.Now()); err != nil || n != 1 {
		t.Fatalf("SettleUnfinished = %d, %v", n, err)
	}
	if stop, _ := sc.held.Stopped(); stop == nil {
		t.Error("the start did not stop the workspace")
	}
}

// TestTranscriptFIFORefused checks that a FIFO in the transcript's place is
// refused at once instead of blocking the open until a writer comes.
func TestTranscriptFIFORefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() {
		done <- transcriptOpens(path)
		_, err := readTranscript(path)
		done <- err
	}()
	for range 2 {
		select {
		case err := <-done:
			if !errors.Is(err, errTranscript) {
				t.Errorf("err = %v, want errTranscript", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the open of a FIFO blocked")
		}
	}
}
