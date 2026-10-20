package config

import "path/filepath"

// The longest socket path, in bytes, each operating system accepts: the size
// of sockaddr_un's sun_path minus the terminating NUL. Linux's sun_path is 108
// bytes; macOS's and the BSDs' are 104, and any other Unix gets that shorter
// limit too, so a path it accepts never fails at bind for its length.
// internal/config cannot import net or the OS-specific syscall types, so the
// values are constants; the internal/serversock tests check them against
// syscall.RawSockaddrUnix.
const (
	darwinSocketPathMax    = 103
	linuxSocketPathMax     = 107
	otherUnixSocketPathMax = 103
)

// The parts of the socket path below HOME. The socket directory holds only the
// socket and the start lock file.
var (
	darwinSocketDirParts = []string{"Library", "Application Support", appDirName, "run"}
	unixSocketDirParts   = []string{".local", "state", appDirName, "run"}
)

// socketFileName is the socket's name in the socket directory.
const socketFileName = "server.sock"

// socketRule returns the parts of the socket directory below HOME and the
// path limit for goos, and false for an operating system with no rule.
func socketRule(goos string) (parts []string, limit int, ok bool) {
	switch goos {
	case "darwin":
		return darwinSocketDirParts, darwinSocketPathMax, true
	case "linux":
		return unixSocketDirParts, linuxSocketPathMax, true
	case "windows", "plan9", "js", "wasip1":
		return nil, 0, false
	default:
		return unixSocketDirParts, otherUnixSocketPathMax, true
	}
}

// socketPath derives the server's socket path for goos from HOME alone, or
// returns a *VarError naming HOME. Contract: the result depends only on goos
// and HOME; no other variable (XDG_STATE_HOME, XDG_RUNTIME_DIR, TMPDIR, ...)
// is read, because the browser starts the host without the user's shell
// environment and both must derive the same path. The result is absolute and
// within the OS limit; the error never holds HOME's value.
func socketPath(goos string, lookup LookupFunc) (string, *VarError) {
	parts, limit, ok := socketRule(goos)
	if !ok {
		return "", missingVarWithReason("HOME", reasonSocketPathAbsent)
	}
	home, ok := absoluteHome(lookup)
	if !ok {
		return "", missingVarWithReason("HOME", reasonSocketPathAbsent)
	}
	path := filepath.Join(append(append([]string{home}, parts...), socketFileName)...)
	if len(path) > limit {
		return "", invalidVar("HOME", reasonSocketPathTooLong)
	}
	return path, nil
}
