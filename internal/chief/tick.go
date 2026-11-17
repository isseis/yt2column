package chief

import (
	"context"
	"slices"
	"strconv"
	"time"
)

// TickOutcome is what `chief tick` did: the snapshot of the round, read
// before any send, and the merged PRs an automatic send was considered for.
type TickOutcome struct {
	Snapshot  Snapshot
	AutoSends []AutoSendResult
}

// Tick is one round of chief's wait (the design, 3.1.1 and 3.7): it records
// the round, takes the snapshot, records an observation of every open PR of
// a task workspace, and considers each PR the previous round of this session
// saw open for an automatic send. The round is recorded with the time its
// reading began, so a send made by hand while the reading runs counts as made
// since this round. Contract: only a PR the previous round saw open, when
// that round is less than observeWindow before this one, can be sent
// automatically; a round that cannot be recorded, or a previous round that
// cannot be read, makes no observation and sends nothing; every merged
// PR that is not sent is shown as a proposal with the conditions that did
// not hold.
func Tick(ctx context.Context, deps Deps, owner Owner) (TickOutcome, error) {
	// The round's time is taken before the reading it covers starts, so a
	// command the human sends by hand while the workspaces are read counts as
	// sent since this round, and the round that sends next sees it.
	at := deps.Now()
	view, err := readRound(ctx, deps)
	if err != nil {
		// No round is recorded: a round that read nothing would become the
		// next tick's previous round and drop every observation before it.
		return TickOutcome{}, err
	}
	start, roundErr := deps.Store.BeginRound(owner.Session, at)
	if roundErr != nil {
		view.problems = append(view.problems, "the round cannot be recorded, so nothing is observed or sent automatically: "+roundErr.Error())
		return TickOutcome{Snapshot: finishSnapshot(deps, owner, view, nil)}, nil
	}
	if start.PruneErr != nil {
		view.problems = append(view.problems, "a record of an older round cannot be removed: "+start.PruneErr.Error())
	}
	open := map[observedPR]struct{}{}
	for i, row := range view.rows {
		f := view.facts[i]
		if row.PR == nil || f.Task.IsZero() || f.Head != headBranch {
			continue
		}
		open[observedPR{row.Workspace, row.PR.Number}] = struct{}{}
		obs := Observation{Session: owner.Session, Round: start.Cur.ID, PR: row.PR.Number, Worktree: row.Workspace, Task: f.Task.String(), Branch: f.Branch}
		if err := deps.Store.RecordObservation(obs); err != nil {
			view.problems = append(view.problems, "the observation of #"+strconv.Itoa(row.PR.Number)+" cannot be recorded: "+err.Error())
		}
	}
	var prev []Observation
	var prevAt time.Time
	if start.Prev != nil && start.Cur.At.Sub(start.Prev.At) < observeWindow {
		prevAt = start.Prev.At
		var errs []error
		prev, errs = deps.Store.Observations(owner.Session, start.Prev.ID)
		for _, err := range errs {
			view.problems = append(view.problems, "an observation of the previous round cannot be read: "+err.Error())
		}
	}
	a := &autoSender{deps: deps, view: &view, owner: owner, round: start.Cur, prevAt: prevAt, herdr: NewHerdr(deps.Cmds), git: NewGitReader(deps.Cmds), tasks: NewTasks(deps.Cmds)}
	results := a.run(ctx, prev, open)
	extra := applyAutoSends(&view, results)
	return TickOutcome{Snapshot: finishSnapshot(deps, owner, view, extra), AutoSends: results}, nil
}

// applyAutoSends updates the rows of the workspaces an automatic send went
// to, read before it, and returns the proposals of the PRs not sent. A row
// sent to is running what was sent; a row whose send failed after an input
// needs a check, since its workspace is stopped. No row an automatic send
// concerns stays idle.
func applyAutoSends(view *roundView, results []AutoSendResult) []PendingItem {
	var extra []PendingItem
	for _, r := range results {
		var sent *ActionResult
		if len(r.Actions) > 0 && r.Actions[0].Input == InputCommand {
			sent = &r.Actions[0]
		}
		row := rowAt(view.rows, r.Workspace)
		if row != nil {
			// An automatic send or its proposal is something to look at, so
			// the workspace is no longer idle.
			row.Idle = false
		}
		switch {
		case sent != nil && sent.Outcome == OutcomeSent && row != nil:
			row.State, row.Next, row.Takeover, row.Unreadable = StateRunning, nil, nil, nil
			// A pane the send typed into has no skipped reset left.
			row.Resets = slices.DeleteFunc(row.Resets, func(it ResetItem) bool {
				return slices.ContainsFunc(r.Actions, func(a ActionResult) bool {
					return a.Pane == it.Pane.String() && (a.Outcome == OutcomeSent || a.Outcome == OutcomeSentThenFailed)
				})
			})
			row.Running = &SendRecord{Pane: sent.Pane, Input: InputCommand.String(), Command: sent.Command, Agent: agentLabel(sent.Agent), Model: sent.Model}
		case sent != nil && sent.Outcome == OutcomeSentThenFailed && row != nil:
			row.State, row.Next, row.Takeover = StateNeedsCheck, nil, nil
			row.Unreadable = []string{"the automatic send failed after an input, so the workspace is stopped until release"}
		case r.Proposal != nil:
			extra = append(extra, PendingItem{ID: proposalID(r.Workspace, r.Proposal), Kind: PendingProposal, Workspace: r.Workspace, Label: r.Label, Proposal: r.Proposal, Reasons: r.Reasons})
		}
	}
	return extra
}

// rowAt returns the row of worktree, nil when the round has none.
func rowAt(rows []Row, worktree string) *Row {
	for i := range rows {
		if rows[i].Workspace == worktree {
			return &rows[i]
		}
	}
	return nil
}
