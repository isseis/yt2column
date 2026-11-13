//go:build test

package main

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

// developmentDocPath is the development guide from this package's directory.
const developmentDocPath = "../../docs/dev/guides/development.md"

// TestDevelopmentDocsDevServer pins the values of the development guide's
// section on the development server to the code: the argument names, the
// values each argument accepts, the longest delay, the model name, the title
// marker, and the make target. Each argument has exactly one row, and no row
// names an argument the code does not have. The accepted-values cells of the
// three enumeration rows must list exactly the values the code accepts, in any
// order, so a wrong, misplaced, or extra value fails the test; the delay row
// must name the two endpoints of the code's range. The values come from this
// package's argument names and its delay bound and from devfake; the make
// target is a literal, because the guide's rule and the Makefile rule share
// only a name that TestMakeBuildDev checks.
func TestDevelopmentDocsDevServer(t *testing.T) {
	development := doctestutil.ReadDoc(t, developmentDocPath)
	section := doctestutil.SectionOutsideBlocks(development, doctestutil.DevServerHeading)
	if section == "" {
		t.Fatalf("development.md has no %q section", doctestutil.DevServerHeading)
	}
	table := doctestutil.FindTable(section, doctestutil.DevArgumentColumn)
	if table == nil {
		t.Fatalf("development.md %q has no arguments table", doctestutil.DevServerHeading)
	}
	accepted := doctestutil.ColumnIndex(table[0], doctestutil.DevAcceptedColumn)
	if accepted < 0 {
		t.Fatalf("development.md %q arguments table has no %q column", doctestutil.DevServerHeading, doctestutil.DevAcceptedColumn)
	}

	known := map[string]struct{}{
		"-" + argArticle:  {},
		"-" + argFail:     {},
		"-" + argFailStep: {},
		"-" + argDelay:    {},
	}
	rowsFor := map[string][][]string{}
	for _, row := range table[1:] {
		if len(row) == 0 {
			continue
		}
		name := doctestutil.StripBackticks(row[0])
		if _, ok := known[name]; !ok {
			t.Errorf("development.md %q has a row for unknown argument %q", doctestutil.DevServerHeading, name)
			continue
		}
		rowsFor[name] = append(rowsFor[name], row)
	}

	checkAccepted := func(arg string, want []string) {
		t.Helper()
		rows := rowsFor["-"+arg]
		if len(rows) != 1 {
			t.Errorf("development.md %q has %d rows for -%s, want 1", doctestutil.DevServerHeading, len(rows), arg)
			return
		}
		row := rows[0]
		if len(row) <= accepted {
			t.Errorf("development.md -%s row has too few cells: %q", arg, row)
			return
		}
		got := backticked(row[accepted])
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("development.md -%s accepts %v, want exactly %v", arg, got, want)
		}
	}

	var articles, failures, failAts []string
	for _, article := range devfake.Articles() {
		articles = append(articles, article.String())
	}
	for _, failure := range devfake.Failures() {
		failures = append(failures, failure.String())
	}
	for _, failAt := range devfake.FailAts() {
		failAts = append(failAts, failAt.String())
	}
	checkAccepted(argArticle, articles)
	checkAccepted(argFail, failures)
	checkAccepted(argFailStep, failAts)
	checkAccepted(argDelay, []string{"0", strconv.FormatUint(maxDelaySeconds, 10)})

	for _, want := range []string{
		"`" + devfake.ModelName + "`",
		"`" + devfake.TitleMarker + "`",
		"`make build-dev`",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("development.md %q does not mention %s", doctestutil.DevServerHeading, want)
		}
	}
}

// backticked returns the code spans of a table cell, in order.
func backticked(cell string) []string {
	var spans []string
	for {
		_, rest, found := strings.Cut(cell, "`")
		if !found {
			return spans
		}
		span, rest, found := strings.Cut(rest, "`")
		if !found {
			return spans
		}
		spans = append(spans, span)
		cell = rest
	}
}
