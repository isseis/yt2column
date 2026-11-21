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

// fableReviewPath is the Fable partial review's procedure, whose "Record
// lines" define the Fable records the fable mode reads.
const fableReviewPath = "../../.claude/commands/_lib/fable-review.md"

// recordTableForm returns the form of the record line named line in a
// record-line table of section: the first code span of the second cell of
// the row whose first cell is the code span line. It fails t unless the form
// holds each of placeholders once. The fixtures write their lines from the
// forms, so a form a document changes and the script does not follow fails
// the tests.
func recordTableForm(t *testing.T, where, section, line string, placeholders ...string) string {
	t.Helper()
	for row := range strings.Lines(section) {
		cells := strings.Split(strings.TrimSpace(row), " | ")
		if len(cells) < 2 || cells[0] != "| `"+line+"`" {
			continue
		}
		form, ok := firstCodeSpan(cells[1])
		if !ok {
			break
		}
		for _, placeholder := range placeholders {
			if strings.Count(form, placeholder) != 1 {
				t.Fatalf("the form %q of %s does not hold %s once", form, line, placeholder)
			}
		}
		return form
	}
	t.Fatalf("%s has no `%s` row with a form", where, line)
	return ""
}

// localReviewStep7 returns step 7 of local-cli-review.md's procedure, whose
// table defines the local CLI review's record lines.
func localReviewStep7(t *testing.T) string {
	t.Helper()
	procedure := requireSection(t, "local-cli-review.md", doctestutil.ReadDoc(t, localReviewPath), "## Procedure")
	return procedureStep(procedure, 7)
}

// seenRangeForm reads the form of the seen-range line from the table of
// local-cli-review.md step 7.
func seenRangeForm(t *testing.T) string {
	t.Helper()
	return recordTableForm(t, "local-cli-review.md step 7", localReviewStep7(t), "Local CLI reviewed:", "<START12>", "<END12>", "(coderabbit)")
}

