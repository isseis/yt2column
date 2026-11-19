package mergepr

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// NewOSRunner returns the production Runner: the shared runner with its
// default, unbounded options.
func NewOSRunner() Runner { return cmdrunner.NewOS(cmdrunner.Options{}) }

// gitEnviron returns the environment the tool's own git runs with: the
// environment of this process at the time of the call minus every inherited
// GIT_* variable (cmdrunner.GitEnv), so a GIT_DIR or GIT_CONFIG_* set by the
// caller cannot point a check at another repository or configuration.
func gitEnviron() []string {
	return cmdrunner.GitEnv(os.Environ())
}

// newGitRunner returns the Runner the protocol-number check and the
// correction lines read the repository with: git in dir (the current
// directory when empty), with gitEnviron captured now.
func newGitRunner(dir string) Runner {
	return cmdrunner.NewOS(cmdrunner.Options{Dir: dir, Env: gitEnviron()})
}

// newTrailerRunner returns the runner the trailer reading runs git with: in
// dir, an empty directory made for one call, with env (gitEnviron captured
// earlier) narrowed so no configuration of the user or of a repository
// reaches git. Contract: the system and global configuration files are off
// (GIT_CONFIG_NOSYSTEM, GIT_CONFIG_GLOBAL=/dev/null); HOME and
// XDG_CONFIG_HOME point at dir, so a git too old to read GIT_CONFIG_GLOBAL
// finds no configuration there either; and GIT_CEILING_DIRECTORIES names
// dir's parent, so git never looks for a repository, whose configuration it
// would read, above dir.
func newTrailerRunner(dir string, env []string) cmdrunner.InputRunner {
	narrowed := slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		return strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "XDG_CONFIG_HOME=")
	})
	narrowed = append(narrowed,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"HOME="+dir,
		"XDG_CONFIG_HOME="+dir,
		"GIT_CEILING_DIRECTORIES="+filepath.Dir(dir),
	)
	return cmdrunner.NewOS(cmdrunner.Options{Dir: dir, Env: narrowed})
}
