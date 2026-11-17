//go:build test

package chief

import (
	"fmt"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// docsInfo returns a document of status with the process record of a
// requirements document.
func docsInfo(status DocStatus, process Process) *DocInfo {
	return &DocInfo{Status: status, Process: process}
}

// TestStageOf checks each stage of the documents, in the order the next
// command is decided, including the stages no sampled document reaches.
func TestStageOf(t *testing.T) {
	approvedPlan := &Plan{Status: DocApproved}
	draftPlan := &Plan{Status: DocDraft}
	cases := []struct {
		name string
		docs *TaskDocs
		want docStage
	}{
		{"documents not read", nil, stageNoRequirements},
		{"no task directory", &TaskDocs{}, stageNoRequirements},
		{"a directory without requirements", &TaskDocs{Dir: "d"}, stageNoRequirements},
		{"approved plan", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull), Architecture: docsInfo(DocApproved, ProcessFull), Plan: approvedPlan}, stagePlanApproved},
		{"approved plan over a draft architecture", &TaskDocs{Dir: "d", Requirements: docsInfo(DocDraft, ProcessFull), Architecture: docsInfo(DocDraft, ProcessFull), Plan: approvedPlan}, stagePlanApproved},
		{"draft plan", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull), Architecture: docsInfo(DocApproved, ProcessFull), Plan: draftPlan}, stagePlanNotApproved},
		{"approved architecture", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull), Architecture: docsInfo(DocApproved, ProcessFull)}, stageArchApproved},
		{"draft architecture", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull), Architecture: docsInfo(DocDraft, ProcessFull)}, stageArchNotApproved},
		{"approved requirements, full", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull)}, stageReqApprovedFull},
		{"approved requirements, no record", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessUnrecorded)}, stageReqApprovedFull},
		{"approved requirements, broken record", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessUnreadable)}, stageProcessUnreadable},
		{"approved requirements, simple", &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessSimple)}, stageOther},
		{"draft requirements", &TaskDocs{Dir: "d", Requirements: docsInfo(DocDraft, ProcessFull)}, stageOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stageOf(c.docs); got != c.want {
				t.Errorf("stageOf = %d, want %d", got, c.want)
			}
		})
	}
	if (docStage(0)) != stageOther {
		t.Error("the zero stage is not the one that assumes least")
	}
}

// TestUrgencyTable gives urgencyOf every combination of the six states, a
// state outside them, the stop (none, a stop, an unreadable stop record), and
// the idle mark, and checks each against the table of the design.
func TestUrgencyTable(t *testing.T) {
	type result struct {
		urgency Urgency
		turn    Turn
	}
	now := result{UrgencyNow, TurnHuman}
	later := result{UrgencyLater, TurnHuman}
	waitAI := result{UrgencyWait, TurnAI}
	waitNone := result{UrgencyWait, TurnNone}
	// want is the expected result of a row that takes input (no stop), by
	// state and idle mark.
	type key struct {
		state WorkspaceState
		idle  bool
	}
	want := map[key]result{
		{StateQuestion, false}:       now,
		{StateQuestion, true}:        now,
		{StateAwaitingReview, false}: later,
		{StateAwaitingReview, true}:  waitNone,
		{StateProposeNext, false}:    later,
		{StateProposeNext, true}:     waitNone,
		{StateNeedsCheck, false}:     later,
		{StateNeedsCheck, true}:      waitNone,
		{StateRunning, false}:        waitAI,
		{StateRunning, true}:         waitNone,
		{StateTaskDone, false}:       waitNone,
		{StateTaskDone, true}:        waitNone,
		{WorkspaceState(99), false}:  now,
		{WorkspaceState(99), true}:   waitNone,
	}
	stops := []struct {
		name string
		set  func(r *Row)
		now  bool
	}{
		{"no stop", func(*Row) {}, false},
		{"a stop", func(r *Row) { r.Stopped = &StopRecord{} }, true},
		{"an unreadable stop record", func(r *Row) { r.StopUnreadable = true }, true},
	}
	for k, w := range want {
		for _, stop := range stops {
			t.Run(fmt.Sprintf("state %d, idle %v, %s", k.state, k.idle, stop.name), func(t *testing.T) {
				row := Row{State: k.state, Idle: k.idle, Unreadable: []string{"anything", "not read by the table"}}
				stop.set(&row)
				expected := w
				if stop.now {
					expected = now
				}
				urgency, turn := urgencyOf(row)
				if (result{urgency, turn}) != expected {
					t.Errorf("urgencyOf = %d/%d, want %d/%d", urgency, turn, expected.urgency, expected.turn)
				}
			})
		}
	}
}

