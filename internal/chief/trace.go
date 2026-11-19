package chief

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/privatefile"
	"github.com/isseis/yt2column/internal/strictjson"
)

// The history records of a worktree. A chief session keeps, in each
// worktree's directory of records, one record per event it saw of the
// worktree's PRs and branches, written only when what a round read differs
// from what the session's events already say. The history of a worktree on
// main is the only source of its task besides a branch name, so a record is
// read only in its exact form (the requirements' 4.2): it can be written by
// any process of the same user.

// tracePrefix starts a history record's name.
const tracePrefix = "trace-"

// traceFileName is a history record's name: the hash of its session, the ID
// of its round (as roundFileName's, so names sort by time), and the hash of
// the event (its kind, PR, and task).
var traceFileName = regexp.MustCompile(`^trace-([0-9a-f]{32})-([0-9]{20}-[0-9a-f]{16})-([0-9a-f]{32})\.json$`)

// traceKind is the kind of one event of a worktree's history. The zero value
// is traceUnknown, which no reader accepts.
type traceKind int

const (
	traceUnknown traceKind = iota
	// tracePROpen: the round saw the PR open on the worktree's branch, which
	// is not main. Branch is that branch; Task is its task, or zero when the
	// branch is no feature branch or its task directory on origin/main
	// contradicts it.
	tracePROpen
	// tracePRMerged: the round read the PR's state as MERGED. PR only.
	tracePRMerged
	// tracePRClosed: the round read the PR's state as CLOSED. PR only.
	tracePRClosed
	// traceBranchTask: the round saw the worktree on a feature branch of
	// Task. Task only.
	traceBranchTask
)

// String names the kind as a record holds it.
func (k traceKind) String() string {
	switch k {
	case tracePROpen:
		return "pr_open"
	case tracePRMerged:
		return "pr_merged"
	case tracePRClosed:
		return "pr_closed"
	case traceBranchTask:
		return "branch_task"
	default:
		return unknownName
	}
}

// traceKinds are the kinds by the names a record holds.
var traceKinds = map[string]traceKind{
	tracePROpen.String():     tracePROpen,
	tracePRMerged.String():   tracePRMerged,
	tracePRClosed.String():   tracePRClosed,
	traceBranchTask.String(): traceBranchTask,
}

// traceEvent is one record of a worktree's history in one chief session.
type traceEvent struct {
	Session  string
	Round    string
	Worktree Worktree
	Kind     traceKind
	PR       int
	Task     paneinput.TaskID
	Branch   string
}

// traceFile is the JSON form of a history record. Every member is written,
// the ones a kind does not use as "" or 0.
type traceFile struct {
	Session  string `json:"session"`
	Round    string `json:"round"`
	Worktree string `json:"worktree"`
	Kind     string `json:"kind"`
	PR       int    `json:"pr"`
	Task     string `json:"task"`
	Branch   string `json:"branch"`
}

// traceKeys are the members of a history record, every one required.
var traceKeys = []string{"session", "round", memberWorktree, "kind", "pr", "task", "branch"}

// memberWorktree is the member of a history record that names its worktree.
const memberWorktree = "worktree"

// file returns the JSON form of e.
func (e traceEvent) file() traceFile {
	return traceFile{Session: e.Session, Round: e.Round, Worktree: e.Worktree.path, Kind: e.Kind.String(), PR: e.PR, Task: e.Task.String(), Branch: e.Branch}
}

// name returns the file name of e. No part of the event's hash holds a NUL
// byte, so two events never give one hash.
func (e traceEvent) name() string {
	return tracePrefix + hashName(e.Session) + "-" + e.Round + "-" + hashName(e.Kind.String()+"\x00"+strconv.Itoa(e.PR)+"\x00"+e.Task.String()) + recordSuffix
}

