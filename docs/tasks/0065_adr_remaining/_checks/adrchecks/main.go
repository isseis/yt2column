//go:build test

// Command adrchecks checks, once per pull request of task 0065, the ADRs the
// task writes and changes, the items it moves between ADRs, and the links it
// adds to docs/dev/project_overview.md:
//
//	go run -tags test ./docs/tasks/0065_adr_remaining/_checks/adrchecks/ -pr <N> <merge-base> <head>
//
// Pass the number of the pull request of the task (1 to 4), the merge-base
// with origin/main (git merge-base origin/main HEAD), and the head of the pull
// request. Every file is read from those commits with git (git show, git
// ls-tree), never from the working tree.
//
// The values the checks compare with are not in this file: they are read from
// the tables of the task's implementation plan at head, each found by the
// HTML comment marker on the line right before it (<!-- adrchecks:NAME -->).
// The tables hold Japanese text, which Go source here does not.
//
//   - adrs: the new ADRs by kebab name, with the pull request that writes each.
//   - history: the ADRs by kebab name, with the tasks their history links to.
//   - moves: the lines moved out of, or rewritten in, existing ADRs.
//   - overview: the policy items of project_overview.md and the ADR each gains.
//   - specs: the documents each new ADR's decision links to.
//   - issues: the issue each new ADR's decision links to.
//
// An ADR whose kebab name is not in the adrs table is an existing ADR, which
// the first pull request already has. An ADR is found by its kebab name, not
// its number, because another task may take a number first.
//
// The checks:
//
//   - history-sources: for each history row whose ADR exists at head, its
//     sixth "##" section (the history) has, for each task of the row, a
//     relative link that resolves to a Markdown document under
//     docs/tasks/<task>_<name>/. Anchors are dropped before resolving.
//   - moved-items: for each moves row of this pull request, the line before the
//     change is in the source ADR at the merge-base and not at head, and for a
//     split or a link row the line after the change is in the source ADR at
//     head. A row of an earlier pull request has its line before the change
//     absent at head. Every line removed between the merge-base and head from
//     the second to the fifth "##" section of a source ADR with a row of this
//     pull request is the line before the change of one of those rows. For
//     each move and split row of this pull request it prints the removed line
//     and the destination ADR's section at the same position, for reading.
//   - move-links: for each pair of a source and a destination of the move and
//     split rows up to this pull request, the source's third "##" section (the
//     decision) has a relative link to the destination ADR.
//   - spec-links: for each specs row, the ADR's decision has a relative link
//     that resolves to each document of the row (anchors dropped).
//   - issue-links: for each issues row, the ADR's decision links to
//     https://github.com/isseis/yt2column/issues/<number>. A row of this or an
//     earlier pull request whose number is not filled in is NG.
//   - overview-items: in project_overview.md, each overview row's item is the
//     top-level bullet at its position in the first "##" section and holds
//     its landmark at both commits; a missing landmark stops the run, so a
//     moved item is not compared with the wrong one. (1) text: the item's
//     line, with the trailing sequence of ADR links removed, is the same at
//     the merge-base and at head. (2) links: the ADRs the trailing links name
//     at head are those they name at the merge-base plus the row's ADR.
//   - claude-md-unchanged: CLAUDE.md is the same at the merge-base and at head.
//
// A value the checks need from an ADR that a later pull request writes is
// PENDING. One that this or an earlier pull request writes and that is missing
// is NG, so an ADR written under a different name is not taken for one not
// written yet.
//
// Each check prints OK, NG, or PENDING with its findings. Exit status: 2 when a
// check cannot be run (wrong arguments, a git command that fails, a table
// whose values do not agree, a document whose structure is not the expected
// one), whatever the other results; otherwise 1 when a check is NG; otherwise
// 0.
//
// Run it from the repository root. make test and CI do not run it, and it is
// not maintained after the task's last merge. The //go:build test first line
// lets it use internal/doctestutil and makes the repository-wide environment
// access check treat the file as test code.
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
	"strconv"
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

const (
	// planPath is the implementation plan whose tables the checks read.
	planPath = "docs/tasks/0065_adr_remaining/03_implementation_plan.md"
	// adrDir is the directory of the ADRs.
	adrDir = "docs/dev/adr"
	// overviewPath is the document whose policy items gain ADR links.
	overviewPath = "docs/dev/project_overview.md"
	// issueURLPrefix is the URL of an issue without its number.
	issueURLPrefix = "https://github.com/isseis/yt2column/issues/"
	// firstPR and lastPR bound the task's pull requests.
	firstPR, lastPR = 1, 4
)

