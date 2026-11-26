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

// ghCommitsCap is how many commits gh 2.96.0 returns for --json commits (its
// query asks for the first 100), so a PR with more shows this many.
const ghCommitsCap = 100

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
	// CommitsCapped is true when gh returned ghCommitsCap commits: the PR
	// has that many or more.
	CommitsCapped bool
	Files         []string
	Body          string
	Reviews       []Review
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
	v.CommitsCapped = v.Commits >= ghCommitsCap
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
	// ReportFromPane is the command's report read from a pane: from its
	// session's transcript now, from its screen in a record kept before.
	ReportFromPane
	// ReportFromDescription is the PR description, used when no single pane
	// has the report.
	ReportFromDescription
)

// String names the source for the output and the records.
func (r ReportSource) String() string {
	switch r {
	case ReportFromPane:
		return "report"
	case ReportFromDescription:
		return "description"
	default:
		return nameNone
	}
}

// parseReportSource reads a source name a record holds.
func parseReportSource(s string) (ReportSource, error) {
	for _, r := range []ReportSource{ReportNone, ReportFromPane, ReportFromDescription} {
		if r.String() == s {
			return r, nil
		}
	}
	return ReportNone, fmt.Errorf("%w: report source %q", errRecord, s)
}

// DescriptionReason is why a PR detail read from the panes shows no pane's
// report. The zero value is DescriptionReasonNone, nothing to show: a pane's
// report is shown, or the panes were not read.
type DescriptionReason int

const (
	// DescriptionReasonNone is no reason to show.
	DescriptionReasonNone DescriptionReason = iota
	// DescriptionReasonNoClaudePane is a workspace with no Claude Code pane.
	DescriptionReasonNoClaudePane
	// DescriptionReasonNoCandidate is a workspace none of whose Claude Code
	// panes has a last report and a PR record naming the PR.
	DescriptionReasonNoCandidate
	// DescriptionReasonSeveralCandidates is a workspace with two or more such
	// panes.
	DescriptionReasonSeveralCandidates
)

// String names the reason for the output.
func (r DescriptionReason) String() string {
	switch r {
	case DescriptionReasonNoClaudePane:
		return "no_claude_pane"
	case DescriptionReasonNoCandidate:
		return "no_candidate"
	case DescriptionReasonSeveralCandidates:
		return "several_candidates"
	default:
		return unknownName
	}
}

// PaneReportKind is what a PR detail found on one Claude Code pane. The zero
// value is PaneReportUnknown, which no reading gives.
type PaneReportKind int

const (
	// PaneReportUnknown is no finding.
	PaneReportUnknown PaneReportKind = iota
	// PaneReportUnreadable is a pane whose transcript cannot be read.
	PaneReportUnreadable
	// PaneReportAbsent is a pane with no last report.
	PaneReportAbsent
	// PaneReportNotNamed is a pane whose last report's session has no PR
	// record naming the PR.
	PaneReportNotNamed
	// PaneReportCandidate is a pane whose last report's session has a PR
	// record naming the PR.
	PaneReportCandidate
)

// String names a finding for the output.
func (k PaneReportKind) String() string {
	switch k {
	case PaneReportUnreadable:
		return nameUnreadable
	case PaneReportAbsent:
		return noReportName
	case PaneReportNotNamed:
		return "not_named"
	case PaneReportCandidate:
		return "candidate"
	default:
		return unknownName
	}
}

// PaneReport is what a PR detail found on one Claude Code pane: Detail says
// why a pane is unreadable or has no report, with its transcript's path.
type PaneReport struct {
	Pane   string
	Kind   PaneReportKind
	Detail string
}

// ReportRecording says whether gathering a PR detail may keep a report
// record. The zero value is ReportRecordNone: only the human's look at the
// detail (chief detail) may keep one, so that the reset's check of an
// unchanged pane starts when the human saw the report, never just before
// /runpr is sent.
type ReportRecording int

