//go:build test

package fixprguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// reviewRound is a Phase 4.5 round: the baseline is taken on a commit that is
// not the repository's first, so a line that misses it still has commits to
// walk; phase4 is the commit on top of it, pushed as Phase 4 pushes it.
type reviewRound struct {
	r      *testRepo
	base   Baseline
	phase4 string
}

// newReviewRound builds the round on r, with n review commits after the
// Phase 4 commit.
func newReviewRound(t *testing.T, r *testRepo, n int) reviewRound {
	t.Helper()
	r.Write("docs/notes/base.md", "base\n")
	r.CommitAll("base")
	r.Git("push", "-q", testRemote, "HEAD:"+headsPrefix+testBranch)
	base := mustBaseline(t)
	r.Write(testFile, "phase 4\n")
	r.CommitAll("phase 4")
	phase4 := r.Head()
	r.Git("push", "-q", testRemote, "HEAD:"+headsPrefix+testBranch)
	round := reviewRound{r: r, base: base, phase4: phase4}
	round.addReviewCommits(n)
	return round
}

// addReviewCommits commits n times on HEAD.
func (p reviewRound) addReviewCommits(n int) {
	for i := range n {
		p.r.Write("docs/notes/review.md", fmt.Sprintf("review %d after %s\n", i, p.r.Head()))
		p.r.CommitAll("review")
	}
}

// mergeOnHead makes HEAD a merge commit of a side branch made at HEAD.
func (p reviewRound) mergeOnHead() {
	p.r.Git("switch", "-q", "-c", "side")
	p.r.Write("docs/notes/side.md", "side\n")
	p.r.CommitAll("side")
	p.r.Git("switch", "-q", testBranch)
	p.r.Write("docs/notes/main.md", "main\n")
	p.r.CommitAll("main")
	p.r.Git("merge", "-q", "--no-ff", "-m", "merge", "side")
}

// wantLineRule fails unless err is a head stop for rule, a rule the
// published list (which the guide is checked against) names.
func wantLineRule(t *testing.T, err error, rule lineRule) {
	t.Helper()
	wantStateParts(t, err, PartHead)
	if stateErr, ok := errors.AsType[*GitStateError](err); ok && stateErr.Rule != string(rule) {
		t.Errorf("rule = %q, want %q", stateErr.Rule, rule)
	}
	if !slices.Contains(HeadLineRules(), string(rule)) {
		t.Errorf("rule %q is not in HeadLineRules", rule)
	}
}

// wantNoRule fails unless err is a GitStateError without a rule.
func wantNoRule(t *testing.T, err error) {
	t.Helper()
	if stateErr, ok := errors.AsType[*GitStateError](err); !ok || stateErr.Rule != "" {
		t.Errorf("err = %v, want a GitStateError without a rule", err)
	}
}

