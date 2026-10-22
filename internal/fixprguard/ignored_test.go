//go:build test

package fixprguard

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// TestIsWatched: the rows come from the watch list table and the examples of
// the requirements, written out here rather than built from the data in
// ignored.go, so removing an item from the data fails its row. Every item of
// the table has a row.
func TestIsWatched(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// The top-level .claude directory, and .claude itself.
		{"claude_settings", ".claude/settings.local.json", true},
		{"claude_itself", ".claude", true},
		{"claude_deep", ".claude/worktrees/x/internal/a.txt", true},
		{"claude_not_top", "a/.claude/settings.local.json", false},
		{"claude_prefix_only", ".claude.bak/settings.local.json", false},

		// Top-level files.
		{"env", ".env", true},
		{"envrc", ".envrc", true},
		{"claude_local_md", "claude.local.md", true},
		{"env_not_top", "a/.env", false},
		{"envrc_not_top", "extension/.envrc", false},
		{"claude_local_md_not_top", "docs/CLAUDE.local.md", false},
		{"envrc_upper", ".ENVRC", true},
		{"claude_local_md_upper", "CLAUDE.local.md", true},

		// Names in any directory.
		{"claude_md", "docs/CLAUDE.md", true},
		{"makefile", "extension/node_modules/pkg/Makefile", true},
		{"gnumakefile", "build/GNUmakefile", true},
		{"go_mod", "build/x/go.mod", true},
		{"go_work", "go.work", true},
		{"go_sum", "go.sum", false},
		{"agents_md", "AGENTS.md", false},
		{"name_as_suffix", "build/xmakefile", false},
		{"name_with_more_after", "go.mod.bak", false},

		// Suffixes in any directory.
		{"go", "build/x_test.go", true},
		{"mk", "build/rules.mk", true},
		{"c", "a/x.c", true},
		{"h", ".venv/include/x.h", true},
		{"cc", "a/x.cc", true},
		{"cpp", "a/x.cpp", true},
		{"cxx", "a/x.cxx", true},
		{"hh", "a/x.hh", true},
		{"hpp", "a/x.hpp", true},
		{"hxx", "a/x.hxx", true},
		{"m", "a/x.m", true},
		{"s", "a/x.s", true},
		{"sx", "a/x.sx", true},
		{"f", "a/x.f", true},
		{"for", "a/x.for", true},
		{"f90", "a/x.f90", true},
		{"swig", "a/x.swig", true},
		{"swigcxx", "a/x.swigcxx", true},
		{"syso", "build/payload.syso", true},
		{"name_is_only_suffix", "build/.go", true},
		{"suffix_not_at_end", "build/x.go.txt", false},

		// Case folding, ASCII and Unicode simple folding.
		{"ascii_upper_suffix", "build/X.GO", true},
		{"ascii_upper_top_dir", ".CLAUDE/settings.local.json", true},
		{"kelvin_sign", "go.wor\u212a", true},
		{"long_s", "a/x.\u017f", true},

		// Bytes that are not valid UTF-8, and control characters.
		{"invalid_utf8_before_suffix", "build/\xff.go", true},
		{"control_character", "build/a\nb.go", true},

		// Not on the list.
		{"ds_store", ".DS_Store", false},
		{"swap_file", ".x.swp", false},
		{"coverage", "coverage.out", false},
		{"vscode", ".vscode/settings.json", false},
		{"venv_python", ".venv/bin/python", false},
		{"node_module_js", "extension/node_modules/pkg/index.js", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWatched(tt.path); got != tt.want {
				t.Errorf("isWatched(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestFoldEqual: foldEqual equates an invalid byte only with the same byte.
// No watch list item contains U+FFFD or an invalid byte, so isWatched cannot
// show this; the rows check foldEqual itself, which an item added to the list
// later would rely on. Reading every invalid byte as U+FFFD, as
// strings.EqualFold does, would equate the first pair and the last.
func TestFoldEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"different_invalid_bytes", "x\xff", "x\xfe", false},
		{"same_invalid_byte", "x\xff", "x\xff", true},
		{"invalid_byte_and_replacement_rune", "x\xff", "x\ufffd", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := foldEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("foldEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestWatchListPatternsMatch: every pattern the guide prints names paths
// isWatched matches, and the list has the 28 items of the requirements table
// (1 directory, 3 top-level files, 5 names, 2 suffixes, 17 Go build inputs).
func TestWatchListPatternsMatch(t *testing.T) {
	patterns := WatchListPatterns()
	if len(patterns) != 28 {
		t.Errorf("WatchListPatterns() has %d items, want 28: %q", len(patterns), patterns)
	}
	for _, pattern := range patterns {
		if p := watchSamplePath(pattern); !isWatched(p) {
			t.Errorf("WatchListPatterns lists %q, but %q is not watched", pattern, p)
		}
		// A leading or trailing "/" marks an item that matches at the top
		// level only, so the same path one directory down must not match.
		if strings.HasPrefix(pattern, "/") || strings.HasSuffix(pattern, "/") {
			if p := "a/" + watchSamplePath(pattern); isWatched(p) {
				t.Errorf("WatchListPatterns lists %q as top level only, but %q is watched", pattern, p)
			}
		}
		// A bare name matches the whole name, not its end, unlike "*.ext".
		if !strings.ContainsAny(pattern, "/*") {
			if p := "a/x" + pattern; isWatched(p) {
				t.Errorf("WatchListPatterns lists %q as a name, but %q is watched", pattern, p)
			}
		}
		// A leading "/" marks a file, so nothing below it matches.
		if strings.HasPrefix(pattern, "/") {
			if p := watchSamplePath(pattern) + "/x"; isWatched(p) {
				t.Errorf("WatchListPatterns lists %q as a file, but %q below it is watched", pattern, p)
			}
		}
	}
}

// watchSamplePath returns a path that pattern, in the guide's notation,
// describes.
func watchSamplePath(pattern string) string {
	switch {
	case strings.HasSuffix(pattern, "/"):
		return pattern + "x"
	case strings.HasPrefix(pattern, "/"):
		return strings.TrimPrefix(pattern, "/")
	case strings.HasPrefix(pattern, "*"):
		return "a/b" + strings.TrimPrefix(pattern, "*")
	default:
		return "a/" + pattern
	}
}

// wideLimits is defaultTargetLimits with a short timeout, so a test that
// narrows one limit leaves every other far from its cap.
func wideLimits() targetLimits {
	lim := defaultTargetLimits
	lim.timeout = 30 * time.Second
	return lim
}

// ignore writes a .gitignore with lines at the top of the work tree. It is
// left uncommitted: its rules apply all the same.
func (r *testRepo) ignore(lines ...string) {
	r.t.Helper()
	r.Write(".gitignore", strings.Join(lines, "\n")+"\n")
}

// collect runs one pass with the test Worktree's runners.
func collect(t *testing.T, lim targetLimits) (targetState, error) {
	t.Helper()
	w, _ := newTestWorktree(t)
	return w.collectTargets(t.Context(), lim)
}

// recordPaths returns the paths of a record, in order, unquoted.
func recordPaths(t *testing.T, record []byte) []string {
	t.Helper()
	var paths []string
	for line := range strings.Lines(string(record)) {
		_, quoted, ok := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
		p, err := strconv.Unquote(quoted)
		if !ok || err != nil {
			t.Fatalf("record line %q is not a digest and a quoted path", line)
		}
		paths = append(paths, p)
	}
	return paths
}

func mkfifo(t *testing.T, name string) {
	t.Helper()
	if err := syscall.Mkfifo(name, 0o600); err != nil {
		t.Fatalf("mkfifo %s: %v", name, err)
	}
}

func symlink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Fatalf("symlink %s: %v", name, err)
	}
}

func mkdir(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(name, testDirMode); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
}

// TestParseIgnoredListing: only the listing shape is accepted; every other
// shape fails the whole listing, with nothing repaired or skipped.
func TestParseIgnoredListing(t *testing.T) {
	tests := []struct {
		name      string
		out       string
		wantFiles []string
		wantDirs  []string
		wantErr   error
	}{
		{"empty", "", nil, nil, nil},
		{"files_and_directory", "a.go\x00build/x.go\x00build/nested/\x00", []string{"a.go", "build/x.go"}, []string{"build/nested"}, nil},
		{"any_bytes_in_a_name", "a\nb\x00\xff.go\x00", []string{"a\nb", "\xff.go"}, nil, nil},
		{"no_final_nul", "a.go\x00b.go", nil, nil, errListIgnoredShape},
		// The rows from empty_item to slash_only all reach the one check for
		// an empty element; each is a shape the design lists.
		{"empty_item", "a.go\x00\x00", nil, nil, errListIgnoredShape},
		{"only_nul", "\x00", nil, nil, errListIgnoredShape},
		{"duplicate", "a.go\x00a.go\x00", nil, nil, errListIgnoredShape},
		{"absolute", "/a.go\x00", nil, nil, errListIgnoredShape},
		{"empty_element", "a//b.go\x00", nil, nil, errListIgnoredShape},
		{"dot_element", "a/./b.go\x00", nil, nil, errListIgnoredShape},
		{"dot_dot_element", "a/../b.go\x00", nil, nil, errListIgnoredShape},
		{"two_trailing_slashes", "build/nested//\x00", nil, nil, errListIgnoredShape},
		{"slash_only", "/\x00", nil, nil, errListIgnoredShape},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, dirs, err := parseIgnoredListing([]byte(tt.out))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(files, tt.wantFiles) || !slices.Equal(dirs, tt.wantDirs) {
				t.Errorf("files, dirs = %q, %q; want %q, %q", files, dirs, tt.wantFiles, tt.wantDirs)
			}
		})
	}
}

// TestCollectTargetsGitFailures: a listing git fails, warns about, or
// overflows stops the pass, each with its own rule.
func TestCollectTargetsGitFailures(t *testing.T) {
	listCall := cmdrunnertestutil.Step{Name: gitCommand, Args: slices.Concat(gitHardening, listIgnoredArgs)}
	t.Run("unreadable_directory_warning", func(t *testing.T) {
		requireNonRoot(t)
		for _, dir := range []string{"build/locked", ".claude/locked"} {
			t.Run(strings.Split(dir, "/")[0], func(t *testing.T) {
				r := newTestRepo(t)
				r.ignore("build/", ".claude/")
				r.Write(dir+"/x.go", "package x\n")
				chmodForTest(t, filepath.Join(r.Dir, dir), 0o311)
				if _, err := collect(t, wideLimits()); !errors.Is(err, errListIgnoredFailed) {
					t.Errorf("error = %v, want %v", err, errListIgnoredFailed)
				}
			})
		}
	})
	exitStep, startStep := listCall, listCall
	exitStep.Err = &cmdrunner.ExitError{Code: 128}
	startStep.Err = exec.ErrNotFound
	for name, step := range map[string]cmdrunnertestutil.Step{"nonzero_exit": exitStep, "not_started": startStep} {
		t.Run(name, func(t *testing.T) {
			newTestRepo(t)
			w, router := newTestWorktree(t)
			router.List = cmdrunnertestutil.New(t, step)
			if _, err := w.collectTargets(t.Context(), wideLimits()); !errors.Is(err, errListIgnoredFailed) {
				t.Errorf("error = %v, want %v", err, errListIgnoredFailed)
			}
		})
	}
	t.Run("output_over_the_cap", func(t *testing.T) {
		r := newTestRepo(t)
		r.ignore("build/")
		r.Write("build/0123456789.txt", "x")
		w, router := newTestWorktree(t)
		router.List = cmdrunner.NewOS(cmdrunner.Options{Env: GitEnv(os.Environ()), StderrIsFailure: true, MaxOutput: 8})
		if _, err := w.collectTargets(t.Context(), wideLimits()); !errors.Is(err, errListIgnoredCap) {
			t.Errorf("error = %v, want %v", err, errListIgnoredCap)
		}
	})
}

// TestCollectTargetsWalk: a nested repository in an ignored directory is
// one listed item, and the files below it are found by walking it.
func TestCollectTargetsWalk(t *testing.T) {
	setup := func(t *testing.T) *testRepo {
		t.Helper()
		r := newTestRepo(t)
		r.ignore("build/")
		r.gitIn(r.Dir, "init", "-q", "--template=", "build/nested")
		r.Write("build/nested/x.go", "package x\n")
		r.Write("build/nested/sub/y.go", "package y\n")
		r.Write("build/nested/readme.txt", "not watched\n")
		mkfifo(t, filepath.Join(r.Dir, "build/nested/pipe.go"))
		symlink(t, "x.go", filepath.Join(r.Dir, "build/nested/link.go"))
		return r
	}
	t.Run("nested_repository", func(t *testing.T) {
		setup(t)
		state, err := collect(t, wideLimits())
		if err != nil {
			t.Fatalf("collectTargets: %v", err)
		}
		// settings.local.json is in although it does not exist and git does
		// not ignore it, and so is the link; the FIFO and readme.txt are not.
		want := []string{settingsLocalPath, "build/nested/link.go", "build/nested/sub/y.go", "build/nested/x.go"}
		if got := recordPaths(t, state.record); !slices.Equal(got, want) {
			t.Errorf("targets = %q, want %q", got, want)
		}
	})
	// A guard against building the targets as a list that keeps duplicates.
	t.Run("settings_listed_once", func(t *testing.T) {
		r := newTestRepo(t)
		r.ignore(".claude/")
		r.Write(settingsLocalPath, "{}\n")
		state, err := collect(t, wideLimits())
		if err != nil {
			t.Fatalf("collectTargets: %v", err)
		}
		if got := recordPaths(t, state.record); !slices.Equal(got, []string{settingsLocalPath}) {
			t.Errorf("targets = %q, want settings.local.json once", got)
		}
	})
	t.Run("directory_item_replaced", func(t *testing.T) {
		t.Chdir(t.TempDir())
		writeFile(t, "file", "x")
		mkdir(t, "dir")
		symlink(t, "dir", "link")
		walked := 0
		for _, item := range []string{"file", "link"} {
			err := walkIgnoredDir(t.Context(), item, &walked, wideLimits(), func(string) error { return nil })
			if !errors.Is(err, errReadIgnoredDir) {
				t.Errorf("%s: error = %v, want %v", item, err, errReadIgnoredDir)
			}
		}
	})
	t.Run("unreadable_directory_in_nested_repository", func(t *testing.T) {
		requireNonRoot(t)
		r := setup(t)
		mkdir(t, filepath.Join(r.Dir, "build/nested/locked"))
		chmodForTest(t, filepath.Join(r.Dir, "build/nested/locked"), 0)
		if _, err := collect(t, wideLimits()); !errors.Is(err, errReadIgnoredDir) {
			t.Errorf("error = %v, want %v", err, errReadIgnoredDir)
		}
	})
}

// TestCollectTargetsClaudeGitNames: git lists nothing below an entry named
// .git, so one below .claude stops the pass, unless it belongs to a nested
// repository git listed.
func TestCollectTargetsClaudeGitNames(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantErr error
	}{
		{"git_directory", ".claude/commands/.git/x.md", errClaudeGitName},
		{"git_directory_upper", ".claude/skills/.GIT/SKILL.md", errClaudeGitName},
		{"git_file", ".claude/agents/.git", errClaudeGitName},
		{"git_suffix_only", ".claude/commands/a.git/x.md", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRepo(t)
			r.ignore(".claude/")
			r.Write(tt.file, "x\n")
			if _, err := collect(t, wideLimits()); !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
	t.Run("nested_repository", func(t *testing.T) {
		r := newTestRepo(t)
		r.ignore(".claude/")
		r.gitIn(r.Dir, "init", "-q", "--template=", ".claude/worktrees/w")
		if _, err := collect(t, wideLimits()); err != nil {
			t.Errorf("collectTargets: %v, want a listed nested repository's .git allowed", err)
		}
	})
	t.Run("linked_worktree", func(t *testing.T) {
		r := newTestRepo(t)
		r.ignore(".claude/")
		r.AddWorktree(filepath.Join(r.Dir, ".claude/worktrees/w"), "other")
		if _, err := collect(t, wideLimits()); err != nil {
			t.Errorf("collectTargets: %v, want an ignored worktree's .git file allowed", err)
		}
	})
	t.Run("claude_is_a_nested_repository", func(t *testing.T) {
		r := newTestRepo(t)
		r.ignore(".claude/")
		r.gitIn(r.Dir, "init", "-q", "--template=", ".claude")
		if _, err := collect(t, wideLimits()); err != nil {
			t.Errorf("collectTargets: %v, want .claude/.git allowed when git lists .claude", err)
		}
	})
}

// blockingRunner stands for a listing that does not return: it ignores the
// context and waits until the test ends, or a few seconds at most, and then
// fails, so the abandoned pass ends without touching the file system.
type blockingRunner struct{ release <-chan struct{} }

func (b blockingRunner) Run(context.Context, string, ...string) ([]byte, error) {
	select {
	case <-b.release:
	case <-time.After(5 * time.Second):
	}
	return nil, context.Canceled
}

// TestCollectTargetsLimits: each count limit stops the pass at its cap and
// not before, and the pass is abandoned at the timeout. Each row first
// passes with the cap at what the work tree holds, so no other limit is
// what stops it one below.
func TestCollectTargetsLimits(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(r *testRepo)
		limit   func(lim *targetLimits) *int
		held    func(t *testing.T) int
		wantErr error
	}{
		{
			name: "target_files",
			setup: func(r *testRepo) {
				r.ignore("build/")
				r.Write("build/a.go", "package a\n")
				r.Write("build/b.go", "package b\n")
			},
			limit: func(lim *targetLimits) *int { return &lim.files },
			// settings.local.json and the two files.
			held:    func(*testing.T) int { return 3 },
			wantErr: errIgnoredTargetCount,
		},
		{
			name: "walked_entries",
			setup: func(r *testRepo) {
				r.ignore("build/")
				r.gitIn(r.Dir, "init", "-q", "--template=", "build/nested")
				r.Write("build/nested/a/x.go", "package x\n")
			},
			limit:   func(lim *targetLimits) *int { return &lim.walkEntries },
			held:    func(t *testing.T) int { return walkedEntries(t, "build/nested") },
			wantErr: errIgnoredWalkEntries,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRepo(t)
			tt.setup(r)
			lim := wideLimits()
			*tt.limit(&lim) = tt.held(t)
			if _, err := collect(t, lim); err != nil {
				t.Fatalf("at the cap: %v, want no error", err)
			}
			*tt.limit(&lim)--
			if _, err := collect(t, lim); !errors.Is(err, tt.wantErr) {
				t.Errorf("one below the cap: %v, want %v", err, tt.wantErr)
			}
		})
	}
	t.Run("timeout", func(t *testing.T) {
		newTestRepo(t)
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		w, router := newTestWorktree(t)
		router.List = blockingRunner{release: release}
		lim := wideLimits()
		lim.timeout = 100 * time.Millisecond
		start := time.Now()
		_, err := w.collectTargets(t.Context(), lim)
		if !errors.Is(err, errIgnoredTimeout) {
			t.Errorf("error = %v, want %v", err, errIgnoredTimeout)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("collectTargets took %v, want it bounded by the timeout", elapsed)
		}
	})
}

