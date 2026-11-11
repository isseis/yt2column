//go:build test

// Command coverage compares, function by function, the statements two
// commits' test runs left unexecuted, for a commit that removes code and
// rewrites the functions that remain. It matches each product function by its
// package, receiver, and name, so a function whose line moved, or that moved
// to another file of the same package, is still compared with itself.
//
//	go build -tags test -o <binary> ./docs/tasks/0083_remove_slack_webhook/_checks/coverage/ \
//		&& <binary> <before-commit> <after-commit> <before.prof> <after.prof>
//
// Build the binary and run it, rather than run it with `go run`: the go
// command reports every non-zero exit of the program as 1, which would hide
// the difference between 1 (a function lost coverage) and 2 (the comparison
// could not be made).
//
// Make each profile from the repository root, with the commit checked out in
// a clean working tree, with
//
//	go test -tags test -count=1 -coverpkg=./... -coverprofile=<profile> ./...
//
// Both runs must pass: a run with a failing test is not a valid result.
//
// A profile line names a block by its file and its start and end line and
// column, with its statement count and how many times it ran. With
// -coverpkg=./... each test binary writes its own line for the same block, so
// the lines of one block are merged by position: its statements count once,
// and it ran when any of its lines has a non-zero count. A block is assigned to
// the function declaration whose source range, read from the file at that
// commit (git show, never the working tree), holds the block's start. A block
// that no function holds (its start lies in a package-level declaration, such
// as the body of a var) is assigned to the outermost declaration that holds it,
// keyed by the names it declares, so it is still compared with itself. A block
// that no declaration holds cannot be compared and stops the run.
//
// A file is product code unless its name ends in _test.go or its first
// //go:build line starts with the test tag (test, test || integration, ...),
// judged from the file at that commit; the blocks of other files are dropped.
//
// For each function it counts the statements and the unexecuted ones (the
// statements of the blocks that never ran), and prints:
//
//   - the functions found only before (the removed ones) and only after (the
//     added ones);
//   - more unexecuted: functions on both sides whose unexecuted statements
//     grew, which fail the comparison;
//   - fewer executed: functions on both sides whose executed statements went
//     down. They do not fail the comparison, because removing an executed
//     statement lowers the count too, but a reader must give each a reason:
//     within one function, removing unexecuted statements can hide executed
//     ones that stopped running, so the first list alone could miss a lost
//     check;
//   - fewer unexecuted: functions on both sides whose unexecuted statements
//     went down, for information.
//
// Exit status: 0 when no function found on both sides has more unexecuted
// statements; 1 when one does; 2 when the comparison cannot be made (wrong
// arguments, an unreadable profile or commit, a line a profile would not
// hold, two lines of one block that disagree on its statement count, a block
// that no declaration holds, a declaration that declares no name, or two
// declarations that map to the same package, receiver, and name).
//
// The first argument ext selects the extension side instead, which compares
// two lcov reports of the extension suite (ext.go).
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

// profileLine matches one block line of a coverprofile:
// file:startLine.startCol,endLine.endCol numStmt count.
var profileLine = regexp.MustCompile(`^(.+\.go):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$`)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "ext" {
		os.Exit(runExt(os.Args[2:]))
	}
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: coverage <before-commit> <after-commit> <before.prof> <after.prof>")
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
		stop("compare", fmt.Errorf("a side has no product function"))
	}
	os.Exit(report(before, after))
}

// counts are one function's statements and the ones no test ran.
type counts struct {
	total      int
	unexecuted int
}

func (c counts) executed() int { return c.total - c.unexecuted }

func (c counts) String() string {
	return fmt.Sprintf("%d of %d unexecuted", c.unexecuted, c.total)
}

