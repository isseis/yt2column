package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/llm"
)

// errAggregate reports evaluation data the aggregation cannot use.
var errAggregate = errors.New("cannot aggregate")

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// evalData is everything aggregate reads.
type evalData struct {
	Configs   configTable
	Runs      []run
	D18       data0018
	Bounds    map[string]micro
	Prepay    map[runKey]bool
	Approvals []approval
	Launches  []launchEntry
	Judg      map[int]judgments // by stage; a stage without a file is absent
	Checks    map[articleKey]articleCheck
	Baseline  baseline
	Excluded  []string
}

// runAggregate implements "aggregate".
func runAggregate(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("aggregate", flag.ContinueOnError)
	root := fs.String("root", "", "the 0087 directory (~/yt2column-eval/0087)")
	base := fs.String("base0018", "", "the 0018 directory (~/yt2column-eval/0018)")
	runsh := fs.String("runsh", "", "the run script, which prints the config table")
	boundsPath := fs.String("bounds", "", "the bounds table estimate wrote")
	baselinePath := fs.String("baseline", "", "baseline.json")
	var excluded stringList
	fs.Var(&excluded, "exclude", "a config the human declined at step 3 of the change condition (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	configs, err := loadConfigTable(*runsh)
	if err != nil {
		return err
	}
	d, err := loadEvalData(*root, *base, *boundsPath, *baselinePath, configs)
	if err != nil {
		return err
	}
	d.Excluded = excluded
	tables, err := aggregateTables(d)
	if err != nil {
		return err
	}
	return writeTables(stdout, tables...)
}

// loadEvalData reads the runs, the 0018 data, and the optional files under
// root: prepayment.tsv, approval, launch.log, and score/<stage>/.
func loadEvalData(root, base, boundsPath, baselinePath string, configs configTable) (evalData, error) {
	d := evalData{Configs: configs, Judg: map[int]judgments{}, Checks: map[articleKey]articleCheck{}}
	var err error
	if d.Runs, err = loadRuns0087(root, configs); err != nil {
		return evalData{}, err
	}
	if d.D18, err = load0018(base); err != nil {
		return evalData{}, err
	}
	bt, err := loadBoundsTable(boundsPath)
	if err != nil {
		return evalData{}, err
	}
	if d.Bounds, err = runBounds(bt); err != nil {
		return evalData{}, err
	}
	if d.Baseline, err = loadBaseline(baselinePath); err != nil {
		return evalData{}, err
	}
	if d.Prepay, err = optional(filepath.Join(root, "prepayment.tsv"), parsePrepayment, map[runKey]bool{}); err != nil {
		return evalData{}, err
	}
	if d.Approvals, err = optional(filepath.Join(root, "approval"), parseApprovals, nil); err != nil {
		return evalData{}, err
	}
	if d.Launches, err = optional(filepath.Join(root, launchLogName), parseLaunchLog, nil); err != nil {
		return evalData{}, err
	}
	for _, stage := range []int{1, 2} {
		dir := filepath.Join(root, "score", strconv.Itoa(stage))
		js, err := optional(filepath.Join(dir, "judgments.tsv"), parseJudgments, nil)
		if err != nil {
			return evalData{}, err
		}
		if js != nil {
			if err := js.checkComplete(obtained(d.Runs, configs.ids(stage))); err != nil {
				return evalData{}, err
			}
			d.Judg[stage] = js
		}
		checks, err := optional(filepath.Join(dir, "check_article.txt"), parseCheckArticle, nil)
		if err != nil {
			return evalData{}, err
		}
		maps.Copy(d.Checks, checks)
	}
	return d, d.checkPrepayment()
}

// optional parses path when it exists and returns empty otherwise.
func optional[T any](path string, parse func(io.Reader) (T, error), empty T) (T, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	return readParsed(path, parse)
}

// checkPrepayment rejects a prepayment entry that names no run, or a run that
// called an LLM and so cannot have stopped before payment.
func (d evalData) checkPrepayment() error {
	for k := range d.Prepay {
		r, ok := d.find(k)
		if !ok || !prepaymentCandidate(r) {
			return fmt.Errorf("%w: prepayment entry %v is not a run that called no LLM", errAggregate, k)
		}
	}
	return nil
}

// find returns the run with the key.
func (d evalData) find(k runKey) (run, bool) {
	for _, r := range d.Runs {
		if r.key() == k {
			return r, true
		}
	}
	return run{}, false
}

// obtained returns the articles of configs whose current run succeeded.
func obtained(runs []run, configs []string) []articleKey {
	var out []articleKey
	for _, r := range runs {
		if r.Attempt == attemptCurrent && r.succeeded() && slices.Contains(configs, r.Config) {
			out = append(out, articleKey{r.Config, r.Article})
		}
	}
	return out
}

// sortedRuns returns the runs in config-table order, then article order, then
// attempt (failed attempts first, the current one last).
func (d evalData) sortedRuns() []run {
	order := func(r run) (int, int, string) {
		ci := slices.IndexFunc(d.Configs, func(c config) bool { return c.ID == r.Config })
		ai := slices.Index(articles(), r.Article)
		at := r.Attempt
		if at == attemptCurrent {
			at = "z"
		}
		return ci, ai, at
	}
	out := slices.Clone(d.Runs)
	slices.SortFunc(out, func(a, b run) int {
		ac, aa, at := order(a)
		bc, ba, bt := order(b)
		if ac != bc {
			return ac - bc
		}
		if aa != ba {
			return aa - ba
		}
		return strings.Compare(at, bt)
	})
	return out
}

// current returns the current run of an article, if any.
func (d evalData) current(cfg, article string) (run, bool) {
	return d.find(runKey{cfg, article, attemptCurrent})
}

// aggregateTables builds every table the record copies.
func aggregateTables(d evalData) ([]*table, error) {
	var out []*table
	builders := []func(evalData) ([]*table, error){
		baselineTables, runTables, identityTable, usageTables, failureTables,
		spendTable, judgmentTables, comparisonTables, approvalTables,
	}
	for _, b := range builders {
		ts, err := b(d)
		if err != nil {
			return nil, err
		}
		out = append(out, ts...)
	}
	return out, nil
}

// fmtTime formats a time in UTC for a table cell.
func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// baselineTables renders the baseline config's artifact identifiers and its
// values from baseline.json.
func baselineTables(d evalData) ([]*table, error) {
	t := newTable("baseline-artifacts", 1, "article", "generation_id", "generation_date",
		"work_generation_same", "shorten_id", "review_id")
	for _, a := range articles() {
		gen, ok, err := readArtifact(filepath.Join(d.D18.Root, "base", "B3", a), a, stageGeneration)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("%w: base/B3/%s has no generation artifact", errAggregate, a)
		}
		work, err := artifactIDs(filepath.Join(d.D18.Root, "work", "B3", a), a)
		if err != nil {
			return nil, err
		}
		t.add(a, gen.ID, gen.CallTime.UTC().Format(time.DateOnly), yesNo(work[stageGeneration] == gen.ID),
			work[stageShorten], work[stageReview])
	}
	b := newTable("baseline", 1, "config", "pass_interp", "pass_literal", "cost_avg_usd", "duration_avg")
	b.add("B3", strconv.Itoa(d.Baseline.PassInterp), strconv.Itoa(d.Baseline.PassLiteral),
		d.Baseline.costAvg.String(), fmtDuration(time.Duration(d.Baseline.DurationAvgMs)*time.Millisecond))
	return []*table{t, b}, nil
}

