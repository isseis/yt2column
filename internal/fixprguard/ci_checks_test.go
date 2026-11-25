//go:build test

package fixprguard

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
	"github.com/isseis/yt2column/internal/ghci"
)

type ciResponse struct {
	out string
	err error
}

func ciGraphQLResponse(head, suiteStatus, suiteConclusion, runStatus, runConclusion string) string {
	suiteConc := "null"
	if suiteConclusion != "" {
		suiteConc = `"` + suiteConclusion + `"`
	}
	runConc := "null"
	if runConclusion != "" {
		runConc = `"` + runConclusion + `"`
	}
	return `{"data":{"repository":{"pullRequest":{"headRefOid":"` + head + `","commits":{"nodes":[{"commit":{"oid":"` + head + `","checkSuites":{"pageInfo":{"hasNextPage":false},"nodes":[` +
		`{"app":{"slug":"sourcery-ai"},"status":"COMPLETED","conclusion":"FAILURE","createdAt":"2026-11-20T10:00:00Z","workflowRun":null,"checkRuns":{"pageInfo":{"hasNextPage":true},"nodes":[]}},` +
		`{"app":{"slug":"github-actions"},"status":"` + suiteStatus + `","conclusion":` + suiteConc + `,"createdAt":"2026-11-20T10:01:00Z","workflowRun":{"workflow":{"name":"CI"}},"checkRuns":{"pageInfo":{"hasNextPage":false},"nodes":[{"name":"Test","status":"` + runStatus + `","conclusion":` + runConc + `,"url":"https://github.com/isseis/yt2column/actions/runs/1/job/1"}]}}` +
		`]}}}]}}}}}`
}

