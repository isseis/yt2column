//go:build test

// Command _checks runs the one-off checks for task 0068: the strictjson user
// list is corrected in exactly the one cell that holds it, CLAUDE.md and
// project_overview.md change only in the item and the line that pointed at the
// list, ADR-0020 changes only in the regions the plan names, the doc-guard CI
// job runs the reconciliation test, that test holds no copy of the list, and a
// commit that changes ADR-0020 records the editorial correction. It takes the
// base and head commits and reads each file at those commits, never the working
// tree.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0068check ./docs/tasks/0068_strictjson_package_reference/_checks/
//	/tmp/0068check [<base> <head>]
//
// The commits default to the merge-base of origin/main and HEAD, and HEAD. It
// prints both commit IDs and the result of every check. It exits 0 when every
// check passes, 1 on a violation, and 2 when it cannot check its input.
// make test and CI do not run it; the directory starts with an underscore,
// which keeps it out of go list.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const (
	dataPath             = "docs/tasks/0068_strictjson_package_reference/_checks/data.json"
	packageRefPath       = "docs/dev/conventions/package_reference.md"
	claudePath           = "CLAUDE.md"
	projectOverviewPath  = "docs/dev/project_overview.md"
	adrPath              = "docs/dev/adr/0020-strict-json.md"
	ciPath               = ".github/workflows/ci.yml"
	testFilePath         = "internal/pipeline/strictjson_users_test.go"
	reconciliationTest   = "TestPackageReferenceStrictjsonUsers"
	strictjsonPackage    = "internal/strictjson"
	strictjsonImportPath = "github.com/isseis/yt2column/internal/strictjson"
	userListMarker       = "Shared by "
	packageTableHeader   = "Package"
	responsibilityColumn = "Responsibility"
	correctionPrefix     = "Editorial correction: " + adrPath + ": "
	correctionSuffix     = "; no decision changed"
)

// Input test functions hold package names made up to exercise the failure
// paths, which the no-copy check does not count as a copy of the list.
var inputTestFunctions = map[string]struct{}{
	"TestGoSources":         {},
	"TestImportersOf":       {},
	"TestStrictjsonRowCell": {},
	"TestParseUserList":     {},
	"TestDiffUsers":         {},
}

var (
	packagePathRE = regexp.MustCompile(`^(cmd|internal)/[A-Za-z0-9._/-]+$`)
	jobNameRE     = regexp.MustCompile(`^  [A-Za-z0-9_-]+:$`)
	commitRE      = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// data holds every Japanese string the checks compare with the documents.
// The Go source keeps its own strings in English.
type data struct {
	ADR adrData `json:"adr"`
}

// adrData names ADR-0020's sections and the strings that identify the changed
// regions.
type adrData struct {
	DecisionHeading string `json:"decision_heading"`
	ResultsHeading  string `json:"results_heading"`
	HistoryHeading  string `json:"history_heading"`
	DecisionPrefix  string `json:"decision_prefix"`
	SentenceEnd     string `json:"decision_sentence_end"`
	HistoryLink     string `json:"history_link"`
}

func main() {
	ok, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0068check:", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

func run(args []string) (bool, error) {
	baseRev, headRev, err := revisions(args)
	if err != nil {
		return false, err
	}
	base, err := resolve(baseRev)
	if err != nil {
		return false, err
	}
	head, err := resolve(headRev)
	if err != nil {
		return false, err
	}
	fmt.Printf("base %s\nhead %s\n", base, head)

	d, err := loadData(head)
	if err != nil {
		return false, err
	}

	checks := []struct {
		name string
		run  func() ([]string, error)
	}{
		{"package_reference.md scope", func() ([]string, error) { return checkPackageRefScope(base, head) }},
		{"CLAUDE.md and project_overview.md scope", func() ([]string, error) { return checkScopeDocs(base, head) }},
		{"ADR-0020 scope", func() ([]string, error) { return checkADR(base, head, d.ADR) }},
		{"doc-guard runs the reconciliation test", func() ([]string, error) { return checkDocGuard(head) }},
		{"reconciliation test has no copy of the list", func() ([]string, error) { return checkNoCopy(head) }},
		{"ADR-0020 correction line", func() ([]string, error) { return checkCorrectionLine(base, head) }},
	}
	failed := 0
	for _, c := range checks {
		violations, err := c.run()
		if err != nil {
			return false, fmt.Errorf("%s: %w", c.name, err)
		}
		if len(violations) == 0 {
			fmt.Printf("OK   %s\n", c.name)
			continue
		}
		failed++
		for _, text := range violations {
			fmt.Printf("FAIL %s: %s\n", c.name, text)
		}
	}
	if failed > 0 {
		fmt.Printf("FAIL %d check(s)\n", failed)
		return false, nil
	}
	fmt.Println("OK every check passed")
	return true, nil
}

func revisions(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		base, err := git("merge-base", "origin/main", "HEAD")
		return strings.TrimSpace(base), "HEAD", err
	case 2:
		return args[0], args[1], nil
	default:
		return "", "", errors.New("usage: 0068check [<base> <head>]")
	}
}

func resolve(rev string) (string, error) {
	rev = strings.TrimSpace(rev)
	out, err := git("rev-parse", "--verify", rev+"^{commit}")
	return strings.TrimSpace(out), err
}

// loadData reads the check's data at head, like the documents it checks, so a
// working-tree edit cannot change what the committed head is checked against.
func loadData(head string) (data, error) {
	var d data
	raw, err := read(head, dataPath)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return d, err
	}
	return d, nil
}