// check checks the values of e: a kind chief knows, every member its kind
// uses set and every other one empty, and a pr_open's task the one its branch
// names. Contract: the writer and the reader run the same check, so nothing
// is written that the reader would refuse. The errors name the broken rule
// only, never a value, since a record's values are untrusted.
func (e traceEvent) check() error {
	if e.Session == "" {
		return fmt.Errorf("%w: a history record without a session", errRecord)
	}
	if !utf8.ValidString(e.Session) || !utf8.ValidString(e.Worktree.path) || !utf8.ValidString(e.Branch) {
		// json.Marshal would write U+FFFD in place of the invalid bytes, so
		// the record read back would not be the event written.
		return fmt.Errorf("%w: a history record with invalid UTF-8", errRecord)
	}
	if !sendIDForm.MatchString(e.Round) {
		// The round is part of the record's name, so a writer must never
		// take one of another form.
		return fmt.Errorf("%w: a history record's round is not a round ID", errRecord)
	}
	var prOK, taskOK, branchOK bool
	switch e.Kind {
	case tracePROpen:
		prOK, branchOK = e.PR > 0, e.Branch != ""
		if e.Task.IsZero() {
			taskOK = true
		} else {
			task, ok := taskOfBranch(e.Branch)
			taskOK = ok && task == e.Task
		}
	case tracePRMerged, tracePRClosed:
		prOK, taskOK, branchOK = e.PR > 0, e.Task.IsZero(), e.Branch == ""
	case traceBranchTask:
		prOK, taskOK, branchOK = e.PR == 0, !e.Task.IsZero(), e.Branch == ""
	case traceUnknown:
		return fmt.Errorf("%w: a history record of no kind", errRecord)
	default:
		return fmt.Errorf("%w: a history record of no known kind", errRecord)
	}
	switch {
	case !prOK:
		return fmt.Errorf("%w: a %s record's PR is not of its kind", errRecord, e.Kind)
	case !taskOK:
		return fmt.Errorf("%w: a %s record's task is not of its kind", errRecord, e.Kind)
	case !branchOK:
		return fmt.Errorf("%w: a %s record's branch is not of its kind", errRecord, e.Kind)
	}
	return nil
}

// parseTraceFile reads one history record. Contract: the data is one JSON
// object with exactly the record's members, each once and of its type, and
// nothing after it (strictjson, since encoding/json passes invalid UTF-8,
// unpaired surrogates, and a repeated member); a task is four ASCII digits
// or empty, never trimmed; the values pass traceEvent.check; anything else
// is errRecord, never read in part or repaired.
func parseTraceFile(data []byte) (traceEvent, error) {
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return traceEvent{}, fmt.Errorf("%w: %w", errRecord, err)
	}
	members, err := object.CollectOnly(traceKeys...)
	if err != nil {
		return traceEvent{}, fmt.Errorf("%w: %w", errRecord, err)
	}
	strs := map[string]string{}
	for _, key := range traceKeys {
		if key == "pr" {
			continue
		}
		text, err := strictjson.RequiredStringAllowEmpty(members, key)
		if err != nil {
			return traceEvent{}, fmt.Errorf("%w: %w", errRecord, err)
		}
		strs[key] = text
	}
	prValue, err := strictjson.Required(members, "pr")
	if err != nil {
		return traceEvent{}, fmt.Errorf("%w: %w", errRecord, err)
	}
	pr, err := prValue.AsNonNegativeInt64()
	if err != nil {
		return traceEvent{}, fmt.Errorf("%w: a history record's PR is not a PR number", errRecord)
	}
	kind, ok := traceKinds[strs["kind"]]
	if !ok {
		return traceEvent{}, fmt.Errorf("%w: a history record of no known kind", errRecord)
	}
	var task paneinput.TaskID
	if strs["task"] != "" {
		if task, err = paneinput.ParseTaskID(strs["task"]); err != nil {
			return traceEvent{}, fmt.Errorf("%w: a history record's task is not four digits", errRecord)
		}
	}
	e := traceEvent{Session: strs["session"], Round: strs["round"], Worktree: Worktree{path: strs[memberWorktree]}, Kind: kind, PR: int(pr), Task: task, Branch: strs["branch"]}
	if err := e.check(); err != nil {
		return traceEvent{}, err
	}
	return e, nil
}

