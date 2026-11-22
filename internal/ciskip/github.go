package ciskip

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
)

const (
	ghHostname             = "github.com"
	ciWorkflowPath         = ".github/workflows/ci.yml"
	pullRequestEvent       = "pull_request"
	statusCompleted        = "completed"
	conclusionSuccess      = "success"
	conclusionFailure      = "failure"
	conclusionSkipped      = "skipped"
	annotationLevelNotice  = "notice"
	baseSHAAnnotationTitle = "ci-base-sha"

	checkChangesJobName = "Check changed files"
	testJobName         = "Test"
	lintJobName         = "Code Quality & Lint"
	extensionJobName    = "Browser extension"
)

// requiredJobNames lists the four CI job display names that must each appear
// exactly once and be completed with an acceptable conclusion on the parent run.
var requiredJobNames = []string{
	checkChangesJobName,
	testJobName,
	lintJobName,
	extensionJobName,
}

// statusLinePattern matches the first line of `gh api -i` output (LF-terminated,
// no CR allowed): HTTP/1.1, HTTP/2, or HTTP/2.0 followed by a 3-digit status code.
var statusLinePattern = regexp.MustCompile(`^HTTP/(?:1\.1|2|2\.0) ([0-9]{3})(?: [^\r\n]*)?$`)

type httpHeader struct {
	name  string
	value string
}

type workflowRun struct {
	id               int64
	path             string
	event            string
	headSHA          string
	status           string
	conclusion       string
	createdAt        time.Time
	runStartedAt     time.Time
	repoFullName     string
	headRepoNull     bool
	headRepoFullName string
	pullRequests     []int64
}

type workflowJob struct {
	id         int64
	name       string
	headSHA    string
	status     string
	conclusion string
}

type checkAnnotation struct {
	level        string
	titlePresent bool
	title        string
	message      string
}

// verifyParentCI executes the three GitHub REST API checks (Runs, Jobs, Annotations)
// against the parent commit and writes the final Reason (and ParentRunID / HTTPStatus)
// into dec.
// Invariant: never leaks non-200 bodies or stderr into dec, never follows Link URLs,
// and sets ReasonSkipped only after every run, job, and annotation check passes.
func (e *Evaluator) verifyParentCI(
	ctx context.Context,
	repo string,
	prNumber int64,
	parentSHA string,
	mergeBaseSHA string,
	dec *Decision,
) {
	latestRun, reason, status, ok := e.findLatestParentRun(ctx, repo, prNumber, parentSHA)
	if !ok {
		dec.Reason = reason
		dec.HTTPStatus = status
		return
	}
	dec.ParentRunID = latestRun.id

	if latestRun.conclusion != conclusionSuccess && latestRun.conclusion != conclusionFailure {
		dec.Reason = ReasonParentRunBadResult
		return
	}

	checkChangesJobID, reason, status, ok := e.verifyParentJobs(ctx, repo, latestRun.id, parentSHA)
	if !ok {
		dec.Reason = reason
		dec.HTTPStatus = status
		return
	}

	if reason, status, ok := e.verifyParentBaseAnnotation(ctx, repo, checkChangesJobID, mergeBaseSHA); !ok {
		dec.Reason = reason
		dec.HTTPStatus = status
		return
	}

	dec.Reason = ReasonSkipped
}

