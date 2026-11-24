// Package ghci reads and waits for GitHub Actions CI checks on a pull request's
// head commit, ignoring external reviewer apps such as CodeRabbit and Sourcery.
//
// It is shared by internal/fixprguard and internal/mergepr so both enforce the
// same GitHub Actions CI boundary. Every response from gh is untrusted: it is
// bounded in size, validated with internal/strictjson, and never echoed into
// error messages.
package ghci

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

const (
	ghCommand         = "gh"
	githubActionsSlug = "github-actions"
	githubURLPrefix   = "https://github.com/"
	fallbackSuiteName = "github-actions-check-suite"

	statusCompleted   = "COMPLETED"
	conclusionSuccess = "SUCCESS"
	conclusionSkipped = "SKIPPED"

	// Fixed bounds on GitHub responses and identifiers.
	maxResponseBytes    = 16 << 20
	maxLoginLength      = 39
	maxRepoNameLength   = 100
	sha1HexLength       = 40
	maxCheckSuites      = 50
	maxCheckRuns        = 100
	expectedCommitNodes = 1

	// JSON member names used across GraphQL and REST responses.
	keyApp         = "app"
	keyCheckRuns   = "checkRuns"
	keyCheckSuites = "checkSuites"
	keyCommit      = "commit"
	keyCommits     = "commits"
	keyConclusion  = "conclusion"
	keyCreatedAt   = "createdAt"
	keyData        = "data"
	keyErrors      = "errors"
	keyHasNextPage = "hasNextPage"
	keyHeadRefOid  = "headRefOid"
	keyName        = "name"
	keyNodes       = "nodes"
	keyOID         = "oid"
	keyPageInfo    = "pageInfo"
	keyPullRequest = "pullRequest"
	keyRepository  = "repository"
	keySlug        = "slug"
	keyStatus      = "status"
	keyURL         = "url"
	keyWorkflow    = "workflow"
	keyWorkflowRun = "workflowRun"
)

// prChecksQuery reads the head commit OID and its check suites and LATEST check
// runs for a pull request. It does not request untrusted free-text fields such
// as output titles, summaries, or details_url.
const prChecksQuery = `query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      headRefOid
      commits(last: 1) {
        nodes {
          commit {
            oid
            checkSuites(first: 50) {
              pageInfo { hasNextPage }
              nodes {
                app { slug }
                status
                conclusion
                createdAt
                workflowRun { workflow { name } }
                checkRuns(first: 100, filterBy: {checkType: LATEST}) {
                  pageInfo { hasNextPage }
                  nodes { name status conclusion url }
                }
              }
            }
          }
        }
      }
    }
  }
}`

var (
	// ErrInvalidRequest reports invalid caller arguments to ResolveRepo,
	// ReadPR, or WaitPR.
	ErrInvalidRequest = errors.New("invalid CI check request")

	errUnknownEnum = errors.New("unknown enum value")
	errRejected    = errors.New("response rejected")
	errLimit       = errors.New("limit exceeded")
)

// State classifies the GitHub Actions CI check status of a pull request head.
// The zero value is StateUnreadable so an uninitialized Result fails closed.
type State int

// CI check states.
const (
	StateUnreadable State = iota // "unreadable"
	StatePending                 // "pending"
	StatePassed                  // "passed"
	StateFailed                  // "failed"
)

// MarshalText encodes the state name and rejects values outside the declared set.
func (s State) MarshalText() ([]byte, error) {
	switch s {
	case StateUnreadable:
		return []byte("unreadable"), nil
	case StatePending:
		return []byte("pending"), nil
	case StatePassed:
		return []byte("passed"), nil
	case StateFailed:
		return []byte("failed"), nil
	default:
		return nil, fmt.Errorf("%w: state", errUnknownEnum)
	}
}

// States returns every declared State value.
func States() []State {
	return []State{StateUnreadable, StatePending, StatePassed, StateFailed}
}

