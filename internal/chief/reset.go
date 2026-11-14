package chief

import (
	"context"
	"fmt"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// prMakerRoute returns the route of the command that created the workspace's
// PR at url: the /runplan route of the plan PR whose created checkbox names
// url on the branch, or the task-document route for any other PR of a task
// branch (/mkreq, /mkarch, /mkplan, /mkplan2 create those). Contract: the
// kind is read from the branch's own plan and never from a screen; a plan
// that cannot be read, and a plan PR whose model requirement has no route,
// give RouteUnknown, so no pane is emptied.
func prMakerRoute(ctx context.Context, tasks Tasks, worktree string, docs *TaskDocs, old OldValues, oldErr error, url string) (paneinput.RouteKey, string) {
	if docs == nil {
		return paneinput.RouteUnknown, "the branch names no task"
	}
	if docs.Dir == "" {
		return paneinput.RouteTaskDocuments, ""
	}
	present, err := tasks.onHead(ctx, worktree, docs.Dir+"/"+planFile)
	switch {
	case err != nil:
		return paneinput.RouteUnknown, "the plan on the branch cannot be listed: " + err.Error()
	case !present:
		return paneinput.RouteTaskDocuments, ""
	case oldErr != nil:
		return paneinput.RouteUnknown, "the old-value table cannot be read"
	}
	plan, err := tasks.BranchPlan(ctx, worktree, *docs, old)
	if err != nil {
		return paneinput.RouteUnknown, "the plan on the branch cannot be read: " + err.Error()
	}
	pr, ok := plan.CreatedAs(url)
	if !ok {
		return paneinput.RouteTaskDocuments, ""
	}
	key := routeKeyOf(pr.Requirement.Value)
	if key == paneinput.RouteUnknown {
		return key, fmt.Sprintf("the model requirement of PR-%s has no route", pr.Target)
	}
	return key, ""
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
// a screen; it is emptied only when it is not runprPane, the kept detail
// read the PR's report from it, its state has not changed since, and its
// screen still shows that report; every other case is OutcomeNotSent with
// its reason. The reset is checked and recorded like any send.
func (s sendContext) resetAfterRunpr(ctx context.Context, runprPane string, pr OpenPR, rec *detailRecord, key paneinput.RouteKey, why string, routing Routing) ActionResult {
	res := ActionResult{Workspace: s.ws.Worktree, Input: InputReset}
	if key == paneinput.RouteUnknown {
		return notSent(res, NotSentUnreadable, "the command that created the PR is not known: "+why)
	}
	route, err := routing.Route(key)
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
	if rec == nil || rec.Source != ReportFromScreen.String() || rec.Pane != res.Pane {
		return notSent(res, NotSentNoReport, "the PR detail kept no report from this pane")
	}
	a, err := s.herdr.Agent(ctx, candidate.Pane)
	if err != nil {
		return notSent(res, NotSentUnreadable, "herdr agent get failed: "+err.Error())
	}
	if a.StateChangeSeq != rec.Seq {
		return notSent(res, NotSentPaneChanged, "the pane changed state since its report was kept")
	}
	raw, err := s.herdr.Screen(ctx, a.Pane, reportSource(a))
	if err != nil {
		return notSent(res, NotSentUnreadable, "herdr agent read failed: "+err.Error())
	}
	if _, ok := readCommandReport(a.Kind, raw, pr.URL); !ok {
		return notSent(res, NotSentNoReport, "the pane no longer shows the PR's report")
	}
	return s.run(ctx, res, a, []step{st}, paneCheck{}, route)
}
