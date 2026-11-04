//go:build test

package coveragecompare

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/strictjson"
)

// The versions the fakes report, which the fixtures pin.
const (
	fakeGoVersion   = "go1.27.1"
	fakeNodeVersion = "v24.21.0"
	fakeNpmVersion  = "11.19.0"
)

// goRepo creates a Go repository whose HEAD deletes B from x/x.go
// (goBaseText to goHeadText) and returns it with BASE.
func goRepo(t *testing.T) (*testRepo, string) {
	t.Helper()
	r := newTestRepo(t)
	r.write("go.mod", "module "+judgeModule+"\n\ngo 1.21\n")
	r.write(judgeGoFile, goBaseText)
	base := r.commitAll("base")
	r.write(judgeGoFile, goHeadText)
	r.commitAll("head")
	return r, base
}

// goProfileText is a Go profile of x/x.go with the blocks.
func goProfileText(blocks ...string) string {
	var b strings.Builder
	b.WriteString("mode: set\n")
	for _, block := range blocks {
		fmt.Fprintf(&b, "%s/%s:%s\n", judgeModule, judgeGoFile, block)
	}
	return b.String()
}

// goBehavior is the fakes of a passing Go run over goRepo: B did not run at
// BASE, and A and C stay covered.
func goBehavior() fakeBehavior {
	base, head := goPassing()
	return fakeBehavior{
		GoVersion: fakeGoVersion,
		Suites: map[string]fakeSuite{
			"base": {Stdout: goJSONPass(2), Report: goProfileText(base.blocks...)},
			"head": {Stdout: goJSONPass(1), Report: goProfileText(head.blocks...)},
		},
	}
}

// goDeleted is what a passing run over goRepo lists as deleted.
var goDeleted = []DeletedFunc{{ID: FuncID{File: judgeGoFile, Line: 7}, Names: []string{"B"}, State: NotRunAtBase}}

// extRepo creates an extension repository whose HEAD deletes b from
// extension/src/a.ts (extBaseText to extHeadText), with the user's
// extension/node_modules installed and ignored, and returns it with BASE.
func extRepo(t *testing.T) (*testRepo, string) {
	t.Helper()
	r := newTestRepo(t)
	r.write(".gitignore", "extension/node_modules/\n")
	r.write(nodeVersionFile, strings.TrimPrefix(fakeNodeVersion, "v")+"\n")
	r.write(packageJSONFile, `{"packageManager":"npm@`+fakeNpmVersion+`"}`+"\n")
	r.write(lockfileFile, "{}\n")
	r.write(judgeExtFile, extBaseText)
	base := r.commitAll("base")
	r.write(judgeExtFile, extHeadText)
	r.commitAll("head")
	r.write("extension/node_modules/"+installedLockName, "{}\n")
	r.write("extension/node_modules/typescript/package.json", "{}\n")
	return r, base
}

// lcovText is an lcov report of extension/src/a.ts with the FN records
// "<line>,<name>,<count>".
func lcovText(fns ...string) string {
	var b strings.Builder
	b.WriteString("TN:\nSF:src/a.ts\n")
	var counts []string
	for _, fn := range fns {
		line, rest, _ := strings.Cut(fn, ",")
		name, count, _ := strings.Cut(rest, ",")
		fmt.Fprintf(&b, "FN:%s,%s\n", line, name)
		counts = append(counts, fmt.Sprintf("FNDA:%s,%s\n", count, name))
	}
	b.WriteString(strings.Join(counts, ""))
	b.WriteString("end_of_record\n")
	return b.String()
}

// rangesText is the range helper's answer for extension/src/a.ts.
func rangesText(ranges []lineRange) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		parts = append(parts, fmt.Sprintf(`{"start":%d,"end":%d}`, r.Start, r.End))
	}
	return `{"files":[{"path":"src/a.ts","functions":[` + strings.Join(parts, ",") + `],"fieldGroups":[]}]}`
}

// extBehavior is the fakes of a passing extension run over extRepo: a, b,
// and c ran at BASE, and a and c still run at HEAD.
func extBehavior() fakeBehavior {
	base, head := extPassing()
	return fakeBehavior{
		NodeVersion: fakeNodeVersion,
		NpmVersion:  fakeNpmVersion,
		Suites: map[string]fakeSuite{
			"base": {Report: lcovText(base.fns...), TAP: tapReportText(2, 0, 0)},
			"head": {Report: lcovText(head.fns...), TAP: tapReportText(1, 0, 0)},
		},
		Ranges: map[string]fakeAnswer{
			"base": {Stdout: rangesText(base.ranges)},
			"head": {Stdout: rangesText(head.ranges)},
		},
	}
}

