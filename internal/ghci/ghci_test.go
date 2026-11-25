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
	"testing/synctest"
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

// The response bodies gh 2.96.0 printed to standard output, exiting with
// status 1, for a pull request number that does not resolve and for a query
// that names a field the schema lacks.
const (
	graphQLNotFoundBody       = `{"data":{"repository":{"pullRequest":null}},"errors":[{"type":"NOT_FOUND","path":["repository","pullRequest"],"locations":[{"line":1,"column":86}],"message":"Could not resolve to a PullRequest with the number of 999999."}]}`
	graphQLUndefinedFieldBody = `{"errors":[{"path":["query","viewer","noSuchField"],"extensions":{"code":"undefinedField","typeName":"User","fieldName":"noSuchField"},"locations":[{"line":1,"column":14}],"message":"Field 'noSuchField' doesn't exist on type 'User'"}]}`
)

// ghGraphQLExit is the failure cmdrunner returns when gh exits with code and
// has written stdout.
func ghGraphQLExit(code int, stdout string) error {
	return &cmdrunner.ExitError{Code: code, Stdout: []byte(stdout), Stderr: []byte("gh: " + testMarker)}
}

func buildCommitNode(commitOID string, suitesNextPage bool, suites []testSuiteSpec) map[string]any {
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
	return map[string]any{
		"commit": map[string]any{
			"oid": commitOID,
			"checkSuites": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": suitesNextPage},
				"nodes":    suiteNodes,
			},
		},
	}
}

