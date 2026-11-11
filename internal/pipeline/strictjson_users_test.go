//go:build test

package pipeline

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// goSource is one Go source file found by the walk, with the package (the
// slash-separated directory relative to the walk root) it belongs to.
type goSource struct {
	pkg  string
	path string
}

// goSources returns every Go source file under root/cmd and root/internal,
// with its slash-separated path relative to root. It returns a file whose name
// ends in _test.go and a file whose build constraints exclude it from the
// current build, because the caller decides what to read. It skips a directory
// named testdata, a directory or file whose name starts with "_" or ".",
// everything below such a directory, and a file outside cmd/ and internal/.
// The name rules apply only to the entries below cmd/ and internal/, not to
// root, so a root whose own path starts with "." or "_" is still walked.
func goSources(root string) ([]goSource, error) {
	var sources []goSource
	for _, base := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipWalkName(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if skipWalkName(d.Name()) || !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			sources = append(sources, goSource{pkg: filepath.ToSlash(filepath.Dir(rel)), path: rel})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return sources, nil
}

// skipWalkName reports whether the walk skips an entry with this name: the
// testdata directory and every name that starts with "_" or ".".
func skipWalkName(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".")
}

// TestGoSources checks which files the walk returns and which it skips.
func TestGoSources(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_root")
	writeSourceFiles(t, root, map[string]string{
		"cmd/tool/main.go":            "package main",
		"cmd/tool/main_windows.go":    "package main",
		"cmd/tool/tagged.go":          "//go:build test\n\npackage main",
		"cmd/tool/ignored.go":         "//go:build ignore\n\npackage main",
		"cmd/tool/x_test.go":          "package main",
		"cmd/tool/_skip.go":           "package main",
		"cmd/tool/.skip.go":           "package main",
		"cmd/tool/testdata/sample.go": "package sample",
		"cmd/tool/_hidden/sample.go":  "package sample",
		"cmd/tool/.hidden/sample.go":  "package sample",
		"top.go":                      "package top",
		"other/lib.go":                "package other",
		"internal/lib/lib.go":         "package lib",
	})

	got, err := goSources(root)
	if err != nil {
		t.Fatalf("goSources returned error: %v", err)
	}
	want := []goSource{
		{pkg: "cmd/tool", path: "cmd/tool/ignored.go"},
		{pkg: "cmd/tool", path: "cmd/tool/main.go"},
		{pkg: "cmd/tool", path: "cmd/tool/main_windows.go"},
		{pkg: "cmd/tool", path: "cmd/tool/tagged.go"},
		{pkg: "cmd/tool", path: "cmd/tool/x_test.go"},
		{pkg: "internal/lib", path: "internal/lib/lib.go"},
	}
	// The walk's order is not part of its contract, so compare the set.
	sortSources := func(sources []goSource) {
		slices.SortFunc(sources, func(a, b goSource) int { return strings.Compare(a.path, b.path) })
	}
	sortSources(got)
	sortSources(want)
	if !slices.Equal(got, want) {
		t.Errorf("goSources returned %v, want %v", got, want)
	}
}

// writeSourceFiles writes each path (slash-separated, relative to root) with
// its content, creating the parent directories.
func writeSourceFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create the directory of %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// importersOf returns, for each package with a non-test Go source that imports
// importPath, the sorted slash-separated paths of the files that import it. An
// alias, blank, or dot import counts. Test files are skipped, and a file it
// cannot read or parse is an error.
func importersOf(root string, sources []goSource, importPath string) (map[string][]string, error) {
	users := map[string][]string{}
	fset := token.NewFileSet()
	for _, src := range sources {
		if strings.HasSuffix(src.path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(src.path)), nil, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", src.path, err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("import path in %s: %w", src.path, err)
			}
			if path == importPath {
				users[src.pkg] = append(users[src.pkg], src.path)
				break
			}
		}
	}
	for pkg := range users {
		slices.Sort(users[pkg])
	}
	return users, nil
}

