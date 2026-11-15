//go:build test && unix

package chief

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
)

// passingFacts returns the facts of an automatic send of /runplan 0001 after
// #42 for which every condition holds.
func passingFacts(t *testing.T) autoSendFacts {
	t.Helper()
	cmd, err := paneinput.NewRunplan(taskID(t, "0001"), paneinput.RouteRunplanOpus)
	if err != nil {
		t.Fatal(err)
	}
	return autoSendFacts{
		obs:           Observation{PR: 42, Worktree: "/w", Task: "0001", Branch: taskBranch},
		present:       true,
		state:         PRStateMerged,
		headRef:       taskBranch,
		remoteMatches: true,
		clean:         true,
		atMain:        true,
		head:          headBranch,
		branch:        nextBranch,
		next:          &Next{Kind: TaskNextRunplan, Command: &cmd, Text: "/runplan 0001"},
		autoSend:      AutoSendAuto,
	}
}

// TestAutoSendConditions takes facts for which every condition of the
// automatic send holds and breaks one condition at a time: each break must
// give a reason, so nothing is sent.
func TestAutoSendConditions(t *testing.T) {
	if why := autoSendConditions(passingFacts(t)); len(why) != 0 {
		t.Fatalf("the passing facts give %v", why)
	}
	errRead := errors.New("read failed")
	cases := map[string]func(*autoSendFacts){
		"the carry ran out":                     func(f *autoSendFacts) { f.expired = true },
		"the workspace is gone":                 func(f *autoSendFacts) { f.present = false },
		"the PR is open":                        func(f *autoSendFacts) { f.state = PRStateOpen },
		"the PR is closed":                      func(f *autoSendFacts) { f.state = PRStateClosed },
		"the PR's state is unread":              func(f *autoSendFacts) { f.state = PRStateUnknown },
		"the PR's state cannot be read":         func(f *autoSendFacts) { f.stateErr = errRead },
		"the head branch is of another task":    func(f *autoSendFacts) { f.headRef = "user/0002-other-01" },
		"the head branch is no feature branch":  func(f *autoSendFacts) { f.headRef = "main" },
		"the fetch failed":                      func(f *autoSendFacts) { f.fetchFailed = true },
		"the remote cannot be read":             func(f *autoSendFacts) { f.remoteErr = errRead },
		"origin/main is not the remote's main":  func(f *autoSendFacts) { f.remoteMatches = false },
		"the git state cannot be read":          func(f *autoSendFacts) { f.gitErr = errRead },
		"the worktree is dirty":                 func(f *autoSendFacts) { f.clean = false },
		"commits beyond origin/main":            func(f *autoSendFacts) { f.ahead = 1 },
		"HEAD is not origin/main":               func(f *autoSendFacts) { f.atMain = false },
		"the merged branch is checked out":      func(f *autoSendFacts) { f.branch = taskBranch },
		"a branch of another task":              func(f *autoSendFacts) { f.branch = "user/0002-other-01" },
		"a branch of no task":                   func(f *autoSendFacts) { f.branch = "main" },
		"HEAD was not read":                     func(f *autoSendFacts) { f.head = headUnread },
		"detached, the report disagrees":        func(f *autoSendFacts) { f.head, f.detachedWhy = headDetached, "no report" },
		"detached, the next is not /runplan":    func(f *autoSendFacts) { f.head, f.next.Kind = headDetached, TaskNextMkplan },
		"an agent runs or waits":                func(f *autoSendFacts) { f.quietWhy = "an agent of the workspace is running" },
		"no next command":                       func(f *autoSendFacts) { f.next, f.nextWhy = nil, "no rule" },
		"the next is /mkreq":                    func(f *autoSendFacts) { f.next.Kind = TaskNextMkreq },
		"the next is /runpr":                    func(f *autoSendFacts) { f.next.Kind = TaskNextRunpr },
		"the next is the takeover":              func(f *autoSendFacts) { f.next.Kind = TaskNextRunplanTakeover },
		"the next has no route":                 func(f *autoSendFacts) { f.next.Command, f.next.Problem = nil, "no route" },
		"the plan PR holds":                     func(f *autoSendFacts) { f.autoSend = AutoSendHold },
		"the plan PR's field is unreadable":     func(f *autoSendFacts) { f.autoSend = AutoSendUnreadable },
		"already sent":                          func(f *autoSendFacts) { f.sent = true },
		"the automatic-send record is unread":   func(f *autoSendFacts) { f.sentErr = errRead },
		"a /runpr report disagrees":             func(f *autoSendFacts) { f.reportWhy = "the report says none" },
		"the workspace is stopped":              func(f *autoSendFacts) { f.stopped = true },
		"another send started during the round": func(f *autoSendFacts) { f.laterSend = true },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			f := passingFacts(t)
			edit(&f)
			if why := autoSendConditions(f); len(why) == 0 {
				t.Error("the broken condition gave no reason")
			}
		})
	}
	t.Run("the task is complete", func(t *testing.T) {
		f := passingFacts(t)
		f.next, f.done = nil, true
		if why := autoSendConditions(f); !slices.Equal(why, []string{"the task is complete"}) {
			t.Errorf("a complete task gave %q", why)
		}
	})
	t.Run("the hold field is not read for /mkarch", func(t *testing.T) {
		f := passingFacts(t)
		f.next.Kind, f.autoSend = TaskNextMkarch, AutoSendHold
		if why := autoSendConditions(f); len(why) != 0 {
			t.Errorf("/mkarch gave %v", why)
		}
	})
	t.Run("detached for /runplan with an agreeing report", func(t *testing.T) {
		f := passingFacts(t)
		f.head, f.branch = headDetached, ""
		if why := autoSendConditions(f); len(why) != 0 {
			t.Errorf("a detached HEAD gave %v", why)
		}
	})
}

