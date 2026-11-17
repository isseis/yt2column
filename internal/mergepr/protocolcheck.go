package mergepr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The protocol-number check makes sure a PR that changes a tool's protocol
// number (the number a developer tool compares with the --protocol its command
// documents pass) merges only with origin/main's number plus one, so two
// branches that raised the same number cannot both reach main.
//
// A tool is a directory internal/<name>/ whose .go files (other than
// _test.go) declare Protocol. The check finds the tools in the git trees it
// reads instead of keeping a list, and reads every revision with the same
// rules, so a PR commit, its parents, origin/main and the PR head are compared
// alike.

const (
	// mainRef is the remote-tracking ref of main, spelled out in full so a
	// local branch named origin/main is never read in its place.
	mainRef = "refs/remotes/origin/main"
	// mainBranch is the only base the check can compare against.
	mainBranch = "main"

	// protocolWord is the identifier a tool declares its number with.
	protocolWord = "Protocol"
	// revListFormat prints one line per commit: the commit ID and its parents,
	// separated by spaces, then a NUL and the subject. The ID keeps the line
	// from being empty, which git rev-list would not print at all.
	revListFormat = "--format=%H %P%x00%s"

	// toolDir is the directory whose direct subdirectories are tools.
	toolDir = "internal"
)

// protocolPathspecs limit a read to the .go files directly inside
// internal/<name>/, without _test.go files. glob keeps * from crossing a
// slash, so a nested package such as internal/llm/claude is not read, and top
// makes the paths relative to the top of the work tree from any directory.
var protocolPathspecs = []string{
	":(top,glob)" + toolDir + "/*/*.go",
	":(top,glob,exclude)" + toolDir + "/*/*_test.go",
}

// protocolState is the state of one tool's number at one revision. The zero
// value is not a valid state, so a value never read is not taken as absent.
type protocolState int

const (
	protocolStateInvalid protocolState = iota
	protocolAbsent                     // no declaration line
	protocolPresent                    // exactly one accepted line
	protocolUnreadable                 // a declaration line not accepted, or two
)

// protocolValue is one tool's number at one revision.
type protocolValue struct {
	State protocolState
	Value int // meaningful only when State is protocolPresent
}

// String shows the value as the error messages print it.
func (v protocolValue) String() string {
	switch v.State {
	case protocolAbsent:
		return "none"
	case protocolPresent:
		return strconv.Itoa(v.Value)
	case protocolUnreadable:
		return "unreadable"
	default:
		return "invalid"
	}
}

// sameProtocolValue reports whether a and b are the same number. An
// unreadable value equals no value, not even another unreadable one, so a
// commit next to an unreadable value always counts as a change and an
// unreadable value is never taken as unchanged.
func sameProtocolValue(a, b protocolValue) bool {
	if a.State == protocolUnreadable || b.State == protocolUnreadable {
		return false
	}
	return a.State == b.State && a.Value == b.Value
}

// protocolChange is a PR commit that changed a tool's number: a commit with
// at most one parent whose value differs from its parent's, or a merge commit
// whose value differs from every parent's.
type protocolChange struct {
	Commit  string
	Subject string
	Before  []protocolValue // one per parent; one absent value for a root commit
	After   protocolValue
}

// protocolMismatch is one tool whose head number disagrees with the expected
// value.
type protocolMismatch struct {
	Package string        // the tool's directory name under internal/
	Main    protocolValue // protocolAbsent for a new tool
	Head    int
	Changes []protocolChange
}

// expected is the number the PR head must have: origin/main's number plus
// one, or 1 when origin/main has no number for the tool.
func (m protocolMismatch) expected() int {
	return expectedProtocol(m.Main)
}

func expectedProtocol(main protocolValue) int {
	if main.State == protocolPresent {
		return main.Value + 1
	}
	return 1
}

// offersHistoryRewrite reports whether the error also offers rebuilding the
// PR's history on origin/main: only when the head number equals origin/main's,
// which is how both a raised number absorbed by merging main and a number
// raised and then restored look.
func offersHistoryRewrite(m protocolMismatch) bool {
	return m.Main.State == protocolPresent && m.Head == m.Main.Value
}

