//go:build test

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/doctestutil"
	claudetestutil "github.com/isseis/yt2column/internal/llm/claude/testutil"
	deepseektestutil "github.com/isseis/yt2column/internal/llm/deepseek/testutil"
	"github.com/isseis/yt2column/internal/llm/provider"
	publishertestutil "github.com/isseis/yt2column/internal/publisher/testutil"
)

const (
	// repositoryRoot is where the Makefile lives, relative to this package.
	repositoryRoot = "../.."

	// deepSeekIntegrationTarget is the make target that runs the DeepSeek
	// adapter's charged integration test.
	deepSeekIntegrationTarget = "test-integration-deepseek"

	// claudeIntegrationTarget is the make target that runs the Claude
	// adapter's charged integration test.
	claudeIntegrationTarget = "test-integration-claude"

	// transcriptIntegrationTarget is the make target that runs the
	// transcript integration test, which has no opt-in.
	transcriptIntegrationTarget = "test-integration"
)

// retiredLLMEnvs are the retired production LLM variables, among them the
// model variable the DeepSeek integration targets used before the rename. A
// target, or the code it builds, that still names one would take its model
// from a value a user set for the production configuration instead of the
// test's own variable. The names come from the config package, not from a
// copy here.
var retiredLLMEnvs = config.RetiredLLMVarsForTest()

// TestMakeTestIntegrationCLI checks the target that runs the CLI integration
// test. It makes one Generate call, so the -timeout value must exceed
// provider.LLMTimeout.
func TestMakeTestIntegrationCLI(t *testing.T) {
	deepseektestutil.CheckChargedTarget(t, deepseektestutil.ChargedTarget{
		Root:       repositoryRoot,
		Target:     cliIntegrationOptions.MakeTarget,
		OptInEnv:   cliIntegrationOptions.OptInEnv,
		Package:    "./cmd/yt2column",
		MinTimeout: provider.LLMTimeout,
	})
}

// TestMakeOptInsAreTargetSpecific pins that each integration target exports
// no opt-in but its own, so running one never arms another's test. That the
// own opt-in is exported is checked by each target's own test.
func TestMakeOptInsAreTargetSpecific(t *testing.T) {
	optIns := []string{
		deepseektestutil.DeepSeekOptInEnv,
		deepseektestutil.CLIOptInEnv,
		claudetestutil.OptInEnv,
		publishertestutil.SlackOptInEnv,
	}
	for _, tc := range []struct {
		target string
		own    string // empty for a target with no opt-in
	}{
		{target: transcriptIntegrationTarget},
		{target: deepSeekIntegrationTarget, own: deepseektestutil.DeepSeekOptInEnv},
		{target: claudeIntegrationTarget, own: claudetestutil.OptInEnv},
		{target: cliIntegrationOptions.MakeTarget, own: deepseektestutil.CLIOptInEnv},
		{target: publishertestutil.SlackIntegrationOptions.MakeTarget, own: publishertestutil.SlackOptInEnv},
	} {
		t.Run(tc.target, func(t *testing.T) {
			_, invocation := deepseektestutil.RunMakeTarget(t, repositoryRoot, tc.target, nil,
				claudetestutil.OptInEnv, publishertestutil.SlackOptInEnv)
			for _, optIn := range optIns {
				if value, ok := invocation.Env[optIn]; ok && optIn != tc.own {
					t.Errorf("make %s exported %s=%q", tc.target, optIn, value)
				}
			}
		})
	}
}

// TestMakeTargetsDoNotReadRetiredModelEnv checks that the two targets that
// run the DeepSeek integration tests take the model from the test's own
// variable: with only a retired variable set and the test variable undefined,
// each target passes the default to go test.
func TestMakeTargetsDoNotReadRetiredModelEnv(t *testing.T) {
	for _, target := range []string{
		deepSeekIntegrationTarget,
		cliIntegrationOptions.MakeTarget,
	} {
		for _, retired := range retiredLLMEnvs {
			t.Run(target+"/"+retired, func(t *testing.T) {
				deepseektestutil.CheckRetiredModelEnvNotRead(t, repositoryRoot, target, retired)
			})
		}
	}
}

