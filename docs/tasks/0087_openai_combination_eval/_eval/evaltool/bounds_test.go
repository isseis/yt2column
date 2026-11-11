package main

import (
	"errors"
	"strings"
	"testing"
)

// testBoundsInput is a small input whose bounds and estimates are computed by
// hand in the tests. Every input category of the three models is priced
// highest as cache write (sol 2.5, astra 12.5, opus 5 USD per million).
func testBoundsInput() boundsInput {
	return boundsInput{
		RunsPerConfig: 10,
		OutputCap:     map[string]uint64{providerOpenAI: 1000, providerClaude: 2000},
		FixedInput: map[string]map[string]uint64{
			"gpt-6-astra":     {stageGeneration: 1000, stageShorten: 500, stageReview: 800},
			"gpt-6.1-sol":     {stageGeneration: 1000, stageShorten: 500, stageReview: 800},
			"claude-opus-5-5": {stageShorten: 600, stageReview: 900},
		},
		ArticlePartMaxBytes: 300,
		EstimateTokens: map[string]map[string]measuredCall{
			"gpt-6.1-sol": {stageGeneration: {1000, 100}, stageShorten: {500, 100}, stageReview: {800, 50}},
			"gpt-6-astra": {stageGeneration: {1000, 100}, stageShorten: {500, 100}, stageReview: {800, 50}},
		},
		EstimateStageUSD: map[string]map[string]string{"claude-opus-5-5": {stageReview: "0.100000"}},
	}
}

// TestComputeBounds compares the bounds with hand-computed values: the same
// model's review adds the previous output limit, a different model's review
// adds the article's byte bound, a resumed config bounds a Claude shorten, and
// an input over 272,000 tokens takes the upper rate tier.
func TestComputeBounds(t *testing.T) {
	bounds, err := computeBounds(testBoundsInput(), mustConfigs(t))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]stageBound{}
	for _, b := range bounds {
		got[b.Config+" "+b.Stage] = b
	}
	for key, want := range map[string]struct {
		input uint64
		usd   string
	}{
		"C-sol generation":   {1000, "0.012500"}, // 1000*2.5 + 1000*10 per million
		"C-sol shorten":      {1500, "0.013750"}, // (500+1000)*2.5 + 1000*10
		"C-sol review":       {1800, "0.014500"}, // (800+1000)*2.5 + 1000*10
		"A-sol shorten":      {2600, "0.053000"}, // (600+2000)*5 + 2000*20
		"A-sol review":       {1100, "0.012750"}, // (800+300)*2.5 + 1000*10
		"B-astra generation": {1000, "0.062500"}, // 1000*12.5 + 1000*50
		"B-astra review":     {1200, "0.046000"}, // (900+300)*5 + 2000*20
	} {
		b, ok := got[key]
		if !ok || b.Input != want.input || b.USD.String() != want.usd {
			t.Errorf("%s = %+v, want input %d usd %s", key, b, want.input, want.usd)
		}
	}
	if _, ok := got["A-sol generation"]; ok {
		t.Errorf("a resumed config must not bound a generation it reuses")
	}
	rb, err := runBounds(boundsTable(bounds))
	if err != nil || rb["A-sol"].String() != "0.065750" {
		t.Errorf("A-sol run bound = %v, %v; want 0.053000 + 0.012750", rb["A-sol"], err)
	}

	long := testBoundsInput()
	long.FixedInput["gpt-6-astra"][stageGeneration] = 300000
	bounds, err = computeBounds(long, mustConfigs(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bounds {
		if b.Config == "C-astra" && b.Stage == stageGeneration && b.USD.String() != "7.575000" {
			t.Errorf("long-input bound = %s, want 300000*25 + 1000*75 per million = 7.575000", b.USD)
		}
	}
}

// TestComputeBoundsRejects stops on a model the rate table cannot price and
// on a stage without a fixed input.
func TestComputeBoundsRejects(t *testing.T) {
	typo, err := parseConfigTable(strings.NewReader("C-sol\t1\topenai/gpt-6.1-sole+medium\topenai/gpt-6.1-sole+medium\tfull\n"))
	if err != nil {
		t.Fatal(err)
	}
	in := testBoundsInput()
	in.FixedInput["gpt-6.1-sole"] = in.FixedInput["gpt-6.1-sol"]
	if _, err := computeBounds(in, typo); !errors.Is(err, errBounds) {
		t.Errorf("an unpriced model: err = %v", err)
	}
	in = testBoundsInput()
	delete(in.FixedInput["claude-opus-5-5"], stageShorten)
	if _, err := computeBounds(in, mustConfigs(t)); !errors.Is(err, errBounds) {
		t.Errorf("a missing fixed input: err = %v", err)
	}
}

// TestEstimates compares the estimate with hand-computed values: input at the
// cache-write rate, a measured average where no tokens are given, and the
// shorten surcharge only for full configs.
func TestEstimates(t *testing.T) {
	est, err := computeEstimates(testBoundsInput(), mustConfigs(t))
	if err != nil {
		t.Fatal(err)
	}
	tb := estimateTable(est)
	ts := []*table{tb}
	for row, want := range map[string][2]string{
		"C-sol":        {"0.060000", "0.022500"}, // 10*(0.0035+0.0025), 10*0.00225
		"A-sol":        {"0.025000", none},       // 10*0.0025
		"B-sol":        {"1.035000", "0.022500"}, // 10*(0.0035+0.1)
		"stage1+B-sol": {"", ""},
	} {
		if want[0] == "" {
			continue
		}
		if got := cell(t, ts, "estimate", "estimate_usd", row); got != want[0] {
			t.Errorf("%s estimate = %s, want %s", row, got, want[0])
		}
		if got := cell(t, ts, "estimate", "shorten_extra_usd", row); got != want[1] {
			t.Errorf("%s shorten extra = %s, want %s", row, got, want[1])
		}
	}
	// Stage 1: C-sol 0.06, C-astra 10*(0.0175+0.0125)=0.3, A-sol 0.025,
	// A-astra 10*0.0125=0.125; with B-sol 1.035.
	if got := cell(t, ts, "estimate", "estimate_usd", "stage1+B-sol"); got != "1.545000" {
		t.Errorf("stage1+B-sol = %s, want 1.545000", got)
	}
}

// TestRates derives each model's rates per million tokens from the rate table,
// with the OpenAI models' upper tier.
func TestRates(t *testing.T) {
	tb, err := ratesTable(mustConfigs(t))
	if err != nil {
		t.Fatal(err)
	}
	ts := []*table{tb}
	for _, c := range []struct{ model, tier, in, cr, cw, out string }{
		{"claude-opus-5-5", tierLow, "4.000000", "0.200000", "5.000000", "20.000000"},
		{"claude-opus-5-5", tierHigh, "4.000000", "0.200000", "5.000000", "20.000000"},
		{"gpt-6.1-sol", tierLow, "2.000000", "0.100000", "2.500000", "10.000000"},
		{"gpt-6.1-sol", tierHigh, "4.000000", "0.200000", "5.000000", "15.000000"},
		{"gpt-6-astra", tierHigh, "20.000000", "2.000000", "25.000000", "75.000000"},
	} {
		got := []string{
			cell(t, ts, "rates", "input", c.model, c.tier), cell(t, ts, "rates", "cache_read", c.model, c.tier),
			cell(t, ts, "rates", "cache_write", c.model, c.tier), cell(t, ts, "rates", "output", c.model, c.tier),
		}
		if want := []string{c.in, c.cr, c.cw, c.out}; strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s %s = %v, want %v", c.model, c.tier, got, want)
		}
	}
}