// historyRewriteRemedy is the second fix a mismatch error offers when
// offersHistoryRewrite holds for one of its tools.
const historyRewriteRemedy = "A tool above has the same number as origin/main. In that case, rebuilding the PR's history on top of origin/main,\n" +
	"so that no commit changes the number, is another fix, but only when the PR's own changes do not change\n" +
	"the tool's contract: the commit that changed the number came from another branch, or the PR raised the\n" +
	"number and restored it. Otherwise rebuilding the history hides the conflict, and the steps above are the only fix."

// installCommand is the command that reinstalls the tool of the package.
func installCommand(pkg string) string {
	return "make install-" + pkg
}

// protocolMismatchError wraps errProtocolMismatch. It names every tool whose
// head number disagrees.
type protocolMismatchError struct {
	MainOID, HeadOID string
	Mismatches       []protocolMismatch
}

// Error lists each mismatch with the commits that changed the number, then
// the steps that fix it.
func (e *protocolMismatchError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v\ncompared origin/main %s with PR head %s\n", errProtocolMismatch, e.MainOID, e.HeadOID)
	rewrite := false
	for _, m := range e.Mismatches {
		fmt.Fprintf(&b, "tool %s: origin/main %v, head %d, expected %d\n", m.Package, m.Main, m.Head, m.expected())
		for _, c := range m.Changes {
			before := make([]string, len(c.Before))
			for i, v := range c.Before {
				before[i] = v.String()
			}
			fmt.Fprintf(&b, "  changed by %s %q: %s -> %v\n", c.Commit, c.Subject, strings.Join(before, ", "), c.After)
		}
		rewrite = rewrite || offersHistoryRewrite(m)
	}
	b.WriteString("To fix it:\n")
	b.WriteString("  1. Merge origin/main into the PR branch (git fetch origin && git merge origin/main; do not rebase).\n")
	b.WriteString("  2. Set the Protocol constant of each tool above to its expected value.\n")
	b.WriteString("  3. Set the numbers the command documents and guides write for it, --protocol values included, to the same value.\n")
	b.WriteString("  4. Commit and push.\n")
	b.WriteString("  5. Reinstall the tool from the PR branch:")
	for _, m := range e.Mismatches {
		fmt.Fprintf(&b, " %s;", installCommand(m.Package))
	}
	b.WriteString("\n")
	b.WriteString("  6. Merge again: run /runpr again if it was used, otherwise /mergepr.\n")
	if rewrite {
		b.WriteString(historyRewriteRemedy)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Unwrap returns errProtocolMismatch.
func (e *protocolMismatchError) Unwrap() error { return errProtocolMismatch }

// unreadableKind names what could not be read. The zero value is not a valid
// kind.
type unreadableKind int

const (
	_ unreadableKind = iota // the zero value, never a valid kind
	// unreadableFetch: origin/main could not be fetched.
	unreadableFetch
	// unreadableCommits: the PR head, origin/main or a revision's files could
	// not be read.
	unreadableCommits
	// unreadableHeadMissing: a PR commit changed the tool's number, but the PR
	// head has no number for it and no PR commit removed it.
	unreadableHeadMissing
	// unreadableValue: a declaration line at origin/main or the PR head is not
	// the accepted form.
	unreadableValue
	// unreadableDuplicate: a tool has two or more declaration lines at
	// origin/main or the PR head.
	unreadableDuplicate
	// unreadableBase: the PR's base is not main and a PR commit changed a
	// number.
	unreadableBase
	// unreadableHistory: the repository is shallow, or the PR commits and
	// their parents could not be listed.
	unreadableHistory
	// unreadableNoTools: origin/main has no tool at all, which means the
	// reading itself went wrong.
	unreadableNoTools
)

// String describes the kind in the error message.
func (k unreadableKind) String() string {
	switch k {
	case unreadableFetch:
		return "could not fetch origin/main"
	case unreadableCommits:
		return "could not read the PR commits"
	case unreadableHeadMissing:
		return "a PR commit changed a tool's number, but the PR head has no Protocol constant for it in the accepted form"
	case unreadableValue:
		return "a Protocol declaration is not the accepted form `const " + protocolWord + " = <1-999999999>` (no leading 0, no _, no type, no comment)"
	case unreadableDuplicate:
		return "a tool has two or more Protocol declarations"
	case unreadableBase:
		return "the PR's base is not main, and a PR commit changed a protocol number"
	case unreadableHistory:
		return "the repository is shallow, or the PR commits and their parents could not be listed"
	case unreadableNoTools:
		return "found no tool with a Protocol constant on origin/main"
	default:
		return "invalid kind"
	}
}

// protocolUnreadableError wraps errProtocolUnreadable.
type protocolUnreadableError struct {
	Kind   unreadableKind
	Rev    string // the revision, when one applies
	Path   string // the file, when one applies
	Line   string // the line as read, shown with %q
	Detail string
}

// Error names the kind and every part that applies.
func (e *protocolUnreadableError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v: %v", errProtocolUnreadable, e.Kind)
	if e.Rev != "" {
		fmt.Fprintf(&b, "; revision %s", e.Rev)
	}
	if e.Path != "" {
		fmt.Fprintf(&b, "; file %s", e.Path)
	}
	if e.Line != "" {
		fmt.Fprintf(&b, "; line %q", e.Line)
	}
	if e.Detail != "" {
		fmt.Fprintf(&b, "; %s", e.Detail)
	}
	return b.String()
}

// Unwrap returns errProtocolUnreadable.
func (e *protocolUnreadableError) Unwrap() error { return errProtocolUnreadable }

// protocolSummary is what Prepare and Merge print on a pass.
type protocolSummary struct {
	MainOID, HeadOID string
	MainTools        map[string]protocolValue
	Changed          []string
}

// String is the one line Prepare and Merge report on a pass: the commits
// compared, each tool origin/main has with its number, and the tools the PR
// changed, so what the check saw can be traced from the report after the
// merge.
func (s protocolSummary) String() string {
	tools := make([]string, 0, len(s.MainTools))
	for _, pkg := range slices.Sorted(maps.Keys(s.MainTools)) {
		tools = append(tools, pkg+" "+s.MainTools[pkg].String())
	}
	changed := "none"
	if len(s.Changed) > 0 {
		changed = strings.Join(s.Changed, ", ")
	}
	return fmt.Sprintf("passed; origin/main %s, head %s; origin/main tools: %s; changed: %s", s.MainOID, s.HeadOID, strings.Join(tools, ", "), changed)
}

// protocolLine is one line of a .go file that holds the word Protocol.
type protocolLine struct {
	Path string // relative to the top of the work tree, slash-separated
	Text string
}

// protocolReading is one tool's value at one revision, with the line that
// made it unreadable.
type protocolReading struct {
	value protocolValue
	kind  unreadableKind // unreadableValue or unreadableDuplicate when unreadable
	path  string
	line  string
}

// valueOf returns the tool's value in readings, absent when it has none.
func valueOf(readings map[string]protocolReading, pkg string) protocolValue {
	if r, ok := readings[pkg]; ok {
		return r.value
	}
	return protocolValue{State: protocolAbsent}
}

// classifyProtocolLines turns the lines one revision holds into each tool's
// value. Contract: only a line of a file directly inside internal/<name>/
// that is not a _test.go file is accepted as input (any other path is an
// error, never skipped); a tool has a value only when it has exactly one
// declaration line and that line is the accepted form; every other
// declaration line makes the value unreadable. Packages without a
// declaration line are left out, which callers read as absent.
func classifyProtocolLines(lines []protocolLine) (map[string]protocolReading, error) {
	readings := make(map[string]protocolReading)
	for _, l := range lines {
		pkg, ok := toolPackage(l.Path)
		if !ok {
			return nil, &protocolUnreadableError{Kind: unreadableCommits, Path: l.Path, Detail: "a path outside internal/<name>/*.go was read"}
		}
		if !declarationLine.MatchString(l.Text) {
			continue
		}
		prev, seen := readings[pkg]
		switch {
		case seen && prev.kind == unreadableDuplicate:
			// Already unreadable for the strongest reason; keep the first report.
		case seen:
			readings[pkg] = protocolReading{value: protocolValue{State: protocolUnreadable}, kind: unreadableDuplicate, path: l.Path, line: l.Text}
		default:
			if n, ok := acceptedNumber(l.Text); ok {
				readings[pkg] = protocolReading{value: protocolValue{State: protocolPresent, Value: n}, path: l.Path, line: l.Text}
			} else {
				readings[pkg] = protocolReading{value: protocolValue{State: protocolUnreadable}, kind: unreadableValue, path: l.Path, line: l.Text}
			}
		}
	}
	return readings, nil
}

// toolPackage returns the <name> of internal/<name>/<file>.go, and false for
// any other path, a _test.go file included.
func toolPackage(path string) (string, bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != toolDir || parts[1] == "" {
		return "", false
	}
	file := parts[2]
	if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
		return "", false
	}
	return parts[1], true
}

// declarationLine matches a line that looks like it declares Protocol: after
// leading blanks and an optional const keyword (or "const (") comes the word
// Protocol, then an = that is not part of ==, !=, <=, >= or :=. Only
// identifiers, a type, commas, blanks and closed block comments may stand
// between the word and the =, so an = inside a string, a struct tag or a
// comment is never taken: a
// struct field and a composite literal element have no such =, and a short
// variable declaration has :=.
var declarationLine = regexp.MustCompile(`^[ \t]*(?:const(?:[ \t]+|[ \t]*\([ \t]*))?` + protocolWord + "(?:[ \t,](?:[^\"'`/=:!<>]|/\\*.*?\\*/)*)?=(?:[^=]|$)")

// acceptedLine matches the only line form whose number is read: exactly
// "const Protocol = " and a decimal of one to nine digits without a leading
// 0, so adding one cannot overflow.
var acceptedLine = regexp.MustCompile(`^const ` + protocolWord + ` = ([1-9][0-9]{0,8})$`)

// acceptedNumber returns the number of an accepted line.
func acceptedNumber(line string) (int, bool) {
	m := acceptedLine.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// protocolCheck reads protocol numbers from git trees and checks a PR head
// against origin/main. It runs git through its own Runner, built by
// newGitRunner, so the check does not depend on the caller's GIT_*
// environment.
type protocolCheck struct {
	run Runner
}

func newProtocolCheck(run Runner) *protocolCheck {
	return &protocolCheck{run: run}
}

// git runs git with the hardening options before the subcommand, so no
// replace ref, pager, color or configured command changes what is read.
func (c *protocolCheck) git(ctx context.Context, args ...string) ([]byte, error) {
	return c.run.Run(ctx, gitCommand, append(cmdrunner.GitHardening(), args...)...)
}

// readRevision returns each tool's value at rev, a full commit ID. Contract:
// only exit status 1 of git grep that wrote nothing at all (no line matched)
// reads as "no declaration line"; the same status with a message, as a
// missing object gives, and any other failure, and any output not in the
// expected shape, is an error, never an empty result.
func (c *protocolCheck) readRevision(ctx context.Context, rev string) (map[string]protocolReading, error) {
	args := []string{
		"grep", "-z", "--no-line-number", "--text", "-F", "--full-name",
		"--no-color", "--no-column", "--no-textconv", "--no-recurse-submodules",
		"-e", protocolWord, rev, "--",
	}
	out, err := c.git(ctx, append(args, protocolPathspecs...)...)
	if err != nil {
		// git grep exits 1 both when no line matched and when it could not
		// read an object; only the former writes nothing at all.
		if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 && len(exitErr.Stdout) == 0 && len(exitErr.Stderr) == 0 {
			return map[string]protocolReading{}, nil
		}
		return nil, &protocolUnreadableError{Kind: unreadableCommits, Rev: rev, Detail: err.Error()}
	}
	lines, err := parseGrepOutput(out, rev)
	if err != nil {
		return nil, err
	}
	return classifyProtocolLines(lines)
}

// parseGrepOutput reads git grep -z output for one revision: each record is
// "<rev>:<path>\0<text>\n". The text may hold NUL bytes (--text), so it is
// everything after the first NUL.
func parseGrepOutput(out []byte, rev string) ([]protocolLine, error) {
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: git grep succeeded without output", errUnexpectedGitOutput)
	}
	if out[len(out)-1] != '\n' {
		return nil, fmt.Errorf("%w: git grep output does not end with a newline", errUnexpectedGitOutput)
	}
	prefix := rev + ":"
	var lines []protocolLine
	for record := range bytes.SplitSeq(out[:len(out)-1], []byte("\n")) {
		rest, ok := bytes.CutPrefix(record, []byte(prefix))
		if !ok {
			return nil, fmt.Errorf("%w: git grep record %q does not start with %s", errUnexpectedGitOutput, record, prefix)
		}
		path, text, ok := bytes.Cut(rest, []byte{0})
		if !ok {
			return nil, fmt.Errorf("%w: git grep record %q has no path separator", errUnexpectedGitOutput, record)
		}
		lines = append(lines, protocolLine{Path: string(path), Text: string(text)})
	}
	return lines, nil
}

// isObjectID reports whether s is a full SHA-1 or SHA-256 object ID in
// lowercase hexadecimal, so it can be passed to git without being read as an
// option or a revision expression.
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := range len(s) {
		if !strings.ContainsRune("0123456789abcdef", rune(s[i])) {
			return false
		}
	}
	return true
}