// walkedEntries counts what walkIgnoredDir visits below dir.
func walkedEntries(t *testing.T, dir string) int {
	t.Helper()
	n := -1 // dir itself
	if err := filepath.WalkDir(dir, func(string, fs.DirEntry, error) error { n++; return nil }); err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return n
}

// stateOf builds the state of paths in the current directory.
func stateOf(t *testing.T, lim targetLimits, paths ...string) (targetState, error) {
	t.Helper()
	return targetStateOf(t.Context(), paths, lim, os.Lstat)
}

// stateWithin is stateOf, failing the test instead of hanging if the pass
// blocks on a special file.
func stateWithin(t *testing.T, paths ...string) (targetState, error) {
	t.Helper()
	type result struct {
		state targetState
		err   error
	}
	done := make(chan result, 1)
	go func() {
		state, err := targetStateOf(t.Context(), paths, wideLimits(), os.Lstat)
		done <- result{state, err}
	}()
	select {
	case r := <-done:
		return r.state, r.err
	case <-time.After(5 * time.Second):
		t.Fatal("the pass blocked")
		return targetState{}, nil
	}
}

// writeFile writes name in the current directory.
func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestTargetEntries: what a target file's entry cannot record stops the
// pass with its rule, and a link that resolves to nothing is recorded.
func TestTargetEntries(t *testing.T) {
	t.Run("stops", func(t *testing.T) {
		tests := []struct {
			name    string
			setup   func(t *testing.T)
			wantErr error
		}{
			{"unreadable_file", func(t *testing.T) {
				requireNonRoot(t)
				writeFile(t, "x.go", "package x\n")
				chmodForTest(t, "x.go", 0)
			}, errReadIgnoredFile},
			{"link_to_directory", func(t *testing.T) {
				mkdir(t, "dir")
				symlink(t, "dir", "x.go")
			}, errIgnoredLinkToDir},
			{"link_target_not_searchable", func(t *testing.T) {
				requireNonRoot(t)
				mkdir(t, "locked")
				writeFile(t, "locked/target", "x")
				symlink(t, "locked/target", "x.go")
				chmodForTest(t, "locked", 0)
			}, errReadIgnoredFile},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				tt.setup(t)
				if _, err := stateOf(t, wideLimits(), "x.go"); !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("unresolved_links_recorded", func(t *testing.T) {
		t.Chdir(t.TempDir())
		writeFile(t, "regular", "x")
		symlink(t, "missing", "enoent.go")
		symlink(t, "loop_b.go", "loop_a.go")
		symlink(t, "loop_a.go", "loop_b.go")
		symlink(t, "regular/x", "enotdir.go")
		links := []string{"enoent.go", "enotdir.go", "loop_a.go"}
		before, err := stateOf(t, wideLimits(), links...)
		if err != nil {
			t.Fatalf("error = %v, want the links recorded as unresolved", err)
		}
		// Once the link resolves, its record line changes.
		writeFile(t, "missing", "x")
		after, err := stateOf(t, wideLimits(), links...)
		if err != nil {
			t.Fatal(err)
		}
		if before.digest == after.digest {
			t.Errorf("a link that came to resolve left the digest unchanged")
		}
	})

	// A special file is recorded by type, never opened: a link to a FIFO,
	// settings.local.json as a FIFO, and settings.local.json below a .claude
	// that is a regular file (recorded as absent).
	special := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"link_to_fifo", func(t *testing.T) { mkfifo(t, "pipe"); symlink(t, "pipe", "x.go") }},
		{"settings_fifo", func(t *testing.T) { mkdir(t, ".claude"); mkfifo(t, settingsLocalPath) }},
		{"claude_is_a_file", func(t *testing.T) { writeFile(t, ".claude", "not a directory") }},
	}
	for _, tt := range special {
		t.Run("recorded/"+tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			tt.setup(t)
			if _, err := stateWithin(t, "x.go", settingsLocalPath); err != nil {
				t.Errorf("error = %v, want the file recorded", err)
			}
		})
	}

	// A sparse file of size bytes reached through five links: the file cap
	// counts one file, the total cap every read of it.
	const size = 1 << 20
	sizeTests := []struct {
		name     string
		atCap    func(lim *targetLimits)
		belowCap func(lim *targetLimits)
		wantErr  error
	}{
		{
			name:     "file_size",
			atCap:    func(lim *targetLimits) { lim.fileBytes = size },
			belowCap: func(lim *targetLimits) { lim.fileBytes = size - 1 },
			wantErr:  errIgnoredFileSize,
		},
		{
			name:     "read_total_through_links",
			atCap:    func(lim *targetLimits) { lim.fileBytes, lim.totalBytes = size, 5*size },
			belowCap: func(lim *targetLimits) { lim.fileBytes, lim.totalBytes = size, 5*size-1 },
			wantErr:  errIgnoredReadTotal,
		},
	}
	for _, tt := range sizeTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("sparse", nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate("sparse", size); err != nil {
				t.Fatal(err)
			}
			var links []string
			for i := range 5 {
				name := "l" + strconv.Itoa(i) + ".go"
				symlink(t, "sparse", name)
				links = append(links, name)
			}
			lim := wideLimits()
			tt.atCap(&lim)
			if _, err := stateOf(t, lim, links...); err != nil {
				t.Fatalf("at the cap: %v, want no error", err)
			}
			tt.belowCap(&lim)
			if _, err := stateOf(t, lim, links...); !errors.Is(err, tt.wantErr) {
				t.Errorf("one below the cap: %v, want %v", err, tt.wantErr)
			}
		})
	}

	// A regular file swapped for a FIFO between lstat and the open fails the
	// open instead of blocking until the timeout.
	t.Run("fifo_swapped_in_after_lstat", func(t *testing.T) {
		r := newTestRepo(t)
		r.ignore(".envrc")
		r.Write(".envrc", "export A=1\n")
		swap := func(name string) (fs.FileInfo, error) {
			info, err := os.Lstat(name)
			if name == ".envrc" {
				// This runs on the pass's goroutine, so it reports with
				// Errorf, never Fatalf.
				if err := os.Remove(name); err != nil {
					t.Errorf("remove: %v", err)
				}
				if err := syscall.Mkfifo(name, 0o600); err != nil {
					t.Errorf("mkfifo: %v", err)
				}
			}
			return info, err
		}
		// If the open blocked, opening the write side releases it.
		t.Cleanup(func() {
			if f, err := os.OpenFile(filepath.Join(r.Dir, ".envrc"), os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				_ = f.Close()
			}
		})
		w, _ := newTestWorktree(t)
		lim := wideLimits()
		lim.timeout = 2 * time.Second
		if _, err := w.collectTargetsWith(t.Context(), lim, swap); !errors.Is(err, errReadIgnoredFile) {
			t.Errorf("error = %v, want %v", err, errReadIgnoredFile)
		}
	})
}

