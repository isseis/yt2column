//go:build test

package chief

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// TestOutputValueNames checks the value names /chief's definition maps: each
// list names every value of its type up to the last declared today, each
// with a name of its own, and leaves out unknown (and running_unknown's ""
// of a decided command); a value just outside each type's range, on either
// side, is named unknown. A value declared after the last one here and left
// unnamed is not caught.
func TestOutputValueNames(t *testing.T) {
	names := OutputValueNames()
	for _, c := range []struct {
		key string
		// first and count are the first value listed and the number of
		// values listed; name names value i.
		first, count int
		name         func(i int) string
	}{
		{"urgency", 0, int(UrgencyWait) + 1, func(i int) string { return urgencyName(Urgency(i)) }},
		{"position.document", 0, int(PositionDocNone) + 1, func(i int) string { return positionDocumentName(PositionDocument(i)) }},
		{"position.progress", 0, int(ProgressTaskDone) + 1, func(i int) string { return progressName(Progress(i)) }},
		{"running_unknown", 1, int(RunningUnknownPanesDisagree), func(i int) string { return runningUnknownName(RunningUnknown(i)) }},
	} {
		t.Run(c.key, func(t *testing.T) {
			got := names[c.key]
			if len(got) != c.count {
				t.Fatalf("names = %v, want %d of them", got, c.count)
			}
			for i, n := range got {
				if n == "" || n == unknownName || slices.Index(got, n) != i || n != c.name(c.first+i) {
					t.Errorf("name %d = %q, want a name of its own", i, n)
				}
			}
			for _, v := range []int{-1, c.first + c.count} {
				if n := c.name(v); n != unknownName {
					t.Errorf("value %d outside the range is named %q, want %q", v, n, unknownName)
				}
			}
		})
	}
	if len(names) != 4 {
		t.Errorf("keys = %v, want the four checked here", names)
	}
	if n := runningUnknownName(RunningUnknownNone); n != "" {
		t.Errorf("a decided command's running_unknown is %q, want empty", n)
	}
}

// TestRunningOutputLaterInput checks that a running command the human typed
// something after is output with later_input set, and one with nothing
// typed after it without.
func TestRunningOutputLaterInput(t *testing.T) {
	cmd, _, err := newRunningCommand(paneinput.CommandRunplan, "0093")
	if err != nil {
		t.Fatal(err)
	}
	for _, later := range []bool{false, true} {
		b, err := json.Marshal(runningOutput(&RunningShown{Command: cmd, Later: later}))
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if got["command"] != "/runplan 0093" || got["later_input"] != later {
			t.Errorf("later %t: running = %s, want /runplan 0093 with later_input %t", later, b, later)
		}
	}
}
