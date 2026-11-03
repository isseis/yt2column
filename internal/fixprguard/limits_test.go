//go:build test

package fixprguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

const (
	// limitsNow is GitHub's clock in the default response: Tuesday,
	// 2026-11-10T00:00:00Z.
	limitsNow = "Tue, 10 Nov 2026 00:00:00 GMT"
	// limitsExpires is the observed expiry of a limit set at
	// 2026-11-02T21:30:10Z.
	limitsExpires = "2027-05-02T21:30:10Z"
	// limitsCreated is a PR created after that limit was set.
	limitsCreated = "2026-11-05T00:00:00Z"
)

// limitsBody is an interaction-limits body with an unused origin member, as
// GitHub sends it.
func limitsBody(limit, expires string) string {
	return `{"limit":"` + limit + `","origin":"user","expires_at":"` + expires + `"}`
}

// limitsOK is a successful --include output with a Date header and an unused
// header.
func limitsOK(date, body string) string {
	return includedOutput(200, []string{"Date: " + date, "Content-Type: application/json; charset=utf-8"}, body)
}

func createdAt(at string) string { return `{"createdAt":"` + at + `"}` }

func unverifiable(reason UnverifiableReason) InteractionLimitsInfo {
	return InteractionLimitsInfo{State: InteractionUnverifiable, Reason: reason}
}

func limitsState(state InteractionState, expiresOn string) InteractionLimitsInfo {
	return InteractionLimitsInfo{State: state, ExpiresOn: expiresOn}
}

// limitsCase scripts the three calls. A stage with an error ends the calls:
// the later stages are not scripted, so the fake fails the test if they run.
type limitsCase struct {
	name       string
	repoErr    error
	created    string // createdAt output; createdAt(limitsCreated) when empty
	createdErr error
	badCreated bool   // the createdAt output is rejected, so the API is not called
	out        string // interaction-limits output
	outErr     error
	expired    bool // run with a context whose deadline has passed
	want       InteractionLimitsInfo
}

func (tc limitsCase) steps() []cmdrunnertestutil.Step {
	repo := repoStep()
	repo.Err = tc.repoErr
	steps := []cmdrunnertestutil.Step{repo}
	if tc.repoErr != nil {
		return steps
	}
	created := tc.created
	if created == "" {
		created = createdAt(limitsCreated)
	}
	steps = append(steps, createdAtStep(created, tc.createdErr))
	if tc.createdErr != nil || tc.badCreated {
		return steps
	}
	return append(steps, interactionLimitsStep(tc.out, tc.outErr))
}

func runLimits(t *testing.T, tc limitsCase) InteractionLimitsInfo {
	t.Helper()
	g, _ := newTestGitHub(t, tc.steps()...)
	ctx := t.Context()
	if tc.expired {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
		t.Cleanup(cancel)
	}
	return g.InteractionLimits(ctx, mustPR(t))
}

