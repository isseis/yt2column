package coveragecompare

// baseHunk is one hunk of a context-free diff, in BASE line numbers. A hunk
// with removed lines removes BASE lines Start to Start+Removed-1; when it also
// adds lines, those removed lines are rewritten lines. A hunk without removed
// lines adds its lines after BASE line Start (0 for the start of the file).
type baseHunk struct {
	Start   int
	Removed int
	Added   int
	// RemovedText holds the removed lines in order, so the hunk can be
	// checked against the BASE source before it is moved.
	RemovedText []string
}

// pureDeletion reports whether the hunk only removes lines.
func (h baseHunk) pureDeletion() bool {
	return h.Removed > 0 && h.Added == 0
}

// last is the last BASE line the hunk removes, or the line its additions
// follow when it removes none.
func (h baseHunk) last() int {
	if h.Removed == 0 {
		return h.Start
	}
	return h.Start + h.Removed - 1
}

// before reports whether the whole hunk lies before a BASE line that no hunk
// removes or rewrites.
func (h baseHunk) before(line int) bool {
	return h.last() < line
}

// lineMapping is the line mapping of one BASE file: how each of its lines
// relates to HEAD. A line no hunk removes or rewrites maps to one HEAD line.
type lineMapping struct {
	// Deleted is true when the file does not exist at HEAD; every line of it
	// is a deleted line.
	Deleted bool
	Hunks   []baseHunk // in BASE order, none overlapping
}

// headLine returns the HEAD line of a BASE line that no hunk removes or
// rewrites. The zero lineMapping, for a file the diff does not name, maps each
// line to itself.
func (m lineMapping) headLine(line int) int {
	head := line
	for _, h := range m.Hunks {
		if h.before(line) {
			head += h.Added - h.Removed
		}
	}
	return head
}

// buildMappings reads the line mapping of every BASE file the diff names. A
// new file has no BASE lines and no mapping. The diff is taken without rename
// detection, so a file whose two paths differ, a removal hunk at line 0, and
// hunks out of order are disagreements with that form and stop as unreadable.
func buildMappings(patch *diffPatch) (map[string]lineMapping, error) {
	mappings := make(map[string]lineMapping, len(patch.Files))
	for _, file := range patch.Files {
		switch {
		case file.OldPath == "":
			continue
		case file.NewPath == "":
			mappings[file.OldPath] = lineMapping{Deleted: true}
			continue
		case file.OldPath != file.NewPath:
			return nil, diffStop(file.OldPath, "the diff names a file under two paths")
		}
		mapping := lineMapping{Hunks: make([]baseHunk, 0, len(file.Hunks))}
		for _, hunk := range file.Hunks {
			h := baseHunk{Start: hunk.BaseStart, Removed: hunk.BaseCount, Added: hunk.HeadCount}
			if h.Removed > 0 && h.Start == 0 {
				return nil, diffStop(file.OldPath, "a hunk removes lines from line 0")
			}
			if n := len(mapping.Hunks); n > 0 && h.Start <= mapping.Hunks[n-1].last() {
				return nil, diffStop(file.OldPath, "the hunks are out of order or overlap")
			}
			for _, change := range hunk.Lines {
				if !change.Added {
					h.RemovedText = append(h.RemovedText, change.Text)
				}
			}
			mapping.Hunks = append(mapping.Hunks, h)
		}
		mappings[file.OldPath] = mapping
	}
	return mappings, nil
}

// diffStop is the unreadable stop for a diff that the line mapping cannot use.
func diffStop(file, detail string) *StopError {
	reason := StopReason{Input: InputDiff, Func: &FuncID{File: file}, Detail: detail}
	return &StopError{Kind: StopUnreadable, Reasons: []StopReason{reason}}
}
