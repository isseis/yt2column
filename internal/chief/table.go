package chief

import (
	"iter"
	"strconv"
	"strings"
)

// The status table's layout values (architecture 3.2).
const (
	// tableGap is the number of spaces between two columns.
	tableGap = 2
	// tableNameMax is the widest the name column grows.
	tableNameMax = 20
	// tableContentMin is the narrowest the content column shrinks: it holds
	// the longest escape Sanitize writes and a tab display with the head of
	// a question.
	tableContentMin = 20
	// tablePaneDefault is the pane width assumed when it cannot be read.
	tablePaneDefault = 80
	// tableMarginWhole is the margin Claude Code takes left of a code block.
	tableMarginWhole = 2
	// tableMarginSplit is the margin for a split pane: the code block margin
	// and the frame Herdr draws inside the pane.
	tableMarginSplit = 4
	// tableValueMaxBytes caps a value before it is split into units: the
	// content column shows at most two lines of at most 1000 columns, so a
	// longer value only costs memory.
	tableValueMaxBytes = 16 << 10
)

// The marks and the drawing characters, written as escapes so the only
// non-ASCII literals of this file are the table's Japanese fixed words.
const (
	markNow     = "\U0001F6D1"
	markLater   = "\U0001F536"
	markWait    = "\U0001F7E2"
	markUnknown = "?"
	ruleRune    = "\u2500"
	ellipsis    = "\u2026"
)

// The status table's fixed words. They are the only Japanese text the tool
// prints, under CLAUDE.md's exception for the status table.
const (
	wordLegendNow   = "すぐ"
	wordLegendLater = "あとで"
	wordLegendWait  = "待つ"
	wordHeadStatus  = "状態"
	wordHeadName    = "名前"
	wordHeadTask    = "タスク"
	wordHeadPR      = "PR"
	wordHeadContent = "内容"
	wordNoRows      = "workspace なし"
	wordLater       = "＋指示"
	wordCount       = "件"
	wordMerge       = "マージ確認"
	// wordUnknown is shown for a value no word is defined for; the table
	// never guesses what such a value means.
	wordUnknown = "?"
)

// tableSources is what a round reads from Herdr for the table only. A
// source that could not be read holds its zero value; the table is still
// laid out, at 80 columns and with pane IDs.
type tableSources struct {
	// paneWidth is the chief pane's width, 0 when it was not read.
	paneWidth int
	// paneSplit is true when the chief pane's tab is split; false when the
	// width was not read.
	paneSplit bool
	// paneTabs maps a pane ID, as Herdr wrote it, to its tab; nil when it
	// was not read.
	paneTabs map[string]tabID
	// tabLabels is every tab's label; nil when it was not read.
	tabLabels map[tabID]tabLabel
	// notes says, in English, which source could not be read and why.
	notes []string
}

// unitClass is how the layout treats a unit at a line break.
type unitClass int

const (
	// classOther is a character other than ASCII: a line may break before
	// and after it.
	classOther unitClass = iota
	// classSpace is a space (U+0020): a line may break before and after it.
	classSpace
	// classWord is a printable ASCII character or an escape: a line breaks
	// inside a run of them only when the run is wider than a line.
	classWord
)

// cellUnit is one display unit as the table lays it out.
type cellUnit struct {
	text  string
	width int
	class unitClass
}

var (
	spaceUnit    = cellUnit{text: " ", width: 1, class: classSpace}
	unknownUnit  = cellUnit{text: wordUnknown, width: 1, class: classWord}
	ellipsisUnit = cellUnit{text: ellipsis, width: unitWidth(ellipsis), class: classOther}
)

// cellOf splits text into the table's units by Sanitize's rule, after
// cutting it at a UTF-8 boundary to tableValueMaxBytes. Every value, an
// outside one or a fixed word, takes this path, so the table holds the same
// escapes as the output's values.
//
// A value cut by the cap ends with an ellipsis, so the cut shows even when
// the head kept ends in spaces the layout drops.
func cellOf(text string) []cellUnit {
	capped := CapHead(text, tableValueMaxBytes)
	units := cellFromUnits(sanitizeUnits(capped.Text))
	if capped.Truncated {
		units = append(units, ellipsisUnit)
	}
	return units
}

