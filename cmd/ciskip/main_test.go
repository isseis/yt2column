//go:build test

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/ciskip"
)

// fakeEvaluator returns a fixed Decision and records the Input it was given.
type fakeEvaluator struct {
	decision ciskip.Decision
	inputs   []ciskip.Input
}

func (f *fakeEvaluator) Evaluate(_ context.Context, in ciskip.Input) ciskip.Decision {
	f.inputs = append(f.inputs, in)
	return f.decision
}

// runTool runs the command with a buffer for each output stream.
func runTool(args []string, eval evaluator) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, eval, &out, &errOut)
	return code, out.String(), errOut.String()
}

// fullArgs is a complete command line with resultFile as its only variable.
func fullArgs(resultFile string) []string {
	return []string{
		"--repo=isseis/yt2column",
		"--head-repo=isseis/yt2column",
		"--base-repo=isseis/yt2column",
		"--base-ref=main",
		"--pr-number=1080",
		"--head-sha=0123456789abcdef0123456789abcdef01234567",
		"--run-attempt=1",
		"--dep-dirs=/tmp/wasm-dep-dirs",
		"--wasm-dep-dirs-ok=true",
		"--result-file=" + resultFile,
	}
}

// resultPath returns a result-file path below a fresh temporary directory.
func resultPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "ciskip-result")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

const (
	parentSHA = "1111111111111111111111111111111111111111"
	baseSHA   = "2222222222222222222222222222222222222222"
)

// TestRunWritesDecision checks the result file holds exactly the decision, the
// notice appears only when a base SHA was recorded, and the one diagnostic
// line carries the declared reason and its bounded identifiers.
func TestRunWritesDecision(t *testing.T) {
	notice := "::notice title=ci-base-sha::" + baseSHA + "\n"
	for _, tc := range []struct {
		name       string
		decision   ciskip.Decision
		wantResult string
		wantStdout string
	}{
		{
			"skip",
			ciskip.Decision{
				Reason:        ciskip.ReasonSkipped,
				ParentSHA:     parentSHA,
				ParentRunID:   42,
				NoticeBaseSHA: baseSHA,
			},
			"skip=true\n",
			notice + "ciskip: decision=skip reason=ok parent=111111111111 run=42 base=222222222222\n",
		},
		{
			"run after recording the base",
			ciskip.Decision{Reason: ciskip.ReasonCodeChanged, NoticeBaseSHA: baseSHA},
			"skip=false\n",
			notice + "ciskip: decision=run reason=code_changed\n",
		},
		{
			"run with a parent run and an HTTP status",
			ciskip.Decision{Reason: ciskip.ReasonAPIFailed, ParentRunID: 42, HTTPStatus: 500},
			"skip=false\n",
			"ciskip: decision=run reason=api_failed run=42 status=500\n",
		},
		{
			"run without a recorded base",
			ciskip.Decision{Reason: ciskip.ReasonMergeCommitMismatch},
			"skip=false\n",
			"ciskip: decision=run reason=merge_commit_mismatch\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resultFile := resultPath(t)
			code, stdout, stderr := runTool(fullArgs(resultFile), &fakeEvaluator{decision: tc.decision})
			if code != exitEvaluated {
				t.Errorf("run = %d, want %d (stderr %q)", code, exitEvaluated, stderr)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want none", stderr)
			}
			if stdout != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tc.wantStdout)
			}
			if got := readFile(t, resultFile); got != tc.wantResult {
				t.Errorf("result file = %q, want %q", got, tc.wantResult)
			}
		})
	}
}

// TestRunRejectsCommandLine checks a command line the tool cannot read exits 2
// with the fixed usage line, evaluates nothing, and prints no flag value.
func TestRunRejectsCommandLine(t *testing.T) {
	const canary = "ghp_doNotPrintThis"
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"unknown flag", []string{"--unknown=" + canary}},
		{"missing flag value", []string{"--result-file"}},
		{"empty result file", []string{"--result-file="}},
		{"extra positional argument", append(fullArgs(resultPath(t)), "extra")},
		{"missing result file", fullArgs("")[:len(fullArgs(""))-1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eval := &fakeEvaluator{}
			code, stdout, stderr := runTool(tc.args, eval)
			if code != exitUsage {
				t.Errorf("run = %d, want %d", code, exitUsage)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want none", stdout)
			}
			if stderr != usage+"\n" {
				t.Errorf("stderr = %q, want the fixed usage line", stderr)
			}
			if strings.Contains(stdout+stderr, canary) {
				t.Errorf("output echoes a flag value: %q", stdout+stderr)
			}
			if len(eval.inputs) != 0 {
				t.Errorf("Evaluate called %d times, want none", len(eval.inputs))
			}
		})
	}
}

