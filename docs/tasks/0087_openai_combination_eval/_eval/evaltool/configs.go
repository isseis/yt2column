package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/llm"
)

// The modes of a config: a full run calls every stage; a resumed run reuses
// B3's generation (and, for one article, its shorten) and calls the rest.
const (
	modeFull   = "full"
	modeResume = "resume"
)

// The providers a config's LLM value can name.
const (
	providerClaude = "claude"
	providerOpenAI = "openai"
)

// errConfigTable reports a config table that does not match its form.
var errConfigTable = errors.New("config table does not match its form")

// llmValue is one parsed YT2COLUMN_LLM value: "<provider>/<model>+<effort>".
type llmValue struct {
	Raw, Provider, Model, Effort string
}

// llmValueRE is the form of an LLM value the evaluation uses.
var llmValueRE = regexp.MustCompile(`^(claude|openai)/([A-Za-z0-9._-]+)\+([a-z]+)$`)

// parseLLMValue reads one LLM value of the config table.
func parseLLMValue(s string) (llmValue, error) {
	m := llmValueRE.FindStringSubmatch(s)
	if m == nil {
		return llmValue{}, fmt.Errorf("%w: LLM value %q", errConfigTable, s)
	}
	return llmValue{Raw: s, Provider: m[1], Model: m[2], Effort: m[3]}, nil
}

// pricingContext returns the pricing context the provider's adapter reports
// for a known usage, which the rate table needs to price it.
func (v llmValue) pricingContext() (llm.PricingContext, error) {
	switch v.Provider {
	case providerClaude:
		return llm.PricingContextUnspecified, nil
	case providerOpenAI:
		return llm.PricingContextStandard30m, nil
	default:
		return 0, fmt.Errorf("%w: provider %q", errConfigTable, v.Provider)
	}
}

// config is one row of the run script's config table.
type config struct {
	ID        string
	Stage     int // 1 or 2
	LLM       llmValue
	ReviewLLM llmValue
	Mode      string
}

// stageModel returns the LLM value that serves stage: generation and shorten
// use the generation value, review the review value.
func (c config) stageModel(stage string) llmValue {
	if stage == stageReview {
		return c.ReviewLLM
	}
	return c.LLM
}

// callableStages returns the stages a run of the config may call: all three
// for a full run; shorten and review for a resumed one (shorten may still run
// when the reused generation is over the length limit).
func (c config) callableStages() []string {
	if c.Mode == modeResume {
		return []string{stageShorten, stageReview}
	}
	return stages
}

// openAIModels returns the OpenAI models the config calls.
func (c config) openAIModels() []string {
	var out []string
	for _, s := range c.callableStages() {
		v := c.stageModel(s)
		if v.Provider == providerOpenAI && !slices.Contains(out, v.Model) {
			out = append(out, v.Model)
		}
	}
	return out
}

// configTable is the run script's table, in the order it lists the configs.
type configTable []config

// byID returns the config with the ID.
func (t configTable) byID(id string) (config, bool) {
	for _, c := range t {
		if c.ID == id {
			return c, true
		}
	}
	return config{}, false
}

// ids returns the config IDs of the given stage (0 for every stage).
func (t configTable) ids(stage int) []string {
	var out []string
	for _, c := range t {
		if stage == 0 || c.Stage == stage {
			out = append(out, c.ID)
		}
	}
	return out
}

// parseConfigTable reads the table "run.sh configs" prints: one config per
// line, "<id>\t<stage>\t<YT2COLUMN_LLM>\t<YT2COLUMN_REVIEW_LLM>\t<mode>".
func parseConfigTable(r io.Reader) (configTable, error) {
	var table configTable
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 5 { //nolint:mnd // the five columns of the table
			return nil, fmt.Errorf("%w: line %q", errConfigTable, scanner.Text())
		}
		c := config{ID: fields[0], Mode: fields[4]}
		switch fields[1] {
		case "1":
			c.Stage = 1
		case "2":
			c.Stage = 2
		default:
			return nil, fmt.Errorf("%w: stage %q", errConfigTable, fields[1])
		}
		if c.Mode != modeFull && c.Mode != modeResume {
			return nil, fmt.Errorf("%w: mode %q", errConfigTable, c.Mode)
		}
		var err error
		if c.LLM, err = parseLLMValue(fields[2]); err != nil {
			return nil, err
		}
		if c.ReviewLLM, err = parseLLMValue(fields[3]); err != nil {
			return nil, err
		}
		if _, dup := table.byID(c.ID); dup || c.ID == "" {
			return nil, fmt.Errorf("%w: config ID %q", errConfigTable, c.ID)
		}
		table = append(table, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(table) == 0 {
		return nil, fmt.Errorf("%w: no configs", errConfigTable)
	}
	return table, nil
}

// runScriptTimeout bounds "run.sh configs", which only prints the table.
const runScriptTimeout = 30 * time.Second

// loadConfigTable runs "<runsh> configs" and parses its output. The run
// script is the only place the table is written (design section 3.1).
func loadConfigTable(runsh string) (configTable, error) {
	ctx, cancel := context.WithTimeout(context.Background(), runScriptTimeout)
	defer cancel()
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, runsh, "configs") //nolint:gosec // the run script path comes from the operator's flag
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s configs: %w", runsh, err)
	}
	return parseConfigTable(&stdout)
}
