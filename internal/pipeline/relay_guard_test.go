//go:build test

package pipeline

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

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

// productionGoFiles returns the repository-relative paths of the production Go
// files under cmd/ and internal/: every .go file except test files and files
// built only with the test or integration tag.
func productionGoFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	fset := token.NewFileSet()
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.PackageClauseOnly)
			if err != nil {
				return err
			}
			if isTestCode(d.Name(), buildConstraint(file)) {
				return nil
			}
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if len(files) == 0 {
		t.Fatal("found no production Go files")
	}
	return files
}

// isTestCode reports whether a file is test code: its name ends in _test.go, or
// its build constraint cannot hold without the test or integration tag. A file
// constrained only by a name suffix (foo_linux.go) is production code. It
// mirrors the helper of the same name in internal/config/envaccess_test.go.
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

// loopbackLiterals are the substrings of a string literal that name a loopback
// address; a TCP listener bound to one is what the server no longer has.
var loopbackLiterals = []string{"127.0.0.1", "[::1]", "::1"}

// tcpNetworks are the network names that make a listener a TCP listener.
var tcpNetworks = map[string]bool{"tcp": true, "tcp4": true, "tcp6": true}

// unixNetworks are the network names that make a listener a Unix socket one.
var unixNetworks = map[string]bool{"unix": true, "unixpacket": true}

// tcpListenerFindings parses src and describes every TCP listener it creates:
// a call to net.ListenTCP, http.ListenAndServe or http.ListenAndServeTLS; a
// call to a Listen function or method (net.Listen, (*net.ListenConfig).Listen)
// with a TCP network name among its first two arguments; and a string literal
// containing a loopback address.
func tcpListenerFindings(t *testing.T, name, src string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var findings []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if what := tcpListenCall(v); what != "" {
				findings = append(findings, what)
			}
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(v.Value)
			if err != nil {
				return true
			}
			for _, lit := range loopbackLiterals {
				if strings.Contains(text, lit) {
					findings = append(findings, "loopback literal "+v.Value)
					break
				}
			}
		}
		return true
	})
	return findings
}

// tcpListenCall describes call when it creates a TCP listener, or returns "".
func tcpListenCall(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	switch sel.Sel.Name {
	case "ListenTCP", "ListenAndServe", "ListenAndServeTLS":
		// Matched on any receiver: http.ListenAndServe, srv.ListenAndServe(),
		// and an aliased import all start a TCP server.
		return sel.Sel.Name
	case "Listen":
		// The network is the first argument of net.Listen and the second of
		// (*net.ListenConfig).Listen. A network that is not a string literal
		// cannot be verified, so it is reported too.
		args := call.Args[:min(len(call.Args), 2)]
		if len(args) == 0 {
			return ""
		}
		unverifiable := true
		for _, arg := range args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			network, err := strconv.Unquote(lit.Value)
			switch {
			case err != nil:
			case tcpNetworks[network]:
				return "Listen on " + lit.Value
			case unixNetworks[network]:
				unverifiable = false
			}
		}
		if unverifiable {
			return "Listen with a network that is not a string literal"
		}
	}
	return ""
}

func TestTCPListenerDetection(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"net.Listen tcp", `package p; import "net"; func f() { net.Listen("tcp", ":0") }`, 1},
		{"net.Listen tcp4", `package p; import "net"; func f() { net.Listen("tcp4", ":0") }`, 1},
		{"ListenConfig.Listen tcp6", `package p; func f() { lc.Listen(ctx, "tcp6", ":0") }`, 1},
		{"net.ListenTCP", `package p; import "net"; func f() { net.ListenTCP("tcp", nil) }`, 1},
		{"http.ListenAndServe", `package p; import "net/http"; func f() { http.ListenAndServe(":0", nil) }`, 1},
		{"http.ListenAndServeTLS", `package p; import "net/http"; func f() { http.ListenAndServeTLS(":0", "", "", nil) }`, 1},
		{"loopback IPv4 literal", `package p; var a = "127.0.0.1:0"`, 1},
		{"loopback IPv6 bracketed literal", `package p; var a = "[::1]:0"`, 1},
		{"loopback IPv6 literal", `package p; var a = "::1"`, 1},
		{"method ListenAndServe", `package p; func f() { srv.ListenAndServe() }`, 1},
		{"variable network", `package p; import "net"; func f() { net.Listen(network, addr) }`, 1},
		{"raw string loopback", "package p; var a = `127.0.0.1:0`", 1},
		{"net.Listen unix", `package p; import "net"; func f() { net.Listen("unix", "/tmp/s") }`, 0},
		{"ListenConfig.Listen unix", `package p; func f() { lc.Listen(ctx, "unix", "/tmp/s") }`, 0},
		{"tcp outside a Listen call", `package p; import "net"; func f() { net.Dial("tcp", "example.com:80") }`, 0},
		{"no network", `package p; var a = "localhost"`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tcpListenerFindings(t, "p.go", tt.src); len(got) != tt.want {
				t.Errorf("findings = %v, want %d", got, tt.want)
			}
		})
	}
}

