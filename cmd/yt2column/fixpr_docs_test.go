//go:build test

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// Paths from cmd/yt2column to the command documents these tests check.
const (
	fixprPath   = "../../.claude/commands/fixpr.md"
	runprPath   = "../../.claude/commands/runpr.md"
	contextPath = "../../.claude/commands/_context.md"
	handoffPath = "../../.claude/commands/_lib/task-pr-handoff.md"
)

// Section headings of fixpr.md and runpr.md the contract tests read.
const (
	fixprConvergence = "## Convergence policy"
	fixprPhase1      = "## Phase 1 — Fetch (this session)"
	fixprPhase2      = "## Phase 2 — Triage (model: default)"
	fixprPhase3      = "## Phase 3 — Fix (model: default)"
	fixprPhase35     = "## Phase 3.5 — Scope check (this session)"
	fixprPhase4      = "## Phase 4 — Build + commit + push (this session)"
	fixprPhase5      = "## Phase 5 — Reply + resolve (this session)"
	fixprPhase6      = "## Phase 6 — Wrap: PR description (model: haiku)"
	runprStep1       = "## 1. Resolve the PR, and create it for a standalone change"
	runprStep3       = "## 3. Wait for Codex's first review and CI, running fixpr as comments appear"
	runprStep4       = "## 4. Confirm, record, and merge"
)

// Names of the _context.md rows that hold the automated reviewers and the PR
// review request, and of the single row they replaced, which must not come
// back.
const (
	reviewerListRow = "Automated reviewer list"
	awaitedRow      = "Awaited PR reviewer"
	requestRow      = "PR review request"
	oldReviewerRow  = "Automated reviewer"
)

