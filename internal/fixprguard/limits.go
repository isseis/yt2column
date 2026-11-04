package fixprguard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

// Interaction limits: /runpr reads the repository's interaction limits once per
// run and stops unless they hold. A response is accepted in two shapes only, the
// no-limit body "{}" and a strict JSON object with a known limit and an RFC 3339
// expires_at, both with HTTP status 200 and one IMF-fixdate Date header. Every
// other response, and every failed call, is the unverifiable state: nothing is
// repaired and no value read before the failing stage is used.

// InteractionState is the state of the repository's interaction limits that
// /runpr reports. The zero value is InteractionUnverifiable, the state that
// assumes least: a state left unset is reported as unverifiable, never as
// active.
type InteractionState int

// The states, in the order /runpr's requirements list them.
const (
	InteractionUnverifiable InteractionState = iota
	InteractionNone
	InteractionWeak
	InteractionPredates
	InteractionExpiring
	InteractionActive
)

// MarshalText names the state. A value outside the list is an error.
func (s InteractionState) MarshalText() ([]byte, error) {
	switch s {
	case InteractionUnverifiable:
		return []byte("unverifiable"), nil
	case InteractionNone:
		return []byte("none"), nil
	case InteractionWeak:
		return []byte("weak"), nil
	case InteractionPredates:
		return []byte("predates"), nil
	case InteractionExpiring:
		return []byte("expiring"), nil
	case InteractionActive:
		return []byte("active"), nil
	default:
		return nil, fmt.Errorf("%w: interaction state", errUnknownEnum)
	}
}

// InteractionStates lists every state; the guide's document guard names each
// through MarshalText.
func InteractionStates() []InteractionState {
	return []InteractionState{
		InteractionUnverifiable, InteractionNone, InteractionWeak,
		InteractionPredates, InteractionExpiring, InteractionActive,
	}
}

// UnverifiableReason names the stage that rejected the response. The zero
// value is the empty name, used for every state but unverifiable.
type UnverifiableReason string

// The reasons, one per stage that can reject.
const (
	// UnverifiableRepository: gh repo view failed or its answer was rejected.
	UnverifiableRepository UnverifiableReason = "repository"
	// UnverifiablePR: the PR's createdAt could not be read.
	UnverifiablePR UnverifiableReason = "pr"
	// UnverifiableGHFailed: gh failed and printed no readable status line.
	UnverifiableGHFailed UnverifiableReason = "gh-failed"
	// UnverifiableTimeout: the calls ran past githubTimeout in all.
	UnverifiableTimeout UnverifiableReason = "timeout"
	// UnverifiableOutputForm: the --include output is not in the accepted
	// shape, or is over the size cap.
	UnverifiableOutputForm UnverifiableReason = "output-form"
	// UnverifiableHTTPStatus: the HTTP status is not 200; HTTPStatus holds it.
	UnverifiableHTTPStatus UnverifiableReason = "http-status"
	// UnverifiableDate: no Date header, more than one, or one that is not an
	// IMF-fixdate.
	UnverifiableDate UnverifiableReason = "date"
	// UnverifiableBody: the body is over maxLimitsBodyBytes or fails the
	// strict JSON check.
	UnverifiableBody UnverifiableReason = "body"
	// UnverifiableLimit: limit is not one of the three known values.
	UnverifiableLimit UnverifiableReason = "limit"
	// UnverifiableExpiresAt: expires_at is not an RFC 3339 date-time.
	UnverifiableExpiresAt UnverifiableReason = "expires-at"
)

// UnverifiableReasons lists every non-empty reason, for the guide's document
// guard.
func UnverifiableReasons() []UnverifiableReason {
	return []UnverifiableReason{
		UnverifiableRepository, UnverifiablePR, UnverifiableGHFailed, UnverifiableTimeout,
		UnverifiableOutputForm, UnverifiableHTTPStatus, UnverifiableDate, UnverifiableBody,
		UnverifiableLimit, UnverifiableExpiresAt,
	}
}

