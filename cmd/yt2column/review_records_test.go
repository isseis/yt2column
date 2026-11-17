//go:build test

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// reviewRecordsPath is the work tree's copy of the review records script.
// The tests run it directly to check its behavior; that the command
// documents run only origin/main's copy is TestReviewRecordsCallSite's
// concern.
const reviewRecordsPath = "../../.claude/commands/_lib/review-records.sh"

// The fixture's plans: one with PR markers and one without. Their contents
// are the samples under testdata/review_records/.
const (
	rrMarkersPlan = "docs/tasks/0001_markers/03_implementation_plan.md"
	rrSinglePlan  = "docs/tasks/0002_single/03_implementation_plan.md"
	rrSource      = "src.txt"
)

// rrPlanSample returns the content of a plan sample.
func rrPlanSample(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "review_records", name))
	if err != nil {
		t.Fatalf("read the plan sample %s: %v", name, err)
	}
	return string(data)
}

// seenRangeForm reads the form of the seen-range line from the table of
// local-cli-review.md step 7: the first code span of the second cell of the
// row whose first cell is `Local CLI reviewed:`. The fixtures write their
// lines from it, so a form the document changes and the script does not
// follow fails the tests.
func seenRangeForm(t *testing.T) string {
	t.Helper()
	procedure := requireSection(t, "local-cli-review.md", doctestutil.ReadDoc(t, localReviewPath), "## Procedure")
	for line := range strings.Lines(procedureStep(procedure, 7)) {
		cells := strings.Split(strings.TrimSpace(line), " | ")
		if len(cells) < 2 || cells[0] != "| `Local CLI reviewed:`" {
			continue
		}
		form, ok := firstCodeSpan(cells[1])
		if !ok {
			break
		}
		for _, placeholder := range []string{"<START12>", "<END12>", "<cli>"} {
			if strings.Count(form, placeholder) != 1 {
				t.Fatalf("the seen-range form %q does not hold %s once", form, placeholder)
			}
		}
		return form
	}
	t.Fatal("local-cli-review.md step 7 has no `Local CLI reviewed:` row with a form")
	return ""
}

// rrFixture is one fixture repository: main has two commits (m0, then mb,
// which adds the plans), refs/remotes/origin/main points at mb, and the
// branch feature forks from it.
type rrFixture struct {
	*gitFixture
	form   string
	m0, mb string
}

func newRRFixture(t *testing.T, form string) *rrFixture {
	t.Helper()
	f := &rrFixture{gitFixture: newGitFixture(t), form: form}
	f.write(rrSource, "m0\n")
	f.commit("m0")
	f.m0 = f.rev("HEAD")
	f.write(rrMarkersPlan, rrPlanSample(t, "markers.md"))
	f.write(rrSinglePlan, rrPlanSample(t, "single.md"))
	f.commit("add the plans")
	f.mb = f.rev("HEAD")
	f.git("update-ref", "refs/remotes/origin/main", f.mb)
	f.git("switch", "-q", "-c", "feature")
	return f
}

func (f *rrFixture) head() string {
	f.t.Helper()
	return f.rev("HEAD")
}

// work commits a change to the source file with subject and returns the
// commit.
func (f *rrFixture) work(subject string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.repo, rrSource))
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(rrSource, string(data)+subject+"\n")
	f.commit(subject)
	return f.head()
}

// record makes an empty commit whose message is subject and body.
func (f *rrFixture) record(subject string, body ...string) {
	f.t.Helper()
	message := subject
	if len(body) > 0 {
		message += "\n\n" + strings.Join(body, "\n")
	}
	f.git("commit", "-q", "--allow-empty", "-m", message)
}

// seen is a seen-range line for start..end reviewed by cli, in the form of
// local-cli-review.md.
func (f *rrFixture) seen(start, end, cli string) string {
	return strings.NewReplacer("<START12>", start[:12], "<END12>", end[:12], "<cli>", cli).Replace(f.form)
}

// plan writes the sample to the plan at path and commits it alone.
func (f *rrFixture) plan(path, sample string) {
	f.t.Helper()
	f.write(path, rrPlanSample(f.t, sample))
	f.commit("update the plan with " + sample)
}

