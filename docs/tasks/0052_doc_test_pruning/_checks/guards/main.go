//go:build test

// Command guards verifies that the four places that write the documentation
// guard command name the same Go packages:
//
//	go run -tags test ./docs/tasks/0052_doc_test_pruning/_checks/guards/ [-final] [<commit>]
//
// The four places are the doc-guard job of the CI workflow, the Development
// Notes section of CLAUDE.md, the CHECKS step of the task branch handoff
// command, and the procedure for changing the automated reviewer list in the
// guard tool guide. Each writes the command as "go test -tags test" followed by
// the packages. A package written with a trailing "/..." names the same
// directory as one without it, so "./cmd/codexreview/..." and
// "./cmd/codexreview" count as the same package.
//
// The command is the one that names ./cmd/yt2column; other "go test -tags test"
// lines, such as the examples in CLAUDE.md's command list, are ignored. A file
// must hold exactly one such command: two would make it unclear which one
// counts.
//
// With -final, the shared package set must also be the set of packages whose
// test files import internal/doctestutil, minus cmd/yt2column-nmhost, which the
// guard commands leave out, and the task _checks programs. This is a proxy for
// the set of packages where a document test remains: it matches only after the
// last pull request has replaced the remaining document reads with doctestutil,
// and it still leaves out the native host, which reads documents but is not
// part of the doc-guard job. Use it in that pull request.
//
// Without a commit it reads the working tree; with one it reads that commit.
//
// Run it from the repository root, once per pull request. make test and CI do
// not run it, and it is not maintained after the merge. It reads no
// environment variable: the underscore keeps the directory out of go list, and
// the //go:build test first line makes the repository-wide environment access
// check treat the file as test code, so that check does not scan it.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

// location is one place that writes the documentation guard command.
type location struct {
	path string
	what string
}

// locations are the four places that write the command, in output order.
var locations = []location{
	{".github/workflows/ci.yml", "doc-guard job"},
	{"CLAUDE.md", "Development Notes"},
	{".claude/commands/_lib/task-pr-handoff.md", "CHECKS"},
	{"docs/dev/guides/fixprguard_guide.md", "automated reviewer list procedure"},
}

// commandRE matches the documentation guard command up to the first character
// that cannot be part of its package list.
var commandRE = regexp.MustCompile("go test -tags test +([^`);,]*)")

// firstPackage is the package that tells the documentation guard command apart
// from the other "go test -tags test" lines.
const firstPackage = "./cmd/yt2column"

// doctestutilImport is the import path a document test uses to read documents.
const doctestutilImport = "github.com/isseis/yt2column/internal/doctestutil"

// excludedPackages are the packages whose test imports do not count for -final:
// the native host is not part of the doc-guard job, and the _checks programs of
// the tasks are not packages of the module.
var excludedPackages = []string{"./cmd/yt2column-nmhost", "./docs/tasks"}

