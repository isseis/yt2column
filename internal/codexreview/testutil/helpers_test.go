//go:build test

package codexreviewtestutil

import "testing"

// TestRoleOf checks how a start of the test binary is told apart, from
// arguments and a behavior file the test describes: only the name codex makes
// it a fake, and a fake with no behavior to follow neither acts nor runs the
// tests.
func TestRoleOf(t *testing.T) {
	// The behavior file exists next to /tmp/bin/codex and nowhere else.
	exists := func(path string) bool { return path == "/tmp/bin/"+behaviorFile }
	tests := []struct {
		name string
		args []string
		want role
	}{
		{"test_binary", []string{"/tmp/go-build/codexreview.test", "-test.run=TestRun"}, roleTests},
		{"test_binary_next_to_behavior", []string{"/tmp/bin/codexreview.test"}, roleTests},
		{"no_arguments", nil, roleTests},
		{"name_containing_codex", []string{"/tmp/bin/codexreview"}, roleTests},
		{"codex_with_behavior", []string{"/tmp/bin/codex", "exec", "review"}, roleFakeCodex},
		{"codex_version_with_behavior", []string{"/tmp/bin/codex", "--version"}, roleFakeCodex},
		{"codex_without_behavior", []string{"/tmp/other/codex", "-test.run=TestRun"}, roleNoBehavior},
		{"grandchild", []string{"/tmp/bin/codex", grandchildFlag, "/tmp/pid"}, roleGrandchild},
		{"grandchild_without_behavior", []string{"/tmp/other/codex", grandchildFlag, "/tmp/pid"}, roleGrandchild},
		{"test_binary_with_grandchild_flag", []string{"/tmp/go-build/codexreview.test", grandchildFlag, "/tmp/pid"}, roleTests},
		{"grandchild_flag_without_pid_file", []string{"/tmp/bin/codex", grandchildFlag}, roleFakeCodex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roleOf(tt.args, exists); got != tt.want {
				t.Errorf("roleOf(%q) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}
