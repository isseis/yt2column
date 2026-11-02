package coveragecompare

import (
	"regexp"
	"sort"
	"strings"
)

// profileLineRE matches one coverage profile line:
//
//	<module path>/<file>:<start line>.<start column>,<end line>.<end column> <statements> <count>
var profileLineRE = regexp.MustCompile(
	`^(.+):([0-9]+)\.([0-9]+),([0-9]+)\.([0-9]+) ([0-9]+) ([0-9]+)$`,
)

// profileBlock is one block of a Go coverage profile, with its file as a path
// relative to the repository root.
type profileBlock struct {
	File       string
	StartLine  int
	StartCol   int
	EndLine    int
	EndCol     int
	Statements int
	Count      int
}

// goProfile is the coverage a Go profile reported, with repeated blocks of the
// same position merged.
type goProfile struct {
	Blocks []profileBlock
}

// profilePosition is the position that identifies one block across test
// binaries.
type profilePosition struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

// parseGoProfile reads a Go coverage profile. modulePath is the module line of
// go.mod; every block's file must be inside it. The profile must start with the
// single "mode: set" line, and every later line must be a well-formed block.
// A line outside that form stops as unreadable rather than being skipped.
func parseGoProfile(data []byte, modulePath string, point Point) (*goProfile, error) {
	const modeLine = "mode: set"

	lines := splitLines(string(data))
	if len(lines) == 0 {
		return nil, stopError(point, InputGoProfile, 0, "the profile is empty", "")
	}
	if lines[0] != modeLine {
		return nil, stopError(point, InputGoProfile, 1, `line 1 is not "mode: set"`, lines[0])
	}

	blocks, err := parseProfileBlocks(lines[1:], modulePath, point)
	if err != nil {
		return nil, err
	}
	return &goProfile{Blocks: blocks}, nil
}

// profileLineOffset is the line number offset of the first block: line 1 is
// the mode line.
const profileLineOffset = 2

// parseProfileBlocks reads every block line, merging a block that several test
// binaries reported and sorting the result.
func parseProfileBlocks(lines []string, modulePath string, point Point) ([]profileBlock, error) {
	blocks := make([]profileBlock, 0, len(lines))
	seen := make(map[profilePosition]int, len(lines))
	for i, line := range lines {
		lineNo := i + profileLineOffset
		if strings.HasPrefix(line, "mode:") {
			return nil, stopError(point, InputGoProfile, lineNo, "a mode line appears after line 1", line)
		}
		block, err := parseProfileBlock(line, modulePath, point, lineNo)
		if err != nil {
			return nil, err
		}
		position := profilePosition{
			File:      block.File,
			StartLine: block.StartLine,
			StartCol:  block.StartCol,
			EndLine:   block.EndLine,
			EndCol:    block.EndCol,
		}
		if index, ok := seen[position]; ok {
			if blocks[index].Statements != block.Statements {
				return nil, stopError(point, InputGoProfile, lineNo, "the same block position is reported with a different statement count", line)
			}
			if block.Count > blocks[index].Count {
				blocks[index].Count = block.Count
			}
			continue
		}
		seen[position] = len(blocks)
		blocks = append(blocks, block)
	}

	sort.Slice(blocks, func(i, j int) bool {
		a, b := blocks[i], blocks[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		if a.StartCol != b.StartCol {
			return a.StartCol < b.StartCol
		}
		if a.EndLine != b.EndLine {
			return a.EndLine < b.EndLine
		}
		return a.EndCol < b.EndCol
	})
	return blocks, nil
}

// parseProfileBlock reads one block line.
func parseProfileBlock(line, modulePath string, point Point, lineNo int) (profileBlock, error) {
	match := profileLineRE.FindStringSubmatch(line)
	if match == nil {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the line is not a coverage block", line)
	}
	rel, ok := profileRelPath(match[1], modulePath)
	if !ok {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the block's file is outside the module or is not a clean relative path", line)
	}
	startLine, ok := parseNonNegativeInt(match[2])
	if !ok {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the start line is not a non-negative integer that fits int", line)
	}
	startCol, ok := parseNonNegativeInt(match[3])
	if !ok {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the start column is not a non-negative integer that fits int", line)
	}
	endLine, ok := parseNonNegativeInt(match[4])
	if !ok {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the end line is not a non-negative integer that fits int", line)
	}
	endCol, ok := parseNonNegativeInt(match[5])
	if !ok {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the end column is not a non-negative integer that fits int", line)
	}
	statements, ok := parseNonNegativeInt(match[6])
	if !ok {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the statement count is not a non-negative integer that fits int", line)
	}
	count, ok := parseNonNegativeInt(match[7])
	if !ok {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the run count is not a non-negative integer that fits int", line)
	}
	if startLine > endLine || (startLine == endLine && startCol > endCol) {
		return profileBlock{}, stopError(point, InputGoProfile, lineNo, "the block starts after it ends", line)
	}
	return profileBlock{
		File:       rel,
		StartLine:  startLine,
		StartCol:   startCol,
		EndLine:    endLine,
		EndCol:     endCol,
		Statements: statements,
		Count:      count,
	}, nil
}

// profileRelPath turns a profile's module-qualified file path into a path
// relative to the repository root. A path outside the module, an absolute
// path, and a path with an empty, "." or ".." element are rejected.
func profileRelPath(file, modulePath string) (string, bool) {
	prefix := modulePath + "/"
	if modulePath == "" || !strings.HasPrefix(file, prefix) {
		return "", false
	}
	rel := file[len(prefix):]
	if rel == "" || strings.HasPrefix(rel, "/") {
		return "", false
	}
	for part := range strings.SplitSeq(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	return rel, true
}
