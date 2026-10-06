//go:build test

package config

import (
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// envFunctions lists the standard library functions that read the process
// environment, by package path. It is not limited to the functions that return
// a named variable's value: the internal readers (os.UserCacheDir,
// os.UserConfigDir, os.UserHomeDir) are listed too, because a caller could
// route the configuration through them. Functions that read a fixed,
// non-secret variable only for their own operation (exec.LookPath's PATH,
// os.TempDir's TMPDIR, os.Getwd's PWD) are deliberately absent.
var envFunctions = map[string]map[string]bool{
	"os": {
		"Getenv":        true,
		"LookupEnv":     true,
		"Environ":       true,
		"ExpandEnv":     true,
		"UserCacheDir":  true,
		"UserConfigDir": true,
		"UserHomeDir":   true,
	},
	"syscall": {
		"Getenv":  true,
		"Environ": true,
	},
}

// envMethodNames is every name in envFunctions. A selector with one of these
// names on a receiver that is not a known os/syscall import is rejected rather
// than allowed, because the reference cannot be resolved (for example
// (*exec.Cmd).Environ, which returns os.Environ when the command has no Env).
var envMethodNames = func() map[string]bool {
	names := map[string]bool{}
	for _, funcs := range envFunctions {
		for name := range funcs {
			names[name] = true
		}
	}
	return names
}()

// secretEnvNames are the secret variable names that may appear only in
// internal/config.
var secretEnvNames = []string{"DEEPSEEK_API_KEY", "SLACK_WEBHOOK_URL"}

// allowedEnvRefs lists the only production references allowed outside
// internal/config, by repo-relative slash path and by reference key with the
// maximum count. The key is "<kind>:<qualified name>", where kind is "call",
// "value", "import", or "secret".
var allowedEnvRefs = map[string]map[string]int{
	"cmd/yt2column/main.go":        {"value:os.LookupEnv": 1},
	"internal/transcript/ytdlp.go": {"call:os.Environ": 1},
}

// envAccessReport is the result of one scan.
type envAccessReport struct {
	scanned  int
	findings []string
	observed map[string]int
}

// envRef is one reference: a kind and a qualified name.
type envRef struct {
	kind      string
	qualified string
}

// scanEnvAccess walks every production Go file under root, reporting each
// reference to an environment-reading function and each secret variable name
// outside the allowed places.
func scanEnvAccess(root string) (envAccessReport, error) {
	rep := envAccessReport{observed: map[string]int{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" {
				return filepath.SkipDir
			}
			// Skip only the repository's top-level build output directory, not
			// a package that happens to be named build at some other depth.
			if path == filepath.Join(root, "build") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		refs, production, err := envRefsInFile(path, d.Name())
		if err != nil {
			return err
		}
		if !production {
			return nil
		}
		rep.scanned++
		classifyEnvRefs(filepath.ToSlash(rel), refs, &rep)
		return nil
	})
	slices.Sort(rep.findings)
	return rep, err
}

// envRefsInFile parses path and reports whether it is production code and the
// environment references it holds.
func envRefsInFile(path, name string) ([]envRef, bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, false, err
	}
	if isTestCode(name, buildConstraint(file)) {
		return nil, false, nil
	}
	return collectEnvRefs(file), true, nil
}

// collectEnvRefs resolves the imports of file and returns every reference to an
// environment-reading function and every secret variable name literal. A secret
// name assembled from more than one literal is not detected; the check targets
// the contiguous literal a developer would write.
func collectEnvRefs(file *ast.File) []envRef {
	aliases := map[string]string{}
	var refs []envRef
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if _, ok := envFunctions[path]; !ok {
			continue
		}
		switch {
		case imp.Name == nil:
			aliases[path] = path
		case imp.Name.Name == "_":
			// A blank import creates no usable reference.
		case imp.Name.Name == ".":
			// A dot import hides the package name, so a reference through it
			// cannot be resolved; reject the import rather than let it slip
			// through.
			refs = append(refs, envRef{kind: "import", qualified: path})
		default:
			aliases[imp.Name.Name] = path
		}
	}

	called := map[*ast.SelectorExpr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if qualified, ok := envSelector(sel, aliases); ok {
					called[sel] = true
					refs = append(refs, envRef{kind: "call", qualified: qualified})
				}
			}
		case *ast.SelectorExpr:
			if called[v] {
				return true
			}
			if qualified, ok := envSelector(v, aliases); ok {
				refs = append(refs, envRef{kind: "value", qualified: qualified})
			} else if method, ok := methodEnvRef(v, aliases); ok {
				refs = append(refs, envRef{kind: "method", qualified: method})
			}
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return true
			}
			for _, secret := range secretEnvNames {
				if strings.Contains(s, secret) {
					refs = append(refs, envRef{kind: "secret", qualified: secret})
				}
			}
		}
		return true
	})
	return refs
}

