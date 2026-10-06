package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// outPathInsideCacheDir reports whether outPath names the cache directory or
// something inside it. The cache directory's contents are pruned and
// the finished video's cache is removed during a run, so an --out path inside
// it could be deleted without telling the user; the CLI refuses it.
//
// The check is fail-closed: whenever the two paths cannot be compared safely (a
// permission error, a symbolic-link loop, or ".." below a component that does
// not exist), it reports true so the caller refuses the path. A path that
// cannot exist because a component is not a directory is compared by name and
// is normally outside, matching what the kernel would resolve.
func outPathInsideCacheDir(outPath, cacheDir string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}

	outDir, outRest, ok := resolveExisting(outPath, cwd)
	if !ok {
		return true
	}
	cacheResolved, cacheRest, ok := resolveExisting(cacheDir, cwd)
	if !ok {
		return true
	}
	return insideResolved(outDir, outRest, cacheResolved, cacheRest)
}

// resolveExisting splits path into the longest existing prefix and the
// components below it. The prefix is made absolute and symbolic links in it are
// resolved in the kernel's order (a link is resolved before a following ".."
// applies), starting at base for a relative path and at the root for an
// absolute one. It reports ok=false when the prefix cannot be resolved safely,
// so the caller treats the path as inside the cache directory.
func resolveExisting(path, base string) (string, []string, bool) {
	current := base
	components := splitComponents(path)
	if filepath.IsAbs(path) {
		current = string(os.PathSeparator)
	}
	for i, component := range components {
		switch component {
		case "", ".":
			continue
		case "..":
			current = filepath.Dir(current)
			continue
		}
		next := filepath.Join(current, component)
		info, err := os.Lstat(next)
		if err != nil {
			if !pathCannotExist(err) {
				return "", nil, false
			}
			return restComponents(current, components[i:])
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(next)
			if err != nil {
				if !pathCannotExist(err) {
					return "", nil, false
				}
				return restComponents(current, components[i:])
			}
			current = resolved
			continue
		}
		current = next
	}
	return current, nil, true
}

// restComponents returns the existing directory and the remaining non-existent
// components. A ".." among them cannot be resolved the way the kernel would, so
// it makes the path unsafe to check.
func restComponents(dir string, components []string) (string, []string, bool) {
	var rest []string
	for _, component := range components {
		switch component {
		case "", ".":
			continue
		case "..":
			return "", nil, false
		default:
			rest = append(rest, component)
		}
	}
	return dir, rest, true
}

// pathCannotExist reports whether err means the path has no meaning on disk: a
// missing component, or a non-directory used as one. The other errors (a
// permission failure, a symbolic-link loop) leave the path unresolved.
func pathCannotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// insideResolved reports whether the path (dir plus rest) is inside the cache
// directory (cacheDir plus cacheRest). The existing parts are compared by file
// identity and by path, and the non-existent parts by name, without case.
func insideResolved(dir string, rest []string, cacheDir string, cacheRest []string) bool {
	switch {
	case sameDir(dir, cacheDir):
		return isComponentPrefix(cacheRest, rest)
	case isWithin(dir, cacheDir):
		return true
	case isWithin(cacheDir, dir):
		rel, err := filepath.Rel(dir, cacheDir)
		if err != nil {
			return true
		}
		return isComponentPrefix(splitComponents(rel), rest)
	default:
		return false
	}
}

// sameDir reports whether a and b name the same directory. The paths are
// already resolved, so os.SameFile also catches an alias like a bind mount.
func sameDir(a, b string) bool {
	if a == b {
		return true
	}
	aInfo, err := os.Stat(a)
	if err != nil {
		return false
	}
	bInfo, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(aInfo, bInfo)
}

// isWithin reports whether path is strictly inside dir.
func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// isComponentPrefix reports whether prefix is a prefix of components, comparing
// names without case. An empty prefix is always a prefix.
func isComponentPrefix(prefix, components []string) bool {
	if len(prefix) > len(components) {
		return false
	}
	for i, p := range prefix {
		if !strings.EqualFold(p, components[i]) {
			return false
		}
	}
	return true
}

// splitComponents splits a slash path into its components, keeping "" and "..".
func splitComponents(path string) []string {
	return strings.Split(filepath.ToSlash(path), "/")
}
