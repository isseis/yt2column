//go:build test

package main

import (
	"errors"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/cachelock"
	transcripttestutil "github.com/isseis/yt2column/internal/transcript/testutil"
)

const (
	// goneBound bounds a wait for a recorded PID to disappear.
	goneBound = 30 * time.Second
	// unlockBound bounds a wait for the lock to be released after the
	// stopping yt-dlp is told to exit.
	unlockBound = 30 * time.Second
	// ignoredSignalWindow is how long TestIgnoredSignalNotSubscribed waits for
	// a wrongly subscribed signal to end the child.
	ignoredSignalWindow = 2 * time.Second
	// resendInterval paces repeated signals in TestSecondSignal.
	resendInterval = 100 * time.Millisecond
)

// childSetup is the environment of one CLI child: the env map it runs with
// and its paths.
type childSetup struct {
	env      map[string]string
	cacheDir string
	outPath  string
	ready    string // created by the child's fake LLM client or publisher when it waits
	ctxDone  string
}

// newChildSetup returns a child environment with a seeded cache when seed is
// set and ytDlpPath as YT2COLUMN_YTDLP_PATH.
func newChildSetup(t *testing.T, ytDlpPath string, seed bool) *childSetup {
	t.Helper()
	base := t.TempDir()
	c := &childSetup{
		cacheDir: filepath.Join(base, "cache"),
		outPath:  filepath.Join(base, "article.md"),
		ready:    filepath.Join(base, "ready"),
		ctxDone:  filepath.Join(base, "llm-ctx-done"),
	}
	if seed {
		seedCache(t, c.cacheDir, runVideoID, validSubtitles, infoFor(runVideoID))
	}
	c.env = map[string]string{
		"YT2COLUMN_MODEL":      "fake-model",
		"DEEPSEEK_API_KEY":     testAPIKey,
		"SLACK_WEBHOOK_URL":    testWebhook,
		"YT2COLUMN_CACHE_DIR":  c.cacheDir,
		"YT2COLUMN_YTDLP_PATH": ytDlpPath,
		childReadyEnv:          c.ready,
		childCtxDoneEnv:        c.ctxDone,
	}
	return c
}

// args returns the CLI arguments of a child.
func (c *childSetup) args() []string {
	return []string{"--out", c.outPath, runVideoURL}
}

// requireExitFailure asserts the child exited normally with exitFailure.
func requireExitFailure(t *testing.T, state *os.ProcessState, child *cliChild) {
	t.Helper()
	if state.ExitCode() != exitFailure {
		t.Fatalf("exit status = %v, want exit code %d\nstderr:\n%s", state, exitFailure, child.stderr.String())
	}
}

// requireInterruptedChildOutput checks the output of a child stopped by a
// signal: an empty standard output, the interruption reported, and no secret
// or raw escape character. Standard error is checked by content only: a
// coverage run can add the runtime's own warnings to it. A caller running with
// --out also checks that the file was not created.
func requireInterruptedChildOutput(t *testing.T, child *cliChild) {
	t.Helper()
	if child.stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", child.stdout.String())
	}
	stderr := child.stderr.String()
	if !strings.Contains(stderr, "interrupted") {
		t.Errorf("stderr does not report the interruption:\n%s", stderr)
	}
	for _, s := range secretStrings() {
		if strings.Contains(stderr, s) {
			t.Error("stderr holds a secret or its tail")
		}
	}
	if strings.Contains(stderr, "\x1b") {
		t.Errorf("stderr holds a raw escape character:\n%q", stderr)
	}
}

// TestSignalDuringYtDlp sends SIGINT and SIGTERM to the production main while
// yt-dlp runs, and checks the exit code, the output, that --out is not
// created, and that neither the fake yt-dlp nor its child outlives the CLI.
func TestSignalDuringYtDlp(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			skipIfIgnored(t, sig)
			stopping := transcripttestutil.NewStopping(t, t.TempDir())
			// A seeded cache and --refresh: yt-dlp still runs, and the
			// cache must survive the interruption.
			setup := newChildSetup(t, stopping.Script, true)
			before := videoEntries(t, setup.cacheDir)
			child := startCLIChild(t, childModeMain, setup.env, append([]string{"--refresh"}, setup.args()...)...)

			waitForFile(t, stopping.Ready)
			pids := []int{readPID(t, stopping.PID), readPID(t, stopping.ChildPID)}
			child.signal(t, sig)
			state := child.wait(t)

			requireExitFailure(t, state, child)
			requireInterruptedChildOutput(t, child)
			requireNoFile(t, setup.outPath)
			if !strings.Contains(child.stderr.String(), "the transcript stage failed") {
				t.Errorf("stderr does not name the transcript stage:\n%s", child.stderr.String())
			}
			for _, pid := range pids {
				waitForProcessGone(t, pid, goneBound)
			}
			if after := videoEntries(t, setup.cacheDir); !slices.Equal(before, after) {
				t.Errorf("the video's cache changed: before %v, after %v", before, after)
			}
		})
	}
}