// retiredLLMEnvMentioned reports whether line names one of retiredLLMEnvs as
// a whole variable name. A longer name that only starts or ends with one, such
// as YT2COLUMN_MODEL_OLD, is another variable and allowed.
func retiredLLMEnvMentioned(line string) bool {
	return retiredLLMEnvRE.MatchString(line)
}

// retiredLLMEnvRE matches any of retiredLLMEnvs as a whole variable name.
var retiredLLMEnvRE = func() *regexp.Regexp {
	quoted := make([]string, 0, len(retiredLLMEnvs))
	for _, name := range retiredLLMEnvs {
		quoted = append(quoted, regexp.QuoteMeta(name))
	}
	return regexp.MustCompile(`(^|[^A-Za-z0-9_])(` + strings.Join(quoted, "|") + `)([^A-Za-z0-9_]|$)`)
}()

// TestRetiredLLMEnvMentioned pins the whole-name boundary of the guard's
// matcher: a retired name matches in the forms a Makefile line or a Go source
// line holds it, and a longer name that contains it does not. The boundary is
// shared by every alternative, so it is checked on one name, and each name is
// checked to match.
func TestRetiredLLMEnvMentioned(t *testing.T) {
	retired := retiredLLMEnvs[0]
	for _, tc := range []struct {
		name string
		line string
		want bool
	}{
		{name: "assignment", line: retired + " ?= deepseek-flash", want: true},
		{name: "export", line: "export " + retired + " := $(" + retired + ")", want: true},
		{name: "make_reference", line: "$(" + retired + ")", want: true},
		{name: "shell_reference", line: `"${` + retired + `}"`, want: true},
		{name: "name_starts_with_identifier", line: "DEFAULT_" + retired, want: false},
		{name: "name_ends_with_identifier", line: retired + "_OLD", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := retiredLLMEnvMentioned(tc.line); got != tc.want {
				t.Errorf("retiredLLMEnvMentioned(%q) = %t, want %t", tc.line, got, tc.want)
			}
		})
	}
	for _, name := range retiredLLMEnvs {
		if !retiredLLMEnvMentioned("$(" + name + ")") {
			t.Errorf("retiredLLMEnvMentioned does not match %s", name)
		}
	}
}

// TestRetiredModelEnvIsGone checks that the Makefile, where the DeepSeek
// integration targets are defined, the Go code under internal/llm/deepseek
// that they build, and the CLI integration test files of this package do not
// name a retired variable. The whole Makefile is scanned rather than the
// targets' lines, so a mention anywhere in it fails. Every scope must hold at
// least one file to scan, so a scope that finds none is a failure rather than
// a scan of nothing that passes.
func TestRetiredModelEnvIsGone(t *testing.T) {
	for _, scope := range []struct {
		name  string
		files func(t *testing.T) []string
	}{
		{name: "Makefile", files: func(*testing.T) []string { return []string{"Makefile"} }},
		{name: "internal/llm/deepseek", files: func(t *testing.T) []string { return goFilesUnder(t, "internal/llm/deepseek") }},
		{name: "cmd/yt2column integration", files: func(t *testing.T) []string {
			var files []string
			for _, file := range goFilesUnder(t, "cmd/yt2column") {
				if strings.Contains(filepath.Base(file), "integration") {
					files = append(files, file)
				}
			}
			return files
		}},
	} {
		t.Run(scope.name, func(t *testing.T) {
			files := scope.files(t)
			if len(files) == 0 {
				t.Fatalf("found no files in %s to scan", scope.name)
			}
			for _, file := range files {
				content, err := os.ReadFile(filepath.Join(repositoryRoot, file))
				if err != nil {
					t.Fatalf("read %s: %v", file, err)
				}
				for i, line := range slices.Collect(strings.Lines(string(content))) {
					if retiredLLMEnvMentioned(line) {
						t.Errorf("%s:%d names a retired variable: %s", file, i+1, strings.TrimSpace(line))
					}
				}
			}
		})
	}
}

// goFilesUnder returns the paths of the Go files under dir, relative to the
// repository root, in order.
func goFilesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join(repositoryRoot, dir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	slices.Sort(files)
	return files
}

