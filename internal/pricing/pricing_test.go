//go:build test

package pricing

import (
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
)

// The formats below are the published rates applied by hand to the given token
// counts. The rate of each category is per million tokens, so a count c at a
// rate r contributes c*r/1e6 USD.

// TestEstimateCategories checks the cost of one stage of each model from four
// distinct, non-zero token counts, so a category mix-up changes the value.
func TestEstimateCategories(t *testing.T) {
	// Saturday noon UTC and Monday 01:00 UTC: deepseek-flash is off-peak and
	// peak respectively.
	offPeak := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	peak := time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)
	broad := llm.KnownUsage(llm.TokenCounts{Input: 1000000, CacheRead: 2000000, CacheWrite: 3000000, Output: 4000000})
	cases := []struct {
		name  string
		model string
		usage llm.Usage
		at    time.Time
		// deepseek-flash off-peak: 1000000*0.15 + 2000000*0.003 + 3000000*0 + 4000000*0.6 = 2.556
		// deepseek-flash peak: 1000000*0.3 + 2000000*0.006 + 3000000*0 + 4000000*1.2 = 5.112
		// claude-opus-5-5: 1000000*4 + 2000000*0.2 + 3000000*5 + 4000000*20 = 99.4
		// claude-sonnet-5-5: 1000000*2 + 2000000*0.1 + 3000000*2.5 + 4000000*10 = 49.7
		// claude-haiku-5-5 (short prompt): 1000*0.1 + 2000*0.01 + 3000*0.125 + 4000000*0.5 = 2.000495
		// claude-haiku-5-5 (long prompt, input over the threshold): 200000*0.5 + 300000*0.05 + 400000*0.625 + 500000*2.5 = 1.615
		want string
	}{
		{"deepseek-flash off-peak", "deepseek-flash", broad, offPeak, "2.556000"},
		{"deepseek-flash peak", "deepseek-flash", broad, peak, "5.112000"},
		{"claude-opus-5-5", "claude-opus-5-5", broad, offPeak, "99.400000"},
		{"claude-sonnet-5-5", "claude-sonnet-5-5", broad, offPeak, "49.700000"},
		{"claude-haiku-5-5 short prompt", "claude-haiku-5-5", llm.KnownUsage(llm.TokenCounts{Input: 1000, CacheRead: 2000, CacheWrite: 3000, Output: 4000000}), offPeak, "2.000495"},
		{"claude-haiku-5-5 long prompt", "claude-haiku-5-5", llm.KnownUsage(llm.TokenCounts{Input: 200000, CacheRead: 300000, CacheWrite: 400000, Output: 500000}), offPeak, "1.615000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			amount, _, ok := Estimate(tc.model, tc.usage, tc.at).Known()
			if !ok {
				t.Fatalf("Estimate(%s) is unknown, want known", tc.model)
			}
			if got := amount.Format(6); got != tc.want {
				t.Errorf("Estimate(%s) = %s USD, want %s", tc.model, got, tc.want)
			}
		})
	}
}