// The values of the merge worlds.
const (
	mainCommit   = "1111111111111111111111111111111111111111"
	nextBranch   = "user/0001-test-task-03"
	prStateLine  = "gh pr view 42 --json " + prStateFields
	prOpenJSON   = `{"headRefName":"user/0001-test-task-02","number":42,"state":"OPEN"}`
	prMergedJSON = `{"headRefName":"user/0001-test-task-02","number":42,"state":"MERGED"}`
)

// mergeWorld is a task workspace w1, with a Claude Code pane (p1) and an
// OpenCode pane (p2), whose branch has PR 42 open, and a started chief
// whose ticks a test runs around the PR's merge.
type mergeWorld struct {
	*sendWorld
	tree string
	deps Deps
}

func newMergeWorld(t *testing.T, docs fakeDocs) *mergeWorld {
	t.Helper()
	sw := newSendWorld(t)
	tree := sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON, Docs: &docs}, "claude", "opencode")
	sw.git(sw.chiefTree, "rev-parse "+originMain, mainCommit+"\n")
	sw.git(sw.chiefTree, "ls-remote origin "+remoteMainRef, mainCommit+"\trefs/heads/main\n")
	sw.git(tree, "status --porcelain --untracked-files=all", "")
	sw.git(tree, "rev-list --count "+originMain+"..HEAD", "0\n")
	sw.git(tree, "rev-parse HEAD", mainCommit+"\n")
	sw.git(tree, "rev-parse "+originMain, mainCommit+"\n")
	sw.rec.replyIn(tree, prStateLine, []byte(prOpenJSON))
	mw := &mergeWorld{sendWorld: sw, tree: tree, deps: sw.finish()}
	if _, err := sw.store.Start(context.Background(), NewHerdr(mw.deps.Cmds), sw.owner(), sw.now); err != nil {
		t.Fatal(err)
	}
	return mw
}

// mkarchDocs are documents whose next command is /mkarch 0001, on Claude
// Code.
var mkarchDocs = fakeDocs{Dir: taskDir, Req: reqApproved}

// runplanDocs are documents whose next command is /runplan 0001 by the
// plan sample.
func runplanDocs(t *testing.T, plan string) fakeDocs {
	return fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, plan)}
}

// merge makes PR 42 merged as /runpr leaves it: the worktree at origin/main
// on branch (detachedHead for none), and the OpenCode pane showing /runpr's
// report with next as its NEXT ("" keeps the sample's "none").
func (mw *mergeWorld) merge(branch, next string) {
	mw.t.Helper()
	mw.git(mw.tree, "rev-parse --abbrev-ref HEAD", branch+"\n")
	if branch != detachedHead {
		mw.rec.replyIn(mw.tree, "gh pr list --head "+branch+" --state open --json headRefName,number,title,url", []byte("[]"))
	}
	mw.rec.replyIn(mw.tree, prStateLine, []byte(prMergedJSON))
	mw.mu.Lock()
	defer mw.mu.Unlock()
	p := mw.byPane["w1:p2"]
	p.Status, p.screen = "done", runprReportScreen(mw.t, 42, next)
}

// tick moves the clock on by d and runs a tick.
func (mw *mergeWorld) tick(d time.Duration) TickOutcome {
	mw.t.Helper()
	mw.now = mw.now.Add(d)
	out, err := Tick(context.Background(), mw.deps, mw.owner())
	if err != nil {
		mw.t.Fatal(err)
	}
	return out
}

// only returns the one automatic-send result of a tick.
func only(t *testing.T, out TickOutcome) AutoSendResult {
	t.Helper()
	if len(out.AutoSends) != 1 {
		t.Fatalf("auto sends = %+v; want one", out.AutoSends)
	}
	return out.AutoSends[0]
}

// sentCommand reports whether res typed its command and returns why not.
func sentCommand(res AutoSendResult) bool {
	return len(res.Reasons) == 0 && len(res.Actions) > 0 && res.Actions[0].Input == InputCommand && res.Actions[0].Outcome == OutcomeSent
}

// TestAutoSendAfterMerge runs the ticks around a merge: the tick that saw
// the PR open sends nothing, the next one after the merge sends /mkarch to
// the Claude Code pane with its inputs and empties the /runpr pane, the row
// shows the command running, and a later tick sends nothing more.
func TestAutoSendAfterMerge(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 {
		t.Fatalf("the tick of the open PR sent %+v", out.AutoSends)
	}
	mw.merge(nextBranch, "`/mkarch 0001`")
	out := mw.tick(time.Minute)
	res := only(t, out)
	if !sentCommand(res) || res.Actions[0].Pane != "w1:p1" || res.Actions[0].Command != "/mkarch 0001" || res.Actions[0].Model != "opus" {
		t.Fatalf("auto send = %+v", res)
	}
	if len(res.Actions) != 2 || res.Actions[1].Input != InputReset || res.Actions[1].Pane != "w1:p2" || res.Actions[1].Outcome != OutcomeSent {
		t.Errorf("reset = %+v; want /new on the /runpr pane", res.Actions[1:])
	}
	want := []string{"w1:p1 /clear", "w1:p1 /model opus", "w1:p1 /mkarch 0001", "w1:p2 /new"}
	if got := mw.rec.prompts(); !slices.Equal(got, want) {
		t.Errorf("prompts = %v, want %v", got, want)
	}
	if row := rowOf(t, out.Snapshot, "ws"); row.State != StateRunning || row.Running == nil || row.Running.Command != "/mkarch 0001" {
		t.Errorf("row = %+v; want running /mkarch 0001", row)
	}
	if slices.ContainsFunc(out.Snapshot.Pending, func(p PendingItem) bool { return p.Kind == PendingProposal }) {
		t.Errorf("the command sent is still proposed: %+v", out.Snapshot.Pending)
	}
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 {
		t.Errorf("a later tick considered %+v", out.AutoSends)
	}
	if got := mw.rec.prompts(); len(got) != len(want) {
		t.Errorf("a later tick typed %v", got[len(want):])
	}
}

