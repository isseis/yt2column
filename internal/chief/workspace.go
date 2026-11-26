package chief

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

// Mergeable is GitHub's mergeable value of an open PR. The zero value is
// MergeableUnknown, the value that sends nothing.
type Mergeable int

const (
	// MergeableUnknown is a value GitHub has not computed yet (UNKNOWN).
	MergeableUnknown Mergeable = iota
	// MergeableNoConflict is a PR that merges without conflicts (MERGEABLE).
	MergeableNoConflict
	// MergeableConflicting is a PR in a merge conflict (CONFLICTING).
	MergeableConflicting
)

// parseMergeable reads GitHub's mergeable value. Contract: only the three
// values GitHub documents are accepted, compared as bytes; any other text,
// a value in another case or with spaces around it included, is an error,
// never a guessed value.
func parseMergeable(s string) (Mergeable, error) {
	switch s {
	case "MERGEABLE":
		return MergeableNoConflict, nil
	case "CONFLICTING":
		return MergeableConflicting, nil
	case "UNKNOWN":
		return MergeableUnknown, nil
	default:
		return MergeableUnknown, fmt.Errorf("%w: mergeable %q", errPRList, s)
	}
}

// CommitID is a full commit ID: 40 lowercase hexadecimal digits. Only its
// constructor makes one; the zero value names no commit and is never read
// from a parsed PR.
type CommitID struct{ hex string }

// errCommitID reports a text that is not a full commit ID.
var errCommitID = errors.New("not a full commit ID")

// ParseCommitID accepts a full commit ID: exactly 40 lowercase hexadecimal
// digits, nothing around them.
func ParseCommitID(s string) (CommitID, error) {
	if !commitID.MatchString(s) {
		return CommitID{}, fmt.Errorf("%w: %q", errCommitID, s)
	}
	return CommitID{hex: s}, nil
}

// IsZero reports whether c names no commit: the zero value, which no
// constructor returns, so it is never used to send or to key a record.
func (c CommitID) IsZero() bool { return c.hex == "" }

// String returns the 40 digits, or "" for the zero value.
func (c CommitID) String() string { return c.hex }

// MarshalText encodes c as its 40 hexadecimal digits, or empty for the zero
// value.
func (c CommitID) MarshalText() ([]byte, error) {
	return []byte(c.hex), nil
}

// UnmarshalText decodes a full commit ID through ParseCommitID, or clears c
// when text is empty so an optional record member may omit it.
func (c *CommitID) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		var zero CommitID
		*c = zero
		return nil
	}
	parsed, err := ParseCommitID(string(text))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// OpenPR is an open pull request of a worktree's branch.
type OpenPR struct {
	Number    int
	Title     string
	URL       string
	Mergeable Mergeable
	// Head is the PR's head commit, as GitHub reports it.
	Head CommitID
	// BaseIsMain is true when the PR's base branch is main, decided once
	// where the list is read, so nothing after it compares the name.
	BaseIsMain bool
	// CrossRepository is true when the PR's head branch is in another
	// repository.
	CrossRepository bool
}

// errGitOutput reports a git output chief cannot read.
var errGitOutput = errors.New("unreadable git output")

// errPRList reports a `gh pr list` output chief cannot read.
var errPRList = errors.New("unreadable gh pr list output")

// prListFields are the members `gh pr list --json` is asked for, each of
// which parsePRList requires.
const prListFields = "headRefName,number,title,url,mergeable,headRefOid,baseRefName,isCrossRepository"

