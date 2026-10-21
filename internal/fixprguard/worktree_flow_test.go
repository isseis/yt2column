//go:build test

package fixprguard

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// prepared is a round that has fixed testFile and been inspected.
type prepared struct {
	r      *testRepo
	base   Baseline
	judged JudgedSetID
}

// prepareRound takes a baseline, applies edit (the fix phase), and inspects.
func prepareRound(t *testing.T, edit func(r *testRepo)) prepared {
	t.Helper()
	r := newTestRepo(t)
	base := mustBaseline(t)
	edit(r)
	return prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
}

func fixTestFile(r *testRepo) { r.Write(testFile, "fixed\n") }

func (p prepared) finish(t *testing.T, steps ...cmdrunnertestutil.Step) (FinishResult, *gitRouter, error) {
	t.Helper()
	w, router := newTestWorktree(t, steps...)
	result, err := w.Finish(t.Context(), p.base, p.judged, FinishRequest{PR: mustPR(t)})
	return result, router, err
}

// mustFinish finishes with build checks that change nothing.
func (p prepared) mustFinish(t *testing.T) FinishResult {
	t.Helper()
	result, _, err := p.finish(t, buildCheckSteps()...)
	if err != nil || !result.Committed {
		t.Fatalf("Finish = %+v, %v", result, err)
	}
	return result
}

func wantJudgedSetChanged(t *testing.T, err error) *JudgedSetError {
	t.Helper()
	judgedErr, ok := errors.AsType[*JudgedSetError](err)
	if !ok || !errors.Is(err, ErrJudgedSetChanged) {
		t.Fatalf("err = %v, want a *JudgedSetError", err)
	}
	return judgedErr
}

// wantNoCommit fails when HEAD moved from the baseline's commit.
func (p prepared) wantNoCommit(t *testing.T, head string) {
	t.Helper()
	if got := p.r.Head(); got != head {
		t.Errorf("HEAD moved to %s", got)
	}
}

// indexOf returns where call is in calls, or -1.
func indexOf(calls []string, call string) int { return slices.Index(calls, call) }

func TestFinishOrder(t *testing.T) {
	p := prepareRound(t, fixTestFile)
	_, router, err := p.finish(t, buildCheckSteps()...)
	if err != nil {
		t.Fatalf("Finish = %v", err)
	}
	order := []string{"git diff", "make fmt", "make test", "make lint", "git add", "git commit"}
	last := -1
	for _, call := range order {
		i := slices.IndexFunc(router.Calls[last+1:], func(c string) bool { return c == call })
		if i < 0 {
			t.Fatalf("%q missing or out of order in %v", call, router.Calls)
		}
		last += i + 1
	}
}

func TestFinishAbsorbsFormatting(t *testing.T) {
	p := prepareRound(t, func(r *testRepo) { r.Write("internal/x/x.go", "package  x\n") })
	format := func() error {
		p.r.Write("internal/x/x.go", "package x\n\n// F is formatted.\nfunc F() {}\n")
		return nil
	}
	w, _ := newTestWorktree(t, makeStep(StageFmt, format), makeStep(StageTest, nil), makeStep(StageLint, nil))
	result, err := w.Finish(t.Context(), p.base, mustInspectPaths(t, p.base, "internal/x/x.go"), FinishRequest{PR: mustPR(t)})
	if err != nil || !result.Committed {
		t.Fatalf("Finish = %+v, %v", result, err)
	}
	if got := p.r.Git("show", "HEAD:internal/x/x.go"); !strings.Contains(got, "F is formatted") {
		t.Errorf("commit holds %q, want the formatted content", got)
	}
}

// mustInspectPaths inspects with a thread on path.
func mustInspectPaths(t *testing.T, base Baseline, path string) JudgedSetID {
	t.Helper()
	w, _ := newTestWorktree(t)
	inspection, err := w.Inspect(t.Context(), base, threadAt(path))
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	return inspection.JudgedSet
}

func TestFinishStopsOnJudgedSetMismatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(t *testing.T, r *testRepo)
	}{
		{"untracked_added", func(_ *testing.T, r *testRepo) { r.Write("docs/notes/late.md", "x\n") }},
		{"content_changed", func(_ *testing.T, r *testRepo) { r.Write(testFile, "fixed again\n") }},
		{"deleted", func(t *testing.T, r *testRepo) {
			if err := os.Remove(filepath.Join(r.Dir, "docs/notes/b.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"mode_changed", func(t *testing.T, r *testRepo) { chmod(t, filepath.Join(r.Dir, testFile), 0o700) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := prepareRound(t, fixTestFile)
			head := p.r.Head()
			tc.edit(t, p.r)
			// No make step is scripted: any make call fails the test.
			_, _, err := p.finish(t)
			wantJudgedSetChanged(t, err)
			p.wantNoCommit(t, head)
		})
	}
}

func TestFinishStopsWhenBuildChecksRewrite(t *testing.T) {
	p := prepareRound(t, fixTestFile)
	head := p.r.Head()
	rewrite := func() error { p.r.Write(testFile, "rewritten by a test\n"); return nil }
	_, router, err := p.finish(t, makeStep(StageFmt, nil), makeStep(StageTest, rewrite), makeStep(StageLint, nil))
	judgedErr := wantJudgedSetChanged(t, err)
	if len(judgedErr.Changes) != 1 || judgedErr.Changes[0].Path != testFile {
		t.Errorf("Changes = %+v, want %s", judgedErr.Changes, testFile)
	}
	if indexOf(router.Calls, "git add") >= 0 {
		t.Errorf("staged after the build checks rewrote a file: %v", router.Calls)
	}
	p.wantNoCommit(t, head)
	t.Run("oversized_file", func(t *testing.T) {
		// Over the line-count cap the file is out of scope, but its content
		// is still pinned.
		big := strings.Repeat("x", maxDigestBytes+1)
		p := prepareRound(t, func(r *testRepo) { r.Write("docs/notes/big.txt", big) })
		rewrite := func() error { p.r.Write("docs/notes/big.txt", "y"+big[1:]); return nil }
		_, _, err := p.finish(t, makeStep(StageFmt, nil), makeStep(StageTest, rewrite), makeStep(StageLint, nil))
		wantJudgedSetChanged(t, err)
	})
	t.Run("formatting_changes_the_paths", func(t *testing.T) {
		p := prepareRound(t, fixTestFile)
		addFile := func() error { p.r.Write("docs/notes/fmt.md", "x\n"); return nil }
		// make test and lint are not scripted: the run stops after fmt.
		_, _, err := p.finish(t, makeStep(StageFmt, addFile))
		wantJudgedSetChanged(t, err)
	})
}

func TestFinishStagesJudgedPathsOnly(t *testing.T) {
	t.Run("pathspec_magic_names", func(t *testing.T) {
		// Taken as pathspecs, ":!a.md" excludes a.md (staging everything
		// else) and ":(top)b.md" names a missing b.md at the top, which fails
		// the add; only a literal add stages these files as named.
		names := []string{":!a.md", ":(top)b.md", "docs/notes/[ab].md"}
		p := prepareRound(t, func(r *testRepo) {
			for _, n := range names {
				r.Write(n, "x\n")
			}
		})
		// The strict check alone does not stop this set: it is exactly what
		// inspect judged.
		result := p.mustFinish(t)
		got := strings.Split(strings.TrimSpace(p.r.Git("diff-tree", "-r", "--name-only", "--no-commit-id", p.base.head, result.Commit)), "\n")
		if !slices.Equal(got, []string{":!a.md", ":(top)b.md", "docs/notes/[ab].md"}) {
			t.Errorf("committed %q", got)
		}
	})
	t.Run("staged_change_outside_the_judged_set", func(t *testing.T) {
		// A change only in the index is invisible to the work tree
		// enumeration; staging the judged paths alone leaves it, and the
		// check of the index finds it.
		p := prepareRound(t, func(r *testRepo) {
			r.Write("docs/notes/b.md", "staged only\n")
			r.Git("add", "docs/notes/b.md")
			r.Write("docs/notes/b.md", "b\n")
			fixTestFile(r)
		})
		head := p.r.Head()
		_, _, err := p.finish(t, buildCheckSteps()...)
		wantJudgedSetChanged(t, err)
		p.wantNoCommit(t, head)
	})
}

func TestFinishRefusesUnpinnedChange(t *testing.T) {
	// A nested repository is listed as a directory with no content digest.
	p := prepareRound(t, func(r *testRepo) {
		r.gitIn(r.Home, "init", "-q", filepath.Join(r.Dir, "docs", "nested"))
	})
	_, _, err := p.finish(t)
	wantJudgedSetChanged(t, err)
}

func TestFinishStageRollback(t *testing.T) {
	r := newTestRepo(t)
	// git add runs this hook; the file it leaves behind makes the index check
	// fail after staging.
	r.WriteHook("post-index-change", "echo x > docs/notes/hook.md\n")
	base := mustBaseline(t)
	fixTestFile(r)
	p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
	head := r.Head()
	_, _, err := p.finish(t, buildCheckSteps()...)
	wantJudgedSetChanged(t, err)
	p.wantNoCommit(t, head)
	if staged := r.Git("diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("staging not undone: %q", staged)
	}
	if got, _ := os.ReadFile(filepath.Join(r.Dir, testFile)); string(got) != "fixed\n" {
		t.Errorf("work tree changed: %q", got)
	}
}

func TestFinishPostCommitTreeCheck(t *testing.T) {
	r := newTestRepo(t)
	// The hook stages a change the judged set does not hold.
	r.WriteHook("pre-commit", "echo hook >> docs/notes/b.md\ngit add docs/notes/b.md\n")
	base := mustBaseline(t)
	fixTestFile(r)
	p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
	result, router, err := p.finish(t, buildCheckSteps()...)
	wantJudgedSetChanged(t, err)
	if result.Commit != "" {
		t.Errorf("Finish returned commit %s", result.Commit)
	}
	if indexOf(router.Calls, "git commit") < 0 {
		t.Fatalf("no commit was made: %v", router.Calls)
	}
	t.Run("commit_inserted_by_hook", func(t *testing.T) {
		// After the commit the hook moves HEAD to a commit of its own on top,
		// with the same tree, so only the parent shows it.
		r := newTestRepo(t)
		r.WriteHook("post-commit", "c=$(git commit-tree HEAD^{tree} -p HEAD -m inserted) && git update-ref HEAD \"$c\"\n")
		base := mustBaseline(t)
		fixTestFile(r)
		p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
		result, _, err := p.finish(t, buildCheckSteps()...)
		wantStateParts(t, err, PartHead)
		if result.Commit != "" {
			t.Errorf("Finish returned commit %s", result.Commit)
		}
	})
	t.Run("clean_filter", func(t *testing.T) {
		// The filter is configured before the baseline, so the state checks
		// pass; only the committed blob differs from the work tree.
		r := newTestRepo(t)
		r.Git("config", "filter.x.clean", "sed s/fixed/EVIL/")
		base := mustBaseline(t)
		fixTestFile(r)
		r.Write(".gitattributes", "*.md filter=x\n")
		p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
		result, _, err := p.finish(t, buildCheckSteps()...)
		wantJudgedSetChanged(t, err)
		if result.Commit != "" {
			t.Errorf("Finish returned commit %s", result.Commit)
		}
	})
}

func TestFinishChecksStateAfterCommit(t *testing.T) {
	r := newTestRepo(t)
	r.WriteHook("pre-commit", "printf '[core]\\n\\tfsmonitor = true\\n' >> .git/config\n")
	base := mustBaseline(t)
	fixTestFile(r)
	p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
	result, router, err := p.finish(t, buildCheckSteps()...)
	wantStateParts(t, err, PartConfig)
	if result.Commit != "" {
		t.Errorf("Finish returned commit %s", result.Commit)
	}
	if last := router.Calls[len(router.Calls)-1]; last != "git commit" {
		t.Errorf("git ran after the commit hook changed the config: %v", router.Calls)
	}
}

func TestFinishStopsWhenHooksChangeConfig(t *testing.T) {
	setFsmonitor := "printf '[core]\\n\\tfsmonitor = true\\n' >> .git/config\n"
	for _, tc := range []struct {
		name, hook, script, last string
	}{
		// git add runs post-index-change; nothing after it may run git.
		{"after_add", "post-index-change", setFsmonitor, "git add"},
		// A failing commit is not undone with git once the config changed.
		{"after_failed_commit", "pre-commit", setFsmonitor + "exit 1\n", "git commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			r.WriteHook(tc.hook, tc.script)
			base := mustBaseline(t)
			fixTestFile(r)
			p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
			_, router, err := p.finish(t, buildCheckSteps()...)
			wantStateParts(t, err, PartConfig)
			if last := router.Calls[len(router.Calls)-1]; last != tc.last {
				t.Errorf("last command = %q, want %q: %v", last, tc.last, router.Calls)
			}
		})
	}
}