// TestEstimatePeakWindow checks the peak/off-peak boundary of deepseek-flash.
// The clock is given explicitly, and the judgment is made in UTC whatever zone
// the value carries.
func TestEstimatePeakWindow(t *testing.T) {
	usage := llm.KnownUsage(llm.TokenCounts{Input: 1000, Output: 100})
	monday := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	jst := time.FixedZone("JST", 9*60*60)
	pst := time.FixedZone("PST", -7*60*60)
	cases := []struct {
		name string
		at   time.Time
		want RateBasis
	}{
		{"monday 00:59:59", monday.Add(59*time.Minute + 59*time.Second), BasisOffPeak},
		{"monday 01:00:00", monday.Add(60 * time.Minute), BasisPeak},
		{"monday 03:59:59", monday.Add(3*time.Hour + 59*time.Minute + 59*time.Second), BasisPeak},
		{"monday 04:00:00", monday.Add(4 * time.Hour), BasisOffPeak},
		{"monday 05:59:59", monday.Add(5*time.Hour + 59*time.Minute + 59*time.Second), BasisOffPeak},
		{"monday 06:00:00", monday.Add(6 * time.Hour), BasisPeak},
		{"monday 09:59:59", monday.Add(9*time.Hour + 59*time.Minute + 59*time.Second), BasisPeak},
		{"monday 10:00:00", monday.Add(10 * time.Hour), BasisOffPeak},
		{"saturday 01:00:00", time.Date(2026, 10, 17, 1, 0, 0, 0, time.UTC), BasisOffPeak},
		{"sunday 01:00:00", time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC), BasisOffPeak},
		// The instant is Friday 22:00 UTC, off-peak; its JST wall clock reads
		// Saturday 07:00, inside the peak window.
		{"instant read in JST sits on saturday", time.Date(2026, 10, 17, 7, 0, 0, 0, jst), BasisOffPeak},
		// The instant is Monday 06:00 UTC, peak; its PST wall clock reads
		// Sunday 23:00, off-peak.
		{"instant read in PST sits on sunday", time.Date(2026, 10, 18, 23, 0, 0, 0, pst), BasisPeak},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, basis, ok := Estimate("deepseek-flash", usage, tc.at).Known()
			if !ok {
				t.Fatalf("Estimate is unknown, want known")
			}
			if basis != tc.want {
				t.Errorf("at %s basis = %v, want %v", tc.at, basis, tc.want)
			}
		})
	}
}

// TestEstimateIgnoresCallTime checks that a Claude stage's cost and basis do
// not depend on the call time.
func TestEstimateIgnoresCallTime(t *testing.T) {
	usage := llm.KnownUsage(llm.TokenCounts{Input: 10, CacheRead: 20, CacheWrite: 30, Output: 40})
	times := []time.Time{
		time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC), // peak window for a time-of-day model
		time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC), // off-peak
		{}, // zero
	}
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"} {
		t.Run(model, func(t *testing.T) {
			var first string
			var firstBasis RateBasis
			for i, at := range times {
				amount, basis, ok := Estimate(model, usage, at).Known()
				if !ok {
					t.Fatalf("Estimate(%s) at %s is unknown, want known", model, at)
				}
				if i == 0 {
					first, firstBasis = amount.Format(6), basis
					continue
				}
				if got := amount.Format(6); got != first {
					t.Errorf("Estimate(%s) at %s = %s USD, want %s", model, at, got, first)
				}
				if basis != firstBasis {
					t.Errorf("Estimate(%s) at %s basis = %v, want %v", model, at, basis, firstBasis)
				}
			}
		})
	}
}

// TestEstimatePromptTier checks the prompt-length threshold of
// claude-haiku-5-5, that the sum includes all three input categories, that the
// sum does not overflow, and that the other models ignore the threshold.
func TestEstimatePromptTier(t *testing.T) {
	at := time.Date(2026, 10, 19, 4, 0, 0, 0, time.UTC)
	usage := func(input, cacheRead, cacheWrite uint64) llm.Usage {
		return llm.KnownUsage(llm.TokenCounts{Input: input, CacheRead: cacheRead, CacheWrite: cacheWrite})
	}
	boundary := []struct {
		name string
		got  llm.Usage
		want RateBasis
	}{
		{"sum at the threshold stays short", usage(100000, 0, 0), BasisShortPrompt},
		{"sum one over the threshold is long", usage(100001, 0, 0), BasisLongPrompt},
		{"cache read alone over the threshold is long", usage(0, 100001, 0), BasisLongPrompt},
		{"cache write alone over the threshold is long", usage(0, 0, 100001), BasisLongPrompt},
	}
	for _, tc := range boundary {
		t.Run(tc.name, func(t *testing.T) {
			_, basis, ok := Estimate("claude-haiku-5-5", tc.got, at).Known()
			if !ok {
				t.Fatalf("Estimate is unknown, want known")
			}
			if basis != tc.want {
				t.Errorf("basis = %v, want %v", basis, tc.want)
			}
		})
	}

	t.Run("sum does not overflow uint64", func(t *testing.T) {
		// A naive sum wraps to 1, which is not over the threshold. The value
		// is the long-prompt rate set: 18446744073709551615*0.5 + 2*0.05.
		const maxUint64 = ^uint64(0)
		amount, basis, ok := Estimate("claude-haiku-5-5", usage(maxUint64, 2, 0), at).Known()
		if !ok {
			t.Fatalf("Estimate is unknown, want known")
		}
		if basis != BasisLongPrompt {
			t.Fatalf("basis = %v, want %v", basis, BasisLongPrompt)
		}
		if got, want := amount.Format(6), "9223372036854.775808"; got != want {
			t.Errorf("amount = %s USD, want %s", got, want)
		}
	})

	t.Run("other models ignore the threshold", func(t *testing.T) {
		cases := []struct {
			model string
			at    time.Time
			want  RateBasis
		}{
			{"deepseek-flash", at, BasisOffPeak},
			{"claude-opus-5-5", at, BasisFlat},
			{"claude-sonnet-5-5", at, BasisFlat},
		}
		for _, tc := range cases {
			_, basis, ok := Estimate(tc.model, usage(100001, 0, 0), tc.at).Known()
			if !ok {
				t.Fatalf("Estimate(%s) is unknown, want known", tc.model)
			}
			if basis != tc.want {
				t.Errorf("Estimate(%s) basis = %v, want %v", tc.model, basis, tc.want)
			}
		}
	})
}

