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
var restrictedConstructors = map[string]map[string]struct{}{
	"Login":       {"ParseLogin": {}},
	"PRNumber":    {"ParsePRNumber": {}},
	"Baseline":    {"newBaseline": {}},
	"Reviewers":   {"ParseReviewers": {}},
	"WorktreeDir": {"CurrentDir": {}, "NewWorktreeDir": {}},
}

// zeroLiteralAllowed names the restricted types whose empty literal T{} may
// appear anywhere: its zero value fails usable(), which every method checks,
// so only a literal that sets fields could build a usable value.
var zeroLiteralAllowed = map[string]struct{}{"Baseline": {}}

// restrictedField describes a struct field whose references are restricted to
// a closed set of functions and methods (constructors and designated methods).
// Code outside those accessors cannot fill, widen, or mutate the field, and no
// other struct may declare a field of the same name (which would convert to
// the restricted type).
type restrictedField struct {
	typeName  string
	accessors map[string]struct{}
}

// restrictedFields maps each restricted field name to its owning type and
// allowed accessors.
var restrictedFields = map[string]restrictedField{
	"logins": {
		typeName: "Reviewers",
		accessors: map[string]struct{}{
			"ParseReviewers":     {},
			"Reviewers.contains": {},
			"Reviewers.usable":   {},
		},
	},
	"rootDir": {
		typeName: "WorktreeDir",
		accessors: map[string]struct{}{
			"CurrentDir":                  {},
			"NewWorktreeDir":              {},
			"WorktreeDir.usable":          {},
			"WorktreeDir.joinName":        {},
			"WorktreeDir.resolveLocation": {},
			"WorktreeDir.relName":         {},
		},
	},
}

// restrictedFieldOwners names the types whose declarations own a field in
// restrictedFields.
var restrictedFieldOwners = map[string]struct{}{
	"Reviewers":   {},
	"WorktreeDir": {},
}

func constructorAllowed(typeName, owner string) bool {
	_, ok := restrictedConstructors[typeName][owner]
	return ok
}

func fieldAllowed(rf restrictedField, accessor string) bool {
	_, ok := rf.accessors[accessor]
	return ok
}

// findRestrictedConstructions reports every composite literal (T{}, &T{}),
// new(T), and var declaration of a restricted type outside its constructors,
// every reference to a restricted field outside its accessors, every alias
// or type defined from a restricted type, and every other field named like
// a restricted field (a same-shape struct converts to the restricted type).
// A struct field, a slice element, or a named result of the type is not
// reported: those zero values are made unusable instead (NewGitHub rejects a
// zero Reviewers, NewWorktree panics on a zero WorktreeDir, and every use
// checks usable()).
func findRestrictedConstructions(fset *token.FileSet, file *ast.File) []string {
	var found []string
	report := func(node ast.Node, typeName string) {
		found = append(found, fset.Position(node.Pos()).String()+": "+typeName)
	}
	// owner is the plain function's name, or "" for a method, so a method
	// never counts as a constructor; accessor is the function's name or
	// "Recv.method" for a method.
	check := func(scope ast.Node, owner, accessor string) {
		ast.Inspect(scope, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.SelectorExpr:
				if rf, ok := restrictedFields[n.Sel.Name]; ok && !fieldAllowed(rf, accessor) {
					report(n, rf.typeName+"."+n.Sel.Name)
				}
			case *ast.KeyValueExpr:
				if key, ok := n.Key.(*ast.Ident); ok {
					if rf, restricted := restrictedFields[key.Name]; restricted && !fieldAllowed(rf, accessor) {
						report(n, rf.typeName+"."+key.Name)
					}
				}
			case *ast.CompositeLit:
				name := restrictedName(n.Type)
				if _, zeroOK := zeroLiteralAllowed[name]; zeroOK && len(n.Elts) == 0 {
					return true
				}
				if name != "" && !constructorAllowed(name, owner) {
					report(n, name)
				}
			case *ast.CallExpr:
				if fn, ok := n.Fun.(*ast.Ident); ok && fn.Name == "new" && len(n.Args) == 1 {
					if name := restrictedName(n.Args[0]); name != "" && !constructorAllowed(name, owner) {
						report(n, name)
					}
				}
			case *ast.ValueSpec:
				if name := restrictedName(n.Type); name != "" && !constructorAllowed(name, owner) {
					report(n, name)
				}
			case *ast.TypeSpec:
				// The declaration of a type owning a restricted field is where
				// its field lives.
				if _, ok := restrictedFieldOwners[n.Name.Name]; ok {
					return false
				}
				// An alias or a type defined from a restricted type would
				// let a literal of the new name slip past restrictedName.
				if name := restrictedName(n.Type); name != "" {
					report(n, name)
				}
			case *ast.Field:
				// A struct of the same shape converts to the restricted type,
				// so no other declaration may have a field of the same name.
				for _, ident := range n.Names {
					if rf, ok := restrictedFields[ident.Name]; ok && !fieldAllowed(rf, accessor) {
						report(n, rf.typeName+"."+ident.Name)
					}
				}
			}
			return true
		})
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		switch {
		case !ok:
			check(decl, "", "")
		case fn.Recv == nil:
			check(fn, fn.Name.Name, fn.Name.Name)
		default:
			check(fn, "", receiverName(fn.Recv)+"."+fn.Name.Name)
		}
	}
	return found
}

