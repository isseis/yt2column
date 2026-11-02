package coveragecompare

import (
	"go/build"
	"go/build/constraint"
	"runtime"
	"slices"
	"strings"
)

// isGoProduct reports whether a Go file is product code: it is not named
// _test.go, and its //go:build expression holds in the measured build without
// the test tag, or does not name the test tag at all. A file with no
// //go:build line is product code. The expression is read with
// go/build/constraint, so a tag name in a comment or a string never counts, and
// a malformed expression stops as unreadable instead of being guessed.
func isGoProduct(fileName string, src []byte, point Point) (bool, error) {
	if strings.HasSuffix(fileName, "_test.go") {
		return false, nil
	}
	var expression constraint.Expr
	for i, line := range splitLines(string(src)) {
		if strings.HasPrefix(line, "package ") {
			break
		}
		if !strings.HasPrefix(line, "//go:build") {
			continue
		}
		parsed, err := constraint.Parse(line)
		if err != nil {
			return false, stopError(point, InputSource, i+1, "the //go:build line is not a valid constraint", line)
		}
		if expression != nil {
			return false, stopError(point, InputSource, i+1, "a second //go:build line appears", line)
		}
		expression = parsed
	}
	if expression == nil {
		return true, nil
	}
	// Evaluate the measured build's tags without the test tag: unix, the
	// host's GOOS, GOARCH, compiler, and release tags hold, and race and
	// integration do not. A constraint that
	// holds there keeps the file product code even when it also names test
	// (unix || test). A constraint that does not hold there is test-only when
	// it names test (test || integration); when it names test nowhere
	// (integration, race), the file is still product code.
	if expression.Eval(measuredBuildTag) {
		return true, nil
	}
	return !namesTestTag(expression, true), nil
}

// measuredBuildTag reports whether a tag other than test holds in the
// measured build, which runs on this host with -tags test only.
func measuredBuildTag(tag string) bool {
	switch tag {
	case "unix", runtime.GOOS, runtime.GOARCH, runtime.Compiler:
		return true
	default:
		return slices.Contains(build.Default.ReleaseTags, tag)
	}
}

// namesTestTag reports whether a build constraint requires the test tag or is
// satisfied by it: whether the test tag appears with positive polarity once
// the negations around it are counted. positive is false inside an odd number
// of negations. A negated test tag is not either: `//go:build !test` builds
// in the default build and is product code, while `!(!test)` requires test.
// A constraint that names only
// integration or unix stays product code too, because those files build with or
// without the test tag and never describe a test helper.
func namesTestTag(expression constraint.Expr, positive bool) bool {
	switch e := expression.(type) {
	case *constraint.TagExpr:
		return positive && e.Tag == "test"
	case *constraint.NotExpr:
		return namesTestTag(e.X, !positive)
	case *constraint.AndExpr:
		return namesTestTag(e.X, positive) || namesTestTag(e.Y, positive)
	case *constraint.OrExpr:
		return namesTestTag(e.X, positive) || namesTestTag(e.Y, positive)
	default:
		return false
	}
}
