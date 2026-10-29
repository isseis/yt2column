//go:build test

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/codexreview"
	"github.com/isseis/yt2column/internal/codexreview/testutil"
)

// childModeEnv, when set, makes the test binary run main instead of the tests:
// TestSignalStopsCodexProcessGroup starts it so, as the installed tool would
// run, with the environment the test gives it.
const childModeEnv = "CODEXREVIEW_TEST_CHILD"

// TestMain first checks whether this process is a codexreview child started by
// the signal test, and if so runs main. Otherwise codexreviewtestutil.Main
// makes a start under the name codex the fake codex, and runs the tests with a
// PATH on which the real codex cannot be found.
func TestMain(m *testing.M) {
	if _, ok := os.LookupEnv(childModeEnv); ok {
		main()
		return
	}
	os.Exit(codexreviewtestutil.Main(m))
}

const (
	fakeVersion   = "codex-cli 0.163.0"
	noFindings    = "The change adds one file. No defects were found.\n"
	turnCompleted = `{"type":"turn.completed","usage":{"input_tokens":0}}`
)

// fixture is a repository whose BASE..HEAD adds a.go, with the fake codex on
// PATH and the test's current directory at the worktree root.
type fixture struct {
	fake *codexreviewtestutil.Fake
	repo *codexreviewtestutil.Repo
	base string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := codexreviewtestutil.NewFake(t)
	repo := codexreviewtestutil.NewRepo(t)
	base := repo.Head()
	repo.Write("a.go", "package a\n")
	repo.CommitAll("add a.go")
	t.Chdir(repo.Dir)
	return fixture{fake: fake, repo: repo, base: base}
}

// noFindingsBehavior is a review that completes and reports no findings.
func noFindingsBehavior() codexreviewtestutil.Behavior {
	message := noFindings
	return codexreviewtestutil.Behavior{Version: fakeVersion, Events: []string{turnCompleted}, LastMessage: &message}
}

// runTool runs the tool in process and returns its exit code and output.
func runTool(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, os.Environ(), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// decodeOne decodes the one JSON object of out, failing the test when out is
// not exactly one object.
func decodeOne(t *testing.T, out string) codexreview.Result {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(out)))
	dec.DisallowUnknownFields()
	var r codexreview.Result
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("output %q holds more than one JSON value (%v)", out, err)
	}
	return r
}

func protocolArg() string { return strconv.Itoa(codexreview.Protocol) }

// TestRunRejectsArguments: a command line other than --protocol N --base SHA,
// each once, is refused as usage before codex starts, even --version.
func TestRunRejectsArguments(t *testing.T) {
	fx := newFixture(t)
	fx.fake.Set(noFindingsBehavior())
	p := protocolArg()
	for _, tc := range []struct {
		name   string
		args   []string
		detail string
	}{
		{"no_arguments", nil, "bad_arguments"},
		{"protocol_missing", []string{"--base", fx.base}, "bad_arguments"},
		{"base_missing", []string{"--protocol", p}, "bad_arguments"},
		{"unknown_flag", []string{"--protocol", p, "--base", fx.base, "--model", "m"}, "bad_arguments"},
		{"protocol_twice", []string{"--protocol", p, "--protocol", p, "--base", fx.base}, "bad_arguments"},
		{"base_twice", []string{"--protocol", p, "--base", fx.base, "--base", fx.base}, "bad_arguments"},
		// Four arguments, the right count, with one flag twice.
		{"protocol_twice_no_base", []string{"--protocol", p, "--protocol", p}, "bad_arguments"},
		{"base_twice_no_protocol", []string{"--base", fx.base, "--base", fx.base}, "bad_arguments"},
		{"positional_argument", []string{"--protocol", p, "--base", fx.base, "extra"}, "bad_arguments"},
		{"help", []string{"-h"}, "bad_arguments"},
		{"single_dash", []string{"-protocol", p, "-base", fx.base}, "bad_arguments"},
		{"equals_form", []string{"--protocol=" + p, "--base=" + fx.base}, "bad_arguments"},
		{"terminator", []string{"--protocol", p, "--base", fx.base, "--"}, "bad_arguments"},
		{"protocol_mismatch", []string{"--protocol", strconv.Itoa(codexreview.Protocol + 1), "--base", fx.base}, "protocol_mismatch"},
		{"protocol_not_a_number", []string{"--protocol", "x", "--base", fx.base}, "protocol_mismatch"},
		// A newer document may add a flag; the stale install is still named.
		{"protocol_mismatch_with_new_flag", []string{"--protocol", strconv.Itoa(codexreview.Protocol + 1), "--base", fx.base, "--new", "v"}, "protocol_mismatch"},
		// The base's form is Run's check (internal/codexreview); this row
		// shows the value reaches it unchanged.
		{"base_ref", []string{"--protocol", p, "--base", "main"}, "bad_arguments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, _ := runTool(t, tc.args...)
			if code != exitFailure {
				t.Errorf("exit %d, want %d", code, exitFailure)
			}
			r := decodeOne(t, out)
			if r.Kind != codexreview.KindUsage || r.Detail != tc.detail || r.Protocol != codexreview.Protocol {
				t.Errorf("result = %s/%q protocol %d, want usage/%q protocol %d", r.Kind, r.Detail, r.Protocol, tc.detail, codexreview.Protocol)
			}
			if starts := fx.fake.Starts(); len(starts) != 0 {
				t.Errorf("codex was started: %q", starts)
			}
		})
	}
}