// parsePRList reads the output of `gh pr list --json` with prListFields.
// Contract: every element is read in full or the whole output is rejected,
// so one element of another shape makes the workspace's PRs unreadable; an
// element whose head branch is not the branch asked for is rejected, so a PR
// of another branch is never shown as this workspace's; the PR's values are
// untrusted input accepted only in the forms parseMergeable, ParseCommitID,
// a non-empty base, and a JSON boolean give, a member that is missing, null,
// of another kind, or repeated being rejected.
func parsePRList(out []byte, branch string) ([]OpenPR, error) {
	items, err := strictjson.ParseArray(out)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPRList, err)
	}
	prs := make([]OpenPR, 0, len(items))
	for i, item := range items {
		object, err := item.AsObject()
		if err != nil {
			return nil, fmt.Errorf("%w: element %d: %w", errPRList, i, err)
		}
		fields, err := object.Collect(strings.Split(prListFields, ",")...)
		if err != nil {
			return nil, fmt.Errorf("%w: element %d: %w", errPRList, i, err)
		}
		head, err := strictjson.RequiredString(fields, "headRefName")
		if err != nil || head != branch {
			return nil, fmt.Errorf("%w: element %d: head branch", errPRList, i)
		}
		numberValue, err := strictjson.Required(fields, "number")
		if err != nil {
			return nil, fmt.Errorf("%w: element %d: %w", errPRList, i, err)
		}
		number, err := numberValue.AsNonNegativeInt64()
		if err != nil || number == 0 || number > 1<<31-1 {
			return nil, fmt.Errorf("%w: element %d: number", errPRList, i)
		}
		title, err := strictjson.RequiredStringAllowEmpty(fields, "title")
		if err != nil {
			return nil, fmt.Errorf("%w: element %d: %w", errPRList, i, err)
		}
		url, err := strictjson.RequiredString(fields, "url")
		if err != nil {
			return nil, fmt.Errorf("%w: element %d: %w", errPRList, i, err)
		}
		pr := OpenPR{Number: int(number), Title: title, URL: url}
		if err := prConflictValues(fields, &pr); err != nil {
			return nil, fmt.Errorf("%w: element %d: %w", errPRList, i, err)
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

// prConflictValues reads into pr the four members of a PR element that
// decide a merge conflict, in the forms parsePRList states.
func prConflictValues(fields map[string]strictjson.Value, pr *OpenPR) error {
	text, err := strictjson.RequiredString(fields, "mergeable")
	if err != nil {
		return err
	}
	if pr.Mergeable, err = parseMergeable(text); err != nil {
		return err
	}
	if text, err = strictjson.RequiredString(fields, "headRefOid"); err != nil {
		return err
	}
	if pr.Head, err = ParseCommitID(text); err != nil {
		return err
	}
	base, err := strictjson.RequiredString(fields, "baseRefName")
	if err != nil {
		return err
	}
	// The one comparison of a PR's base with main, as bytes.
	pr.BaseIsMain = base == mainBranch
	crossValue, err := strictjson.Required(fields, "isCrossRepository")
	if err != nil {
		return err
	}
	if pr.CrossRepository, err = crossValue.AsBool(); err != nil {
		return fmt.Errorf("isCrossRepository: %w", err)
	}
	return nil
}

// GitReader reads git and GitHub state of worktrees.
type GitReader struct {
	cmds Commands
}

// NewGitReader returns the reader that runs git and gh through cmds.
func NewGitReader(cmds Commands) GitReader { return GitReader{cmds: cmds} }

// Fetch updates origin/main from chief's own worktree. It runs once per
// round: refs/remotes/origin/main is shared by every worktree, and /runpr
// and /mergepr fetch it too, so one fetch per workspace would collide with
// them more often.
func (g GitReader) Fetch(ctx context.Context, chiefTree string) error {
	_, err := g.cmds.gitLimit(ctx, gitFetchTimeout, chiefTree, "fetch", "origin", "main")
	return err
}

// CommonDir returns the git common directory of the repository dir belongs
// to, as an absolute path with links resolved.
func (g GitReader) CommonDir(ctx context.Context, dir string) (string, error) {
	out, err := g.cmds.git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	common := strings.TrimSpace(string(out))
	if !filepath.IsAbs(common) {
		return "", fmt.Errorf("%w: git common dir %q is not absolute", errGitOutput, common)
	}
	return filepath.EvalSymlinks(common)
}

// detachedHead is what `git rev-parse --abbrev-ref HEAD` prints when HEAD is
// no branch.
const detachedHead = "HEAD"

// Branch returns the branch checked out in worktree, "" when HEAD is
// detached.
func (g GitReader) Branch(ctx context.Context, worktree string) (string, error) {
	out, err := g.cmds.git(ctx, worktree, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	switch branch {
	case "":
		return "", fmt.Errorf("%w: no branch name", errGitOutput)
	case detachedHead:
		return "", nil
	}
	return branch, nil
}

// Head returns the commit at HEAD of worktree.
func (g GitReader) Head(ctx context.Context, worktree string) (string, error) {
	out, err := g.cmds.git(ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	head := strings.TrimSpace(string(out))
	if !commitID.MatchString(head) {
		return "", fmt.Errorf("%w: HEAD %q", errGitOutput, head)
	}
	return head, nil
}

// OpenPRs returns the open pull requests whose head is branch, read by gh in
// worktree so the repository is the worktree's own remote.
func (g GitReader) OpenPRs(ctx context.Context, worktree, branch string) ([]OpenPR, error) {
	out, err := g.cmds.gh(ctx, worktree, "pr", "list", "--head", branch, "--state", "open", "--json", prListFields)
	if err != nil {
		return nil, err
	}
	return parsePRList(out, branch)
}

// Ahead returns how many commits worktree's HEAD has beyond origin/main.
func (g GitReader) Ahead(ctx context.Context, worktree string) (int, error) {
	out, err := g.cmds.git(ctx, worktree, "rev-list", "--count", originMain+"..HEAD")
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: rev-list count %q", errGitOutput, strings.TrimSpace(string(out)))
	}
	return n, nil
}

// TaskWorkspace is one Herdr workspace whose agents work in a worktree of
// this repository.
type TaskWorkspace struct {
	ID    string
	Label string
	// Worktree is the canonical path of the workspace's worktree, and Tree
	// the same path as the type records are kept by.
	Worktree string
	Tree     Worktree
	// Agents are the workspace's agents in Herdr's order.
	Agents []Agent
}

// FindTaskWorkspaces decides which workspaces are task workspaces. The
// chief's workspace, the one of chiefPane, is never one of them. Another
// workspace is one when an agent's working directory lies in a worktree with
// the same git common directory as chief's. Contract: the result is in
// workspace-path order; a directory git cannot read as a repository is not a
// task workspace's (chief's own worktree, which must be readable, shows git
// itself works); a workspace whose agents lie in different worktrees is
// listed once, by its first agent's worktree.
func FindTaskWorkspaces(ctx context.Context, git GitReader, cmds Commands, agents []Agent, workspaces []Workspace, chiefPane PaneID) ([]TaskWorkspace, error) {
	var chief *Agent
	for i := range agents {
		if agents[i].Pane == chiefPane {
			chief = &agents[i]
		}
	}
	if chief == nil {
		return nil, fmt.Errorf("%w: the chief pane %s is not among Herdr's agents", errHerdrOutput, chiefPane)
	}
	chiefCommon, err := git.CommonDir(ctx, chief.Dir)
	if err != nil {
		return nil, fmt.Errorf("read the chief worktree's git directory: %w", err)
	}
	labels := map[string]string{}
	for _, w := range workspaces {
		labels[w.ID] = w.Label
	}
	var order []string
	byWorkspace := map[string][]Agent{}
	for _, a := range agents {
		if a.Workspace == chief.Workspace {
			continue
		}
		if _, seen := byWorkspace[a.Workspace]; !seen {
			order = append(order, a.Workspace)
		}
		byWorkspace[a.Workspace] = append(byWorkspace[a.Workspace], a)
	}
	var found []TaskWorkspace
	for _, id := range order {
		for _, a := range byWorkspace[id] {
			common, err := git.CommonDir(ctx, a.Dir)
			if err != nil || common != chiefCommon {
				continue
			}
			tree, err := CanonicalWorktree(ctx, cmds, a.Dir)
			if err != nil {
				continue
			}
			found = append(found, TaskWorkspace{ID: id, Label: labels[id], Worktree: tree.String(), Tree: tree, Agents: byWorkspace[id]})
			break
		}
	}
	slices.SortFunc(found, func(a, b TaskWorkspace) int { return strings.Compare(a.Worktree, b.Worktree) })
	return found, nil
}

// taskOfBranch returns the task a branch names and whether it follows the
// feature-branch convention.
func taskOfBranch(branch string) (paneinput.TaskID, bool) {
	task, _, ok := branchTask(branch)
	return task, ok
}

// Clean reports whether worktree has no uncommitted change and no untracked
// file that is not ignored, by `git status --porcelain`. Untracked files are
// asked for by the option, so a configuration that hides them does not make
// a worktree read as clean.
func (g GitReader) Clean(ctx context.Context, worktree string) (bool, error) {
	out, err := g.cmds.git(ctx, worktree, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return false, err
	}
	return len(out) == 0, nil
}

// gitWorktree is the git subcommand the worktree operations share.
const gitWorktree = "worktree"

// WorktreeEntry is one registration `git worktree list --porcelain` reports.
type WorktreeEntry struct {
	// Path is the registered path, as git prints it (not made canonical).
	Path string
	// Locked reports a `locked` line: the worktree is pinned against removal,
	// with or without a reason.
	Locked bool
	// Prunable reports a `prunable` line: the registration is left over for a
	// directory that is gone.
	Prunable bool
}

// Worktrees returns every worktree registered in the repository dir belongs
// to, read with `git worktree list --porcelain`. Contract: every block is read
// in full or the whole output is rejected; a block whose first line is not an
// absolute `worktree <path>` (a path git quotes is not absolute), a repeated path, or an
// output with no block is unreadable. Lines other than `worktree`, `locked`,
// and `prunable` (HEAD, branch, detached, bare) are not read.
func (g GitReader) Worktrees(ctx context.Context, dir string) ([]WorktreeEntry, error) {
	out, err := g.cmds.git(ctx, dir, gitWorktree, "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(out)
}

// parseWorktreeList reads the output of `git worktree list --porcelain`.
func parseWorktreeList(out []byte) ([]WorktreeEntry, error) {
	text, ok := strings.CutSuffix(string(out), "\n\n")
	if !ok && !strings.HasSuffix(string(out), "\n") {
		return nil, fmt.Errorf("%w: worktree list does not end with a line end", errGitOutput)
	}
	text = strings.TrimSuffix(text, "\n")
	var entries []WorktreeEntry
	for block := range strings.SplitSeq(text, "\n\n") {
		lines := strings.Split(block, "\n")
		path, ok := strings.CutPrefix(lines[0], "worktree ")
		if !ok || !filepath.IsAbs(path) {
			return nil, fmt.Errorf("%w: worktree list block does not start with an absolute worktree path", errGitOutput)
		}
		if slices.ContainsFunc(entries, func(e WorktreeEntry) bool { return e.Path == path }) {
			return nil, fmt.Errorf("%w: worktree list repeats a path", errGitOutput)
		}
		entry := WorktreeEntry{Path: path}
		for _, line := range lines[1:] {
			word, _, _ := strings.Cut(line, " ")
			switch word {
			case "locked":
				entry.Locked = true
			case "prunable":
				entry.Prunable = true
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// IsAncestor reports whether ancestor is an ancestor of descendant (or the
// same commit), read with `git merge-base --is-ancestor`. Contract: exit
// status 0 is yes and 1 is no; every other outcome (a commit that does not
// exist, git failing) is an error, never a no. Both arguments must be full
// commit IDs, so neither can be read as an option or a ref name.
func (g GitReader) IsAncestor(ctx context.Context, dir, ancestor, descendant string) (bool, error) {
	if !commitID.MatchString(ancestor) || !commitID.MatchString(descendant) {
		return false, fmt.Errorf("%w: not a full commit ID", errGitOutput)
	}
	_, err := g.cmds.git(ctx, dir, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
		return false, nil
	}
	return false, err
}

// Parents returns the parents of commit, in order, read with
// `git rev-list --parents -n 1`. Contract: commit is a CommitID, so it
// cannot be read as an option or a ref name; the first field git prints must
// be commit itself and every other field a full commit ID, so any other
// output is an error, never a shorter list.
func (g GitReader) Parents(ctx context.Context, dir string, commit CommitID) ([]CommitID, error) {
	if commit.IsZero() {
		return nil, fmt.Errorf("%w: no commit", errGitOutput)
	}
	out, err := g.cmds.git(ctx, dir, "rev-list", "--parents", "-n", "1", commit.String())
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 || fields[0] != commit.String() {
		return nil, fmt.Errorf("%w: rev-list --parents of %s", errGitOutput, commit)
	}
	parents := make([]CommitID, 0, len(fields)-1)
	for _, f := range fields[1:] {
		p, err := ParseCommitID(f)
		if err != nil {
			return nil, fmt.Errorf("%w: parent %q of %s", errGitOutput, f, commit)
		}
		parents = append(parents, p)
	}
	return parents, nil
}

// HasCommit reports whether commit is a commit this repository holds, read
// with `git rev-parse --verify --quiet`. Contract: exit status 0 is yes and
// 1 is no; every other outcome is an error, never a no. commit is a
// CommitID, so it cannot be read as an option or a ref name.
func (g GitReader) HasCommit(ctx context.Context, dir string, commit CommitID) (bool, error) {
	if commit.IsZero() {
		return false, fmt.Errorf("%w: no commit", errGitOutput)
	}
	_, err := g.cmds.git(ctx, dir, "rev-parse", "--verify", "--quiet", commit.String()+"^{commit}")
	if err == nil {
		return true, nil
	}
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
		return false, nil
	}
	return false, err
}

// branchForRefspec is the form of a branch name FetchBranch accepts: no
// option or refspec syntax can be read out of it.
var branchForRefspec = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._/-]{0,199}$`)

// FetchBranch fetches the one branch of origin into refs/remotes/origin/<branch>
// for a display that needs a commit the worktree lacks. Contract: the refspec
// names exactly that branch and that remote-tracking ref, and --no-tags and
// --no-prune and an empty --refmap (which drops the remote.origin.fetch
// mappings) keep the fetch from writing any other ref (origin/main
// included); main and HEAD in any letter case (a case-insensitive file system makes
// refs/remotes/origin/Main the same file as origin/main), whose tracking refs
// other readers use, a branch name outside branchForRefspec, or with "..", "//", a
// trailing "/" or ".", or a ".lock" suffix, is refused without running git.
func (g GitReader) FetchBranch(ctx context.Context, dir, branch string) error {
	if strings.EqualFold(branch, mainBranch) || strings.EqualFold(branch, "HEAD") || !branchForRefspec.MatchString(branch) || strings.Contains(branch, "..") || strings.Contains(branch, "//") ||
		strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("%w: branch name %q", errGitOutput, branch)
	}
	refspec := "+refs/heads/" + branch + ":refs/remotes/origin/" + branch
	_, err := g.cmds.gitLimit(ctx, gitFetchTimeout, dir, "fetch", "--no-tags", "--no-prune", "--no-write-fetch-head", "--refmap=", "origin", refspec)
	return err
}

// OriginMain returns the commit origin/main points at, as worktree sees it.
func (g GitReader) OriginMain(ctx context.Context, worktree string) (string, error) {
	out, err := g.cmds.git(ctx, worktree, "rev-parse", originMain)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	if !commitID.MatchString(id) {
		return "", fmt.Errorf("%w: origin/main %q", errGitOutput, id)
	}
	return id, nil
}

// remoteMainRef is the ref `git ls-remote` is asked for.
const remoteMainRef = "refs/heads/main"

// RemoteMain returns the commit main points at on the remote, read with
// `git ls-remote` in chief's worktree. Contract: the output is exactly one
// line naming that ref, or it is rejected.
func (g GitReader) RemoteMain(ctx context.Context, chiefTree string) (string, error) {
	out, err := g.cmds.git(ctx, chiefTree, "ls-remote", "origin", remoteMainRef)
	if err != nil {
		return "", err
	}
	line, ok := strings.CutSuffix(string(out), "\n")
	id, ref, tab := strings.Cut(line, "\t")
	if !ok || !tab || ref != remoteMainRef || !commitID.MatchString(id) || strings.Contains(line, "\n") {
		return "", fmt.Errorf("%w: git ls-remote gave %q", errGitOutput, string(out))
	}
	return id, nil
}

// PRState is a PR's state on GitHub. The zero value is PRStateUnknown, which
// no reader returns: a state of another value is unreadable.
type PRState int

const (
	// PRStateUnknown is no state.
	PRStateUnknown PRState = iota
	// PRStateOpen is an open PR.
	PRStateOpen
	// PRStateMerged is a merged PR.
	PRStateMerged
	// PRStateClosed is a PR closed without a merge.
	PRStateClosed
)

// ghOpen is the state gh gives an open pull request or issue.
const ghOpen = "OPEN"

// prStates maps gh's state values.
var prStates = map[string]PRState{ghOpen: PRStateOpen, "MERGED": PRStateMerged, "CLOSED": PRStateClosed}

// prStateFields are the fields PRStateOf asks gh for.
const prStateFields = "number,state,headRefName"

// PRStateOf returns the state and the head branch of the PR number, read by
// gh in dir. Contract: the output must name that number, a known state, and
// a head branch, or it is rejected.
func (g GitReader) PRStateOf(ctx context.Context, dir string, number int) (PRState, string, error) {
	out, err := g.cmds.gh(ctx, dir, "pr", "view", strconv.Itoa(number), "--json", prStateFields)
	if err != nil {
		return PRStateUnknown, "", err
	}
	return parsePRState(out, number)
}

// parsePRState reads `gh pr view --json number,state,headRefName`.
func parsePRState(out []byte, number int) (PRState, string, error) {
	object, err := strictjson.ParseObject(out)
	if err != nil {
		return PRStateUnknown, "", fmt.Errorf("%w: %w", errGHOutput, err)
	}
	fields, err := object.Collect("number", "state", "headRefName")
	if err != nil {
		return PRStateUnknown, "", fmt.Errorf("%w: %w", errGHOutput, err)
	}
	numberValue, err := strictjson.Required(fields, "number")
	if err != nil {
		return PRStateUnknown, "", fmt.Errorf("%w: %w", errGHOutput, err)
	}
	n, err := numberValue.AsNonNegativeInt64()
	if err != nil || n != int64(number) {
		return PRStateUnknown, "", fmt.Errorf("%w: the view is not of #%d", errGHOutput, number)
	}
	stateText, err := strictjson.RequiredString(fields, "state")
	if err != nil {
		return PRStateUnknown, "", fmt.Errorf("%w: %w", errGHOutput, err)
	}
	state, ok := prStates[stateText]
	if !ok {
		return PRStateUnknown, "", fmt.Errorf("%w: unknown PR state %q", errGHOutput, stateText)
	}
	head, err := strictjson.RequiredString(fields, "headRefName")
	if err != nil {
		return PRStateUnknown, "", fmt.Errorf("%w: %w", errGHOutput, err)
	}
	return state, head, nil
}
