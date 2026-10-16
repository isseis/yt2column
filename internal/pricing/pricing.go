// Package pricing estimates what an LLM call cost from its token counts and
// the providers' published rates, which this package holds as constants.
// It uses writer's value types only and never calls into writer.
package pricing

import (
	"math/big"
	"slices"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
)

// RateBasis is which rate set a known cost used. The zero value BasisUnset is
// not a basis; a known cost never carries it.
type RateBasis int

const (
	// BasisUnset is the zero value: no rate set was chosen.
	BasisUnset RateBasis = iota
	// BasisFlat is the one rate set of a model that does not vary its rates.
	BasisFlat
	// BasisOffPeak is the off-peak rate set of a time-of-day model.
	BasisOffPeak
	// BasisPeak is the peak rate set of a time-of-day model.
	BasisPeak
	// BasisShortPrompt is the rate set of a prompt-tier model at or below its
	// threshold.
	BasisShortPrompt
	// BasisLongPrompt is the rate set of a prompt-tier model above its
	// threshold.
	BasisLongPrompt
)

// UnknownReason is why a stage cost is unknown. The zero value ReasonUnset
// means no reason was determined; together with ReasonInvalid it marks a
// state the rate table or the callers should never produce.
type UnknownReason int

const (
	// ReasonUnset is the zero value: no reason was determined.
	ReasonUnset UnknownReason = iota
	// ReasonUsageUnknown means the stage's token counts are unknown.
	ReasonUsageUnknown
	// ReasonNoModel means the stage has no verified model name.
	ReasonNoModel
	// ReasonUnpricedModel means the model is not in the rate table.
	ReasonUnpricedModel
	// ReasonInvalid marks a broken invariant: no call time for a time-of-day
	// model, or an unknown rate scheme.
	ReasonInvalid
)

// StageCost is the estimated cost of one stage: known, with its amount and
// the rate set used, or unknown, with a reason. The zero value is unknown with
// ReasonUnset, so it is never read as 0 USD.
type StageCost struct {
	known  bool
	amount Amount
	basis  RateBasis
	reason UnknownReason
}

// Known returns the amount, the rate set used, and true when the cost is known.
func (c StageCost) Known() (Amount, RateBasis, bool) {
	if !c.known {
		return Amount{}, BasisUnset, false
	}
	return c.amount, c.basis, true
}

// Unknown returns the reason and true when the cost is unknown.
func (c StageCost) Unknown() (UnknownReason, bool) {
	if c.known {
		return ReasonUnset, false
	}
	return c.reason, true
}

// Total is the sum of the costs of a set of stages: known, or unknown when any
// stage in the set is unknown. The zero value is unknown.
type Total struct {
	known  bool
	amount Amount
}

// Amount returns the amount and true when the total is known.
func (t Total) Amount() (Amount, bool) {
	if !t.known {
		return Amount{}, false
	}
	return t.amount, true
}

// Estimate returns the cost of one call of model with usage, whose Generate
// returned at callTime. The model is matched against the rate table exactly.
// callTime is read in UTC and is used only by a time-of-day model.
func Estimate(model string, usage llm.Usage, callTime time.Time) StageCost {
	counts, known := usage.Counts()
	if !known {
		return unknownCost(ReasonUsageUnknown)
	}
	if model == "" {
		return unknownCost(ReasonNoModel)
	}
	entry, ok := lookupModel(model)
	if !ok {
		return unknownCost(ReasonUnpricedModel)
	}
	return estimateEntry(entry, counts, callTime)
}

// estimateEntry returns the cost of counts against one table entry.
func estimateEntry(entry modelRates, counts llm.TokenCounts, callTime time.Time) StageCost {
	chosen, basis, ok := chooseRates(entry, counts, callTime)
	if !ok {
		return unknownCost(ReasonInvalid)
	}
	return StageCost{known: true, amount: costOf(counts, chosen), basis: basis}
}

