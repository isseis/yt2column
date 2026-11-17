//go:build test

package chief

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

func issueNumber(t *testing.T, s string) paneinput.IssueNumber {
	t.Helper()
	n, err := paneinput.ParseIssueNumber(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// spaceHome returns a new directory under /tmp whose path holds only the
// characters a transcript directory allows (t.TempDir's path can hold "_"),
// removed when the test ends.
func spaceHome(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ts")
	if err != nil {
		t.Fatalf("make a home under /tmp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestNewTaskSpace(t *testing.T) {
	space, err := NewTaskSpace("/home/user", issueNumber(t, "42"))
	if err != nil {
		t.Fatal(err)
	}
	if space.Path() != "/home/user/.herdr/worktrees/yt2column/issue-42" || space.Label() != "issue-42" || space.Issue().String() != "42" {
		t.Errorf("space = %q, %q, %q", space.Path(), space.Label(), space.Issue())
	}
	cases := []struct {
		name  string
		home  string
		issue paneinput.IssueNumber
		want  error
	}{
		{"no issue", "/home/user", paneinput.IssueNumber{}, errNoIssue},
		{"relative home", "home/user", issueNumber(t, "42"), errHomeNotAbsolute},
		{"empty home", "", issueNumber(t, "42"), errHomeNotAbsolute},
		{"a character a transcript path cannot be decided for", "/home/us_er", issueNumber(t, "42"), errUnsupportedPath},
		{"a space in the home", "/home/my user", issueNumber(t, "42"), errUnsupportedPath},
		{"a non-ASCII home", "/home/\u00e9", issueNumber(t, "42"), errUnsupportedPath},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewTaskSpace(c.home, c.issue); !errors.Is(err, c.want) {
				t.Errorf("NewTaskSpace = %v, want %v", err, c.want)
			}
		})
	}
}

func TestTaskSpaceWorktree(t *testing.T) {
	space := func(t *testing.T, home string) TaskSpace {
		t.Helper()
		s, err := NewTaskSpace(home, issueNumber(t, "7"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	t.Run("a path that does not exist yet keeps its rest after the resolved ancestor", func(t *testing.T) {
		home := spaceHome(t)
		got, err := space(t, home).Worktree()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, ".herdr/worktrees/yt2column/issue-7"); got.String() != want {
			t.Errorf("Worktree = %q, want %q", got, want)
		}
	})
	t.Run("a home reached through a link resolves to its target", func(t *testing.T) {
		base := spaceHome(t)
		if err := os.Mkdir(filepath.Join(base, "real"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("real", filepath.Join(base, "link")); err != nil {
			t.Fatal(err)
		}
		got, err := space(t, filepath.Join(base, "link")).Worktree()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(base, "real/.herdr/worktrees/yt2column/issue-7"); got.String() != want {
			t.Errorf("Worktree = %q, want %q", got, want)
		}
	})
	t.Run("a link inside the existing part resolves too", func(t *testing.T) {
		home := spaceHome(t)
		elsewhere := spaceHome(t)
		if err := os.Mkdir(filepath.Join(home, ".herdr"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, filepath.Join(home, ".herdr", "worktrees")); err != nil {
			t.Fatal(err)
		}
		got, err := space(t, home).Worktree()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(elsewhere, "yt2column/issue-7"); got.String() != want {
			t.Errorf("Worktree = %q, want %q", got, want)
		}
	})
	t.Run("an existing directory at the path is still the path", func(t *testing.T) {
		home := spaceHome(t)
		target := filepath.Join(home, ".herdr/worktrees/yt2column/issue-7")
		if err := os.MkdirAll(target, 0o700); err != nil {
			t.Fatal(err)
		}
		got, err := space(t, home).Worktree()
		if err != nil || got.String() != target {
			t.Errorf("Worktree = %q, %v, want %q", got, err, target)
		}
	})
	t.Run("a link that resolves to a character a transcript path cannot hold", func(t *testing.T) {
		base := spaceHome(t)
		if err := os.Mkdir(filepath.Join(base, "bad_dir"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("bad_dir", filepath.Join(base, "link")); err != nil {
			t.Fatal(err)
		}
		s := space(t, filepath.Join(base, "link"))
		if _, err := s.Worktree(); !errors.Is(err, errUnsupportedPath) {
			t.Errorf("Worktree = %v, want errUnsupportedPath", err)
		}
	})
	t.Run("a part of the path that is not a directory", func(t *testing.T) {
		for _, name := range []string{".herdr", ".herdr/worktrees/yt2column"} {
			home := spaceHome(t)
			full := filepath.Join(home, name)
			if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := space(t, home).Worktree(); !errors.Is(err, errTargetExists) {
				t.Errorf("a file at %s: Worktree = %v, want errTargetExists", name, err)
			}
		}
	})
	t.Run("a link to a file in the way", func(t *testing.T) {
		home := spaceHome(t)
		file := filepath.Join(home, "file")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(file, filepath.Join(home, ".herdr")); err != nil {
			t.Fatal(err)
		}
		if _, err := space(t, home).Worktree(); !errors.Is(err, errTargetExists) {
			t.Errorf("Worktree = %v, want errTargetExists", err)
		}
	})
	t.Run("a link to an existing directory at the path", func(t *testing.T) {
		home := spaceHome(t)
		parent := filepath.Join(home, ".herdr/worktrees/yt2column")
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
		other := filepath.Join(home, "other")
		if err := os.Mkdir(other, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(other, filepath.Join(parent, "issue-7")); err != nil {
			t.Fatal(err)
		}
		if got, err := space(t, home).Worktree(); !errors.Is(err, errTargetExists) {
			t.Errorf("Worktree = %q, %v, want errTargetExists", got, err)
		}
	})
	t.Run("a link with no target at the path", func(t *testing.T) {
		home := spaceHome(t)
		parent := filepath.Join(home, ".herdr/worktrees/yt2column")
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(home, "nowhere"), filepath.Join(parent, "issue-7")); err != nil {
			t.Fatal(err)
		}
		if _, err := space(t, home).Worktree(); !errors.Is(err, errTargetExists) {
			t.Errorf("Worktree = %v, want errTargetExists", err)
		}
	})
	t.Run("the zero value returns an error and touches nothing", func(t *testing.T) {
		if _, err := (TaskSpace{}).Worktree(); !errors.Is(err, errNoTaskSpace) {
			t.Errorf("Worktree = %v, want errNoTaskSpace", err)
		}
	})
}

// TestTaskSpaceWorktreeMatchesCanonicalWorktree makes the worktree with git
// and checks that CanonicalWorktree reads back the path TaskSpace.Worktree
// returned before it existed.
func TestTaskSpaceWorktreeMatchesCanonicalWorktree(t *testing.T) {
	home := spaceHome(t)
	repo := newTestRepo(t)
	s, err := NewTaskSpace(home, issueNumber(t, "9"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := s.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	repo.Git("worktree", "add", "--detach", s.Path())
	got, err := CanonicalWorktree(t.Context(), repo.RealCommands(), s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("CanonicalWorktree = %q, want %q", got, want)
	}
}

func TestWorkspacesAtPath(t *testing.T) {
	tree := Worktree{path: "/home/u/.herdr/worktrees/yt2column/issue-7"}
	pane := func(workspace, dir string) listedPane {
		return listedPane{Workspace: workspace, Dir: dir}
	}
	cases := []struct {
		name  string
		panes []listedPane
		want  []string
	}{
		{"the path itself", []listedPane{pane("w1", tree.path)}, []string{"w1"}},
		{"below the path", []listedPane{pane("w1", tree.path+"/internal/chief")}, []string{"w1"}},
		{"outside the path", []listedPane{pane("w1", "/home/u/src")}, nil},
		{"a name that only starts like the path", []listedPane{pane("w1", tree.path+"0"), pane("w2", tree.path+"-b/x")}, nil},
		{"the parent of the path", []listedPane{pane("w1", "/home/u/.herdr/worktrees/yt2column")}, nil},
		{"a trailing slash or dots in the pane's directory", []listedPane{pane("w1", tree.path+"/./sub/..")}, []string{"w1"}},
		{"shell panes and agent panes count, each workspace once", []listedPane{
			pane("w3", tree.path),
			{Workspace: "w3", Dir: tree.path, HasAgent: true},
			pane("w4", tree.path+"/x"),
		}, []string{"w3", "w4"}},
		{"no panes", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := workspacesAtPath(c.panes, tree); !slices.Equal(got, c.want) {
				t.Errorf("workspacesAtPath = %v, want %v", got, c.want)
			}
		})
	}
	t.Run("the zero worktree matches nothing", func(t *testing.T) {
		if got := workspacesAtPath([]listedPane{pane("w1", "/")}, Worktree{}); got != nil {
			t.Errorf("workspacesAtPath = %v, want none", got)
		}
	})
	t.Run("a pane directory reached through a link", func(t *testing.T) {
		base := spaceHome(t)
		target := filepath.Join(base, "real")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(base, "link")); err != nil {
			t.Fatal(err)
		}
		got := workspacesAtPath([]listedPane{pane("w1", filepath.Join(base, "link"))}, Worktree{path: target})
		if !slices.Equal(got, []string{"w1"}) {
			t.Errorf("workspacesAtPath = %v, want [w1]", got)
		}
	})
}

// TestWorkspacesAtPathReadsTheSample finds the path's workspace in a real
// `herdr pane list` output: the sample's chief worktree holds a shell pane, an
// OpenCode pane, and a Claude Code pane.
func TestWorkspacesAtPathReadsTheSample(t *testing.T) {
	for _, name := range []string{"herdr/pane-list-agents.json", "herdr/pane-list-shells.json"} {
		panes, err := parsePaneList(readTestdata(t, name))
		if err != nil {
			t.Fatal(err)
		}
		if len(panes) == 0 {
			t.Fatalf("%s: no panes", name)
		}
		dir := panes[0].Dir
		got := workspacesAtPath(panes, Worktree{path: dir})
		if len(got) == 0 || !slices.Contains(got, panes[0].Workspace) {
			t.Errorf("%s: workspacesAtPath(%q) = %v, want %s among them", name, dir, got, panes[0].Workspace)
		}
		if other := workspacesAtPath(panes, Worktree{path: "/nowhere/" + strings.TrimPrefix(dir, "/")}); other != nil {
			t.Errorf("%s: a path nobody works in found %v", name, other)
		}
	}
}

// TestHerdrPanes reads the panes with `herdr pane list`, the one call that
// shows the shell-only panes `herdr agent list` omits.
func TestHerdrPanes(t *testing.T) {
	rec := newRecorder(t)
	rec.reply("herdr pane list", readTestdata(t, "herdr/pane-list-shells.json"))
	panes, err := NewHerdr(rec.commands(nil)).panes(t.Context())
	if err != nil || len(panes) == 0 {
		t.Fatalf("panes = %v, %v", panes, err)
	}
	for _, p := range panes {
		if p.HasAgent {
			t.Errorf("a shell-only sample has an agent pane: %+v", p)
		}
	}
	rec.fail("herdr pane list", errors.New("herdr failed"))
	if _, err := NewHerdr(rec.commands(nil)).panes(t.Context()); err == nil {
		t.Error("a failed herdr call is read as panes")
	}
}