// gateRecorder is a testing.TB that records Skip and Fatal instead of ending
// the test, so a test can observe what gateCLIIntegration decided and that it
// did not go on to call the body. It embeds a nil testing.TB, so any other
// method the gate might call (Skipf, Fatalf, SkipNow, ...) panics instead of
// reaching the real test and letting a case pass unchecked.
type gateRecorder struct {
	testing.TB
	skipped []string
	failed  []string
}

// Helper is a no-op; the gate calls it first.
func (r *gateRecorder) Helper() {}

// Skip records the reason instead of skipping.
func (r *gateRecorder) Skip(args ...any) {
	r.skipped = append(r.skipped, fmt.Sprint(args...))
}

// Fatal records the reason instead of failing.
func (r *gateRecorder) Fatal(args ...any) {
	r.failed = append(r.failed, fmt.Sprint(args...))
}

// TestCLIIntegrationSettings checks the decision the CLI integration test
// makes in each environment: it runs only with its own opt-in set to exactly
// 1, fails rather than skips without the test API key even when the
// production key is set, and fails on an HTTP/2 debug setting. When it does
// not run, the body that calls run and the LLM client is never reached, and
// the reason holds neither key nor the last eight characters of either.
func TestCLIIntegrationSettings(t *testing.T) {
	const (
		testKey       = "sk-integration-TESTKEYVALUE-0123456789-TAIL8TST"
		productionKey = testAPIKey
		model         = "deepseek-custom"
	)
	// The environment names the CLI opt-in directly, not through
	// cliIntegrationOptions, so a wrong opt-in in the options is caught.
	complete := map[string]string{
		deepseektestutil.CLIOptInEnv: deepseektestutil.OptInValue,
		deepseektestutil.APIKeyEnv:   testKey,
		deepseektestutil.ModelEnv:    model,
		"DEEPSEEK_API_KEY":           productionKey,
	}
	keys := []string{testKey, testKey[len(testKey)-8:], productionKey, productionKey[len(productionKey)-8:]}

	// outcomes counts what the gate did: called the body, skipped, failed.
	type outcomes struct{ ran, skipped, failed int }
	var (
		ran     = outcomes{ran: 1}
		skipped = outcomes{skipped: 1}
		failed  = outcomes{failed: 1}
	)
	for _, tc := range []struct {
		name   string
		change map[string]string
		want   outcomes
	}{
		{name: "opt_in_unset", change: map[string]string{deepseektestutil.CLIOptInEnv: ""}, want: skipped},
		{name: "opt_in_zero", change: map[string]string{deepseektestutil.CLIOptInEnv: "0"}, want: skipped},
		{name: "opt_in_true", change: map[string]string{deepseektestutil.CLIOptInEnv: "true"}, want: skipped},
		{name: "opt_in_padded", change: map[string]string{deepseektestutil.CLIOptInEnv: " 1"}, want: skipped},
		{name: "deepseek_opt_in_only", change: map[string]string{deepseektestutil.CLIOptInEnv: "", deepseektestutil.DeepSeekOptInEnv: deepseektestutil.OptInValue}, want: skipped},
		{name: "test_key_missing_with_production_key_set", change: map[string]string{deepseektestutil.APIKeyEnv: ""}, want: failed},
		{name: "godebug_http2debug_1", change: map[string]string{deepseektestutil.GODEBUGEnv: "http2debug=1"}, want: failed},
		{name: "godebug_http2debug_2_among_others", change: map[string]string{deepseektestutil.GODEBUGEnv: "gctrace=1,http2debug=2"}, want: failed},
		{name: "complete", want: ran},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := maps.Clone(complete)
			maps.Copy(env, tc.change)
			recorder := &gateRecorder{}
			var bodySettings []deepseektestutil.IntegrationSettings
			gateCLIIntegration(recorder, func(name string) string { return env[name] }, func(s deepseektestutil.IntegrationSettings) {
				bodySettings = append(bodySettings, s)
			})

			got := outcomes{ran: len(bodySettings), skipped: len(recorder.skipped), failed: len(recorder.failed)}
			if got != tc.want {
				t.Fatalf("gate outcomes = %+v, want %+v", got, tc.want)
			}
			for _, reason := range recorder.skipped {
				// A skip must point at the CLI's own opt-in and target, never
				// at the other charged target.
				for _, want := range []string{deepseektestutil.CLIOptInEnv, "make test-integration-cli`"} {
					if !strings.Contains(reason, want) {
						t.Errorf("skip reason %q does not mention %q", reason, want)
					}
				}
			}
			for _, reason := range slices.Concat(recorder.skipped, recorder.failed) {
				for _, key := range keys {
					if strings.Contains(reason, key) {
						t.Error("the reason holds an API key or its last eight characters")
					}
				}
			}
			if tc.want != ran {
				return
			}
			if bodySettings[0].Model != model {
				t.Errorf("Model = %q, want %q", bodySettings[0].Model, model)
			}
			if key, err := bodySettings[0].APIKey.Reveal(); err != nil || key != testKey {
				t.Errorf("APIKey is not the value of %s (Reveal error = %v)", deepseektestutil.APIKeyEnv, err)
			}
		})
	}
}

