package chief

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// ErrBadRequest reports a send request whose arguments do not fit together:
// an unknown command kind, an issue number for another command than /mkreq,
// a PR number for another than /runpr, or a missing one. It is a usage
// error; nothing is read.
var ErrBadRequest = errors.New("the send request is malformed")

// RequestKind is the command a send request names. The zero value is no
// command.
type RequestKind int

const (
	// RequestUnknown is no command.
	RequestUnknown RequestKind = iota
	// RequestMkreq is /mkreq with the human's issue number.
	RequestMkreq
	// RequestMkarch is /mkarch of the workspace's task.
	RequestMkarch
	// RequestMkplan is /mkplan of the workspace's task.
	RequestMkplan
	// RequestRunplan is /runplan of the workspace's task by its plan's route.
	RequestRunplan
	// RequestRunplanTakeover is /runplan handed to Claude Code.
	RequestRunplanTakeover
	// RequestRunpr is /runpr of the workspace's open PR.
	RequestRunpr
)

// requestKinds are the --command values.
var requestKinds = map[string]RequestKind{
	"mkreq":            RequestMkreq,
	"mkarch":           RequestMkarch,
	"mkplan":           RequestMkplan,
	"runplan":          RequestRunplan,
	"runplan-takeover": RequestRunplanTakeover,
	"runpr":            RequestRunpr,
}

// SendRequest is a validated `chief send`: the workspace, the command kind,
// and the one number the kind takes. Only ParseSendRequest makes one.
type SendRequest struct {
	workspace string
	kind      RequestKind
	issue     paneinput.IssueNumber
	pr        fixprguard.PRNumber
}

// ParseSendRequest validates the arguments of `chief send`; an empty issue
// or pr means the flag was not given. Contract: --issue is taken by mkreq
// alone and required by it, --pr by runpr alone and required by it, and each
// number passes its grammar; anything else is ErrBadRequest. The task is
// never an argument: the tool reads it from the workspace.
func ParseSendRequest(workspace, command, issue, pr string) (SendRequest, error) {
	kind, ok := requestKinds[command]
	if !ok {
		return SendRequest{}, fmt.Errorf("%w: unknown command %q", ErrBadRequest, command)
	}
	if !filepath.IsAbs(workspace) {
		return SendRequest{}, fmt.Errorf("%w: --workspace is not an absolute path", ErrBadRequest)
	}
	req := SendRequest{workspace: workspace, kind: kind}
	if (kind == RequestMkreq) != (issue != "") {
		return SendRequest{}, fmt.Errorf("%w: --issue goes with mkreq and only with it", ErrBadRequest)
	}
	if (kind == RequestRunpr) != (pr != "") {
		return SendRequest{}, fmt.Errorf("%w: --pr goes with runpr and only with it", ErrBadRequest)
	}
	var err error
	if issue != "" {
		if req.issue, err = paneinput.ParseIssueNumber(issue); err != nil {
			return SendRequest{}, fmt.Errorf("%w: %w", ErrBadRequest, err)
		}
	}
	if pr != "" {
		if req.pr, err = fixprguard.ParsePRNumber(pr); err != nil {
			return SendRequest{}, fmt.Errorf("%w: %w", ErrBadRequest, err)
		}
	}
	return req, nil
}

// located is chief's view of Herdr for one requested workspace.
type located struct {
	chiefPane PaneID
	chief     Agent
	chiefTree Worktree
	ws        TaskWorkspace
	found     bool
}

// locate finds the task workspace whose worktree is path. Contract: path is
// compared with the canonical worktrees of the current task workspaces after
// its own links are resolved, and git never runs in it; a path that is no
// current task workspace's worktree is not found.
func locate(ctx context.Context, deps Deps, path string) (located, error) {
	herdr := NewHerdr(deps.Cmds)
	chiefPane, err := ParsePaneID(deps.Env.PaneID)
	if err != nil {
		return located{}, err
	}
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return located{}, err
	}
	workspaces, err := herdr.Workspaces(ctx)
	if err != nil {
		return located{}, err
	}
	chief, err := chiefWorkspaceOf(agents, chiefPane)
	if err != nil {
		return located{}, err
	}
	chiefTree, err := CanonicalWorktree(ctx, deps.Cmds, chief.Dir)
	if err != nil {
		return located{}, fmt.Errorf("read the chief worktree: %w", err)
	}
	found, err := FindTaskWorkspaces(ctx, NewGitReader(deps.Cmds), deps.Cmds, agents, workspaces, chiefPane)
	if err != nil {
		return located{}, err
	}
	loc := located{chiefPane: chiefPane, chief: chief, chiefTree: chiefTree}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return loc, nil
	}
	for _, w := range found {
		if w.Worktree == resolved {
			loc.ws, loc.found = w, true
		}
	}
	return loc, nil
}

// SendOutcome is what `chief send` did: one result for the command and,
// after /runpr, one for the reset.
type SendOutcome struct {
	Actions []ActionResult
}