// extDeleted is what a passing run over extRepo lists as deleted.
var extDeleted = []DeletedFunc{{ID: FuncID{File: judgeExtFile, Line: 5}, Names: []string{"b"}, State: RanAtBase}}

// runTool runs the tool over repo with the test's environment.
func runTool(ctx context.Context, env *fakeEnv, repo *testRepo, target Target, base string) Result {
	return Run(ctx, Options{Target: target, Base: base, RepoRoot: repo.dir, Env: env.environ})
}

// suiteStarts returns the starts that ran a test suite.
func suiteStarts(starts []fakeStart) []fakeStart {
	var suites []fakeStart
	for _, start := range starts {
		if start.suite() {
			suites = append(suites, start)
		}
	}
	return suites
}

// requireNpmVersionOnly fails the test unless every npm start was npm
// --version: the tool installs nothing.
func requireNpmVersionOnly(t *testing.T, starts []fakeStart) {
	t.Helper()
	for _, start := range starts {
		if start.Name == "npm" && !slices.Equal(start.Args, []string{"--version"}) {
			t.Errorf("npm started with %q, want only --version", start.Args)
		}
	}
}

// requireStopKind fails the test unless r stopped with kind.
func requireStopKind(t *testing.T, r Result, kind StopKind) {
	t.Helper()
	if r.Outcome != OutcomeStop || r.Stop != kind {
		t.Fatalf("result = outcome %v stop %v (reasons %+v), want stop %v", r.Outcome, r.Stop, r.Reasons, kind)
	}
}

