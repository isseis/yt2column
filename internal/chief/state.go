package chief

import (
	"fmt"
	"slices"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// WorkspaceState is the state of one task workspace. The zero value is
// StateNeedsCheck, the state that assumes least.
type WorkspaceState int

const (
	// StateNeedsCheck is a workspace chief cannot read or classify.
	StateNeedsCheck WorkspaceState = iota
	// StateQuestion is a workspace with an agent stopped on a question.
	StateQuestion
	// StateRunning is a workspace with a working agent.
	StateRunning
	// StateAwaitingReview is a workspace whose branch has an open PR.
	StateAwaitingReview
	// StateProposeNext is a workspace with a next command to offer.
	StateProposeNext
	// StateTaskDone is a workspace whose task is complete.
	StateTaskDone
)

// String returns the name the output uses for the state.
func (s WorkspaceState) String() string {
	switch s {
	case StateQuestion:
		return nameQuestion
	case StateRunning:
		return "running"
	case StateAwaitingReview:
		return "awaiting_review"
	case StateProposeNext:
		return "propose_next"
	case StateTaskDone:
		return "task_done"
	default:
		return "needs_check"
	}
}

// rank is the position of a state in the order rows are listed: the states
// that wait on the human first, then running and done.
func (s WorkspaceState) rank() int {
	i := slices.Index(stateOrder, s)
	if i < 0 {
		return len(stateOrder)
	}
	return i
}

// stateOrder is the order rows are listed in.
var stateOrder = []WorkspaceState{StateQuestion, StateAwaitingReview, StateProposeNext, StateNeedsCheck, StateRunning, StateTaskDone}

// TaskNextKind is the kind of the next command of a task.
type TaskNextKind int

const (
	// TaskNextNone is no next command.
	TaskNextNone TaskNextKind = iota
	// TaskNextMkreq is /mkreq; the human names the issue.
	TaskNextMkreq
	// TaskNextMkarch is /mkarch <task>.
	TaskNextMkarch
	// TaskNextMkplan is /mkplan <task>.
	TaskNextMkplan
	// TaskNextRunplan is /runplan <task>.
	TaskNextRunplan
	// TaskNextRunplanTakeover is /runplan <task> handed to Claude Code after an
	// OpenCode run stopped before it opened the PR.
	TaskNextRunplanTakeover
	// TaskNextRunpr is /runpr <PR>, offered with a PR's detail.
	TaskNextRunpr
)

// Next is a command chief can offer for a workspace. Command is nil when
// none can be sent: /mkreq, which needs the human's issue number, or a
// command whose route is not readable (Problem says why).
type Next struct {
	Kind    TaskNextKind
	Command *paneinput.Command
	// Text is the command line as the human reads it; /mkreq's is "/mkreq".
	Text string
	// Agent and Model are the route's agent and model for display, empty
	// when no route is known.
	Agent paneinput.AgentKind
	Model string
	// Key distinguishes this proposal from the same command line proposed
	// again later (the target of the next plan PR); it is part of the
	// item's identity, never shown.
	Key string
	// Problem is why Command is nil although a command is due; empty
	// otherwise.
	Problem string
}

// QuestionItem is one agent's question as read from its screen.
type QuestionItem struct {
	Pane     string
	Agent    paneinput.AgentKind
	Question Question
}

// Row is one task workspace as the status table shows it.
type Row struct {
	// Workspace is the canonical worktree path, the identity the other
	// subcommands take.
	Workspace string
	Label     string
	// Task is the task number from the branch name, empty when none.
	Task  string
	State WorkspaceState
	Next  *Next
	// Takeover is the proposal of /runplan handed to Claude Code (the
	// runplan takeover), set only when every condition of it holds.
	Takeover *Next
	// Unreadable names what could not be read or matched when State is
	// StateNeedsCheck.
	Unreadable    []string
	MergeQuestion bool
	Questions     []QuestionItem
	PR            *OpenPR
	// Running is chief's last send to a working pane, nil when chief sent
	// it nothing; it is shown only, never read to decide.
	Running *SendRecord
	// Stopped is the workspace's stop, nil when it takes input.
	Stopped *StopRecord
}

// agentFacts is one agent with the reading of its screen.
type agentFacts struct {
	Agent Agent
	// ScreenRead is false when the screen could not be fetched.
	ScreenRead bool
	Screen     Screen
}

// workspaceFacts is everything classify needs about one workspace, read
// before it runs; classify itself reads nothing.
type workspaceFacts struct {
	Workspace string
	Label     string
	Agents    []agentFacts
	// Unreadable collects what the readers could not read (git, gh, task
	// documents), each as a short name.
	Unreadable []string
	// Head is what the worktree's HEAD is; a task is read only from a branch.
	Head headState
	// Task is the task of the branch, zero when the branch names none.
	Task paneinput.TaskID
	// Docs is origin/main's documents of the task, nil when there is no task.
	Docs *TaskDocs
	PR   *OpenPR
	// Routing is the routing table, RoutingErr why it could not be read.
	Routing    Routing
	RoutingErr error
	// FetchFailed is true when this round's `git fetch origin main` failed,
	// so origin/main may be stale.
	FetchFailed bool
	// Takeover holds what the takeover proposal needs, read only when its
	// other conditions hold.
	Takeover takeoverFacts
}

// headState is what a worktree's HEAD is. The zero value is headUnread, the
// reading that assumes least: no next command is decided from it.
type headState int

const (
	// headUnread is a HEAD that was not read.
	headUnread headState = iota
	// headBranch is a checked-out branch.
	headBranch
	// headDetached is a detached HEAD. /runpr leaves one both after a task's
	// final PR and when it stopped before creating the next branch of the
	// task, so the worktree names no task and cannot be told to have none.
	headDetached
)

// takeoverFacts are the reads behind the takeover proposal.
type takeoverFacts struct {
	// Read is true when the facts below were read.
	Read bool
	// Ahead is the number of commits HEAD has beyond origin/main.
	Ahead int
	// StepsDone is true when the branch's plan marks every step of the next
	// PR done.
	StepsDone bool
}

// routeKeyOf maps a model requirement reading to the route of its /runplan.
// A reading with no route (absent or unreadable) gives RouteUnknown.
func routeKeyOf(r Requirement) paneinput.RouteKey {
	switch r {
	case RequirementOpus:
		return paneinput.RouteRunplanOpus
	case RequirementSonnetReview:
		return paneinput.RouteRunplanSonnetReview
	case RequirementSonnet:
		return paneinput.RouteRunplanSonnet
	default:
		return paneinput.RouteUnknown
	}
}

// makeNext builds a Next for a command and its route, or a Next with a
// Problem when the route cannot be read.
func makeNext(kind TaskNextKind, cmd paneinput.Command, key paneinput.RouteKey, routing Routing, routingErr error) *Next {
	text, err := cmd.Text()
	if err != nil {
		return &Next{Kind: kind, Problem: "the command cannot be built: " + err.Error()}
	}
	next := &Next{Kind: kind, Text: text}
	if routingErr != nil {
		next.Problem = "the routing table cannot be read"
		return next
	}
	route, err := routing.Route(key)
	if err != nil {
		next.Problem = "the routing table has no route for the command"
		return next
	}
	model, err := route.ModelText()
	if err != nil {
		next.Problem = "the route's model cannot be read"
		return next
	}
	next.Command = &cmd
	next.Agent = route.Agent()
	next.Model = model
	return next
}

// headProblem returns why no next command can be decided from a HEAD, empty
// for a checked-out branch, the only HEAD a task is read from.
func headProblem(h headState) string {
	switch h {
	case headBranch:
		return ""
	case headDetached:
		return "HEAD is detached, so no branch tells whether the worktree has a task"
	case headUnread:
		return "the HEAD of the worktree was not read"
	default:
		return fmt.Sprintf("the HEAD of the worktree is of no known kind: %d", h)
	}
}

// decideNext decides the next command of a task from origin/main's documents
// (rules of the design, 3.1.4). Contract: the first matching rule wins and
// none matching returns TaskNextNone with a reason in why; it never guesses a
// command from a document it could not read.
func decideNext(f workspaceFacts) (next *Next, done bool, why string) {
	if why := headProblem(f.Head); why != "" {
		return nil, false, why
	}
	if f.Task.IsZero() {
		return &Next{Kind: TaskNextMkreq, Text: "/mkreq"}, false, ""
	}
	docs := f.Docs
	if docs == nil || docs.Dir == "" || docs.Requirements == nil {
		return nil, false, "no requirements document of the task on origin/main"
	}
	req, arch, plan := docs.Requirements, docs.Architecture, docs.Plan
	switch {
	case plan != nil && plan.Status == DocApproved:
		return nextFromPlan(f, plan)
	case plan != nil:
		return nil, false, "the plan is not approved"
	case arch != nil && arch.Status == DocApproved:
		cmd, err := paneinput.NewMkplan(f.Task)
		if err != nil {
			return nil, false, "the /mkplan command cannot be built"
		}
		return makeNext(TaskNextMkplan, cmd, paneinput.RouteTaskDocuments, f.Routing, f.RoutingErr), false, ""
	case arch != nil:
		return nil, false, "the architecture document is not approved"
	case req.Status == DocApproved && (req.Process == ProcessFull || req.Process == ProcessUnrecorded):
		cmd, err := paneinput.NewMkarch(f.Task)
		if err != nil {
			return nil, false, "the /mkarch command cannot be built"
		}
		return makeNext(TaskNextMkarch, cmd, paneinput.RouteTaskDocuments, f.Routing, f.RoutingErr), false, ""
	case req.Status == DocApproved && req.Process == ProcessUnreadable:
		return nil, false, "the process record of the requirements document is unreadable"
	default:
		return nil, false, "no rule gives a next command"
	}
}

// nextFromPlan decides the next command from an approved plan: the task is
// done when every PR is merged, else the next PR to create picks the route of
// its /runplan by its model requirement.
func nextFromPlan(f workspaceFacts, plan *Plan) (next *Next, done bool, why string) {
	if plan.Done() {
		return nil, true, ""
	}
	pr, ok := plan.NextPR()
	if !ok {
		return nil, false, "the plan has no PR left to create and is not complete"
	}
	// /runpr offers no /runplan for a plan with no step left to do, so a
	// plan PR left to create without one is not guessed to be a /runplan.
	if !plan.StepsLeft() {
		return nil, false, "the plan has a PR left to create but no step left to do"
	}
	key := routeKeyOf(pr.Requirement.Value)
	if key == paneinput.RouteUnknown {
		problem := "the model requirement of the next PR is absent"
		if pr.Requirement.Value == RequirementUnreadable {
			problem = fmt.Sprintf("the model requirement of the next PR is unreadable: %q", pr.Requirement.Text)
		}
		return &Next{Kind: TaskNextRunplan, Text: "/runplan " + f.Task.String(), Problem: problem}, false, ""
	}
	cmd, err := paneinput.NewRunplan(f.Task, key)
	if err != nil {
		return nil, false, "the /runplan command cannot be built"
	}
	n := makeNext(TaskNextRunplan, cmd, key, f.Routing, f.RoutingErr)
	n.Key = pr.Target
	return n, false, ""
}

// needsTakeoverCheck reports whether the takeover proposal's other
// conditions hold, so its reads (the commits ahead and the branch's plan) are
// worth making: the next command is a /runplan whose route is OpenCode for a
// requirement whose review needs Claude Code, nothing is open or running,
// and a Claude Code agent waits for input.
func needsTakeoverCheck(f workspaceFacts, next *Next) bool {
	if next == nil || next.Kind != TaskNextRunplan || next.Command == nil || next.Agent != paneinput.AgentOpenCode {
		return false
	}
	if f.PR != nil || len(f.Unreadable) > 0 {
		return false
	}
	key := next.Command.Route()
	if key != paneinput.RouteRunplanOpus && key != paneinput.RouteRunplanSonnetReview {
		return false
	}
	return takeoverTargetReady(f.Agents)
}

// takeoverTargetReady reports whether every agent accepts input and one is a
// Claude Code agent.
func takeoverTargetReady(agents []agentFacts) bool {
	claude := false
	for _, a := range agents {
		if a.Agent.Status != AgentIdle && a.Agent.Status != AgentDone {
			return false
		}
		if a.Agent.Kind == paneinput.AgentClaudeCode {
			claude = true
		}
	}
	return claude
}

// classify decides a workspace's row from facts: the first state of the
// design's table (3.1.4) that matches. Contract: anything unread or matching
// no shape is StateNeedsCheck with its names in Unreadable, never another
// state guessed from the rest; the zero facts classify as StateNeedsCheck.
func classify(f workspaceFacts) Row {
	row := Row{Workspace: f.Workspace, Label: f.Label, PR: f.PR}
	if !f.Task.IsZero() {
		row.Task = f.Task.String()
	}
	unreadable := slices.Clone(f.Unreadable)
	for _, a := range f.Agents {
		switch {
		case !a.ScreenRead:
			unreadable = append(unreadable, "screen of pane "+a.Agent.Pane.String())
		case a.Screen.Kind == ScreenQuestion && a.Screen.Question != nil:
			row.Questions = append(row.Questions, QuestionItem{Pane: a.Agent.Pane.String(), Agent: a.Agent.Kind, Question: *a.Screen.Question})
			if a.Screen.Question.Merge {
				row.MergeQuestion = true
			}
		case a.Agent.Status == AgentWorking:
			// A working agent's screen is output in progress; it is not read
			// for a question.
		case a.Screen.Kind == ScreenPrompt && a.Agent.Status != AgentBlocked:
			// An agent that waits for input and shows its prompt.
		default:
			// Blocked without a known question shape, or a screen of no known
			// shape: whether a question waits cannot be told.
			unreadable = append(unreadable, "screen of pane "+a.Agent.Pane.String()+" matches no known shape")
		}
	}
	if len(unreadable) > 0 {
		row.State = StateNeedsCheck
		row.Unreadable = unreadable
		return row
	}
	switch {
	case len(row.Questions) > 0:
		row.State = StateQuestion
		return row
	case slices.ContainsFunc(f.Agents, func(a agentFacts) bool { return a.Agent.Status == AgentWorking }):
		row.State = StateRunning
		return row
	case f.PR != nil:
		row.State = StateAwaitingReview
		return row
	}
	next, done, why := decideNext(f)
	switch {
	case done:
		row.State = StateTaskDone
	case next == nil:
		row.State = StateNeedsCheck
		row.Unreadable = []string{why}
	case f.FetchFailed:
		row.State = StateNeedsCheck
		row.Unreadable = []string{"git fetch origin main failed, so the documents may be stale"}
	default:
		row.State = StateProposeNext
		row.Next = next
		row.Takeover = takeoverProposal(f, next)
	}
	return row
}

// takeoverProposal returns the proposal to hand a stopped /runplan to Claude
// Code, or nil when a condition of it does not hold. Every condition must
// hold: the next command's route (OpenCode for opus or sonnet+opus-review),
// commits beyond origin/main, no open PR, agents that accept input, and the
// branch's plan with every step of the PR done.
func takeoverProposal(f workspaceFacts, next *Next) *Next {
	if !needsTakeoverCheck(f, next) || !f.Takeover.Read || f.Takeover.Ahead == 0 || !f.Takeover.StepsDone {
		return nil
	}
	cmd, err := paneinput.NewRunplan(f.Task, paneinput.RouteRunplanTakeover)
	if err != nil {
		return nil
	}
	n := makeNext(TaskNextRunplanTakeover, cmd, paneinput.RouteRunplanTakeover, f.Routing, f.RoutingErr)
	n.Key = next.Key
	return n
}
