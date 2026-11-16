//go:build test

package llmhttptest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Func is one function or method declared in a Go file of a package, read
// from the source without building the package. It exists for the checks that
// a js/wasm-only line of code is reached from every send path: a native test
// cannot run a file that only a js build compiles, but it can read the file.
type Func struct {
	// Name is the declared name, without the receiver.
	Name string
	// File is the base name of the file that declares it.
	File string
	body *ast.BlockStmt
}

// ParseDir reads every non-test Go file of dir and returns the functions and
// methods they declare, whatever their build constraints are, so a file that
// only a js build compiles is read too. Several files may declare the same
// name under different constraints; each declaration is returned.
func ParseDir(t testing.TB, dir string) []Func {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var funcs []Func
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				funcs = append(funcs, Func{Name: fn.Name.Name, File: name, body: fn.Body})
			}
		}
	}
	return funcs
}

// Calls reports whether the function body calls a function or method named
// name: `name(...)` or `x.name(...)`. Calls inside a function literal count.
func (f Func) Calls(name string) bool {
	found := false
	ast.Inspect(f.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			found = found || fun.Name == name
		case *ast.SelectorExpr:
			found = found || fun.Sel.Name == name
		}
		return true
	})
	return found
}

// SetsHeader reports whether the function body calls `<x>.Header.Set(name,
// value)` with the two string literals given.
func (f Func) SetsHeader(name, value string) bool {
	found := false
	ast.Inspect(f.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		set, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || set.Sel.Name != "Set" {
			return true
		}
		if header, ok := set.X.(*ast.SelectorExpr); !ok || header.Sel.Name != "Header" {
			return true
		}
		found = found || literalIs(call.Args[0], name) && literalIs(call.Args[1], value)
		return true
	})
	return found
}

// literalIs reports whether expr is a string literal with the given value.
func literalIs(expr ast.Expr, want string) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	got, err := strconv.Unquote(lit.Value)
	return err == nil && got == want
}

// Callers returns the sorted names of the functions in funcs that call name.
// A name declared more than once appears once per declaration.
func Callers(funcs []Func, name string) []string {
	var names []string
	for _, f := range funcs {
		if f.Calls(name) {
			names = append(names, f.Name)
		}
	}
	slices.Sort(names)
	return names
}

// Named returns the declarations of name in funcs.
func Named(funcs []Func, name string) []Func {
	var out []Func
	for _, f := range funcs {
		if f.Name == name {
			out = append(out, f)
		}
	}
	return out
}
