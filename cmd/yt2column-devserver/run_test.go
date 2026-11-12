//go:build test

package main

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/serversock"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
	writertestutil "github.com/isseis/yt2column/internal/writer/testutil"
)

// errBuildRefused is what a constructor substituted by a test returns.
var errBuildRefused = errors.New("the test refuses to build this component")

// homeEntries returns the names under home, to check a run created nothing.
func homeEntries(t *testing.T, home string) []string {
	t.Helper()
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// mustScenario returns the checked scenario of a combination the test knows is
// valid.
func mustScenario(t *testing.T, a devfake.Article, f devfake.Failure, at devfake.FailAt, delay time.Duration) devfake.Scenario {
	t.Helper()
	s, err := devfake.NewScenario(a, f, at, delay)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestRunArgsAccepted checks the specifications parseArgs accepts: each yields
// the scenario devfake.NewScenario builds from the same selections.
func TestRunArgsAccepted(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want devfake.Scenario
	}{
		{"no arguments", nil, mustScenario(t, devfake.ArticleStandard, devfake.FailureNone, devfake.FailAtNone, 0)},
		{"each article", []string{"-article", "elements"}, mustScenario(t, devfake.ArticleElements, devfake.FailureNone, devfake.FailAtNone, 0)},
		{"two dashes and an equals sign", []string{"--article=long"}, mustScenario(t, devfake.ArticleLong, devfake.FailureNone, devfake.FailAtNone, 0)},
		{"a failure and its call", []string{"-fail", "api-error", "-fail-step", "review"}, mustScenario(t, devfake.ArticleStandard, devfake.FailureAPIError, devfake.FailAtReview, 0)},
		{"the shorten call of the long article", []string{"-article", "long", "-fail", "truncated", "-fail-step", "shorten"}, mustScenario(t, devfake.ArticleLong, devfake.FailureTruncated, devfake.FailAtShorten, 0)},
		{"delay 0", []string{"-delay", "0"}, mustScenario(t, devfake.ArticleStandard, devfake.FailureNone, devfake.FailAtNone, 0)},
		{"the longest delay", []string{"-delay", "600"}, mustScenario(t, devfake.ArticleStandard, devfake.FailureNone, devfake.FailAtNone, devfake.MaxDelay)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArgs(tc.args)
			if err != nil {
				t.Fatalf("parseArgs(%q) = %v, want no error", tc.args, err)
			}
			if got.LogValue().String() != tc.want.LogValue().String() {
				t.Errorf("parseArgs(%q) = %v, want %v", tc.args, got.LogValue(), tc.want.LogValue())
			}
		})
	}
}

// TestRunArgsRejected checks that a rejected specification makes run exit with
// usage, print the usage after the reason, and create nothing under HOME. The
// reason's wording is matched because the diagnostic is what the operator
// reads; run returns only an exit code otherwise.
func TestRunArgsRejected(t *testing.T) {
	const bigNumber = "99999999999999999999999"
	for _, tc := range []struct {
		name   string
		args   []string
		reason string
	}{
		{"unknown argument", []string{"-port", "8765"}, "flag provided but not defined"},
		{"unknown article", []string{"-article", "short"}, "not an accepted value"},
		{"unknown failure", []string{"-fail", "crash", "-fail-step", "draft"}, "not an accepted value"},
		{"unknown call", []string{"-fail", "empty", "-fail-step", "write"}, "not an accepted value"},
		{"none is not a failure", []string{"-fail", "none", "-fail-step", "draft"}, "not an accepted value"},
		{"none is not a call", []string{"-fail", "empty", "-fail-step", "none"}, "not an accepted value"},
		{"missing value", []string{"-article"}, "flag needs an argument"},
		{"extra word", []string{"extra"}, "unexpected extra argument"},
		{"extra word after an argument", []string{"-article", "long", "extra"}, "unexpected extra argument"},
		{"bare double dash", []string{"--"}, "unexpected extra argument"},
		{"repeated article", []string{"-article", "long", "-article", "long"}, "given more than once"},
		{"repeated delay", []string{"-delay", "1", "-delay", "1"}, "given more than once"},
		{"failure without a call", []string{"-fail", "empty"}, "given together"},
		{"call without a failure", []string{"-fail-step", "draft"}, "given together"},
		{"shorten call of the standard article", []string{"-fail", "empty", "-fail-step", "shorten"}, "only the long article"},
		{"shorten call of the elements article", []string{"-article", "elements", "-fail", "empty", "-fail-step", "shorten"}, "only the long article"},
		{"delay over the longest", []string{"-delay", "601"}, "over the longest delay"},
		{"delay that overflows a duration", []string{"-delay", "18446744074"}, "over the longest delay"},
		{"delay that does not fit an integer", []string{"-delay", bigNumber}, "over the longest delay"},
		{"delay with a leading zero", []string{"-delay", "01"}, "leading zero"},
		{"empty delay", []string{"-delay", ""}, "decimal digits"},
		{"negative delay", []string{"-delay", "-1"}, "decimal digits"},
		{"fractional delay", []string{"-delay", "1.5"}, "decimal digits"},
		{"delay with a unit", []string{"-delay", "1s"}, "decimal digits"},
		{"non-printable value", []string{"-article", "a\x1bb"}, "not an accepted value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			code, stderr := runToEnd(t, env, productionDeps(), tc.args...)
			if code != exitUsage {
				t.Errorf("exit code = %d, want %d\nstderr:\n%s", code, exitUsage, stderr)
			}
			if !strings.Contains(stderr, tc.reason) {
				t.Errorf("stderr lacks the reason %q:\n%s", tc.reason, stderr)
			}
			if !strings.Contains(stderr, "usage: "+programName) {
				t.Errorf("stderr lacks the usage:\n%s", stderr)
			}
			if strings.ContainsRune(stderr, '\x1b') {
				t.Errorf("stderr holds a raw escape character:\n%q", stderr)
			}
			if got := homeEntries(t, env.home); len(got) != 0 {
				t.Errorf("a rejected start created %v under HOME", got)
			}
			if got := env.lookedUp(); len(got) != 0 {
				t.Errorf("a rejected start read the environment: %v", got)
			}
		})
	}
}

