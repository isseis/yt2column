package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/writer"
)

// testBounds are run bounds for computeSpend: one config's run bound each.
var testBounds = map[string]micro{"C-sol": 1_000_000, "C-astra": 2_000_000, "A-sol": 500_000, "A-astra": 700_000, "B-sol": 900_000, "B-astra": 1_900_000}

// TestComputeSpend counts a run's run-origin costs, never a reused stage, and
// counts a run at its bound when a cost is unknown or the run wrote no
// "cost-total: run" line. Only the exclusion list removes a run, and launch
// passes none.
func TestComputeSpend(t *testing.T) {
	paid := run{Config: "A-sol", Article: "a-1", Attempt: attemptCurrent, Log: mustLog(t, logText([]stageLines{
		{Stage: stageGeneration, Origin: originReused, USD: "0.305524", Model: "claude-opus-5-5"},
		{Stage: stageReview, Origin: originRun, USD: "0.045000", Model: "gpt-6.1-sol"},
	}, "known usd=0.045000", "known usd=0.350524", "1.0"))}
	unknownCost := run{Config: "C-sol", Article: "b-1", Attempt: attemptCurrent, Log: mustLog(t, logText([]stageLines{
		{Stage: stageGeneration, Origin: originRun, USD: "0.070000", Model: "gpt-6.1-sol"},
		{Stage: stageReview, Origin: originRun, USD: "unknown", Model: "gpt-6.1-sol"},
	}, "unknown", "unknown", "1.0"))}
	killed := run{
		Config: "C-astra", Article: "c-1", Attempt: "f1", When: when{HasExit: true, Exit: 2},
		Log: mustLog(t, logText([]stageLines{{Stage: stageGeneration, Origin: originRun, USD: "0.300000", Model: "gpt-6-astra"}}, "", "", "")),
	}
	noLLM := run{Config: "C-astra", Article: "d-1", Attempt: "f1", When: when{HasExit: true, Exit: 2}, Log: mustLog(t, "yt2column: missing\n")}

	s, err := computeSpend([]run{paid}, testBounds, nil)
	if err != nil || s.Known != 45000 || s.BoundRuns != 0 || s.Total() != 45000 {
		t.Fatalf("a paid run alone: %+v, %v; want only its run-origin cost 0.045000", s, err)
	}
	all := []run{paid, unknownCost, killed, noLLM}
	s, err = computeSpend(all, testBounds, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Known != 45000 || s.BoundRuns != 3 || s.UnknownStages != 1 || s.Bound != 1_000_000+2*2_000_000 {
		t.Errorf("launch spend = %+v", s)
	}
	exclude := map[runKey]bool{noLLM.key(): true}
	s, err = computeSpend(all, testBounds, exclude)
	if err != nil {
		t.Fatal(err)
	}
	if s.BoundRuns != 2 || len(s.Excluded) != 1 || s.Excluded[0].Bound != 2_000_000 {
		t.Errorf("record spend = %+v", s)
	}
	if !prepaymentCandidate(noLLM) || prepaymentCandidate(killed) || prepaymentCandidate(paid) {
		t.Errorf("only a finished run without run tokens is a prepayment candidate")
	}
	if _, err := computeSpend([]run{{Config: "Z"}}, testBounds, nil); !errors.Is(err, errSpend) {
		t.Errorf("a config without a bound: err = %v", err)
	}
}

// mustLog parses a fake log.
func mustLog(t *testing.T, text string) runLog {
	t.Helper()
	l, err := parseRunLog(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// evalFixture is a fake evaluation: C-sol and A-sol with ten successful runs
// each, judged; the 0018 tree; and the files aggregate reads.
type evalFixture struct {
	Root, Base0018 string
	Configs        configTable
	Bounds         *table
	Baseline       baseline
}

// newEvalFixture writes the fake evaluation.
func newEvalFixture(t *testing.T) evalFixture {
	t.Helper()
	dir := t.TempDir()
	f := evalFixture{Root: filepath.Join(dir, "0087"), Base0018: filepath.Join(dir, "0018"), Configs: mustConfigs(t)}
	genIDs, shortenIDs := fake0018(t, f.Base0018)
	for i, a := range articles() {
		usd := fmt.Sprintf("0.%06d", 100000+i*1000)
		writeRun(t, f.Root, fakeRun{
			Config: "C-sol", Article: a, Start: t0, Exit: 0,
			Log: fullRunLog("gpt-6.1-sol", "0.050000", usd, addUSD("0.050000", usd), fmt.Sprintf("1:%02d.0", 10+i)),
		})
		after := stageGeneration
		if a == "roNZ-rLxhUE-1" {
			after = stageShorten
		}
		writeRun(t, f.Root, fakeRun{
			Config: "A-sol", Article: a, After: after, Start: t0.Add(time.Hour), Exit: 0,
			Log: logText([]stageLines{
				{Stage: stageGeneration, Origin: originReused, USD: "0.300000", Model: "claude-opus-5-5"},
				{Stage: stageReview, Origin: originRun, CW: 15000, Out: 700, USD: "0.040000", Model: "gpt-6.1-sol"},
			}, "known usd=0.040000", "known usd=0.340000", "20.0"),
		})
		work := filepath.Join(f.Root, "work", "A-sol", a)
		gen := testRecord(writer.StageGeneration, "T"+a, "\nBody of "+a+".\n", "claude-opus-5-5", writer.Digest("material", a))
		if got := writeRecord(t, work, a, gen); got != genIDs[a] {
			t.Fatalf("fixture: generation copy id differs")
		}
		if a == "roNZ-rLxhUE-1" {
			sh := testRecord(writer.StageShorten, "S"+a, "\nShort body.\n", "claude-opus-5-5", genIDs[a])
			if got := writeRecord(t, work, a, sh); got != shortenIDs[a] {
				t.Fatalf("fixture: shorten copy id differs")
			}
		}
	}
	writeFile(t, filepath.Join(f.Root, "approval"), t0.Add(-time.Minute).Format(time.RFC3339)+"\tC-sol,C-astra,A-sol,A-astra\t30\n")
	writeJudgments(t, filepath.Join(f.Root, "score", "1", "judgments.tsv"), []string{"C-sol", "A-sol"}, nil)
	var err error
	f.Bounds, err = boundsFromTestInput()
	if err != nil {
		t.Fatal(err)
	}
	f.Baseline = testBaseline()
	return f
}

// addUSD adds two six-place amounts.
func addUSD(a, b string) string {
	x, _ := parseMicro(a)
	y, _ := parseMicro(b)
	return (x + y).String()
}

// writeJudgments writes a complete judgments file in which every article
// passes, except the changes in edit ("<config> <article> <criterion>" to
// "interp,literal,judge,human_check").
func writeJudgments(t *testing.T, path string, configs []string, edit map[string]string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(judgmentsHeader + "\n")
	for _, c := range configs {
		for _, a := range articles() {
			for _, crit := range allCriteria {
				row := defaultJudgment(crit)
				if e, ok := edit[c+" "+a+" "+crit]; ok {
					row = strings.ReplaceAll(e, ",", "\t")
				}
				if row == "" {
					continue
				}
				fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", a, c, crit, row)
			}
		}
	}
	writeFile(t, path, b.String())
}

// defaultJudgment is a passing judgment of one criterion, with the human
// check the rules require.
func defaultJudgment(crit string) string {
	switch {
	case crit == critLength:
		return "pass\tpass\tscript\tno"
	case crit == critEvidence:
		return "pass\tpass\tllm\tyes"
	case slices.Contains(requiredCriteria, crit):
		return "pass\tpass\tllm\tno"
	case slices.Contains(qualityCriteria, crit):
		return "3\t3\tllm\tno"
	default:
		return "no\t-\tllm\tyes"
	}
}

// boundsFromTestInput computes the bounds table from the committed-style
// test input.
func boundsFromTestInput() (*table, error) {
	b, err := computeBounds(testBoundsInput(), mustConfigsNoT())
	if err != nil {
		return nil, err
	}
	return boundsTable(b), nil
}

// testBaseline is a baseline with 8 passes, 0.361554 USD, and 1:53.4.
func testBaseline() baseline {
	return baseline{
		PassInterp: 8, PassLiteral: 7, CostAvgUSD: "0.361554", costAvg: 361554, DurationAvgMs: 113400,
		RequiredFailByVideo: map[string]int{"roNZ-rLxhUE": 0, "18tbwwPM91A": 0, "zW1RBacSqjk": 0, "leE02uqgaKg": 1, "EqnGQDU71-Y": 0},
		Constraints:         map[string]bool{critConstraint1: true, critConstraint2: true, critConstraint3: true},
	}
}

// load reads the fixture as aggregate does.
func (f evalFixture) load(t *testing.T) (evalData, error) {
	t.Helper()
	d := evalData{Configs: f.Configs, Judg: map[int]judgments{}, Checks: map[articleKey]articleCheck{}, Baseline: f.Baseline}
	var err error
	if d.Runs, err = loadRuns0087(f.Root, f.Configs); err != nil {
		return evalData{}, err
	}
	if d.D18, err = load0018(f.Base0018); err != nil {
		return evalData{}, err
	}
	if d.Bounds, err = runBounds(f.Bounds); err != nil {
		return evalData{}, err
	}
	d.Prepay, err = optional(filepath.Join(f.Root, "prepayment.tsv"), parsePrepayment, map[runKey]bool{})
	if err != nil {
		return evalData{}, err
	}
	if d.Approvals, err = optional(filepath.Join(f.Root, "approval"), parseApprovals, nil); err != nil {
		return evalData{}, err
	}
	js, err := optional(filepath.Join(f.Root, "score", "1", "judgments.tsv"), parseJudgments, nil)
	if err != nil {
		return evalData{}, err
	}
	if js != nil {
		if err := js.checkComplete(obtained(d.Runs, f.Configs.ids(1))); err != nil {
			return evalData{}, err
		}
		d.Judg[1] = js
	}
	return d, d.checkPrepayment()
}

// mustTables aggregates the fixture.
func (f evalFixture) mustTables(t *testing.T) []*table {
	t.Helper()
	d, err := f.load(t)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := aggregateTables(d)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// cell returns the cell of the table row whose keys are keys.
func cell(t *testing.T, ts []*table, name, column string, keys ...string) string {
	t.Helper()
	tb, ok := findTable(ts, name)
	if !ok {
		t.Fatalf("no table %s", name)
	}
	col := slices.Index(tb.Columns, column)
	for _, row := range tb.Rows {
		if slices.Equal(row[:len(keys)], keys) {
			return row[col]
		}
	}
	t.Fatalf("table %s has no row %v", name, keys)
	return ""
}

// TestAggregateComparison checks the comparison values of the positive
// control: costs and durations of successful runs only, the composed duration
// of a resumed config (B3's generation, plus B3's shorten for the article
// resumed after shorten), and the passes by both readings.
func TestAggregateComparison(t *testing.T) {
	f := newEvalFixture(t)
	ts := f.mustTables(t)
	// C-sol: costs 0.150000..0.159000, walls 1:10.0..1:19.0.
	for col, want := range map[string]string{
		"usable": "10", "not_obtained": "0", "pass_interp": "10", "pass_literal": "10",
		"quality_3": "60", "cost_avg_usd": "0.154500", "cost_max_usd": "0.159000", "cost_max_article": "EqnGQDU71-Y-2",
		"duration_avg": "1:14.5", "duration_max": "1:19.0", "failed_runs": "0",
	} {
		if got := cell(t, ts, "comparison", col, "C-sol"); got != want {
			t.Errorf("C-sol %s = %s, want %s", col, got, want)
		}
	}
	// A-sol: B3g wall 1:0i.00 plus the run's 20 s; roNZ-rLxhUE-1 (i = 0) adds
	// B3's 47 s shorten time.
	if got := cell(t, ts, "run-costs", "duration", "A-sol", "roNZ-rLxhUE-1", attemptCurrent); got != "2:07.0" {
		t.Errorf("A-sol roNZ-rLxhUE-1 duration = %s, want 1:00 + 0:20 + 0:47 = 2:07.0", got)
	}
	if got := cell(t, ts, "run-costs", "duration", "A-sol", "roNZ-rLxhUE-2", attemptCurrent); got != "1:21.0" {
		t.Errorf("A-sol roNZ-rLxhUE-2 duration = %s, want 1:01 + 0:20", got)
	}
	if got := cell(t, ts, "resume-identity", "off_procedure", "A-sol", "roNZ-rLxhUE-1"); got != "no" {
		t.Errorf("identical artifacts must keep to the procedure")
	}
	if got := cell(t, ts, "stage-usage", "reused_from", "A-sol", "zW1RBacSqjk-1", attemptCurrent, stageGeneration); got != "B3:zW1RBacSqjk-1" {
		t.Errorf("reused_from = %s", got)
	}
	if got := cell(t, ts, "spend", "known_usd", "record"); got != "1.945000" {
		t.Errorf("spend = %s, want C-sol 1.545000 + A-sol 0.400000 (reused stages excluded)", got)
	}
	if got := cell(t, ts, "run-conditions", "value", "all_kept_conditions"); got != "yes" {
		t.Errorf("run conditions = %s", got)
	}
}

// TestAggregateOffProcedure takes a resumed article off the comparison when
// its shorten ran or a reused identifier differs.
func TestAggregateOffProcedure(t *testing.T) {
	f := newEvalFixture(t)
	writeFile(t, filepath.Join(f.Root, "log", "A-sol", "zW1RBacSqjk-1.txt"), logText([]stageLines{
		{Stage: stageGeneration, Origin: originReused, USD: "0.300000", Model: "claude-opus-5-5"},
		{Stage: stageShorten, Origin: originRun, USD: "0.200000", Model: "claude-opus-5-5"},
		{Stage: stageReview, Origin: originRun, USD: "0.040000", Model: "gpt-6.1-sol"},
	}, "known usd=0.240000", "known usd=0.540000", "20.0"))
	writeRecord(t, filepath.Join(f.Root, "work", "A-sol", "leE02uqgaKg-2"), "leE02uqgaKg-2",
		testRecord(writer.StageGeneration, "Other", "\nOther body.\n", "claude-opus-5-5", writer.Digest("x")))
	ts := f.mustTables(t)
	if got := cell(t, ts, "resume-identity", "shorten_ran", "A-sol", "zW1RBacSqjk-1"); got != "yes" {
		t.Errorf("shorten_ran = %s", got)
	}
	if got := cell(t, ts, "resume-identity", "generation_match", "A-sol", "leE02uqgaKg-2"); got != "no" {
		t.Errorf("generation_match = %s", got)
	}
	if got := cell(t, ts, "comparison", "usable", "A-sol"); got != "8" {
		t.Errorf("usable = %s, want 8", got)
	}
}

// TestAggregateFailures records a failed run's stage and cost, leaves a
// confirmed prepayment run out of the record's spend and the retries but not
// out of launch's spend, and flags an article retried more than twice.
func TestAggregateFailures(t *testing.T) {
	f := newEvalFixture(t)
	for n := 1; n <= 3; n++ {
		writeRun(t, f.Root, fakeRun{
			Config: "C-sol", Article: "18tbwwPM91A-1", Attempt: fmt.Sprintf("f%d", n), Start: t0, Exit: 1,
			Log: logText([]stageLines{{Stage: stageGeneration, Origin: originRun, CW: 17300, Out: 6080, USD: "0.104000", Model: "gpt-6.1-sol"}}, "known usd=0.104000", "known usd=0.104000", "1:00.0"),
		})
	}
	writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "zW1RBacSqjk-2", Attempt: "f1", Start: t0, Exit: 2, Log: "yt2column: config error\n"})
	ts := f.mustTables(t)
	if got := cell(t, ts, "failed-runs", "failed_stage", "C-sol", "18tbwwPM91A-1", "f1"); got != stageGeneration {
		t.Errorf("failed_stage = %s", got)
	}
	if got := cell(t, ts, "failed-runs", "prepayment_candidate", "C-sol", "zW1RBacSqjk-2", "f1"); got != "yes" {
		t.Errorf("prepayment_candidate = %s", got)
	}
	if got := cell(t, ts, "retries", "over_limit", "C-sol", "18tbwwPM91A-1"); got != "yes" {
		t.Errorf("three retries must exceed the limit of two")
	}
	if got := cell(t, ts, "retries", "retries", "C-sol", "zW1RBacSqjk-2"); got != "1" {
		t.Errorf("an unconfirmed prepayment run counts as a retry: got %s", got)
	}
	if got := cell(t, ts, "comparison", "failed_usd", "C-sol"); got != "0.312000" {
		t.Errorf("failed_usd = %s", got)
	}
	launchBefore := cell(t, ts, "spend", "total_usd", "launch")

	writeFile(t, filepath.Join(f.Root, "prepayment.tsv"), "C-sol\tzW1RBacSqjk-2\tf1\n")
	ts = f.mustTables(t)
	if tb, _ := findTable(ts, "retries"); slices.ContainsFunc(tb.Rows, func(r []string) bool { return r[1] == "zW1RBacSqjk-2" }) {
		t.Errorf("a confirmed prepayment run must not count as a retry")
	}
	if got := cell(t, ts, "spend", "excluded_runs", "record"); got != "1" {
		t.Errorf("record excluded_runs = %s", got)
	}
	if got := cell(t, ts, "spend", "total_usd", "launch"); got != launchBefore {
		t.Errorf("launch spend changed with the prepayment list: %s -> %s", launchBefore, got)
	}
	if got := cell(t, ts, "spend", "excluded_runs", "launch"); got != "0" {
		t.Errorf("launch must exclude nothing")
	}

	writeFile(t, filepath.Join(f.Root, "prepayment.tsv"), "C-sol\t18tbwwPM91A-1\tf1\n")
	if _, err := f.load(t); !errors.Is(err, errAggregate) {
		t.Errorf("a prepayment entry for a run that called an LLM: err = %v", err)
	}
}

// TestAggregateJudgments counts passes by both readings and the quality
// distribution, and lists what needs a quote.
func TestAggregateJudgments(t *testing.T) {
	f := newEvalFixture(t)
	writeJudgments(t, filepath.Join(f.Root, "score", "1", "judgments.tsv"), []string{"C-sol", "A-sol"}, map[string]string{
		"C-sol roNZ-rLxhUE-1 evidence":    "pass,fail,llm,yes",
		"C-sol zW1RBacSqjk-2 structure":   "1,2,human,yes",
		"C-sol EqnGQDU71-Y-1 coverage":    "2,2,llm,yes",
		"C-sol leE02uqgaKg-2 constraint1": "yes,-,llm,yes",
	})
	ts := f.mustTables(t)
	for col, want := range map[string]string{"pass_interp": "9", "pass_literal": "9", "quality_1": "1", "quality_2": "1", "quality_3": "58"} {
		if got := cell(t, ts, "comparison", col, "C-sol"); got != want {
			t.Errorf("C-sol %s = %s, want %s", col, got, want)
		}
	}
	if got := cell(t, ts, "quotes-needed", "reading", "C-sol", "roNZ-rLxhUE-1", critEvidence); got != "literal" {
		t.Errorf("reading = %s", got)
	}
	if got := cell(t, ts, "quotes-needed", "kind", "C-sol", "zW1RBacSqjk-2", critStructure); got != "quality-1" {
		t.Errorf("kind = %s", got)
	}
	if got := cell(t, ts, "judges", critStructure, "C-sol", "zW1RBacSqjk-2"); got != judgeHuman {
		t.Errorf("judge = %s", got)
	}
	if got := cell(t, ts, "constraints-fixed", critConstraint1, "C-sol"); got != "no" {
		t.Errorf("constraint1 fixed = %s", got)
	}
	if got := cell(t, ts, "constraints-fixed", critConstraint2, "C-sol"); got != "yes" {
		t.Errorf("constraint2 fixed = %s", got)
	}
}

// TestPreApprovalRuns lists a run that started before the first approval, not
// one that started at it, and reads the first line of an appended record.
func TestPreApprovalRuns(t *testing.T) {
	f := newEvalFixture(t)
	writeFile(t, filepath.Join(f.Root, "approval"),
		t0.Format(time.RFC3339)+"\tC-sol,A-sol\t30\n"+t0.Add(2*time.Hour).Format(time.RFC3339)+"\tC-sol,A-sol\t40\n")
	writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "roNZ-rLxhUE-2", Attempt: "f1", Start: t0.Add(-time.Second), Exit: 2, Log: "x\n"})
	ts := f.mustTables(t)
	tb, _ := findTable(ts, "pre-approval-runs")
	if len(tb.Rows) != 1 || tb.Rows[0][1] != "roNZ-rLxhUE-2" || tb.Rows[0][2] != "f1" {
		t.Errorf("pre-approval runs = %v; want only the run one second before", tb.Rows)
	}
}

