package fixprguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

// Review state: /runpr asks a reviewer for a manual PR review by posting a
// request comment, then reads here whether the review finished. The answer
// comes from the reviewer's check runs, which only that GitHub App can create,
// never from a comment body. A comment body is compared with the request text
// and then dropped; a check run's title is compared with the fixed titles of
// the classification table and never printed; the check run's details_url is
// never read.

const (
	// Names of JSON members and gh arguments used in several places.
	keyDatabaseID = "databaseId"
	keyURL        = "url"
	keyBody       = "body"
	keyCommit     = "commit"
	keyUser       = "user"
	keyData       = "data"
	ghAPI         = "api"
	// ghUserEndpoint is the gh api endpoint of the authenticated user.
	ghUserEndpoint = "user"
	keyPageInfo    = "pageInfo"
	keyLogin       = "login"
	keyType        = "type"

	// keyHasNextPage is the page-info member that says a list continues.
	keyHasNextPage = "hasNextPage"

	// Titles and conclusions of the classification table.
	keyName               = "name"
	checkNameCodeRabbit   = "CodeRabbit"
	checkNameSourcery     = "Sourcery review"
	statusCompleted       = "COMPLETED"
	conclusionSuccess     = "SUCCESS"
	conclusionNeutral     = "NEUTRAL"
	conclusionSkipped     = "SKIPPED"
	titleFinished         = "Finished"
	titleRateLimited      = "Review rate limited"
	titleAutoReviewOff    = "Review skipped: automatic reviews are disabled"
	titleSourceryFinished = "✅ Completed successfully"
	titleSourcerySkipped  = "⏭️ Skipped"
	botLoginSuffix        = "[bot]"
	restPerPage           = 100
	commitsPerPage        = 20
	sha1HexLength         = 40
	maxRequestTextLength  = 1024

	// reviewWaitLimit is how long a request is waited for: past it the state is
	// timed-out, even when a check run of the request is still running.
	reviewWaitLimit = 15 * time.Minute

	// Fixed limits; going over one is the unreadable state with reason limit.
	maxRESTItems      = 3000
	maxReviewComments = maxRESTItems
	maxReviewReviews  = maxRESTItems
	maxPRCommits      = 1000
	maxCheckSuites    = 50
	maxCheckRuns      = 100
	maxPRCommitPages  = maxPRCommits/commitsPerPage + 1
)

// ReviewProfile declares which reviewer's CheckRun naming, classification,
// automatic-review, observed status-comment, and competing-run rules apply.
// Contract: the zero value ProfileUnknown fails closed (MarshalText returns
// errUnknownEnum, usable returns false, and callers reject it with
// ErrInvalidInput).
type ReviewProfile int

// Declared reviewer profiles.
const (
	ProfileUnknown    ReviewProfile = iota // invalid zero value
	ProfileCodeRabbit                      // "coderabbit": CheckRun "CodeRabbit", manual only
	ProfileSourcery                        // "sourcery": CheckRun "Sourcery review", auto + manual
)

// ParseReviewProfile parses s into a declared ReviewProfile without repairing
// it; any unknown value returns ErrInvalidInput without echoing s.
func ParseReviewProfile(s string) (ReviewProfile, error) {
	switch s {
	case "coderabbit":
		return ProfileCodeRabbit, nil
	case "sourcery":
		return ProfileSourcery, nil
	default:
		return ProfileUnknown, fmt.Errorf("%w: review profile", ErrInvalidInput)
	}
}

func (p ReviewProfile) usable() bool {
	return p == ProfileCodeRabbit || p == ProfileSourcery
}

func (p ReviewProfile) checkName() string {
	switch p {
	case ProfileCodeRabbit:
		return checkNameCodeRabbit
	case ProfileSourcery:
		return checkNameSourcery
	default:
		return ""
	}
}

// MarshalText names the profile. ProfileUnknown and any value outside the
// declared set return errUnknownEnum.
func (p ReviewProfile) MarshalText() ([]byte, error) {
	switch p {
	case ProfileCodeRabbit:
		return []byte("coderabbit"), nil
	case ProfileSourcery:
		return []byte("sourcery"), nil
	default:
		return nil, fmt.Errorf("%w: review profile", errUnknownEnum)
	}
}

// ReviewProfiles lists every valid ReviewProfile, for the guide's document
// guard.
func ReviewProfiles() []ReviewProfile {
	return []ReviewProfile{ProfileCodeRabbit, ProfileSourcery}
}

// RunKind classifies one reviewer check run. The zero value is RunUnknown, the
// interpretation that assumes least: a run left unclassified is never read as
// a finished review.
type RunKind int

// The kinds of check run.
const (
	RunUnknown     RunKind = iota // "unknown"
	RunRunning                    // "running"
	RunFinished                   // "finished"
	RunRateLimited                // "rate-limited"
	RunSkipNotice                 // "skip-notice"; never a response
	RunSkipped                    // "skipped"; Sourcery terminal SKIPPED
)

// MarshalText names the kind. A value outside the list is an error.
func (k RunKind) MarshalText() ([]byte, error) {
	switch k {
	case RunUnknown:
		return []byte("unknown"), nil
	case RunRunning:
		return []byte("running"), nil
	case RunFinished:
		return []byte("finished"), nil
	case RunRateLimited:
		return []byte("rate-limited"), nil
	case RunSkipNotice:
		return []byte("skip-notice"), nil
	case RunSkipped:
		return []byte("skipped"), nil
	default:
		return nil, fmt.Errorf("%w: run kind", errUnknownEnum)
	}
}

// RequestState is the state of the latest request comment. The zero value is
// RequestIndeterminate, the state that assumes least.
type RequestState int

// The states, in the order the rules of the state machine decide them.
const (
	RequestIndeterminate RequestState = iota // "indeterminate"
	RequestUnreadable                        // "unreadable"
	RequestNone                              // "none"
	RequestWaiting                           // "waiting"
	RequestCompleted                         // "completed"
	RequestRateLimited                       // "rate-limited"
	RequestSkipped                           // "skipped"
	RequestUnrecognized                      // "unrecognized"
	RequestTimedOut                          // "timed-out"
)

