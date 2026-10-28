// The rate table below is the single source of the providers' published rates.
// See docs/dev/guides/pricing_update.md for how to update it.

package pricing

import (
	"errors"
	"time"
)

// microUSDPerMTok is a rate in 10^-6 USD per million tokens. A rate of
// $0.003 per million tokens is 3000. It is unsigned, so a negative rate
// cannot be written.
type microUSDPerMTok uint64

// rates is one rate set: the rate of each of the four token categories.
type rates struct {
	input      microUSDPerMTok
	cacheRead  microUSDPerMTok
	cacheWrite microUSDPerMTok
	output     microUSDPerMTok
}

// rateScheme is how a model's rate set is chosen. The zero value is not a
// scheme, so an entry that forgets to declare one fails validation.
type rateScheme int

const (
	schemeUnset      rateScheme = iota
	schemeFlat                  // one rate set
	schemeTimeOfDay             // off-peak and peak rate sets, chosen by the call time
	schemePromptTier            // two rate sets, chosen by the prompt length
)

// modelRates is one entry of the rate table. Each field other than model,
// scheme, and base belongs to one scheme and must be zero for the others.
type modelRates struct {
	model     string
	scheme    rateScheme
	base      rates    // flat: the rates; time of day: off-peak; prompt tier: at or below the threshold
	alternate rates    // time of day: peak; prompt tier: above the threshold
	threshold uint64   // prompt tier only: the largest prompt length that uses base
	peak      peakRule // time of day only
}

// peakRule is the weekly peak window of a time-of-day model, in UTC: the
// weekdays it applies to and the half-open [start, end) minute-of-day ranges.
type peakRule struct {
	weekdays []time.Weekday
	ranges   []minuteRange
}

// minuteRange is a half-open [start, end) range of minutes after 00:00 UTC.
type minuteRange struct {
	start, end int
}

// maxMinuteOfDay bounds a minuteRange: minutes run from 0 through 1439.
// minutesPerHour converts a clock hour to minutes.
const (
	maxMinuteOfDay = 24 * 60
	minutesPerHour = 60
)

// Sentinel errors for the rules the rate table must satisfy. validate returns
// one of them; only a unit test calls validate.
var (
	errEmptyModel           = errors.New("pricing: rate table: empty model name")
	errDuplicateModel       = errors.New("pricing: rate table: duplicate model name")
	errUnknownScheme        = errors.New("pricing: rate table: unknown rate scheme")
	errMissingAlternate     = errors.New("pricing: rate table: scheme needs an alternate rate set")
	errUnusedField          = errors.New("pricing: rate table: field is not used by the scheme")
	errNonPositiveThreshold = errors.New("pricing: rate table: prompt-length threshold is not positive")
	errInvalidPeakRule      = errors.New("pricing: rate table: invalid peak rule")
)

// rateTable holds the rates this package prices with. Each entry declares how
// its rate set is chosen (scheme) and carries, in that scheme's fields only,
// the rate sets and the rule that selects one, with its source and checked
// date in the comment above it.
var rateTable = []modelRates{
	// Source: https://api-docs.deepseek.com/quick_start/pricing (checked 2026-10-16)
	{
		model:  "deepseek-flash",
		scheme: schemeTimeOfDay,
		// The cache-write rate is 0 because this model reports no cache-write
		// category: the adapter always declares that category 0.
		base:      rates{input: 150000, cacheRead: 3000, cacheWrite: 0, output: 600000},  //nolint:mnd // published per-million-token rates
		alternate: rates{input: 300000, cacheRead: 6000, cacheWrite: 0, output: 1200000}, //nolint:mnd // published per-million-token rates
		// Source: https://api-docs.deepseek.com/quick_start/pricing (checked 2026-10-16)
		peak: peakRule{
			weekdays: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
			ranges:   []minuteRange{{start: 60, end: 240}, {start: 360, end: 600}}, //nolint:mnd // peak window in minutes of the day
		},
	},
	// Source: https://platform.claude.com/docs/en/about-claude/pricing (checked 2026-10-16)
	{
		model:  "claude-opus-5-5",
		scheme: schemeFlat,
		base:   rates{input: 4000000, cacheRead: 200000, cacheWrite: 5000000, output: 20000000}, //nolint:mnd // published per-million-token rates
	},
	// Source: https://platform.claude.com/docs/en/about-claude/pricing (checked 2026-10-16)
	{
		model:  "claude-sonnet-5-5",
		scheme: schemeFlat,
		base:   rates{input: 2000000, cacheRead: 100000, cacheWrite: 2500000, output: 10000000}, //nolint:mnd // published per-million-token rates
	},
	// Source: https://platform.claude.com/docs/en/about-claude/pricing (checked 2026-10-16)
	{
		model:     "claude-haiku-5-5",
		scheme:    schemePromptTier,
		base:      rates{input: 100000, cacheRead: 10000, cacheWrite: 125000, output: 500000},  //nolint:mnd // published per-million-token rates
		alternate: rates{input: 500000, cacheRead: 50000, cacheWrite: 625000, output: 2500000}, //nolint:mnd // published per-million-token rates
		threshold: 100000,                                                                      //nolint:mnd // published prompt-length threshold in tokens
	},
}

// validate reports the first rule the table breaks, or nil when it satisfies
// every rule. The rate table is a constant, so only a unit test calls this.
func validate(table []modelRates) error {
	seen := make(map[string]bool, len(table))
	for _, m := range table {
		if m.model == "" {
			return errEmptyModel
		}
		if seen[m.model] {
			return errDuplicateModel
		}
		seen[m.model] = true
		if err := validateEntry(m); err != nil {
			return err
		}
	}
	return nil
}

// validateEntry checks the fields one entry uses against its scheme.
func validateEntry(m modelRates) error {
	switch m.scheme {
	case schemeFlat:
		if m.alternate != (rates{}) || m.threshold != 0 || !peakIsZero(m.peak) {
			return errUnusedField
		}
	case schemeTimeOfDay:
		if m.threshold != 0 {
			return errUnusedField
		}
		if m.alternate == (rates{}) {
			return errMissingAlternate
		}
		return validatePeakRule(m.peak)
	case schemePromptTier:
		if !peakIsZero(m.peak) {
			return errUnusedField
		}
		if m.alternate == (rates{}) {
			return errMissingAlternate
		}
		if m.threshold == 0 {
			return errNonPositiveThreshold
		}
	default:
		return errUnknownScheme
	}
	return nil
}

// peakIsZero reports whether a peak rule carries no rule at all.
func peakIsZero(p peakRule) bool {
	return len(p.weekdays) == 0 && len(p.ranges) == 0
}

// validatePeakRule checks the weekly peak window: non-empty, distinct
// weekdays in Sunday through Saturday, and non-empty, in-day, non-overlapping
// half-open ranges.
func validatePeakRule(p peakRule) error {
	if len(p.weekdays) == 0 || len(p.ranges) == 0 {
		return errInvalidPeakRule
	}
	seen := make(map[time.Weekday]bool, len(p.weekdays))
	for _, d := range p.weekdays {
		if d < time.Sunday || d > time.Saturday || seen[d] {
			return errInvalidPeakRule
		}
		seen[d] = true
	}
	for i, r := range p.ranges {
		if r.start < 0 || r.end > maxMinuteOfDay || r.start >= r.end {
			return errInvalidPeakRule
		}
		for _, other := range p.ranges[:i] {
			if r.start < other.end && other.start < r.end {
				return errInvalidPeakRule
			}
		}
	}
	return nil
}
