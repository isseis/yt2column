//go:build test

// Command residue checks, once per pull request of task 0059, that what the
// task removes is gone at the pull request's head and that the browser
// extension did not change:
//
//	go run -tags test ./docs/tasks/0059_remove_legacy_articles_api/_checks/residue/ <merge-base> <head>
//
// Pass the merge-base with origin/main (git merge-base origin/main HEAD) and
// the head of the pull request. Every file is read from those commits with
// git (git ls-tree, git show, git diff), never from the working tree, and the
// Go code is checked by its syntax tree (packages, declarations, and string
// literals), not by a text search, so a local variable or another package's
// identifier with the same name does not count.
//
// The checks:
//
//   - host: at head, internal/nmhost declares none of articlesURL, form,
//     formLegacy, and formNew at the top level, its type destination has no
//     field named form, and no string literal in a non-test file of
//     internal/nmhost or cmd/yt2column-nmhost contains "/articles". A non-test
//     file is a Go file whose name does not end in _test.go and whose first
//     //go:build line does not start with the test tag.
//   - extension: extension/ is the same at the merge-base and at head.
//
// Each check prints OK or NG with its findings. Exit status: 0 when every
// check holds, 1 when one does not, 2 when a check cannot be run (wrong
// arguments, or a git command or a parse that fails).
//
// Run it from the repository root. make test and CI do not run it, and it is
// not maintained after the merge. The //go:build test first line makes the
// repository-wide environment access check treat the file as test code.
package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// testConstraint matches a first //go:build line that starts with the test
// tag. The tag must end there, so a tag such as "testing" is not matched.
var testConstraint = regexp.MustCompile(`^//go:build test([^A-Za-z0-9_.]|$)`)

// check is one named check. It returns the findings that break it, none when
// it holds, or an error when it cannot be run.
type check struct {
	name string
	run  func(base, head string) ([]string, error)
}

var checks = []check{
	{"host", checkHost},
	{"extension", checkExtension},
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: residue <merge-base> <head>")
		os.Exit(2)
	}
	base, head := os.Args[1], os.Args[2]
	failed := false
	for _, c := range checks {
		findings, err := c.run(base, head)
		if err != nil {
			fmt.Fprintf(os.Stderr, "residue: %s: %v\n", c.name, err)
			os.Exit(2)
		}
		if len(findings) == 0 {
			fmt.Printf("OK   %s\n", c.name)
			continue
		}
		failed = true
		fmt.Printf("NG   %s: %d finding(s)\n", c.name, len(findings))
		for _, f := range findings {
			fmt.Printf("     %s\n", f)
		}
	}
	if failed {
		os.Exit(1)
	}
}

// checkHost checks that the host holds no target or value of the retired
// message form.
func checkHost(_, head string) ([]string, error) {
	const hostPkg = "internal/nmhost"
	files, err := packageFiles(head, hostPkg)
	if err != nil {
		return nil, err
	}
	var findings []string
	for _, f := range files {
		for _, name := range []string{"articlesURL", "form", "formLegacy", "formNew"} {
			if pos, ok := f.topLevel(name); ok {
				findings = append(findings, fmt.Sprintf("%s declares %s", pos, name))
			}
		}
		if pos, ok := f.field("destination", "form"); ok {
			findings = append(findings, fmt.Sprintf("%s declares the field destination.form", pos))
		}
	}
	cmdFiles, err := packageFiles(head, "cmd/yt2column-nmhost")
	if err != nil {
		return nil, err
	}
	for _, f := range slices.Concat(files, cmdFiles) {
		if !f.product {
			continue
		}
		for _, pos := range f.stringsContaining("/articles") {
			findings = append(findings, fmt.Sprintf("%s has a string literal containing /articles", pos))
		}
	}
	return findings, nil
}

// checkExtension checks that extension/ did not change between the commits.
func checkExtension(base, head string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--quiet", base, head, "--", "extension/") //nolint:gosec // commits this tool was given
	err := cmd.Run()
	if err == nil {
		return nil, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
		out, err := exec.Command("git", "diff", "--name-only", base, head, "--", "extension/").Output() //nolint:gosec // as above
		if err != nil {
			return nil, fmt.Errorf("git diff --name-only: %w", err)
		}
		var findings []string
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			findings = append(findings, line+" changed")
		}
		return findings, nil
	}
	return nil, fmt.Errorf("git diff --quiet: %w", err)
}

// goFile is one parsed Go file at a commit.
type goFile struct {
	path    string
	product bool // neither a _test.go file nor a file with a test build constraint
	fset    *token.FileSet
	file    *ast.File
}

// packageFiles parses every Go file directly in dir at commit.
func packageFiles(commit, dir string) ([]goFile, error) {
	out, err := exec.Command("git", "ls-tree", "--name-only", commit, dir+"/").Output() //nolint:gosec // a commit this tool was given
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s %s: %w", commit, dir, err)
	}
	var files []goFile
	for name := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if !strings.HasSuffix(name, ".go") || path.Dir(name) != dir {
			continue
		}
		data, err := exec.Command("git", "show", commit+":"+name).Output() //nolint:gosec // as above
		if err != nil {
			return nil, fmt.Errorf("git show %s:%s: %w", commit, name, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, data, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s at %s: %w", name, commit, err)
		}
		files = append(files, goFile{
			path:    name,
			product: !strings.HasSuffix(name, "_test.go") && !hasTestConstraint(file),
			fset:    fset,
			file:    file,
		})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no Go file in %s at %s", dir, commit)
	}
	return files, nil
}

// hasTestConstraint reports whether the file's first //go:build line starts
// with the test tag.
func hasTestConstraint(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:build") {
				return testConstraint.MatchString(c.Text)
			}
		}
	}
	return false
}

// topLevel returns the position of the top-level declaration of name (a
// constant, variable, type, or function without a receiver).
func (f goFile) topLevel(name string) (token.Position, bool) {
	for _, decl := range f.file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name == name {
				return f.fset.Position(d.Name.Pos()), true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == name {
						return f.fset.Position(s.Name.Pos()), true
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name == name {
							return f.fset.Position(n.Pos()), true
						}
					}
				}
			}
		}
	}
	return token.Position{}, false
}

// field returns the position of the field named field in the struct type
// named typeName declared at the top level.
func (f goFile) field(typeName, field string) (token.Position, bool) {
	for _, decl := range f.file.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range d.Specs {
			s, ok := spec.(*ast.TypeSpec)
			if !ok || s.Name.Name != typeName {
				continue
			}
			st, ok := s.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, fl := range st.Fields.List {
				for _, n := range fl.Names {
					if n.Name == field {
						return f.fset.Position(n.Pos()), true
					}
				}
			}
		}
	}
	return token.Position{}, false
}

// stringsContaining returns the positions of the string literals whose value
// contains text.
func (f goFile) stringsContaining(text string) []token.Position {
	var found []token.Position
	ast.Inspect(f.file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			value = lit.Value
		}
		if strings.Contains(value, text) {
			found = append(found, f.fset.Position(lit.Pos()))
		}
		return true
	})
	return found
}
