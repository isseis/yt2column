package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	f.Req = launchRequest{
		Root: f.Root, RunScript: stub, CLI: "/bin/false", Stage1Dir: filepath.Join(f.Root, "score", "1"),
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
	if len(calls) != 10 || !strings.HasPrefix(calls[0], "run C-sol ") || !strings.Contains(calls[0], " /bin/false "+launchTokenFlag+" ") {
		t.Errorf("calls = %q", calls)
	}
	for _, c := range calls {
		_, rest, _ := strings.Cut(c, " "+launchTokenFlag+" ")
		token, lock, _ := strings.Cut(rest, " lock=")
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
	cases := []struct {
		name string
		edit func(t *testing.T, f *launchFixture)
	}{
		{"no approval", func(_ *testing.T, f *launchFixture) { _ = os.Remove(filepath.Join(f.Root, "approval")) }},
		{"broken approval", func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, "approval"), "yesterday\tC-sol\t30\n")
		}},
		{"config not approved", func(_ *testing.T, f *launchFixture) { f.Req.Config = "B-astra" }},
		{"the last approval drops the config", func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol\t30\n"+
				t0.Add(time.Minute).Format(time.RFC3339)+"\tC-astra\t30\n")
		}},
		{"limit one micro below the reservation", func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol\t"+(10*cSolRunBound-1).String()+"\n")
		}},
		{"a log whose spend cannot be read", func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{Config: "C-astra", Article: "roNZ-rLxhUE-1", Start: t0, Exit: 1, Log: "yt2column: cost: broken\n"})
		}},
		{"a run that has not finished", func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{Config: "C-astra", Article: "roNZ-rLxhUE-1", Start: t0, Unfinished: true, Log: ""})
		}},
		{"another launch holds the lock", func(t *testing.T, f *launchFixture) {
			writeFile(t, filepath.Join(f.Root, launchLockName), "other")
		}},
		{"now before the last approval", func(_ *testing.T, f *launchFixture) { f.Req.Now = t0.Add(-time.Second) }},
		{"a target article already has a current run", func(t *testing.T, f *launchFixture) {
			writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "zW1RBacSqjk-1", Start: t0, Exit: 0, Log: "x\n"})
		}},
		{"a third retry", func(t *testing.T, f *launchFixture) {
			for _, n := range []string{"f1", "f2"} {
				writeRun(t, f.Root, fakeRun{Config: "C-sol", Article: "zW1RBacSqjk-1", Attempt: n, Start: t0, Exit: 2, Log: "x\n"})
			}
			writeFile(t, filepath.Join(f.Root, "approval"), t0.Format(time.RFC3339)+"\tC-sol\t30\n")
			f.Req.Retry = "zW1RBacSqjk-1"
		}},
		{"B-sol while a stage-1 config has not run", func(t *testing.T, f *launchFixture) {
			for _, c := range []string{"C-sol", "C-astra", "A-sol"} {
				f.finishConfig(t, c, t0)
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), []string{"C-sol", "C-astra", "A-sol"}, nil)
			writeFile(t, filepath.Join(f.Req.Stage1Dir, stage1ConfirmedName), t0.Format(time.RFC3339)+"\n")
			f.Req.Config = "B-sol"
		}},
		{"stale bounds table", func(_ *testing.T, f *launchFixture) { f.Req.BoundsInput.ArticlePartMaxBytes++ }},
		{"an unpriced model", func(t *testing.T, f *launchFixture) {
			c, err := parseConfigTable(strings.NewReader(strings.Replace(testConfigTable, "gpt-6.1-sol+medium\tfull", "gpt-6.1-sole+medium\tfull", 1)))
			if err != nil {
				t.Fatal(err)
			}
			f.Req.Configs = c
		}},
		{"A-sol within 30 minutes of C-sol", func(t *testing.T, f *launchFixture) {
			f.finishConfig(t, "C-sol", f.Req.Now.Add(-cacheGap+time.Second))
			f.Req.Config = "A-sol"
		}},
		{"B-sol before the stage-1 judgments are confirmed", func(t *testing.T, f *launchFixture) {
			for _, c := range []string{"C-sol", "C-astra", "A-sol", "A-astra"} {
				f.finishConfig(t, c, t0)
			}
			writeJudgments(t, filepath.Join(f.Req.Stage1Dir, "judgments.tsv"), []string{"C-sol", "C-astra", "A-sol", "A-astra"}, nil)
			f.Req.Config = "B-sol"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newLaunchFixture(t)
			c.edit(t, &f)
			calls, err := f.launch(t, f.Req)
			if !errors.Is(err, errLaunch) || len(calls) != 0 {
				t.Errorf("err = %v, calls = %d; want errLaunch and no run started", err, len(calls))
			}
		})
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
	t.Run("an unknown cost counts the run at its bound", func(t *testing.T) {
		f := newLaunchFixture(t)
		writeRun(t, f.Root, fakeRun{
			Config: "C-sol", Article: "zW1RBacSqjk-1", Attempt: "f1", Start: t0, Exit: 1,
			Log: logText([]stageLines{{Stage: stageReview, Origin: originRun, USD: "unknown", Model: "m"}}, "unknown", "unknown", "1.0"),
		})
		writeFile(t, filepath.Join(f.Root, "approval"), fmt.Sprintf("%s\tC-sol\t%s\n", t0.Format(time.RFC3339), 2*cSolRunBound-1))
		f.Req.Retry = "zW1RBacSqjk-1"
		if calls, err := f.launch(t, f.Req); !errors.Is(err, errLaunch) || len(calls) != 0 {
			t.Errorf("the unknown run must count as one bound: err = %v, calls = %d", err, len(calls))
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
