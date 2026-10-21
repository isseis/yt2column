package fixprguard

import "errors"

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
	// ErrResponseRejected reports a GitHub call that failed or a response that
	// failed the strict check or the size cap. No partial result is used.
	ErrResponseRejected = errors.New("GitHub response rejected")
	// ErrLimitExceeded reports an input over a fixed limit, such as the number
	// of threads list reads.
	ErrLimitExceeded = errors.New("input exceeds a fixed limit")

	errNonceExhausted = errors.New("no fence nonce absent from the data")
	errUnknownEnum    = errors.New("enum value has no name")
)
