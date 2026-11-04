//go:build test

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// Paths, from the repository root, of the documents the ADR guard reads.
const (
	adrConventionPath = "docs/dev/conventions/adr.md"
	adrDir            = "docs/dev/adr"
	adrIndexName      = "README.md"
)

// The headings of the convention's sections that hold the definitions the
// guard reads.
const (
	adrTemplateHeading = "## Template"
	adrStatusHeading   = "## Status"
	adrIndexHeading    = "## Index"
)

// The fixed shape of the definitions: the template has one "##" heading per
// required ADR section, the status table one row per status value, and the
// index one column each for the number, the title, and the status.
const (
	adrTemplateSections = 6
	adrStatusValues     = 2
	adrIndexColumns     = 3
)

// adrStatusRowName is the first cell of the status row in an ADR's first
// section. It is Japanese because the ADR is (guard-test literal exception).
const adrStatusRowName = "状態"

var (
	// adrNameRE is the file name of an ADR: a four-digit number and a kebab
	// name of lowercase ASCII letters and digits.
	adrNameRE = regexp.MustCompile(`^([0-9]{4})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
	// adrFirstLineRE is an ADR's first line: its number and its title.
	adrFirstLineRE = regexp.MustCompile(`^# ADR-([0-9]{4}): (.+)$`)
	// adrIndexLinkRE is the first cell of an index row: a link to the ADR
	// whose text is the ADR's name.
	adrIndexLinkRE = regexp.MustCompile(`^\[ADR-([0-9]{4})\]\(([^)\s]+)\)$`)
)

// adrIssueKind is a kind of mismatch the ADR guard reports.
type adrIssueKind int

const (
	adrBadName adrIssueKind = iota + 1
	adrBadFirstLine
	adrDuplicateNumber
	adrMissingNumber
	adrNotIndexed
	adrIndexRowWithoutFile
	adrIndexRowShape
	adrIndexLink
	adrTitleMismatch
	adrTitlePipe
	adrStatusMismatch
	adrHeadingMismatch
	adrStatusValue
	adrDefinitionUnreadable
	adrDefinitionCount
)

func (k adrIssueKind) String() string {
	switch k {
	case adrBadName:
		return "entry is not README.md or an NNNN-<kebab>.md file"
	case adrBadFirstLine:
		return "first line is not \"# ADR-NNNN: <title>\" with the file's number"
	case adrDuplicateNumber:
		return "number used more than once"
	case adrMissingNumber:
		return "number missing from the sequence starting at 0001"
	case adrNotIndexed:
		return "ADR has no index row"
	case adrIndexRowWithoutFile:
		return "index row names no ADR file"
	case adrIndexRowShape:
		return "index row has the wrong number of cells"
	case adrIndexLink:
		return "index row's first cell is not a link to the ADR's file"
	case adrTitleMismatch:
		return "index title differs from the ADR's title"
	case adrTitlePipe:
		return "ADR title contains |"
	case adrStatusMismatch:
		return "index status differs from the ADR's status"
	case adrHeadingMismatch:
		return "## headings differ from the template's"
	case adrStatusValue:
		return "first section has no single status row with a value the convention defines"
	case adrDefinitionUnreadable:
		return "definition cannot be read"
	case adrDefinitionCount:
		return "definition has the wrong number of entries"
	}
	return "unknown issue kind " + strconv.Itoa(int(k))
}

// adrIssue is one mismatch: the file it is in, its kind, and what was found.
type adrIssue struct {
	path   string
	kind   adrIssueKind
	detail string
}

// adrEntry is one entry directly under the ADR directory. content is read only
// for a file.
type adrEntry struct {
	name    string
	dir     bool
	content string
}

// adrInput is what the guard checks: the convention, the index, and the
// entries of the ADR directory, the index's own entry included.
type adrInput struct {
	convention string
	index      string
	entries    []adrEntry
}

// adrDefinitions are the values the convention defines for ADRs and the index.
type adrDefinitions struct {
	sections    []string
	statuses    []string
	indexHeader []string
}

// adrRecord is one well-named ADR file, with what its first line and its first
// section say.
type adrRecord struct {
	name   string
	number int
	title  string
	status string
}

// checkADRRecords reports every mismatch between the ADRs, the index, and the
// convention. The definitions are read from the convention, never assumed:
// when one cannot be read, that is reported and nothing is checked against
// it, so a broken convention cannot make the guard pass silently.
func checkADRRecords(in adrInput) []adrIssue {
	defs, issues := readADRDefinitions(in.convention)
	if len(issues) > 0 {
		return issues
	}
	var records []adrRecord
	for _, entry := range in.entries {
		if strings.HasPrefix(entry.name, ".") || (!entry.dir && entry.name == adrIndexName) {
			continue
		}
		rec, recIssues := checkADRFile(entry, defs)
		issues = append(issues, recIssues...)
		if rec != nil {
			records = append(records, *rec)
		}
	}
	issues = append(issues, checkADRNumbers(records)...)
	return append(issues, checkADRIndex(in.index, defs, records)...)
}

// readADRDefinitions reads the template's "##" headings, the status values,
// and the index header from their sections of the convention.
func readADRDefinitions(convention string) (adrDefinitions, []adrIssue) {
	var defs adrDefinitions
	var issues []adrIssue
	fail := func(kind adrIssueKind, format string, args ...any) {
		issues = append(issues, adrIssue{path: adrConventionPath, kind: kind, detail: fmt.Sprintf(format, args...)})
	}

	// A second section under the same heading would be ignored, since a
	// section is read from the first matching heading.
	headings := doctestutil.HeadingsOutsideBlocks(convention)
	for _, want := range []string{adrTemplateHeading, adrStatusHeading, adrIndexHeading} {
		if n := countFunc(headings, func(h string) bool { return strings.TrimSpace(h) == want }); n > 1 {
			fail(adrDefinitionUnreadable, "%q appears %d times", want, n)
		}
	}

	if block, ok := onlyBlock(convention, adrTemplateHeading); !ok {
		fail(adrDefinitionUnreadable, "%q needs exactly one code block", adrTemplateHeading)
	} else {
		for _, line := range block.Lines {
			if doctestutil.HeadingLevel(line) == 2 {
				defs.sections = append(defs.sections, strings.TrimSpace(line))
			}
		}
		if len(defs.sections) != adrTemplateSections {
			fail(adrDefinitionCount, "the template has %d ## headings, want %d", len(defs.sections), adrTemplateSections)
		}
	}

	if tables := tablesOutsideBlocks(doctestutil.SectionOutsideBlocks(convention, adrStatusHeading)); len(tables) != 1 {
		fail(adrDefinitionUnreadable, "%q needs exactly one table outside code blocks, found %d", adrStatusHeading, len(tables))
	} else {
		for _, row := range tables[0][1:] {
			if len(row) > 0 {
				defs.statuses = append(defs.statuses, doctestutil.StripBackticks(row[0]))
			}
		}
		if len(defs.statuses) != adrStatusValues {
			fail(adrDefinitionCount, "the status table has %d values, want %d", len(defs.statuses), adrStatusValues)
		}
	}

	// A section without exactly one code block yields an empty block, which
	// holds no table.
	indexBlock, _ := onlyBlock(convention, adrIndexHeading)
	if tables := doctestutil.MarkdownTables(strings.Join(indexBlock.Lines, "\n")); len(tables) != 1 {
		fail(adrDefinitionUnreadable, "%q needs exactly one code block holding exactly one table, found %d tables", adrIndexHeading, len(tables))
	} else {
		defs.indexHeader = tables[0][0]
		if len(defs.indexHeader) != adrIndexColumns {
			fail(adrDefinitionCount, "the index header has %d columns, want %d", len(defs.indexHeader), adrIndexColumns)
		}
	}
	return defs, issues
}

// countFunc returns how many elements of s satisfy f.
func countFunc(s []string, f func(string) bool) int {
	n := 0
	for _, v := range s {
		if f(v) {
			n++
		}
	}
	return n
}

// onlyBlock returns the one closed code block of the section under heading,
// and false when the section is missing or has zero or several code blocks.
func onlyBlock(doc, heading string) (doctestutil.FencedBlock, bool) {
	blocks := doctestutil.FencedBlocks(doctestutil.SectionOutsideBlocks(doc, heading))
	if len(blocks) != 1 || !blocks[0].Closed {
		return doctestutil.FencedBlock{}, false
	}
	return blocks[0], true
}

// tablesOutsideBlocks returns the tables of text that lie outside fenced code
// blocks. A code block line ends a table, as a blank line does.
func tablesOutsideBlocks(text string) [][][]string {
	var b strings.Builder
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(text))) {
		if !line.Block {
			b.WriteString(line.Text)
		}
		b.WriteString("\n")
	}
	return doctestutil.MarkdownTables(b.String())
}

