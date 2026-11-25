//go:build test

package chief

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// codeBlockFence matches a line CommonMark could read as the end of a fenced
// code block: zero to three spaces, then three backquotes or three tildes.
var codeBlockFence = regexp.MustCompile("^ {0,3}(```|~~~)")

// errCodeBlock reports a table line that could close the code block or be
// drawn outside it.
var errCodeBlock = errors.New("table line may leave the code block")

// codeBlockCheck checks that a table pasted into a code block cannot close
// it: every line is empty or starts with a fixed word, a status mark, or
// four or more spaces, never starts like a closing fence, and holds no CR or
// LF. The heads are written here as the requirements give them, not taken
// from table.go.
func codeBlockCheck(lines []string) error {
	heads := []string{
		"\U0001F6D1 ", "\U0001F536 ", "\U0001F7E2 ", "? ",
		"状態", "\u2500", "workspace なし",
	}
	for i, line := range lines {
		switch {
		case strings.ContainsAny(line, "\r\n"):
			return fmt.Errorf("%w: line %d holds a CR or LF", errCodeBlock, i)
		case line == "":
			continue
		case codeBlockFence.MatchString(line):
			return fmt.Errorf("%w: line %d starts like a fence: %q", errCodeBlock, i, line)
		case strings.HasPrefix(line, "    "):
			continue
		}
		if !slices.ContainsFunc(heads, func(h string) bool { return strings.HasPrefix(line, h) }) {
			return fmt.Errorf("%w: line %d starts with neither a fixed word, a mark, nor four spaces: %q", errCodeBlock, i, line)
		}
	}
	return nil
}

// layoutChecked lays out the table and fails the test when a line could
// leave the code block. Every test that builds a table goes through it.
func layoutChecked(t *testing.T, rows []Row, src tableSources) (lines, unreadable []string) {
	t.Helper()
	lines, unreadable = layoutTable(rows, src)
	if err := codeBlockCheck(lines); err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(lines, "\n"))
	}
	return lines, unreadable
}

func cellText(units []cellUnit) string {
	var b strings.Builder
	for _, u := range units {
		b.WriteString(u.text)
	}
	return b.String()
}

func question(pane, text string, merge bool) QuestionItem {
	return QuestionItem{Pane: pane, Question: Question{Text: text, Merge: merge}}
}

// labelledSources is a wide pane whose tab table shows each pane by label.
func labelledSources(paneLabels map[string]string) tableSources {
	src := tableSources{paneWidth: 1000, paneTabs: map[string]tabID{}, tabLabels: map[tabID]tabLabel{}}
	for pane, label := range paneLabels {
		tab := tabID{id: "t-" + pane}
		src.paneTabs[pane] = tab
		src.tabLabels[tab] = tabLabel{text: label, hasLabel: label != ""}
	}
	return src
}

func runningCommand(t *testing.T, kind paneinput.CommandKind, arg string) RunningCommand {
	t.Helper()
	cmd, _, err := newRunningCommand(kind, arg)
	if err != nil {
		t.Fatal(err)
	}
	return cmd
}