// limitsCases are every row of TestInteractionLimits; TestInteractionLimitsLists
// checks that they reach every state and reason.
func limitsCases() []limitsCase {
	ok := func(body string) string { return limitsOK(limitsNow, body) }
	active := limitsState(InteractionActive, "2027-05-02")
	exitWith := func(stdout string) error {
		return &cmdrunner.ExitError{Code: ghExitFailure, Stdout: []byte(stdout), Stderr: []byte("gh: request failed")}
	}
	// padded is a body of exactly n bytes, padded inside an unused member.
	padded := func(n int) string {
		head := `{"limit":"collaborators_only","expires_at":"` + limitsExpires + `","origin":"`
		return head + strings.Repeat("u", n-len(head)-len(`"}`)) + `"}`
	}
	return []limitsCase{
		// Accepted shapes.
		{name: "no_limit", out: ok(noLimitBody), want: limitsState(InteractionNone, "")},
		{name: "collaborators_only", out: ok(limitsBody("collaborators_only", limitsExpires)), want: active},
		{name: "existing_users", out: ok(limitsBody("existing_users", limitsExpires)), want: limitsState(InteractionWeak, "2027-05-02")},
		{name: "contributors_only", out: ok(limitsBody("contributors_only", limitsExpires)), want: limitsState(InteractionWeak, "2027-05-02")},
		{
			name: "unused_members_and_their_duplicates",
			out:  ok(`{"origin":"user","limit":"collaborators_only","origin":"repository","expires_at":"` + limitsExpires + `","x":[1]}`),
			want: active,
		},
		{name: "fractional_seconds", out: ok(limitsBody("collaborators_only", "2027-05-02T21:30:10.123456Z")), want: active},
		{name: "body_at_size_cap", out: ok(padded(maxLimitsBodyBytes)), want: active},
		{name: "lower_case_date_header", out: includedOutput(200, []string{"date: " + limitsNow}, limitsBody("collaborators_only", limitsExpires)), want: active},
		{name: "upper_case_date_header", out: includedOutput(200, []string{"DATE: " + limitsNow}, limitsBody("collaborators_only", limitsExpires)), want: active},
		// The expiry in another offset: ExpiresOn is the UTC date.
		{name: "expires_with_offset", out: ok(limitsBody("collaborators_only", "2027-05-03T06:30:10+09:00")), want: active},

		// Expiry boundaries against the Date header.
		{name: "expires_at_now", out: ok(limitsBody("collaborators_only", "2026-11-10T00:00:00Z")), want: limitsState(InteractionNone, "")},
		{name: "expires_in_14_days", out: ok(limitsBody("collaborators_only", "2026-11-24T00:00:00Z")), want: limitsState(InteractionExpiring, "2026-11-24")},
		{name: "expires_in_14_days_and_a_second", out: ok(limitsBody("collaborators_only", "2026-11-24T00:00:01Z")), want: limitsState(InteractionActive, "2026-11-24")},

		// Boundaries of the time the limit was set: six calendar months before
		// expires_at.
		{name: "created_when_set", created: createdAt("2026-11-02T21:30:10Z"), out: ok(limitsBody("collaborators_only", limitsExpires)), want: active},
		{
			name: "created_a_second_before_set", created: createdAt("2026-11-02T21:30:09Z"),
			out: ok(limitsBody("collaborators_only", limitsExpires)), want: limitsState(InteractionPredates, "2027-05-02"),
		},
		// expires_at on the last day of its month: set on the last day of the
		// earlier month.
		{name: "month_end_created_when_set", created: createdAt("2026-08-31T00:00:00Z"), out: ok(limitsBody("collaborators_only", "2027-02-28T00:00:00Z")), want: limitsState(InteractionActive, "2027-02-28")},
		{
			name: "month_end_created_a_second_before", created: createdAt("2026-08-30T23:59:59Z"),
			out: ok(limitsBody("collaborators_only", "2027-02-28T00:00:00Z")), want: limitsState(InteractionPredates, "2027-02-28"),
		},
		// expires_at on August 30, a day February does not have: set on the
		// last day of February, not rolled into March.
		{
			name: "missing_day_created_when_set", created: createdAt("2027-02-28T00:00:00Z"),
			out: limitsOK("Tue, 02 Mar 2027 00:00:00 GMT", limitsBody("collaborators_only", "2027-08-30T00:00:00Z")), want: limitsState(InteractionActive, "2027-08-30"),
		},
		{
			name: "missing_day_created_a_second_before", created: createdAt("2027-02-27T23:59:59Z"),
			out: limitsOK("Tue, 02 Mar 2027 00:00:00 GMT", limitsBody("collaborators_only", "2027-08-30T00:00:00Z")), want: limitsState(InteractionPredates, "2027-08-30"),
		},
		{
			name: "missing_day_created_on_march_1", created: createdAt("2027-03-01T00:00:00Z"),
			out: limitsOK("Tue, 02 Mar 2027 00:00:00 GMT", limitsBody("collaborators_only", "2027-08-30T00:00:00Z")), want: limitsState(InteractionActive, "2027-08-30"),
		},

		// The order of the states: each input meets two, and the earlier wins.
		{name: "expired_and_weak", out: ok(limitsBody("existing_users", "2026-11-09T23:59:59Z")), want: limitsState(InteractionNone, "")},
		{
			name: "weak_and_predates", created: createdAt("2026-01-01T00:00:00Z"),
			out: ok(limitsBody("contributors_only", limitsExpires)), want: limitsState(InteractionWeak, "2027-05-02"),
		},
		{
			name: "predates_and_expiring", created: createdAt("2026-05-19T00:00:00Z"),
			out: ok(limitsBody("collaborators_only", "2026-11-20T00:00:00Z")), want: limitsState(InteractionPredates, "2026-11-20"),
		},
		{
			name: "expired_and_predates", created: createdAt("2026-01-01T00:00:00Z"),
			out: ok(limitsBody("collaborators_only", "2026-11-09T00:00:00Z")), want: limitsState(InteractionNone, ""),
		},

		// The body: the rejection rules of the requirements, one per row.
		{name: "invalid_utf8", out: ok("{\"limit\":\"collaborators_only\xff\",\"expires_at\":\"" + limitsExpires + "\"}"), want: unverifiable(UnverifiableBody)},
		{name: "unpaired_surrogate", out: ok(`{"limit":"collaborators_only","expires_at":"` + limitsExpires + `","origin":"\ud800"}`), want: unverifiable(UnverifiableBody)},
		{name: "array", out: ok(`[` + limitsBody("collaborators_only", limitsExpires) + `]`), want: unverifiable(UnverifiableBody)},
		{name: "null", out: ok(`null`), want: unverifiable(UnverifiableBody)},
		{name: "string", out: ok(`"collaborators_only"`), want: unverifiable(UnverifiableBody)},
		{name: "trailing_data", out: ok(limitsBody("collaborators_only", limitsExpires) + `x`), want: unverifiable(UnverifiableBody)},
		{name: "second_value", out: ok(limitsBody("collaborators_only", limitsExpires) + `{}`), want: unverifiable(UnverifiableBody)},
		{name: "limit_missing", out: ok(`{"expires_at":"` + limitsExpires + `"}`), want: unverifiable(UnverifiableBody)},
		{name: "expires_at_missing", out: ok(`{"limit":"collaborators_only"}`), want: unverifiable(UnverifiableBody)},
		{name: "limit_duplicate", out: ok(`{"limit":"existing_users","limit":"collaborators_only","expires_at":"` + limitsExpires + `"}`), want: unverifiable(UnverifiableBody)},
		{name: "expires_at_duplicate", out: ok(`{"limit":"collaborators_only","expires_at":"2026-01-01T00:00:00Z","expires_at":"` + limitsExpires + `"}`), want: unverifiable(UnverifiableBody)},
		{name: "limit_null", out: ok(`{"limit":null,"expires_at":"` + limitsExpires + `"}`), want: unverifiable(UnverifiableBody)},
		{name: "limit_number", out: ok(`{"limit":1,"expires_at":"` + limitsExpires + `"}`), want: unverifiable(UnverifiableBody)},
		{name: "expires_at_null", out: ok(`{"limit":"collaborators_only","expires_at":null}`), want: unverifiable(UnverifiableBody)},
		{name: "expires_at_number", out: ok(`{"limit":"collaborators_only","expires_at":1793223010}`), want: unverifiable(UnverifiableBody)},
		{name: "body_over_size_cap", out: ok(padded(maxLimitsBodyBytes + 1)), want: unverifiable(UnverifiableBody)},
		{name: "empty_body", out: ok(""), want: unverifiable(UnverifiableBody)},
		{name: "unknown_limit", out: ok(limitsBody("everyone", limitsExpires)), want: unverifiable(UnverifiableLimit)},
		{name: "upper_case_limit", out: ok(limitsBody("COLLABORATORS_ONLY", limitsExpires)), want: unverifiable(UnverifiableLimit)},
		{name: "expires_at_without_offset", out: ok(limitsBody("collaborators_only", "2027-05-02T21:30:10")), want: unverifiable(UnverifiableExpiresAt)},
		{name: "expires_at_date_only", out: ok(limitsBody("collaborators_only", "2027-05-02")), want: unverifiable(UnverifiableExpiresAt)},
		// No partial result: a past expiry read beside an unknown limit is
		// not taken as no limit.
		{name: "unknown_limit_with_past_expiry", out: ok(limitsBody("everyone", "2026-01-01T00:00:00Z")), want: unverifiable(UnverifiableLimit)},

		// The status and the shape of the --include output.
		{name: "status_204", out: includedOutput(204, []string{"Date: " + limitsNow}, ""), want: InteractionLimitsInfo{Reason: UnverifiableHTTPStatus, HTTPStatus: 204}},
		{name: "no_empty_line", out: "HTTP/2.0 200 OK\nDate: " + limitsNow + "\r\n", want: unverifiable(UnverifiableOutputForm)},
		{name: "no_status_line", out: "{}", want: unverifiable(UnverifiableOutputForm)},
		{name: "output_over_size_cap", out: ok(strings.Repeat(" ", maxResponseBytes)), want: unverifiable(UnverifiableOutputForm)},
		{name: "date_missing", out: includedOutput(200, []string{"Content-Length: 2"}, noLimitBody), want: unverifiable(UnverifiableDate)},
		{name: "date_twice", out: includedOutput(200, []string{"Date: " + limitsNow, "Date: " + limitsNow}, noLimitBody), want: unverifiable(UnverifiableDate)},
		{name: "date_joined", out: limitsOK(limitsNow+", "+limitsNow, noLimitBody), want: unverifiable(UnverifiableDate)},
		{name: "date_rfc850", out: limitsOK("Tuesday, 10-Nov-26 00:00:00 GMT", noLimitBody), want: unverifiable(UnverifiableDate)},
		{name: "date_ansi_c", out: limitsOK("Tue Nov 10 00:00:00 2026", noLimitBody), want: unverifiable(UnverifiableDate)},
		{name: "date_other_zone", out: limitsOK("Tue, 10 Nov 2026 00:00:00 JST", noLimitBody), want: unverifiable(UnverifiableDate)},

		// A failed gh api call.
		{name: "gh_failed_with_status", outErr: exitWith(includedOutput(403, nil, `{"message":"secret-marker"}`)), want: InteractionLimitsInfo{Reason: UnverifiableHTTPStatus, HTTPStatus: 403}},
		{name: "gh_failed_without_status", outErr: exitWith("secret-marker"), want: unverifiable(UnverifiableGHFailed)},
		{name: "gh_not_started", outErr: errors.New("start gh"), want: unverifiable(UnverifiableGHFailed)},
		{name: "gh_output_too_large", outErr: fmt.Errorf("gh: %w", cmdrunner.ErrOutputTooLarge), want: unverifiable(UnverifiableOutputForm)},

		// The repository and the PR.
		{name: "repository_failed", repoErr: exitWith(""), want: unverifiable(UnverifiableRepository)},
		{name: "pr_view_failed", createdErr: exitWith(""), want: unverifiable(UnverifiablePR)},
		{name: "created_at_missing", badCreated: true, created: `{"number":42}`, want: unverifiable(UnverifiablePR)},
		{name: "created_at_duplicate", badCreated: true, created: `{"createdAt":"2026-01-01T00:00:00Z","createdAt":"` + limitsCreated + `"}`, want: unverifiable(UnverifiablePR)},
		{name: "created_at_number", badCreated: true, created: `{"createdAt":1}`, want: unverifiable(UnverifiablePR)},
		{name: "created_at_not_rfc3339", badCreated: true, created: `{"createdAt":"2026-11-05 00:00:00"}`, want: unverifiable(UnverifiablePR)},

		// The shared deadline: a stage that fails once it has passed timed
		// out, whatever it returned; before it has, the stage's own reason
		// stands.
		{name: "timeout_at_repository", expired: true, repoErr: exitWith(""), want: unverifiable(UnverifiableTimeout)},
		{name: "timeout_at_pr", expired: true, createdErr: exitWith(""), want: unverifiable(UnverifiableTimeout)},
		{name: "timeout_at_api", expired: true, outErr: exitWith(includedOutput(403, nil, "")), want: unverifiable(UnverifiableTimeout)},
		{name: "deadline_error_before_deadline_at_repository", repoErr: fmt.Errorf("gh: %w", context.DeadlineExceeded), want: unverifiable(UnverifiableRepository)},
		{name: "deadline_error_before_deadline_at_api", outErr: fmt.Errorf("gh: %w", context.DeadlineExceeded), want: unverifiable(UnverifiableGHFailed)},
	}
}