// The positions, counted from 1, of an ADR's "##" sections the checks read.
const (
	decisionSection = 3
	historySection  = 6
	// The sections a moved line may come from: background, decision,
	// rejected alternatives, consequences.
	firstMovableSection = 2
	lastMovableSection  = 5
)

// The kinds of a moves row.
const (
	kindMove  = "move"
	kindSplit = "split"
	kindLink  = "link"
)

var (
	// linkRE matches an inline Markdown link and captures its target.
	linkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)
	// trailingADRLinksRE matches the sequence of ADR links an item ends with,
	// each after one space.
	trailingADRLinksRE = regexp.MustCompile(`(?: \[ADR-[0-9]{4}\]\(adr/[0-9]{4}-[a-z0-9-]+\.md\))+$`)
	// kebabRE matches a kebab name.
	kebabRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	// taskRE matches a task number.
	taskRE = regexp.MustCompile(`^[0-9]{4}$`)
	// issueRE matches a filled-in issue number.
	issueRE = regexp.MustCompile(`^[1-9][0-9]*$`)
	// adrNameRE matches an ADR file name and captures its kebab name.
	adrNameRE = regexp.MustCompile(`^[0-9]{4}-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)
)

var (
	// errStructure reports a document whose structure is not the expected one.
	errStructure = errors.New("unexpected document structure")
	// errTable reports a plan table whose values do not agree.
	errTable = errors.New("plan table mismatch")
)

// moveRow is one row of the moves table.
type moveRow struct {
	pr     int
	source string
	dest   string
	kind   string
	before string
	after  string
}

// historyRow is one row of the history table.
type historyRow struct {
	kebab string
	tasks []string
}

// specRow is one row of the specs table.
type specRow struct {
	kebab string
	docs  []string
}

// issueRow is one row of the issues table; number is "" when it is not filled
// in.
type issueRow struct {
	kebab  string
	number string
}

// overviewRow is one row of the overview table.
type overviewRow struct {
	position int
	landmark string
	adr      string
}

// plan holds the values of the plan's tables.
type plan struct {
	// adrPR is the pull request that writes each new ADR, by kebab name.
	adrPR    map[string]int
	history  []historyRow
	moves    []moveRow
	overview []overviewRow
	specs    []specRow
	issues   []issueRow
}

// prOf returns the pull request that writes the ADR: the adrs table's value
// for a new ADR, and the first pull request for an existing one.
func (p *plan) prOf(kebab string) int {
	if pr, ok := p.adrPR[kebab]; ok {
		return pr
	}
	return firstPR
}

func main() {
	pr := flag.Int("pr", 0, "the task's pull request, 1 to 4")
	flag.Parse()
	if flag.NArg() != 2 || *pr < firstPR || *pr > lastPR {
		fmt.Fprintln(os.Stderr, "usage: adrchecks -pr <1-4> <merge-base> <head>")
		os.Exit(2)
	}
	base, head := flag.Arg(0), flag.Arg(1)
	p, baseADRs, headADRs, err := load(base, head)
	if err != nil {
		fmt.Fprintf(os.Stderr, "adrchecks: %v\n", err)
		os.Exit(2)
	}
	c := checker{plan: p, pr: *pr, base: base, head: head, baseADRs: baseADRs, headADRs: headADRs}
	checks := []struct {
		name string
		run  func() (result, []string, error)
	}{
		{"history-sources", c.historySources},
		{"moved-items", c.movedItems},
		{"move-links", c.moveLinks},
		{"spec-links", c.specLinks},
		{"issue-links", c.issueLinks},
		{"overview-items", c.overviewItems},
		{"claude-md-unchanged", c.claudeMDUnchanged},
	}
	overall := resultOK
	broken := false
	for _, ch := range checks {
		r, findings, err := ch.run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "adrchecks: %s: %v\n", ch.name, err)
			broken = true
			continue
		}
		overall = worse(overall, r)
		fmt.Printf("%-7s %s\n", r, ch.name)
		for _, f := range findings {
			for line := range strings.Lines(f) {
				fmt.Printf("        %s\n", strings.TrimSuffix(line, "\n"))
			}
		}
	}
	switch {
	case broken:
		os.Exit(2)
	case overall == resultNG:
		os.Exit(1)
	}
}

// load reads the plan at head, the ADR files at both commits, and checks that
// the plan's tables agree with each other and with the ADRs at the merge-base.
func load(base, head string) (*plan, map[string][]string, map[string][]string, error) {
	doc, err := show(head, planPath)
	if err != nil {
		return nil, nil, nil, err
	}
	p, err := parsePlan(doc)
	if err != nil {
		return nil, nil, nil, err
	}
	baseADRs, err := adrFiles(base)
	if err != nil {
		return nil, nil, nil, err
	}
	headADRs, err := adrFiles(head)
	if err != nil {
		return nil, nil, nil, err
	}
	known := func(kebab string) bool {
		_, isNew := p.adrPR[kebab]
		_, exists := baseADRs[kebab]
		return isNew || exists
	}
	for _, h := range p.history {
		if !known(h.kebab) {
			return nil, nil, nil, fmt.Errorf("%w: history: %s is neither in the adrs table nor an ADR at the merge-base", errTable, h.kebab)
		}
	}
	for _, m := range p.moves {
		if !known(m.source) {
			return nil, nil, nil, fmt.Errorf("%w: moves: source %s is neither in the adrs table nor an ADR at the merge-base", errTable, m.source)
		}
	}
	return p, baseADRs, headADRs, nil
}

// parsePlan reads the six tables of the plan and checks their values.
func parsePlan(doc string) (*plan, error) {
	lines := doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc)))
	table := func(name string, columns int) ([][]string, error) {
		return markedTable(lines, name, columns)
	}
	p := &plan{adrPR: make(map[string]int)}

	rows, err := table("adrs", 5)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		kebab, err := kebabCell("adrs", r[3])
		if err != nil {
			return nil, err
		}
		pr, err := prCell("adrs", r[4])
		if err != nil {
			return nil, err
		}
		if _, dup := p.adrPR[kebab]; dup {
			return nil, fmt.Errorf("%w: adrs: %s appears twice", errTable, kebab)
		}
		p.adrPR[kebab] = pr
	}
	isNew := func(kebab string) bool {
		_, ok := p.adrPR[kebab]
		return ok
	}

	if rows, err = table("history", 4); err != nil {
		return nil, err
	}
	for _, r := range rows {
		kebab, err := kebabCell("history", r[0])
		if err != nil {
			return nil, err
		}
		tasks := strings.Fields(r[1])
		if len(tasks) == 0 || slices.ContainsFunc(tasks, func(t string) bool { return !taskRE.MatchString(t) }) {
			return nil, fmt.Errorf("%w: history: %s: tasks %q", errTable, kebab, r[1])
		}
		p.history = append(p.history, historyRow{kebab: kebab, tasks: tasks})
	}

	if rows, err = table("moves", 6); err != nil {
		return nil, err
	}
	for _, r := range rows {
		pr, err := prCell("moves", r[0])
		if err != nil {
			return nil, err
		}
		source, err := kebabCell("moves", r[1])
		if err != nil {
			return nil, err
		}
		dest, err := kebabCell("moves", r[2])
		if err != nil {
			return nil, err
		}
		if !isNew(dest) {
			return nil, fmt.Errorf("%w: moves: destination %s is not in the adrs table", errTable, dest)
		}
		if p.adrPR[dest] != pr {
			return nil, fmt.Errorf("%w: moves: a row of pull request %d moves to %s, which pull request %d writes", errTable, pr, dest, p.adrPR[dest])
		}
		m := moveRow{pr: pr, source: source, dest: dest, kind: r[3], before: r[4], after: r[5]}
		switch m.kind {
		case kindMove:
			if m.after != "-" {
				return nil, fmt.Errorf("%w: moves: a move row from %s has a line after the change", errTable, source)
			}
		case kindSplit, kindLink:
		default:
			return nil, fmt.Errorf("%w: moves: unknown kind %q", errTable, m.kind)
		}
		if m.before == "" {
			return nil, fmt.Errorf("%w: moves: a row from %s has no line before the change", errTable, source)
		}
		p.moves = append(p.moves, m)
	}

	if rows, err = table("overview", 3); err != nil {
		return nil, err
	}
	for _, r := range rows {
		position, err := strconv.Atoi(r[0])
		if err != nil || position < 1 {
			return nil, fmt.Errorf("%w: overview: position %q", errTable, r[0])
		}
		adr, err := kebabCell("overview", r[2])
		if err != nil {
			return nil, err
		}
		if !isNew(adr) {
			return nil, fmt.Errorf("%w: overview: %s is not in the adrs table", errTable, adr)
		}
		if r[1] == "" {
			return nil, fmt.Errorf("%w: overview: position %d has no landmark", errTable, position)
		}
		p.overview = append(p.overview, overviewRow{position: position, landmark: r[1], adr: adr})
	}

	if rows, err = table("specs", 2); err != nil {
		return nil, err
	}
	for _, r := range rows {
		kebab, err := kebabCell("specs", r[0])
		if err != nil {
			return nil, err
		}
		if !isNew(kebab) {
			return nil, fmt.Errorf("%w: specs: %s is not in the adrs table", errTable, kebab)
		}
		docs := strings.Fields(r[1])
		if len(docs) == 0 {
			return nil, fmt.Errorf("%w: specs: %s names no document", errTable, kebab)
		}
		p.specs = append(p.specs, specRow{kebab: kebab, docs: docs})
	}

	if rows, err = table("issues", 2); err != nil {
		return nil, err
	}
	for _, r := range rows {
		kebab, err := kebabCell("issues", r[0])
		if err != nil {
			return nil, err
		}
		if !isNew(kebab) {
			return nil, fmt.Errorf("%w: issues: %s is not in the adrs table", errTable, kebab)
		}
		// A cell that is not a number is the placeholder of a number not
		// filled in yet.
		number := strings.TrimPrefix(r[1], "#")
		if !issueRE.MatchString(number) {
			number = ""
		}
		p.issues = append(p.issues, issueRow{kebab: kebab, number: number})
	}
	return p, nil
}

// markedTable returns the body rows of the table on the lines right after the
// marker <!-- adrchecks:name -->, each with exactly columns cells.
func markedTable(lines []doctestutil.DocLine, name string, columns int) ([][]string, error) {
	marker := "<!-- adrchecks:" + name + " -->"
	at := -1
	for i, l := range lines {
		if !l.Block && strings.TrimSpace(l.Text) == marker {
			if at >= 0 {
				return nil, fmt.Errorf("%w: two %s markers", errTable, marker)
			}
			at = i
		}
	}
	if at < 0 {
		return nil, fmt.Errorf("%w: no %s marker", errTable, marker)
	}
	var rows [][]string
	for i := at + 1; i < len(lines) && !lines[i].Block && strings.HasPrefix(strings.TrimSpace(lines[i].Text), "|"); i++ {
		cells, err := tableCells(lines[i].Text)
		if err != nil {
			return nil, fmt.Errorf("%s table: %w", name, err)
		}
		if len(cells) != columns {
			return nil, fmt.Errorf("%w: %s table: a row has %d cells, want %d: %s", errTable, name, len(cells), columns, lines[i].Text)
		}
		rows = append(rows, cells)
	}
	// The first two rows are the header and the delimiter row.
	if len(rows) < 3 {
		return nil, fmt.Errorf("%w: no table with rows right after %s", errTable, marker)
	}
	return rows[2:], nil
}

// tableCells splits a table row into its trimmed cells. A "\|" is part of a
// cell and becomes "|".
func tableCells(row string) ([]string, error) {
	row = strings.TrimSpace(row)
	if !strings.HasPrefix(row, "|") || !strings.HasSuffix(row, "|") || strings.HasSuffix(row, `\|`) || len(row) < 2 {
		return nil, fmt.Errorf("%w: not a table row: %s", errTable, row)
	}
	inner := row[1 : len(row)-1]
	var cells []string
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		switch {
		case inner[i] == '\\' && i+1 < len(inner) && inner[i+1] == '|':
			b.WriteByte('|')
			i++
		case inner[i] == '|':
			cells = append(cells, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteByte(inner[i])
		}
	}
	return append(cells, strings.TrimSpace(b.String())), nil
}

// kebabCell returns the kebab name written as a code span in a cell.
func kebabCell(table, cell string) (string, error) {
	inner, ok := strings.CutPrefix(cell, "`")
	if ok {
		inner, ok = strings.CutSuffix(inner, "`")
	}
	if !ok || !kebabRE.MatchString(inner) {
		return "", fmt.Errorf("%w: %s: %q is not a kebab name in a code span", errTable, table, cell)
	}
	return inner, nil
}

