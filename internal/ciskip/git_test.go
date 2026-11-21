//go:build test

package ciskip

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

func TestResolveSyntheticMergeBase(t *testing.T) {
	t.Run("valid_two_parents_matching_head", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "git",
			Args: gitArgs("rev-list", "--parents", "-n", "1", "HEAD"),
			Out:  testSHA3 + " " + testSHA1 + " " + testSHA2 + "\n",
		})
		base, reason, ok := resolveSyntheticMergeBase(t.Context(), fake, time.Second, testSHA2)
		if !ok || reason != ReasonUnspecified || base != testSHA1 {
			t.Errorf("got (%q, %q, %v), want (%q, %q, true)", base, reason, ok, testSHA1, ReasonUnspecified)
		}
	})

	cases := []struct {
		name string
		out  string
		err  error
		want Reason
	}{
		{name: "head2_mismatch", out: testSHA3 + " " + testSHA1 + " " + testSHA4 + "\n", want: ReasonMergeCommitMismatch},
		{name: "single_parent_not_merge", out: testSHA2 + " " + testSHA1 + "\n", want: ReasonMergeCommitMismatch},
		{name: "root_commit", out: testSHA2 + "\n", want: ReasonMergeCommitMismatch},
		{name: "three_parents_octopus", out: testSHA4 + " " + testSHA1 + " " + testSHA2 + " " + testSHA3 + "\n", want: ReasonMergeCommitMismatch},
		{name: "missing_trailing_newline", out: testSHA3 + " " + testSHA1 + " " + testSHA2, want: ReasonGitFailed},
		{name: "empty_output", out: "", want: ReasonGitFailed},
		{name: "uppercase_sha", out: strings.Repeat("A", 40) + " " + testSHA1 + " " + testSHA2 + "\n", want: ReasonGitFailed},
		{name: "extra_line", out: testSHA3 + " " + testSHA1 + " " + testSHA2 + "\n" + testSHA4 + "\n", want: ReasonGitFailed},
		{name: "git_error", err: errors.New("exit 128"), want: ReasonGitFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "git",
				Args: gitArgs("rev-list", "--parents", "-n", "1", "HEAD"),
				Out:  tc.out,
				Err:  tc.err,
			})
			base, reason, ok := resolveSyntheticMergeBase(t.Context(), fake, time.Second, testSHA2)
			if ok || base != "" || reason != tc.want {
				t.Errorf("got (%q, %q, %v), want (\"\", %q, false)", base, reason, ok, tc.want)
			}
		})
	}
}

func TestCheckOriginMain(t *testing.T) {
	cases := []struct {
		name       string
		out        string
		err        error
		wantReason Reason
		wantOK     bool
	}{
		{name: "matches", out: testSHA1 + "\n", wantReason: ReasonUnspecified, wantOK: true},
		{name: "moved", out: testSHA2 + "\n", wantReason: ReasonMainMovedAtCheckout, wantOK: false},
		{name: "missing_trailing_newline", out: testSHA1, wantReason: ReasonGitFailed, wantOK: false},
		{name: "malformed_output", out: "not-a-sha\n", wantReason: ReasonGitFailed, wantOK: false},
		{name: "git_error", err: errors.New("unknown revision"), wantReason: ReasonGitFailed, wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "git",
				Args: gitArgs("rev-parse", "--verify", originMainCommitRef),
				Out:  tc.out,
				Err:  tc.err,
			})
			reason, ok := verifyOriginMain(t.Context(), fake, time.Second, testSHA1)
			if ok != tc.wantOK || reason != tc.wantReason {
				t.Errorf("got (%q, %v), want (%q, %v)", reason, ok, tc.wantReason, tc.wantOK)
			}
		})
	}
}

