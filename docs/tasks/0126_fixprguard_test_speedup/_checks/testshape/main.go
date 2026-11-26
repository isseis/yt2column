//go:build test

// Command testshape is a temporary AST verification script for task 0126.
// It checks test parallelism, process-state isolation, environment-variable
// sourcing, and git fixture sequences across internal/fixprguard and
// cmd/fixprguard.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	pkgInternal = "internal/fixprguard"
	pkgCmd      = "cmd/fixprguard"
	pkgTestHelp = "internal/fixprguard/fixprguardtest"
)

type funcKey struct {
	pkg  string
	name string
}

// serialTests is the complete list of tests allowed to stay serial (02_architecture.md section 4).
var serialTests = map[funcKey]struct{}{
	{pkgInternal, "TestGitEnvKeepsGitOnTheWorktree"}: {},
	{pkgInternal, "TestWorktreeIgnoresProcessDir"}:   {},
	{pkgCmd, "TestGhTargetEnvRejected"}:              {},
	{pkgCmd, "TestGhRunsWithoutGitVariables"}:        {},
	{pkgCmd, "TestListRunnerRejectsStderr"}:          {},
	{pkgCmd, "TestRunInProcessDir"}:                  {},
}

// serialHelpers lists helper functions used exclusively by serialTests.
var serialHelpers = map[funcKey]struct{}{}

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: testshape <parallel|process-state|env-source|fixture-git> [args...]")
	}
	var err error
	switch os.Args[1] {
	case "parallel":
		if len(os.Args) != 2 {
			fatalf("parallel takes no arguments")
		}
		err = checkParallel()
	case "process-state":
		if len(os.Args) != 2 {
			fatalf("process-state takes no arguments")
		}
		err = checkProcessState()
	case "env-source":
		if len(os.Args) != 2 {
			fatalf("env-source takes no arguments")
		}
		err = checkEnvSource()
	case "fixture-git":
		if len(os.Args) != 3 {
			fatalf("usage: testshape fixture-git <before-commit>")
		}
		err = checkFixtureGit(os.Args[2])
	default:
		fatalf("unknown subcommand: %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "testshape %s: FAIL: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	fmt.Printf("testshape %s: PASS\n", os.Args[1])
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "testshape: ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func listGoFiles(dir string, pred func(name string) bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if pred(e.Name()) {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	slices.Sort(out)
	return out, nil
}

func parseGoFile(fset *token.FileSet, path string) (*ast.File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(fset, path, src, 0)
}

func isTestFunc(fn *ast.FuncDecl) (tParam string, ok bool) {
	if fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Body == nil {
		return "", false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return "", false
	}
	field := fn.Type.Params.List[0]
	if len(field.Names) > 1 {
		return "", false
	}
	if !isTestingTType(field.Type) {
		return "", false
	}
	if len(field.Names) == 0 || field.Names[0].Name == "_" {
		return "", true
	}
	return field.Names[0].Name, true
}

func blockCallsParallel(body *ast.BlockStmt, tName string) bool {
	if tName == "" {
		return false
	}
	for _, stmt := range body.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Parallel" {
			continue
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == tName {
			return true
		}
	}
	return false
}

// anyCallToParallel reports whether body contains any call to tName.Parallel()
// anywhere inside it (including inside branches), excluding nested t.Run
// callbacks whose own *testing.T is checked by checkSubtests.
func anyCallToParallel(body *ast.BlockStmt, tName string) bool {
	if tName == "" {
		return false
	}
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Parallel" {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == tName {
			found = true
			return false
		}
		return true
	})
	return found
}