func TestIsTestCode(t *testing.T) {
	tests := []struct {
		name, file, build string
		want              bool
	}{
		{"plain production file", "a.go", "", false},
		{"test file", "a_test.go", "", true},
		{"test tag only", "a.go", "test", true},
		{"test or integration", "a.go", "test || integration", true},
		{"test and integration", "a.go", "test && integration", true},
		{"not test", "a.go", "!test", false},
		{"test or unix", "a.go", "test || unix", false},
		{"unix only", "a.go", "unix", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expr constraint.Expr
			if tt.build != "" {
				var err error
				if expr, err = constraint.Parse("//go:build " + tt.build); err != nil {
					t.Fatalf("parse constraint: %v", err)
				}
			}
			if got := isTestCode(tt.file, expr); got != tt.want {
				t.Errorf("isTestCode(%q, %q) = %v, want %v", tt.file, tt.build, got, tt.want)
			}
		})
	}
}

func TestNoTCPListenerInProduction(t *testing.T) {
	for _, rel := range productionGoFiles(t) {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, finding := range tcpListenerFindings(t, rel, string(src)) {
			t.Errorf("%s: %s: the server listens on a Unix socket only", rel, finding)
		}
	}
}

// removedTokenWords are the identifiers and file names of the shared-token
// version that no production code or current document may mention.
var removedTokenWords = []string{"authtoken", "YT2COLUMN_SERVER_PORT", "server-token"}

// upgradeHeading is the README section that tells an upgrading user which
// leftovers of the shared-token version to delete; it names the token file.
const upgradeHeading = "### Upgrading from the shared-token version"

// splitSection splits doc into the section that starts at the line equal to
// heading and ends before the next heading of the same or a higher level, and
// the rest of doc.
func splitSection(doc, heading string) (rest, section string) {
	level := len(heading) - len(strings.TrimLeft(heading, "#"))
	var kept, cut strings.Builder
	skipping := false
	fenced := false
	for line := range strings.Lines(doc) {
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, "```") {
			fenced = !fenced
		}
		if !fenced && trimmed == heading {
			skipping = true
			cut.WriteString(line)
			continue
		}
		if skipping && !fenced {
			n := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			if n > 0 && n <= level && strings.HasPrefix(trimmed[n:], " ") {
				skipping = false
			}
		}
		if skipping {
			cut.WriteString(line)
		} else {
			kept.WriteString(line)
		}
	}
	return kept.String(), cut.String()
}

// withoutSection returns doc without the section splitSection cuts.
func withoutSection(doc, heading string) string {
	rest, _ := splitSection(doc, heading)
	return rest
}

