//go:build test && unix

package chief

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/privatefile"
)

// Feature branches of three tasks, and the worktree the history tests use.
const (
	branchT1 = "user/0101-first-task-01"
	branchT2 = "user/0102-second-task-03"
	branchT3 = "user/0103-third-task-01"
)

var traceTree = Worktree{path: "/work/a"}

// traceRound is the ID of the n-th round of a test session; IDs sort as n
// does.
func traceRound(n int) string { return fmt.Sprintf("%020d-%016x", n, n) }

// ev is an event of ownerA's session in traceTree, in round n.
func ev(n int, kind traceKind, pr int, task paneinput.TaskID, branch string) traceEvent {
	return traceEvent{Session: ownerA().Session, Round: traceRound(n), Worktree: traceTree, Kind: kind, PR: pr, Task: task, Branch: branch}
}

// putTrace writes data under name into the directory of w's records.
func putTrace(t *testing.T, store Store, w Worktree, name string, data []byte) string {
	t.Helper()
	records, err := store.Records(w)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(records.dir, name)
	if _, err := privatefile.Create(path, data); err != nil {
		t.Fatal(err)
	}
	return path
}

// traceJSON is the JSON of e as the writer writes it.
func traceJSON(t *testing.T, e traceEvent) []byte {
	t.Helper()
	data, err := json.Marshal(e.file())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestReadTraceRejects writes one record in a form the reader must refuse
// next to a readable one, and checks that the reader returns it as an
// errRecord under its path and reads no event from it, while the readable
// record is still read. A record of exactly the size limit is read.
func TestReadTraceRejects(t *testing.T) {
	t.Parallel()
	t1, t2 := taskID(t, "0101"), taskID(t, "0102")
	open := ev(1, tracePROpen, 7, t1, branchT1)
	noTaskOpen := ev(1, tracePROpen, 7, paneinput.TaskID{}, "scratch")
	merged := ev(2, tracePRMerged, 7, paneinput.TaskID{}, "")
	// edit replaces the first occurrence of old in open's JSON.
	edit := func(old, repl string) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			text := string(traceJSON(t, open))
			if !strings.Contains(text, old) {
				t.Fatalf("%q is not in %s", old, text)
			}
			return []byte(strings.Replace(text, old, repl, 1))
		}
	}
	// own is a case of e's JSON under e's own name, so only the check of
	// e's members can refuse it.
	own := func(e traceEvent) *traceEvent { return &e }
	none := paneinput.TaskID{}
	other := open
	other.Worktree = Worktree{path: "/work/b"}
	cases := []struct {
		name string
		// fileName and data are the record's name and content: open's name
		// when fileName is "", and own's when event is set.
		fileName string
		data     func(*testing.T) []byte
		event    *traceEvent
	}{
		{name: "a task with a path", data: edit(`"task":"0101"`, `"task":"../x"`)},
		{name: "a task with a trailing space", data: edit(`"task":"0101"`, `"task":"0101 "`)},
		{name: "invalid UTF-8", data: edit(`"branch":"`, "\"branch\":\"\xff")},
		{name: "an unpaired surrogate", data: edit(`"branch":"`, `"branch":"\ud800`)},
		{name: "a repeated member", data: edit(`"pr":7`, `"pr":7,"pr":7`)},
		{name: "an unknown member", data: edit(`"pr":7`, `"pr":7,"note":""`)},
		{name: "a member named in another case", data: edit(`"pr":7`, `"pr":7,"PR":7`)},
		{name: "a member of another type", data: edit(`"pr":7`, `"pr":"7"`)},
		// The missing member's empty value would be valid for the kind and
		// is not in the name, so only the check that it is present refuses it.
		{name: "a missing member", fileName: noTaskOpen.name(), data: func(t *testing.T) []byte {
			return []byte(strings.Replace(string(traceJSON(t, noTaskOpen)), `,"task":""`, ``, 1))
		}},
		{name: "a pr_open without its PR", event: own(ev(1, tracePROpen, 0, t1, branchT1))},
		{name: "a pr_open without its branch", event: own(ev(1, tracePROpen, 7, none, ""))},
		{name: "a pr_open whose task is not its branch's", event: own(ev(1, tracePROpen, 7, t2, branchT1))},
		{name: "a pr_open with a task on a branch of none", event: own(ev(1, tracePROpen, 7, t1, "scratch"))},
		{name: "a pr_merged with a task", event: own(ev(1, tracePRMerged, 7, t1, ""))},
		{name: "a pr_closed with a branch", event: own(ev(1, tracePRClosed, 7, none, branchT1))},
		{name: "a branch_task with a PR", event: own(ev(1, traceBranchTask, 7, t1, ""))},
		{name: "a branch_task without its task", event: own(ev(1, traceBranchTask, 0, none, ""))},
		{name: "a branch_task with a branch", event: own(ev(1, traceBranchTask, 0, t1, branchT1))},
		{name: "a session other than the name's", data: edit(ownerA().Session, ownerB().Session)},
		{name: "a round other than the name's", data: edit(traceRound(1), traceRound(3))},
		{name: "an event other than the name's", data: edit(`"pr":7`, `"pr":8`)},
		{name: "a worktree other than the directory's", event: own(other)},
		{name: "a name of no record's form", fileName: "trace-backup.json", data: func(t *testing.T) []byte { return traceJSON(t, open) }},
		{name: "trailing data", data: func(t *testing.T) []byte { return append(traceJSON(t, open), "{}"...) }},
		{name: "over the size limit", data: func(t *testing.T) []byte {
			return traceJSON(t, ev(1, tracePROpen, 7, none, strings.Repeat("b", recordLimit)))
		}},
		// The rows below are refused end to end: the parse refuses them
		// first, and the members' check or the name would refuse them too,
		// so no single check of the reader is shown by them alone. The
		// writer test shows the members' check of the round's form and of
		// a merge's PR, where the name and the open are not compared.
		{name: "a negative PR", data: edit(`"pr":7`, `"pr":-7`)},
		{name: "an unknown kind", data: edit(`"kind":"pr_open"`, `"kind":"pr_seen"`)},
		{name: "a pr_merged without its PR", event: own(ev(1, tracePRMerged, 0, none, ""))},
		{name: "a round that is no round ID", fileName: "trace-" + hashName(ownerA().Session) + "-" + traceRound(1) + "-" + strings.Repeat("0", 32) + ".json", data: func(t *testing.T) []byte {
			e := open
			e.Round = "1"
			return traceJSON(t, e)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := newStore(t)
			name, data := c.fileName, c.data
			switch {
			case c.event != nil:
				e := *c.event
				name, data = e.name(), func(t *testing.T) []byte { return traceJSON(t, e) }
			case name == "":
				name = open.name()
			}
			path := putTrace(t, store, traceTree, name, data(t))
			// A readable open of the same PR in round 0, before every case's
			// round, which the reader keeps reading; it is also the open a
			// pr_merged or pr_closed case needs first, so the order check
			// passes it and only the case's own check refuses it.
			fine := ev(0, tracePROpen, 7, t1, branchT1)
			putTrace(t, store, traceTree, fine.name(), traceJSON(t, fine))
			h := store.readTraces(ownerA().Session)
			if len(h.unreadable) != 1 || h.unreadable[0].Path != path || !errors.Is(h.unreadable[0].Err, errRecord) {
				t.Fatalf("unreadable = %+v; want only %s as an errRecord", h.unreadable, path)
			}
			if got := h.events[traceTree]; !reflect.DeepEqual(got, []traceEvent{fine}) {
				t.Errorf("events = %+v; want only the readable record", got)
			}
		})
	}
	// The order check, for both kinds that end a PR: every record below is
	// of the reader's form and named as its own, so only the order the
	// events are applied in refuses the end.
	for _, kind := range []traceKind{tracePRMerged, tracePRClosed} {
		t.Run(fmt.Sprintf("a %s of a PR never seen open", kind), func(t *testing.T) {
			store := newStore(t)
			end := ev(2, kind, 7, none, "")
			path := putTrace(t, store, traceTree, end.name(), traceJSON(t, end))
			h := store.readTraces(ownerA().Session)
			if len(h.unreadable) != 1 || h.unreadable[0].Path != path || !errors.Is(h.unreadable[0].Err, errRecord) || len(h.events) != 0 {
				t.Fatalf("history = %+v; want the %s unreadable", h, kind)
			}
		})
		t.Run(fmt.Sprintf("a %s before the open of its PR", kind), func(t *testing.T) {
			store := newStore(t)
			early := ev(1, kind, 7, none, "")
			late := ev(2, tracePROpen, 7, t1, branchT1)
			path := putTrace(t, store, traceTree, early.name(), traceJSON(t, early))
			putTrace(t, store, traceTree, late.name(), traceJSON(t, late))
			h := store.readTraces(ownerA().Session)
			if len(h.unreadable) != 1 || h.unreadable[0].Path != path || !errors.Is(h.unreadable[0].Err, errRecord) || !reflect.DeepEqual(h.events[traceTree], []traceEvent{late}) {
				t.Fatalf("history = %+v; want the %s unreadable and the open read", h, kind)
			}
		})
	}
	t.Run("a record of exactly the size limit", func(t *testing.T) {
		store := newStore(t)
		e := ev(1, tracePROpen, 7, paneinput.TaskID{}, "")
		e.Branch = strings.Repeat("b", recordLimit-len(traceJSON(t, e)))
		data := traceJSON(t, e)
		if len(data) != recordLimit {
			t.Fatalf("the record is %d bytes, want %d", len(data), recordLimit)
		}
		putTrace(t, store, traceTree, e.name(), data)
		putTrace(t, store, traceTree, merged.name(), traceJSON(t, merged))
		h := store.readTraces(ownerA().Session)
		if len(h.unreadable) != 0 || !reflect.DeepEqual(h.events[traceTree], []traceEvent{e, merged}) {
			t.Errorf("%d unreadable; want both records read", len(h.unreadable))
		}
	})
}

