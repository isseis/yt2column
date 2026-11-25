//go:build test

package chief

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// TestNewRunningCommand checks, for each of the six kinds, an argument of
// the accepted form, no argument, and arguments that must be dropped, and
// that a command the constructor makes shows as the text typed into a pane.
func TestNewRunningCommand(t *testing.T) {
	cases := []struct {
		name        string
		kind        paneinput.CommandKind
		arg         string
		wantText    string
		wantDropped bool
	}{
		{"mkreq with an issue", paneinput.CommandMkreq, "890", "/mkreq 890", false},
		{"mkarch with a task", paneinput.CommandMkarch, "0093", "/mkarch 0093", false},
		{"mkplan with a task", paneinput.CommandMkplan, "0093", "/mkplan 0093", false},
		{"runplan with a task", paneinput.CommandRunplan, "0093", "/runplan 0093", false},
		{"runpr with a PR", paneinput.CommandRunpr, "926", "/runpr 926", false},
		{"mergemain with a PR", paneinput.CommandMergemain, "1019", "/mergemain 1019", false},
		{"a task number given to /mergemain", paneinput.CommandMergemain, "0093", "/mergemain", true},
		{"no argument", paneinput.CommandRunplan, "", "/runplan", false},
		{"a second argument", paneinput.CommandMkreq, "890 0093", "/mkreq", true},
		{"a control character", paneinput.CommandRunplan, "00\x1b93", "/runplan", true},
		{"a newline that would start another command", paneinput.CommandRunplan, "0093\n/runpr 1", "/runplan", true},
		{"a text that is not a number", paneinput.CommandRunpr, "main", "/runpr", true},
		{"an issue with a leading zero", paneinput.CommandMkreq, "0890", "/mkreq", true},
		{"a task that is not four digits", paneinput.CommandMkarch, "93", "/mkarch", true},
		{"a PR number given to a task command", paneinput.CommandRunplan, "926", "/runplan", true},
		{"a task number given to /runpr", paneinput.CommandRunpr, "0093", "/runpr", true},
		{"a flag", paneinput.CommandRunpr, "--force", "/runpr", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, dropped, err := newRunningCommand(c.kind, c.arg)
			if err != nil {
				t.Fatalf("newRunningCommand: %v", err)
			}
			if dropped != c.wantDropped || cmd.Text() != c.wantText || cmd.Kind() != c.kind {
				t.Errorf("newRunningCommand = %q, kind %d, dropped %v; want %q, kind %d, dropped %v", cmd.Text(), cmd.Kind(), dropped, c.wantText, c.kind, c.wantDropped)
			}
		})
	}
}

// TestNewRunningCommandRejectsOtherKinds checks that a kind outside the five
// task commands makes no command.
func TestNewRunningCommandRejectsOtherKinds(t *testing.T) {
	for _, kind := range []paneinput.CommandKind{paneinput.CommandUnknown, paneinput.CommandKind(-1), paneinput.CommandKind(99)} {
		cmd, dropped, err := newRunningCommand(kind, "0093")
		if !errors.Is(err, paneinput.ErrInvalid) || dropped || cmd != (RunningCommand{}) {
			t.Errorf("newRunningCommand(%d) = %+v, %v, %v; want the zero value and ErrInvalid", kind, cmd, dropped, err)
		}
	}
}

