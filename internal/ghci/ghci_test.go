//go:build test

package ghci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

const (
	testOwner   = "octo-org"
	testRepo    = "yt2column"
	testPR      = 42
	testHead    = "0123456789abcdef0123456789abcdef01234567"
	testAltHead = "fedcba9876543210fedcba9876543210fedcba98"
	testRunURL1 = "https://github.com/octo-org/yt2column/actions/runs/100/job/1"
	testRunURL2 = "https://github.com/octo-org/yt2column/actions/runs/100/job/2"
	testMarker  = "FORBIDDEN-UNTRUSTED-MARKER-9f8e7d"
)

type testRunSpec struct {
	name       string
	status     string
	conclusion *string
	url        string
	extra      map[string]any
}

type testSuiteSpec struct {
	appNull        bool
	appSlug        string
	status         string
	conclusion     *string
	createdAt      string
	hasWorkflowRun bool
	workflowNull   bool
	workflowName   string
	runsNextPage   bool
	runs           []testRunSpec
}

func testReq() Request {
	return Request{
		Owner:        testOwner,
		Repo:         testRepo,
		PRNumber:     testPR,
		ExpectedHead: testHead,
	}
}

func testGraphQLArgs(req Request) []string {
	return []string{
		"api",
		"graphql",
		"-f", "owner=" + req.Owner,
		"-f", "name=" + req.Repo,
		"-F", "number=" + strconv.Itoa(req.PRNumber),
		"-f", "query=" + prChecksQuery,
	}
}