// checkADRFile checks one entry other than the index and returns the record of
// a well-named ADR file, or nil when the entry is not one.
func checkADRFile(entry adrEntry, defs adrDefinitions) (*adrRecord, []adrIssue) {
	p := path.Join(adrDir, entry.name)
	match := adrNameRE.FindStringSubmatch(entry.name)
	if entry.dir || match == nil || match[1] == "0000" {
		return nil, []adrIssue{{path: p, kind: adrBadName, detail: entry.name}}
	}
	number, _ := strconv.Atoi(match[1])
	rec := &adrRecord{name: entry.name, number: number}
	var issues []adrIssue

	firstLine, _, _ := strings.Cut(entry.content, "\n")
	first := adrFirstLineRE.FindStringSubmatch(strings.TrimSpace(firstLine))
	switch {
	case first == nil || first[1] != match[1]:
		issues = append(issues, adrIssue{path: p, kind: adrBadFirstLine, detail: firstLine})
	default:
		rec.title = strings.TrimSpace(first[2])
		if strings.Contains(rec.title, "|") {
			issues = append(issues, adrIssue{path: p, kind: adrTitlePipe, detail: rec.title})
		}
	}

	var headings []string
	for _, h := range doctestutil.HeadingsOutsideBlocks(entry.content) {
		if doctestutil.HeadingLevel(h) == 2 {
			headings = append(headings, strings.TrimSpace(h))
		}
	}
	if !slices.Equal(headings, defs.sections) {
		issues = append(issues, adrIssue{path: p, kind: adrHeadingMismatch, detail: fmt.Sprintf("have %q, want %q", headings, defs.sections)})
	}

	status, ok := adrStatus(entry.content, headings)
	if !ok || !slices.Contains(defs.statuses, status) {
		issues = append(issues, adrIssue{path: p, kind: adrStatusValue, detail: fmt.Sprintf("status %q, allowed %q", status, defs.statuses)})
	} else {
		rec.status = status
	}
	return rec, issues
}