func TestFinishBuildCheckRunnerFailure(t *testing.T) {
	// make that did not run is a build check failure without output.
	p := prepareRound(t, fixTestFile)
	step := makeStep(StageFmt, nil)
	step.Err = errors.New("exec: not found")
	_, _, err := p.finish(t, step)
	if !errors.Is(err, ErrBuildChecksFailed) {
		t.Fatalf("Finish = %v, want ErrBuildChecksFailed", err)
	}
	if _, ok := errors.AsType[*BuildCheckError](err); ok {
		t.Errorf("a runner failure carries target output: %v", err)
	}
}

func TestFinishCommitSubject(t *testing.T) {
	p := prepareRound(t, fixTestFile)
	result := p.mustFinish(t)
	if got := strings.TrimSpace(p.r.Git("log", "-1", "--format=%s", result.Commit)); got != "fix: address PR #"+testPR+" review comments" {
		t.Errorf("subject = %q", got)
	}
	if got := strings.TrimSpace(p.r.Git("rev-parse", result.Commit+"^")); got != p.base.head {
		t.Errorf("parent = %s, want the baseline's HEAD", got)
	}
}

func TestFinishEmptyJudgedSet(t *testing.T) {
	p := prepareRound(t, func(*testRepo) {})
	head := p.r.Head()
	result, _, err := p.finish(t)
	if err != nil || result.Committed || result.Commit != "" {
		t.Errorf("Finish = %+v, %v; want nothing committed", result, err)
	}
	p.wantNoCommit(t, head)
}

