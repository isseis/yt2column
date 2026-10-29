//go:build test

package codexreviewtestutil

import "testing"

// TestRoleOf checks how a start of the test binary is told apart, from
// arguments and an environment built by the test: only the name codex makes
// it a fake, and a fake with no behavior to follow neither acts nor runs the
// tests.
func TestRoleOf(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want role
	}{
		{"test_binary", []string{"/tmp/go-build/codexreview.test", "-test.run=TestRun"}, nil, roleTests},
		{"test_binary_with_behavior", []string{"/tmp/go-build/codexreview.test"}, map[string]string{BehaviorEnv: "{}"}, roleTests},
		{"no_arguments", nil, map[string]string{BehaviorEnv: "{}"}, roleTests},
		{"name_containing_codex", []string{"/tmp/bin/codexreview"}, map[string]string{BehaviorEnv: "{}"}, roleTests},
		{"codex_with_behavior", []string{"/tmp/bin/codex", "exec", "review"}, map[string]string{BehaviorEnv: "{}"}, roleFakeCodex},
		{"bare_codex_with_behavior", []string{"codex", "--version"}, map[string]string{BehaviorEnv: "{}"}, roleFakeCodex},
		{"codex_without_behavior", []string{"/tmp/bin/codex", "-test.run=TestRun"}, nil, roleNoBehavior},
		{"grandchild", []string{"/tmp/bin/codex"}, map[string]string{BehaviorEnv: "{}", grandchildEnv: "/tmp/pid"}, roleGrandchild},
		{"grandchild_without_behavior", []string{"/tmp/bin/codex"}, map[string]string{grandchildEnv: "/tmp/pid"}, roleGrandchild},
		{"test_binary_with_grandchild_variable", []string{"/tmp/go-build/codexreview.test"}, map[string]string{grandchildEnv: "/tmp/pid"}, roleTests},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				v, ok := tt.env[key]
				return v, ok
			}
			if got := roleOf(tt.args, lookup); got != tt.want {
				t.Errorf("roleOf(%q) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}
