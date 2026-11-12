package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// launchFixture is a 0087 root, a stub run script that only records its
// arguments, and a request that passes every check.
type launchFixture struct {
	Root, Calls string
	Req         launchRequest
}

// newLaunchFixture writes the stub and an approval of the stage-1 configs and
// B-sol with a 30 USD limit, at t0.
func newLaunchFixture(t *testing.T) launchFixture {
	t.Helper()
	dir := t.TempDir()
	f := launchFixture{Root: filepath.Join(dir, "0087"), Calls: filepath.Join(dir, "calls")}
	stub := filepath.Join(dir, "run.sh")
	// The stub records its arguments, the token it was handed, and the lock's
	// token at the time it ran.
	writeFile(t, stub, "#!/bin/sh\necho \"$* lock=$(cat "+filepath.Join(f.Root, launchLockName)+")\" >> "+f.Calls+"\n")
	if err := os.Chmod(stub, 0o700); err != nil { //nolint:gosec // the stub must be executable
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol,C-astra,A-sol,A-astra,B-sol\t30\n")
	in := testBoundsInput()
	bounds, err := computeBounds(in, mustConfigs(t))
	if err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(dir, "yt2column")
	writeFile(t, cli, "pinned binary")
	sum := sha256.Sum256([]byte("pinned binary"))
	f.Req = launchRequest{
		Root: f.Root, RunScript: stub, CLI: cli, CLISHA256: hex.EncodeToString(sum[:]), Stage1Dir: filepath.Join(f.Root, "score", "1"),
		Config: "C-sol", Now: t0.Add(time.Hour), BoundsInput: in, Bounds: boundsTable(bounds), Configs: mustConfigs(t),
	}
	return f
}

// launch runs the checks and, when they pass, the stub; it returns the stub's
// calls.
func (f launchFixture) launch(t *testing.T, q launchRequest) ([]string, error) {
	t.Helper()
	_, err := launchRuns(q)
	if err != nil && !errors.Is(err, errLaunch) {
		t.Fatal(err)
	}
	return f.calls(t), err
}

// calls returns the stub's recorded calls.
func (f launchFixture) calls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(f.Calls)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// finishConfig writes ten finished, successful runs of cfg that ended at end,
// each costing 0.010000.
func (f launchFixture) finishConfig(t *testing.T, cfg string, end time.Time) {
	t.Helper()
	for _, a := range articles() {
		writeRun(t, f.Root, fakeRun{
			Config: cfg, Article: a, Start: end.Add(-time.Minute), End: end, Exit: 0,
			Log: logText([]stageLines{{Stage: stageReview, Origin: originRun, USD: "0.010000", Model: "m"}},
				"known usd=0.010000", "known usd=0.010000", "1.0"),
		})
	}
}

// cSolRunBound is C-sol's run bound in testBoundsInput: 0.0125 + 0.01375 +
// 0.0145.
const cSolRunBound = micro(40750)

// TestLaunchPositive starts the run script once per article, with the config,
// the article, and the CLI, and records the launch.
func TestLaunchPositive(t *testing.T) {
	f := newLaunchFixture(t)
	calls, err := f.launch(t, f.Req)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 10 || !strings.HasPrefix(calls[0], "run C-sol ") || !strings.Contains(calls[0], " "+f.Req.CLI+" "+launchTokenFlag+" ") {
		t.Errorf("calls = %q", calls)
	}
	var started []string
	for _, c := range calls {
		started = append(started, strings.Fields(c)[2])
	}
	slices.Sort(started)
	want := slices.Clone(articles())
	slices.Sort(want)
	if !slices.Equal(started, want) {
		t.Errorf("started %v, want each article once", started)
	}
	for _, c := range calls {
		_, rest, _ := strings.Cut(c, " "+launchTokenFlag+" ")
		_, lock, _ := strings.Cut(rest, " lock=")
		token := strings.Fields(rest)[0]
		if !strings.Contains(rest, " "+evalRootFlag+" "+filepath.Dir(f.Root)+" ") {
			t.Errorf("the run was not handed the evaluation root: %q", c)
		}
		if token == "" || token != lock {
			t.Errorf("the run got token %q while the lock held %q", token, lock)
		}
	}
	if _, err := os.Stat(filepath.Join(f.Root, launchLockName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the lock must be removed after the runs exit: %v", err)
	}
	entries, err := readParsed(filepath.Join(f.Root, launchLogName), parseLaunchLog)
	if err != nil || len(entries) != 1 || entries[0].Config != "C-sol" || len(entries[0].Articles) != 10 {
		t.Errorf("launch log = %+v, %v", entries, err)
	}
}

// TestLaunchStops changes one input of the positive control at a time and
// requires the stub never to start.
func TestLaunchStops(t *testing.T) {
	stage1 := []string{"C-sol", "C-astra", "A-sol", "A-astra"}
	cases := []struct {
		name string
		kind stopKind
		also error // a sentinel the stop must also carry, when one names the cause
		edit func(t *testing.T, f *launchFixture)
	}{
		{"no approval", stopApproval, nil, func(_ *testing.T, f *launchFixture) { _ = os.Remove(filepath.Join(f.Root, "approval")) }},
		{"broken approval", stopApproval, errApproval, func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, "approval"), "yesterday\tC-sol\t30\n")
		}},
		{"stage-1 config not approved", stopApproval, nil, func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol,C-astra,A-sol\t30\n")
			f.Req.Config = "A-astra"
		}},
		{"the last approval drops the config", stopApproval, nil, func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol\t30\n"+
				t0.Add(time.Minute).Format(time.RFC3339)+"\tC-astra\t30\n")
		}},
		{"now before the last approval", stopApproval, nil, func(_ *testing.T, f *launchFixture) { f.Req.Now = t0.Add(-time.Second) }},
		{"limit one micro below the reservation", stopReservation, nil, func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol\t"+(10*cSolRunBound-1).String()+"\n")
		}},
		{"an unknown cost counts the run at its bound", stopReservation, nil, func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{
				Config: "C-sol", Article: "zW1RBacSqjk-1", Attempt: "f1", Start: t0, Exit: 1,
				Log: logText([]stageLines{{Stage: stageReview, Origin: originRun, USD: "unknown", Model: "m"}}, "unknown", "unknown", "1.0"),
			})
			writeFile(t, filepath.Join(f.Root, "approval"), fmt.Sprintf("%s\tC-sol\t%s\n", t0.Format(time.RFC3339), 2*cSolRunBound-1))
			f.Req.Retry = "zW1RBacSqjk-1"
		}},
		{"a log whose spend cannot be read", stopRunRecords, errLogLine, func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{Config: "C-astra", Article: "roNZ-rLxhUE-1", Start: t0, Exit: 1, Log: "yt2column: cost: broken\n"})
		}},
		{"a run that has not finished", stopRunState, nil, func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{Config: "C-astra", Article: "roNZ-rLxhUE-1", Start: t0, Unfinished: true, Log: ""})
		}},
		{"a target article already has a current run", stopRunState, nil, func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "zW1RBacSqjk-1", Start: t0, Exit: 0, Log: "x\n"})
		}},
		{"a root not named 0087", stopInput, nil, func(_ *testing.T, f *launchFixture) { f.Req.Root = filepath.Dir(f.Root) }},
		{"a retry of an article outside the evaluation", stopInput, nil, func(_ *testing.T, f *launchFixture) { f.Req.Retry = "abcdefghijk-1" }},
		{"a CLI other than the pinned binary", stopBinary, nil, func(t *testing.T, f *launchFixture) {
			writeFile(t, f.Req.CLI, "rebuilt binary")
		}},
		{"a run killed before its record closed", stopRunState, nil, func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{Config: "C-astra", Article: "roNZ-rLxhUE-1", Attempt: "f1", Start: t0, Unfinished: true, Log: ""})
		}},
		{"another launch holds the lock", stopLock, nil, func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, launchLockName), "other")
		}},
		{"a third retry", stopRetry, nil, func(t *testing.T, f *launchFixture) {
			for _, n := range []string{"f1", "f2", "f3"} {
				writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "zW1RBacSqjk-1", Attempt: n, Start: t0, Exit: 2, Log: "x\n"})
			}
			f.Req.Retry = "zW1RBacSqjk-1"
		}},
		{"a full launch over an article that used its retries", stopRetry, nil, func(t *testing.T, f *launchFixture) {
			for _, n := range []string{"f1", "f2", "f3"} {
				writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "zW1RBacSqjk-1", Attempt: n, Start: t0, Exit: 2, Log: "x\n"})
			}
		}},
		{"stale bounds table", stopBounds, nil, func(_ *testing.T, f *launchFixture) { f.Req.BoundsInput.ArticlePartMaxBytes++ }},
		{"an unpriced model", stopBounds, errUnpriced, func(t *testing.T, f *launchFixture) {
			c, err := parseConfigTable(strings.NewReader(strings.Replace(testConfigTable, "gpt-6.1-sol+medium\tfull", "gpt-6.1-sole+medium\tfull", 1)))
			if err != nil {
				t.Fatal(err)
			}
			f.Req.Configs = c
			// The unpriced model has every input, so only the price stops it.
			f.Req.BoundsInput.FixedInput["gpt-6.1-sole"] = f.Req.BoundsInput.FixedInput["gpt-6.1-sol"]
			f.Req.BoundsInput.EstimateTokens["gpt-6.1-sole"] = f.Req.BoundsInput.EstimateTokens["gpt-6.1-sol"]
		}},
		{"A-sol within 30 minutes of C-sol", stopCacheGap, nil, func(t *testing.T, f *launchFixture) {
			f.finishConfig(t, "C-sol", f.Req.Now.Add(-cacheGap+time.Second))
			f.Req.Config = "A-sol"
		}},
		{"B-sol before the stage-1 judgments are confirmed", stopStage1, nil, func(t *testing.T, f *launchFixture) {
			for _, c := range stage1 {
				f.finishConfig(t, c, t0)
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), stage1, nil)
			f.Req.Config = "B-sol"
		}},
		{"B-sol with an incomplete stage-1 judgment", stopStage1, errJudgments, func(t *testing.T, f *launchFixture) {
			for _, c := range stage1 {
				f.finishConfig(t, c, t0)
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), stage1, map[string]string{"A-astra roNZ-rLxhUE-1 notation": ""})
			writeFile(t, filepath.Join(f.Req.Stage1Dir, stage1ConfirmedName), t0.Format(time.RFC3339)+"\n")
			f.Req.Config = "B-sol"
		}},
		{"B-sol with a stage-1 run after the confirmation", stopStage1, nil, func(t *testing.T, f *launchFixture) {
			for _, c := range stage1 {
				f.finishConfig(t, c, t0)
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), stage1, nil)
			writeFile(t, filepath.Join(f.Req.Stage1Dir, stage1ConfirmedName), t0.Add(-time.Second).Format(time.RFC3339)+"\n")
			f.Req.Config = "B-sol"
		}},
		{"B-sol with a confirmation after now", stopStage1, nil, func(t *testing.T, f *launchFixture) {
			for _, c := range stage1 {
				f.finishConfig(t, c, t0)
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), stage1, nil)
			writeFile(t, filepath.Join(f.Req.Stage1Dir, stage1ConfirmedName), f.Req.Now.Add(time.Second).Format(time.RFC3339)+"\n")
			f.Req.Config = "B-sol"
		}},
		{"B-sol while a stage-1 article awaits its retry", stopStage1, nil, func(t *testing.T, f *launchFixture) {
			for _, c := range stage1 {
				f.finishConfig(t, c, t0)
			}
			// A-astra's zW1RBacSqjk-1 failed once and was retired, not retried.
			_ = os.Remove(filepath.Join(f.Root, "log", "A-astra", "zW1RBacSqjk-1.txt"))
			_ = os.Remove(filepath.Join(f.Root, "log", "A-astra", "zW1RBacSqjk-1.when"))
			writeRun(t, f.Root, fakeRun{Config: "A-astra", Article: "zW1RBacSqjk-1", Attempt: "f1", Start: t0.Add(-2 * time.Minute), End: t0.Add(-time.Minute), Exit: 1, Log: "x\n"})
			// The judgments leave out the article that was not obtained, so
			// they are complete and only the retry state can stop the launch.
			omit := map[string]string{}
			for _, c := range allCriteria {
				omit["A-astra zW1RBacSqjk-1 "+c] = ""
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), stage1, omit)
			writeFile(t, filepath.Join(f.Req.Stage1Dir, stage1ConfirmedName), t0.Format(time.RFC3339)+"\n")
			f.Req.Config = "B-sol"
		}},
		{"B-sol while a stage-1 config has not run", stopStage1, nil, func(t *testing.T, f *launchFixture) {
			for _, c := range stage1[:3] {
				f.finishConfig(t, c, t0)
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), stage1[:3], nil)
			writeFile(t, filepath.Join(f.Req.Stage1Dir, stage1ConfirmedName), t0.Format(time.RFC3339)+"\n")
			f.Req.Config = "B-sol"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newLaunchFixture(t)
			c.edit(t, &f)
			calls, err := f.launch(t, f.Req)
			s, ok := errors.AsType[*launchStop](err)
			if !ok || s.Kind != c.kind || len(calls) != 0 {
				t.Errorf("err = %v, calls = %d; want a stop of kind %d and no run started", err, len(calls), c.kind)
			}
			if c.also != nil && !errors.Is(err, c.also) {
				t.Errorf("err = %v; want it to carry %v", err, c.also)
			}
		})
	}
}

