package main

import (
	"slices"
	"testing"
	"time"
)

// candidateStats is a config that is a candidate by (ii) and (iii) against
// testBaseline: 8 passes, no per-video failure over the baseline, an average
// cost just below the baseline's, and the baseline's average duration.
func candidateStats() configStats {
	return configStats{
		Config: "C-sol", Usable: 10, Judged: true, PassInterp: 8, PassLiteral: 7,
		FailsByVideo: map[string]int{"leE02uqgaKg": 1},
		CostSum:      361554*10 - 1,
		DurSum:       113400 * 10 * time.Millisecond,
	}
}

// TestJudgeF006 checks each condition at its boundary against one positive
// control, changing one value at a time.
func TestJudgeF006(t *testing.T) {
	b := testBaseline()
	base := judgeF006(candidateStats(), b)
	if !base.CondII || !base.CondIII || base.CondI || !base.Candidate {
		t.Fatalf("positive control = %+v; want (ii), (iii), and a candidate", base)
	}
	cases := []struct {
		name          string
		edit          func(*configStats)
		i, ii, iii, c bool
	}{
		{"two more passes is (i)", func(s *configStats) { s.PassInterp = 10; s.CostSum = 361554 * 20 }, true, false, true, true},
		{"one more pass at a higher cost is not (i)", func(s *configStats) { s.PassInterp = 9; s.CostSum = 361554 * 20 }, false, false, true, false},
		{"a cost equal to the baseline is not lower", func(s *configStats) { s.CostSum = 361554 * 10 }, false, false, true, false},
		{"one pass fewer fails (ii)", func(s *configStats) { s.PassInterp = 7 }, false, false, true, false},
		{"a video with more required failures fails (ii)", func(s *configStats) { s.FailsByVideo["roNZ-rLxhUE"] = 1 }, false, false, true, false},
		{"exactly three times the cost meets (iii)", func(s *configStats) { s.PassInterp = 10; s.CostSum = 3 * 361554 * 10 }, true, false, true, true},
		{"over three times the cost fails (iii)", func(s *configStats) { s.PassInterp = 10; s.CostSum = 3*361554*10 + 1 }, true, false, false, false},
		{"exactly twice the duration meets (iii)", func(s *configStats) { s.DurSum = 2 * 113400 * 10 * time.Millisecond }, false, true, true, true},
		{"over twice the duration fails (iii)", func(s *configStats) { s.DurSum = (2*113400*10 + 1) * time.Millisecond }, false, true, false, false},
		{"a missing article fails (ii) and (iii)", func(s *configStats) { s.Usable = 9; s.PassInterp = 10 }, true, false, false, false},
		{"an unknown cost fails (ii) and (iii)", func(s *configStats) { s.CostUnknown = true }, false, false, false, false},
	}
	for _, c := range cases {
		s := candidateStats()
		s.FailsByVideo = map[string]int{"leE02uqgaKg": 1}
		c.edit(&s)
		r := judgeF006(s, b)
		if r.CondI != c.i || r.CondII != c.ii || r.CondIII != c.iii || r.Candidate != c.c {
			t.Errorf("%s: (i) %v (ii) %v (iii) %v candidate %v; want %v %v %v %v",
				c.name, r.CondI, r.CondII, r.CondIII, r.Candidate, c.i, c.ii, c.iii, c.c)
		}
	}
}

// TestJudgeF006AuxChanges reports when the auxiliary cost would change (ii).
func TestJudgeF006AuxChanges(t *testing.T) {
	s := candidateStats()
	s.HasCacheRead, s.AuxCostSum = true, s.CostSum
	if judgeF006(s, testBaseline()).AuxChanges {
		t.Errorf("an equal auxiliary cost changes nothing")
	}
	s.AuxCostSum = 361554 * 10
	if !judgeF006(s, testBaseline()).AuxChanges {
		t.Errorf("an auxiliary cost at the baseline turns (ii) off and must be reported")
	}
}

// TestSelectF006 picks the most passes, then the lowest cost, skips an
// excluded candidate, and reports a tie on both.
func TestSelectF006(t *testing.T) {
	mk := func(id string, pass int, cost micro) f006Result {
		return f006Result{Stats: configStats{Config: id, PassInterp: pass, CostSum: cost}, Candidate: true}
	}
	results := []f006Result{mk("C-sol", 9, 100), mk("A-sol", 10, 300), mk("A-astra", 10, 200), {Stats: configStats{Config: "B-sol", PassInterp: 10}}}
	if got := selectF006(results, nil); got != "A-astra" {
		t.Errorf("select = %s, want the cheaper of the two with 10 passes", got)
	}
	if got := selectF006(results, []string{"A-astra"}); got != "A-sol" {
		t.Errorf("select after excluding A-astra = %s", got)
	}
	if got := selectF006(results, []string{"A-astra", "A-sol", "C-sol"}); got != "none" {
		t.Errorf("select with no candidate left = %s", got)
	}
	if got := selectF006(append(results, mk("C-astra", 10, 200)), nil); got != "tie:A-astra,C-astra" {
		t.Errorf("select on a tie = %s", got)
	}
}

// TestSelectB makes B with the C model that passed more, or both on a tie,
// B-sol first.
func TestSelectB(t *testing.T) {
	for _, c := range []struct {
		astra, sol int
		want       []string
	}{
		{7, 6, []string{configBAstra}},
		{6, 7, []string{configBSol}},
		{7, 7, []string{configBSol, configBAstra}},
	} {
		if got := selectB(c.astra, c.sol).Configs; !slices.Equal(got, c.want) {
			t.Errorf("selectB(%d, %d) = %v, want %v", c.astra, c.sol, got, c.want)
		}
	}
}
