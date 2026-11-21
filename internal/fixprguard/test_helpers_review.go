//go:build test

package fixprguard

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// Fixtures of the review-state tests. Times are UTC; rsCreated is when the
// request comment under test was created.
const (
	rsUser        = "isseis"
	rsRequestText = "please review"
	rsCreated     = "2026-11-20T10:00:00Z"
	// rsNow is GitHub's clock five minutes after rsCreated.
	rsNow = "2026-11-20T10:05:00Z"

	// rsRunningID is the id of the one running check run the tests use.
	rsRunningID = 11

	rsStageRepo     = "repo"
	rsStageUser     = "user"
	rsStagePull     = "pull"
	rsStageComments = "comments"
	rsStageGraphQL  = "graphql"
	rsStageReviews  = "reviews"
)

var (
	rsHead   = strings.Repeat("a", sha1HexLength)
	rsSecond = strings.Repeat("b", sha1HexLength)
)

func rsURL(id int64) string {
	return "https://github.com/isseis/yt2column/pull/42#issuecomment-" + strconv.FormatInt(id, 10)
}

func rsRunURL(id int64) string {
	return "https://github.com/isseis/yt2column/runs/" + strconv.FormatInt(id, 10)
}

func rsReviewURL(id int64) string {
	return "https://github.com/isseis/yt2column/pull/42#pullrequestreview-" + strconv.FormatInt(id, 10)
}

// rsComment is one conversation comment of the PR. A zero updated is the
// created time (an unedited comment).
type rsComment struct {
	id      int64
	login   string
	kind    string
	body    string
	created string
	updated string
}

// rsRequestComment is a request: the user's own comment with the request text.
func rsRequestComment(id int64, created string) rsComment {
	return rsComment{id: id, login: rsUser, kind: typenameUser, body: rsRequestText, created: created}
}

func (c rsComment) json() map[string]any {
	updated := c.updated
	if updated == "" {
		updated = c.created
	}
	return map[string]any{
		"id": c.id, keyUser: map[string]any{keyLogin: c.login, keyType: c.kind, "id": 1},
		keyBody: c.body, "created_at": c.created, "updated_at": updated, "html_url": rsURL(c.id),
		"author_association": "OWNER",
	}
}

func (c rsComment) isRequest() bool {
	return c.kind == typenameUser && c.login == rsUser && c.body == rsRequestText
}

// rsRun is one check run; an empty string is null. url is derived from id.
type rsRun struct {
	id         int64
	status     string
	conclusion string
	title      string
	started    string
	completed  string
}

func rsFinished(id int64, started, completed string) rsRun {
	return rsRun{id: id, status: statusCompleted, conclusion: conclusionSuccess, title: titleFinished, started: started, completed: completed}
}

func rsRateLimited(id int64, at string) rsRun {
	return rsRun{id: id, status: statusCompleted, conclusion: conclusionNeutral, title: titleRateLimited, started: at, completed: at}
}

func rsSkipNotice(id int64, at string) rsRun {
	return rsRun{id: id, status: statusCompleted, conclusion: conclusionSkipped, title: titleAutoReviewOff, started: at, completed: at}
}

func rsRunning(started string) rsRun {
	return rsRun{id: rsRunningID, status: "IN_PROGRESS", started: started}
}

func rsQueued(id int64) rsRun { return rsRun{id: id, status: "QUEUED"} }

func (r rsRun) json() map[string]any {
	null := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	return map[string]any{
		keyDatabaseID: r.id, "status": r.status, "conclusion": null(r.conclusion), "title": null(r.title),
		"startedAt": null(r.started), "completedAt": null(r.completed), keyURL: rsRunURL(r.id),
		// Members the tool does not read: they must not matter or be printed.
		"detailsUrl": "https://coderabbit.example/details?scope=secret-scope",
	}
}

// rsSuite is a check suite of an App; app is the slug, "" for a null app.
type rsSuite struct {
	app      string
	runs     []rsRun
	moreRuns bool
}

// rsCommit is one commit of the PR and its check suites.
type rsCommit struct {
	oid    string
	suites []rsSuite
	more   bool // the suites' hasNextPage
}

// rsCommitOf is a commit holding runs in one suite of the reviewer.
func rsCommitOf(oid string, runs ...rsRun) rsCommit {
	return rsCommit{oid: oid, suites: []rsSuite{{app: testReviewer, runs: runs}}}
}

