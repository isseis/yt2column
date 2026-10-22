//go:build test

package main

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/fixprguard"
)

// commandsDir is the directory of the command documents, _lib/ included.
const commandsDir = repositoryRoot + "/.claude/commands"

// mentionCheck is the sentence every section that posts text to GitHub states
// before its posting command, so the user's account never posts the mention.
const mentionCheck = "**Mention check.** Before posting, run `grep -qiF -- '<review-mention>' <file>` " +
	"on each file of text to post. Post only when it exits `1` for every file: exit code `0` means " +
	"the text contains the mention, and any other code means the check failed. Never rewrite the " +
	"text to get past the check."

// postingSection is a section of a command document that posts text to
// GitHub: where it is, the posting commands it runs, and what it does when
// the text holds the mention.
type postingSection struct {
	name     string
	path     string
	section  func(t *testing.T, doc string) string
	commands []string
	handling []string
}

// headingSection returns a section finder for the section under heading.
func headingSection(heading string) func(t *testing.T, doc string) string {
	return func(t *testing.T, doc string) string {
		t.Helper()
		return requireSection(t, heading, doc, heading)
	}
}

// itemSection returns a section finder for the list item that starts with
// first and ends where next starts.
func itemSection(first, next string) func(t *testing.T, doc string) string {
	return func(t *testing.T, doc string) string {
		t.Helper()
		at := strings.Index(doc, first)
		if at < 0 {
			t.Fatalf("no item %q", first)
		}
		end := strings.Index(doc[at:], next)
		if end < 0 {
			t.Fatalf("item %q is not followed by %q", first, next)
		}
		return doc[at : at+end]
	}
}

// postingSections are the seven sections that post text to GitHub, each with
// the handling the architecture assigns it when the text holds the mention.
var postingSections = []postingSection{
	{
		"fixpr.md Phase 5", fixprPath, headingSection(fixprPhase5),
		[]string{"fixprguard reply "},
		[]string{"When the check fails, do not run `reply` for that thread: list it under \"Skipped threads\", " +
			"as for an empty `replyBody`, and go on with the next thread."},
	},
	{
		"fixpr.md Phase 6", fixprPath, headingSection(fixprPhase6),
		[]string{"fixprguard edit-pr "},
		[]string{
			"and run the mention check on both files.",
			"When the check fails, do not run `edit-pr`: leave the PR description as it is, " +
				"and list it in the Final report.",
		},
	},
	{
		"runpr.md step 1", runprPath, headingSection(runprStep1),
		[]string{"gh pr create "},
		[]string{"When the check fails, do not create the PR: stop and report."},
	},
	{
		"runpr.md step 4", runprPath, headingSection(runprStep4),
		[]string{"gh issue comment <issue> --body-file", "gh issue create ", "fixprguard reply "},
		[]string{
			"Write the issue comment or body to a file outside the work tree, and run the mention check " +
				"on it before posting it",
			"When the check fails, do not post: run `mergepr discard --state <state-file>`, stop, and " +
				"report, without merging.",
			"write the reply to a file outside the work tree, run the mention check on it, and only when " +
				"it passes run",
			"If `reply` stops for any thread, or a reply fails the mention check, do not merge",
		},
	},
	{
		"runpr.md step 5", runprPath, headingSection(runprStep5),
		[]string{"gh issue close "},
		[]string{"When the check fails, do not close the issue without its comment: stop and report."},
	},
	{
		"_lib/task-pr-handoff.md Part A step 4", handoffPath, headingSection(handoffLinkIssue),
		[]string{"gh issue create ", "gh api -X POST "},
		[]string{"When the check fails, do not create the link sub-issue: stop and report."},
	},
	{
		"_lib/task-pr-handoff.md Part B step 3", handoffPath,
		itemSection("3. Push and create the PR for the current branch.", "\n4. Run AFTER_CREATE"),
		[]string{"gh pr create "},
		[]string{"When the check fails, do not create the PR: stop and report."},
	},
}