// UnreadableReason explains why CI checks or repository resolution could not be
// read. The zero value is empty and used only when State is not StateUnreadable.
type UnreadableReason int

// Reasons for StateUnreadable and RepoError.
const (
	UnreadableNone        UnreadableReason = iota // ""
	UnreadableCall                                // "call-failed"
	UnreadableRejected                            // "rejected"
	UnreadableLimit                               // "limit"
	UnreadableHeadChanged                         // "head-changed"
)

// MarshalText encodes the reason name and rejects values outside the declared set.
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
	case UnreadableHeadChanged:
		return []byte("head-changed"), nil
	default:
		return nil, fmt.Errorf("%w: unreadable reason", errUnknownEnum)
	}
}

// UnreadableReasons returns every non-empty UnreadableReason value.
func UnreadableReasons() []UnreadableReason {
	return []UnreadableReason{UnreadableCall, UnreadableRejected, UnreadableLimit, UnreadableHeadChanged}
}

// RepoError reports why ResolveRepo failed with a fixed message that never
// carries untrusted output or command arguments.
type RepoError struct {
	Reason UnreadableReason
}

// Error returns a fixed diagnostic string describing the failure reason.
func (e *RepoError) Error() string {
	switch e.Reason {
	case UnreadableCall:
		return "resolve repository: gh call failed"
	case UnreadableRejected:
		return "resolve repository: response rejected"
	case UnreadableLimit:
		return "resolve repository: response size limit exceeded"
	default:
		return "resolve repository: failed"
	}
}

// FailingCheck describes one failing GitHub Actions check run or check suite.
type FailingCheck struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
}

// Request identifies the pull request whose GitHub Actions CI checks are read.
type Request struct {
	Owner        string
	Repo         string
	PRNumber     int
	ExpectedHead string // optional; 40 lowercase hex when set
}

// Result is the evaluated GitHub Actions CI check status of a pull request head.
type Result struct {
	State   State            `json:"state"`
	Reason  UnreadableReason `json:"reason"`
	Head    string           `json:"head"`
	Suites  int              `json:"suites"`
	Checks  int              `json:"checks"`
	Failing []FailingCheck   `json:"failing"`
}

// WaitOptions bounds polling in WaitPR.
type WaitOptions struct {
	PollInterval   time.Duration
	Timeout        time.Duration
	PerCallTimeout time.Duration
	NoCheckGrace   time.Duration
	Sleep          func(context.Context, time.Duration) error
}

// ResolveRepo runs "gh repo view --json nameWithOwner" and strictly parses and
// validates the repository owner and name without echoing untrusted output.
//
// Contract: invalid arguments return ErrInvalidRequest; any gh failure, size
// cap violation, or strict JSON/identifier rejection returns *RepoError.
func ResolveRepo(ctx context.Context, run cmdrunner.Runner) (owner, repo string, err error) {
	if run == nil {
		return "", "", fmt.Errorf("%w: no runner", ErrInvalidRequest)
	}
	out, err := run.Run(ctx, ghCommand, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
			return "", "", &RepoError{Reason: UnreadableLimit}
		}
		return "", "", &RepoError{Reason: UnreadableCall}
	}
	if len(out) > maxResponseBytes {
		return "", "", &RepoError{Reason: UnreadableLimit}
	}
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return "", "", &RepoError{Reason: UnreadableRejected}
	}
	members, err := obj.Collect("nameWithOwner")
	if err != nil {
		return "", "", &RepoError{Reason: UnreadableRejected}
	}
	full, err := strictjson.RequiredString(members, "nameWithOwner")
	if err != nil {
		return "", "", &RepoError{Reason: UnreadableRejected}
	}
	owner, repo, ok := strings.Cut(full, "/")
	if !ok || !validOwner(owner) || !validRepoName(repo) {
		return "", "", &RepoError{Reason: UnreadableRejected}
	}
	return owner, repo, nil
}