// TestExitCodes: findings and no_findings exit 0, state_changed exits 3, and
// every other kind, an unknown one included, exits 1.
func TestExitCodes(t *testing.T) {
	for _, tc := range []struct {
		kind codexreview.Kind
		want int
	}{
		{codexreview.KindFindings, 0},
		{codexreview.KindNoFindings, 0},
		{codexreview.KindStateChanged, 3},
		{codexreview.KindUsage, 1},
		{codexreview.KindPrecondition, 1},
		{codexreview.KindCodexNotFound, 1},
		{codexreview.KindTimeout, 1},
		{codexreview.KindCodexFailed, 1},
		{codexreview.KindIncomplete, 1},
		{codexreview.KindOutputTooLarge, 1},
		{codexreview.KindOutputEmpty, 1},
		{codexreview.KindUnreadable, 1},
		{"unknown", 1},
		{"", 1},
	} {
		if got := exitCode(tc.kind); got != tc.want {
			t.Errorf("exitCode(%q) = %d, want %d", tc.kind, got, tc.want)
		}
	}
}

// TestOutputIsOneJSONObject: a run that reaches codex prints exactly one
// JSON object carrying the binary's protocol, with the exit code of its kind:
// 0 for a completed review, 3 when codex changed the git state.
func TestOutputIsOneJSONObject(t *testing.T) {
	for _, tc := range []struct {
		name string
		git  [][]string
		kind codexreview.Kind
		code int
	}{
		{"reviewed", nil, codexreview.KindNoFindings, exitReviewed},
		{"state_changed", [][]string{{"branch", "made-by-codex"}}, codexreview.KindStateChanged, exitStateChanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t)
			b := noFindingsBehavior()
			b.Git, b.GitDir = tc.git, fx.repo.Dir
			fx.fake.Set(b)
			code, out, stderr := runTool(t, "--protocol", protocolArg(), "--base", fx.base)
			if code != tc.code {
				t.Fatalf("exit %d, want %d (%s)", code, tc.code, stderr)
			}
			r := decodeOne(t, out)
			if r.Kind != tc.kind || r.Protocol != codexreview.Protocol {
				t.Errorf("result = kind %s, protocol %d; want kind %s, protocol %d", r.Kind, r.Protocol, tc.kind, codexreview.Protocol)
			}
		})
	}
}

// startTool starts the test binary as the tool in the fixture's repository.
func startTool(t *testing.T, fx fixture) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "--protocol", protocolArg(), "--base", fx.base) //nolint:gosec // the test binary, run as the tool
	cmd.Dir = fx.repo.Dir
	cmd.Env = append(os.Environ(), childModeEnv+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	// Bounded, so a tool that ignores the signal cannot hang the test.
	cmd.WaitDelay = 30 * time.Second
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd, &stdout, &stderr
}

// waitTool waits for the tool to exit and checks that it ended with exit 1
// and no result.
func waitTool(t *testing.T, cmd *exec.Cmd, stdout, stderr *bytes.Buffer) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the tool did not exit after the signal")
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); !ok || exitErr.ExitCode() != exitFailure {
		t.Errorf("tool ended with %v, want exit status %d (%s)", err, exitFailure, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want no result", stdout.String())
	}
}

// TestSignalStopsCodexProcessGroup runs the test binary as the tool, waits
// until the fake codex has started its grandchild, and sends the tool
// SIGTERM: the grandchild, in codex's process group, stops too, and the tool
// exits 1 with no result, since nothing changed and nobody waits for the
// review.
func TestSignalStopsCodexProcessGroup(t *testing.T) {
	fx := newFixture(t)
	pidFile := filepath.Join(fx.fake.Dir, "grandchild.pid")
	b := noFindingsBehavior()
	b.GrandchildPIDFile = pidFile
	b.Hang = true
	fx.fake.Set(b)

	cmd, stdout, stderr := startTool(t, fx)
	grandchild := codexreviewtestutil.ReadPID(t, pidFile)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitTool(t, cmd, stdout, stderr)
	if !codexreviewtestutil.WaitGone(grandchild) {
		t.Errorf("grandchild %d still runs after the tool stopped", grandchild)
	}
}

// TestSecondSignalStillStopsCodex: a fake codex that ignores SIGTERM outlives
// the first signal's request to stop, and a second SIGTERM reaches the tool
// during the grace before the group is killed. The tool must not end then:
// it kills codex when the grace is over and only then exits 1 with no
// result.
func TestSecondSignalStillStopsCodex(t *testing.T) {
	fx := newFixture(t)
	pidFile := filepath.Join(fx.fake.Dir, "codex.pid")
	b := noFindingsBehavior()
	b.PIDFile = pidFile
	b.IgnoreTerm = true
	b.Hang = true
	fx.fake.Set(b)

	cmd, stdout, stderr := startTool(t, fx)
	codex := codexreviewtestutil.ReadPID(t, pidFile)
	for range 2 {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		// Well inside the grace, so the second signal comes before the kill.
		time.Sleep(200 * time.Millisecond)
	}
	waitTool(t, cmd, stdout, stderr)
	if !codexreviewtestutil.WaitGone(codex) {
		t.Errorf("codex %d still runs after the tool stopped", codex)
	}
}
