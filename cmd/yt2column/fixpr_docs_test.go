//go:build test

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// Paths from cmd/yt2column to the command documents these tests check.
const (
	fixprPath              = "../../.claude/commands/fixpr.md"
	runprPath              = "../../.claude/commands/runpr.md"
	contextPath            = "../../.claude/commands/_context.md"
	handoffPath            = "../../.claude/commands/_lib/task-pr-handoff.md"
	mkreqPath              = "../../.claude/commands/mkreq.md"
	localReviewPath        = "../../.claude/commands/_lib/local-cli-review.md"
	harnessAntigravityPath = "../../.claude/commands/_lib/harness-antigravity.md"
)

// Section headings of fixpr.md and runpr.md the contract tests read.
const (
	fixprConvergence = "## Convergence policy"
	fixprPhase1      = "## Phase 1 — Fetch (this session)"
	fixprPhase2      = "## Phase 2 — Triage (model: default)"
	fixprPhase3      = "## Phase 3 — Fix (model: default)"
	fixprPhase35     = "## Phase 3.5 — Scope check (this session)"
	fixprPhase4      = "## Phase 4 — Build + commit + push (this session)"
	fixprPhase45     = "## Phase 4.5 — Local CLI review of the fix (model: default)"
	fixprPhase5      = "## Phase 5 — Reply + resolve (this session)"
	fixprPhase6      = "## Phase 6 — Wrap: PR description (role: low)"
	runprStep1       = "## 1. Resolve the PR, and create it for a standalone change"
	runprStep3       = "## 3. Wait for CI and the review threads, running fixpr as comments appear"
	runprStep4       = "## 4. Confirm, record, and merge"
)

// Names of the _context.md rows that hold the automated reviewers, the PR
// review request, and the rejected mentions, and of the single row they
// replaced, which must not come back.
const (
	reviewerListRow     = "Automated reviewer list"
	awaitedRow          = "Awaited PR reviewer"
	requestRow          = "PR review request"
	rejectedMentionsRow = "Rejected mentions"
	oldReviewerRow      = "Automated reviewer"
)

// wantReviewers is the automated reviewer list the task decided on, in sorted
// order, since the list's names are sorted before they are compared with it.
// Adding or removing a reviewer changes _context.md and this value in the same
// commit.
var wantReviewers = []string{"coderabbitai", "sourcery-ai"}

// flat joins s's whitespace runs into single spaces and drops the blockquote
// marker of each line, so a phrase matches however the document wraps it,
// inside an agent prompt (a blockquote) too.
func flat(s string) string {
	var words []string
	for line := range strings.Lines(s) {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, ">"); ok {
			line = rest
		}
		words = append(words, strings.Fields(line)...)
	}
	return strings.Join(words, " ")
}

// contextRow returns the row of _context.md whose first cell is name, from
// whichever of its Name/Value tables holds it.
func contextRow(t *testing.T, name string) []string {
	t.Helper()
	for _, table := range doctestutil.MarkdownTables(doctestutil.ReadDoc(t, contextPath)) {
		if len(table) > 0 && len(table[0]) > 0 && table[0][0] == "Name" {
			if row := doctestutil.TableRow(table, name); len(row) >= 2 {
				return row
			}
		}
	}
	t.Fatalf("_context.md has no %q row", name)
	return nil
}

// requireSection returns the section of doc under heading, failing the test
// when it is missing or empty.
func requireSection(t *testing.T, name, doc, heading string) string {
	t.Helper()
	section := doctestutil.SectionByHeading(doc, heading)
	if strings.TrimSpace(section) == "" {
		t.Fatalf("%s has no section %q", name, heading)
	}
	return section
}

// requirePhrases fails t for each phrase the text does not contain, comparing
// with whitespace runs folded.
func requirePhrases(t *testing.T, where, text string, phrases ...string) {
	t.Helper()
	got := flat(text)
	for _, phrase := range phrases {
		if !strings.Contains(got, flat(phrase)) {
			t.Errorf("%s does not say %q", where, phrase)
		}
	}
}

// commitSubjectTemplate is the subject finish commits with, with the PR number
// replaced by the NUMBER placeholder the command documents use.
func commitSubjectTemplate(t *testing.T) string {
	t.Helper()
	const sample = "4242"
	pr, err := fixprguard.ParsePRNumber(sample)
	if err != nil {
		t.Fatalf("ParsePRNumber(%q): %v", sample, err)
	}
	subject := fixprguard.CommitSubject(pr)
	if !strings.Contains(subject, "#"+sample) {
		t.Fatalf("CommitSubject = %q, which does not carry the PR number", subject)
	}
	return strings.Replace(subject, sample, "NUMBER", 1)
}

// TestFixprDocContract checks the values of the guard tool that fixpr.md
// states: the Convergence policy and Phase 4 write the subject finish commits
// with, and Phase 3.5 names the stop a dirty work tree causes.
func TestFixprDocContract(t *testing.T) {
	doc := doctestutil.ReadDoc(t, fixprPath)

	t.Run("round_counting", func(t *testing.T) {
		subject := "`" + commitSubjectTemplate(t) + "`"
		for _, heading := range []string{fixprConvergence, fixprPhase4} {
			requirePhrases(t, "fixpr.md "+heading, requireSection(t, "fixpr.md", doc, heading), subject)
		}
	})

	t.Run("out_of_scope_stop", func(t *testing.T) {
		requirePhrases(t, "fixpr.md Phase 3.5", requireSection(t, "fixpr.md", doc, fixprPhase35),
			"`"+fixprguard.StopDirtyWorkTree+"`",
		)
	})
}

