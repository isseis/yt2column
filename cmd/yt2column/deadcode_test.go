//go:build test

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// deadcodeArgs are the arguments the deadcode target must pass to the deadcode
// command: the whole module, with the test and integration build tags, so a
// function reached from any main package or any package's tests is reachable.
var deadcodeArgs = []string{"-test", "-tags", "test,integration", "./..."}

// Reasons the deadcode target must print. The target chooses one by the
// command's exit status and by whether it wrote anything, never by the shape
// of what it wrote, so a change to the report format cannot turn a failure
// into a success.
const (
	deadcodeReportReason = "Error: deadcode reported unreachable functions or produced unexpected output"
	deadcodeExitReason   = "Error: deadcode failed with exit status 1"
	deadcodeTempError    = "Error: cannot create a temporary file for deadcode output"
)

// deadcodeNotFoundMessage is what the target prints, on standard output, when
// the command DEADCODECMD names cannot be found. The first line names that
// value, which may be a path.
func deadcodeNotFoundMessage(command string) string {
	return "Error: " + command + " is required but not found\n" +
		"Please install deadcode: go install golang.org/x/tools/cmd/deadcode@latest\n"
}

// verdictCase is one way the deadcode command can end and what the target must
// do about it.
type verdictCase struct {
	// name is the subtest name.
	name string
	// stdout and stderr are what the stub writes.
	stdout, stderr string
	// status is the stub's exit status.
	status int
	// failMktempAt makes the recording mktemp fail on its n-th call (1-based);
	// 0 never fails.
	failMktempAt int
	// withoutDeadcode keeps the default command name and puts only make and sh
	// on PATH, so the recipe cannot find the command.
	withoutDeadcode bool
	// namedNotFound instead names a DEADCODECMD value that does not exist, on
	// the normal PATH, so the message must name that value.
	namedNotFound bool
	// wantSuccess is whether make must exit 0.
	wantSuccess bool
	// wantStdout is the exact standard output; the empty string means none.
	wantStdout string
	// wantStderrHas are strings standard error must contain.
	wantStderrHas []string
	// wantStderrLacks are strings standard error must not contain.
	wantStderrLacks []string
	// wantStderrEmpty requires standard error to be empty, which is what a
	// successful run prints: nothing on either stream.
	wantStderrEmpty bool
	// wantStubRan is whether the stub must have been executed.
	wantStubRan bool
	// wantTempFiles is how many temporary files the recipe must create.
	wantTempFiles int
}

