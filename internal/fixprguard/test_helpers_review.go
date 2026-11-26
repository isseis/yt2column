//go:build test

package fixprguard

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// Fixtures of the review-state tests. Times are UTC; rsCreated is when the
// request comment under test was created.
const (
	rsUser             = "isseis"
	rsRequestText      = "please review"
	rsSourceryReviewer = "sourcery-ai"
	rsSourceryReq1     = "sourcery review"
	rsSourceryReq2     = "sourcery alias request"
	rsCreated          = "2026-11-20T10:00:00Z"
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

func (c rsComment) isRequestFor(requests []string) bool {
	return isRequestComment(c.kind, c.login, rsUser, c.body, requests)
}

// rsRun is one check run; an empty string is null (except name, which defaults
// to checkNameCodeRabbit when empty). url is derived from id.
type rsRun struct {
	id         int64
	name       string
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

func rsSourceryFinished(id int64, started, completed string) rsRun {
	return rsRun{id: id, name: checkNameSourcery, status: statusCompleted, conclusion: conclusionSuccess, title: titleSourceryFinished, started: started, completed: completed}
}

func rsSourcerySkipped(id int64, at string) rsRun {
	return rsRun{id: id, name: checkNameSourcery, status: statusCompleted, conclusion: conclusionSkipped, title: titleSourcerySkipped, started: at, completed: at}
}

func rsSourceryRunning(id int64, started string) rsRun {
	return rsRun{id: id, name: checkNameSourcery, status: "IN_PROGRESS", started: started}
}

func rsSourceryQueued(id int64) rsRun {
	return rsRun{id: id, name: checkNameSourcery, status: "QUEUED"}
}

func (r rsRun) json() map[string]any {
	null := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	name := r.name
	if name == "" {
		name = checkNameCodeRabbit
	}
	return map[string]any{
		keyDatabaseID: r.id, keyName: name, "status": r.status, "conclusion": null(r.conclusion), "title": null(r.title),
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

// rsCommitOf is a commit holding runs in one suite of the default reviewer.
func rsCommitOf(oid string, runs ...rsRun) rsCommit {
	return rsCommit{oid: oid, suites: []rsSuite{{app: testReviewer, runs: runs}}}
}

// rsSourceryCommitOf is a commit holding runs in one suite of Sourcery.
func rsSourceryCommitOf(oid string, runs ...rsRun) rsCommit {
	return rsCommit{oid: oid, suites: []rsSuite{{app: rsSourceryReviewer, runs: runs}}}
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

// rsSourceryBotReview is a PR review by Sourcery.
func rsSourceryBotReview(id int64, commit, submitted string) rsReview {
	return rsReview{login: rsSourceryReviewer + botLoginSuffix, kind: typenameBot, commit: commit, submitted: submitted, id: id}
}

// rsInput scripts a PRReviewState call. profile defaults to ProfileCodeRabbit,
// reviewer to testReviewer (or rsSourceryReviewer for ProfileSourcery),
// requests to []string{rsRequestText} (or Sourcery defaults for ProfileSourcery),
// head to rsHead, and now to rsNow. edit changes the response text of a stage
// (the body of each REST page, the whole GraphQL response); fail makes the
// stage's call fail and ends the script there; stop ends it after the stage.
type rsInput struct {
	profile    ReviewProfile
	reviewer   string
	requests   []string
	comments   []rsComment
	commits    []rsCommit
	reviews    []rsReview
	head       string
	now        string
	pullStatus int // HTTP status of the pull response; http.StatusOK when 0
	edit       map[string]func(string) string
	fail       map[string]error
	// stop ends the script after the named stage, for an input that is
	// rejected there so no later call is made.
	stop string
}

func (in rsInput) resolvedProfile() ReviewProfile {
	if in.profile != ProfileUnknown {
		return in.profile
	}
	return ProfileCodeRabbit
}

func (in rsInput) resolvedReviewer() string {
	if in.reviewer != "" {
		return in.reviewer
	}
	if in.resolvedProfile() == ProfileSourcery {
		return rsSourceryReviewer
	}
	return testReviewer
}

func (in rsInput) resolvedRequests() []string {
	if len(in.requests) > 0 {
		return slices.Clone(in.requests)
	}
	if in.resolvedProfile() == ProfileSourcery {
		return []string{rsSourceryReq1, rsSourceryReq2}
	}
	return []string{rsRequestText}
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
	head := in.head
	if head == "" {
		head = rsHead
	}
	now := in.now
	if now == "" {
		now = rsNow
	}
	if add(rsStagePull, in.pullStep(t, head, now)) || in.stop == rsStagePull {
		return steps
	}
	userOut := `{"login":"` + rsUser + `","id":1}`
	if f := in.edit[rsStageUser]; f != nil {
		userOut = f(userOut)
	}
	if add(rsStageUser, cmdrunnertestutil.Step{Name: "gh", Args: []string{ghAPI, ghUserEndpoint}, Out: userOut}) || in.stop == rsStageUser {
		return steps
	}
	comments, hasRequest := in.commentItems()
	steps = append(steps, in.restSteps(t, rsStageComments, "issues/"+testPR+"/comments", comments)...)
	if in.fail[rsStageComments] != nil || in.stop == rsStageComments {
		return steps
	}
	if !hasRequest && in.resolvedProfile() == ProfileCodeRabbit {
		return steps
	}
	graphql, broke := in.graphqlSteps(t, head)
	steps = append(steps, graphql...)
	if broke || in.stop == rsStageGraphQL || !hasRequest {
		return steps
	}
	return append(steps, in.restSteps(t, rsStageReviews, "pulls/"+testPR+"/reviews", in.reviewItems())...)
}

func (in rsInput) commentItems() (items []any, hasRequest bool) {
	items = make([]any, 0, len(in.comments))
	reqTexts := in.resolvedRequests()
	for _, c := range in.comments {
		items = append(items, c.json())
		hasRequest = hasRequest || c.isRequestFor(reqTexts)
	}
	return items, hasRequest
}

func (in rsInput) reviewItems() []any {
	items := make([]any, 0, len(in.reviews))
	for _, r := range in.reviews {
		items = append(items, r.json())
	}
	return items
}

// pullStep is the gh api --include step for the PR's head.
func (in rsInput) pullStep(t testing.TB, head, now string) cmdrunnertestutil.Step {
	t.Helper()
	body := `{"head":{"sha":"` + head + `","ref":"feature"},"title":"secret-title"}`
	if f := in.edit[rsStagePull]; f != nil {
		body = f(body)
	}
	status := in.pullStatus
	if status == 0 {
		status = http.StatusOK
	}
	return includeStep("pulls/"+testPR, includedOutput(status, []string{"Date: " + rsDate(t, now)}, body), nil)
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

// rsWant is the state a call is expected to return for ProfileCodeRabbit:
// Head and Now as the defaults give them, and the first request (id 1, created
// at rsCreated) when the state has one.
func rsWant(state RequestState, edit ...func(*PRReviewState)) PRReviewState {
	return rsWantFor(ProfileCodeRabbit, state, edit...)
}

// rsWantFor builds an expected PRReviewState for profile.
func rsWantFor(profile ReviewProfile, state RequestState, edit ...func(*PRReviewState)) PRReviewState {
	out := newReviewState(profile, state)
	out.Head, out.Now = rsHead, rsNow
	if state != RequestNone {
		out.Request = &ReviewRequest{ID: 1, URL: rsURL(1), CreatedAt: rsCreated}
	}
	for _, e := range edit {
		e(&out)
	}
	return out
}

// rsUnreadable is the unreadable state with its reason for ProfileCodeRabbit;
// head is "" when the PR's head was not read.
func rsUnreadable(reason UnreadableReason, head string) PRReviewState {
	return rsUnreadableFor(ProfileCodeRabbit, reason, head)
}

// rsUnreadableFor is the unreadable state for profile.
func rsUnreadableFor(profile ReviewProfile, reason UnreadableReason, head string) PRReviewState {
	out := newReviewState(profile, RequestUnreadable)
	out.Reason, out.Head = reason, head
	return out
}

// runReviewState runs PRReviewState over the scripted calls, with the input's
// profile, reviewer, and request texts.
func runReviewState(t *testing.T, in rsInput) PRReviewState {
	t.Helper()
	return runReviewStateStepsFor(t.Context(), t, in.steps(t), ReviewStateRequest{
		PR:       mustPR(t),
		Profile:  in.resolvedProfile(),
		Reviewer: mustLogin(t, in.resolvedReviewer()),
		Requests: in.resolvedRequests(),
	})
}

// runReviewStateSteps runs PRReviewState under ctx over steps for
// ProfileCodeRabbit, for a test whose calls rsInput cannot script.
func runReviewStateSteps(ctx context.Context, t *testing.T, steps []cmdrunnertestutil.Step) PRReviewState {
	t.Helper()
	return runReviewStateStepsFor(ctx, t, steps, ReviewStateRequest{
		PR:       mustPR(t),
		Profile:  ProfileCodeRabbit,
		Reviewer: mustLogin(t, testReviewer),
		Requests: []string{rsRequestText},
	})
}

func runReviewStateStepsFor(ctx context.Context, t *testing.T, steps []cmdrunnertestutil.Step, req ReviewStateRequest) PRReviewState {
	t.Helper()
	g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t, steps...))
	if err != nil {
		t.Fatalf("NewGitHubWithoutReviewer: %v", err)
	}
	state, err := g.PRReviewState(ctx, req)
	if err != nil {
		t.Fatalf("PRReviewState: %v", err)
	}
	return state
}
