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
	return p.finishBody(t, "", steps...)
}

// finishBody finishes with the commit body file bodyFile ("" for none).
func (p prepared) finishBody(t *testing.T, bodyFile string, steps ...cmdrunnertestutil.Step) (FinishResult, *gitRouter, error) {
	t.Helper()
	w, router := newTestWorktree(t, steps...)
	result, err := w.Finish(t.Context(), p.base, p.judged, FinishRequest{PR: mustPR(t), BodyFile: bodyFile})
	return result, router, err
}

// storedMessage reads commit's message as stored in the object, so nothing
// git log would reformat is hidden.
func (p prepared) storedMessage(t *testing.T, commit string) string {
	t.Helper()
	_, message, ok := strings.Cut(p.r.Git("cat-file", "commit", commit), "\n\n")
	if !ok {
		t.Fatalf("commit %s has no message", commit)
	}
	return message
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
	// The fake already holds make fmt, test and lint to their order; the
	// staging and the commit must follow the last of them.
	p := prepareRound(t, fixTestFile)
	_, router, err := p.finish(t, buildCheckSteps()...)
	if err != nil {
		t.Fatalf("Finish = %v", err)
	}
	lint := indexOf(router.Calls, "make lint")
	if add, commit := indexOf(router.Calls, "git add"), indexOf(router.Calls, "git commit"); add < lint || commit < add {
		t.Errorf("staging or commit out of order: %v", router.Calls)
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
		if !slices.Equal(got, names) {
			t.Errorf("committed %q", got)
		}
	})
}

func TestFinishRefusesUnpinnedChange(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(r *testRepo)
	}{
		// A nested repository is listed as a directory with no digest.
		{"nested_repository", func(r *testRepo) {
			r.gitIn(r.Home, "init", "-q", filepath.Join(r.Dir, "docs", "nested"))
		}},
		// A change staged and then restored in the work tree exists only in
		// the index, which a commit would hold.
		{"staged_only", func(r *testRepo) {
			r.Write("docs/notes/b.md", "staged only\n")
			r.Git("add", "docs/notes/b.md")
			r.Write("docs/notes/b.md", "b\n")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := prepareRound(t, tc.edit)
			if got := mustInspect(t, p.base).OutOfScope; len(got) != 1 || got[0].Kind != ChangeOther {
				t.Errorf("OutOfScope = %+v, want one other change", got)
			}
			// No make step is scripted: the refusal comes before them.
			_, _, err := p.finish(t)
			wantJudgedSetChanged(t, err)
		})
	}
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