// TestUrgencyIgnoresWhyARowNeedsCheck checks that a row that needs a check
// is later whatever it says it could not read.
func TestUrgencyIgnoresWhyARowNeedsCheck(t *testing.T) {
	for _, why := range [][]string{nil, {"git fetch origin main failed"}, {"task documents on origin/main", "screen of pane w1:p1"}} {
		if urgency, turn := urgencyOf(Row{State: StateNeedsCheck, Unreadable: why}); urgency != UrgencyLater || turn != TurnHuman {
			t.Errorf("urgencyOf(needs check, %v) = %d/%d, want later/human", why, urgency, turn)
		}
	}
}

func mustPlan(t *testing.T, name string) *Plan {
	t.Helper()
	return mustPlanDoc(t, planSample(t, name))
}

func mustPlanDoc(t *testing.T, doc []byte) *Plan {
	t.Helper()
	plan, err := ParsePlan(doc, testOldValues)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	return &plan
}

// planBeforeTheFirstPR is marker-opus.md with its first PR not started.
func planBeforeTheFirstPR(t *testing.T) *Plan {
	t.Helper()
	doc := string(planSample(t, "marker-opus.md"))
	started := "- [x] PR を作成した: https://github.com/owner/repo/pull/1\n- [x] PR がマージされた\n"
	if !strings.Contains(doc, started) {
		t.Fatal("marker-opus.md no longer holds a merged first PR")
	}
	return mustPlanDoc(t, []byte(strings.Replace(doc, started, "- [ ] PR を作成した\n- [ ] PR がマージされた\n", 1)))
}

// planFacts returns the facts of an approved plan.
func planFacts(t *testing.T, plan *Plan) positionFacts {
	t.Helper()
	return positionFactsOf(workspaceFacts{
		Head: headBranch, Task: mustTask(t, "0001"),
		Docs: &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull), Architecture: docsInfo(DocApproved, ProcessFull), Plan: plan},
	})
}