// runTables renders the runs, the launches in order with the articles
// each started in parallel, and the run conditions every record holds.
func runTables(d evalData) ([]*table, error) {
	runs := newTable("runs", keysRun, "config", "article", "attempt", "after", "llm", "review_llm", "start", "end", "exit",
		"generation_id", "shorten_id", "review_id")
	env := newTable("run-conditions", 1, "item", "value")
	ytdlpFalse, keepCache := 0, 0
	var binaries []string
	for _, r := range d.sortedRuns() {
		ids, err := artifactIDs(r.WorkDir, r.Article)
		if err != nil {
			return nil, err
		}
		end, exit := none, none
		if r.When.HasEnd {
			end = fmtTime(r.When.End)
		}
		if r.When.HasExit {
			exit = strconv.Itoa(r.When.Exit)
		}
		runs.add(r.Config, r.Article, r.Attempt, r.When.After, r.When.LLM, r.When.ReviewLLM, fmtTime(r.When.Start),
			end, exit, ids[stageGeneration], ids[stageShorten], ids[stageReview])
		if r.When.YtDlpPath == "/usr/bin/false" {
			ytdlpFalse++
		}
		if r.When.KeepCache {
			keepCache++
		}
		if !slices.Contains(binaries, r.When.BinarySHA256) {
			binaries = append(binaries, r.When.BinarySHA256)
		}
	}
	n := len(d.Runs)
	env.add("runs", strconv.Itoa(n))
	env.add("ytdlp_path_false", strconv.Itoa(ytdlpFalse))
	env.add("keep_cache", strconv.Itoa(keepCache))
	kept := none
	if n > 0 {
		kept = yesNo(ytdlpFalse == n && keepCache == n)
	}
	env.add("all_kept_conditions", kept)
	binCell := none
	if len(binaries) > 0 {
		binCell = strings.Join(binaries, ",")
	}
	env.add("binary_sha256", binCell)
	launches := newTable("launches", 1, "seq", "launched_at", "config", "articles")
	for i, l := range d.Launches {
		launches.add(strconv.Itoa(i+1), fmtTime(l.At), l.Config, strings.Join(l.Articles, ","))
	}
	return []*table{runs, env, launches}, nil
}