var (
	// errNoUserList reports that a row has no single user list marker.
	errNoUserList = errors.New("no single user list")
	// errNoRow reports that the document has no row for the component.
	errNoRow = errors.New("no strictjson row")
	// errDuplicateRow reports that the document has more than one row for the
	// component.
	errDuplicateRow = errors.New("duplicate strictjson row")
)

// checkPackageRefScope verifies that package_reference.md differs only inside
// the user list of the internal/strictjson row.
func checkPackageRefScope(base, head string) ([]string, error) {
	baseText, err := read(base, packageRefPath)
	if err != nil {
		return nil, err
	}
	headText, err := read(head, packageRefPath)
	if err != nil {
		return nil, err
	}
	baseSkeleton, err := withoutUserList(baseText)
	if err != nil {
		return nil, err
	}
	headSkeleton, err := withoutUserList(headText)
	if err != nil {
		if errors.Is(err, errNoUserList) || errors.Is(err, errNoRow) || errors.Is(err, errDuplicateRow) {
			return []string{err.Error()}, nil
		}
		return nil, err
	}
	if baseSkeleton != headSkeleton {
		return []string{"the file differs outside the user list of the internal/strictjson row"}, nil
	}
	return nil, nil
}

// withoutUserList returns text with the user list of the internal/strictjson
// row replaced by a marker, leaving everything else as it was.
func withoutUserList(text string) (string, error) {
	lines := strings.Split(text, "\n")
	found := -1
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "| `"+strictjsonPackage+"` |") {
			continue
		}
		if found >= 0 {
			return "", fmt.Errorf("%w: %s has more than one row for %s", errDuplicateRow, packageRefPath, strictjsonPackage)
		}
		found = i
	}
	if found < 0 {
		return "", fmt.Errorf("%w: %s has no row for %s", errNoRow, packageRefPath, strictjsonPackage)
	}
	line := lines[found]
	if n := strings.Count(line, userListMarker); n != 1 {
		return "", fmt.Errorf("%w: the row for %s has %d %q markers", errNoUserList, strictjsonPackage, n, userListMarker)
	}
	if !strings.HasSuffix(line, " |") {
		return "", fmt.Errorf("%s: the row for %s does not end a table cell", packageRefPath, strictjsonPackage)
	}
	split := strings.Index(line, userListMarker)
	lines[found] = line[:split] + "<user list>" + line[len(line)-2:]
	return strings.Join(lines, "\n"), nil
}

// checkScopeDocs verifies that CLAUDE.md and project_overview.md differ only
// inside the item and the line that described the user list.
func checkScopeDocs(base, head string) ([]string, error) {
	var out []string
	claudeViolation, err := sameOutside(base, head, claudePath, claudeItem)
	if err != nil {
		return nil, err
	}
	out = append(out, claudeViolation...)
	overviewViolation, err := sameOutside(base, head, projectOverviewPath, overviewLine)
	if err != nil {
		return nil, err
	}
	out = append(out, overviewViolation...)
	return out, nil
}