// wantReviewers is the automated reviewer list the task decided on, in sorted
// order, since the list's names are sorted before they are compared with it.
// Adding or removing a reviewer changes _context.md and this value in the same
// commit.
var wantReviewers = []string{"chatgpt-codex-connector", "coderabbitai"}

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
// step 1 runs fixprguard head before gh pr checkout, the document never
// queries review threads itself, step 3 names the stops fixpr reports, and
// step 3's listings print no comment body (see checkListings).
func TestRunprDocContract(t *testing.T) {
	doc := doctestutil.ReadDoc(t, runprPath)
	step3 := requireSection(t, "runpr.md", doc, runprStep3)

	t.Run("head_before_checkout", func(t *testing.T) {
		step1 := requireSection(t, "runpr.md", doc, runprStep1)
		head := strings.Index(step1, "fixprguard head ")
		checkout := strings.Index(step1, "gh pr checkout")
		if head < 0 || checkout < 0 || head > checkout {
			t.Errorf("runpr.md step 1 must run fixprguard head (at %d) before gh pr checkout (at %d)", head, checkout)
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

	t.Run("listings_print_no_body", func(t *testing.T) {
		checkListings(t, "runpr.md step 3", step3, runprListingFixtures(), step3ReadCalls)
	})
}

// TestHandoffListingPrintsNoBody checks that the listing
// _lib/task-pr-handoff.md Part B step 5 polls for Codex's start marker prints
// no comment body, as runpr.md step 3's listings do (see checkListings).
func TestHandoffListingPrintsNoBody(t *testing.T) {
	step5 := itemSection("5. Make sure Codex starts reviewing the PR.", "\n6. ")(t, doctestutil.ReadDoc(t, handoffPath))
	checkListings(t, "_lib/task-pr-handoff.md Part B step 5", step5, handoffListingFixtures(), handoffReadCalls)
}

// The values the listing filters' placeholders are replaced with before the
// filters run. They hold no character a jq string literal would need to
// escape.
const (
	fakeAwaited = "codex-bot"
	fakeUser    = "octo-user"
	fakeRequest = fixprguard.CodexMention + " review"
)

// bodyMarker is put wherever a fake body can hold it. A filter that prints a
// body, or a part of one, prints it; the filters print no other text that
// could hold it.
const bodyMarker = "zq7mark"

var (
	// paginatedListing is a gh api --paginate listing: the endpoint under
	// repos/<owner>/<repo>/ and the --jq filter, which holds no single quote.
	paginatedListing = regexp.MustCompile(`gh api --paginate repos/<owner>/<repo>/(\S+) --jq '([^']*)'`)
	// leftPlaceholder is a placeholder the replacement did not cover.
	leftPlaceholder = regexp.MustCompile(`<[a-z][a-z-]*>`)
)

// runprListingKeys are the keys each kind of object runpr.md step 3's
// listings print may hold. None of them carries a body: state is derived from
// a summary comment's state line, never copied from it.
var runprListingKeys = map[string][]string{
	"summary": {"kind", "url", "updated_at", "state"},
	"codex":   {"kind", "url", "created_at"},
	"request": {"kind", "url", "created_at"},
	"mention": {"kind", "url", "created_at", "updated_at"},
}

// listingItem is one item of a fake GitHub response and what the filter must
// print for it: kind is "" when it must print nothing, and state is the
// summary comment's expected state.
type listingItem struct {
	userType, login string
	body            any
	kind, state     string
}

// listingFixture is the fake response for one listing: its items, the field
// of an item each output key other than kind and state must equal, and the
// keys each kind of output object may hold. An object without a kind key is
// of kind kindless, when that is set.
type listingFixture struct {
	items    []listingItem
	source   map[string]string
	keys     map[string][]string
	kindless string
}

// kindOf returns the kind of obj, an object the listing printed.
func (f listingFixture) kindOf(obj map[string]any) any {
	if kind, ok := obj["kind"]; ok || f.kindless == "" {
		return kind
	}
	return f.kindless
}

// marked wraps line in bodyMarker at its start and end.
func marked(line string) string { return bodyMarker + " " + line + " " + bodyMarker }

// summaryBody is a Codex summary comment's body: the first line, the state
// line built from stateJSON, and a marked line of findings.
func summaryBody(stateLine string) string {
	return "<!-- codex-pull-request-review-summary -->\n" + stateLine + "\n" + marked("Findings follow.")
}

// stateLine is a well-formed state line around stateJSON.
func stateLine(stateJSON string) string {
	return "<!-- codex-review-state:v1 " + stateJSON + " -->"
}

// runprListingFixtures are the fake responses, by endpoint. The items take
// every branch of the filters: each summary state, Codex's other comments, a
// request and the near-requests that are mentions, the author checks, and
// bodies with and without the mention, null included.
func runprListingFixtures() map[string]listingFixture {
	sha := strings.Repeat("a1", 20)
	reviewState := func(status string, rest string) string {
		return `{"code_review":{"status":` + status + `,"commit_sha":"` + sha + `"` + rest + `},"note":"` + bodyMarker + `"}`
	}
	completed := reviewState(`"completed"`, `,"has_any_findings":true,"note":"`+bodyMarker+`"`)
	bot, user := "Bot", "User"
	awaited := fakeAwaited + "[bot]"
	comments := map[string]string{"url": "html_url", "created_at": "created_at", "updated_at": "updated_at"}
	return map[string]listingFixture{
		"issues/<number>/comments": {source: comments, keys: runprListingKeys, items: []listingItem{
			{bot, awaited, summaryBody(stateLine(completed)), "summary", "completed " + sha},
			{bot, awaited, summaryBody(stateLine(reviewState(`"completed"`, `,"has_any_findings":false`))), "summary", "completed " + sha},
			{bot, awaited, summaryBody(stateLine(reviewState(`"running"`, `,"note":"`+bodyMarker+`"`))), "summary", "running " + sha},
			// The state line's shape: marked, cut short, or ended by a CR.
			{bot, awaited, summaryBody(marked(stateLine(completed))), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(completed) + "\r"), "summary", "indeterminate"},
			{bot, awaited, "<!-- codex-pull-request-review-summary -->", "summary", "indeterminate"},
			// Not JSON, and JSON of the wrong shape or types.
			{bot, awaited, summaryBody(stateLine(bodyMarker + " not json")), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(`["` + bodyMarker + `"]`)), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(`{"code_review":"` + bodyMarker + `"}`)), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(reviewState(`1`, ``))), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(`{"code_review":{"status":"running","commit_sha":"abc1234"}}`)), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(`{"code_review":{"status":"running","commit_sha":7}}`)), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(reviewState(`"completed"`, `,"has_any_findings":"`+bodyMarker+`"`))), "summary", "indeterminate"},
			{bot, awaited, summaryBody(stateLine(reviewState(`"completed"`, ``))), "summary", "indeterminate"},
			// An unknown status, which the marker itself is.
			{bot, awaited, summaryBody(stateLine(reviewState(`"`+bodyMarker+`"`, ``))), "summary", "indeterminate"},
			// Codex's other comments.
			{bot, awaited, marked("A note.") + "\n" + marked(fixprguard.CodexMention+" a second line."), "codex", ""},
			{bot, awaited, nil, "codex", ""},
			// The request, and the comments that are not one.
			{user, fakeUser, "  " + fakeRequest + "\n", "request", ""},
			{user, fakeUser, marked(fakeRequest), "mention", ""},
			{user, "other-user", fakeRequest, "mention", ""},
			{bot, fakeUser, fakeRequest, "mention", ""},
			// The summary's author must be a Bot.
			{user, awaited, summaryBody(stateLine(completed)), "", ""},
			{user, "other-user", marked(strings.ToUpper(fixprguard.CodexMention)+" please look") + "\n" + marked("Thanks."), "mention", ""},
			{user, "other-user", marked("No mention here."), "", ""},
			{bot, "other-bot[bot]", marked(fixprguard.CodexMention + " from another Bot"), "mention", ""},
			{user, "other-user", nil, "", ""},
		}},
		"pulls/<number>/comments": {source: comments, keys: runprListingKeys, items: []listingItem{
			{bot, awaited, marked(fixprguard.CodexMention + " from Codex"), "", ""},
			{user, "other-user", marked("No mention here."), "", ""},
			{user, "other-user", marked("Ask "+strings.ToUpper(fixprguard.CodexMention)) + "\n" + marked("again"), "mention", ""},
			{user, awaited, marked(fixprguard.CodexMention + " from a User"), "mention", ""},
			{bot, "other-bot[bot]", marked(fixprguard.CodexMention + " from another Bot"), "mention", ""},
			{user, "other-user", nil, "", ""},
		}},
		"pulls/<number>/reviews": {source: map[string]string{"url": "html_url", "created_at": "submitted_at"}, keys: runprListingKeys, items: []listingItem{
			{bot, awaited, marked(fixprguard.CodexMention + " from Codex"), "", ""},
			{user, "other-user", "", "", ""},
			{user, "other-user", marked("Ask "+fixprguard.CodexMention) + "\n" + marked("again"), "mention", ""},
			{bot, "other-bot[bot]", marked(fixprguard.CodexMention + " from another Bot"), "mention", ""},
			{user, "other-user", nil, "", ""},
		}},
	}
}