// prCell returns the pull request number in a cell.
func prCell(table, cell string) (int, error) {
	pr, err := strconv.Atoi(cell)
	if err != nil || pr < firstPR || pr > lastPR {
		return 0, fmt.Errorf("%w: %s: pull request %q", errTable, table, cell)
	}
	return pr, nil
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

// adrFiles returns the ADR file names directly under the ADR directory at rev,
// by kebab name. A kebab name with two files keeps both, which findADR
// reports.
func adrFiles(rev string) (map[string][]string, error) {
	out, err := git("ls-tree", "--name-only", rev, adrDir+"/")
	if err != nil {
		return nil, err
	}
	files := make(map[string][]string)
	for line := range strings.Lines(out) {
		name := path.Base(strings.TrimSpace(line))
		if m := adrNameRE.FindStringSubmatch(name); m != nil {
			files[m[1]] = append(files[m[1]], name)
		}
	}
	return files, nil
}

// findADR returns the path of the one file of the ADR with the kebab name, ""
// when there is none, and an error when there are two or more.
func findADR(adrs map[string][]string, kebab string) (string, error) {
	switch files := adrs[kebab]; len(files) {
	case 0:
		return "", nil
	case 1:
		return path.Join(adrDir, files[0]), nil
	default:
		return "", fmt.Errorf("%w: two ADR files named %s: %v", errStructure, kebab, files)
	}
}

// h2Sections returns the "##" sections of doc outside code blocks, in order:
// each one's lines after its heading, up to the next "##" or "#" heading.
func h2Sections(doc string) [][]string {
	var sections [][]string
	for _, l := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc))) {
		level := 0
		if !l.Block {
			level = doctestutil.HeadingLevel(l.Text)
		}
		switch {
		case level == 2:
			sections = append(sections, nil)
		case level == 1:
			if len(sections) > 0 {
				return sections
			}
		case len(sections) > 0:
			sections[len(sections)-1] = append(sections[len(sections)-1], l.Text)
		}
	}
	return sections
}

