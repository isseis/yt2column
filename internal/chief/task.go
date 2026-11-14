package chief

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// This file reads a task's documents by the grammar the repository defines
// for them: the status table ("Cell text" in
// docs/dev/conventions/requirements_process.md) and the PR markers
// (_context.md, "PR marker conventions", parts 2 to 4). The Japanese literals
// below must equal the headings, labels, and line prefixes of the Japanese
// plans. The marker reading is the one opus-review-decision.sh executes; the
// parity test runs both on the same plans.

// errTaskDocument reports a task document the tool cannot read by that
// grammar. The caller treats the task as unreadable and never guesses.
var errTaskDocument = errors.New("unreadable task document")

// originMain is the ref every decision reads the task documents from, spelled
// in full so a local branch named origin/main is never read instead.
const originMain = "refs/remotes/origin/main"

// taskRoot is the directory that holds the task directories.
const taskRoot = "docs/tasks"

// The task documents a reader looks for in a task directory.
const (
	requirementsFile = "01_requirements.md"
	architectureFile = "02_architecture.md"
	planFile         = "03_implementation_plan.md"
)

// DocStatus is the Status cell of a task document. The zero value is
// DocStatusUnreadable: a cell of another text, or a table without one row of
// it.
type DocStatus int

const (
	// DocStatusUnreadable is a Status that cannot be read.
	DocStatusUnreadable DocStatus = iota
	// DocDraft is `draft`.
	DocDraft
	// DocApproved is `approved`.
	DocApproved
	// DocWithdrawn is `withdrawn`.
	DocWithdrawn
)

// Process is the Process row of a requirements document. The zero value is
// ProcessUnreadable: a broken record.
type Process int

const (
	// ProcessUnreadable is a broken record: two rows, or another cell text.
	ProcessUnreadable Process = iota
	// ProcessUnrecorded is a requirements document without the row, which
	// takes the full process.
	ProcessUnrecorded
	// ProcessFull is `full`.
	ProcessFull
	// ProcessSimple is `simple`.
	ProcessSimple
)

// DocInfo is what the status table of one task document says.
type DocInfo struct {
	Status DocStatus
	// Process is read for the requirements document only.
	Process Process
}

// statusHeading is the heading the status table follows.
const statusHeading = "## Document Status"

// statusCells returns, for each item, the cell texts of the rows of the
// status table: the first table outside a code block after the Document
// Status heading. A row's cell text is the bytes between its leading
// "| <Item> | " and its trailing " |".
func statusCells(doc []byte) (map[string][]string, error) {
	lines := docLines(doc)
	start := -1
	for i, l := range lines {
		if !l.fenced && l.text == statusHeading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("%w: no %q heading", errTaskDocument, statusHeading)
	}
	cells := map[string][]string{}
	inTable := false
	for _, l := range lines[start:] {
		if l.fenced {
			if inTable {
				break
			}
			continue
		}
		if !strings.HasPrefix(l.text, "|") {
			if inTable {
				break
			}
			continue
		}
		inTable = true
		rest, ok := strings.CutPrefix(l.text, "| ")
		if !ok {
			continue
		}
		item, cell, ok := strings.Cut(rest, " | ")
		if !ok {
			continue
		}
		if cell, ok = strings.CutSuffix(cell, " |"); ok {
			cells[item] = append(cells[item], cell)
		}
	}
	if !inTable {
		return nil, fmt.Errorf("%w: no status table", errTaskDocument)
	}
	return cells, nil
}

