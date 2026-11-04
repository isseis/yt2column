// Command realrun checks that the two coverage-compare entry points pass on
// the repository's real test suites, and that the values they print match
// values the check measures on its own:
//
//	go run ./docs/tasks/0055_coverage_compare/_checks/realrun \
//	  -go-test <Go test file> -ext-test <extension test file>
//
// It clones the current HEAD into a temporary directory, links the user's
// extension/node_modules into the clone entry by entry, and commits the
// deletion of the two named test files there. The clone is never pushed, so
// the repository keeps its tests. Both make targets then run BASE=HEAD~1, and
// the check compares the printed commit IDs, tree ID, toolchain version, test
// counts, and file and function counts with values it reads from the clone and
// from go and node.
//
// The two test files must contain no product code, so deleting them can only
// remove tests: a Go test file whose functions only exercise test helpers, and
// an extension file under extension/test/. Each file's test count is measured
// separately: Go by parsing the file at BASE, the extension by running node
// with the TAP reporter on that file at BASE.
//
// The check does not read the process environment: the commands it starts
// inherit it by default, and the task's environment-access check scans this
// directory. make test and CI do not run it because go list ignores the
// directory whose name starts with an underscore; it is run once on the plan's
// PR branch, and it is not maintained after the merge.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	goTest := flag.String("go-test", "", "Go test file whose deletion is committed in the clone")
	extTest := flag.String("ext-test", "", "extension test file whose deletion is committed in the clone")
	flag.Parse()
	if *goTest == "" || *extTest == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: realrun -go-test <file.go> -ext-test <file.ts>")
		os.Exit(2)
	}
	if err := run(*goTest, *extTest); err != nil {
		fmt.Fprintf(os.Stderr, "realrun: %v\n", err)
		os.Exit(1)
	}
}

// run performs the whole check and returns the first mismatch.
func run(goTest, extTest string) error {
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	base, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "coverage-compare-realrun-")
	if err != nil {
		return fmt.Errorf("create the temporary directory: %w", err)
	}
	defer os.RemoveAll(temp)

	clone := filepath.Join(temp, "repo")
	if _, err := git(root, "clone", "--shared", "--no-checkout", root, clone); err != nil {
		return err
	}
	if _, err := git(clone, "-c", "core.logAllRefUpdates=false", "-c", "core.hooksPath=/dev/null", "checkout", "--detach", base); err != nil {
		return err
	}
	if err := linkNodeModules(filepath.Join(root, "extension", "node_modules"), filepath.Join(clone, "extension", "node_modules")); err != nil {
		return err
	}

	goTests, err := countGoTests(base, goTest, clone)
	if err != nil {
		return err
	}
	extTests, err := countExtensionTests(clone, extTest)
	if err != nil {
		return err
	}

	if _, err := git(clone, "rm", "--quiet", "--", goTest, extTest); err != nil {
		return err
	}
	if _, err := git(clone,
		"-c", "user.name=coverage-compare realrun",
		"-c", "user.email=realrun@example.invalid",
		"-c", "commit.gpgSign=false",
		"-c", "core.hooksPath=/dev/null",
		"commit", "--quiet", "-m", "test: delete one Go test file and one extension test file",
	); err != nil {
		return err
	}
	head, err := git(clone, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	tree, err := git(clone, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return err
	}
	parent, err := git(clone, "rev-parse", "HEAD~1")
	if err != nil {
		return err
	}
	if parent != base {
		return fmt.Errorf("the deletion commit's parent is %s, want the original HEAD %s", parent, base)
	}

	goVersion, err := commandOutput("", "go", "env", "GOVERSION")
	if err != nil {
		return err
	}
	nodeVersion, err := commandOutput("", "node", "--version")
	if err != nil {
		return err
	}

	goOut, err := makeTarget(clone, "coverage-compare")
	if err != nil {
		return err
	}
	if err := checkResult(goOut, want{
		base: base, head: head, tree: tree, toolchain: goVersion,
		testDiff: goTests, deleted: 0,
	}); err != nil {
		return fmt.Errorf("coverage-compare: %w", err)
	}
	fmt.Printf("realrun: go  base %s head %s deleted tests %d\n", base, head, goTests)

	extOut, err := makeTarget(clone, "ext-coverage-compare")
	if err != nil {
		return err
	}
	if err := checkResult(extOut, want{
		base: base, head: head, tree: tree, toolchain: nodeVersion,
		testDiff: extTests, deleted: 0,
	}); err != nil {
		return fmt.Errorf("ext-coverage-compare: %w", err)
	}
	fmt.Printf("realrun: ext base %s head %s deleted tests %d\n", base, head, extTests)
	fmt.Println("realrun: OK")
	return nil
}