// TestRunningCommandTextMatchesThePaneInput checks that the text of each
// kind is the line chief types for the same command, so the name table and
// the typed text agree.
func TestRunningCommandTextMatchesThePaneInput(t *testing.T) {
	task, err := paneinput.ParseTaskID("0093")
	if err != nil {
		t.Fatal(err)
	}
	issue, err := paneinput.ParseIssueNumber("890")
	if err != nil {
		t.Fatal(err)
	}
	pr, err := fixprguard.ParsePRNumber("926")
	if err != nil {
		t.Fatal(err)
	}
	must := func(c paneinput.Command, err error) paneinput.Command {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	mkreq := must(paneinput.NewMkreq(issue))
	mkarch := must(paneinput.NewMkarch(task))
	mkplan := must(paneinput.NewMkplan(task))
	runplan := must(paneinput.NewRunplan(task, paneinput.RouteRunplanHigh))
	runpr := must(paneinput.NewRunpr(pr))
	mergemain := must(paneinput.NewMergemain(pr))
	cases := []struct {
		kind paneinput.CommandKind
		arg  string
		sent paneinput.Command
	}{
		{paneinput.CommandMkreq, "890", mkreq},
		{paneinput.CommandMkarch, "0093", mkarch},
		{paneinput.CommandMkplan, "0093", mkplan},
		{paneinput.CommandRunplan, "0093", runplan},
		{paneinput.CommandRunpr, "926", runpr},
		{paneinput.CommandMergemain, "926", mergemain},
	}
	for _, c := range cases {
		sent, err := c.sent.Text()
		if err != nil {
			t.Fatal(err)
		}
		cmd, _, err := newRunningCommand(c.kind, c.arg)
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Text() != sent {
			t.Errorf("kind %d: Text() = %q, the pane input is %q", c.kind, cmd.Text(), sent)
		}
	}
}

// TestRunningCommandRevalidates checks that a value built inside the package
// without the constructor shows only what validates: an argument that does
// not validate is dropped, and a kind outside the five shows nothing. The
// constructor is the only way to a command with an unvalidated argument
// outside the package, and the text method guards the rest.
func TestRunningCommandRevalidates(t *testing.T) {
	cases := []struct {
		name string
		cmd  RunningCommand
		text string
		kind paneinput.CommandKind
	}{
		{"the zero value", RunningCommand{}, "", paneinput.CommandUnknown},
		{"a kind outside the five", RunningCommand{kind: paneinput.CommandKind(99), arg: "0093"}, "", paneinput.CommandUnknown},
		{"an argument that was not validated", RunningCommand{kind: paneinput.CommandRunplan, arg: "0093\n/runpr 1"}, "/runplan", paneinput.CommandRunplan},
		{"an argument of another kind's form", RunningCommand{kind: paneinput.CommandRunplan, arg: "926"}, "/runplan", paneinput.CommandRunplan},
		{"a valid argument", RunningCommand{kind: paneinput.CommandRunplan, arg: "0093"}, "/runplan 0093", paneinput.CommandRunplan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.cmd.Text() != c.text || c.cmd.Kind() != c.kind {
				t.Errorf("Text() = %q, Kind() = %d; want %q, %d", c.cmd.Text(), c.cmd.Kind(), c.text, c.kind)
			}
		})
	}
}

// runWorld is a worktree with a Claude Code configuration directory and a
// state directory, in which the running-command tests write transcripts and
// send records.
type runWorld struct {
	t       *testing.T
	sw      *sendWorld
	deps    Deps
	tree    string
	held    *HeldWorkspace
	session int
	sends   int
}

func newRunWorld(t *testing.T) *runWorld {
	t.Helper()
	sw := newSendWorld(t)
	tree := sw.addTree(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch})
	deps := sw.finish()
	records, err := deps.Store.Records(Worktree{path: tree})
	if err != nil {
		t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() })
	return &runWorld{t: t, sw: sw, deps: deps, tree: tree, held: held}
}

// claude returns a working Claude Code pane whose session's transcript holds
// lines, each ended by a newline.
func (rw *runWorld) claude(pane string, lines ...string) Agent {
	rw.t.Helper()
	rw.session++
	a := Agent{Pane: PaneID{id: pane}, Kind: paneinput.AgentClaudeCode, Status: AgentWorking, Session: fmt.Sprintf("00000000-0000-4000-8000-2000000000%02d", rw.session), Dir: rw.tree}
	p := &simPane{fakeAgent: fakeAgent{Dir: rw.tree}}
	if err := rw.sw.writeTranscript(p, a.Session, append([]string{modeLine(a.Session)}, lines...)...); err != nil {
		rw.t.Fatal(err)
	}
	return a
}

// transcript returns the path of a Claude Code pane's transcript.
func (rw *runWorld) transcript(a Agent) string {
	return rw.sw.transcript(&simPane{fakeAgent: fakeAgent{Dir: a.Dir}}, a.Session)
}

// openCode returns a working OpenCode pane.
func (rw *runWorld) openCode(pane string) Agent {
	return Agent{Pane: PaneID{id: pane}, Kind: paneinput.AgentOpenCode, Status: AgentWorking, Dir: rw.tree}
}

