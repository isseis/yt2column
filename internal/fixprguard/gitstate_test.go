//go:build test

package fixprguard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
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

// stopLayer is the check a config change is seen by.
type stopLayer int

const (
	// layerUnchecked leaves the git calls unchecked.
	layerUnchecked stopLayer = iota
	// layerFiles sees it from the files alone: no git runs.
	layerFiles
	// layerList sees it in git config --list, the only git that runs.
	layerList
)

// wantLayer fails unless calls stopped at layer.
func wantLayer(t *testing.T, layer stopLayer, calls []string) {
	t.Helper()
	switch layer {
	case layerUnchecked:
	case layerFiles:
		if n := gitCalls(calls); n != 0 {
			t.Errorf("ran git %d times, want none: %v", n, calls)
		}
	case layerList:
		if !slices.Equal(calls, []string{gitConfigList}) {
			t.Errorf("calls = %v, want git config --list only", calls)
		}
	default:
		t.Fatalf("unknown layer %d", layer)
	}
}

func TestBaselineDetectsChange(t *testing.T) {
	hook := func(r *testRepo) string { return filepath.Join(r.Dir, ".git", "hooks", "pre-commit") }
	repoConfig := func(r *testRepo) string { return filepath.Join(r.Dir, ".git", "config") }
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, r *testRepo) // before the baseline
		edit  func(t *testing.T, r *testRepo) // after it
		parts []string
		layer stopLayer
	}{
		{name: "new_commit", edit: func(_ *testing.T, r *testRepo) {
			r.Write(testFile, "changed\n")
			r.CommitAll("agent commit")
		}, parts: []string{PartHead}},
		{name: "branch_switch_same_commit", edit: func(_ *testing.T, r *testRepo) {
			r.Git("switch", "-q", "-c", "other")
		}, parts: []string{PartHead}},
		{name: "detached_head_same_commit", edit: func(_ *testing.T, r *testRepo) {
			r.Git("switch", "-q", "--detach")
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
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "included_config", setup: func(_ *testing.T, r *testRepo) {
			r.Git("config", "include.path", filepath.Join(r.Home, "included"))
		}, edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Home, "included"), "[filter \"x\"]\n\tclean = cat\n")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "commondir_added", edit: func(t *testing.T, r *testRepo) {
			// git reads commondir in a main checkout too; a copy keeps every
			// file the digests read unchanged.
			r.gitIn(r.Home, "clone", "-q", "--bare", r.Dir, filepath.Join(r.Home, "copy.git"))
			appendFile(t, filepath.Join(r.Dir, ".git", "commondir"), filepath.Join(r.Home, "copy.git")+"\n")
		}, parts: []string{PartConfig, PartInfo}, layer: layerFiles},
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
		{name: "config", edit: func(_ *testing.T, r *testRepo) { r.Git("config", "core.fsmonitor", "true") }, parts: []string{PartConfig}, layer: layerList},
		// The current branch's settings, under every name git gives them.
		{name: "current_branch_added", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "branch."+testBranch+".pushRemote", testRemote)
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "current_branch_removed", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "--unset", "branch."+testBranch+".merge")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "current_branch_with_dot", setup: func(_ *testing.T, r *testRepo) {
			r.Git("switch", "-q", "-c", testBranch+".a")
		}, edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "branch."+testBranch+".a.remote", testRemote)
		}, parts: []string{PartConfig}, layer: layerList},
		// git lowercases the subsection of the old [branch.Name] syntax, so
		// this is the current branch's setting.
		{name: "current_branch_old_syntax", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, repoConfig(r), "[branch.Feature]\n\tpushRemote = "+testRemote+"\n")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "main_branch", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "branch."+mainBranch+".pushRemote", testRemote)
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "two_level_branch_setting", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "branch.autoSetupMerge", "always")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "remote_push_default", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "remote.pushDefault", testRemote)
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "remote_added", edit: func(_ *testing.T, r *testRepo) {
			r.Git("remote", "add", "upstream", r.Remote)
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "include_added", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "include.path", filepath.Join(r.Home, "absent"))
		}, parts: []string{PartConfig}, layer: layerList},
		// Moving a setting to another file changes its origin, and here its
		// place in the listing too: the global file is read first.
		{name: "origin_moved", setup: func(_ *testing.T, r *testRepo) {
			r.Git("config", "core.abbrev", "12")
		}, edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "--unset", "core.abbrev")
			r.Git("config", "--global", "core.abbrev", "12")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "multi_value_order", setup: func(_ *testing.T, r *testRepo) {
			r.Git("config", "--add", "remote."+testRemote+".push", "refs/heads/a")
			r.Git("config", "--add", "remote."+testRemote+".push", "refs/heads/b")
		}, edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "--unset-all", "remote."+testRemote+".push")
			r.Git("config", "--add", "remote."+testRemote+".push", "refs/heads/b")
			r.Git("config", "--add", "remote."+testRemote+".push", "refs/heads/a")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "worktree_config", setup: func(t *testing.T, r *testRepo) {
			// git reads config.worktree only with worktreeConfig on, and the
			// file exists at begin, so the change is in the listed values,
			// not in the file's type.
			r.Git("config", "extensions.worktreeConfig", "true")
			appendFile(t, filepath.Join(r.Dir, ".git", "config.worktree"), "")
		}, edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Dir, ".git", "config.worktree"), "[core]\n\tsshCommand = evil\n")
		}, parts: []string{PartConfig}, layer: layerList},
		// A worktree config file that did not exist at begin is a type
		// change, seen from the files alone whatever it holds.
		{name: "worktree_config_created", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Dir, ".git", "config.worktree"), "")
		}, parts: []string{PartConfig}, layer: layerFiles},
		{name: "git_file_points_elsewhere", edit: func(t *testing.T, r *testRepo) {
			if err := os.Rename(filepath.Join(r.Dir, ".git"), filepath.Join(r.Home, "moved.git")); err != nil {
				t.Fatal(err)
			}
			r.Write(".git", "gitdir: "+filepath.Join(r.Home, "moved.git")+"\n")
		}, parts: []string{PartConfig}, layer: layerFiles},
		{name: "linked_worktree_common_config", setup: func(t *testing.T, r *testRepo) {
			linked := filepath.Join(r.Home, "linked")
			r.Git("worktree", "add", "-q", "-b", "linked-branch", linked)
			t.Chdir(linked)
		}, edit: func(t *testing.T, r *testRepo) {
			// The shared config lives in the main repository's git directory.
			appendFile(t, filepath.Join(r.Dir, ".git", "config"), "[core]\n\tfsmonitor = true\n")
		}, parts: []string{PartConfig}, layer: layerList},
		{name: "info_exclude", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Dir, ".git", "info", "exclude"), "*.go\n")
		}, parts: []string{PartInfo}, layer: layerFiles},
		{name: "info_attributes", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Dir, ".git", "info", "attributes"), "*.md -diff\n")
		}, parts: []string{PartInfo}, layer: layerFiles},
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
			w, router := newTestWorktree(t)
			_, err := w.compareState(t.Context(), base, base.head, checkWithRemote)
			wantStateParts(t, err, tc.parts...)
			wantLayer(t, tc.layer, router.Calls)
		})
	}

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
	// Restaging the file with a new mtime rewrites the index bytes.
	index := filepath.Join(r.Dir, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(r.Dir, testFile), later, later); err != nil {
		t.Fatal(err)
	}
	r.Git("add", testFile)
	if after, _ := os.ReadFile(index); bytes.Equal(after, before) {
		t.Fatal("the index did not change")
	}
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
		// The version before this baseline shape was introduced.
		{"previous_version", with(0, "v1")},
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
		if !slices.Equal(router.Calls, []string{gitConfigList}) {
			t.Errorf("calls after the config changed = %v, want git config --list only", router.Calls)
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
		// After make test changed the config, only git config --list runs:
		// not make lint, and no other git.
		test := indexOf(router.Calls, makeCommand+" "+StageTest)
		if test < 0 || !slices.Equal(router.Calls[test+1:], []string{gitConfigList}) {
			t.Errorf("calls = %v, want git config --list only after make test", router.Calls)
		}
	})
}

