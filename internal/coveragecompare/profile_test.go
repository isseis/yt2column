//go:build test

package coveragecompare

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const testModulePath = "github.com/isseis/yt2column"

// requireStop fails the test unless err is a StopError of the given kind and
// returns its first reason.
func requireStop(t *testing.T, err error, point Point, input Input) StopReason {
	t.Helper()
	stop, ok := errors.AsType[*StopError](err)
	if !ok {
		t.Fatalf("error = %v, want *StopError", err)
	}
	if stop.Kind != StopUnreadable {
		t.Fatalf("stop kind = %v, want %v (reasons: %+v)", stop.Kind, StopUnreadable, stop.Reasons)
	}
	if len(stop.Reasons) == 0 {
		t.Fatal("stop has no reasons")
	}
	reason := stop.Reasons[0]
	if reason.Point != point {
		t.Errorf("reason point = %v, want %v", reason.Point, point)
	}
	if reason.Input != input {
		t.Errorf("reason input = %v, want %v", reason.Input, input)
	}
	return reason
}

func TestParseProfile(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		want     []profileBlock
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
		{name: "rejects an empty profile", data: "", wantLine: 0},
		{name: "rejects a missing mode line", data: testModulePath + "/x.go:1.1,1.2 1 1\n", wantLine: 1},
		{
			name:     "rejects a mode line that is not mode: set",
			data:     "mode: count\n",
			wantLine: 1,
		},
		{
			name:     "rejects a second mode line",
			data:     "mode: set\nmode: set\n",
			wantLine: 2,
		},
		{
			name:     "rejects a mode line after the first line",
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 1 1\nmode: set\n",
			wantLine: 3,
		},
		{
			name:     "rejects a blank line",
			data:     "mode: set\n\n" + testModulePath + "/x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a malformed block",
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a negative run count",
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 1 -1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a signed run count",
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 1 +1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a statement count that does not fit int",
			data:     "mode: set\n" + testModulePath + "/x.go:1.1,1.2 " + strings.Repeat("9", 40) + " 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a block that starts after it ends",
			data:     "mode: set\n" + testModulePath + "/x.go:5.1,4.9 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a block whose start column is after its end column on one line",
			data:     "mode: set\n" + testModulePath + "/x.go:5.9,5.1 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a file outside the module",
			data:     "mode: set\nexample.com/other/x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a file that is the module path itself",
			data:     "mode: set\n" + testModulePath + ":1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects an absolute file path",
			data:     "mode: set\n" + testModulePath + "//x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a path with a dot element",
			data:     "mode: set\n" + testModulePath + "/./x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name:     "rejects a path with a parent element",
			data:     "mode: set\n" + testModulePath + "/a/../x.go:1.1,1.2 1 1\n",
			wantLine: 2,
		},
		{
			name: "rejects the same block position with a different statement count",
			data: "mode: set\n" +
				testModulePath + "/x.go:1.1,1.2 1 1\n" +
				testModulePath + "/x.go:1.1,1.2 2 1\n",
			wantLine: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile, err := parseGoProfile([]byte(test.data), testModulePath, PointBase)
			if test.wantLine != 0 || test.want == nil {
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
	// AC-32: no input size limit. A single line longer than 1 MiB must not hit
	// an implicit scanner limit.
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