// TestCLIIntegrationTestBuildTag pins the build tag that keeps the CLI
// integration test, which calls the real DeepSeek API, out of `make test`
// while `make lint` still compiles it through `go vet -tags integration`.
func TestCLIIntegrationTestBuildTag(t *testing.T) {
	if err := deepseektestutil.FirstLineIs("integration_test.go", "//go:build integration"); err != nil {
		t.Errorf("integration_test.go must start with %q: %v", "//go:build integration", err)
	}
}

// TestMakeBuildBuildsEveryBinary checks the recipes `make -n` expands for the
// build and all targets: each builds the CLI, the server, and the Native
// Messaging host, and no line runs the host binary with install or uninstall,
// because registering the host is the user's explicit step. The recipes are
// read after expansion, since the Makefile names the packages through
// variables.
func TestMakeBuildBuildsEveryBinary(t *testing.T) {
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	packages := []string{"./cmd/yt2column", "./cmd/yt2column-server", "./cmd/yt2column-nmhost"}
	for _, target := range []string{"build", "all"} {
		t.Run(target, func(t *testing.T) {
			cmd := exec.Command(makePath, "-n", target) //nolint:gosec // make from PATH with a fixed target
			cmd.Dir = repositoryRoot
			// Only PATH and HOME, so the outer make's MAKEFLAGS and variables
			// do not change the expansion.
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("make -n %s error = %v, output:\n%s", target, err, output)
			}
			built := map[string]struct{}{}
			for line := range strings.Lines(string(output)) {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[1] == "build" {
					built[fields[len(fields)-1]] = struct{}{}
				}
				for i, field := range fields[:max(len(fields)-1, 0)] {
					if strings.HasSuffix(field, "yt2column-nmhost") && (fields[i+1] == "install" || fields[i+1] == "uninstall") {
						t.Errorf("make -n %s runs the host with %s: %q", target, fields[i+1], strings.TrimSpace(line))
					}
				}
			}
			for _, pkg := range packages {
				if _, ok := built[pkg]; !ok {
					t.Errorf("make -n %s does not build %s; output:\n%s", target, pkg, output)
				}
			}
		})
	}
}

// coverageCompareGoEnv is the go environment the coverage-compare recipes
// must give go list and go build, so the user's go configuration cannot
// change the build and nothing is fetched.
var coverageCompareGoEnv = map[string]string{
	"GOTOOLCHAIN": "local",
	"GOENV":       "off",
	"GOFLAGS":     "",
	"GOPROXY":     "off",
	"GOWORK":      "off",
}

// The coverage-compare targets share one recipe, the coverage_compare define,
// and differ only in the word they pass to the tool. The recipe is checked
// through recipeTarget; the other target is checked for its word alone.
const (
	recipeTarget = "coverage-compare"
	recipeWord   = "go"
)

