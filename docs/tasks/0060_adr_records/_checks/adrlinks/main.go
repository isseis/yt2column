//go:build test

// Command adrlinks checks, once per pull request of task 0060, the links the
// task adds to CLAUDE.md, the ADRs, and docs/dev/project_overview.md:
//
//	go run -tags test ./docs/tasks/0060_adr_records/_checks/adrlinks/ [-final] <merge-base> <head>
//
// Pass the merge-base with origin/main (git merge-base origin/main HEAD) and
// the head of the pull request. Every file is read from those commits with git
// (git show, git ls-tree), never from the working tree.
//
// The checks:
//
//   - quick-links: the "## Quick Links" section of CLAUDE.md at head has a
//     relative link to docs/dev/adr/README.md and one to
//     docs/dev/conventions/adr.md.
//   - history-sources: for each ADR in adrSources that exists at head, its
//     sixth "##" section (the history) has, for each of its source tasks, a
//     relative link that resolves to a file under docs/tasks/<task>_<name>/.
//     Anchors are dropped before resolving. An ADR is found by its kebab name,
//     not its number, because another task may take a number first.
//   - viewer-appendix: the viewer-opening ADR has a relative link that
//     resolves to docs/tasks/0050_viewer_tab/02_architecture.md.
//   - overview-items: in docs/dev/project_overview.md, the second to the fifth
//     top-level bullet of the first "##" section (the four policy items the
//     task links from). (1) text: each item's line, with the trailing sequence
//     of ADR links removed, is the same at the merge-base and at head. (2)
//     links: the files the trailing ADR links of each item name at head are
//     exactly the ADRs of overviewLinks that exist at head. Each item is
//     located by its position and must carry its landmark code span at both
//     commits; a missing landmark stops the run, so a moved item is not
//     compared with the wrong one.
//
// Each check prints OK, NG, or PENDING (an ADR it needs is not written yet)
// with its findings. Exit status: 2 when a check cannot be run (wrong
// arguments, a git command that fails, a document whose structure is not the
// expected one), whatever the other results; otherwise 1 when a check is NG,
// or, with -final, PENDING; otherwise 0.
//
// Run it from the repository root. make test and CI do not run it, and it is
// not maintained after the merge. The //go:build test first line lets it use
// internal/doctestutil and makes the repository-wide environment access check
// treat the file as test code.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// result is the outcome of one check.
type result int

const (
	resultOK result = iota
	resultPending
	resultNG
)

func (r result) String() string {
	switch r {
	case resultOK:
		return "OK"
	case resultPending:
		return "PENDING"
	}
	return "NG"
}

// worse returns the worse of two results: NG over PENDING over OK.
func worse(a, b result) result {
	return max(a, b)
}

// adrDir is the directory of the ADRs at head.
const adrDir = "docs/dev/adr"

// adrSources are the ADRs of the task by kebab name, each with the source
// tasks its history must link to (the tasks of the requirements' table that
// are not in parentheses).
var adrSources = []struct {
	kebab string
	tasks []string
}{
	{"llm-adapter-stdlib", []string{"0003", "0009", "0051"}},
	{"post-generation-steps", []string{"0010", "0013"}},
	{"llm-config-contract", []string{"0009", "0041", "0042", "0045", "0051"}},
	{"production-profile", []string{"0008", "0018", "0040", "0042"}},
	{"server-architecture", []string{"0011", "0020", "0026"}},
	{"threat-model", []string{"0011", "0020", "0026"}},
	{"manual-transcript-copy", []string{"0015", "0024", "0025", "0029"}},
	{"launch-path", []string{"0007", "0020", "0027", "0050"}},
	{"viewer-opening", []string{"0050"}},
	{"extension-stores-nothing", []string{"0020", "0026", "0050"}},
	{"webhook-publishing", []string{"0006", "0016", "0028", "0044"}},
	{"review-setup", []string{"0031", "0049"}},
	{"deterministic-checks-in-go", []string{"0030", "0049", "0055"}},
	{"untrusted-review-threads", []string{"0030", "0033", "0036", "0057"}},
	{"document-test-policy", []string{"0012", "0014", "0038", "0052"}},
}

