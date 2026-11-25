package chief

import (
	"slices"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// ConflictPhase is how far the resolution of a conflict row has gone for the
// PR's current head. The zero value is ConflictRecordsUnreadable, which never
// sends automatically.
type ConflictPhase int

const (
	// ConflictRecordsUnreadable is a row whose send records cannot be read.
	ConflictRecordsUnreadable ConflictPhase = iota
	// ConflictUnsent is a row with no resolution send for the PR's head.
	ConflictUnsent
	// ConflictSending is a row whose last resolution send has started and
	// has no end yet.
	ConflictSending
	// ConflictEndedBeforeCommand is a row whose last resolution send ended
	// before /mergemain was typed.
	ConflictEndedBeforeCommand
	// ConflictOutcomeUnknown is a row whose last resolution send cannot be
	// told to have started /mergemain or not.
	ConflictOutcomeUnknown
	// ConflictUnresolved is a row whose last resolution send started
	// /mergemain, with every agent waiting for input and the PR still
	// conflicting at the same head.
	ConflictUnresolved
)

// conflictPhaseName names a phase for the output, unknownName for a value
// outside the range.
func conflictPhaseName(p ConflictPhase) string {
	switch p {
	case ConflictRecordsUnreadable:
		return "records_unreadable"
	case ConflictUnsent:
		return "unsent"
	case ConflictSending:
		return "sending"
	case ConflictEndedBeforeCommand:
		return "ended_before_command"
	case ConflictOutcomeUnknown:
		return "outcome_unknown"
	case ConflictUnresolved:
		return "unresolved"
	default:
		return unknownName
	}
}

// conflictPhaseOf decides how far the resolution of a conflict row has gone
// for pr from the worktree's send records (the design, 3.3.2 and 3.3.3).
// Contract: if the records directory cannot be read, or any send record
// (start or end) in the directory cannot be read, it returns
// ConflictRecordsUnreadable, which never sends automatically; otherwise the
// newest /mergemain send matching pr's number and head decides the phase, or
// ConflictUnsent when none matches.
func conflictPhaseOf(records WorkspaceRecords, pr OpenPR) ConflictPhase {
	if pr.Number <= 0 || pr.Head.IsZero() {
		return ConflictRecordsUnreadable
	}
	starts, ends, err := records.sendFiles()
	if err != nil {
		return ConflictRecordsUnreadable
	}
	slices.Sort(starts)
	var lastID string
	for _, id := range starts {
		rec, err := records.readSendStart(id)
		if err != nil {
			return ConflictRecordsUnreadable
		}
		if rec.Kind == paneinput.CommandMergemain.Name() && rec.PR == pr.Number && rec.Head == pr.Head {
			lastID = id
		}
	}
	var lastEnd *sendEnd
	for id := range ends {
		end, err := records.sendEnded(id)
		if err != nil {
			return ConflictRecordsUnreadable
		}
		if id == lastID {
			lastEnd = end
		}
	}
	if lastID == "" {
		return ConflictUnsent
	}
	return conflictPhaseOfEnd(lastEnd)
}

// conflictPhaseOfEnd maps the end record of the newest /mergemain send for
// the PR and head to its phase (the design, 3.3.2). A nil end means the send
// has no end record yet; a missing step member (Release or an older record)
// is never treated as StepUnknown.
func conflictPhaseOfEnd(end *sendEnd) ConflictPhase {
	if end == nil {
		return ConflictSending
	}
	if end.Step == nil {
		return ConflictOutcomeUnknown
	}
	step, ok := parseRecordStep(*end.Step)
	if !ok {
		return ConflictOutcomeUnknown
	}
	switch end.Outcome {
	case OutcomeNotSent.String():
		if step == StepUnknown {
			return ConflictEndedBeforeCommand
		}
	case OutcomeSent.String():
		if step == StepCommand {
			return ConflictUnresolved
		}
	case OutcomeSentThenFailed.String():
		switch step {
		case StepClear, StepModel:
			return ConflictEndedBeforeCommand
		case StepCommand:
			return ConflictOutcomeUnknown
		}
	}
	return ConflictOutcomeUnknown
}
