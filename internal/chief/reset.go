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

// lastInputMadePR checks that in, the last line the human typed into the
// candidate's Claude Code session, is the command that created the PR, its
// name and argument compared whole: a pane the human went on using after
// that command, even with another command of the same task, is never
// emptied. It returns NotSentUnknown when the check passes.
func lastInputMadePR(in HumanInput, made maker) (NotSentReason, string) {
	switch {
	case in.Kind == HumanInputUnreadable:
		return NotSentNoTranscript, "a line of the transcript cannot be read"
	case in.Kind != HumanInputCommand || in.Name != made.name:
		return NotSentOtherInput, "the session's last human input is not the command that created the PR"
	case made.args == "" && in.Args == "", made.args != "" && in.Args != made.args:
		return NotSentOtherInput, "the session's last command has another argument than the command that created the PR"
	}
	return NotSentUnknown, ""
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

// resetAfterRunpr empties the pane of the command that created the PR, after
// /runpr went to runprPane (the design, 3.1.3). Contract: the candidate is
// chosen by the routing table (the one pane of the maker's agent), never by
// a screen; it is emptied only when it is not runprPane, the kept record
// read the PR's report from it, its state has not changed since, and its
// session's transcript, read once after that state, still ends in a last
// report, has a PR record naming the PR, and has the command that created
// the PR as the human's last input; every other case is OutcomeNotSent with
// its reason, and a transcript that cannot be read is never replaced by the
// screen. The reset is checked and recorded like any send.
func (s sendContext) resetAfterRunpr(ctx context.Context, runprPane string, pr OpenPR, rec *detailRecord, made maker, why string, routing Routing) ActionResult {
	res := ActionResult{Workspace: s.ws.Worktree, Input: InputReset}
	if made.route == paneinput.RouteUnknown {
		return notSent(res, NotSentUnreadable, "the command that created the PR is not known: "+why)
	}
	route, err := routing.Route(made.route)
	if err != nil {
		return notSent(res, NotSentUnreadable, "the routing table has no route for the command that created the PR")
	}
	res.Agent = route.Agent()
	st, ok := resetPrompt(route.Agent())
	if !ok {
		return notSent(res, NotSentUnreadable, "the route has no agent")
	}
	res.Command, _ = st.prompt.Text()
	candidate, panes, ok := targetPane(s.ws, route.Agent())
	if !ok {
		res.Panes = panes
		return notSent(res, NotSentPaneAmbiguous, "the workspace has no single pane of the agent that created the PR")
	}
	res.Pane = candidate.Pane.String()
	if res.Pane == runprPane {
		return notSent(res, NotSentSamePane, "the pane that created the PR runs /runpr now")
	}
	if rec == nil || rec.Source != ReportFromPane.String() || rec.Pane != res.Pane {
		return notSent(res, NotSentNoReport, "the PR detail kept no report from this pane")
	}
	a, err := s.herdr.Agent(ctx, candidate.Pane)
	if err != nil {
		return notSent(res, NotSentUnreadable, "herdr agent get failed: "+err.Error())
	}
	if !sameAgent(a, candidate) {
		return notSent(res, NotSentUnreadable, "the pane is no longer the same agent of the workspace")
	}
	if a.StateChangeSeq != rec.Seq {
		return notSent(res, NotSentPaneChanged, "the pane changed state since its report was kept")
	}
	if a.Kind != paneinput.AgentClaudeCode {
		return notSent(res, NotSentNoReport, "only a Claude Code session records its last report and human input")
	}
	cr := readClaudeReport(s.deps.Env, a)
	switch cr.Report.Kind {
	case SessionReportFound:
	case SessionReportAbsent:
		return notSent(res, NotSentNoReport, "the pane has no last report: "+cr.Detail)
	default:
		return notSent(res, cr.Reason, "the transcript of the pane cannot be read: "+cr.Detail)
	}
	if !cr.Report.Names(pr.Number, pr.URL) {
		return notSent(res, NotSentNoReport, "the pane's session has no PR record naming the PR")
	}
	if reason, detail := lastInputMadePR(cr.Report.LastHuman, made); reason != NotSentUnknown {
		return notSent(res, reason, detail)
	}
	seq := a.StateChangeSeq
	return s.run(ctx, res, a, []step{st}, paneCheck{session: a.Session, seq: &seq}, route)
}