// readDocInfo reads the Status cell, and with process the Process cell, of a
// task document.
func readDocInfo(doc []byte, process bool) (DocInfo, error) {
	cells, err := statusCells(doc)
	if err != nil {
		return DocInfo{}, err
	}
	info := DocInfo{}
	status := cells["Status"]
	if len(status) != 1 {
		return DocInfo{}, fmt.Errorf("%w: %d Status rows", errTaskDocument, len(status))
	}
	switch status[0] {
	case "`draft`":
		info.Status = DocDraft
	case "`approved`":
		info.Status = DocApproved
	case "`withdrawn`":
		info.Status = DocWithdrawn
	default:
		return DocInfo{}, fmt.Errorf("%w: Status %q", errTaskDocument, status[0])
	}
	if !process {
		return info, nil
	}
	switch rows := cells["Process"]; {
	case len(rows) == 0:
		info.Process = ProcessUnrecorded
	case len(rows) > 1:
		return DocInfo{}, fmt.Errorf("%w: %d Process rows", errTaskDocument, len(rows))
	case rows[0] == "`full`":
		info.Process = ProcessFull
	case rows[0] == "`simple`":
		info.Process = ProcessSimple
	default:
		return DocInfo{}, fmt.Errorf("%w: Process %q", errTaskDocument, rows[0])
	}
	return info, nil
}

// docLine is one line of a document, its trailing CR removed, with whether
// it is inside a fenced code block or is a fence marker itself.
type docLine struct {
	text   string
	fenced bool
}

// docLines splits doc into lines and marks the fenced ones by the fence rule
// of opus-review-decision.sh: a fence marker is a run of three or more ` or ~
// indented by at most three spaces; a fence closes only on a run of the same
// character at least as long, with nothing but blanks after it; a fence-like
// line inside a fence is content. unclosed is true when a fence never closes.
func docLines(doc []byte) []docLine {
	lines, _ := fencedLines(doc)
	return lines
}

func fencedLines(doc []byte) (lines []docLine, unclosed bool) {
	inFence := false
	var fenceChar byte
	fenceLen := 0
	for raw := range bytes.SplitSeq(doc, []byte("\n")) {
		text := strings.TrimSuffix(string(raw), "\r")
		marker, char, run, rest := fenceMarker(text)
		switch {
		case marker && !inFence:
			inFence, fenceChar, fenceLen = true, char, run
			lines = append(lines, docLine{text: text, fenced: true})
		case marker && char == fenceChar && run >= fenceLen && strings.Trim(rest, " \t") == "":
			inFence = false
			lines = append(lines, docLine{text: text, fenced: true})
		default:
			lines = append(lines, docLine{text: text, fenced: inFence})
		}
	}
	return lines, inFence
}

// fenceMarker reports whether text starts a fence run after at most three
// spaces, with the run's character and length and the text after the run.
func fenceMarker(text string) (ok bool, char byte, run int, rest string) {
	t := ""
	indent := len(text) - len(strings.TrimLeft(text, " "))
	if text == "" || (indent <= 3 && indent < len(text) && text[indent] != '\t') {
		t = text[indent:]
	}
	if !strings.HasPrefix(t, "```") && !strings.HasPrefix(t, "~~~") {
		return false, 0, 0, ""
	}
	char = t[0]
	for run < len(t) && t[run] == char {
		run++
	}
	return true, char, run, t[run:]
}

// Requirement is the reading of a plan PR's implementation model requirement.
// The zero value is RequirementUnreadable: a value that is neither a value nor
// an old value, or more than one place that could hold it.
type Requirement int

const (
	// RequirementUnreadable is a value that cannot be read.
	RequirementUnreadable Requirement = iota
	// RequirementAbsent is no value.
	RequirementAbsent
	// RequirementOpus is opus.
	RequirementOpus
	// RequirementSonnetReview is sonnet+opus-review.
	RequirementSonnetReview
	// RequirementSonnet is sonnet.
	RequirementSonnet
)

// requirementValues maps the three values.
var requirementValues = map[string]Requirement{
	"opus":               RequirementOpus,
	"sonnet+opus-review": RequirementSonnetReview,
	"sonnet":             RequirementSonnet,
}

// RequirementReading is a requirement with the text as written: for an old
// value, the old value; for an unreadable value, its text.
type RequirementReading struct {
	Value Requirement
	Text  string
}