// verdictCases are the rows of the verdict table: silence and exit 0 is the
// only success, and every other ending fails with the reason that matches its
// exit status.
func verdictCases() []verdictCase {
	const report = "internal/lib/lib.go:10:6: unreachable func: Unused\ninternal/lib/lib.go:20:6: unreachable func: AlsoUnused\n"
	const buildError = "deadcode: cannot load packages: build failed\n"
	return []verdictCase{
		{
			name:            "no_report",
			status:          0,
			wantSuccess:     true,
			wantStderrEmpty: true,
			wantStubRan:     true,
			wantTempFiles:   2,
		},
		{
			name:          "unreachable_report",
			stdout:        report,
			status:        0,
			wantStdout:    report,
			wantStderrHas: []string{deadcodeReportReason},
			wantStubRan:   true,
			wantTempFiles: 2,
		},
		{
			name:          "analysis_failure",
			stderr:        buildError,
			status:        1,
			wantStderrHas: []string{buildError, deadcodeExitReason},
			wantStubRan:   true,
			wantTempFiles: 2,
		},
		{
			name:          "exit_status_only",
			status:        1,
			wantStderrHas: []string{deadcodeExitReason},
			wantStubRan:   true,
			wantTempFiles: 2,
		},
		{
			// Another status, so the reason's number comes from the command
			// rather than from a literal 1 in the recipe.
			name:          "exit_status_other",
			status:        7,
			wantStderrHas: []string{"Error: deadcode failed with exit status 7"},
			wantStubRan:   true,
			wantTempFiles: 2,
		},
		{
			// The exit status decides the reason, so a report alongside a
			// failure is shown as the analysis failing, not as a report.
			name:          "exit_status_and_report",
			stdout:        report,
			status:        1,
			wantStdout:    report,
			wantStderrHas: []string{deadcodeExitReason},
			wantStderrLacks: []string{
				deadcodeReportReason,
			},
			wantStubRan:   true,
			wantTempFiles: 2,
		},
		{
			name:          "stderr_only",
			stderr:        buildError,
			status:        0,
			wantStderrHas: []string{buildError, deadcodeReportReason},
			wantStubRan:   true,
			wantTempFiles: 2,
		},
		{
			// A newline is one byte of output, which the target's byte-count
			// test sees where a command substitution would strip it.
			name:          "newline_only",
			stdout:        "\n",
			status:        0,
			wantStdout:    "\n",
			wantStderrHas: []string{deadcodeReportReason},
			wantStubRan:   true,
			wantTempFiles: 2,
		},
		{
			name:          "temporary_file_unavailable",
			status:        0,
			failMktempAt:  1,
			wantStderrHas: []string{deadcodeTempError},
			wantStderrLacks: []string{
				deadcodeExitReason,
			},
			wantStubRan:   false,
			wantTempFiles: 0,
		},
		{
			// The first file was created, so the recipe must remove it when the
			// second cannot be.
			name:          "second_temporary_file_unavailable",
			status:        0,
			failMktempAt:  2,
			wantStderrHas: []string{deadcodeTempError},
			wantStderrLacks: []string{
				deadcodeExitReason,
			},
			wantStubRan:   false,
			wantTempFiles: 1,
		},
		{
			// The restricted PATH has no mktemp either, so this row also pins
			// that the command is looked up before any temporary file is made.
			name:            "deadcode_not_found",
			withoutDeadcode: true,
			wantStdout:      deadcodeNotFoundMessage("deadcode"),
			wantStderrLacks: []string{deadcodeTempError},
		},
		{
			name:          "deadcode_not_found_named_value",
			namedNotFound: true,
		},
	}
}

