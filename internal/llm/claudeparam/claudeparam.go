// Package claudeparam holds the Claude adapter's parameter types: the
// output_config.effort value and an Anthropic workspace ID. Both the adapter
// (internal/llm/claude) and the configuration loader (internal/config) use
// them, so the list of effort values lives here alone.
package claudeparam

import (
	"errors"
	"fmt"
)

// Effort is the output_config.effort value. The zero value is EffortUnset.
type Effort int

// The accepted Effort values. EffortUnset is the zero value, which no
// construction accepts, so a client is never built with it.
const (
	EffortUnset Effort = iota
	EffortLow
	EffortMedium
	EffortHigh
	EffortXHigh
	EffortMax
)

// The effort wire values. They are listed once here and nowhere else, so a
// change to the accepted set updates every use.
const (
	effortLow    = "low"
	effortMedium = "medium"
	effortHigh   = "high"
	effortXHigh  = "xhigh"
	effortMax    = "max"
)

// unknownEffort is the placeholder String returns for EffortUnset and for a
// value outside the accepted range. ParseEffort rejects it, so a string round
// trip of an invalid Effort fails instead of inventing an accepted value.
const unknownEffort = "unknown"

// Static errors for the rejected values. err113 requires a static sentinel;
// the sentinels are unexported because a caller reports the rejection with its
// own fixed reason and never inspects this error.
var (
	errUnknownEffort    = errors.New("unknown effort")
	errEmptyWorkspaceID = errors.New("workspace ID is empty")
	errNonPrintableWkID = errors.New("workspace ID contains a character outside the printable ASCII range")
)

// The printable ASCII range a workspace ID may use, excluding space.
const (
	workspaceIDMin = 0x21
	workspaceIDMax = 0x7e
)

// ParseEffort returns the Effort for one of "low", "medium", "high", "xhigh",
// "max". Any other string, including a differently cased or padded one,
// returns an error.
func ParseEffort(value string) (Effort, error) {
	switch value {
	case effortLow:
		return EffortLow, nil
	case effortMedium:
		return EffortMedium, nil
	case effortHigh:
		return EffortHigh, nil
	case effortXHigh:
		return EffortXHigh, nil
	case effortMax:
		return EffortMax, nil
	default:
		return EffortUnset, fmt.Errorf("%w: %q", errUnknownEffort, value)
	}
}

// String returns the wire value; EffortUnset and unknown values return a
// fixed placeholder.
func (e Effort) String() string {
	switch e {
	case EffortLow:
		return effortLow
	case EffortMedium:
		return effortMedium
	case EffortHigh:
		return effortHigh
	case EffortXHigh:
		return effortXHigh
	case EffortMax:
		return effortMax
	default:
		return unknownEffort
	}
}

// Valid reports whether e is one of the five accepted effort values. The
// accepted values are the contiguous constants from EffortLow through
// EffortMax, so the range check is the whole list.
func (e Effort) Valid() bool {
	return e >= EffortLow && e <= EffortMax
}

// WorkspaceID is an Anthropic workspace ID. The zero value means none was
// specified; a non-zero value can only come from ParseWorkspaceID.
type WorkspaceID struct {
	value string
}

// ParseWorkspaceID accepts a non-empty string of printable ASCII characters
// other than space (0x21-0x7E). Any other string, the empty string included,
// returns an error.
func ParseWorkspaceID(value string) (WorkspaceID, error) {
	if value == "" {
		return WorkspaceID{}, errEmptyWorkspaceID
	}
	for i := range len(value) {
		if c := value[i]; c < workspaceIDMin || c > workspaceIDMax {
			return WorkspaceID{}, errNonPrintableWkID
		}
	}
	return WorkspaceID{value: value}, nil
}

// Value returns the workspace ID and whether one was specified.
func (w WorkspaceID) Value() (string, bool) {
	if w.value == "" {
		return "", false
	}
	return w.value, true
}