// send records a send to pane whose start is rec and, unless outcome is "",
// whose end has outcome. Sends are recorded a second apart, in call order.
func (rw *runWorld) send(rec SendRecord, outcome string) {
	rw.t.Helper()
	rw.sends++
	at := rw.sw.now.Add(time.Duration(rw.sends) * time.Second)
	id, err := rw.held.BeginSend(rec, at)
	if err != nil {
		rw.t.Fatal(err)
	}
	if outcome != "" {
		if err := rw.held.EndSend(id, outcome, "", at); err != nil {
			rw.t.Fatal(err)
		}
	}
}

// The agent labels a send record declares.
var (
	claudeCodeLabel = agentLabel(paneinput.AgentClaudeCode)
	openCodeLabel   = agentLabel(paneinput.AgentOpenCode)
)

// command records a command send of kind and arg to agent in pane, with
// model.
func (rw *runWorld) command(agent, pane, kind, arg, model, outcome string) {
	rw.send(SendRecord{Pane: pane, Input: InputCommand.String(), Command: "/" + kind + " " + arg, Agent: agent, Model: model, Kind: kind, Arg: arg}, outcome)
}

// run decides the running command of agents from the world's records.
func (rw *runWorld) run(agents ...Agent) (string, string, RunningUnknown) {
	shown, why := runningOf(context.Background(), rw.deps, agents, rw.held.lastSentCommand)
	if shown == nil {
		return "", "", why
	}
	return shown.Command.Text(), shown.Model, why
}