// TestTableFixedWords checks every value of each type the table shows a
// word for: the values are counted by the output's name functions up to the
// first unknown, so a value added to a type and its output name without a
// table word fails here.
func TestTableFixedWords(t *testing.T) {
	marks := map[string]string{"now": "\U0001F6D1", "later": "\U0001F536", "wait": "\U0001F7E2"}
	names := namesUpToUnknown(urgencyName)
	for i, name := range names {
		if got := urgencyMark(Urgency(i)); got != marks[name] {
			t.Errorf("urgencyMark(%s) = %q, want %q", name, got, marks[name])
		}
	}
	if got := urgencyMark(Urgency(len(names))); got != "?" {
		t.Errorf("urgencyMark(unknown) = %q, want ?", got)
	}

	progress := map[string]string{
		"needs_check": "要確認", "awaiting_creation": "次を提案", "creating": "作業中",
		"runpr_running": "作業中", "running": "作業中", "merge_confirmation": "マージ確認",
		"question": "質問で停止", "awaiting_review": "レビュー待ち", "task_done": "タスク完了",
		"conflict": "コンフリクト", "conflict_unresolved": "解消できず", "resolving": "解消中",
	}
	names = namesUpToUnknown(progressName)
	for i, name := range names {
		if want, ok := progress[name]; !ok || progressWord(Progress(i)) != want {
			t.Errorf("progressWord(%s) = %q, want %q", name, progressWord(Progress(i)), want)
		}
	}
	if got := progressWord(Progress(len(names))); got != "?" {
		t.Errorf("progressWord(unknown) = %q, want ?", got)
	}

	documents := map[string]string{
		"needs_check": "要確認", "no_task": "-", "requirements": "要件", "architecture": "設計",
		"plan": "計画", "plan_pr": "PR-3", "none": "なし",
	}
	names = namesUpToUnknown(positionDocumentName)
	for i, name := range names {
		got := documentWord(Position{Document: PositionDocument(i), PlanPR: 3})
		if want, ok := documents[name]; !ok || got != want {
			t.Errorf("documentWord(%s) = %q, want %q", name, got, want)
		}
	}
	if got := documentWord(Position{Document: PositionDocument(len(names))}); got != "?" {
		t.Errorf("documentWord(unknown) = %q, want ?", got)
	}

	reasons := map[string]string{
		"": "", "no_session": "セッション不明", "unsupported_path": "記録の場所が不明",
		"unreadable": "記録を読めない", "over_cap": "記録が大きすぎる", "partial_line": "記録の書き込み中",
		"not_task_command": "読めるタスクのコマンドなし", "no_sent_command": "送ったコマンドの記録なし（OpenCode）",
		"panes_disagree": "pane ごとに異なる",
	}
	names = namesUpToUnknown(runningUnknownName)
	for i, name := range names {
		got := runningUnknownWord(RunningUnknown(i))
		if want, ok := reasons[name]; !ok || got != want {
			t.Errorf("runningUnknownWord(%q) = %q, want %q", name, got, want)
		}
	}
	if got := runningUnknownWord(RunningUnknown(len(names))); got != "?" {
		t.Errorf("runningUnknownWord(unknown) = %q, want ?", got)
	}
}

// TestTableContent checks the content string of each status word other
// than the question ones (TestTableQuestions): its parts, in order, and no
// part or separator for a missing value.
func TestTableContent(t *testing.T) {
	runpr := runningCommand(t, paneinput.CommandRunpr, "981")
	tests := []struct {
		name string
		row  Row
		want string
	}{
		{"review with a PR", Row{Position: Position{Progress: ProgressAwaitingReview}, PR: &OpenPR{Number: 9, Title: "feat: x"}}, "feat: x"},
		{"review without a PR", Row{Position: Position{Progress: ProgressAwaitingReview}}, ""},
		{"conflict with a PR", Row{Position: Position{Progress: ProgressConflict}, PR: &OpenPR{Number: 9, Title: "feat: x"}}, "feat: x"},
		{"conflict unresolved with a PR", Row{Position: Position{Progress: ProgressConflictUnresolved}, PR: &OpenPR{Number: 9, Title: "feat: x"}}, "feat: x"},
		{"resolving", Row{Position: Position{Progress: ProgressResolving}, Running: &RunningShown{Command: runningCommand(t, paneinput.CommandMergemain, "9")}}, "/mergemain 9"},
		{"sendable proposal with its model", Row{Position: Position{Progress: ProgressAwaitingCreation}, Next: &Next{Command: &paneinput.Command{}, Text: "/runplan 0102", Model: "opus"}}, "/runplan 0102 (opus)"},
		{"sendable proposal with no model", Row{Position: Position{Progress: ProgressAwaitingCreation}, Next: &Next{Command: &paneinput.Command{}, Text: "/runplan 0102"}}, "/runplan 0102"},
		{"proposal that cannot be sent", Row{Position: Position{Progress: ProgressAwaitingCreation}, Next: &Next{Text: "/runplan 0102", Model: "opus", Problem: "route unreadable"}}, "route unreadable"},
		{"no proposal", Row{Position: Position{Progress: ProgressAwaitingCreation}}, ""},
		{"running with later input and a model", Row{Position: Position{Progress: ProgressRunprRunning}, Running: &RunningShown{Command: runpr, Later: true, Model: "sonnet"}}, "/runpr 981 ＋指示 (sonnet)"},
		{"running with nothing more", Row{Position: Position{Progress: ProgressCreating}, Running: &RunningShown{Command: runpr}}, "/runpr 981"},
		{"running command unknown", Row{Position: Position{Progress: ProgressRunning}, RunningUnknown: RunningUnknownNoSession}, "セッション不明"},
		{"running with no reason", Row{Position: Position{Progress: ProgressRunning}}, ""},
		{"needs check", Row{Position: Position{Progress: ProgressNeedsCheck}, Unreadable: []string{"git status", "gh pr list"}}, "git status, gh pr list"},
		{"task done", Row{Position: Position{Progress: ProgressTaskDone}, PR: &OpenPR{Number: 9, Title: "x"}}, ""},
		{"task done with its removal proposed", Row{Position: Position{Progress: ProgressTaskDone}, RemovalRow: true}, "削除を提案"},
		{"removal row of another progress", Row{Position: Position{Progress: ProgressAwaitingReview}, RemovalRow: true}, ""},
		{"unknown progress", Row{Position: Position{Progress: Progress(99)}, PR: &OpenPR{Number: 9, Title: "x"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var notes []string
			if got := cellText(contentOf(tt.row, labelledSources(nil), &notes)); got != tt.want {
				t.Errorf("content = %q, want %q", got, tt.want)
			}
			if len(notes) != 0 {
				t.Errorf("notes = %q, want none", notes)
			}
		})
	}
}

// TestTableQuestions checks which question a row shows and the words put
// before it.
func TestTableQuestions(t *testing.T) {
	src := labelledSources(map[string]string{"w1:p1": "claude", "w1:p2": "opencode"})
	tests := []struct {
		name      string
		progress  Progress
		questions []QuestionItem
		want      string
	}{
		{"one merge question", ProgressMergeConfirmation, []QuestionItem{question("w1:p1", "merge?", true)}, "[claude] merge?"},
		{"one other question", ProgressQuestion, []QuestionItem{question("w1:p2", "which?", false)}, "[opencode] which?"},
		{"two merge questions show the first", ProgressMergeConfirmation, []QuestionItem{question("w1:p1", "first", true), question("w1:p2", "second", true)}, "2 件 [claude] first"},
		{"a merge question goes first", ProgressQuestion, []QuestionItem{question("w1:p1", "other", false), question("w1:p2", "merge?", true)}, "2 件 マージ確認 [opencode] merge?"},
		{"two other questions show the first", ProgressQuestion, []QuestionItem{question("w1:p2", "a", false), question("w1:p1", "b", false)}, "2 件 [opencode] a"},
		{"an empty question text", ProgressQuestion, []QuestionItem{question("w1:p1", "", false)}, "[claude]"},
		{"no question", ProgressQuestion, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := Row{Position: Position{Progress: tt.progress}, Questions: tt.questions}
			var notes []string
			if got := cellText(contentOf(row, src, &notes)); got != tt.want {
				t.Errorf("content = %q, want %q", got, tt.want)
			}
			layoutChecked(t, []Row{row}, src)
		})
	}
}

