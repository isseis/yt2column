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
//     internal/nmhost or cmd/yt2column-nmhost contains "/articles" (each
//     literal is checked alone, so a target split across a concatenation is
//     not found; the declaration checks name the retired values). A non-test
//     file is a Go file whose name does not end in _test.go and whose first
//     //go:build line does not start with the test tag.
//   - extension: extension/ is the same at the merge-base and at head.
//   - removed: at head, internal/server, internal/publisher, and
//     cmd/yt2column-server declare none of the removed names (top-level
//     declarations and methods; the deps field newNotifier), no Go file
//     refers to one of them through a package qualifier, no Go file imports
//     internal/failure or internal/failure/testutil, and internal/server
//     still declares classifyWrite.
//   - publisher: internal/server's WebhookPublisher is an interface
//     with the one method PostMessages and nothing embedded; neither
//     internal/server nor cmd/yt2column-server depends on internal/pipeline,
//     directly or not (go list -deps in a temporary worktree of head, which
//     is removed however the run ends); and the only call of a selector
//     named Publish in non-test code is in internal/pipeline. Only calls are
//     checked: without type information a method value x.Publish cannot be
//     told from a read of a field named Publish, which articlecache and the
//     status response have.
//   - post: in the non-test code of internal/server and
//     cmd/yt2column-server, the only selector named PostMessages is inside
//     internal/server's runPublish.
//   - words: no comment and no string literal of a non-test file of
//     internal/server, cmd/yt2column-server, internal/serversock, or
//     internal/publisher contains, ignoring case, any of retiredWords.
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

// errNoGoFile reports a directory with no Go file.
var errNoGoFile = errors.New("no Go file")

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
	{"removed", checkRemoved},
	{"publisher", checkPublisher},
	{"post", checkPost},
	{"words", checkWords},
}

// removedNames are the removed names, by the package that declared
// them. A method counts as declared too.
var removedNames = map[string][]string{
	"internal/server": {
		"ArticlesPath", "routeArticles", "articlesHandler", "startJob", "runJob",
		"classify", "stageOf", "failureKind", "reasonToKind", "notify",
		"notifyAsync", "notifyTimeout", "logJobFailure", "logJobSuccess",
		"errNilWebhookNotifier", "detailTooManyRequests",
	},
	"internal/publisher": {
		"SlackFailureNotifier", "NewSlackFailureNotifier", "ErrSlackInvalidNotice",
		"ErrSlackNoticeRejected", "NewSlackFailureNotifierForLoopbackTest",
		"newSlackFailureNotifierForLoopbackTest", "errZeroNotifier",
	},
	"cmd/yt2column-server": {"newNotifier", "llmCallsPerJob"},
}

// modulePath is the module's import path.
const modulePath = "github.com/isseis/yt2column"

// retiredWords are the retired words from the comments and the string
// literals of the server's code, compared ignoring case. "notice" alone is not
// one: the stop notice stays.
var retiredWords = []string{"/articles", "failure notice", "notifier", "legacy"}

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

// checkRemoved checks that the removed names are gone (removedNames,
// the deps field newNotifier, the failure packages) and that classifyWrite,
// which the job API uses, stays.
func checkRemoved(_, head string) ([]string, error) {
	var findings []string
	for dir, names := range removedNames {
		files, err := packageFiles(head, dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			for _, name := range names {
				if pos, ok := f.declares(name); ok {
					findings = append(findings, fmt.Sprintf("%s declares %s", pos, name))
				}
			}
			if pos, ok := f.field("deps", "newNotifier"); ok {
				findings = append(findings, fmt.Sprintf("%s declares the field deps.newNotifier", pos))
			}
		}
	}
	all, err := allGoFiles(head)
	if err != nil {
		return nil, err
	}
	for _, f := range all {
		for _, imp := range f.file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == modulePath+"/internal/failure" || p == modulePath+"/internal/failure/testutil" {
				findings = append(findings, fmt.Sprintf("%s imports %s", f.fset.Position(imp.Pos()), p))
			}
		}
		for dir, names := range removedNames {
			for _, pos := range f.qualifiedRefs(modulePath+"/"+dir, names) {
				findings = append(findings, fmt.Sprintf("%s refers to a removed name of %s", pos, dir))
			}
		}
	}
	files, err := packageFiles(head, "internal/server")
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(files, func(f goFile) bool { _, ok := f.declares("classifyWrite"); return ok }) {
		findings = append(findings, "internal/server does not declare classifyWrite")
	}
	return findings, nil
}

