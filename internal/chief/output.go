package chief

import (
	"time"
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
	// Sendable is true when the command can be sent with the agent and model
	// shown; false means Problem says why not.
	Sendable bool   `json:"sendable"`
	Agent    string `json:"agent"`
	Model    string `json:"model"`
	Problem  string `json:"problem"`
}

// QuestionOutput is one agent's question.
type QuestionOutput struct {
	Pane      string         `json:"pane"`
	Agent     string         `json:"agent"`
	Kind      string         `json:"kind"`
	Merge     bool           `json:"merge"`
	Relayable bool           `json:"relayable"`
	Text      string         `json:"text"`
	Options   []OptionOutput `json:"options"`
	Tabs      []string       `json:"tabs"`
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
	FirstSeen time.Time       `json:"first_seen"`
	New       bool            `json:"new"`
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
		Relayable: q.Question.Relay == RelayAllowed,
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
			row.Running = &RunningOutput{Pane: r.Running.Pane, Command: Sanitize(r.Running.Command), Agent: Sanitize(r.Running.Agent), Model: Sanitize(r.Running.Model), Started: r.Running.Started}
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
			ID:        p.ID,
			Kind:      p.Kind.String(),
			Workspace: p.Workspace,
			Label:     Sanitize(p.Label),
			PR:        prOutput(p.PR),
			Proposal:  nextOutput(p.Proposal),
			FirstSeen: p.FirstSeen,
			New:       p.New,
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
	RecordError  string         `json:"record_error"`
	Skippable    bool           `json:"review_skippable"`
	Runpr        *NextOutput    `json:"runpr"`
}

// SizeOutput is a PR's size.
type SizeOutput struct {
	ChangedFiles int `json:"changed_files"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	Commits      int `json:"commits"`
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
		Workspace: a.Workspace,
		Pane:      a.Pane,
		Input:     a.Input.String(),
		Command:   a.Command,
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
	out.Workspace, out.Label, out.Task = d.Workspace, Sanitize(d.Label), d.Task
	out.PR = prOutput(&d.PR)
	out.ViewError, out.ChecksError, out.RecordError = Sanitize(d.ViewErr), Sanitize(d.ChecksErr), Sanitize(d.RecordErr)
	if d.View != nil {
		out.Size = &SizeOutput{ChangedFiles: d.View.ChangedFiles, Additions: d.View.Additions, Deletions: d.View.Deletions, Commits: d.View.Commits}
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
	out.Skippable = d.ReviewSkippable
	out.Runpr = nextOutput(d.Runpr)
	return out
}

// Output converts a release into the JSON type.
func (o ReleaseOutcome) Output() ReleaseOutput {
	return ReleaseOutput{Protocol: Protocol, Released: o.Released, Problem: Sanitize(o.Problem)}
}
