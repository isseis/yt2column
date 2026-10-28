//go:build test

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// docRepoRoot is the repository root relative to this package's directory.
const docRepoRoot = "../.."

// docRef is one path a document must resolve: a relative Markdown link
// target, or a repository path written in a code span. relative tells the
// caller which directory the path is written from.
type docRef struct {
	path     string
	relative bool
}

// repoDocPathRE matches a repository path in a code span, with an optional
// line or line-range suffix.
var repoDocPathRE = regexp.MustCompile(`^docs/dev/[A-Za-z0-9_./-]+\.md(?::[0-9]+(?:-[0-9]+)?)?$`)

// lineSuffixRE matches a line or line-range suffix on a repository path.
var lineSuffixRE = regexp.MustCompile(`:[0-9]+(?:-[0-9]+)?$`)

// docPathRefs returns the paths outside fenced code blocks that doc must
// resolve: the target of each relative Markdown link, without its anchor, and
// each path under docs/dev/ written in a code span. A link to an external URL,
// a mailto: link, an anchor in the same document, and a <...> placeholder are
// not paths to a file. A link or a path inside a code block is an example, not
// a reference.
func docPathRefs(doc string) []docRef {
	var refs []docRef
	for _, line := range scanDocLines(slices.Collect(strings.Lines(doc))) {
		if line.block {
			continue
		}
		for _, match := range markdownLinkRE.FindAllStringSubmatch(line.text, -1) {
			target := match[2]
			if !isRelativeLinkTarget(target) {
				continue
			}
			path, _, _ := strings.Cut(target, "#")
			refs = append(refs, docRef{path: path, relative: true})
		}
		for _, span := range inlineCodeSpans(line.text) {
			span = strings.TrimSpace(span)
			if !repoDocPathRE.MatchString(span) {
				continue
			}
			refs = append(refs, docRef{path: lineSuffixRE.ReplaceAllString(span, "")})
		}
	}
	return refs
}

// isRelativeLinkTarget reports whether target is a link to a path relative to
// the linking document.
func isRelativeLinkTarget(target string) bool {
	switch {
	case strings.HasPrefix(target, "#"),
		strings.HasPrefix(target, "<"),
		strings.HasPrefix(target, "mailto:"),
		strings.Contains(target, "://"):
		return false
	}
	return true
}

// inlineCodeSpans returns the contents of the inline code spans of line.
func inlineCodeSpans(line string) []string {
	var spans []string
	for _, match := range codeSpanRE.FindAllStringSubmatch(line, -1) {
		spans = append(spans, match[1])
	}
	return spans
}

// instructionDocs returns the documents that tell an agent what to do, from
// the repository root: CLAUDE.md, AGENTS.md, the command definitions, and the
// documents under docs/dev/. Past task documents are records of the work when
// it was done, so they are not targets.
func instructionDocs(t *testing.T) []string {
	t.Helper()
	docs := []string{"CLAUDE.md", "AGENTS.md"}
	for _, root := range []string{".claude/commands", "docs/dev"} {
		err := filepath.WalkDir(filepath.Join(docRepoRoot, root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			rel, relErr := filepath.Rel(docRepoRoot, path)
			if relErr != nil {
				return relErr
			}
			docs = append(docs, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return docs
}

// TestInstructionDocPaths: every path an instruction document writes resolves.
// A relative link must name a file that exists beside the document, and a
// repository path under docs/dev/ written in a code span must name a file that
// exists from the repository root. Each document's link count is checked
// against the extraction, so a link form the helper drops does not go
// unchecked. A branch created before the paths moved would otherwise keep
// pointing the agent at a document that no longer exists.
func TestInstructionDocPaths(t *testing.T) {
	for _, rel := range instructionDocs(t) {
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(docRepoRoot, rel)
			doc := readDoc(t, path)
			targets := markdownLinkTargets(doc)
			openers := 0
			for _, line := range scanDocLines(slices.Collect(strings.Lines(doc))) {
				if !line.block {
					openers += strings.Count(line.text, "](")
				}
			}
			if len(targets) != openers {
				t.Errorf("%s has %d link openers outside code blocks but the extraction found %d targets; a link form it drops would go unchecked", rel, openers, len(targets))
			}
			refs := docPathRefs(doc)
			for _, ref := range refs {
				base := docRepoRoot
				if ref.relative {
					base = filepath.Dir(path)
				}
				if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(ref.path))); err != nil {
					if ref.relative {
						t.Errorf("%s links to %q, which does not exist", rel, ref.path)
					} else {
						t.Errorf("%s names %q, which does not exist", rel, ref.path)
					}
				}
			}
		})
	}
}

// TestDocPathRefs: the extraction keeps the paths a document must resolve and
// drops the forms that are not references to a file: an external URL, a
// mailto: link, an anchor in the same document, a <...> placeholder, and
// anything inside a code block.
func TestDocPathRefs(t *testing.T) {
	doc := strings.Join([]string{
		"[a](docs/dev/security.md)",
		"[b](../project_overview.md#top)",
		"[c](https://example.com/x.md)",
		"[d](mailto:someone@example.com)",
		"[e](#section)",
		"[f](<url>)",
		"[g](docs/dev/production_profile.md)",
		"`docs/dev/security.md`",
		"`docs/dev/conventions/requirements_process.md:12`",
		"`docs/dev/guides/cli.md:3-5`",
		"`docs/dev/`",
		"```text",
		"[h](docs/dev/missing.md)",
		"`docs/dev/missing.md`",
		"```",
	}, "\n")
	want := []docRef{
		{path: "docs/dev/security.md", relative: true},
		{path: "../project_overview.md", relative: true},
		{path: "docs/dev/production_profile.md", relative: true},
		{path: "docs/dev/security.md"},
		{path: "docs/dev/conventions/requirements_process.md"},
		{path: "docs/dev/guides/cli.md"},
	}
	if got := docPathRefs(doc); !slices.Equal(got, want) {
		t.Errorf("docPathRefs = %v, want %v", got, want)
	}
}
