//go:build test

package numreserve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// TestRunnerOptions checks the runners get the tool's bounds and the
// environments the architecture gives (3.2): git without inherited GIT_*
// variables and without prompts, gh without color and without prompts, and
// every other variable kept.
func TestRunnerOptions(t *testing.T) {
	environ := []string{
		"HOME=/home/user", "GH_TOKEN=token", "GIT_DIR=/elsewhere",
		"CLICOLOR_FORCE=1", "GH_FORCE_TTY=1", "NO_COLOR=", "GH_PROMPT_DISABLED=0",
		"GH_REPO=other/repo", "GH_HOST=example.invalid",
	}
	b := bounds{maxOutput: 123, waitDelay: 7 * time.Second}
	git, gh := runnerOptions(environ, b)
	for _, o := range []struct {
		name      string
		maxOutput int
		waitDelay time.Duration
	}{{"git", git.MaxOutput, git.WaitDelay}, {"gh", gh.MaxOutput, gh.WaitDelay}} {
		if o.maxOutput != b.maxOutput || o.waitDelay != b.waitDelay {
			t.Errorf("%s runner: MaxOutput %d, WaitDelay %v; want the bounds' %d, %v", o.name, o.maxOutput, o.waitDelay, b.maxOutput, b.waitDelay)
		}
	}
	if !git.ProcessGroup || git.GroupStopGrace <= 0 {
		t.Errorf("git runner: ProcessGroup %v, GroupStopGrace %v; want its own group with a grace", git.ProcessGroup, git.GroupStopGrace)
	}
	if slices.Contains(git.Env, "GIT_DIR=/elsewhere") || !slices.Contains(git.Env, "GIT_TERMINAL_PROMPT=0") || !slices.Contains(git.Env, "HOME=/home/user") {
		t.Errorf("git env = %q", git.Env)
	}
	want := []string{"HOME=/home/user", "GH_TOKEN=token", "GIT_DIR=/elsewhere", "NO_COLOR=1", "GH_PROMPT_DISABLED=1"}
	if !slices.Equal(gh.Env, want) {
		t.Errorf("gh env = %q, want %q", gh.Env, want)
	}
}

// TestNewRefusesUnderTest checks the production Tool runs no command inside
// go test. git and gh are replaced on PATH by scripts that record being run,
// and the token variables are removed, so a broken guard cannot reach GitHub.
func TestNewRefusesUnderTest(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	script := "#!/bin/sh\necho ran >> " + marker + "\nexit 1\n"
	for _, name := range []string{"git", "gh"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	_, err := New(os.Environ()).Reserve(context.Background(), SpaceTask)
	if !errors.Is(err, ErrUnderTest) {
		t.Errorf("Reserve = %v, want ErrUnderTest", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a git or gh command ran (marker: %v)", statErr)
	}
}

// TestReserveRejectsInvalidSpace checks a Space outside the declared ones
// runs no command.
func TestReserveRejectsInvalidSpace(t *testing.T) {
	for _, space := range []Space{spaceInvalid, SpaceADR + 1} {
		tool := newTestTool(cmdrunnertestutil.New(t), defaultBounds())
		if _, err := tool.Reserve(context.Background(), space); !errors.Is(err, ErrInvalidSpace) {
			t.Errorf("Reserve(%d) = %v, want ErrInvalidSpace", space, err)
		}
	}
}

func TestParseSpace(t *testing.T) {
	for arg, want := range map[string]Space{"task": SpaceTask, "adr": SpaceADR} {
		if got, err := ParseSpace(arg); err != nil || got != want {
			t.Errorf("ParseSpace(%q) = %v, %v; want %v", arg, got, err, want)
		}
	}
	for _, arg := range []string{"", "Task", "ADR", "tasks", "adr ", "-adr"} {
		if _, err := ParseSpace(arg); !errors.Is(err, ErrInvalidSpace) {
			t.Errorf("ParseSpace(%q) = %v, want ErrInvalidSpace", arg, err)
		}
	}
}

// TestStopKindReasons checks every declared kind has its own reason word and
// the zero value and an undeclared value have none.
func TestStopKindReasons(t *testing.T) {
	seen := map[string]StopKind{}
	for _, kind := range StopKinds() {
		reason, ok := kind.Reason()
		if !ok || reason == "" {
			t.Errorf("kind %d has no reason word", kind)
		}
		if other, dup := seen[reason]; dup {
			t.Errorf("kinds %d and %d share the reason %q", other, kind, reason)
		}
		seen[reason] = kind
	}
	for _, kind := range []StopKind{stopInvalid, StopLocalGit + 1} {
		if reason, ok := kind.Reason(); ok {
			t.Errorf("kind %d has the reason %q, want none", kind, reason)
		}
	}
}