// OldValues is the old-value table of _context.md, "PR marker conventions",
// part 4: an old value and the value it is read as.
type OldValues map[string]Requirement

// contextMarkerSection is the heading of the section the old-value table is
// read from.
const contextMarkerSection = "### PR marker conventions"

// oldValueRow is a two-cell row whose second cell is one of the three values,
// the form opus-review-decision.sh reads as an old-value row.
var oldValueRow = regexp.MustCompile("^[ \t]*\\| `([^`]+)` \\| `(opus|sonnet\\+opus-review|sonnet)` \\|[ \t]*$")

// anyHeading matches a Markdown heading of any level.
var anyHeading = regexp.MustCompile(`^#+[ \t]`)

// ParseOldValues reads the old-value table from _context.md as
// opus-review-decision.sh does: every matching row inside the "PR marker
// conventions" section, with an old value that has two rows dropped so it
// reads as unreadable.
func ParseOldValues(contextDoc []byte) OldValues {
	counts := map[string]int{}
	values := OldValues{}
	inSection := false
	for raw := range bytes.SplitSeq(contextDoc, []byte("\n")) {
		line := string(raw)
		if anyHeading.MatchString(line) {
			inSection = strings.TrimRight(line, " \t") == contextMarkerSection
			continue
		}
		if !inSection {
			continue
		}
		if m := oldValueRow.FindStringSubmatch(line); m != nil {
			if strings.ContainsAny(m[1], " \t") {
				// The script splits an old value on blanks, so it never
				// matches one that holds a blank; neither does the tool.
				continue
			}
			counts[m[1]]++
			values[m[1]] = requirementValues[m[2]]
		}
	}
	for old, n := range counts {
		if n != 1 {
			delete(values, old)
		}
	}
	return values
}

// The field labels of a PR marker and of the PR structure section of chapter
// 3, and that section's heading text.
const (
	requirementField = "**実装モデル要件**: "
	autoSendField    = "**自動送信**: "
	stepsField       = "**対象ステップ**: "
	prStructure      = "PR 構成"
)

var (
	// markerHeadingLoose matches any PR marker heading, the form
	// opus-review-decision.sh counts to tell a plan with markers.
	markerHeadingLoose = regexp.MustCompile(`^### PR-[0-9]+ 作成ポイント`)
	// markerHeading matches a PR marker heading with a target number the
	// script accepts, followed by ":" or nothing.
	markerHeading = regexp.MustCompile(`^### PR-([1-9][0-9]*) 作成ポイント(?::|[ \t]*$)`)
	// sectionNumber is the leading number of a heading text.
	sectionNumber = regexp.MustCompile(`^[0-9]+(?:[.][0-9]+)*[.]?[ \t]+`)
	// chapterThree matches the heading of chapter 3.
	chapterThree = regexp.MustCompile(`^## 3(?:[.]|[ \t])`)
)

// fieldPlace is what locating a field gives: the value, or why there is none.
type fieldPlace int

const (
	// placeMultiple is more than one place, an unclosed fence, a value line
	// inside a fence of the section, or (for a plan without markers) a marker
	// heading: a failure.
	placeMultiple fieldPlace = iota
	// placeNoSection is no section for the target.
	placeNoSection
	// placeAbsent is a section without the field.
	placeAbsent
	// placeValue is one value.
	placeValue
)

// singleTarget is the target of a plan without PR markers.
const singleTarget = "single"