// InteractionLimitsInfo is the output of interaction-limits. ExpiresOn is the
// UTC date of expires_at for weak, predates, expiring and active, and empty
// otherwise. Reason is set for unverifiable only, and HTTPStatus only when
// Reason is http-status. None of them carries a value copied from the response
// body or a header.
type InteractionLimitsInfo struct {
	Protocol   int                `json:"protocol"`
	State      InteractionState   `json:"state"`
	ExpiresOn  string             `json:"expiresOn"`
	Reason     UnverifiableReason `json:"reason"`
	HTTPStatus int                `json:"httpStatus"`
}

const (
	// maxLimitsBodyBytes caps the body of an interaction-limits response.
	maxLimitsBodyBytes = 64 << 10
	// noLimitBody is the exact body GitHub returns when no limit is set.
	noLimitBody = "{}"
	// expiringWithin is how close the expiry may come before expiring: a
	// length of time (14 days of 24 hours), not calendar days.
	expiringWithin = 14 * 24 * time.Hour
	// limitPeriodMonths is the period the user always sets the limit for; the
	// time it was set is estimated by subtracting it from expires_at.
	limitPeriodMonths = 6
	// expiresOnLayout formats ExpiresOn.
	expiresOnLayout = time.DateOnly
	// dateHeader is the only header read.
	dateHeader = "Date"

	limitCollaboratorsOnly = "collaborators_only"
	limitExistingUsers     = "existing_users"
	limitContributorsOnly  = "contributors_only"
)

// limitsError is a stage's failure. It wraps ErrResponseRejected and declares
// its reason as a value, so the reason is never inferred from an error's text.
// status is the HTTP status for UnverifiableHTTPStatus and zero otherwise.
type limitsError struct {
	reason UnverifiableReason
	status int
}

func (e *limitsError) Error() string {
	return fmt.Sprintf("%s: interaction limits: %s", ErrResponseRejected, e.reason)
}

func (e *limitsError) Unwrap() error { return ErrResponseRejected }

func failAt(reason UnverifiableReason) *limitsError { return &limitsError{reason: reason} }

// limitsResponse is an accepted interaction-limits response. none is the
// no-limit body; limit and expires are unset then.
type limitsResponse struct {
	now     time.Time
	none    bool
	limit   string
	expires time.Time
}

// InteractionLimits reads the repository's interaction limits and the PR's
// creation time, and classifies them against GitHub's clock, within
// githubTimeout in all. It never returns an error: every failure is the
// unverifiable state with its reason, and no partial result (an expiry date
// read before a later stage failed) is kept. Protocol is left for the caller
// to set. It judges no thread, so a GitHub without reviewers may call it.
func (g *GitHub) InteractionLimits(ctx context.Context, pr PRNumber) InteractionLimitsInfo {
	// One deadline covers the three calls; runGH's own bound is no later.
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()
	created, resp, failure := g.readInteractionLimits(ctx, pr)
	if failure != nil {
		// A stage that failed after the shared deadline passed timed out,
		// whatever error it returned.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return InteractionLimitsInfo{State: InteractionUnverifiable, Reason: UnverifiableTimeout}
		}
		return InteractionLimitsInfo{State: InteractionUnverifiable, Reason: failure.reason, HTTPStatus: failure.status}
	}
	return classifyLimits(resp, created)
}