// TestRunningCommandPerPane checks the running command of one working pane:
// a Claude Code pane by its session's last human input, an OpenCode pane by
// chief's last command sent to it and recorded as sent, and each reason a
// pane decides none; a pane that is not working is not read.
func TestRunningCommandPerPane(t *testing.T) {
	sent := OutcomeSent.String()
	cases := []struct {
		name  string
		setup func(rw *runWorld) []Agent
		want  string
		why   RunningUnknown
	}{
		{"a Claude Code pane running a task command", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"))}
		}, "/runplan 0093", RunningUnknownNone},
		{"a Claude Code pane whose last input is free text", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"), typedLine("go on"))}
		}, "/runplan 0093", RunningUnknownNone},
		{"a Claude Code pane with free text only", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", typedLine("go on"))}
		}, "", RunningUnknownNotTaskCommand},
		{"a Claude Code pane with no session", func(rw *runWorld) []Agent {
			a := rw.claude("w1:p1", commandLine("/runplan 0093"))
			a.Session = ""
			return []Agent{a}
		}, "", RunningUnknownNoSession},
		{"a Claude Code pane in a directory the rule does not cover", func(rw *runWorld) []Agent {
			a := rw.claude("w1:p1", commandLine("/runplan 0093"))
			a.Dir += "_x"
			return []Agent{a}
		}, "", RunningUnknownUnsupportedPath},
		{"a Claude Code pane without a transcript", func(rw *runWorld) []Agent {
			a := rw.claude("w1:p1")
			if err := os.Remove(rw.transcript(a)); err != nil {
				t.Fatal(err)
			}
			return []Agent{a}
		}, "", RunningUnknownUnreadable},
		{"a Claude Code transcript with a line that is not JSON", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"), `{"type":`)}
		}, "", RunningUnknownUnreadable},
		{"a Claude Code transcript whose last line ends during the re-reads", func(rw *runWorld) []Agent {
			a := rw.claude("w1:p1", commandLine("/runplan 0093"))
			path := rw.transcript(a)
			if err := os.WriteFile(path, []byte(modeLine(a.Session)+"\n"+commandLine("/runplan 0093")+"\n"+commandLine("/runpr 926")[:20]), 0o600); err != nil {
				t.Fatal(err)
			}
			waits := 0
			rw.deps.Sleep = func(context.Context, time.Duration) error {
				if waits++; waits == 2 {
					return os.WriteFile(path, []byte(modeLine(a.Session)+"\n"+commandLine("/runplan 0093")+"\n"+commandLine("/runpr 926")+"\n"), 0o600)
				}
				return nil
			}
			return []Agent{a}
		}, "/runpr 926", RunningUnknownNone},
		{"a start record that names another send", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runpr", "926", "m", sent)
			starts, _, err := rw.held.sendFiles()
			if err != nil || len(starts) != 1 {
				t.Fatalf("sends = %v, %v", starts, err)
			}
			// A later start without an end names the sent one.
			forged := fmt.Sprintf(`{"id":%q,"pane":"w1:p1","input":"command","command":"/runpr 927","agent":"","model":"","prompts":null,"started":"2026-11-14T09:00:00Z","kind":"runpr","arg":"927"}`, starts[0])
			if err := os.WriteFile(filepath.Join(rw.held.dir, sendPrefix+"99999999999999999999-0123456789abcdef"+startSuffix), []byte(forged), 0o600); err != nil {
				t.Fatal(err)
			}
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownUnreadable},
		// The reason is what running_unknown will show for a caller's
		// mistake; it must not read as a decided command.
		{"no working pane", func(rw *runWorld) []Agent {
			a := rw.claude("w1:p1", commandLine("/runplan 0093"))
			a.Status = AgentIdle
			return []Agent{a}
		}, "", RunningUnknownPanesDisagree},
		{"a Claude Code transcript over the cap", func(rw *runWorld) []Agent {
			a := rw.claude("w1:p1", commandLine("/runplan 0093"))
			if err := os.Truncate(rw.transcript(a), sessionTranscriptLimit+1); err != nil {
				t.Fatal(err)
			}
			return []Agent{a}
		}, "", RunningUnknownOverCap},
		{"a Claude Code transcript still ending inside a line", func(rw *runWorld) []Agent {
			a := rw.claude("w1:p1", commandLine("/runplan 0093"))
			f, err := os.OpenFile(rw.transcript(a), os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = f.Close() }()
			if _, err := f.WriteString(`{"type":"user","origin":{"kind":"hum`); err != nil {
				t.Fatal(err)
			}
			return []Agent{a}
		}, "", RunningUnknownPartialLine},
		{"an OpenCode pane sent a command", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runpr", "926", "m", sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "/runpr 926", RunningUnknownNone},
		{"an OpenCode pane answered by a relay after its command", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runpr", "926", "m", sent)
			rw.send(SendRecord{Pane: "w1:p1", Input: InputRelay.String(), Command: "1. Yes"}, sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "/runpr 926", RunningUnknownNone},
		{"an OpenCode pane reset after its command", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runpr", "926", "m", sent)
			rw.send(SendRecord{Pane: "w1:p1", Input: InputReset.String(), Command: "/new"}, sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"a reset whose record declares a kind", func(rw *runWorld) []Agent {
			// Only a command send declares a kind; the input, not the kind,
			// decides that a reset is no command.
			rw.send(SendRecord{Pane: "w1:p1", Input: InputReset.String(), Command: "/new", Kind: "runpr", Arg: "926"}, sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"an OpenCode pane's record from before the kind was declared", func(rw *runWorld) []Agent {
			rw.send(SendRecord{Pane: "w1:p1", Input: InputCommand.String(), Command: "/runpr 926"}, sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"an OpenCode pane's record of a kind not in the table", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "fixpr", "926", "m", sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"an OpenCode send without its end", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runpr", "926", "m", "")
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"an OpenCode send that failed after an input", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runpr", "926", "m", OutcomeSentThenFailed.String())
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"an OpenCode pane chief never sent to", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p2", "runpr", "926", "m", sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"an OpenCode pane whose record cannot be read", func(rw *runWorld) []Agent {
			name := sendPrefix + "01794646800000000000-0123456789abcdef" + startSuffix
			if err := os.WriteFile(filepath.Join(rw.held.dir, name), []byte(`{"pane":"w1:p1","extra":1}`), 0o600); err != nil {
				t.Fatal(err)
			}
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownUnreadable},
		{"an OpenCode pane whose last command went to another agent", func(rw *runWorld) []Agent {
			// The pane's agent was replaced after chief's send.
			rw.command(claudeCodeLabel, "w1:p1", "runpr", "926", "opus", sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "", RunningUnknownNoSentCommand},
		{"a pane of no known agent", func(rw *runWorld) []Agent {
			a := rw.openCode("w1:p1")
			a.Kind = paneinput.AgentUnknown
			return []Agent{a}
		}, "", RunningUnknownUnreadable},
		{"an idle pane beside the working one", func(rw *runWorld) []Agent {
			idle := rw.openCode("w1:p2")
			idle.Status = AgentIdle
			return []Agent{idle, rw.claude("w1:p1", commandLine("/mkarch 0093"))}
		}, "/mkarch 0093", RunningUnknownNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rw := newRunWorld(t)
			text, _, why := rw.run(c.setup(rw)...)
			if text != c.want || why != c.why {
				t.Errorf("running = %q, %d; want %q, %d", text, why, c.want, c.why)
			}
		})
	}
}

// TestRunningCommandPanes checks two working panes: a command is shown only
// when both decide the same one with no argument dropped; otherwise the
// first undecided pane's reason, or the panes' disagreement, is given.
func TestRunningCommandPanes(t *testing.T) {
	sent := OutcomeSent.String()
	cases := []struct {
		name  string
		setup func(rw *runWorld) []Agent
		want  string
		why   RunningUnknown
	}{
		{"the same command on both", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p2", "runplan", "0093", "m", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093")), rw.openCode("w1:p2")}
		}, "/runplan 0093", RunningUnknownNone},
		{"different commands", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p2", "runpr", "926", "m", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093")), rw.openCode("w1:p2")}
		}, "", RunningUnknownPanesDisagree},
		{"the same kind with other arguments", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093")), rw.claude("w1:p2", commandLine("/runplan 0094"))}
		}, "", RunningUnknownPanesDisagree},
		{"one pane undecided", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093")), rw.openCode("w1:p2")}
		}, "", RunningUnknownNoSentCommand},
		{"both undecided, the first pane's reason", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", typedLine("hello")), rw.openCode("w1:p2")}
		}, "", RunningUnknownNotTaskCommand},
		{"an argument dropped on the first pane", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runpr main")), rw.claude("w1:p2", commandLine("/runpr"))}
		}, "", RunningUnknownPanesDisagree},
		{"an argument dropped on the second pane", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runpr")), rw.claude("w1:p2", commandLine("/runpr main"))}
		}, "", RunningUnknownPanesDisagree},
		{"an argument dropped on both", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runpr main")), rw.claude("w1:p2", commandLine("/runpr dev"))}
		}, "", RunningUnknownPanesDisagree},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rw := newRunWorld(t)
			text, _, why := rw.run(c.setup(rw)...)
			if text != c.want || why != c.why {
				t.Errorf("running = %q, %d; want %q, %d", text, why, c.want, c.why)
			}
		})
	}
}