// TestRunHelp checks that -h and -help print the usage, with every accepted
// value, create nothing, and exit 0.
func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "-help", "--help"} {
		t.Run(arg, func(t *testing.T) {
			env := newTestEnv(t)
			code, stderr := runToEnd(t, env, productionDeps(), arg)
			if code != exitOK {
				t.Errorf("exit code = %d, want %d", code, exitOK)
			}
			for _, want := range []string{"usage: " + programName, spellings(devfake.Articles()), spellings(devfake.Failures()), spellings(devfake.FailAts())} {
				if !strings.Contains(stderr, want) {
					t.Errorf("the usage lacks %q:\n%s", want, stderr)
				}
			}
			if got := homeEntries(t, env.home); len(got) != 0 {
				t.Errorf("the help request created %v under HOME", got)
			}
		})
	}
}

// TestStartupFailures checks each step that can stop the start before it
// serves, in the production server's way: the exit code, the cause on standard
// error, and that nothing a start leaves behind was created.
func TestStartupFailures(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, e *testEnv, d *deps)
		want  int
		// cause is a part of standard error that names the cause; run
		// returns only an exit code, so its standard error is the contract.
		cause string
		// hint, when set, is a second part standard error must hold.
		hint string
		// noSocketDir marks a failure before the socket directory exists.
		noSocketDir bool
	}{
		{
			name:        "HOME unset",
			setup:       func(_ *testing.T, e *testEnv, _ *deps) { delete(e.vars, "HOME") },
			want:        exitUsage,
			cause:       "configuration: HOME",
			noSocketDir: true,
		},
		{
			name:        "HOME relative",
			setup:       func(_ *testing.T, e *testEnv, _ *deps) { e.vars["HOME"] = "relative/home" },
			want:        exitUsage,
			cause:       "configuration: HOME",
			noSocketDir: true,
		},
		{
			name: "writer not built",
			setup: func(_ *testing.T, _ *testEnv, d *deps) {
				d.newWriter = func(llm.LLMClient, writer.Options) (writer.ProgressWriter, error) { return nil, errBuildRefused }
			},
			want:        exitUsage,
			cause:       "build the article writer",
			noSocketDir: true,
		},
		{
			name: "socket directory with a mode other than 0700",
			setup: func(t *testing.T, e *testEnv, _ *deps) {
				dir := filepath.Dir(e.socketPath())
				if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // the test makes the unsafe mode on purpose
					t.Fatal(err)
				}
				if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // MkdirAll's mode is masked by the umask
					t.Fatal(err)
				}
			},
			want:  exitFailure,
			cause: "does not have mode 0700",
			hint:  "nothing was repaired or removed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			d := productionDeps()
			tc.setup(t, env, &d)
			code, stderr := runToEnd(t, env, d)
			if code != tc.want {
				t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, tc.want, stderr)
			}
			for _, sub := range []string{tc.cause, tc.hint} {
				if !strings.Contains(stderr, sub) {
					t.Errorf("stderr lacks %q:\n%s", sub, stderr)
				}
			}
			if strings.Contains(stderr, "msg=listening") {
				t.Errorf("a failed start wrote the listening line:\n%s", stderr)
			}
			if tc.noSocketDir {
				if _, err := os.Stat(filepath.Dir(env.socketPath())); !os.IsNotExist(err) {
					t.Errorf("the socket directory exists after a failure before the socket: %v", err)
				}
			}
		})
	}
}

