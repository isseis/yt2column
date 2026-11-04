//go:build test

package coveragecompare

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunPreconditions checks each check before the suites run: the run
// stops with the right kind, has started no suite, and has run npm only as
// npm --version. Three rows meet the conditions of two neighboring checks at
// once and stop at the earlier one, in the order usage, not-ancestor, dirty,
// toolchain. A file git ignores is not an uncommitted change, so it alone
// lets the run pass. The row whose BASE starts with "-" checks the stop kind
// only: the ^{commit} suffix already keeps such a BASE from matching an
// option, so the row does not depend on --end-of-options.
func TestRunPreconditions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		target Target
		// prepare changes the repository and the fakes and returns the
		// BASE argument; base is the BASE goRepo or extRepo made.
		prepare func(r *testRepo, b *fakeBehavior, base string) string
		pass    bool
		stop    StopKind
		// named rows check the point and the file (empty for none) their
		// one reason names.
		named bool
		point Point
		file  string
	}{
		{name: "BASE is empty", stop: StopUsage, prepare: func(*testRepo, *fakeBehavior, string) string { return "" }},
		{name: "BASE does not resolve", stop: StopUsage, prepare: func(*testRepo, *fakeBehavior, string) string { return "nope" }},
		{name: "BASE names a tree", stop: StopUsage, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return r.rev("HEAD^{tree}")
		}},
		{name: "BASE starts with a dash", stop: StopUsage, prepare: func(*testRepo, *fakeBehavior, string) string { return "--help" }},
		{name: "BASE is HEAD", stop: StopNotAncestor, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return r.rev("HEAD")
		}},
		{name: "BASE is not an ancestor", stop: StopNotAncestor, prepare: func(r *testRepo, _ *fakeBehavior, base string) string {
			return strings.TrimSpace(r.git("commit-tree", r.rev("HEAD^{tree}"), "-p", base, "-m", "side"))
		}},
		{name: "a tracked file is modified", stop: StopDirty, prepare: func(r *testRepo, _ *fakeBehavior, base string) string {
			r.write(judgeGoFile, "changed\n")
			return base
		}},
		{name: "a change is staged", stop: StopDirty, prepare: func(r *testRepo, _ *fakeBehavior, base string) string {
			r.write(judgeGoFile, "changed\n")
			r.git("add", judgeGoFile)
			return base
		}},
		{name: "an untracked file exists", stop: StopDirty, prepare: func(r *testRepo, _ *fakeBehavior, base string) string {
			r.write("new.txt", "new\n")
			return base
		}},
		{name: "an ignored file alone is not a change", pass: true, prepare: func(r *testRepo, _ *fakeBehavior, base string) string {
			r.writeGitDir("info/exclude", "*.log\n")
			r.write("build.log", "log\n")
			return base
		}},
		{name: "BASE requires a newer go", stop: StopToolchain, named: true, point: PointBase, file: "go.mod", prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return newGoModCommits(r, "go 1.99\n", "go 1.21\n")
		}},
		{name: "HEAD requires a newer go", stop: StopToolchain, named: true, point: PointHead, file: "go.mod", prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return newGoModCommits(r, "go 1.21\n", "go 1.99\n")
		}},
		{name: "go.mod has no go line", stop: StopToolchain, named: true, point: PointHead, file: "go.mod", prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return newGoModCommits(r, "go 1.21\n", "")
		}},
		{name: "go.mod has two go lines", stop: StopToolchain, named: true, point: PointHead, file: "go.mod", prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return newGoModCommits(r, "go 1.21\n", "go 1.21\ngo 1.21\n")
		}},
		{name: "go.mod is absent at BASE", stop: StopToolchain, named: true, point: PointBase, file: "go.mod", prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			r.git("rm", "-q", "go.mod")
			base := r.commitAll("no go.mod")
			r.write("go.mod", "module "+judgeModule+"\n\ngo 1.21\n")
			r.commitAll("go.mod again")
			return base
		}},
		{name: "go env prints no version", stop: StopToolchain, named: true, prepare: func(_ *testRepo, b *fakeBehavior, base string) string {
			b.GoVersion = "devel go1.28-abcdef"
			return base
		}},
		{name: "node differs from .node-version", target: TargetExt, stop: StopToolchain, named: true, point: PointBase, file: nodeVersionFile, prepare: func(_ *testRepo, b *fakeBehavior, base string) string {
			b.NodeVersion = "v24.0.0"
			return base
		}},
		{name: "npm differs from packageManager", target: TargetExt, stop: StopToolchain, named: true, point: PointBase, file: packageJSONFile, prepare: func(_ *testRepo, b *fakeBehavior, base string) string {
			b.NpmVersion = "10.0.0"
			return base
		}},
		{name: ".node-version is empty at HEAD", target: TargetExt, stop: StopToolchain, named: true, point: PointHead, file: nodeVersionFile, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return newExtPinCommit(r, nodeVersionFile, "\n")
		}},
		{name: "packageManager is not npm", target: TargetExt, stop: StopToolchain, named: true, point: PointHead, file: packageJSONFile, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return newExtPinCommit(r, packageJSONFile, `{"packageManager":"pnpm@9.0.0"}`+"\n")
		}},
		{name: "packageManager names no npm version", target: TargetExt, stop: StopToolchain, named: true, point: PointHead, file: packageJSONFile, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			return newExtPinCommit(r, packageJSONFile, `{"packageManager":"npm@"}`+"\n")
		}},
		{name: "node_modules is missing", target: TargetExt, stop: StopToolchain, named: true, file: "extension/node_modules", prepare: func(r *testRepo, _ *fakeBehavior, base string) string {
			if err := os.RemoveAll(filepath.Join(r.dir, "extension", "node_modules")); err != nil {
				r.t.Fatalf("remove node_modules: %v", err)
			}
			return base
		}},
		{name: "the lockfiles differ", target: TargetExt, stop: StopToolchain, named: true, file: lockfileFile, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			base := r.rev("HEAD")
			r.write(lockfileFile, `{"changed":true}`+"\n")
			r.write(judgeExtFile, extHeadText+"\n")
			r.commitAll("change the lockfile")
			return base
		}},
		{name: "usage before dirty", stop: StopUsage, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			r.write("new.txt", "new\n")
			return ""
		}},
		{name: "not-ancestor before dirty", stop: StopNotAncestor, prepare: func(r *testRepo, _ *fakeBehavior, _ string) string {
			r.write("new.txt", "new\n")
			return r.rev("HEAD")
		}},
		{name: "dirty before toolchain", stop: StopDirty, prepare: func(r *testRepo, b *fakeBehavior, base string) string {
			r.write("new.txt", "new\n")
			b.GoVersion = "go1.20"
			return base
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := fixtures[0]
			if tc.target == TargetExt {
				f = fixtures[1]
			}
			repo, base := f.repo(t)
			b := f.behavior()
			base = tc.prepare(repo, &b, base)
			env := newFakeEnv(t)
			env.set(b)

			r := runTool(context.Background(), env, repo, f.target, base)

			if tc.pass {
				if r.Outcome != OutcomePass {
					t.Fatalf("outcome = %v (stop %v, reasons %+v), want a pass", r.Outcome, r.Stop, r.Reasons)
				}
				return
			}
			requireStopKind(t, r, tc.stop)
			if tc.named {
				file := ""
				if len(r.Reasons) == 1 && r.Reasons[0].Func != nil {
					file = r.Reasons[0].Func.File
				}
				if len(r.Reasons) != 1 || r.Reasons[0].Point != tc.point || file != tc.file {
					t.Errorf("reasons = %+v, want one naming file %q at %v", r.Reasons, tc.file, tc.point)
				}
			}
			starts := env.starts()
			if suites := suiteStarts(starts); len(suites) > 0 {
				t.Errorf("suite starts = %d, want none before the checks pass", len(suites))
			}
			requireNpmVersionOnly(t, starts)
			env.requireTmpEmpty()
		})
	}
}