// MarshalText names the state. A value outside the list is an error.
func (s RequestState) MarshalText() ([]byte, error) {
	switch s {
	case RequestIndeterminate:
		return []byte("indeterminate"), nil
	case RequestUnreadable:
		return []byte("unreadable"), nil
	case RequestNone:
		return []byte("none"), nil
	case RequestWaiting:
		return []byte("waiting"), nil
	case RequestCompleted:
		return []byte("completed"), nil
	case RequestRateLimited:
		return []byte("rate-limited"), nil
	case RequestSkipped:
		return []byte("skipped"), nil
	case RequestUnrecognized:
		return []byte("unrecognized"), nil
	case RequestTimedOut:
		return []byte("timed-out"), nil
	default:
		return nil, fmt.Errorf("%w: request state", errUnknownEnum)
	}
}

// RequestStates lists every state; the guide's document guard names each
// through MarshalText.
func RequestStates() []RequestState {
	return []RequestState{
		RequestIndeterminate, RequestUnreadable, RequestNone, RequestWaiting,
		RequestCompleted, RequestRateLimited, RequestSkipped, RequestUnrecognized, RequestTimedOut,
	}
}

// AutoReviewState is the state of the non-manual CheckRun on head chosen by
// the four-step priority of AutoReview (ProfileSourcery only; AutoNone when
// absent or ProfileCodeRabbit).
type AutoReviewState int

// The states of an automatic PR review on head.
const (
	AutoNone         AutoReviewState = iota // "none"
	AutoRunning                             // "running"
	AutoCompleted                           // "completed"
	AutoSkipped                             // "skipped"
	AutoUnrecognized                        // "unrecognized"
	AutoTimedOut                            // "timed-out"
)

// MarshalText names the automatic review state. A value outside the list is an
// error.
func (s AutoReviewState) MarshalText() ([]byte, error) {
	switch s {
	case AutoNone:
		return []byte("none"), nil
	case AutoRunning:
		return []byte("running"), nil
	case AutoCompleted:
		return []byte("completed"), nil
	case AutoSkipped:
		return []byte("skipped"), nil
	case AutoUnrecognized:
		return []byte("unrecognized"), nil
	case AutoTimedOut:
		return []byte("timed-out"), nil
	default:
		return nil, fmt.Errorf("%w: auto review state", errUnknownEnum)
	}
}

// AutoReviewStates lists every automatic review state, for the guide's
// document guard.
func AutoReviewStates() []AutoReviewState {
	return []AutoReviewState{
		AutoNone, AutoRunning, AutoCompleted, AutoSkipped, AutoUnrecognized, AutoTimedOut,
	}
}

// UnreadableReason says why the state could not be read. The zero value is the
// empty name, used for every state but unreadable.
type UnreadableReason int

// The reasons.
const (
	UnreadableNone     UnreadableReason = iota // "" (the state is not unreadable)
	UnreadableCall                             // "call-failed": a gh call failed
	UnreadableRejected                         // "rejected": a response failed the strict checks
	UnreadableLimit                            // "limit": a fixed limit was exceeded
)

// MarshalText names the reason. A value outside the list is an error.
func (r UnreadableReason) MarshalText() ([]byte, error) {
	switch r {
	case UnreadableNone:
		return []byte(""), nil
	case UnreadableCall:
		return []byte("call-failed"), nil
	case UnreadableRejected:
		return []byte("rejected"), nil
	case UnreadableLimit:
		return []byte("limit"), nil
	default:
		return nil, fmt.Errorf("%w: unreadable reason", errUnknownEnum)
	}
}

// UnreadableReasons lists every non-empty reason, for the guide's document
// guard.
func UnreadableReasons() []UnreadableReason {
	return []UnreadableReason{UnreadableCall, UnreadableRejected, UnreadableLimit}
}

// IndeterminateReason says why the latest request's response could not be
// determined. The zero value is the empty name, used for every state but
// indeterminate.
type IndeterminateReason int

// The reasons.
const (
	IndeterminateNone          IndeterminateReason = iota // "" (the state is not indeterminate)
	IndeterminateCompetingRuns                            // "competing-runs": two or more check runs compete as the response
	IndeterminateEditedRequest                            // "edited-request": the request comment was edited
)

// MarshalText names the reason. A value outside the list is an error.
func (r IndeterminateReason) MarshalText() ([]byte, error) {
	switch r {
	case IndeterminateNone:
		return []byte(""), nil
	case IndeterminateCompetingRuns:
		return []byte("competing-runs"), nil
	case IndeterminateEditedRequest:
		return []byte("edited-request"), nil
	default:
		return nil, fmt.Errorf("%w: indeterminate reason", errUnknownEnum)
	}
}

// IndeterminateReasons lists every non-empty reason, for the guide's document
// guard.
func IndeterminateReasons() []IndeterminateReason {
	return []IndeterminateReason{IndeterminateCompetingRuns, IndeterminateEditedRequest}
}

// RunKinds lists every run kind, for the guide's document guard.
func RunKinds() []RunKind {
	return []RunKind{RunUnknown, RunRunning, RunFinished, RunRateLimited, RunSkipNotice, RunSkipped}
}

// ReviewRequest is a request comment.
type ReviewRequest struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"` // RFC 3339 UTC
}

// ReviewResponse is a request comment's response.
type ReviewResponse struct {
	Kind       RunKind `json:"kind"`
	Status     string  `json:"status"`     // the check run's status enum, as GitHub writes it
	Conclusion string  `json:"conclusion"` // its conclusion enum, or ""
	Commit     string  `json:"commit"`     // the commit the run belongs to, 40 lowercase hex
	URL        string  `json:"url"`        // the check run's url, never details_url
	StartedAt  string  `json:"startedAt"`
}

// AutoReview is the non-manual check run on head chosen by the four-step
// priority (ProfileSourcery only; null when absent or unreadable).
type AutoReview struct {
	State      AutoReviewState `json:"state"`
	Kind       RunKind         `json:"kind"`
	Status     string          `json:"status"`
	Conclusion string          `json:"conclusion"`
	Commit     string          `json:"commit"`
	URL        string          `json:"url"`
	StartedAt  string          `json:"startedAt"` // "" when status != COMPLETED and startedAt is null
}

// ReviewMarker is a completion marker: a finished review of Commit from an
// unedited manual request.
type ReviewMarker struct {
	Commit      string `json:"commit"`
	RequestID   int64  `json:"requestId"`
	RequestURL  string `json:"requestUrl"`
	CheckRunURL string `json:"checkRunUrl"`
	ReviewURL   string `json:"reviewUrl"` // the reviewer's PR review of Commit submitted after the request, or ""
	CompletedAt string `json:"completedAt"`
}

