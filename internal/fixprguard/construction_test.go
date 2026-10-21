//go:build test

package fixprguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restrictedConstructors names, for each type whose zero value must not be
// usable, the only functions allowed to build a value of it.
var restrictedConstructors = map[string]string{
	"Login":    "ParseLogin",
	"PRNumber": "ParsePRNumber",
	"Baseline": "newBaseline",
}

// zeroLiteralAllowed names the restricted types whose empty literal T{} may
// appear anywhere: its zero value fails usable(), which every method checks,
// so only a literal that sets fields could build a usable value.
var zeroLiteralAllowed = map[string]struct{}{"Baseline": {}}

// findRestrictedConstructions reports every composite literal (T{}, &T{}),
// new(T), and var declaration of a restricted type outside its constructor.
// A struct field, a slice element, or a named result of the type is not
// reported: those zero values are made unusable instead (NewGitHub rejects a
// zero Login, and every use checks usable()).
func findRestrictedConstructions(fset *token.FileSet, file *ast.File) []string {
	var found []string
	report := func(node ast.Node, typeName string) {
		found = append(found, fset.Position(node.Pos()).String()+": "+typeName)
	}
	check := func(scope ast.Node, owner string) {
		ast.Inspect(scope, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CompositeLit:
				name := restrictedName(n.Type)
				if _, zeroOK := zeroLiteralAllowed[name]; zeroOK && len(n.Elts) == 0 {
					return true
				}
				if name != "" && restrictedConstructors[name] != owner {
					report(n, name)
				}
			case *ast.CallExpr:
				if fn, ok := n.Fun.(*ast.Ident); ok && fn.Name == "new" && len(n.Args) == 1 {
					if name := restrictedName(n.Args[0]); name != "" && restrictedConstructors[name] != owner {
						report(n, name)
					}
				}
			case *ast.ValueSpec:
				if name := restrictedName(n.Type); name != "" && restrictedConstructors[name] != owner {
					report(n, name)
				}
			}
			return true
		})
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
			check(fn, fn.Name.Name)
			continue
		}
		check(decl, "")
	}
	return found
}

func restrictedName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	if _, restricted := restrictedConstructors[ident.Name]; !restricted {
		return ""
	}
	return ident.Name
}

func TestRestrictedTypesBuiltByConstructors(t *testing.T) {
	t.Run("package", func(t *testing.T) {
		paths, err := filepath.Glob("*.go")
		if err != nil {
			t.Fatalf("glob: %v", err)
		}
		fset := token.NewFileSet()
		checked := 0
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") || strings.HasPrefix(path, "test_helpers") {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			file, err := parser.ParseFile(fset, path, src, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			checked++
			for _, f := range findRestrictedConstructions(fset, file) {
				t.Errorf("restricted type built outside its constructor at %s", f)
			}
		}
		if checked == 0 {
			t.Fatal("checked no source files")
		}
	})

	// Each form the check claims to detect is detected when added once, and
	// the same literal inside the constructor is allowed.
	const header = "package fixprguard\n"
	for _, tc := range []struct {
		name string
		src  string
		want int
	}{
		{"constructor", "func ParseLogin(s string) (Login, error) { return Login{name: s}, nil }", 0},
		{"composite_literal", "func f() Login { return Login{} }", 1},
		{"address_of_literal", "func f() *PRNumber { return &PRNumber{n: 1} }", 1},
		{"new", "func f() *Login { return new(Login) }", 1},
		{"local_var", "func f() { var l Login; _ = l }", 1},
		{"package_var", "var zero PRNumber", 1},
		{"package_literal", "var zero = Login{name: \"x\"}", 1},
		{"other_constructor", "func ParsePRNumber(s string) (Login, error) { return Login{}, nil }", 1},
		{"method_named_like_constructor", "func (x T) ParseLogin() Login { return Login{} }", 1},
		{"field_not_reported", "type T struct{ l Login }", 0},
		{"baseline_constructor", "func newBaseline(h string) Baseline { return Baseline{head: h} }", 0},
		{"baseline_with_fields", "func ParseBaseline(h string) Baseline { return Baseline{head: h} }", 1},
		{"baseline_zero_literal", "func (w *W) Baseline() (Baseline, error) { return Baseline{}, nil }", 0},
		{"login_zero_literal_still_reported", "func (w *W) f() Login { return Login{} }", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", header+tc.src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := findRestrictedConstructions(fset, file); len(got) != tc.want {
				t.Errorf("found %v, want %d", got, tc.want)
			}
		})
	}
}