// traceUnreadable is a history record, or a directory of records, that
// cannot be read: its path and why.
type traceUnreadable struct {
	Path string
	Err  error
}

// traceHistory is one chief session's history records of every worktree:
// the events of each worktree, and every record or directory that cannot be
// read. Only Store.readTraces makes a non-empty one.
type traceHistory struct {
	events     map[Worktree][]traceEvent
	unreadable []traceUnreadable
}

// readTraces reads session's history records from the directory of every
// worktree in the state directory, creating nothing. Contract: a record of
// another session is not read; a record that cannot be read, a name that
// starts with tracePrefix but is of no record's form (whose session cannot be
// told, so it counts as this session's), and a worktree directory that
// cannot be listed are returned in unreadable, never skipped, since the
// events they hide could name another task.
//
// A record's path and error go into the row's reason through
// mainTaskResult.Detail, so a path is quoted: a name of no record's form is
// chosen by whatever process made it.
func (s Store) readTraces(session string) traceHistory {
	h := traceHistory{events: map[Worktree][]traceEvent{}}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		h.unreadable = append(h.unreadable, traceUnreadable{Path: s.dir, Err: err})
		return h
	}
	for _, e := range entries {
		if !workspaceDirName.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(s.dir, e.Name())
		if e.Type()&os.ModeType != os.ModeDir {
			// A link is not followed, as existingRecords does not follow one.
			h.unreadable = append(h.unreadable, traceUnreadable{Path: dir, Err: fmt.Errorf("%w: not a directory", errRecord)})
			continue
		}
		events, bad, err := WorkspaceRecords{dir: dir}.traces(session)
		if err != nil {
			h.unreadable = append(h.unreadable, traceUnreadable{Path: dir, Err: err})
			continue
		}
		h.unreadable = append(h.unreadable, bad...)
		if len(events) > 0 {
			h.events[events[0].Worktree] = events
		}
	}
	return h
}

// traces reads session's history records of this worktree's directory, in
// the order they are applied (sortTrace). Contract: each record's session,
// round, and event must be the ones its name gives (the round's form, the
// kind's members, and the task are checked as the writer checks them), and
// its worktree the one whose directory holds it; a pr_merged or pr_closed of
// a PR with no pr_open before it, in the order the events are applied, is
// unreadable, since a merge of a PR not seen open first is no merge chief
// read (the writer writes a merge only after the open's round); err is a
// failure to list the directory.
func (r WorkspaceRecords) traces(session string) (events []traceEvent, bad []traceUnreadable, err error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, nil, err
	}
	own := tracePrefix + hashName(session) + "-"
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, tracePrefix) {
			continue
		}
		path := filepath.Join(r.dir, name)
		if !traceFileName.MatchString(name) {
			bad = append(bad, traceUnreadable{Path: path, Err: fmt.Errorf("%w: a name of no history record's form", errRecord)})
			continue
		}
		if !strings.HasPrefix(name, own) {
			continue
		}
		data, err := privatefile.Read(path, recordLimit)
		switch {
		case errors.Is(err, privatefile.ErrNotExist):
			// Removed under the exclusion since it was listed.
			continue
		case err != nil:
			bad = append(bad, traceUnreadable{Path: path, Err: fmt.Errorf("%w: %w", errRecord, err)})
			continue
		}
		ev, err := parseTraceFile(data)
		switch {
		case err != nil:
		case ev.name() != name:
			// The name holds the hash of the session, so this also refuses a
			// record of another session under this session's name.
			err = fmt.Errorf("%w: the record names another session, round, or event than its name", errRecord)
		case workspaceDirOf(ev.Worktree) != filepath.Base(r.dir):
			err = fmt.Errorf("%w: the record names another worktree than its directory's", errRecord)
		}
		if err != nil {
			bad = append(bad, traceUnreadable{Path: path, Err: err})
			continue
		}
		events = append(events, ev)
	}
	sortTrace(events)
	opened := map[int]struct{}{}
	kept := events[:0]
	for _, ev := range events {
		switch ev.Kind {
		case tracePROpen:
			opened[ev.PR] = struct{}{}
		case tracePRMerged, tracePRClosed:
			if _, ok := opened[ev.PR]; !ok {
				bad = append(bad, traceUnreadable{Path: filepath.Join(r.dir, ev.name()), Err: fmt.Errorf("%w: a %s record of a PR not seen open before it", errRecord, ev.Kind)})
				continue
			}
		}
		kept = append(kept, ev)
	}
	return kept, bad, nil
}

