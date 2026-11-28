//go:build test

package fixprguard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// at is a time on the day of the tests.
func at(clock string) string { return "2026-11-20T" + clock + "Z" }

// reviewStateCase is one row of TestReviewState: the calls and the state they
// must give.
type reviewStateCase struct {
	name string
	in   rsInput
	want PRReviewState
}

// requestAt is a request comment created at the given clock time.
func requestAt(id int64, clock string) rsComment { return rsRequestComment(id, at(clock)) }

// defaultComments is one request created at rsCreated.
func defaultComments() []rsComment { return []rsComment{rsRequestComment(1, rsCreated)} }

// withResponse is the want edit for a response of the given run.
func withResponse(kind RunKind, run rsRun, commit string) func(*PRReviewState) {
	return func(s *PRReviewState) {
		s.Response = &ReviewResponse{
			Kind: kind, Status: run.status, Conclusion: run.conclusion, Commit: commit,
			URL: rsRunURL(run.id), StartedAt: run.started,
		}
	}
}

func withMarkers(markers ...ReviewMarker) func(*PRReviewState) {
	return func(s *PRReviewState) { s.Markers = markers }
}

func withNow(now string) func(*PRReviewState) { return func(s *PRReviewState) { s.Now = now } }

func withRequest(id int64, created string) func(*PRReviewState) {
	return func(s *PRReviewState) {
		s.Request = &ReviewRequest{ID: id, URL: rsURL(id), CreatedAt: created}
	}
}

func marker(requestID int64, commit string, run rsRun, reviewURL string) ReviewMarker {
	return ReviewMarker{
		Commit: commit, RequestID: requestID, RequestURL: rsURL(requestID), CheckRunURL: rsRunURL(run.id),
		ReviewURL: reviewURL, CompletedAt: run.completed,
	}
}

func indeterminate(reason IndeterminateReason, urls ...string) func(*PRReviewState) {
	return func(s *PRReviewState) {
		s.IndeterminateReason = reason
		if urls != nil {
			s.IndeterminateURLs = urls
		}
	}
}

// reviewStateCases are the rows of the state rules, one behavior per row.
func reviewStateCases() []reviewStateCase {
	finished := rsFinished(11, at("10:00:20"), at("10:03:00"))
	earlier := rsFinished(21, at("09:00:20"), at("09:02:00"))
	laterNow := withNow(at("10:15:01"))
	rows := []reviewStateCase{
		// No request comment: only a comment of the user with the request text
		// counts, and nothing after the comments is read.
		{
			name: "none_without_comments", in: rsInput{},
			want: func() PRReviewState { s := rsWant(RequestNone); s.Request = nil; return s }(),
		},
		{
			name: "none_for_other_user_same_body",
			in:   rsInput{comments: []rsComment{{id: 1, login: "alice", kind: typenameUser, body: rsRequestText, created: rsCreated}}},
			want: func() PRReviewState { s := rsWant(RequestNone); s.Request = nil; return s }(),
		},
		{
			name: "none_for_bot_with_the_users_name",
			in:   rsInput{comments: []rsComment{{id: 1, login: rsUser, kind: typenameBot, body: rsRequestText, created: rsCreated}}},
			want: func() PRReviewState { s := rsWant(RequestNone); s.Request = nil; return s }(),
		},
		{
			name: "none_for_the_reviewer_as_a_user",
			in:   rsInput{comments: []rsComment{{id: 1, login: testReviewer, kind: typenameUser, body: rsRequestText, created: rsCreated}}},
			want: func() PRReviewState { s := rsWant(RequestNone); s.Request = nil; return s }(),
		},
		{
			name: "none_for_another_body",
			in:   rsInput{comments: []rsComment{{id: 1, login: rsUser, kind: typenameUser, body: rsRequestText + " now", created: rsCreated}}},
			want: func() PRReviewState { s := rsWant(RequestNone); s.Request = nil; return s }(),
		},
		{
			// White space around the request text does not make another
			// request: GitHub keeps a trailing newline or CRLF of a comment.
			name: "waiting_for_a_request_with_surrounding_white_space",
			in: rsInput{
				comments: []rsComment{{id: 1, login: rsUser, kind: typenameUser, body: " \r\n" + rsRequestText + "\r\n", created: rsCreated}},
				commits:  []rsCommit{rsCommitOf(rsHead)},
			},
			want: rsWant(RequestWaiting),
		},

		// Waiting and the responses that are not a completion.
		{name: "waiting_without_runs", in: rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead)}}, want: rsWant(RequestWaiting)},
		{
			name: "waiting_while_running",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsRunning(at("10:00:20")))}},
			want: rsWant(RequestWaiting, withResponse(RunRunning, rsRunning(at("10:00:20")), rsHead)),
		},
		{
			name: "queued_run_without_start_is_not_a_response",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsQueued(11))}},
			want: rsWant(RequestWaiting),
		},
		{
			name: "skip_notice_is_not_a_response",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsSkipNotice(11, at("10:00:20")))}},
			want: rsWant(RequestWaiting),
		},
		{
			name: "run_before_the_request_is_not_a_response",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsFinished(11, at("09:59:50"), at("09:59:58")))}},
			want: rsWant(RequestWaiting),
		},
		{
			name: "run_in_the_same_second_is_not_a_response",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsFinished(11, rsCreated, at("10:03:00")))}},
			want: rsWant(RequestWaiting),
		},
		{
			name: "run_of_another_app_is_ignored",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{{
				oid: rsHead, suites: []rsSuite{{app: "other-app", runs: []rsRun{finished}}},
			}}},
			want: rsWant(RequestWaiting),
		},
		{
			name: "suite_without_an_app_is_ignored",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{{
				oid: rsHead, suites: []rsSuite{{app: "", runs: []rsRun{finished}}},
			}}},
			want: rsWant(RequestWaiting),
		},

		// Completion, with the PR review of the same commit as a URL.
		{
			name: "completed_without_a_pr_review",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, finished)}},
			want: rsWant(RequestCompleted, withResponse(RunFinished, finished, rsHead), withMarkers(marker(1, rsHead, finished, ""))),
		},
		{
			name: "completed_with_the_pr_review_of_the_commit",
			in: rsInput{
				comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, finished)},
				reviews: []rsReview{
					{login: "alice", kind: typenameUser, commit: rsHead, submitted: at("10:03:10"), id: 1},
					{login: "other-app[bot]", kind: typenameBot, commit: rsHead, submitted: at("10:03:10"), id: 2},
					{login: testReviewer + botLoginSuffix, kind: typenameUser, commit: rsHead, submitted: at("10:03:10"), id: 3},
					rsBotReview(4, rsSecond, at("10:03:10")),
					rsBotReview(5, rsHead, at("09:59:00")),
					rsBotReview(6, rsHead, ""),
					rsBotReview(7, rsHead, at("10:03:10")),
					rsBotReview(8, rsHead, at("10:03:20")),
				},
			},
			want: rsWant(RequestCompleted, withResponse(RunFinished, finished, rsHead),
				withMarkers(marker(1, rsHead, finished, rsReviewURL(7)))),
		},
		{
			name: "run_after_the_response_is_not_counted",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{
				rsCommitOf(rsHead, finished), rsCommitOf(rsSecond, rsFinished(12, at("10:04:00"), at("10:04:30"))),
			}},
			want: rsWant(RequestCompleted, withResponse(RunFinished, finished, rsHead), withMarkers(marker(1, rsHead, finished, ""))),
		},
		{
			name: "rate_limited",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsRateLimited(11, at("10:00:20")))}},
			want: rsWant(RequestRateLimited, withResponse(RunRateLimited, rsRateLimited(11, at("10:00:20")), rsHead)),
		},
		{
			name: "success_with_another_title_is_unrecognized",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsRun{
				id: 11, status: statusCompleted, conclusion: conclusionSuccess, title: "No files to review", started: at("10:00:20"), completed: at("10:00:30"),
			})}},
			want: rsWant(RequestUnrecognized, withResponse(RunUnknown, rsRun{
				id: 11, status: statusCompleted, conclusion: conclusionSuccess, started: at("10:00:20"),
			}, rsHead)),
		},
		{
			name: "failure_is_unrecognized",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsRun{
				id: 11, status: statusCompleted, conclusion: "FAILURE", title: "Review failed", started: at("10:00:20"), completed: at("10:00:30"),
			})}},
			want: rsWant(RequestUnrecognized, withResponse(RunUnknown, rsRun{
				id: 11, status: statusCompleted, conclusion: "FAILURE", started: at("10:00:20"),
			}, rsHead)),
		},

		// The wait limit: 15 minutes, strictly past it.
		{
			name: "waiting_at_exactly_the_limit", in: rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead)}, now: at("10:15:00")},
			want: rsWant(RequestWaiting, withNow(at("10:15:00"))),
		},
		{
			name: "timed_out_a_second_past_the_limit", in: rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead)}, now: at("10:15:01")},
			want: rsWant(RequestTimedOut, laterNow),
		},
		{
			name: "timed_out_with_a_run_still_running",
			in: rsInput{
				comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsRunning(at("10:00:20")))}, now: at("10:15:01"),
			},
			want: rsWant(RequestTimedOut, laterNow, withResponse(RunRunning, rsRunning(at("10:00:20")), rsHead)),
		},
		{
			name: "queued_run_without_start_times_out",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsQueued(11))}, now: at("10:15:01")},
			want: rsWant(RequestTimedOut, laterNow),
		},
		{
			name: "completed_after_the_limit_stays_completed",
			in:   rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, finished)}, now: at("10:30:00")},
			want: rsWant(RequestCompleted, withNow(at("10:30:00")), withResponse(RunFinished, finished, rsHead), withMarkers(marker(1, rsHead, finished, ""))),
		},

		// Competing responses and edited requests are never a completion.
		{
			name: "competing_runs_in_the_same_second",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{
				rsCommitOf(rsHead, rsFinished(11, at("10:00:20"), at("10:03:00")), rsFinished(12, at("10:00:20"), at("10:03:05"))),
			}},
			want: rsWant(RequestIndeterminate, indeterminate(IndeterminateCompetingRuns, rsRunURL(11), rsRunURL(12))),
		},
		{
			// Both runs end in the second they start, so only the shared start
			// second shows that they compete.
			name: "competing_runs_that_finish_in_their_first_second",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{
				rsCommitOf(rsHead, rsRateLimited(11, at("10:00:20")), rsRateLimited(12, at("10:00:20"))),
			}},
			want: rsWant(RequestIndeterminate, indeterminate(IndeterminateCompetingRuns, rsRunURL(11), rsRunURL(12))),
		},
		{
			name: "competing_runs_that_overlap",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{
				rsCommitOf(rsHead, rsFinished(11, at("10:00:20"), at("10:03:00")), rsFinished(12, at("10:02:59"), at("10:04:00"))),
			}},
			want: rsWant(RequestIndeterminate, indeterminate(IndeterminateCompetingRuns, rsRunURL(11), rsRunURL(12))),
		},
		{
			name: "competing_run_started_while_the_first_still_runs",
			in: rsInput{comments: defaultComments(), commits: []rsCommit{
				rsCommitOf(rsHead, rsRunning(at("10:00:20")), rsFinished(12, at("10:01:00"), at("10:02:00"))),
			}},
			want: rsWant(RequestIndeterminate, indeterminate(IndeterminateCompetingRuns, rsRunURL(11), rsRunURL(12))),
		},
		{
			name: "edited_request",
			in: rsInput{
				comments: []rsComment{{id: 1, login: rsUser, kind: typenameUser, body: rsRequestText, created: rsCreated, updated: at("10:01:00")}},
				commits:  []rsCommit{rsCommitOf(rsHead, finished)},
			},
			want: rsWant(RequestIndeterminate, indeterminate(IndeterminateEditedRequest)),
		},
		{
			// The edited request is the latest by its last update although it
			// was created first.
			name: "edited_request_is_latest_by_update_time",
			in: rsInput{
				comments: []rsComment{
					{id: 1, login: rsUser, kind: typenameUser, body: rsRequestText, created: rsCreated, updated: at("10:20:00")},
					requestAt(2, "10:10:00"),
				},
				commits: []rsCommit{rsCommitOf(rsHead)},
			},
			want: rsWant(RequestIndeterminate, indeterminate(IndeterminateEditedRequest)),
		},
		{
			// Edited and unedited requests with one update time: the edited
			// one is the latest, whatever the ids.
			name: "edited_request_is_latest_in_a_tie",
			in: rsInput{
				comments: []rsComment{
					{id: 1, login: rsUser, kind: typenameUser, body: rsRequestText, created: at("09:50:00"), updated: rsCreated},
					rsRequestComment(2, rsCreated),
				},
				commits: []rsCommit{rsCommitOf(rsHead)},
			},
			want: rsWant(RequestIndeterminate, withRequest(1, at("09:50:00")), indeterminate(IndeterminateEditedRequest)),
		},

		// Earlier requests: their finished responses are completion markers.
		{
			name: "marker_of_an_earlier_request_while_the_latest_waits",
			in: rsInput{
				comments: []rsComment{requestAt(1, "09:00:00"), requestAt(2, "10:00:00")},
				commits:  []rsCommit{rsCommitOf(rsSecond, earlier)},
			},
			want: rsWant(RequestWaiting, withRequest(2, at("10:00:00")),
				withMarkers(marker(1, rsSecond, earlier, ""))),
		},
		{
			name: "markers_oldest_first_when_both_complete",
			in: rsInput{
				comments: []rsComment{requestAt(2, "10:00:00"), requestAt(1, "09:00:00")},
				commits: []rsCommit{
					rsCommitOf(rsSecond, earlier),
					rsCommitOf(rsHead, finished),
				},
			},
			want: rsWant(RequestCompleted, withRequest(2, at("10:00:00")), withResponse(RunFinished, finished, rsHead),
				withMarkers(marker(1, rsSecond, earlier, ""), marker(2, rsHead, finished, ""))),
		},
		{
			// A run that started after the next request's last update answers
			// that request, not the earlier one.
			name: "earlier_response_ends_at_the_next_request",
			in: rsInput{
				comments: []rsComment{requestAt(1, "09:00:00"), requestAt(2, "10:00:00")},
				commits:  []rsCommit{rsCommitOf(rsHead, finished)},
			},
			want: rsWant(RequestCompleted, withRequest(2, at("10:00:00")), withResponse(RunFinished, finished, rsHead),
				withMarkers(marker(2, rsHead, finished, ""))),
		},
		{
			name: "no_marker_for_an_edited_earlier_request",
			in: rsInput{
				comments: []rsComment{
					{id: 1, login: rsUser, kind: typenameUser, body: rsRequestText, created: at("09:00:00"), updated: at("09:10:00")},
					requestAt(2, "10:00:00"),
				},
				commits: []rsCommit{rsCommitOf(rsSecond, earlier)},
			},
			want: rsWant(RequestWaiting, withRequest(2, at("10:00:00"))),
		},
		{
			name: "no_marker_for_an_earlier_request_with_competing_runs",
			in: rsInput{
				comments: []rsComment{requestAt(1, "09:00:00"), requestAt(2, "10:00:00")},
				commits: []rsCommit{rsCommitOf(rsSecond,
					earlier, rsFinished(12, at("09:00:20"), at("09:02:10")))},
			},
			want: rsWant(RequestWaiting, withRequest(2, at("10:00:00"))),
		},
		{
			name: "no_marker_for_an_earlier_request_that_was_rate_limited",
			in: rsInput{
				comments: []rsComment{requestAt(1, "09:00:00"), requestAt(2, "10:00:00")},
				commits:  []rsCommit{rsCommitOf(rsSecond, rsRateLimited(11, at("09:00:20")))},
			},
			want: rsWant(RequestWaiting, withRequest(2, at("10:00:00"))),
		},
	}
	return rows
}

