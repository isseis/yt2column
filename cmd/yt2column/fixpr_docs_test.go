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

	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
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
	fixprPhase6      = "## Phase 6 — Wrap: PR description (model: haiku)"
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
var wantReviewers = []string{"coderabbitai"}

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
// pr checkout, the document never
// queries review threads itself, step 3 names the stops fixpr reports, and
// step 3's listings print no comment body (see checkListings).
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

	t.Run("listings_print_no_body", func(t *testing.T) {
		checkListings(t, "runpr.md step 3", step3, runprListingFixtures(), step3ReadCalls, true)
	})

	t.Run("request_comment_agrees_with_review_state", func(t *testing.T) {
		checkRequestAgreementWithReviewState(t, step3)
	})
}

// The values the listing filters' placeholders are replaced with before the
// filters run. They hold no character a jq string literal would need to
// escape.
const (
	fakeAwaited = "coderabbit-bot"
	fakeUser    = "octo-user"
	fakeRequest = "@coderabbitai review"
)

// Observed first lines of CodeRabbit's status comments that runpr.md step 3's
// mention listings exclude as non-mentions.
const (
	crObservedHeader1 = "<!-- coderabbit-ai-generated:v1 -->"
	crObservedHeader2 = "<!-- This is an auto-generated comment: coding agent status by CodeRabbit -->"
)

// bodyMarker is put wherever a fake body can hold it. A filter that prints a
// body, or a part of one, prints it; the filters print no other text that
// could hold it.
const bodyMarker = "zq7mark"

var (
	// paginatedListing is a gh api --paginate listing: the endpoint under
	// repos/<owner>/<repo>/, the --jq filter, which holds no single quote,
	// and the rest of the line the filter ends on.
	paginatedListing = regexp.MustCompile(`gh api --paginate repos/<owner>/<repo>/(\S+) --jq '([^']*)'(.*)`)
	// stderrToFile is the rest of a listing's line when it writes its
	// standard error to a file in the directory the run made for it, and
	// nothing else, so an error message quoting a body never reaches the
	// session. The file's name is the submatch.
	stderrToFile = regexp.MustCompile(`^ 2>"<stderr-dir>/([a-z-]+)"$`)
	// leftPlaceholder is a placeholder the replacement did not cover.
	leftPlaceholder = regexp.MustCompile(`<[a-z][a-z-]*>`)
)

// runprListingKeys are the keys each kind of object runpr.md step 3's
// listings print may hold. None of them carries a body: by_user and by_bot
// are computed from the author.
var runprListingKeys = map[string][]string{
	"request": {"kind", "url", "created_at"},
	"mention": {"kind", "url", "created_at", "updated_at", "by_user", "by_bot"},
}

// mentionAuthorKeys are the keys a mention must hold: runpr.md decides
// whether to stop by them, so a filter that leaves one out fails.
var mentionAuthorKeys = []string{"by_user", "by_bot"}

// runprRequiredKeys are the keys each kind of object runpr.md step 3's
// listings must hold.
var runprRequiredKeys = map[string][]string{
	"mention": mentionAuthorKeys,
}

// mentionWriter is who wrote a mention, which by_user and by_bot must report:
// notMention for an item that is no mention.
type mentionWriter int

const (
	notMention mentionWriter = iota
	byTheUser                // a User named <user-login>: by_user true
	byABot                   // any Bot: by_bot true
	byAPerson                // any other User: both false
)

// listingItem is one item of a fake GitHub response and what the filter must
// print for it: kind is "" when it must print nothing, and wrote is a
// mention's expected author.
type listingItem struct {
	userType, login string
	body            any
	kind            string
	wrote           mentionWriter
}

// listingFixture is the fake response for one listing: its items, the field
// of an item each output key other than kind must equal, and the keys each
// kind of output object may hold. required lists the keys each kind of object
// must hold, when a command decides by them.
type listingFixture struct {
	items    []listingItem
	source   map[string]string
	keys     map[string][]string
	required map[string][]string
}

// marked wraps line in bodyMarker at its start and end.
func marked(line string) string { return bodyMarker + " " + line + " " + bodyMarker }

