package chief

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// Errors of the place a task is made in. A caller turns each into the
// refusal that names it; the sentinels are what it tells them apart by.
var (
	// errHomeNotAbsolute reports a home directory that is not an absolute path.
	errHomeNotAbsolute = errors.New("the home directory is not an absolute path")
	// errNoIssue reports a task space asked for with no issue number.
	errNoIssue = errors.New("no issue number")
	// errTargetExists reports a path that something other than a directory
	// already holds, on the way to the worktree or at it.
	errTargetExists = errors.New("something is already at the path")
	// errNoTaskSpace reports a method called on the zero TaskSpace.
	errNoTaskSpace = errors.New("no task space")
)

// taskSpaceRoot is the directory below $HOME that holds the worktrees chief
// makes: the repository's name is a value the requirements fix, not one read
// from the remote.
var taskSpaceRoot = []string{".herdr", "worktrees", "yt2column"}

// taskLabelPrefix starts the label of a task's workspace and the name of its
// worktree's directory.
const taskLabelPrefix = "issue-"

// TaskSpace is where chief makes a task's worktree and workspace for one
// issue: the path $HOME/.herdr/worktrees/yt2column/issue-<N> and the label
// issue-<N>. Only NewTaskSpace makes one, from an issue number and the home
// directory, so no caller string reaches either value. The zero value makes
// nothing: its methods return an error without touching the file system.
type TaskSpace struct {
	issue paneinput.IssueNumber
	// path is as built from $HOME, before links are resolved.
	path  string
	label string
}

// NewTaskSpace returns the space of issue under home, which must be an
// absolute path. Contract: the built path holds only the characters a
// transcript directory can be decided for (transcriptDirChars), or it is
// refused with errUnsupportedPath, so a space that cannot receive the agents'
// transcripts is never made.
func NewTaskSpace(home string, issue paneinput.IssueNumber) (TaskSpace, error) {
	if issue.String() == "" {
		return TaskSpace{}, errNoIssue
	}
	if !filepath.IsAbs(home) {
		return TaskSpace{}, fmt.Errorf("%w: %q", errHomeNotAbsolute, home)
	}
	label := taskLabelPrefix + issue.String()
	path := filepath.Join(append([]string{home}, append(slices.Clone(taskSpaceRoot), label)...)...)
	if !transcriptDirChars.MatchString(path) {
		return TaskSpace{}, fmt.Errorf("%w: %q", errUnsupportedPath, path)
	}
	return TaskSpace{issue: issue, path: path, label: label}, nil
}

// Issue returns the issue the space is for.
func (s TaskSpace) Issue() paneinput.IssueNumber { return s.issue }

// Path returns the path as built from $HOME, before links are resolved.
func (s TaskSpace) Path() string { return s.path }

// Label returns the label of the task's workspace.
func (s TaskSpace) Label() string { return s.label }

// Worktree returns the canonical path the worktree will have: the nearest
// existing ancestor with its links resolved, joined with the rest of the
// path. Contract: once the worktree exists, CanonicalWorktree on it returns
// the same Worktree, or the creation stops at that stage. A path that holds a
// character outside transcriptDirChars after the links are resolved is
// refused with errUnsupportedPath, and an existing ancestor that is not a
// directory with errTargetExists.
func (s TaskSpace) Worktree() (Worktree, error) {
	if s.path == "" {
		return Worktree{}, errNoTaskSpace
	}
	ancestor, rest, err := nearestExisting(s.path)
	if err != nil {
		return Worktree{}, err
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Lstat found the entry, so it is a link with no target.
			return Worktree{}, fmt.Errorf("%w: %q", errTargetExists, ancestor)
		}
		return Worktree{}, err
	}
	if len(rest) > 0 {
		info, err := os.Stat(resolved)
		if err != nil {
			return Worktree{}, err
		}
		if !info.IsDir() {
			return Worktree{}, fmt.Errorf("%w: %q is not a directory", errTargetExists, ancestor)
		}
	}
	canonical := filepath.Join(append([]string{resolved}, rest...)...)
	if !transcriptDirChars.MatchString(canonical) {
		return Worktree{}, fmt.Errorf("%w: %q", errUnsupportedPath, canonical)
	}
	return Worktree{path: canonical}, nil
}

// nearestExisting returns the deepest ancestor of path (path itself
// included) that exists, without following a link at its last element, and
// the elements below it.
func nearestExisting(path string) (string, []string, error) {
	var rest []string
	for current := path; ; current = filepath.Dir(current) {
		_, err := os.Lstat(current)
		if err == nil {
			slices.Reverse(rest)
			return current, rest, nil
		}
		// ENOTDIR: a part of the path above is a file, so nothing lies below it.
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			return "", nil, err
		}
		if filepath.Dir(current) == current {
			return "", nil, fmt.Errorf("%w: no part of %q exists", errTargetExists, path)
		}
		rest = append(rest, filepath.Base(current))
	}
}

// workspacesAtPath returns the IDs of the workspaces that have a pane whose
// working directory is tree or lies below it, in the order of their first
// pane. Every pane counts: chief's own workspace and a workspace with only
// shell panes are workspaces of the path too. A directory that only shares a
// prefix with the path (the path followed by more name characters) is not
// below it. A pane directory is also compared with its links resolved, so a
// directory reached through a link still counts; one that cannot be resolved
// is compared as reported.
func workspacesAtPath(panes []listedPane, tree Worktree) []string {
	if tree.path == "" {
		return nil
	}
	var ids []string
	for _, p := range panes {
		if slices.Contains(ids, p.Workspace) {
			continue
		}
		if dirWithin(p.Dir, tree.path) {
			ids = append(ids, p.Workspace)
			continue
		}
		if resolved, err := filepath.EvalSymlinks(p.Dir); err == nil && dirWithin(resolved, tree.path) {
			ids = append(ids, p.Workspace)
		}
	}
	return ids
}

// dirWithin reports whether dir is root or lies below it.
func dirWithin(dir, root string) bool {
	dir = filepath.Clean(dir)
	return dir == root || strings.HasPrefix(dir, strings.TrimSuffix(root, "/")+"/")
}

// panes returns every pane `herdr pane list` reports, with or without an
// agent.
func (h Herdr) panes(ctx context.Context) ([]listedPane, error) {
	out, err := h.cmds.herdr(ctx, "pane", "list")
	if err != nil {
		return nil, err
	}
	return parsePaneList(out)
}