// recipeStub is a fake go for the coverage-compare recipes, first on make's
// PATH. Its shell scripts record into record, a directory outside make's
// TMPDIR, so the test can check that TMPDIR is left empty.
type recipeStub struct {
	bin    string // holds the fake go, first on PATH
	config string // the switches the fake go and the substitute tool read
	record string // what they record
	tmp    string // make's TMPDIR
}

// The switch files a test creates in recipeStub.config.
const (
	stubListFails  = "list-fails"  // go list -m exits 1
	stubBuildFails = "build-fails" // go build exits 1
	stubToolExit   = "tool-exit"   // the substitute tool's exit code
	stubToolWaits  = "tool-waits"  // the substitute tool waits for SIGTERM
)

// substituteFinalLine is what the substitute tool writes before it exits, in
// place of the real tool's final line.
const substituteFinalLine = "substitute tool final line"

// newRecipeStub writes the fake go and the substitute tool that its go build
// copies to the -o path. The scripts embed their directories in single
// quotes, so a directory holding one is refused.
func newRecipeStub(t *testing.T) recipeStub {
	t.Helper()
	s := recipeStub{bin: t.TempDir(), config: t.TempDir(), record: t.TempDir(), tmp: t.TempDir()}
	for _, dir := range []string{s.bin, s.config, s.record} {
		if strings.Contains(dir, "'") {
			t.Fatalf("temporary directory %q holds a single quote", dir)
		}
	}
	var goScript strings.Builder
	fmt.Fprintf(&goScript, "#!/bin/sh\nconfig='%s'\nrecord='%s'\n", s.config, s.record)
	goScript.WriteString("{\n\tprintf 'call\\n'\n\tfor arg in \"$@\"; do printf 'arg %s\\n' \"$arg\"; done\n")
	for _, name := range slices.Sorted(maps.Keys(coverageCompareGoEnv)) {
		fmt.Fprintf(&goScript, "\tif [ \"${%[1]s+set}\" = set ]; then printf 'env %[1]s=%%s\\n' \"$%[1]s\"; fi\n", name)
	}
	goScript.WriteString("} >> \"$record/go\"\n")
	goScript.WriteString(`case "$1" in
list)
	[ -e "$config/` + stubListFails + `" ] && exit 1
	echo example.com/stub
	exit 0 ;;
build)
	[ -e "$config/` + stubBuildFails + `" ] && exit 1
	out=
	while [ $# -gt 0 ]; do
		[ "$1" = -o ] && out=$2
		shift
	done
	cp "$config/tool" "$out" && chmod +x "$out"
	exit ;;
esac
exit 1
`)
	tool := fmt.Sprintf(`#!/bin/sh
config='%s'
record='%s'
for arg in "$@"; do printf 'arg %%s\n' "$arg"; done > "$record/tool"
if [ -e "$config/%s" ]; then
	trap 'echo term > "$record/tool-term"; sleep 0.3; echo %q; echo exited > "$record/tool-exited"; exit 3' TERM
	echo $$ > "$record/tool-ready"
	while :; do sleep 0.05; done
fi
echo %q
exit "$(cat "$config/%s" 2>/dev/null || echo 0)"
`, s.config, s.record, stubToolWaits, substituteFinalLine, substituteFinalLine, stubToolExit)
	writeStubFile(t, filepath.Join(s.bin, "go"), goScript.String(), 0o700)
	writeStubFile(t, filepath.Join(s.config, "tool"), tool, 0o700)
	return s
}

// writeStubFile writes one of the stub's files.
func writeStubFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil { //nolint:gosec // a stub file inside the test's temporary directory
		t.Fatal(err)
	}
}

// set creates the switch file name holding content.
func (s recipeStub) set(t *testing.T, name, content string) {
	t.Helper()
	writeStubFile(t, filepath.Join(s.config, name), content, 0o600)
}

// command is make with the given arguments in the repository root, with an
// environment of PATH (the fake go first), HOME, and TMPDIR only, plus extra.
func (s recipeStub) command(t *testing.T, extra []string, args ...string) *exec.Cmd {
	t.Helper()
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	cmd := exec.Command(makePath, args...) //nolint:gosec // make from PATH with test-chosen arguments
	cmd.Dir = repositoryRoot
	cmd.Env = append([]string{
		"PATH=" + s.bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"TMPDIR=" + s.tmp,
	}, extra...)
	return cmd
}

