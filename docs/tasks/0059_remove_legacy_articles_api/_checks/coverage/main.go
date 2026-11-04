//go:build test

// Command coverage compares the function coverage of two commits when the
// later one removes code: it matches each product function by its package,
// receiver, and name, so a function whose line moved, or that moved to another
// file of the same package, is still compared with itself.
//
//	go run -tags test ./docs/tasks/0059_remove_legacy_articles_api/_checks/coverage/ \
//		<before-commit> <after-commit> <before.txt> <after.txt>
//
// Make each output from the repository root, with the commit checked out, with
//
//	go test -tags test -count=1 -coverpkg=./... -coverprofile=<profile> ./...
//	go tool cover -func=<profile> > <output>
//
// Both runs must pass: a run with a failing test is not a valid result.
//
// The cover tool names a function by its file, line, and name, without the
// receiver. The receiver is found by reading the file at that commit (git
// show, never the working tree) and taking the function declared at that line.
// A file is product code unless its name ends in _test.go or its first
// //go:build line starts with the test tag (test, test || integration, ...),
// judged from the file at that commit; the lines of other files are dropped,
// as is the total line.
//
// It prints the functions found only before (the removed ones, which the
// reader checks against what the commit meant to remove), the functions found
// only after, and every function found on both sides whose coverage changed.
//
// Exit status: 0 when no function found on both sides lost coverage; 1 when
// one did; 2 when the comparison cannot be made (wrong arguments, an unreadable
// output or commit, a line the cover tool would not write, or a function that
// does not map to exactly one package, receiver, and name on one side).
//
// Run it from the repository root. make test and CI do not run it, and it is
// not maintained after the merge. The //go:build test first line makes the
// repository-wide environment access check treat the file as test code.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// testConstraint matches a first //go:build line that starts with the test
// tag. The tag must end there, so a tag such as "testing" is not matched.
var testConstraint = regexp.MustCompile(`^//go:build test([^A-Za-z0-9_.]|$)`)

// stopf returns a failure that makes the comparison impossible.
func stopf(format string, args ...any) error { return fmt.Errorf(format, args...) }

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: coverage <before-commit> <after-commit> <before.txt> <after.txt>")
		os.Exit(2)
	}
	before, err := load(os.Args[1], os.Args[3])
	if err != nil {
		stop("read the first side", err)
	}
	after, err := load(os.Args[2], os.Args[4])
	if err != nil {
		stop("read the second side", err)
	}
	fmt.Printf("before  %s %s (%d product functions)\n", os.Args[1], os.Args[3], len(before))
	fmt.Printf("after   %s %s (%d product functions)\n", os.Args[2], os.Args[4], len(after))
	if len(before) == 0 || len(after) == 0 {
		stop("compare", stopf("a side has no product function"))
	}
	os.Exit(report(before, after))
}

// report prints the comparison and returns the exit status.
func report(before, after map[string]float64) int {
	var onlyBefore, onlyAfter, lower, higher []string
	for _, k := range sortedKeys(before) {
		b := before[k]
		a, ok := after[k]
		switch {
		case !ok:
			onlyBefore = append(onlyBefore, fmt.Sprintf("%s (%.1f%%)", k, b))
		case a < b:
			lower = append(lower, fmt.Sprintf("%s: %.1f%% -> %.1f%%", k, b, a))
		case a > b:
			higher = append(higher, fmt.Sprintf("%s: %.1f%% -> %.1f%%", k, b, a))
		}
	}
	for _, k := range sortedKeys(after) {
		if _, ok := before[k]; !ok {
			onlyAfter = append(onlyAfter, fmt.Sprintf("%s (%.1f%%)", k, after[k]))
		}
	}
	section("only before (removed)", onlyBefore)
	section("only after (added)", onlyAfter)
	section("lower coverage", lower)
	section("higher coverage", higher)
	if len(lower) > 0 {
		fmt.Printf("NG   %d function(s) found on both sides lost coverage\n", len(lower))
		return 1
	}
	fmt.Println("OK   no function found on both sides lost coverage")
	return 0
}