// checkReviewState runs the scripted calls and compares the whole state.
func checkReviewState(t *testing.T, in rsInput, want PRReviewState) {
	t.Helper()
	if got := runReviewState(t, in); !reflect.DeepEqual(got, want) {
		t.Errorf("state = %s\nwant    %s", mustJSON(t, got), mustJSON(t, want))
	}
}

func TestReviewState(t *testing.T) {
	t.Parallel()
	for _, tc := range reviewStateCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkReviewState(t, tc.in, tc.want)
		})
	}
}

// TestReviewStateUnreadable: a failed call, a response outside the strict
// shape, and a limit are the unreadable state with their reason. The head is
// set once it was read and checked, and nothing else is kept.
func TestReviewStateUnreadable(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	bad := func(old, replacement string) func(testing.TB, string) string {
		return func(t testing.TB, s string) string {
			t.Helper()
			if !strings.Contains(s, old) {
				t.Fatalf("response %q lacks %q", s, old)
			}
			return strings.Replace(s, old, replacement, 1)
		}
	}
	whole := func(text string) func(testing.TB, string) string {
		return func(testing.TB, string) string { return text }
	}
	one := rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsFinished(11, at("10:00:20"), at("10:03:00")))}}
	with := func(stage string, f func(testing.TB, string) string) rsInput {
		in := one
		in.edit = map[string]func(testing.TB, string) string{stage: f}
		in.stop = stage
		return in
	}
	failing := func(stage string, err error) rsInput {
		in := one
		in.fail = map[string]error{stage: err}
		return in
	}
	rows := []struct {
		name string
		in   rsInput
		want PRReviewState
	}{
		// Failed calls.
		{"repository_call_fails", failing(rsStageRepo, boom), rsUnreadable(UnreadableCall, "")},
		{"user_call_fails", failing(rsStageUser, boom), rsUnreadable(UnreadableCall, rsHead)},
		{"pull_call_fails", failing(rsStagePull, boom), rsUnreadable(UnreadableCall, "")},
		{"pull_status_is_not_200", rsInput{pullStatus: http.StatusNoContent, stop: rsStagePull}, rsUnreadable(UnreadableCall, "")},
		{"comments_call_fails", failing(rsStageComments, boom), rsUnreadable(UnreadableCall, rsHead)},
		{"graphql_call_fails", failing(rsStageGraphQL, boom), rsUnreadable(UnreadableCall, rsHead)},
		{"reviews_call_fails", failing(rsStageReviews, boom), rsUnreadable(UnreadableCall, rsHead)},
		{"output_over_the_cap", failing(rsStageGraphQL, cmdrunner.ErrOutputTooLarge), rsUnreadable(UnreadableLimit, rsHead)},
		{"graphql_response_over_max_bytes", with(rsStageGraphQL, whole(strings.Repeat(" ", maxResponseBytes+1))), rsUnreadable(UnreadableLimit, rsHead)},

		// The user and the head.
		{"user_login_outside_the_grammar", with(rsStageUser, bad(rsUser, "bad name")), rsUnreadable(UnreadableRejected, rsHead)},
		{"user_login_duplicated", with(rsStageUser, bad(`"id":1`, `"login":"x"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"head_not_hex", with(rsStagePull, bad(rsHead, strings.Repeat("g", 40))), rsUnreadable(UnreadableRejected, "")},
		{"head_upper_case", with(rsStagePull, bad(rsHead, strings.Repeat("A", 40))), rsUnreadable(UnreadableRejected, "")},
		{"head_too_short", with(rsStagePull, bad(rsHead, strings.Repeat("a", 39))), rsUnreadable(UnreadableRejected, "")},
		{"head_member_missing", with(rsStagePull, whole(`{"title":"x"}`)), rsUnreadable(UnreadableRejected, "")},
		{"head_sha_duplicated", with(rsStagePull, whole(`{"head":{"sha":"`+rsHead+`","sha":"`+rsHead+`"}}`)), rsUnreadable(UnreadableRejected, "")},

		// Comments.
		{"comments_not_an_array", with(rsStageComments, whole(`{}`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_id_duplicated", with(rsStageComments, bad(`"id":1,`, `"id":1,"id":1,`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_id_zero", with(rsStageComments, bad(`"id":1,`, `"id":0,`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_id_string", with(rsStageComments, bad(`"id":1,`, `"id":"1",`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_user_missing", with(rsStageComments, bad(`,"user":{"id":1,"login":"`+rsUser+`","type":"User"}`, ``)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_user_a_string", with(rsStageComments, bad(`"user":{"id":1,"login":"`+rsUser+`","type":"User"}`, `"user":"x"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_body_null", with(rsStageComments, bad(`"body":"`+rsRequestText+`"`, `"body":null`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_created_at_not_a_time", with(rsStageComments, bad(`"created_at":"`+rsCreated+`"`, `"created_at":"yesterday"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"comment_url_not_github", with(rsStageComments, bad(`"html_url":"https://github.com/`, `"html_url":"https://example.com/`)), rsUnreadable(UnreadableRejected, rsHead)},

		// Commits and check runs.
		{"graphql_head_changed", with(rsStageGraphQL, bad(`"headRefOid":"`+rsHead+`"`, `"headRefOid":"`+rsSecond+`"`)), rsUnreadable(UnreadableCall, rsHead)},
		{"graphql_head_not_hex", with(rsStageGraphQL, bad(`"headRefOid":"`+rsHead+`"`, `"headRefOid":"x"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"graphql_errors_member", with(rsStageGraphQL, bad(`{"data"`, `{"errors":[],"data"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"commit_oid_not_hex", with(rsStageGraphQL, bad(`"oid":"`+rsHead+`"`, `"oid":"xyz"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_completed_without_title", with(rsStageGraphQL, bad(`"title":"Finished"`, `"title":null`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_completed_without_end", with(rsStageGraphQL, bad(`"completedAt":"`+at("10:03:00")+`"`, `"completedAt":null`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_status_null", with(rsStageGraphQL, bad(`"status":"COMPLETED"`, `"status":null`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_start_not_a_time", with(rsStageGraphQL, bad(`"startedAt":"`+at("10:00:20")+`"`, `"startedAt":"soon"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_url_not_github", with(rsStageGraphQL, bad(`"url":"https://github.com/`, `"url":"https://example.com/`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_title_duplicated", with(rsStageGraphQL, bad(`"title":"Finished"`, `"title":"Finished","title":"Finished"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_name_mismatch", with(rsStageGraphQL, bad(`"name":"CodeRabbit"`, `"name":"Other"`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_name_missing", with(rsStageGraphQL, bad(`"name":"CodeRabbit",`, "")), rsUnreadable(UnreadableRejected, rsHead)},
		{
			"suites_continue", func() rsInput {
				in := one
				in.commits = []rsCommit{{oid: rsHead, suites: []rsSuite{{app: testReviewer}}, more: true}}
				in.stop = rsStageGraphQL
				return in
			}(), rsUnreadable(UnreadableLimit, rsHead),
		},
		{
			"runs_continue", func() rsInput {
				in := one
				in.commits = []rsCommit{{oid: rsHead, suites: []rsSuite{{app: testReviewer, moreRuns: true}}}}
				in.stop = rsStageGraphQL
				return in
			}(), rsUnreadable(UnreadableLimit, rsHead),
		},

		// Reviews.
		{"review_commit_not_a_string", with(rsStageReviews, whole(`[{"user":null,"commit_id":1,"submitted_at":null,"html_url":"`+rsReviewURL(1)+`"}]`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"review_time_not_a_time", with(rsStageReviews, whole(`[{"user":null,"commit_id":"x","submitted_at":"later","html_url":"`+rsReviewURL(1)+`"}]`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"review_user_type_missing", with(rsStageReviews, whole(`[{"user":{"login":"a"},"commit_id":"x","submitted_at":null,"html_url":"`+rsReviewURL(1)+`"}]`)), rsUnreadable(UnreadableRejected, rsHead)},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkReviewState(t, tc.in, tc.want)
		})
	}
}

// TestReviewStateLimits: each fixed limit is a boundary. At the limit the
// state is read; one over is the unreadable state with reason limit.
func TestReviewStateLimits(t *testing.T) {
	t.Parallel()
	noise := func(n int) []rsComment {
		out := make([]rsComment, 0, n+1)
		for i := range n {
			out = append(out, rsComment{id: int64(i + 2), login: "alice", kind: typenameUser, body: "x", created: rsCreated})
		}
		return append(out, rsRequestComment(1, rsCreated))
	}
	reviews := func(n int) []rsReview {
		out := make([]rsReview, 0, n)
		for i := range n {
			out = append(out, rsReview{login: "alice", kind: typenameUser, commit: rsHead, id: int64(i + 1)})
		}
		return out
	}
	commits := func(n int) []rsCommit {
		out := make([]rsCommit, 0, n)
		for range n {
			out = append(out, rsCommitOf(rsHead))
		}
		return out
	}
	rows := []struct {
		name string
		in   rsInput
		want PRReviewState
	}{
		{"comments_at_the_limit", rsInput{comments: noise(maxReviewComments - 1), commits: commits(1)}, rsWant(RequestWaiting)},
		{"comments_over_the_limit", rsInput{comments: noise(maxReviewComments), commits: commits(1), stop: rsStageComments}, rsUnreadable(UnreadableLimit, rsHead)},
		{"reviews_at_the_limit", rsInput{comments: defaultComments(), commits: commits(1), reviews: reviews(maxReviewReviews)}, rsWant(RequestWaiting)},
		{"reviews_over_the_limit", rsInput{comments: defaultComments(), commits: commits(1), reviews: reviews(maxReviewReviews + 1)}, rsUnreadable(UnreadableLimit, rsHead)},
		{"commits_at_the_limit", rsInput{comments: defaultComments(), commits: commits(maxPRCommits)}, rsWant(RequestWaiting)},
		{"commits_over_the_limit", rsInput{comments: defaultComments(), commits: commits(maxPRCommits + 1), stop: rsStageGraphQL}, rsUnreadable(UnreadableLimit, rsHead)},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkReviewState(t, tc.in, tc.want)
		})
	}
}

// TestReviewStateReadBounds: the bounds on what one response or one read may
// hold, which the count limits of TestReviewStateLimits do not reach, and the
// expired deadline. Each row scripts only the calls up to the bound, so a read
// that went past it would make an unscripted call.
func TestReviewStateReadBounds(t *testing.T) {
	t.Parallel()
	// upTo is the calls of a read with a request comment, up to and including
	// the comments.
	upTo := func(t *testing.T) []cmdrunnertestutil.Step {
		return rsInput{comments: defaultComments(), stop: rsStageComments}.steps(t)
	}
	commitsPage := func(t *testing.T, commits []rsCommit, more bool, cursor string) cmdrunnertestutil.Step {
		nodes := make([]any, 0, len(commits))
		for _, c := range commits {
			nodes = append(nodes, c.json())
		}
		return graphqlStep(rsCommitsResponse(t, rsHead, nodes, more, cursor))
	}
	suites := func(n int) []rsSuite {
		out := make([]rsSuite, n)
		for i := range out {
			out[i] = rsSuite{app: testReviewer}
		}
		return out
	}
	runs := func(n int) []rsRun {
		out := make([]rsRun, n)
		for i := range out {
			out[i] = rsQueued(int64(i + 1))
		}
		return out
	}
	rows := []struct {
		name  string
		steps func(t *testing.T) []cmdrunnertestutil.Step
		want  PRReviewState
	}{
		{"graphql_pages_never_end", func(t *testing.T) []cmdrunnertestutil.Step {
			steps := upTo(t)
			for i := range maxPRCommitPages {
				steps = append(steps, commitsPage(t, nil, true, "cursor"+strconv.Itoa(i)))
			}
			return steps
		}, rsUnreadable(UnreadableLimit, rsHead)},
		{"graphql_page_over_its_size", func(t *testing.T) []cmdrunnertestutil.Step {
			commits := make([]rsCommit, commitsPerPage+1)
			for i := range commits {
				commits[i] = rsCommitOf(rsHead)
			}
			return append(upTo(t), commitsPage(t, commits, false, ""))
		}, rsUnreadable(UnreadableRejected, rsHead)},
		{"suites_over_the_limit", func(t *testing.T) []cmdrunnertestutil.Step {
			return append(upTo(t), commitsPage(t, []rsCommit{{oid: rsHead, suites: suites(maxCheckSuites + 1)}}, false, ""))
		}, rsUnreadable(UnreadableLimit, rsHead)},
		{"runs_over_the_limit", func(t *testing.T) []cmdrunnertestutil.Step {
			commit := rsCommit{oid: rsHead, suites: []rsSuite{{app: testReviewer, runs: runs(maxCheckRuns + 1)}}}
			return append(upTo(t), commitsPage(t, []rsCommit{commit}, false, ""))
		}, rsUnreadable(UnreadableLimit, rsHead)},
		{"rest_page_over_its_size", func(t *testing.T) []cmdrunnertestutil.Step {
			comments := make([]any, 0, restPerPage+1)
			comments = append(comments, rsRequestComment(1, rsCreated).json())
			for i := range restPerPage {
				comments = append(comments, rsComment{id: int64(i + 2), login: "alice", kind: typenameUser, body: "x", created: rsCreated}.json())
			}
			in := rsInput{comments: defaultComments(), stop: rsStageComments}
			in.edit = map[string]func(testing.TB, string) string{rsStageComments: func(t testing.TB, _ string) string { return mustJSON(t, comments) }}
			return in.steps(t)
		}, rsUnreadable(UnreadableRejected, rsHead)},
		{"user_response_over_max_bytes", func(t *testing.T) []cmdrunnertestutil.Step {
			in := rsInput{stop: rsStageUser}
			in.edit = map[string]func(testing.TB, string) string{rsStageUser: func(testing.TB, string) string { return strings.Repeat(" ", maxResponseBytes+1) }}
			return in.steps(t)
		}, rsUnreadable(UnreadableLimit, rsHead)},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := runReviewStateSteps(t.Context(), t, tc.steps(t)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("state = %s\nwant    %s", mustJSON(t, got), mustJSON(t, tc.want))
			}
		})
	}

	// A read that fails after the deadline passed is a failed call, whatever
	// the response it failed on: here a head outside the grammar, which is
	// rejected while the deadline holds.
	t.Run("deadline_passed", func(t *testing.T) {
		t.Parallel()
		in := rsInput{stop: rsStagePull}
		in.edit = map[string]func(testing.TB, string) string{rsStagePull: func(_ testing.TB, s string) string { return strings.Replace(s, rsHead, "x", 1) }}
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
		t.Cleanup(cancel)
		want := rsUnreadable(UnreadableCall, "")
		if got := runReviewStateSteps(ctx, t, in.steps(t)); !reflect.DeepEqual(got, want) {
			t.Errorf("state = %s\nwant    %s", mustJSON(t, got), mustJSON(t, want))
		}
	})
}

// TestReviewStateReads: the commit query asks for every check run matching
// $checkName (not only the latest of its name), asks for name, and never asks
// for detailsUrl or databaseId. Both profiles send their declared checkName.
func TestReviewStateReads(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		profile   ReviewProfile
		reviewer  string
		requests  []string
		wantCheck string
	}{
		{"coderabbit", ProfileCodeRabbit, testReviewer, []string{rsRequestText}, checkNameCodeRabbit},
		{"sourcery", ProfileSourcery, rsSourceryReviewer, []string{rsSourceryReq1}, checkNameSourcery},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := rsInput{
				profile:  tc.profile,
				reviewer: tc.reviewer,
				requests: tc.requests,
				comments: []rsComment{{id: 1, login: rsUser, kind: typenameUser, body: tc.requests[0], created: rsCreated}},
				commits:  []rsCommit{{oid: rsHead}, {oid: rsSecond}},
			}
			fake := cmdrunnertestutil.New(t, in.steps(t)...)
			g, err := NewGitHubWithoutReviewer(fake)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := g.PRReviewState(t.Context(), ReviewStateRequest{
				PR: mustPR(t), Profile: tc.profile, Reviewer: mustLogin(t, tc.reviewer), Requests: tc.requests,
			}); err != nil {
				t.Fatal(err)
			}
			type graphqlReq struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			var reqs []graphqlReq
			for _, stdin := range fake.Stdin {
				if stdin == nil {
					continue
				}
				var req graphqlReq
				if err := json.Unmarshal(stdin, &req); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				reqs = append(reqs, req)
			}
			if len(reqs) != 1 {
				t.Fatalf("GraphQL requests = %d, want 1", len(reqs))
			}
			for _, want := range []string{
				"$checkName: String!", "pullRequest(number: $number)", "commits(first: 20",
				"checkName: $checkName", "checkType: ALL", "headRefOid", "checkSuites(first: 50)", "checkRuns(first: 100",
				"nodes { name status conclusion title startedAt completedAt url }",
			} {
				if !strings.Contains(reqs[0].Query, want) {
					t.Errorf("query lacks %q", want)
				}
			}
			if got := reqs[0].Variables["checkName"]; got != tc.wantCheck {
				t.Errorf("variables.checkName = %v, want %q", got, tc.wantCheck)
			}
			// detailsUrl carries a share value and databaseId overflows 32 bits;
			// neither may be asked for.
			for _, forbidden := range []string{"detailsurl", "databaseid"} {
				if strings.Contains(strings.ToLower(reqs[0].Query), forbidden) {
					t.Errorf("query asks for %s", forbidden)
				}
			}
		})
	}
}

// TestReviewStateOutputHasNoText: no comment body, check run title, details
// URL or PR review body reaches the output, in any state or profile.
func TestReviewStateOutputHasNoText(t *testing.T) {
	t.Parallel()
	secret := "secret-text"
	comments := append(defaultComments(), rsComment{id: 2, login: "alice", kind: typenameUser, body: secret, created: rsCreated})
	titled := rsRun{id: 11, status: statusCompleted, conclusion: conclusionSuccess, title: secret, started: at("10:00:20"), completed: at("10:00:30")}
	sourceryTitled := rsRun{id: 12, name: checkNameSourcery, status: statusCompleted, conclusion: "FAILURE", title: secret, started: at("10:00:20"), completed: at("10:00:30")}
	for _, in := range []rsInput{
		{comments: comments, commits: []rsCommit{rsCommitOf(rsHead, titled)}, reviews: []rsReview{rsBotReview(1, rsHead, at("10:00:40"))}},
		{comments: comments, commits: []rsCommit{rsCommitOf(rsHead, rsFinished(11, at("10:00:20"), at("10:03:00")))}, reviews: []rsReview{rsBotReview(1, rsHead, at("10:03:10"))}},
		{comments: comments, commits: []rsCommit{rsCommitOf(rsHead)}, edit: map[string]func(testing.TB, string) string{rsStageGraphQL: func(testing.TB, string) string { return "{" }}, stop: rsStageGraphQL},
		{profile: ProfileSourcery, comments: []rsComment{{id: 2, login: "alice", kind: typenameUser, body: secret, created: rsCreated}}, commits: []rsCommit{rsSourceryCommitOf(rsHead, sourceryTitled)}},
		{profile: ProfileSourcery, comments: []rsComment{{id: 2, login: "alice", kind: typenameUser, body: secret, created: rsCreated}}, commits: []rsCommit{rsSourceryCommitOf(rsHead, rsSourceryFinished(13, at("10:00:20"), at("10:03:00")))}},
	} {
		out := mustJSON(t, runReviewState(t, in))
		for _, leak := range []string{secret, "secret-scope", "secret-review-body", "secret-title", "detailsUrl", "details_url"} {
			if strings.Contains(out, leak) {
				t.Errorf("output %s holds %q", out, leak)
			}
		}
	}
}

// TestReviewStateArguments: an unusable argument is an error, and no call
// runs.
func TestReviewStateArguments(t *testing.T) {
	t.Parallel()
	good := ReviewStateRequest{
		PR: mustPR(t), Profile: ProfileCodeRabbit, Reviewer: mustLogin(t, testReviewer), Requests: []string{rsRequestText},
	}
	for name, req := range map[string]ReviewStateRequest{
		"zero_pr":            {Profile: good.Profile, Reviewer: good.Reviewer, Requests: good.Requests},
		"zero_profile":       {PR: good.PR, Reviewer: good.Reviewer, Requests: good.Requests},
		"invalid_profile":    {PR: good.PR, Profile: ReviewProfile(99), Reviewer: good.Reviewer, Requests: good.Requests},
		"zero_reviewer":      {PR: good.PR, Profile: good.Profile, Requests: good.Requests},
		"empty_requests":     {PR: good.PR, Profile: good.Profile, Reviewer: good.Reviewer},
		"empty_request_text": {PR: good.PR, Profile: good.Profile, Reviewer: good.Reviewer, Requests: []string{""}},
		"padded_request":     {PR: good.PR, Profile: good.Profile, Reviewer: good.Reviewer, Requests: []string{" " + rsRequestText}},
		"duplicate_request":  {PR: good.PR, Profile: good.Profile, Reviewer: good.Reviewer, Requests: []string{rsRequestText, rsRequestText}},
		"long_request":       {PR: good.PR, Profile: good.Profile, Reviewer: good.Reviewer, Requests: []string{strings.Repeat("x", maxRequestTextLength+1)}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := g.PRReviewState(t.Context(), req); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

// TestReviewStateEnumsRejectOutsideValues: a value outside an enumeration is
// an error, never a guessed name, so an unset or corrupted state cannot reach
// the output as a valid word. (That every listed value has its word in the
// guide is TestGuideListsReviewStateValues' check.)
func TestReviewStateEnumsRejectOutsideValues(t *testing.T) {
	t.Parallel()
	for name, marshal := range map[string]func() ([]byte, error){
		"request_state":        RequestState(99).MarshalText,
		"unreadable_reason":    UnreadableReason(99).MarshalText,
		"indeterminate_reason": IndeterminateReason(99).MarshalText,
		"run_kind":             RunKind(99).MarshalText,
		"review_profile_zero":  ProfileUnknown.MarshalText,
		"review_profile_high":  ReviewProfile(99).MarshalText,
		"auto_review_state":    AutoReviewState(99).MarshalText,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := marshal(); !errors.Is(err, errUnknownEnum) {
				t.Errorf("err = %v, want errUnknownEnum", err)
			}
		})
	}
	// Every declared AutoReviewState marshals cleanly.
	wantAuto := []string{"none", "running", "completed", "skipped", "unrecognized", "timed-out"}
	var gotAuto []string
	for _, s := range AutoReviewStates() {
		raw, err := s.MarshalText()
		if err != nil {
			t.Fatalf("AutoReviewState(%d).MarshalText: %v", s, err)
		}
		gotAuto = append(gotAuto, string(raw))
	}
	if !slices.Equal(gotAuto, wantAuto) {
		t.Errorf("AutoReviewStates() = %v, want %v", gotAuto, wantAuto)
	}
	// Zero PRReviewState fails closed because ProfileUnknown refuses to marshal.
	if _, err := json.Marshal(PRReviewState{}); !errors.Is(err, errUnknownEnum) {
		t.Errorf("json.Marshal(PRReviewState{}) err = %v, want errUnknownEnum", err)
	}
	// With Profile set, the zero State assumes least (indeterminate).
	zeroState := mustJSON(t, PRReviewState{Profile: ProfileCodeRabbit})
	if !strings.Contains(zeroState, `"state":"indeterminate"`) {
		t.Errorf("zero PRReviewState JSON = %s, want state indeterminate", zeroState)
	}
	zeroResp := mustJSON(t, ReviewResponse{})
	if !strings.Contains(zeroResp, `"kind":"unknown"`) {
		t.Errorf("zero ReviewResponse JSON = %s, want kind unknown", zeroResp)
	}
	zeroAuto := mustJSON(t, AutoReview{})
	if !strings.Contains(zeroAuto, `"state":"none"`) || !strings.Contains(zeroAuto, `"kind":"unknown"`) {
		t.Errorf("zero AutoReview JSON = %s, want state none and kind unknown", zeroAuto)
	}
}

func TestParseReviewProfile(t *testing.T) {
	t.Parallel()
	wantProfiles := []ReviewProfile{ProfileCodeRabbit, ProfileSourcery}
	wantNames := map[ReviewProfile]string{
		ProfileCodeRabbit: "coderabbit",
		ProfileSourcery:   "sourcery",
	}
	profiles := ReviewProfiles()
	if !slices.Equal(profiles, wantProfiles) {
		t.Fatalf("ReviewProfiles() = %v, want %v", profiles, wantProfiles)
	}
	for _, p := range profiles {
		raw, err := p.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText(%v) = %v", p, err)
		}
		want := wantNames[p]
		if string(raw) != want {
			t.Errorf("MarshalText(%v) = %q, want %q", p, raw, want)
		}
		got, err := ParseReviewProfile(want)
		if err != nil || got != p {
			t.Errorf("ParseReviewProfile(%q) = %v, %v, want %v", want, got, err, p)
		}
	}
	for _, bad := range []string{"", "CodeRabbit", "SOURCERY", "coderabbitai", "sourcery-ai", " coderabbit", "unknown"} {
		t.Run("invalid/"+bad, func(t *testing.T) {
			t.Parallel()
			got, err := ParseReviewProfile(bad)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ParseReviewProfile(%q) = %v, want ErrInvalidInput", bad, err)
			}
			if got != ProfileUnknown {
				t.Errorf("ParseReviewProfile(%q) profile = %v, want ProfileUnknown", bad, got)
			}
			// The error text is the contract here: it must not echo untrusted input.
			if bad != "" && strings.Contains(err.Error(), bad) {
				t.Errorf("error %q echoes input %q", err, bad)
			}
		})
	}
}

func TestIsRequestComment(t *testing.T) {
	t.Parallel()
	requests := []string{"@coderabbitai review", "@sourcery-ai review", "@sourceryai review"}
	for _, tc := range []struct {
		name  string
		kind  string
		login string
		body  string
		want  bool
	}{
		{"exact_first", typenameUser, rsUser, "@coderabbitai review", true},
		{"exact_second", typenameUser, rsUser, "@sourcery-ai review", true},
		{"exact_third_trimmed", typenameUser, rsUser, " \r\n@sourceryai review\r\n ", true},
		{"other_user", typenameUser, "alice", "@coderabbitai review", false},
		{"bot_same_name", typenameBot, rsUser, "@coderabbitai review", false},
		{"empty_kind", "", rsUser, "@coderabbitai review", false},
		{"extra_words", typenameUser, rsUser, "@coderabbitai review please", false},
		{"wrong_case", typenameUser, rsUser, "@CodeRabbitAI review", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isRequestComment(tc.kind, tc.login, rsUser, tc.body, requests); got != tc.want {
				t.Errorf("isRequestComment = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReviewStateNonPRCommitIgnored: a finished check run belonging to a commit
// outside the PR's commit list, even if mixed into the GraphQL response outside
// pullRequest.commits.nodes, is never read as a response, automatic review, or
// completion marker.
func TestReviewStateNonPRCommitIgnored(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		outside rsCommit
		in      rsInput
		want    PRReviewState
	}{
		{
			name:    "coderabbit",
			outside: rsCommitOf(rsSecond, rsFinished(99, at("10:00:20"), at("10:03:00"))),
			in:      rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead)}},
			want:    rsWant(RequestWaiting),
		},
		{
			name:    "sourcery_auto",
			outside: rsSourceryCommitOf(rsHead, rsSourceryFinished(99, at("10:00:20"), at("10:03:00"))),
			in:      rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead)}},
			want:    rsWantFor(ProfileSourcery, RequestNone),
		},
		{
			name:    "sourcery_manual",
			outside: rsSourceryCommitOf(rsSecond, rsSourceryFinished(99, at("10:00:20"), at("10:03:00"))),
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{{id: 1, login: rsUser, kind: typenameUser, body: rsSourceryReq1, created: rsCreated}},
				commits:  []rsCommit{rsSourceryCommitOf(rsHead)},
			},
			want: rsWantFor(ProfileSourcery, RequestWaiting),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outside := mustJSON(t, tc.outside.json())
			in := tc.in
			in.edit = map[string]func(testing.TB, string) string{
				rsStageGraphQL: func(_ testing.TB, s string) string {
					return strings.Replace(s, `"commits":{`, `"otherCommit":`+outside+`,"commits":{`, 1)
				},
			}
			checkReviewState(t, in, tc.want)
		})
	}
}

// TestReviewStateObserved: responses shaped after real probe observations for
// both CodeRabbit and Sourcery AI, read from files under testdata/review_state.
// Unread members and unread bodies stay out of the result.
func TestReviewStateObserved(t *testing.T) {
	t.Parallel()
	const runURL = "https://github.com/isseis/yt2column/runs/"
	read := func(t *testing.T, name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(isolation.PackageDir, "testdata", "review_state", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(data)
	}
	crFirst := "7cefaa5e00000000000000000000000000000000"
	crHead := "f24ee60a00000000000000000000000000000000"
	crWant := newReviewState(ProfileCodeRabbit, RequestCompleted)
	crWant.Head, crWant.Now = crHead, "2026-11-20T04:35:00Z"
	crWant.Request = &ReviewRequest{ID: 7030003, URL: "https://github.com/isseis/yt2column/pull/1064#issuecomment-7030003", CreatedAt: "2026-11-20T04:26:01Z"}
	crWant.Response = &ReviewResponse{
		Kind: RunFinished, Status: statusCompleted, Conclusion: conclusionSuccess, Commit: crHead,
		URL: runURL + "64000005", StartedAt: "2026-11-20T04:26:25Z",
	}
	crWant.Markers = []ReviewMarker{
		{
			Commit: crFirst, RequestID: 7030001, RequestURL: "https://github.com/isseis/yt2column/pull/1064#issuecomment-7030001",
			CheckRunURL: runURL + "64000001", CompletedAt: "2026-11-20T03:22:24Z",
		},
		{
			Commit: crHead, RequestID: 7030003, RequestURL: "https://github.com/isseis/yt2column/pull/1064#issuecomment-7030003",
			CheckRunURL: runURL + "64000005", ReviewURL: "https://github.com/isseis/yt2column/pull/1064#pullrequestreview-4638479000",
			CompletedAt: "2026-11-20T04:29:18Z",
		},
	}

	sFirst := "86a8f47600000000000000000000000000000000"
	sHead := "701e370100000000000000000000000000000000"
	sWant := newReviewState(ProfileSourcery, RequestCompleted)
	sWant.Head, sWant.Now = sHead, "2026-11-23T10:35:00Z"
	sWant.Request = &ReviewRequest{
		ID: 7097744738, URL: "https://github.com/isseis/yt2column/pull/1094#issuecomment-7097744738", CreatedAt: "2026-11-23T10:17:45Z",
	}
	sWant.Response = &ReviewResponse{
		Kind: RunFinished, Status: statusCompleted, Conclusion: conclusionSuccess, Commit: sFirst,
		URL: runURL + "137624879984", StartedAt: "2026-11-23T10:17:52Z",
	}
	sWant.AutoReview = &AutoReview{
		State: AutoSkipped, Kind: RunSkipped, Status: statusCompleted, Conclusion: conclusionSkipped, Commit: sHead,
		URL: runURL + "137631000000", StartedAt: "2026-11-23T10:30:00Z",
	}
	// Only the manual request's completion on sFirst is a marker; the
	// overlapping automatic completion on the second commit neither competes
	// nor enters Markers.
	sWant.Markers = []ReviewMarker{
		{
			Commit: sFirst, RequestID: 7097744738, RequestURL: "https://github.com/isseis/yt2column/pull/1094#issuecomment-7097744738",
			CheckRunURL: runURL + "137624879984", ReviewURL: "https://github.com/isseis/yt2column/pull/1094#pullrequestreview-6019960677",
			CompletedAt: "2026-11-23T10:27:39Z",
		},
	}

	for _, tc := range []struct {
		name     string
		prefix   string
		now      string
		profile  ReviewProfile
		reviewer string
		requests []string
		want     PRReviewState
	}{
		{"coderabbit", "", "Fri, 20 Nov 2026 04:35:00 GMT", ProfileCodeRabbit, testReviewer, []string{rsRequestText}, crWant},
		{"sourcery", "sourcery_", "Mon, 23 Nov 2026 10:35:00 GMT", ProfileSourcery, rsSourceryReviewer, []string{rsSourceryReq1, rsSourceryReq2}, sWant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			steps := []cmdrunnertestutil.Step{
				repoStep(),
				includeStep("pulls/"+testPR, includedOutput(200, []string{"Date: " + tc.now}, read(t, tc.prefix+"pull.json")), nil),
				{Name: "gh", Args: []string{ghAPI, ghUserEndpoint}, Out: `{"login":"isseis"}`},
				includeStep("issues/"+testPR+"/comments?per_page=100&page=1", includedOutput(200, []string{"Date: " + tc.now}, read(t, tc.prefix+"comments.json")), nil),
				graphqlStep(read(t, tc.prefix+"commits.json")),
				includeStep("pulls/"+testPR+"/reviews?per_page=100&page=1", includedOutput(200, []string{"Date: " + tc.now}, read(t, tc.prefix+"reviews.json")), nil),
			}
			got := runReviewStateStepsFor(t.Context(), t, steps, ReviewStateRequest{
				PR: mustPR(t), Profile: tc.profile, Reviewer: mustLogin(t, tc.reviewer), Requests: tc.requests,
			})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("state = %s\nwant    %s", mustJSON(t, got), mustJSON(t, tc.want))
			}
		})
	}
}

