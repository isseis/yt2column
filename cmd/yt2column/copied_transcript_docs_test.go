//go:build test

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Paths from cmd/yt2column to the copied-transcript survey documents.
const (
	copiedTranscriptReadmePath       = repositoryRoot + "/testdata/README.md"
	copiedTranscriptArchitecturePath = repositoryRoot + "/docs/tasks/0015_copied_transcript_source/02_architecture.md"
	panelLinesRequirementsPath       = repositoryRoot + "/docs/tasks/0024_copied_transcript_panel_lines/01_requirements.md"
	packageReferencePath             = repositoryRoot + "/docs/dev/developer_guide/package_reference.md"
	translationGlossaryPath          = repositoryRoot + "/docs/translation_glossary.md"
)

// TestCopiedTranscriptSurveyRecords checks the copied-transcript survey record:
// testdata/README.md names the source, license, and provenance of each saved
// sample, and the 0015 architecture records the per-video attributes, the
// observed formats, the h:mm:ss observation, and the long-video selection
// investigation. Whether a sample is saved is read from a list of attribute
// tables: the 0015 table and the 0024 requirements table that records the
// samples 0024 saved. A sample passes when any table in the list has its row
// marked saved; the order of the tables does not matter. The sample set is
// read from testdata/, so a newly saved sample without a record fails this
// guard.
func TestCopiedTranscriptSurveyRecords(t *testing.T) {
	sampleIDs := copiedTranscriptSampleIDs(t)
	readmeSection := sectionByHeading(readDoc(t, copiedTranscriptReadmePath), "## コピーした字幕（文字起こしパネルからのコピー）")
	if readmeSection == "" {
		t.Fatal("testdata/README.md has no copied-transcript section")
	}
	for _, id := range sampleIDs {
		if !strings.Contains(readmeSection, id+".transcript_copy.txt") {
			t.Errorf("testdata/README.md does not name the file %s.transcript_copy.txt", id)
		}
		if !strings.Contains(readmeSection, "https://www.youtube.com/watch?v="+id) {
			t.Errorf("testdata/README.md does not record the URL of %s", id)
		}
	}
	if !strings.Contains(readmeSection, "Creative Commons Attribution 3.0") {
		t.Error("testdata/README.md does not state the CC BY license")
	}
	for _, item := range []string{"取得日", "取得方法", "加工内容", "取得者を特定できる情報"} {
		if !strings.Contains(readmeSection, item) {
			t.Errorf("testdata/README.md copied-transcript section has no %s item", item)
		}
	}

	architecture := readDoc(t, copiedTranscriptArchitecturePath)

	survey := sectionByHeadingPrefix(architecture, "#### 1.4.1.")
	if survey == "" {
		t.Fatal("the architecture has no 1.4.1 survey section")
	}
	attributes := findTable(survey, "testdata/")
	if attributes == nil {
		t.Fatal("1.4.1 has no per-video attribute table")
	}
	for _, column := range []string{"動画 ID", "字幕の言語", "自動/手動", "長さ", "ライセンス", "ブラウザ", "表示言語"} {
		if !tableHasColumn(attributes, column) {
			t.Errorf("1.4.1 attribute table has no %q column", column)
		}
	}
	overrides := findTable(sectionByHeading(readDoc(t, panelLinesRequirementsPath), "### 1.1. 本タスクで保存するサンプルの記録"), "testdata/")
	if overrides == nil {
		t.Fatal("0024 requirements 1.1 has no per-video attribute table")
	}
	for _, id := range sampleIDs {
		if !sampleRecordedSaved([][][]string{attributes, overrides}, id) {
			t.Errorf("no attribute table records that the sample %s is saved", id)
		}
	}
	for _, combo := range []struct{ language, kind string }{
		{"日本語", "自動"}, {"日本語", "手動"}, {"英語", "自動"}, {"英語", "手動"},
	} {
		if !surveyHasCombination(attributes, combo.language, combo.kind) {
			t.Errorf("1.4.1 has no %s/%s row", combo.language, combo.kind)
		}
	}

	formats := findTable(survey, "行の区切り")
	if formats == nil {
		t.Fatal("1.4.1 has no observed-format table")
	}
	for _, column := range []string{"時刻・本文の並び", "読み上げ用ラベル", "時刻の文法", "行の区切り", "空行", "本文の行数", "前後の空白・末尾の改行", "混じる表示項目"} {
		if !tableHasColumn(formats, column) {
			t.Errorf("1.4.1 observed-format table has no %q column", column)
		}
	}
	for _, observed := range []string{"2:10:26", "1:00:07"} {
		if !strings.Contains(survey, observed) {
			t.Errorf("1.4.1 does not record the h:mm:ss observation %s", observed)
		}
	}

	long := sectionByHeadingPrefix(architecture, "#### 1.4.2.")
	if long == "" {
		t.Fatal("the architecture has no 1.4.2 selection section")
	}
	for _, want := range []string{"選択できた", "2 時間 10 分", "CLI"} {
		if !strings.Contains(long, want) {
			t.Errorf("1.4.2 does not record %q", want)
		}
	}
}

