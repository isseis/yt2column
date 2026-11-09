//go:build test

package main

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// correctionLead starts every correction line. A line that starts with it but
// is not well formed is a violation, so a malformed line cannot hide among the
// well-formed ones.
const (
	correctionLead   = "Editorial correction: "
	correctionSuffix = "; no decision changed"
)

// The kinds of violation check reports, one per check.
const (
	kindData         = "data"
	kindScope        = "scope"
	kindStatus       = "status"
	kindTitle        = "title"
	kindIndex        = "index"
	kindLinks        = "links"
	kindLines        = "lines"
	kindHistoryOrder = "history-order"
	kindHeadings     = "headings"
	kindCorrections  = "corrections"
	kindAllowance    = "allowance"
)

var (
	inlineLinkRE = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)
	autolinkRE   = regexp.MustCompile(`<(https?://[^>\s]+)>`)
	bareURLRE    = regexp.MustCompile(`https?://[A-Za-z0-9._~:/?#@!$&'*+,;=%-]+`)
	codeSpanRE   = regexp.MustCompile("`[^`]*`")
	// unreadableRE matches the link forms the extraction does not read: a
	// reference link, a reference definition, an autolink the autolink form
	// does not match, and an inline link the inline form does not match.
	unreadableRE   = regexp.MustCompile(`\]\(|\]\[|^\s*\[[^\]]*\]:|<https?:`)
	historyDateRE  = regexp.MustCompile(`^\s*[-*]\s+([0-9]{4}-[0-9]{2}-[0-9]{2})`)
	indexRowADRRE  = regexp.MustCompile(`\[ADR-([0-9]{4})\]`)
	urlTrailPunct  = ".,;:!?'"
	indexTitleCell = 2 // cells[0] is the empty text before the first '|'
)

// violation is one failed check.
type violation struct {
	kind string
	file string
	text string
}

// report collects violations and the notes on allowed and added changes,
// which check prints for the PR review.
type report struct {
	violations []violation
	notes      []string
}

func (r *report) fail(kind, file, format string, args ...any) {
	r.violations = append(r.violations, violation{kind, file, fmt.Sprintf(format, args...)})
}

func (r *report) note(format string, args ...any) {
	r.notes = append(r.notes, fmt.Sprintf(format, args...))
}

// count returns how many violations of a kind the report holds.
func (r *report) count(kind string) int {
	n := 0
	for _, v := range r.violations {
		if v.kind == kind {
			n++
		}
	}
	return n
}

// checkRun holds what every check of one run reads.
type checkRun struct {
	base, head string
	name       string
	data       checkData
	group      group
	baseDocs   map[string]doc
	headDocs   map[string]doc
	r          *report
}

