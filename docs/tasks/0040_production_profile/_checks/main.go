// Command _checks verifies the before/after relations of task 0040's
// acceptance criteria. It compares the merge-base of origin/main and HEAD with
// HEAD and prints one result per check, with the line number and content of
// each line that does not hold:
//
//   - the README's headings outside code blocks, without the two headings this
//     task adds, are the same, in the same order;
//   - the 0008 evaluation record's section 6 keeps every existing line in
//     order (only additions are allowed), and its Document Status rows for
//     status, review date, and reviewer are unchanged;
//   - security.md from section 1 on is unchanged;
//   - project_overview.md's environment variable section is unchanged.
//
// Run it once on the PR branch:
//
//	go run ./docs/tasks/0040_production_profile/_checks/
//
// make test and CI do not run it. It reads no environment variable and names
// no secret: the directory starts with an underscore, which keeps it out of
// go list but not out of the repository-wide environment access check.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const (
	readmePath      = "README.md"
	evaluationPath  = "docs/tasks/0008_column_prompt/02_evaluation.md"
	securityPath    = "docs/dev/security.md"
	overviewPath    = "docs/dev/project_overview.md"
	overviewHeading = "## 設定（環境変数）"

	evaluationSectionPrefix = "## 6."
	securitySectionPrefix   = "## 1."

	quickStartHeading        = "## Quick start"
	recommendedConfigHeading = "### Recommended configuration"
)

func main() {
	base, err := gitOutput("merge-base", "origin/main", "HEAD")
	if err != nil {
		fail("merge-base", err)
	}
	head, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		fail("rev-parse", err)
	}
	fmt.Printf("merge-base %s\n", base)
	fmt.Printf("head       %s\n", head)

	checks := []struct {
		name string
		run  func(base, head string) []string
	}{
		{"README headings", checkReadmeHeadings},
		{"evaluation record section 6", checkEvaluationSection},
		{"security.md section 1 on", checkSecuritySection},
		{"project_overview.md environment variable section", checkOverviewSection},
	}
	failed := 0
	for _, check := range checks {
		problems := check.run(base, head)
		if len(problems) == 0 {
			fmt.Printf("OK   %s\n", check.name)
			continue
		}
		failed++
		fmt.Printf("NG   %s: %d difference(s)\n", check.name, len(problems))
		for _, problem := range problems {
			fmt.Printf("     %s\n", problem)
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}

// fail prints an error that stops every check and exits non-zero.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "checks: %s: %v\n", what, err)
	os.Exit(1)
}

// gitOutput runs git with args and returns its standard output without the
// trailing newline.
func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// fileAt returns the content of path at rev.
func fileAt(rev, path string) (string, error) {
	return gitOutput("show", rev+":"+path)
}

// checkReadmeHeadings compares the README's headings outside code blocks,
// ignoring the two headings this task adds.
func checkReadmeHeadings(base, head string) []string {
	before, err := fileAt(base, readmePath)
	if err != nil {
		return []string{err.Error()}
	}
	after, err := fileAt(head, readmePath)
	if err != nil {
		return []string{err.Error()}
	}
	old := headings(before)
	added := []string{quickStartHeading, recommendedConfigHeading}
	got := slices.DeleteFunc(headings(after), func(heading string) bool {
		return slices.Contains(added, heading)
	})
	var problems []string
	for i := range max(len(old), len(got)) {
		switch {
		case i >= len(old):
			problems = append(problems, fmt.Sprintf("heading %d is %q, which the old document does not have", i+1, got[i]))
		case i >= len(got):
			problems = append(problems, fmt.Sprintf("heading %d is missing: %q", i+1, old[i]))
		case old[i] != got[i]:
			problems = append(problems, fmt.Sprintf("heading %d is %q, want %q", i+1, got[i], old[i]))
		}
	}
	return problems
}