// cellFromUnits turns display units into the table's units. A newline and a
// tab become one space, chosen by the kind the unit declares; a unit of no
// known kind shows "?", never its text. The class of a text unit is read
// from its text, under CLAUDE.md's exception for the table layout.
func cellFromUnits(units iter.Seq[displayUnit]) []cellUnit {
	var out []cellUnit
	for u := range units {
		switch u.kind {
		case unitNewline, unitTab:
			out = append(out, spaceUnit)
		case unitText:
			out = append(out, cellUnit{text: u.text, width: unitWidth(u.text), class: classOf(u.text)})
		default:
			out = append(out, unknownUnit)
		}
	}
	return out
}

// classOf classifies the text of one display unit.
func classOf(text string) unitClass {
	if text == " " {
		return classSpace
	}
	for i := range len(text) {
		if text[i] < '!' || text[i] > '~' {
			return classOther
		}
	}
	return classWord
}

// joinCells joins the non-empty parts with one space between two of them.
func joinCells(parts ...[]cellUnit) []cellUnit {
	var out []cellUnit
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, spaceUnit)
		}
		out = append(out, p...)
	}
	return out
}

func cellsWidth(units []cellUnit) int {
	w := 0
	for _, u := range units {
		w += u.width
	}
	return w
}

func trimSpaceUnits(units []cellUnit) []cellUnit {
	for len(units) > 0 && units[len(units)-1].class == classSpace {
		units = units[:len(units)-1]
	}
	return units
}

// urgencyMark is the mark of an urgency.
func urgencyMark(u Urgency) string {
	switch u {
	case UrgencyNow:
		return markNow
	case UrgencyLater:
		return markLater
	case UrgencyWait:
		return markWait
	default:
		return markUnknown
	}
}

// progressWord is the status word of a progress.
func progressWord(p Progress) string {
	switch p {
	case ProgressNeedsCheck:
		return "要確認"
	case ProgressAwaitingCreation:
		return "次を提案"
	case ProgressCreating, ProgressRunprRunning, ProgressRunning:
		return "作業中"
	case ProgressMergeConfirmation:
		return "マージ確認"
	case ProgressQuestion:
		return "質問で停止"
	case ProgressAwaitingReview:
		return "レビュー待ち"
	case ProgressTaskDone:
		return "タスク完了"
	default:
		return wordUnknown
	}
}

// documentWord is the document stage of a position.
func documentWord(p Position) string {
	switch p.Document {
	case PositionDocNeedsCheck:
		return "要確認"
	case PositionDocNoTask:
		return "-"
	case PositionDocRequirements:
		return "要件"
	case PositionDocArchitecture:
		return "設計"
	case PositionDocPlan:
		return "計画"
	case PositionDocPlanPR:
		return "PR-" + strconv.Itoa(p.PlanPR)
	case PositionDocNone:
		return "なし"
	default:
		return wordUnknown
	}
}

// runningUnknownWord is why no running command is decided, "" for a decided
// one.
func runningUnknownWord(u RunningUnknown) string {
	switch u {
	case RunningUnknownNone:
		return ""
	case RunningUnknownNoSession:
		return "セッション不明"
	case RunningUnknownUnsupportedPath:
		return "記録の場所が不明"
	case RunningUnknownUnreadable:
		return "記録を読めない"
	case RunningUnknownOverCap:
		return "記録が大きすぎる"
	case RunningUnknownPartialLine:
		return "記録の書き込み中"
	case RunningUnknownNotTaskCommand:
		return "読めるタスクのコマンドなし"
	case RunningUnknownNoSentCommand:
		return "送ったコマンドの記録なし（OpenCode）"
	case RunningUnknownPanesDisagree:
		return "pane ごとに異なる"
	default:
		return wordUnknown
	}
}

// legendLine is the table's first line.
func legendLine() string {
	return markNow + " " + wordLegendNow + "  " + markLater + " " + wordLegendLater + "  " + markWait + " " + wordLegendWait
}

// rowCells is one row's cells: the first line's status, name, task, PR, and
// content, and the second line's document stage.
type rowCells struct {
	status, name, task, pr, doc, content []cellUnit
}

// cellsOf builds a row's cells. Notes for question panes shown by their ID
// are appended to notes.
func cellsOf(row Row, src tableSources, notes *[]string) rowCells {
	task, pr := "-", "-"
	if row.Task != "" {
		task = row.Task
	}
	if row.PR != nil {
		pr = "#" + strconv.Itoa(row.PR.Number)
	}
	return rowCells{
		status:  cellOf(urgencyMark(row.Urgency) + " " + progressWord(row.Position.Progress)),
		name:    cutCell(cellOf(row.Label), tableNameMax),
		task:    cellOf(task),
		pr:      cellOf(pr),
		doc:     cellOf(documentWord(row.Position)),
		content: contentOf(row, src, notes),
	}
}