// TestTableTabDisplay checks the tab display and the five cases that show
// the pane ID instead, each with a note after the sources' own notes.
func TestTableTabDisplay(t *testing.T) {
	tab := tabID{id: "w1:t1"}
	labelled := map[tabID]tabLabel{tab: {text: "claude", hasLabel: true}}
	onTab := map[string]tabID{"w1:p1": tab}
	tests := []struct {
		name   string
		src    tableSources
		want   string
		noting bool
	}{
		{"label", tableSources{paneTabs: onTab, tabLabels: labelled}, "[claude] q", false},
		{"pane tabs not read", tableSources{tabLabels: labelled}, "[w1:p1] q", true},
		{"pane on no tab", tableSources{paneTabs: map[string]tabID{"w1:p9": tab}, tabLabels: labelled}, "[w1:p1] q", true},
		{"tab list not read", tableSources{paneTabs: onTab}, "[w1:p1] q", true},
		{"tab not listed", tableSources{paneTabs: onTab, tabLabels: map[tabID]tabLabel{{id: "w1:t2"}: {text: "x", hasLabel: true}}}, "[w1:p1] q", true},
		{"empty label", tableSources{paneTabs: onTab, tabLabels: map[tabID]tabLabel{tab: {}}}, "[w1:p1] q", true},
		{"declared without a label", tableSources{paneTabs: onTab, tabLabels: map[tabID]tabLabel{tab: {text: "x"}}}, "[w1:p1] q", true},
	}
	// Each case that shows the pane ID says why in its own note, so a case
	// handled by another case's branch shows up as a repeated note.
	seen := map[string]string{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.src.notes = []string{"tab list: unreadable"}
			row := Row{Position: Position{Progress: ProgressQuestion}, Questions: []QuestionItem{question("w1:p1", "q", false)}}
			lines, unreadable := layoutChecked(t, []Row{row}, tt.src)
			if !strings.HasSuffix(lines[4], "  "+tt.want) {
				t.Errorf("first row line = %q, want it to end with %q", lines[4], tt.want)
			}
			want := 1
			if tt.noting {
				want = 2
			}
			if len(unreadable) != want || unreadable[0] != "tab list: unreadable" {
				t.Errorf("unreadable = %q, want the source note and %d more", unreadable, want-1)
			}
			if tt.noting {
				if !strings.Contains(unreadable[1], "w1:p1") {
					t.Errorf("note %q does not name the pane", unreadable[1])
				}
				if other, ok := seen[unreadable[1]]; ok && other != "declared without a label" && tt.name != "declared without a label" {
					t.Errorf("note %q is the same as case %q's", unreadable[1], other)
				}
				seen[unreadable[1]] = tt.name
			}
		})
	}
}

