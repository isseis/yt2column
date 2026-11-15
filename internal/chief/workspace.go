package chief

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/strictjson"
)

// OpenPR is an open pull request of a worktree's branch.
type OpenPR struct {
	Number int
	Title  string
	URL    string
}

// errGitOutput reports a git output chief cannot read.
var errGitOutput = errors.New("unreadable git output")

// errPRList reports a `gh pr list` output chief cannot read.
var errPRList = errors.New("unreadable gh pr list output")

// parsePRList reads the output of `gh pr list --json
// headRefName,number,title,url`. Contract: every element is read in full or
// the whole output is rejected, and an element whose head branch is not the
// branch asked for is rejected, so a PR of another branch is never shown as
// this workspace's.
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
		fields, err := object.Collect("headRefName", "number", "title", "url")
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
		prs = append(prs, OpenPR{Number: int(number), Title: title, URL: url})
	}
	return prs, nil
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
	out, err := g.cmds.gh(ctx, worktree, "pr", "list", "--head", branch, "--state", "open", "--json", "headRefName,number,title,url")
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

// prStates maps gh's state values.
var prStates = map[string]PRState{"OPEN": PRStateOpen, "MERGED": PRStateMerged, "CLOSED": PRStateClosed}

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
