package coveragecompare

import (
	"regexp"
	"strings"
)

// diffHunkRE matches a hunk header. The trailing function name is ignored. A
// missing count means one line.
var diffHunkRE = regexp.MustCompile(
	`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?: .*)?$`,
)

// diffMetaRE matches the file header lines other than the paths: the blob
// index and the mode changes.
var diffMetaRE = regexp.MustCompile(
	`^index [0-9a-f]+\.\.[0-9a-f]+(?: [0-7]{6})?$|^(?:new file mode|deleted file mode|old mode|new mode) [0-7]{6}$`,
)

// diffChange is one removed or added line of a hunk. Text is without the
// leading "-" or "+".
type diffChange struct {
	Added bool
	Text  string
}

// diffHunk is one @@ header and its lines. With no context lines, every line is
// a removal or an addition.
type diffHunk struct {
	BaseStart int
	BaseCount int
	HeadStart int
	HeadCount int
	Lines     []diffChange
}

// diffFile is one file's diff. OldPath is empty for a new file and NewPath is
// empty for a deleted one; both are relative to the repository root.
type diffFile struct {
	OldPath string
	NewPath string
	Hunks   []diffHunk
}

// diffPatch is the output of one git diff command.
type diffPatch struct {
	Files []diffFile
}

// diffParser holds the state of one diff parse.
type diffParser struct {
	patch   *diffPatch
	point   Point
	current *diffFile
	hunk    *diffHunk
	oldSeen bool
	newSeen bool
	created bool // the file header has "new file mode"
	deleted bool // the file header has "deleted file mode"
}

// parseDiff reads git diff output taken with no context lines. It accepts only
// the file headers and hunk lines the requirements list: a file created,
// modified, or deleted, and hunks whose removed and added line counts match
// their headers. Anything else, including a binary file, a quoted path, a path
// with whitespace, and a context line, stops as unreadable.
func parseDiff(data []byte, point Point) (*diffPatch, error) {
	parser := &diffParser{patch: &diffPatch{}, point: point}
	for i, line := range splitLines(string(data)) {
		if err := parser.line(line, i+1); err != nil {
			return nil, err
		}
	}
	if err := parser.finishFile(0, ""); err != nil {
		return nil, err
	}
	return parser.patch, nil
}

// line takes one line of the patch.
func (p *diffParser) line(line string, lineNo int) error {
	if p.hunk != nil && p.hunkLine(line) {
		return nil
	}
	switch {
	case strings.HasPrefix(line, "diff --git "):
		return p.startFile(line, lineNo)
	case p.current == nil:
		return stopError(p.point, InputDiff, lineNo, "a line appears before the first file header", line)
	case diffMetaRE.MatchString(line):
		if p.oldSeen || p.newSeen || p.hunk != nil {
			return stopError(p.point, InputDiff, lineNo, "a file header line appears after its paths", line)
		}
		switch {
		case strings.HasPrefix(line, "new file mode "):
			p.created = true
		case strings.HasPrefix(line, "deleted file mode "):
			p.deleted = true
		}
		return nil
	case strings.HasPrefix(line, "--- "):
		return p.startOldPath(line, lineNo)
	case strings.HasPrefix(line, "+++ "):
		return p.startNewPath(line, lineNo)
	case strings.HasPrefix(line, "Binary files "):
		return stopError(p.point, InputDiff, lineNo, "a binary file's diff is not accepted", line)
	case strings.HasPrefix(line, "@@ "):
		return p.startHunk(line, lineNo)
	default:
		return stopError(p.point, InputDiff, lineNo, "the line is not an accepted diff line", line)
	}
}

// hunkLine takes a line inside an open hunk. It reports false when the line
// belongs to the next hunk or file.
func (p *diffParser) hunkLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "+"):
		p.hunk.Lines = append(p.hunk.Lines, diffChange{Added: true, Text: line[1:]})
		return true
	case strings.HasPrefix(line, "-"):
		p.hunk.Lines = append(p.hunk.Lines, diffChange{Text: line[1:]})
		return true
	case line == `\ No newline at end of file`:
		return true
	default:
		return false
	}
}

// startFile finishes the previous file and begins a new one.
func (p *diffParser) startFile(line string, lineNo int) error {
	if err := p.finishFile(lineNo, line); err != nil {
		return err
	}
	oldPath, newPath, ok := parseDiffHeader(line)
	if !ok {
		return stopError(p.point, InputDiff, lineNo, "the diff header is not accepted", line)
	}
	p.current = &diffFile{OldPath: oldPath, NewPath: newPath}
	return nil
}

// startOldPath records the --- path.
func (p *diffParser) startOldPath(line string, lineNo int) error {
	if p.oldSeen {
		return stopError(p.point, InputDiff, lineNo, "a second --- line appears", line)
	}
	path, ok := parseDiffPath(line, "--- ", "a/")
	if !ok || (path != "" && path != p.current.OldPath) || (path == "") != p.created {
		return stopError(p.point, InputDiff, lineNo, "the --- path is not accepted", line)
	}
	p.current.OldPath = path
	p.oldSeen = true
	return nil
}