// TestRunningModel checks when the model of chief's send record is shown:
// only for one working pane whose last command sent, recorded as sent, is
// the running command with its argument whole.
func TestRunningModel(t *testing.T) {
	sent := OutcomeSent.String()
	cases := []struct {
		name  string
		setup func(rw *runWorld) []Agent
		want  string
		model string
	}{
		{"an OpenCode pane's sent command", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runpr", "926", "flash", sent)
			return []Agent{rw.openCode("w1:p1")}
		}, "/runpr 926", "flash"},
		{"a Claude Code pane running the command chief sent", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runplan", "0093", "opus", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"))}
		}, "/runplan 0093", "opus"},
		{"a Claude Code pane running another command than chief sent", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runplan", "0094", "opus", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"))}
		}, "/runplan 0093", ""},
		{"a Claude Code pane with an input after the command chief sent", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runplan", "0093", "opus", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"), typedLine("go on"))}
		}, "/runplan 0093", ""},
		{"a Claude Code pane whose send failed after an input", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runplan", "0093", "opus", OutcomeSentThenFailed.String())
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"))}
		}, "/runplan 0093", ""},
		{"a Claude Code pane whose records cannot be read", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runplan", "0093", "opus", sent)
			name := sendPrefix + "99999999999999999999-0123456789abcdef" + startSuffix
			if err := os.WriteFile(filepath.Join(rw.held.dir, name), []byte(`{"pane":"w1:p1","extra":1}`), 0o600); err != nil {
				t.Fatal(err)
			}
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"))}
		}, "/runplan 0093", ""},
		{"a record whose argument does not validate", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runpr", "main", "opus", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runpr"))}
		}, "/runpr", ""},
		{"a pane whose argument was dropped, with a record of no argument", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runpr", "", "opus", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runpr main"))}
		}, "/runpr", ""},
		{"a Claude Code pane whose last command went to another agent", func(rw *runWorld) []Agent {
			rw.command(openCodeLabel, "w1:p1", "runplan", "0093", "flash", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"))}
		}, "/runplan 0093", ""},
		{"two working panes", func(rw *runWorld) []Agent {
			rw.command(claudeCodeLabel, "w1:p1", "runplan", "0093", "opus", sent)
			rw.command(openCodeLabel, "w1:p2", "runplan", "0093", "flash", sent)
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093")), rw.openCode("w1:p2")}
		}, "/runplan 0093", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rw := newRunWorld(t)
			text, model, why := rw.run(c.setup(rw)...)
			if text != c.want || model != c.model || why != RunningUnknownNone {
				t.Errorf("running = %q, model %q, %d; want %q, model %q", text, model, why, c.want, c.model)
			}
		})
	}
}