func runCheck(name string, args []string) (int, error) {
	var baseRev, headRev string
	switch len(args) {
	case 0:
		out, err := git("merge-base", "origin/main", "HEAD")
		if err != nil {
			return 0, err
		}
		baseRev, headRev = strings.TrimSpace(out), "HEAD"
	case 2:
		baseRev, headRev = args[0], args[1]
	default:
		return 0, errUsage
	}
	base, err := resolve(baseRev)
	if err != nil {
		return 0, err
	}
	head, err := resolve(headRev)
	if err != nil {
		return 0, err
	}
	fmt.Printf("group %s\nbase %s\nhead %s\n", name, base, head)
	ok, err := isAncestor(base, head)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errors.New("the base commit is not an ancestor of the head commit")
	}
	d, err := loadData(head)
	if err != nil {
		return 0, err
	}
	g, err := groupFor(d, name)
	if err != nil {
		return 0, err
	}
	if err := printBlobs(head); err != nil {
		return 0, err
	}

	c := &checkRun{base: base, head: head, name: name, data: d, group: g, baseDocs: map[string]doc{}, headDocs: map[string]doc{}, r: &report{}}
	for _, p := range uniqueADRPaths(g.reviewed()) {
		if c.baseDocs[p], err = readDoc(base, p); err != nil {
			return 0, err
		}
		if c.headDocs[p], err = readDoc(head, p); err != nil {
			return 0, err
		}
	}

	checks := []struct {
		kinds []string
		desc  string
		run   func() error
	}{
		{[]string{kindData}, "the groups cover ADR-0001 to ADR-0038 once each, the group holds at most 8 ADRs, carried ADRs come from earlier groups, and every allowance names a reviewed ADR", c.checkData},
		{[]string{kindScope}, "every changed file is a reviewed ADR, the index, or the group's files under the task directory", c.checkScope},
		{[]string{kindStatus}, "each reviewed ADR's status section is byte-identical", c.checkStatus},
		{[]string{kindTitle}, "each reviewed ADR's first line is unchanged unless the data retitles it", c.checkTitle},
		{[]string{kindIndex}, "the index is unchanged except the title cell of a retitled ADR", c.checkIndex},
		{[]string{kindLinks}, "each reviewed ADR keeps every link target of the base unless the data changes it", c.checkLinks},
		{[]string{kindLines, kindHistoryOrder}, "each reviewed ADR keeps every non-empty line of its alternatives, consequences, and history sections, and the PR adds no date inversion to its history", c.checkLines},
		{[]string{kindHeadings}, "each reviewed ADR keeps every level-2 and level-3 heading of the base", c.checkHeadings},
		{[]string{kindCorrections}, "each non-merge commit that changes reviewed ADRs or the index changes one ADR at most and carries a well-formed correction line for each file", c.checkCorrections},
	}
	for _, ch := range checks {
		before := len(c.r.violations)
		if err := ch.run(); err != nil {
			return 0, fmt.Errorf("%s: %w", strings.Join(ch.kinds, "/"), err)
		}
		status := "OK  "
		if len(c.r.violations) > before {
			status = "FAIL"
		}
		fmt.Printf("%s %s: %s\n", status, strings.Join(ch.kinds, "/"), ch.desc)
	}
	allowance := c.r.count(kindAllowance)
	if allowance > 0 {
		fmt.Printf("FAIL %s: %d allowance(s) in the data match no change\n", kindAllowance, allowance)
	} else {
		fmt.Printf("OK   %s: every allowance in the data matches a change\n", kindAllowance)
	}
	for _, n := range c.r.notes {
		fmt.Println(n)
	}
	if len(c.r.violations) > 0 {
		for _, v := range c.r.violations {
			fmt.Printf("FAIL %s %s: %s\n", v.kind, v.file, v.text)
		}
		fmt.Printf("FAIL %d violation(s)\n", len(c.r.violations))
		return 1, nil
	}
	fmt.Println("OK every check passed")
	return 0, nil
}

// printBlobs prints the blob IDs of the data file and the Go files of the
// check at the head, so a recorded result names the data and the program
// that produced it.
func printBlobs(head string) error {
	files, err := gitLines("ls-tree", "--name-only", "--full-tree", head, checkDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f != dataPath && !strings.HasSuffix(f, ".go") {
			continue
		}
		blob, err := git("rev-parse", "--verify", head+":"+f)
		if err != nil {
			return err
		}
		fmt.Printf("blob %s %s\n", strings.TrimSpace(blob), f)
	}
	return nil
}