// TestSignalDuringGenerate sends SIGINT and SIGTERM while the LLM call is in
// progress, and checks the exit code, the output, that --out is not created,
// and that the video's cache is kept.
func TestSignalDuringGenerate(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			skipIfIgnored(t, sig)
			tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())
			setup := newChildSetup(t, tripwire, true)
			before := videoEntries(t, setup.cacheDir)
			child := startCLIChild(t, childModeStall, setup.env, setup.args()...)

			waitForFile(t, setup.ready)
			child.signal(t, sig)
			state := child.wait(t)

			requireExitFailure(t, state, child)
			requireInterruptedChildOutput(t, child)
			requireNoFile(t, setup.outPath)
			requireNoFile(t, marker)
			if after := videoEntries(t, setup.cacheDir); !slices.Equal(before, after) {
				t.Errorf("the video's cache changed: before %v, after %v", before, after)
			}
		})
	}
}

// TestSignalDuringSlackPost sends SIGINT and SIGTERM while a --slack post is
// in flight, and checks the exit code, the output, that yt-dlp is not
// started, and that the video's cache is kept. The child's publisher returns
// what SlackWebhookPublisher returns for a canceled first message.
func TestSignalDuringSlackPost(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			skipIfIgnored(t, sig)
			tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())
			setup := newChildSetup(t, tripwire, true)
			before := videoEntries(t, setup.cacheDir)
			child := startCLIChild(t, childModeSlackStall, setup.env, "--slack", runVideoURL)

			waitForFile(t, setup.ready)
			child.signal(t, sig)
			state := child.wait(t)

			requireExitFailure(t, state, child)
			requireInterruptedChildOutput(t, child)
			for _, want := range []string{"the publish stage failed", "posted 0 of 1 messages"} {
				if !strings.Contains(child.stderr.String(), want) {
					t.Errorf("stderr does not contain %q:\n%s", want, child.stderr.String())
				}
			}
			requireNoFile(t, marker)
			if after := videoEntries(t, setup.cacheDir); !slices.Equal(before, after) {
				t.Errorf("the video's cache changed: before %v, after %v", before, after)
			}
		})
	}
}

// TestSIGKILLWhileYtDlpRuns kills the CLI while yt-dlp runs. The orphaned
// yt-dlp keeps the inherited lock, so a second run fails fast without touching
// anything; once yt-dlp exits, the lock is free and a run succeeds.
func TestSIGKILLWhileYtDlpRuns(t *testing.T) {
	stopping := transcripttestutil.NewStopping(t, t.TempDir())
	setup := newChildSetup(t, stopping.Script, false)
	child := startCLIChild(t, childModeMain, setup.env, setup.args()...)
	waitForFile(t, stopping.Ready)
	child.signal(t, syscall.SIGKILL)
	child.wait(t)

	// The second run, while the orphaned yt-dlp still runs.
	e := newRunEnv(t)
	e.cacheDir = setup.cacheDir
	e.env["YT2COLUMN_CACHE_DIR"] = setup.cacheDir
	cacheBefore := snapshotTree(t, setup.cacheDir)
	if code := e.run(); code != exitFailure {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, e.stderr.String())
	}
	stderr := e.stderr.String()
	lockPath := filepath.Join(setup.cacheDir, lockFileName)
	for _, want := range []string{"another run holds the cache directory lock", lockPath, "yt-dlp was running", "lsof"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, stderr)
		}
	}
	if after := snapshotTree(t, setup.cacheDir); !maps.Equal(cacheBefore, after) {
		t.Errorf("the cache directory changed:\nbefore %v\nafter  %v", cacheBefore, after)
	}
	requireNoFile(t, e.tripwireMarker)
	requireNoFile(t, e.outPath)
	if got := e.counter.count(); got != 0 {
		t.Errorf("Generate calls = %d, want 0", got)
	}

	// Release the orphan; the lock is freed once every holder has exited.
	writeMarker(stopping.Release)
	waitForLockFree(t, setup.cacheDir)

	seedCache(t, setup.cacheDir, runVideoID, validSubtitles, infoFor(runVideoID))
	final := newRunEnv(t)
	final.cacheDir = setup.cacheDir
	final.env["YT2COLUMN_CACHE_DIR"] = setup.cacheDir
	if code := final.run(); code != exitOK {
		t.Fatalf("exit code after yt-dlp exited = %d, want %d\nstderr:\n%s", code, exitOK, final.stderr.String())
	}
}