const (
	packageTableHeader    = "Package"
	responsibilityColumn  = "Responsibility"
	strictjsonPackageName = "internal/strictjson"
	strictjsonImportPath  = "github.com/isseis/yt2column/internal/strictjson"
	userListMarker        = "Shared by "
)

var (
	// errNoPackageTable reports that the document has no table whose header
	// names the package column.
	errNoPackageTable = errors.New("no package table")
	// errNoResponsibilityColumn reports that the table has no responsibility
	// column, or that the row has no cell for it.
	errNoResponsibilityColumn = errors.New("no responsibility column")
	// errNoStrictjsonRow reports that no table row names the component.
	errNoStrictjsonRow = errors.New("no strictjson row")
	// errDuplicateStrictjsonRow reports that more than one table row names the
	// component.
	errDuplicateStrictjsonRow = errors.New("duplicate strictjson row")
	// errNoUserList reports that the cell has no "Shared by " list, or has it
	// more than once.
	errNoUserList = errors.New("no single user list")
	// errMalformedUserList reports that the list does not have the accepted
	// form.
	errMalformedUserList = errors.New("malformed user list")
)

// strictjsonRowCell returns the responsibility cell of the single row whose
// package cell names the component, searched in every table whose header names
// the package column. It fails when no such table exists, when the row is
// missing, when the row's table has no responsibility column, or when the row
// appears more than once (in one table or in different tables). Every error
// names the table header, the column, the component, and the number of
// matching rows.
func strictjsonRowCell(doc string) (string, error) {
	const context = "table header %q, column %q, package %q, matched rows %d"
	tables := 0
	matched := 0
	var row []string
	col := -1
	for _, table := range doctestutil.MarkdownTables(doc) {
		if len(table) == 0 {
			continue
		}
		pkgCol := doctestutil.ColumnIndex(table[0], packageTableHeader)
		if pkgCol < 0 {
			continue
		}
		tables++
		tableCol := doctestutil.ColumnIndex(table[0], responsibilityColumn)
		for _, candidate := range table[1:] {
			if pkgCol >= len(candidate) || doctestutil.StripBackticks(candidate[pkgCol]) != strictjsonPackageName {
				continue
			}
			matched++
			row = candidate
			col = tableCol
		}
	}
	if tables == 0 {
		return "", fmt.Errorf("%w: "+context, errNoPackageTable, packageTableHeader, responsibilityColumn, strictjsonPackageName, 0)
	}
	switch matched {
	case 0:
		return "", fmt.Errorf("%w: "+context, errNoStrictjsonRow, packageTableHeader, responsibilityColumn, strictjsonPackageName, 0)
	case 1:
	default:
		return "", fmt.Errorf("%w: "+context, errDuplicateStrictjsonRow, packageTableHeader, responsibilityColumn, strictjsonPackageName, matched)
	}
	if col < 0 {
		return "", fmt.Errorf("%w: "+context, errNoResponsibilityColumn, packageTableHeader, responsibilityColumn, strictjsonPackageName, 1)
	}
	if col >= len(row) {
		return "", fmt.Errorf("%w: "+context+", the row has %d cells", errNoResponsibilityColumn, packageTableHeader, responsibilityColumn, strictjsonPackageName, 1, len(row))
	}
	return row[col], nil
}