// TestAutoSendReportMismatch checks that a /runpr report whose NEXT differs
// from the tool's next command in the command, in the task, or by naming
// none stops the send, and that the command is proposed with the reason.
func TestAutoSendReportMismatch(t *testing.T) {
	for _, next := range []string{"", "`/mkplan 0001`", "`/mkarch 0002`", "/mkarch"} {
		t.Run(next, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			mw.tick(time.Minute)
			mw.merge(nextBranch, next)
			out := mw.tick(time.Minute)
			res := only(t, out)
			if sentCommand(res) || len(mw.rec.prompts()) != 0 {
				t.Fatalf("sent with the report's NEXT %q: %+v", next, res)
			}
			if !slices.ContainsFunc(res.Reasons, func(r string) bool { return strings.Contains(r, "w1:p2") }) {
				t.Errorf("reasons = %v; want the /runpr report's pane", res.Reasons)
			}
			i := slices.IndexFunc(out.Snapshot.Pending, func(p PendingItem) bool { return p.Kind == PendingProposal })
			if i < 0 || out.Snapshot.Pending[i].Proposal.Text != "/mkarch 0001" || len(out.Snapshot.Pending[i].Reasons) == 0 {
				t.Errorf("pending = %+v; want /mkarch 0001 proposed with the reasons", out.Snapshot.Pending)
			}
		})
	}
}

// TestAutoSendHold checks that a plan PR whose auto-send field is hold is
// not sent and is proposed with the reason.
func TestAutoSendHold(t *testing.T) {
	mw := newMergeWorld(t, runplanDocs(t, "marker-hold.md"))
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/runplan 0001`")
	res := only(t, mw.tick(time.Minute))
	if sentCommand(res) || !slices.Contains(res.Reasons, "the plan PR's auto-send field is hold") {
		t.Fatalf("auto send = %+v; want the hold reason", res)
	}
	if res.Proposal == nil || res.Proposal.Text != "/runplan 0001" {
		t.Errorf("proposal = %+v", res.Proposal)
	}
}

// TestAutoSendUnobservedMerge checks that a merge no earlier round of the
// session saw open is not sent and stays a proposal: the first tick after
// the merge, and a tick whose previous round is observeWindow or more
// before it. A previous round just inside the window sends.
func TestAutoSendUnobservedMerge(t *testing.T) {
	t.Run("the first tick", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.merge(nextBranch, "`/mkarch 0001`")
		out := mw.tick(time.Minute)
		if len(out.AutoSends) != 0 || len(mw.rec.prompts()) != 0 {
			t.Fatalf("an unobserved merge was considered: %+v", out.AutoSends)
		}
		if row := rowOf(t, out.Snapshot, "ws"); row.State != StateProposeNext || row.Next == nil || row.Next.Text != "/mkarch 0001" {
			t.Errorf("row = %+v; want /mkarch 0001 proposed", row)
		}
	})
	for gap, sends := range map[time.Duration]bool{observeWindow: false, observeWindow - time.Second: true} {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/mkarch 0001`")
		out := mw.tick(gap)
		if got := len(out.AutoSends) == 1 && sentCommand(out.AutoSends[0]); got != sends {
			t.Errorf("a previous round %s before: sent %v, want %v (%+v)", gap, got, sends, out.AutoSends)
		}
	}
}

// TestAutoSendOlderObservation checks that an observation of a round before
// the previous one never counts: the PR seen open, then a round that could
// not read the PR list, then the merge.
func TestAutoSendOlderObservation(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.rec.failIn(mw.tree, "gh pr list --head "+taskBranch+" --state open --json headRefName,number,title,url", errors.New("gh failed"))
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 {
		t.Fatalf("the round of the still open PR considered %+v", out.AutoSends)
	}
	mw.merge(nextBranch, "`/mkarch 0001`")
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 || len(mw.rec.prompts()) != 0 {
		t.Errorf("an observation older than the previous round was sent: %+v", out.AutoSends)
	}
}

// TestAutoSendTaskMismatch checks that a merged PR whose head branch names
// another task than the observation's is not sent.
func TestAutoSendTaskMismatch(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	mw.rec.replyIn(mw.tree, prStateLine, []byte(`{"headRefName":"user/0002-other-task-01","number":42,"state":"MERGED"}`))
	res := only(t, mw.tick(time.Minute))
	if sentCommand(res) || len(mw.rec.prompts()) != 0 {
		t.Fatalf("a PR of another task was sent: %+v", res)
	}
	if !slices.ContainsFunc(res.Reasons, func(r string) bool { return strings.HasPrefix(r, `the head branch "user/0002-other-task-01"`) }) {
		t.Errorf("reasons = %q; want the head branch's", res.Reasons)
	}
}

// TestAutoSendDetached checks a detached HEAD: /runplan is sent when the one
// /runpr pane's last report names the merged PR and the command, and not
// when it names another PR.
func TestAutoSendDetached(t *testing.T) {
	for pr, sends := range map[int]bool{42: true, 43: false} {
		mw := newMergeWorld(t, runplanDocs(t, "marker-opus.md"))
		mw.tick(time.Minute)
		mw.merge(detachedHead, "`/runplan 0001`")
		if pr != 42 {
			mw.byPane["w1:p2"].screen = runprReportScreen(t, pr, "`/runplan 0001`")
		}
		res := only(t, mw.tick(time.Minute))
		if sentCommand(res) != sends {
			t.Errorf("a report naming #%d: sent %v, want %v (%+v)", pr, sentCommand(res), sends, res)
		}
		if !sends && !slices.ContainsFunc(res.Reasons, func(r string) bool { return strings.HasPrefix(r, "HEAD is detached and") }) {
			t.Errorf("a report naming #%d: reasons %q; want the detached HEAD's", pr, res.Reasons)
		}
	}
}