func buildGraphQLResponseWithCommits(t *testing.T, headRefOID string, commitNodes []any) string {
	t.Helper()
	payload := map[string]any{
		"data": map[string]any{
			"repository": map[string]any{
				"pullRequest": map[string]any{
					"headRefOid": headRefOID,
					"commits": map[string]any{
						"nodes": commitNodes,
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

func buildGraphQLResponse(t *testing.T, headRefOID, commitOID string, suitesNextPage bool, suites []testSuiteSpec) string {
	t.Helper()
	return buildGraphQLResponseWithCommits(t, headRefOID, []any{
		buildCommitNode(commitOID, suitesNextPage, suites),
	})
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

	errorCases := []struct {
		name       string
		out        string
		err        error
		wantReason UnreadableReason
	}{
		{
			name:       "gh_call_failure",
			err:        fmt.Errorf("gh failed: %s", testMarker),
			wantReason: UnreadableCall,
		},
		{
			name:       "output_too_large_error",
			err:        fmt.Errorf("wrapped %s: %w", testMarker, cmdrunner.ErrOutputTooLarge),
			wantReason: UnreadableLimit,
		},
		{
			name:       "oversized_output",
			out:        testMarker + strings.Repeat(" ", maxResponseBytes),
			wantReason: UnreadableLimit,
		},
		{name: "rejected_not_json", out: "not-json " + testMarker, wantReason: UnreadableRejected},
		{name: "rejected_duplicate_key", out: `{"nameWithOwner":"a/b","nameWithOwner":"c/d"}`, wantReason: UnreadableRejected},
		{name: "rejected_trailing_json", out: `{"nameWithOwner":"a/b"} {}`, wantReason: UnreadableRejected},
		// Place invalid UTF-8 and unpaired surrogate in an unconsumed member so
		// strictjson whole-document validation is isolated from validRepoName.
		{name: "rejected_invalid_utf8", out: "{\"nameWithOwner\":\"octo/repo\",\"extra\":\"x\xff\"}", wantReason: UnreadableRejected},
		{name: "rejected_unpaired_surrogate", out: `{"nameWithOwner":"octo/repo","extra":"\uD800"}`, wantReason: UnreadableRejected},
		{name: "rejected_missing_member", out: `{}`, wantReason: UnreadableRejected},
		{name: "rejected_empty_string", out: `{"nameWithOwner":""}`, wantReason: UnreadableRejected},
		{name: "rejected_no_slash", out: `{"nameWithOwner":"octo-org"}`, wantReason: UnreadableRejected},
		{name: "rejected_extra_slash", out: `{"nameWithOwner":"octo-org/repo/extra"}`, wantReason: UnreadableRejected},
		{name: "rejected_bad_owner_leading_hyphen", out: `{"nameWithOwner":"-octo/repo"}`, wantReason: UnreadableRejected},
		{name: "rejected_bad_owner_underscore", out: `{"nameWithOwner":"octo_org/repo"}`, wantReason: UnreadableRejected},
		{name: "rejected_bad_owner_too_long", out: `{"nameWithOwner":"` + strings.Repeat("a", maxLoginLength+1) + `/repo"}`, wantReason: UnreadableRejected},
		{name: "rejected_bad_repo_dot", out: `{"nameWithOwner":"octo/."}`, wantReason: UnreadableRejected},
		{name: "rejected_bad_repo_dotdot", out: `{"nameWithOwner":"octo/.."}`, wantReason: UnreadableRejected},
		{name: "rejected_bad_repo_too_long", out: `{"nameWithOwner":"octo/` + strings.Repeat("a", maxRepoNameLength+1) + `"}`, wantReason: UnreadableRejected},
		{name: "rejected_bad_repo_space", out: `{"nameWithOwner":"octo/my repo"}`, wantReason: UnreadableRejected},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "gh",
				Args: []string{"repo", "view", "--json", "nameWithOwner"},
				Out:  tc.out,
				Err:  tc.err,
			})
			_, _, err := ResolveRepo(context.Background(), fake)
			repoErr, ok := errors.AsType[*RepoError](err)
			if !ok || repoErr.Reason != tc.wantReason {
				t.Fatalf("got err=%v, want *RepoError(%v)", err, tc.wantReason)
			}
			// RepoError.Error() is contractually a fixed non-empty diagnostic
			// that must never echo untrusted command output or arguments.
			if repoErr.Error() == "" || strings.Contains(repoErr.Error(), testMarker) {
				t.Fatalf("unexpected RepoError message: %q", repoErr.Error())
			}
		})
	}

	t.Run("repo_error_default_reason_message", func(t *testing.T) {
		// Observable contract: unknown RepoError reasons still format a non-empty fallback message.
		if msg := (&RepoError{Reason: UnreadableReason(99)}).Error(); msg == "" {
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

	manySuites := make([]testSuiteSpec, maxCheckSuites+1)
	for i := range manySuites {
		manySuites[i] = passingCISuite()
	}

	manyRuns := make([]testRunSpec, maxCheckRuns+1)
	for i := range manyRuns {
		manyRuns[i] = testRunSpec{
			name:       fmt.Sprintf("job-%d", i),
			status:     "COMPLETED",
			conclusion: new("SUCCESS"),
			url:        testRunURL1,
		}
	}
	overCapRunsSuite := passingCISuite()
	overCapRunsSuite.runs = manyRuns

	cases := []struct {
		name string
		req  Request
		out  string
		err  error
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
			name: "multiple_distinct_workflows_not_deduplicated",
			req:  req,
			out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
				passingCISuite(),
				{
					appSlug:        "github-actions",
					status:         "COMPLETED",
					conclusion:     new("FAILURE"),
					createdAt:      "2026-11-24T10:05:00Z",
					hasWorkflowRun: true,
					workflowName:   "Docs",
					runs: []testRunSpec{
						{name: "Check docs", status: "COMPLETED", conclusion: new("FAILURE"), url: testRunURL2},
					},
				},
			}),
			want: Result{
				State:  StateFailed,
				Head:   testHead,
				Suites: 2,
				Checks: 3,
				Failing: []FailingCheck{
					{Name: "Check docs", Conclusion: "FAILURE", URL: testRunURL2},
				},
			},
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
					{Name: "github-actions-check-suite", Conclusion: "STARTUP_FAILURE", URL: ""},
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
		{
			name: "gh_call_error_returns_unreadable_call",
			req:  req,
			err:  fmt.Errorf("network failure: %s", testMarker),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_not_found_exit_is_rejected",
			req:  req,
			err:  ghGraphQLExit(1, graphQLNotFoundBody),
			want: Result{State: StateUnreadable, Reason: UnreadableRejected, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_forbidden_exit_is_rejected",
			req:  req,
			err:  ghGraphQLExit(1, `{"data":null,"errors":[{"type":"FORBIDDEN","message":"`+testMarker+`"}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableRejected, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_query_validation_exit_is_rejected",
			req:  req,
			err:  ghGraphQLExit(1, graphQLUndefinedFieldBody),
			want: Result{State: StateUnreadable, Reason: UnreadableRejected, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_rate_limited_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, `{"errors":[{"type":"RATE_LIMITED","message":"`+testMarker+`"}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_untyped_timeout_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, `{"data":null,"errors":[{"message":"`+testMarker+`"}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_mixed_permanent_and_transient_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, `{"errors":[{"type":"NOT_FOUND","message":"a"},{"type":"RATE_LIMITED","message":"b"}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_type_takes_precedence_over_extensions_code",
			req:  req,
			err:  ghGraphQLExit(1, `{"errors":[{"type":"RATE_LIMITED","extensions":{"code":"x"},"message":"a"}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_null_type_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, `{"errors":[{"type":null,"extensions":{"code":"undefinedField"}}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_empty_extensions_code_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, `{"errors":[{"extensions":{"code":""}}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "graphql_empty_errors_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, `{"data":null,"errors":[]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "non_json_stdout_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, "<html>"+testMarker+"</html>"),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "duplicate_errors_member_exit_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, `{"errors":[{"type":"NOT_FOUND"}],"errors":[{"type":"NOT_FOUND"}]}`),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "not_found_body_with_other_exit_code_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(2, graphQLNotFoundBody),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "oversized_not_found_stdout_is_call_failed",
			req:  req,
			err:  ghGraphQLExit(1, graphQLNotFoundBody+strings.Repeat(" ", maxResponseBytes)),
			want: Result{State: StateUnreadable, Reason: UnreadableCall, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "output_too_large_error_returns_unreadable_limit",
			req:  req,
			err:  fmt.Errorf("wrapped %s: %w", testMarker, cmdrunner.ErrOutputTooLarge),
			want: Result{State: StateUnreadable, Reason: UnreadableLimit, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "oversized_response_returns_unreadable_limit",
			req:  req,
			out:  testMarker + strings.Repeat(" ", maxResponseBytes),
			want: Result{State: StateUnreadable, Reason: UnreadableLimit, Head: "", Failing: []FailingCheck{}},
		},
		{
			name: "too_many_suites_returns_limit",
			req:  req,
			out:  buildGraphQLResponse(t, testHead, testHead, false, manySuites),
			want: Result{State: StateUnreadable, Reason: UnreadableLimit, Head: testHead, Failing: []FailingCheck{}},
		},
		{
			name: "too_many_github_actions_runs_returns_limit",
			req:  req,
			out:  buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{overCapRunsSuite}),
			want: Result{State: StateUnreadable, Reason: UnreadableLimit, Head: testHead, Failing: []FailingCheck{}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
				Name: "gh",
				Args: testGraphQLArgs(tc.req),
				Out:  tc.out,
				Err:  tc.err,
			})
			got, err := ReadPR(context.Background(), fake, tc.req)
			if err != nil {
				t.Fatalf("ReadPR unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			raw, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if strings.Contains(string(raw), testMarker) {
				t.Fatalf("marshaled result leaked marker: %s", raw)
			}
		})
	}
}

func TestReadPRStrictJSONRejections(t *testing.T) {
	req := testReq()
	args := testGraphQLArgs(req)
	validJSON := buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{passingCISuite()})

	responseWithCreatedAt := func(ts string) string {
		return buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), createdAt: ts, hasWorkflowRun: true, workflowName: "CI"},
		})
	}
	responseWithRunURL := func(rawURL string) string {
		return buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: new("SUCCESS"), url: rawURL}},
			},
		})
	}

	cases := []struct {
		name string
		out  string
	}{
		// Invalid UTF-8 inside a JSON string isolates strictjson UTF-8 validation
		// because encoding/json silently replaces raw invalid bytes with U+FFFD.
		{name: "invalid_utf8", out: strings.Replace(validJSON, `"Test"`, "\"Te\xffst\"", 1)},
		{name: "unpaired_surrogate", out: strings.Replace(validJSON, `"Test"`, `"\uD800"`, 1)},
		{name: "trailing_json", out: validJSON + " {}"},
		{name: "duplicate_headRefOid", out: strings.Replace(validJSON, `"headRefOid":"`+testHead+`"`, `"headRefOid":"`+testHead+`","headRefOid":"`+testHead+`"`, 1)},
		{name: "graphql_errors_non_empty", out: strings.Replace(validJSON, `{"data":`, `{"errors":[{"message":"`+testMarker+`"}],"data":`, 1)},
		{name: "graphql_errors_not_array", out: strings.Replace(validJSON, `{"data":`, `{"errors":"boom","data":`, 1)},
		{name: "null_pull_request", out: `{"data":{"repository":{"pullRequest":null}}}`},
		{name: "missing_suite_app_member", out: `{"data":{"repository":{"pullRequest":{"headRefOid":"` + testHead + `","commits":{"nodes":[{"commit":{"oid":"` + testHead + `","checkSuites":{"pageInfo":{"hasNextPage":false},"nodes":[{"status":"COMPLETED","conclusion":"SUCCESS","createdAt":"2026-11-24T10:00:00Z","workflowRun":null,"checkRuns":{"pageInfo":{"hasNextPage":false},"nodes":[]}}]}}}]}}}}}`},
		{name: "empty_commits_nodes", out: `{"data":{"repository":{"pullRequest":{"headRefOid":"` + testHead + `","commits":{"nodes":[]}}}}}`},
		{name: "two_commits_nodes", out: buildGraphQLResponseWithCommits(t, testHead, []any{
			buildCommitNode(testHead, false, []testSuiteSpec{passingCISuite()}),
			buildCommitNode(testHead, false, []testSuiteSpec{passingCISuite()}),
		})},
		{name: "uppercase_headRefOid", out: strings.ReplaceAll(validJSON, testHead, strings.ToUpper(testHead))},
		{name: "completed_suite_null_conclusion", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{appSlug: "github-actions", status: "COMPLETED", conclusion: nil, hasWorkflowRun: true, workflowName: "CI"},
		})},
		{name: "completed_suite_empty_conclusion", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{appSlug: "github-actions", status: "COMPLETED", conclusion: new(""), hasWorkflowRun: true, workflowName: "CI"},
		})},
		{name: "completed_run_null_conclusion", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: nil, url: testRunURL1}},
			},
		})},
		{name: "completed_run_empty_conclusion", out: buildGraphQLResponse(t, testHead, testHead, false, []testSuiteSpec{
			{
				appSlug: "github-actions", status: "COMPLETED", conclusion: new("SUCCESS"), hasWorkflowRun: true, workflowName: "CI",
				runs: []testRunSpec{{name: "Test", status: "COMPLETED", conclusion: new(""), url: testRunURL1}},
			},
		})},
		{name: "invalid_createdAt", out: responseWithCreatedAt("not-a-date")},
		{name: "createdAt_comma_fraction", out: responseWithCreatedAt("2026-11-24T10:00:00,5Z")},
		{name: "createdAt_out_of_range_offset", out: responseWithCreatedAt("2026-11-24T10:00:00+99:00")},
		{name: "non_github_url", out: responseWithRunURL("https://evil.example/runs/1")},
		{name: "github_prefix_spoof_host", out: responseWithRunURL("https://github.com.evil.example/runs/1")},
		{name: "github_url_empty_path", out: responseWithRunURL("https://github.com/")},
		{name: "github_url_query_only_path", out: responseWithRunURL("https://github.com/?x=1")},
		{name: "github_url_fragment_only_path", out: responseWithRunURL("https://github.com/#frag")},
		{name: "github_url_bad_percent_escape", out: responseWithRunURL("https://github.com/%zz")},
		{name: "github_url_with_newline", out: responseWithRunURL("https://github.com/octo/repo/runs/1\ninjected")},
		{name: "github_url_with_space", out: responseWithRunURL("https://github.com/octo/repo/runs/1 bad")},
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

	stepOut := func(out string) cmdrunnertestutil.Step {
		return cmdrunnertestutil.Step{Name: "gh", Args: args, Out: out}
	}
	stepErr := func(msg string) cmdrunnertestutil.Step {
		return cmdrunnertestutil.Step{Name: "gh", Args: args, Err: errors.New(msg)}
	}

	immediateStopCases := []struct {
		name       string
		out        string
		wantState  State
		wantReason UnreadableReason
	}{
		{name: "failed", out: failedOut, wantState: StateFailed, wantReason: UnreadableNone},
		{name: "head_changed", out: headChangedOut, wantState: StateUnreadable, wantReason: UnreadableHeadChanged},
		{name: "rejected", out: "not-json", wantState: StateUnreadable, wantReason: UnreadableRejected},
		{name: "limit", out: buildGraphQLResponse(t, testHead, testHead, true, nil), wantState: StateUnreadable, wantReason: UnreadableLimit},
	}
	for _, tc := range immediateStopCases {
		t.Run("stops_immediately_on_"+tc.name, func(t *testing.T) {
			fake := cmdrunnertestutil.New(t, stepOut(tc.out))
			got, err := WaitPR(context.Background(), fake, req, WaitOptions{
				PollInterval:   30 * time.Second,
				Timeout:        10 * time.Minute,
				PerCallTimeout: time.Minute,
				NoCheckGrace:   2 * time.Minute,
				Sleep: func(context.Context, time.Duration) error {
					t.Fatal("Sleep must not be called on terminal outcome")
					return nil
				},
			})
			if err != nil {
				t.Fatalf("WaitPR unexpected error: %v", err)
			}
			if got.State != tc.wantState || got.Reason != tc.wantReason {
				t.Fatalf("got %+v, want state=%v reason=%v", got, tc.wantState, tc.wantReason)
			}
		})
	}

	pollingCases := []struct {
		name       string
		steps      []cmdrunnertestutil.Step
		opts       WaitOptions
		wantErr    error
		wantState  State
		wantReason UnreadableReason
		wantSuites int
		wantSleeps int
	}{
		{
			name:       "retries_pending_and_transient_call_failure_until_passed",
			steps:      []cmdrunnertestutil.Step{stepErr("transient 502"), stepOut(emptyPending), stepOut(runningPending), stepOut(passedOut)},
			opts:       WaitOptions{PollInterval: 30 * time.Second, Timeout: 10 * time.Minute, PerCallTimeout: time.Minute, NoCheckGrace: 2 * time.Minute},
			wantState:  StatePassed,
			wantSuites: 1,
			wantSleeps: 3,
		},
		{
			name:       "no_check_grace_expires_when_suites_zero",
			steps:      []cmdrunnertestutil.Step{stepOut(emptyPending), stepOut(emptyPending), stepOut(emptyPending)},
			opts:       WaitOptions{PollInterval: 30 * time.Second, Timeout: 10 * time.Minute, PerCallTimeout: time.Minute, NoCheckGrace: time.Minute},
			wantErr:    context.DeadlineExceeded,
			wantState:  StatePending,
			wantSuites: 0,
			wantSleeps: 2,
		},
		{
			name:       "zero_no_check_grace_stops_immediately_when_suites_zero",
			steps:      []cmdrunnertestutil.Step{stepOut(emptyPending)},
			opts:       WaitOptions{PollInterval: 30 * time.Second, Timeout: 10 * time.Minute, PerCallTimeout: time.Minute, NoCheckGrace: 0},
			wantErr:    context.DeadlineExceeded,
			wantState:  StatePending,
			wantSuites: 0,
			wantSleeps: 0,
		},
		{
			name:       "suites_positive_waits_past_no_check_grace_until_timeout",
			steps:      []cmdrunnertestutil.Step{stepOut(runningPending), stepOut(runningPending), stepOut(runningPending), stepOut(runningPending)},
			opts:       WaitOptions{PollInterval: 30 * time.Second, Timeout: 90 * time.Second, PerCallTimeout: time.Minute, NoCheckGrace: 30 * time.Second},
			wantErr:    context.DeadlineExceeded,
			wantState:  StatePending,
			wantSuites: 1,
			wantSleeps: 3,
		},
		{
			name:       "suites_appearing_clears_no_check_grace",
			steps:      []cmdrunnertestutil.Step{stepOut(emptyPending), stepOut(runningPending), stepOut(runningPending), stepOut(passedOut)},
			opts:       WaitOptions{PollInterval: 30 * time.Second, Timeout: 5 * time.Minute, PerCallTimeout: time.Minute, NoCheckGrace: 30 * time.Second},
			wantState:  StatePassed,
			wantSuites: 1,
			wantSleeps: 3,
		},
		{
			name:       "transient_call_failure_after_zero_suites_respects_no_check_grace",
			steps:      []cmdrunnertestutil.Step{stepOut(emptyPending), stepErr("transient 503")},
			opts:       WaitOptions{PollInterval: 30 * time.Second, Timeout: 10 * time.Minute, PerCallTimeout: time.Minute, NoCheckGrace: 30 * time.Second},
			wantErr:    context.DeadlineExceeded,
			wantState:  StateUnreadable,
			wantReason: UnreadableCall,
			wantSleeps: 1,
		},
		{
			name:       "persistent_call_failure_exhausts_timeout",
			steps:      []cmdrunnertestutil.Step{stepErr("transient 502"), stepErr("transient 502"), stepErr("transient 502")},
			opts:       WaitOptions{PollInterval: 30 * time.Second, Timeout: time.Minute, PerCallTimeout: time.Minute, NoCheckGrace: 30 * time.Second},
			wantErr:    context.DeadlineExceeded,
			wantState:  StateUnreadable,
			wantReason: UnreadableCall,
			wantSleeps: 2,
		},
	}
	for _, tc := range pollingCases {
		t.Run(tc.name, func(t *testing.T) {
			var sleeps []time.Duration
			opts := tc.opts
			opts.Sleep = instantSleep(&sleeps)
			fake := cmdrunnertestutil.New(t, tc.steps...)
			got, err := WaitPR(context.Background(), fake, req, opts)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("WaitPR unexpected error: %v", err)
				}
			} else if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got err=%v, want %v", err, tc.wantErr)
			}
			if got.State != tc.wantState || got.Reason != tc.wantReason || got.Suites != tc.wantSuites {
				t.Fatalf("got %+v, want state=%v reason=%v suites=%d", got, tc.wantState, tc.wantReason, tc.wantSuites)
			}
			if len(sleeps) != tc.wantSleeps {
				t.Fatalf("got %d sleeps (%v), want %d", len(sleeps), sleeps, tc.wantSleeps)
			}
		})
	}

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
		fake := cmdrunnertestutil.New(t, stepOut(runningPending), stepOut(passedOut))
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

	t.Run("per_call_timeout_bounds_runner_context_and_retries", func(t *testing.T) {
		var sleeps []time.Duration
		calls := 0
		perCall := 2 * time.Second
		runner := runnerFunc(func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
			calls++
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected per-call deadline on runner context")
			}
			if rem := time.Until(deadline); rem <= 0 || rem > perCall+500*time.Millisecond {
				t.Fatalf("unexpected per-call deadline remaining: %v (want <= %v)", rem, perCall)
			}
			if calls == 1 {
				return nil, context.DeadlineExceeded
			}
			return []byte(passedOut), nil
		})
		got, err := WaitPR(context.Background(), runner, req, WaitOptions{
			PollInterval:   30 * time.Second,
			Timeout:        10 * time.Minute,
			PerCallTimeout: perCall,
			NoCheckGrace:   time.Minute,
			Sleep:          instantSleep(&sleeps),
		})
		if err != nil {
			t.Fatalf("WaitPR unexpected error: %v", err)
		}
		if got.State != StatePassed || calls != 2 || len(sleeps) != 1 {
			t.Fatalf("got %+v calls=%d sleeps=%d, want StatePassed calls=2 sleeps=1", got, calls, len(sleeps))
		}
	})

	t.Run("canceled_context_during_read_stops_immediately", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		runner := runnerFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			cancel()
			return nil, context.Canceled
		})
		got, err := WaitPR(ctx, runner, req, WaitOptions{
			PollInterval:   30 * time.Second,
			Timeout:        10 * time.Minute,
			PerCallTimeout: time.Minute,
			NoCheckGrace:   time.Minute,
			Sleep: func(context.Context, time.Duration) error {
				t.Fatal("Sleep must not be called after context cancellation during read")
				return nil
			},
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got err=%v, want context.Canceled", err)
		}
		if got.State != StateUnreadable || got.Reason != UnreadableCall {
			t.Fatalf("got %+v, want unreadable/call-failed", got)
		}
	})

	t.Run("canceled_context_during_default_sleep_stops_promptly", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			runner := runnerFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				calls++
				go func() {
					time.Sleep(5 * time.Millisecond)
					cancel()
				}()
				return []byte(runningPending), nil
			})
			start := time.Now()
			got, err := WaitPR(ctx, runner, req, WaitOptions{
				PollInterval:   time.Minute,
				Timeout:        5 * time.Minute,
				PerCallTimeout: time.Second,
				NoCheckGrace:   time.Second,
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got err=%v, want context.Canceled", err)
			}
			if elapsed := time.Since(start); elapsed >= time.Minute {
				t.Fatalf("default sleep did not stop before poll interval: elapsed %v", elapsed)
			}
			if got.State != StatePending || calls != 1 {
				t.Fatalf("got %+v calls=%d, want StatePending calls=1", got, calls)
			}
		})
	})
}

