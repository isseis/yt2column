//go:build test

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// adrCandidatesPath is the work tree's copy of the candidates script. The
// tests run it directly to check its behavior; that /fixpr runs only
// origin/main's copy is TestADRCorrectionCandidatesCallSite's concern.
const adrCandidatesPath = "../../.claude/commands/_lib/adr-correction-candidates.sh"

// ADR paths of the fixture's main. Each case changes the ones it needs, so
// only the property it checks keeps a path off the list.
const (
	acAlpha   = "docs/dev/adr/0001-alpha.md"
	acBeta    = "docs/dev/adr/0002-beta.md"
	acGamma   = "docs/dev/adr/0003-gamma.md"
	acDelta   = "docs/dev/adr/0004-delta.md"
	acEpsilon = "docs/dev/adr/0005-epsilon.md"
	acZeta    = "docs/dev/adr/0006-zeta.md"
	acIndex   = "docs/dev/adr/README.md"
	acTaskDoc = "docs/tasks/0001_task/01_requirements.md"
	// acOwnADR is created on the branch, so it is not on origin/main.
	acOwnADR = "docs/dev/adr/0009-own.md"
)

// acOffForm are files on main whose names do not have the ADR file form.
// Their stems share nothing with the other fixture files, since a
// case-insensitive file system (macOS) would otherwise fold two of them into
// one.
var acOffForm = []string{
	"docs/dev/adr/0101--double.md",
	"docs/dev/adr/0102-trailing-.md",
	"docs/dev/adr/0103-Upper.md",
	"docs/dev/adr/104-short.md",
	"docs/dev/adr/01050-long.md",
	"docs/dev/adr/0106-text.txt",
	"docs/dev/adr/nested/0107-nested.md",
	"docs/dev/adr/0108-caf\u00e9.md",
}

// acFixture is one fixture repository: main holds the ADRs and the other
// files, refs/remotes/origin/main points at it, and the branch forks from it.
// home and tmp are the case's own empty HOME and TMPDIR (tmp is checked empty
// after each run); outside holds the argument files, out of the repository.
type acFixture struct {
	t                        *testing.T
	repo, home, tmp, outside string
	main                     string
}

func newACFixture(t *testing.T) *acFixture {
	t.Helper()
	f := &acFixture{t: t, home: t.TempDir(), tmp: t.TempDir(), outside: t.TempDir()}
	f.repo = filepath.Join(t.TempDir(), "repo")
	f.gitIn(filepath.Dir(f.repo), "init", "-q", "-b", "main", f.repo)
	for _, path := range slices.Concat([]string{acAlpha, acBeta, acGamma, acDelta, acEpsilon, acZeta, acIndex, acTaskDoc}, acOffForm) {
		f.write(path, "main "+path+"\n")
	}
	f.commit("main")
	f.main = f.rev("HEAD")
	f.git("update-ref", "refs/remotes/origin/main", f.main)
	f.git("switch", "-q", "-c", "feature")
	return f
}

// env is the allowlisted environment of every git and script run, so no
// variable of the test's own environment (a GIT_DIR a pre-commit hook
// exports) reaches them.
func (f *acFixture) env() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + f.home, "TMPDIR=" + f.tmp,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
	}
}

// gitIn runs git in dir with the fixture's environment and a fixed author,
// failing t on an error, and returns its standard output.
func (f *acFixture) gitIn(dir string, args ...string) string {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(f.env(),
		"GIT_AUTHOR_NAME=Octo User", "GIT_AUTHOR_EMAIL=octo@example.invalid",
		"GIT_COMMITTER_NAME=Octo User", "GIT_COMMITTER_EMAIL=octo@example.invalid")
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("git %v: %v: %s", args, err, stderr.String())
	}
	return stdout.String()
}

func (f *acFixture) git(args ...string) string {
	f.t.Helper()
	return f.gitIn(f.repo, args...)
}

func (f *acFixture) rev(name string) string {
	f.t.Helper()
	return strings.TrimSpace(f.git("rev-parse", "--verify", name))
}

