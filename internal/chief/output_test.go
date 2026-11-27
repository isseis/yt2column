//go:build test

package chief

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// TestOutputValueNames checks the names the output gives each row item
// (urgency, position.document, position.progress, running_unknown, state,
// and turn): walking each type's values from zero up to the first named
// unknown (namesUpToUnknown) gives every value up to the last declared
// today, each with a name of its own, and leaves out unknown (and
// running_unknown's "" of a decided command, its value zero); a value just
// outside each type's range, on either side, is named unknown. A value
// declared after the last one here and left unnamed is not caught.
func TestOutputValueNames(t *testing.T) {
	t.Parallel()
	if n := runningUnknownName(RunningUnknownNone); n != "" {
		t.Fatalf("a decided command's running_unknown is %q, want empty", n)
	}
	for _, c := range []struct {
		key string
		// first and count are the first value listed and the number of
		// values listed; name names value i.
		first, count int
		name         func(i int) string
	}{
		{"urgency", 0, int(UrgencyWait) + 1, func(i int) string { return urgencyName(Urgency(i)) }},
		{"position.document", 0, int(PositionDocNone) + 1, func(i int) string { return positionDocumentName(PositionDocument(i)) }},
		{"position.progress", 0, int(ProgressResolving) + 1, func(i int) string { return progressName(Progress(i)) }},
		{"conflict.phase", 0, int(ConflictUnresolved) + 1, func(i int) string { return conflictPhaseName(ConflictPhase(i)) }},
		{"running_unknown", 1, int(RunningUnknownPanesDisagree), func(i int) string { return runningUnknownName(RunningUnknown(i)) }},
		{"state", 0, int(StateConflict) + 1, func(i int) string { return WorkspaceState(i).String() }},
		{"turn", 0, int(TurnNone) + 1, func(i int) string { return turnName(Turn(i)) }},
	} {
		t.Run(c.key, func(t *testing.T) {
			got := namesUpToUnknown(c.name)[c.first:]
			if len(got) != c.count {
				t.Fatalf("names = %v, want %d of them", got, c.count)
			}
			for i, n := range got {
				if n == "" || slices.Index(got, n) != i {
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
}

// TestRunningOutputLaterInput checks that a running command the human typed
// something after is output with later_input set, and one with nothing
// typed after it without.
func TestRunningOutputLaterInput(t *testing.T) {
	t.Parallel()
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

// TestSnapshotOutputTable checks the table members of a snapshot's JSON
// when every table source was read and the question pane is shown by its
// tab's label: table holds the lines, and table_unreadable is [] and not
// null.
func TestSnapshotOutputTable(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	// The label holds a backslash, which the table shows doubled: the
	// output must carry the table as it was laid out, not sanitized a
	// second time.
	w.add(fakeWorkspace{ID: "w1", Label: `a\b`, Branch: taskBranch, Status: "blocked", Screen: "claude-question-single.ansi"})
	snap := take(t, w)
	data, err := json.Marshal(snap.Output())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Table           []string        `json:"table"`
		TableUnreadable json.RawMessage `json:"table_unreadable"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	tableFirstLines(t, got.Table, 1)
	if !slices.Equal(got.Table, snap.Table) {
		t.Errorf("table = %q, want the snapshot's table %q", got.Table, snap.Table)
	}
	if string(got.TableUnreadable) != "[]" {
		t.Errorf("table_unreadable = %s, want []", got.TableUnreadable)
	}
}
