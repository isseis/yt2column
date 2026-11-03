//go:build test

package coveragecompare

import (
	"slices"
	"testing"
)

// sameFileRanges reports whether two range-helper answers are equal.
func sameFileRanges(a, b fileRanges) bool {
	return a.Path == b.Path && slices.Equal(a.Functions, b.Functions) && slices.Equal(a.FieldGroups, b.FieldGroups)
}

func TestParseRanges(t *testing.T) {
	t.Run("accepts the ranges of the requested files in order", func(t *testing.T) {
		data := `{"files":[` +
			`{"path":"src/a.ts","functions":[{"start":1,"end":3},{"start":5,"end":5}],"fieldGroups":[{"start":10,"end":12}]},` +
			`{"path":"scripts/b.ts","functions":[],"fieldGroups":[]}]}`
		got, err := parseRanges([]byte(data), PointHead, []string{"extension/src/a.ts", "extension/scripts/b.ts"})
		if err != nil {
			t.Fatalf("parseRanges: %v", err)
		}
		want := []fileRanges{
			{
				Path:        "extension/src/a.ts",
				Functions:   []lineRange{{Start: 1, End: 3}, {Start: 5, End: 5}},
				FieldGroups: []lineRange{{Start: 10, End: 12}},
			},
			{
				Path:        "extension/scripts/b.ts",
				Functions:   []lineRange{},
				FieldGroups: []lineRange{},
			},
		}
		if len(got) != len(want) {
			t.Fatalf("files = %+v, want %+v", got, want)
		}
		for i := range want {
			if !sameFileRanges(got[i], want[i]) {
				t.Errorf("file %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("accepts no requested files", func(t *testing.T) {
		got, err := parseRanges([]byte(`{"files":[]}`), PointBase, nil)
		if err != nil {
			t.Fatalf("parseRanges: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("files = %+v, want none", got)
		}
	})

	rejects := []struct {
		name  string
		data  string
		files []string
	}{
		{
			name:  "a value that is not an object",
			data:  `[]`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "data after the object",
			data:  `{"files":[]} {}`,
			files: nil,
		},
		{
			name:  "an unlisted top-level member",
			data:  `{"files":[],"extra":0}`,
			files: nil,
		},
		{
			name:  "a repeated files member",
			data:  `{"files":[],"files":[]}`,
			files: nil,
		},
		{
			name:  "files that is not an array",
			data:  `{"files":{}}`,
			files: nil,
		},
		{
			name:  "a file entry that is not an object",
			data:  `{"files":[1]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "an unlisted file member",
			data:  `{"files":[{"path":"src/a.ts","functions":[],"fieldGroups":[],"extra":0}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a missing path",
			data:  `{"files":[{"functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a path that is not a string",
			data:  `{"files":[{"path":1,"functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "an empty path",
			data:  `{"files":[{"path":"","functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "missing functions",
			data:  `{"files":[{"path":"src/a.ts","fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "functions that are not an array",
			data:  `{"files":[{"path":"src/a.ts","functions":{},"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "missing fieldGroups",
			data:  `{"files":[{"path":"src/a.ts","functions":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "fieldGroups that are not an array",
			data:  `{"files":[{"path":"src/a.ts","functions":[],"fieldGroups":null}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a range that is not an object",
			data:  `{"files":[{"path":"src/a.ts","functions":[1],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "an unlisted range member",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":1,"end":1,"extra":0}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a missing start",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"end":1}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a missing end",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":1}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a fractional start",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":1.5,"end":2}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "an exponent start",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":1e3,"end":1000}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a negative start",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":-1,"end":2}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a start before line 1",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":0,"end":2}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a start beyond int",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":9223372036854775808,"end":9223372036854775808}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "an end before the start",
			data:  `{"files":[{"path":"src/a.ts","functions":[{"start":5,"end":4}],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name:  "a missing file",
			data:  `{"files":[{"path":"src/a.ts","functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts", "extension/scripts/b.ts"},
		},
		{
			name: "an extra file",
			data: `{"files":[{"path":"src/a.ts","functions":[],"fieldGroups":[]},` +
				`{"path":"scripts/b.ts","functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
		{
			name: "a repeated file",
			data: `{"files":[{"path":"src/a.ts","functions":[],"fieldGroups":[]},` +
				`{"path":"src/a.ts","functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts", "extension/scripts/b.ts"},
		},
		{
			name: "files out of order",
			data: `{"files":[{"path":"scripts/b.ts","functions":[],"fieldGroups":[]},` +
				`{"path":"src/a.ts","functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts", "extension/scripts/b.ts"},
		},
		{
			name:  "a path that was not requested",
			data:  `{"files":[{"path":"src/b.ts","functions":[],"fieldGroups":[]}]}`,
			files: []string{"extension/src/a.ts"},
		},
	}

	for _, test := range rejects {
		t.Run("rejects "+test.name, func(t *testing.T) {
			_, err := parseRanges([]byte(test.data), PointBase, test.files)
			reason := requireStop(t, err, PointBase, InputRanges)
			if reason.Line != 1 {
				t.Errorf("reason line = %d, want 1", reason.Line)
			}
		})
	}
}