// historySection is the position, counted from 1, of an ADR's history among
// its "##" sections.
const historySection = 6

// viewerADR and viewerAppendix are the ADR that must link to the viewer task's
// architecture, and that architecture's path.
const (
	viewerADR      = "viewer-opening"
	viewerAppendix = "docs/tasks/0050_viewer_tab/02_architecture.md"
)

// overviewItems are the policy items of project_overview.md the task links
// from: the position of the item among the top-level bullets of the first
// "##" section (counted from 1), the code span that marks it, and the ADRs, by
// kebab name, it links to.
var overviewItems = []struct {
	position int
	landmark string
	adrs     []string
}{
	{2, "`cmd/yt2column-nmhost`", []string{"server-architecture", "launch-path"}},
	{3, "`CopiedSource`", []string{"manual-transcript-copy"}},
	{4, "`claude-opus-5-5`", []string{"llm-adapter-stdlib", "production-profile"}},
	{5, "`google.golang.org/genai`", []string{"llm-adapter-stdlib"}},
}

var (
	// linkRE matches an inline Markdown link and captures its target.
	linkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)
	// trailingADRLinksRE matches the sequence of ADR links an item ends with,
	// each after one space.
	trailingADRLinksRE = regexp.MustCompile(`(?: \[ADR-[0-9]{4}\]\(adr/[0-9]{4}-[a-z0-9-]+\.md\))+$`)
)

// errStructure reports a document whose structure is not the expected one.
var errStructure = errors.New("unexpected document structure")

func main() {
	final := flag.Bool("final", false, "treat PENDING as a failure")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: adrlinks [-final] <merge-base> <head>")
		os.Exit(2)
	}
	base, head := flag.Arg(0), flag.Arg(1)
	adrs, err := adrFiles(head)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adrlinks: %v\n", err)
		os.Exit(2)
	}
	checks := []struct {
		name string
		run  func() (result, []string, error)
	}{
		{"quick-links", func() (result, []string, error) { return checkQuickLinks(head) }},
		{"history-sources", func() (result, []string, error) { return checkHistorySources(head, adrs) }},
		{"viewer-appendix", func() (result, []string, error) { return checkViewerAppendix(head, adrs) }},
		{"overview-items", func() (result, []string, error) { return checkOverviewItems(base, head, adrs) }},
	}
	overall := resultOK
	broken := false
	for _, c := range checks {
		r, findings, err := c.run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "adrlinks: %s: %v\n", c.name, err)
			broken = true
			continue
		}
		overall = worse(overall, r)
		fmt.Printf("%-7s %s\n", r, c.name)
		for _, f := range findings {
			fmt.Printf("        %s\n", f)
		}
	}
	switch {
	case broken:
		os.Exit(2)
	case overall == resultNG, overall == resultPending && *final:
		os.Exit(1)
	}
}

// git runs git with args and returns its standard output.
func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// show returns the file at p in rev.
func show(rev, p string) (string, error) {
	return git("show", rev+":"+p)
}