// Send sends the command the human chose for a workspace (the design, 6.3).
// Contract: the command is sent only when the tool itself offers it for the
// workspace now (the next command of its row, the takeover the row proposes,
// /mkreq to a workspace without a task, or /runpr of the branch's open PR of
// that number); the route comes from origin/main's routing table; the send
// holds the workspace's exclusion, settles unfinished sends first, and sends
// nothing to a stopped workspace; after /runpr the detail is kept before the
// send and the maker's pane is reset only as resetAfterRunpr allows.
func Send(ctx context.Context, deps Deps, req SendRequest) (SendOutcome, error) {
	if req.kind == RequestUnknown {
		return SendOutcome{}, fmt.Errorf("%w: a request no parser made", ErrBadRequest)
	}
	loc, err := locate(ctx, deps, req.workspace)
	if err != nil {
		return SendOutcome{}, err
	}
	one := func(r ActionResult) SendOutcome { return SendOutcome{Actions: []ActionResult{r}} }
	base := ActionResult{Workspace: req.workspace, Input: InputCommand}
	if !loc.found {
		return one(notSent(base, NotSentNoWorkspace, "the path is no current task workspace's worktree")), nil
	}
	base.Workspace = loc.ws.Worktree
	routing, routingErr := ReadRouting(ctx, deps.Cmds, loc.chiefTree.String())
	if routingErr != nil {
		return one(notSent(base, NotSentUnreadable, "the routing table cannot be read: "+routingErr.Error())), nil
	}
	tasks := NewTasks(deps.Cmds)
	git := NewGitReader(deps.Cmds)
	herdr := NewHerdr(deps.Cmds)
	old, oldErr := readOldValues(ctx, tasks, loc.chiefTree.String())
	f := readWorkspace(ctx, herdr, git, tasks, loc.ws)
	f.Routing = routing
	row := classifyWorkspace(ctx, git, tasks, f, old, oldErr)
	cmd, why := offered(row, req)
	if why != "" {
		return one(notSent(base, NotSentNotOffered, why)), nil
	}
	base.Command, _ = cmd.Text()
	route, err := routing.Route(cmd.Route())
	if err != nil {
		return one(notSent(base, NotSentUnreadable, "the routing table has no route for the command")), nil
	}
	base.Agent = route.Agent()
	base.Model, _ = route.ModelText()
	records, err := deps.Store.Records(loc.ws.Tree)
	if err != nil {
		return one(notSent(base, NotSentUnreadable, "the workspace's records cannot be opened: "+err.Error())), nil
	}
	held, err := records.Lock()
	if errors.Is(err, ErrWorkspaceBusy) {
		return one(notSent(base, NotSentBusy, "another send to the workspace is under way")), nil
	}
	if err != nil {
		return one(notSent(base, NotSentUnreadable, "the workspace's exclusion cannot be taken: "+err.Error())), nil
	}
	defer func() { _ = held.Close() }()
	if _, err := held.SettleUnfinished(deps.Now()); err != nil {
		return one(notSent(base, NotSentUnreadable, "an unfinished send cannot be settled: "+err.Error())), nil
	}
	stop, err := held.Stopped()
	switch {
	case err != nil:
		return one(notSent(base, NotSentStopped, "the stop record cannot be read, so the workspace counts as stopped: "+err.Error())), nil
	case stop != nil:
		return one(notSent(base, NotSentStopped, "the workspace is stopped until release: "+stop.Reason)), nil
	}
	writer, err := NewPaneWriter(deps.Cmds, loc.chiefPane, loc.chief.Workspace)
	if err != nil {
		return SendOutcome{}, err
	}
	sc := sendContext{deps: deps, herdr: herdr, writer: writer, held: held, ws: loc.ws}
	var rec *detailRecord
	docsDir := ""
	if f.Docs != nil {
		docsDir = f.Docs.Dir
	}
	if req.kind == RequestRunpr {
		rec = ensureDetail(ctx, deps, git, loc.ws, *row.PR, docsDir, routing)
	}
	res := sc.sendCommand(ctx, cmd, route)
	out := SendOutcome{Actions: []ActionResult{res}}
	if req.kind == RequestRunpr && res.Outcome == OutcomeSent {
		key, why := prMakerRoute(ctx, tasks, loc.ws.Worktree, f.Docs, old, oldErr, row.PR.URL)
		out.Actions = append(out.Actions, sc.resetAfterRunpr(ctx, res.Pane, *row.PR, rec, key, why, routing))
	}
	return out, nil
}

// offered returns the command of req when the workspace's row offers it, or
// why it does not.
func offered(row Row, req SendRequest) (paneinput.Command, string) {
	next := func(kind TaskNextKind) (paneinput.Command, string) {
		if row.State != StateProposeNext || row.Next == nil || row.Next.Kind != kind || row.Next.Command == nil {
			return paneinput.Command{}, fmt.Sprintf("the workspace's next command is not this one (state %s)", row.State)
		}
		return *row.Next.Command, ""
	}
	switch req.kind {
	case RequestMkreq:
		return offeredMkreq(row, req.issue)
	case RequestMkarch:
		return next(TaskNextMkarch)
	case RequestMkplan:
		return next(TaskNextMkplan)
	case RequestRunplan:
		return next(TaskNextRunplan)
	case RequestRunplanTakeover:
		if row.State != StateProposeNext || row.Takeover == nil || row.Takeover.Command == nil {
			return paneinput.Command{}, "the workspace has no takeover proposal"
		}
		return *row.Takeover.Command, ""
	case RequestRunpr:
		return offeredRunpr(row, req.pr)
	default:
		return paneinput.Command{}, "no command"
	}
}

