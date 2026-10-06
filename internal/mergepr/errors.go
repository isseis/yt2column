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
	errWorkDirMismatch  = errors.New("state file does not name the directory it lives in; not removing anything")
)