func buildGraphQLResponse(t *testing.T, headRefOID, commitOID string, suitesNextPage bool, suites []testSuiteSpec) string {
	t.Helper()
	suiteNodes := make([]map[string]any, 0, len(suites))
	for _, s := range suites {
		var app any
		if !s.appNull {
			app = map[string]any{"slug": s.appSlug}
		}
		var workflowRun any
		if s.hasWorkflowRun {
			if s.workflowNull {
				workflowRun = map[string]any{"workflow": nil}
			} else {
				workflowRun = map[string]any{"workflow": map[string]any{"name": s.workflowName}}
			}
		}
		runNodes := make([]map[string]any, 0, len(s.runs))
		for _, r := range s.runs {
			node := map[string]any{
				"name":       r.name,
				"status":     r.status,
				"conclusion": r.conclusion,
				"url":        r.url,
			}
			maps.Copy(node, r.extra)
			runNodes = append(runNodes, node)
		}
		createdAt := s.createdAt
		if createdAt == "" {
			createdAt = "2026-11-24T10:00:00Z"
		}
		suiteNodes = append(suiteNodes, map[string]any{
			"app":         app,
			"status":      s.status,
			"conclusion":  s.conclusion,
			"createdAt":   createdAt,
			"workflowRun": workflowRun,
			"checkRuns": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": s.runsNextPage},
				"nodes":    runNodes,
			},
		})
	}
	payload := map[string]any{
		"data": map[string]any{
			"repository": map[string]any{
				"pullRequest": map[string]any{
					"headRefOid": headRefOID,
					"commits": map[string]any{
						"nodes": []any{
							map[string]any{
								"commit": map[string]any{
									"oid": commitOID,
									"checkSuites": map[string]any{
										"pageInfo": map[string]any{"hasNextPage": suitesNextPage},
										"nodes":    suiteNodes,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal graphql fixture: %v", err)
	}
	return string(b)
}

func passingCISuite() testSuiteSpec {
	return testSuiteSpec{
		appSlug:        "github-actions",
		status:         "COMPLETED",
		conclusion:     new("SUCCESS"),
		createdAt:      "2026-11-24T10:00:00Z",
		hasWorkflowRun: true,
		workflowName:   "CI",
		runs: []testRunSpec{
			{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: testRunURL1},
			{name: "Browser extension", status: "COMPLETED", conclusion: new("SKIPPED"), url: testRunURL2},
		},
	}
}

func TestResolveRepo(t *testing.T) {
	t.Run("valid_owner_and_repo", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: []string{"repo", "view", "--json", "nameWithOwner"},
			Out:  `{"nameWithOwner":"octo-org/my_repo.v1-test"}`,
		})
		owner, repo, err := ResolveRepo(context.Background(), fake)
		if err != nil {
			t.Fatalf("ResolveRepo unexpected error: %v", err)
		}
		if owner != "octo-org" || repo != "my_repo.v1-test" {
			t.Fatalf("got %q/%q, want octo-org/my_repo.v1-test", owner, repo)
		}
	})

	t.Run("nil_runner_returns_invalid_request", func(t *testing.T) {
		_, _, err := ResolveRepo(context.Background(), nil)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("got err=%v, want ErrInvalidRequest", err)
		}
	})

	t.Run("gh_failure_returns_call_failed_without_leaking_text", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: []string{"repo", "view", "--json", "nameWithOwner"},
			Err:  fmt.Errorf("gh failed: %s", testMarker),
		})
		_, _, err := ResolveRepo(context.Background(), fake)
		repoErr, ok := errors.AsType[*RepoError](err)
		if !ok {
			t.Fatalf("got err=%T (%v), want *RepoError", err, err)
		}
		if repoErr.Reason != UnreadableCall {
			t.Fatalf("got reason=%v, want UnreadableCall", repoErr.Reason)
		}
		if strings.Contains(repoErr.Error(), testMarker) {
			t.Fatalf("RepoError leaked marker: %q", repoErr.Error())
		}
	})

	t.Run("output_too_large_error_returns_limit", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: []string{"repo", "view", "--json", "nameWithOwner"},
			Err:  fmt.Errorf("wrapped: %w", cmdrunner.ErrOutputTooLarge),
		})
		_, _, err := ResolveRepo(context.Background(), fake)
		repoErr, ok := errors.AsType[*RepoError](err)
		if !ok || repoErr.Reason != UnreadableLimit {
			t.Fatalf("got err=%v, want *RepoError(UnreadableLimit)", err)
		}
		if !strings.Contains(repoErr.Error(), "limit") {
			t.Fatalf("unexpected limit error message: %q", repoErr.Error())
		}
	})

	t.Run("oversized_output_returns_limit", func(t *testing.T) {
		oversized := strings.Repeat(" ", maxResponseBytes+1)
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: []string{"repo", "view", "--json", "nameWithOwner"},
			Out:  oversized,
		})
		_, _, err := ResolveRepo(context.Background(), fake)
		repoErr, ok := errors.AsType[*RepoError](err)
		if !ok || repoErr.Reason != UnreadableLimit {
			t.Fatalf("got err=%v, want *RepoError(UnreadableLimit)", err)
		}
	})

	rejectedCases := []struct {
		name string
		out  string
	}{
		{name: "not_json", out: "not-json " + testMarker},
		{name: "duplicate_key", out: `{"nameWithOwner":"a/b","nameWithOwner":"c/d"}`},
		{name: "trailing_json", out: `{"nameWithOwner":"a/b"} {}`},
		{name: "invalid_utf8", out: "{\"nameWithOwner\":\"a/b\xff\"}"},
		{name: "unpaired_surrogate", out: `{"nameWithOwner":"a/\uD800"}`},
		{name: "missing_member", out: `{}`},
		{name: "empty_string", out: `{"nameWithOwner":""}`},
		{name: "no_slash", out: `{"nameWithOwner":"octo-org"}`},
		{name: "extra_slash", out: `{"nameWithOwner":"octo-org/repo/extra"}`},
		{name: "bad_owner_leading_hyphen", out: `{"nameWithOwner":"-octo/repo"}`},
		{name: "bad_owner_underscore", out: `{"nameWithOwner":"octo_org/repo"}`},
		{name: "bad_owner_too_long", out: `{"nameWithOwner":"` + strings.Repeat("a", maxLoginLength+1) + `/repo"}`},
		{name: "bad_repo_dot", out: `{"nameWithOwner":"octo/."}`},
		{name: "bad_repo_dotdot", out: `{"nameWithOwner":"octo/.."}`},
		{name: "bad_repo_too_long", out: `{"nameWithOwner":"octo/` + strings.Repeat("a", maxRepoNameLength+1) + `"}`},
		{name: "bad_repo_space", out: `{"nameWithOwner":"octo/my repo"}`},
	}
	for _, tc := range rejectedCases {
		t.Run("rejected_"+tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "gh",
				Args: []string{"repo", "view", "--json", "nameWithOwner"},
				Out:  tc.out,
			})
			_, _, err := ResolveRepo(context.Background(), fake)
			repoErr, ok := errors.AsType[*RepoError](err)
			if !ok || repoErr.Reason != UnreadableRejected {
				t.Fatalf("got err=%v, want *RepoError(UnreadableRejected)", err)
			}
			if strings.Contains(repoErr.Error(), testMarker) {
				t.Fatalf("RepoError leaked marker: %q", repoErr.Error())
			}
		})
	}

	t.Run("repo_error_default_reason_message", func(t *testing.T) {
		msg := (&RepoError{Reason: UnreadableReason(99)}).Error()
		if msg == "" {
			t.Fatal("expected non-empty fallback message")
		}
	})
}