// waitForLockFree waits until the cache directory lock can be taken.
func waitForLockFree(t *testing.T, cacheDir string) {
	t.Helper()
	deadline := time.Now().Add(unlockBound)
	for time.Now().Before(deadline) {
		lock, err := cachelock.Acquire(cacheDir)
		if err == nil {
			_ = lock.Close()
			return
		}
		if !errors.Is(err, cachelock.ErrLocked) {
			t.Fatalf("Acquire: %v", err)
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("the lock in %s was not released within %v", cacheDir, unlockBound)
}

// TestSIGKILLWithoutYtDlp kills the CLI while it holds the lock but runs no
// yt-dlp; the lock dies with it, so the next run succeeds.
func TestSIGKILLWithoutYtDlp(t *testing.T) {
	tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
	setup := newChildSetup(t, tripwire, true)
	child := startCLIChild(t, childModeStall, setup.env, setup.args()...)
	waitForFile(t, setup.ready)
	if lock, err := cachelock.Acquire(setup.cacheDir); !errors.Is(err, cachelock.ErrLocked) {
		if err == nil {
			_ = lock.Close()
		}
		t.Fatalf("Acquire while the child generates = %v, want ErrLocked: the child must hold the lock when it is killed", err)
	}
	child.signal(t, syscall.SIGKILL)
	child.wait(t)

	e := newRunEnv(t)
	e.cacheDir = setup.cacheDir
	e.env["YT2COLUMN_CACHE_DIR"] = setup.cacheDir
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
}

// TestSubscribedSignals checks that SIGINT and SIGHUP are each subscribed
// only when not ignored at startup, and SIGTERM always.
func TestSubscribedSignals(t *testing.T) {
	for _, intIgnored := range []bool{false, true} {
		for _, hupIgnored := range []bool{false, true} {
			ignored := func(sig os.Signal) bool {
				return (sig == os.Interrupt && intIgnored) || (sig == syscall.SIGHUP && hupIgnored)
			}
			got := subscribedSignals(ignored)
			if !slices.Contains(got, os.Signal(syscall.SIGTERM)) {
				t.Errorf("SIGINT ignored = %v, SIGHUP ignored = %v: subscribed %v, want SIGTERM", intIgnored, hupIgnored, got)
			}
			if slices.Contains(got, os.Interrupt) == intIgnored {
				t.Errorf("SIGINT ignored = %v: subscribed %v", intIgnored, got)
			}
			if slices.Contains(got, os.Signal(syscall.SIGHUP)) == hupIgnored {
				t.Errorf("SIGHUP ignored = %v: subscribed %v", hupIgnored, got)
			}
		}
	}
}

// TestIgnoredSignalNotSubscribed starts the CLI (through runWithSignals) with
// SIGINT or SIGHUP ignored, as a shell's background job or nohup does, and checks that
// the signal does not interrupt the run: the child must still be generating
// after the signal. Only a wrongly subscribed signal could end it within the
// window, so a slow machine cannot make the test fail.
func TestIgnoredSignalNotSubscribed(t *testing.T) {
	for _, tc := range []struct {
		trapName string
		sig      syscall.Signal
	}{
		{"INT", syscall.SIGINT},
		{"HUP", syscall.SIGHUP},
	} {
		t.Run(tc.sig.String(), func(t *testing.T) {
			tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
			setup := newChildSetup(t, tripwire, true)
			child := startCLIChildIgnoring(t, tc.trapName, childModeStall, setup.env, setup.args()...)
			waitForFile(t, setup.ready)
			child.signal(t, tc.sig)

			time.Sleep(ignoredSignalWindow)
			if child.exited() {
				t.Fatalf("the child exited after an ignored %v; stderr:\n%s", tc.sig, child.stderr.String())
			}
		})
	}
}

// skipIfIgnored skips the test when this process was started with sig
// ignored: a child inherits that, so the CLI rightly does not subscribe to it.
func skipIfIgnored(t *testing.T, sig syscall.Signal) {
	t.Helper()
	if signal.Ignored(sig) {
		t.Skipf("this test process ignores %v (for example as a background job or under nohup), so a child inherits that and does not subscribe to it", sig)
	}
}

// TestSIGHUP sends SIGHUP to a child started with SIGHUP not ignored.
func TestSIGHUP(t *testing.T) {
	skipIfIgnored(t, syscall.SIGHUP)
	tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
	setup := newChildSetup(t, tripwire, true)
	child := startCLIChild(t, childModeStall, setup.env, setup.args()...)
	waitForFile(t, setup.ready)
	child.signal(t, syscall.SIGHUP)
	state := child.wait(t)

	requireExitFailure(t, state, child)
	requireInterruptedChildOutput(t, child)
	requireNoFile(t, setup.outPath)
}

// TestSecondSignal stops the CLI in an LLM client that ignores its context.
// After the first SIGTERM ends the context, a later SIGTERM must terminate the
// process by the default disposition. SIGTERM is used because it is
// subscribed even when this test process was started with SIGINT ignored.
func TestSecondSignal(t *testing.T) {
	skipIfIgnored(t, syscall.SIGTERM)
	tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
	setup := newChildSetup(t, tripwire, true)
	child := startCLIChild(t, childModeIgnoreCtx, setup.env, setup.args()...)
	waitForFile(t, setup.ready)
	child.signal(t, syscall.SIGTERM)
	waitForFile(t, setup.ctxDone)

	// The subscription is dropped just after the context ends, so repeat the
	// signal until the default disposition applies.
	deadline := time.Now().Add(exitBound)
	for !child.exited() && time.Now().Before(deadline) {
		_ = child.cmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(resendInterval)
	}
	state := child.wait(t)
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Fatalf("exit status = %v, want termination by SIGTERM\nstderr:\n%s", state, child.stderr.String())
	}
}