// readInteractionLimits runs the three stages in order and stops at the first
// failure.
func (g *GitHub) readInteractionLimits(ctx context.Context, pr PRNumber) (time.Time, limitsResponse, *limitsError) {
	if !pr.usable() {
		return time.Time{}, limitsResponse{}, failAt(UnverifiablePR)
	}
	repo, err := g.resolveRepository(ctx)
	if err != nil {
		return time.Time{}, limitsResponse{}, failAt(UnverifiableRepository)
	}
	created, err := g.prCreatedAt(ctx, pr)
	if err != nil {
		return time.Time{}, limitsResponse{}, failAt(UnverifiablePR)
	}
	out, err := g.runGH(ctx, nil, "api", "--include", "repos/"+repo.owner+"/"+repo.name+"/interaction-limits")
	if err != nil {
		return time.Time{}, limitsResponse{}, limitsCallFailure(err)
	}
	resp, failure := parseLimitsResponse(out)
	if failure != nil {
		return time.Time{}, limitsResponse{}, failure
	}
	return created, resp, nil
}

// prCreatedAt reads the PR's createdAt, an RFC 3339 date-time.
func (g *GitHub) prCreatedAt(ctx context.Context, pr PRNumber) (time.Time, error) {
	value, err := g.prViewString(ctx, pr, createdAtMember)
	if err != nil {
		return time.Time{}, err
	}
	created, ok := parseRFC3339(value)
	if !ok {
		return time.Time{}, rejected("pr view createdAt")
	}
	return created, nil
}

// limitsCallFailure maps a failed gh api call: a status read from the status
// line gh printed with --include, never from the error text.
func limitsCallFailure(err error) *limitsError {
	if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
		return failAt(UnverifiableOutputForm)
	}
	exitErr, ok := errors.AsType[*cmdrunner.ExitError](err)
	if !ok {
		return failAt(UnverifiableGHFailed)
	}
	status, _, _, ok := splitIncluded(exitErr.Stdout)
	// A success status beside a failed exit says nothing about the
	// request, so it is not reported as a refused status.
	if !ok || (status >= 200 && status <= 299) {
		return failAt(UnverifiableGHFailed)
	}
	return &limitsError{reason: UnverifiableHTTPStatus, status: status}
}

// parseLimitsResponse checks the --include output of a successful call: the
// shape, the status, the Date header, then the body.
func parseLimitsResponse(out []byte) (limitsResponse, *limitsError) {
	if len(out) > maxResponseBytes {
		return limitsResponse{}, failAt(UnverifiableOutputForm)
	}
	status, headers, body, ok := splitIncluded(out)
	if !ok {
		return limitsResponse{}, failAt(UnverifiableOutputForm)
	}
	if status != http.StatusOK {
		return limitsResponse{}, &limitsError{reason: UnverifiableHTTPStatus, status: status}
	}
	now, ok := parseDateHeader(headers)
	if !ok {
		return limitsResponse{}, failAt(UnverifiableDate)
	}
	if string(body) == noLimitBody {
		return limitsResponse{now: now, none: true}, nil
	}
	limit, expires, failure := parseLimitsBody(body)
	if failure != nil {
		return limitsResponse{}, failure
	}
	return limitsResponse{now: now, limit: limit, expires: expires}, nil
}

// parseDateHeader returns the time of the one Date header, an IMF-fixdate
// (http.TimeFormat). The obsolete RFC 850 and ANSI C forms are not accepted,
// and gh joins repeated headers into one value, which no longer parses.
// time.Parse also takes a weekday that does not match the date, names in
// another case, and extra spaces, so the value must be what formatting the
// parsed time gives back. Header
// names are matched without regard to case, as HTTP defines them; every other
// header is ignored.
func parseDateHeader(headers []string) (time.Time, bool) {
	var value string
	count := 0
	for _, line := range headers {
		name, v, found := strings.Cut(line, ":")
		if !found || !strings.EqualFold(name, dateHeader) {
			continue
		}
		count++
		value = v
	}
	if count != 1 {
		return time.Time{}, false
	}
	value = strings.Trim(value, " \t")
	now, err := time.Parse(http.TimeFormat, value)
	if err != nil || now.Format(http.TimeFormat) != value {
		return time.Time{}, false
	}
	return now, true
}