// resolveCommit returns the commit ID rev names.
func (c *protocolCheck) resolveCommit(ctx context.Context, rev string) (string, error) {
	out, err := c.git(ctx, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	oid := strings.TrimSuffix(string(out), "\n")
	if !isObjectID(oid) {
		return "", fmt.Errorf("%w: git rev-parse printed %q, not a commit ID", errUnexpectedGitOutput, out)
	}
	return oid, nil
}

// prCommit is one PR commit with its parents and subject.
type prCommit struct {
	oid     string
	parents []string
	subject string
}

// listPRCommits lists the commits reachable from head but not from main,
// oldest first, with their parents.
func (c *protocolCheck) listPRCommits(ctx context.Context, head, main string) ([]prCommit, error) {
	out, err := c.git(ctx, "rev-list", "--reverse", "--no-commit-header", revListFormat, head, "^"+main)
	if err != nil {
		return nil, err
	}
	return parseRevList(out)
}

// parseRevList reads git rev-list output with revListFormat. Any other shape
// is an error, never a shorter list.
func parseRevList(out []byte) ([]prCommit, error) {
	if len(out) == 0 {
		return nil, nil
	}
	text, ok := strings.CutSuffix(string(out), "\n")
	if !ok {
		return nil, fmt.Errorf("%w: git rev-list output does not end with a newline", errUnexpectedGitOutput)
	}
	var commits []prCommit
	for line := range strings.SplitSeq(text, "\n") {
		ids, subject, ok := strings.Cut(line, "\x00")
		if !ok {
			return nil, fmt.Errorf("%w: git rev-list line %q has no subject separator", errUnexpectedGitOutput, line)
		}
		oids := strings.Split(strings.TrimSuffix(ids, " "), " ")
		for _, id := range oids {
			if !isObjectID(id) {
				return nil, fmt.Errorf("%w: git rev-list line %q holds %q, not a commit ID", errUnexpectedGitOutput, line, id)
			}
		}
		commits = append(commits, prCommit{oid: oids[0], parents: oids[1:], subject: subject})
	}
	return commits, nil
}

// check decides whether the PR whose head is headOID and whose base is
// baseRefName may merge. It returns the summary and nil to pass, a
// *protocolMismatchError when a tool's head number is not the expected value,
// and a *protocolUnreadableError when something it needs cannot be read.
// Contract: a git failure or an unreadable value never reads as "absent" or
// "unchanged"; the check stops instead.
func (c *protocolCheck) check(ctx context.Context, headOID, baseRefName string) (protocolSummary, error) {
	mainOID, commits, err := c.history(ctx, headOID)
	if err != nil {
		return protocolSummary{}, err
	}
	readings, err := c.readRevisions(ctx, mainOID, headOID, commits)
	if err != nil {
		return protocolSummary{}, err
	}
	mainReadings, headReadings := readings[mainOID], readings[headOID]
	if err := checkReadable(mainOID, mainReadings, headOID, headReadings); err != nil {
		return protocolSummary{}, err
	}

	// Rule 3: which tools the PR commits changed.
	changes := changedTools(commits, readings)
	summary := protocolSummary{MainOID: mainOID, HeadOID: headOID, MainTools: make(map[string]protocolValue), Changed: slices.Sorted(maps.Keys(changes))}
	for pkg, r := range mainReadings {
		summary.MainTools[pkg] = r.value
	}
	if len(summary.Changed) == 0 {
		return summary, nil
	}
	// Rule 4: only a PR into main can be compared with origin/main.
	if baseRefName != mainBranch {
		return protocolSummary{}, &protocolUnreadableError{Kind: unreadableBase, Detail: "base " + strconv.Quote(baseRefName) + "; changed: " + strings.Join(summary.Changed, ", ")}
	}
	if err := compareChanged(summary, changes, mainReadings, headReadings); err != nil {
		return protocolSummary{}, err
	}
	return summary, nil
}

// history checks that the repository is not shallow (rule 1), resolves
// origin/main and the PR head, and lists the PR commits with their parents
// (rule 2).
func (c *protocolCheck) history(ctx context.Context, headOID string) (string, []prCommit, error) {
	out, err := c.git(ctx, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return "", nil, &protocolUnreadableError{Kind: unreadableHistory, Detail: err.Error()}
	}
	if strings.TrimSuffix(string(out), "\n") != "false" {
		return "", nil, &protocolUnreadableError{Kind: unreadableHistory, Detail: "the repository is shallow; run git fetch --unshallow"}
	}
	mainOID, err := c.resolveCommit(ctx, mainRef)
	if err != nil {
		return "", nil, &protocolUnreadableError{Kind: unreadableCommits, Rev: mainRef, Detail: err.Error()}
	}
	if !isObjectID(headOID) {
		return "", nil, &protocolUnreadableError{Kind: unreadableCommits, Rev: headOID, Detail: "the PR head is not a full commit ID"}
	}
	if _, err := c.resolveCommit(ctx, headOID); err != nil {
		return "", nil, &protocolUnreadableError{Kind: unreadableCommits, Rev: headOID, Detail: err.Error()}
	}
	commits, err := c.listPRCommits(ctx, headOID, mainOID)
	if err != nil {
		return "", nil, &protocolUnreadableError{Kind: unreadableHistory, Rev: headOID, Detail: err.Error()}
	}
	return mainOID, commits, nil
}

// readRevisions reads every tool's value at origin/main, the PR head, each
// PR commit and each parent, by commit ID.
func (c *protocolCheck) readRevisions(ctx context.Context, mainOID, headOID string, commits []prCommit) (map[string]map[string]protocolReading, error) {
	revs := []string{mainOID, headOID}
	for _, pc := range commits {
		revs = append(revs, pc.oid)
		revs = append(revs, pc.parents...)
	}
	readings := make(map[string]map[string]protocolReading)
	for _, rev := range revs {
		if _, done := readings[rev]; done {
			continue
		}
		r, err := c.readRevision(ctx, rev)
		if err != nil {
			if unreadable, ok := errors.AsType[*protocolUnreadableError](err); ok {
				unreadable.Rev = rev
				return nil, unreadable
			}
			return nil, &protocolUnreadableError{Kind: unreadableCommits, Rev: rev, Detail: err.Error()}
		}
		readings[rev] = r
	}
	return readings, nil
}

// checkReadable stops on an unreadable value at origin/main or the PR head,
// which cannot be compared, and on an origin/main without any tool, which
// means the reading went wrong (rule 1).
func checkReadable(mainOID string, mainReadings map[string]protocolReading, headOID string, headReadings map[string]protocolReading) error {
	for _, side := range []struct {
		oid      string
		readings map[string]protocolReading
	}{{mainOID, mainReadings}, {headOID, headReadings}} {
		for _, pkg := range slices.Sorted(maps.Keys(side.readings)) {
			if r := side.readings[pkg]; r.value.State == protocolUnreadable {
				return &protocolUnreadableError{Kind: r.kind, Rev: side.oid, Path: r.path, Line: r.line}
			}
		}
	}
	if len(mainReadings) == 0 {
		return &protocolUnreadableError{Kind: unreadableNoTools, Rev: mainOID}
	}
	return nil
}

// changedTools returns, for each tool a PR commit changed, the commits that
// changed it (rule 3).
func changedTools(commits []prCommit, readings map[string]map[string]protocolReading) map[string][]protocolChange {
	tools := make(map[string]struct{})
	for _, r := range readings {
		for pkg := range r {
			tools[pkg] = struct{}{}
		}
	}
	changes := make(map[string][]protocolChange)
	for _, pc := range commits {
		for pkg := range tools {
			if change, ok := commitChange(pc, pkg, readings); ok {
				changes[pkg] = append(changes[pkg], change)
			}
		}
	}
	return changes
}

// compareChanged applies rules 5 and 6 to the changed tools: a tool a PR
// commit removed is not checked, one that vanished without such a commit
// cannot be read, and every other one must have origin/main's number plus
// one at the head.
func compareChanged(summary protocolSummary, changes map[string][]protocolChange, mainReadings, headReadings map[string]protocolReading) error {
	var mismatches []protocolMismatch
	for _, pkg := range summary.Changed {
		head := valueOf(headReadings, pkg)
		if head.State == protocolAbsent {
			if removedBy(changes[pkg]) {
				continue
			}
			return &protocolUnreadableError{Kind: unreadableHeadMissing, Rev: summary.HeadOID, Detail: "tool " + pkg}
		}
		main := valueOf(mainReadings, pkg)
		if head.Value != expectedProtocol(main) {
			mismatches = append(mismatches, protocolMismatch{Package: pkg, Main: main, Head: head.Value, Changes: changes[pkg]})
		}
	}
	if len(mismatches) > 0 {
		return &protocolMismatchError{MainOID: summary.MainOID, HeadOID: summary.HeadOID, Mismatches: mismatches}
	}
	return nil
}

// commitChange reports whether the PR commit pc changed pkg's number. A
// commit with at most one parent changed it when its value differs from its
// parent's (a root commit's parent value is absent); a merge commit changed
// it only when its value differs from every parent's, so merging main into a
// branch that did not touch the number is not a change.
func commitChange(pc prCommit, pkg string, readings map[string]map[string]protocolReading) (protocolChange, bool) {
	after := valueOf(readings[pc.oid], pkg)
	before := []protocolValue{{State: protocolAbsent}}
	if len(pc.parents) > 0 {
		before = make([]protocolValue, len(pc.parents))
		for i, p := range pc.parents {
			before[i] = valueOf(readings[p], pkg)
		}
	}
	for _, v := range before {
		if sameProtocolValue(after, v) {
			return protocolChange{}, false
		}
	}
	return protocolChange{Commit: pc.oid, Subject: pc.subject, Before: before, After: after}, true
}

// removedBy reports whether one of the changes made the number absent.
func removedBy(changes []protocolChange) bool {
	return slices.ContainsFunc(changes, func(c protocolChange) bool { return c.After.State == protocolAbsent })
}
