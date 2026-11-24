package mergepr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// MergemainRestoreResult is the state restore left: HEAD at the base head, no
// merge in progress, and no tracked change.
type MergemainRestoreResult struct {
	Head string
}

// MergemainRestore resets the branch to the base head when the work tree is
// in a state the take-in tool made and nothing else changed.
//
// Contract: it resets only in one of two states. A merge in progress, with
// HEAD at the base head, MERGE_HEAD at main, and the index and the work tree
// differing from the merge git computes only in .md files and protocol
// declaration lines; or a merge commit not pushed, with HEAD a merge commit
// whose parents are the base head and main in that order, the remote-tracking
// ref of the branch still at the base head, and no tracked change. In either
// state an untracked file that is not ignored, or an ignored file where the
// base head tracks a file, stops it. It never runs git clean or git merge
// --abort, never removes an ignored or untracked file, and never uses the
// network. Every stop is other_changes, with nothing reset.
func (m *Mergemain) MergemainRestore(ctx context.Context, pr MergemainPR, headBranch, baseHead, main string) (MergemainRestoreResult, error) {
	a := mergeArgs{pr: pr, branch: headBranch, baseHead: baseHead, main: main}
	if err := m.checkMergeArgs(ctx, a, MergemainStopPremise); err != nil {
		return MergemainRestoreResult{}, err
	}
	if err := m.checkTop(ctx, RestoreNotNeeded); err != nil {
		return MergemainRestoreResult{}, err
	}
	if err := m.checkOnBranch(ctx, headBranch, MergemainStopOtherChanges, RestoreNotNeeded); err != nil {
		return MergemainRestoreResult{}, err
	}
	if err := m.checkRestorable(ctx, a); err != nil {
		return MergemainRestoreResult{}, err
	}
	if _, err := m.git.git(ctx, "reset", "-q", "--hard", "--no-recurse-submodules", baseHead); err != nil {
		return MergemainRestoreResult{}, otherChanges("git reset failed: %v", err)
	}
	head, err := m.git.resolveCommit(ctx, "HEAD")
	if err != nil || head != baseHead {
		return MergemainRestoreResult{}, otherChanges("after the reset, HEAD is not the base head %s", baseHead)
	}
	if _, inMerge, err := m.mergeHead(ctx); err != nil || inMerge {
		return MergemainRestoreResult{}, otherChanges("after the reset, a merge is still in progress")
	}
	if clean, err := m.workTreeClean(ctx, false); err != nil || !clean {
		return MergemainRestoreResult{}, otherChanges("after the reset, a tracked file still has changes")
	}
	return MergemainRestoreResult{Head: head}, nil
}

// otherChanges is a restore's stop: nothing was reset.
func otherChanges(format string, args ...any) *MergemainStopError {
	return stopError(MergemainStopOtherChanges, RestoreNotNeeded, format, args...)
}

