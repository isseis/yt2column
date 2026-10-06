//go:build test

package main

import (
	"errors"
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
	// resendInterval paces repeated signals in TestSecondSignal.
	resendInterval = 100 * time.Millisecond
)

// childSetup is the environment of one CLI child: the env map it runs with
// and its paths.
type childSetup struct {
	env      map[string]string
	cacheDir string
	outPath  string
	llmReady string
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
		llmReady: filepath.Join(base, "llm-ready"),
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
		childReadyEnv:          c.llmReady,
		childCtxDoneEnv:        c.ctxDone,
	}
	return c
}

// args returns the CLI arguments of a child.
func (c *childSetup) args() []string {
	return []string{"--out", c.outPath, runVideoURL}
}

// requireExitCode asserts the child exited normally with want.
func requireExitCode(t *testing.T, state *os.ProcessState, want int, child *cliChild) {
	t.Helper()
	if state.ExitCode() != want {
		t.Fatalf("exit status = %v, want exit code %d\nstderr:\n%s", state, want, child.stderr.String())
	}
}

// requireInterruptedChildOutput checks the output of a child stopped by a
// signal: an empty standard output, the interruption reported, and no secret
// or raw escape character. Standard error is checked by content only: a
// coverage run can add the runtime's own warnings to it.
func requireInterruptedChildOutput(t *testing.T, child *cliChild, outPath string) {
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
	requireNoFile(t, outPath)
}

// TestSignalDuringYtDlp sends SIGINT and SIGTERM to the production main while
// yt-dlp runs, and checks the exit code, the output, that --out is not
// created, and that neither the fake yt-dlp nor its child outlives the CLI.
func TestSignalDuringYtDlp(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			stopping := transcripttestutil.NewStopping(t, t.TempDir())
			setup := newChildSetup(t, stopping.Script, false)
			child := startCLIChild(t, childModeMain, setup.env, setup.args()...)

			waitForFile(t, stopping.Ready)
			pids := []int{readPID(t, stopping.PID), readPID(t, stopping.ChildPID)}
			child.signal(t, sig)
			state := child.wait(t)

			requireExitCode(t, state, exitFailure, child)
			requireInterruptedChildOutput(t, child, setup.outPath)
			if !strings.Contains(child.stderr.String(), "the transcript stage failed") {
				t.Errorf("stderr does not name the transcript stage:\n%s", child.stderr.String())
			}
			for _, pid := range pids {
				waitForProcessGone(t, pid, goneBound)
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
			tripwire, marker := transcripttestutil.NewTripwire(t, t.TempDir())
			setup := newChildSetup(t, tripwire, true)
			before := videoEntries(t, setup.cacheDir)
			child := startCLIChild(t, childModeStall, setup.env, setup.args()...)

			waitForFile(t, setup.llmReady)
			child.signal(t, sig)
			state := child.wait(t)

			requireExitCode(t, state, exitFailure, child)
			requireInterruptedChildOutput(t, child, setup.outPath)
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
	if after := snapshotTree(t, setup.cacheDir); !equalSnapshots(cacheBefore, after) {
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
	waitForFile(t, setup.llmReady)
	child.signal(t, syscall.SIGKILL)
	child.wait(t)

	e := newRunEnv(t)
	e.cacheDir = setup.cacheDir
	e.env["YT2COLUMN_CACHE_DIR"] = setup.cacheDir
	if code := e.run(); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, e.stderr.String())
	}
}

// TestSubscribedSignals checks that SIGHUP is subscribed only when it was not
// ignored at startup, and SIGINT and SIGTERM always.
func TestSubscribedSignals(t *testing.T) {
	for _, hupIgnored := range []bool{false, true} {
		ignored := func(sig os.Signal) bool { return sig == syscall.SIGHUP && hupIgnored }
		got := subscribedSignals(ignored)
		if !slices.Contains(got, os.Interrupt) || !slices.Contains(got, os.Signal(syscall.SIGTERM)) {
			t.Errorf("SIGHUP ignored = %v: subscribed %v, want SIGINT and SIGTERM", hupIgnored, got)
		}
		if slices.Contains(got, os.Signal(syscall.SIGHUP)) == hupIgnored {
			t.Errorf("SIGHUP ignored = %v: subscribed %v", hupIgnored, got)
		}
	}
}

// TestSIGHUP sends SIGHUP to a child started with SIGHUP not ignored.
func TestSIGHUP(t *testing.T) {
	if signal.Ignored(syscall.SIGHUP) {
		t.Skip("this test process ignores SIGHUP (for example under nohup), so a child inherits that and does not subscribe to it")
	}
	tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
	setup := newChildSetup(t, tripwire, true)
	child := startCLIChild(t, childModeStall, setup.env, setup.args()...)
	waitForFile(t, setup.llmReady)
	child.signal(t, syscall.SIGHUP)
	state := child.wait(t)

	requireExitCode(t, state, exitFailure, child)
	requireInterruptedChildOutput(t, child, setup.outPath)
}

// TestSecondSignal stops the CLI in an LLM client that ignores its context.
// After the first SIGINT ends the context, a later SIGINT must terminate the
// process by the default disposition.
func TestSecondSignal(t *testing.T) {
	tripwire, _ := transcripttestutil.NewTripwire(t, t.TempDir())
	setup := newChildSetup(t, tripwire, true)
	child := startCLIChild(t, childModeIgnoreCtx, setup.env, setup.args()...)
	waitForFile(t, setup.llmReady)
	child.signal(t, syscall.SIGINT)
	waitForFile(t, setup.ctxDone)

	// The subscription is dropped just after the context ends, so repeat the
	// signal until the default disposition applies.
	deadline := time.Now().Add(exitBound)
	for !child.exited() && time.Now().Before(deadline) {
		_ = child.cmd.Process.Signal(syscall.SIGINT)
		time.Sleep(resendInterval)
	}
	state := child.wait(t)
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGINT {
		t.Fatalf("exit status = %v, want termination by SIGINT\nstderr:\n%s", state, child.stderr.String())
	}
}
