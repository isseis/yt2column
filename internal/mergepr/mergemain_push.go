package mergepr

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// MergemainPushResult is the result of a push that landed: the merge commit,
// now the PR's head.
type MergemainPushResult struct {
	NewHead string
}

// MergemainPush pushes the merge commit at HEAD to the PR's head branch after
// checking it right before the push.
//
// Contract: it pushes only when, read again just before the push, the PR is
// open, into main, from a branch of origin's repository, and that branch is
// the current one at the base head; HEAD is a merge commit whose parents are
// the base head and main, main is the value begin recorded and an ancestor of
// main fetched again in this call; merging the parents leaves no conflict out
// of scope; the merge commit differs from that merge only in .md files and in
// the declaration lines of the tools the branch changed; each of those tools
// has main's number plus one; no tracked file has a change; origin's push URL
// is its fetch URL; and origin's branch is still the base head. It pushes the
// checked commit ID with an explicit refspec, without force, tags, or
// submodules. Every stop before the push asks for a restore, other than a
// tracked change, which restore would refuse; after a failed push, origin's
// branch is read again to tell a rejected push from one whose result is
// unknown.
func (m *Mergemain) MergemainPush(ctx context.Context, pr MergemainPR, baseHead, main string) (MergemainPushResult, error) {
	if pr.number == "" || !isCommitArg(baseHead) || !isCommitArg(main) {
		return MergemainPushResult{}, stopError(MergemainStopPremise, RestoreNeeded, "the PR number, the base head, and main must be given in their accepted forms")
	}
	branch, err := m.checkPushTarget(ctx, pr, baseHead)
	if err != nil {
		return MergemainPushResult{}, err
	}
	head, err := m.checkMergeCommit(ctx, pr, baseHead, main)
	if err != nil {
		return MergemainPushResult{}, err
	}
	if clean, err := m.workTreeClean(ctx, false); err != nil || !clean {
		return MergemainPushResult{}, stopError(MergemainStopOtherChanges, RestoreNotNeeded, "a tracked file has changes after the merge commit")
	}
	if err := m.checkMergeScope(ctx, head, baseHead, main); err != nil {
		return MergemainPushResult{}, err
	}
	if err := m.checkPushURL(ctx); err != nil {
		return MergemainPushResult{}, err
	}
	remote, err := m.remoteBranch(ctx, branch)
	if err != nil || remote != baseHead {
		return MergemainPushResult{}, stopError(MergemainStopPremise, RestoreNeeded, "origin's %s is not the base head %s", branch, baseHead)
	}
	refspec := head + ":" + headsPrefix + branch
	if _, pushErr := m.git.git(ctx, "push", "-q", "--no-follow-tags", "--recurse-submodules=no", "--", originRemote, refspec); pushErr != nil {
		after, err := m.remoteBranch(ctx, branch)
		switch {
		case err == nil && after == head:
			return MergemainPushResult{NewHead: head}, nil
		case err == nil && after == baseHead:
			return MergemainPushResult{}, stopError(MergemainStopPushRejected, RestoreNeeded, "origin rejected the push of %s: %v", head, pushErr)
		default:
			return MergemainPushResult{}, stopError(MergemainStopPushUnknown, RestoreNotNeeded, "the push of %s failed and origin's %s could not be read as either commit: %v", head, branch, errors.Join(pushErr, err))
		}
	}
	return MergemainPushResult{NewHead: head}, nil
}

// checkPushTarget reads the PR again and returns its head branch when it is
// the current branch and the PR's head is the base head.
func (m *Mergemain) checkPushTarget(ctx context.Context, pr MergemainPR, baseHead string) (string, error) {
	if err := m.checkTop(ctx, RestoreNeeded); err != nil {
		return "", err
	}
	v, err := m.readPR(ctx, pr, RestoreNeeded)
	if err != nil {
		return "", err
	}
	if err := m.checkRefFormat(ctx, v.headBranch, RestoreNeeded); err != nil {
		return "", err
	}
	if err := m.checkOnBranch(ctx, v.headBranch, MergemainStopPremise, RestoreNeeded); err != nil {
		return "", err
	}
	if v.headOID != baseHead {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "the PR head %s is not the base head %s", v.headOID, baseHead)
	}
	return v.headBranch, nil
}