// newExtPinCommit commits content to the pin file as a new HEAD, with the
// product file changed too, and returns the old HEAD as BASE.
func newExtPinCommit(r *testRepo, file, content string) string {
	r.t.Helper()
	base := r.rev("HEAD")
	r.write(file, content)
	r.write(judgeExtFile, extHeadText+"\n")
	r.commitAll("change " + file)
	return base
}

// newGoModCommits commits go.mod with the go lines baseLine and headLine
// (empty for none) as a new BASE and a new HEAD, keeping x/x.go, and returns
// the new BASE.
func newGoModCommits(r *testRepo, baseLine, headLine string) string {
	r.t.Helper()
	r.write("go.mod", "module "+judgeModule+"\n\n"+baseLine)
	base := r.commitAll("BASE go.mod")
	r.write("go.mod", "module "+judgeModule+"\n\n"+headLine)
	r.write(judgeGoFile, goHeadText+"\n")
	r.commitAll("HEAD go.mod")
	return base
}

// TestRunKeepsUserExcludesFile checks that a file the user's git ignores
// through core.excludesFile is not an uncommitted change, although the run
// reads no other setting of the global configuration. The global row names
// the file with ~, which only a read as a path expands; the repository row
// sets core.excludesFile in the repository's configuration too, which outranks
// the global one, as it does for the user's git. Each row first checks that
// the file is ignored only through the setting: without it, the run stops as
// dirty.
func TestRunKeepsUserExcludesFile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		// repository sets core.excludesFile in the repository too.
		repository bool
	}{
		{name: "global configuration"},
		{name: "repository configuration outranks the global one", repository: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, base := goRepo(t)
			b := goBehavior()
			repo.write("scratch.local", "scratch\n")
			env := newFakeEnv(t)
			env.set(b)

			requireStopKind(t, runTool(context.Background(), env, repo, TargetGo, base), StopDirty)

			global := filepath.Join(env.home, "global-ignore")
			// The global file ignores nothing in the repository row, so
			// only the repository's file can make the run pass.
			ignored := "scratch.local\n"
			if tc.repository {
				ignored = "other.local\n"
				local := filepath.Join(resolvedTempDir(t), "local-ignore")
				if err := os.WriteFile(local, []byte("scratch.local\n"), 0o600); err != nil {
					t.Fatalf("write the repository's excludes file: %v", err)
				}
				repo.git("config", "core.excludesFile", local)
			}
			if err := os.WriteFile(global, []byte(ignored), 0o600); err != nil {
				t.Fatalf("write the global excludes file: %v", err)
			}
			config := "[core]\n\texcludesFile = ~/global-ignore\n"
			if err := os.WriteFile(filepath.Join(env.home, ".gitconfig"), []byte(config), 0o600); err != nil {
				t.Fatalf("write the global configuration: %v", err)
			}

			r := runTool(context.Background(), env, repo, TargetGo, base)

			if r.Outcome != OutcomePass {
				t.Errorf("outcome = %v (stop %v, reasons %+v), want a pass", r.Outcome, r.Stop, r.Reasons)
			}
		})
	}
}