func mustTask(t *testing.T, s string) paneinput.TaskID {
	t.Helper()
	id, err := paneinput.ParseTaskID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustMkreq(t *testing.T) *RunningCommand {
	t.Helper()
	cmd, _, err := newRunningCommand(paneinput.CommandMkreq, "890")
	if err != nil {
		t.Fatal(err)
	}
	return &cmd
}

func mustRunning(t *testing.T, kind paneinput.CommandKind, arg string) *RunningCommand {
	t.Helper()
	cmd, _, err := newRunningCommand(kind, arg)
	if err != nil {
		t.Fatal(err)
	}
	return &cmd
}

// TestDocumentRule gives documentOf, for each of the rules 1 and 3 to 10 of
// the design, facts only that rule matches, and checks the document and N.
func TestDocumentRule(t *testing.T) {
	branch := positionFacts{head: headBranch}
	task := func(stage docStage) positionFacts {
		return positionFacts{head: headBranch, hasTask: true, stage: stage}
	}
	proposeMkreq := func(p positionFacts) Row {
		return Row{State: StateProposeNext, Next: &Next{Kind: TaskNextMkreq, Text: "/mkreq"}, position: p}
	}
	cases := []struct {
		name    string
		row     Row
		running *RunningCommand
		want    PositionDocument
		wantPR  int
	}{
		{"rule 1: the fetch failed", Row{State: StateNeedsCheck, position: positionFacts{head: headBranch, hasTask: true, stage: stageReqApprovedFull, fetchFailed: true}}, nil, PositionDocNeedsCheck, 0},
		{"rule 1: the documents could not be read", Row{State: StateNeedsCheck, position: positionFacts{head: headBranch, hasTask: true, docsUnreadable: true}}, nil, PositionDocNeedsCheck, 0},
		{"rule 1 wins over a running /mkreq", Row{State: StateRunning, position: positionFacts{head: headDetached, fetchFailed: true}}, mustMkreq(t), PositionDocNeedsCheck, 0},
		{"rule 3: a proposal of /mkreq on a branch that names no task", proposeMkreq(branch), nil, PositionDocRequirements, 0},
		{"rule 3: a running /mkreq on a detached HEAD", Row{State: StateRunning, position: positionFacts{head: headDetached}}, mustMkreq(t), PositionDocRequirements, 0},
		{"rule 3: a running /mkreq on a branch that names no task", Row{State: StateRunning, position: branch}, mustMkreq(t), PositionDocRequirements, 0},
		{"rule 4: awaiting review on a branch that names no task", Row{State: StateAwaitingReview, position: branch}, nil, PositionDocNoTask, 0},
		{"rule 4: a branch that names no task with another command running", Row{State: StateRunning, position: branch}, mustRunning(t, paneinput.CommandRunpr, "12"), PositionDocNoTask, 0},
		{"rule 4: a running row with no command decided", Row{State: StateRunning, position: branch}, nil, PositionDocNoTask, 0},
		{"rule 5: no requirements document", Row{State: StateNeedsCheck, position: task(stageNoRequirements)}, nil, PositionDocRequirements, 0},
		{"rule 6: every PR merged", Row{State: StateTaskDone, position: positionFacts{head: headBranch, hasTask: true, stage: stagePlanApproved, planDone: true}}, nil, PositionDocNone, 0},
		{"rule 7: the first PR to merge is PR-4", Row{State: StateProposeNext, position: positionFacts{head: headBranch, hasTask: true, stage: stagePlanApproved, planPR: 4}}, nil, PositionDocPlanPR, 4},
		{"rule 8: approved architecture", Row{State: StateProposeNext, position: task(stageArchApproved)}, nil, PositionDocPlan, 0},
		{"rule 9: approved requirements", Row{State: StateProposeNext, position: task(stageReqApprovedFull)}, nil, PositionDocArchitecture, 0},
		{"rule 10: a plan that is approved but names no PR", Row{State: StateNeedsCheck, position: task(stagePlanApproved)}, nil, PositionDocNeedsCheck, 0},
		{"rule 10: a draft plan", Row{State: StateNeedsCheck, position: task(stagePlanNotApproved)}, nil, PositionDocNeedsCheck, 0},
		{"rule 10: a draft architecture", Row{State: StateNeedsCheck, position: task(stageArchNotApproved)}, nil, PositionDocNeedsCheck, 0},
		{"rule 10: a broken process record", Row{State: StateNeedsCheck, position: task(stageProcessUnreadable)}, nil, PositionDocNeedsCheck, 0},
		{"rule 10: draft requirements", Row{State: StateNeedsCheck, position: task(stageOther)}, nil, PositionDocNeedsCheck, 0},
		{"rule 10: a HEAD that was not read", Row{State: StateNeedsCheck, position: positionFacts{head: headUnread}}, nil, PositionDocNeedsCheck, 0},
		{"rule 10: a detached HEAD with nothing running", Row{State: StateNeedsCheck, position: positionFacts{head: headDetached}}, nil, PositionDocNeedsCheck, 0},
		{"rule 10: a detached HEAD with another command running", Row{State: StateRunning, position: positionFacts{head: headDetached}}, mustRunning(t, paneinput.CommandRunplan, "0001"), PositionDocNeedsCheck, 0},
		{"rule 10: a detached HEAD whose proposal is /mkreq", proposeMkreq(positionFacts{head: headDetached}), nil, PositionDocNeedsCheck, 0},
		{"the state needs a check but the documents are read (AC-08)", Row{State: StateNeedsCheck, Unreadable: []string{"screen of pane w1:p1"}, position: task(stageArchApproved)}, nil, PositionDocPlan, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := c.row.State
			got, n := documentOf(c.row, c.running)
			if got != c.want || n != c.wantPR {
				t.Errorf("documentOf = %d, %d; want %d, %d", got, n, c.want, c.wantPR)
			}
			if c.row.State != state {
				t.Errorf("documentOf changed the state: %v -> %v", state, c.row.State)
			}
		})
	}
}

