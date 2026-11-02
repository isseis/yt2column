//go:build test

package coveragecompare

import (
	"slices"
	"strings"
	"testing"
)

func TestParseGoTestJSON(t *testing.T) {
	t.Run("counts top-level tests and keeps the first failures' output", func(t *testing.T) {
		data := strings.Join([]string{
			`{"Action":"run","Package":"example/p","Test":"TestA"}`,
			`{"Action":"output","Package":"example/p","Test":"TestA","Output":"=== RUN   TestA\n"}`,
			`{"Action":"run","Package":"example/p","Test":"TestA/sub"}`,
			`{"Action":"pass","Package":"example/p","Test":"TestA/sub"}`,
			`{"Action":"output","Package":"example/p","Test":"TestA","Output":"    a_test.go:1: message\n"}`,
			`{"Action":"fail","Package":"example/p","Test":"TestA"}`,
			`{"Action":"pass","Package":"example/p","Test":"TestB"}`,
			`{"Action":"skip","Package":"example/p","Test":"TestC"}`,
			`{"Action":"fail","Package":"example/p"}`,
		}, "\n") + "\n"

		report, err := parseGoTestJSON([]byte(data), PointBase)
		if err != nil {
			t.Fatalf("parseGoTestJSON: %v", err)
		}
		if report.Tests != 3 {
			t.Errorf("tests = %d, want 3 (subtests are not counted)", report.Tests)
		}
		want := []goTestFailure{
			{Package: "example/p", Test: "TestA", Output: []string{
				"=== RUN   TestA\n",
				"    a_test.go:1: message\n",
			}},
			{Package: "example/p"},
		}
		if !slices.EqualFunc(report.Failures, want, equalGoTestFailure) {
			t.Errorf("failures = %+v, want %+v", report.Failures, want)
		}
	})

	t.Run("reads a build failure's compiler output once", func(t *testing.T) {
		// The shape go1.27.1 writes: build-output lines keyed by the import
		// path, a build-fail, then the package's own output and fail lines.
		data := strings.Join([]string{
			`{"ImportPath":"example/p/q [example/p/q.test]","Action":"build-output","Output":"./x_test.go:5:28: undefined: y\n"}`,
			`{"ImportPath":"example/p/q [example/p/q.test]","Action":"build-fail"}`,
			`{"Action":"output","Package":"example/p/q","Output":"FAIL\texample/p/q [build failed]\n"}`,
			`{"Action":"fail","Package":"example/p/q","FailedBuild":"example/p/q [example/p/q.test]"}`,
		}, "\n") + "\n"

		report, err := parseGoTestJSON([]byte(data), PointHead)
		if err != nil {
			t.Fatalf("parseGoTestJSON: %v", err)
		}
		if report.Tests != 0 {
			t.Errorf("tests = %d, want 0", report.Tests)
		}
		want := []goTestFailure{{
			Package: "example/p/q",
			Output:  []string{"./x_test.go:5:28: undefined: y\n", "FAIL\texample/p/q [build failed]\n"},
		}}
		if !slices.EqualFunc(report.Failures, want, equalGoTestFailure) {
			t.Errorf("failures = %+v, want %+v", report.Failures, want)
		}
	})

	t.Run("accepts an output line over 1 MiB", func(t *testing.T) {
		long := strings.Repeat("x", 1<<20)
		data := `{"Action":"output","Package":"example/p","Output":"` + long + `"}` + "\n"
		report, err := parseGoTestJSON([]byte(data), PointBase)
		if err != nil {
			t.Fatalf("parseGoTestJSON: %v", err)
		}
		if report.Tests != 0 {
			t.Errorf("tests = %d, want 0", report.Tests)
		}
	})

	t.Run("rejects malformed JSON lines", func(t *testing.T) {
		tests := map[string]string{
			"empty line":       "\n",
			"plain text":       "not json\n",
			"JSON array":       "[]\n",
			"duplicate member": `{"Action":"pass","Action":"fail"}`,
			"non-string field": `{"Action":1}`,
			"null output":      `{"Action":"output","Output":null}`,
		}
		for name, data := range tests {
			t.Run(name, func(t *testing.T) {
				_, err := parseGoTestJSON([]byte(data), PointBase)
				requireStop(t, err, PointBase, InputGoTestJSON)
			})
		}
	})
}

// equalGoTestFailure compares two failures including their output lines.
func equalGoTestFailure(a, b goTestFailure) bool {
	return a.Package == b.Package && a.Test == b.Test && slices.Equal(a.Output, b.Output)
}