func TestFinishBuildCheckFailureOutput(t *testing.T) {
	p := prepareRound(t, fixTestFile)
	head := p.r.Head()
	stdout := strings.Repeat("o", maxBuildOutputTail)
	failing := makeStep(StageTest, nil)
	failing.Err = &cmdrunner.ExitError{Code: 2, Stdout: []byte(stdout), Stderr: []byte("FAIL tail")}
	_, _, err := p.finish(t, makeStep(StageFmt, nil), failing)
	buildErr, ok := errors.AsType[*BuildCheckError](err)
	if !ok || !errors.Is(err, ErrBuildChecksFailed) || buildErr.Stage != StageTest {
		t.Fatalf("Finish = %v, want a test stage *BuildCheckError", err)
	}
	text := buildErr.Output.Text
	if !strings.HasPrefix(text, fenceBeginPrefix+buildErr.Output.Nonce+"\n") || !strings.HasSuffix(text, fenceEndPrefix+buildErr.Output.Nonce+"\n") {
		t.Errorf("output is not fenced: %.80q", text)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(text, fenceBeginPrefix+buildErr.Output.Nonce+"\n"), "\n"+fenceEndPrefix+buildErr.Output.Nonce+"\n")
	if len(inner) != maxBuildOutputTail || !strings.HasSuffix(inner, "FAIL tail") {
		t.Errorf("output tail is %d bytes ending %q", len(inner), inner[max(0, len(inner)-12):])
	}
	p.wantNoCommit(t, head)
}

func TestFinishAfterOutOfScope(t *testing.T) {
	p := prepareRound(t, func(r *testRepo) {
		fixTestFile(r)
		r.Write("internal/y/notes.md", "out of scope\n")
	})
	w, _ := newTestWorktree(t)
	inspection, err := w.Inspect(t.Context(), p.base, threadAt(testFile))
	if err != nil || len(inspection.OutOfScope) != 1 || inspection.JudgedSet != p.judged {
		t.Fatalf("Inspect = %+v, %v; want one change out of scope", inspection, err)
	}
	// The user answered to go on.
	result := p.mustFinish(t)
	if got := p.r.Git("show", "--name-only", "--format=", result.Commit); !strings.Contains(got, "internal/y/notes.md") {
		t.Errorf("commit holds %q, want the out-of-scope file", got)
	}
}