// checkRestorable decides whether the work tree is in one of the two states
// restore may reset, and stops otherwise.
func (m *Mergemain) checkRestorable(ctx context.Context, a mergeArgs) error {
	head, err := m.git.resolveCommit(ctx, "HEAD")
	if err != nil {
		return otherChanges("could not read HEAD: %v", err)
	}
	mergeHead, inMerge, err := m.mergeHead(ctx)
	if err != nil {
		return otherChanges("could not read MERGE_HEAD: %v", err)
	}
	switch {
	case inMerge && head == a.baseHead && mergeHead == a.main:
		if err := m.checkMergeUntouched(ctx, a); err != nil {
			return err
		}
	case !inMerge:
		parents, err := m.commitParents(ctx, head)
		if err != nil {
			return otherChanges("could not read HEAD's parents: %v", err)
		}
		if !slices.Equal(parents, []string{a.baseHead, a.main}) {
			return otherChanges("HEAD %s is neither the merge in progress nor a merge commit of the base head and main", head)
		}
		remote, err := m.refCommit(ctx, remoteBranchPrefix+a.branch)
		if err != nil || remote != a.baseHead {
			return otherChanges("%s%s is not the base head, so the merge commit may have been pushed", remoteBranchPrefix, a.branch)
		}
		if clean, err := m.workTreeClean(ctx, false); err != nil || !clean {
			return otherChanges("a tracked file changed after the merge commit")
		}
	default:
		return otherChanges("HEAD %s and MERGE_HEAD %s are not the merge of main %s into the base head %s", head, mergeHead, a.main, a.baseHead)
	}
	untracked, err := m.git.git(ctx, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return otherChanges("could not list the untracked files: %v", err)
	}
	if len(untracked) > 0 {
		return otherChanges("an untracked file that is not ignored exists, and restore does not know who made it")
	}
	ignored, err := m.ignoredPaths(ctx)
	if err != nil {
		return otherChanges("could not list the ignored files: %v", err)
	}
	basePaths, err := m.treePaths(ctx, a.baseHead)
	if err != nil {
		return otherChanges("could not list the base head's files: %v", err)
	}
	if p, ok := blockingPath(ignored, basePaths); ok {
		return otherChanges("the ignored file %s is where the base head tracks a file, and a reset would remove it", p)
	}
	return nil
}

// commitParents returns the parents of a commit, read from the object as
// stored.
func (m *Mergemain) commitParents(ctx context.Context, commit string) ([]string, error) {
	out, err := m.git.git(ctx, "cat-file", "commit", commit)
	if err != nil {
		return nil, err
	}
	header, _, _ := bytes.Cut(out, []byte("\n\n"))
	var parents []string
	for line := range strings.Lines(string(header)) {
		key, value, _ := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
		if key == "parent" {
			if !isObjectID(value) {
				return nil, fmt.Errorf("%w: parent %q", errUnexpectedGitOutput, value)
			}
			parents = append(parents, value)
		}
	}
	return parents, nil
}

// mergeTreeResult is the merge git computes for two commits without the work
// tree: its tree, each path's entry in the tree, and the stage entries of
// the conflicted paths.
type mergeTreeResult struct {
	tree      string
	entries   map[string]stageEntry
	conflicts map[string][]stageEntry
}

// mergeTree runs git merge-tree on ours and theirs with the take-in
// configuration. ours and theirs are passed as given, so they are also the
// labels of the conflict markers.
func (m *Mergemain) mergeTree(ctx context.Context, ours, theirs string) (mergeTreeResult, error) {
	args := slices.Concat(mergeConfig, []string{"merge-tree", "--write-tree", "-z", "--no-messages", ours, theirs})
	out, err := m.git.git(ctx, args...)
	if err != nil {
		exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
		if !ok || exitErr.Code != 1 {
			return mergeTreeResult{}, err
		}
		out = exitErr.Stdout
	}
	records, err := splitNUL(out)
	if err != nil {
		return mergeTreeResult{}, err
	}
	if len(records) == 0 || !isObjectID(string(records[0])) {
		return mergeTreeResult{}, fmt.Errorf("%w: merge-tree did not print a tree", errUnexpectedGitOutput)
	}
	conflicts, err := parseStageEntries(records[1:])
	if err != nil {
		return mergeTreeResult{}, err
	}
	tree := string(records[0])
	entries, err := m.treeEntries(ctx, tree)
	if err != nil {
		return mergeTreeResult{}, err
	}
	return mergeTreeResult{tree: tree, entries: entries, conflicts: conflicts}, nil
}

