//go:build test

package artifact

import "github.com/isseis/yt2column/internal/llm"

// PricingContextSpellingsForTest returns the version 4 spelling of every
// llm.PricingContext, for the guard tests that compare the documents with the
// code.
func PricingContextSpellingsForTest() []string {
	return []string{
		pricingContextWire(llm.PricingContextUnspecified),
		pricingContextWire(llm.PricingContextStandard30m),
		pricingContextWire(llm.PricingContextUnverified),
	}
}
