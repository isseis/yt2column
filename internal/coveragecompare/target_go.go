package coveragecompare

import (
	"context"
)

// testTag is the build tag the suite is built with, the one that marks test
// helpers (isGoProduct).
const testTag = "test"

// goPathspec is what the Go diff covers: every Go file at any depth.
var goPathspec = []string{"*.go"}

// pathspec is the Go files.
func (goTarget) pathspec() []string { return goPathspec }

// checkToolchain checks the installed go against both points' go.mod and
// returns its version.
func (t goTarget) checkToolchain(ctx context.Context, base, head source) (string, error) {
	goVersion, err := t.rc.version(ctx, "go", "env", "GOVERSION")
	if err != nil {
		return "", err
	}
	if err := checkGoVersion(goVersion, base, head); err != nil {
		return "", err
	}
	return goVersion, nil
}

// prepare needs nothing for Go.
func (goTarget) prepare(string) error { return nil }

// run runs the Go suite of a checkout with every package's statements
// counted, whichever package's test runs them (-coverpkg=./...), and without
// the test cache (-count=1). -race is not given: it does not change which
// statements run.
func (t goTarget) run(ctx context.Context, point Point, checkout string) (rawRun, error) {
	profile := t.rc.reportPath(point, ".cover")
	code, stdout, err := t.rc.runSuite(ctx, point, checkout, "go",
		"test", "-tags", testTag, "-count=1", "-json", "-coverpkg=./...", "-coverprofile="+profile, "./...")
	if err != nil {
		return rawRun{}, err
	}
	coverage, err := readReportFile(profile, code, point)
	if err != nil {
		return rawRun{}, err
	}
	return rawRun{ExitCode: code, Stdout: stdout, Coverage: coverage}, nil
}
