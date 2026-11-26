//go:build test

package chief

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestParsePRList reads a PR list whose elements carry every member chief
// asks for, and rejects the whole list for an element of any other shape:
// the four members that decide a merge conflict are untrusted values taken
// only in their exact forms, so one element read in part never makes a row.
func TestParsePRList(t *testing.T) {
	const head = "0123456789abcdef0123456789abcdef01234567"
	element := func(members string) string {
		return `{"headRefName":"b","number":1,"title":"t","url":"u",` + members + `}`
	}
	// members are the four conflict members in a valid form, in order.
	members := [][2]string{{"mergeable", `"CONFLICTING"`}, {"headRefOid", `"` + head + `"`}, {"baseRefName", `"main"`}, {"isCrossRepository", "false"}}
	// without joins the members, leaving out the one named skip.
	without := func(skip string) string {
		var parts []string
		for _, m := range members {
			if m[0] != skip {
				parts = append(parts, `"`+m[0]+`":`+m[1])
			}
		}
		return strings.Join(parts, ",")
	}
	valid := without("")
	prs, err := parsePRList([]byte("["+element(valid)+"]"), "b")
	if err != nil || len(prs) != 1 {
		t.Fatalf("parsePRList = %+v, %v", prs, err)
	}
	if got := prs[0]; got.Number != 1 || got.Mergeable != MergeableConflicting || got.Head.String() != head || !got.BaseIsMain || got.CrossRepository {
		t.Errorf("parsePRList = %+v", got)
	}
	accepted := []struct {
		name      string
		members   string
		mergeable Mergeable
		baseMain  bool
		cross     bool
	}{
		{"mergeable", strings.Replace(valid, "CONFLICTING", "MERGEABLE", 1), MergeableNoConflict, true, false},
		{"not computed", strings.Replace(valid, "CONFLICTING", "UNKNOWN", 1), MergeableUnknown, true, false},
		{"another base", strings.Replace(valid, `"main"`, `"release"`, 1), MergeableConflicting, false, false},
		// A base is any non-empty string; only main, compared as bytes, is
		// main.
		{"a base with a space around main", strings.Replace(valid, `"main"`, `" main"`, 1), MergeableConflicting, false, false},
		{"another repository", strings.Replace(valid, "false", "true", 1), MergeableConflicting, true, true},
	}
	for _, c := range accepted {
		prs, err := parsePRList([]byte("["+element(c.members)+"]"), "b")
		if err != nil || len(prs) != 1 {
			t.Errorf("%s: parsePRList = %+v, %v", c.name, prs, err)
			continue
		}
		if got := prs[0]; got.Mergeable != c.mergeable || got.BaseIsMain != c.baseMain || got.CrossRepository != c.cross || got.Head.String() != head {
			t.Errorf("%s: parsePRList = %+v", c.name, got)
		}
	}
	// The sample of the list with every member chief asks for, as gh gave
	// it.
	sample, err := parsePRList(readTestdata(t, "gh/pr-list-open-1144.json"), "owner/0124-checks-proportion-02")
	if err != nil || len(sample) != 1 || sample[0].Number != 1144 || sample[0].Mergeable != MergeableNoConflict || sample[0].Head.String() != "cf1b86c57cb876daf040247915145edefbd0eefc" || !sample[0].BaseIsMain || sample[0].CrossRepository {
		t.Errorf("the sampled list = %+v, %v", sample, err)
	}
	if prs, err := parsePRList(readTestdata(t, "gh/pr-list-none.json"), "any"); err != nil || len(prs) != 0 {
		t.Errorf("an empty list = %+v, %v", prs, err)
	}
	rejected := map[string]string{
		"another branch":   `[{"headRefName":"other","number":1,"title":"t","url":"u",` + valid + `}]`,
		"zero number":      `[{"headRefName":"b","number":0,"title":"t","url":"u",` + valid + `}]`,
		"missing url":      `[{"headRefName":"b","number":1,"title":"t",` + valid + `}]`,
		"object not array": `{"headRefName":"b"}`,
		// Real gh output in the shape chief asked for before: kept as the
		// old shape's rejection, beside the generated missing-member rows.
		"a list without the conflict members": string(readTestdata(t, "gh/pr-list-open-867.json")),
	}
	for _, m := range members {
		rejected["no "+m[0]] = "[" + element(without(m[0])) + "]"
		rejected["a null "+m[0]] = "[" + element(without(m[0])+`,"`+m[0]+`":null`) + "]"
		rejected["a repeated "+m[0]] = "[" + element(valid+`,"`+m[0]+`":`+m[1]) + "]"
	}
	for name, members := range map[string]string{
		"a mergeable of another case":    strings.Replace(valid, "CONFLICTING", "Conflicting", 1),
		"a mergeable with a space":       strings.Replace(valid, `"CONFLICTING"`, `"CONFLICTING "`, 1),
		"a mergeable of another value":   strings.Replace(valid, "CONFLICTING", "DIRTY", 1),
		"a mergeable that is a boolean":  strings.Replace(valid, `"CONFLICTING"`, "true", 1),
		"an empty mergeable":             strings.Replace(valid, `"CONFLICTING"`, `""`, 1),
		"a head of 39 digits":            strings.Replace(valid, head, head[1:], 1),
		"a head of 41 digits":            strings.Replace(valid, head, head+"0", 1),
		"a head with an uppercase digit": strings.Replace(valid, head, "A"+head[1:], 1),
		"a head with a space":            strings.Replace(valid, head, " "+head[1:], 1),
		"a head that is a number":        strings.Replace(valid, `"`+head+`"`, "1", 1),
		"an empty base":                  strings.Replace(valid, `"main"`, `""`, 1),
		"a base that is a number":        strings.Replace(valid, `"main"`, "1", 1),
		"a cross-repository string":      strings.Replace(valid, "false", `"false"`, 1),
		"a cross-repository number":      strings.Replace(valid, "false", "0", 1),
	} {
		rejected[name] = "[" + element(members) + "]"
	}
	// One element of another shape rejects the whole list, the valid
	// element before it included.
	rejected["a valid element and a rejected one"] = "[" + element(valid) + "," + element(strings.Replace(valid, "CONFLICTING", "conflicting", 1)) + "]"
	for name, doc := range rejected {
		if prs, err := parsePRList([]byte(doc), "b"); !errors.Is(err, errPRList) || prs != nil {
			t.Errorf("%s: parsePRList = %+v, %v; want errPRList", name, prs, err)
		}
	}
}