// TestJudgmentsRejected rejects a file that lacks a judgment, or lacks a human
// check the rules require.
func TestJudgmentsRejected(t *testing.T) {
	for name, edit := range map[string]map[string]string{
		"missing criterion":       {"C-sol roNZ-rLxhUE-1 notation": ""},
		"unchecked evidence pass": {"C-sol roNZ-rLxhUE-1 evidence": "pass,pass,llm,no"},
		"unchecked constraint":    {"C-sol roNZ-rLxhUE-1 constraint2": "no,-,llm,no"},
		"unchecked fail":          {"C-sol roNZ-rLxhUE-1 format_rules": "fail,fail,llm,no"},
		"length by the llm":       {"C-sol roNZ-rLxhUE-1 length": "pass,pass,llm,yes"},
		"notation by the script":  {"C-sol roNZ-rLxhUE-1 notation": "pass,pass,script,no"},
		"bad quality value":       {"C-sol roNZ-rLxhUE-1 coverage": "4,3,llm,yes"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newEvalFixture(t)
			writeJudgments(t, filepath.Join(f.Root, "score", "1", "judgments.tsv"), []string{"C-sol", "A-sol"}, edit)
			if _, err := f.load(t); !errors.Is(err, errJudgments) {
				t.Errorf("err = %v, want errJudgments", err)
			}
		})
	}
}