// ghAPIGet calls `gh api --hostname github.com -i <endpoint>`, enforces the per-command
// and overall timeouts, validates the LF status line and CRLF header block, and returns
// the HTTP 200 body and whether a Link header declares a next page.
// Invariant: non-200 headers/bodies and ExitError.Stderr are discarded immediately;
// httpStatus is non-zero only when a parseable non-200 HTTP status code was observed.
func (e *Evaluator) ghAPIGet(ctx context.Context, endpoint string) (body []byte, hasNext bool, reason Reason, httpStatus int, ok bool) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, false, ReasonAPITimeout, 0, false
	}

	cmdCtx := ctx
	if e.limits.commandTimeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, e.limits.commandTimeout)
		defer cancel()
	}

	out, err := e.gh.Run(cmdCtx, "gh", "api", "--hostname", ghHostname, "-i", endpoint)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(cmdCtx.Err(), context.DeadlineExceeded) || errors.Is(err, exec.ErrWaitDelay) {
			return nil, false, ReasonAPITimeout, 0, false
		}
		if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
			return nil, false, ReasonAPIOutputTooLarge, 0, false
		}
		if exitErr, isExit := errors.AsType[*cmdrunner.ExitError](err); isExit {
			if code, parsed := parseStatusCode(exitErr.Stdout); parsed && code != http.StatusOK {
				return nil, false, ReasonAPIFailed, code, false
			}
		}
		return nil, false, ReasonAPIFailed, 0, false
	}

	code, parsed := parseStatusCode(out)
	if !parsed {
		return nil, false, ReasonAPIInvalidResponse, 0, false
	}
	if code != http.StatusOK {
		return nil, false, ReasonAPIFailed, code, false
	}

	headers, body, splitOK := splitGHResponse(out)
	if !splitOK {
		return nil, false, ReasonAPIInvalidResponse, 0, false
	}

	next, linkOK := hasNextPage(headers)
	if !linkOK {
		return nil, false, ReasonAPIInvalidResponse, 0, false
	}
	return body, next, ReasonUnspecified, 0, true
}

// parseStatusCode extracts the 3-digit HTTP status code from the LF-terminated
// first line of `gh api -i` output.
func parseStatusCode(out []byte) (int, bool) {
	first, _, found := bytes.Cut(out, []byte{'\n'})
	if !found {
		return 0, false
	}
	m := statusLinePattern.FindSubmatch(first)
	if m == nil {
		return 0, false
	}
	code, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0, false
	}
	return code, true
}

// splitGHResponse separates `gh api -i` output after the LF-terminated status line
// into CRLF-terminated header lines and the body following the first blank CRLF line.
func splitGHResponse(out []byte) ([]httpHeader, []byte, bool) {
	_, rest, found := bytes.Cut(out, []byte{'\n'})
	if !found {
		return nil, nil, false
	}
	var headers []httpHeader
	for {
		line, after, found := bytes.Cut(rest, []byte("\r\n"))
		if !found {
			return nil, nil, false
		}
		if len(line) == 0 {
			return headers, after, true
		}
		if bytes.ContainsAny(line, "\r\n") {
			return nil, nil, false
		}
		name, value, hasColon := strings.Cut(string(line), ":")
		if !hasColon || name == "" || strings.ContainsAny(name, " \t") {
			return nil, nil, false
		}
		headers = append(headers, httpHeader{name: name, value: value})
		rest = after
	}
}

// hasNextPage parses every Link header (case-insensitive header name) per RFC 8288
// and reports whether any link item declares rel="next" or rel=next.
// Invariant: text inside <URI> or inside other quoted parameter values is never
// misread as rel=next; malformed Link headers return ok=false (fail-safe).
func hasNextPage(headers []httpHeader) (hasNext bool, ok bool) {
	for _, h := range headers {
		if !strings.EqualFold(h.name, "Link") {
			continue
		}
		next, parsed := parseLinkHeaderHasNext(h.value)
		if !parsed {
			return false, false
		}
		if next {
			hasNext = true
		}
	}
	return hasNext, true
}