// TestParseCommitID accepts only a full commit ID and gives the zero value
// for anything else.
func TestParseCommitID(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef01234567"
	got, err := ParseCommitID(id)
	if err != nil || got.String() != id || got.IsZero() {
		t.Fatalf("ParseCommitID(%q) = %q, %v", id, got, err)
	}
	for _, s := range []string{"", id[1:], id + "0", strings.ToUpper(id), id + "\n", "HEAD"} {
		if got, err := ParseCommitID(s); !errors.Is(err, errCommitID) || !got.IsZero() {
			t.Errorf("ParseCommitID(%q) = %q, %v; want errCommitID and the zero value", s, got, err)
		}
	}
}

func TestBranch(t *testing.T) {
	for out, want := range map[string]string{"user/0001-x-01\n": "user/0001-x-01", "HEAD\n": ""} {
		rec := newRecorder(t)
		rec.reply("git rev-parse --abbrev-ref HEAD", []byte(out))
		got, err := NewGitReader(rec.commands(nil)).Branch(context.Background(), t.TempDir())
		if err != nil || got != want {
			t.Errorf("Branch(%q) = %q, %v; want %q", out, got, err, want)
		}
	}
}

// TestFindTaskWorkspaces checks the chief and task workspaces are told apart
// by the git common directory: the chief's own workspace and a workspace of
// another repository or of no repository are not task workspaces.
func TestFindTaskWorkspaces(t *testing.T) {
	w := newWorld(t)
	w.add(fakeWorkspace{ID: "w1", Label: "task", Branch: "scratch"})
	// A second pane of the chief's own workspace must not count.
	w.agents = append(w.agents, fakeAgent{Pane: "wc:p2", Workspace: "wc", Kind: "claude", Status: "idle", Dir: w.chiefTree, Seq: 2})
	other := filepath.Join(filepath.Dir(w.chiefTree), "elsewhere")
	notRepo := filepath.Join(filepath.Dir(w.chiefTree), "norepo")
	for _, d := range []string{other, notRepo} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	w.agents = append(w.agents,
		fakeAgent{Pane: "w9:p1", Workspace: "w9", Kind: "claude", Status: "idle", Dir: other, Seq: 3},
		fakeAgent{Pane: "w8:p1", Workspace: "w8", Kind: "claude", Status: "idle", Dir: notRepo, Seq: 4})
	w.labels = append(w.labels, [2]string{"w9", "another repo"}, [2]string{"w8", "no repo"})
	w.git(other, "rev-parse --path-format=absolute --git-common-dir", filepath.Join(filepath.Dir(w.chiefTree), "other.git"))
	if err := os.MkdirAll(filepath.Join(filepath.Dir(w.chiefTree), "other.git"), 0o700); err != nil {
		t.Fatal(err)
	}
	w.rec.failIn(notRepo, "git rev-parse --path-format=absolute --git-common-dir", errors.New("not a git repository"))
	deps := w.finish()
	cmds := deps.Cmds
	herdr := NewHerdr(cmds)
	agents, err := herdr.Agents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	spaces, err := herdr.Workspaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	chiefPane, _ := ParsePaneID(worldChiefPane)
	found, err := FindTaskWorkspaces(context.Background(), NewGitReader(cmds), cmds, agents, spaces, chiefPane)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].ID != "w1" || found[0].Label != "task" || found[0].Worktree != w.trees["w1"] {
		t.Errorf("found = %+v, want only the task workspace w1", found)
	}
	t.Run("the chief pane is not a Herdr agent", func(t *testing.T) {
		gone, _ := ParsePaneID("zz:p9")
		if _, err := FindTaskWorkspaces(context.Background(), NewGitReader(cmds), cmds, agents, spaces, gone); !errors.Is(err, errHerdrOutput) {
			t.Errorf("error = %v, want errHerdrOutput", err)
		}
	})
}

