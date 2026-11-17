//go:build test

package chief

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// guardedTypes lists the types whose values only a constructor may make (the
// types that write to Herdr or git, or decide where), each with the functions
// allowed to build one. A type is added in the change that adds it.
var guardedTypes = map[string][]string{
	"TaskSpace": {"NewTaskSpace"},
}

// emptyImporter gives every import an empty package, so type checking of
// this package alone goes on past the imports it cannot resolve: the guard
// needs only the types the package declares itself.
type emptyImporter struct{}

func (emptyImporter) Import(path string) (*types.Package, error) {
	name := path[strings.LastIndex(path, "/")+1:]
	pkg := types.NewPackage(path, name)
	pkg.MarkComplete()
	return pkg, nil
}

// constructionViolations returns, for the files of one package, every
// composite literal of a guarded type (the zero literal T{} included) and
// every assignment to a field of one that is outside the functions allowed
// for that type.
func constructionViolations(t *testing.T, fset *token.FileSet, files []*ast.File) []string {
	t.Helper()
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: emptyImporter{}, Error: func(error) {}}
	_, _ = conf.Check("chief", fset, files, info)
	guarded := func(typ types.Type) (string, bool) {
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = ptr.Elem()
		}
		named, ok := typ.(*types.Named)
		if !ok {
			return "", false
		}
		_, ok = guardedTypes[named.Obj().Name()]
		return named.Obj().Name(), ok
	}
	var out []string
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				var typeName string
				switch n := n.(type) {
				case *ast.CompositeLit:
					if id, ok := n.Type.(*ast.Ident); ok {
						if _, isGuarded := guardedTypes[id.Name]; isGuarded {
							typeName = id.Name
						}
					}
				case *ast.AssignStmt:
					for _, lhs := range n.Lhs {
						if sel, ok := lhs.(*ast.SelectorExpr); ok {
							if s := info.Selections[sel]; s != nil && s.Kind() == types.FieldVal {
								if name, ok := guarded(s.Recv()); ok {
									typeName = name
								}
							}
						}
					}
				}
				if typeName != "" && !slices.Contains(guardedTypes[typeName], fn.Name.Name) {
					out = append(out, typeName+" built in "+fn.Name.Name+" at "+fset.Position(n.Pos()).String())
				}
				return true
			})
		}
	}
	return out
}

// TestGuardedTypesAreBuiltOnlyByTheirConstructors reads the package's source
// that is not a test and finds no literal of, or assignment to a field of, a
// guarded type outside its constructor.
func TestGuardedTypesAreBuiltOnlyByTheirConstructors(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatal("no source files read")
	}
	for _, v := range constructionViolations(t, fset, files) {
		t.Errorf("%s", v)
	}
}

// TestConstructionGuardFindsViolations runs the guard on source that breaks
// it, so the guard cannot pass because it reads nothing.
func TestConstructionGuardFindsViolations(t *testing.T) {
	const src = `package chief

type TaskSpace struct{ path string }

func NewTaskSpace() TaskSpace { return TaskSpace{path: "ok"} }

func viaLiteral() TaskSpace { return TaskSpace{path: "x"} }

func viaZero() TaskSpace { return TaskSpace{} }

func viaField(s TaskSpace) TaskSpace { s.path = "x"; return s }

func viaPointer(s *TaskSpace) { s.path = "x" }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "violations.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := constructionViolations(t, fset, []*ast.File{file})
	for _, fn := range []string{"viaLiteral", "viaZero", "viaField", "viaPointer"} {
		if !slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, "built in "+fn+" at") }) {
			t.Errorf("no violation found in %s: %v", fn, got)
		}
	}
	if slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, "built in NewTaskSpace at") }) {
		t.Errorf("the constructor was reported: %v", got)
	}
}
