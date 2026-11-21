package fixprguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	statusCompleted      = "COMPLETED"
	conclusionSuccess    = "SUCCESS"
	conclusionNeutral    = "NEUTRAL"
	conclusionSkipped    = "SKIPPED"
	titleFinished        = "Finished"
	titleRateLimited     = "Review rate limited"
	titleAutoReviewOff   = "Review skipped: automatic reviews are disabled"
	botLoginSuffix       = "[bot]"
	restPerPage          = 100
	commitsPerPage       = 20
	sha1HexLength        = 40
	maxRequestTextLength = 1024

	// reviewWaitLimit is how long a request is waited for: past it the state is
	// timed-out, even when a check run of the request is still running.
	reviewWaitLimit = 15 * time.Minute

	// Fixed limits; going over one is the unreadable state with reason limit.
	maxReviewComments = 3000
	maxReviewReviews  = 3000
	maxPRCommits      = 1000
	maxCheckSuites    = 50
	maxCheckRuns      = 100
	maxPRCommitPages  = maxPRCommits/commitsPerPage + 1
)

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
		RequestCompleted, RequestRateLimited, RequestUnrecognized, RequestTimedOut,
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
	return []RunKind{RunUnknown, RunRunning, RunFinished, RunRateLimited, RunSkipNotice}
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

// ReviewMarker is a completion marker: a finished review of Commit.
type ReviewMarker struct {
	Commit      string `json:"commit"`
	RequestID   int64  `json:"requestId"`
	RequestURL  string `json:"requestUrl"`
	CheckRunURL string `json:"checkRunUrl"`
	ReviewURL   string `json:"reviewUrl"` // the reviewer's PR review of Commit submitted after the request, or ""
	CompletedAt string `json:"completedAt"`
}

// PRReviewState is the output of review-state. No member holds a comment body, a
// check run's title, or its details_url. Request and Response are null when
// they do not exist; Markers and IndeterminateURLs are empty lists, never
// null.
type PRReviewState struct {
	Protocol            int                 `json:"protocol"`
	State               RequestState        `json:"state"`               // the state of the latest request comment
	Reason              UnreadableReason    `json:"reason"`              // set for state unreadable only
	IndeterminateReason IndeterminateReason `json:"indeterminateReason"` // set for state indeterminate only
	Head                string              `json:"head"`                // the PR's head, 40 lowercase hex; "" only when it could not be read
	Now                 string              `json:"now"`                 // GitHub's clock, RFC 3339 UTC; "" for unreadable
	Request             *ReviewRequest      `json:"request"`             // the latest request comment; null for none and unreadable
	Response            *ReviewResponse     `json:"response"`            // its response; null when it has none
	IndeterminateURLs   []string            `json:"indeterminateUrls"`   // competing check run URLs, for competing-runs only
	Markers             []ReviewMarker      `json:"markers"`             // every completion marker, oldest first
}

// newReviewState returns a state with the lists empty rather than null.
func newReviewState(state RequestState) PRReviewState {
	return PRReviewState{State: state, IndeterminateURLs: []string{}, Markers: []ReviewMarker{}}
}