// TestConfigFileTypeCheckedBeforeGit: a repository config file replaced by
// a FIFO after begin stops the run before any git runs, git config --list
// included, since git could read different content from it each time it
// opens it. git config --list is replaced to fail at once, so a missing type
// check fails on the call count instead of waiting on the FIFO.
func TestConfigFileTypeCheckedBeforeGit(t *testing.T) {
	r := newTestRepo(t)
	base := mustBaseline(t)
	config := filepath.Join(r.Dir, ".git", "config")
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(config, 0o600); err != nil {
		t.Fatal(err)
	}
	w, router := newTestWorktree(t)
	router.ConfigList = &configListResult{Err: &cmdrunner.ExitError{Code: 128}}
	_, err := w.Inspect(t.Context(), base, threadAt(testFile))
	wantStateParts(t, err, PartConfig)
	if n := gitCalls(router.Calls); n != 0 {
		t.Errorf("ran git %d times after the config file became a FIFO: %v", n, router.Calls)
	}
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

func TestOtherBranchConfigIgnored(t *testing.T) {
	repoConfig := func(r *testRepo) string { return filepath.Join(r.Dir, ".git", "config") }
	section := "[branch \"x\"]\n\tremote = " + testRemote + "\n\tmerge = refs/heads/x\n"
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, r *testRepo) // before the baseline
		edit  func(t *testing.T, r *testRepo) // after it
		// fileOnly marks an edit that changes the repository's config file
		// but not the listed values; every other edit changes the listing.
		fileOnly bool
		// checkOnly compares with CheckState alone: the edit is ignored
		// there, but git refuses to run with it, so the later git calls of
		// compareState fail.
		checkOnly bool
	}{
		{name: "written_with_git_config", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "branch.x.remote", testRemote)
		}},
		{name: "section_added", edit: func(t *testing.T, r *testRepo) { appendFile(t, repoConfig(r), section) }},
		{name: "section_removed", setup: func(t *testing.T, r *testRepo) { appendFile(t, repoConfig(r), section) }, edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "--remove-section", "branch.x")
		}},
		// git lists [branch ""] but every other git command stops on it with
		// "bad config variable", so only CheckState is compared here.
		{name: "empty_subsection_section_added", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, repoConfig(r), "[branch \"\"]\n\tremote = "+testRemote+"\n")
		}, checkOnly: true},
		{name: "global_config", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "--global", "branch.x.remote", testRemote)
		}},
		{name: "worktree_config", setup: func(t *testing.T, r *testRepo) {
			r.Git("config", "extensions.worktreeConfig", "true")
			appendFile(t, filepath.Join(r.Dir, ".git", "config.worktree"), "")
		}, edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "--worktree", "branch.x.remote", testRemote)
		}},
		{name: "include_path_target", setup: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Home, "included"), "")
			r.Git("config", "include.path", filepath.Join(r.Home, "included"))
		}, edit: func(t *testing.T, r *testRepo) { appendFile(t, filepath.Join(r.Home, "included"), section) }},
		{name: "include_if_target", setup: func(t *testing.T, r *testRepo) {
			appendFile(t, filepath.Join(r.Home, "conditional"), "")
			r.Git("config", "includeIf.onbranch:"+testBranch+".path", filepath.Join(r.Home, "conditional"))
		}, edit: func(t *testing.T, r *testRepo) { appendFile(t, filepath.Join(r.Home, "conditional"), section) }},
		// Names that overlap the current branch's: a longer subsection, and
		// one differing only in case.
		{name: "subsection_extends_current_branch", edit: func(_ *testing.T, r *testRepo) {
			r.Git("config", "branch."+testBranch+".a.remote", testRemote)
		}},
		{name: "subsection_differs_in_case", edit: func(t *testing.T, r *testRepo) {
			recased := strings.ToUpper(testBranch[:1]) + testBranch[1:]
			appendFile(t, repoConfig(r), "[branch \""+recased+"\"]\n\tpushRemote = elsewhere\n")
		}},
		{name: "comment_blank_and_indent", edit: func(t *testing.T, r *testRepo) {
			appendFile(t, repoConfig(r), "# a comment\n\n\t\n; another\n")
		}, fileOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			if tc.setup != nil {
				tc.setup(t, r)
			}
			base := mustBaseline(t)
			listBefore := r.ConfigList()
			fileBefore, err := os.ReadFile(repoConfig(r))
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(t, r)
			// The edit must be visible to the layer that ignores it, so only
			// that layer can let the comparison pass.
			if tc.fileOnly {
				fileAfter, err := os.ReadFile(repoConfig(r))
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Equal(fileAfter, fileBefore) || r.ConfigList() != listBefore {
					t.Fatal("the edit must change the config file and not the listing")
				}
			} else if r.ConfigList() == listBefore {
				t.Fatal("the edit does not change the listing")
			}
			w, _ := newTestWorktree(t)
			if tc.checkOnly {
				if err := w.CheckState(t.Context(), base); err != nil {
					t.Errorf("CheckState = %v, want nil", err)
				}
				return
			}
			if _, err := w.compareState(t.Context(), base, base.head, checkWithRemote); err != nil {
				t.Errorf("compareState = %v, want nil", err)
			}
		})
	}
}