func TestParseWorktreeListSample(t *testing.T) {
	got, err := parseWorktreeList(readTestdata(t, "git/worktree-list-porcelain.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []WorktreeEntry{
		{Path: "/Users/user/src/repo/main"},
		{Path: "/Users/user/src/repo/detached"},
		{Path: "/Users/user/src/repo/gone", Prunable: true},
		{Path: "/Users/user/src/repo/locked", Locked: true},
		{Path: "/Users/user/src/repo/lockednoreason", Locked: true},
		{Path: "/Users/user/src/repo/onbranch"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("parseWorktreeList = %+v, want %+v", got, want)
	}
}

func TestParseWorktreeListRejects(t *testing.T) {
	cases := map[string]string{
		"empty":                  "",
		"a block with no path":   "HEAD 6f1d\n\n",
		"a relative path":        "worktree repo\nHEAD 6f1d\n\n",
		"a quoted path":          "worktree \"/tmp/a\\tb\"\nHEAD 6f1d\n\n",
		"a repeated path":        "worktree /a\nHEAD 6f1d\n\nworktree /a\nHEAD 6f1d\n\n",
		"a second block broken":  "worktree /a\nHEAD 6f1d\n\nbranch refs/heads/x\n\n",
		"no line end at the end": "worktree /a\nHEAD 6f1d",
	}
	for name, out := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := parseWorktreeList([]byte(out)); !errors.Is(err, errGitOutput) {
				t.Errorf("parseWorktreeList = %+v, %v, want errGitOutput", got, err)
			}
		})
	}
}