// locateField finds the field line of target ("N" of a marker, or
// singleTarget) by the rules of opus-review-decision.sh's locate_value, which
// _context.md, "PR marker conventions", parts 2 and 3, define.
func locateField(doc []byte, target, field string) (fieldPlace, string) {
	lines, unclosed := fencedLines(doc)
	markers, sections, values, fencedValues := 0, 0, 0, 0
	scan := sectionScan{target: target}
	value := ""
	for _, l := range lines {
		if l.fenced {
			if scan.inSection && strings.HasPrefix(l.text, field) {
				fencedValues++
			}
			continue
		}
		if target == singleTarget && markerHeadingLoose.MatchString(l.text) {
			markers++
		}
		if anyHeading.MatchString(l.text) {
			if scan.heading(l.text) {
				sections++
			}
			continue
		}
		if scan.inSection && strings.HasPrefix(l.text, field) {
			values++
			value = strings.Trim(l.text[len(field):], " \t\r")
		}
	}
	switch {
	case unclosed || markers > 0 || fencedValues > 0:
		return placeMultiple, ""
	case sections == 0:
		return placeNoSection, ""
	case sections > 1 || values > 1:
		return placeMultiple, ""
	case values == 0:
		return placeAbsent, ""
	default:
		return placeValue, value
	}
}

// chapterLevel is the deepest heading level that starts a chapter.
const chapterLevel = 2

// sectionScan tracks, heading by heading, whether the lines read are inside
// the section that holds target's fields.
type sectionScan struct {
	target     string
	inSection  bool
	inChapter3 bool
}

// heading moves the scan past a heading line and reports whether the
// heading starts a section of target. For a plan without markers, a chapter
// heading only sets whether chapter 3 begins; a deeper heading starts the
// section when its text without its number is prStructure inside chapter 3.
func (s *sectionScan) heading(text string) bool {
	if s.target != singleTarget {
		m := markerHeading.FindStringSubmatch(text)
		s.inSection = m != nil && m[1] == s.target
		return s.inSection
	}
	if level := len(text) - len(strings.TrimLeft(text, "#")); level <= chapterLevel {
		s.inChapter3 = chapterThree.MatchString(text)
		s.inSection = false
		return false
	}
	name := strings.TrimLeft(strings.TrimLeft(text, "#"), " \t")
	name = sectionNumber.ReplaceAllString(name, "")
	s.inSection = s.inChapter3 && strings.Trim(name, " \t\r") == prStructure
	return s.inSection
}

// readRequirement reads the implementation model requirement of target.
func readRequirement(doc []byte, target string, old OldValues) RequirementReading {
	place, text := locateField(doc, target, requirementField)
	switch place {
	case placeNoSection, placeAbsent:
		return RequirementReading{Value: RequirementAbsent}
	case placeValue:
		if v, ok := requirementValues[text]; ok {
			return RequirementReading{Value: v, Text: text}
		}
		if v, ok := old[text]; ok {
			return RequirementReading{Value: v, Text: text}
		}
		return RequirementReading{Value: RequirementUnreadable, Text: text}
	default:
		return RequirementReading{Value: RequirementUnreadable}
	}
}

// AutoSend is the reading of a plan PR's auto-send field. The zero value is
// AutoSendUnreadable: more than one place, or a value other than the two.
type AutoSend int

const (
	// AutoSendUnreadable is a field that cannot be read; nothing is sent
	// automatically.
	AutoSendUnreadable AutoSend = iota
	// AutoSendAuto lets chief send /runplan after the merge, also when the
	// field is absent.
	AutoSendAuto
	// AutoSendHold keeps chief from sending /runplan automatically.
	AutoSendHold
)

// readAutoSend reads the auto-send field of target. A missing field is auto,
// so plans written before the field existed keep working; there is no old
// value.
func readAutoSend(doc []byte, target string) AutoSend {
	place, text := locateField(doc, target, autoSendField)
	switch place {
	case placeNoSection, placeAbsent:
		return AutoSendAuto
	case placeValue:
		switch text {
		case "auto":
			return AutoSendAuto
		case "hold":
			return AutoSendHold
		}
	}
	return AutoSendUnreadable
}

