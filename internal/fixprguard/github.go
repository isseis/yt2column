package fixprguard

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

const (
	ghCommand = "gh"

	// githubTimeout bounds every gh call.
	githubTimeout = 2 * time.Minute

	// maxRepoNameLength is GitHub's limit on a repository name.
	maxRepoNameLength = 100

	headFields = "isCrossRepository,headRepository,headRepositoryOwner"
)

// GitHub reads GitHub on behalf of /fixpr and /runpr. It holds no state between
// calls: every call judges threads afresh from the current comments and
// permissions.
type GitHub struct {
	run       cmdrunner.InputRunner
	reviewers Reviewers
}

// NewGitHub fails when reviewers is not usable, so a missing --reviewer stops
// the run instead of trusting no bot or a default one.
func NewGitHub(run cmdrunner.InputRunner, reviewers Reviewers) (*GitHub, error) {
	if run == nil {
		return nil, fmt.Errorf("%w: no runner", ErrInvalidInput)
	}
	if !reviewers.usable() {
		return nil, fmt.Errorf("%w: reviewer list", ErrInvalidInput)
	}
	return &GitHub{run: run, reviewers: reviewers}, nil
}

// NewGitHubWithoutReviewer returns a GitHub for the calls that judge no
// thread: Head, CurrentPR and EditPR. Its reviewers are the zero Reviewers,
// and the calls that judge threads (List, Begin, ThreadPaths, Reply) refuse to
// run on it, so a subcommand that takes no --reviewer cannot judge with none.
func NewGitHubWithoutReviewer(run cmdrunner.InputRunner) (*GitHub, error) {
	if run == nil {
		return nil, fmt.Errorf("%w: no runner", ErrInvalidInput)
	}
	return &GitHub{run: run}, nil
}

// requireReviewer guards every call that judges threads.
func (g *GitHub) requireReviewer() error {
	if !g.reviewers.usable() {
		return fmt.Errorf("%w: reviewer list", ErrInvalidInput)
	}
	return nil
}

// runGH runs gh under githubTimeout, feeding stdin when it is not nil.
func (g *GitHub) runGH(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, githubTimeout)
	defer cancel()
	if stdin == nil {
		return g.run.Run(ctx, ghCommand, args...)
	}
	return g.run.RunInput(ctx, stdin, ghCommand, args...)
}

// ghFailure maps a failed gh call to ErrResponseRejected. The command's error
// is dropped: it carries the arguments and standard error, which must not reach
// the tool's errors or output. Only whether the call timed out is kept.
func ghFailure(rule string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %s timed out", ErrResponseRejected, rule)
	}
	return fmt.Errorf("%w: %s failed", ErrResponseRejected, rule)
}

// repository is the owner and name of the local repository, as gh resolves it.
type repository struct {
	owner string
	name  string
}

// resolveRepository asks gh for the local repository. The owner and name are
// never taken from the caller, and both are checked before they are put into a
// request path.
func (g *GitHub) resolveRepository(ctx context.Context) (repository, error) {
	out, err := g.runGH(ctx, nil, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return repository{}, ghFailure("repo view", err)
	}
	if len(out) > maxResponseBytes {
		return repository{}, rejected("response size")
	}
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return repository{}, rejected("repo view")
	}
	members, err := obj.Collect("nameWithOwner")
	if err != nil {
		return repository{}, rejected("repo view")
	}
	full, err := strictjson.RequiredString(members, "nameWithOwner")
	if err != nil {
		return repository{}, rejected("repo view")
	}
	owner, name, ok := strings.Cut(full, "/")
	if !ok || !validLogin(owner) || !validRepoName(name) {
		return repository{}, rejected("repository name")
	}
	return repository{owner: owner, name: name}, nil
}

// validRepoName accepts ASCII letters, digits, '.', '-' and '_', but not "."
// or "..".
func validRepoName(s string) bool {
	if s == "" || s == "." || s == ".." || len(s) > maxRepoNameLength {
		return false
	}
	for i := range len(s) {
		if !isASCIIAlnum(s[i]) && s[i] != '.' && s[i] != '-' && s[i] != '_' {
			return false
		}
	}
	return true
}

// HeadInfo is the output of head.
type HeadInfo struct {
	Protocol       int  `json:"protocol"`
	SameRepository bool `json:"sameRepository"`
}

