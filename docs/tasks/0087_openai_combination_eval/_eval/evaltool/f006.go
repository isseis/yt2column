package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// baseline is _eval/baseline.json: the values of the baseline config (B3)
// that the change condition compares against, each with its source in
// Sources.
type baseline struct {
	Sources             map[string]string `json:"sources"`
	PassInterp          int               `json:"pass_interp"`
	PassLiteral         int               `json:"pass_literal"`
	CostAvgUSD          string            `json:"cost_avg_usd"`
	DurationAvgMs       int64             `json:"duration_avg_ms"`
	RequiredFailByVideo map[string]int    `json:"required_fail_by_video"`
	Constraints         map[string]bool   `json:"constraints"`

	costAvg micro
}

// errBaseline reports a baseline file that lacks a value the change
// condition needs.
var errBaseline = errors.New("baseline file lacks a value")

// loadBaseline reads baseline.json, refusing unknown keys and a value that
// does not cover every video and constraint.
func loadBaseline(path string) (baseline, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from the operator's flag
	if err != nil {
		return baseline{}, err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var b baseline
	if err := dec.Decode(&b); err != nil {
		return baseline{}, fmt.Errorf("%s: %w", path, err)
	}
	if b.costAvg, err = parseMicro(b.CostAvgUSD); err != nil {
		return baseline{}, fmt.Errorf("%s: %w", path, err)
	}
	for _, v := range videos {
		if _, ok := b.RequiredFailByVideo[v]; !ok {
			return baseline{}, fmt.Errorf("%w: %s lacks the failures of %s", errBaseline, path, v)
		}
	}
	for _, c := range constraintCriteria {
		if _, ok := b.Constraints[c]; !ok {
			return baseline{}, fmt.Errorf("%w: %s lacks %s", errBaseline, path, c)
		}
	}
	return b, nil
}

// configStats are the values of one config that the comparison and the change
// condition read. Only the config's usable articles count: obtained by a successful run
// that kept to the procedure.
type configStats struct {
	Config       string
	Usable       int
	Judged       bool // the config's judgments are complete
	PassInterp   int
	PassLiteral  int
	FailsByVideo map[string]int
	CostSum      micro
	CostUnknown  bool
	DurSum       time.Duration
	// AuxCostSum is CostSum with each stage's cache-read input recounted at
	// the cache-write rate; HasCacheRead says whether any stage read the cache.
	AuxCostSum   micro
	HasCacheRead bool
	// The comparison's other values: the quality judgments by level (index
	// 1 to 3, interpretive reading), the maxima, and the stages that read
	// the prompt cache.
	Quality         [4]int
	MaxCost         micro
	MaxCostArticle  string
	MaxDur          time.Duration
	MaxDurArticle   string
	CacheReadStages int
}

// complete reports whether the config has every article with a known cost,
// which (ii) and (iii) require (requirements decision 12).
func (s configStats) complete() bool {
	return s.Usable == len(articles()) && !s.CostUnknown
}

// f006Result is the judgment of one config against the condition for changing
// the production profile.
type f006Result struct {
	Stats                   configStats
	CondI, CondII, CondIII  bool
	PassOK, VideoOK, CostOK bool
	Candidate               bool
	// AuxChanges says whether (ii) or (iii) would differ with the auxiliary
	// cost (design section 3.5).
	AuxChanges bool
}

// judgeF006 applies step 1 of the change condition to one config:
// candidate when (i) or (ii), and (iii), hold. Averages are compared as
// sums against the baseline average times the article count, so no rounding
// moves a value across a limit.
func judgeF006(s configStats, b baseline) f006Result {
	r := f006Result{Stats: s}
	if !s.Judged {
		return r
	}
	n := int64(s.Usable)
	r.CondI = s.PassInterp >= b.PassInterp+2 //nolint:mnd // a gain of two or more articles
	r.PassOK = s.PassInterp >= b.PassInterp
	r.VideoOK = true
	for _, v := range videos {
		if s.FailsByVideo[v] > b.RequiredFailByVideo[v] {
			r.VideoOK = false
		}
	}
	condII := func(cost micro) bool {
		return s.complete() && r.PassOK && r.VideoOK && int64(cost) < int64(b.costAvg)*n
	}
	condIII := func(cost micro) bool {
		return s.complete() && int64(cost) <= 3*int64(b.costAvg)*n && //nolint:mnd // three times the baseline cost
			s.DurSum.Milliseconds() <= 2*b.DurationAvgMs*n //nolint:mnd // twice the baseline duration
	}
	r.CostOK = s.complete() && int64(s.CostSum) < int64(b.costAvg)*n
	r.CondII = condII(s.CostSum)
	r.CondIII = condIII(s.CostSum)
	r.Candidate = (r.CondI || r.CondII) && r.CondIII
	if s.HasCacheRead {
		r.AuxChanges = condII(s.AuxCostSum) != r.CondII || condIII(s.AuxCostSum) != r.CondIII
	}
	return r
}

// selectF006 applies step 2 of the change condition to the candidates not in excluded: the most
// passing articles, then the lowest average cost. It returns "none" when no
// candidate remains, and "tie:<a>,<b>" when two are equal on both, which the
// human decides.
func selectF006(results []f006Result, excluded []string) string {
	var best []f006Result
	for _, r := range results {
		if !r.Candidate || slices.Contains(excluded, r.Stats.Config) {
			continue
		}
		switch {
		case len(best) == 0:
			best = []f006Result{r}
		case r.Stats.PassInterp > best[0].Stats.PassInterp ||
			(r.Stats.PassInterp == best[0].Stats.PassInterp && r.Stats.CostSum < best[0].Stats.CostSum):
			best = []f006Result{r}
		case r.Stats.PassInterp == best[0].Stats.PassInterp && r.Stats.CostSum == best[0].Stats.CostSum:
			best = append(best, r)
		}
	}
	switch len(best) {
	case 0:
		return "none"
	case 1:
		return best[0].Stats.Config
	default:
		var ids []string
		for _, r := range best {
			ids = append(ids, r.Stats.Config)
		}
		return "tie:" + strings.Join(ids, ",")
	}
}

// bSelection is the choice of the stage-2 model from the two C configs'
// passing articles: the model with more, or both on a tie, sol first.
type bSelection struct {
	AstraPass, SolPass int
	Configs            []string
}

// The configs whose passing articles choose the stage-2 model, and the
// stage-2 config of each model.
const (
	configCAstra = "C-astra"
	configCSol   = "C-sol"
	configBAstra = "B-astra"
	configBSol   = "B-sol"
)

// selectB chooses stage 2: B is made with the C model that passed more
// articles, or with both (B-sol first) when they tie.
func selectB(astraPass, solPass int) bSelection {
	s := bSelection{AstraPass: astraPass, SolPass: solPass}
	switch {
	case astraPass > solPass:
		s.Configs = []string{configBAstra}
	case solPass > astraPass:
		s.Configs = []string{configBSol}
	default:
		s.Configs = []string{configBSol, configBAstra}
	}
	return s
}

// f006Tables renders the change-condition results: one row per config, one row per
// config and video for (ii)'s per-video comparison, and the selection.
func f006Tables(results []f006Result, b baseline, excluded []string) (perConfig, perVideo, selection *table) {
	perConfig = newTable("f006", 1, "config", "pass_interp", "pass_gain", "cond_i", "pass_ok", "video_ok",
		"cost_avg_usd", "cost_lower", "cond_ii", "duration_avg", "cond_iii", "candidate", "aux_changes")
	perVideo = newTable("f006-video", keysArticle, "config", "video", "required_fail", "baseline_required_fail", "ok")
	for _, r := range results {
		s := r.Stats
		if !s.Judged {
			continue
		}
		costAvg, durAvg := none, none
		if s.complete() {
			costAvg = avgMicro(s.CostSum, s.Usable).String()
			durAvg = fmtDuration(avgDuration(s.DurSum, s.Usable))
		}
		perConfig.add(s.Config, strconv.Itoa(s.PassInterp), strconv.Itoa(s.PassInterp-b.PassInterp),
			yesNo(r.CondI), yesNo(r.PassOK), yesNo(r.VideoOK), costAvg, yesNo(r.CostOK), yesNo(r.CondII),
			durAvg, yesNo(r.CondIII), yesNo(r.Candidate), yesNo(r.AuxChanges))
		for _, v := range videos {
			perVideo.add(s.Config, v, strconv.Itoa(s.FailsByVideo[v]), strconv.Itoa(b.RequiredFailByVideo[v]),
				yesNo(s.FailsByVideo[v] <= b.RequiredFailByVideo[v]))
		}
	}
	selection = newTable("f006-selection", 1, "excluded", "selected")
	ex := none
	if len(excluded) > 0 {
		ex = strings.Join(excluded, ",")
	}
	selection.add(ex, selectF006(results, excluded))
	return perConfig, perVideo, selection
}
