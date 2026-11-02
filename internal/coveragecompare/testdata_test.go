//go:build test

package coveragecompare

import (
	"os"
	"testing"
)

func TestParseRealSamples(t *testing.T) {
	t.Run("go profile", func(t *testing.T) {
		profile, err := parseGoProfile(readTestdata(t, "go-profile.txt"), testModulePath, PointBase)
		if err != nil {
			t.Fatalf("parseGoProfile: %v", err)
		}
		if len(profile.Blocks) != 9 {
			t.Errorf("blocks = %d, want 9", len(profile.Blocks))
		}
		first := profile.Blocks[0]
		want := profileBlock{
			File:       "internal/slackwebhook/slackwebhook.go",
			StartLine:  21,
			StartCol:   2,
			EndLine:    22,
			EndCol:     16,
			Statements: 2,
			Count:      1,
		}
		if first != want {
			t.Errorf("first block = %+v, want %+v", first, want)
		}
	})

	t.Run("go test -json", func(t *testing.T) {
		report, err := parseGoTestJSON(readTestdata(t, "go-test-json.txt"), PointBase)
		if err != nil {
			t.Fatalf("parseGoTestJSON: %v", err)
		}
		if report.Tests != 1 {
			t.Errorf("tests = %d, want 1", report.Tests)
		}
		if len(report.Failures) != 0 {
			t.Errorf("failures = %+v, want none", report.Failures)
		}
	})

	t.Run("lcov", func(t *testing.T) {
		report, err := parseLCOV(readTestdata(t, "lcov.txt"), PointBase)
		if err != nil {
			t.Fatalf("parseLCOV: %v", err)
		}
		if len(report.Files) != 2 {
			t.Fatalf("files = %+v, want 2", report.Files)
		}
		functions := 0
		for _, file := range report.Files {
			functions += len(file.Functions)
		}
		if functions != 15 {
			t.Errorf("functions = %d, want 15", functions)
		}
		if report.Files[0].Path != "extension/eslint.config.js" || isExtProduct(report.Files[0].Path) {
			t.Errorf("first file = %+v, want non-product eslint.config.js", report.Files[0])
		}
		if report.Files[1].Path != "extension/scripts/check-dist.ts" || !isExtProduct(report.Files[1].Path) {
			t.Errorf("second file = %+v, want product scripts/check-dist.ts", report.Files[1])
		}
	})

	t.Run("tap", func(t *testing.T) {
		report, err := parseTAP(readTestdata(t, "tap.txt"), "", PointBase)
		if err != nil {
			t.Fatalf("parseTAP: %v", err)
		}
		if report.Tests != 2006 || report.Fail != 0 || report.Cancelled != 0 {
			t.Errorf("counts = tests %d fail %d cancelled %d, want 2006 0 0", report.Tests, report.Fail, report.Cancelled)
		}
	})
}

// readTestdata reads one sample from testdata/.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return data
}