// merge merges the branch side into the current branch and returns the merge
// commit.
func (f *rrFixture) merge(side string) string {
	f.t.Helper()
	f.git("merge", "-q", "--no-ff", "--no-edit", side)
	return f.head()
}

// side makes a commit on a new branch name forked from base, with a change
// to its own file, comes back to feature, and returns the commit.
func (f *rrFixture) side(name, base string) string {
	f.t.Helper()
	f.git("switch", "-q", "-c", name, base)
	f.write(name+".txt", name+"\n")
	f.commit(name)
	commit := f.head()
	f.git("switch", "-q", "feature")
	return commit
}

// partial is the output for the unseen commits ids: PARTIAL with their
// count, then each as "<12-hex-id> <subject>" in the order git rev-list
// --reverse lists the branch.
func (f *rrFixture) partial(ids ...string) []string {
	f.t.Helper()
	out := []string{"PARTIAL " + strconv.Itoa(len(ids))}
	for c := range strings.FieldsSeq(f.git("rev-list", "--reverse", "HEAD", "^refs/remotes/origin/main")) {
		if slices.Contains(ids, c) {
			subject := strings.TrimSpace(f.git("log", "-1", "--format=%s", c))
			out = append(out, c[:12]+" "+subject)
		}
	}
	if len(out) != len(ids)+1 {
		f.t.Fatalf("partial: some of %v are not on the branch", ids)
	}
	return out
}

// rrCase is one coverage case: setup builds the branch and returns the
// arguments after the mode; want is the expected output, nil when the run
// must fail with "PARTIAL" alone and exit 1.
type rrCase struct {
	name  string
	setup func(f *rrFixture) []string
	env   func(f *rrFixture) []string
	want  func(f *rrFixture) []string
}

var rrAll = func(*rrFixture) []string { return []string{"ALL"} }

// rrNone returns the arguments for LAST_REVIEWED none, HEAD, and plan when
// it is not "".
func rrNone(f *rrFixture, plan string) []string {
	args := []string{"none", f.head()}
	if plan != "" {
		args = append(args, plan)
	}
	return args
}

