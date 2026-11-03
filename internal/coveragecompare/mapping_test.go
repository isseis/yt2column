//go:build test

package coveragecompare

import (
	"strings"
	"testing"
)

// parseTestDiff parses a diff a test writes, one line per element.
func parseTestDiff(t *testing.T, lines ...string) *diffPatch {
	t.Helper()
	if len(lines) == 0 {
		return &diffPatch{}
	}
	patch, err := parseDiff([]byte(strings.Join(lines, "\n")+"\n"), PointNone)
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	return patch
}

func TestLineMapping(t *testing.T) {
	patch := parseTestDiff(t,
		"diff --git a/m.go b/m.go",
		"--- a/m.go",
		"+++ b/m.go",
		"@@ -0,0 +1,2 @@",
		"+top one",
		"+top two",
		"@@ -5,3 +6,0 @@",
		"-five",
		"-six",
		"-seven",
		"@@ -10,1 +9,2 @@",
		"-ten",
		"+ten a",
		"+ten b",
		"diff --git a/gone.go b/gone.go",
		"deleted file mode 100644",
		"--- a/gone.go",
		"+++ /dev/null",
		"@@ -1,1 +0,0 @@",
		"-package gone",
		"diff --git a/new.go b/new.go",
		"new file mode 100644",
		"--- /dev/null",
		"+++ b/new.go",
		"@@ -0,0 +1,1 @@",
		"+package new",
	)
	mappings, err := buildMappings(patch)
	if err != nil {
		t.Fatalf("buildMappings: %v", err)
	}
	m := mappings["m.go"]
	for base, want := range map[int]int{1: 3, 4: 6, 8: 7, 9: 8, 11: 11, 20: 20} {
		if got := m.headLine(base); got != want {
			t.Errorf("headLine(%d) = %d, want %d", base, got, want)
		}
	}
	if !mappings["gone.go"].Deleted {
		t.Error("a deleted file is not marked deleted")
	}
	if _, ok := mappings["new.go"]; ok {
		t.Error("a new file has a BASE mapping")
	}
	if got := (lineMapping{}).headLine(7); got != 7 {
		t.Errorf("a file the diff does not name maps line 7 to %d", got)
	}

	t.Run("rejects diffs the mapping cannot use", func(t *testing.T) {
		rejected := map[string]*diffPatch{
			"two paths": {Files: []diffFile{{OldPath: "a.go", NewPath: "b.go"}}},
			"removal at line 0": {Files: []diffFile{{OldPath: "a.go", NewPath: "a.go", Hunks: []diffHunk{
				{BaseStart: 0, BaseCount: 1, Lines: []diffChange{{Text: "x"}}},
			}}}},
			"hunks out of order": {Files: []diffFile{{OldPath: "a.go", NewPath: "a.go", Hunks: []diffHunk{
				{BaseStart: 5, BaseCount: 1, Lines: []diffChange{{Text: "x"}}},
				{BaseStart: 5, BaseCount: 0, HeadStart: 5, HeadCount: 1, Lines: []diffChange{{Added: true, Text: "y"}}},
			}}}},
		}
		for name, patch := range rejected {
			t.Run(name, func(t *testing.T) {
				_, err := buildMappings(patch)
				requireStop(t, err, PointNone, InputDiff)
			})
		}
	})
}
