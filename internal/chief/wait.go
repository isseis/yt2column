package chief

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// WaitBaseline is the opaque baseline `chief wait` compares against: what
// the agents of the task workspaces were when a snapshot or a tick read them.
// The session passes it back and never builds or parses it.
type WaitBaseline string

// The waits of `chief wait`: how often it reads Herdr, how long it waits by
// default, and the longest wait it takes. /runpr can keep its pane busy for
// minutes after a merge, so the default leaves room for that.
const (
	waitPollInterval   = 3 * time.Second
	waitDefaultTimeout = 10 * time.Minute
	waitMaxTimeout     = time.Hour
)

var (
	// ErrBadWait reports a wait request whose baseline or time limit is
	// malformed. It is a usage error; nothing is read.
	ErrBadWait = errors.New("the wait request is malformed")
	// ErrWaitTimeout reports a wait that ran out before anything changed.
	ErrWaitTimeout = errors.New("nothing changed before the wait's time limit")
)

// baselineForm is the form baselineOf gives.
var baselineForm = regexp.MustCompile(`^[0-9a-f]{64}$`)

// WaitRequest is a validated `chief wait`. Only ParseWaitRequest makes one.
type WaitRequest struct {
	since   WaitBaseline
	timeout time.Duration
}

// ParseWaitRequest validates the arguments of `chief wait`; an empty timeout
// means the flag was not given. Contract: the baseline has the form the tool
// gives, and the timeout is a whole number of seconds from 1 to the longest
// wait; anything else is ErrBadWait.
func ParseWaitRequest(since, timeout string) (WaitRequest, error) {
	if !baselineForm.MatchString(since) {
		return WaitRequest{}, fmt.Errorf("%w: --since is not a baseline the tool gave", ErrBadWait)
	}
	req := WaitRequest{since: WaitBaseline(since), timeout: waitDefaultTimeout}
	if timeout != "" {
		n, err := strconv.Atoi(timeout)
		if err != nil || n < 1 || n > int(waitMaxTimeout/time.Second) || strconv.Itoa(n) != timeout {
			return WaitRequest{}, fmt.Errorf("%w: --timeout is not a number of seconds from 1 to %d", ErrBadWait, int(waitMaxTimeout/time.Second))
		}
		req.timeout = time.Duration(n) * time.Second
	}
	return req, nil
}

// baselineOf returns the baseline of the agents of the task workspaces
// named in workspaces: each agent's workspace, pane, agent, status, and
// state_change_seq. Contract: an agent of any other workspace (chief's own
// among them) does not change it, and the order Herdr lists the agents in
// does not either.
func baselineOf(agents []Agent, workspaces map[string]struct{}) WaitBaseline {
	var lines []string
	for _, a := range agents {
		if _, ok := workspaces[a.Workspace]; !ok {
			continue
		}
		lines = append(lines, strings.Join([]string{a.Workspace, a.Pane.String(), agentLabel(a.Kind), statusName(a.Status), strconv.FormatInt(a.StateChangeSeq, 10)}, "\x00"))
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return WaitBaseline(hex.EncodeToString(sum[:]))
}

// workspaceIDs returns the Herdr IDs of the task workspaces.
func workspaceIDs(found []TaskWorkspace) map[string]struct{} {
	ids := map[string]struct{}{}
	for _, w := range found {
		ids[w.ID] = struct{}{}
	}
	return ids
}

// taskWorkspaceIDs reads Herdr's workspace list and returns the IDs of the
// task workspaces among agents.
func taskWorkspaceIDs(ctx context.Context, deps Deps, herdr Herdr, chiefPane PaneID, agents []Agent) (map[string]struct{}, error) {
	workspaces, err := herdr.Workspaces(ctx)
	if err != nil {
		return nil, err
	}
	found, err := FindTaskWorkspaces(ctx, NewGitReader(deps.Cmds), deps.Cmds, agents, workspaces, chiefPane)
	if err != nil {
		return nil, err
	}
	return workspaceIDs(found), nil
}

// listedWorkspaces returns, sorted, the IDs of the workspaces the agent list
// holds other than chief's own: the membership a wait compares to tell when
// the workspaces it watches must be decided again.
func listedWorkspaces(agents []Agent, chiefPane PaneID) []string {
	chief := ""
	for _, a := range agents {
		if a.Pane == chiefPane {
			chief = a.Workspace
			break
		}
	}
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		if a.Workspace == chief || slices.Contains(ids, a.Workspace) {
			continue
		}
		ids = append(ids, a.Workspace)
	}
	slices.Sort(ids)
	return ids
}

// WaitOutcome is what `chief wait` saw: the baseline of the change.
type WaitOutcome struct {
	Baseline WaitBaseline
}

// Wait waits until the agents of the task workspaces differ from the
// baseline (the design, 3.1.1). Contract: it returns at once when they
// differ already; a task workspace opened during the wait is watched from the
// read that lists it; an agent that stays blocked, and any change in chief's
// own workspace, does not end it; it reads Herdr only and sends nothing; a
// Herdr failure, and the time limit, end it with an error. Before each read
// it checks that owner still holds the lock and that chief's pane still has
// owner's session, so a wait left running by a cleared chief session stops
// instead of acting for the session that took the lock over.
func Wait(ctx context.Context, deps Deps, owner Owner, req WaitRequest) (WaitOutcome, error) {
	if req.since == "" {
		return WaitOutcome{}, fmt.Errorf("%w: a request no parser made", ErrBadWait)
	}
	herdr := NewHerdr(deps.Cmds)
	chiefPane, err := ParsePaneID(deps.Env.PaneID)
	if err != nil {
		return WaitOutcome{}, err
	}
	agents, err := herdr.Agents(ctx)
	if err != nil {
		return WaitOutcome{}, err
	}
	ids, err := taskWorkspaceIDs(ctx, deps, herdr, chiefPane, agents)
	if err != nil {
		return WaitOutcome{}, err
	}
	listed := listedWorkspaces(agents, chiefPane)
	start := deps.Now()
	for {
		if err := stillOwner(deps.Store, owner, agents, chiefPane); err != nil {
			return WaitOutcome{}, err
		}
		if b := baselineOf(agents, ids); b != req.since {
			return WaitOutcome{Baseline: b}, nil
		}
		if deps.Now().Sub(start) >= req.timeout {
			return WaitOutcome{}, ErrWaitTimeout
		}
		if err := deps.Sleep(ctx, waitPollInterval); err != nil {
			return WaitOutcome{}, err
		}
		if agents, err = herdr.Agents(ctx); err != nil {
			return WaitOutcome{}, err
		}
		// A workspace opened during the wait brings an agent the baseline
		// compared so far does not cover, so the list's workspaces changing
		// decides the watched set again.
		if now := listedWorkspaces(agents, chiefPane); !slices.Equal(now, listed) {
			listed = now
			if ids, err = taskWorkspaceIDs(ctx, deps, herdr, chiefPane, agents); err != nil {
				return WaitOutcome{}, err
			}
		}
	}
}

// stillOwner checks, for a subcommand that runs on, that owner still holds
// the lock and that chief's pane in agents still has owner's session: after
// /clear the pane has another session, and the process the cleared session
// left running must not go on as its owner.
func stillOwner(store Store, owner Owner, agents []Agent, chiefPane PaneID) error {
	if err := store.Require(owner); err != nil {
		return err
	}
	i := slices.IndexFunc(agents, func(a Agent) bool { return a.Pane == chiefPane })
	if i < 0 || agents[i].Session != owner.Session {
		return fmt.Errorf("%w: chief's pane no longer has the session that started this run", ErrNotOwner)
	}
	return nil
}
