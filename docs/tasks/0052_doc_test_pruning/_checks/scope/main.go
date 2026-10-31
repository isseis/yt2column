//go:build test

// Command scope verifies which files one pull request of the document-test
// pruning task may change. It takes the pull request number and two commits,
// the merge-base with main and the head of the pull request, and reads the
// files that differ between them:
//
//	go run -tags test ./docs/tasks/0052_doc_test_pruning/_checks/scope/ -pr 1 <merge-base> <head>
//
// The pull request may change the files its row of the architecture document's
// change table lists, the implementation plan of this task, and any file under
// this task's _checks directory. The three PRs divide the work as follows:
//
//   - PR 1: the testing policy in CLAUDE.md and the two places that write the
//     documentation guard command.
//   - PR 2: the document tests and helpers of cmd/yt2column, and the list of
//     document tests in the Development Notes section of CLAUDE.md.
//   - PR 3: the document tests of cmd/fixprguard, cmd/codexreview,
//     internal/pipeline, and the extension, the same list in CLAUDE.md, plus
//     the two documents of task 0050 that PR 3 may have to change. A change to
//     those two documents is listed but allowed only in PR 3.
//
// Every other file is a reason to fail, including the other documents of this
// task (its requirements, its architecture, and its design handoff). When
// CLAUDE.md changed, every sentence of the section that begins "Document tests
// check values, structure, and executable examples, never prose" at the
// merge-base must still be in the same section at the head, so that the policy
// a pull request adds cannot rewrite or drop the rules already there.
//
// A sentence is compared after its line breaks and runs of white space are
// folded to one space. The section is the bullet with that bold lead and its
// indented continuation lines.
//
// Run it from the repository root, once per pull request. make test and CI do
// not run it, and it is not maintained after the merge. It reads no
// environment variable: the underscore keeps the directory out of go list, and
// the //go:build test first line makes the repository-wide environment access
// check treat the file as test code, so that check does not scan it.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

const (
	planPath     = "docs/tasks/0052_doc_test_pruning/03_implementation_plan.md"
	checksPrefix = "docs/tasks/0052_doc_test_pruning/_checks/"
	claudePath   = "CLAUDE.md"

	// policyLead is the bold lead of the document-test policy section.
	policyLead = "**Document tests check values, structure, and executable examples, never prose.**"
)

// alwaysAllowed are the files every pull request of this task may change.
var alwaysAllowed = []string{planPath, checksPrefix}

// allowedByPR is the set of files each pull request may change, from the
// architecture document's change table. A path ending in a slash allows every
// file under that directory.
var allowedByPR = map[int][]string{
	1: {
		claudePath,
		".claude/commands/_lib/task-pr-handoff.md",
		"docs/dev/guides/fixprguard_guide.md",
	},
	2: {
		claudePath,
		"cmd/yt2column/docs_test.go",
		"cmd/yt2column/readme_structure_test.go",
		"cmd/yt2column/run_test.go",
		"cmd/yt2column/fixpr_docs_test.go",
		"cmd/yt2column/codex_review_docs_test.go",
		"internal/doctestutil/readme.go",
		"internal/config/test_helpers.go",
	},
	3: {
		claudePath,
		"cmd/fixprguard/docs_test.go",
		"cmd/codexreview/docs_test.go",
		"internal/pipeline/pipeline_test.go",
		"extension/test/metadataDocs.test.ts",
	},
}

// viewerTabDocs are the task 0050 documents that PR 3 may change, and only when
// the decision recorded in step 3-1 of the plan changes them.
var viewerTabDocs = []string{
	"docs/tasks/0050_viewer_tab/02_architecture.md",
	"docs/tasks/0050_viewer_tab/03_implementation_plan.md",
}

