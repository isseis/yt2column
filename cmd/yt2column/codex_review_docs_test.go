//go:build test

package main

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
		[]string{"When the check fails, do not run `edit-pr`: leave the PR description as it is, " +
			"and list it in the Final report."},
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
			"write the issue comment or body to a file outside the work tree, and each thread's reply " +
				"(below) to its own file, and run the mention check on all of them",
			"When the check fails, do not post: run `mergepr discard --state <state-file>`, stop, and " +
				"report, without merging.",
		},
	},
	{
		"runpr.md step 5", runprPath, headingSection(runprStep5),
		[]string{"gh issue close "},
		[]string{"When the check fails, do not close the issue without its comment: stop and report."},
	},
	{
		"_lib/task-pr-handoff.md Part A step 4", handoffPath, headingSection(handoffLinkIssue),
		[]string{"gh issue create "},
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
			if !strings.Contains(strings.ToLower(line), "@codex") {
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
// GitHub states the mention check before its posting commands, and what it
// does when the text holds the mention.
func TestPostingPathsCheckMention(t *testing.T) {
	for _, ps := range postingSections {
		t.Run(ps.name, func(t *testing.T) {
			section := flat(ps.section(t, readDoc(t, ps.path)))
			check := strings.Index(section, flat(mentionCheck))
			if check < 0 {
				t.Fatalf("%s does not state the mention check", ps.name)
			}
			for _, command := range ps.commands {
				at := strings.Index(section, command)
				if at < 0 {
					t.Errorf("%s does not run %q", ps.name, command)
				} else if at < check {
					t.Errorf("%s runs %q before the mention check", ps.name, command)
				}
			}
			requirePhrases(t, ps.name, section, ps.handling...)
		})
	}
}

var (
	// postingCommand matches a command that writes text to GitHub.
	postingCommand = regexp.MustCompile(`gh (issue (create|comment|close|edit)|pr (comment|create|edit|review|merge))\b|fixprguard (reply|edit-pr)\b`)
	// textFlag marks a match as an invocation: a sentence that only names a
	// command passes none of these.
	textFlag = regexp.MustCompile(`--(body|body-file|title|title-file|comment|protocol)\b`)
)

// TestPostingCommandsInventory checks that every invocation of a posting
// command in the command documents lies in one of the sections that run the
// mention check, or is a PR review request, which is the one text that holds
// the mention, or is in mergepr.md, whose squash merge is not a comment. A new
// posting path added without the check fails here.
func TestPostingCommandsInventory(t *testing.T) {
	type span struct{ start, end int }
	sections := map[string][]span{}
	for _, ps := range postingSections {
		doc := readDoc(t, ps.path)
		section := ps.section(t, doc)
		at := strings.Index(doc, section)
		sections[filepath.Clean(ps.path)] = append(sections[filepath.Clean(ps.path)], span{at, at + len(section)})
	}

	err := filepath.WalkDir(commandsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		if filepath.Base(path) == "mergepr.md" {
			return nil
		}
		doc := readDoc(t, path)
		for _, m := range postingCommand.FindAllStringIndex(doc, -1) {
			rest, _, _ := strings.Cut(doc[m[1]:], "\n")
			rest, _, _ = strings.Cut(rest, "`")
			if !textFlag.MatchString(rest) || strings.HasPrefix(rest, ` <number> --body "<review-request>"`) {
				continue
			}
			inSection := false
			for _, s := range sections[filepath.Clean(path)] {
				inSection = inSection || (s.start <= m[0] && m[0] < s.end)
			}
			if !inSection {
				line := strings.Count(doc[:m[0]], "\n") + 1
				t.Errorf("%s:%d posts with %q outside the sections that run the mention check", path, line, doc[m[0]:m[1]])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", commandsDir, err)
	}
}