// TestTableNewlinesAndTabs checks that a newline and a tab show as a space
// in the table only, and that a unit of no kind shows as "?".
func TestTableNewlinesAndTabs(t *testing.T) {
	row := Row{
		Label:     "ws\tone",
		Position:  Position{Progress: ProgressQuestion},
		Questions: []QuestionItem{question("w1:p1", "line one\nline\ttwo", false)},
	}
	before := Row{
		Label:     row.Label,
		Position:  row.Position,
		Questions: []QuestionItem{question("w1:p1", "line one\nline\ttwo", false)},
	}
	lines, _ := layoutChecked(t, []Row{row}, labelledSources(map[string]string{"w1:p1": "a\nb"}))
	if !strings.Contains(lines[4], "ws one") || !strings.HasSuffix(lines[4], "[a b] line one line two") {
		t.Errorf("first row line = %q", lines[4])
	}
	if !reflect.DeepEqual(row, before) {
		t.Errorf("row changed: %+v", row)
	}

	units := func(yield func(displayUnit) bool) {
		for _, u := range []displayUnit{{text: "a", kind: unitText}, {text: "\x1b"}, {text: "\n", kind: unitNewline}, {text: "\t", kind: unitTab}} {
			if !yield(u) {
				return
			}
		}
	}
	if got := cellText(cellFromUnits(units)); got != "a?  " {
		t.Errorf("cellFromUnits = %q, want %q", got, "a?  ")
	}
}

// TestTableShape checks the order of the lines, that every column starts
// at the same display column on the header and every row, the values of
// both lines of a row, and that no line ends with a space.
func TestTableShape(t *testing.T) {
	t.Run("no rows", func(t *testing.T) {
		lines, _ := layoutChecked(t, nil, tableSources{})
		want := []string{"\U0001F6D1 すぐ  \U0001F536 あとで  \U0001F7E2 待つ", ""}
		if len(lines) != 5 || !slices.Equal(lines[:2], want) || lines[4] != "workspace なし" {
			t.Fatalf("lines = %q", lines)
		}
		if !strings.HasPrefix(lines[2], "状態") || strings.Trim(lines[3], "\u2500") != "" {
			t.Errorf("header or rule = %q, %q", lines[2], lines[3])
		}
	})

	rows := []Row{
		{Label: "\u65e5\u672c\u8a9e\u306e\u540d\u524d", Task: "0102", Urgency: UrgencyNow, Position: Position{Document: PositionDocPlanPR, PlanPR: 2, Progress: ProgressAwaitingReview}, PR: &OpenPR{Number: 1012, Title: "\u5168\u89d2\u306e\u984c\u540d"}},
		{Label: "ws", Urgency: UrgencyWait, Position: Position{Document: PositionDocNoTask, Progress: ProgressTaskDone}, PR: &OpenPR{Number: 7}},
	}
	lines, _ := layoutChecked(t, rows, tableSources{paneWidth: 100})
	if len(lines) != 9 || lines[6] != "" {
		t.Fatalf("lines = %q, want legend, blank, header, rule, 2 rows of 2 lines with a blank between", lines)
	}
	// The columns are measured with unitWidth, the layout's own function, so
	// this checks that the columns agree; TestTableGolden checks the widths.
	column := func(line, value string) int {
		i := strings.Index(line, value)
		if i < 0 {
			t.Fatalf("%q not in %q", value, line)
		}
		return unitWidth(line[:i])
	}
	header := lines[2]
	starts := map[string]int{
		"name": column(header, "名前"), "task": column(header, "タスク"),
		"pr": column(header, "PR"), "content": column(header, "内容"),
	}
	checks := []struct {
		line   int
		column string
		value  string
	}{
		{4, "name", "\u65e5\u672c\u8a9e\u306e\u540d\u524d"},
		{4, "task", "0102"},
		{4, "pr", "#1012"},
		{4, "content", "\u5168\u89d2\u306e\u984c\u540d"},
		{5, "task", "PR-2"},
		{7, "name", "ws"},
		{7, "task", "-"},
		{7, "pr", "#7"},
		{8, "task", "-"},
	}
	for _, c := range checks {
		if got := column(lines[c.line], c.value); got != starts[c.column] {
			t.Errorf("line %d: %q starts at column %d, want %d (%s)", c.line, c.value, got, starts[c.column], c.column)
		}
	}
	if !strings.HasPrefix(lines[4], "\U0001F6D1 レビュー待ち") || !strings.HasPrefix(lines[7], "\U0001F7E2 タスク完了") {
		t.Errorf("status cells = %q, %q", lines[4], lines[7])
	}
	if strings.TrimSpace(lines[5]) != "PR-2" || strings.TrimSpace(lines[8]) != "-" {
		t.Errorf("second lines = %q, %q", lines[5], lines[8])
	}
	for i, line := range lines {
		if strings.HasSuffix(line, " ") {
			t.Errorf("line %d ends with a space: %q", i, line)
		}
	}
}