func checkParallel() error {
	fset := token.NewFileSet()
	seenSerial := make(map[funcKey]bool)
	var errs []string

	for _, pkg := range []string{pkgInternal, pkgCmd} {
		files, err := listGoFiles(pkg, func(name string) bool {
			return strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "test_helpers")
		})
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("no _test.go files in %s", pkg)
		}
		testCount := 0
		for _, path := range files {
			file, err := parseGoFile(fset, path)
			if err != nil {
				return err
			}
			isTestFile := strings.HasSuffix(path, "_test.go")
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if !isTestFile {
					if fn.Body != nil {
						checkHelperSubtests(fset, pkg, fn, &errs)
					}
					continue
				}
				tName, ok := isTestFunc(fn)
				if !ok {
					if fn.Body != nil {
						checkHelperSubtests(fset, pkg, fn, &errs)
					}
					continue
				}
				testCount++
				key := funcKey{pkg, fn.Name.Name}
				_, isSerial := serialTests[key]
				hasParallel := blockCallsParallel(fn.Body, tName)
				if isSerial {
					seenSerial[key] = true
					if anyCallToParallel(fn.Body, tName) {
						errs = append(errs, fmt.Sprintf("%s: serial-listed test %s calls %s.Parallel()", fset.Position(fn.Pos()), fn.Name.Name, tName))
					}
				} else if !hasParallel {
					errs = append(errs, fmt.Sprintf("%s: test %s.%s missing t.Parallel()", fset.Position(fn.Pos()), pkg, fn.Name.Name))
				}
				tNames := make(map[string]struct{})
				if tName != "" {
					tNames[tName] = struct{}{}
				}
				checkSubtests(fset, pkg, fn.Name.Name, fn.Body, tNames, isSerial, &errs)
			}
		}
		if testCount == 0 {
			return fmt.Errorf("no Test functions found in %s", pkg)
		}
	}

	for key := range serialTests {
		if !seenSerial[key] {
			errs = append(errs, fmt.Sprintf("serial-listed test %s.%s not found", key.pkg, key.name))
		}
	}

	if len(errs) > 0 {
		return errors.New("\n  " + strings.Join(errs, "\n  "))
	}
	return nil
}

func testingTParamNames(params *ast.FieldList) []string {
	if params == nil {
		return nil
	}
	var names []string
	for _, field := range params.List {
		if isTestingTType(field.Type) {
			for _, ident := range field.Names {
				if ident.Name != "_" {
					names = append(names, ident.Name)
				}
			}
		}
	}
	return names
}

func isTestingTType(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "T" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == "testing"
}

func checkHelperSubtests(fset *token.FileSet, pkg string, fn *ast.FuncDecl, errs *[]string) {
	tNames := make(map[string]struct{})
	for _, n := range testingTParamNames(fn.Type.Params) {
		tNames[n] = struct{}{}
	}
	if len(tNames) == 0 {
		return
	}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, isT := tNames[ident.Name]; isT {
			*errs = append(*errs, fmt.Sprintf("%s: helper %s.%s calls %s.Run (subtests in helpers are unsupported)", fset.Position(call.Pos()), pkg, fn.Name.Name, ident.Name))
			return false
		}
		return true
	})
}

func checkSubtests(fset *token.FileSet, pkg, topName string, body *ast.BlockStmt, tNames map[string]struct{}, isSerial bool, errs *[]string) {
	ast.Inspect(body, func(node ast.Node) bool {
		if lit, ok := node.(*ast.FuncLit); ok && lit.Body != body {
			// Track any *testing.T parameter names introduced by helper closures.
			extra := testingTParamNames(lit.Type.Params)
			if len(extra) > 0 {
				next := make(map[string]struct{}, len(tNames)+len(extra))
				for k := range tNames {
					next[k] = struct{}{}
				}
				for _, n := range extra {
					next[n] = struct{}{}
				}
				checkSubtests(fset, pkg, topName, lit.Body, next, isSerial, errs)
				return false
			}
			return true
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Run" {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, isT := tNames[ident.Name]; !isT {
			return true
		}
		if len(call.Args) != 2 {
			*errs = append(*errs, fmt.Sprintf("%s: %s.%s t.Run has %d args, want 2", fset.Position(call.Pos()), pkg, topName, len(call.Args)))
			return false
		}
		lit, ok := call.Args[1].(*ast.FuncLit)
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: %s.%s t.Run callback is not a function literal", fset.Position(call.Pos()), pkg, topName))
			return false
		}
		if lit.Type.Params == nil || len(lit.Type.Params.List) != 1 || len(lit.Type.Params.List[0].Names) > 1 {
			*errs = append(*errs, fmt.Sprintf("%s: %s.%s t.Run callback has unexpected params", fset.Position(lit.Pos()), pkg, topName))
			return false
		}
		subT := ""
		if len(lit.Type.Params.List[0].Names) == 1 && lit.Type.Params.List[0].Names[0].Name != "_" {
			subT = lit.Type.Params.List[0].Names[0].Name
		}
		hasParallel := blockCallsParallel(lit.Body, subT)
		if isSerial {
			if hasParallel {
				*errs = append(*errs, fmt.Sprintf("%s: subtest under serial test %s.%s calls %s.Parallel()", fset.Position(lit.Pos()), pkg, topName, subT))
			}
		} else if !hasParallel {
			*errs = append(*errs, fmt.Sprintf("%s: subtest under %s.%s missing t.Parallel()", fset.Position(lit.Pos()), pkg, topName))
		}
		next := make(map[string]struct{}, len(tNames)+1)
		for k := range tNames {
			next[k] = struct{}{}
		}
		if subT != "" {
			next[subT] = struct{}{}
		}
		checkSubtests(fset, pkg, topName, lit.Body, next, isSerial, errs)
		return false
	})
}