func TestWithoutSection(t *testing.T) {
	tests := []struct {
		name, doc, want string
	}{
		{
			"ends at the next heading of the same or higher level",
			"## A\nkeep a\n### Skip\ndrop\n#### Sub\ndrop too\n### Next\nkeep next\n## B\nkeep b\n",
			"## A\nkeep a\n### Next\nkeep next\n## B\nkeep b\n",
		},
		{
			"a hash line in a fenced block does not end the section",
			"### Skip\n```\n# comment\n```\ndrop\n## B\nkeep b\n",
			"## B\nkeep b\n",
		},
		{"section at end of file", "keep\n### Skip\ndrop\n", "keep\n"},
		{"absent heading", "keep\n", "keep\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withoutSection(tt.doc, "### Skip"); got != tt.want {
				t.Errorf("withoutSection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSharedTokenRemoved(t *testing.T) {
	// texts maps a repository-relative path to the text that must not name a
	// removed word.
	texts := map[string]string{}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}
	for _, rel := range productionGoFiles(t) {
		texts[rel] = read(rel)
	}
	readme := read("README.md")
	texts["README.md"] = withoutSection(readme, upgradeHeading)
	texts["CLAUDE.md"] = read("CLAUDE.md")
	texts["docs/translation_glossary.md"] = read("docs/translation_glossary.md")
	err := filepath.WalkDir(filepath.Join(repoRoot, "docs", "dev"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		texts[rel] = read(rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs/dev: %v", err)
	}
	_, upgrade := splitSection(readme, upgradeHeading)
	if !strings.Contains(upgrade, "server-token") {
		t.Fatalf("README.md %q section does not name server-token; the exemption is stale", upgradeHeading)
	}
	for _, rel := range slices.Sorted(maps.Keys(texts)) {
		for _, word := range removedTokenWords {
			if strings.Contains(strings.ToLower(texts[rel]), strings.ToLower(word)) {
				t.Errorf("%s mentions %q, removed with the shared token", rel, word)
			}
		}
	}
	// The upgrade section may name the token file but not the other words.
	for _, word := range removedTokenWords {
		if word != "server-token" && strings.Contains(strings.ToLower(upgrade), strings.ToLower(word)) {
			t.Errorf("README.md %q section mentions %q", upgradeHeading, word)
		}
	}
}

// hostDirs are the repository-relative directories of the Native Messaging
// host's production code.
var hostDirs = []string{"internal/nmhost/", "cmd/yt2column-nmhost/"}

// hostStdoutMain is the one host file that may reference standard output: the
// entry point wires it into the relay. No other host code writes to it, so the
// one reply frame is all the browser reads there.
const hostStdoutMain = "cmd/yt2column-nmhost/main.go"

// stdoutRefs are the references that write to standard output, by import
// path.
var stdoutRefs = map[string][]string{
	"os":      {"Stdout"},
	"fmt":     {"Print", "Printf", "Println"},
	"syscall": {"Stdout"},
}

// spawnRefs are the references that start another program, by import path.
// Importing os/exec is rejected by depguard instead.
var spawnRefs = map[string][]string{
	"os":      {"StartProcess"},
	"syscall": {"Exec", "ForkExec", "StartProcess"},
}

// hostFiles returns the production Go files of the host.
func hostFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, rel := range productionGoFiles(t) {
		if slices.ContainsFunc(hostDirs, func(dir string) bool { return strings.HasPrefix(rel, dir) }) {
			files = append(files, rel)
		}
	}
	if len(files) == 0 {
		t.Fatal("found no production Go files of the host")
	}
	return files
}

// selectorFindings parses src and describes every reference to a name in refs
// (pkg.Name through the file's own name for the import, including an alias)
// and every dot import of a package in refs, which would hide such a
// reference.
func selectorFindings(t *testing.T, name, src string, refs map[string][]string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var findings []string
	local := map[string]string{} // the file's name for an import -> its path
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("%s: unquote import %s: %v", name, imp.Path.Value, err)
		}
		if _, watched := refs[path]; !watched {
			continue
		}
		switch {
		case imp.Name == nil:
			local[filepath.Base(path)] = path
		case imp.Name.Name == ".":
			findings = append(findings, "dot import of "+path)
		default:
			local[imp.Name.Name] = path
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if path, ok := local[pkg.Name]; ok && slices.Contains(refs[path], sel.Sel.Name) {
			findings = append(findings, path+"."+sel.Sel.Name)
		}
		return true
	})
	return findings
}

// TestSelectorDetection checks the finder's mechanism (import names, aliases,
// dot imports, other packages) once per reference map; the maps' entries are
// the guards' declared scope and are not restated here.
func TestSelectorDetection(t *testing.T) {
	tests := []struct {
		name string
		refs map[string][]string
		src  string
		want int
	}{
		{"os.Stdout", stdoutRefs, `package p; import "os"; var w = os.Stdout`, 1},
		{"aliased import", stdoutRefs, `package p; import o "os"; var w = o.Stdout`, 1},
		{"dot import", stdoutRefs, `package p; import . "fmt"; func f() { Println() }`, 1},
		{"os.Stderr", stdoutRefs, `package p; import "os"; var w = os.Stderr`, 0},
		{"same name, other package", stdoutRefs, `package p; import "example.com/os"; var w = os.Stdout`, 0},
		{"os.StartProcess", spawnRefs, `package p; import "os"; var f = os.StartProcess`, 1},
		{"syscall.Getpid", spawnRefs, `package p; import "syscall"; var f = syscall.Getpid`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectorFindings(t, "p.go", tt.src, tt.refs); len(got) != tt.want {
				t.Errorf("findings = %v, want %d", got, tt.want)
			}
		})
	}
}

func TestHostStdoutOnlyInMain(t *testing.T) {
	for _, rel := range hostFiles(t) {
		if rel == hostStdoutMain {
			continue
		}
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, finding := range selectorFindings(t, rel, string(src), stdoutRefs) {
			t.Errorf("%s: %s: only %s may write to standard output, the browser's channel", rel, finding, hostStdoutMain)
		}
	}
}

func TestHostSpawnsNoProcess(t *testing.T) {
	for _, rel := range hostFiles(t) {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, finding := range selectorFindings(t, rel, string(src), spawnRefs) {
			t.Errorf("%s: %s: the host starts no other program", rel, finding)
		}
	}
}
