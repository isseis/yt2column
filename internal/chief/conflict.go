package chief

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