// checkMergeCommit checks that HEAD is the merge commit of the base head and
// main, that main is what begin recorded, and that main is an ancestor of
// main fetched again now and read from FETCH_HEAD in this call. It returns
// HEAD's commit ID.
func (m *Mergemain) checkMergeCommit(ctx context.Context, pr MergemainPR, baseHead, main string) (string, error) {
	if _, inMerge, err := m.mergeHead(ctx); err != nil || inMerge {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "a merge is still in progress; commit it first")
	}
	head, err := m.git.resolveCommit(ctx, "HEAD")
	if err != nil {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "could not read HEAD: %v", err)
	}
	parents, err := m.commitParents(ctx, head)
	if err != nil || !slices.Equal(parents, []string{baseHead, main}) {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "HEAD %s is not a merge commit whose parents are the base head %s and main %s", head, baseHead, main)
	}
	recorded, err := m.refCommit(ctx, pr.mainRefOf())
	if err != nil || recorded != main {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "main %s is not the main begin recorded in %s", main, pr.mainRefOf())
	}
	if _, err := m.git.git(ctx, "fetch", "-q", "--no-tags", "--no-prune", "--no-recurse-submodules", "--refmap=", "--", originRemote, headsPrefix+mainBranch); err != nil {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "could not fetch main from origin: %v", err)
	}
	fetched, err := m.git.resolveCommit(ctx, "FETCH_HEAD")
	if err != nil {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "could not read the fetched main: %v", err)
	}
	inMain, err := m.isAncestor(ctx, main, fetched)
	if err != nil || !inMain {
		return "", stopError(MergemainStopPremise, RestoreNeeded, "main %s is not in the history of origin's main %s", main, fetched)
	}
	return head, nil
}

// checkMergeScope checks what the merge commit head changed (3.8.3 of the
// design): the merge of its parents has no conflict out of scope; head's
// tree differs from that merge only in .md files that stay regular files and
// in files of the tools the branch changed whose versions differ only in the
// declaration line; and every tool the branch changed has main's number plus
// one at head.
func (m *Mergemain) checkMergeScope(ctx context.Context, head, baseHead, main string) error {
	base, err := m.mergeBase(ctx, baseHead, main)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNeeded, "could not find one merge base: %v", err)
	}
	expected, err := m.branchProtocolTools(ctx, baseHead, main)
	if err != nil {
		return stopError(MergemainStopOutOfScope, RestoreNeeded, "could not read the protocol numbers: %v", err)
	}
	computed, err := m.mergeTree(ctx, baseHead, main)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNeeded, "could not compute the merge of the parents: %v", err)
	}
	conflicts, _, err := m.classifyConflicts(ctx, base, baseHead, main, computed.conflicts, expected)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNeeded, "could not classify the conflicts of the parents: %v", err)
	}
	if slices.ContainsFunc(conflicts, func(c PathConflict) bool { return c.Class == PathConflictOutOfScope }) {
		stop := stopError(MergemainStopOutOfScope, RestoreNeeded, "merging the parents leaves a conflict out of scope")
		stop.Conflicts = conflicts
		return stop
	}
	if err := m.checkMergeDiff(ctx, head, base, baseHead, main, computed, expected); err != nil {
		return err
	}
	readings, err := (&protocolCheck{m.git}).readRevision(ctx, head)
	if err != nil {
		return stopError(MergemainStopOutOfScope, RestoreNeeded, "could not read the merge commit's protocol numbers: %v", err)
	}
	for _, pkg := range slices.Sorted(maps.Keys(expected)) {
		if got := valueOf(readings, pkg); got.State != protocolPresent || got.Value != expected[pkg] {
			return stopError(MergemainStopOutOfScope, RestoreNeeded, "tool %s has %v at the merge commit, not main's number plus one, %d", pkg, got, expected[pkg])
		}
	}
	return nil
}