func TestInspectLeavesWorkTree(t *testing.T) {
	r := newTestRepo(t)
	base := mustBaseline(t)
	fixTestFile(r)
	r.Write("internal/y/notes.md", "out of scope\n")
	r.Write("docs/notes/b.md", "staged\n")
	r.Git("add", "docs/notes/b.md")
	before := r.Git("status", "--porcelain=v1", "--untracked-files=all")
	index, err := os.ReadFile(filepath.Join(r.Dir, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	mustInspect(t, base)
	if after := r.Git("status", "--porcelain=v1", "--untracked-files=all"); after != before {
		t.Errorf("status changed from %q to %q", before, after)
	}
	if got, _ := os.ReadFile(filepath.Join(r.Dir, ".git", "index")); !slices.Equal(got, index) {
		t.Errorf("inspect rewrote the index")
	}
}

func TestPushRechecks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(t *testing.T, p prepared, commit string)
		parts []string
	}{
		{"head_is_not_the_commit", func(_ *testing.T, p prepared, _ string) {
			p.r.Write("docs/notes/extra.md", "x\n")
			p.r.CommitAll("after finish")
		}, []string{PartHead}},
		{"hooks_changed", func(_ *testing.T, p prepared, _ string) { p.r.WriteHook("pre-push", "exit 0\n") }, []string{PartHooks}},
		{"config_changed", func(_ *testing.T, p prepared, _ string) {
			p.r.Git("config", "remote."+testRemote+".receivepack", "evil")
		}, []string{PartConfig}},
		{"remote_branch_moved", func(_ *testing.T, p prepared, commit string) {
			p.r.Git("push", "-q", testRemote, commit+":"+headsPrefix+testBranch)
		}, []string{PartRemote}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := prepareRound(t, fixTestFile)
			commit := p.mustFinish(t).Commit
			tc.edit(t, p, commit)
			remote := p.r.RemoteHead()
			w, router := newTestWorktree(t)
			_, err := w.Push(t.Context(), p.base, commit)
			wantStateParts(t, err, tc.parts...)
			if indexOf(router.Calls, "git push") >= 0 {
				t.Errorf("pushed: %v", router.Calls)
			}
			// A changed config stops the push before any git runs.
			if n := gitCalls(router.Calls); tc.parts[0] == PartConfig && n != 0 {
				t.Errorf("ran git %d times after the config changed: %v", n, router.Calls)
			}
			if got := p.r.RemoteHead(); got != remote {
				t.Errorf("remote moved to %s", got)
			}
		})
	}
	t.Run("commit_not_on_the_baseline", func(t *testing.T) {
		// HEAD is the given SHA, but it is not a child of the baseline's HEAD.
		p := prepareRound(t, fixTestFile)
		p.mustFinish(t)
		p.r.Write("docs/notes/extra.md", "x\n")
		p.r.CommitAll("second")
		w, _ := newTestWorktree(t)
		_, err := w.Push(t.Context(), p.base, p.r.Head())
		wantStateParts(t, err, PartHead)
	})
}

func TestPushExplicitRefspec(t *testing.T) {
	r := newTestRepo(t)
	// A configured push refspec would send the branch elsewhere, and
	// followTags would send a tag along.
	r.Git("config", "remote."+testRemote+".push", headsPrefix+"*:"+headsPrefix+"other/*")
	r.Git("config", "push.followTags", "true")
	base := mustBaseline(t)
	fixTestFile(r)
	p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
	commit := p.mustFinish(t).Commit
	r.Git("tag", "-a", "-m", "agent tag", "agent-tag", commit)
	w, _ := newTestWorktree(t)
	result, err := w.Push(t.Context(), base, commit)
	if err != nil || !result.Pushed || result.Commit != commit {
		t.Fatalf("Push = %+v, %v", result, err)
	}
	if got := r.RemoteHead(); got != commit {
		t.Errorf("remote branch = %s, want %s", got, commit)
	}
	if refs := r.gitIn(r.Remote, "for-each-ref", "--format=%(refname)", headsPrefix+"other/", "refs/tags/"); refs != "" {
		t.Errorf("pushed more than the branch: %q", refs)
	}
}

func TestPushFailure(t *testing.T) {
	p := prepareRound(t, fixTestFile)
	commit := p.mustFinish(t).Commit
	hooks := filepath.Join(p.r.Remote, "hooks")
	if err := os.MkdirAll(hooks, testDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-receive"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil { //nolint:gosec // a hook of the test's bare repository
		t.Fatal(err)
	}
	w, _ := newTestWorktree(t)
	if _, err := w.Push(t.Context(), p.base, commit); !errors.Is(err, ErrGitFailed) {
		t.Errorf("Push = %v, want ErrGitFailed", err)
	}
	for _, bad := range []string{"", "HEAD", strings.Repeat("A", sha1OIDLength)} {
		if _, err := w.Push(t.Context(), p.base, bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Push(%q) = %v, want ErrInvalidInput", bad, err)
		}
	}
}

