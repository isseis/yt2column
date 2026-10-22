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
// Two lists narrow the rules. The deny list (the execution-related files)
// applies everywhere: an execution-related file is in scope only when it is p
// and the thread is not a named untrusted one, whose author chose p. The
// allow list applies to rules 2 and 3 only: they admit a file that is not
// execution-related only when its name ends in .md or .txt, a directory
// element is testdata, or it is a .go file under the .go rule. The .go rule
// takes every .go file before the rest of the allow list: a .go file is in
// scope beside p only when p is a .go file in the same directory (a test of
// the same package) and the thread is not named. Deny-list matching folds
// case with strings.ToLower and allow-list matching does not, so neither direction widens
// the scope through case. A change that is not a regular file, and a path
// that fails validScopePath, is always out of scope.

// taskDirPrefix is the parent of the task directories rule 3 covers.
const taskDirPrefix = "docs/tasks/"

// taskNumberDigits is the length of the NNNN in docs/tasks/NNNN_<name>/.
const taskNumberDigits = 4

// The deny list, the execution-related files: what make fmt, make test, make
// lint and CI read as configuration, compile or run; the instruction files
// Claude Code and the automated reviewers read; and every path with an element
// that starts with ".". Matching folds case with strings.ToLower, because a
// case-insensitive file system resolves makefile to Makefile.
var (
	// execNames match a base name in any directory.
	execNames = []string{"makefile", "gnumakefile", "go.mod", "go.sum", "go.work", "go.work.sum"}
	// instructionFileNames match a base name in any directory: the files
	// Claude Code and the automated reviewers read as instructions for the
	// directory they are in.
	instructionFileNames = []string{"claude.md", "agents.md", "claude.local.md", "agent.md", "gemini.md", "agents.override.md"}
	// execSuffixes match the end of a base name in any directory.
	execSuffixes = []string{".mk"}
	// goBuildInputSuffixes match the end of a base name in any directory: the
	// files besides .go files that the go command builds into a package
	// (go help filetype, and the Fortran files of go/build).
	goBuildInputSuffixes = []string{
		".c", ".h", ".cc", ".cpp", ".cxx", ".hh", ".hpp", ".hxx", ".m",
		".s", ".sx", ".f", ".for", ".f90", ".swig", ".swigcxx", ".syso",
	}
	// execDirs match every path below them.
	execDirs = []string{"extension/scripts/"}
	// execExtensionFiles match a file directly under extension/.
	execExtensionFiles = []string{
		"package.json", "package-lock.json", "eslint.config.js", "tsconfig.json", "tsconfig.core.json", "tsconfig.build.json",
	}
)

// The allow list rules 2 and 3 admit besides the .go rule. Matching is
// case-sensitive: the list widens the scope, so README.MD and TestData/ stay
// out.
var (
	// allowSuffixes match the end of a base name.
	allowSuffixes = []string{".md", ".txt"}
	// allowDirElements match a directory element, not the base name.
	allowDirElements = []string{"testdata"}
)

// ExecutionFilePatterns lists the execution-related file definitions (the
// deny list) in the form the guide prints them: a bare name or "*.ext"
// matches a file name in any directory, ".*" matches any path element that
// starts with ".", and a pattern ending in "/" or naming a file under
// "extension/" is a path from the repository root. Matching folds case with
// strings.ToLower.
func ExecutionFilePatterns() []string {
	patterns := slices.Concat(execNames, instructionFileNames)
	for _, s := range slices.Concat(execSuffixes, goBuildInputSuffixes) {
		patterns = append(patterns, "*"+s)
	}
	patterns = append(patterns, ".*")
	patterns = append(patterns, execDirs...)
	for _, s := range execExtensionFiles {
		patterns = append(patterns, "extension/"+s)
	}
	return patterns
}

// AllowListPatterns lists the allow list that rules 2 and 3 admit, in the form
// the guide prints them: "*.md" and "*.txt" match the end of a file name and
// "testdata/" a directory element, both case-sensitively; "*.go" is not a
// pattern but stands for the .go rule, which folds case.
func AllowListPatterns() []string {
	var patterns []string
	for _, s := range allowSuffixes {
		patterns = append(patterns, "*"+s)
	}
	for _, d := range allowDirElements {
		patterns = append(patterns, d+"/")
	}
	return append(patterns, "*.go")
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
	if slices.Contains(execNames, base) || slices.Contains(instructionFileNames, base) {
		return true
	}
	for _, s := range slices.Concat(execSuffixes, goBuildInputSuffixes) {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	// validScopePath has already refused the "." and ".." elements, so an
	// element that starts with "." names a file or a directory.
	for elem := range strings.SplitSeq(folded, "/") {
		if strings.HasPrefix(elem, ".") {
			return true
		}
	}
	for _, d := range execDirs {
		if strings.HasPrefix(folded, d) {
			return true
		}
	}
	if rest, ok := strings.CutPrefix(folded, "extension/"); ok && slices.Contains(execExtensionFiles, rest) {
		return true
	}
	return false
}

// inAllowList reports whether p matches allowSuffixes or allowDirElements,
// without folding case. The .go rule is applied by inScopeOf, not here.
func inAllowList(p string) bool {
	dir, base := path.Split(p)
	for _, s := range allowSuffixes {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	for elem := range strings.SplitSeq(strings.TrimSuffix(dir, "/"), "/") {
		if slices.Contains(allowDirElements, elem) {
			return true
		}
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

// inScopeOf applies the rules for one thread path p, in this order: the deny
// list, p itself, the ASCII check, the .go rule, the places of rules 2 and 3,
// and the allow list. The .go rule comes before the allow list so a .go file
// under testdata/ is judged by the .go rule alone.
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
	if !sameDir {
		dir := taskDir(p)
		if dir == "" || !strings.HasPrefix(changed, dir) {
			return false
		}
	}
	return inAllowList(changed)
}