// TestMakeDeadcodeVerdict checks the deadcode target's verdict: it succeeds
// only when the deadcode command exits 0 and writes nothing to either stream,
// and fails with a reason in every other case. Each case runs make with
// DEADCODECMD replaced by a stub that writes chosen bytes and exits with a
// chosen status, and checks make's exit code, standard output, the lines in
// standard error, the stub's arguments, whether the stub ran, and that every
// temporary file the recipe created was removed.
func TestMakeDeadcodeVerdict(t *testing.T) {
	// Read the Makefile from this process before running make: a file the test
	// process opens is part of the test's cache key, so editing the Makefile
	// alone cannot leave a cached PASS behind.
	if _, err := os.ReadFile(filepath.Join(repositoryRoot, "Makefile")); err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	// Resolve the real mktemp before any case changes PATH: the recording
	// wrapper calls it.
	realMktemp, err := exec.LookPath("mktemp")
	if err != nil {
		t.Fatalf("mktemp is not on PATH: %v", err)
	}

	for _, tc := range verdictCases() {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// A directory with a space in its name, so every case also checks
			// that DEADCODECMD reaches the shell quoted.
			stub := writeDeadcodeStub(t, filepath.Join(dir, "stub dir"), tc.stdout, tc.stderr, tc.status)
			recorder := writeRecordingMktemp(t, filepath.Join(dir, "bin"), realMktemp, tc.failMktempAt)

			var cmd *exec.Cmd
			wantStdout := tc.wantStdout
			switch {
			case tc.namedNotFound:
				// The value is a path with a space, and the message must name
				// it rather than the default command name.
				missing := filepath.Join(dir, "absent dir", "deadcode")
				wantStdout = deadcodeNotFoundMessage(missing)
				cmd = exec.Command(makePath, "deadcode", "DEADCODECMD="+missing)
				cmd.Env = envWithPath(os.Getenv("PATH"))
			case tc.withoutDeadcode:
				// make by absolute path, with only make and sh on PATH: the
				// recipe's own commands are shell builtins, so it fails before
				// reaching the stub. The same shape as the extension's make -n
				// check, which runs make without node on PATH.
				path := deadcodeAbsentPATH(t, filepath.Join(dir, "only"))
				cmd = exec.Command(makePath, "deadcode")
				cmd.Env = envWithPath(path)
			default:
				cmd = exec.Command(makePath, "deadcode", "DEADCODECMD="+stub.path)
				cmd.Env = envWithPath(recorder.dir + string(os.PathListSeparator) + os.Getenv("PATH"))
			}
			cmd.Dir = repositoryRoot
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()

			if success := err == nil; success != tc.wantSuccess {
				t.Fatalf("make error = %v (want success %t); stdout:\n%s\nstderr:\n%s",
					err, tc.wantSuccess, stdout.String(), stderr.String())
			}
			if got := stdout.String(); got != wantStdout {
				t.Errorf("make stdout = %q, want %q", got, wantStdout)
			}
			if tc.wantStderrEmpty && stderr.Len() != 0 {
				t.Errorf("make stderr = %q, want it empty on success", stderr.String())
			}
			for _, want := range tc.wantStderrHas {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("make stderr does not contain %q; stderr:\n%s", want, stderr.String())
				}
			}
			for _, unwanted := range tc.wantStderrLacks {
				if strings.Contains(stderr.String(), unwanted) {
					t.Errorf("make stderr contains %q; stderr:\n%s", unwanted, stderr.String())
				}
			}
			args, ran := readStubArgs(stub.argsPath)
			if ran != tc.wantStubRan {
				t.Fatalf("the deadcode stub ran = %t, want %t; stdout:\n%s\nstderr:\n%s",
					ran, tc.wantStubRan, stdout.String(), stderr.String())
			}
			if ran && !slices.Equal(args, deadcodeArgs) {
				t.Errorf("deadcode arguments = %q, want %q", args, deadcodeArgs)
			}

			created := recorder.created(t)
			if len(created) != tc.wantTempFiles {
				t.Errorf("the recipe created %d temporary files %q, want %d", len(created), created, tc.wantTempFiles)
			}
			for _, path := range created {
				if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("the recipe left the temporary file %s behind: stat error = %v", path, err)
				}
			}
		})
	}

	// DEADCODECMD is assigned with `=`, so a value in the environment cannot
	// silently replace the command: only the command line can. A failing stub
	// in the environment must be ignored in favour of the passing command on
	// PATH, which then records the expected arguments.
	t.Run("environment_does_not_replace_the_command", func(t *testing.T) {
		dir := t.TempDir()
		passing := writeDeadcodeStub(t, filepath.Join(dir, "passing stub"), "", "", 0)
		failing := writeDeadcodeStub(t, filepath.Join(dir, "failing stub"), "", "", 1)
		bin := filepath.Join(dir, "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatalf("create the PATH directory: %v", err)
		}
		if err := os.Symlink(passing.path, filepath.Join(bin, "deadcode")); err != nil {
			t.Fatalf("link the passing stub as deadcode: %v", err)
		}
		cmd := exec.Command(makePath, "deadcode")
		cmd.Dir = repositoryRoot
		cmd.Env = append(envWithPath(bin+string(os.PathListSeparator)+os.Getenv("PATH")), "DEADCODECMD="+failing.path)
		got := runDeadcodeCommand(t, cmd)
		if got.err != nil || len(got.stdout) != 0 || len(got.stderr) != 0 {
			t.Fatalf("make deadcode did not ignore the environment's DEADCODECMD and succeed with the command on PATH:\n%s", got.output())
		}
		args, ran := readStubArgs(passing.argsPath)
		if !ran || !slices.Equal(args, deadcodeArgs) {
			t.Errorf("the command on PATH recorded %q (ran %t), want %q", args, ran, deadcodeArgs)
		}
	})
}

// deadcodeStub is the fake deadcode command: a shell script that records its
// arguments, writes the case's standard output and standard error, and exits
// with the case's status.
type deadcodeStub struct {
	// path is the script, to pass as DEADCODECMD.
	path string
	// argsPath is where the script records its arguments, one per line. It
	// exists only if the script ran.
	argsPath string
}