func TestCheckReviewStateLine(t *testing.T) {
	for _, tc := range []struct {
		name    string
		edit    func(p reviewRound)
		commits int      // the commits after the Phase 4 commit, when it passes
		rule    lineRule // the head rule it stops on, when it does not
	}{
		{name: "no_review_commit"},
		{name: "one_review_commit", edit: func(p reviewRound) { p.addReviewCommits(1) }, commits: 1},
		{name: "three_review_commits", edit: func(p reviewRound) { p.addReviewCommits(3) }, commits: 3},
		{name: "at_the_limit", edit: func(p reviewRound) { p.addReviewCommits(maxReviewCommits) }, commits: maxReviewCommits},
		{name: "over_the_limit", edit: func(p reviewRound) { p.addReviewCommits(maxReviewCommits + 1) }, rule: lineTooLong},
		{name: "head_is_a_merge", edit: func(p reviewRound) { p.mergeOnHead() }, rule: lineMergeCommit},
		{name: "merge_inside_the_line", edit: func(p reviewRound) {
			p.mergeOnHead()
			p.addReviewCommits(1)
		}, rule: lineMergeCommit},
		{name: "head_is_the_baseline", edit: func(p reviewRound) {
			p.r.Git("reset", "-q", "--hard", p.base.head)
		}, rule: lineNoPhase4},
		{name: "line_not_from_the_baseline", edit: func(p reviewRound) {
			// Built on the baseline's parent, a line longer than the limit never
			// reaches the baseline's commit: the walk stops at the limit, before
			// the first commit, as it does on a branch rebased onto newer
			// history.
			p.r.Git("reset", "-q", "--hard", p.base.head+"^")
			p.addReviewCommits(maxReviewCommits + 1)
		}, rule: lineTooLong},
		{name: "head_has_no_parent", edit: func(p reviewRound) {
			orphan := strings.TrimSpace(p.r.Git("commit-tree", "HEAD^{tree}", "-m", "orphan"))
			p.r.Git("reset", "-q", "--hard", orphan)
		}, rule: lineRootCommit},
		{name: "another_phase4_commit", edit: func(p reviewRound) {
			// A line from the baseline, but its first commit is not the one
			// named.
			p.r.Git("reset", "-q", "--hard", p.base.head)
			p.r.Write(testFile, "another phase 4\n")
			p.r.CommitAll("another phase 4")
			p.addReviewCommits(1)
		}, rule: linePhase4Mismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newReviewRound(t, newTestRepo(t), 0)
			if tc.edit != nil {
				tc.edit(p)
			}
			w, router := newTestWorktree(t)
			got, err := w.CheckReviewState(t.Context(), p.base, p.phase4)
			if tc.rule != "" {
				wantLineRule(t, err, tc.rule)
			} else {
				want := ReviewState{Protocol: Protocol, Base: p.base.head, Head: p.r.Head(), Commits: tc.commits}
				if err != nil || got != want {
					t.Fatalf("CheckReviewState = %+v, %v, want %+v", got, err, want)
				}
			}
			// The push target is read only after the line passes, so only a
			// passing row can show that check reads none.
			if tc.rule == "" && indexOf(router.Calls, gitCommand+" ls-remote") >= 0 {
				t.Errorf("read the push target: %v", router.Calls)
			}
		})
	}

	t.Run("commit_names_a_review_commit", func(t *testing.T) {
		// The value passed only narrows the check: naming another commit of
		// the line stops instead of moving where the line starts.
		p := newReviewRound(t, newTestRepo(t), 2)
		w, _ := newTestWorktree(t)
		_, err := w.CheckReviewState(t.Context(), p.base, p.r.Head())
		wantLineRule(t, err, linePhase4Mismatch)
	})

	t.Run("push_target_not_moved", func(t *testing.T) {
		// check reads no push target, so it passes when Phase 4's push has
		// not reached the remote.
		p := newReviewRound(t, newTestRepo(t), 1)
		p.r.Git("push", "-q", "-f", testRemote, p.base.head+":"+headsPrefix+testBranch)
		w, router := newTestWorktree(t)
		if _, err := w.CheckReviewState(t.Context(), p.base, p.phase4); err != nil {
			t.Errorf("CheckReviewState = %v, want nil", err)
		}
		if indexOf(router.Calls, gitCommand+" ls-remote") >= 0 {
			t.Errorf("read the push target: %v", router.Calls)
		}
	})
}

func TestCheckReviewStateOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(t *testing.T, p reviewRound)
		parts []string
		layer stopLayer
	}{
		{name: "branch_switched", edit: func(_ *testing.T, p reviewRound) {
			p.r.Git("switch", "-q", "-c", "other")
		}, parts: []string{PartBranch}},
		{name: "detached_head", edit: func(_ *testing.T, p reviewRound) {
			p.r.Git("switch", "-q", "--detach")
		}, parts: []string{PartBranch}},
		{name: "branch_switched_onto_a_merge", edit: func(_ *testing.T, p reviewRound) {
			// HEAD would fail its line too; the branch is reported alone.
			p.mergeOnHead()
			p.r.Git("switch", "-q", "-c", "other")
		}, parts: []string{PartBranch}},
		{name: "config_changed", edit: func(_ *testing.T, p reviewRound) {
			p.r.Git("config", "remote."+testRemote+".receivepack", "evil")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "hook_added", edit: func(_ *testing.T, p reviewRound) {
			p.r.WriteHook("pre-commit", "exit 0\n")
		}, parts: []string{PartHooks}},
		{name: "info_changed", edit: func(t *testing.T, p reviewRound) {
			appendFile(t, filepath.Join(p.r.Dir, ".git", "info", "exclude"), "extra\n")
		}, parts: []string{PartInfo}, layer: layerFiles},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newReviewRound(t, newTestRepo(t), 1)
			tc.edit(t, p)
			w, router := newTestWorktree(t)
			_, err := w.CheckReviewState(t.Context(), p.base, p.phase4)
			wantStateParts(t, err, tc.parts...)
			wantNoRule(t, err)
			wantLayer(t, tc.layer, router.Calls)
		})
	}

	t.Run("ignored_changed", func(t *testing.T) {
		// A target file written after the baseline stops on ignored and
		// writes the stop record for the user to compare.
		p := newReviewRound(t, ignoringRepo(t, "build/"), 1)
		p.r.Write("build/x_test.go", "package x\n")
		w, _ := newTestWorktree(t)
		_, err := w.CheckReviewState(t.Context(), p.base, p.phase4)
		wantStateParts(t, err, PartIgnored)
		wantNoRule(t, err)
		if _, err := os.Lstat(filepath.Join(p.r.Dir, ".git", ignoredStopRecord)); err != nil {
			t.Errorf("stop record: %v", err)
		}
	})
}

func TestCheckReviewStateInput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		commit func(p reviewRound) string
		dir    func(t *testing.T, p reviewRound) string // "" keeps the work tree
	}{
		{name: "commit_uppercase", commit: func(p reviewRound) string { return strings.ToUpper(p.phase4) }},
		{name: "commit_short", commit: func(p reviewRound) string { return p.phase4[:12] }},
		{name: "commit_empty", commit: func(reviewRound) string { return "" }},
		{name: "below_the_top", commit: func(p reviewRound) string { return p.phase4 }, dir: func(_ *testing.T, p reviewRound) string {
			return filepath.Join(p.r.Dir, "docs")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newReviewRound(t, newTestRepo(t), 1)
			if tc.dir != nil {
				t.Chdir(tc.dir(t, p))
			}
			w, router := newTestWorktree(t)
			_, err := w.CheckReviewState(t.Context(), p.base, tc.commit(p))
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("CheckReviewState = %v, want ErrInvalidInput", err)
			}
			if n := gitCalls(router.Calls); n != 0 {
				t.Errorf("ran git %d times, want none: %v", n, router.Calls)
			}
		})
	}

	t.Run("git_file_malformed", func(t *testing.T) {
		// A .git in another shape is a changed state, not a usage error.
		p := newReviewRound(t, newTestRepo(t), 1)
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".git"), "not a gitdir line\n")
		t.Chdir(dir)
		w, router := newTestWorktree(t)
		_, err := w.CheckReviewState(t.Context(), p.base, p.phase4)
		wantStateParts(t, err, PartConfig)
		if n := gitCalls(router.Calls); n != 0 {
			t.Errorf("ran git %d times, want none: %v", n, router.Calls)
		}
	})
}

// TestGitStateErrorText: the one-line diagnostic is the output contract for a
// head stop (a fixed rule word and never a SHA), so its wording is checked.
func TestGitStateErrorText(t *testing.T) {
	plain := &GitStateError{Parts: []string{PartConfig}}
	if got, want := plain.Error(), ErrGitStateChanged.Error()+": config"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	ruled := lineError(lineTooLong)
	if got, want := ruled.Error(), ErrGitStateChanged.Error()+": head (rule "+string(lineTooLong)+")"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	// The rules must be told apart by name alone.
	rules := HeadLineRules()
	if distinct := len(slices.Compact(slices.Sorted(slices.Values(rules)))); distinct != len(rules) {
		t.Errorf("HeadLineRules() = %v, want distinct names", rules)
	}
}

