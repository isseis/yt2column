package chief

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// InputKind is what one action typed into a pane. The zero value is no
// input.
type InputKind int

const (
	// InputUnknown is no input.
	InputUnknown InputKind = iota
	// InputCommand is a task command with the inputs before it.
	InputCommand
	// InputReset is the /clear or /new that empties a pane.
	InputReset
)

// String names the input kind for the output.
func (k InputKind) String() string {
	switch k {
	case InputCommand:
		return "command"
	case InputReset:
		return "reset"
	default:
		return unknownName
	}
}

// ActionOutcome separates "nothing was sent" from "sent, then failed". The
// zero value is OutcomeUnknown, which is shown and treated as sent-then-
// failed, never as nothing sent.
type ActionOutcome int

const (
	// OutcomeUnknown is an outcome no step set.
	OutcomeUnknown ActionOutcome = iota
	// OutcomeSent is every input sent and confirmed.
	OutcomeSent
	// OutcomeNotSent is nothing typed into the pane.
	OutcomeNotSent
	// OutcomeSentThenFailed is an input typed, then a step failed; the
	// workspace is stopped until release.
	OutcomeSentThenFailed
)

// String names the outcome for the output and the records.
func (o ActionOutcome) String() string {
	switch o {
	case OutcomeSent:
		return "sent"
	case OutcomeNotSent:
		return "not_sent"
	case OutcomeSentThenFailed:
		return "sent_then_failed"
	default:
		return outcomeUnknown
	}
}

// NotSentReason says why nothing was sent, or which check failed after an
// input. The zero value is NotSentUnknown and is never read as permission to
// send.
type NotSentReason int

const (
	// NotSentUnknown is a reason no step set.
	NotSentUnknown NotSentReason = iota
	// NotSentNotAccepting is a pane that is not idle or done, or that shows
	// a question.
	NotSentNotAccepting
	// NotSentInputLeft is unsent text in the pane's input line.
	NotSentInputLeft
	// NotSentModelMismatch is an OpenCode pane of another model, or a Claude
	// Code pane that switched to another model.
	NotSentModelMismatch
	// NotSentNoTranscript is a Claude Code pane whose session or transcript
	// is unknown, or whose model switch could not be confirmed.
	NotSentNoTranscript
	// NotSentPaneAmbiguous is zero or several panes of the route's agent.
	NotSentPaneAmbiguous
	// NotSentUnreadable is something the tool could not read: a screen of no
	// known shape, the routing table, Herdr, or a record.
	NotSentUnreadable
	// NotSentStopped is a workspace stopped until release.
	NotSentStopped
	// NotSentBusy is a workspace another send holds.
	NotSentBusy
	// NotSentNotOffered is a command the tool does not offer for the
	// workspace now: another next command, no open PR of that number, no
	// takeover, or a task already present for /mkreq.
	NotSentNotOffered
	// NotSentNoWorkspace is a --workspace that is no current task
	// workspace.
	NotSentNoWorkspace
	// NotSentSamePane is a reset candidate that is the pane /runpr went to.
	NotSentSamePane
	// NotSentNoReport is a reset candidate whose screen no longer shows the
	// report the PR detail kept, or that never did.
	NotSentNoReport
	// NotSentPaneChanged is a reset candidate whose state changed since the
	// PR detail kept its report.
	NotSentPaneChanged
	// NotSentNoStep is an input that did not finish: /clear kept the
	// session, /new showed no new session, or the command did not start.
	NotSentNoStep
)

// String names the reason for the output.
func (r NotSentReason) String() string {
	names := map[NotSentReason]string{
		NotSentNotAccepting:  "not_accepting",
		NotSentInputLeft:     "input_left",
		NotSentModelMismatch: "model_mismatch",
		NotSentNoTranscript:  "no_transcript",
		NotSentPaneAmbiguous: "pane_ambiguous",
		NotSentUnreadable:    "unreadable",
		NotSentStopped:       "stopped",
		NotSentBusy:          "busy",
		NotSentNotOffered:    "not_offered",
		NotSentNoWorkspace:   "no_workspace",
		NotSentSamePane:      "same_pane",
		NotSentNoReport:      "no_report",
		NotSentPaneChanged:   "pane_changed",
		NotSentNoStep:        "step_not_finished",
	}
	if name, ok := names[r]; ok {
		return name
	}
	return unknownName
}

