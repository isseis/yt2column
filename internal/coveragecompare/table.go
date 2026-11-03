package coveragecompare

import (
	"cmp"
	"context"
	"slices"
	"strings"
)

// source reads the files of one point (BASE or HEAD). exists is false when the
// file is absent from the point's commit; err is a failure to read it.
type source interface {
	point() Point
	readFile(path string) (data []byte, exists bool, err error)
}

// lineRange is one line range, 1-based and inclusive.
type lineRange struct {
	Start int
	End   int
}

// fileRanges is the range helper's answer for one extension file. Path is
// relative to the repository root.
type fileRanges struct {
	Path        string
	Functions   []lineRange // every function-like node with a body
	FieldGroups []lineRange // per class: its instance fields, first to last
}

// rangeReader gives the function ranges of extension files at one point: the
// range helper run with the point's checkout as its root. It answers one entry
// per requested file.
type rangeReader interface {
	functionRanges(ctx context.Context, src source, files []string) ([]fileRanges, error)
}

// report is one point's parsed coverage report. files lists every file the
// report names, product code or not, sorted.
type report struct {
	files   []string
	profile *goProfile  // the Go target's report
	lcov    *lcovReport // the extension target's report
}

// funcEntry is one row of a function table: one Go function declaration, or
// the extension functions and field groups that start on one line.
type funcEntry struct {
	ID    FuncID
	End   int      // the last line of the range
	Names []string // empty when the report gives the row no name
	// Recorded is true when the point's report has a record for the row:
	// for Go, its file is in the profile; for the extension, its line has FN
	// records.
	Recorded bool
	Cov      Coverage // meaningful only when Recorded
}

// funcTable is the function table of one point: the functions of the product
// files read, from the source, with the report's records attached.
type funcTable struct {
	point    Point
	entries  []funcEntry         // sorted by file, then first line
	inReport map[string]struct{} // product files the point's report names
	measure  Measure
	// unmatched holds the reasons found while building the table that make
	// the run stop as unmatched. They are reported only after the
	// nothing-measured check, which comes first.
	unmatched []StopReason
}

// newFuncTable returns an empty table of a point.
func newFuncTable(point Point) funcTable {
	return funcTable{point: point, inReport: make(map[string]struct{})}
}

// lookup returns the row that starts at id.
func (t funcTable) lookup(id FuncID) (funcEntry, bool) {
	i, found := slices.BinarySearchFunc(t.entries, id, compareEntryID)
	if !found {
		return funcEntry{}, false
	}
	return t.entries[i], true
}

// fileEntries returns the rows of one file.
func (t funcTable) fileEntries(file string) []funcEntry {
	start, _ := slices.BinarySearchFunc(t.entries, FuncID{File: file}, compareEntryID)
	end := start
	for end < len(t.entries) && t.entries[end].ID.File == file {
		end++
	}
	return t.entries[start:end]
}

// enclosingRecorded returns the innermost recorded row whose range strictly
// encloses a line that is not its own first line.
func (t funcTable) enclosingRecorded(id FuncID) (funcEntry, bool) {
	var best funcEntry
	found := false
	for _, entry := range t.fileEntries(id.File) {
		if !entry.Recorded || entry.ID.Line >= id.Line || entry.End < id.Line {
			continue
		}
		if !found || entry.ID.Line > best.ID.Line {
			best, found = entry, true
		}
	}
	return best, found
}

// sortEntries puts the rows in lookup order.
func (t *funcTable) sortEntries() {
	slices.SortFunc(t.entries, func(a, b funcEntry) int {
		return compareFuncID(a.ID, b.ID)
	})
}

// compareFuncID orders function IDs by file, then line.
func compareFuncID(a, b FuncID) int {
	return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
}

// compareEntryID orders a row against an ID for binary search.
func compareEntryID(entry funcEntry, id FuncID) int {
	return compareFuncID(entry.ID, id)
}

