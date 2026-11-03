//go:build test

package coveragecompare

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
)

// goTableSource is a Go file whose line numbers the table tests rely on:
//
//	3-5   a package variable initialized by a function literal
//	8-13  f, with a doc comment on line 7
//	17    T.g, a value receiver with a type parameter
//	19-20 (*T).p
//	21    a and b, two declarations on one line
const goTableSource = `package x

var v = func() int {
	return 1
}()

// f has a doc comment.
func f() int {
	if v > 0 {
		return 1
	}
	return 0
}

type T[K any] struct{}

func (T[K]) g() {}

func (t *T[K]) p() {
}
func a() {}; func b() {}
`

func TestBuildGoTable(t *testing.T) {
	src := &fakeSource{at: PointBase, files: map[string]string{
		"x/x.go": goTableSource,
		"x/y.go": "package x\n\nfunc unrecorded() {\n\tprintln()\n}\n",
	}}
	profile := &goProfile{Blocks: []profileBlock{
		// The initializer's block starts outside every declaration.
		{File: "x/x.go", StartLine: 3, StartCol: 20, EndLine: 5, EndCol: 2, Statements: 1, Count: 1},
		{File: "x/x.go", StartLine: 8, StartCol: 14, EndLine: 9, EndCol: 10, Statements: 1, Count: 1},
		{File: "x/x.go", StartLine: 9, StartCol: 10, EndLine: 11, EndCol: 3, Statements: 1000, Count: 0},
		{File: "x/x.go", StartLine: 12, StartCol: 2, EndLine: 12, EndCol: 10, Statements: 999, Count: 1},
		{File: "x/x.go", StartLine: 19, StartCol: 20, EndLine: 20, EndCol: 2, Statements: 0, Count: 0},
	}}

	table, err := buildGoTable(src, profile, []string{"x/x.go", "x/y.go"})
	if err != nil {
		t.Fatalf("buildGoTable: %v", err)
	}

	stmts := func(covered, total int) Coverage {
		return Coverage{Unit: UnitStatements, Covered: covered, Total: total}
	}
	want := []funcEntry{
		{ID: FuncID{"x/x.go", 8}, End: 13, Names: []string{"f"}, Recorded: true, Cov: stmts(1000, 2000)},
		{ID: FuncID{"x/x.go", 17}, End: 17, Names: []string{"T.g"}, Recorded: true, Cov: stmts(0, 0)},
		{ID: FuncID{"x/x.go", 19}, End: 20, Names: []string{"(*T).p"}, Recorded: true, Cov: stmts(0, 0)},
		{ID: FuncID{"x/x.go", 21}, End: 21, Names: []string{"a", "b"}, Recorded: true, Cov: stmts(0, 0)},
		{ID: FuncID{"x/y.go", 3}, End: 5, Names: []string{"unrecorded"}, Cov: stmts(0, 0)},
	}
	if !reflect.DeepEqual(table.entries, want) {
		t.Errorf("entries =\n%+v\nwant\n%+v", table.entries, want)
	}
	if want := (Measure{Files: 1, Functions: 5}); table.measure != want {
		t.Errorf("measure = %+v, want %+v (only the profiled file counts)", table.measure, want)
	}
	if len(table.unmatched) != 1 {
		t.Fatalf("unmatched = %+v, want one reason for the two declarations on line 21", table.unmatched)
	}
	if got := table.unmatched[0]; got.Func == nil || *got.Func != (FuncID{"x/x.go", 21}) || got.Point != PointBase {
		t.Errorf("unmatched reason = %+v, want x/x.go:21 at base", got)
	}

	t.Run("an unparsable file stops as unreadable", func(t *testing.T) {
		broken := &fakeSource{at: PointHead, files: map[string]string{"b.go": "package b\n\nfunc {\n"}}
		_, err := buildGoTable(broken, &goProfile{}, []string{"b.go"})
		reason := requireStop(t, err, PointHead, InputSource)
		if reason.Line != 3 {
			t.Errorf("reason line = %d, want 3", reason.Line)
		}
	})
}