// PRReviewState is the output of review-state. No member holds a comment body, a
// check run's title, or its details_url. Profile is always populated from the
// request's Profile on every return path. Request, Response, and AutoReview
// are null when they do not exist; Markers and IndeterminateURLs are empty
// lists, never null.
type PRReviewState struct {
	Protocol            int                 `json:"protocol"`
	Profile             ReviewProfile       `json:"profile"`             // always populated from req.Profile (including unreadable/none)
	State               RequestState        `json:"state"`               // the state of the latest request comment
	Reason              UnreadableReason    `json:"reason"`              // set for state unreadable only
	IndeterminateReason IndeterminateReason `json:"indeterminateReason"` // set for state indeterminate only
	Head                string              `json:"head"`                // the PR's head, 40 lowercase hex; "" only when it could not be read
	Now                 string              `json:"now"`                 // GitHub's clock, RFC 3339 UTC; "" for unreadable
	Request             *ReviewRequest      `json:"request"`             // the latest request comment; null for none and unreadable
	Response            *ReviewResponse     `json:"response"`            // its response; null when it has none
	AutoReview          *AutoReview         `json:"autoReview"`          // non-manual check run on head (ProfileSourcery only); null when absent or unreadable
	IndeterminateURLs   []string            `json:"indeterminateUrls"`   // competing check run URLs, for competing-runs only
	Markers             []ReviewMarker      `json:"markers"`             // every completion marker, sorted by CompletedAt then CheckRunURL ascending
}

// newReviewState returns a state with Profile set and the lists empty rather
// than null.
func newReviewState(profile ReviewProfile, state RequestState) PRReviewState {
	return PRReviewState{Profile: profile, State: state, IndeterminateURLs: []string{}, Markers: []ReviewMarker{}}
}

// ReviewStateRequest names what PRReviewState reads. Profile chooses the
// check run naming, classification, automatic-review, and competing-run rules;
// Reviewer is the login of the reviewer App whose check runs and PR reviews
// count; Requests is the non-empty list of unique exact texts of manual
// request comments. Only ParsePRNumber, ParseReviewProfile, and ParseLogin
// build the usable values.
type ReviewStateRequest struct {
	PR       PRNumber
	Profile  ReviewProfile
	Reviewer Login
	Requests []string
}

// reviewError is a read failure with the reason it reports. It is a plain
// value, not an error: it never leaves the package, because every failure
// becomes the unreadable state, and it carries no text of a response.
type reviewError struct {
	reason UnreadableReason
}

func failReview(reason UnreadableReason) *reviewError { return &reviewError{reason: reason} }

// errReviewLimit marks a parse that went over a fixed limit; errReviewHead
// marks a page whose headRefOid differs from the head read first.
var (
	errReviewLimit = errors.New("review state limit")
	errReviewHead  = errors.New("review state head changed")
)

// parseFailure maps a parse error to the reason it reports.
func parseFailure(err error) *reviewError {
	switch {
	case errors.Is(err, errReviewLimit):
		return failReview(UnreadableLimit)
	case errors.Is(err, errReviewHead):
		return failReview(UnreadableCall)
	default:
		return failReview(UnreadableRejected)
	}
}

// requestComment is a comment that is a request: the user's own comment with
// exactly the request text. The body is not kept.
type requestComment struct {
	id      int64
	url     string
	created time.Time
	updated time.Time
}

func (c requestComment) edited() bool { return !c.updated.Equal(c.created) }

// checkRun is one reviewer check run of a commit of the PR. started and
// completed are nil for a run that has not started or finished.
type checkRun struct {
	commit     string
	status     string
	conclusion string
	title      string
	started    *time.Time
	completed  *time.Time
	url        string
}

// kind classifies the run by the profile's table of status, conclusion and
// title. A COMPLETED run outside the table is RunUnknown and never a finished
// review.
func (r checkRun) kind(profile ReviewProfile) RunKind {
	if r.status != statusCompleted {
		return RunRunning
	}
	switch profile {
	case ProfileCodeRabbit:
		switch {
		case r.conclusion == conclusionSuccess && r.title == titleFinished:
			return RunFinished
		case r.conclusion == conclusionNeutral && r.title == titleRateLimited:
			return RunRateLimited
		case r.conclusion == conclusionSkipped && r.title == titleAutoReviewOff:
			return RunSkipNotice
		default:
			return RunUnknown
		}
	case ProfileSourcery:
		switch {
		case r.conclusion == conclusionSuccess && r.title == titleSourceryFinished:
			return RunFinished
		case r.conclusion == conclusionSkipped && r.title == titleSourcerySkipped:
			return RunSkipped
		default:
			return RunUnknown
		}
	default:
		return RunUnknown
	}
}

// prReview is a PR review of the reviewer App.
type prReview struct {
	commit    string
	submitted *time.Time
	url       string
}

// reviewReader holds what one PRReviewState call has read. head is set once the
// PR's head has been read and checked; now is the Date header of the latest
// REST response.
type reviewReader struct {
	g    *GitHub
	repo repository
	pr   PRNumber
	head string
	now  time.Time
}

// PRReviewState reads the state of the latest request comment of the PR (and
// for ProfileSourcery, any non-manual automatic check run on head). It returns
// an error only for an unusable argument; every read failure is the unreadable
// state with its reason and Profile set, and no partial result is kept.
// Protocol is left for the caller to set. It judges no thread.
func (g *GitHub) PRReviewState(ctx context.Context, req ReviewStateRequest) (PRReviewState, error) {
	if !req.PR.usable() || !req.Profile.usable() || !req.Reviewer.usable() {
		return PRReviewState{}, fmt.Errorf("%w: review state arguments", ErrInvalidInput)
	}
	if err := validateRequestTexts(req.Requests); err != nil {
		return PRReviewState{}, err
	}
	// One deadline covers every call; runGH's own bound is no later.
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()
	r := &reviewReader{g: g, pr: req.PR}
	state, failure := r.read(ctx, req)
	if failure != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			failure = failReview(UnreadableCall)
		}
		out := newReviewState(req.Profile, RequestUnreadable)
		out.Reason = failure.reason
		out.Head = r.head
		return out, nil
	}
	return state, nil
}

