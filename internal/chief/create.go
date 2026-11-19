package chief

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The time values of a creation (the plan, decision 6).
const (
	// createLimit bounds a whole creation, shorter than the ten minutes the
	// /chief session gives the call, so the tool writes its output before it
	// is stopped: two agent starts of up to a minute each and the /mkreq send
	// with its checks fit in it.
	createLimit = 7 * time.Minute
	// agentStartWait is the --timeout `herdr agent start` waits for the agent
	// to take input, in milliseconds as herdr takes it.
	agentStartWait = "60000"
)

// The tab labels of a task's workspace, as the hand-made workspaces have
// them: the shell, OpenCode, and Claude Code, one pane each. The agents'
// tabs are named by Herdr's names of their agents.
const (
	tabShell    = "cmd"
	tabOpenCode = herdrOpenCode
	tabClaude   = herdrClaude
)

// CreateRefusal says why nothing was created. The zero value is never
// treated as permission to create.
type CreateRefusal int

const (
	// CreateRefusalUnknown is no refusal.
	CreateRefusalUnknown CreateRefusal = iota
	// CreateRefusalIssueForm is an issue number of another form than
	// paneinput.ParseIssueNumber accepts.
	CreateRefusalIssueForm
	// CreateRefusalIssueNotFound is a number that names nothing.
	CreateRefusalIssueNotFound
	// CreateRefusalIssueClosed is a closed issue.
	CreateRefusalIssueClosed
	// CreateRefusalIssueDraft is an open issue with the label draft.
	CreateRefusalIssueDraft
	// CreateRefusalIssueIsPR is a number that names a pull request.
	CreateRefusalIssueIsPR
	// CreateRefusalIssueUnreadable is an answer of no accepted shape.
	CreateRefusalIssueUnreadable
	// CreateRefusalTargetExists is a path, a worktree registration, or a
	// workspace already there; Existing names it.
	CreateRefusalTargetExists
	// CreateRefusalPathUnsupported is a path chief cannot send to.
	CreateRefusalPathUnsupported
	// CreateRefusalFetchFailed is a failed fetch of origin/main.
	CreateRefusalFetchFailed
	// CreateRefusalBusy is a creation, a tick's automatic sends, or a send
	// to the same path under way.
	CreateRefusalBusy
)

// createRefusalNames are the output spellings of the refusals.
var createRefusalNames = map[CreateRefusal]string{
	CreateRefusalIssueForm:       "issue_form",
	CreateRefusalIssueNotFound:   "issue_not_found",
	CreateRefusalIssueClosed:     "issue_closed",
	CreateRefusalIssueDraft:      "issue_draft",
	CreateRefusalIssueIsPR:       "issue_is_pr",
	CreateRefusalIssueUnreadable: "issue_unreadable",
	CreateRefusalTargetExists:    "target_exists",
	CreateRefusalPathUnsupported: namePathUnsupported,
	CreateRefusalFetchFailed:     "fetch_failed",
	CreateRefusalBusy:            nameBusy,
}

// String returns the output spelling, "" for no refusal.
func (r CreateRefusal) String() string { return createRefusalNames[r] }

// issueRefusals maps every issue status but an open issue to its refusal.
var issueRefusals = map[IssueStatus]CreateRefusal{
	IssueUnreadable:  CreateRefusalIssueUnreadable,
	IssueDraft:       CreateRefusalIssueDraft,
	IssueClosed:      CreateRefusalIssueClosed,
	IssuePullRequest: CreateRefusalIssueIsPR,
	IssueNotFound:    CreateRefusalIssueNotFound,
}

// CreateStage names the stage a creation stopped at. The zero value is no
// stage.
type CreateStage int

const (
	// CreateStageUnknown is no stage.
	CreateStageUnknown CreateStage = iota
	// CreateStageRecords removes the records an earlier use of the path left.
	CreateStageRecords
	// CreateStageWorktree adds the worktree.
	CreateStageWorktree
	// CreateStageRecord writes the created record.
	CreateStageRecord
	// CreateStageWorkspace creates the workspace.
	CreateStageWorkspace
	// CreateStageTabs renames the first tab and creates the other two.
	CreateStageTabs
	// CreateStageOpenCode starts OpenCode.
	CreateStageOpenCode
	// CreateStageClaude starts Claude Code.
	CreateStageClaude
	// CreateStagePanes reads the agents again and checks them.
	CreateStagePanes
	// CreateStageDeadline is the time limit of the whole creation passing.
	CreateStageDeadline
)