// PlanPR is one PR of a plan: a PR marker, or the plan itself when it has no
// markers.
type PlanPR struct {
	// Target is N of the marker, or "single".
	Target      string
	Requirement RequirementReading
	AutoSend    AutoSend
	// Created and Merged are the PR's created and merged checkboxes
	// (createdBox, mergedBox); for a plan without markers, its single-PR
	// record.
	Created bool
	Merged  bool
	// CreatedURL is the text after the label and ": " of a done created
	// checkbox, the PR's URL; empty when the line names none.
	CreatedURL string
	// Steps are the marker's target steps; nil for a plan without markers.
	Steps []string
	// stepsErr is why the marker's target steps could not be read; only the
	// check that they are done needs them, so it alone fails.
	stepsErr error
}

// Plan is what an implementation plan says about its PRs.
type Plan struct {
	Status DocStatus
	PRs    []PlanPR
	// steps maps each step's ID to whether its checkbox is done or skipped.
	steps map[string]bool
}

var (
	// createdBox and mergedBox are a PR's two progress checkboxes in the
	// forms the plans on main use: not started, done, or skipped, with any
	// list indent, and the label alone or followed by ": " (the PR's URL) or
	// a note in full-width parentheses.
	createdBox = regexp.MustCompile(`^-[ \t]+\[([ x-])\][ \t]+PR を作成した(?:$|: (.*)|（)`)
	mergedBox  = regexp.MustCompile(`^-[ \t]+\[([ x-])\][ \t]+PR がマージされた(?:$|（)`)
	// progressLike is any checkbox line with a progress label; one that is
	// not in a form above makes the plan unreadable rather than read as no
	// checkbox.
	progressLike = regexp.MustCompile(`^[ \t>]*(?:[-*+]|[0-9]+[.)])[ \t\x{3000}]*\[[^\]]*\][ \t\x{3000}]*PR[ \t\x{3000}]*(?:を作成した|がマージされた)`)
	// stepBox is a step's checkbox: done, skipped, or not started.
	stepBox = regexp.MustCompile(`^-[ \t]+\[([ x-])\][ \t]+\*\*ステップ ([0-9]+-[0-9]+)\*\*`)
	// stepID is one step of a marker's target step list.
	stepID = regexp.MustCompile(`^[0-9]+-[0-9]+$`)
)

// ParsePlan reads a plan: its Status, its PRs in order with their values and
// progress, and its step checkboxes. Contract: a plan whose PRs cannot be told
// apart (a repeated marker number, a marker heading the script does not
// accept, a progress checkbox missing or repeated) is rejected, never read
// in part.
func ParsePlan(doc []byte, old OldValues) (Plan, error) {
	info, err := readDocInfo(doc, false)
	if err != nil {
		return Plan{}, err
	}
	lines, unclosed := fencedLines(doc)
	if unclosed {
		return Plan{}, fmt.Errorf("%w: unclosed code fence", errTaskDocument)
	}
	plan := Plan{Status: info.Status, steps: map[string]bool{}}
	var sections [][]docLine
	var current []docLine
	var targets []string
	inMarker := false
	for _, l := range lines {
		if l.fenced {
			continue
		}
		if m := stepBox.FindStringSubmatch(l.text); m != nil {
			if _, dup := plan.steps[m[2]]; dup {
				return Plan{}, fmt.Errorf("%w: step %s twice", errTaskDocument, m[2])
			}
			plan.steps[m[2]] = m[1] != " "
		}
		if anyHeading.MatchString(l.text) {
			if inMarker {
				sections = append(sections, current)
			}
			inMarker = false
			if markerHeadingLoose.MatchString(l.text) {
				m := markerHeading.FindStringSubmatch(l.text)
				if m == nil {
					return Plan{}, fmt.Errorf("%w: marker heading %q", errTaskDocument, l.text)
				}
				if slices.Contains(targets, m[1]) {
					return Plan{}, fmt.Errorf("%w: PR-%s twice", errTaskDocument, m[1])
				}
				targets = append(targets, m[1])
				inMarker, current = true, nil
			}
			continue
		}
		if inMarker {
			current = append(current, l)
		}
	}
	if inMarker {
		sections = append(sections, current)
	}
	if len(targets) == 0 {
		pr, err := singlePR(doc, lines, old)
		if err != nil {
			return Plan{}, err
		}
		plan.PRs = []PlanPR{pr}
		return plan, nil
	}
	for i, target := range targets {
		pr := PlanPR{
			Target:      target,
			Requirement: readRequirement(doc, target, old),
			AutoSend:    readAutoSend(doc, target),
		}
		if pr.Created, pr.Merged, pr.CreatedURL, err = progress(sections[i], true); err != nil {
			return Plan{}, fmt.Errorf("PR-%s: %w", target, err)
		}
		pr.Steps, pr.stepsErr = markerSteps(sections[i])
		plan.PRs = append(plan.PRs, pr)
	}
	return plan, nil
}