// TestReviewStateDateHeader: GitHub's clock is the Date header of a REST
// response, which must be one IMF-fixdate. Without it the 15-minute limit
// could not be judged, so the read is rejected, and no head is reported when
// it is the PR's own response that lacks it.
func TestReviewStateDateHeader(t *testing.T) {
	t.Parallel()
	pull := `{"head":{"sha":"` + rsHead + `"}}`
	for name, headers := range map[string][]string{
		"missing": nil,
		"invalid": {"Date: yesterday"},
		"twice":   {"Date: " + limitsNow, "Date: " + limitsNow},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			steps := []cmdrunnertestutil.Step{
				repoStep(),
				includeStep("pulls/"+testPR, includedOutput(200, headers, pull), nil),
			}
			g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t, steps...))
			if err != nil {
				t.Fatal(err)
			}
			got, err := g.PRReviewState(t.Context(), ReviewStateRequest{
				PR: mustPR(t), Profile: ProfileCodeRabbit, Reviewer: mustLogin(t, testReviewer), Requests: []string{rsRequestText},
			})
			if err != nil {
				t.Fatal(err)
			}
			if want := rsUnreadable(UnreadableRejected, ""); !reflect.DeepEqual(got, want) {
				t.Errorf("state = %s, want %s", mustJSON(t, got), mustJSON(t, want))
			}
		})
	}
}

