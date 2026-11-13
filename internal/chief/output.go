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
		return "unknown"
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