// writeDeadcodeStub writes the stub into dir, creating it.
func writeDeadcodeStub(t *testing.T, dir, stdout, stderr string, status int) *deadcodeStub {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create the stub directory: %v", err)
	}
	stdoutPath := filepath.Join(dir, "stdout")
	stderrPath := filepath.Join(dir, "stderr")
	for path, content := range map[string]string{stdoutPath: stdout, stderrPath: stderr} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	stub := &deadcodeStub{
		path:     filepath.Join(dir, "deadcode"),
		argsPath: filepath.Join(dir, "args"),
	}
	script := "#!/bin/sh\n" +
		": > " + shellQuote(stub.argsPath) + "\n" +
		"for arg in \"$@\"; do\n" +
		"\tprintf '%s\\n' \"$arg\" >> " + shellQuote(stub.argsPath) + "\n" +
		"done\n" +
		"cat " + shellQuote(stdoutPath) + "\n" +
		"cat " + shellQuote(stderrPath) + " >&2\n" +
		"exit " + strconv.Itoa(status) + "\n"
	if err := os.WriteFile(stub.path, []byte(script), 0o755); err != nil {
		t.Fatalf("write the stub: %v", err)
	}
	return stub
}

// readStubArgs reads the arguments the stub recorded. It reports false when
// the stub did not run.
func readStubArgs(path string) ([]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var args []string
	for line := range strings.Lines(string(data)) {
		args = append(args, strings.TrimSuffix(line, "\n"))
	}
	return args, true
}

// recordingMktemp is a wrapper placed first on PATH: it calls the real mktemp,
// records every path it creates, and can fail on a chosen call.
type recordingMktemp struct {
	// dir holds the wrapper, to prepend to PATH.
	dir string
	// recordPath lists the paths the wrapper created, one per line.
	recordPath string
}

// writeRecordingMktemp writes the wrapper into dir, creating it.
// failAt is the 1-based call to fail at; 0 never fails.
func writeRecordingMktemp(t *testing.T, dir, realMktemp string, failAt int) *recordingMktemp {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create the mktemp wrapper directory: %v", err)
	}
	recorder := &recordingMktemp{
		dir:        dir,
		recordPath: filepath.Join(dir, "created"),
	}
	callsPath := filepath.Join(dir, "calls")
	script := "#!/bin/sh\n" +
		"count=0\n" +
		"if [ -f " + shellQuote(callsPath) + " ]; then\n" +
		"\tread -r count < " + shellQuote(callsPath) + "\n" +
		"fi\n" +
		"count=$((count + 1))\n" +
		"printf '%s' \"$count\" > " + shellQuote(callsPath) + "\n" +
		"if [ \"$count\" -eq " + strconv.Itoa(failAt) + " ]; then\n" +
		"\techo 'mktemp: simulated failure' >&2\n" +
		"\texit 1\n" +
		"fi\n" +
		"path=$(" + shellQuote(realMktemp) + " \"$@\") || exit $?\n" +
		"printf '%s\\n' \"$path\" >> " + shellQuote(recorder.recordPath) + "\n" +
		"printf '%s\\n' \"$path\"\n"
	if err := os.WriteFile(filepath.Join(dir, "mktemp"), []byte(script), 0o755); err != nil {
		t.Fatalf("write the mktemp wrapper: %v", err)
	}
	return recorder
}

// created returns the paths the wrapper created, in order.
func (r *recordingMktemp) created(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(r.recordPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		t.Fatalf("read %s: %v", r.recordPath, err)
	}
	var paths []string
	for line := range strings.Lines(string(data)) {
		if path := strings.TrimSuffix(line, "\n"); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

// deadcodeAbsentPATH returns a directory holding only symlinks to make and sh,
// to be the whole PATH of a make child. The recipe checks for the command
// before it runs anything, so with only these two it reports the command
// missing.
func deadcodeAbsentPATH(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create the PATH directory: %v", err)
	}
	for _, name := range []string{"make", "sh"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("%s is not on PATH: %v", name, err)
		}
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatalf("link %s: %v", name, err)
		}
	}
	return dir
}

// envWithPath returns the allowlisted parent environment with PATH replaced by
// path, so every make run in this test starts from the same base and differs
// only in PATH.
func envWithPath(path string) []string {
	env := allowlistedEnv()
	for i, entry := range env {
		if strings.HasPrefix(entry, "PATH=") {
			env[i] = "PATH=" + path
			return env
		}
	}
	return append(env, "PATH="+path)
}