// parseUserList returns the code-span names of the cell's "Shared by " list, in
// the order written and with duplicates kept. It rejects a missing or repeated
// marker and every form other than one item, two items joined by " and ", or
// three or more items whose last two are joined by ", and ".
func parseUserList(cell string) ([]string, error) {
	if n := strings.Count(cell, userListMarker); n != 1 {
		return nil, fmt.Errorf("%w: %q appears %d times", errNoUserList, userListMarker, n)
	}
	rest := cell[strings.Index(cell, userListMarker)+len(userListMarker):]
	name, rest, err := cutListName(rest)
	if err != nil {
		return nil, err
	}
	names := []string{name}
	if rest == "" {
		return names, nil
	}
	if after, ok := strings.CutPrefix(rest, " and "); ok {
		last, rest, err := cutListName(after)
		if err != nil {
			return nil, err
		}
		if rest != "" {
			return nil, malformedUserList("text after the last item: %q", rest)
		}
		return append(names, last), nil
	}
	for {
		after, ok := strings.CutPrefix(rest, ", ")
		if !ok {
			return nil, malformedUserList("separator %q is neither \", \" nor \" and \"", rest)
		}
		if final, isAnd := strings.CutPrefix(after, "and "); isAnd {
			if len(names) < 2 {
				return nil, malformedUserList("two items are joined by \", and \"")
			}
			last, rest, err := cutListName(final)
			if err != nil {
				return nil, err
			}
			if rest != "" {
				return nil, malformedUserList("text after the last item: %q", rest)
			}
			return append(names, last), nil
		}
		name, tail, err := cutListName(after)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
		if tail == "" {
			return nil, malformedUserList("the last item is not preceded by \", and \"")
		}
		rest = tail
	}
}

// cutListName removes the leading code span of s and returns its name and the
// rest.
func cutListName(s string) (name, rest string, err error) {
	if !strings.HasPrefix(s, "`") {
		return "", "", malformedUserList("item %q is not a code span", s)
	}
	end := strings.IndexByte(s[1:], '`')
	if end < 0 {
		return "", "", malformedUserList("item %q has no closing backquote", s)
	}
	name = s[1 : end+1]
	if name == "" {
		return "", "", malformedUserList("an item is empty")
	}
	return name, s[end+2:], nil
}

// malformedUserList returns an error that wraps errMalformedUserList and
// describes the part of the list that does not fit the form.
func malformedUserList(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errMalformedUserList, fmt.Sprintf(format, args...))
}

// diffUsers compares the listed names with the packages the code uses and
// returns the users no name lists, the names no user justifies, and the names
// listed more than once, each sorted.
func diffUsers(listed []string, users map[string][]string) (missing, extra, duplicated []string) {
	counts := map[string]int{}
	for _, name := range listed {
		counts[name]++
	}
	for name, n := range counts {
		if n > 1 {
			duplicated = append(duplicated, name)
		}
	}
	for pkg := range users {
		if counts[pkg] == 0 {
			missing = append(missing, pkg)
		}
	}
	for name := range counts {
		if _, ok := users[name]; !ok {
			extra = append(extra, name)
		}
	}
	slices.Sort(missing)
	slices.Sort(extra)
	slices.Sort(duplicated)
	return missing, extra, duplicated
}

// strictjsonUsersReport returns one failure line for each difference.
func strictjsonUsersReport(missing, extra, duplicated []string, users map[string][]string) []string {
	var lines []string
	for _, pkg := range missing {
		lines = append(lines, fmt.Sprintf("package_reference.md does not list %s, which imports %s (files: %s)",
			pkg, strictjsonImportPath, strings.Join(users[pkg], ", ")))
	}
	for _, name := range extra {
		lines = append(lines, fmt.Sprintf("package_reference.md lists %s, which no non-test source under cmd/ or internal/ imports %s",
			name, strictjsonImportPath))
	}
	for _, name := range duplicated {
		lines = append(lines, fmt.Sprintf("package_reference.md lists %s more than once", name))
	}
	return lines
}