// TestRunEndToEnd runs the tool over a repository with the fakes, from the
// commands to the result: the values the result reports come from the fakes'
// answers and the repository, the suites run as the design gives them, and the
// pinned commit IDs are used even when BASE's ref moves during the run.
func TestRunEndToEnd(t *testing.T) {
	t.Parallel()
	goLost, extLost := goBehavior(), extBehavior()
	goLost.Suites["head"], extLost.Suites["head"] = fixtures[0].lost, fixtures[1].lost

	for _, tc := range []struct {
		name     string
		target   Target
		behavior fakeBehavior
		want     Outcome
		lost     []LostFunc
	}{
		{name: "go pass", target: TargetGo, behavior: goBehavior(), want: OutcomePass},
		{
			name: "go fail", target: TargetGo, behavior: goLost, want: OutcomeFail,
			lost: []LostFunc{{
				ID: FuncID{File: judgeGoFile, Line: 3}, Names: []string{"A"},
				Before: Coverage{Unit: UnitStatements, Covered: 1, Total: 1}, After: Coverage{Unit: UnitStatements, Covered: 0, Total: 1},
			}},
		},
		{name: "ext pass", target: TargetExt, behavior: extBehavior(), want: OutcomePass},
		{
			name: "ext fail", target: TargetExt, behavior: extLost, want: OutcomeFail,
			lost: []LostFunc{{
				ID: FuncID{File: judgeExtFile, Line: 1}, Names: []string{"a"},
				Before: Coverage{Unit: UnitFunctions, Covered: 1, Total: 1}, After: Coverage{Unit: UnitFunctions, Covered: 0, Total: 1},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var repo *testRepo
			var base string
			deleted, toolchain := goDeleted, fakeGoVersion
			want := Result{Base: Measure{Tests: 2, Files: 1, Functions: 3}, Head: Measure{Tests: 1, Files: 1, Functions: 2}}
			if tc.target == TargetGo {
				repo, base = goRepo(t)
			} else {
				repo, base = extRepo(t)
				deleted, toolchain = extDeleted, fakeNodeVersion
			}
			// BASE is given as a branch that the toolchain check moves to
			// HEAD, before the checkouts are made: the checkouts, the diff,
			// and the result keep the commit ID the run resolved first.
			repo.git("branch", "basebranch", base)
			b := tc.behavior
			b.VersionGit, b.GitDir = [][]string{{"branch", "-f", "basebranch", "HEAD"}}, repo.dir
			env := newFakeEnv(t)
			env.set(b)

			r := runTool(context.Background(), env, repo, tc.target, "basebranch")

			if r.Outcome != tc.want {
				t.Fatalf("outcome = %v, want %v (stop %v, reasons %+v)", r.Outcome, tc.want, r.Stop, r.Reasons)
			}
			want.Outcome, want.Target, want.BaseArg = tc.want, tc.target, "basebranch"
			want.BaseCommit, want.HeadCommit, want.HeadTree = base, repo.rev("HEAD"), repo.rev("HEAD^{tree}")
			want.Toolchain, want.Deleted, want.Lost = toolchain, deleted, tc.lost
			if !reflect.DeepEqual(r, want) {
				t.Errorf("result =\n%+v\nwant\n%+v", r, want)
			}
			starts := env.starts()
			requireNpmVersionOnly(t, starts)
			suites := suiteStarts(starts)
			requireSuiteCommands(t, tc.target, env, repo, base, suites)
			if tc.target == TargetExt {
				requireRangeHelperCommands(t, starts, filepath.Dir(filepath.Dir(suites[1].Dir)))
			}
			env.requireTmpEmpty()
		})
	}
}

// requireSuiteCommands checks the two suite starts: BASE then HEAD, each in
// its own checkout of the pinned commit under the work directory and never in
// the user's repository, with the design's command line and the reports under the work
// directory. An extension checkout's node_modules is a real directory whose
// entries link to the user's.
func requireSuiteCommands(t *testing.T, target Target, env *fakeEnv, repo *testRepo, base string, suites []fakeStart) {
	t.Helper()
	if len(suites) != 2 {
		t.Fatalf("suite starts = %d, want 2", len(suites))
	}
	for i, point := range []string{"base", "head"} {
		start := suites[i]
		if want := map[string]string{"base": base, "head": repo.rev("HEAD")}[point]; start.Head != want {
			t.Errorf("%s suite ran on commit %s, want %s", point, start.Head, want)
		}
		checkout := filepath.Dir(start.Dir)
		if target == TargetGo {
			checkout = start.Dir
		}
		workDir := filepath.Dir(checkout)
		if filepath.Base(checkout) != point || filepath.Dir(workDir) != env.tmp || !strings.HasPrefix(filepath.Base(workDir), workPrefix) {
			t.Errorf("%s suite ran in %s, want <TMPDIR>/%s*/%s", point, start.Dir, workPrefix, point)
		}
		var want []string
		if target == TargetGo {
			want = []string{
				"test", "-tags", "test", "-count=1", "-json", "-coverpkg=./...",
				"-coverprofile=" + filepath.Join(workDir, point+".cover"), "./...",
			}
		} else {
			want = []string{
				"--test", "--experimental-test-coverage",
				"--test-reporter=lcov", "--test-reporter-destination=" + filepath.Join(workDir, point+".lcov"),
				"--test-reporter=tap", "--test-reporter-destination=" + filepath.Join(workDir, point+".tap"),
				"test/**/*.test.ts",
			}
			if start.Dir != filepath.Join(checkout, "extension") {
				t.Errorf("%s suite ran in %s, want the checkout's extension directory", point, start.Dir)
			}
			modules := start.NodeModules
			userModules := filepath.Join(repo.dir, "extension", "node_modules")
			wantLinks := map[string]string{
				installedLockName: filepath.Join(userModules, installedLockName),
				"typescript":      filepath.Join(userModules, "typescript"),
			}
			if modules == nil || !modules.RealDir || !reflect.DeepEqual(modules.Links, wantLinks) {
				t.Errorf("%s node_modules = %+v, want a real directory with links %v", point, modules, wantLinks)
			}
		}
		if !slices.Equal(start.Args, want) {
			t.Errorf("%s suite args =\n%q\nwant\n%q", point, start.Args, want)
		}
	}
}

// requireRangeHelperCommands checks the range helper's starts in the work
// directory the suites ran in: one per
// point, BASE first, each running the HEAD checkout's helper from the HEAD
// checkout's extension directory, so typescript comes from there, over the
// point's own checkout, with the product file relative to extension/.
func requireRangeHelperCommands(t *testing.T, starts []fakeStart, workDir string) {
	t.Helper()
	var helpers []fakeStart
	for _, start := range starts {
		if start.Name == fakeNode && len(start.Args) > 0 && filepath.Base(start.Args[0]) == "function-ranges.ts" {
			helpers = append(helpers, start)
		}
	}
	if len(helpers) != 2 {
		t.Fatalf("range helper starts = %d, want 2", len(helpers))
	}
	headExtension := filepath.Join(workDir, "head", "extension")
	for i, point := range []string{"base", "head"} {
		start := helpers[i]
		want := []string{
			filepath.Join(headExtension, "scripts", "function-ranges.ts"),
			"--root", filepath.Join(workDir, point, "extension"),
			"--", "src/a.ts",
		}
		if start.Dir != headExtension || !slices.Equal(start.Args, want) {
			t.Errorf("%s range helper ran in %s with %q, want %s with %q", point, start.Dir, start.Args, headExtension, want)
		}
	}
}

// fixture is one target's repository and passing fakes.
type fixture struct {
	name     string
	target   Target
	repo     func(*testing.T) (*testRepo, string)
	behavior func() fakeBehavior
	// failing is a suite that ran and failed TestBroken, or the test
	// "broken".
	failing fakeSuite
	// unreadable is a suite whose report cannot be read.
	unreadable fakeSuite
	// lost is a HEAD suite under which the first function no longer runs.
	lost fakeSuite
}

// fixtures are the Go and the extension fixtures.
var fixtures = []fixture{
	{
		name: "go", target: TargetGo, repo: goRepo, behavior: goBehavior,
		failing: fakeSuite{
			Exit:   1,
			Stdout: `{"Action":"fail","Package":"` + judgeModule + `/x","Test":"TestBroken"}` + "\n",
			Report: goProfileText(),
		},
		unreadable: fakeSuite{Stdout: goJSONPass(1), Report: "not a profile\n"},
		lost:       fakeSuite{Stdout: goJSONPass(1), Report: goProfileText("3.14,5.2 1 0", "7.14,9.2 1 1")},
	},
	{
		name: "ext", target: TargetExt, repo: extRepo, behavior: extBehavior,
		failing:    fakeSuite{Exit: 1, Report: lcovText(), TAP: "not ok 1 - broken\n" + tapReportText(1, 1, 0)},
		unreadable: fakeSuite{Report: "not lcov\n", TAP: tapReportText(1, 0, 0)},
		lost:       fakeSuite{Report: lcovText("1,a,0", "5,c,2"), TAP: tapReportText(1, 0, 0)},
	},
}

// failedTest is the name the failing suites give their failed test.
func (f fixture) failedTest() string {
	if f.target == TargetGo {
		return "TestBroken"
	}
	return "broken"
}

// TestRunStopsOnBaseFailure checks that a failure of the BASE suite stops the
// run as test-failed at BASE before the HEAD suite runs.
func TestRunStopsOnBaseFailure(t *testing.T) {
	t.Parallel()
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			repo, base := f.repo(t)
			b := f.behavior()
			b.Suites["base"] = f.failing
			env := newFakeEnv(t)
			env.set(b)

			r := runTool(context.Background(), env, repo, f.target, base)

			requireStopKind(t, r, StopTestFailed)
			if len(r.Reasons) != 1 || r.Reasons[0].Point != PointBase || r.Reasons[0].Test != f.failedTest() {
				t.Errorf("reasons = %+v, want one at BASE naming %q", r.Reasons, f.failedTest())
			}
			if suites := suiteStarts(env.starts()); len(suites) != 1 {
				t.Errorf("suite starts = %d, want only BASE's", len(suites))
			}
		})
	}
}