// traceRank orders the events of one round: a merge or a close comes before
// the branch and the opens the same round saw. Contract: a feature branch of
// another task seen in the round a merge was read counts as after the merge,
// the side that does not identify (the design, 3.2); the order never comes
// from the file names, whose event hashes have no meaning.
func traceRank(k traceKind) int {
	const (
		rankEnd = iota
		rankBranch
		rankOpen
	)
	switch k {
	case tracePRMerged, tracePRClosed:
		return rankEnd
	case traceBranchTask:
		return rankBranch
	default:
		return rankOpen
	}
}

// sortTrace sorts events in the order they are applied: by round, then by
// traceRank, then by PR and task so the order is total.
func sortTrace(events []traceEvent) {
	slices.SortStableFunc(events, func(a, b traceEvent) int {
		return cmp.Or(
			strings.Compare(a.Round, b.Round),
			cmp.Compare(traceRank(a.Kind), traceRank(b.Kind)),
			cmp.Compare(a.PR, b.PR),
			strings.Compare(a.Task.String(), b.Task.String()),
			cmp.Compare(a.Kind, b.Kind),
		)
	})
}

// tracePRState is the last state a worktree's history gives a PR. The zero
// value is tracePRNone, a PR with no event.
type tracePRState int

const (
	tracePRNone tracePRState = iota
	tracePRIsOpen
	tracePRIsMerged
	tracePRIsClosed
)

// traceState is the events of one worktree applied in order: each PR's last
// state, and the task of the last branch_task.
type traceState struct {
	events []traceEvent
	prs    map[int]tracePRState
	// branchTask is the task of the last branch_task, zero when there is
	// none.
	branchTask paneinput.TaskID
	// branchAfterEnd is false when a pr_merged or pr_closed has no
	// branch_task after it (the last one), true otherwise.
	branchAfterEnd bool
}

// applyTrace applies the events of one worktree in the order of sortTrace.
func applyTrace(events []traceEvent) traceState {
	sorted := slices.Clone(events)
	sortTrace(sorted)
	st := traceState{events: sorted, prs: map[int]tracePRState{}, branchAfterEnd: true}
	for _, e := range sorted {
		switch e.Kind {
		case tracePROpen:
			st.prs[e.PR] = tracePRIsOpen
		case tracePRMerged:
			st.prs[e.PR], st.branchAfterEnd = tracePRIsMerged, false
		case tracePRClosed:
			st.prs[e.PR], st.branchAfterEnd = tracePRIsClosed, false
		case traceBranchTask:
			st.branchTask, st.branchAfterEnd = e.Task, true
		}
	}
	return st
}

// openPRs returns, in number order, the PRs whose last state is open.
func (st traceState) openPRs() []int {
	var open []int
	for pr, s := range st.prs {
		if s == tracePRIsOpen {
			open = append(open, pr)
		}
	}
	slices.Sort(open)
	return open
}