// treeEntries lists every file entry of a tree-ish as a stage-0 entry.
func (m *Mergemain) treeEntries(ctx context.Context, treeish string, pathspec ...string) (map[string]stageEntry, error) {
	args := []string{"ls-tree", "-r", "-z", "--full-tree", treeish}
	if len(pathspec) > 0 {
		args = append(append(args, "--"), pathspec...)
	}
	out, err := m.git.git(ctx, args...)
	if err != nil {
		return nil, err
	}
	records, err := splitNUL(out)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]stageEntry, len(records))
	for _, r := range records {
		info, name, ok := bytes.Cut(r, []byte{'\t'})
		fields := strings.Split(string(info), " ")
		if !ok || len(name) == 0 || len(fields) != 3 || !isObjectID(fields[2]) {
			return nil, fmt.Errorf("%w: ls-tree entry %q", errUnexpectedGitOutput, r)
		}
		entries[string(name)] = stageEntry{mode: fields[0], oid: fields[2]}
	}
	return entries, nil
}

// versionAt returns the content of p at commit, or nil and false when the
// commit has no regular file there.
func (m *Mergemain) versionAt(ctx context.Context, commit, p string) ([]byte, bool, error) {
	entries, err := m.treeEntries(ctx, commit, ":(top,literal)"+p)
	if err != nil {
		return nil, false, err
	}
	e, ok := entries[p]
	if !ok || (e.mode != regularMode && e.mode != executableMode) {
		return nil, false, nil
	}
	content, err := m.blob(ctx, e.oid)
	return content, err == nil, err
}

// onlyProtocolDiffers reports whether content, a version of the file p, is
// the merge of its versions at base, ours, and theirs apart from the one
// accepted declaration line.
//
// Contract: p must be a file directly inside internal/<name>/ that is not a
// _test.go file; each version that exists must hold exactly one accepted
// declaration line (a missing version counts as empty); the three versions,
// their declaration lines replaced by the placeholder, must merge without
// conflict; and content, its line replaced the same way, must equal that
// merge byte for byte.
func (m *Mergemain) onlyProtocolDiffers(ctx context.Context, p string, content []byte, base, ours, theirs string) (bool, error) {
	if _, ok := toolPackage(p); !ok {
		return false, nil
	}
	var stripped [3][]byte
	for i, commit := range []string{base, ours, theirs} {
		version, exists, err := m.versionAt(ctx, commit, p)
		if err != nil {
			return false, err
		}
		if !exists {
			continue
		}
		s, count, ok := stripProtocol(version)
		if !ok || count != 1 {
			return false, nil
		}
		stripped[i] = s
	}
	merged, clean, err := m.mergeFile(ctx, stripped[0], stripped[1], stripped[2])
	if err != nil || !clean {
		return false, err
	}
	got, count, ok := stripProtocol(content)
	if !ok || count != 1 {
		return false, nil
	}
	return bytes.Equal(got, merged), nil
}

// checkMergeUntouched stops unless the index and the work tree of the merge
// in progress differ from the merge git computes (merge-tree of HEAD and
// main, with the same labels as the merge) only in .md files and protocol
// declaration lines: the take-in changes nothing else, so any other
// difference was made by someone else, and a reset would discard it.
func (m *Mergemain) checkMergeUntouched(ctx context.Context, a mergeArgs) error {
	base, err := m.mergeBase(ctx, a.baseHead, a.main)
	if err != nil {
		return otherChanges("could not find one merge base: %v", err)
	}
	computed, err := m.mergeTree(ctx, "HEAD", a.main)
	if err != nil {
		return otherChanges("could not compute the merge: %v", err)
	}
	expected := maps.Clone(computed.entries)
	expectedStages := make(map[string][]stageEntry, len(expected))
	for p, e := range expected {
		expectedStages[p] = []stageEntry{e}
	}
	maps.Copy(expectedStages, computed.conflicts)
	out, err := m.git.git(ctx, "ls-files", "-s", "-z")
	if err != nil {
		return otherChanges("could not list the index: %v", err)
	}
	records, err := splitNUL(out)
	if err != nil {
		return otherChanges("could not list the index: %v", err)
	}
	actual, err := parseStageEntries(records)
	if err != nil {
		return otherChanges("could not list the index: %v", err)
	}
	allowed := func(p string, content []byte) (bool, error) {
		if strings.HasSuffix(p, markdownSuffix) {
			return true, nil
		}
		if content == nil {
			return false, nil
		}
		return m.onlyProtocolDiffers(ctx, p, content, base, a.baseHead, a.main)
	}
	if err := m.checkIndexUntouched(ctx, expectedStages, actual, allowed); err != nil {
		return err
	}
	return m.checkWorkTreeUntouched(ctx, computed, actual, allowed)
}