// TestRunResultFileWriteFailure checks an unwritable result file exits 2 with
// one fixed line that names no path, after the decision is reported.
func TestRunResultFileWriteFailure(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "missing")
	resultFile := filepath.Join(missingDir, "ciskip-result")
	eval := &fakeEvaluator{decision: ciskip.Decision{Reason: ciskip.ReasonCodeChanged}}

	code, stdout, stderr := runTool(fullArgs(resultFile), eval)
	if code != exitUsage {
		t.Errorf("run = %d, want %d", code, exitUsage)
	}
	if stderr != writeFailureLine+"\n" {
		t.Errorf("stderr = %q, want %q", stderr, writeFailureLine+"\n")
	}
	if strings.Contains(stderr, missingDir) || strings.Contains(stderr, "ciskip-result") {
		t.Errorf("stderr names the result path: %q", stderr)
	}
	wantStdout := "ciskip: decision=run reason=code_changed\n"
	if stdout != wantStdout {
		t.Errorf("stdout = %q, want %q", stdout, wantStdout)
	}
}

// TestRunMapsFlagsToInput checks every flag reaches its own Input field, so a
// swapped pair of names cannot silently evaluate the wrong value.
func TestRunMapsFlagsToInput(t *testing.T) {
	eval := &fakeEvaluator{decision: ciskip.Decision{Reason: ciskip.ReasonInvalidInput}}
	if code, _, stderr := runTool(fullArgs(resultPath(t)), eval); code != exitEvaluated {
		t.Fatalf("run = %d, want %d (stderr %q)", code, exitEvaluated, stderr)
	}
	if len(eval.inputs) != 1 {
		t.Fatalf("Evaluate called %d times, want once", len(eval.inputs))
	}
	want := ciskip.Input{
		Repo:          "isseis/yt2column",
		HeadRepo:      "isseis/yt2column",
		BaseRepo:      "isseis/yt2column",
		BaseRef:       "main",
		PRNumber:      "1080",
		HeadSHA:       "0123456789abcdef0123456789abcdef01234567",
		RunAttempt:    "1",
		DepDirsFile:   "/tmp/wasm-dep-dirs",
		WasmDepDirsOK: "true",
	}
	if got := eval.inputs[0]; got != want {
		t.Errorf("Input = %+v, want %+v", got, want)
	}
}

// TestRunEvaluatesEmptyContext checks missing context values are an ordinary
// evaluation, not a usage error: the evaluator fails safe and the exit code is
// 0, so the workflow falls back to its normal classification.
func TestRunEvaluatesEmptyContext(t *testing.T) {
	resultFile := resultPath(t)
	code, stdout, stderr := runTool([]string{"--result-file=" + resultFile}, ciskip.New(nil))
	if code != exitEvaluated {
		t.Errorf("run = %d, want %d (stderr %q)", code, exitEvaluated, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want none", stderr)
	}
	wantStdout := "ciskip: decision=run reason=fork_or_repo_mismatch\n"
	if stdout != wantStdout {
		t.Errorf("stdout = %q, want %q", stdout, wantStdout)
	}
	if got := readFile(t, resultFile); got != "skip=false\n" {
		t.Errorf("result file = %q, want %q", got, "skip=false\n")
	}
}

// TestRunDoesNotEchoFlagValues checks the notice and the diagnostic line carry
// no flag value, so a secret or an unverified argument cannot reach the log.
func TestRunDoesNotEchoFlagValues(t *testing.T) {
	const canary = "ghp_doNotPrintThis"
	resultFile := resultPath(t)
	args := fullArgs(resultFile)
	args[7] = "--dep-dirs=/" + canary
	eval := &fakeEvaluator{decision: ciskip.Decision{
		Reason:        ciskip.ReasonSkipped,
		ParentSHA:     parentSHA,
		ParentRunID:   42,
		NoticeBaseSHA: baseSHA,
	}}

	code, stdout, stderr := runTool(args, eval)
	if code != exitEvaluated {
		t.Errorf("run = %d, want %d", code, exitEvaluated)
	}
	if strings.Contains(stdout+stderr, canary) {
		t.Errorf("output echoes a flag value: %q", stdout+stderr)
	}
	if strings.Count(stdout, "\n") != 2 {
		t.Errorf("stdout = %q, want the notice and one diagnostic line", stdout)
	}
}
