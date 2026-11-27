//go:build test

package mergepr

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shared takeIn states are copied and adjusted instead of rebuilt in every
// case, which is only sound while the adjusted copy is the same state a rebuild
// gives. These tests build each shared state from scratch in a fresh directory
// and compare it with a case's adjusted copy.

// takeInShape describes k by what a copied and adjusted takeIn must preserve:
// diff-files output (checked before any command that might write the index),
// refs with the tree, subject, and parent count of each commit in both origin
// and work, HEAD, local configuration (with the own origin path normalized),
// index entries, MERGE_HEAD and its commit shape, and work tree files. When
// copied is true, it fails t if any file under origin or work other than
// work/.git/FETCH_HEAD contains snapshotRootPrefix.
func takeInShape(t *testing.T, k *takeIn, copied bool) string {
	t.Helper()
	var b strings.Builder

	// Run diff-files first so no earlier git command can refresh the index.
	b.WriteString("diff-files:\n" + k.Git("diff-files") + "\n")

	// Commit IDs differ with commit time, so replace this takeIn's own main
	// commit ID where git embeds it (conflict markers and merge subjects).
	mainOID := ""
	if refs := k.gitIn(k.Origin, "for-each-ref", "--format=%(objectname)", "refs/heads/"+mainBranch); refs != "" {
		mainOID = strings.TrimSpace(refs)
	}
	normOID := func(s string) string {
		if mainOID == "" {
			return s
		}
		return strings.ReplaceAll(s, mainOID, "<MAIN_OID>")
	}

	writeRepoSummary := func(label, dir string) {
		b.WriteString(label + " refs:\n")
		for ref := range strings.FieldsSeq(k.gitIn(dir, "for-each-ref", "--format=%(refname)")) {
			b.WriteString("ref " + ref + ":\n")
			for line := range strings.SplitSeq(k.gitIn(dir, "log", "--format=%T|%s|%P", ref), "\n") {
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, "|", 3)
				parents := len(strings.Fields(parts[2]))
				fmt.Fprintf(&b, "  %s|%s|%d parents\n", parts[0], normOID(parts[1]), parents)
			}
		}
		b.WriteString(label + " head: " + strings.TrimSpace(k.gitIn(dir, "symbolic-ref", "HEAD")) + "\n")
		cfg := strings.ReplaceAll(k.gitIn(dir, "config", "--local", "--list"), k.Origin, "<ORIGIN>")
		b.WriteString(label + " config:\n" + cfg + "\n")
	}
	writeRepoSummary("origin", k.Origin)
	writeRepoSummary("work", k.Work)

	b.WriteString("index:\n" + k.Git("ls-files", "-s") + "\n")
	if _, err := os.Stat(filepath.Join(k.Work, ".git", "MERGE_HEAD")); err == nil {
		parents := len(strings.Fields(k.Git("log", "-1", "--format=%P", "MERGE_HEAD")))
		fmt.Fprintf(&b, "MERGE_HEAD: %s|%d parents\n", strings.TrimSpace(k.Git("log", "-1", "--format=%T|%s", "MERGE_HEAD")), parents)
	}

	if copied {
		allowedFetchHead := filepath.Join(k.Work, ".git", "FETCH_HEAD")
		for _, root := range []string{k.Origin, k.Work} {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				if path == allowedFetchHead {
					return nil
				}
				data, err := os.ReadFile(path) //nolint:gosec // a file inside the test's own takeIn directory
				if err != nil {
					return err
				}
				if bytes.Contains(data, []byte(snapshotRootPrefix)) {
					t.Errorf("%s holds the path of a snapshot builder's directory", path)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walk %s: %v", root, err)
			}
		}
	}

	b.WriteString("files:\n")
	err := filepath.WalkDir(k.Work, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == filepath.Join(k.Work, ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(k.Work, path)
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			b.WriteString(filepath.ToSlash(rel) + " -> " + target + "\n")
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // a file inside the test's own takeIn work tree
		if err != nil {
			return err
		}
		b.WriteString(filepath.ToSlash(rel) + "\n" + normOID(string(data)) + "\n")
		return nil
	})
	if err != nil {
		t.Fatalf("read work tree: %v", err)
	}
	return b.String()
}