// singlePR reads the PR of a plan without markers: its values from the PR
// structure section, and its progress from the single-PR record, which is absent
// until the PR is created.
func singlePR(doc []byte, lines []docLine, old OldValues) (PlanPR, error) {
	pr := PlanPR{
		Target:      singleTarget,
		Requirement: readRequirement(doc, singleTarget, old),
		AutoSend:    readAutoSend(doc, singleTarget),
	}
	var err error
	pr.Created, pr.Merged, pr.CreatedURL, err = progress(lines, false)
	return pr, err
}

// progress reads a PR's two progress checkboxes from lines. In a marker both
// must appear once; a single-PR record may be absent, before the PR exists.
// Created and merged are true for a done or a skipped box.
func progress(lines []docLine, marker bool) (created, merged bool, url string, err error) {
	var createdRows, mergedRows, urls []string
	for _, l := range lines {
		if l.fenced {
			continue
		}
		if m := createdBox.FindStringSubmatch(l.text); m != nil {
			createdRows = append(createdRows, m[1])
			urls = append(urls, strings.TrimSpace(m[2]))
		} else if m := mergedBox.FindStringSubmatch(l.text); m != nil {
			mergedRows = append(mergedRows, m[1])
		} else if progressLike.MatchString(l.text) {
			return false, false, "", fmt.Errorf("%w: progress checkbox %q", errTaskDocument, l.text)
		}
	}
	if !marker && len(createdRows) == 0 && len(mergedRows) == 0 {
		return false, false, "", nil
	}
	if len(createdRows) != 1 || len(mergedRows) != 1 {
		return false, false, "", fmt.Errorf("%w: %d created and %d merged checkboxes", errTaskDocument, len(createdRows), len(mergedRows))
	}
	// A skipped box ("-") is settled like a done one: the PR is not to be
	// created, so it is never the next PR.
	if createdRows[0] == "x" {
		url = urls[0]
	}
	return createdRows[0] != " ", mergedRows[0] != " ", url, nil
}

// markerSteps reads a marker's target steps: one line, the step IDs
// separated by " / ".
func markerSteps(lines []docLine) ([]string, error) {
	var steps []string
	found := 0
	for _, l := range lines {
		rest, ok := strings.CutPrefix(l.text, stepsField)
		if !ok {
			continue
		}
		found++
		for s := range strings.SplitSeq(strings.TrimSpace(rest), " / ") {
			if !stepID.MatchString(s) {
				return nil, fmt.Errorf("%w: target step %q", errTaskDocument, s)
			}
			steps = append(steps, s)
		}
	}
	if found != 1 {
		return nil, fmt.Errorf("%w: %d target step lines", errTaskDocument, found)
	}
	return steps, nil
}

// CreatedAs returns the PR whose created checkbox is done with url, the
// whole URL compared; ok is false when no PR of the plan names it.
func (p Plan) CreatedAs(url string) (PlanPR, bool) {
	for _, pr := range p.PRs {
		if url != "" && pr.CreatedURL == url {
			return pr, true
		}
	}
	return PlanPR{}, false
}

