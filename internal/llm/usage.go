package llm

import "errors"

// TokenCounts is the token count one LLM call reported, in the four
// categories that are priced differently. The fields are unsigned, so a
// negative count cannot be represented.
type TokenCounts struct {
	Input      uint64 // input neither read from nor written to the prompt cache
	CacheRead  uint64 // input read from the prompt cache
	CacheWrite uint64 // input written to the prompt cache
	Output     uint64 // output, reasoning and thinking output included
}

// UnknownRule is why a usage is unknown. The zero value RuleNotRead means no
// usage was read: the call failed before its usage was read, or the client
// reports none.
type UnknownRule int

// The UnknownRule values. RuleNotRead and RuleRecordedUnknown describe no
// broken member; every other rule does.
const (
	RuleNotRead               UnknownRule = iota
	RuleMissing                           // a required member is missing
	RuleDuplicate                         // a consumed member appears more than once
	RuleNotObject                         // the usage member is not an object
	RuleNotNonNegativeInteger             // a count is not a non-negative integer that fits int64
	RuleRecordedUnknown                   // a saved record stores the usage as unknown
	RuleInconsistent                      // the counts of a breakdown do not fit within its total
)

// PricingContext is the provider-independent condition a usage's counts were
// produced under, which decides whether a rate that requires a condition may
// price them. It is separate from whether the counts are known: a call can
// report exact counts under a condition the caller could not confirm.
type PricingContext int

// The PricingContext values.
const (
	// PricingContextUnspecified reports no extra pricing condition. Only a rate
	// that requires no condition prices such a usage.
	PricingContextUnspecified PricingContext = iota
	// PricingContextStandard30m reports a confirmed Standard processing tier
	// with a 30-minute prompt cache retention.
	PricingContextStandard30m
	// PricingContextUnverified reports that the condition a rate needs could
	// not be confirmed.
	PricingContextUnverified
)

// ErrInvalidPricingContext rejects a PricingContext outside the enumeration.
var ErrInvalidPricingContext = errors.New("invalid pricing context")

// valid reports whether c is one of the enumerated values.
func (c PricingContext) valid() bool {
	switch c {
	case PricingContextUnspecified, PricingContextStandard30m, PricingContextUnverified:
		return true
	default:
		return false
	}
}

// Usage is the token usage of one call: known counts with the pricing
// context they were produced under, or unknown with a rule and the member it
// applies to. The zero value is unknown with RuleNotRead, so it is never read
// as zero tokens. An unknown usage always has PricingContextUnspecified: with
// no counts there is nothing to price. It is comparable with ==.
type Usage struct {
	known   bool
	counts  TokenCounts
	pricing PricingContext
	rule    UnknownRule
	member  string
}

// KnownUsage returns the usage with the given counts and
// PricingContextUnspecified.
func KnownUsage(counts TokenCounts) Usage {
	return Usage{known: true, counts: counts}
}

// KnownUsageWithPricingContext returns the usage with the given counts and
// pricing context. It rejects a context outside the enumeration with
// ErrInvalidPricingContext, so a Usage never holds one.
func KnownUsageWithPricingContext(counts TokenCounts, pricing PricingContext) (Usage, error) {
	if !pricing.valid() {
		return Usage{}, ErrInvalidPricingContext
	}
	return Usage{known: true, counts: counts, pricing: pricing}, nil
}

// UnknownUsage returns an unknown usage. member names the usage member the
// rule applies to, relative to the provider's usage object ("" for the usage
// member itself). It must be a constant of the caller, never a response or
// file value, and must not contain a space.
func UnknownUsage(rule UnknownRule, member string) Usage {
	return Usage{rule: rule, member: member}
}

// Counts returns the counts and true when the usage is known, or the zero
// counts and false when it is unknown.
func (u Usage) Counts() (TokenCounts, bool) {
	if !u.known {
		return TokenCounts{}, false
	}
	return u.counts, true
}

// PricingContext returns the pricing context of a known usage, and
// PricingContextUnspecified for an unknown one.
func (u Usage) PricingContext() PricingContext {
	return u.pricing
}

// UnknownReason returns why the usage is unknown as one space-free token, or
// "" when it is known: "usage:<rule>" or "usage.<member>:<rule>" for a broken
// rule, "not-read" for RuleNotRead, "recorded-unknown" for
// RuleRecordedUnknown, and "invalid-rule" for a rule outside the enumeration.
func (u Usage) UnknownReason() string {
	if u.known {
		return ""
	}
	switch u.rule {
	case RuleNotRead:
		return "not-read"
	case RuleRecordedUnknown:
		return "recorded-unknown"
	case RuleMissing:
		return u.brokenReason("missing")
	case RuleDuplicate:
		return u.brokenReason("duplicate")
	case RuleNotObject:
		return u.brokenReason("not-an-object")
	case RuleNotNonNegativeInteger:
		return u.brokenReason("not-a-non-negative-integer")
	case RuleInconsistent:
		return u.brokenReason("inconsistent")
	default:
		return "invalid-rule"
	}
}

// brokenReason renders the reason of a broken rule, with the member it applies
// to when the caller named one.
func (u Usage) brokenReason(rule string) string {
	if u.member == "" {
		return "usage:" + rule
	}
	return "usage." + u.member + ":" + rule
}