// TestRunprDocContract checks what runpr.md must hold for its trust boundary:
// step 1 runs fixprguard head, then fixprguard interaction-limits, before gh
// pr checkout, the document never queries review threads itself, step 3 names
// the stops fixpr reports, and step 3 runs no unchecked gh api call.
func TestRunprDocContract(t *testing.T) {
	doc := doctestutil.ReadDoc(t, runprPath)
	step3 := requireSection(t, "runpr.md", doc, runprStep3)

	t.Run("head_before_checkout", func(t *testing.T) {
		step1 := requireSection(t, "runpr.md", doc, runprStep1)
		head := strings.Index(step1, "fixprguard head ")
		limits := strings.Index(step1, "fixprguard interaction-limits ")
		checkout := strings.Index(step1, "gh pr checkout")
		if head < 0 || limits < 0 || checkout < 0 || head > limits || limits > checkout {
			t.Errorf("runpr.md step 1 must run fixprguard head (at %d), then fixprguard interaction-limits (at %d), before gh pr checkout (at %d)",
				head, limits, checkout)
		}
	})

	t.Run("fixpr_stops", func(t *testing.T) {
		requirePhrases(t, "runpr.md step 3", step3,
			"`"+fixprguard.StopJudgedSetChanged+"`",
			"`"+fixprguard.StopGitStateChanged+"`",
		)
	})

	t.Run("no_body_fetch", func(t *testing.T) {
		for _, removed := range []string{"reviewThreads", "comments(first:"} {
			if strings.Contains(doc, removed) {
				t.Errorf("runpr.md still queries review threads itself (%q)", removed)
			}
		}
	})

	t.Run("no_unchecked_gh_api", func(t *testing.T) {
		checkGhAPICalls(t, "runpr.md step 3", step3, step3ReadCalls)
		if strings.Contains(step3, "gh pr checks") {
			t.Error("runpr.md step 3 still calls gh pr checks")
		}
	})
}

// bodyMarker is put wherever a fake body can hold it. A filter that prints a
// body, or a part of one, prints it; the filters print no other text that
// could hold it.
const bodyMarker = "zq7mark"

// leftPlaceholder matches angle-bracket placeholders like <number>.
var leftPlaceholder = regexp.MustCompile(`<[a-z][a-z-]*>`)

// step3ReadCalls is empty: runpr.md step 3 reads PR mentions, review state,
// review threads, and CI checks exclusively through fixprguard, with no direct
// gh api call.
var step3ReadCalls []string

// checkGhAPICalls fails t for a gh api call in section that does not start
// with one of allowed, so a read that prints whole responses cannot be added
// beside the listings unchecked.
func checkGhAPICalls(t *testing.T, where, section string, allowed []string) {
	t.Helper()
	rest := section
	for {
		at := strings.Index(rest, "gh api ")
		if at < 0 {
			return
		}
		call := rest[at:]
		if !slices.ContainsFunc(allowed, func(c string) bool { return strings.HasPrefix(call, c) }) {
			line, _, _ := strings.Cut(call, "\n")
			t.Errorf("%s runs %q, which is neither a listing nor a known read", where, line)
		}
		rest = call[len("gh api "):]
	}
}

// gojqStrictPrelude reproduces gojq's strict string-type check on rtrimstr
// when running under C jq (where null | rtrimstr(...) silently returns null,
// whereas gh's embedded gojq errors on non-string input).
const gojqStrictPrelude = `def rtrimstr($s): if type != "string" then error("rtrimstr: non-string") elif endswith($s) then .[:length - ($s | length)] else . end; `

// runJQ runs filter on input as gh --jq runs it on a page, and returns what
// it printed. It fails t unless jq exits 0 with nothing on standard error.
func runJQ(t *testing.T, jq, filter string, input any) string {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(jq, "-c", gojqStrictPrelude+filter)
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("jq: %v (stderr %q)", err, stderr.String())
	}
	if stderr.Len() > 0 {
		t.Errorf("jq printed to standard error: %q", stderr.String())
	}
	return stdout.String()
}

// TestFixprguardCallSites checks that each fixprguard subcommand is called in
// the part of the command documents that the design assigns it to.
func TestFixprguardCallSites(t *testing.T) {
	fixpr := doctestutil.ReadDoc(t, fixprPath)
	runpr := doctestutil.ReadDoc(t, runprPath)
	for _, tc := range []struct {
		name, doc, heading, call string
	}{
		{"fixpr.md", fixpr, fixprPhase1, "fixprguard begin "},
		{"fixpr.md", fixpr, fixprPhase3, "fixprguard fence "},
		{"fixpr.md", fixpr, fixprPhase35, "fixprguard inspect "},
		{"fixpr.md", fixpr, fixprPhase4, "fixprguard finish "},
		{"fixpr.md", fixpr, fixprPhase4, "fixprguard push "},
		{"fixpr.md", fixpr, fixprPhase45, "fixprguard check "},
		{"fixpr.md", fixpr, fixprPhase45, "fixprguard push-review "},
		{"fixpr.md", fixpr, fixprPhase5, "fixprguard reply "},
		{"fixpr.md", fixpr, fixprPhase6, "fixprguard edit-pr "},
		{"fixpr.md", fixpr, fixprPhase6, "fixprguard append-pr "},
		{"runpr.md", runpr, runprStep1, "fixprguard head "},
		{"runpr.md", runpr, runprStep1, "fixprguard interaction-limits "},
		{"runpr.md", runpr, runprStep3, "fixprguard mentions "},
		{"runpr.md", runpr, runprStep3, "fixprguard review-state "},
		{"runpr.md", runpr, runprStep3, "fixprguard list "},
		{"runpr.md", runpr, runprStep3, "fixprguard ci-checks "},
		{"runpr.md", runpr, runprStep4, "fixprguard mentions "},
		{"runpr.md", runpr, runprStep4, "fixprguard review-state "},
		{"runpr.md", runpr, runprStep4, "fixprguard reply "},
		{"runpr.md", runpr, runprStep4, "fixprguard edit-pr "},
	} {
		if !strings.Contains(requireSection(t, tc.name, tc.doc, tc.heading), tc.call) {
			t.Errorf("%s %q does not call %q", tc.name, tc.heading, strings.TrimSpace(tc.call))
		}
	}
}

// bodyRead matches a gh --json field list that asks for the body.
var bodyRead = regexp.MustCompile(`--json\s+\S*\bbody\b`)

// countFilterCommand is the form of the command fixpr.md Phase 6 runs to
// count the correction paragraphs of the PR description; the filter holds no
// single quote.
var countFilterCommand = regexp.MustCompile(`^gh pr view <number> --json body --jq '([^']*)'$`)

// correctionOpener starts the line of a correction paragraph in a PR
// description. It is a bold marker of the Japanese rule text.
const correctionOpener = "**編集上の訂正（"

