package chief

import (
	"fmt"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// The types below are the JSON the tool prints. Every string that came from
// outside (a label, a question, a PR title, a problem text) is passed through
// Sanitize on the way in, so a terminal that shows the output cannot be
// rewritten by it.

// SnapshotOutput is the output of `chief snapshot`.
type SnapshotOutput struct {
	Protocol int             `json:"protocol"`
	StateDir string          `json:"state_dir"`
	Rows     []RowOutput     `json:"rows"`
	Pending  []PendingOutput `json:"pending"`
	// NewNotices lists the items not announced before in this session.
	NewNotices []ItemID `json:"new_notices"`
	Problems   []string `json:"problems"`
	// Baseline is the value `chief wait --since` takes.
	Baseline WaitBaseline `json:"baseline"`
}

// RowOutput is one row of the status table.
type RowOutput struct {
	Workspace     string           `json:"workspace"`
	Label         string           `json:"label"`
	Task          string           `json:"task"`
	State         string           `json:"state"`
	Next          *NextOutput      `json:"next"`
	Takeover      *NextOutput      `json:"takeover"`
	Unreadable    []string         `json:"unreadable"`
	MergeQuestion bool             `json:"merge_question"`
	Questions     []QuestionOutput `json:"questions"`
	PR            *PROutput        `json:"pr"`
	Running       *RunningOutput   `json:"running"`
	Stopped       *StopOutput      `json:"stopped"`
}

// NextOutput is a command offered for a workspace.
type NextOutput struct {
	// Command is the command line, such as "/runplan 0088". A /mkreq has no
	// issue number: the human names it.
	Command string `json:"command"`
	// Request is the --command value `chief send` takes to send this
	// command; a /mkreq is sent with the issue number the human names.
	Request string `json:"request"`
	// Sendable is true when the command can be sent with the agent and model
	// shown; false means Problem says why not.
	Sendable bool   `json:"sendable"`
	Agent    string `json:"agent"`
	Model    string `json:"model"`
	Problem  string `json:"problem"`
}

// QuestionOutput is one agent's question.
type QuestionOutput struct {
	Pane      string `json:"pane"`
	Agent     string `json:"agent"`
	Kind      string `json:"kind"`
	Merge     bool   `json:"merge"`
	Relayable bool   `json:"relayable"`
	// Check is the value `chief relay --check` takes for this question,
	// absent when the question is not relayed.
	Check   CheckValue     `json:"check,omitempty"`
	Text    string         `json:"text"`
	Options []OptionOutput `json:"options"`
	Tabs    []string       `json:"tabs"`
}

// OptionOutput is one option of a question.
type OptionOutput struct {
	Number      int    `json:"number"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// PROutput is an open pull request.
type PROutput struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// PendingOutput is one item waiting on the human.
type PendingOutput struct {
	ID        ItemID          `json:"id"`
	Kind      string          `json:"kind"`
	Workspace string          `json:"workspace"`
	Label     string          `json:"label"`
	Question  *QuestionOutput `json:"question"`
	PR        *PROutput       `json:"pr"`
	Proposal  *NextOutput     `json:"proposal"`
	// Reset is a skipped reset offered to the human, set on a reset item.
	Reset *ResetOutput `json:"reset"`
	// Reasons are the conditions of the automatic send that did not hold
	// for a proposal after a merge.
	Reasons []string `json:"reasons"`
	// ReviewSkippable is set on a review item whose PR detail was gathered
	// for the current head and judged the review skippable.
	ReviewSkippable bool      `json:"review_skippable"`
	FirstSeen       time.Time `json:"first_seen"`
	New             bool      `json:"new"`
	// Number is the item's number in this session's lists, kept for the
	// session and never reused; 0 when none could be given.
	Number int `json:"number"`
}

// ResetOutput is a skipped reset offered to the human: the pane, its agent,
// the input that empties it, the scene it was skipped in, the command that
// ran in the pane, and the value `chief reset --check` takes for it.
type ResetOutput struct {
	Pane    string     `json:"pane"`
	Agent   string     `json:"agent"`
	Input   string     `json:"input"`
	Scene   string     `json:"scene"`
	Command string     `json:"command"`
	Check   CheckValue `json:"check"`
}

// resetOutput converts a reset item.
func resetOutput(it *ResetItem) *ResetOutput {
	if it == nil {
		return nil
	}
	input := ""
	if st, ok := resetPrompt(it.Agent); ok {
		input, _ = st.prompt.Text()
	}
	return &ResetOutput{Pane: Sanitize(it.Pane.String()), Agent: agentLabel(it.Agent), Input: input, Scene: it.Scene.String(), Command: Sanitize(it.Command.text()), Check: it.Check}
}

// questionKindName names a question shape for the output.
func questionKindName(k QuestionKind) string {
	switch k {
	case QuestionSingle:
		return "single_select"
	case QuestionMulti:
		return "multi_select"
	case QuestionTabs:
		return "tabs"
	case QuestionPermission:
		return "permission"
	default:
		return unknownName
	}
}

func sanitizeAll(texts []string) []string {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = Sanitize(t)
	}
	return out
}

func nextOutput(n *Next) *NextOutput {
	if n == nil {
		return nil
	}
	return &NextOutput{
		Command:  n.Text,
		Request:  requestNames[n.Kind],
		Sendable: n.Command != nil,
		Agent:    agentLabel(n.Agent),
		Model:    Sanitize(n.Model),
		Problem:  Sanitize(n.Problem),
	}
}

func questionOutput(q QuestionItem) QuestionOutput {
	out := QuestionOutput{
		Pane:      q.Pane,
		Agent:     agentLabel(q.Agent),
		Kind:      questionKindName(q.Question.Kind),
		Merge:     q.Question.Merge,
		Relayable: q.Check != "",
		Check:     q.Check,
		Text:      Sanitize(q.Question.Text),
		Options:   []OptionOutput{},
		Tabs:      sanitizeAll(q.Question.Tabs),
	}
	for _, o := range q.Question.Options {
		out.Options = append(out.Options, OptionOutput{Number: o.Number, Label: Sanitize(o.Label), Description: Sanitize(o.Description)})
	}
	return out
}

func prOutput(p *OpenPR) *PROutput {
	if p == nil {
		return nil
	}
	return &PROutput{Number: p.Number, Title: Sanitize(p.Title), URL: Sanitize(p.URL)}
}

// Output converts a snapshot into the JSON types. Lists are never nil, so
// the output holds [] and not null.
func (s Snapshot) Output() SnapshotOutput {
	out := SnapshotOutput{
		Protocol:   Protocol,
		StateDir:   s.StateDir,
		Rows:       []RowOutput{},
		Pending:    []PendingOutput{},
		NewNotices: []ItemID{},
		Problems:   sanitizeAll(s.Problems),
		Baseline:   s.Baseline,
	}
	for _, r := range s.Rows {
		row := RowOutput{
			Workspace:     r.Workspace,
			Label:         Sanitize(r.Label),
			Task:          r.Task,
			State:         r.State.String(),
			Next:          nextOutput(r.Next),
			Takeover:      nextOutput(r.Takeover),
			Unreadable:    sanitizeAll(r.Unreadable),
			MergeQuestion: r.MergeQuestion,
			Questions:     []QuestionOutput{},
			PR:            prOutput(r.PR),
		}
		if r.Running != nil {
			row.Running = &RunningOutput{Pane: Sanitize(r.Running.Pane), Command: Sanitize(r.Running.Command), Agent: Sanitize(r.Running.Agent), Model: Sanitize(r.Running.Model), Started: r.Running.Started}
		}
		if r.Stopped != nil {
			row.Stopped = &StopOutput{Reason: Sanitize(r.Stopped.Reason), Screen: Sanitize(r.Stopped.Screen), At: r.Stopped.At}
		}
		for _, q := range r.Questions {
			row.Questions = append(row.Questions, questionOutput(q))
		}
		out.Rows = append(out.Rows, row)
	}
	for _, p := range s.Pending {
		item := PendingOutput{
			ID:              p.ID,
			Kind:            p.Kind.String(),
			Workspace:       p.Workspace,
			Label:           Sanitize(p.Label),
			PR:              prOutput(p.PR),
			Proposal:        nextOutput(p.Proposal),
			Reset:           resetOutput(p.Reset),
			Reasons:         sanitizeAll(p.Reasons),
			ReviewSkippable: p.ReviewSkippable,
			FirstSeen:       p.FirstSeen,
			Number:          p.Number,
			New:             p.New,
		}
		if p.Question != nil {
			q := questionOutput(*p.Question)
			item.Question = &q
		}
		out.Pending = append(out.Pending, item)
		if p.New {
			out.NewNotices = append(out.NewNotices, p.ID)
		}
	}
	return out
}

// RunningOutput is what chief last sent to a working pane, shown on a
// running row; it comes only from chief's send record, never from a screen.
type RunningOutput struct {
	Pane    string    `json:"pane"`
	Command string    `json:"command"`
	Agent   string    `json:"agent"`
	Model   string    `json:"model"`
	Started time.Time `json:"started"`
}

// StopOutput is a workspace's stop: why, what its pane showed, and when.
type StopOutput struct {
	Reason string    `json:"reason"`
	Screen string    `json:"screen"`
	At     time.Time `json:"at"`
}

// ActionOutput is one send or reset.
type ActionOutput struct {
	Workspace string   `json:"workspace"`
	Pane      string   `json:"pane"`
	Input     string   `json:"input"`
	Command   string   `json:"command"`
	Agent     string   `json:"agent"`
	Model     string   `json:"model"`
	Outcome   string   `json:"outcome"`
	Reason    string   `json:"reason"`
	Step      string   `json:"step"`
	Detail    string   `json:"detail"`
	Left      string   `json:"left"`
	Panes     []string `json:"panes"`
	Status    string   `json:"status"`
	Screen    string   `json:"screen"`
}

// SendOutput is the output of `chief send`.
type SendOutput struct {
	Protocol int            `json:"protocol"`
	Actions  []ActionOutput `json:"actions"`
}

// CheckOutput is one CI job, its bucket as gh gave it.
type CheckOutput struct {
	Name     string `json:"name"`
	Workflow string `json:"workflow"`
	State    string `json:"state"`
	Bucket   string `json:"bucket"`
}

// ReviewOutput is one PR review's author and state.
type ReviewOutput struct {
	Author string `json:"author"`
	State  string `json:"state"`
}

// DetailOutput is the output of `chief detail`. An item that could not be
// read is null with its *_error set.
type DetailOutput struct {
	Protocol     int            `json:"protocol"`
	Problem      string         `json:"problem"`
	Workspace    string         `json:"workspace"`
	Label        string         `json:"label"`
	Task         string         `json:"task"`
	PR           *PROutput      `json:"pr"`
	Size         *SizeOutput    `json:"size"`
	Reviews      []ReviewOutput `json:"reviews"`
	ViewError    string         `json:"view_error"`
	Checks       []CheckOutput  `json:"checks"`
	ChecksError  string         `json:"checks_error"`
	ReportSource string         `json:"report_source"`
	Report       string         `json:"report"`
	ReportCut    bool           `json:"report_truncated"`
	// DescriptionReason is why no pane's report is shown, null when one is
	// or when a kept detail was used without the panes being read again.
	DescriptionReason *DescriptionReasonOutput `json:"description_reason"`
	RecordError       string                   `json:"record_error"`
	Skippable         bool                     `json:"review_skippable"`
	Runpr             *NextOutput              `json:"runpr"`
}

// DescriptionReasonOutput is why a PR detail shows no pane's report:
// no_claude_pane, no_candidate (no Claude Code pane's last report has a PR
// record naming the PR), or several_candidates, with what was found on each
// Claude Code pane.
type DescriptionReasonOutput struct {
	Reason string                  `json:"reason"`
	Panes  []DescriptionPaneOutput `json:"panes"`
}

// DescriptionPaneOutput is what a PR detail found on one Claude Code pane:
// unreadable, no_report, not_named, or candidate, with why it is unreadable
// or has no report and the transcript's path.
type DescriptionPaneOutput struct {
	Pane   string `json:"pane"`
	Result string `json:"result"`
	Detail string `json:"detail"`
}

// SizeOutput is a PR's size.
type SizeOutput struct {
	ChangedFiles int `json:"changed_files"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	Commits      int `json:"commits"`
	// CommitsAtLeast is true when Commits is gh's cap: the PR has that many
	// or more, and CommitsNote says where the total is.
	CommitsAtLeast bool   `json:"commits_at_least"`
	CommitsNote    string `json:"commits_note"`
}

// ReleaseOutput is the output of `chief release`.
type ReleaseOutput struct {
	Protocol int    `json:"protocol"`
	Released bool   `json:"released"`
	Problem  string `json:"problem"`
}

// statusName names an agent status for the output.
func statusName(s AgentStatus) string {
	for name, v := range agentStatuses {
		if v == s {
			return name
		}
	}
	return unknownName
}

// Output converts a send's results into the JSON types.
func (o SendOutcome) Output() SendOutput {
	out := SendOutput{Protocol: Protocol, Actions: []ActionOutput{}}
	for _, a := range o.Actions {
		out.Actions = append(out.Actions, actionOutput(a))
	}
	return out
}

func actionOutput(a ActionResult) ActionOutput {
	status := ""
	if a.Status != AgentStatusUnknown {
		status = statusName(a.Status)
	}
	return ActionOutput{
		Workspace: Sanitize(a.Workspace),
		Pane:      Sanitize(a.Pane),
		Input:     a.Input.String(),
		Command:   Sanitize(a.Command),
		Agent:     agentLabel(a.Agent),
		Model:     Sanitize(a.Model),
		Outcome:   a.Outcome.String(),
		Reason:    reasonName(a),
		Step:      a.Step.String(),
		Detail:    Sanitize(a.Detail),
		Left:      Sanitize(a.Left),
		Panes:     sanitizeAll(a.Panes),
		Status:    status,
		Screen:    Sanitize(a.Screen),
	}
}

// reasonName is the reason of a result that did not send everything; a sent
// result has none.
func reasonName(a ActionResult) string {
	if a.Outcome == OutcomeSent {
		return ""
	}
	return a.Reason.String()
}

// Output converts a detail into the JSON types.
func (o DetailOutcome) Output() DetailOutput {
	out := DetailOutput{Protocol: Protocol, Problem: Sanitize(o.Problem), Reviews: []ReviewOutput{}, Checks: []CheckOutput{}}
	d := o.Detail
	if d == nil {
		return out
	}
	out.Workspace, out.Label, out.Task = Sanitize(d.Workspace), Sanitize(d.Label), d.Task
	out.PR = prOutput(&d.PR)
	out.ViewError, out.ChecksError, out.RecordError = Sanitize(d.ViewErr), Sanitize(d.ChecksErr), Sanitize(d.RecordErr)
	if d.View != nil {
		out.Size = &SizeOutput{ChangedFiles: d.View.ChangedFiles, Additions: d.View.Additions, Deletions: d.View.Deletions, Commits: d.View.Commits}
		if d.View.CommitsCapped {
			out.Size.CommitsAtLeast = true
			out.Size.CommitsNote = fmt.Sprintf("gh returns at most %d commits, so the PR has %d or more; the total is on the PR's page on GitHub (gh pr view %d --web)", ghCommitsCap, ghCommitsCap, d.PR.Number)
		}
		for _, r := range d.View.Reviews {
			out.Reviews = append(out.Reviews, ReviewOutput{Author: Sanitize(r.Author), State: Sanitize(r.State)})
		}
	}
	if d.ChecksErr != "" {
		out.Checks = nil
	}
	for _, c := range d.Checks {
		out.Checks = append(out.Checks, CheckOutput{Name: Sanitize(c.Name), Workflow: Sanitize(c.Workflow), State: Sanitize(c.State), Bucket: Sanitize(c.Bucket)})
	}
	if d.ViewErr != "" {
		out.Reviews = nil
	}
	out.ReportSource, out.Report, out.ReportCut = d.Report.String(), Sanitize(d.ReportText), d.ReportTruncated
	if d.DescriptionReason != DescriptionReasonNone {
		reason := &DescriptionReasonOutput{Reason: d.DescriptionReason.String(), Panes: []DescriptionPaneOutput{}}
		for _, p := range d.ReportPanes {
			reason.Panes = append(reason.Panes, DescriptionPaneOutput{Pane: Sanitize(p.Pane), Result: p.Kind.String(), Detail: Sanitize(p.Detail)})
		}
		out.DescriptionReason = reason
	}
	out.Skippable = d.ReviewSkippable
	out.Runpr = nextOutput(d.Runpr)
	return out
}

// Output converts a release into the JSON type.
func (o ReleaseOutcome) Output() ReleaseOutput {
	return ReleaseOutput{Protocol: Protocol, Released: o.Released, Problem: Sanitize(o.Problem)}
}

// TickOutput is the output of `chief tick`: the snapshot's members and the
// merged PRs an automatic send was considered for.
type TickOutput struct {
	SnapshotOutput
	AutoSends []AutoSendOutput `json:"auto_sends"`
}

// AutoSendOutput is one merged PR an automatic send was considered for. An
// empty Reasons with a sent command action is an automatic send.
type AutoSendOutput struct {
	Workspace string         `json:"workspace"`
	Label     string         `json:"label"`
	PR        int            `json:"pr"`
	Task      string         `json:"task"`
	Proposal  *NextOutput    `json:"proposal"`
	Reasons   []string       `json:"reasons"`
	Actions   []ActionOutput `json:"actions"`
	// ReportChecks are the Claude Code panes the NEXT check let pass, read
	// again under the workspace's exclusion, each with what it found.
	ReportChecks []ReportCheckOutput `json:"report_checks"`
}

// ReportCheckOutput is what the NEXT check found on one Claude Code pane:
// agreed (its last report's NEXT names the command), no_report (with why,
// and the transcript's path), or no_next (no line names NEXT).
type ReportCheckOutput struct {
	Pane   string `json:"pane"`
	Result string `json:"result"`
	Detail string `json:"detail"`
}

// Output converts a tick into the JSON types. Lists are never nil.
func (o TickOutcome) Output() TickOutput {
	out := TickOutput{SnapshotOutput: o.Snapshot.Output(), AutoSends: []AutoSendOutput{}}
	for _, r := range o.AutoSends {
		a := AutoSendOutput{Workspace: Sanitize(r.Workspace), Label: Sanitize(r.Label), PR: r.PR, Task: Sanitize(r.Task), Proposal: nextOutput(r.Proposal), Reasons: sanitizeAll(r.Reasons), Actions: []ActionOutput{}, ReportChecks: []ReportCheckOutput{}}
		for _, act := range r.Actions {
			a.Actions = append(a.Actions, actionOutput(act))
		}
		for _, c := range r.ReportChecks {
			a.ReportChecks = append(a.ReportChecks, ReportCheckOutput{Pane: Sanitize(c.Pane), Result: c.Kind.String(), Detail: Sanitize(c.Detail)})
		}
		out.AutoSends = append(out.AutoSends, a)
	}
	return out
}

// WaitOutput is the output of `chief wait` when something changed.
type WaitOutput struct {
	Protocol int          `json:"protocol"`
	Outcome  string       `json:"outcome"`
	Baseline WaitBaseline `json:"baseline"`
}

// Output converts a wait into the JSON type.
func (o WaitOutcome) Output() WaitOutput {
	return WaitOutput{Protocol: Protocol, Outcome: "changed", Baseline: o.Baseline}
}

// MadeOutput is one thing a creation made.
type MadeOutput struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// CreateOutput is the output of `chief create`. result is refused (refusal
// set), stopped (stage and made set), or made (made set, with mkreq the
// result of the /mkreq send).
type CreateOutput struct {
	Protocol int    `json:"protocol"`
	Result   string `json:"result"`
	Issue    string `json:"issue"`
	Title    string `json:"title"`
	TitleCut bool   `json:"title_truncated"`
	Refusal  string `json:"refusal"`
	// Existing names what is already at the path.
	Existing  []string      `json:"existing"`
	Made      []MadeOutput  `json:"made"`
	Stage     string        `json:"stage"`
	Failure   string        `json:"failure"`
	Workspace string        `json:"workspace"`
	Label     string        `json:"label"`
	Mkreq     *ActionOutput `json:"mkreq"`
}

// notReadyReason is the reason of a /mkreq not sent because Claude Code did
// not take input after its start. Only `chief create` gives it.
const notReadyReason = "agent_not_ready"

// Output converts a creation into the JSON type. Lists are never nil; mkreq
// is null unless every stage passed.
func (o CreateOutcome) Output() CreateOutput {
	out := CreateOutput{
		Protocol: Protocol, Result: o.Result.String(), Issue: o.Issue.String(),
		// The title is already made visible and cut (Issue.Title).
		Title: o.Title, TitleCut: o.TitleCut, Refusal: o.Refusal.String(),
		Existing: sanitizeAll(o.Existing), Made: []MadeOutput{}, Stage: o.Stage.String(), Failure: Sanitize(o.Failure),
		Workspace: Sanitize(o.Workspace), Label: Sanitize(o.Label),
	}
	for _, m := range o.Made {
		out.Made = append(out.Made, MadeOutput{Kind: m.Kind.String(), ID: Sanitize(m.ID)})
	}
	switch {
	case o.NotReady != nil:
		command := ""
		if cmd, err := paneinput.NewMkreq(o.Issue); err == nil {
			command, _ = cmd.Text()
		}
		out.Mkreq = &ActionOutput{
			Workspace: Sanitize(o.Workspace), Pane: Sanitize(o.NotReady.Pane.String()), Input: InputCommand.String(),
			Command: command, Agent: agentLabel(paneinput.AgentClaudeCode),
			Outcome: OutcomeNotSent.String(), Reason: notReadyReason, Status: statusName(o.NotReady.Status),
			Detail: Sanitize(o.NotReady.Code + ": " + o.NotReady.Message), Panes: []string{},
		}
	case o.Mkreq != nil:
		a := actionOutput(*o.Mkreq)
		out.Mkreq = &a
	}
	return out
}

// TargetOutput is a workspace /mkreq can go to.
type TargetOutput struct {
	Workspace string `json:"workspace"`
	Label     string `json:"label"`
}

// TargetsOutput is the output of `chief targets`: refusal is issue_form for
// a number of another form, and workspaces the task workspaces whose row
// offers /mkreq.
type TargetsOutput struct {
	Protocol   int            `json:"protocol"`
	Issue      string         `json:"issue"`
	Refusal    string         `json:"refusal"`
	Failure    string         `json:"failure"`
	Workspaces []TargetOutput `json:"workspaces"`
}

// Output converts the targets into the JSON type. Lists are never nil.
func (o TargetsOutcome) Output() TargetsOutput {
	out := TargetsOutput{Protocol: Protocol, Issue: o.Issue.String(), Refusal: o.Refusal.String(), Failure: Sanitize(o.Failure), Workspaces: []TargetOutput{}}
	for _, w := range o.Workspaces {
		out.Workspaces = append(out.Workspaces, TargetOutput{Workspace: Sanitize(w.Workspace), Label: Sanitize(w.Label)})
	}
	return out
}
