//go:build test

package doctestutil

import "testing"

// TestRunsCommandHonoursTheName: RunsCommand finds the command it is asked
// about, through a wrapper as well, and not another one; the table of the
// command forms is TestRunsCodex in cmd/codexreview.
func TestRunsCommandHonoursTheName(t *testing.T) {
	for _, tc := range []struct {
		text, name string
		want       bool
	}{
		{"herdr agent prompt --pane x hi", "herdr", true},
		{"env FOO=1 herdr pane run x", "herdr", true},
		{"codex exec review", "herdr", false},
		{"herdr agent prompt x", "codex", false},
	} {
		if got := RunsCommand(tc.text, tc.name); got != tc.want {
			t.Errorf("RunsCommand(%q, %q) = %v, want %v", tc.text, tc.name, got, tc.want)
		}
	}
}