// TestLockHeld checks that a start stops with the production start lock's
// refusal while another process holds the lock, and never says it runs a fake.
func TestLockHeld(t *testing.T) {
	env := newTestEnv(t)
	lock, err := serversock.Acquire(env.socketPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Release() })

	code, stderr := runToEnd(t, env, productionDeps())
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitFailure, stderr)
	}
	if !strings.Contains(stderr, "another server is running") {
		t.Errorf("stderr does not say another server runs:\n%s", stderr)
	}
	if strings.Contains(stderr, "msg=listening") || strings.Contains(stderr, "fake") {
		t.Errorf("a start stopped at the lock said it runs on a fake:\n%s", stderr)
	}
}

// scenarioCombinations returns every article and failure selection devfake
// accepts: each article with no failure, and with each failure at each call.
func scenarioCombinations(t *testing.T) []devfake.Scenario {
	t.Helper()
	var out []devfake.Scenario
	for _, a := range devfake.Articles() {
		out = append(out, mustScenario(t, a, devfake.FailureNone, devfake.FailAtNone, 0))
		for _, f := range devfake.Failures() {
			for _, at := range devfake.FailAts() {
				if _, err := devfake.NewScenario(a, f, at, 0); err == nil {
					out = append(out, mustScenario(t, a, f, at, 0))
				}
			}
		}
	}
	return out
}

// TestScenarioCombinations checks, for every combination the arguments accept,
// that a generation start and a status read through the real server end in the
// article or the failure the combination selects, and that the calls waited
// the number of times the combination's call count says.
func TestScenarioCombinations(t *testing.T) {
	const count = 31 // 9 for the standard and the elements article each, 13 for the long one
	combos := scenarioCombinations(t)
	if len(combos) != count {
		t.Fatalf("the accepted combinations number %d, want %d", len(combos), count)
	}
	failureCodes := map[devfake.Failure]string{
		devfake.FailureEmpty:     "llm_empty",
		devfake.FailureTruncated: "llm_truncated",
		devfake.FailureAPIError:  "llm_failed",
		devfake.FailureTimeout:   "llm_timeout",
	}
	for _, a := range devfake.Articles() {
		for _, f := range append([]devfake.Failure{devfake.FailureNone}, devfake.Failures()...) {
			for _, at := range append([]devfake.FailAt{devfake.FailAtNone}, devfake.FailAts()...) {
				if _, err := devfake.NewScenario(a, f, at, 0); err != nil {
					continue
				}
				t.Run(describe(a, f, at), func(t *testing.T) {
					checkCombination(t, a, f, at, failureCodes)
				})
			}
		}
	}
}