// adrStatus returns the value of the one status row of the one table in the
// ADR's first "##" section, and false when that section, table, or row is
// missing or not unique.
func adrStatus(doc string, headings []string) (string, bool) {
	if len(headings) == 0 {
		return "", false
	}
	tables := tablesOutsideBlocks(doctestutil.SectionOutsideBlocks(doc, headings[0]))
	if len(tables) != 1 {
		return "", false
	}
	var values []string
	for _, row := range tables[0][1:] {
		if len(row) >= 2 && doctestutil.StripBackticks(row[0]) == adrStatusRowName {
			values = append(values, doctestutil.StripBackticks(row[1]))
		}
	}
	if len(values) != 1 {
		return "", false
	}
	return values[0], true
}

// checkADRNumbers reports a number used by two ADR files and every number
// missing between 0001 and the highest number in use.
func checkADRNumbers(records []adrRecord) []adrIssue {
	var issues []adrIssue
	byNumber := make(map[int][]string)
	highest := 0
	for _, rec := range records {
		byNumber[rec.number] = append(byNumber[rec.number], rec.name)
		highest = max(highest, rec.number)
	}
	for n := 1; n <= highest; n++ {
		switch names := byNumber[n]; {
		case len(names) == 0:
			issues = append(issues, adrIssue{path: adrDir, kind: adrMissingNumber, detail: fmt.Sprintf("%04d", n)})
		case len(names) > 1:
			issues = append(issues, adrIssue{path: adrDir, kind: adrDuplicateNumber, detail: strings.Join(names, ", ")})
		}
	}
	return issues
}