// TestFixprPhase6CountFilter checks the one command of fixpr.md that this
// session runs to read the PR description: it is the only command line
// starting with gh whose --json field list holds body (agent prompts are
// blockquotes, whose lines start with >), it is in
// Phase 6, and its --jq filter prints the number of correction paragraphs and
// never the body, so the description does not enter the session. jq must be installed: a
// missing jq fails the test instead of skipping it.
func TestFixprPhase6CountFilter(t *testing.T) {
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Fatal("unsupported test environment: jq is not installed")
	}
	doc := doctestutil.ReadDoc(t, fixprPath)
	section := requireSection(t, "fixpr.md", doc, fixprPhase6)
	var filters []string
	reads := 0
	for line := range strings.Lines(doc) {
		line = strings.TrimSpace(line)
		// A command line of a code block starts with the command; prose that
		// names a read, such as the Authorization list, does not run it.
		if !strings.HasPrefix(line, "gh ") || !bodyRead.MatchString(line) {
			continue
		}
		if !strings.Contains(section, line) {
			t.Errorf("fixpr.md reads the body outside the agent prompts and outside Phase 6: %q", line)
		}
		reads++
		if m := countFilterCommand.FindStringSubmatch(line); m != nil {
			filters = append(filters, m[1])
		}
	}
	if reads != 1 || len(filters) != 1 {
		t.Fatalf("fixpr.md reads the body %d times outside the agent prompts, %d of them as the count command; want 1 and 1", reads, len(filters))
	}
	paragraph := correctionOpener + "/fixpr、2026-11-02）:**\n\n- `docs/tasks/0001_x/01_requirements.md`: " + bodyMarker
	for _, tc := range []struct {
		name string
		body any
		want string
	}{
		{"two_paragraphs", bodyMarker + "\n\n" + paragraph + "\n\n" + paragraph, "2"},
		{"lookalikes_not_counted", bodyMarker + "\n  " + correctionOpener + "x\nsee " + correctionOpener + "x\n\n" + paragraph, "1"},
		{"crlf", bodyMarker + "\r\n" + correctionOpener + "PR の作成、2026-11-02）:**\r\n", "1"},
		{"empty", "", "0"},
		{"null", nil, "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Every body holds bodyMarker, so an exact count also shows
			// that no part of the body was printed.
			out := runJQ(t, jq, filters[0], map[string]any{"body": tc.body})
			if out != tc.want+"\n" {
				t.Errorf("the filter printed %q, want %q", out, tc.want+"\n")
			}
		})
	}
}

// TestFixprBuildChecksMatchTool checks that the build checks fixpr.md and
// _context.md state are the make targets finish runs, in the same order: the
// tool holds them as fixed values, so a change to one side alone fails here.
func TestFixprBuildChecksMatchTool(t *testing.T) {
	makeTarget := regexp.MustCompile(`make ([a-z-]+)`)
	targets := func(text string) []string {
		var out []string
		for _, m := range makeTarget.FindAllStringSubmatch(text, -1) {
			out = append(out, m[1])
		}
		return out
	}
	want := fixprguard.BuildCheckTargets()

	fixpr := doctestutil.ReadDoc(t, fixprPath)
	_, after, found := strings.Cut(fixpr, "Build checks for this project")
	if !found {
		t.Fatal("fixpr.md does not state the build checks")
	}
	line, _, _ := strings.Cut(after, "\n\n")
	if got := targets(line); !slices.Equal(got, want) {
		t.Errorf("fixpr.md build checks are %v, want the tool's %v", got, want)
	}

	row := contextRow(t, "Build checks (run after edits)")
	if got := targets(strings.Join(row, " ")); !slices.Equal(got, want) {
		t.Errorf("_context.md build checks are %v, want the tool's %v", got, want)
	}
}

// reviewerRowProblem is a kind of defect checkContextReviewers finds in the
// reviewer rows of _context.md. Each check has its own kind, so a test can
// tell which check caught a broken copy.
type reviewerRowProblem string

const (
	// problemRowCount: a required reviewer row is missing or repeated, or a
	// retired single-reviewer row is still present.
	problemRowCount reviewerRowProblem = "row count"
	// problemListShape: the list value before the first " — " is not
	// backquoted names separated by ", ".
	problemListShape reviewerRowProblem = "list shape"
	// problemListParse: fixprguard.ParseReviewers rejects the names.
	problemListParse reviewerRowProblem = "list parse"
	// problemListNames: the list is not exactly wantReviewers.
	problemListNames reviewerRowProblem = "list names"
	// problemPRReviewersTable: the ### PR reviewers 5-column table is missing,
	// repeated, has the wrong columns, or holds an unexpected number of rows.
	problemPRReviewersTable reviewerRowProblem = "pr reviewers table"
	// problemPRReviewerLogin: a PR reviewers row's Reviewer cell is not a
	// single backquoted valid login, or repeats a login.
	problemPRReviewerLogin reviewerRowProblem = "pr reviewer login"
	// problemPRReviewerProfile: a PR reviewers row's Profile cell is not a
	// single backquoted valid ReviewProfile, or repeats a profile.
	problemPRReviewerProfile reviewerRowProblem = "pr reviewer profile"
	// problemPRReviewerMode: a PR reviewers row's Mode cell is not a single
	// backquoted declared mode ("manual" or "auto+manual").
	problemPRReviewerMode reviewerRowProblem = "pr reviewer mode"
	// problemPRReviewerRequest: a PR reviewers row's Post request or Accepted
	// requests cell is malformed, contains a mention outside
	// fixprguard.RejectedMentions(), repeats an accepted request, or omits
	// Post request from Accepted requests.
	problemPRReviewerRequest reviewerRowProblem = "pr reviewer request"
	// problemPRReviewersValues: the parsed PR reviewers rows do not match
	// wantPRReviewers.
	problemPRReviewersValues reviewerRowProblem = "pr reviewers values"
	// problemRejectedShape: the rejected-mentions value before the first " — "
	// is not backquoted mentions separated by ", ".
	problemRejectedShape reviewerRowProblem = "rejected shape"
	// problemRejectedNames: the rejected-mentions list is not exactly
	// fixprguard.RejectedMentions().
	problemRejectedNames reviewerRowProblem = "rejected names"
)

// prReviewerSpec is one row of _context.md's ### PR reviewers table.
type prReviewerSpec struct {
	reviewer string
	profile  string
	mode     string
	post     string
	accepted []string
}

var (
	prReviewersHeading = "### PR reviewers"
	prReviewersHeader  = []string{"Reviewer", "Profile", "Mode", "Post request", "Accepted requests"}
	wantPRReviewers    = []prReviewerSpec{
		{
			reviewer: "coderabbitai",
			profile:  "coderabbit",
			mode:     "manual",
			post:     "@coderabbitai review",
			accepted: []string{"@coderabbitai review"},
		},
		{
			reviewer: "sourcery-ai",
			profile:  "sourcery",
			mode:     "auto+manual",
			post:     "@sourcery-ai review",
			accepted: []string{"@sourcery-ai review", "@sourceryai review"},
		},
	}
)

