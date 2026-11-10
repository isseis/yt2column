// Package openaiparam holds the OpenAI adapter's parameter type: the reasoning
// effort. Both the adapter (internal/llm/openai) and the configuration loader
// (internal/config) use it, so the list of values lives here alone.
package openaiparam

import (
	"errors"
	"fmt"
)

// Effort is the reasoning effort. The zero value is EffortUnset.
type Effort int

// The accepted Effort values. EffortUnset is the zero value, which no
// construction accepts, so a client is never built with it. EffortNone is a
// distinct accepted value: it turns reasoning off rather than leaving the
// value unset.
const (
	EffortUnset Effort = iota
	EffortNone
	EffortMinimal
	EffortLow
	EffortMedium
	EffortHigh
	EffortXHigh
	EffortMax
)

// The effort wire values. They are listed once here and nowhere else, so a
// change to the accepted set updates every use.
const (
	effortNone    = "none"
	effortMinimal = "minimal"
	effortLow     = "low"
	effortMedium  = "medium"
	effortHigh    = "high"
	effortXHigh   = "xhigh"
	effortMax     = "max"
)

// unknownEffort is the placeholder String returns for EffortUnset and for a
// value outside the accepted range. ParseEffort rejects it, so a string round
// trip of an invalid Effort fails instead of inventing an accepted value.
const unknownEffort = "unknown"

// errUnknownEffort is the sentinel ParseEffort wraps. It is unexported because
// a caller reports the rejection with its own fixed reason and never inspects
// this error.
var errUnknownEffort = errors.New("unknown effort")

// ParseEffort returns the Effort for one of "none", "minimal", "low",
// "medium", "high", "xhigh", "max". Any other string, including a differently
// cased or padded one, returns an error.
func ParseEffort(value string) (Effort, error) {
	switch value {
	case effortNone:
		return EffortNone, nil
	case effortMinimal:
		return EffortMinimal, nil
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
// fixed placeholder that ParseEffort rejects.
func (e Effort) String() string {
	switch e {
	case EffortNone:
		return effortNone
	case EffortMinimal:
		return effortMinimal
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

// Valid reports whether e is one of the seven accepted values. It is false for
// EffortUnset. The accepted values are the contiguous constants from EffortNone
// through EffortMax, so the range check is the whole list.
func (e Effort) Valid() bool {
	return e >= EffortNone && e <= EffortMax
}

// EffortValues returns the accepted wire values in a fixed order.
func EffortValues() []string {
	return []string{effortNone, effortMinimal, effortLow, effortMedium, effortHigh, effortXHigh, effortMax}
}