func TestCIChecks(t *testing.T) {
	boom := errors.New("boom")

	// Validate both gh calls through a local runner because ghci's GraphQL
	// query string is unexported.
	runWith := func(t *testing.T, ctx context.Context, req CIChecksRequest, responses []ciResponse) (CIChecksResult, error) {
		t.Helper()
		idx := 0
		r := &ciFakeRunner{
			run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				if idx >= len(responses) {
					t.Fatalf("unexpected call %d: %s %v", idx, name, args)
				}
				if name != ghCommand {
					t.Fatalf("call %d command = %q, want %q", idx, name, ghCommand)
				}
				switch idx {
				case 0:
					if !slices.Equal(args, []string{"repo", "view", "--json", "nameWithOwner"}) {
						t.Errorf("repo view args = %v", args)
					}
				case 1:
					for _, wantArg := range []string{"owner=" + testOwner, "name=" + testRepoName, "number=" + testPR} {
						if !slices.Contains(args, wantArg) {
							t.Errorf("graphql args %v lack %q", args, wantArg)
						}
					}
				}
				resp := responses[idx]
				idx++
				return []byte(resp.out), resp.err
			},
		}
		g, err := NewGitHubWithoutReviewer(r)
		if err != nil {
			t.Fatalf("NewGitHubWithoutReviewer: %v", err)
		}
		res, err := g.CIChecks(ctx, req)
		if idx != len(responses) {
			t.Errorf("ran %d calls, want %d", idx, len(responses))
		}
		return res, err
	}

	// Parse headRefOid successfully, then trigger a post-head limit error so
	// ReadPR returns StateUnreadable with Head == rsHead; CIChecks must
	// rewrite Reason to UnreadableCall and clear Head to "".
	postHeadLimit := `{"data":{"repository":{"pullRequest":{"headRefOid":"` + rsHead + `","commits":{"nodes":[{"commit":{"oid":"` + rsHead + `","checkSuites":{"pageInfo":{"hasNextPage":true},"nodes":[]}}}]}}}}}`

	for _, tc := range []struct {
		name      string
		req       CIChecksRequest
		responses []ciResponse
		expired   bool
		want      CIChecksResult
	}{
		{
			name: "passed_ignoring_external_app",
			req:  CIChecksRequest{PR: mustPR(t), ExpectedHead: rsHead},
			responses: []ciResponse{
				{out: repositoryView},
				{out: ciGraphQLResponse(rsHead, "COMPLETED", "SUCCESS", "COMPLETED", "SUCCESS")},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StatePassed,
				Reason:   ghci.UnreadableNone,
				Head:     rsHead,
				Suites:   1,
				Checks:   1,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "pending",
			req:  CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{
				{out: repositoryView},
				{out: ciGraphQLResponse(rsHead, "IN_PROGRESS", "", "IN_PROGRESS", "")},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StatePending,
				Reason:   ghci.UnreadableNone,
				Head:     rsHead,
				Suites:   1,
				Checks:   1,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "failed_carries_failing_check",
			req:  CIChecksRequest{PR: mustPR(t), ExpectedHead: rsHead},
			responses: []ciResponse{
				{out: repositoryView},
				{out: ciGraphQLResponse(rsHead, "COMPLETED", "FAILURE", "COMPLETED", "FAILURE")},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateFailed,
				Reason:   ghci.UnreadableNone,
				Head:     rsHead,
				Suites:   1,
				Checks:   1,
				Failing: []ghci.FailingCheck{
					{Name: "Test", Conclusion: "FAILURE", URL: "https://github.com/isseis/yt2column/actions/runs/1/job/1"},
				},
			},
		},
		{
			name: "head_changed",
			req:  CIChecksRequest{PR: mustPR(t), ExpectedHead: rsHead},
			responses: []ciResponse{
				{out: repositoryView},
				{out: ciGraphQLResponse(rsSecond, "COMPLETED", "SUCCESS", "COMPLETED", "SUCCESS")},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableHeadChanged,
				Head:     rsSecond,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "repo_call_failed",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{err: boom}},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableCall,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "repo_rejected",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{out: `{"nameWithOwner":"invalid"}`}},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableRejected,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "repo_limit",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{err: cmdrunner.ErrOutputTooLarge}},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableLimit,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "read_pr_call_failed",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{out: repositoryView}, {err: boom}},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableCall,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "read_pr_rejected",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{out: repositoryView}, {out: `{not json`}},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableRejected,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "read_pr_limit",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{out: repositoryView}, {err: cmdrunner.ErrOutputTooLarge}},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableLimit,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "deadline_passed_at_repo",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{out: `{"nameWithOwner":"invalid"}`}},
			expired:   true,
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableCall,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name:      "deadline_passed_at_read_pr_clears_head",
			req:       CIChecksRequest{PR: mustPR(t)},
			responses: []ciResponse{{out: repositoryView}, {out: postHeadLimit}},
			expired:   true,
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableCall,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "deadline_passed_preserves_head_changed",
			req:  CIChecksRequest{PR: mustPR(t), ExpectedHead: rsHead},
			responses: []ciResponse{
				{out: repositoryView},
				{out: ciGraphQLResponse(rsSecond, "COMPLETED", "SUCCESS", "COMPLETED", "SUCCESS")},
			},
			expired: true,
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableHeadChanged,
				Head:     rsSecond,
				Failing:  []ghci.FailingCheck{},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if tc.expired {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				t.Cleanup(cancel)
			}
			got, err := runWith(t, ctx, tc.req, tc.responses)
			if err != nil {
				t.Fatalf("CIChecks = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got = %s\nwant  %s", mustJSON(t, got), mustJSON(t, tc.want))
			}
			if len(tc.want.Failing) == 0 && !strings.Contains(mustJSON(t, got), `"failing":[]`) {
				t.Errorf("JSON lacks empty failing slice: %s", mustJSON(t, got))
			}
		})
	}

	for name, req := range map[string]CIChecksRequest{
		"zero_pr":        {},
		"short_head":     {PR: mustPR(t), ExpectedHead: strings.Repeat("a", 39)},
		"uppercase_head": {PR: mustPR(t), ExpectedHead: strings.Repeat("A", 40)},
		"non_hex_head":   {PR: mustPR(t), ExpectedHead: strings.Repeat("g", 40)},
	} {
		t.Run("invalid/"+name, func(t *testing.T) {
			g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := g.CIChecks(t.Context(), req); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("CIChecks = %v, want ErrInvalidInput", err)
			}
		})
	}

	t.Run("zero_result_fails_closed", func(t *testing.T) {
		if got := mustJSON(t, CIChecksResult{}); !strings.Contains(got, `"state":"unreadable"`) {
			t.Errorf("zero CIChecksResult JSON = %s, want state unreadable", got)
		}
	})
}

type ciFakeRunner struct {
	run func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func (f *ciFakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f.run(ctx, name, args...)
}

func (f *ciFakeRunner) RunInput(ctx context.Context, _ []byte, name string, args ...string) ([]byte, error) {
	return f.run(ctx, name, args...)
}