// checkPublisher checks that the server asks the Webhook publisher for
// PostMessages alone, that the server's code does not depend on the
// pipeline, and that only the pipeline selects a Publish.
func checkPublisher(_, head string) ([]string, error) {
	var findings []string
	files, err := packageFiles(head, "internal/server")
	if err != nil {
		return nil, err
	}
	found := false
	for _, f := range files {
		methods, embedded, ok := f.interfaceMethods("WebhookPublisher")
		if !ok {
			continue
		}
		found = true
		if !slices.Equal(methods, []string{"PostMessages"}) || embedded != 0 {
			findings = append(findings, fmt.Sprintf("%s: WebhookPublisher has methods %v and %d embedded, want PostMessages alone", f.path, methods, embedded))
		}
	}
	if !found {
		findings = append(findings, "internal/server declares no interface WebhookPublisher")
	}
	deps, err := listDeps(head, "./internal/server", "./cmd/yt2column-server")
	if err != nil {
		return nil, err
	}
	if slices.Contains(deps, modulePath+"/internal/pipeline") {
		findings = append(findings, "internal/server or cmd/yt2column-server depends on internal/pipeline")
	}
	all, err := allGoFiles(head)
	if err != nil {
		return nil, err
	}
	for _, f := range all {
		if !f.product || path.Dir(f.path) == "internal/pipeline" {
			continue
		}
		for _, pos := range f.calls("Publish") {
			findings = append(findings, fmt.Sprintf("%s calls Publish outside internal/pipeline", pos))
		}
	}
	return findings, nil
}

// checkPost checks that only internal/server's runPublish selects
// PostMessages in the server's non-test code.
func checkPost(_, head string) ([]string, error) {
	var findings []string
	calls := 0
	for _, dir := range []string{"internal/server", "cmd/yt2column-server"} {
		files, err := packageFiles(head, dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if !f.product {
				continue
			}
			for _, sel := range f.selectors("PostMessages") {
				if dir == "internal/server" && sel.fn == "runPublish" {
					calls++
					continue
				}
				findings = append(findings, fmt.Sprintf("%s selects PostMessages in %s", sel.pos, sel.fn))
			}
		}
	}
	if calls == 0 {
		findings = append(findings, "runPublish does not select PostMessages")
	}
	return findings, nil
}

// checkWords checks that the server's non-test comments and string literals
// hold none of retiredWords.
func checkWords(_, head string) ([]string, error) {
	var findings []string
	for _, dir := range []string{"internal/server", "cmd/yt2column-server", "internal/serversock", "internal/publisher"} {
		files, err := packageFiles(head, dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if !f.product {
				continue
			}
			for _, word := range retiredWords {
				for _, pos := range f.textContaining(word) {
					findings = append(findings, fmt.Sprintf("%s holds %q", pos, word))
				}
			}
		}
	}
	return findings, nil
}

// listDeps runs go list -deps on pkgs in a temporary worktree of commit and
// returns the import paths it prints. The worktree is removed before it
// returns, whatever happened.
func listDeps(commit string, pkgs ...string) (deps []string, err error) {
	dir, err := os.MkdirTemp("", "residue-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	tree := path.Join(dir, "tree")
	if out, err := exec.Command("git", "worktree", "add", "--detach", tree, commit).CombinedOutput(); err != nil { //nolint:gosec // a commit this tool was given
		return nil, fmt.Errorf("git worktree add: %w: %s", err, out)
	}
	defer func() {
		if out, rmErr := exec.Command("git", "worktree", "remove", "--force", tree).CombinedOutput(); rmErr != nil && err == nil { //nolint:gosec // as above
			err = fmt.Errorf("git worktree remove: %w: %s", rmErr, out)
		}
	}()
	cmd := exec.Command("go", append([]string{"list", "-deps"}, pkgs...)...) //nolint:gosec // fixed packages
	cmd.Dir = tree
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -deps: %w", err)
	}
	return strings.Fields(string(out)), nil
}