// handoffListingFixtures is the fake response for the start-marker listing,
// which prints a kindless {url, updated_at} for each Codex summary comment:
// the summary, Codex's other comments, the author checks, and a null body.
func handoffListingFixtures() map[string]listingFixture {
	awaited := fakeAwaited + "[bot]"
	summary := summaryBody(stateLine(`{"code_review":{"status":"running"},"note":"` + bodyMarker + `"}`))
	return map[string]listingFixture{
		"issues/<number>/comments": {
			source:   map[string]string{"url": "html_url", "updated_at": "updated_at", "created_at": "created_at"},
			keys:     map[string][]string{"start": {"url", "updated_at"}},
			kindless: "start",
			items: []listingItem{
				{"Bot", awaited, summary, "start", ""},
				{"Bot", awaited, marked("A note.") + "\n" + marked("Another line."), "", ""},
				{"Bot", awaited, nil, "", ""},
				{"User", awaited, summary, "", ""},
				{"Bot", "other-bot[bot]", summary, "", ""},
				{"User", "other-user", marked(fixprguard.CodexMention + " please look"), "", ""},
			},
		},
	}
}

// handoffReadCalls are the gh api calls of _lib/task-pr-handoff.md Part B
// step 5: the listing and the Date header.
var handoffReadCalls = []string{
	"gh api --paginate ",
	"gh api -i repos/<owner>/<repo>/pulls/<number> | grep -i -m 1 '^date:'",
}

// timeFieldSeconds gives each time field its own second, so a value copied
// from the wrong field differs from the right one.
var timeFieldSeconds = map[string]int{"created_at": 10, "updated_at": 20, "submitted_at": 30}

// fakeTimeBase is the time the fake items' times count from, a minute apart.
var fakeTimeBase = time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

// response is the fake GitHub response for fixture, one object per item, with
// a distinct URL and distinct times for each.
func (f listingFixture) response() []map[string]any {
	var out []map[string]any
	for i, it := range f.items {
		obj := map[string]any{
			"user":     map[string]any{"type": it.userType, "login": it.login},
			"body":     it.body,
			"html_url": fmt.Sprintf("https://github.test/item/%d", i),
		}
		for _, field := range f.source {
			if second, ok := timeFieldSeconds[field]; ok {
				at := fakeTimeBase.Add(time.Duration(i)*time.Minute + time.Duration(second)*time.Second)
				obj[field] = at.Format(time.RFC3339)
			}
		}
		out = append(out, obj)
	}
	return out
}