// sameOutside verifies that the file at base and head is equal after region
// replaces the changed block with a marker in each.
func sameOutside(base, head, filePath string, region func([]string) (int, int, error)) ([]string, error) {
	baseText, err := read(base, filePath)
	if err != nil {
		return nil, err
	}
	headText, err := read(head, filePath)
	if err != nil {
		return nil, err
	}
	baseSkeleton, err := replaceRegion(baseText, region)
	if err != nil {
		return nil, err
	}
	headSkeleton, err := replaceRegion(headText, region)
	if err != nil {
		return nil, err
	}
	if baseSkeleton != headSkeleton {
		return []string{fmt.Sprintf("%s differs outside the internal/strictjson item", filePath)}, nil
	}
	return nil, nil
}

// replaceRegion replaces the lines region returns with a marker.
func replaceRegion(text string, region func([]string) (int, int, error)) (string, error) {
	lines := strings.Split(text, "\n")
	start, end, err := region(lines)
	if err != nil {
		return "", err
	}
	lines = slices.Concat(lines[:start], []string{"<strictjson item>"}, lines[end:])
	return strings.Join(lines, "\n"), nil
}

// claudeItem returns the line range of CLAUDE.md's strictjson bullet.
func claudeItem(lines []string) (int, int, error) {
	const first = "- `internal/strictjson`:"
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, first) {
			if start >= 0 {
				return 0, 0, fmt.Errorf("%s has more than one %q item", claudePath, first)
			}
			start = i
		}
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("%s has no %q item", claudePath, first)
	}
	end := start + 1
	for end < len(lines) && lines[end] != "" && !strings.HasPrefix(lines[end], "- ") {
		end++
	}
	return start, end, nil
}

// overviewLine returns the line range of project_overview.md's strictjson
// directory line.
func overviewLine(lines []string) (int, int, error) {
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "internal/strictjson/") {
			if start >= 0 {
				return 0, 0, fmt.Errorf("%s has more than one internal/strictjson/ line", projectOverviewPath)
			}
			start = i
		}
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("%s has no internal/strictjson/ line", projectOverviewPath)
	}
	return start, start + 1, nil
}

// adrDoc is one version of ADR-0020 split into the regions the change may
// touch.
type adrDoc struct {
	lines        []string
	decisionItem string
	def          bool
	decisionLine int
	results      adrSection
	history      adrSection
}

// adrSection is a section body's line range, the heading line excluded.
type adrSection struct {
	start, end int
}

func parseADR(text string, d adrData) (adrDoc, error) {
	doc := adrDoc{lines: strings.Split(text, "\n")}
	var err error
	doc.results.start, doc.results.end, err = sectionRange(doc.lines, d.ResultsHeading)
	if err != nil {
		return adrDoc{}, err
	}
	doc.history.start, doc.history.end, err = sectionRange(doc.lines, d.HistoryHeading)
	if err != nil {
		return adrDoc{}, err
	}
	decStart, decEnd, err := sectionRange(doc.lines, d.DecisionHeading)
	if err != nil {
		return adrDoc{}, err
	}
	for i := decStart; i < decEnd; i++ {
		if !strings.HasPrefix(strings.TrimSpace(doc.lines[i]), "-   **"+d.DecisionPrefix) {
			continue
		}
		if doc.def {
			return adrDoc{}, fmt.Errorf("%s has more than one decision item that starts with %q", adrPath, d.DecisionPrefix)
		}
		doc.def = true
		doc.decisionLine = i
		doc.decisionItem = doc.lines[i]
	}
	if !doc.def {
		return adrDoc{}, fmt.Errorf("%s has no decision item that starts with %q", adrPath, d.DecisionPrefix)
	}
	return doc, nil
}