// sampleRecordedSaved reports whether any of tables has a row for id whose
// testdata/ column records the sample as saved.
func sampleRecordedSaved(tables [][][]string, id string) bool {
	for _, table := range tables {
		saved := tableColumnIndex(table, "testdata/")
		row := tableRow(table, id)
		if saved >= 0 && saved < len(row) && strings.Contains(row[saved], "保存する") {
			return true
		}
	}
	return false
}

// copiedTranscriptSampleIDs returns the video IDs of the committed
// copied-transcript samples. It reads the directory rather than a fixed list,
// so a newly saved sample without a survey record fails the guard.
func copiedTranscriptSampleIDs(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(repositoryRoot + "/testdata/*.transcript_copy.txt")
	if err != nil {
		t.Fatalf("glob the copied transcript samples: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no copied transcript samples found under testdata/")
	}
	ids := make([]string, len(paths))
	for i, path := range paths {
		ids[i] = strings.TrimSuffix(filepath.Base(path), ".transcript_copy.txt")
	}
	return ids
}

// tableHasColumn reports whether any header cell of table contains name.
func tableHasColumn(table [][]string, name string) bool {
	return tableColumnIndex(table, name) >= 0
}

// tableColumnIndex returns the index of the first header cell containing name,
// or -1 when there is none.
func tableColumnIndex(table [][]string, name string) int {
	if len(table) == 0 {
		return -1
	}
	for i, cell := range table[0] {
		if strings.Contains(stripBackticks(cell), name) {
			return i
		}
	}
	return -1
}

// surveyHasCombination reports whether some row has both language in its
// subtitle-language cell and kind in its automatic/manual cell.
func surveyHasCombination(table [][]string, language, kind string) bool {
	languageColumn := tableColumnIndex(table, "字幕の言語")
	kindColumn := tableColumnIndex(table, "自動/手動")
	if languageColumn < 0 || kindColumn < 0 {
		return false
	}
	for _, row := range table[1:] {
		if len(row) > languageColumn && len(row) > kindColumn &&
			strings.Contains(row[languageColumn], language) && strings.Contains(row[kindColumn], kind) {
			return true
		}
	}
	return false
}

// TestClaudeMDCopiedTranscriptException checks the "Declare, don't infer" item
// in CLAUDE.md states the copied-transcript exception's scope, reason, and
// boundary, the four kinds of panel line the parse skips (the prefix, form A and
// form B chapter headings, and the trailing region), the three ways those skips
// stay inside the boundary, and the rejection of a damaged timestamp. Each
// phrase that pins one sentence occurs in that sentence only, so dropping the
// sentence fails the test even where its words recur elsewhere in the item.
func TestClaudeMDCopiedTranscriptException(t *testing.T) {
	item := claudeBullet(readDoc(t, claudePath), "Declare, don't infer")
	if item == "" {
		t.Fatal("CLAUDE.md has no 'Declare, don't infer' item")
	}
	// Join the wrapped lines so a multi-word phrase matches wherever the
	// document breaks its lines.
	item = strings.Join(strings.Fields(item), " ")
	for _, want := range []string{
		"copied-transcript parser",
		"form A",
		"form B",
		"screen-reader label",
		"Segments",
		"external value",
		"cannot express in a type",
		"internal/transcript",
		"matches neither form is rejected",
		"prefix before the first segment line",
		"Chapter N: ",
		"chapter heading",
		"timestamp-expected position",
		"trailing",
		"prefix ends where the timestamp grammar finds the first segment line",
		"identified by its position",
		"only the explicitly defined lines",
		"rejected, not skipped",
	} {
		if !strings.Contains(item, want) {
			t.Errorf("the 'Declare, don't infer' item does not mention %q", want)
		}
	}
}

