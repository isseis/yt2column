//go:build !js

package config

import (
	"errors"
	"runtime"
)

// The server and host configuration cannot build for js/wasm: it reads the
// configuration file through internal/privatefile, which has no js build.
// The article generation module needs internal/config for LoadExtension only,
// so everything the server and the host read lives behind this build tag. The
// removal of the server and the host removes this file, and then the tag
// disappears with it.

// ServerConfig is the server configuration. Its fields are unexported, so a
// non-zero ServerConfig can only come from LoadServer. The LLM settings come
// from the six keys of the configuration file, never from the environment.
// LoadServer never reads or checks YT2COLUMN_CACHE_DIR, YT2COLUMN_YTDLP_PATH,
// or the cache directory defaults.
type ServerConfig struct {
	llm               LLMConfig
	socketPath        string // absolute, within the OS limit
	http2DebugEnabled bool
}

// LLM returns the validated LLM configuration.
func (c ServerConfig) LLM() LLMConfig {
	return c.llm
}

// SocketPath returns the absolute path of the server's Unix domain socket,
// derived from HOME alone. It is within the OS limit.
func (c ServerConfig) SocketPath() string {
	return c.socketPath
}

// HTTP2DebugEnabled reports whether GODEBUG contains http2debug=1 or
// http2debug=2. LoadServer never rejects this setting; the caller warns instead.
func (c ServerConfig) HTTP2DebugEnabled() bool {
	return c.http2DebugEnabled
}

// LoadServer reads the server configuration. It derives the configuration
// file's path from HOME, reads and parses the file, and validates the six
// keys from it; HOME (the socket path) and GODEBUG (the warning) come from
// lookup.
// It reads no other variable. It returns the zero ServerConfig on any error,
// and stops at the first stage that fails: HOME, the file, its format, then
// the values.
//
// Contract: a key in the process environment never reaches the
// configuration, since the value loaders shared with Load read the file's
// lookup and only the socket path and the GODEBUG check read lookup. The
// rejections of one stage are joined flat, one error per line of the
// server's output: the format rejections, or the value rejections, each of a
// key read from the file marked VarSourceConfigFile.
func LoadServer(lookup LookupFunc) (ServerConfig, error) {
	loc, homeErr := serverFileLocationFor(runtime.GOOS, lookup)
	if homeErr != nil {
		return ServerConfig{}, homeErr
	}
	file, err := readServerFile(loc.path)
	if err != nil {
		return ServerConfig{}, err
	}

	llm, errs := loadLLM(file.lookup, Need{Generation: true, Review: true})
	for i, e := range errs {
		if varErr, ok := errors.AsType[*VarError](e); ok {
			errs[i] = file.keyError(varErr)
		}
	}
	sockPath, sockErr := socketPath(runtime.GOOS, lookup)
	if sockErr != nil {
		// socketPath's *VarError is a typed nil on success, which errors.Join
		// would keep as a non-nil error.
		errs = append(errs, sockErr)
	}
	if err := errors.Join(errs...); err != nil {
		return ServerConfig{}, err
	}
	return ServerConfig{
		llm:               llm,
		socketPath:        sockPath,
		http2DebugEnabled: HTTP2DebugRequested(lookup),
	}, nil
}

// HostConfig is the relay configuration of the Native Messaging host and of the
// development server (cmd/yt2column-devserver), which needs the server's socket
// path and nothing else. Its field is unexported, so a non-zero HostConfig can
// only come from LoadHost.
type HostConfig struct {
	socketPath string // absolute, within the OS limit
}

// SocketPath returns the absolute path of the server's Unix domain socket,
// derived from HOME alone with the same rule as ServerConfig.SocketPath.
func (c HostConfig) SocketPath() string {
	return c.socketPath
}

// LoadHost reads HOME, and nothing else, through lookup. Contract: it derives
// the socket path with the same function as LoadServer, so a host the browser
// starts and a server started from a shell reach one socket whenever they see
// the same HOME. The development server is the second caller; that it reads
// HOME only is what keeps it from reading the server's configuration file or
// any secret, so a change that makes LoadHost read another variable changes
// what both entry points read. It returns a *VarError naming HOME, and the
// zero HostConfig, when HOME is unusable or the derived path exceeds the OS
// limit.
func LoadHost(lookup LookupFunc) (HostConfig, error) {
	sockPath, err := socketPath(runtime.GOOS, lookup)
	if err != nil {
		return HostConfig{}, err
	}
	return HostConfig{socketPath: sockPath}, nil
}

// LoadHostHome reads HOME, and nothing else, through lookup, and returns it
// when it is set and absolute. install and uninstall derive the browsers'
// manifest directories from it. Contract: it never checks the socket path, so
// a user whose socket path is too long can still remove the manifest. It
// returns a *VarError naming HOME when HOME is unset, empty, or relative.
func LoadHostHome(lookup LookupFunc) (string, error) {
	home, ok := absoluteHome(lookup)
	if !ok {
		return "", missingVarWithReason("HOME", reasonHomeAbsent)
	}
	return home, nil
}