// TestTableNameColumn checks the name column's width and the cut at 20
// columns.
func TestTableNameColumn(t *testing.T) {
	tests := []struct {
		name  string
		label string
		want  string
		width int
	}{
		{"short name widens to the header", "ab", "ab", 4},
		{"20 columns kept", "abcdefghijklmnopqrst", "abcdefghijklmnopqrst", 20},
		{"21 columns cut", "abcdefghijklmnopqrstu", "abcdefghijklmnopqrs\u2026", 20},
		{"wide character before the ellipsis", "abcdefghijklmnopqr\u6f22\u5b57", "abcdefghijklmnopqr\u2026", 19},
		{"wide character across column 20", "abcdefghijklmnopqrs\u6f22", "abcdefghijklmnopqrs\u2026", 20},
		{"spaces before the cut dropped", "abcdefghijklmnopq   xyz", "abcdefghijklmnopq\u2026", 18},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, _ := layoutChecked(t, []Row{{Label: tt.label, Position: Position{Progress: ProgressTaskDone}}}, tableSources{})
			rest, ok := strings.CutPrefix(lines[4], "\U0001F6D1 タスク完了  ")
			if !ok || !strings.HasPrefix(rest, tt.want) {
				t.Fatalf("row line = %q, want the name %q", lines[4], tt.want)
			}
			if got := unitWidth(lines[2][strings.Index(lines[2], "名前"):strings.Index(lines[2], "タスク")]); got != max(tt.width, 4)+2 {
				t.Errorf("name column and gap take %d columns of the header, want %d", got, max(tt.width, 4)+2)
			}
		})
	}
}

// TestTableWidth checks the usable width: the pane width less 2, less 4 for
// a split pane, 80 less 2 when the width was not read, and the content
// column kept at 20 columns when the pane is narrower.
func TestTableWidth(t *testing.T) {
	// Status 9 (the mark, a space, and the three-character needs-check word),
	// name 4, task 6, PR 2, and four gaps of 2: 29 columns before the content
	// column.
	rows := []Row{{
		Label: "ws", Urgency: UrgencyNow,
		Position:   Position{Document: PositionDocNoTask, Progress: ProgressNeedsCheck},
		Unreadable: []string{strings.Repeat("word ", 200)},
	}}
	tests := []struct {
		name    string
		src     tableSources
		usable  int
		content int
	}{
		{"unsplit pane", tableSources{paneWidth: 100}, 98, 69},
		{"split pane", tableSources{paneWidth: 100, paneSplit: true}, 96, 67},
		{"width not read", tableSources{}, 78, 49},
		{"split but width not read", tableSources{paneSplit: true}, 78, 49},
		{"content 21", tableSources{paneWidth: 52}, 50, 21},
		{"content 20", tableSources{paneWidth: 51}, 49, 20},
		{"content would be 19", tableSources{paneWidth: 50}, 48, 20},
		{"width 1", tableSources{paneWidth: 1}, -1, 20},
		{"width 1000", tableSources{paneWidth: 1000}, 998, 969},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, _ := layoutChecked(t, rows, tt.src)
			if got := unitWidth(lines[3]); got != 29+tt.content {
				t.Errorf("rule is %d columns, want %d", got, 29+tt.content)
			}
			for i, line := range lines[4:] {
				if bound := max(tt.usable, 29+tableContentMin); unitWidth(line) > bound {
					t.Errorf("line %d is %d columns, over %d", i+4, unitWidth(line), bound)
				}
			}
			if !strings.HasPrefix(lines[4], "\U0001F6D1 要確認") || !strings.HasPrefix(lines[5], strings.Repeat(" ", 9+2+4+2)+"-") {
				t.Errorf("row lines = %q", lines[4:])
			}
		})
	}
}