func TestFinishStopsWhenHooksChangeConfig(t *testing.T) {
	setFsmonitor := "printf '[core]\\n\\tfsmonitor = true\\n' >> .git/config\n"
	for _, tc := range []struct {
		name, hook, script, last string
	}{
		// git add runs post-index-change; nothing after it may run git.
		{"after_add", "post-index-change", setFsmonitor, "git add"},
		// A commit whose hooks changed the config is not checked with git.
		{"after_commit", "pre-commit", setFsmonitor, "git commit"},
		// A failing commit is not undone with git once the config changed.
		{"after_failed_commit", "pre-commit", setFsmonitor + "exit 1\n", "git commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			r.WriteHook(tc.hook, tc.script)
			base := mustBaseline(t)
			fixTestFile(r)
			p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
			result, router, err := p.finish(t, buildCheckSteps()...)
			wantStateParts(t, err, PartConfig)
			if result.Commit != "" {
				t.Errorf("Finish returned commit %s", result.Commit)
			}
			// The hook's change is seen by git config --list, and nothing runs
			// after it.
			n := len(router.Calls)
			if n < 2 || router.Calls[n-2] != tc.last || router.Calls[n-1] != gitConfigList {
				t.Errorf("calls = %v, want %q then git config --list last", router.Calls, tc.last)
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
	if got := p.storedMessage(t, result.Commit); got != "fix: address PR #"+testPR+" review comments\n" {
		t.Errorf("message = %q, want the subject alone", got)
	}
	if got := strings.TrimSpace(p.r.Git("rev-parse", result.Commit+"^")); got != p.base.head {
		t.Errorf("parent = %s, want the baseline's HEAD", got)
	}
}

// correction is a commit body line checkCommitBody accepts.
const correction = "Editorial correction: docs/tasks/0001_x/01_requirements.md: fixed a stale name; no decision changed"

// subjectAndBody is the message finish builds for body: the subject, an empty
// line, and body, which the caller gives ending in one LF.
func subjectAndBody(body string) string {
	return "fix: address PR #" + testPR + " review comments\n\n" + body
}

func TestFinishCommitBody(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{name: "ends_with_lf", body: correction + "\n", want: subjectAndBody(correction + "\n")},
		{name: "no_trailing_lf", body: correction + "\n" + correction, want: subjectAndBody(correction + "\n" + correction + "\n")},
		// git's default cleanup would strip the trailing spaces.
		{name: "trailing_spaces", body: correction + "  \n", want: subjectAndBody(correction + "  \n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := prepareRound(t, fixTestFile)
			result, _, err := p.finishBody(t, writeTextFile(t, tc.body), buildCheckSteps()...)
			if err != nil || !result.Committed {
				t.Fatalf("Finish = %+v, %v", result, err)
			}
			if got := p.storedMessage(t, result.Commit); got != tc.want {
				t.Errorf("message = %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("file_rewritten_during_build_checks", func(t *testing.T) {
		p := prepareRound(t, fixTestFile)
		file := writeTextFile(t, correction+"\n")
		rewrite := func() error { return os.WriteFile(file, []byte("Editorial correction: rewritten\n"), 0o600) }
		result, _, err := p.finishBody(t, file, makeStep(StageFmt, rewrite), makeStep(StageTest, nil), makeStep(StageLint, nil))
		if err != nil || !result.Committed {
			t.Fatalf("Finish = %+v, %v", result, err)
		}
		if got := p.storedMessage(t, result.Commit); got != subjectAndBody(correction+"\n") {
			t.Errorf("message = %q, want the body that was checked", got)
		}
	})
}

// TestFinishRejectsBodyFile: a body file finish refuses stops the run before
// any make target (the fake has no step) and before any commit.
func TestFinishRejectsBodyFile(t *testing.T) {
	for name, body := range map[string]string{
		"mention":             correction + " @codex\n",
		"other_line":          correction + "\nSigned-off-by: x\n",
		"no_description":      "Editorial correction:  \n",
		"at_sign":             correction + " a@b\n",
		"carriage_return":     correction + "\r\n",
		"escape":              correction + " \x1b[31m\n",
		"line_separator":      correction + " a\u2028b\n",
		"empty_line":          correction + "\n\n" + correction + "\n",
		"two_trailing_lfs":    correction + "\n\n",
		"rejected_as_a_file":  "",
		"leading_empty_line":  "\n" + correction + "\n",
		"prefix_not_at_start": " " + correction + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := prepareRound(t, fixTestFile)
			head := p.r.Head()
			_, _, err := p.finishBody(t, writeTextFile(t, body))
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Finish = %v, want ErrInvalidInput", err)
			}
			p.wantNoCommit(t, head)
		})
	}
	t.Run("checked_before_the_work_tree", func(t *testing.T) {
		// The work tree no longer matches the judged set, so a comparison
		// made first would stop with a judged-set change instead.
		p := prepareRound(t, fixTestFile)
		p.r.Write("docs/notes/late.md", "x\n")
		_, _, err := p.finishBody(t, writeTextFile(t, "not a correction\n"))
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Finish = %v, want ErrInvalidInput", err)
		}
	})
	t.Run("file_inside_the_work_tree", func(t *testing.T) {
		// A valid body written in the work tree is a change made after the
		// judgment.
		p := prepareRound(t, fixTestFile)
		head := p.r.Head()
		p.r.Write("body.txt", correction+"\n")
		_, _, err := p.finishBody(t, filepath.Join(p.r.Dir, "body.txt"))
		wantJudgedSetChanged(t, err)
		p.wantNoCommit(t, head)
	})
}

// TestFinishDetectsRewrittenMessage: a commit-msg hook, in place before the
// baseline so the state checks pass, appends a trailer. With a body the
// stored message is not the one finish built, so it stops without a SHA;
// without one the message is not compared, as before body files existed.
func TestFinishDetectsRewrittenMessage(t *testing.T) {
	prepare := func(t *testing.T) prepared {
		r := newTestRepo(t)
		r.WriteHook("commit-msg", "printf 'Signed-off-by: hook\\n' >> \"$1\"\n")
		base := mustBaseline(t)
		fixTestFile(r)
		return prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
	}
	t.Run("with_body", func(t *testing.T) {
		p := prepare(t)
		result, _, err := p.finishBody(t, writeTextFile(t, correction+"\n"), buildCheckSteps()...)
		wantStateParts(t, err, PartHead)
		if result.Commit != "" {
			t.Errorf("Finish returned commit %s", result.Commit)
		}
	})
	t.Run("without_body", func(t *testing.T) {
		p := prepare(t)
		result := p.mustFinish(t)
		if got := p.storedMessage(t, result.Commit); !strings.HasSuffix(got, "Signed-off-by: hook\n") {
			t.Errorf("message = %q, want the hook's trailer", got)
		}
	})
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
		name     string
		edit     func(t *testing.T, p prepared, commit string)
		parts    []string
		listOnly bool // the change is seen before any git but git config --list runs
	}{
		{"head_is_not_the_commit", func(_ *testing.T, p prepared, _ string) {
			p.r.Write("docs/notes/extra.md", "x\n")
			p.r.CommitAll("after finish")
		}, []string{PartHead}, false},
		{"hooks_changed", func(_ *testing.T, p prepared, _ string) { p.r.WriteHook("pre-push", "exit 0\n") }, []string{PartHooks}, false},
		{"config_changed", func(_ *testing.T, p prepared, _ string) {
			p.r.Git("config", "remote."+testRemote+".receivepack", "evil")
		}, []string{PartConfig}, true},
		{"remote_branch_moved", func(_ *testing.T, p prepared, commit string) {
			p.r.Git("push", "-q", testRemote, commit+":"+headsPrefix+testBranch)
		}, []string{PartRemote}, false},
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
			if tc.listOnly && !slices.Equal(router.Calls, []string{gitConfigList}) {
				t.Errorf("calls after the config changed = %v, want git config --list only", router.Calls)
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
		_, _, err := p.finish(t)
		// No make step is scripted: any make call fails the test.
		wantJudgedSetChanged(t, err)
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
}

// TestRoundIgnoresOtherBranchConfig: another session working in its own
// worktree adds a branch, pushes it, and removes both between inspect,
// finish and push. Each step rewrites the shared configuration's branch
// settings of that branch only, and none stops the round.
func TestRoundIgnoresOtherBranchConfig(t *testing.T) {
	r := newTestRepo(t)
	base := mustBaseline(t)
	other := filepath.Join(r.Home, "other")
	const branch = "x"
	elsewhere := func(step func()) {
		t.Helper()
		before := r.ConfigList()
		step()
		if r.ConfigList() == before {
			t.Fatal("the step does not change the listing")
		}
	}
	elsewhere(func() { r.AddWorktree(other, branch) })
	fixTestFile(r)
	p := prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
	elsewhere(func() { r.gitIn(other, "push", "-q", "-u", testRemote, branch) })
	commit := p.mustFinish(t).Commit
	elsewhere(func() {
		r.Git("worktree", "remove", other)
		r.Git("branch", "-d", branch)
	})
	w, _ := newTestWorktree(t)
	if _, err := w.Push(t.Context(), base, commit); err != nil {
		t.Fatalf("Push = %v", err)
	}
	if got := r.RemoteHead(); got != commit {
		t.Errorf("remote = %s, want %s", got, commit)
	}
}

// prepareIgnoringRound is prepareRound in a repository that ignores build/,
// where the caller writes a target file later.
func prepareIgnoringRound(t *testing.T) prepared {
	t.Helper()
	r := ignoringRepo(t, "build/")
	base := mustBaseline(t)
	fixTestFile(r)
	return prepared{r: r, base: base, judged: mustInspect(t, base).JudgedSet}
}

// TestFinishStopsOnIgnoredChange: a target file written before finish stops
// it before any build check, and one written by make test stops it before
// make lint; neither commits. The make fake fails any target it has no step
// for.
func TestFinishStopsOnIgnoredChange(t *testing.T) {
	t.Run("before_finish", func(t *testing.T) {
		p := prepareIgnoringRound(t)
		head := p.r.Head()
		p.r.Write("build/x_test.go", "package x\n")
		_, router, err := p.finish(t)
		wantStateParts(t, err, PartIgnored)
		if slices.ContainsFunc(router.Calls, func(c string) bool { return strings.HasPrefix(c, makeCommand+" ") }) {
			t.Errorf("ran a build check: %v", router.Calls)
		}
		p.wantNoCommit(t, head)
	})
	t.Run("written_by_make_test", func(t *testing.T) {
		p := prepareIgnoringRound(t)
		head := p.r.Head()
		write := func() error {
			p.r.Write("build/x_test.go", "package x\n")
			return nil
		}
		_, _, err := p.finish(t, makeStep(StageFmt, nil), makeStep(StageTest, write))
		wantStateParts(t, err, PartIgnored)
		p.wantNoCommit(t, head)
	})
}

// TestPushStopsOnIgnoredChange: a target file written after finish stops
// push before it pushes.
func TestPushStopsOnIgnoredChange(t *testing.T) {
	p := prepareIgnoringRound(t)
	commit := p.mustFinish(t).Commit
	remote := p.r.RemoteHead()
	p.r.Write("build/x_test.go", "package x\n")
	w, router := newTestWorktree(t)
	_, err := w.Push(t.Context(), p.base, commit)
	wantStateParts(t, err, PartIgnored)
	if indexOf(router.Calls, "git push") >= 0 {
		t.Errorf("pushed: %v", router.Calls)
	}
	if got := p.r.RemoteHead(); got != remote {
		t.Errorf("remote moved to %s", got)
	}
}