// TestRunStopsOnHeadFailure checks that a failure of the HEAD suite alone
// stops the run as test-failed at HEAD.
func TestRunStopsOnHeadFailure(t *testing.T) {
	t.Parallel()
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			repo, base := f.repo(t)
			b := f.behavior()
			b.Suites["head"] = f.failing
			env := newFakeEnv(t)
			env.set(b)

			r := runTool(context.Background(), env, repo, f.target, base)

			requireStopKind(t, r, StopTestFailed)
			if len(r.Reasons) != 1 || r.Reasons[0].Point != PointHead || r.Reasons[0].Test != f.failedTest() {
				t.Errorf("reasons = %+v, want one at HEAD naming %q", r.Reasons, f.failedTest())
			}
			if suites := suiteStarts(env.starts()); len(suites) != 2 {
				t.Errorf("suite starts = %d, want BASE's and HEAD's", len(suites))
			}
		})
	}
}

// repoState is the user's state the run must leave as it found it: HEAD,
// the current branch (none when HEAD is detached), git status, and the
// worktree list.
func repoState(r *testRepo) string {
	r.t.Helper()
	return strings.Join([]string{
		r.rev("HEAD"),
		r.git("branch", "--show-current"),
		r.git(statusArgs...),
		r.git("worktree", "list", "--porcelain"),
	}, "\x00")
}

