package mergepr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/isseis/yt2column/internal/numreserve"
)

// Sentinel errors. Callers and tests match them with errors.Is.
var (
	errPRNotOpen        = errors.New("PR is not open")
	errPRNotMerged      = errors.New("PR is not merged")
	errBaseChanged      = errors.New("PR base changed after prepare")
	errChecksFailed     = errors.New("CI checks failed")
	errInvalidState     = errors.New("invalid state")
	errInvalidSubject   = errors.New("squash subject is not a non-empty single line")
	errLocalBranchMoved = errors.New("local head branch moved after prepare; not deleted")
	errMergedHeadMoved  = errors.New("PR merged a different head than prepared; local branches left untouched")
	errUnprintablePath  = errors.New("work directory path contains a line break and cannot be reported on a line-oriented output")
	errWorkDirMismatch  = errors.New("state file does not name a generated work directory it lives in; not removing anything")
)

// Sentinel errors of the protocol-number check. Both messages start with
// "protocol check: " so a report can tell a stop of this check from the other
// stops of prepare and merge.
var (
	errProtocolMismatch   = errors.New("protocol check: number is not origin/main's value plus one")
	errProtocolUnreadable = errors.New("protocol check: cannot check protocol numbers")
)

// errNumberUnreadable reports a number check that could not read its inputs.
// Its message starts with "number check: " so a report can tell a stop of
// this check from the other stops of prepare and merge.
var errNumberUnreadable = errors.New("number check: cannot check the numbers the PR adds")

// NumberIssuesError reports the findings of the number check on the task and
// ADR numbers a PR adds. A caller prints each finding with its Line method,
// one line each.
type NumberIssuesError struct {
	Issues []numreserve.AddedNumberIssue
}

// Error implements error.
func (e *NumberIssuesError) Error() string {
	return fmt.Sprintf("number check: %d finding(s) on the numbers the PR adds", len(e.Issues))
}

// errUnexpectedGitOutput reports git output a check cannot parse. It is
// always wrapped in the error the calling check reports, which is what
// callers match.
var errUnexpectedGitOutput = errors.New("unexpected git output")

// errCorrectionUnreadable reports that the correction lines of the PR's
// commits could not be read: the repository is shallow, a ref did not
// resolve, rev-list or cat-file failed or printed an unexpected form, or the
// trailer reading failed. The preparation still holds. Its message starts
// with "correction check: " so a report can tell a stop of this check from
// the other stops.
var errCorrectionUnreadable = errors.New("correction check: cannot read the correction lines of the PR's commits")

// errMergeNotObjectID reports a merge commit ID that is not a full object
// ID, so it is never passed to git.
var errMergeNotObjectID = errors.New("the merge commit is not a full commit ID")

// errCorrectionMismatch reports that the drops, the subject, or the body do
// not match the correction lines of the range. Its message starts with
// "correction check: ".
var errCorrectionMismatch = errors.New("correction check: the message does not match the PR's correction lines")

// errCorrectionsNotCarried reports that the post-merge comparison found a
// difference or could not read the merge commit. Its message starts with
// "post-merge check: ", because the PR is merged by then and no preparation
// is left to discard.
var errCorrectionsNotCarried = errors.New("post-merge check: the merged commit does not carry the body's correction lines")

// errInvalidDrop reports a --drop or --drop-rejected value of the wrong form,
// or a line named twice.
var errInvalidDrop = errors.New("invalid drop")

// errNotLineNumber reports a number that is not a positive decimal without a
// sign or a leading zero that fits an int.
var errNotLineNumber = errors.New("not a positive decimal line number without a sign or a leading zero")

// CorrectionMismatchError reports why the message does not match. Every
// line field holds shown forms, so a line from the body cannot disturb the
// terminal. Error starts with "correction check: "; Unwrap returns
// errCorrectionMismatch.
type CorrectionMismatchError struct {
	Missing         []string       // in the block, not in the body
	Extra           []string       // in the body (or the subject), not in the block
	NotParagraph    bool           // the reserved-prefix lines are not one paragraph of their own
	TrailersChanged bool           // the block changes which trailers git reads from the body
	Undeclared      []RejectedLine // rejected lines with no --drop-rejected
	UnknownDrops    []string       // --drop or --drop-rejected values that name no line
}

// Error counts each reason; a caller prints the lines from the fields.
func (e *CorrectionMismatchError) Error() string {
	var reasons []string
	add := func(n int, what string) {
		if n > 0 {
			reasons = append(reasons, fmt.Sprintf("%d %s", n, what))
		}
	}
	add(len(e.Missing), "missing line(s)")
	add(len(e.Extra), "extra line(s)")
	if e.NotParagraph {
		reasons = append(reasons, "the lines are not one paragraph of their own")
	}
	if e.TrailersChanged {
		reasons = append(reasons, "the lines change which trailers git reads")
	}
	add(len(e.Undeclared), "rejected line(s) without --drop-rejected")
	add(len(e.UnknownDrops), "drop(s) naming no line")
	return errCorrectionMismatch.Error() + ": " + strings.Join(reasons, "; ")
}

// Unwrap returns errCorrectionMismatch.
func (e *CorrectionMismatchError) Unwrap() error { return errCorrectionMismatch }

// CorrectionsNotCarriedError reports the post-merge comparison's result.
// Error starts with "post-merge check: " and names MergeCommit; Unwrap
// returns errCorrectionsNotCarried.
type CorrectionsNotCarriedError struct {
	MergeCommit string
	Missing     []string // in the body, not in the merge commit (shown forms)
	Extra       []string // in the merge commit, not in the body (shown forms)
	Detail      string   // set when the merge commit could not be read
}

// Error names the merge commit, says that nothing is left to discard, and
// counts the lines or gives why the commit could not be read.
func (e *CorrectionsNotCarriedError) Error() string {
	msg := fmt.Sprintf("%v: the PR is merged as %s, and no preparation is left to discard", errCorrectionsNotCarried, e.MergeCommit)
	if e.Detail != "" {
		return msg + "; could not read the merged commit: " + e.Detail
	}
	return fmt.Sprintf("%s; %d missing line(s), %d extra line(s)", msg, len(e.Missing), len(e.Extra))
}

// Unwrap returns errCorrectionsNotCarried.
func (e *CorrectionsNotCarriedError) Unwrap() error { return errCorrectionsNotCarried }