// TestReviewStateTolerates: members the tool does not need must not make a
// read unreadable. A check run needs no id, a review of a commit GitHub no
// longer has may carry no commit_id, and a pending review has no
// submitted_at; the finished run still completes the request.
func TestReviewStateTolerates(t *testing.T) {
	t.Parallel()
	finished := rsFinished(11, at("10:00:20"), at("10:03:00"))
	in := rsInput{
		comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, finished)},
		reviews: []rsReview{{login: testReviewer + botLoginSuffix, kind: typenameBot, id: 1}, rsBotReview(2, rsHead, at("10:03:10"))},
		edit: map[string]func(testing.TB, string) string{
			rsStageGraphQL: func(_ testing.TB, s string) string { return strings.Replace(s, `"databaseId":11,`, "", 1) },
			rsStageReviews: func(_ testing.TB, s string) string {
				s = strings.Replace(s, `"commit_id":"",`, `"commit_id":null,`, 1)
				return strings.Replace(s, `"submitted_at":null,`, "", 1)
			},
		},
	}
	want := rsWant(RequestCompleted, withResponse(RunFinished, finished, rsHead), withMarkers(marker(1, rsHead, finished, rsReviewURL(2))))
	checkReviewState(t, in, want)
}

// TestReviewStateCommitPages: the commits are read in pages of twenty. Each
// request after the first carries the cursor of the page before it, a run on a
// later page is read, and a head that differs on any one page, not only the
// first, discards the read.
func TestReviewStateCommitPages(t *testing.T) {
	t.Parallel()
	const commitCount = 45
	finished := rsFinished(11, at("10:00:20"), at("10:03:00"))
	commits := make([]rsCommit, 0, commitCount)
	for range commitCount - 1 {
		commits = append(commits, rsCommitOf(rsHead))
	}
	commits = append(commits, rsCommitOf(rsSecond, finished))
	in := rsInput{comments: defaultComments(), commits: commits}
	call := func(t *testing.T, steps []cmdrunnertestutil.Step) (PRReviewState, *cmdrunnertestutil.Fake) {
		t.Helper()
		fake := cmdrunnertestutil.New(t, steps...)
		g, err := NewGitHubWithoutReviewer(fake)
		if err != nil {
			t.Fatal(err)
		}
		state, err := g.PRReviewState(t.Context(), ReviewStateRequest{
			PR: mustPR(t), Profile: ProfileCodeRabbit, Reviewer: mustLogin(t, testReviewer), Requests: []string{rsRequestText},
		})
		if err != nil {
			t.Fatal(err)
		}
		return state, fake
	}

	t.Run("cursors_and_later_pages", func(t *testing.T) {
		t.Parallel()
		got, fake := call(t, in.steps(t))
		want := rsWant(RequestCompleted, withResponse(RunFinished, finished, rsSecond), withMarkers(marker(1, rsSecond, finished, "")))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("state = %s\nwant    %s", mustJSON(t, got), mustJSON(t, want))
		}
		var after []any
		for _, stdin := range fake.Stdin {
			if stdin != nil {
				after = append(after, requestVariables(t, stdin)["after"])
			}
		}
		if !reflect.DeepEqual(after, []any{nil, "cursor20", "cursor40"}) {
			t.Errorf("cursors sent = %v, want [nil cursor20 cursor40]", after)
		}
	})

	t.Run("head_changed_on_the_second_page", func(t *testing.T) {
		t.Parallel()
		steps := in.steps(t)
		var graphql []int
		for i, step := range steps {
			if slices.Equal(step.Args, []string{ghAPI, "graphql", "--input", "-"}) {
				graphql = append(graphql, i)
			}
		}
		if len(graphql) != 3 {
			t.Fatalf("scripted GraphQL pages = %d, want 3", len(graphql))
		}
		steps[graphql[1]].Out = strings.Replace(steps[graphql[1]].Out, `"headRefOid":"`+rsHead+`"`, `"headRefOid":"`+rsSecond+`"`, 1)
		// The read ends at the second page: the third page and the reviews are
		// never asked for.
		got, _ := call(t, steps[:graphql[1]+1])
		if want := rsUnreadable(UnreadableCall, rsHead); !reflect.DeepEqual(got, want) {
			t.Errorf("state = %s, want %s", mustJSON(t, got), mustJSON(t, want))
		}
	})
}