// report prints the comparison and returns the exit status.
func report(before, after map[string]counts) int {
	var onlyBefore, onlyAfter, moreUnexecuted, fewerExecuted, fewerUnexecuted []string
	for _, k := range sortedKeys(before) {
		b := before[k]
		a, ok := after[k]
		if !ok {
			onlyBefore = append(onlyBefore, fmt.Sprintf("%s (%v)", k, b))
			continue
		}
		change := fmt.Sprintf("%s: %v -> %v", k, b, a)
		if a.unexecuted > b.unexecuted {
			moreUnexecuted = append(moreUnexecuted, change)
		}
		if a.unexecuted < b.unexecuted {
			fewerUnexecuted = append(fewerUnexecuted, change)
		}
		if a.executed() < b.executed() {
			fewerExecuted = append(fewerExecuted, fmt.Sprintf("%s: executed %d -> %d (%v -> %v)", k, b.executed(), a.executed(), b, a))
		}
	}
	for _, k := range sortedKeys(after) {
		if _, ok := before[k]; !ok {
			onlyAfter = append(onlyAfter, fmt.Sprintf("%s (%v)", k, after[k]))
		}
	}
	section("only before (removed)", onlyBefore)
	section("only after (added)", onlyAfter)
	section("more unexecuted (fails)", moreUnexecuted)
	section("fewer executed (give each a reason)", fewerExecuted)
	section("fewer unexecuted", fewerUnexecuted)
	if len(moreUnexecuted) > 0 {
		fmt.Printf("NG   %d function(s) found on both sides have more unexecuted statements\n", len(moreUnexecuted))
		return 1
	}
	fmt.Println("OK   no function found on both sides has more unexecuted statements")
	return 0
}

func section(title string, lines []string) {
	fmt.Printf("%s: %d\n", title, len(lines))
	for _, l := range lines {
		fmt.Printf("     %s\n", l)
	}
}

func sortedKeys(m map[string]counts) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// block is one coverprofile block position.
type block struct {
	file                                 string
	startLine, startCol, endLine, endCol int
}

// blockState is a block's merged lines: its statement count and whether any
// of its lines ran.
type blockState struct {
	stmts int
	ran   bool
}

// owner is the key a block counts toward, with the key's package, and the
// declaration's position, which tells two declarations of one key apart.
type owner struct {
	key  string
	decl string
}

// load reads one coverprofile made at commit and returns the counts of each
// product function, keyed by "package receiver.name" (or "package name" for a
// function without a receiver).
func load(commit, profile string) (map[string]counts, error) {
	module, err := modulePath(commit)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(profile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %v", profile, err)
	}
	blocks, err := parseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", profile, err)
	}
	files := map[string]*sourceFile{}
	// owners records, per key, the declaration it came from, so two
	// declarations that map to one key are caught.
	owners := map[string]string{}
	out := map[string]counts{}
	for _, b := range sortedBlocks(blocks) {
		state := blocks[b]
		rel, ok := strings.CutPrefix(b.file, module+"/")
		if !ok {
			return nil, fmt.Errorf("file %q is outside the module %s", b.file, module)
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
		o, err := src.ownerAt(b.startLine, b.startCol)
		if err != nil {
			return nil, fmt.Errorf("%s:%d.%d at %s: %v", rel, b.startLine, b.startCol, commit, err)
		}
		o.key = path.Dir(rel) + " " + o.key
		if prev, dup := owners[o.key]; dup && prev != o.decl {
			return nil, fmt.Errorf("the declarations at %s and %s both map to %s at %s", prev, o.decl, o.key, commit)
		}
		owners[o.key] = o.decl
		c := out[o.key]
		c.total += state.stmts
		if !state.ran {
			c.unexecuted += state.stmts
		}
		out[o.key] = c
	}
	return out, nil
}

// parseProfile reads the block lines of a coverprofile and merges the lines
// of each block by position.
func parseProfile(data []byte) (map[block]blockState, error) {
	blocks := map[block]blockState{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			if !strings.HasPrefix(line, "mode: ") {
				return nil, fmt.Errorf("the first line %q is not a mode line", line)
			}
			continue
		}
		if line == "" {
			continue
		}
		m := profileLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("unexpected line %q", line)
		}
		nums := make([]int, 6)
		for i := range nums {
			n, err := strconv.Atoi(m[i+2])
			if err != nil {
				return nil, fmt.Errorf("line %q: %v", line, err)
			}
			nums[i] = n
		}
		b := block{file: m[1], startLine: nums[0], startCol: nums[1], endLine: nums[2], endCol: nums[3]}
		stmts, count := nums[4], nums[5]
		prev, seen := blocks[b]
		if seen && prev.stmts != stmts {
			return nil, fmt.Errorf("two lines of the block %s:%d.%d,%d.%d disagree on its statement count (%d and %d)", b.file, b.startLine, b.startCol, b.endLine, b.endCol, prev.stmts, stmts)
		}
		blocks[b] = blockState{stmts: stmts, ran: prev.ran || count > 0}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if first {
		return nil, fmt.Errorf("empty profile")
	}
	return blocks, nil
}