// checkListings runs each gh api --paginate listing of section, which where
// names, with jq on its fake response from fixtures, and checks that it prints
// no body: every object the filter builds holds only its kind's keys, no
// output holds bodyMarker, and every printed value is the item's own non-body
// field or, for state, the state the item's state line yields. Every other
// gh api call of section must be one of readCalls. jq must be installed: a
// missing jq fails the test instead of skipping it, so the check never passes
// unrun.
func checkListings(t *testing.T, where, section string, fixtures map[string]listingFixture, readCalls []string) {
	t.Helper()
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Fatal("unsupported test environment: jq is not installed")
	}
	checkGhAPICalls(t, where, section, readCalls)
	matches := paginatedListing.FindAllStringSubmatch(section, -1)
	if n := strings.Count(section, "gh api --paginate"); n != len(matches) || n != len(fixtures) {
		t.Fatalf("%s has %d gh api --paginate listings, %d of them in the expected form; want %d",
			where, n, len(matches), len(fixtures))
	}
	replacer := strings.NewReplacer(
		"<awaited-reviewer>", fakeAwaited,
		"<user-login>", fakeUser,
		"<review-request>", fakeRequest,
		"<review-mention>", fixprguard.CodexMention,
	)
	seen := map[string]struct{}{}
	for _, m := range matches {
		endpoint, filter := m[1], replacer.Replace(m[2])
		fixture, ok := fixtures[endpoint]
		if !ok {
			t.Errorf("%s lists %s, which has no fake response", where, endpoint)
			continue
		}
		if _, dup := seen[endpoint]; dup {
			t.Errorf("%s lists %s twice", where, endpoint)
		}
		seen[endpoint] = struct{}{}
		if left := leftPlaceholder.FindString(filter); left != "" {
			t.Errorf("the %s filter holds the placeholder %s, which the test does not replace", endpoint, left)
			continue
		}
		t.Run(endpoint, func(t *testing.T) {
			checkListingConstructors(t, filter, fixture)
			response := fixture.response()
			checkListingOutput(t, fixture, response, runJQ(t, jq, filter, response))
		})
	}
}

// step3ReadCalls are the gh api calls of runpr.md step 3 other than the
// listings: reading the user's login and the Date header, and the two reads
// that report a stop, which may print a body because the session reads it
// only to report the stop.
var step3ReadCalls = []string{
	"gh api --paginate ",
	"gh api user --jq .login",
	"gh api -i repos/<owner>/<repo>/pulls/<number> | grep -i -m 1 '^date:'",
	"gh api repos/<owner>/<repo>/issues/comments/<id> --jq .body",
	`gh api repos/<owner>/<repo>/issues/comments/<id> --jq '(.body // "" | split("\n"))[1] // "" | .[:500]'`,
}

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

// checkListingConstructors checks the keys of every object a filter builds,
// whether or not a fake item reaches it: each starts with a kind of
// fixture.keys, or has no kind when fixture.kindless is set, and holds only
// keys that kind may hold. It does not read the values, nor a branch whose
// result is not an object: those are checked only by running the filter on
// the items that reach them. A "{" followed by a digit is a regex quantifier,
// not an object.
func checkListingConstructors(t *testing.T, filter string, fixture listingFixture) {
	t.Helper()
	for i := 0; i+1 < len(filter); i++ {
		if filter[i] != '{' || '0' <= filter[i+1] && filter[i+1] <= '9' {
			continue
		}
		body, ok := jqObjectBody(filter[i+1:])
		if !ok {
			t.Errorf("the filter has an unclosed object at %q", filter[i:])
			return
		}
		entries := splitTopLevel(body)
		kind, isKind := strings.CutPrefix(strings.TrimSpace(entries[0]), "kind: ")
		kind = strings.Trim(kind, `"`)
		if !isKind {
			kind = fixture.kindless
		}
		allowed, known := fixture.keys[kind]
		if kind == "" || !known {
			t.Errorf("the filter builds {%s}, which does not start with a known kind", body)
			continue
		}
		for _, entry := range entries {
			key, _, _ := strings.Cut(entry, ":")
			if !slices.Contains(allowed, strings.TrimSpace(key)) {
				t.Errorf("the filter builds {%s}, whose entry %q its kind may not hold", body, strings.TrimSpace(entry))
			}
		}
	}
}

// jqObjectBody returns the text of s up to the brace that closes the object
// opened just before s, skipping nested brackets and string literals.
func jqObjectBody(s string) (string, bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			i = skipJQString(s, i)
		case '(', '[', '{':
			depth++
		case ')', ']':
			depth--
		case '}':
			if depth == 0 {
				return s[:i], true
			}
			depth--
		}
	}
	return "", false
}

// splitTopLevel splits an object body at its top-level commas.
func splitTopLevel(body string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '"':
			i = skipJQString(body, i)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, body[start:i])
				start = i + 1
			}
		}
	}
	return append(out, body[start:])
}

// skipJQString returns the index of the quote that closes the string literal
// opening at s[open].
func skipJQString(s string, open int) int {
	i := open + 1
	for i < len(s) && s[i] != '"' {
		if s[i] == '\\' {
			i++
		}
		i++
	}
	return i
}

