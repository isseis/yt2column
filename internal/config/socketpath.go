package config

import (
	"path/filepath"
	"slices"
)

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

// darwinAppSupportParts is macOS's per-user application support directory
// below HOME, which holds both the socket directory and the configuration
// file's directory.
var darwinAppSupportParts = []string{"Library", "Application Support"}

// The parts of the socket path below HOME. The socket directory holds only the
// socket and the start lock file.
var (
	darwinSocketDirParts = slices.Concat(darwinAppSupportParts, []string{appDirName, "run"})
	unixSocketDirParts   = []string{".local", "state", appDirName, "run"}
)

// socketFileName is the socket's name in the socket directory.
const socketFileName = "server.sock"

// osClass is how the path rules under HOME (the socket and the server's
// configuration file) tell operating systems apart. Its zero value,
// osNoRule, is a system with no rule, so an unclassified system derives no
// path.
type osClass int

const (
	// osNoRule is a system with no rule under HOME.
	osNoRule osClass = iota
	// osDarwin is macOS.
	osDarwin
	// osLinux is Linux.
	osLinux
	// osOtherUnix is any other Unix, which follows the Linux layout.
	osOtherUnix
)

// classifyOS returns goos's class. Both path rules switch on it, so they agree
// on which system is which; a class a rule does not handle derives no path.
func classifyOS(goos string) osClass {
	switch goos {
	case "darwin":
		return osDarwin
	case "linux":
		return osLinux
	case "windows", "plan9", "js", "wasip1":
		return osNoRule
	default:
		return osOtherUnix
	}
}

// socketRule returns the parts of the socket directory below HOME and the
// path limit for goos, and false for an operating system with no rule.
func socketRule(goos string) (parts []string, limit int, ok bool) {
	switch classifyOS(goos) {
	case osDarwin:
		return darwinSocketDirParts, darwinSocketPathMax, true
	case osLinux:
		return unixSocketDirParts, linuxSocketPathMax, true
	case osOtherUnix:
		return unixSocketDirParts, otherUnixSocketPathMax, true
	default:
		return nil, 0, false
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
