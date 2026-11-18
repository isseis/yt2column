//go:build test && unix

package chief

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/isseis/yt2column/internal/cachelock"
	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
)

// createIssue is the issue the creation tests make a task for: the number
// of the open issue sample.
const createIssue = "839"

// errInjected is a failure a test puts where the code under test meets it.
var errInjected = errors.New("injected failure")

// fakePane is one pane of the fake Herdr's pane list.
type fakePane struct {
	Pane, Tab, Workspace, Dir string
	// Agent is the agent's Herdr name, empty for a shell pane.
	Agent string
}

func paneListOutput(panes []fakePane) []byte {
	var objs []string
	for _, p := range panes {
		agent := ""
		if p.Agent != "" {
			agent = fmt.Sprintf(`"agent":%q,`, p.Agent)
		}
		objs = append(objs, fmt.Sprintf(`{%s"cwd":%q,"pane_id":%q,"tab_id":%q,"workspace_id":%q}`, agent, p.Dir, p.Pane, p.Tab, p.Workspace))
	}
	return []byte(`{"id":"cli:pane:list","result":{"panes":[` + strings.Join(objs, ",") + `],"type":"pane_list"}}`)
}

func rootPane(ws, tab, pane, cwd string) string {
	return fmt.Sprintf(`"root_pane":{"cwd":%q,"pane_id":%q,"tab_id":%q,"workspace_id":%q}`, cwd, pane, tab, ws)
}

// workspaceCreateOutput is the output of creating workspace w1 with its
// first tab w1:t1 and pane w1:p1.
func workspaceCreateOutput(cwd, label string) []byte {
	const ws, tab, pane = "w1", "w1:t1", "w1:p1"
	return fmt.Appendf(nil, `{"id":"cli:workspace:create","result":{%s,"tab":{"tab_id":%q,"workspace_id":%q},"type":"workspace_created","workspace":{"label":%q,"workspace_id":%q}}}`,
		rootPane(ws, tab, pane, cwd), tab, ws, label, ws)
}

func tabCreateOutput(ws, tab, pane, cwd string) []byte {
	return fmt.Appendf(nil, `{"id":"cli:tab:create","result":{%s,"tab":{"tab_id":%q,"workspace_id":%q},"type":"tab_created"}}`, rootPane(ws, tab, pane, cwd), tab, ws)
}

func tabRenameOutput(ws, tab, label string) []byte {
	return fmt.Appendf(nil, `{"id":"cli:tab:rename","result":{"tab":{"label":%q,"tab_id":%q,"workspace_id":%q},"type":"tab_info"}}`, label, tab, ws)
}

func agentStartOutput(a fakeAgent, name string) []byte {
	object := strings.Replace(a.object(), "{", fmt.Sprintf(`{"name":%q,`, name), 1)
	return []byte(`{"id":"cli:agent:start","result":{"agent":` + object + `,"type":"agent_started"}}`)
}

func processInfoOutput(pane string, shell, group int, pids ...int) []byte {
	var procs []string
	for _, pid := range pids {
		procs = append(procs, fmt.Sprintf(`{"pid":%d}`, pid))
	}
	return fmt.Appendf(nil, `{"id":"cli:pane:process_info","result":{"process_info":{"foreground_process_group_id":%d,"foreground_processes":[%s],"pane_id":%q,"shell_pid":%d},"type":"pane_process_info"}}`,
		group, strings.Join(procs, ","), pane, shell)
}

// createWorld is a send world in which `chief create` can make the task of
// issue 839: GitHub answers the open issue sample, origin/main is at commit,
// and the writes of a creation are allowed, each changing the fake as the
// real one did while sampling (the worktree's directory appears, the
// workspace's panes are listed, a started agent is listed and takes input).
type createWorld struct {
	*sendWorld
	deps   Deps
	tree   string
	commit string
	listed []fakePane
}

// newCreateWorld returns the world; setup runs before the writes of a
// creation are allowed, so a write it allows first answers in their place.
func newCreateWorld(t *testing.T, setup func(cw *createWorld)) *createWorld {
	t.Helper()
	sw := newSendWorld(t)
	cw := &createWorld{sendWorld: sw, commit: strings.Repeat("c", 40)}
	cw.tree = filepath.Join(sw.env.Home, ".herdr", "worktrees", "yt2column", "issue-"+createIssue)
	cw.listed = []fakePane{{Pane: worldChiefPane, Tab: "wc:t1", Workspace: "wc", Dir: sw.chiefTree, Agent: "claude"}}
	sw.labels = append(sw.labels, [2]string{"w1", "issue-" + createIssue})
	cw.issueAnswer(createIssue, reply{out: readTestdata(t, "gh/issue-open-839.json")})
	sw.git(sw.chiefTree, "rev-parse "+originMain, cw.commit+"\n")
	sw.git(sw.chiefTree, "worktree list --porcelain", "worktree "+sw.chiefTree+"\nHEAD "+cw.commit+"\nbranch refs/heads/main\n\n")
	sw.rec.replyFunc("herdr pane list", func() reply {
		sw.mu.Lock()
		defer sw.mu.Unlock()
		return reply{out: paneListOutput(cw.listed)}
	})
	for _, p := range []string{"w1:p2", "w1:p3"} {
		sw.rec.reply("herdr pane process-info --pane "+p, processInfoOutput(p, 100, 100, 100))
	}
	if setup != nil {
		setup(cw)
	}
	cw.allowCreation()
	cw.deps = sw.finish()
	return cw
}

// issueAnswer sets GitHub's answer about issue n, in chief's worktree.
func (sw *sendWorld) issueAnswer(n string, rep reply) {
	line := "gh " + strings.Join(issueArgs(issueNumber(sw.t, n)), " ")
	sw.rec.mu.Lock()
	defer sw.rec.mu.Unlock()
	sw.rec.dirReplies[sw.chiefTree+"\x00"+line] = rep
}

// issueRefusalCase is an answer about an issue that is not an open issue
// without the draft label, and the refusal both a creation and the targets
// give it.
type issueRefusalCase struct {
	number  string
	answer  reply
	refusal CreateRefusal
}