// Head confirms that the PR's head is a branch of this repository: only when
// isCrossRepository is the boolean false and headRepository is an object.
// true, null, a missing member, or any other value is ErrCrossRepository.
func (g *GitHub) Head(ctx context.Context, pr PRNumber) (HeadInfo, error) {
	if !pr.usable() {
		return HeadInfo{}, fmt.Errorf("%w: PR number", ErrInvalidInput)
	}
	out, err := g.runGH(ctx, nil, "pr", "view", pr.String(), "--json", headFields)
	if err != nil {
		return HeadInfo{}, ghFailure("pr view", err)
	}
	if len(out) > maxResponseBytes {
		return HeadInfo{}, rejected("response size")
	}
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return HeadInfo{}, rejected("pr view")
	}
	members, err := obj.Collect("isCrossRepository", "headRepository")
	if err != nil {
		return HeadInfo{}, rejected("pr view")
	}
	cross, ok := members["isCrossRepository"]
	if !ok {
		return HeadInfo{}, ErrCrossRepository
	}
	if isCross, err := cross.AsBool(); err != nil || isCross {
		return HeadInfo{}, ErrCrossRepository
	}
	head, ok := members["headRepository"]
	if !ok {
		return HeadInfo{}, ErrCrossRepository
	}
	if _, err := head.AsObject(); err != nil {
		return HeadInfo{}, ErrCrossRepository
	}
	return HeadInfo{Protocol: Protocol, SameRepository: true}, nil
}

// CurrentPR returns the PR of the current branch, as gh resolves it, for a
// subcommand called without a PR number. The number passes the same check as
// one given on the command line.
func (g *GitHub) CurrentPR(ctx context.Context) (PRNumber, error) {
	digits, err := g.currentPRDigits(ctx)
	// ParsePRNumber is the only constructor; on a failure it returns the
	// zero value, which no other call accepts.
	pr, parseErr := ParsePRNumber(digits)
	if err != nil {
		return pr, err
	}
	if parseErr != nil {
		return pr, rejected("PR number")
	}
	return pr, nil
}

// currentPRDigits reads the current branch's PR number in decimal.
func (g *GitHub) currentPRDigits(ctx context.Context) (string, error) {
	out, err := g.runGH(ctx, nil, "pr", "view", "--json", "number")
	if err != nil {
		return "", ghFailure("pr view", err)
	}
	if len(out) > maxResponseBytes {
		return "", rejected("response size")
	}
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return "", rejected("pr view")
	}
	members, err := obj.Collect("number")
	if err != nil {
		return "", rejected("pr view")
	}
	value, err := strictjson.Required(members, "number")
	if err != nil {
		return "", rejected("pr view")
	}
	n, err := value.AsNonNegativeInt64()
	if err != nil {
		return "", rejected("pr view")
	}
	return strconv.FormatInt(n, 10), nil
}

// LookupBudget reports the permission lookups a call used and has left.
type LookupBudget struct {
	Used      int `json:"used"`
	Remaining int `json:"remaining"`
}

// ProcessThread is a thread begin hands to triage: the format /fixpr used
// before, plus the fingerprint. Body joins every fetched comment as
// "@login: text". Named is true for an untrusted thread the caller accepted.
type ProcessThread struct {
	ThreadID    string      `json:"threadId"`
	DatabaseID  int64       `json:"databaseId"`
	AuthorType  string      `json:"authorType"`
	Body        string      `json:"body"`
	Path        string      `json:"path"`
	Line        *int64      `json:"line"`
	URL         string      `json:"url"`
	Fingerprint Fingerprint `json:"fingerprint"`
	Named       bool        `json:"named"`
}

// UntrustedThread is an untrusted thread that is not processed. It has no body
// field: an untrusted author's text is never fetched.
type UntrustedThread struct {
	ThreadID    string      `json:"threadId"`
	URL         string      `json:"url"`
	Fingerprint Fingerprint `json:"fingerprint"`
	Findings    []Finding   `json:"findings"`
}

// Change stages at which a thread was found to have changed.
const (
	// changedSinceAccept: the thread's fingerprint differs from the one the
	// caller named it with.
	changedSinceAccept = "since-accept"
	// changedSinceJudgment: the stage-two fetch saw different comments than the
	// stage-one judgment.
	changedSinceJudgment = "since-judgment"
)

// ChangedThread is a thread not processed because its comments changed.
type ChangedThread struct {
	ThreadID string `json:"threadId"`
	URL      string `json:"url"`
	Stage    string `json:"stage"`
}

// Round is the output of begin. Only Threads carries bodies, and only of the
// threads judged trusted or named with a matching fingerprint.
type Round struct {
	Protocol          int               `json:"protocol"`
	Number            int               `json:"number"`
	CapHit            bool              `json:"capHit"`
	Threads           []ProcessThread   `json:"threads"`
	Untrusted         []UntrustedThread `json:"untrusted"`
	Changed           []ChangedThread   `json:"changed"`
	AcceptNotFound    []string          `json:"acceptNotFound"`
	PermissionLookups LookupBudget      `json:"permissionLookups"`
}