func TestReadPRValidation(t *testing.T) {
	fake := cmdrunnertestutil.New(t)
	cases := []struct {
		name string
		run  cmdrunner.Runner
		req  Request
	}{
		{name: "nil_runner", run: nil, req: testReq()},
		{name: "empty_owner", run: fake, req: Request{Owner: "", Repo: testRepo, PRNumber: testPR}},
		{name: "invalid_owner", run: fake, req: Request{Owner: "-bad", Repo: testRepo, PRNumber: testPR}},
		{name: "empty_repo", run: fake, req: Request{Owner: testOwner, Repo: "", PRNumber: testPR}},
		{name: "dotdot_repo", run: fake, req: Request{Owner: testOwner, Repo: "..", PRNumber: testPR}},
		{name: "zero_pr", run: fake, req: Request{Owner: testOwner, Repo: testRepo, PRNumber: 0}},
		{name: "negative_pr", run: fake, req: Request{Owner: testOwner, Repo: testRepo, PRNumber: -5}},
		{name: "short_expected_head", run: fake, req: Request{Owner: testOwner, Repo: testRepo, PRNumber: testPR, ExpectedHead: "0123"}},
		{name: "uppercase_expected_head", run: fake, req: Request{Owner: testOwner, Repo: testRepo, PRNumber: testPR, ExpectedHead: strings.ToUpper(testHead)}},
		{name: "non_hex_expected_head", run: fake, req: Request{Owner: testOwner, Repo: testRepo, PRNumber: testPR, ExpectedHead: strings.Repeat("z", sha1HexLength)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadPR(context.Background(), tc.run, tc.req)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("got err=%v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestReadPR(t *testing.T) {
	req := testReq()
	args := testGraphQLArgs(req)

	cases := []struct {
		name string
		req  Request
		out  string
		want Result
	}{
		{
			name: "passed_success_and_skipped_mix",
			req:  req,
			out:  buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{passingCISuite()}),
			want: Result{State: StatePassed, Head: testHead, Suites: 1, Checks: 2, Failing: []FailingCheck{}},
		},
		{
			name: "passed_without_expected_head",
			req:  Request{Owner: testOwner, Repo: testRepo, PRNumber: testPR},
			out:  buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{passingCISuite()}),
			want: Result{State: StatePassed, Head: testHead, Suites: 1, Checks: 2, Failing: []FailingCheck{}},
		},
		{
			name: "external_apps_ignored_including_failure_queued_and_over_cap_hasNextPage",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:      "sourcery-ai",
					status:       "COMPLETED",
					conclusion:   new("FAILURE"),
					runsNextPage: true,
					runs: []testRunSpec{
						{name: "Sourcery review", status: "COMPLETED", conclusion: new("FAILURE"), url: testRunURL1},
					},
				},
				{
					appSlug:      "sourcery-ai",
					status:       "QUEUED",
					runsNextPage: true,
				},
				{
					appSlug:    "coderabbitai",
					status:     "COMPLETED",
					conclusion: new("SKIPPED"),
					runs: []testRunSpec{
						{name: "CodeRabbit", status: "COMPLETED", conclusion: new("SKIPPED"), url: testRunURL2},
					},
				},
				{
					appNull:      true,
					status:       "COMPLETED",
					conclusion:   new("FAILURE"),
					runsNextPage: true,
				},
				passingCISuite(),
			}),
			want: Result{State: StatePassed, Head: testHead, Suites: 1, Checks: 2, Failing: []FailingCheck{}},
		},
		{
			name: "only_external_apps_gives_pending_zero_suites",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:    "sourcery-ai",
					status:     "COMPLETED",
					conclusion: new("SUCCESS"),
					runs: []testRunSpec{
						{name: "Sourcery review", status: "COMPLETED", conclusion: new("SUCCESS"), url: testRunURL1},
					},
				},
			}),
			want: Result{State: StatePending, Head: testHead, Suites: 0, Checks: 0, Failing: []FailingCheck{}},
		},
		{
			name: "same_workflow_rerun_keeps_latest_createdAt_passing",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("FAILURE"),
					createdAt:      "2026-11-24T09:00:00Z",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Test", status: "COMPLETED", conclusion: new("FAILURE"), url: testRunURL1},
					},
				},
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("SUCCESS"),
					createdAt:      "2026-11-24T10:00:00Z",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: testRunURL2},
					},
				},
			}),
			want: Result{State: StatePassed, Head: testHead, Suites: 1, Checks: 1, Failing: []FailingCheck{}},
		},
		{
			name: "same_workflow_rerun_newer_first_still_picks_latest_createdAt",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("SUCCESS"),
					createdAt:      "2026-11-24T11:00:00Z",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: testRunURL2},
					},
				},
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("CANCELLED"),
					createdAt:      "2026-11-24T09:00:00Z",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Test", status: "COMPLETED", conclusion: new("CANCELLED"), url: testRunURL1},
					},
				},
			}),
			want: Result{State: StatePassed, Head: testHead, Suites: 1, Checks: 1, Failing: []FailingCheck{}},
		},
		{
			name: "same_workflow_rerun_newer_failed_beats_older_success",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("SUCCESS"),
					createdAt:      "2026-11-24T09:00:00Z",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: testRunURL1},
					},
				},
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("FAILURE"),
					createdAt:      "2026-11-24T10:00:00Z",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Test", status: "COMPLETED", conclusion: new("FAILURE"), url: testRunURL2},
					},
				},
			}),
			want: Result{
				State:  StateFailed,
				Head:   testHead,
				Suites: 1,
				Checks: 1,
				Failing: []FailingCheck{
					{Name: "Test", Conclusion: "FAILURE", URL: testRunURL2},
				},
			},
		},
		{
			name: "empty_queued_suite_without_workflowRun_is_excluded",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "QUEUED",
					hasWorkflowRun: false,
				},
				passingCISuite(),
			}),
			want: Result{State: StatePassed, Head: testHead, Suites: 1, Checks: 2, Failing: []FailingCheck{}},
		},
		{
			name: "empty_queued_suite_with_workflowRun_is_pending_with_suite_count",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "QUEUED",
					hasWorkflowRun: true,
					workflowName:   "CI",
				},
			}),
			want: Result{State: StatePending, Head: testHead, Suites: 1, Checks: 0, Failing: []FailingCheck{}},
		},
		{
			name: "zero_checkrun_startup_failure_with_null_workflowRun_fails_immediately",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("STARTUP_FAILURE"),
					hasWorkflowRun: false,
				},
			}),
			want: Result{
				State:  StateFailed,
				Head:   testHead,
				Suites: 1,
				Checks: 0,
				Failing: []FailingCheck{
					{Name: fallbackSuiteName, Conclusion: "STARTUP_FAILURE", URL: ""},
				},
			},
		},
		{
			name: "suite_level_failure_without_failing_run_uses_workflow_name",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("FAILURE"),
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: testRunURL1},
					},
				},
			}),
			want: Result{
				State:  StateFailed,
				Head:   testHead,
				Suites: 1,
				Checks: 1,
				Failing: []FailingCheck{
					{Name: "CI", Conclusion: "FAILURE", URL: ""},
				},
			},
		},
		{
			name: "fail_fast_while_sibling_run_still_in_progress",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "IN_PROGRESS",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Lint", status: "COMPLETED", conclusion: new("TIMED_OUT"), url: testRunURL1},
						{name: "Test", status: "IN_PROGRESS", url: testRunURL2},
					},
				},
			}),
			want: Result{
				State:  StateFailed,
				Head:   testHead,
				Suites: 1,
				Checks: 2,
				Failing: []FailingCheck{
					{Name: "Lint", Conclusion: "TIMED_OUT", URL: testRunURL1},
				},
			},
		},
		{
			name: "neutral_and_unknown_conclusions_fail_closed",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("NEUTRAL"),
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "B", status: "COMPLETED", conclusion: new("NEUTRAL"), url: testRunURL2},
						{name: "A", status: "COMPLETED", conclusion: new("FUTURE_STATE"), url: testRunURL1},
					},
				},
			}),
			want: Result{
				State:  StateFailed,
				Head:   testHead,
				Suites: 1,
				Checks: 2,
				Failing: []FailingCheck{
					{Name: "A", Conclusion: "FUTURE_STATE", URL: testRunURL1},
					{Name: "B", Conclusion: "NEUTRAL", URL: testRunURL2},
				},
			},
		},
		{
			name: "pending_when_suite_in_progress_even_if_existing_runs_passed",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "IN_PROGRESS",
					hasWorkflowRun: true,
					workflowName:   "CI",
					runs: []testRunSpec{
						{name: "Check changed files", status: "COMPLETED", conclusion: new("SUCCESS"), url: testRunURL1},
					},
				},
			}),
			want: Result{State: StatePending, Head: testHead, Suites: 1, Checks: 1, Failing: []FailingCheck{}},
		},
		{
			name: "pending_when_run_queued",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("SUCCESS"),
					hasWorkflowRun: true,
					workflowNull:   true,
					runs: []testRunSpec{
						{name: "Test", status: "QUEUED", url: testRunURL1},
					},
				},
			}),
			want: Result{State: StatePending, Head: testHead, Suites: 1, Checks: 1, Failing: []FailingCheck{}},
		},
		{
			name: "head_changed_when_expected_head_differs",
			req:  req,
			out:  buildGraphQLResponse(t, testAltHead, testAltHead, false, []testSuiteSpec{passingCISuite()}),
			want: Result{State: StateUnreadable, Reason: UnreadableHeadChanged, Head: testAltHead, Failing: []FailingCheck{}},
		},
		{
			name: "commit_oid_mismatch_is_rejected_not_head_changed",
			req:  req,
			out:  buildGraphQLResponse(t, testAltHead, testHead, false, []testSuiteSpec{passingCISuite()}),
			want: Result{State: StateUnreadable, Reason: UnreadableRejected, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "suites_hasNextPage_is_limit",
			req:  req,
			out:  buildGraphQLResponse(t, testHead, testHead, true, []testSuiteSpec{passingCISuite()}),
			want: Result{State: StateUnreadable, Reason: UnreadableLimit, Head: testHead, Failing: []FailingCheck{}},
		},
		{
			name: "github_actions_runs_hasNextPage_is_limit",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("SUCCESS"),
					hasWorkflowRun: true,
					workflowName:   "CI",
					runsNextPage:   true,
				},
			}),
			want: Result{State: StateUnreadable, Reason: UnreadableLimit, Head: testHead, Failing: []FailingCheck{}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "gh",
				Args: testGraphQLArgs(tc.req),
				Out:  tc.out,
			})
			got, err := ReadPR(context.Background(), fake, tc.req)
			if err != nil {
				t.Fatalf("ReadPR unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			if got.Failing == nil {
				t.Fatal("Result.Failing must be non-nil slice")
			}
		})
	}

	t.Run("gh_call_error_returns_unreadable_call", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: args,
			Err:  fmt.Errorf("network failure: %s", testMarker),
		})
		got, err := ReadPR(context.Background(), fake, req)
		if err != nil {
			t.Fatalf("ReadPR unexpected error: %v", err)
		}
		want := Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		if strings.Contains(string(raw), testMarker) {
			t.Fatalf("marshaled result leaked marker: %s", raw)
		}
	})

	t.Run("output_too_large_error_returns_unreadable_limit", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: args,
			Err:  cmdrunner.ErrOutputTooLarge,
		})
		got, err := ReadPR(context.Background(), fake, req)
		if err != nil {
			t.Fatalf("ReadPR unexpected error: %v", err)
		}
		if got.State != StateUnreadable || got.Reason != UnreadableLimit {
			t.Fatalf("got %+v, want unreadable/limit", got)
		}
	})

	t.Run("oversized_response_returns_unreadable_limit", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: args,
			Out:  strings.Repeat(" ", maxResponseBytes+1),
		})
		got, err := ReadPR(context.Background(), fake, req)
		if err != nil {
			t.Fatalf("ReadPR unexpected error: %v", err)
		}
		if got.State != StateUnreadable || got.Reason != UnreadableLimit {
			t.Fatalf("got %+v, want unreadable/limit", got)
		}
	})

	t.Run("too_many_suites_returns_limit", func(t *testing.T) {
		suites := make([]testSuiteSpec, maxCheckSuites+1)
		for i := range suites {
			suites[i] = passingCISuite()
		}
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: args,
			Out:  buildGraphQLResponse(t, testHead, testHead, false, suites),
		})
		got, err := ReadPR(context.Background(), fake, req)
		if err != nil {
			t.Fatalf("ReadPR unexpected error: %v", err)
		}
		if got.State != StateUnreadable || got.Reason != UnreadableLimit {
			t.Fatalf("got %+v, want unreadable/limit", got)
		}
	})

	t.Run("too_many_github_actions_runs_returns_limit", func(t *testing.T) {
		runs := make([]testRunSpec, maxCheckRuns+1)
		for i := range runs {
			runs[i] = testRunSpec{
				name:       fmt.Sprintf("job-%d", i),
				status:     "COMPLETED",
				conclusion: new("SUCCESS"),
				url:        testRunURL1,
			}
		}
		suite := passingCISuite()
		suite.runs = runs
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh",
			Args: args,
			Out:  buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{suite}),
		})
		got, err := ReadPR(context.Background(), fake, req)
		if err != nil {
			t.Fatalf("ReadPR unexpected error: %v", err)
		}
		if got.State != StateUnreadable || got.Reason != UnreadableLimit {
			t.Fatalf("got %+v, want unreadable/limit", got)
		}
	})
}