// TestReadTraceSessionOnly checks that the reader reads the records of its
// own session in every worktree's directory and never another session's,
// and that a worktree directory it cannot list, or an entry of a worktree
// directory's name that is no directory, is unreadable.
func TestReadTraceSessionOnly(t *testing.T) {
	t.Parallel()
	t1 := taskID(t, "0101")
	store := newStore(t)
	mine := ev(1, tracePROpen, 7, t1, branchT1)
	putTrace(t, store, traceTree, mine.name(), traceJSON(t, mine))
	theirs := mine
	theirs.Session = ownerB().Session
	putTrace(t, store, traceTree, theirs.name(), traceJSON(t, theirs))
	elsewhere := mine
	elsewhere.Worktree = Worktree{path: "/work/b"}
	putTrace(t, store, elsewhere.Worktree, elsewhere.name(), traceJSON(t, elsewhere))
	h := store.readTraces(ownerA().Session)
	want := map[Worktree][]traceEvent{traceTree: {mine}, elsewhere.Worktree: {elsewhere}}
	if len(h.unreadable) != 0 || !reflect.DeepEqual(h.events, want) {
		t.Fatalf("history = %+v; want only this session's records of both worktrees", h)
	}
	if h := store.readTraces(ownerB().Session); len(h.unreadable) != 0 || !reflect.DeepEqual(h.events, map[Worktree][]traceEvent{traceTree: {theirs}}) {
		t.Errorf("the other session's history = %+v", h)
	}
	if os.Geteuid() == 0 {
		t.Skip("root lists a directory of mode 0")
	}
	locked := filepath.Join(store.Dir(), workspaceDirOf(Worktree{path: "/work/c"}))
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	// An entry of a worktree directory's name that is no directory cannot
	// be listed either.
	notDir := filepath.Join(store.Dir(), workspaceDirOf(Worktree{path: "/work/d"}))
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// A link to a readable directory of records is not followed.
	link := filepath.Join(store.Dir(), workspaceDirOf(Worktree{path: "/work/e"}))
	if err := os.Symlink(filepath.Join(store.Dir(), workspaceDirOf(traceTree)), link); err != nil {
		t.Fatal(err)
	}
	h = store.readTraces(ownerA().Session)
	var paths []string
	for _, u := range h.unreadable {
		paths = append(paths, u.Path)
	}
	slices.Sort(paths)
	wantPaths := []string{locked, notDir, link}
	slices.Sort(wantPaths)
	if !slices.Equal(paths, wantPaths) || !reflect.DeepEqual(h.events, want) {
		t.Errorf("history = %+v; want the three entries unreadable and the records still read", h)
	}
	if err := os.Chmod(store.Dir(), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(store.Dir(), 0o700) })
	h = store.readTraces(ownerA().Session)
	if len(h.unreadable) != 1 || h.unreadable[0].Path != store.Dir() || len(h.events) != 0 {
		t.Errorf("history of a state directory that cannot be listed = %+v; want it unreadable", h)
	}
}

