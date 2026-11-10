//go:build test

package artifact

import "github.com/isseis/yt2column/internal/llm"

// FormatVersionsForTest returns the format version Marshal writes and the
// older version Parse also reads, for the guard tests that compare the
// documents with the code.
func FormatVersionsForTest() (written, alsoRead int) {
	return artifactVersion, artifactVersion3
}

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