// TestGitWorktreesOfARealRepository reads the output of the real git for a
// detached, a locked, and a prunable worktree.
func TestGitWorktreesOfARealRepository(t *testing.T) {
	repo := newTestRepo(t)
	base := filepath.Dir(repo.Dir)
	detached, locked, gone := filepath.Join(base, "detached"), filepath.Join(base, "locked"), filepath.Join(base, "gone")
	repo.Git("worktree", "add", "-q", "--detach", detached)
	repo.Git("worktree", "add", "-q", "--detach", locked)
	repo.Git("worktree", "lock", "--reason", "pinned", locked)
	repo.Git("worktree", "add", "-q", "--detach", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	git := NewGitReader(repo.RealCommands())
	got, err := git.Worktrees(t.Context(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]WorktreeEntry{}
	for _, e := range got {
		byPath[e.Path] = e
	}
	want := map[string]WorktreeEntry{
		repo.Dir: {Path: repo.Dir},
		detached: {Path: detached},
		locked:   {Path: locked, Locked: true},
		gone:     {Path: gone, Prunable: true},
	}
	if !maps.Equal(byPath, want) || len(got) != len(want) {
		t.Errorf("Worktrees = %+v, want %+v", got, want)
	}
}

func TestGitIsAncestor(t *testing.T) {
	repo := newTestRepo(t)
	first := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Write("second.txt", "x\n")
	second := repo.CommitAll("second")
	repo.Git("switch", "-q", "-c", "side", first)
	repo.Write("side.txt", "x\n")
	side := repo.CommitAll("side")
	git := NewGitReader(repo.RealCommands())
	ctx := t.Context()
	cases := []struct {
		name                 string
		ancestor, descendant string
		want                 bool
		wantErr              bool
	}{
		{"an ancestor", first, second, true, false},
		{"the same commit", second, second, true, false},
		{"not an ancestor", second, first, false, false},
		{"commits on two branches", second, side, false, false},
		{"a commit that does not exist", strings.Repeat("0", 40), second, false, true},
		{"not a full commit ID", "HEAD", second, false, true},
		{"an option", "--all", second, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := git.IsAncestor(ctx, repo.Dir, c.ancestor, c.descendant)
			if (err != nil) != c.wantErr || got != c.want {
				t.Errorf("IsAncestor = %v, %v, want %v (error %v)", got, err, c.want, c.wantErr)
			}
		})
	}
}

// TestIsAncestorRefusesBeforeRunningGit checks that an argument that is not a
// full commit ID never reaches git, where an option could be read.
func TestIsAncestorRefusesBeforeRunningGit(t *testing.T) {
	rec := newRecorder(t)
	git := NewGitReader(rec.commands(nil))
	for _, bad := range []string{"--all", "HEAD", "-h", strings.Repeat("A", 40), strings.Repeat("0", 39)} {
		if _, err := git.IsAncestor(t.Context(), "/work/tree", bad, strings.Repeat("0", 40)); !errors.Is(err, errGitOutput) {
			t.Errorf("IsAncestor(%q) = %v, want errGitOutput", bad, err)
		}
	}
	if calls := rec.recorded(); len(calls) != 0 {
		t.Errorf("git ran: %v", calls)
	}
}

// TestGitCleanCountsStagedAndNestedFiles runs Clean against the real git: a
// staged change and an untracked file in a nested new directory are not
// clean, and an ignored file is.
func TestGitCleanCountsStagedAndNestedFiles(t *testing.T) {
	repo := newTestRepo(t)
	repo.Write(".gitignore", "ignored/\n")
	repo.CommitAll("ignore")
	git := NewGitReader(repo.RealCommands())
	clean := func() bool {
		t.Helper()
		ok, err := git.Clean(t.Context(), repo.Dir)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !clean() {
		t.Fatal("a clean worktree reads as changed")
	}
	repo.Write("ignored/deep/file.txt", "x\n")
	if !clean() {
		t.Error("a file in an ignored directory reads as a change")
	}
	repo.Write("a/b/c/new.txt", "x\n")
	if clean() {
		t.Error("an untracked file in a nested new directory reads as clean")
	}
	repo.Git("add", "a/b/c/new.txt")
	if clean() {
		t.Error("a staged new file reads as clean")
	}
	repo.Git("reset", "-q")
	if err := os.RemoveAll(filepath.Join(repo.Dir, "a")); err != nil {
		t.Fatal(err)
	}
	repo.Write("README.md", "staged change\n")
	repo.Git("add", "README.md")
	if clean() {
		t.Error("a staged change to a tracked file reads as clean")
	}
}

// TestGitParentsAndHasCommit runs Parents and HasCommit against the real
// git: a merge commit has its two parents in order, a root commit none, and
// a commit the repository lacks is no, not an error; a name that is not a
// full commit ID is refused.
func TestGitParentsAndHasCommit(t *testing.T) {
	repo := newTestRepo(t)
	first := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	repo.Git("switch", "-q", "-c", "side", first)
	repo.Write("side.txt", "x\n")
	side := repo.CommitAll("side")
	repo.Git("switch", "-q", "-")
	repo.Write("main.txt", "x\n")
	main := repo.CommitAll("main")
	repo.Git("merge", "--no-ff", "-q", "-m", "merge", "side")
	merge := strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
	git := NewGitReader(repo.RealCommands())
	ctx := t.Context()
	if got, err := git.Parents(ctx, repo.Dir, merge); err != nil || !slices.Equal(got, []string{main, side}) {
		t.Errorf("Parents(merge) = %v, %v; want [%s %s]", got, err, main, side)
	}
	if got, err := git.Parents(ctx, repo.Dir, first); err != nil || len(got) != 0 {
		t.Errorf("Parents(first) = %v, %v; want none: a root commit", got, err)
	}
	for _, bad := range []string{"HEAD", "--all", strings.Repeat("0", 39)} {
		if _, err := git.Parents(ctx, repo.Dir, bad); !errors.Is(err, errGitOutput) {
			t.Errorf("Parents(%q) = %v, want errGitOutput", bad, err)
		}
		if _, err := git.HasCommit(ctx, repo.Dir, bad); !errors.Is(err, errGitOutput) {
			t.Errorf("HasCommit(%q) = %v, want errGitOutput", bad, err)
		}
	}
	if has, err := git.HasCommit(ctx, repo.Dir, merge); err != nil || !has {
		t.Errorf("HasCommit(merge) = %v, %v", has, err)
	}
	if has, err := git.HasCommit(ctx, repo.Dir, strings.Repeat("0", 40)); err != nil || has {
		t.Errorf("HasCommit(absent) = %v, %v; want false without an error", has, err)
	}
}

// TestFetchBranchRefusesBeforeRunningGit checks that a branch name that
// could be read as an option or a refspec never reaches git.
func TestFetchBranchRefusesBeforeRunningGit(t *testing.T) {
	rec := newRecorder(t)
	git := NewGitReader(rec.commands(nil))
	for _, bad := range []string{"", "-x", "--force", "a b", "a..b", "a//b", "a/", "a.", "a.lock", "a:b", "+a", "a\nb", "a~1", "a^", "a*", "a?", "a[", "a@{1}", "a\\b"} {
		if err := git.FetchBranch(t.Context(), "/work/tree", bad); !errors.Is(err, errGitOutput) {
			t.Errorf("FetchBranch(%q) = %v, want errGitOutput", bad, err)
		}
	}
	if calls := rec.recorded(); len(calls) != 0 {
		t.Errorf("git ran: %v", calls)
	}
}