// TestAutoSendNextRound checks the carry: a merge whose workspace was still
// running in the round that saw it merged is sent by the next round, while
// carryWindow has not run out since the merge was first seen, and not after.
func TestAutoSendNextRound(t *testing.T) {
	for gap, sends := range map[time.Duration]bool{time.Minute: true, carryWindow - time.Second: true, carryWindow: false} {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/mkarch 0001`")
		mw.byPane["w1:p1"].Status = "working"
		res := only(t, mw.tick(time.Minute))
		if sentCommand(res) || !slices.Contains(res.Reasons, "an agent of the workspace is running") {
			t.Fatalf("the round with a running agent: %+v", res)
		}
		mw.byPane["w1:p1"].Status = "idle"
		out := mw.tick(gap)
		got := len(out.AutoSends) == 1 && sentCommand(out.AutoSends[0])
		if got != sends {
			t.Errorf("the next round %s later: sent %v, want %v (%+v)", gap, got, sends, out.AutoSends)
		}
		if !sends && len(mw.rec.prompts()) != 0 {
			t.Errorf("typed %v after the carry ran out", mw.rec.prompts())
		}
		if !sends {
			if out := mw.tick(time.Minute); len(out.AutoSends) != 0 {
				t.Errorf("the merge was carried after its window ran out: %+v", out.AutoSends)
			}
		}
	}
}

// TestAutoSendTaskComplete checks that the merge of a task's last PR sends
// nothing, says the task is complete, and is not carried.
func TestAutoSendTaskComplete(t *testing.T) {
	mw := newMergeWorld(t, runplanDocs(t, "marker-all-merged.md"))
	mw.tick(time.Minute)
	mw.merge(nextBranch, "")
	res := only(t, mw.tick(time.Minute))
	if sentCommand(res) || !slices.Contains(res.Reasons, "the task is complete") {
		t.Fatalf("the last PR's merge = %+v; want the task complete", res)
	}
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 {
		t.Errorf("a complete task's merge was carried: %+v", out.AutoSends)
	}
}

// TestAutoSendOnce checks that a merged PR is sent at most once: not again
// by a later observation of it (as after chief restarted), not when its
// automatic-send record cannot be read, and not again after the record was
// written and the send failed before its first input.
func TestAutoSendOnce(t *testing.T) {
	t.Run("observed again after the send", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/mkarch 0001`")
		out := mw.tick(time.Minute)
		if !sentCommand(only(t, out)) {
			t.Fatalf("the first send = %+v", out.AutoSends)
		}
		mw.byPane["w1:p1"].Status = "idle"
		mw.byPane["w1:p1"].screen = readScreenSample(t, "claude-idle.ansi")
		mw.byPane["w1:p2"].screen = runprReportScreen(t, 42, "`/mkarch 0001`")
		ids, err := mw.store.sessionRounds(mw.owner().Session)
		if err != nil {
			t.Fatal(err)
		}
		if err := mw.store.RecordObservation(Observation{Session: mw.owner().Session, Round: ids[len(ids)-1], PR: 42, Worktree: mw.tree, Task: "0001", Branch: taskBranch}); err != nil {
			t.Fatal(err)
		}
		before := len(mw.rec.prompts())
		res := only(t, mw.tick(time.Minute))
		if sentCommand(res) || !slices.Contains(res.Reasons, "#42 was already sent automatically") || len(mw.rec.prompts()) != before {
			t.Errorf("a second send = %+v", res)
		}
	})
	t.Run("the record cannot be read", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/mkarch 0001`")
		if err := os.WriteFile(filepath.Join(mw.store.Dir(), autoSendName(42)), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		res := only(t, mw.tick(time.Minute))
		if sentCommand(res) || len(mw.rec.prompts()) != 0 {
			t.Fatalf("sent with an unreadable record: %+v", res)
		}
		if out := mw.tick(time.Minute); len(out.AutoSends) != 0 {
			t.Errorf("an unreadable record was carried: %+v", out.AutoSends)
		}
	})
	t.Run("recorded, then the send record fails", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/mkarch 0001`")
		mw.store.beforeCreate = func(name string) error {
			if strings.HasPrefix(name, sendPrefix) {
				return errors.New("disk full")
			}
			return nil
		}
		mw.deps.Store = mw.store
		res := only(t, mw.tick(time.Minute))
		if len(res.Actions) != 1 || res.Actions[0].Outcome != OutcomeRecordedNotSent || len(mw.rec.prompts()) != 0 {
			t.Fatalf("auto send = %+v; want recorded and not sent", res)
		}
		mw.store.beforeCreate = nil
		mw.deps.Store = mw.store
		if out := mw.tick(time.Minute); len(out.AutoSends) != 0 || len(mw.rec.prompts()) != 0 {
			t.Errorf("the used-up send was tried again: %+v", out.AutoSends)
		}
	})
}