func TestVerifyParentCount(t *testing.T) {
	t.Run("single_parent_fake", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "git",
			Args: gitArgs("rev-list", "--parents", "-n", "1", testSHA2),
			Out:  testSHA2 + " " + testSHA1 + "\n",
		})
		parent, reason, ok := resolveSingleParent(t.Context(), fake, time.Second, testSHA2)
		if !ok || reason != ReasonUnspecified || parent != testSHA1 {
			t.Errorf("got (%q, %q, %v), want (%q, %q, true)", parent, reason, ok, testSHA1, ReasonUnspecified)
		}
	})

	fakeCases := []struct {
		name string
		out  string
		err  error
		want Reason
	}{
		{name: "root_commit", out: testSHA2 + "\n", want: ReasonNotSingleParent},
		{name: "merge_two_parents", out: testSHA2 + " " + testSHA1 + " " + testSHA3 + "\n", want: ReasonNotSingleParent},
		{name: "merge_three_parents", out: testSHA2 + " " + testSHA1 + " " + testSHA3 + " " + testSHA4 + "\n", want: ReasonNotSingleParent},
		{name: "missing_trailing_newline", out: testSHA2 + " " + testSHA1, want: ReasonGitFailed},
		{name: "unexpected_commit_sha", out: testSHA4 + " " + testSHA1 + "\n", want: ReasonGitFailed},
		{name: "git_error", err: errors.New("missing object"), want: ReasonGitFailed},
	}
	for _, tc := range fakeCases {
		t.Run(tc.name+"_fake", func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "git",
				Args: gitArgs("rev-list", "--parents", "-n", "1", testSHA2),
				Out:  tc.out,
				Err:  tc.err,
			})
			parent, reason, ok := resolveSingleParent(t.Context(), fake, time.Second, testSHA2)
			if ok || parent != "" || reason != tc.want {
				t.Errorf("got (%q, %q, %v), want (\"\", %q, false)", parent, reason, ok, tc.want)
			}
		})
	}

	t.Run("real_git_root_single_and_merge", func(t *testing.T) {
		repo := newTestGitRepo(t)
		gitRunner, _ := repo.runners()

		root := repo.commit("root", map[string]string{"docs/a.md": "v1\n"})
		side := repo.commit("side", map[string]string{"docs/b.md": "v1\n"}, root)
		single := repo.commit("single", map[string]string{"docs/a.md": "v2\n"}, root)
		merge := repo.commit("merge", map[string]string{"docs/a.md": "v2\n", "docs/b.md": "v1\n"}, single, side)

		if parent, reason, ok := resolveSingleParent(t.Context(), gitRunner, 5*time.Second, root); ok || parent != "" || reason != ReasonNotSingleParent {
			t.Errorf("root commit: got (%q, %q, %v), want (\"\", %q, false)", parent, reason, ok, ReasonNotSingleParent)
		}
		if parent, reason, ok := resolveSingleParent(t.Context(), gitRunner, 5*time.Second, single); !ok || reason != ReasonUnspecified || parent != root {
			t.Errorf("single-parent commit: got (%q, %q, %v), want (%q, %q, true)", parent, reason, ok, root, ReasonUnspecified)
		}
		if parent, reason, ok := resolveSingleParent(t.Context(), gitRunner, 5*time.Second, merge); ok || parent != "" || reason != ReasonNotSingleParent {
			t.Errorf("merge commit: got (%q, %q, %v), want (\"\", %q, false)", parent, reason, ok, ReasonNotSingleParent)
		}
	})
}