// TestTableWrap checks where the content breaks into two lines and where
// the second line is cut (architecture 3.6), at a content width of 20.
func TestTableWrap(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		first, second string
	}{
		{"fits on one line", "abc def", "abc def", ""},
		{"breaks at a space", "aaaa bbbb cccc dddd eeee", "aaaa bbbb cccc dddd", "eeee"},
		{"keeps a word whole", "aaaaaaaaaaaaaaaaa bbbbbbbbbb", "aaaaaaaaaaaaaaaaa", "bbbbbbbbbb"},
		{"breaks a word wider than a line", "x " + strings.Repeat("a", 30), "x " + strings.Repeat("a", 18), strings.Repeat("a", 12)},
		{"breaks between a word and other characters", "abcdefghijklmnopq\u6f22\u5b57", "abcdefghijklmnopq\u6f22", "\u5b57"},
		{"moves a wide character that does not fit", strings.Repeat("a", 19) + "\u6f22", strings.Repeat("a", 19), "\u6f22"},
		{"drops the spaces at a break", strings.Repeat("a", 20) + "   bbb", strings.Repeat("a", 20), "bbb"},
		{"keeps the leading spaces", "  abc  def", "  abc  def", ""},
		{"breaks after leading spaces before a word that fits a line", "   " + strings.Repeat("a", 19), "", strings.Repeat("a", 19)},
		{"a full-width space is not a space", strings.Repeat("a", 19) + "\u3000bb", strings.Repeat("a", 19), "\u3000bb"},
		{"cuts the second line", "aaaa bbbb cccc dddd eeee ffff gggg hhhh iiii", "aaaa bbbb cccc dddd", "eeee ffff gggg hhhh\u2026"},
		{"drops spaces before the ellipsis", "aaaa bbbb cccc dddd eeee ffff gggg hhh iiii", "aaaa bbbb cccc dddd", "eeee ffff gggg hhh\u2026"},
		{"never breaks inside an escape", strings.Repeat("a", 17) + "\x1bb", strings.Repeat("a", 17), `\x1bb`},
		{"never cuts inside an escape", strings.Repeat("a", 20) + " " + strings.Repeat("b", 17) + "\u200bcccc", strings.Repeat("a", 20), strings.Repeat("b", 17) + "\u2026"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, second := wrapContent(cellOf(tt.text), 20)
			if cellText(first) != tt.first || cellText(second) != tt.second {
				t.Errorf("lines = %q, %q, want %q, %q", cellText(first), cellText(second), tt.first, tt.second)
			}
		})
	}
}

// TestTableEscapes puts an ESC, a C1 control, a zero-width space, and an
// invalid byte into each value from outside the table shows, and checks
// that each shows as its escape.
func TestTableEscapes(t *testing.T) {
	const value = "\x1b\u0085\u200b\xff"
	const escaped = `\x1b\u0085\u200b\xff`
	src := labelledSources(map[string]string{"w1:p1": "x", "w1:p2": value})
	runpr := runningCommand(t, paneinput.CommandRunpr, "981")
	tests := []struct {
		name string
		row  Row
		src  tableSources
	}{
		{"name", Row{Label: value}, src},
		{"PR title", Row{Position: Position{Progress: ProgressAwaitingReview}, PR: &OpenPR{Number: 1, Title: value}}, src},
		{"question text", Row{Position: Position{Progress: ProgressQuestion}, Questions: []QuestionItem{question("w1:p1", value, false)}}, src},
		{"problem", Row{Position: Position{Progress: ProgressAwaitingCreation}, Next: &Next{Problem: value}}, src},
		{"unreadable", Row{Position: Position{Progress: ProgressNeedsCheck}, Unreadable: []string{value}}, src},
		{"model", Row{Position: Position{Progress: ProgressRunning}, Running: &RunningShown{Command: runpr, Model: value}}, src},
		{"tab label", Row{Position: Position{Progress: ProgressQuestion}, Questions: []QuestionItem{question("w1:p2", "q", false)}}, src},
		{"pane ID", Row{Position: Position{Progress: ProgressQuestion}, Questions: []QuestionItem{question(value, "q", false)}}, tableSources{paneWidth: 1000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, unreadable := layoutChecked(t, []Row{tt.row}, tt.src)
			table := strings.Join(lines, "\n")
			if !strings.Contains(table, escaped) {
				t.Errorf("table does not show %s:\n%s", escaped, table)
			}
			for _, raw := range []string{"\x1b", "\u0085", "\u200b", "\xff"} {
				if strings.Contains(table+strings.Join(unreadable, ""), raw) {
					t.Errorf("table or notes hold the raw %q", raw)
				}
			}
		})
	}
}

