//go:build test

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/tasknaming"
)

// tasksDir is the path, from the repository root, of the task directories.
const tasksDir = "docs/tasks"

// taskEntry is one entry directly under docs/tasks/.
type taskEntry struct {
	name string
	dir  bool
}

// malformedTaskEntries returns the names of the entries that are not a task
// directory: a directory whose name has the task directory form. Contract:
// nothing is skipped, a dot-file included, because numreserve and mergepr
// stop on any such entry of origin/main's docs/tasks/.
func malformedTaskEntries(entries []taskEntry) []string {
	var bad []string
	for _, e := range entries {
		if _, ok := tasknaming.ParseTaskDir(e.name); !e.dir || !ok {
			bad = append(bad, e.name)
		}
	}
	return bad
}

func TestMalformedTaskEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []taskEntry
		want    []string
	}{
		{"task directories and the template", []taskEntry{{"0000_template", true}, {"0100_number_reservation", true}}, nil},
		{"a file of the task form", []taskEntry{{"0101_notes", false}}, []string{"0101_notes"}},
		{"a file", []taskEntry{{"README.md", false}, {".DS_Store", false}}, []string{"README.md", ".DS_Store"}},
		{"a directory of another form", []taskEntry{{"draft", true}}, []string{"draft"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := malformedTaskEntries(tt.entries); !slices.Equal(got, tt.want) {
				t.Errorf("malformedTaskEntries = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTaskDirectoryForms: every entry of docs/tasks/ is a task directory.
// numreserve stops on any other entry of origin/main's docs/tasks/, so one
// merged without mergepr's check would stop every later number decision. Like
// TestADRRecords, it reads the work tree, untracked entries included.
func TestTaskDirectoryForms(t *testing.T) {
	dirEntries, err := os.ReadDir(filepath.Join(repositoryRoot, filepath.FromSlash(tasksDir)))
	if err != nil {
		t.Fatalf("read %s: %v", tasksDir, err)
	}
	entries := make([]taskEntry, 0, len(dirEntries))
	for _, e := range dirEntries {
		entries = append(entries, taskEntry{name: e.Name(), dir: e.IsDir()})
	}
	for _, name := range malformedTaskEntries(entries) {
		t.Errorf("%s/%s is not a task directory (NNNN_<name>)", tasksDir, name)
	}
}
