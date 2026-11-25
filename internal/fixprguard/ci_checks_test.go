//go:build test

package fixprguard

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
	"github.com/isseis/yt2column/internal/ghci"
)

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

	// Verify both repo view and GraphQL calls through a local runner because
	// ghci's GraphQL query string is unexported.
	type callRecord struct {
		name string
		args []string
	}
	runWith := func(t *testing.T, ctx context.Context, req CIChecksRequest, responses []struct {
		out string
		err error
	},
	) (CIChecksResult, []callRecord, error) {
		t.Helper()
		var calls []callRecord
		idx := 0
		r := &ciFakeRunner{
			run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				calls = append(calls, callRecord{name: name, args: append([]string(nil), args...)})
				if idx >= len(responses) {
					t.Fatalf("unexpected call %d: %s %v", idx, name, args)
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
		return res, calls, err
	}

	for _, tc := range []struct {
		name      string
		req       CIChecksRequest
		responses []struct {
			out string
			err error
		}
		want CIChecksResult
	}{
		{
			name: "passed_ignoring_external_app",
			req:  CIChecksRequest{PR: mustPR(t), ExpectedHead: rsHead},
			responses: []struct {
				out string
				err error
			}{
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
			responses: []struct {
				out string
				err error
			}{
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
			responses: []struct {
				out string
				err error
			}{
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
			responses: []struct {
				out string
				err error
			}{
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
			name: "repo_call_failed",
			req:  CIChecksRequest{PR: mustPR(t)},
			responses: []struct {
				out string
				err error
			}{
				{err: boom},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableCall,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "repo_rejected",
			req:  CIChecksRequest{PR: mustPR(t)},
			responses: []struct {
				out string
				err error
			}{
				{out: `{"nameWithOwner":"invalid"}`},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableRejected,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "repo_limit",
			req:  CIChecksRequest{PR: mustPR(t)},
			responses: []struct {
				out string
				err error
			}{
				{err: cmdrunner.ErrOutputTooLarge},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableLimit,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "read_pr_call_failed",
			req:  CIChecksRequest{PR: mustPR(t)},
			responses: []struct {
				out string
				err error
			}{
				{out: repositoryView},
				{err: boom},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableCall,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "read_pr_rejected",
			req:  CIChecksRequest{PR: mustPR(t)},
			responses: []struct {
				out string
				err error
			}{
				{out: repositoryView},
				{out: `{not json`},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableRejected,
				Failing:  []ghci.FailingCheck{},
			},
		},
		{
			name: "read_pr_limit",
			req:  CIChecksRequest{PR: mustPR(t)},
			responses: []struct {
				out string
				err error
			}{
				{out: repositoryView},
				{err: cmdrunner.ErrOutputTooLarge},
			},
			want: CIChecksResult{
				Protocol: Protocol,
				State:    ghci.StateUnreadable,
				Reason:   ghci.UnreadableLimit,
				Failing:  []ghci.FailingCheck{},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, calls, err := runWith(t, t.Context(), tc.req, tc.responses)
			if err != nil {
				t.Fatalf("CIChecks = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got = %s\nwant  %s", mustJSON(t, got), mustJSON(t, tc.want))
			}
			if tc.name == "passed_ignoring_external_app" {
				if len(calls) != 2 || calls[0].name != "gh" || !reflect.DeepEqual(calls[0].args, []string{"repo", "view", "--json", "nameWithOwner"}) {
					t.Errorf("unexpected repo call: %+v", calls)
				}
				wantPrefix := []string{"api", "graphql", "-f", "owner=" + testOwner, "-f", "name=" + testRepoName, "-F", "number=" + testPR}
				if len(calls) != 2 || calls[1].name != "gh" || len(calls[1].args) < len(wantPrefix) || !reflect.DeepEqual(calls[1].args[:len(wantPrefix)], wantPrefix) {
					t.Errorf("unexpected graphql call: %+v", calls)
				}
				if !strings.Contains(mustJSON(t, got), `"failing":[]`) {
					t.Errorf("JSON lacks empty failing slice: %s", mustJSON(t, got))
				}
			}
		})
	}

	t.Run("deadline_passed_at_repo", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
		t.Cleanup(cancel)
		got, _, err := runWith(t, ctx, CIChecksRequest{PR: mustPR(t)}, []struct {
			out string
			err error
		}{
			{out: `{"nameWithOwner":"invalid"}`},
		})
		if err != nil {
			t.Fatalf("CIChecks = %v", err)
		}
		want := CIChecksResult{Protocol: Protocol, State: ghci.StateUnreadable, Reason: ghci.UnreadableCall, Failing: []ghci.FailingCheck{}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %s\nwant  %s", mustJSON(t, got), mustJSON(t, want))
		}
	})

	t.Run("deadline_passed_at_read_pr_clears_head", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
		t.Cleanup(cancel)
		got, _, err := runWith(t, ctx, CIChecksRequest{PR: mustPR(t)}, []struct {
			out string
			err error
		}{
			{out: repositoryView},
			{out: `{not json`},
		})
		if err != nil {
			t.Fatalf("CIChecks = %v", err)
		}
		want := CIChecksResult{Protocol: Protocol, State: ghci.StateUnreadable, Reason: ghci.UnreadableCall, Failing: []ghci.FailingCheck{}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %s\nwant  %s", mustJSON(t, got), mustJSON(t, want))
		}
	})

	t.Run("deadline_passed_preserves_head_changed", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
		t.Cleanup(cancel)
		got, _, err := runWith(t, ctx, CIChecksRequest{PR: mustPR(t), ExpectedHead: rsHead}, []struct {
			out string
			err error
		}{
			{out: repositoryView},
			{out: ciGraphQLResponse(rsSecond, "COMPLETED", "SUCCESS", "COMPLETED", "SUCCESS")},
		})
		if err != nil {
			t.Fatalf("CIChecks = %v", err)
		}
		want := CIChecksResult{Protocol: Protocol, State: ghci.StateUnreadable, Reason: ghci.UnreadableHeadChanged, Head: rsSecond, Failing: []ghci.FailingCheck{}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %s\nwant  %s", mustJSON(t, got), mustJSON(t, want))
		}
	})

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
