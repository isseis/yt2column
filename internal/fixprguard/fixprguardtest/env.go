//go:build test

// Package fixprguardtest holds the test-only helpers that internal/fixprguard
// and cmd/fixprguard share: the isolation of the test binary's working
// directory and the environment a test's git runs with. It must not import
// internal/fixprguard, so the package under test can import it.
package fixprguardtest

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// overridden is the number of variables Env sets: HOME, XDG_CONFIG_HOME, PWD.
const overridden = 3

// Env returns cmdrunner.GitEnv(os.Environ()) with HOME and XDG_CONFIG_HOME
// pointing under home and PWD set to dir, the Dir of the runner it is for.
// GitEnv drops every GIT_* variable, so the user's git configuration cannot
// be shut out with GIT_CONFIG_GLOBAL; replacing HOME shuts it out instead.
// The system configuration is not shut out, as before this helper existed.
func Env(home, dir string) []string {
	return withTestValues(cmdrunner.GitEnv(os.Environ()), home, dir)
}

// withTestValues drops HOME, XDG_CONFIG_HOME, and PWD from base and appends
// the per-test values.
func withTestValues(base []string, home, dir string) []string {
	env := make([]string, 0, len(base)+overridden)
	for _, entry := range base {
		if hasName(entry, "HOME") || hasName(entry, "XDG_CONFIG_HOME") || hasName(entry, "PWD") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "PWD="+dir)
}

// hasName reports whether the environment entry sets the variable name.
func hasName(entry, name string) bool {
	return strings.HasPrefix(entry, name+"=")
}