// Section headings of runpr.md and _lib/task-pr-handoff.md that only these
// tests read.
const (
	runprStep5       = "## 5. Switch branch, then run or show NEXT"
	handoffLinkIssue = "### Choosing the issue to link"
)

// TestCodexMentionOnlyInRequestRow checks that the mention appears, in any
// letter case, in no command document and not in CLAUDE.md, except on
// _context.md's "PR review request" row: a document that wrote it would lead
// a session to post it from the user's account.
func TestCodexMentionOnlyInRequestRow(t *testing.T) {
	rows := 0
	for _, path := range append(commandDocs(t), claudePath) {
		for i, line := range strings.Split(readDoc(t, path), "\n") {
			if !strings.Contains(strings.ToLower(line), strings.ToLower(fixprguard.CodexMention)) {
				continue
			}
			if filepath.Clean(path) == filepath.Clean(contextPath) && strings.HasPrefix(line, "| "+requestRow+" | ") {
				rows++
				continue
			}
			t.Errorf("%s:%d writes the mention outside _context.md's %q row", path, i+1, requestRow)
		}
	}
	if rows != 1 {
		t.Errorf("_context.md's %q row holds the mention on %d lines, want 1", requestRow, rows)
	}
}

// TestPostingPathsCheckMention checks that each section that posts text to
// GitHub states the mention check, runs its posting commands after it, and
// says what it does when the text holds the mention. Whether any invocation
// comes before the check is TestPostingCommandsInventory's to judge.
func TestPostingPathsCheckMention(t *testing.T) {
	for _, ps := range postingSections {
		t.Run(ps.name, func(t *testing.T) {
			section := flat(ps.section(t, readDoc(t, ps.path)))
			check := strings.Index(section, flat(mentionCheck))
			if check < 0 {
				t.Fatalf("%s does not state the mention check", ps.name)
			}
			for _, command := range ps.commands {
				if !strings.Contains(section[check:], command) {
					t.Errorf("%s does not run %q after the mention check", ps.name, command)
				}
			}
			requirePhrases(t, ps.name, section, ps.handling...)
		})
	}
}

// requestExemption is how the documents that post the PR review request say
// it is the one text exempt from the mention check.
const requestExemption = "The request is the one text a command posts that contains the mention, " +
	"so the mention check does not apply to it."

// TestRequestExemptFromMentionCheck checks that both documents that post the
// PR review request say why the mention check does not apply to it.
func TestRequestExemptFromMentionCheck(t *testing.T) {
	requirePhrases(t, "runpr.md step 3", requireSection(t, "runpr.md", readDoc(t, runprPath), runprStep3), requestExemption)
	requirePhrases(t, "_lib/task-pr-handoff.md Part B step 5", handoffStep5(t), requestExemption)
}

var (
	// postingCommand matches a command that can write text to GitHub.
	postingCommand = regexp.MustCompile(`gh (issue (create|comment|close|edit)|pr (comment|create|edit|review|merge)|api)\b|fixprguard (reply|edit-pr)\b`)
	// textFlag marks a match as an invocation: a sentence that only names a
	// command passes none of these.
	textFlag = regexp.MustCompile(`(^|\s)(--(body|body-file|title|title-file|subject|comment|protocol|field|raw-field|input)|-[bFtcf])(\s|=|$)`)
	// readMethod marks a gh api invocation as a read: with a text flag and no
	// method, gh api sends a POST.
	readMethod = regexp.MustCompile(`(?i)(-X|--method)[ =]?GET\b`)
)

// readOnlyInvocations are the gh api invocations inside a posting section that
// pass a text flag but write nothing: the link count query. Its query follows
// on later lines, so the code block it opens must hold no mutation.
var readOnlyInvocations = []string{
	`gh api graphql -F owner='{owner}' -F repo='{repo}' -F n=<issue> -f query='`,
}

