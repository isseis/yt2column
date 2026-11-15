package chief

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// CheckValue is the opaque check value of a relayable question. The session
// passes back the value a snapshot gave and never builds or parses one.
type CheckValue string

// checkVersion is the first part of every check value, so a value made by
// another rule never equals one made by this rule.
const checkVersion = "chief-check-1"

// checkValueForm is the shape of every check value: the hex of a SHA-256.
var checkValueForm = regexp.MustCompile(`^[0-9a-f]{64}$`)

// checkValue returns the check value of question q in agent a's pane.
// Contract: it changes when any of these changes: the agent, the pane, its
// agent session, its state_change_seq, the question text as read, any
// option's number, label, description, or relayability as read, or the
// selection's position; each part is written with its length, so two
// different part lists never give one value.
func checkValue(a Agent, q Question) CheckValue {
	parts := []string{
		checkVersion,
		strconv.Itoa(int(a.Kind)),
		a.Pane.String(),
		a.Session,
		strconv.FormatInt(a.StateChangeSeq, 10),
		q.Text,
		strconv.Itoa(q.Cursor),
	}
	for _, o := range q.Options {
		parts = append(parts, strconv.Itoa(o.Number), o.Label, o.Description, strconv.FormatBool(o.Relayable))
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(strconv.Itoa(len(p)))
		b.WriteByte(':')
		b.WriteString(p)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return CheckValue(hex.EncodeToString(sum[:]))
}

// relayable reports whether q in a's pane is a question chief relays: a
// single-select question the reader allowed, with no tabs and an option to
// relay, in the pane of an agent whose keys were checked.
func relayable(a Agent, q Question) bool {
	switch a.Kind {
	case paneinput.AgentClaudeCode, paneinput.AgentOpenCode:
	default:
		return false
	}
	return q.Kind == QuestionSingle && q.Relay == RelayAllowed && len(q.Tabs) == 0 &&
		slices.ContainsFunc(q.Options, func(o Option) bool { return o.Relayable })
}

// relayCheck returns the check value a snapshot shows for q in a's pane, or
// "" for a question chief does not relay.
func relayCheck(a Agent, q Question) CheckValue {
	if !relayable(a, q) {
		return ""
	}
	return checkValue(a, q)
}

// RelayRequest is a validated `chief relay`: the workspace, the check value
// a snapshot gave, and the option the human chose. Only ParseRelayRequest
// makes one.
type RelayRequest struct {
	workspace string
	check     CheckValue
	option    int
}

// ParseRelayRequest validates the arguments of `chief relay`. Contract: the
// workspace is an absolute path, the check value has the shape every check
// value has, and the answer is "1=<n>": a relay answers the one question of a
// screen, with an option number of one digit from 1. Anything else is
// ErrBadRequest; the pane is never an argument.
func ParseRelayRequest(workspace, check, answer string) (RelayRequest, error) {
	if !filepath.IsAbs(workspace) {
		return RelayRequest{}, fmt.Errorf("%w: --workspace is not an absolute path", ErrBadRequest)
	}
	if !checkValueForm.MatchString(check) {
		return RelayRequest{}, fmt.Errorf("%w: --check is not a check value", ErrBadRequest)
	}
	question, number, ok := strings.Cut(answer, "=")
	if !ok || question != "1" {
		return RelayRequest{}, fmt.Errorf("%w: --answer is not 1=<option number>", ErrBadRequest)
	}
	if len(number) != 1 || number[0] < '1' || number[0] > '0'+paneinput.MaxOptionNumber {
		return RelayRequest{}, fmt.Errorf("%w: --answer's option number %q is not one digit from 1", ErrBadRequest, number)
	}
	return RelayRequest{workspace: workspace, check: CheckValue(check), option: int(number[0] - '0')}, nil
}

// relayTarget is the pane whose question a relay answers, as read under the
// workspace's exclusion.
type relayTarget struct {
	agent    Agent
	question Question
}

// findRelayTarget reads every agent pane of the workspace again and returns
// the one whose question gives check now, or whether a read failed when none
// does. Contract: each pane's screen is
// read before its agent, and the value is made from that agent reading, so a
// question answered and asked again with the same text before its screen was
// read carries the new state_change_seq and does not match the value of the
// first; a pane whose directory is not the worktree's root is never read.
func (s sendContext) findRelayTarget(ctx context.Context, check CheckValue) (target relayTarget, found, readFailed bool) {
	for _, listed := range s.ws.Agents {
		if listed.Kind == paneinput.AgentUnknown || !inWorktree(listed.Dir, s.ws.Worktree) {
			continue
		}
		raw, err := s.herdr.Screen(ctx, listed.Pane, ScreenVisible)
		if err != nil {
			readFailed = true
			continue
		}
		after, err := s.herdr.Agent(ctx, listed.Pane)
		if err != nil {
			readFailed = true
			continue
		}
		if !sameAgent(after, listed) {
			continue
		}
		screen := readScreen(after.Kind, raw)
		if screen.Kind != ScreenQuestion || screen.Question == nil {
			continue
		}
		if checkValue(after, *screen.Question) == check {
			return relayTarget{agent: after, question: *screen.Question}, true, false
		}
	}
	return relayTarget{}, false, readFailed
}

// Relay sends the option the human chose to the question a snapshot showed
// (the design, 3.6). Contract: it holds the workspace's exclusion, settles
// unfinished sends, and sends nothing to a stopped workspace; it reads the
// panes again and sends only to the one pane whose question gives the check
// value now; it sends only to a relayable question, only while Herdr reports
// the agent blocked, and only an option the question has and
// relays; what it sends is the key of the option's number, never a text; the
// start record is written before the key, and a failure once the key may
// have been pressed is never retried but stops the workspace.
func Relay(ctx context.Context, deps Deps, req RelayRequest) (SendOutcome, error) {
	if req.check == "" {
		return SendOutcome{}, fmt.Errorf("%w: a request no parser made", ErrBadRequest)
	}
	loc, err := locate(ctx, deps, req.workspace)
	if err != nil {
		return SendOutcome{}, err
	}
	one := func(r ActionResult) SendOutcome { return SendOutcome{Actions: []ActionResult{r}} }
	base := ActionResult{Workspace: req.workspace, Input: InputRelay}
	if !loc.found {
		return one(notSent(base, NotSentNoWorkspace, "the path is no current task workspace's worktree, or more than one Herdr workspace holds it")), nil
	}
	base.Workspace = loc.ws.Worktree
	held, refused := holdWorkspace(ctx, deps, &loc, base)
	if refused != nil {
		return one(*refused), nil
	}
	defer func() { _ = held.Close() }()
	writer, err := NewPaneWriter(deps.Cmds, loc.chiefPane, loc.chief.Workspace)
	if err != nil {
		return SendOutcome{}, err
	}
	sc := sendContext{deps: deps, herdr: NewHerdr(deps.Cmds), writer: writer, held: held, ws: loc.ws}
	return one(sc.relay(ctx, base, req)), nil
}

// relay checks the target and presses its key, for Relay.
func (s sendContext) relay(ctx context.Context, res ActionResult, req RelayRequest) ActionResult {
	target, ok, readFailed := s.findRelayTarget(ctx, req.check)
	switch {
	case !ok && readFailed:
		return notSent(res, NotSentUnreadable, "no pane shows the question as it was shown, and a pane could not be read")
	case !ok:
		return notSent(res, NotSentCheckMismatch, "no pane shows the question as it was shown: it changed, was answered, or the agent moved")
	}
	a, q := target.agent, target.question
	res.Pane, res.Agent = a.Pane.String(), a.Kind
	if !relayable(a, q) {
		return notSent(res, NotSentNotRelayable, "the question is not a single-select question chief relays; answer it in the pane")
	}
	// Herdr reported both agents blocked on their questions when sampled,
	// so an agent in any other state is not sent the key: a digit typed into
	// an input line would be text the human did not choose.
	if a.Status != AgentBlocked {
		return notSent(res, NotSentNotAccepting, "the agent is not blocked on the question")
	}
	i := slices.IndexFunc(q.Options, func(o Option) bool { return o.Number == req.option })
	if i < 0 {
		return notSent(res, NotSentBadOption, fmt.Sprintf("the question has no option %d", req.option))
	}
	option := q.Options[i]
	res.Command = fmt.Sprintf("%d. %s", option.Number, option.Label)
	if !option.Relayable {
		return notSent(res, NotSentBadOption, fmt.Sprintf("option %d is answered in the pane, not relayed", option.Number))
	}
	key, err := paneinput.SelectOption(a.Kind, option.Number)
	if err != nil {
		return notSent(res, NotSentUnreadable, "the option's key cannot be made: "+err.Error())
	}
	keyName, _ := key.Key()
	id, err := s.held.BeginSend(SendRecord{Pane: res.Pane, Input: res.Input.String(), Command: res.Command, Agent: agentLabel(a.Kind), Prompts: []string{"key " + keyName}}, s.deps.Now())
	if err != nil {
		return notSent(res, NotSentUnreadable, "the send record cannot be written: "+err.Error())
	}
	if err := s.writer.SendKeys(ctx, a, key); err != nil {
		if errors.Is(err, errWriteRefused) {
			// The writer refused before running herdr: nothing reached the
			// pane.
			res = notSent(res, NotSentUnreadable, err.Error())
			if eerr := s.held.EndSend(id, res.Outcome.String(), err.Error(), s.deps.Now()); eerr != nil {
				res.Detail += "; the end record cannot be written, so the workspace will stop: " + eerr.Error()
			}
			return res
		}
		// Herdr may have pressed the key before it failed, so a failed write
		// counts as sent.
		return s.fail(res, id, StepKey, &checkFailure{reason: NotSentUnreadable, detail: "herdr agent send-keys failed: " + err.Error(), agent: a})
	}
	if _, failed := s.poll(ctx, a, "the agent did not leave the question after the key", func(now Agent, _ string) bool {
		return now.Status != a.Status || now.StateChangeSeq != a.StateChangeSeq
	}); failed != nil {
		return s.fail(res, id, StepKey, failed)
	}
	res.Outcome = OutcomeSent
	if err := s.held.EndSend(id, res.Outcome.String(), "", s.deps.Now()); err != nil {
		res.Detail = "sent, but the end record cannot be written, so the workspace is stopped: " + err.Error()
		_ = s.held.Stop(res.Detail, "", id, s.deps.Now())
	}
	return res
}
