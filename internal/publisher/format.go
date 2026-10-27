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

// FormatArticleCost returns the article's cost as the --out file's
// estimated_cost_usd value and the Slack footnote amount, and whether it is
// known. When it is not known, usd is "" and each caller writes its own
// placeholder.
func FormatArticleCost(runUsage writer.RunUsage) (usd string, known bool) {
	amount, known := articleCost(runUsage)
	if !known {
		return "", false
	}
	return amount.Format(articleCostDecimals), true
}

// modelVersionOrNone returns version, or the "(none)" placeholder shown when a
// provider reports no version, for the model entries of the output and the
// review log.
func modelVersionOrNone(version string) string {
	if version == "" {
		return "(none)"
	}
	return version
}
