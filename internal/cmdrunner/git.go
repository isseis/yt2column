package cmdrunner

import "strings"

// GitHardening returns the global options that come before every git
// subcommand a developer tool runs. They read objects as stored, never through
// refs/replace, so every tree a tool compares is the real one. They also turn
// off configuration that would launch a command during a read-only operation:
// the fsmonitor hook, a configured SSH command, the ext transport, and
// automatic gc and maintenance (which run hooks of their own). A pager and
// colors are off so output is parsed as git wrote it.
//
// git 2.12 or later is required: it is the first to read
// protocol.<name>.allow, without which protocol.ext.allow=never is silently
// ignored.
//
// Each call returns a new slice, so a caller that appends to or edits the
// result cannot change what the next caller gets.
func GitHardening() []string {
	return []string{
		"--no-pager",
		"--no-replace-objects",
		"-c", "core.fsmonitor=false",
		"-c", "core.sshCommand=ssh",
		"-c", "protocol.ext.allow=never",
		"-c", "gc.auto=0",
		"-c", "maintenance.auto=false",
		"-c", "color.ui=false",
	}
}

// GitEnv returns environ without any inherited GIT_* variable, which could
// point git at another repository, index or configuration, and with optional
// locks and terminal prompts turned off. A runner that starts git, or a
// command that starts git itself, runs with this environment.
func GitEnv(environ []string) []string {
	added := []string{"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0"}
	env := make([]string, 0, len(environ)+len(added))
	for _, entry := range environ {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return append(env, added...)
}