// sectionRange returns the body line range of the section under heading: the
// line after the heading to the next level-2 heading.
func sectionRange(lines []string, heading string) (int, int, error) {
	doc := strings.Join(lines, "\n")
	section := doctestutil.SectionByHeading(doc, heading)
	if section == "" {
		return 0, 0, fmt.Errorf("heading %q not found", heading)
	}
	headings := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == heading {
			headings++
		}
	}
	if headings != 1 {
		return 0, 0, fmt.Errorf("heading %q appears %d times", heading, headings)
	}
	offset := strings.Index(doc, section)
	if offset < 0 {
		return 0, 0, fmt.Errorf("heading %q is not part of the document", heading)
	}
	headingLine := strings.Count(doc[:offset], "\n")
	body := strings.TrimPrefix(section, lines[headingLine]+"\n")
	bodyLines := strings.Count(body, "\n")
	if body != "" && !strings.HasSuffix(body, "\n") {
		bodyLines++
	}
	return headingLine + 1, headingLine + 1 + bodyLines, nil
}

// checkADR verifies that ADR-0020 changes only in the three planned regions.
func checkADR(base, head string, d adrData) ([]string, error) {
	baseText, err := read(base, adrPath)
	if err != nil {
		return nil, err
	}
	headText, err := read(head, adrPath)
	if err != nil {
		return nil, err
	}
	baseDoc, err := parseADR(baseText, d)
	if err != nil {
		return nil, err
	}
	headDoc, err := parseADR(headText, d)
	if err != nil {
		return nil, err
	}

	var out []string
	first, err := firstSentence(baseDoc.decisionItem, d.SentenceEnd)
	if err != nil {
		return nil, err
	}
	if headDoc.decisionItem != first {
		out = append(out, "the decision item is not the first sentence of the base commit's item")
	}
	if item, ok := oneAppendedItem(baseDoc.resultsLines(), headDoc.resultsLines()); !ok {
		out = append(out, "the results section is not the base commit's items with exactly one item appended")
	} else if !strings.HasPrefix(item, "- ") {
		out = append(out, "the appended results line is not a list item")
	}
	if item, ok := oneAppendedItem(baseDoc.historyLines(), headDoc.historyLines()); !ok {
		out = append(out, "the history section is not the base commit's items with exactly one item appended")
	} else if !strings.Contains(item, d.HistoryLink) {
		out = append(out, "the appended history item has no link to this task's documents")
	}
	if !slices.Equal(baseDoc.skeleton(), headDoc.skeleton()) {
		out = append(out, "lines outside the changed regions differ from the base commit")
	}
	return out, nil
}

func (a adrDoc) resultsLines() []string { return a.lines[a.results.start:a.results.end] }
func (a adrDoc) historyLines() []string { return a.lines[a.history.start:a.history.end] }

// skeleton returns the document with each changed region replaced by one
// marker line.
func (a adrDoc) skeleton() []string {
	var out []string
	for i, line := range a.lines {
		switch {
		case i == a.decisionLine:
			out = append(out, "<decision item>")
		case i == a.results.start:
			out = append(out, "<results>")
		case i >= a.results.start && i < a.results.end:
		case i == a.history.start:
			out = append(out, "<history>")
		case i >= a.history.start && i < a.history.end:
		default:
			out = append(out, line)
		}
	}
	return out
}

// firstSentence returns item up to the end of its bold first sentence, which
// ends with marker.
func firstSentence(item, marker string) (string, error) {
	if marker == "" {
		return "", errors.New("the decision sentence end marker is empty")
	}
	i := strings.Index(item, marker)
	if i < 0 {
		return "", fmt.Errorf("the decision item has no bold first sentence: %q", item)
	}
	return item[:i+len(marker)], nil
}

// oneAppendedItem reports whether head is base with exactly one item appended,
// keeping the trailing blank lines the same.
func oneAppendedItem(base, head []string) (string, bool) {
	b := trimTrailingBlank(base)
	h := trimTrailingBlank(head)
	if len(base)-len(b) != len(head)-len(h) {
		return "", false
	}
	if len(h) != len(b)+1 || !slices.Equal(h[:len(b)], b) {
		return "", false
	}
	return h[len(b)], true
}

