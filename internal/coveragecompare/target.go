package coveragecompare

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The bounds of a test suite's process group once the run is canceled.
const (
	// suiteStopGrace is how long the suite has, after SIGTERM, to clean up
	// before its process group is killed.
	suiteStopGrace = 5 * time.Second
	// suitePipeWait bounds the wait for an output pipe still held by a
	// process that left the group after the group was killed.
	suitePipeWait = time.Second
)

// runContext is what the execution methods of the targets share: the user's
// repository, the work directory, the environment the run was given (for
// git), and the environment of the go and node commands.
type runContext struct {
	repoRoot string
	workDir  string
	environ  []string
	child    []string
}

// newRunContext returns the context of a run in workDir.
func newRunContext(repoRoot, workDir string, environ []string) *runContext {
	return &runContext{
		repoRoot: repoRoot,
		workDir:  workDir,
		environ:  environ,
		child:    childEnv(environ, filepath.Join(workDir, "tmp")),
	}
}

// repoGit is the git of the user's repository.
func (c *runContext) repoGit() gitRunner {
	return newGit(c.repoRoot, c.environ)
}

// checkoutDir is where a point's checkout is expanded.
func (c *runContext) checkoutDir(point Point) string {
	return filepath.Join(c.workDir, point.String())
}

// reportPath is where a point's report with the extension ext is written.
func (c *runContext) reportPath(point Point, ext string) string {
	return filepath.Join(c.workDir, point.String()+ext)
}

// version runs a command that prints a version, in the work directory with
// the child environment, and returns its output without the final newline.
func (c *runContext) version(ctx context.Context, name string, args ...string) (string, error) {
	runner := cmdrunner.NewOS(cmdrunner.Options{Dir: c.workDir, Env: c.child})
	out, err := runner.Run(ctx, name, args...)
	if err != nil {
		return "", commandStop(ctx, PointNone, "running "+name+" "+strings.Join(args, " ")+" failed", err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// runSuite runs a point's test suite in dir and returns its exit code and
// standard output.
//
// Contract: the suite runs as its own process group, so a canceled run stops
// every process the suite started (SIGTERM, then SIGKILL after
// suiteStopGrace); its output is not capped. A canceled run is the tool's
// interruption even when the suite then exits unsuccessfully, so a signal is
// never reported as a test failure. A command that cannot be started is the
// tool's own failure; one that runs and exits unsuccessfully is a result.
func (c *runContext) runSuite(ctx context.Context, point Point, dir, name string, args ...string) (int, []byte, error) {
	runner := cmdrunner.NewOS(cmdrunner.Options{
		Dir:            dir,
		Env:            c.child,
		ProcessGroup:   true,
		GroupStopGrace: suiteStopGrace,
		WaitDelay:      suitePipeWait,
	})
	out, err := runner.Run(ctx, name, args...)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, nil, interruptedStop(ctxErr)
	}
	if err == nil {
		return 0, out, nil
	}
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok {
		return exitErr.Code, exitErr.Stdout, nil
	}
	return 0, nil, internalStop(point, "running the test suite failed", err)
}

// readReportFile reads a report the suite wrote. A suite that failed may not
// have written it; its run stops as test-failed before any report is read,
// so the missing file is left empty then.
func readReportFile(path string, exitCode int, point Point) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is under the work directory the tool created
	if err == nil {
		return data, nil
	}
	if exitCode != 0 && errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return nil, internalStop(point, "reading a report of the test suite failed", err)
}
