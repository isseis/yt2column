package coveragecompare

import (
	"go/build/constraint"
	"strings"
)

// isGoProduct reports whether a Go file is product code: it is not named
// _test.go, and its //go:build expression does not require the test tag or hold
// whenever the test tag holds. A file with no //go:build line is product code.
// The expression is read with go/build/constraint, so a tag name in a comment
// or a string never counts, and a malformed expression stops as unreadable
// instead of being guessed.
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
	return !namesTestTag(expression), nil
}

// namesTestTag reports whether a build constraint requires the test tag or is
// satisfied by it. A negated test tag is not either: `//go:build !test` builds
// in the default build and is product code. A constraint that names only
// integration or unix stays product code too, because those files build with or
// without the test tag and never describe a test helper.
func namesTestTag(expression constraint.Expr) bool {
	switch e := expression.(type) {
	case *constraint.TagExpr:
		return e.Tag == "test"
	case *constraint.NotExpr:
		return false
	case *constraint.AndExpr:
		return namesTestTag(e.X) || namesTestTag(e.Y)
	case *constraint.OrExpr:
		return namesTestTag(e.X) || namesTestTag(e.Y)
	default:
		return false
	}
}
