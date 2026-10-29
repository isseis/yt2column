package codexreview

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// locateFindings turns the findings of a final message into Findings relative
// to root, which must be absolute with its symbolic links resolved. changed
// holds the paths, relative to root with forward slashes, that BASE..HEAD
// changed.
//
// Contract: a path is compared with root after the symbolic links of its
// directories are resolved, so /var and /private/var name the same file. A
// finding whose path is not under root makes the whole message unreadable
// (false, and no findings): codex reviewed this worktree, so such a path means
// the output is not what the grammar describes. InRange is true only when the
// path changed in BASE..HEAD, is a regular file, and holds the end line.
func locateFindings(root string, changed map[string]struct{}, raw []rawFinding) ([]Finding, bool) {
	findings := make([]Finding, 0, len(raw))
	for _, f := range raw {
		rel, ok := relativeToRoot(root, f.path)
		if !ok {
			return nil, false
		}
		_, inDiff := changed[rel]
		findings = append(findings, Finding{
			Path:    rel,
			Start:   f.start,
			End:     f.end,
			InRange: inDiff && holdsLine(filepath.Join(root, filepath.FromSlash(rel)), f.end),
			Title:   f.title,
			Body:    f.body,
		})
	}
	return findings, true
}

// relativeToRoot returns path relative to root, with forward slashes, when it
// names something strictly under root. A relative path never does: Rel
// refuses to relate it to the absolute root.
func relativeToRoot(root, path string) (string, bool) {
	resolved := resolveDirectories(filepath.Clean(path))
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// resolveDirectories resolves the symbolic links of the longest existing
// directory above path and keeps the rest as written. The last element is
// never resolved, so a finding on a tracked symbolic link keeps the name git
// lists, and a deleted file still has a path.
func resolveDirectories(path string) string {
	dir, rest := filepath.Dir(path), filepath.Base(path)
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

// holdsLine reports whether path is a regular file with at least line lines.
func holdsLine(path string, line int) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(path) //nolint:gosec // the path is under the worktree root, checked by relativeToRoot
	if err != nil {
		return false
	}
	lines := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lines++
	}
	return line <= lines
}
