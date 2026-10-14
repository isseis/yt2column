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
)

// TestCopiedTranscriptSurveyRecords checks the copied-transcript survey record:
// testdata/README.md names the source, license, and provenance of each saved
// sample, and the task's architecture records the per-video attributes, the
// observed formats, the h:mm:ss observation, and the long-video selection
// investigation. The sample set is read from testdata/, so a newly saved
// sample without a record fails this guard.
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
	saved := tableColumnIndex(attributes, "testdata/")
	for _, id := range sampleIDs {
		row := tableRow(attributes, id)
		if row == nil {
			t.Errorf("1.4.1 attribute table has no row for %s", id)
			continue
		}
		if saved < 0 || saved >= len(row) || !strings.Contains(row[saved], "保存する") {
			t.Errorf("1.4.1 row %s does not record that its sample is saved", id)
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