type pkgFile struct {
	pkg  string
	path string
	file *ast.File
}

func loadTestAndHelperFiles(fset *token.FileSet) ([]pkgFile, error) {
	var all []pkgFile
	for _, pkg := range []string{pkgInternal, pkgCmd} {
		files, err := listGoFiles(pkg, func(name string) bool {
			return strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "test_helpers")
		})
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no test/helper files in %s", pkg)
		}
		for _, p := range files {
			f, err := parseGoFile(fset, p)
			if err != nil {
				return nil, err
			}
			all = append(all, pkgFile{pkg: pkg, path: p, file: f})
		}
	}
	helperFiles, err := listGoFiles(pkgTestHelp, func(string) bool { return true })
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", pkgTestHelp, err)
	}
	if len(helperFiles) == 0 {
		return nil, fmt.Errorf("no .go files in %s", pkgTestHelp)
	}
	for _, p := range helperFiles {
		f, err := parseGoFile(fset, p)
		if err != nil {
			return nil, err
		}
		all = append(all, pkgFile{pkg: pkgTestHelp, path: p, file: f})
	}
	return all, nil
}

func osImportName(file *ast.File) (string, error) {
	for _, imp := range file.Imports {
		val, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return "", err
		}
		if val != "os" {
			continue
		}
		if imp.Name == nil {
			return "os", nil
		}
		if imp.Name.Name == "." {
			return "", errors.New("dot import of os is not supported")
		}
		if imp.Name.Name == "_" {
			return "", nil
		}
		return imp.Name.Name, nil
	}
	return "", nil
}

func isSerialAllowedFunc(pkg, name string) bool {
	key := funcKey{pkg, name}
	if _, ok := serialTests[key]; ok {
		return true
	}
	_, ok := serialHelpers[key]
	return ok
}

func checkProcessState() error {
	fset := token.NewFileSet()
	files, err := loadTestAndHelperFiles(fset)
	if err != nil {
		return err
	}
	var errs []string
	for _, pf := range files {
		osName, err := osImportName(pf.file)
		if err != nil {
			return fmt.Errorf("%s: %w", pf.path, err)
		}
		for _, decl := range pf.file.Decls {
			fnName := ""
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				fnName = fn.Name.Name
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				sel, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Chdir", "Setenv", "Unsetenv":
				default:
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				isOSCall := ok && osName != "" && ident.Name == osName
				if sel.Sel.Name == "Unsetenv" && !isOSCall {
					return true
				}
				if pf.pkg == pkgTestHelp && fnName == "IsolateWorkingDir" && isOSCall && sel.Sel.Name == "Chdir" {
					return true
				}
				if isSerialAllowedFunc(pf.pkg, fnName) {
					return true
				}
				errs = append(errs, fmt.Sprintf("%s: disallowed %s.%s in %s.%s", fset.Position(sel.Pos()), exprText(sel.X), sel.Sel.Name, pf.pkg, fnName))
				return true
			})
		}
	}
	if len(errs) > 0 {
		return errors.New("\n  " + strings.Join(errs, "\n  "))
	}
	return nil
}

