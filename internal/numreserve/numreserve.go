// Package numreserve decides a collision-free task or ADR number by creating
// a record tag on GitHub, and checks the numbers a PR adds before a merge.
//
// The number is unique because GitHub refuses to create a ref that already
// exists: the tool creates refs/tags/<space>-number/NNNN through the REST API
// and returns the number only when that creation succeeds. It moves on to the
// next number only when a creation fails with 422 and reading the ref back
// shows a ref of exactly that name; every other failure, every input of an
// unexpected form, and every exceeded bound stops it with a StopError.
//
// The tool reads the output of git and gh to choose its behavior: the HTTP
// status line and the rate-limit headers of gh api -i, the ref of a GET
// response (whose whole body must be strict JSON), git ls-remote lines,
// git ls-tree entries, git for-each-ref lines, the one line of git
// rev-parse --show-toplevel (a local-git stop when it is not one absolute
// path), the one line of git merge-base (a malformed stop when it is not one
// commit ID), and, only to choose the branches a stop's detail names, git
// cat-file --batch-check lines. These are values
// written by external commands that cannot be expressed in a type; a line or
// entry of any form the tool does not accept stops it as malformed (a status
// line as network-or-github) rather than being read more widely.
package numreserve

import (
	"errors"
	"fmt"
	"strconv"
)

// Repository is the GitHub repository the record tags live in. The tool
// never reads origin's URL: in a cloud session it may be a proxy URL, and a
// URL may carry a token that must not reach a stop detail.
const Repository = "isseis/yt2column"

// Space is a number space. Its zero value is invalid, so a caller must choose.
type Space int

const (
	spaceInvalid Space = iota
	// SpaceTask is the task number space: refs/tags/task-number/NNNN and
	// docs/tasks/NNNN_<name>.
	SpaceTask
	// SpaceADR is the ADR number space: refs/tags/adr-number/NNNN and
	// docs/dev/adr/NNNN-<kebab>.md.
	SpaceADR
)

// ErrInvalidSpace reports a space argument other than "task" and "adr", or a
// Space value outside the declared ones.
var ErrInvalidSpace = errors.New("invalid number space")

// ParseSpace accepts "task" and "adr" only. It is the one entry from a string
// to a Space.
func ParseSpace(arg string) (Space, error) {
	switch arg {
	case "task":
		return SpaceTask, nil
	case "adr":
		return SpaceADR, nil
	default:
		return spaceInvalid, fmt.Errorf("%w: %q", ErrInvalidSpace, arg)
	}
}

// TagPrefix returns the record tag namespace of the space, ending in "/".
// The record ruleset the user sets targets every name under it.
func (s Space) TagPrefix() (string, error) {
	switch s {
	case SpaceTask:
		return "refs/tags/task-number/", nil
	case SpaceADR:
		return "refs/tags/adr-number/", nil
	default:
		return "", fmt.Errorf("%w: %d", ErrInvalidSpace, int(s))
	}
}

// StopKind is the declared reason the tool stopped without a number. Its
// zero value is invalid.
type StopKind int

const (
	stopInvalid StopKind = iota
	// StopPermission means the creation was refused for lack of permission.
	StopPermission
	// StopNetworkOrGitHub means a network, GitHub, or time-limit failure, or a
	// failure that could not be confirmed as "already used".
	StopNetworkOrGitHub
	// StopAttempts means the creation was tried the maximum number of times.
	StopAttempts
	// StopExhausted means no four-digit number is left.
	StopExhausted
	// StopJump means the candidate is too far above the base maximum.
	StopJump
	// StopListLimit means a listing exceeded its bound.
	StopListLimit
	// StopMalformed means an input of a form the tool does not accept.
	StopMalformed
	// StopLocalGit means a local git command failed.
	StopLocalGit
)

// stopKinds lists every declared StopKind, in order.
var stopKinds = []StopKind{
	StopPermission,
	StopNetworkOrGitHub,
	StopAttempts,
	StopExhausted,
	StopJump,
	StopListLimit,
	StopMalformed,
	StopLocalGit,
}

// StopKinds returns every declared StopKind. Each call returns a new slice.
func StopKinds() []StopKind {
	return append([]StopKind(nil), stopKinds...)
}

// Reason returns the reason word the tool prints for the kind, and false for
// the zero value or a value outside the declared ones, which have none.
func (k StopKind) Reason() (string, bool) {
	switch k {
	case StopPermission:
		return "permission", true
	case StopNetworkOrGitHub:
		return "network-or-github", true
	case StopAttempts:
		return "attempts", true
	case StopExhausted:
		return "exhausted", true
	case StopJump:
		return "jump", true
	case StopListLimit:
		return "list-limit", true
	case StopMalformed:
		return "malformed", true
	case StopLocalGit:
		return "local-git", true
	default:
		return "", false
	}
}

// StopError carries the kind, the step, and the values a user needs to find
// the cause (branch names, file paths, a status code). It never carries a
// response body, a command's standard error, a remote URL, or a token: the
// tool builds Step and Detail from its own words, numbers, and quoted values.
// Callers read it with errors.AsType[*StopError].
type StopError struct {
	Kind   StopKind
	Step   string
	Detail string
}

// Error implements error.
func (e *StopError) Error() string {
	reason, ok := e.Kind.Reason()
	if !ok {
		reason = "unknown"
	}
	return fmt.Sprintf("stop %s: %s: %s", reason, e.Step, e.Detail)
}

func stop(kind StopKind, step, format string, args ...any) *StopError {
	return &StopError{Kind: kind, Step: step, Detail: fmt.Sprintf(format, args...)}
}

// maxValueBytes bounds each untrusted value (a branch name, a file path) in a
// stop detail, before it is quoted.
const maxValueBytes = 200

// quote returns an untrusted value as one Go-quoted string of at most
// maxValueBytes bytes of the value, so a newline or a control character in
// the value cannot forge an output line.
func quote(value string) string {
	if len(value) > maxValueBytes {
		value = value[:maxValueBytes]
	}
	return strconv.Quote(value)
}