// receiverName returns the type name of a method receiver (T or *T).
func receiverName(recv *ast.FieldList) string {
	if len(recv.List) != 1 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
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
		{"reviewers_constructor", "func ParseReviewers(n []string) (Reviewers, error) { return Reviewers{logins: nil}, nil }", 0},
		{"reviewers_zero_literal", "func f() Reviewers { return Reviewers{} }", 1},
		{"reviewers_literal_with_field", "func f(m map[Login]struct{}) Reviewers { return Reviewers{logins: m} }", 2},
		{"reviewers_address_of_literal", "func f() *Reviewers { return &Reviewers{} }", 1},
		{"reviewers_new", "func f() *Reviewers { return new(Reviewers) }", 1},
		{"reviewers_var", "func f() { var r Reviewers; _ = r }", 1},
		{"reviewers_in_other_constructor", "func ParseLogin(s string) (Login, error) { _ = Reviewers{}; return Login{name: s}, nil }", 1},
		{"reviewers_field_contains", "func (r Reviewers) contains(l Login) bool { _, ok := r.logins[l]; return ok }", 0},
		{"reviewers_field_usable", "func (r *Reviewers) usable() bool { return len(r.logins) > 0 }", 0},
		// The field rows below share one check; they list the ways of
		// filling or widening the set that it must keep closed.
		{"reviewers_field_named_result", "func f() (r Reviewers) { r.logins = map[Login]struct{}{}; return }", 1},
		{"reviewers_field_element", "func f(r Reviewers, l Login) { r.logins[l] = struct{}{} }", 1},
		{"reviewers_field_delete", "func f(r Reviewers, l Login) { delete(r.logins, l) }", 1},
		{"reviewers_field_alias", "func f(r Reviewers) { m := r.logins; _ = m }", 1},
		{"reviewers_field_read", "func f(r Reviewers) int { return len(r.logins) }", 1},
		{"reviewers_field_other_method", "func (r Reviewers) add(l Login) { r.logins[l] = struct{}{} }", 1},
		{"reviewers_field_method_of_other_type", "func (w W) contains(r Reviewers) int { return len(r.logins) }", 1},
		{"reviewers_declaration", "type Reviewers struct{ logins map[Login]struct{} }", 0},
		{"reviewers_alias", "type rv = Reviewers", 1},
		{"reviewers_defined_type", "type rv Reviewers", 1},
		{"login_alias", "type name = Login", 1},
		{"same_shape_struct", "type shape struct{ logins map[Login]struct{} }", 1},
		{"worktree_dir_current_dir", "func CurrentDir() WorktreeDir { return WorktreeDir{rootDir: \".\"} }", 0},
		{"worktree_dir_new_constructor", "func NewWorktreeDir(p string) (WorktreeDir, error) { return WorktreeDir{rootDir: p}, nil }", 0},
		{"worktree_dir_zero_literal", "func f() WorktreeDir { return WorktreeDir{} }", 1},
		{"worktree_dir_literal_with_field", "func f() WorktreeDir { return WorktreeDir{rootDir: \"/tmp\"} }", 2},
		{"worktree_dir_field_read", "func f(d WorktreeDir) string { return d.rootDir }", 1},
		{"worktree_dir_same_shape_struct", "type shape struct{ rootDir string }", 1},
		// Cross-field accessor isolation: a Reviewers accessor cannot reach
		// WorktreeDir.rootDir and a WorktreeDir accessor cannot reach
		// Reviewers.logins.
		{"worktree_dir_field_in_reviewers_accessor", "func ParseReviewers(d WorktreeDir) string { return d.rootDir }", 1},
		{"reviewers_field_in_worktree_dir_accessor", "func CurrentDir(r Reviewers) int { return len(r.logins) }", 1},
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