// uniqueADRPaths returns the well-formed ADR paths of paths, each once, in
// order. A malformed or repeated path is a data violation reported by
// checkData.
func uniqueADRPaths(paths []string) []string {
	var out []string
	for _, p := range paths {
		if adrNumber(p) != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// checkData verifies the data values: the five first-review groups cover
// ADR-0001 to ADR-0038 once each, a later group reviews nothing for the first
// time, the run group reviews at most eight ADRs, an ADR carried in was first
// reviewed by an earlier group, and every allowance names a reviewed ADR.
func (c *checkRun) checkData() error {
	r, d := c.r, c.data
	headADRs, err := gitLines("ls-tree", "--name-only", "--full-tree", c.head, "docs/dev/adr/")
	if err != nil {
		return err
	}
	exists := func(p string) bool { return slices.Contains(headADRs, p) }
	checkPath := func(where, p string) {
		switch {
		case adrNumber(p) == "":
			r.fail(kindData, dataPath, "%s: %q is not an ADR path", where, p)
		case !exists(p):
			r.fail(kindData, dataPath, "%s: %q does not exist at the head", where, p)
		}
	}

	seen := map[string][]string{} // ADR number -> groups that review it first
	names := make([]string, 0, len(d.Groups))
	for n := range d.Groups {
		names = append(names, n)
	}
	slices.SortFunc(names, func(a, b string) int { return groupPosition(a) - groupPosition(b) })
	for i := 1; i <= primaryGroups; i++ {
		if _, ok := d.Groups[fmt.Sprintf("s%d", i)]; !ok {
			r.fail(kindData, dataPath, "group s%d is missing", i)
		}
	}
	for _, n := range names {
		g := d.Groups[n]
		if groupPosition(n) > primaryGroups && len(g.ADRs) > 0 {
			r.fail(kindData, dataPath, "group %s lists ADRs for a first review; only s1 to s%d do", n, primaryGroups)
		}
		for _, p := range g.ADRs {
			checkPath("group "+n+" adrs", p)
			if num := adrNumber(p); num != "" {
				seen[num] = append(seen[num], n)
			}
		}
	}
	for num, gs := range seen {
		if len(gs) > 1 {
			r.fail(kindData, dataPath, "ADR-%s is in the adrs of %d groups: %v", num, len(gs), gs)
		}
		if n, _ := strconv.Atoi(num); n < firstADR || n > lastADR {
			r.fail(kindData, dataPath, "ADR-%s is outside ADR-%04d to ADR-%04d", num, firstADR, lastADR)
		}
	}
	for n := firstADR; n <= lastADR; n++ {
		if _, ok := seen[fmt.Sprintf("%04d", n)]; !ok {
			r.fail(kindData, dataPath, "ADR-%04d is in no group's adrs", n)
		}
	}

	g := c.group
	reviewed := g.reviewed()
	if len(reviewed) > maxGroupADRs {
		r.fail(kindData, dataPath, "group %s reviews %d ADRs, more than %d", c.name, len(reviewed), maxGroupADRs)
	}
	for i, p := range reviewed {
		if slices.Index(reviewed, p) != i {
			r.fail(kindData, dataPath, "group %s names %q twice", c.name, p)
		}
	}
	pos := groupPosition(c.name)
	for _, p := range g.CarriedIn {
		checkPath("group "+c.name+" carried_in", p)
		earlier := false
		for _, gs := range seen[adrNumber(p)] {
			if groupPosition(gs) < pos && slices.Contains(d.Groups[gs].ADRs, p) {
				earlier = true
			}
		}
		if !earlier {
			r.fail(kindData, dataPath, "group %s carries in %q, which no earlier group reviewed first", c.name, p)
		}
	}
	inGroup := func(where, p string) {
		if !slices.Contains(reviewed, p) {
			r.fail(kindData, dataPath, "%s names %q, which group %s does not review", where, p, c.name)
		}
	}
	for _, p := range g.RetitledADRs {
		inGroup("retitled_adrs", p)
	}
	for _, ch := range g.LinkChanges {
		inGroup("link_changes", ch.ADR)
		if ch.Before == "" || ch.Reason == "" {
			r.fail(kindData, dataPath, "a link change of %q has no before or no reason", ch.ADR)
		}
	}
	for _, ch := range g.LineChanges {
		inGroup("line_changes", ch.ADR)
		if ch.Before == "" || ch.After == "" || ch.Reason == "" {
			r.fail(kindData, dataPath, "a line change of %q has no before, no after, or no reason", ch.ADR)
		}
	}
	for _, f := range g.TaskDocChanges {
		if !isTaskDoc(f.Path) || f.Reason == "" {
			r.fail(kindData, dataPath, "task_doc_changes names %q: it must be a task document under %s, outside %s, with a reason", f.Path, taskDir, checkDir)
		}
	}
	for _, u := range g.UnrecordedCommits {
		if u.Reason == "" {
			r.fail(kindData, dataPath, "unrecorded commit %s has no reason", u.Commit)
		}
	}
	return nil
}

// checkScope verifies that the range changes only the reviewed ADRs, the
// index, and the group's files under the task directory, and another task
// document of this task or the Go files of the check only when the data says
// why.
func (c *checkRun) checkScope() error {
	changed, err := changedFiles(c.base, c.head)
	if err != nil {
		return err
	}
	allowed := append(c.group.reviewed(), c.data.Index, dataPath, taskDir+"review_"+c.name+".md", taskDir+"03_implementation_plan.md")
	goChanged := false
	docUsed := make([]bool, len(c.group.TaskDocChanges))
	for _, p := range changed {
		if slices.Contains(allowed, p) {
			continue
		}
		if i := slices.IndexFunc(c.group.TaskDocChanges, func(f fileReason) bool { return f.Path == p }); i >= 0 && isTaskDoc(p) {
			docUsed[i] = true
			c.r.note("ALLOW scope %s: %s", p, c.group.TaskDocChanges[i].Reason)
			continue
		}
		if strings.HasPrefix(p, checkDir) && strings.HasSuffix(p, ".go") {
			goChanged = true
			if c.group.CheckerChanges != "" {
				continue
			}
		}
		c.r.fail(kindScope, p, "a file outside the group's scope changed (a wrong base commit mixes in another task's files)")
	}
	if c.group.CheckerChanges != "" {
		if goChanged {
			c.r.note("ALLOW scope %s: the check's Go files changed: %s", checkDir, c.group.CheckerChanges)
		} else {
			c.r.fail(kindAllowance, dataPath, "checker_changes is set, but no Go file under %s changed", checkDir)
		}
	}
	for i, f := range c.group.TaskDocChanges {
		if !docUsed[i] && isTaskDoc(f.Path) { // checkData reports a path that is not a task document
			c.r.fail(kindAllowance, dataPath, "task_doc_changes names %s, but the range does not change it", f.Path)
		}
	}
	if c.r.count(kindScope) > 0 {
		fmt.Println("NOTE changed files:")
		for _, p := range changed {
			fmt.Println("NOTE   " + p)
		}
	}
	return nil
}

// isTaskDoc reports whether a path is a task document of this task: a file
// under the task directory and outside the check's own directory.
func isTaskDoc(p string) bool {
	return strings.HasPrefix(p, taskDir) && !strings.HasPrefix(p, checkDir)
}

// checkStatus verifies that each reviewed ADR's status section is
// byte-identical.
func (c *checkRun) checkStatus() error {
	for _, p := range uniqueADRPaths(c.group.reviewed()) {
		b, err := sectionWithHeading(c.baseDocs[p], c.data.Headings.Status)
		if err != nil {
			return err
		}
		h, err := sectionWithHeading(c.headDocs[p], c.data.Headings.Status)
		if err != nil {
			return err
		}
		if b != h {
			c.r.fail(kindStatus, p, "the status section changed")
		}
	}
	return nil
}

// checkTitle verifies that each reviewed ADR's first line is unchanged, and
// that each ADR the data retitles has a changed first line.
func (c *checkRun) checkTitle() error {
	for _, p := range uniqueADRPaths(c.group.reviewed()) {
		b, h := firstLine(c.baseDocs[p]), firstLine(c.headDocs[p])
		retitled := slices.Contains(c.group.RetitledADRs, p)
		switch {
		case b == h && retitled:
			c.r.fail(kindAllowance, p, "retitled_adrs names the ADR, but its first line did not change")
		case b != h && !retitled:
			c.r.fail(kindTitle, p, "the first line changed from %q to %q", b, h)
		case b != h:
			c.r.note("ALLOW title %s: %q -> %q", p, b, h)
		}
	}
	return nil
}

func firstLine(d doc) string {
	if len(d.lines) == 0 {
		return ""
	}
	return strings.TrimSuffix(d.lines[0], "\n")
}

// checkIndex verifies that the index is unchanged line for line, except the
// title cell of the row of an ADR the data retitles.
func (c *checkRun) checkIndex() error {
	b, err := readDoc(c.base, c.data.Index)
	if err != nil {
		return err
	}
	h, err := readDoc(c.head, c.data.Index)
	if err != nil {
		return err
	}
	if len(b.lines) != len(h.lines) {
		c.r.fail(kindIndex, c.data.Index, "the index has %d lines at the head, %d at the base", len(h.lines), len(b.lines))
	}
	retitled := map[string]bool{}
	for _, p := range c.group.RetitledADRs {
		retitled[adrNumber(p)] = true
	}
	for i := range min(len(b.lines), len(h.lines)) {
		bl, hl := strings.TrimSuffix(b.lines[i], "\n"), strings.TrimSuffix(h.lines[i], "\n")
		if bl == hl {
			continue
		}
		if num, ok := titleCellOnly(bl, hl); ok && retitled[num] {
			c.r.note("ALLOW index %s: the title cell of ADR-%s changed", c.data.Index, num)
			continue
		}
		c.r.fail(kindIndex, c.data.Index, "line %d changed from %q to %q", i+1, bl, hl)
	}
	return nil
}

// titleCellOnly reports whether two index rows of the same ADR differ only in
// the title cell, and returns the ADR number.
func titleCellOnly(before, after string) (string, bool) {
	bm, am := indexRowADRRE.FindStringSubmatch(before), indexRowADRRE.FindStringSubmatch(after)
	if bm == nil || am == nil || bm[1] != am[1] {
		return "", false
	}
	bc, ac := strings.Split(before, "|"), strings.Split(after, "|")
	if len(bc) != len(ac) || len(bc) <= indexTitleCell {
		return "", false
	}
	for i := range bc {
		if i != indexTitleCell && bc[i] != ac[i] {
			return "", false
		}
	}
	return bm[1], true
}

// linkSet holds a document's link targets and the lines whose link form the
// extraction cannot read.
type linkSet struct {
	targets    []string
	unreadable []int
}

// extractLinks returns the link targets outside fenced code blocks and code
// spans: inline links, autolinks, and bare URLs. A relative target resolves to
// a repository-relative path with its anchor dropped; a URL stays as written.
// A link to an anchor in the same document is not a target.
func extractLinks(d doc) linkSet {
	var s linkSet
	add := func(t string) {
		if !slices.Contains(s.targets, t) {
			s.targets = append(s.targets, t)
		}
	}
	for i, l := range d.scanned {
		if l.Block {
			continue
		}
		text := codeSpanRE.ReplaceAllString(l.Text, "")
		for _, m := range inlineLinkRE.FindAllStringSubmatch(text, -1) {
			if t := resolveTarget(d.path, m[2]); t != "" {
				add(t)
			}
		}
		text = inlineLinkRE.ReplaceAllString(text, "")
		for _, m := range autolinkRE.FindAllStringSubmatch(text, -1) {
			add(m[1])
		}
		text = autolinkRE.ReplaceAllString(text, "")
		for _, m := range bareURLRE.FindAllString(text, -1) {
			add(strings.TrimRight(m, urlTrailPunct))
		}
		text = bareURLRE.ReplaceAllString(text, "")
		if unreadableRE.MatchString(text) {
			s.unreadable = append(s.unreadable, i+1)
		}
	}
	return s
}

// resolveTarget returns the target an inline link points at, or "" for an
// anchor in the same document.
func resolveTarget(docPath, target string) string {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return target
	}
	file, _, _ := strings.Cut(target, "#")
	if file == "" {
		return ""
	}
	return path.Clean(path.Join(path.Dir(docPath), file))
}

// checkLinks verifies that each reviewed ADR keeps, at the head, every link
// target it had at the base, except a target the data changes or removes, and
// that every link form is readable.
func (c *checkRun) checkLinks() error {
	used := make([]bool, len(c.group.LinkChanges))
	for _, p := range uniqueADRPaths(c.group.reviewed()) {
		b, h := extractLinks(c.baseDocs[p]), extractLinks(c.headDocs[p])
		for _, ln := range b.unreadable {
			c.r.fail(kindLinks, p, "line %d at the base has a link form the check cannot read", ln)
		}
		for _, ln := range h.unreadable {
			c.r.fail(kindLinks, p, "line %d at the head has a link form the check cannot read", ln)
		}
		for _, t := range b.targets {
			if slices.Contains(h.targets, t) {
				continue
			}
			i := slices.IndexFunc(c.group.LinkChanges, func(ch change) bool { return ch.ADR == p && ch.Before == t })
			switch {
			case i < 0:
				c.r.fail(kindLinks, p, "the link target %q is gone", t)
			case c.group.LinkChanges[i].After != "" && !slices.Contains(h.targets, c.group.LinkChanges[i].After):
				c.r.fail(kindLinks, p, "the link target %q is gone, and its replacement %q is not there", t, c.group.LinkChanges[i].After)
			default:
				used[i] = true
				ch := c.group.LinkChanges[i]
				if ch.After == "" {
					c.r.note("ALLOW links %s: removed %q (%s)", p, t, ch.Reason)
				} else {
					c.r.note("ALLOW links %s: %q -> %q (%s)", p, t, ch.After, ch.Reason)
				}
			}
		}
	}
	for i, ch := range c.group.LinkChanges {
		if !used[i] {
			c.r.fail(kindAllowance, ch.ADR, "link_changes allows %q -> %q, but no such change was made", ch.Before, ch.After)
		}
	}
	return nil
}

// checkLines verifies that each reviewed ADR keeps every non-empty line of
// its alternatives, consequences, and history sections, except a line the
// data replaces, and that the PR puts no history entry out of date order.
// Lines the head adds are printed for the PR review.
func (c *checkRun) checkLines() error {
	used := make([]bool, len(c.group.LineChanges))
	hd := c.data.Headings
	for _, p := range uniqueADRPaths(c.group.reviewed()) {
		for _, heading := range []string{hd.Alternatives, hd.Consequences, hd.History} {
			b, err := sectionLines(c.baseDocs[p], heading)
			if err != nil {
				return err
			}
			h, err := sectionLines(c.headDocs[p], heading)
			if err != nil {
				return err
			}
			// free maps a line's text to the indexes of the head lines with
			// that text that no base line has matched yet, in order.
			free := map[string][]int{}
			for i, l := range h {
				free[l] = append(free[l], i)
			}
			take := func(l string) int {
				i := free[l][0]
				free[l] = free[l][1:]
				return i
			}
			var kept []int // head indexes of the base lines, in base order
			for _, l := range b {
				if len(free[l]) > 0 {
					kept = append(kept, take(l))
					continue
				}
				i := -1
				for j, ch := range c.group.LineChanges {
					if !used[j] && ch.ADR == p && ch.Before == l && len(free[ch.After]) > 0 {
						i = j
						break
					}
				}
				if i >= 0 {
					used[i] = true
					kept = append(kept, take(c.group.LineChanges[i].After))
					c.r.note("ALLOW lines %s %s: %q -> %q (%s)", p, heading, l, c.group.LineChanges[i].After, c.group.LineChanges[i].Reason)
					continue
				}
				c.r.fail(kindLines, p, "a line of %s is gone: %q", heading, l)
			}
			added := make([]bool, len(h))
			for _, idx := range free {
				for _, i := range idx {
					added[i] = true
				}
			}
			for i, l := range h {
				if added[i] {
					c.r.note("ADDED %s %s: %s", p, heading, l)
				}
			}
			if heading == hd.History {
				c.checkHistoryOrder(p, h, kept, added)
			}
		}
	}
	for i, ch := range c.group.LineChanges {
		if !used[i] {
			c.r.fail(kindAllowance, ch.ADR, "line_changes allows %q -> %q, but no such change was made", ch.Before, ch.After)
		}
	}
	return nil
}

// checkHistoryOrder verifies that the PR adds no date inversion to the
// history: the lines kept from the base stay in the base's order, and each
// added entry is dated no earlier than the nearest dated entry above it and no
// later than the nearest dated entry below it. An inversion the base already
// has is left as it is: the history is in the order the decisions reached
// main, which reordering could falsify.
func (c *checkRun) checkHistoryOrder(p string, h []string, kept []int, added []bool) {
	if !slices.IsSorted(kept) {
		c.r.fail(kindHistoryOrder, p, "the history entries kept from the base are not in the base's order")
	}
	date := func(l string) string {
		if m := historyDateRE.FindStringSubmatch(l); m != nil {
			return m[1]
		}
		return ""
	}
	for i, l := range h {
		d := date(l)
		if !added[i] || d == "" {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if prev := date(h[j]); prev != "" {
				if d < prev {
					c.r.fail(kindHistoryOrder, p, "the added entry dated %s comes after one dated %s", d, prev)
				}
				break
			}
		}
		for j := i + 1; j < len(h); j++ {
			if next := date(h[j]); next != "" {
				if d > next {
					c.r.fail(kindHistoryOrder, p, "the added entry dated %s comes before one dated %s", d, next)
				}
				break
			}
		}
	}
}

// sectionLines returns the non-empty lines of a section after its heading,
// without their newlines.
func sectionLines(d doc, heading string) ([]string, error) {
	start, end, err := sectionRange(d, heading)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range d.lines[start+1 : end] {
		if t := strings.TrimSuffix(l, "\n"); strings.TrimSpace(t) != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// checkHeadings verifies that each level-2 and level-3 heading of each
// reviewed ADR at the base is still there at the head, as written.
func (c *checkRun) checkHeadings() error {
	for _, p := range uniqueADRPaths(c.group.reviewed()) {
		left := map[string]int{}
		for _, h := range doctestutil.HeadingsOutsideBlocks(c.headDocs[p].text) {
			left[strings.TrimSpace(h)]++
		}
		for _, h := range doctestutil.HeadingsOutsideBlocks(c.baseDocs[p].text) {
			if lv := doctestutil.HeadingLevel(h); lv != 2 && lv != 3 {
				continue
			}
			t := strings.TrimSpace(h)
			if left[t] > 0 {
				left[t]--
				continue
			}
			c.r.fail(kindHeadings, p, "the heading %q is gone", t)
		}
	}
	return nil
}

// checkCorrections verifies each non-merge commit of the range. A commit that
// changes reviewed ADRs or the index changes at most one ADR and has a
// correction line for each of those files, unless the data lists it as a
// commit without one. Every line starting with the correction lead is well
// formed and names a file the commit changes. A merge commit that takes in
// main carries other tasks' changes and is skipped.
func (c *checkRun) checkCorrections() error {
	commits, err := gitLines("rev-list", "--no-merges", "--reverse", c.base+".."+c.head)
	if err != nil {
		return err
	}
	watched := append(c.group.reviewed(), c.data.Index)
	unrecorded := map[string]int{}
	for i, u := range c.group.UnrecordedCommits {
		full, err := resolve(u.Commit)
		if err != nil {
			c.r.fail(kindAllowance, dataPath, "unrecorded_commits names %q, which is not a commit", u.Commit)
			continue
		}
		unrecorded[full] = i
	}
	used := map[string]bool{}
	for _, commit := range commits {
		paths, err := diffPaths(commit)
		if err != nil {
			return err
		}
		message, err := git("log", "-1", "--format=%B", commit)
		if err != nil {
			return err
		}
		named := map[string]bool{}
		for line := range strings.SplitSeq(message, "\n") {
			if !strings.HasPrefix(line, correctionLead) {
				continue
			}
			p, ok := parseCorrection(line)
			if !ok {
				c.r.fail(kindCorrections, commit, "malformed correction line %q", line)
				continue
			}
			if !slices.Contains(paths, p) {
				c.r.fail(kindCorrections, commit, "the correction line names %s, which the commit does not change", p)
			}
			named[p] = true
		}
		var touched []string
		adrs := 0
		for _, p := range paths {
			if slices.Contains(watched, p) {
				touched = append(touched, p)
				if p != c.data.Index {
					adrs++
				}
			}
		}
		if len(touched) == 0 {
			continue
		}
		if i, ok := unrecorded[commit]; ok {
			used[commit] = true
			c.r.note("ALLOW corrections %s: no correction line (%s)", commit, c.group.UnrecordedCommits[i].Reason)
			continue
		}
		if adrs > 1 {
			c.r.fail(kindCorrections, commit, "the commit changes %d reviewed ADRs, want one", adrs)
		}
		for _, p := range touched {
			if !named[p] {
				c.r.fail(kindCorrections, commit, "the commit changes %s but has no correction line for it", p)
			}
		}
	}
	for full, i := range unrecorded {
		if !used[full] {
			c.r.fail(kindAllowance, dataPath, "unrecorded_commits names %s, which is not a commit of the range that changes a reviewed ADR or the index", c.group.UnrecordedCommits[i].Commit)
		}
	}
	return nil
}

// parseCorrection returns the document path of a well-formed correction line:
// the lead, a path without spaces, ": ", a non-empty description, and the
// fixed suffix, with neither the mention character nor an angle bracket.
func parseCorrection(line string) (string, bool) {
	if strings.ContainsAny(line, "@<") || !strings.HasSuffix(line, correctionSuffix) {
		return "", false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(line, correctionLead), correctionSuffix)
	p, desc, ok := strings.Cut(body, ": ")
	if !ok || p == "" || strings.ContainsAny(p, " \t") || strings.TrimSpace(desc) == "" {
		return "", false
	}
	return p, true
}

// doc is one document at one commit: its lines with their newlines, and the
// code-fence classification of each line.
type doc struct {
	path    string
	commit  string
	text    string
	lines   []string
	scanned []doctestutil.DocLine
}

func readDoc(commit, p string) (doc, error) {
	text, err := git("show", commit+":"+p)
	if err != nil {
		return doc{}, err
	}
	lines := slices.Collect(strings.Lines(text))
	return doc{path: p, commit: commit, text: text, lines: lines, scanned: doctestutil.ScanDocLines(lines)}, nil
}

func (d doc) where() string {
	return d.path + " at " + d.commit
}

// sectionRange returns the line index of the heading and the index just past
// the section's last line. The heading must appear exactly once outside code
// blocks; a missing or repeated heading is a read failure.
func sectionRange(d doc, heading string) (int, int, error) {
	want := strings.TrimSpace(heading)
	count := 0
	for _, h := range doctestutil.HeadingsOutsideBlocks(d.text) {
		if strings.TrimSpace(h) == want {
			count++
		}
	}
	if count != 1 {
		return 0, 0, fmt.Errorf("%s: heading %q appears %d times outside code blocks, want 1", d.where(), heading, count)
	}
	start := slices.IndexFunc(d.scanned, func(l doctestutil.DocLine) bool {
		return !l.Block && strings.TrimSpace(l.Text) == want
	})
	level := doctestutil.HeadingLevel(d.scanned[start].Text)
	end := len(d.lines)
	for i := start + 1; i < len(d.scanned); i++ {
		l := d.scanned[i]
		if lv := doctestutil.HeadingLevel(l.Text); !l.Block && lv > 0 && lv <= level {
			end = i
			break
		}
	}
	return start, end, nil
}

// sectionWithHeading returns the section's text with its heading line as
// written.
func sectionWithHeading(d doc, heading string) (string, error) {
	start, end, err := sectionRange(d, heading)
	if err != nil {
		return "", err
	}
	return strings.Join(d.lines[start:end], ""), nil
}