// identityCheck is the resumed-config check of one run (design section 3.2).
type identityCheck struct {
	GenMatch, ShortenMatch, ShortenRan string
	OffProcedure                       bool
}

// checkIdentity compares a resumed run's reused artifacts with B3's: the
// generation from base/B3 (work/B3 for the article resumed after shorten, with
// its shorten too). A shorten the run called itself, or an identifier that
// differs, takes the run off the procedure.
func (d evalData) checkIdentity(r run) (identityCheck, error) {
	src := filepath.Join(d.D18.Root, "base", "B3", r.Article)
	if r.When.After == stageShorten {
		src = filepath.Join(d.D18.Root, "work", "B3", r.Article)
	}
	want, err := artifactIDs(src, r.Article)
	if err != nil {
		return identityCheck{}, err
	}
	got, err := artifactIDs(r.WorkDir, r.Article)
	if err != nil {
		return identityCheck{}, err
	}
	c := identityCheck{GenMatch: yesNo(got[stageGeneration] == want[stageGeneration]), ShortenMatch: none}
	ok := got[stageGeneration] == want[stageGeneration] && want[stageGeneration] != none
	if r.When.After == stageShorten {
		c.ShortenMatch = yesNo(got[stageShorten] == want[stageShorten])
		ok = ok && got[stageShorten] == want[stageShorten] && want[stageShorten] != none
	}
	cost, ran := r.Log.Costs[stageShorten]
	ran = ran && cost.Origin == originRun
	c.ShortenRan = yesNo(ran)
	c.OffProcedure = !ok || ran
	return c, nil
}

// identityTable renders the check of every current run of a resumed config.
func identityTable(d evalData) ([]*table, error) {
	t := newTable("resume-identity", keysArticle, "config", "article", "after", "generation_match", "shorten_match",
		"shorten_ran", "off_procedure")
	for _, r := range d.sortedRuns() {
		c, _ := d.Configs.byID(r.Config)
		if c.Mode != modeResume || r.Attempt != attemptCurrent || !r.succeeded() {
			continue
		}
		ic, err := d.checkIdentity(r)
		if err != nil {
			return nil, err
		}
		t.add(r.Config, r.Article, r.When.After, ic.GenMatch, ic.ShortenMatch, ic.ShortenRan, yesNo(ic.OffProcedure))
	}
	return []*table{t}, nil
}