// invocation returns the command line that starts at doc[at:], with the
// backslash continuations joined, cut at the end of a backquoted span.
func invocation(doc string, at int) string {
	var b strings.Builder
	rest := doc[at:]
	for {
		line, next, _ := strings.Cut(rest, "\n")
		cont, isCont := strings.CutSuffix(line, "\\")
		b.WriteString(cont)
		if !isCont || next == "" {
			break
		}
		rest = next
	}
	command, _, _ := strings.Cut(b.String(), "`")
	return command
}

// isReadOnly reports whether command, starting at doc[at:], is one of
// readOnlyInvocations with no mutation in the rest of its code block.
func isReadOnly(doc string, at int, command string) bool {
	if !slices.ContainsFunc(readOnlyInvocations, func(c string) bool { return strings.HasPrefix(command, c) }) {
		return false
	}
	block, _, _ := strings.Cut(doc[at:], "```")
	return !strings.Contains(block, "mutation")
}

// isPost reports whether command, an invocation postingCommand matched,
// writes text to GitHub: it passes a text flag, and a gh api call does not
// ask for GET.
func isPost(command string) bool {
	if !textFlag.MatchString(command) {
		return false
	}
	if strings.HasPrefix(command, "gh api") {
		return !readMethod.MatchString(command)
	}
	return true
}

// TestPostingCommandsInventory checks every invocation of a posting command in
// the command documents: it must be a PR review request, which is the one text
// that holds the mention, or the squash merge in mergepr.md, which is not a
// comment, or one of a posting section's listed commands, after that
// section's mention check. A new posting command added without the check
// fails here, inside a posting section too. A post the documents describe in
// prose, without a command, is out of its reach.
func TestPostingCommandsInventory(t *testing.T) {
	// span is a posting section's place in its document: where it starts,
	// where its first mention check starts, and where it ends.
	type span struct {
		start, check, end int
		commands          []string
	}
	sections := map[string][]span{}
	for _, ps := range postingSections {
		doc := readDoc(t, ps.path)
		section := ps.section(t, doc)
		at := strings.Index(doc, section)
		check := strings.Index(section, "**Mention check.**")
		if check < 0 {
			t.Fatalf("%s has no mention check", ps.name)
		}
		key := filepath.Clean(ps.path)
		sections[key] = append(sections[key], span{at, at + check, at + len(section), ps.commands})
	}

	posts := 0
	for _, path := range commandDocs(t) {
		doc := readDoc(t, path)
		key := filepath.Clean(path)
		for _, m := range postingCommand.FindAllStringIndex(doc, -1) {
			command := invocation(doc, m[0])
			if !isPost(command) || strings.HasPrefix(command, `gh pr comment <number> --body "<review-request>"`) ||
				isReadOnly(doc, m[0], command) ||
				(filepath.Base(path) == "mergepr.md" && strings.HasPrefix(command, "gh pr merge")) {
				continue
			}
			posts++
			line := strings.Count(doc[:m[0]], "\n") + 1
			where := "outside the sections that run the mention check"
			for _, s := range sections[key] {
				switch {
				case m[0] < s.start || s.end <= m[0]:
				case m[0] < s.check:
					where = "before its section's mention check"
				case !slices.ContainsFunc(s.commands, func(c string) bool { return strings.HasPrefix(command, c) }):
					where = "in a posting section that does not list it"
				default:
					where = ""
				}
			}
			if where != "" {
				t.Errorf("%s:%d posts with %q %s", path, line, command, where)
			}
		}
	}
	// A matcher that stopped matching would pass every document.
	if posts == 0 {
		t.Error("found no posting command in the command documents")
	}
}