// parseLinkHeaderHasNext structurally parses one RFC 8288 Link header field value.
func parseLinkHeaderHasNext(value string) (hasNext bool, ok bool) {
	i := skipOWS(value, 0)
	if i >= len(value) {
		return false, true
	}
	for i < len(value) {
		if value[i] != '<' {
			return false, false
		}
		endURI := strings.IndexByte(value[i+1:], '>')
		if endURI < 0 {
			return false, false
		}
		// Advance past both '<' and '>' of <URI>; the URI itself is never followed.
		i += endURI + len("<>")
		i = skipOWS(value, i)
		seenRel := false

		for i < len(value) && value[i] == ';' {
			paramName, paramVal, hasVal, quoted, nextIdx, valid := parseLinkParam(value, i+1)
			if !valid {
				return false, false
			}
			i = skipOWS(value, nextIdx)
			// Per RFC 8288 section 3.3, only the first rel parameter in a link-value applies.
			if strings.EqualFold(paramName, "rel") && !seenRel {
				seenRel = true
				if hasVal && relContainsNext(paramVal, quoted) {
					hasNext = true
				}
			}
		}

		if i >= len(value) {
			break
		}
		if value[i] != ',' {
			return false, false
		}
		i = skipOWS(value, i+1)
		if i >= len(value) {
			return false, false
		}
	}
	return hasNext, true
}

func skipOWS(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// parseLinkParam parses a single link-param starting right after ';'.
func parseLinkParam(value string, start int) (name, val string, hasVal, quoted bool, nextIdx int, ok bool) {
	i := skipOWS(value, start)
	name, i, ok = readLinkToken(value, i)
	if !ok {
		return "", "", false, false, start, false
	}
	i = skipOWS(value, i)
	if i >= len(value) || value[i] == ';' || value[i] == ',' {
		return name, "", false, false, i, true
	}
	if value[i] != '=' {
		return "", "", false, false, start, false
	}
	i = skipOWS(value, i+1)
	if i >= len(value) {
		return "", "", false, false, start, false
	}
	if value[i] == '"' {
		quoted = true
		val, i, ok = readLinkQuotedString(value, i)
	} else {
		val, i, ok = readLinkToken(value, i)
	}
	if !ok {
		return "", "", false, false, start, false
	}
	return name, val, true, quoted, i, true
}

// readLinkToken reads a non-empty unquoted RFC 8288 parameter name or token value.
func readLinkToken(s string, start int) (string, int, bool) {
	i := start
	for i < len(s) {
		c := s[i]
		if c == ' ' || c == '\t' || c == ';' || c == ',' || c == '=' {
			break
		}
		if c == '"' || c < 0x21 || c > 0x7e {
			return "", start, false
		}
		i++
	}
	if i == start {
		return "", start, false
	}
	return s[start:i], i, true
}

// readLinkQuotedString reads an RFC 7230/8288 quoted-string starting at opening '"'.
func readLinkQuotedString(s string, start int) (string, int, bool) {
	if start >= len(s) || s[start] != '"' {
		return "", start, false
	}
	var b strings.Builder
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		if c == '\\' {
			if i+1 >= len(s) {
				return "", start, false
			}
			i++
			b.WriteByte(s[i])
			continue
		}
		if c == '"' {
			return b.String(), i + 1, true
		}
		b.WriteByte(c)
	}
	return "", start, false
}

// relContainsNext reports whether a rel parameter value designates "next".
// Per RFC 8288 section 3.3, a quoted rel value may hold space-separated relation types.
func relContainsNext(val string, quoted bool) bool {
	if !quoted {
		return strings.EqualFold(val, "next")
	}
	for field := range strings.FieldsSeq(val) {
		if strings.EqualFold(field, "next") {
			return true
		}
	}
	return false
}

type prRunMatch uint8

const (
	prRunIgnore prRunMatch = iota
	prRunMatched
	prRunAmbiguous
)

func classifyRunForPR(r workflowRun, repo, parentSHA string, prNumber int64) prRunMatch {
	if r.headRepoNull ||
		r.path != ciWorkflowPath ||
		r.event != pullRequestEvent ||
		r.headSHA != parentSHA ||
		r.repoFullName != repo ||
		r.headRepoFullName != repo {
		return prRunIgnore
	}
	switch len(r.pullRequests) {
	case 0:
		return prRunAmbiguous
	case 1:
		if r.pullRequests[0] == prNumber {
			return prRunMatched
		}
		return prRunIgnore
	default:
		if slices.Contains(r.pullRequests, prNumber) {
			return prRunAmbiguous
		}
		return prRunIgnore
	}
}