// SentStep is the last input typed before a failure. The zero value is no
// input.
type SentStep int

const (
	// StepUnknown is no input.
	StepUnknown SentStep = iota
	// StepClear is Claude Code's /clear.
	StepClear
	// StepModel is Claude Code's /model.
	StepModel
	// StepNew is OpenCode's /new.
	StepNew
	// StepCommand is the task command.
	StepCommand
)

// String names the step for the output.
func (s SentStep) String() string {
	switch s {
	case StepClear:
		return "clear"
	case StepModel:
		return "model"
	case StepNew:
		return "new"
	case StepCommand:
		return "command"
	default:
		return ""
	}
}

// ActionResult is one send or reset, for the report: where it went, what it
// typed, and how it ended.
type ActionResult struct {
	Workspace string
	Pane      string
	Input     InputKind
	// Command is the command line, or the reset line, typed or meant.
	Command string
	Agent   paneinput.AgentKind
	Model   string
	Outcome ActionOutcome
	Reason  NotSentReason
	// Step is the last input typed, for OutcomeSentThenFailed.
	Step SentStep
	// Detail says in English what the reason was about.
	Detail string
	// Left is the unsent text of the input line, for NotSentInputLeft.
	Left string
	// Panes lists the workspace's panes, for NotSentPaneAmbiguous.
	Panes []string
	// Status and Screen are the pane's status and the tail of its screen
	// after a failure.
	Status AgentStatus
	Screen string
}

// The waits after an input. Herdr reports no completion of /clear, /model,
// or /new and the start of a command shows only in the agent's status, so
// the tool reads again at pollInterval up to pollAttempts times (15 seconds).
const (
	pollInterval = 500 * time.Millisecond
	pollAttempts = 30
)

// step is one input of a send and the check that it finished.
type step struct {
	kind   SentStep
	prompt paneinput.Prompt
}

// sendContext is what one send or reset works with: the readers, the writer,
// the held workspace, and the clock.
type sendContext struct {
	deps   Deps
	herdr  Herdr
	writer PaneWriter
	held   *HeldWorkspace
	ws     TaskWorkspace
}

// paneCheck is what the check before an input asks for besides a pane that
// accepts input with an empty input line.
type paneCheck struct {
	// transcript asks that a Claude Code pane's transcript can be found and
	// opened: before the first input of a command.
	transcript bool
	// model asks that an OpenCode pane shows the route's model: before the
	// first input and before the command after /new.
	model paneinput.OpenCodeModel
	// session, when set, asks that the pane still has this agent session:
	// the one the previous input left, so a session changed by anyone else
	// between two inputs stops the send.
	session string
	// seq, when set, asks that the pane's state has not changed since it
	// was read: a reset sends only to the pane whose report it just read.
	seq *int64
}

// checkFailure is a failed check: its reason, what it was about, and the
// pane's state then.
type checkFailure struct {
	reason NotSentReason
	detail string
	left   string
	agent  Agent
	screen string
}