// recipeRun is what one make run of a coverage-compare target left.
type recipeRun struct {
	code     int
	stdout   string
	stderr   string
	goCalls  []recipeGoCall
	toolArgs []string // nil when the substitute tool did not run
}

// String gives make's output for a failure message.
func (r recipeRun) String() string {
	return fmt.Sprintf("stdout %q, stderr %q", r.stdout, r.stderr)
}

// recipeGoCall is one call of the fake go: its arguments and the recorded
// variables that were set.
type recipeGoCall struct {
	args []string
	env  map[string]string
}

// run runs make with args and returns what it and the stubs left.
func (s recipeStub) run(t *testing.T, extra []string, args ...string) recipeRun {
	t.Helper()
	cmd := s.command(t, extra, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	run := recipeRun{stdout: stdout.String(), stderr: stderr.String()}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		run.code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("make %v: %v", args, err)
	}
	run.goCalls = s.goCalls(t)
	run.toolArgs = s.recordedArgs(t, "tool")
	return run
}

// goCalls reads the fake go's record.
func (s recipeStub) goCalls(t *testing.T) []recipeGoCall {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.record, "go")) //nolint:gosec // a path inside the test's temporary directory
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls []recipeGoCall
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSuffix(line, "\n")
		if line == "call" {
			calls = append(calls, recipeGoCall{env: map[string]string{}})
			continue
		}
		if len(calls) == 0 {
			t.Fatalf("fake go record starts with %q", line)
		}
		last := &calls[len(calls)-1]
		if arg, ok := strings.CutPrefix(line, "arg "); ok {
			last.args = append(last.args, arg)
		} else if entry, ok := strings.CutPrefix(line, "env "); ok {
			name, value, _ := strings.Cut(entry, "=")
			last.env[name] = value
		} else {
			t.Fatalf("unexpected fake go record line %q", line)
		}
	}
	return calls
}

// recordedArgs reads an argument record, nil when the file is missing.
func (s recipeStub) recordedArgs(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.record, name)) //nolint:gosec // a path inside the test's temporary directory
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	args := []string{}
	for line := range strings.Lines(string(data)) {
		arg, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "arg ")
		if !ok {
			t.Fatalf("unexpected argument record line %q", line)
		}
		args = append(args, arg)
	}
	return args
}

// checkTmpEmpty fails the test when make's TMPDIR holds anything: the
// recipe's temporary directory must be gone.
func (s recipeStub) checkTmpEmpty(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(s.tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("TMPDIR holds %d entries after make returned, want none (first %q)", len(entries), entries[0].Name())
	}
}

// finalLine is the last line of out.
func finalLine(out string) string {
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	return lines[len(lines)-1]
}