// TestAutoSendBusyWorkspace checks that a workspace another send holds is
// not sent to, nothing is recorded, and the next round sends.
func TestAutoSendBusyWorkspace(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	records, err := mw.store.Records(Worktree{path: mw.tree})
	if err != nil {
		t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		t.Fatal(err)
	}
	res := only(t, mw.tick(time.Minute))
	if len(res.Actions) != 1 || res.Actions[0].Reason != NotSentBusy {
		t.Fatalf("auto send to a held workspace = %+v", res)
	}
	if sent, err := mw.store.AutoSent(42); sent || err != nil {
		t.Fatalf("the busy workspace's send was recorded: %v, %v", sent, err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if !sentCommand(only(t, mw.tick(time.Minute))) {
		t.Error("the next round did not send")
	}
}

// TestResetAfterAutoSend checks the reset of the /runpr pane: it is emptied
// when the command went to another pane (TestAutoSendAfterMerge), and left
// as it is, with the reason, when the command went to that pane or the
// pane's last report no longer names the merged PR.
func TestResetAfterAutoSend(t *testing.T) {
	t.Run("the command went to the /runpr pane", func(t *testing.T) {
		mw := newMergeWorld(t, runplanDocs(t, "marker-sonnet.md"))
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/runplan 0001`")
		res := only(t, mw.tick(time.Minute))
		if !sentCommand(res) || res.Actions[0].Pane != "w1:p2" {
			t.Fatalf("auto send = %+v; want /runplan on OpenCode", res)
		}
		if len(res.Actions) != 2 || res.Actions[1].Reason != NotSentSamePane || res.Actions[1].Outcome != OutcomeNotSent {
			t.Errorf("reset = %+v; want none, for the same pane", res.Actions[1:])
		}
	})
	t.Run("the report names another command", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "")
		// The /runpr pane's last message names the PR but no NEXT, which
		// lets the send go on (no report disagrees) and is no report of the
		// command sent, so the pane is not emptied.
		screen := strings.Replace(runprReportScreen(t, 42, ""), runprNoneLine+"\n", "", 1)
		if !strings.Contains(screen, "#42") || strings.Contains(screen, "NEXT:") {
			t.Fatal("the edited report still has its NEXT line or lost its PR")
		}
		mw.byPane["w1:p2"].screen = screen
		res := only(t, mw.tick(time.Minute))
		if !sentCommand(res) || len(res.Actions) != 2 || res.Actions[1].Reason != NotSentNoReport {
			t.Fatalf("auto send = %+v; want the command sent and no reset", res)
		}
	})
	t.Run("the report names another PR", func(t *testing.T) {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/mkarch 0001`")
		mw.byPane["w1:p2"].screen = runprReportScreen(t, 43, "`/mkarch 0001`")
		res := only(t, mw.tick(time.Minute))
		if !sentCommand(res) || len(res.Actions) != 2 || res.Actions[1].Reason != NotSentNoReport {
			t.Fatalf("auto send = %+v; want the command sent and no reset", res)
		}
		if slices.Contains(mw.rec.prompts(), "w1:p2 /new") {
			t.Error("the /runpr pane was emptied")
		}
	})
}

// TestAutoSendStopsForANewOwner checks that a tick whose chief session was
// replaced while it ran sends nothing: the owner is checked again under the
// workspace's exclusion, before the first input, by the lock (another
// session took it over) and, on its own, by the session chief's pane has
// now (the session was cleared and nothing took the lock yet).
func TestAutoSendStopsForANewOwner(t *testing.T) {
	for _, c := range []struct {
		name    string
		paneToo bool
		// call is the read of the merged PR's state before which the
		// change comes: 1, before the exclusion; 2, while the conditions
		// are read again under it.
		call int
	}{
		{"the lock is taken over", false, 1},
		{"only the pane's session changes", true, 1},
		{"the lock is taken over under the exclusion", false, 2},
	} {
		paneToo := c.paneToo
		t.Run(c.name, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			mw.tick(time.Minute)
			mw.merge(nextBranch, "`/mkarch 0001`")
			old := mw.owner()
			// The change comes while the tick reads the merged PR, after
			// it read the previous round's observations and before it takes
			// the exclusion.
			took, calls := false, 0
			mw.rec.replyFunc(prStateLine, func() reply {
				calls++
				if calls == c.call {
					took = true
					cleared := mw.chief
					cleared.Session = "00000000-0000-4000-8000-0000000000c9"
					if paneToo {
						mw.chief = cleared
						mw.rec.reply("herdr agent get "+worldChiefPane, agentGetOutput(cleared))
					} else if _, err := mw.store.Start(context.Background(), herdrWith(t, cleared), Owner{Pane: cleared.Pane, Session: cleared.Session}, mw.now); err != nil {
						t.Error(err)
					}
				}
				return reply{out: []byte(prMergedJSON)}
			})
			mw.now = mw.now.Add(time.Minute)
			out, err := Tick(context.Background(), mw.deps, old)
			if err != nil {
				t.Fatal(err)
			}
			res := only(t, out)
			if !took || len(res.Actions) != 1 || res.Actions[0].Outcome != OutcomeNotSent || len(mw.rec.prompts()) != 0 {
				t.Fatalf("auto send of the replaced owner = %+v (took over %v), prompts %v", res, took, mw.rec.prompts())
			}
			if sent, _ := mw.store.AutoSent(42); sent {
				t.Error("the replaced owner recorded the automatic send")
			}
		})
	}
}

// TestTickWithoutRound checks that a tick whose round cannot be recorded
// records no observation of the open PR, considers no merge, and says why.
func TestTickWithoutRound(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	observations := func() int {
		entries, err := os.ReadDir(mw.store.Dir())
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), observePrefix) {
				n++
			}
		}
		return n
	}
	before := observations()
	mw.store.beforeCreate = func(name string) error {
		if strings.HasPrefix(name, roundPrefix) {
			return errors.New("disk full")
		}
		return nil
	}
	mw.deps.Store = mw.store
	out := mw.tick(time.Minute)
	if got := observations(); got != before {
		t.Errorf("observations = %d, want %d: the tick without its round observed the open PR", got, before)
	}
	if !slices.ContainsFunc(out.Snapshot.Problems, func(p string) bool { return strings.Contains(p, "the round cannot be recorded") }) {
		t.Errorf("problems = %v; want the round's failure", out.Snapshot.Problems)
	}
	mw.merge(nextBranch, "`/mkarch 0001`")
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 || len(mw.rec.prompts()) != 0 {
		t.Errorf("a tick without its round considered %+v", out.AutoSends)
	}
}