// read runs the reads in order and stops at the first failure: the
// repository, the PR's head, the user, the conversation comments (for
// ProfileCodeRabbit with no request comment, nothing after them is read; for
// ProfileSourcery, check runs are still read to build AutoReview), the PR's
// commits with their check runs, then the reviewer's PR reviews (when at least
// one request comment exists). The head is read before the user so that a
// failed gh api user still reports the head it managed to read.
func (r *reviewReader) read(ctx context.Context, req ReviewStateRequest) (PRReviewState, *reviewError) {
	repo, err := r.g.resolveRepository(ctx)
	if err != nil {
		return PRReviewState{}, failReview(UnreadableCall)
	}
	r.repo = repo
	head, failure := r.readHead(ctx)
	if failure != nil {
		return PRReviewState{}, failure
	}
	r.head = head
	user, failure := r.readUser(ctx)
	if failure != nil {
		return PRReviewState{}, failure
	}
	requests, failure := r.readRequests(ctx, user, req.Requests)
	if failure != nil {
		return PRReviewState{}, failure
	}
	if len(requests) == 0 && req.Profile == ProfileCodeRabbit {
		out := newReviewState(req.Profile, RequestNone)
		out.Head, out.Now = r.head, formatReviewTime(r.now)
		return out, nil
	}
	runs, commitIndex, failure := r.readRuns(ctx, req.Profile, req.Reviewer)
	if failure != nil {
		return PRReviewState{}, failure
	}
	if len(requests) == 0 {
		out := newReviewState(req.Profile, RequestNone)
		out.Head, out.Now = r.head, formatReviewTime(r.now)
		out.AutoReview = r.selectAutoReview(runs, nil, false)
		return out, nil
	}
	reviews, failure := r.readReviews(ctx, req.Reviewer)
	if failure != nil {
		return PRReviewState{}, failure
	}
	return r.decide(req.Profile, requests, runs, commitIndex, reviews), nil
}

func formatReviewTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// readUser reads the login of the user gh acts as; only that user's comments
// can be request comments.
func (r *reviewReader) readUser(ctx context.Context) (string, *reviewError) {
	out, err := r.g.runGH(ctx, nil, ghAPI, ghUserEndpoint)
	if err != nil {
		return "", callFailure(err)
	}
	if len(out) > maxResponseBytes {
		return "", failReview(UnreadableLimit)
	}
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return "", failReview(UnreadableRejected)
	}
	members, err := obj.Collect(keyLogin)
	if err != nil {
		return "", failReview(UnreadableRejected)
	}
	login, err := strictjson.RequiredString(members, keyLogin)
	if err != nil || !validLogin(login) {
		return "", failReview(UnreadableRejected)
	}
	return login, nil
}

// callFailure maps a failed gh call: an output over the cap is a limit, any
// other failure is a failed call.
func callFailure(err error) *reviewError {
	if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
		return failReview(UnreadableLimit)
	}
	return failReview(UnreadableCall)
}

// restGet reads a REST path of the repository with gh api --include and
// returns the body of a 200 response. The Date header of every response must
// be one IMF-fixdate; the latest one becomes now.
func (r *reviewReader) restGet(ctx context.Context, path string) ([]byte, *reviewError) {
	out, err := r.g.runGH(ctx, nil, ghAPI, "--include", "repos/"+r.repo.owner+"/"+r.repo.name+"/"+path)
	if err != nil {
		return nil, callFailure(err)
	}
	if len(out) > maxResponseBytes {
		return nil, failReview(UnreadableLimit)
	}
	status, headers, body, ok := splitIncluded(out)
	if !ok {
		return nil, failReview(UnreadableRejected)
	}
	if status != http.StatusOK {
		return nil, failReview(UnreadableCall)
	}
	now, ok := parseDateHeader(headers)
	if !ok {
		return nil, failReview(UnreadableRejected)
	}
	r.now = now
	return body, nil
}

// readHead reads the PR's head commit, which every later page must agree
// with.
func (r *reviewReader) readHead(ctx context.Context) (string, *reviewError) {
	body, failure := r.restGet(ctx, "pulls/"+r.pr.String())
	if failure != nil {
		return "", failure
	}
	obj, err := strictjson.ParseObject(body)
	if err != nil {
		return "", failReview(UnreadableRejected)
	}
	head, err := memberObject(obj, "head")
	if err != nil {
		return "", failReview(UnreadableRejected)
	}
	members, err := head.Collect("sha")
	if err != nil {
		return "", failReview(UnreadableRejected)
	}
	sha, err := strictjson.RequiredString(members, "sha")
	if err != nil || !validHex(sha, sha1HexLength) {
		return "", failReview(UnreadableRejected)
	}
	return sha, nil
}

// restPages reads every page of a REST list, at most maxRESTItems items in
// all, and hands each item to each. A page with fewer than restPerPage items
// is the last.
func (r *reviewReader) restPages(ctx context.Context, path string, each func(strictjson.Value) error) *reviewError {
	count := 0
	for page := 1; ; page++ {
		body, failure := r.restGet(ctx, fmt.Sprintf("%s?per_page=%d&page=%d", path, restPerPage, page))
		if failure != nil {
			return failure
		}
		items, err := strictjson.ParseArray(body)
		if err != nil || len(items) > restPerPage {
			return failReview(UnreadableRejected)
		}
		count += len(items)
		if count > maxRESTItems {
			return failReview(UnreadableLimit)
		}
		for _, item := range items {
			if err := each(item); err != nil {
				return parseFailure(err)
			}
		}
		if len(items) < restPerPage {
			return nil
		}
	}
}

// readRequests reads the PR's conversation comments and returns the request
// comments. A comment is a request only when its author is the user and of
// kind User, and its body matches one of requestTexts; the same name as a
// Bot, or another user with the same body, is not.
func (r *reviewReader) readRequests(ctx context.Context, user string, requestTexts []string) ([]requestComment, *reviewError) {
	var requests []requestComment
	failure := r.restPages(ctx, "issues/"+r.pr.String()+"/comments", func(item strictjson.Value) error {
		obj, err := item.AsObject()
		if err != nil {
			return rejected("comment")
		}
		members, err := obj.Collect("id", keyUser, keyBody, "created_at", "updated_at", "html_url")
		if err != nil {
			return rejected("comment")
		}
		id, err := positiveInt64Member(members, "id")
		if err != nil {
			return err
		}
		login, kind, err := userMember(members)
		if err != nil {
			return err
		}
		body, err := strictjson.RequiredStringAllowEmpty(members, keyBody)
		if err != nil {
			return rejected("comment body")
		}
		created, err := timeMember(members, "created_at")
		if err != nil {
			return err
		}
		updated, err := timeMember(members, "updated_at")
		if err != nil {
			return err
		}
		url, err := strictjson.RequiredString(members, "html_url")
		if err != nil || !validCommentURL(url) {
			return rejected("comment html_url")
		}
		if isRequestComment(kind, login, user, body, requestTexts) {
			requests = append(requests, requestComment{id: id, url: url, created: created, updated: updated})
		}
		return nil
	})
	return requests, failure
}