func withAutoReview(state AutoReviewState, kind RunKind, run rsRun, commit string) func(*PRReviewState) {
	return func(s *PRReviewState) {
		s.AutoReview = &AutoReview{
			State:      state,
			Kind:       kind,
			Status:     run.status,
			Conclusion: run.conclusion,
			Commit:     commit,
			URL:        rsRunURL(run.id),
			StartedAt:  run.started,
		}
	}
}

func sourceryReqComment(id int64, body, clock string) rsComment {
	return rsComment{id: id, login: rsUser, kind: typenameUser, body: body, created: at(clock)}
}

// TestReviewStateSourceryAuto verifies ProfileSourcery automatic review
// reading, four-step priority selection, tie-breaking, unstarted exclusion
// while a manual request is waiting without a response, check-run name
// verification, Profile population on every path, and that automatic
// completions never enter Markers.
func TestReviewStateSourceryAuto(t *testing.T) {
	t.Parallel()
	finished := rsSourceryFinished(21, at("10:01:00"), at("10:03:00"))
	skipped := rsSourcerySkipped(22, at("10:01:30"))
	unrec := rsRun{
		id: 23, name: checkNameSourcery, status: statusCompleted, conclusion: "FAILURE",
		title: "Blocking security issue", started: at("10:01:40"), completed: at("10:02:00"),
	}
	inLimitRunning := rsSourceryRunning(24, at("10:00:30"))
	unstarted := rsSourceryQueued(25)
	timedOutRunning := rsSourceryRunning(26, at("09:49:59")) // 15m01s before rsNow (10:05:00)
	exactLimitRunning := rsSourceryRunning(27, at("09:50:00"))
	boom := errors.New("boom")

	rows := []reviewStateCase{
		{
			name: "no_requests_no_runs",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{{oid: rsHead}}},
			want: rsWantFor(ProfileSourcery, RequestNone),
		},
		{
			name: "empty_queued_check_suite_ignored",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{{oid: rsHead, suites: []rsSuite{{app: rsSourceryReviewer, runs: nil}}}},
			},
			want: rsWantFor(ProfileSourcery, RequestNone),
		},
		{
			name: "auto_running_within_limit",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead, inLimitRunning)}},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoRunning, RunRunning, inLimitRunning, rsHead)),
		},
		{
			name: "auto_running_at_exact_limit",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead, exactLimitRunning)}},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoRunning, RunRunning, exactLimitRunning, rsHead)),
		},
		{
			name: "auto_running_unstarted_startedAt_null",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead, unstarted)}},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoRunning, RunRunning, unstarted, rsHead)),
		},
		{
			name: "auto_timed_out_past_limit",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead, timedOutRunning)}},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoTimedOut, RunRunning, timedOutRunning, rsHead)),
		},
		{
			name: "auto_completed_excluded_from_markers",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead, finished)}},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoCompleted, RunFinished, finished, rsHead)),
		},
		{
			name: "auto_skipped",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead, skipped)}},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoSkipped, RunSkipped, skipped, rsHead)),
		},
		{
			name: "auto_unrecognized_failure",
			in:   rsInput{profile: ProfileSourcery, commits: []rsCommit{rsSourceryCommitOf(rsHead, unrec)}},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoUnrecognized, RunUnknown, unrec, rsHead)),
		},
		{
			name: "ancestor_commit_runs_ignored",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{
					rsSourceryCommitOf(rsSecond, inLimitRunning, finished),
					rsSourceryCommitOf(rsHead),
				},
			},
			want: rsWantFor(ProfileSourcery, RequestNone),
		},
		{
			name: "priority_1_in_limit_running_beats_2_3_4",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{rsSourceryCommitOf(rsHead, finished, timedOutRunning, unstarted, inLimitRunning)},
			},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoRunning, RunRunning, inLimitRunning, rsHead)),
		},
		{
			name: "priority_2_unstarted_beats_3_4",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{rsSourceryCommitOf(rsHead, finished, timedOutRunning, unstarted)},
			},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoRunning, RunRunning, unstarted, rsHead)),
		},
		{
			name: "priority_3_timed_out_running_beats_4",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{rsSourceryCommitOf(rsHead, finished, timedOutRunning)},
			},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoTimedOut, RunRunning, timedOutRunning, rsHead)),
		},
		{
			name: "tie_break_latest_started_wins",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{rsSourceryCommitOf(rsHead,
					rsSourceryFinished(31, at("10:01:00"), at("10:02:00")),
					rsSourcerySkipped(32, at("10:02:30")),
				)},
			},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoSkipped, RunSkipped, rsSourcerySkipped(32, at("10:02:30")), rsHead)),
		},
		{
			name: "tie_break_longest_url_beats_shorter_url",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{rsSourceryCommitOf(rsHead,
					rsSourceryFinished(999, at("10:01:00"), at("10:02:00")),
					rsSourceryFinished(1000, at("10:01:00"), at("10:02:00")),
				)},
			},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoCompleted, RunFinished, rsSourceryFinished(1000, at("10:01:00"), at("10:02:00")), rsHead)),
		},
		{
			name: "tie_break_lex_greatest_url_for_unstarted",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{rsSourceryCommitOf(rsHead, rsSourceryQueued(100), rsSourceryQueued(200))},
			},
			want: rsWantFor(ProfileSourcery, RequestNone, withAutoReview(AutoRunning, RunRunning, rsSourceryQueued(200), rsHead)),
		},
		{
			name: "unstarted_excluded_while_manual_waiting_without_response",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits:  []rsCommit{rsSourceryCommitOf(rsHead, unstarted)},
			},
			want: rsWantFor(ProfileSourcery, RequestWaiting),
		},
		{
			name: "unstarted_excluded_falls_back_to_prior_auto_while_manual_waiting",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits: []rsCommit{rsSourceryCommitOf(rsHead,
					rsSourcerySkipped(20, at("09:55:00")),
					unstarted,
				)},
			},
			want: rsWantFor(ProfileSourcery, RequestWaiting, withAutoReview(AutoSkipped, RunSkipped, rsSourcerySkipped(20, at("09:55:00")), rsHead)),
		},
		{
			name: "unstarted_included_once_manual_has_response",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits:  []rsCommit{rsSourceryCommitOf(rsHead, inLimitRunning, unstarted)},
			},
			want: rsWantFor(ProfileSourcery, RequestWaiting,
				withResponse(RunRunning, inLimitRunning, rsHead),
				withAutoReview(AutoRunning, RunRunning, unstarted, rsHead),
			),
		},
		{
			name: "name_mismatch_rejected_with_profile_set",
			in: rsInput{
				profile: ProfileSourcery,
				commits: []rsCommit{rsSourceryCommitOf(rsHead, rsFinished(21, at("10:01:00"), at("10:03:00")))},
				stop:    rsStageGraphQL,
			},
			want: rsUnreadableFor(ProfileSourcery, UnreadableRejected, rsHead),
		},
		{
			name: "unreadable_preserves_profile_sourcery",
			in:   rsInput{profile: ProfileSourcery, fail: map[string]error{rsStageRepo: boom}},
			want: rsUnreadableFor(ProfileSourcery, UnreadableCall, ""),
		},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkReviewState(t, tc.in, tc.want)
		})
	}
}