// TestInteractionLimits checks all four values of every result. The rows that
// put "secret-marker" into gh's output also check that no value of the result
// carries it.
func TestInteractionLimits(t *testing.T) {
	for _, tc := range limitsCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := runLimits(t, tc)
			if got != tc.want {
				t.Errorf("InteractionLimits = %+v, want %+v", got, tc.want)
			}
			out, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("json.Marshal = %v", err)
			}
			if strings.Contains(string(out), "secret-marker") {
				t.Errorf("output %s carries the response", out)
			}
		})
	}
}

// TestInteractionLimitsLists checks that the rows reach every state and every
// reason, and that every rejection names a listed reason.
func TestInteractionLimitsLists(t *testing.T) {
	states := map[InteractionState]struct{}{}
	reasons := map[UnverifiableReason]struct{}{}
	for _, tc := range limitsCases() {
		states[tc.want.State] = struct{}{}
		if tc.want.State == InteractionUnverifiable {
			if !slices.Contains(UnverifiableReasons(), tc.want.Reason) {
				t.Errorf("row %s: reason %q is not listed", tc.name, tc.want.Reason)
			}
			reasons[tc.want.Reason] = struct{}{}
		} else if tc.want.Reason != "" || tc.want.HTTPStatus != 0 {
			t.Errorf("row %s: state %v with reason %q, status %d", tc.name, tc.want.State, tc.want.Reason, tc.want.HTTPStatus)
		}
	}
	if got := slices.Sorted(maps.Keys(states)); !slices.Equal(got, InteractionStates()) {
		t.Errorf("rows reach states %v, want %v", got, InteractionStates())
	}
	want := slices.Sorted(slices.Values(UnverifiableReasons()))
	if got := slices.Sorted(maps.Keys(reasons)); !slices.Equal(got, want) {
		t.Errorf("rows reach reasons %v, want %v", got, want)
	}
}