// TestHashCapped: content longer than the size the file reported (a file
// extended after the check, a file system that reports 0) is not read past
// either cap.
func TestHashCapped(t *testing.T) {
	content := strings.Repeat("x", 1000)
	tests := []struct {
		name      string
		fileBytes int64
		read      int64 // already read in this pass
		wantErr   error
	}{
		{"within_both_caps", 1000, 0, nil},
		{"over_the_file_cap", 9, 0, errIgnoredFileSize},
		{"over_the_total", 1000, 1, errIgnoredReadTotal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lim := wideLimits()
			lim.fileBytes, lim.totalBytes = tt.fileBytes, 1000
			total := tt.read
			if _, err := hashCapped(strings.NewReader(content), 0, lim, &total); !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if read, bound := total-tt.read, min(lim.fileBytes, lim.totalBytes-tt.read)+1; read > bound {
				t.Errorf("read %d bytes, want at most %d (one past the smaller cap)", read, bound)
			}
		})
	}
}

// TestTargetStateDigest: what does not change what runs leaves the digest
// unchanged, and a value
// cannot pass for an entry boundary. The one path below, unquoted, would
// write the same bytes as the absent entries of m and n.
func TestTargetStateDigest(t *testing.T) {
	t.Chdir(t.TempDir())
	writeFile(t, "a.go", "package a\n")
	first, err := stateOf(t, wideLimits(), "a.go")
	if err != nil {
		t.Fatal(err)
	}
	// The modification time and the read and write bits are not recorded.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes("a.go", old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod("a.go", 0o644); err != nil { //nolint:gosec // a file in the test's temporary directory
		t.Fatal(err)
	}
	again, err := stateOf(t, wideLimits(), "a.go")
	if err != nil {
		t.Fatal(err)
	}
	if first.digest != again.digest {
		t.Errorf("a change of time or read and write bits changed the digest")
	}
	two, err := stateOf(t, wideLimits(), "m", "n")
	if err != nil {
		t.Fatal(err)
	}
	one, err := stateOf(t, wideLimits(), "m\nabsent\n1\nn")
	if err != nil {
		t.Fatal(err)
	}
	if one.digest == two.digest {
		t.Errorf("one path made of entry fields has the digest of two absent entries")
	}
	// Without the kind markers, a dangling link a -> b would write the
	// values of the absent entries of a, b, and unresolved.
	symlink(t, "b", "a")
	link, err := stateOf(t, wideLimits(), "a")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	absent, err := stateOf(t, wideLimits(), "a", "b", "unresolved")
	if err != nil {
		t.Fatal(err)
	}
	if link.digest == absent.digest {
		t.Errorf("a dangling link has the digest of three absent entries")
	}
}

