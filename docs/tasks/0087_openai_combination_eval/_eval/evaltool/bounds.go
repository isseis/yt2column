package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/pricing"
)

// boundsInput is _eval/bounds-input.json: the measured token counts and the
// limits the estimate and the bounds are computed from (design section 3.5). Every
// value's source is in Sources.
type boundsInput struct {
	Sources map[string]string `json:"sources"`
	// RunsPerConfig is the number of articles one config runs.
	RunsPerConfig int `json:"runs_per_config"`
	// OutputCap is each provider adapter's default output limit, in tokens.
	OutputCap map[string]uint64 `json:"output_cap"`
	// FixedInput is the measured input of a stage, by model and stage, in
	// tokens: the template and material part, with the article part the
	// measurement included left in.
	FixedInput map[string]map[string]uint64 `json:"fixed_input"`
	// ArticlePartMaxBytes bounds the UTF-8 bytes of an article's title and
	// body handed to a review whose model differs from the generation's.
	ArticlePartMaxBytes uint64 `json:"article_part_max_bytes"`
	// EstimateTokens is the measured input and output of a stage, by model
	// and stage, for the estimate.
	EstimateTokens map[string]map[string]measuredCall `json:"estimate_tokens"`
	// EstimateStageUSD is a measured average cost of a stage, by model and
	// stage, used for the estimate where no token measurement exists.
	EstimateStageUSD map[string]map[string]string `json:"estimate_stage_usd"`
}

// measuredCall is one measured call: its input and output tokens.
type measuredCall struct {
	Input  uint64 `json:"input"`
	Output uint64 `json:"output"`
}

// errBounds reports bounds input or a bounds table that cannot price a config.
var errBounds = errors.New("cannot compute the bounds")

// loadBoundsInput reads bounds-input.json, refusing unknown keys.
func loadBoundsInput(path string) (boundsInput, error) {
	f, err := os.Open(path) //nolint:gosec // the path comes from the operator's flag
	if err != nil {
		return boundsInput{}, err
	}
	defer func() { _ = f.Close() }()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var in boundsInput
	if err := dec.Decode(&in); err != nil {
		return boundsInput{}, fmt.Errorf("%s: %w", path, err)
	}
	if in.RunsPerConfig <= 0 || in.ArticlePartMaxBytes == 0 {
		return boundsInput{}, fmt.Errorf("%w: %s: runs_per_config and article_part_max_bytes must be positive", errBounds, path)
	}
	return in, nil
}

// priceCall returns the exact cost of one call with counts on model, which the
// provider of v serves. It fails when the rate table cannot price it, which
// includes a model ID the table does not hold (a typo in the config table).
func priceCall(v llmValue, counts llm.TokenCounts) (pricing.Amount, error) {
	ctx, err := v.pricingContext()
	if err != nil {
		return pricing.Amount{}, err
	}
	usage, err := llm.KnownUsageWithPricingContext(counts, ctx)
	if err != nil {
		return pricing.Amount{}, err
	}
	amount, _, known := pricing.Estimate(v.Model, usage, time.Time{}).Known()
	if !known {
		reason, _ := pricing.Estimate(v.Model, usage, time.Time{}).Unknown()
		return pricing.Amount{}, fmt.Errorf("%w: model %s is not priced (reason %d)", errBounds, v.Model, reason)
	}
	return amount, nil
}

// maxInputCost returns the cost of a call with the given input and output,
// with the whole input counted in whichever input category the rate table
// prices highest, so the bound holds however the provider splits the input.
func maxInputCost(v llmValue, input, output uint64) (micro, error) {
	var best micro
	for _, counts := range []llm.TokenCounts{
		{Input: input, Output: output},
		{CacheRead: input, Output: output},
		{CacheWrite: input, Output: output},
	} {
		amount, err := priceCall(v, counts)
		if err != nil {
			return 0, err
		}
		m, err := ceilMicro(amount)
		if err != nil {
			return 0, err
		}
		best = max(best, m)
	}
	return best, nil
}