func TestReadPRGraphQLErrorExitAfterCancelIsCallFailed(t *testing.T) {
	req := testReq()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
		Name: "gh",
		Args: testGraphQLArgs(req),
		Err:  errors.Join(context.Canceled, ghGraphQLExit(1, graphQLNotFoundBody)),
	})
	got, err := ReadPR(ctx, fake, req)
	if err != nil {
		t.Fatalf("ReadPR unexpected error: %v", err)
	}
	if got.State != StateUnreadable || got.Reason != UnreadableCall {
		t.Fatalf("got %+v, want unreadable/call-failed", got)
	}
}

func TestWaitPRStopsImmediatelyOnPermanentGraphQLError(t *testing.T) {
	req := testReq()
	fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
		Name: "gh",
		Args: testGraphQLArgs(req),
		Err:  ghGraphQLExit(1, graphQLNotFoundBody),
	})
	got, err := WaitPR(context.Background(), fake, req, WaitOptions{
		PollInterval:   30 * time.Second,
		Timeout:        10 * time.Minute,
		PerCallTimeout: time.Minute,
		NoCheckGrace:   2 * time.Minute,
		Sleep: func(context.Context, time.Duration) error {
			t.Fatal("Sleep must not be called on a permanent GraphQL error")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("WaitPR unexpected error: %v", err)
	}
	if got.State != StateUnreadable || got.Reason != UnreadableRejected {
		t.Fatalf("got %+v, want unreadable/rejected", got)
	}
}

type runnerFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, name, args...)
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
