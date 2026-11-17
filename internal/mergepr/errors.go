package mergepr

import "errors"

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

// errUnexpectedGitOutput reports git output the check cannot parse. The check
// wraps it in a protocolUnreadableError, so it never stops a run on its own.
var errUnexpectedGitOutput = errors.New("unexpected git output")