func selectLatestCompletedRun(matched []workflowRun) (workflowRun, Reason, bool) {
	if len(matched) == 0 {
		return workflowRun{}, ReasonNoParentRun, false
	}
	for _, r := range matched {
		if r.status != statusCompleted {
			return workflowRun{}, ReasonParentRunIncomplete, false
		}
	}
	latest := matched[0]
	for _, r := range matched[1:] {
		if compareWorkflowRuns(r, latest) > 0 {
			latest = r
		}
	}
	return latest, ReasonUnspecified, true
}

// findLatestParentRun paginates the CI workflow runs for parentSHA, fails closed on
// ambiguous same-repo PR links or any non-completed matching run, and returns the
// latest completed matching run by (run_started_at, created_at, id).
func (e *Evaluator) findLatestParentRun(
	ctx context.Context,
	repo string,
	prNumber int64,
	parentSHA string,
) (workflowRun, Reason, int, bool) {
	var matched []workflowRun
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf(
			"repos/%s/actions/workflows/ci.yml/runs?event=pull_request&head_sha=%s&per_page=100&page=%d",
			repo, parentSHA, page,
		)
		body, hasNext, reason, status, ok := e.ghAPIGet(ctx, endpoint)
		if !ok {
			return workflowRun{}, reason, status, false
		}
		runs, parsed := parseWorkflowRunsPage(body)
		if !parsed {
			return workflowRun{}, ReasonAPIInvalidResponse, 0, false
		}
		for _, r := range runs {
			switch classifyRunForPR(r, repo, parentSHA, prNumber) {
			case prRunMatched:
				matched = append(matched, r)
			case prRunAmbiguous:
				return workflowRun{}, ReasonAmbiguousPRLink, 0, false
			case prRunIgnore:
			}
		}
		if !hasNext {
			break
		}
		if page >= e.limits.maxRunPages {
			return workflowRun{}, ReasonAPIPaginationLimit, 0, false
		}
	}

	latest, reason, ok := selectLatestCompletedRun(matched)
	if !ok {
		return workflowRun{}, reason, 0, false
	}
	return latest, ReasonUnspecified, 0, true
}

func compareWorkflowRuns(a, b workflowRun) int {
	if c := a.runStartedAt.Compare(b.runStartedAt); c != 0 {
		return c
	}
	if c := a.createdAt.Compare(b.createdAt); c != 0 {
		return c
	}
	return cmp.Compare(a.id, b.id)
}

func parseWorkflowRunsPage(body []byte) ([]workflowRun, bool) {
	obj, err := strictjson.ParseObject(body)
	if err != nil {
		return nil, false
	}
	top, err := obj.CollectExactCase("workflow_runs")
	if err != nil {
		return nil, false
	}
	runsVal, err := strictjson.Required(top, "workflow_runs")
	if err != nil {
		return nil, false
	}
	rawRuns, err := runsVal.AsArray()
	if err != nil {
		return nil, false
	}

	runs := make([]workflowRun, 0, len(rawRuns))
	for _, elem := range rawRuns {
		run, ok := parseWorkflowRun(elem)
		if !ok {
			return nil, false
		}
		runs = append(runs, run)
	}
	return runs, true
}