// runJQ runs filter on input as gh --jq runs it on a page, and returns what
// it printed. It fails t unless jq exits 0 with nothing on standard error.
func runJQ(t *testing.T, jq, filter string, input any) string {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(jq, "-c", filter)
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

// checkListingOutput checks what a listing printed for fixture.
func checkListingOutput(t *testing.T, fixture listingFixture, response []map[string]any, out string) {
	t.Helper()
	if strings.Contains(strings.ToLower(out), bodyMarker) {
		t.Errorf("the filter printed a body or a part of one:\n%s", out)
	}
	byURL := map[string]int{}
	for i, obj := range response {
		byURL[obj["html_url"].(string)] = i
	}
	printed := map[int]int{}
	dec := json.NewDecoder(strings.NewReader(out))
	for {
		var obj map[string]any
		if err := dec.Decode(&obj); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("the filter printed %q, not a sequence of JSON objects: %v", out, err)
		}
		url, _ := obj["url"].(string)
		i, ok := byURL[url]
		if !ok {
			t.Errorf("printed %v, whose url is no item's", obj)
			continue
		}
		printed[i]++
		item := fixture.items[i]
		if kind := fixture.kindOf(obj); kind != item.kind {
			t.Errorf("item %d: printed kind %v, want %q", i, kind, item.kind)
		}
		for key, value := range obj {
			switch {
			case !slices.Contains(fixture.keys[item.kind], key):
				t.Errorf("item %d: printed the key %q, which a %q may not hold", i, key, item.kind)
			case key == "kind":
			case key == "state":
				if value != item.state {
					t.Errorf("item %d: printed state %v, want %q", i, value, item.state)
				}
			case value != response[i][fixture.source[key]]:
				t.Errorf("item %d: printed %s = %v, want its %s %v", i, key, value, fixture.source[key], response[i][fixture.source[key]])
			}
		}
	}
	for i, item := range fixture.items {
		want := 0
		if item.kind != "" {
			want = 1
		}
		if printed[i] != want {
			t.Errorf("item %d (%s %s, want kind %q) printed %d times, want %d", i, item.userType, item.login, item.kind, printed[i], want)
		}
	}
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
		{"fixpr.md", fixpr, fixprPhase5, "fixprguard reply "},
		{"fixpr.md", fixpr, fixprPhase6, "fixprguard edit-pr "},
		{"runpr.md", runpr, runprStep1, "fixprguard head "},
		{"runpr.md", runpr, runprStep3, "fixprguard list "},
		{"runpr.md", runpr, runprStep4, "fixprguard reply "},
	} {
		if !strings.Contains(requireSection(t, tc.name, tc.doc, tc.heading), tc.call) {
			t.Errorf("%s %q does not call %q", tc.name, tc.heading, strings.TrimSpace(tc.call))
		}
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
	// problemRowCount: a reviewer row is missing or repeated, or the old
	// single-reviewer row is still there.
	problemRowCount reviewerRowProblem = "row count"
	// problemListShape: the list value before the first " — " is not
	// backquoted names separated by ", ".
	problemListShape reviewerRowProblem = "list shape"
	// problemListParse: fixprguard.ParseReviewers rejects the names.
	problemListParse reviewerRowProblem = "list parse"
	// problemListNames: the list is not exactly wantReviewers.
	problemListNames reviewerRowProblem = "list names"
	// problemAwaitedShape: the awaited value before the first " — " is not
	// exactly one backquoted name.
	problemAwaitedShape reviewerRowProblem = "awaited shape"
	// problemAwaitedParse: fixprguard.ParseLogin rejects the awaited name.
	problemAwaitedParse reviewerRowProblem = "awaited parse"
	// problemRequestShape: the request value before the first " — " is not
	// exactly one backquoted text.
	problemRequestShape reviewerRowProblem = "request shape"
	// problemRequestMention: the request's first word is not the mention
	// fixprguard rejects, fixprguard.CodexMention.
	problemRequestMention reviewerRowProblem = "request mention"
)

// valueSeparator ends the names part of a reviewer row's value; the
// description follows it.
const valueSeparator = " — "

var (
	// reviewerListShape is the strict form of the list's names part: backquoted
	// names joined by a comma and exactly one space.
	reviewerListShape = regexp.MustCompile("^`[^`]+`(, `[^`]+`)*$")
	// awaitedShape is the strict form of the awaited reviewer's names part.
	awaitedShape = regexp.MustCompile("^`([^`]+)`$")
	backquoted   = regexp.MustCompile("`([^`]+)`")
)

// contextReviewers is what checkContextReviewers reads from _context.md: the
// list's names, the awaited reviewer's name, and the PR review request's
// description. A field is empty when its check failed before reaching it.
type contextReviewers struct {
	names       []string
	awaited     string
	requestDesc string
	problems    []reviewerRowProblem
}

