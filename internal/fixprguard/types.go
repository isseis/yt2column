// Package fixprguard implements the deterministic checks behind /fixpr and
// /runpr: which review threads are trusted, fetching thread bodies only for the
// threads that will be processed, and fencing untrusted text before a prompt
// embeds it.
//
// It is a developer tool. Every value that comes from GitHub is untrusted: it is
// checked strictly, never repaired, and never echoed into an error message.
package fixprguard

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Protocol is the version of the tool's input and output contract. The command
// documents pass it back with --protocol, so an installed binary that predates
// the documents stops instead of judging by older rules.
const Protocol = 1

const (
	// maxLoginLength is GitHub's limit on an account name.
	maxLoginLength = 39
	// maxPRNumberDigits keeps a PR number well inside int.
	maxPRNumberDigits = 9
	// maxCommentIDDigits keeps a comment database ID inside int64.
	maxCommentIDDigits = 18
	// maxThreadIDLength bounds a thread node ID; real ones are about 30 bytes.
	maxThreadIDLength = 100
	// fingerprintLength is the length of a hex-encoded SHA-256.
	fingerprintLength = 64

	threadIDPrefix = "PRRT_"

	// GraphQL __typename values of the two author kinds that can be trusted.
	typenameUser = "User"
	typenameBot  = "Bot"
)

// Login is a GitHub account name that passed the login grammar: ASCII letters,
// digits and hyphens, starting with a letter or digit. The zero value is "no
// usable name"; only ParseLogin builds a usable one, so a Login can be put into
// a request path without further checks.
type Login struct {
	name string
}

// ParseLogin returns s as a Login, rejecting anything outside the grammar
// without repairing it.
func ParseLogin(s string) (Login, error) {
	if !validLogin(s) {
		return Login{}, fmt.Errorf("%w: login grammar", ErrInvalidInput)
	}
	return Login{name: s}, nil
}

// String returns the account name; it is empty for the zero Login.
func (l Login) String() string { return l.name }

func (l Login) usable() bool { return l.name != "" }

func validLogin(s string) bool {
	if s == "" || len(s) > maxLoginLength || !isASCIIAlnum(s[0]) {
		return false
	}
	for i := range len(s) {
		if !isASCIIAlnum(s[i]) && s[i] != '-' {
			return false
		}
	}
	return true
}

