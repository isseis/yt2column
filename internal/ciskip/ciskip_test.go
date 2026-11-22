//go:build test

package ciskip

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

func TestDecisionSkip(t *testing.T) {
	if !(Decision{Reason: ReasonSkipped}).Skip() {
		t.Error("Decision{Reason: ReasonSkipped}.Skip() = false, want true")
	}

	nonSkip := []Reason{
		ReasonUnspecified,
		ReasonInvalidInput,
		ReasonForkOrRepoMismatch,
		ReasonNonMainBase,
		ReasonManualRerun,
		ReasonWasmDepDirsFailed,
		ReasonMergeCommitMismatch,
		ReasonMainMovedAtCheckout,
		ReasonNotSingleParent,
		ReasonCodeChanged,
		ReasonExtensionChanged,
		ReasonGitFailed,
		ReasonExtensionCheckFailed,
		ReasonAPIFailed,
		ReasonAPITimeout,
		ReasonAPIOutputTooLarge,
		ReasonAPIInvalidResponse,
		ReasonAPIPaginationLimit,
		ReasonNoParentRun,
		ReasonAmbiguousPRLink,
		ReasonParentRunIncomplete,
		ReasonParentRunBadResult,
		ReasonParentJobsInvalid,
		ReasonParentBaseMissing,
		ReasonParentBaseDuplicate,
		ReasonMainAdvanced,
		Reason("unknown_future_reason"),
		Reason("OK"),
		Reason("ok "),
	}
	for _, r := range nonSkip {
		if (Decision{Reason: r}).Skip() {
			t.Errorf("Decision{Reason: %q}.Skip() = true, want false", r)
		}
	}
}