// mainTaskKind is what the history of a worktree on main gives. The zero
// value is mainTaskUnread, the reading that assumes least.
type mainTaskKind int

const (
	mainTaskUnread mainTaskKind = iota
	mainTaskUnreadable
	mainTaskNoMerge
	mainTaskOtherTask
	mainTaskOpenPR
	mainTaskIdentified
)

// mainTaskResult is the identification of a worktree on main. Only
// identifyMainTask makes one, so a row is never given a task that the
// rules of the identification did not give.
type mainTaskResult struct {
	Kind mainTaskKind
	// Task and Branch are set for mainTaskIdentified: the task and the branch
	// of the last PR of it whose merge this session read.
	Task   paneinput.TaskID
	Branch string
	// Detail names the PRs, tasks, or records behind the result, for the
	// row's reason.
	Detail string
}

// identifyMainTask identifies the task of the worktree w on main from the
// session's history h, by the first rule of the design's 3.2 that holds:
// a record of any worktree that cannot be read; no merge read; merged PRs of
// more than one task, or of no task; another task seen on the worktree (a PR
// of another task or of none, or a feature branch of another task at or
// after the last merge of the task's PR); a PR whose last state is open;
// otherwise the task. Contract: it decides from h alone and never guesses:
// whatever the rules cannot tell is not identified.
func identifyMainTask(h traceHistory, w Worktree) mainTaskResult {
	if len(h.unreadable) > 0 {
		parts := make([]string, 0, len(h.unreadable))
		for _, u := range h.unreadable {
			parts = append(parts, strconv.Quote(u.Path)+": "+u.Err.Error())
		}
		return mainTaskResult{Kind: mainTaskUnreadable, Detail: strings.Join(parts, "; ")}
	}
	st := applyTrace(h.events[w])
	merged := map[int]struct{}{}
	lastMerge, lastMergePos := 0, -1
	for i, e := range st.events {
		if e.Kind == tracePRMerged {
			merged[e.PR] = struct{}{}
			lastMerge, lastMergePos = e.PR, i
		}
	}
	if len(merged) == 0 {
		return mainTaskResult{Kind: mainTaskNoMerge}
	}
	task, branch, mergedOther := mergedTask(st, merged, lastMerge)
	if len(mergedOther) > 0 {
		return mainTaskResult{Kind: mainTaskOtherTask, Detail: strings.Join(mergedOther, ", ")}
	}
	if other := otherTasks(st, task, lastMergePos); len(other) > 0 {
		return mainTaskResult{Kind: mainTaskOtherTask, Detail: strings.Join(other, ", ")}
	}
	if open := st.openPRs(); len(open) > 0 {
		return mainTaskResult{Kind: mainTaskOpenPR, Detail: prList(open)}
	}
	return mainTaskResult{Kind: mainTaskIdentified, Task: task, Branch: branch}
}

// mainTaskReason returns why a worktree on main has no next command from
// its identification (the design, 4.2), empty for an identified task.
// Contract: every kind but mainTaskIdentified has a reason, the zero value
// included, so a worktree on main is never read as one without a task.
func mainTaskReason(r mainTaskResult) string {
	switch r.Kind {
	case mainTaskIdentified:
		return ""
	case mainTaskUnreadable:
		return "the worktree is on main and a history record of this chief session cannot be read (start a new chief session with /clear): " + r.Detail
	case mainTaskNoMerge:
		return "the worktree is on main and this chief session has seen no merged PR of it, so its task is not known"
	case mainTaskOtherTask:
		return "the worktree is on main and this chief session has seen it with more than one task or with a PR of no task: " + r.Detail
	case mainTaskOpenPR:
		return "the worktree is on main and a PR this chief session saw open on it is not known to be merged or closed: " + r.Detail
	default:
		return "the worktree is on main and its history was not read, so its task is not known"
	}
}

