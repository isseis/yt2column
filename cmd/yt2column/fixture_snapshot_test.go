//go:build test

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// The shared fixture states are copied instead of rebuilt in every case, which
// is only sound while a copy is the same state a rebuild gives. These tests
// build each shared state the way the cases used to (the build function, run
// in a fresh directory, with no sharing) and compare it with a case's copy.

// snapshotBuilderMarker is part of the name of every directory the snapshot
// store builds in. A file of a copy that holds it names the builder's
// directory, which the copy does not own.
const snapshotBuilderMarker = "yt2column-snapshots-"

// repoShape describes the repository of f by what its copy must preserve: the
// refs with the tree, subject, and parent count of the commit each names (the
// commit IDs differ with the commit time, and a parent is named by one), the local configuration, the index
// entries, the work tree files with their content, and MERGE_HEAD. It fails
// t when a file of the repository holds the name of a snapshot builder's
// directory.
func repoShape(t *testing.T, f *gitFixture) string {
	t.Helper()
	var b strings.Builder
	refs := strings.Fields(f.git("for-each-ref", "--format=%(refname)"))
	b.WriteString("refs:\n")
	for _, ref := range refs {
		// The commit ID differs with the commit time, and so does a parent's
		// ID, so the shape holds the tree, the subject, and the parent count.
		parents := len(strings.Fields(f.git("log", "-1", "--format=%P", ref)))
		fmt.Fprintf(&b, "%s %s|%d parents\n", ref, strings.TrimSpace(f.git("log", "-1", "--format=%T|%s", ref)), parents)
	}
	b.WriteString("head: " + strings.TrimSpace(f.git("symbolic-ref", "HEAD")) + "\n")
	b.WriteString("config:\n" + f.git("config", "--local", "--list"))
	b.WriteString("index:\n" + f.git("ls-files", "-s"))
	if _, err := os.Stat(filepath.Join(f.repo, ".git", "MERGE_HEAD")); err == nil {
		b.WriteString("MERGE_HEAD present\n")
	}
	b.WriteString("files:\n")
	err := filepath.WalkDir(f.repo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(filepath.Dir(path)) == "hooks" {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // a file of the test's own fixture repository
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(snapshotBuilderMarker)) {
			t.Errorf("%s holds the path of a snapshot builder's directory", path)
		}
		rel, err := filepath.Rel(f.repo, path)
		if err != nil {
			return err
		}
		// Git's own object and bookkeeping files differ in names and times
		// between builds; their content is compared through the commands above.
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return nil
		}
		b.WriteString(filepath.ToSlash(rel) + "\n" + string(data) + "\n")
		return nil
	})
	if err != nil {
		t.Fatalf("read the repository: %v", err)
	}
	return b.String()
}

// rebuiltFixture builds a state with build in a fresh directory, as a case
// did before states were shared, and returns a fixture over its repository.
func rebuiltFixture(t *testing.T, build func(bt *testing.T, dir string)) *gitFixture {
	t.Helper()
	dir := t.TempDir()
	build(t, dir)
	return &gitFixture{t: t, home: filepath.Join(dir, "home"), tmp: filepath.Join(dir, "tmp"), repo: filepath.Join(dir, "repo")}
}

func TestFixtureSnapshotsMatchRebuilt(t *testing.T) {
	t.Parallel()
	realContext := doctestutil.ReadDoc(t, contextPath)
	markers := markersPlanWith(t, "high")
	single := rrPlanSample(t, "single.md")
	cases := []struct {
		name    string
		copied  func(t *testing.T) *gitFixture
		rebuilt func(t *testing.T) *gitFixture
	}{
		{
			name:   "rr_fixture_with_context",
			copied: func(t *testing.T) *gitFixture { return newRRFixtureWithContext(t, "", realContext).gitFixture },
			rebuilt: func(t *testing.T) *gitFixture {
				return rebuiltFixture(t, func(bt *testing.T, dir string) { buildRRBase(bt, dir, realContext) })
			},
		},
		{
			name:   "rr_fixture_without_context",
			copied: func(t *testing.T) *gitFixture { return newRRFixtureWithContext(t, "", "").gitFixture },
			rebuilt: func(t *testing.T) *gitFixture {
				return rebuiltFixture(t, func(bt *testing.T, dir string) { buildRRBase(bt, dir, "") })
			},
		},
		{
			name: "hrd_fixture",
			copied: func(t *testing.T) *gitFixture {
				return newHRDFixture(t, scriptBash{}, realContext, markers, single).gitFixture
			},
			rebuilt: func(t *testing.T) *gitFixture {
				return rebuiltFixture(t, func(bt *testing.T, dir string) { buildHRDBase(bt, dir, realContext, markers, single) })
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, want := repoShape(t, tc.copied(t)), repoShape(t, tc.rebuilt(t))
			if got != want {
				t.Errorf("the copy differs from the rebuilt state\ncopy:\n%s\nrebuilt:\n%s", got, want)
			}
		})
	}
}