// allowedDiff reports whether a path may differ from the computed merge with
// content (nil when it is not a regular file or is missing).
type allowedDiff func(p string, content []byte) (bool, error)

// checkIndexUntouched compares the index entries with the computed merge's.
func (m *Mergemain) checkIndexUntouched(ctx context.Context, expectedStages, actual map[string][]stageEntry, allowed allowedDiff) error {
	for _, p := range slices.Sorted(maps.Keys(unionKeys(expectedStages, actual))) {
		if sameStages(expectedStages[p], actual[p]) {
			continue
		}
		var content []byte
		if es := actual[p]; len(es) == 1 && es[0].stage == 0 && (es[0].mode == regularMode || es[0].mode == executableMode) {
			var err error
			if content, err = m.blob(ctx, es[0].oid); err != nil {
				return otherChanges("could not read %s: %v", p, err)
			}
		}
		if ok, err := allowed(p, content); err != nil || !ok {
			return otherChanges("the index entry of %s differs from the merge beyond the take-in's changes", p)
		}
	}
	return nil
}

// checkWorkTreeUntouched compares the work tree with the index, and a
// conflicted file with the computed merge's file.
func (m *Mergemain) checkWorkTreeUntouched(ctx context.Context, computed mergeTreeResult, actual map[string][]stageEntry, allowed allowedDiff) error {
	out, err := m.git.git(ctx, "diff-files", "-z", "--name-only")
	if err != nil {
		return otherChanges("could not compare the work tree with the index: %v", err)
	}
	records, err := splitNUL(out)
	if err != nil {
		return otherChanges("could not compare the work tree with the index: %v", err)
	}
	top, err := m.top(ctx)
	if err != nil {
		return otherChanges("could not find the top of the work tree: %v", err)
	}
	for _, r := range records {
		p := string(r)
		content, readErr := readRegular(filepath.Join(top, filepath.FromSlash(p)))
		if es := actual[p]; len(es) > 0 && es[0].stage != 0 && readErr == nil {
			// A conflicted file holds what the merge wrote, which is the
			// computed merge's file.
			if e, ok := computed.entries[p]; ok {
				same, err := m.sameAsBlob(ctx, p, e.oid)
				if err != nil {
					return otherChanges("could not read %s: %v", p, err)
				}
				if same {
					continue
				}
			}
		}
		if readErr != nil {
			content = nil
		}
		if ok, err := allowed(p, content); err != nil || !ok {
			return otherChanges("the work tree file %s differs from the merge beyond the take-in's changes", p)
		}
	}
	return nil
}

// readRegular reads a regular file without following a symbolic link.
func readRegular(full string) ([]byte, error) {
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", errTakeInFile, full)
	}
	return os.ReadFile(full) //nolint:gosec // a path inside the work tree, named by git
}

// sameAsBlob reports whether the work tree file p, cleaned as git add would,
// is the blob oid.
func (m *Mergemain) sameAsBlob(ctx context.Context, p, oid string) (bool, error) {
	out, err := m.git.git(ctx, "hash-object", "--path="+p, "--", p)
	if err != nil {
		return false, err
	}
	return strings.TrimSuffix(string(out), "\n") == oid, nil
}

// unionKeys returns a set of the keys of both maps.
func unionKeys[V any](a, b map[string]V) map[string]struct{} {
	keys := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	return keys
}

// sameStages reports whether two lists hold the same entries in any order.
func sameStages(a, b []stageEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for _, e := range a {
		if !slices.Contains(b, e) {
			return false
		}
	}
	return true
}