// isRequestComment reports whether a PR conversation comment is a manual
// review request from the authenticated user. Contract: only a User whose
// login matches user byte for byte and whose body after strings.TrimSpace
// equals one of requests is a request comment; a Bot or another user never is.
func isRequestComment(kind, login, user, body string, requests []string) bool {
	if kind != typenameUser || login != user {
		return false
	}
	return slices.Contains(requests, strings.TrimSpace(body))
}

// positiveInt64Member reads a required member that is a positive integer.
func positiveInt64Member(members map[string]strictjson.Value, key string) (int64, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return 0, rejected(key)
	}
	n, err := value.AsNonNegativeInt64()
	if err != nil || n == 0 {
		return 0, rejected(key)
	}
	return n, nil
}

// userMember reads a required member holding null or an object with login and
// type strings: null (a deleted account) is no user, and its login and type
// are empty.
func userMember(members map[string]strictjson.Value) (login, kind string, err error) {
	value, err := strictjson.Required(members, keyUser)
	if err != nil {
		return "", "", rejected("user")
	}
	if value.IsNull() {
		return "", "", nil
	}
	obj, err := value.AsObject()
	if err != nil {
		return "", "", rejected("user")
	}
	fields, err := obj.Collect(keyLogin, keyType)
	if err != nil {
		return "", "", rejected("user")
	}
	if login, err = strictjson.RequiredStringAllowEmpty(fields, keyLogin); err != nil {
		return "", "", rejected("user login")
	}
	if kind, err = strictjson.RequiredStringAllowEmpty(fields, keyType); err != nil {
		return "", "", rejected("user type")
	}
	return login, kind, nil
}

// timeMember reads a required member that is an RFC 3339 date-time.
func timeMember(members map[string]strictjson.Value, key string) (time.Time, error) {
	text, err := strictjson.RequiredString(members, key)
	if err != nil {
		return time.Time{}, rejected(key)
	}
	t, ok := parseRFC3339(text)
	if !ok {
		return time.Time{}, rejected(key)
	}
	return t, nil
}

// reviewCommitsQuery reads one page of the PR's commits with the reviewer's
// check runs of each: every check run, not only the latest of its name
// (checkType: ALL). The page holds few commits so the node count of the query
// stays under GitHub's limit.
const reviewCommitsQuery = `query($owner: String!, $name: String!, $number: Int!, $checkName: String!, $after: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      headRefOid
      commits(first: 20, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          commit {
            oid
            checkSuites(first: 50) {
              pageInfo { hasNextPage }
              nodes {
                app { slug }
                checkRuns(first: 100, filterBy: {checkName: $checkName, checkType: ALL}) {
                  pageInfo { hasNextPage }
                  nodes { name status conclusion title startedAt completedAt url }
                }
              }
            }
          }
        }
      }
    }
  }
}`

// commitsPage is one page of reviewCommitsQuery: its commit OIDs in PR order,
// the reviewer's check runs of its commits, and how to read the next page.
type commitsPage struct {
	commitOIDs  []string
	runs        []checkRun
	hasNextPage bool
	endCursor   string
}

// readRuns reads every page of the PR's commits and returns the reviewer's
// check runs together with each commit's ordinal position in PR commit order.
// Each page must carry the head read first: a push or a force push in between
// discards the whole read.
func (r *reviewReader) readRuns(ctx context.Context, profile ReviewProfile, reviewer Login) ([]checkRun, map[string]int, *reviewError) {
	var runs []checkRun
	commitIndex := make(map[string]int)
	commits := 0
	after := ""
	wantCheckName := profile.checkName()
	for pages := 1; ; pages++ {
		variables := map[string]any{
			"owner":     r.repo.owner,
			"name":      r.repo.name,
			"number":    r.pr.n,
			"checkName": wantCheckName,
		}
		if after != "" {
			variables["after"] = after
		}
		request, err := json.Marshal(map[string]any{"query": reviewCommitsQuery, "variables": variables})
		if err != nil {
			return nil, nil, failReview(UnreadableCall)
		}
		out, err := r.g.runGH(ctx, request, ghAPI, "graphql", "--input", "-")
		if err != nil {
			return nil, nil, callFailure(err)
		}
		if len(out) > maxResponseBytes {
			return nil, nil, failReview(UnreadableLimit)
		}
		page, err := parseCommitsPage(out, r.head, reviewer.String(), wantCheckName)
		if err != nil {
			return nil, nil, parseFailure(err)
		}
		for _, oid := range page.commitOIDs {
			commitIndex[oid] = commits
			commits++
		}
		if commits > maxPRCommits {
			return nil, nil, failReview(UnreadableLimit)
		}
		runs = append(runs, page.runs...)
		if !page.hasNextPage {
			return runs, commitIndex, nil
		}
		if pages >= maxPRCommitPages {
			return nil, nil, failReview(UnreadableLimit)
		}
		after = page.endCursor
	}
}

// parseCommitsPage checks one page of reviewCommitsQuery. A check suite of
// another App is skipped without reading its runs. A page that holds more
// suites or runs than the query's page size (hasNextPage) is over a limit.
func parseCommitsPage(out []byte, wantHead, reviewer, wantCheckName string) (commitsPage, error) {
	data, err := graphqlData(out)
	if err != nil {
		return commitsPage{}, err
	}
	repo, err := memberObject(data, "repository")
	if err != nil {
		return commitsPage{}, err
	}
	pull, err := memberObject(repo, "pullRequest")
	if err != nil {
		return commitsPage{}, err
	}
	members, err := pull.Collect("headRefOid")
	if err != nil {
		return commitsPage{}, rejected("headRefOid")
	}
	gotHead, err := strictjson.RequiredString(members, "headRefOid")
	if err != nil || !validHex(gotHead, sha1HexLength) {
		return commitsPage{}, rejected("headRefOid")
	}
	if gotHead != wantHead {
		return commitsPage{}, errReviewHead
	}
	commits, err := memberObject(pull, "commits")
	if err != nil {
		return commitsPage{}, err
	}
	page, err := parseCommitsPageInfo(commits)
	if err != nil {
		return commitsPage{}, err
	}
	nodes, err := memberNodes(commits)
	if err != nil {
		return commitsPage{}, err
	}
	if len(nodes) > commitsPerPage {
		return commitsPage{}, rejected("commit page size")
	}
	page.commitOIDs = make([]string, 0, len(nodes))
	for _, node := range nodes {
		oid, runs, err := parseCommitNode(node, reviewer, wantCheckName)
		if err != nil {
			return commitsPage{}, err
		}
		page.commitOIDs = append(page.commitOIDs, oid)
		page.runs = append(page.runs, runs...)
	}
	return page, nil
}