func issueRefusalCases(t *testing.T) []issueRefusalCase {
	t.Helper()
	return []issueRefusalCase{
		{"838", reply{out: readTestdata(t, "gh/issue-closed-838.json")}, CreateRefusalIssueClosed},
		{"932", reply{out: readTestdata(t, "gh/issue-draft-932.json")}, CreateRefusalIssueDraft},
		{"897", reply{out: readTestdata(t, "gh/issue-pr-897.json")}, CreateRefusalIssueIsPR},
		{"999999", reply{err: &cmdrunner.ExitError{Code: 1, Stdout: readTestdata(t, "gh/issue-missing.json")}}, CreateRefusalIssueNotFound},
		{"840", reply{out: readTestdata(t, "gh/issue-open-839.json")}, CreateRefusalIssueUnreadable}, // the answer of another number
		{"841", reply{err: errInjected}, CreateRefusalIssueUnreadable},
	}
}

// writeLines are the writes of a whole creation, in order.
func (cw *createWorld) writeLines() []string {
	label := "issue-" + createIssue
	return []string{
		"git worktree add --detach " + cw.tree + " " + cw.commit,
		"herdr workspace create --cwd " + cw.tree + " --label " + label + " --no-focus",
		"herdr tab rename w1:t1 cmd",
		"herdr tab create --workspace w1 --cwd " + cw.tree + " --label opencode --no-focus",
		"herdr tab create --workspace w1 --cwd " + cw.tree + " --label claude --no-focus",
		"herdr agent start " + label + "-opencode --kind opencode --pane w1:p2 --timeout 60000",
		"herdr agent start " + label + "-claude --kind claude --pane w1:p3 --timeout 60000",
	}
}

// writeArgs returns the arguments of the write line (its words after the
// program), which the allowances take.
func writeArgs(line string) (string, []string) {
	words := strings.Fields(line)
	return words[0], words[1:]
}

// allowCreation allows every write of a creation.
func (cw *createWorld) allowCreation() {
	lines := cw.writeLines()
	name, args := writeArgs(lines[0])
	cw.rec.allowWriteDo(name, cw.chiefTree, args, func() reply {
		if err := os.MkdirAll(cw.tree, 0o700); err != nil {
			return reply{err: err}
		}
		cw.git(cw.tree, "rev-parse --show-toplevel", cw.tree)
		cw.git(cw.tree, "rev-parse --path-format=absolute --git-common-dir", cw.common)
		return reply{}
	})
	name, args = writeArgs(lines[1])
	cw.rec.allowWriteDo(name, "", args, func() reply {
		cw.list(fakePane{Pane: "w1:p1", Tab: "w1:t1", Workspace: "w1", Dir: cw.tree})
		return reply{out: workspaceCreateOutput(cw.tree, "issue-"+createIssue)}
	})
	cw.rec.allowTabRename("w1:t1", tabShell, tabRenameOutput("w1", "w1:t1", tabShell))
	for i, line := range lines[3:5] {
		tab, pane := fmt.Sprintf("w1:t%d", i+2), fmt.Sprintf("w1:p%d", i+2)
		name, args := writeArgs(line)
		cw.rec.allowWriteDo(name, "", args, func() reply {
			cw.list(fakePane{Pane: pane, Tab: tab, Workspace: "w1", Dir: cw.tree})
			return reply{out: tabCreateOutput("w1", tab, pane, cw.tree)}
		})
	}
	for i, kind := range []string{"opencode", "claude"} {
		pane := fmt.Sprintf("w1:p%d", i+2)
		name, args := writeArgs(lines[5+i])
		cw.rec.allowWriteDo(name, "", args, func() reply {
			p := cw.startAgent(pane, kind, "idle", cw.tree)
			return reply{out: agentStartOutput(p.fakeAgent, "issue-"+createIssue+"-"+kind)}
		})
	}
}

// list adds a pane to Herdr's pane list.
func (cw *createWorld) list(p fakePane) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.listed = append(cw.listed, p)
}

// startAgent makes the agent of kind run in pane of w1 with status, working
// in dir: listed, read, and typed into like any other. A started Claude Code
// has a session but no transcript yet, as the sampled one did.
func (cw *createWorld) startAgent(pane, kind, status, dir string) *simPane {
	cw.mu.Lock()
	p := &simPane{fakeAgent: fakeAgent{Pane: pane, Workspace: "w1", Kind: kind, Status: status, Dir: dir, Seq: 10}}
	switch kind {
	case "claude":
		p.Session = cw.newSession()
		p.screen = readScreenSample(cw.t, "claude-started.ansi")
	default:
		p.screen = readScreenSample(cw.t, "opencode-done.ansi")
	}
	cw.panes = append(cw.panes, p)
	cw.byPane[pane] = p
	for i := range cw.listed {
		if cw.listed[i].Pane == pane {
			cw.listed[i].Agent = kind
		}
	}
	cw.mu.Unlock()
	cw.servePane(p)
	return p
}

// create runs Create for issue.
func (cw *createWorld) create(issue string) CreateOutcome {
	cw.t.Helper()
	out, err := Create(context.Background(), cw.deps, issue)
	if err != nil {
		cw.t.Fatalf("Create(%q): %v", issue, err)
	}
	return out
}

// writes returns the writes run so far, prompts and keys left out.
func (cw *createWorld) writes() []string {
	var out []string
	for _, c := range cw.rec.recorded() {
		if !isReadOnly(c) && !isPrompt(c) && !isKeys(c) {
			out = append(out, c.line())
		}
	}
	return out
}

// createdRecord returns the created record of the task's worktree, nil when
// there is none.
func (cw *createWorld) createdRecord() *CreatedRecord {
	cw.t.Helper()
	records, ok, err := cw.deps.Store.existingRecords(Worktree{path: cw.tree})
	if err != nil || !ok {
		cw.t.Fatalf("records of %s: ok %v, %v", cw.tree, ok, err)
	}
	rec, err := records.Created(cw.env.Home)
	if err != nil {
		cw.t.Fatalf("Created: %v", err)
	}
	return rec
}

// madeKinds returns the kinds of what was made, in order.
func madeKinds(made []Made) []string {
	var out []string
	for _, m := range made {
		out = append(out, m.Kind.String())
	}
	return out
}