// valueSeparator ends the names part of a reviewer row's value; the
// description follows it.
const valueSeparator = " — "

var (
	// reviewerListShape is the strict form of a comma-space separated list of
	// backquoted values.
	reviewerListShape = regexp.MustCompile("^`[^`]+`(, `[^`]+`)*$")
	// singleBackquoted is the strict form of a single backquoted value.
	singleBackquoted = regexp.MustCompile("^`([^`]+)`$")
	backquoted       = regexp.MustCompile("`([^`]+)`")
)

// contextReviewers is what checkContextReviewers reads from _context.md: the
// automated reviewer list and the PR reviewers table.
type contextReviewers struct {
	names       []string
	prReviewers []prReviewerSpec
	problems    []reviewerRowProblem
}

// checkContextReviewers reads the reviewer rows and ### PR reviewers table of
// _context.md and reports every defect it finds. Only the part of a Name/Value
// row before the first " — " is read as values. The retired single-reviewer
// rows (Awaited PR reviewer, PR review request, Automated reviewer) must be
// absent.
func checkContextReviewers(doc string) contextReviewers {
	var got contextReviewers
	count := map[string][][]string{}
	for _, table := range doctestutil.MarkdownTables(doc) {
		if len(table) == 0 || len(table[0]) == 0 || table[0][0] != "Name" {
			continue
		}
		for _, name := range []string{reviewerListRow, awaitedRow, requestRow, rejectedMentionsRow, oldReviewerRow} {
			count[name] = append(count[name], tableRowsWithFirstCell(table, name)...)
		}
	}
	if len(count[reviewerListRow]) != 1 || len(count[rejectedMentionsRow]) != 1 ||
		len(count[awaitedRow]) != 0 || len(count[requestRow]) != 0 || len(count[oldReviewerRow]) != 0 {
		got.problems = append(got.problems, problemRowCount)
	}

	// A row without a value cell is checked as an empty value, so it fails
	// the shape check instead of being skipped.
	if rows := count[reviewerListRow]; len(rows) == 1 {
		got.checkList(valueCell(rows[0]))
	}
	if rows := count[rejectedMentionsRow]; len(rows) == 1 {
		got.checkRejected(valueCell(rows[0]))
	}
	got.checkPRReviewers(doc)
	return got
}

// valueCell returns row's second cell, or "" when the row has none.
func valueCell(row []string) string {
	if len(row) < 2 {
		return ""
	}
	return row[1]
}

// checkList checks the list row's value and records its names.
func (c *contextReviewers) checkList(value string) {
	names, _, found := strings.Cut(value, valueSeparator)
	if !found || !reviewerListShape.MatchString(names) {
		c.problems = append(c.problems, problemListShape)
		return
	}
	var list []string
	for _, m := range backquoted.FindAllStringSubmatch(names, -1) {
		list = append(list, m[1])
	}
	if _, err := fixprguard.ParseReviewers(list); err != nil {
		c.problems = append(c.problems, problemListParse)
		return
	}
	if sorted := slices.Sorted(slices.Values(list)); !slices.Equal(sorted, wantReviewers) {
		c.problems = append(c.problems, problemListNames)
	}
	c.names = list
}

// checkPRReviewers checks the ### PR reviewers 5-column table.
func (c *contextReviewers) checkPRReviewers(doc string) {
	section := doctestutil.SectionByHeading(doc, prReviewersHeading)
	if strings.TrimSpace(section) == "" {
		c.problems = append(c.problems, problemPRReviewersTable)
		return
	}
	tables := doctestutil.MarkdownTables(section)
	if len(tables) != 1 {
		c.problems = append(c.problems, problemPRReviewersTable)
		return
	}
	table := tables[0]
	if len(table) < 2 || !slices.Equal(table[0], prReviewersHeader) {
		c.problems = append(c.problems, problemPRReviewersTable)
		return
	}
	rows := table[1:]
	if len(rows) != len(wantPRReviewers) {
		c.problems = append(c.problems, problemPRReviewersTable)
	}
	seenLogin := map[string]struct{}{}
	seenProfile := map[string]struct{}{}
	rejected := fixprguard.RejectedMentions()
	var parsed []prReviewerSpec
	rowProblem := false
	for _, row := range rows {
		if len(row) != len(prReviewersHeader) {
			c.problems = append(c.problems, problemPRReviewersTable)
			return
		}
		mLogin := singleBackquoted.FindStringSubmatch(row[0])
		if mLogin == nil {
			c.problems = append(c.problems, problemPRReviewerLogin)
			rowProblem = true
			continue
		}
		if _, err := fixprguard.ParseLogin(mLogin[1]); err != nil {
			c.problems = append(c.problems, problemPRReviewerLogin)
			rowProblem = true
			continue
		}
		if _, dup := seenLogin[mLogin[1]]; dup {
			c.problems = append(c.problems, problemPRReviewerLogin)
			rowProblem = true
			continue
		}
		seenLogin[mLogin[1]] = struct{}{}

		mProfile := singleBackquoted.FindStringSubmatch(row[1])
		if mProfile == nil {
			c.problems = append(c.problems, problemPRReviewerProfile)
			rowProblem = true
			continue
		}
		if _, err := fixprguard.ParseReviewProfile(mProfile[1]); err != nil {
			c.problems = append(c.problems, problemPRReviewerProfile)
			rowProblem = true
			continue
		}
		if _, dup := seenProfile[mProfile[1]]; dup {
			c.problems = append(c.problems, problemPRReviewerProfile)
			rowProblem = true
			continue
		}
		seenProfile[mProfile[1]] = struct{}{}

		mMode := singleBackquoted.FindStringSubmatch(row[2])
		if mMode == nil || (mMode[1] != "manual" && mMode[1] != "auto+manual") {
			c.problems = append(c.problems, problemPRReviewerMode)
			rowProblem = true
			continue
		}

		mPost := singleBackquoted.FindStringSubmatch(row[3])
		if mPost == nil || !reviewerListShape.MatchString(row[4]) {
			c.problems = append(c.problems, problemPRReviewerRequest)
			rowProblem = true
			continue
		}
		var accepted []string
		seenReq := map[string]struct{}{}
		badReq := false
		for _, m := range backquoted.FindAllStringSubmatch(row[4], -1) {
			words := strings.Fields(m[1])
			if len(words) == 0 || !slices.Contains(rejected, words[0]) {
				badReq = true
				break
			}
			if _, dup := seenReq[m[1]]; dup {
				badReq = true
				break
			}
			seenReq[m[1]] = struct{}{}
			accepted = append(accepted, m[1])
		}
		if words := strings.Fields(mPost[1]); len(words) == 0 || !slices.Contains(rejected, words[0]) || !slices.Contains(accepted, mPost[1]) {
			badReq = true
		}
		if badReq {
			c.problems = append(c.problems, problemPRReviewerRequest)
			rowProblem = true
			continue
		}
		parsed = append(parsed, prReviewerSpec{
			reviewer: mLogin[1],
			profile:  mProfile[1],
			mode:     mMode[1],
			post:     mPost[1],
			accepted: accepted,
		})
	}
	c.prReviewers = parsed
	if !rowProblem && len(parsed) == len(wantPRReviewers) {
		same := true
		for i := range wantPRReviewers {
			a, b := parsed[i], wantPRReviewers[i]
			if a.reviewer != b.reviewer || a.profile != b.profile || a.mode != b.mode || a.post != b.post || !slices.Equal(a.accepted, b.accepted) {
				same = false
				break
			}
		}
		if !same {
			c.problems = append(c.problems, problemPRReviewersValues)
		}
	}
}