// parseCommitsPageInfo reads hasNextPage, and endCursor when there is a next
// page.
func parseCommitsPageInfo(commits strictjson.Object) (commitsPage, error) {
	pageInfo, err := memberObject(commits, keyPageInfo)
	if err != nil {
		return commitsPage{}, err
	}
	hasNext, err := memberBool(pageInfo, keyHasNextPage)
	if err != nil {
		return commitsPage{}, err
	}
	page := commitsPage{hasNextPage: hasNext}
	if hasNext {
		members, err := pageInfo.Collect("endCursor")
		if err != nil {
			return commitsPage{}, rejected("endCursor")
		}
		if page.endCursor, err = strictjson.RequiredString(members, "endCursor"); err != nil {
			return commitsPage{}, rejected("endCursor")
		}
	}
	return page, nil
}

// parseCommitNode reads the commit OID and the reviewer's check runs of one
// commit node.
func parseCommitNode(node strictjson.Value, reviewer, wantCheckName string) (string, []checkRun, error) {
	obj, err := node.AsObject()
	if err != nil {
		return "", nil, rejected("commit node")
	}
	commit, err := memberObject(obj, keyCommit)
	if err != nil {
		return "", nil, err
	}
	members, err := commit.Collect("oid")
	if err != nil {
		return "", nil, rejected("commit oid")
	}
	oid, err := strictjson.RequiredString(members, "oid")
	if err != nil || !validHex(oid, sha1HexLength) {
		return "", nil, rejected("commit oid")
	}
	suites, err := memberObject(commit, "checkSuites")
	if err != nil {
		return "", nil, err
	}
	if err := requireNoNextPage(suites); err != nil {
		return "", nil, err
	}
	nodes, err := memberNodes(suites)
	if err != nil {
		return "", nil, err
	}
	if len(nodes) > maxCheckSuites {
		return "", nil, errReviewLimit
	}
	var runs []checkRun
	for _, suiteNode := range nodes {
		suiteRuns, err := parseSuite(suiteNode, reviewer, oid, wantCheckName)
		if err != nil {
			return "", nil, err
		}
		runs = append(runs, suiteRuns...)
	}
	return oid, runs, nil
}

// requireNoNextPage reports a list that GitHub says continues past the page
// the query asked for: the rest would be unread, so the read is over a limit.
func requireNoNextPage(list strictjson.Object) error {
	pageInfo, err := memberObject(list, keyPageInfo)
	if err != nil {
		return err
	}
	more, err := memberBool(pageInfo, keyHasNextPage)
	if err != nil {
		return err
	}
	if more {
		return errReviewLimit
	}
	return nil
}

