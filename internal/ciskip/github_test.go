//go:build test

package ciskip

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// deadlineBlockingRunner implements cmdrunner.Runner by asserting the call context
// carries a deadline within maxRemaining and waiting until that context expires.
type deadlineBlockingRunner struct {
	t            *testing.T
	maxRemaining time.Duration
	called       bool
}

func (r *deadlineBlockingRunner) Run(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	r.called = true
	dl, ok := ctx.Deadline()
	if !ok {
		r.t.Errorf("gh runner context has no deadline")
		return nil, errors.New("missing deadline")
	}
	if rem := time.Until(dl); rem <= 0 || rem > r.maxRemaining {
		r.t.Errorf("gh runner deadline remaining = %v, want in (0, %v]", rem, r.maxRemaining)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func evalWithDepFile(t *testing.T, depFile string, limits evalLimits, ghSteps ...cmdrunnertestutil.Step) Decision {
	t.Helper()
	gitFake := cmdrunnertestutil.New(t, localGitPassSteps("docs/note.md\x00")...)
	scriptFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
		Name: "bash",
		Args: []string{hasExtensionChangesScript, depFile},
		Out:  "false\n",
	})
	ghFake := cmdrunnertestutil.New(t, ghSteps...)
	ev := newWithRunners(gitFake, scriptFake, ghFake, limits)
	return ev.Evaluate(t.Context(), validEvalInput(depFile))
}

func evalWithGHSteps(t *testing.T, limits evalLimits, ghSteps ...cmdrunnertestutil.Step) Decision {
	t.Helper()
	depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))
	return evalWithDepFile(t, depFile, limits, ghSteps...)
}

type testJobSpec struct {
	id         int64
	name       string
	headSHA    string
	status     string
	conclusion string // JSON literal, e.g. `"success"` or `null`
}

func jobsPayload(specs ...testJobSpec) string {
	parts := make([]string, 0, len(specs))
	for _, s := range specs {
		sha := s.headSHA
		if sha == "" {
			sha = testSHA4
		}
		status := s.status
		if status == "" {
			status = "completed"
		}
		concl := s.conclusion
		if concl == "" {
			concl = `"success"`
		}
		parts = append(parts, fmt.Sprintf(
			`{"id": %d, "name": %q, "head_sha": %q, "status": %q, "conclusion": %s}`,
			s.id, s.name, sha, status, concl,
		))
	}
	return `{"jobs": [` + strings.Join(parts, ",") + `]}`
}

func runJSON(id int64, path, event, headSHA, status, conclusionJSON, createdAt, startedAt, repo, headRepoJSON, prsJSON string) string {
	return fmt.Sprintf(`{
		"id": %d,
		"path": %q,
		"event": %q,
		"head_sha": %q,
		"status": %q,
		"conclusion": %s,
		"created_at": %q,
		"run_started_at": %q,
		"repository": {"full_name": %q},
		"head_repository": %s,
		"pull_requests": %s,
		"extra_unconsumed_field": {"nested": [1, 2, 3]}
	}`, id, path, event, headSHA, status, conclusionJSON, createdAt, startedAt, repo, headRepoJSON, prsJSON)
}

func standardRunJSON(id int64, status, conclusionJSON, createdAt, startedAt string) string {
	return runJSON(
		id,
		ciWorkflowPath,
		pullRequestEvent,
		testSHA4,
		status,
		conclusionJSON,
		createdAt,
		startedAt,
		testRepo,
		fmt.Sprintf(`{"full_name": %q}`, testRepo),
		`[{"number": 1073}]`,
	)
}