// createStageNames are the output spellings of the stages.
var createStageNames = map[CreateStage]string{
	CreateStageRecords:   "records",
	CreateStageWorktree:  "worktree",
	CreateStageRecord:    "record",
	CreateStageWorkspace: "workspace", //nolint:goconst // an output spelling, which the /chief definition reads
	CreateStageTabs:      "tabs",
	CreateStageOpenCode:  herdrOpenCode,
	CreateStageClaude:    herdrClaude,
	CreateStagePanes:     "panes",
	CreateStageDeadline:  "deadline",
}

// String returns the output spelling, "" for no stage.
func (s CreateStage) String() string { return createStageNames[s] }

// CreateResult is the kind of a creation's outcome; only the fields of that
// kind are set. The zero value is no result and is never shown as done.
type CreateResult int

const (
	// CreateResultUnknown is no result.
	CreateResultUnknown CreateResult = iota
	// CreateRefused made nothing: Refusal says why.
	CreateRefused
	// CreateStopped stopped at Stage: Made lists what was made, and /mkreq
	// was not sent.
	CreateStopped
	// CreateMade made everything: Made lists it, and Mkreq or NotReady says
	// what became of /mkreq.
	CreateMade
)

// createResultNames are the output spellings of the results.
var createResultNames = map[CreateResult]string{
	CreateRefused: "refused",
	CreateStopped: nameStopped,
	CreateMade:    "made",
}

// String returns the output spelling, "" for no result.
func (r CreateResult) String() string { return createResultNames[r] }

// MadeKind is the kind of a thing a creation made. The zero value is no
// kind.
type MadeKind int

const (
	// MadeUnknown is no kind.
	MadeUnknown MadeKind = iota
	// MadeWorktree is the worktree's directory, by its path.
	MadeWorktree
	// MadeRecord is the created record, by its path.
	MadeRecord
	// MadeWorkspace is the workspace, by its Herdr ID.
	MadeWorkspace
	// MadeTab is a tab, by its Herdr ID.
	MadeTab
	// MadePane is a pane, by its Herdr ID.
	MadePane
	// MadeAgent is a started agent, by its Herdr name.
	MadeAgent
	// MadeRegistration is git's registration of a worktree, by its path; a
	// removal that stops lists it when git still registers the worktree.
	MadeRegistration
)

// madeKindNames are the output spellings of the kinds.
var madeKindNames = map[MadeKind]string{
	MadeWorktree:     "worktree",
	MadeRecord:       "record",
	MadeWorkspace:    "workspace",
	MadeTab:          "tab",
	MadePane:         "pane", //nolint:goconst // an output spelling, which the /chief definition reads
	MadeAgent:        "agent",
	MadeRegistration: "registration",
}

// String returns the output spelling, "" for no kind.
func (k MadeKind) String() string { return madeKindNames[k] }

// Made is one thing a creation made, or one a removal that stopped left,
// for the human to see or clean up.
type Made struct {
	Kind MadeKind
	// ID is the path, or the ID or name Herdr returned.
	ID string
	// Label is a workspace's label as Herdr reported it, the name the
	// human sees in Herdr; it is empty for every other kind and when
	// Herdr's output could not be read (ID is then the label asked for).
	Label string
}

// NotReady is Claude Code that Herdr started but reported not yet taking
// input: /mkreq was not sent to it. Code and Message are Herdr's and only
// shown; Status is what `herdr agent get` read, which says whether the human
// answers a question there (blocked) or looks at the pane.
type NotReady struct {
	Pane    PaneID
	Status  AgentStatus
	Code    string
	Message string
}

// CreateOutcome is what `chief create` did.
type CreateOutcome struct {
	Result CreateResult
	Issue  paneinput.IssueNumber
	// Title is the issue's title, already made visible and cut (Issue.Title);
	// it decides nothing.
	Title    string
	TitleCut bool
	Refusal  CreateRefusal
	// Existing names what is already at the path (target_exists), and
	// Failure says more of a refusal or of the stage stopped at.
	Existing []string
	Made     []Made
	Stage    CreateStage
	Failure  string
	// Workspace is the made workspace's worktree, the identity the other
	// subcommands take, once it is made.
	Workspace string
	Label     string
	// Mkreq is the result of the /mkreq send; NotReady is set instead when
	// Claude Code did not take input.
	Mkreq    *ActionResult
	NotReady *NotReady
}