// usable reports whether a run's article enters the comparison: the current
// run succeeded and, for a resumed config, kept to the procedure.
func (d evalData) usable(r run) (bool, error) {
	if r.Attempt != attemptCurrent || !r.succeeded() {
		return false, nil
	}
	c, _ := d.Configs.byID(r.Config)
	if c.Mode != modeResume {
		return true, nil
	}
	ic, err := d.checkIdentity(r)
	return !ic.OffProcedure, err
}

// duration returns how long the article took (design section 3.7): a full
// run's wall time; for a resumed run, B3's generation time plus the run's
// time, plus B3's shorten time when the run resumed after shorten.
func (d evalData) duration(r run) (time.Duration, error) {
	if !r.Log.HasWall {
		return 0, fmt.Errorf("%w: %s %s %s has no time line", errAggregate, r.Config, r.Article, r.Attempt)
	}
	c, _ := d.Configs.byID(r.Config)
	if c.Mode != modeResume {
		return r.Log.Wall, nil
	}
	return d.D18.resumedDuration(r)
}

// resumedDuration composes the duration of a run resumed from B3's artifacts,
// the way 0018 composed C03's (0018 evaluation record section 4.8).
func (d data0018) resumedDuration(r run) (time.Duration, error) {
	gen := d.Runs["B3g"][r.Article]
	if !gen.Log.HasWall || !r.Log.HasWall {
		return 0, fmt.Errorf("%w: %s lacks a time line", errAggregate, r.Article)
	}
	total := gen.Log.Wall + r.Log.Wall
	if r.When.After == stageShorten {
		extra, err := d.shortenTime0018(r.Article)
		if err != nil {
			return 0, err
		}
		total += extra
	}
	return total, nil
}

// usageTables renders each stage's tokens and cost and each run's totals and
// duration.
func usageTables(d evalData) ([]*table, error) {
	u := newTable("stage-usage", keysRunStage, "config", "article", "attempt", "stage", "origin", "usage", "input", "cache_read",
		"cache_write", "output", "model", "usd", "unknown_reason", "reused_from")
	rc := newTable("run-costs", keysRun, "config", "article", "attempt", "exit", "cost_run_usd", "cost_article_usd",
		"wall", "duration", "usable")
	for _, r := range d.sortedRuns() {
		for _, s := range stages {
			t, hasT := r.Log.Tokens[s]
			c, hasC := r.Log.Costs[s]
			if !hasT && !hasC {
				continue
			}
			u.add(stageUsageRow(r, s, t, hasT, c, hasC)...)
		}
		ok, err := d.usable(r)
		if err != nil {
			return nil, err
		}
		dur := none
		if ok {
			dd, err := d.duration(r)
			if err != nil {
				return nil, err
			}
			dur = fmtDuration(dd)
		}
		exit, wall := none, none
		if r.When.HasExit {
			exit = strconv.Itoa(r.When.Exit)
		}
		if r.Log.HasWall {
			wall = fmtDuration(r.Log.Wall)
		}
		rc.add(r.Config, r.Article, r.Attempt, exit, totalCell(r.Log.TotalRun), totalCell(r.Log.TotalArticle), wall, dur, yesNo(ok))
	}
	return []*table{u, rc}, nil
}

// stageUsageRow renders one stage of one run.
func stageUsageRow(r run, s string, t tokenLine, hasT bool, c costLine, hasC bool) []string {
	origin, usage, in, cr, cw, out, model, usd, reason, from := none, none, none, none, none, none, none, none, none, none
	if hasT {
		origin, model = t.Origin, t.Model
		if model == "" {
			model = wordNoModel
		}
		usage = wordUnknown
		if t.Known {
			usage = "known"
			in, cr, cw, out = u64(t.Counts.Input), u64(t.Counts.CacheRead), u64(t.Counts.CacheWrite), u64(t.Counts.Output)
		} else {
			reason = t.Reason
		}
	}
	if hasC {
		origin = c.Origin
		if c.Known {
			usd = c.USD.String()
		} else {
			usd = wordUnknown
			reason = c.Reason
		}
	}
	if origin == originReused {
		from = "B3:" + r.Article
	}
	return []string{r.Config, r.Article, r.Attempt, s, origin, usage, in, cr, cw, out, model, usd, reason, from}
}