// checkCombination runs one combination through a started server.
func checkCombination(t *testing.T, a devfake.Article, f devfake.Failure, at devfake.FailAt, failureCodes map[devfake.Failure]string) {
	t.Helper()
	env := newTestEnv(t)
	rec := newWaitRecorder()
	args := []string{"-article", a.String(), "-delay", "1"}
	if f != devfake.FailureNone {
		args = append(args, "-fail", f.String(), "-fail-step", at.String())
	}
	startServer(t, env, rec.deps(), args...)
	got := waitJobDone(t, env.socketPath(), startGeneration(t, env.socketPath(), "AAAAAAAAAAA"))

	// calls is how many fake calls the combination makes: the draft, the
	// shorten call of the long article, and the review.
	calls := 2
	if a == devfake.ArticleLong {
		calls = 3
	}
	switch at {
	case devfake.FailAtDraft:
		calls = 1
	case devfake.FailAtShorten:
		calls = 2
	default:
	}
	if got := rec.calls(); got != calls {
		t.Errorf("calls that waited = %d, want %d", got, calls)
	}
	for _, d := range rec.delays {
		if d != time.Second {
			t.Errorf("a call waited %v, want 1s", d)
		}
	}

	if f == devfake.FailureNone {
		if got.Job.State != "done" || got.Job.Article == nil {
			t.Fatalf("job = %+v, want a done article", got.Job)
		}
		if !strings.Contains(got.Job.Article.Markdown, devfake.TitleMarker) {
			t.Errorf("article lacks the title marker %q:\n%s", devfake.TitleMarker, got.Job.Article.Markdown)
		}
		if got.Job.Article.Cost.State != "unknown" {
			t.Errorf("cost state = %q, want unknown", got.Job.Article.Cost.State)
		}
		return
	}
	if got.Job.State != "failed" || got.Job.Article != nil {
		t.Fatalf("job = %+v, want a failed job with no article", got.Job)
	}
	if got.Job.Failure != failureCodes[f] {
		t.Errorf("failure = %q, want %q", got.Job.Failure, failureCodes[f])
	}
	steps := map[string]string{"draft": got.Job.Steps.Draft, "shorten": got.Job.Steps.Shorten, "review": got.Job.Steps.Review}
	for _, step := range []string{"draft", "shorten", "review"} {
		switch {
		case step == at.String() && steps[step] != "failed":
			t.Errorf("step %s = %q, want failed", step, steps[step])
		case step != at.String() && slices.Index([]string{"draft", "shorten", "review"}, step) < slices.Index([]string{"draft", "shorten", "review"}, at.String()):
			if steps[step] != "done" && steps[step] != "skipped" {
				t.Errorf("step %s before the failed %s = %q, want done or skipped", step, at, steps[step])
			}
		}
	}
}

// TestJobsIndependent checks that generations of different videos share no
// state: two overlap, held inside their first call so neither is ahead, and a
// third starts after both end; all three end the same way, for a success and
// for a failure.
func TestJobsIndependent(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"success", []string{"-article", "long"}},
		{"failure at the shorten call", []string{"-article", "long", "-fail", "empty", "-fail-step", "shorten"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			rec := newWaitRecorder()
			rec.release = make(chan struct{})
			startServer(t, env, rec.deps(), append(tc.args, "-delay", "1")...)
			path := env.socketPath()

			keyA := startGeneration(t, path, "AAAAAAAAAAA")
			keyB := startGeneration(t, path, "BBBBBBBBBBB")
			rec.waitCalls(t, 2) // both jobs are inside their first call
			close(rec.release)
			results := []jobResponse{waitJobDone(t, path, keyA), waitJobDone(t, path, keyB)}
			results = append(results, waitJobDone(t, path, startGeneration(t, path, "CCCCCCCCCCC")))

			// The article names its video, so the video IDs are masked before
			// the jobs are compared.
			masked := make([]jobResponse, len(results))
			for i, r := range results {
				masked[i] = r
				if r.Job.Article != nil {
					article := *r.Job.Article
					for _, id := range []string{"AAAAAAAAAAA", "BBBBBBBBBBB", "CCCCCCCCCCC"} {
						article.Markdown = strings.ReplaceAll(article.Markdown, id, "VIDEO")
					}
					masked[i].Job.Article = &article
				}
			}
			if masked[0].Job.State == "running" || masked[0].Job.State == "" {
				t.Fatalf("first job did not end: %+v", masked[0].Job)
			}
			for i, r := range masked[1:] {
				if !reflect.DeepEqual(r.Job, masked[0].Job) {
					t.Errorf("job %d = %+v (%+v), want the same as job 0 = %+v (%+v)", i+1, r.Job, r.Job.Article, masked[0].Job, masked[0].Job.Article)
				}
			}
		})
	}
}

