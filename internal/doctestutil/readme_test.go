//go:build test

package doctestutil

import (
	"fmt"
	"testing"
)

// fatalRecorder is a testing.TB that records a Fatal message instead of
// failing the test. Fatalf deliberately does not end the calling goroutine,
// so the helper under test keeps running and the caller can observe what it
// returned after reporting.
type fatalRecorder struct {
	testing.TB
	message string
}

// Fatalf records the formatted message instead of failing.
func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.message = fmt.Sprintf(format, args...)
}

// TestLocatorIn checks that a locator gives the spelling of each README
// language, and that a language without a spelling, the zero value and an
// out-of-range value included, and an empty spelling fail the test and
// return "" rather than a value that would match nothing silently.
func TestLocatorIn(t *testing.T) {
	both := Locator{JA: "## 設定", EN: "## Configuration"}
	for _, tc := range []struct {
		name    string
		locator Locator
		lang    Lang
		want    string
		fails   bool
	}{
		{"japanese", both, LangJA, "## 設定", false},
		{"english", both, LangEN, "## Configuration", false},
		{"unknown language", both, LangUnknown, "", true},
		{"out of range", both, LangEN + 1, "", true},
		{"empty japanese", Locator{EN: "## Configuration"}, LangJA, "", true},
		{"empty english", Locator{JA: "## 設定"}, LangEN, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &fatalRecorder{TB: t}
			got := tc.locator.In(recorder, tc.lang)
			if got != tc.want {
				t.Errorf("In = %q, want %q", got, tc.want)
			}
			if failed := recorder.message != ""; failed != tc.fails {
				t.Errorf("In reported a failure %t (%q), want %t", failed, recorder.message, tc.fails)
			}
		})
	}
}

// TestRejectionDisplayIn checks that each README language gets its own
// spelling of the display categories and that any other language fails.
func TestRejectionDisplayIn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lang  Lang
		want  RejectionDisplaySpelling
		fails bool
	}{
		{"japanese", LangJA, RejectionDisplayJA, false},
		{"english", LangEN, RejectionDisplayEN, false},
		{"unknown language", LangUnknown, RejectionDisplaySpelling{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &fatalRecorder{TB: t}
			if got := RejectionDisplayIn(recorder, tc.lang); got != tc.want {
				t.Errorf("RejectionDisplayIn = %+v, want %+v", got, tc.want)
			}
			if failed := recorder.message != ""; failed != tc.fails {
				t.Errorf("RejectionDisplayIn reported a failure %t (%q), want %t", failed, recorder.message, tc.fails)
			}
		})
	}
}