// u64 formats a count.
func u64(n uint64) string { return strconv.FormatUint(n, 10) }

// totalCell renders a cost-total line.
func totalCell(t *totalLine) string {
	switch {
	case t == nil:
		return none
	case !t.Known:
		return wordUnknown
	default:
		return t.USD.String()
	}
}

// failureTables renders the failed runs, the runs that may have
// stopped before payment, and the retries of each article.
func failureTables(d evalData) ([]*table, error) {
	f := newTable("failed-runs", keysRun, "config", "article", "attempt", "exit", "failed_stage", "input", "cache_read",
		"cache_write", "output", "usd", "prepayment_candidate", "prepayment_confirmed")
	retries := map[articleKey]int{}
	for _, r := range d.sortedRuns() {
		if r.Attempt == attemptCurrent && (r.succeeded() || !r.finished()) {
			continue
		}
		stage := r.Log.lastRunStage()
		in, cr, cw, out, usd := none, none, none, none, none
		if t, ok := r.Log.Tokens[stage]; ok && t.Known {
			in, cr, cw, out = u64(t.Counts.Input), u64(t.Counts.CacheRead), u64(t.Counts.CacheWrite), u64(t.Counts.Output)
		}
		if c, ok := r.Log.Costs[stage]; ok {
			usd = wordUnknown
			if c.Known {
				usd = c.USD.String()
			}
		}
		if stage == "" {
			stage = none
		}
		exit := none
		if r.When.HasExit {
			exit = strconv.Itoa(r.When.Exit)
		}
		f.add(r.Config, r.Article, r.Attempt, exit, stage, in, cr, cw, out, usd, yesNo(prepaymentCandidate(r)), yesNo(d.Prepay[r.key()]))
		if r.Attempt != attemptCurrent && !d.Prepay[r.key()] {
			retries[articleKey{r.Config, r.Article}]++
		}
	}
	rt := newTable("retries", keysArticle, "config", "article", "retries", "obtained", "over_limit")
	for _, c := range d.Configs {
		for _, a := range articles() {
			n, ok := retries[articleKey{c.ID, a}]
			if !ok {
				continue
			}
			cur, has := d.current(c.ID, a)
			rt.add(c.ID, a, strconv.Itoa(n), yesNo(has && cur.succeeded()), yesNo(n > 2)) //nolint:mnd // two retries per article
		}
	}
	return []*table{f, rt}, nil
}

// spendTable renders the spend as launch counts it and as the record counts
// it, which leaves out the runs the human confirmed as stopped before payment
// (plan section 1.2 item 7).
func spendTable(d evalData) ([]*table, error) {
	t := newTable("spend", 1, "scope", "known_usd", "unknown_stages", "bound_runs", "bound_usd", "total_usd",
		"excluded_runs", "excluded_bound_usd")
	for _, scope := range []struct {
		name    string
		exclude map[runKey]bool
	}{{"launch", nil}, {"record", d.Prepay}} {
		s, err := computeSpend(d.Runs, d.Bounds, scope.exclude)
		if err != nil {
			return nil, err
		}
		var exBound micro
		for _, e := range s.Excluded {
			exBound += e.Bound
		}
		t.add(scope.name, s.Known.String(), strconv.Itoa(s.UnknownStages), strconv.Itoa(s.BoundRuns), s.Bound.String(),
			s.Total().String(), strconv.Itoa(len(s.Excluded)), exBound.String())
	}
	ex := newTable("spend-excluded", keysRun, "config", "article", "attempt", "bound_usd")
	s, err := computeSpend(d.Runs, d.Bounds, d.Prepay)
	if err != nil {
		return nil, err
	}
	for _, e := range s.Excluded {
		ex.add(e.Key.Config, e.Key.Article, e.Key.Attempt, e.Bound.String())
	}
	return []*table{t, ex}, nil
}

