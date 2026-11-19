package chief

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// resetCheckVersion is the first part of a reset item's check value, so it
// never equals a value made by another rule, a question's among them.
const resetCheckVersion = "chief-reset-check-1"

// ResetItem is a skipped reset offered to the human: the pane, its agent,
// the scene it was skipped in, the command that ran in the pane, and the
// value `chief reset --check` takes for it.
type ResetItem struct {
	Pane    PaneID
	Agent   paneinput.AgentKind
	Scene   skipScene
	Command ranCommand
	Check   CheckValue
	// record is the skip record the item was made from.
	record skipRecord
}

// id is the item's identity in the pending list: the workspace and the
// pane, session, and state_change_seq of the skip.
func (it ResetItem) id(workspace string) ItemID {
	return itemID("reset", workspace, it.record.pane.String(), it.record.session, strconv.FormatInt(it.record.seq, 10))
}

// resetCheck returns the check value of the reset item of rec in the
// worktree. Contract: it changes when the worktree, the Herdr workspace, the
// pane, its session, its state_change_seq, the agent, the scene, or the
// command shown changes; the value is opaque and the session never builds
// or reads one.
func resetCheck(worktree string, rec skipRecord) CheckValue {
	return checkValueOf([]string{
		resetCheckVersion,
		worktree,
		rec.workspace,
		rec.pane.String(),
		rec.session,
		strconv.FormatInt(rec.seq, 10),
		agentLabel(rec.agent),
		rec.scene.String(),
		rec.command.text(),
	})
}

// resetItemOf returns the reset item of rec in ws, with the pane's agent as
// ws lists it, or false when the record gives no item now. Contract: an item is
// made only when rec names the Herdr workspace of ws, its pane is the one
// pane of its agent in ws, the pane's session and state_change_seq are the
// record's, its agent waits for input, and the routing table routes the
// command to the record's agent. The caller checks the workspace's own
// state (not needing a check, not stopped). The round and `chief reset`
// both make their items here, so an item shown is an item `chief reset`
// takes while nothing changed.
func resetItemOf(ws TaskWorkspace, rec skipRecord, routing Routing) (ResetItem, Agent, bool) {
	if rec.workspace != ws.ID {
		return ResetItem{}, Agent{}, false
	}
	pane, _, ok := targetPane(ws, rec.agent)
	switch {
	case !ok || pane.Pane != rec.pane:
		return ResetItem{}, Agent{}, false
	case pane.Session != rec.session || pane.StateChangeSeq != rec.seq:
		return ResetItem{}, Agent{}, false
	case pane.Status != AgentIdle && pane.Status != AgentDone:
		return ResetItem{}, Agent{}, false
	}
	route, err := routing.Route(rec.command.route)
	if err != nil || route.Agent() != rec.agent {
		return ResetItem{}, Agent{}, false
	}
	return ResetItem{Pane: rec.pane, Agent: rec.agent, Scene: rec.scene, Command: rec.command, Check: resetCheck(ws.Worktree, rec), record: rec}, pane, true
}

// OfferedResetRequest is a validated `chief reset`: the workspace and the
// check value of the reset item the human chose. Only
// ParseOfferedResetRequest makes one.
type OfferedResetRequest struct {
	workspace string
	check     CheckValue
}

// ParseOfferedResetRequest validates the arguments of `chief reset`.
// Contract: the workspace is an absolute path and the check value has the
// shape every check value has; anything else is ErrBadRequest.
func ParseOfferedResetRequest(workspace, check string) (OfferedResetRequest, error) {
	if !filepath.IsAbs(workspace) {
		return OfferedResetRequest{}, fmt.Errorf("%w: --workspace is not an absolute path", ErrBadRequest)
	}
	if !checkValueForm.MatchString(check) {
		return OfferedResetRequest{}, fmt.Errorf("%w: --check is not a check value", ErrBadRequest)
	}
	return OfferedResetRequest{workspace: workspace, check: CheckValue(check)}, nil
}