// TestGitClean runs the clean check on a real repository: clean, an
// ignored file, an untracked file, and a changed tracked file.
func TestGitClean(t *testing.T) {
	repo := newTestRepo(t)
	repo.Write(".gitignore", "ignored.txt\n")
	repo.CommitAll("ignore")
	// The real git runs from the test tools only for this test; herdr and
	// gh are still not found.
	t.Setenv("PATH", testToolsDir)
	git := NewGitReader(NewCommands(func(o cmdrunner.Options) cmdrunner.Runner { return cmdrunner.NewOS(o) }, repo.Env()))
	clean := func() bool {
		t.Helper()
		ok, err := git.Clean(context.Background(), repo.Dir)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !clean() {
		t.Fatal("a clean worktree reads as changed")
	}
	repo.Write("ignored.txt", "x\n")
	if !clean() {
		t.Error("an ignored file reads as a change")
	}
	repo.Git("config", "status.showUntrackedFiles", "no")
	repo.Write("new.txt", "x\n")
	if clean() {
		t.Error("an untracked file reads as clean")
	}
	if err := os.Remove(filepath.Join(repo.Dir, "new.txt")); err != nil {
		t.Fatal(err)
	}
	repo.Write("README.md", "changed\n")
	if clean() {
		t.Error("a changed tracked file reads as clean")
	}
}

// TestParsePRState reads the state of a PR as gh gives it, and rejects a
// view of another PR, of an unknown state, or without a head branch.
func TestParsePRState(t *testing.T) {
	cases := map[string]struct {
		out     string
		want    PRState
		wantErr bool
	}{
		"merged":           {prMergedJSON, PRStateMerged, false},
		"open":             {prOpenJSON, PRStateOpen, false},
		"another PR":       {strings.Replace(prMergedJSON, `42`, `43`, 1), PRStateUnknown, true},
		"an unknown state": {strings.Replace(prMergedJSON, `MERGED`, `DRAFT`, 1), PRStateUnknown, true},
		"no head branch":   {strings.Replace(prMergedJSON, `"headRefName":"user/0001-test-task-02",`, ``, 1), PRStateUnknown, true},
	}
	for name, c := range cases {
		got, head, err := parsePRState([]byte(c.out), 42)
		if got != c.want || errors.Is(err, errGHOutput) != c.wantErr || (err == nil && head != taskBranch) {
			t.Errorf("%s: %d, %q, %v; want %d, error %v", name, got, head, err, c.want, c.wantErr)
		}
	}
}

// TestAutoSendRechecksUnderTheExclusion changes the world between the first
// reading of the conditions and their reading under the workspace's
// exclusion: an agent starts, another send starts, or the next command
// changes. Each time nothing is typed.
func TestAutoSendRechecksUnderTheExclusion(t *testing.T) {
	cases := []struct {
		name string
		// call is the read of the merged PR's state (1: the first reading,
		// 2: the reading under the exclusion) before which change runs.
		call int
		// docs are the documents before the change; empty is mkarchDocs.
		docs   *fakeDocs
		change func(mw *mergeWorld)
		check  func(t *testing.T, res AutoSendResult)
	}{
		{name: "the route of the next command changes", call: 2, docs: new(runplanDocs(t, "marker-opus.md")), change: func(mw *mergeWorld) {
			mw.docs(mw.tree, runplanDocs(mw.t, "marker-sonnet.md"))
		}},
		{name: "an agent starts", call: 1, change: func(mw *mergeWorld) { mw.byPane["w1:p1"].Status = "working" }},
		{name: "another send starts", call: 1, change: func(mw *mergeWorld) {
			records, err := mw.store.Records(Worktree{path: mw.tree})
			if err != nil {
				mw.t.Fatal(err)
			}
			held, err := records.Lock()
			if err != nil {
				mw.t.Fatal(err)
			}
			defer func() { _ = held.Close() }()
			id, err := held.BeginSend(SendRecord{Pane: "w1:p1", Input: InputCommand.String(), Command: "/mkarch 0001"}, mw.now)
			if err != nil {
				mw.t.Fatal(err)
			}
			if err := held.EndSend(id, OutcomeSent.String(), "", mw.now); err != nil {
				mw.t.Fatal(err)
			}
		}},
		{name: "the next command changes", call: 2, change: func(mw *mergeWorld) {
			mw.docs(mw.tree, fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved})
		}, check: func(t *testing.T, res AutoSendResult) {
			if res.Proposal == nil || res.Proposal.Text != "/mkplan 0001" {
				t.Errorf("proposal = %+v; want the changed /mkplan 0001", res.Proposal)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			docs := mkarchDocs
			if c.docs != nil {
				docs = *c.docs
			}
			mw := newMergeWorld(t, docs)
			mw.tick(time.Minute)
			mw.merge(nextBranch, "")
			// A pane with no NEXT line agrees with any next command.
			mw.byPane["w1:p2"].screen = readScreenSample(t, "opencode-done.ansi")
			calls := 0
			mw.rec.replyFunc(prStateLine, func() reply {
				calls++
				if calls == c.call {
					c.change(mw)
				}
				return reply{out: []byte(prMergedJSON)}
			})
			res := only(t, mw.tick(time.Minute))
			if calls < 2 {
				t.Fatalf("the state was read %d times; the test reached no reading under the exclusion", calls)
			}
			if sentCommand(res) || len(mw.rec.prompts()) != 0 {
				t.Fatalf("sent after %s: %+v, prompts %v", c.name, res, mw.rec.prompts())
			}
			if sent, _ := mw.store.AutoSent(42); sent {
				t.Error("the automatic send was recorded")
			}
			if c.check != nil {
				c.check(t, res)
			}
		})
	}
}

// TestAutoSendCarryKeepsTheFirstSighting carries a merge through two rounds
// with a running agent and checks that the carry window counts from the
// round that first saw the merge, not from the last carry.
func TestAutoSendCarryKeepsTheFirstSighting(t *testing.T) {
	for last, sends := range map[time.Duration]bool{carryWindow - time.Second: true, carryWindow: false} {
		mw := newMergeWorld(t, mkarchDocs)
		mw.tick(time.Minute)
		mw.merge(nextBranch, "`/mkarch 0001`")
		mw.byPane["w1:p1"].Status = "working"
		first := mw.now.Add(time.Minute)
		mw.tick(time.Minute)
		mw.tick(time.Minute)
		mw.byPane["w1:p1"].Status = "idle"
		out := mw.tick(first.Add(last).Sub(mw.now))
		got := len(out.AutoSends) == 1 && sentCommand(out.AutoSends[0])
		if got != sends {
			t.Errorf("%s after the first sighting: sent %v, want %v (%+v)", last, got, sends, out.AutoSends)
		}
	}
}

// TestAutoSendAfterAFailedTick checks that a tick that fails to read Herdr
// records no round, so the next tick still counts the observation before
// it and sends.
func TestAutoSendAfterAFailedTick(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	errBusy := errors.New("herdr is busy")
	mw.rec.fail("herdr workspace list", errBusy)
	mw.now = mw.now.Add(time.Minute)
	if _, err := Tick(context.Background(), mw.deps, mw.owner()); !errors.Is(err, errBusy) {
		t.Fatal("the tick with a failing Herdr did not fail")
	}
	mw.rec.reply("herdr workspace list", workspaceListOutput(mw.labels))
	mw.merge(nextBranch, "`/mkarch 0001`")
	if res := only(t, mw.tick(time.Minute)); !sentCommand(res) {
		t.Errorf("auto send after a failed tick = %+v", res)
	}
}

// TestAutoSendUncheckedConditions checks that a merge that cannot be
// confirmed (its state cannot be read, or its workspace is gone) names that
// reason and says the other conditions were not checked, never claiming a
// value of a fact it did not read.
func TestAutoSendUncheckedConditions(t *testing.T) {
	notChecked := "the other conditions were not checked, since the merge could not be confirmed"
	for name, edit := range map[string]func(*autoSendFacts){
		"the state cannot be read": func(f *autoSendFacts) { f.stateErr = errors.New("gh failed") },
		"the workspace is gone":    func(f *autoSendFacts) { f.present = false },
	} {
		f := autoSendFacts{obs: Observation{PR: 42, Worktree: "/w", Task: "0001"}, present: true, state: PRStateMerged, headRef: taskBranch, partial: true}
		edit(&f)
		why := autoSendConditions(f)
		if len(why) != 2 || why[1] != notChecked {
			t.Errorf("%s: reasons %q; want its own reason and %q", name, why, notChecked)
		}
	}
}

// TestSentSince checks the boundary of SentSince: a send started at the
// given time counts, one started before it does not.
func TestSentSince(t *testing.T) {
	store := newStore(t)
	records, err := store.Records(Worktree{path: "/w"})
	if err != nil {
		t.Fatal(err)
	}
	held, err := records.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()
	at := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	if _, err := held.BeginSend(SendRecord{Pane: "w1:p1"}, at); err != nil {
		t.Fatal(err)
	}
	for since, want := range map[time.Time]bool{at: true, at.Add(time.Nanosecond): false, at.Add(-time.Second): true} {
		if got, err := records.SentSince(since); err != nil || got != want {
			t.Errorf("SentSince(%s) = %v, %v; want %v", since, got, err, want)
		}
	}
}

// TestAutoSendFailureAfterInput checks an automatic send whose command fails
// after an input: the row needs a check, the workspace is stopped, and the
// used-up send is not tried again.
func TestAutoSendFailureAfterInput(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	mw.react = func(p *simPane, text string) error {
		if text == "/mkarch 0001" {
			return errors.New("herdr lost the pane")
		}
		return mw.realReact(p, text)
	}
	out := mw.tick(time.Minute)
	res := only(t, out)
	if len(res.Actions) != 1 || res.Actions[0].Outcome != OutcomeSentThenFailed {
		t.Fatalf("auto send = %+v; want sent, then failed", res)
	}
	if row := rowOf(t, out.Snapshot, "ws"); row.State != StateNeedsCheck {
		t.Errorf("row = %+v; want needs check", row)
	}
	if records, ok, err := mw.store.existingRecords(Worktree{path: mw.tree}); err != nil || !ok {
		t.Fatalf("records = %v, %v", ok, err)
	} else if stop, err := records.Stopped(); err != nil || stop == nil {
		t.Errorf("stop = %+v, %v; want the workspace stopped", stop, err)
	}
	before := len(mw.rec.prompts())
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 || len(mw.rec.prompts()) != before {
		t.Errorf("the failed send was tried again: %+v", out.AutoSends)
	}
}

// TestAutoSendStateUnreadable checks that a merge whose state cannot be read
// is not sent, gives that reason, and is not carried to the next round.
func TestAutoSendStateUnreadable(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	mw.rec.failIn(mw.tree, prStateLine, errors.New("gh failed"))
	if res := only(t, mw.tick(time.Minute)); sentCommand(res) || !slices.ContainsFunc(res.Reasons, func(r string) bool { return strings.HasPrefix(r, "the state of #42 cannot be read") }) {
		t.Fatalf("auto send without the PR's state = %+v; want its reason", res)
	}
	mw.rec.replyIn(mw.tree, prStateLine, []byte(prMergedJSON))
	if out := mw.tick(time.Minute); len(out.AutoSends) != 0 {
		t.Errorf("an unread state was carried: %+v", out.AutoSends)
	}
}

// TestAutoSendAfterAManualSend checks that a command the human sent by hand
// after the round that saw the PR open is not sent again by itself, though
// the command ended and left the workspace waiting: when the next round sees
// the merge at once, when the merge is carried through a round in which the
// command ran, and when a start of the same session ran before the tick.
func TestAutoSendAfterAManualSend(t *testing.T) {
	for _, c := range []struct {
		name         string
		carry, start bool
	}{
		{name: "the next round"},
		{name: "after a carry", carry: true},
		{name: "after a start", start: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			mw.tick(time.Minute)
			mw.now = mw.now.Add(time.Second)
			records, err := mw.store.Records(Worktree{path: mw.tree})
			if err != nil {
				t.Fatal(err)
			}
			held, err := records.Lock()
			if err != nil {
				t.Fatal(err)
			}
			id, err := held.BeginSend(SendRecord{Pane: "w1:p1", Input: InputCommand.String(), Command: "/mkarch 0001"}, mw.now)
			if err == nil {
				err = held.EndSend(id, OutcomeSent.String(), "", mw.now)
			}
			if cerr := held.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				t.Fatal(err)
			}
			mw.merge(nextBranch, "`/mkarch 0001`")
			if c.carry {
				// The command sent by hand runs through the round that first
				// sees the merge, so the merge is carried.
				mw.byPane["w1:p1"].Status = "working"
				if res := only(t, mw.tick(time.Minute)); sentCommand(res) {
					t.Fatalf("sent with a running agent: %+v", res)
				}
				mw.byPane["w1:p1"].Status = "idle"
			}
			if c.start {
				if _, err := mw.store.Start(context.Background(), NewHerdr(mw.deps.Cmds), mw.owner(), mw.now); err != nil {
					t.Fatal(err)
				}
			}
			res := only(t, mw.tick(time.Minute))
			if sentCommand(res) || len(mw.rec.prompts()) != 0 {
				t.Errorf("the command sent by hand was sent again: %+v", res)
			}
		})
	}
}