// stageOf returns the evaluation stage (1 or 2) of a config.
func (d evalData) stageOf(cfg string) int {
	c, _ := d.Configs.byID(cfg)
	return c.Stage
}

// judgmentTables renders the judgments with their judges, the mechanical
// values, the articles and criteria that need a quote, and the product
// constraints.
func judgmentTables(d evalData) ([]*table, error) {
	cols := []string{wordConfig, wordArticle}
	jcols := slices.Concat(cols, requiredCriteria, qualityCriteria, []string{"goal_interp", "goal_literal"})
	jt := newTable("judgments", keysArticle, jcols...)
	who := newTable("judges", keysArticle, slices.Concat(cols, requiredCriteria, qualityCriteria, constraintCriteria)...)
	quotes := newTable("quotes-needed", keysRun, "config", "article", "criterion", "reading", "kind")
	cons := newTable("constraints", keysArticle, slices.Concat(cols, constraintCriteria)...)
	checks := newTable("article-checks", keysArticle, "config", "article", "title_chars", "sections", "length")
	for _, c := range d.Configs {
		js, ok := d.Judg[c.Stage]
		for _, a := range articles() {
			if ch, has := d.Checks[articleKey{c.ID, a}]; has {
				checks.add(c.ID, a, strconv.Itoa(ch.TitleChars), strconv.Itoa(ch.Sections), strconv.Itoa(ch.Length))
			}
			if _, judged := js[judgmentKey{c.ID, a, critLength}]; !ok || !judged {
				continue
			}
			row, judges, consRow := []string{c.ID, a}, []string{c.ID, a}, []string{c.ID, a}
			for _, crit := range allCriteria {
				j := js[judgmentKey{c.ID, a, crit}]
				judges = append(judges, judgeCell(j))
				if slices.Contains(constraintCriteria, crit) {
					consRow = append(consRow, j.Interp)
					continue
				}
				row = append(row, j.Interp+"/"+j.Literal)
				if q := quoteKind(j); q != "" {
					quotes.add(c.ID, a, crit, quoteReading(j, q), q)
				}
			}
			row = append(row, yesNo(js.goalMet(c.ID, a, true)), yesNo(js.goalMet(c.ID, a, false)))
			jt.add(row...)
			who.add(judges...)
			cons.add(consRow...)
		}
	}
	return []*table{jt, who, quotes, cons, checks}, nil
}

// judgeCell renders who decided a judgment: the script, the LLM, the LLM with
// a human's confirmation, or a human who changed it.
func judgeCell(j judgment) string {
	if j.Judge == judgeLLM && j.HumanCheck {
		return "llm+human"
	}
	return j.Judge
}

// quoteKind returns why a judgment needs a quote: a required
// criterion that fails, or a quality criterion at 1, by either reading.
func quoteKind(j judgment) string {
	switch {
	case slices.Contains(requiredCriteria, j.Criterion) && (j.Interp == valFail || j.Literal == valFail):
		return "required-fail"
	case slices.Contains(qualityCriteria, j.Criterion) && (j.Interp == "1" || j.Literal == "1"):
		return "quality-1"
	default:
		return ""
	}
}

// quoteReading returns which reading gave the result that needs the quote.
func quoteReading(j judgment, kind string) string {
	bad := valFail
	if kind == "quality-1" {
		bad = "1"
	}
	switch {
	case j.Interp == bad && j.Literal == bad:
		return "both"
	case j.Interp == bad:
		return "interp"
	default:
		return "literal"
	}
}

