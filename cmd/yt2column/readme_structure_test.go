//go:build test

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// TestReadmeVersionsShareStructure checks that README.md and README.en.md
// have the same heading levels in the same order and the same number of code
// blocks. The heading text is not compared: the versions are in different
// languages. A missing version fails rather than skips, so deleting
// README.en.md cannot pass, and a version in a form the structure parser
// rejects fails naming that version. The doc-guard CI job runs this package,
// so a pull request that changes only a README is checked too.
func TestReadmeVersionsShareStructure(t *testing.T) {
	versions := twoReadmeVersions(t)
	structures := make([]doctestutil.Structure, len(versions))
	for i, v := range versions {
		s, err := doctestutil.ParseStructure(doctestutil.ReadDoc(t, v.From(repositoryRoot)))
		if err != nil {
			t.Fatalf("%s: %v", v.Path, err)
		}
		structures[i] = s
	}
	if diff := doctestutil.CompareStructures(structures[0], structures[1]); diff.Kind != doctestutil.StructureSame {
		t.Errorf("%s (left) and %s (right) do not share their structure: %v", versions[0].Path, versions[1].Path, diff)
	}
}

// TestReadmeVersionLinks checks the links that tie the documents together:
// each README version links to the other version before its first heading, so
// a reader can switch language at the top; each version links to the two
// developer guides; and cli.md links to the configuration section of
// README.md, which holds the variables the CLI shares with the server.
// TestReadmeLinks checks that every one of these links resolves.
func TestReadmeVersionLinks(t *testing.T) {
	versions := twoReadmeVersions(t)
	for i, v := range versions {
		other := versions[1-i]
		t.Run(v.Path, func(t *testing.T) {
			path := v.From(repositoryRoot)
			doc := doctestutil.ReadDoc(t, path)
			if !linksTo(markdownLinkTargets(beforeFirstHeading(doc)), path, other.From(repositoryRoot), "") {
				t.Errorf("%s has no link to %s before its first heading", v.Path, other.Path)
			}
			targets := markdownLinkTargets(doc)
			for _, guide := range readmeGuides {
				if !linksTo(targets, path, filepath.Join(repositoryRoot, guide), "") {
					t.Errorf("%s has no link to %s", v.Path, guide)
				}
			}
		})
	}

	t.Run("cli.md", func(t *testing.T) {
		readme := versions[0]
		anchor := doctestutil.AnchorFromHeading(doctestutil.ReadmeConfigurationHeading.In(t, readme.Lang))
		if !linksTo(markdownLinkTargets(doctestutil.ReadDoc(t, cliDocPath)), cliDocPath, readme.From(repositoryRoot), anchor) {
			t.Errorf("cli.md has no link to %s#%s", readme.Path, anchor)
		}
	})
}

// twoReadmeVersions returns the README versions, the primary version first,
// failing t unless there are exactly two: these tests compare one pair.
func twoReadmeVersions(t *testing.T) []doctestutil.ReadmeVersion {
	t.Helper()
	versions := doctestutil.ReadmeVersions()
	if len(versions) != 2 || versions[0].Lang != doctestutil.LangJA {
		t.Fatalf("want the Japanese primary README and its English version, got %+v", versions)
	}
	return versions
}

// beforeFirstHeading returns the lines of doc before its first heading outside
// a fenced code block.
func beforeFirstHeading(doc string) string {
	var b strings.Builder
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(doc))) {
		if !line.Block && doctestutil.HeadingLevel(line.Text) > 0 {
			break
		}
		b.WriteString(line.Text)
	}
	return b.String()
}

// linksTo reports whether one of targets, links written in the document at
// from, resolves to the file want, and, when anchor is not empty, names that
// anchor.
func linksTo(targets []string, from, want, anchor string) bool {
	for _, target := range targets {
		if strings.Contains(target, "://") {
			continue
		}
		path, gotAnchor, _ := strings.Cut(target, "#")
		if path == "" || gotAnchor != anchor {
			continue
		}
		if filepath.Clean(filepath.Join(filepath.Dir(from), path)) == filepath.Clean(want) {
			return true
		}
	}
	return false
}
