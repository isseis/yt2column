package config

import "path/filepath"

// appDirName is the directory the application's cache and configuration live
// in under the operating system's user directory.
const appDirName = "yt2column"

// tokenFileName is the shared token file's name under the configuration
// directory.
const tokenFileName = "server-token"

// dirOverride is the OS-specific part of a user-directory rule: the
// subdirectory of HOME macOS uses, the XDG variable the other Unix systems
// prefer, the subdirectory of HOME used when that variable is unset, and the
// reason to report when HOME cannot provide an absolute base.
type dirOverride struct {
	darwin     []string
	xdgEnv     string
	xdgDefault []string
	absent     string
}

// cacheDirOverride is the rule for the cache directory (os.UserCacheDir).
var cacheDirOverride = dirOverride{
	darwin:     []string{"Library", "Caches"},
	xdgEnv:     "XDG_CACHE_HOME",
	xdgDefault: []string{".cache"},
	absent:     reasonCacheDirAbsent,
}

// configDirOverride is the rule for the configuration directory
// (os.UserConfigDir). On macOS the configuration lives in Library/Application
// Support, matching the cache's choice of the OS directory rather than the XDG
// fallback.
var configDirOverride = dirOverride{
	darwin:     []string{"Library", "Application Support"},
	xdgEnv:     "XDG_CONFIG_HOME",
	xdgDefault: []string{".config"},
	absent:     reasonConfigDirAbsent,
}

// defaultCacheDir returns the default YT2COLUMN_CACHE_DIR for goos, derived
// from the values lookup returns, and whether one could be derived. It follows
// the same rules as os.UserCacheDir for the supported operating systems, but
// also rejects a relative result: a relative cache directory would change what
// the cache pruning and removal act on with the current directory.
func defaultCacheDir(goos string, lookup LookupFunc) (string, bool) {
	base, err := userDirBase(goos, lookup, cacheDirOverride)
	if err != nil {
		return "", false
	}
	return filepath.Join(base, appDirName), true
}

// defaultConfigDir returns the default configuration directory for goos,
// derived from the values lookup returns, or a *VarError naming the variable
// that prevented the derivation. It follows the same rules as os.UserConfigDir
// for the supported operating systems and rejects a relative result.
func defaultConfigDir(goos string, lookup LookupFunc) (string, *VarError) {
	base, err := userDirBase(goos, lookup, configDirOverride)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// userDirBase returns the base directory of the user-directory rule o for goos,
// under which the application directory is placed, or a *VarError naming the
// variable that prevented the derivation. macOS uses the darwin subdirectory of
// HOME; the other Unix systems prefer the XDG variable when it is set and
// absolute, and fall back to the subdirectory of HOME; the remaining systems
// have no rule and report HOME so the caller names a variable.
func userDirBase(goos string, lookup LookupFunc, o dirOverride) (string, *VarError) {
	switch goos {
	case "darwin":
		home, ok := absoluteEnv(lookup, "HOME")
		if !ok {
			return "", missingVarWithReason("HOME", o.absent)
		}
		return filepath.Join(append([]string{home}, o.darwin...)...), nil
	case "windows", "plan9", "js", "wasip1":
		return "", missingVarWithReason("HOME", o.absent)
	default:
		if xdg, ok := lookup(o.xdgEnv); ok && xdg != "" {
			if !filepath.IsAbs(xdg) {
				return "", invalidVar(o.xdgEnv, reasonDirRelative)
			}
			return xdg, nil
		}
		home, ok := absoluteEnv(lookup, "HOME")
		if !ok {
			return "", missingVarWithReason("HOME", o.absent)
		}
		return filepath.Join(append([]string{home}, o.xdgDefault...)...), nil
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
