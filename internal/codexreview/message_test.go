//go:build test

package codexreview

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// readTestdata returns a file under testdata as a string.
func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// TestParseFinalMessage checks the two accepted forms of a final message and
// that every other shape is unreadable with no findings. The testdata
// messages are fictional, in the shapes codex wrote in the recorded runs.
func TestParseFinalMessage(t *testing.T) {
	// sep is the separator before a finding's location: an em dash between spaces.
	const sep = " \u2014 "
	const summary = "A summary paragraph.\n\n"
	const finding = "- [P2] A title" + sep + "/work/repo/a.go:3-4\n  The body.\n"
	tests := []struct {
		name     string
		text     string
		form     messageForm
		findings []rawFinding
	}{
		{
			name: "two_findings",
			text: readTestdata(t, "final_message_two_findings.txt"),
			form: formFindings,
			findings: []rawFinding{
				{
					path: "/work/repo/internal/sample/inside.go", start: 5, end: 7,
					title: "Writes to a nil map",
					body:  "Counts writes to a map it never initialized, so the first call panics.\nCreate it with make before writing.",
				},
				{
					// The title holds the separator itself; the location follows the last one.
					path: "/work/repo/internal/sample/limit.go", start: 12, end: 12,
					title: "Boundary" + sep + "the limit itself is rejected",
					body:  "An input exactly as long as the limit should be accepted, but the comparison uses <= instead of <.\n\nThe tests only try a value one below the limit.",
				},
			},
		},
		{
			name: "one_finding",
			text: readTestdata(t, "final_message_one_finding.txt"),
			form: formFindings,
			findings: []rawFinding{{
				path: "/work/repo/internal/sample/index.go", start: 3, end: 4,
				title: "Allows a negative index",
				body:  "When the argument is 0, i-1 is -1 and the read goes out of range.",
			}},
		},
		{
			name: "no_findings",
			text: readTestdata(t, "final_message_no_findings.txt"),
			form: formNoFindings,
		},
		{
			name:     "finding_without_body",
			text:     summary + "Review comment:\n- [P1] A title" + sep + "/work/repo/a.go:1-2\n",
			form:     formFindings,
			findings: []rawFinding{{path: "/work/repo/a.go", start: 1, end: 2, title: "A title"}},
		},
		{name: "heading_without_findings", text: summary + "Review comment:\n\n", form: formUnreadable},
		{name: "two_headings", text: summary + "Review comment:\n" + finding + "Full review comments:\n" + finding, form: formUnreadable},
		{name: "changed_finding_marker", text: summary + "Review comment:\n* [P1] A title" + sep + "/work/repo/a.go:3-4\n", form: formUnreadable},
		{name: "unindented_line_after_heading", text: summary + "Review comment:\n" + finding + "A closing remark.\n", form: formUnreadable},
		{name: "body_before_first_finding", text: summary + "Review comment:\n  A stray body line.\n" + finding, form: formUnreadable},
		{name: "start_after_end", text: summary + "Review comment:\n- [P1] A title" + sep + "/work/repo/a.go:5-4\n", form: formUnreadable},
		{name: "zero_line", text: summary + "Review comment:\n- [P1] A title" + sep + "/work/repo/a.go:0-4\n", form: formUnreadable},
		{name: "relative_path", text: summary + "Review comment:\n- [P1] A title" + sep + "repo/a.go:3-4\n", form: formUnreadable},
		{name: "single_line_location", text: summary + "Review comment:\n- [P1] A title" + sep + "/work/repo/a.go:3\n", form: formUnreadable},
		{name: "blank_title", text: summary + "Review comment:\n- [P1]  " + sep + "/work/repo/a.go:3-4\n", form: formUnreadable},
		{name: "heading_with_trailing_space", text: summary + "Review comment: \n" + finding, form: formUnreadable},
		{name: "no_summary", text: "Review comment:\n" + finding, form: formUnreadable},
		{name: "finding_line_in_summary", text: finding + "\nReview comment:\n" + finding, form: formUnreadable},
		{name: "two_paragraphs_without_heading", text: "First paragraph.\n\nSecond paragraph.\n", form: formUnreadable},
		{name: "list_marker_without_heading", text: "Notes on the change.\n- one item\n", form: formUnreadable},
		{name: "numbered_item_without_heading", text: "Notes on the change.\n1. one item\n", form: formUnreadable},
		{name: "priority_marker_without_heading", text: "One issue [P1] is in a.go.\n", form: formUnreadable},
		{name: "colon_at_end_without_heading", text: "The findings are:\n", form: formUnreadable},
		{name: "blank", text: "\n\n", form: formUnreadable},
		{name: "invalid_utf8", text: "No issues found \xff.\n", form: formUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form, findings := parseFinalMessage(tt.text)
			if form != tt.form {
				t.Fatalf("form = %d, want %d", form, tt.form)
			}
			if !reflect.DeepEqual(findings, tt.findings) {
				t.Errorf("findings = %#v, want %#v", findings, tt.findings)
			}
		})
	}
}