// TestDocumentRuleReadsThePlan checks rule 6 and rule 7, and the choice of
// rule 10 over a PR number it cannot tell, on plans read from samples: before
// the first PR, a PR created and not merged, a middle PR, a plan without
// markers, and a plan whose PRs are all merged.
func TestDocumentRuleReadsThePlan(t *testing.T) {
	cases := []struct {
		sample string
		want   PositionDocument
		wantPR int
	}{
		{"marker-opus.md", PositionDocPlanPR, 2},
		{"marker-opus-pr-open.md", PositionDocPlanPR, 2},
		{"marker-real-forms.md", PositionDocPlanPR, 3},
		{"single-sonnet.md", PositionDocPlanPR, 1},
		{"marker-all-merged.md", PositionDocNone, 0},
		{"single-recorded.md", PositionDocNone, 0},
		// Every box is merged but one PR was never created, so the plan is
		// not complete and no PR number can be named.
		{"marker-merged-not-created.md", PositionDocNeedsCheck, 0},
	}
	for _, c := range cases {
		t.Run(c.sample, func(t *testing.T) {
			row := Row{State: StateProposeNext, position: planFacts(t, mustPlan(t, c.sample))}
			got, n := documentOf(row, nil)
			if got != c.want || n != c.wantPR {
				t.Errorf("documentOf = %d, %d; want %d, %d (facts %+v)", got, n, c.want, c.wantPR, row.position)
			}
		})
	}
	t.Run("before the first PR", func(t *testing.T) {
		row := Row{State: StateProposeNext, position: planFacts(t, planBeforeTheFirstPR(t))}
		if got, n := documentOf(row, nil); got != PositionDocPlanPR || n != 1 {
			t.Errorf("documentOf = %d, %d; want plan PR 1 (facts %+v)", got, n, row.position)
		}
	})
}

// TestClassifyKeepsPositionFacts checks that classify keeps what the document
// rule reads on the row for each of the six states, and for the failures that
// the facts name.
func TestClassifyKeepsPositionFacts(t *testing.T) {
	task := mustTask(t, "0001")
	planDocs := func(plan string) *TaskDocs {
		return &TaskDocs{Dir: "d", Requirements: docsInfo(DocApproved, ProcessFull), Architecture: docsInfo(DocApproved, ProcessFull), Plan: mustPlan(t, plan)}
	}
	prompt := func(status AgentStatus) []agentFacts {
		return []agentFacts{{Agent: Agent{Kind: paneinput.AgentClaudeCode, Status: status}, ScreenRead: true, Screen: Screen{Kind: ScreenPrompt}}}
	}
	question := []agentFacts{{Agent: Agent{Kind: paneinput.AgentClaudeCode, Status: AgentBlocked}, ScreenRead: true, Screen: Screen{Kind: ScreenQuestion, Question: &Question{Text: "Q?", Options: []Option{{}}}}}}
	cases := []struct {
		name  string
		facts workspaceFacts
		state WorkspaceState
		want  positionFacts
	}{
		{"needs check: nothing read", workspaceFacts{}, StateNeedsCheck, positionFacts{}},
		{
			"needs check: documents unreadable after a failed fetch",
			workspaceFacts{Head: headBranch, Branch: "b", Task: task, Unreadable: []string{"task documents on origin/main"}, DocsUnreadable: true, FetchFailed: true},
			StateNeedsCheck,
			positionFacts{fetchFailed: true, docsUnreadable: true, head: headBranch, hasTask: true},
		},
		{
			"question",
			workspaceFacts{Head: headBranch, Branch: "b", Task: task, Agents: question, Docs: planDocs("marker-opus.md")},
			StateQuestion,
			positionFacts{head: headBranch, hasTask: true, stage: stagePlanApproved, planPR: 2},
		},
		{
			"running",
			workspaceFacts{Head: headBranch, Branch: "b", Task: task, Agents: prompt(AgentWorking), Docs: planDocs("marker-opus-pr-open.md")},
			StateRunning,
			positionFacts{head: headBranch, hasTask: true, stage: stagePlanApproved, planPR: 2},
		},
		{
			"awaiting review, a branch that names no task",
			workspaceFacts{Head: headBranch, Branch: "fix/x", Agents: prompt(AgentIdle), PR: &OpenPR{Number: 7}},
			StateAwaitingReview,
			positionFacts{head: headBranch},
		},
		{
			"propose next",
			workspaceFacts{Head: headBranch, Branch: "b", Task: task, Agents: prompt(AgentIdle), Docs: planDocs("marker-opus.md")},
			StateProposeNext,
			positionFacts{head: headBranch, hasTask: true, stage: stagePlanApproved, planPR: 2},
		},
		{
			"task done",
			workspaceFacts{Head: headBranch, Branch: "b", Task: task, Agents: prompt(AgentIdle), Docs: planDocs("marker-all-merged.md")},
			StateTaskDone,
			positionFacts{head: headBranch, hasTask: true, stage: stagePlanApproved, planDone: true},
		},
		{"a detached HEAD", workspaceFacts{Head: headDetached, Agents: prompt(AgentIdle)}, StateNeedsCheck, positionFacts{head: headDetached}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row := classify(c.facts)
			if row.State != c.state {
				t.Fatalf("state = %v, want %v (unreadable %v)", row.State, c.state, row.Unreadable)
			}
			if row.position != c.want {
				t.Errorf("position facts = %+v, want %+v", row.position, c.want)
			}
		})
	}
}

