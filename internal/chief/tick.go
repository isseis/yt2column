package chief

import (
	"context"
	"errors"
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
// the round, records an observation of every open PR of a task workspace,
// writes the session's history of each worktree, makes the rows from it,
// and considers each PR the previous round of this session saw open for an
// automatic send. The round is recorded with the time its
// reading began, so a send made by hand while the reading runs counts as made
// since this round. Contract: only a PR the previous round saw open, when
// that round is less than observeWindow before this one, can be sent
// automatically; a round that cannot be recorded, or a previous round that
// cannot be read, makes no observation and sends nothing; a round that
// cannot take the creation exclusion sends nothing and carries every PR it
// would have sent; every merged PR that is not sent is shown as a proposal
// with the conditions that did not hold; a round that cannot be recorded
// writes no history, and the rows go by the history already written.
func Tick(ctx context.Context, deps Deps, owner Owner) (TickOutcome, error) {
	// The round's time is taken before the reading it covers starts, so a
	// command the human sends by hand while the workspaces are read counts as
	// sent since this round, and the round that sends next sees it.
	at := deps.Now()
	view, err := readFacts(ctx, deps)
	if err != nil {
		// No round is recorded: a round that read nothing would become the
		// next tick's previous round and drop every observation before it.
		return TickOutcome{}, err
	}
	// The table sources are read before the round is recorded, so both
	// snapshots below lay out the same table.
	src := readTableSources(ctx, deps, view)
	start, roundErr := deps.Store.BeginRound(owner.Session, at)
	if roundErr != nil {
		view.problems = append(view.problems, "the round cannot be recorded, so nothing is observed or sent automatically: "+roundErr.Error())
		// Nothing is written; the rows go by the history written so far.
		buildRows(ctx, deps, &view, deps.Store.readTraces(owner.Session), nil)
		return TickOutcome{Snapshot: finishSnapshot(deps, owner, view, src, nil)}, nil
	}
	if start.PruneErr != nil {
		view.problems = append(view.problems, "a record of an older round cannot be removed: "+start.PruneErr.Error())
	}
	open := recordObservations(deps, &view, owner, start.Cur.ID)
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
	// The history is written before the rows are made, so the round that
	// reads a merge already shows the task it identifies; the creation
	// exclusion is not needed for it.
	unread, problems := writeHistory(ctx, deps, &view, owner, start.Cur.ID)
	view.problems = append(view.problems, problems...)
	buildRows(ctx, deps, &view, deps.Store.readTraces(owner.Session), unread)
	// The creation exclusion is held from before the first automatic send
	// until the last one ends, so `create` never starts between a send's
	// check and its input.
	creation, lockErr := deps.Store.LockCreation()
	if lockErr != nil {
		view.problems = append(view.problems, "the creation exclusion cannot be taken, so nothing is sent automatically this round: "+lockErr.Error())
	}
	a := &autoSender{deps: deps, view: &view, owner: owner, round: start.Cur, prevAt: prevAt, herdr: NewHerdr(deps.Cmds), git: NewGitReader(deps.Cmds), tasks: NewTasks(deps.Cmds), noSend: lockErr}
	results := a.run(ctx, prev, open)
	if creation != nil {
		if err := creation.Close(); err != nil {
			view.problems = append(view.problems, "the creation exclusion cannot be released: "+err.Error())
		}
	}
	extra := applyAutoSends(&view, deps.Store, results)
	return TickOutcome{Snapshot: finishSnapshot(deps, owner, view, src, extra), AutoSends: results}, nil
}

// recordObservations records an observation of every open PR of a feature
// branch of a task workspace for the round, and returns those PRs. Contract:
// it reads the facts the round read, never the rows, so it observes what it
// did before the rows were made later in the round; a worktree on main is
// not observed, whatever its history.
func recordObservations(deps Deps, view *roundView, owner Owner, round string) map[observedPR]struct{} {
	open := map[observedPR]struct{}{}
	for _, f := range view.facts {
		if f.PR == nil || f.Task.IsZero() || f.Head != headBranch {
			continue
		}
		open[observedPR{f.Workspace, f.PR.Number}] = struct{}{}
		obs := Observation{Session: owner.Session, Round: round, PR: f.PR.Number, Worktree: f.Workspace, Task: f.Task.String(), Branch: f.Branch}
		if err := deps.Store.RecordObservation(obs); err != nil {
			view.problems = append(view.problems, "the observation of #"+strconv.Itoa(f.PR.Number)+" cannot be recorded: "+err.Error())
		}
	}
	return open
}

// applyAutoSends updates the rows of the workspaces an automatic send went
// to, read before it, and returns the proposals of the PRs not sent. A row
// sent to is running the proposal's command with the model sent; a row
// whose send failed after an input needs a check. No row an automatic send
// concerns stays idle. Contract: the running command is made from the
// proposal's declared command, never from the text sent or from a
// transcript; the stop of every workspace an input was typed into is read
// again from store, so a stop the send made shows in this round, and a stop
// that cannot be read again marks the row StopUnreadable and is named in
// the round's problems.
func applyAutoSends(view *roundView, store Store, results []AutoSendResult) []PendingItem {
	var extra []PendingItem
	for _, r := range results {
		var sent *ActionResult
		if len(r.Actions) > 0 && r.Actions[0].Input == InputCommand {
			sent = &r.Actions[0]
		}
		i := slices.IndexFunc(view.rows, func(row Row) bool { return row.Workspace == r.Workspace })
		var row *Row
		if i >= 0 {
			row = &view.rows[i]
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
			row.Running, row.RunningUnknown = sentRunning(r.Proposal, sent.Model)
		case sent != nil && sent.Outcome == OutcomeSentThenFailed && row != nil:
			row.State, row.Next, row.Takeover = StateNeedsCheck, nil, nil
			row.Unreadable = []string{"the automatic send failed after an input, so the workspace is stopped until release"}
		case r.Proposal != nil:
			extra = append(extra, PendingItem{ID: proposalID(r.Workspace, r.Proposal), Kind: PendingProposal, Workspace: r.Workspace, Label: r.Label, Proposal: r.Proposal, Reasons: r.Reasons})
		}
		if row != nil && slices.ContainsFunc(r.Actions, typedInput) {
			if err := rereadStop(view, i, store); err != nil {
				view.problems = append(view.problems, "the stop of "+r.Workspace+" cannot be read again after the automatic send: "+err.Error())
			}
		}
	}
	return extra
}

// sentRunning is the running command of a row an automatic send sent the
// proposal's command to, with the model sent; nil when the proposal names no
// command chief can show.
func sentRunning(proposal *Next, model string) (*RunningShown, RunningUnknown) {
	if proposal == nil || proposal.Command == nil {
		return nil, RunningUnknownNoSentCommand
	}
	kind, arg, err := proposal.Command.Parts()
	if err != nil {
		return nil, RunningUnknownNoSentCommand
	}
	cmd, dropped, err := newRunningCommand(kind, arg)
	if err != nil || dropped {
		return nil, RunningUnknownNoSentCommand
	}
	return &RunningShown{Command: cmd, Model: model}, RunningUnknownNone
}

// typedInput reports whether an action may have typed an input into its
// pane: every outcome but the two that typed nothing.
func typedInput(a ActionResult) bool {
	return a.Outcome != OutcomeNotSent && a.Outcome != OutcomeRecordedNotSent
}

// errNoRoundWorktree reports a row whose worktree the round did not find.
var errNoRoundWorktree = errors.New("the round has no worktree for the row")

// rereadStop reads the stop of the workspace of the round's row i again.
// A workspace the round did not find, or whose stop cannot be read, is
// StopUnreadable.
func rereadStop(view *roundView, i int, store Store) error {
	row := &view.rows[i]
	if i >= len(view.found) || view.found[i].Worktree != row.Workspace {
		row.StopUnreadable = true
		return errNoRoundWorktree
	}
	records, ok, err := store.existingRecords(view.found[i].Tree)
	if err != nil {
		row.StopUnreadable = true
		return err
	}
	if !ok {
		return nil
	}
	stop, err := records.Stopped()
	if err != nil {
		row.StopUnreadable = true
		return err
	}
	if stop != nil {
		row.Stopped = stop
	}
	return nil
}
