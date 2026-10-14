//go:build test

package failure

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestNoticeFieldSet fixes the exported fields of Notice. Adding a field would
// widen what a failure post may contain, so a change here must revisit the
// notice text and the callers together.
func TestNoticeFieldSet(t *testing.T) {
	want := map[string]string{
		"RequestID": "string",
		"VideoID":   "string",
		"Kind":      "failure.Kind",
	}
	typ := reflect.TypeFor[Notice]()
	if typ.NumField() != len(want) {
		t.Errorf("Notice has %d fields, want %d", typ.NumField(), len(want))
	}
	if got := exportedFieldTypes(typ); !maps.Equal(got, want) {
		t.Errorf("Notice fields = %v, want %v", got, want)
	}
}

// TestNotifierMethodSet fixes the Notifier contract: a single method, Notify,
// with the signature func(context.Context, Notice) error.
func TestNotifierMethodSet(t *testing.T) {
	typ := reflect.TypeFor[Notifier]()
	if typ.NumMethod() != 1 {
		t.Fatalf("Notifier has %d methods, want 1", typ.NumMethod())
	}
	m := typ.Method(0)
	if m.Name != "Notify" {
		t.Errorf("Notifier method = %q, want %q", m.Name, "Notify")
	}
	if want := reflect.TypeFor[func(context.Context, Notice) error](); m.Type != want {
		t.Errorf("Notifier.Notify type = %s, want %s", m.Type, want)
	}
}

// TestKindZeroValue fixes the zero value of Kind. Notice{} must mean
// "unclassified", so KindUnknown must stay the first constant.
func TestKindZeroValue(t *testing.T) {
	var k Kind
	if k != KindUnknown {
		t.Errorf("zero Kind = %d, want KindUnknown", k)
	}
}

// TestKindConstants fixes the names and declaration order of the Kind
// constants. The list is the source of truth the failure text and the
// classification switch are written against.
func TestKindConstants(t *testing.T) {
	want := []string{
		"KindUnknown",
		"KindBodyTooLarge",
		"KindInvalidBody",
		"KindInvalidVideoURL",
		"KindInvalidTranscript",
		"KindTranscriptTooLong",
		"KindLLMTimeout",
		"KindLLMEmpty",
		"KindLLMTruncated",
		"KindLLM",
		"KindPublish",
		"KindInternal",
	}
	got := kindConstantNames(t)
	if len(got) == 0 {
		t.Fatalf("found no Kind constants in the package")
	}
	if !slices.Equal(got, want) {
		t.Errorf("Kind constants = %v, want %v", got, want)
	}
}

// kindConstantNames reads every non-test Go file in the package directory and
// returns the names of the constants declared with type Kind, in declaration
// order.
func kindConstantNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	fset := token.NewFileSet()
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		names = append(names, kindConstantsInFile(file)...)
	}
	return names
}

// kindConstantsInFile returns the names of the Kind-typed constants of one
// file, in declaration order. Within a const block a spec without a type
// repeats the previous spec's type, so the last explicit type is tracked.
func kindConstantsInFile(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		var current ast.Expr
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if vs.Type != nil {
				current = vs.Type
			}
			if !isKindType(current) {
				continue
			}
			for _, id := range vs.Names {
				names = append(names, id.Name)
			}
		}
	}
	return names
}

// isKindType reports whether expr names the Kind type.
func isKindType(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "Kind"
}

func exportedFieldTypes(t reflect.Type) map[string]string {
	fields := make(map[string]string, t.NumField())
	for f := range t.Fields() {
		if f.IsExported() {
			fields[f.Name] = f.Type.String()
		}
	}
	return fields
}