// buildGoTable builds one point's Go function table. files are the product
// files to read, each present at the point. Every declaration becomes a row,
// and each profile block belongs to the declaration whose range holds its
// start; a block outside every declaration belongs to none. A file the
// profile does not name gives unrecorded rows. Two declarations that start on
// the same line are merged into one row and recorded as an unmatched reason.
func buildGoTable(src source, profile *goProfile, files []string) (funcTable, error) {
	point := src.point()
	table := newFuncTable(point)
	blocksByFile := make(map[string][]profileBlock)
	for _, block := range profile.Blocks {
		blocksByFile[block.File] = append(blocksByFile[block.File], block)
	}
	for _, file := range files {
		data, err := readPresent(src, file)
		if err != nil {
			return funcTable{}, err
		}
		funcs, err := parseGoFuncs(file, data, point)
		if err != nil {
			return funcTable{}, err
		}
		blocks, recorded := blocksByFile[file]
		if recorded {
			table.inReport[file] = struct{}{}
			table.measure.Files++
			table.measure.Functions += len(funcs)
		}
		table.addGoRows(file, funcs, blocks, recorded)
	}
	table.sortEntries()
	return table, nil
}

// addGoRows adds one file's declarations to the table.
func (t *funcTable) addGoRows(file string, funcs []goFunc, blocks []profileBlock, recorded bool) {
	rows := make(map[int]int)            // first line to row index
	duplicated := make(map[int]struct{}) // first lines already reported
	for _, fn := range funcs {
		covered, total := 0, 0
		for _, block := range blocks {
			start := goPosition{Line: block.StartLine, Col: block.StartCol}
			if start.before(fn.Start) || !start.before(fn.End) {
				continue
			}
			total += block.Statements
			if block.Count > 0 {
				covered += block.Statements
			}
		}
		id := FuncID{File: file, Line: fn.Start.Line}
		if index, ok := rows[id.Line]; ok {
			row := &t.entries[index]
			row.Names = append(row.Names, fn.Name)
			row.End = max(row.End, fn.End.Line)
			row.Cov.Covered += covered
			row.Cov.Total += total
			if _, reported := duplicated[id.Line]; !reported {
				duplicated[id.Line] = struct{}{}
				t.unmatched = append(t.unmatched, StopReason{
					Point:  t.point,
					Input:  InputSource,
					Func:   &FuncID{File: id.File, Line: id.Line},
					Detail: "two or more Go function declarations start on the same line",
				})
			}
			continue
		}
		rows[id.Line] = len(t.entries)
		t.entries = append(t.entries, funcEntry{
			ID:       id,
			End:      fn.End.Line,
			Names:    []string{fn.Name},
			Recorded: recorded,
			Cov:      Coverage{Unit: UnitStatements, Covered: covered, Total: total},
		})
	}
}

// buildExtTable builds one point's extension function table. files are the
// product files to read, each present at the point. The range helper's
// functions and field groups that start on one line make one row, whose range
// ends at the latest of their ends. A line with FN records gets their count
// and how many of them ran; an FN line where no range starts means the report
// and the source disagree, and stops as unreadable. Names are never used to
// match a record to a row.
func buildExtTable(ctx context.Context, src source, ranges rangeReader, lcov *lcovReport, files []string) (funcTable, error) {
	point := src.point()
	table := newFuncTable(point)
	if len(files) == 0 {
		return table, nil
	}
	answer, err := ranges.functionRanges(ctx, src, files)
	if err != nil {
		return funcTable{}, err
	}
	byPath := make(map[string]fileRanges, len(answer))
	for _, entry := range answer {
		byPath[entry.Path] = entry
	}
	recordsByFile := make(map[string][]lcovFunction, len(lcov.Files))
	for _, file := range lcov.Files {
		recordsByFile[file.Path] = file.Functions
	}
	for _, file := range files {
		fileRange, ok := byPath[file]
		if !ok {
			reason := StopReason{
				Point:  point,
				Input:  InputRanges,
				Func:   &FuncID{File: file},
				Detail: "the range helper gave no ranges for a requested file",
			}
			return funcTable{}, &StopError{Kind: StopUnreadable, Reasons: []StopReason{reason}}
		}
		records, recorded := recordsByFile[file]
		if recorded {
			table.inReport[file] = struct{}{}
			table.measure.Files++
			table.measure.Functions += len(records)
		}
		if err := table.addExtRows(file, fileRange, records); err != nil {
			return funcTable{}, err
		}
	}
	table.sortEntries()
	return table, nil
}