// Begin judges the unresolved threads on the first page (100 threads; CapHit
// reports more) and fetches bodies only for the threads it will process: the
// trusted ones and those accepted with their current fingerprint. A thread
// whose comments differ between the judgment and the body fetch is reported as
// changed and not processed. It checks the PR head first.
func (g *GitHub) Begin(ctx context.Context, pr PRNumber, accept []Accept) (Round, error) {
	if err := g.requireReviewer(); err != nil {
		return Round{}, err
	}
	accepted, err := checkAccepts(accept)
	if err != nil {
		return Round{}, err
	}
	if _, err := g.Head(ctx, pr); err != nil {
		return Round{}, err
	}
	repo, err := g.resolveRepository(ctx)
	if err != nil {
		return Round{}, err
	}
	page, err := g.fetchThreadPage(ctx, repo, pr, "")
	if err != nil {
		return Round{}, err
	}
	round := Round{
		Protocol:       Protocol,
		Number:         pr.n,
		CapHit:         page.hasNextPage,
		Threads:        []ProcessThread{},
		Untrusted:      []UntrustedThread{},
		Changed:        []ChangedThread{},
		AcceptNotFound: []string{},
	}
	perms := newPermissionLookups(g, repo)
	var selected []thread
	named := map[string]struct{}{}
	seen := map[string]struct{}{}
	for _, t := range page.threads {
		if t.resolved {
			continue
		}
		seen[t.id] = struct{}{}
		assessment := assessThread(ctx, authorsOf(t), !t.moreComments, g.reviewers, perms.lookup)
		fp := fingerprint(t)
		acc, isNamed := accepted[t.id]
		switch {
		case assessment.Trusted:
			selected = append(selected, t)
		case isNamed && acc.Fingerprint == fp && len(t.comments) > 0:
			selected = append(selected, t)
			named[t.id] = struct{}{}
		default:
			round.Untrusted = append(round.Untrusted, UntrustedThread{
				ThreadID: t.id, URL: threadURL(t), Fingerprint: fp, Findings: assessment.Findings,
			})
			if isNamed && acc.Fingerprint != fp {
				round.Changed = append(round.Changed, ChangedThread{ThreadID: t.id, URL: threadURL(t), Stage: changedSinceAccept})
			}
		}
	}
	for _, a := range accept {
		if _, ok := seen[a.ThreadID]; !ok {
			round.AcceptNotFound = append(round.AcceptNotFound, a.ThreadID)
		}
	}
	round.PermissionLookups = perms.budget()
	if err := g.addBodies(ctx, &round, selected, named); err != nil {
		return Round{}, err
	}
	return round, nil
}

// addBodies fetches the selected threads with their bodies and adds each to
// round.Threads, or to round.Changed when its comments differ from the ones
// judged.
func (g *GitHub) addBodies(ctx context.Context, round *Round, selected []thread, named map[string]struct{}) error {
	if len(selected) == 0 {
		return nil
	}
	ids := make([]string, len(selected))
	for i, t := range selected {
		ids[i] = t.id
	}
	withBodies, err := g.fetchBodies(ctx, ids)
	if err != nil {
		return err
	}
	for i, t := range withBodies {
		judged := selected[i]
		if t.resolved || fingerprint(t) != fingerprint(judged) {
			round.Changed = append(round.Changed, ChangedThread{ThreadID: t.id, URL: threadURL(judged), Stage: changedSinceJudgment})
			continue
		}
		_, isNamed := named[t.id]
		round.Threads = append(round.Threads, processThread(t, isNamed, g.reviewers))
	}
	return nil
}

// processThread renders a fetched thread in the triage format. The first
// comment gives the database ID, path, line and URL. authorType is "Bot" only
// when the first author is a Bot of the reviewers, judged by the trustsBot
// that assessThread uses, and "User" otherwise, so another bot's thread a user
// named is answered like a person's. Begin never selects a thread without
// comments.
func processThread(t thread, named bool, reviewers Reviewers) ProcessThread {
	first := t.comments[0]
	authorType := typenameUser
	if reviewers.trustsBot(authorsOf(t)[0]) {
		authorType = typenameBot
	}
	parts := make([]string, 0, len(t.comments))
	for _, c := range t.comments {
		name := "unknown"
		if login, err := ParseLogin(c.loginRaw); err == nil && c.authorPresent {
			name = login.String()
		}
		parts = append(parts, "@"+name+": "+c.body)
	}
	return ProcessThread{
		ThreadID:    t.id,
		DatabaseID:  first.databaseID,
		AuthorType:  authorType,
		Body:        strings.Join(parts, "\n\n"),
		Path:        first.path,
		Line:        first.line,
		URL:         first.url,
		Fingerprint: fingerprint(t),
		Named:       named,
	}
}

