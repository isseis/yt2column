//go:build test

package fixprguard

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// wantStateParts fails unless err is a *GitStateError naming exactly parts.
func wantStateParts(t *testing.T, err error, parts ...string) {
	t.Helper()
	stateErr, ok := errors.AsType[*GitStateError](err)
	if !ok || !errors.Is(err, ErrGitStateChanged) {
		t.Fatalf("err = %v, want a *GitStateError", err)
	}
	if !slices.Equal(stateErr.Parts, parts) {
		t.Errorf("parts = %v, want %v", stateErr.Parts, parts)
	}
}

func chmod(t *testing.T, name string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), testDirMode); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // a file of the test's temporary repository
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBaselineDetectsChange(t *testing.T) {
	hook := func(r *testRepo) string { return filepath.Join(r.Dir, ".git", "hooks", "pre-commit") }
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, r *testRepo) // before the baseline
		edit  func(t *testing.T, r *testRepo) // after it
		parts []string
	}{
		{name: "new_commit", edit: func(_ *testing.T, r *testRepo) {
			r.Write(testFile, "changed\n")
			r.CommitAll("agent commit")
		}, parts: []string{PartHead}},
		{name: "reset", setup: func(_ *testing.T, r *testRepo) {
			r.Write(testFile, "second\n")
			r.CommitAll("second")
		}, edit: func(_ *testing.T, r *testRepo) { r.Git("reset", "-q", "--hard", "HEAD~1") }, parts: []string{PartHead}},
		{name: "branch_switch_same_commit", edit: func(_ *testing.T, r *testRepo) {
			r.Git("switch", "-q", "-c", "other")
		}, parts: []string{PartHead}},
		{name: "remote_moved", edit: func(_ *testing.T, r *testRepo) {
			r.Write(testFile, "remote\n")
			r.CommitAll("pushed elsewhere")
			r.Git("push", "-q", testRemote, "HEAD:"+headsPrefix+testBranch)
			r.Git("reset", "-q", "--hard", "HEAD~1")
		}, parts: []string{PartRemote}},
		{name: "global_config", edit: func(_ *testing.T, r *testRepo) {
			// Outside the repository's files; git config --list reads it.
			r.Git("config", "--global", "url."+r.Remote+"-elsewhere.pushInsteadOf", r.Remote)
		}, parts: []string{PartConfig}},
		{name: "included_config", setup: func(_ *testing.T, r *testRepo) {
			r.Git("config", "include.path", filepath.Join(r.Home, "included"))
		}, edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Home, "included"), "[filter \"x\"]\n\tclean = cat\n")
		}, parts: []string{PartConfig}},
		{name: "commondir_added", edit: func(t *testing.T, r *testRepo) {
			// git reads commondir in a main checkout too; a copy keeps every
			// file the digests read unchanged.
			r.gitIn(r.Home, "clone", "-q", "--bare", r.Dir, filepath.Join(r.Home, "copy.git"))
			appendFile(t, filepath.Join(r.Dir, ".git", "commondir"), filepath.Join(r.Home, "copy.git")+"\n")
		}, parts: []string{PartConfig, PartInfo}},
		{name: "hook_added", edit: func(_ *testing.T, r *testRepo) { r.WriteHook("pre-commit", "exit 0\n") }, parts: []string{PartHooks}},
		{name: "hook_removed", setup: func(_ *testing.T, r *testRepo) { r.WriteHook("pre-commit", "exit 0\n") }, edit: func(t *testing.T, r *testRepo) {
			if err := os.Remove(hook(r)); err != nil {
				t.Fatal(err)
			}
		}, parts: []string{PartHooks}},
		{name: "hook_executable_bit", setup: func(t *testing.T, r *testRepo) {
			r.WriteHook("pre-commit", "exit 0\n")
			chmod(t, hook(r), 0o600)
		}, edit: func(t *testing.T, r *testRepo) { chmod(t, hook(r), 0o700) }, parts: []string{PartHooks}},
		{name: "hook_link_target", setup: func(t *testing.T, r *testRepo) {
			r.WriteHook("one", "exit 0\n")
			r.WriteHook("two", "exit 0\n")
			if err := os.Symlink("one", hook(r)); err != nil {
				t.Fatal(err)
			}
		}, edit: func(t *testing.T, r *testRepo) {
			if err := os.Remove(hook(r)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("two", hook(r)); err != nil {
				t.Fatal(err)
			}
		}, parts: []string{PartHooks}},
		{name: "hook_link_target_content", setup: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Home, "hook.sh"), "#!/bin/sh\nexit 0\n")
			if err := os.MkdirAll(filepath.Dir(hook(r)), testDirMode); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(r.Home, "hook.sh"), hook(r)); err != nil {
				t.Fatal(err)
			}
		}, edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Home, "hook.sh"), "echo changed\n")
		}, parts: []string{PartHooks}},
		{name: "hooks_path_target", setup: func(t *testing.T, r *testRepo) {
			if err := os.MkdirAll(filepath.Join(r.Home, "hooks"), testDirMode); err != nil {
				t.Fatal(err)
			}
			r.Git("config", "--global", "core.hooksPath", filepath.Join(r.Home, "hooks"))
		}, edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Home, "hooks", "pre-push"), "exit 0\n")
		}, parts: []string{PartHooks}},
		{name: "config", edit: func(_ *testing.T, r *testRepo) { r.Git("config", "core.fsmonitor", "true") }, parts: []string{PartConfig}},
		{name: "worktree_config", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Dir, ".git", "config.worktree"), "[core]\n\tsshCommand = evil\n")
		}, parts: []string{PartConfig}},
		{name: "git_file_points_elsewhere", edit: func(t *testing.T, r *testRepo) {
			if err := os.Rename(filepath.Join(r.Dir, ".git"), filepath.Join(r.Home, "moved.git")); err != nil {
				t.Fatal(err)
			}
			r.Write(".git", "gitdir: "+filepath.Join(r.Home, "moved.git")+"\n")
		}, parts: []string{PartConfig}},
		{name: "info_exclude", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Dir, ".git", "info", "exclude"), "*.go\n")
		}, parts: []string{PartInfo}},
		{name: "info_attributes", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Dir, ".git", "info", "attributes"), "*.md -diff\n")
		}, parts: []string{PartInfo}},
		{name: "skip_worktree", edit: func(_ *testing.T, r *testRepo) {
			r.Git("update-index", "--skip-worktree", testFile)
		}, parts: []string{PartInfo}},
		{name: "assume_unchanged", edit: func(_ *testing.T, r *testRepo) {
			r.Git("update-index", "--assume-unchanged", testFile)
		}, parts: []string{PartInfo}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			if tc.setup != nil {
				tc.setup(t, r)
			}
			base := mustBaseline(t)
			w, _ := newTestWorktree(t)
			if _, err := w.compareState(t.Context(), base, base.head, checkWithRemote); err != nil {
				t.Fatalf("compareState before the edit = %v", err)
			}
			tc.edit(t, r)
			_, err := w.compareState(t.Context(), base, base.head, checkWithRemote)
			wantStateParts(t, err, tc.parts...)
		})
	}

	t.Run("linked_worktree_common_config", func(t *testing.T) {
		r := newTestRepo(t)
		linked := filepath.Join(r.Home, "linked")
		r.Git("worktree", "add", "-q", "-b", "linked-branch", linked)
		t.Chdir(linked)
		r.Git("config", "branch.linked-branch.remote", testRemote)
		base := mustBaseline(t)
		// The shared config lives in the main repository's git directory.
		appendFile(t, filepath.Join(r.Dir, ".git", "config"), "[core]\n\tfsmonitor = true\n")
		w, _ := newTestWorktree(t)
		_, err := w.compareState(t.Context(), base, base.head, checkWithRemote)
		wantStateParts(t, err, PartConfig)
	})

	t.Run("baseline_refuses_flagged_entry", func(t *testing.T) {
		r := newTestRepo(t)
		r.Git("update-index", "--skip-worktree", testFile)
		w, _ := newTestWorktree(t)
		_, err := w.Baseline(t.Context())
		wantStateParts(t, err, PartInfo)
	})

	t.Run("baseline_refuses_detached_head", func(t *testing.T) {
		r := newTestRepo(t)
		r.Git("switch", "-q", "--detach")
		w, _ := newTestWorktree(t)
		if _, err := w.Baseline(t.Context()); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Baseline = %v, want ErrInvalidInput", err)
		}
	})
}