// checkContextReviewers reads the reviewer rows of the _context.md text doc
// and reports every defect it finds. Only the part of a value before the
// first " — " is read as names, so code in the description such as `[bot]`
// is never taken for a name. The list goes through fixprguard.ParseReviewers,
// the function the tool itself parses --reviewer with, so the guard and the
// tool accept the same names.
func checkContextReviewers(doc string) contextReviewers {
	var got contextReviewers
	count := map[string][][]string{}
	for _, table := range doctestutil.MarkdownTables(doc) {
		if len(table) == 0 || len(table[0]) == 0 || table[0][0] != "Name" {
			continue
		}
		for _, name := range []string{reviewerListRow, awaitedRow, requestRow, oldReviewerRow} {
			count[name] = append(count[name], tableRowsWithFirstCell(table, name)...)
		}
	}
	if len(count[reviewerListRow]) != 1 || len(count[awaitedRow]) != 1 || len(count[requestRow]) != 1 ||
		len(count[oldReviewerRow]) != 0 {
		got.problems = append(got.problems, problemRowCount)
	}

	// A row without a value cell is checked as an empty value, so it fails
	// the shape check instead of being skipped.
	if rows := count[reviewerListRow]; len(rows) == 1 {
		got.checkList(valueCell(rows[0]))
	}
	if rows := count[awaitedRow]; len(rows) == 1 {
		got.checkAwaited(valueCell(rows[0]))
	}
	if rows := count[requestRow]; len(rows) == 1 {
		got.checkRequest(valueCell(rows[0]))
	}
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

// checkAwaited checks the awaited reviewer row's value and records its name.
func (c *contextReviewers) checkAwaited(value string) {
	name, _, found := strings.Cut(value, valueSeparator)
	m := awaitedShape.FindStringSubmatch(name)
	if !found || m == nil {
		c.problems = append(c.problems, problemAwaitedShape)
		return
	}
	if _, err := fixprguard.ParseLogin(m[1]); err != nil {
		c.problems = append(c.problems, problemAwaitedParse)
		return
	}
	c.awaited = m[1]
}

// checkRequest checks the PR review request row's value and records the
// request. The request's first word must be the mention the guard tool
// rejects in replies and PR descriptions, so the tool and the command
// documents keep out the same mention.
func (c *contextReviewers) checkRequest(value string) {
	request, desc, found := strings.Cut(value, valueSeparator)
	m := awaitedShape.FindStringSubmatch(request)
	if !found || m == nil {
		c.problems = append(c.problems, problemRequestShape)
		return
	}
	if words := strings.Fields(m[1]); len(words) == 0 || words[0] != fixprguard.CodexMention {
		c.problems = append(c.problems, problemRequestMention)
		return
	}
	c.requestDesc = desc
}

// TestContextAndCommandsAgree checks that _context.md is the one place the
// automated reviewers' names are written: its reviewer rows parse and hold
// the expected names, the PR review request row's description names the
// mention fixprguard rejects, and fixpr.md, runpr.md, and
// _lib/task-pr-handoff.md never write a reviewer's name.
func TestContextAndCommandsAgree(t *testing.T) {
	got := checkContextReviewers(doctestutil.ReadDoc(t, contextPath))
	if len(got.problems) > 0 {
		t.Fatalf("_context.md reviewer rows have problems %v", got.problems)
	}
	if got.awaited != "chatgpt-codex-connector" {
		t.Errorf("_context.md %s = %q, want chatgpt-codex-connector", awaitedRow, got.awaited)
	}
	requirePhrases(t, "_context.md "+requestRow, got.requestDesc, "`"+fixprguard.CodexMention+"`")

	// The documents write every reviewer value through a placeholder, so a
	// name has no place in them at all. A name followed by [bot] contains the
	// name, so this rejects that form too.
	for _, tc := range []struct{ name, path string }{
		{"fixpr.md", fixprPath},
		{"runpr.md", runprPath},
		{"_lib/task-pr-handoff.md", handoffPath},
	} {
		doc := doctestutil.ReadDoc(t, tc.path)
		for _, name := range append(slices.Clone(got.names), got.awaited) {
			if strings.Contains(doc, name) {
				t.Errorf("%s writes the reviewer name %q", tc.name, name)
			}
		}
	}
}

// TestContextReviewerRowsMalformed checks that checkContextReviewers reports
// each kind of broken reviewer row, on copies of the real _context.md with one
// row edited, and that it reads exactly two names from a description holding
// backquoted code and a second separator.
func TestContextReviewerRowsMalformed(t *testing.T) {
	doc := doctestutil.ReadDoc(t, contextPath)
	listLine := contextLine(t, doc, reviewerListRow)
	awaitedLine := contextLine(t, doc, awaitedRow)
	requestLine := contextLine(t, doc, requestRow)
	listRow := func(value string) string { return "| " + reviewerListRow + " | " + value + " |" }
	awaitedRowLine := func(value string) string { return "| " + awaitedRow + " | " + value + " |" }
	requestRowLine := func(value string) string { return "| " + requestRow + " | " + value + " |" }
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
			listRow("`coderabbitai`, `chatgpt-codex-connector` — named in `[bot]`-less form — see `fixpr.md`"), nil,
		},
		{"list_row_missing", listLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"awaited_row_missing", awaitedLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"list_row_twice", listLine, listLine + "\n" + listLine, []reviewerRowProblem{problemRowCount}},
		{"awaited_row_twice", awaitedLine, awaitedLine + "\n" + awaitedLine, []reviewerRowProblem{problemRowCount}},
		{"old_row_left", awaitedLine, awaitedLine + "\n| " + oldReviewerRow + " | `coderabbitai` — the reviewer |", []reviewerRowProblem{problemRowCount}},
		{"list_no_value_cell", listLine, "| " + reviewerListRow + " |", []reviewerRowProblem{problemListShape}},
		{"list_value_only_description", listLine, listRow(desc), []reviewerRowProblem{problemListShape}},
		{"no_separator", listLine, listRow("`coderabbitai`, `chatgpt-codex-connector`"), []reviewerRowProblem{problemListShape}},
		{"no_backquotes", listLine, listRow("coderabbitai, chatgpt-codex-connector" + desc), []reviewerRowProblem{problemListShape}},
		{"no_space_after_comma", listLine, listRow("`coderabbitai`,`chatgpt-codex-connector`" + desc), []reviewerRowProblem{problemListShape}},
		{"two_spaces_after_comma", listLine, listRow("`coderabbitai`,  `chatgpt-codex-connector`" + desc), []reviewerRowProblem{problemListShape}},
		{"trailing_separator", listLine, listRow("`coderabbitai`, `chatgpt-codex-connector`," + desc), []reviewerRowProblem{problemListShape}},
		{"name_outside_grammar", listLine, listRow("`coderabbitai[bot]`, `chatgpt-codex-connector`" + desc), []reviewerRowProblem{problemListParse}},
		{"name_twice", listLine, listRow("`coderabbitai`, `chatgpt-codex-connector`, `coderabbitai`" + desc), []reviewerRowProblem{problemListParse}},
		{"third_name", listLine, listRow("`coderabbitai`, `chatgpt-codex-connector`, `other-bot`" + desc), []reviewerRowProblem{problemListNames}},
		{"name_replaced", listLine, listRow("`coderabbitai`, `other-bot`" + desc), []reviewerRowProblem{problemListNames}},
		{"awaited_two_names", awaitedLine, awaitedRowLine("`coderabbitai`, `chatgpt-codex-connector`" + desc), []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_no_value_cell", awaitedLine, "| " + awaitedRow + " |", []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_value_only_description", awaitedLine, awaitedRowLine(desc), []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_no_separator", awaitedLine, awaitedRowLine("`coderabbitai`"), []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_outside_grammar", awaitedLine, awaitedRowLine("`coderabbitai[bot]`" + desc), []reviewerRowProblem{problemAwaitedParse}},
		{"request_row_missing", requestLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"request_row_twice", requestLine, requestLine + "\n" + requestLine, []reviewerRowProblem{problemRowCount}},
		{"request_two_values", requestLine, requestRowLine("`@codex review`, `@codex`" + desc), []reviewerRowProblem{problemRequestShape}},
		{"request_no_backquotes", requestLine, requestRowLine("@codex review" + desc), []reviewerRowProblem{problemRequestShape}},
		{"request_no_separator", requestLine, requestRowLine("`@codex review`"), []reviewerRowProblem{problemRequestShape}},
		{"request_other_mention", requestLine, requestRowLine("`@chatgpt review`" + desc), []reviewerRowProblem{problemRequestMention}},
		{"request_longer_mention", requestLine, requestRowLine("`@codexbot review`" + desc), []reviewerRowProblem{problemRequestMention}},
		{"request_mention_not_first", requestLine, requestRowLine("`review @codex`" + desc), []reviewerRowProblem{problemRequestMention}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Count(doc, tc.old) != 1 {
				t.Fatalf("_context.md holds %q %d times, want once", tc.old, strings.Count(doc, tc.old))
			}
			got := checkContextReviewers(strings.Replace(doc, tc.old, tc.new, 1))
			if !slices.Equal(got.problems, tc.want) {
				t.Errorf("problems = %v, want %v", got.problems, tc.want)
			}
			if tc.want == nil && len(got.names) != len(wantReviewers) {
				t.Errorf("names = %q, want %d names", got.names, len(wantReviewers))
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
	// Phase 4.5 keeps git push: its commits come after finish, so fixprguard
	// push, which requires HEAD to be finish's commit, cannot push them.
	for _, heading := range []string{fixprPhase1, fixprPhase2, fixprPhase3, fixprPhase35, fixprPhase4, fixprPhase5, fixprPhase6} {
		if strings.Contains(requireSection(t, "fixpr.md", fixpr, heading), "git push") {
			t.Errorf("fixpr.md %q pushes with git push instead of fixprguard push", heading)
		}
	}
}

// readmeSyncMarker opens the README sync step in fixpr.md and
// _lib/task-pr-handoff.md.
const readmeSyncMarker = "**README sync.**"

// mktransPath is the path from cmd/yt2column to mktrans.md, which the README
// sync runs as a sub-step.
const mktransPath = "../../.claude/commands/mktrans.md"

// readmeSyncIn returns the part of section from the README sync marker to the
// section's end. It fails t unless doc holds the marker exactly once and that
// one is inside section after the text after.
func readmeSyncIn(t *testing.T, name, doc, section, after string) string {
	t.Helper()
	if n := strings.Count(doc, readmeSyncMarker); n != 1 {
		t.Fatalf("%s holds %q %d times, want once", name, readmeSyncMarker, n)
	}
	before, step, ok := strings.Cut(section, readmeSyncMarker)
	if !ok {
		t.Fatalf("%s runs the README sync outside the section it belongs in", name)
	}
	if !strings.Contains(before, after) {
		t.Errorf("%s runs the README sync before %q", name, after)
	}
	return step
}

// TestReadmeSyncStepPlacement checks where the README sync runs /mktrans
// inside the commands that push, since the sync sits at the guard tool's
// trust boundary: in fixpr.md inside Phase 3 after the Fix agent's prompt (so
// before the scope check, which judges README.en.md with the fix), with no
// staging or committing by hand there (Phase 4's guarded commit carries it;
// git push is checked for every phase by TestRemovedCommandLiterals); in
// _lib/task-pr-handoff.md inside Part B item 1 after the local review, so
// before CHECKS; and in mktrans.md, that the commit steps stay in the section
// for a standalone run, since a sub-step's caller owns the commit. Whether a
// sentence in prose asks for a commit is left to the PR review.
func TestReadmeSyncStepPlacement(t *testing.T) {
	t.Run("fixpr_in_phase_3_after_the_fix_agent", func(t *testing.T) {
		fixpr := doctestutil.ReadDoc(t, fixprPath)
		phase3 := requireSection(t, "fixpr.md", fixpr, fixprPhase3)
		step := readmeSyncIn(t, "fixpr.md", fixpr, phase3, "Reply with only this JSON")
		for _, literal := range []string{"git add", "git commit"} {
			if strings.Contains(step, literal) {
				t.Errorf("fixpr.md's README sync runs %q instead of leaving the commit to Phase 4", literal)
			}
		}
	})
	t.Run("handoff_in_part_b_item_1_after_the_local_review", func(t *testing.T) {
		handoff := doctestutil.ReadDoc(t, handoffPath)
		item1 := itemSection("1. When LOCAL_REVIEW is `run`", "\n2. Run CHECKS.")(t, handoff)
		readmeSyncIn(t, "_lib/task-pr-handoff.md", handoff, item1, "coderabbit-local-review.md")
	})
	t.Run("mktrans_commits_only_standalone", func(t *testing.T) {
		const commitSection = "## Commit (standalone only)"
		mktrans := doctestutil.ReadDoc(t, mktransPath)
		// The commit steps are the numbered items of the standalone section.
		steps := map[string]struct{}{}
		for line := range strings.Lines(requireSection(t, "mktrans.md", mktrans, commitSection)) {
			if numbered.MatchString(line) {
				steps[strings.TrimSpace(line)] = struct{}{}
			}
		}
		if len(steps) == 0 {
			t.Fatalf("mktrans.md has no numbered commit step in %q", commitSection)
		}
		// Walk the document by its level-2 sections, so a step copied word
		// for word into another section is found by where it is.
		current := ""
		for line := range strings.Lines(mktrans) {
			if doctestutil.HeadingLevel(line) == 2 {
				current = strings.TrimSpace(line)
				if current != commitSection && strings.Contains(strings.ToLower(current), "commit") {
					t.Errorf("mktrans.md has a second commit section %q", current)
				}
				continue
			}
			if _, ok := steps[strings.TrimSpace(line)]; ok && current != commitSection {
				t.Errorf("mktrans.md gives the commit step %q in %q", strings.TrimSpace(line), current)
			}
		}
	})
}

// numbered matches a numbered list item line.
var numbered = regexp.MustCompile(`^\d+\. `)