func TestReadPRStrictJSONRejections(t *testing.T) {
	req := testReq()
	args := testGraphQLArgs(req)
	validJSON := buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{passingCISuite()})

	cases := []struct {
		name string
		out  string
	}{
		{name: "invalid_utf8", out: validJSON + "\xff"},
		{name: "unpaired_surrogate", out: strings.Replace(validJSON, `"Test"`, `"\uD800"`, 1)},
		{name: "trailing_json", out: validJSON + " {}"},
		{name: "duplicate_headRefOid", out: strings.Replace(validJSON, `"headRefOid":"`+testHead+`"`, `"headRefOid":"`+testHead+`","headRefOid":"`+testHead+`"`, 1)},
		{name: "graphql_errors_non_empty", out: `{"data":null,"errors":[{"message":"` + testMarker + `"}]}`},
		{name: "graphql_errors_not_array", out: `{"data":{},"errors":"boom"}`},
		{name: "empty_commits_nodes", out: `{"data":{"repository":{"pullRequest":{"headRefOid":"` + testHead + `","commits":{"nodes":[]}}}}}`},
		{name: "two_commits_nodes", out: `{"data":{"repository":{"pullRequest":{"headRefOid":"` + testHead + `","commits":{"nodes":[{"commit":{"oid":"` + testHead + `"}},{"commit":{"oid":"` + testHead + `"}}]}}}}}`},
		{name: "uppercase_headRefOid", out: strings.ReplaceAll(validJSON, testHead, strings.ToUpper(testHead))},
		{name: "completed_suite_null_conclusion", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{appSlug: "github-actions", status: "COMPLETED", conclusion: nil, hasWorkflowRun: true, workflowName: "CI"},
		})},
		{name: "completed_run_null_conclusion", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: nil, url: testRunURL1}},
			},
		})},
		{name: "invalid_createdAt", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), createdAt: "not-a-date", hasWorkflowRun: true, workflowName: "CI"},
		})},
		{name: "non_github_url", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: "https://evil.example/runs/1"}},
			},
		})},
		{name: "github_prefix_spoof_host", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: "https://github.com.evil.example/runs/1"}},
			},
		})},
		{name: "github_url_empty_path", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: "https://github.com/"}},
			},
		})},
		{name: "github_url_with_space", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: "https://github.com/octo/repo/runs/1 bad"}},
			},
		})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "gh",
				Args: args,
				Out:  tc.out,
			})
			got, err := ReadPR(context.Background(), fake, req)
			if err != nil {
				t.Fatalf("ReadPR unexpected error: %v", err)
			}
			if got.State != StateUnreadable || got.Reason != UnreadableRejected {
				t.Fatalf("got %+v, want unreadable/rejected", got)
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if strings.Contains(string(raw), testMarker) {
				t.Fatalf("result leaked marker: %s", raw)
			}
		})
	}
}