// adrFiles returns the file names directly under the ADR directory at head,
// by kebab name, for the names of the form NNNN-<kebab>.md. A kebab name with
// two files keeps both, which the checks report.
func adrFiles(head string) (map[string][]string, error) {
	out, err := git("ls-tree", "--name-only", head, adrDir+"/")
	if err != nil {
		return nil, err
	}
	nameRE := regexp.MustCompile(`^[0-9]{4}-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)
	files := make(map[string][]string)
	for line := range strings.Lines(out) {
		name := path.Base(strings.TrimSpace(line))
		if m := nameRE.FindStringSubmatch(name); m != nil {
			files[m[1]] = append(files[m[1]], name)
		}
	}
	return files, nil
}

// findADR returns the one file of the ADR with the kebab name, "" when there
// is none, and an error when there are two or more.
func findADR(adrs map[string][]string, kebab string) (string, error) {
	switch files := adrs[kebab]; len(files) {
	case 0:
		return "", nil
	case 1:
		return files[0], nil
	default:
		return "", fmt.Errorf("%w: two ADR files named %s: %v", errStructure, kebab, files)
	}
}

// resolvedLinks returns the targets of the relative links outside code blocks
// in text, without their anchors, resolved against dir and cleaned.
func resolvedLinks(text, dir string) []string {
	var targets []string
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(text))) {
		if line.Block {
			continue
		}
		for _, m := range linkRE.FindAllStringSubmatch(line.Text, -1) {
			target := m[1]
			if strings.HasPrefix(target, "#") || strings.HasPrefix(target, "<") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			file, _, _ := strings.Cut(target, "#")
			targets = append(targets, path.Join(dir, file))
		}
	}
	return targets
}

// checkQuickLinks checks that CLAUDE.md's Quick Links link to the index and
// the convention.
func checkQuickLinks(head string) (result, []string, error) {
	doc, err := show(head, "CLAUDE.md")
	if err != nil {
		return 0, nil, err
	}
	section := doctestutil.SectionOutsideBlocks(doc, "## Quick Links")
	if strings.TrimSpace(section) == "" {
		return 0, nil, fmt.Errorf("%w: CLAUDE.md has no Quick Links section", errStructure)
	}
	links := resolvedLinks(section, ".")
	var findings []string
	for _, want := range []string{adrDir + "/README.md", "docs/dev/conventions/adr.md"} {
		if !slices.Contains(links, want) {
			findings = append(findings, "no link to "+want)
		}
	}
	if len(findings) > 0 {
		return resultNG, findings, nil
	}
	return resultOK, nil, nil
}

// h2Headings returns the "##" heading lines of doc outside code blocks.
func h2Headings(doc string) []string {
	var headings []string
	for _, h := range doctestutil.HeadingsOutsideBlocks(doc) {
		if doctestutil.HeadingLevel(h) == 2 {
			headings = append(headings, h)
		}
	}
	return headings
}

// checkHistorySources checks that each written ADR's history links to a
// document of each of its source tasks.
func checkHistorySources(head string, adrs map[string][]string) (result, []string, error) {
	r := resultOK
	var findings []string
	for _, src := range adrSources {
		file, err := findADR(adrs, src.kebab)
		if err != nil {
			return 0, nil, err
		}
		if file == "" {
			r = worse(r, resultPending)
			findings = append(findings, "pending: "+src.kebab)
			continue
		}
		doc, err := show(head, path.Join(adrDir, file))
		if err != nil {
			return 0, nil, err
		}
		headings := h2Headings(doc)
		if len(headings) < historySection {
			return 0, nil, fmt.Errorf("%w: %s has %d ## sections, want at least %d", errStructure, file, len(headings), historySection)
		}
		links := resolvedLinks(doctestutil.SectionOutsideBlocks(doc, headings[historySection-1]), adrDir)
		var missing []string
		for _, task := range src.tasks {
			if !slices.ContainsFunc(links, func(l string) bool { return underTask(l, task) }) {
				missing = append(missing, task)
			}
		}
		if len(missing) > 0 {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s: no history link to task %s", file, strings.Join(missing, ", ")))
			continue
		}
		findings = append(findings, "ok: "+file)
	}
	return r, findings, nil
}

// underTask reports whether p is a file under a directory of the task with the
// number task: docs/tasks/<task>_<name>/<file>.
func underTask(p, task string) bool {
	rest, ok := strings.CutPrefix(p, "docs/tasks/"+task+"_")
	if !ok {
		return false
	}
	dir, file, ok := strings.Cut(rest, "/")
	return ok && dir != "" && file != ""
}

// checkViewerAppendix checks that the viewer ADR links to the viewer task's
// architecture.
func checkViewerAppendix(head string, adrs map[string][]string) (result, []string, error) {
	file, err := findADR(adrs, viewerADR)
	if err != nil {
		return 0, nil, err
	}
	if file == "" {
		return resultPending, []string{"pending: " + viewerADR}, nil
	}
	doc, err := show(head, path.Join(adrDir, file))
	if err != nil {
		return 0, nil, err
	}
	if !slices.Contains(resolvedLinks(doc, adrDir), viewerAppendix) {
		return resultNG, []string{file + " has no link to " + viewerAppendix}, nil
	}
	return resultOK, nil, nil
}

// overviewBullets returns the top-level bullet lines of the first "##" section
// of project_overview.md at rev, outside code blocks.
func overviewBullets(rev string) ([]string, error) {
	doc, err := show(rev, "docs/dev/project_overview.md")
	if err != nil {
		return nil, err
	}
	headings := h2Headings(doc)
	if len(headings) == 0 {
		return nil, fmt.Errorf("%w: project_overview.md at %s has no ## section", errStructure, rev)
	}
	var bullets []string
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(doctestutil.SectionOutsideBlocks(doc, headings[0])))) {
		if !line.Block && strings.HasPrefix(line.Text, "- ") {
			bullets = append(bullets, line.Text)
		}
	}
	return bullets, nil
}

// overviewItem returns the item at position (counted from 1) of bullets and
// fails when it is missing or does not carry its landmark.
func overviewItem(bullets []string, rev string, position int, landmark string) (string, error) {
	if position > len(bullets) {
		return "", fmt.Errorf("%w: project_overview.md at %s has %d top-level bullets, want at least %d", errStructure, rev, len(bullets), position)
	}
	item := bullets[position-1]
	if !strings.Contains(item, landmark) {
		return "", fmt.Errorf("%w: project_overview.md at %s: bullet %d does not contain %s", errStructure, rev, position, landmark)
	}
	return item, nil
}

// checkOverviewItems checks that the four policy items keep their text and
// link to the ADRs of their subjects that exist at head.
func checkOverviewItems(base, head string, adrs map[string][]string) (result, []string, error) {
	baseBullets, err := overviewBullets(base)
	if err != nil {
		return 0, nil, err
	}
	headBullets, err := overviewBullets(head)
	if err != nil {
		return 0, nil, err
	}
	textResult, linkResult := resultOK, resultOK
	var findings []string
	for _, it := range overviewItems {
		before, err := overviewItem(baseBullets, base, it.position, it.landmark)
		if err != nil {
			return 0, nil, err
		}
		after, err := overviewItem(headBullets, head, it.position, it.landmark)
		if err != nil {
			return 0, nil, err
		}
		if trailingADRLinksRE.ReplaceAllString(before, "") != trailingADRLinksRE.ReplaceAllString(after, "") {
			textResult = resultNG
			findings = append(findings, fmt.Sprintf("text: bullet %d (%s) changed", it.position, it.landmark))
		}

		linked := resolvedLinks(trailingADRLinksRE.FindString(after), "docs/dev")
		var want, pending []string
		for _, kebab := range it.adrs {
			file, err := findADR(adrs, kebab)
			if err != nil {
				return 0, nil, err
			}
			if file == "" {
				pending = append(pending, kebab)
				continue
			}
			want = append(want, path.Join(adrDir, file))
		}
		slices.Sort(linked)
		slices.Sort(want)
		switch {
		case !slices.Equal(linked, want):
			linkResult = resultNG
			findings = append(findings, fmt.Sprintf("links: bullet %d (%s) links to %v, want %v", it.position, it.landmark, linked, want))
		case len(pending) > 0:
			linkResult = worse(linkResult, resultPending)
			findings = append(findings, fmt.Sprintf("links: bullet %d (%s) pending: %s", it.position, it.landmark, strings.Join(pending, ", ")))
		}
	}
	findings = append([]string{"text: " + textResult.String(), "links: " + linkResult.String()}, findings...)
	return worse(textResult, linkResult), findings, nil
}