// ReadPR reads the head commit's GitHub Actions check suites and LATEST check
// runs once and evaluates their state.
//
// Contract: caller argument errors return ErrInvalidRequest; every external
// failure, schema violation, cap breach, or ExpectedHead mismatch returns a
// StateUnreadable Result with a nil error and a non-nil Failing slice.
func ReadPR(ctx context.Context, run cmdrunner.Runner, req Request) (Result, error) {
	if err := validateRequest(run, req); err != nil {
		return Result{}, err
	}
	out, err := run.Run(
		ctx,
		ghCommand,
		"api",
		"graphql",
		"-f", "owner="+req.Owner,
		"-f", "name="+req.Repo,
		"-F", "number="+strconv.Itoa(req.PRNumber),
		"-f", "query="+prChecksQuery,
	)
	if err != nil {
		if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
			return unreadableResult(UnreadableLimit, ""), nil
		}
		return unreadableResult(UnreadableCall, ""), nil
	}
	if len(out) > maxResponseBytes {
		return unreadableResult(UnreadableLimit, ""), nil
	}
	return parsePRChecks(out, req.ExpectedHead), nil
}

// WaitPR polls ReadPR until GitHub Actions CI checks reach a terminal outcome,
// NoCheckGrace expires with no evaluated suite, or Timeout / ctx expires.
//
// Contract: only transient UnreadableCall and StatePending are retried;
// StatePassed, StateFailed, and permanent unreadable reasons (head-changed,
// rejected, limit) return immediately. Once StatePending with Suites == 0 is
// observed, NoCheckGrace continues counting across transient UnreadableCall
// responses until a non-zero suite appears so flaky calls cannot extend the
// grace window indefinitely. Each ReadPR call is bounded by PerCallTimeout,
// and its child context is always canceled before the next step.
func WaitPR(ctx context.Context, run cmdrunner.Runner, req Request, opts WaitOptions) (Result, error) {
	if err := validateWaitArgs(run, req, opts); err != nil {
		return Result{}, err
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = sleepWithContext
	}
	waitCtx, waitCancel := context.WithTimeout(ctx, opts.Timeout)
	defer waitCancel()

	lastWall := time.Now()
	var elapsed time.Duration
	inNoCheck := false
	var noCheckStart time.Duration
	lastRes := unreadableResult(UnreadableCall, "")

	for {
		if err := waitCtx.Err(); err != nil {
			return lastRes, err
		}
		res, err := readPRWithTimeout(waitCtx, opts.PerCallTimeout, run, req)
		if err != nil {
			return Result{}, err
		}
		now := time.Now()
		if d := now.Sub(lastWall); d > 0 {
			elapsed += d
		}
		lastRes = res

		switch res.State {
		case StatePassed, StateFailed:
			return res, nil
		case StateUnreadable:
			if res.Reason != UnreadableCall {
				return res, nil
			}
			if err := waitCtx.Err(); err != nil {
				return res, err
			}
		case StatePending:
			if res.Suites == 0 {
				if !inNoCheck {
					inNoCheck = true
					noCheckStart = elapsed
				}
				if elapsed-noCheckStart >= opts.NoCheckGrace {
					return res, context.DeadlineExceeded
				}
			} else {
				inNoCheck = false
			}
		default:
			return unreadableResult(UnreadableRejected, res.Head), nil
		}

		remaining := opts.Timeout - elapsed
		if remaining <= 0 {
			return lastRes, context.DeadlineExceeded
		}
		delay := min(opts.PollInterval, remaining)
		if inNoCheck {
			graceRemaining := opts.NoCheckGrace - (elapsed - noCheckStart)
			if graceRemaining <= 0 {
				return lastRes, context.DeadlineExceeded
			}
			delay = min(delay, graceRemaining)
		}
		sleepStart := time.Now()
		if err := sleep(waitCtx, delay); err != nil {
			return lastRes, err
		}
		afterSleep := time.Now()
		elapsed += max(afterSleep.Sub(sleepStart), delay)
		lastWall = afterSleep
	}
}