// checkPane reads the pane again and checks that it accepts input now.
// Contract: every reading is fresh (the agent from `herdr agent get`, the
// screen from `herdr agent read`); a pane that moved, changed agent, or
// cannot be read fails; it returns the agent as read, for the wait after the
// input.
func (s sendContext) checkPane(ctx context.Context, target Agent, check paneCheck) (Agent, *checkFailure) {
	a, err := s.herdr.Agent(ctx, target.Pane)
	if err != nil {
		return target, &checkFailure{reason: NotSentUnreadable, detail: "herdr agent get failed: " + err.Error(), agent: target}
	}
	if a.Workspace != target.Workspace || a.Kind != target.Kind {
		return a, &checkFailure{reason: NotSentUnreadable, detail: "the pane is no longer the same agent of the workspace", agent: a}
	}
	if check.session != "" && a.Session != check.session {
		return a, &checkFailure{reason: NotSentPaneChanged, detail: "the pane's agent session changed", agent: a}
	}
	if check.seq != nil && a.StateChangeSeq != *check.seq {
		return a, &checkFailure{reason: NotSentPaneChanged, detail: "the pane's state changed since it was read", agent: a}
	}
	raw, err := s.herdr.Screen(ctx, a.Pane, ScreenVisible)
	if err != nil {
		return a, &checkFailure{reason: NotSentUnreadable, detail: "herdr agent read failed: " + err.Error(), agent: a}
	}
	summary := screenSummary(raw)
	fail := func(reason NotSentReason, detail string) (Agent, *checkFailure) {
		return a, &checkFailure{reason: reason, detail: detail, agent: a, screen: summary}
	}
	if a.Status != AgentIdle && a.Status != AgentDone {
		return fail(NotSentNotAccepting, "the agent is not idle or done")
	}
	screen := readScreen(a.Kind, raw)
	switch screen.Kind {
	case ScreenPrompt:
	case ScreenQuestion:
		return fail(NotSentNotAccepting, "the pane shows a question")
	default:
		return fail(NotSentUnreadable, "the screen matches no known shape")
	}
	if !screen.Input.Empty {
		f := &checkFailure{reason: NotSentInputLeft, detail: "the input line holds unsent text", left: screen.Input.Left, agent: a, screen: summary}
		return a, f
	}
	if check.transcript {
		path, err := transcriptPath(s.deps.Env, a.Dir, a.Session)
		if err == nil {
			err = transcriptOpens(path)
		}
		if err != nil {
			return fail(NotSentNoTranscript, err.Error())
		}
	}
	if check.model != (paneinput.OpenCodeModel{}) && !check.model.Matches(screen.OpenCodeModel) {
		return fail(NotSentModelMismatch, fmt.Sprintf("the pane shows model %q, not %q; switch it in the pane, then send again", screen.OpenCodeModel, check.model.String()))
	}
	return a, nil
}

// targetPane returns the one agent of kind in the workspace. Contract: zero
// or several agents of the kind is no target, so a pane the human may be
// talking to is never chosen among several; chief's own workspace is never a
// task workspace, so its panes are never among them.
func targetPane(ws TaskWorkspace, kind paneinput.AgentKind) (Agent, []string, bool) {
	var found []Agent
	var panes []string
	for _, a := range ws.Agents {
		panes = append(panes, a.Pane.String()+" "+agentLabel(a.Kind))
		if a.Kind == kind && kind != paneinput.AgentUnknown {
			found = append(found, a)
		}
	}
	if len(found) != 1 {
		return Agent{}, panes, false
	}
	return found[0], panes, true
}

// sendCommand sends cmd by route to the workspace: the agent's reset, its
// model, and the command, each after its check (the design, 3.1.3).
// Contract: when a check fails before the first input nothing is typed and
// the result is OutcomeNotSent; once an input is typed, a failure is never
// retried: the workspace is stopped and the result is
// OutcomeSentThenFailed; the start record is written before the first input
// and the end record after the last.
func (s sendContext) sendCommand(ctx context.Context, cmd paneinput.Command, route paneinput.Route) ActionResult {
	text, _ := cmd.Text()
	model, _ := route.ModelText()
	res := ActionResult{Workspace: s.ws.Worktree, Input: InputCommand, Command: text, Agent: route.Agent(), Model: model}
	target, panes, ok := targetPane(s.ws, route.Agent())
	if !ok {
		res.Outcome, res.Reason, res.Panes = OutcomeNotSent, NotSentPaneAmbiguous, panes
		res.Detail = "the workspace has no single pane of the route's agent"
		return res
	}
	res.Pane = target.Pane.String()
	typeCmd, err := paneinput.TypeCommand(cmd)
	if err != nil {
		return notSent(res, NotSentUnreadable, "the command cannot be typed: "+err.Error())
	}
	var steps []step
	first := paneCheck{}
	switch route.Agent() {
	case paneinput.AgentClaudeCode:
		switchModel, err := paneinput.SwitchModel(route.ClaudeModel())
		if err != nil {
			return notSent(res, NotSentUnreadable, "the route's model cannot be typed: "+err.Error())
		}
		steps = []step{{StepClear, paneinput.ClearSession()}, {StepModel, switchModel}, {StepCommand, typeCmd}}
		first.transcript = true
	case paneinput.AgentOpenCode:
		steps = []step{{StepNew, paneinput.NewSession()}, {StepCommand, typeCmd}}
		first.model = route.OpenCodeModel()
	default:
		return notSent(res, NotSentUnreadable, "the route has no agent")
	}
	return s.run(ctx, res, target, steps, first, route)
}