func section(title string, lines []string) {
	fmt.Printf("%s: %d\n", title, len(lines))
	for _, l := range lines {
		fmt.Printf("     %s\n", l)
	}
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// load reads one "go tool cover -func" output made at commit and returns the
// coverage of each product function, keyed by "package receiver.name" (or
// "package name" for a function without a receiver).
func load(commit, output string) (map[string]float64, error) {
	module, err := modulePath(commit)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(output)
	if err != nil {
		return nil, stopf("read %s: %v", output, err)
	}
	files := map[string]*sourceFile{}
	out := map[string]float64{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || fields[0] == "total:" {
			continue
		}
		if len(fields) != 3 {
			return nil, stopf("unexpected line %q", scanner.Text())
		}
		location, name, percent := fields[0], fields[1], fields[2]
		file, lineText, ok := strings.Cut(strings.TrimSuffix(location, ":"), ":")
		if !ok {
			return nil, stopf("no line number in %q", location)
		}
		line, err := strconv.Atoi(lineText)
		if err != nil {
			return nil, stopf("line number in %q: %v", location, err)
		}
		value, err := strconv.ParseFloat(strings.TrimSuffix(percent, "%"), 64)
		if err != nil || !strings.HasSuffix(percent, "%") {
			return nil, stopf("coverage in %q is not a percentage", scanner.Text())
		}
		rel, ok := strings.CutPrefix(file, module+"/")
		if !ok {
			return nil, stopf("file %q is outside the module %s", file, module)
		}
		src, ok := files[rel]
		if !ok {
			src, err = readSource(commit, rel)
			if err != nil {
				return nil, err
			}
			files[rel] = src
		}
		if !src.product {
			continue
		}
		recv, err := src.receiverAt(line, name)
		if err != nil {
			return nil, stopf("%s at %s: %v", location, commit, err)
		}
		key := path.Dir(rel) + " " + name
		if recv != "" {
			key = path.Dir(rel) + " " + recv + "." + name
		}
		if _, dup := out[key]; dup {
			return nil, stopf("%s maps to %s, which another function of the output at %s maps to too", location, key, commit)
		}
		out[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, stopf("read %s: %v", output, err)
	}
	return out, nil
}

// modulePath returns the module path go.mod declares at commit.
func modulePath(commit string) (string, error) {
	data, err := gitShow(commit, "go.mod")
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", stopf("go.mod at %s declares no module", commit)
}

// sourceFile is one Go file at a commit.
type sourceFile struct {
	product bool
	fset    *token.FileSet
	file    *ast.File
}

// readSource reads and, when it is product code, parses rel at commit.
func readSource(commit, rel string) (*sourceFile, error) {
	data, err := gitShow(commit, rel)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(rel, "_test.go") || hasTestConstraint(data) {
		return &sourceFile{}, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution)
	if err != nil {
		return nil, stopf("parse %s at %s: %v", rel, commit, err)
	}
	return &sourceFile{product: true, fset: fset, file: file}, nil
}

// hasTestConstraint reports whether the file's first //go:build line starts
// with the test tag. Only the lines before the package clause are read.
func hasTestConstraint(data []byte) bool {
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//go:build") {
			return testConstraint.MatchString(line)
		}
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}
	return false
}

// receiverAt returns the receiver type name of the function named name whose
// declaration starts at line, and "" for a function without a receiver. It
// fails unless exactly one declaration matches.
func (s *sourceFile) receiverAt(line int, name string) (string, error) {
	var found []string
	for _, decl := range s.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || s.fset.Position(fn.Pos()).Line != line {
			continue
		}
		found = append(found, receiverName(fn))
	}
	if len(found) != 1 {
		return "", fmt.Errorf("%d declarations of %s start at line %d, want 1", len(found), name, line)
	}
	return found[0], nil
}

// receiverName returns the base type name of fn's receiver, without a pointer
// or type parameters, and "" when fn has none.
func receiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.Ident:
			return e.Name
		default:
			return fmt.Sprintf("%T", e)
		}
	}
}

// gitShow returns the content of rel at commit.
func gitShow(commit, rel string) ([]byte, error) {
	out, err := exec.Command("git", "show", commit+":"+rel).Output() //nolint:gosec // a commit and path this tool was given
	if err != nil {
		return nil, stopf("git show %s:%s: %v", commit, rel, err)
	}
	return out, nil
}

// stop prints a failure that makes the comparison impossible and exits 2.
func stop(what string, err error) {
	fmt.Fprintf(os.Stderr, "coverage: %s: %v\n", what, err)
	os.Exit(2)
}