// stateEntries returns the names under the state directory.
func stateEntries(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		names = append(names, strings.TrimPrefix(path, dir))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// fullMade is what a whole creation makes, by kind.
var fullMade = []string{"worktree", "record", "workspace", "tab", "pane", "tab", "pane", "tab", "pane", "agent", "agent"}

// TestCreateStages makes a task's worktree and workspace and checks every
// write in order: the worktree on a detached HEAD at the commit read after
// the fetch, the workspace labelled issue-839 with the tabs cmd, opencode,
// and claude at the worktree, and the two agents; no herdr call names a
// session, and the outcome lists everything made.
func TestCreateStages(t *testing.T) {
	cw := newCreateWorld(t, nil)
	out := cw.create(createIssue)
	if out.Result != CreateMade || out.Stage != CreateStageUnknown {
		t.Fatalf("Create = %+v, want made", out)
	}
	if got, want := cw.writes(), cw.writeLines(); !slices.Equal(got, want) {
		t.Errorf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	calls := cw.rec.recorded()
	at := func(line string) int {
		return slices.IndexFunc(calls, func(c recordedCall) bool { return c.line() == line })
	}
	fetch, readMain, add := at("git fetch origin main"), at("git rev-parse "+originMain), at(cw.writeLines()[0])
	if fetch < 0 || readMain < fetch || add < readMain {
		t.Errorf("fetch at %d, origin/main read at %d, worktree add at %d; want them in that order", fetch, readMain, add)
	}
	for _, c := range calls {
		if c.name == herdrProgram && slices.ContainsFunc(c.args, func(a string) bool { return strings.HasPrefix(a, "--session") }) {
			t.Errorf("herdr ran with a session: %s", c.line())
		}
	}
	if got := madeKinds(out.Made); !slices.Equal(got, fullMade) {
		t.Errorf("made %v, want %v", got, fullMade)
	}
	if out.Workspace != cw.tree || out.Label != "issue-"+createIssue || out.Title != "Sample title of an open issue" {
		t.Errorf("workspace %q, label %q, title %q", out.Workspace, out.Label, out.Title)
	}
	o := out.Output()
	if o.Result != "made" || o.Issue != createIssue || len(o.Made) != len(fullMade) || o.Made[0].ID != cw.tree || o.Mkreq == nil || o.Mkreq.Outcome != "sent" {
		t.Errorf("output = %+v", o)
	}
}

// TestCreateSendsMkreq checks that a creation that passes every stage types
// /clear, /model opus, and /mkreq of the issue into the Claude Code pane it
// started, and only there, and keeps the record of the issue.
func TestCreateSendsMkreq(t *testing.T) {
	cw := newCreateWorld(t, nil)
	out := cw.create(createIssue)
	if out.Mkreq == nil || out.Mkreq.Outcome != OutcomeSent {
		t.Fatalf("mkreq = %+v, want sent", out.Mkreq)
	}
	want := []string{"w1:p3 /clear", "w1:p3 /model opus", "w1:p3 /mkreq " + createIssue}
	if got := cw.rec.prompts(); !slices.Equal(got, want) {
		t.Errorf("typed %v, want %v", got, want)
	}
	if rec := cw.createdRecord(); rec == nil || rec.Issue.String() != createIssue || rec.Worktree.String() != cw.tree {
		t.Errorf("created record = %+v", rec)
	}
}

// TestWorkspaceMakerStartsAgentsWithoutArguments checks that each agent
// start runs with the agent start's own time limit, long enough for
// Herdr's wait. Its arguments (only the name, the kind, the pane, and the
// wait, so no permission or trust is granted for the human) are the exact
// write lines TestCreateStages compares.
func TestWorkspaceMakerStartsAgentsWithoutArguments(t *testing.T) {
	cw := newCreateWorld(t, nil)
	cw.create(createIssue)
	starts := 0
	for _, c := range cw.rec.recorded() {
		if c.name != herdrProgram || len(c.args) < 2 || c.args[0] != herdrAgent || c.args[1] != "start" {
			continue
		}
		starts++
		if c.limit <= agentStartTimeout-time.Second || c.limit > agentStartTimeout {
			t.Errorf("%s ran with %v left, want just under %v", c.line(), c.limit, agentStartTimeout)
		}
	}
	if starts != 2 {
		t.Fatalf("agent starts = %d, want 2", starts)
	}
}

// TestWorkspaceMakerWaitsForTheShell gives OpenCode's new pane a shell
// that runs startup processes for the first reads and checks that the agent
// is started once the shell is alone in its foreground, after a wait of
// shellInterval between the reads.
func TestWorkspaceMakerWaitsForTheShell(t *testing.T) {
	cw := newCreateWorld(t, nil)
	reads := 0
	cw.rec.replyFunc("herdr pane process-info --pane w1:p2", func() reply {
		reads++
		if reads < 3 {
			return reply{out: processInfoOutput("w1:p2", 300, 300, 300, 301)}
		}
		return reply{out: processInfoOutput("w1:p2", 300, 300, 300)}
	})
	var waits []time.Duration
	cw.deps.Sleep = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	out := cw.create(createIssue)
	if out.Result != CreateMade {
		t.Fatalf("Create = %+v, want made", out)
	}
	if reads != 3 {
		t.Errorf("process-info read %d times, want 3", reads)
	}
	if n := len(slices.DeleteFunc(slices.Clone(waits), func(d time.Duration) bool { return d != shellInterval })); n != 2 {
		t.Errorf("waits %v, want two of %v", waits, shellInterval)
	}
}

// TestWorkspaceMakerUsesOnlyItsIDs checks that a maker writes only to the
// workspace its own create returned (TestCreateStages compares every write
// with the lines naming w1 and its tabs and panes): a maker no constructor
// made runs nothing, and a tab Herdr reports in another workspace stops the
// maker before anything is written to it.
func TestWorkspaceMakerUsesOnlyItsIDs(t *testing.T) {
	t.Run("the zero maker runs nothing", func(t *testing.T) {
		rec := newRecorder(t)
		m := WorkspaceMaker{cmds: rec.commands(nil)}
		if _, _, err := m.makeWorkspace(context.Background(), func(Made) {}); !errors.Is(err, errMakerRefused) {
			t.Errorf("makeWorkspace = %v, want errMakerRefused", err)
		}
		created := createdWorkspace{Label: "issue-839", First: createdTab{Workspace: "w1", Tab: tabID{id: "w1:t1"}, Pane: PaneID{id: "w1:p1"}}}
		if _, err := m.makeTabs(context.Background(), created, func(Made) {}); !errors.Is(err, errMakerRefused) {
			t.Errorf("makeTabs = %v, want errMakerRefused", err)
		}
		if _, err := m.startAgent(context.Background(), PaneID{id: "w1:p2"}, herdrClaude, paneinput.AgentClaudeCode, "w1", func(Made) {}); !errors.Is(err, errMakerRefused) {
			t.Errorf("startAgent = %v, want errMakerRefused", err)
		}
		if calls := rec.recorded(); len(calls) != 0 {
			t.Errorf("ran %v", calls)
		}
	})
	t.Run("a tab of another workspace stops the maker", func(t *testing.T) {
		cw := newCreateWorld(t, func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[3])
			cw.rec.allowWrite(name, "", args, reply{out: tabCreateOutput("wc", "wc:t9", "wc:p9", cw.tree)})
		})
		out := cw.create(createIssue)
		if out.Result != CreateStopped || out.Stage != CreateStageTabs {
			t.Fatalf("Create = %+v, want stopped at tabs", out)
		}
		if got, want := cw.writes(), cw.writeLines()[:4]; !slices.Equal(got, want) {
			t.Errorf("writes %v, want %v", got, want)
		}
		if slices.ContainsFunc(out.Made, func(m Made) bool { return strings.HasPrefix(m.ID, "wc") }) {
			t.Errorf("made %+v holds an ID of another workspace", out.Made)
		}
	})
}

