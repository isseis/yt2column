//go:build test

// This file is not built in place: the residue check copies it into
// internal/config of a temporary worktree of the commit it checks and runs
// TestLoadIgnoresSlackWebhookURL there.

package config

import (
	"fmt"
	"testing"
)

// TestLoadIgnoresSlackWebhookURL checks that Load accepts an environment
// whose SLACK_WEBHOOK_URL holds a value LoadServer rejects: only the server
// reads the variable.
func TestLoadIgnoresSlackWebhookURL(t *testing.T) {
	for _, tc := range []struct {
		value    string
		sentinel error
	}{
		{"", ErrMissing},
		{"http://mattermost.example.com/hooks/x", ErrInvalid},
	} {
		t.Run(fmt.Sprintf("%q", tc.value), func(t *testing.T) {
			env := with(validEnv(), slackEnv, tc.value)
			if _, err := Load(envLookup(env)); err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			// The server rejects the same value, so the acceptance above is
			// the CLI ignoring it, not the value being valid.
			_, err := LoadServer(envLookup(with(env, "HOME", "/home/user")))
			assertVarError(t, err, tc.sentinel, slackEnv)
		})
	}
}