// sortedBlocks returns the blocks in a fixed order, so an error names the same
// block on every run.
func sortedBlocks(m map[block]blockState) []block {
	keys := make([]block, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b block) int {
		if c := strings.Compare(a.file, b.file); c != 0 {
			return c
		}
		if a.startLine != b.startLine {
			return a.startLine - b.startLine
		}
		return a.startCol - b.startCol
	})
	return keys
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
	return "", fmt.Errorf("go.mod at %s declares no module", commit)
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
		return nil, fmt.Errorf("parse %s at %s: %v", rel, commit, err)
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

// ownerAt returns the owner of the block that starts at line.col: the function
// declaration whose source range holds it, or, for a block no function holds,
// the outermost declaration that holds it, keyed by the names it declares. It
// fails unless exactly one declaration matches.
func (s *sourceFile) ownerAt(line, col int) (owner, error) {
	var found []ast.Decl
	for _, decl := range s.file.Decls {
		if s.holds(decl, line, col) {
			found = append(found, decl)
		}
	}
	if len(found) != 1 {
		return owner{}, fmt.Errorf("%d declarations hold the block, want 1", len(found))
	}
	decl := found[0]
	if fn, ok := decl.(*ast.FuncDecl); ok {
		return owner{key: funcKey(fn), decl: s.declPosition(fn.Pos())}, nil
	}
	names, err := declaredNames(decl)
	if err != nil {
		return owner{}, err
	}
	return owner{key: strings.Join(names, ", "), decl: s.declPosition(decl.Pos())}, nil
}

// holds reports whether decl's source range holds the position line.col.
func (s *sourceFile) holds(decl ast.Decl, line, col int) bool {
	start, end := s.fset.Position(decl.Pos()), s.fset.Position(decl.End())
	return !before(line, col, start.Line, start.Column) && !before(end.Line, end.Column, line, col)
}

// declaredNames returns the names the declaration at top level declares: the
// names of its values and types. A declaration of one name per spec is told
// apart from its neighbour's; a single call shared by several names is one
// entry joined by ", ", so a change that adds a name shows up as a removed and
// an added key.
func declaredNames(decl ast.Decl) ([]string, error) {
	gen, ok := decl.(*ast.GenDecl)
	if !ok {
		return nil, fmt.Errorf("unexpected declaration at %d", decl.Pos())
	}
	var names []string
	for _, spec := range gen.Specs {
		switch s := spec.(type) {
		case *ast.ValueSpec:
			var declared []string
			for _, n := range s.Names {
				declared = append(declared, n.Name)
			}
			names = append(names, strings.Join(declared, ", "))
		case *ast.TypeSpec:
			names = append(names, s.Name.Name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("the declaration declares no name")
	}
	return names, nil
}

// funcKey returns the comparison key of fn without its package: the name, or
// the receiver's base type name and the name.
func funcKey(fn *ast.FuncDecl) string {
	if recv := receiverName(fn); recv != "" {
		return recv + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// declPosition returns the position of a declaration as file:line.
func (s *sourceFile) declPosition(pos token.Pos) string {
	p := s.fset.Position(pos)
	return fmt.Sprintf("%s:%d", p.Filename, p.Line)
}

// before reports whether line1.col1 comes before line2.col2.
func before(line1, col1, line2, col2 int) bool {
	return line1 < line2 || line1 == line2 && col1 < col2
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
		return nil, fmt.Errorf("git show %s:%s: %v", commit, rel, err)
	}
	return out, nil
}

// stop prints a failure that makes the comparison impossible and exits 2.
func stop(what string, err error) {
	fmt.Fprintf(os.Stderr, "coverage: %s: %v\n", what, err)
	os.Exit(2)
}
