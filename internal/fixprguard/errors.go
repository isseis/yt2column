package fixprguard

import (
	"errors"
	"fmt"
)

// Sentinel errors. Callers and tests match them with errors.Is. A wrapped
// error adds only the name of the rule that was broken, never a value from
// GitHub or the arguments and standard error of an external command.
var (
	// ErrInvalidInput rejects a malformed flag value: a login, a PR number, a
	// thread or comment ID, a fingerprint, or an accept pair.
	ErrInvalidInput = errors.New("invalid input")
	// ErrCrossRepository reports a PR whose head is not a branch of this
	// repository.
	ErrCrossRepository = errors.New("PR head is not in this repository")
	// ErrThreadChanged reports a thread whose comments changed after the
	// fingerprint the caller holds was taken.
	ErrThreadChanged = errors.New("thread comments changed")
	// ErrThreadNotTrusted refuses a write to a thread that is neither trusted
	// nor named with its current fingerprint.
	ErrThreadNotTrusted = errors.New("thread is not trusted")
	// ErrCommentNotInThread refuses a reply to a comment that is not the
	// thread's first one.
	ErrCommentNotInThread = fmt.Errorf("%w: comment is not the thread's first", ErrInvalidInput)
	// ErrReplyNotResolved reports a reply that was, or may have been, posted to
	// a thread that was then not resolved, so the caller does not post it again.
	ErrReplyNotResolved = errors.New("reply may be posted but thread not resolved")
	// ErrResponseRejected reports a GitHub call that failed or a response that
	// failed the strict check or the size cap. No partial result is used.
	ErrResponseRejected = errors.New("GitHub response rejected")
	// ErrLimitExceeded reports an input over a fixed limit, such as the number
	// of threads list reads or of changed paths.
	ErrLimitExceeded = errors.New("input exceeds a fixed limit")
	// ErrDirtyWorkTree reports uncommitted changes, untracked files included,
	// before a round starts.
	ErrDirtyWorkTree = errors.New("work tree has uncommitted changes")
	// ErrGitStateChanged reports a git state that differs from the baseline;
	// a *GitStateError names the parts.
	ErrGitStateChanged = errors.New("git state differs from the baseline")
	// ErrJudgedSetChanged reports changes that differ from the set inspect
	// judged; a *JudgedSetError lists the changes found instead.
	ErrJudgedSetChanged = errors.New("changes differ from the judged set")
	// ErrBuildChecksFailed reports a failed make target; a *BuildCheckError
	// names the stage.
	ErrBuildChecksFailed = errors.New("build checks failed")
	// ErrGitFailed reports a git command that failed or whose output was not
	// in the expected shape, and a git metadata file that could not be read.
	ErrGitFailed = errors.New("git command failed")

	errNonceExhausted = errors.New("no fence nonce absent from the data")
	errUnknownEnum    = errors.New("enum value has no name")
	errThreadCap      = fmt.Errorf("%w: thread cap", ErrLimitExceeded)
)

// The stop kinds: the machine-readable name of why a subcommand stopped, as
// the output's stop field and the command documents name it.
const (
	StopDirtyWorkTree     = "dirty-work-tree"
	StopCrossRepository   = "cross-repository"
	StopGitStateChanged   = "git-state-changed"
	StopJudgedSetChanged  = "judged-set-changed"
	StopThreadCap         = "thread-cap"
	StopChangeCap         = "change-cap"
	StopThreadChanged     = "thread-changed"
	StopThreadNotTrusted  = "thread-not-trusted"
	StopReplyNotResolved  = "reply-not-resolved"
	StopResponseRejected  = "response-rejected"
	StopBuildChecksFailed = "build-checks-failed"
	StopGitFailed         = "git-failed"
	// StopFailed covers any other failure, so no failure goes unnamed.
	StopFailed = "failed"
)

// StopKinds lists every stop kind StopKind can return.
func StopKinds() []string {
	return []string{
		StopDirtyWorkTree, StopCrossRepository, StopGitStateChanged, StopJudgedSetChanged, StopThreadCap,
		StopChangeCap, StopThreadChanged, StopThreadNotTrusted, StopReplyNotResolved, StopResponseRejected, StopBuildChecksFailed, StopGitFailed, StopFailed,
	}
}

// StopKind names the stop kind of a subcommand's error.
func StopKind(err error) string {
	switch {
	// First: a reply that may exist must be reported as such whatever else
	// the error wraps, so the caller does not post it again.
	case errors.Is(err, ErrReplyNotResolved):
		return StopReplyNotResolved
	case errors.Is(err, ErrDirtyWorkTree):
		return StopDirtyWorkTree
	case errors.Is(err, ErrCrossRepository):
		return StopCrossRepository
	case errors.Is(err, ErrGitStateChanged):
		return StopGitStateChanged
	case errors.Is(err, ErrJudgedSetChanged):
		return StopJudgedSetChanged
	case errors.Is(err, errThreadCap):
		return StopThreadCap
	case errors.Is(err, errChangeCap):
		return StopChangeCap
	case errors.Is(err, ErrThreadChanged):
		return StopThreadChanged
	case errors.Is(err, ErrThreadNotTrusted):
		return StopThreadNotTrusted
	case errors.Is(err, ErrResponseRejected):
		return StopResponseRejected
	case errors.Is(err, ErrBuildChecksFailed):
		return StopBuildChecksFailed
	case errors.Is(err, ErrGitFailed):
		return StopGitFailed
	default:
		return StopFailed
	}
}