// NextPR returns the next plan PR to create, the definition /runpr uses: the
// first marker whose created checkbox is not checked, or, for a plan without
// markers, the plan itself until its single-PR record exists.
func (p Plan) NextPR() (PlanPR, bool) {
	for _, pr := range p.PRs {
		if !pr.Created {
			return pr, true
		}
	}
	return PlanPR{}, false
}

// StepsLeft reports whether some step of the plan is not started. A plan
// with no step checkbox has none left.
func (p Plan) StepsLeft() bool {
	for _, done := range p.steps {
		if !done {
			return true
		}
	}
	return false
}

// Done reports whether the task is complete: the plan is approved and every
// PR, or the single-PR record, is merged.
func (p Plan) Done() bool {
	if p.Status != DocApproved || len(p.PRs) == 0 {
		return false
	}
	for _, pr := range p.PRs {
		if !pr.Created || !pr.Merged {
			return false
		}
	}
	return true
}

// StepsDone reports whether every step of the PR target is done or skipped:
// for a marker, its target steps; for a plan without markers, every step of
// the plan. A step without a checkbox is an error.
func (p Plan) StepsDone(target string) (bool, error) {
	i := slices.IndexFunc(p.PRs, func(pr PlanPR) bool { return pr.Target == target })
	if i < 0 {
		return false, fmt.Errorf("%w: no PR %s", errTaskDocument, target)
	}
	if err := p.PRs[i].stepsErr; err != nil {
		return false, fmt.Errorf("PR-%s: %w", target, err)
	}
	steps := p.PRs[i].Steps
	if target == singleTarget {
		if len(p.steps) == 0 {
			return false, fmt.Errorf("%w: no step checkboxes", errTaskDocument)
		}
		steps = slices.Collect(maps.Keys(p.steps))
	}
	for _, s := range steps {
		done, ok := p.steps[s]
		if !ok {
			return false, fmt.Errorf("%w: step %s has no checkbox", errTaskDocument, s)
		}
		if !done {
			return false, nil
		}
	}
	return true, nil
}