// checkRejected checks the Rejected mentions row's value against
// fixprguard.RejectedMentions().
func (c *contextReviewers) checkRejected(value string) {
	names, _, found := strings.Cut(value, valueSeparator)
	if !found || !reviewerListShape.MatchString(names) {
		c.problems = append(c.problems, problemRejectedShape)
		return
	}
	var list []string
	for _, m := range backquoted.FindAllStringSubmatch(names, -1) {
		list = append(list, m[1])
	}
	want := slices.Sorted(slices.Values(fixprguard.RejectedMentions()))
	if sorted := slices.Sorted(slices.Values(list)); !slices.Equal(sorted, want) {
		c.problems = append(c.problems, problemRejectedNames)
	}
}

// TestContextAndCommandsAgree checks that _context.md is the one place the
// automated reviewer names are written: its reviewer rows and ### PR reviewers
// table parse and hold the expected values, its rejected mentions match
// fixprguard, and fixpr.md, runpr.md, mkreq.md, and _lib/task-pr-handoff.md
// never write any reviewer's login.
func TestContextAndCommandsAgree(t *testing.T) {
	got := checkContextReviewers(doctestutil.ReadDoc(t, contextPath))
	if len(got.problems) > 0 {
		t.Fatalf("_context.md reviewer rows have problems %v", got.problems)
	}
	allNames := slices.Clone(got.names)
	for _, pr := range got.prReviewers {
		if !slices.Contains(allNames, pr.reviewer) {
			allNames = append(allNames, pr.reviewer)
		}
	}

	// The documents write every reviewer value through a placeholder, so a
	// name has no place in them at all. A name followed by [bot] contains the
	// name, so this rejects that form too.
	for _, tc := range []struct{ name, path string }{
		{"fixpr.md", fixprPath},
		{"runpr.md", runprPath},
		{"mkreq.md", mkreqPath},
		{"_lib/task-pr-handoff.md", handoffPath},
	} {
		doc := doctestutil.ReadDoc(t, tc.path)
		for _, name := range allNames {
			if strings.Contains(doc, name) {
				t.Errorf("%s writes the reviewer name %q", tc.name, name)
			}
		}
	}
}

