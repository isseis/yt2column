package fixprguard

import (
	"fmt"
	"path/filepath"
	"strings"
)

// WorktreeDir is the top of the work tree a Worktree guards. The zero value
// is not usable: NewWorktree panics on it.
type WorktreeDir struct{ rootDir string }

// CurrentDir is the process's current directory, as ".". The CLI passes it:
// a name joined with it is the name itself, so every path the tool opens is
// the one it opened before WorktreeDir existed.
func CurrentDir() WorktreeDir { return WorktreeDir{rootDir: "."} }

// NewWorktreeDir accepts an absolute path other than "/" that filepath.Clean
// leaves as it is, and fails with ErrInvalidInput otherwise. It does not touch
// the file system: whether a .git is there is checked where it is checked
// today.
func NewWorktreeDir(path string) (WorktreeDir, error) {
	if path == "" || path == "/" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return WorktreeDir{}, fmt.Errorf("%w: worktree directory", ErrInvalidInput)
	}
	return WorktreeDir{rootDir: path}, nil
}

func (d WorktreeDir) usable() bool { return d.rootDir != "" }

// joinName returns the OS path for a work-tree-relative name. It does not call
// filepath.Clean, so a trailing "/" keeps its OS meaning; for CurrentDir it
// returns name unchanged without prepending "./".
func (d WorktreeDir) joinName(name string) string {
	if d.rootDir == "." {
		return name
	}
	return d.rootDir + "/" + name
}

// resolveLocation returns the OS path for a location reported by git (such as
// a .git file's gitdir or rev-parse --git-path): an absolute path is returned
// as is, and a relative path is joined with d.
func (d WorktreeDir) resolveLocation(loc string) string {
	if filepath.IsAbs(loc) {
		return loc
	}
	return d.joinName(loc)
}

// relName recovers the work-tree-relative name from an OS path walked under d.
// For CurrentDir it returns osPath unchanged; otherwise osPath must begin with
// d's path followed by "/", and that prefix is stripped once.
func (d WorktreeDir) relName(osPath string) (string, bool) {
	if d.rootDir == "." {
		return osPath, true
	}
	return strings.CutPrefix(osPath, d.rootDir+"/")
}