// parseLimitsBody checks a body that is not the no-limit one. Members other
// than limit and expires_at, and their duplicates, are ignored because GitHub
// may add members.
func parseLimitsBody(body []byte) (limit string, expires time.Time, failure *limitsError) {
	if len(body) > maxLimitsBodyBytes {
		return "", time.Time{}, failAt(UnverifiableBody)
	}
	obj, err := strictjson.ParseObject(body)
	if err != nil {
		return "", time.Time{}, failAt(UnverifiableBody)
	}
	members, err := obj.Collect("limit", "expires_at")
	if err != nil {
		return "", time.Time{}, failAt(UnverifiableBody)
	}
	limit, err = requiredStringMember(members, "limit")
	if err != nil {
		return "", time.Time{}, failAt(UnverifiableBody)
	}
	expiresRaw, err := requiredStringMember(members, "expires_at")
	if err != nil {
		return "", time.Time{}, failAt(UnverifiableBody)
	}
	switch limit {
	case limitCollaboratorsOnly, limitExistingUsers, limitContributorsOnly:
	default:
		return "", time.Time{}, failAt(UnverifiableLimit)
	}
	expires, ok := parseRFC3339(expiresRaw)
	if !ok {
		return "", time.Time{}, failAt(UnverifiableExpiresAt)
	}
	return limit, expires, nil
}

// parseRFC3339 reads an RFC 3339 date-time with its offset. time.Parse also
// takes an offset hour above 23, an offset minute above 59, and a comma before
// the fraction, which RFC 3339 does not allow; those are rejected here.
func parseRFC3339(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil || strings.Contains(s, ",") {
		return time.Time{}, false
	}
	if strings.HasSuffix(s, "Z") {
		return t, true
	}
	// The offset is the last six bytes, "+hh:mm" or "-hh:mm", whose two-digit
	// fields time.Parse has already checked, so they compare as strings.
	offset := s[len(s)-len("+hh:mm"):]
	return t, offset[1:3] <= "23" && offset[4:6] <= "59"
}

// classifyLimits applies the states in the requirements' order: the first that
// holds wins.
func classifyLimits(resp limitsResponse, created time.Time) InteractionLimitsInfo {
	if resp.none || !resp.expires.After(resp.now) {
		return InteractionLimitsInfo{State: InteractionNone}
	}
	expiresOn := resp.expires.UTC().Format(expiresOnLayout)
	switch {
	case resp.limit != limitCollaboratorsOnly:
		return InteractionLimitsInfo{State: InteractionWeak, ExpiresOn: expiresOn}
	case created.Before(limitSetAt(resp.expires)):
		return InteractionLimitsInfo{State: InteractionPredates, ExpiresOn: expiresOn}
	case resp.expires.Sub(resp.now) <= expiringWithin:
		return InteractionLimitsInfo{State: InteractionExpiring, ExpiresOn: expiresOn}
	default:
		return InteractionLimitsInfo{State: InteractionActive, ExpiresOn: expiresOn}
	}
}

// limitSetAt estimates when the limit was set: expires_at in UTC moved back
// limitPeriodMonths calendar months, keeping the day and the time of day. When
// that month has no such day, or expires_at falls on the last day of its month,
// the estimate is the last day of the earlier month. However GitHub rolls a
// month end, the estimate is then no earlier than the real time, so a PR that
// predates the limit is never judged to follow it.
func limitSetAt(expires time.Time) time.Time {
	e := expires.UTC()
	year, month, day := e.Date()
	setYear, setMonth := year, month-limitPeriodMonths
	if setMonth < time.January {
		setYear--
		setMonth += 12
	}
	setLast := daysIn(setYear, setMonth)
	if day > setLast || day == daysIn(year, month) {
		day = setLast
	}
	hour, minute, sec := e.Clock()
	return time.Date(setYear, setMonth, day, hour, minute, sec, e.Nanosecond(), time.UTC)
}

// daysIn returns the number of days in month of year.
func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