func checkEnvSource() error {
	fset := token.NewFileSet()
	files, err := loadTestAndHelperFiles(fset)
	if err != nil {
		return err
	}
	var errs []string
	for _, pf := range files {
		osName, err := osImportName(pf.file)
		if err != nil {
			return fmt.Errorf("%s: %w", pf.path, err)
		}
		if osName == "" {
			continue
		}
		for _, decl := range pf.file.Decls {
			fnName := ""
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				fnName = fn.Name.Name
			}
			// Track os.Environ() nodes that appear specifically on the RHS of a
			// cmd.Env assignment inside TestGuideIgnoredCommands (the shell block).
			allowedCmdEnvNodes := make(map[ast.Node]bool)
			if pf.pkg == pkgCmd && filepath.Base(pf.path) == "docs_test.go" && fnName == "TestGuideIgnoredCommands" {
				ast.Inspect(decl, func(n ast.Node) bool {
					assign, ok := n.(*ast.AssignStmt)
					if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
						return true
					}
					if exprText(assign.Lhs[0]) != "cmd.Env" {
						return true
					}
					ast.Inspect(assign.Rhs[0], func(rn ast.Node) bool {
						if sel, ok := rn.(*ast.SelectorExpr); ok && sel.Sel.Name == "Environ" {
							if id, ok := sel.X.(*ast.Ident); ok && id.Name == osName {
								allowedCmdEnvNodes[sel] = true
							}
						}
						return true
					})
					return true
				})
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				sel, ok := node.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Environ" {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != osName {
					return true
				}
				if pf.pkg == pkgTestHelp && fnName == "Env" {
					return true
				}
				if allowedCmdEnvNodes[sel] {
					return true
				}
				if isSerialAllowedFunc(pf.pkg, fnName) {
					return true
				}
				errs = append(errs, fmt.Sprintf("%s: disallowed %s.Environ in %s.%s", fset.Position(sel.Pos()), osName, pf.pkg, fnName))
				return true
			})
		}
	}
	if len(errs) > 0 {
		return errors.New("\n  " + strings.Join(errs, "\n  "))
	}
	return nil
}

func exprText(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprText(x.X) + "." + x.Sel.Name
	default:
		return "<expr>"
	}
}

type fixtureTarget struct {
	pkg  string
	path string
}

var fixtureTargets = []fixtureTarget{
	{pkg: pkgInternal, path: "internal/fixprguard/test_helpers_git.go"},
	{pkg: pkgCmd, path: "cmd/fixprguard/test_helpers.go"},
}