// applyMainTask fills the facts of a worktree on main from its
// identification: for mainTaskIdentified, the task and origin/main's
// documents read with Tasks.Read on the branch of the task's last merged
// PR, as a feature branch's are; for a history that cannot be read, the
// reason as a failed read, so the row needs a check before anything else
// decides it; for any other kind, the result alone, whose reason
// headProblem gives. Facts of any other HEAD are returned as they are.
// Contract: it is the only place an identification reaches a row's facts,
// so no other HEAD ever takes a task from the history.
func applyMainTask(ctx context.Context, tasks Tasks, f workspaceFacts, result mainTaskResult) workspaceFacts {
	if f.Head != headMain {
		return f
	}
	f.MainTask = result
	switch result.Kind {
	case mainTaskIdentified:
		f.Task = result.Task
		docs, err := tasks.Read(ctx, f.Workspace, result.Branch)
		if err != nil {
			f.Unreadable = append(f.Unreadable, "task documents on origin/main")
			f.DocsUnreadable = true
		} else {
			f.Docs = &docs
		}
	case mainTaskUnreadable:
		f.Unreadable = append(f.Unreadable, mainTaskReason(result))
	}
	return f
}

// mergedTask returns the task every pr_open of the merged PRs names and the
// branch of the last merged PR, lastMerge. When those opens name more than
// one task, or no task, it returns them as the other tasks instead.
func mergedTask(st traceState, merged map[int]struct{}, lastMerge int) (task paneinput.TaskID, branch string, other []string) {
	tasks := map[paneinput.TaskID]struct{}{}
	var opens []traceEvent
	for _, e := range st.events {
		if _, ok := merged[e.PR]; ok && e.Kind == tracePROpen {
			tasks[e.Task] = struct{}{}
			opens = append(opens, e)
			if e.PR == lastMerge {
				branch = e.Branch
			}
		}
	}
	_, noTask := tasks[paneinput.TaskID{}]
	if len(tasks) != 1 || noTask {
		return paneinput.TaskID{}, "", describeOpens(opens)
	}
	for t := range tasks {
		task = t
	}
	return task, branch, nil
}

// otherTasks returns what the worktree's history shows of a task other than
// task: a pr_open of another task or of none, and a branch_task of another
// task at or after the last merge of task's PR (lastMergePos).
func otherTasks(st traceState, task paneinput.TaskID, lastMergePos int) []string {
	var opens []traceEvent
	var other []string
	for i, e := range st.events {
		switch {
		case e.Kind == tracePROpen && e.Task != task:
			opens = append(opens, e)
		case e.Kind == traceBranchTask && e.Task != task && i > lastMergePos:
			other = append(other, "a branch of "+e.Task.String())
		}
	}
	return append(describeOpens(opens), other...)
}

// describeOpens names the PRs of opens with their tasks, each once.
func describeOpens(opens []traceEvent) []string {
	var out []string
	for _, e := range opens {
		text := "#" + strconv.Itoa(e.PR) + " of no task"
		if !e.Task.IsZero() {
			text = "#" + strconv.Itoa(e.PR) + " of " + e.Task.String()
		}
		if !slices.Contains(out, text) {
			out = append(out, text)
		}
	}
	return out
}

// prList names the PRs as "#1, #2".
func prList(prs []int) string {
	parts := make([]string, 0, len(prs))
	for _, pr := range prs {
		parts = append(parts, "#"+strconv.Itoa(pr))
	}
	return strings.Join(parts, ", ")
}

// tracePlace is where a round read a worktree's HEAD, as the write
// conditions take it. The zero value is tracePlaceUnread, from which nothing
// is written.
type tracePlace int

const (
	tracePlaceUnread tracePlace = iota
	// tracePlaceMain is the branch main, declared by the reader that
	// compares the branch's name; nothing here compares it.
	tracePlaceMain
	// tracePlaceBranch is a checked-out branch other than main.
	tracePlaceBranch
	// tracePlaceDetached is a detached HEAD.
	tracePlaceDetached
)