// want is what the printed values must be.
type want struct {
	base, head string
	tree       string
	toolchain  string
	testDiff   int
	deleted    int
}

// checkResult parses one tool output and reports the first value that is not
// what want says, or a passing run that is missing a value.
func checkResult(out string, w want) error {
	last := lastLine(out)
	if last != "coverage-compare: PASS" {
		return fmt.Errorf("the last line is %q, want a passing run", last)
	}
	r, err := parseResult(out)
	if err != nil {
		return err
	}
	if r.base != w.base {
		return fmt.Errorf("base is %q, want %q", r.base, w.base)
	}
	if r.head != w.head {
		return fmt.Errorf("head is %q, want %q", r.head, w.head)
	}
	if r.tree != w.tree {
		return fmt.Errorf("tree is %q, want %q", r.tree, w.tree)
	}
	if r.toolchain != w.toolchain {
		return fmt.Errorf("toolchain is %q, want %q", r.toolchain, w.toolchain)
	}
	if got := r.testsBase - r.testsHead; got != w.testDiff {
		return fmt.Errorf("the test counts differ by %d (base %d, head %d), want the %d deleted tests", got, r.testsBase, r.testsHead, w.testDiff)
	}
	if r.deleted != w.deleted {
		return fmt.Errorf("deleted is %d, want %d", r.deleted, w.deleted)
	}
	for _, point := range []struct {
		name  string
		files int
		funcs int
		tests int
	}{
		{"base", r.filesBase, r.functionsBase, r.testsBase},
		{"head", r.filesHead, r.functionsHead, r.testsHead},
	} {
		if point.tests <= 0 {
			return fmt.Errorf("%s has %d tests, want more than 0", point.name, point.tests)
		}
		if point.files <= 0 {
			return fmt.Errorf("%s has %d files, want more than 0", point.name, point.files)
		}
		if point.funcs <= 0 {
			return fmt.Errorf("%s has %d functions, want more than 0", point.name, point.funcs)
		}
	}
	return nil
}

// printed holds the values read from one tool output.
type printed struct {
	base, head, tree, toolchain  string
	testsBase, testsHead         int
	filesBase, filesHead         int
	functionsBase, functionsHead int
	deleted                      int
}

// parseResult reads the aligned "<label> <value>" lines of one tool output.
func parseResult(out string) (printed, error) {
	var r printed
	var err error
	deletedLines := 0
	for _, line := range strings.Split(out, "\n") {
		label, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch label {
		case "base":
			r.base = value
		case "head":
			fields := strings.Fields(value)
			if len(fields) != 3 || fields[1] != "tree" {
				return r, fmt.Errorf("read the head line %q", line)
			}
			r.head, r.tree = fields[0], fields[2]
		case "toolchain":
			r.toolchain = value
		case "tests":
			r.testsBase, r.testsHead, err = measures(value)
			if err != nil {
				return r, fmt.Errorf("read the tests line: %w", err)
			}
		case "files":
			r.filesBase, r.filesHead, err = measures(value)
			if err != nil {
				return r, fmt.Errorf("read the files line: %w", err)
			}
		case "functions":
			r.functionsBase, r.functionsHead, err = measures(value)
			if err != nil {
				return r, fmt.Errorf("read the functions line: %w", err)
			}
		case "deleted":
			deletedLines++
			if deletedLines == 1 {
				r.deleted, err = strconv.Atoi(value)
				if err != nil {
					return r, fmt.Errorf("read the deleted count %q: %w", value, err)
				}
			}
		}
	}
	if deletedLines != 1 {
		return r, fmt.Errorf("the output has %d deleted lines, want 1 for a run that deleted no function", deletedLines)
	}
	return r, nil
}