// TestProgress checks each row of the progress table, a "question" row that
// holds no question, and a running command of a kind outside the five.
func TestProgress(t *testing.T) {
	merge := QuestionItem{Question: Question{Merge: true}}
	plain := QuestionItem{Question: Question{}}
	cases := []struct {
		name    string
		row     Row
		running *RunningCommand
		want    Progress
	}{
		{"propose next", Row{State: StateProposeNext}, nil, ProgressAwaitingCreation},
		{"running /mkreq", Row{State: StateRunning}, mustMkreq(t), ProgressCreating},
		{"running /mkarch", Row{State: StateRunning}, mustRunning(t, paneinput.CommandMkarch, "0001"), ProgressCreating},
		{"running /mkplan", Row{State: StateRunning}, mustRunning(t, paneinput.CommandMkplan, "0001"), ProgressCreating},
		{"running /runplan", Row{State: StateRunning}, mustRunning(t, paneinput.CommandRunplan, "0001"), ProgressCreating},
		{"running /runpr", Row{State: StateRunning}, mustRunning(t, paneinput.CommandRunpr, "12"), ProgressRunprRunning},
		{"running with no command decided", Row{State: StateRunning}, nil, ProgressRunning},
		{"running a command of a kind outside the five", Row{State: StateRunning}, &RunningCommand{}, ProgressRunning},
		{"only the merge question", Row{State: StateQuestion, Questions: []QuestionItem{merge}}, nil, ProgressMergeConfirmation},
		{"two merge questions", Row{State: StateQuestion, Questions: []QuestionItem{merge, merge}}, nil, ProgressMergeConfirmation},
		{"the merge question and another", Row{State: StateQuestion, Questions: []QuestionItem{merge, plain}}, nil, ProgressQuestion},
		{"another question only", Row{State: StateQuestion, Questions: []QuestionItem{plain}}, nil, ProgressQuestion},
		{"a question row with no question", Row{State: StateQuestion}, nil, ProgressQuestion},
		{"awaiting review", Row{State: StateAwaitingReview}, nil, ProgressAwaitingReview},
		{"task done", Row{State: StateTaskDone}, nil, ProgressTaskDone},
		{"needs check", Row{State: StateNeedsCheck}, nil, ProgressNeedsCheck},
		{"a state outside the six", Row{State: WorkspaceState(99)}, nil, ProgressNeedsCheck},
		{"merge questions on a row that needs a check", Row{State: StateNeedsCheck, Questions: []QuestionItem{merge}}, nil, ProgressNeedsCheck},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := progressOf(c.row, c.running); got != c.want {
				t.Errorf("progressOf = %d, want %d", got, c.want)
			}
		})
	}
}

// TestSnapshotRowKeepsWhyTheDocumentsAreUnknown reads a workspace whose
// documents cannot be read through the fake Herdr and checks that the row
// keeps that as a fact, so the document rule never reads Unreadable's text.
func TestSnapshotRowKeepsWhyTheDocumentsAreUnknown(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "broken", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: statusDoc("| Status | `approved` |\n| Process | full |\n")}})
	w.add(fakeWorkspace{ID: "w2", Label: "readable", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved}})
	snap := take(t, w)
	broken, readable := rowOf(t, snap, "broken"), rowOf(t, snap, "readable")
	if !broken.position.docsUnreadable || broken.position.hasTask != true {
		t.Errorf("broken row facts = %+v, want unreadable documents of a task", broken.position)
	}
	if got, _ := documentOf(broken, nil); got != PositionDocNeedsCheck {
		t.Errorf("the document of a row with unreadable documents = %d, want needs check", got)
	}
	if readable.position.docsUnreadable || readable.position.stage != stageReqApprovedFull {
		t.Errorf("readable row facts = %+v, want the requirements stage", readable.position)
	}
	if got, _ := documentOf(readable, nil); got != PositionDocArchitecture {
		t.Errorf("the document of the readable row = %d, want architecture", got)
	}
}
