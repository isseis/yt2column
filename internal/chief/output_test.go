//go:build test

package chief

import (
	"slices"
	"testing"
)

// TestOutputValueNames checks the value names /chief's definition maps: each
// list holds, in order, the name the output gives every declared value of
// its type, with no gap and no repeat, and leaves out unknown; a value just
// outside each type's range, on either side, is named unknown.
func TestOutputValueNames(t *testing.T) {
	names := OutputValueNames()
	for _, c := range []struct {
		key string
		// count is the number of declared values; name names value i.
		count int
		name  func(i int) string
	}{
		{"state", int(StateTaskDone) + 1, func(i int) string { return WorkspaceState(i).String() }},
		{"urgency", int(UrgencyWait) + 1, func(i int) string { return urgencyName(Urgency(i)) }},
		{"turn", int(TurnNone) + 1, func(i int) string { return turnName(Turn(i)) }},
		{"position.document", int(PositionDocNone) + 1, func(i int) string { return positionDocumentName(PositionDocument(i)) }},
		{"position.progress", int(ProgressTaskDone) + 1, func(i int) string { return progressName(Progress(i)) }},
	} {
		t.Run(c.key, func(t *testing.T) {
			got := names[c.key]
			if len(got) != c.count {
				t.Fatalf("names = %v, want %d of them", got, c.count)
			}
			for i, n := range got {
				if n != c.name(i) || n == "" || n == unknownName || slices.Index(got, n) != i {
					t.Errorf("name %d = %q, want the output's name %q, once", i, n, c.name(i))
				}
			}
			for _, v := range []int{-1, c.count} {
				if n := c.name(v); n != unknownName {
					t.Errorf("value %d outside the range is named %q, want %q", v, n, unknownName)
				}
			}
		})
	}
	if len(names) != 5 {
		t.Errorf("keys = %v, want the five the definition maps", names)
	}
	// running_unknown is not shown by /chief, so it is not in the map, but
	// its range is named the same way.
	for _, v := range []RunningUnknown{-1, RunningUnknownPanesDisagree + 1} {
		if n := runningUnknownName(v); n != unknownName {
			t.Errorf("running_unknown %d is named %q, want %q", v, n, unknownName)
		}
	}
	if n := runningUnknownName(RunningUnknownNone); n != "" {
		t.Errorf("a decided command's running_unknown is %q, want empty", n)
	}
	seen := map[string]struct{}{}
	for v := RunningUnknownNoSession; v <= RunningUnknownPanesDisagree; v++ {
		n := runningUnknownName(v)
		if _, dup := seen[n]; dup || n == "" || n == unknownName {
			t.Errorf("running_unknown %d is named %q, want a name of its own", v, n)
		}
		seen[n] = struct{}{}
	}
}
