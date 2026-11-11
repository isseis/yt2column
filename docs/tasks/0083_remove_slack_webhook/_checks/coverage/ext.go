//go:build test

// The extension side compares two lcov reports of the extension suite, made
// at two commits, function by function:
//
//	<binary> ext <before-commit> <after-commit> <before.lcov> <after.lcov>
//
// Make each report from the extension directory, with the commit checked out
// in a clean working tree, with
//
//	node --test --experimental-test-coverage --test-reporter=lcov \
//		--test-reporter-destination=<report> --test-reporter=dot \
//		--test-reporter-destination=/dev/null "test/**/*.test.ts"
//
// Both runs must pass: a run with a failing test is not a valid result.
//
// A report holds only the files some test loaded, so a function missing from
// the after report is not proof it was removed. The source at the after
// commit is read (git show, never the working tree): a product file of the
// before report that still exists at the after commit but is missing from the
// after report stops the run, and a function missing from the after report
// whose name still appears in its file at the after commit is listed for the
// reader to check (a name can survive as another declaration, such as a
// constant).
//
// A file is product code when its path, relative to the extension directory,
// is under src/ or scripts/. Each FN record is paired with the FNDA record at
// the same position of its section. A named function is matched by its file
// and name. node names a function it cannot name anonymous_<n>, where n is the
// function's index among all the functions of the file, so that name moves
// when an earlier function is added or removed: anonymous functions are not matched, and the files
// whose number of anonymous functions changed are listed. A name used more
// than once in a file (two methods of one name, two arrow functions assigned
// to one property) is matched occurrence by occurrence, in line order, when
// both sides have the same number of occurrences; otherwise it cannot be
// matched and stops the run.
//
// It prints the functions found only before (the removed ones) and only after
// (the added ones), the functions that ran before and not after, which fail
// the comparison, and the files whose number of anonymous functions changed.
//
// Exit status: 0 when no function matched on both sides went from run to not
// run; 1 when one did; 2 when the comparison cannot be made (an unreadable
// report, a record the reports do not hold, an FN record without its FNDA
// record, a side without a product function, a name whose occurrences differ
// in number between the sides, or a file the after report lacks although the
// after commit has it).
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// anonymousName matches the name node gives a function it cannot name.
var anonymousName = regexp.MustCompile(`^anonymous_[0-9]+$`)

// lcovSkipped are the record kinds read past: their values take no part in
// the comparison.
var lcovSkipped = []string{"TN:", "FNF:", "FNH:", "DA:", "LF:", "LH:", "BRDA:", "BRF:", "BRH:"}

// extFunction is one named function of a product file: its first line and how
// many times it ran.
type extFunction struct {
	line int
	runs int
}

// extFile is one product file of a report: its named functions by name, in
// line order, and the number of its anonymous functions.
type extFile struct {
	named     map[string][]extFunction
	anonymous int
}

// runExt compares two lcov reports and returns the exit status.
func runExt(args []string) int {
	if len(args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: coverage ext <before-commit> <after-commit> <before.lcov> <after.lcov>")
		return 2
	}
	// The after commit's source decides what was removed, so an unknown
	// commit, which would make every file look removed, stops the run.
	if _, err := gitShow(args[1], "extension/package.json"); err != nil {
		stop("read the after commit", err)
	}
	before, err := loadLCOV(args[2])
	if err != nil {
		stop("read the first side", err)
	}
	after, err := loadLCOV(args[3])
	if err != nil {
		stop("read the second side", err)
	}
	fmt.Printf("before  %s %s (%d product files)\n", args[0], args[2], len(before))
	fmt.Printf("after   %s %s (%d product files)\n", args[1], args[3], len(after))
	if namedCount(before) == 0 || namedCount(after) == 0 {
		stop("compare", fmt.Errorf("a side has no named product function"))
	}
	status, err := reportExt(args[1], before, after)
	if err != nil {
		stop("compare", err)
	}
	return status
}

func namedCount(files map[string]*extFile) int {
	n := 0
	for _, f := range files {
		for _, fns := range f.named {
			n += len(fns)
		}
	}
	return n
}