// uncheckoutableBase gives repo a new BASE whose tree holds a ".git" file,
// which git refuses to check out, and a HEAD on top of it with the old HEAD's
// tree, and returns the new BASE. The user's worktree stays clean: it holds
// HEAD's tree.
func uncheckoutableBase(r *testRepo, base string) string {
	r.t.Helper()
	blob := strings.TrimSpace(r.gitInput("x\n", "hash-object", "-w", "--stdin"))
	tree := strings.TrimSpace(r.gitInput(r.git("ls-tree", base)+"100644 blob "+blob+"\t.git\n", "mktree"))
	badBase := strings.TrimSpace(r.git("commit-tree", tree, "-p", base, "-m", "uncheckoutable base"))
	head := strings.TrimSpace(r.git("commit-tree", r.rev("HEAD^{tree}"), "-p", badBase, "-m", "head"))
	r.git("reset", "-q", "--hard", head)
	return badBase
}

// TestRunCleansUp checks that every way a run ends leaves TMPDIR, which holds
// the work directory, empty and the user's state unchanged: a pass (also
// from a detached HEAD, which has no current branch), a failure, a test
// failure (also one that wrote no report, as a build failure does), an
// unreadable report, a failed range helper, a
// BASE checkout that cannot be made, a context canceled
// before the run, and one canceled while a suite runs. A cancellation is an
// internal stop even though the suite then exits unsuccessfully, and it
// leaves no process of the suite behind, not even one the suite started.
func TestRunCleansUp(t *testing.T) {
	t.Parallel()
	type cleanCase struct {
		name   string
		ext    bool // the case applies to the extension only
		modify func(*fakeBehavior, fixture)
		// prepare changes the repository and returns the BASE to give.
		prepare  func(*testRepo, string) string
		canceled bool
		running  bool // cancel once the BASE suite is running
		outcome  Outcome
		stop     StopKind
		// detail is the first reason's detail for the BASE given, when set.
		// Detail is compared as text because an internal stop has no other
		// observable that tells its causes apart.
		detail func(base string) string
	}
	noReport := func(point string) func(*fakeBehavior, fixture) {
		return func(b *fakeBehavior, f fixture) {
			s := f.failing
			s.NoReport = true
			b.Suites[point] = s
		}
	}
	cases := []cleanCase{
		{name: "pass", outcome: OutcomePass},
		{name: "pass from a detached HEAD", outcome: OutcomePass, prepare: func(r *testRepo, base string) string {
			r.git("checkout", "-q", "--detach")
			return base
		}},
		{name: "fail", outcome: OutcomeFail, modify: func(b *fakeBehavior, f fixture) { b.Suites["head"] = f.lost }},
		{name: "test failure", stop: StopTestFailed, modify: func(b *fakeBehavior, f fixture) { b.Suites["head"] = f.failing }},
		{name: "BASE failure without a report", stop: StopTestFailed, modify: noReport("base")},
		{name: "HEAD failure without a report", stop: StopTestFailed, modify: noReport("head")},
		{name: "unreadable report", stop: StopUnreadable, modify: func(b *fakeBehavior, f fixture) { b.Suites["head"] = f.unreadable }},
		{name: "range helper exits unsuccessfully", ext: true, stop: StopInternal, modify: func(b *fakeBehavior, _ fixture) {
			b.Ranges["head"] = fakeAnswer{Exit: 1}
		}},
		{name: "range helper writes to standard error", ext: true, stop: StopInternal, modify: func(b *fakeBehavior, _ fixture) {
			b.Ranges["head"] = fakeAnswer{Stdout: b.Ranges["head"].Stdout, Stderr: "warning\n"}
		}},
		{name: "checkout cannot be made", stop: StopInternal, prepare: uncheckoutableBase, detail: func(base string) string { return "checking out " + base + " failed" }},
		{name: "canceled before the run", stop: StopInternal, canceled: true, detail: interrupted},
		{name: "canceled while a suite runs", stop: StopInternal, running: true, detail: interrupted},
	}
	for _, f := range fixtures {
		for _, tc := range cases {
			if tc.ext && f.target != TargetExt {
				continue
			}
			t.Run(f.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				repo, base := f.repo(t)
				if tc.prepare != nil {
					base = tc.prepare(repo, base)
				}
				b := f.behavior()
				if tc.modify != nil {
					tc.modify(&b, f)
				}
				ready := filepath.Join(resolvedTempDir(t), "ready")
				if tc.running {
					s := b.Suites["base"]
					s.Ready = ready
					b.Suites["base"] = s
				}
				env := newFakeEnv(t)
				env.set(b)
				before := repoState(repo)

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if tc.canceled {
					cancel()
				}
				done := make(chan Result, 1)
				go func() { done <- runTool(ctx, env, repo, f.target, base) }()
				var suite []int
				if tc.running {
					suite = readPIDs(t, ready)
					cancel()
				}
				r := <-done

				if tc.outcome == OutcomeStop {
					requireStopKind(t, r, tc.stop)
				} else if r.Outcome != tc.outcome {
					t.Fatalf("outcome = %v (stop %v, reasons %+v), want %v", r.Outcome, r.Stop, r.Reasons, tc.outcome)
				}
				if tc.detail != nil {
					detail := tc.detail(base)
					if len(r.Reasons) == 0 || r.Reasons[0].Detail != detail {
						t.Errorf("reasons = %+v, want the first to be %q", r.Reasons, detail)
					}
				}
				env.requireTmpEmpty()
				if after := repoState(repo); after != before {
					t.Errorf("user state after the run =\n%q\nwant\n%q", after, before)
				}
				for _, pid := range suite {
					if !processGone(pid) {
						t.Errorf("process %d of the suite is still running", pid)
					}
				}
			})
		}
	}
}