// TestIsPost checks the classifier the inventory relies on, on invocations
// written in each form a command document may use, so a change to the
// matchers that stops recognizing one form fails here even while the real
// documents still hold other posts.
func TestIsPost(t *testing.T) {
	for _, tc := range []struct {
		name, doc string
		want      bool
	}{
		{"long_body_file", "gh pr comment <n> --body-file <f>", true},
		{"short_body", `gh pr comment <n> -b "$(cat <f>)"`, true},
		{"short_body_file", "gh pr comment <n> -F <f>", true},
		{"continuation", "gh issue comment <n> \\\n  --body-file <f>", true},
		{"api_explicit_post", "gh api -X POST repos/o/r/issues/1/comments -f body=x", true},
		{"api_implicit_post", "gh api repos/o/r/issues/1/comments -f body=x", true},
		{"graphql_mutation", "gh api graphql -f query='mutation { x }'", true},
		{"review_body", "gh pr review <n> -b x", true},
		{"merge_subject", "gh pr merge <n> --subject x", true},
		{"guard_reply", "fixprguard reply --protocol 3 <n>", true},
		{"name_only", "gh issue comment`, and", false},
		{"api_read", "gh api --paginate repos/o/r/issues/1/comments --jq '.[]'", false},
		{"api_get_with_field", "gh api -X GET repos/o/r/issues -f state=open", false},
		{"backquote_ends_command", "gh pr comment` (the request) --body-file", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPost(invocation(tc.doc, 0)); got != tc.want {
				t.Errorf("isPost(%q) = %v, want %v", invocation(tc.doc, 0), got, tc.want)
			}
		})
	}
}

// Paths of the files only the PR-review guards read.
const (
	escalationPath   = commandsDir + "/_lib/pr-review-escalation.md"
	agentsPath       = repositoryRoot + "/AGENTS.md"
	coderabbitPath   = repositoryRoot + "/.coderabbit.yaml"
	observationsPath = repositoryRoot + "/docs/tasks/0031_codex_pr_review/observations.md"
)

// commandDocs returns the paths of every command document, _lib/ included.
func commandDocs(t *testing.T) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(commandsDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk %s: %v", commandsDir, err)
	}
	return paths
}

// removedPRReviewPhrases are the phrases that described CodeRabbit as the PR
// reviewer before the switch to Codex. The local CLI review keeps the name
// CodeRabbit, so the name alone is not forbidden.
var removedPRReviewPhrases = []string{
	"@coderabbitai review",
	"CodeRabbit PR review",
	"CodeRabbit's PR review",
	"waits for CodeRabbit and CI",
	"Drive a PR through CodeRabbit",
	"the PR-side CodeRabbit review",
	"CodeRabbit has not reviewed",
	"ask CodeRabbit to review",
	"CodeRabbit is still `PENDING`",
	"threads CodeRabbit still posts",
	"the CodeRabbit request",
	"CodeRabbit's reviews and comments",
	"Make sure CodeRabbit reviews the PR",
	"before CodeRabbit is asked",
	"do not wait for CodeRabbit",
	"the CodeRabbit threads",
	"CodeRabbit's last PR review",
}

// TestNoCodeRabbitPRReview checks that no command document and not CLAUDE.md
// still asks CodeRabbit for a PR review, waits for one, or names CodeRabbit as
// the PR reviewer, and that runpr.md no longer judges a review by CodeRabbit's
// status context or its reviewed-range line.
func TestNoCodeRabbitPRReview(t *testing.T) {
	for _, path := range append(commandDocs(t), claudePath) {
		doc := strings.ToLower(flat(readDoc(t, path)))
		for _, phrase := range removedPRReviewPhrases {
			if strings.Contains(doc, strings.ToLower(phrase)) {
				t.Errorf("%s still says %q", path, phrase)
			}
		}
	}
	runpr := readDoc(t, runprPath)
	if strings.Contains(runpr, "StatusContext") {
		t.Error("runpr.md still reads a StatusContext")
	}
	if regexp.MustCompile(`between \[0-9a-f\]\{40\}`).MatchString(runpr) {
		t.Error("runpr.md still searches for CodeRabbit's reviewed range")
	}
}