func TestValidateDepDirsFile(t *testing.T) {
	t.Run("valid_typical", func(t *testing.T) {
		p := writeTempFile(t, "deps", []byte("internal/llm/\ninternal/writer/\n"))
		if reason, ok := validateDepDirsFile(p, defaultScriptMaxOutput); !ok || reason != ReasonUnspecified {
			t.Errorf("got (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}
	})

	t.Run("valid_exact_64KiB", func(t *testing.T) {
		line := "internal/pkg/\n"
		repeat := defaultScriptMaxOutput / len(line)
		rem := defaultScriptMaxOutput % len(line)
		var buf bytes.Buffer
		for range repeat {
			buf.WriteString(line)
		}
		if rem > 0 {
			// Fill the exact remaining bytes with a valid "/"-terminated directory name.
			buf.WriteString(strings.Repeat("a", rem-1))
			buf.WriteByte('/')
		}
		if buf.Len() != defaultScriptMaxOutput {
			t.Fatalf("built %d bytes, want %d", buf.Len(), defaultScriptMaxOutput)
		}
		p := writeTempFile(t, "deps-exact", buf.Bytes())
		if reason, ok := validateDepDirsFile(p, defaultScriptMaxOutput); !ok || reason != ReasonUnspecified {
			t.Errorf("exact 64 KiB: got (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}
	})

	// Build a strictly defaultScriptMaxOutput + 1 byte file whose single line ends in "/".
	oversize := make([]byte, defaultScriptMaxOutput+1)
	for i := range oversize {
		oversize[i] = 'a'
	}
	oversize[len(oversize)-1] = '/'

	invalidCases := []struct {
		name    string
		content []byte
	}{
		{name: "empty_file", content: nil},
		{name: "blank_lines_only", content: []byte("\n\n\n")},
		{name: "missing_trailing_slash", content: []byte("internal/llm\n")},
		{name: "crlf_line", content: []byte("internal/llm/\r\n")},
		{name: "whitespace_only_line", content: []byte("internal/llm/\n   \n")},
		{name: "nul_byte", content: []byte("internal/llm/\x00\n")},
		{name: "exceeds_64KiB_by_1", content: oversize},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeTempFile(t, tc.name, tc.content)
			if reason, ok := validateDepDirsFile(p, defaultScriptMaxOutput); ok || reason != ReasonWasmDepDirsFailed {
				t.Errorf("got (%q, %v), want (%q, false)", reason, ok, ReasonWasmDepDirsFailed)
			}
		})
	}

	dir := t.TempDir()
	pathCases := []struct {
		name string
		path string
	}{
		{name: "empty_path", path: ""},
		{name: "directory_path", path: dir},
		{name: "missing_file", path: filepath.Join(dir, "does-not-exist")},
	}
	for _, tc := range pathCases {
		t.Run(tc.name, func(t *testing.T) {
			if reason, ok := validateDepDirsFile(tc.path, defaultScriptMaxOutput); ok || reason != ReasonWasmDepDirsFailed {
				t.Errorf("got (%q, %v), want (%q, false)", reason, ok, ReasonWasmDepDirsFailed)
			}
		})
	}
}

func TestIsNonCodePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Non-code paths
		{path: "README.md", want: true},
		{path: "README.en.md", want: true},
		{path: "docs/tasks/0119_ci_skip_doc_only_commit/03_implementation_plan.md", want: true},
		{path: "docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh", want: true},
		{path: "docs/dev/adr/0054-ci-skip-doc-only-commit.md", want: true},
		{path: ".claude/commands/runplan.md", want: true},
		{path: ".claude/commands/_lib/review-records.sh", want: true},
		{path: "LICENSE", want: true},
		{path: "internal/llm/devfake/articles/sample.md", want: true},
		{path: "extension/README.md", want: true},

		// Code paths
		{path: "", want: false},
		{path: "internal/foo/a.go", want: false},
		{path: "internal/ciskip/ciskip.go", want: false},
		{path: "cmd/ciskip/main.go", want: false},
		{path: ".github/workflows/ci.yml", want: false},
		{path: "Makefile", want: false},
		{path: "go.mod", want: false},
		{path: "go.sum", want: false},
		{path: "extension/src/popup.ts", want: false},
		{path: "extension/scripts/has-extension-changes.sh", want: false},
		{path: "extension/scripts/wasm-dep-dirs.sh", want: false},
		{path: "LICENSE.txt", want: false},
		{path: "sub/LICENSE", want: false},
		{path: "sub/docs/a.go", want: false},
		{path: "sub/.claude/a.sh", want: false},
		{path: "docs", want: false},
		{path: ".claude", want: false},
		{path: "foo.md.go", want: false},
		{path: "foo.MD", want: false},
		{path: "internal/foo.md\na.go", want: false},
		{path: "internal/a.go\ndocs/x.md", want: true}, // ends in .md, same as ci.yml regex \.md$
	}

	for _, tc := range cases {
		if got := isNonCodePath(tc.path); got != tc.want {
			t.Errorf("isNonCodePath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestClassifyParentDiff(t *testing.T) {
	depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))
	diffCmdArgs := gitArgs("-c", "core.quotePath=false", "diff", "-z", "--no-renames", "--name-only", testSHA1, testSHA2, "--")

	t.Run("empty_commit_skips_script", func(t *testing.T) {
		gitFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "git",
			Args: diffCmdArgs,
			Out:  "",
		})
		// Zero scripted steps: fails the test if has-extension-changes.sh is invoked.
		scriptFake := cmdrunnertestutil.New(t)
		reason, ok := classifyParentDiff(t.Context(), gitFake, scriptFake, time.Second, testSHA1, testSHA2, depFile, defaultScriptMaxOutput)
		if !ok || reason != ReasonUnspecified {
			t.Errorf("got (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}
	})

	t.Run("invalid_dep_dirs_stops_before_git", func(t *testing.T) {
		// Neither git nor script may be called when depDirsFile fails validation.
		gitFake := cmdrunnertestutil.New(t)
		scriptFake := cmdrunnertestutil.New(t)
		reason, ok := classifyParentDiff(t.Context(), gitFake, scriptFake, time.Second, testSHA1, testSHA2, depFile, 4)
		if ok || reason != ReasonWasmDepDirsFailed {
			t.Errorf("got (%q, %v), want (%q, false)", reason, ok, ReasonWasmDepDirsFailed)
		}
	})

	t.Run("non_code_non_extension_passes", func(t *testing.T) {
		raw := "docs/tasks/0119_ci_skip_doc_only_commit/03_implementation_plan.md\x00.claude/commands/runplan.md\x00LICENSE\x00"
		gitFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "git",
			Args: diffCmdArgs,
			Out:  raw,
		})
		scriptFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "bash",
			Args: []string{hasExtensionChangesScript, depFile},
			Out:  "false\n",
		})
		reason, ok := classifyParentDiff(t.Context(), gitFake, scriptFake, time.Second, testSHA1, testSHA2, depFile, defaultScriptMaxOutput)
		if !ok || reason != ReasonUnspecified {
			t.Errorf("got (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}
		if len(scriptFake.Stdin) != 1 || string(scriptFake.Stdin[0]) != raw {
			t.Errorf("script stdin = %q, want %q", scriptFake.Stdin, raw)
		}
	})

	t.Run("code_change_stops_before_script", func(t *testing.T) {
		gitFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "git",
			Args: diffCmdArgs,
			Out:  "docs/a.md\x00internal/ciskip/git.go\x00",
		})
		scriptFake := cmdrunnertestutil.New(t)
		reason, ok := classifyParentDiff(t.Context(), gitFake, scriptFake, time.Second, testSHA1, testSHA2, depFile, defaultScriptMaxOutput)
		if ok || reason != ReasonCodeChanged {
			t.Errorf("got (%q, %v), want (%q, false)", reason, ok, ReasonCodeChanged)
		}
	})

	scriptRejectCases := []struct {
		name string
		path string
		out  string
		err  error
		want Reason
	}{
		{name: "extension_changed", path: "README.md\x00", out: "true\n", want: ReasonExtensionChanged},
		{name: "script_exit_error", path: "docs/a.md\x00", err: &cmdrunner.ExitError{Code: 2}, want: ReasonExtensionCheckFailed},
		{name: "script_unexpected_stderr", path: "docs/a.md\x00", err: cmdrunner.ErrUnexpectedStderr, want: ReasonExtensionCheckFailed},
		{name: "script_output_too_large", path: "docs/a.md\x00", err: cmdrunner.ErrOutputTooLarge, want: ReasonExtensionCheckFailed},
		{name: "script_timeout", path: "docs/a.md\x00", err: context.DeadlineExceeded, want: ReasonExtensionCheckFailed},
		{name: "script_empty_stdout", path: "docs/a.md\x00", out: "", want: ReasonExtensionCheckFailed},
		{name: "script_missing_newline", path: "docs/a.md\x00", out: "false", want: ReasonExtensionCheckFailed},
		{name: "script_garbage_stdout", path: "docs/a.md\x00", out: "maybe\n", want: ReasonExtensionCheckFailed},
	}
	for _, tc := range scriptRejectCases {
		t.Run(tc.name, func(t *testing.T) {
			gitFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "git",
				Args: diffCmdArgs,
				Out:  tc.path,
			})
			scriptFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "bash",
				Args: []string{hasExtensionChangesScript, depFile},
				Out:  tc.out,
				Err:  tc.err,
			})
			reason, ok := classifyParentDiff(t.Context(), gitFake, scriptFake, time.Second, testSHA1, testSHA2, depFile, defaultScriptMaxOutput)
			if ok || reason != tc.want {
				t.Errorf("got (%q, %v), want (%q, false)", reason, ok, tc.want)
			}
		})
	}

	diffFailCases := []struct {
		name string
		out  string
		err  error
	}{
		{name: "missing_trailing_nul", out: "docs/a.md"},
		{name: "empty_path_only_nul", out: "\x00"},
		{name: "consecutive_nuls", out: "docs/a.md\x00\x00docs/b.md\x00"},
		{name: "git_diff_error", err: errors.New("git failed")},
		{name: "git_diff_output_too_large", err: cmdrunner.ErrOutputTooLarge},
	}
	for _, tc := range diffFailCases {
		t.Run(tc.name, func(t *testing.T) {
			gitFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "git",
				Args: diffCmdArgs,
				Out:  tc.out,
				Err:  tc.err,
			})
			scriptFake := cmdrunnertestutil.New(t)
			reason, ok := classifyParentDiff(t.Context(), gitFake, scriptFake, time.Second, testSHA1, testSHA2, depFile, defaultScriptMaxOutput)
			if ok || reason != ReasonGitFailed {
				t.Errorf("got (%q, %v), want (%q, false)", reason, ok, ReasonGitFailed)
			}
		})
	}
}

