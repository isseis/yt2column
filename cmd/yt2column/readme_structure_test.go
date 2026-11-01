//go:build test

package main

import (
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