func readPRWithTimeout(ctx context.Context, timeout time.Duration, run cmdrunner.Runner, req Request) (Result, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return ReadPR(callCtx, run, req)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateRequest(run cmdrunner.Runner, req Request) error {
	if run == nil {
		return fmt.Errorf("%w: no runner", ErrInvalidRequest)
	}
	if !validOwner(req.Owner) {
		return fmt.Errorf("%w: owner", ErrInvalidRequest)
	}
	if !validRepoName(req.Repo) {
		return fmt.Errorf("%w: repo", ErrInvalidRequest)
	}
	if req.PRNumber <= 0 {
		return fmt.Errorf("%w: PR number", ErrInvalidRequest)
	}
	if req.ExpectedHead != "" && !validHex(req.ExpectedHead, sha1HexLength) {
		return fmt.Errorf("%w: expected head", ErrInvalidRequest)
	}
	return nil
}

func validateWaitArgs(run cmdrunner.Runner, req Request, opts WaitOptions) error {
	if err := validateRequest(run, req); err != nil {
		return err
	}
	if opts.PollInterval <= 0 {
		return fmt.Errorf("%w: poll interval", ErrInvalidRequest)
	}
	if opts.Timeout <= 0 {
		return fmt.Errorf("%w: timeout", ErrInvalidRequest)
	}
	if opts.PerCallTimeout <= 0 {
		return fmt.Errorf("%w: per-call timeout", ErrInvalidRequest)
	}
	if opts.NoCheckGrace < 0 {
		return fmt.Errorf("%w: no-check grace", ErrInvalidRequest)
	}
	return nil
}

func unreadableResult(reason UnreadableReason, head string) Result {
	return Result{
		State:   StateUnreadable,
		Reason:  reason,
		Head:    head,
		Failing: []FailingCheck{},
	}
}

type parsedCheckRun struct {
	name       string
	status     string
	conclusion string
	url        string
}

type parsedCheckSuite struct {
	status         string
	conclusion     string
	createdAt      time.Time
	hasWorkflowRun bool
	workflowName   string
	runs           []parsedCheckRun
}

func parsePRChecks(out []byte, expectedHead string) Result {
	headRefOID, suitesObj, err := parseHeadAndSuites(out)
	if err != nil {
		return unreadableResult(UnreadableRejected, "")
	}
	if expectedHead != "" && headRefOID != expectedHead {
		return unreadableResult(UnreadableHeadChanged, headRefOID)
	}
	suites, err := parseCheckSuites(suitesObj)
	if err != nil {
		if errors.Is(err, errLimit) {
			return unreadableResult(UnreadableLimit, headRefOID)
		}
		return unreadableResult(UnreadableRejected, headRefOID)
	}
	return evaluateSuites(headRefOID, suites)
}

func parseHeadAndSuites(out []byte) (headRefOID string, suitesObj strictjson.Object, err error) {
	data, err := parseGraphQLData(out)
	if err != nil {
		return "", strictjson.Object{}, err
	}
	repo, err := memberObject(data, keyRepository)
	if err != nil {
		return "", strictjson.Object{}, err
	}
	pull, err := memberObject(repo, keyPullRequest)
	if err != nil {
		return "", strictjson.Object{}, err
	}
	pullMembers, err := pull.Collect(keyHeadRefOid, keyCommits)
	if err != nil {
		return "", strictjson.Object{}, errRejected
	}
	headRefOID, err = strictjson.RequiredString(pullMembers, keyHeadRefOid)
	if err != nil || !validHex(headRefOID, sha1HexLength) {
		return "", strictjson.Object{}, errRejected
	}
	commitObj, err := parseSingleCommit(pullMembers)
	if err != nil {
		return "", strictjson.Object{}, err
	}
	commitMembers, err := commitObj.Collect(keyOID, keyCheckSuites)
	if err != nil {
		return "", strictjson.Object{}, errRejected
	}
	commitOID, err := strictjson.RequiredString(commitMembers, keyOID)
	if err != nil || !validHex(commitOID, sha1HexLength) || commitOID != headRefOID {
		return "", strictjson.Object{}, errRejected
	}
	suitesVal, err := strictjson.Required(commitMembers, keyCheckSuites)
	if err != nil {
		return "", strictjson.Object{}, errRejected
	}
	suitesObj, err = suitesVal.AsObject()
	if err != nil {
		return "", strictjson.Object{}, errRejected
	}
	return headRefOID, suitesObj, nil
}

func parseSingleCommit(pullMembers map[string]strictjson.Value) (strictjson.Object, error) {
	commitsVal, err := strictjson.Required(pullMembers, keyCommits)
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	commitsObj, err := commitsVal.AsObject()
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	commitNodes, err := memberNodes(commitsObj)
	if err != nil || len(commitNodes) != expectedCommitNodes {
		return strictjson.Object{}, errRejected
	}
	commitWrapper, err := commitNodes[0].AsObject()
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	return memberObject(commitWrapper, keyCommit)
}

func parseGraphQLData(out []byte) (strictjson.Object, error) {
	root, err := strictjson.ParseObject(out)
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	members, err := root.Collect(keyData, keyErrors)
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	if errsVal, ok := members[keyErrors]; ok && !errsVal.IsNull() {
		arr, err := errsVal.AsArray()
		if err != nil || len(arr) > 0 {
			return strictjson.Object{}, errRejected
		}
	}
	dataVal, err := strictjson.Required(members, keyData)
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	dataObj, err := dataVal.AsObject()
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	return dataObj, nil
}

func parseCheckSuites(suitesObj strictjson.Object) ([]parsedCheckSuite, error) {
	if err := requireNoNextPage(suitesObj); err != nil {
		return nil, err
	}
	nodes, err := memberNodes(suitesObj)
	if err != nil {
		return nil, err
	}
	if len(nodes) > maxCheckSuites {
		return nil, errLimit
	}
	var suites []parsedCheckSuite
	for _, node := range nodes {
		suite, include, err := parseCheckSuite(node)
		if err != nil {
			return nil, err
		}
		if include {
			suites = append(suites, suite)
		}
	}
	return suites, nil
}

// parseCheckSuite parses one CheckSuite node. Suites whose app is null or whose
// app.slug differs from "github-actions" are skipped BEFORE inspecting
// checkRuns.pageInfo.hasNextPage so an external app exceeding the check-run cap
// cannot block CI evaluation.
func parseCheckSuite(node strictjson.Value) (parsedCheckSuite, bool, error) {
	obj, err := node.AsObject()
	if err != nil {
		return parsedCheckSuite{}, false, errRejected
	}
	slug, ok, err := suiteAppSlug(obj)
	if err != nil {
		return parsedCheckSuite{}, false, err
	}
	if !ok || slug != githubActionsSlug {
		return parsedCheckSuite{}, false, nil
	}
	members, err := obj.Collect(keyStatus, keyConclusion, keyCreatedAt, keyWorkflowRun, keyCheckRuns)
	if err != nil {
		return parsedCheckSuite{}, false, errRejected
	}
	status, err := strictjson.RequiredString(members, keyStatus)
	if err != nil {
		return parsedCheckSuite{}, false, errRejected
	}
	conclusion, hasConclusion, err := nullableStringMember(members, keyConclusion)
	if err != nil {
		return parsedCheckSuite{}, false, err
	}
	if status == statusCompleted && (!hasConclusion || conclusion == "") {
		return parsedCheckSuite{}, false, errRejected
	}
	createdAtStr, err := strictjson.RequiredString(members, keyCreatedAt)
	if err != nil {
		return parsedCheckSuite{}, false, errRejected
	}
	createdAt, ok := parseRFC3339(createdAtStr)
	if !ok {
		return parsedCheckSuite{}, false, errRejected
	}
	hasWorkflowRun, workflowName, err := parseWorkflowRun(members)
	if err != nil {
		return parsedCheckSuite{}, false, err
	}
	checkRunsVal, err := strictjson.Required(members, keyCheckRuns)
	if err != nil {
		return parsedCheckSuite{}, false, errRejected
	}
	checkRunsObj, err := checkRunsVal.AsObject()
	if err != nil {
		return parsedCheckSuite{}, false, errRejected
	}
	runs, err := parseCheckRuns(checkRunsObj)
	if err != nil {
		return parsedCheckSuite{}, false, err
	}
	return parsedCheckSuite{
		status:         status,
		conclusion:     conclusion,
		createdAt:      createdAt,
		hasWorkflowRun: hasWorkflowRun,
		workflowName:   workflowName,
		runs:           runs,
	}, true, nil
}

func suiteAppSlug(obj strictjson.Object) (string, bool, error) {
	members, err := obj.Collect(keyApp)
	if err != nil {
		return "", false, errRejected
	}
	appVal, err := strictjson.Required(members, keyApp)
	if err != nil {
		return "", false, errRejected
	}
	if appVal.IsNull() {
		return "", false, nil
	}
	appObj, err := appVal.AsObject()
	if err != nil {
		return "", false, errRejected
	}
	slugMembers, err := appObj.Collect(keySlug)
	if err != nil {
		return "", false, errRejected
	}
	slug, err := strictjson.RequiredStringAllowEmpty(slugMembers, keySlug)
	if err != nil {
		return "", false, errRejected
	}
	return slug, true, nil
}

func parseWorkflowRun(members map[string]strictjson.Value) (hasWorkflowRun bool, workflowName string, err error) {
	val, err := strictjson.Required(members, keyWorkflowRun)
	if err != nil {
		return false, "", errRejected
	}
	if val.IsNull() {
		return false, "", nil
	}
	wrObj, err := val.AsObject()
	if err != nil {
		return false, "", errRejected
	}
	wrMembers, err := wrObj.Collect(keyWorkflow)
	if err != nil {
		return false, "", errRejected
	}
	wfVal, err := strictjson.Required(wrMembers, keyWorkflow)
	if err != nil {
		return false, "", errRejected
	}
	if wfVal.IsNull() {
		return true, "", nil
	}
	wfObj, err := wfVal.AsObject()
	if err != nil {
		return false, "", errRejected
	}
	wfMembers, err := wfObj.Collect(keyName)
	if err != nil {
		return false, "", errRejected
	}
	name, _, err := nullableStringMember(wfMembers, keyName)
	if err != nil {
		return false, "", err
	}
	return true, name, nil
}

func parseCheckRuns(checkRunsObj strictjson.Object) ([]parsedCheckRun, error) {
	if err := requireNoNextPage(checkRunsObj); err != nil {
		return nil, err
	}
	nodes, err := memberNodes(checkRunsObj)
	if err != nil {
		return nil, err
	}
	if len(nodes) > maxCheckRuns {
		return nil, errLimit
	}
	runs := make([]parsedCheckRun, 0, len(nodes))
	for _, node := range nodes {
		run, err := parseCheckRun(node)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func parseCheckRun(node strictjson.Value) (parsedCheckRun, error) {
	obj, err := node.AsObject()
	if err != nil {
		return parsedCheckRun{}, errRejected
	}
	members, err := obj.Collect(keyName, keyStatus, keyConclusion, keyURL)
	if err != nil {
		return parsedCheckRun{}, errRejected
	}
	name, err := strictjson.RequiredString(members, keyName)
	if err != nil {
		return parsedCheckRun{}, errRejected
	}
	status, err := strictjson.RequiredString(members, keyStatus)
	if err != nil {
		return parsedCheckRun{}, errRejected
	}
	conclusion, hasConclusion, err := nullableStringMember(members, keyConclusion)
	if err != nil {
		return parsedCheckRun{}, err
	}
	if status == statusCompleted && (!hasConclusion || conclusion == "") {
		return parsedCheckRun{}, errRejected
	}
	rawURL, err := strictjson.RequiredString(members, keyURL)
	if err != nil || !validGitHubURL(rawURL) {
		return parsedCheckRun{}, errRejected
	}
	return parsedCheckRun{
		name:       name,
		status:     status,
		conclusion: conclusion,
		url:        rawURL,
	}, nil
}

// evaluateSuites filters empty un-triggered suites, keeps only the newest
// createdAt suite per named workflow, and applies fail-fast -> pending ->
// passed evaluation.
func evaluateSuites(head string, suites []parsedCheckSuite) Result {
	evaluated := selectEvaluatedSuites(suites)
	totalChecks, failing := collectFailingChecks(evaluated)
	if len(failing) > 0 {
		return Result{
			State:   StateFailed,
			Reason:  UnreadableNone,
			Head:    head,
			Suites:  len(evaluated),
			Checks:  totalChecks,
			Failing: failing,
		}
	}
	if len(evaluated) == 0 || hasPendingSuiteOrRun(evaluated) {
		return Result{
			State:   StatePending,
			Reason:  UnreadableNone,
			Head:    head,
			Suites:  len(evaluated),
			Checks:  totalChecks,
			Failing: failing,
		}
	}
	if !allSuitesAndRunsPassed(evaluated) {
		return Result{State: StateFailed, Head: head, Suites: len(evaluated), Checks: totalChecks, Failing: failing}
	}
	return Result{
		State:   StatePassed,
		Reason:  UnreadableNone,
		Head:    head,
		Suites:  len(evaluated),
		Checks:  totalChecks,
		Failing: failing,
	}
}

func collectFailingChecks(evaluated []parsedCheckSuite) (int, []FailingCheck) {
	totalChecks := 0
	failing := []FailingCheck{}
	for _, suite := range evaluated {
		totalChecks += len(suite.runs)
		suiteHasFailingRun := false
		for _, run := range suite.runs {
			if run.status == statusCompleted && !isPassingConclusion(run.conclusion) {
				failing = append(failing, FailingCheck{
					Name:       run.name,
					Conclusion: run.conclusion,
					URL:        run.url,
				})
				suiteHasFailingRun = true
			}
		}
		if suite.status == statusCompleted && !isPassingConclusion(suite.conclusion) && !suiteHasFailingRun {
			name := suite.workflowName
			if name == "" {
				name = fallbackSuiteName
			}
			failing = append(failing, FailingCheck{
				Name:       name,
				Conclusion: suite.conclusion,
				URL:        "",
			})
		}
	}
	slices.SortFunc(failing, func(a, b FailingCheck) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		if c := cmp.Compare(a.URL, b.URL); c != 0 {
			return c
		}
		return cmp.Compare(a.Conclusion, b.Conclusion)
	})
	return totalChecks, failing
}

func hasPendingSuiteOrRun(evaluated []parsedCheckSuite) bool {
	for _, suite := range evaluated {
		if suite.status != statusCompleted {
			return true
		}
		for _, run := range suite.runs {
			if run.status != statusCompleted {
				return true
			}
		}
	}
	return false
}

func allSuitesAndRunsPassed(evaluated []parsedCheckSuite) bool {
	for _, suite := range evaluated {
		if suite.status != statusCompleted || !isPassingConclusion(suite.conclusion) {
			return false
		}
		for _, run := range suite.runs {
			if run.status != statusCompleted || !isPassingConclusion(run.conclusion) {
				return false
			}
		}
	}
	return true
}

func selectEvaluatedSuites(suites []parsedCheckSuite) []parsedCheckSuite {
	var unnamed []parsedCheckSuite
	latestByWorkflow := map[string]parsedCheckSuite{}
	var workflowOrder []string

	for _, suite := range suites {
		if len(suite.runs) == 0 && !suite.hasWorkflowRun && suite.status != statusCompleted {
			continue
		}
		if suite.hasWorkflowRun && suite.workflowName != "" {
			prev, exists := latestByWorkflow[suite.workflowName]
			if !exists {
				workflowOrder = append(workflowOrder, suite.workflowName)
				latestByWorkflow[suite.workflowName] = suite
			} else if !suite.createdAt.Before(prev.createdAt) {
				latestByWorkflow[suite.workflowName] = suite
			}
			continue
		}
		unnamed = append(unnamed, suite)
	}

	evaluated := make([]parsedCheckSuite, 0, len(workflowOrder)+len(unnamed))
	for _, name := range workflowOrder {
		evaluated = append(evaluated, latestByWorkflow[name])
	}
	evaluated = append(evaluated, unnamed...)
	return evaluated
}

func isPassingConclusion(conclusion string) bool {
	return conclusion == conclusionSuccess || conclusion == conclusionSkipped
}

func memberObject(obj strictjson.Object, key string) (strictjson.Object, error) {
	members, err := obj.Collect(key)
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	val, err := strictjson.Required(members, key)
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	child, err := val.AsObject()
	if err != nil {
		return strictjson.Object{}, errRejected
	}
	return child, nil
}

func memberNodes(obj strictjson.Object) ([]strictjson.Value, error) {
	members, err := obj.Collect(keyNodes)
	if err != nil {
		return nil, errRejected
	}
	val, err := strictjson.Required(members, keyNodes)
	if err != nil {
		return nil, errRejected
	}
	arr, err := val.AsArray()
	if err != nil {
		return nil, errRejected
	}
	return arr, nil
}

func requireNoNextPage(list strictjson.Object) error {
	pageInfo, err := memberObject(list, keyPageInfo)
	if err != nil {
		return err
	}
	members, err := pageInfo.Collect(keyHasNextPage)
	if err != nil {
		return errRejected
	}
	val, err := strictjson.Required(members, keyHasNextPage)
	if err != nil {
		return errRejected
	}
	more, err := val.AsBool()
	if err != nil {
		return errRejected
	}
	if more {
		return errLimit
	}
	return nil
}

func nullableStringMember(members map[string]strictjson.Value, key string) (string, bool, error) {
	val, err := strictjson.Required(members, key)
	if err != nil {
		return "", false, errRejected
	}
	if val.IsNull() {
		return "", false, nil
	}
	s, err := val.AsString()
	if err != nil {
		return "", false, errRejected
	}
	return s, true, nil
}

func validOwner(s string) bool {
	if s == "" || len(s) > maxLoginLength || !isASCIIAlnum(s[0]) {
		return false
	}
	for i := range len(s) {
		if !isASCIIAlnum(s[i]) && s[i] != '-' {
			return false
		}
	}
	return true
}

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

func validHex(s string, length int) bool {
	if len(s) != length {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// validGitHubURL accepts only an https://github.com/ URL with a non-empty path,
// printable ASCII characters without spaces, and no userinfo or alternate host.
func validGitHubURL(s string) bool {
	if !strings.HasPrefix(s, githubURLPrefix) || len(s) <= len(githubURLPrefix) {
		return false
	}
	for i := range len(s) {
		if s[i] <= ' ' || s[i] > '~' {
			return false
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "https" && u.Host == "github.com" && u.User == nil && u.Path != "" && u.Path != "/"
}

// parseRFC3339 reads an RFC 3339 timestamp, rejecting comma fractional
// separators and out-of-range numeric offsets that time.Parse accepts.
func parseRFC3339(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil || strings.Contains(s, ",") {
		return time.Time{}, false
	}
	if strings.HasSuffix(s, "Z") {
		return t, true
	}
	if len(s) < len("+hh:mm") {
		return time.Time{}, false
	}
	offset := s[len(s)-len("+hh:mm"):]
	return t, offset[1:3] <= "23" && offset[4:6] <= "59"
}

func isASCIIAlnum(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