// Create makes the worktree and the workspace of a new task for the issue
// numbered issueArg, starts its agents, and sends /mkreq (the design, 3.3).
// Contract: nothing is made unless the number has the issue form, the task
// space has a path chief can send to, GitHub says the number is an open
// issue with no draft label, origin/main was fetched, the creation
// exclusion and the path's workspace exclusion are both held, and nothing
// is at the path (a file, a directory, a link, a worktree registration, or
// a workspace with a pane there); the path and every name come from the
// number alone; Herdr is written only through a WorkspaceMaker of the task
// space; a stage that fails stops the creation there, with what was made
// listed and nothing removed or sent; /mkreq is sent only when every stage
// passed and Claude Code started taking input, by sendAfterCreate under
// the exclusion held since before anything was made. An error is returned
// only before anything was made.
func Create(ctx context.Context, deps Deps, issueArg string) (CreateOutcome, error) {
	ctx, cancel := context.WithTimeout(ctx, createLimit)
	defer cancel()
	issue, err := paneinput.ParseIssueNumber(issueArg)
	if err != nil {
		return refusedCreate(CreateOutcome{}, CreateRefusalIssueForm, err.Error()), nil
	}
	out := CreateOutcome{Issue: issue}
	space, err := NewTaskSpace(deps.Env.Home, issue)
	if err != nil {
		return refusedCreate(out, CreateRefusalPathUnsupported, err.Error()), nil
	}
	if _, err := space.Worktree(); err != nil {
		if refusal, ok := spaceRefusal(err); ok {
			return refusedAt(out, refusal, space.Path(), err), nil
		}
		return CreateOutcome{}, err
	}
	herdr := NewHerdr(deps.Cmds)
	git := NewGitReader(deps.Cmds)
	chiefTree, found, refusal, failure, err := readOpenIssue(ctx, deps, herdr, git, issue)
	if err != nil {
		return CreateOutcome{}, err
	}
	out.Title, out.TitleCut = found.Title, found.TitleCut
	if refusal != CreateRefusalUnknown {
		return refusedCreate(out, refusal, failure), nil
	}
	if err := git.Fetch(ctx, chiefTree.String()); err != nil {
		return refusedCreate(out, CreateRefusalFetchFailed, err.Error()), nil
	}
	commit, err := git.OriginMain(ctx, chiefTree.String())
	if err != nil {
		return refusedCreate(out, CreateRefusalFetchFailed, "origin/main cannot be read after the fetch: "+err.Error()), nil
	}
	creation, err := deps.Store.LockCreation()
	if errors.Is(err, ErrCreationBusy) {
		return refusedCreate(out, CreateRefusalBusy, "a creation or an automatic send is under way"), nil
	}
	if err != nil {
		return CreateOutcome{}, err
	}
	defer func() { _ = creation.Close() }()
	// The space is resolved again under the exclusions: what is at the path
	// is judged only while no other creation can make it.
	tree, err := space.Worktree()
	if err != nil {
		if refusal, ok := spaceRefusal(err); ok {
			return refusedAt(out, refusal, space.Path(), err), nil
		}
		return CreateOutcome{}, err
	}
	records, err := deps.Store.Records(tree)
	if err != nil {
		return CreateOutcome{}, err
	}
	held, err := records.Lock()
	if errors.Is(err, ErrWorkspaceBusy) {
		return refusedCreate(out, CreateRefusalBusy, "a send to the path or its removal is under way"), nil
	}
	if err != nil {
		return CreateOutcome{}, err
	}
	defer func() { _ = held.Close() }()
	existing, err := existingAt(ctx, herdr, git, chiefTree, space, tree)
	if err != nil {
		return CreateOutcome{}, err
	}
	if len(existing) > 0 {
		out.Existing = existing
		return refusedCreate(out, CreateRefusalTargetExists, ""), nil
	}
	return makeTask(ctx, deps, held, chiefTree, space, tree, commit, out), nil
}

// refusedCreate is out refused for refusal.
func refusedCreate(out CreateOutcome, refusal CreateRefusal, failure string) CreateOutcome {
	out.Result, out.Refusal, out.Failure = CreateRefused, refusal, failure
	return out
}