func TestClassifyDiffWithRealGitAndScript(t *testing.T) {
	depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\ninternal/writer/\n"))

	t.Run("synthetic_merge_and_origin_main", func(t *testing.T) {
		repo := newTestGitRepo(t)
		gitRunner, scriptRunner := repo.runners()

		mainCommit := repo.commit("main", map[string]string{
			"internal/foo/a.go": "package foo\n",
			"docs/a.md":         "v1\n",
		})
		parentCommit := repo.commit("code change", map[string]string{
			"internal/foo/a.go": "package foo\n// edited\n",
			"docs/a.md":         "v1\n",
		}, mainCommit)
		docCommit := repo.commit("doc only", map[string]string{
			"internal/foo/a.go": "package foo\n// edited\n",
			"docs/a.md":         "v2\n",
			".claude/note.txt":  "note\n",
			"LICENSE":           "MIT\n",
		}, parentCommit)
		synthMerge := repo.commit("synthetic merge", map[string]string{
			"internal/foo/a.go": "package foo\n// edited\n",
			"docs/a.md":         "v2\n",
			".claude/note.txt":  "note\n",
			"LICENSE":           "MIT\n",
		}, mainCommit, docCommit)

		repo.setHead(synthMerge)
		repo.setOriginMain(mainCommit)

		base, reason, ok := resolveSyntheticMergeBase(t.Context(), gitRunner, 5*time.Second, docCommit)
		if !ok || reason != ReasonUnspecified || base != mainCommit {
			t.Fatalf("resolveSyntheticMergeBase = (%q, %q, %v), want (%q, %q, true)", base, reason, ok, mainCommit, ReasonUnspecified)
		}
		if reason, ok := verifyOriginMain(t.Context(), gitRunner, 5*time.Second, base); !ok || reason != ReasonUnspecified {
			t.Fatalf("verifyOriginMain = (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}
		parent, reason, ok := resolveSingleParent(t.Context(), gitRunner, 5*time.Second, docCommit)
		if !ok || reason != ReasonUnspecified || parent != parentCommit {
			t.Fatalf("resolveSingleParent = (%q, %q, %v), want (%q, %q, true)", parent, reason, ok, parentCommit, ReasonUnspecified)
		}
		if reason, ok := classifyParentDiff(t.Context(), gitRunner, scriptRunner, 5*time.Second, parent, docCommit, depFile, defaultScriptMaxOutput); !ok || reason != ReasonUnspecified {
			t.Fatalf("classifyParentDiff(doc only) = (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}

		// Advancing origin/main after synthetic merge creation must be caught.
		movedMain := repo.commit("main moved", map[string]string{"docs/other.md": "x\n"}, mainCommit)
		repo.setOriginMain(movedMain)
		if reason, ok := verifyOriginMain(t.Context(), gitRunner, 5*time.Second, base); ok || reason != ReasonMainMovedAtCheckout {
			t.Errorf("verifyOriginMain after move = (%q, %v), want (%q, false)", reason, ok, ReasonMainMovedAtCheckout)
		}
	})

	cases := []struct {
		name       string
		parentTree map[string]string
		headTree   map[string]string
		wantOK     bool
		wantReason Reason
	}{
		{
			name:       "empty_commit",
			parentTree: map[string]string{"internal/a.go": "package a\n"},
			headTree:   map[string]string{"internal/a.go": "package a\n"},
			wantOK:     true,
			wantReason: ReasonUnspecified,
		},
		{
			name:       "doc_only_with_non_ascii_path",
			parentTree: map[string]string{"docs/caf\xc3\xa9.md": "v1\n"},
			headTree:   map[string]string{"docs/caf\xc3\xa9.md": "v2\n"},
			wantOK:     true,
			wantReason: ReasonUnspecified,
		},
		{
			name:       "code_renamed_to_doc_detected_on_old_side",
			parentTree: map[string]string{"internal/foo/a.go": "package foo\n// enough text for rename detection\nfunc F() {}\n"},
			headTree:   map[string]string{"docs/a.md": "package foo\n// enough text for rename detection\nfunc F() {}\n"},
			wantOK:     false,
			wantReason: ReasonCodeChanged,
		},
		{
			name:       "readme_renamed_to_doc_detected_on_old_side",
			parentTree: map[string]string{"README.md": "# Title\nEnough identical content for git rename detection.\n"},
			headTree:   map[string]string{"docs/archived_readme.md": "# Title\nEnough identical content for git rename detection.\n"},
			wantOK:     false,
			wantReason: ReasonExtensionChanged,
		},
		{
			name:       "wasm_dep_md_renamed_to_doc_detected_on_old_side",
			parentTree: map[string]string{"internal/llm/devfake/articles/sample.md": "# Sample\nIdentical content for rename detection.\n"},
			headTree:   map[string]string{"docs/moved_sample.md": "# Sample\nIdentical content for rename detection.\n"},
			wantOK:     false,
			wantReason: ReasonExtensionChanged,
		},
		{
			name:       "readme_en_changed",
			parentTree: map[string]string{"README.en.md": "v1\n"},
			headTree:   map[string]string{"README.en.md": "v2\n"},
			wantOK:     false,
			wantReason: ReasonExtensionChanged,
		},
		{
			name:       "development_guide_changed",
			parentTree: map[string]string{"docs/dev/guides/development.md": "v1\n"},
			headTree:   map[string]string{"docs/dev/guides/development.md": "v2\n"},
			wantOK:     false,
			wantReason: ReasonExtensionChanged,
		},
		{
			name:       "extension_non_ascii_md_path",
			parentTree: map[string]string{"extension/caf\xc3\xa9.md": "v1\n"},
			headTree:   map[string]string{"extension/caf\xc3\xa9.md": "v2\n"},
			wantOK:     false,
			wantReason: ReasonExtensionChanged,
		},
		{
			name:       "extension_tab_path_md",
			parentTree: map[string]string{"extension/a\tb.md": "v1\n"},
			headTree:   map[string]string{"extension/a\tb.md": "v2\n"},
			wantOK:     false,
			wantReason: ReasonExtensionChanged,
		},
		{
			name:       "extension_newline_path_md",
			parentTree: map[string]string{"extension/a\nb.md": "v1\n"},
			headTree:   map[string]string{"extension/a\nb.md": "v2\n"},
			wantOK:     false,
			wantReason: ReasonExtensionChanged,
		},
		{
			name:       "code_non_ascii_path",
			parentTree: map[string]string{"internal/caf\xc3\xa9.go": "package x\n"},
			headTree:   map[string]string{"internal/caf\xc3\xa9.go": "package x\n// changed\n"},
			wantOK:     false,
			wantReason: ReasonCodeChanged,
		},
		{
			name:       "code_tab_path",
			parentTree: map[string]string{"internal/a\tb.go": "package x\n"},
			headTree:   map[string]string{"internal/a\tb.go": "package x\n// changed\n"},
			wantOK:     false,
			wantReason: ReasonCodeChanged,
		},
		{
			name:       "code_newline_path",
			parentTree: map[string]string{"internal/a\nb.go": "package x\n"},
			headTree:   map[string]string{"internal/a\nb.go": "package x\n// changed\n"},
			wantOK:     false,
			wantReason: ReasonCodeChanged,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTestGitRepo(t)
			gitRunner, scriptRunner := repo.runners()

			parent := repo.commit("parent", tc.parentTree)
			head := repo.commit("head", tc.headTree, parent)

			reason, ok := classifyParentDiff(t.Context(), gitRunner, scriptRunner, 5*time.Second, parent, head, depFile, defaultScriptMaxOutput)
			if ok != tc.wantOK || reason != tc.wantReason {
				t.Errorf("classifyParentDiff = (%q, %v), want (%q, %v)", reason, ok, tc.wantReason, tc.wantOK)
			}
		})
	}
}

// recordingInputRunner wraps a real cmdrunner.InputRunner to record the raw error
// returned by RunInput so deadlock tests can distinguish exit 2 from a timeout.
type recordingInputRunner struct {
	inner  cmdrunner.InputRunner
	calls  int
	rawErr error
}

func (r *recordingInputRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunInput(ctx, nil, name, args...)
}

func (r *recordingInputRunner) RunInput(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	r.calls++
	out, err := r.inner.RunInput(ctx, stdin, name, args...)
	r.rawErr = err
	return out, err
}

// TestCheckExtensionChangesEarlyExitNoDeadlock verifies that passing more than 64 KiB
// on stdin to has-extension-changes.sh when it exits early (exit 2 on an invalid
// dependency directory line without reading stdin) returns promptly with exit code 2
// rather than timing out on a deadlocked pipe.
func TestCheckExtensionChangesEarlyExitNoDeadlock(t *testing.T) {
	_, scriptOpts, _ := runnerOptions(os.Environ(), defaultLimits())
	scriptOpts.Dir = filepath.Join("..", "..")
	rec := &recordingInputRunner{inner: cmdrunner.NewOS(scriptOpts)}

	// Invalid line (no trailing "/") forces has-extension-changes.sh to exit 2
	// before entering the stdin read loop.
	badDepFile := writeTempFile(t, "bad-deps", []byte("internal/llm\n"))

	// Build > 256 KiB of NUL-delimited input (well above the 64 KiB pipe buffer).
	largeStdin := bytes.Repeat([]byte("docs/entry.md\x00"), 20000)
	if len(largeStdin) <= 64<<10 {
		t.Fatalf("largeStdin len = %d, want > 65536", len(largeStdin))
	}

	reason, ok := checkExtensionChanges(t.Context(), rec, 5*time.Second, badDepFile, largeStdin)
	if ok || reason != ReasonExtensionCheckFailed {
		t.Errorf("got (%q, %v), want (%q, false)", reason, ok, ReasonExtensionCheckFailed)
	}
	if rec.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", rec.calls)
	}
	if errors.Is(rec.rawErr, context.DeadlineExceeded) {
		t.Fatalf("script timed out (possible stdin deadlock): %v", rec.rawErr)
	}
	exitErr, isExit := errors.AsType[*cmdrunner.ExitError](rec.rawErr)
	if !isExit || exitErr.Code != 2 {
		t.Fatalf("raw error = %v, want *cmdrunner.ExitError with exit code 2", rec.rawErr)
	}
}