// TestTableLargeValues checks that huge values still fit: two content
// lines and a 20-column name, and a cap at 16 KiB that never splits a
// UTF-8 sequence.
func TestTableLargeValues(t *testing.T) {
	rows := []Row{
		{Label: strings.Repeat("n", 64<<10), Position: Position{Progress: ProgressAwaitingReview}, PR: &OpenPR{Number: 1, Title: strings.Repeat("title ", 64<<10/6)}},
		{Label: "q", Position: Position{Progress: ProgressQuestion}, Questions: []QuestionItem{question("w1:p1", "a"+strings.Repeat("\u0301", 10000), false)}},
	}
	lines, _ := layoutChecked(t, rows, tableSources{})
	if len(lines) != 9 {
		t.Fatalf("lines = %d, want 9", len(lines))
	}
	for _, i := range []int{4, 5, 7, 8} {
		if got := unitWidth(lines[i]); got > 78 {
			t.Errorf("line %d is %d columns", i, got)
		}
	}
	if !strings.Contains(lines[4], strings.Repeat("n", 19)+"\u2026") {
		t.Errorf("name not cut to 20 columns: %q", lines[4])
	}

	// cellOf applies the 16 KiB cap and shows the cut with an ellipsis, also
	// when the head kept ends in spaces; TestCapHead covers the UTF-8 boundary.
	if units := cellOf(strings.Repeat("a", tableValueMaxBytes-1) + "\u6f22"); len(units) != tableValueMaxBytes || units[len(units)-1] != ellipsisUnit {
		t.Errorf("cut at 16 KiB kept %d units, want %d ending in the ellipsis", len(units), tableValueMaxBytes)
	}
	spaced := Row{Position: Position{Progress: ProgressAwaitingReview}, PR: &OpenPR{Number: 1, Title: "Merge?" + strings.Repeat("\n", 17000) + "tail"}}
	if lines, _ := layoutChecked(t, []Row{spaced}, tableSources{}); !strings.HasSuffix(lines[4]+lines[5], "\u2026") {
		t.Errorf("cut of a head ending in spaces does not show: %q", lines[4:])
	}
}