// refusedAt is out refused for refusal, naming path as what exists when the
// refusal is target_exists.
func refusedAt(out CreateOutcome, refusal CreateRefusal, path string, err error) CreateOutcome {
	if refusal == CreateRefusalTargetExists {
		out.Existing = []string{path}
	}
	return refusedCreate(out, refusal, err.Error())
}

// spaceRefusal maps the errors of TaskSpace.Worktree that are refusals.
func spaceRefusal(err error) (CreateRefusal, bool) {
	switch {
	case errors.Is(err, errUnsupportedPath):
		return CreateRefusalPathUnsupported, true
	case errors.Is(err, errTargetExists):
		return CreateRefusalTargetExists, true
	}
	return CreateRefusalUnknown, false
}

// chiefWorktree reads chief's own worktree from the pane chief runs in.
// readOpenIssue reads issue from GitHub in chief's worktree, which it also
// returns. Contract: the refusal is CreateRefusalUnknown only for an open
// issue without the draft label; an answer that cannot be read, a gh that
// fails, and a status issueRefusals does not know are
// CreateRefusalIssueUnreadable, with failure saying why. An error is
// returned only when chief's worktree cannot be found, before GitHub is
// asked.
func readOpenIssue(ctx context.Context, deps Deps, herdr Herdr, git GitReader, issue paneinput.IssueNumber) (Worktree, Issue, CreateRefusal, string, error) {
	chiefTree, err := chiefWorktree(ctx, deps, herdr)
	if err != nil {
		return Worktree{}, Issue{}, CreateRefusalUnknown, "", err
	}
	found, err := git.ReadIssue(ctx, chiefTree.String(), issue)
	if err != nil {
		return chiefTree, Issue{}, CreateRefusalIssueUnreadable, err.Error(), nil
	}
	if found.Status == IssueOpen {
		return chiefTree, found, CreateRefusalUnknown, "", nil
	}
	refusal, ok := issueRefusals[found.Status]
	if !ok {
		// A status this map does not know is not read more widely.
		refusal = CreateRefusalIssueUnreadable
	}
	return chiefTree, found, refusal, "", nil
}

func chiefWorktree(ctx context.Context, deps Deps, herdr Herdr) (Worktree, error) {
	chiefPane, err := ParsePaneID(deps.Env.PaneID)
	if err != nil {
		return Worktree{}, err
	}
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return Worktree{}, err
	}
	chief, err := chiefWorkspaceOf(agents, chiefPane)
	if err != nil {
		return Worktree{}, err
	}
	tree, err := CanonicalWorktree(ctx, deps.Cmds, chief.Dir)
	if err != nil {
		return Worktree{}, fmt.Errorf("read the chief worktree: %w", err)
	}
	return tree, nil
}

