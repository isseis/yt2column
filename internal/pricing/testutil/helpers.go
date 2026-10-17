//go:build test || integration

// Package pricingtestutil formats the estimated cost of a stage for the log
// lines of the integration tests that call a real LLM API. It is built both by
// the unit tests (`-tags test`), which check the formatting, and by the
// integration tests (`-tags integration`), which use it.
package pricingtestutil

import (
	"fmt"

	"github.com/isseis/yt2column/internal/pricing"
)

// costDecimals is how many decimal places the logged amount shows, matching
// the CLI cost line's precision.
const costDecimals = 6

// CostSummary renders one stage's estimated cost as a single line: the amount
// with six decimals and the rate basis for a known cost, or the reason for an
// unknown one. The words are the same spellings the CLI's cost line uses
// (architecture section 3.6), but the two are deliberately not tied by a
// guard: this log is a diagnostic, not a contract (architecture section 3.8),
// so a drift here never affects a user's output.
func CostSummary(cost pricing.StageCost) string {
	if amount, basis, known := cost.Known(); known {
		return fmt.Sprintf("known usd=%s %s", amount.Format(costDecimals), costBasisWord(basis))
	}
	reason, _ := cost.Unknown()
	return fmt.Sprintf("unknown reason=%s", costReasonWord(reason))
}

// costBasisWord returns the fixed word of a known cost's rate basis, or
// "invalid" for the zero value and a value outside the enumeration.
func costBasisWord(basis pricing.RateBasis) string {
	switch basis {
	case pricing.BasisFlat:
		return "flat"
	case pricing.BasisOffPeak:
		return "off-peak"
	case pricing.BasisPeak:
		return "peak"
	case pricing.BasisShortPrompt:
		return "short-prompt"
	case pricing.BasisLongPrompt:
		return "long-prompt"
	default:
		return "invalid"
	}
}

// costReasonWord returns the fixed word of an unknown cost's reason, or
// "invalid" for ReasonUnset, ReasonInvalid, and a value outside the
// enumeration.
func costReasonWord(reason pricing.UnknownReason) string {
	switch reason {
	case pricing.ReasonUsageUnknown:
		return "usage-unknown"
	case pricing.ReasonNoModel:
		return "no-model"
	case pricing.ReasonUnpricedModel:
		return "unpriced-model"
	default:
		return "invalid"
	}
}
