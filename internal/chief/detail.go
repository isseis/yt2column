package chief

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/fixprguard"
	"github.com/isseis/yt2column/internal/privatefile"
	"github.com/isseis/yt2column/internal/strictjson"
)

// errGHOutput reports gh output of a shape chief does not read.
var errGHOutput = errors.New("unreadable gh output")

// errNoHead reports a detail that is not kept: without the PR's head commit
// it has no name.
var errNoHead = errors.New("the detail is not kept without the PR's head commit")

// The fields chief asks gh for. The review bodies come with reviews, but
// only each review's author and state are read; no body reaches the output.
const (
	prViewFields   = "state,number,title,url,headRefName,headRefOid,additions,deletions,changedFiles,commits,files,body,reviews"
	prChecksFields = "name,state,bucket,workflow"
)

// prChecksExitCodes are the exit codes gh pr checks may end with although
// its output is a result: 8 while a job is pending, 1 when a job failed or
// there is none. The sampled gh exited 0 in every case; a non-zero exit is a
// result only when its standard output reads strictly.
var prChecksExitCodes = []int{1, 8}

// commitID is the form of a head commit.
var commitID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Review is one PR review's author and state; its body is never read.
type Review struct {
	Author string
	State  string
}

// Check is one CI job's result as gh reports it, never summarized.
type Check struct {
	Name     string
	Workflow string
	State    string
	Bucket   string
}

// prView is what `gh pr view` says about a PR.
type prView struct {
	Number       int
	State        string
	Title        string
	URL          string
	HeadRef      string
	HeadOID      string
	Additions    int
	Deletions    int
	ChangedFiles int
	Commits      int
	Files        []string
	Body         string
	Reviews      []Review
}

