//go:build test

package main

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief"
	"github.com/isseis/yt2column/internal/doctestutil"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	return doctestutil.ReadDoc(t, filepath.Join(repoRoot, filepath.FromSlash(rel)))
}

// statusTable is the document status table a plan needs to be read.
const statusTable = "## Document Status\n\n| Item | Value |\n|---|---|\n| Status | `approved` |\n\n"

// codeSpan matches a code span.
var codeSpan = regexp.MustCompile("`([^`]+)`")

// numberedHeading matches a section heading with a number, such as the
// subsection heading mkplan.md step 8.5 names, and not a heading it names to
// count, such as a phase's.
var numberedHeading = regexp.MustCompile(`^### [0-9]+\.`)

// autoSendOf reads the auto-send field of a plan's first PR with the tool's
// plan reader.
func autoSendOf(t *testing.T, plan string) chief.AutoSend {
	t.Helper()
	p, err := chief.ParsePlan([]byte(statusTable+plan), nil)
	if err != nil {
		t.Fatalf("the tool cannot read the plan: %v\n%s", err, plan)
	}
	pr, ok := p.NextPR()
	if !ok {
		t.Fatalf("the plan has no PR to create:\n%s", plan)
	}
	return pr.AutoSend
}

// autoSendValues are the values each documented form is filled with and
// what the tool must read them as.
var autoSendValues = map[string]chief.AutoSend{"auto": chief.AutoSendAuto, "hold": chief.AutoSendHold}

// TestMarkerFormsCarryAutoSend checks that the forms the commands make a plan
// write hold the auto-send field in a form the tool reads: the PR marker of
// mkplan2.md step 6 and the PR structure subsection of mkplan.md step 8.5
// (which _lib/simple-process.md writes through), each filled with both
// values; and that _context.md lists the field among the PR marker's fields
// under the label of those forms.
func TestMarkerFormsCarryAutoSend(t *testing.T) {
	marker := markerForm(t)
	label := fieldLabel(t, marker)
	for value, want := range autoSendValues {
		if got := autoSendOf(t, fillAutoSend(t, marker, label, value)); got != want {
			t.Errorf("mkplan2.md step 6 with %s: the tool reads %d, want %d", value, got, want)
		}
		if got := autoSendOf(t, singleForm(t, label, value)); got != want {
			t.Errorf("mkplan.md step 8.5 with %s: the tool reads %d, want %d", value, got, want)
		}
	}
	fields := contextFields(t)
	if !slices.Contains(fields, strings.TrimSuffix(label, ": ")) {
		t.Errorf("_context.md lists the fields %v, without %q", fields, strings.TrimSuffix(label, ": "))
	}
}

// markerForm returns the PR marker form of mkplan2.md step 6, its indent
// removed and its marker number made 1.
func markerForm(t *testing.T) string {
	t.Helper()
	doc := readRepoFile(t, ".claude/commands/mkplan2.md")
	i := strings.Index(doc, "\n6. Insert `### PR-N ")
	if i < 0 {
		t.Fatal("mkplan2.md has no step 6 that inserts the PR markers")
	}
	blocks := doctestutil.FencedBlocks(doc[i:])
	if len(blocks) == 0 || !blocks[0].Closed {
		t.Fatal("mkplan2.md step 6 has no closed code block")
	}
	var lines []string
	for _, l := range blocks[0].Lines {
		lines = append(lines, strings.TrimPrefix(strings.TrimSuffix(l, "\n"), "   "))
	}
	form := strings.Join(lines, "\n") + "\n"
	return strings.Replace(form, "### PR-N ", "### PR-1 ", 1)
}

// fieldLabel returns the label of the form's field line whose values are
// the auto-send values, such as "**X**: " of "**X**: auto | hold".
func fieldLabel(t *testing.T, form string) string {
	t.Helper()
	var found []string
	for line := range strings.Lines(form) {
		label, values, ok := strings.Cut(strings.TrimSuffix(line, "\n"), ": ")
		if !ok || !strings.HasPrefix(label, "**") {
			continue
		}
		alternatives := strings.Split(values, " | ")
		slices.Sort(alternatives)
		if slices.Equal(alternatives, []string{"auto", "hold"}) {
			found = append(found, label+": ")
		}
	}
	if len(found) != 1 {
		t.Fatalf("the marker form has %d lines of the values auto | hold, want one:\n%s", len(found), form)
	}
	return found[0]
}

// fillAutoSend returns form with its auto-send line given value.
func fillAutoSend(t *testing.T, form, label, value string) string {
	t.Helper()
	line := label + "auto | hold\n"
	if !strings.Contains(form, line) {
		t.Fatalf("the form has no line %q", line)
	}
	return strings.Replace(form, line, label+value+"\n", 1)
}

// singleForm returns the PR structure subsection mkplan.md step 8.5 writes,
// built from the heading and the field lines it names, with the auto-send
// field set to value.
func singleForm(t *testing.T, label, value string) string {
	t.Helper()
	doc := readRepoFile(t, ".claude/commands/mkplan.md")
	start := strings.Index(doc, "\n8.5. ")
	end := strings.Index(doc, "\n9. ")
	if start < 0 || end < start {
		t.Fatal("mkplan.md has no step 8.5 before step 9")
	}
	step := doc[start:end]
	heading := ""
	var fields []string
	for _, m := range codeSpan.FindAllStringSubmatch(step, -1) {
		switch span := m[1]; {
		case numberedHeading.MatchString(span) && heading == "":
			heading = span
		case strings.HasPrefix(span, "**") && strings.Contains(span, "**: "):
			fields = append(fields, span)
		}
	}
	if heading == "" {
		t.Fatal("mkplan.md step 8.5 names no subsection heading")
	}
	var lines []string
	for _, f := range fields {
		if strings.HasPrefix(f, label) {
			f = label + value
		}
		lines = append(lines, f)
	}
	if n := len(slices.DeleteFunc(slices.Clone(lines), func(l string) bool { return l != label+value })); n != 1 {
		t.Fatalf("mkplan.md step 8.5 names the field lines %v, %d of them %q; want one", fields, n, label)
	}
	chapter, _, _ := strings.Cut(strings.TrimPrefix(heading, "### "), ".")
	return "## " + chapter + ". x\n\n" + heading + "\n\n" + strings.Join(lines, "\n\n") + "\n"
}

// contextFields returns the field labels _context.md's PR marker
// conventions list, without their asterisks' content changed.
func contextFields(t *testing.T) []string {
	t.Helper()
	section := doctestutil.SectionByHeading(readRepoFile(t, ".claude/commands/_context.md"), "### PR marker conventions")
	for line := range strings.Lines(section) {
		rest, ok := strings.CutPrefix(line, "- Fields: ")
		if !ok {
			continue
		}
		var fields []string
		for _, m := range codeSpan.FindAllStringSubmatch(rest, -1) {
			fields = append(fields, m[1])
		}
		return fields
	}
	t.Fatal("_context.md's PR marker conventions have no field list")
	return nil
}