// contentOf builds a row's content string. The parts are chosen by the
// row's progress, the value the status column shows, so the two columns
// never disagree; an empty part is left out with its separator.
func contentOf(row Row, src tableSources, notes *[]string) []cellUnit {
	switch row.Position.Progress {
	case ProgressMergeConfirmation, ProgressQuestion:
		return questionContent(row, src, notes)
	case ProgressAwaitingReview:
		if row.PR != nil {
			return cellOf(row.PR.Title)
		}
	case ProgressAwaitingCreation:
		if row.Next == nil {
			return nil
		}
		if row.Next.Command != nil {
			return joinCells(cellOf(row.Next.Text), modelCell(row.Next.Model))
		}
		return cellOf(row.Next.Problem)
	case ProgressCreating, ProgressRunprRunning, ProgressRunning:
		if row.Running == nil {
			return cellOf(runningUnknownWord(row.RunningUnknown))
		}
		var later []cellUnit
		if row.Running.Later {
			later = cellOf(wordLater)
		}
		return joinCells(cellOf(row.Running.Command.Text()), later, modelCell(row.Running.Model))
	case ProgressNeedsCheck:
		return cellOf(strings.Join(row.Unreadable, ", "))
	}
	return nil
}

// modelCell is a model in parentheses, nothing for no model.
func modelCell(model string) []cellUnit {
	if model == "" {
		return nil
	}
	return cellOf("(" + model + ")")
}

// questionContent shows one question: the first merge question in the
// tool's order, or else the first question, with the count when there are
// two or more and the merge word when the row is stopped on a question.
func questionContent(row Row, src tableSources, notes *[]string) []cellUnit {
	if len(row.Questions) == 0 {
		return nil
	}
	shown := row.Questions[0]
	for _, q := range row.Questions {
		if q.Question.Merge {
			shown = q
			break
		}
	}
	var count, merge []cellUnit
	if len(row.Questions) > 1 {
		count = cellOf(strconv.Itoa(len(row.Questions)) + " " + wordCount)
	}
	if row.Position.Progress == ProgressQuestion && shown.Question.Merge {
		merge = cellOf(wordMerge)
	}
	return joinCells(count, merge, tabDisplay(shown.Pane, src, notes), cellOf(shown.Question.Text))
}

// tabDisplay shows a question pane by its tab's label, or by its pane ID
// when the tab or its label cannot be found; the latter adds a note.
func tabDisplay(pane string, src tableSources, notes *[]string) []cellUnit {
	text := pane
	reason := ""
	tab, onTab := src.paneTabs[pane]
	label, listed := src.tabLabels[tab]
	switch {
	case src.paneTabs == nil:
		reason = "the panes' tabs could not be read"
	case !onTab:
		reason = "the agent list names no tab for it"
	case src.tabLabels == nil:
		reason = "the tab list could not be read"
	case !listed:
		reason = "its tab is not in the tab list"
	case !label.hasLabel:
		reason = "its tab has no label"
	default:
		text = label.text
	}
	if reason != "" {
		*notes = append(*notes, "question pane "+Sanitize(CapHead(pane, tableValueMaxBytes).Text)+" shown by its ID: "+reason)
	}
	return append(append(cellOf("["), cellOf(text)...), cellOf("]")...)
}

// cutCell fits units into width columns: units wider than that keep the
// longest head that fits with an ellipsis after it, without the spaces
// before the ellipsis. The cut is between units, so an escape is never cut.
func cutCell(units []cellUnit, width int) []cellUnit {
	if cellsWidth(units) <= width {
		return units
	}
	head, w := 0, 0
	for head < len(units) && w+units[head].width <= width-ellipsisUnit.width {
		w += units[head].width
		head++
	}
	out := append([]cellUnit(nil), trimSpaceUnits(units[:head])...)
	return append(out, ellipsisUnit)
}