func TestValidateBaseContext(t *testing.T) {
	valid := Input{
		Repo:     "isseis/yt2column",
		HeadRepo: "isseis/yt2column",
		BaseRepo: "isseis/yt2column",
		BaseRef:  "main",
		HeadSHA:  testSHA1,
	}

	t.Run("valid_same_repo_main", func(t *testing.T) {
		reason, ok := validateBaseContext(valid)
		if !ok || reason != ReasonUnspecified {
			t.Errorf("validateBaseContext(valid) = (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}
	})

	t.Run("valid_repo_punctuation_and_exact_max_len_and_hex_digits", func(t *testing.T) {
		in := valid
		in.Repo = "a.b-c_1/" + strings.Repeat("x", maxRepoLen-len("a.b-c_1/"))
		in.HeadRepo = in.Repo
		in.BaseRepo = in.Repo
		in.HeadSHA = "0123456789abcdef0123456789abcdef01234567"
		if len(in.Repo) != maxRepoLen {
			t.Fatalf("len(in.Repo) = %d, want %d", len(in.Repo), maxRepoLen)
		}
		if reason, ok := validateBaseContext(in); !ok || reason != ReasonUnspecified {
			t.Errorf("validateBaseContext(boundary) = (%q, %v), want (%q, true)", reason, ok, ReasonUnspecified)
		}
	})

	setRepo := func(repo string) func(*Input) {
		return func(in *Input) {
			in.Repo, in.HeadRepo, in.BaseRepo = repo, repo, repo
		}
	}

	cases := []struct {
		name   string
		mutate func(*Input)
		want   Reason
	}{
		{name: "fork_head_repo_differs", mutate: func(in *Input) { in.HeadRepo = "attacker/yt2column" }, want: ReasonForkOrRepoMismatch},
		{name: "deleted_fork_empty_head_repo", mutate: func(in *Input) { in.HeadRepo = "" }, want: ReasonForkOrRepoMismatch},
		{name: "base_repo_differs_from_repo", mutate: func(in *Input) { in.BaseRepo = "other/yt2column" }, want: ReasonForkOrRepoMismatch},
		{name: "repo_empty", mutate: setRepo(""), want: ReasonForkOrRepoMismatch},
		{name: "repo_query_injection", mutate: setRepo("isseis/yt2column?x=1"), want: ReasonForkOrRepoMismatch},
		{name: "repo_fragment_percent_amp", mutate: setRepo("isseis/yt2column#a%20&b"), want: ReasonForkOrRepoMismatch},
		{name: "repo_dotdot_segment", mutate: setRepo("isseis/.."), want: ReasonForkOrRepoMismatch},
		{name: "repo_dot_segment", mutate: setRepo("./yt2column"), want: ReasonForkOrRepoMismatch},
		{name: "repo_extra_slash", mutate: setRepo("isseis/yt2column/extra"), want: ReasonForkOrRepoMismatch},
		{name: "repo_whitespace_not_trimmed", mutate: setRepo(" isseis/yt2column"), want: ReasonForkOrRepoMismatch},
		{name: "repo_no_slash", mutate: setRepo("noslash"), want: ReasonForkOrRepoMismatch},
		{name: "repo_empty_owner", mutate: setRepo("/yt2column"), want: ReasonForkOrRepoMismatch},
		{name: "repo_empty_name", mutate: setRepo("isseis/"), want: ReasonForkOrRepoMismatch},
		{name: "repo_exceeds_max_len", mutate: setRepo("a/" + strings.Repeat("b", maxRepoLen-1)), want: ReasonForkOrRepoMismatch},
		{name: "non_main_base_ref", mutate: func(in *Input) { in.BaseRef = "release" }, want: ReasonNonMainBase},
		{name: "base_ref_exact_max_len_non_main", mutate: func(in *Input) { in.BaseRef = strings.Repeat("m", maxBaseRefLen) }, want: ReasonNonMainBase},
		{name: "empty_base_ref", mutate: func(in *Input) { in.BaseRef = "" }, want: ReasonNonMainBase},
		{name: "base_ref_with_whitespace", mutate: func(in *Input) { in.BaseRef = "main " }, want: ReasonNonMainBase},
		{name: "base_ref_exceeds_max_len", mutate: func(in *Input) { in.BaseRef = strings.Repeat("m", maxBaseRefLen+1) }, want: ReasonInvalidInput},
		{name: "head_sha_uppercase_last_char", mutate: func(in *Input) { in.HeadSHA = testSHA1[:39] + "A" }, want: ReasonInvalidInput},
		{name: "head_sha_short", mutate: func(in *Input) { in.HeadSHA = testSHA1[:39] }, want: ReasonInvalidInput},
		{name: "head_sha_long", mutate: func(in *Input) { in.HeadSHA = testSHA1 + "0" }, want: ReasonInvalidInput},
		{name: "head_sha_non_hex_last_char", mutate: func(in *Input) { in.HeadSHA = testSHA1[:39] + "g" }, want: ReasonInvalidInput},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			tc.mutate(&in)
			got, ok := validateBaseContext(in)
			if ok {
				t.Fatalf("validateBaseContext() ok = true, want false (%q)", tc.want)
			}
			if got != tc.want {
				t.Errorf("validateBaseContext() reason = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateRemainingContext(t *testing.T) {
	valid := Input{
		PRNumber:      "1073",
		RunAttempt:    "1",
		WasmDepDirsOK: "true",
		DepDirsFile:   "/tmp/wasm-dep-dirs",
	}

	t.Run("valid", func(t *testing.T) {
		pr, reason, ok := validateRemainingContext(valid)
		if !ok || reason != ReasonUnspecified || pr != 1073 {
			t.Errorf("validateRemainingContext(valid) = (%d, %q, %v), want (1073, %q, true)", pr, reason, ok, ReasonUnspecified)
		}
	})

	t.Run("valid_max_10_digits_pr", func(t *testing.T) {
		in := valid
		in.PRNumber = "9999999999"
		pr, reason, ok := validateRemainingContext(in)
		if !ok || reason != ReasonUnspecified || pr != 9999999999 {
			t.Errorf("validateRemainingContext(max PR) = (%d, %q, %v), want (9999999999, %q, true)", pr, reason, ok, ReasonUnspecified)
		}
	})

	cases := []struct {
		name   string
		mutate func(*Input)
		want   Reason
	}{
		{name: "pr_empty", mutate: func(in *Input) { in.PRNumber = "" }, want: ReasonInvalidInput},
		{name: "pr_zero", mutate: func(in *Input) { in.PRNumber = "0" }, want: ReasonInvalidInput},
		{name: "pr_leading_zero", mutate: func(in *Input) { in.PRNumber = "01" }, want: ReasonInvalidInput},
		{name: "pr_negative", mutate: func(in *Input) { in.PRNumber = "-1" }, want: ReasonInvalidInput},
		{name: "pr_plus_sign", mutate: func(in *Input) { in.PRNumber = "+1" }, want: ReasonInvalidInput},
		{name: "pr_padded", mutate: func(in *Input) { in.PRNumber = " 10 " }, want: ReasonInvalidInput},
		{name: "pr_non_digit_suffix", mutate: func(in *Input) { in.PRNumber = "1a" }, want: ReasonInvalidInput},
		{name: "pr_too_long", mutate: func(in *Input) { in.PRNumber = "10000000000" }, want: ReasonInvalidInput},
		{name: "run_attempt_2_manual_rerun", mutate: func(in *Input) { in.RunAttempt = "2" }, want: ReasonManualRerun},
		{name: "run_attempt_max_10_digits_manual_rerun", mutate: func(in *Input) { in.RunAttempt = "9999999999" }, want: ReasonManualRerun},
		{name: "run_attempt_zero", mutate: func(in *Input) { in.RunAttempt = "0" }, want: ReasonInvalidInput},
		{name: "run_attempt_negative", mutate: func(in *Input) { in.RunAttempt = "-1" }, want: ReasonInvalidInput},
		{name: "run_attempt_plus_sign", mutate: func(in *Input) { in.RunAttempt = "+1" }, want: ReasonInvalidInput},
		{name: "run_attempt_leading_zero", mutate: func(in *Input) { in.RunAttempt = "01" }, want: ReasonInvalidInput},
		{name: "run_attempt_non_int", mutate: func(in *Input) { in.RunAttempt = "one" }, want: ReasonInvalidInput},
		{name: "run_attempt_trailing_space", mutate: func(in *Input) { in.RunAttempt = "1 " }, want: ReasonInvalidInput},
		{name: "wasm_dep_dirs_false", mutate: func(in *Input) { in.WasmDepDirsOK = "false" }, want: ReasonWasmDepDirsFailed},
		{name: "wasm_dep_dirs_empty", mutate: func(in *Input) { in.WasmDepDirsOK = "" }, want: ReasonInvalidInput},
		{name: "wasm_dep_dirs_uppercase", mutate: func(in *Input) { in.WasmDepDirsOK = "TRUE" }, want: ReasonInvalidInput},
		{name: "wasm_dep_dirs_padded", mutate: func(in *Input) { in.WasmDepDirsOK = "true " }, want: ReasonInvalidInput},
		{name: "dep_dirs_file_empty", mutate: func(in *Input) { in.DepDirsFile = "" }, want: ReasonWasmDepDirsFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			tc.mutate(&in)
			pr, got, ok := validateRemainingContext(in)
			if ok || pr != 0 {
				t.Fatalf("validateRemainingContext() = (%d, %q, %v), want (0, %q, false)", pr, got, ok, tc.want)
			}
			if got != tc.want {
				t.Errorf("validateRemainingContext() reason = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunnerOptions(t *testing.T) {
	environ := []string{
		"HOME=/home/ci",
		"PATH=/usr/bin",
		"GH_TOKEN=secret-gh-token",
		"GITHUB_TOKEN=secret-github-token",
		"GIT_DIR=/elsewhere",
		"GIT_CONFIG_PARAMETERS=bad",
		"CISKIP_REPO=isseis/yt2column",
		"CISKIP_NORMAL_CODE=true",
		"CLICOLOR_FORCE=1",
		"GH_FORCE_TTY=1",
		"NO_COLOR=0",
		"GH_PROMPT_DISABLED=0",
		"GH_REPO=evil/repo",
		"GH_HOST=evil.example",
		"GH_DEBUG=api",
		"GH_PAGER=cat",
		"PAGER=less",
	}

	// Use distinct non-default limits so swapping any two fields fails the test.
	limits := evalLimits{
		gitMaxOutput:    1111,
		scriptMaxOutput: 2222,
		ghMaxOutput:     3333,
		waitDelay:       7 * time.Second,
		groupStopGrace:  13 * time.Second,
	}
	gitOpts, scriptOpts, ghOpts := runnerOptions(environ, limits)

	if gitOpts.MaxOutput != limits.gitMaxOutput || !gitOpts.ProcessGroup || gitOpts.StderrIsFailure ||
		gitOpts.WaitDelay != limits.waitDelay || gitOpts.GroupStopGrace != limits.groupStopGrace {
		t.Errorf("unexpected gitOpts: %+v", gitOpts)
	}
	if scriptOpts.MaxOutput != limits.scriptMaxOutput || scriptOpts.ProcessGroup || !scriptOpts.StderrIsFailure ||
		scriptOpts.WaitDelay != limits.waitDelay {
		t.Errorf("unexpected scriptOpts: %+v", scriptOpts)
	}
	if ghOpts.MaxOutput != limits.ghMaxOutput || !ghOpts.ProcessGroup || ghOpts.StderrIsFailure ||
		ghOpts.WaitDelay != limits.waitDelay || ghOpts.GroupStopGrace != limits.groupStopGrace {
		t.Errorf("unexpected ghOpts: %+v", ghOpts)
	}

	for _, check := range []struct {
		name string
		env  []string
	}{
		{"git", gitOpts.Env},
		{"script", scriptOpts.Env},
	} {
		for _, forbidden := range []string{
			"GH_TOKEN=secret-gh-token",
			"GITHUB_TOKEN=secret-github-token",
			"GIT_DIR=/elsewhere",
			"GIT_CONFIG_PARAMETERS=bad",
			"CISKIP_REPO=isseis/yt2column",
			"CISKIP_NORMAL_CODE=true",
		} {
			if slices.Contains(check.env, forbidden) {
				t.Errorf("%s env still contains %q: %v", check.name, forbidden, check.env)
			}
		}
		for _, required := range []string{
			"HOME=/home/ci",
			"PATH=/usr/bin",
			"GIT_OPTIONAL_LOCKS=0",
			"GIT_TERMINAL_PROMPT=0",
		} {
			if !slices.Contains(check.env, required) {
				t.Errorf("%s env missing %q: %v", check.name, required, check.env)
			}
		}
	}

	wantGHEnv := []string{
		"HOME=/home/ci",
		"PATH=/usr/bin",
		"GH_TOKEN=secret-gh-token",
		"GITHUB_TOKEN=secret-github-token",
		"NO_COLOR=1",
		"GH_PROMPT_DISABLED=1",
	}
	if !slices.Equal(ghOpts.Env, wantGHEnv) {
		t.Errorf("gh env = %q, want %q", ghOpts.Env, wantGHEnv)
	}
}

func TestEvaluateSkipEligible(t *testing.T) {
	depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))

	t.Run("doc_only_diff_skips", func(t *testing.T) {
		diffOut := "docs/tasks/0119_ci_skip_doc_only_commit/03_implementation_plan.md\x00.claude/commands/runplan.md\x00LICENSE\x00"
		gitFake := cmdrunnertestutil.New(t, localGitPassSteps(diffOut)...)
		scriptFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "bash",
			Args: []string{hasExtensionChangesScript, depFile},
			Out:  "false\n",
		})
		ghFake := cmdrunnertestutil.New(t, ghHappySteps(testRunID)...)

		ev := newWithRunners(gitFake, scriptFake, ghFake, defaultLimits())
		dec := ev.Evaluate(t.Context(), validEvalInput(depFile))

		if !dec.Skip() || dec.Reason != ReasonSkipped {
			t.Fatalf("decision = %+v, want Skip()=true Reason=%q", dec, ReasonSkipped)
		}
		if dec.NoticeBaseSHA != testSHA1 || dec.ParentSHA != testSHA4 || dec.ParentRunID != testRunID || dec.HTTPStatus != 0 {
			t.Errorf("decision fields = %+v, want NoticeBaseSHA=%q ParentSHA=%q ParentRunID=%d HTTPStatus=0",
				dec, testSHA1, testSHA4, testRunID)
		}
	})

	t.Run("empty_commit_skips_without_calling_extension_script", func(t *testing.T) {
		gitFake := cmdrunnertestutil.New(t, localGitPassSteps("")...)
		// Zero scripted steps: fails if has-extension-changes.sh is invoked on an empty diff.
		scriptFake := cmdrunnertestutil.New(t)
		ghFake := cmdrunnertestutil.New(t, ghHappySteps(testRunID)...)

		ev := newWithRunners(gitFake, scriptFake, ghFake, defaultLimits())
		dec := ev.Evaluate(t.Context(), validEvalInput(depFile))

		if !dec.Skip() || dec.Reason != ReasonSkipped {
			t.Fatalf("decision = %+v, want Skip()=true Reason=%q", dec, ReasonSkipped)
		}
		if dec.NoticeBaseSHA != testSHA1 || dec.ParentSHA != testSHA4 || dec.ParentRunID != testRunID {
			t.Errorf("decision fields = %+v, want NoticeBaseSHA=%q ParentSHA=%q ParentRunID=%d",
				dec, testSHA1, testSHA4, testRunID)
		}
	})

	t.Run("empty_commit_still_validates_dep_dirs_file_before_diff", func(t *testing.T) {
		badDepFile := writeTempFile(t, "empty-deps", nil)
		// Only synthetic merge, origin/main, and single-parent run; diff/script/gh must not run.
		gitFake := cmdrunnertestutil.New(t, localGitPassSteps("")[:3]...)
		scriptFake := cmdrunnertestutil.New(t)
		ghFake := cmdrunnertestutil.New(t)

		ev := newWithRunners(gitFake, scriptFake, ghFake, defaultLimits())
		dec := ev.Evaluate(t.Context(), validEvalInput(badDepFile))

		if dec.Skip() || dec.Reason != ReasonWasmDepDirsFailed {
			t.Fatalf("decision = %+v, want Reason=%q", dec, ReasonWasmDepDirsFailed)
		}
		if dec.NoticeBaseSHA != testSHA1 {
			t.Errorf("NoticeBaseSHA = %q, want %q", dec.NoticeBaseSHA, testSHA1)
		}
	})
}

func TestEvaluateChangedFilesReject(t *testing.T) {
	depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))

	t.Run("code_change_stops_before_script_and_gh", func(t *testing.T) {
		gitFake := cmdrunnertestutil.New(t, localGitPassSteps(".github/workflows/ci.yml\x00")...)
		// Neither script nor gh may be called when a code change is detected.
		scriptFake := cmdrunnertestutil.New(t)
		ghFake := cmdrunnertestutil.New(t)

		ev := newWithRunners(gitFake, scriptFake, ghFake, defaultLimits())
		dec := ev.Evaluate(t.Context(), validEvalInput(depFile))

		if dec.Skip() || dec.Reason != ReasonCodeChanged {
			t.Fatalf("decision = %+v, want Reason=%q", dec, ReasonCodeChanged)
		}
		if dec.NoticeBaseSHA != testSHA1 || dec.ParentSHA != testSHA4 {
			t.Errorf("decision = %+v, want NoticeBaseSHA=%q ParentSHA=%q", dec, testSHA1, testSHA4)
		}
	})

	t.Run("extension_change_stops_before_gh", func(t *testing.T) {
		gitFake := cmdrunnertestutil.New(t, localGitPassSteps("README.md\x00")...)
		scriptFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "bash",
			Args: []string{hasExtensionChangesScript, depFile},
			Out:  "true\n",
		})
		ghFake := cmdrunnertestutil.New(t)

		ev := newWithRunners(gitFake, scriptFake, ghFake, defaultLimits())
		dec := ev.Evaluate(t.Context(), validEvalInput(depFile))

		if dec.Skip() || dec.Reason != ReasonExtensionChanged {
			t.Fatalf("decision = %+v, want Reason=%q", dec, ReasonExtensionChanged)
		}
		if dec.NoticeBaseSHA != testSHA1 || dec.ParentSHA != testSHA4 {
			t.Errorf("decision = %+v, want NoticeBaseSHA=%q ParentSHA=%q", dec, testSHA1, testSHA4)
		}
	})
}

func TestEvaluateContextValidation(t *testing.T) {
	depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))

	// Base-context failure must stop before git/script/gh and leave NoticeBaseSHA empty.
	// Detailed base-context rules live in TestValidateBaseContext.
	t.Run("base_context_reject_leaves_notice_empty", func(t *testing.T) {
		in := validEvalInput(depFile)
		in.HeadRepo = "other/yt2column"
		ev := newWithRunners(cmdrunnertestutil.New(t), cmdrunnertestutil.New(t), cmdrunnertestutil.New(t), defaultLimits())
		dec := ev.Evaluate(t.Context(), in)

		if dec.Skip() || dec.Reason != ReasonForkOrRepoMismatch {
			t.Fatalf("decision = %+v, want Reason=%q", dec, ReasonForkOrRepoMismatch)
		}
		if dec.NoticeBaseSHA != "" {
			t.Errorf("NoticeBaseSHA = %q, want empty", dec.NoticeBaseSHA)
		}
	})

	// Remaining-context failure runs after HEAD^2 == HeadSHA verification, so it must
	// preserve NoticeBaseSHA and stop before origin/main, script, or gh.
	// Detailed remaining-context rules live in TestValidateRemainingContext.
	postMergeRejects := []struct {
		name   string
		mutate func(*Input)
		want   Reason
	}{
		{name: "manual_rerun", mutate: func(in *Input) { in.RunAttempt = "2" }, want: ReasonManualRerun},
		{name: "wasm_dep_dirs_ok_false", mutate: func(in *Input) { in.WasmDepDirsOK = "false" }, want: ReasonWasmDepDirsFailed},
	}

	for _, tc := range postMergeRejects {
		t.Run("remaining_"+tc.name, func(t *testing.T) {
			in := validEvalInput(depFile)
			tc.mutate(&in)
			gitFake := cmdrunnertestutil.New(t, localGitPassSteps("")[0])
			ev := newWithRunners(gitFake, cmdrunnertestutil.New(t), cmdrunnertestutil.New(t), defaultLimits())
			dec := ev.Evaluate(t.Context(), in)

			if dec.Skip() || dec.Reason != tc.want {
				t.Fatalf("decision = %+v, want Reason=%q", dec, tc.want)
			}
			if dec.NoticeBaseSHA != testSHA1 {
				t.Errorf("NoticeBaseSHA = %q, want %q", dec.NoticeBaseSHA, testSHA1)
			}
		})
	}
}