// checkMergeDiff checks that the merge commit's tree differs from the
// computed merge of its parents only in .md files that stay regular files and
// in the declaration lines of the tools the branch changed.
func (m *Mergemain) checkMergeDiff(ctx context.Context, head, base, baseHead, main string, computed mergeTreeResult, expected map[string]int) error {
	headEntries, err := m.treeEntries(ctx, head)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNeeded, "could not list the merge commit's files: %v", err)
	}
	for _, p := range slices.Sorted(maps.Keys(unionEntryKeys(computed.entries, headEntries))) {
		got, inHead := headEntries[p]
		want, inMerge := computed.entries[p]
		if inHead && inMerge && got == want {
			continue
		}
		if !inHead || (got.mode != regularMode && got.mode != executableMode) {
			return stopError(MergemainStopOutOfScope, RestoreNeeded, "the merge commit removes %s or makes it something other than a regular file", p)
		}
		if strings.HasSuffix(p, markdownSuffix) {
			continue
		}
		pkg, isTool := toolPackage(p)
		if _, changed := expected[pkg]; !isTool || !changed {
			return stopError(MergemainStopOutOfScope, RestoreNeeded, "the merge commit changes %s, which is not a .md file or a file of a tool the branch changed", p)
		}
		content, err := m.blob(ctx, got.oid)
		if err != nil {
			return stopError(MergemainStopPremise, RestoreNeeded, "could not read %s: %v", p, err)
		}
		ok, err := m.onlyProtocolDiffers(ctx, p, content, base, baseHead, main)
		if err != nil {
			return stopError(MergemainStopPremise, RestoreNeeded, "could not compare %s: %v", p, err)
		}
		if !ok {
			return stopError(MergemainStopOutOfScope, RestoreNeeded, "the merge commit changes %s beyond its protocol declaration line", p)
		}
	}
	return nil
}

// unionEntryKeys returns a set of the keys of both maps.
func unionEntryKeys(a, b map[string]stageEntry) map[string]struct{} {
	keys := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	return keys
}

// checkPushURL stops unless origin has one fetch URL and one push URL, as git
// resolves them with insteadOf, pushInsteadOf, and pushurl, and they are the
// same, so ls-remote reads the branch the push sends to.
func (m *Mergemain) checkPushURL(ctx context.Context) error {
	fetchURL, err := m.oneURL(ctx, "get-url", "--all", originRemote)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNeeded, "could not read origin's fetch URL: %v", err)
	}
	pushURL, err := m.oneURL(ctx, "get-url", "--push", "--all", originRemote)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNeeded, "could not read origin's push URL: %v", err)
	}
	if fetchURL != pushURL {
		return stopError(MergemainStopPremise, RestoreNeeded, "origin's push URL is not its fetch URL")
	}
	return nil
}

// oneURL runs git remote with args and returns its one line.
func (m *Mergemain) oneURL(ctx context.Context, args ...string) (string, error) {
	out, err := m.git.git(ctx, append([]string{"remote"}, args...)...)
	if err != nil {
		return "", err
	}
	text, ok := strings.CutSuffix(string(out), "\n")
	if !ok || text == "" || strings.Contains(text, "\n") {
		return "", fmt.Errorf("%w: not exactly one URL", errUnexpectedGitOutput)
	}
	return text, nil
}

// remoteBranch reads origin's branch with ls-remote. It matches the ref name
// exactly, since ls-remote matches patterns by suffix, and returns an error
// when the branch is missing or listed twice.
func (m *Mergemain) remoteBranch(ctx context.Context, branch string) (string, error) {
	ref := headsPrefix + branch
	out, err := m.git.git(ctx, "ls-remote", "--", originRemote, ref)
	if err != nil {
		return "", err
	}
	oid := ""
	for line := range strings.Lines(string(out)) {
		got, name, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		if !ok || name != ref {
			continue
		}
		if oid != "" || !isObjectID(got) {
			return "", fmt.Errorf("%w: ls-remote output", errUnexpectedGitOutput)
		}
		oid = got
	}
	if oid == "" {
		return "", fmt.Errorf("%w: origin has no %s", errUnexpectedGitOutput, ref)
	}
	return oid, nil
}