// runprListingFixtures are the fake responses, by endpoint. The items cover
// both rejected mentions (in mixed case), CodeRabbit's two observed status
// headers (with and without trailing CR), unobserved CodeRabbit posts, other
// Bots (including Codex), a request and the near-requests that are mentions,
// the author checks, and bodies with and without a mention, null included.
func runprListingFixtures() map[string]listingFixture {
	mentions := fixprguard.RejectedMentions()
	m0, m1 := mentions[0], mentions[len(mentions)-1]
	bot, user := "Bot", "User"
	awaited := fakeAwaited + "[bot]"
	comments := map[string]string{"url": "html_url", "created_at": "created_at", "updated_at": "updated_at"}
	return map[string]listingFixture{
		"issues/<number>/comments": {source: comments, keys: runprListingKeys, required: runprRequiredKeys, items: []listingItem{
			// CodeRabbit's two observed headers (LF and CRLF) are excluded even
			// when their bodies carry a rejected mention.
			{bot, awaited, crObservedHeader1 + "\n" + marked(m1+" summary"), "", notMention},
			{bot, awaited, crObservedHeader1 + "\r\n" + marked(m0+" summary"), "", notMention},
			{bot, awaited, crObservedHeader2 + "\n" + marked(m1+" status"), "", notMention},
			{bot, awaited, crObservedHeader2 + "\r\n" + marked(m0+" status"), "", notMention},
			// Any other CodeRabbit comment with a rejected mention counts as by_bot;
			// empty and null bodies print nothing and must not crash gojq's rtrimstr.
			{bot, awaited, marked("Unobserved header.") + "\n" + marked(m1+" note"), "mention", byABot},
			{bot, awaited, "", "", notMention},
			{bot, awaited, nil, "", notMention},
			// A User with the awaited login and an observed header is still a person.
			{user, awaited, crObservedHeader1 + "\n" + marked(m1+" spoof"), "mention", byAPerson},
			// The user's exact request comment, and the near-requests that are mentions.
			{user, fakeUser, "  " + fakeRequest + "\n", "request", notMention},
			{user, fakeUser, "\t" + fakeRequest + "\r\n", "request", notMention},
			{user, fakeUser, marked(fakeRequest), "mention", byTheUser},
			{user, "other-user", fakeRequest, "mention", byAPerson},
			{bot, fakeUser, fakeRequest, "mention", byABot},
			// Both rejected mentions across author kinds and cases.
			{user, "other-user", marked(strings.ToUpper(m0)+" please look") + "\n" + marked("Thanks."), "mention", byAPerson},
			{user, "other-user", marked(strings.ToUpper(m1) + " please look"), "mention", byAPerson},
			{user, "other-user", marked("No mention here."), "", notMention},
			{bot, "chatgpt-codex-connector[bot]", marked(m0 + " summary from Codex"), "mention", byABot},
			{bot, "other-bot[bot]", marked(m1 + " from another Bot"), "mention", byABot},
			{user, "other-user", nil, "", notMention},
		}},
		"pulls/<number>/comments": {source: comments, keys: runprListingKeys, required: runprRequiredKeys, items: []listingItem{
			{user, fakeUser, marked(m0 + " from the user"), "mention", byTheUser},
			{user, fakeUser, marked(m1 + " from the user"), "mention", byTheUser},
			{bot, fakeUser, marked(m1 + " from a Bot named like the user"), "mention", byABot},
			{bot, awaited, crObservedHeader1 + "\r\n" + marked(m1+" observed"), "", notMention},
			{bot, awaited, crObservedHeader2 + "\n" + marked(m0+" observed"), "", notMention},
			{bot, awaited, marked(m1 + " from unobserved CodeRabbit"), "mention", byABot},
			{bot, awaited, "", "", notMention},
			{bot, awaited, nil, "", notMention},
			{user, "other-user", marked("No mention here."), "", notMention},
			{user, "other-user", marked("Ask "+strings.ToUpper(m0)) + "\n" + marked("again"), "mention", byAPerson},
			{user, awaited, crObservedHeader1 + "\n" + marked(m1+" from a User"), "mention", byAPerson},
			{bot, "other-bot[bot]", marked(m0 + " from another Bot"), "mention", byABot},
			{user, "other-user", nil, "", notMention},
		}},
		"pulls/<number>/reviews": {source: map[string]string{"url": "html_url", "created_at": "submitted_at"}, keys: runprListingKeys, required: runprRequiredKeys, items: []listingItem{
			{user, fakeUser, marked(m0 + " from the user"), "mention", byTheUser},
			{user, fakeUser, marked(m1 + " from the user"), "mention", byTheUser},
			{bot, fakeUser, marked(m0 + " from a Bot named like the user"), "mention", byABot},
			{user, awaited, crObservedHeader2 + "\n" + marked(m1+" from a User"), "mention", byAPerson},
			{bot, awaited, crObservedHeader1 + "\n" + marked(m1+" observed"), "", notMention},
			{bot, awaited, crObservedHeader2 + "\r\n" + marked(m0+" observed"), "", notMention},
			{bot, awaited, marked(m1 + " from unobserved CodeRabbit"), "mention", byABot},
			{bot, awaited, "", "", notMention},
			{bot, awaited, nil, "", notMention},
			{user, "other-user", "", "", notMention},
			{user, "other-user", marked("Ask "+strings.ToUpper(m1)) + "\n" + marked("again"), "mention", byAPerson},
			{bot, "other-bot[bot]", marked(m0 + " from another Bot"), "mention", byABot},
			{user, "other-user", nil, "", notMention},
		}},
	}
}