// TestIdentifyMainTask applies each rule of the identification, in its
// order, to the events of a worktree on main.
func TestIdentifyMainTask(t *testing.T) {
	t.Parallel()
	t1, t2 := taskID(t, "0101"), taskID(t, "0102")
	none := paneinput.TaskID{}
	open := func(n, pr int, task paneinput.TaskID, branch string) traceEvent {
		return ev(n, tracePROpen, pr, task, branch)
	}
	merged := func(n, pr int) traceEvent { return ev(n, tracePRMerged, pr, none, "") }
	closed := func(n, pr int) traceEvent { return ev(n, tracePRClosed, pr, none, "") }
	branch := func(n int, task paneinput.TaskID) traceEvent { return ev(n, traceBranchTask, 0, task, "") }
	cases := []struct {
		name       string
		events     []traceEvent
		unreadable bool
		kind       mainTaskKind
		task       paneinput.TaskID
		branch     string
		detail     string
	}{
		{"no event", nil, false, mainTaskNoMerge, none, "", ""},
		{
			"a PR merged after it was seen open",
			[]traceEvent{branch(1, t1), open(1, 7, t1, branchT1), merged(2, 7)},
			false, mainTaskIdentified, t1, branchT1, "",
		},
		{
			"a record of some worktree cannot be read",
			[]traceEvent{branch(1, t1), open(1, 7, t1, branchT1), merged(2, 7)},
			true, mainTaskUnreadable, none, "", `"/x": ` + errRecord.Error(),
		},
		{
			"merged PRs of two tasks",
			[]traceEvent{open(1, 7, t1, branchT1), merged(2, 7), open(3, 8, t2, branchT2), merged(4, 8)},
			false, mainTaskOtherTask, none, "", "#7 of 0101, #8 of 0102",
		},
		{
			"a merged PR of no task",
			[]traceEvent{open(1, 7, none, "scratch"), merged(2, 7)},
			false, mainTaskOtherTask, none, "", "#7 of no task",
		},
		{
			"a feature branch of another task after the merge",
			[]traceEvent{branch(1, t1), open(1, 7, t1, branchT1), merged(2, 7), branch(3, t2)},
			false, mainTaskOtherTask, none, "", "a branch of 0102",
		},
		{
			"another task's branch, then the task's again, after the merge",
			[]traceEvent{branch(1, t1), open(1, 7, t1, branchT1), merged(2, 7), branch(3, t2), branch(4, t1)},
			false, mainTaskOtherTask, none, "", "a branch of 0102",
		},
		{
			"another task's branch before the task's PR was merged",
			[]traceEvent{branch(1, t2), branch(2, t1), open(2, 7, t1, branchT1), merged(3, 7)},
			false, mainTaskIdentified, t1, branchT1, "",
		},
		{
			"the merge read while on another task's branch",
			[]traceEvent{branch(1, t1), open(1, 7, t1, branchT1), branch(2, t2), branch(3, t2), merged(3, 7)},
			false, mainTaskOtherTask, none, "", "a branch of 0102",
		},
		{
			"the merge and another task's branch in one round",
			[]traceEvent{branch(3, t2), merged(3, 7), open(1, 7, t1, branchT1)},
			false, mainTaskOtherTask, none, "", "a branch of 0102",
		},
		{
			"a PR of another task seen open only",
			[]traceEvent{open(1, 7, t1, branchT1), merged(2, 7), open(3, 8, t2, branchT2)},
			false, mainTaskOtherTask, none, "", "#8 of 0102",
		},
		{
			"a PR of no task closed",
			[]traceEvent{open(1, 7, t1, branchT1), merged(2, 7), open(3, 8, none, "scratch"), closed(4, 8)},
			false, mainTaskOtherTask, none, "", "#8 of no task",
		},
		{"a PR seen open and not merged", []traceEvent{open(1, 7, t1, branchT1)}, false, mainTaskNoMerge, none, "", ""},
		{"a PR closed without a merge", []traceEvent{open(1, 7, t1, branchT1), closed(2, 7)}, false, mainTaskNoMerge, none, "", ""},
		{
			"a PR of the task still open next to a merged one",
			[]traceEvent{open(1, 7, t1, branchT1), merged(2, 7), open(3, 8, t1, branchT1)},
			false, mainTaskOpenPR, none, "", "#8",
		},
		{
			"a closed PR opened again",
			[]traceEvent{open(1, 7, t1, branchT1), merged(2, 7), open(3, 8, t1, branchT1), closed(4, 8), open(5, 8, t1, branchT1)},
			false, mainTaskOpenPR, none, "", "#8",
		},
		{
			"a closed PR opened again and merged",
			[]traceEvent{open(1, 8, t1, branchT1), closed(2, 8), open(3, 8, t1, branchT1), merged(4, 8)},
			false, mainTaskIdentified, t1, branchT1, "",
		},
		{
			"two merged PRs of the task: the last merge's branch",
			[]traceEvent{open(1, 8, t1, "user/0101-first-task-02"), open(1, 7, t1, branchT1), merged(2, 8), merged(3, 7)},
			false, mainTaskIdentified, t1, branchT1, "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := traceHistory{events: map[Worktree][]traceEvent{traceTree: c.events}}
			if c.unreadable {
				h.unreadable = []traceUnreadable{{Path: "/x", Err: errRecord}}
			}
			got := identifyMainTask(h, traceTree)
			if got.Kind != c.kind || got.Task != c.task || got.Branch != c.branch || got.Detail != c.detail {
				t.Errorf("identifyMainTask = %+v; want kind %d, task %q, branch %q, detail %q", got, c.kind, c.task, c.branch, c.detail)
			}
		})
	}
	t.Run("another worktree's events", func(t *testing.T) {
		h := traceHistory{events: map[Worktree][]traceEvent{{path: "/work/b"}: {open(1, 7, t1, branchT1), merged(2, 7)}}}
		if got := identifyMainTask(h, traceTree); got.Kind != mainTaskNoMerge {
			t.Errorf("identifyMainTask = %+v; want no merge for a worktree without events", got)
		}
	})
}