func checkFixtureGit(beforeRef string) error {
	commit, err := gitOutput("rev-parse", "--verify", beforeRef+"^{commit}")
	if err != nil {
		return fmt.Errorf("resolve %s: %w", beforeRef, err)
	}
	commit = strings.TrimSpace(commit)
	head, err := gitOutput("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}
	head = strings.TrimSpace(head)
	fmt.Printf("before=%s head=%s\n", commit, head)

	for _, target := range fixtureTargets {
		beforeSrc, err := gitOutput("show", commit+":"+target.path)
		if err != nil {
			return fmt.Errorf("git show %s:%s: %w", commit, target.path, err)
		}
		afterSrc, err := os.ReadFile(target.path)
		if err != nil {
			return err
		}

		beforeCalls, err := extractGitSeq(target.path+"@before", []byte(beforeSrc), "newTestRepo")
		if err != nil {
			return fmt.Errorf("%s before newTestRepo: %w", target.pkg, err)
		}
		buildEntry, err := resolveBuildEntry(target.path+"@after", afterSrc)
		if err != nil {
			return fmt.Errorf("%s after: %w", target.pkg, err)
		}
		buildCalls, err := extractGitSeq(target.path+"@after", afterSrc, buildEntry)
		if err != nil {
			return fmt.Errorf("%s after %s: %w", target.pkg, buildEntry, err)
		}
		afterNewCalls, err := extractGitSeq(target.path+"@after", afterSrc, "newTestRepo")
		if err != nil {
			return fmt.Errorf("%s after newTestRepo: %w", target.pkg, err)
		}

		if !equalGitSeqs(beforeCalls, buildCalls) {
			return fmt.Errorf("%s: %s git sequence differs from before newTestRepo\n  before (%d): %v\n  build  (%d): %v",
				target.pkg, buildEntry, len(beforeCalls), beforeCalls, len(buildCalls), buildCalls)
		}
		if len(afterNewCalls) >= len(beforeCalls) {
			return fmt.Errorf("%s: after newTestRepo git count (%d) is not fewer than before (%d)",
				target.pkg, len(afterNewCalls), len(beforeCalls))
		}
		fmt.Printf("package=%s before_newTestRepo=%d %s=%d after_newTestRepo=%d\n",
			target.pkg, len(beforeCalls), buildEntry, len(buildCalls), len(afterNewCalls))
	}
	return nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func equalGitSeqs(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

type fixtureExtractor struct {
	fset   *token.FileSet
	consts map[string]string
	funcs  map[string]*ast.FuncDecl
}

func resolveBuildEntry(filename string, src []byte) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return "", err
	}
	var found []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if fn.Name.Name == "build" || fn.Name.Name == "buildTestRepo" {
			found = append(found, fn.Name.Name)
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("expected exactly one of build or buildTestRepo, found %v", found)
	}
	return found[0], nil
}

func extractGitSeq(filename string, src []byte, entryFunc string) ([][]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	ex := &fixtureExtractor{
		fset:   fset,
		consts: make(map[string]string),
		funcs:  make(map[string]*ast.FuncDecl),
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok != token.CONST {
				continue
			}
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if v, err := strconv.Unquote(lit.Value); err == nil {
								ex.consts[name.Name] = v
							}
						}
					}
				}
			}
		case *ast.FuncDecl:
			ex.funcs[d.Name.Name] = d
		}
	}
	fn, ok := ex.funcs[entryFunc]
	if !ok || fn.Body == nil {
		return nil, fmt.Errorf("function %s not found", entryFunc)
	}
	env := map[string]string{
		"root":     "<root>",
		"r.Dir":    "<work>",
		"r.Remote": "<remote>",
		"r.Home":   "<home>",
		"work":     "<work>",
		"dir":      "<work>",
		"remote":   "<remote>",
		"home":     "<home>",
	}
	var calls [][]string
	if err := ex.walkBlock(fn.Body, env, &calls, 0); err != nil {
		return nil, err
	}
	return calls, nil
}

func (ex *fixtureExtractor) walkBlock(body *ast.BlockStmt, env map[string]string, calls *[][]string, depth int) error {
	if depth > 4 {
		return errors.New("helper call depth exceeded")
	}
	for _, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			if err := ex.handleAssign(s, env, calls, depth); err != nil {
				return err
			}
		case *ast.ExprStmt:
			if err := ex.handleExprCall(s.X, env, calls, depth); err != nil {
				return err
			}
		case *ast.IfStmt:
			if s.Else != nil {
				return fmt.Errorf("%s: unsupported if-else in straight-line fixture", ex.fset.Position(s.Pos()))
			}
			if !isErrNotNilCheck(s.Cond) {
				return fmt.Errorf("%s: unsupported branch condition in fixture", ex.fset.Position(s.Pos()))
			}
			if !bodyOnlyFailsOrReturns(s.Body) {
				return fmt.Errorf("%s: unsupported if body in fixture", ex.fset.Position(s.Pos()))
			}
			if s.Init != nil {
				switch initStmt := s.Init.(type) {
				case *ast.AssignStmt:
					if err := ex.handleAssign(initStmt, env, calls, depth); err != nil {
						return err
					}
				default:
					return fmt.Errorf("%s: unsupported if init statement", ex.fset.Position(s.Pos()))
				}
			}
		case *ast.ReturnStmt:
			for _, res := range s.Results {
				if err := ex.handleReturnExpr(res, env, calls, depth); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("%s: unsupported statement %T in fixture", ex.fset.Position(stmt.Pos()), stmt)
		}
	}
	return nil
}

func (ex *fixtureExtractor) handleReturnExpr(expr ast.Expr, env map[string]string, calls *[][]string, depth int) error {
	switch e := expr.(type) {
	case *ast.Ident, *ast.BasicLit:
		return nil
	case *ast.CallExpr:
		if isPureErrorCall(e) {
			return nil
		}
		return ex.handleExprCall(e, env, calls, depth)
	default:
		return fmt.Errorf("%s: unsupported return expression %T in fixture", ex.fset.Position(expr.Pos()), expr)
	}
}