func coverageCases() []rrCase {
	// unseenAfterSeen is a branch whose first commit a seen-range line covers
	// and whose last commit nothing covers.
	unseenAfterSeen := func(f *rrFixture) []string {
		c1 := f.work("c1")
		f.record("record", f.seen(f.mb, c1, "coderabbit"))
		f.work("c2")
		return rrNone(f, "")
	}
	return []rrCase{
		// The commits judged.
		{name: "no commits", setup: func(f *rrFixture) []string { return rrNone(f, "") }, want: rrAll},
		{name: "unseen commit", setup: func(f *rrFixture) []string {
			f.work("c1")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "seen by coderabbit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: rrAll},
		{name: "seen by codex", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "codex"))
			return rrNone(f, "")
		}, want: rrAll},
		{name: "commit after the seen range", setup: unseenAfterSeen, want: func(f *rrFixture) []string {
			return f.partial(f.rev("HEAD"))
		}},
		{name: "LAST_REVIEWED limits the commits judged", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.record("record", f.seen(c1, c2, "coderabbit"))
			return []string{c1, f.head()}
		}, want: rrAll},
		{name: "LAST_REVIEWED is HEAD", setup: func(f *rrFixture) []string {
			f.work("c1")
			return []string{f.head(), f.head()}
		}, want: rrAll},
		{name: "LAST_REVIEWED is not an ancestor of HEAD", setup: func(f *rrFixture) []string {
			other := f.side("other", f.mb)
			f.work("c1")
			return []string{other, f.head()}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "empty commit", setup: func(f *rrFixture) []string {
			f.record("empty")
			return rrNone(f, "")
		}, want: rrAll},
		{name: "merge commit inside a seen range", setup: func(f *rrFixture) []string {
			f.side("side", f.mb)
			merge := f.merge("side")
			f.record("record", f.seen(f.mb, merge, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		{name: "not run lines do not cancel a seen range", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record",
				"Local CLI review: not run "+f.mb[:12]+".."+c1[:12]+" (coderabbit: other; codex: timeout)",
				f.seen(f.mb, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: rrAll},
		{name: "not run line alone", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", "Local CLI review: not run "+f.mb[:12]+".."+c1[:12]+" (coderabbit: other; codex: timeout)")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		// The checks of a seen-range line.
		{name: "indented and quoted lines", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", "  "+f.seen(f.mb, c1, "coderabbit"), "see "+f.seen(f.mb, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		{name: "another reviewer", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", strings.Replace(f.seen(f.mb, c1, "coderabbit"), "(coderabbit)", "(fable)", 1))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		{name: "START outside the branch", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.m0, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		// START is not an ancestor of END, yet END..START would cover c1.
		{name: "START not an ancestor of END", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			s1 := f.side("side", f.mb)
			f.merge("side")
			f.record("record", f.seen(s1, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string {
			return f.partial(f.rev("HEAD~2"), f.rev("HEAD~1^2"), f.rev("HEAD~1"))
		}},
		// END is on a side branch that is merged only after the line.
		{name: "END not an ancestor of the line's commit", setup: func(f *rrFixture) []string {
			s1 := f.side("side", f.mb)
			f.record("record", f.seen(f.mb, s1, "coderabbit"))
			f.merge("side")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD^2"), f.rev("HEAD")) }},
		// A branch whose name is c1's abbreviation points at origin/main;
		// resolving the name would make the line cover c1 and c2.
		{name: "abbreviation that is also a branch name", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.git("branch", c1[:12], f.mb)
			f.record("record", f.seen(c1, c2, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~2"), f.rev("HEAD~1")) }},
		// A line on origin/main is not read; the END checks back this up,
		// since a commit origin/main reaches cannot have a branch commit as
		// an ancestor.
		{name: "line on origin/main", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			tree := f.rev(f.mb + "^{tree}")
			onMain := strings.TrimSpace(f.git("commit-tree", tree, "-p", f.mb, "-m", "main record\n\n"+f.seen(f.mb, c1, "coderabbit")))
			f.git("update-ref", "refs/remotes/origin/main", onMain)
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// The plan checkbox commits.
		{name: "PR created in a marker section", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		// runpr.md marks these two boxes in its pre-merge commit, together
		// with the checklist chapter's line, after the escalation has run;
		// the case checks the marking rule, and that commit stays unseen.
		{name: "PR merged and next branch in a marker section", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			f.plan(rrMarkersPlan, "markers-merged.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		{name: "next branch not applicable", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-skipped.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		{name: "single-PR record added", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded.md")
			return rrNone(f, rrSinglePlan)
		}, want: rrAll},
		{name: "single-PR record merged", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded.md")
			f.plan(rrSinglePlan, "single-merged.md")
			return rrNone(f, rrSinglePlan)
		}, want: rrAll},
		{name: "PLAN not passed", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "PLAN names another task's plan", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			return rrNone(f, "docs/tasks/0009_missing/03_implementation_plan.md")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox in a code fence", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-fenced.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox in the checklist chapter of a plan with markers", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-section6.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox in another chapter", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-other-chapter.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox text reworded", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-reworded.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "URL of another site", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-bad-url.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// A plan with CRLF line ends is read like one with LF ends, as the
		// Opus review's decision script reads it.
		{name: "CRLF plan", setup: func(f *rrFixture) []string {
			crlf := func(sample string) string {
				return strings.ReplaceAll(rrPlanSample(f.t, sample), "\n", "\r\n")
			}
			f.write(rrMarkersPlan, crlf("markers.md"))
			f.commit("CRLF plan")
			f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
			f.write(rrMarkersPlan, crlf("markers-created.md"))
			f.commit("CRLF checkbox")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		// The lines read the same with or without the final newline, so no
		// checkbox changes: a change must mark one.
		{name: "final newline only", setup: func(f *rrFixture) []string {
			f.write(rrMarkersPlan, strings.TrimSuffix(rrPlanSample(f.t, "markers.md"), "\n"))
			f.commit("drop the final newline")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox and another line", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created-other.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox and another file", setup: func(f *rrFixture) []string {
			f.write(rrMarkersPlan, rrPlanSample(f.t, "markers-created.md"))
			f.write(rrSource, "m0\nchanged\n")
			f.commit("checkbox and source")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "unclosed fence", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-unclosed.md")
			f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
			f.plan(rrMarkersPlan, "markers-unclosed-created.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record in another chapter", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-section7.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record unchecked", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-unchecked.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// A plan the branch marks binary, or gives a diff driver, is read the
		// same: the script reads blobs, not diffs.
		{name: "attributes of the branch", setup: func(f *rrFixture) []string {
			f.write(".gitattributes", "*.md binary\n")
			f.commit("attributes")
			f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
			f.plan(rrMarkersPlan, "markers-created.md")
			if !strings.Contains(f.git("diff", "HEAD~1", "HEAD"), "Binary") {
				f.t.Fatal("the attributes do not make plain git show the plan as binary, so the case checks nothing")
			}
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		// The inputs and the failures.
		{name: "LAST_REVIEWED of another form", setup: func(f *rrFixture) []string {
			return []string{"HEAD", f.head()}
		}},
		{name: "HEAD of another form", setup: func(*rrFixture) []string {
			return []string{"none", "HEAD"}
		}},
		{name: "HEAD abbreviated", setup: func(f *rrFixture) []string {
			return []string{"none", f.head()[:12]}
		}},
		{name: "HEAD names no commit", setup: func(*rrFixture) []string {
			return []string{"none", strings.Repeat("0", 40)}
		}},
		{name: "LAST_REVIEWED names no commit", setup: func(f *rrFixture) []string {
			return []string{strings.Repeat("0", 40), f.head()}
		}},
		{name: "PLAN of another form", setup: func(f *rrFixture) []string {
			return rrNone(f, "../"+rrMarkersPlan)
		}},
		{name: "too many arguments", setup: func(f *rrFixture) []string {
			return append(rrNone(f, rrMarkersPlan), "extra")
		}},
		{name: "too few arguments", setup: func(*rrFixture) []string { return []string{"none"} }},
		{name: "no origin/main", setup: func(f *rrFixture) []string {
			f.git("update-ref", "-d", "refs/remotes/origin/main")
			return rrNone(f, "")
		}},
		{name: "origin/main names a tree", setup: func(f *rrFixture) []string {
			f.git("update-ref", "refs/remotes/origin/main", f.rev(f.mb+"^{tree}"))
			return rrNone(f, "")
		}},
		// The environment. GIT_DIR and GIT_WORK_TREE name a second fixture,
		// which has no HEAD commit of this one, so a script that honored
		// them would fail.
		{name: "GIT_ variables", setup: unseenAfterSeen, env: func(f *rrFixture) []string {
			other := newRRFixture(f.t, f.form)
			return []string{"GIT_DIR=" + filepath.Join(other.repo, ".git"), "GIT_WORK_TREE=" + other.repo}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "clean environment variable preset", setup: unseenAfterSeen, env: func(f *rrFixture) []string {
			other := newRRFixture(f.t, f.form)
			return []string{
				"REVIEW_RECORDS_CLEAN_ENV=1", "BASH_FUNC_command%%=() { return 0; }",
				"GIT_DIR=" + filepath.Join(other.repo, ".git"), "GIT_WORK_TREE=" + other.repo,
			}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "exported functions", setup: unseenAfterSeen, env: func(*rrFixture) []string {
			return []string{
				"BASH_FUNC_git%%=() { return 0; }",
				"BASH_FUNC_awk%%=() { return 0; }",
				"BASH_FUNC_printf%%=() { builtin printf 'ALL\\n'; }",
			}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "BASH_ENV", setup: unseenAfterSeen, env: func(f *rrFixture) []string {
			bashEnv := filepath.Join(f.home, "bash-env")
			fake := "git() { return 0; }\nprintf() { builtin printf 'ALL\\n'; }\n"
			if err := os.WriteFile(bashEnv, []byte(fake), 0o600); err != nil {
				f.t.Fatalf("write BASH_ENV: %v", err)
			}
			return []string{"BASH_ENV=" + bashEnv}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "SHELLOPTS", setup: unseenAfterSeen, env: func(*rrFixture) []string {
			return []string{"SHELLOPTS=errexit:noglob"}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
	}
}

// rrConfigScenario is a git setting a user could have that changes what
// plain git prints, with the plain git command whose output it changes.
type rrConfigScenario struct {
	name   string
	config [2]string
	plain  []string
}

func rrConfigScenarios() []rrConfigScenario {
	diffLast := []string{"diff", "HEAD~1", "HEAD"}
	return []rrConfigScenario{
		{name: "diff.noprefix", config: [2]string{"diff.noprefix", "true"}, plain: diffLast},
		{name: "color.diff", config: [2]string{"color.diff", "always"}, plain: diffLast},
		{name: "diff.external", config: [2]string{"diff.external", "true"}, plain: diffLast},
		{name: "core.abbrev", config: [2]string{"core.abbrev", "20"}, plain: []string{"log", "-1", "--format=%h"}},
		{name: "diff.relative", config: [2]string{"diff.relative", "true"}, plain: []string{"-C", "docs", "diff", "--name-only", "HEAD~4", "HEAD~3"}},
		{name: "diff driver", config: [2]string{"diff.upper.textconv", "tr a-z A-Z <"}, plain: []string{"diff", "HEAD~4", "HEAD~3"}},
		{name: "core.quotePath", config: [2]string{"core.quotePath", "false"}, plain: []string{"diff", "--name-only", "HEAD~1", "HEAD"}},
	}
}

// rrConfigBranch builds the branch the configuration scenarios run on: a
// plan checkbox commit, a seen commit, and an unseen commit whose file has a
// non-ASCII name (so core.quotePath changes plain git's output).
func rrConfigBranch(f *rrFixture) []string {
	// The attribute names a driver only the diff driver scenario defines.
	f.write(".gitattributes", "*.md diff=upper\n")
	f.commit("attributes")
	f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
	f.plan(rrMarkersPlan, "markers-created.md")
	c := f.work("c1")
	f.record("record", f.seen(f.mb, c, "coderabbit"))
	f.write("caf\u00e9.txt", "x\n")
	f.commit("unseen")
	return rrNone(f, rrMarkersPlan)
}

// TestReviewRecordsCoverage runs the coverage mode of the review records
// script on fixture repositories: the commits judged, the checks of a
// seen-range line, the plan checkbox commits, the failures, and that the
// caller's environment and git configuration change nothing. Each case runs
// with each bash scriptBashes returns.
//
// Some guards back each other, so breaking one alone fails no case: the
// env -i run (the script also drops the GIT_ variables and the functions),
// reading lines only from origin/main..HEAD (END must be a branch commit and
// an ancestor of the line's commit), the check that END is a branch commit
// (a range whose END origin/main reaches covers no branch commit), the ref
// check of an abbreviation and resolving a prefix only as an ID (either
// alone keeps a ref named like the prefix from standing in), and the
// check that END is not the line's own commit (a message cannot name its
// own commit).
func TestReviewRecordsCoverage(t *testing.T) {
	for _, tool := range []string{"git", "awk", "tr"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("%s is not on PATH: %v", tool, err)
		}
	}
	form := seenRangeForm(t)
	for _, bash := range scriptBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			for _, c := range coverageCases() {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					f := newRRFixture(t, form)
					args := append([]string{"coverage"}, c.setup(f)...)
					var opts scriptRunOptions
					if c.env != nil {
						opts.extraEnv = c.env(f)
					}
					run := f.runScript(bash, reviewRecordsPath, args, opts)
					if c.want == nil {
						if run.code != 1 || run.stdout != "PARTIAL\n" || run.stderr == "" {
							t.Errorf("got %v, want exit 1, output PARTIAL alone, and a diagnostic", run)
						}
						return
					}
					want := strings.Join(c.want(f), "\n") + "\n"
					if run.code != 0 || run.stdout != want {
						t.Errorf("got %v, want output %q and exit 0", run, want)
					}
				})
			}
			for _, scope := range []string{"repository", "global"} {
				for _, s := range rrConfigScenarios() {
					t.Run("git config/"+scope+"/"+s.name, func(t *testing.T) {
						t.Parallel()
						f := newRRFixture(t, form)
						args := append([]string{"coverage"}, rrConfigBranch(f)...)
						want := f.runScript(bash, reviewRecordsPath, args, scriptRunOptions{})
						if want.code != 0 || !strings.HasPrefix(want.stdout, "PARTIAL 1\n") {
							t.Fatalf("without %s the script did not print PARTIAL 1, so the scenario checks nothing: %v", s.name, want)
						}
						before := f.git(s.plain...)
						file := filepath.Join(f.repo, ".git", "config")
						if scope == "global" {
							file = filepath.Join(f.home, ".gitconfig")
						}
						f.git("config", "--file", file, s.config[0], s.config[1])
						if after := f.git(s.plain...); after == before {
							t.Fatalf("%s does not change plain git's output %q, so the scenario checks nothing", s.name, before)
						}
						if got := f.runScript(bash, reviewRecordsPath, args, scriptRunOptions{}); got != want {
							t.Errorf("with %s set the script printed %v, without it %v", s.name, got, want)
						}
					})
				}
			}
		})
	}
}

// TestReviewRecordsMode checks that a mode other than coverage fails with no
// output, so a caller reads it as the failure of its mode.
func TestReviewRecordsMode(t *testing.T) {
	for _, bash := range scriptBashes(t) {
		for _, mode := range [][]string{nil, {"Coverage"}, {"fable"}} {
			f := newRRFixture(t, "")
			run := f.runScript(bash, reviewRecordsPath, mode, scriptRunOptions{})
			if run.code != 1 || run.stdout != "" || run.stderr == "" {
				t.Errorf("%s with mode %q: got %v, want exit 1, no output, and a diagnostic", bash.name, mode, run)
			}
		}
	}
}

// The code block lines a command document may write the review records
// script in: the reading of origin/main's commit, by its full reference so a
// local branch named origin/main cannot stand in; the write-out of that
// commit's copy to <records-script>, by the commit ID the reading printed;
// and the run of that copy.
const (
	reviewRecordsRef         = "git rev-parse --verify 'refs/remotes/origin/main^{commit}'"
	reviewRecordsShow        = "git show <origin-main-commit>:.claude/commands/_lib/review-records.sh > <records-script>"
	reviewRecordsRunCoverage = "bash <records-script> coverage <LAST_REVIEWED> <HEAD> [<PLAN>]"
)

// reviewRecordsNames are the code spans that only name the review records
// script or its placeholder.
var reviewRecordsNames = []string{
	".claude/commands/_lib/review-records.sh", "_lib/review-records.sh", "<records-script>",
}

// TestReviewRecordsCallSite checks that the command documents run the review
// records script only as origin/main's copy: every line of a backquote-fenced
// block that names the script or its placeholder is the write-out or a run,
// and every code span that names them is the name alone. Running the work
// tree's copy would let the branch under review change the decision that
// lets it skip a PR review. runpr.md's step 3 has the reading of the
// reference, the write-out, and the run on consecutive lines, so a renamed
// form does not pass by matching nothing. A line that names neither the
// script nor its placeholder, or a block fenced otherwise, is out of its
// reach.
func TestReviewRecordsCallSite(t *testing.T) {
	for _, rel := range doctestutil.CommandDocs(t, repositoryRoot) {
		blockLines, spans := codeTexts(doctestutil.ReadDoc(t, filepath.Join(repositoryRoot, rel)))
		for i, code := range slices.Concat(blockLines, spans) {
			if !strings.Contains(code, "review-records") && !strings.Contains(code, "<records-script>") {
				continue
			}
			inBlock := i < len(blockLines)
			switch {
			case inBlock && (code == reviewRecordsShow || code == reviewRecordsRunCoverage):
			case !inBlock && slices.Contains(reviewRecordsNames, code):
			default:
				t.Errorf("%s has %q, which neither names the review records script nor writes out or runs origin/main's copy", rel, code)
			}
		}
	}
	step3 := requireSection(t, "runpr.md", doctestutil.ReadDoc(t, runprPath), runprStep3)
	blockLines, _ := codeTexts(step3)
	if i := slices.Index(blockLines, reviewRecordsShow); i < 1 || i+1 >= len(blockLines) ||
		blockLines[i-1] != reviewRecordsRef || blockLines[i+1] != reviewRecordsRunCoverage {
		t.Errorf("runpr.md's step 3 has no code block lines %q, %q, %q in this order",
			reviewRecordsRef, reviewRecordsShow, reviewRecordsRunCoverage)
	}
}
