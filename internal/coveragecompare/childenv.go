package coveragecompare

import (
	"slices"
	"strings"
)

// childEnvNames are the variables of the user's environment that the go and
// node commands receive, when the user's environment has them: what they need
// to find programs and the user's caches, and to read text.
var childEnvNames = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "TZ",
	"GOCACHE", "GOMODCACHE", "GOPATH",
}

// childEnvPrefix passes the locale categories (LC_ALL, LC_CTYPE, ...) too.
const childEnvPrefix = "LC_"

// childEnv returns the environment of every go and node command the tool
// runs (the tests, go env, node --version, npm --version, and the range
// helper). tmp is the temporary directory under the work directory.
//
// Contract: nothing else of the user's environment passes, so neither point
// can run only part of its suite while still exiting 0: not GOFLAGS or a go
// env -w setting (GOENV=off), not NODE_OPTIONS (--test-name-pattern,
// --test-only) or any other NODE_* or GIT_* variable, and no secret such as an
// API key. The variables the tool sets keep go from fetching a toolchain or a
// module (GOTOOLCHAIN=local, GOPROXY=off) and put every temporary file under
// the work directory, which the run removes.
func childEnv(environ []string, tmp string) []string {
	var env []string
	for _, entry := range environ {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if slices.Contains(childEnvNames, name) || strings.HasPrefix(name, childEnvPrefix) {
			env = append(env, entry)
		}
	}
	return append(env,
		"TMPDIR="+tmp,
		"GOTMPDIR="+tmp,
		"GOTOOLCHAIN=local",
		"GOENV=off",
		"GOFLAGS=",
		"GOPROXY=off",
		"GOWORK=off",
	)
}
