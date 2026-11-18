package mergepr

import (
	"errors"
	"fmt"

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

// errUnexpectedGitOutput reports git output the check cannot parse. The check
// reports it as the Detail of a protocolUnreadableError, which is what callers
// match.
var errUnexpectedGitOutput = errors.New("unexpected git output")