// timeFieldSeconds gives each time field its own second, so a value copied
// from the wrong field differs from the right one.
var timeFieldSeconds = map[string]int{"created_at": 10, "updated_at": 20, "submitted_at": 30}

// fakeTimeBase is the time the fake items' times count from, a minute apart.
var fakeTimeBase = time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

// fakeIDBase is the id of the first fake item. The ids count up from it, so
// no id equals an item's index.
const fakeIDBase = 7000

// response is the fake GitHub response for fixture, one object per item, with
// a distinct id, URL, and times for each. The id is a float64, the type JSON
// numbers decode to, so it compares equal with the id a filter prints.
func (f listingFixture) response() []map[string]any {
	var out []map[string]any
	for i, it := range f.items {
		obj := map[string]any{
			"user":     map[string]any{"type": it.userType, "login": it.login},
			"body":     it.body,
			"id":       float64(fakeIDBase + i),
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

// listingReplacer returns the placeholder replacer for runpr.md step 3's
// listing filters. extra holds further placeholder and value pairs beyond the
// four standard ones.
func listingReplacer(t *testing.T, extra ...string) *strings.Replacer {
	t.Helper()
	rejectedJSON, err := json.Marshal(fixprguard.RejectedMentions())
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReplacer(append([]string{
		"<awaited-reviewer>", fakeAwaited,
		"<user-login>", fakeUser,
		"<review-request>", fakeRequest,
		"<rejected-mentions>", string(rejectedJSON),
	}, extra...)...)
}

// checkListings runs each gh api --paginate listing of section, which where
// names, with jq on its fake response from fixtures, and checks that it prints
// no body: every object the filter builds holds only its kind's keys, no
// output holds bodyMarker, and every printed value is the item's own non-body
// field (or by_user / by_bot computed from the author). Every other gh api
// call of section must be one of readCalls. When stderrFile is set, each
// listing must write its standard error to a file of its own (stderrToFile),
// so one listing's error is never overwritten by another's before its size is
// read. extra holds further placeholder and value pairs for the filters,
// beyond the four every listing may use. jq must be installed: a missing jq
// fails the test instead of skipping it, so the check never passes unrun.
func checkListings(t *testing.T, where, section string, fixtures map[string]listingFixture, readCalls []string, stderrFile bool, extra ...string) {
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
	replacer := listingReplacer(t, extra...)
	seen := map[string]struct{}{}
	stderrFiles := map[string]string{}
	for _, m := range matches {
		endpoint, filter := m[1], replacer.Replace(m[2])
		if stderrFile {
			if file := stderrToFile.FindStringSubmatch(m[3]); file == nil {
				t.Errorf("%s lists %s without writing its standard error to a file alone: the line ends %q", where, endpoint, m[3])
			} else if other, dup := stderrFiles[file[1]]; dup {
				t.Errorf("%s lists %s and %s with the same standard error file %q", where, other, endpoint, file[1])
			} else {
				stderrFiles[file[1]] = endpoint
			}
		}
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

// step3ReadCalls are the gh api calls of runpr.md step 3: the three mention
// listings and reading the user's login.
var step3ReadCalls = []string{
	"gh api --paginate ",
	"gh api user --jq .login",
}

// checkRequestAgreementWithReviewState checks that runpr.md step 3's
// conversation-comment jq filter and fixprguard.PRReviewState classify the
// exact same comments as request comments, so a comment the jq filter skips
// as a request is also a request to review-state and vice versa.
func checkRequestAgreementWithReviewState(t *testing.T, step3 string) {
	t.Helper()
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Fatal("unsupported test environment: jq is not installed")
	}
	var rawFilter string
	for _, m := range paginatedListing.FindAllStringSubmatch(step3, -1) {
		if m[1] == "issues/<number>/comments" {
			rawFilter = m[2]
			break
		}
	}
	if rawFilter == "" {
		t.Fatal("runpr.md step 3 has no issues/<number>/comments listing")
	}
	filter := listingReplacer(t).Replace(rawFilter)

	pr, err := fixprguard.ParsePRNumber("42")
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := fixprguard.ParseLogin(fakeAwaited)
	if err != nil {
		t.Fatal(err)
	}
	headSHA := strings.Repeat("a1", 20)
	const (
		dateHeader = "HTTP/2.0 200 OK\r\nDate: Fri, 23 Oct 2026 09:05:00 GMT\r\n\r\n"
		createdAt  = "2026-10-23T09:00:00Z"
		commentURL = "https://github.com/isseis/yt2column/pull/42#issuecomment-7001"
	)

	for _, tc := range []struct {
		name        string
		userType    string
		login       string
		body        string
		wantRequest bool
	}{
		{"exact", "User", fakeUser, fakeRequest, true},
		{"spaces_and_lf", "User", fakeUser, "  " + fakeRequest + "\n", true},
		{"tabs_and_crlf", "User", fakeUser, "\t " + fakeRequest + " \r\n", true},
		{"extra_prefix", "User", fakeUser, "please " + fakeRequest, false},
		{"extra_suffix", "User", fakeUser, fakeRequest + " please", false},
		{"wrong_case", "User", fakeUser, strings.ToUpper(fakeRequest), false},
		{"empty_body", "User", fakeUser, "", false},
		{"other_user", "User", "other-user", fakeRequest, false},
		{"bot_same_login", "Bot", fakeUser, fakeRequest, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := map[string]any{
				"id":         float64(7001),
				"user":       map[string]any{"type": tc.userType, "login": tc.login},
				"body":       tc.body,
				"created_at": createdAt,
				"updated_at": createdAt,
				"html_url":   commentURL,
			}
			// 1) Run the documented jq filter.
			out := runJQ(t, jq, filter, []map[string]any{item})
			jqRequest := false
			dec := json.NewDecoder(strings.NewReader(out))
			for {
				var obj map[string]any
				if err := dec.Decode(&obj); err == io.EOF {
					break
				} else if err != nil {
					t.Fatalf("decode jq output %q: %v", out, err)
				}
				if obj["kind"] == "request" {
					jqRequest = true
				}
			}
			// 2) Run fixprguard.PRReviewState on the same comment.
			commentsJSON, err := json.Marshal([]map[string]any{item})
			if err != nil {
				t.Fatal(err)
			}
			steps := []cmdrunnertestutil.Step{
				{Name: "gh", Args: []string{"repo", "view", "--json", "nameWithOwner"}, Out: `{"nameWithOwner":"isseis/yt2column"}`},
				{Name: "gh", Args: []string{"api", "--include", "repos/isseis/yt2column/pulls/42"}, Out: dateHeader + `{"head":{"sha":"` + headSHA + `"}}`},
				{Name: "gh", Args: []string{"api", "user"}, Out: `{"login":"` + fakeUser + `"}`},
				{Name: "gh", Args: []string{"api", "--include", "repos/isseis/yt2column/issues/42/comments?per_page=100&page=1"}, Out: dateHeader + string(commentsJSON)},
			}
			if tc.wantRequest {
				steps = append(steps,
					cmdrunnertestutil.Step{
						Name: "gh",
						Args: []string{"api", "graphql", "--input", "-"},
						Out:  `{"data":{"repository":{"pullRequest":{"headRefOid":"` + headSHA + `","commits":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}}`,
					},
					cmdrunnertestutil.Step{
						Name: "gh",
						Args: []string{"api", "--include", "repos/isseis/yt2column/pulls/42/reviews?per_page=100&page=1"},
						Out:  dateHeader + `[]`,
					},
				)
			}
			fake := cmdrunnertestutil.New(t, steps...)
			gh, err := fixprguard.NewGitHubWithoutReviewer(fake)
			if err != nil {
				t.Fatalf("NewGitHubWithoutReviewer: %v", err)
			}
			got, err := gh.PRReviewState(t.Context(), fixprguard.ReviewStateRequest{
				PR:       pr,
				Reviewer: reviewer,
				Request:  fakeRequest,
			})
			if err != nil {
				t.Fatalf("PRReviewState: %v", err)
			}
			if got.State == fixprguard.RequestUnreadable {
				t.Fatalf("PRReviewState returned unreadable (reason=%v)", got.Reason)
			}
			guardRequest := got.Request != nil
			if jqRequest != tc.wantRequest || guardRequest != tc.wantRequest {
				t.Errorf("jq request=%v, review-state request=%v, want %v", jqRequest, guardRequest, tc.wantRequest)
			}
		})
	}
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
// fixture.keys and holds only keys that kind may hold. It does not read the
// values, nor a branch whose result is not an object: those are checked only
// by running the filter on the items that reach them. A "{" followed by a
// digit is a regex quantifier, not an object.
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
		allowed, known := fixture.keys[kind]
		if !isKind || kind == "" || !known {
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

// checkListingOutput checks what a listing printed for fixture.
func checkListingOutput(t *testing.T, fixture listingFixture, response []map[string]any, out string) {
	t.Helper()
	if strings.Contains(strings.ToLower(out), bodyMarker) {
		t.Errorf("the filter printed a body or a part of one:\n%s", out)
	}
	for i, item := range fixture.items {
		if (item.kind == "mention") != (item.wrote != notMention) {
			t.Fatalf("item %d: kind %q with writer %d; a mention needs its writer, and only a mention has one", i, item.kind, item.wrote)
		}
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
		if kind := obj["kind"]; kind != item.kind {
			t.Errorf("item %d: printed kind %v, want %q", i, kind, item.kind)
		}
		for _, key := range fixture.required[item.kind] {
			if _, ok := obj[key]; !ok {
				t.Errorf("item %d: printed no %s", i, key)
			}
		}
		for key, value := range obj {
			switch {
			case !slices.Contains(fixture.keys[item.kind], key):
				t.Errorf("item %d: printed the key %q, which a %q may not hold", i, key, item.kind)
			case key == "kind":
			case key == "by_user":
				if value != (item.wrote == byTheUser) {
					t.Errorf("item %d: printed by_user %v, want %t", i, value, item.wrote == byTheUser)
				}
			case key == "by_bot":
				if value != (item.wrote == byABot) {
					t.Errorf("item %d: printed by_bot %v, want %t", i, value, item.wrote == byABot)
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
		{"fixpr.md", fixpr, fixprPhase45, "fixprguard check "},
		{"fixpr.md", fixpr, fixprPhase45, "fixprguard push-review "},
		{"fixpr.md", fixpr, fixprPhase5, "fixprguard reply "},
		{"fixpr.md", fixpr, fixprPhase6, "fixprguard edit-pr "},
		{"fixpr.md", fixpr, fixprPhase6, "fixprguard append-pr "},
		{"runpr.md", runpr, runprStep1, "fixprguard head "},
		{"runpr.md", runpr, runprStep1, "fixprguard interaction-limits "},
		{"runpr.md", runpr, runprStep3, "fixprguard review-state "},
		{"runpr.md", runpr, runprStep3, "fixprguard list "},
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
	// problemRequestMention: the request's first word is not one of the
	// mentions fixprguard rejects (fixprguard.RejectedMentions).
	problemRequestMention reviewerRowProblem = "request mention"
	// problemRejectedShape: the rejected-mentions value before the first " — "
	// is not backquoted mentions separated by ", ".
	problemRejectedShape reviewerRowProblem = "rejected shape"
	// problemRejectedNames: the rejected-mentions list is not exactly
	// fixprguard.RejectedMentions().
	problemRejectedNames reviewerRowProblem = "rejected names"
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
// list's names, the awaited reviewer's name, and the PR review request. A
// field is empty when its check failed before reaching it.
type contextReviewers struct {
	names    []string
	awaited  string
	request  string
	problems []reviewerRowProblem
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
		for _, name := range []string{reviewerListRow, awaitedRow, requestRow, rejectedMentionsRow, oldReviewerRow} {
			count[name] = append(count[name], tableRowsWithFirstCell(table, name)...)
		}
	}
	if len(count[reviewerListRow]) != 1 || len(count[awaitedRow]) != 1 || len(count[requestRow]) != 1 ||
		len(count[rejectedMentionsRow]) != 1 || len(count[oldReviewerRow]) != 0 {
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
	if rows := count[rejectedMentionsRow]; len(rows) == 1 {
		got.checkRejected(valueCell(rows[0]))
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
// request. The request's first word must be one of the mentions the guard tool
// rejects in replies and PR descriptions, so the tool and the command
// documents keep out the same mentions.
func (c *contextReviewers) checkRequest(value string) {
	request, _, found := strings.Cut(value, valueSeparator)
	m := awaitedShape.FindStringSubmatch(request)
	if !found || m == nil {
		c.problems = append(c.problems, problemRequestShape)
		return
	}
	if words := strings.Fields(m[1]); len(words) == 0 || !slices.Contains(fixprguard.RejectedMentions(), words[0]) {
		c.problems = append(c.problems, problemRequestMention)
		return
	}
	c.request = m[1]
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
// automated reviewer's name is written: its reviewer rows parse and hold the
// expected names, its request and rejected mentions match fixprguard, and
// fixpr.md, runpr.md, mkreq.md, and _lib/task-pr-handoff.md never write a
// reviewer's name.
func TestContextAndCommandsAgree(t *testing.T) {
	got := checkContextReviewers(doctestutil.ReadDoc(t, contextPath))
	if len(got.problems) > 0 {
		t.Fatalf("_context.md reviewer rows have problems %v", got.problems)
	}
	if got.awaited != "coderabbitai" {
		t.Errorf("_context.md %s = %q, want coderabbitai", awaitedRow, got.awaited)
	}
	if got.request != "@coderabbitai review" {
		t.Errorf("_context.md %s = %q, want %q", requestRow, got.request, "@coderabbitai review")
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
		for _, name := range append(slices.Clone(got.names), got.awaited) {
			if strings.Contains(doc, name) {
				t.Errorf("%s writes the reviewer name %q", tc.name, name)
			}
		}
	}
}

// TestContextReviewerRowsMalformed checks that checkContextReviewers reports
// each kind of broken reviewer row, on copies of the real _context.md with one
// row edited, and that it reads the expected names from a description holding
// backquoted code and a second separator.
func TestContextReviewerRowsMalformed(t *testing.T) {
	doc := doctestutil.ReadDoc(t, contextPath)
	listLine := contextLine(t, doc, reviewerListRow)
	awaitedLine := contextLine(t, doc, awaitedRow)
	requestLine := contextLine(t, doc, requestRow)
	rejectedLine := contextLine(t, doc, rejectedMentionsRow)
	listRow := func(value string) string { return "| " + reviewerListRow + " | " + value + " |" }
	awaitedRowLine := func(value string) string { return "| " + awaitedRow + " | " + value + " |" }
	requestRowLine := func(value string) string { return "| " + requestRow + " | " + value + " |" }
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
			listRow("`coderabbitai` — named in `[bot]`-less form — see `fixpr.md`"), nil,
		},
		{"list_row_missing", listLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"awaited_row_missing", awaitedLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"rejected_row_missing", rejectedLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"list_row_twice", listLine, listLine + "\n" + listLine, []reviewerRowProblem{problemRowCount}},
		{"awaited_row_twice", awaitedLine, awaitedLine + "\n" + awaitedLine, []reviewerRowProblem{problemRowCount}},
		{"rejected_row_twice", rejectedLine, rejectedLine + "\n" + rejectedLine, []reviewerRowProblem{problemRowCount}},
		{"old_row_left", awaitedLine, awaitedLine + "\n| " + oldReviewerRow + " | `coderabbitai` — the reviewer |", []reviewerRowProblem{problemRowCount}},
		{"list_no_value_cell", listLine, "| " + reviewerListRow + " |", []reviewerRowProblem{problemListShape}},
		{"list_value_only_description", listLine, listRow(desc), []reviewerRowProblem{problemListShape}},
		{"no_separator", listLine, listRow("`coderabbitai`"), []reviewerRowProblem{problemListShape}},
		{"no_backquotes", listLine, listRow("coderabbitai" + desc), []reviewerRowProblem{problemListShape}},
		{"no_space_after_comma", listLine, listRow("`coderabbitai`,`other-bot`" + desc), []reviewerRowProblem{problemListShape}},
		{"two_spaces_after_comma", listLine, listRow("`coderabbitai`,  `other-bot`" + desc), []reviewerRowProblem{problemListShape}},
		{"trailing_separator", listLine, listRow("`coderabbitai`," + desc), []reviewerRowProblem{problemListShape}},
		{"name_outside_grammar", listLine, listRow("`coderabbitai[bot]`" + desc), []reviewerRowProblem{problemListParse}},
		{"name_twice", listLine, listRow("`coderabbitai`, `coderabbitai`" + desc), []reviewerRowProblem{problemListParse}},
		{"second_name", listLine, listRow("`coderabbitai`, `other-bot`" + desc), []reviewerRowProblem{problemListNames}},
		{"name_replaced", listLine, listRow("`other-bot`" + desc), []reviewerRowProblem{problemListNames}},
		{"awaited_two_names", awaitedLine, awaitedRowLine("`coderabbitai`, `other-bot`" + desc), []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_no_value_cell", awaitedLine, "| " + awaitedRow + " |", []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_value_only_description", awaitedLine, awaitedRowLine(desc), []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_no_separator", awaitedLine, awaitedRowLine("`coderabbitai`"), []reviewerRowProblem{problemAwaitedShape}},
		{"awaited_outside_grammar", awaitedLine, awaitedRowLine("`coderabbitai[bot]`" + desc), []reviewerRowProblem{problemAwaitedParse}},
		{"request_row_missing", requestLine + "\n", "", []reviewerRowProblem{problemRowCount}},
		{"request_row_twice", requestLine, requestLine + "\n" + requestLine, []reviewerRowProblem{problemRowCount}},
		{"request_two_values", requestLine, requestRowLine("`@coderabbitai review`, `@coderabbitai`" + desc), []reviewerRowProblem{problemRequestShape}},
		{"request_no_backquotes", requestLine, requestRowLine("@coderabbitai review" + desc), []reviewerRowProblem{problemRequestShape}},
		{"request_no_separator", requestLine, requestRowLine("`@coderabbitai review`"), []reviewerRowProblem{problemRequestShape}},
		{"request_other_mention", requestLine, requestRowLine("`@other-bot review`" + desc), []reviewerRowProblem{problemRequestMention}},
		{"request_longer_mention", requestLine, requestRowLine("`@coderabbitai-bot review`" + desc), []reviewerRowProblem{problemRequestMention}},
		{"request_mention_not_first", requestLine, requestRowLine("`review @coderabbitai`" + desc), []reviewerRowProblem{problemRequestMention}},
		{"rejected_no_value_cell", rejectedLine, "| " + rejectedMentionsRow + " |", []reviewerRowProblem{problemRejectedShape}},
		{"rejected_no_separator", rejectedLine, rejectedRowLine("`@codex`, `@coderabbitai`"), []reviewerRowProblem{problemRejectedShape}},
		{"rejected_no_backquotes", rejectedLine, rejectedRowLine("@codex, @coderabbitai" + desc), []reviewerRowProblem{problemRejectedShape}},
		{"rejected_missing_one", rejectedLine, rejectedRowLine("`@codex`" + desc), []reviewerRowProblem{problemRejectedNames}},
		{"rejected_extra_one", rejectedLine, rejectedRowLine("`@codex`, `@coderabbitai`, `@other`" + desc), []reviewerRowProblem{problemRejectedNames}},
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
	// chaining, or stdout tee.
	ghBodySafeRedirectRE = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)*gh\s+[^|;&\n]+--json(?:=|\s+)\S*\bbody\b[^|;&\n]*\s+1?>\s*(?:"<scratchpad>/[^"\s]+"|<scratchpad>/[^\s"']+)\s*$`)
)

// extractGHBodyCalls collects every gh ... --json ... body command from all
// fenced code blocks (joining backslash continuations) and all inline code
// spans in doc (folding soft-wrapped newlines to spaces). Each command line is
// kept whole without splitting on ;, &&, or || so a chained leak after a
// redirect is caught as unsafe.
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
		if ghBodyCommandRE.MatchString(raw) {
			calls = append(calls, raw)
		}
	}
	return calls
}

// checkHarnessAntigravityPhase6Redirect verifies that doc contains at least
// one gh ... --json ... body invocation and that every such invocation writes
// directly to a scratchpad file with a single output redirection (ignoring
// angle-bracket placeholders like <number>) and without leaking stdout.
func checkHarnessAntigravityPhase6Redirect(doc string) (int, []string) {
	calls := extractGHBodyCalls(doc)
	var unsafe []string
	for _, call := range calls {
		stripped := leftPlaceholder.ReplaceAllString(call, "")
		if !ghBodySafeRedirectRE.MatchString(call) || strings.Count(stripped, ">") != 1 {
			unsafe = append(unsafe, call)
		}
	}
	return len(calls), unsafe
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
		t.Errorf("harness-antigravity.md has non-redirected gh body read: %q", call)
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
		{"direct_redirect", "```bash\ngh pr view <number> --json title,body > \"<scratchpad>/<run-id>-pr.json\"\n```\n", 1, 0},
		{"equals_json_with_redirect", "```bash\ngh pr view <number> --json=title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 0},
		{"env_prefix_with_redirect", "```bash\nGH_HOST=github.com gh pr view <number> --json title,body > \"<scratchpad>/pr.json\"\n```\n", 1, 0},
		{"multiline_with_redirect", "```bash\ngh pr view <number> \\\n  --json title,body \\\n  > \"<scratchpad>/pr.json\"\n```\n", 1, 0},
		{"missing_redirect", "```bash\ngh pr view <number> --json title,body\n```\n", 1, 1},
		{"equals_json_missing_redirect", "```bash\ngh pr view <number> --json=title,body\n```\n", 1, 1},
		{"env_prefix_missing_redirect", "```bash\nGH_HOST=github.com gh pr view <number> --json title,body\n```\n", 1, 1},
		{"command_sub_missing_redirect", "```bash\nout=$(gh pr view <number> --json title,body)\n```\n", 1, 1},
		{"command_wrapper_missing_redirect", "```bash\ncommand gh pr view <number> --json title,body\n```\n", 1, 1},
		{"chained_cat_leaks_body", "```bash\ngh pr view <number> --json title,body > \"<scratchpad>/pr.json\" && cat \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"unspaced_semicolon_gh", "```bash\necho ok;gh pr view <number> --json title,body\n```\n", 1, 1},
		{"multiline_missing_redirect", "```bash\ngh pr view <number> \\\n  --json title,body\n```\n", 1, 1},
		{"inline_code_missing_redirect", "Run `gh pr view <number> --json title,body` first.\n", 1, 1},
		{"wrapped_inline_code_missing_redirect", "Run `gh pr view <number>\n--json title,body` first.\n", 1, 1},
		{"pipe_to_tee_leaks_stdout", "```bash\ngh pr view <number> --json title,body | tee \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"double_redirect_rejected", "```bash\ngh pr view <number> --json title,body > /dev/stdout > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"unspaced_double_redirect_rejected", "```bash\ngh pr view <number> --json title,body>/dev/stdout > \"<scratchpad>/pr.json\"\n```\n", 1, 1},
		{"redirect_outside_scratchpad", "```bash\ngh pr view <number> --json title,body > /tmp/pr.json\n```\n", 1, 1},
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