// featureBranch is the feature branch name of _context.md: a GitHub user
// name, the task directory name with "_" replaced by "-", and a two-digit
// sequence number. A branch follows the convention only when its slug is the
// directory's name, so Read checks the directory it finds against it.
var featureBranch = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?/(([0-9]{4})(?:-[a-z0-9]+)+)-[0-9]{2}$`)

// taskDirName is the form of a task directory name.
var taskDirName = regexp.MustCompile(`^[0-9]{4}_[a-z0-9]+(?:_[a-z0-9]+)*$`)

// branchTask returns the task of a feature branch name and the slug the
// directory name must give. ok is false for any other name.
func branchTask(branch string) (task paneinput.TaskID, slug string, ok bool) {
	m := featureBranch.FindStringSubmatch(branch)
	if m == nil {
		return paneinput.TaskID{}, "", false
	}
	task, err := paneinput.ParseTaskID(m[2])
	if err != nil {
		return paneinput.TaskID{}, "", false
	}
	return task, m[1], true
}

// TaskDocs is what origin/main holds for a task.
type TaskDocs struct {
	Task paneinput.TaskID
	// Dir is the task directory, such as docs/tasks/0088_chief_of_staff, or
	// "" when origin/main has none for the task yet.
	Dir          string
	Requirements *DocInfo
	Architecture *DocInfo
	Plan         *Plan
}

// Tasks reads task documents with git in a worktree.
type Tasks struct {
	cmds Commands
}

// NewTasks returns the task reader that runs git through cmds.
func NewTasks(cmds Commands) Tasks { return Tasks{cmds: cmds} }

// errNotTaskBranch reports a branch name that is not a feature branch; such a
// workspace has no task.
var errNotTaskBranch = errors.New("not a feature branch")

// Read reads, from origin/main in worktree, the documents of the task the
// feature branch names. Contract: the task comes from the branch name and the
// one directory of its number on origin/main, never from a screen; a
// directory whose name does not give the branch's slug, or two directories of
// one number, make the task unreadable.
func (t Tasks) Read(ctx context.Context, worktree, branch string) (TaskDocs, error) {
	task, slug, ok := branchTask(branch)
	if !ok {
		return TaskDocs{}, errNotTaskBranch
	}
	names, err := t.listTree(ctx, worktree, taskRoot+"/")
	if err != nil {
		return TaskDocs{}, err
	}
	var dirs []string
	for _, name := range names {
		base := strings.TrimPrefix(name, taskRoot+"/")
		if strings.HasPrefix(base, task.String()+"_") {
			dirs = append(dirs, base)
		}
	}
	docs := TaskDocs{Task: task}
	switch {
	case len(dirs) == 0:
		return docs, nil
	case len(dirs) > 1:
		return TaskDocs{}, fmt.Errorf("%w: %d directories for task %s", errTaskDocument, len(dirs), task)
	case !taskDirName.MatchString(dirs[0]) || strings.ReplaceAll(dirs[0], "_", "-") != slug:
		return TaskDocs{}, fmt.Errorf("%w: directory %q does not match branch %q", errTaskDocument, dirs[0], branch)
	}
	docs.Dir = taskRoot + "/" + dirs[0]
	files, err := t.listTree(ctx, worktree, docs.Dir+"/")
	if err != nil {
		return TaskDocs{}, err
	}
	for _, f := range []struct {
		name string
		read func([]byte) error
	}{
		{requirementsFile, func(b []byte) error {
			info, err := readDocInfo(b, true)
			docs.Requirements = &info
			return err
		}},
		{architectureFile, func(b []byte) error {
			info, err := readDocInfo(b, false)
			docs.Architecture = &info
			return err
		}},
		{planFile, func(b []byte) error {
			contextDoc, err := t.show(ctx, worktree, originMain, contextPath)
			if err != nil {
				return err
			}
			plan, err := ParsePlan(b, ParseOldValues(contextDoc))
			docs.Plan = &plan
			return err
		}},
	} {
		path := docs.Dir + "/" + f.name
		if !slices.Contains(files, path) {
			continue
		}
		body, err := t.show(ctx, worktree, originMain, path)
		if err != nil {
			return TaskDocs{}, err
		}
		if err := f.read(body); err != nil {
			return TaskDocs{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	return docs, nil
}

// BranchPlan reads the plan at HEAD of worktree, for the check that the
// steps of the next PR are done. old is the old-value table of origin/main.
func (t Tasks) BranchPlan(ctx context.Context, worktree string, docs TaskDocs, old OldValues) (Plan, error) {
	if docs.Dir == "" {
		return Plan{}, fmt.Errorf("%w: no task directory", errTaskDocument)
	}
	body, err := t.show(ctx, worktree, "HEAD", docs.Dir+"/"+planFile)
	if err != nil {
		return Plan{}, err
	}
	return ParsePlan(body, old)
}

// listTree returns the paths git lists at origin/main under prefix.
func (t Tasks) listTree(ctx context.Context, worktree, prefix string) ([]string, error) {
	out, err := t.cmds.git(ctx, worktree, "ls-tree", "-z", "--name-only", originMain, "--", prefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for name := range strings.SplitSeq(string(out), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// onHead reports whether path is a file at HEAD of worktree.
func (t Tasks) onHead(ctx context.Context, worktree, path string) (bool, error) {
	out, err := t.cmds.git(ctx, worktree, "ls-tree", "-z", "--name-only", "HEAD", "--", path)
	if err != nil {
		return false, err
	}
	return strings.TrimSuffix(string(out), "\x00") == path, nil
}

// show returns the blob at path in rev.
func (t Tasks) show(ctx context.Context, worktree, rev, path string) ([]byte, error) {
	return t.cmds.git(ctx, worktree, "show", rev+":"+path)
}