// stageBound is the bound of one stage of one run of a config.
type stageBound struct {
	Config, Stage, Model string
	Input, Output        uint64
	USD                  micro
}

// computeBounds returns the bound of every stage each config may call (design
// section 3.5): the output bound is the adapter's limit; the input bound is the
// stage's fixed input plus the previous stage's output bound, or, when the
// review's model differs from the generation's, plus the article part's byte
// bound (one token is assumed to cover at least one byte).
func computeBounds(in boundsInput, configs configTable) ([]stageBound, error) {
	var out []stageBound
	for _, c := range configs {
		for _, s := range c.callableStages() {
			v := c.stageModel(s)
			fixed, ok := in.FixedInput[v.Model][s]
			if !ok {
				return nil, fmt.Errorf("%w: no fixed input for %s %s", errBounds, v.Model, s)
			}
			outCap, ok := in.OutputCap[v.Provider]
			if !ok {
				return nil, fmt.Errorf("%w: no output limit for %s", errBounds, v.Provider)
			}
			input := fixed
			switch {
			case s == stageGeneration:
			case s == stageShorten || (c.ReviewLLM.Provider == c.LLM.Provider && c.ReviewLLM.Model == c.LLM.Model):
				prevCap, ok := in.OutputCap[c.LLM.Provider]
				if !ok {
					return nil, fmt.Errorf("%w: no output limit for %s", errBounds, c.LLM.Provider)
				}
				input += prevCap
			default:
				input += in.ArticlePartMaxBytes
			}
			usd, err := maxInputCost(v, input, outCap)
			if err != nil {
				return nil, err
			}
			out = append(out, stageBound{Config: c.ID, Stage: s, Model: v.Model, Input: input, Output: outCap, USD: usd})
		}
	}
	return out, nil
}

// boundsTable renders the bounds as the "bounds" table that launch reads.
func boundsTable(bounds []stageBound) *table {
	t := newTable("bounds", keysArticle, "config", "stage", "model", "input_bound", "output_bound", "usd")
	for _, b := range bounds {
		t.add(b.Config, b.Stage, b.Model, strconv.FormatUint(b.Input, 10), strconv.FormatUint(b.Output, 10), b.USD.String())
	}
	return t
}

// runBounds returns each config's bound for one run: the sum of its stage
// bounds in the bounds table. The same function serves launch's reservation
// and aggregate's spend, so both count a run the same way.
func runBounds(t *table) (map[string]micro, error) {
	if t.Name != "bounds" || len(t.Columns) != 6 { //nolint:mnd // the six columns of boundsTable
		return nil, fmt.Errorf("%w: not a bounds table", errBounds)
	}
	out := map[string]micro{}
	for _, row := range t.Rows {
		usd, err := parseMicro(row[5])
		if err != nil {
			return nil, err
		}
		out[row[0]] += usd
	}
	return out, nil
}

// loadBoundsTable reads the bounds table file estimate wrote.
func loadBoundsTable(path string) (*table, error) {
	tables, err := readParsed(path, readTables)
	if err != nil {
		return nil, err
	}
	t, ok := findTable(tables, "bounds")
	if !ok || len(tables) != 1 {
		return nil, fmt.Errorf("%w: %s holds no single bounds table", errBounds, path)
	}
	return t, nil
}

// estimateCall returns the estimated cost of one call of stage on model: the
// measured tokens, with the input counted at the cache-write rate and no cache
// read (design section 3.5), or a measured average cost where no tokens are given.
func estimateCall(in boundsInput, v llmValue, stage string) (micro, error) {
	if call, ok := in.EstimateTokens[v.Model][stage]; ok {
		amount, err := priceCall(v, llm.TokenCounts{CacheWrite: call.Input, Output: call.Output})
		if err != nil {
			return 0, err
		}
		return ceilMicro(amount)
	}
	if usd, ok := in.EstimateStageUSD[v.Model][stage]; ok {
		return parseMicro(usd)
	}
	return 0, fmt.Errorf("%w: no estimate input for %s %s", errBounds, v.Model, stage)
}

