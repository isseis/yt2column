package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// spendResult is what the runs have spent (design section 3.5, plan section 1.2 item 7).
type spendResult struct {
	// Known is the sum of the run-origin cost lines of the runs whose costs
	// are all known and that wrote "cost-total: run".
	Known micro
	// BoundRuns counts the runs counted at their config's run bound instead:
	// a run with an unknown run-origin cost line, or with no "cost-total: run"
	// line (a run stopped before its summary, or one that never called an
	// LLM, which the log cannot tell apart).
	BoundRuns int
	Bound     micro
	// UnknownStages counts the run-origin cost lines that are unknown.
	UnknownStages int
	// Excluded lists the runs left out because the human confirmed they
	// stopped before any payment, with their run bound.
	Excluded []excludedRun
}

// excludedRun is one run left out of the spend.
type excludedRun struct {
	Key   runKey
	Bound micro
}

// Total is the spend: the known costs plus the bounds.
func (s spendResult) Total() micro { return s.Known + s.Bound }

// errSpend reports a run whose spend cannot be determined.
var errSpend = errors.New("cannot determine the spend")

// computeSpend sums what the runs spent. A reused stage was paid for by B3 and
// is never counted. exclude holds the runs the human confirmed as stopped
// before payment; launch passes none, so it counts every run.
func computeSpend(runs []run, bounds map[string]micro, exclude map[runKey]bool) (spendResult, error) {
	var s spendResult
	for _, r := range runs {
		bound, ok := bounds[r.Config]
		if !ok {
			return spendResult{}, fmt.Errorf("%w: no run bound for config %s", errSpend, r.Config)
		}
		if exclude[r.key()] {
			s.Excluded = append(s.Excluded, excludedRun{Key: r.key(), Bound: bound})
			continue
		}
		known, unknownStages := runCost(r.Log)
		s.UnknownStages += unknownStages
		if unknownStages > 0 || r.Log.TotalRun == nil {
			s.BoundRuns++
			s.Bound += bound
			continue
		}
		s.Known += known
	}
	return s, nil
}

// runCost sums a run's known run-origin cost lines and counts its unknown
// ones.
func runCost(l runLog) (micro, int) {
	var sum micro
	unknown := 0
	for _, c := range l.Costs {
		if c.Origin != originRun {
			continue
		}
		if !c.Known {
			unknown++
			continue
		}
		sum += c.USD
	}
	return sum, unknown
}

// prepaymentCandidate reports whether a finished run called no LLM: it has an
// exit code and no run-origin tokens line. Such a run may have stopped before
// payment (a configuration error or a rejected input), or may have been
// killed after paying (a panic also exits 2), so only a human can confirm it.
func prepaymentCandidate(r run) bool {
	return r.When.HasExit && !r.Log.hasRunTokens()
}

// errPrepayment reports a prepayment list that does not match its form.
var errPrepayment = errors.New("prepayment list does not match its form")

// parsePrepayment reads prepayment.tsv: one run per line,
// "<config>\t<article>\t<attempt>", where attempt is "cur" or "f<N>".
func parsePrepayment(r io.Reader) (map[runKey]bool, error) {
	out := map[runKey]bool{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 3 || !articleRE.MatchString(fields[1]) || //nolint:mnd // three columns
			(fields[2] != attemptCurrent && !strings.HasPrefix(fields[2], "f")) {
			return nil, fmt.Errorf("%w: line %q", errPrepayment, scanner.Text())
		}
		k := runKey{fields[0], fields[1], fields[2]}
		if out[k] {
			return nil, fmt.Errorf("%w: %v appears twice", errPrepayment, k)
		}
		out[k] = true
	}
	return out, scanner.Err()
}