// shellQuote quotes s for the stub shell scripts, whose paths come from
// t.TempDir and contain a space.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// deadcodeAssumedVersion is the golang.org/x/tools version whose reachability
// decisions the fixture below records. A failure that names another version
// points at the version as the cause.
const deadcodeAssumedVersion = "v0.47.0"

// deadcodeFixtureFiles is the fixture module the scope test analyses: a copy of
// the module's shape, small enough to reason about. Each function is reachable
// from exactly one kind of root, so a report about one says which root the
// target failed to count.
var deadcodeFixtureFiles = map[string]string{
	// No require and no toolchain line, and a go line no newer than the
	// repository's, so go list never downloads anything.
	"go.mod": "module example.com/fixture\n\ngo 1.22\n",

	// cmd/cli stands in for cmd/yt2column: the entry point the old arguments
	// already analysed.
	"cmd/cli/main.go": `package main

import "example.com/fixture/internal/lib"

func main() {
	lib.FromCLI()
}
`,

	// cmd/srv is a second main package: reachable only if the whole module is
	// analysed.
	"cmd/srv/main.go": `package main

import "example.com/fixture/internal/lib"

func main() {
	lib.OnlySrv()
}
`,

	"internal/lib/lib.go": `package lib

// FromCLI is reached from the CLI main package.
func FromCLI() {}

// OnlySrv is reached only from the server main package.
func OnlySrv() {}

// OnlyTest is reached only from this package's test.
func OnlyTest() {}

// OnlyIntegration is reached only from a test built with the integration tag.
func OnlyIntegration() {}
`,

	"internal/lib/lib_test.go": `package lib_test

import (
	"testing"

	"example.com/fixture/internal/lib"
)

func TestOnlyTest(t *testing.T) {
	lib.OnlyTest()
}
`,

	"internal/lib/integration_test.go": `//go:build integration

package lib_test

import (
	"testing"

	"example.com/fixture/internal/lib"
)

func TestOnlyIntegration(t *testing.T) {
	lib.OnlyIntegration()
}
`,

	// internal/helper has no test of its own: only another package's test
	// reaches Helper.
	"internal/helper/helper.go": `//go:build test

package helper

// Helper is reached only from another package's test.
func Helper() {}
`,

	"internal/other/other.go": `package other

// Other is reached from this package's own test.
func Other() {}
`,

	"internal/other/other_test.go": `package other_test

import (
	"testing"

	"example.com/fixture/internal/helper"
	"example.com/fixture/internal/other"
)

func TestOther(t *testing.T) {
	other.Other()
	helper.Helper()
}
`,
}

// deadcodeRun is one run of the deadcode target or of the deadcode command.
type deadcodeRun struct {
	err    error
	stdout []byte
	stderr []byte
}

// output returns both streams, labelled, for a failure message.
func (r deadcodeRun) output() string {
	return fmt.Sprintf("exit error: %v\nstandard output:\n%s\nstandard error:\n%s", r.err, r.stdout, r.stderr)
}

// deadcodeScope is where the scope test finds the real deadcode command.
type deadcodeScope struct {
	path    string
	version string
	make    string
	// makefile is the absolute path of the repository's Makefile, passed with
	// -f so -C can point make at the fixture module.
	makefile string
}

// note says which deadcode version the failure was observed with, so a
// reachability change between versions is not read as a defect in the target.
func (s *deadcodeScope) note() string {
	return "the deadcode command is " + s.version + "; this test records " + deadcodeAssumedVersion
}

// runTarget runs the repository's deadcode target in the fixture module root.
func (s *deadcodeScope) runTarget(t *testing.T, root string) deadcodeRun {
	t.Helper()
	cmd := exec.Command(s.make, "-s", "-C", root, "-f", s.makefile, "deadcode")
	cmd.Env = envWithPath(os.Getenv("PATH"))
	return runDeadcodeCommand(t, cmd)
}

// run itself runs the deadcode command in root, to check a premise about which
// root a function is reachable from.
func (s *deadcodeScope) run(t *testing.T, root string, args ...string) deadcodeRun {
	t.Helper()
	cmd := exec.Command(s.path, args...)
	cmd.Dir = root
	cmd.Env = envWithPath(os.Getenv("PATH"))
	return runDeadcodeCommand(t, cmd)
}