// checkADRIndex checks the index table against the ADR files: one row per
// file, linked by its number and its file name, with the file's title and
// status.
func checkADRIndex(index string, defs adrDefinitions, records []adrRecord) []adrIssue {
	p := path.Join(adrDir, adrIndexName)
	var tables [][][]string
	for _, table := range tablesOutsideBlocks(index) {
		if slices.Equal(table[0], defs.indexHeader) {
			tables = append(tables, table)
		}
	}
	if len(tables) != 1 {
		return []adrIssue{{path: p, kind: adrDefinitionUnreadable, detail: fmt.Sprintf("want exactly one table with the header %q, found %d", defs.indexHeader, len(tables))}}
	}

	var issues []adrIssue
	add := func(kind adrIssueKind, format string, args ...any) {
		issues = append(issues, adrIssue{path: p, kind: kind, detail: fmt.Sprintf(format, args...)})
	}
	byNumber := make(map[int][]adrRecord)
	for _, rec := range records {
		byNumber[rec.number] = append(byNumber[rec.number], rec)
	}
	indexed := make(map[string]struct{})
	rowNumbers := make(map[int]struct{})
	for _, row := range tables[0][1:] {
		if len(row) != len(defs.indexHeader) {
			add(adrIndexRowShape, "%q", row)
			continue
		}
		link := adrIndexLinkRE.FindStringSubmatch(row[0])
		if link == nil {
			add(adrIndexLink, "%q", row[0])
			continue
		}
		number, _ := strconv.Atoi(link[1])
		if _, seen := rowNumbers[number]; seen {
			add(adrDuplicateNumber, "two rows for ADR-%s", link[1])
			continue
		}
		rowNumbers[number] = struct{}{}
		recs := byNumber[number]
		if len(recs) == 0 {
			add(adrIndexRowWithoutFile, "ADR-%s", link[1])
			continue
		}
		// A duplicate number is reported by checkADRNumbers; the row is
		// compared with the file its link names.
		rec := recs[0]
		for _, r := range recs {
			if r.name == link[2] {
				rec = r
			}
		}
		indexed[rec.name] = struct{}{}
		if link[2] != rec.name {
			add(adrIndexLink, "ADR-%s links to %q, want %q", link[1], link[2], rec.name)
		}
		if title := doctestutil.StripBackticks(row[1]); rec.title != "" && title != doctestutil.StripBackticks(rec.title) {
			add(adrTitleMismatch, "ADR-%s: index %q, ADR %q", link[1], title, rec.title)
		}
		if status := doctestutil.StripBackticks(row[2]); rec.status != "" && status != rec.status {
			add(adrStatusMismatch, "ADR-%s: index %q, ADR %q", link[1], status, rec.status)
		}
	}
	for _, rec := range records {
		if _, ok := indexed[rec.name]; !ok {
			issues = append(issues, adrIssue{path: path.Join(adrDir, rec.name), kind: adrNotIndexed, detail: rec.name})
		}
	}
	return issues
}

// TestADRRecords: the repository's ADRs and index agree with each other and
// with the definitions in the ADR convention. The index is where a reader
// learns which decisions are in effect, so a missing row, a stale status, or a
// duplicate number would mislead every later design.
func TestADRRecords(t *testing.T) {
	dir := filepath.Join(repositoryRoot, filepath.FromSlash(adrDir))
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", adrDir, err)
	}
	in := adrInput{
		convention: doctestutil.ReadDoc(t, filepath.Join(repositoryRoot, filepath.FromSlash(adrConventionPath))),
		index:      doctestutil.ReadDoc(t, filepath.Join(dir, adrIndexName)),
	}
	for _, e := range dirEntries {
		entry := adrEntry{name: e.Name(), dir: e.IsDir()}
		if !entry.dir && !strings.HasPrefix(entry.name, ".") {
			entry.content = doctestutil.ReadDoc(t, filepath.Join(dir, entry.name))
		}
		in.entries = append(in.entries, entry)
	}
	for _, issue := range checkADRRecords(in) {
		t.Errorf("%s: %s: %s", issue.path, issue.kind, issue.detail)
	}
}

// adrMD turns each "'" of s into a backtick, so a fixture with code spans and
// fences can be written as a raw string. The fixtures use no apostrophe.
func adrMD(s string) string {
	return strings.ReplaceAll(s, "'", "`")
}