// Totals returns the cost of this run (the stages with OriginRun) and the cost
// of the article (the stages with OriginRun or OriginReused). A stage with
// OriginNone is in neither set. The article total is unknown when the
// generation stage is OriginNone, because an article always has one.
func Totals(run writer.RunUsage) (thisRun, article Total) {
	thisRun = sumStages(run, func(origin writer.UsageOrigin) bool {
		return origin == writer.OriginRun
	})
	if run.Generation.Origin == writer.OriginNone {
		return thisRun, Total{}
	}
	article = sumStages(run, func(origin writer.UsageOrigin) bool {
		return origin == writer.OriginRun || origin == writer.OriginReused
	})
	return thisRun, article
}

// sumStages sums the costs of the stages of run that include accepts. An empty
// set is a known 0; a single unknown stage makes the whole total unknown.
func sumStages(run writer.RunUsage, include func(writer.UsageOrigin) bool) Total {
	var sum Amount
	for _, stage := range [...]writer.StageUsage{run.Generation, run.Shorten, run.Review} {
		if !include(stage.Origin) {
			continue
		}
		amount, _, known := Estimate(stage.Model, stage.Usage, stage.CallTime).Known()
		if !known {
			return Total{}
		}
		sum = sum.add(amount)
	}
	return Total{known: true, amount: sum}
}

// lookupModel finds the rate table entry whose model matches exactly.
func lookupModel(model string) (modelRates, bool) {
	for _, entry := range rateTable {
		if entry.model == model {
			return entry, true
		}
	}
	return modelRates{}, false
}

// chooseRates selects the rate set of entry. The second result is false when
// the entry's scheme is outside the enumeration, or a time-of-day entry has no
// call time; both are failures the table's validation and writer's invariant
// keep out of a real run.
func chooseRates(entry modelRates, counts llm.TokenCounts, callTime time.Time) (rates, RateBasis, bool) {
	switch entry.scheme {
	case schemeFlat:
		return entry.base, BasisFlat, true
	case schemeTimeOfDay:
		if callTime.IsZero() {
			return rates{}, BasisUnset, false
		}
		if inPeak(entry.peak, callTime.UTC()) {
			return entry.alternate, BasisPeak, true
		}
		return entry.base, BasisOffPeak, true
	case schemePromptTier:
		if promptLengthOver(counts, entry.threshold) {
			return entry.alternate, BasisLongPrompt, true
		}
		return entry.base, BasisShortPrompt, true
	default:
		return rates{}, BasisUnset, false
	}
}

// inPeak reports whether at falls in the model's weekly peak window. at is in
// UTC; a range includes its start and excludes its end.
func inPeak(p peakRule, at time.Time) bool {
	if !slices.Contains(p.weekdays, at.Weekday()) {
		return false
	}
	minute := at.Hour()*minutesPerHour + at.Minute()
	for _, r := range p.ranges {
		if minute >= r.start && minute < r.end {
			return true
		}
	}
	return false
}

// promptLengthOver reports whether the sum of the three input categories
// exceeds threshold. It compares against the remaining allowance instead of
// adding, so the sum cannot overflow uint64.
func promptLengthOver(counts llm.TokenCounts, threshold uint64) bool {
	remaining := threshold
	for _, n := range [...]uint64{counts.Input, counts.CacheRead, counts.CacheWrite} {
		if n > remaining {
			return true
		}
		remaining -= n
	}
	return false
}

// costOf returns the sum of each category's count times its rate, in units of
// 10^-12 USD, using exact big-integer arithmetic.
func costOf(counts llm.TokenCounts, r rates) Amount {
	pairs := [...]struct {
		count uint64
		rate  microUSDPerMTok
	}{
		{counts.Input, r.input},
		{counts.CacheRead, r.cacheRead},
		{counts.CacheWrite, r.cacheWrite},
		{counts.Output, r.output},
	}
	total := new(big.Int)
	for _, p := range pairs {
		count := new(big.Int).SetUint64(p.count)
		rate := new(big.Int).SetUint64(uint64(p.rate))
		total.Add(total, count.Mul(count, rate))
	}
	return Amount{pico: total}
}

// unknownCost returns an unknown stage cost with reason.
func unknownCost(reason UnknownReason) StageCost {
	return StageCost{reason: reason}
}
