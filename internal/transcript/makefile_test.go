//go:build test

package transcript

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/maketestutil"
)

// The parse deadline is checked by one go test run that -run narrows to
// TestParseCopiedLargeInputs. A renamed test, an added -race, a dropped
// -count=1, or a removed call or CI step would leave that run passing without
// the check, so these tests pin the recipe and the places that run it.

// parseNoraceTarget is the make target that runs the parse deadline check.
const parseNoraceTarget = "test-parse-norace"

// repositoryRoot is where the Makefile lives, relative to this package.
var repositoryRoot = filepath.Join("..", "..")

// largeInputTestName returns TestParseCopiedLargeInputs's name from the
// function value, so renaming the test changes what the guard expects.
func largeInputTestName() string {
	name := runtime.FuncForPC(reflect.ValueOf(TestParseCopiedLargeInputs).Pointer()).Name()
	return name[strings.LastIndex(name, ".")+1:]
}

// TestMakeTestParseNorace checks the go test arguments of test-parse-norace:
// the test build tag without -race or coverage, no cached result, only
// TestParseCopiedLargeInputs in this package, and a -timeout above the seven
// inputs' deadlines together but below go test's default of ten minutes. The
// deadline itself must stay under the server's 30s parse budget.
func TestMakeTestParseNorace(t *testing.T) {
	const (
		timeoutPlaceholder = "<timeout>"
		runPlaceholder     = "<run>"
		// largeInputs is how many inputs TestParseCopiedLargeInputs parses;
		// keep it in step with that test's table.
		largeInputs    = 7
		defaultTimeout = 10 * time.Minute
		parseBudget    = 30 * time.Second
	)
	wantArgs := []string{
		"-tags", "test", "-count=1", "-v",
		"-timeout", timeoutPlaceholder, "-run", runPlaceholder, "./internal/transcript",
	}

	_, invocation := maketestutil.RunTarget(t, repositoryRoot, parseNoraceTarget, nil, nil, nil)
	args := slices.Clone(invocation.Args)
	if i := slices.Index(args, "-timeout"); i >= 0 && i+1 < len(args) {
		timeout, err := time.ParseDuration(args[i+1])
		if err != nil || timeout <= largeInputs*copiedParseDeadline || timeout >= defaultTimeout {
			t.Errorf("-timeout %q (parse error %v), want a duration above %s and below %s",
				args[i+1], err, largeInputs*copiedParseDeadline, defaultTimeout)
		}
		args[i+1] = timeoutPlaceholder
	}
	if i := slices.Index(args, "-run"); i >= 0 && i+1 < len(args) {
		if want := "^" + largeInputTestName() + "$"; args[i+1] != want {
			t.Errorf("-run %q, want %q", args[i+1], want)
		}
		args[i+1] = runPlaceholder
	}
	if !slices.Equal(args, wantArgs) {
		t.Errorf("GOTEST arguments = %q, want %q", invocation.Args, wantArgs)
	}
	if copiedParseDeadline >= parseBudget {
		t.Errorf("copiedParseDeadline = %s, want under the %s parse budget", copiedParseDeadline, parseBudget)
	}
}

// TestMakeTestRunsParseNorace checks that `make test` ends by running
// test-parse-norace. The stub GOTEST keeps only its last call, and the
// command-line GOTEST reaches the sub-make, so that last call's arguments are
// those of test-parse-norace exactly when `make test` runs it last.
func TestMakeTestRunsParseNorace(t *testing.T) {
	_, want := maketestutil.RunTarget(t, repositoryRoot, parseNoraceTarget, nil, nil, nil)
	_, got := maketestutil.RunTarget(t, repositoryRoot, "test", nil, nil, nil)
	if !slices.Equal(got.Args, want.Args) {
		t.Errorf("make test last ran GOTEST with %q, want the %s arguments %q", got.Args, parseNoraceTarget, want.Args)
	}
}

// ciStep is one step of a workflow job: its keys and its run value.
type ciStep struct {
	keys []string
	run  string
}

// TestCIRunsParseNorace checks that the Test job of ci.yml runs
// test-parse-norace as its own step after `make test-ci`, and that nothing
// lets that step fail without failing the job: no step-level if or
// continue-on-error on the step, and no job-level continue-on-error. ci.yml is
// read by line, at the indentation it uses: jobs at two spaces, job keys at
// four, steps at six, and step keys at eight.
func TestCIRunsParseNorace(t *testing.T) {
	const (
		jobIndent     = "  "
		jobKeyIndent  = "    "
		stepIndent    = "      - "
		stepKeyIndent = "        "
	)
	path := filepath.Join(repositoryRoot, ".github", "workflows", "ci.yml")
	content, err := os.ReadFile(path) //nolint:gosec // a repository file the test names
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	inJob, foundJob := false, false
	var jobKeys []string
	var steps []ciStep
	// key returns the key of a "key: value" line and its trimmed value.
	key := func(line string) (string, string) {
		k, v, _ := strings.Cut(strings.TrimSpace(line), ":")
		return k, strings.TrimSpace(v)
	}
	addStepKey := func(line string) {
		k, v := key(line)
		step := &steps[len(steps)-1]
		step.keys = append(step.keys, k)
		if k == "run" {
			step.run = v
		}
	}
	for line := range strings.Lines(string(content)) {
		line = strings.TrimRight(line, "\n")
		if line == jobIndent+"test:" {
			inJob, foundJob = true, true
			continue
		}
		if !inJob {
			continue
		}
		rest, ok := strings.CutPrefix(line, jobIndent)
		if ok && rest != "" && rest[0] != ' ' && rest[0] != '#' {
			break // the next job
		}
		switch {
		case strings.HasPrefix(line, stepIndent):
			steps = append(steps, ciStep{})
			addStepKey(strings.TrimPrefix(line, stepIndent))
		case strings.HasPrefix(line, stepKeyIndent) && len(steps) > 0:
			if body := strings.TrimPrefix(line, stepKeyIndent); body != "" && body[0] != ' ' && body[0] != '#' {
				addStepKey(body)
			}
		case strings.HasPrefix(line, jobKeyIndent):
			if body := strings.TrimPrefix(line, jobKeyIndent); body != "" && body[0] != ' ' && body[0] != '#' {
				k, _ := key(body)
				jobKeys = append(jobKeys, k)
			}
		}
	}
	if !foundJob {
		t.Fatalf("%s: no test job", path)
	}
	if slices.Contains(jobKeys, "continue-on-error") {
		t.Errorf("%s: the test job has continue-on-error", path)
	}

	runIndex := func(run string) int {
		return slices.IndexFunc(steps, func(s ciStep) bool { return s.run == run })
	}
	parse, testCI := runIndex("make "+parseNoraceTarget), runIndex("make test-ci")
	if parse < 0 {
		t.Fatalf("%s: the test job has no step that runs %q", path, "make "+parseNoraceTarget)
	}
	if testCI < 0 || parse < testCI {
		t.Errorf("%s: the %s step is at %d, want it after the make test-ci step (at %d)", path, parseNoraceTarget, parse, testCI)
	}
	for _, k := range []string{"if", "continue-on-error"} {
		if slices.Contains(steps[parse].keys, k) {
			t.Errorf("%s: the %s step has %s", path, parseNoraceTarget, k)
		}
	}
}