// TestContextReviewerRowsMalformed checks that checkContextReviewers reports
// each kind of broken reviewer row or ### PR reviewers row, on copies of the
// real _context.md with one line edited, and that it reads the expected names
// from a description holding backquoted code and a second separator.
func TestContextReviewerRowsMalformed(t *testing.T) {
	doc := doctestutil.ReadDoc(t, contextPath)
	listLine := contextLine(t, doc, reviewerListRow)
	rejectedLine := contextLine(t, doc, rejectedMentionsRow)
	crRow := "| `coderabbitai` | `coderabbit` | `manual` | `@coderabbitai review` | `@coderabbitai review` |"
	sourceryRow := "| `sourcery-ai` | `sourcery` | `auto+manual` | `@sourcery-ai review` | `@sourcery-ai review`, `@sourceryai review` |"
	listRow := func(value string) string { return "| " + reviewerListRow + " | " + value + " |" }
	rejectedRowLine := func(value string) string { return "| " + rejectedMentionsRow + " | " + value + " |" }
	const desc = " — the automated reviewers"

	for _, tc := range []struct {
		name     string
		old, new string
		want     []reviewerRowProblem
	}{
		// The control row: the unedited file has no problem.
		{"real", listLine, listLine, nil},
		{
			"description_with_code_and_separator", listLine,
			listRow("`coderabbitai`, `sourcery-ai` — named in `[bot]`-less form — see `fixpr.md`"), nil,
		},
		{"list_row_missing", listLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"rejected_row_missing", rejectedLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"list_row_twice", listLine, listLine + "\n" + listLine, []reviewerRowProblem{problemRowCount}},
		{"rejected_row_twice", rejectedLine, rejectedLine + "\n" + rejectedLine, []reviewerRowProblem{problemRowCount}},
		{"retired_awaited_row_present", rejectedLine, rejectedLine + "\n| " + awaitedRow + " | `coderabbitai` — retired |", []reviewerRowProblem{problemRowCount}},
		{"retired_request_row_present", rejectedLine, rejectedLine + "\n| " + requestRow + " | `@coderabbitai review` — retired |", []reviewerRowProblem{problemRowCount}},
		{"old_row_left", rejectedLine, rejectedLine + "\n| " + oldReviewerRow + " | `coderabbitai` — the reviewer |", []reviewerRowProblem{problemRowCount}},
		{"list_no_value_cell", listLine, "| " + reviewerListRow + " |", []reviewerRowProblem{problemListShape}},
		{"list_value_only_description", listLine, listRow(desc), []reviewerRowProblem{problemListShape}},
		{"no_separator", listLine, listRow("`coderabbitai`, `sourcery-ai`"), []reviewerRowProblem{problemListShape}},
		{"no_backquotes", listLine, listRow("coderabbitai, sourcery-ai" + desc), []reviewerRowProblem{problemListShape}},
		{"no_space_after_comma", listLine, listRow("`coderabbitai`,`sourcery-ai`" + desc), []reviewerRowProblem{problemListShape}},
		{"two_spaces_after_comma", listLine, listRow("`coderabbitai`,  `sourcery-ai`" + desc), []reviewerRowProblem{problemListShape}},
		{"trailing_separator", listLine, listRow("`coderabbitai`, `sourcery-ai`," + desc), []reviewerRowProblem{problemListShape}},
		{"name_outside_grammar", listLine, listRow("`coderabbitai`, `sourcery-ai[bot]`" + desc), []reviewerRowProblem{problemListParse}},
		{"name_twice", listLine, listRow("`coderabbitai`, `sourcery-ai`, `sourcery-ai`" + desc), []reviewerRowProblem{problemListParse}},
		{"third_name", listLine, listRow("`coderabbitai`, `sourcery-ai`, `other-bot`" + desc), []reviewerRowProblem{problemListNames}},
		{"name_missing", listLine, listRow("`coderabbitai`" + desc), []reviewerRowProblem{problemListNames}},
		{"name_replaced", listLine, listRow("`coderabbitai`, `other-bot`" + desc), []reviewerRowProblem{problemListNames}},
		{"name_case_differs", listLine, listRow("`coderabbitai`, `Sourcery-AI`" + desc), []reviewerRowProblem{problemListNames}},
		{"pr_reviewers_heading_missing", "\n" + prReviewersHeading + "\n", "\n### Other table\n", []reviewerRowProblem{problemPRReviewersTable}},
		{"pr_reviewers_row_missing", sourceryRow + "\n", "", []reviewerRowProblem{problemPRReviewersTable}},
		{"pr_reviewers_bad_login", crRow, "| `coderabbitai[bot]` | `coderabbit` | `manual` | `@coderabbitai review` | `@coderabbitai review` |", []reviewerRowProblem{problemPRReviewerLogin}},
		{"pr_reviewers_dup_login", sourceryRow, "| `coderabbitai` | `sourcery` | `auto+manual` | `@sourcery-ai review` | `@sourcery-ai review`, `@sourceryai review` |", []reviewerRowProblem{problemPRReviewerLogin}},
		{"pr_reviewers_bad_profile", crRow, "| `coderabbitai` | `unknown` | `manual` | `@coderabbitai review` | `@coderabbitai review` |", []reviewerRowProblem{problemPRReviewerProfile}},
		{"pr_reviewers_dup_profile", sourceryRow, "| `sourcery-ai` | `coderabbit` | `auto+manual` | `@sourcery-ai review` | `@sourcery-ai review`, `@sourceryai review` |", []reviewerRowProblem{problemPRReviewerProfile}},
		{"pr_reviewers_bad_mode", crRow, "| `coderabbitai` | `coderabbit` | `auto` | `@coderabbitai review` | `@coderabbitai review` |", []reviewerRowProblem{problemPRReviewerMode}},
		{"pr_reviewers_post_not_in_accepted", sourceryRow, "| `sourcery-ai` | `sourcery` | `auto+manual` | `@sourcery-ai review` | `@sourceryai review` |", []reviewerRowProblem{problemPRReviewerRequest}},
		{"pr_reviewers_unknown_mention", crRow, "| `coderabbitai` | `coderabbit` | `manual` | `@other-bot review` | `@other-bot review` |", []reviewerRowProblem{problemPRReviewerRequest}},
		{"pr_reviewers_mention_not_first", crRow, "| `coderabbitai` | `coderabbit` | `manual` | `review @coderabbitai` | `review @coderabbitai` |", []reviewerRowProblem{problemPRReviewerRequest}},
		{"pr_reviewers_values_changed", crRow, "| `other-bot` | `coderabbit` | `manual` | `@coderabbitai review` | `@coderabbitai review` |", []reviewerRowProblem{problemPRReviewersValues}},
		{"rejected_no_value_cell", rejectedLine, "| " + rejectedMentionsRow + " |", []reviewerRowProblem{problemRejectedShape}},
		{"rejected_no_separator", rejectedLine, rejectedRowLine("`@codex`, `@coderabbitai`, `@sourcery-ai`, `@sourceryai`"), []reviewerRowProblem{problemRejectedShape}},
		{"rejected_no_backquotes", rejectedLine, rejectedRowLine("@codex, @coderabbitai, @sourcery-ai, @sourceryai" + desc), []reviewerRowProblem{problemRejectedShape}},
		{"rejected_missing_one", rejectedLine, rejectedRowLine("`@codex`, `@coderabbitai`, `@sourcery-ai`" + desc), []reviewerRowProblem{problemRejectedNames}},
		{"rejected_extra_one", rejectedLine, rejectedRowLine("`@codex`, `@coderabbitai`, `@sourcery-ai`, `@sourceryai`, `@other`" + desc), []reviewerRowProblem{problemRejectedNames}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Count(doc, tc.old) != 1 {
				t.Fatalf("_context.md holds %q %d times, want once", tc.old, strings.Count(doc, tc.old))
			}
			got := checkContextReviewers(strings.Replace(doc, tc.old, tc.new, 1))
			if !slices.Equal(got.problems, tc.want) {
				t.Errorf("problems = %v, want %v", got.problems, tc.want)
			}
			if tc.want == nil && (len(got.names) != len(wantReviewers) || len(got.prReviewers) != len(wantPRReviewers)) {
				t.Errorf("names = %q, prReviewers = %+v; want %d and %d", got.names, got.prReviewers, len(wantReviewers), len(wantPRReviewers))
			}
		})
	}
}

// contextLine returns the line of the _context.md text doc that holds the
// table row whose first cell is name.
func contextLine(t *testing.T, doc, name string) string {
	t.Helper()
	for line := range strings.Lines(doc) {
		line = strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(line, "| "+name+" | ") {
			return line
		}
	}
	t.Fatalf("_context.md has no %q row", name)
	return ""
}

// TestRemovedCommandLiterals checks that fixpr.md runs none of the
// hand-written commands that would bypass the guard tool: staging, committing,
// and pushing by hand, fetching thread comments itself, and passing a PR title
// inline.
func TestRemovedCommandLiterals(t *testing.T) {
	fixpr := doctestutil.ReadDoc(t, fixprPath)
	for _, literal := range []string{"git add -A", "git commit -m", "comments(first:", "reviewThreads"} {
		if strings.Contains(fixpr, literal) {
			t.Errorf("fixpr.md still contains %q", literal)
		}
	}
	for line := range strings.Lines(fixpr) {
		if strings.Contains(line, "gh pr edit") && strings.Contains(line, `--title "`) {
			t.Errorf("fixpr.md still passes a PR title inline: %q", strings.TrimSpace(line))
		}
	}
	for _, heading := range []string{fixprPhase1, fixprPhase2, fixprPhase3, fixprPhase35, fixprPhase4, fixprPhase45, fixprPhase5, fixprPhase6} {
		if strings.Contains(requireSection(t, "fixpr.md", fixpr, heading), "git push") {
			t.Errorf("fixpr.md %q pushes with git push instead of fixprguard push or push-review", heading)
		}
	}
}