const (
	// ReportRecordNone keeps no report record.
	ReportRecordNone ReportRecording = iota
	// ReportRecordAllowed keeps a report record when the panes are read
	// again under a kept description and exactly one has the report.
	ReportRecordAllowed
)

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
	// Skippable is the PR's review-skippable judgment at Head, kept so a
	// snapshot can show it on the PR's item without reading GitHub.
	Skippable bool `json:"review_skippable"`
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
	// DescriptionReason is why no pane's report is shown, with what was
	// found on each Claude Code pane in ReportPanes; both are unset when a
	// pane's report is shown, and when a kept detail is used without the
	// panes being read again.
	DescriptionReason DescriptionReason
	ReportPanes       []PaneReport
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
// it. Contract: it runs only reads (gh pr view, gh pr checks, and the Claude
// Code panes' transcripts, never a screen); a failed item is reported as
// unavailable and the others are still returned; a record already kept for
// the PR's head commit supplies the report, so the report survives a pane
// that was emptied since; a record is written once and never replaced, and
// a report record is written only when recording is ReportRecordAllowed.
func CollectDetail(ctx context.Context, deps Deps, ws TaskWorkspace, pr OpenPR, docsDir string, routing Routing, routingErr error, recording ReportRecording) PRDetail {
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
		if v, err = parsePRView(out); err == nil && (v.Number != pr.Number || v.State != ghOpen || v.URL != pr.URL) {
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
	rec, reading, err := keptReport(deps, ws, pr, d.View, d.ReviewSkippable, recording)
	if err != nil {
		d.RecordErr = err.Error()
	}
	src, _ := parseReportSource(rec.Source)
	d.Report, d.ReportText, d.ReportTruncated = src, rec.Text, rec.Truncated
	if src != ReportFromPane {
		d.DescriptionReason, d.ReportPanes = reading.reason, reading.panes
	}
	if d.View != nil && err == nil {
		d.record = &rec
	}
	if recording == ReportRecordAllowed && d.View != nil && src != ReportFromPane {
		d.inheritReport(ctx, deps, ws, pr)
	}
	return d
}

// inheritReport shows the pane report of an earlier head in place of the
// description when inheritedPaneReport finds one. Contract: display only;
// d.record keeps the record of this head, so nothing that reads the kept
// record (the reset, the review-skippable mark) sees the inherited report.
func (d *PRDetail) inheritReport(ctx context.Context, deps Deps, ws TaskWorkspace, pr OpenPR) {
	if inherited, ok := inheritedPaneReport(ctx, deps, ws, pr, d.View); ok {
		d.Report, d.ReportText, d.ReportTruncated = ReportFromPane, inherited.Text, inherited.Truncated
		d.DescriptionReason, d.ReportPanes = DescriptionReasonNone, nil
	}
}

// maxDetailMergeHops is how many first parents inheritedPaneReport follows
// from the PR's head before it gives up.
const maxDetailMergeHops = 10

// inheritedPaneReport finds the report record of an earlier head of the PR
// that a /mergemain merge commit (or a chain of them) took the head from, for
// the display of `chief detail`. Contract: it follows the first parent of the
// head while the commit has exactly two parents and the second parent is in
// origin/main's history (the other side of the merge is already merged), at
// most maxDetailMergeHops times, and returns the first pane report record it
// finds; it never writes a record, and every failure (a commit that cannot be
// read, a fetch that fails, a parent that is not such a merge, no record) is
// no inheritance. A head the worktree lacks is fetched first with
// FetchBranch, which writes only refs/remotes/origin/<branch>, so origin/main
// is the value the round read.
func inheritedPaneReport(ctx context.Context, deps Deps, ws TaskWorkspace, pr OpenPR, view *prView) (detailRecord, bool) {
	git := NewGitReader(deps.Cmds)
	head := view.HeadOID
	if !commitID.MatchString(head) {
		return detailRecord{}, false
	}
	if has, err := git.HasCommit(ctx, ws.Worktree, head); err != nil {
		return detailRecord{}, false
	} else if !has {
		if err := git.FetchBranch(ctx, ws.Worktree, view.HeadRef); err != nil {
			return detailRecord{}, false
		}
		if has, err := git.HasCommit(ctx, ws.Worktree, head); err != nil || !has {
			return detailRecord{}, false
		}
	}
	main, err := git.OriginMain(ctx, ws.Worktree)
	if err != nil {
		return detailRecord{}, false
	}
	records, err := deps.Store.Records(ws.Tree)
	if err != nil {
		return detailRecord{}, false
	}
	cur := head
	for range maxDetailMergeHops {
		parents, err := git.Parents(ctx, ws.Worktree, cur)
		if err != nil || len(parents) != 2 {
			return detailRecord{}, false
		}
		if merged, err := git.IsAncestor(ctx, ws.Worktree, parents[1], main); err != nil || !merged {
			return detailRecord{}, false
		}
		cur = parents[0]
		kept, ok, err := records.keptPaneReport(pr.Number, cur)
		if err != nil {
			return detailRecord{}, false
		}
		if ok && kept.Source == ReportFromPane.String() {
			return kept, true
		}
	}
	return detailRecord{}, false
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

// reportReading is why a reading of the panes found no single pane's
// report, and what it found on each Claude Code pane; it is empty when a
// pane's report was found, or when the panes were not read.
type reportReading struct {
	reason DescriptionReason
	panes  []PaneReport
}

// keptReport returns the report of the PR and why no pane's report stands,
// keeping what it read. Contract: a kept record is never replaced, and of
// the two kept for the head commit the report record goes before the detail
// record (keptPaneReport), so a report read before a pane was emptied wins
// over a later reading of the emptied pane. With no record kept, the panes
// are read and the detail record is kept. With only a record whose source
// is not a pane's report, the panes are read again when recording is
// ReportRecordAllowed, and a report record is kept when exactly one pane has
// the report; otherwise that record stands. skippable is the
// review-skippable judgment of view, kept with the report.
func keptReport(deps Deps, ws TaskWorkspace, pr OpenPR, view *prView, skippable bool, recording ReportRecording) (detailRecord, reportReading, error) {
	read := func() (detailRecord, reportReading) {
		rec, reading := readReport(deps.Env, ws, pr, view, deps.Now())
		rec.Skippable = skippable
		return rec, reading
	}
	if view == nil {
		rec, reading := read()
		return rec, reading, errNoHead
	}
	records, err := deps.Store.Records(ws.Tree)
	if err != nil {
		rec, reading := read()
		return rec, reading, err
	}
	head := view.HeadOID
	kept, ok, err := records.keptPaneReport(pr.Number, head)
	switch {
	case err != nil:
		rec, reading := read()
		return rec, reading, err
	case ok && (kept.Source == ReportFromPane.String() || recording != ReportRecordAllowed):
		return kept, reportReading{}, nil
	case ok:
		rec, reading := read()
		if rec.Source != ReportFromPane.String() {
			return kept, reading, nil
		}
		rec.Head = head
		return keepRecord(records, reportName(pr.Number, head), rec, reading)
	}
	rec, reading := read()
	rec.Head = head
	return keepRecord(records, detailName(pr.Number, head), rec, reading)
}

// keepPaneReport keeps a report record (report-<PR>-<head>.json) from the
// workspace's Claude Code panes before /mergemain clears the pane. Contract:
// it never calls gh pr view and never writes a PR description record; it
// writes a report record only when exactly one Claude Code pane's last report
// names pr by number and URL, leaves any existing pane report untouched, and
// returns nil without creating a file when no single pane report can be read.
func keepPaneReport(deps Deps, ws TaskWorkspace, pr OpenPR) error {
	if pr.Number <= 0 || pr.Head.IsZero() {
		return nil
	}
	rec, _ := readReport(deps.Env, ws, pr, nil, deps.Now())
	if rec.Source != ReportFromPane.String() {
		return nil
	}
	records, err := deps.Store.Records(ws.Tree)
	if err != nil {
		return err
	}
	head := pr.Head.String()
	if kept, ok, err := records.keptPaneReport(pr.Number, head); err != nil {
		return err
	} else if ok && kept.Source == ReportFromPane.String() {
		return nil
	}
	rec.Head = head
	err = records.store().create(reportName(pr.Number, head), rec)
	if errors.Is(err, privatefile.ErrExist) {
		return nil
	}
	return err
}

// keepRecord writes rec under name, which must not exist. When another
// process wrote it first, the record kept for the head commit is returned
// instead, so a kept record is never replaced.
func keepRecord(records WorkspaceRecords, name string, rec detailRecord, reading reportReading) (detailRecord, reportReading, error) {
	err := records.store().create(name, rec)
	if !errors.Is(err, privatefile.ErrExist) {
		return rec, reading, err
	}
	kept, ok, err := records.keptPaneReport(rec.PR, rec.Head)
	if err != nil || !ok {
		return rec, reading, errors.Join(errRecord, err)
	}
	return kept, reportReading{}, nil
}

// readReport reads the report of the command that created pr from the
// transcripts of the workspace's Claude Code panes. Contract: a pane is a
// candidate when its session has a last report and a PR record naming pr,
// number and URL both; exactly one candidate's report is taken, capped from
// its head, with the pane's state as ws.Agents gave it before the transcript
// was read; with none, or with several, the description stands in, and the
// reading says why. A pane whose transcript cannot be read counts as one
// with no report, and no screen is read in its place.
func readReport(env Env, ws TaskWorkspace, pr OpenPR, view *prView, now time.Time) (detailRecord, reportReading) {
	rec := detailRecord{PR: pr.Number, Saved: now.UTC(), Source: ReportNone.String()}
	var reading reportReading
	var hits []detailRecord
	for _, a := range ws.Agents {
		if a.Kind != paneinput.AgentClaudeCode {
			continue
		}
		cr := readClaudeReport(env, a)
		found := PaneReport{Pane: a.Pane.String(), Detail: cr.Detail}
		switch cr.Report.Kind {
		case SessionReportFound:
			found.Kind = PaneReportNotNamed
			if cr.Report.Names(pr.Number, pr.URL) {
				found.Kind = PaneReportCandidate
				capped := CapHead(cr.Report.Text, headMaxBytes)
				hits = append(hits, detailRecord{Text: capped.Text, Truncated: capped.Truncated, Pane: a.Pane.String(), Seq: a.StateChangeSeq})
			}
		case SessionReportAbsent:
			found.Kind = PaneReportAbsent
		default:
			found.Kind = PaneReportUnreadable
		}
		reading.panes = append(reading.panes, found)
	}
	switch {
	case len(hits) == 1:
		rec.Source, rec.Text, rec.Truncated, rec.Pane, rec.Seq = ReportFromPane.String(), hits[0].Text, hits[0].Truncated, hits[0].Pane, hits[0].Seq
		return rec, reportReading{}
	case len(reading.panes) == 0:
		reading.reason = DescriptionReasonNoClaudePane
	case len(hits) == 0:
		reading.reason = DescriptionReasonNoCandidate
	default:
		reading.reason = DescriptionReasonSeveralCandidates
	}
	if view != nil {
		capped := CapHead(view.Body, headMaxBytes)
		rec.Source, rec.Text, rec.Truncated = ReportFromDescription.String(), capped.Text, capped.Truncated
	}
	return rec, reading
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
