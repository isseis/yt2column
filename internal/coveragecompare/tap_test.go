//go:build test

package coveragecompare

import (
	"slices"
	"strings"
	"testing"
)

const testExtensionDir = "/checkout/extension"

func TestParseTAP(t *testing.T) {
	valid := strings.Join([]string{
		"TAP version 13",
		"# Subtest: outer",
		"    # Subtest: inner",
		"    not ok 1 - inner",
		"      ---",
		"      duration_ms: 1.022",
		"      type: 'test'",
		"      location: '/checkout/extension/test/x.test.ts:4:5'",
		"      failureType: 'cancelledByParent'",
		"      error: 'test did not finish before its parent and was cancelled'",
		"      code: 'ERR_TEST_FAILURE'",
		"      ...",
		"    1..1",
		"not ok 1 - outer",
		"  ---",
		"  duration_ms: 1.1",
		"  type: 'test'",
		"  location: '/checkout/extension/test/x.test.ts:3:1'",
		"  failureType: 'testCodeFailure'",
		"  error: 'boom'",
		"  stack: |-",
		"    TestContext.<anonymous> (file:///checkout/extension/test/x.test.ts:5:10)",
		"  ...",
		"1..1",
		"# tests 2",
		"# suites 0",
		"# pass 0",
		"# fail 1",
		"# cancelled 1",
		"# skipped 0",
		"# todo 0",
		"# duration_ms 73.1",
	}, "\n") + "\n"

	t.Run("reads counts, failures, and the first diagnostic location", func(t *testing.T) {
		report, err := parseTAP([]byte(valid), testExtensionDir, PointBase)
		if err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
		if report.Tests != 2 || report.Fail != 1 || report.Cancelled != 1 {
			t.Errorf("counts = tests %d fail %d cancelled %d, want 2 1 1", report.Tests, report.Fail, report.Cancelled)
		}
		want := []tapFailure{
			{Name: "inner", File: "extension/test/x.test.ts"},
			{Name: "outer"},
		}
		if !slices.Equal(report.Failures, want) {
			t.Errorf("failures = %+v, want %+v", report.Failures, want)
		}
	})

	t.Run("ignores an indented # tests line", func(t *testing.T) {
		data := "    # tests 5\n" + valid
		report, err := parseTAP([]byte(data), testExtensionDir, PointBase)
		if err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
		if report.Tests != 2 {
			t.Errorf("tests = %d, want 2", report.Tests)
		}
	})

	t.Run("reads a quoted location", func(t *testing.T) {
		data := strings.Join([]string{
			"not ok 1 - x",
			`  location: "/checkout/extension/test/y.test.ts:10:2"`,
			"# tests 1",
			"# fail 1",
			"# cancelled 0",
		}, "\n") + "\n"
		report, err := parseTAP([]byte(data), testExtensionDir, PointHead)
		if err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
		if len(report.Failures) != 1 || report.Failures[0].File != "extension/test/y.test.ts" {
			t.Errorf("failures = %+v, want the file from the quoted location", report.Failures)
		}
	})

	t.Run("leaves the file empty when the location is outside the extension", func(t *testing.T) {
		data := strings.Join([]string{
			"not ok 1 - x",
			"  location: '/elsewhere/x.test.ts:1:1'",
			"# tests 1",
			"# fail 1",
			"# cancelled 0",
		}, "\n") + "\n"
		report, err := parseTAP([]byte(data), testExtensionDir, PointBase)
		if err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
		if report.Failures[0].File != "" {
			t.Errorf("file = %q, want empty", report.Failures[0].File)
		}
	})

	t.Run("accepts a diagnostic line over 1 MiB", func(t *testing.T) {
		long := strings.Repeat("e", 1<<20)
		data := strings.Join([]string{
			"not ok 1 - x",
			"  ---",
			"  error: '" + long + "'",
			"  location: '/checkout/extension/test/x.test.ts:1:1'",
			"  ...",
			"# tests 1",
			"# fail 1",
			"# cancelled 0",
		}, "\n") + "\n"
		if _, err := parseTAP([]byte(data), testExtensionDir, PointBase); err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
	})

	t.Run("reads not ok at any indentation", func(t *testing.T) {
		data := strings.Join([]string{
			"        not ok 3 - deep",
			"not ok 4 - shallow",
			"# tests 2",
			"# fail 2",
			"# cancelled 0",
		}, "\n") + "\n"
		report, err := parseTAP([]byte(data), testExtensionDir, PointBase)
		if err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
		want := []tapFailure{{Name: "deep"}, {Name: "shallow"}}
		if !slices.Equal(report.Failures, want) {
			t.Errorf("failures = %+v, want %+v", report.Failures, want)
		}
	})

	tests := map[string]string{
		"missing tests line":         "# fail 0\n# cancelled 0\n",
		"missing fail line":          "# tests 1\n# cancelled 0\n",
		"missing cancelled line":     "# tests 1\n# fail 0\n",
		"two unindented tests lines": "# tests 1\n# tests 1\n# fail 0\n# cancelled 0\n",
		"two fail lines":             "# tests 1\n# fail 0\n# fail 0\n# cancelled 0\n",
		"two cancelled lines":        "# tests 1\n# fail 0\n# cancelled 0\n# cancelled 0\n",
		"negative test count":        "# tests -1\n# fail 0\n# cancelled 0\n",
		"fractional fail count":      "# tests 1\n# fail 0.5\n# cancelled 0\n",
		"signed cancelled count":     "# tests 1\n# fail 0\n# cancelled +0\n",
		"oversized test count":       "# tests " + strings.Repeat("9", 40) + "\n# fail 0\n# cancelled 0\n",
	}

	for name, data := range tests {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := parseTAP([]byte(data), testExtensionDir, PointBase)
			requireStop(t, err, PointBase, InputTAP)
		})
	}
}