func TestBuildExtTable(t *testing.T) {
	const file = "extension/src/a.ts"
	ranges := fileRanges{
		// Two functions start on line 1 (an arrow returning an arrow).
		Functions:   []lineRange{{Start: 1, End: 10}, {Start: 1, End: 3}, {Start: 12, End: 14}, {Start: 20, End: 22}},
		FieldGroups: []lineRange{{Start: 16, End: 18}},
	}
	lcovWith := func(names ...string) *lcovReport {
		return &lcovReport{Files: []lcovFile{{Path: file, Functions: []lcovFunction{
			{Line: 1, Name: names[0], Count: 2},
			{Line: 1, Name: names[1], Count: 0},
			{Line: 12, Name: "named", Count: 0},
			{Line: 16, Name: "<instance_members_initializer>", Count: 1},
		}}}}
	}
	src := &fakeSource{
		at:     PointBase,
		files:  map[string]string{file: "", "extension/src/b.ts": ""},
		ranges: map[string]fileRanges{file: ranges, "extension/src/b.ts": {Functions: []lineRange{{Start: 2, End: 4}}}},
	}

	table, err := buildExtTable(context.Background(), src, fakeRanges{}, lcovWith("anonymous_1", "anonymous_2"), []string{file, "extension/src/b.ts"})
	if err != nil {
		t.Fatalf("buildExtTable: %v", err)
	}
	fns := func(ran, total int) Coverage { return Coverage{Unit: UnitFunctions, Covered: ran, Total: total} }
	want := []funcEntry{
		{ID: FuncID{file, 1}, End: 10, Names: []string{"anonymous_1", "anonymous_2"}, Recorded: true, Cov: fns(1, 2)},
		{ID: FuncID{file, 12}, End: 14, Names: []string{"named"}, Recorded: true, Cov: fns(0, 1)},
		{ID: FuncID{file, 16}, End: 18, Names: []string{"<instance_members_initializer>"}, Recorded: true, Cov: fns(1, 1)},
		// A line without FN records has no name and no record.
		{ID: FuncID{file, 20}, End: 22},
		// A file the report does not name gives unrecorded rows.
		{ID: FuncID{"extension/src/b.ts", 2}, End: 4},
	}
	if !reflect.DeepEqual(table.entries, want) {
		t.Errorf("entries =\n%+v\nwant\n%+v", table.entries, want)
	}
	if want := (Measure{Files: 1, Functions: 4}); table.measure != want {
		t.Errorf("measure = %+v, want %+v", table.measure, want)
	}

	t.Run("anonymous names that change between runs do not change the rows", func(t *testing.T) {
		renamed, err := buildExtTable(context.Background(), src, fakeRanges{}, lcovWith("anonymous_7", "anonymous_3"), []string{file})
		if err != nil {
			t.Fatalf("buildExtTable: %v", err)
		}
		if len(renamed.entries) != 4 {
			t.Fatalf("rows = %+v, want the four rows of a.ts", renamed.entries)
		}
		for i, entry := range renamed.entries {
			entry.Names = nil
			original := table.entries[i]
			original.Names = nil
			if !reflect.DeepEqual(entry, original) {
				t.Errorf("row %d = %+v, want %+v apart from its names", i, entry, original)
			}
		}
	})

	t.Run("an FN line where no range starts stops as unreadable", func(t *testing.T) {
		report := &lcovReport{Files: []lcovFile{{Path: file, Functions: []lcovFunction{{Line: 2, Name: "inner", Count: 1}}}}}
		_, err := buildExtTable(context.Background(), src, fakeRanges{}, report, []string{file})
		reason := requireStop(t, err, PointBase, InputLCOV)
		if reason.Func == nil || *reason.Func != (FuncID{file, 2}) {
			t.Errorf("reason function = %v, want %s:2", reason.Func, file)
		}
	})

	t.Run("a file the range helper does not answer stops as unreadable", func(t *testing.T) {
		missing := &fakeSource{at: PointHead, files: map[string]string{file: ""}}
		_, err := buildExtTable(context.Background(), missing, fakeRanges{}, &lcovReport{}, []string{file})
		requireStop(t, err, PointHead, InputRanges)
	})

	t.Run("a range helper failure is returned as it is", func(t *testing.T) {
		failure := errors.New("helper failed")
		_, err := buildExtTable(context.Background(), src, fakeRanges{err: failure}, &lcovReport{}, []string{file})
		if !errors.Is(err, failure) {
			t.Errorf("error = %v, want the helper's failure", err)
		}
	})
}