// TestLocalCodeRabbitReviewKept checks that the local CLI review is still
// called CodeRabbit where the documents run it: the switch moved only the PR
// review to Codex.
func TestLocalCodeRabbitReviewKept(t *testing.T) {
	requireSection(t, "fixpr.md", readDoc(t, fixprPath), "## Phase 4.5 — Local CodeRabbit review of the fix (model: default)")
	requirePhrases(t, "_lib/task-pr-handoff.md Part B step 1", readDoc(t, handoffPath),
		"1. When LOCAL_REVIEW is `run`, review the branch with the local CodeRabbit CLI:")
	requirePhrases(t, "CLAUDE.md", readDoc(t, claudePath),
		"`coderabbit review` (the local CodeRabbit CLI,",
		"Later changes are reviewed with the local CodeRabbit CLI",
	)
}

// description returns the description line of a command document's front
// matter.
func description(t *testing.T, path string) string {
	t.Helper()
	doc := readDoc(t, path)
	rest, ok := strings.CutPrefix(doc, "---\n")
	front, _, closed := strings.Cut(rest, "\n---\n")
	if !ok || !closed {
		t.Fatalf("%s has no front matter", path)
	}
	for line := range strings.Lines(front) {
		if value, ok := strings.CutPrefix(line, "description: "); ok {
			return strings.TrimSpace(value)
		}
	}
	t.Fatalf("%s has no description", path)
	return ""
}

// TestDocsDescribeCodexReview checks that the documents that describe the PR
// review name Codex as the reviewer, and that CLAUDE.md records the Codex
// settings the user keeps outside the repository: a review only when a PR is
// created, with the thorough code review off.
func TestDocsDescribeCodexReview(t *testing.T) {
	requirePhrases(t, "runpr.md description", description(t, runprPath), "Drive a PR through Codex's PR review and CI")
	requirePhrases(t, "_lib/pr-review-escalation.md description", description(t, escalationPath),
		"when to request a further Codex PR review after the first")
	handoff := readDoc(t, handoffPath)
	opening, _, _ := strings.Cut(handoff, "\n## ")
	requirePhrases(t, "_lib/task-pr-handoff.md opening", opening, "It waits for Codex's PR review and CI, resolves the threads")
	_, step7, found := strings.Cut(handoff, "\n7. In REPORT, give the PR URL")
	if !found {
		t.Fatal("_lib/task-pr-handoff.md has no Part B step 7")
	}
	requirePhrases(t, "_lib/task-pr-handoff.md Part B step 7", step7, "Then stop: do not wait for Codex and do not merge.")
	requirePhrases(t, "CLAUDE.md", readDoc(t, claudePath),
		"`/runpr [PR | branch | task]` - Drive a PR through Codex's PR review and CI",
		"it waits for Codex's PR review and CI and resolves the threads with `/fixpr`",
		"Codex is set up outside the repository, by the user: it reviews automatically only when a PR "+
			"is created, never on a push, and its thorough code review is off.",
	)
}

