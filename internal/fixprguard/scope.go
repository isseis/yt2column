package fixprguard

import (
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The scope rules decide, for each changed path, whether the fix phase was
// allowed to change it given the paths of the threads it handled. They are
// pure string functions: the result never depends on the file system, so a
// case-insensitive macOS volume and a case-sensitive Linux CI judge alike.
//
// A changed path is in scope for a thread path p when one of these holds:
//
//  1. it is p itself;
//  2. it is in p's directory (not in a subdirectory);
//  3. p is under docs/tasks/NNNN_<name>/ and it is under the same directory,
//     at any depth.
//
// Rules 2 and 3 never admit an execution-related file or a .go file, with one
// exception: a .go file in the directory of a .go p (a test of the same
// package). For a named untrusted thread, whose author chose p, an
// execution-related file is out of scope even when it is p, and a .go file is
// in scope only when it is p. A change that is not a regular file, and a path
// that fails validScopePath, is always out of scope.

// taskDirPrefix is the parent of the task directories rule 3 covers.
const taskDirPrefix = "docs/tasks/"

// taskNumberDigits is the length of the NNNN in docs/tasks/NNNN_<name>/.
const taskNumberDigits = 4

// The execution-related files: what make fmt, make test, make lint and CI read
// as configuration, compile or run. Matching folds ASCII case, because a
// case-insensitive file system resolves makefile to Makefile.
var (
	// execNames match a base name in any directory.
	execNames = []string{"makefile", "gnumakefile", "go.mod", "go.sum", "go.work", "go.work.sum", ".tool-versions", ".mise.toml"}
	// execSuffixes match the end of a base name in any directory.
	execSuffixes = []string{".mk"}
	// execDirs match every path below them.
	execDirs = []string{".claude/", ".github/", "extension/scripts/"}
	// execTopLevel match a file at the repository root; every root file whose
	// name starts with "." matches too.
	execTopLevel = []string{"claude.md", "agents.md"}
	// execExtensionFiles match a file directly under extension/.
	execExtensionFiles = []string{
		"package.json", "package-lock.json", ".npmrc", ".node-version", ".prettierrc.json",
		".prettierignore", "eslint.config.js", "tsconfig.json", "tsconfig.core.json", "tsconfig.build.json",
	}
)

// ExecutionFilePatterns lists the execution-related file definitions in the
// form the guide prints them: a bare name or "*.ext" matches in any
// directory, a trailing "/" is a directory, a leading "/" is the repository
// root, and "*.go" stands for the .go rule.
func ExecutionFilePatterns() []string {
	patterns := slices.Clone(execNames)
	for _, s := range execSuffixes {
		patterns = append(patterns, "*"+s)
	}
	patterns = append(patterns, "*.go")
	patterns = append(patterns, execDirs...)
	patterns = append(patterns, "/.*")
	for _, s := range execTopLevel {
		patterns = append(patterns, "/"+s)
	}
	for _, s := range execExtensionFiles {
		patterns = append(patterns, "extension/"+s)
	}
	return patterns
}

// validScopePath accepts a repository-relative slash path that is valid
// UTF-8, has no control character, is not absolute, has no ".." element, and
// is unchanged by cleaning. A path that fails is out of scope and is never
// printed.
func validScopePath(p string) bool {
	if p == "" || !utf8.ValidString(p) || strings.HasPrefix(p, "/") || path.Clean(p) != p {
		return false
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return false
		}
	}
	return !slices.Contains(strings.Split(p, "/"), "..")
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// isExecutionRelated reports whether p is one of the execution-related files.
func isExecutionRelated(p string) bool {
	folded := strings.ToLower(p)
	base := path.Base(folded)
	if slices.Contains(execNames, base) {
		return true
	}
	for _, s := range execSuffixes {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	for _, d := range execDirs {
		if strings.HasPrefix(folded, d) {
			return true
		}
	}
	if !strings.Contains(folded, "/") && (strings.HasPrefix(folded, ".") || slices.Contains(execTopLevel, folded)) {
		return true
	}
	if rest, ok := strings.CutPrefix(folded, "extension/"); ok && slices.Contains(execExtensionFiles, rest) {
		return true
	}
	return false
}

func isGoFile(p string) bool { return strings.HasSuffix(strings.ToLower(p), ".go") }

// taskDir returns the docs/tasks/NNNN_<name>/ directory p is under, or "".
func taskDir(p string) string {
	rest, ok := strings.CutPrefix(p, taskDirPrefix)
	if !ok {
		return ""
	}
	dir, _, ok := strings.Cut(rest, "/")
	if !ok || len(dir) <= taskNumberDigits+1 || dir[taskNumberDigits] != '_' {
		return ""
	}
	for i := range taskNumberDigits {
		if dir[i] < '0' || dir[i] > '9' {
			return ""
		}
	}
	return taskDirPrefix + dir + "/"
}

// inScope reports whether a change of kind at changed is in scope for the
// thread paths. Only ChangeAdd, ChangeModify and ChangeDelete of a regular
// file can be in scope.
func inScope(changed string, kind ChangeKind, threads []ThreadPath) bool {
	switch kind {
	case ChangeAdd, ChangeModify, ChangeDelete:
	default:
		return false
	}
	if !validScopePath(changed) {
		return false
	}
	for _, t := range threads {
		if validScopePath(t.Path) && inScopeOf(changed, t.Path, t.Named) {
			return true
		}
	}
	return false
}

// inScopeOf applies the rules for one thread path p.
func inScopeOf(changed, p string, named bool) bool {
	if isExecutionRelated(changed) {
		return !named && changed == p
	}
	if changed == p {
		return true
	}
	// A non-ASCII name could reach another file through Unicode
	// normalization or case folding, so only an exact match admits it.
	if !isASCII(changed) {
		return false
	}
	sameDir := path.Dir(changed) == path.Dir(p)
	if isGoFile(changed) {
		return !named && isGoFile(p) && sameDir
	}
	if sameDir {
		return true
	}
	dir := taskDir(p)
	return dir != "" && strings.HasPrefix(changed, dir)
}