func TestGoFuncNames(t *testing.T) {
	src := joinLines(
		"package x",
		"type P[K, V any] struct{}",
		"type T int",
		"func (P[K, V]) list() {}",
		"func (t (*T)) paren() {}",
		"func (t (T)) value() {}",
	)
	funcs, err := parseGoFuncs("x.go", []byte(src), PointBase)
	if err != nil {
		t.Fatalf("parseGoFuncs: %v", err)
	}
	var got []string
	for _, fn := range funcs {
		got = append(got, fn.Name)
	}
	if want := []string{"P.list", "(*T).paren", "T.value"}; !slices.Equal(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

func TestProductFiles(t *testing.T) {
	src := &fakeSource{at: PointHead, files: map[string]string{
		"a.go":            "package a\n",
		"helpers.go":      "//go:build test\n\npackage a\n",
		"extension/src/x": "",
	}}
	candidates := []string{"a.go", "absent.go", "helpers.go"}

	got, err := productFiles(isGoSourceProduct, src, report{}, candidates)
	if err != nil {
		t.Fatalf("productFiles: %v", err)
	}
	if want := []string{"a.go"}; !slices.Equal(got, want) {
		t.Errorf("Go product files = %v, want %v", got, want)
	}

	got, err = productFiles(isExtSourceProduct, src, report{}, []string{"extension/src/x", "a.go"})
	if err != nil {
		t.Fatalf("productFiles: %v", err)
	}
	if want := []string{"extension/src/x"}; !slices.Equal(got, want) {
		t.Errorf("extension product files = %v, want %v", got, want)
	}

	t.Run("a file the point's own report names must be present", func(t *testing.T) {
		_, err := productFiles(isGoSourceProduct, src, report{files: []string{"absent.go"}}, candidates)
		requireStop(t, err, PointHead, InputSource)
	})

	t.Run("a read failure is internal", func(t *testing.T) {
		failure := errors.New("read failed")
		broken := &fakeSource{at: PointBase, readErr: failure}
		_, err := productFiles(isGoSourceProduct, broken, report{}, []string{"a.go"})
		stop, ok := errors.AsType[*StopError](err)
		if !ok || stop.Kind != StopInternal || !errors.Is(err, failure) {
			t.Errorf("error = %v, want an internal stop wrapping the read failure", err)
		}
	})
}

func TestCandidateFiles(t *testing.T) {
	base := report{files: []string{"b.go", "c.go"}}
	head := report{files: []string{"a.go", "b.go"}}
	patch := &diffPatch{Files: []diffFile{
		{OldPath: "d.go", NewPath: "d.go"},
		{NewPath: "e.go"},
		{OldPath: "c.go"},
	}}
	if got, want := candidateFiles(base, head, patch), []string{"a.go", "b.go", "c.go", "d.go", "e.go"}; !slices.Equal(got, want) {
		t.Errorf("candidateFiles = %v, want %v", got, want)
	}
}

func TestGoModulePath(t *testing.T) {
	accepted := map[string]string{
		"module example.com/m\n\ngo 1.27\n":       "example.com/m",
		"// lead\nmodule example.com/m // note\n": "example.com/m",
		"module \"example.com/quoted\"\n":         "example.com/quoted",
	}
	for data, want := range accepted {
		got, err := goModulePath([]byte(data), PointBase)
		if err != nil || got != want {
			t.Errorf("goModulePath(%q) = %q, %v; want %q", data, got, err, want)
		}
	}
	rejected := []string{
		"go 1.27\n",
		"module (\n\texample.com/m\n)\n",
		"module \"\"\n",
		"module a b\n",
		"module \"unterminated\n",
	}
	for _, data := range rejected {
		_, err := goModulePath([]byte(data), PointHead)
		requireStop(t, err, PointHead, InputSource)
	}
}