// TestCopiedTranscriptDocsUpdated checks the documents that name CopiedSource or
// the copied-transcript exception stay updated, and that the glossary defines
// the six survey terms.
func TestCopiedTranscriptDocsUpdated(t *testing.T) {
	claude := readDoc(t, claudePath)

	fragile := claudeBullet(claude, "Delegate the fragile part")
	if fragile == "" {
		t.Fatal("CLAUDE.md has no 'Delegate the fragile part' item")
	}
	for _, want := range []string{"yt-dlp", "parser", "fallback"} {
		if !strings.Contains(fragile, want) {
			t.Errorf("the 'Delegate the fragile part' item does not mention %q", want)
		}
	}

	// The Architecture Overview entry, not the whole section: the "Declare,
	// don't infer" exception in a later subsection also names CopiedSource, so
	// a section-wide search would pass even if this entry lost it.
	if entry := markdownBullet(claude, "- `TranscriptSource`"); !strings.Contains(entry, "CopiedSource") {
		t.Error("CLAUDE.md's Architecture Overview TranscriptSource entry does not mention CopiedSource")
	}

	overview := readDoc(t, projectOverviewPath)
	for _, heading := range []string{"## パイプライン", "## 想定ディレクトリ構成"} {
		if section := sectionByHeading(overview, heading); !strings.Contains(section, "CopiedSource") {
			t.Errorf("project_overview.md %s does not mention CopiedSource", heading)
		}
	}
	if policies := sectionByHeading(overview, "## 決定済みの方針"); !strings.Contains(policies, "yt-dlp") || !strings.Contains(policies, "代わりの手段") {
		t.Error("project_overview.md 決定済みの方針 does not record the copied-transcript exception")
	}

	reference := readDoc(t, packageReferencePath)
	table := findTable(reference, "Package")
	if table == nil {
		t.Fatal("package_reference.md has no package table")
	}
	row := tableRow(table, "internal/transcript")
	if row == nil {
		t.Fatal("package_reference.md has no internal/transcript row")
	}
	cells := strings.Join(row, " ")
	for _, want := range []string{"CopiedSource", "NewCopiedSource", "ErrCopiedTranscriptFormat"} {
		if !strings.Contains(cells, want) {
			t.Errorf("the package_reference.md internal/transcript row does not mention %s", want)
		}
	}

	glossary := readDoc(t, translationGlossaryPath)
	terms := findTable(glossary, "日本語")
	if terms == nil {
		t.Fatal("translation_glossary.md has no term table")
	}
	for _, term := range []string{"コピーした字幕", "形 A", "形 B", "時刻の行", "本文の行", "読み上げ用ラベル"} {
		if tableRow(terms, term) == nil {
			t.Errorf("translation_glossary.md has no row for %q", term)
		}
	}
}

// TestClaudeBullet pins claudeBullet's boundaries: it includes the item's
// continuation lines, stops at the next top-level bullet, stops at a heading,
// matches the bold name exactly, and returns the empty string when the item is
// absent.
func TestClaudeBullet(t *testing.T) {
	const doc = `# Top
intro
- **A**: a body
  a continuation
- **B**: b body
- **Blank**: body line

  after a blank line
## Next
- **Declare, don't infer**: first line
  second line
- **Delegate the fragile part**: body
`
	for _, tc := range []struct {
		name string
		want string
	}{
		{
			name: "A",
			want: "- **A**: a body\n  a continuation\n",
		},
		{
			name: "B",
			want: "- **B**: b body\n",
		},
		{
			name: "Blank",
			want: "- **Blank**: body line\n",
		},
		{
			name: "Declare, don't infer",
			want: "- **Declare, don't infer**: first line\n  second line\n",
		},
		{
			name: "Delegate the fragile part",
			want: "- **Delegate the fragile part**: body\n",
		},
		{
			name: "Declare",
			want: "",
		},
		{
			name: "Delegate",
			want: "",
		},
		{
			name: "Absent",
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := claudeBullet(doc, tc.name); got != tc.want {
				t.Errorf("claudeBullet(doc, %q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// claudeBullet returns the text of the top-level bullet in doc whose lead-in is
// the bold name (for example "Declare, don't infer"). It is markdownBullet with
// the bold lead-in form.
func claudeBullet(doc, name string) string {
	return markdownBullet(doc, "- **"+name+"**:")
}

// markdownBullet returns the text of the top-level bullet in doc that starts
// with marker, including its continuation lines, up to the next top-level
// bullet, heading, or blank line. The marker is matched exactly, so a name that
// only prefixes another (for example "Delegate" against "Delegate the fragile
// part") does not match, and a missing bullet returns the empty string.
func markdownBullet(doc, marker string) string {
	inItem := false
	var b strings.Builder
	for line := range strings.Lines(doc) {
		if !inItem {
			if strings.HasPrefix(line, marker) {
				inItem = true
				b.WriteString(line)
			}
			continue
		}
		if strings.HasPrefix(line, "- ") || headingLevel(line) > 0 || strings.TrimSpace(line) == "" {
			break
		}
		b.WriteString(line)
	}
	return b.String()
}
