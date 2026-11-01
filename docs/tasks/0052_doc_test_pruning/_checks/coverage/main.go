//go:build test

// Command coverage compares two "go tool cover -func" outputs and reports every
// function of the product code whose coverage differs between them:
//
//	go run -tags test ./docs/tasks/0052_doc_test_pruning/_checks/coverage/ <before.txt> <after.txt>
//
// Make each output from the repository root with
//
//	go test -tags test -count=1 -coverpkg=./... -coverprofile=<profile> ./...
//	go tool cover -func=<profile> > <output>
//
// once before and once after the commit that removes or replaces document tests
// or their helpers. Both runs must pass: a run with a failing test is not a
// valid result.
//
// The product code is every Go file that is neither a _test.go file nor a file
// whose build constraint starts with "test" (test, test || integration,
// test && race, ...). _test.go files never appear in the output; the files
// with a test constraint are found with git grep in the working tree and their
// lines are dropped, as is the total line. The output names each file by its
// module path, so the module prefix is removed before the comparison.
//
// A function is identified by its file, its line, and its name. The product
// code does not change in this task, so a function on one side and not on the
// other is reported as a difference too.
//
// Run it from the repository root. make test and CI do not run it, and it is
// not maintained after the merge. It reads no environment variable: the
// underscore keeps the directory out of go list, and the //go:build test first
// line makes the repository-wide environment access check treat the file as
// test code, so that check does not scan it.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// modulePrefix is the module path the cover tool prefixes each file with.
const modulePrefix = "github.com/isseis/yt2column/"

// testConstraintPattern finds the files whose build constraint starts with the
// test tag. The tag must end there, so a tag such as "testing" is not matched.
const testConstraintPattern = `^//go:build test([^A-Za-z0-9_.]|$)`

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: coverage <before.txt> <after.txt>")
		os.Exit(2)
	}
	testFiles, err := testConstraintFiles()
	if err != nil {
		fail("list the files with a test build constraint", err)
	}
	before, err := readFunc(os.Args[1], testFiles)
	if err != nil {
		fail("read the first output", err)
	}
	after, err := readFunc(os.Args[2], testFiles)
	if err != nil {
		fail("read the second output", err)
	}
	fmt.Printf("before       %s (%d product functions)\n", os.Args[1], len(before))
	fmt.Printf("after        %s (%d product functions)\n", os.Args[2], len(after))
	if len(before) == 0 || len(after) == 0 {
		fail("compare", fmt.Errorf("an output has no product function"))
	}

	keys := make([]string, 0, len(before)+len(after))
	for k := range before {
		keys = append(keys, k)
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var differ []string
	for _, k := range keys {
		b, inBefore := before[k]
		a, inAfter := after[k]
		switch {
		case !inBefore:
			differ = append(differ, fmt.Sprintf("%s: only after (%s)", k, a))
		case !inAfter:
			differ = append(differ, fmt.Sprintf("%s: only before (%s)", k, b))
		case a != b:
			differ = append(differ, fmt.Sprintf("%s: %s -> %s", k, b, a))
		}
	}
	if len(differ) > 0 {
		fmt.Printf("NG   %d product function(s) differ\n", len(differ))
		for _, d := range differ {
			fmt.Printf("     %s\n", d)
		}
		os.Exit(1)
	}
	fmt.Printf("OK   all %d product functions have the same coverage\n", len(keys))
}

// readFunc reads one "go tool cover -func" output and returns the coverage of
// each product function, keyed by "file:line name".
func readFunc(path string, testFiles map[string]struct{}) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "total:" {
			continue
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected line %q", scanner.Text())
		}
		location, name, percent := fields[0], fields[1], fields[2]
		file, line, ok := strings.Cut(strings.TrimSuffix(location, ":"), ":")
		if !ok {
			return nil, fmt.Errorf("no line number in %q", location)
		}
		rel, ok := strings.CutPrefix(file, modulePrefix)
		if !ok {
			return nil, fmt.Errorf("file %q is outside the module", file)
		}
		if _, isTest := testFiles[rel]; isTest {
			continue
		}
		key := rel + ":" + line + " " + name
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("function %s appears twice", key)
		}
		out[key] = percent
	}
	return out, scanner.Err()
}

// testConstraintFiles lists the Go files of the working tree whose build
// constraint starts with the test tag.
func testConstraintFiles() (map[string]struct{}, error) {
	out, err := exec.Command("git", "grep", "-lE", testConstraintPattern, "--", "*.go").Output()
	if err != nil {
		return nil, fmt.Errorf("git grep: %w", err)
	}
	files := map[string]struct{}{}
	for line := range strings.SplitSeq(string(out), "\n") {
		if line != "" {
			files[line] = struct{}{}
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("git grep found no file")
	}
	return files, nil
}

// fail prints an error that stops the comparison and exits non-zero.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "coverage: %s: %v\n", what, err)
	os.Exit(1)
}
