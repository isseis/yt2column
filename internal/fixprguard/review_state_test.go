//go:build test

package fixprguard

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

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
	for _, tc := range reviewStateCases() {
		t.Run(tc.name, func(t *testing.T) { checkReviewState(t, tc.in, tc.want) })
	}
}

// TestReviewStateUnreadable: a failed call, a response outside the strict
// shape, and a limit are the unreadable state with their reason. The head is
// set once it was read and checked, and nothing else is kept.
func TestReviewStateUnreadable(t *testing.T) {
	boom := errors.New("boom")
	bad := func(old, replacement string) func(string) string {
		return func(s string) string {
			if !strings.Contains(s, old) {
				t.Fatalf("response %q lacks %q", s, old)
			}
			return strings.Replace(s, old, replacement, 1)
		}
	}
	whole := func(text string) func(string) string { return func(string) string { return text } }
	one := rsInput{comments: defaultComments(), commits: []rsCommit{rsCommitOf(rsHead, rsFinished(11, at("10:00:20"), at("10:03:00")))}}
	with := func(stage string, f func(string) string) rsInput {
		in := one
		in.edit = map[string]func(string) string{stage: f}
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
		{"user_call_fails", failing(rsStageUser, boom), rsUnreadable(UnreadableCall, "")},
		{"pull_call_fails", failing(rsStagePull, boom), rsUnreadable(UnreadableCall, "")},
		{"pull_status_is_not_200", failing(rsStagePull, rsServerError()), rsUnreadable(UnreadableCall, "")},
		{"comments_call_fails", failing(rsStageComments, boom), rsUnreadable(UnreadableCall, rsHead)},
		{"graphql_call_fails", failing(rsStageGraphQL, boom), rsUnreadable(UnreadableCall, rsHead)},
		{"reviews_call_fails", failing(rsStageReviews, boom), rsUnreadable(UnreadableCall, rsHead)},
		{"output_over_the_cap", failing(rsStageGraphQL, cmdrunner.ErrOutputTooLarge), rsUnreadable(UnreadableLimit, rsHead)},

		// The user and the head.
		{"user_login_outside_the_grammar", with(rsStageUser, bad(rsUser, "bad name")), rsUnreadable(UnreadableRejected, "")},
		{"user_login_duplicated", with(rsStageUser, bad(`"id":1`, `"login":"x"`)), rsUnreadable(UnreadableRejected, "")},
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
		{"run_id_zero", with(rsStageGraphQL, bad(`"databaseId":11`, `"databaseId":0`)), rsUnreadable(UnreadableRejected, rsHead)},
		{"run_title_duplicated", with(rsStageGraphQL, bad(`"title":"Finished"`, `"title":"Finished","title":"Finished"`)), rsUnreadable(UnreadableRejected, rsHead)},
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
		t.Run(tc.name, func(t *testing.T) { checkReviewState(t, tc.in, tc.want) })
	}
}

// TestReviewStateLimits: each fixed limit is a boundary. At the limit the
// state is read; one over is the unreadable state with reason limit.
func TestReviewStateLimits(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) { checkReviewState(t, tc.in, tc.want) })
	}
}