// TestTargetRecordLines: each line is a 64-digit hex digest, a space, and
// the path quoted in printable ASCII; a change to one file changes its line
// only.
func TestTargetRecordLines(t *testing.T) {
	t.Chdir(t.TempDir())
	// A name that is not valid UTF-8 cannot be created on APFS, so the last
	// two are recorded as absent; the others exist. The fourth is valid
	// UTF-8 that is not ASCII.
	paths := []string{"a\nb.go", "b.go", "c\x01.go", "\xc3\xa9.go", "\xff.go"}
	for _, p := range paths[:3] {
		writeFile(t, p, p)
	}
	state, err := stateOf(t, wideLimits(), paths...)
	if err != nil {
		t.Fatal(err)
	}
	lines := slices.Collect(strings.Lines(string(state.record)))
	if len(lines) != len(paths) {
		t.Fatalf("record has %d lines, want %d: %q", len(lines), len(paths), state.record)
	}
	for i, line := range lines {
		digest, quoted, _ := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
		if !validHex(digest, digestLength) {
			t.Errorf("line %d digest = %q, want 64 hex digits", i, digest)
		}
		for _, c := range []byte(quoted) {
			if c < ' ' || c > '~' {
				t.Errorf("line %d = %q, want printable ASCII", i, line)
				break
			}
		}
		if got, err := strconv.Unquote(quoted); err != nil || got != paths[i] {
			t.Errorf("line %d path = %q (%v), want %q", i, got, err, paths[i])
		}
	}
	writeFile(t, "b.go", "changed")
	changed, err := stateOf(t, wideLimits(), paths...)
	if err != nil {
		t.Fatal(err)
	}
	after := slices.Collect(strings.Lines(string(changed.record)))
	if len(after) != len(lines) {
		t.Fatalf("record has %d lines after the change, want %d", len(after), len(lines))
	}
	for i := range lines {
		if (lines[i] != after[i]) != (paths[i] == "b.go") {
			t.Errorf("line %d: before %q, after %q; want only b.go's line changed", i, lines[i], after[i])
		}
	}
}