// trimTrailingBlank returns lines without its trailing blank lines.
func trimTrailingBlank(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

// checkDocGuard verifies that the doc-guard job runs the package holding the
// reconciliation test, with the test build tag.
func checkDocGuard(head string) ([]string, error) {
	text, err := read(head, ciPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(text, "\n")
	jobStart := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "doc-guard:" {
			jobStart = i
			break
		}
	}
	if jobStart < 0 {
		return nil, fmt.Errorf("%s has no doc-guard job", ciPath)
	}
	jobEnd := len(lines)
	for i := jobStart + 1; i < len(lines); i++ {
		if jobNameRE.MatchString(lines[i]) {
			jobEnd = i
			break
		}
	}
	runLine := ""
	for _, line := range lines[jobStart:jobEnd] {
		if strings.Contains(line, "go test ") {
			runLine = line
			break
		}
	}
	if runLine == "" {
		return nil, fmt.Errorf("%s: the doc-guard job has no go test line", ciPath)
	}

	var out []string
	found, err := definingFile(head, path.Dir(testFilePath), reconciliationTest)
	if err != nil {
		return nil, err
	}
	wantPackage := "./" + path.Dir(found)
	if !slices.Contains(strings.Fields(runLine), wantPackage) {
		out = append(out, fmt.Sprintf("the doc-guard go test line does not name %s", wantPackage))
	}
	if !strings.Contains(runLine, "-tags test") {
		out = append(out, "the doc-guard go test line does not pass -tags test")
	}
	source, err := read(head, found)
	if err != nil {
		return nil, err
	}
	if err := buildTagHolds(found, source); err != nil {
		out = append(out, err.Error())
	}
	return out, nil
}

// definingFile returns the path of the Go file at head that defines funcName
// in dir.
func definingFile(head, dir, funcName string) (string, error) {
	paths, err := gitPaths("ls-tree", "-r", "-z", "--name-only", head, "--", dir)
	if err != nil {
		return "", err
	}
	for _, filePath := range paths {
		if !strings.HasSuffix(filePath, ".go") {
			continue
		}
		source, err := read(head, filePath)
		if err != nil {
			return "", err
		}
		file, err := parser.ParseFile(token.NewFileSet(), filePath, source, 0)
		if err != nil {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name != nil && fn.Name.Name == funcName {
				return filePath, nil
			}
		}
	}
	return "", fmt.Errorf("head %s has no file under %s defining %s", head, dir, funcName)
}

// buildTagHolds reports whether every build constraint before the package
// clause lets a test build compile the file.
func buildTagHolds(filePath, source string) error {
	type wireConstraint struct {
		line string
		expr constraint.Expr
	}
	var constraints []wireConstraint
	for line := range strings.Lines(source) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			break
		}
		if !strings.HasPrefix(trimmed, "//go:build") && !strings.HasPrefix(trimmed, "// +build") {
			continue
		}
		expr, err := constraint.Parse(trimmed)
		if err != nil {
			return fmt.Errorf("%s: %w", filePath, err)
		}
		constraints = append(constraints, wireConstraint{line: trimmed, expr: expr})
	}
	for _, c := range constraints {
		if !c.expr.Eval(func(tag string) bool { return tag == "test" }) {
			return fmt.Errorf("the build constraint %q in %s does not hold with -tags test", c.line, filePath)
		}
	}
	return nil
}

// checkNoCopy verifies that the reconciliation test and the file's top-level
// values hold no package name from the list except the component's own.
func checkNoCopy(head string) ([]string, error) {
	doc, err := read(head, packageRefPath)
	if err != nil {
		return nil, err
	}
	cell, err := userCell(doc)
	if err != nil {
		return nil, err
	}
	listed, err := listNames(cell)
	if err != nil {
		return nil, err
	}

	source, err := read(head, testFilePath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, testFilePath, source, 0)
	if err != nil {
		return nil, err
	}
	var skip [][2]token.Pos
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Body == nil {
			return true
		}
		if fn.Name.Name == reconciliationTest {
			found = true
		}
		if _, ok := inputTestFunctions[fn.Name.Name]; ok {
			skip = append(skip, [2]token.Pos{fn.Body.Pos(), fn.Body.End()})
		}
		return true
	})
	if !found {
		return nil, fmt.Errorf("%s has no %s function", testFilePath, reconciliationTest)
	}

	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		for _, span := range skip {
			if lit.Pos() >= span[0] && lit.Pos() < span[1] {
				return true
			}
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if value == strictjsonPackage || value == strictjsonImportPath {
			return true
		}
		if slices.Contains(listed, value) || packagePathRE.MatchString(value) {
			out = append(out, fmt.Sprintf("%s:%d: %q looks like a copy of the user list",
				testFilePath, fset.Position(lit.Pos()).Line, value))
		}
		return true
	})
	return out, nil
}

