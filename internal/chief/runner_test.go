//go:build test

package chief

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// TestCommandsEnvironment checks the environment, directory, and time limit
// each kind of command runs with: gh without GH_REPO and GH_HOST, git and gh
// without inherited GIT_* variables, herdr with the environment unchanged.
func TestCommandsEnvironment(t *testing.T) {
	environ := []string{"GH_REPO=other/repo", "GH_HOST=example.com", "GIT_DIR=/elsewhere", "HERDR_PANE_ID=w1:p1", "KEEP=1"}
	rec := newRecorder(t)
	rec.reply("gh pr list", []byte("gh out"))
	rec.reply("git rev-parse HEAD", []byte("git out"))
	rec.reply("herdr agent list", []byte("herdr out"))
	cmds := rec.commands(environ)
	ctx := context.Background()
	for _, step := range []struct {
		want string
		run  func() ([]byte, error)
	}{
		{"gh out", func() ([]byte, error) { return cmds.gh(ctx, "/work/tree", "pr", "list") }},
		{"git out", func() ([]byte, error) { return cmds.git(ctx, "/work/tree", "rev-parse", "HEAD") }},
		{"herdr out", func() ([]byte, error) { return cmds.herdr(ctx, "agent", "list") }},
	} {
		if out, err := step.run(); err != nil || string(out) != step.want {
			t.Fatalf("output = %q, %v; want %q", out, err, step.want)
		}
	}
	calls := rec.recorded()
	if len(calls) != 3 {
		t.Fatalf("ran %d commands, want 3", len(calls))
	}
	cases := []struct {
		call    recordedCall
		dir     string
		limit   time.Duration
		has     []string
		hasNone []string
	}{
		{calls[0], "/work/tree", ghTimeout, []string{"KEEP=1", "HERDR_PANE_ID=w1:p1", "GIT_OPTIONAL_LOCKS=0"}, []string{"GH_REPO=other/repo", "GH_HOST=example.com", "GIT_DIR=/elsewhere"}},
		{calls[1], "/work/tree", gitTimeout, []string{"KEEP=1", "GH_REPO=other/repo", "GIT_OPTIONAL_LOCKS=0"}, []string{"GIT_DIR=/elsewhere"}},
		{calls[2], "", herdrTimeout, environ, nil},
	}
	if !slices.Equal(calls[2].env, environ) {
		t.Errorf("herdr env = %v, want exactly %v", calls[2].env, environ)
	}
	for _, c := range cases {
		t.Run(c.call.name, func(t *testing.T) {
			if c.call.dir != c.dir {
				t.Errorf("dir = %q, want %q", c.call.dir, c.dir)
			}
			for _, entry := range c.has {
				if !slices.Contains(c.call.env, entry) {
					t.Errorf("env lacks %s: %v", entry, c.call.env)
				}
			}
			for _, entry := range c.hasNone {
				if slices.Contains(c.call.env, entry) {
					t.Errorf("env has %s: %v", entry, c.call.env)
				}
			}
			if c.call.limit <= 0 || c.call.limit > c.limit || c.call.limit < c.limit-time.Second {
				t.Errorf("time left = %v, want just under %v", c.call.limit, c.limit)
			}
		})
	}
}

// TestHerdrEnvironmentNeverInherited checks that herdr gets the environment
// passed in even when it is empty: a nil one would inherit the process's.
func TestHerdrEnvironmentNeverInherited(t *testing.T) {
	rec := newRecorder(t)
	rec.reply("herdr agent list", nil)
	if _, err := rec.commands(nil).herdr(context.Background(), "agent", "list"); err != nil {
		t.Fatal(err)
	}
	if env := rec.recorded()[0].env; env == nil || len(env) != 0 {
		t.Errorf("env = %#v, want an empty non-nil environment", env)
	}
}

// TestCommandsRejectRelativeDir checks that git and gh never run in a
// directory given relative to chief's own.
func TestCommandsRejectRelativeDir(t *testing.T) {
	rec := newRecorder(t)
	cmds := rec.commands(nil)
	if _, err := cmds.git(context.Background(), "work/tree", "rev-parse", "HEAD"); !errors.Is(err, errRelativeDir) {
		t.Errorf("git error = %v, want errRelativeDir", err)
	}
	if _, err := cmds.gh(context.Background(), "", "pr", "list"); !errors.Is(err, errRelativeDir) {
		t.Errorf("gh error = %v, want errRelativeDir", err)
	}
	if calls := rec.recorded(); len(calls) != 0 {
		t.Errorf("ran %v", calls)
	}
}