// stats computes one config's comparison values.
func (d evalData) stats(c config) (configStats, error) {
	s := configStats{Config: c.ID, FailsByVideo: map[string]int{}}
	js, judged := d.Judg[c.Stage]
	s.Judged = judged && len(obtained(d.Runs, []string{c.ID})) > 0
	for _, a := range articles() {
		r, ok := d.current(c.ID, a)
		if !ok {
			continue
		}
		use, err := d.usable(r)
		if err != nil || !use {
			if err != nil {
				return configStats{}, err
			}
			continue
		}
		s.Usable++
		if err := d.addCostAndTime(&s, c, r); err != nil {
			return configStats{}, err
		}
		if s.Judged {
			for _, crit := range qualityCriteria {
				n, _ := strconv.Atoi(js[judgmentKey{c.ID, a, crit}].Interp)
				s.Quality[n]++
			}
			if js.goalMet(c.ID, a, true) {
				s.PassInterp++
			}
			if js.goalMet(c.ID, a, false) {
				s.PassLiteral++
			}
			if js.requiredFailInterp(c.ID, a) {
				s.FailsByVideo[videoOf(a)]++
			}
		}
	}
	return s, nil
}

// addCostAndTime adds one usable run's cost, auxiliary cost, and duration.
func (d evalData) addCostAndTime(s *configStats, c config, r run) error {
	dur, err := d.duration(r)
	if err != nil {
		return err
	}
	s.DurSum += dur
	if dur > s.MaxDur {
		s.MaxDur, s.MaxDurArticle = dur, r.Article
	}
	if !r.Log.TotalArticle.Known {
		s.CostUnknown = true
		return nil
	}
	s.CostSum += r.Log.TotalArticle.USD
	if r.Log.TotalArticle.USD > s.MaxCost {
		s.MaxCost, s.MaxCostArticle = r.Log.TotalArticle.USD, r.Article
	}
	for _, st := range stages {
		cl, ok := r.Log.Costs[st]
		if !ok {
			continue
		}
		t := r.Log.Tokens[st]
		if cl.Origin != originRun || !t.Known || t.Counts.CacheRead == 0 {
			s.AuxCostSum += cl.USD
			continue
		}
		s.HasCacheRead = true
		s.CacheReadStages++
		amount, err := priceCall(c.stageModel(st), llm.TokenCounts{
			Input: t.Counts.Input, CacheWrite: t.Counts.CacheWrite + t.Counts.CacheRead, Output: t.Counts.Output,
		})
		if err != nil {
			return err
		}
		m, err := ceilMicro(amount)
		if err != nil {
			return err
		}
		s.AuxCostSum += m
	}
	return nil
}

// comparisonTables renders the comparison, the change condition, whether each
// product constraint is fixed, and the stage-2 choice.
func comparisonTables(d evalData) ([]*table, error) {
	cmp := newTable("comparison", 1, "config", "usable", "not_obtained", "pass_interp", "pass_literal", "quality_1",
		"quality_2", "quality_3", "cost_avg_usd", "cost_max_usd", "cost_max_article", "duration_avg", "duration_max",
		"duration_max_article", "failed_runs", "failed_usd", "failed_unknown_cost", "cache_read_stages", "aux_cost_avg_usd")
	var results []f006Result
	for _, c := range d.Configs {
		if !slices.ContainsFunc(d.Runs, func(r run) bool { return r.Config == c.ID }) {
			continue
		}
		s, err := d.stats(c)
		if err != nil {
			return nil, err
		}
		cmp.add(d.comparisonRow(c, s)...)
		results = append(results, judgeF006(s, d.Baseline))
	}
	perConfig, perVideo, selection := f006Tables(results, d.Baseline, d.Excluded)
	fixed := newTable("constraints-fixed", 1, slices.Concat([]string{"config"}, constraintCriteria)...)
	fixedRow := []string{"B3"}
	for _, cc := range constraintCriteria {
		fixedRow = append(fixedRow, yesNo(!d.Baseline.Constraints[cc]))
	}
	fixed.add(fixedRow...)
	for _, r := range results {
		if !r.Stats.Judged {
			continue
		}
		row := []string{r.Stats.Config}
		js := d.Judg[d.stageOf(r.Stats.Config)]
		for _, cc := range constraintCriteria {
			found := false
			for _, a := range articles() {
				if j, ok := js[judgmentKey{r.Stats.Config, a, cc}]; ok && j.Interp == valYes {
					found = true
				}
			}
			row = append(row, yesNo(r.Stats.Usable == len(articles()) && !found))
		}
		fixed.add(row...)
	}
	sel := newTable("b-selection", 1, "row", "c_astra_pass", "c_sol_pass", "b_configs")
	byID := map[string]f006Result{}
	for _, r := range results {
		byID[r.Stats.Config] = r
	}
	if a, sol := byID[configCAstra], byID[configCSol]; a.Stats.Judged && sol.Stats.Judged {
		b := selectB(a.Stats.PassInterp, sol.Stats.PassInterp)
		sel.add("B", strconv.Itoa(b.AstraPass), strconv.Itoa(b.SolPass), strings.Join(b.Configs, ","))
	}
	return []*table{cmp, perConfig, perVideo, selection, fixed, sel}, nil
}

