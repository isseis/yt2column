//go:build test

package mergepr

import (
	"errors"
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