// parseSuite reads the check runs of one check suite, or none when the suite
// belongs to another App (a null app included).
func parseSuite(node strictjson.Value, reviewer, commit, wantCheckName string) ([]checkRun, error) {
	obj, err := node.AsObject()
	if err != nil {
		return nil, rejected("check suite")
	}
	members, err := obj.Collect("app")
	if err != nil {
		return nil, rejected("check suite")
	}
	app, err := strictjson.Required(members, "app")
	if err != nil {
		return nil, rejected("check suite app")
	}
	if app.IsNull() {
		return nil, nil
	}
	appObj, err := app.AsObject()
	if err != nil {
		return nil, rejected("check suite app")
	}
	appMembers, err := appObj.Collect("slug")
	if err != nil {
		return nil, rejected("check suite app")
	}
	slug, err := strictjson.RequiredStringAllowEmpty(appMembers, "slug")
	if err != nil {
		return nil, rejected("check suite app slug")
	}
	if slug != reviewer {
		return nil, nil
	}
	checkRuns, err := memberObject(obj, "checkRuns")
	if err != nil {
		return nil, err
	}
	if err := requireNoNextPage(checkRuns); err != nil {
		return nil, err
	}
	nodes, err := memberNodes(checkRuns)
	if err != nil {
		return nil, err
	}
	if len(nodes) > maxCheckRuns {
		return nil, errReviewLimit
	}
	runs := make([]checkRun, 0, len(nodes))
	for _, runNode := range nodes {
		run, err := parseCheckRun(runNode, commit, wantCheckName)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// parseCheckRun reads one check run. name must equal wantCheckName exactly;
// status is required; conclusion, title, startedAt and completedAt are required
// members that may be null unless the status is COMPLETED, when all four must
// be strings.
func parseCheckRun(node strictjson.Value, commit, wantCheckName string) (checkRun, error) {
	obj, err := node.AsObject()
	if err != nil {
		return checkRun{}, rejected("check run")
	}
	members, err := obj.Collect(keyName, "status", "conclusion", "title", "startedAt", "completedAt", keyURL)
	if err != nil {
		return checkRun{}, rejected("check run")
	}
	name, err := strictjson.RequiredString(members, keyName)
	if err != nil || name != wantCheckName {
		return checkRun{}, rejected("check run name")
	}
	run := checkRun{commit: commit}
	if run.status, err = strictjson.RequiredString(members, "status"); err != nil {
		return checkRun{}, rejected("check run status")
	}
	var present [4]bool
	if run.conclusion, present[0], err = nullableStringMember(members, "conclusion"); err != nil {
		return checkRun{}, err
	}
	if run.title, present[1], err = nullableStringMember(members, "title"); err != nil {
		return checkRun{}, err
	}
	if run.started, present[2], err = nullableTimeMember(members, "startedAt"); err != nil {
		return checkRun{}, err
	}
	if run.completed, present[3], err = nullableTimeMember(members, "completedAt"); err != nil {
		return checkRun{}, err
	}
	if run.status == statusCompleted {
		for _, ok := range present {
			if !ok {
				return checkRun{}, rejected("completed check run")
			}
		}
	}
	if run.url, err = strictjson.RequiredString(members, keyURL); err != nil || !validCommentURL(run.url) {
		return checkRun{}, rejected("check run url")
	}
	return run, nil
}

// nullableStringMember reads a required member that is null or a string (the
// empty string included).
func nullableStringMember(members map[string]strictjson.Value, key string) (string, bool, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return "", false, rejected(key)
	}
	if value.IsNull() {
		return "", false, nil
	}
	s, err := value.AsString()
	if err != nil {
		return "", false, rejected(key)
	}
	return s, true, nil
}

// nullableTimeMember reads a required member that is null or an RFC 3339
// date-time.
func nullableTimeMember(members map[string]strictjson.Value, key string) (*time.Time, bool, error) {
	s, present, err := nullableStringMember(members, key)
	if err != nil || !present {
		return nil, false, err
	}
	t, ok := parseRFC3339(s)
	if !ok {
		return nil, false, rejected(key)
	}
	return &t, true, nil
}

// optionalTimeMember reads a member that may be missing or null, and is an
// RFC 3339 date-time otherwise.
func optionalTimeMember(members map[string]strictjson.Value, key string) (*time.Time, error) {
	if _, ok := members[key]; !ok {
		return nil, nil
	}
	t, _, err := nullableTimeMember(members, key)
	return t, err
}

// readReviews reads the PR reviews of the reviewer App: its login with the
// bot suffix, of kind Bot. They only add a URL to a completion marker.
func (r *reviewReader) readReviews(ctx context.Context, reviewer Login) ([]prReview, *reviewError) {
	var reviews []prReview
	wantLogin := reviewer.String() + botLoginSuffix
	failure := r.restPages(ctx, "pulls/"+r.pr.String()+"/reviews", func(item strictjson.Value) error {
		obj, err := item.AsObject()
		if err != nil {
			return rejected("review")
		}
		members, err := obj.Collect(keyUser, "commit_id", "submitted_at", "html_url")
		if err != nil {
			return rejected("review")
		}
		login, kind, err := userMember(members)
		if err != nil {
			return err
		}
		// A review of a commit GitHub no longer has may carry no commit_id, and
		// a pending review (one the user has not submitted) has no submitted_at:
		// neither may make the whole read unreadable.
		commit, _, err := nullableStringMember(members, "commit_id")
		if err != nil {
			return rejected("review commit_id")
		}
		submitted, err := optionalTimeMember(members, "submitted_at")
		if err != nil {
			return err
		}
		url, err := strictjson.RequiredString(members, "html_url")
		if err != nil || !validCommentURL(url) {
			return rejected("review html_url")
		}
		if kind == typenameBot && login == wantLogin {
			reviews = append(reviews, prReview{commit: commit, submitted: submitted, url: url})
		}
		return nil
	})
	return reviews, failure
}

// decide applies the state rules to what was read. requests is not empty.
func (r *reviewReader) decide(profile ReviewProfile, requests []requestComment, runs []checkRun, commitIndex map[string]int, reviews []prReview) PRReviewState {
	sortRequests(requests)
	last := len(requests) - 1
	latest := requests[last]
	out := newReviewState(profile, RequestWaiting)
	out.Head, out.Now = r.head, formatReviewTime(r.now)
	out.Request = &ReviewRequest{ID: latest.id, URL: latest.url, CreatedAt: formatReviewTime(latest.created)}
	assigned := make(map[string]struct{})
	// A completion marker of every unedited earlier request whose response is
	// one finished run; the latest request's own is added when it completes.
	// The response of an edited request is assigned too, so it is never shown
	// as an automatic review, but it is never a completion marker.
	for i := range last {
		upper := requests[i+1].updated
		resp := assignResponse(profile, requests[i], &upper, runs, commitIndex, assigned)
		if resp != nil && !requests[i].edited() && resp.kind(profile) == RunFinished {
			out.Markers = append(out.Markers, markerFor(requests[i], *resp, reviews))
		}
	}
	switch {
	case latest.edited():
		out.State = RequestIndeterminate
		out.IndeterminateReason = IndeterminateEditedRequest
		assignResponse(profile, latest, nil, runs, commitIndex, assigned)
	default:
		resp, competing := responseOf(profile, latest, nil, runs, commitIndex)
		switch {
		case len(competing) > 0:
			out.State = RequestIndeterminate
			out.IndeterminateReason = IndeterminateCompetingRuns
			for _, run := range competing {
				out.IndeterminateURLs = append(out.IndeterminateURLs, run.url)
			}
		case resp != nil:
			assigned[resp.url] = struct{}{}
			out.Response = responseOutput(profile, *resp)
			switch resp.kind(profile) {
			case RunFinished:
				out.State = RequestCompleted
				out.Markers = append(out.Markers, markerFor(latest, *resp, reviews))
			case RunRateLimited:
				out.State = RequestRateLimited
			case RunSkipped:
				out.State = RequestSkipped
			case RunRunning:
				if r.now.Sub(latest.created) > reviewWaitLimit {
					out.State = RequestTimedOut
				}
			default:
				// RunUnknown, or any unexpected kind, fails secure as unrecognized.
				out.State = RequestUnrecognized
			}
		default:
			if r.now.Sub(latest.created) > reviewWaitLimit {
				out.State = RequestTimedOut
			}
		}
	}
	sort.SliceStable(out.Markers, func(i, j int) bool {
		if out.Markers[i].CompletedAt != out.Markers[j].CompletedAt {
			return out.Markers[i].CompletedAt < out.Markers[j].CompletedAt
		}
		return out.Markers[i].CheckRunURL < out.Markers[j].CheckRunURL
	})
	if profile == ProfileSourcery {
		excludeUnstarted := out.State == RequestWaiting && out.Response == nil
		out.AutoReview = r.selectAutoReview(runs, assigned, excludeUnstarted)
	}
	return out
}

// assignResponse finds the response of req as responseOf does and, when it is
// one run without competitors, records it in assigned and returns it; it
// returns nil otherwise.
func assignResponse(profile ReviewProfile, req requestComment, upper *time.Time, runs []checkRun, commitIndex map[string]int, assigned map[string]struct{}) *checkRun {
	resp, competing := responseOf(profile, req, upper, runs, commitIndex)
	if len(competing) > 0 || resp == nil {
		return nil
	}
	assigned[resp.url] = struct{}{}
	return resp
}

// sortRequests orders requests by when they were last updated, oldest first.
// Among requests updated in the same second an edited one is later; a last
// tie is broken by the comment id, which grows with creation time.
func sortRequests(requests []requestComment) {
	sort.SliceStable(requests, func(i, j int) bool {
		a, b := requests[i], requests[j]
		if !a.updated.Equal(b.updated) {
			return a.updated.Before(b.updated)
		}
		if a.edited() != b.edited() {
			return !a.edited()
		}
		return a.id < b.id
	})
}

// responseOf finds the response of a request among the runs: the earliest
// counted run that started strictly after the request was created (within
// reviewWaitLimit for ProfileSourcery, and strictly before upper, the next
// request's last update, when upper is not nil). A skip notice is never
// counted: it exists without a request. A run that has not started is not
// counted either. competing lists the runs that compete for the response: two
// or more that started in the earliest second, or one that started before the
// earliest finished (the earliest, when it is still running, never finishes
// before another starts); for ProfileSourcery, an overlapping run on a later
// commit (higher commitIndex) does not compete, while one on the same or an
// earlier commit does. When competing is non-empty, resp is nil.
func responseOf(profile ReviewProfile, req requestComment, upper *time.Time, runs []checkRun, commitIndex map[string]int) (resp *checkRun, competing []checkRun) {
	var counted []checkRun
	for _, run := range runs {
		if run.kind(profile) == RunSkipNotice || run.started == nil || !run.started.After(req.created) {
			continue
		}
		if profile == ProfileSourcery && run.started.Sub(req.created) > reviewWaitLimit {
			continue
		}
		if upper != nil && !run.started.Before(*upper) {
			continue
		}
		counted = append(counted, run)
	}
	if len(counted) == 0 {
		return nil, nil
	}
	sort.SliceStable(counted, func(i, j int) bool { return counted[i].started.Before(*counted[j].started) })
	earliest := counted[0]
	for _, run := range counted[1:] {
		sameSecond := run.started.Equal(*earliest.started)
		beforeEnd := earliest.completed == nil || run.started.Before(*earliest.completed)
		if !sameSecond && !beforeEnd {
			continue
		}
		if !sameSecond && profile == ProfileSourcery && commitIndex[run.commit] > commitIndex[earliest.commit] {
			continue
		}
		if competing == nil {
			competing = []checkRun{earliest}
		}
		competing = append(competing, run)
	}
	if len(competing) > 0 {
		return nil, competing
	}
	return &earliest, nil
}

type autoCandidate struct {
	priority int
	state    AutoReviewState
	kind     RunKind
	run      checkRun
}

// selectAutoReview picks the single non-manual check run on r.head for
// ProfileSourcery by four-step priority:
// 1. running within reviewWaitLimit (started != nil)
// 2. running unstarted (started == nil)
// 3. running past reviewWaitLimit (started != nil -> AutoTimedOut)
// 4. terminal (finished / skipped / unknown)
// Ties within a priority prefer the latest startedAt, then the longest URL,
// then the lexicographically greatest URL. Unstarted runs are excluded while
// the latest manual request is waiting without a response.
func (r *reviewReader) selectAutoReview(runs []checkRun, assigned map[string]struct{}, excludeUnstarted bool) *AutoReview {
	var best *autoCandidate
	for _, run := range runs {
		if run.commit != r.head {
			continue
		}
		if _, taken := assigned[run.url]; taken {
			continue
		}
		kind := run.kind(ProfileSourcery)
		if kind == RunSkipNotice {
			continue
		}
		if excludeUnstarted && run.started == nil {
			continue
		}
		cand := classifyAutoCandidate(run, kind, r.now)
		if best == nil || betterAutoCandidate(cand, *best) {
			c := cand
			best = &c
		}
	}
	if best == nil {
		return nil
	}
	ar := &AutoReview{
		State:      best.state,
		Kind:       best.kind,
		Status:     best.run.status,
		Conclusion: best.run.conclusion,
		Commit:     best.run.commit,
		URL:        best.run.url,
	}
	if best.run.started != nil {
		ar.StartedAt = formatReviewTime(*best.run.started)
	}
	return ar
}

const (
	autoPriorityStartedRunning = iota + 1
	autoPriorityUnstartedRunning
	autoPriorityTimedOut
	autoPriorityTerminal
)

func classifyAutoCandidate(run checkRun, kind RunKind, now time.Time) autoCandidate {
	if kind == RunRunning {
		if run.started == nil {
			return autoCandidate{priority: autoPriorityUnstartedRunning, state: AutoRunning, kind: kind, run: run}
		}
		if now.Sub(*run.started) > reviewWaitLimit {
			return autoCandidate{priority: autoPriorityTimedOut, state: AutoTimedOut, kind: kind, run: run}
		}
		return autoCandidate{priority: autoPriorityStartedRunning, state: AutoRunning, kind: kind, run: run}
	}
	state := AutoUnrecognized
	switch kind {
	case RunFinished:
		state = AutoCompleted
	case RunSkipped:
		state = AutoSkipped
	}
	return autoCandidate{priority: autoPriorityTerminal, state: state, kind: kind, run: run}
}

func betterAutoCandidate(a, b autoCandidate) bool {
	if a.priority != b.priority {
		return a.priority < b.priority
	}
	switch {
	case a.run.started != nil && b.run.started == nil:
		return true
	case a.run.started == nil && b.run.started != nil:
		return false
	case a.run.started != nil && b.run.started != nil && !a.run.started.Equal(*b.run.started):
		return a.run.started.After(*b.run.started)
	}
	if len(a.run.url) != len(b.run.url) {
		return len(a.run.url) > len(b.run.url)
	}
	return a.run.url > b.run.url
}

// markerFor builds the marker of a request answered by a finished run. The PR
// review URL is the first review of the same commit submitted after the
// request, or empty.
func markerFor(req requestComment, resp checkRun, reviews []prReview) ReviewMarker {
	marker := ReviewMarker{
		Commit: resp.commit, RequestID: req.id, RequestURL: req.url, CheckRunURL: resp.url,
		CompletedAt: formatReviewTime(*resp.completed),
	}
	for _, review := range reviews {
		if review.commit == resp.commit && review.submitted != nil && review.submitted.After(req.created) {
			marker.ReviewURL = review.url
			break
		}
	}
	return marker
}

// responseOutput is the output form of a response; a run that has not
// finished has no conclusion.
func responseOutput(profile ReviewProfile, run checkRun) *ReviewResponse {
	out := &ReviewResponse{
		Kind: run.kind(profile), Status: run.status, Conclusion: run.conclusion,
		Commit: run.commit, URL: run.url,
	}
	if run.started != nil {
		out.StartedAt = formatReviewTime(*run.started)
	}
	return out
}