func TestEvaluateBaseConsistency(t *testing.T) {
	depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))
	passGit := localGitPassSteps("")

	cases := []struct {
		name       string
		gitSteps   []cmdrunnertestutil.Step
		wantReason Reason
		wantNotice string
	}{
		{
			name: "merge_commit_mismatch_leaves_notice_empty",
			gitSteps: []cmdrunnertestutil.Step{{
				Name: testGitCmd,
				Args: gitArgs("rev-list", "--parents", "-n", "1", "HEAD"),
				Out:  testSHA3 + " " + testSHA1 + " " + testSHA4 + "\n",
			}},
			wantReason: ReasonMergeCommitMismatch,
			wantNotice: "",
		},
		{
			name: "origin_main_moved_preserves_notice_base_sha",
			gitSteps: []cmdrunnertestutil.Step{
				passGit[0],
				{
					Name: testGitCmd,
					Args: gitArgs("rev-parse", "--verify", originMainCommitRef),
					Out:  testSHA4 + "\n",
				},
			},
			wantReason: ReasonMainMovedAtCheckout,
			wantNotice: testSHA1,
		},
		{
			name: "merge_commit_pr_head_preserves_notice_base_sha",
			gitSteps: []cmdrunnertestutil.Step{
				passGit[0],
				passGit[1],
				{
					Name: testGitCmd,
					Args: gitArgs("rev-list", "--parents", "-n", "1", testSHA2),
					Out:  testSHA2 + " " + testSHA4 + " " + testSHA1 + "\n",
				},
			},
			wantReason: ReasonNotSingleParent,
			wantNotice: testSHA1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := newWithRunners(
				cmdrunnertestutil.New(t, tc.gitSteps...),
				cmdrunnertestutil.New(t),
				cmdrunnertestutil.New(t),
				defaultLimits(),
			)
			dec := ev.Evaluate(t.Context(), validEvalInput(depFile))

			if dec.Skip() || dec.Reason != tc.wantReason {
				t.Fatalf("decision = %+v, want Reason=%q", dec, tc.wantReason)
			}
			if dec.NoticeBaseSHA != tc.wantNotice {
				t.Errorf("NoticeBaseSHA = %q, want %q", dec.NoticeBaseSHA, tc.wantNotice)
			}
		})
	}
}