func TestInteractionStateText(t *testing.T) {
	names := map[string]struct{}{}
	for _, s := range InteractionStates() {
		text, err := s.MarshalText()
		if err != nil || len(text) == 0 {
			t.Errorf("MarshalText(%d) = %q, %v", s, text, err)
		}
		names[string(text)] = struct{}{}
	}
	if len(names) != len(InteractionStates()) {
		t.Errorf("state names %v are not distinct", names)
	}
	if _, err := InteractionState(len(InteractionStates())).MarshalText(); !errors.Is(err, errUnknownEnum) {
		t.Errorf("MarshalText of a value past the list = %v, want errUnknownEnum", err)
	}
	// A result whose state was never set reports unverifiable.
	out, err := json.Marshal(InteractionLimitsInfo{})
	if err != nil || !strings.Contains(string(out), `"state":"unverifiable"`) {
		t.Errorf("zero InteractionLimitsInfo = %s, %v; want state unverifiable", out, err)
	}
}

// TestLimitsStageErrors: a stage's failure is ErrResponseRejected.
func TestLimitsStageErrors(t *testing.T) {
	_, failure := parseLimitsResponse([]byte(limitsOK(limitsNow, `null`)))
	if failure == nil || !errors.Is(failure, ErrResponseRejected) {
		t.Errorf("parseLimitsResponse = %v, want ErrResponseRejected", failure)
	}
}

