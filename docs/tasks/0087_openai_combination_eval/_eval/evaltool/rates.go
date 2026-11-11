package main

import (
	"flag"
	"io"
	"slices"

	"github.com/isseis/yt2column/internal/llm"
)

// The probes rates uses. The OpenAI models change rates when the input
// exceeds 272,000 tokens (internal/pricing/rates.go), so the lower tier is
// probed with 100,000 tokens, scaled to a million, and the upper tier with a
// million tokens.
const (
	probeLowTokens  = 100_000
	probeLowScale   = 10
	probeHighTokens = 1_000_000
	tierLow         = "le272000"
	tierHigh        = "gt272000"
)

// tierRates are the four per-million-token rates of one tier.
type tierRates struct{ Input, CacheRead, CacheWrite, Output micro }

// probeRates derives the rates of model v from pricing.Estimate, so the rates
// the record shows are the ones the CLI priced with, not a copy.
func probeRates(v llmValue) (low, high tierRates, err error) {
	cost := func(c llm.TokenCounts) (micro, error) {
		a, err := priceCall(v, c)
		if err != nil {
			return 0, err
		}
		return ceilMicro(a)
	}
	diff := func(with, without llm.TokenCounts) (micro, error) {
		a, err := cost(with)
		if err != nil {
			return 0, err
		}
		b, err := cost(without)
		return a - b, err
	}
	var errs [8]error
	low.Input, errs[0] = cost(llm.TokenCounts{Input: probeLowTokens})
	low.CacheRead, errs[1] = cost(llm.TokenCounts{CacheRead: probeLowTokens})
	low.CacheWrite, errs[2] = cost(llm.TokenCounts{CacheWrite: probeLowTokens})
	low.Output, errs[3] = cost(llm.TokenCounts{Output: probeHighTokens})
	high.Input, errs[4] = cost(llm.TokenCounts{Input: probeHighTokens})
	high.CacheRead, errs[5] = cost(llm.TokenCounts{CacheRead: probeHighTokens})
	high.CacheWrite, errs[6] = cost(llm.TokenCounts{CacheWrite: probeHighTokens})
	high.Output, errs[7] = diff(llm.TokenCounts{Input: probeHighTokens, Output: probeHighTokens}, llm.TokenCounts{Input: probeHighTokens})
	for _, e := range errs {
		if e != nil {
			return tierRates{}, tierRates{}, e
		}
	}
	low.Input *= probeLowScale
	low.CacheRead *= probeLowScale
	low.CacheWrite *= probeLowScale
	return low, high, nil
}

// ratesTable lists the rates of every model the configs use, both tiers, in
// USD per million tokens. A model with one rate set shows it in both tiers.
func ratesTable(configs configTable) (*table, error) {
	t := newTable("rates", keysArticle, "model", "tier", "input", "cache_read", "cache_write", "output")
	var seen []string
	for _, c := range configs {
		for _, v := range []llmValue{c.LLM, c.ReviewLLM} {
			if slices.Contains(seen, v.Model) {
				continue
			}
			seen = append(seen, v.Model)
			low, high, err := probeRates(v)
			if err != nil {
				return nil, err
			}
			for _, tier := range []struct {
				name string
				r    tierRates
			}{{tierLow, low}, {tierHigh, high}} {
				t.add(v.Model, tier.name, tier.r.Input.String(), tier.r.CacheRead.String(), tier.r.CacheWrite.String(), tier.r.Output.String())
			}
		}
	}
	return t, nil
}

// runRates implements "rates".
func runRates(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("rates", flag.ContinueOnError)
	runsh := fs.String("runsh", "", "the run script, which prints the config table")
	if err := fs.Parse(args); err != nil {
		return err
	}
	configs, err := loadConfigTable(*runsh)
	if err != nil {
		return err
	}
	t, err := ratesTable(configs)
	if err != nil {
		return err
	}
	return writeTables(stdout, t)
}