// threadURL is the first comment's URL; a thread with no comments has none.
func threadURL(t thread) string {
	if len(t.comments) == 0 {
		return ""
	}
	return t.comments[0].url
}

// ListedThread is one unresolved thread in list's output. It has no body
// field: /runpr never reads a thread body.
type ListedThread struct {
	ThreadID    string      `json:"threadId"`
	URL         string      `json:"url"`
	Trusted     bool        `json:"trusted"`
	Fingerprint Fingerprint `json:"fingerprint"`
	Findings    []Finding   `json:"findings"`
}

// ThreadList is the output of list.
type ThreadList struct {
	Protocol          int            `json:"protocol"`
	Threads           []ListedThread `json:"threads"`
	PermissionLookups LookupBudget   `json:"permissionLookups"`
}

// List judges every unresolved thread over all pages, without fetching any
// body. It checks the PR head first and refuses a PR with more than
// maxListThreads threads.
func (g *GitHub) List(ctx context.Context, pr PRNumber) (ThreadList, error) {
	if err := g.requireReviewer(); err != nil {
		return ThreadList{}, err
	}
	if _, err := g.Head(ctx, pr); err != nil {
		return ThreadList{}, err
	}
	repo, err := g.resolveRepository(ctx)
	if err != nil {
		return ThreadList{}, err
	}
	threads, err := g.fetchAllThreads(ctx, repo, pr)
	if err != nil {
		return ThreadList{}, err
	}
	perms := newPermissionLookups(g, repo)
	list := ThreadList{Protocol: Protocol, Threads: []ListedThread{}}
	for _, t := range threads {
		if t.resolved {
			continue
		}
		assessment := assessThread(ctx, authorsOf(t), !t.moreComments, g.reviewers, perms.lookup)
		findings := assessment.Findings
		if findings == nil {
			findings = []Finding{}
		}
		list.Threads = append(list.Threads, ListedThread{
			ThreadID: t.id, URL: threadURL(t), Trusted: assessment.Trusted, Fingerprint: fingerprint(t), Findings: findings,
		})
	}
	list.PermissionLookups = perms.budget()
	return list, nil
}

// ThreadPath is the path of a thread handed to the fix phase. Named is true
// when the thread is not trusted and was accepted by name; the scope rules are
// stricter for such a thread, because its author chose the path.
type ThreadPath struct {
	ThreadID string
	Path     string
	Named    bool
}

// ThreadPaths fetches the given threads afresh and returns their paths, so the
// caller cannot widen the change scope by naming a path. Each thread must be
// unresolved and still have the fingerprint the caller holds; otherwise the
// call fails with ErrThreadChanged. A thread that is not trusted now is
// returned as Named: holding its current fingerprint is the only proof of
// naming the tool can check.
func (g *GitHub) ThreadPaths(ctx context.Context, pr PRNumber, threads []Accept) ([]ThreadPath, error) {
	if err := g.requireReviewer(); err != nil {
		return nil, err
	}
	if _, err := checkAccepts(threads); err != nil {
		return nil, err
	}
	if !pr.usable() {
		return nil, fmt.Errorf("%w: PR number", ErrInvalidInput)
	}
	repo, err := g.resolveRepository(ctx)
	if err != nil {
		return nil, err
	}
	byID, err := g.unresolvedThreads(ctx, repo, pr)
	if err != nil {
		return nil, err
	}
	perms := newPermissionLookups(g, repo)
	paths := make([]ThreadPath, 0, len(threads))
	for _, want := range threads {
		t, ok := byID[want.ThreadID]
		if !ok || fingerprint(t) != want.Fingerprint || len(t.comments) == 0 {
			return nil, ErrThreadChanged
		}
		assessment := assessThread(ctx, authorsOf(t), !t.moreComments, g.reviewers, perms.lookup)
		paths = append(paths, ThreadPath{ThreadID: t.id, Path: t.comments[0].path, Named: !assessment.Trusted})
	}
	return paths, nil
}

// unresolvedThreads reads every page of the PR's threads without bodies and
// returns the unresolved ones by ID, for the calls that judge given threads
// afresh.
func (g *GitHub) unresolvedThreads(ctx context.Context, repo repository, pr PRNumber) (map[string]thread, error) {
	fetched, err := g.fetchAllThreads(ctx, repo, pr)
	if err != nil {
		return nil, err
	}
	byID := map[string]thread{}
	for _, t := range fetched {
		if !t.resolved {
			byID[t.id] = t
		}
	}
	return byID, nil
}
