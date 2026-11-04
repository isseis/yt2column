package coveragecompare

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// extTestGlob is the extension's test files, as the test script of
// extension/package.json names them; a test keeps the two equal.
const extTestGlob = "test/**/*.test.ts"

// extPathspec is what the extension diff covers: the product code
// directories.
var extPathspec = []string{"extension/src", "extension/scripts"}

// pathspec is the extension's product code directories.
func (extTarget) pathspec() []string { return extPathspec }

// checkToolchain checks the installed node and npm, the user's
// extension/node_modules, and both points' lockfiles, and returns the node
// version. npm runs only as npm --version: the tool installs nothing.
func (t extTarget) checkToolchain(ctx context.Context, base, head source) (string, error) {
	nodeVersion, err := t.rc.version(ctx, "node", "--version")
	if err != nil {
		return "", err
	}
	npmVersion, err := t.rc.version(ctx, "npm", "--version")
	if err != nil {
		return "", err
	}
	if err := checkExtVersions(nodeVersion, npmVersion, t.rc.repoRoot, base, head); err != nil {
		return "", err
	}
	return nodeVersion, nil
}

// prepare links the user's dependencies into the checkout.
func (t extTarget) prepare(checkout string) error {
	return linkNodeModules(t.deps.userModules, checkout)
}

// run runs the extension suite of a checkout from its extension directory,
// with the lcov and TAP reports written to the work directory. node is run
// directly, not through npm run test, so the report destinations can be
// given. The installed dependencies are read before the BASE suite and
// compared after the HEAD suite.
func (t extTarget) run(ctx context.Context, point Point, checkout string) (rawRun, error) {
	if point == PointBase {
		if err := t.deps.recordBase(); err != nil {
			return rawRun{}, err
		}
	}
	lcov := t.rc.reportPath(point, ".lcov")
	tap := t.rc.reportPath(point, ".tap")
	extensionDir := filepath.Join(checkout, "extension")
	code, _, err := t.rc.runSuite(ctx, point, extensionDir, "node",
		"--test", "--experimental-test-coverage",
		"--test-reporter=lcov", "--test-reporter-destination="+lcov,
		"--test-reporter=tap", "--test-reporter-destination="+tap,
		extTestGlob)
	if err != nil {
		return rawRun{}, err
	}
	raw := rawRun{ExitCode: code, ExtensionDir: extensionDir}
	if raw.Coverage, err = readReportFile(lcov, code, point); err != nil {
		return rawRun{}, err
	}
	if raw.TAP, err = readReportFile(tap, code, point); err != nil {
		return rawRun{}, err
	}
	if point == PointHead {
		if err := t.deps.checkHead(); err != nil {
			return rawRun{}, err
		}
	}
	return raw, nil
}

// helperRanges runs the range helper of the HEAD checkout over the files of
// either point's checkout, so BASE is read even when its commit predates the
// helper. typescript comes from the HEAD checkout's node_modules.
type helperRanges struct {
	rc *runContext
}

// functionRanges runs the helper with the point's extension directory as
// --root. The helper fails the run as internal when it exits unsuccessfully or
// writes to standard error; its standard output is read by parseRanges.
func (h helperRanges) functionRanges(ctx context.Context, src source, files []string) ([]fileRanges, error) {
	point := src.point()
	if point != PointBase && point != PointHead {
		return nil, internalStop(point, "the range helper was asked about no point", errNoPoint)
	}
	headExtension := filepath.Join(h.rc.checkoutDir(PointHead), "extension")
	args := []string{
		filepath.Join(headExtension, "scripts", "function-ranges.ts"),
		"--root", filepath.Join(h.rc.checkoutDir(point), "extension"),
		"--",
	}
	for _, file := range files {
		path, ok := helperPath(file)
		if !ok {
			return nil, internalStop(point, "a requested file is not under extension/", errNotExtensionFile)
		}
		args = append(args, path)
	}
	runner := cmdrunner.NewOS(cmdrunner.Options{Dir: headExtension, Env: h.rc.child, StderrIsFailure: true})
	out, err := runner.Run(ctx, "node", args...)
	if err != nil {
		return nil, commandStop(ctx, point, "the range helper failed", err)
	}
	return parseRanges(out, point, files)
}

var (
	// errNoPoint reports a source that is neither BASE nor HEAD.
	errNoPoint = errors.New("the source names no point")
	// errNotExtensionFile reports a product file outside extension/.
	errNotExtensionFile = errors.New("the file is not under extension/")
)
