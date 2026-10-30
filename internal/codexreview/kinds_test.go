//go:build test

package codexreview

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// declaredConstants parses the package's non-test sources and returns the
// string values of the constants declared with type Kind, and of those whose
// name starts with "detail" or "Detail" and whose value is a string literal.
func declaredConstants(t *testing.T) (kinds []Kind, details []string) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		fileName := entry.Name()
		if !strings.HasSuffix(fileName, ".go") || strings.HasSuffix(fileName, "_test.go") || strings.HasPrefix(fileName, "test_") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), fileName, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					if typ, ok := vs.Type.(*ast.Ident); ok && typ.Name == "Kind" {
						kinds = append(kinds, Kind(value))
					} else if strings.HasPrefix(strings.ToLower(name.Name), "detail") {
						details = append(details, value)
					}
				}
			}
		}
	}
	return kinds, details
}

// TestKindsListsEveryConstant: Kinds lists every constant declared with type
// Kind, and every string constant named detail* or Detail*, each under some kind, so a
// kind or token declared that way and not listed fails here before the guard
// that checks the shared procedure's table against Kinds can miss it. Which
// kind a token is listed under is not checked here; the procedure acts on one
// token, dirty_tree, whose kind TestRunPreconditions shows. The exit_status_
// prefix is listed in its <n> form.
func TestKindsListsEveryConstant(t *testing.T) {
	declaredKinds, declaredDetails := declaredConstants(t)
	if len(declaredKinds) == 0 || len(declaredDetails) == 0 {
		t.Fatalf("found %d kinds and %d details; the parse missed the declarations", len(declaredKinds), len(declaredDetails))
	}
	catalog := Kinds()
	gotKinds := slices.Sorted(maps.Keys(catalog))
	slices.Sort(declaredKinds)
	if !slices.Equal(gotKinds, declaredKinds) {
		t.Errorf("Kinds() keys = %q, want the declared kinds %q", gotKinds, declaredKinds)
	}
	var gotDetails []string
	for _, details := range catalog {
		gotDetails = append(gotDetails, details...)
	}
	slices.Sort(gotDetails)
	wantDetails := make([]string, 0, len(declaredDetails))
	for _, d := range declaredDetails {
		if d == detailExitStatusPrefix {
			d = detailExitStatus
		}
		wantDetails = append(wantDetails, d)
	}
	slices.Sort(wantDetails)
	if !slices.Equal(gotDetails, wantDetails) {
		t.Errorf("Kinds() details = %q, want each declared detail once: %q", gotDetails, wantDetails)
	}
}