func TestVerifyParentCIValid(t *testing.T) {
	t.Run("doc_guard_and_secret_scan_failure_still_passes", func(t *testing.T) {
		runsBody := fmt.Sprintf(`{"workflow_runs": [%s]}`,
			standardRunJSON(testRunID, "completed", `"failure"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:05Z"))
		jobsBody := fmt.Sprintf(`{"jobs": [
			{"id": %d, "name": "Check changed files", "head_sha": %q, "status": "completed", "conclusion": "success"},
			{"id": 9002, "name": "Test", "head_sha": %q, "status": "completed", "conclusion": "success"},
			{"id": 9003, "name": "Code Quality & Lint", "head_sha": %q, "status": "completed", "conclusion": "success"},
			{"id": 9004, "name": "Browser extension", "head_sha": %q, "status": "completed", "conclusion": "skipped"},
			{"id": 9005, "name": "Documentation guard", "head_sha": %q, "status": "completed", "conclusion": "failure"},
			{"id": 9006, "name": "Secret scan", "head_sha": %q, "status": "completed", "conclusion": "failure"}
		]}`, testCheckJobID, testSHA4, testSHA4, testSHA4, testSHA4, testSHA4, testSHA4)
		annBody := fmt.Sprintf(`[
			{"annotation_level": "warning", "title": null, "message": "unrelated"},
			{"annotation_level": "notice", "title": "ci-base-sha", "message": %q}
		]`, testSHA1)

		dec := evalWithGHSteps(t, defaultLimits(),
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(runsEndpoint(1)),
				Out:  ghHTTPResponse("HTTP/1.1 200 OK", nil, runsBody),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(jobsEndpoint(testRunID, 1)),
				Out:  ghHTTPResponse("HTTP/2 200", nil, jobsBody),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(annotationsEndpoint(1)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, annBody),
			},
		)
		if !dec.Skip() || dec.Reason != ReasonSkipped || dec.ParentRunID != testRunID || dec.NoticeBaseSHA != testSHA1 {
			t.Errorf("decision = %+v, want Skip()=true Reason=%q ParentRunID=%d NoticeBaseSHA=%q",
				dec, ReasonSkipped, testRunID, testSHA1)
		}
	})

	t.Run("multi_page_runs_jobs_and_annotations_with_link_variants", func(t *testing.T) {
		// Page 1 of runs has a newer cancelled run for a different single PR (must be ignored);
		// page 2 has the matching run. Uses lowercase "link", unquoted rel=next, and multi-value rel.
		runsP1 := fmt.Sprintf(`{"workflow_runs": [%s]}`, runJSON(
			9999, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"cancelled"`,
			"2026-11-21T11:00:00Z", "2026-11-21T11:00:01Z", testRepo,
			fmt.Sprintf(`{"full_name": %q}`, testRepo), `[{"number": 9999}]`,
		))
		runsP2 := fmt.Sprintf(`{"workflow_runs": [%s]}`,
			standardRunJSON(testRunID, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:05Z"))

		jobsP1 := jobsPayload(
			testJobSpec{id: testCheckJobID, name: checkChangesJobName},
			testJobSpec{id: 9002, name: testJobName},
		)
		jobsP2 := jobsPayload(
			testJobSpec{id: 9003, name: lintJobName},
			testJobSpec{id: 9004, name: extensionJobName},
		)

		annP1 := `[{"annotation_level": "notice", "title": "other", "message": "ignored"}]`
		annP2 := fmt.Sprintf(`[{"annotation_level": "notice", "title": "ci-base-sha", "message": %q}]`, testSHA1)

		dec := evalWithGHSteps(t, defaultLimits(),
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(runsEndpoint(1)),
				Out: ghHTTPResponse("HTTP/2.0 200 OK", []string{
					`link: <https://api.github.com/ignored?page=2>; rel=next, <https://api.github.com/ignored?page=2>; rel="last"`,
				}, runsP1),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(runsEndpoint(2)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsP2),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(jobsEndpoint(testRunID, 1)),
				Out: ghHTTPResponse("HTTP/2.0 200 OK", []string{
					`LINK: <https://api.github.com/ignored?page=2>; REL="prev NEXT"`,
				}, jobsP1),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(jobsEndpoint(testRunID, 2)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, jobsP2),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(annotationsEndpoint(1)),
				Out: ghHTTPResponse("HTTP/2.0 200 OK", []string{
					`Link: <https://api.github.com/ignored?page=2>; rel="next"`,
				}, annP1),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(annotationsEndpoint(2)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, annP2),
			},
		)
		if !dec.Skip() || dec.Reason != ReasonSkipped || dec.ParentRunID != testRunID {
			t.Errorf("multi-page decision = %+v, want Skip()=true Reason=%q ParentRunID=%d", dec, ReasonSkipped, testRunID)
		}
	})
}

func TestVerifyParentCISkipChain(t *testing.T) {
	steps := ghHappySteps(testRunID)
	steps[1].Out = ghHTTPResponse("HTTP/2.0 200 OK", nil, jobsPayload(
		testJobSpec{id: testCheckJobID, name: checkChangesJobName, conclusion: `"success"`},
		testJobSpec{id: 9002, name: testJobName, conclusion: `"skipped"`},
		testJobSpec{id: 9003, name: lintJobName, conclusion: `"skipped"`},
		testJobSpec{id: 9004, name: extensionJobName, conclusion: `"skipped"`},
	))

	dec := evalWithGHSteps(t, defaultLimits(), steps...)
	if !dec.Skip() || dec.Reason != ReasonSkipped || dec.NoticeBaseSHA != testSHA1 || dec.ParentRunID != testRunID {
		t.Errorf("skip chain decision = %+v, want Skip()=true Reason=%q NoticeBaseSHA=%q ParentRunID=%d",
			dec, ReasonSkipped, testSHA1, testRunID)
	}
}

func TestVerifyParentCIRunSelection(t *testing.T) {
	t.Run("no_matching_runs_and_non_matching_filtered_out", func(t *testing.T) {
		sameHeadRepo := fmt.Sprintf(`{"full_name": %q}`, testRepo)
		runsBody := fmt.Sprintf(`{"workflow_runs": [%s, %s, %s, %s, %s, %s, %s, %s]}`,
			// Different workflow
			runJSON(101, ".github/workflows/deepseek-review.yml", pullRequestEvent, testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, sameHeadRepo, `[{"number": 1073}]`),
			// Different event
			runJSON(102, ciWorkflowPath, "push", testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, sameHeadRepo, `[{"number": 1073}]`),
			// Different head_sha
			runJSON(103, ciWorkflowPath, pullRequestEvent, testSHA1, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, sameHeadRepo, `[{"number": 1073}]`),
			// Different repository
			runJSON(104, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", "other/repo", sameHeadRepo, `[{"number": 1073}]`),
			// Different head_repository
			runJSON(105, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, `{"full_name": "fork/yt2column"}`, `[{"number": 1073}]`),
			// Deleted fork (head_repository: null) with empty pull_requests must be ignored, not ambiguous
			runJSON(106, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, `null`, `[]`),
			// Same repo, single different PR (isolates case 1 PR-number check)
			runJSON(107, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, sameHeadRepo, `[{"number": 999}]`),
			// Same repo, multiple different PRs without our PR
			runJSON(108, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, sameHeadRepo, `[{"number": 900}, {"number": 901}]`),
		)

		dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
			Name: "gh",
			Args: ghAPIArgs(runsEndpoint(1)),
			Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
		})
		if dec.Skip() || dec.Reason != ReasonNoParentRun || dec.NoticeBaseSHA != testSHA1 {
			t.Errorf("decision = %+v, want Reason=%q NoticeBaseSHA=%q", dec, ReasonNoParentRun, testSHA1)
		}
	})

	t.Run("head_repository_non_object_is_invalid_response", func(t *testing.T) {
		runsBody := fmt.Sprintf(`{"workflow_runs": [%s]}`,
			runJSON(108, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:01Z", testRepo, `"bad-string"`, `[{"number": 1073}]`))
		dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
			Name: "gh",
			Args: ghAPIArgs(runsEndpoint(1)),
			Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
		})
		if dec.Skip() || dec.Reason != ReasonAPIInvalidResponse {
			t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAPIInvalidResponse)
		}
	})

	ambiguousCases := []struct {
		name string
		prs  string
	}{
		{name: "empty_pull_requests", prs: `[]`},
		{name: "multiple_pull_requests_including_self", prs: `[{"number": 999}, {"number": 1073}]`},
		{name: "duplicate_self_pull_request", prs: `[{"number": 1073}, {"number": 1073}]`},
	}
	for _, tc := range ambiguousCases {
		t.Run("ambiguous_"+tc.name, func(t *testing.T) {
			runsBody := fmt.Sprintf(`{"workflow_runs": [%s, %s]}`,
				standardRunJSON(5001, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:05Z"),
				runJSON(5002, ciWorkflowPath, pullRequestEvent, testSHA4, "completed", `"success"`,
					"2026-11-21T10:01:00Z", "2026-11-21T10:01:05Z", testRepo,
					fmt.Sprintf(`{"full_name": %q}`, testRepo), tc.prs),
			)
			dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(runsEndpoint(1)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
			})
			if dec.Skip() || dec.Reason != ReasonAmbiguousPRLink {
				t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAmbiguousPRLink)
			}
		})
	}

	t.Run("tiebreak_selects_latest_by_run_started_at_created_at_id", func(t *testing.T) {
		tieCases := []struct {
			name      string
			runs      []string
			wantRunID int64
			wantSkip  bool
			wantRsn   Reason
		}{
			{
				name: "newer_run_started_at_wins_even_with_smaller_id",
				runs: []string{
					standardRunJSON(9000, "completed", `"cancelled"`, "2026-11-21T10:05:00Z", "2026-11-21T10:05:00Z"),
					standardRunJSON(1000, "completed", `"success"`, "2026-11-21T09:00:00Z", "2026-11-21T10:10:00Z"),
				},
				wantRunID: 1000,
				wantSkip:  true,
				wantRsn:   ReasonSkipped,
			},
			{
				// Older created_at has larger id so only created_at selects run 1000 (cancelled),
				// also confirming an older success run is ignored when the latest run is cancelled.
				name: "same_run_started_at_newer_created_at_wins_over_larger_id",
				runs: []string{
					standardRunJSON(2000, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:10:00Z"),
					standardRunJSON(1000, "completed", `"cancelled"`, "2026-11-21T10:01:00Z", "2026-11-21T10:10:00Z"),
				},
				wantRunID: 1000,
				wantSkip:  false,
				wantRsn:   ReasonParentRunBadResult,
			},
			{
				name: "same_timestamps_higher_id_wins",
				runs: []string{
					standardRunJSON(1000, "completed", `"cancelled"`, "2026-11-21T10:00:00Z", "2026-11-21T10:10:00Z"),
					standardRunJSON(2000, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:10:00Z"),
				},
				wantRunID: 2000,
				wantSkip:  true,
				wantRsn:   ReasonSkipped,
			},
		}

		for _, tc := range tieCases {
			t.Run(tc.name, func(t *testing.T) {
				runsBody := fmt.Sprintf(`{"workflow_runs": [%s]}`, strings.Join(tc.runs, ","))
				steps := []cmdrunnertestutil.Step{{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
				}}
				if tc.wantSkip {
					steps = append(steps, ghHappySteps(tc.wantRunID)[1:]...)
				}
				dec := evalWithGHSteps(t, defaultLimits(), steps...)
				if dec.Skip() != tc.wantSkip || dec.Reason != tc.wantRsn || dec.ParentRunID != tc.wantRunID {
					t.Errorf("decision = %+v, want Skip=%v Reason=%q ParentRunID=%d",
						dec, tc.wantSkip, tc.wantRsn, tc.wantRunID)
				}
			})
		}
	})
}

func TestVerifyParentCIInvalidStates(t *testing.T) {
	t.Run("incomplete_run_even_when_newer_run_completed", func(t *testing.T) {
		for _, status := range []string{"in_progress", "queued"} {
			t.Run(status, func(t *testing.T) {
				runsBody := fmt.Sprintf(`{"workflow_runs": [%s, %s]}`,
					standardRunJSON(1000, status, `null`, "2026-11-21T09:00:00Z", "2026-11-21T09:00:05Z"),
					standardRunJSON(2000, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:05Z"),
				)
				dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
				})
				if dec.Skip() || dec.Reason != ReasonParentRunIncomplete {
					t.Errorf("decision = %+v, want Reason=%q", dec, ReasonParentRunIncomplete)
				}
			})
		}
	})

	t.Run("disallowed_run_conclusions", func(t *testing.T) {
		for _, concl := range []string{`"cancelled"`, `"timed_out"`, `"startup_failure"`, `"stale"`, `"action_required"`, `"neutral"`, `"skipped"`, `null`} {
			t.Run(strings.Trim(concl, `"`), func(t *testing.T) {
				runsBody := fmt.Sprintf(`{"workflow_runs": [%s]}`,
					standardRunJSON(testRunID, "completed", concl, "2026-11-21T10:00:00Z", "2026-11-21T10:00:05Z"))
				dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
				})
				if dec.Skip() || dec.Reason != ReasonParentRunBadResult || dec.ParentRunID != testRunID {
					t.Errorf("decision = %+v, want Reason=%q ParentRunID=%d", dec, ReasonParentRunBadResult, testRunID)
				}
			})
		}
	})

	t.Run("invalid_rfc3339_timestamps", func(t *testing.T) {
		badTimestamps := []struct {
			name      string
			createdAt string
			startedAt string
		}{
			{name: "bad_created_at", createdAt: "not-a-date", startedAt: "2026-11-21T10:00:05Z"},
			{name: "bad_run_started_at", createdAt: "2026-11-21T10:00:00Z", startedAt: "2026-11-21 10:00:05"},
			{name: "non_utc_offset", createdAt: "2026-11-21T19:00:00+09:00", startedAt: "2026-11-21T10:00:05Z"},
			{name: "fractional_seconds", createdAt: "2026-11-21T10:00:00.123Z", startedAt: "2026-11-21T10:00:05Z"},
		}
		for _, tc := range badTimestamps {
			t.Run(tc.name, func(t *testing.T) {
				runsBody := fmt.Sprintf(`{"workflow_runs": [%s]}`,
					standardRunJSON(testRunID, "completed", `"success"`, tc.createdAt, tc.startedAt))
				dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, runsBody),
				})
				if dec.Skip() || dec.Reason != ReasonAPIInvalidResponse {
					t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAPIInvalidResponse)
				}
			})
		}
	})

	t.Run("invalid_jobs_states", func(t *testing.T) {
		okRunStep := ghHappySteps(testRunID)[0]

		jobCases := []struct {
			name     string
			jobsJSON string
		}{
			{
				name: "missing_check_changes",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName},
				),
			},
			{
				name: "missing_extension",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName},
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName},
				),
			},
			{
				name: "duplicate_required_job",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName},
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9012, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName},
				),
			},
			{
				name: "job_head_sha_mismatch",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName},
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName},
					testJobSpec{id: 9005, name: "Documentation guard", headSHA: testSHA1},
				),
			},
			{
				name: "in_progress_with_conclusion_success",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName},
					testJobSpec{id: 9002, name: testJobName, status: "in_progress", conclusion: `"success"`},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName},
				),
			},
			{
				name: "check_changes_skipped_not_allowed",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName, conclusion: `"skipped"`},
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName},
				),
			},
			{
				name: "check_changes_failed",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName, conclusion: `"failure"`},
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName},
				),
			},
			{
				name: "heavy_job_failure",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName},
					testJobSpec{id: 9002, name: testJobName, conclusion: `"failure"`},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName},
				),
			},
			{
				name: "heavy_job_cancelled",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName},
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName, conclusion: `"cancelled"`},
					testJobSpec{id: 9004, name: extensionJobName},
				),
			},
			{
				name: "heavy_job_null_conclusion",
				jobsJSON: jobsPayload(
					testJobSpec{id: 9001, name: checkChangesJobName},
					testJobSpec{id: 9002, name: testJobName},
					testJobSpec{id: 9003, name: lintJobName},
					testJobSpec{id: 9004, name: extensionJobName, conclusion: `null`},
				),
			},
		}

		for _, tc := range jobCases {
			t.Run(tc.name, func(t *testing.T) {
				dec := evalWithGHSteps(t, defaultLimits(),
					okRunStep,
					cmdrunnertestutil.Step{
						Name: "gh",
						Args: ghAPIArgs(jobsEndpoint(testRunID, 1)),
						Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, tc.jobsJSON),
					},
				)
				if dec.Skip() || dec.Reason != ReasonParentJobsInvalid || dec.ParentRunID != testRunID {
					t.Errorf("decision = %+v, want Reason=%q ParentRunID=%d", dec, ReasonParentJobsInvalid, testRunID)
				}
			})
		}
	})
}