// TestInteractionLimitsSharedDeadline: the three calls share one deadline, no
// later than githubTimeout after InteractionLimits returned. Each call's own
// githubTimeout would give the later calls later deadlines.
func TestInteractionLimitsSharedDeadline(t *testing.T) {
	tc := limitsCase{out: limitsOK(limitsNow, limitsBody("collaborators_only", limitsExpires))}
	runner := &deadlineRunner{Fake: cmdrunnertestutil.New(t, tc.steps()...)}
	g, err := NewGitHubWithoutReviewer(runner)
	if err != nil {
		t.Fatalf("NewGitHubWithoutReviewer = %v", err)
	}
	if got := g.InteractionLimits(t.Context(), mustPR(t)); got.State != InteractionActive {
		t.Fatalf("InteractionLimits = %+v, want active", got)
	}
	end := time.Now()
	if len(runner.at) != 3 || runner.at[0].IsZero() {
		t.Fatalf("deadlines = %v, want three", runner.at)
	}
	for _, at := range runner.at[1:] {
		if !at.Equal(runner.at[0]) {
			t.Errorf("deadlines = %v, want one shared deadline", runner.at)
		}
	}
	if runner.at[0].After(end.Add(githubTimeout)) {
		t.Errorf("deadline %v is past %v after the end %v", runner.at[0], githubTimeout, end)
	}
}
