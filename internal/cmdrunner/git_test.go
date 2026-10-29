//go:build test

package cmdrunner

import (
	"slices"
	"testing"
)

func TestGitEnvDropsInheritedGitVariables(t *testing.T) {
	environ := []string{
		"HOME=/home/user",
		// Inherited, these would point git at another repository and config.
		"GIT_DIR=/elsewhere/.git",
		"GIT_CONFIG_PARAMETERS='core.fsmonitor'='true'",
		// Only the prefix is dropped, not a name that contains GIT_.
		"MY_GIT_DIR=/kept",
		"PATH=/usr/bin",
	}
	got := GitEnv(environ)
	want := []string{"HOME=/home/user", "MY_GIT_DIR=/kept", "PATH=/usr/bin", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0"}
	if !slices.Equal(got, want) {
		t.Errorf("GitEnv = %q, want %q", got, want)
	}
}

func TestGitHardeningReturnsFreshSlice(t *testing.T) {
	first := GitHardening()
	want := slices.Clone(first)
	for i := range first {
		first[i] = "edited"
	}
	if got := GitHardening(); !slices.Equal(got, want) {
		t.Errorf("GitHardening after editing an earlier result = %q, want %q", got, want)
	}
}