// failRemotePushes makes the remote refuse every push.
func failRemotePushes(t *testing.T, r *testRepo) {
	t.Helper()
	hooks := filepath.Join(r.Remote, "hooks")
	if err := os.MkdirAll(hooks, testDirMode); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(hooks, "pre-receive"), "#!/bin/sh\nexit 1\n")
	chmod(t, filepath.Join(hooks, "pre-receive"), 0o700)
}

func TestPushReviewed(t *testing.T) {
	t.Run("pushes_the_checked_head", func(t *testing.T) {
		p := newReviewRound(t, newTestRepo(t), 2)
		w, _ := newTestWorktree(t)
		got, err := w.PushReviewed(t.Context(), p.base, p.phase4)
		want := PushResult{Protocol: Protocol, Pushed: true, Commit: p.r.Head()}
		if err != nil || got != want {
			t.Fatalf("PushReviewed = %+v, %v, want %+v", got, err, want)
		}
		if remote := p.r.RemoteHead(); remote != want.Commit {
			t.Errorf("remote branch = %s, want %s", remote, want.Commit)
		}
	})

	t.Run("head_moved_after_the_check", func(t *testing.T) {
		// HEAD moves after the push target is read; the push still sends the
		// OID that was checked.
		p := newReviewRound(t, newTestRepo(t), 1)
		checked := p.r.Head()
		w, router := newTestWorktree(t)
		router.Before = map[string]func(){gitCommand + " push": func() { p.addReviewCommits(1) }}
		got, err := w.PushReviewed(t.Context(), p.base, p.phase4)
		if err != nil || got.Commit != checked {
			t.Fatalf("PushReviewed = %+v, %v, want the checked commit %s", got, err, checked)
		}
		if p.r.Head() == checked {
			t.Fatal("HEAD did not move before the push")
		}
		if remote := p.r.RemoteHead(); remote != checked {
			t.Errorf("remote branch = %s, want the checked %s", remote, checked)
		}
	})

	t.Run("explicit_refspec", func(t *testing.T) {
		// A configured push refspec would send the branch elsewhere, and
		// followTags would send a tag of the review commit along.
		r := newTestRepo(t)
		r.Git("config", "remote."+testRemote+".push", headsPrefix+"*:"+headsPrefix+"other/*")
		r.Git("config", "push.followTags", "true")
		p := newReviewRound(t, r, 1)
		r.Git("tag", "-a", "-m", "agent tag", "agent-tag", r.Head())
		w, _ := newTestWorktree(t)
		if _, err := w.PushReviewed(t.Context(), p.base, p.phase4); err != nil {
			t.Fatalf("PushReviewed = %v", err)
		}
		if got := r.RemoteHead(); got != r.Head() {
			t.Errorf("remote branch = %s, want %s", got, r.Head())
		}
		if refs := r.gitIn(r.Remote, "for-each-ref", "--format=%(refname)", headsPrefix+"other/", "refs/tags/"); refs != "" {
			t.Errorf("pushed more than the branch: %q", refs)
		}
	})

	// A failed push and a failed push target read are both git failures; the
	// rule word in the diagnostic, the output contract the fixpr document
	// reports by, tells them apart.
	t.Run("push_failure", func(t *testing.T) {
		p := newReviewRound(t, newTestRepo(t), 1)
		failRemotePushes(t, p.r)
		w, _ := newTestWorktree(t)
		_, err := w.PushReviewed(t.Context(), p.base, p.phase4)
		if !errors.Is(err, ErrGitFailed) || !strings.Contains(err.Error(), "push failed") {
			t.Errorf("PushReviewed = %v, want a git failure of push", err)
		}
	})

	t.Run("ls_remote_failure", func(t *testing.T) {
		p := newReviewRound(t, newTestRepo(t), 1)
		w, router := newTestWorktree(t)
		router.Before = map[string]func(){gitCommand + " ls-remote": func() {
			if err := os.Rename(p.r.Remote, p.r.Remote+".gone"); err != nil {
				t.Fatal(err)
			}
		}}
		_, err := w.PushReviewed(t.Context(), p.base, p.phase4)
		if !errors.Is(err, ErrGitFailed) || !strings.Contains(err.Error(), "ls-remote failed") || strings.Contains(err.Error(), "push") {
			t.Errorf("PushReviewed = %v, want a git failure of ls-remote, not of push", err)
		}
	})
}