// interrupted is the detail of a canceled run's reason.
func interrupted(string) string { return "the run was interrupted" }

// waitBound bounds how long a test waits for a fake's file or for a process
// group to be gone.
const waitBound = 10 * time.Second

// readPIDs waits for path to hold the suite's and its grandchild's PIDs and
// returns them.
func readPIDs(t *testing.T, path string) []int {
	t.Helper()
	deadline := time.Now().Add(waitBound)
	for {
		if data, err := os.ReadFile(path); err == nil {
			var pids []int
			for field := range strings.FieldsSeq(string(data)) {
				if pid, err := strconv.Atoi(field); err == nil && pid > 0 {
					pids = append(pids, pid)
				}
			}
			if len(pids) == 2 {
				for _, pid := range pids {
					t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
				}
				return pids
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no PIDs in %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// processGone reports whether the process is gone within waitBound. A
// process that has exited but is not yet reaped still exists for kill(2), so
// the wait gives its new parent time to reap it.
func processGone(pid int) bool {
	deadline := time.Now().Add(waitBound)
	for {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRunDetectsUserStateChange checks that a run whose suite changes the
// user's repository stops as internal and names what changed: one row per
// item the run compares, and one for the installed dependencies changing
// between the BASE and the HEAD suite.
func TestRunDetectsUserStateChange(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		target Target
		suite  func(repo *testRepo, other string) fakeSuite
		detail string
	}{
		{
			name: "branch", target: TargetGo,
			suite: func(r *testRepo, _ string) fakeSuite {
				return fakeSuite{Git: [][]string{{"switch", "-q", "-c", "moved"}}, GitDir: r.dir}
			},
			detail: "the user's " + stateBranch + " changed during the run",
		},
		{
			name: "HEAD", target: TargetGo,
			suite: func(r *testRepo, _ string) fakeSuite {
				return fakeSuite{Git: [][]string{{"commit", "-q", "--allow-empty", "-m", "moved"}}, GitDir: r.dir}
			},
			detail: "the user's " + stateHead + " changed during the run",
		},
		{
			name: "tracked file", target: TargetGo,
			suite: func(r *testRepo, _ string) fakeSuite {
				return fakeSuite{Writes: map[string]string{filepath.Join(r.dir, judgeGoFile): "changed\n"}}
			},
			detail: "the user's " + stateStatus + " changed during the run",
		},
		{
			name: "worktree", target: TargetGo,
			suite: func(r *testRepo, other string) fakeSuite {
				return fakeSuite{Git: [][]string{{"worktree", "add", "-q", "--detach", other}}, GitDir: r.dir}
			},
			detail: "the user's " + stateWorktrees + " changed during the run",
		},
		{
			name: "installed dependencies", target: TargetExt,
			suite: func(r *testRepo, _ string) fakeSuite {
				return fakeSuite{Writes: map[string]string{filepath.Join(r.dir, "extension", "node_modules", installedLockName): "changed\n"}}
			},
			detail: "extension/node_modules changed during the run",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := fixtures[0]
			if tc.target == TargetExt {
				f = fixtures[1]
			}
			repo, base := f.repo(t)
			b := f.behavior()
			change := tc.suite(repo, filepath.Join(resolvedTempDir(t), "other"))
			s := b.Suites["base"]
			s.Git, s.GitDir, s.Writes = change.Git, change.GitDir, change.Writes
			b.Suites["base"] = s
			env := newFakeEnv(t)
			env.set(b)

			r := runTool(context.Background(), env, repo, f.target, base)

			requireStopKind(t, r, StopInternal)
			// The detail is the observable that names the changed item: an
			// internal stop has no other.
			if len(r.Reasons) != 1 || r.Reasons[0].Detail != tc.detail {
				t.Errorf("reasons = %+v, want one: %q", r.Reasons, tc.detail)
			}
			env.requireTmpEmpty()
		})
	}
}

// TestDiffIgnoresUserGitConfig checks that the user's git configuration
// cannot change the diff the run reads. HEAD deletes two functions two lines
// apart, so diff.interHunkContext would merge the two hunks into one with
// context lines, and diff.noprefix would drop the a/ and b/ prefixes; with
// those settings, an external diff, and a textconv in the repository, the run
// reads the diff as without them, and neither external command runs.
// (diff.relative and diff.algorithm are overridden too, but no input here
// tells them apart: the run reads the diff from the repository root, and a
// pure deletion is placed by the tool whatever position git gives it.) The global
// configuration is not read at all: with the same settings there, and an
// attributes file there that marks Go files binary, the run still passes,
// and so does it with the global attributes file in its default place, which
// git reads without any configuration, marking Go files not diffable.
// A renamed file is a deletion and an addition, so its functions stop the
// run as unmatched rather than the rename lines making the diff unreadable.
func TestDiffIgnoresUserGitConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		global bool // write the settings to HOME's .gitconfig
		// attributes writes HOME's default global attributes file, which git
		// reads even without the global configuration, marking Go files
		// as not diffable.
		attributes bool
		rename     bool
	}{
		{name: "repository configuration"},
		{name: "global configuration", global: true},
		{name: "global attributes", attributes: true},
		{name: "rename detection", rename: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := resolvedTempDir(t)
			marker := filepath.Join(dir, "ran")
			script := filepath.Join(dir, "touch.sh")
			if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch "+marker+"\ncat \"$1\"\n"), 0o700); err != nil {
				t.Fatalf("write the script: %v", err)
			}
			settings := [][2]string{
				{"diff.noprefix", "true"},
				{"diff.interHunkContext", "3"},
				{"diff.renames", "true"},
				{"diff.external", script},
				{"diff.conv.textconv", script},
			}
			repo, base, b, deleted := twoDeletionsRepo(t)
			if tc.rename {
				repo, base = goRepo(t)
				b = goBehavior()
				repo.git("rm", "-q", judgeGoFile)
				repo.write("y/y.go", goHeadText)
				repo.commitAll("rename")
				b.Suites["head"] = fakeSuite{Stdout: goJSONPass(1), Report: strings.ReplaceAll(b.Suites["head"].Report, judgeGoFile, "y/y.go")}
			}
			env := newFakeEnv(t)
			var global strings.Builder
			if tc.global {
				// Only a global setting the diff's options do not override
				// shows whether the global configuration is read: an
				// attributes file that marks every Go file binary, which
				// would make the diff unreadable.
				attributes := filepath.Join(dir, "attributes")
				if err := os.WriteFile(attributes, []byte("*.go diff=bin\n"), 0o600); err != nil {
					t.Fatalf("write the attributes: %v", err)
				}
				fmt.Fprintf(&global, "[core]\n\tattributesFile = %s\n[diff \"bin\"]\n\tbinary = true\n", attributes)
			} else if !tc.attributes {
				// The repository's attributes outrank the global file, so
				// the attributes row writes none.
				repo.writeGitDir("info/attributes", "*.go diff=conv\n")
			}
			for _, kv := range settings {
				if tc.global {
					section, key, _ := strings.Cut(kv[0], ".")
					if sub, name, ok := strings.Cut(key, "."); ok {
						fmt.Fprintf(&global, "[%s %q]\n\t%s = %s\n", section, sub, name, kv[1])
					} else {
						fmt.Fprintf(&global, "[%s]\n\t%s = %s\n", section, key, kv[1])
					}
				} else {
					repo.git("config", kv[0], kv[1])
				}
			}
			if tc.attributes {
				attributes := filepath.Join(env.home, ".config", "git", "attributes")
				if err := os.MkdirAll(filepath.Dir(attributes), 0o700); err != nil {
					t.Fatalf("create %s: %v", filepath.Dir(attributes), err)
				}
				if err := os.WriteFile(attributes, []byte("*.go -diff\n"), 0o600); err != nil {
					t.Fatalf("write the attributes: %v", err)
				}
			}
			if err := os.WriteFile(filepath.Join(env.home, ".gitconfig"), []byte(global.String()), 0o600); err != nil {
				t.Fatalf("write the global configuration: %v", err)
			}
			env.set(b)

			r := runTool(context.Background(), env, repo, TargetGo, base)

			if tc.rename {
				requireStopKind(t, r, StopUnmatched)
			} else if r.Outcome != OutcomePass || !reflect.DeepEqual(r.Deleted, deleted) {
				t.Errorf("result = outcome %v deleted %+v (stop %v, reasons %+v), want a pass deleting %+v",
					r.Outcome, r.Deleted, r.Stop, r.Reasons, deleted)
			}
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("an external diff or textconv ran (stat %s: %v)", marker, err)
			}
		})
	}
}

