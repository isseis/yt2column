package chief

import (
	"slices"
	"strconv"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// Urgency is how soon the human should act on a row. The zero value is
// UrgencyNow, the reading that calls the human.
type Urgency int

const (
	// UrgencyNow is a row the human should act on now.
	UrgencyNow Urgency = iota
	// UrgencyLater is a row that waits on the human without a stop.
	UrgencyLater
	// UrgencyWait is a row that waits on no one the human has to answer.
	UrgencyWait
)

// Turn is who moves next on a row. The zero value is TurnHuman.
type Turn int

const (
	// TurnHuman is a row the human moves next.
	TurnHuman Turn = iota
	// TurnAI is a row an agent moves next.
	TurnAI
	// TurnNone is a row nobody moves.
	TurnNone
)

// PositionDocument is the document part of a task's position. The zero value
// is PositionDocNeedsCheck, the reading that assumes least.
type PositionDocument int

const (
	// PositionDocNeedsCheck is a position that cannot be told.
	PositionDocNeedsCheck PositionDocument = iota
	// PositionDocNoTask is a worktree with no task.
	PositionDocNoTask
	// PositionDocRequirements is a task at its requirements document.
	PositionDocRequirements
	// PositionDocArchitecture is a task at its architecture document.
	PositionDocArchitecture
	// PositionDocPlan is a task at its implementation plan.
	PositionDocPlan
	// PositionDocPlanPR is a task at a PR of its plan; Position.PlanPR is N.
	PositionDocPlanPR
	// PositionDocNone is a task past its documents: every PR is merged.
	PositionDocNone
)

// Progress is the progress part of a task's position. The zero value is
// ProgressNeedsCheck.
type Progress int

const (
	// ProgressNeedsCheck is a progress that cannot be told.
	ProgressNeedsCheck Progress = iota
	// ProgressAwaitingCreation is a next command waiting to be sent.
	ProgressAwaitingCreation
	// ProgressCreating is a command that creates a document or a PR running.
	ProgressCreating
	// ProgressRunprRunning is /runpr running.
	ProgressRunprRunning
	// ProgressRunning is a command running that is not decided.
	ProgressRunning
	// ProgressMergeConfirmation is /runpr's merge question waiting.
	ProgressMergeConfirmation
	// ProgressQuestion is another question waiting.
	ProgressQuestion
	// ProgressAwaitingReview is an open PR waiting for review.
	ProgressAwaitingReview
	// ProgressTaskDone is a completed task.
	ProgressTaskDone
)

// Position is a task's position: the document and the progress.
type Position struct {
	Document PositionDocument
	// PlanPR is N of PR-N, set only when Document is PositionDocPlanPR.
	PlanPR   int
	Progress Progress
}

// docStage is where origin/main's documents of a task stand. Next-command
// decision and the document rule both read it, so neither repeats the
// document judgment. The zero value is stageOther, the stage that assumes
// least.
type docStage int

const (
	// stageOther matches none of the stages below: the requirements document
	// is not approved, a simple-process task lacks its architecture document
	// or plan, or the documents were not read.
	stageOther docStage = iota
	// stageNoRequirements is no task directory or no requirements document.
	stageNoRequirements
	// stagePlanApproved is an approved plan.
	stagePlanApproved
	// stagePlanNotApproved is a plan that is not approved.
	stagePlanNotApproved
	// stageArchApproved is an approved architecture document and no plan.
	stageArchApproved
	// stageArchNotApproved is an architecture document that is not approved
	// and no plan.
	stageArchNotApproved
	// stageReqApprovedFull is an approved requirements document of the full
	// process (or with no process record) and no later document.
	stageReqApprovedFull
	// stageProcessUnreadable is an approved requirements document whose
	// process record is broken, and no later document.
	stageProcessUnreadable
)

// stageOf returns the stage of a task's documents. Contract: the first
// matching stage of the table in the design (3.3) wins, in the order the next
// command is decided; documents that were not read (nil) are
// stageNoRequirements, the stage of a task with none.
func stageOf(docs *TaskDocs) docStage {
	if docs == nil || docs.Dir == "" || docs.Requirements == nil {
		return stageNoRequirements
	}
	req, arch, plan := docs.Requirements, docs.Architecture, docs.Plan
	switch {
	case plan != nil && plan.Status == DocApproved:
		return stagePlanApproved
	case plan != nil:
		return stagePlanNotApproved
	case arch != nil && arch.Status == DocApproved:
		return stageArchApproved
	case arch != nil:
		return stageArchNotApproved
	case req.Status == DocApproved && (req.Process == ProcessFull || req.Process == ProcessUnrecorded):
		return stageReqApprovedFull
	case req.Status == DocApproved && req.Process == ProcessUnreadable:
		return stageProcessUnreadable
	default:
		return stageOther
	}
}

// positionFacts is what the document rule reads about a workspace; classify
// keeps it on the row.
type positionFacts struct {
	// fetchFailed is true when this round's git fetch origin main failed.
	fetchFailed bool
	// docsUnreadable is true when origin/main's task documents could not be
	// read.
	docsUnreadable bool
	head           headState
	// hasTask is true when the branch follows the feature branch convention.
	hasTask bool
	// stage is the stage of origin/main's documents, read only when hasTask
	// and the documents were read; stageOther otherwise.
	stage docStage
	// planDone is true when every PR of an approved plan is merged.
	planDone bool
	// planPR is N of the first plan PR not merged, for stagePlanApproved; 0
	// when it cannot be told.
	planPR int
}

// positionFactsOf reads the facts of a workspace for the document rule.
func positionFactsOf(f workspaceFacts) positionFacts {
	p := positionFacts{fetchFailed: f.FetchFailed, docsUnreadable: f.DocsUnreadable, head: f.Head, hasTask: !f.Task.IsZero()}
	if !p.hasTask || p.docsUnreadable || f.Docs == nil {
		return p
	}
	p.stage = stageOf(f.Docs)
	if p.stage == stagePlanApproved {
		p.planDone = f.Docs.Plan.Done()
		p.planPR = firstUnmergedPR(*f.Docs.Plan)
	}
	return p
}

// firstUnmergedPR returns N of the first PR of the plan whose merged box is
// not checked (1 for the single PR of a plan without markers), and 0 when
// every PR is merged or the number cannot be read.
func firstUnmergedPR(plan Plan) int {
	for _, pr := range plan.PRs {
		if pr.Merged {
			continue
		}
		if pr.Target == singleTarget {
			return 1
		}
		n, err := strconv.Atoi(pr.Target)
		if err != nil || n < 1 {
			return 0
		}
		return n
	}
	return 0
}

// urgencyOf maps a row to its urgency and turn. Contract: the first matching
// row of the table in the design (3.2) wins, with a removal row (marked by
// Row.RemovalRow) after the questions and before the idle rows; it reads
// only the row's declared values, never Unreadable's text; a state of none
// of the six matches no row and gets the zero values, which call the human.
func urgencyOf(r Row) (Urgency, Turn) {
	switch {
	case r.Stopped != nil || r.StopUnreadable:
		return UrgencyNow, TurnHuman
	case r.State == StateQuestion:
		return UrgencyNow, TurnHuman
	case r.RemovalRow:
		return UrgencyLater, TurnHuman
	case r.Idle:
		return UrgencyWait, TurnNone
	case r.State == StateAwaitingReview || r.State == StateProposeNext || r.State == StateNeedsCheck:
		return UrgencyLater, TurnHuman
	case r.State == StateRunning:
		return UrgencyWait, TurnAI
	case r.State == StateTaskDone:
		return UrgencyWait, TurnNone
	default:
		return UrgencyNow, TurnHuman
	}
}

// documentOf decides the document part of a row's position, and N of PR-N
// for PositionDocPlanPR, by the ten rules of the design (3.3), the first
// match winning, with a removal row (Row.RemovalRow) having no document
// right after the rule of a failed fetch or unreadable documents. running
// is the command the row's agents run, nil when none is decided. It never
// changes the row's state.
func documentOf(r Row, running *RunningCommand) (PositionDocument, int) {
	switch {
	case r.position.fetchFailed || r.position.docsUnreadable:
		return PositionDocNeedsCheck, 0
	case r.RemovalRow:
		return PositionDocNone, 0
	default:
		return taskDocumentOf(r, running)
	}
}

// taskDocumentOf applies the rules of documentOf that follow a successful
// read and a row that is not a removal row, in their order.
func taskDocumentOf(r Row, running *RunningCommand) (PositionDocument, int) {
	p := r.position
	runningMkreq := running != nil && running.Kind() == paneinput.CommandMkreq
	switch {
	case !p.hasTask && runningMkreq:
		return PositionDocRequirements, 0
	case !p.hasTask && p.head == headBranch && r.State == StateProposeNext && r.Next != nil && r.Next.Kind == TaskNextMkreq:
		return PositionDocRequirements, 0
	case !p.hasTask && p.head == headBranch:
		return PositionDocNoTask, 0
	case p.stage == stageNoRequirements:
		return PositionDocRequirements, 0
	case p.stage == stagePlanApproved && p.planDone:
		return PositionDocNone, 0
	case p.stage == stagePlanApproved && p.planPR > 0:
		return PositionDocPlanPR, p.planPR
	case p.stage == stageArchApproved:
		return PositionDocPlan, 0
	case p.stage == stageReqApprovedFull:
		return PositionDocArchitecture, 0
	default:
		return PositionDocNeedsCheck, 0
	}
}

// onlyMergeQuestions reports whether the row waits on questions and every one
// is a merge confirmation. A row with no question is not one.
func onlyMergeQuestions(r Row) bool {
	return r.State == StateQuestion && len(r.Questions) > 0 && !slices.ContainsFunc(r.Questions, func(q QuestionItem) bool { return !q.Question.Merge })
}

// progressOf decides the progress part of a row's position from its state,
// the command its agents run (nil when none is decided), and its questions.
func progressOf(r Row, running *RunningCommand) Progress {
	switch r.State {
	case StateProposeNext:
		return ProgressAwaitingCreation
	case StateRunning:
		switch {
		case running == nil:
			return ProgressRunning
		case running.Kind() == paneinput.CommandRunpr:
			return ProgressRunprRunning
		case running.Kind() == paneinput.CommandUnknown:
			return ProgressRunning
		default:
			return ProgressCreating
		}
	case StateQuestion:
		if onlyMergeQuestions(r) {
			return ProgressMergeConfirmation
		}
		return ProgressQuestion
	case StateAwaitingReview:
		return ProgressAwaitingReview
	case StateTaskDone:
		return ProgressTaskDone
	default:
		return ProgressNeedsCheck
	}
}
