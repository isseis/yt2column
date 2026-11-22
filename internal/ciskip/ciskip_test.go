//go:build test

package ciskip

import (
	"slices"
	"strings"
	"testing"
	"time"
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