// ReviewStateRequest names what PRReviewState reads. Reviewer is the login of
// the reviewer App whose check runs and PR reviews count; Request is the exact
// text of a request comment. Only ParseLogin and ParsePRNumber build the
// usable values.
type ReviewStateRequest struct {
	PR       PRNumber
	Reviewer Login
	Request  string
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

// kind classifies the run by the table of status, conclusion and title. A
// COMPLETED run outside the table is RunUnknown and never a finished review.
func (r checkRun) kind() RunKind {
	if r.status != statusCompleted {
		return RunRunning
	}
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

// PRReviewState reads the state of the latest request comment of the PR. It
// returns an error only for an unusable argument; every read failure is the
// unreadable state with its reason, and no partial result is kept. Protocol
// is left for the caller to set. It judges no thread.
func (g *GitHub) PRReviewState(ctx context.Context, req ReviewStateRequest) (PRReviewState, error) {
	if !req.PR.usable() || !req.Reviewer.usable() || req.Request == "" || len(req.Request) > maxRequestTextLength {
		return PRReviewState{}, fmt.Errorf("%w: review state arguments", ErrInvalidInput)
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
		out := newReviewState(RequestUnreadable)
		out.Reason = failure.reason
		out.Head = r.head
		return out, nil
	}
	return state, nil
}

// read runs the reads in order and stops at the first failure: the
// repository, the user, the PR's head, the conversation comments (with no
// request comment, nothing after them is read), the PR's commits with their
// check runs, then the reviewer's PR reviews.
func (r *reviewReader) read(ctx context.Context, req ReviewStateRequest) (PRReviewState, *reviewError) {
	repo, err := r.g.resolveRepository(ctx)
	if err != nil {
		return PRReviewState{}, failReview(UnreadableCall)
	}
	r.repo = repo
	user, failure := r.readUser(ctx)
	if failure != nil {
		return PRReviewState{}, failure
	}
	head, failure := r.readHead(ctx)
	if failure != nil {
		return PRReviewState{}, failure
	}
	r.head = head
	requests, failure := r.readRequests(ctx, user, req.Request)
	if failure != nil {
		return PRReviewState{}, failure
	}
	if len(requests) == 0 {
		out := newReviewState(RequestNone)
		out.Head, out.Now = r.head, formatReviewTime(r.now)
		return out, nil
	}
	runs, failure := r.readRuns(ctx, req.Reviewer)
	if failure != nil {
		return PRReviewState{}, failure
	}
	reviews, failure := r.readReviews(ctx, req.Reviewer)
	if failure != nil {
		return PRReviewState{}, failure
	}
	return r.decide(requests, runs, reviews), nil
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

// restPages reads every page of a REST list, at most limit items in all, and
// hands each item to each. A page with fewer than restPerPage items is the
// last.
func (r *reviewReader) restPages(ctx context.Context, path string, limit int, each func(strictjson.Value) error) *reviewError {
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
		if count > limit {
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
// kind User, and its body is exactly the request text; the same name as a
// Bot, or another user with the same body, is not.
func (r *reviewReader) readRequests(ctx context.Context, user, request string) ([]requestComment, *reviewError) {
	var requests []requestComment
	failure := r.restPages(ctx, "issues/"+r.pr.String()+"/comments", maxReviewComments, func(item strictjson.Value) error {
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
		if kind == typenameUser && login == user && strings.TrimSpace(body) == request {
			requests = append(requests, requestComment{id: id, url: url, created: created, updated: updated})
		}
		return nil
	})
	return requests, failure
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
const reviewCommitsQuery = `query($owner: String!, $name: String!, $number: Int!, $after: String) {
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
                checkRuns(first: 100, filterBy: {checkName: "CodeRabbit", checkType: ALL}) {
                  pageInfo { hasNextPage }
                  nodes { status conclusion title startedAt completedAt url }
                }
              }
            }
          }
        }
      }
    }
  }
}`

// commitsPage is one page of reviewCommitsQuery: the reviewer's check runs of
// its commits, and how to read the next page.
type commitsPage struct {
	commits     int
	runs        []checkRun
	hasNextPage bool
	endCursor   string
}

// readRuns reads every page of the PR's commits and returns the reviewer's
// check runs. Each page must carry the head read first: a push or a force
// push in between discards the whole read.
func (r *reviewReader) readRuns(ctx context.Context, reviewer Login) ([]checkRun, *reviewError) {
	var runs []checkRun
	commits := 0
	after := ""
	for pages := 1; ; pages++ {
		variables := map[string]any{"owner": r.repo.owner, "name": r.repo.name, "number": r.pr.n}
		if after != "" {
			variables["after"] = after
		}
		request, err := json.Marshal(map[string]any{"query": reviewCommitsQuery, "variables": variables})
		if err != nil {
			return nil, failReview(UnreadableCall)
		}
		out, err := r.g.runGH(ctx, request, ghAPI, "graphql", "--input", "-")
		if err != nil {
			return nil, callFailure(err)
		}
		if len(out) > maxResponseBytes {
			return nil, failReview(UnreadableLimit)
		}
		page, err := parseCommitsPage(out, r.head, reviewer.String())
		if err != nil {
			return nil, parseFailure(err)
		}
		commits += page.commits
		if commits > maxPRCommits {
			return nil, failReview(UnreadableLimit)
		}
		runs = append(runs, page.runs...)
		if !page.hasNextPage {
			return runs, nil
		}
		if pages >= maxPRCommitPages {
			return nil, failReview(UnreadableLimit)
		}
		after = page.endCursor
	}
}

// parseCommitsPage checks one page of reviewCommitsQuery. A check suite of
// another App is skipped without reading its runs. A page that holds more
// suites or runs than the query's page size (hasNextPage) is over a limit.
func parseCommitsPage(out []byte, wantHead, reviewer string) (commitsPage, error) {
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
	page.commits = len(nodes)
	for _, node := range nodes {
		runs, err := parseCommitNode(node, reviewer)
		if err != nil {
			return commitsPage{}, err
		}
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

// parseCommitNode reads the reviewer's check runs of one commit node.
func parseCommitNode(node strictjson.Value, reviewer string) ([]checkRun, error) {
	obj, err := node.AsObject()
	if err != nil {
		return nil, rejected("commit node")
	}
	commit, err := memberObject(obj, keyCommit)
	if err != nil {
		return nil, err
	}
	members, err := commit.Collect("oid")
	if err != nil {
		return nil, rejected("commit oid")
	}
	oid, err := strictjson.RequiredString(members, "oid")
	if err != nil || !validHex(oid, sha1HexLength) {
		return nil, rejected("commit oid")
	}
	suites, err := memberObject(commit, "checkSuites")
	if err != nil {
		return nil, err
	}
	if err := requireNoNextPage(suites); err != nil {
		return nil, err
	}
	nodes, err := memberNodes(suites)
	if err != nil {
		return nil, err
	}
	if len(nodes) > maxCheckSuites {
		return nil, errReviewLimit
	}
	var runs []checkRun
	for _, suiteNode := range nodes {
		suiteRuns, err := parseSuite(suiteNode, reviewer, oid)
		if err != nil {
			return nil, err
		}
		runs = append(runs, suiteRuns...)
	}
	return runs, nil
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
func parseSuite(node strictjson.Value, reviewer, commit string) ([]checkRun, error) {
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
		run, err := parseCheckRun(runNode, commit)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// parseCheckRun reads one check run. status is required; conclusion, title,
// startedAt and completedAt are required members that may be null unless the
// status is COMPLETED, when all four must be strings.
func parseCheckRun(node strictjson.Value, commit string) (checkRun, error) {
	obj, err := node.AsObject()
	if err != nil {
		return checkRun{}, rejected("check run")
	}
	members, err := obj.Collect("status", "conclusion", "title", "startedAt", "completedAt", keyURL)
	if err != nil {
		return checkRun{}, rejected("check run")
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
	failure := r.restPages(ctx, "pulls/"+r.pr.String()+"/reviews", maxReviewReviews, func(item strictjson.Value) error {
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
func (r *reviewReader) decide(requests []requestComment, runs []checkRun, reviews []prReview) PRReviewState {
	sortRequests(requests)
	last := len(requests) - 1
	latest := requests[last]
	out := newReviewState(RequestWaiting)
	out.Head, out.Now = r.head, formatReviewTime(r.now)
	out.Request = &ReviewRequest{ID: latest.id, URL: latest.url, CreatedAt: formatReviewTime(latest.created)}
	// A completion marker of every earlier request whose response is one
	// finished run; the latest request's own is added when it completes.
	for i := range last {
		upper := requests[i+1].updated
		if marker, ok := markerOf(requests[i], &upper, runs, reviews); ok {
			out.Markers = append(out.Markers, marker)
		}
	}
	if latest.edited() {
		out.State = RequestIndeterminate
		out.IndeterminateReason = IndeterminateEditedRequest
		return out
	}
	resp, competing := responseOf(latest, nil, runs)
	if len(competing) > 0 {
		out.State = RequestIndeterminate
		out.IndeterminateReason = IndeterminateCompetingRuns
		for _, run := range competing {
			out.IndeterminateURLs = append(out.IndeterminateURLs, run.url)
		}
		return out
	}
	if resp != nil {
		out.Response = responseOutput(*resp)
		switch resp.kind() {
		case RunFinished:
			out.State = RequestCompleted
			out.Markers = append(out.Markers, markerFor(latest, *resp, reviews))
			return out
		case RunRateLimited:
			out.State = RequestRateLimited
			return out
		case RunRunning:
			// Still running; check the wait limit below.
		default:
			// RunUnknown, or any unexpected kind, fails secure as unrecognized.
			out.State = RequestUnrecognized
			return out
		}
	}
	if r.now.Sub(latest.created) > reviewWaitLimit {
		out.State = RequestTimedOut
	}
	return out
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
// counted run that started strictly after the request was created (and
// strictly before upper, the next request's last update, when upper is not
// nil). A skip notice is never counted: it exists without a request. A run
// that has not started is not counted either. competing lists the runs that
// compete for the response: two or more that started in the earliest second,
// or one that started before the earliest finished (the earliest, when it is
// still running, never finishes before another starts); then resp is nil.
func responseOf(req requestComment, upper *time.Time, runs []checkRun) (resp *checkRun, competing []checkRun) {
	var counted []checkRun
	for _, run := range runs {
		if run.kind() == RunSkipNotice || run.started == nil || !run.started.After(req.created) {
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
		if sameSecond || beforeEnd {
			if competing == nil {
				competing = []checkRun{earliest}
			}
			competing = append(competing, run)
		}
	}
	if len(competing) > 0 {
		return nil, competing
	}
	return &earliest, nil
}

// markerOf returns the completion marker of an earlier request: only when the
// request was not edited, its response is determined, and that response is a
// finished run.
func markerOf(req requestComment, upper *time.Time, runs []checkRun, reviews []prReview) (ReviewMarker, bool) {
	if req.edited() {
		return ReviewMarker{}, false
	}
	resp, competing := responseOf(req, upper, runs)
	if len(competing) > 0 || resp == nil || resp.kind() != RunFinished {
		return ReviewMarker{}, false
	}
	return markerFor(req, *resp, reviews), true
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
func responseOutput(run checkRun) *ReviewResponse {
	out := &ReviewResponse{
		Kind: run.kind(), Status: run.status, Conclusion: run.conclusion,
		Commit: run.commit, URL: run.url,
	}
	if run.started != nil {
		out.StartedAt = formatReviewTime(*run.started)
	}
	return out
}