// existingAt returns what is already at the task space's path: an entry of
// the file system there (a link too), a worktree git registers there (one
// whose directory is gone too), and a workspace with a pane there. A read
// that fails is an error: nothing is made over what could not be read.
func existingAt(ctx context.Context, herdr Herdr, git GitReader, chiefTree Worktree, space TaskSpace, tree Worktree) ([]string, error) {
	var existing []string
	for _, path := range uniquePaths(space.Path(), tree.path) {
		_, err := os.Lstat(path)
		switch {
		case err == nil:
			existing = append(existing, "path "+path)
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
	}
	entries, err := git.Worktrees(ctx, chiefTree.String())
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Path == space.Path() || e.Path == tree.path || resolvedPath(e.Path) == tree.path {
			existing = append(existing, "worktree "+e.Path)
		}
	}
	panes, err := herdr.panes(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range workspacesAtPath(panes, tree) {
		existing = append(existing, "workspace "+id)
	}
	return existing, nil
}

// uniquePaths returns the paths without repeats.
func uniquePaths(paths ...string) []string {
	var out []string
	for _, p := range paths {
		if p != "" && (len(out) == 0 || out[len(out)-1] != p) {
			out = append(out, p)
		}
	}
	return out
}

// resolvedPath returns path with the links of its deepest existing
// ancestor resolved, so a registration git kept for a directory that is
// gone, reached through a link, still compares with a canonical path. A
// path that cannot be resolved is returned as it is.
func resolvedPath(path string) string {
	ancestor, rest, err := nearestExisting(filepath.Clean(path))
	if err != nil {
		return path
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return path
	}
	return filepath.Join(append([]string{resolved}, rest...)...)
}

// addTaskWorktree adds the worktree of space at commit, on a detached HEAD,
// with git run in chief's worktree. Contract: it takes a TaskSpace and
// never a path, so no worktree is added anywhere but at a task space; it
// creates no branch (--detach); commit must be a full commit ID.
func addTaskWorktree(ctx context.Context, cmds Commands, chiefTree Worktree, space TaskSpace, commit string) error {
	tree, err := space.Worktree()
	if err != nil {
		return err
	}
	if !commitID.MatchString(commit) {
		return fmt.Errorf("%w: %q is not a full commit ID", errGitOutput, commit)
	}
	_, err = cmds.git(ctx, chiefTree.String(), gitWorktree, "add", "--detach", tree.path, commit)
	return err
}

// makeTask runs the stages of a creation once every check passed, with
// held the exclusion of tree's records.
func makeTask(ctx context.Context, deps Deps, held *HeldWorkspace, chiefTree Worktree, space TaskSpace, tree Worktree, commit string, out CreateOutcome) CreateOutcome {
	stop := func(stage CreateStage, err error) CreateOutcome {
		out.Result, out.Stage, out.Failure = CreateStopped, stage, err.Error()
		if ctx.Err() != nil {
			out.Stage, out.Failure = CreateStageDeadline, fmt.Sprintf("the time limit passed at stage %s: %v", stage, err)
		}
		return out
	}
	if err := held.ClearRecords(); err != nil {
		return stop(CreateStageRecords, err)
	}
	if err := ctx.Err(); err != nil {
		return stop(CreateStageWorktree, err)
	}
	if err := addTaskWorktree(ctx, deps.Cmds, chiefTree, space, commit); err != nil {
		// git may leave the directory behind when it stops halfway.
		if _, statErr := os.Lstat(tree.path); statErr == nil {
			out.Made = append(out.Made, Made{Kind: MadeWorktree, ID: tree.path})
		}
		return stop(CreateStageWorktree, err)
	}
	out.Made = append(out.Made, Made{Kind: MadeWorktree, ID: tree.path})
	made, err := CanonicalWorktree(ctx, deps.Cmds, tree.path)
	if err != nil {
		return stop(CreateStageWorktree, err)
	}
	if made != tree {
		return stop(CreateStageWorktree, fmt.Errorf("%w: the worktree was made at %q, not at %q", errNotAsMade, made.path, tree.path))
	}
	if err := ctx.Err(); err != nil {
		return stop(CreateStageRecord, err)
	}
	if err := held.KeepCreated(space, deps.Now()); err != nil {
		return stop(CreateStageRecord, err)
	}
	out.Made = append(out.Made, Made{Kind: MadeRecord, ID: filepath.Join(held.dir, createdFile)})
	maker, err := NewWorkspaceMaker(deps.Cmds, deps.Sleep, space)
	if err != nil {
		return stop(CreateStageWorkspace, err)
	}
	ws, stage, err := maker.makeWorkspace(ctx, func(m Made) { out.Made = append(out.Made, m) })
	if err != nil {
		return stop(stage, err)
	}
	if err := ctx.Err(); err != nil {
		return stop(CreateStagePanes, err)
	}
	if err := checkMadePanes(ctx, NewHerdr(deps.Cmds), ws, tree); err != nil {
		return stop(CreateStagePanes, err)
	}
	out.Result, out.Workspace, out.Label = CreateMade, tree.path, space.Label()
	if ws.NotReady != nil {
		out.NotReady = ws.NotReady
		return out
	}
	res := sendAfterCreate(ctx, deps, held, tree, ws.ID, space.Issue())
	out.Mkreq = &res
	return out
}

// checkMadePanes reads Herdr's agents again and checks that the workspace
// made has exactly one Claude Code and one OpenCode agent, each in the pane
// it was started in and working in the worktree's root, and no other agent.
func checkMadePanes(ctx context.Context, herdr Herdr, ws madeWorkspace, tree Worktree) error {
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return err
	}
	var claude, openCode int
	for _, a := range agents {
		if a.Workspace != ws.ID {
			continue
		}
		switch {
		case a.Kind == paneinput.AgentClaudeCode && a.Pane == ws.Claude && inWorktree(a.Dir, tree.path):
			claude++
		case a.Kind == paneinput.AgentOpenCode && a.Pane == ws.OpenCode && inWorktree(a.Dir, tree.path):
			openCode++
		default:
			return fmt.Errorf("%w: the workspace has an agent of another kind, pane, or directory: %s in %q", errNotAsMade, a.Pane, a.Dir)
		}
	}
	if claude != 1 || openCode != 1 {
		return fmt.Errorf("%w: the workspace has %d Claude Code and %d OpenCode agents, not one of each", errNotAsMade, claude, openCode)
	}
	return nil
}