func parseWorkflowRun(val strictjson.Value) (workflowRun, bool) {
	obj, err := val.AsObject()
	if err != nil {
		return workflowRun{}, false
	}
	m, err := obj.CollectExactCase(
		"id",
		"path",
		"event",
		"head_sha",
		"status",
		"conclusion",
		"created_at",
		"run_started_at",
		"repository",
		"head_repository",
		"pull_requests",
	)
	if err != nil {
		return workflowRun{}, false
	}

	id, ok := requiredPositiveInt64(m, "id")
	if !ok {
		return workflowRun{}, false
	}
	path, err := strictjson.RequiredStringAllowEmpty(m, "path")
	if err != nil {
		return workflowRun{}, false
	}
	event, err := strictjson.RequiredStringAllowEmpty(m, "event")
	if err != nil {
		return workflowRun{}, false
	}
	headSHA, err := strictjson.RequiredString(m, "head_sha")
	if err != nil || !isHexSHA40(headSHA) {
		return workflowRun{}, false
	}
	status, err := strictjson.RequiredString(m, "status")
	if err != nil {
		return workflowRun{}, false
	}
	conclusion, ok := requiredNullableNonEmptyString(m, "conclusion")
	if !ok {
		return workflowRun{}, false
	}
	createdAt, ok := requiredRFC3339UTC(m, "created_at")
	if !ok {
		return workflowRun{}, false
	}
	runStartedAt, ok := requiredRFC3339UTC(m, "run_started_at")
	if !ok {
		return workflowRun{}, false
	}

	repoFullName, ok := requiredRepoFullName(m, "repository")
	if !ok {
		return workflowRun{}, false
	}
	headRepoFullName, headRepoNull, ok := requiredNullableRepoFullName(m, "head_repository")
	if !ok {
		return workflowRun{}, false
	}
	prNumbers, ok := requiredPullRequestNumbers(m, "pull_requests")
	if !ok {
		return workflowRun{}, false
	}

	return workflowRun{
		id:               id,
		path:             path,
		event:            event,
		headSHA:          headSHA,
		status:           status,
		conclusion:       conclusion,
		createdAt:        createdAt,
		runStartedAt:     runStartedAt,
		repoFullName:     repoFullName,
		headRepoNull:     headRepoNull,
		headRepoFullName: headRepoFullName,
		pullRequests:     prNumbers,
	}, true
}

func requiredRFC3339UTC(m map[string]strictjson.Value, key string) (time.Time, bool) {
	raw, err := strictjson.RequiredString(m, key)
	if err != nil {
		return time.Time{}, false
	}
	return parseRFC3339UTC(raw)
}

func requiredRepoFullName(m map[string]strictjson.Value, key string) (string, bool) {
	v, err := strictjson.Required(m, key)
	if err != nil {
		return "", false
	}
	return parseRepoFullName(v)
}

func requiredNullableRepoFullName(m map[string]strictjson.Value, key string) (fullName string, isNull bool, ok bool) {
	v, err := strictjson.Required(m, key)
	if err != nil {
		return "", false, false
	}
	if v.IsNull() {
		return "", true, true
	}
	fullName, ok = parseRepoFullName(v)
	return fullName, false, ok
}

func requiredPullRequestNumbers(m map[string]strictjson.Value, key string) ([]int64, bool) {
	v, err := strictjson.Required(m, key)
	if err != nil {
		return nil, false
	}
	rawPRs, err := v.AsArray()
	if err != nil {
		return nil, false
	}
	prNumbers := make([]int64, 0, len(rawPRs))
	for _, prElem := range rawPRs {
		prObj, err := prElem.AsObject()
		if err != nil {
			return nil, false
		}
		prMembers, err := prObj.CollectExactCase("number")
		if err != nil {
			return nil, false
		}
		num, ok := requiredPositiveInt64(prMembers, "number")
		if !ok {
			return nil, false
		}
		prNumbers = append(prNumbers, num)
	}
	return prNumbers, true
}

func parseRepoFullName(val strictjson.Value) (string, bool) {
	obj, err := val.AsObject()
	if err != nil {
		return "", false
	}
	m, err := obj.CollectExactCase("full_name")
	if err != nil {
		return "", false
	}
	fullName, err := strictjson.RequiredStringAllowEmpty(m, "full_name")
	if err != nil {
		return "", false
	}
	return fullName, true
}