// The sections of the test convention. The template, status values, and index
// header are ASCII on purpose: the checker reads them from the convention, so
// only the status row name is fixed in the test.
var (
	adrTestTemplateSection = adrMD(`## Template

'''markdown
# ADR-NNNN: <title>

## Status

| Item | Value |
|---|---|
| ROW | 'active' |

## Context

## Decision

## Alternatives

## Consequences

## History
'''
`)
	adrTestStatusSection = adrMD(`## Status

| Value | Name | Meaning |
|---|---|---|
| 'active' | active | in effect |
| 'obsolete' | obsolete | ended |
`)
	adrTestIndexSection = adrMD(`## Index

'''markdown
| No | Title | Status |
|---|---|---|
'''
`)
)

// adrTestConvention returns a convention made of the given sections.
func adrTestConvention(sections ...string) string {
	return "# Convention\n\n" + strings.Join(sections, "\n")
}

// adrTestDoc returns an ADR with the template's sections and the given number,
// title, and status.
func adrTestDoc(number, title, status string) string {
	return adrMD(fmt.Sprintf(`# ADR-%s: %s

## Status

| Item | Value |
|---|---|
| %s | '%s' |

## Context

## Decision

## Alternatives

## Consequences

## History
`, number, title, adrStatusRowName, status))
}

// adrTestIndex returns an index whose table has the given rows.
func adrTestIndex(rows ...string) string {
	return "# Index\n\nText.\n\n| No | Title | Status |\n|---|---|---|\n" + strings.Join(rows, "\n") + "\n"
}

// adrTestIndexRow returns an index row for the ADR file name with the title and
// status.
func adrTestIndexRow(name, title, status string) string {
	return adrMD(fmt.Sprintf("| [ADR-%s](%s) | %s | '%s' |", name[:4], name, title, status))
}

// adrValidInput returns a convention, an index, and two ADRs that agree.
func adrValidInput() adrInput {
	return adrInput{
		convention: adrTestConvention(adrTestTemplateSection, adrTestStatusSection, adrTestIndexSection),
		index: adrTestIndex(
			adrTestIndexRow("0001-first.md", "First", "active"),
			adrTestIndexRow("0002-second-one.md", "Second", "obsolete"),
		),
		entries: []adrEntry{
			{name: adrIndexName},
			{name: "0001-first.md", content: adrTestDoc("0001", "First", "active")},
			{name: "0002-second-one.md", content: adrTestDoc("0002", "Second", "obsolete")},
		},
	}
}