// TestRunningLater checks Later: it is set when the human typed something
// after the running command on any working pane, and not otherwise.
func TestRunningLater(t *testing.T) {
	cases := []struct {
		name  string
		setup func(rw *runWorld) []Agent
		later bool
	}{
		{"the command is the last input", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"))}
		}, false},
		{"free text after the command", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"), typedLine("go on"))}
		}, true},
		{"free text after the command on the first pane", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093"), typedLine("go on")), rw.claude("w1:p2", commandLine("/runplan 0093"))}
		}, true},
		{"free text after the command on the second pane", func(rw *runWorld) []Agent {
			return []Agent{rw.claude("w1:p1", commandLine("/runplan 0093")), rw.claude("w1:p2", commandLine("/runplan 0093"), typedLine("go on"))}
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rw := newRunWorld(t)
			shown, why := runningOf(context.Background(), rw.deps, c.setup(rw), rw.held.lastSentCommand)
			if shown == nil || why != RunningUnknownNone {
				t.Fatalf("running = %+v, %d; want /runplan 0093", shown, why)
			}
			if shown.Command.Text() != "/runplan 0093" || shown.Later != c.later {
				t.Errorf("running = %q, later %t; want /runplan 0093, later %t", shown.Command.Text(), shown.Later, c.later)
			}
		})
	}
}

// TestRunningOfPaneLaterInput checks what a Claude Code pane's reading says
// of the human's input after its last task command, for each way the
// transcript reads: no file yet is told apart from one that cannot be read,
// and a session with no task command from one with no input at all.
func TestRunningOfPaneLaterInput(t *testing.T) {
	cases := []struct {
		name  string
		setup func(rw *runWorld) Agent
		want  LaterInput
	}{
		{"no transcript yet", func(rw *runWorld) Agent {
			a := rw.claude("w1:p1")
			if err := os.Remove(rw.transcript(a)); err != nil {
				t.Fatal(err)
			}
			return a
		}, LaterInputNoTranscript},
		{"a line that is not JSON", func(rw *runWorld) Agent {
			return rw.claude("w1:p1", commandLine("/mergemain 926"), `{"type":`)
		}, LaterInputUnreadable},
		{"the task command is the last input", func(rw *runWorld) Agent {
			return rw.claude("w1:p1", commandLine("/mergemain 926"))
		}, LaterInputNone},
		{"free text after the task command", func(rw *runWorld) Agent {
			return rw.claude("w1:p1", commandLine("/mergemain 926"), typedLine("go on"))
		}, LaterInputAfter},
		{"no human input", func(rw *runWorld) Agent { return rw.claude("w1:p1") }, LaterInputNoTaskEmpty},
		{"free text and no task command", func(rw *runWorld) Agent {
			return rw.claude("w1:p1", typedLine("go on"))
		}, LaterInputNoTaskInput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rw := newRunWorld(t)
			p := runningOfPane(context.Background(), rw.deps, c.setup(rw), rw.held.lastSentCommand)
			if p.input != c.want {
				t.Errorf("input = %d, want %d", p.input, c.want)
			}
		})
	}
}
