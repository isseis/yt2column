//go:build test

package main

import (
	"bytes"
	"errors"
	"io/fs"
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

// deadcodeNotFoundStdout is what the target prints, on standard output, when
// the deadcode command is missing from PATH.
const deadcodeNotFoundStdout = "Error: deadcode is required but not found\n" +
	"Please install deadcode: go install golang.org/x/tools/cmd/deadcode@latest\n"

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
	// wantSuccess is whether make must exit 0.
	wantSuccess bool
	// wantStdout is the exact standard output; the empty string means none.
	wantStdout string
	// wantStderrHas are strings standard error must contain.
	wantStderrHas []string
	// wantStderrLacks are strings standard error must not contain.
	wantStderrLacks []string
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
			name:          "no_report",
			status:        0,
			wantSuccess:   true,
			wantStubRan:   true,
			wantTempFiles: 2,
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
			name:            "deadcode_not_found",
			withoutDeadcode: true,
			wantStdout:      deadcodeNotFoundStdout,
			wantStubRan:     false,
			wantTempFiles:   0,
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
			switch {
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
			if got := stdout.String(); got != tc.wantStdout {
				t.Errorf("make stdout = %q, want %q", got, tc.wantStdout)
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

// writeDeadcodeStub writes the stub into dir, which must not exist yet.
func writeDeadcodeStub(t *testing.T, dir, stdout, stderr string, status int) *deadcodeStub {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create the stub directory: %v", err)
	}
	stdoutPath := filepath.Join(dir, "stdout")
	stderrPath := filepath.Join(dir, "stderr")
	for path, content := range map[string]string{stdoutPath: stdout, stderrPath: stderr} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
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

// writeRecordingMktemp writes the wrapper into dir, which must not exist yet.
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