// addSharingWorkspace adds a Herdr workspace w2 on the worktree of w1, with
// a Claude Code agent of status, to the replies of Herdr from now on.
func (mw *mergeWorld) addSharingWorkspace(status string) {
	mw.t.Helper()
	p := &simPane{fakeAgent: fakeAgent{Pane: "w2:p1", Workspace: "w2", Kind: "claude", Status: status, Dir: mw.tree, Seq: 10, Session: mw.newSession()}, screen: readScreenSample(mw.t, "claude-idle.ansi")}
	if err := mw.writeTranscript(p, p.Session, modeLine(p.Session)); err != nil {
		mw.t.Fatal(err)
	}
	for _, src := range []string{"visible", "recent"} {
		mw.rec.replyFunc("herdr agent read "+p.Pane+" --source "+src+" --lines 200 --format ansi", func() reply { return reply{out: []byte(p.screen)} })
	}
	mw.rec.replyFunc("herdr agent get "+p.Pane, func() reply { return reply{out: agentGetOutput(p.fakeAgent)} })
	mw.panes = append(mw.panes, p)
	mw.byPane[p.Pane] = p
	mw.labels = append(mw.labels, [2]string{"w2", "ws2"})
	mw.rec.reply("herdr workspace list", workspaceListOutput(mw.labels))
}