// startNewPath records the +++ path.
func (p *diffParser) startNewPath(line string, lineNo int) error {
	if !p.oldSeen || p.newSeen {
		return stopError(p.point, InputDiff, lineNo, "the +++ line has no matching --- line", line)
	}
	path, ok := parseDiffPath(line, "+++ ", "b/")
	if !ok || (path != "" && path != p.current.NewPath) || (path == "") != p.deleted {
		return stopError(p.point, InputDiff, lineNo, "the +++ path is not accepted", line)
	}
	p.current.NewPath = path
	p.newSeen = true
	return nil
}

// startHunk finishes the previous hunk and begins a new one.
func (p *diffParser) startHunk(line string, lineNo int) error {
	if err := p.finishHunk(lineNo, line); err != nil {
		return err
	}
	if !p.oldSeen || !p.newSeen {
		return stopError(p.point, InputDiff, lineNo, "a hunk header appears before the file paths", line)
	}
	parsed, ok := parseDiffHunkHeader(line)
	if !ok {
		return stopError(p.point, InputDiff, lineNo, "the hunk header is not accepted", line)
	}
	p.hunk = &parsed
	return nil
}

// finishHunk checks an open hunk's line counts and stores it.
func (p *diffParser) finishHunk(lineNo int, excerpt string) error {
	if p.hunk == nil {
		return nil
	}
	removed, added := 0, 0
	for _, change := range p.hunk.Lines {
		if change.Added {
			added++
		} else {
			removed++
		}
	}
	if removed != p.hunk.BaseCount || added != p.hunk.HeadCount {
		return stopError(p.point, InputDiff, lineNo, "the hunk's line counts do not match its header", excerpt)
	}
	p.current.Hunks = append(p.current.Hunks, *p.hunk)
	p.hunk = nil
	return nil
}

// finishFile checks and stores the current file.
func (p *diffParser) finishFile(lineNo int, excerpt string) error {
	if err := p.finishHunk(lineNo, excerpt); err != nil {
		return err
	}
	if p.current == nil {
		return nil
	}
	if p.oldSeen != p.newSeen {
		return stopError(p.point, InputDiff, lineNo, "a file header is missing its --- or +++ line", excerpt)
	}
	if p.created && p.deleted {
		return stopError(p.point, InputDiff, lineNo, "a file is new and deleted at once", excerpt)
	}
	// A file with no --- and +++ lines (an empty file created or deleted, or
	// a mode change) takes its paths from the diff --git line, so the mode
	// lines decide which side is absent.
	if p.created {
		p.current.OldPath = ""
	}
	if p.deleted {
		p.current.NewPath = ""
	}
	p.patch.Files = append(p.patch.Files, *p.current)
	p.current = nil
	p.oldSeen = false
	p.newSeen = false
	p.created = false
	p.deleted = false
	return nil
}

// parseDiffHeader reads the two paths of "diff --git a/<path> b/<path>". A
// path with whitespace or a quote is rejected because git's own line cannot
// then be split into two paths unambiguously.
func parseDiffHeader(line string) (oldPath, newPath string, ok bool) {
	rest := strings.TrimPrefix(line, "diff --git ")
	if !strings.HasPrefix(rest, "a/") {
		return "", "", false
	}
	split := strings.Index(rest, " b/")
	if split < 0 {
		return "", "", false
	}
	oldPath = rest[len("a/"):split]
	newPath = rest[split+len(" b/"):]
	if !validDiffPath(oldPath) || !validDiffPath(newPath) {
		return "", "", false
	}
	return oldPath, newPath, true
}

// parseDiffPath reads the path of a "--- " or "+++ " line. /dev/null is the
// empty path.
func parseDiffPath(line, marker, prefix string) (path string, ok bool) {
	rest := strings.TrimPrefix(line, marker)
	if rest == "/dev/null" {
		return "", true
	}
	rel, found := strings.CutPrefix(rest, prefix)
	if !found || !validDiffPath(rel) {
		return "", false
	}
	return rel, true
}

// validDiffPath rejects an empty path and a path with whitespace or a quote.
func validDiffPath(path string) bool {
	if path == "" {
		return false
	}
	return !strings.ContainsAny(path, " \t\"")
}

// parseDiffHunkHeader reads the four position values of a hunk header.
func parseDiffHunkHeader(line string) (diffHunk, bool) {
	match := diffHunkRE.FindStringSubmatch(line)
	if match == nil {
		return diffHunk{}, false
	}
	baseStart, ok := parseNonNegativeInt(match[1])
	if !ok {
		return diffHunk{}, false
	}
	baseCount, ok := parseDiffCount(match[2])
	if !ok {
		return diffHunk{}, false
	}
	headStart, ok := parseNonNegativeInt(match[3])
	if !ok {
		return diffHunk{}, false
	}
	headCount, ok := parseDiffCount(match[4])
	if !ok {
		return diffHunk{}, false
	}
	return diffHunk{BaseStart: baseStart, BaseCount: baseCount, HeadStart: headStart, HeadCount: headCount}, true
}

// parseDiffCount returns the hunk count, which is one when the header omits it.
func parseDiffCount(text string) (int, bool) {
	if text == "" {
		return 1, true
	}
	return parseNonNegativeInt(text)
}