// TestJobWriterFailure checks that a failure to build the writer inside a job
// is reported as the writer contract requires: a *writer.TokenUsageError with
// an empty usage, and a job that ends failed with no article.
func TestJobWriterFailure(t *testing.T) {
	t.Run("perJobWriter", func(t *testing.T) {
		jw := perJobWriter{
			scenario:  mustScenario(t, devfake.ArticleStandard, devfake.FailureNone, devfake.FailAtNone, 0),
			newWriter: func(llm.LLMClient, writer.Options) (writer.ProgressWriter, error) { return nil, errBuildRefused },
		}
		for name, call := range map[string]func() error{
			"Write": func() error { _, err := jw.Write(t.Context(), transcript.Transcript{}); return err },
			"WriteWithProgress": func() error {
				_, err := jw.WriteWithProgress(t.Context(), transcript.Transcript{}, nil)
				return err
			},
		} {
			err := call()
			usageErr, ok := errors.AsType[*writer.TokenUsageError](err)
			if !ok || !errors.Is(err, errBuildRefused) {
				t.Fatalf("%s error = %v, want a *writer.TokenUsageError wrapping the build failure", name, err)
			}
			if usageErr.RunUsage != (writer.RunUsage{}) {
				t.Errorf("%s usage = %+v, want empty", name, usageErr.RunUsage)
			}
		}
	})
	t.Run("through the server", func(t *testing.T) {
		env := newTestEnv(t)
		d := productionDeps()
		calls := 0
		d.newWriter = func(c llm.LLMClient, o writer.Options) (writer.ProgressWriter, error) {
			calls++
			if calls == 1 { // the check at start
				return writer.NewProgressWriter(c, o)
			}
			return nil, errBuildRefused
		}
		startServer(t, env, d)
		got := waitJobDone(t, env.socketPath(), startGeneration(t, env.socketPath(), "AAAAAAAAAAA"))
		if got.Job.State != "failed" || got.Job.Article != nil {
			t.Errorf("job = %+v, want a failed job with no article", got.Job)
		}
	})
}

// TestIgnoresConfigAndEnv checks that the server's configuration file and the
// environment do not change a start: whatever the file is (missing, with the
// wrong mode, malformed), and whatever other variables are set, the listening
// line (but for the socket) and the article are the same, and only HOME is
// read.
func TestIgnoresConfigAndEnv(t *testing.T) {
	writeFile := func(mode os.FileMode, content string) func(*testing.T, *testEnv) {
		return func(t *testing.T, e *testEnv) {
			t.Helper()
			path := config.ServerFilePathForTest(e.home)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), mode); err != nil { //nolint:gosec // the test makes the unsafe mode on purpose
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil { //nolint:gosec // WriteFile's mode is masked by the umask
				t.Fatal(err)
			}
		}
	}
	var lines []map[string]string
	var markdowns []string
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, *testEnv)
	}{
		{"no file", func(*testing.T, *testEnv) {}},
		{"mode 0644", writeFile(0o644, "YT2COLUMN_LLM=deepseek/some-model\nDEEPSEEK_API_KEY=sk-not-used\n")},
		{"malformed", writeFile(0o600, "this is not a configuration file\n\x00\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			tc.setup(t, env)
			maps.Copy(env.vars, map[string]string{"DEEPSEEK_API_KEY": "sk-decoy", "YT2COLUMN_LLM": "nonsense/", "GODEBUG": "http2debug=1", "XDG_CONFIG_HOME": "/nonexistent"})
			srv := startServer(t, env, productionDeps(), "-article", "elements")
			line := srv.waitStderr(t, "msg=listening")
			attrs := logAttrs(t, line)
			delete(attrs, "socket") // differs by HOME
			lines = append(lines, attrs)
			got := waitJobDone(t, env.socketPath(), startGeneration(t, env.socketPath(), "AAAAAAAAAAA"))
			if got.Job.Article == nil {
				t.Fatalf("job = %+v, want a done article", got.Job)
			}
			markdowns = append(markdowns, got.Job.Article.Markdown)
			if got, want := env.lookedUp(), []string{"HOME"}; !slices.Equal(got, want) {
				t.Errorf("the run looked up %v, want only %v", got, want)
			}
			if strings.Contains(srv.stderr.String(), "http2debug") {
				t.Errorf("GODEBUG changed the output:\n%s", srv.stderr.String())
			}
		})
	}
	for i := 1; i < len(lines); i++ {
		if !reflect.DeepEqual(lines[i], lines[0]) {
			t.Errorf("listening attributes of case %d = %v, want %v", i, lines[i], lines[0])
		}
		// The article names its video, not its HOME, so it is equal as is.
		if markdowns[i] != markdowns[0] {
			t.Errorf("the article of case %d differs from case 0", i)
		}
	}
}

// logAttrPattern matches one key=value attribute of a slog text line; a value
// with a space is quoted.
var logAttrPattern = regexp.MustCompile(`(\S+?)=("(?:[^"\\]|\\.)*"|\S*)`)

