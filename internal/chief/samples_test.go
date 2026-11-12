//go:build test

package chief

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// TestSamplesFollowReplacementRules checks every file under testdata against
// the replacement rules of testdata/README.md: home paths, UUIDs, and GitHub
// noreply addresses appear only in their replaced forms.
func TestSamplesFollowReplacementRules(t *testing.T) {
	rules := []struct {
		name    string
		found   *regexp.Regexp
		allowed *regexp.Regexp
	}{
		{"home path", regexp.MustCompile(`/Users/[^/\s"]+|-Users-[A-Za-z0-9]+-`), regexp.MustCompile(`^(/Users/user|-Users-user-)$`)},
		{"UUID", regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`), regexp.MustCompile(`^00000000-0000-4000-8000-[0-9]{12}$`)},
		{"noreply address", regexp.MustCompile(`[0-9A-Za-z+_.-]+@users\.noreply\.github\.com`), regexp.MustCompile(`^1\+owner@users\.noreply\.github\.com$`)},
		{"home shorthand", regexp.MustCompile(`/home/[^/\s"]+`), regexp.MustCompile(`^$`)},
	}
	checked := 0
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "README.md" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		for _, r := range rules {
			for _, v := range r.found.FindAllString(string(data), -1) {
				if !r.allowed.MatchString(v) {
					t.Errorf("%s: %s %q is not replaced", path, r.name, v)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no sample was checked")
	}
}

// screenRows lists, for each row of the design's table of what the readers
// read (02_architecture.md 3.4.1), the samples of each agent. A row an agent
// does not have, or a sample that could not be taken, is listed with its
// reason in testdata/README.md and left out here.
var screenRows = map[string]map[string][]string{
	"question shapes": {
		"claude": {
			"claude-question-single.ansi", "claude-question-single-cursor.ansi", "claude-question-multi.ansi",
			"claude-question-tabs.ansi", "claude-question-tabs-second.ansi", "claude-question-tabs-review.ansi", "claude-permission.ansi",
		},
		"opencode": {
			"opencode-question-single.ansi", "opencode-question-single-cursor.ansi", "opencode-question-multi.ansi",
			"opencode-question-tabs.ansi", "opencode-question-tabs-second.ansi", "opencode-question-tabs-review.ansi",
		},
	},
	"merge question": {
		"claude":   {"claude-merge-question.ansi"},
		"opencode": {"opencode-merge-question.ansi"},
	},
	"runpr report": {
		"opencode": {"opencode-runpr-report.txt"},
	},
	"command report": {
		"claude": {"claude-command-report.ansi"},
	},
	"model line and new session": {
		"opencode": {"opencode-after-new.ansi", "opencode-done.ansi"},
	},
	"input line": {
		"claude":   {"claude-idle.ansi", "claude-input-left.ansi", "claude-working.ansi", "claude-after-clear.ansi", "claude-after-model.ansi"},
		"opencode": {"opencode-input-left.ansi", "opencode-done.ansi", "opencode-after-new.ansi"},
	},
}

// TestScreenSamplesCoverEveryRow checks that every row lists only samples
// with an expected reading, and that every sample is in a row and has one.
func TestScreenSamplesCoverEveryRow(t *testing.T) {
	wants := map[string]struct{}{}
	for name := range claudeScreenWants {
		wants[name] = struct{}{}
	}
	for name := range openCodeScreenWants {
		wants[name] = struct{}{}
	}
	listed := map[string]struct{}{}
	for row, agents := range screenRows {
		for agent, files := range agents {
			for _, f := range files {
				if _, ok := wants[f]; !ok {
					t.Errorf("%s/%s: %s has no expected reading", row, agent, f)
				}
				listed[f] = struct{}{}
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join("testdata", "screens"))
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, e := range entries {
		_, inRow := listed[e.Name()]
		_, hasWant := wants[e.Name()]
		if !inRow || !hasWant {
			missing = append(missing, e.Name())
		}
	}
	slices.Sort(missing)
	if len(missing) > 0 {
		t.Errorf("samples without a row or an expected reading: %q", missing)
	}
}