// TestCreateStopsAtEachStage fails each stage of a creation in turn and
// checks that it stops there: no later write and no input, the outcome names
// the stage and lists what was made, and the created record stays once it
// was written.
func TestCreateStopsAtEachStage(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(cw *createWorld)
		after  func(cw *createWorld)
		stage  CreateStage
		writes int
		made   int
	}{
		{"a record of an earlier use cannot be removed", nil, func(cw *createWorld) {
			dir := filepath.Join(cw.deps.Store.Dir(), workspaceDirOf(Worktree{path: cw.tree}))
			if err := os.MkdirAll(filepath.Join(dir, stopFile), 0o700); err != nil {
				cw.t.Fatal(err)
			}
		}, CreateStageRecords, 0, 0},
		{"git cannot add the worktree", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[0])
			cw.rec.allowWrite(name, cw.chiefTree, args, reply{err: errInjected})
		}, nil, CreateStageWorktree, 1, 0},
		{"the worktree is not where it was asked for", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[0])
			cw.rec.allowWriteDo(name, cw.chiefTree, args, func() reply {
				if err := os.MkdirAll(cw.tree, 0o700); err != nil {
					return reply{err: err}
				}
				cw.git(cw.tree, "rev-parse --show-toplevel", cw.chiefTree)
				return reply{}
			})
		}, nil, CreateStageWorktree, 1, 1},
		{"git stops halfway and leaves the directory", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[0])
			cw.rec.allowWriteDo(name, cw.chiefTree, args, func() reply {
				if err := os.MkdirAll(cw.tree, 0o700); err != nil {
					return reply{err: err}
				}
				return reply{err: errInjected}
			})
		}, nil, CreateStageWorktree, 1, 1},
		{"the created record cannot be written", nil, func(cw *createWorld) {
			cw.deps.Store.beforeCreate = func(name string) error {
				if name == createdFile {
					return errInjected
				}
				return nil
			}
		}, CreateStageRecord, 1, 1},
		{"herdr cannot create the workspace", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[1])
			cw.rec.allowWrite(name, "", args, reply{err: errInjected})
		}, nil, CreateStageWorkspace, 2, 2},
		{"the workspace's output cannot be read", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[1])
			cw.rec.allowWrite(name, "", args, reply{out: []byte("{}")})
		}, nil, CreateStageWorkspace, 2, 3},
		{"the workspace is labelled otherwise", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[1])
			cw.rec.allowWrite(name, "", args, reply{out: workspaceCreateOutput(cw.tree, "other")})
		}, nil, CreateStageWorkspace, 2, 5},
		{"the workspace is opened elsewhere", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[1])
			cw.rec.allowWrite(name, "", args, reply{out: workspaceCreateOutput(cw.chiefTree, "issue-"+createIssue)})
		}, nil, CreateStageWorkspace, 2, 5},
		{"herdr cannot rename the first tab", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[2])
			cw.rec.allowWrite(name, "", args, reply{err: errInjected})
		}, nil, CreateStageTabs, 3, 5},
		{"the rename reports another label", func(cw *createWorld) {
			cw.rec.allowTabRename("w1:t1", tabShell, tabRenameOutput("w1", "w1:t1", "other"))
		}, nil, CreateStageTabs, 3, 5},
		{"herdr cannot create a tab", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[3])
			cw.rec.allowWrite(name, "", args, reply{err: errInjected})
		}, nil, CreateStageTabs, 4, 5},
		{"a tab's output cannot be read", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[3])
			cw.rec.allowWrite(name, "", args, reply{out: []byte("{}")})
		}, nil, CreateStageTabs, 4, 6},
		{"a tab is opened elsewhere", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[3])
			cw.rec.allowWrite(name, "", args, reply{out: tabCreateOutput("w1", "w1:t2", "w1:p2", cw.chiefTree)})
		}, nil, CreateStageTabs, 4, 7},
		{"OpenCode's shell is busy", func(cw *createWorld) {
			cw.rec.reply("herdr pane process-info --pane w1:p2", processInfoOutput("w1:p2", 100, 200, 200))
		}, nil, CreateStageOpenCode, 5, 9},
		{"OpenCode's start output cannot be read", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[5])
			cw.rec.allowWrite(name, "", args, reply{out: []byte("{}")})
		}, nil, CreateStageOpenCode, 6, 10},
		{"OpenCode is reported under another name", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[5])
			cw.rec.allowWriteDo(name, "", args, func() reply {
				p := cw.startAgent("w1:p2", "opencode", "idle", cw.tree)
				return reply{out: agentStartOutput(p.fakeAgent, "someone-else")}
			})
		}, nil, CreateStageOpenCode, 6, 10},
		{"Claude Code fails to start and no agent is there", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[6])
			cw.rec.allowWrite(name, "", args, reply{err: &cmdrunner.ExitError{Code: 1, Stderr: readTestdata(cw.t, "herdr/agent-start-name-taken.stderr")}})
			cw.rec.fail("herdr agent get w1:p3", &cmdrunner.ExitError{Code: 1})
		}, nil, CreateStageClaude, 7, 10},
		{"Claude Code works in another directory", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[6])
			cw.rec.allowWriteDo(name, "", args, func() reply {
				p := cw.startAgent("w1:p3", "claude", "idle", filepath.Join(cw.tree, "elsewhere"))
				return reply{out: agentStartOutput(p.fakeAgent, "issue-"+createIssue+"-claude")}
			})
		}, nil, CreateStagePanes, 7, 11},
		{"Claude Code is reported but not listed", func(cw *createWorld) {
			name, args := writeArgs(cw.writeLines()[6])
			cw.rec.allowWrite(name, "", args, reply{out: agentStartOutput(fakeAgent{Pane: "w1:p3", Workspace: "w1", Kind: "claude", Status: "idle", Dir: cw.tree, Seq: 10}, "issue-"+createIssue+"-claude")})
		}, nil, CreateStagePanes, 7, 11},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cw := newCreateWorld(t, c.setup)
			if c.after != nil {
				c.after(cw)
			}
			out := cw.create(createIssue)
			if out.Result != CreateStopped || out.Stage != c.stage || out.Failure == "" {
				t.Fatalf("Create = %+v, want stopped at %s", out, c.stage)
			}
			if got, want := cw.writes(), cw.writeLines()[:c.writes]; !slices.Equal(got, want) {
				t.Errorf("writes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if got := madeKinds(out.Made); !slices.Equal(got, fullMade[:c.made]) {
				t.Errorf("made %v, want %v", got, fullMade[:c.made])
			}
			if got := cw.rec.prompts(); len(got) != 0 {
				t.Errorf("typed %v", got)
			}
			if c.made >= 2 {
				if rec := cw.createdRecord(); rec == nil || rec.Issue.String() != createIssue {
					t.Errorf("created record = %+v, want the one of the issue", rec)
				}
			}
			o := out.Output()
			if o.Result != "stopped" || o.Stage != c.stage.String() || o.Mkreq != nil {
				t.Errorf("output = %+v", o)
			}
			// A workspace made is named by its label too, as Herdr reported
			// it: the name Herdr shows the human who cleans up.
			label := "issue-" + createIssue
			switch c.name {
			case "the workspace is labelled otherwise":
				label = "other"
			case "the workspace's output cannot be read":
				label = ""
			}
			for _, m := range o.Made {
				if want := map[bool]string{true: label}[m.Kind == MadeWorkspace.String()]; m.Label != want {
					t.Errorf("made %+v: label %q, want %q", m, m.Label, want)
				}
			}
		})
	}
}