// wrapContent flows the content into two lines of at most width columns
// (architecture 3.6). The first line ends at the last break point before
// which the line, without its trailing spaces, fits. A break point is a
// boundary next to a space or a character other than ASCII, or a boundary
// inside a word wider than a line. The spaces at a break are dropped, and a
// rest that does not fit the second line is cut with an ellipsis.
func wrapContent(units []cellUnit, width int) (first, second []cellUnit) {
	if len(units) == 0 {
		return nil, nil
	}
	breakable := breakPoints(units, width)
	best, w, trailing := 0, 0, 0
	for p := 1; p <= len(units); p++ {
		u := units[p-1]
		w += u.width
		if u.class == classSpace {
			trailing += u.width
		} else {
			trailing = 0
		}
		if w-trailing > width {
			break
		}
		if breakable[p] {
			best = p
		}
	}
	// Trailing spaces never count against the width, so the first line
	// takes every space at its break, and the rest starts with a unit that
	// is not a space: the spaces at a break are dropped with the first
	// line's trailing spaces.
	return trimSpaceUnits(units[:best]), cutCell(trimSpaceUnits(units[best:]), width)
}

// breakPoints reports, for each boundary 0..len(units), whether a line may
// break there.
func breakPoints(units []cellUnit, width int) []bool {
	breakable := make([]bool, len(units)+1)
	breakable[len(units)] = true
	for start := 0; start < len(units); {
		if units[start].class != classWord {
			breakable[start] = start > 0
			breakable[start+1] = true
			start++
			continue
		}
		end, w := start, 0
		for end < len(units) && units[end].class == classWord {
			w += units[end].width
			end++
		}
		if start > 0 {
			breakable[start] = true
		}
		if w > width {
			for p := start + 1; p < end; p++ {
				breakable[p] = true
			}
		}
		start = end
	}
	return breakable
}

// usableWidth is the width every line of the table should fit in: the
// chief pane's width less Claude Code's code block margin, and less the
// frame of a split pane; 80 columns, not split, when the width was not read.
func usableWidth(src tableSources) int {
	width, margin := src.paneWidth, tableMarginWhole
	if width <= 0 {
		width = tablePaneDefault
	} else if src.paneSplit {
		margin = tableMarginSplit
	}
	return width - margin
}

// layoutTable lays out the status table from the sorted rows. It returns
// the lines, each without a newline or trailing spaces, and src.notes
// followed by a note for each question pane shown by its pane ID.
//
// Contract: every line is empty or starts with a fixed word, a mark, or at
// least four spaces, never with a value from outside, so no value can close
// the code block the table is shown in. The layout reads only the rows and
// src, and changes neither.
func layoutTable(rows []Row, src tableSources) (lines []string, unreadable []string) {
	unreadable = append([]string{}, src.notes...)
	cells := make([]rowCells, len(rows))
	for i, row := range rows {
		cells[i] = cellsOf(row, src, &unreadable)
	}
	head := rowCells{
		status:  cellOf(wordHeadStatus),
		name:    cellOf(wordHeadName),
		task:    cellOf(wordHeadTask),
		pr:      cellOf(wordHeadPR),
		content: cellOf(wordHeadContent),
	}
	widths := [4]int{cellsWidth(head.status), cellsWidth(head.name), cellsWidth(head.task), cellsWidth(head.pr)}
	for _, c := range cells {
		widths[0] = max(widths[0], cellsWidth(c.status))
		widths[1] = max(widths[1], cellsWidth(c.name))
		widths[2] = max(widths[2], cellsWidth(c.task), cellsWidth(c.doc))
		widths[3] = max(widths[3], cellsWidth(c.pr))
	}
	fixed := widths[0] + widths[1] + widths[2] + widths[3] + len(widths)*tableGap
	contentWidth := max(usableWidth(src)-fixed, tableContentMin)

	lines = []string{
		legendLine(),
		"",
		joinColumns(widths, head.status, head.name, head.task, head.pr, head.content),
		strings.Repeat(ruleRune, fixed+contentWidth),
	}
	if len(rows) == 0 {
		return append(lines, wordNoRows), unreadable
	}
	for i, c := range cells {
		if i > 0 {
			lines = append(lines, "")
		}
		first, second := wrapContent(c.content, contentWidth)
		lines = append(lines,
			joinColumns(widths, c.status, c.name, c.task, c.pr, first),
			joinColumns(widths, nil, nil, c.doc, nil, second))
	}
	return lines, unreadable
}

// joinColumns writes one line: the four fixed columns padded to their
// widths, the content, and no trailing spaces.
func joinColumns(widths [4]int, status, name, task, pr, content []cellUnit) string {
	var b strings.Builder
	for i, col := range [4][]cellUnit{status, name, task, pr} {
		for _, u := range col {
			b.WriteString(u.text)
		}
		b.WriteString(strings.Repeat(" ", widths[i]-cellsWidth(col)+tableGap))
	}
	for _, u := range content {
		b.WriteString(u.text)
	}
	return strings.TrimRight(b.String(), " ")
}
