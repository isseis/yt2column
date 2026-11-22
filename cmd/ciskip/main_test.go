//go:build test

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
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
// Every flag gets a distinct value, so a binding swapped between two flags
// changes the evaluation input.
func fullArgs(resultFile string) []string {
	return []string{
		"--repo=isseis/yt2column",
		"--head-repo=isseis/yt2column-head",
		"--base-repo=isseis/yt2column-base",
		"--base-ref=trunk",
		"--pr-number=1080",
		"--head-sha=0123456789abcdef0123456789abcdef01234567",
		"--run-attempt=2",
		"--dep-dirs=/tmp/wasm-dep-dirs",
		"--wasm-dep-dirs-ok=false",
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
			ciskip.Decision{Reason: ciskip.ReasonCodeChanged, ParentSHA: parentSHA, NoticeBaseSHA: baseSHA},
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
			"run with a parent run only",
			ciskip.Decision{Reason: ciskip.ReasonAPITimeout, ParentRunID: 42},
			"skip=false\n",
			"ciskip: decision=run reason=api_timeout run=42\n",
		},
		{
			"run with an HTTP status only",
			ciskip.Decision{Reason: ciskip.ReasonAPIFailed, HTTPStatus: 503},
			"skip=false\n",
			"ciskip: decision=run reason=api_failed status=503\n",
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

// withFlagValue returns a copy of args with the named flag's value replaced.
func withFlagValue(args []string, flag, value string) []string {
	out := slices.Clone(args)
	for i, arg := range out {
		if strings.HasPrefix(arg, "--"+flag+"=") {
			out[i] = "--" + flag + "=" + value
		}
	}
	return out
}

// captureProcessStderr redirects the process's standard error to a file and
// returns a reader for what was written. The writers passed to run cannot see
// text the flag package writes to os.Stderr.
func captureProcessStderr(t *testing.T) func() string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("create stderr file: %v", err)
	}
	original := os.Stderr
	os.Stderr = f
	t.Cleanup(func() {
		os.Stderr = original
		_ = f.Close()
	})
	return func() string {
		t.Helper()
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatalf("seek stderr file: %v", err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("read stderr file: %v", err)
		}
		return string(data)
	}
}

// TestRunRejectsCommandLine checks a command line the tool cannot read exits 2
// with one fixed usage line, evaluates nothing, and writes no flag value to
// either standard error.
func TestRunRejectsCommandLine(t *testing.T) {
	const canary = "ghp_doNotPrintThis"
	for _, tc := range []struct {
		name   string
		args   []string
		canary string
	}{
		{"no arguments", nil, ""},
		{"unknown flag", []string{"--unknown=" + canary}, canary},
		{"missing flag value", []string{"--repo=" + canary, "--result-file"}, canary},
		{"empty result file", withFlagValue(fullArgs(""), "repo", canary), canary},
		{"extra positional argument", append(fullArgs(resultPath(t)), canary), canary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readProcessStderr := captureProcessStderr(t)
			eval := &fakeEvaluator{}
			code, stdout, stderr := runTool(tc.args, eval)
			if code != exitUsage {
				t.Errorf("run = %d, want %d", code, exitUsage)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want none", stdout)
			}
			if stderr != usage+"\n" || !strings.HasPrefix(stderr, "usage: ciskip ") || strings.Count(stderr, "\n") != 1 {
				t.Errorf("stderr = %q, want the fixed one-line usage", stderr)
			}
			if got := readProcessStderr(); got != "" {
				t.Errorf("process stderr = %q, want none", got)
			}
			if tc.canary != "" && strings.Contains(stdout+stderr, tc.canary) {
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
	if stderr != "ciskip: the result file could not be written\n" {
		t.Errorf("stderr = %q, want the fixed write-failure line", stderr)
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
		HeadRepo:      "isseis/yt2column-head",
		BaseRepo:      "isseis/yt2column-base",
		BaseRef:       "trunk",
		PRNumber:      "1080",
		HeadSHA:       "0123456789abcdef0123456789abcdef01234567",
		RunAttempt:    "2",
		DepDirsFile:   "/tmp/wasm-dep-dirs",
		WasmDepDirsOK: "false",
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
	if !strings.HasPrefix(stdout, "ciskip: decision=run reason=") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("stdout = %q, want one decision=run diagnostic line", stdout)
	}
	if got := readFile(t, resultFile); got != "skip=false\n" {
		t.Errorf("result file = %q, want %q", got, "skip=false\n")
	}
}

// TestRunDoesNotEchoFlagValues checks the notice and the diagnostic line carry
// no flag value, so a secret or an unverified argument cannot reach the log.
func TestRunDoesNotEchoFlagValues(t *testing.T) {
	flagValues := [][2]string{
		{"repo", "canaryrepo"},
		{"head-repo", "canaryheadrepo"},
		{"base-repo", "canarybaserepo"},
		{"base-ref", "canarybaseref"},
		{"pr-number", "canarypr"},
		{"head-sha", "canarysha"},
		{"run-attempt", "canaryattempt"},
		{"dep-dirs", "canarydepdirs"},
		{"wasm-dep-dirs-ok", "canarywasm"},
	}
	resultFile := resultPath(t)
	args := make([]string, 0, len(flagValues)+1)
	for _, fv := range flagValues {
		args = append(args, "--"+fv[0]+"="+fv[1])
	}
	args = append(args, "--result-file="+resultFile)
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
	for _, fv := range flagValues {
		if strings.Contains(stdout+stderr, fv[1]) {
			t.Errorf("output echoes the value of --%s: %q", fv[0], stdout+stderr)
		}
	}
	if strings.Contains(stdout+stderr, resultFile) {
		t.Errorf("output echoes the value of --result-file: %q", stdout+stderr)
	}
}

// TestShortSHA checks the diagnostic line keeps a short SHA as it is and cuts a
// long one to the 12 characters the line shows.
func TestShortSHA(t *testing.T) {
	for _, tc := range []struct {
		name, sha, want string
	}{
		{"empty", "", ""},
		{"exactly 12", "0123456789ab", "0123456789ab"},
		{"13 characters", "0123456789abc", "0123456789ab"},
		{"40 characters", parentSHA, "111111111111"},
	} {
		if got := shortSHA(tc.sha); got != tc.want {
			t.Errorf("%s: shortSHA(%q) = %q, want %q", tc.name, tc.sha, got, tc.want)
		}
	}
}
