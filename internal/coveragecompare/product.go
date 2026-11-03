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
// go/build/constraint from the line comments before the package clause, so a
// tag name in a comment or a string never counts, and a malformed expression
// stops as unreadable instead of being guessed. Block comments are skipped as a
// whole, so an example of a build line inside one is not a constraint and does
// not end the scan.
func isGoProduct(fileName string, src []byte, point Point) (bool, error) {
	if strings.HasSuffix(fileName, "_test.go") {
		return false, nil
	}
	var expression constraint.Expr
	for _, comment := range headerLineComments(string(src)) {
		if !constraint.IsGoBuild(comment.text) {
			continue
		}
		parsed, err := constraint.Parse(comment.text)
		if err != nil {
			return false, stopError(point, InputSource, comment.line, "the //go:build line is not a valid constraint", comment.text)
		}
		if expression != nil {
			return false, stopError(point, InputSource, comment.line, "a second //go:build line appears", comment.text)
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

// headerComment is a line comment from a file header, with its 1-based line
// number.
type headerComment struct {
	line int
	text string
}

// headerLineComments returns the line comments that appear before the package
// clause, each with its 1-based line number. Block comments are skipped as a
// whole, so a build-line example inside one is not read as a constraint, and
// the scan carries on after the block comment ends.
func headerLineComments(text string) []headerComment {
	var comments []headerComment
	line := 1
	atLineStart := true
	for i := 0; i < len(text); {
		switch {
		case text[i] == '\n':
			line++
			atLineStart = true
			i++
		case text[i] == ' ' || text[i] == '\t' || text[i] == '\r':
			i++
		case strings.HasPrefix(text[i:], "/*"):
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				// An unterminated block comment hides the rest of the file,
				// so no later line is an actual constraint.
				return comments
			}
			skipped := text[i : i+2+end+2]
			line += strings.Count(skipped, "\n")
			i += len(skipped)
			// A token appeared on this line, so a later // on the same
			// physical line is not at its start and is not a build line.
			atLineStart = false
		case strings.HasPrefix(text[i:], "//"):
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text) - i
			}
			if atLineStart {
				comments = append(comments, headerComment{line: line, text: text[i : i+end]})
			}
			i += end
		case atLineStart && startsPackageClause(text[i:]):
			return comments
		default:
			atLineStart = false
			i++
		}
	}
	return comments
}

// startsPackageClause reports whether text begins with a package clause: the
// keyword followed by whitespace. A tab or a newline after the keyword is valid
// Go, so the clause must not be recognized by one literal space.
func startsPackageClause(text string) bool {
	rest, ok := strings.CutPrefix(text, "package")
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	switch rest[0] {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
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
		return positive && e.Tag == testTag
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
