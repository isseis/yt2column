//go:build test

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// Path from cmd/yt2column to the copied-transcript sample records.
const copiedTranscriptReadmePath = repositoryRoot + "/testdata/README.md"

// TestCopiedTranscriptSurveyRecords checks that testdata/README.md names each
// committed sample file and its source URL. The sample set is read from
// testdata/, so a newly saved sample without a record fails this guard.
func TestCopiedTranscriptSurveyRecords(t *testing.T) {
	sampleIDs := copiedTranscriptSampleIDs(t)
	readmeSection := doctestutil.SectionByHeading(doctestutil.ReadDoc(t, copiedTranscriptReadmePath), "## コピーした字幕（文字起こしパネルからのコピー）")
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
}

// copiedTranscriptSampleIDs returns the video IDs of the committed
// copied-transcript samples. It reads the directory rather than a fixed list,
// so a newly saved sample without a record fails the guard.
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