// TestTraceWrites checks the write conditions: each event is written for a
// change only, and nothing is written for a HEAD not read, a PR still open,
// or a state that was not read.
func TestTraceWrites(t *testing.T) {
	t.Parallel()
	t1, t2 := taskID(t, "0101"), taskID(t, "0102")
	none := paneinput.TaskID{}
	openT1 := []traceEvent{ev(1, traceBranchTask, 0, t1, ""), ev(1, tracePROpen, 7, t1, branchT1)}
	onT1 := traceReading{Place: tracePlaceBranch, Branch: branchT1, BranchTask: t1, DocsRead: true}
	onMain := traceReading{Place: tracePlaceMain}
	read := func(s PRState) map[int]prStateRead { return map[int]prStateRead{7: {State: s}} }
	withPR := func(r traceReading, pr int) traceReading { r.PR = pr; return r }
	const now = 5
	cases := []struct {
		name    string
		history []traceEvent
		reading traceReading
		states  map[int]prStateRead
		want    []traceEvent
	}{
		{
			"a feature branch with a PR, first seen", nil, withPR(onT1, 7), nil,
			[]traceEvent{ev(now, traceBranchTask, 0, t1, ""), ev(now, tracePROpen, 7, t1, branchT1)},
		},
		{
			"the task directory not read", nil,
			traceReading{Place: tracePlaceBranch, Branch: branchT1, BranchTask: t1, PR: 7},
			nil,
			[]traceEvent{ev(now, traceBranchTask, 0, t1, ""), ev(now, tracePROpen, 7, none, branchT1)},
		},
		{
			"a branch of no task with a PR", nil,
			traceReading{Place: tracePlaceBranch, Branch: "scratch", PR: 7},
			nil,
			[]traceEvent{ev(now, tracePROpen, 7, none, "scratch")},
		},
		{"nothing changed", openT1, withPR(onT1, 7), read(PRStateMerged), nil},
		{"merged, read on main", openT1, onMain, read(PRStateMerged), []traceEvent{ev(now, tracePRMerged, 7, none, "")}},
		{"closed, read on main", openT1, onMain, read(PRStateClosed), []traceEvent{ev(now, tracePRClosed, 7, none, "")}},
		{"merged, read on a detached HEAD", openT1, traceReading{Place: tracePlaceDetached}, read(PRStateMerged), []traceEvent{ev(now, tracePRMerged, 7, none, "")}},
		{"still open", openT1, onMain, read(PRStateOpen), nil},
		{"the state not read", openT1, onMain, map[int]prStateRead{7: {State: PRStateMerged, Err: errors.New("gh failed")}}, nil},
		{"the state not asked", openT1, onMain, nil, nil},
		{"the HEAD not read", openT1, traceReading{}, read(PRStateMerged), nil},
		{
			"merged, read on the task's branch", openT1, onT1, read(PRStateMerged),
			[]traceEvent{ev(now, tracePRMerged, 7, none, ""), ev(now, traceBranchTask, 0, t1, "")},
		},
		{
			"merged, read on another task's branch", openT1,
			traceReading{Place: tracePlaceBranch, Branch: branchT2, BranchTask: t2, DocsRead: true},
			read(PRStateMerged),
			[]traceEvent{ev(now, tracePRMerged, 7, none, ""), ev(now, traceBranchTask, 0, t2, "")},
		},
		{
			"another task's branch again after a merge read off it",
			append(slices.Clone(openT1), ev(2, traceBranchTask, 0, t2, ""), ev(3, tracePRMerged, 7, none, "")),
			traceReading{Place: tracePlaceBranch, Branch: branchT2, BranchTask: t2, DocsRead: true},
			nil,
			[]traceEvent{ev(now, traceBranchTask, 0, t2, "")},
		},
		{
			"the same branch after the merge, written once",
			append(slices.Clone(openT1), ev(3, tracePRMerged, 7, none, ""), ev(4, traceBranchTask, 0, t2, "")),
			traceReading{Place: tracePlaceBranch, Branch: branchT2, BranchTask: t2, DocsRead: true},
			nil, nil,
		},
		{
			"another task's branch", openT1[:1],
			traceReading{Place: tracePlaceBranch, Branch: branchT2, BranchTask: t2, DocsRead: true},
			nil,
			[]traceEvent{ev(now, traceBranchTask, 0, t2, "")},
		},
		{
			"a closed PR open again", append(slices.Clone(openT1), ev(2, tracePRClosed, 7, none, "")), withPR(onT1, 7), nil,
			// The branch is written again: no branch_task follows the close.
			[]traceEvent{ev(now, traceBranchTask, 0, t1, ""), ev(now, tracePROpen, 7, t1, branchT1)},
		},
		{"a PR seen on main is still read", openT1, withPR(onMain, 7), read(PRStateMerged), []traceEvent{ev(now, tracePRMerged, 7, none, "")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := traceWrites(applyTrace(c.history), c.reading, c.states, ownerA().Session, traceRound(now), traceTree)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("traceWrites = %+v\nwant %+v", got, c.want)
			}
		})
	}
}