// allGoFiles parses every Go file under cmd/ and internal/ at commit.
func allGoFiles(commit string) ([]goFile, error) {
	out, err := exec.Command("git", "ls-tree", "-r", "-d", "--name-only", commit, "cmd/", "internal/").Output() //nolint:gosec // a commit this tool was given
	if err != nil {
		return nil, fmt.Errorf("git ls-tree -d %s: %w", commit, err)
	}
	var all []goFile
	for dir := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains("/"+dir+"/", "/testdata/") {
			continue
		}
		files, err := packageFiles(commit, dir)
		if errors.Is(err, errNoGoFile) {
			continue
		}
		if err != nil {
			return nil, err
		}
		all = append(all, files...)
	}
	return all, nil
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
		return nil, fmt.Errorf("%w: %s at %s", errNoGoFile, dir, commit)
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

// declares returns the position of the declaration of name: a top-level
// declaration, or a method of any receiver.
func (f goFile) declares(name string) (token.Position, bool) {
	if pos, ok := f.topLevel(name); ok {
		return pos, true
	}
	for _, decl := range f.file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Recv != nil && d.Name.Name == name {
			return f.fset.Position(d.Name.Pos()), true
		}
	}
	return token.Position{}, false
}

// qualifiedRefs returns the positions of the selectors pkg.Name, for each
// name in names, where pkg is the file's name for the import path importPath.
func (f goFile) qualifiedRefs(importPath string, names []string) []token.Position {
	local := ""
	for _, imp := range f.file.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == importPath {
			local = path.Base(p)
			if imp.Name != nil {
				local = imp.Name.Name
			}
		}
	}
	if local == "" {
		return nil
	}
	var found []token.Position
	ast.Inspect(f.file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == local && slices.Contains(names, sel.Sel.Name) {
			found = append(found, f.fset.Position(sel.Pos()))
		}
		return true
	})
	return found
}

// interfaceMethods returns the method names, in order, and the number of
// embedded types of the top-level interface type typeName.
func (f goFile) interfaceMethods(typeName string) (methods []string, embedded int, ok bool) {
	for _, decl := range f.file.Decls {
		d, isGen := decl.(*ast.GenDecl)
		if !isGen {
			continue
		}
		for _, spec := range d.Specs {
			s, isType := spec.(*ast.TypeSpec)
			if !isType || s.Name.Name != typeName {
				continue
			}
			it, isIface := s.Type.(*ast.InterfaceType)
			if !isIface {
				return nil, 0, false
			}
			for _, m := range it.Methods.List {
				if len(m.Names) == 0 {
					embedded++
				}
				for _, n := range m.Names {
					methods = append(methods, n.Name)
				}
			}
			return methods, embedded, true
		}
	}
	return nil, 0, false
}

// selector is one selector expression found by selectors: its position and
// the function that holds it ("" at the top level).
type selector struct {
	pos token.Position
	fn  string
}

// selectors returns every selector expression x.name in the file, calls and
// method values alike.
func (f goFile) selectors(name string) []selector {
	var found []selector
	for _, decl := range f.file.Decls {
		fn := ""
		if d, ok := decl.(*ast.FuncDecl); ok {
			fn = d.Name.Name
		}
		ast.Inspect(decl, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
				found = append(found, selector{pos: f.fset.Position(sel.Pos()), fn: fn})
			}
			return true
		})
	}
	return found
}

// textContaining returns the positions of the comments and string literals
// that contain word, ignoring case.
func (f goFile) textContaining(word string) []token.Position {
	word = strings.ToLower(word)
	var found []token.Position
	for _, group := range f.file.Comments {
		for _, c := range group.List {
			if strings.Contains(strings.ToLower(c.Text), word) {
				found = append(found, f.fset.Position(c.Pos()))
			}
		}
	}
	ast.Inspect(f.file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			value = lit.Value
		}
		if strings.Contains(strings.ToLower(value), word) {
			found = append(found, f.fset.Position(lit.Pos()))
		}
		return true
	})
	return found
}

// calls returns the positions of the calls x.name(...) in the file.
func (f goFile) calls(name string) []token.Position {
	var found []token.Position
	ast.Inspect(f.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
			found = append(found, f.fset.Position(sel.Pos()))
		}
		return true
	})
	return found
}
