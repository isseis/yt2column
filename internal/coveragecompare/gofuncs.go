package coveragecompare

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

// goPosition is a 1-based line and byte column of a Go file, the unit both
// go/token and the coverage profile use.
type goPosition struct {
	Line int
	Col  int
}

// before reports whether p comes strictly before q.
func (p goPosition) before(q goPosition) bool {
	return p.Line < q.Line || (p.Line == q.Line && p.Col < q.Col)
}

// goFunc is one function declaration of a Go file. Start is the func keyword,
// so a doc comment before it is outside the range; End is just past the
// closing brace (or past the signature of a declaration without a body).
type goFunc struct {
	Name  string
	Start goPosition
	End   goPosition
}

// parseGoFuncs returns the function declarations of a Go file in source order.
// A method's name carries its receiver type, as T.Name or (*T).Name, without
// type parameters. A file go/parser rejects stops as unreadable.
func parseGoFuncs(file string, src []byte, point Point) ([]goFunc, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		line := 0
		if list, ok := errors.AsType[scanner.ErrorList](err); ok && len(list) > 0 {
			line = list[0].Pos.Line
		}
		reason := StopReason{
			Point:   point,
			Input:   InputSource,
			Line:    line,
			Func:    &FuncID{File: file},
			Detail:  "the Go source does not parse",
			Excerpt: err.Error(),
		}
		return nil, &StopError{Kind: StopUnreadable, Reasons: []StopReason{reason}}
	}
	var funcs []goFunc
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start := fset.Position(fn.Pos())
		end := fset.Position(fn.End())
		funcs = append(funcs, goFunc{
			Name:  goFuncName(fn),
			Start: goPosition{Line: start.Line, Col: start.Column},
			End:   goPosition{Line: end.Line, Col: end.Column},
		})
	}
	return funcs, nil
}

// goFuncName is the name the output shows for a declaration.
func goFuncName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv, pointer := goReceiverType(fn.Recv.List[0].Type)
	if pointer {
		return "(*" + recv + ")." + fn.Name.Name
	}
	return recv + "." + fn.Name.Name
}

// goReceiverType returns a receiver's type name without type parameters, and
// whether the receiver is a pointer.
func goReceiverType(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.StarExpr:
		name, _ := goReceiverType(e.X)
		return name, true
	case *ast.ParenExpr:
		return goReceiverType(e.X)
	case *ast.IndexExpr:
		return goReceiverType(e.X)
	case *ast.IndexListExpr:
		return goReceiverType(e.X)
	case *ast.Ident:
		return e.Name, false
	default:
		return "?", false
	}
}

// goModulePath reads the module path from a go.mod file: the value of its
// first module directive, without a trailing comment and unquoted when it is
// quoted. A go.mod with no module directive, a block form, or an empty or
// malformed value stops as unreadable.
func goModulePath(data []byte, point Point) (string, error) {
	for i, raw := range bytes.Split(data, []byte("\n")) {
		line := string(raw)
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = line[:comment]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "module" {
			continue
		}
		if len(fields) != 2 || fields[1] == "(" {
			return "", goModError(point, i+1, string(raw))
		}
		path := fields[1]
		if strings.HasPrefix(path, `"`) || strings.HasPrefix(path, "`") {
			unquoted, err := strconv.Unquote(path)
			if err != nil {
				return "", goModError(point, i+1, string(raw))
			}
			path = unquoted
		}
		if path == "" || strings.ContainsAny(path, " \t\"`") {
			return "", goModError(point, i+1, string(raw))
		}
		return path, nil
	}
	return "", goModError(point, 0, "")
}

// goModError is the stop for a go.mod whose module path cannot be read.
func goModError(point Point, line int, excerpt string) *StopError {
	reason := StopReason{
		Point:   point,
		Input:   InputSource,
		Line:    line,
		Func:    &FuncID{File: "go.mod"},
		Detail:  "go.mod has no single readable module path",
		Excerpt: excerpt,
	}
	return &StopError{Kind: StopUnreadable, Reasons: []StopReason{reason}}
}
