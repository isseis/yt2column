//go:build test

package main

import (
	"errors"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	// stillRunningWindow is how long TestSignalWaitsForJob checks that the
	// child keeps running after the first signal while its job is blocked.
	stillRunningWindow = 500 * time.Millisecond
	// resendInterval paces repeated signals in TestSecondSignalExitsAtOnce.
	resendInterval = 100 * time.Millisecond
)

// skipIfIgnored skips when this test process ignores sig, which a child
// inherits, so the child would not subscribe to it.
func skipIfIgnored(t *testing.T, sig syscall.Signal) {
	t.Helper()
	if signal.Ignored(sig) {
		t.Skipf("this test process ignores %v (for example as a background job or under nohup), so a child inherits that and does not subscribe to it", sig)
	}
}

// blockingChild starts a server child whose LLM client blocks, sends it one
// accepted request, and returns once the job is inside the LLM call, with the
// path that releases it and the child's socket path.
func blockingChild(t *testing.T) (child *serverChild, releasePath, socketPath string) {
	t.Helper()
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	release := filepath.Join(dir, "release")
	e := newServerEnv(t)
	child = startServerChild(t, childModeBlock, e, map[string]string{childReadyEnv: ready, childReleaseEnv: release})
	mustPostArticle(t, e.socketPath(t), requestBody(testValidRaw), 202)
	waitForFile(t, ready)
	return child, release, e.socketPath(t)
}

// TestSignalStopsServer sends SIGINT, SIGTERM, and SIGHUP to the production
// main while no job runs, and checks that it stops with exitOK and wrote
// nothing to standard output. Which signals signalrun subscribes is its own
// test's; all three are sent here so a main that handled signals itself, with
// a narrower set, would fail.
func TestSignalStopsServer(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			skipIfIgnored(t, sig)
			child := startServerChild(t, childModeMain, newServerEnv(t), nil)
			child.signal(t, sig)
			if state := child.wait(t); state.ExitCode() != exitOK {
				t.Fatalf("exit status = %v, want exit code %d\nstderr:\n%s", state, exitOK, child.stderr.String())
			}
			if out := child.stdout.String(); out != "" {
				t.Errorf("stdout = %q, want nothing written to it", out)
			}
		})
	}
}

// TestSignalWaitsForJob checks that the first signal does not end an
// in-flight job: the child keeps running until the job finishes, then exits
// with exitOK, and its socket is gone. That the log counts the pending job is
// internal/server's TestServeShutdownWaitsForJobs.
func TestSignalWaitsForJob(t *testing.T) {
	skipIfIgnored(t, syscall.SIGTERM)
	child, release, socket := blockingChild(t)
	child.signal(t, syscall.SIGTERM)

	time.Sleep(stillRunningWindow)
	if child.exited() {
		t.Fatalf("the child exited while its job was blocked\nstderr:\n%s", child.stderr.String())
	}
	writeMarker(release)
	state := child.wait(t)
	if state.ExitCode() != exitOK {
		t.Fatalf("exit status = %v, want exit code %d\nstderr:\n%s", state, exitOK, child.stderr.String())
	}
	stderr := child.stderr.String()
	if !strings.Contains(stderr, "job succeeded") {
		t.Errorf("stderr does not report the job's success:\n%s", stderr)
	}
	if _, err := os.Lstat(socket); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the socket %s is still there after a normal stop: %v", socket, err)
	}
}

// TestSecondSignalExitsAtOnce checks that a second signal ends the child by
// the signal's default disposition while its job is still blocked. It repeats
// cmd/yt2column's TestSecondSignal on purpose: there the CLI's job is
// canceled, here the server's job deliberately is not, so this is the case
// the second signal exists for.
func TestSecondSignalExitsAtOnce(t *testing.T) {
	skipIfIgnored(t, syscall.SIGTERM)
	child, _, _ := blockingChild(t)
	child.signal(t, syscall.SIGTERM)

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