// TestCreateDeadline ends the creation's context while the workspace is
// created, as its time limit does, and checks that the creation stops at
// the next stage with the stage deadline, listing what was made and writing nothing more.
func TestCreateDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cw := newCreateWorld(t, func(cw *createWorld) {
		name, args := writeArgs(cw.writeLines()[1])
		cw.rec.allowWriteDo(name, "", args, func() reply {
			cancel()
			cw.list(fakePane{Pane: "w1:p1", Tab: "w1:t1", Workspace: "w1", Dir: cw.tree})
			return reply{out: workspaceCreateOutput(cw.tree, "issue-"+createIssue)}
		})
	})
	out, err := Create(ctx, cw.deps, createIssue)
	if err != nil || out.Result != CreateStopped || out.Stage != CreateStageDeadline {
		t.Fatalf("Create = %+v, %v; want stopped at the deadline", out, err)
	}
	if got, want := cw.writes(), cw.writeLines()[:2]; !slices.Equal(got, want) {
		t.Errorf("writes %v, want %v", got, want)
	}
	if got := madeKinds(out.Made); !slices.Equal(got, fullMade[:5]) {
		t.Errorf("made %v", got)
	}
}

// TestCreateRefusesIssue checks every answer about the issue that is not an
// open issue without the draft label: nothing is run after the read, nothing
// is made, and the refusal names the answer.
func TestCreateRefusesIssue(t *testing.T) {
	for _, c := range issueRefusalCases(t) {
		t.Run(c.refusal.String()+" "+c.number, func(t *testing.T) {
			cw := newCreateWorld(t, func(cw *createWorld) { cw.issueAnswer(c.number, c.answer) })
			before := stateEntries(t, cw.deps.Store.Dir())
			out := cw.create(c.number)
			if out.Result != CreateRefused || out.Refusal != c.refusal || len(out.Made) != 0 {
				t.Fatalf("Create = %+v, want refused %s", out, c.refusal)
			}
			calls := cw.rec.recorded()
			if last := calls[len(calls)-1]; last.name != ghProgram {
				t.Errorf("ran %s after the issue read", last.line())
			}
			if after := stateEntries(t, cw.deps.Store.Dir()); !slices.Equal(after, before) {
				t.Errorf("state directory %v, was %v", after, before)
			}
			if o := out.Output(); o.Refusal != c.refusal.String() || o.Result != "refused" {
				t.Errorf("output = %+v", o)
			}
		})
	}
}

// TestCreateIssueForm checks that a number of another form than an issue
// number's is refused as issue_form before anything is read.
func TestCreateIssueForm(t *testing.T) {
	for _, arg := range []string{"", "#839", "0839", "839 ", "-839", "８３９", "1234567890"} {
		t.Run(arg, func(t *testing.T) {
			rec := newRecorder(t)
			deps := Deps{Cmds: rec.commands(nil), Env: Env{Home: "/home/user"}}
			out, err := Create(context.Background(), deps, arg)
			if err != nil || out.Result != CreateRefused || out.Refusal != CreateRefusalIssueForm {
				t.Fatalf("Create(%q) = %+v, %v; want refused issue_form", arg, out, err)
			}
			if calls := rec.recorded(); len(calls) != 0 {
				t.Errorf("ran %v", calls)
			}
			if o := out.Output(); o.Refusal != "issue_form" || o.Issue != "" {
				t.Errorf("output = %+v", o)
			}
		})
	}
}