// TestReviewStateSourceryManual verifies ProfileSourcery manual request
// matching: multiple accepted request texts, 15-minute response window,
// same/earlier-commit competing runs vs later-commit non-competing runs,
// RequestSkipped, marker construction (manual only), and exclusion of assigned
// runs from AutoReview.
func TestReviewStateSourceryManual(t *testing.T) {
	t.Parallel()
	manualDone := rsSourceryFinished(41, at("10:00:20"), at("10:03:00"))
	manualSkipped := rsSourcerySkipped(42, at("10:00:20"))
	manualUnrec := rsRun{
		id: 43, name: checkNameSourcery, status: statusCompleted, conclusion: "FAILURE",
		title: "Error", started: at("10:00:20"), completed: at("10:01:00"),
	}
	priorAutoSkipped := rsSourcerySkipped(40, at("09:50:00"))
	laterCommitAuto := rsSourceryFinished(44, at("10:01:30"), at("10:02:30"))
	earlierCommitManual := rsSourceryFinished(45, at("10:01:00"), at("10:04:00"))

	rows := []reviewStateCase{
		{
			name: "matches_second_accepted_request_text",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq2, "10:00:00")},
				commits:  []rsCommit{rsSourceryCommitOf(rsHead, manualDone)},
				reviews:  []rsReview{rsSourceryBotReview(10, rsHead, at("10:02:50"))},
			},
			want: rsWantFor(ProfileSourcery, RequestCompleted,
				withResponse(RunFinished, manualDone, rsHead),
				withMarkers(marker(1, rsHead, manualDone, rsReviewURL(10))),
			),
		},
		{
			name: "manual_completed_excludes_assigned_run_from_auto_but_keeps_earlier_auto",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits:  []rsCommit{rsSourceryCommitOf(rsHead, priorAutoSkipped, manualDone)},
			},
			want: rsWantFor(ProfileSourcery, RequestCompleted,
				withResponse(RunFinished, manualDone, rsHead),
				withAutoReview(AutoSkipped, RunSkipped, priorAutoSkipped, rsHead),
				withMarkers(marker(1, rsHead, manualDone, "")),
			),
		},
		{
			name: "manual_skipped",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits:  []rsCommit{rsSourceryCommitOf(rsHead, manualSkipped)},
			},
			want: rsWantFor(ProfileSourcery, RequestSkipped, withResponse(RunSkipped, manualSkipped, rsHead)),
		},
		{
			name: "manual_unrecognized",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits:  []rsCommit{rsSourceryCommitOf(rsHead, manualUnrec)},
			},
			want: rsWantFor(ProfileSourcery, RequestUnrecognized, withResponse(RunUnknown, manualUnrec, rsHead)),
		},
		{
			name: "run_past_15m_response_window_not_stolen_by_expired_manual_request",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits: []rsCommit{rsSourceryCommitOf(rsHead,
					rsSourceryFinished(46, at("10:15:01"), at("10:18:00")),
				)},
				now: at("10:25:00"),
			},
			want: rsWantFor(ProfileSourcery, RequestTimedOut,
				withNow(at("10:25:00")),
				withAutoReview(AutoCompleted, RunFinished, rsSourceryFinished(46, at("10:15:01"), at("10:18:00")), rsHead),
			),
		},
		{
			name: "run_at_exact_15m_response_window_answers_manual_request",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits: []rsCommit{rsSourceryCommitOf(rsHead,
					rsSourceryFinished(47, at("10:15:00"), at("10:18:00")),
				)},
				now: at("10:25:00"),
			},
			want: rsWantFor(ProfileSourcery, RequestCompleted,
				withNow(at("10:25:00")),
				withResponse(RunFinished, rsSourceryFinished(47, at("10:15:00"), at("10:18:00")), rsHead),
				withMarkers(marker(1, rsHead, rsSourceryFinished(47, at("10:15:00"), at("10:18:00")), "")),
			),
		},
		{
			// The response of an edited earlier request is assigned to it, so
			// it is not shown as an automatic review, and it is no marker.
			name: "edited_earlier_request_response_is_not_auto",
			in: rsInput{
				profile: ProfileSourcery,
				comments: []rsComment{
					{id: 1, login: rsUser, kind: typenameUser, body: rsSourceryReq1, created: at("09:50:00"), updated: at("09:55:00")},
					sourceryReqComment(2, rsSourceryReq1, "10:00:00"),
				},
				commits: []rsCommit{rsSourceryCommitOf(rsHead, rsSourceryFinished(81, at("09:50:20"), at("09:53:00")))},
			},
			want: rsWantFor(ProfileSourcery, RequestWaiting, withRequest(2, at("10:00:00"))),
		},
		{
			// Likewise for an edited latest request: indeterminate, and its
			// response is not shown as an automatic review.
			name: "edited_latest_request_response_is_not_auto",
			in: rsInput{
				profile: ProfileSourcery,
				comments: []rsComment{
					{id: 1, login: rsUser, kind: typenameUser, body: rsSourceryReq1, created: at("10:00:00"), updated: at("10:01:00")},
				},
				commits: []rsCommit{rsSourceryCommitOf(rsHead, manualDone)},
			},
			want: rsWantFor(ProfileSourcery, RequestIndeterminate, indeterminate(IndeterminateEditedRequest)),
		},
		{
			name: "same_commit_overlapping_runs_compete",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits: []rsCommit{rsSourceryCommitOf(rsHead,
					rsSourceryFinished(51, at("10:00:20"), at("10:03:00")),
					rsSourceryFinished(52, at("10:02:00"), at("10:04:00")),
				)},
			},
			want: rsWantFor(ProfileSourcery, RequestIndeterminate,
				indeterminate(IndeterminateCompetingRuns, rsRunURL(51), rsRunURL(52)),
				// Neither run was assigned to a manual request because they competed.
				withAutoReview(AutoCompleted, RunFinished, rsSourceryFinished(52, at("10:02:00"), at("10:04:00")), rsHead),
			),
		},
		{
			name: "later_commit_overlapping_auto_run_does_not_compete",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits: []rsCommit{
					rsSourceryCommitOf(rsSecond, manualDone),
					rsSourceryCommitOf(rsHead, laterCommitAuto),
				},
			},
			want: rsWantFor(ProfileSourcery, RequestCompleted,
				withResponse(RunFinished, manualDone, rsSecond),
				withAutoReview(AutoCompleted, RunFinished, laterCommitAuto, rsHead),
				withMarkers(marker(1, rsSecond, manualDone, "")),
			),
		},
		{
			name: "earlier_commit_overlapping_run_competes_when_later_commit_started_first",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits: []rsCommit{
					rsSourceryCommitOf(rsSecond, earlierCommitManual),
					rsSourceryCommitOf(rsHead, manualDone),
				},
			},
			want: rsWantFor(ProfileSourcery, RequestIndeterminate,
				indeterminate(IndeterminateCompetingRuns, rsRunURL(41), rsRunURL(45)),
				withAutoReview(AutoCompleted, RunFinished, manualDone, rsHead),
			),
		},
		{
			name: "same_second_different_commit_competes",
			in: rsInput{
				profile:  ProfileSourcery,
				comments: []rsComment{sourceryReqComment(1, rsSourceryReq1, "10:00:00")},
				commits: []rsCommit{
					rsSourceryCommitOf(rsSecond, rsSourceryFinished(61, at("10:00:20"), at("10:03:00"))),
					rsSourceryCommitOf(rsHead, rsSourceryFinished(62, at("10:00:20"), at("10:03:05"))),
				},
			},
			want: rsWantFor(ProfileSourcery, RequestIndeterminate,
				indeterminate(IndeterminateCompetingRuns, rsRunURL(61), rsRunURL(62)),
				withAutoReview(AutoCompleted, RunFinished, rsSourceryFinished(62, at("10:00:20"), at("10:03:05")), rsHead),
			),
		},
		{
			name: "earlier_manual_marker_preserved_and_head_auto_completed_not_in_markers",
			in: rsInput{
				profile: ProfileSourcery,
				comments: []rsComment{
					sourceryReqComment(1, rsSourceryReq1, "09:00:00"),
					sourceryReqComment(2, rsSourceryReq2, "10:00:00"),
				},
				commits: []rsCommit{
					rsSourceryCommitOf(rsSecond, rsSourceryFinished(71, at("09:00:20"), at("09:02:00"))),
					rsSourceryCommitOf(rsHead, rsSourceryFinished(72, at("09:50:00"), at("09:52:00"))),
				},
			},
			want: rsWantFor(ProfileSourcery, RequestWaiting,
				withRequest(2, at("10:00:00")),
				withAutoReview(AutoCompleted, RunFinished, rsSourceryFinished(72, at("09:50:00"), at("09:52:00")), rsHead),
				withMarkers(marker(1, rsSecond, rsSourceryFinished(71, at("09:00:20"), at("09:02:00")), "")),
			),
		},
		{
			name: "markers_sorted_by_completed_at_then_check_run_url",
			in: rsInput{
				profile: ProfileSourcery,
				comments: []rsComment{
					sourceryReqComment(1, rsSourceryReq1, "09:00:00"),
					sourceryReqComment(2, rsSourceryReq1, "09:01:00"),
					sourceryReqComment(3, rsSourceryReq2, "09:02:00"),
				},
				commits: []rsCommit{
					rsSourceryCommitOf(rsHead,
						rsSourceryFinished(90, at("09:00:20"), at("09:05:00")),
						rsSourceryFinished(92, at("09:01:20"), at("09:03:00")),
						rsSourceryFinished(91, at("09:02:20"), at("09:03:00")),
					),
				},
			},
			want: rsWantFor(ProfileSourcery, RequestCompleted,
				withRequest(3, at("09:02:00")),
				withResponse(RunFinished, rsSourceryFinished(91, at("09:02:20"), at("09:03:00")), rsHead),
				withMarkers(
					marker(3, rsHead, rsSourceryFinished(91, at("09:02:20"), at("09:03:00")), ""),
					marker(2, rsHead, rsSourceryFinished(92, at("09:01:20"), at("09:03:00")), ""),
					marker(1, rsHead, rsSourceryFinished(90, at("09:00:20"), at("09:05:00")), ""),
				),
			),
		},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkReviewState(t, tc.in, tc.want)
		})
	}
}