func (c rsCommit) json() map[string]any {
	suites := make([]any, 0, len(c.suites))
	for _, s := range c.suites {
		runs := make([]any, 0, len(s.runs))
		for _, r := range s.runs {
			runs = append(runs, r.json())
		}
		var app any
		if s.app != "" {
			app = map[string]any{"slug": s.app}
		}
		suites = append(suites, map[string]any{
			"app":       app,
			"checkRuns": map[string]any{keyPageInfo: map[string]any{keyHasNextPage: s.moreRuns}, nodesKey: runs},
		})
	}
	return map[string]any{keyCommit: map[string]any{
		"oid": c.oid, "checkSuites": map[string]any{keyPageInfo: map[string]any{keyHasNextPage: c.more}, nodesKey: suites},
	}}
}

// rsReview is one PR review.
type rsReview struct {
	login     string
	kind      string
	commit    string
	submitted string // empty is null
	id        int64
}

func (r rsReview) json() map[string]any {
	var submitted any
	if r.submitted != "" {
		submitted = r.submitted
	}
	return map[string]any{
		keyUser:        map[string]any{keyLogin: r.login, keyType: r.kind},
		"commit_id":    r.commit,
		"submitted_at": submitted,
		"html_url":     rsReviewURL(r.id),
		keyBody:        "secret-review-body",
	}
}

// rsBotReview is a PR review by the reviewer App.
func rsBotReview(id int64, commit, submitted string) rsReview {
	return rsReview{login: testReviewer + botLoginSuffix, kind: typenameBot, commit: commit, submitted: submitted, id: id}
}

// rsInput scripts a PRReviewState call. head defaults to rsHead and now to
// rsNow. edit changes the response text of a stage (the body of each REST
// page, the whole GraphQL response); fail makes the stage's call fail and ends
// the script there; stop ends it after the stage.
type rsInput struct {
	comments []rsComment
	commits  []rsCommit
	reviews  []rsReview
	head     string
	now      string
	edit     map[string]func(string) string
	fail     map[string]error
	// stop ends the script after the named stage, for an input that is
	// rejected there so no later call is made.
	stop string
}

// rsDate formats an RFC 3339 time as the Date header gh prints.
func rsDate(t testing.TB, rfc string) string {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		t.Fatalf("parse %q: %v", rfc, err)
	}
	return parsed.UTC().Format(http.TimeFormat)
}

// restSteps is the gh calls that read a REST list in pages of restPerPage,
// ending with a short or empty page.
func (in rsInput) restSteps(t testing.TB, stage, path string, items []any) []cmdrunnertestutil.Step {
	t.Helper()
	now := in.now
	if now == "" {
		now = rsNow
	}
	var steps []cmdrunnertestutil.Step
	for page := 1; ; page++ {
		start := (page - 1) * restPerPage
		end := min(start+restPerPage, len(items))
		chunk := []any{}
		if start < len(items) {
			chunk = items[start:end]
		}
		body := mustJSON(t, chunk)
		if f := in.edit[stage]; f != nil {
			body = f(body)
		}
		out := includedOutput(http.StatusOK, []string{"Date: " + rsDate(t, now)}, body)
		steps = append(steps, includeStep(path+"?per_page=100&page="+strconv.Itoa(page), out, in.fail[stage]))
		if in.fail[stage] != nil || len(chunk) < restPerPage {
			return steps
		}
	}
}

// steps is every gh call of a PRReviewState call in order.
func (in rsInput) steps(t testing.TB) []cmdrunnertestutil.Step {
	t.Helper()
	failed := func(stage string, step cmdrunnertestutil.Step) ([]cmdrunnertestutil.Step, bool) {
		if err := in.fail[stage]; err != nil {
			step.Err = err
			return []cmdrunnertestutil.Step{step}, true
		}
		return []cmdrunnertestutil.Step{step}, false
	}
	var steps []cmdrunnertestutil.Step
	add := func(stage string, step cmdrunnertestutil.Step) bool {
		s, stop := failed(stage, step)
		steps = append(steps, s...)
		return stop
	}
	if add(rsStageRepo, repoStep()) || in.stop == rsStageRepo {
		return steps
	}
	userOut := `{"login":"` + rsUser + `","id":1}`
	if f := in.edit[rsStageUser]; f != nil {
		userOut = f(userOut)
	}
	if add(rsStageUser, cmdrunnertestutil.Step{Name: "gh", Args: []string{ghAPI, keyUser}, Out: userOut}) || in.stop == rsStageUser {
		return steps
	}
	head := in.head
	if head == "" {
		head = rsHead
	}
	now := in.now
	if now == "" {
		now = rsNow
	}
	pullBody := `{"head":{"sha":"` + head + `","ref":"feature"},"title":"secret-title"}`
	if f := in.edit[rsStagePull]; f != nil {
		pullBody = f(pullBody)
	}
	pullOut := includedOutput(http.StatusOK, []string{"Date: " + rsDate(t, now)}, pullBody)
	if add(rsStagePull, includeStep("pulls/"+testPR, pullOut, nil)) || in.stop == rsStagePull {
		return steps
	}
	comments := make([]any, 0, len(in.comments))
	hasRequest := false
	for _, c := range in.comments {
		comments = append(comments, c.json())
		hasRequest = hasRequest || c.isRequest()
	}
	steps = append(steps, in.restSteps(t, rsStageComments, "issues/"+testPR+"/comments", comments)...)
	if in.fail[rsStageComments] != nil || !hasRequest || in.stop == rsStageComments {
		return steps
	}
	graphql, broke := in.graphqlSteps(t, head)
	steps = append(steps, graphql...)
	if broke || in.stop == rsStageGraphQL {
		return steps
	}
	reviews := make([]any, 0, len(in.reviews))
	for _, r := range in.reviews {
		reviews = append(reviews, r.json())
	}
	return append(steps, in.restSteps(t, rsStageReviews, "pulls/"+testPR+"/reviews", reviews)...)
}