func TestRoundStopsAtRightPlace(t *testing.T) {
	t.Run("file_added_after_inspect", func(t *testing.T) {
		p := prepareRound(t, fixTestFile)
		p.r.Write("docs/notes/while_asking.md", "x\n")
		_, router, err := p.finish(t)
		wantJudgedSetChanged(t, err)
		if slices.ContainsFunc(router.Calls, func(c string) bool { return strings.HasPrefix(c, makeCommand) }) {
			t.Errorf("make ran: %v", router.Calls)
		}
	})
	t.Run("hook_added_during_fix", func(t *testing.T) {
		r := newTestRepo(t)
		base := mustBaseline(t)
		fixTestFile(r)
		r.WriteHook("pre-commit", "exit 0\n")
		w, _ := newTestWorktree(t)
		_, err := w.Inspect(t.Context(), base, threadAt(testFile))
		wantStateParts(t, err, PartHooks)
	})
	t.Run("remote_moved_before_inspect", func(t *testing.T) {
		r := newTestRepo(t)
		base := mustBaseline(t)
		fixTestFile(r)
		r.Git("stash", "-q")
		r.Write("docs/notes/b.md", "pushed elsewhere\n")
		r.CommitAll("elsewhere")
		r.Git("push", "-q", testRemote, "HEAD:"+headsPrefix+testBranch)
		r.Git("reset", "-q", "--hard", "HEAD~1")
		r.Git("stash", "pop", "-q")
		w, _ := newTestWorktree(t)
		_, err := w.Inspect(t.Context(), base, threadAt(testFile))
		wantStateParts(t, err, PartRemote)
	})
	t.Run("replaced_base_commit", func(t *testing.T) {
		// The agent stages a Makefile and replaces the baseline's commit
		// with one whose tree holds it, so a diff through replace refs
		// would not show the Makefile.
		r := newTestRepo(t)
		base := mustBaseline(t)
		r.Write("Makefile", "all:\n\techo EVIL\n")
		r.Git("add", "Makefile")
		tree := strings.TrimSpace(r.Git("write-tree"))
		forged := strings.TrimSpace(r.Git("commit-tree", tree, "-m", "initial"))
		r.Git("replace", base.head, forged)
		fixTestFile(r)
		got := mustInspect(t, base).OutOfScope
		if !slices.ContainsFunc(got, func(c ScopedChange) bool { return c.Path == "Makefile" }) {
			t.Errorf("OutOfScope = %+v, want the Makefile", got)
		}
	})
	t.Run("out_of_scope_change", func(t *testing.T) {
		r := newTestRepo(t)
		base := mustBaseline(t)
		fixTestFile(r)
		r.Write("internal/x/x.go", "package x\n\nfunc F() {}\n")
		if got := mustInspect(t, base).OutOfScope; len(got) != 1 || got[0].Path != "internal/x/x.go" {
			t.Errorf("OutOfScope = %+v, want internal/x/x.go", got)
		}
	})
	t.Run("build_checks_rewrite_in_scope_file", func(t *testing.T) {
		p := prepareRound(t, fixTestFile)
		rewrite := func() error { p.r.Write("docs/notes/b.md", "rewritten\n"); return nil }
		_, router, err := p.finish(t, makeStep(StageFmt, nil), makeStep(StageTest, nil), makeStep(StageLint, rewrite))
		wantJudgedSetChanged(t, err)
		if indexOf(router.Calls, "git add") >= 0 {
			t.Errorf("staged: %v", router.Calls)
		}
	})
	t.Run("clean_round_reaches_push", func(t *testing.T) {
		p := prepareRound(t, fixTestFile)
		commit := p.mustFinish(t).Commit
		w, _ := newTestWorktree(t)
		if _, err := w.Push(t.Context(), p.base, commit); err != nil {
			t.Fatalf("Push = %v", err)
		}
		if got := p.r.RemoteHead(); got != commit {
			t.Errorf("remote = %s, want %s", got, commit)
		}
	})
}
