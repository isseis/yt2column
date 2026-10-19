package publisher

import (
	"github.com/isseis/yt2column/internal/pricing"
	"github.com/isseis/yt2column/internal/writer"
)

// articleCostDecimals is how many decimal places an article's cost shows. The
// article reader needs only a rough figure, so it is coarser than the six
// decimals of the CLI summary.
const articleCostDecimals = 4

// articleCost returns the cost of the whole article (the stages this run
// called plus the ones it reused) in US dollars, and whether that total is
// known. It never guesses a value, so an unpriced model or an unknown usage
// yields false.
func articleCost(runUsage writer.RunUsage) (pricing.Amount, bool) {
	_, article := pricing.Totals(runUsage)
	return article.Amount()
}

// modelVersionOrNone returns version, or the "(none)" placeholder shown when a
// provider reports no version, for the model lines of the output and the
// review log.
func modelVersionOrNone(version string) string {
	if version == "" {
		return "(none)"
	}
	return version
}
