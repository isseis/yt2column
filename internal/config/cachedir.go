package config

import "path/filepath"

// defaultCacheDir returns the default YT2COLUMN_CACHE_DIR for goos, derived
// from the values lookup returns, and whether one could be derived. It
// follows the same rules as os.UserCacheDir for the supported operating
// systems, but also rejects a relative result: a relative cache directory would
// change what the cache pruning and removal act on with the current directory.
func defaultCacheDir(goos string, lookup LookupFunc) (string, bool) {
	switch goos {
	case "darwin":
		home, ok := absoluteEnv(lookup, "HOME")
		if !ok {
			return "", false
		}
		return filepath.Join(home, "Library", "Caches", "yt2column"), true
	case "windows", "plan9", "js", "wasip1":
		return "", false
	default:
		if xdg, ok := lookup("XDG_CACHE_HOME"); ok && xdg != "" {
			if !filepath.IsAbs(xdg) {
				return "", false
			}
			return filepath.Join(xdg, "yt2column"), true
		}
		home, ok := absoluteEnv(lookup, "HOME")
		if !ok {
			return "", false
		}
		return filepath.Join(home, ".cache", "yt2column"), true
	}
}

// absoluteEnv returns the absolute value of the named variable. An unset,
// empty, or relative value reports false.
func absoluteEnv(lookup LookupFunc, name string) (string, bool) {
	value, ok := lookup(name)
	if !ok || value == "" || !filepath.IsAbs(value) {
		return "", false
	}
	return value, true
}
