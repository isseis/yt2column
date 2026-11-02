//go:build test

package coveragecompare

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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
		if len(report.Failures) != 2 {
			t.Fatalf("failures = %+v, want 2", report.Failures)
		}
		if report.Failures[0].Name != "inner" || report.Failures[0].File != "extension/test/x.test.ts" {
			t.Errorf("first failure = %+v, want inner in extension/test/x.test.ts", report.Failures[0])
		}
		if report.Failures[1].Name != "outer" {
			t.Errorf("second failure = %+v, want outer", report.Failures[1])
		}
		diagnostic := report.Failures[0].Diagnostic
		if len(diagnostic) != 8 || diagnostic[len(diagnostic)-1] != "      ..." {
			t.Errorf("diagnostic = %q, want the block ending at ...", diagnostic)
		}
		if !slices.Contains(diagnostic, "      error: 'test did not finish before its parent and was cancelled'") {
			t.Errorf("diagnostic = %q, want the error line", diagnostic)
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

	t.Run("leaves the file empty when the location is not a path with a line and a column", func(t *testing.T) {
		data := strings.Join([]string{
			"not ok 1 - x",
			"  location: 'somewhere else'",
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

	t.Run("resolves a checkout path whose symbolic links node resolved", func(t *testing.T) {
		base := t.TempDir()
		realDir := filepath.Join(base, "real-extension")
		if err := os.MkdirAll(filepath.Join(realDir, "test"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(realDir, "test", "x.test.ts"), nil, 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		linkDir := filepath.Join(base, "link-extension")
		if err := os.Symlink(realDir, linkDir); err != nil {
			t.Skipf("symbolic links are unavailable: %v", err)
		}
		resolved, err := filepath.EvalSymlinks(realDir)
		if err != nil {
			t.Fatalf("EvalSymlinks: %v", err)
		}

		// The directory passed in is a link; only resolving it matches node's
		// location.
		report := parseTapFailureAt(t, resolved+"/test/x.test.ts:1:1", linkDir)
		if report.Failures[0].File != "extension/test/x.test.ts" {
			t.Errorf("file = %q, want extension/test/x.test.ts", report.Failures[0].File)
		}

		// The location is the link path and the directory is real; only
		// resolving the file's path matches.
		report = parseTapFailureAt(t, linkDir+"/test/x.test.ts:1:1", realDir)
		if report.Failures[0].File != "extension/test/x.test.ts" {
			t.Errorf("file = %q, want extension/test/x.test.ts", report.Failures[0].File)
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
		report, err := parseTAP([]byte(data), testExtensionDir, PointBase)
		if err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
		diagnostic := strings.Join(report.Failures[0].Diagnostic, "\n")
		if !strings.Contains(diagnostic, long) {
			t.Error("the diagnostic block does not hold the whole long line")
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

// parseTapFailureAt parses one TAP report with a single failure at location,
// measured from extensionDir.
func parseTapFailureAt(t *testing.T, location, extensionDir string) *tapReport {
	t.Helper()
	data := strings.Join([]string{
		"not ok 1 - x",
		"  location: '" + location + "'",
		"# tests 1",
		"# fail 1",
		"# cancelled 0",
	}, "\n") + "\n"
	report, err := parseTAP([]byte(data), extensionDir, PointBase)
	if err != nil {
		t.Fatalf("parseTAP: %v", err)
	}
	return report
}
