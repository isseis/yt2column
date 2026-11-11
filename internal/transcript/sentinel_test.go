//go:build test

package transcript

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestExportedSentinelErrorsUnchanged fixes the exported sentinel errors of the
// package. A new sentinel changes the error contract that callers branch on
// with errors.Is, so adding one must revisit the callers and this guard.
func TestExportedSentinelErrorsUnchanged(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	want := []string{
		"ErrCopiedTranscriptFormat",
		"ErrInvalidVideoURL",
		"ErrNoSubtitles",
		"ErrParseInfo",
		"ErrParseSubtitles",
		"ErrYtDlpExec",
	}

	var got []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, _ := parsePackageFile(t, name)
		got = append(got, exportedErrNames(file)...)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("exported sentinel errors = %v, want %v; a new sentinel changes the error contract callers branch on, so revisit the callers and this guard", got, want)
	}
}

// exportedErrNames returns the package-level var and const identifiers of the
// file that are exported and start with "Err".
func exportedErrNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.VAR && gen.Tok != token.CONST) {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if ast.IsExported(name.Name) && strings.HasPrefix(name.Name, "Err") {
					names = append(names, name.Name)
				}
			}
		}
	}
	return names
}

// parsePackageFile parses a source file of the package under test. The test
// working directory is the package directory.
func parsePackageFile(t *testing.T, name string) (*ast.File, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return file, fset
}