// runDeadcodeCommand runs cmd and captures both streams.
func runDeadcodeCommand(t *testing.T, cmd *exec.Cmd) deadcodeRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return deadcodeRun{err: err, stdout: stdout.Bytes(), stderr: stderr.Bytes()}
}

// TestMakeDeadcodeScope checks which functions the real deadcode command counts
// as reachable through the deadcode target. It needs the deadcode command, so
// it skips where that is not on PATH (CI does not install it); the environment
// check is the PATH lookup alone.
//
// Each function in the fixture is reachable from exactly one root: the CLI main
// package, a second main package, another package's test, its own package's
// test, or an integration test. A report about one therefore names the root the
// target failed to count. The isolation subtests first confirm that claim by
// excluding that root from a direct deadcode run: without that, a function
// another root also reached would stay silent, and the silence would not show
// that the target counted this root.
//
// A test result is cached on the files this process opened and the environment,
// so installing or upgrading the deadcode command can leave a cached SKIP or
// PASS behind: rerun with -count=1 after either.
func TestMakeDeadcodeScope(t *testing.T) {
	deadcodePath, err := exec.LookPath("deadcode")
	if err != nil {
		t.Skip("the deadcode command is not on PATH; install it with go install golang.org/x/tools/cmd/deadcode@latest, then rerun with -count=1")
	}
	// Read the Makefile from this process so editing it alone cannot leave a
	// cached PASS behind.
	if _, err := os.ReadFile(filepath.Join(repositoryRoot, "Makefile")); err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatalf("make is not on PATH: %v", err)
	}
	// The fixture's go list runs in the child, so go must be there too.
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go is not on PATH: %v", err)
	}
	makefile, err := filepath.Abs(filepath.Join(repositoryRoot, "Makefile"))
	if err != nil {
		t.Fatalf("resolve the Makefile path: %v", err)
	}
	scope := &deadcodeScope{
		path:     deadcodePath,
		version:  deadcodeModuleVersion(t, deadcodePath),
		make:     makePath,
		makefile: makefile,
	}

	// The fixture as written must be silent: nothing is reported, so all four
	// functions its roots reach are counted as reachable. OnlySrv is reached
	// from cmd/srv, Helper from another package's test, OnlyTest from its own
	// package's test, and OnlyIntegration from an integration test. The
	// isolation subtests below show each root is the only one reaching its
	// function, so one run covers all four; a report names the function, and
	// the failure message prints it.
	t.Run("silent_success", func(t *testing.T) {
		root := writeDeadcodeFixture(t, nil)
		got := scope.runTarget(t, root)
		if got.err != nil {
			t.Fatalf("make deadcode failed on the fixture as written (%s):\n%s", scope.note(), got.output())
		}
		if len(got.stdout) != 0 || len(got.stderr) != 0 {
			t.Fatalf("make deadcode printed on the fixture as written; success must be silent, so a function the fixture reaches was reported (%s):\n%s", scope.note(), got.output())
		}
	})

	// The isolation premise: each function must be reported once its own root
	// is left out of the analysis, so the silent success above shows the target
	// counting that root.
	t.Run("isolation", func(t *testing.T) {
		root := writeDeadcodeFixture(t, nil)
		for _, tc := range []struct {
			name      string
			functions []string
			args      []string
		}{
			{
				// The old argument list, widened only with the integration tag
				// and the internal packages: cmd/srv is the one excluded.
				name:      "other_entry_point_excluded",
				functions: []string{"OnlySrv"},
				args:      []string{"-test", "-tags", "test,integration", "./cmd/cli", "./internal/..."},
			},
			{
				// The previous target's argument list.
				name:      "cli_entry_point_only",
				functions: []string{"OnlySrv"},
				args:      []string{"-test", "-tags", "test", "./cmd/cli"},
			},
			{
				// Tests are not roots without -test, so no test reaches these
				// two, whether the test is in the helper's package or another.
				name:      "tests_excluded",
				functions: []string{"Helper", "OnlyTest"},
				args:      []string{"-tags", "test,integration", "./..."},
			},
			{
				// Without the integration tag the integration test is not
				// built, so nothing reaches OnlyIntegration.
				name:      "integration_tag_excluded",
				functions: []string{"OnlyIntegration"},
				args:      []string{"-test", "-tags", "test", "./..."},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := scope.run(t, root, tc.args...)
				if got.err != nil {
					t.Fatalf("deadcode %q failed (%s):\n%s", tc.args, scope.note(), got.output())
				}
				for _, function := range tc.functions {
					if !bytes.Contains(got.stdout, []byte(function)) {
						t.Errorf("deadcode %q did not report %s, so the fixture does not isolate it from the other roots (%s):\n%s",
							tc.args, function, scope.note(), got.output())
					}
				}
			})
		}
	})

	t.Run("reported", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			file     string
			content  string
			line     int
			function string
		}{
			{
				// A package-level function in an ordinary file that nothing
				// references.
				name:     "package_function",
				file:     "internal/lib/unused.go",
				content:  "package lib\n\nfunc Unused() {}\n",
				line:     3,
				function: "Unused",
			},
			{
				// A function in a main package other than the CLI's, referenced
				// by nothing.
				name:     "other_entry_point_function",
				file:     "cmd/srv/unused.go",
				content:  "package main\n\nfunc unusedFromSrv() {}\n",
				line:     3,
				function: "unusedFromSrv",
			},
			{
				// A function in an integration-tagged file, referenced by
				// nothing.
				name:     "integration_tagged_function",
				file:     "internal/lib/unused_integration.go",
				content:  "//go:build integration\n\npackage lib\n\nfunc UnusedIntegration() {}\n",
				line:     5,
				function: "UnusedIntegration",
			},
			{
				// A function in a package outside the entry points and apart
				// from the packages they import, so a narrowed package list
				// that still covers the CLI is caught.
				name:     "package_outside_the_entry_points",
				file:     "internal/other/unused.go",
				content:  "package other\n\nfunc UnusedInOther() {}\n",
				line:     3,
				function: "UnusedInOther",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := writeDeadcodeFixture(t, map[string]string{tc.file: tc.content})
				got := scope.runTarget(t, root)
				if got.err == nil {
					t.Fatalf("make deadcode succeeded although nothing reaches %s, so a report would be missed (%s):\n%s",
						tc.function, scope.note(), got.output())
				}
				location := fmt.Sprintf("%s:%d:", tc.file, tc.line)
				if !bytes.Contains(got.stdout, []byte(location)) || !bytes.Contains(got.stdout, []byte(tc.function)) {
					t.Fatalf("make deadcode did not report %s at %s (%s):\n%s",
						tc.function, location, scope.note(), got.output())
				}
			})
		}
	})

	t.Run("compile_error", func(t *testing.T) {
		root := writeDeadcodeFixture(t, map[string]string{
			"internal/lib/broken.go": "package lib\n\nfunc Broken() { anywhere() }\n",
		})
		got := scope.runTarget(t, root)
		if got.err == nil {
			t.Fatalf("make deadcode succeeded although the fixture does not compile (%s):\n%s", scope.note(), got.output())
		}
		if !bytes.Contains(got.stderr, []byte("undefined: anywhere")) {
			// The compiler's diagnostic names the fixture's own undefined
			// identifier, so the wording is the payload the recipe must forward
			// unchanged; there is no error type to check.
			t.Fatalf("make deadcode did not show the deadcode command's compiler error (%s):\n%s", scope.note(), got.output())
		}
	})
}

// writeDeadcodeFixture writes the fixture module into a new temporary
// directory and returns its root. extra holds files that replace or extend the
// base fixture, keyed by slash-separated path.
func writeDeadcodeFixture(t *testing.T, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := maps.Clone(deadcodeFixtureFiles)
	maps.Copy(files, extra)
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create the directory of %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

// deadcodeModuleVersion returns the module and version the deadcode command was
// built from, as go version -m reports them. The version is a diagnostic for a
// failure that may come from a different deadcode version, not a premise of the
// test, so a command that reports none (a wrapper script, or a binary built
// without build info) yields a message instead of failing the test.
func deadcodeModuleVersion(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("go", "version", "-m", path)
	cmd.Env = envWithPath(os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("%s: go version -m failed (%v)", path, err)
	}
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "mod" {
			return fields[1] + " " + fields[2]
		}
	}
	return fmt.Sprintf("%s: go version -m printed no module line", path)
}