// methodEnvRef reports sel's method name when it selects an environment-reader
// name on a receiver that is not a known os/syscall import alias. The reference
// cannot be resolved to a standard library function, so the scan rejects it
// rather than assuming it is safe.
func methodEnvRef(sel *ast.SelectorExpr, aliases map[string]string) (string, bool) {
	if ident, ok := sel.X.(*ast.Ident); ok {
		if _, isImport := aliases[ident.Name]; isImport {
			return "", false
		}
	}
	if !envMethodNames[sel.Sel.Name] {
		return "", false
	}
	return sel.Sel.Name, true
}

// envSelector resolves sel against the import aliases and reports the
// qualified name when it references an environment-reading function.
func envSelector(sel *ast.SelectorExpr, aliases map[string]string) (string, bool) {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	path, ok := aliases[ident.Name]
	if !ok {
		return "", false
	}
	if !envFunctions[path][sel.Sel.Name] {
		return "", false
	}
	return path + "." + sel.Sel.Name, true
}

// classifyEnvRefs records refs from rel: allowed references are observed, and
// every other production reference is a finding.
func classifyEnvRefs(rel string, refs []envRef, rep *envAccessReport) {
	counts := map[string]int{}
	for _, ref := range refs {
		counts[ref.kind+":"+ref.qualified]++
	}
	for key, n := range counts {
		if envRefAllowed(rel, key, n) {
			rep.observed[rel+" "+key] += n
			continue
		}
		rep.findings = append(rep.findings, rel+" "+key)
	}
}

// envRefAllowed reports whether key may appear n times at rel.
func envRefAllowed(rel, key string, n int) bool {
	if isConfigPath(rel) {
		return true
	}
	limit, ok := allowedEnvRefs[rel][key]
	return ok && n <= limit
}

// isConfigPath reports whether rel is under internal/config, where the process
// environment may be read.
func isConfigPath(rel string) bool {
	const prefix = "internal/config"
	return rel == prefix || strings.HasPrefix(rel, prefix+"/")
}

// isTestCode reports whether a file is test code: its name ends in _test.go, or
// its build constraint cannot hold without the test or integration tag. A file
// constrained only by a name suffix (foo_linux.go) is production code.
func isTestCode(name string, expr constraint.Expr) bool {
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	if expr == nil {
		return false
	}
	if satisfiable(expr, map[string]bool{"test": false, "integration": false}) {
		return false
	}
	for _, fixed := range []map[string]bool{
		{"test": true, "integration": false},
		{"test": false, "integration": true},
		{"test": true, "integration": true},
	} {
		if satisfiable(expr, fixed) {
			return true
		}
	}
	return false
}

// buildConstraint returns the //go:build expression of file, or nil when the
// file has none.
func buildConstraint(file *ast.File) constraint.Expr {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, "//go:build ") {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err == nil {
				return expr
			}
		}
	}
	return nil
}

// satisfiable reports whether expr can be true when the tags in fixed have the
// given values and every other tag is free.
func satisfiable(expr constraint.Expr, fixed map[string]bool) bool {
	free := freeTags(expr, fixed)
	assign := make(map[string]bool, len(fixed)+len(free))
	maps.Copy(assign, fixed)
	return assignTags(expr, free, 0, assign)
}

// assignTags searches the assignments of the free tags for one that satisfies
// expr.
func assignTags(expr constraint.Expr, free []string, i int, assign map[string]bool) bool {
	if i == len(free) {
		return expr.Eval(func(tag string) bool { return assign[tag] })
	}
	for _, value := range []bool{false, true} {
		assign[free[i]] = value
		if assignTags(expr, free, i+1, assign) {
			return true
		}
	}
	return false
}

// freeTags returns the tags of expr, in first-seen order, that are not fixed.
func freeTags(expr constraint.Expr, fixed map[string]bool) []string {
	var tags []string
	seen := map[string]bool{}
	var walk func(constraint.Expr)
	walk = func(e constraint.Expr) {
		switch v := e.(type) {
		case *constraint.TagExpr:
			if _, isFixed := fixed[v.Tag]; !isFixed && !seen[v.Tag] {
				seen[v.Tag] = true
				tags = append(tags, v.Tag)
			}
		case *constraint.NotExpr:
			walk(v.X)
		case *constraint.AndExpr:
			walk(v.X)
			walk(v.Y)
		case *constraint.OrExpr:
			walk(v.X)
			walk(v.Y)
		}
	}
	walk(expr)
	return tags
}

func TestEnvAccessConfined(t *testing.T) {
	rep, err := scanEnvAccess("../..")
	if err != nil {
		t.Fatalf("scanEnvAccess() error = %v", err)
	}
	if rep.scanned == 0 {
		t.Fatal("scanEnvAccess() scanned no files")
	}
	for _, finding := range rep.findings {
		t.Errorf("environment reference outside the allowed places: %s", finding)
	}
	if got := rep.observed["internal/transcript/ytdlp.go call:os.Environ"]; got != 1 {
		t.Errorf("observed internal/transcript/ytdlp.go os.Environ = %d, want 1", got)
	}
	if got := rep.observed["cmd/yt2column/main.go value:os.LookupEnv"]; got != 1 {
		t.Errorf("observed cmd/yt2column/main.go os.LookupEnv = %d, want 1", got)
	}
}