func main() {
	pr := flag.Int("pr", 0, "pull request number of the task: 1, 2, or 3")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: scope -pr <1|2|3> <merge-base> <head>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *pr < 1 || *pr > 3 || flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	base, head := flag.Arg(0), flag.Arg(1)
	baseID, err := gitOutput("rev-parse", base)
	if err != nil {
		fail("rev-parse", err)
	}
	headID, err := gitOutput("rev-parse", head)
	if err != nil {
		fail("rev-parse", err)
	}
	fmt.Printf("pull request %d\n", *pr)
	fmt.Printf("merge-base   %s\n", baseID)
	fmt.Printf("head         %s\n", headID)

	// --no-renames lists both sides of a rename, so moving a file out of an
	// allowed path cannot hide the change behind its new name.
	changed, err := gitOutput("diff", "--no-renames", "--name-only", baseID, headID)
	if err != nil {
		fail("diff", err)
	}
	failed := false
	allowed := append(append([]string{}, alwaysAllowed...), allowedByPR[*pr]...)
	var wrong, viewer []string
	fmt.Println("---- changed files")
	for _, path := range nonEmptyLines(changed) {
		switch {
		case allowedPath(path, allowed):
			fmt.Printf("OK   %s\n", path)
		case *pr == 3 && slices.Contains(viewerTabDocs, path):
			fmt.Printf("LIST %s (task 0050 document)\n", path)
			viewer = append(viewer, path)
		default:
			fmt.Printf("NG   %s\n", path)
			wrong = append(wrong, path)
		}
	}
	if len(viewer) > 0 {
		fmt.Printf("---- task 0050 documents changed: %d\n", len(viewer))
		for _, path := range viewer {
			fmt.Println(path)
		}
	}
	if len(wrong) > 0 {
		failed = true
		fmt.Printf("NG   files outside the pull request's change set: %d\n", len(wrong))
		for _, path := range wrong {
			fmt.Println(path)
		}
	} else {
		fmt.Println("OK   every changed file is in the pull request's change set")
	}

	if slices.Contains(nonEmptyLines(changed), claudePath) {
		if problems := checkPolicy(baseID, headID); len(problems) > 0 {
			failed = true
			fmt.Printf("NG   CLAUDE.md policy section: %d sentence(s) lost\n", len(problems))
			for _, p := range problems {
				fmt.Printf("     %s\n", p)
			}
		} else {
			fmt.Println("OK   every sentence of the CLAUDE.md policy section is still there")
		}
	}
	if failed {
		os.Exit(1)
	}
}

// allowedPath reports whether path is one of the allowed files or lies under an
// allowed directory.
func allowedPath(path string, allowed []string) bool {
	for _, a := range allowed {
		if strings.HasSuffix(a, "/") {
			if strings.HasPrefix(path, a) {
				return true
			}
			continue
		}
		if path == a {
			return true
		}
	}
	return false
}

// nonEmptyLines splits output into its non-empty lines.
func nonEmptyLines(output string) []string {
	var out []string
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// checkPolicy compares the document-test policy section at base and at head and
// returns the sentences of the base section that the head section lacks.
func checkPolicy(base, head string) []string {
	before, err := gitOutput("show", base+":"+claudePath)
	if err != nil {
		fail("read CLAUDE.md at the merge-base", err)
	}
	after, err := gitOutput("show", head+":"+claudePath)
	if err != nil {
		fail("read CLAUDE.md at the head", err)
	}
	oldSection, ok := policySection(before)
	if !ok {
		fail("find the policy section at the merge-base", errNoSection)
	}
	newSection, ok := policySection(after)
	if !ok {
		return []string{"the section itself is gone"}
	}
	// Compare whole sentences: a sentence that gained a prefix is not the
	// sentence that was there, even though it still contains it.
	newSentences := sentences(normalize(newSection))
	var missing []string
	for _, sentence := range sentences(normalize(oldSection)) {
		if !slices.Contains(newSentences, sentence) {
			missing = append(missing, truncate(sentence))
		}
	}
	return missing
}

// errNoSection reports a policy section the program cannot find.
var errNoSection = fmt.Errorf("no bullet with the document-test policy lead")

// policySection returns the policy section of a CLAUDE.md text: the bullet with
// the policy lead and its indented continuation lines.
func policySection(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "- ") && strings.Contains(line, policyLead) {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := start + 1
	for end < len(lines) {
		line := lines[end]
		if strings.TrimSpace(line) == "" {
			end++
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		end++
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n")), true
}

// spaceRE matches a run of white space, line breaks included.
var spaceRE = regexp.MustCompile(`\s+`)

// normalize folds each run of white space to one space.
func normalize(text string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(text, " "))
}

// sentenceEndRE matches the end of a sentence: a period, a question mark, or an
// exclamation mark, optional closing characters, white space, and the start of
// the next sentence. Splitting too often only makes shorter sentences, which
// are still looked for as text, so the rule errs that way.
var sentenceEndRE = regexp.MustCompile("[.?!][)\"'*]* +[A-Z`*(\"0-9]")

// sentences splits a normalized text into sentences.
func sentences(text string) []string {
	var out []string
	for {
		loc := sentenceEndRE.FindStringIndex(text)
		if loc == nil {
			break
		}
		end := loc[1] - 1 // keep the next sentence's first character
		out = append(out, strings.TrimSpace(text[:end]))
		text = text[end:]
	}
	if s := strings.TrimSpace(text); s != "" {
		out = append(out, s)
	}
	return out
}

// truncate shortens a sentence for a problem line.
func truncate(text string) string {
	if len(text) > 70 {
		return text[:70] + "..."
	}
	return text
}

// fail prints an error that stops every check and exits non-zero.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "scope: %s: %v\n", what, err)
	os.Exit(1)
}

// gitOutput runs git with args and returns its standard output without the
// trailing newline.
func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