// measures reads a "base N head M" value.
func measures(value string) (int, int, error) {
	fields := strings.Fields(value)
	if len(fields) != 4 || fields[0] != "base" || fields[2] != "head" {
		return 0, 0, fmt.Errorf("value %q is not \"base N head M\"", value)
	}
	base, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("read the base value %q: %w", fields[1], err)
	}
	head, err := strconv.Atoi(fields[3])
	if err != nil {
		return 0, 0, fmt.Errorf("read the head value %q: %w", fields[3], err)
	}
	return base, head, nil
}

// lastLine is the last non-empty line of out.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// repositoryRoot returns the current directory, which must be the root of a
// git repository.
func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("read the current directory: %w", err)
	}
	top, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("resolve the current directory: %w", err)
	}
	resolvedTop, err := filepath.EvalSymlinks(top)
	if err != nil {
		return "", fmt.Errorf("resolve the repository root: %w", err)
	}
	if resolvedDir != resolvedTop {
		return "", fmt.Errorf("run the check from the repository root %s, not %s", resolvedTop, resolvedDir)
	}
	return resolvedTop, nil
}

// git runs one git command in dir and returns its trimmed standard output.
func git(dir string, args ...string) (string, error) {
	return commandOutput(dir, "git", args...)
}

// commandOutput runs one command in dir and returns its trimmed standard
// output.
func commandOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w (stderr: %s)", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// makeTarget runs one coverage-compare target at BASE=HEAD~1 in dir, echoes
// its standard output, and returns that output.
func makeTarget(dir, target string) (string, error) {
	cmd := exec.Command("make", target, "BASE=HEAD~1")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("make %s: %w", target, err)
	}
	return stdout.String(), nil
}

// linkNodeModules gives the clone the user's extension dependencies: a real
// directory holding one symbolic link per entry, as the tool does for its own
// checkouts.
func linkNodeModules(userModules, cloneModules string) error {
	entries, err := os.ReadDir(userModules)
	if err != nil {
		return fmt.Errorf("read %s: %w", userModules, err)
	}
	if err := os.Mkdir(cloneModules, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", cloneModules, err)
	}
	for _, entry := range entries {
		if err := os.Symlink(filepath.Join(userModules, entry.Name()), filepath.Join(cloneModules, entry.Name())); err != nil {
			return fmt.Errorf("link %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// countGoTests reads the Go test file at BASE and counts its top-level test
// functions, which is what go test -json reports as tests.
func countGoTests(base, name, clone string) (int, error) {
	src, err := commandOutput(clone, "git", "show", base+":"+name)
	if err != nil {
		return 0, err
	}
	file, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	count := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name == "TestMain" || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		if isTestingT(fn.Type.Params) {
			count++
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("%s has no top-level test function", name)
	}
	return count, nil
}

// isTestingT reports whether params is exactly one *testing.T parameter.
func isTestingT(params *ast.FieldList) bool {
	if params == nil || len(params.List) != 1 || len(params.List[0].Names) != 1 {
		return false
	}
	star, ok := params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "testing" && selector.Sel.Name == "T"
}

// countExtensionTests runs the extension test file at BASE in the clone with
// the TAP reporter and returns the "# tests" count node reports for it.
func countExtensionTests(clone, name string) (int, error) {
	file, ok := strings.CutPrefix(name, "extension/")
	if !ok {
		return 0, fmt.Errorf("%s is not under extension/", name)
	}
	tap, err := commandOutput(filepath.Join(clone, "extension"), "node", "--test", "--test-reporter=tap", file)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(tap, "\n") {
		rest, ok := strings.CutPrefix(line, "# tests ")
		if !ok {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return 0, fmt.Errorf("read the TAP test count %q: %w", rest, err)
		}
		if count == 0 {
			return 0, errors.New("the extension test file reports no test")
		}
		return count, nil
	}
	return 0, errors.New("the TAP output has no # tests line")
}
