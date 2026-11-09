package config

import (
	"path/filepath"
	"slices"
)

// serverFileName is the configuration file's name in its directory.
const serverFileName = "config.env"

// serverFileDirs are the directories of the server configuration file's rule
// below HOME, which config init creates when they are missing. HOME is the
// base, which must already exist. The rule is the same on macOS and the other
// Unix systems: the user edits the file by hand from a terminal, so one path
// without a space serves every system.
var serverFileDirs = []string{".config", appDirName}

// serverFileLocation is where the server's configuration file lives. Only
// serverFileLocationFor builds one, from HOME alone. It is unexported: the
// callers outside the package receive only the path (InitServerFile) or a
// test accessor.
type serverFileLocation struct {
	path string   // base + dirs + file name; absolute and clean
	base string   // must already exist for config init
	dirs []string // created below base by config init when missing
}

// serverFileLocationFor derives the configuration file's location for goos
// from HOME alone, or returns a *VarError naming HOME. Contract: the result
// depends only on goos and HOME; no other variable (XDG_CONFIG_HOME, TMPDIR,
// ...) is read, so the server started by a service manager and config init
// run from a shell find the same file. The error never holds HOME's value.
// Unlike the socket path, no length limit applies.
func serverFileLocationFor(goos string, lookup LookupFunc) (serverFileLocation, *VarError) {
	switch classifyOS(goos) {
	case osDarwin, osLinux, osOtherUnix:
	default:
		return serverFileLocation{}, missingVarWithReason("HOME", reasonConfigPathAbsent)
	}
	home, ok := absoluteHome(lookup)
	if !ok {
		return serverFileLocation{}, missingVarWithReason("HOME", reasonConfigPathAbsent)
	}
	base := filepath.Clean(home)
	return serverFileLocation{
		path: filepath.Join(slices.Concat([]string{base}, serverFileDirs, []string{serverFileName})...),
		base: base,
		dirs: slices.Clone(serverFileDirs),
	}, nil
}