// errNotAsMade reports a worktree or workspace that, read again after it was
// made, is not the one that was asked for.
var errNotAsMade = errors.New("what was made is not what was asked for")

// errMakerRefused reports a WorkspaceMaker no constructor made, or Herdr
// output that would make it act on another workspace than its own.
var errMakerRefused = errors.New("the workspace maker refused")

// WorkspaceMaker makes one task's workspace in Herdr. Only NewWorkspaceMaker
// makes one, from a TaskSpace; it creates the workspace labelled with the
// space's label at the space's worktree, and every later call (tab rename,
// tab create, agent start) takes only an ID that its own workspace create
// returned, so it never writes to another workspace (chief's own among
// them). The zero value runs nothing: every method refuses before a call.
type WorkspaceMaker struct {
	cmds  Commands
	sleep func(context.Context, time.Duration) error
	space TaskSpace
	tree  Worktree
}

// NewWorkspaceMaker returns the maker of space's workspace; sleep waits
// between the reads of a new pane's shell.
func NewWorkspaceMaker(cmds Commands, sleep func(context.Context, time.Duration) error, space TaskSpace) (WorkspaceMaker, error) {
	tree, err := space.Worktree()
	if err != nil {
		return WorkspaceMaker{}, err
	}
	return WorkspaceMaker{cmds: cmds, sleep: sleep, space: space, tree: tree}, nil
}

// made reports a maker no constructor made, which runs nothing.
func (m WorkspaceMaker) made() error {
	if m.space.path == "" || m.tree.path == "" || m.sleep == nil {
		return fmt.Errorf("%w: it was not made by NewWorkspaceMaker", errMakerRefused)
	}
	return nil
}

const (
	// shellInterval and shellAttempts bound the wait for a new pane's shell
	// to reach its prompt (10 seconds): the shell runs its startup files,
	// with child processes in its foreground, for a moment after Herdr
	// returns the tab.
	shellInterval = 250 * time.Millisecond
	shellAttempts = 40
)

// madeWorkspace is the workspace a maker made: its ID and the panes of its
// agents, and NotReady when Claude Code was started but does not take input.
type madeWorkspace struct {
	ID       string
	OpenCode PaneID
	Claude   PaneID
	NotReady *NotReady
}

// makeWorkspace creates the workspace, its three tabs, and its two agents, calling
// made with each thing as it is made. Contract: it stops at the first stage
// that fails and returns that stage; a tab or pane Herdr reports in another
// workspace, an agent reported in another pane or of another name or kind,
// and a pane whose shell is not at its prompt are failures; an agent start
// that ends in Herdr's error response is a failure unless `herdr agent get`
// reads the agent in its pane, which for OpenCode passes and for Claude
// Code passes with NotReady set.
func (m WorkspaceMaker) makeWorkspace(ctx context.Context, made func(Made)) (madeWorkspace, CreateStage, error) {
	if err := m.made(); err != nil {
		return madeWorkspace{}, CreateStageWorkspace, err
	}
	out, err := m.cmds.herdr(ctx, herdrWorkspace, "create", "--cwd", m.tree.path, "--label", m.space.Label(), "--no-focus")
	if err != nil {
		return madeWorkspace{}, CreateStageWorkspace, err
	}
	created, err := parseWorkspaceCreate(out)
	if err != nil {
		// Herdr made a workspace whose ID cannot be read: it is named by
		// its label, so the human still sees that it exists.
		made(Made{Kind: MadeWorkspace, ID: m.space.Label()})
		return madeWorkspace{}, CreateStageWorkspace, err
	}
	ws := madeWorkspace{ID: created.First.Workspace}
	made(Made{Kind: MadeWorkspace, ID: ws.ID, Label: created.Label})
	made(Made{Kind: MadeTab, ID: created.First.Tab.String()})
	made(Made{Kind: MadePane, ID: created.First.Pane.String()})
	if created.Label != m.space.Label() || !inWorktree(created.First.Dir, m.tree.path) {
		return ws, CreateStageWorkspace, fmt.Errorf("%w: the workspace was labelled %q at %q", errMakerRefused, created.Label, created.First.Dir)
	}
	if err := ctx.Err(); err != nil {
		return ws, CreateStageTabs, err
	}
	tabs, err := m.makeTabs(ctx, created, made)
	if err != nil {
		return ws, CreateStageTabs, err
	}
	for _, step := range []struct {
		stage CreateStage
		label string
		kind  paneinput.AgentKind
		herdr string
	}{
		{CreateStageOpenCode, tabOpenCode, paneinput.AgentOpenCode, herdrOpenCode},
		{CreateStageClaude, tabClaude, paneinput.AgentClaudeCode, herdrClaude},
	} {
		pane := tabs[step.label].Pane
		notReady, err := m.startAgent(ctx, pane, step.herdr, step.kind, ws.ID, made)
		if err != nil {
			return ws, step.stage, err
		}
		switch step.kind {
		case paneinput.AgentOpenCode:
			ws.OpenCode = pane
		case paneinput.AgentClaudeCode:
			ws.Claude, ws.NotReady = pane, notReady
		}
	}
	return ws, CreateStageUnknown, nil
}