// twoDeletionsRepo creates a Go repository whose HEAD deletes B and D from
// x/x.go, with only the one-line C between them, and returns it with BASE,
// the fakes of a passing run, and the deleted functions.
func twoDeletionsRepo(t *testing.T) (*testRepo, string, fakeBehavior, []DeletedFunc) {
	t.Helper()
	r := newTestRepo(t)
	r.write("go.mod", "module "+judgeModule+"\n\ngo 1.21\n")
	r.write(judgeGoFile, joinLines(
		"package x", "",
		"func A() int {", "\treturn 1", "}", "",
		"func B() int {", "\treturn 2", "}", "",
		"func C() int { return 3 }", "",
		"func D() int {", "\treturn 4", "}",
	))
	base := r.commitAll("base")
	r.write(judgeGoFile, joinLines(
		"package x", "",
		"func A() int {", "\treturn 1", "}", "",
		"func C() int { return 3 }",
	))
	r.commitAll("delete B and D")
	b := fakeBehavior{
		GoVersion: fakeGoVersion,
		Suites: map[string]fakeSuite{
			"base": {Stdout: goJSONPass(1), Report: goProfileText("3.14,5.2 1 1", "7.14,9.2 1 0", "11.14,11.25 1 1", "13.14,15.2 1 0")},
			"head": {Stdout: goJSONPass(1), Report: goProfileText("3.14,5.2 1 1", "7.14,7.25 1 1")},
		},
	}
	deleted := []DeletedFunc{
		{ID: FuncID{File: judgeGoFile, Line: 7}, Names: []string{"B"}, State: NotRunAtBase},
		{ID: FuncID{File: judgeGoFile, Line: 13}, Names: []string{"D"}, State: NotRunAtBase},
	}
	return r, base, b, deleted
}

