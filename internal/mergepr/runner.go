package mergepr

import (
	"os"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// NewOSRunner returns the production Runner: the shared runner with its
// default, unbounded options.
func NewOSRunner() Runner { return cmdrunner.NewOS(cmdrunner.Options{}) }

// newGitRunner returns the Runner the protocol-number check runs git with: in
// dir (the current directory when empty), with the environment of this
// process at the time of the call minus every inherited GIT_* variable
// (cmdrunner.GitEnv), so a GIT_DIR or GIT_CONFIG_* set by the caller cannot
// point the check at another repository or configuration.
func newGitRunner(dir string) Runner {
	return cmdrunner.NewOS(cmdrunner.Options{Dir: dir, Env: cmdrunner.GitEnv(os.Environ())})
}