// configEstimate is one config's estimate for its runs: the stages other than
// shorten, and separately the shorten stage as if it ran on every article
// (not counted for a resumed config, whose shorten is expected not to run).
type configEstimate struct {
	Config       string
	Stage        int
	Base         micro
	ShortenExtra micro
	HasExtra     bool
}

// computeEstimates returns each config's estimate.
func computeEstimates(in boundsInput, configs configTable) ([]configEstimate, error) {
	var out []configEstimate
	n := micro(in.RunsPerConfig)
	for _, c := range configs {
		e := configEstimate{Config: c.ID, Stage: c.Stage}
		for _, s := range c.callableStages() {
			if s == stageShorten {
				continue
			}
			usd, err := estimateCall(in, c.stageModel(s), s)
			if err != nil {
				return nil, err
			}
			e.Base += usd * n
		}
		if c.Mode == modeFull {
			usd, err := estimateCall(in, c.LLM, stageShorten)
			if err != nil {
				return nil, err
			}
			e.ShortenExtra, e.HasExtra = usd*n, true
		}
		out = append(out, e)
	}
	return out, nil
}

// estimateTable renders the estimates, followed by the sums the approval shows:
// stage 1 with each stage-2 config, and every config.
func estimateTable(estimates []configEstimate) *table {
	t := newTable("estimate", 1, "row", "stage", "estimate_usd", "shorten_extra_usd")
	var stage1, stage1Extra, all, allExtra micro
	for _, e := range estimates {
		extra := none
		if e.HasExtra {
			extra = e.ShortenExtra.String()
		}
		t.add(e.Config, strconv.Itoa(e.Stage), e.Base.String(), extra)
		all += e.Base
		allExtra += e.ShortenExtra
		if e.Stage == 1 {
			stage1 += e.Base
			stage1Extra += e.ShortenExtra
		}
	}
	for _, e := range estimates {
		if e.Stage == 2 { //nolint:mnd // stage 2
			t.add("stage1+"+e.Config, none, (stage1 + e.Base).String(), (stage1Extra + e.ShortenExtra).String())
		}
	}
	t.add("all", none, all.String(), allExtra.String())
	return t
}

// runEstimate implements "estimate": it prints the estimate and bounds tables
// and, with -write-bounds, writes the bounds table for launch.
func runEstimate(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("estimate", flag.ContinueOnError)
	inputPath := fs.String("input", "", "bounds-input.json")
	runsh := fs.String("runsh", "", "the run script, which prints the config table")
	writeBounds := fs.String("write-bounds", "", "file to write the bounds table to (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	in, err := loadBoundsInput(*inputPath)
	if err != nil {
		return err
	}
	configs, err := loadConfigTable(*runsh)
	if err != nil {
		return err
	}
	bounds, estimates, err := estimateAndBounds(in, configs)
	if err != nil {
		return err
	}
	if *writeBounds != "" {
		if err := writeTableFile(*writeBounds, bounds); err != nil {
			return err
		}
	}
	return writeTables(stdout, estimates, bounds)
}

// estimateAndBounds computes both tables estimate prints.
func estimateAndBounds(in boundsInput, configs configTable) (bounds, estimates *table, err error) {
	b, err := computeBounds(in, configs)
	if err != nil {
		return nil, nil, err
	}
	e, err := computeEstimates(in, configs)
	if err != nil {
		return nil, nil, err
	}
	return boundsTable(b), estimateTable(e), nil
}

// writeTableFile writes one table to path.
func writeTableFile(path string, t *table) error {
	f, err := os.Create(path) //nolint:gosec // the path comes from the operator's flag
	if err != nil {
		return err
	}
	if err := writeTables(f, t); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
