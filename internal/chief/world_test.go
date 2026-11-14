//go:build test

package chief

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeAgent is one agent of a fake Herdr.
type fakeAgent struct {
	Pane, Workspace, Kind, Status, Session, Dir string
	Seq                                         int
}

func (a fakeAgent) object() string {
	session := ""
	if a.Session != "" {
		session = fmt.Sprintf(`"agent_session":{"agent":%q,"kind":"id","source":"herdr:%[1]s","value":%q},`, a.Kind, a.Session)
	}
	return fmt.Sprintf(`{"agent":%q,%s"agent_status":%q,"cwd":%q,"pane_id":%q,"state_change_seq":%d,"workspace_id":%q}`,
		a.Kind, session, a.Status, a.Dir, a.Pane, a.Seq, a.Workspace)
}

func agentListOutput(agents []fakeAgent) []byte {
	var objs []string
	for _, a := range agents {
		objs = append(objs, a.object())
	}
	return []byte(`{"id":"cli:agent:list","result":{"agents":[` + strings.Join(objs, ",") + `],"type":"agent_list"}}`)
}

func agentGetOutput(a fakeAgent) []byte {
	return []byte(`{"id":"cli:agent:get","result":{"agent":` + a.object() + `,"type":"agent_info"}}`)
}

func workspaceListOutput(labels [][2]string) []byte {
	var objs []string
	for _, l := range labels {
		objs = append(objs, fmt.Sprintf(`{"label":%q,"workspace_id":%q}`, l[1], l[0]))
	}
	return []byte(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[` + strings.Join(objs, ",") + `]}}`)
}

// fakeDocs are the task documents a fake origin/main holds for one task.
type fakeDocs struct {
	// Dir is the task directory name, such as 0001_test_task.
	Dir string
	// Req, Arch, and Plan are the documents' content; nil is absent.
	Req, Arch, Plan []byte
}

const fakeTable = "## Document Status\n\n| Item | Value |\n|---|---|\n%s"

// statusDoc returns a task document with the given status table rows.
func statusDoc(rows string) []byte { return []byte(fmt.Sprintf(fakeTable, rows)) }

// fakeWorkspace is one task workspace of a fake Herdr.
type fakeWorkspace struct {
	ID, Label, Branch string
	// Kind is the agent name, "claude" when empty.
	Kind string
	// Status is the status of its one Claude Code agent.
	Status string
	// Screen is the name of the screen sample the agent shows.
	Screen string
	// PRs is the gh pr list output for the branch; "" means no PR.
	PRs  string
	Docs *fakeDocs
}

// world is a fake Herdr and repository for the snapshot tests: chief runs in
// the workspace "wc", and every added workspace has one Claude Code agent in
// a worktree directory of its own.
type world struct {
	t         *testing.T
	rec       *recorder
	env       Env
	store     Store
	now       time.Time
	chief     fakeAgent
	chiefTree string
	common    string
	agents    []fakeAgent
	labels    [][2]string
	trees     map[string]string
}

const worldChiefPane = "wc:p1"

// newWorld sets up the fake chief workspace, the state directory, and the
// replies chief's own worktree needs.
func newWorld(t *testing.T) *world {
	t.Helper()
	// The root is not t.TempDir(): its name holds the test's name with "_"
	// for blanks, and a Claude Code transcript is looked for only under a
	// working directory of the sampled characters.
	tmp, err := os.MkdirTemp("", "chief-world-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	root, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	w := &world{
		t:         t,
		rec:       newRecorder(t),
		now:       time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC),
		chiefTree: filepath.Join(root, "chief"),
		common:    filepath.Join(root, "repo.git"),
		trees:     map[string]string{},
	}
	for _, d := range []string{w.chiefTree, w.common, filepath.Join(root, "state")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	w.env = Env{PaneID: worldChiefPane, XDGStateHome: filepath.Join(root, "state"), Home: root}
	w.store, err = OpenStore(w.env.XDGStateHome)
	if err != nil {
		t.Fatal(err)
	}
	w.chief = fakeAgent{Pane: worldChiefPane, Workspace: "wc", Kind: "claude", Status: "working", Session: "00000000-0000-4000-8000-0000000000c1", Dir: w.chiefTree, Seq: 1}
	w.agents = []fakeAgent{w.chief}
	w.labels = [][2]string{{"wc", "chief"}}
	w.git(w.chiefTree, "rev-parse --show-toplevel", w.chiefTree)
	w.git(w.chiefTree, "rev-parse --path-format=absolute --git-common-dir", w.common)
	w.rec.replyIn(w.chiefTree, "git fetch origin main", nil)
	w.rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", repoContext(t))
	return w
}

// git sets the reply of a git command line in dir.
func (w *world) git(dir, line, out string) { w.rec.replyIn(dir, "git "+line, []byte(out)) }

// add adds a task workspace with one agent showing its screen sample and
// returns its worktree.
func (w *world) add(ws fakeWorkspace) string {
	w.t.Helper()
	tree := w.addTree(ws)
	pane := ws.ID + ":p1"
	status := ws.Status
	if status == "" {
		status = "idle"
	}
	kind := ws.Kind
	if kind == "" {
		kind = "claude"
	}
	w.agents = append(w.agents, fakeAgent{Pane: pane, Workspace: ws.ID, Kind: kind, Status: status, Session: "00000000-0000-4000-8000-" + fmt.Sprintf("%012d", len(w.agents)), Dir: tree, Seq: 10})
	screen := ws.Screen
	if screen == "" {
		screen = "claude-idle.ansi"
	}
	w.rec.reply("herdr agent read "+pane+" --source visible --lines 200 --format ansi", []byte(readScreenSample(w.t, screen)))
	return tree
}

// addTree adds a workspace's worktree with the replies of its git state, its
// open PRs, and its task documents, and returns it; its agents are added by
// the caller.
func (w *world) addTree(ws fakeWorkspace) string {
	w.t.Helper()
	tree := filepath.Join(filepath.Dir(w.chiefTree), "tree-"+ws.ID)
	if err := os.MkdirAll(tree, 0o700); err != nil {
		w.t.Fatal(err)
	}
	w.trees[ws.ID] = tree
	w.labels = append(w.labels, [2]string{ws.ID, ws.Label})
	w.git(tree, "rev-parse --show-toplevel", tree)
	w.git(tree, "rev-parse --path-format=absolute --git-common-dir", w.common)
	w.git(tree, "rev-parse --abbrev-ref HEAD", ws.Branch+"\n")
	if ws.Branch != "" && ws.Branch != detachedHead {
		prs := ws.PRs
		if prs == "" {
			prs = "[]"
		}
		w.rec.replyIn(tree, "gh pr list --head "+ws.Branch+" --state open --json headRefName,number,title,url", []byte(prs))
	}
	if ws.Docs != nil {
		w.docs(tree, *ws.Docs)
	} else if _, _, ok := branchTask0(ws.Branch); ok {
		w.rec.replyIn(tree, "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/", nil)
	}
	return tree
}

func branchTask0(branch string) (string, string, bool) {
	task, slug, ok := branchTask(branch)
	return task.String(), slug, ok
}

// docs sets the replies of the task document reads in tree.
func (w *world) docs(tree string, d fakeDocs) {
	w.rec.replyIn(tree, "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/", []byte("docs/tasks/"+d.Dir+"\x00"))
	var names []string
	for _, f := range []struct {
		file string
		body []byte
	}{{requirementsFile, d.Req}, {architectureFile, d.Arch}, {planFile, d.Plan}} {
		if f.body == nil {
			continue
		}
		path := "docs/tasks/" + d.Dir + "/" + f.file
		names = append(names, path)
		w.rec.replyIn(tree, "git show refs/remotes/origin/main:"+path, f.body)
	}
	w.rec.replyIn(tree, "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/"+d.Dir+"/", []byte(strings.Join(names, "\x00")+"\x00"))
}

// finish sets the Herdr replies from the agents added so far.
func (w *world) finish() Deps {
	w.rec.reply("herdr agent list", agentListOutput(w.agents))
	w.rec.reply("herdr workspace list", workspaceListOutput(w.labels))
	w.rec.reply("herdr agent get "+worldChiefPane, agentGetOutput(w.chief))
	return Deps{Env: w.env, Cmds: w.rec.commands(os.Environ()), Store: w.store, Now: func() time.Time { return w.now }}
}

// owner is the identity chief's pane has.
func (w *world) owner() Owner {
	return Owner{Pane: w.chief.Pane, Session: w.chief.Session}
}