// checkEvaluationSection compares the 0008 evaluation record's section 6: every
// existing line must still be there, in order, so only additions are allowed.
// It also compares the three Document Status rows this task may not change.
func checkEvaluationSection(base, head string) []string {
	before, err := fileAt(base, evaluationPath)
	if err != nil {
		return []string{err.Error()}
	}
	after, err := fileAt(head, evaluationPath)
	if err != nil {
		return []string{err.Error()}
	}
	oldSection := sectionFrom(before, evaluationSectionPrefix)
	newSection := sectionFrom(after, evaluationSectionPrefix)
	if oldSection == nil || newSection == nil {
		return []string{"section 6 is missing"}
	}
	var problems []string
	next := 0
	for i, line := range oldSection {
		at := slices.Index(newSection[next:], line)
		if at < 0 {
			problems = append(problems, fmt.Sprintf("section 6 line %d is missing or changed: %q", i+1, line))
			continue
		}
		next += at + 1
	}
	for _, prefix := range []string{"| Status |", "| Review date |", "| Reviewer |"} {
		oldRow := rowStartingWith(before, prefix)
		if got := rowStartingWith(after, prefix); got != oldRow {
			problems = append(problems, fmt.Sprintf("Document Status row is %q, want %q", got, oldRow))
		}
	}
	return problems
}

// checkSecuritySection compares security.md from section 1 to the end.
func checkSecuritySection(base, head string) []string {
	return checkSectionUnchanged(base, head, securityPath, securitySectionPrefix)
}

// checkOverviewSection compares project_overview.md's environment variable
// section, from its heading to the next level-two heading.
func checkOverviewSection(base, head string) []string {
	before, err := fileAt(base, overviewPath)
	if err != nil {
		return []string{err.Error()}
	}
	after, err := fileAt(head, overviewPath)
	if err != nil {
		return []string{err.Error()}
	}
	old := overviewSection(before)
	got := overviewSection(after)
	if old == nil || got == nil {
		return []string{fmt.Sprintf("the %q section is missing", overviewHeading)}
	}
	var problems []string
	for i := range max(len(old), len(got)) {
		switch {
		case i >= len(old):
			problems = append(problems, fmt.Sprintf("line %d is added: %q", i+1, got[i]))
		case i >= len(got):
			problems = append(problems, fmt.Sprintf("line %d is missing: %q", i+1, old[i]))
		case old[i] != got[i]:
			problems = append(problems, fmt.Sprintf("line %d is %q, want %q", i+1, got[i], old[i]))
		}
	}
	return problems
}

// checkSectionUnchanged compares the section that starts at the line with
// prefix and runs to the end of the file.
func checkSectionUnchanged(base, head, path, prefix string) []string {
	before, err := fileAt(base, path)
	if err != nil {
		return []string{err.Error()}
	}
	after, err := fileAt(head, path)
	if err != nil {
		return []string{err.Error()}
	}
	old := sectionFrom(before, prefix)
	got := sectionFrom(after, prefix)
	if old == nil || got == nil {
		return []string{fmt.Sprintf("the %q section is missing", prefix)}
	}
	var problems []string
	for i := range max(len(old), len(got)) {
		switch {
		case i >= len(old):
			problems = append(problems, fmt.Sprintf("line %d is added: %q", i+1, got[i]))
		case i >= len(got):
			problems = append(problems, fmt.Sprintf("line %d is missing: %q", i+1, old[i]))
		case old[i] != got[i]:
			problems = append(problems, fmt.Sprintf("line %d is %q, want %q", i+1, got[i], old[i]))
		}
	}
	return problems
}

// headings returns the level-one to level-three heading lines of doc that are
// outside a fenced code block.
func headings(doc string) []string {
	var found []string
	inBlock := false
	for line := range strings.Lines(doc) {
		text := strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(strings.TrimSpace(text), "```") {
			inBlock = !inBlock
			continue
		}
		if inBlock {
			continue
		}
		if level := headingLevel(text); level > 0 && level <= 3 {
			found = append(found, text)
		}
	}
	return found
}

// headingLevel reports the level of a Markdown heading line, or zero when the
// line is not one.
func headingLevel(line string) int {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n == len(line) || (line[n] != ' ' && line[n] != '\t') {
		return 0
	}
	return n
}

// sectionFrom returns the lines from the first line whose text starts with
// prefix to the end of doc, or nil when there is none.
func sectionFrom(doc, prefix string) []string {
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return lines[i:]
		}
	}
	return nil
}

// overviewSection returns the lines from the environment variable heading to
// the next level-two heading, or nil when there is none.
func overviewSection(doc string) []string {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != overviewHeading {
			continue
		}
		start = i
		break
	}
	if start < 0 {
		return nil
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return lines[start:end]
}

// rowStartingWith returns the first line of doc whose text starts with prefix.
func rowStartingWith(doc, prefix string) string {
	for line := range strings.Lines(doc) {
		if text := strings.TrimSuffix(line, "\n"); strings.HasPrefix(strings.TrimSpace(text), prefix) {
			return text
		}
	}
	return ""
}