// TestLaunchClockSkew refuses a -now far from the system clock before it
// reads anything else.
func TestLaunchClockSkew(t *testing.T) {
	err := runLaunch([]string{"-now", time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)}, io.Discard)
	if s, ok := errors.AsType[*launchStop](err); !ok || s.Kind != stopClock {
		t.Errorf("err = %v, want a clock stop", err)
	}
}

// TestLaunchBoundaries starts at the exact limits that still allow it.
func TestLaunchBoundaries(t *testing.T) {
	t.Run("limit equal to the reservation", func(t *testing.T) {
		f := newLaunchFixture(t)
		writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol\t"+(10*cSolRunBound).String()+"\n")
		if calls, err := f.launch(t, f.Req); err != nil || len(calls) != 10 {
			t.Errorf("err = %v, calls = %d", err, len(calls))
		}
	})
	t.Run("A-sol exactly 30 minutes after C-sol", func(t *testing.T) {
		f := newLaunchFixture(t)
		f.finishConfig(t, "C-sol", f.Req.Now.Add(-cacheGap))
		f.Req.Config = "A-sol"
		if calls, err := f.launch(t, f.Req); err != nil || len(calls) != 10 {
			t.Errorf("err = %v, calls = %d", err, len(calls))
		}
	})
	t.Run("C-astra right after C-sol, which shares no model", func(t *testing.T) {
		f := newLaunchFixture(t)
		f.finishConfig(t, "C-sol", f.Req.Now.Add(-time.Second))
		f.Req.Config = "C-astra"
		if calls, err := f.launch(t, f.Req); err != nil || len(calls) != 10 {
			t.Errorf("err = %v, calls = %d", err, len(calls))
		}
	})
	t.Run("a retry reserves one run bound", func(t *testing.T) {
		f := newLaunchFixture(t)
		writeRun(t, f.Root, fakeRun{
			Config: "C-sol", Article: "zW1RBacSqjk-1", Attempt: "f1", Start: t0, Exit: 1,
			Log: logText([]stageLines{{Stage: stageReview, Origin: originRun, USD: "0.010000", Model: "m"}}, "known usd=0.010000", "known usd=0.010000", "1.0"),
		})
		limit := micro(10000) + cSolRunBound
		writeFile(t, filepath.Join(f.Root, "approval"), fmt.Sprintf("%s\tC-sol\t%s\n", t0.Format(time.RFC3339), limit))
		f.Req.Retry = "zW1RBacSqjk-1"
		calls, err := f.launch(t, f.Req)
		if err != nil || len(calls) != 1 || !strings.Contains(calls[0], "zW1RBacSqjk-1") {
			t.Errorf("err = %v, calls = %q", err, calls)
		}
	})
	t.Run("the second retry", func(t *testing.T) {
		f := newLaunchFixture(t)
		for _, n := range []string{"f1", "f2"} {
			writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "zW1RBacSqjk-1", Attempt: n, Start: t0, Exit: 2, Log: "x\n"})
		}
		f.Req.Retry = "zW1RBacSqjk-1"
		if calls, err := f.launch(t, f.Req); err != nil || len(calls) != 1 {
			t.Errorf("err = %v, calls = %d", err, len(calls))
		}
	})
	t.Run("a root given with a trailing slash", func(t *testing.T) {
		f := newLaunchFixture(t)
		f.Req.Root += "/"
		calls, err := f.launch(t, f.Req)
		if err != nil || len(calls) != 10 || !strings.Contains(calls[0], " "+evalRootFlag+" "+filepath.Dir(f.Root)+" ") {
			t.Errorf("err = %v, calls = %q", err, calls)
		}
	})
	t.Run("a run retired unfinished after another config's runs", func(t *testing.T) {
		f := newLaunchFixture(t)
		writeRun(t, f.Root, fakeRun{Config: "C-astra", Article: "roNZ-rLxhUE-1", Attempt: "f1", Start: t0, Unfinished: true, Log: ""})
		path := filepath.Join(f.Root, "failed", "C-astra", "roNZ-rLxhUE-1-f1", "log", "roNZ-rLxhUE-1.when")
		data, err := os.ReadFile(path) //nolint:gosec // a file the test wrote
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, string(data)+"retired_unfinished="+t0.Format(time.RFC3339)+"\n")
		if calls, err := f.launch(t, f.Req); err != nil || len(calls) != 10 {
			t.Errorf("err = %v, calls = %d", err, len(calls))
		}
	})
	t.Run("B-sol after the stage-1 judgments are confirmed", func(t *testing.T) {
		f := newLaunchFixture(t)
		for _, c := range []string{"C-sol", "C-astra", "A-sol", "A-astra"} {
			f.finishConfig(t, c, t0)
		}
		writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), []string{"C-sol", "C-astra", "A-sol", "A-astra"}, nil)
		writeFile(t, filepath.Join(f.Req.Stage1Dir, stage1ConfirmedName), t0.Format(time.RFC3339)+"\n")
		f.Req.Config = "B-sol"
		if calls, err := f.launch(t, f.Req); err != nil || len(calls) != 10 {
			t.Errorf("err = %v, calls = %d", err, len(calls))
		}
	})
}