// logAttrs reads the attributes of a slog text line, without the time, level,
// and msg.
func logAttrs(t *testing.T, line string) map[string]string {
	t.Helper()
	attrs := map[string]string{}
	for _, m := range logAttrPattern.FindAllStringSubmatch(line, -1) {
		value := m[2]
		if strings.HasPrefix(value, `"`) {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				t.Fatalf("attribute %s: %v", m[1], err)
			}
			value = unquoted
		}
		switch m[1] {
		case "time", "level", "msg":
		default:
			attrs[m[1]] = value
		}
	}
	return attrs
}

// TestListeningLine checks the one line that says the server runs on a fake:
// the socket path derived from HOME, the fake marker, the model, and the
// scenario in the arguments' spellings.
func TestListeningLine(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want map[string]string
	}{
		{
			name: "no failure",
			args: []string{"-article", "elements", "-delay", "3"},
			want: map[string]string{"llm": "fake", "model": devfake.ModelName, "scenario.article": "elements", "scenario.fail": "none", "scenario.delay": "3"},
		},
		{
			name: "a failure",
			args: []string{"-article", "long", "-fail", "api-error", "-fail-step", "shorten"},
			want: map[string]string{"llm": "fake", "model": devfake.ModelName, "scenario.article": "long", "scenario.fail": "api-error", "scenario.failStep": "shorten", "scenario.delay": "0"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			srv := startServer(t, env, productionDeps(), tc.args...)
			got := logAttrs(t, srv.waitStderr(t, "msg=listening"))
			tc.want["socket"] = filepath.Join(env.home, config.SocketPathTailForTest(runtime.GOOS))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("listening attributes = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestStopNotice checks the stop: a generation held in a call is canceled and
// logged, the notice is written, the exit code is 0, and the start lock is
// released. The notice may come after Serve returns, so it is read with a
// bound.
func TestStopNotice(t *testing.T) {
	env := newTestEnv(t)
	rec := newWaitRecorder()
	rec.release = make(chan struct{}) // never closed: only the context ends the wait
	srv := startServer(t, env, rec.deps(), "-delay", "1")
	startGeneration(t, env.socketPath(), "AAAAAAAAAAA")
	rec.waitCalls(t, 1)

	if code := srv.stop(t); code != exitOK {
		t.Fatalf("exit code = %d, want %d\nstderr:\n%s", code, exitOK, srv.stderr.String())
	}
	srv.waitStderr(t, "stopping:")
	srv.waitStderr(t, "generation job canceled by the shutdown")
	srv.waitStderr(t, "msg=stopped")

	lock, err := serversock.Acquire(env.socketPath())
	if err != nil {
		t.Fatalf("the start lock was not released: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

// TestProductionDeps checks the values main passes: the scenario function
// returns the scenario it is given, and a start with it, whose delay is 0,
// completes an article without any substituted wait.
func TestProductionDeps(t *testing.T) {
	s := mustScenario(t, devfake.ArticleLong, devfake.FailureEmpty, devfake.FailAtReview, 2*time.Second)
	if got := productionDeps().scenario(s); got.LogValue().String() != s.LogValue().String() {
		t.Errorf("scenario(s) = %v, want %v", got.LogValue(), s.LogValue())
	}

	env := newTestEnv(t)
	startServer(t, env, productionDeps(), "-article", "long")
	got := waitJobDone(t, env.socketPath(), startGeneration(t, env.socketPath(), "AAAAAAAAAAA"))
	if got.Job.State != "done" {
		t.Errorf("job = %+v, want done", got.Job)
	}
}

// TestWriterOptionsMatchCLIDefaults checks that the writer of every job is
// built with the options of a CLI run with every post-generation step at its
// default, which writertestutil.DefaultOptions defines, with the job's own
// review client.
func TestWriterOptionsMatchCLIDefaults(t *testing.T) {
	var got []writer.Options
	jw := perJobWriter{
		scenario: mustScenario(t, devfake.ArticleStandard, devfake.FailureNone, devfake.FailAtNone, 0),
		newWriter: func(_ llm.LLMClient, o writer.Options) (writer.ProgressWriter, error) {
			got = append(got, o)
			return nil, errBuildRefused
		},
	}
	if _, err := jw.jobWriter(); !errors.Is(err, errBuildRefused) {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Review.Client == nil {
		t.Fatalf("options = %+v, want one set with a review client", got)
	}
	if want := writertestutil.DefaultOptions(got[0].Review.Client); !reflect.DeepEqual(got[0], want) {
		t.Errorf("writer options = %+v, want %+v", got[0], want)
	}
}