func TestBaselineIgnoresIndexAndFetchHead(t *testing.T) {
	r := newTestRepo(t)
	base := mustBaseline(t)
	r.Git("fetch", "-q", testRemote)
	if _, err := os.Stat(filepath.Join(r.Dir, ".git", "FETCH_HEAD")); err != nil {
		t.Fatalf("fetch wrote no FETCH_HEAD: %v", err)
	}
	r.Git("read-tree", "HEAD")
	r.Git("update-index", "--refresh")
	r.Git("checkout", "-q", testBranch)
	w, _ := newTestWorktree(t)
	if _, err := w.compareState(t.Context(), base, base.head, checkWithRemote); err != nil {
		t.Errorf("compareState = %v, want nil", err)
	}
}

func TestBaselineMalformedRejected(t *testing.T) {
	newTestRepo(t)
	token := mustBaseline(t).String()
	parsed, err := ParseBaseline(token)
	if err != nil || parsed.String() != token {
		t.Fatalf("ParseBaseline(round trip) = %q, %v", parsed.String(), err)
	}
	fields := strings.Split(token, ".")
	with := func(i int, v string) string {
		f := slices.Clone(fields)
		f[i] = v
		return strings.Join(f, ".")
	}
	for _, tc := range []struct{ name, token string }{
		{"empty", ""},
		{"version", with(0, "v2")},
		{"missing_field", strings.Join(fields[:len(fields)-1], ".")},
		{"extra_field", token + ".00"},
		{"head_upper_case", with(1, strings.ToUpper(fields[1]))},
		{"head_short", with(1, fields[1][1:])},
		{"remote_word", with(3, "absent")},
		{"digest_short", with(5, fields[5][1:])},
		{"digest_not_hex", with(6, strings.Repeat("g", digestLength))},
		{"surrounding_space", " " + token},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseBaseline(tc.token); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("ParseBaseline = %v, want ErrInvalidInput", err)
			}
		})
	}
	t.Run("zero_baseline", func(t *testing.T) {
		w, router := newTestWorktree(t)
		if err := w.CheckState(t.Context(), Baseline{}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("CheckState(zero) = %v, want ErrInvalidInput", err)
		}
		if n := gitCalls(router.Calls); n != 0 {
			t.Errorf("ran git %d times with a zero baseline", n)
		}
	})
}