func TestReadPRDoesNotOutputUntrustedFields(t *testing.T) {
	req := testReq()
	suite := testSuiteSpec{
		appSlug:        "github-actions",
		status:         "COMPLETED",
		conclusion:     new("FAILURE"),
		hasWorkflowRun: true,
		workflowName:   "CI",
		runs: []testRunSpec{
			{
				name:       "Test",
				status:     "COMPLETED",
				conclusion: new("FAILURE"),
				url:        testRunURL1,
				extra: map[string]any{
					"title":       testMarker + "-title",
					"details_url": "https://evil.example/" + testMarker,
					"output":      map[string]any{"summary": testMarker + "-summary"},
				},
			},
		},
	}
	fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
		Name: "gh",
		Args: testGraphQLArgs(req),
		Out:  buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{suite}),
	})
	got, err := ReadPR(context.Background(), fake, req)
	if err != nil {
		t.Fatalf("ReadPR unexpected error: %v", err)
	}
	if got.State != StateFailed {
		t.Fatalf("got state=%v, want StateFailed", got.State)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), testMarker) {
		t.Fatalf("marshaled result leaked forbidden text: %s", raw)
	}
}

func TestWaitPRValidation(t *testing.T) {
	fake := cmdrunnertestutil.New(t)
	good := WaitOptions{
		PollInterval:   30 * time.Second,
		Timeout:        10 * time.Minute,
		PerCallTimeout: time.Minute,
		NoCheckGrace:   2 * time.Minute,
	}
	cases := []struct {
		name string
		req  Request
		opts WaitOptions
	}{
		{name: "invalid_req", req: Request{}, opts: good},
		{name: "zero_poll", req: testReq(), opts: WaitOptions{PollInterval: 0, Timeout: time.Minute, PerCallTimeout: time.Second}},
		{name: "zero_timeout", req: testReq(), opts: WaitOptions{PollInterval: time.Second, Timeout: 0, PerCallTimeout: time.Second}},
		{name: "zero_per_call", req: testReq(), opts: WaitOptions{PollInterval: time.Second, Timeout: time.Minute, PerCallTimeout: 0}},
		{name: "negative_grace", req: testReq(), opts: WaitOptions{PollInterval: time.Second, Timeout: time.Minute, PerCallTimeout: time.Second, NoCheckGrace: -time.Second}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := WaitPR(context.Background(), fake, tc.req, tc.opts)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("got err=%v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestWaitPR(t *testing.T) {
	req := testReq()
	args := testGraphQLArgs(req)
	emptyPending := buildGraphQLResponse(t, testHead, testHead, false, nil)
	runningPending := buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
		{
			appSlug:        "github-actions",
			status:         "IN_PROGRESS",
			hasWorkflowRun: true,
			workflowName:   "CI",
		},
	})
	passedOut := buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{passingCISuite()})
	failedOut := buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
		{
			appSlug:        "github-actions",
			status:         "COMPLETED",
			conclusion:     new("FAILURE"),
			hasWorkflowRun: true,
			workflowName:   "CI",
			runs: []testRunSpec{
				{name: "Test", status: "COMPLETED", conclusion: new("FAILURE"), url: testRunURL1},
			},
		},
	})
	headChangedOut := buildGraphQLResponse(t, testAltHead, testAltHead, false, []testSuiteSpec{passingCISuite()})

	instantSleep := func(sleeps *[]time.Duration) func(context.Context, time.Duration) error {
		return func(ctx context.Context, d time.Duration) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			*sleeps = append(*sleeps, d)
			return nil
		}
	}

	t.Run("retries_pending_and_transient_call_failure_until_passed", func(t *testing.T) {
		var sleeps []time.Duration
		fake := cmdrunnertestutil.New(t,
			cmdrunnertestutil.Step{Name: "gh", Args: args, Err: errors.New("transient 502")},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: emptyPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: runningPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: passedOut},
		)
		got, err := WaitPR(context.Background(), fake, req, WaitOptions{
			PollInterval:   30 * time.Second,
			Timeout:        10 * time.Minute,
			PerCallTimeout: time.Minute,
			NoCheckGrace:   2 * time.Minute,
			Sleep:          instantSleep(&sleeps),
		})
		if err != nil {
			t.Fatalf("WaitPR unexpected error: %v", err)
		}
		if got.State != StatePassed {
			t.Fatalf("got %+v, want StatePassed", got)
		}
		if len(sleeps) != 3 {
			t.Fatalf("got %d sleeps (%v), want 3", len(sleeps), sleeps)
		}
	})

	t.Run("stops_immediately_on_failed_without_sleeping", func(t *testing.T) {
		var sleeps []time.Duration
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{Name: "gh", Args: args, Out: failedOut})
		got, err := WaitPR(context.Background(), fake, req, WaitOptions{
			PollInterval:   30 * time.Second,
			Timeout:        10 * time.Minute,
			PerCallTimeout: time.Minute,
			NoCheckGrace:   2 * time.Minute,
			Sleep:          instantSleep(&sleeps),
		})
		if err != nil {
			t.Fatalf("WaitPR unexpected error: %v", err)
		}
		if got.State != StateFailed || len(sleeps) != 0 {
			t.Fatalf("got state=%v sleeps=%d, want StateFailed and 0 sleeps", got.State, len(sleeps))
		}
	})

	permanentCases := []struct {
		name   string
		out    string
		reason UnreadableReason
	}{
		{name: "head_changed", out: headChangedOut, reason: UnreadableHeadChanged},
		{name: "rejected", out: "not-json", reason: UnreadableRejected},
		{name: "limit", out: buildGraphQLResponse(t, testHead, testHead, true, nil), reason: UnreadableLimit},
	}
	for _, tc := range permanentCases {
		t.Run("stops_immediately_on_"+tc.name, func(t *testing.T) {
			var sleeps []time.Duration
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{Name: "gh", Args: args, Out: tc.out})
			got, err := WaitPR(context.Background(), fake, req, WaitOptions{
				PollInterval:   30 * time.Second,
				Timeout:        10 * time.Minute,
				PerCallTimeout: time.Minute,
				NoCheckGrace:   2 * time.Minute,
				Sleep:          instantSleep(&sleeps),
			})
			if err != nil {
				t.Fatalf("WaitPR unexpected error: %v", err)
			}
			if got.State != StateUnreadable || got.Reason != tc.reason || len(sleeps) != 0 {
				t.Fatalf("got %+v sleeps=%d, want unreadable/%v with 0 sleeps", got, len(sleeps), tc.reason)
			}
		})
	}

	t.Run("no_check_grace_expires_when_suites_zero", func(t *testing.T) {
		var sleeps []time.Duration
		fake := cmdrunnertestutil.New(t,
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: emptyPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: emptyPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: emptyPending},
		)
		got, err := WaitPR(context.Background(), fake, req, WaitOptions{
			PollInterval:   30 * time.Second,
			Timeout:        10 * time.Minute,
			PerCallTimeout: time.Minute,
			NoCheckGrace:   time.Minute,
			Sleep:          instantSleep(&sleeps),
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got err=%v, want context.DeadlineExceeded", err)
		}
		if got.State != StatePending || got.Suites != 0 {
			t.Fatalf("got %+v, want pending with Suites=0", got)
		}
		if len(sleeps) != 2 {
			t.Fatalf("got %d sleeps (%v), want 2", len(sleeps), sleeps)
		}
	})

	t.Run("zero_no_check_grace_stops_immediately_when_suites_zero", func(t *testing.T) {
		var sleeps []time.Duration
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{Name: "gh", Args: args, Out: emptyPending})
		got, err := WaitPR(context.Background(), fake, req, WaitOptions{
			PollInterval:   30 * time.Second,
			Timeout:        10 * time.Minute,
			PerCallTimeout: time.Minute,
			NoCheckGrace:   0,
			Sleep:          instantSleep(&sleeps),
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got err=%v, want context.DeadlineExceeded", err)
		}
		if got.State != StatePending || len(sleeps) != 0 {
			t.Fatalf("got %+v sleeps=%d, want pending with 0 sleeps", got, len(sleeps))
		}
	})

	t.Run("suites_positive_waits_past_no_check_grace_until_timeout", func(t *testing.T) {
		var sleeps []time.Duration
		fake := cmdrunnertestutil.New(t,
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: runningPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: runningPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: runningPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: runningPending},
		)
		got, err := WaitPR(context.Background(), fake, req, WaitOptions{
			PollInterval:   30 * time.Second,
			Timeout:        90 * time.Second,
			PerCallTimeout: time.Minute,
			NoCheckGrace:   30 * time.Second,
			Sleep:          instantSleep(&sleeps),
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got err=%v, want context.DeadlineExceeded", err)
		}
		if got.State != StatePending || got.Suites != 1 {
			t.Fatalf("got %+v, want pending with Suites=1", got)
		}
		if len(sleeps) != 3 {
			t.Fatalf("got %d sleeps (%v), want 3", len(sleeps), sleeps)
		}
	})

	t.Run("canceled_context_before_first_call_invokes_nothing", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		fake := cmdrunnertestutil.New(t)
		got, err := WaitPR(ctx, fake, req, WaitOptions{
			PollInterval:   time.Second,
			Timeout:        time.Minute,
			PerCallTimeout: time.Second,
			NoCheckGrace:   time.Second,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got err=%v, want context.Canceled", err)
		}
		if got.State != StateUnreadable || got.Failing == nil {
			t.Fatalf("got %+v, want StateUnreadable with non-nil Failing", got)
		}
	})

	t.Run("default_sleep_succeeds", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t,
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: runningPending},
			cmdrunnertestutil.Step{Name: "gh", Args: args, Out: passedOut},
		)
		got, err := WaitPR(context.Background(), fake, req, WaitOptions{
			PollInterval:   time.Millisecond,
			Timeout:        5 * time.Second,
			PerCallTimeout: time.Second,
			NoCheckGrace:   time.Second,
		})
		if err != nil {
			t.Fatalf("WaitPR unexpected error: %v", err)
		}
		if got.State != StatePassed {
			t.Fatalf("got %+v, want StatePassed", got)
		}
	})
}