// graphqlSteps is the GraphQL pages of the PR's commits, twenty commits each;
// failed is true when the scripted call fails, which ends the script.
func (in rsInput) graphqlSteps(t testing.TB, head string) (steps []cmdrunnertestutil.Step, failed bool) {
	t.Helper()
	for i := 0; ; i += commitsPerPage {
		page := []any{}
		for j := i; j < min(i+commitsPerPage, len(in.commits)); j++ {
			page = append(page, in.commits[j].json())
		}
		more := i+commitsPerPage < len(in.commits)
		cursor := ""
		if more {
			cursor = "cursor" + strconv.Itoa(i+commitsPerPage)
		}
		out := rsCommitsResponse(t, head, page, more, cursor)
		if f := in.edit[rsStageGraphQL]; f != nil {
			out = f(out)
		}
		step := graphqlStep(out)
		if err := in.fail[rsStageGraphQL]; err != nil {
			step.Err = err
			return append(steps, step), true
		}
		steps = append(steps, step)
		if !more {
			return steps, false
		}
	}
}

// rsCommitsResponse is one GraphQL page of the PR's commits.
func rsCommitsResponse(t testing.TB, head string, nodes []any, more bool, cursor string) string {
	t.Helper()
	pageInfo := map[string]any{keyHasNextPage: more, "endCursor": nil}
	if cursor != "" {
		pageInfo["endCursor"] = cursor
	}
	return mustJSON(t, map[string]any{keyData: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
		"headRefOid": head, "commits": map[string]any{keyPageInfo: pageInfo, nodesKey: nodes},
	}}}})
}

// rsServerError is the error gh api --include returns for a 5xx status.
func rsServerError() error {
	return &cmdrunner.ExitError{
		Code:   ghExitFailure,
		Stdout: []byte(includedOutput(http.StatusInternalServerError, []string{"Date: Tue, 10 Nov 2026 00:00:00 GMT"}, `{"message":"secret-message"}`)),
		Stderr: []byte("gh: request failed"),
	}
}

// rsWant is the state a call is expected to return: Head and Now as the
// defaults give them, and the first request (id 1, created at rsCreated) when
// the state has one.
func rsWant(state RequestState, edit ...func(*PRReviewState)) PRReviewState {
	out := newReviewState(state)
	out.Head, out.Now = rsHead, rsNow
	out.Request = &ReviewRequest{ID: 1, URL: rsURL(1), CreatedAt: rsCreated}
	for _, e := range edit {
		e(&out)
	}
	return out
}

// rsUnreadable is the unreadable state with its reason; head is "" when the
// PR's head was not read.
func rsUnreadable(reason UnreadableReason, head string) PRReviewState {
	out := newReviewState(RequestUnreadable)
	out.Reason, out.Head = reason, head
	return out
}

// runReviewState runs PRReviewState over the scripted calls, with the test
// reviewer and the request text.
func runReviewState(t *testing.T, in rsInput) PRReviewState {
	t.Helper()
	g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t, in.steps(t)...))
	if err != nil {
		t.Fatalf("NewGitHubWithoutReviewer: %v", err)
	}
	state, err := g.PRReviewState(t.Context(), ReviewStateRequest{PR: mustPR(t), Reviewer: mustLogin(t, testReviewer), Request: rsRequestText})
	if err != nil {
		t.Fatalf("PRReviewState: %v", err)
	}
	return state
}
