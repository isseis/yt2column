//go:build test

package coveragecompare

import (
	"slices"
	"strings"
	"testing"
)

func TestParseProfile(t *testing.T) {
	tests := []struct {
		name     string
		module   string
		data     string
		want     []profileBlock
		wantErr  bool
		wantLine int
	}{
		{
			name: "accepts blocks and keeps order by file and position",
			data: "mode: set\n" +
				testModulePath + "/internal/a/a.go:3.1,4.2 2 1\n" +
				testModulePath + "/cmd/b/b.go:1.1,1.9 1 0\n",
			want: []profileBlock{
				{File: "cmd/b/b.go", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 9, Statements: 1, Count: 0},
				{File: "internal/a/a.go", StartLine: 3, StartCol: 1, EndLine: 4, EndCol: 2, Statements: 2, Count: 1},
			},
		},
		{
			name: "merges repeated blocks and keeps a run count when any test binary ran the block",
			data: "mode: set\n" +
				testModulePath + "/x.go:1.1,1.2 1 0\n" +
				testModulePath + "/x.go:1.1,1.2 1 3\n" +
				testModulePath + "/x.go:1.1,1.2 1 0\n",
			want: []profileBlock{
				{File: "x.go", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 2, Statements: 1, Count: 3},
			},
		},
		{
			name: "accepts a profile with no blocks",
			data: "mode: set\n",
			want: []profileBlock{},
		},
		{
			name:   "accepts a file under another module path",
			module: "example.com/other",
			data:   "mode: set\nexample.com/other/x.go:1.1,1.2 1 1\n",
			want:   []profileBlock{{File: "x.go", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 2, Statements: 1, Count: 1}},
		},
		{
			name:    "rejects an empty profile",
			wantErr: true, data: "", wantLine: 0,
		},
		{
			name:    "rejects a missing mode line",
			wantErr: true, data: testModulePath + "/x.go:1.1,1.2 1 1\n", wantLine: 1,
		},
		{
			name:     "rejects a mode line that is not mode: set",
			wantErr:  true,
			data:     "mode: count\n",
			wantLine: 1,
		},
		{
			name:     "rejects a second mode line",
			wantErr:  true,
			data:     "mode: set\nmode: set\n",
			wantLine: 2,
		},
		{
			name:     "rejects a blank line",
			wantErr:  true,
			data:     "mode: set\n\n" + testModulePath + "/x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a malformed block",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a negative run count",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 1 -1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a signed run count",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 1 +1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a statement count that does not fit int",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 " + strings.Repeat("9", 40) + " 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a block that starts after it ends",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/x.go:5.1,4.9 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a block whose start column is after its end column on one line",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/x.go:5.9,5.1 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a file outside the module",
			wantErr:  true,
			data:     "mode: set\nexample.com/other/x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a file that is the module path itself",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + ":1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a path with an empty element",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "//x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a path with a dot element",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/./x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a path with a parent element",
			wantErr:  true,
			data:     "mode: set\n" + testModulePath + "/a/../x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:    "rejects the same block position with a different statement count",
			wantErr: true,
			data: "mode: set\n" +
				testModulePath + "/x.go:1.1,1.2 1 1\n" +
				testModulePath + "/x.go:1.1,1.2 2 1\n",
			wantLine: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := test.module
			if module == "" {
				module = testModulePath
			}
			profile, err := parseGoProfile([]byte(test.data), module, PointBase)
			if test.wantErr {
				reason := requireStop(t, err, PointBase, InputGoProfile)
				if test.wantLine != 0 && reason.Line != test.wantLine {
					t.Errorf("reason line = %d, want %d", reason.Line, test.wantLine)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoProfile: %v", err)
			}
			if !slices.Equal(profile.Blocks, test.want) {
				t.Errorf("blocks = %+v, want %+v", profile.Blocks, test.want)
			}
		})
	}
}

func TestParseProfileAcceptsLongLine(t *testing.T) {
	// No input size limit: a single line longer than 1 MiB must not hit an
	// implicit scanner limit.
	longFile := strings.Repeat("a", 1<<20)
	data := "mode: set\n" + testModulePath + "/" + longFile + ".go:1.1,1.2 1 1\n"
	profile, err := parseGoProfile([]byte(data), testModulePath, PointHead)
	if err != nil {
		t.Fatalf("parseGoProfile: %v", err)
	}
	if len(profile.Blocks) != 1 || len(profile.Blocks[0].File) != len(longFile)+len(".go") {
		t.Fatalf("blocks = %+v, want one block with the long file", profile.Blocks)
	}
}