// TestConfigListUnreadable: a configuration git listed at begin but cannot
// list now is a config change, seen before git rev-parse runs.
func TestConfigListUnreadable(t *testing.T) {
	r := newTestRepo(t)
	base := mustBaseline(t)
	appendFile(t, filepath.Join(r.Dir, ".git", "config"), "[core\n")
	w, router := newTestWorktree(t)
	wantStateParts(t, w.CheckState(t.Context(), base), PartConfig)
	if !slices.Equal(router.Calls, []string{gitConfigList}) {
		t.Errorf("calls = %v, want git config --list only", router.Calls)
	}
}

// TestConfigListFailureKinds: how a failed or malformed git config --list is
// reported. In CheckState, after begin listed the configuration, a timeout,
// a nonzero exit, or a malformed listing is a config change; an output over
// the cap, a cancellation, or any other failure is a git failure. At begin
// every one of them is a git failure and no baseline is taken.
func TestConfigListFailureKinds(t *testing.T) {
	exit := func(code int) *cmdrunner.ExitError { return &cmdrunner.ExitError{Code: code} }
	for _, tc := range []struct {
		name    string
		result  configListResult
		changed bool // a config change in CheckState rather than a git failure
	}{
		// Joined with an exit error, so the cap is what keeps it a git failure.
		{name: "output_over_cap", result: configListResult{Err: errors.Join(fmt.Errorf("git: %w", cmdrunner.ErrOutputTooLarge), exit(1))}},
		{name: "cancelled", result: configListResult{Err: errors.Join(context.Canceled, exit(-1))}},
		{name: "timed_out", result: configListResult{Err: errors.Join(context.DeadlineExceeded, exit(-1))}, changed: true},
		{name: "nonzero_exit", result: configListResult{Err: exit(128)}, changed: true},
		{name: "not_started", result: configListResult{Err: errors.New("exec: not found")}},
		{name: "malformed_listing", result: configListResult{Out: []byte("no delimiters")}, changed: true},
	} {
		t.Run("check_state/"+tc.name, func(t *testing.T) {
			newTestRepo(t)
			base := mustBaseline(t)
			w, router := newTestWorktree(t)
			router.ConfigList = &tc.result
			err := w.CheckState(t.Context(), base)
			if tc.changed {
				wantStateParts(t, err, PartConfig)
			} else if _, isState := errors.AsType[*GitStateError](err); isState || !errors.Is(err, ErrGitFailed) {
				t.Errorf("CheckState = %v, want ErrGitFailed and no *GitStateError", err)
			}
			if !slices.Equal(router.Calls, []string{gitConfigList}) {
				t.Errorf("calls = %v, want git config --list only", router.Calls)
			}
		})
		t.Run("begin/"+tc.name, func(t *testing.T) {
			newTestRepo(t)
			w, router := newTestWorktree(t)
			router.ConfigList = &tc.result
			base, err := w.Baseline(t.Context())
			if _, isState := errors.AsType[*GitStateError](err); isState || !errors.Is(err, ErrGitFailed) {
				t.Errorf("Baseline = %v, want ErrGitFailed and no *GitStateError", err)
			}
			if base.usable() {
				t.Errorf("Baseline returned %q with the error", base)
			}
		})
	}
}
