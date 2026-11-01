//go:build test

package main

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// mentionCheckCommand is the command every section that posts text to GitHub
// runs in its mention check before its posting command, so the user's account
// never posts the mention. The guard tool checks the text only for reply and
// edit-pr, so on every other posting path this command is what stops it.
const mentionCheckCommand = "`grep -qiF -- '<review-mention>' <file>`"

// postingSection is a section of a command document that posts text to
// GitHub: where it is and the posting commands it runs.
type postingSection struct {
	name     string
	path     string
	section  func(t *testing.T, doc string) string
	commands []string
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

// postingSections are the seven sections that post text to GitHub.
var postingSections = []postingSection{
	{"fixpr.md Phase 5", fixprPath, headingSection(fixprPhase5), []string{"fixprguard reply "}},
	{"fixpr.md Phase 6", fixprPath, headingSection(fixprPhase6), []string{"fixprguard edit-pr "}},
	{"runpr.md step 1", runprPath, headingSection(runprStep1), []string{"gh pr create "}},
	{
		"runpr.md step 4", runprPath, headingSection(runprStep4),
		[]string{"gh issue comment <issue> --body-file", "gh issue create ", "fixprguard reply "},
	},
	{"runpr.md step 5", runprPath, headingSection(runprStep5), []string{"gh issue close "}},
	{
		"_lib/task-pr-handoff.md Part A step 4", handoffPath, headingSection(handoffLinkIssue),
		[]string{"gh issue create ", "gh api -X POST "},
	},
	{
		"_lib/task-pr-handoff.md Part B step 3", handoffPath,
		itemSection("3. Push and create the PR for the current branch.", "\n4. Run AFTER_CREATE"),
		[]string{"gh pr create "},
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
	var paths []string
	for _, rel := range doctestutil.CommandDocs(t, repositoryRoot) {
		paths = append(paths, filepath.Join(repositoryRoot, rel))
	}
	rows := 0
	for _, path := range append(paths, claudePath) {
		for i, line := range strings.Split(doctestutil.ReadDoc(t, path), "\n") {
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
// GitHub runs the mention check's command: the first sentence after its
// "**Mention check.**" label holds mentionCheckCommand. Whether any posting
// command comes before the check is TestPostingCommandsInventory's to judge.
func TestPostingPathsCheckMention(t *testing.T) {
	const label = "**Mention check.**"
	for _, ps := range postingSections {
		t.Run(ps.name, func(t *testing.T) {
			_, rest, found := strings.Cut(flat(ps.section(t, doctestutil.ReadDoc(t, ps.path))), label)
			if !found {
				t.Fatalf("%s has no %s", ps.name, label)
			}
			if sentence, _, _ := strings.Cut(rest, ". "); !strings.Contains(sentence, mentionCheckCommand) {
				t.Errorf("%s %s does not run %s: %q", ps.name, label, mentionCheckCommand, sentence)
			}
		})
	}
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
		doc := doctestutil.ReadDoc(t, ps.path)
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
	for _, rel := range doctestutil.CommandDocs(t, repositoryRoot) {
		path := filepath.Join(repositoryRoot, rel)
		doc := doctestutil.ReadDoc(t, path)
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
	agentsPath     = repositoryRoot + "/AGENTS.md"
	coderabbitPath = repositoryRoot + "/.coderabbit.yaml"
)

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
// CodeRabbit's automatic PR review off, and that autoReviewDisabled tells the
// real file from copies broken in one place.
func TestCodeRabbitConfigStopsReview(t *testing.T) {
	doc := doctestutil.ReadDoc(t, coderabbitPath)
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

// TestAgentsCodeReviewRules checks that AGENTS.md holds exactly one heading of
// the section Codex reads its review rules from.
func TestAgentsCodeReviewRules(t *testing.T) {
	doc := doctestutil.ReadDoc(t, agentsPath)
	const heading = "## Code Review Rules"
	if n := strings.Count(doc, "\n"+heading+"\n"); n != 1 {
		t.Errorf("AGENTS.md has %d %q headings, want 1", n, heading)
	}
}