// rrFixture is one fixture repository: main has two commits (m0, then mb,
// which adds the plans), refs/remotes/origin/main points at mb, and the
// branch feature forks from it.
type rrFixture struct {
	*gitFixture
	form   string
	forms  fableForms
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
	return strings.NewReplacer("<START12>", start[:12], "<END12>", end[:12], "(coderabbit)", "("+cli+")").Replace(f.form)
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

// rrSubmodule commits a submodule sub at m0 with the .gitmodules gitmodules,
// records the commit as seen, and commits a move of sub to mb, checking that
// plain git with the fixture's settings shows no change in the move.
func rrSubmodule(f *rrFixture, gitmodules string) {
	f.t.Helper()
	f.write(".gitmodules", gitmodules)
	f.git("add", ".gitmodules")
	f.git("update-index", "--add", "--cacheinfo", "160000,"+f.m0+",sub")
	f.git("commit", "-q", "-m", "add sub")
	f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
	f.git("update-index", "--cacheinfo", "160000,"+f.mb+",sub")
	f.git("commit", "-q", "-m", "move sub")
	if f.git("diff-tree", "--no-commit-id", "-r", "HEAD~1", "HEAD") != "" {
		f.t.Fatal("plain git shows the move of sub, so the case checks nothing")
	}
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
		// A rewritten history cannot be judged, even when the commits after
		// the fork are seen.
		{name: "LAST_REVIEWED is not an ancestor of HEAD", setup: func(f *rrFixture) []string {
			other := f.side("other", f.mb)
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			return []string{other, f.head()}
		}},
		// HEAD went back behind LAST_REVIEWED: LAST_REVIEWED..HEAD is empty,
		// yet what Codex reviewed is not what is merged.
		{name: "HEAD behind LAST_REVIEWED", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			back := f.head()
			f.work("c2")
			return []string{f.head(), back}
		}},
		{name: "empty commit", setup: func(f *rrFixture) []string {
			f.record("empty")
			return rrNone(f, "")
		}, want: rrAll},
		// A replace ref makes c1 read as an empty commit to plain git.
		{name: "replace ref", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			tree := f.rev(f.mb + "^{tree}")
			empty := strings.TrimSpace(f.git("commit-tree", tree, "-p", f.mb, "-m", "c1"))
			f.git("replace", c1, empty)
			if strings.TrimSpace(f.git("diff-tree", "--no-commit-id", "-r", f.mb, c1)) != "" {
				f.t.Fatal("the replace ref does not hide c1's change from plain git, so the case checks nothing")
			}
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// The branch's .gitmodules asks git to ignore the submodule, which
		// hides a move of its commit from plain git.
		{name: "submodule ignored by .gitmodules", setup: func(f *rrFixture) []string {
			rrSubmodule(f, "[submodule \"sub\"]\n\tpath = sub\n\turl = ./sub\n\tignore = all\n")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "submodule ignored by the configuration", setup: func(f *rrFixture) []string {
			f.git("config", "submodule.sub.ignore", "all")
			rrSubmodule(f, "[submodule \"sub\"]\n\tpath = sub\n\turl = ./sub\n")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
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
		{name: "text after the form", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit")+" and more")
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
		{name: "green gate confirmed in a marker section", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-greengate.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		{name: "green gate text reworded", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-greengate-reworded.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
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
		{name: "next branch text reworded", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-next-reworded.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "list prefix changed", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-prefix-changed.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record with a URL of another site", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-bad-url.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record with another second line", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-wrong-second.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record of three lines", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-three.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record and another line", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-and-more.md")
			return rrNone(f, rrSinglePlan)
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
		// A checkbox commit that also turns every line end into CRLF.
		{name: "checkbox and line ends", setup: func(f *rrFixture) []string {
			f.write(rrMarkersPlan, strings.ReplaceAll(rrPlanSample(f.t, "markers-created.md"), "\n", "\r\n"))
			f.commit("checkbox and CRLF")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
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
		// A function named command, which only env -i drops, would stop every
		// git the script runs.
		{name: "clean environment variable preset", setup: unseenAfterSeen, env: func(*rrFixture) []string {
			return []string{
				"REVIEW_RECORDS_CLEAN_ENV=1", "BASH_FUNC_command%%=() { return 0; }",
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
			// command is not one of the functions the script unsets, so
			// only env -i (which drops BASH_ENV) keeps it out.
			fake := "command() { return 0; }\n"
			if err := os.WriteFile(bashEnv, []byte(fake), 0o600); err != nil {
				f.t.Fatalf("write BASH_ENV: %v", err)
			}
			return []string{"BASH_ENV=" + bashEnv}
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
		{name: "i18n.logOutputEncoding", config: [2]string{"i18n.logOutputEncoding", "ISO-8859-1"}, plain: []string{"log", "-1", "--format=%s"}},
		{name: "core.quotePath", config: [2]string{"core.quotePath", "false"}, plain: []string{"diff", "--name-only", "HEAD~1", "HEAD"}},
	}
}

// rrConfigBranch builds the branch the configuration scenarios run on: a
// plan checkbox commit, a seen commit, and an unseen commit whose file name
// and subject are not ASCII (so core.quotePath and i18n.logOutputEncoding
// change plain git's output).
func rrConfigBranch(f *rrFixture) []string {
	// The attribute names a driver only the diff driver scenario defines.
	f.write(".gitattributes", "*.md diff=upper\n")
	f.commit("attributes")
	f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
	f.plan(rrMarkersPlan, "markers-created.md")
	c := f.work("c1")
	f.record("record", f.seen(f.mb, c, "coderabbit"))
	f.write("caf\u00e9.txt", "x\n")
	f.commit("unseen caf\u00e9")
	return rrNone(f, rrMarkersPlan)
}

// TestReviewRecordsCoverage runs the coverage mode of the review records
// script on fixture repositories: the commits judged, the checks of a
// seen-range line, the plan checkbox commits, the failures, and that the
// caller's environment and git configuration change nothing. Each case runs
// with each bash scriptBashes returns.
//
// Some guards back each other, so breaking one alone fails no case:
// unsetting the GIT_ variables (env -i drops them too), reading lines only
// from origin/main..HEAD (END must be a branch commit and an ancestor of
// the line's commit), the check that END is a branch commit (a range whose
// END origin/main reaches covers no branch commit), resolving a prefix only
// as an ID (the ref check rejects a prefix a ref is named after), the check
// that PLAN is given (the changed path must equal it), and the check that
// END is not the line's own commit (a message cannot name its own commit).
//
// Most configuration scenarios and the attributes case guard the reading
// through plumbing commands, which no such setting reaches today: they fail
// only when the script starts reading through a porcelain diff. Only
// core.abbrev and i18n.logOutputEncoding reach a command the script runs
// (git log for the listed subjects).
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

// TestReviewRecordsMode checks that a mode other than coverage and fable
// fails with no output, so a caller reads it as the failure of its mode.
func TestReviewRecordsMode(t *testing.T) {
	for _, bash := range scriptBashes(t) {
		for _, mode := range [][]string{nil, {"Coverage"}, {"Fable"}} {
			f := newRRFixture(t, "")
			run := f.runScript(bash, reviewRecordsPath, mode, scriptRunOptions{})
			if run.code != 1 || run.stdout != "" || run.stderr == "" {
				t.Errorf("%s with mode %q: got %v, want exit 1, no output, and a diagnostic", bash.name, mode, run)
			}
		}
	}
}

// fableForms are the record line forms the fable mode reads, from the
// documents that define them: the Fable records and the checkpoint mark of
// fable-review.md's "Record lines", and the not-run line of
// local-cli-review.md step 7.
type fableForms struct {
	run, notRun, checkpoint, localNotRun string
}

func readFableForms(t *testing.T) fableForms {
	t.Helper()
	records := requireSection(t, "fable-review.md", doctestutil.ReadDoc(t, fableReviewPath), "## Record lines")
	where := "fable-review.md \"Record lines\""
	return fableForms{
		run: recordTableForm(t, where, records, "Fable review: run",
			"<START12>", "<END12>", "<model-id>", "<seconds>", "<tokens>"),
		notRun: recordTableForm(t, where, records, "Fable review: not run",
			"<START12>", "<END12>", "<reason>", "[; <seconds>s, tokens <tokens>]"),
		checkpoint: recordTableForm(t, where, records, "Fable review: checkpoint"),
		localNotRun: recordTableForm(t, "local-cli-review.md step 7", localReviewStep7(t), "Local CLI review: not run",
			"<START12>", "<END12>"),
	}
}

// rrRange replaces the range placeholders of form with start and end.
func rrRange(form, start, end string) string {
	return strings.NewReplacer("<START12>", start[:12], "<END12>", end[:12]).Replace(form)
}

// fableRun is a run record for start..end, every count and the cost filled
// in.
func (f *rrFixture) fableRun(start, end string) string {
	return strings.NewReplacer("<model-id>", "claude-fable-example", "<n>", "1", "<seconds>", "120", "<tokens>", "4567").
		Replace(rrRange(f.forms.run, start, end))
}

// fableNotRun is a not-run record for start..end with reason, with the cost
// when withCost is true and without it otherwise.
func (f *rrFixture) fableNotRun(start, end, reason string, withCost bool) string {
	line := strings.Replace(rrRange(f.forms.notRun, start, end), "<reason>", reason, 1)
	if withCost {
		line = strings.NewReplacer("[", "", "]", "", "<seconds>", "75", "<tokens>", "unknown").Replace(line)
	} else {
		line = strings.Replace(line, "[; <seconds>s, tokens <tokens>]", "", 1)
	}
	return line
}

// localNotRun is a local CLI review not-run line for start..end.
func (f *rrFixture) localNotRun(start, end string) string {
	return strings.ReplaceAll(rrRange(f.forms.localNotRun, start, end), "<kind>", "other")
}

// readme commits a change to README.en.md alone.
func (f *rrFixture) readme(content string) {
	f.t.Helper()
	f.write("README.en.md", content)
	f.commit("sync README.en.md")
}

// rrFable returns the fable mode's arguments for entry, HEAD, and the
// fixture's plan with PR markers.
func rrFable(f *rrFixture, entry string) []string {
	return []string{entry, f.head(), rrMarkersPlan}
}

// rrValid is the output of a valid record.
var rrValid = func(*rrFixture) []string { return []string{"VALID"} }

// rrMissing returns the output MISSING from the commit name resolves to.
func rrMissing(name string) func(f *rrFixture) []string {
	return func(f *rrFixture) []string { return []string{"MISSING " + f.rev(name)} }
}

// fableCases are the fable mode's cases; a case whose setup does not pass
// an entry runs at launch.
func fableCases() []rrCase {
	launch := func(f *rrFixture) []string { return rrFable(f, "launch") }
	// recorded adds one work commit c1 and a run record of merge-base..c1
	// in the empty commit after it.
	recorded := func(f *rrFixture) {
		c1 := f.work("c1")
		f.record("record", f.fableRun(f.mb, c1))
	}
	// unseenAfterRecord adds a work commit after a valid record.
	unseenAfterRecord := func(f *rrFixture) []string {
		recorded(f)
		f.work("c2")
		return launch(f)
	}
	return []rrCase{
		// The record.
		{name: "no commits", setup: launch, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "no record", setup: func(f *rrFixture) []string {
			f.work("c1")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "run record", setup: func(f *rrFixture) []string {
			recorded(f)
			return launch(f)
		}, want: rrValid},
		{name: "not run record with its cost", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableNotRun(f.mb, c1, "timeout", true))
			return launch(f)
		}, want: rrValid},
		{name: "not run record without a cost", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableNotRun(f.mb, c1, "agent-cannot-call", false))
			return launch(f)
		}, want: rrValid},
		{name: "not run record of another reason", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableNotRun(f.mb, c1, "busy", false))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "work after the record", setup: unseenAfterRecord, want: rrMissing("HEAD~1")},
		{name: "indented and quoted records", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", "  "+f.fableRun(f.mb, c1), "see "+f.fableRun(f.mb, c1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "checkpoint mark alone", setup: func(f *rrFixture) []string {
			f.work("c1")
			f.record("mark", f.forms.checkpoint)
			return rrFable(f, "checkpoint")
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// A record whose range fails the checks is ignored.
		{name: "record with START outside the branch", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableRun(f.m0, c1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "record naming its own commit's descendant", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.git("reset", "-q", "--hard", c1)
			f.record("record", f.fableRun(f.mb, c2))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// The last record decides, in either order.
		{name: "last of two records valid", setup: func(f *rrFixture) []string {
			recorded(f)
			r1 := f.head()
			c2 := f.work("c2")
			f.record("record", f.fableRun(r1, c2))
			return launch(f)
		}, want: rrValid},
		{name: "last of two records invalid", setup: func(f *rrFixture) []string {
			recorded(f)
			r1 := f.head()
			c2 := f.work("c2")
			f.record("record", f.fableRun(r1, c2))
			f.work("c3")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "last record covers less than an earlier one", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.record("record", f.fableRun(f.mb, c2))
			f.record("record", f.fableRun(f.mb, c1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.rev("HEAD~3")} }},
		// The fixes of the same run.
		{name: "fix inside a seen range of the record's commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			fix := f.work("fix")
			f.record("record", f.fableRun(f.mb, c1), f.seen(f.mb, fix, "coderabbit"))
			return launch(f)
		}, want: rrValid},
		{name: "fix inside a not-run line of the record's commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			fix := f.work("fix")
			f.record("record", f.fableRun(f.mb, c1), f.localNotRun(c1, fix))
			return launch(f)
		}, want: rrValid},
		{name: "fix outside every range of the record's commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.work("fix")
			f.record("record", f.fableRun(f.mb, c1), f.seen(f.mb, c1, "coderabbit"))
			return launch(f)
		}, want: rrMissing("HEAD~2")},
		{name: "fix inside a range of a later commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			fix := f.work("fix")
			f.record("record", f.fableRun(f.mb, c1))
			f.record("record", f.seen(f.mb, fix, "coderabbit"))
			return launch(f)
		}, want: rrMissing("HEAD~3")},
		// The side commit is inside the record's line by its IDs, but the
		// line fails the checks: its END is not an ancestor of the record's
		// commit.
		{name: "range of the record's commit beyond it", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			s1 := f.side("side", c1)
			f.record("record", f.fableRun(f.mb, c1), f.seen(c1, s1, "coderabbit"))
			f.merge("side")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.rev("HEAD~2")} }},
		// The record starts inside the branch, so it stays valid after
		// origin/main moves to m3. x, the first commit it misses, has the
		// parent m2, which origin/main reaches behind the merge-base m3: a
		// record from m2 would fail the checks, so START falls back to the
		// merge-base.
		{name: "parent of the first missing commit behind the merge-base", setup: func(f *rrFixture) []string {
			c0 := f.work("c0")
			c1 := f.work("c1")
			f.record("record", f.fableRun(c0, c1))
			tree := f.rev(f.mb + "^{tree}")
			m2 := strings.TrimSpace(f.git("commit-tree", tree, "-p", f.mb, "-m", "m2"))
			m3 := strings.TrimSpace(f.git("commit-tree", tree, "-p", m2, "-m", "m3"))
			f.git("update-ref", "refs/remotes/origin/main", m3)
			f.side("x", m2)
			f.merge("x")
			f.git("merge", "-q", "--no-ff", "--no-edit", m3)
			if f.rev("x^1") != m2 || strings.TrimSpace(f.git("merge-base", "HEAD", m3)) != m3 {
				f.t.Fatal("x's parent is not m2 or the merge-base is not m3, so the case checks nothing")
			}
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// A merge of origin/main moves the merge-base past a record that
		// starts at the old one, which then fails the checks.
		{name: "record from a merge-base origin/main moved past", setup: func(f *rrFixture) []string {
			recorded(f)
			tree := f.rev(f.mb + "^{tree}")
			m2 := strings.TrimSpace(f.git("commit-tree", tree, "-p", f.mb, "-m", "m2"))
			f.git("update-ref", "refs/remotes/origin/main", m2)
			f.git("merge", "-q", "--no-ff", "--no-edit", m2)
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// The commits without content.
		{name: "README.en.md after the record", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.readme("two\n")
			return launch(f)
		}, want: rrValid},
		{name: "README.en.md created after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			f.readme("one\n")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "README.en.md and another file", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.write("README.en.md", "two\n")
			f.write(rrSource, "changed\n")
			f.commit("README.en.md and source")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "plan checkbox after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			f.plan(rrMarkersPlan, "markers-created.md")
			return launch(f)
		}, want: rrValid},
		{name: "plan checkbox and README.en.md together", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.write(rrMarkersPlan, rrPlanSample(f.t, "markers-created.md"))
			f.write("README.en.md", "two\n")
			f.commit("checkbox and README.en.md")
			return launch(f)
		}, want: rrValid},
		{name: "another line of the plan", setup: func(f *rrFixture) []string {
			recorded(f)
			f.plan(rrMarkersPlan, "markers-created-other.md")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "checkbox of another task's plan", setup: func(f *rrFixture) []string {
			recorded(f)
			f.plan(rrSinglePlan, "single-recorded.md")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "plan absent", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.readme("two\n")
			return []string{"launch", f.head(), "docs/tasks/0009_missing/03_implementation_plan.md"}
		}, want: rrValid},
		// The merge brings in an empty commit, so it changes nothing against
		// its first parent: only the merge rule keeps it from counting.
		{name: "merge after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			f.git("switch", "-q", "-c", "side")
			f.record("empty side")
			f.git("switch", "-q", "feature")
			f.merge("side")
			return launch(f)
		}, want: rrMissing("HEAD^1")},
		{name: "attributes of the branch", setup: func(f *rrFixture) []string {
			f.write(".gitattributes", "*.md binary\n")
			f.commit("attributes")
			recorded(f)
			f.plan(rrMarkersPlan, "markers-created.md")
			if !strings.Contains(f.git("diff", "HEAD~1", "HEAD"), "Binary") {
				f.t.Fatal("the attributes do not make plain git show the plan as binary, so the case checks nothing")
			}
			return launch(f)
		}, want: rrValid},
		{name: "run record of another form", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", strings.Replace(f.fableRun(f.mb, c1), ", invalid 1", "", 1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "run record of another model", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", strings.Replace(f.fableRun(f.mb, c1), "claude-fable-example", "claude-opus-example", 1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// The checkpoint mark.
		// The last mark decides: the fix between the two marks is after the
		// first one only.
		{name: "work between two marks", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("gate fix")
			f.record("mark", f.forms.checkpoint)
			f.work("later fix")
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~3")},
		{name: "work after the mark at the checkpoint", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("gate fix")
			return rrFable(f, "checkpoint")
		}, want: rrValid},
		{name: "work after the mark at launch", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("gate fix")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "work before the mark", setup: func(f *rrFixture) []string {
			recorded(f)
			f.work("phase group")
			f.record("mark", f.forms.checkpoint)
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~2")},
		{name: "implementation record after the mark", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("phase group")
			f.record("record", "Implementation model: "+f.mb[:12]+".."+f.head()[:12]+" claude-opus-example")
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~2")},
		{name: "mark in a commit with content", setup: func(f *rrFixture) []string {
			recorded(f)
			f.write(rrSource, "marked\n")
			f.commit("mark\n\n" + f.forms.checkpoint)
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~1")},
		// The inputs and the failures.
		{name: "ENTRY of another form", setup: func(f *rrFixture) []string { return rrFable(f, "Launch") }},
		{name: "HEAD of another form", setup: func(*rrFixture) []string {
			return []string{"launch", "HEAD", rrMarkersPlan}
		}},
		{name: "HEAD names no commit", setup: func(*rrFixture) []string {
			return []string{"launch", strings.Repeat("0", 40), rrMarkersPlan}
		}},
		{name: "PLAN of another form", setup: func(f *rrFixture) []string {
			return []string{"launch", f.head(), "../" + rrMarkersPlan}
		}},
		{name: "PLAN not passed", setup: func(f *rrFixture) []string { return []string{"launch", f.head()} }},
		{name: "too many arguments", setup: func(f *rrFixture) []string { return append(rrFable(f, "launch"), "extra") }},
		{name: "no origin/main", setup: func(f *rrFixture) []string {
			f.git("update-ref", "-d", "refs/remotes/origin/main")
			return launch(f)
		}},
		// The environment, as for coverage.
		{name: "GIT_ variables", setup: unseenAfterRecord, env: func(f *rrFixture) []string {
			other := newRRFixture(f.t, f.form)
			return []string{"GIT_DIR=" + filepath.Join(other.repo, ".git"), "GIT_WORK_TREE=" + other.repo}
		}, want: rrMissing("HEAD~1")},
		{name: "exported functions", setup: unseenAfterRecord, env: func(*rrFixture) []string {
			return []string{
				"BASH_FUNC_git%%=() { return 0; }",
				"BASH_FUNC_printf%%=() { builtin printf 'VALID\\n'; }",
				"REVIEW_RECORDS_CLEAN_ENV=1",
			}
		}, want: rrMissing("HEAD~1")},
		{name: "BASH_ENV", setup: unseenAfterRecord, env: func(f *rrFixture) []string {
			bashEnv := filepath.Join(f.home, "bash-env")
			if err := os.WriteFile(bashEnv, []byte("command() { return 0; }\n"), 0o600); err != nil {
				f.t.Fatalf("write BASH_ENV: %v", err)
			}
			return []string{"BASH_ENV=" + bashEnv}
		}, want: rrMissing("HEAD~1")},
	}
}

// rrFableConfigBranch builds a branch on which the fable mode prints
// MISSING from the commit before the last: a record, a plan checkbox commit,
// two empty commits, and a last commit whose file name and subject are not
// ASCII, the first commit no rule covers. HEAD~4..HEAD~3 is the checkbox
// commit, as in rrConfigBranch, so the configuration scenarios' plain git
// commands change their output.
func rrFableConfigBranch(f *rrFixture) []string {
	f.write(".gitattributes", "*.md diff=upper\n")
	f.commit("attributes")
	f.record("record", f.fableRun(f.mb, f.head()))
	f.plan(rrMarkersPlan, "markers-created.md")
	f.record("empty")
	f.record("empty")
	f.write("caf\u00e9.txt", "x\n")
	f.commit("unseen caf\u00e9")
	return rrFable(f, "launch")
}

// TestReviewRecordsFable runs the fable mode of the review records script on
// fixture repositories: the last record, the fixes of the same run, the
// commits without content, the checkpoint mark, the failures, and that the
// caller's environment and git configuration change nothing. Each case runs
// with each bash scriptBashes returns.
//
// The environment cases and the failures of the shared branch loading repeat
// TestReviewRecordsCoverage's on purpose: they show the fable mode keeps the
// same guarantees and fails with its own output. The configuration
// scenarios guard the reading through plumbing commands, which no such
// setting reaches today: they fail only when the fable mode starts reading
// through a porcelain command.
func TestReviewRecordsFable(t *testing.T) {
	form := seenRangeForm(t)
	forms := readFableForms(t)
	for _, bash := range scriptBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			for _, c := range fableCases() {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					f := newRRFixture(t, form)
					f.forms = forms
					args := append([]string{"fable"}, c.setup(f)...)
					var opts scriptRunOptions
					if c.env != nil {
						opts.extraEnv = c.env(f)
					}
					run := f.runScript(bash, reviewRecordsPath, args, opts)
					if c.want == nil {
						if run.code != 1 || run.stdout != "MISSING\n" || run.stderr == "" {
							t.Errorf("got %v, want exit 1, output MISSING alone, and a diagnostic", run)
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
						f.forms = forms
						args := append([]string{"fable"}, rrFableConfigBranch(f)...)
						want := f.runScript(bash, reviewRecordsPath, args, scriptRunOptions{})
						if want.code != 0 || want.stdout != "MISSING "+f.rev("HEAD~1")+"\n" {
							t.Fatalf("without %s the script did not print MISSING from the commit before the last, so the scenario checks nothing: %v", s.name, want)
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

// The code block lines a command document may write the review records
// script in: the reading of origin/main's commit, by its full reference so a
// local branch named origin/main cannot stand in; the write-out of that
// commit's copy to <records-script>, by the commit ID the reading printed;
// and the run of that copy.
const (
	reviewRecordsRef          = "git rev-parse --verify 'refs/remotes/origin/main^{commit}'"
	reviewRecordsShow         = "git --no-replace-objects show <origin-main-commit>:.claude/commands/_lib/review-records.sh > <records-script>"
	reviewRecordsRunCoverage  = "bash <records-script> coverage <LAST_REVIEWED> <HEAD> [<PLAN>]"
	reviewRecordsRunRemaining = "bash <records-script> coverage none <HEAD> [<PLAN>]"
	reviewRecordsRunFable     = "bash <records-script> fable <ENTRY> <HEAD> <PLAN>"
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
// lets it skip a PR review or a Fable partial review. task-pr-handoff.md's
// Part B step 2.5, runpr.md's step 3, and fable-review.md's validity decision,
// which the launch and the checkpoint entries run, have the reading of the
// reference, the write-out, and the run on consecutive lines, so a renamed form
// does not pass by matching nothing.
// A line that names neither the script nor its placeholder, or a block
// fenced otherwise, is out of its reach.
func TestReviewRecordsCallSite(t *testing.T) {
	for _, rel := range doctestutil.CommandDocs(t, repositoryRoot) {
		blockLines, spans := codeTexts(doctestutil.ReadDoc(t, filepath.Join(repositoryRoot, rel)))
		for i, code := range slices.Concat(blockLines, spans) {
			if !strings.Contains(code, "review-records") && !strings.Contains(code, "<records-script>") {
				continue
			}
			inBlock := i < len(blockLines)
			switch {
			case inBlock && (code == reviewRecordsShow || code == reviewRecordsRunCoverage || code == reviewRecordsRunRemaining || code == reviewRecordsRunFable):
			case !inBlock && slices.Contains(reviewRecordsNames, code):
			default:
				t.Errorf("%s has %q, which neither names the review records script nor writes out or runs origin/main's copy", rel, code)
			}
		}
	}
	for _, site := range []struct {
		name, path, run string
		section         func(*testing.T, string) string
	}{
		{"handoff step 2.5", handoffPath, reviewRecordsRunRemaining, itemSection("\n2.5. ", "\n3. ")},
		{"runpr.md's step 3", runprPath, reviewRecordsRunCoverage, headingSection(runprStep3)},
		{"fable-review.md's validity decision", fableReviewPath, reviewRecordsRunFable, headingSection("## The validity decision")},
	} {
		blockLines, _ := codeTexts(site.section(t, doctestutil.ReadDoc(t, site.path)))
		if i := slices.Index(blockLines, reviewRecordsShow); i < 1 || i+1 >= len(blockLines) ||
			blockLines[i-1] != reviewRecordsRef || blockLines[i+1] != site.run {
			t.Errorf("%s has no code block lines %q, %q, %q in this order",
				site.name, reviewRecordsRef, reviewRecordsShow, site.run)
		}
	}
}

// Paths of the other documents that define or write record lines.
const (
	opusReviewPath = "../../.claude/commands/_lib/opus-review.md"
	runplanPath    = "../../.claude/commands/runplan.md"
)

// recordPrefix is a record line's prefix: its text up to and including the
// first colon.
func recordPrefix(t *testing.T, where, line string) string {
	t.Helper()
	prefix, _, ok := strings.Cut(line, ":")
	if !ok {
		t.Fatalf("%s has the record line %q, which has no colon", where, line)
	}
	return prefix + ":"
}

// tableLinePrefixes returns the prefixes of the record lines named in the
// first column of the record-line tables in section, the tables whose
// header's first cell is "Line".
func tableLinePrefixes(t *testing.T, where, section string) []string {
	t.Helper()
	var prefixes []string
	inTable := false
	for row := range strings.Lines(section) {
		trimmed := strings.TrimSpace(row)
		if !strings.HasPrefix(trimmed, "|") {
			inTable = false
			continue
		}
		first, _, ok := strings.Cut(strings.TrimPrefix(trimmed, "| "), " | ")
		if first == "Line" {
			inTable = true
			continue
		}
		if !inTable || !ok || !strings.HasPrefix(first, "`") {
			continue
		}
		if line, found := firstCodeSpan(first); found {
			prefixes = append(prefixes, recordPrefix(t, where, line))
		}
	}
	return prefixes
}

// TestRecordLinePrefixesDistinct checks that the record lines the Fable
// partial review and the seen-range line add cannot be read as an existing
// record line: no new prefix equals an existing one, and neither starts
// with the other. The existing prefixes are those of local-cli-review.md
// step 7's table (the seen-range line aside) and of the code blocks of
// opus-review.md's "Record lines", which _lib/opus-review-decision.sh and
// the local CLI review's readers read; the new ones are those of
// fable-review.md's "Record lines" table and the seen-range line.
func TestRecordLinePrefixesDistinct(t *testing.T) {
	const seenPrefix = "Local CLI reviewed:"
	existing := map[string]struct{}{}
	var added []string
	for _, prefix := range tableLinePrefixes(t, "local-cli-review.md step 7", localReviewStep7(t)) {
		if prefix == seenPrefix {
			added = append(added, prefix)
			continue
		}
		existing[prefix] = struct{}{}
	}
	localCount := len(existing)
	opus := requireSection(t, "opus-review.md", doctestutil.ReadDoc(t, opusReviewPath), "## Record lines")
	blockLines, _ := codeTexts(opus)
	for _, line := range blockLines {
		existing[recordPrefix(t, "opus-review.md \"Record lines\"", line)] = struct{}{}
	}
	fable := requireSection(t, "fable-review.md", doctestutil.ReadDoc(t, fableReviewPath), "## Record lines")
	fablePrefixes := map[string]struct{}{}
	for _, prefix := range tableLinePrefixes(t, "fable-review.md \"Record lines\"", fable) {
		fablePrefixes[prefix] = struct{}{}
		added = append(added, prefix)
	}
	// The lower bounds keep a table the reading misses from passing with
	// nothing compared.
	if localCount < 3 || len(existing)-localCount < 2 || len(fablePrefixes) < 3 || !slices.Contains(added, seenPrefix) {
		t.Fatalf("read %d prefixes from local-cli-review.md step 7, %d more from opus-review.md, %d from fable-review.md, and the seen-range line %v; want at least 3, 2, 3, and true",
			localCount, len(existing)-localCount, len(fablePrefixes), slices.Contains(added, seenPrefix))
	}
	for _, n := range added {
		for e := range existing {
			if strings.HasPrefix(n, e) || strings.HasPrefix(e, n) {
				t.Errorf("the new record line prefix %q overlaps the existing prefix %q", n, e)
			}
		}
	}
}

// mentionCheckReference is how a section names the mention check that runs
// before a commit of record lines.
const mentionCheckReference = "`_lib/local-cli-review.md` step 7, \"Mention check\""

// TestRecordCommitsCheckMention checks that each section that makes a commit
// holding a Fable line runs the mention check first: it holds the check's
// command or names local-cli-review.md step 7's "Mention check". A Fable line
// carries the session's summaries of Fable's findings, which may hold the
// mention.
func TestRecordCommitsCheckMention(t *testing.T) {
	runplan := doctestutil.ReadDoc(t, runplanPath)
	var step78 string
	for line := range strings.Lines(runplan) {
		if strings.HasPrefix(line, "7.8. ") {
			step78 = line
		}
	}
	fable := doctestutil.ReadDoc(t, fableReviewPath)
	for _, site := range []struct{ name, text string }{
		{"local-cli-review.md step 7", localReviewStep7(t)},
		{"runplan.md step 7.8", step78},
		{"fable-review.md \"Record lines\"", requireSection(t, "fable-review.md", fable, "## Record lines")},
		{"fable-review.md \"The standalone path\"", requireSection(t, "fable-review.md", fable, "## The standalone path")},
	} {
		text := flat(site.text)
		if text == "" {
			t.Errorf("%s is missing", site.name)
			continue
		}
		if !strings.Contains(text, mentionCheckCommand) && !strings.Contains(text, flat(mentionCheckReference)) {
			t.Errorf("%s neither runs %s nor names %s", site.name, mentionCheckCommand, mentionCheckReference)
		}
	}
}