// TestAutoSendSharedWorktree checks that a worktree two Herdr workspaces
// hold gets no automatic send, whichever is listed first, as a send the
// human asks for does not: when the round sees both, and when the second
// opens after the round read Herdr, which only the reading under the
// exclusion sees.
func TestAutoSendSharedWorktree(t *testing.T) {
	for _, underExclusion := range []bool{false, true} {
		t.Run(map[bool]string{false: "in the round", true: "under the exclusion"}[underExclusion], func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			mw.tick(time.Minute)
			mw.merge(nextBranch, "`/mkarch 0001`")
			if underExclusion {
				// The second opens after the round read Herdr, while the
				// first reading of the merge runs.
				calls := 0
				mw.rec.replyFunc(prStateLine, func() reply {
					calls++
					if calls == 1 {
						mw.mu.Lock()
						mw.addSharingWorkspace("idle")
						mw.mu.Unlock()
					}
					return reply{out: []byte(prMergedJSON)}
				})
			} else {
				mw.addSharingWorkspace("working")
				// With no one workspace there, the state is read in chief's
				// worktree, as for a workspace that is gone.
				mw.rec.replyIn(mw.chiefTree, prStateLine, []byte(prMergedJSON))
			}
			res := only(t, mw.tick(time.Minute))
			if sentCommand(res) || len(mw.rec.prompts()) != 0 {
				t.Fatalf("sent to a worktree two workspaces hold: %+v, prompts %v", res, mw.rec.prompts())
			}
			if sent, _ := mw.store.AutoSent(42); sent {
				t.Error("the automatic send was recorded")
			}
			if !underExclusion && !slices.ContainsFunc(res.Reasons, func(r string) bool { return strings.Contains(r, "2 Herdr workspaces") }) {
				t.Errorf("reasons = %v; want the two workspaces named", res.Reasons)
			}
		})
	}
}

// TestResetChecksAgain checks that the /runpr pane is not emptied when,
// while the command was typed, the pane moved (its last message still the
// report) or chief's session changed.
func TestResetChecksAgain(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(mw *mergeWorld)
	}{
		{"the /runpr pane moved", func(mw *mergeWorld) { mw.byPane["w1:p2"].Seq++ }},
		{"chief's session changed", func(mw *mergeWorld) {
			cleared := mw.chief
			cleared.Session = "00000000-0000-4000-8000-0000000000c9"
			mw.chief = cleared
			mw.rec.reply("herdr agent get "+worldChiefPane, agentGetOutput(cleared))
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			mw := newMergeWorld(t, mkarchDocs)
			mw.tick(time.Minute)
			mw.merge(nextBranch, "`/mkarch 0001`")
			mw.react = func(p *simPane, text string) error {
				if text == "/mkarch 0001" {
					c.change(mw)
				}
				return mw.realReact(p, text)
			}
			res := only(t, mw.tick(time.Minute))
			if !sentCommand(res) {
				t.Fatalf("auto send = %+v; want the command sent", res)
			}
			if len(res.Actions) != 2 || res.Actions[1].Input != InputReset || res.Actions[1].Outcome != OutcomeNotSent {
				t.Errorf("reset = %+v; want none", res.Actions[1:])
			}
			if slices.Contains(mw.rec.prompts(), "w1:p2 /new") {
				t.Error("the /runpr pane was emptied")
			}
		})
	}
}
