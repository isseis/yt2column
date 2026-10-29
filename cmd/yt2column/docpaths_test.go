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

	"github.com/isseis/yt2column/internal/doctestutil"
)

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
// a mailto: link, an anchor in the same document, a <...> placeholder, and a
// link or path written inside a code span or a code block are examples, not
// references.
func docPathRefs(doc string) []docRef {
	var refs []docRef
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc))) {
		if line.Block {
			continue
		}
		spans := codeSpanRE.FindAllStringSubmatchIndex(line.Text, -1)
		for _, match := range markdownLinkRE.FindAllStringSubmatchIndex(line.Text, -1) {
			if inCodeSpan(spans, match[0]) {
				continue
			}
			target := line.Text[match[4]:match[5]]
			if !isRelativeLinkTarget(target) {
				continue
			}
			file, _, _ := strings.Cut(target, "#")
			refs = append(refs, docRef{path: file, relative: true})
		}
		for _, span := range spans {
			content := strings.TrimSpace(line.Text[span[0]+1 : span[1]-1])
			if !repoDocPathRE.MatchString(content) {
				continue
			}
			refs = append(refs, docRef{path: lineSuffixRE.ReplaceAllString(content, "")})
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

// inCodeSpan reports whether offset lies inside one of spans, the index
// pairs FindAllStringSubmatchIndex returns for the code spans of a line.
func inCodeSpan(spans [][]int, offset int) bool {
	for _, span := range spans {
		if offset >= span[0] && offset < span[1] {
			return true
		}
	}
	return false
}

// instructionDocs returns the documents that tell an agent what to do, from
// the repository root: CLAUDE.md, AGENTS.md, the command definitions, and the
// documents under docs/dev/. Past task documents are records of the work when
// it was done, so they are not targets.
func instructionDocs(t *testing.T) []string {
	t.Helper()
	docs := []string{"CLAUDE.md", "AGENTS.md"}
	for _, root := range []string{".claude/commands", "docs/dev"} {
		err := filepath.WalkDir(filepath.Join(repositoryRoot, root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			rel, relErr := filepath.Rel(repositoryRoot, path)
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
			path := filepath.Join(repositoryRoot, rel)
			doc := doctestutil.ReadDoc(t, path)
			checkLinkCount(t, rel, doc, len(markdownLinkTargets(doc)))
			for _, ref := range docPathRefs(doc) {
				base := repositoryRoot
				verb := "names"
				if ref.relative {
					base = filepath.Dir(path)
					verb = "links to"
				}
				if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(ref.path))); err != nil {
					t.Errorf("%s %s %q, which does not exist", rel, verb, ref.path)
				}
			}
		})
	}
}

// TestDocPathRefs: the extraction keeps the paths a document must resolve and
// drops the forms that are not references to a file.
func TestDocPathRefs(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want []docRef
	}{
		{
			name: "relative link",
			doc:  "[a](docs/dev/security.md)",
			want: []docRef{{path: "docs/dev/security.md", relative: true}},
		},
		{
			name: "link with an anchor",
			doc:  "[a](../project_overview.md#top)",
			want: []docRef{{path: "../project_overview.md", relative: true}},
		},
		{
			name: "external URL, mailto, anchor and placeholder",
			doc:  "[a](https://example.com/x.md) [b](mailto:someone@example.com) [c](#section) [d](<url>)",
		},
		{
			name: "link written in a code span",
			doc:  "`[a](docs/dev/missing.md)` is an example",
		},
		{
			name: "fenced code block",
			doc:  "```text\n[a](docs/dev/missing.md)\n`docs/dev/missing.md`\n```",
		},
		{
			name: "repository path in a code span",
			doc:  "`docs/dev/security.md`",
			want: []docRef{{path: "docs/dev/security.md"}},
		},
		{
			name: "repository path with a line or a line range",
			doc:  "`docs/dev/conventions/requirements_process.md:12` `docs/dev/guides/cli.md:3-5`",
			want: []docRef{
				{path: "docs/dev/conventions/requirements_process.md"},
				{path: "docs/dev/guides/cli.md"},
			},
		},
		{
			name: "directory path in a code span",
			doc:  "`docs/dev/`",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := docPathRefs(tc.doc)
			slices.SortFunc(got, compareDocRef)
			want := slices.Clone(tc.want)
			slices.SortFunc(want, compareDocRef)
			if !slices.Equal(got, want) {
				t.Errorf("docPathRefs = %v, want %v", got, tc.want)
			}
		})
	}
}

// compareDocRef orders refs by path, then by whether the path is relative.
func compareDocRef(a, b docRef) int {
	if c := strings.Compare(a.path, b.path); c != 0 {
		return c
	}
	switch {
	case a.relative == b.relative:
		return 0
	case a.relative:
		return -1
	default:
		return 1
	}
}