func TestEvaluateFailSafe(t *testing.T) {
	uninitializedCases := []struct {
		name string
		ev   func(t *testing.T) *Evaluator
	}{
		{
			name: "nil_evaluator",
			ev:   func(*testing.T) *Evaluator { return nil },
		},
		{
			name: "zero_value_evaluator",
			ev:   func(*testing.T) *Evaluator { return &Evaluator{} },
		},
		{
			name: "nil_git_runner",
			ev: func(t *testing.T) *Evaluator {
				return newWithRunners(nil, cmdrunnertestutil.New(t), cmdrunnertestutil.New(t), defaultLimits())
			},
		},
		{
			name: "nil_script_runner",
			ev: func(t *testing.T) *Evaluator {
				return newWithRunners(cmdrunnertestutil.New(t), nil, cmdrunnertestutil.New(t), defaultLimits())
			},
		},
		{
			name: "nil_gh_runner",
			ev: func(t *testing.T) *Evaluator {
				return newWithRunners(cmdrunnertestutil.New(t), cmdrunnertestutil.New(t), nil, defaultLimits())
			},
		},
	}

	for _, tc := range uninitializedCases {
		t.Run(tc.name, func(t *testing.T) {
			dec := tc.ev(t).Evaluate(t.Context(), validEvalInput("/tmp/unused"))
			if dec.Skip() || dec.Reason != ReasonUnspecified || dec.NoticeBaseSHA != "" {
				t.Errorf("Evaluate = %+v, want zero Decision", dec)
			}
		})
	}

	t.Run("new_builds_production_evaluator", func(t *testing.T) {
		ev := New(os.Environ())
		if ev == nil {
			t.Fatal("New(os.Environ()) returned nil")
		}
		if ev.limits != defaultLimits() {
			t.Errorf("New limits = %+v, want %+v", ev.limits, defaultLimits())
		}
		// Base context check runs without invoking external commands and fails if any runner is nil.
		dec := ev.Evaluate(t.Context(), Input{Repo: testRepo, BaseRepo: testRepo, HeadRepo: "fork/yt2column", BaseRef: "main", HeadSHA: testSHA2})
		if dec.Skip() || dec.Reason != ReasonForkOrRepoMismatch {
			t.Errorf("production evaluator fork decision = %+v, want Reason=%q", dec, ReasonForkOrRepoMismatch)
		}
	})
}