func TestVerifyParentBaseAnnotation(t *testing.T) {
	prefixSteps := ghHappySteps(testRunID)[:2]

	cases := []struct {
		name       string
		annBody    string
		wantReason Reason
	}{
		{
			name:       "zero_annotations",
			annBody:    `[]`,
			wantReason: ReasonParentBaseMissing,
		},
		{
			name:       "only_unrelated_annotations",
			annBody:    `[{"annotation_level": "warning", "title": "ci-base-sha", "message": "` + testSHA1 + `"}, {"annotation_level": "notice", "title": null, "message": "` + testSHA1 + `"}]`,
			wantReason: ReasonParentBaseMissing,
		},
		{
			name: "duplicate_annotations",
			annBody: fmt.Sprintf(`[
				{"annotation_level": "notice", "title": "ci-base-sha", "message": %q},
				{"annotation_level": "notice", "title": "ci-base-sha", "message": %q}
			]`, testSHA1, testSHA1),
			wantReason: ReasonParentBaseDuplicate,
		},
		{
			name:       "uppercase_sha_rejected",
			annBody:    `[{"annotation_level": "notice", "title": "ci-base-sha", "message": "` + strings.Repeat("A", 40) + `"}]`,
			wantReason: ReasonAPIInvalidResponse,
		},
		{
			name:       "padded_sha_rejected",
			annBody:    `[{"annotation_level": "notice", "title": "ci-base-sha", "message": " ` + testSHA1 + ` "}]`,
			wantReason: ReasonAPIInvalidResponse,
		},
		{
			name:       "main_advanced_mismatch",
			annBody:    fmt.Sprintf(`[{"annotation_level": "notice", "title": "ci-base-sha", "message": %q}]`, testSHA2),
			wantReason: ReasonMainAdvanced,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps := append(append([]cmdrunnertestutil.Step{}, prefixSteps...), cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(annotationsEndpoint(1)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, tc.annBody),
			})
			dec := evalWithGHSteps(t, defaultLimits(), steps...)
			if dec.Skip() || dec.Reason != tc.wantReason || dec.NoticeBaseSHA != testSHA1 || dec.ParentRunID != testRunID {
				t.Errorf("decision = %+v, want Reason=%q NoticeBaseSHA=%q ParentRunID=%d",
					dec, tc.wantReason, testSHA1, testRunID)
			}
		})
	}
}