// offeredMkreq returns /mkreq of issue when the row is a workspace without a
// task that is offered one.
func offeredMkreq(row Row, issue paneinput.IssueNumber) (paneinput.Command, string) {
	if row.State != StateProposeNext || row.Next == nil || row.Next.Kind != TaskNextMkreq {
		return paneinput.Command{}, "the workspace is not one without a task that waits for input"
	}
	cmd, err := paneinput.NewMkreq(issue)
	if err != nil {
		return paneinput.Command{}, err.Error()
	}
	return cmd, ""
}

// offeredRunpr returns /runpr of pr when pr is the row's open PR awaiting
// review.
func offeredRunpr(row Row, pr fixprguard.PRNumber) (paneinput.Command, string) {
	if row.State != StateAwaitingReview || row.PR == nil || strconv.Itoa(row.PR.Number) != pr.String() {
		return paneinput.Command{}, "the PR is not the open PR of the workspace's branch awaiting review"
	}
	cmd, err := paneinput.NewRunpr(pr)
	if err != nil {
		return paneinput.Command{}, err.Error()
	}
	return cmd, ""
}

// ensureDetail returns the detail kept for the PR at the worktree's HEAD,
// gathering and keeping it first when none is kept. A detail that cannot be
// kept gives nil, which keeps every pane from being reset.
func ensureDetail(ctx context.Context, deps Deps, git GitReader, ws TaskWorkspace, pr OpenPR, docsDir string, routing Routing) *detailRecord {
	if head, err := git.Head(ctx, ws.Worktree); err == nil {
		if records, err := deps.Store.Records(ws.Tree); err == nil {
			if rec, ok, err := records.readDetail(pr.Number, head); err == nil && ok {
				return &rec
			}
		}
	}
	return CollectDetail(ctx, deps, ws, pr, docsDir, routing, nil).record
}

// DetailOutcome is what `chief detail` found: the PR's detail, or why there
// is none.
type DetailOutcome struct {
	Detail  *PRDetail
	Problem string
}

// Detail gathers the detail of the workspace's open PR.
func Detail(ctx context.Context, deps Deps, path string) (DetailOutcome, error) {
	loc, err := locate(ctx, deps, path)
	if err != nil {
		return DetailOutcome{}, err
	}
	if !loc.found {
		return DetailOutcome{Problem: "the path is no current task workspace's worktree"}, nil
	}
	git := NewGitReader(deps.Cmds)
	branch, err := git.Branch(ctx, loc.ws.Worktree)
	if err != nil || branch == "" {
		return DetailOutcome{Problem: "the worktree has no branch to read a PR of"}, nil
	}
	prs, err := git.OpenPRs(ctx, loc.ws.Worktree, branch)
	switch {
	case err != nil:
		return DetailOutcome{Problem: "the open pull requests cannot be read: " + err.Error()}, nil
	case len(prs) != 1:
		return DetailOutcome{Problem: fmt.Sprintf("the branch has %d open pull requests, not one", len(prs))}, nil
	}
	docsDir := ""
	if docs, err := NewTasks(deps.Cmds).Read(ctx, loc.ws.Worktree, branch); err == nil {
		docsDir = docs.Dir
	}
	routing, routingErr := ReadRouting(ctx, deps.Cmds, loc.chiefTree.String())
	d := CollectDetail(ctx, deps, loc.ws, prs[0], docsDir, routing, routingErr)
	return DetailOutcome{Detail: &d}, nil
}

// ReleaseOutcome is what `chief release` did.
type ReleaseOutcome struct {
	// Released is true when a stop was removed.
	Released bool
	Problem  string
}

// Release lifts the stop of the workspace at path, at the human's choice.
func Release(ctx context.Context, deps Deps, path string) (ReleaseOutcome, error) {
	loc, err := locate(ctx, deps, path)
	if err != nil {
		return ReleaseOutcome{}, err
	}
	if !loc.found {
		return ReleaseOutcome{Problem: "the path is no current task workspace's worktree"}, nil
	}
	records, err := deps.Store.Records(loc.ws.Tree)
	if err != nil {
		return ReleaseOutcome{}, err
	}
	held, err := records.Lock()
	if errors.Is(err, ErrWorkspaceBusy) {
		return ReleaseOutcome{Problem: "another send to the workspace is under way"}, nil
	}
	if err != nil {
		return ReleaseOutcome{}, err
	}
	defer func() { _ = held.Close() }()
	released, err := held.Release(deps.Now())
	if err != nil {
		return ReleaseOutcome{}, err
	}
	return ReleaseOutcome{Released: released}, nil
}