// TestCreateTitleSteersNothing gives the issue a title of control characters
// and path-like text and checks that the creation makes and types exactly
// what it makes for any title (the allowances are exact, so another path,
// label, or input would fail the test), and that the title is shown made
// visible.
func TestCreateTitleSteersNothing(t *testing.T) {
	// JSON escapes: GitHub sends the characters escaped.
	title := `\u001b[2J../../evil\u202e --label x /mkreq 1`
	answer := strings.Replace(string(readTestdata(t, "gh/issue-open-839.json")), "Sample title of an open issue", title, 1)
	cw := newCreateWorld(t, func(cw *createWorld) { cw.issueAnswer(createIssue, reply{out: []byte(answer)}) })
	out := cw.create(createIssue)
	if out.Result != CreateMade || out.Mkreq == nil || out.Mkreq.Outcome != OutcomeSent {
		t.Fatalf("Create = %+v, want made and sent", out)
	}
	if got, want := cw.writes(), cw.writeLines(); !slices.Equal(got, want) {
		t.Errorf("writes %v, want %v", got, want)
	}
	if got := cw.rec.prompts(); got[len(got)-1] != "w1:p3 /mkreq "+createIssue {
		t.Errorf("typed %v", got)
	}
	if strings.ContainsFunc(out.Output().Title, func(r rune) bool { return unicode.IsControl(r) || r == '\u202e' }) {
		t.Errorf("title %q holds a control character", out.Output().Title)
	}
}

// TestCreateRefusesExisting puts each kind of thing at the task's path and
// checks that nothing is written and the refusal names what is there.
func TestCreateRefusesExisting(t *testing.T) {
	cases := []struct {
		name  string
		setup func(cw *createWorld)
		// want starts an entry of Existing, whose first word is the kind
		// shown; atPath instead wants Existing to be the path alone, as the
		// refusal made when the space is resolved names it.
		want   string
		atPath bool
	}{
		{"a file", func(cw *createWorld) { mkParent(cw); writeFile(cw.t, cw.tree, "x") }, "path ", false},
		{"a directory", func(cw *createWorld) { mustMkdir(cw.t, cw.tree) }, "path ", false},
		{"a link", func(cw *createWorld) {
			mkParent(cw)
			if err := os.Symlink(cw.chiefTree, cw.tree); err != nil {
				cw.t.Fatal(err)
			}
		}, "", true},
		{"a registration whose directory is gone", func(cw *createWorld) {
			cw.git(cw.chiefTree, "worktree list --porcelain", "worktree "+cw.chiefTree+"\nHEAD "+cw.commit+"\nbranch refs/heads/main\n\nworktree "+cw.tree+"\nHEAD "+cw.commit+"\ndetached\nprunable gitdir file points to non-existent location\n\n")
		}, "worktree ", false},
		{"a workspace with a pane below the path", func(cw *createWorld) {
			cw.listed = append(cw.listed, fakePane{Pane: "w9:p1", Tab: "w9:t1", Workspace: "w9", Dir: filepath.Join(cw.tree, "sub")})
		}, "workspace w9", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cw := newCreateWorld(t, c.setup)
			out := cw.create(createIssue)
			if out.Result != CreateRefused || out.Refusal != CreateRefusalTargetExists {
				t.Fatalf("Create = %+v, want refused target_exists", out)
			}
			if c.atPath {
				if !slices.Equal(out.Existing, []string{cw.tree}) {
					t.Errorf("existing = %v, want [%s]", out.Existing, cw.tree)
				}
			} else if !slices.ContainsFunc(out.Existing, func(e string) bool { return strings.HasPrefix(e, c.want) }) {
				t.Errorf("existing = %v, want an entry starting %q", out.Existing, c.want)
			}
			if got := cw.writes(); len(got) != 0 {
				t.Errorf("wrote %v", got)
			}
		})
	}
	t.Run("a home whose path chief cannot send to", func(t *testing.T) {
		cw := newCreateWorld(t, nil)
		cw.deps.Env.Home = filepath.Join(cw.env.Home, "home_dir")
		before := len(cw.rec.recorded())
		out := cw.create(createIssue)
		if out.Result != CreateRefused || out.Refusal != CreateRefusalPathUnsupported {
			t.Fatalf("Create = %+v, want refused unsupported_path", out)
		}
		if calls := cw.rec.recorded(); len(calls) != before {
			t.Errorf("ran %v", calls[before:])
		}
	})
}

// TestCreateRechecksTheSpaceUnderTheExclusions puts a link at the path
// while origin/main is fetched, after the first check of the space and
// before the exclusions, and checks that the check made under them refuses.
func TestCreateRechecksTheSpaceUnderTheExclusions(t *testing.T) {
	cw := newCreateWorld(t, nil)
	cw.rec.replyFunc("git fetch origin main", func() reply {
		mkParent(cw)
		if err := os.Symlink(cw.chiefTree, cw.tree); err != nil {
			return reply{err: err}
		}
		return reply{}
	})
	out := cw.create(createIssue)
	if out.Result != CreateRefused || out.Refusal != CreateRefusalTargetExists || !slices.Equal(out.Existing, []string{cw.tree}) {
		t.Fatalf("Create = %+v, want refused target_exists naming the path", out)
	}
	if got := cw.writes(); len(got) != 0 {
		t.Errorf("wrote %v", got)
	}
}

// TestCreateFetchFailure checks that a failed fetch, or an origin/main
// that cannot be read after it, refuses the creation before the exclusions
// are taken and anything is written.
func TestCreateFetchFailure(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(cw *createWorld)
	}{
		{"the fetch fails", func(cw *createWorld) { cw.rec.failIn(cw.chiefTree, "git fetch origin main", errInjected) }},
		{"origin/main cannot be read after the fetch", func(cw *createWorld) { cw.git(cw.chiefTree, "rev-parse "+originMain, "not a commit\n") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			cw := newCreateWorld(t, c.setup)
			out := cw.create(createIssue)
			if out.Result != CreateRefused || out.Refusal != CreateRefusalFetchFailed {
				t.Fatalf("Create = %+v, want refused fetch_failed", out)
			}
			if got := cw.writes(); len(got) != 0 {
				t.Errorf("wrote %v", got)
			}
			if _, err := os.Lstat(filepath.Join(cw.deps.Store.Dir(), creationDir)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the creation exclusion's directory: %v, want none", err)
			}
		})
	}
}