// adrSection returns the lines of the ADR's "##" section at position (counted
// from 1).
func adrSection(doc, file, rev string, position int) ([]string, error) {
	sections := h2Sections(doc)
	if len(sections) < historySection {
		return nil, fmt.Errorf("%w: %s at %s has %d ## sections, want at least %d", errStructure, file, rev, len(sections), historySection)
	}
	return sections[position-1], nil
}

// resolvedLinks returns the targets of the relative links outside code blocks
// in lines, without their anchors, resolved against dir and cleaned.
func resolvedLinks(lines []string, dir string) []string {
	var targets []string
	for _, line := range doctestutil.ScanDocLines(lines) {
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

// rawLinks returns the targets of the links outside code blocks in lines, as
// written.
func rawLinks(lines []string) []string {
	var targets []string
	for _, line := range doctestutil.ScanDocLines(lines) {
		if line.Block {
			continue
		}
		for _, m := range linkRE.FindAllStringSubmatch(line.Text, -1) {
			targets = append(targets, m[1])
		}
	}
	return targets
}

// checker runs the checks for one pull request.
type checker struct {
	plan               *plan
	pr                 int
	base, head         string
	baseADRs, headADRs map[string][]string
}

// lookup returns the ADR's path and text at head. When the ADR is missing it
// returns "" and the result the missing ADR gives: PENDING when a later pull
// request writes it, NG otherwise.
func (c *checker) lookup(kebab string) (file, doc string, missing result, err error) {
	file, err = findADR(c.headADRs, kebab)
	if err != nil {
		return "", "", 0, err
	}
	if file == "" {
		if c.plan.prOf(kebab) > c.pr {
			return "", "", resultPending, nil
		}
		return "", "", resultNG, nil
	}
	doc, err = show(c.head, file)
	return file, doc, 0, err
}

// missingFinding describes a missing ADR for the result it gives.
func missingFinding(r result, kebab string) string {
	if r == resultPending {
		return "pending: " + kebab
	}
	return "missing: " + kebab
}

// historySources checks that each written ADR's history links to a document
// of each of its tasks.
func (c *checker) historySources() (result, []string, error) {
	r := resultOK
	var findings []string
	for _, h := range c.plan.history {
		file, doc, missing, err := c.lookup(h.kebab)
		if err != nil {
			return 0, nil, err
		}
		if file == "" {
			r = worse(r, missing)
			findings = append(findings, missingFinding(missing, h.kebab))
			continue
		}
		section, err := adrSection(doc, file, c.head, historySection)
		if err != nil {
			return 0, nil, err
		}
		links := resolvedLinks(section, adrDir)
		var absent []string
		for _, task := range h.tasks {
			if !slices.ContainsFunc(links, func(l string) bool { return underTask(l, task) }) {
				absent = append(absent, task)
			}
		}
		if len(absent) > 0 {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s: no history link to task %s", file, strings.Join(absent, ", ")))
			continue
		}
		findings = append(findings, "ok: "+file)
	}
	return r, findings, nil
}

// underTask reports whether p is a Markdown document under a directory of
// the task with the number task: docs/tasks/<task>_<name>/<file>.md.
func underTask(p, task string) bool {
	rest, ok := strings.CutPrefix(p, "docs/tasks/"+task+"_")
	if !ok {
		return false
	}
	dir, file, ok := strings.Cut(rest, "/")
	return ok && dir != "" && strings.HasSuffix(file, ".md")
}

// movable holds an ADR's lines of the movable sections at one commit, trimmed,
// with the position of the section each comes from.
type movable struct {
	lines    []string
	sections []int
}

// movableLines returns the trimmed non-empty lines of the second to the fifth
// "##" sections of the ADR, outside code blocks.
func movableLines(doc, file, rev string) (movable, error) {
	var m movable
	for position := firstMovableSection; position <= lastMovableSection; position++ {
		section, err := adrSection(doc, file, rev, position)
		if err != nil {
			return movable{}, err
		}
		for _, l := range doctestutil.ScanDocLines(section) {
			if t := strings.TrimSpace(l.Text); t != "" && !l.Block {
				m.lines = append(m.lines, t)
				m.sections = append(m.sections, position)
			}
		}
	}
	return m, nil
}

// sectionOf returns the section position of the line in m, or 0.
func (m movable) sectionOf(line string) int {
	if i := slices.Index(m.lines, line); i >= 0 {
		return m.sections[i]
	}
	return 0
}

// removedLines returns the lines of before that after holds fewer times, each
// as often as it is missing.
func removedLines(before, after []string) []string {
	left := make(map[string]int)
	for _, l := range after {
		left[l]++
	}
	var removed []string
	for _, l := range before {
		if left[l] > 0 {
			left[l]--
			continue
		}
		removed = append(removed, l)
	}
	return removed
}

// sourceLines returns a source ADR's movable lines at a commit, or ok false
// when the ADR does not exist there.
func (c *checker) sourceLines(rev string, adrs map[string][]string, kebab string) (movable, bool, error) {
	file, err := findADR(adrs, kebab)
	if err != nil || file == "" {
		return movable{}, false, err
	}
	doc, err := show(rev, file)
	if err != nil {
		return movable{}, false, err
	}
	m, err := movableLines(doc, file, rev)
	return m, err == nil, err
}

// movedItems checks the moves rows against the source ADRs at both commits.
func (c *checker) movedItems() (result, []string, error) {
	r := resultOK
	var findings, shown []string
	// The sources with a row of this pull request, in table order.
	var current []string
	for _, m := range c.plan.moves {
		if m.pr > c.pr {
			r = worse(r, resultPending)
			findings = append(findings, fmt.Sprintf("pending: pull request %d: %s -> %s (%s)", m.pr, m.source, m.dest, m.kind))
			continue
		}
		baseLines, baseOK, err := c.sourceLines(c.base, c.baseADRs, m.source)
		if err != nil {
			return 0, nil, err
		}
		headLines, headOK, err := c.sourceLines(c.head, c.headADRs, m.source)
		if err != nil {
			return 0, nil, err
		}
		if !headOK || (m.pr == c.pr && !baseOK) {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s -> %s (%s): source ADR missing", m.source, m.dest, m.kind))
			continue
		}
		var problems []string
		if slices.Contains(headLines.lines, m.before) {
			problems = append(problems, "the line before the change is still at head")
		}
		if m.pr == c.pr {
			if !slices.Contains(current, m.source) {
				current = append(current, m.source)
			}
			if !slices.Contains(baseLines.lines, m.before) {
				problems = append(problems, "the line before the change is not at the merge-base")
			}
			if m.kind != kindMove && !slices.Contains(headLines.lines, m.after) {
				problems = append(problems, "the line after the change is not at head")
			}
			if m.kind != kindLink {
				text, err := c.sideBySide(m, baseLines.sectionOf(m.before))
				if err != nil {
					return 0, nil, err
				}
				shown = append(shown, text)
			}
		}
		if len(problems) > 0 {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s -> %s (%s, pull request %d): %s: %s", m.source, m.dest, m.kind, m.pr, strings.Join(problems, "; "), m.before))
			continue
		}
		findings = append(findings, fmt.Sprintf("ok: %s -> %s (%s, pull request %d)", m.source, m.dest, m.kind, m.pr))
	}
	for _, source := range current {
		baseLines, _, err := c.sourceLines(c.base, c.baseADRs, source)
		if err != nil {
			return 0, nil, err
		}
		headLines, _, err := c.sourceLines(c.head, c.headADRs, source)
		if err != nil {
			return 0, nil, err
		}
		for _, removed := range removedLines(baseLines.lines, headLines.lines) {
			if !slices.ContainsFunc(c.plan.moves, func(m moveRow) bool {
				return m.pr == c.pr && m.source == source && m.before == removed
			}) {
				r = worse(r, resultNG)
				findings = append(findings, fmt.Sprintf("%s: a removed line is in no row of pull request %d: %s", source, c.pr, removed))
			}
		}
	}
	return r, append(findings, shown...), nil
}

// sideBySide returns, for reading, a moved line and the destination ADR's
// section at the position the line came from.
func (c *checker) sideBySide(m moveRow, position int) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "---- %s -> %s (%s), section %d\n", m.source, m.dest, m.kind, position)
	fmt.Fprintf(&b, "removed: %s\n", m.before)
	if m.kind == kindSplit {
		fmt.Fprintf(&b, "kept:    %s\n", m.after)
	}
	file, doc, _, err := c.lookup(m.dest)
	if err != nil {
		return "", err
	}
	if file == "" || position == 0 {
		b.WriteString("destination: (none)")
		return b.String(), nil
	}
	section, err := adrSection(doc, file, c.head, position)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "destination %s, section %d:\n", file, position)
	for _, l := range section {
		if strings.TrimSpace(l) != "" {
			fmt.Fprintf(&b, "  %s\n", l)
		}
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// moveLinks checks that each source of the move and split rows up to this
// pull request links from its decision to the destination.
func (c *checker) moveLinks() (result, []string, error) {
	r := resultOK
	var findings []string
	type pair struct{ source, dest string }
	var seen []pair
	for _, m := range c.plan.moves {
		if m.kind == kindLink || slices.Contains(seen, pair{m.source, m.dest}) {
			continue
		}
		seen = append(seen, pair{m.source, m.dest})
		if m.pr > c.pr {
			r = worse(r, resultPending)
			findings = append(findings, fmt.Sprintf("pending: %s -> %s", m.source, m.dest))
			continue
		}
		destFile, err := findADR(c.headADRs, m.dest)
		if err != nil {
			return 0, nil, err
		}
		file, doc, _, err := c.lookup(m.source)
		if err != nil {
			return 0, nil, err
		}
		if file == "" || destFile == "" {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s -> %s: ADR missing", m.source, m.dest))
			continue
		}
		section, err := adrSection(doc, file, c.head, decisionSection)
		if err != nil {
			return 0, nil, err
		}
		if !slices.Contains(resolvedLinks(section, adrDir), destFile) {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s: the decision has no link to %s", file, destFile))
			continue
		}
		findings = append(findings, fmt.Sprintf("ok: %s -> %s", file, destFile))
	}
	return r, findings, nil
}

// specLinks checks that each written ADR's decision links to its documents.
func (c *checker) specLinks() (result, []string, error) {
	r := resultOK
	var findings []string
	for _, s := range c.plan.specs {
		file, doc, missing, err := c.lookup(s.kebab)
		if err != nil {
			return 0, nil, err
		}
		if file == "" {
			r = worse(r, missing)
			findings = append(findings, missingFinding(missing, s.kebab))
			continue
		}
		section, err := adrSection(doc, file, c.head, decisionSection)
		if err != nil {
			return 0, nil, err
		}
		links := resolvedLinks(section, adrDir)
		var absent []string
		for _, d := range s.docs {
			if !slices.Contains(links, d) {
				absent = append(absent, d)
			}
		}
		if len(absent) > 0 {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s: the decision has no link to %s", file, strings.Join(absent, ", ")))
			continue
		}
		findings = append(findings, "ok: "+file)
	}
	return r, findings, nil
}

// issueLinks checks that each written ADR's decision links to its issue.
func (c *checker) issueLinks() (result, []string, error) {
	r := resultOK
	var findings []string
	for _, is := range c.plan.issues {
		if is.number == "" {
			if c.plan.prOf(is.kebab) > c.pr {
				r = worse(r, resultPending)
				findings = append(findings, "pending: "+is.kebab)
			} else {
				r = worse(r, resultNG)
				findings = append(findings, is.kebab+": the issue number is not filled in")
			}
			continue
		}
		file, doc, missing, err := c.lookup(is.kebab)
		if err != nil {
			return 0, nil, err
		}
		if file == "" {
			r = worse(r, missing)
			findings = append(findings, missingFinding(missing, is.kebab))
			continue
		}
		section, err := adrSection(doc, file, c.head, decisionSection)
		if err != nil {
			return 0, nil, err
		}
		want := issueURLPrefix + is.number
		if !slices.Contains(rawLinks(section), want) {
			r = worse(r, resultNG)
			findings = append(findings, fmt.Sprintf("%s: the decision has no link to %s", file, want))
			continue
		}
		findings = append(findings, "ok: "+file)
	}
	return r, findings, nil
}

// overviewBullets returns the top-level bullet lines of the first "##" section
// of project_overview.md at rev, outside code blocks.
func overviewBullets(rev string) ([]string, error) {
	doc, err := show(rev, overviewPath)
	if err != nil {
		return nil, err
	}
	sections := h2Sections(doc)
	if len(sections) == 0 {
		return nil, fmt.Errorf("%w: project_overview.md at %s has no ## section", errStructure, rev)
	}
	var bullets []string
	for _, line := range doctestutil.ScanDocLines(sections[0]) {
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

// trailingADRs returns the sorted, de-duplicated ADR paths the trailing links
// of an item name.
func trailingADRs(item string) []string {
	links := resolvedLinks([]string{trailingADRLinksRE.FindString(item)}, "docs/dev")
	slices.Sort(links)
	return slices.Compact(links)
}

// overviewItems checks that the policy items keep their text and gain the
// links to their ADRs.
func (c *checker) overviewItems() (result, []string, error) {
	baseBullets, err := overviewBullets(c.base)
	if err != nil {
		return 0, nil, err
	}
	headBullets, err := overviewBullets(c.head)
	if err != nil {
		return 0, nil, err
	}
	textResult, linkResult := resultOK, resultOK
	var findings []string
	for _, it := range c.plan.overview {
		before, err := overviewItem(baseBullets, c.base, it.position, it.landmark)
		if err != nil {
			return 0, nil, err
		}
		after, err := overviewItem(headBullets, c.head, it.position, it.landmark)
		if err != nil {
			return 0, nil, err
		}
		if trailingADRLinksRE.ReplaceAllString(before, "") != trailingADRLinksRE.ReplaceAllString(after, "") {
			textResult = resultNG
			findings = append(findings, fmt.Sprintf("text: bullet %d (%s) changed", it.position, it.landmark))
		}

		want := trailingADRs(before)
		file, err := findADR(c.headADRs, it.adr)
		if err != nil {
			return 0, nil, err
		}
		pending := false
		switch {
		case file != "":
			want = append(want, file)
			slices.Sort(want)
			want = slices.Compact(want)
		case c.plan.prOf(it.adr) > c.pr:
			pending = true
		default:
			linkResult = resultNG
			findings = append(findings, fmt.Sprintf("links: bullet %d (%s): %s missing", it.position, it.landmark, it.adr))
			continue
		}
		linked := trailingADRs(after)
		switch {
		case !slices.Equal(linked, want):
			linkResult = resultNG
			findings = append(findings, fmt.Sprintf("links: bullet %d (%s) links to %v, want %v", it.position, it.landmark, linked, want))
		case pending:
			linkResult = worse(linkResult, resultPending)
			findings = append(findings, fmt.Sprintf("links: bullet %d (%s) pending: %s", it.position, it.landmark, it.adr))
		}
	}
	findings = append([]string{"text: " + textResult.String(), "links: " + linkResult.String()}, findings...)
	return worse(textResult, linkResult), findings, nil
}

// claudeMDUnchanged checks that CLAUDE.md is the same at both commits.
func (c *checker) claudeMDUnchanged() (result, []string, error) {
	before, err := show(c.base, "CLAUDE.md")
	if err != nil {
		return 0, nil, err
	}
	after, err := show(c.head, "CLAUDE.md")
	if err != nil {
		return 0, nil, err
	}
	if before != after {
		return resultNG, []string{"CLAUDE.md changed"}, nil
	}
	return resultOK, nil, nil
}
