//go:build test

package coveragecompare

import (
	"slices"
	"strings"
	"testing"
)

// equalDiffHunk compares two hunks including their lines.
func equalDiffHunk(a, b diffHunk) bool {
	return a.BaseStart == b.BaseStart && a.BaseCount == b.BaseCount &&
		a.HeadStart == b.HeadStart && a.HeadCount == b.HeadCount &&
		slices.Equal(a.Lines, b.Lines)
}

func TestParseDiff(t *testing.T) {
	t.Run("reads modified, new, deleted, and mode-only files", func(t *testing.T) {
		data := strings.Join([]string{
			"diff --git a/internal/a.go b/internal/a.go",
			"index 1111111..2222222 100644",
			"--- a/internal/a.go",
			"+++ b/internal/a.go",
			"@@ -2,0 +3,2 @@ func f() {",
			"+added one",
			"+added two",
			"@@ -10,2 +11,0 @@",
			"-removed one",
			"-removed two",
			"diff --git a/internal/new.go b/internal/new.go",
			"new file mode 100644",
			"index 0000000..1111111",
			"--- /dev/null",
			"+++ b/internal/new.go",
			"@@ -0,0 +1,2 @@",
			"+package x",
			"+",
			"diff --git a/internal/old.go b/internal/old.go",
			"deleted file mode 100644",
			"index 1111111..0000000",
			"--- a/internal/old.go",
			"+++ /dev/null",
			"@@ -1,2 +0,0 @@",
			"-package x",
			"-",
			"diff --git a/run.sh b/run.sh",
			"old mode 100644",
			"new mode 100755",
		}, "\n") + "\n"

		patch, err := parseDiff([]byte(data), PointBase)
		if err != nil {
			t.Fatalf("parseDiff: %v", err)
		}
		if len(patch.Files) != 4 {
			t.Fatalf("files = %+v, want 4", patch.Files)
		}
		modified := patch.Files[0]
		if modified.OldPath != "internal/a.go" || modified.NewPath != "internal/a.go" {
			t.Errorf("modified paths = %q -> %q", modified.OldPath, modified.NewPath)
		}
		if len(modified.Hunks) != 2 {
			t.Fatalf("modified hunks = %+v, want 2", modified.Hunks)
		}
		wantHunks := []diffHunk{
			{BaseStart: 2, BaseCount: 0, HeadStart: 3, HeadCount: 2, Lines: []diffChange{
				{Added: true, Text: "added one"},
				{Added: true, Text: "added two"},
			}},
			{BaseStart: 10, BaseCount: 2, HeadStart: 11, HeadCount: 0, Lines: []diffChange{
				{Text: "removed one"},
				{Text: "removed two"},
			}},
		}
		for i := range wantHunks {
			if !equalDiffHunk(modified.Hunks[i], wantHunks[i]) {
				t.Errorf("modified hunk %d = %+v, want %+v", i, modified.Hunks[i], wantHunks[i])
			}
		}
		created := patch.Files[1]
		if created.OldPath != "" || created.NewPath != "internal/new.go" {
			t.Errorf("new file paths = %q -> %q", created.OldPath, created.NewPath)
		}
		if len(created.Hunks) != 1 || created.Hunks[0].Lines[1].Text != "" {
			t.Errorf("new file hunks = %+v", created.Hunks)
		}
		deleted := patch.Files[2]
		if deleted.OldPath != "internal/old.go" || deleted.NewPath != "" {
			t.Errorf("deleted file paths = %q -> %q", deleted.OldPath, deleted.NewPath)
		}
		modeOnly := patch.Files[3]
		if modeOnly.OldPath != "run.sh" || modeOnly.NewPath != "run.sh" || len(modeOnly.Hunks) != 0 {
			t.Errorf("mode-only file = %+v, want run.sh with no hunks", modeOnly)
		}
	})

	t.Run("accepts the no-newline marker inside a hunk", func(t *testing.T) {
		data := strings.Join([]string{
			"diff --git a/x.go b/x.go",
			"index 1111111..2222222 100644",
			"--- a/x.go",
			"+++ b/x.go",
			"@@ -1 +1 @@",
			"-old",
			`\ No newline at end of file`,
			"+new",
			`\ No newline at end of file`,
		}, "\n") + "\n"
		patch, err := parseDiff([]byte(data), PointBase)
		if err != nil {
			t.Fatalf("parseDiff: %v", err)
		}
		if len(patch.Files) != 1 || len(patch.Files[0].Hunks) != 1 {
			t.Fatalf("files = %+v, want one hunk", patch.Files)
		}
	})

	t.Run("accepts an added line over 1 MiB", func(t *testing.T) {
		long := strings.Repeat("x", 1<<20)
		data := strings.Join([]string{
			"diff --git a/x.go b/x.go",
			"--- a/x.go",
			"+++ b/x.go",
			"@@ -0,0 +1 @@",
			"+" + long,
		}, "\n") + "\n"
		patch, err := parseDiff([]byte(data), PointHead)
		if err != nil {
			t.Fatalf("parseDiff: %v", err)
		}
		if len(patch.Files) != 1 || len(patch.Files[0].Hunks[0].Lines[0].Text) != len(long) {
			t.Fatalf("files = %+v, want the long line", patch.Files)
		}
	})

	tests := map[string]string{
		"an unknown header line":                 "diff --git a/x.go b/x.go\nsimilarity index 90%\n--- a/x.go\n+++ b/x.go\n",
		"a binary file":                          "diff --git a/x.bin b/x.bin\nindex 1111111..2222222 100644\nBinary files a/x.bin and b/x.bin differ\n",
		"a quoted path":                          "diff --git a/x.go b/x.go\n--- \"a/x.go\"\n+++ b/x.go\n",
		"a path with a space":                    "diff --git a/x.go b/x.go\n--- a/x y.go\n+++ b/x.go\n",
		"a mismatched line count":                "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1,2 +1,1 @@\n-old\n+new\n",
		"a context line":                         "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1,2 +1,2 @@\n old\n-old\n+new\n",
		"a malformed hunk header":                "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +x @@\n",
		"a header before the first file":         "index 1111111..2222222 100644\n",
		"a --- line without +++":                 "diff --git a/x.go b/x.go\n--- a/x.go\n",
		"a malformed diff header":                "diff --git x.go y.go\n--- a/x.go\n+++ b/x.go\n",
		"a blank line":                           "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n\n",
		"a hunk before the paths":                "diff --git a/x.go b/x.go\n@@ -0,0 +1 @@\n+x\n",
		"a file that is new and deleted at once": "diff --git a/x.go b/x.go\n--- /dev/null\n+++ /dev/null\n",
		"a record after the paths":               "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\nnew mode 100644\n",
	}

	for name, data := range tests {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := parseDiff([]byte(data), PointBase)
			requireStop(t, err, PointBase, InputDiff)
		})
	}
}

func TestParseDiffReportsTheGivenPoint(t *testing.T) {
	_, err := parseDiff([]byte("not a diff\n"), PointHead)
	requireStop(t, err, PointHead, InputDiff)
}
