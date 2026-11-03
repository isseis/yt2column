package coveragecompare

import "math/bits"

// compareGo compares each matched Go function that ran at BASE. Coverage is
// compared as an unrounded fraction of statements, so a drop that rounding
// would hide still counts. A HEAD partner with no statements in the HEAD
// profile (its file is not in the profile, or no block belongs to it) has no
// HEAD record, and the run cannot compare it.
func compareGo(pairs []matchedPair) ([]LostFunc, []StopReason) {
	var lost []LostFunc
	var reasons []StopReason
	for _, pair := range pairs {
		before := pair.base.Cov
		// An unrecorded row and a row with no statements both cover
		// nothing, so this skips them too.
		if before.Covered == 0 {
			continue
		}
		after := pair.head.Cov
		if after.Total == 0 {
			reasons = append(reasons, StopReason{
				Point:  PointBase,
				Func:   &FuncID{File: pair.base.ID.File, Line: pair.base.ID.Line},
				Detail: "the HEAD profile records no statement of the matched HEAD function",
			})
			continue
		}
		if statementsDropped(before, after) {
			lost = append(lost, LostFunc{ID: pair.base.ID, Names: pair.base.Names, Before: before, After: after})
		}
	}
	return lost, reasons
}

// statementsDropped reports whether after covers a smaller fraction of its
// statements than before does. Both totals are positive. The products are
// compared in 128 bits, so no fraction is rounded.
func statementsDropped(before, after Coverage) bool {
	afterHi, afterLo := bits.Mul64(uint64(after.Covered), uint64(before.Total))   //nolint:gosec // counts are non-negative
	beforeHi, beforeLo := bits.Mul64(uint64(before.Covered), uint64(after.Total)) //nolint:gosec // counts are non-negative
	return afterHi < beforeHi || (afterHi == beforeHi && afterLo < beforeLo)
}

// compareExt compares each matched line group of extension functions that has
// FN records at BASE, by how many of the line's functions ran. A HEAD file the
// HEAD lcov does not name ran nothing: its line has the BASE line's number of
// functions, none of them run. Otherwise the HEAD line must have the same
// number of FN records as the BASE line; fewer that ran is a drop, and more
// that ran, a missing line, or a different number cannot be compared. When a
// line goes missing or loses records, the innermost recorded HEAD function
// around it is named as the likely cause if none of its functions ran.
func compareExt(head funcTable, pairs []matchedPair) ([]LostFunc, []StopReason) {
	var lost []LostFunc
	var reasons []StopReason
	for _, pair := range pairs {
		before := pair.base.Cov
		if !pair.base.Recorded {
			continue
		}
		unmatched := func(detail string, withOuter bool) {
			reason := StopReason{
				Point:  PointBase,
				Func:   &FuncID{File: pair.base.ID.File, Line: pair.base.ID.Line},
				Detail: detail,
			}
			if withOuter {
				if outer, ok := head.enclosingRecorded(pair.head.ID); ok && outer.Cov.Covered == 0 {
					reason.Outer = &FuncID{File: outer.ID.File, Line: outer.ID.Line}
				}
			}
			reasons = append(reasons, reason)
		}
		after := pair.head.Cov
		_, fileInReport := head.inReport[pair.head.ID.File]
		switch {
		case !fileInReport:
			after = Coverage{Unit: UnitFunctions, Covered: 0, Total: before.Total}
		case after.Total != before.Total:
			// A line without FN records has a total of zero, so a missing
			// line is a line that lost every record.
			detail := "the number of functions on the line changed"
			if !pair.head.Recorded {
				detail = "the line's FN records are missing from the HEAD lcov"
			}
			unmatched(detail, after.Total < before.Total)
			continue
		}
		switch {
		case after.Covered < before.Covered:
			lost = append(lost, LostFunc{ID: pair.base.ID, Names: pair.base.Names, Before: before, After: after})
		case after.Covered > before.Covered:
			unmatched("more functions on the line ran at HEAD than at BASE", false)
		}
	}
	return lost, reasons
}

// runState is what a deleted function's BASE record says: not run when the
// report has no record of it or it ran nothing, no statements for a Go
// function without any, and ran otherwise.
func runState(entry funcEntry) RunState {
	if !entry.Recorded {
		return NotRunAtBase
	}
	switch entry.Cov.Unit {
	case UnitStatements:
		switch {
		case entry.Cov.Total == 0:
			return NoStatements
		case entry.Cov.Covered == 0:
			return NotRunAtBase
		default:
			return RanAtBase
		}
	case UnitFunctions:
		if entry.Cov.Covered == 0 {
			return NotRunAtBase
		}
		return RanAtBase
	default:
		return RanUnknown
	}
}