func TestEnvAccessScannerDetects(t *testing.T) {
	files := map[string]string{
		"internal/config/config.go": `package config

import "os"

var _ = os.Getenv("X")
const _ = "DEEPSEEK_API_KEY"
`,
		"internal/transcript/ytdlp.go": `package transcript

import (
	"os"
	"syscall"
)

func f() {
	_ = os.Environ()
	_ = os.Getenv("X")
	_ = syscall.Getenv("Y")
}
`,
		"cmd/yt2column/main.go": `package main

import "os"

func main() {
	run(os.LookupEnv)
}

func run(func(string) (string, bool)) {}

func other() {
	_ = os.Getenv("X")
	_ = os.LookupEnv("Y")
}
`,
		"cmd/yt2column/run.go": `package main

import "os"

func runOther() {
	_ = os.LookupEnv("Z")
}
`,
		"internal/other/alias.go": `package other

import o "os"

func f() {
	_ = o.Getenv("X")
}
`,
		"internal/other/dot.go": `package other

import . "os"

func f() {
	_ = Getenv("X")
}
`,
		"internal/other/envvalue.go": `package other

import "os"

var lookup = os.Getenv
`,
		"internal/other/internalreader.go": `package other

import "os"

func f() {
	_, _ = os.UserCacheDir()
}
`,
		"internal/other/methodreader.go": `package other

import "os/exec"

func f() {
	c := exec.Command("x")
	_ = c.Environ()
}
`,
		"internal/other/others.go": `package other

import (
	"os"
	"syscall"
)

func f() {
	_ = os.ExpandEnv("$X")
	_, _ = os.UserConfigDir()
	_, _ = os.UserHomeDir()
	_ = syscall.Environ()
}
`,
		"internal/other/buildunix.go": `//go:build unix

package other

import "os"

func f() { _ = os.Getenv("X") }
`,
		"internal/other/buildlinux.go": `//go:build linux

package other

import "os"

func f() { _ = os.Getenv("X") }
`,
		"internal/other/buildwindows.go": `//go:build windows

package other

import "os"

func f() { _ = os.Getenv("X") }
`,
		"internal/other/platform_linux.go": `package other

import "os"

func f() { _ = os.Getenv("X") }
`,
		"internal/other/tagonly.go": `//go:build test

package other

import "os"

func f() { _ = os.Getenv("X") }
`,
		"internal/other/tagor.go": `//go:build test || integration

package other

import "os"

func f() { _ = os.Getenv("X") }
`,
		"internal/other/named_test.go": `package other

import "os"

func f() { _ = os.Getenv("X") }

const _ = "SLACK_WEBHOOK_URL"
`,
		"internal/other/secretname.go": `package other

const key = "SLACK_WEBHOOK_URL"
`,
	}

	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", rel, err)
		}
	}

	rep, err := scanEnvAccess(root)
	if err != nil {
		t.Fatalf("scanEnvAccess() error = %v", err)
	}
	want := []string{
		"cmd/yt2column/main.go call:os.Getenv",
		"cmd/yt2column/main.go call:os.LookupEnv",
		"cmd/yt2column/run.go call:os.LookupEnv",
		"internal/other/alias.go call:os.Getenv",
		"internal/other/buildlinux.go call:os.Getenv",
		"internal/other/buildunix.go call:os.Getenv",
		"internal/other/buildwindows.go call:os.Getenv",
		"internal/other/dot.go import:os",
		"internal/other/envvalue.go value:os.Getenv",
		"internal/other/internalreader.go call:os.UserCacheDir",
		"internal/other/methodreader.go method:Environ",
		"internal/other/others.go call:os.ExpandEnv",
		"internal/other/others.go call:os.UserConfigDir",
		"internal/other/others.go call:os.UserHomeDir",
		"internal/other/others.go call:syscall.Environ",
		"internal/other/platform_linux.go call:os.Getenv",
		"internal/other/secretname.go secret:SLACK_WEBHOOK_URL",
		"internal/transcript/ytdlp.go call:os.Getenv",
		"internal/transcript/ytdlp.go call:syscall.Getenv",
	}
	if !slices.Equal(rep.findings, want) {
		t.Errorf("scanEnvAccess() findings = %v, want %v", rep.findings, want)
	}
	if rep.scanned != len(files)-3 {
		t.Errorf("scanEnvAccess() scanned %d files, want %d", rep.scanned, len(files)-3)
	}
	for _, key := range []string{
		"internal/config/config.go call:os.Getenv",
		"internal/config/config.go secret:DEEPSEEK_API_KEY",
		"internal/transcript/ytdlp.go call:os.Environ",
		"cmd/yt2column/main.go value:os.LookupEnv",
	} {
		if rep.observed[key] == 0 {
			t.Errorf("observed[%q] = 0, want > 0", key)
		}
	}
}
