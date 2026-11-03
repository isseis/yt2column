package coveragecompare

import (
	"slices"
)

// matchedPair is a remaining BASE function and the HEAD function that starts
// at its first line's HEAD position.
type matchedPair struct {
	base funcEntry
	head funcEntry
}

// classification is how the BASE functions relate to HEAD: the deleted
// functions, the matched pairs to compare, and the reasons the run cannot
// match every function.
type classification struct {
	deleted   []funcEntry
	pairs     []matchedPair
	unmatched []StopReason
}

// rangeClass is how the diff touches one function's range.
type rangeClass int

const (
	// rangeRewritten is the zero value: the range has rewritten lines, the
	// first of the unmatched classes.
	rangeRewritten rangeClass = iota
	rangePartlyDeleted
	rangeAddedInside
	rangeDeleted
	rangeRemains
)

// unmatchedDetail is the stop reason's sentence for an unmatched class.
func (c rangeClass) unmatchedDetail() string {
	switch c {
	case rangePartlyDeleted:
		return "only part of the function's range is deleted"
	case rangeAddedInside:
		return "lines are added inside the function's range"
	case rangeRewritten:
		return "the function's range has rewritten lines"
	default:
		return "the function's range has rewritten lines"
	}
}

// classify sorts every BASE function into deleted, remaining, or unmatched,
// and pairs each remaining function with the HEAD function that starts at its
// first line's HEAD position. A remaining function with no such partner, and a
// HEAD function no remaining function pairs with, are unmatched. Before a file
// is classified, each hunk that only deletes lines is moved to a position that
// deletes no function's range in part, when the BASE text allows the same
// deletion there. baseSrc gives the BASE text of the files whose lines the diff
// removes; the removed lines must match it.
func classify(base, head funcTable, patch *diffPatch, baseSrc source) (classification, error) {
	mappings, err := buildMappings(patch)
	if err != nil {
		return classification{}, err
	}
	var result classification
	matched := make(map[FuncID]struct{})
	for start := 0; start < len(base.entries); {
		file := base.entries[start].ID.File
		end := start
		for end < len(base.entries) && base.entries[end].ID.File == file {
			end++
		}
		entries := base.entries[start:end]
		start = end

		mapping := mappings[file]
		if mapping.Deleted {
			result.deleted = append(result.deleted, entries...)
			continue
		}
		if err := checkRemovedText(baseSrc, file, mapping); err != nil {
			return classification{}, err
		}
		if len(mapping.Hunks) > 0 {
			reasons, err := placeDeletions(baseSrc, file, &mapping, entries)
			if err != nil {
				return classification{}, err
			}
			result.unmatched = append(result.unmatched, reasons...)
		}
		for _, entry := range entries {
			result.classifyEntry(entry, mapping, head, matched)
		}
	}
	for _, entry := range head.entries {
		if _, ok := matched[entry.ID]; ok {
			continue
		}
		result.unmatched = append(result.unmatched, StopReason{
			Point:  PointHead,
			Func:   &FuncID{File: entry.ID.File, Line: entry.ID.Line},
			Detail: "a HEAD function matches no remaining BASE function",
		})
	}
	return result, nil
}

// classifyEntry sorts one BASE function.
func (c *classification) classifyEntry(entry funcEntry, mapping lineMapping, head funcTable, matched map[FuncID]struct{}) {
	class := mapping.classifyRange(entry.ID.Line, entry.End)
	switch class {
	case rangeDeleted:
		c.deleted = append(c.deleted, entry)
	case rangeRemains:
		partner := FuncID{File: entry.ID.File, Line: mapping.headLine(entry.ID.Line)}
		headEntry, ok := head.lookup(partner)
		if !ok {
			c.unmatched = append(c.unmatched, StopReason{
				Point:  PointBase,
				Func:   &FuncID{File: entry.ID.File, Line: entry.ID.Line},
				Detail: "no HEAD function starts at the remaining function's HEAD position",
			})
			return
		}
		matched[partner] = struct{}{}
		c.pairs = append(c.pairs, matchedPair{base: entry, head: headEntry})
	default:
		c.unmatched = append(c.unmatched, StopReason{
			Point:  PointBase,
			Func:   &FuncID{File: entry.ID.File, Line: entry.ID.Line},
			Detail: class.unmatchedDetail(),
		})
	}
}

// classifyRange reports how the hunks touch the range [first, last]: deleted
// when one hunk that only deletes lines covers it, remaining when no hunk
// removes or rewrites a line of it and no line is added inside it, and one of
// the unmatched classes otherwise. An addition after the last line or right
// before the first line is outside the range; one after any other line of it
// is inside. A hunk that rewrites lines puts its added lines where the
// rewritten lines were, so its rewritten lines alone decide.
func (m lineMapping) classifyRange(first, last int) rangeClass {
	rewritten, partial, added := false, false, false
	for _, h := range m.Hunks {
		if h.Removed == 0 {
			if first <= h.Start && h.Start < last {
				added = true
			}
			continue
		}
		if h.Start > last || h.last() < first {
			continue
		}
		switch {
		case h.pureDeletion() && h.Start <= first && last <= h.last():
			return rangeDeleted
		case h.pureDeletion():
			partial = true
		default:
			rewritten = true
		}
	}
	switch {
	case rewritten:
		return rangeRewritten
	case partial:
		return rangePartlyDeleted
	case added:
		return rangeAddedInside
	default:
		return rangeRemains
	}
}

