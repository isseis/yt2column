//go:build test

// Command _checks runs the one-off checks for task 0075 on one commit: the
// files the task deletes are gone, no file outside the task records and the
// ADRs holds a line that matches the word list beyond the listed exclusions,
// and neither the Makefile nor the CI workflows refer to this directory. It
// reads every file, its word list included, from the given commit through
// git, never from the working tree; the only working-tree file it reads is
// its own source, to print the source's blob ID.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0075check ./docs/tasks/0075_remove_lightweight_process/_checks/
//	/tmp/0075check <commit>
//
// It prints the commit ID it checked, the blob IDs of its source and of the
// data file it read, and the result of each check. It exits 0 when every
// check passes, 1 on a violation, and 2 when it cannot read its input or
// cannot parse git's output. make test and CI do not run it; the directory
// starts with an underscore, which keeps it out of go list.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	taskDir    = "docs/tasks/0075_remove_lightweight_process/"
	dataPath   = taskDir + "_checks/data.json"
	sourcePath = taskDir + "_checks/main.go"
)

// data holds the deleted files, the word list, the exclusions, the scope of
// the word search, and the files and pattern of the CI check. The words
// include Japanese ones, which the Go source does not hold.
type data struct {
	Deleted []string `json:"deleted"`
	Words   []string `json:"words"`
	// Excluded are the matched lines that are not references, each written as
	// a path and the whole content of the line. Each must match exactly once.
	Excluded []exclusion `json:"excluded"`
	// OutOfScope are the path prefixes the word search leaves out, and
	// BackInScope the prefixes inside them it searches all the same.
	OutOfScope  []string `json:"out_of_scope"`
	BackInScope []string `json:"back_in_scope"`
	// CIFiles are the pathspecs that must not refer to CIPattern.
	CIFiles   []string `json:"ci_files"`
	CIPattern string   `json:"ci_pattern"`
}

type exclusion struct {
	Path string `json:"path"`
	Line string `json:"line"`
}

// match is one line git grep reported.
type match struct {
	path string
	line int
	text string
}

// errUnreadable marks a failure to read the input or to parse git's output,
// which ends the run with exit status 2 rather than reporting a violation.
var errUnreadable = errors.New("unreadable input")

// gitConfig neutralizes the user's and the repository's settings that change
// what git grep and git show print.
var gitConfig = []string{
	"-c", "core.quotePath=false",
	"-c", "color.ui=never",
	"-c", "color.grep=never",
	"-c", "grep.lineNumber=false",
	"-c", "grep.column=false",
	"-c", "grep.fullName=false",
	"-c", "grep.patternType=default",
	"-c", "grep.extendedRegexp=false",
}

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0075check:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func run(args []string) (bool, error) {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		return false, fmt.Errorf("%w: usage: 0075check <commit>", errUnreadable)
	}
	commit, err := gitText("rev-parse", "--verify", "--end-of-options", args[0]+"^{commit}")
	if err != nil {
		return false, err
	}
	commit = strings.TrimSpace(commit)
	sourceBlob, err := gitText("hash-object", "--no-filters", "--", sourcePath)
	if err != nil {
		return false, err
	}
	dataBlob, err := gitText("rev-parse", "--verify", commit+":"+dataPath)
	if err != nil {
		return false, err
	}
	fmt.Printf("commit %s\nmain.go blob %s\ndata.json blob %s\n", commit, strings.TrimSpace(sourceBlob), strings.TrimSpace(dataBlob))

	d, err := loadData(commit)
	if err != nil {
		return false, err
	}

	checks := []struct {
		name string
		desc string
		run  func() ([]string, error)
	}{
		{"deleted", "none of the deleted files is in the commit", func() ([]string, error) { return checkDeleted(commit, d) }},
		{"words", "outside the task records and the ADRs, no line matches the word list beyond the exclusions, and each exclusion matches exactly once", func() ([]string, error) { return checkWords(commit, d) }},
		{"ci", "the Makefile and the CI workflows do not refer to this directory", func() ([]string, error) { return checkCI(commit, d) }},
	}
	failed := 0
	for _, check := range checks {
		found, err := check.run()
		if err != nil {
			return false, fmt.Errorf("%s: %w", check.name, err)
		}
		if len(found) == 0 {
			fmt.Printf("OK   %s: %s\n", check.name, check.desc)
			continue
		}
		for _, v := range found {
			fmt.Printf("FAIL %s %s\n", check.name, v)
		}
		failed += len(found)
	}
	if failed > 0 {
		fmt.Printf("FAIL %d violation(s)\n", failed)
		return false, nil
	}
	fmt.Println("OK every check passed")
	return true, nil
}