// verifyParentJobs paginates the latest-attempt jobs of runID and confirms that
// every job matches parentSHA and each of the four required jobs appears exactly
// once, is completed, and has an acceptable conclusion.
func (e *Evaluator) verifyParentJobs(
	ctx context.Context,
	repo string,
	runID int64,
	parentSHA string,
) (int64, Reason, int, bool) {
	seen := make(map[string]workflowJob, len(requiredJobNames))
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf(
			"repos/%s/actions/runs/%d/jobs?filter=latest&per_page=100&page=%d",
			repo, runID, page,
		)
		body, hasNext, reason, status, ok := e.ghAPIGet(ctx, endpoint)
		if !ok {
			return 0, reason, status, false
		}
		jobs, parsed := parseJobsPage(body)
		if !parsed {
			return 0, ReasonAPIInvalidResponse, 0, false
		}
		for _, j := range jobs {
			if j.headSHA != parentSHA {
				return 0, ReasonParentJobsInvalid, 0, false
			}
			if !slices.Contains(requiredJobNames, j.name) {
				continue
			}
			if _, dup := seen[j.name]; dup {
				return 0, ReasonParentJobsInvalid, 0, false
			}
			seen[j.name] = j
		}
		if !hasNext {
			break
		}
		if page >= e.limits.maxJobPages {
			return 0, ReasonAPIPaginationLimit, 0, false
		}
	}

	if len(seen) != len(requiredJobNames) {
		return 0, ReasonParentJobsInvalid, 0, false
	}
	for _, name := range requiredJobNames {
		j := seen[name]
		if j.status != statusCompleted {
			return 0, ReasonParentJobsInvalid, 0, false
		}
		if name == checkChangesJobName {
			if j.conclusion != conclusionSuccess {
				return 0, ReasonParentJobsInvalid, 0, false
			}
			continue
		}
		if j.conclusion != conclusionSuccess && j.conclusion != conclusionSkipped {
			return 0, ReasonParentJobsInvalid, 0, false
		}
	}
	return seen[checkChangesJobName].id, ReasonUnspecified, 0, true
}

func parseJobsPage(body []byte) ([]workflowJob, bool) {
	obj, err := strictjson.ParseObject(body)
	if err != nil {
		return nil, false
	}
	top, err := obj.CollectExactCase("jobs")
	if err != nil {
		return nil, false
	}
	jobsVal, err := strictjson.Required(top, "jobs")
	if err != nil {
		return nil, false
	}
	rawJobs, err := jobsVal.AsArray()
	if err != nil {
		return nil, false
	}

	jobs := make([]workflowJob, 0, len(rawJobs))
	for _, elem := range rawJobs {
		job, ok := parseWorkflowJob(elem)
		if !ok {
			return nil, false
		}
		jobs = append(jobs, job)
	}
	return jobs, true
}

func parseWorkflowJob(val strictjson.Value) (workflowJob, bool) {
	obj, err := val.AsObject()
	if err != nil {
		return workflowJob{}, false
	}
	m, err := obj.CollectExactCase("id", "name", "head_sha", "status", "conclusion")
	if err != nil {
		return workflowJob{}, false
	}

	id, ok := requiredPositiveInt64(m, "id")
	if !ok {
		return workflowJob{}, false
	}
	name, err := strictjson.RequiredString(m, "name")
	if err != nil {
		return workflowJob{}, false
	}
	headSHA, err := strictjson.RequiredString(m, "head_sha")
	if err != nil || !isHexSHA40(headSHA) {
		return workflowJob{}, false
	}
	status, err := strictjson.RequiredString(m, "status")
	if err != nil {
		return workflowJob{}, false
	}
	conclusion, ok := requiredNullableNonEmptyString(m, "conclusion")
	if !ok {
		return workflowJob{}, false
	}
	return workflowJob{
		id:         id,
		name:       name,
		headSHA:    headSHA,
		status:     status,
		conclusion: conclusion,
	}, true
}