// userCell returns the responsibility cell of package_reference.md's
// internal/strictjson row.
func userCell(doc string) (string, error) {
	table := doctestutil.FindTable(doc, packageTableHeader)
	if table == nil {
		return "", fmt.Errorf("%s has no table with a %q header", packageRefPath, packageTableHeader)
	}
	col := doctestutil.ColumnIndex(table[0], responsibilityColumn)
	if col < 0 {
		return "", fmt.Errorf("%s has no %q column", packageRefPath, responsibilityColumn)
	}
	var rows [][]string
	for _, row := range table[1:] {
		if len(row) > 0 && doctestutil.StripBackticks(row[0]) == strictjsonPackage {
			rows = append(rows, row)
		}
	}
	if len(rows) != 1 {
		return "", fmt.Errorf("%s has %d rows for %s", packageRefPath, len(rows), strictjsonPackage)
	}
	if col >= len(rows[0]) {
		return "", fmt.Errorf("%s: the row for %s has no %q cell", packageRefPath, strictjsonPackage, responsibilityColumn)
	}
	return rows[0][col], nil
}

// listNames returns the code-span names after the user list marker.
func listNames(cell string) ([]string, error) {
	if n := strings.Count(cell, userListMarker); n != 1 {
		return nil, fmt.Errorf("%s: the cell has %d user list markers", packageRefPath, n)
	}
	rest := cell[strings.Index(cell, userListMarker)+len(userListMarker):]
	var names []string
	for {
		start := strings.IndexByte(rest, '`')
		if start < 0 {
			break
		}
		end := strings.IndexByte(rest[start+1:], '`')
		if end < 0 {
			return nil, fmt.Errorf("%s: unclosed code span in %q", packageRefPath, rest)
		}
		names = append(names, rest[start+1:start+1+end])
		rest = rest[start+1+end+1:]
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s: no code span after %q", packageRefPath, userListMarker)
	}
	return names, nil
}

// checkCorrectionLine verifies that a commit that changes ADR-0020 records the
// editorial correction.
func checkCorrectionLine(base, head string) ([]string, error) {
	out, err := git("log", "--no-show-signature", "--no-merges", "--format=%H", base+".."+head)
	if err != nil {
		return nil, err
	}
	checked := 0
	for commit := range strings.Lines(out) {
		commit = strings.TrimSpace(commit)
		if !commitRE.MatchString(commit) {
			continue
		}
		files, err := gitPaths("diff-tree", "--no-commit-id", "--name-only", "-r", "-z", "--no-renames", "--no-ext-diff", commit)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(files, adrPath) {
			continue
		}
		checked++
		message, err := git("log", "--no-show-signature", "-1", "--format=%B", commit)
		if err != nil {
			return nil, err
		}
		if hasCorrectionLine(message) {
			return nil, nil
		}
	}
	if checked == 0 {
		return []string{"no non-merge commit changes " + adrPath}, nil
	}
	return []string{"no commit that changes " + adrPath + " has a correction line"}, nil
}

// hasCorrectionLine reports whether message has a correction line for ADR-0020
// with neither "@" nor "<".
func hasCorrectionLine(message string) bool {
	for line := range strings.Lines(message) {
		line = strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(line, correctionPrefix) &&
			strings.HasSuffix(line, correctionSuffix) &&
			!strings.ContainsAny(line, "@<") {
			return true
		}
	}
	return false
}

func read(commit, filePath string) (string, error) {
	return git("show", commit+":"+filePath)
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		msg := ""
		if errors.As(err, &exitErr) {
			msg = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return string(out), nil
}

func gitPaths(args ...string) ([]string, error) {
	out, err := git(args...)
	if err != nil {
		return nil, err
	}
	paths := strings.Split(out, "\x00")
	return slices.DeleteFunc(paths, func(s string) bool { return s == "" }), nil
}