// TestImportersOf checks which packages the import read finds.
func TestImportersOf(t *testing.T) {
	const target = "example.com/target"
	root := t.TempDir()
	writeSourceFiles(t, root, map[string]string{
		"internal/plain/a.go":             "package plain\n\nimport \"example.com/target\"\n",
		"internal/plain/z.go":             "package plain\n\nimport \"example.com/target\"\n",
		"internal/plain/testonly_test.go": "package plain\n\nimport \"example.com/target\"\n",
		"internal/tagged/helper.go":       "//go:build test\n\npackage tagged\n\nimport \"example.com/target\"\n",
		"internal/alias/a.go":             "package alias\n\nimport sj \"example.com/target\"\n",
		"internal/blank/a.go":             "package blank\n\nimport _ \"example.com/target\"\n",
		"internal/dot/a.go":               "package dot\n\nimport . \"example.com/target\"\n",
		"internal/other/a.go":             "package other\n\nimport \"example.com/other\"\n",
		"internal/testonly/only_test.go":  "package testonly\n\nimport \"example.com/target\"\n",
		"cmd/.keep":                       "",
	})
	sources, err := goSources(root)
	if err != nil {
		t.Fatalf("goSources returned error: %v", err)
	}
	got, err := importersOf(root, sources, target)
	if err != nil {
		t.Fatalf("importersOf returned error: %v", err)
	}
	want := map[string][]string{
		"internal/alias":  {"internal/alias/a.go"},
		"internal/blank":  {"internal/blank/a.go"},
		"internal/dot":    {"internal/dot/a.go"},
		"internal/plain":  {"internal/plain/a.go", "internal/plain/z.go"},
		"internal/tagged": {"internal/tagged/helper.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("importersOf returned %v, want %v", got, want)
	}

	// The files of a package are sorted even when the sources come in another
	// order.
	reversed := slices.Clone(sources)
	slices.Reverse(reversed)
	got, err = importersOf(root, reversed, target)
	if err != nil {
		t.Fatalf("importersOf on reversed sources returned error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("importersOf on reversed sources returned %v, want %v", got, want)
	}

	broken := t.TempDir()
	writeSourceFiles(t, broken, map[string]string{
		"internal/broken/bad.go": "package broken\n\nimport \"example.com/target\n",
		"cmd/.keep":              "",
	})
	sources, err = goSources(broken)
	if err != nil {
		t.Fatalf("goSources returned error: %v", err)
	}
	if _, err := importersOf(broken, sources, target); err == nil {
		t.Error("importersOf accepted a file it cannot parse")
	}
}

// TestStrictjsonRowCell checks the row lookup's success and failure paths. The
// failure rows also check the message: the accepted plan requires each error to
// name the table header, the column, the component, and the number of matching
// rows, so that wording is part of the contract.
func TestStrictjsonRowCell(t *testing.T) {
	const (
		header  = "| Package | Responsibility |\n|---|---|\n"
		context = packageTableHeader
		column  = responsibilityColumn
		pkg     = strictjsonPackageName
	)
	cases := []struct {
		name        string
		doc         string
		want        string
		wantErr     error
		wantMessage []string
	}{
		{
			name: "one row",
			doc:  header + "| `internal/strictjson` | the responsibility |\n| `internal/other` | other |\n",
			want: "the responsibility",
		},
		{
			name: "row in the second table",
			doc: header + "| `internal/other` | other |\n\n" +
				header + "| `internal/strictjson` | the second table's responsibility |\n",
			want: "the second table's responsibility",
		},
		{
			name: "one row in each of two tables",
			doc: header + "| `internal/strictjson` | first |\n\n" +
				header + "| `internal/strictjson` | second |\n",
			wantErr:     errDuplicateStrictjsonRow,
			wantMessage: []string{context, column, pkg, "matched rows 2"},
		},
		{
			name:        "no table",
			doc:         "# Packages\n\nNo table.\n",
			wantErr:     errNoPackageTable,
			wantMessage: []string{context, column, pkg, "matched rows 0"},
		},
		{
			name:        "table without a package column",
			doc:         "| Other | Thing |\n|---|---|\n| `internal/strictjson` | x |\n",
			wantErr:     errNoPackageTable,
			wantMessage: []string{context, column, pkg, "matched rows 0"},
		},
		{
			name: "package column is not first",
			doc: "| Responsibility | Package |\n|---|---|\n" +
				"| the responsibility | `internal/strictjson` |\n",
			want: "the responsibility",
		},
		{
			name:        "no responsibility column",
			doc:         "| Package | Other |\n|---|---|\n| `internal/strictjson` | x |\n",
			wantErr:     errNoResponsibilityColumn,
			wantMessage: []string{context, column, pkg, "matched rows 1"},
		},
		{
			name: "row is in a table without a responsibility column",
			doc: header + "| `internal/other` | other |\n\n" +
				"| Package | Notes |\n|---|---|\n| `internal/strictjson` | x |\n",
			wantErr:     errNoResponsibilityColumn,
			wantMessage: []string{context, column, pkg, "matched rows 1"},
		},
		{
			name:        "no row",
			doc:         header + "| `internal/other` | other |\n",
			wantErr:     errNoStrictjsonRow,
			wantMessage: []string{context, column, pkg, "matched rows 0"},
		},
		{
			name:        "duplicate row",
			doc:         header + "| `internal/strictjson` | first |\n| `internal/strictjson` | second |\n",
			wantErr:     errDuplicateStrictjsonRow,
			wantMessage: []string{context, column, pkg, "matched rows 2"},
		},
		{
			name:        "row without the cell",
			doc:         header + "| `internal/strictjson` |\n",
			wantErr:     errNoResponsibilityColumn,
			wantMessage: []string{context, column, pkg, "matched rows 1", "the row has 1 cells"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := strictjsonRowCell(tc.doc)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("strictjsonRowCell returned error %v, want %v", err, tc.wantErr)
				}
				for _, want := range tc.wantMessage {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("strictjsonRowCell returned error: %v", err)
			}
			if got != tc.want {
				t.Errorf("strictjsonRowCell returned %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParseUserList checks the accepted list forms and the rejected ones.
func TestParseUserList(t *testing.T) {
	cases := []struct {
		name    string
		cell    string
		want    []string
		wantErr error
	}{
		{name: "one item", cell: "Shared by `a`", want: []string{"a"}},
		{name: "two items", cell: "Shared by `a` and `b`", want: []string{"a", "b"}},
		{name: "three items", cell: "Shared by `a`, `b`, and `c`", want: []string{"a", "b", "c"}},
		{name: "after contract text", cell: "Extracts values. Shared by `a`, `b`, and `c`", want: []string{"a", "b", "c"}},
		{name: "duplicate kept", cell: "Shared by `a`, `b`, and `a`", want: []string{"a", "b", "a"}},
		{name: "no list", cell: "Extracts values.", wantErr: errNoUserList},
		{name: "two markers", cell: "Shared by `a` and Shared by `b`", wantErr: errNoUserList},
		{name: "not a code span", cell: "Shared by a", wantErr: errMalformedUserList},
		{name: "comma without and for three", cell: "Shared by `a`, `b`, `c`", wantErr: errMalformedUserList},
		{name: "comma and for two", cell: "Shared by `a`, and `b`", wantErr: errMalformedUserList},
		{name: "trailing text", cell: "Shared by `a`.", wantErr: errMalformedUserList},
		{name: "unclosed code span", cell: "Shared by `a", wantErr: errMalformedUserList},
		{name: "no final item", cell: "Shared by `a` and ", wantErr: errMalformedUserList},
		{name: "empty item", cell: "Shared by ``", wantErr: errMalformedUserList},
		{name: "text after two items", cell: "Shared by `a` and `b`.", wantErr: errMalformedUserList},
		{name: "text after three items", cell: "Shared by `a`, `b`, and `c`.", wantErr: errMalformedUserList},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseUserList(tc.cell)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseUserList returned error %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseUserList returned error: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("parseUserList returned %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDiffUsers checks the three differences and the report they produce.
func TestDiffUsers(t *testing.T) {
	cases := []struct {
		name       string
		listed     []string
		users      map[string][]string
		wantMiss   []string
		wantExtra  []string
		wantDup    []string
		wantReport []string
	}{
		{
			name: "user not listed",
			users: map[string][]string{
				"internal/foo": {"internal/foo/a.go"},
				"internal/bar": {"internal/bar/b.go", "internal/bar/z.go"},
			},
			wantMiss:   []string{"internal/bar", "internal/foo"},
			wantReport: []string{"does not list internal/bar,", "does not list internal/foo,", "internal/foo/a.go", "internal/bar/b.go", "internal/bar/z.go"},
		},
		{
			name:       "names that are not users",
			listed:     []string{"internal/missing", "internal/strictjson"},
			users:      map[string][]string{},
			wantExtra:  []string{"internal/missing", "internal/strictjson"},
			wantReport: []string{"lists internal/missing,", "lists internal/strictjson,"},
		},
		{
			name:   "name listed twice",
			listed: []string{"internal/foo", "internal/foo", "internal/bar", "internal/bar"},
			users: map[string][]string{
				"internal/foo": {"internal/foo/a.go"},
				"internal/bar": {"internal/bar/b.go"},
			},
			wantDup:    []string{"internal/bar", "internal/foo"},
			wantReport: []string{"lists internal/bar more than once", "lists internal/foo more than once"},
		},
		{
			name:   "no differences",
			listed: []string{"internal/foo"},
			users:  map[string][]string{"internal/foo": {"internal/foo/a.go"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			miss, extra, dup := diffUsers(tc.listed, tc.users)
			if !slices.Equal(miss, tc.wantMiss) {
				t.Errorf("missing = %v, want %v", miss, tc.wantMiss)
			}
			if !slices.Equal(extra, tc.wantExtra) {
				t.Errorf("extra = %v, want %v", extra, tc.wantExtra)
			}
			if !slices.Equal(dup, tc.wantDup) {
				t.Errorf("duplicated = %v, want %v", dup, tc.wantDup)
			}
			report := strings.Join(strictjsonUsersReport(miss, extra, dup, tc.users), "\n")
			if len(tc.wantReport) == 0 {
				if report != "" {
					t.Errorf("report = %q, want none", report)
				}
				return
			}
			for _, want := range tc.wantReport {
				if !strings.Contains(report, want) {
					t.Errorf("report %q does not contain %q", report, want)
				}
			}
		})
	}
}

// TestPackageReferenceStrictjsonUsers checks that the user list in
// package_reference.md names exactly the packages whose non-test sources import
// the component, with no duplicate.
func TestPackageReferenceStrictjsonUsers(t *testing.T) {
	doc := doctestutil.ReadDoc(t, filepath.Join(repoRoot, "docs/dev/conventions/package_reference.md"))
	cell, err := strictjsonRowCell(doc)
	if err != nil {
		t.Fatalf("find the %s user list: %v", strictjsonPackageName, err)
	}
	listed, err := parseUserList(cell)
	if err != nil {
		t.Fatalf("read the %s user list: %v", strictjsonPackageName, err)
	}
	sources, err := goSources(repoRoot)
	if err != nil {
		t.Fatalf("scan %s: %v", repoRoot, err)
	}
	if len(sources) == 0 {
		t.Fatalf("found no Go sources under %s/cmd or %s/internal", repoRoot, repoRoot)
	}
	users, err := importersOf(repoRoot, sources, strictjsonImportPath)
	if err != nil {
		t.Fatalf("read the imports under %s: %v", repoRoot, err)
	}
	if len(users) == 0 {
		t.Fatalf("found no non-test source under %s importing %s", repoRoot, strictjsonImportPath)
	}
	missing, extra, duplicated := diffUsers(listed, users)
	for _, line := range strictjsonUsersReport(missing, extra, duplicated, users) {
		t.Error(line)
	}
}
