//go:build test

package mergepr

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestOSRunnerReturnsOutput(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nprintf 'hello'\n")
	out, err := NewOSRunner().Run(t.Context(), script)
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if string(out) != "hello" {
		t.Errorf("Run output = %q, want %q", out, "hello")
	}
}

func TestOSRunnerReportsFailure(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nprintf 'boom' >&2\nexit 3\n")
	_, err := NewOSRunner().Run(t.Context(), script)
	if err == nil {
		t.Fatal("Run error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "exit status 3") {
		t.Errorf("Run error = %v, want it to mention the exit status", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("Run error = %v, want it to include stderr", err)
	}
}

func TestOSRunnerTruncatesLargeOutput(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nprintf '%*s' 2097152 ''\n")
	_, err := NewOSRunner().Run(t.Context(), script)
	if !errors.Is(err, errTooLarge) {
		t.Fatalf("Run error = %v, want errTooLarge", err)
	}
}

func TestOSRunnerRedactsCredentials(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nprintf '%s' \"$1\" >&2\nexit 1\n")
	_, err := NewOSRunner().Run(t.Context(), script, "https://ghp_secret@github.com/isseis/yt2column.git")
	if err == nil {
		t.Fatal("Run error = nil, want a failure")
	}
	if strings.Contains(err.Error(), "ghp_secret") {
		t.Errorf("Run error = %v, want the credential redacted", err)
	}
	if !strings.Contains(err.Error(), "://[redacted]@") {
		t.Errorf("Run error = %v, want a redaction marker", err)
	}
}

func TestOSRunnerOverridesGitConfig(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nprintf '%s|%s|%s' \"$GIT_CONFIG_GLOBAL\" \"$GIT_CONFIG_SYSTEM\" \"$GIT_CONFIG_NOSYSTEM\"\n")
	out, err := NewOSRunner().Run(t.Context(), script)
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	if string(out) != "/dev/null|/dev/null|1" {
		t.Errorf("child git config env = %q, want /dev/null|/dev/null|1", out)
	}
}

func TestChildEnvDisablesExecutableConfig(t *testing.T) {
	t.Setenv("GH_TOKEN", "secret")
	t.Setenv("GIT_SSH_COMMAND", "!./tracked-ssh")
	t.Setenv("GIT_ASKPASS", "!./tracked-askpass")
	env := childEnv()

	if !hasEnv(env, "GH_TOKEN=secret") {
		t.Error("child env did not include GH_TOKEN for gh's credential helper")
	}
	if !hasEnv(env, "GIT_CONFIG_KEY_0=core.hooksPath") || !hasEnv(env, "GIT_CONFIG_VALUE_0=/dev/null") {
		t.Error("git child did not disable repository hooks")
	}
	if !hasEnv(env, "GIT_CONFIG_KEY_1=core.fsmonitor") || !hasEnv(env, "GIT_CONFIG_VALUE_1=false") {
		t.Error("git child did not disable the repository fsmonitor")
	}
	if !hasEnv(env, "GIT_CONFIG_KEY_2=credential.helper") || !hasEnv(env, "GIT_CONFIG_VALUE_2=") {
		t.Error("git child did not reset the repository credential helper")
	}
	if !hasEnv(env, "GIT_CONFIG_KEY_3=credential.https://github.com.helper") || !hasEnv(env, "GIT_CONFIG_VALUE_3=!gh auth git-credential") {
		t.Error("git child did not configure gh's credential helper for HTTPS")
	}
	if !hasEnv(env, "GIT_CONFIG_KEY_4=maintenance.auto") || !hasEnv(env, "GIT_CONFIG_VALUE_4=false") {
		t.Error("git child did not disable automatic maintenance")
	}
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_SSH_COMMAND=") || strings.HasPrefix(entry, "GIT_ASKPASS=") {
			t.Errorf("child env kept the command-valued variable %q", entry)
		}
	}
}

func hasEnv(env []string, want string) bool {
	return slices.Contains(env, want)
}

func TestOSRunnerEnvAllowlist(t *testing.T) {
	t.Setenv("MERGE_PR_TEST_SECRET", "leaked")
	script := writeScript(t, "#!/bin/sh\nprintf '%s|%s' \"$MERGE_PR_TEST_SECRET\" \"$PATH\"\n")
	out, err := NewOSRunner().Run(t.Context(), script)
	if err != nil {
		t.Fatalf("Run error = %v, want nil", err)
	}
	parts := strings.SplitN(string(out), "|", 2)
	if len(parts) != 2 {
		t.Fatalf("Run output = %q, want a secret|path pair", out)
	}
	if parts[0] != "" {
		t.Errorf("child saw MERGE_PR_TEST_SECRET = %q, want it stripped from the environment", parts[0])
	}
	if parts[1] == "" {
		t.Error("child saw an empty PATH, want it passed through")
	}
}