// TestEstimateLargeCounts checks that the counts near the top of the integer
// ranges do not overflow, with the value computed by hand from the published
// off-peak rates of deepseek-flash (0.15 + 0.003 + 0 + 0.6 = 0.753 USD per
// million tokens, times four equal counts).
func TestEstimateLargeCounts(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) // off-peak
	cases := []struct {
		name  string
		count uint64
		want  string
	}{
		{"int64 max", 9223372036854775807, "6945199143751.646183"},
		{"uint64 max", ^uint64(0), "13890398287503.292366"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := llm.KnownUsage(llm.TokenCounts{Input: tc.count, CacheRead: tc.count, CacheWrite: tc.count, Output: tc.count})
			amount, _, ok := Estimate("deepseek-flash", c, at).Known()
			if !ok {
				t.Fatalf("Estimate is unknown, want known")
			}
			if got := amount.Format(6); got != tc.want {
				t.Errorf("amount = %s USD, want %s", got, tc.want)
			}
		})
	}
}

// TestEstimateUnknown checks every reason a stage cost is unknown, and the
// exact model-name matching that produces the unpriced-model reason.
func TestEstimateUnknown(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	usage := llm.KnownUsage(llm.TokenCounts{Input: 1})
	unknown := llm.UnknownUsage(llm.RuleNotRead, "")
	cases := []struct {
		name  string
		model string
		usage llm.Usage
		at    time.Time
		want  UnknownReason
	}{
		{"unknown usage", "deepseek-flash", unknown, at, ReasonUsageUnknown},
		{"unknown usage before a missing model", "", unknown, at, ReasonUsageUnknown},
		{"missing model", "", usage, at, ReasonNoModel},
		{"unpriced deepseek", "deepseek-v4-pro", usage, at, ReasonUnpricedModel},
		{"unpriced claude", "claude-fable-5-1", usage, at, ReasonUnpricedModel},
		{"unpriced other provider", "gpt-5", usage, at, ReasonUnpricedModel},
		{"a differently cased name is unpriced", "DeepSeek-Flash", usage, at, ReasonUnpricedModel},
		{"a suffixed name is unpriced", "deepseek-flash-latest", usage, at, ReasonUnpricedModel},
		{"a leading space is unpriced", " deepseek-flash", usage, at, ReasonUnpricedModel},
		{"a trailing space is unpriced", "deepseek-flash ", usage, at, ReasonUnpricedModel},
		{"a time-of-day model without a call time", "deepseek-flash", usage, time.Time{}, ReasonInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cost := Estimate(tc.model, tc.usage, tc.at)
			if _, _, ok := cost.Known(); ok {
				t.Fatalf("Estimate(%q) is known, want unknown", tc.model)
			}
			reason, ok := cost.Unknown()
			if !ok {
				t.Fatalf("Estimate(%q) reports known and unknown", tc.model)
			}
			if reason != tc.want {
				t.Errorf("Estimate(%q) reason = %v, want %v", tc.model, reason, tc.want)
			}
		})
	}
}

