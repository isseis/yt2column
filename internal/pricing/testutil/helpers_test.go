//go:build test

package pricingtestutil

import (
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/pricing"
)

func TestCostSummary(t *testing.T) {
	// The expected amounts are hand-calculated from the rate table: a rate is
	// microUSD per million tokens, which is picoUSD per token, so the amount is
	// the sum of tokens times rate, rendered with six decimals.
	usage := llm.KnownUsage(llm.TokenCounts{Input: 100, Output: 10})
	// Monday 01:00 UTC is inside deepseek-flash's peak window; 04:30 UTC is
	// outside both of its ranges.
	peak := time.Date(2026, time.October, 19, 1, 0, 0, 0, time.UTC)
	offPeak := time.Date(2026, time.October, 19, 4, 30, 0, 0, time.UTC)
	cases := []struct {
		name string
		cost pricing.StageCost
		want string
	}{
		// opus: 100*4000000 + 10*20000000 = 600,000,000 picoUSD.
		{"flat", pricing.Estimate("claude-opus-5-5", usage, time.Time{}), "known usd=0.000600 flat"},
		// peak: 100*300000 + 10*1200000 = 42,000,000 picoUSD.
		{"peak", pricing.Estimate("deepseek-flash", usage, peak), "known usd=0.000042 peak"},
		// off-peak: 100*150000 + 10*600000 = 21,000,000 picoUSD.
		{"off-peak", pricing.Estimate("deepseek-flash", usage, offPeak), "known usd=0.000021 off-peak"},
		// haiku short (100,000 tokens is at the threshold): 100000*100000.
		{"short prompt", pricing.Estimate("claude-haiku-5-5", llm.KnownUsage(llm.TokenCounts{Input: 100000}), time.Time{}), "known usd=0.010000 short-prompt"},
		// haiku long (100,001 tokens is over the threshold): 100001*500000.
		{"long prompt", pricing.Estimate("claude-haiku-5-5", llm.KnownUsage(llm.TokenCounts{Input: 100001}), time.Time{}), "known usd=0.050001 long-prompt"},
		{"known zero", pricing.Estimate("claude-opus-5-5", llm.KnownUsage(llm.TokenCounts{}), time.Time{}), "known usd=0.000000 flat"},
		{"unknown usage", pricing.Estimate("claude-opus-5-5", llm.UnknownUsage(llm.RuleNotRead, ""), time.Time{}), "unknown reason=usage-unknown"},
		{"no model", pricing.Estimate("", usage, time.Time{}), "unknown reason=no-model"},
		{"unpriced model", pricing.Estimate("gpt-5", usage, time.Time{}), "unknown reason=unpriced-model"},
		// A time-of-day model with a zero call time breaks an invariant.
		{"invalid", pricing.Estimate("deepseek-flash", usage, time.Time{}), "unknown reason=invalid"},
		{"zero value", pricing.StageCost{}, "unknown reason=invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CostSummary(tc.cost); got != tc.want {
				t.Errorf("CostSummary = %q, want %q", got, tc.want)
			}
		})
	}
}