func isPureErrorCall(call *ast.CallExpr) bool {
	name := callName(call.Fun)
	if name != "fmt.Errorf" && name != "errors.New" {
		return false
	}
	hasNestedCall := false
	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			if _, ok := n.(*ast.CallExpr); ok {
				hasNestedCall = true
				return false
			}
			return true
		})
	}
	return !hasNestedCall
}

func isErrNotNilCheck(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	x, ok1 := bin.X.(*ast.Ident)
	y, ok2 := bin.Y.(*ast.Ident)
	return ok1 && ok2 && x.Name == "err" && y.Name == "nil"
}

func bodyOnlyFailsOrReturns(body *ast.BlockStmt) bool {
	if len(body.List) != 1 {
		return false
	}
	switch s := body.List[0].(type) {
	case *ast.ReturnStmt:
		for _, res := range s.Results {
			switch e := res.(type) {
			case *ast.Ident, *ast.BasicLit:
			case *ast.CallExpr:
				if !isPureErrorCall(e) {
					return false
				}
			default:
				return false
			}
		}
		return true
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && (sel.Sel.Name == "Fatalf" || sel.Sel.Name == "Fatal")
	default:
		return false
	}
}

func (ex *fixtureExtractor) handleAssign(s *ast.AssignStmt, env map[string]string, calls *[][]string, depth int) error {
	if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
		if ident, ok := s.Lhs[0].(*ast.Ident); ok {
			if val, ok := ex.evalPathExpr(s.Rhs[0], env); ok {
				env[ident.Name] = val
				return nil
			}
			if val, err := ex.evalString(s.Rhs[0], env); err == nil {
				env[ident.Name] = val
				return nil
			}
			delete(env, ident.Name)
			if call, ok := s.Rhs[0].(*ast.CallExpr); ok {
				return ex.handleExprCall(call, env, calls, depth)
			}
		}
	} else {
		for _, lhs := range s.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok {
				delete(env, ident.Name)
			}
		}
	}
	// Ignore struct literal setup like r := &testRepo{...} and t.TempDir() etc.,
	// unless RHS contains a git/helper call, nested call, or func literal.
	for _, rhs := range s.Rhs {
		switch r := rhs.(type) {
		case *ast.CallExpr:
			if err := ex.handleExprCall(r, env, calls, depth); err != nil {
				return err
			}
		case *ast.FuncLit:
			return fmt.Errorf("%s: unsupported func literal assignment in fixture", ex.fset.Position(r.Pos()))
		default:
			if err := ex.checkArgCalls([]ast.Expr{rhs}, env, calls, depth); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ex *fixtureExtractor) handleExprCall(expr ast.Expr, env map[string]string, calls *[][]string, depth int) error {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return fmt.Errorf("%s: unexpected expression statement %T", ex.fset.Position(expr.Pos()), expr)
	}
	name := callName(call.Fun)
	switch name {
	case "t.Helper", "r.t.Helper", "t.Setenv", "t.Chdir", "t.TempDir",
		"os.MkdirAll", "os.WriteFile", "r.Write", "writeFile",
		"fixtureTemplate.Copy", "tpl.Copy", "template.Copy":
		return ex.checkArgCalls(call.Args, env, calls, depth)
	case "r.Git", "gitCmd", "gitWork":
		args, err := ex.evalArgs(call.Args, env)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", ex.fset.Position(call.Pos()), name, err)
		}
		*calls = append(*calls, append([]string{"<work>"}, args...))
		return nil
	case "r.gitIn", "gitIn":
		if len(call.Args) < 2 {
			return fmt.Errorf("%s: %s has fewer than 2 args", ex.fset.Position(call.Pos()), name)
		}
		dir, err := ex.evalString(call.Args[0], env)
		if err != nil {
			return fmt.Errorf("%s: %s dir: %w", ex.fset.Position(call.Pos()), name, err)
		}
		args, err := ex.evalArgs(call.Args[1:], env)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", ex.fset.Position(call.Pos()), name, err)
		}
		*calls = append(*calls, append([]string{dir}, args...))
		return nil
	case "r.pinIgnoreConfig", "pinIgnoreConfig", "r.CommitAll", "commitAll":
		baseName := name
		if idx := strings.LastIndex(baseName, "."); idx >= 0 {
			baseName = baseName[idx+1:]
		}
		// For free function pinIgnoreConfig(..., gitCmd, ...), verify the
		// callback argument runs in the work tree (r.Git or gitWork).
		if name == "pinIgnoreConfig" {
			matched := false
			for _, arg := range call.Args {
				if t := exprText(arg); t == "r.Git" || t == "gitWork" {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("%s: pinIgnoreConfig git callback is not r.Git or gitWork", ex.fset.Position(call.Pos()))
			}
		}
		fn, ok := ex.funcs[baseName]
		if !ok || fn.Body == nil {
			return fmt.Errorf("%s: helper %s not found", ex.fset.Position(call.Pos()), baseName)
		}
		subEnv := make(map[string]string, len(env)+2)
		for k, v := range env {
			subEnv[k] = v
		}
		if baseName == "CommitAll" || baseName == "commitAll" {
			if len(call.Args) >= 1 {
				lastArg := call.Args[len(call.Args)-1]
				if val, err := ex.evalString(lastArg, env); err == nil {
					subEnv["message"] = val
				}
			}
		}
		return ex.walkBlock(fn.Body, subEnv, calls, depth+1)
	default:
		if strings.HasSuffix(name, ".Copy") {
			return ex.checkArgCalls(call.Args, env, calls, depth)
		}
		return fmt.Errorf("%s: unrecognized call %s in fixture", ex.fset.Position(call.Pos()), name)
	}
}