// reportExt prints the comparison and returns the exit status.
func reportExt(afterCommit string, before, after map[string]*extFile) (int, error) {
	var onlyBefore, stillNamed, onlyAfter, stoppedRunning, anonymousChanged []string
	sources := map[string]string{}
	// source returns the file at the after commit, or "" when it does not
	// exist there.
	source := func(p string) string {
		if text, ok := sources[p]; ok {
			return text
		}
		data, err := gitShow(afterCommit, "extension/"+p)
		text := ""
		if err == nil {
			text = string(data)
		}
		sources[p] = text
		return text
	}
	paths := map[string]struct{}{}
	for p := range before {
		paths[p] = struct{}{}
	}
	for p := range after {
		paths[p] = struct{}{}
	}
	for _, p := range sortedSet(paths) {
		b, a := before[p], after[p]
		if b != nil && a == nil && source(p) != "" {
			return 0, fmt.Errorf("%s is in the before report and at the after commit but not in the after report: no test loads it any more", p)
		}
		if b == nil {
			b = &extFile{}
		}
		if a == nil {
			a = &extFile{}
		}
		if b.anonymous != a.anonymous {
			anonymousChanged = append(anonymousChanged, fmt.Sprintf("%s: %d -> %d anonymous", p, b.anonymous, a.anonymous))
		}
		names := map[string]struct{}{}
		for n := range b.named {
			names[n] = struct{}{}
		}
		for n := range a.named {
			names[n] = struct{}{}
		}
		for _, n := range sortedSet(names) {
			bf, af := b.named[n], a.named[n]
			switch {
			case len(af) == 0:
				named := identifierIn(source(p), n)
				for _, f := range bf {
					line := fmt.Sprintf("%s %s (line %d, ran %d)", p, n, f.line, f.runs)
					onlyBefore = append(onlyBefore, line)
					if named {
						stillNamed = append(stillNamed, line)
					}
				}
			case len(bf) == 0:
				for _, f := range af {
					onlyAfter = append(onlyAfter, fmt.Sprintf("%s %s (line %d, ran %d)", p, n, f.line, f.runs))
				}
			case len(bf) != len(af):
				return 0, fmt.Errorf("%s %s occurs %d time(s) before and %d after, so its occurrences cannot be matched", p, n, len(bf), len(af))
			default:
				for i := range bf {
					if bf[i].runs > 0 && af[i].runs == 0 {
						stoppedRunning = append(stoppedRunning, fmt.Sprintf("%s %s (line %d -> %d): ran %d -> 0", p, n, bf[i].line, af[i].line, bf[i].runs))
					}
				}
			}
		}
	}
	section("only before (removed)", onlyBefore)
	section("only before, name still in the after source (check each)", stillNamed)
	section("only after (added)", onlyAfter)
	section("ran before, not after (fails)", stoppedRunning)
	section("anonymous count changed (not compared)", anonymousChanged)
	if len(stoppedRunning) > 0 {
		fmt.Printf("NG   %d function(s) matched on both sides ran before and not after\n", len(stoppedRunning))
		return 1, nil
	}
	fmt.Println("OK   no function matched on both sides ran before and not after")
	return 0, nil
}

func sortedSet(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// loadLCOV reads one lcov report and returns its product files by path.
func loadLCOV(report string) (map[string]*extFile, error) {
	data, err := os.ReadFile(report)
	if err != nil {
		return nil, fmt.Errorf("read %s: %v", report, err)
	}
	files := map[string]*extFile{}
	var (
		path   string
		inSF   bool
		fns    []extFunction
		fnName []string
		runs   []int
	)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		switch {
		case line == "":
		case strings.HasPrefix(line, "SF:"):
			if inSF {
				return nil, fmt.Errorf("%s:%d: an SF record inside a section", report, lineNo)
			}
			path, inSF = strings.TrimPrefix(line, "SF:"), true
			if _, dup := files[path]; dup {
				return nil, fmt.Errorf("%s:%d: the section %s repeats", report, lineNo, path)
			}
			fns, fnName, runs = nil, nil, nil
		case strings.HasPrefix(line, "FN:"):
			n, name, ok := numberAndName(strings.TrimPrefix(line, "FN:"))
			if !inSF || !ok {
				return nil, fmt.Errorf("%s:%d: unexpected record %q", report, lineNo, line)
			}
			fns = append(fns, extFunction{line: n})
			fnName = append(fnName, name)
		case strings.HasPrefix(line, "FNDA:"):
			n, name, ok := numberAndName(strings.TrimPrefix(line, "FNDA:"))
			if !inSF || !ok {
				return nil, fmt.Errorf("%s:%d: unexpected record %q", report, lineNo, line)
			}
			if len(runs) >= len(fnName) || fnName[len(runs)] != name {
				return nil, fmt.Errorf("%s:%d: the FNDA record %q has no FN record at its position", report, lineNo, line)
			}
			runs = append(runs, n)
		case line == "end_of_record":
			if !inSF {
				return nil, fmt.Errorf("%s:%d: end_of_record outside a section", report, lineNo)
			}
			if len(runs) != len(fns) {
				return nil, fmt.Errorf("%s:%d: %s has %d FN and %d FNDA records", report, lineNo, path, len(fns), len(runs))
			}
			inSF = false
			if !isExtProductPath(path) {
				continue
			}
			f := &extFile{named: map[string][]extFunction{}}
			for i, fn := range fns {
				fn.runs = runs[i]
				if anonymousName.MatchString(fnName[i]) {
					f.anonymous++
					continue
				}
				f.named[fnName[i]] = append(f.named[fnName[i]], fn)
			}
			for _, list := range f.named {
				slices.SortFunc(list, func(a, b extFunction) int { return a.line - b.line })
			}
			files[path] = f
		default:
			if !slices.ContainsFunc(lcovSkipped, func(p string) bool { return strings.HasPrefix(line, p) }) {
				return nil, fmt.Errorf("%s:%d: unexpected record %q", report, lineNo, line)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if inSF {
		return nil, fmt.Errorf("%s: the last section has no end_of_record", report)
	}
	return files, nil
}

// numberAndName splits "<number>,<name>".
func numberAndName(s string) (int, string, bool) {
	num, name, ok := strings.Cut(s, ",")
	if !ok || name == "" {
		return 0, "", false
	}
	n, err := strconv.Atoi(num)
	if err != nil || n < 0 {
		return 0, "", false
	}
	return n, name, true
}

// isExtProductPath reports whether a path relative to the extension directory
// is product code: a file under src/ or scripts/.
func isExtProductPath(p string) bool {
	return strings.HasPrefix(p, "src/") || strings.HasPrefix(p, "scripts/")
}

// identifierIn reports whether name, or its last dotted part for a method
// such as "record.onWait", appears in text as a whole identifier.
func identifierIn(text, name string) bool {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return regexp.MustCompile(`(^|[^A-Za-z0-9_$])` + regexp.QuoteMeta(name) + `($|[^A-Za-z0-9_$])`).MatchString(text)
}