// addExtRows adds one file's line groups to the table and attaches the FN
// records of each line.
func (t *funcTable) addExtRows(file string, ranges fileRanges, records []lcovFunction) error {
	ends := make(map[int]int) // first line to the latest end
	for _, group := range [][]lineRange{ranges.Functions, ranges.FieldGroups} {
		for _, r := range group {
			ends[r.Start] = max(ends[r.Start], r.End)
		}
	}
	byLine := make(map[int][]lcovFunction)
	for _, record := range records {
		if _, ok := ends[record.Line]; !ok {
			reason := StopReason{
				Point:  t.point,
				Input:  InputLCOV,
				Func:   &FuncID{File: file, Line: record.Line},
				Detail: "an FN line starts no function range in the source",
			}
			return &StopError{Kind: StopUnreadable, Reasons: []StopReason{reason}}
		}
		byLine[record.Line] = append(byLine[record.Line], record)
	}
	for start, end := range ends {
		entry := funcEntry{ID: FuncID{File: file, Line: start}, End: end}
		if onLine, ok := byLine[start]; ok {
			entry.Recorded = true
			entry.Cov = Coverage{Unit: UnitFunctions, Total: len(onLine)}
			for _, record := range onLine {
				entry.Names = append(entry.Names, record.Name)
				if record.Count > 0 {
					entry.Cov.Covered++
				}
			}
		}
		t.entries = append(t.entries, entry)
	}
	return nil
}

// candidateFiles returns the files both points read: every file either
// point's report names and every path of the diff, sorted and without
// repeats. A file only one report names (one whose only test was deleted) is
// read at both points, so its rows have counterparts; a diff path is read even
// when no report names it, so a renamed file or a file no test loads still has
// its functions classified.
func candidateFiles(base, head report, patch *diffPatch) []string {
	var files []string
	files = append(files, base.files...)
	files = append(files, head.files...)
	for _, file := range patch.Files {
		for _, path := range []string{file.OldPath, file.NewPath} {
			if path != "" {
				files = append(files, path)
			}
		}
	}
	slices.Sort(files)
	return slices.Compact(files)
}

// productFiles returns the candidates that are present at the point and that
// isProduct counts as product code there. A file the point's own report names
// must be present at the point; when it is not, the report and the source
// disagree.
func productFiles(isProduct func(source, string) (bool, error), src source, rep report, candidates []string) ([]string, error) {
	var files []string
	for _, file := range candidates {
		_, exists, err := src.readFile(file)
		if err != nil {
			return nil, &StopError{Kind: StopInternal, Err: err}
		}
		if !exists {
			if _, named := slices.BinarySearch(rep.files, file); named {
				reason := StopReason{
					Point:  src.point(),
					Input:  InputSource,
					Func:   &FuncID{File: file},
					Detail: "the point's report names a file that is absent from its source",
				}
				return nil, &StopError{Kind: StopUnreadable, Reasons: []StopReason{reason}}
			}
			continue
		}
		product, err := isProduct(src, file)
		if err != nil {
			return nil, err
		}
		if product {
			files = append(files, file)
		}
	}
	return files, nil
}

// isGoSourceProduct reports whether a Go file at a point is product code.
func isGoSourceProduct(src source, file string) (bool, error) {
	data, err := readPresent(src, file)
	if err != nil {
		return false, err
	}
	return isGoProduct(file, data, src.point())
}

// isExtSourceProduct reports whether an extension file is product code. The
// decision depends on the path alone.
func isExtSourceProduct(_ source, file string) (bool, error) {
	return isExtProduct(file), nil
}

// readPresent reads a file the caller knows is present at the point. A file
// that turns out to be absent is a disagreement between the inputs.
func readPresent(src source, file string) ([]byte, error) {
	data, exists, err := src.readFile(file)
	if err != nil {
		return nil, &StopError{Kind: StopInternal, Err: err}
	}
	if !exists {
		reason := StopReason{
			Point:  src.point(),
			Input:  InputSource,
			Func:   &FuncID{File: file},
			Detail: "a file to read is absent from the point's source",
		}
		return nil, &StopError{Kind: StopUnreadable, Reasons: []StopReason{reason}}
	}
	return data, nil
}