// TestCoverageCompareRecipes runs the coverage-compare recipe with a fake go
// first on PATH, whose go build writes a substitute tool. It checks HEAD's
// go.mod check, the go environment of go list and go build, that BASE
// reaches the tool as one argument only from make's command line and never
// as shell syntax, the build failure, the reading of the tool's exit codes,
// and that every ending leaves TMPDIR empty; and that ext-coverage-compare,
// which runs the same recipe, passes the tool the word ext.
func TestCoverageCompareRecipes(t *testing.T) {
	t.Parallel()
	for _, stop := range []struct {
		name    string
		prepare func(t *testing.T, s *recipeStub)
		final   string
		goCalls int // go list, then go build
	}{
		{"go_list_fails", func(t *testing.T, s *recipeStub) { s.set(t, stubListFails, "") }, "coverage-compare: STOP toolchain", 1},
		{"build_fails", func(t *testing.T, s *recipeStub) { s.set(t, stubBuildFails, "") }, "coverage-compare: STOP internal", 2},
	} {
		t.Run(stop.name, func(t *testing.T) {
			t.Parallel()
			s := newRecipeStub(t)
			stop.prepare(t, &s)
			run := s.run(t, nil, recipeTarget, "BASE=HEAD~1")
			if run.code == 0 || finalLine(run.stdout) != stop.final {
				t.Errorf("exit %d, %s; want a non-zero exit and the final line %q", run.code, run, stop.final)
			}
			if len(run.goCalls) != stop.goCalls || run.toolArgs != nil {
				t.Errorf("go ran %d times and the tool ran %t; want %d go calls and no tool", len(run.goCalls), run.toolArgs != nil, stop.goCalls)
			}
			// Only the build failure gets as far as creating the
			// temporary directory.
			if stop.goCalls == 2 {
				s.checkTmpEmpty(t)
			}
		})
	}
	t.Run("base_from_command_line", func(t *testing.T) {
		t.Parallel()
		s := newRecipeStub(t)
		marker := filepath.Join(s.record, "injected")
		base := `x" ; touch ` + marker + ` ; echo "` + "`touch " + marker + "`" + ` ' ; touch ` + marker + ` ; echo ' y`
		run := s.run(t, nil, recipeTarget, "BASE="+base)
		if run.code != 0 || finalLine(run.stdout) != substituteFinalLine {
			t.Errorf("exit %d, %s; want exit 0 and the tool's final line", run.code, run)
		}
		if want := []string{recipeWord, "--base", base}; !slices.Equal(run.toolArgs, want) {
			t.Errorf("tool arguments = %q, want %q", run.toolArgs, want)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Error("a command inside BASE ran")
		}
		if len(run.goCalls) != 2 || run.goCalls[0].args[0] != "list" || run.goCalls[1].args[0] != "build" {
			t.Fatalf("go calls = %+v, want go list then go build", run.goCalls)
		}
		for _, call := range run.goCalls {
			if !maps.Equal(call.env, coverageCompareGoEnv) {
				t.Errorf("go %s environment = %v, want %v", call.args[0], call.env, coverageCompareGoEnv)
			}
		}
		s.checkTmpEmpty(t)

		dry := s.command(t, nil, "-n", recipeTarget, "BASE="+base)
		output, err := dry.CombinedOutput()
		if err != nil {
			t.Fatalf("make -n: %v\n%s", err, output)
		}
		if strings.Contains(string(output), base) {
			t.Errorf("make -n output holds BASE as recipe text:\n%s", output)
		}
	})
	// Neither a BASE from the environment nor a command-line
	// COVERAGE_COMPARE_BASE without BASE reaches the tool.
	for _, other := range []struct {
		name string
		env  []string
		args []string
	}{
		{"base_from_environment_only", []string{"BASE=HEAD~1"}, nil},
		{"variable_on_command_line", nil, []string{"COVERAGE_COMPARE_BASE=HEAD~1"}},
	} {
		t.Run(other.name, func(t *testing.T) {
			t.Parallel()
			s := newRecipeStub(t)
			run := s.run(t, other.env, append([]string{recipeTarget}, other.args...)...)
			if want := []string{recipeWord, "--base", ""}; !slices.Equal(run.toolArgs, want) {
				t.Errorf("tool arguments = %q, want %q", run.toolArgs, want)
			}
			s.checkTmpEmpty(t)
		})
	}
	for _, exit := range []struct {
		code     string
		zero     bool
		internal bool
	}{{"0", true, false}, {"1", false, false}, {"3", false, false}, {"2", false, true}} {
		t.Run("tool_exits_"+exit.code, func(t *testing.T) {
			t.Parallel()
			s := newRecipeStub(t)
			s.set(t, stubToolExit, exit.code)
			run := s.run(t, nil, recipeTarget, "BASE=HEAD~1")
			if (run.code == 0) != exit.zero {
				t.Errorf("make exit %d, want zero %t (%s)", run.code, exit.zero, run)
			}
			want := substituteFinalLine
			if exit.internal {
				want = "coverage-compare: STOP internal"
			}
			if got := finalLine(run.stdout); got != want {
				t.Errorf("final line = %q, want %q (%s)", got, want, run)
			}
			s.checkTmpEmpty(t)
		})
	}
	t.Run("ext_target_word", func(t *testing.T) {
		t.Parallel()
		s := newRecipeStub(t)
		run := s.run(t, nil, "ext-coverage-compare", "BASE=HEAD~1")
		if want := []string{"ext", "--base", "HEAD~1"}; run.code != 0 || !slices.Equal(run.toolArgs, want) {
			t.Errorf("exit %d, tool arguments %q (%s); want exit 0 and %q", run.code, run.toolArgs, run, want)
		}
	})
}