// SendOfferedReset sends the reset of the item the human chose (the
// design, 3.4). Contract: it holds the workspace's exclusion, settles
// unfinished sends, and sends nothing to a stopped workspace; it reads the
// workspace, its skip records, and the routing table again under the
// exclusion and sends only to the item whose check value is req's now; a
// Claude Code pane's last human input must be the item's command (its name
// and argument after /runpr, its name after an automatic send); the pane is
// checked like any send before the input; the item is used up, and chief's
// owner checked again, after every other check and before the send record,
// so an item is sent at most once and an item not sent stays; what is typed
// is /clear or /new only.
func SendOfferedReset(ctx context.Context, deps Deps, owner Owner, req OfferedResetRequest) (SendOutcome, error) {
	if req.check == "" {
		return SendOutcome{}, fmt.Errorf("%w: a request no parser made", ErrBadRequest)
	}
	loc, err := locate(ctx, deps, req.workspace)
	if err != nil {
		return SendOutcome{}, err
	}
	one := func(r ActionResult) SendOutcome { return SendOutcome{Actions: []ActionResult{r}} }
	base := ActionResult{Workspace: req.workspace, Input: InputReset}
	if !loc.found {
		return one(notSent(base, NotSentNoWorkspace, "the path is no current task workspace's worktree, or more than one Herdr workspace holds it")), nil
	}
	base.Workspace = loc.ws.Worktree
	routing, routingErr := ReadRouting(ctx, deps.Cmds, loc.chiefTree.String())
	if routingErr != nil {
		return one(notSent(base, NotSentUnreadable, "the routing table cannot be read: "+routingErr.Error())), nil
	}
	held, refused := holdWorkspace(ctx, deps, &loc, base)
	if refused != nil {
		return one(*refused), nil
	}
	defer func() { _ = held.Close() }()
	tasks := NewTasks(deps.Cmds)
	git := NewGitReader(deps.Cmds)
	herdr := NewHerdr(deps.Cmds)
	old, oldErr := readOldValues(ctx, tasks, loc.chiefTree.String())
	f := readWorkspace(ctx, herdr, git, tasks, loc.ws)
	f.Routing = routing
	// A worktree on main is classified from the caller's history, as the
	// round that offered the item classified it.
	f = applyMainTask(ctx, tasks, f, identifyMainTask(deps.Store.readTraces(owner.Session), loc.ws.Tree))
	row := classifyWorkspace(ctx, git, tasks, f, old, oldErr)
	if row.State == StateNeedsCheck {
		return one(notSent(base, NotSentCheckMismatch, "the workspace needs a check, so it offers no reset")), nil
	}
	found, err := held.skipRecords()
	if err != nil {
		return one(notSent(base, NotSentUnreadable, "the skip records cannot be listed: "+err.Error())), nil
	}
	var item ResetItem
	var target Agent
	chosen := false
	for _, fr := range found {
		if fr.err != nil {
			continue
		}
		it, pane, ok := resetItemOf(loc.ws, fr.rec, routing)
		if ok && it.Check == req.check {
			item, target, chosen = it, pane, true
			break
		}
	}
	if !chosen {
		return one(notSent(base, NotSentCheckMismatch, "no reset item is offered as it was shown: the pane changed, was sent to, or the item was used")), nil
	}
	return one(sendReset(ctx, deps, owner, &loc, held, herdr, item, target, routing, base)), nil
}

// sendReset checks the chosen item's pane and sends its reset, for
// SendOfferedReset.
func sendReset(ctx context.Context, deps Deps, owner Owner, loc *located, held *HeldWorkspace, herdr Herdr, item ResetItem, target Agent, routing Routing, res ActionResult) ActionResult {
	res.Pane, res.Agent = item.Pane.String(), item.Agent
	st, ok := resetPrompt(item.Agent)
	if !ok {
		return notSent(res, NotSentUnreadable, "the item has no agent")
	}
	res.Command, _ = st.prompt.Text()
	route, err := routing.Route(item.Command.route)
	if err != nil {
		return notSent(res, NotSentUnreadable, "the routing table has no route for the item's command")
	}
	writer, err := NewPaneWriter(deps.Cmds, loc.chiefPane, loc.chief.Workspace)
	if err != nil {
		return notSent(res, NotSentUnreadable, err.Error())
	}
	sc := sendContext{deps: deps, herdr: herdr, writer: writer, held: held, ws: loc.ws}
	if item.Agent == paneinput.AgentClaudeCode {
		var reason NotSentReason
		var detail string
		switch item.Scene {
		case sceneAfterRunpr:
			reason, detail = sc.paneLastInputMadePR(target, item.Command.maker())
		case sceneAfterAutoSend:
			reason, detail = sc.lastInputRanRunpr(target)
		default:
			reason, detail = NotSentUnreadable, "the item has no scene"
		}
		if reason != NotSentUnknown {
			return notSent(res, reason, detail)
		}
	}
	sc.beforeBegin = func() (NotSentReason, error) {
		// The owner may have changed while the workspace was read.
		if err := checkOwner(ctx, deps, owner); err != nil {
			return NotSentConditions, fmt.Errorf("chief's lock or session changed: %w", err)
		}
		if err := held.useSkip(item.record); err != nil {
			return NotSentUnreadable, fmt.Errorf("the reset item cannot be used up, so nothing is sent: %w", err)
		}
		return NotSentUnknown, nil
	}
	seq := target.StateChangeSeq
	return sc.run(ctx, res, target, []step{st}, paneCheck{session: target.Session, seq: &seq}, route, nil)
}