// TestEstimateUnknownScheme checks that a rate set outside the enumeration
// fails closed with ReasonInvalid.
func TestEstimateUnknownScheme(t *testing.T) {
	counts := llm.TokenCounts{Input: 1}
	cost := estimateEntry(modelRates{scheme: rateScheme(99)}, counts, time.Time{})
	reason, ok := cost.Unknown()
	if !ok || reason != ReasonInvalid {
		t.Errorf("estimateEntry with an unknown scheme = %v, want unknown with %v", reason, ReasonInvalid)
	}
}

// TestZeroValuesAreUnknown checks that the zero StageCost and Total read as
// unknown, never as 0 USD.
func TestZeroValuesAreUnknown(t *testing.T) {
	var cost StageCost
	if _, _, ok := cost.Known(); ok {
		t.Error("the zero StageCost is known, want unknown")
	}
	if reason, ok := cost.Unknown(); !ok || reason != ReasonUnset {
		t.Errorf("the zero StageCost reason = %v (unknown %v), want %v", reason, ok, ReasonUnset)
	}
	var total Total
	if _, ok := total.Amount(); ok {
		t.Error("the zero Total is known, want unknown")
	}
}

// TestTotals checks which stages each total includes and how an unknown stage
// propagates. Each known stage is one claude-opus-5-5 input token, worth
// 4,000,000 units of 10^-12 USD, or 0.000004 USD.
func TestTotals(t *testing.T) {
	type stageSpec struct {
		origin writer.UsageOrigin
		known  bool
	}
	run := stageSpec{writer.OriginRun, true}
	reused := stageSpec{writer.OriginReused, true}
	reusedUnknown := stageSpec{writer.OriginReused, false}
	runUnknown := stageSpec{writer.OriginRun, false}
	none := stageSpec{writer.OriginNone, false}
	cases := []struct {
		name                     string
		gen, short, review       stageSpec
		wantThisRun, wantArticle string // "" means unknown
	}{
		{"all three ran", run, run, run, "0.000012", "0.000012"},
		{"after generation", reused, run, run, "0.000008", "0.000012"},
		{"after shorten", reused, reused, run, "0.000004", "0.000012"},
		{"after review", reused, reused, reused, "0.000000", "0.000012"},
		{"shorten skipped", run, none, run, "0.000008", "0.000008"},
		{"only a reused stage is unknown", run, reusedUnknown, none, "0.000004", ""},
		{"no stage has an origin", none, none, none, "0.000000", ""},
		{"a run stage is unknown", runUnknown, none, none, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			thisRun, article := Totals(writer.RunUsage{
				Generation: stageUsage(tc.gen.origin, tc.gen.known),
				Shorten:    stageUsage(tc.short.origin, tc.short.known),
				Review:     stageUsage(tc.review.origin, tc.review.known),
			})
			checkTotal(t, "this run", thisRun, tc.wantThisRun)
			checkTotal(t, "article", article, tc.wantArticle)
		})
	}
}

// stageUsage builds one stage of a run whose model is priced as a flat Claude
// model, so each known stage costs 0.000004 USD.
func stageUsage(origin writer.UsageOrigin, known bool) writer.StageUsage {
	if origin == writer.OriginNone {
		return writer.StageUsage{}
	}
	usage := llm.KnownUsage(llm.TokenCounts{Input: 1})
	if !known {
		usage = llm.UnknownUsage(llm.RuleNotRead, "")
	}
	return writer.StageUsage{Origin: origin, Model: "claude-opus-5-5", Usage: usage}
}

// checkTotal asserts a total is unknown when want is "", or known with the
// exact 6-place amount otherwise.
func checkTotal(t *testing.T, name string, total Total, want string) {
	t.Helper()
	amount, ok := total.Amount()
	if want == "" {
		if ok {
			t.Errorf("%s total is known (%s USD), want unknown", name, amount.Format(6))
		}
		return
	}
	if !ok {
		t.Fatalf("%s total is unknown, want %s USD", name, want)
	}
	if got := amount.Format(6); got != want {
		t.Errorf("%s total = %s USD, want %s", name, got, want)
	}
}