// TestWriteTraceUnderTheExclusion checks that an event written under the
// worktree's exclusion is read back, that a record of the same name is not
// replaced, and that an event the reader would refuse or of another
// worktree is not written.
func TestWriteTraceUnderTheExclusion(t *testing.T) {
	t.Parallel()
	t1 := taskID(t, "0101")
	store := newStore(t)
	held := heldRecords(t, store, traceTree.path)
	first := ev(1, tracePROpen, 7, paneinput.TaskID{}, "scratch")
	if err := held.writeTrace(first); err != nil {
		t.Fatal(err)
	}
	same := first
	same.Branch = "another"
	if err := held.writeTrace(same); err != nil {
		t.Errorf("writing the same event again = %v, want no failure", err)
	}
	refused := ev(1, tracePROpen, 8, t1, "scratch")
	if err := held.writeTrace(refused); !errors.Is(err, errRecord) {
		t.Errorf("writing an event the reader refuses = %v, want errRecord", err)
	}
	badRound := ev(1, tracePROpen, 8, t1, branchT1)
	badRound.Round = "../r"
	noSession := ev(1, tracePROpen, 8, t1, branchT1)
	noSession.Session = ""
	// A merge of PR 0 is refused by the members' check alone: the reader's
	// check of a merge never seen open does not run on a write. A branch of
	// invalid UTF-8 would be written with U+FFFD in its place, and a record
	// over the size limit would not be read back.
	for _, bad := range []traceEvent{
		badRound, noSession, ev(1, tracePRMerged, 0, paneinput.TaskID{}, ""),
		ev(1, tracePROpen, 8, paneinput.TaskID{}, "scratch\xff"), ev(1, tracePROpen, 8, paneinput.TaskID{}, strings.Repeat("b", recordLimit)),
	} {
		if err := held.writeTrace(bad); !errors.Is(err, errRecord) {
			t.Errorf("writing %+v = %v, want errRecord", bad, err)
		}
	}
	elsewhere := ev(1, tracePROpen, 9, t1, branchT1)
	elsewhere.Worktree = Worktree{path: "/work/b"}
	if err := held.writeTrace(elsewhere); !errors.Is(err, errRecord) {
		t.Errorf("writing another worktree's event = %v, want errRecord", err)
	}
	h := store.readTraces(ownerA().Session)
	if len(h.unreadable) != 0 || !reflect.DeepEqual(h.events, map[Worktree][]traceEvent{traceTree: {first}}) {
		t.Errorf("history = %+v; want only the first event", h)
	}
}
