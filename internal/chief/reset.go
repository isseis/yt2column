package chief

import (
	"context"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// maker is the command that created a workspace's open PR: the route its
// agent comes from, and its name and argument as the transcript records
// them. An empty args means any non-empty argument (/mkreq's issue, which no
// document names).
type maker struct {
	route paneinput.RouteKey
	name  string
	args  string
}

// prMaker returns the command that created the workspace's open PR. Contract:
// while the PR is open it is not on origin/main, so the next command that
// origin/main's documents give the task (the rules of the status table) is
// the command that created it; a task branch whose task origin/main does not
// have yet was opened by /mkreq. When no next command is decided (a PR of
// /mkplan2 or of /runpr, documents that cannot be read, a route that cannot
// be read) there is no maker, so no pane is emptied.
func prMaker(f workspaceFacts) (maker, string) {
	if f.Docs != nil && f.Docs.Dir == "" && !f.Task.IsZero() {
		return maker{route: paneinput.RouteTaskDocuments, name: nameMkreq}, ""
	}
	next, _, why := decideNext(f)
	if next == nil || next.Command == nil {
		if why == "" && next != nil {
			why = next.Problem
		}
		return maker{}, "no command of the documents created the PR: " + why
	}
	text, err := next.Command.Text()
	if err != nil {
		return maker{}, err.Error()
	}
	name, args, _ := strings.Cut(strings.TrimPrefix(text, "/"), " ")
	return maker{route: next.Command.Route(), name: name, args: args}, ""
}

// whyTranscriptLine is why a transcript with a line that cannot be read
// is not used.
const whyTranscriptLine = "a line of the transcript cannot be read"

// lastInputMadePR checks that in, the last line the human typed into the
// candidate's Claude Code session, is the command that created the PR, its
// name and argument compared whole: a pane the human went on using after
// that command, even with another command of the same task, is never
// emptied. It returns NotSentUnknown when the check passes.
func lastInputMadePR(in HumanInput, made maker) (NotSentReason, string) {
	switch {
	case in.Kind == HumanInputUnreadable:
		return NotSentNoTranscript, whyTranscriptLine
	case in.Kind != HumanInputCommand || in.Name != made.name:
		return NotSentOtherInput, "the session's last human input is not the command that created the PR"
	case made.args == "" && in.Args == "", made.args != "" && in.Args != made.args:
		return NotSentOtherInput, "the session's last command has another argument than the command that created the PR"
	}
	return NotSentUnknown, ""
}

// paneLastInputMadePR reads the last line the human typed into the session
// of the candidate a and checks it with lastInputMadePR. An OpenCode pane
// records no human input, so it never passes. It returns NotSentUnknown
// when the check passes.
func (s sendContext) paneLastInputMadePR(a Agent, made maker) (NotSentReason, string) {
	if a.Kind != paneinput.AgentClaudeCode {
		return NotSentNoReport, "only a Claude Code session records its human input"
	}
	in, reason, detail := s.lastHumanInput(a)
	if reason != NotSentUnknown {
		return reason, detail
	}
	return lastInputMadePR(in, made)
}

// lastInputRanRunpr checks that the last line the human typed into a
// Claude Code pane's session is /runpr, by its name alone: the human may
// call /runpr with a PR, a branch, or a task. It returns NotSentUnknown when
// the check passes.
func (s sendContext) lastInputRanRunpr(a Agent) (NotSentReason, string) {
	in, reason, detail := s.lastHumanInput(a)
	switch {
	case reason != NotSentUnknown:
		return reason, detail
	case in.Kind != HumanInputCommand || in.Name != nameRunpr:
		return NotSentOtherInput, "the session's last human input is not /runpr"
	}
	return NotSentUnknown, ""
}

// lastHumanInput reads the last line the human typed into the session of a
// Claude Code pane a. It returns NotSentUnknown when the transcript was
// read, and otherwise why not.
func (s sendContext) lastHumanInput(a Agent) (HumanInput, NotSentReason, string) {
	path, err := transcriptPath(s.deps.Env, a.Dir, a.Session)
	if err != nil {
		reason, detail := transcriptFailure(err)
		return HumanInput{}, reason, detail
	}
	data, err := readTranscript(path, sessionTranscriptLimit)
	if err != nil {
		return HumanInput{}, NotSentNoTranscript, err.Error()
	}
	in := readLastHumanInput(data)
	if in.Kind == HumanInputUnreadable {
		return HumanInput{}, NotSentNoTranscript, whyTranscriptLine
	}
	return in, NotSentUnknown, ""
}

// resetPrompt returns the input that empties a pane of agent and its step.
func resetPrompt(agent paneinput.AgentKind) (step, bool) {
	switch agent {
	case paneinput.AgentClaudeCode:
		return step{StepClear, paneinput.ClearSession()}, true
	case paneinput.AgentOpenCode:
		return step{StepNew, paneinput.NewSession()}, true
	default:
		return step{}, false
	}
}

// whyOpenCodeMaker is why a reset after /runpr of an OpenCode pane is not
// offered.
const whyOpenCodeMaker = "an OpenCode pane records no human input, so chief cannot tell that the command ran there"

// skipKind is why a reset was skipped with NotSentNoReport. The zero value
// is not offered.
type skipKind int

const (
	// skipNotOffered is any other skip, or a reset that was sent: a report
	// was read and was another one, or no reset is offered for the pane.
	skipNotOffered skipKind = iota
	// skipReportAbsent is a skip because no report was found or could be
	// read, which may be offered to the human.
	skipReportAbsent
)

// resetSkip is what a reset that was not sent leaves for the offer: the kind
// of the skip and, for skipReportAbsent, the candidate pane as listed under
// the workspace's exclusion, the command that ran in it, and, when set, the
// state_change_seq the pane must still have; why says, for a skip that is
// not offered, why not.
type resetSkip struct {
	kind    skipKind
	why     string
	pane    Agent
	command ranCommand
	seq     *int64
}

// notOffered is a skip that is not offered, with why.
func notOffered(why string) resetSkip { return resetSkip{kind: skipNotOffered, why: why} }

// resetAfterRunpr empties the pane of the command that created the PR, after
// /runpr went to runprPane (the design, 3.1.3). Contract: the candidate is
// chosen by the routing table (the one pane of the maker's agent), never by
// a screen; it is emptied only when it is not runprPane, the kept record
// read the PR's report from it, its state has not changed since, and its
// session's transcript, read once after that state, still ends in a last
// report, has a PR record naming the PR, and has the command that created
// the PR as the human's last input; every other case is OutcomeNotSent with
// its reason, and a transcript that cannot be read is never replaced by the
// screen. The reset is checked and recorded like any send. A skip with
// NotSentNoReport is skipReportAbsent only where no report was found on a
// Claude Code candidate: the detail kept the PR description because no pane
// showed the report, or the candidate's transcript holds no last report.
func (s sendContext) resetAfterRunpr(ctx context.Context, runprPane string, pr OpenPR, rec *detailRecord, made maker, why string, routing Routing) (ActionResult, resetSkip) {
	res := ActionResult{Workspace: s.ws.Worktree, Input: InputReset}
	if made.route == paneinput.RouteUnknown {
		return notSent(res, NotSentUnreadable, "the command that created the PR is not known: "+why), resetSkip{}
	}
	route, err := routing.Route(made.route)
	if err != nil {
		return notSent(res, NotSentUnreadable, "the routing table has no route for the command that created the PR"), resetSkip{}
	}
	res.Agent = route.Agent()
	st, ok := resetPrompt(route.Agent())
	if !ok {
		return notSent(res, NotSentUnreadable, "the route has no agent"), resetSkip{}
	}
	res.Command, _ = st.prompt.Text()
	candidate, panes, ok := targetPane(s.ws, route.Agent())
	if !ok {
		res.Panes = panes
		return notSent(res, NotSentPaneAmbiguous, "the workspace has no single pane of the agent that created the PR"), resetSkip{}
	}
	res.Pane = candidate.Pane.String()
	if res.Pane == runprPane {
		return notSent(res, NotSentSamePane, "the pane that created the PR runs /runpr now"), resetSkip{}
	}
	// absent is the skip of a candidate that shows no report.
	absent := func() resetSkip {
		if candidate.Kind != paneinput.AgentClaudeCode {
			return notOffered(whyOpenCodeMaker)
		}
		return resetSkip{kind: skipReportAbsent, pane: candidate, command: ranCommand{kind: commandKinds[made.name], argument: made.args, route: made.route}}
	}
	if rec == nil || rec.Source != ReportFromPane.String() || rec.Pane != res.Pane {
		return notSent(res, NotSentNoReport, "the PR detail kept no report from this pane"), detailSkip(rec, absent)
	}
	a, err := s.herdr.Agent(ctx, candidate.Pane)
	if err != nil {
		return notSent(res, NotSentUnreadable, "herdr agent get failed: "+err.Error()), resetSkip{}
	}
	if !sameAgent(a, candidate) {
		return notSent(res, NotSentUnreadable, "the pane is no longer the same agent of the workspace"), resetSkip{}
	}
	if a.StateChangeSeq != rec.Seq {
		return notSent(res, NotSentPaneChanged, "the pane changed state since its report was kept"), resetSkip{}
	}
	if a.Kind != paneinput.AgentClaudeCode {
		return notSent(res, NotSentNoReport, "only a Claude Code session records its last report and human input"), notOffered(whyOpenCodeMaker)
	}
	cr := readClaudeReport(s.deps.Env, a)
	switch cr.Report.Kind {
	case SessionReportFound:
	case SessionReportAbsent:
		// The pane must not move between the transcript read and the offer.
		skip := absent()
		skip.seq = &rec.Seq
		return notSent(res, NotSentNoReport, "the pane has no last report: "+cr.Detail), skip
	default:
		return notSent(res, cr.Reason, "the transcript of the pane cannot be read: "+cr.Detail), notOffered("the transcript of the pane cannot be read")
	}
	if !cr.Report.Names(pr.Number, pr.URL) {
		return notSent(res, NotSentNoReport, "the pane's session has no PR record naming the PR"), notOffered("the pane's last report is another one: its session has no PR record naming the PR")
	}
	if reason, detail := lastInputMadePR(cr.Report.LastHuman, made); reason != NotSentUnknown {
		return notSent(res, reason, detail), resetSkip{}
	}
	seq := a.StateChangeSeq
	return s.run(ctx, res, a, []step{st}, paneCheck{session: a.Session, seq: &seq}, route, nil), resetSkip{}
}

// detailSkip is the skip of a reset whose kept PR detail has no report from
// the candidate: only a detail that kept the PR description, because no
// pane showed the report, is a report not found (absent's skip).
func detailSkip(rec *detailRecord, absent func() resetSkip) resetSkip {
	switch {
	case rec == nil:
		return notOffered("no PR detail was kept")
	case rec.Source == ReportFromDescription.String():
		return absent()
	case rec.Source == ReportFromPane.String():
		return notOffered("the PR detail kept the report from another pane")
	default:
		return notOffered("the PR detail holds no report")
	}
}

// The notes offerSkip adds to the detail of a reset skipped with
// NotSentNoReport: the wording says the human what came of the skip.
const (
	skipOfferedNote    = "; offered to the human as a reset item"
	skipNotOfferedNote = "; not offered to the human: "
	skipUnwrittenNote  = "; the skip record cannot be written, so it is not offered to the human: "
)

// offerSkip keeps a reset skipped with NotSentNoReport for the human to
// choose, and says in res's detail what came of it (whether it is offered,
// why not, or that its record could not be written). Contract: only a
// skipReportAbsent skip is kept, and only when the candidate, read again,
// is still the same agent of the workspace waiting for input and passes
// check, the scene's own conditions; the result's outcome and reason are
// never changed.
func (s sendContext) offerSkip(ctx context.Context, res ActionResult, skip resetSkip, scene skipScene, check func(Agent) string) ActionResult {
	if res.Outcome != OutcomeNotSent || res.Reason != NotSentNoReport {
		return res
	}
	not := func(why string) ActionResult {
		res.Detail += skipNotOfferedNote + why
		return res
	}
	if skip.kind != skipReportAbsent {
		return not(skip.why)
	}
	a, err := s.herdr.Agent(ctx, skip.pane.Pane)
	switch {
	case err != nil:
		return not("herdr agent get failed: " + err.Error())
	case !sameAgent(a, skip.pane):
		return not("the pane is no longer the same agent of the workspace")
	case a.Status != AgentIdle && a.Status != AgentDone:
		return not("the pane's agent is not waiting for input")
	case skip.seq != nil && a.StateChangeSeq != *skip.seq:
		return not("the pane changed state since it was read")
	}
	// An idle agent may still show a question; the pane must wait at its
	// prompt.
	raw, err := s.herdr.Screen(ctx, a.Pane, ScreenVisible)
	switch {
	case err != nil:
		return not("herdr agent read failed: " + err.Error())
	case readScreen(a.Kind, raw).Kind != ScreenPrompt:
		return not("the pane does not wait at its prompt")
	}
	if why := check(a); why != "" {
		return not(why)
	}
	rec, err := newSkipRecord(s.ws.ID, a, scene, skip.command, s.deps.Now())
	if err != nil {
		return not(err.Error())
	}
	if err := s.held.recordSkip(rec); err != nil {
		res.Detail += skipUnwrittenNote + err.Error()
		return res
	}
	res.Detail += skipOfferedNote
	return res
}
