package fixprguard

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The watch list: the files git ignores that the tool still compares with
// the baseline, because a later session or a later make test reads them. It
// is kept apart from the deny list of the scope rules, which decides what a
// round may change, and shares only goBuildInputSuffixes with it. Matching
// reads the repository-relative path alone, never the file system, and folds
// case with Unicode simple folding (foldEqual). A case-insensitive macOS
// volume also applies the full folding that expands one rune into several
// (U+FB01 to "fi"); a name equal to an item only under that expansion is not
// matched, a residual risk the guide records.
var (
	// watchTopDir matches every path whose first element it is, the element
	// itself included.
	watchTopDir = ".claude"
	// watchTopFiles match a path of one element.
	watchTopFiles = []string{".env", ".envrc", "claude.local.md"}
	// watchNames match a base name in any directory.
	watchNames = []string{"claude.md", "makefile", "gnumakefile", "go.mod", "go.work"}
	// watchSuffixes match the end of a base name in any directory, a name
	// that is only the suffix included; goBuildInputSuffixes are matched the
	// same way.
	watchSuffixes = []string{".go", ".mk"}
	// watchAllSuffixes is every suffix the watch list matches.
	watchAllSuffixes = slices.Concat(watchSuffixes, goBuildInputSuffixes)
)

// WatchListPatterns lists the watch list in the form the guide prints it:
// ".claude/" matches every path below the top-level .claude, a pattern
// starting with "/" matches that file at the top level only, a bare name
// matches a file name in any directory, and "*.ext" matches the end of a file
// name in any directory. Matching folds case as foldEqual does.
func WatchListPatterns() []string {
	patterns := []string{watchTopDir + "/"}
	for _, f := range watchTopFiles {
		patterns = append(patterns, "/"+f)
	}
	patterns = append(patterns, watchNames...)
	for _, s := range watchAllSuffixes {
		patterns = append(patterns, "*"+s)
	}
	return patterns
}

// isWatched reports whether a repository-relative slash path is on the watch
// list. The path is matched as it is, never cleaned or normalized: an
// invalid byte or a control character in a name does not take a path off the
// list.
func isWatched(p string) bool {
	elems := strings.Split(p, "/")
	if foldEqual(elems[0], watchTopDir) {
		return true
	}
	equalTo := func(name string) func(string) bool {
		return func(s string) bool { return foldEqual(name, s) }
	}
	if len(elems) == 1 && slices.ContainsFunc(watchTopFiles, equalTo(p)) {
		return true
	}
	name := elems[len(elems)-1]
	if slices.ContainsFunc(watchNames, equalTo(name)) {
		return true
	}
	return slices.ContainsFunc(watchAllSuffixes, func(s string) bool {
		return hasFoldSuffix(name, s)
	})
}

// foldUnit is one unit of a string as foldEqual compares it: a valid rune as
// its code point, which is not negative, or an invalid byte b as -1-b.
type foldUnit int32

// foldUnits splits s into its valid runes and its invalid bytes.
func foldUnits(s string) []foldUnit {
	units := make([]foldUnit, 0, len(s))
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			units = append(units, foldUnit(-1-int32(s[0])))
		} else {
			units = append(units, foldUnit(r))
		}
		s = s[size:]
	}
	return units
}

// sameFold reports whether two units are equal under Unicode simple case
// folding: two runes whose unicode.SimpleFold orbits meet (k, K and KELVIN
// SIGN; s, S and LONG S), or the same invalid byte. An invalid byte equals no
// rune, U+FFFD included.
func sameFold(a, b foldUnit) bool {
	if a == b {
		return true
	}
	if a < 0 || b < 0 {
		return false
	}
	for r := unicode.SimpleFold(rune(a)); r != rune(a); r = unicode.SimpleFold(r) {
		if r == rune(b) {
			return true
		}
	}
	return false
}

// foldEqual reports whether a and b are equal unit by unit under sameFold.
// strings.EqualFold is not used: it reads every invalid byte as U+FFFD and so
// equates different invalid byte sequences.
func foldEqual(a, b string) bool {
	return slices.EqualFunc(foldUnits(a), foldUnits(b), sameFold)
}

// hasFoldSuffix reports whether name ends with suffix under sameFold. The
// units are compared, not the bytes, since folded runes differ in length
// (KELVIN SIGN is three bytes, k one).
func hasFoldSuffix(name, suffix string) bool {
	n, s := foldUnits(name), foldUnits(suffix)
	return len(n) >= len(s) && slices.EqualFunc(n[len(n)-len(s):], s, sameFold)
}