// TestCoverageCompareRecipeForwardsSignal starts the recipe with a
// substitute tool that waits, and sends SIGTERM to the make process alone:
// the recipe's shell forwards it to the tool and waits until the tool, which
// takes a moment to exit, has written its final line and exited. So when
// make returns, the tool has exited, its final line is the last line, and the
// recipe's temporary directory is gone.
func TestCoverageCompareRecipeForwardsSignal(t *testing.T) {
	s := newRecipeStub(t)
	s.set(t, stubToolWaits, "")
	cmd := s.command(t, nil, recipeTarget, "BASE=HEAD~1")
	// Files, not pipes: Wait would wait for a pipe to close, which the
	// tool holds open, so it would wait for the tool even when make
	// had returned before it.
	stdoutPath, stderrPath := filepath.Join(s.record, "make-stdout"), filepath.Join(s.record, "make-stderr")
	cmd.Stdout, cmd.Stderr = createFile(t, stdoutPath), createFile(t, stderrPath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	ready := filepath.Join(s.record, "tool-ready")
	if !waitForPath(ready) {
		t.Fatal("the substitute tool did not start")
	}
	// A recipe that does not forward the signal leaves the waiting
	// tool running; stop it once the test is over.
	if data, err := os.ReadFile(ready); err == nil { //nolint:gosec // a path inside the test's temporary directory
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
		}
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("make did not exit after SIGTERM")
	}
	// Checked before reading anything else, so a tool still running
	// has no time to finish.
	_, exitedErr := os.Stat(filepath.Join(s.record, "tool-exited"))
	stdout, stderr := readFile(t, stdoutPath), readFile(t, stderrPath)
	if exitedErr != nil {
		t.Error("make returned before the substitute tool exited")
	}
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		t.Errorf("make ended with %v, want a non-zero exit (stderr %q)", err, stderr)
	}
	if _, statErr := os.Stat(filepath.Join(s.record, "tool-term")); statErr != nil {
		t.Error("the substitute tool did not receive SIGTERM")
	}
	if got := finalLine(stdout); got != substituteFinalLine {
		t.Errorf("final line = %q, want %q (stdout %q, stderr %q)", got, substituteFinalLine, stdout, stderr)
	}
	s.checkTmpEmpty(t)
}

// createFile creates path for writing and closes it when the test ends.
func createFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Create(path) //nolint:gosec // a path inside the test's temporary directory
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// readFile returns path's content, failing the test when it cannot be read.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // a path inside the test's temporary directory
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// waitForPath reports whether path exists within 30 seconds, which covers
// make's start, go list, and the build on a loaded machine.
func waitForPath(path string) bool {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// TestCoverageCompareDocumented checks that CLAUDE.md lists each
// coverage-compare target in its own commands section, with the BASE
// argument. That the Makefile defines the targets is shown by
// TestCoverageCompareRecipes, which runs them, and by
// TestExtensionDocumentation for the ext- target; its claudeExtTargetRE reads
// the whole of CLAUDE.md, so only this test places the ext- target in
// "Extension Commands".
func TestCoverageCompareDocumented(t *testing.T) {
	claude := doctestutil.ReadDoc(t, claudePath)
	for _, tc := range []struct {
		target  string
		heading string
	}{
		{"coverage-compare", "### Test Commands"},
		{"ext-coverage-compare", "### Extension Commands"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			section := doctestutil.SectionByHeading(claude, tc.heading)
			if section == "" {
				t.Fatalf("CLAUDE.md has no %q section", tc.heading)
			}
			bullet := "- `make " + tc.target + " BASE=<commit>`"
			if !slices.ContainsFunc(strings.Split(section, "\n"), func(line string) bool { return strings.HasPrefix(line, bullet) }) {
				t.Errorf("CLAUDE.md %q does not list %s", tc.heading, bullet)
			}
		})
	}
}