// TestRunAcceptsLargeOutput checks that the run has no cap of its own on a
// suite's output: a -json output of over 20 MiB with lines over 1 MiB is read
// to a verdict.
func TestRunAcceptsLargeOutput(t *testing.T) {
	t.Parallel()
	repo, base := goRepo(t)
	b := goBehavior()
	for point, s := range b.Suites {
		s.LargeStdout = true
		b.Suites[point] = s
	}
	env := newFakeEnv(t)
	env.set(b)

	r := runTool(context.Background(), env, repo, TargetGo, base)

	if r.Outcome != OutcomePass || r.Base.Tests != 2 {
		t.Errorf("result = outcome %v tests %d (stop %v, reasons %+v), want a pass with 2 BASE tests", r.Outcome, r.Base.Tests, r.Stop, r.Reasons)
	}
}

// TestExtTestGlobMatchesPackageJSON checks that the tool runs the same test
// files as the test script of extension/package.json, which is
// node --test with the glob as its one quoted argument.
func TestExtTestGlobMatchesPackageJSON(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "extension", "package.json"))
	if err != nil {
		t.Fatalf("read extension/package.json: %v", err)
	}
	object, err := strictjson.ParseObject(data)
	if err != nil {
		t.Fatalf("parse extension/package.json: %v", err)
	}
	members, err := object.Collect("scripts")
	if err != nil {
		t.Fatalf("read scripts: %v", err)
	}
	value, err := strictjson.Required(members, "scripts")
	if err != nil {
		t.Fatalf("read scripts: %v", err)
	}
	scripts, err := value.AsObject()
	if err != nil {
		t.Fatalf("read scripts: %v", err)
	}
	members, err = scripts.Collect("test")
	if err != nil {
		t.Fatalf("read the test script: %v", err)
	}
	script, err := strictjson.RequiredString(members, "test")
	if err != nil {
		t.Fatalf("read the test script: %v", err)
	}
	quoted, ok := strings.CutPrefix(script, "node --test ")
	if !ok {
		t.Fatalf("test script = %q, want node --test <glob>", script)
	}
	glob, err := strconv.Unquote(quoted)
	if err != nil {
		t.Fatalf("test script = %q, want one quoted glob after node --test: %v", script, err)
	}
	if glob != extTestGlob {
		t.Errorf("package.json tests %q, the tool tests %q", glob, extTestGlob)
	}
}