// legacySetup is the original Setup procedure (sequential Commit, push, and
// switch), kept here as the reference for TestTakeInSnapshotsMatchRebuilt.
func legacySetup(k *takeIn, base, branch, main map[string]string) (baseHead, mainOID string) {
	k.t.Helper()
	k.Commit("base", base)
	k.Git("push", "-q", "origin", mainBranch)
	k.Git("switch", "-q", "-c", takeInBranch)
	baseHead = k.Commit("branch", branch)
	k.Git("push", "-q", "origin", takeInBranch)
	k.Git("switch", "-q", mainBranch)
	mainOID = k.Commit("main", main)
	k.Git("push", "-q", "origin", mainBranch)
	k.Git("switch", "-q", takeInBranch)
	return baseHead, mainOID
}

func TestTakeInSnapshotsMatchRebuilt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		copied  func(t *testing.T) *takeIn
		rebuilt func(t *testing.T) *takeIn
	}{
		{
			name:   "empty_take_in",
			copied: newTakeIn,
			rebuilt: func(t *testing.T) *takeIn {
				return buildTakeInBase(t, t.TempDir())
			},
		},
		{
			name: "conflict_in_progress",
			copied: func(t *testing.T) *takeIn {
				k, _ := newConflictInProgressTakeIn(t)
				return k
			},
			rebuilt: func(t *testing.T) *takeIn {
				k := buildTakeInBase(t, t.TempDir())
				buildConflictInProgressState(t, k)
				return k
			},
		},
		{
			name: "after_merge_commit",
			copied: func(t *testing.T) *takeIn {
				k, _ := newPushFlowTakeIn(t)
				return k
			},
			rebuilt: func(t *testing.T) *takeIn {
				k := buildTakeInBase(t, t.TempDir())
				buildPushFlowState(t, k)
				return k
			},
		},
		{
			name: "setup_matches_legacy",
			copied: func(t *testing.T) *takeIn {
				k := newTakeIn(t)
				base := baseFiles()
				base["link.md"] = symlinkPrefix + "doc.md"
				branch := map[string]string{"doc.md": "branch\n", "keep/x.txt": deletedFile, "link.md": symlinkPrefix + "README.md"}
				main := map[string]string{"doc.md": "main\n", "m.md": "m\n"}
				baseHead, mainOID := k.Setup(base, branch, main)
				if got := k.Git("rev-parse", "HEAD"); got != baseHead {
					t.Errorf("HEAD = %s, want baseHead %s", got, baseHead)
				}
				if got := k.Git("rev-parse", "refs/heads/"+mainBranch); got != mainOID {
					t.Errorf("main = %s, want mainOID %s", got, mainOID)
				}
				if k.Git("rev-parse", baseHead+"^") != k.Git("rev-parse", mainOID+"^") {
					t.Errorf("branch and main do not share the base commit as their parent")
				}
				return k
			},
			rebuilt: func(t *testing.T) *takeIn {
				k := buildTakeInBase(t, t.TempDir())
				base := baseFiles()
				base["link.md"] = symlinkPrefix + "doc.md"
				branch := map[string]string{"doc.md": "branch\n", "keep/x.txt": deletedFile, "link.md": symlinkPrefix + "README.md"}
				main := map[string]string{"doc.md": "main\n", "m.md": "m\n"}
				legacySetup(k, base, branch, main)
				return k
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, want := takeInShape(t, tc.copied(t), true), takeInShape(t, tc.rebuilt(t), false)
			if got != want {
				t.Errorf("the copy differs from the rebuilt state\ncopy:\n%s\nrebuilt:\n%s", got, want)
			}
		})
	}
}