// TestDefaultLimitsAcceptDependencyTree: an ignored dependency tree of
// thousands of files that are not on the watch list, one nested repository
// among them, with C headers and a Makefile that are, stays within the
// default limits. It is a smoke test with a wide margin: the walk cap would
// have to drop below about 2,000 to fail it. The listing's own output cap is
// osEnv's runner's, not exercised here.
func TestDefaultLimitsAcceptDependencyTree(t *testing.T) {
	r := newTestRepo(t)
	r.ignore("node_modules/")
	for i := range 3000 {
		r.Write("node_modules/pkg"+strconv.Itoa(i%100)+"/f"+strconv.Itoa(i)+".js", "module.exports = 1;\n")
	}
	// A dependency cloned with its history is a nested repository: git
	// lists it as one item and the tool walks it.
	r.gitIn(r.Dir, "init", "-q", "--template=", "node_modules/cloned")
	for i := range 2000 {
		r.Write("node_modules/cloned/lib/f"+strconv.Itoa(i)+".js", "module.exports = 1;\n")
	}
	for i := range 20 {
		r.Write("node_modules/native"+strconv.Itoa(i)+"/include/x.h", "#define X 1\n")
		r.Write("node_modules/native"+strconv.Itoa(i)+"/Makefile", "all:\n")
	}
	state, err := collect(t, defaultTargetLimits)
	if err != nil {
		t.Fatalf("collectTargets: %v", err)
	}
	if got := len(recordPaths(t, state.record)); got != 41 {
		t.Errorf("targets = %d, want 41 (40 files and settings.local.json)", got)
	}
}