// loadData reads the data file from the checked commit and rejects unknown
// members and an empty list where a check needs one.
func loadData(commit string) (data, error) {
	var d data
	raw, err := gitText("show", "--no-textconv", commit+":"+dataPath)
	if err != nil {
		return d, err
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("%w: %s: %v", errUnreadable, dataPath, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return d, fmt.Errorf("%w: %s: trailing data", errUnreadable, dataPath)
	}
	if len(d.Deleted) == 0 || len(d.Words) == 0 || len(d.CIFiles) == 0 || d.CIPattern == "" {
		return d, fmt.Errorf("%w: %s: a required list is empty", errUnreadable, dataPath)
	}
	for _, w := range d.Words {
		if w == "" {
			return d, fmt.Errorf("%w: %s: an empty word", errUnreadable, dataPath)
		}
	}
	for _, e := range d.Excluded {
		if e.Path == "" || e.Line == "" {
			return d, fmt.Errorf("%w: %s: an exclusion without a path or a line", errUnreadable, dataPath)
		}
	}
	return d, nil
}

// checkDeleted reports each deleted file the commit still holds. git ls-tree
// must succeed and print nothing.
func checkDeleted(commit string, d data) ([]string, error) {
	args := append([]string{"ls-tree", "-r", "--full-tree", "--name-only", commit, "--"}, d.Deleted...)
	out, err := gitText(args...)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, path := range splitLines(out) {
		found = append(found, path+": the file is still in the commit")
	}
	return found, nil
}

// checkWords reports every line of the commit, in the search scope, that
// matches a word of the list and is not an exclusion, and every exclusion that
// does not match exactly once.
func checkWords(commit string, d data) ([]string, error) {
	args := []string{"--attr-source=" + commit, "grep", "-z", "-n", "-i", "-F", "--no-color", "--no-column", "--no-textconv", "--full-name"}
	for _, w := range d.Words {
		args = append(args, "-e", w)
	}
	args = append(args, commit, "--", ":(top)")
	matches, err := grep(commit, args)
	if err != nil {
		return nil, err
	}
	hits := make([]int, len(d.Excluded))
	var found []string
	for _, m := range matches {
		if !inScope(m.path, d) {
			continue
		}
		excluded := false
		for i, e := range d.Excluded {
			if m.path == e.Path && m.text == e.Line {
				hits[i]++
				excluded = true
			}
		}
		if !excluded {
			found = append(found, fmt.Sprintf("%s:%d: %s", m.path, m.line, m.text))
		}
	}
	for i, e := range d.Excluded {
		if hits[i] != 1 {
			found = append(found, fmt.Sprintf("%s: the exclusion %q matched %d time(s), want 1", e.Path, e.Line, hits[i]))
		}
	}
	return found, nil
}

// inScope reports whether the word search covers path: a path outside every
// out-of-scope prefix, or inside a prefix brought back into scope.
func inScope(path string, d data) bool {
	for _, p := range d.BackInScope {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	for _, p := range d.OutOfScope {
		if strings.HasPrefix(path, p) {
			return false
		}
	}
	return true
}

// checkCI reports every line of the CI files that holds the pattern.
func checkCI(commit string, d data) ([]string, error) {
	args := []string{"--attr-source=" + commit, "grep", "-z", "-n", "-F", "--no-color", "--no-column", "--no-textconv", "--full-name", "-e", d.CIPattern, commit, "--"}
	for _, f := range d.CIFiles {
		args = append(args, ":(top)"+f)
	}
	matches, err := grep(commit, args)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, m := range matches {
		found = append(found, fmt.Sprintf("%s:%d: %s", m.path, m.line, m.text))
	}
	return found, nil
}

// grep runs git grep with -z and parses each output line as
// "<commit>:<path>\x00<line number>\x00<content>". Exit status 1 with no
// output means no match; any other failure, any warning on standard error,
// and any line of another shape (such as "Binary file ... matches") is
// unreadable. The callers read attributes from the checked commit
// (--attr-source), so the working tree's .gitattributes cannot change the
// result.
func grep(commit string, args []string) ([]match, error) {
	cmd := exec.Command("git", append(append([]string{}, gitConfig...), args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 && len(out) == 0 && stderr.Len() == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: git grep: %v: %s", errUnreadable, err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() > 0 {
		return nil, fmt.Errorf("%w: git grep wrote to standard error: %s", errUnreadable, strings.TrimSpace(stderr.String()))
	}
	prefix := commit + ":"
	var matches []match
	for _, line := range splitLines(string(out)) {
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) != 3 || !strings.HasPrefix(parts[0], prefix) {
			return nil, fmt.Errorf("%w: unexpected git grep line %q", errUnreadable, line)
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil || n < 1 {
			return nil, fmt.Errorf("%w: unexpected line number in %q", errUnreadable, line)
		}
		path := strings.TrimPrefix(parts[0], prefix)
		if path == "" {
			return nil, fmt.Errorf("%w: empty path in %q", errUnreadable, line)
		}
		matches = append(matches, match{path: path, line: n, text: parts[2]})
	}
	return matches, nil
}

// splitLines splits git's output into lines, with none for empty output.
func splitLines(out string) []string {
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// gitText runs git with the neutralizing settings and returns its standard
// output; any failure is unreadable.
func gitText(args ...string) (string, error) {
	cmd := exec.Command("git", append(append([]string{}, gitConfig...), args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: git %s: %v: %s", errUnreadable, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