// TestCheckADRRecords: the checker reports each kind of mismatch, and nothing
// for inputs that agree. Each case breaks one thing, so the set of kinds it
// reports pins which check reacts to it.
func TestCheckADRRecords(t *testing.T) {
	valid := adrValidInput()
	withEntries := func(entries ...adrEntry) adrInput {
		in := valid
		in.entries = append(slices.Clone(valid.entries), entries...)
		return in
	}
	// withSecond replaces the content of the second ADR.
	withSecond := func(content string) adrInput {
		in := valid
		in.entries = slices.Clone(valid.entries)
		in.entries[2].content = content
		return in
	}
	withIndex := func(rows ...string) adrInput {
		in := valid
		in.index = adrTestIndex(rows...)
		return in
	}
	withConvention := func(sections ...string) adrInput {
		in := valid
		in.convention = adrTestConvention(sections...)
		return in
	}
	row1 := adrTestIndexRow("0001-first.md", "First", "active")
	row2 := adrTestIndexRow("0002-second-one.md", "Second", "obsolete")
	twoBlocks := func(section string) string {
		return section + adrMD("\n'''text\nmore\n'''\n")
	}

	for _, tc := range []struct {
		name string
		in   adrInput
		want []adrIssueKind
	}{
		{name: "valid", in: valid},
		{
			name: "empty index and no ADR",
			in:   adrInput{convention: valid.convention, index: adrTestIndex(), entries: []adrEntry{{name: adrIndexName}}},
		},
		{
			name: "empty index with an ADR",
			in: adrInput{convention: valid.convention, index: adrTestIndex(), entries: []adrEntry{
				{name: adrIndexName}, {name: "0001-first.md", content: adrTestDoc("0001", "First", "active")},
			}},
			want: []adrIssueKind{adrNotIndexed},
		},
		{
			name: "dot entries are not counted",
			in:   withEntries(adrEntry{name: ".DS_Store", content: "x"}, adrEntry{name: ".cache", dir: true}),
		},
		{
			name: "subdirectory",
			in:   withEntries(adrEntry{name: "0003-sub.md", dir: true}),
			want: []adrIssueKind{adrBadName},
		},
		{
			name: "other file",
			in:   withEntries(adrEntry{name: "notes.txt", content: "x"}),
			want: []adrIssueKind{adrBadName},
		},
		{
			name: "name with an uppercase letter",
			in:   withEntries(adrEntry{name: "0003-Third.md", content: adrTestDoc("0003", "Third", "active")}),
			want: []adrIssueKind{adrBadName},
		},
		{
			name: "number 0000",
			in:   withEntries(adrEntry{name: "0000-zero.md", content: adrTestDoc("0000", "Zero", "active")}),
			want: []adrIssueKind{adrBadName},
		},
		{
			name: "first line with another number",
			in:   withSecond(adrTestDoc("0003", "Second", "obsolete")),
			want: []adrIssueKind{adrBadFirstLine},
		},
		{
			name: "first line without a title",
			in:   withSecond(strings.Replace(adrTestDoc("0002", "Second", "obsolete"), ": Second", ": ", 1)),
			want: []adrIssueKind{adrBadFirstLine},
		},
		{
			// The duplicate comes before the file the index row links to, so
			// the row must be compared with the file its link names, not the
			// first file with the number.
			name: "duplicate file number",
			in: adrInput{convention: valid.convention, index: valid.index, entries: []adrEntry{
				valid.entries[0], valid.entries[1],
				{name: "0002-other.md", content: adrTestDoc("0002", "Other", "active")},
				valid.entries[2],
			}},
			want: []adrIssueKind{adrDuplicateNumber, adrNotIndexed},
		},
		{
			name: "missing number",
			in: adrInput{
				convention: valid.convention,
				index:      adrTestIndex(row1, adrTestIndexRow("0003-third.md", "Third", "active")),
				entries: []adrEntry{
					{name: adrIndexName},
					{name: "0001-first.md", content: adrTestDoc("0001", "First", "active")},
					{name: "0003-third.md", content: adrTestDoc("0003", "Third", "active")},
				},
			},
			want: []adrIssueKind{adrMissingNumber},
		},
		{
			name: "index row without a file",
			in:   withIndex(row1, row2, adrTestIndexRow("0003-third.md", "Third", "active")),
			want: []adrIssueKind{adrIndexRowWithoutFile},
		},
		{
			name: "two index rows for one number",
			in:   withIndex(row1, row2, row2),
			want: []adrIssueKind{adrDuplicateNumber},
		},
		{
			name: "index row with an extra cell",
			in:   withIndex(row1, row2+" x |"),
			want: []adrIssueKind{adrIndexRowShape, adrNotIndexed},
		},
		{
			name: "index row without a link",
			in:   withIndex(row1, adrMD("| ADR-0002 | Second | 'obsolete' |")),
			want: []adrIssueKind{adrIndexLink, adrNotIndexed},
		},
		{
			name: "index row linking to another file name",
			in:   withIndex(row1, adrTestIndexRow("0002-second.md", "Second", "obsolete")),
			want: []adrIssueKind{adrIndexLink},
		},
		{
			name: "index title differs",
			in:   withIndex(row1, adrTestIndexRow("0002-second-one.md", "Second title", "obsolete")),
			want: []adrIssueKind{adrTitleMismatch},
		},
		{
			name: "title with a pipe",
			in:   withSecond(adrTestDoc("0002", "Sec | ond", "obsolete")),
			want: []adrIssueKind{adrTitlePipe, adrTitleMismatch},
		},
		{
			name: "index status differs",
			in:   withIndex(row1, adrTestIndexRow("0002-second-one.md", "Second", "active")),
			want: []adrIssueKind{adrStatusMismatch},
		},
		{
			name: "missing heading",
			in:   withSecond(strings.Replace(adrTestDoc("0002", "Second", "obsolete"), "## History\n", "", 1)),
			want: []adrIssueKind{adrHeadingMismatch},
		},
		{
			name: "headings out of order",
			in: withSecond(strings.Replace(
				strings.Replace(adrTestDoc("0002", "Second", "obsolete"), "## Context", "## TMP", 1),
				"## Decision", "## Context", 1)),
			want: []adrIssueKind{adrHeadingMismatch},
		},
		{
			name: "status value the convention does not define",
			in:   withSecond(adrTestDoc("0002", "Second", "draft")),
			want: []adrIssueKind{adrStatusValue},
		},
		{
			name: "no status row",
			in:   withSecond(strings.Replace(adrTestDoc("0002", "Second", "obsolete"), "| "+adrStatusRowName+" |", "| Other |", 1)),
			want: []adrIssueKind{adrStatusValue},
		},
		{
			name: "two status rows",
			in:   withSecond(strings.Replace(adrTestDoc("0002", "Second", "obsolete"), "|---|---|\n", adrMD("|---|---|\n| "+adrStatusRowName+" | 'active' |\n"), 1)),
			want: []adrIssueKind{adrStatusValue},
		},
		{
			name: "no table in the first section",
			in:   withSecond(strings.Replace(adrTestDoc("0002", "Second", "obsolete"), "| Item | Value |\n|---|---|\n", "", 1)),
			want: []adrIssueKind{adrStatusValue},
		},
		{
			name: "index without the table",
			in:   adrInput{convention: valid.convention, index: "# Index\n", entries: valid.entries},
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "index with two tables",
			in:   adrInput{convention: valid.convention, index: valid.index + "\n" + adrTestIndex(), entries: valid.entries},
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "convention without the template section",
			in:   withConvention(adrTestStatusSection, adrTestIndexSection),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "status section twice",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection, adrTestIndexSection, adrTestStatusSection),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "template section without a code block",
			in:   withConvention("## Template\n\nText.\n", adrTestStatusSection, adrTestIndexSection),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "template section with two code blocks",
			in:   withConvention(twoBlocks(adrTestTemplateSection), adrTestStatusSection, adrTestIndexSection),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			// Placed last, so no later fence can close the template's block.
			name: "template code block not closed",
			in:   withConvention(adrTestStatusSection, adrTestIndexSection, strings.TrimSuffix(adrTestTemplateSection, "```\n")),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "template with five headings",
			in:   withConvention(strings.Replace(adrTestTemplateSection, "## History\n", "", 1), adrTestStatusSection, adrTestIndexSection),
			want: []adrIssueKind{adrDefinitionCount},
		},
		{
			name: "status section without a table",
			in:   withConvention(adrTestTemplateSection, "## Status\n\nText.\n", adrTestIndexSection),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "status section with two tables",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection+"\n| A |\n|---|\n| b |\n", adrTestIndexSection),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "status table with three values",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection+adrMD("| 'draft' | draft | not yet |\n"), adrTestIndexSection),
			want: []adrIssueKind{adrDefinitionCount},
		},
		{
			name: "index section without a code block",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection, "## Index\n\nText.\n"),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "index section with two code blocks",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection, twoBlocks(adrTestIndexSection)),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "index code block without a table",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection, adrMD("## Index\n\n'''text\nno table\n'''\n")),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "index code block with two tables",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection, adrMD("## Index\n\n'''markdown\n| No | Title | Status |\n|---|---|---|\n\n| A |\n|---|\n'''\n")),
			want: []adrIssueKind{adrDefinitionUnreadable},
		},
		{
			name: "index header with two columns",
			in:   withConvention(adrTestTemplateSection, adrTestStatusSection, adrMD("## Index\n\n'''markdown\n| No | Title |\n|---|---|\n'''\n")),
			want: []adrIssueKind{adrDefinitionCount},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues := checkADRRecords(tc.in)
			var got []adrIssueKind
			for _, issue := range issues {
				if !slices.Contains(got, issue.kind) {
					got = append(got, issue.kind)
				}
			}
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("kinds = %v, want %v; issues: %+v", got, want, issues)
			}
		})
	}
}