// notSent fills a result for nothing typed.
func notSent(res ActionResult, reason NotSentReason, detail string) ActionResult {
	res.Outcome, res.Reason, res.Detail = OutcomeNotSent, reason, detail
	return res
}

// run types steps into target after checking it before each, waiting for
// each to finish.
func (s sendContext) run(ctx context.Context, res ActionResult, target Agent, steps []step, first paneCheck, route paneinput.Route) ActionResult {
	agent, failed := s.checkPane(ctx, target, first)
	if failed != nil {
		res = notSent(res, failed.reason, failed.detail)
		res.Left, res.Status = failed.left, failed.agent.Status
		return res
	}
	var prompts []string
	for _, st := range steps {
		line, err := st.prompt.Text()
		if err != nil {
			return notSent(res, NotSentUnreadable, "an input cannot be typed: "+err.Error())
		}
		prompts = append(prompts, line)
	}
	id, err := s.held.BeginSend(SendRecord{Pane: res.Pane, Input: res.Input.String(), Command: res.Command, Agent: agentLabel(res.Agent), Model: res.Model, Prompts: prompts}, s.deps.Now())
	if err != nil {
		return notSent(res, NotSentUnreadable, "the send record cannot be written: "+err.Error())
	}
	last := StepUnknown
	for _, st := range steps {
		if last != StepUnknown {
			check := paneCheck{session: agent.Session}
			if st.kind == StepCommand && route.Agent() == paneinput.AgentOpenCode {
				check.model = route.OpenCodeModel()
			}
			agent, failed = s.checkPane(ctx, agent, check)
			if failed != nil {
				return s.fail(res, id, last, failed)
			}
		}
		if err := s.writer.Prompt(ctx, agent, st.prompt); err != nil {
			if errors.Is(err, errWriteRefused) && last == StepUnknown {
				// The writer refused before running herdr: nothing reached
				// the pane.
				res = notSent(res, NotSentUnreadable, err.Error())
				if eerr := s.held.EndSend(id, res.Outcome.String(), err.Error(), s.deps.Now()); eerr != nil {
					res.Detail += "; the end record cannot be written, so the workspace will stop: " + eerr.Error()
				}
				return res
			}
			// Herdr may have typed the input before it failed, so a failed
			// write counts as typed.
			return s.fail(res, id, st.kind, &checkFailure{reason: NotSentUnreadable, detail: "herdr agent prompt failed: " + err.Error(), agent: agent})
		}
		agent, failed = s.await(ctx, agent, st, route)
		if failed != nil {
			return s.fail(res, id, st.kind, failed)
		}
		last = st.kind
	}
	res.Outcome = OutcomeSent
	if err := s.held.EndSend(id, res.Outcome.String(), "", s.deps.Now()); err != nil {
		// The start without an end would stop the workspace at the next
		// settle; stop it now so the record and the result agree.
		res.Detail = "sent, but the end record cannot be written, so the workspace is stopped: " + err.Error()
		_ = s.held.Stop(res.Detail, "", id, s.deps.Now())
	}
	return res
}

// fail records a failure after an input: the stop first, then the end, so a
// send that ended has its stop written. When the stop cannot be written the
// end is not written either, and the start alone stops the workspace at the
// next settle.
func (s sendContext) fail(res ActionResult, id string, last SentStep, f *checkFailure) ActionResult {
	res.Outcome, res.Reason, res.Step, res.Detail = OutcomeSentThenFailed, f.reason, last, f.detail
	res.Left, res.Status, res.Screen = f.left, f.agent.Status, f.screen
	reason := fmt.Sprintf("after %s: %s: %s", last, f.reason, f.detail)
	if err := s.held.Stop(reason, f.screen, id, s.deps.Now()); err != nil {
		res.Detail += "; the stop record cannot be written: " + err.Error()
		return res
	}
	if err := s.held.EndSend(id, res.Outcome.String(), reason, s.deps.Now()); err != nil {
		res.Detail += "; the end record cannot be written: " + err.Error()
	}
	return res
}