func wantInvalidInput(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestPushReviewedStops(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reviews int
		edit    func(p reviewRound)
		commit  func(p reviewRound) string // the Phase 4 commit passed; p.phase4 when nil
		below   bool                       // run from a directory below the top
		layer   stopLayer                  // the git calls the stop allows
		check   func(t *testing.T, err error)
	}{
		{name: "nothing_to_send", check: func(t *testing.T, err error) { t.Helper(); wantInvalidInput(t, err) }},
		{name: "nothing_to_send_and_branch_switched", edit: func(p reviewRound) {
			// The refusal comes after the comparison, so a changed state is
			// reported as such even with nothing to send.
			p.r.Git("switch", "-q", "-c", "other")
		}, check: func(t *testing.T, err error) { t.Helper(); wantStateParts(t, err, PartBranch) }},
		{
			name: "commit_malformed", reviews: 1, commit: func(p reviewRound) string { return strings.ToUpper(p.phase4) }, layer: layerFiles,
			check: func(t *testing.T, err error) { t.Helper(); wantInvalidInput(t, err) },
		},
		{
			name: "below_the_top", reviews: 1, below: true, layer: layerFiles,
			check: func(t *testing.T, err error) { t.Helper(); wantInvalidInput(t, err) },
		},
		{name: "phase4_push_not_arrived", reviews: 1, edit: func(p reviewRound) {
			p.r.Git("push", "-q", "-f", testRemote, p.base.head+":"+headsPrefix+testBranch)
		}, check: func(t *testing.T, err error) { t.Helper(); wantStateParts(t, err, PartRemote) }},
		{name: "remote_moved_on", reviews: 1, edit: func(p reviewRound) {
			p.r.Git("push", "-q", testRemote, p.r.Head()+":"+headsPrefix+testBranch)
			p.addReviewCommits(1)
		}, check: func(t *testing.T, err error) { t.Helper(); wantStateParts(t, err, PartRemote) }},
		{name: "config_changed", reviews: 1, edit: func(p reviewRound) {
			p.r.Git("config", "remote."+testRemote+".receivepack", "evil")
		}, layer: layerList, check: func(t *testing.T, err error) { t.Helper(); wantStateParts(t, err, PartConfig) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newReviewRound(t, newTestRepo(t), tc.reviews)
			if tc.edit != nil {
				tc.edit(p)
			}
			remote := p.r.RemoteHead()
			phase4 := p.phase4
			if tc.commit != nil {
				phase4 = tc.commit(p)
			}
			if tc.below {
				t.Chdir(filepath.Join(p.r.Dir, "docs"))
			}
			w, router := newTestWorktree(t)
			_, err := w.PushReviewed(t.Context(), p.base, phase4)
			tc.check(t, err)
			wantLayer(t, tc.layer, router.Calls)
			if indexOf(router.Calls, gitCommand+" push") >= 0 {
				t.Errorf("pushed: %v", router.Calls)
			}
			if got := p.r.RemoteHead(); got != remote {
				t.Errorf("remote moved to %s", got)
			}
		})
	}
}