// checkArgCalls inspects call arguments for nested calls so a git invocation
// hidden inside an ignored file/helper call cannot be silently dropped.
func (ex *fixtureExtractor) checkArgCalls(args []ast.Expr, env map[string]string, calls *[][]string, depth int) error {
	for _, arg := range args {
		var nestedErr error
		ast.Inspect(arg, func(n ast.Node) bool {
			if nestedErr != nil {
				return false
			}
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			cn := callName(c.Fun)
			switch cn {
			case "filepath.Join", "filepath.Dir", "string", "[]byte", "t.TempDir":
				return true
			default:
				nestedErr = ex.handleExprCall(c, env, calls, depth)
				return false
			}
		})
		if nestedErr != nil {
			return nestedErr
		}
	}
	return nil
}

func callName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return exprText(f.X) + "." + f.Sel.Name
	case *ast.ArrayType:
		if f.Len == nil {
			if elem, ok := f.Elt.(*ast.Ident); ok && elem.Name == "byte" {
				return "[]byte"
			}
		}
		return "<unknown>"
	default:
		return "<unknown>"
	}
}

func (ex *fixtureExtractor) evalArgs(exprs []ast.Expr, env map[string]string) ([]string, error) {
	out := make([]string, 0, len(exprs))
	for _, e := range exprs {
		v, err := ex.evalString(e, env)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (ex *fixtureExtractor) evalPathExpr(e ast.Expr, env map[string]string) (string, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok || callName(call.Fun) != "filepath.Join" || len(call.Args) != 2 {
		return "", false
	}
	base, err1 := ex.evalString(call.Args[0], env)
	elem, err2 := ex.evalString(call.Args[1], env)
	if err1 != nil || err2 != nil {
		return "", false
	}
	if base == "<root>" {
		switch elem {
		case "work":
			return "<work>", true
		case "remote.git":
			return "<remote>", true
		case "home":
			return "<home>", true
		}
	}
	if base == "<home>" && elem == "excludes" {
		return "<excludes>", true
	}
	return base + "/" + elem, true
}

func (ex *fixtureExtractor) evalString(e ast.Expr, env map[string]string) (string, error) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			return strconv.Unquote(x.Value)
		}
	case *ast.Ident:
		if v, ok := env[x.Name]; ok {
			return v, nil
		}
		if v, ok := ex.consts[x.Name]; ok {
			return v, nil
		}
	case *ast.SelectorExpr:
		key := exprText(x)
		if v, ok := env[key]; ok {
			return v, nil
		}
	case *ast.CallExpr:
		if v, ok := ex.evalPathExpr(x, env); ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("cannot evaluate argument %T (%s)", e, exprText(e))
}