func isASCIIAlnum(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// PRNumber is a PR number that passed the decimal-digits check. The zero value
// is not a usable number; only ParsePRNumber builds one.
type PRNumber struct {
	n int
}

// ParsePRNumber accepts decimal digits only: no sign, no leading zero, no URL
// or branch name.
func ParsePRNumber(s string) (PRNumber, error) {
	n, ok := parseDecimal(s, maxPRNumberDigits)
	if !ok {
		return PRNumber{}, fmt.Errorf("%w: PR number grammar", ErrInvalidInput)
	}
	return PRNumber{n: int(n)}, nil
}

// String returns the number in decimal.
func (p PRNumber) String() string { return strconv.Itoa(p.n) }

func (p PRNumber) usable() bool { return p.n > 0 }

// ParseCommentID accepts a comment database ID in decimal digits only.
func ParseCommentID(s string) (int64, error) {
	n, ok := parseDecimal(s, maxCommentIDDigits)
	if !ok {
		return 0, fmt.Errorf("%w: comment ID grammar", ErrInvalidInput)
	}
	return n, nil
}

// parseDecimal accepts a positive decimal integer of at most maxDigits digits
// with no sign and no leading zero.
func parseDecimal(s string, maxDigits int) (int64, bool) {
	if s == "" || len(s) > maxDigits || s[0] == '0' {
		return 0, false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// validThreadID accepts a review thread node ID: the PRRT_ prefix followed by
// ASCII letters, digits, '-' and '_'. A URL or a path never matches.
func validThreadID(s string) bool {
	rest, ok := strings.CutPrefix(s, threadIDPrefix)
	if !ok || rest == "" || len(s) > maxThreadIDLength {
		return false
	}
	for i := range len(rest) {
		if !isASCIIAlnum(rest[i]) && rest[i] != '-' && rest[i] != '_' {
			return false
		}
	}
	return true
}

// AuthorKind mirrors GraphQL __typename. The zero value is unknown, the
// interpretation that assumes least.
type AuthorKind int

// Author kinds.
const (
	AuthorUnknown AuthorKind = iota
	AuthorUser
	AuthorBot
)

// authorKindOf maps a GraphQL __typename to an AuthorKind; any other value is
// unknown.
func authorKindOf(typename string) AuthorKind {
	switch typename {
	case typenameUser:
		return AuthorUser
	case typenameBot:
		return AuthorBot
	default:
		return AuthorUnknown
	}
}

// Reason is why a thread is not trusted. ReasonNone is the zero value.
type Reason int

// Reasons, lettered as the requirements list them.
const (
	ReasonNone                  Reason = iota
	ReasonNoWritePermission            // (a) names the account
	ReasonOtherBot                     // (b) names the account
	ReasonNoAuthorName                 // (c) no author, empty or unusable name
	ReasonPermissionUnavailable        // (d) carries a Cause
	ReasonIncompleteComments           // (e)
	ReasonUnknownAuthorKind            // (f)
)

// MarshalText names the reason. ReasonNone and unknown values are an error, so
// a reason that was never set cannot reach the output.
func (r Reason) MarshalText() ([]byte, error) {
	switch r {
	case ReasonNoWritePermission:
		return []byte("no-write-permission"), nil
	case ReasonOtherBot:
		return []byte("other-bot"), nil
	case ReasonNoAuthorName:
		return []byte("no-author-name"), nil
	case ReasonPermissionUnavailable:
		return []byte("permission-unavailable"), nil
	case ReasonIncompleteComments:
		return []byte("incomplete-comments"), nil
	case ReasonUnknownAuthorKind:
		return []byte("unknown-author-kind"), nil
	default:
		return nil, fmt.Errorf("%w: reason", errUnknownEnum)
	}
}

// Cause says why a permission lookup failed, without any value from GitHub.
type Cause int

// Causes of a failed permission lookup.
const (
	CauseNone Cause = iota
	CauseNotFound
	CauseForbidden
	CauseRateLimited
	CauseBudgetExceeded
	CauseRejectedResponse
	CauseFailed
)

// MarshalText names the cause. CauseNone and unknown values are an error.
func (c Cause) MarshalText() ([]byte, error) {
	switch c {
	case CauseNotFound:
		return []byte("not-found"), nil
	case CauseForbidden:
		return []byte("forbidden"), nil
	case CauseRateLimited:
		return []byte("rate-limited"), nil
	case CauseBudgetExceeded:
		return []byte("budget-exceeded"), nil
	case CauseRejectedResponse:
		return []byte("rejected-response"), nil
	case CauseFailed:
		return []byte("failed"), nil
	default:
		return nil, fmt.Errorf("%w: cause", errUnknownEnum)
	}
}

// Finding is one reason a thread is not trusted. Account is set for (a) and
// (b) only, Cause for (d) only, and CommentURL for (c) only: a name that failed
// the login grammar is never printed, so the comment's URL points at it instead.
type Finding struct {
	Reason     Reason
	Account    Login
	Cause      Cause
	CommentURL string
}

// MarshalJSON writes only the fields the reason carries.
func (f Finding) MarshalJSON() ([]byte, error) {
	out := struct {
		Reason     Reason `json:"reason"`
		Account    string `json:"account,omitempty"`
		Cause      *Cause `json:"cause,omitempty"`
		CommentURL string `json:"commentUrl,omitempty"`
	}{Reason: f.Reason, Account: f.Account.String(), CommentURL: f.CommentURL}
	if f.Cause != CauseNone {
		out.Cause = &f.Cause
	}
	return json.Marshal(out)
}

// Assessment is the zero value "not trusted"; Trusted is set only when every
// comment's author is a trusted author.
type Assessment struct {
	Trusted  bool
	Findings []Finding
}

// Fingerprint identifies the comments of a thread (IDs, authors, times) at one
// moment: a hex-encoded SHA-256.
type Fingerprint string

func validFingerprint(f Fingerprint) bool { return validHex(string(f), fingerprintLength) }

// Accept names a thread together with the fingerprint the user saw. It is the
// value of --accept (a named thread) and of --thread.
type Accept struct {
	ThreadID    string
	Fingerprint Fingerprint
}

// ParseAccept parses "<thread ID>=<fingerprint>", rejecting either half outside
// its grammar.
func ParseAccept(s string) (Accept, error) {
	id, fingerprint, ok := strings.Cut(s, "=")
	if !ok || !validThreadID(id) || !validFingerprint(Fingerprint(fingerprint)) {
		return Accept{}, fmt.Errorf("%w: accept grammar", ErrInvalidInput)
	}
	return Accept{ThreadID: id, Fingerprint: Fingerprint(fingerprint)}, nil
}

// checkAccepts re-checks pairs a caller built without ParseAccept and rejects a
// thread named twice.
func checkAccepts(accepts []Accept) (map[string]Accept, error) {
	byID := make(map[string]Accept, len(accepts))
	for _, accept := range accepts {
		if !validThreadID(accept.ThreadID) || !validFingerprint(accept.Fingerprint) {
			return nil, fmt.Errorf("%w: accept grammar", ErrInvalidInput)
		}
		if _, dup := byID[accept.ThreadID]; dup {
			return nil, fmt.Errorf("%w: thread named twice", ErrInvalidInput)
		}
		byID[accept.ThreadID] = accept
	}
	return byID, nil
}