// TestTableGolden compares a whole table with lines written out by hand,
// without the display width function.
func TestTableGolden(t *testing.T) {
	runpr := runningCommand(t, paneinput.CommandRunpr, "981")
	rows := []Row{
		{
			Label: "\u65e5\u672c\u8a9ews", Task: "0102", Urgency: UrgencyNow,
			Position:  Position{Document: PositionDocPlanPR, PlanPR: 2, Progress: ProgressQuestion},
			PR:        &OpenPR{Number: 1012},
			Questions: []QuestionItem{question("w1:p2", "\u3069\u3061\u3089\u306b\u3057\u307e\u3059\u304b\uff1f option-a or option-b", false), question("w1:p3", "second", false)},
		},
		{
			Label: "beta\x1b", Task: "0099", Urgency: UrgencyLater,
			Position: Position{Document: PositionDocNone, Progress: ProgressAwaitingReview},
			PR:       &OpenPR{Number: 985, Title: "feat(chief): lay out the status table in the tool and paste it"},
		},
		{
			Label: "a-very-long-workspace-name-x", Urgency: UrgencyWait,
			Position: Position{Document: PositionDocNoTask, Progress: ProgressRunning},
			Running:  &RunningShown{Command: runpr, Later: true, Model: "sonnet"},
		},
	}
	src := labelledSources(map[string]string{"w1:p2": "claude"})
	src.paneWidth = 80
	sp := func(n int) string { return strings.Repeat(" ", n) }
	want := []string{
		"\U0001F6D1 すぐ  \U0001F536 あとで  \U0001F7E2 待つ",
		"",
		"状態" + sp(13) + "名前" + sp(18) + "タスク" + sp(2) + "PR" + sp(5) + "内容",
		strings.Repeat("\u2500", 78),
		"\U0001F6D1 質問で停止" + sp(4) + "\u65e5\u672c\u8a9ews" + sp(14) + "0102" + sp(4) + "#1012" + sp(2) + "2 件 [claude] \u3069\u3061\u3089\u306b\u3057",
		sp(39) + "PR-2" + sp(11) + "\u307e\u3059\u304b\uff1f option-a or op\u2026",
		"",
		"\U0001F536 レビュー待ち" + sp(2) + `beta\x1b` + sp(14) + "0099" + sp(4) + "#985" + sp(3) + "feat(chief): lay out the",
		sp(39) + "なし" + sp(11) + "status table in the too\u2026",
		"",
		"\U0001F7E2 作業中" + sp(8) + "a-very-long-workspa\u2026" + sp(2) + "-" + sp(7) + "-" + sp(6) + "/runpr 981 ＋指示",
		sp(39) + "-" + sp(14) + "(sonnet)",
	}
	lines, unreadable := layoutChecked(t, rows, src)
	if !slices.Equal(lines, want) {
		t.Errorf("table:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	if len(unreadable) != 0 {
		t.Errorf("unreadable = %q, want none", unreadable)
	}
}

// fenceValues are values that could close a code block or start Markdown.
var fenceValues = []string{"```", "~~~", "   ```", " leading", "#title", "|pipe", "-dash", "x\n```", "x\n~~~ y", "\r```"}

// TestTableKeepsCodeBlockOpen checks a table whose status column is as
// narrow as it gets, with every word unknown. The fence-like values of every
// value from outside are the seeds of FuzzTableKeepsCodeBlockOpen, which
// `make test` runs.
func TestTableKeepsCodeBlockOpen(t *testing.T) {
	t.Run("unknown values", func(t *testing.T) {
		layoutChecked(t, []Row{{Label: "```", Urgency: Urgency(99), Position: Position{Document: PositionDocument(99), Progress: Progress(99)}}}, tableSources{})
	})
}

// fuzzRows builds rows that carry each value from outside the table shows:
// the tab label goes through the sources (see the fuzz target), and pane is
// a question pane no tab is found for, shown by its ID.
func fuzzRows(name, title, text, pane, problem, unreadable, model string) []Row {
	return []Row{
		{Label: name, Position: Position{Progress: ProgressQuestion}, Questions: []QuestionItem{question("w1:p1", text, false), question("w1:p2", text, true)}},
		{Label: name, Position: Position{Progress: ProgressMergeConfirmation}, Questions: []QuestionItem{question(pane, text, true)}},
		{Label: name, Position: Position{Progress: ProgressAwaitingReview}, PR: &OpenPR{Number: 1, Title: title}},
		{Label: name, Position: Position{Progress: ProgressAwaitingCreation}, Next: &Next{Problem: problem}},
		{Label: name, Position: Position{Progress: ProgressAwaitingCreation}, Next: &Next{Command: &paneinput.Command{}, Text: "/runpr 1", Model: model}},
		{Label: name, Position: Position{Progress: ProgressNeedsCheck}, Unreadable: []string{unreadable, unreadable}},
		{Label: name, Position: Position{Progress: ProgressRunning}, Running: &RunningShown{Model: model}},
	}
}

// FuzzTableKeepsCodeBlockOpen checks the code block check and the usable
// width on tables built from arbitrary values. TestMain's PATH isolation
// keeps `go test -fuzz` from starting its workers in this package, so fuzz
// it in a copy of the package without main_test.go; `make test` runs the
// seeds.
func FuzzTableKeepsCodeBlockOpen(f *testing.F) {
	for _, v := range fenceValues {
		f.Add(v, v, v, v, v, v, v, v)
	}
	f.Add("\x1b[2J", "\u202e", "\xff", "\u0301", "\t", "\n\n", "  ", "w1:p9")
	f.Fuzz(func(t *testing.T, name, title, text, tabLabel, problem, unreadable, model, pane string) {
		src := labelledSources(map[string]string{"w1:p1": tabLabel})
		src.paneWidth = 80
		lines, _ := layoutChecked(t, fuzzRows(name, title, text, pane, problem, unreadable, model), src)
		for i, line := range lines {
			if got := unitWidth(line); got > 78 {
				t.Errorf("line %d is %d columns", i, got)
			}
		}
	})
}

// TestCodeBlockCheckRejects checks that the code block check rejects each
// kind of line that could leave the block.
func TestCodeBlockCheckRejects(t *testing.T) {
	for _, line := range []string{"```", "   ```x", "~~~", " ~~~", "value", "#title", "  value", "   value", "\U0001F6D1 a\rb", "    a\nb"} {
		if err := codeBlockCheck([]string{"", line}); !errors.Is(err, errCodeBlock) {
			t.Errorf("codeBlockCheck(%q) = %v, want errCodeBlock", line, err)
		}
	}
	if err := codeBlockCheck([]string{"", "    ```", "\U0001F7E2 x", "workspace なし"}); err != nil {
		t.Errorf("codeBlockCheck rejected allowed lines: %v", err)
	}
}