// verifyParentBaseAnnotation paginates the annotations of checkChangesJobID and
// verifies that exactly one notice titled "ci-base-sha" exists and equals mergeBaseSHA.
func (e *Evaluator) verifyParentBaseAnnotation(
	ctx context.Context,
	repo string,
	checkChangesJobID int64,
	mergeBaseSHA string,
) (Reason, int, bool) {
	var matched []string
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf(
			"repos/%s/check-runs/%d/annotations?per_page=100&page=%d",
			repo, checkChangesJobID, page,
		)
		body, hasNext, reason, status, ok := e.ghAPIGet(ctx, endpoint)
		if !ok {
			return reason, status, false
		}
		annotations, parsed := parseAnnotationsPage(body)
		if !parsed {
			return ReasonAPIInvalidResponse, 0, false
		}
		for _, a := range annotations {
			if a.level == annotationLevelNotice && a.titlePresent && a.title == baseSHAAnnotationTitle {
				matched = append(matched, a.message)
			}
		}
		if !hasNext {
			break
		}
		if page >= e.limits.maxAnnotationPages {
			return ReasonAPIPaginationLimit, 0, false
		}
	}

	switch len(matched) {
	case 0:
		return ReasonParentBaseMissing, 0, false
	case 1:
	default:
		return ReasonParentBaseDuplicate, 0, false
	}
	if !isHexSHA40(matched[0]) {
		return ReasonAPIInvalidResponse, 0, false
	}
	if matched[0] != mergeBaseSHA {
		return ReasonMainAdvanced, 0, false
	}
	return ReasonUnspecified, 0, true
}

func parseAnnotationsPage(body []byte) ([]checkAnnotation, bool) {
	rawItems, err := strictjson.ParseArray(body)
	if err != nil {
		return nil, false
	}
	annotations := make([]checkAnnotation, 0, len(rawItems))
	for _, elem := range rawItems {
		ann, ok := parseCheckAnnotation(elem)
		if !ok {
			return nil, false
		}
		annotations = append(annotations, ann)
	}
	return annotations, true
}

func parseCheckAnnotation(val strictjson.Value) (checkAnnotation, bool) {
	obj, err := val.AsObject()
	if err != nil {
		return checkAnnotation{}, false
	}
	m, err := obj.CollectExactCase("annotation_level", "title", "message")
	if err != nil {
		return checkAnnotation{}, false
	}
	level, err := strictjson.RequiredStringAllowEmpty(m, "annotation_level")
	if err != nil {
		return checkAnnotation{}, false
	}
	titleVal, err := strictjson.Required(m, "title")
	if err != nil {
		return checkAnnotation{}, false
	}
	titlePresent := !titleVal.IsNull()
	title := ""
	if titlePresent {
		title, err = titleVal.AsString()
		if err != nil {
			return checkAnnotation{}, false
		}
	}
	message, err := strictjson.RequiredStringAllowEmpty(m, "message")
	if err != nil {
		return checkAnnotation{}, false
	}
	return checkAnnotation{
		level:        level,
		titlePresent: titlePresent,
		title:        title,
		message:      message,
	}, true
}

func requiredPositiveInt64(m map[string]strictjson.Value, key string) (int64, bool) {
	v, err := strictjson.Required(m, key)
	if err != nil {
		return 0, false
	}
	n, err := v.AsNonNegativeInt64()
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func requiredNullableNonEmptyString(m map[string]strictjson.Value, key string) (string, bool) {
	v, err := strictjson.Required(m, key)
	if err != nil {
		return "", false
	}
	if v.IsNull() {
		return "", true
	}
	s, err := v.AsString()
	if err != nil || s == "" {
		return "", false
	}
	return s, true
}

// parseRFC3339UTC parses a canonical UTC RFC3339 timestamp (YYYY-MM-DDTHH:MM:SSZ)
// and rejects non-UTC offsets, fractional seconds, and zero timestamps.
func parseRFC3339UTC(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil || t.IsZero() || t.UTC().Format(time.RFC3339) != s {
		return time.Time{}, false
	}
	return t, true
}
