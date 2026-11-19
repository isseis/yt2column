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
	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/tasknaming"
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
	// It is meaningless when stepsErr is set.
	steps map[string]bool
	// stepsErr is why the plan's steps could not be read (a *stepError). It
	// is not a ParsePlan error: only the two judgments that use the steps
	// (StepsLeft and StepsDone) fail on it, so PR progress and completion
	// are still read for a plan whose steps are unreadable.
	stepsErr error
}

var (
	// createdBox and mergedBox are a PR's two progress checkboxes in the
	// forms the plans on main use: not started, done, or skipped, with any
	// list indent, and the label alone or followed by ": " (the PR's URL) or
	// a note in full-width parentheses.
	createdBox = regexp.MustCompile(`^-[ \t]+\[([ x-])\][ \t]+PR を作成した(?:$|: |（)`)
	mergedBox  = regexp.MustCompile(`^-[ \t]+\[([ x-])\][ \t]+PR がマージされた(?:$|（)`)
	// progressLike is any checkbox line with a progress label; one that is
	// not in a form above makes the plan unreadable rather than read as no
	// checkbox.
	progressLike = regexp.MustCompile(`^[ \t>]*(?:[-*+]|[0-9]+[.)])[ \t\x{3000}]*\[[^\]]*\][ \t\x{3000}]*PR[ \t\x{3000}]*(?:を作成した|がマージされた)`)
	// stepBox is a form-A step: a checkbox line, done, skipped, or not
	// started, read anywhere in the document.
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
	plan := Plan{Status: info.Status}
	plan.steps, plan.stepsErr = readSteps(lines)
	var sections [][]docLine
	var current []docLine
	var targets []string
	inMarker := false
	for _, l := range lines {
		if l.fenced {
			continue
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
		if pr.Created, pr.Merged, err = progress(sections[i], true); err != nil {
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
	pr.Created, pr.Merged, err = progress(lines, false)
	return pr, err
}

// progress reads a PR's two progress checkboxes from lines. In a marker both
// must appear once; a single-PR record may be absent, before the PR exists.
// Created and merged are true for a done or a skipped box.
func progress(lines []docLine, marker bool) (created, merged bool, err error) {
	var createdRows, mergedRows []string
	for _, l := range lines {
		if l.fenced {
			continue
		}
		if m := createdBox.FindStringSubmatch(l.text); m != nil {
			createdRows = append(createdRows, m[1])
		} else if m := mergedBox.FindStringSubmatch(l.text); m != nil {
			mergedRows = append(mergedRows, m[1])
		} else if progressLike.MatchString(l.text) {
			return false, false, fmt.Errorf("%w: progress checkbox %q", errTaskDocument, l.text)
		}
	}
	if !marker && len(createdRows) == 0 && len(mergedRows) == 0 {
		return false, false, nil
	}
	if len(createdRows) != 1 || len(mergedRows) != 1 {
		return false, false, fmt.Errorf("%w: %d created and %d merged checkboxes", errTaskDocument, len(createdRows), len(mergedRows))
	}
	// A skipped box ("-") is settled like a done one: the PR is not to be
	// created, so it is never the next PR.
	return createdRows[0] != " ", mergedRows[0] != " ", nil
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

// StepsLeft reports whether some step of the plan is not started. It returns
// the plan's step-reading error when the steps cannot be read, and errNoSteps
// when the plan has no step of any form.
func (p Plan) StepsLeft() (bool, error) {
	if p.stepsErr != nil {
		return false, p.stepsErr
	}
	if len(p.steps) == 0 {
		return false, errNoSteps
	}
	for _, done := range p.steps {
		if !done {
			return true, nil
		}
	}
	return false, nil
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
// the plan. A step without a checkbox is an error, and so is a plan whose
// steps cannot be read.
func (p Plan) StepsDone(target string) (bool, error) {
	if p.stepsErr != nil {
		return false, p.stepsErr
	}
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

// branchTask returns the task of a feature branch name and the slug the
// directory name must give. A branch follows the convention only when its
// slug is the directory's name, so Read checks the directory it finds against
// it. ok is false for any other name.
func branchTask(branch string) (task paneinput.TaskID, slug string, ok bool) {
	number, slug, ok := tasknaming.ParseBranch(branch)
	if !ok {
		return paneinput.TaskID{}, "", false
	}
	task, err := paneinput.ParseTaskID(number.String())
	if err != nil {
		return paneinput.TaskID{}, "", false
	}
	return task, slug, true
}

// isTaskDir reports whether name is a task directory name.
func isTaskDir(name string) bool {
	_, ok := tasknaming.ParseTaskDir(name)
	return ok
}

// taskDirMatches reports whether dir is a task directory whose name gives the
// slug of the branch: the directory form holds, and "_" replaced by "-" is the
// slug.
func taskDirMatches(dir, slug string) bool {
	return isTaskDir(dir) && strings.ReplaceAll(dir, "_", "-") == slug
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
// directory that is not a task directory name, one whose "_"-to-"-" form does
// not give the branch's slug, or two directories of one number, make the task
// unreadable.
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
	case !taskDirMatches(dirs[0], slug):
		return TaskDocs{}, fmt.Errorf("%w: directory %q does not match branch %q", errTaskDocument, dirs[0], branch)
	}
	docs.Dir = taskRoot + "/" + dirs[0]
	return t.readDocuments(ctx, worktree, docs)
}

// ReadDir reads, from origin/main in worktree, the documents of the task
// directory named dirName (such as 0088_chief_of_staff, a name found by
// IssueIndex.Tasks). Contract: it reads the documents by the same rules as Read; the
// name must be a task directory's form and a directory of origin/main, or the
// task is unreadable.
func (t Tasks) ReadDir(ctx context.Context, worktree, dirName string) (TaskDocs, error) {
	if !isTaskDir(dirName) {
		return TaskDocs{}, fmt.Errorf("%w: %q is not a task directory name", errTaskDocument, dirName)
	}
	task, err := paneinput.ParseTaskID(dirName[:4])
	if err != nil {
		return TaskDocs{}, fmt.Errorf("%w: %q: %w", errTaskDocument, dirName, err)
	}
	names, err := t.listTree(ctx, worktree, taskRoot+"/")
	if err != nil {
		return TaskDocs{}, err
	}
	if !slices.Contains(names, taskRoot+"/"+dirName) {
		return TaskDocs{}, fmt.Errorf("%w: origin/main has no directory %q", errTaskDocument, dirName)
	}
	return t.readDocuments(ctx, worktree, TaskDocs{Task: task, Dir: taskRoot + "/" + dirName})
}

// readDocuments reads the documents docs.Dir holds on origin/main into docs.
func (t Tasks) readDocuments(ctx context.Context, worktree string, docs TaskDocs) (TaskDocs, error) {
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

// issueLinePrefix is the start of the line of a requirements document that
// names the issue the task answers.
const issueLinePrefix = "対応 issue: "

// gitGrep is the git subcommand that searches the tree of a ref.
const gitGrep = "grep"

// issueLineSearch is what IssueIndex hands `git grep`: a line whose first
// column starts with issueLinePrefix, in the requirements documents one
// directory below the task root. -G pins the pattern to a basic regular
// expression, so a grep.patternType setting cannot turn the anchor into text
// and silently leave the index empty.
var issueLineSearch = []string{gitGrep, "-G", "-n", "^" + issueLinePrefix, originMain, "--", ":(glob)" + taskRoot + "/*/" + requirementsFile}

// issueLineForm is one line of `git grep -n` output for issueLineSearch: the
// ref, the path of a requirements document, the line number, and the line.
var issueLineForm = regexp.MustCompile(`^` + regexp.QuoteMeta(originMain) + `:` + regexp.QuoteMeta(taskRoot) + `/([^/:]+)/` + regexp.QuoteMeta(requirementsFile) + `:[0-9]+:(.*)$`)

// issueMentionEnd is the full-width parenthesis that starts the remarks of an
// issue line: what follows it is not read.
const issueMentionEnd = "（"

// hashNumber matches an issue number written `#<digits>`; linkTarget matches
// the destination of a Markdown link, and urlNumber the digits that end it.
var (
	hashNumber = regexp.MustCompile(`#([0-9]+)`)
	linkTarget = regexp.MustCompile(`\]\(([^)\s]*)\)`)
	urlNumber  = regexp.MustCompile(`/([0-9]+)/?$`)
	// bareURLNumber matches the number ending an issue or pull request path
	// anywhere in the text, link or not.
	bareURLNumber = regexp.MustCompile(`/(?:issues|pull)/([0-9]+)`)
)

// issueNumbersOf returns the issue numbers an issue line names, as digit
// strings without leading zeros. Contract: it errs toward naming more numbers,
// so the one-task rule of the caller holds less often and fewer removals are
// proposed. Only the part before the first full-width parenthesis is read. It
// names every `#<digits>` there, link or not, and for each Markdown link
// destination the digits that end it, so a link whose text and destination
// disagree names both numbers.
func issueNumbersOf(text string) map[string]struct{} {
	text = strings.TrimPrefix(text, issueLinePrefix)
	text, _, _ = strings.Cut(text, issueMentionEnd)
	numbers := map[string]struct{}{}
	add := func(digits string) {
		if trimmed := strings.TrimLeft(digits, "0"); trimmed != "" {
			numbers[trimmed] = struct{}{}
		}
	}
	for _, m := range hashNumber.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	for _, m := range bareURLNumber.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	for _, link := range linkTarget.FindAllStringSubmatch(text, -1) {
		if m := urlNumber.FindStringSubmatch(link[1]); m != nil {
			add(m[1])
		}
	}
	return numbers
}

// IssueIndex is the issue lines of every requirements document on
// origin/main, as one `git grep` read them: the task directory of each line
// and the issue numbers it names. Only Tasks.IssueIndex makes one; the zero
// value names no task.
type IssueIndex struct {
	lines []issueLine
}

// issueLine is one issue line: its task directory and the numbers it names.
type issueLine struct {
	dir     string
	numbers map[string]struct{}
}

// IssueIndex reads, from origin/main in worktree, the lines that start with
// issueLinePrefix in every requirements document, with one `git grep`, so a
// round asks git once for every issue it looks up. Contract: a line it
// cannot read as `git grep -n` output for a requirements document of a task
// directory makes the whole reading unreadable; git's exit status 1 (no
// line) is an empty index, never an error.
func (t Tasks) IssueIndex(ctx context.Context, worktree string) (IssueIndex, error) {
	out, err := t.cmds.git(ctx, worktree, issueLineSearch...)
	if err != nil {
		exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
		if !ok || exitErr.Code != 1 {
			return IssueIndex{}, err
		}
		out = nil
	}
	var ix IssueIndex
	for line := range strings.SplitSeq(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		m := issueLineForm.FindStringSubmatch(line)
		if m == nil || !isTaskDir(m[1]) {
			return IssueIndex{}, fmt.Errorf("%w: a line of git grep that is not an issue line of a requirements document", errTaskDocument)
		}
		ix.lines = append(ix.lines, issueLine{dir: m[1], numbers: issueNumbersOf(m[2])})
	}
	return ix, nil
}

// Tasks returns the names of the task directories whose requirements
// document names issue on an issue line, in name order. The caller proposes
// a removal only when exactly one directory is returned, and that is a
// proposal a human and the removal checks still decide.
func (ix IssueIndex) Tasks(issue paneinput.IssueNumber) ([]string, error) {
	if issue.String() == "" {
		return nil, errNoIssueNumber
	}
	var dirs []string
	for _, l := range ix.lines {
		if _, cited := l.numbers[issue.String()]; cited && !slices.Contains(dirs, l.dir) {
			dirs = append(dirs, l.dir)
		}
	}
	slices.Sort(dirs)
	return dirs, nil
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

// show returns the blob at path in rev.
func (t Tasks) show(ctx context.Context, worktree, rev, path string) ([]byte, error) {
	return t.cmds.git(ctx, worktree, "show", rev+":"+path)
}

// errNoSteps is StepsLeft's result for a plan with no step of any form. It is
// not an errTaskDocument: a plan with no steps is well formed.
var errNoSteps = errors.New("no step in the plan")

// stepErrorKind is which of the four conditions made a plan's steps
// unreadable. The zero value is none of them.
type stepErrorKind int

const (
	// stepNoCheckbox is a heading or bold step with no step checkbox in its
	// range. Detail is the step ID.
	stepNoCheckbox stepErrorKind = iota + 1
	// stepNearMiss is a line in chapter 2 that looks like a step line but is
	// in no accepted form. Detail is the line.
	stepNearMiss
	// stepOtherCheckbox is a checkbox in a form-B or form-C step's range that
	// is not a step checkbox. Detail is the line.
	stepOtherCheckbox
	// stepDuplicate is a step ID that appears twice, in any form. Detail is
	// the step ID.
	stepDuplicate
)

// stepError reports why the steps of a plan cannot be read. It wraps
// errTaskDocument.
type stepError struct {
	Kind   stepErrorKind
	Detail string
}

func (e *stepError) Error() string {
	return fmt.Sprintf("%v: steps: kind %d: %q", errTaskDocument, e.Kind, e.Detail)
}

func (e *stepError) Unwrap() error { return errTaskDocument }

var (
	// chapter2Heading is the heading that opens chapter 2 of a plan.
	chapter2Heading = regexp.MustCompile(`^## 2\.[ \t]`)
	// stepHeading is a form-B step: a heading whose text starts with the
	// step ID. The heading's level is the number of '#'.
	stepHeading = regexp.MustCompile(`^(#+)[ \t]+ステップ ([0-9]+-[0-9]+)(?:$|[:：])`)
	// stepBold is the start of a form-C step: a bold paragraph; readSteps
	// also checks the closing "**".
	stepBold = regexp.MustCompile(`^\*\*ステップ ([0-9]+-[0-9]+)[:：]`)
	// nearHeading, nearBold, and nearCheckbox are lines that look like a step
	// line but, once the accepted forms are ruled out, are in none of them.
	nearHeading  = regexp.MustCompile(`^#+[ \t]+ステップ[ \t]*[0-9]`)
	nearBold     = regexp.MustCompile(`^\*\*ステップ[ \t]*[0-9]`)
	nearCheckbox = regexp.MustCompile(`^[ \t]*(?:[-*+]|[0-9]+[.)])[ \t]+\[.\][ \t]+\*\*ステップ[ \t]*[0-9A-Za-z]`)
	// rangeStepBox is a step checkbox inside a form-B or form-C range.
	rangeStepBox = regexp.MustCompile(`^[ \t]*-[ \t]+\[([ x-])\]`)
	// otherBox is a checkbox of any list form.
	otherBox = regexp.MustCompile(`^[ \t]*(?:[-*+]|[0-9]+[.)])[ \t]+\[.\]`)
)

// stepRange is the open range of a form-B or form-C step.
type stepRange struct {
	id string
	// level is the heading level of a form-B step; 0 for form C.
	level int
	marks []byte
}

// stepReader is the state of readSteps.
type stepReader struct {
	steps      map[string]bool
	firstErr   error
	open       *stepRange
	inChapter2 bool
}

func (r *stepReader) fail(kind stepErrorKind, detail string) {
	if r.firstErr == nil {
		r.firstErr = &stepError{Kind: kind, Detail: detail}
	}
}

func (r *stepReader) add(id string, done bool) {
	if _, dup := r.steps[id]; dup {
		r.fail(stepDuplicate, id)
	}
	r.steps[id] = done
}

func (r *stepReader) closeRange() {
	if r.open == nil {
		return
	}
	if len(r.open.marks) == 0 {
		r.fail(stepNoCheckbox, r.open.id)
	}
	r.add(r.open.id, !slices.Contains(r.open.marks, ' '))
	r.open = nil
}

// heading reads a heading line.
func (r *stepReader) heading(text string) {
	level := len(text) - len(strings.TrimLeft(text, "#"))
	if r.open != nil && (r.open.level == 0 || level <= r.open.level) {
		r.closeRange()
	}
	if level <= chapterLevel {
		r.inChapter2 = chapter2Heading.MatchString(text)
		return
	}
	if !r.inChapter2 {
		return
	}
	if m := stepHeading.FindStringSubmatch(text); m != nil {
		r.closeRange()
		r.open = &stepRange{id: m[2], level: len(m[1])}
	} else if nearHeading.MatchString(text) {
		r.fail(stepNearMiss, text)
	}
}

// line reads a line that is not a heading.
func (r *stepReader) line(text string) {
	if m := stepBox.FindStringSubmatch(text); m != nil {
		r.closeRange()
		r.add(m[2], m[1] != " ")
		return
	}
	if !r.inChapter2 {
		return
	}
	if m := stepBold.FindStringSubmatch(text); m != nil && strings.HasSuffix(text, "**") && strings.Count(text, "**") == 2 {
		r.closeRange()
		r.open = &stepRange{id: m[1]}
		return
	}
	if nearBold.MatchString(text) || nearCheckbox.MatchString(text) {
		r.fail(stepNearMiss, text)
		return
	}
	if r.open == nil {
		return
	}
	if m := rangeStepBox.FindStringSubmatch(text); m != nil {
		r.open.marks = append(r.open.marks, m[1][0])
	} else if otherBox.MatchString(text) {
		r.fail(stepOtherCheckbox, text)
	}
}

// readSteps reads the plan's steps from the lines outside code fences, as
// 01_requirements.md section 3.1 defines. Form A is read anywhere; forms B
// and C only in chapter 2. It returns the first problem as a *stepError, and
// the steps then mean nothing.
func readSteps(lines []docLine) (map[string]bool, error) {
	r := &stepReader{steps: map[string]bool{}}
	for _, l := range lines {
		switch {
		case l.fenced:
		case anyHeading.MatchString(l.text):
			r.heading(l.text)
		default:
			r.line(l.text)
		}
	}
	r.closeRange()
	if r.firstErr != nil {
		return nil, r.firstErr
	}
	return r.steps, nil
}
