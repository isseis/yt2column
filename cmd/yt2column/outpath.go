package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
// cannot exist because a component is not a directory has the part below that
// component compared by name, like a non-existent part. Existing directories
// are compared by file identity, walking the hierarchy rather than the text of
// the path, so a descendant reached through a bind mount of an ancestor is
// recognized. A bind mount of a cache subdirectory at an unrelated path cannot
// be reached by walking up; the cache's own descendants are scanned for it.
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
// absolute one. A component that is not a directory ends the prefix: nothing
// can exist below it, so the rest is treated as non-existent. It reports
// ok=false when the prefix cannot be resolved safely, so the caller treats the
// path as inside the cache directory.
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
			if err := confirmSearchable(current); err != nil {
				return "", nil, false
			}
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
			resolvedInfo, err := os.Stat(resolved)
			if err != nil {
				if !pathCannotExist(err) {
					return "", nil, false
				}
				return restComponents(current, components[i:])
			}
			if !resolvedInfo.IsDir() {
				return restComponents(current, components[i:])
			}
			current = resolved
			continue
		}
		if !info.IsDir() {
			return restComponents(current, components[i:])
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

// confirmSearchable reports whether dir can be searched by the caller.
// Resolving "dir/." needs the search permission on dir, just as the kernel
// needs it to apply a following "..", so a failure means the path cannot be
// resolved the way the kernel would and the caller must fail closed.
func confirmSearchable(dir string) error {
	_, err := os.Stat(dir + string(os.PathSeparator) + ".")
	return err
}

// insideResolved reports whether the path (dir plus rest) is inside the cache
// directory (cacheDir plus cacheRest). The existing parts are compared by file
// identity, walking the directory hierarchy rather than the text of the path,
// so a descendant reached through an alias such as a bind mount is still
// recognized; the non-existent parts are compared by name, without case.
func insideResolved(dir string, rest []string, cacheDir string, cacheRest []string) bool {
	switch {
	case sameDir(dir, cacheDir):
		return isComponentPrefix(cacheRest, rest)
	case isAncestor(cacheDir, dir):
		// dir is a real directory inside cacheDir. The full cache path is
		// cacheDir plus its missing tail, so dir is inside it when that tail is
		// a prefix of the path from cacheDir down to dir. The prefix is
		// compared without case, like the non-existent parts.
		relComponents, ok := pathFrom(cacheDir, dir)
		if !ok {
			return true
		}
		return isComponentPrefix(cacheRest, relComponents)
	case isAncestor(dir, cacheDir):
		// cacheDir is inside dir; the path from dir to cacheDir is compared by
		// name, then the cache's missing tail.
		relComponents, ok := pathFrom(dir, cacheDir)
		if !ok {
			return true
		}
		if !isComponentPrefix(relComponents, rest) {
			return false
		}
		return isComponentPrefix(cacheRest, rest[len(relComponents):])
	default:
		// Neither directory contains the other by walking up the paths. The
		// output may still sit on a cache descendant reached through an alias
		// such as a bind mount of a cache subdirectory at an unrelated path,
		// which the walk cannot see; compare the cache's own descendants too.
		return aliasesCacheDescendant(dir, cacheDir)
	}
}

// maxCacheDescendantDepth bounds the cache-descendant scan. The cache layout is
// shallow (a slot directory per video), and the bound also stops a bind mount
// that makes the tree cyclic; reaching it makes the answer unknown, and the
// caller then stays fail-closed.
const maxCacheDescendantDepth = 4

// aliasesCacheDescendant reports whether dir is an existing descendant of
// cacheDir, or lies below one, reached through an alias such as a bind mount of
// a cache subdirectory placed at an unrelated path. Walking up from dir cannot
// see such an alias, so the cache's own entries are compared by file identity
// instead. The walk is bounded in depth; when it cannot finish safely it reports
// true so the caller stays fail-closed.
func aliasesCacheDescendant(dir, cacheDir string) bool {
	info, err := os.Stat(cacheDir)
	if err != nil {
		return true
	}
	if !info.IsDir() {
		return false
	}
	var walk func(current string, depth int) (bool, bool)
	walk = func(current string, depth int) (bool, bool) {
		if depth > maxCacheDescendantDepth {
			return false, false
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			return false, false
		}
		for _, entry := range entries {
			child := filepath.Join(current, entry.Name())
			if isAncestor(child, dir) {
				return true, true
			}
			if entry.IsDir() {
				if found, ok := walk(child, depth+1); found || !ok {
					return found, ok
				}
			}
		}
		return false, true
	}
	found, ok := walk(cacheDir, 0)
	if !ok {
		return true
	}
	return found
}

// sameDir reports whether a and b name the same directory. os.SameFile
// recognizes an alias such as a bind mount or a case-insensitive spelling.
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

// isAncestor reports whether ancestor is dir or a directory above dir. It walks
// up by file identity, so a directory reached through an alias is recognized
// even though its path does not have ancestor as a textual prefix.
func isAncestor(ancestor, dir string) bool {
	for d := dir; ; {
		if sameDir(d, ancestor) {
			return true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return false
		}
		d = parent
	}
}

// pathFrom returns the components from ancestor down to descendant when
// ancestor is a directory above descendant, walking up by file identity so an
// alias is recognized. The names are taken from descendant's path.
func pathFrom(ancestor, descendant string) ([]string, bool) {
	var reversed []string
	for d := descendant; ; {
		if sameDir(d, ancestor) {
			slices.Reverse(reversed)
			return reversed, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return nil, false
		}
		reversed = append(reversed, filepath.Base(d))
		d = parent
	}
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