// traceReading is what one round read of one worktree, as the write
// conditions take it.
type traceReading struct {
	Place tracePlace
	// Branch is the branch for tracePlaceBranch.
	Branch string
	// BranchTask is the task the branch's name gives, zero when the branch
	// is no feature branch.
	BranchTask paneinput.TaskID
	// DocsRead is true when the round read the task directory of
	// BranchTask from origin/main without a contradiction.
	DocsRead bool
	// PR is the one open PR the round saw on the branch, 0 for none.
	PR int
}

// prStateRead is what reading the state of one PR gave in a round: its
// state, its head branch, and the failure, as PRStateOf returns them.
type prStateRead struct {
	State   PRState
	HeadRef string
	Err     error
}

// traceReadingOf is what the round read of a worktree, from its facts, as
// the write conditions take it. Contract: the place is the HEAD the reader
// declared (headMain is main, never a branch compared here); a branch's
// task is the one its name gives, and DocsRead is true only when the
// round read that task's documents from origin/main.
func traceReadingOf(f workspaceFacts) traceReading {
	r := traceReading{Branch: f.Branch}
	switch f.Head {
	case headMain:
		r.Place = tracePlaceMain
	case headBranch:
		r.Place = tracePlaceBranch
		r.BranchTask, r.DocsRead = f.Task, f.Docs != nil && !f.DocsUnreadable
	case headDetached:
		r.Place = tracePlaceDetached
	default:
		r.Place = tracePlaceUnread
	}
	if f.PR != nil {
		r.PR = f.PR.Number
	}
	return r
}

// writeHistory writes this round's events of every worktree of view to
// owner's history (the design, 3.3), after reading the state of each PR
// open in the history that the round did not see open on its worktree's
// branch. It returns the PRs whose state could not be read and the round's
// problems. Contract: a PR's state is read at most once per round (the
// round's table, view.prStates, which the automatic send's first reading
// shares); an event is written only under the worktree's exclusion, and
// only when the worktree's branch read again under it is the one the round
// read and, for a merge or a close, the PR's open record is still there;
// whatever is not written is named in the problems, never retried in the
// round.
func writeHistory(ctx context.Context, deps Deps, view *roundView, owner Owner, round string) (unread map[int]struct{}, problems []string) {
	git := NewGitReader(deps.Cmds)
	history := deps.Store.readTraces(owner.Session)
	unread = map[int]struct{}{}
	for i, w := range view.found {
		f := view.facts[i]
		reading := traceReadingOf(f)
		st := applyTrace(history.events[w.Tree])
		for _, pr := range st.unseenOpenPRs(reading) {
			if read := view.prState(ctx, git, w.Worktree, pr); read.Err != nil {
				unread[pr] = struct{}{}
				problems = append(problems, "the state of #"+strconv.Itoa(pr)+" cannot be read for the history of "+w.Worktree+": "+read.Err.Error())
			}
		}
		events := traceWrites(st, reading, view.prStates, owner.Session, round, w.Tree)
		if len(events) == 0 {
			continue
		}
		if err := writeEvents(ctx, deps.Store, git, w, f.Branch, owner.Session, events); err != nil {
			problems = append(problems, "the history record of "+w.Worktree+" cannot be written: "+err.Error())
		}
	}
	return unread, problems
}

// errHistoryChanged reports a worktree whose branch or history changed
// between the round's reading and its exclusion.
var errHistoryChanged = errors.New("the worktree changed since the round read it")