// autoReviewDisabled reports whether the .coderabbit.yaml text sets
// reviews.auto_review.enabled to false, with exactly one auto_review under
// reviews. The standard library has no YAML parser, so it reads the nesting
// from the indentation and skips comment lines; it accepts only the block
// style the file uses.
func autoReviewDisabled(text string) bool {
	type key struct {
		indent int
		name   string
	}
	var stack []key
	autoReviews, enabled := 0, []string{}
	for line := range strings.Lines(text) {
		line = strings.TrimRight(line, "\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		var path []string
		for _, k := range stack {
			path = append(path, k.name)
		}
		path = append(path, name)
		stack = append(stack, key{indent, name})
		value, _, _ = strings.Cut(value, " #")
		switch strings.Join(path, ".") {
		case "reviews.auto_review":
			autoReviews++
		case "reviews.auto_review.enabled":
			enabled = append(enabled, strings.TrimSpace(value))
		}
	}
	return autoReviews == 1 && len(enabled) == 1 && enabled[0] == "false"
}

// TestCodeRabbitConfigStopsReview checks that .coderabbit.yaml turns
// CodeRabbit's automatic PR review off and says why, and that
// autoReviewDisabled tells the real file from copies broken in one place.
func TestCodeRabbitConfigStopsReview(t *testing.T) {
	doc := readDoc(t, coderabbitPath)
	requirePhrases(t, ".coderabbit.yaml", doc,
		"CodeRabbit does not review PRs: Codex does the PR review",
		"The local CLI review (`coderabbit review`) is still used",
	)
	// Each broken copy keeps a reviews.auto_review key, so only where
	// enabled sits decides the result.
	const enabled = "  auto_review:\n    enabled: false\n"
	const premerge = "  pre_merge_checks:\n"
	for _, tc := range []struct {
		name, old, new string
		want           bool
	}{
		{"real", enabled, enabled, true},
		{"commented_out", enabled, "  auto_review:\n    drafts: false\n    # enabled: false\n", false},
		{"enabled_true", enabled, "  auto_review:\n    enabled: true\n", false},
		{"directly_under_reviews", enabled, "  auto_review:\n    drafts: false\n  enabled: false\n", false},
		{"under_pre_merge_checks", enabled + premerge, "  auto_review:\n    drafts: false\n" + premerge + "    auto_review:\n      enabled: false\n", false},
		{"auto_review_twice", enabled, enabled + enabled, false},
		// Each of these rows only one mechanism of the helper decides: the
		// count of auto_review keys, the inline-comment strip, and the
		// comment-line skip.
		{"second_auto_review_without_enabled", enabled, enabled + "  auto_review:\n    drafts: true\n", false},
		{"inline_comment", enabled, "  auto_review:\n    enabled: false # PR review is Codex's\n", true},
		{"comment_line_between", enabled, "  auto_review:\n# note: kept\n    enabled: false\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Count(doc, tc.old) != 1 {
				t.Fatalf(".coderabbit.yaml holds %q %d times, want once", tc.old, strings.Count(doc, tc.old))
			}
			broken := strings.Replace(doc, tc.old, tc.new, 1)
			if got := autoReviewDisabled(broken); got != tc.want {
				t.Errorf("autoReviewDisabled = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAgentsCodeReviewRules checks that AGENTS.md holds the section Codex reads
// its review rules from, under the heading its documentation names, and that
// the section refers to CLAUDE.md rather than copying it.
func TestAgentsCodeReviewRules(t *testing.T) {
	doc := readDoc(t, agentsPath)
	const heading = "## Code Review Rules"
	if n := strings.Count(doc, "\n"+heading+"\n"); n != 1 {
		t.Errorf("AGENTS.md has %d %q headings, want 1", n, heading)
	}
	if strings.Contains(doc, "## Review guidelines") {
		t.Error("AGENTS.md still has the old \"## Review guidelines\" heading")
	}
	// The reference is the section's opening, before its rules.
	const opening = "Besides the rules below, follow the conventions, design principles, and testing rules in " +
		"[CLAUDE.md](CLAUDE.md); they are not repeated here."
	if !strings.HasPrefix(flat(sectionByHeading(doc, heading)), opening) {
		t.Errorf("AGENTS.md %q does not open with %q", heading, opening)
	}
}

var (
	// observationDate and observationBasis are what every observation item
	// must carry: a date, and a PR, an API call, or a URL as its basis.
	observationDate  = regexp.MustCompile(`2026-\d\d-\d\d`)
	observationBasis = regexp.MustCompile(`PR #\d+|gh api|https://`)
	prNumber         = regexp.MustCompile(`PR #\d+`)
	hexRun           = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
)

// observationResults are, for each item of the observation record, a
// sentence that states its result. The literals match the Japanese record, so
// they are the exception to the English-only literal rule.
var observationResults = map[string]string{
	"### 1.": "**完了の印の候補:**",
	"### 2.": "再レビューはなかった",
	"### 3.": "REST の名前は、GraphQL の名前に `[bot]` を付けた形である",
	"### 4.": "Codex のステータスコンテキストもチェックランも付かなかった",
	"### 5.": "どれも上限に達しなかった",
	"### 6.": "見出しは `## Code Review Rules` である",
	"### 7.": "| 約 1 分 10 秒 |",
	"### 8.": "では、Codex は実行されず、利用者の割り当ても消費されない。Codex は断りのコメントを返す。",
	"### 9.": "レビューのタイミングは「PR 作成時のみ」、徹底的なコードレビューは無効",
}

// observationBlocks splits an item of the observation record into its
// observations: each top-level bullet with its sub-bullets, and each other
// line on its own.
func observationBlocks(item string) []string {
	var blocks []string
	for line := range strings.Lines(item) {
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, " ") && len(blocks) > 0:
			blocks[len(blocks)-1] += line
		default:
			blocks = append(blocks, line)
		}
	}
	return blocks
}

// hasCommitSHA reports whether block names a commit: a run of 7 to 40
// hexadecimal digits with a letter in it, so an all-digit ID is not taken
// for one.
func hasCommitSHA(block string) bool {
	for _, run := range hexRun.FindAllString(block, -1) {
		if strings.ContainsAny(run, "abcdef") {
			return true
		}
	}
	return false
}

// TestCodexObservationRecord checks that the observation record the design
// was based on holds what the requirements ask of it: each of the nine items
// with a date, a basis, and its result; each observation on a PR with the PR
// and its requested commit, or why that commit cannot be told; the four author
// values of a thread; both a review with findings and one without; and the
// section on what would have stopped the design.
func TestCodexObservationRecord(t *testing.T) {
	doc := readDoc(t, observationsPath)
	items := sectionByHeading(doc, "## AC-01 の各項目")
	itemSection := func(prefix string) string {
		for line := range strings.Lines(items) {
			if strings.HasPrefix(line, prefix+" ") {
				return sectionByHeading(items, strings.TrimSpace(line))
			}
		}
		return ""
	}
	prObservations := 0
	for prefix, result := range observationResults {
		item := itemSection(prefix)
		if strings.TrimSpace(item) == "" {
			t.Errorf("observations.md has no item %q", prefix)
			continue
		}
		if !observationDate.MatchString(item) || !observationBasis.MatchString(item) {
			t.Errorf("observations.md item %q lacks a date or a basis", prefix)
		}
		requirePhrases(t, "observations.md item "+prefix, item, result)
		for _, block := range observationBlocks(item) {
			if !prNumber.MatchString(block) {
				continue
			}
			prObservations++
			if !hasCommitSHA(block) && !strings.Contains(block, "決められない") {
				t.Errorf("observations.md item %q observes a PR without its requested commit: %q", prefix, strings.TrimSpace(block))
			}
		}
	}
	// A record whose PR numbers stopped matching would pass the loop above.
	if prObservations == 0 {
		t.Error("observations.md has no observation on a PR")
	}
	requirePhrases(t, "observations.md item 3", itemSection("### 3."),
		"`__typename`: `Bot`", "`login`: `chatgpt-codex-connector`",
		"`type`: `Bot`", "`login`: `chatgpt-codex-connector[bot]`",
	)
	requirePhrases(t, "observations.md item 1", itemSection("### 1."),
		"PR レビュー（`chatgpt-codex-connector[bot]`、`state: COMMENTED`）が投稿された",
		"**指摘が 0 件のとき（2026-10-21、PR #413）:** PR レビューは投稿されなかった",
	)
	requireSection(t, "observations.md", doc, "## AC-05 に当たる点")
}