// TestReviewStateReads: the commit query asks for every check run named after
// the reviewer (not only the latest of its name), and never for the details
// URL. The wording of the query is the contract with GitHub, so it is matched
// as text.
func TestReviewStateReads(t *testing.T) {
	in := rsInput{
		comments: defaultComments(),
		commits:  []rsCommit{rsCommitOf(rsHead), rsCommitOf(rsSecond)},
	}
	fake := cmdrunnertestutil.New(t, in.steps(t)...)
	g, err := NewGitHubWithoutReviewer(fake)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.PRReviewState(t.Context(), ReviewStateRequest{PR: mustPR(t), Reviewer: mustLogin(t, testReviewer), Request: rsRequestText}); err != nil {
		t.Fatal(err)
	}
	var queries []string
	for _, stdin := range fake.Stdin {
		if stdin == nil {
			continue
		}
		var request struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(stdin, &request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		queries = append(queries, request.Query)
	}
	if len(queries) != 1 {
		t.Fatalf("GraphQL requests = %d, want 1", len(queries))
	}
	for _, want := range []string{`checkName: "CodeRabbit"`, "checkType: ALL", "headRefOid", "checkSuites(first: 50)", "checkRuns(first: 100"} {
		if !strings.Contains(queries[0], want) {
			t.Errorf("query lacks %q", want)
		}
	}
	// The details URL carries a share value of the reviewer's site; it must
	// not be asked for.
	if strings.Contains(strings.ToLower(queries[0]), "detailsurl") {
		t.Error("query asks for detailsUrl")
	}
}

// TestReviewStateOutputHasNoText: no comment body, check run title, details
// URL or PR review body reaches the output, in any state.
func TestReviewStateOutputHasNoText(t *testing.T) {
	secret := "secret-text"
	comments := append(defaultComments(), rsComment{id: 2, login: "alice", kind: typenameUser, body: secret, created: rsCreated})
	titled := rsRun{id: 11, status: statusCompleted, conclusion: conclusionSuccess, title: secret, started: at("10:00:20"), completed: at("10:00:30")}
	for _, in := range []rsInput{
		{comments: comments, commits: []rsCommit{rsCommitOf(rsHead, titled)}, reviews: []rsReview{rsBotReview(1, rsHead, at("10:00:40"))}},
		{comments: comments, commits: []rsCommit{rsCommitOf(rsHead, rsFinished(11, at("10:00:20"), at("10:03:00")))}, reviews: []rsReview{rsBotReview(1, rsHead, at("10:03:10"))}},
		{comments: comments, commits: []rsCommit{rsCommitOf(rsHead)}, edit: map[string]func(string) string{rsStageGraphQL: func(string) string { return "{" }}, stop: rsStageGraphQL},
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
	good := ReviewStateRequest{PR: mustPR(t), Reviewer: mustLogin(t, testReviewer), Request: rsRequestText}
	for name, req := range map[string]ReviewStateRequest{
		"zero_pr":       {Reviewer: good.Reviewer, Request: good.Request},
		"zero_reviewer": {PR: good.PR, Request: good.Request},
		"empty_request": {PR: good.PR, Reviewer: good.Reviewer},
		"long_request":  {PR: good.PR, Reviewer: good.Reviewer, Request: strings.Repeat("x", maxRequestTextLength+1)},
	} {
		t.Run(name, func(t *testing.T) {
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
	for name, marshal := range map[string]func() ([]byte, error){
		"request_state":        RequestState(99).MarshalText,
		"unreadable_reason":    UnreadableReason(99).MarshalText,
		"indeterminate_reason": IndeterminateReason(99).MarshalText,
		"run_kind":             RunKind(99).MarshalText,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := marshal(); !errors.Is(err, errUnknownEnum) {
				t.Errorf("err = %v, want errUnknownEnum", err)
			}
		})
	}
}

// TestReviewStateObserved: a sequence of three requests in one PR, shaped after
// the responses observed in the probe PR (the SHAs, ids and the request text
// are synthetic; the members, the check run triples and the order of the runs
// are as observed), read from files: a finished review, a rate-limited one, and a finished one with a PR
// review. Unread members and an unread body stay out of the result.
func TestReviewStateObserved(t *testing.T) {
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join("testdata", "review_state", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(data)
	}
	const head = "f24ee60a00000000000000000000000000000000"
	now := "Fri, 20 Nov 2026 04:35:00 GMT"
	steps := []cmdrunnertestutil.Step{
		repoStep(),
		{Name: "gh", Args: []string{ghAPI, ghUserEndpoint}, Out: `{"login":"isseis"}`},
		includeStep("pulls/"+testPR, includedOutput(200, []string{"Date: " + now}, read("pull.json")), nil),
		includeStep("issues/"+testPR+"/comments?per_page=100&page=1", includedOutput(200, []string{"Date: " + now}, read("comments.json")), nil),
		graphqlStep(read("commits.json")),
		includeStep("pulls/"+testPR+"/reviews?per_page=100&page=1", includedOutput(200, []string{"Date: " + now}, read("reviews.json")), nil),
	}
	g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t, steps...))
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.PRReviewState(t.Context(), ReviewStateRequest{PR: mustPR(t), Reviewer: mustLogin(t, testReviewer), Request: rsRequestText})
	if err != nil {
		t.Fatal(err)
	}
	const (
		first  = "7cefaa5e00000000000000000000000000000000"
		runURL = "https://github.com/isseis/yt2column/runs/"
	)
	want := newReviewState(RequestCompleted)
	want.Head, want.Now = head, "2026-11-20T04:35:00Z"
	want.Request = &ReviewRequest{ID: 7030003, URL: "https://github.com/isseis/yt2column/pull/1064#issuecomment-7030003", CreatedAt: "2026-11-20T04:26:01Z"}
	want.Response = &ReviewResponse{
		Kind: RunFinished, Status: statusCompleted, Conclusion: conclusionSuccess, Commit: head,
		URL: runURL + "64000005", StartedAt: "2026-11-20T04:26:25Z",
	}
	want.Markers = []ReviewMarker{
		{
			Commit: first, RequestID: 7030001, RequestURL: "https://github.com/isseis/yt2column/pull/1064#issuecomment-7030001",
			CheckRunURL: runURL + "64000001", CompletedAt: "2026-11-20T03:22:24Z",
		},
		{
			Commit: head, RequestID: 7030003, RequestURL: "https://github.com/isseis/yt2column/pull/1064#issuecomment-7030003",
			CheckRunURL: runURL + "64000005", ReviewURL: "https://github.com/isseis/yt2column/pull/1064#pullrequestreview-4638479000",
			CompletedAt: "2026-11-20T04:29:18Z",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("state = %s\nwant    %s", mustJSON(t, got), mustJSON(t, want))
	}
}

// TestReviewStateDateHeader: GitHub's clock is the Date header of a REST
// response, which must be one IMF-fixdate. Without it the 15-minute limit
// could not be judged, so the read is rejected, and no head is reported when
// it is the PR's own response that lacks it.
func TestReviewStateDateHeader(t *testing.T) {
	pull := `{"head":{"sha":"` + rsHead + `"}}`
	for name, headers := range map[string][]string{
		"missing": nil,
		"invalid": {"Date: yesterday"},
		"twice":   {"Date: " + limitsNow, "Date: " + limitsNow},
	} {
		t.Run(name, func(t *testing.T) {
			steps := []cmdrunnertestutil.Step{
				repoStep(),
				{Name: "gh", Args: []string{ghAPI, ghUserEndpoint}, Out: `{"login":"` + rsUser + `"}`},
				includeStep("pulls/"+testPR, includedOutput(200, headers, pull), nil),
			}
			g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t, steps...))
			if err != nil {
				t.Fatal(err)
			}
			got, err := g.PRReviewState(t.Context(), ReviewStateRequest{PR: mustPR(t), Reviewer: mustLogin(t, testReviewer), Request: rsRequestText})
			if err != nil {
				t.Fatal(err)
			}
			if want := rsUnreadable(UnreadableRejected, ""); !reflect.DeepEqual(got, want) {
				t.Errorf("state = %s, want %s", mustJSON(t, got), mustJSON(t, want))
			}
		})
	}
}

// TestReviewStateCommitPages: the commits are read in pages of twenty. Each
// request after the first carries the cursor of the page before it, a run on a
// later page is read, and a head that differs on any one page, not only the
// first, discards the read.
func TestReviewStateCommitPages(t *testing.T) {
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
		state, err := g.PRReviewState(t.Context(), ReviewStateRequest{PR: mustPR(t), Reviewer: mustLogin(t, testReviewer), Request: rsRequestText})
		if err != nil {
			t.Fatal(err)
		}
		return state, fake
	}

	t.Run("cursors_and_later_pages", func(t *testing.T) {
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