// write writes content to path in the work tree, creating its directory.
func (f *acFixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.repo, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		f.t.Fatalf("create the directory of %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

// change appends a line to each path in the work tree, a content change git
// reports as M.
func (f *acFixture) change(paths ...string) {
	f.t.Helper()
	for _, path := range paths {
		f.write(path, "main "+path+"\nchanged\n")
	}
}

// commit stages every change and commits it with message.
func (f *acFixture) commit(message string) {
	f.t.Helper()
	f.git("add", "-A")
	f.git("commit", "-q", "--allow-empty", "-m", message)
}

// acArg is one argument file: its content, or no file at all when missing.
type acArg struct {
	content string
	missing bool
}

// acRunOptions changes how run runs the script: in dir (relative to the
// repository; its root when ""), with stdin, and with extraEnv after the
// fixture's environment (a later entry wins).
type acRunOptions struct {
	dir, stdin string
	extraEnv   []string
}

// acRun is what one run printed and returned.
type acRun struct {
	code   int
	stdout string
	stderr string
}

func (r acRun) String() string {
	return fmt.Sprintf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
}

// run writes each argument to its own file outside the repository, as /fixpr
// writes each path, runs the script with bash, and checks that the run left
// TMPDIR empty.
func (f *acFixture) run(bash scriptBash, args []acArg, opts acRunOptions) acRun {
	f.t.Helper()
	files := make([]string, len(args))
	for i, arg := range args {
		files[i] = filepath.Join(f.outside, fmt.Sprintf("path-%d", i))
		if arg.missing {
			continue
		}
		if err := os.WriteFile(files[i], []byte(arg.content), 0o600); err != nil {
			f.t.Fatalf("write an argument file: %v", err)
		}
	}
	script, err := filepath.Abs(adrCandidatesPath)
	if err != nil {
		f.t.Fatalf("resolve the script path: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash.path, append([]string{script}, files...)...) //nolint:gosec // bash with the test's script
	cmd.Dir = filepath.Join(f.repo, filepath.FromSlash(opts.dir))
	cmd.Env = append(f.env(), opts.extraEnv...)
	cmd.Stdin = strings.NewReader(opts.stdin)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	run := acRun{stdout: stdout.String(), stderr: stderr.String()}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		run.code = exitErr.ExitCode()
	} else if err != nil {
		f.t.Fatalf("run the script: %v", err)
	}
	if entries, err := os.ReadDir(f.tmp); err != nil || len(entries) != 0 {
		f.t.Errorf("the script left TMPDIR with %d entries (error %v)", len(entries), err)
	}
	return run
}

// header is the first line of a run with arguments.
func (f *acFixture) header() string {
	return "origin-main " + f.main
}

// acExact is an argument file holding path exactly.
func acExact(path string) acArg { return acArg{content: path} }

// acCase is one fixture case: what it does to the branch, the arguments, the
// options, the environment it adds, and the output it expects (nil when the
// run must fail with a status other than 0).
type acCase struct {
	name  string
	setup func(f *acFixture)
	args  []acArg
	opts  acRunOptions
	env   func(f *acFixture) []string
	want  func(f *acFixture) []string
}

// acLines returns want as a case expectation that ignores the fixture.
func acLines(want ...string) func(*acFixture) []string {
	return func(*acFixture) []string { return want }
}

func adrCandidatesCases() []acCase {
	changeAlpha := func(f *acFixture) { f.change(acAlpha) }
	ownADR := func(f *acFixture) {
		f.write(acOwnADR, "own\n")
		f.commit("add an ADR of the branch")
		f.change(acOwnADR)
	}
	return []acCase{
		// The list.
		{name: "content change", setup: changeAlpha, want: acLines(acAlpha)},
		{name: "nothing changed", setup: func(*acFixture) {}, want: acLines()},
		{name: "subdirectory", setup: changeAlpha, opts: acRunOptions{dir: "docs/dev"}, want: acLines(acAlpha)},
		// The change check.
		{name: "deleted", setup: func(f *acFixture) {
			if err := os.Remove(filepath.Join(f.repo, acDelta)); err != nil {
				f.t.Fatal(err)
			}
		}, want: acLines()},
		{name: "replaced by a symbolic link", setup: func(f *acFixture) {
			full := filepath.Join(f.repo, acEpsilon)
			if err := os.Remove(full); err != nil {
				f.t.Fatal(err)
			}
			if err := os.Symlink("0001-alpha.md", full); err != nil {
				f.t.Fatal(err)
			}
		}, want: acLines()},
		// git never reports a rename as M, with rename detection on or off,
		// so this case cannot tell that the script turns detection off.
		{name: "renamed", setup: func(f *acFixture) {
			f.git("mv", acZeta, "docs/dev/adr/0006-zeta-renamed.md")
		}, want: acLines()},
		// The form check.
		{name: "index", setup: func(f *acFixture) { f.change(acIndex) }, want: acLines()},
		{name: "task document", setup: func(f *acFixture) { f.change(acTaskDoc) }, want: acLines()},
		{name: "names off the form", setup: func(f *acFixture) { f.change(acOffForm...) }, want: acLines()},
		// The main check.
		{name: "ADR of the branch", setup: ownADR, want: acLines()},
		{name: "local branch named origin/main", setup: func(f *acFixture) {
			ownADR(f)
			f.git("branch", "origin/main", "HEAD")
		}, want: acLines()},
		// The comparison.
		{name: "exact path", setup: changeAlpha, args: []acArg{acExact(acAlpha)}, want: func(f *acFixture) []string {
			return []string{f.header(), "match " + acAlpha}
		}},
		{name: "near misses", setup: changeAlpha, args: []acArg{
			acExact("docs/dev/adr/0001-\u0430lpha.md"), // Cyrillic a
			acExact(acAlpha + "\n"),
			acExact("./" + acAlpha),
			acExact(acAlpha + "\n" + acBeta),
			acExact(acAlpha + "\x00"),
			acExact("docs/dev/adr/../adr/0001-alpha.md"),
			acExact("docs/dev/adr/0001-ALPHA.md"),
			acExact(acAlpha + "/"),
			acExact(acIndex),
			acExact(""),
			{missing: true},
		}, want: func(f *acFixture) []string {
			return append([]string{f.header()}, slices.Repeat([]string{"no-match"}, 11)...)
		}},
		{name: "argument order", setup: func(f *acFixture) { f.change(acAlpha, acGamma) }, args: []acArg{
			acExact(acGamma), acExact(acBeta), acExact(acAlpha),
		}, want: func(f *acFixture) []string {
			return []string{f.header(), "match " + acGamma, "no-match", "match " + acAlpha}
		}},
		{name: "empty list", setup: func(*acFixture) {}, args: []acArg{acExact(acAlpha)}, want: func(f *acFixture) []string {
			return []string{f.header(), "no-match"}
		}},
		// The reference check and failures.
		{name: "no origin/main, ADR changed", setup: func(f *acFixture) {
			f.git("update-ref", "-d", "refs/remotes/origin/main")
			f.change(acAlpha)
		}},
		{name: "no origin/main, nothing changed", setup: func(f *acFixture) {
			f.git("update-ref", "-d", "refs/remotes/origin/main")
		}},
		{name: "origin/main names a blob", setup: func(f *acFixture) {
			f.git("update-ref", "refs/remotes/origin/main", f.rev("HEAD:"+acAlpha))
			f.change(acAlpha)
		}},
		{name: "HEAD unreadable", setup: func(f *acFixture) {
			f.git("symbolic-ref", "HEAD", "refs/heads/no-such-branch")
			f.change(acAlpha)
		}},
		// The inputs and the environment.
		// GIT_DIR and GIT_WORK_TREE name a second repository whose only
		// change is to acBeta, so a script that honored them would list it.
		{
			name: "standard input and GIT_ variables", setup: changeAlpha,
			opts: acRunOptions{stdin: acBeta + "\n"},
			env: func(f *acFixture) []string {
				other := newACFixture(f.t)
				other.change(acBeta)
				return []string{
					"GIT_DIR=" + filepath.Join(other.repo, ".git"), "GIT_WORK_TREE=" + other.repo,
					"GIT_CONFIG_PARAMETERS='diff.relative'='true'",
				}
			}, want: acLines(acAlpha),
		},
		// Under a UTF-8 locale a range such as [a-z] may take in a letter
		// with a diacritic, so acOffForm's 0108-caf\u00e9.md would match.
		{name: "locale", setup: func(f *acFixture) {
			f.change(acAlpha)
			f.change(acOffForm...)
		}, env: func(*acFixture) []string {
			return []string{"LC_ALL=en_US.UTF-8", "LANG=en_US.UTF-8"}
		}, want: acLines(acAlpha)},
		// A printf function would replace every line the script prints. The
		// script runs again under env -i, so neither the function BASH_ENV
		// defines nor the exported one reaches it.
		{
			name: "shell functions, BASH_ENV, and SHELLOPTS", setup: changeAlpha, args: []acArg{acExact(acAlpha)},
			env: func(f *acFixture) []string {
				bashEnv := filepath.Join(f.outside, "bash-env")
				fake := "printf() { builtin printf 'fake\\n'; }\n"
				if err := os.WriteFile(bashEnv, []byte(fake), 0o600); err != nil {
					f.t.Fatalf("write BASH_ENV: %v", err)
				}
				return []string{
					"BASH_ENV=" + bashEnv,
					"BASH_FUNC_cmp%%=() { return 0; }",
					"BASH_FUNC_printf%%=() { builtin printf 'fake\\n'; }",
					"SHELLOPTS=errexit:noglob",
				}
			}, want: func(f *acFixture) []string {
				return []string{f.header(), "match " + acAlpha}
			},
		},
	}
}

// acConfigScenario is a setting a user could have that changes what plain git
// prints for the scenario's change: the change, the directory, the setting,
// and the plain git command whose output the setting changes.
type acConfigScenario struct {
	name   string
	setup  func(f *acFixture)
	dir    string
	config [2]string
}

func acConfigScenarios() []acConfigScenario {
	return []acConfigScenario{
		{
			// Relative to docs/dev, plain git names the file
			// adr/0001-alpha.md, which has no ADR file form.
			name: "diff.relative", dir: "docs/dev",
			setup:  func(f *acFixture) { f.change(acAlpha) },
			config: [2]string{"diff.relative", "true"},
		},
		{
			// With autocrlf, plain git sees a change of line endings alone
			// as no change.
			name:   "core.autocrlf",
			setup:  func(f *acFixture) { f.write(acBeta, "main "+acBeta+"\r\n") },
			config: [2]string{"core.autocrlf", "true"},
		},
	}
}

// TestADRCorrectionCandidates runs the candidates script /fixpr runs on
// fixture repositories: the list, the comparison of argument files with it,
// the failures, and that the caller's input, environment, and git
// configuration change nothing. Each case changes only what the property it
// checks keeps off the list, and runs with each bash scriptBashes returns.
//
// Some guards of the script back each other, so breaking one alone fails no
// case: the reference check (git ls-tree fails on a missing reference too),
// turning off diff.relative (the script runs at the top of the work tree,
// where a relative diff names the same paths), the anchor of the form (git
// ls-tree lists only docs/dev/adr/), the explicit character lists of the form
// (the script also runs under LC_ALL=C), and rename detection (git never
// reports a rename as M).
func TestADRCorrectionCandidates(t *testing.T) {
	for _, tool := range []string{"git", "cmp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("%s is not on PATH: %v", tool, err)
		}
	}
	for _, bash := range scriptBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			for _, c := range adrCandidatesCases() {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					f := newACFixture(t)
					c.setup(f)
					opts := c.opts
					if c.env != nil {
						opts.extraEnv = c.env(f)
					}
					run := f.run(bash, c.args, opts)
					if c.want == nil {
						if run.code == 0 {
							t.Errorf("the run succeeded, want a failure: %v", run)
						}
						return
					}
					want := c.want(f)
					got := slices.Collect(strings.Lines(run.stdout))
					for i := range got {
						got[i] = strings.TrimSuffix(got[i], "\n")
					}
					if run.code != 0 || !slices.Equal(got, want) || (len(want) > 0) != strings.HasSuffix(run.stdout, "\n") {
						t.Errorf("got %v, want lines %q and exit 0", run, want)
					}
				})
			}
			for _, scope := range []string{"repository", "global"} {
				for _, s := range acConfigScenarios() {
					t.Run("git config/"+scope+"/"+s.name, func(t *testing.T) {
						t.Parallel()
						checkACIgnoresGitConfig(t, bash, scope, s)
					})
				}
			}
		})
	}
}

// checkACIgnoresGitConfig sets s's setting in scope, after checking that it
// changes plain git's output for s's change; the script must print the same
// as without it.
func checkACIgnoresGitConfig(t *testing.T, bash scriptBash, scope string, s acConfigScenario) {
	t.Helper()
	f := newACFixture(t)
	s.setup(f)
	dir := filepath.Join(f.repo, filepath.FromSlash(s.dir))
	plain := func() string { return f.gitIn(dir, "diff", "--name-only", "HEAD") }
	before := plain()
	opts := acRunOptions{dir: s.dir}
	want := f.run(bash, nil, opts)
	if want.code != 0 || want.stdout == "" {
		t.Fatalf("without %s the script printed nothing or failed, so the scenario checks nothing: %v", s.name, want)
	}
	file := filepath.Join(f.repo, ".git", "config")
	if scope == "global" {
		file = filepath.Join(f.home, ".gitconfig")
	}
	f.git("config", "--file", file, s.config[0], s.config[1])
	if after := plain(); after == before {
		t.Fatalf("%s does not change plain git's output %q, so the scenario checks nothing", s.name, before)
	}
	if got := f.run(bash, nil, opts); got != want {
		t.Errorf("with %s set the script printed %v, without it %v", s.name, got, want)
	}
}
