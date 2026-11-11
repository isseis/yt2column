package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

// errReproduce reports 0018 values the aggregation does not reproduce.
var errReproduce = errors.New("0018 values not reproduced")

// summary0018 is the cost and duration summary of one 0018 config, computed
// with the functions aggregate uses for 0087.
type summary0018 struct {
	Config           string
	CostSum, CostMax micro
	CostMaxArticle   string
	DurSum, DurMax   time.Duration
	DurMaxArticle    string
	N                int
}

// summarize0018 summarizes a 0018 config's current runs. B3 and C03 resumed
// from B3g's generation, so their durations are composed the way aggregate
// composes a resumed 0087 run's.
func summarize0018(d data0018, cfg string) (summary0018, error) {
	s := summary0018{Config: cfg}
	for _, a := range articles() {
		r := d.Runs[cfg][a]
		if !r.succeeded() || !r.Log.TotalArticle.Known {
			return summary0018{}, fmt.Errorf("%w: %s %s did not succeed with a known cost", errReproduce, cfg, a)
		}
		dur, err := d.resumedDuration(r)
		if err != nil {
			return summary0018{}, err
		}
		cost := r.Log.TotalArticle.USD
		s.N++
		s.CostSum += cost
		s.DurSum += dur
		if cost > s.CostMax {
			s.CostMax, s.CostMaxArticle = cost, a
		}
		if dur > s.DurMax {
			s.DurMax, s.DurMaxArticle = dur, a
		}
	}
	return s, nil
}

// runReproduce0018 implements "reproduce0018": it prints B3's and C03's sums,
// averages, and maxima, and with -baseline checks that B3's averages equal
// baseline.json's.
func runReproduce0018(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("reproduce0018", flag.ContinueOnError)
	base := fs.String("base0018", "", "the 0018 directory")
	baselinePath := fs.String("baseline", "", "baseline.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := load0018(*base)
	if err != nil {
		return err
	}
	b, err := loadBaseline(*baselinePath)
	if err != nil {
		return err
	}
	t := newTable("reproduce0018", 1, "config", "cost_sum_usd", "cost_avg_usd", "cost_max_usd", "cost_max_article",
		"duration_avg", "duration_max", "duration_max_article")
	for _, cfg := range []string{"B3", "C03"} {
		s, err := summarize0018(d, cfg)
		if err != nil {
			return err
		}
		costAvg := avgMicro(s.CostSum, s.N)
		durAvg := avgDuration(s.DurSum, s.N)
		t.add(cfg, s.CostSum.String(), costAvg.String(), s.CostMax.String(), s.CostMaxArticle,
			fmtDuration(durAvg), fmtDuration(s.DurMax), s.DurMaxArticle)
		if cfg == "B3" && (costAvg != b.costAvg ||
			fmtDuration(durAvg) != fmtDuration(time.Duration(b.DurationAvgMs)*time.Millisecond)) {
			return fmt.Errorf("%w: B3's averages differ from baseline.json", errReproduce)
		}
	}
	return writeTables(stdout, t)
}