// comparisonRow renders one config's comparison values.
func (d evalData) comparisonRow(c config, s configStats) []string {
	// A failed run is any finished run that did not succeed, the current one
	// included (an article given up after its last retry keeps it there),
	// except a run the human confirmed as stopped before payment. A run whose
	// cost is not fully known is counted apart rather than as 0.
	failedRuns, failedUnknown := 0, 0
	var failedUSD micro
	for _, r := range d.Runs {
		if r.Config != c.ID || !r.finished() || r.succeeded() || d.Prepay[r.key()] {
			continue
		}
		failedRuns++
		sum, unknown := runCost(r.Log)
		if unknown > 0 || uncosted(r.Log) || (r.Log.hasRunTokens() && (r.Log.TotalRun == nil || !r.Log.TotalRun.Known)) {
			failedUnknown++
			continue
		}
		failedUSD += sum
	}
	pass, lit, q1, q2, q3 := none, none, none, none, none
	if s.Judged {
		pass, lit = strconv.Itoa(s.PassInterp), strconv.Itoa(s.PassLiteral)
		q1, q2, q3 = strconv.Itoa(s.Quality[1]), strconv.Itoa(s.Quality[2]), strconv.Itoa(s.Quality[3])
	}
	costAvg, costMax, costMaxArt, durAvg, durMax, durMaxArt, aux := none, none, none, none, none, none, none
	if s.Usable > 0 && !s.CostUnknown {
		costAvg, costMax, costMaxArt = avgMicro(s.CostSum, s.Usable).String(), s.MaxCost.String(), s.MaxCostArticle
	}
	if s.Usable > 0 {
		durAvg, durMax, durMaxArt = fmtDuration(avgDuration(s.DurSum, s.Usable)), fmtDuration(s.MaxDur), s.MaxDurArticle
	}
	if s.HasCacheRead && !s.CostUnknown {
		aux = avgMicro(s.AuxCostSum, s.Usable).String()
	}
	return []string{
		c.ID, strconv.Itoa(s.Usable), strconv.Itoa(len(articles()) - s.Usable), pass, lit, q1, q2, q3,
		costAvg, costMax, costMaxArt, durAvg, durMax, durMaxArt, strconv.Itoa(failedRuns), failedUSD.String(),
		strconv.Itoa(failedUnknown), strconv.Itoa(s.CacheReadStages), aux,
	}
}

// approvalTables renders the approvals and the runs that started before the
// first approval.
func approvalTables(d evalData) ([]*table, error) {
	ap := newTable("approvals", 1, "seq", "approved_at", "configs", "limit_usd")
	early := newTable("pre-approval-runs", keysRun, "config", "article", "attempt", "start")
	for i, a := range d.Approvals {
		ap.add(strconv.Itoa(i+1), fmtTime(a.At), strings.Join(a.Configs, ","), a.Limit.String())
	}
	for _, r := range d.sortedRuns() {
		if len(d.Approvals) == 0 || r.When.Start.Before(d.Approvals[0].At) {
			early.add(r.Config, r.Article, r.Attempt, fmtTime(r.When.Start))
		}
	}
	return []*table{ap, early}, nil
}