func TestEnumSafetyAndZeroResult(t *testing.T) {
	wantStates := []string{"unreadable", "pending", "passed", "failed"}
	gotStates := make([]string, 0, len(States()))
	for _, s := range States() {
		text, err := s.MarshalText()
		if err != nil {
			t.Fatalf("State(%d).MarshalText: %v", s, err)
		}
		gotStates = append(gotStates, string(text))
	}
	if !reflect.DeepEqual(gotStates, wantStates) {
		t.Fatalf("States()=%v, want %v", gotStates, wantStates)
	}
	if _, err := State(99).MarshalText(); err == nil {
		t.Fatal("State(99).MarshalText succeeded, want error")
	}

	wantReasons := []string{"call-failed", "rejected", "limit", "head-changed"}
	gotReasons := make([]string, 0, len(UnreadableReasons()))
	for _, r := range UnreadableReasons() {
		if r == UnreadableNone {
			t.Fatal("UnreadableReasons() must not include UnreadableNone")
		}
		text, err := r.MarshalText()
		if err != nil {
			t.Fatalf("UnreadableReason(%d).MarshalText: %v", r, err)
		}
		gotReasons = append(gotReasons, string(text))
	}
	if !reflect.DeepEqual(gotReasons, wantReasons) {
		t.Fatalf("UnreadableReasons()=%v, want %v", gotReasons, wantReasons)
	}
	if text, err := UnreadableNone.MarshalText(); err != nil || string(text) != "" {
		t.Fatalf("UnreadableNone.MarshalText() = (%q, %v), want (\"\", nil)", text, err)
	}
	if _, err := UnreadableReason(99).MarshalText(); err == nil {
		t.Fatal("UnreadableReason(99).MarshalText succeeded, want error")
	}

	raw, err := json.Marshal(Result{})
	if err != nil {
		t.Fatalf("json.Marshal(Result{}): %v", err)
	}
	if !strings.Contains(string(raw), `"state":"unreadable"`) || !strings.Contains(string(raw), `"reason":""`) {
		t.Fatalf("zero Result marshaled as %s, want state=unreadable and reason=\"\"", raw)
	}
}