// procedureStep returns step n of a numbered procedure: from the line that
// starts "<n>. **" up to the next line that starts a numbered bold step, or ""
// when there is no such line.
func procedureStep(doc string, n int) string {
	start := fmt.Sprintf("%d. **", n)
	var b strings.Builder
	in := false
	for line := range strings.Lines(doc) {
		if numberedStep.MatchString(line) {
			if in {
				break
			}
			in = strings.HasPrefix(line, start)
		}
		if in {
			b.WriteString(line)
		}
	}
	return b.String()
}

// numberedStep matches the first line of a numbered bold step at column 0.
var numberedStep = regexp.MustCompile(`^[0-9]+\. \*\*`)

// TestLocalReviewGuard: the guard call at the trust boundary of fixpr.md
// Phase 4.5 is wired in. local-cli-review.md declares a GUARD input, steps 1,
// 5, 6, and 7 (the record commit) run it (each names it; how many points
// one step holds is read in review), and fixpr.md Phase 4.5 passes
// fixprguard check as GUARD.
func TestLocalReviewGuard(t *testing.T) {
	review := doctestutil.ReadDoc(t, localReviewPath)
	inputs := requireSection(t, "local-cli-review.md", review, "## Inputs the calling command must supply")
	if !strings.Contains("\n"+inputs, "\n- **GUARD**") {
		t.Error("local-cli-review.md declares no **GUARD** input")
	}
	procedure := requireSection(t, "local-cli-review.md", review, "## Procedure")
	for _, n := range []int{1, 5, 6, 7} {
		step := procedureStep(procedure, n)
		if step == "" {
			t.Errorf("local-cli-review.md has no step %d", n)
		} else if !strings.Contains(step, "GUARD") {
			t.Errorf("local-cli-review.md step %d does not run GUARD", n)
		}
	}
	phase := flat(requireSection(t, "fixpr.md", doctestutil.ReadDoc(t, fixprPath), fixprPhase45))
	// The value passed runs to the next bold input name.
	_, after, ok := strings.Cut(phase, "**GUARD** = ")
	value, _, _ := strings.Cut(after, "**")
	if !ok || !strings.Contains(value, "`fixprguard check`") {
		t.Error("fixpr.md Phase 4.5 does not pass the fixprguard check call as **GUARD**")
	}
}

var (
	// ghBodyCommandRE matches any shell command invoking gh (at command start,
	// after VAR=val assignments, after wrapper words like command/env, after
	// shell list/pipe operators, or inside $(...)) whose --json field list
	// requests body.
	ghBodyCommandRE = regexp.MustCompile(`(?:^|[\s(;&|])gh\b.*--json(?:=|\s+)\S*\bbody\b`)
	// ghBodySafeRedirectRE matches a single gh --json ... body command that
	// writes stdout directly to a scratchpad file with no pipe, command
	// chaining, or stdout tee; group 1 holds its VAR=val assignment prefix.
	ghBodySafeRedirectRE = regexp.MustCompile(`^\s*((?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)*)gh\s+[^|;&\n]+--json(?:=|\s+)\S*\bbody\b[^|;&\n]*\s+1?>\s*(?:"<scratchpad>/[^"\s]+"|<scratchpad>/[^\s"']+)\s*$`)
)

// stripShellComment returns s up to the first unquoted shell comment (# at the
// start of a word outside single/double quotes and backslash escapes).
func stripShellComment(s string) string {
	inSingle := false
	inDouble := false
	escaped := false
	wordStart := true
	for i := range len(s) {
		c := s[i]
		if escaped {
			escaped = false
			wordStart = false
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			wordStart = false
			continue
		}
		if inDouble {
			switch c {
			case '\\':
				escaped = true
			case '"':
				inDouble = false
			}
			wordStart = false
			continue
		}
		switch c {
		case '\\':
			escaped = true
			wordStart = false
		case '\'':
			inSingle = true
			wordStart = false
		case '"':
			inDouble = true
			wordStart = false
		case '#':
			if wordStart {
				return strings.TrimRight(s[:i], " \t")
			}
			wordStart = false
		case ' ', '\t', '\r', '\n', ';', '&', '|', '(', ')':
			wordStart = true
		default:
			wordStart = false
		}
	}
	return s
}

// extractGHBodyCalls collects every gh ... --json ... body command from all
// fenced code blocks (joining backslash continuations) and all inline code
// spans in doc (folding soft-wrapped newlines to spaces), stripping unquoted
// trailing shell comments so a commented-out redirection is never accepted as
// executed. Each command line is kept whole without splitting on ;, &&, or ||
// so a chained leak after a redirect is caught as unsafe.
func extractGHBodyCalls(doc string) []string {
	var rawCommands []string
	var pending strings.Builder
	flush := func() {
		if pending.Len() > 0 {
			rawCommands = append(rawCommands, pending.String())
			pending.Reset()
		}
	}
	var prose strings.Builder
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc))) {
		switch {
		case line.Fence:
			flush()
		case line.Block:
			trimmed := strings.TrimSpace(line.Text)
			if trimmed == "" {
				flush()
				continue
			}
			if before, cont := strings.CutSuffix(trimmed, "\\"); cont {
				pending.WriteString(strings.TrimRight(before, " \t"))
				pending.WriteByte(' ')
				continue
			}
			pending.WriteString(trimmed)
			flush()
		default:
			prose.WriteString(line.Text)
			prose.WriteByte('\n')
		}
	}
	flush()
	for _, m := range codeSpanRE.FindAllStringSubmatch(prose.String(), -1) {
		span := strings.ReplaceAll(m[1], "\r\n", " ")
		span = strings.ReplaceAll(span, "\n", " ")
		rawCommands = append(rawCommands, strings.TrimSpace(span))
	}
	var calls []string
	for _, raw := range rawCommands {
		execPart := stripShellComment(raw)
		if ghBodyCommandRE.MatchString(execPart) {
			calls = append(calls, execPart)
		}
	}
	return calls
}