// checkRemovedText checks that every line a hunk removes is the BASE line at
// that position, so a diff that does not belong to the BASE source stops
// instead of being classified.
func checkRemovedText(baseSrc source, file string, mapping lineMapping) error {
	if !slices.ContainsFunc(mapping.Hunks, func(h baseHunk) bool { return h.Removed > 0 }) {
		return nil
	}
	lines, err := baseLines(baseSrc, file)
	if err != nil {
		return err
	}
	for _, h := range mapping.Hunks {
		for i, text := range h.RemovedText {
			line := h.Start + i
			if line > len(lines) || lines[line-1] != text {
				return diffStop(file, "a removed line does not match the BASE source")
			}
		}
	}
	return nil
}

// baseLines reads a BASE file as lines.
func baseLines(baseSrc source, file string) ([]string, error) {
	data, err := readPresent(baseSrc, file)
	if err != nil {
		return nil, err
	}
	return splitLines(string(data)), nil
}

// placeDeletions moves each hunk of a file that only deletes lines to a
// position that deletes no function's range in part: an accepted position.
// Moving a block of deleted lines by one line gives the same HEAD text when
// the line that enters the block equals the line that leaves it, so every
// such position is the same diff. The block never moves next to or across
// another hunk. With no accepted position, the hunk stays where git put it
// and the range check reports the function it cuts. With several accepted
// positions that delete different functions, which function was deleted is
// not decided by the diff, and the run stops as unmatched. Otherwise git's
// position is kept when it is accepted, and the earliest accepted position is
// used when it is not; every accepted position then deletes the same
// functions.
func placeDeletions(baseSrc source, file string, mapping *lineMapping, entries []funcEntry) ([]StopReason, error) {
	if !slices.ContainsFunc(mapping.Hunks, baseHunk.pureDeletion) {
		return nil, nil
	}
	lines, err := baseLines(baseSrc, file)
	if err != nil {
		return nil, err
	}
	var reasons []StopReason
	for i, h := range mapping.Hunks {
		if !h.pureDeletion() {
			continue
		}
		first, last := mapping.slideWindow(i, lines)
		var accepted []int
		var deletedSets [][]FuncID
		for start := first; start <= last; start++ {
			deleted, ok := deletionAt(entries, start, start+h.Removed-1)
			if ok {
				accepted = append(accepted, start)
				deletedSets = append(deletedSets, deleted)
			}
		}
		if len(accepted) == 0 {
			continue
		}
		if differing, ambiguous := differingFunc(deletedSets); ambiguous {
			reasons = append(reasons, StopReason{
				Point:  PointBase,
				Input:  InputDiff,
				Func:   &differing,
				Detail: "the deleted lines fit several positions that delete different functions",
			})
			continue
		}
		if !slices.Contains(accepted, h.Start) {
			mapping.Hunks[i].Start = accepted[0]
		}
	}
	return reasons, nil
}

// slideWindow returns the earliest and the latest start the pure deletion
// hunk i can take while it deletes the same text and stays apart from every
// other hunk of the file.
func (m lineMapping) slideWindow(i int, lines []string) (int, int) {
	h := m.Hunks[i]
	text := func(line int) string { return lines[line-1] }
	blockEnd := func(start int) int { return start + h.Removed - 1 }
	first := h.Start
	// Moving up one line: the line above the block enters it, and the
	// block's last line leaves it.
	for first > 1 && text(first-1) == text(blockEnd(first)) && m.apart(i, first-1, blockEnd(first-1)) {
		first--
	}
	last := h.Start
	// Moving down one line: the line below the block enters it, and the
	// block's first line leaves it.
	for blockEnd(last+1) <= len(lines) && text(blockEnd(last+1)) == text(last) && m.apart(i, last+1, blockEnd(last+1)) {
		last++
	}
	return first, last
}

// apart reports whether a block of deleted lines [first, last] neither
// overlaps nor touches any hunk of the file other than hunk i. A block that
// touched another hunk would be one hunk in git's diff, with a different
// meaning.
func (m lineMapping) apart(i, first, last int) bool {
	for j, other := range m.Hunks {
		if j == i {
			continue
		}
		if other.Removed == 0 {
			// Additions after line k touch a block that ends at k or starts
			// at k+1, and fall inside one that spans both.
			if first <= other.Start+1 && last >= other.Start {
				return false
			}
			continue
		}
		if first <= other.last()+1 && last >= other.Start-1 {
			return false
		}
	}
	return true
}

// deletionAt reports whether deleting [first, last] cuts no function's range
// in part, and the functions whose whole range it deletes.
func deletionAt(entries []funcEntry, first, last int) ([]FuncID, bool) {
	var deleted []FuncID
	for _, entry := range entries {
		overlaps := entry.ID.Line <= last && entry.End >= first
		inside := first <= entry.ID.Line && entry.End <= last
		switch {
		case inside:
			deleted = append(deleted, entry.ID)
		case overlaps:
			return nil, false
		}
	}
	return deleted, true
}

// differingFunc reports whether the sets differ, and the first function one set
// holds and another does not.
func differingFunc(sets [][]FuncID) (FuncID, bool) {
	for _, set := range sets[1:] {
		if slices.Equal(set, sets[0]) {
			continue
		}
		for _, id := range sets[0] {
			if !slices.Contains(set, id) {
				return id, true
			}
		}
		for _, id := range set {
			if !slices.Contains(sets[0], id) {
				return id, true
			}
		}
	}
	return FuncID{}, false
}
