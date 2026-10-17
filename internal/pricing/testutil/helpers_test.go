//go:build test

package pricingtestutil

import (
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/pricing"
)

func TestCostSummary(t *testing.T) {
	usage := llm.KnownUsage(llm.TokenCounts{Input: 100, Output: 10})
	// Monday 01:00 UTC is inside deepseek-flash's peak window.
	peak := time.Date(2026, time.October, 19, 1, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		cost pricing.StageCost
		want string
	}{
		{"flat", pricing.Estimate("claude-opus-5-5", usage, time.Time{}), "known usd=0.000600 flat"},
		{"peak", pricing.Estimate("deepseek-flash", usage, peak), "known usd=0.000042 peak"},
		{"long prompt", pricing.Estimate("claude-haiku-5-5", llm.KnownUsage(llm.TokenCounts{Input: 100001}), time.Time{}), "known usd=0.050001 long-prompt"},
		{"unknown usage", pricing.Estimate("claude-opus-5-5", llm.UnknownUsage(llm.RuleNotRead, ""), time.Time{}), "unknown reason=usage-unknown"},
		{"no model", pricing.Estimate("", usage, time.Time{}), "unknown reason=no-model"},
		{"unpriced model", pricing.Estimate("gpt-5", usage, time.Time{}), "unknown reason=unpriced-model"},
		{"zero value", pricing.StageCost{}, "unknown reason=invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CostSummary(tc.cost); got != tc.want {
				t.Errorf("CostSummary = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("word fallbacks", func(t *testing.T) {
		if got := costBasisWord(pricing.BasisUnset); got != "invalid" {
			t.Errorf("costBasisWord(BasisUnset) = %q, want invalid", got)
		}
		if got := costBasisWord(pricing.RateBasis(99)); got != "invalid" {
			t.Errorf("costBasisWord(99) = %q, want invalid", got)
		}
		if got := costReasonWord(pricing.ReasonUnset); got != "invalid" {
			t.Errorf("costReasonWord(ReasonUnset) = %q, want invalid", got)
		}
		if got := costReasonWord(pricing.ReasonInvalid); got != "invalid" {
			t.Errorf("costReasonWord(ReasonInvalid) = %q, want invalid", got)
		}
		if got := costReasonWord(pricing.UnknownReason(99)); got != "invalid" {
			t.Errorf("costReasonWord(99) = %q, want invalid", got)
		}
	})
}
