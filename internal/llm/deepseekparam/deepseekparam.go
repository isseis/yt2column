// Package deepseekparam holds the DeepSeek adapter's parameter type: the
// reasoning_effort value. Both the adapter (internal/llm/deepseek) and the
// configuration loader (internal/config) use it, so the list of values lives
// here alone.
package deepseekparam

import (
	"errors"
	"fmt"
)

// Effort is the reasoning_effort value. The zero value is EffortUnspecified,
// which means no value was specified: the request carries no
// reasoning_effort member.
type Effort int

// The accepted Effort values. EffortUnspecified is the zero value, which
// means "no value specified" and is a valid state for a client: unlike
// claudeparam.EffortUnset, a construction accepts it.
const (
	EffortUnspecified Effort = iota
	EffortNone
	EffortLow
	EffortHigh
	EffortMax
)

// The effort wire values. They are listed once here and nowhere else, so a
// change to the accepted set updates every use.
const (
	effortNone = "none"
	effortLow  = "low"
	effortHigh = "high"
	effortMax  = "max"
)

// unknownEffort is the placeholder String returns for EffortUnspecified and
// for a value outside the accepted range. ParseEffort rejects it, so a string
// round trip of an unspecified or invalid Effort fails instead of inventing an
// accepted value.
const unknownEffort = "unknown"

// errUnknownEffort is the sentinel ParseEffort wraps. It is unexported because
// a caller reports the rejection with its own fixed reason and never inspects
// this error.
var errUnknownEffort = errors.New("unknown effort")

// ParseEffort returns the Effort for one of "none", "low", "high", "max". Any
// other string, the empty string and differently cased or padded ones
// included, returns an error. On error it returns EffortUnspecified, which a
// caller must not use: a client built with it sends no reasoning_effort.
func ParseEffort(value string) (Effort, error) {
	switch value {
	case effortNone:
		return EffortNone, nil
	case effortLow:
		return EffortLow, nil
	case effortHigh:
		return EffortHigh, nil
	case effortMax:
		return EffortMax, nil
	default:
		return EffortUnspecified, fmt.Errorf("%w: %q", errUnknownEffort, value)
	}
}

// String returns the wire value; EffortUnspecified and unknown values return a
// fixed placeholder that ParseEffort rejects.
func (e Effort) String() string {
	switch e {
	case EffortNone:
		return effortNone
	case EffortLow:
		return effortLow
	case EffortHigh:
		return effortHigh
	case EffortMax:
		return effortMax
	default:
		return unknownEffort
	}
}

// Valid reports whether e is one of the four accepted values. It is false for
// EffortUnspecified. The accepted values are the contiguous constants from
// EffortNone through EffortMax, so the range check is the whole list.
func (e Effort) Valid() bool {
	return e >= EffortNone && e <= EffortMax
}

// EffortValues returns the accepted wire values in a fixed order.
func EffortValues() []string {
	return []string{effortNone, effortLow, effortHigh, effortMax}
}
