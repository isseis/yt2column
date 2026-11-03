package coveragecompare

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// workPrefix names the work directory under the temporary directory.
const workPrefix = "coveragecompare-"

// makeWorkDir creates the work directory: the one directory, readable and
// writable by its owner alone, that holds everything the run creates (the
// checkouts, the reports, and the temporary directory of the go and node
// commands), so removing it removes them all. It is made under the TMPDIR of
// environ, the environment the run was given, or under the system's
// temporary directory when environ has none; its symbolic links are resolved,
// so paths the commands report compare equal to the paths the tool built.
func makeWorkDir(environ []string) (string, error) {
	dir, err := os.MkdirTemp(envValue(environ, "TMPDIR"), workPrefix)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err := os.Mkdir(filepath.Join(resolved, "tmp"), 0o700); err != nil {
		_ = os.RemoveAll(resolved)
		return "", err
	}
	return resolved, nil
}

// envValue returns the value of the first entry of environ named name, or "".
func envValue(environ []string, name string) string {
	for _, entry := range environ {
		if key, value, ok := strings.Cut(entry, "="); ok && key == name {
			return value
		}
	}
	return ""
}

// makeCheckout expands commit into dir, outside the user's repository.
//
// Contract: the user's worktree list does not change. A --shared clone is not
// registered as a worktree of the user's repository (git worktree add would
// be), and it reads the user's objects in place, so creating it copies
// nothing. No hook runs: the clone does not check out, and the checkout runs
// with core.hooksPath=/dev/null. No reflog is written (core.logAllRefUpdates
// is off): writing one needs a committer identity, which the throwaway
// checkout has none of without the global configuration, and git then looks
// up the host's name, which can take seconds.
func makeCheckout(ctx context.Context, repo gitRunner, repoRoot, dir, commit string, environ []string) error {
	if _, err := repo.run(ctx, "-c", noReflog, "clone", "-q", "--shared", "--no-checkout", "--", repoRoot, dir); err != nil {
		return commandStop(ctx, PointNone, "cloning a checkout failed", err)
	}
	checkout := newGit(dir, environ)
	if _, err := checkout.run(ctx, "-c", noReflog, "-c", "core.hooksPath=/dev/null", "checkout", "-q", "--detach", commit); err != nil {
		return commandStop(ctx, PointNone, "checking out "+commit+" failed", err)
	}
	return nil
}

// noReflog turns reflogs off for one git command.
const noReflog = "core.logAllRefUpdates=false"

// linkNodeModules gives a checkout the user's extension dependencies.
//
// Contract: the checkout's extension/node_modules is a real directory holding
// one symbolic link per entry of the user's extension/node_modules, so both
// points run with the same dependencies and nothing is copied. A link to the
// whole directory would not do: git check-ignore does not follow it, and an
// extension test checks that the files under node_modules are ignored.
func linkNodeModules(userModules, checkout string) error {
	entries, err := os.ReadDir(userModules)
	if err != nil {
		return internalStop(PointNone, "reading the user's extension/node_modules failed", err)
	}
	dir := filepath.Join(checkout, "extension", "node_modules")
	if err := os.Mkdir(dir, 0o700); err != nil {
		return internalStop(PointNone, "creating a checkout's extension/node_modules failed", err)
	}
	for _, entry := range entries {
		if err := os.Symlink(filepath.Join(userModules, entry.Name()), filepath.Join(dir, entry.Name())); err != nil {
			return internalStop(PointNone, "linking a checkout's extension/node_modules failed", err)
		}
	}
	return nil
}

// installedLockName is the file npm writes into node_modules to record what
// it installed.
const installedLockName = ".package-lock.json"

// dependencyGuard checks that the user's extension dependencies do not change
// between the start of the BASE tests and the end of the HEAD tests, as when
// they are installed again from another terminal. Otherwise BASE and HEAD
// would not be measured with the same dependencies.
type dependencyGuard struct {
	userModules string
	atBase      []byte
}

// recordBase reads the installed lockfile before the BASE tests.
func (g *dependencyGuard) recordBase() error {
	data, err := os.ReadFile(filepath.Join(g.userModules, installedLockName))
	if err != nil {
		return internalStop(PointBase, "reading extension/node_modules/"+installedLockName+" failed", err)
	}
	g.atBase = data
	return nil
}

// checkHead compares the installed lockfile after the HEAD tests with the one
// read before the BASE tests.
func (g *dependencyGuard) checkHead() error {
	data, err := os.ReadFile(filepath.Join(g.userModules, installedLockName))
	if err != nil {
		return internalStop(PointHead, "reading extension/node_modules/"+installedLockName+" failed", err)
	}
	if g.atBase == nil || !bytes.Equal(g.atBase, data) {
		return internalStop(PointHead, "extension/node_modules changed during the run", errDependenciesChanged)
	}
	return nil
}

// errDependenciesChanged reports dependencies installed again during a run.
var errDependenciesChanged = errors.New("the installed dependencies changed between BASE and HEAD")
