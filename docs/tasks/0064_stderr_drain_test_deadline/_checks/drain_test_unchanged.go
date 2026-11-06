// Command _checks verifies a before/after relation of task 0064: the task
// changes only the deadline constant of TestCommandExecutorDrainsStderr in
// internal/transcript/exec_test.go, and the new deadline is 30 seconds.
//
// For each of two commits it reads the test function with go/parser, without
// comments. The deadline constant is the identifier passed as the second
// argument of the function's single context.WithTimeout call; its declaration
// is removed from the function's const declaration, and every reference to it
// is renamed to one placeholder. The two functions are then printed and
// compared as Go token sequences, so a different line layout or comment does
// not count as a difference, while a changed burst size, script, check, or
// failure message does. It also checks that the deadline constant of the
// second commit is declared as 30 * time.Second.
//
// Run it on the PR branch, after every change of the PR is committed:
//
//	go run ./docs/tasks/0064_stderr_drain_test_deadline/_checks/ [<before> <after>]
//
// The commits default to the merge-base of origin/main and HEAD, and HEAD. It
// prints both commit IDs and the result of each check, and exits 1 when a
// check fails and 2 when it cannot run. make test and CI do not run it. The
// directory starts with an underscore, which keeps it out of go list.
package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const (
	testFile = "internal/transcript/exec_test.go"
	testName = "TestCommandExecutorDrainsStderr"
	// placeholder replaces the deadline constant's name in both commits, so a
	// rename alone is not a difference.
	placeholder = "deadlinePlaceholder"
	wantValue   = "30 * time.Second"
)

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "drain_test_unchanged:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func run(args []string) (bool, error) {
	var before, after string
	switch len(args) {
	case 0:
		before, after = "origin/main", "HEAD"
		base, err := git("merge-base", before, after)
		if err != nil {
			return false, err
		}
		before = base
	case 2:
		before, after = args[0], args[1]
	default:
		return false, errors.New("usage: drain_test_unchanged [<before> <after>]")
	}
	beforeID, err := git("rev-parse", "--verify", before+"^{commit}")
	if err != nil {
		return false, err
	}
	afterID, err := git("rev-parse", "--verify", after+"^{commit}")
	if err != nil {
		return false, err
	}
	fmt.Printf("before %s\nafter  %s\n", beforeID, afterID)

	beforeTokens, _, err := normalizedTest(beforeID)
	if err != nil {
		return false, fmt.Errorf("before: %w", err)
	}
	afterTokens, afterValue, err := normalizedTest(afterID)
	if err != nil {
		return false, fmt.Errorf("after: %w", err)
	}

	ok := true
	if slices.Equal(beforeTokens, afterTokens) {
		fmt.Printf("OK   %s is the same apart from the deadline constant\n", testName)
	} else {
		ok = false
		i := 0
		for i < len(beforeTokens) && i < len(afterTokens) && beforeTokens[i] == afterTokens[i] {
			i++
		}
		fmt.Printf("FAIL %s differs apart from the deadline constant, from token %d: before %q, after %q\n",
			testName, i, tokenAt(beforeTokens, i), tokenAt(afterTokens, i))
	}
	if afterValue == wantValue {
		fmt.Printf("OK   the deadline constant of the after commit is %s\n", wantValue)
	} else {
		ok = false
		fmt.Printf("FAIL the deadline constant of the after commit is %s, want %s\n", afterValue, wantValue)
	}
	return ok, nil
}

// normalizedTest returns the token sequence of the test function at commit,
// with the deadline constant's declaration removed and its references renamed
// to placeholder, and the expression the constant is declared as.
func normalizedTest(commit string) ([]string, string, error) {
	src, err := git("show", commit+":"+testFile)
	if err != nil {
		return nil, "", err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, testFile, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, "", err
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Recv == nil && d.Name.Name == testName {
			fn = d
		}
	}
	if fn == nil || fn.Body == nil {
		return nil, "", fmt.Errorf("%s not found in %s", testName, testFile)
	}
	name, err := deadlineName(fn)
	if err != nil {
		return nil, "", err
	}
	value, err := removeConst(fn, name)
	if err != nil {
		return nil, "", err
	}
	ast.Inspect(fn, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			id.Name = placeholder
		}
		return true
	})
	var out strings.Builder
	if err := format.Node(&out, fset, fn); err != nil {
		return nil, "", err
	}
	tokens, err := tokenize(out.String())
	if err != nil {
		return nil, "", err
	}
	return tokens, value, nil
}

// deadlineName returns the identifier passed as the second argument of the
// function's only context.WithTimeout call.
func deadlineName(fn *ast.FuncDecl) (string, error) {
	var names []string
	var bad error
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "WithTimeout" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "context" {
			return true
		}
		if len(call.Args) != 2 {
			bad = errors.New("context.WithTimeout call without two arguments")
			return true
		}
		id, ok := call.Args[1].(*ast.Ident)
		if !ok {
			bad = fmt.Errorf("the deadline of context.WithTimeout is %s, not a constant name", types.ExprString(call.Args[1]))
			return true
		}
		names = append(names, id.Name)
		return true
	})
	if bad != nil {
		return "", bad
	}
	if len(names) != 1 {
		return "", fmt.Errorf("found %d context.WithTimeout calls, want 1", len(names))
	}
	return names[0], nil
}

// removeConst removes the declaration of name from the const declarations at
// the top level of the function body and returns its value expression. The
// name must be declared there exactly once, alone in its spec.
func removeConst(fn *ast.FuncDecl, name string) (string, error) {
	var value string
	found := 0
	var stmts []ast.Stmt
	for _, stmt := range fn.Body.List {
		var g *ast.GenDecl
		if decl, ok := stmt.(*ast.DeclStmt); ok {
			g, _ = decl.Decl.(*ast.GenDecl)
		}
		if g == nil || g.Tok != token.CONST {
			stmts = append(stmts, stmt)
			continue
		}
		var specs []ast.Spec
		for _, spec := range g.Specs {
			vs := spec.(*ast.ValueSpec)
			if !slices.ContainsFunc(vs.Names, func(id *ast.Ident) bool { return id.Name == name }) {
				specs = append(specs, spec)
				continue
			}
			if len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Type != nil {
				return "", fmt.Errorf("the declaration of %s is not a single untyped constant", name)
			}
			found++
			value = types.ExprString(vs.Values[0])
		}
		if len(specs) == 0 {
			continue
		}
		g.Specs = specs
		if len(specs) == 1 {
			g.Lparen = token.NoPos
		}
		stmts = append(stmts, stmt)
	}
	if found != 1 {
		return "", fmt.Errorf("the deadline constant %s is declared %d times in the function's const declarations, want 1", name, found)
	}
	fn.Body.List = stmts
	return value, nil
}

// tokenize returns the Go tokens of src, leaving out the semicolons the
// scanner inserts at line ends, so the line layout does not matter.
func tokenize(src string) ([]string, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	var errs scanner.ErrorList
	s.Init(file, []byte(src), func(pos token.Position, msg string) { errs.Add(pos, msg) }, 0)
	var tokens []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue
		}
		if lit == "" {
			lit = tok.String()
		}
		tokens = append(tokens, tok.String()+" "+lit)
	}
	if errs.Len() > 0 {
		return nil, errs.Err()
	}
	return tokens, nil
}

func tokenAt(tokens []string, i int) string {
	if i < len(tokens) {
		return tokens[i]
	}
	return "(end)"
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		var stderr string
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr)
	}
	if args[0] == "show" {
		return string(out), nil
	}
	return strings.TrimSpace(string(out)), nil
}
