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