// checkHarnessAntigravityPhase6Redirect verifies that doc contains at least
// one gh ... --json ... body invocation and that every such invocation writes
// directly to a scratchpad file with a single output redirection (ignoring
// angle-bracket placeholders like <number>), without leaking stdout, and
// with GH_DEBUG set to off so gh debug output cannot write the body to stderr.
func checkHarnessAntigravityPhase6Redirect(doc string) (int, []string) {
	calls := extractGHBodyCalls(doc)
	var unsafe []string
	for _, call := range calls {
		stripped := leftPlaceholder.ReplaceAllString(call, "")
		m := ghBodySafeRedirectRE.FindStringSubmatch(call)
		if m == nil || strings.Count(stripped, ">") != 1 || !ghDebugDisabled(m[1]) {
			unsafe = append(unsafe, call)
		}
	}
	return len(calls), unsafe
}

// ghDebugDisabled reports whether the VAR=val assignment prefix of a gh call
// sets GH_DEBUG to a value gh reads as off, and sets DEBUG, when it sets it,
// to such a value too. gh's debug output writes the HTTP response, and with it
// the PR body, to standard error, which the redirection of standard output
// does not catch; GH_DEBUG takes precedence over DEBUG, so setting it to off
// also shields the call from an inherited DEBUG.
func ghDebugDisabled(assignments string) bool {
	off := func(v string) bool { return v == "0" || v == "false" || v == "no" }
	ghDebugOff := false
	for a := range strings.FieldsSeq(assignments) {
		name, value, _ := strings.Cut(a, "=")
		switch name {
		case "GH_DEBUG":
			ghDebugOff = off(value)
			if !ghDebugOff {
				return false
			}
		case "DEBUG":
			if !off(value) {
				return false
			}
		}
	}
	return ghDebugOff
}

// TestHarnessAntigravityPhase6Redirect checks that harness-antigravity.md
// isolates untrusted PR bodies in fixpr Phase 6 by redirecting every
// gh ... --json ... body invocation directly to a scratchpad file with zero
// stdout output to the main conversation.
func TestHarnessAntigravityPhase6Redirect(t *testing.T) {
	doc := doctestutil.ReadDoc(t, harnessAntigravityPath)
	count, unsafe := checkHarnessAntigravityPhase6Redirect(doc)
	if count == 0 {
		t.Fatal("harness-antigravity.md contains no gh --json ... body invocation; want at least 1")
	}
	for _, call := range unsafe {
		t.Errorf("harness-antigravity.md has an unsafe gh body read (not redirected to the scratchpad alone, or without GH_DEBUG off): %q", call)
	}

	// Verify the checker distinguishes safe direct redirects from zero-match
	// documents and unsafe invocations (missing redirects, chained commands,
	// env-var/wrapper prefixes, multiline continuations, wrapped inline spans,
	// pipes, and extra redirections).
	cases := []struct {
		name       string
		doc        string
		wantCount  int
		wantUnsafe int
	}{
		{"zero_matches", "```bash\ngit status\n```\n", 0, 0},
		{"direct_redirect", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body > \"<scratchpad>/<run-id>-pr.json\"\n```\n", 1, 0},
		{"equals_json_with_redirect", "```bash\nGH_DEBUG=0 gh pr view <number> --json=title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 0},
		{"env_prefix_with_redirect", "```bash\nGH_HOST=github.com GH_DEBUG=0 gh pr view <number> --json title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 0},
		{"multiline_with_redirect", "```bash\nGH_DEBUG=0 gh pr view <number> \\\n  --json title,body \\\n  > \"<scratchpad>/pr.json\"\n```\n", 1, 0},
		{"missing_redirect", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body\n```\n", 1, 1},
		{"equals_json_missing_redirect", "```bash\nGH_DEBUG=0 gh pr view <number> --json=title,body\n```\n", 1, 1},
		{"env_prefix_missing_redirect", "```bash\nGH_HOST=github.com GH_DEBUG=0 gh pr view <number> --json title,body\n```\n", 1, 1},
		{"command_sub_missing_redirect", "```bash\nout=$(GH_DEBUG=0 gh pr view <number> --json title,body)\n```\n", 1, 1},
		{"command_wrapper_missing_redirect", "```bash\nGH_DEBUG=0 command gh pr view <number> --json title,body\n```\n", 1, 1},
		{"chained_cat_leaks_body", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body > \"<scratchpad>/pr.json\" && cat \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"unspaced_semicolon_gh", "```bash\necho ok;GH_DEBUG=0 gh pr view <number> --json title,body\n```\n", 1, 1},
		{"multiline_missing_redirect", "```bash\nGH_DEBUG=0 gh pr view <number> \\\n  --json title,body\n```\n", 1, 1},
		{"inline_code_missing_redirect", "Run `GH_DEBUG=0 gh pr view <number> --json title,body` first.\n", 1, 1},
		{"wrapped_inline_code_missing_redirect", "Run `GH_DEBUG=0 gh pr view <number>\n--json title,body` first.\n", 1, 1},
		{"pipe_to_tee_leaks_stdout", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body | tee \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"double_redirect_rejected", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body > /dev/stdout > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"unspaced_double_redirect_rejected", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body>/dev/stdout > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"commented_out_redirect_rejected", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body # > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"trailing_comment_after_redirect", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body > \"<scratchpad>/pr.json\" # save metadata\n```\n", 1, 0},
		{"redirect_outside_scratchpad", "```bash\nGH_DEBUG=0 gh pr view <number> --json title,body > /tmp/pr.json\n```\n", 1, 1},
		{"gh_debug_unset_rejected", "```bash\ngh pr view <number> --json title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"gh_debug_api_rejected", "```bash\nGH_DEBUG=api gh pr view <number> --json title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"debug_truthy_rejected", "```bash\nGH_DEBUG=0 DEBUG=1 gh pr view <number> --json title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"debug_off_with_gh_debug_off", "```bash\nGH_DEBUG=0 DEBUG=0 gh pr view <number> --json title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCount, gotUnsafe := checkHarnessAntigravityPhase6Redirect(tc.doc)
			if gotCount != tc.wantCount || len(gotUnsafe) != tc.wantUnsafe {
				t.Errorf("checkHarnessAntigravityPhase6Redirect = (%d, %v), want count=%d unsafe=%d",
					gotCount, gotUnsafe, tc.wantCount, tc.wantUnsafe)
			}
		})
	}
}