// makeTabs renames the first tab of the workspace created and creates the
// tabs of the two agents in it, returning those by label.
func (m WorkspaceMaker) makeTabs(ctx context.Context, created createdWorkspace, made func(Made)) (map[string]createdTab, error) {
	if err := m.made(); err != nil {
		return nil, err
	}
	out, err := m.cmds.herdr(ctx, herdrTab, "rename", created.First.Tab.String(), tabShell)
	if err != nil {
		return nil, err
	}
	renamed, err := parseTabRename(out)
	if err != nil {
		return nil, err
	}
	if renamed.Tab != created.First.Tab || renamed.Workspace != created.First.Workspace || renamed.Label != tabShell {
		return nil, fmt.Errorf("%w: the rename reported tab %s of %s labelled %q", errMakerRefused, renamed.Tab, renamed.Workspace, renamed.Label)
	}
	tabs := map[string]createdTab{}
	for _, label := range []string{tabOpenCode, tabClaude} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out, err := m.cmds.herdr(ctx, herdrTab, "create", "--workspace", created.First.Workspace, "--cwd", m.tree.path, "--label", label, "--no-focus")
		if err != nil {
			return nil, err
		}
		tab, err := parseTabCreate(out, created.First.Workspace)
		if err != nil {
			// Herdr made a tab whose ID cannot be read, or one it reports
			// in another workspace: it is named by its label.
			made(Made{Kind: MadeTab, ID: label})
			return nil, err
		}
		made(Made{Kind: MadeTab, ID: tab.Tab.String()})
		made(Made{Kind: MadePane, ID: tab.Pane.String()})
		if !inWorktree(tab.Dir, m.tree.path) {
			return nil, fmt.Errorf("%w: tab %s was opened at %q", errMakerRefused, tab.Tab, tab.Dir)
		}
		tabs[label] = tab
	}
	return tabs, nil
}

// startAgent starts the agent of kind (Herdr's name herdrKind) in pane of
// the workspace ws, once the pane's shell is at its prompt, which it waits
// for up to shellAttempts reads. It returns
// NotReady when Herdr answered with an error response but `herdr agent get`
// reads the agent in the pane.
func (m WorkspaceMaker) startAgent(ctx context.Context, pane PaneID, herdrKind string, kind paneinput.AgentKind, ws string, made func(Made)) (*NotReady, error) {
	if err := m.made(); err != nil {
		return nil, err
	}
	if err := m.waitShell(ctx, pane); err != nil {
		return nil, err
	}
	name := m.space.Label() + "-" + herdrKind
	// No argument but the name, the kind, the pane, and the wait is given:
	// the agent starts with no permission or trust granted for the human.
	out, err := m.cmds.herdrLimit(ctx, agentStartTimeout, herdrAgent, "start", name, "--kind", herdrKind, "--pane", pane.String(), "--timeout", agentStartWait)
	if err == nil {
		// Herdr started an agent, whatever its output says: it is listed by
		// the name asked for before the output is checked.
		made(Made{Kind: MadeAgent, ID: name})
		started, err := parseAgentStart(out)
		if err != nil {
			return nil, err
		}
		a := started.Agent
		if started.Name != name || a.Kind != kind || a.Pane != pane || a.Workspace != ws {
			return nil, fmt.Errorf("%w: herdr reported agent %q in pane %s of %s", errMakerRefused, started.Name, a.Pane, a.Workspace)
		}
		return nil, nil
	}
	exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
	if !ok {
		return nil, err
	}
	response, parseErr := parseHerdrError(exitErr.Stderr, "cli:agent:start")
	if parseErr != nil {
		return nil, fmt.Errorf("%w (%w)", err, parseErr)
	}
	// Herdr's code is only shown: whether the start failed is read from the
	// pane itself.
	a, getErr := NewHerdr(m.cmds).Agent(ctx, pane)
	if getErr != nil || a.Pane != pane || a.Workspace != ws || a.Kind != kind {
		return nil, fmt.Errorf("%w: herdr agent start answered %s and no agent of the kind is in pane %s", errMakerRefused, response.Code, pane)
	}
	made(Made{Kind: MadeAgent, ID: name})
	return &NotReady{Pane: pane, Status: a.Status, Code: response.Code, Message: response.Message}, nil
}