// TestCreateReplacesOldRecords leaves the records of an earlier use of the
// path (a created record of another issue, a stop, and an unfinished send)
// and checks that the creation removes them, sends /mkreq, and keeps the
// record of the new issue.
func TestCreateReplacesOldRecords(t *testing.T) {
	cw := newCreateWorld(t, nil)
	held := holdFor(t, cw.deps, cw.tree)
	if err := held.store().create(createdFile, createdJSON{Worktree: cw.tree, Issue: "12", Created: cw.now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := held.Stop("an earlier send failed", "", "", cw.now); err != nil {
		t.Fatal(err)
	}
	if _, err := held.BeginSend(SendRecord{Pane: "w0:p1", Input: "command", Command: "/mkreq 12"}, cw.now); err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	out := cw.create(createIssue)
	if out.Result != CreateMade || out.Mkreq == nil || out.Mkreq.Outcome != OutcomeSent {
		t.Fatalf("Create = %+v, want made and sent", out)
	}
	rec := cw.createdRecord()
	if rec == nil || rec.Issue.String() != createIssue || !rec.Created.Equal(cw.now) {
		t.Errorf("created record = %+v, want the new issue's", rec)
	}
}

// TestCreateAgentNotReady starts Claude Code with Herdr's error response
// while the agent is in its pane, and checks that the creation is made but
// /mkreq is not sent, with the status read from the pane.
func TestCreateAgentNotReady(t *testing.T) {
	for _, status := range []string{"idle", "blocked"} {
		t.Run(status, func(t *testing.T) {
			cw := newCreateWorld(t, func(cw *createWorld) {
				name, args := writeArgs(cw.writeLines()[6])
				cw.rec.allowWriteDo(name, "", args, func() reply {
					cw.startAgent("w1:p3", "claude", status, cw.tree)
					return reply{err: &cmdrunner.ExitError{Code: 1, Stderr: readTestdata(cw.t, "herdr/agent-start-timeout.stderr")}}
				})
			})
			out := cw.create(createIssue)
			if out.Result != CreateMade || out.Mkreq != nil || out.NotReady == nil || out.NotReady.Status != agentStatuses[status] || out.NotReady.Code != "timeout" {
				t.Fatalf("Create = %+v, want made with Claude Code not ready", out)
			}
			if got := cw.rec.prompts(); len(got) != 0 {
				t.Errorf("typed %v", got)
			}
			o := out.Output()
			if o.Mkreq == nil || o.Mkreq.Outcome != "not_sent" || o.Mkreq.Reason != notReadyReason || o.Mkreq.Status != status || o.Mkreq.Pane != "w1:p3" || o.Mkreq.Command != "/mkreq "+createIssue {
				t.Errorf("output mkreq = %+v", o.Mkreq)
			}
		})
	}
}

// TestCreateBusyDuringTick runs a creation while a tick holds the creation
// exclusion for its automatic send and checks that the creation is refused
// busy before anything is written; after the tick, the exclusion is free.
func TestCreateBusyDuringTick(t *testing.T) {
	mw := newMergeWorld(t, mkarchDocs)
	mw.tick(time.Minute)
	mw.merge(nextBranch, "`/mkarch 0001`")
	issueLine := "gh " + strings.Join(issueArgs(issueNumber(t, createIssue)), " ")
	mw.rec.replyIn(mw.chiefTree, issueLine, readTestdata(t, "gh/issue-open-839.json"))
	mw.git(mw.chiefTree, "worktree list --porcelain", "worktree "+mw.chiefTree+"\n\n")
	mw.rec.replyFunc("herdr pane list", func() reply { return reply{out: paneListOutput(nil)} })
	var during *CreateOutcome
	remote := "git ls-remote origin " + remoteMainRef
	mw.rec.replyFunc(remote, func() reply {
		if during == nil {
			out, err := Create(context.Background(), mw.deps, createIssue)
			if err != nil {
				t.Errorf("Create during the tick: %v", err)
			}
			during = &out
		}
		return reply{out: []byte(mainCommit + "\trefs/heads/main\n")}
	})
	if res := only(t, mw.tick(time.Minute)); !sentCommand(res) {
		t.Fatalf("auto send = %+v; the tick did not send", res)
	}
	if during == nil || during.Result != CreateRefused || during.Refusal != CreateRefusalBusy {
		t.Fatalf("Create during the tick = %+v, want refused busy", during)
	}
	lock, err := mw.deps.Store.LockCreation()
	if err != nil {
		t.Fatalf("LockCreation after the tick: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestLockCreationBusy takes the creation exclusion twice and checks that
// the second is ErrCreationBusy alone, without cachelock's error, whose
// advice is about yt-dlp's cache.
func TestLockCreationBusy(t *testing.T) {
	cw := newCreateWorld(t, nil)
	lock, err := cw.deps.Store.LockCreation()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	_, err = cw.deps.Store.LockCreation()
	if !errors.Is(err, ErrCreationBusy) || errors.Is(err, cachelock.ErrLocked) {
		t.Errorf("second LockCreation = %v, want ErrCreationBusy alone", err)
	}
}

// TestCreateBusyWhileThePathIsHeld holds the exclusion of the task's
// worktree, as a send or a removal does, and checks that the creation is
// refused busy before anything is written.
func TestCreateBusyWhileThePathIsHeld(t *testing.T) {
	cw := newCreateWorld(t, nil)
	holdFor(t, cw.deps, cw.tree)
	out := cw.create(createIssue)
	if out.Result != CreateRefused || out.Refusal != CreateRefusalBusy {
		t.Fatalf("Create = %+v, want refused busy", out)
	}
	if got := cw.writes(); len(got) != 0 {
		t.Errorf("wrote %v", got)
	}
}

// TestTargets checks the targets of /mkreq: a number of another form is
// refused before anything is read; an answer about the issue that is not an
// open issue without the draft label is refused as a creation refuses it,
// with nothing run after the read; otherwise the issue's title is given and
// the workspaces whose row offers /mkreq are listed. No record is written.
func TestTargets(t *testing.T) {
	ctx := context.Background()
	t.Run("issue_form", func(t *testing.T) {
		rec := newRecorder(t)
		out, err := Targets(ctx, Deps{Cmds: rec.commands(nil)}, "#839")
		if err != nil || out.Refusal != CreateRefusalIssueForm || len(out.Workspaces) != 0 {
			t.Fatalf("Targets = %+v, %v", out, err)
		}
		if calls := rec.recorded(); len(calls) != 0 {
			t.Errorf("ran %v", calls)
		}
		if o := out.Output(); o.Refusal != "issue_form" || o.Workspaces == nil {
			t.Errorf("output = %+v", o)
		}
	})
	for _, c := range []struct {
		name string
		free bool
	}{{"a workspace without a task", true}, {"no candidate", false}} {
		t.Run(c.name, func(t *testing.T) {
			sw := newSendWorld(t)
			var free string
			if c.free {
				free = sw.addTask(fakeWorkspace{ID: "w1", Label: "free", Branch: "user/feature"}, "claude")
			}
			sw.addTask(fakeWorkspace{ID: "w2", Label: "task", Branch: taskBranch, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved, Arch: archApproved, Plan: planSample(t, "marker-opus.md")}}, "claude")
			sw.issueAnswer(createIssue, reply{out: readTestdata(t, "gh/issue-open-839.json")})
			deps := sw.finish()
			before := stateEntries(t, deps.Store.Dir())
			out, err := Targets(ctx, deps, createIssue)
			if err != nil || out.Refusal != CreateRefusalUnknown || out.Issue.String() != createIssue || out.Title != "Sample title of an open issue" {
				t.Fatalf("Targets = %+v, %v", out, err)
			}
			var want []TargetWorkspace
			if c.free {
				want = []TargetWorkspace{{Workspace: free, Label: "free"}}
			}
			if !slices.Equal(out.Workspaces, want) {
				t.Errorf("workspaces = %+v, want %+v", out.Workspaces, want)
			}
			if after := stateEntries(t, deps.Store.Dir()); !slices.Equal(after, before) {
				t.Errorf("state directory %v, was %v", after, before)
			}
			if o := out.Output(); len(o.Workspaces) != len(want) || o.Refusal != "" || o.Title != out.Title {
				t.Errorf("output = %+v", o)
			}
		})
	}
	for _, c := range issueRefusalCases(t) {
		t.Run(c.refusal.String()+" "+c.number, func(t *testing.T) {
			sw := newSendWorld(t)
			sw.addTask(fakeWorkspace{ID: "w1", Label: "free", Branch: "user/feature"}, "claude")
			sw.issueAnswer(c.number, c.answer)
			deps := sw.finish()
			before := stateEntries(t, deps.Store.Dir())
			out, err := Targets(ctx, deps, c.number)
			if err != nil || out.Refusal != c.refusal || len(out.Workspaces) != 0 {
				t.Fatalf("Targets = %+v, %v; want refused %s with no workspace", out, err, c.refusal)
			}
			calls := sw.rec.recorded()
			if last := calls[len(calls)-1]; last.name != ghProgram || !isIssueRead(last.args) {
				t.Errorf("ran %s after the issue read", last.line())
			}
			if after := stateEntries(t, deps.Store.Dir()); !slices.Equal(after, before) {
				t.Errorf("state directory %v, was %v", after, before)
			}
			if o := out.Output(); o.Refusal != c.refusal.String() || o.Workspaces == nil {
				t.Errorf("output = %+v", o)
			}
		})
	}
}

// TestWorktreeAddDetached adds a task's worktree with the real git and
// checks that it is on a detached HEAD at the commit, that no branch was
// made, and that a value that is not a full commit ID is refused before git
// runs.
func TestWorktreeAddDetached(t *testing.T) {
	repo := newTestRepo(t)
	cmds := repo.RealCommands()
	// The home is under /tmp and not t.TempDir(), whose name may hold a
	// character a task space refuses ("_"), as in newWorld.
	tmp, err := os.MkdirTemp("/tmp", "chief-add-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	home, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	space, err := NewTaskSpace(home, issueNumber(t, createIssue))
	if err != nil {
		t.Fatal(err)
	}
	chiefTree := Worktree{path: repo.Dir}
	if err := addTaskWorktree(context.Background(), cmds, chiefTree, space, "main"); err == nil {
		t.Fatal("a branch name was taken for a commit")
	}
	if _, err := os.Lstat(space.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the refused add left %s: %v", space.Path(), err)
	}
	commit := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	branches := repo.Git("branch", "--list")
	if err := addTaskWorktree(context.Background(), cmds, chiefTree, space, commit); err != nil {
		t.Fatalf("addTaskWorktree: %v", err)
	}
	if got := strings.TrimSpace(repo.Run(filepath.Join(testToolsDir, gitProgram), "-C", space.Path(), "rev-parse", "--abbrev-ref", "HEAD")); got != detachedHead {
		t.Errorf("HEAD of the worktree is %q, want detached", got)
	}
	if got := strings.TrimSpace(repo.Run(filepath.Join(testToolsDir, gitProgram), "-C", space.Path(), "rev-parse", "HEAD")); got != commit {
		t.Errorf("the worktree is at %s, want %s", got, commit)
	}
	if got := repo.Git("branch", "--list"); got != branches {
		t.Errorf("branches %q, were %q", got, branches)
	}
	tree, err := CanonicalWorktree(context.Background(), cmds, space.Path())
	if want, _ := space.Worktree(); err != nil || tree != want {
		t.Errorf("CanonicalWorktree = %v, %v; want %v", tree, err, want)
	}
	if err := addTaskWorktree(context.Background(), cmds, chiefTree, TaskSpace{}, commit); !errors.Is(err, errNoTaskSpace) {
		t.Errorf("addTaskWorktree of the zero space = %v, want errNoTaskSpace", err)
	}
}

// mkParent creates the directory that holds the task's path.
func mkParent(cw *createWorld) { mustMkdir(cw.t, filepath.Dir(cw.tree)) }

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