// parsePRView reads `gh pr view --json prViewFields`. Contract: every field
// is read in full or the whole output is rejected; of a review only the
// author's login and the state are read.
func parsePRView(out []byte) (prView, error) {
	object, err := strictjson.ParseObject(out)
	if err != nil {
		return prView{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	m, err := object.Collect("state", "number", "title", "url", "headRefName", "headRefOid", "additions", "deletions", "changedFiles", "commits", "files", "body", "reviews")
	if err != nil {
		return prView{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	var v prView
	var errs []error
	str := func(key string, dst *string, allowEmpty bool) {
		var s string
		var err error
		if allowEmpty {
			s, err = strictjson.RequiredStringAllowEmpty(m, key)
		} else {
			s, err = strictjson.RequiredString(m, key)
		}
		errs = append(errs, err)
		*dst = s
	}
	num := func(key string, dst *int) {
		value, err := strictjson.Required(m, key)
		if err == nil {
			var n int64
			n, err = value.AsNonNegativeInt64()
			if err == nil && n > 1<<31-1 {
				err = fmt.Errorf("%w: %s out of range", errGHOutput, key)
			}
			*dst = int(n)
		}
		errs = append(errs, err)
	}
	str("state", &v.State, false)
	str("title", &v.Title, true)
	str("url", &v.URL, false)
	str("headRefName", &v.HeadRef, false)
	str("headRefOid", &v.HeadOID, false)
	str("body", &v.Body, true)
	for key, dst := range map[string]*int{"number": &v.Number, "additions": &v.Additions, "deletions": &v.Deletions, "changedFiles": &v.ChangedFiles} {
		num(key, dst)
	}
	if err := errors.Join(errs...); err != nil {
		return prView{}, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	if v.Number == 0 || !commitID.MatchString(v.HeadOID) {
		return prView{}, fmt.Errorf("%w: number or head commit", errGHOutput)
	}
	commits, err := requiredGHArray(m, "commits")
	if err != nil {
		return prView{}, err
	}
	v.Commits = len(commits)
	files, err := requiredGHArray(m, "files")
	if err != nil {
		return prView{}, err
	}
	for i, f := range files {
		path, err := objectString(f, "path")
		if err != nil {
			return prView{}, fmt.Errorf("%w: file %d: %w", errGHOutput, i, err)
		}
		v.Files = append(v.Files, path)
	}
	reviews, err := requiredGHArray(m, "reviews")
	if err != nil {
		return prView{}, err
	}
	for i, r := range reviews {
		review, err := parseReview(r)
		if err != nil {
			return prView{}, fmt.Errorf("%w: review %d: %w", errGHOutput, i, err)
		}
		v.Reviews = append(v.Reviews, review)
	}
	return v, nil
}

// parseReview reads a review's author login and state, and nothing else.
func parseReview(v strictjson.Value) (Review, error) {
	object, err := v.AsObject()
	if err != nil {
		return Review{}, err
	}
	m, err := object.Collect("author", "state")
	if err != nil {
		return Review{}, err
	}
	state, err := strictjson.RequiredString(m, "state")
	if err != nil {
		return Review{}, err
	}
	authorValue, err := strictjson.Required(m, "author")
	if err != nil {
		return Review{}, err
	}
	login, err := objectString(authorValue, "login")
	if err != nil {
		return Review{}, err
	}
	return Review{Author: login, State: state}, nil
}

func requiredGHArray(m map[string]strictjson.Value, key string) ([]strictjson.Value, error) {
	value, err := strictjson.Required(m, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	items, err := value.AsArray()
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errGHOutput, key, err)
	}
	return items, nil
}

// objectString returns the non-empty string member key of an object value.
func objectString(v strictjson.Value, key string) (string, error) {
	object, err := v.AsObject()
	if err != nil {
		return "", err
	}
	m, err := object.Collect(key)
	if err != nil {
		return "", err
	}
	return strictjson.RequiredString(m, key)
}

// parsePRChecks reads `gh pr checks --json prChecksFields`: every job's
// name, workflow, state, and bucket, as gh gives them.
func parsePRChecks(out []byte) ([]Check, error) {
	items, err := strictjson.ParseArray(out)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGHOutput, err)
	}
	checks := make([]Check, 0, len(items))
	for i, item := range items {
		object, err := item.AsObject()
		if err != nil {
			return nil, fmt.Errorf("%w: check %d: %w", errGHOutput, i, err)
		}
		m, err := object.Collect("name", "state", "bucket", "workflow")
		if err != nil {
			return nil, fmt.Errorf("%w: check %d: %w", errGHOutput, i, err)
		}
		var c Check
		var errs []error
		for key, dst := range map[string]*string{"name": &c.Name, "state": &c.State, "bucket": &c.Bucket} {
			s, err := strictjson.RequiredString(m, key)
			errs = append(errs, err)
			*dst = s
		}
		c.Workflow, err = strictjson.RequiredStringAllowEmpty(m, "workflow")
		if err := errors.Join(append(errs, err)...); err != nil {
			return nil, fmt.Errorf("%w: check %d: %w", errGHOutput, i, err)
		}
		checks = append(checks, c)
	}
	return checks, nil
}

// ReportSource is where the summary of the command that created a PR comes
// from. The zero value is ReportNone: neither was read.
type ReportSource int

const (
	// ReportNone is no report and no description.
	ReportNone ReportSource = iota
	// ReportFromScreen is the command's report on a pane's screen.
	ReportFromScreen
	// ReportFromDescription is the PR description, used when no pane shows
	// the report.
	ReportFromDescription
)

// String names the source for the output and the records.
func (r ReportSource) String() string {
	switch r {
	case ReportFromScreen:
		return "report"
	case ReportFromDescription:
		return "description"
	default:
		return "none"
	}
}

// parseReportSource reads a source name a record holds.
func parseReportSource(s string) (ReportSource, error) {
	for _, r := range []ReportSource{ReportNone, ReportFromScreen, ReportFromDescription} {
		if r.String() == s {
			return r, nil
		}
	}
	return ReportNone, fmt.Errorf("%w: report source %q", errRecord, s)
}

// detailRecord is a PR detail kept for one PR and head commit: the report or
// the description it was read from, and, for a report, the pane and its
// state then, which narrow a later reset.
type detailRecord struct {
	PR        int       `json:"pr"`
	Head      string    `json:"head"`
	Saved     time.Time `json:"saved"`
	Source    string    `json:"source"`
	Text      string    `json:"text"`
	Truncated bool      `json:"truncated"`
	Pane      string    `json:"pane"`
	Seq       int64     `json:"seq"`
}

func detailName(pr int, head string) string {
	return detailPrefix + strconv.Itoa(pr) + "-" + head + recordSuffix
}

// readDetail returns the detail kept for pr at head; ok is false when none is
// kept.
func (r WorkspaceRecords) readDetail(pr int, head string) (detailRecord, bool, error) {
	var rec detailRecord
	err := r.store().read(detailName(pr, head), &rec)
	switch {
	case errors.Is(err, privatefile.ErrNotExist):
		return detailRecord{}, false, nil
	case err != nil:
		return detailRecord{}, false, err
	case rec.PR != pr || rec.Head != head:
		return detailRecord{}, false, fmt.Errorf("%w: detail of PR %d at %s names another", errRecord, pr, head)
	}
	if _, err := parseReportSource(rec.Source); err != nil {
		return detailRecord{}, false, err
	}
	return rec, true, nil
}

// PRDetail is what `chief detail` shows about a workspace's open PR. An item
// that could not be read has its error set and the rest are still given.
type PRDetail struct {
	Workspace string
	Label     string
	Task      string
	PR        OpenPR
	View      *prView
	ViewErr   string
	// Checks are the CI jobs; empty with no ChecksErr is a PR without jobs.
	Checks    []Check
	ChecksErr string
	Report    ReportSource
	// ReportText is the report or the description, capped.
	ReportText      string
	ReportTruncated bool
	// ReviewSkippable is true when the PR changes only the task's plan and
	// implementation handoff.
	ReviewSkippable bool
	// Runpr is /runpr with its route, for the choice to send it.
	Runpr *Next
	// RecordErr is why the detail could not be kept, empty when it was.
	RecordErr string
	record    *detailRecord
}

// CollectDetail gathers the detail of the workspace's open PR pr and keeps
// it. Contract: it runs only reads (gh pr view, gh pr checks, and the panes'
// screens); a failed item is reported as unavailable and the others are
// still returned; a detail already kept for the PR's head commit supplies
// the report, so the report survives a pane that was emptied since; a new
// detail is written once and never replaced.
func CollectDetail(ctx context.Context, deps Deps, ws TaskWorkspace, pr OpenPR, docsDir string, routing Routing, routingErr error) PRDetail {
	herdr := NewHerdr(deps.Cmds)
	d := PRDetail{Workspace: ws.Worktree, Label: ws.Label, PR: pr}
	if task, ok := taskOfDir(docsDir); ok {
		d.Task = task
	}
	number := strconv.Itoa(pr.Number)
	if prn, err := fixprguard.ParsePRNumber(number); err == nil {
		if cmd, err := paneinput.NewRunpr(prn); err == nil {
			d.Runpr = makeNext(TaskNextRunpr, cmd, paneinput.RouteRunpr, routing, routingErr)
		}
	}
	out, err := deps.Cmds.gh(ctx, ws.Worktree, "pr", "view", number, "--json", prViewFields)
	if err == nil {
		var v prView
		if v, err = parsePRView(out); err == nil && (v.Number != pr.Number || v.State != "OPEN" || v.URL != pr.URL) {
			err = fmt.Errorf("%w: gh pr view gave another or a closed PR", errGHOutput)
		}
		if err == nil {
			d.View = &v
		}
	}
	if err != nil {
		d.ViewErr = err.Error()
	}
	d.Checks, err = readChecks(ctx, deps.Cmds, ws.Worktree, number)
	if err != nil {
		d.ChecksErr = err.Error()
	}
	if d.View != nil {
		d.ReviewSkippable = reviewSkippable(*d.View, docsDir)
	}
	rec, err := keptReport(ctx, deps, herdr, ws, pr, d.View)
	if err != nil {
		d.RecordErr = err.Error()
	}
	src, _ := parseReportSource(rec.Source)
	d.Report, d.ReportText, d.ReportTruncated = src, rec.Text, rec.Truncated
	if d.View != nil && err == nil {
		d.record = &rec
	}
	return d
}

// readChecks reads the PR's CI jobs. A non-zero exit of the codes gh uses for
// pending and failed jobs is a result when its standard output reads.
func readChecks(ctx context.Context, cmds Commands, worktree, number string) ([]Check, error) {
	out, err := cmds.gh(ctx, worktree, "pr", "checks", number, "--json", prChecksFields)
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && slices.Contains(prChecksExitCodes, exitErr.Code) {
		if checks, perr := parsePRChecks(exitErr.Stdout); perr == nil {
			return checks, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return parsePRChecks(out)
}

// keptReport returns the report of the PR: the one kept for its head commit
// when there is one, else the one read now from the panes (or the
// description), which is then kept. Contract: a kept detail is never
// replaced, so the report read before a pane was emptied wins over a later
// reading of the emptied pane.
func keptReport(ctx context.Context, deps Deps, herdr Herdr, ws TaskWorkspace, pr OpenPR, view *prView) (detailRecord, error) {
	rec := readReport(ctx, herdr, ws, pr, view, deps.Now())
	if view == nil {
		return rec, errNoHead
	}
	records, err := deps.Store.Records(ws.Tree)
	if err != nil {
		return rec, err
	}
	rec.Head = view.HeadOID
	err = records.store().create(detailName(pr.Number, view.HeadOID), rec)
	if !errors.Is(err, privatefile.ErrExist) {
		return rec, err
	}
	kept, ok, err := records.readDetail(pr.Number, view.HeadOID)
	if err != nil || !ok {
		return rec, errors.Join(errRecord, err)
	}
	return kept, nil
}

// readReport reads the report of the command that created pr from the
// workspace's panes. Exactly one pane must show it; with none, or with
// several, the description stands in, and the source says so.
func readReport(ctx context.Context, herdr Herdr, ws TaskWorkspace, pr OpenPR, view *prView, now time.Time) detailRecord {
	rec := detailRecord{PR: pr.Number, Saved: now.UTC(), Source: ReportNone.String()}
	var hits []detailRecord
	for _, a := range ws.Agents {
		raw, err := herdr.Screen(ctx, a.Pane, reportSource(a))
		if err != nil {
			continue
		}
		if text, ok := readCommandReport(a.Kind, raw, pr.URL); ok {
			capped := CapScreen(text)
			hits = append(hits, detailRecord{Text: capped.Text, Truncated: capped.Truncated, Pane: a.Pane.String(), Seq: a.StateChangeSeq})
		}
	}
	switch {
	case len(hits) == 1:
		rec.Source, rec.Text, rec.Truncated, rec.Pane, rec.Seq = ReportFromScreen.String(), hits[0].Text, hits[0].Truncated, hits[0].Pane, hits[0].Seq
	case view != nil:
		capped := CapDescription(view.Body)
		rec.Source, rec.Text, rec.Truncated = ReportFromDescription.String(), capped.Text, capped.Truncated
	}
	return rec
}

// reportSource is where a pane's report is read from: Herdr serves only the
// visible rows of a blocked agent, and the recent scrollback otherwise.
func reportSource(a Agent) ScreenSource {
	if a.Status == AgentBlocked {
		return ScreenVisible
	}
	return ScreenRecent
}

// reviewSkippable reports whether the PR changes only the task's plan and
// implementation handoff: the file list is complete (as many files as gh
// counts) and names no other file.
func reviewSkippable(v prView, docsDir string) bool {
	if docsDir == "" || v.ChangedFiles == 0 || len(v.Files) != v.ChangedFiles {
		return false
	}
	allowed := map[string]struct{}{docsDir + "/" + planFile: {}, docsDir + "/" + implementationHandoffFile: {}}
	for _, f := range v.Files {
		if _, ok := allowed[f]; !ok {
			return false
		}
	}
	return true
}

// implementationHandoffFile is the task document of review concerns for the
// plan.
const implementationHandoffFile = "implementation_handoff.md"

// taskOfDir returns the task number of a task directory such as
// docs/tasks/0088_chief_of_staff.
func taskOfDir(dir string) (string, bool) {
	const prefix = taskRoot + "/"
	if len(dir) < len(prefix)+4 || dir[:len(prefix)] != prefix {
		return "", false
	}
	task, err := paneinput.ParseTaskID(dir[len(prefix) : len(prefix)+4])
	if err != nil {
		return "", false
	}
	return task.String(), true
}