// waitShell reads pane's processes until its shell is alone in the
// foreground. Contract: an unreadable read or one about another pane fails
// at once; a shell still busy after shellAttempts reads fails.
func (m WorkspaceMaker) waitShell(ctx context.Context, pane PaneID) error {
	for attempt := range shellAttempts {
		if attempt > 0 {
			if err := m.sleep(ctx, shellInterval); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := NewHerdr(m.cmds).processes(ctx, pane)
		if err != nil {
			return err
		}
		if info.ShellInForeground() {
			return nil
		}
	}
	return fmt.Errorf("%w: the shell of pane %s is not at its prompt", errMakerRefused, pane)
}

// TargetsOutcome is what `chief targets` found: the issue as GitHub
// answered, and the workspaces /mkreq of it can go to now.
type TargetsOutcome struct {
	Issue paneinput.IssueNumber
	// Title is the issue's title, made visible and cut (Issue.Title), when
	// GitHub answered with an issue.
	Title    string
	TitleCut bool
	// Refusal is CreateRefusalIssueForm for a number of another form, the
	// refusal of an answer that is not an open issue without the draft label
	// (the same as a creation's), and otherwise none.
	Refusal    CreateRefusal
	Failure    string
	Workspaces []TargetWorkspace
}

// TargetWorkspace is a task workspace whose row offers /mkreq.
type TargetWorkspace struct {
	Workspace string
	Label     string
}

// Targets checks the form of the issue number, reads the issue from GitHub,
// and lists the task workspaces whose row offers /mkreq now (the design,
// 3.3.5). Contract: it reads only: it makes nothing and writes no record
// (no notice either); a number of another form reads nothing; an issue
// that is not open, or has the draft label, is refused as a creation would
// refuse it, with no workspace listed and no round read. `chief create`
// reads the issue again before it makes anything.
func Targets(ctx context.Context, deps Deps, issueArg string) (TargetsOutcome, error) {
	issue, err := paneinput.ParseIssueNumber(issueArg)
	if err != nil {
		return TargetsOutcome{Refusal: CreateRefusalIssueForm, Failure: err.Error()}, nil
	}
	_, found, refusal, failure, err := readOpenIssue(ctx, deps, NewHerdr(deps.Cmds), NewGitReader(deps.Cmds), issue)
	if err != nil {
		return TargetsOutcome{}, err
	}
	out := TargetsOutcome{Issue: issue, Title: found.Title, TitleCut: found.TitleCut}
	if refusal != CreateRefusalUnknown {
		out.Refusal, out.Failure = refusal, failure
		return out, nil
	}
	view, err := readFacts(ctx, deps)
	if err != nil {
		return TargetsOutcome{}, err
	}
	// No history is read: a worktree on main offers no /mkreq whatever its
	// task, so its identification cannot change the list.
	buildRows(ctx, deps, &view, traceHistory{})
	for _, row := range view.rows {
		// The same check as the send of /mkreq, so every workspace listed
		// is one `chief send --command mkreq` accepts.
		if _, why := offeredMkreq(row, issue); why == "" {
			out.Workspaces = append(out.Workspaces, TargetWorkspace{Workspace: row.Workspace, Label: row.Label})
		}
	}
	return out, nil
}
