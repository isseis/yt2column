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
	runprStep5       = "## 5. Switch branch, then run NEXT"
	handoffLinkIssue = "### Choosing the issue to link"
)

// TestCodexMentionOnlyInRequestRow checks that the mention appears, in any
// letter case, in no command document and not in CLAUDE.md, except on
// _context.md's "PR review request" row: a document that wrote it would lead
// a session to post it from the user's account.
func TestCodexMentionOnlyInRequestRow(t *testing.T) {
	paths := []string{claudePath}
	err := filepath.WalkDir(commandsDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk %s: %v", commandsDir, err)
	}
	rows := 0
	for _, path := range paths {
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
	err := filepath.WalkDir(commandsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
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
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", commandsDir, err)
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