// writeEvents writes events into w's history under its exclusion, after
// reading w's branch and session's history again under it. Contract:
// nothing is written unless the branch is still branch (empty for a
// detached HEAD) and every merge or close has its PR's open record; a
// failure to write one event stops the rest.
func writeEvents(ctx context.Context, store Store, git GitReader, w TaskWorkspace, branch, session string, events []traceEvent) error {
	records, err := store.Records(w.Tree)
	if err != nil {
		return err
	}
	held, err := records.Lock()
	if err != nil {
		return fmt.Errorf("its exclusion cannot be taken: %w", err)
	}
	defer func() { _ = held.Close() }()
	now, err := git.Branch(ctx, w.Worktree)
	switch {
	case err != nil:
		return fmt.Errorf("its branch cannot be read again: %w", err)
	case now != branch:
		return fmt.Errorf("%w: its branch is no longer the one the round read", errHistoryChanged)
	}
	kept, _, err := held.traces(session)
	if err != nil {
		return fmt.Errorf("its history cannot be read again: %w", err)
	}
	opened := map[int]struct{}{}
	for _, e := range kept {
		if e.Kind == tracePROpen {
			opened[e.PR] = struct{}{}
		}
	}
	for _, e := range events {
		if _, ok := opened[e.PR]; (e.Kind == tracePRMerged || e.Kind == tracePRClosed) && !ok {
			return fmt.Errorf("%w: the open record of #%d is gone", errHistoryChanged, e.PR)
		}
	}
	for _, e := range events {
		if err := held.writeTrace(e); err != nil {
			return err
		}
	}
	return nil
}

// unseenOpenPRs returns, in number order, the PRs whose last state is open
// and that the round did not see open on the worktree's branch: the PRs
// whose state the round reads for the history. A HEAD that was not read sees
// nothing and asks for nothing, since whether a PR is still on the branch
// cannot be told.
func (st traceState) unseenOpenPRs(r traceReading) []int {
	if r.Place == tracePlaceUnread {
		return nil
	}
	return slices.DeleteFunc(st.openPRs(), func(pr int) bool { return r.Place == tracePlaceBranch && pr == r.PR })
}

// traceWrites returns the events a round writes for the worktree w, whose
// history is st, from what the round read (r) and the states it read of the
// PRs (states, by number), by the write conditions of the design's 3.1.
// Contract: it writes only a change: a merge or a close of a PR open in the
// history and not seen open, only when its state was read as such; a pr_open
// of the one open PR of a branch other than main whose last state is not
// open; a branch_task of a feature branch whose task is not the last one,
// in a round that writes a merge or a close, or when no branch_task follows
// the last merge or close (a worktree that comes back to a feature branch
// after a merge written off it), so the identification sees the branch after
// the merge; nothing for a HEAD not read, a
// state not read, or a PR still open. It reads and writes nothing itself.
func traceWrites(st traceState, r traceReading, states map[int]prStateRead, session, round string, w Worktree) []traceEvent {
	at := func(kind traceKind, pr int, task paneinput.TaskID, branch string) traceEvent {
		return traceEvent{Session: session, Round: round, Worktree: w, Kind: kind, PR: pr, Task: task, Branch: branch}
	}
	var out []traceEvent
	for _, pr := range st.unseenOpenPRs(r) {
		read, ok := states[pr]
		if !ok || read.Err != nil {
			continue
		}
		switch read.State {
		case PRStateMerged:
			out = append(out, at(tracePRMerged, pr, paneinput.TaskID{}, ""))
		case PRStateClosed:
			out = append(out, at(tracePRClosed, pr, paneinput.TaskID{}, ""))
		}
	}
	if r.Place != tracePlaceBranch {
		return out
	}
	if !r.BranchTask.IsZero() && (len(out) > 0 || r.BranchTask != st.branchTask || !st.branchAfterEnd) {
		out = append(out, at(traceBranchTask, 0, r.BranchTask, ""))
	}
	if r.PR > 0 && st.prs[r.PR] != tracePRIsOpen {
		task := paneinput.TaskID{}
		if r.DocsRead {
			task = r.BranchTask
		}
		out = append(out, at(tracePROpen, r.PR, task, r.Branch))
	}
	return out
}
