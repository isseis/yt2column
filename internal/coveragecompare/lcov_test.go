//go:build test

package coveragecompare

import (
	"slices"
	"strings"
	"testing"
)

func TestParseLCOV(t *testing.T) {
	t.Run("accepts sections with skipped records", func(t *testing.T) {
		data := strings.Join([]string{
			"TN:",
			"SF:src/a.ts",
			"FN:1,foo",
			"FNF:2",
			"FNDA:3,foo",
			"FN:5,bar",
			"FNDA:0,bar",
			"FNH:1",
			"DA:1,3",
			"LF:2",
			"LH:1",
			"end_of_record",
			"TN:",
			"SF:scripts/b.ts",
			"FN:2,baz",
			"FNDA:1,baz",
			"BRDA:2,1,0,1",
			"BRF:1",
			"BRH:1",
			"end_of_record",
		}, "\n") + "\n"

		report, err := parseLCOV([]byte(data), PointBase)
		if err != nil {
			t.Fatalf("parseLCOV: %v", err)
		}
		want := []lcovFile{
			{Path: "extension/scripts/b.ts", Functions: []lcovFunction{{Line: 2, Name: "baz", Count: 1}}},
			{Path: "extension/src/a.ts", Functions: []lcovFunction{
				{Line: 1, Name: "foo", Count: 3},
				{Line: 5, Name: "bar", Count: 0},
			}},
		}
		if !slices.EqualFunc(report.Files, want, func(a, b lcovFile) bool {
			return a.Path == b.Path && slices.Equal(a.Functions, b.Functions)
		}) {
			t.Errorf("files = %+v, want %+v", report.Files, want)
		}
	})

	t.Run("accepts a function name over 1 MiB", func(t *testing.T) {
		long := strings.Repeat("n", 1<<20)
		data := "SF:src/a.ts\nFN:1," + long + "\nFNDA:1," + long + "\nend_of_record\n"
		report, err := parseLCOV([]byte(data), PointHead)
		if err != nil {
			t.Fatalf("parseLCOV: %v", err)
		}
		if len(report.Files) != 1 || len(report.Files[0].Functions) != 1 || len(report.Files[0].Functions[0].Name) != len(long) {
			t.Fatalf("files = %+v, want one long-named function", report.Files)
		}
	})

	tests := map[string]string{
		"unknown record":                "SF:src/a.ts\nXX:1\nend_of_record\n",
		"unclosed section":              "SF:src/a.ts\nFN:1,a\nFNDA:1,a\n",
		"record outside a section":      "FN:1,a\n",
		"end of record outside":         "end_of_record\n",
		"data record outside a section": "DA:1,1\n",
		"repeated SF":                   "SF:src/a.ts\nend_of_record\nSF:src/a.ts\nend_of_record\n",
		"a section inside another":      "SF:src/a.ts\nSF:scripts/b.ts\nend_of_record\n",
		"FN and FNDA count mismatch":    "SF:src/a.ts\nFN:1,a\nFN:2,b\nFNDA:1,a\nend_of_record\n",
		"FN and FNDA name mismatch":     "SF:src/a.ts\nFN:1,a\nFNDA:1,b\nend_of_record\n",
		"negative line":                 "SF:src/a.ts\nFN:-1,a\nFNDA:0,a\nend_of_record\n",
		"fractional count":              "SF:src/a.ts\nFN:1,a\nFNDA:1.5,a\nend_of_record\n",
		"signed count":                  "SF:src/a.ts\nFN:1,a\nFNDA:+1,a\nend_of_record\n",
		"count that does not fit int":   "SF:src/a.ts\nFN:1,a\nFNDA:" + strings.Repeat("9", 40) + ",a\nend_of_record\n",
		"absolute SF path":              "SF:/etc/passwd\nend_of_record\n",
		"empty SF path":                 "SF:\nend_of_record\n",
		"SF path with a parent element": "SF:src/../../x.ts\nend_of_record\n",
		"FN without a comma":            "SF:src/a.ts\nFN:1\nFNDA:0,a\nend_of_record\n",
		"FNDA without a comma":          "SF:src/a.ts\nFN:1,a\nFNDA:0\nend_of_record\n",
		"blank line":                    "SF:src/a.ts\n\nend_of_record\n",
	}

	for name, data := range tests {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := parseLCOV([]byte(data), PointBase)
			requireStop(t, err, PointBase, InputLCOV)
		})
	}
}

func TestIsExtProduct(t *testing.T) {
	tests := map[string]bool{
		"extension/src/a.ts":         true,
		"extension/src/ui/a.ts":      true,
		"extension/scripts/a.ts":     true,
		"extension/srcx/a.ts":        false,
		"extension/scriptsx/a.ts":    false,
		"extension/test/a.test.ts":   false,
		"extension/eslint.config.js": false,
		"src/a.ts":                   false,
		"extension/src":              false,
	}

	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			if got := isExtProduct(path); got != want {
				t.Errorf("isExtProduct(%q) = %v, want %v", path, got, want)
			}
		})
	}
}
