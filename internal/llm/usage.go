package llm

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

// The UnknownRule values. RuleMissing and the rules after it describe a broken
// member; RuleNotRead and RuleRecordedUnknown do not.
const (
	RuleNotRead               UnknownRule = iota
	RuleMissing                           // a required member is missing
	RuleDuplicate                         // a consumed member appears more than once
	RuleNotObject                         // the usage member is not an object
	RuleNotNonNegativeInteger             // a count is not a non-negative integer that fits int64
	RuleRecordedUnknown                   // a saved record stores the usage as unknown
)

// Usage is the token usage of one call: known counts, or unknown with a rule
// and the member it applies to. The zero value is unknown with RuleNotRead,
// so it is never read as zero tokens. It is comparable with ==.
type Usage struct {
	known  bool
	counts TokenCounts
	rule   UnknownRule
	member string
}

// KnownUsage returns the usage with the given counts.
func KnownUsage(counts TokenCounts) Usage {
	return Usage{known: true, counts: counts}
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