func TestGitNotLaunchedAfterConfigChange(t *testing.T) {
	t.Run("inspect", func(t *testing.T) {
		r := newTestRepo(t)
		base := mustBaseline(t)
		appendFile(t, filepath.Join(r.Dir, ".git", "config"), "[core]\n\tfsmonitor = true\n")
		w, router := newTestWorktree(t)
		_, err := w.Inspect(t.Context(), base, threadAt(testFile))
		wantStateParts(t, err, PartConfig)
		if n := gitCalls(router.Calls); n != 0 {
			t.Errorf("ran git %d times after the config changed: %v", n, router.Calls)
		}
	})
	t.Run("finish_after_make_test", func(t *testing.T) {
		r := newTestRepo(t)
		base := mustBaseline(t)
		r.Write(testFile, "fixed\n")
		judged := mustInspect(t, base).JudgedSet
		rewrite := func() error {
			appendFile(t, filepath.Join(r.Dir, ".git", "config"), "[core]\n\tfsmonitor = true\n")
			return nil
		}
		w, router := newTestWorktree(t, makeStep(StageFmt, nil), makeStep(StageTest, rewrite))
		_, err := w.Finish(t.Context(), base, judged, FinishRequest{PR: mustPR(t)})
		wantStateParts(t, err, PartConfig)
		if last := router.Calls[len(router.Calls)-1]; last != makeCommand+" "+StageTest {
			t.Errorf("last command = %q, want make test: %v", last, router.Calls)
		}
	})
}

func TestGitEnvDropsInheritedGitVariables(t *testing.T) {
	r := newTestRepo(t)
	// Inherited, these would point git at another repository and config.
	t.Setenv("GIT_DIR", filepath.Join(r.Home, "nowhere"))
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.fsmonitor'='true'")
	env := GitEnv(os.Environ())
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_") && entry != "GIT_OPTIONAL_LOCKS=0" && entry != "GIT_TERMINAL_PROMPT=0" {
			t.Errorf("GitEnv kept %q", entry)
		}
	}
	for _, want := range []string{"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "HOME=" + r.Home} {
		if !slices.Contains(env, want) {
			t.Errorf("GitEnv lacks %q", want)
		}
	}
	w, _ := newTestWorktree(t)
	if _, err := w.Baseline(t.Context()); err != nil {
		t.Errorf("Baseline with GIT_DIR inherited = %v", err)
	}
}