// await waits until the input of st finished: /clear gave the pane a new
// session, /model is recorded in the new session's transcript with the
// route's model, /new shows a new session, and the command made the agent
// start. Contract: a wait that runs out is a failure, never success.
func (s sendContext) await(ctx context.Context, before Agent, st step, route paneinput.Route) (Agent, *checkFailure) {
	switch st.kind {
	case StepClear:
		return s.poll(ctx, before, "the session did not change after /clear", func(a Agent, _ string) bool {
			return a.Session != "" && a.Session != before.Session
		})
	case StepNew:
		return s.poll(ctx, before, "the pane shows no new session after /new", func(a Agent, raw string) bool {
			return readScreen(a.Kind, raw).NewSession
		})
	case StepModel:
		return s.awaitModel(ctx, before, route.ClaudeModel())
	case StepCommand:
		return s.poll(ctx, before, "the agent did not start the command", func(a Agent, _ string) bool {
			return a.Status == AgentWorking || a.Status == AgentBlocked || a.StateChangeSeq != before.StateChangeSeq
		})
	default:
		return before, &checkFailure{reason: NotSentUnreadable, detail: "an input of no known kind", agent: before}
	}
}

// poll reads the pane until done holds, up to pollAttempts times.
func (s sendContext) poll(ctx context.Context, before Agent, why string, done func(Agent, string) bool) (Agent, *checkFailure) {
	last := before
	var raw string
	for range pollAttempts {
		if err := s.deps.Sleep(ctx, pollInterval); err != nil {
			return last, &checkFailure{reason: NotSentNoStep, detail: err.Error(), agent: last}
		}
		a, err := s.herdr.Agent(ctx, before.Pane)
		if err != nil {
			continue
		}
		if a.Workspace != before.Workspace || a.Kind != before.Kind {
			return a, &checkFailure{reason: NotSentUnreadable, detail: "the pane is no longer the same agent of the workspace", agent: a}
		}
		last = a
		raw, err = s.herdr.Screen(ctx, a.Pane, ScreenVisible)
		if err != nil {
			continue
		}
		if done(a, raw) {
			return a, nil
		}
	}
	summary := screenSummary(raw)
	return last, &checkFailure{reason: NotSentNoStep, detail: why, agent: last, screen: summary}
}

// awaitModel reads the new session's transcript until its /model record
// names want, up to transcriptReads times. A record of another model fails
// at once: the pane runs another model than the route's.
func (s sendContext) awaitModel(ctx context.Context, before Agent, want paneinput.ClaudeModel) (Agent, *checkFailure) {
	path, err := transcriptPath(s.deps.Env, before.Dir, before.Session)
	if err != nil {
		return before, &checkFailure{reason: NotSentNoTranscript, detail: err.Error(), agent: before}
	}
	why := "no /model record in the transcript"
	for range transcriptReads {
		if err := s.deps.Sleep(ctx, transcriptInterval); err != nil {
			return before, &checkFailure{reason: NotSentNoTranscript, detail: err.Error(), agent: before}
		}
		data, err := readTranscript(path)
		if err != nil {
			why = err.Error()
			continue
		}
		got := readModelSwitch(data)
		switch {
		case got.Kind == SwitchFound && got.Model == want:
			return before, nil
		case got.Kind == SwitchFound:
			return before, &checkFailure{reason: NotSentModelMismatch, detail: "the transcript records a switch to another model", agent: before}
		case got.Kind == SwitchUnreadable:
			why = "a line of the transcript cannot be read"
		}
	}
	return before, &checkFailure{reason: NotSentNoTranscript, detail: why, agent: before}
}

// errNoChiefPane reports a chief pane Herdr does not list.
var errNoChiefPane = errors.New("the chief pane is not among Herdr's agents")

// chiefWorkspaceOf returns the workspace chief's pane is in.
func chiefWorkspaceOf(agents []Agent, chiefPane PaneID) (Agent, error) {
	i := slices.IndexFunc(agents, func(a Agent) bool { return a.Pane == chiefPane })
	if i < 0 {
		return Agent{}, fmt.Errorf("%w: %s", errNoChiefPane, chiefPane)
	}
	return agents[i], nil
}