func main() {
	final := flag.Bool("final", false, "also require the set of packages whose tests read documents through doctestutil")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: guards [-final] [<commit>]")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	rev := flag.Arg(0)
	if rev != "" {
		id, err := gitOutput("rev-parse", rev)
		if err != nil {
			fail("rev-parse", err)
		}
		rev = id
		fmt.Printf("commit %s\n", rev)
	} else {
		fmt.Println("commit (working tree)")
	}

	failed := false
	sets := make([][]string, len(locations))
	fmt.Println("---- commands")
	for i, loc := range locations {
		text, err := readFile(rev, loc.path)
		if err != nil {
			fail("read "+loc.path, err)
		}
		pkgs, err := packages(text)
		if err != nil {
			failed = true
			fmt.Printf("NG   %-40s %s: %v\n", loc.path, loc.what, err)
			continue
		}
		sets[i] = pkgs
		fmt.Printf("OK   %-40s %s: %s\n", loc.path, loc.what, strings.Join(pkgs, " "))
	}

	fmt.Println("---- comparison")
	common := sets[0]
	for i, pkgs := range sets {
		if i == 0 || pkgs == nil {
			continue
		}
		if !slices.Equal(pkgs, common) {
			failed = true
			fmt.Printf("NG   %s differs from %s\n", locations[i].path, locations[0].path)
			fmt.Printf("     %s: %s\n", locations[0].path, strings.Join(common, " "))
			fmt.Printf("     %s: %s\n", locations[i].path, strings.Join(pkgs, " "))
		}
	}
	if !failed {
		fmt.Printf("OK   the four places name the same %d package(s): %s\n", len(common), strings.Join(common, " "))
	}

	if *final {
		want, err := packagesWithDoctestutil(rev)
		if err != nil {
			fail("find the packages whose tests read documents through doctestutil", err)
		}
		fmt.Println("---- packages whose tests read documents through doctestutil")
		for _, pkg := range want {
			fmt.Println(pkg)
		}
		if !slices.Equal(common, want) {
			failed = true
			fmt.Println("NG   the guard commands do not name the packages whose tests read documents through doctestutil")
			fmt.Printf("     guard commands: %s\n", strings.Join(common, " "))
			fmt.Printf("     document tests: %s\n", strings.Join(want, " "))
		} else {
			fmt.Println("OK   the guard commands name the packages whose tests read documents through doctestutil")
		}
	}
	if failed {
		os.Exit(1)
	}
}

// packages returns the sorted package list of the documentation guard command
// in text. It fails when text holds no such command, or more than one.
func packages(text string) ([]string, error) {
	var found [][]string
	for _, m := range commandRE.FindAllStringSubmatch(text, -1) {
		var pkgs []string
		for _, field := range strings.Fields(m[1]) {
			if !strings.HasPrefix(field, "./") {
				break
			}
			pkgs = append(pkgs, normalizePackage(field))
		}
		if slices.Contains(pkgs, firstPackage) {
			found = append(found, pkgs)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("no documentation guard command")
	case 1:
		slices.Sort(found[0])
		return found[0], nil
	}
	return nil, fmt.Errorf("%d documentation guard commands", len(found))
}

// normalizePackage drops a trailing "/..." or "/" so that a package written
// with either names the same directory.
func normalizePackage(pkg string) string {
	pkg = strings.TrimSuffix(pkg, "/...")
	return strings.TrimSuffix(pkg, "/")
}

// packagesWithDoctestutil returns the sorted packages whose test files import
// internal/doctestutil at rev, minus excludedPackages.
func packagesWithDoctestutil(rev string) ([]string, error) {
	args := []string{"grep", "-l", doctestutilImport}
	if rev != "" {
		args = append(args, rev)
	}
	args = append(args, "--", "*_test.go")
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil // git grep exits 1 when nothing matches
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	seen := map[string]struct{}{}
	var pkgs []string
	for _, path := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if path == "" {
			continue
		}
		// git grep prefixes each path with the commit when it reads a commit.
		path = strings.TrimPrefix(path, rev+":")
		dir := "."
		if i := strings.LastIndex(path, "/"); i >= 0 {
			dir = path[:i]
		}
		pkg := "./" + dir
		if dir == "." {
			pkg = "."
		}
		if excluded(pkg) {
			continue
		}
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		pkgs = append(pkgs, pkg)
	}
	slices.Sort(pkgs)
	return pkgs, nil
}

// excluded reports whether pkg is one of excludedPackages or lies under one.
func excluded(pkg string) bool {
	for _, e := range excludedPackages {
		if pkg == e || strings.HasPrefix(pkg, e+"/") {
			return true
		}
	}
	return false
}

// readFile returns the content of path at rev, or in the working tree when rev
// is empty.
func readFile(rev, path string) (string, error) {
	if rev == "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return gitOutput("show", rev+":"+path)
}

// fail prints an error that stops every check and exits non-zero.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "guards: %s: %v\n", what, err)
	os.Exit(1)
}

// gitOutput runs git with args and returns its standard output without the
// trailing newline.
func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