func TestVerifyParentCIAPIFailures(t *testing.T) {
	t.Run("non_200_exit_error_extracts_status_and_hides_secrets", func(t *testing.T) {
		const secretToken = "ghp_SUPER_SECRET_TOKEN_VALUE"
		exitErr := &cmdrunner.ExitError{
			Code:   1,
			Stdout: []byte("HTTP/2.0 403 Forbidden\nX-Debug: " + secretToken + "\r\n\r\n{\"message\":\"" + secretToken + "\"}"),
			Stderr: []byte("gh: " + secretToken),
		}
		dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
			Name: "gh",
			Args: ghAPIArgs(runsEndpoint(1)),
			Err:  exitErr,
		})
		if dec.Skip() || dec.Reason != ReasonAPIFailed || dec.HTTPStatus != 403 {
			t.Fatalf("decision = %+v, want Reason=%q HTTPStatus=403", dec, ReasonAPIFailed)
		}
		serialized := fmt.Sprintf("%+v", dec)
		if strings.Contains(serialized, secretToken) {
			t.Errorf("Decision leaked secret token: %s", serialized)
		}
	})

	t.Run("non_200_on_jobs_and_annotations_preserves_run_id_and_notice", func(t *testing.T) {
		happy := ghHappySteps(testRunID)
		cases := []struct {
			name       string
			steps      []cmdrunnertestutil.Step
			wantStatus int
		}{
			{
				name: "jobs_http_500",
				steps: []cmdrunnertestutil.Step{
					happy[0],
					{
						Name: "gh",
						Args: ghAPIArgs(jobsEndpoint(testRunID, 1)),
						Err:  &cmdrunner.ExitError{Code: 1, Stdout: []byte("HTTP/2.0 500 Internal Server Error\n\r\n{}")},
					},
				},
				wantStatus: 500,
			},
			{
				name: "annotations_http_403",
				steps: []cmdrunnertestutil.Step{
					happy[0],
					happy[1],
					{
						Name: "gh",
						Args: ghAPIArgs(annotationsEndpoint(1)),
						Err:  &cmdrunner.ExitError{Code: 1, Stdout: []byte("HTTP/2.0 403 Forbidden\n\r\n{}")},
					},
				},
				wantStatus: 403,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				dec := evalWithGHSteps(t, defaultLimits(), tc.steps...)
				if dec.Skip() || dec.Reason != ReasonAPIFailed || dec.HTTPStatus != tc.wantStatus ||
					dec.ParentRunID != testRunID || dec.NoticeBaseSHA != testSHA1 {
					t.Errorf("decision = %+v, want Reason=%q HTTPStatus=%d ParentRunID=%d NoticeBaseSHA=%q",
						dec, ReasonAPIFailed, tc.wantStatus, testRunID, testSHA1)
				}
			})
		}
	})

	t.Run("exit_error_with_200_or_unparseable_stdout_leaves_http_status_zero", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			stdout string
		}{
			{name: "http_200_in_exit_error", stdout: "HTTP/2.0 200 OK\n\r\n{}"},
			{name: "garbage_stdout", stdout: "not http"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Err:  &cmdrunner.ExitError{Code: 1, Stdout: []byte(tc.stdout)},
				})
				if dec.Skip() || dec.Reason != ReasonAPIFailed || dec.HTTPStatus != 0 {
					t.Errorf("decision = %+v, want Reason=%q HTTPStatus=0", dec, ReasonAPIFailed)
				}
			})
		}
	})

	t.Run("exit_zero_with_non_200_status_maps_to_api_failed", func(t *testing.T) {
		const secretToken = "ghp_EXIT_ZERO_NON200_SECRET"
		dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
			Name: "gh",
			Args: ghAPIArgs(runsEndpoint(1)),
			Out:  ghHTTPResponse("HTTP/2.0 502 Bad Gateway", nil, `{"message":"`+secretToken+`"}`),
		})
		if dec.Skip() || dec.Reason != ReasonAPIFailed || dec.HTTPStatus != 502 {
			t.Errorf("decision = %+v, want Reason=%q HTTPStatus=502", dec, ReasonAPIFailed)
		}
		if serialized := fmt.Sprintf("%+v", dec); strings.Contains(serialized, secretToken) {
			t.Errorf("Decision leaked secret token: %s", serialized)
		}
	})

	t.Run("non_exit_error_maps_to_api_failed", func(t *testing.T) {
		dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
			Name: "gh",
			Args: ghAPIArgs(runsEndpoint(1)),
			Err:  errors.New("exec: gh: not found"),
		})
		if dec.Skip() || dec.Reason != ReasonAPIFailed || dec.HTTPStatus != 0 {
			t.Errorf("decision = %+v, want Reason=%q HTTPStatus=0", dec, ReasonAPIFailed)
		}
	})

	t.Run("output_too_large", func(t *testing.T) {
		dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
			Name: "gh",
			Args: ghAPIArgs(runsEndpoint(1)),
			Err:  fmt.Errorf("gh api: %w", cmdrunner.ErrOutputTooLarge),
		})
		if dec.Skip() || dec.Reason != ReasonAPIOutputTooLarge || dec.HTTPStatus != 0 {
			t.Errorf("decision = %+v, want Reason=%q HTTPStatus=0", dec, ReasonAPIOutputTooLarge)
		}
	})

	t.Run("command_timeouts", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  error
		}{
			{name: "deadline_exceeded", err: context.DeadlineExceeded},
			{name: "joined_deadline_and_exit_error", err: errors.Join(context.DeadlineExceeded, &cmdrunner.ExitError{Code: -1, Stdout: []byte("HTTP/2.0 500\n")})},
			{name: "wait_delay", err: exec.ErrWaitDelay},
		} {
			t.Run(tc.name, func(t *testing.T) {
				dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Err:  tc.err,
				})
				if dec.Skip() || dec.Reason != ReasonAPITimeout || dec.HTTPStatus != 0 {
					t.Errorf("decision = %+v, want Reason=%q HTTPStatus=0", dec, ReasonAPITimeout)
				}
			})
		}
		t.Run("enforces_command_timeout_deadline", func(t *testing.T) {
			depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))
			synctest.Test(t, func(t *testing.T) {
				limits := defaultLimits()
				limits.runTimeout = time.Minute
				limits.commandTimeout = 5 * time.Second

				gitFake := cmdrunnertestutil.New(t, localGitPassSteps("docs/note.md\x00")...)
				scriptFake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
					Name: "bash",
					Args: []string{hasExtensionChangesScript, depFile},
					Out:  "false\n",
				})
				ghRunner := &deadlineBlockingRunner{t: t, maxRemaining: limits.commandTimeout}

				ev := newWithRunners(gitFake, scriptFake, ghRunner, limits)
				dec := ev.Evaluate(t.Context(), validEvalInput(depFile))
				if !ghRunner.called {
					t.Fatalf("gh runner was not called")
				}
				if dec.Skip() || dec.Reason != ReasonAPITimeout || dec.HTTPStatus != 0 {
					t.Errorf("decision = %+v, want Reason=%q HTTPStatus=0", dec, ReasonAPITimeout)
				}
			})
		})
	})

	t.Run("overall_run_timeout_expires_before_next_api_call", func(t *testing.T) {
		depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))
		synctest.Test(t, func(t *testing.T) {
			limits := defaultLimits()
			limits.runTimeout = 10 * time.Second
			limits.commandTimeout = time.Minute

			okRunStep := ghHappySteps(testRunID)[0]
			okRunStep.Effect = func() error {
				time.Sleep(limits.runTimeout + time.Second)
				return nil
			}
			// Jobs API must not even be invoked once RunTimeout has elapsed.
			dec := evalWithDepFile(t, depFile, limits, okRunStep)
			if dec.Skip() || dec.Reason != ReasonAPITimeout || dec.ParentRunID != testRunID {
				t.Errorf("decision = %+v, want Reason=%q ParentRunID=%d", dec, ReasonAPITimeout, testRunID)
			}
		})
	})

	t.Run("overall_run_timeout_expires_on_final_annotations_call", func(t *testing.T) {
		depFile := writeTempFile(t, "wasm-dep-dirs", []byte("internal/llm/devfake/\n"))
		synctest.Test(t, func(t *testing.T) {
			limits := defaultLimits()
			limits.runTimeout = 10 * time.Second
			limits.commandTimeout = time.Minute

			steps := ghHappySteps(testRunID)
			steps[2].Effect = func() error {
				time.Sleep(limits.runTimeout + time.Second)
				return nil
			}
			dec := evalWithDepFile(t, depFile, limits, steps...)
			if dec.Skip() || dec.Reason != ReasonAPITimeout || dec.ParentRunID != testRunID || dec.NoticeBaseSHA != testSHA1 {
				t.Errorf("decision = %+v, want Reason=%q ParentRunID=%d NoticeBaseSHA=%q",
					dec, ReasonAPITimeout, testRunID, testSHA1)
			}
		})
	})

	t.Run("framing_and_link_header_errors", func(t *testing.T) {
		// Every case uses a valid empty workflow_runs body so that if a framing or Link
		// check were broken, findLatestParentRun would return ReasonNoParentRun instead
		// of ReasonAPIInvalidResponse.
		const validRunsBody = `{"workflow_runs":[]}`
		framingCases := []struct {
			name string
			raw  string
		}{
			{name: "empty_output", raw: ""},
			{name: "crlf_status_line", raw: "HTTP/2.0 200 OK\r\nContent-Type: application/json\r\n\r\n" + validRunsBody},
			{name: "bad_status_protocol", raw: "HTTP/3.0 200 OK\n\r\n" + validRunsBody},
			{name: "missing_header_blank_line", raw: "HTTP/2.0 200 OK\nContent-Type: application/json\r\n" + validRunsBody},
			{name: "malformed_header_no_colon", raw: "HTTP/2.0 200 OK\nBadHeader\r\n\r\n" + validRunsBody},
			{name: "malformed_header_space_in_name", raw: "HTTP/2.0 200 OK\nBad Name: 1\r\n\r\n" + validRunsBody},
			{name: "malformed_link_unclosed_uri", raw: ghHTTPResponse("HTTP/2.0 200 OK", []string{`Link: <https://api.github.com/x; rel="next"`}, validRunsBody)},
			{name: "malformed_link_unclosed_quote", raw: ghHTTPResponse("HTTP/2.0 200 OK", []string{`Link: <https://api.github.com/x>; rel="next`}, validRunsBody)},
			{name: "malformed_link_trailing_comma", raw: ghHTTPResponse("HTTP/2.0 200 OK", []string{`Link: <https://api.github.com/x>; rel="last", `}, validRunsBody)},
			{name: "malformed_link_dangling_escape", raw: ghHTTPResponse("HTTP/2.0 200 OK", []string{`Link: <https://api.github.com/x>; title="abc\`}, validRunsBody)},
			{name: "malformed_link_invalid_token_char", raw: ghHTTPResponse("HTTP/2.0 200 OK", []string{`Link: <https://api.github.com/x>; rel=bad"val`}, validRunsBody)},
			{name: "malformed_link_slash_in_unquoted_token", raw: ghHTTPResponse("HTTP/2.0 200 OK", []string{`Link: <https://api.github.com/x>; rel=prev/next`}, validRunsBody)},
		}
		for _, tc := range framingCases {
			t.Run(tc.name, func(t *testing.T) {
				dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Out:  tc.raw,
				})
				if dec.Skip() || dec.Reason != ReasonAPIInvalidResponse {
					t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAPIInvalidResponse)
				}
			})
		}
	})

	t.Run("link_header_ignores_evil_host_and_false_rel_substrings", func(t *testing.T) {
		// Page 1 carries multiple Link headers, a valueless parameter, and an evil host URL;
		// evaluator MUST request fixed github.com endpoint page=2.
		// Page 2 carries rel="next" inside URI, inside an escaped quoted title, and after a
		// valueless or prior rel parameter (ignored per RFC 8288 section 3.3);
		// evaluator MUST NOT request page=3.
		runsP1 := `{"workflow_runs": []}`
		runsP2 := fmt.Sprintf(`{"workflow_runs": [%s]}`,
			standardRunJSON(testRunID, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:05Z"))
		happy := ghHappySteps(testRunID)

		dec := evalWithGHSteps(t, defaultLimits(),
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(runsEndpoint(1)),
				Out: ghHTTPResponse("HTTP/2.0 200 OK", []string{
					`Link: <https://api.github.com/prev>; rel="prev"`,
					`Link: <https://evil.example/steal?page=99>; foo; rel="next"`,
				}, runsP1),
			},
			cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(runsEndpoint(2)),
				Out: ghHTTPResponse("HTTP/2.0 200 OK", []string{
					`Link: <https://api.github.com/x?q=a,b;rel="next">; title="fake\"; rel=next"; rel="last"; rel="next", <https://api.github.com/y>; rel; rel="next"`,
				}, runsP2),
			},
			happy[1],
			happy[2],
		)
		if !dec.Skip() || dec.Reason != ReasonSkipped {
			t.Errorf("decision = %+v, want Skip()=true Reason=%q", dec, ReasonSkipped)
		}
	})

	t.Run("pagination_limit_exceeded", func(t *testing.T) {
		limits := defaultLimits()
		limits.maxRunPages = 2
		limits.maxJobPages = 1
		limits.maxAnnotationPages = 1

		nextHdr := []string{`Link: <https://api.github.com/next>; rel="next"`}
		happy := ghHappySteps(testRunID)

		cases := []struct {
			name  string
			steps []cmdrunnertestutil.Step
		}{
			{
				name: "runs_limit",
				steps: []cmdrunnertestutil.Step{
					{Name: "gh", Args: ghAPIArgs(runsEndpoint(1)), Out: ghHTTPResponse("HTTP/2.0 200 OK", nextHdr, `{"workflow_runs":[]}`)},
					{Name: "gh", Args: ghAPIArgs(runsEndpoint(2)), Out: ghHTTPResponse("HTTP/2.0 200 OK", nextHdr, `{"workflow_runs":[]}`)},
				},
			},
			{
				name: "jobs_limit",
				steps: []cmdrunnertestutil.Step{
					happy[0],
					{Name: "gh", Args: ghAPIArgs(jobsEndpoint(testRunID, 1)), Out: ghHTTPResponse("HTTP/2.0 200 OK", nextHdr, `{"jobs":[]}`)},
				},
			},
			{
				name: "annotations_limit",
				steps: []cmdrunnertestutil.Step{
					happy[0],
					happy[1],
					{Name: "gh", Args: ghAPIArgs(annotationsEndpoint(1)), Out: ghHTTPResponse("HTTP/2.0 200 OK", nextHdr, `[]`)},
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				dec := evalWithGHSteps(t, limits, tc.steps...)
				if dec.Skip() || dec.Reason != ReasonAPIPaginationLimit {
					t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAPIPaginationLimit)
				}
			})
		}
	})

	t.Run("strictjson_violations", func(t *testing.T) {
		validRun := standardRunJSON(testRunID, "completed", `"success"`, "2026-11-21T10:00:00Z", "2026-11-21T10:00:05Z")

		runsInvalid := []struct {
			name string
			body string
		}{
			{name: "invalid_utf8", body: "{\"workflow_runs\":[],\"x\":\"\xff\"}"},
			{name: "unpaired_surrogate", body: `{"workflow_runs":[],"x":"\uD800"}`},
			{name: "trailing_json", body: `{"workflow_runs":[]}{}`},
			{name: "duplicate_consumed_key", body: `{"workflow_runs":[],"workflow_runs":[]}`},
			{name: "case_variant_consumed_key", body: `{"workflow_runs":[],"Workflow_Runs":[]}`},
			{name: "id_zero", body: `{"workflow_runs":[` + strings.Replace(validRun, fmt.Sprintf(`"id": %d`, testRunID), `"id": 0`, 1) + `]}`},
			{name: "id_float", body: `{"workflow_runs":[` + strings.Replace(validRun, fmt.Sprintf(`"id": %d`, testRunID), `"id": 1.0`, 1) + `]}`},
			{name: "id_exponent", body: `{"workflow_runs":[` + strings.Replace(validRun, fmt.Sprintf(`"id": %d`, testRunID), `"id": 1e2`, 1) + `]}`},
			{name: "empty_conclusion_string", body: `{"workflow_runs":[` + strings.Replace(validRun, `"conclusion": "success"`, `"conclusion": ""`, 1) + `]}`},
			{name: "pr_number_zero", body: `{"workflow_runs":[` + strings.Replace(validRun, `"number": 1073`, `"number": 0`, 1) + `]}`},
		}
		for _, tc := range runsInvalid {
			t.Run("runs_"+tc.name, func(t *testing.T) {
				dec := evalWithGHSteps(t, defaultLimits(), cmdrunnertestutil.Step{
					Name: "gh",
					Args: ghAPIArgs(runsEndpoint(1)),
					Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, tc.body),
				})
				if dec.Skip() || dec.Reason != ReasonAPIInvalidResponse {
					t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAPIInvalidResponse)
				}
			})
		}

		happy := ghHappySteps(testRunID)
		t.Run("jobs_case_variant_key", func(t *testing.T) {
			body := fmt.Sprintf(`{"jobs": [{"id": 9001, "name": "Check changed files", "head_sha": %q, "status": "completed", "Status": "completed", "conclusion": "success"}]}`, testSHA4)
			dec := evalWithGHSteps(t, defaultLimits(), happy[0], cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(jobsEndpoint(testRunID, 1)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, body),
			})
			if dec.Skip() || dec.Reason != ReasonAPIInvalidResponse {
				t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAPIInvalidResponse)
			}
		})

		t.Run("annotations_duplicate_message_key", func(t *testing.T) {
			body := fmt.Sprintf(`[{"annotation_level": "notice", "title": "ci-base-sha", "message": %q, "message": %q}]`, testSHA1, testSHA1)
			dec := evalWithGHSteps(t, defaultLimits(), happy[0], happy[1], cmdrunnertestutil.Step{
				Name: "gh",
				Args: ghAPIArgs(annotationsEndpoint(1)),
				Out:  ghHTTPResponse("HTTP/2.0 200 OK", nil, body),
			})
			if dec.Skip() || dec.Reason != ReasonAPIInvalidResponse {
				t.Errorf("decision = %+v, want Reason=%q", dec, ReasonAPIInvalidResponse)
			}
		})
	})
}
